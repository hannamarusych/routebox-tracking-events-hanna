# Architecture

## Overview

RouteBox Tracking Events is a small, focused Go service. It sits at the edge of the RouteBox platform, converting inbound carrier webhook traffic into a normalized, deduplicated event stream that other services can rely on. It intentionally does not contain business logic about shipments, routing, or customer-facing status — that logic lives downstream.

## Internal Design

- **HTTP layer** (`internal/server`): built on chi, exposes one route family (`/v1/webhooks/<carrier>`) plus health checks.
- **Carrier adapters** (`internal/carriers`): one file per carrier (UPS, FedEx, DHL, USPS). Each adapter is responsible for its own signature verification scheme and payload shape, then converts the payload into a shared internal event struct.
- **Auth** (`internal/auth`): validates carrier signatures using per-carrier secrets loaded from `CARRIER_SIGNATURE_SECRETS`.
- **Persistence** (`internal/db`): writes normalized events into the `carrier_events_raw` table using pgx, relying on a Postgres unique constraint plus `ON CONFLICT DO NOTHING` for deduplication.

The design favors simplicity over flexibility: adding a new carrier means adding a new adapter, not changing the core pipeline.

## Fit Within the RouteBox Platform

This service is the ingestion boundary for the platform. Carriers talk to it directly over the public internet; nothing else does. Downstream, `routebox-shipments-api-hanna` and `routebox-ops-console-hanna` read from the same `carrier_events_raw` table to build shipment status and operational views. `routebox-infra-tf-hanna` provisions the ECS service, database, and secrets this service depends on.

Communication is deliberately one-directional and asynchronous: this service writes, others read. There are no synchronous service-to-service calls into or out of this component, which keeps the ingestion path resilient to slowdowns elsewhere in the platform.

## Deployment Architecture

Runs as a container on ECS, built and deployed through the shared Jenkins pipeline (`deployToEcs`). Configuration and secrets (database URL, carrier signing secrets, AWS credentials) are injected at deploy time from Secrets Manager. See [docs/DEPLOYMENT.md](./docs/DEPLOYMENT.md) for the full flow and [docs/TROUBLESHOOTING.md](./docs/TROUBLESHOOTING.md) for the AWS authentication tradeoff specific to this service.

## Observability

Health today is inferred mostly from infrastructure-level metrics (throughput, error rates, container health) rather than dedicated application dashboards. Improving observability for this service is tracked in [ROADMAP.md](./ROADMAP.md).

## Security Considerations

- Carrier signatures are verified per-request before any event is persisted.
- Payloads are size-limited (`MAX_PAYLOAD_BYTES`) to reduce abuse potential.
- AWS authentication currently uses long-lived access keys rather than the ECS task role. This is a known, documented tradeoff — see [docs/TROUBLESHOOTING.md](./docs/TROUBLESHOOTING.md) for the full history and reasoning.

## Future Improvements

- Migrate AWS authentication back to the ECS task role once the underlying SDK credential-refresh issue is resolved.
- Update `docker-compose.yml` so local development works without depending on the dev AWS environment.
- Add DHL v2 webhook format support.
- Add service-level dashboards and alerting rather than relying solely on infrastructure metrics.
