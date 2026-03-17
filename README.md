# Argus - Distributed Website Uptime Checker

Argus is a production-ready distributed website uptime checker built with **Go**, **Fiber**, **MySQL**, and **Asynq (Redis)**.  
It provides a clean API and a lightweight web admin panel for managing monitored websites and viewing uptime status.

## Project Overview

Argus tracks websites and periodically checks their availability.

- Add websites with custom check intervals.
- List monitored websites and latest status.
- Remove websites that are no longer needed.
- Run checks asynchronously through Asynq workers.
- Use a scheduler to regularly dispatch due website checks.

## Tech Stack

- **Backend:** Go 1.21+
- **Web Framework:** Fiber v2
- **Database:** MySQL (`database/sql`)
- **Queue + Scheduler:** Asynq + Redis
- **Frontend:** HTML + Tailwind CSS (CDN) + Vanilla JS
- **Linting:** Revive

## System Architecture

Argus follows a clean, layered architecture:

1. **API Layer (`internal/api`)**
   - Fiber handlers parse requests and return JSON responses.
2. **Service Layer (`internal/service`)**
   - Business rules/validation (URL format, interval constraints).
3. **Repository Layer (`internal/repository`)**
   - MySQL persistence using `database/sql`.
4. **Worker Layer (`internal/worker`)**
   - Asynq task types and processors.
   - Scheduler enqueues dispatch tasks.
   - Worker dispatches website checks and updates DB.
5. **Models (`internal/models`)**
   - Shared domain entities.

### Runtime Flow

1. User adds website via API or Admin Panel.
2. Website row is stored with `next_check_at`.
3. Asynq Scheduler periodically enqueues `website:enqueue_due_checks`.
4. Worker picks dispatch task, queries due websites, and enqueues `website:check` tasks.
5. Worker executes HTTP GET with a 5-second timeout and updates website status in MySQL.

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
│   │   └── website_handler.go
│   ├── config/
│   │   └── config.go
│   ├── models/
│   │   └── website.go
│   ├── repository/
│   │   └── website_repository.go
│   ├── service/
│   │   └── website_service.go
│   └── worker/
│       ├── processor.go
│       └── tasks.go
├── web/
│   └── index.html
├── docker-compose.yml
├── revive.toml
└── README.md
```

## Database Schema

See `db/schema.sql`.

```sql
CREATE DATABASE IF NOT EXISTS argus;
USE argus;

CREATE TABLE IF NOT EXISTS websites (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    url VARCHAR(2083) NOT NULL UNIQUE,
    check_interval_seconds INT NOT NULL,
    status ENUM('pending', 'up', 'down') NOT NULL DEFAULT 'pending',
    last_checked_at DATETIME NULL,
    next_check_at DATETIME NOT NULL,
    last_status_code INT NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_websites_next_check_at (next_check_at)
);
```

## Setup Instructions

### 1) Start dependencies

```bash
docker compose up -d
```

This starts:

- MySQL on `localhost:3306`
- Redis on `localhost:6379`

### 2) Install Go dependencies

```bash
go mod tidy
```

### 3) Run the application

```bash
go run ./cmd/api
```

App defaults:

- API and Admin UI: `http://localhost:8080`
- API base path: `/api`

### 4) Optional environment variables

| Variable | Default |
|---|---|
| `HTTP_ADDR` | `:8080` |
| `MYSQL_DSN` | `argus:argus@tcp(localhost:3306)/argus?parseTime=true` |
| `REDIS_ADDR` | `localhost:6379` |
| `REDIS_PASSWORD` | `` |
| `REDIS_DB` | `0` |
| `SCHEDULER_INTERVAL` | `30s` |

## API Endpoints

### `POST /api/websites`

Create a website monitor.

**Request body**

```json
{
  "url": "https://example.com",
  "checkInterval": 30
}
```

**Response** `201 Created`

```json
{
  "id": 1,
  "url": "https://example.com",
  "checkInterval": 30,
  "status": "pending"
}
```

---

### `GET /api/websites`

List all monitored websites.

**Response** `200 OK`

```json
[
  {
    "id": 1,
    "url": "https://example.com",
    "checkInterval": 30,
    "status": "up",
    "lastCheckedAt": "2026-01-01T12:00:00Z",
    "nextCheckAt": "2026-01-01T12:00:30Z",
    "lastStatusCode": 200,
    "createdAt": "2026-01-01T11:59:00Z",
    "updatedAt": "2026-01-01T12:00:00Z"
  }
]
```

---

### `DELETE /api/websites/:id`

Delete a website monitor.

**Response** `204 No Content`

## Linting

Use Revive with strict rules:

```bash
revive -config revive.toml ./...
```

## Notes on Production Readiness

- Manual dependency injection in `cmd/api/main.go`.
- Graceful shutdown for Fiber, Asynq server, scheduler, and DB connection.
- Context propagation from API handlers to repository and worker operations.
- Early-return coding style and error wrapping for observability.
