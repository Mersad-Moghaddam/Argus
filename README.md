# Argus — Production-grade Uptime Monitoring Platform

Argus is a Go-based uptime monitoring service using Hexagonal Architecture (Ports & Adapters), MySQL, Redis/Asynq workers, and an outbox-driven alert integration model.

It covers two bounded contexts:

1. **Website uptime monitoring** — the original single-tenant HTTP/keyword/heartbeat/TLS monitor,
   protected by a global `X-API-Key`.
2. **API route monitoring** — multi-user, project-based monitoring of individual API operations,
   with OpenAPI/Swagger import and its own bearer-token accounts.

The two are additive and independent: they share the process, database and worker runtime, but have
separate tables, separate authentication and separate HTTP route trees.

## Architecture

- `cmd/api` — service entrypoint
- `internal/domain` — aggregates/entities/policies (incl. route health + incident rules)
- `internal/domain/ports` — repository + integration interfaces
- `internal/application` — use-cases/orchestration
- `internal/openapi` — OpenAPI 3.x / Swagger 2.0 parser (JSON+YAML, local-only `$ref` resolution)
- `internal/adapters/inbound` — HTTP/worker inbound adapters (API-key and bearer middleware)
- `internal/adapters/outbound` — MySQL, notifier and other outbound adapters
- `internal/worker` — asynq task handlers and the SSRF-hardened check evaluator
- `internal/platform` — framework/bootstrap/runtime wiring only
- `internal/testsupport` — in-memory port fakes, imported only by tests
- `db/migrations` — versioned SQL migrations (up/down)
- `frontend` — separated UI assets (`app.js` = uptime dashboard, `projects.js` = API Projects)

## Core capabilities

### Website uptime monitoring
- Monitor types: `http_status`, `keyword`, `heartbeat`, `tls_expiry`
- Incident lifecycle + maintenance suppression
- Status pages
- Outbox-driven asynchronous alert dispatch with dedupe
- API key protection for API routes

### Project-based API route monitoring
- Projects with create/edit/archive/delete, search, filter and pagination
- Thousands of API routes per project, added manually, in bulk, or imported
- OpenAPI 3.x and Swagger 2.0 import from file upload or pasted text, with validation,
  preview, per-route selection, conflict detection and a final result report
- Re-import adds new routes and refreshes spec metadata without ever overwriting
  user-defined monitoring settings; routes dropped from a spec are reported and
  disabled on request, never silently deleted
- Persistent background monitoring engine: per-route intervals, timeouts, bounded retries,
  concurrency limits, duplicate-job prevention, rolling metric aggregation and history retention
- Route health states: `healthy`, `degraded`, `failing`, `disabled`, `unknown`
- Automatic incident open after N consecutive failures and resolve after M consecutive successes
- Per-project authorization (owner/editor/viewer); non-members and nonexistent projects are
  indistinguishable (both 404) so project IDs cannot be enumerated

### Security
- SSRF hardening for every outbound check: the address policy runs in the dialer, so it sees the
  resolved IP and is immune to DNS rebinding, and it is re-applied on every redirect hop
- Cloud metadata endpoints, reserved/CGNAT/benchmark ranges, multicast and unspecified addresses are
  always blocked; loopback and private ranges are blocked by default and can be opted into with
  `ROUTE_ALLOW_PRIVATE_TARGETS=true` for internal monitoring
- Scheme allow-list, embedded-credential rejection, redirect cap, response size cap,
  request-body limits, spec size/complexity limits, and no remote `$ref` fetching
- Configured request secrets are masked on every read path and stripped on cross-origin redirects

## Run locally

```bash
docker compose up -d
go run ./cmd/api
```

Open UI at: `http://localhost:8080`

Detailed usage instructions: [`USER_GUIDE.md`](USER_GUIDE.md)

## UI Preview

![Argus dashboard preview](docs/main-page-preview.svg)
