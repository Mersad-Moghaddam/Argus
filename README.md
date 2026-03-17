# Argus - Distributed Website Uptime Checker

Argus is a production-oriented uptime monitoring service built with Go, Fiber, MySQL, and Asynq.
It provides a clean REST API and a lightweight admin panel for managing monitored websites.

The admin panel now includes a **System & Worker Logs** section with live events for startup, scheduling, task enqueues, and website check results.

It also shows **Ping latency (ms)** per website and supports adding a **custom health-check URL** from a separate form.

A custom project favicon is served from `web/assets/favicon.svg`.

## Why this architecture is professional

This project uses **clear layering** and **loose coupling**:

- `cmd/api`: thin entrypoint only (no heavy wiring logic)
- `internal/app`: dependency composition root (manual DI)
- `internal/platform`: infrastructure adapters (MySQL, Fiber server, Asynq runtime)
- `internal/api`: HTTP handlers
- `internal/service`: business rules
- `internal/repository`: persistence abstraction and MySQL implementation
- `internal/worker`: task payloads + processors

This separation keeps `main.go` simple and moves infrastructure concerns into dedicated packages.

## Project Structure

```text
.
├── cmd/
│   └── api/
│       └── main.go
├── db/
│   └── schema.sql
├── internal/
│   ├── api/
│   ├── app/
│   ├── config/
│   ├── models/
│   ├── platform/
│   │   ├── httpserver/
│   │   ├── storage/
│   │   └── worker/
│   ├── repository/
│   ├── service/
│   └── worker/
├── web/
│   └── index.html
├── .env.example
├── docker-compose.yml
├── revive.toml
└── README.md
```

## Database Schema

```sql
CREATE DATABASE IF NOT EXISTS argus;
USE argus;

CREATE TABLE IF NOT EXISTS websites (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    url VARCHAR(2083) NOT NULL UNIQUE,
    health_check_url VARCHAR(2083) NULL,
    check_interval_seconds INT NOT NULL,
    status ENUM('pending', 'up', 'down') NOT NULL DEFAULT 'pending',
    last_checked_at DATETIME NULL,
    next_check_at DATETIME NOT NULL,
    last_status_code INT NOT NULL DEFAULT 0,
    last_latency_ms INT NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_websites_next_check_at (next_check_at)
);
```


## Main Page Preview

![Argus main page](browser:/tmp/codex_browser_invocations/a9c54364a77af8c1/artifacts/artifacts/main-page.png)

## Runtime flow

1. User adds website through API/UI.
2. Scheduler enqueues `website:enqueue_due_checks` on interval.
3. Worker queries due websites and enqueues `website:check` tasks.
4. Worker runs HTTP GET with 5s timeout.
5. Only `http` and `https` URLs are accepted at creation time.
6. Worker checks `health_check_url` when provided; otherwise it checks the main URL.
7. MySQL row is updated with status (`up/down`), status code, ping latency (ms), and next check time.

## Setup

### 1) Start infrastructure

```bash
docker compose up -d
```

### 2) Configure environment

```bash
cp .env.example .env
```

Update `.env` if needed.

### 3) Install dependencies

```bash
go mod tidy
```

### 4) Run application

```bash
go run ./cmd/api
```

Server and admin UI are available at `http://localhost:8080`.

## Environment variables (.env)

| Variable | Default |
|---|---|
| `HTTP_ADDR` | `:8080` |
| `MYSQL_DSN` | `argus:argus@tcp(localhost:3306)/argus?parseTime=true` |
| `REDIS_ADDR` | `localhost:6379` |
| `REDIS_PASSWORD` | `` |
| `REDIS_DB` | `0` |
| `SCHEDULER_INTERVAL` | `30s` |

## API Endpoints

### POST `/api/websites`

```json
{
  "url": "https://example.com",
  "healthCheckUrl": "https://example.com/health",
  "checkInterval": 30
}
```

### GET `/api/websites`
Returns all websites.

### DELETE `/api/websites/:id`
Deletes website by id.

## Linting

```bash
revive -config revive.toml ./...
```


### GET `/api/logs`
Returns recent in-memory operational logs (newest first).

Query params:
- `limit` (optional, default `200`)
- `websiteId` (optional, filter logs for a single website)

Each log includes timestamp, level (`info`/`warn`/`error`), source (`system` or `worker`), action, message, and detailed key-value metadata.
