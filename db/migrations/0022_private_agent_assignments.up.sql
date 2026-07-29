-- Explicit assignments are the only way a private agent receives executable
-- work. The control plane still never dials the private target itself.
CREATE TABLE IF NOT EXISTS private_agent_assignments (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    project_id BIGINT NOT NULL,
    environment_id BIGINT NOT NULL,
    route_id BIGINT NOT NULL,
    name VARCHAR(120) NOT NULL,
    method ENUM('GET','HEAD') NOT NULL,
    target VARCHAR(2083) NOT NULL,
    interval_seconds INT NOT NULL,
    timeout_ms INT NOT NULL,
    enabled TINYINT(1) NOT NULL DEFAULT 1,
    created_by_user_id BIGINT NOT NULL,
    revoked_at DATETIME NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_private_agent_assignments_environment (environment_id, enabled, revoked_at),
    UNIQUE KEY uq_private_agent_assignment_route_environment (route_id, environment_id),
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE,
    FOREIGN KEY (environment_id) REFERENCES project_environments(id) ON DELETE CASCADE,
    FOREIGN KEY (route_id) REFERENCES api_routes(id) ON DELETE CASCADE,
    FOREIGN KEY (created_by_user_id) REFERENCES users(id) ON DELETE RESTRICT
);
