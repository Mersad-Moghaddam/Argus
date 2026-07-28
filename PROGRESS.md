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
| Identity | Token-based project auth; legacy API key can fail open | Cookie sessions, CSRF, expiry, revocation, secure defaults | `internal/application/auth.go`, `internal/adapters/inbound/http/middleware.go`, `frontend/projects.js` | In progress — cookie sessions, global routable account access, password change, and session revocation landed; recovery remains |
| Product shell | Guest and project controls can coexist | Separate public, identity, and authenticated shells | `frontend/index.html`, `frontend/app.js`, `frontend/projects.js` | In progress — global account, authentication, and isolated project shells landed; authenticated navigation/onboarding remain |
| Route monitoring | Imported routes can be evaluated as recurring requests | Catalog-only import; explicit budgeted synthetics | `internal/application/imports.go`, `internal/worker/route_processor.go` | Planned |
| URL handling | Validation is split and partly deferred | One canonical backend pipeline and preview API | `internal/domain/endpoint.go`, route/import services, worker composer | In progress — core canonical pipeline, authenticated preview endpoint, hash migration, and default production environment creation complete; environment management remains |
| Telemetry and SLOs | No telemetry-first pipeline or SLO control plane | Authenticated OTLP, mapping, SLOs, incidents, self-monitoring | New v2 components | Planned |
| Accessibility and motion | Known hidden-state, dialog, table, refresh, and motion defects | WCAG 2.2 AA and all five motion plans | `animation-plans/`, blueprint §5 | Planned |

## Security, threat, and migration traceability

| Contract | Required outcome | Owner area | Acceptance evidence | Status | Commit |
| --- | --- | --- | --- | --- | --- |
| SEC-001, SEC-003, SEC-004, SEC-009 | Secure sessions; fail-closed auth; abuse limits; coalesced session activity | auth, HTTP middleware, MySQL | lifecycle, CSRF, rate-limit, negative-auth tests | In progress — browser session, CSRF, session inventory/revoke-others, rate limit, fail-closed legacy guard, and coalesced last-used writes landed; endpoint-specific limits still expand beyond auth | Pending |
| SEC-002, SEC-010, SEC-011 | CSP-safe DOM rendering and exclusive accessible UI state | frontend | browser and keyboard tests | In progress — global auth route, isolated guest/authenticated project states, and validated return destinations landed; full keyboard evidence remains | Pending |
| SEC-005, SEC-007 | Catalog/synthetic separation and canonical validation | routes, imports, worker | zero-import-traffic and normalization tests | In progress — imports are catalog-only; safe canary method gate, canonical pipeline, and preview endpoint landed; migration remains | Pending |
| SEC-006 | Encrypted/rotatable synthetic secret references | secrets, migrations | redaction and rotation tests | Planned | — |
| SEC-008 | Endpoint limits and server timeouts | HTTP platform | configuration and slow-client tests | In progress — explicit server timeouts, security headers, and auth control-payload limit landed; import-specific limits and slow-client integration evidence remain | Pending |
| Threat model: tenant isolation | Scope every request, job, token, mapping, incident, and export | API, stores, workers | cross-tenant negative tests | Planned | — |
| Threat model: SSRF | Preserve dial- and redirect-time address validation | worker, synthetic policy | redirect, DNS, IPv4/IPv6 test suite | Planned | — |
| Migration | Additive, restartable, reversible changes and conflict reporting | migrations, backfill | fresh and legacy migration tests | In progress — additive schema, canonical dual-write, and an operator-run restartable legacy backfill/conflict workflow landed; cutover remains | Pending |

## Delivery roadmap

| Checkpoint | Scope | Status | Commit |
| --- | --- | --- | --- |
| 0 | Branch, baseline evidence, architecture/traceability controls | In progress | — |
| 1a | Cookie session and browser credential hardening | Complete | Pending |
| 1b | Account session inventory and revoke-other-sessions controls | Complete | Pending |
| 1c | HTTP timeout, browser-header, and authentication-abuse baseline | Complete | Pending |
| 1d | Global account actions, routable identity flow, and safe project return | Complete | Pending |
| 1e | Password change with sibling-session revocation | Complete | Pending |
| 1f | Authenticated account screen and session-management UI | Complete | Pending |
| 1 | Identity, authorization, secure global shell | In progress | — |
| 2a | Catalog-only OpenAPI imports and safe synthetic guardrails | Complete | Pending |
| 2b | Canonical endpoint pipeline and preview API | Complete | Pending |
| 2c | Additive environment and canonical-identity schema | Complete | Pending |
| 2d | Canonical identity dual-write for route mutations | Complete | Pending |
| 2e | Restartable legacy canonical-identity backfill command | Complete | Pending |
| 2f | Default production environment for each new project | Complete | Pending |
| 4a | Hidden-state contract and motion-plan implementation | Complete | Pending |
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
| 2026-07-28 | 1a | Opaque server-stored session token is issued as an HttpOnly, SameSite=Lax cookie; browser mutations require a CSRF cookie/header match; project frontend no longer reads or persists bearer credentials; legacy API keys are memory-only and an unset legacy key fails closed. Focused Go and JavaScript syntax checks passed. | Complete |
| 2026-07-28 | 2a | OpenAPI commit now creates disabled catalog entries only. Explicit synthetic activation is restricted to GET/HEAD; POST, PUT, PATCH, DELETE, OPTIONS, and TRACE cannot be enabled. Broad route polling is now opt-in through `ROUTE_BROAD_POLLING_ENABLED`; fresh v2 configuration does not schedule a request per imported operation. Focused application and HTTP API tests passed. | Complete |
| 2026-07-28 | 4a | Implemented the shared `.hidden`/`[hidden]` rendering contract and all five motion-plan outcomes: static brand mark, instant tab changes, semantic reduced motion, state-driven refresh pulse, and fine-pointer hover behavior for primary controls. Browser accessibility snapshots confirmed that the guest Projects view excludes the hidden authenticated shell and registration-only fields. | Complete |
| 2026-07-28 | 2b | Added one IDNA-aware structured canonicalizer for manual routes, bulk input, OpenAPI import, updates, and worker fetch-target construction. It returns stable codes/fields, preserves the established trailing-slash identity policy, and is exposed through an editor-only normalization preview endpoint with duplicate, safety, traffic, and daily-request feedback. Domain, application, and API tests passed. | Complete |
| 2026-07-28 | 2c | Added an ordered, reversible migration for project environments, nullable versioned canonical identity/hash fields, and an operator-visible collision ledger. It retains legacy route fields during the migration window. Storage migration parser tests passed; live MySQL smoke testing remains available through `MYSQL_TEST_DSN`. | Complete |
| 2026-07-28 | 2d | Manual and OpenAPI-created/updated routes now persist canonical identity, a 32-byte SHA-256 lookup hash, and canonicalizer version `1`. The MySQL route store dual-writes and reads these fields. Focused domain, application, and API tests passed. | Complete |
| 2026-07-28 | 2e | Added `cmd/backfill-endpoint-identity`: an operator-run, dry-run-capable, bounded batch backfill that uses the domain canonicalizer, records invalid legacy rows and exact canonical duplicates in the conflict ledger, and can safely resume. Duplicate detection and writes share a transaction with an index-range lock. Command compilation and domain tests passed. | Complete |
| 2026-07-28 | 2f | New projects now create a default `production` environment within the project/owner transaction. Its base URL is intentionally empty until onboarding or an integration configures it. MySQL adapter compilation and migration-storage tests passed. | Complete |
| 2026-07-28 | Regression checkpoint | `GOCACHE=/tmp/argus-go-build go test ./...` passed after identity, canonical route, migration, and default-environment changes. | Complete |
| 2026-07-28 | 1b | Added authenticated session inventory and revoke-other-sessions controls. Only the current-session marker is returned; token hashes remain server-only. Revocation removes every sibling session while retaining the session used for the request. Application and API tests passed. | Complete |
| 2026-07-28 | 1c | Added explicit Fiber read/write/idle timeouts, strict CSP and companion browser headers, a 256 KiB authentication/control payload guard, and per-IP authentication throttling with `429` responses. Platform, application, and HTTP API tests passed. | Complete |
| 2026-07-28 | 1d | Moved registration/login out of the private Projects tab into dedicated `#/register` and `#/login` routes, with Register as the primary global guest action. Guest project navigation redirects to login and only accepts a constrained same-origin `#/projects/...` return target; header actions reflect the cookie-authenticated session. JavaScript syntax, diff checks, and focused domain/API tests passed. | Complete |
| 2026-07-28 | 1e | Added CSRF-protected `POST /api/auth/password`. It verifies the current password, writes a bcrypt hash, retains the current session, and revokes sibling sessions. Focused application, API, and HTTP-platform tests passed. | Complete |
| 2026-07-28 | 1f | Added authenticated `#/account` with password-change controls, current-session-safe revoke-others action, and an active-session inventory. It uses the cookie/CSRF project client and displays no session token material. JavaScript syntax and diff checks passed. | Complete |
