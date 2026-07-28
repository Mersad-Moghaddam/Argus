-- Password recovery is intentionally separate from browser sessions. Raw
-- reset values never enter MySQL; only a SHA-256 hash is retained.
CREATE TABLE IF NOT EXISTS password_recovery_tokens (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    user_id BIGINT NOT NULL,
    token_hash CHAR(64) NOT NULL,
    expires_at DATETIME NOT NULL,
    used_at DATETIME NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE KEY uq_password_recovery_tokens_hash (token_hash),
    INDEX idx_password_recovery_tokens_user (user_id, created_at DESC),
    INDEX idx_password_recovery_tokens_expiry (expires_at),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);
