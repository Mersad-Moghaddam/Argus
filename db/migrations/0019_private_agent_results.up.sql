CREATE TABLE IF NOT EXISTS private_agent_results (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    agent_id BIGINT NOT NULL,
    project_id BIGINT NOT NULL,
    environment_id BIGINT NOT NULL,
    idempotency_key_hash BINARY(32) NOT NULL,
    outcome ENUM('success','failure') NOT NULL,
    summary VARCHAR(240) NULL,
    received_at DATETIME NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE KEY uq_private_agent_result_replay (agent_id, idempotency_key_hash),
    INDEX idx_private_agent_results_project (project_id, received_at),
    CONSTRAINT fk_private_agent_result_agent FOREIGN KEY (agent_id) REFERENCES private_agents(id) ON DELETE CASCADE,
    CONSTRAINT fk_private_agent_result_project FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE,
    CONSTRAINT fk_private_agent_result_environment FOREIGN KEY (environment_id) REFERENCES project_environments(id) ON DELETE CASCADE
);
