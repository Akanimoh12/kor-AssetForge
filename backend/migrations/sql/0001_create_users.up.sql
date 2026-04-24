CREATE TABLE IF NOT EXISTS users (
    id BIGSERIAL PRIMARY KEY,
    stellar_address VARCHAR(56) NOT NULL,
    email VARCHAR(255) NOT NULL,
    username VARCHAR(50) NOT NULL,
    password_hash TEXT NOT NULL,
    role VARCHAR(20) NOT NULL DEFAULT 'user',
    kyc_verified BOOLEAN NOT NULL DEFAULT false,
    accredited_investor BOOLEAN NOT NULL DEFAULT false,
    email_verified BOOLEAN NOT NULL DEFAULT false,
    email_token VARCHAR(64),
    email_token_expires TIMESTAMPTZ,
    password_reset_token VARCHAR(64),
    password_reset_expires TIMESTAMPTZ,
    last_login_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    CONSTRAINT uq_users_stellar_address UNIQUE (stellar_address),
    CONSTRAINT uq_users_email UNIQUE (email),
    CONSTRAINT uq_users_username UNIQUE (username)
);

CREATE INDEX IF NOT EXISTS idx_users_email_token ON users (email_token) WHERE email_token IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_users_password_reset_token ON users (password_reset_token) WHERE password_reset_token IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_users_deleted_at ON users (deleted_at);
