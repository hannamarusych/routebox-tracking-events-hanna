# routebox-tracking-events

Go service. Ingests carrier webhook events (UPS, FedEx, DHL, USPS, plus a long tail of regional carriers), validates signatures, dedupes, and writes raw events to the `carrier_events_raw` table for downstream processing.

Stateless except for the database. High write throughput. The least exciting service we own when it's working.

> Read [`routebox-platform-docs`](https://github.com/312school/routebox-platform-docs) first if you haven't.

## What this service does

1. HTTP receiver for carrier webhook payloads (`/v1/webhooks/<carrier>`)
2. Validates the carrier-specific signature (HMAC variations per carrier)
3. Parses the payload into a normalized event format
4. Dedupes against the last 24h of events for the same carrier+tracking_number
5. Writes to `carrier_events_raw` in Postgres
6. Returns 200 to the carrier

Downstream services (`shipments-api`, `ops-console`) read `carrier_events_raw` to update shipment statuses.

## Repo layout

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
├── docker-compose.yml         # OUT OF DATE — see notes
├── Jenkinsfile
├── go.mod
├── go.sum
└── README.md
```

## The AWS access key thing

This service does not use the ECS task IAM role to authenticate with AWS. It uses long-lived IAM access keys, injected as environment variables from Secrets Manager.

Yes, this is bad. We know.

The history is in [`routebox-platform-docs/notes/handover.md`](https://github.com/312school/routebox-platform-docs/blob/main/notes/handover.md). Short version: when this service was migrated to ECS, we couldn't get carrier webhook signature verification to work reliably with the task role due to the way our Go AWS SDK version refreshes credentials. The auth header would intermittently arrive in a state that made HMAC verification fail. About 1 in 50 webhooks. Carriers retry, but we'd lose the dedup window and double-count, which broke billing reconciliation.

The "fix" was the long-lived keys. The keys live in Secrets Manager (`routebox/tracking-events/aws-credentials`) and are rotated quarterly via a Jenkins job (`tracking-events-rotate-keys`). The rotation includes a re-deploy to pick up the new values.

**Don't disable the rotation job.** **Don't refactor away the env-var auth path** without spending a week understanding why it exists. The special-case logic in [`routebox-jenkins`](https://github.com/312school/routebox-jenkins)'s `deployToEcs` is what wires the secret into the container.

If you fix this properly — the SDK has changed since we set this up, and the underlying issue may be solvable now — write up what you found. There is a TODO with this service's name on it in the handover.

## Running locally

```
docker compose up
```

The compose file is **out of date** — it references a Postgres image that no longer pulls cleanly and skips the LocalStack setup the AWS auth path requires. Local dev for this service is currently broken. We test against dev AWS instead. Updating the compose file is a TODO that has been a TODO for a long time.

## Deploys

Standard Jenkins flow, but with the special-case secret wiring noted above. The `Jenkinsfile` here imports the shared library and calls `deployToEcs(service: 'tracking-events', env: ...)`, which knows about the access-key injection.

## Database

Writes to `carrier_events_raw`. Reads nothing — this service is genuinely write-only against the DB. The table is heavily indexed for the dedupe lookups; see [`routebox-db-migrations`](https://github.com/312school/routebox-db-migrations) for the schema and migration history.

## Configuration

Environment variables. Interesting ones:

- `DATABASE_URL` — Postgres connection
- `AWS_ACCESS_KEY_ID` / `AWS_SECRET_ACCESS_KEY` — long-lived, see above
- `CARRIER_SIGNATURE_SECRETS` — JSON map of carrier → signing secret, pulled from Secrets Manager
- `DEDUP_WINDOW_HOURS` — default 24
- `MAX_PAYLOAD_BYTES` — default 256KB; carriers occasionally send bigger, those get rejected with a 413

## Known issues

- The long-lived keys (above)
- `docker-compose.yml` rotted (above)
- The DHL carrier handler has a TODO from years ago about handling their v2 webhook format. They never enforced the migration. We're still on v1.
- Dedup is best-effort — Postgres unique constraint on `(carrier, tracking_number, event_type, event_time)` plus `ON CONFLICT DO NOTHING`. In rare races, the same event lands twice. Downstream tolerates it.

For more, read [`routebox-platform-docs/notes/handover.md`](https://github.com/312school/routebox-platform-docs/blob/main/notes/handover.md).
