# Argus v2 delivery progress

## Delivery control

- Branch: `feat/argus-v2`
- Integration base: `origin/main` at `3ca0c98` (2026-07-28)
- Source contract: `docs/audit-2026-07-28-en/ARGUS_V2_IMPLEMENTATION_AGENT_PROMPT.md`
- Working rule: every completed checkpoint is committed and pushed before the
  next checkpoint begins.

## Baseline

| Area | Current behavior | Required v2 change | Evidence | Status |
| --- | --- | --- | --- | --- |
| Identity | Token-based project auth; legacy API key can fail open | Cookie sessions, CSRF, expiry, revocation, secure defaults | `internal/application/auth.go`, `internal/adapters/inbound/http/middleware.go` | Planned |
| Product shell | Guest and project controls can coexist | Separate public, identity, and authenticated shells | `frontend/index.html`, `frontend/app.js`, `frontend/projects.js` | Planned |
| Route monitoring | Imported routes can be evaluated as recurring requests | Catalog-only import; explicit budgeted synthetics | `internal/application/imports.go`, `internal/worker/route_processor.go` | Planned |
| URL handling | Validation is split and partly deferred | One canonical backend pipeline and preview API | `internal/application/routes.go`, `internal/worker/route_evaluator.go` | Planned |
| Telemetry and SLOs | No telemetry-first pipeline or SLO control plane | Authenticated OTLP, mapping, SLOs, incidents, self-monitoring | New v2 components | Planned |
| Accessibility and motion | Known hidden-state, dialog, table, refresh, and motion defects | WCAG 2.2 AA and all five motion plans | `animation-plans/`, blueprint §5 | Planned |

## Security, threat, and migration traceability

| Contract | Required outcome | Owner area | Acceptance evidence | Status | Commit |
| --- | --- | --- | --- | --- | --- |
| SEC-001, SEC-003, SEC-004, SEC-009 | Secure sessions; fail-closed auth; abuse limits; coalesced session activity | auth, HTTP middleware, MySQL | lifecycle, CSRF, rate-limit, negative-auth tests | Planned | — |
| SEC-002, SEC-010, SEC-011 | CSP-safe DOM rendering and exclusive accessible UI state | frontend | browser and keyboard tests | Planned | — |
| SEC-005, SEC-007 | Catalog/synthetic separation and canonical validation | routes, imports, worker | zero-import-traffic and normalization tests | Planned | — |
| SEC-006 | Encrypted/rotatable synthetic secret references | secrets, migrations | redaction and rotation tests | Planned | — |
| SEC-008 | Endpoint limits and server timeouts | HTTP platform | configuration and slow-client tests | Planned | — |
| Threat model: tenant isolation | Scope every request, job, token, mapping, incident, and export | API, stores, workers | cross-tenant negative tests | Planned | — |
| Threat model: SSRF | Preserve dial- and redirect-time address validation | worker, synthetic policy | redirect, DNS, IPv4/IPv6 test suite | Planned | — |
| Migration | Additive, restartable, reversible changes and conflict reporting | migrations, backfill | fresh and legacy migration tests | Planned | — |

## Delivery roadmap

| Checkpoint | Scope | Status | Commit |
| --- | --- | --- | --- |
| 0 | Branch, baseline evidence, architecture/traceability controls | In progress | — |
| 1 | Identity, authorization, secure global shell | Planned | — |
| 2 | Environments, endpoint canonicalization, safe synthetic migration | Planned | — |
| 3 | Telemetry ingestion, metric storage, mapping, SLO and incidents | Planned | — |
| 4 | Onboarding and all management views; accessibility and motion | Planned | — |
| 5 | Migration, operations, documentation, complete verification | Planned | — |

## Decisions and constraints

- OpenAPI import is catalog-only and must make zero outbound requests.
- Synthetic checks are disabled at creation, default to `GET`/`HEAD`, and
  never allow `TRACE`; unsafe methods require a separately authorized policy.
- URL normalization does not replace outbound SSRF controls at dial and every
  redirect hop.
- Missing, stale, paused, and maintenance data are distinct states; no-data is
  never silently treated as healthy.
- The central control plane does not probe private customer networks; private
  targets require an environment-local agent.

## Checkpoint evidence

| Date | Checkpoint | Evidence | Result |
| --- | --- | --- | --- |
| 2026-07-28 | 0 | `go test ./...` and `go test -race ./...` passed; `govulncheck ./...` passed; `git diff --check` passed. `staticcheck ./...` was not runnable through the mandated RTK wrapper (`./... matched no packages`) and remains a CI verification item. | Complete |
