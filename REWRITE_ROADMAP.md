# Argus Rewrite Roadmap (From Scratch)

This roadmap is a practical, step-by-step plan to rebuild Argus cleanly while keeping production reliability as the top priority.

---

## 0) Define scope, success criteria, and constraints (Day 0)

### Goals
- Rebuild Argus with **Hexagonal Architecture**:
  - `internal/domain`
  - `internal/domain/ports`
  - `internal/application`
  - `internal/adapters/inbound`
  - `internal/adapters/outbound`
  - `internal/platform`
- Preserve and improve current product capabilities (monitors, incidents, maintenance, status pages, alerts, heartbeat).
- Add production-grade reliability (outbox, idempotency, migrations, observability, security).

### Deliverables
- Architecture Decision Records (ADRs) for:
  - domain boundaries
  - outbox event contracts
  - auth model
  - SSRF policy
  - CQRS-lite read model approach

---

## 1) Create clean project skeleton (Day 1)

1. Initialize folder structure and package boundaries.
2. Add lint/test/format tooling and CI workflow.
3. Add local dev environment (`docker-compose` for MySQL, Redis, optional Loki/Prometheus/Tempo).
4. Add migration tool and migration runner CLI:
   - choose one: `goose` / `atlas` / `migrate`
5. Add Makefile tasks:
   - `make dev`
   - `make test`
   - `make lint`
   - `make migrate-up`
   - `make migrate-down`

**Exit criteria:** skeleton builds and CI passes with empty app.

---

## 2) Domain-first implementation (Days 2–4)

Implement domain modules with no framework dependencies.

### Modules to implement
- `monitor`
- `incident`
- `maintenance`
- `alerting`
- `statuspage`

### For each module
1. Entities/aggregates/value objects.
2. Domain invariants and policies.
3. State transitions as pure functions/methods.
4. Domain errors (typed).

### Mandatory domain policies
- Monitor type validation/normalization.
- Incident state machine (open → acknowledged → resolved).
- Maintenance suppression rules.

**Exit criteria:** strong unit test coverage for all policies/state transitions.

---

## 3) Ports and use-cases (Days 5–7)

### Define ports (`internal/domain/ports`)
- `MonitorStore`
- `IncidentStore`
- `MaintenanceStore`
- `StatusPageStore`
- `AlertChannelStore`
- `OutboxStore`
- `Notifier`
- `Clock`
- `TxManager` (for transactional boundaries)

### Implement use-cases (`internal/application`)
- `CreateMonitor`
- `ProcessCheckResult`
- `ReceiveHeartbeat`
- `OpenIncident`
- `ResolveIncident`

### Rules
- Use-cases orchestrate; domain enforces invariants.
- All write-side incident/alert side effects emit outbox events inside transaction.

**Exit criteria:** use-cases tested with mocked ports.

---

## 4) Data model & migrations (Days 8–9)

1. Create baseline migration `0001` with all core tables.
2. Create separate migration(s) for outbox and read model projections.
3. Add down migrations for rollback.
4. Add migration smoke test in CI.

### Required tables
- `websites`
- `website_checks`
- `incidents`
- `maintenance_windows`
- `alert_channels`
- `status_pages`
- `outbox_events`
- `incident_timeline` (audit trail)

### Outbox requirements
- Unique `dedupe_key`
- retry metadata (`retry_count`, `last_error`, `last_attempted_at`)
- status and timestamps

**Exit criteria:** migrations up/down work on clean DB and existing DB clone.

---

## 5) Outbound adapters (Days 10–12)

### MySQL adapters
- Implement each bounded-context store separately.
- Keep write operations transactional.
- Add CQRS-lite read repositories optimized for dashboard/status APIs.

### Asynq adapters
- Producer for check scheduling/outbox dispatch.
- Consumer handlers for checks and outbox dispatcher.
- Idempotency checks in consumer logic.

### Notifier adapters
- Slack
- Webhook
- Email (provider-backed)

**Exit criteria:** integration tests with ephemeral MySQL/Redis pass.

---

## 6) Inbound adapters (Days 13–14)

### HTTP API
- Handlers call application use-cases only.
- Input validation and mapping kept in adapter layer.
- Pagination/filter/sort for large collections.

### Worker handlers
- `enqueue_due_checks`
- `check_website`
- `dispatch_outbox`

### AuthN/AuthZ
- API key or JWT authentication.
- Role-based authorization on admin endpoints.

**Exit criteria:** contract tests for handlers and payloads pass.

---

## 7) Security hardening (Days 15–16)

1. SSRF protections:
   - deny private CIDRs
   - block metadata endpoints
   - DNS rebinding guard
   - enforce outbound allow/deny policy
2. Secrets management integration:
   - Vault/SSM/KMS in production profile
3. Add secure defaults:
   - timeout budgets
   - max payload size
   - strict TLS settings

**Exit criteria:** security checklist and SSRF tests pass.

---

## 8) Reliability & scalability (Days 17–18)

1. Batching + cursor pagination for due checks.
2. Worker concurrency and queue weights configurable by env.
3. DB pool tuning exposed by config and monitored.
4. Keyword checks using streaming scan and content-type gate.
5. Outbox retry policy + dead-letter strategy.

**Exit criteria:** load test baseline established; no unbounded queries.

---

## 9) Observability & ops maturity (Days 19–20)

1. OpenTelemetry instrumentation (traces + metrics + logs correlation IDs).
2. Exporters:
   - Prometheus metrics
   - Loki/ELK logs
   - Tempo/Jaeger traces
3. SLO dashboards:
   - uptime %
   - MTTR
   - MTBF
4. Incident timeline and audit trail APIs.

**Exit criteria:** dashboards operational in staging.

---

## 10) Frontend rewrite (parallel track)

1. Replace single HTML page with componentized app (React/Vue/Svelte).
2. Generate typed API client from OpenAPI.
3. Add UX features:
   - pagination/filter/sort
   - incident timeline views
   - maintenance calendar
   - status page management

**Exit criteria:** frontend test suite and accessibility checks pass.

---

## 11) Test strategy (continuous)

### Required test layers
- **Unit tests**: domain policies + monitor evaluators (http/keyword/heartbeat/tls).
- **Integration tests**: MySQL repositories with ephemeral DB.
- **Worker flow tests**: mocked ports + idempotency scenarios.
- **Contract tests**: HTTP handlers and public status page payloads.
- **Migration tests**: up/down and data compatibility.

### Quality gates
- Minimum coverage threshold (e.g., 80% domain/application).
- CI blocks merge on lint/test/migration failures.

---

## 12) Rollout strategy (final)

1. Deploy rewritten service in shadow mode.
2. Dual-write/dual-read where necessary for safe migration.
3. Compare key metrics between old/new:
   - check throughput
   - incident correctness
   - alert delivery success
4. Gradual cutover with rollback plan.

**Exit criteria:** production cutover completed with no data loss and acceptable SLOs.

---

## Suggested milestone plan

- **Milestone 1:** Foundation + Domain + Ports
- **Milestone 2:** Use-cases + Migrations + MySQL adapters
- **Milestone 3:** Workers + Outbox + Notifiers
- **Milestone 4:** HTTP API + Auth + SSRF hardening
- **Milestone 5:** Observability + Frontend + Rollout

---

## First 7 tasks to start immediately

1. Freeze current API contract and write OpenAPI spec.
2. Add ADRs for architecture and outbox semantics.
3. Set up migration tool and create baseline migration.
4. Implement `monitor` + `incident` domain modules with tests.
5. Define all domain ports and use-case interfaces.
6. Build `CreateMonitor` and `ProcessCheckResult` use-cases.
7. Implement outbox table + dispatcher worker with idempotent dedupe.
