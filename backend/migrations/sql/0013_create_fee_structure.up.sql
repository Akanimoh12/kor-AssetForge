CREATE TABLE IF NOT EXISTS fee_configurations (
    id BIGSERIAL PRIMARY KEY,
    asset_type VARCHAR(100) NOT NULL,
    base_fee_bps SMALLINT NOT NULL DEFAULT 50,
    description TEXT,
    active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(asset_type)
);

CREATE TABLE IF NOT EXISTS fee_discount_tiers (
    id BIGSERIAL PRIMARY KEY,
    fee_config_id BIGINT NOT NULL REFERENCES fee_configurations(id),
    tier_name VARCHAR(50) NOT NULL,
    min_volume_stroops NUMERIC(38, 0) NOT NULL,
    max_volume_stroops NUMERIC(38, 0),
    discount_bps SMALLINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS fee_transactions (
    id BIGSERIAL PRIMARY KEY,
    asset_id BIGINT NOT NULL REFERENCES assets(id),
    user_address VARCHAR(56) NOT NULL,
    transaction_hash VARCHAR(255),
    transaction_amount NUMERIC(20, 8) NOT NULL,
    asset_type VARCHAR(100) NOT NULL,
    base_fee_bps SMALLINT NOT NULL,
    applied_discount_bps SMALLINT NOT NULL DEFAULT 0,
    total_fee_stroops NUMERIC(20, 8) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'completed',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS fee_reports (
    id BIGSERIAL PRIMARY KEY,
    period_start TIMESTAMPTZ NOT NULL,
    period_end TIMESTAMPTZ NOT NULL,
    total_volume NUMERIC(38, 0) NOT NULL DEFAULT 0,
    total_fees_collected NUMERIC(20, 8) NOT NULL DEFAULT 0,
    transaction_count BIGINT NOT NULL DEFAULT 0,
    generated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_fee_configurations_asset_type ON fee_configurations (asset_type);
CREATE INDEX IF NOT EXISTS idx_fee_configurations_active ON fee_configurations (active);
CREATE INDEX IF NOT EXISTS idx_fee_discount_tiers_fee_config_id ON fee_discount_tiers (fee_config_id);
CREATE INDEX IF NOT EXISTS idx_fee_transactions_asset_id ON fee_transactions (asset_id);
CREATE INDEX IF NOT EXISTS idx_fee_transactions_user_address ON fee_transactions (user_address);
CREATE INDEX IF NOT EXISTS idx_fee_transactions_created_at ON fee_transactions (created_at DESC);
CREATE INDEX IF NOT EXISTS idx_fee_reports_period ON fee_reports (period_start, period_end);
