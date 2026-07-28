-- Rollback removes the additive v2 identity metadata. Legacy api_routes data
-- remains intact because its original columns are never rewritten here.
DROP TABLE IF EXISTS route_canonicalization_conflicts;
ALTER TABLE api_routes
    DROP FOREIGN KEY fk_api_routes_environment,
    DROP INDEX idx_api_routes_environment,
    DROP INDEX idx_api_routes_canonical_lookup,
    DROP COLUMN environment_id,
    DROP COLUMN canonical_identity,
    DROP COLUMN canonical_hash,
    DROP COLUMN canonical_version;
DROP TABLE IF EXISTS project_environments;
