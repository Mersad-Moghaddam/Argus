# Argus Security Best-Practices Review

Date: 2026-07-28

Baseline: `b576998`

Scope: Go/Fiber backend, vanilla JavaScript frontend, authentication,
authorization, outbound monitoring, OpenAPI import, MySQL/Redis worker paths

Mode: review and remediation plan only

## Executive summary

Argus has several strong controls worth retaining:

- passwords use bcrypt;
- opaque session tokens use `crypto/rand` and are stored hashed;
- project roles are checked centrally and hide project existence;
- SQL repositories use parameterized queries;
- route checks bound time, response bytes, redirects, and retries;
- the route evaluator validates resolved IP addresses at dial time, revalidates
  redirects, blocks metadata targets, and strips secrets on cross-origin
  redirects;
- Asynq task uniqueness limits duplicate route work.

The main risks are connected:

1. browser credentials are script-readable while unsafe inline-handler DOM
   construction exists;
2. legacy management routes fail open when `API_KEY` is missing;
3. the system can repeatedly send state-changing methods to user-selected
   targets;
4. authentication and outbound work lack sufficient abuse budgets.

Treat all High findings as release blockers for internet-facing or multi-tenant
operation.

## SEC-001 — Browser credentials are stored in `localStorage`

- Rule ID: JS-STORAGE-001
- Severity: High
- Location:
  - `frontend/projects.js:108-127`
  - `frontend/app.js:154-158,615-636`
- Evidence:

```js
return localStorage.getItem(TOKEN_KEY) || '';
localStorage.setItem(TOKEN_KEY, token);
localStorage.setItem('argus_api_key', el.apiKey.value.trim());
```

- Impact: any same-origin XSS or compromised third-party script can extract a
  30-day bearer token and the legacy API key.
- Fix: use an opaque server-side session in an `HttpOnly`, `Secure`,
  `SameSite=Lax` or stricter cookie; rotate it after login and privilege
  changes; add CSRF controls; use scoped, expiring, display-once automation
  tokens outside the browser session.
- Mitigation: shorten TTL, remove unsafe DOM/inline handlers, deploy strict CSP,
  and add revoke-all-sessions.
- False-positive notes: non-sensitive preferences such as theme may remain in
  `localStorage`.

## SEC-002 — Stored DOM-XSS risk from executable string interpolation

- Rule ID: JS-XSS-001 / JS-XSS-004
- Severity: High
- Location: `frontend/app.js:263-278`
- Evidence:

```js
el.table.innerHTML = filtered.map((w) => `
  <button onclick="confirmDeleteMonitor(
    ${w.id},
    '${escapeHtml(w.url).replace(/'/g, "\\'")}'
  )">Delete</button>
`).join('');
```

- Impact: HTML escaping is not JavaScript-string escaping. Character references
  are decoded when inline handler source is compiled, creating an unsafe output
  context. A stored target value may execute in another dashboard session.
- Fix: remove inline `onclick`; render inert text with `textContent`; place only
  a validated numeric ID in `data-*`; use delegated `addEventListener`; retrieve
  the corresponding object from trusted in-memory state.
- Mitigation: strict CSP without `unsafe-inline`.
- False-positive notes: exploitability depends on current URL validation and
  write access, but the context mismatch remains insecure.

## SEC-003 — Legacy management authentication fails open

- Rule ID: GO-AUTHZ-001 / secure-default configuration
- Severity: High
- Location:
  - `internal/adapters/inbound/http/middleware.go:11-20`
  - `internal/config/config.go:49-62`
- Evidence:

```go
if apiKey == "" {
    return c.Next()
}
```

- Impact: omitting one variable exposes management, logs, and outbound-monitor
  creation. Default `:8080` binding is not loopback-only.
- Fix: unify management routes behind the user/project identity model; reject
  production startup without a secure auth mode; if a local bypass is required,
  require an explicit development flag and loopback bind.
- Mitigation: reverse-proxy authentication and network allowlisting.
- False-positive notes: only a deliberately loopback-only development process
  can reasonably accept the bypass.

## SEC-004 — Authentication endpoints have no visible abuse controls

- Rule ID: GO-AUTHN-002
- Severity: High for internet exposure; Medium for trusted private deployment
- Location:
  - `internal/api/auth_handler.go:16-54`
  - `internal/platform/httpserver/fiber.go:23-28`
- Evidence: register/login are public and the middleware stack has no rate
  limiter. Bcrypt makes uncontrolled attempts CPU-expensive.
- Impact: credential stuffing, account enumeration pressure, password guessing,
  and CPU denial of service.
- Fix:
  - use layered per-IP, per-account/email, and global token-bucket limits;
  - return generic login failures;
  - add exponential delay without holding server workers;
  - log security events without passwords/tokens;
  - support edge/WAF controls but enforce application limits too.
- Mitigation: trusted-network exposure and reverse-proxy limits.
- False-positive notes: no external proxy configuration exists in the
  repository, so it cannot be assumed.

## SEC-005 — State-changing route methods can be probed and retried

- Rule ID: GO-SSRF-002 / operational safety
- Severity: Critical for production credentials; High otherwise
- Location:
  - `internal/domain/route.go:95-104`
  - `internal/worker/route_evaluator.go:266-345`
  - `frontend/index.html:396-401`
- Evidence: `POST`, `PUT`, `PATCH`, `DELETE`, and `TRACE` are allowed; the
  evaluator sends the configured method and can repeat it `retries + 1` times.
- Impact: create/update/delete side effects, repeated charges or messages,
  account changes, and misleading failures because body-dependent operations
  are sent with no request body.
- Fix: separate endpoint catalog from synthetic checks; import creates no
  traffic; allow GET/HEAD only by default; prohibit TRACE; require an isolated
  fixture/idempotency/cleanup policy for any explicit unsafe check; never
  automatically retry a non-idempotent action.
- Mitigation: immediately disable existing mutating routes and audit historical
  checks.
- False-positive notes: some services implement idempotent POST, but Argus
  cannot infer that property from the HTTP method or OpenAPI document.

## SEC-006 — Route credentials are stored as plaintext JSON

- Rule ID: GO-SECRETS-001
- Severity: High
- Location:
  - `db/migrations/0005_projects.up.sql:45-83`
  - `internal/application/routes.go:154-193`
  - `frontend/index.html:449-452`
- Evidence: the `headers` JSON column stores request headers required by the
  worker. Sensitive values are masked on read, but masking is not encryption.
- Impact: database backup/read compromise reveals Authorization, API-key, and
  Cookie credentials for monitored services.
- Fix: store encrypted secret objects through a dedicated secret provider;
  reference secret IDs from synthetic configuration; envelope-encrypt with
  key-version metadata; support rotation/revoke/audit; never return secret
  values after creation.
- Mitigation: database encryption, least-privilege DB accounts, backup access
  controls, shorter-lived target credentials.
- False-positive notes: read redaction is a valuable disclosure control but
  does not protect database contents.

## SEC-007 — Route validation is late and malformed headers fail silently

- Rule ID: GO-VALIDATION-001
- Severity: Medium
- Location: `internal/application/routes.go:105-166,196-236`
- Evidence:

```go
BaseURL: strings.TrimRight(strings.TrimSpace(input.BaseURL), "/")
...
if err := json.Unmarshal([]byte(headers), &probe); err != nil {
    return ""
}
```

- Impact: invalid targets enter persistence/queues and fail later; malformed
  header JSON becomes an empty value rather than a clear validation error.
- Fix: centralize canonical validation in create/update/import/bulk paths;
  return stable field errors; use the same final URL builder for preview and
  execution; reject malformed headers.
- Mitigation: execution-time validation already fails closed for unsafe route
  targets.
- False-positive notes: this is primarily an integrity and operability issue
  because the newer evaluator contains strong runtime SSRF controls.

## SEC-008 — Global body limit and server timeouts are not endpoint-specific

- Rule ID: GO-DOS-001 / GO-NET-002
- Severity: Medium
- Location: `internal/platform/httpserver/fiber.go:18-28`
- Evidence: every route inherits the 15 MiB upload allowance; Fiber server
  read/write/idle timeouts are not explicitly configured.
- Impact: small auth/JSON endpoints accept unnecessarily large requests and
  slow clients may consume resources longer than intended.
- Fix: keep a bounded upload route for OpenAPI, but enforce much smaller
  auth/control JSON limits; configure read, write, idle, and header/read-buffer
  constraints; apply decompression and multipart budgets.
- Mitigation: reverse-proxy timeouts/body limits.
- False-positive notes: fasthttp/Fiber internals provide protections, but
  explicit application limits are required for predictable deployment.

## SEC-009 — Every authenticated request writes session last-used time

- Rule ID: GO-DOS-002 / session lifecycle
- Severity: Medium
- Location:
  - `internal/application/auth.go:97-125`
  - `internal/adapters/outbound/mysql/users.go:64-66`
- Evidence: `Authenticate` calls `TouchToken` after each successful lookup.
- Impact: normal dashboard polling converts reads into continuous MySQL writes;
  an authorized client can amplify database load.
- Fix: update last-used only when older than a coarse threshold, asynchronously
  coalesce updates, or derive recent activity from a session cache.
- Mitigation: slow UI polling and DB write capacity.
- False-positive notes: the write is not a confidentiality failure; severity is
  availability/scale.

## SEC-010 — One missing hidden-state utility exposes contradictory UI

- Rule ID: JS-STATE-001 / accessibility-security clarity
- Severity: Medium; High if hidden privileged controls become actionable
- Location:
  - `frontend/index.html:292-337`
  - `frontend/styles.css` (no global `.hidden` rule)
- Evidence: the real-browser snapshot showed both auth gate and project shell,
  and showed Display name in sign-in mode.
- Impact: users cannot trust whether they are authenticated; hidden controls
  remain focusable/announced; future privilege-dependent UI may be exposed.
- Fix:

```css
.hidden,
[hidden] {
  display: none !important;
}
```

  Then test the guest/authenticated accessibility trees. Server authorization
  remains mandatory regardless of visibility.
- Mitigation: current project APIs still enforce bearer authentication.
- False-positive notes: UI visibility is never an authorization boundary, but
  contradictory state is still a serious product and accessibility defect.

## SEC-011 — The project route table uses non-semantic interactive headers

- Rule ID: JS-A11Y-INTERACTION-001
- Severity: Low security; High usability/accessibility
- Location: `frontend/projects.js:847-863`
- Evidence: sortable `<th>` elements receive `data-action` rather than
  containing a keyboard-operable button and `aria-sort`.
- Impact: keyboard and assistive-technology users cannot reliably discover or
  operate sorting; visual and programmatic state can diverge.
- Fix: place a `<button type="button">` in each sortable header, update
  `aria-sort` on `<th>`, preserve focus, and announce changed results narrowly.
- False-positive notes: included because accessibility is an explicit project
  requirement, not because it is an exploit.

## Cross-cutting hardening backlog

1. Add one secure browser session and project-scoped automation credentials.
2. Remove legacy fail-open management access.
3. Remove inline event handlers and deploy a strict nonce/hash CSP.
4. Add auth/control-plane/ingestion/probe rate and resource budgets.
5. Separate endpoint catalog, telemetry source, and synthetic configuration.
6. Encrypt monitored-target secrets and add rotation/audit.
7. Keep dial-time SSRF controls and add egress network policy.
8. Add production HTTP timeouts and endpoint-specific body limits.
9. Add dependency, static analysis, vulnerability, race, and fuzz checks to CI.
10. Add negative tenant/auth tests for every management route and ingestion
    mapping.

## Verification

At minimum:

```text
go test ./...
go test -race ./...
govulncheck ./...
staticcheck ./...
```

Add focused fuzz targets for URL normalization, URL composition, redirect
validation, OpenAPI parsing, and header parsing. Add browser tests proving that
credentials do not exist in Web Storage, inline handlers are absent, CSP blocks
unexpected script execution, and guest/authenticated DOM states are exclusive.
