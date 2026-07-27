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

1. Domain + models + migrations (schema foundation). **Done.**
2. Ports + OpenAPI/Swagger parser package + parser unit tests. **Done.**
3. MySQL adapters for auth/projects/routes/checks/incidents/imports. **Done.**
4. Application services (auth, projects, routes, import, monitoring/health). **Done.**
5. HTTP middleware + handlers + route wiring (fiber app, config, body limits). **Done.**
6. Worker: route monitoring engine (below). **Done.**
7. Backend tests (below). **Done.**
8. Frontend (below). **Done.**
9. End-to-end validation pass (below). **Next.**
10. Final report (below).

Each section ends with `go build ./...`, `go vet ./...`, relevant
`go test ./...` runs, and a git commit scoped to that section. This file's
"Progress Log" below is updated after every commit.


Each section below still ends with `go build ./...`, `go vet ./...`, the
relevant `go test ./...` run, a git commit scoped to that section, a
Progress Log update, and a push to `origin/main`.

### Section 6 - Background monitoring worker

Goal: a persistent, SSRF-safe, concurrency-controlled background engine
that checks every enabled route on its own schedule, with no frontend
timers involved anywhere.

1. Extract the existing SSRF target validator (`worker.validateTarget`) and
   the HTTP GET check logic out of `internal/worker/processor.go` into a
   shared, reusable evaluator (still inside `internal/worker`, no new
   package needed) capable of:
   - arbitrary HTTP methods (route checks are not always GET),
   - per-route timeout (`route.TimeoutMS`),
   - per-route retries (`route.Retries`) with bounded backoff, recording
     the attempt count that produced the final result,
   - a custom `http.Client.CheckRedirect` that re-validates every redirect
     hop against the same private/loopback/link-local/metadata blocklist
     (redirects are a classic SSRF bypass) and caps the number of hops,
   - expected-status-range matching (`route.ExpectedStatusRange`,
     e.g. `200-399`) instead of a hardcoded 2xx/3xx check,
   - path-parameter substitution using stored parameter examples/defaults
     (fallback synthetic values) so routes with `{id}`-style templates can
     still be dispatched,
   - response size cap (reuse the existing 1MB `io.LimitReader` pattern),
   - redacted custom headers applied to the outgoing request from the
     route's `Headers` JSON (never logged in cleartext).
2. New asynq task types in `internal/worker/tasks.go`:
   - `route:enqueue_due_checks` - cursor-paginated scan of due, enabled
     routes (mirrors `HandleEnqueueDueChecks`), enqueuing one
     `route:check` task per route with `asynq.Unique(interval)` keyed by
     route ID to guarantee duplicate-job prevention.
   - `route:check` - runs one evaluation, then calls
     `Service.ProcessRouteCheckResult` (already implemented in Section 4)
     to persist the check, update health state, and open/resolve
     incidents.
   - `route:aggregate_metrics` - calls `RouteStore.AggregateRouteMetrics`
     then `AggregateProjectMetrics` on a fixed cadence (e.g. every 60s),
     giving the dashboard cheap, pre-computed reads instead of scanning
     raw `route_checks` per request.
   - `route:prune_checks` - calls `RouteStore.PruneRouteChecks` with a
     configurable retention window (e.g. 30-90 days) on a daily cadence.
3. Concurrency control: reuse the existing asynq `Queues` weighting
   (`critical`/`default`); route checks run on `default` so they cannot
   starve the legacy website checks on `critical`, and asynq's own
   `Concurrency` setting bounds total in-flight work. No unbounded
   goroutines anywhere.
4. Config additions (`internal/config/config.go`): route check retention
   window, aggregation interval, default per-route timeout ceiling - all
   with safe defaults so no env changes are required to run.
5. Wiring: register the new task handlers on the existing `asynq.ServeMux`
   in `worker.Processor.Register`, and schedule the periodic tasks in
   `internal/platform/worker/asynq_runtime.go` next to the existing
   `@every` registrations. `internal/app/application.go` needs no
   structural change beyond passing the already-shared `mysql.Store`.
6. Tests: table-driven tests for the shared evaluator (private IP
   rejection, metadata endpoint rejection, redirect-to-private-IP
   rejection, timeout behavior, expected-status-range matching, retry
   counting) using `httptest.Server`, plus a test that
   `route:enqueue_due_checks` only enqueues enabled/due routes and skips
   disabled ones.

### Section 7 - Backend automated tests

Goal: satisfy acceptance criteria 3, 4, 5 with real, runnable tests (not
aspirational documentation).

1. Domain: already covered (Section 4) - extend only if gaps are found.
2. `internal/openapi`: already covered (Section 2) - extend if gaps found
   (e.g. a malformed-YAML fixture, a Swagger2 spec with `formData`).
3. `internal/application`: add a lightweight in-memory fake implementation
   of each new port (`fakeUserStore`, `fakeProjectStore`, `fakeRouteStore`,
   `fakeRouteIncidentStore`, `fakeImportStore`) local to the test file(s),
   following Go's standard "hand-rolled fake" testing style (no new mock
   framework dependency). Cover:
   - register/login/duplicate-email/bad-password/token-expiry,
   - project creation + `AuthorizeProject` (owner/editor/viewer, wrong
     user, nonexistent project - all must behave per the 404-for-both rule),
   - `ValidateImport` conflict detection (create/update/skip/duplicate
     in spec/removed-from-spec) against a seeded fake route set,
   - `CommitImport` never overwrites monitoring config on update, disables
     (not deletes) unselected-for-removal-then-selected routes, and
     produces accurate created/updated/skipped/removed counts,
   - `ProcessRouteCheckResult` incident open-after-N-failures and
     resolve-after-M-successes end-to-end against the fake stores (this is
     the acceptance-criteria-4 test).
4. `internal/api`: `httptest`-based handler tests using the fakes from (3)
   wired into a real `fiber.App` via `httpserver.NewFiberApp` (or a
   trimmed router built the same way), covering:
   - missing/invalid bearer token -> 401,
   - authenticated but non-member project access -> 404,
   - viewer role attempting an editor/owner action -> 403,
   - malformed OpenAPI upload -> 400 with a useful error,
   - oversized upload -> 413,
   - duplicate route creation -> 400 with `ErrDuplicateRoute`,
   - partial bulk-create failure reporting (mixed valid/invalid rows).
5. `internal/worker`: extend `processor_test.go` (or a new
   `route_processor_test.go`) with the SSRF/redirect/timeout/retry table
   tests from Section 6 plus a test proving a slow/failing route does not
   block enqueuing of other due routes (bounded timeout enforcement).
6. Migration smoke test: a small `go test` (or `make`/script target) that
   runs `ApplyMigrations` against a throwaway MySQL schema (via
   `docker-compose`) up, then down, then up again, asserting no errors -
   this is the existing project's only DB-dependent test category, so it
   is opt-in behind a build tag or `MYSQL_DSN` env check, consistent with
   how a project without existing DB integration tests would introduce
   them without breaking CI when no DB is reachable.
7. Large-import acceptance test: generate a >=500 route OpenAPI document
   (same technique as `TestParseLargeSpecification`), run it through
   `ValidateImport` + `CommitImport` against the fakes, and assert all 500+
   routes are created and independently searchable/filterable via
   `ListRoutes`, directly demonstrating acceptance criterion 1 at the
   service layer (the true end-to-end run happens manually in Section 9
   against real MySQL).

### Section 8 - Frontend

Goal: every required page/state, matching the existing vanilla JS/CSS
conventions in `frontend/app.js`, `index.html`, `styles.css` (tabs, toasts,
modals, skeleton loaders, the `api()` fetch helper, `escapeHtml`, empty
states, relative time formatting) - no new frontend framework.

1. Auth: a minimal login/register panel (email + password), token stored
   in `localStorage` under a project-auth-specific key (kept separate from
   the existing `apiKey` local storage entry so both auth schemes coexist),
   an `apiProjects()` fetch helper mirroring `api()` but sending
   `Authorization: Bearer <token>` and handling 401 by returning to the
   login panel.
2. New top-level nav tab "Projects" (added next to the existing Monitors/
   Incidents/Status Pages tabs in `index.html`), with its own internal
   mini view-router (list / detail / route-detail / import-wizard) driven
   by a `location.hash` sub-route (e.g. `#/projects/42/routes/7`) so
   browser back/forward and refresh behave sensibly.
3. Projects list (main dashboard) view:
   - cards/table of all projects with route counts, aggregate health
     (healthy/degraded/failing/disabled/unknown counts), uptime24h,
     avg latency24h, open incidents, last-check relative time,
   - search box + status filter (active/archived), create-project modal,
   - archive/unarchive/delete actions with the existing confirm-modal
     pattern, edit-project modal,
   - empty state ("no projects yet") and skeleton loading state reusing
     `showTableSkeleton`-style helpers.
4. Project dashboard view:
   - summary metric cards (routes total/healthy/degraded/failing/disabled,
     uptime24h, avg latency, open incidents),
   - a lightweight time-range chart (plain `<canvas>` + vanilla JS
     sparkline/line rendering, no chart library dependency, consistent
     with the project's zero-heavy-dependency frontend) for uptime/latency
     over selectable ranges (1h/24h/7d) sourced from `route_checks`
     aggregation endpoints added as thin read APIs in Section 6/7 if not
     already covered by Section 5's handlers (add a
     `GET /api/projects/:id/metrics/timeseries` endpoint backed by a
     grouped `route_checks` query if the dashboard needs finer granularity
     than the cached 24h columns),
   - incidents list (open + recently resolved) with relative timestamps,
   - route table: search, method/status/tag/enabled/deprecated filters,
     sortable columns, pagination (or windowed rendering for very large
     route counts, matching the "hundreds/thousands of routes" requirement
     without pulling in a virtualization library - render only the current
     page from the server-paginated API),
   - row actions: enable/disable, edit, delete, open detail; bulk
     selection + bulk delete/enable/disable with the existing confirm
     modal.
5. Route detail view: configuration (method, path, base URL, parameters,
   request body, responses, security, tags, deprecated flag - all
   read/edit), current health badge, uptime/latency stats, status-code
   history table, recent checks, recent incidents for that route.
6. Import wizard (multi-step, modal or dedicated view):
   - step 1: upload file or paste spec text, optional base-URL override,
   - step 2: validation result - parsed route count, warnings, per-item
     table with conflict badges (new/changed/unchanged/duplicate/removed)
     and checkboxes (pre-checked per the backend's default selection),
     with bulk select-all/none and per-conflict-type quick filters,
   - step 3: commit - progress indicator, then a result report screen
     (created/updated/skipped/disabled counts + any per-row warnings),
   - error states for malformed specs, oversized files, and network
     failures, each with a clear retry path.
7. Styling: extend `styles.css` with the new component classes needed
   above, reusing existing design tokens/variables (colors, spacing,
   radii) rather than introducing a parallel style system.
8. Docs: update `USER_GUIDE.md` with a "Projects & API Route Monitoring"
   section (creating a project, importing a spec, reading the dashboard,
   understanding health states) and update `README.md`'s capability list
   and architecture section to mention the new bounded context.

### Section 9 - End-to-end validation pass

1. `go build ./...`, `go vet ./...`, `go test ./...` (already continuously
   green through Section 5; re-verify after 6-8).
2. `docker compose up -d` against `docker-compose.yml`, run the app,
   apply migrations for real, and manually walk through: register ->
   create project -> generate/import a 500+ route OpenAPI file -> resolve
   conflicts -> commit -> confirm routes appear, are searchable/filterable
   -> disable/edit/delete a route -> wait for/force a check cycle -> watch
   metrics update -> simulate consecutive failures (point a route at a
   deliberately failing local test server) -> confirm one incident opens
   -> fix it -> confirm the incident resolves per the tested rule.
3. Exercise the negative paths manually once each: wrong-user project
   access (expect 404), viewer attempting a write (expect 403), malformed
   spec upload (expect 400 with message), oversized upload (expect 413),
   SSRF attempt (route pointed at `169.254.169.254` or `127.0.0.1`, expect
   the check to fail closed with a clear reason, never a real request).
4. Fix every regression found during the above before moving on; re-run
   the full automated test suite after each fix.
5. Record concrete verification evidence (command output, screenshots or
   described UI states) for the final report.

### Section 10 - Final report

Produce the closing summary required by the task: architecture recap,
full schema diff, full API surface, frontend pages/components added,
security protections implemented, full test inventory, exact commands run
for verification, and honest notes on any known limitations or follow-up
work.

## Progress log

- **Section 1 (commit `9e06cf7`)** — Done. Added this plan, route health/
  incident domain policies (`internal/domain/route.go`), new models
  (`users`, `projects`, `routes`, `import`), and additive migrations
  `0004_auth` (users, auth_tokens) and `0005_projects` (projects,
  project_members, api_routes, route_checks, route_incidents,
  route_import_jobs). `go build ./...` clean.
- **Section 2 (commit `9ab86c5`)** — Done. Added new domain ports
  (`UserStore`, `AuthTokenStore`, `ProjectStore`, `RouteStore`,
  `RouteIncidentStore`, `ImportStore`) and the `internal/openapi` package:
  OpenAPI 3.x / Swagger 2.0 JSON+YAML parser with local-only `$ref`
  resolution (no remote fetches — SSRF hardening), circular-ref guard,
  document size/operation-count limits, and spec-hash based change
  detection. 9 unit tests added, including a 520-operation generated spec
  and an over-limit rejection test. All passing.
- **Section 3 (commit `5386d61`)** — Done. Implemented MySQL adapters for
  all new ports: `users.go` (accounts + bearer tokens), `projects.go`
  (CRUD, membership, search/filter/pagination), `routes.go` (CRUD, bulk
  insert/delete, due-check listing, check recording, batched 24h metric
  aggregation via single `UPDATE...JOIN` queries for both routes and
  projects, bounded-batch retention pruning), `route_incidents.go`, and
  `imports.go` (job persistence with parsed items as JSON). `go build`
  and `go vet` clean.
- **Section 4 (commit `8dec12b`)** — Done. Extended `application.Service`
  (backward-compatible additive constructor params) with: `auth.go`
  (register/login/logout/authenticate, bcrypt + sha256-hashed opaque
  bearer tokens, 30-day TTL), `projects.go` (create/update/archive/delete,
  slug generation, `AuthorizeProject` central 404-for-both-cases
  authz helper), `routes.go` (manual create/bulk-create with per-row
  partial-failure reporting, update/enable/disable/delete, and
  `ProcessRouteCheckResult` which is the single place check outcomes turn
  into route status + persisted check + incident open/resolve, reusing the
  existing outbox for alert dispatch), and `imports.go` (`ValidateImport`
  parses+diffs a spec into a persisted preview job with per-item
  create/update/skip/remove actions and conflict labels; `CommitImport`
  applies a user-confirmed selection, only ever touching spec-derived
  metadata on updates — never user-owned monitoring config — and disables
  (never hard-deletes) routes removed from the spec unless explicitly
  selected). Added domain unit tests for `ComputeRouteStatus`,
  `RouteIncidentPolicy`, and method/path normalization. Full repo
  `go build`, `go vet`, `go test ./...` clean.
- **Section 5 (commit `d38a7f3`)** — Done. Added `BearerAuth` middleware
  (independent from the legacy `APIKeyAuth`) and handlers: `auth_handler.go`
  (register/login/logout/me), `project_handler.go` (list/create/get/update/
  archive/unarchive/delete), `route_handler.go` (list with search/filter/
  sort/pagination, create, bulk create with per-row error reporting,
  get/update/enable/disable/delete, bulk delete, check history, project
  incidents), `import_handler.go` (validate via multipart file upload or
  JSON paste, get job, commit with per-item selections). Added
  `project_authz.go`: a single `authorizeProject` helper used by every
  project-scoped handler, returning 404 for both "project does not exist"
  and "caller is not a member" (prevents project-ID enumeration), 403 for
  insufficient role, and route/import lookups additionally verify the
  resource's `projectId` matches the URL to block cross-project access via
  guessed IDs. Route/import responses redact sensitive header values
  (`RedactHeaders`) so configured secrets are never echoed back. Wired
  everything into `internal/platform/httpserver/fiber.go`: legacy
  `/api/websites|checks|...` routes keep the exact same `APIKeyAuth`
  behavior as before (fully backward compatible); new `/api/auth`,
  `/api/projects...` routes use bearer auth; Fiber `BodyLimit` raised to
  15MB to safely accept large OpenAPI upload requests ahead of the
  parser's own 10MB document cap. `go build`, `go vet`, `go test ./...`
  all clean.
- **Section 6 (commit pending)** — Done. Added a persistent Asynq route
  monitoring engine with cursor-paginated due-route enqueueing, per-route
  unique-task dedupe, bounded worker concurrency, arbitrary HTTP methods,
  path-parameter substitution, per-route timeout/retry policies, expected
  status ranges, response-size limits, and custom headers. The evaluator
  validates HTTP(S) targets before dispatch, blocks private/loopback/
  link-local/metadata and special-purpose IP ranges, dials only validated
  resolved addresses, and revalidates every redirect hop. Added scheduled
  batched 24-hour route/project aggregation and bounded retention pruning
  with safe configuration defaults. Worker tests cover unsafe targets,
  redirect-to-metadata rejection, timeouts, retries/attempt counts, status
  ranges, methods, and path substitution. Full `go test ./...`, `go build
  ./...`, and `go vet ./...` pass with an isolated Go build cache.
- **Section 7 (commit pending)** — Done. Added hand-rolled in-memory port
  fakes and service-level acceptance tests covering registration, login,
  duplicate emails, bad credentials, bearer-token expiry, owner/editor/
  viewer authorization, indistinguishable nonmember/not-found access,
  mixed-result bulk creation, a generated 520-route OpenAPI validate/
  preview/commit/search flow, re-import metadata updates that preserve all
  user monitoring settings, explicitly selected removed-route disabling,
  and the full failure-threshold/recovery-threshold incident lifecycle
  (including proof that consecutive extra failures do not create duplicate
  incidents). Together with the existing parser/domain tests and Section 6
  evaluator tests, malformed specifications, excessive specs, local ref
  safety, duplicates, timeouts, redirects, retries, private/metadata
  targets, partial failures, health transitions, and recovery are covered.
  Full `go test ./...`, `go build ./...`, and `go vet ./...` pass.
- **Section 8 (commit pending)** — Done. Extended the existing Watchtower
  vanilla JS/CSS UI without changing its theme or design system: separate
  project register/login session, hash-routed project list/dashboard/route
  detail/import views, owner-only archive/restore/delete controls, project
  create/edit, eight aggregate metric cards, real grouped 1h/24h/7d
  time-series chart data, incident summaries, a server-paginated searchable/
  filterable/sortable route table, role-aware manual route CRUD and bulk
  enable/disable/delete, complete configuration/check/status/incident route
  detail, and a three-step upload-or-paste import wizard with conflict
  filtering, selection, and final counts. Added responsive/loading/error/
  empty states and updated the README and user guide. Added the bounded
  grouped metrics API and a joined project/member query that returns the
  viewer role without N+1 reads. Tightened base-URL and header validation
  and preserved omitted secret headers during unrelated edits. `node
  --check frontend/app.js`, full Go tests/build/vet, and local HTTP delivery
  of all three frontend assets pass. Browser screenshot inspection was not
  possible because no browser backend was available; Docker-backed runtime
  validation was not possible because Docker is absent from the environment.
- **Section 9 (next)** — Real migration/API workflow where dependencies are
  available, negative-path HTTP checks, final regression/security review,
  and complete verification evidence.
