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
POST /api/websites
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
POST /api/websites/:id/heartbeat
```

- If heartbeats stop beyond grace time, monitor turns down.

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
POST /api/alert-channels
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
POST /api/maintenance-windows
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
POST /api/status-pages
{
  "slug": "public-status",
  "title": "Public Service Status"
}
```

### Read public status page

```http
GET /api/public/status/public-status
```

---

## 7. Common endpoints

- `GET /api/websites?limit=100&offset=0`
- `GET /api/checks?limit=100` (latest ping/check history)
- `DELETE /api/websites/:id`
- `GET /api/incidents?limit=100&offset=0`
- `GET /api/status-pages?limit=100&offset=0`
- `GET /api/logs`

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

## 9. MariaDB migration troubleshooting

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

---

## 10. Projects and API route monitoring

Open the **Projects** tab. Project accounts use a bearer session that is
separate from the legacy API-key field, so both monitoring systems can run
side-by-side without changing existing behavior.

### Create and manage a project

1. Register or sign in from the Projects tab.
2. Select **New project**.
3. Set defaults for interval, timeout, retries, failures required to open an
   incident, and successful checks required to recover.
4. Open a project card to see route health, 24-hour uptime and response time,
   incidents, and the server-paginated route table.

Owners can archive, restore, and delete projects. Editors can change projects,
routes, and imports. Viewers have read-only access. A user who is not a project
member receives the same `404` response as a nonexistent project.

### Add routes manually or in bulk

Use **Add route** for one operation. The API also supports bulk creation:

```http
POST /api/projects/:projectId/routes/bulk
Authorization: Bearer <project-session-token>
Content-Type: application/json

{
  "routes": [{
    "method": "GET",
    "path": "/v1/orders/{id}",
    "baseUrl": "https://api.example.com",
    "monitorIntervalSeconds": 60,
    "timeoutMs": 5000,
    "retries": 1,
    "expectedStatusRange": "200-399"
  }]
}
```

Bulk results report successful and failed rows independently. The route table
supports server-side search, method and health filters, enabled/disabled
filtering, sorting, pagination, selection, enable/disable, and deletion.

### Import OpenAPI or Swagger

Select **Import spec** inside a project:

1. Upload a JSON/YAML file or paste the complete specification. An optional
   base URL can override the specification server.
2. Review the preview. Every operation is classified as new, changed,
   unchanged/duplicate, or removed. Select the exact rows to apply.
3. Commit and review the created, updated, skipped, and disabled counts.

Supported formats are OpenAPI 3.x and Swagger 2.0, including local references,
servers/base paths, methods, parameters, request bodies, responses, tags,
security requirements, and deprecated operations. Remote references are never
fetched. Re-import updates specification metadata only; it does not overwrite
intervals, timeouts, retries, thresholds, enabled state, headers, or expected
status ranges chosen by the user. Routes removed from a new specification are
disabled only when explicitly selected.

### Route health and incidents

- `unknown` — enabled but never checked.
- `healthy` — the latest check succeeded and no failure streak is active.
- `degraded` — checks are failing but have not reached the failure threshold.
- `failing` — failures reached the threshold and one incident is open.
- `disabled` — background monitoring is turned off.

Checks run in the persistent worker, not in the browser. Each route uses its
configured method, timeout, retries, status range, and interval. An incident
opens once after the failure threshold and resolves after the configured
number of consecutive successes.

### Monitoring security and scale

Monitored URLs are untrusted. Argus accepts only HTTP(S), rejects URL
credentials and private, loopback, link-local, metadata, multicast, and
special-purpose addresses, validates resolved addresses again while dialing,
and revalidates every redirect. Specifications are size- and complexity-
bounded, JSON/YAML references are resolved locally without network access,
sensitive headers are redacted in API responses, and bulk operations always
enforce project membership and role.

Route checks and dashboard reads use cursor scans, server pagination, indexed
time-series history, batched aggregation, bounded pruning, worker concurrency,
and unique queued jobs. Configure retention and worker limits with the
`ROUTE_*` variables in `.env.example`.
