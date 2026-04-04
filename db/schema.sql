CREATE DATABASE IF NOT EXISTS argus;
USE argus;

CREATE TABLE IF NOT EXISTS status_pages (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    slug VARCHAR(120) NOT NULL UNIQUE,
    title VARCHAR(255) NOT NULL,
    is_public TINYINT(1) NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS websites (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    url VARCHAR(2083) NOT NULL UNIQUE,
    health_check_url VARCHAR(2083) NULL,
    check_interval_seconds INT NOT NULL,
    monitor_type ENUM('http_status', 'keyword', 'heartbeat', 'tls_expiry') NOT NULL DEFAULT 'http_status',
    expected_keyword VARCHAR(512) NULL,
    tls_expiry_threshold_days INT NOT NULL DEFAULT 14,
    heartbeat_grace_seconds INT NOT NULL DEFAULT 0,
    status ENUM('pending', 'up', 'down') NOT NULL DEFAULT 'pending',
    last_checked_at DATETIME NULL,
    last_heartbeat_received_at DATETIME NULL,
    next_check_at DATETIME NOT NULL,
    last_status_code INT NOT NULL DEFAULT 0,
    last_latency_ms INT NOT NULL DEFAULT 0,
    status_page_id BIGINT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_websites_next_check_at (next_check_at),
    INDEX idx_websites_status_page_id (status_page_id),
    CONSTRAINT fk_websites_status_page FOREIGN KEY (status_page_id) REFERENCES status_pages(id) ON DELETE SET NULL
);

-- Backward-compatible online migration for existing installations.
ALTER TABLE websites
    ADD COLUMN IF NOT EXISTS monitor_type ENUM('http_status', 'keyword', 'heartbeat', 'tls_expiry') NOT NULL DEFAULT 'http_status',
    ADD COLUMN IF NOT EXISTS expected_keyword VARCHAR(512) NULL,
    ADD COLUMN IF NOT EXISTS tls_expiry_threshold_days INT NOT NULL DEFAULT 14,
    ADD COLUMN IF NOT EXISTS heartbeat_grace_seconds INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS last_heartbeat_received_at DATETIME NULL,
    ADD COLUMN IF NOT EXISTS status_page_id BIGINT NULL;

ALTER TABLE websites
    ADD INDEX IF NOT EXISTS idx_websites_next_check_at (next_check_at),
    ADD INDEX IF NOT EXISTS idx_websites_status_page_id (status_page_id);

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
    CONSTRAINT fk_website_checks_website FOREIGN KEY (website_id) REFERENCES websites(id) ON DELETE CASCADE
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
    INDEX idx_incidents_website_state (website_id, state),
    CONSTRAINT fk_incidents_website FOREIGN KEY (website_id) REFERENCES websites(id) ON DELETE CASCADE
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
    INDEX idx_maintenance_active (starts_at, ends_at),
    CONSTRAINT fk_maintenance_website FOREIGN KEY (website_id) REFERENCES websites(id) ON DELETE CASCADE
);
