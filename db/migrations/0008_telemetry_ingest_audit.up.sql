-- This table is an auditable low-cardinality receiver ledger, not a metrics
-- backend. It intentionally excludes samples, trace IDs, span names, URLs,
-- attributes, and payloads; those belong in the dedicated time-series system.
CREATE TABLE IF NOT EXISTS telemetry_ingest_records (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    project_id BIGINT NOT NULL,
    environment_id BIGINT NOT NULL,
    credential_id BIGINT NOT NULL,
    signal_type ENUM('metrics', 'traces') NOT NULL,
    service_name VARCHAR(160) NOT NULL DEFAULT '',
    deployment_environment VARCHAR(160) NOT NULL DEFAULT '',
    item_count INT NOT NULL DEFAULT 0,
    received_at DATETIME NOT NULL,
    INDEX idx_telemetry_ingest_records_project_received (project_id, received_at DESC),
    INDEX idx_telemetry_ingest_records_environment_received (environment_id, received_at DESC),
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE,
    FOREIGN KEY (environment_id) REFERENCES project_environments(id) ON DELETE CASCADE,
    FOREIGN KEY (credential_id) REFERENCES telemetry_credentials(id) ON DELETE CASCADE
);
