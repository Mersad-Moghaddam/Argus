ALTER TABLE private_agent_results
    ADD COLUMN assignment_id BIGINT NULL AFTER environment_id,
    ADD INDEX idx_private_agent_results_assignment (assignment_id, received_at),
    ADD CONSTRAINT fk_private_agent_result_assignment
        FOREIGN KEY (assignment_id) REFERENCES private_agent_assignments(id) ON DELETE SET NULL;
