CREATE DATABASE IF NOT EXISTS argus;
USE argus;

CREATE TABLE IF NOT EXISTS websites (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    url VARCHAR(2083) NOT NULL UNIQUE,
    check_interval_seconds INT NOT NULL,
    status ENUM('pending', 'up', 'down') NOT NULL DEFAULT 'pending',
    last_checked_at DATETIME NULL,
    next_check_at DATETIME NOT NULL,
    last_status_code INT NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_websites_next_check_at (next_check_at)
);
