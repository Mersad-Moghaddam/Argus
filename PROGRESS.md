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
| Identity | Token-based project auth; legacy API key can fail open | Cookie sessions, CSRF, expiry, revocation, secure defaults | `internal/application/auth.go`, `internal/adapters/inbound/http/middleware.go`, `frontend/projects.js` | In progress — cookie sessions, global routable account access, password change/session revocation, and secure recovery landed; endpoint-specific limits remain |
| Product shell | Guest and project controls can coexist | Separate public, identity, and authenticated shells | `frontend/index.html`, `frontend/app.js`, `frontend/projects.js` | In progress — global account, authentication, and isolated project shells landed; authenticated navigation/onboarding remain |
| Route monitoring | Imported routes can be evaluated as recurring requests | Catalog-only import; explicit budgeted synthetics | `internal/application/imports.go`, `internal/worker/route_processor.go` | Planned |
| URL handling | Validation is split and partly deferred | One canonical backend pipeline and preview API | `internal/domain/endpoint.go`, route/import services, worker composer | In progress — core canonical pipeline, authenticated preview endpoint, hash migration, and default production environment creation complete; environment management remains |
| Telemetry and SLOs | No telemetry-first pipeline or SLO control plane | Authenticated OTLP, mapping, SLOs, incidents, self-monitoring | New v2 components | Planned |
| Accessibility and motion | Known hidden-state, dialog, table, refresh, and motion defects | WCAG 2.2 AA and all five motion plans | `animation-plans/`, blueprint §5 | Planned |

## Security, threat, and migration traceability

| Contract | Required outcome | Owner area | Acceptance evidence | Status | Commit |
| --- | --- | --- | --- | --- | --- |
| SEC-001, SEC-003, SEC-004, SEC-009 | Secure sessions; fail-closed auth; abuse limits; coalesced session activity | auth, HTTP middleware, MySQL | lifecycle, CSRF, rate-limit, negative-auth tests | In progress — browser session, CSRF, session inventory/revoke-others, rate limit, fail-closed legacy guard, and coalesced last-used writes landed; endpoint-specific limits still expand beyond auth | Pending |
| SEC-002, SEC-010, SEC-011 | CSP-safe DOM rendering and exclusive accessible UI state | frontend | browser and keyboard tests | In progress — global auth route, isolated guest/authenticated project states, validated return destinations, and semantic route-table sort controls landed; full keyboard evidence remains | Pending |
| SEC-005, SEC-007 | Catalog/synthetic separation and canonical validation | routes, imports, worker | zero-import-traffic and normalization tests | In progress — imports are catalog-only; safe canary method gate, canonical pipeline, and preview endpoint landed; migration remains | Pending |
| SEC-006 | Encrypted/rotatable synthetic secret references | secrets, migrations | redaction and rotation tests | In progress — new route-header writes use versioned AES-GCM ciphertext; legacy plaintext backfill and key rotation remain | Pending |
| SEC-008 | Endpoint limits and server timeouts | HTTP platform | configuration and slow-client tests | In progress — explicit server timeouts, security headers, and auth control-payload limit landed; import-specific limits and slow-client integration evidence remain | Pending |
| Threat model: tenant isolation | Scope every request, job, token, mapping, incident, and export | API, stores, workers | cross-tenant negative tests | Planned | — |
| Threat model: SSRF | Preserve dial- and redirect-time address validation | worker, synthetic policy | redirect, DNS, IPv4/IPv6 test suite | Planned | — |
| Migration | Additive, restartable, reversible changes and conflict reporting | migrations, backfill | fresh and legacy migration tests | In progress — additive schema, canonical dual-write, and an operator-run restartable legacy backfill/conflict workflow landed; cutover remains | Pending |

## Delivery roadmap

| Checkpoint | Scope | Status | Commit |
| --- | --- | --- | --- |
| 0 | Branch, baseline evidence, architecture/traceability controls | In progress | — |
| 1a | Cookie session and browser credential hardening | Complete | `edfa349` |
| 1b | Account session inventory and revoke-other-sessions controls | Complete | `412cec8` |
| 1c | HTTP timeout, browser-header, and authentication-abuse baseline | Complete | `0e38bd8` |
| 1d | Global account actions, routable identity flow, and safe project return | Complete | `507f152` |
| 1e | Password change with sibling-session revocation | Complete | `88aa6ae` |
| 1f | Authenticated account screen and session-management UI | Complete | `4332cab` |
| 1g | Secure password-recovery workflow and delivery boundary | Complete | Current checkpoint |
| 3n | Project-scoped heartbeat monitors and idempotent job pings | Complete | Current checkpoint |
| 3o | Source-aware route incident evidence and acknowledgement | Complete | Current checkpoint |
| 3p | SLO state-transition notification outbox | Complete | Current checkpoint |
| 2i | AEAD boundary for synthetic request headers | In progress | Current checkpoint |
| 1 | Identity, authorization, secure global shell | In progress | — |
| 2a | Catalog-only OpenAPI imports and safe synthetic guardrails | Complete | `8ab613a` |
| 2b | Canonical endpoint pipeline and preview API | Complete | `580e1bb` |
| 2c | Additive environment and canonical-identity schema | Complete | `154cca8` |
| 2d | Canonical identity dual-write for route mutations | Complete | `3ae13b0` |
| 2e | Restartable legacy canonical-identity backfill command | Complete | `4057f15`, `5b0a9e3` |
| 2f | Default production environment for each new project | Complete | `5982c00` |
| 2g | Tenant-scoped project environment API | Complete | `d72bdfd`, `7d6855d`, `f89ea41` |
| 2h | Project environment visibility in authenticated UI | Complete | `0e04dbe`, `8b945e4`, `f451d18` |
| 3a | OpenTelemetry provider lifecycle foundation | Complete | `6531814` |
| 3b | Low-cardinality HTTP trace and metric instrumentation | Complete | `d6fd3b4`, `8e7c619` |
| 3c | Tenant-bound telemetry ingestion credentials | Complete | Current checkpoint |
| 3d | Authenticated OTLP/HTTP receiver and bounded ingestion diagnostics | Complete | Current checkpoint |
| 3e | Project-visible telemetry freshness and mapping diagnostics | Complete | Current checkpoint |
| 3f | Editor OTLP credential and one-time-secret workflow | Complete | Current checkpoint |
| 3g | Durable Prometheus-compatible metrics-store foundation | Complete | Current checkpoint |
| 3h | Safe OTLP HTTP RED-metric export to VictoriaMetrics | Complete | Current checkpoint |
| 3i | Tenant-safe manual telemetry-to-route mappings | Complete | Current checkpoint |
| 3j | Explicit SLO evaluation and burn-rate policy | Complete | Current checkpoint |
| 3k | Versioned SLO definitions and evaluation evidence | Complete | Current checkpoint |
| 3l | Scheduled safe-metrics SLO evaluator | Complete | Current checkpoint |
| 4b | Family/purpose control-plane route taxonomy | Complete | Current checkpoint |
| 4c | Authenticated project identity and source-selection onboarding | Complete | Current checkpoint |
| 4d | Project-visible SLO definition management | Complete | Current checkpoint |
| 4e | Semantic, accessible route-table sorting controls | Complete | Current checkpoint |
| 4f | Focus-contained project dialogs | Complete | Current checkpoint |
| 3m | Project-visible telemetry-route mapping management | Complete | Current checkpoint |
| 4a | Hidden-state contract and motion-plan implementation | Complete | `3a326a1` |
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
| 2026-07-28 | 0 | `go test ./...` and `govulncheck ./...` passed; `git diff --check` passed. Application race tests pass, but the API race suite exceeded a 90-second timeout during bcrypt-heavy registration tests and the full race run was stopped after prolonged no-progress output; neither is recorded as passing. `staticcheck ./...` was not runnable through the mandated RTK wrapper (`./... matched no packages`) and remains a CI verification item. | Complete |
| 2026-07-28 | 1a | Opaque server-stored session token is issued as an HttpOnly, SameSite=Lax cookie; browser mutations require a CSRF cookie/header match; project frontend no longer reads or persists bearer credentials; legacy API keys are memory-only and an unset legacy key fails closed. Focused Go and JavaScript syntax checks passed. | Complete |
| 2026-07-28 | 2a | OpenAPI commit now creates disabled catalog entries only. Explicit synthetic activation is restricted to GET/HEAD; POST, PUT, PATCH, DELETE, OPTIONS, and TRACE cannot be enabled. Broad route polling is now opt-in through `ROUTE_BROAD_POLLING_ENABLED`; fresh v2 configuration does not schedule a request per imported operation. Focused application and HTTP API tests passed. | Complete |
| 2026-07-28 | 4a | Implemented the shared `.hidden`/`[hidden]` rendering contract and all five motion-plan outcomes: static brand mark, instant tab changes, semantic reduced motion, state-driven refresh pulse, and fine-pointer hover behavior for primary controls. Browser accessibility snapshots confirmed that the guest Projects view excludes the hidden authenticated shell and registration-only fields. | Complete |
| 2026-07-28 | 2b | Added one IDNA-aware structured canonicalizer for manual routes, bulk input, OpenAPI import, updates, and worker fetch-target construction. It returns stable codes/fields, preserves the established trailing-slash identity policy, and is exposed through an editor-only normalization preview endpoint with duplicate, safety, traffic, and daily-request feedback. Domain, application, and API tests passed. | Complete |
| 2026-07-28 | 2c | Added an ordered, reversible migration for project environments, nullable versioned canonical identity/hash fields, and an operator-visible collision ledger. It retains legacy route fields during the migration window. Storage migration parser tests passed; live MySQL smoke testing remains available through `MYSQL_TEST_DSN`. | Complete |
| 2026-07-28 | 2d | Manual and OpenAPI-created/updated routes now persist canonical identity, a 32-byte SHA-256 lookup hash, and canonicalizer version `1`. The MySQL route store dual-writes and reads these fields. Focused domain, application, and API tests passed. | Complete |
| 2026-07-28 | 2e | Added `cmd/backfill-endpoint-identity`: an operator-run, dry-run-capable, bounded batch backfill that uses the domain canonicalizer, records invalid legacy rows and exact canonical duplicates in the conflict ledger, and can safely resume. Duplicate detection and writes share a transaction with an index-range lock. Command compilation and domain tests passed. | Complete |
| 2026-07-28 | 2f | New projects now create a default `production` environment within the project/owner transaction. Its base URL is intentionally empty until onboarding or an integration configures it. MySQL adapter compilation and migration-storage tests passed. | Complete |
| 2026-07-28 | 2g | Added viewer-scoped environment listing and editor-scoped creation API routes. Environment base URLs and origins pass through the central backend normalizer before persistence. Focused application and API tests cover default environment creation, canonical persistence, and listing. | Complete |
| 2026-07-28 | 2h | The authenticated project view now loads and renders its environment contexts, marking the default and safely showing an unconfigured base URL when appropriate. Editors create environments in an accessible modal with inline validation errors through the secured API. JavaScript syntax and diff checks passed. | Complete |
| 2026-07-28 | Regression checkpoint | `GOCACHE=/tmp/argus-go-build go test ./...` passed after identity, canonical route, migration, and default-environment changes. | Complete |
| 2026-07-28 | 3a | Added OpenTelemetry Go API/SDK providers with Argus service resource attributes and deterministic shutdown through the application lifecycle. Exporters and authenticated OTLP ingestion remain subsequent telemetry checkpoints. Focused observability/application/HTTP-platform compilation tests passed. | Complete |
| 2026-07-28 | 3b | Added Fiber server spans plus request-count and duration-histogram metrics with method, normalized route template, response status, and duration only; raw URLs, query strings, IDs, and user data are not added as telemetry attributes. HTTP-platform and API tests passed. | Complete |
| 2026-07-28 | 3c | Added editor-managed, one-time opaque OTLP credentials bound server-side to a project and environment. SHA-256 hashes are persisted; list responses expose only a short prefix; credentials have expiry, rate-limit configuration, rotation, revocation, and last-use tracking. Application and API tests cover binding, secret non-leakage, rotation, and revocation. | Complete |
| 2026-07-28 | 3d | Added OTLP/HTTP protobuf endpoints for metrics and traces. They authenticate the opaque credential, derive project/environment only from its server-side binding, enforce payload/resource/item/rate bounds, and retain only allowlisted service and deployment-environment diagnostics. Raw attributes, measurements, span names, trace IDs, URLs, and payloads are never persisted in MySQL. Full Go tests passed. | Complete |
| 2026-07-28 | 3e | Added viewer-scoped ingestion diagnostics and a project telemetry-signals card. It shows only the safe service/deployment labels, signal type, bounded item count, and last-seen time, so operators can troubleshoot freshness without exposing raw telemetry. Focused API/application tests and JavaScript syntax checks passed. | Complete |
| 2026-07-28 | 3f | Editors can issue an OTLP credential for an existing environment in an accessible dialog, choose its expiry, and copy the one-time secret from a second warning dialog. The secret is cleared on every dismissal path and never enters persistent browser state. JavaScript syntax and full Go regression checks passed. | Complete |
| 2026-07-28 | 3g | Added a pinned VictoriaMetrics single-node Compose service and ADR covering Prometheus-compatible queries, 30-day retention, memory/disk safeguards, loopback-only local access, health checking, named-volume durability, and snapshot backup/restore. Full Go tests passed; Docker Compose validation remains unavailable because Docker is not installed in this workspace. | Complete |
| 2026-07-28 | 3h | Added a tested OTLP histogram bridge to the internal VictoriaMetrics JSON import API. Only recognized HTTP server-duration histograms are exported, with fixed project/environment/service/deployment/method/template/status labels and bounded bucket expansion; malformed histograms and unsafe dynamic numeric routes do not create series. Focused API and adapter HTTP tests passed. | Complete |
| 2026-07-28 | 3i | Added a project-scoped manual mapping control plane that binds an environment, service/deployment identity, and one catalog route. Foreign-project routes and environments are rejected before persistence; mappings snapshot the route method/template and are exposed through viewer/editor-scoped project APIs. Full Go tests passed. | Complete |
| 2026-07-28 | 3j | Added a pure, tested SLO policy that distinguishes healthy, unhealthy, no-data, stale, paused, maintenance, and configuration-error outcomes. It calculates observed performance, remaining error budget, burn rate, and a two-window burn alert without treating missing telemetry as healthy. Domain tests passed. | Complete |
| 2026-07-28 | 3k | Added project-scoped, version-one SLO definitions, immutable JSON definition snapshots, and bounded aggregate evaluation history. Viewer/editor APIs enforce project roles; availability and latency inputs, rolling/burn windows, low-traffic minimums, and provenance are validated. Full Go regression tests passed. | Complete |
| 2026-07-28 | 3l | Added a scheduled, internal-only VictoriaMetrics reader that generates fixed project-scoped MetricsQL expressions for availability and histogram latency SLIs, then persists immutable evaluator evidence. Low traffic is no-data, stale telemetry remains distinct, and metric-query failures produce configuration-error evidence. Domain and worker evaluator tests passed; adapter test compilation and diff checks passed. The loopback-enabled adapter runtime tests could not be run because the elevated permission review timed out twice. | Complete |
| 2026-07-28 | 4b | Removed the `/api` control-plane prefix. Every Argus management route now begins with a capability family and purpose (for example `/identity/login`, `/project/catalog`, `/route/catalog`, and `/telemetry/credentials`); OTLP remains on its standard `/v1/*` protocol paths. Browser clients, HTTP tests, README, user guide, implementation plan, and ADRs were migrated. Full Go regression tests and frontend syntax checks passed. | Complete |
| 2026-07-29 | 4c | Replaced new-project polling-first entry with an authenticated, four-step identity/source/review/next-actions flow. It preserves only non-sensitive project drafts locally, creates no partial project before confirmation, explicitly distinguishes passive telemetry, catalog-only OpenAPI import, disabled synthetic setup, heartbeat, and deferred setup, and directs OpenAPI users to the zero-traffic import wizard. Completion offers an explicit starter 99.9% availability SLO rather than silently creating one. Existing project editing retains advanced defaults. JavaScript syntax and diff checks passed. | Complete |
| 2026-07-29 | 4d | Added the authenticated project SLO surface over the existing viewer/editor-scoped SLO API. Editors can create bounded availability or latency definitions; all project members can inspect definition targets, rolling windows, and minimum-event policy. The UI states that no-data, stale, maintenance, and configuration errors are distinct from healthy. JavaScript syntax and diff checks passed. | Complete |
| 2026-07-29 | 4e | Replaced clickable route-table header cells with keyboard-operable buttons and synchronized `aria-sort` values. The visual sorting treatment remains unchanged while assistive technology receives the active direction. JavaScript syntax and diff checks passed. | Complete |
| 2026-07-29 | 4f | Added a shared focus trap and Escape behavior for every project dialog, including onboarding, environment, telemetry credential/secret, SLO, route, bulk, and confirmation dialogs. Opening a project dialog now makes the page chrome inert and closing it restores focus to its trigger. JavaScript syntax and diff checks passed. | Complete |
| 2026-07-29 | 3m | Exposed the tenant-safe telemetry route-mapping control plane in the project dashboard. Editors can create or remove a mapping through scoped environment, service/deployment, and catalog-route selectors; viewers can inspect the bounded mapping evidence. JavaScript syntax and diff checks passed. | Complete |
| 2026-07-28 | 1b | Added authenticated session inventory and revoke-other-sessions controls. Only the current-session marker is returned; token hashes remain server-only. Revocation removes every sibling session while retaining the session used for the request. Application and API tests passed. | Complete |
| 2026-07-28 | 1c | Added explicit Fiber read/write/idle timeouts, strict CSP and companion browser headers, a 256 KiB authentication/control payload guard, and per-IP authentication throttling with `429` responses. Platform, application, and HTTP API tests passed. | Complete |
| 2026-07-28 | 1d | Moved registration/login out of the private Projects tab into dedicated `#/register` and `#/login` routes, with Register as the primary global guest action. Guest project navigation redirects to login and only accepts a constrained same-origin `#/projects/...` return target; header actions reflect the cookie-authenticated session. JavaScript syntax, diff checks, and focused domain/API tests passed. | Complete |
| 2026-07-28 | 1e | Added CSRF-protected `POST /api/auth/password`. It verifies the current password, writes a bcrypt hash, retains the current session, and revokes sibling sessions. Focused application, API, and HTTP-platform tests passed. | Complete |
| 2026-07-28 | 1f | Added authenticated `#/account` with password-change controls, current-session-safe revoke-others action, and an active-session inventory. It uses the cookie/CSRF project client and displays no session token material. JavaScript syntax and diff checks passed. | Complete |
| 2026-07-29 | 1g | Added a generic password-recovery request/completion flow with 30-minute opaque tokens hashed at rest, atomic single-use consumption, password replacement, and revocation of every existing session. Reset tokens are delivered only through an optional operator-configured HTTPS webhook and never through API responses, browser storage, MySQL plaintext, or logs. Full Go regression tests, API/application recovery tests, JavaScript syntax, and diff checks passed. | Complete |
| 2026-07-29 | 3n | Added project-bound heartbeat monitors for scheduled workloads. Editors issue or revoke one-time opaque tokens; only token hashes and hashed per-monitor idempotency keys reach MySQL. The unauthenticated receive boundary accepts a bounded outcome only, rejects revoked/unknown tokens, and duplicate retries cannot refresh liveness. The authenticated dashboard creates monitors, reveals the secret once, and explains healthy, late, missing, and revoked states. Full Go regression tests, frontend syntax, and diff checks passed. | Complete |
| 2026-07-29 | 3o | Added source and source-key attribution, bounded evaluation evidence, and editor acknowledgement to project route incidents. Existing incidents migrate as `synthetic`; acknowledgement preserves visibility and does not interfere with automatic recovery. Application tests cover source/evidence, acknowledgement, and resolving an acknowledged incident. | Complete |
| 2026-07-29 | 3p | Connected scheduled SLO evaluation to the transactional outbox. The evaluator emits a tenant-scoped notification only for state transitions, including unhealthy and recovery transitions; repeated identical evaluations are silent. Worker tests cover unhealthy, deduplicated repeat, and recovery delivery intents. | Complete |
| 2026-07-29 | 2i | Added a versioned AES-256-GCM storage boundary for project-route request headers. New single and bulk route writes plus edits require the operator-provided `ROUTE_SECRET_ENCRYPTION_KEY` for non-empty headers and place ciphertext in an additive column; reads decrypt ciphertext while retaining a temporary legacy plaintext fallback. AEAD round-trip/wrong-key tests passed. A restartable legacy backfill and key-rotation workflow remain. | In progress |
