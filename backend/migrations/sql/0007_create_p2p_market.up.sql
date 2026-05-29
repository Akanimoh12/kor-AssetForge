CREATE TABLE IF NOT EXISTS p2p_orders (
    id BIGSERIAL PRIMARY KEY,
    asset_id BIGINT NOT NULL REFERENCES assets(id),
    owner_address VARCHAR(56) NOT NULL,
    side VARCHAR(4) NOT NULL CHECK (side IN ('buy', 'sell')),
    price BIGINT NOT NULL,
    quantity BIGINT NOT NULL,
    filled_quantity BIGINT NOT NULL DEFAULT 0,
    status VARCHAR(20) NOT NULL DEFAULT 'open',
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_p2p_orders_asset_id ON p2p_orders (asset_id);
CREATE INDEX IF NOT EXISTS idx_p2p_orders_owner_address ON p2p_orders (owner_address);
CREATE INDEX IF NOT EXISTS idx_p2p_orders_side ON p2p_orders (side);
CREATE INDEX IF NOT EXISTS idx_p2p_orders_status ON p2p_orders (status);
CREATE INDEX IF NOT EXISTS idx_p2p_orders_price ON p2p_orders (price);
CREATE INDEX IF NOT EXISTS idx_p2p_orders_deleted_at ON p2p_orders (deleted_at);

CREATE TABLE IF NOT EXISTS p2p_trades (
    id BIGSERIAL PRIMARY KEY,
    asset_id BIGINT NOT NULL REFERENCES assets(id),
    buy_order_id BIGINT NOT NULL REFERENCES p2p_orders(id),
    sell_order_id BIGINT NOT NULL REFERENCES p2p_orders(id),
    buyer_address VARCHAR(56) NOT NULL,
    seller_address VARCHAR(56) NOT NULL,
    price BIGINT NOT NULL,
    quantity BIGINT NOT NULL,
    total_value BIGINT NOT NULL,
    tx_hash VARCHAR(255),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_p2p_trades_asset_id ON p2p_trades (asset_id);
CREATE INDEX IF NOT EXISTS idx_p2p_trades_buy_order_id ON p2p_trades (buy_order_id);
CREATE INDEX IF NOT EXISTS idx_p2p_trades_sell_order_id ON p2p_trades (sell_order_id);
CREATE INDEX IF NOT EXISTS idx_p2p_trades_buyer_address ON p2p_trades (buyer_address);
CREATE INDEX IF NOT EXISTS idx_p2p_trades_seller_address ON p2p_trades (seller_address);
CREATE INDEX IF NOT EXISTS idx_p2p_trades_created_at ON p2p_trades (created_at DESC);

CREATE TABLE IF NOT EXISTS price_points (
    id BIGSERIAL PRIMARY KEY,
    asset_id BIGINT NOT NULL REFERENCES assets(id),
    price BIGINT NOT NULL,
    volume BIGINT NOT NULL DEFAULT 0,
    high BIGINT NOT NULL,
    low BIGINT NOT NULL,
    open BIGINT NOT NULL,
    close BIGINT NOT NULL,
    timestamp TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_price_points_asset_id ON price_points (asset_id);
CREATE INDEX IF NOT EXISTS idx_price_points_timestamp ON price_points (timestamp DESC);
