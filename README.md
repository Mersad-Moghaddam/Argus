<div align="center">
  <img src="docs/argus-hero.svg" width="100%" alt="Argus — the all-seeing, self-hosted uptime control center" />
</div>

<div align="center">

[![Go](https://img.shields.io/badge/Go-1.26.5-00ADD8?style=flat-square&logo=go&logoColor=white)](go.mod)
[![MySQL](https://img.shields.io/badge/MySQL-8.4-4479A1?style=flat-square&logo=mysql&logoColor=white)](docker-compose.yml)
[![Redis](https://img.shields.io/badge/Redis-8.8-DC382D?style=flat-square&logo=redis&logoColor=white)](docker-compose.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-F5C451?style=flat-square)](LICENSE)
[![PRs welcome](https://img.shields.io/badge/PRs-welcome-3DDC97?style=flat-square)](CONTRIBUTING.md)

**A fast, self-hosted uptime monitor and incident control center built in Go.**

`PANOPTES MODE: ON` · Some eyes may rest. The watch never does.

[Quick start](#quick-start) · [Features](#what-argus-watches) · [API](#http-api) · [Architecture](#architecture) · [Roadmap](#project-status-and-roadmap) · [Contributing](CONTRIBUTING.md)

</div>

---

Argus watches websites, health endpoints, expected content, heartbeat signals, and TLS certificates. It records every check, opens and resolves incidents, suppresses noise during maintenance, and dispatches alerts asynchronously—all from a responsive dashboard you can run on your own infrastructure.

The name honors [Argus Panoptes](https://www.theoi.com/Gigante/GiganteArgosPanoptes.html)—the many-eyed, “all-seeing” guardian of Greek myth. Some of his eyes could sleep while others kept watch; after his death, Hera preserved them in the peacock’s tail. Argus turns that ancient image of continuous vigilance into a modern monitoring system.

> [!IMPORTANT]
> The website-monitoring workflow is the original production core. The newer **Projects + OpenAPI route monitoring** subsystem now also includes a background evaluator, aggregate metrics, project UI, and import wizard. A 2026 architecture review found that probing every imported route—especially state-changing methods—is not a safe or scalable default. See [Monitoring v2 design and delivery plan](#monitoring-v2-design-and-delivery-plan).

## Monitoring v2 design and delivery plan

The current route-monitoring implementation remains documented in
[PROJECT_MONITORING_PLAN.md](PROJECT_MONITORING_PLAN.md). The proposed next
generation uses passive OpenTelemetry data for broad endpoint coverage and
keeps active synthetic requests limited to explicit, safe canaries.

- [Transformation index](docs/TRANSFORMATION_INDEX.fa.md)
- [Product, UI/UX, accessibility, and user-flow audit](docs/reviews/2026-07-28-product-ux-audit.fa.md)
- [Monitoring v2 architecture decision](docs/architecture/MONITORING_V2_ADR.fa.md)
- [URL and route normalization specification](docs/architecture/URL_ROUTE_NORMALIZATION_SPEC.fa.md)
- [Scrum product backlog and sprint forecast](docs/planning/MONITORING_V2_SCRUM_PLAN.fa.md)
- [Security best-practices report](security_best_practices_report.md)
- [Repository threat model](Argus-threat-model.md)
- [Motion audit and implementation plans](animation-plans/README.md)

## Why Argus?

- **Four monitoring modes** — HTTP status, response keyword, heartbeat, and TLS expiry.
- **Incident-aware by design** — failures open incidents; recoveries resolve them automatically.
- **Reliable alert delivery** — a transactional outbox decouples checks from webhook and Slack-compatible notifications.
- **Maintenance without blindness** — checks continue while alerts are muted for planned work.
- **Public status views** — group monitors into shareable status pages.
- **Operationally simple** — one Go process, MySQL for durable state, Redis/Asynq for scheduled work.
- **Secure defaults** — outbound target validation, metadata/private-network blocking, request body limits, security headers, hashed bearer tokens, and local-only OpenAPI reference resolution.
- **No frontend toolchain** — the dashboard is accessible, responsive vanilla HTML, CSS, and JavaScript served by the API process.

## Dashboard

The control center includes live health totals, searchable and sortable monitors, incident history, maintenance scheduling, alert channels, public status pages, ping history, toast feedback, light/dark themes, and a 30-second auto-refresh loop.

## What Argus watches

| Monitor | What it does | Healthy when |
| --- | --- | --- |
| `http_status` | Sends an HTTP `GET` to the target or optional health-check URL. | Response is `2xx` or `3xx`. |
| `keyword` | Reads up to 1 MiB of text/JSON response content. | The configured, case-sensitive keyword is present. |
| `heartbeat` | Waits for an external process to ping Argus. | The latest heartbeat is inside its grace period. |
| `tls_expiry` | Opens a TLS connection and inspects the leaf certificate. | Expiry is beyond the configured threshold (14 days by default). |

All monitor intervals must be at least 10 seconds. HTTP checks time out after 5 seconds; keyword checks after 6 seconds; TLS handshakes after 5 seconds.

<div align="center">
  <img src="docs/argus-watch-methods.svg" width="920" alt="Four animated mythic instruments representing Argus HTTP, keyword, heartbeat, and TLS monitoring" />
</div>

<div align="center">
  <img src="docs/monitoring-flow.svg" width="920" alt="Animated Argus monitoring flow from scheduler through checks and incidents to alert delivery" />
</div>

### Incident and alert lifecycle

1. Asynq Scheduler periodically discovers due monitors.
2. Each monitor becomes a deduplicated job in the `critical` queue.
3. A bounded worker pool evaluates the target and persists its result.
4. A transition to `down` opens an incident; a transition to `up` resolves it.
5. When alerts are not suppressed by an active maintenance window, Argus writes a deduplicated outbox event.
6. The dispatcher delivers pending events to enabled webhook and Slack-compatible channels.

Email is represented in the data model and dashboard, but a delivery adapter has not been implemented yet.

## Quick start

### Prerequisites

- [Go 1.26.5](https://go.dev/doc/install) or a compatible newer release
- [Docker](https://docs.docker.com/get-docker/) with Docker Compose v2
- Free local ports `3306`, `6379`, and `8080`

### 1. Clone and start the data services

```bash
git clone https://github.com/Mersad-Moghaddam/Argus.git
cd Argus
docker compose up -d
docker compose ps
```

This starts MySQL 8.4 and Redis 8.8 with persistent named volumes.

### 2. Start Argus

```bash
go run ./cmd/api
```

The application connects to MySQL and Redis, applies every `*.up.sql` migration in lexical order, starts the scheduler and workers, and serves the dashboard at [http://localhost:8080](http://localhost:8080).

### 3. Create your first monitor

Use the **Add monitor** form in the dashboard, or call the API:

```bash
curl --request POST http://localhost:8080/api/websites \
  --header 'Content-Type: application/json' \
  --data '{
    "url": "https://example.com",
    "checkInterval": 30,
    "monitorType": "http_status"
  }'
```

If you configure `API_KEY`, also send `--header 'X-API-Key: your-secret-key'`. The dashboard can store that key in browser local storage for subsequent requests.

### Stop or reset

```bash
# Stop containers but keep monitoring data
docker compose down

# Remove containers and their MySQL/Redis volumes
docker compose down --volumes
```

> [!CAUTION]
> The second command permanently deletes local Argus data.

## Configuration

Argus reads environment variables and automatically loads a local `.env` file when present.

| Variable | Default | Purpose |
| --- | --- | --- |
| `HTTP_ADDR` | `:8080` | API and dashboard listen address. |
| `MYSQL_DSN` | `argus:argus@tcp(localhost:3306)/argus?parseTime=true` | Go MySQL driver DSN. Keep `parseTime=true`. |
| `REDIS_ADDR` | `localhost:6379` | Redis address used by Asynq. |
| `REDIS_PASSWORD` | empty | Redis password. |
| `REDIS_DB` | `0` | Redis logical database index. |
| `API_KEY` | empty | Optional `X-API-Key` required by the website-monitoring API. |
| `SCHEDULER_INTERVAL` | `30s` | How often Argus scans for due website checks. |
| `WORKER_CONCURRENCY` | `10` | Maximum concurrent Asynq jobs. |
| `QUEUE_CRITICAL_WEIGHT` | `6` | Relative capacity for monitor-check jobs. |
| `QUEUE_DEFAULT_WEIGHT` | `4` | Relative capacity for scheduling and outbox jobs. |
| `DUE_CHECK_BATCH_SIZE` | `200` | Reserved due-check batch setting; the current website processor uses batches of 200. |
| `DB_MAX_OPEN_CONNS` | `25` | Maximum open MySQL connections. |
| `DB_MAX_IDLE_CONNS` | `25` | Maximum idle MySQL connections. |
| `DB_CONN_MAX_LIFETIME` | `5m` | Maximum lifetime of a pooled connection. |

Example production-oriented `.env`:

```dotenv
HTTP_ADDR=:8080
MYSQL_DSN=argus:replace-me@tcp(mysql:3306)/argus?parseTime=true
REDIS_ADDR=redis:6379
REDIS_PASSWORD=replace-me
API_KEY=replace-with-a-long-random-secret
WORKER_CONCURRENCY=20
DB_MAX_OPEN_CONNS=40
```

Invalid integer or duration values fall back to defaults; an invalid `REDIS_DB` prevents startup. Go duration syntax such as `30s`, `5m`, or `1h` is accepted.

## HTTP API

Responses are JSON unless an endpoint returns `204 No Content`. The website API uses optional global API-key auth; the project API uses its own bearer-token sessions.

### Website monitoring API

When `API_KEY` is non-empty, include `X-API-Key: <key>` on every endpoint below.

| Method | Endpoint | Purpose |
| --- | --- | --- |
| `GET` | `/api/websites?limit=100&offset=0` | List monitors. |
| `POST` | `/api/websites` | Create a monitor. |
| `DELETE` | `/api/websites/:id` | Delete a monitor and its dependent history. |
| `POST` | `/api/websites/:id/heartbeat` | Record a heartbeat. |
| `GET` | `/api/checks?limit=100` | Read recent check history. |
| `GET` | `/api/incidents?limit=100&offset=0` | Read incident history. |
| `POST` | `/api/alert-channels` | Create a webhook, Slack, or email channel. |
| `POST` | `/api/maintenance-windows` | Schedule alert suppression globally or for one monitor. |
| `GET` | `/api/status-pages?limit=100&offset=0` | List status pages. |
| `POST` | `/api/status-pages` | Create a status page. |
| `GET` | `/api/public/status/:slug` | Read a public status page payload. |
| `GET` | `/api/logs` | Read the in-memory operational log buffer. |

See the step-by-step [User Guide](USER_GUIDE.md) for request bodies and daily workflows.

### Projects and OpenAPI route API

This API is an **in-progress foundation** for multi-project route monitoring. Registration and login return an opaque token valid for 30 days. Argus stores only its SHA-256 hash.

```bash
# Register
curl --request POST http://localhost:8080/api/auth/register \
  --header 'Content-Type: application/json' \
  --data '{"email":"operator@example.com","password":"change-me-now","name":"Operator"}'

# Use the returned token
curl http://localhost:8080/api/projects \
  --header 'Authorization: Bearer <token>'
```

| Area | Endpoints |
| --- | --- |
| Auth | `POST /api/auth/register`, `POST /api/auth/login`, `POST /api/auth/logout`, `GET /api/auth/me` |
| Projects | `GET/POST /api/projects`, `GET/PUT/DELETE /api/projects/:projectId`, archive and unarchive actions |
| Routes | Project-scoped list, create, update, delete, enable/disable, bulk create/delete, check history, and incidents |
| Imports | Validate, preview, and commit OpenAPI 3.x or Swagger 2.0 JSON/YAML documents up to 10 MiB |

OpenAPI imports resolve only local `$ref` pointers—Argus does not fetch external references. Import commits preserve user-owned monitoring configuration and disable, rather than delete, explicitly selected routes that disappeared from a specification.

## Architecture

Argus follows a ports-and-adapters layout: business rules point inward, while HTTP, MySQL, Redis/Asynq, and outbound notifications remain replaceable edge concerns.

<div align="center">
  <img src="docs/argus-architecture.svg" width="920" alt="Animated Greek-temple diagram of Argus ports-and-adapters architecture" />
</div>

```text
Browser / API clients
        │
        ▼
Fiber HTTP adapter ─────────────── Static dashboard
        │
        ▼
Application services ──────────── OpenAPI parser
        │
        ▼
Domain policies + ports
        │
        ├──────── MySQL adapters ─── durable state + outbox
        ├──────── Asynq workers ──── Redis scheduling/queues
        └──────── HTTP notifier ──── webhooks / Slack
```

| Path | Responsibility |
| --- | --- |
| `cmd/api` | Process entrypoint and graceful shutdown. |
| `internal/domain` | Pure validation, health-state, and incident policies. |
| `internal/domain/ports` | Storage and integration contracts. |
| `internal/application` | Use cases for monitors, incidents, auth, projects, routes, and imports. |
| `internal/adapters/inbound` | Fiber authentication middleware. |
| `internal/adapters/outbound` | MySQL repositories and outbound notification delivery. |
| `internal/api` | HTTP handlers and endpoint registration. |
| `internal/openapi` | Bounded OpenAPI 3.x / Swagger 2.0 JSON/YAML parsing and local reference resolution. |
| `internal/platform` | MySQL, migrations, Fiber, Redis, and Asynq runtime wiring. |
| `internal/worker` | Scheduled website evaluation and outbox processing. |
| `db/migrations` | Ordered, additive MySQL schema migrations. |
| `frontend` | Dependency-free dashboard assets. |

## Data and migrations

- MySQL persists monitors, checks, incidents, maintenance windows, status pages, alert channels, the outbox, users, projects, API routes, route checks/incidents, and import jobs.
- Redis contains Asynq queues and scheduler state; it is not the source of truth for monitoring history.
- Startup applies all `db/migrations/*.up.sql` files in filename order.
- Duplicate-column error `1060` is treated as an idempotent compatibility case.
- Down migrations exist for controlled development rollback, but startup never applies them automatically.

Named Docker volumes retain both MySQL and Redis state across container restarts.

## Security model

Argus is intended to monitor public network targets. Before direct HTTP and keyword checks, it resolves the hostname and blocks loopback, RFC1918/private, link-local, and cloud metadata addresses. Additional safeguards include:

- optional global API-key protection through Fiber middleware;
- independent bearer auth for project APIs, bcrypt password hashing, hashed session tokens, and role-aware project authorization;
- generic `404` responses for unauthorized/nonexistent projects to reduce enumeration;
- a 15 MiB HTTP body limit and 10 MiB OpenAPI document limit;
- local-only `$ref` resolution and YAML structural limits;
- 1 MiB response read caps for HTTP content checks;
- Fiber recovery, Helmet security headers, ETags, and response compression.

> [!WARNING]
> Treat Argus as privileged infrastructure. Put it behind TLS and a trusted reverse proxy, set `API_KEY`, use strong MySQL/Redis credentials, restrict database ports, and review outbound-network policy before exposing it publicly. The newer route evaluator revalidates redirect hops and resolved addresses; the legacy website checker still uses a less-hardened client and should not be treated as equivalent.

Please report vulnerabilities privately according to [SECURITY.md](SECURITY.md).

## Development

```bash
# Run all unit tests
go test ./...

# Run race detection
go test -race ./...

# Static analysis
go vet ./...

# Optional linting (requires revive)
revive -config revive.toml ./...
```

The current suite covers domain policies, migration error handling, OpenAPI/Swagger parsing and reference resolution, large/malformed document limits, and metadata-target blocking.

### Useful local checks

```bash
gofmt -w $(git ls-files '*.go')
go build ./...
docker compose logs -f mysql redis
```

## Production checklist

- [ ] Set a long, random `API_KEY`.
- [ ] Replace all example MySQL and Redis credentials.
- [ ] Terminate TLS at a reverse proxy and restrict trusted inbound traffic.
- [ ] Do not expose MySQL (`3306`) or Redis (`6379`) to the public internet.
- [ ] Use durable, backed-up storage for MySQL.
- [ ] Tune worker concurrency, queue weights, and DB pooling for your workload.
- [ ] Add process supervision, health probes, centralized logs, and metrics.
- [ ] Pin and regularly update container images and Go dependencies.
- [ ] Test restore, migration, incident, maintenance, and alert-delivery workflows.

## Project status and roadmap

| Capability | Status |
| --- | --- |
| Website/keyword/heartbeat/TLS monitoring | Available |
| Check history, incidents, maintenance, status pages | Available |
| Webhook and Slack-compatible outbox delivery | Available |
| Responsive website-monitoring dashboard | Available |
| Project users, roles, CRUD, route CRUD | Available |
| OpenAPI 3.x / Swagger 2.0 import preview and commit | Available |
| Project route-check worker and aggregate metrics | Available; redesign proposed |
| Project dashboard and import wizard | Available; UX redesign proposed |
| Monitoring v2 telemetry, SLOs, and budgeted canaries | Proposed |
| Email delivery adapter | Planned |
| Expanded integration/E2E coverage and observability | Planned |

The implementation history lives in [PROJECT_MONITORING_PLAN.md](PROJECT_MONITORING_PLAN.md). The new product and architecture package starts at [docs/TRANSFORMATION_INDEX.fa.md](docs/TRANSFORMATION_INDEX.fa.md). Broader modernization notes remain in [REWRITE_ROADMAP.md](REWRITE_ROADMAP.md).

## Contributing

Issues, focused pull requests, tests, documentation, and design discussion are welcome. Start with [CONTRIBUTING.md](CONTRIBUTING.md), follow the [Code of Conduct](CODE_OF_CONDUCT.md), and check the roadmap before beginning a large change.

When proposing a feature, explain the operational problem it solves, its failure modes, and how it preserves the domain/ports boundary.

## Support

- Read the [User Guide](USER_GUIDE.md).
- Search or open a [GitHub issue](https://github.com/Mersad-Moghaddam/Argus/issues).
- Use [GitHub Discussions](https://github.com/Mersad-Moghaddam/Argus/discussions) for ideas and architecture questions when enabled.
- Follow [SECURITY.md](SECURITY.md) for sensitive reports.

## License

Argus is available under the [MIT License](LICENSE).

## Acknowledgements

Argus is powered by [Fiber](https://gofiber.io/), [Asynq](https://github.com/hibiken/asynq), [go-sql-driver/mysql](https://github.com/go-sql-driver/mysql), [go-redis](https://github.com/redis/go-redis), and the wider Go open-source ecosystem.

<div align="center">
  <sub>Built for operators who prefer quiet systems—and fast answers when they are not.</sub>
</div>
