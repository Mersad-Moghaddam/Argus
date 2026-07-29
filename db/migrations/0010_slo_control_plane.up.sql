-- SLO definitions are project-scoped configuration. Version snapshots make
-- historical evaluations reproducible after an SLO is edited in the future.
CREATE TABLE IF NOT EXISTS slo_definitions (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    project_id BIGINT NOT NULL,
    created_by_user_id BIGINT NOT NULL,
    name VARCHAR(160) NOT NULL,
    sli_kind ENUM('availability', 'latency') NOT NULL,
    target_percent DECIMAL(5,3) NOT NULL,
    window_seconds INT NOT NULL,
    latency_threshold_ms INT NOT NULL DEFAULT 0,
    min_events INT NOT NULL DEFAULT 0,
    short_window_seconds INT NOT NULL,
    short_burn_rate DECIMAL(8,3) NOT NULL,
    long_window_seconds INT NOT NULL,
    long_burn_rate DECIMAL(8,3) NOT NULL,
    paused BOOLEAN NOT NULL DEFAULT FALSE,
    version INT NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE KEY uq_slo_definitions_project_name (project_id, name),
    INDEX idx_slo_definitions_project (project_id, id),
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE,
    FOREIGN KEY (created_by_user_id) REFERENCES users(id) ON DELETE RESTRICT
);

CREATE TABLE IF NOT EXISTS slo_definition_versions (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    slo_id BIGINT NOT NULL,
    project_id BIGINT NOT NULL,
    version INT NOT NULL,
    definition_json JSON NOT NULL,
    created_by_user_id BIGINT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE KEY uq_slo_definition_versions (slo_id, version),
    INDEX idx_slo_definition_versions_project (project_id, slo_id, version),
    FOREIGN KEY (slo_id) REFERENCES slo_definitions(id) ON DELETE CASCADE,
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE,
    FOREIGN KEY (created_by_user_id) REFERENCES users(id) ON DELETE RESTRICT
);

-- Evaluations deliberately retain aggregate counts and policy output only;
-- raw telemetry remains in the metrics backend and is never copied here.
CREATE TABLE IF NOT EXISTS slo_evaluations (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    slo_id BIGINT NOT NULL,
    project_id BIGINT NOT NULL,
    definition_version INT NOT NULL,
    status ENUM('healthy', 'unhealthy', 'no_data', 'stale', 'paused', 'maintenance', 'configuration_error') NOT NULL,
    observed_percent DECIMAL(6,3) NULL,
    error_budget_remaining DECIMAL(7,3) NULL,
    burn_rate DECIMAL(10,3) NULL,
    good_events BIGINT NOT NULL DEFAULT 0,
    total_events BIGINT NOT NULL DEFAULT 0,
    window_started_at DATETIME NOT NULL,
    window_ended_at DATETIME NOT NULL,
    observed_at DATETIME NULL,
    provenance VARCHAR(160) NOT NULL,
    evaluated_at DATETIME NOT NULL,
    INDEX idx_slo_evaluations_slo_evaluated (slo_id, evaluated_at DESC),
    INDEX idx_slo_evaluations_project_evaluated (project_id, evaluated_at DESC),
    FOREIGN KEY (slo_id) REFERENCES slo_definitions(id) ON DELETE CASCADE,
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
);
