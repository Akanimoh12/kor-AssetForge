CREATE TABLE IF NOT EXISTS assets (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    symbol VARCHAR(20) NOT NULL,
    asset_type VARCHAR(50) NOT NULL,
    total_supply BIGINT NOT NULL DEFAULT 0,
    fractions BIGINT NOT NULL DEFAULT 1,
    contract_id VARCHAR(255),
    owner_address VARCHAR(56) NOT NULL,
    metadata TEXT,
    image_url TEXT,
    document_url TEXT,
    verified BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_assets_symbol ON assets (symbol);
CREATE INDEX IF NOT EXISTS idx_assets_owner_address ON assets (owner_address);
CREATE INDEX IF NOT EXISTS idx_assets_asset_type ON assets (asset_type);
CREATE INDEX IF NOT EXISTS idx_assets_verified ON assets (verified);
CREATE INDEX IF NOT EXISTS idx_assets_created_at ON assets (created_at DESC);
CREATE INDEX IF NOT EXISTS idx_assets_deleted_at ON assets (deleted_at);
