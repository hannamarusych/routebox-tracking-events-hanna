# RouteBox Tracking Events

## Overview

RouteBox Tracking Events is the ingestion service for carrier webhook data across the RouteBox platform. It receives shipment status updates directly from carriers (UPS, FedEx, DHL, USPS, and a long tail of regional carriers), validates and deduplicates each event, and persists the raw event stream so downstream services can update shipment state. It exists to give the platform a single, reliable entry point for carrier data instead of every service integrating with every carrier independently.

---

## RouteBox Platform

This repository is one component of the RouteBox platform.

The platform is split across five repositories:

- **[routebox-infra-tf-hanna](https://github.com/hannamarusych/routebox-infra-tf-hanna)** — AWS infrastructure-as-code with Terraform: modular resources, environment separation, and state management.
- **[routebox-shipments-api-hanna](https://github.com/hannamarusych/routebox-shipments-api-hanna)** — customer-facing REST service that owns the shipment lifecycle (create, read, update, webhook delivery).
- **[routebox-tracking-events-hanna](https://github.com/hannamarusych/routebox-tracking-events-hanna)** — carrier webhook ingestion service that validates, deduplicates, and persists status events from UPS, FedEx, DHL, USPS, and regional carriers. *(this repository)*
- **[routebox-route-optimizer-hanna](https://github.com/hannamarusych/routebox-route-optimizer-hanna)** — queue-driven route-calculation worker that solves multi-stop vehicle routing with Google OR-Tools as a horizontally scalable background process.
- **[routebox-ops-console-hanna](https://github.com/hannamarusych/routebox-ops-console-hanna)** — internal admin dashboard (Rails) for support, billing, and operations staff to manage tenants and investigate shipments.

---

## Key Highlights

- Real-world infrastructure patterns
- Event ingestion at high write throughput
- Carrier webhook signature validation (HMAC, per-carrier)
- Deduplication against a rolling time window
- AWS (ECS, Secrets Manager)
- Platform engineering practices

---

## Architecture

See [ARCHITECTURE.md](./ARCHITECTURE.md) for the full design, and the [diagrams](./diagrams) folder for visuals as they are added.

---

## Technology Stack

- Go 1.21
- chi (HTTP router)
- pgx (PostgreSQL driver)
- AWS SDK for Go
- PostgreSQL
- Docker, Jenkins (CI/CD)
- Deployed on AWS ECS

---

## Role in the Platform

This service is the boundary between the outside world (carrier webhook calls) and the RouteBox platform. It is intentionally narrow in scope: validate, dedupe, and record. It does not interpret shipment business logic, calculate routes, or expose data to end users directly. That separation keeps the ingestion path simple and fast, which matters because carriers expect a quick response and will retry aggressively if they do not get one.

## How It Interacts With the Infrastructure

The service runs as a container on the ECS infrastructure defined in `routebox-infra-tf-hanna`. It reads carrier signing secrets and AWS credentials from Secrets Manager, and writes to a shared PostgreSQL instance. Deploys go through the shared Jenkins pipeline, which wires the service-specific secrets into the ECS task at deploy time.

## Communication With Other RouteBox Services

This service does not call other RouteBox services directly, and no other service calls it except carriers over the public webhook endpoints. Instead, it communicates asynchronously through the database: it writes to the `carrier_events_raw` table, and downstream services (`routebox-shipments-api-hanna` and `routebox-ops-console-hanna`) read from that table to update shipment status and surface tracking information. This keeps ingestion decoupled from the services that depend on it, so a slowdown downstream never causes carrier webhooks to fail.

## Deployment

Deployed via the standard Jenkins pipeline described in the Jenkinsfile, which calls into the shared `routebox-jenkins` library's `deployToEcs` function. This service has one deployment special case worth knowing about: it authenticates to AWS using long-lived access keys rather than the ECS task role, for reasons explained in [docs/TROUBLESHOOTING.md](./docs/TROUBLESHOOTING.md). See [docs/DEPLOYMENT.md](./docs/DEPLOYMENT.md) for the full flow.

## Monitoring

The service is stateless aside from its database writes, so health is primarily tracked through write throughput, error rates on webhook validation, and dedupe collisions. Known gaps and the reasoning behind current tradeoffs are documented in [docs/TROUBLESHOOTING.md](./docs/TROUBLESHOOTING.md).

## Where This Fits in the Overall Architecture

RouteBox Tracking Events sits at the edge of the platform, in front of the database that `routebox-shipments-api-hanna` and `routebox-ops-console-hanna` both read from. It is the first hop for any external carrier data entering the system, and the platform's tracking accuracy depends on it staying fast and reliable.

---

## What This Service Does

- Receives carrier webhook payloads at `/v1/webhooks/<carrier>`
- Validates the carrier-specific signature (HMAC, varies per carrier)
- Parses each payload into a normalized event format
- Deduplicates against the last 24 hours of events for the same carrier and tracking number
- Writes accepted events to the `carrier_events_raw` table in PostgreSQL
- Returns a 200 response to the carrier

## Repository Layout

```
.
├── cmd/
│   └── server/
│       └── main.go
├── internal/
│   ├── carriers/
│   │   ├── ups.go
│   │   ├── fedex.go
│   │   ├── dhl.go
│   │   └── usps.go
│   ├── auth/
│   ├── db/
│   └── server/
├── Dockerfile
├── docker-compose.yml
├── Jenkinsfile
├── go.mod
└── go.sum
```

## Configuration

Key environment variables:

- `DATABASE_URL` — PostgreSQL connection string
- `AWS_ACCESS_KEY_ID` / `AWS_SECRET_ACCESS_KEY` — long-lived credentials, see [docs/TROUBLESHOOTING.md](./docs/TROUBLESHOOTING.md)
- `CARRIER_SIGNATURE_SECRETS` — JSON map of carrier to signing secret, sourced from Secrets Manager
- `DEDUP_WINDOW_HOURS` — default 24
- `MAX_PAYLOAD_BYTES` — default 256KB; larger payloads are rejected with a 413

## Running Locally

```
docker compose up
```

Local dev currently has known gaps around the Postgres image and AWS auth path — see [docs/TROUBLESHOOTING.md](./docs/TROUBLESHOOTING.md) for details and current workarounds.

## Known Issues

- Authentication uses long-lived AWS keys instead of the ECS task role (documented tradeoff, see Troubleshooting)
- `docker-compose.yml` is out of date for local development
- The DHL carrier handler has not been migrated to DHL's v2 webhook format
- Deduplication is best-effort; rare races can allow the same event to land twice, which downstream consumers tolerate

For more background, see [docs/TROUBLESHOOTING.md](./docs/TROUBLESHOOTING.md).
