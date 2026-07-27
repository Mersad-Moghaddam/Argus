# Project-Based API Monitoring — Implementation Plan

This document tracks the full scope of work needed to extend Argus (currently
a single-tenant website/heartbeat/TLS uptime monitor) into a multi-project API
route monitoring platform, without breaking existing behavior. It exists so
the work can proceed section-by-section, with a commit after each completed
section.

## Architectural approach

Follow the existing hexagonal layout exactly:

- `internal/domain` — pure policy/state functions (route health states,
  incident open/resolve rules, path/method normalization, import diffing).
- `internal/domain/ports` — new repository interfaces (`ProjectStore`,
  `RouteStore`, `RouteCheckStore`, `RouteIncidentStore`, `ImportStore`,
  `UserStore`, `AuthTokenStore`).
- `internal/application` — new use-case methods added to (or alongside) the
  existing `Service` type: auth, project CRUD, route CRUD/bulk, import
  validate/commit, monitoring/aggregation helpers.
- `internal/adapters/outbound/mysql` — new files implementing the new ports
  against MySQL, split by aggregate for readability.
- `internal/adapters/inbound/http` — new `BearerAuth` + `ProjectAuthz`
  middleware, added alongside the existing `APIKeyAuth`.
- `internal/api` — new handlers: `auth_handler.go`, `project_handler.go`,
  `route_handler.go`, `import_handler.go`.
- `internal/openapi` — new package: OpenAPI 3.x / Swagger 2.0 JSON+YAML
  parser, local `$ref` resolution, size limits, route extraction.
- `internal/worker` — new asynq tasks for route checks, metric aggregation,
  and check-history retention, reusing/extending the existing SSRF-safe HTTP
  check logic.
- `db/migrations` — additive migrations only (`0004`, `0005`, ...); existing
  tables/columns are untouched.
- `frontend` — extend the existing vanilla JS/CSS single-page app with a new
  "Projects" tab, its own mini view-router, and reusable components (route
  table, import wizard, charts), matching existing style conventions
  (toasts, modals, skeleton loaders, `api()` fetch helper).

Existing website/heartbeat/TLS monitoring, its API routes, DB tables and the
single global `X-API-Key` protection remain fully intact and untouched. The
new project/route subsystem is additive and uses its own bearer-token user
auth, mounted under `/api/projects...` and `/api/auth...`.

## Data model (new tables)

1. `users`, `auth_tokens` — email/password accounts + opaque bearer tokens.
2. `projects` — name/slug/description/status(active|archived), per-project
   default interval/timeout/retries/failure-threshold, cached aggregate
   metrics columns (routes_total/healthy/degraded/failing/disabled,
   uptime_24h_pct, avg_latency_24h_ms, open_incidents, last_check_at).
3. `project_members` — owner/editor/viewer roles for authorization.
4. `api_routes` — one row per monitored operation (method+path+base URL),
   OpenAPI metadata (tags, parameters, request body, responses, security,
   deprecated, operationId) stored as JSON columns, monitoring config
   (interval/timeout/retries/thresholds/enabled), live health/aggregate
   columns, `UNIQUE(project_id, method, path)`.
5. `route_checks` — time-series check results, indexed by
   `(route_id, checked_at)` and `(project_id, checked_at)`, pruned by a
   retention job.
6. `route_incidents` — open/resolved incident windows per route.
7. `route_import_jobs` — validate → preview → commit lifecycle, parsed items
   stored as JSON on the job row (avoids a huge preview table), final counts
   (created/updated/skipped/removed) as the "result report".

## Route health states (single source of truth: `internal/domain/route.go`)

- `disabled` — monitoring turned off by user.
- `unknown` — never checked yet.
- `healthy` — last check succeeded and no active failure streak.
- `degraded` — failing, but below the consecutive-failure threshold.
- `failing` — consecutive failures ≥ threshold (incident open).

Incident rule (tested): opens after N consecutive failures (default 3),
resolves after M consecutive successes (default 1), both configurable per
project/route.

## Security requirements checklist

- SSRF: reuse/extend `worker.validateTarget` into a shared, testable
  `internal/monitor` (or reuse existing `worker` package) validator: block
  loopback/private/link-local/metadata IPs, re-validate on every redirect hop
  via `http.Client.CheckRedirect`, cap redirects, enforce scheme allow-list.
- Upload/paste limits: max spec size, Fiber `BodyLimit`, YAML/JSON decode
  depth & node-count guards (no billion-laughs/alias bombs).
- Secret redaction: header values that look like secrets (Authorization,
  API keys, cookies) are masked before storage/echoing in import previews
  and logs.
- AuthZ: every project-scoped route requires bearer auth + membership check;
  unauthorized/nonexistent projects both return 404 to avoid enumeration.
- Bulk operations (bulk add/import/delete) always re-validate project
  ownership and per-row input before mutating.

## Section plan (commit after each)

1. Domain + models + migrations (schema foundation). **[this commit]**
2. Ports + OpenAPI/Swagger parser package + parser unit tests.
3. MySQL adapters for auth/projects/routes/checks/incidents/imports.
4. Application services (auth, projects, routes, import, monitoring/health).
5. HTTP middleware + handlers + route wiring (fiber app, config, body limits).
6. Worker: route check task, SSRF-safe evaluator, scheduling, aggregation
   job, retention job; wiring in `internal/app` and asynq runtime.
7. Backend tests: unit (domain/parser/ssrf), integration-style handler tests
   with sqlmock or a real MySQL via docker-compose, worker tests.
8. Frontend: Projects tab, project dashboard, route table, route detail,
   import wizard, auth screens; styles; docs (`USER_GUIDE.md`, `README.md`).
9. End-to-end validation pass: build, vet, test, migration smoke test,
   manual UI walkthrough notes, fix regressions.
10. Final report: architecture, schema, APIs, pages, security, tests,
    commands executed, verification evidence.

Each section ends with `go build ./...`, `go vet ./...`, relevant
`go test ./...` runs, and a git commit scoped to that section.
