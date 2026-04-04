CREATE TABLE IF NOT EXISTS outbox_events (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  event_type VARCHAR(64) NOT NULL,
  aggregate_id BIGINT NOT NULL,
  dedupe_key VARCHAR(255) NOT NULL,
  payload JSON NOT NULL,
  status ENUM('pending','processed') NOT NULL DEFAULT 'pending',
  available_at DATETIME NOT NULL,
  processed_at DATETIME NULL,
  retry_count INT NOT NULL DEFAULT 0,
  last_error VARCHAR(1024) NULL,
  last_attempted_at DATETIME NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE KEY uq_outbox_dedupe (dedupe_key),
  KEY idx_outbox_pending (status, available_at)
);
