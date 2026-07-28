# Argus Security Best-Practices Review

Date: 2026-07-28
Scope: Go/Fiber backend, vanilla JavaScript frontend, route evaluator, authentication and monitoring architecture at commit `b576998`
Mode: evidence-based review only; no security fixes were applied in this branch

## Executive summary

Argus already has several strong controls: passwords are bcrypt-hashed, opaque session tokens are generated from `crypto/rand` and stored hashed, project APIs enforce owner/editor/viewer authorization, route checks cap time/body/redirects, and the newer route evaluator validates resolved IPs and strips credentials on cross-origin redirects.

The highest-risk gaps form three chains:

1. credentials are readable by JavaScript while the frontend contains inline event-handler construction;
2. the legacy API fails open when `API_KEY` is absent, while it can create outbound-monitoring targets;
3. imported routes are active by default and can use state-changing HTTP methods.

These should be addressed before expanding the product or exposing it as a multi-tenant service.

## Finding SEC-001 — Browser credentials are stored in `localStorage`

- Rule ID: JS-STORAGE-001
- Severity: High
- Location:
  - `frontend/projects.js:108-127` — `getToken`, `getUser`, `setSession`, `clearSession`
  - `frontend/app.js:155-158,615-636` — legacy API key storage and request header
- Evidence:

```js
return localStorage.getItem(TOKEN_KEY) || '';
localStorage.setItem(TOKEN_KEY, token);
localStorage.setItem('argus_api_key', el.apiKey.value.trim());
```

- Impact: any successful same-origin XSS, malicious third-party script or compromised browser extension with page access can read the bearer token and API key. The project token lasts 30 days, increasing replay value.
- Fix:
  - use an opaque server-side browser session in a `HttpOnly`, `Secure` production, `SameSite=Lax` or stricter cookie;
  - rotate the session after registration/login and sensitive privilege changes;
  - protect state-changing cookie-authenticated requests with CSRF controls;
  - move automation tokens into a project/account token screen; show the secret once and store only its hash server-side.
- Mitigation: until migration, shorten token TTL, deploy a strict CSP, eliminate inline handlers/unsafe DOM sinks and provide server-side revoke-all-sessions.
- False positive notes: theme preference in `localStorage` is appropriate because it is non-sensitive. The finding applies to auth/session/API credentials.

## Finding SEC-002 — Stored DOM-XSS risk from inline event-handler interpolation

- Rule ID: JS-XSS-004 / JS-XSS-001
- Severity: High
- Location: `frontend/app.js:263-278` in monitor table rendering; related inline handler at `frontend/app.js:324-335`
- Evidence:

```js
el.table.innerHTML = filtered.map((w) => `
  ...
  <button class="danger sm"
    onclick="confirmDeleteMonitor(${w.id}, '${escapeHtml(w.url).replace(/'/g, "\\'")}')">
    Delete
  </button>
`).join('');
```

- Impact: HTML escaping is not JavaScript-string escaping. Character references such as `&#39;` are decoded by the HTML parser before the inline handler is compiled. A crafted stored URL containing quote/JavaScript-context characters can potentially break out of the string when the handler is executed. If the legacy API is unauthenticated (SEC-003), an external attacker may be able to seed the value; otherwise an authorized writer or compromised import path can target another dashboard user.
- Fix:
  - remove inline `onclick`;
  - render inert text/data using `textContent`/DOM construction;
  - store only numeric IDs in `data-*`;
  - use one delegated `addEventListener` and read the URL from the already-held state by ID, not from executable markup.
- Mitigation: strict CSP without `unsafe-inline` blocks inline handlers. CSP is defense in depth and does not replace the code change.
- False positive notes: exploitability depends on the exact URL validation and who can create monitors. The context mismatch is still unsafe and should be removed even if present validation makes a specific payload difficult.

## Finding SEC-003 — Legacy API authentication fails open when `API_KEY` is empty

- Rule ID: GO-AUTHZ-001 / secure-default configuration
- Severity: High
- Location:
  - `internal/adapters/inbound/http/middleware.go:10-20` — `APIKeyAuth`
  - `internal/config/config.go:49-53` — `APIKey: os.Getenv("API_KEY")`
  - `.env.example:1-6` — no required `API_KEY`
- Evidence:

```go
if apiKey == "" {
    return c.Next()
}
```

- Impact: a deployment that omits one environment variable exposes legacy create/update/delete monitoring APIs and logs without authentication. Those APIs can trigger server-side outbound requests, so impact includes data modification, SSRF attempts, resource consumption and the XSS chain in SEC-002.
- Fix:
  - fail startup in production when a legacy API is reachable without a configured auth mode;
  - migrate all private endpoints to the user/project session and scoped automation-token model;
  - if an explicit local development bypass is required, gate it behind a separate `DEV_ALLOW_UNAUTHENTICATED=true`, loopback bind and loud startup warning.
- Mitigation: reverse-proxy authentication and network allowlisting.
- False positive notes: a strictly local, loopback-only development deployment may intentionally omit auth. The default bind is `:8080`, not loopback-only, so omission is unsafe by default.

## Finding SEC-004 — Auth endpoints have no visible rate limiting

- Rule ID: GO-AUTHN-002
- Severity: High for internet exposure; Medium for trusted-network self-hosting
- Location:
  - `internal/api/auth_handler.go:17-53` — public register/login routes
  - `internal/platform/httpserver/fiber.go:23-29` — global middleware stack
- Evidence: registration and login are mounted publicly; the middleware stack includes recover, helmet, ETag and compression, but no limiter. A repository-wide search found no Fiber limiter or equivalent auth throttle.
- Impact: credential stuffing, password guessing, account-creation abuse and bcrypt CPU exhaustion. A 15 MiB global body limit is much larger than auth payloads need.
- Fix:
  - distributed limiter backed by Redis for multi-instance consistency;
  - combined IP/prefix and normalized-account buckets, with care not to make account-lockout denial easy;
  - progressive delay, audit metrics and `429`/`Retry-After`;
  - endpoint-specific small body limits.
- Mitigation: edge/WAF rate limit and connection limits.
- False positive notes: a reverse proxy may enforce a limiter outside this repository. Verify the deployed edge configuration before closing the finding.

## Finding SEC-005 — Authentication lifecycle is incomplete for an account-based product

- Rule ID: GO-AUTHN-003
- Severity: Medium
- Location:
  - `internal/application/auth.go:27-52` — minimum eight-character password only
  - `internal/application/auth.go:35,119-137` — fixed 30-day bearer token
  - `internal/api/auth_handler.go:17-76` — register/login/logout/me only
- Evidence:

```go
const tokenTTL = 30 * 24 * time.Hour
if len(password) < 8 { ... }
```

- Impact: long replay window, no password reset/email verification, no session inventory or revoke-all flow, no credential breach screening, and no explicit maximum password byte length before bcrypt. Product users cannot recover securely from compromise or forgotten credentials.
- Fix:
  - session family metadata, rotation, revoke current/all, last-used/device display;
  - reset and verification tokens that are single-use, short-lived and stored hashed;
  - accept long passphrases, define a bcrypt-compatible maximum in bytes and use a current password policy;
  - consider breached-password screening without logging password material.
- Mitigation: shorten TTL and expose an administrator revoke mechanism.
- False positive notes: email delivery may intentionally be out of MVP scope, but an internet-facing signup flow needs a documented recovery/verification decision.

## Finding SEC-006 — Legacy outbound checks do not use the hardened route transport

- Rule ID: GO-SSRF-001
- Severity: High
- Location:
  - `internal/worker/processor.go:154-185` — `http.DefaultClient.Do`
  - contrast: `internal/worker/route_evaluator.go:137-155,159-258` — hardened dialer and redirect validation
- Evidence:

```go
if err := validateTarget(target); err != nil { ... }
resp, err := http.DefaultClient.Do(req)
```

- Impact: validating a URL before `http.DefaultClient` performs its own DNS lookup does not guarantee the dialed IP remains allowed. Default redirects are also not checked with the newer evaluator's per-hop target policy. This creates DNS-rebinding and redirect-to-private/metadata risk, especially when chained with fail-open legacy authentication.
- Fix:
  - use one shared hardened transport for every active check;
  - validate the resolved IP in `DialContext`/connection control;
  - revalidate every redirect and strip credentials cross-origin;
  - prefer an outbound-only private probe agent for customer private networks.
- Mitigation: egress firewall denying metadata, loopback, link-local and private ranges from public probe workers.
- False positive notes: external egress policies may block the destination at the network layer. Verify them; application parity is still required.

## Finding SEC-007 — Active checks permit unsafe methods and retries by default

- Rule ID: GO-BUSINESS-LOGIC-001
- Severity: Critical for production targets with usable credentials; otherwise High
- Location:
  - `internal/domain/route.go:95-104` — permits POST/PUT/PATCH/DELETE/TRACE
  - `internal/application/routes.go:114-145` — new route defaults enabled
  - `internal/application/imports.go:181-192` — imported route defaults enabled
  - `internal/worker/route_evaluator.go:289-330` — retries the same method
- Evidence:

```go
case "GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS", "TRACE":
...
Source: "import", Enabled: true,
...
for attempt := 1; attempt <= retries+1; attempt++ {
    outcome = e.attempt(ctx, route.Method, ...)
}
```

- Impact: monitoring can create, mutate or delete production data. Repeating non-idempotent POST/PATCH calls can multiply side effects. An OpenAPI import can turn a catalog action into thousands of real requests without a separate informed-consent step.
- Fix:
  - separate endpoint catalog from monitor policy;
  - import/create with synthetic disabled;
  - recommend GET/HEAD canaries only;
  - require sandbox, scoped fixture, idempotency and cleanup for state-changing exceptions;
  - make retries method/policy-aware and budgeted.
- Mitigation: immediately block unsafe methods at scheduler dispatch and revoke route-check credentials.
- False positive notes: a dedicated test API may make these methods safe by contract. The system currently cannot prove or enforce that contract, so the default is unsafe.

## Finding SEC-008 — CSP/HSTS are not explicitly configured and inline handlers prevent a strict policy

- Rule ID: JS-CSP-001 / JS-CSP-002
- Severity: Medium
- Location:
  - `internal/platform/httpserver/fiber.go:23-29` — `helmet.New()` with default config
  - `frontend/index.html:12-16` — remote Google Fonts
  - `frontend/app.js:276-277,333` — inline handlers
- Evidence:

```go
app.Use(helmet.New())
```

No explicit CSP/HSTS policy is visible in repository configuration. Current Fiber guidance warns that production applications should explicitly configure these rather than assume defaults. Inline event handlers require weakening `script-src` or will stop working under a strict policy.

- Impact: reduced defense in depth against XSS/clickjacking/resource injection and possible transport-downgrade exposure if the edge also omits HSTS.
- Fix:
  - remove inline handlers;
  - self-host fonts or narrowly allow required origins;
  - set header-delivered CSP, `frame-ancestors`, HSTS at the TLS edge, `nosniff`, referrer and permissions policies;
  - begin with report-only at the edge, then enforce.
- Mitigation: edge-configured headers; verify with an integration test against the deployed response.
- False positive notes: an ingress/CDN may already supply CSP/HSTS. They are not visible here and must be verified at runtime.

## Finding SEC-009 — Server request timeouts are not explicit

- Rule ID: GO-HTTP-001
- Severity: Medium
- Location:
  - `internal/platform/httpserver/fiber.go:23-24` — Fiber config
  - `internal/app/application.go:71-75` — raw listener
- Evidence:

```go
app := fiber.New(fiber.Config{
    AppName: "Argus Distributed Uptime Checker",
    BodyLimit: maxUploadBytes,
})
```

- Impact: absent explicit read/write/idle timeouts, slow or stalled clients may hold resources longer than intended. A single 15 MiB body limit also applies to small auth/CRUD requests.
- Fix: set and load-test `ReadTimeout`, `WriteTimeout`, `IdleTimeout`, header/body limits and concurrency limits appropriate to uploads versus normal API routes.
- Mitigation: enforce tighter ingress timeouts and request sizes.
- False positive notes: Fiber/fasthttp and the reverse proxy have defaults that may partially limit exposure; explicit application/edge contracts are still needed.

## Finding SEC-010 — Secret-bearing route headers are stored as plaintext JSON

- Rule ID: GO-SECRETS-001
- Severity: Medium to High, depending on database access model
- Location:
  - `internal/application/routes.go:143-173` — accepts header JSON and only redacts on read
  - `internal/worker/route_evaluator.go:323-327` — sends stored headers
  - route persistence under `internal/adapters/outbound/mysql/`
- Evidence: comments state that redaction occurs on read. The evaluator necessarily receives plaintext values from storage.
- Impact: database backup/read compromise exposes Authorization, Cookie and API-key material for monitored services. Redaction in API responses does not protect data at rest.
- Fix:
  - store secret references or envelope-encrypted values with key rotation;
  - separate non-secret header metadata from secret values;
  - least-privilege monitor credentials and one-time display;
  - audit access without logging values.
- Mitigation: encrypted disk/backup, tightly scoped DB account and rapid credential rotation.
- False positive notes: infrastructure-level encryption at rest may exist, but it does not protect against a compromised application/DB reader and is not visible in this repository.

## Positive controls observed

- `internal/application/auth.go:140-150`: 256-bit random tokens and SHA-256 hashes at rest.
- `internal/application/auth.go:52,82`: bcrypt password hashing and constant library comparison.
- `internal/application/auth.go:99-117`: expiry and constant-time token-hash equality check.
- `internal/worker/route_evaluator.go:159-258`: dial-time IP validation, redirect caps and cross-origin secret stripping.
- `internal/platform/httpserver/fiber.go:21-28`: bounded body, panic recovery, Helmet, ETag and compression.
- API tests cover authorization matrices and secret redaction.

## Recommended remediation order

1. SEC-007: stop unsafe/default-on requests.
2. SEC-003 + SEC-006: fail closed and unify the outbound transport.
3. SEC-002 + SEC-001: eliminate inline handler injection and browser-readable auth credentials.
4. SEC-004 + SEC-005: complete account/session abuse controls.
5. SEC-010: encrypted/reference-based monitor secrets.
6. SEC-008 + SEC-009: explicit production HTTP security baseline.

## Verification plan

- security unit/integration tests for every finding;
- browser test proving no credential exists in Web Storage;
- CSP enforcement test with zero violations in primary flows;
- DNS-rebinding and redirect SSRF harness;
- unsafe-method import test proving zero outbound requests;
- distributed auth rate-limit test;
- cross-tenant authorization suite;
- secret scan over logs, API responses and backups/fixtures;
- deployment header/timeouts verification at the real edge.

## References

- [OWASP Session Management Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Session_Management_Cheat_Sheet.html)
- [OWASP DOM-based XSS Prevention](https://cheatsheetseries.owasp.org/cheatsheets/DOM_based_XSS_Prevention_Cheat_Sheet.html)
- [OWASP SSRF Prevention](https://cheatsheetseries.owasp.org/cheatsheets/Server_Side_Request_Forgery_Prevention_Cheat_Sheet.html)
- [OWASP ASVS 5.0](https://owasp.org/www-project-application-security-verification-standard/)
- [RFC 9110 safe/idempotent methods](https://www.rfc-editor.org/rfc/rfc9110.html)
- [Fiber documentation](https://docs.gofiber.io/)
