CREATE TABLE IF NOT EXISTS route_header_secret_rotations (
    rotation_id VARCHAR(100) CHARACTER SET ascii COLLATE ascii_bin PRIMARY KEY,
    source_key_fingerprint BINARY(32) NOT NULL,
    destination_key_fingerprint BINARY(32) NOT NULL,
    last_route_id BIGINT NOT NULL DEFAULT 0,
    completed_at DATETIME NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);
