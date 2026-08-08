# Roadmap

## Current State (Sprint 4)

- Core webhook ingestion pipeline for UPS, FedEx, DHL, and USPS is in place and running in production.
- Deduplication against a rolling 24-hour window is implemented at the database layer.
- Documentation brought up to the RouteBox platform standard: architecture, deployment, state management, and troubleshooting guides added.

## In Progress

- Investigating whether the AWS SDK for Go has resolved the credential-refresh issue that originally forced this service onto long-lived IAM keys.
- Reviewing options for replacing the out-of-date local `docker-compose.yml` setup.

## Planned

- Add DHL v2 webhook format support.
- Add service-level dashboards for webhook validation error rates and dedupe collisions.
- Revisit local development experience so it does not depend on the shared dev AWS environment.

## Longer Term

- Migrate authentication back to the ECS task role once the SDK issue is confirmed fixed, and retire the long-lived access keys entirely.
- Explore an event-stream-based handoff (rather than direct table reads) to downstream services as the platform's event volume grows.
- Contribute this service's context to the future `routebox-platform` documentation repository, including its role in the platform's service catalog and architecture decision records.
