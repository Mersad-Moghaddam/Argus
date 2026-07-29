CREATE TABLE IF NOT EXISTS project_incidents (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    project_id BIGINT NOT NULL,
    source VARCHAR(32) NOT NULL,
    source_key VARCHAR(160) NOT NULL,
    state ENUM('open','acknowledged','resolved') NOT NULL DEFAULT 'open',
    title VARCHAR(240) NOT NULL,
    evidence JSON NULL,
    started_at DATETIME NOT NULL,
    acknowledged_at DATETIME NULL,
    acknowledged_by_user_id BIGINT NULL,
    resolved_at DATETIME NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_project_incidents_source_state (project_id, source, source_key, state),
    INDEX idx_project_incidents_project_state (project_id, state, started_at),
    CONSTRAINT fk_project_incidents_project FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE,
    CONSTRAINT fk_project_incidents_acknowledger FOREIGN KEY (acknowledged_by_user_id) REFERENCES users(id) ON DELETE SET NULL
);
