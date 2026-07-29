-- Project-scoped heartbeat monitors are independent from legacy websites.
-- Tokens and idempotency keys are represented only by SHA-256 hashes.
CREATE TABLE IF NOT EXISTS heartbeat_monitors (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    project_id BIGINT NOT NULL,
    environment_id BIGINT NOT NULL,
    created_by_user_id BIGINT NOT NULL,
    name VARCHAR(120) NOT NULL,
    token_prefix VARCHAR(32) NOT NULL,
    token_hash BINARY(32) NOT NULL,
    expected_interval_seconds INT NOT NULL,
    grace_period_seconds INT NOT NULL,
    revoked_at DATETIME NULL,
    last_received_at DATETIME NULL,
    last_outcome ENUM('success', 'failure') NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE KEY uq_heartbeat_monitors_token_hash (token_hash),
    INDEX idx_heartbeat_monitors_project (project_id, revoked_at, last_received_at),
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE,
    FOREIGN KEY (environment_id) REFERENCES project_environments(id) ON DELETE CASCADE,
    FOREIGN KEY (created_by_user_id) REFERENCES users(id) ON DELETE RESTRICT
);

CREATE TABLE IF NOT EXISTS heartbeat_receipts (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    monitor_id BIGINT NOT NULL,
    idempotency_key_hash BINARY(32) NOT NULL,
    outcome ENUM('success', 'failure') NOT NULL DEFAULT 'success',
    received_at DATETIME NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE KEY uq_heartbeat_receipts_monitor_key (monitor_id, idempotency_key_hash),
    INDEX idx_heartbeat_receipts_monitor_received (monitor_id, received_at DESC),
    FOREIGN KEY (monitor_id) REFERENCES heartbeat_monitors(id) ON DELETE CASCADE
);
