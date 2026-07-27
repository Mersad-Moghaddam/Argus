SET @migration_sql = IF(
    EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='websites' AND column_name='health_check_url'),
    'SELECT 1',
    'ALTER TABLE websites ADD COLUMN health_check_url VARCHAR(2083) NULL'
);
PREPARE migration_stmt FROM @migration_sql; EXECUTE migration_stmt; DEALLOCATE PREPARE migration_stmt;

SET @migration_sql = IF(
    EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='websites' AND column_name='monitor_type'),
    'SELECT 1',
    'ALTER TABLE websites ADD COLUMN monitor_type ENUM(''http_status'', ''keyword'', ''heartbeat'', ''tls_expiry'') NOT NULL DEFAULT ''http_status'''
);
PREPARE migration_stmt FROM @migration_sql; EXECUTE migration_stmt; DEALLOCATE PREPARE migration_stmt;

SET @migration_sql = IF(
    EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='websites' AND column_name='expected_keyword'),
    'SELECT 1',
    'ALTER TABLE websites ADD COLUMN expected_keyword VARCHAR(512) NULL'
);
PREPARE migration_stmt FROM @migration_sql; EXECUTE migration_stmt; DEALLOCATE PREPARE migration_stmt;

SET @migration_sql = IF(
    EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='websites' AND column_name='tls_expiry_threshold_days'),
    'SELECT 1',
    'ALTER TABLE websites ADD COLUMN tls_expiry_threshold_days INT NOT NULL DEFAULT 14'
);
PREPARE migration_stmt FROM @migration_sql; EXECUTE migration_stmt; DEALLOCATE PREPARE migration_stmt;

SET @migration_sql = IF(
    EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='websites' AND column_name='heartbeat_grace_seconds'),
    'SELECT 1',
    'ALTER TABLE websites ADD COLUMN heartbeat_grace_seconds INT NOT NULL DEFAULT 0'
);
PREPARE migration_stmt FROM @migration_sql; EXECUTE migration_stmt; DEALLOCATE PREPARE migration_stmt;

SET @migration_sql = IF(
    EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='websites' AND column_name='last_heartbeat_received_at'),
    'SELECT 1',
    'ALTER TABLE websites ADD COLUMN last_heartbeat_received_at DATETIME NULL'
);
PREPARE migration_stmt FROM @migration_sql; EXECUTE migration_stmt; DEALLOCATE PREPARE migration_stmt;

SET @migration_sql = IF(
    EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='websites' AND column_name='status_page_id'),
    'SELECT 1',
    'ALTER TABLE websites ADD COLUMN status_page_id BIGINT NULL'
);
PREPARE migration_stmt FROM @migration_sql; EXECUTE migration_stmt; DEALLOCATE PREPARE migration_stmt;

SET @migration_sql = IF(
    EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='websites' AND column_name='next_check_at'),
    'SELECT 1',
    'ALTER TABLE websites ADD COLUMN next_check_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP'
);
PREPARE migration_stmt FROM @migration_sql; EXECUTE migration_stmt; DEALLOCATE PREPARE migration_stmt;

SET @migration_sql = IF(
    EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='websites' AND column_name='last_status_code'),
    'SELECT 1',
    'ALTER TABLE websites ADD COLUMN last_status_code INT NOT NULL DEFAULT 0'
);
PREPARE migration_stmt FROM @migration_sql; EXECUTE migration_stmt; DEALLOCATE PREPARE migration_stmt;

SET @migration_sql = IF(
    EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='websites' AND column_name='last_latency_ms'),
    'SELECT 1',
    'ALTER TABLE websites ADD COLUMN last_latency_ms INT NOT NULL DEFAULT 0'
);
PREPARE migration_stmt FROM @migration_sql; EXECUTE migration_stmt; DEALLOCATE PREPARE migration_stmt;

CREATE TABLE IF NOT EXISTS incidents (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    website_id BIGINT NOT NULL,
    state ENUM('open', 'acknowledged', 'resolved') NOT NULL DEFAULT 'open',
    started_at DATETIME NOT NULL,
    acknowledged_at DATETIME NULL,
    resolved_at DATETIME NULL,
    last_failure_reason VARCHAR(1024) NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_incidents_website_state (website_id, state)
);

CREATE TABLE IF NOT EXISTS website_checks (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    website_id BIGINT NOT NULL,
    status ENUM('up', 'down') NOT NULL,
    status_code INT NOT NULL DEFAULT 0,
    latency_ms INT NOT NULL DEFAULT 0,
    failure_reason VARCHAR(1024) NULL,
    checked_at DATETIME NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_website_checks_website_id_checked_at (website_id, checked_at DESC),
    FOREIGN KEY (website_id) REFERENCES websites(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS alert_channels (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(128) NOT NULL,
    channel_type ENUM('webhook', 'slack', 'email') NOT NULL DEFAULT 'webhook',
    target VARCHAR(1024) NOT NULL,
    enabled TINYINT(1) NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS maintenance_windows (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    website_id BIGINT NULL,
    starts_at DATETIME NOT NULL,
    ends_at DATETIME NOT NULL,
    mute_alerts TINYINT(1) NOT NULL DEFAULT 1,
    reason VARCHAR(512) NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_maintenance_active (starts_at, ends_at)
);

CREATE TABLE IF NOT EXISTS status_pages (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    slug VARCHAR(120) NOT NULL UNIQUE,
    title VARCHAR(255) NOT NULL,
    is_public TINYINT(1) NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);
