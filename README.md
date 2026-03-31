# Argus - Distributed Website Uptime Checker

Argus is a production-oriented uptime monitoring service built with Go, Fiber, MySQL, and Asynq.
It provides a REST API and admin panel for managing monitored websites with multiple monitor strategies.

## New production features

1. **Alerting + Incident lifecycle**
   - Open incident on downtime and auto-resolve on recovery.
   - Alert channels (`webhook`, `slack webhook`, `email placeholder`) with suppression during maintenance windows.

2. **Public status pages**
   - Create named status pages (`slug`, `title`) and assign monitors.
   - Public endpoint for external consumers: `GET /api/public/status/:slug`.

3. **Maintenance windows**
   - Global or per-website maintenance windows.
   - Mute alerts while still recording check results.

4. **Heartbeat monitors**
   - Heartbeat monitor type with grace period.
   - Heartbeat ingestion endpoint: `POST /api/websites/:id/heartbeat`.

5. **Advanced monitor types**
   - `http_status` (default)
   - `keyword` (body contains expected keyword)
   - `tls_expiry` (fails when cert expires within threshold)

## Architecture

- `cmd/api`: entrypoint
- `internal/app`: dependency composition root
- `internal/api`: HTTP handlers
- `internal/service`: business logic
- `internal/repository`: MySQL repository
- `internal/worker`: asynchronous checks, incidents, and alert dispatch
- `web`: admin UI

## Main API endpoints

- `POST /api/websites`
- `GET /api/websites`
- `DELETE /api/websites/:id`
- `POST /api/websites/:id/heartbeat`
- `GET /api/incidents`
- `POST /api/alert-channels`
- `POST /api/maintenance-windows`
- `GET /api/status-pages`
- `POST /api/status-pages`
- `GET /api/public/status/:slug`
- `GET /api/logs`

## Setup

```bash
docker compose up -d
cp .env.example .env
go run ./cmd/api
```

Server and UI are available at `http://localhost:8080`.
