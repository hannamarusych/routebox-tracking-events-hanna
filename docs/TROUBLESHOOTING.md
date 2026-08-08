# Troubleshooting

## AWS Authentication: Long-Lived Access Keys

This service authenticates to AWS using long-lived IAM access keys, injected as environment variables from Secrets Manager, rather than using the ECS task role. This is a known, intentional tradeoff rather than an oversight.

**Background:** when this service was migrated to ECS, carrier webhook signature verification would intermittently fail (roughly 1 in 50 webhooks) because of how the Go AWS SDK version in use at the time refreshed task-role credentials. The auth header would occasionally arrive mid-refresh in a state that broke HMAC verification. Carriers retry on failure, but retries fell outside the dedup window, which caused duplicate events and broke billing reconciliation downstream.

**Current mitigation:** long-lived keys, stored in Secrets Manager at `routebox/tracking-events/aws-credentials`, rotated quarterly via the `tracking-events-rotate-keys` Jenkins job. Rotation includes a redeploy so running tasks pick up the new values.

**Guidance:**
- Do not disable the rotation job.
- Do not remove the environment-variable auth path without first understanding why it exists — see the note above.
- The AWS SDK for Go has changed since this workaround was introduced. If the underlying credential-refresh issue has been fixed, revisiting this is worthwhile. Document any findings if you investigate.

## Local Development Is Broken

`docker-compose.yml` references a Postgres image that no longer pulls cleanly and does not include the LocalStack setup the AWS auth path requires. As a result, local development against this service currently does not work out of the box. The current workaround is to develop and test against the dev AWS environment instead. Updating the compose file is a known open item — see [ROADMAP.md](../ROADMAP.md).

## DHL Webhook Format

The DHL carrier handler has not been updated to DHL's v2 webhook format. DHL has not enforced the migration, so the service remains on v1. This should be revisited before DHL deprecates v1 support.

## Duplicate Events Downstream

Deduplication is best-effort, not exactly-once. In rare race conditions the same event can be written twice. Downstream consumers (`routebox-shipments-api-hanna`, `routebox-ops-console-hanna`) are expected to tolerate occasional duplicates rather than this service guaranteeing exactly-once delivery.

## Oversized Payloads

Carriers occasionally send payloads larger than `MAX_PAYLOAD_BYTES` (default 256KB). These are rejected with a 413. If a specific carrier is consistently hitting this limit, consider raising the limit for that carrier rather than globally.
