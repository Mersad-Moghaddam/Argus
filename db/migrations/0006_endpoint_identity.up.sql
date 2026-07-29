-- V2 endpoint identity rollout. Columns remain nullable during the dual-read
-- migration window so legacy routes can be backfilled deterministically and
-- conflicts can be reviewed without data loss.
CREATE TABLE IF NOT EXISTS project_environments (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    project_id BIGINT NOT NULL,
    name VARCHAR(120) NOT NULL,
    canonical_base_url VARCHAR(2083) NOT NULL DEFAULT '',
    canonical_origin VARCHAR(512) NOT NULL DEFAULT '',
    is_default TINYINT(1) NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE KEY uq_project_environment_name (project_id, name),
    UNIQUE KEY uq_project_environment_base_url (project_id, canonical_base_url),
    INDEX idx_project_environments_project (project_id),
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
);

ALTER TABLE api_routes
    ADD COLUMN environment_id BIGINT NULL,
    ADD COLUMN canonical_identity VARCHAR(4096) NULL,
    ADD COLUMN canonical_hash BINARY(32) NULL,
    ADD COLUMN canonical_version SMALLINT NOT NULL DEFAULT 1,
    ADD INDEX idx_api_routes_environment (environment_id),
    ADD INDEX idx_api_routes_canonical_lookup (project_id, canonical_hash),
    ADD CONSTRAINT fk_api_routes_environment FOREIGN KEY (environment_id) REFERENCES project_environments(id) ON DELETE SET NULL;

CREATE TABLE IF NOT EXISTS route_canonicalization_conflicts (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    project_id BIGINT NOT NULL,
    route_id BIGINT NOT NULL,
    conflicting_route_id BIGINT NULL,
    conflict_type ENUM('exact_duplicate', 'environment_collision', 'legacy_prefix_collision', 'semantic_ambiguity', 'invalid_legacy_value') NOT NULL,
    details VARCHAR(2000) NOT NULL DEFAULT '',
    resolved_at DATETIME NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE KEY uq_route_canonical_conflict (route_id, conflict_type),
    INDEX idx_route_canonical_conflicts_project (project_id, resolved_at),
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE,
    FOREIGN KEY (route_id) REFERENCES api_routes(id) ON DELETE CASCADE,
    FOREIGN KEY (conflicting_route_id) REFERENCES api_routes(id) ON DELETE SET NULL
);
