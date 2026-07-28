-- Compatibility migration: brings a pre-0001 installation up to the current
-- `websites` shape and creates any table an older deployment is missing.
--
-- Portability note: `ALTER TABLE ... ADD COLUMN IF NOT EXISTS` is a MariaDB
-- extension and is a syntax error on MySQL 8 (the server docker-compose.yml
-- ships). Each column is therefore added through an information_schema check
-- plus a prepared statement, which is idempotent on both engines. `DO 0` is
-- the no-op branch. No statement contains an embedded semicolon, so the plain
-- splitter in ApplyMigrations can run them as-is; ApplyMigrations pins one
-- connection per file so the user variable and prepared statement below stay
-- in the same session.

SET @ddl = (SELECT IF(COUNT(*) = 0, 'ALTER TABLE websites ADD COLUMN health_check_url VARCHAR(2083) NULL', 'DO 0')
    FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'websites' AND column_name = 'health_check_url');
PREPARE stmt FROM @ddl;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

SET @ddl = (SELECT IF(COUNT(*) = 0, 'ALTER TABLE websites ADD COLUMN monitor_type ENUM(''http_status'', ''keyword'', ''heartbeat'', ''tls_expiry'') NOT NULL DEFAULT ''http_status''', 'DO 0')
    FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'websites' AND column_name = 'monitor_type');
PREPARE stmt FROM @ddl;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

SET @ddl = (SELECT IF(COUNT(*) = 0, 'ALTER TABLE websites ADD COLUMN expected_keyword VARCHAR(512) NULL', 'DO 0')
    FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'websites' AND column_name = 'expected_keyword');
PREPARE stmt FROM @ddl;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

SET @ddl = (SELECT IF(COUNT(*) = 0, 'ALTER TABLE websites ADD COLUMN tls_expiry_threshold_days INT NOT NULL DEFAULT 14', 'DO 0')
    FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'websites' AND column_name = 'tls_expiry_threshold_days');
PREPARE stmt FROM @ddl;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

SET @ddl = (SELECT IF(COUNT(*) = 0, 'ALTER TABLE websites ADD COLUMN heartbeat_grace_seconds INT NOT NULL DEFAULT 0', 'DO 0')
    FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'websites' AND column_name = 'heartbeat_grace_seconds');
PREPARE stmt FROM @ddl;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

SET @ddl = (SELECT IF(COUNT(*) = 0, 'ALTER TABLE websites ADD COLUMN last_heartbeat_received_at DATETIME NULL', 'DO 0')
    FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'websites' AND column_name = 'last_heartbeat_received_at');
PREPARE stmt FROM @ddl;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

SET @ddl = (SELECT IF(COUNT(*) = 0, 'ALTER TABLE websites ADD COLUMN status_page_id BIGINT NULL', 'DO 0')
    FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'websites' AND column_name = 'status_page_id');
PREPARE stmt FROM @ddl;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

SET @ddl = (SELECT IF(COUNT(*) = 0, 'ALTER TABLE websites ADD COLUMN next_check_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP', 'DO 0')
    FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'websites' AND column_name = 'next_check_at');
PREPARE stmt FROM @ddl;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

SET @ddl = (SELECT IF(COUNT(*) = 0, 'ALTER TABLE websites ADD COLUMN last_status_code INT NOT NULL DEFAULT 0', 'DO 0')
    FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'websites' AND column_name = 'last_status_code');
PREPARE stmt FROM @ddl;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

SET @ddl = (SELECT IF(COUNT(*) = 0, 'ALTER TABLE websites ADD COLUMN last_latency_ms INT NOT NULL DEFAULT 0', 'DO 0')
    FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'websites' AND column_name = 'last_latency_ms');
PREPARE stmt FROM @ddl;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS status_pages (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    slug VARCHAR(120) NOT NULL UNIQUE,
    title VARCHAR(255) NOT NULL,
    is_public TINYINT(1) NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

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
