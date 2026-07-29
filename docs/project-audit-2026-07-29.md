# Argus Comprehensive Engineering Audit

**Date:** 2026-07-29  
**Scope:** Full repository review of the Go backend, browser dashboard, authentication, workers, network egress, storage/migrations, Docker development configuration, documentation, and automated-test setup.  
**Assessment type:** Code and configuration audit. No production data, deployment host, or live third-party target was accessed.

## Executive summary

Argus has a well-structured Go codebase, substantial backend test coverage, CSRF protection for cookie-authenticated requests, tenant-scoped project APIs, bounded OpenAPI parsing, and a strong dial-time SSRF policy for synthetic route checks. The baseline Go checks are healthy:

| Check | Result |
| --- | --- |
| `go test ./...` | Passed: 371 tests across 25 packages |
| `go vet ./...` | Passed |
| `go build ./cmd/...` | Passed |
| JavaScript syntax checks | Passed for `frontend/app.js` and `frontend/projects.js` |
| `go test -cover ./...` | Passed |

However, the release has one immediate browser regression and several high-priority functional and security issues. The most urgent issue is a contradiction between the CSP header and the dashboard implementation: the header blocks the inline event handlers used by the legacy monitor UI. In a standards-compliant browser, delete, heartbeat, and public-link-copy actions do not execute.

The highest-priority remediation order is:

1. Replace all inline event handlers with registered/delegated JavaScript listeners.
2. Repair the public status-page route and decide whether it is an API payload or a shareable HTML status view.
3. Bind MySQL and Redis to loopback/internal networking and remove development credentials from any production path.
4. Harden webhook delivery at DNS resolution/dial time.
5. Make the quick-start authentication and cookie configuration work without hidden manual steps.

## Review method and limitations

The review combined source inspection, route and data-flow tracing, configuration review, and the checks listed above. `govulncheck` could not complete because this execution environment blocks outbound DNS/network access to the Go vulnerability database. `staticcheck ./...` was not usable through the environment command proxy, so it should run in CI. Browser/E2E tests could not be executed here because the required npm execution environment was not available; this is also a project gap because no browser test suite or visible CI workflow is present in the repository.

Severity definitions:

- **P0:** Release blocker; a primary user flow is broken now.
- **P1:** High-impact functional, security, or deployment issue; fix before exposing the service or relying on the affected feature.
- **P2:** Important reliability, security-hardening, or maintainability issue; schedule in the next engineering cycle.
- **P3:** Lower-risk correctness, product-semantics, or documentation issue.

## Findings

### P0 — CSP blocks working legacy-dashboard actions

**Evidence**

- `internal/platform/httpserver/fiber.go` sends `Content-Security-Policy: ... script-src 'self' ...`, which correctly disallows inline script and inline event-handler execution.
- `frontend/app.js` renders inline `onclick` attributes for monitor heartbeat, monitor deletion, and status-link copying (for example, the handlers around the monitor-table and status-page renderers).

**Impact**

Modern browsers block those inline handlers. The affected controls render, but clicking them does nothing: users cannot delete a legacy monitor, trigger its manual heartbeat, or copy a status-page URL. This is a visible regression in core dashboard behavior.

**Required fix**

Keep the strict CSP. Remove inline `onclick` markup and use event delegation or explicit `addEventListener` registrations with `data-action` and `data-id` attributes. Add a Playwright test that clicks each affected control under the production CSP header. Do not weaken CSP with `'unsafe-inline'` merely to preserve the current implementation.

### P1 — “Public” status pages are neither publicly reachable nor rendered as public pages

**Evidence**

- `internal/platform/httpserver/fiber.go` registers every `FeatureHandler` route with the legacy `APIKeyAuth` guard.
- `internal/api/feature_handler.go` consequently guards `GET /status/public/:slug` as well.
- `APIKeyAuth` returns `401` without the configured key and returns `503` when `API_KEY` is unset.
- The dashboard copies `/status/public/{slug}` as a public sharing link, but the handler returns a JSON API payload rather than an HTML status view.
- `GetPublicStatusPage` does not enforce `page.IsPublic`; a future private status page would be returned if this route were simply made unauthenticated.

**Impact**

The advertised shareable status-page capability is broken. A recipient of the copied link cannot access it without an administrator key, and even after authorization receives raw JSON rather than the promised status view. A naïve guard removal would introduce private-page disclosure.

**Required fix**

Split public-status registration from management routes. Serve either a deliberately designed public HTML view or rename/document it as a public JSON endpoint and change the dashboard copy text. Enforce `IsPublic` in the service/handler, return `404` for missing or private slugs, and add unauthenticated integration tests for public, private, missing, and management endpoints.

### P1 — Quick start leaves the legacy monitor dashboard disabled

**Evidence**

- `.env.example` does not define `API_KEY`.
- `internal/adapters/inbound/http/middleware.go` deliberately fails closed when `API_KEY` is empty.
- The quick-start command starts the application with the sample/default configuration.
- The legacy dashboard sends the API-key field with an empty value until an operator supplies one.

**Impact**

After following the documented quick start, legacy monitor, alert, maintenance, status-page, log, and check-history requests receive `503`. The project/authentication UI may work, but the original uptime-monitoring dashboard does not. Documentation suggests that the key is optional, while the implementation makes it mandatory for this subsystem.

**Required fix**

Choose and document one clear onboarding model: require `API_KEY` at startup with an actionable error and add it to the setup flow, or migrate legacy controls to user/session authorization. Add a smoke test starting from `.env.example` and exercising a legacy monitor request.

### P1 — Alert-channel webhook validation does not prevent DNS rebinding

**Evidence**

- `internal/application/service.go` validates webhook URL scheme and literal private IP addresses, but does not validate the IP address resolved for a hostname.
- `internal/adapters/outbound/notifier/http_notifier.go` uses a normal HTTP transport without a dial-time destination policy.
- Redirects are correctly disabled, but that does not protect a hostname that resolves or rebinds to a private address at connection time.

**Impact**

An actor able to create an alert channel, or an attacker using a leaked legacy API key, can point a hostname at loopback, private-network, or cloud-metadata addresses through DNS rebinding. The Argus server then sends incident payloads to an internal target.

**Required fix**

Use the same dial-time network policy already implemented by `internal/worker/route_evaluator.go`, or extract it into a shared outbound-egress package. Validate every resolved IP immediately before connecting; block loopback, private, link-local, metadata, multicast, and reserved ranges; keep redirects disabled; and add DNS-rebinding and private-address tests. The route evaluator itself is a positive example: it already validates resolved addresses and redirect targets.

### P1 — Docker Compose exposes MySQL and Redis on all host interfaces

**Evidence**

- `docker-compose.yml` publishes MySQL as `3306:3306` and Redis as `6379:6379`.
- MySQL is configured with development credentials and Redis has no password in the Compose configuration.
- VictoriaMetrics is correctly bound to `127.0.0.1`, showing the intended safer pattern is already used elsewhere.

**Impact**

On a host with a reachable network interface and permissive firewall, MySQL and Redis can be accessed from the LAN or internet. This risks database compromise, queue manipulation, and service takeover.

**Required fix**

For local development, bind the ports explicitly to `127.0.0.1`; for deployment, do not publish them and place the application and data services on an internal Compose network. Use environment-provided secrets for production and provide a separate production Compose/deployment definition.

### P1 — Local HTTP authentication is easy to misconfigure and can loop back to login

**Evidence**

- `internal/config/config.go` defaults `AUTH_COOKIE_SECURE` to `true`.
- The quick start uses `http://localhost:8080`.
- `.env.example` does not set `AUTH_COOKIE_SECURE=false` for local HTTP development.
- `frontend/projects.js` already contains user-facing guidance for this exact situation.

**Impact**

On browser/platform combinations that do not retain or resend Secure cookies on HTTP, registration or login appears to succeed but the next authenticated request returns the user to the login page. This matches the reported sign-in behavior.

**Required fix**

Keep the production default as `true`, add `AUTH_COOKIE_SECURE=false` to the development `.env.example`, and emit a startup warning when a Secure cookie is configured behind a plainly HTTP listener. Add a browser test for register → session restore → authenticated dashboard on local HTTP and on HTTPS.

### P2 — Status-page input is not validated and is rendered as raw HTML

**Evidence**

- `CreateStatusPage` in `internal/application/service.go` passes `Slug` and `Title` directly to storage without validation.
- `frontend/app.js` inserts `p.slug` directly into an `innerHTML` template in the status-page list.
- The slug is also interpolated into an inline event handler.

**Impact**

An administrator-key holder can persist arbitrary markup in a slug and inject it into every legacy-dashboard viewer’s DOM. The current strict CSP mitigates direct inline-script execution, but it does not make raw HTML injection acceptable and it is especially risky if CSP is ever relaxed, a same-origin script gadget exists, or the markup changes the page structure.

**Required fix**

Validate slugs server-side (for example, lower-case ASCII letters, digits, and hyphens with a bounded length) and validate title length/non-blank input. Render every dynamic value through text nodes or robust attribute escaping; do not interpolate it into JavaScript source. Add negative tests for markup, whitespace, slash, quote, duplicate, and overlong slugs.

### P2 — Browser authentication returns a reusable 30-day bearer token to JavaScript

**Evidence**

- `internal/api/auth_handler.go` returns the raw token in the JSON bodies of register and login responses.
- The browser client uses the HttpOnly cookie and does not need this response field.
- Session tokens last 30 days in `internal/application/auth.go`.

**Impact**

The stated HttpOnly-cookie boundary is weakened because a browser script that can observe the authentication response can obtain a reusable bearer credential. The dashboard does not persist it, which is good, but it is still unnecessarily exposed to in-page JavaScript and browser tooling.

**Required fix**

Separate browser session issuance from programmatic bearer-token issuance, or make token inclusion an explicit non-browser opt-in. The normal browser register/login response should contain the user only. Add tests that assert no raw session token appears in cookie-authenticated browser responses.

### P2 — Worker persistence and outbox-state errors are discarded

**Evidence**

- In `internal/worker/processor.go`, errors from `RecordCheck` and `ProcessIncidentTransition` are ignored after `MarkChecked` succeeds.
- The same file ignores errors from `MarkFailed` and `MarkProcessed` in outbox dispatch.

**Impact**

A task may be acknowledged as successful while check history is absent, incidents are not opened/resolved, or notification state is not durably updated. This can cause missing alerts, repeated notifications, or an inconsistent operational timeline.

**Required fix**

Propagate the error to Asynq when retrying is safe, or implement a durable idempotent retry policy per step. Log/metric every failed transition and add store-failure tests for check recording, incident transition, and both outbox state changes.

### P2 — Invalid configuration silently falls back to defaults

**Evidence**

- `mustInt`, `mustBool`, and `mustDuration` in `internal/config/config.go` return the fallback on parse failure.
- Most important timeouts, concurrency limits, queue weights, and the cookie-security setting use these helpers.

**Impact**

A typo such as an invalid timeout or `AUTH_COOKIE_SECURE=ture` changes operational behavior silently. Operators may believe a configured safety limit is active when the application is using an unrelated default.

**Required fix**

Parse configuration into values plus errors, aggregate errors at startup, and fail before opening listeners for invalid safety-sensitive settings. If backward compatibility requires fallbacks for a small subset, log a high-visibility warning naming the invalid variable. Add table-driven invalid-environment tests.

### P2 — Startup and frontend serving depend on the process working directory

**Evidence**

- `internal/app/application.go` loads migrations from the relative path `db/migrations`.
- `internal/platform/httpserver/fiber.go` serves `./frontend` and `./frontend/index.html`.

**Impact**

A compiled binary started by systemd, a release directory, or a container with a different working directory can fail migration startup or serve a broken/empty dashboard. The binary is not self-contained.

**Required fix**

Embed frontend assets and migrations with `go:embed`, or use validated configurable absolute paths. Add an integration test that runs the built binary from outside the repository root.

### P2 — Several list endpoints accept unbounded or invalid pagination values

**Evidence**

- Multiple handlers use `strconv.Atoi` while discarding errors and pass the values to SQL `LIMIT`/`OFFSET`; examples include website, incident, status-page, route, route-check, SLO, telemetry-ingress, and project-incident list handlers.
- Only the in-memory log endpoint performs positive-limit validation.

**Impact**

Clients can request excessively large result sets or invalid/negative values. Depending on MySQL behavior, this produces expensive reads, errors, or inconsistent API responses. This is particularly problematic for check and incident history tables that grow continuously.

**Required fix**

Create a shared pagination parser that rejects malformed/negative values and caps limits per resource. Apply it to every handler and add boundary tests for omitted, zero, negative, non-numeric, and oversized values.

### P2 — No automated browser regression suite or visible CI pipeline

**Evidence**

- No `package.json`, browser/E2E test specification, or visible GitHub Actions workflow was found.
- Backend tests exercise substantial API behavior, but cannot detect CSP-vs-DOM regressions, login redirects, history routing, or interactive control failures.

**Impact**

Frontend failures can be merged and released even while every Go test passes. The CSP regression is an example of the class of issue that server-side tests do not catch.

**Required fix**

Add CI for `go test ./...`, `go vet ./...`, `staticcheck ./...`, JavaScript syntax/lint checks, and dependency vulnerability scanning. Add Playwright tests for register/login/session restore, navigation without hash routes, CSP-protected monitor actions, status-page sharing, and direct route refresh.

### P3 — Legacy monitor semantics can report misleading health information

**Evidence**

- `checkKeyword` in `internal/worker/processor.go` treats a response as healthy when the configured text exists, regardless of HTTP status. A 500 error page containing that text is therefore `up`.
- When `HealthCheckURL` is used, the incident transition receives the monitor’s original URL rather than the actual checked URL.
- TLS-expiry monitors accept an HTTP URL and then initiate TLS to its host, which is surprising configuration behavior.

**Impact**

Alerts can identify the wrong endpoint and keyword monitoring can mask failed HTTP responses. TLS configuration is ambiguous for operators.

**Required fix**

Make keyword success require the expected status range as well as the keyword (or document a deliberate exception), use the effective check URL in history/incident payloads, and require an HTTPS URL for TLS-expiry monitors. Add focused tests.

### P3 — Documentation conflicts with implementation in several places

**Evidence**

- `USER_GUIDE.md` tells users to write the legacy API key to `localStorage`, while `frontend/app.js` intentionally keeps it in memory only.
- Quick start treats `API_KEY` as optional even though the legacy API is disabled without it.
- README lists email among notification channel API capabilities, while the delivery adapter is not implemented; another README section describes that limitation correctly.

**Impact**

Users follow instructions that do not work, may store a management secret in XSS-readable storage, and can expect unavailable functionality.

**Required fix**

Update all onboarding and API documentation to describe the current security model and delivered capabilities exactly. Remove the `localStorage` command. Maintain a short compatibility/feature-status table so API-model fields are not mistaken for implemented delivery features.

## Prioritized remediation plan

| Phase | Work | Acceptance criteria |
| --- | --- | --- |
| Release blocker | Replace inline handlers while retaining strict CSP | Browser tests prove delete, heartbeat, and copy actions work; CSP has no `'unsafe-inline'` |
| Before exposure | Repair public status access/rendering and `IsPublic` enforcement | Anonymous public slug succeeds; private/missing slugs return 404; management endpoints remain protected |
| Before exposure | Restrict Compose data-service networking and harden webhook egress | MySQL/Redis are not reachable from non-loopback networks; DNS-rebinding test is blocked at dial time |
| Next cycle | Fix `.env.example` onboarding, browser token exposure, and config parsing | Local HTTP login persists correctly; browser responses omit raw token; invalid critical env values fail startup |
| Next cycle | Propagate worker errors, cap pagination, remove CWD dependence | Store-failure and pagination tests pass; release binary works outside repository root |
| Continuous | Add CI and browser E2E coverage; align docs | Every pull request runs quality gates and key user journeys |

## Strengths worth preserving

- Project API authorization is tenant-scoped and has substantial authorization tests.
- Cookie-authenticated unsafe requests have a readable CSRF token and server-side CSRF verification.
- Synthetic route evaluation uses bounded response reads, timeout caps, redirect validation, credential stripping across origin changes, and dial-time IP validation; this is the right baseline for other outbound clients.
- OpenAPI import explicitly avoids remote `$ref` fetching and enforces document, operation, depth, and reference-resolution limits.
- Backend unit and integration coverage is meaningful; the main gap is cross-layer/browser validation rather than an absence of Go tests.

## Follow-up verification

After the P0/P1 fixes, run the complete test suite in CI with a normal network-enabled environment, including `staticcheck ./...`, `govulncheck ./...`, and browser tests. Then perform a deployment-specific review of reverse-proxy TLS, firewall rules, secrets injection, backup access, and the actual production Compose/Kubernetes manifests. This report does not apply code changes; it records the defects and the evidence needed to close them.
