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
6. Worker: route monitoring engine (below). **Next.**
7. Backend tests (below).
8. Frontend (below).
9. End-to-end validation pass (below).
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
- **Section 4 (commit pending)** — Done. Extended `application.Service`
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
- **Section 5 (commit pending)** — Done. Added `BearerAuth` middleware
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
- **Section 6 (commit pending)** — Done. Added `internal/worker/
  route_evaluator.go`: a shared, hardened HTTP evaluator used by every route
  check. Its address policy runs in the dialer's `Control` hook, so it sees
  the *resolved* IP immediately before connect — this closes the DNS-rebinding
  hole a hostname-only pre-flight check leaves open, and it applies
  identically to every redirect hop. Blocks (always) cloud metadata
  hostnames/ranges, IPv6 link-local, CGNAT, benchmarking, reserved and
  documentation ranges, multicast and unspecified addresses; blocks (by
  default policy) loopback, RFC1918/ULA private and link-local unicast, with
  `ROUTE_ALLOW_PRIVATE_TARGETS=true` as a documented opt-in for operators
  monitoring internal APIs — metadata endpoints stay blocked either way.
  Also: scheme allow-list, embedded-credential rejection, redirect cap with
  per-hop re-validation, `Authorization`/`Cookie`/API-key stripping on
  cross-origin redirects, per-route timeout (clamped to a ceiling), bounded
  exponential retry with attempt counting, no retry for policy-blocked
  targets, expected-status-range matching (`200-399`, `200,201`, `200-204,301`),
  1MB response cap, custom header injection with hop-by-hop headers filtered,
  and URL-escaped path-parameter substitution (`{id}` / `:id`) driven by the
  spec's example/default values. Added `route_processor.go` with four asynq
  handlers — `route:enqueue_due_checks` (cursor-paginated scan, one
  `asynq.Unique`-keyed task per route for duplicate-job prevention, tolerant
  of `ErrDuplicateTask`), `route:check` (re-reads the route so stale config is
  never used, then delegates to `ProcessRouteCheckResult`),
  `route:aggregate_metrics`, and `route:prune_checks` (bounded batches with an
  iteration cap). All new tasks run on the `default` queue so they cannot
  starve legacy website checks on `critical`; asynq's `Concurrency` bounds
  in-flight work and no unbounded goroutines are created. Config gained ten
  `ROUTE_*` knobs, all with safe defaults. Wired into
  `platform/worker/asynq_runtime.go` and `app/application.go`. 30 new worker
  tests (SSRF table incl. DNS rebinding, redirect cap, blocked-redirect,
  secret stripping, timeout, retry counting, status ranges, path-param
  escaping, enqueue filtering/pagination/dedupe, prune batching, aggregation
  window). Added `.gitattributes` (`*.go text eol=lf`) and normalized the
  tree to LF: with `core.autocrlf=true` a Windows checkout rewrote every Go
  file to CRLF, which made `gofmt -l` report the entire repository as
  unformatted. `gofmt -l` is now clean, `go build`, `go vet`, `go test ./...`
  all pass.
- **Section 7 (commit pending)** — Done. Added `internal/testsupport`: one
  shared set of hand-rolled, in-memory port implementations (no mock
  framework) mirroring the MySQL adapters' observable semantics, used by both
  the application and API test suites. It depends only on `domain`/`models`
  so it cannot create an import cycle, and is referenced exclusively from
  `_test.go` files so it is never linked into `cmd/api`.

  Application tests (`auth_test.go`, `projects_test.go`, `routes_test.go`,
  `imports_test.go`): register/login/logout/authenticate incl. duplicate
  email, wrong password, unknown account returning the *same* error,
  token expiry, per-session token isolation; project create/update/archive/
  unarchive/delete, default clamping, slug uniqueness, membership+status
  filtering; the full `AuthorizeProject` owner/editor/viewer matrix including
  a test that the non-member and nonexistent-project errors are
  byte-identical; route create/normalize/duplicate detection, bulk-create
  partial-failure reporting per input row, update field preservation and
  clamping, disabled-state derivation, project-scoped bulk delete, header
  redaction; `ProcessRouteCheckResult` metric updates and the full
  healthy→degraded→failing→healthy progression asserting exactly ONE incident
  opens after N consecutive failures and exactly one resolution after M
  successes (acceptance criteria 3 and 4), with per-route configurable
  thresholds; import validate/commit covering create/update/skip/
  duplicate-in-spec/removed-from-spec, malformed/non-spec/no-paths/oversized/
  bad-source-type rejection, cross-project job isolation, double-commit
  conflict, Swagger 2.0 host+basePath+scheme handling, per-row commit failure
  reporting that does not abort the batch, and the re-import contract:
  metadata refreshed, user monitoring config (interval/timeout/retries/
  status range/thresholds/headers) provably untouched, removals reported but
  never pre-selected and applied as *disable*, never delete.

  API tests (`internal/api/handlers_test.go`, package `api_test`) drive the
  real `httpserver.NewFiberApp` — same middleware stack and body limit as
  production — covering 401 for missing/garbage/empty/unknown bearer tokens
  across every project endpoint, 404 for non-members with a byte-identical
  body to the nonexistent-project case, 403 for viewer-attempting-write and
  editor-attempting-owner-action, 400 for invalid project IDs, duplicate
  routes, malformed JSON and malformed/non-spec uploads, 413 for oversized
  uploads and pastes, 409 for commit replay, bulk-create per-row reporting
  and the 5000-row cap, cross-project route access via guessed route IDs
  (404, resource untouched), project-scoped bulk delete over HTTP, secret
  header redaction on every read path while storage keeps the real value,
  full route lifecycle, all list query parameters, and a 600-route import
  driven end to end through multipart upload → preview → commit → search.

  **Bug found and fixed:** `httpserver.NewFiberApp` mounted the legacy
  `APIKeyAuth` via `apiGroup.Group("", mw)`, which in Fiber registers the
  middleware on the *whole* `/api` subtree — so with `API_KEY` set, every new
  `/api/auth` and `/api/projects` endpoint returned 401 `unauthorized` and the
  entire project subsystem was unreachable. Section 5's tests had not covered
  a non-empty API key, so it was invisible. Fixed by attaching each scheme's
  guard per route via a new `api.guarded` helper (`internal/api/guards.go`)
  and making every `Register*Routes` function take variadic guards; the two
  auth schemes are now independent regardless of registration order, and
  `TestLegacyAPIKeyRoutesAreUnchanged` locks the behavior in both directions.

  Also added `internal/platform/storage/migrate_test.go`: a DB-free structural
  test (every up migration has a down migration, none are empty, none use
  `DELIMITER` which the naive `;` splitter cannot handle, lexical order
  matches apply order) plus an opt-in up→up→down→up smoke test against a
  throwaway MySQL schema, gated on `MYSQL_TEST_DSN` so the suite stays green
  without a database.
- **Section 8 (commit pending)** — Done. Added the read API the dashboard
  charts need: `RouteStore.AggregateCheckTimeseries` (single grouped query
  bucketing `route_checks` by a fixed width, covered by the existing
  `(project_id, checked_at)` / `(route_id, checked_at)` indexes and capped at
  500 buckets), `Service.ListMetricsTimeseries` (five named ranges — 1h/6h/
  24h/7d/30d — each chosen to yield ≤ 56 buckets; an unknown range falls back
  to 24h instead of erroring so a stale bookmark cannot break the page), and
  `GET /api/projects/:id/metrics/timeseries?range=&routeId=` which verifies a
  supplied `routeId` belongs to the project before exposing its data.

  Frontend: new `frontend/projects.js` (self-contained IIFE, no framework and
  no third-party JavaScript) plus an "API Projects" tab in `index.html` and
  new component classes appended to `styles.css` using only the existing
  design tokens. It reuses `app.js`'s `showToast`, `escapeHtml`,
  `relativeTime`, `setButtonLoading` and `activateTab` rather than
  duplicating them, and leaves every existing behaviour untouched. Contains:
  a sign-in/registration gate storing its token under `argus_project_token`
  (kept separate from the legacy `argus_api_key`), an `apiProjects()` fetch
  helper that surfaces the server's JSON `error` field and returns to the gate
  on 401; a `location.hash` sub-router (`#/projects`,
  `#/projects/:id`, `#/projects/:id/routes/:routeId`, `#/projects/:id/import`)
  with breadcrumbs so back/forward and refresh behave; the projects dashboard
  (cards with route counts, per-state health chips, uptime, latency, failures,
  open incidents and last-check time, plus search, status filter, create/edit
  modal, archive/restore/delete with confirmation, pagination, empty, error
  and skeleton states); the project dashboard (ten summary tiles, a range
  picker driving two hand-rolled `<canvas>` charts for uptime and latency,
  incident list with durations, and a route table with server-side search,
  method/health/tag/enabled/deprecated filters, sortable columns,
  pagination, row actions and bulk enable/disable/delete with bounded
  concurrency); the route detail page (full configuration, collapsible
  parameters/request body/responses/security blocks, health, 24h stats, a
  status-code distribution, the recent check log and per-route incidents);
  and the three-step import wizard (upload or paste with a client-side size
  pre-check, a preview table with new/changed/unchanged/duplicate/removed
  badges, per-conflict quick filters, per-row and bulk selection, paginated
  at 100 rows so a 600-operation spec stays responsive, then a result
  report). All destructive actions go through a confirmation modal; secrets
  are never echoed into the edit form. Interactions use event delegation
  rather than inline `onclick` string interpolation.

  Docs: `USER_GUIDE.md` gained a full "Projects & API route monitoring"
  section (sign-in and the role matrix, creating projects, the four ways to
  add routes, the import wizard and its conflict badges, the re-import
  guarantees, reading the dashboard, the health-state table, the incident
  rule, how background checking works, the untrusted-URL threat model, the
  complete project API reference and every `ROUTE_*` setting) plus migration
  portability notes. `README.md` now describes both bounded contexts, the
  updated package layout and the security posture.

- **Section 9 (commit pending)** — Done. Ran the full stack against real
  MySQL 8 and Redis and fixed every regression found. Three were real bugs
  that would have prevented the app from starting on a stock MySQL server:

  1. `websites.url VARCHAR(2083) NOT NULL UNIQUE` (pre-existing, `0001`) needs
     an 8332-byte index under `utf8mb4`, over InnoDB's 3072-byte limit, so
     `CREATE TABLE` failed outright. Now a 500-character prefix index.
  2. `UNIQUE (project_id, method, path)` on `api_routes` (mine, `0005`) had the
     same defect at 4105 bytes. Now a 700-character prefix on `path`.
  3. `ALTER TABLE ... ADD COLUMN IF NOT EXISTS` in `0003` is a MariaDB
     extension and a **syntax error on MySQL 8** — the engine
     `docker-compose.yml` actually ships. Rewritten as an
     `information_schema` check plus `PREPARE`/`EXECUTE` per column, which is
     idempotent on both engines. That exposed a second problem: session state
     does not survive `database/sql`'s connection pool, so `ApplyMigrations`
     now pins one connection per file. And a third: splitting the batch on a
     naive `;` tore statements apart at semicolons inside comments, so the
     splitter is now quote- and comment-aware (`SplitStatements`, 13 unit
     tests).

  Verified live against MySQL: migrations up → up → down → up clean;
  register/login; project creation; a generated 600-operation OpenAPI 3.0
  document (300 paths, component `$ref`s for parameters/schemas/responses,
  path-level shared parameters, tags, security, request bodies, deprecated
  flags) uploaded as multipart in 167 ms and parsed to exactly 600 preview
  items, all correctly labelled `create`; committing a deliberate 450-of-600
  selection created exactly 450 routes in 0.79 s and skipped 150; commit
  replay returned 409. Search/filter/sort/paginate were exercised over the
  450 real rows (13 filter combinations, 12 sort permutations, and a walk of
  all 5 pages asserting every row was returned exactly once with no
  duplicates). The worker then checked all 450 routes unprompted: at two
  consecutive failures every route read `degraded`, and on the third all 450
  flipped to `failing` with exactly 450 incidents opened — one per route, and
  further failing cycles (2150 checks) opened no duplicates. The aggregation
  job populated the cached project columns, and the timeseries endpoint
  returned correctly bucketed data for every named range with an unknown
  range falling back to 24h. Negative paths all confirmed: 401 without or
  with a bad token, 404 for a non-member *with a byte-identical body to a
  nonexistent project*, 400 for an invalid project ID / duplicate route /
  malformed spec / valid-JSON-but-not-a-spec / empty spec, 413 for a 10 MB+
  upload, 409 on commit replay, and both auth schemes proven mutually
  inert (legacy key rejected on `/api/projects`, bearer token rejected on
  `/api/websites`, legacy key still 200 on `/api/websites`). Finally, 16
  routes were pointed at forbidden targets — loopback v4/v6, `localhost`, all
  three RFC1918 ranges, AWS and GCP metadata, CGNAT, benchmark, unspecified,
  a public hostname resolving to loopback, and the `file`/`gopher`/`ftp`
  schemes plus embedded credentials — and every one failed closed with a
  `blocked target address` reason and `lastStatusCode = 0`, proving no HTTP
  exchange ever took place.

- **Section 10 (next)** — Final report.
