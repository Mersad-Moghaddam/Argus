# Argus User Guide

This guide explains each feature and how to use Argus in day-to-day operations.

## 1. Start the project

```bash
docker compose up -d
go run ./cmd/api
```

Open `http://localhost:8080`.

> If API key auth is enabled (`API_KEY`), set it in browser console:
>
> ```js
> localStorage.setItem('argus_api_key', 'your-key')
> ```

---

## 2. Add a monitor

From the dashboard:
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

