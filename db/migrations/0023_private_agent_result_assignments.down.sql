ALTER TABLE private_agent_results
    DROP FOREIGN KEY fk_private_agent_result_assignment,
    DROP INDEX idx_private_agent_results_assignment,
    DROP COLUMN assignment_id;
