-- Durable UTC-day counters let all scheduler instances enforce the same
-- project and global synthetic-request limits before enqueueing work.
CREATE TABLE IF NOT EXISTS synthetic_budget_windows (
    scope ENUM('global','project') NOT NULL,
    project_id BIGINT NOT NULL DEFAULT 0,
    window_day DATE NOT NULL,
    request_count BIGINT NOT NULL DEFAULT 0,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (scope, project_id, window_day),
    CONSTRAINT chk_synthetic_budget_scope CHECK (
        (scope = 'global' AND project_id = 0) OR
        (scope = 'project' AND project_id > 0)
    )
);

-- Budget shedding is not a check failure. Keep compact operational evidence
-- rather than manufacturing a failing route_checks record.
CREATE TABLE IF NOT EXISTS synthetic_check_skips (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    route_id BIGINT NOT NULL,
    project_id BIGINT NOT NULL,
    reason ENUM('project_daily_budget','global_daily_budget','project_concurrency','global_concurrency') NOT NULL,
    skipped_at DATETIME NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_synthetic_check_skips_project_time (project_id, skipped_at DESC),
    INDEX idx_synthetic_check_skips_route_time (route_id, skipped_at DESC),
    FOREIGN KEY (route_id) REFERENCES api_routes(id) ON DELETE CASCADE,
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
);

-- Lock rows serialize lease admission, while the lease table records only
-- active work and self-heals after a worker crash through expires_at.
CREATE TABLE IF NOT EXISTS synthetic_concurrency_locks (
    scope ENUM('global','project') NOT NULL,
    project_id BIGINT NOT NULL DEFAULT 0,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (scope, project_id),
    CONSTRAINT chk_synthetic_concurrency_scope CHECK (
        (scope = 'global' AND project_id = 0) OR
        (scope = 'project' AND project_id > 0)
    )
);

CREATE TABLE IF NOT EXISTS synthetic_execution_leases (
    lease_key VARCHAR(96) NOT NULL PRIMARY KEY,
    project_id BIGINT NOT NULL,
    expires_at DATETIME NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_synthetic_execution_leases_expiry (expires_at),
    INDEX idx_synthetic_execution_leases_project_expiry (project_id, expires_at),
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
);
