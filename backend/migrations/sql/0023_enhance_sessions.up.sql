CREATE TABLE IF NOT EXISTS device_sessions (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id),
    session_token VARCHAR(128) NOT NULL,
    ip_address VARCHAR(45),
    user_agent TEXT,
    device_fingerprint VARCHAR(255),
    device_type VARCHAR(32),
    browser VARCHAR(64),
    os VARCHAR(64),
    country_code VARCHAR(8),
    city VARCHAR(128),
    timezone VARCHAR(64),
    last_active_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL,
    is_revoked BOOLEAN NOT NULL DEFAULT FALSE,
    revoked_at TIMESTAMPTZ,
    revoked_by BIGINT REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    CONSTRAINT uq_device_sessions_token UNIQUE (session_token)
);

CREATE INDEX IF NOT EXISTS idx_device_sessions_user_id ON device_sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_device_sessions_token ON device_sessions(session_token);
CREATE INDEX IF NOT EXISTS idx_device_sessions_expires_at ON device_sessions(expires_at);
CREATE INDEX IF NOT EXISTS idx_device_sessions_is_revoked ON device_sessions(is_revoked);
CREATE INDEX IF NOT EXISTS idx_device_sessions_last_active ON device_sessions(last_active_at);
CREATE INDEX IF NOT EXISTS idx_device_sessions_fingerprint ON device_sessions(device_fingerprint);
