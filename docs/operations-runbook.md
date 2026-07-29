# Operations runbook

This runbook uses bounded diagnostics and operational signals only. Never paste
enrollment tokens, OTLP credentials, webhook URLs, request headers, or raw
telemetry into tickets or logs.

## OTLP ingestion outage

1. Confirm the Collector can reach the configured Argus HTTP origin or the
   TLS-protected gRPC listener.
2. Check the project **Telemetry signals** card for the last accepted record
   and the Collector exporter queue/retry telemetry.
3. Confirm the credential is active and scoped for the intended signal.
4. For gRPC, verify `OTLP_GRPC_ADDR`, TLS termination, and network policy.
5. Rotate a credential only after ruling out pipeline/network failure; the old
   secret cannot be recovered after the one-time display closes.

## Exporter failure, queue, or buffer pressure

The Collector owns retry queues and any persistent WAL/buffer. Inspect queue
depth, retry count, storage capacity, and exporter error rate before changing
batch sizes. Restore downstream connectivity first; do not disable retries or
discard buffered telemetry to make an alert disappear. Scale collector storage
or reduce telemetry volume deliberately when the buffer has a sustained
backlog.

## High cardinality or quota rejection

Argus retains only bounded telemetry diagnostics and translates only its fixed
HTTP duration metric labels. Investigate new service/deployment identities or
unbounded application labels at the source. For `429`/`ResourceExhausted`,
reduce export volume or intentionally adjust the credential rate limit; do not
create duplicate credentials to evade a quota.

## SLO evaluation lag or no data

Check VictoriaMetrics availability and query latency, then compare the latest
telemetry signal timestamp with the SLO window and stale threshold. `no data`,
`stale`, maintenance, and configuration errors are not healthy states. Confirm
the service identity has a project-scoped route mapping before changing the
SLO definition.

## Synthetic overload

Inspect scheduler lag, project/global request budgets, concurrency leases, and
recorded synthetic skips. Budget shedding deliberately defers checks and must
not be converted into failure incidents. Lower selected check frequency or
disable nonessential synthetics before raising global ceilings.

## Private agent offline or assignment failure

Confirm the agent can make outbound HTTPS requests to Argus, its enrollment
token is not revoked, and its signed configuration public key is current.
Check the dashboard for healthy/stale/offline state and the assignment-scoped
incident. Assignment results must use an active assignment in the agent's
exact environment; redirects and non-2xx responses are failures by design.
Revoke compromised agents or assignments, then re-enroll rather than trying to
recover a displayed token.

## Notification backlog

Inspect transactional-outbox retry attempts and delivery failures. Ensure the
configured destination is available and its integration secret remains valid.
Do not delete underlying incidents to suppress retries; correct the delivery
target or temporarily use an explicit maintenance window.

## Migration failure or rollback

Stop new deploys, preserve the migration error and database backup, and assess
the last successfully applied migration. Migrations are ordered and have a
paired down migration, but rollback is an operator decision: take a verified
backup and test it against a copy first. For route-secret migration, run
`migrate-route-secrets -dry-run` before any write or key rotation operation.
Never remove an old encryption key until its durable rotation checkpoint shows
no records remain on it.
