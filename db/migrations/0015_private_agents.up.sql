CREATE TABLE IF NOT EXISTS private_agents (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    project_id BIGINT NOT NULL,
    environment_id BIGINT NOT NULL,
    created_by_user_id BIGINT NOT NULL,
    name VARCHAR(120) NOT NULL,
    token_prefix VARCHAR(32) NOT NULL,
    token_hash BINARY(32) NOT NULL,
    version VARCHAR(80) NOT NULL DEFAULT '',
    last_seen_at DATETIME NULL,
    revoked_at DATETIME NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE KEY uq_private_agents_token_hash (token_hash),
    INDEX idx_private_agents_project (project_id, revoked_at, last_seen_at),
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE,
    FOREIGN KEY (environment_id) REFERENCES project_environments(id) ON DELETE CASCADE,
    FOREIGN KEY (created_by_user_id) REFERENCES users(id) ON DELETE RESTRICT
);
