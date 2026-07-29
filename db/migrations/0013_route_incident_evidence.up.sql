-- Existing route incidents are synthetic-source incidents. The additional
-- fields make source, evidence, and acknowledgement explicit without
-- changing prior lifecycle history.
ALTER TABLE route_incidents
    ADD COLUMN source VARCHAR(32) NOT NULL DEFAULT 'synthetic' AFTER project_id,
    ADD COLUMN source_key VARCHAR(160) NOT NULL DEFAULT '' AFTER source,
    ADD COLUMN evidence JSON NULL AFTER last_failure_reason,
    ADD COLUMN acknowledged_at DATETIME NULL AFTER started_at,
    ADD COLUMN acknowledged_by_user_id BIGINT NULL AFTER acknowledged_at,
    ADD INDEX idx_route_incidents_project_source_state (project_id, source, state),
    ADD CONSTRAINT fk_route_incidents_acknowledged_by FOREIGN KEY (acknowledged_by_user_id) REFERENCES users(id) ON DELETE SET NULL;
