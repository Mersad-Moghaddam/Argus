# Argus — Production-grade Uptime Monitoring Platform

Argus is a Go-based uptime monitoring service using Hexagonal Architecture (Ports & Adapters), MySQL, Redis/Asynq workers, and an outbox-driven alert integration model.

## Architecture (new)

- `cmd/api` — service entrypoint
- `internal/domain` — aggregates/entities/policies
- `internal/domain/ports` — repository + integration interfaces
- `internal/application` — use-cases/orchestration
- `internal/adapters/inbound` — HTTP/worker inbound adapters
- `internal/adapters/outbound` — MySQL, notifier and other outbound adapters
- `internal/platform` — framework/bootstrap/runtime wiring only
- `internal/openapi` — bounded JSON/YAML parsing, local reference resolution, route extraction
- `internal/worker` — persistent website and API-route monitoring tasks
- `db/migrations` — versioned SQL migrations (up/down)
- `frontend` — separated UI assets

## Core capabilities

- Monitor types: `http_status`, `keyword`, `heartbeat`, `tls_expiry`
- Incident lifecycle + maintenance suppression
- Status pages
- Outbox-driven asynchronous alert dispatch with dedupe
- API key protection for API routes
- SSRF hardening for outbound checks
- Multi-user project monitoring with owner/editor/viewer authorization
- OpenAPI 3.x and Swagger 2.0 validate-preview-select import and safe re-import
- Persistent route checks with retries, timeouts, incident thresholds, aggregation, and retention
- Paginated project and route dashboards designed for large API portfolios

## Run locally

```bash
docker compose up -d
go run ./cmd/api
```

Open UI at: `http://localhost:8080`

Detailed usage instructions: [`USER_GUIDE.md`](USER_GUIDE.md)

## UI Preview

![Argus dashboard preview](docs/main-page-preview.svg)
