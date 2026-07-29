ALTER TABLE route_incidents
    DROP FOREIGN KEY fk_route_incidents_acknowledged_by,
    DROP INDEX idx_route_incidents_project_source_state,
    DROP COLUMN acknowledged_by_user_id,
    DROP COLUMN acknowledged_at,
    DROP COLUMN evidence,
    DROP COLUMN source_key,
    DROP COLUMN source;
