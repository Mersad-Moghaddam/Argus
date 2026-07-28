-- OTLP credentials bind telemetry to an Argus project and environment on the
-- server. Resource attributes supplied by a collector are metadata only and
-- can never select a tenant.
CREATE TABLE IF NOT EXISTS telemetry_credentials (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    project_id BIGINT NOT NULL,
    environment_id BIGINT NOT NULL,
    created_by_user_id BIGINT NOT NULL,
    name VARCHAR(120) NOT NULL,
    token_prefix VARCHAR(32) NOT NULL,
    token_hash BINARY(32) NOT NULL,
    scopes VARCHAR(200) NOT NULL,
    rate_limit_per_minute INT NOT NULL DEFAULT 600,
    expires_at DATETIME NULL,
    revoked_at DATETIME NULL,
    last_used_at DATETIME NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE KEY uq_telemetry_credentials_token_hash (token_hash),
    INDEX idx_telemetry_credentials_project (project_id, revoked_at, expires_at),
    INDEX idx_telemetry_credentials_environment (environment_id),
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE,
    FOREIGN KEY (environment_id) REFERENCES project_environments(id) ON DELETE CASCADE,
    FOREIGN KEY (created_by_user_id) REFERENCES users(id) ON DELETE RESTRICT
);
