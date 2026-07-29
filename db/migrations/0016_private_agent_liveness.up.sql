ALTER TABLE private_agents
    ADD COLUMN expected_interval_seconds INT NOT NULL DEFAULT 60 AFTER version;
