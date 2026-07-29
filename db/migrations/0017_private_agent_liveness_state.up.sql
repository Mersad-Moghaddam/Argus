ALTER TABLE private_agents ADD COLUMN liveness_state VARCHAR(16) NOT NULL DEFAULT 'offline' AFTER expected_interval_seconds;
CREATE INDEX idx_private_agents_liveness ON private_agents (liveness_state, id);
