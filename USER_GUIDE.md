# Argus User Guide

This guide explains each feature and how to use Argus in day-to-day operations.

## 1. Start the project

```bash
docker compose up -d
go run ./cmd/api
```

Open `http://localhost:8080`.

> If API key auth is enabled (`API_KEY`), you can either paste it into the **API key** field in the
> top bar and click **Save key**, or set it directly in the browser console:
>
> ```js
> localStorage.setItem('argus_api_key', 'your-key')
> ```

### Dashboard tour

The control center is organized into tabs so related actions stay together:

- **Overview** — live stat cards (total/up/down monitors, open incidents), quick-add forms for
  monitors, alert channels and status pages, plus a summary of recent incidents.
- **Monitors** — full monitor table with search, status filter, and per-row actions (heartbeat ping,
  delete with confirmation).
- **Incidents** — complete incident history and the maintenance window scheduler.
- **Alerts & Status Pages** — manage public status pages with one-click link copying.
- **Ping History** — the latest raw check results across all monitors.
- **API Projects** — project-based API route monitoring (see section 10). This tab has its own
  sign-in, separate from the `API_KEY` above.

Other UI niceties: a light/dark theme toggle, toast notifications for every action, an auto-refresh
countdown (every 30s), and a fully responsive layout for mobile/tablet use.

---

## 2. Add a monitor

From the **Overview** tab:
1. Enter URL (e.g. `https://example.com`).
2. Select check interval (seconds).
3. Choose monitor type:
   - `http_status`
   - `keyword`
   - `heartbeat`
   - `tls_expiry`
4. Optional: set expected keyword for `keyword` monitors.
5. Click **Add monitor**.

API equivalent:

```http
POST /monitor/websites
Content-Type: application/json
X-API-Key: <key>

{
  "url": "https://example.com",
  "checkInterval": 30,
  "monitorType": "http_status"
}
```

---

## 3. Monitor types and behavior

### HTTP status monitor
- Sends HTTP GET.
- `2xx/3xx` = up, otherwise down.

### Keyword monitor
- Fetches response (up to 1MB).
- Validates content type and searches for expected text.

### Heartbeat monitor
- Expects periodic heartbeat call:

```http
POST /monitor/heartbeat/:id
```

- If heartbeats stop beyond grace time, monitor turns down.

For API Project heartbeats, Argus evaluates liveness every minute. A late or
missing run creates one operational incident; the next unique receipt resolves
it. The job's reported success/failure outcome remains
separate from the liveness state.

### TLS expiry monitor
- Checks cert expiration.
- Fails when cert expiration is under configured threshold.

---

## 4. Incidents and alerting

- When a monitor transitions to `down`, an incident opens.
- When it recovers to `up`, incident resolves.
- Alerts are emitted via outbox + dispatcher workers (reliable async flow).

### Add alert channel

```http
POST /notification/channels
{
  "name": "Ops Webhook",
  "channelType": "webhook",
  "target": "https://hooks.example.com/argus"
}
```

Supported channel types:
- `webhook`
- `slack`
- `email` (adapter placeholder)

---

## 5. Maintenance windows

Mute alerts during planned work:

```http
POST /notification/maintenance
{
  "websiteId": 1,
  "startsAt": "2026-04-04T10:00:00Z",
  "endsAt": "2026-04-04T11:00:00Z",
  "reason": "Planned deployment"
}
```

During active window:
- checks continue
- incidents may still update
- alerts are suppressed

---

## 6. Status pages

### Create status page

```http
POST /status/pages
{
  "slug": "public-status",
  "title": "Public Service Status"
}
```

### Read public status page

```http
GET /status/public/public-status
```

---

## 7. Common endpoints

- `GET /monitor/websites?limit=100&offset=0`
- `GET /system/checks?limit=100` (latest ping/check history)
- `DELETE /monitor/websites/:id`
- `GET /system/incidents?limit=100&offset=0`
- `GET /status/pages?limit=100&offset=0`
- `GET /system/logs`

---

## 8. Reliability and ops notes

- DB schema is automatically migrated at startup from `db/migrations/*.up.sql`.
- Workers run scheduled check enqueue + outbox dispatch tasks.
- For production, configure:
  - `API_KEY`
  - DB pool env vars
  - worker concurrency/queue settings
  - Redis/MySQL credentials

---

## 9. Projects & API route monitoring

The **API Projects** tab monitors individual API operations rather than whole sites. It is a separate
bounded context: its own tables, its own accounts, and its own `/project/...` routes. Everything
in sections 1–8 keeps working exactly as before and is unaffected.

### 9.1 Sign in

API Projects uses email/password accounts and an opaque, server-stored browser session, independent
from the legacy `API_KEY` used by the uptime dashboard. Register or log in from the global header.
The browser receives an HttpOnly, SameSite=Lax session cookie plus a CSRF cookie; it never stores a
project credential in Web Storage. Sessions are valid for 30 days and can be reviewed or revoked
from **Account**.

```http
POST /identity/register   { "email": "you@example.com", "password": "at-least-8-chars", "name": "You" }
POST /identity/login      { "email": "you@example.com", "password": "at-least-8-chars" }
POST /identity/logout     Cookie: argus_session=...; X-CSRF-Token: ...
GET  /identity/profile    Cookie: argus_session=...
POST /identity/recovery/request { "email": "you@example.com" }
POST /identity/recovery/complete { "token": "one-time-token", "newPassword": "new-passphrase" }
```

Use **Forgot your password?** from the login page to begin recovery. The request response is
identical for every address, and Argus stores only a hash of the short-lived, single-use token.
Recovery delivery is an operator-configured HTTPS webhook (`RECOVERY_DELIVERY_URL`); it must send
the token to a verified account channel. When no delivery integration is configured, Argus safely
accepts the request without disclosing whether an account exists, but cannot complete recovery.

Whoever creates a project becomes its **owner**. Roles are `owner` > `editor` > `viewer`:

| Action | viewer | editor | owner |
| --- | --- | --- | --- |
| Read project, routes, checks, incidents, metrics | yes | yes | yes |
| Create/edit/delete routes, bulk operations, import | no | yes | yes |
| Edit project settings | no | yes | yes |
| Archive/restore/delete project | no | no | yes |

A user who is not a member of a project gets `404`, identical to the response for a project that
does not exist — so project IDs cannot be probed.

### 9.2 Create a project

Click **New project**. The authenticated source-aware flow first asks for the project identity and
an initial environment (with an optional canonical base URL), then
asks how it should be observed: **OpenTelemetry** (recommended), an **OpenAPI catalog**, a disabled
**Synthetic check**, a **Heartbeat**, clearly labeled non-production **Sample data**, or **Do this later**. It preserves a non-sensitive local draft
until the project is created and does not create target traffic during setup. OpenAPI continues to
the import wizard. OpenTelemetry, Synthetic, and Heartbeat selections offer a direct handoff to
their scoped setup dialog after the new project dashboard has loaded; every other choice opens the
project dashboard with the relevant next action.
The completion step also offers an explicit starter 99.9% availability SLO; it remains `no data`
until the configured minimum eligible telemetry events exist.

Interval, timeout, retries and incident thresholds are advanced project settings. They become the
defaults inherited by *new* routes; changing them later never rewrites existing routes.

```http
POST /project/catalog
{ "name": "Payments API", "defaultIntervalSeconds": 300, "defaultTimeoutMs": 5000,
  "defaultRetries": 1, "failureThreshold": 3, "recoverySuccessThreshold": 1 }
```

Projects can be searched, filtered by status, archived (keeps monitoring, hides from the active
list), restored, and deleted. Deleting a project removes its routes, checks, incidents and import
jobs.

### 9.3 Add routes

Four ways, all producing the same kind of route:

1. **Add route** — one operation at a time, with full monitoring configuration.
2. **Bulk add** — paste one `METHOD /path` per line plus a shared base URL, up to 5000 rows.
   Every row is reported individually, so a few bad lines never discard the good ones.
3. **Import spec → upload file** — an OpenAPI 3.x or Swagger 2.0 document, JSON or YAML.
4. **Import spec → paste** — the same, pasted as text.

A route's identity within a project is `METHOD` + `path`; adding the same pair twice returns
`400 route already exists for method and path`.

### 9.4 Importing an OpenAPI / Swagger specification

The wizard has three steps.

**Step 1 — provide the specification.** Upload a file or paste the text, optionally overriding the
base URL. Without an override, the base URL comes from `servers[0].url` (OpenAPI 3) or
`schemes` + `host` + `basePath` (Swagger 2). Limits: 10 MB per document and 5000 operations.
Local `$ref`s are resolved; remote `$ref` URLs are never fetched, so a specification cannot be used
to make the server issue arbitrary requests.

**Step 2 — review and select.** Every operation is diffed against the project's existing routes and
labelled:

| Badge | Meaning | Pre-selected? |
| --- | --- | --- |
| `new` | Not in the project yet; will be created | yes |
| `changed` | Exists, and the spec-derived definition differs | yes |
| `unchanged` | Exists and is byte-identical; nothing to do | no |
| `duplicate` | Two operations in this spec normalize to the same route | no (not selectable) |
| `removed` | Exists in the project but is absent from this spec | **no** |

Filter by badge, select or deselect everything shown, or reset to the recommended selection. Only
selected rows are applied.

**Step 3 — result report.** Counts of created, updated, disabled and skipped routes, plus any
per-row warnings.

What re-importing guarantees:

- New operations are added.
- Changed operations get fresh **spec metadata** — summary, description, tags, parameters, request
  body, responses, security, deprecated flag, base URL.
- Your **monitoring settings are never touched**: interval, timeout, retries, expected status range,
  failure/recovery thresholds, custom headers and enabled state all survive a re-import.
- Operations that disappeared from the spec are reported as `removed` but are **not** pre-selected.
  Selecting one **disables** the route — its configuration and check history are kept. Nothing is
  ever deleted by an import.

### 9.5 Reading the project dashboard

- **Summary tiles** — route counts per health state, 24h uptime, 24h average latency, open
  incidents, last check time.
- **Charts** — uptime % and response time over `1h`, `6h`, `24h`, `7d` or `30d`. These are served
  pre-bucketed by `GET /route/metrics/:id?range=24h`, so the browser never
  downloads raw check rows.
- **Incidents** — open and recently resolved, with duration and failure reason.
- **Route table** — search by path/name/summary, filter by method, health, tag, enabled and
  deprecated, sort by any column, and page through results. Searching, filtering, sorting and
  paging all happen in SQL, so a project with thousands of routes stays responsive.
- **Bulk actions** — select rows (or a whole page) and enable, disable or delete them together.
- **Service-level objectives** — editors can create availability or latency objectives with a
  target, rolling window, and minimum eligible-event threshold. Objectives use project-scoped
  telemetry only; missing, stale, maintenance, and configuration-error evidence is never rendered
  as healthy.
- **Telemetry route mappings** — editors can bind a trusted environment and service/deployment
  identity to one catalog route. The mapping never grants a telemetry sender the ability to choose
  a project or route outside the credential's server-side scope.
- **Environments** — editors can add or edit an environment’s name and optional base URL. The URL
  is normalized server-side before it is saved; updating one environment never changes another.
- **Heartbeats** — editors create a heartbeat for a selected environment and save the generated
  `argus_hb_...` token once. Send `POST /heartbeat/ping` with that token in `Authorization: Bearer`
  and a distinct `Idempotency-Key` for every scheduled run. The optional body only accepts
  `{ "outcome": "success" }` or `{ "outcome": "failure" }`; Argus never stores arbitrary job
  metadata. The dashboard exposes healthy, late, missing, and revoked states. Reusing a key is
  accepted as a retry but cannot extend the monitor's last-seen time; revoking the monitor rejects
  its token immediately.
- **Incidents** — every synthetic-route incident records its source and bounded evaluation
  evidence. Editors can choose **Acknowledge** to show that a person is responding; this never
  hides the incident or changes the recovery rule. A healthy check resolves the acknowledged
  incident, while viewers can still inspect its source, timing, and failure reason.

### 9.6 Route health states

One definition, applied everywhere (`internal/domain/route.go`):

| State | Meaning |
| --- | --- |
| `disabled` | Monitoring is switched off for this route. It is never checked. |
| `unknown` | Enabled but never checked yet. |
| `healthy` | The last check succeeded and there is no active failure streak. |
| `degraded` | Currently failing, but below the configured failure threshold. |
| `failing` | Consecutive failures have reached the threshold; an incident is open. |

The incident rule is configurable per project and per route: an incident **opens** after
`failureThreshold` consecutive failures (default 3) and **resolves** after `recoverySuccesses`
consecutive successes (default 1). Repeated failures while an incident is already open do not open a
second one.

### 9.7 Route detail

Each route has its own page showing configuration (target URL, interval, timeout, retries, expected
status range, incident rule, next check, tags, headers, and the imported parameters/request
body/responses/security blocks), current health, 24h uptime and latency, a status-code distribution,
the recent check log with failure reasons and attempt counts, and its incidents.

### 9.8 How checking works

Checks run entirely in the background worker — there are no frontend timers involved. Four scheduled
asynq tasks:

| Task | Cadence | What it does |
| --- | --- | --- |
| `route:enqueue_due_checks` | `ROUTE_SCHEDULER_INTERVAL` (15s) | Cursor-paginates enabled, due routes and enqueues one check each, keyed so a route already queued is never enqueued twice |
| `route:check` | on demand | Performs one request, then updates health, records the check and opens/resolves incidents |
| `route:aggregate_metrics` | `ROUTE_AGGREGATE_INTERVAL` (60s) | Refreshes the cached rolling-window columns on routes and projects with two batched statements |
| `route:prune_checks` | daily | Deletes check rows older than `ROUTE_CHECK_RETENTION` in bounded batches |

Route checks run on the `default` queue so they can never starve the legacy website checks on
`critical`; total in-flight work is bounded by `WORKER_CONCURRENCY`.

### 9.9 Monitored URLs are untrusted

Every outbound check is treated as hostile input:

- The address policy runs in the dialer's connect hook, so it inspects the **resolved IP**. A
  public hostname that resolves to a private address is refused, which closes the DNS-rebinding
  hole a hostname-only check would leave open. The same policy is applied to every redirect hop.
- **Always blocked**, regardless of configuration: cloud metadata endpoints
  (`169.254.169.254`, `metadata.google.internal`, `fd00:ec2::254`, …), IPv6 link-local,
  `0.0.0.0/8`, CGNAT `100.64.0.0/10`, `192.0.0.0/24`, benchmark `198.18.0.0/15`, reserved
  `240.0.0.0/4`, documentation `2001:db8::/32`, multicast and unspecified addresses.
- **Blocked by default**: loopback, RFC1918/ULA private ranges and link-local unicast. Set
  `ROUTE_ALLOW_PRIVATE_TARGETS=true` to monitor internal APIs — metadata endpoints stay blocked.
- Only `http` and `https` are allowed; URLs with embedded credentials are rejected.
- Redirects are capped, and `Authorization`, `Cookie` and API-key headers are stripped when a
  redirect crosses to a different origin.
- Response bodies are read up to 1 MB and discarded; per-route timeouts are clamped to a ceiling,
  and retries use bounded backoff. A policy-blocked target is never retried.
- Configured header values that look like secrets are masked in every API response, import preview
  and log line. The stored value stays intact so checks still authenticate.

### 9.10 Project API reference

All of these require `Authorization: Bearer <token>`.

```http
GET    /project/catalog?search=&status=&limit=&offset=
POST   /project/catalog
GET    /project/catalog/:projectId
PUT    /project/catalog/:projectId
POST   /project/archive/:projectId
POST   /project/restore/:projectId
DELETE /project/catalog/:projectId

GET    /route/catalog/:projectId?search=&method=&status=&tag=&enabled=&deprecated=&sortBy=&sortDir=&limit=&offset=
POST   /route/catalog/:projectId
POST   /route/bulk/:projectId
POST   /route/removal/:projectId
GET    /route/catalog/:projectId/:routeId
PUT    /route/catalog/:projectId/:routeId
POST   /route/enable/:projectId/:routeId
POST   /route/disable/:projectId/:routeId
DELETE /route/catalog/:projectId/:routeId
GET    /route/checks/:projectId/:routeId?limit=&offset=

GET    /route/incidents/:projectId?routeId=&state=&limit=&offset=
GET    /route/metrics/:projectId?range=1h|6h|24h|7d|30d&routeId=

GET    /incident/catalog/:projectId?state=&limit=&offset=
POST   /incident/acknowledge/:projectId/:incidentId

POST   /import/validation/:projectId      (multipart "file", or JSON {"spec": "..."})
GET    /import/job/:projectId/:jobId
POST   /import/commit/:projectId/:jobId
```

### 9.11 Configuration

All route-monitoring settings have safe defaults; none are required.

| Variable | Default | Purpose |
| --- | --- | --- |
| `ROUTE_SCHEDULER_INTERVAL` | `15s` | How often due routes are scanned and enqueued |
| `ROUTE_DUE_BATCH_SIZE` | `200` | Routes fetched per cursor page during that scan |
| `ROUTE_AGGREGATE_INTERVAL` | `60s` | How often cached dashboard metrics are refreshed |
| `ROUTE_AGGREGATE_WINDOW` | `24h` | Rolling window for the cached uptime/latency columns |
| `ROUTE_CHECK_RETENTION` | `720h` (30d) | How long raw `route_checks` rows are kept |
| `ROUTE_CHECK_PRUNE_BATCH` | `5000` | Rows deleted per pruning batch |
| `ROUTE_MAX_TIMEOUT` | `30s` | Ceiling applied on top of each route's own timeout |
| `ROUTE_MAX_REDIRECTS` | `5` | Redirect hop cap |
| `ROUTE_ALLOW_PRIVATE_TARGETS` | `false` | Allow loopback/private targets (metadata stays blocked) |
| `ROUTE_USER_AGENT` | `Argus-Monitor/1.0` | User-Agent sent on route checks |

---

## 10. Migration notes and troubleshooting

### Engine portability

Migrations are applied at startup from `db/migrations/*.up.sql` and are designed to run on both
MySQL 8 and MariaDB:

- `0003_compatibility.up.sql` adds missing `websites` columns through an `information_schema` check
  plus `PREPARE`/`EXECUTE`, because `ALTER TABLE ... ADD COLUMN IF NOT EXISTS` is a MariaDB
  extension and a syntax error on MySQL 8. `ApplyMigrations` therefore runs each file on a single
  pinned connection so that the user variable and prepared statement stay in one session.
- Uniqueness on long `VARCHAR` columns (`websites.url`, `api_routes.path`) uses a prefix index.
  A full-length index on those columns needs more than InnoDB's 3072-byte key limit under
  `utf8mb4` and the `CREATE TABLE` would fail outright. The prefixes (500 and 700 characters) are
  far longer than any real URL or API path.
- The statement splitter is quote- and comment-aware, so a semicolon inside a comment or a string
  literal does not split a statement in half.

Existing installations are unaffected: every statement is `CREATE TABLE IF NOT EXISTS` or a guarded
`ALTER`, so re-running migrations is a no-op.

### Duplicate key errors

If you see `ERROR 1005 ... errno: 121 "Duplicate key on write or update"` during manual table creation,
it usually means old FK metadata/constraint names conflict with previous schema attempts.

Recommended path:

1. Use Argus startup migrations instead of manual copy/paste SQL.
2. Apply latest migrations including compatibility migration `0003_compatibility.up.sql`.
3. If the DB was partially initialized, drop incomplete tables and retry migrations:

```sql
DROP TABLE IF EXISTS website_checks, incidents, maintenance_windows;
```

Then restart the app so migrations re-apply cleanly.
