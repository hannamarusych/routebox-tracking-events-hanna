# State Management

## Overview

This service is almost entirely stateless at the application layer. The only state it owns is what it writes to PostgreSQL.

## What State Exists

- **carrier_events_raw**: the single table this service writes to. Stores the normalized event, the originating carrier, tracking number, event type, event time, and raw payload.
- No in-memory caching of carrier data between requests.
- No session state; each webhook request is handled independently.

## Deduplication as State Management

Because this service can receive the same webhook more than once (carriers retry on timeout or non-200 responses), deduplication is a core state-management concern:

- A Postgres unique constraint on (`carrier`, `tracking_number`, `event_type`, `event_time`) combined with `ON CONFLICT DO NOTHING` prevents duplicate rows.
- A rolling 24-hour dedup window (`DEDUP_WINDOW_HOURS`) is used for additional application-level checks before insert.
- This is best-effort, not exactly-once. Rare races can allow the same event to land twice; downstream consumers are expected to tolerate this.

## Schema Ownership

The schema for `carrier_events_raw` and its indexes live in `routebox-db-migrations`, not in this repository. This service only reads its own configuration; it does not manage migrations.

## Local State

Local development against a real database is currently broken (see [TROUBLESHOOTING.md](./TROUBLESHOOTING.md)); development and testing against the dev AWS environment is the current workaround.
