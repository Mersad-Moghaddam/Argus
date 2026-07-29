CREATE TABLE IF NOT EXISTS telemetry_route_mappings (
 id BIGINT AUTO_INCREMENT PRIMARY KEY, project_id BIGINT NOT NULL, environment_id BIGINT NOT NULL, route_id BIGINT NOT NULL,
 service_name VARCHAR(160) NOT NULL, deployment_environment VARCHAR(160) NOT NULL DEFAULT '', http_method VARCHAR(16) NOT NULL, route_template VARCHAR(1024) NOT NULL, source ENUM('manual') NOT NULL DEFAULT 'manual', created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
 UNIQUE KEY uq_telemetry_route_mapping_identity (project_id, environment_id, route_id, service_name, deployment_environment),
 INDEX idx_telemetry_route_mappings_project (project_id),
 FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE,
 FOREIGN KEY (environment_id) REFERENCES project_environments(id) ON DELETE CASCADE,
 FOREIGN KEY (route_id) REFERENCES api_routes(id) ON DELETE CASCADE
);
