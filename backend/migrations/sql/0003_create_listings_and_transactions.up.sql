CREATE TABLE IF NOT EXISTS listings (
    id BIGSERIAL PRIMARY KEY,
    asset_id BIGINT NOT NULL REFERENCES assets(id),
    amount BIGINT NOT NULL,
    price_per_unit NUMERIC(20, 8) NOT NULL,
    seller_addr VARCHAR(56) NOT NULL,
    listing_id VARCHAR(255),
    active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_listings_asset_id ON listings (asset_id);
CREATE INDEX IF NOT EXISTS idx_listings_seller_addr ON listings (seller_addr);
CREATE INDEX IF NOT EXISTS idx_listings_active ON listings (active);
CREATE INDEX IF NOT EXISTS idx_listings_deleted_at ON listings (deleted_at);

CREATE TABLE IF NOT EXISTS transactions (
    id BIGSERIAL PRIMARY KEY,
    asset_id BIGINT NOT NULL REFERENCES assets(id),
    amount BIGINT NOT NULL,
    from_address VARCHAR(56) NOT NULL,
    to_address VARCHAR(56) NOT NULL,
    tx_hash VARCHAR(255),
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_transactions_asset_id ON transactions (asset_id);
CREATE INDEX IF NOT EXISTS idx_transactions_from_address ON transactions (from_address);
CREATE INDEX IF NOT EXISTS idx_transactions_to_address ON transactions (to_address);
CREATE INDEX IF NOT EXISTS idx_transactions_status ON transactions (status);
CREATE INDEX IF NOT EXISTS idx_transactions_created_at ON transactions (created_at DESC);
