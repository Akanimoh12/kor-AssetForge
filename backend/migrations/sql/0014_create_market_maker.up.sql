CREATE TABLE IF NOT EXISTS market_maker_bots (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'inactive',
    managed_asset_id BIGINT NOT NULL REFERENCES assets(id),
    operator_address VARCHAR(56) NOT NULL,
    min_spread_bps SMALLINT NOT NULL DEFAULT 50,
    max_position_stroops NUMERIC(38, 0) NOT NULL,
    inventory_target_stroops NUMERIC(38, 0) NOT NULL,
    total_volume_stroops NUMERIC(38, 0) NOT NULL DEFAULT 0,
    profit_loss_stroops NUMERIC(20, 8) NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS market_maker_orders (
    id BIGSERIAL PRIMARY KEY,
    bot_id BIGINT NOT NULL REFERENCES market_maker_bots(id),
    order_type VARCHAR(10) NOT NULL,
    asset_id BIGINT NOT NULL REFERENCES assets(id),
    price_stroops NUMERIC(20, 8) NOT NULL,
    amount_units BIGINT NOT NULL,
    filled_units BIGINT NOT NULL DEFAULT 0,
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    order_hash VARCHAR(255),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS market_maker_trades (
    id BIGSERIAL PRIMARY KEY,
    bot_id BIGINT NOT NULL REFERENCES market_maker_bots(id),
    order_id BIGINT NOT NULL REFERENCES market_maker_orders(id),
    counterparty_address VARCHAR(56) NOT NULL,
    side VARCHAR(10) NOT NULL,
    price_stroops NUMERIC(20, 8) NOT NULL,
    amount_units BIGINT NOT NULL,
    fee_stroops NUMERIC(20, 8) NOT NULL,
    profit_stroops NUMERIC(20, 8),
    tx_hash VARCHAR(255),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS market_maker_inventory (
    id BIGSERIAL PRIMARY KEY,
    bot_id BIGINT NOT NULL REFERENCES market_maker_bots(id),
    asset_id BIGINT NOT NULL REFERENCES assets(id),
    held_units BIGINT NOT NULL DEFAULT 0,
    cost_basis_stroops NUMERIC(20, 8) NOT NULL DEFAULT 0,
    unrealized_pnl_stroops NUMERIC(20, 8) NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(bot_id, asset_id)
);

CREATE TABLE IF NOT EXISTS market_maker_health_checks (
    id BIGSERIAL PRIMARY KEY,
    bot_id BIGINT NOT NULL REFERENCES market_maker_bots(id),
    is_healthy BOOLEAN NOT NULL,
    uptime_percentage NUMERIC(5, 2),
    last_trade_time TIMESTAMPTZ,
    active_orders_count BIGINT NOT NULL DEFAULT 0,
    message TEXT,
    checked_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_market_maker_bots_status ON market_maker_bots (status);
CREATE INDEX IF NOT EXISTS idx_market_maker_bots_operator ON market_maker_bots (operator_address);
CREATE INDEX IF NOT EXISTS idx_market_maker_orders_bot_id ON market_maker_orders (bot_id);
CREATE INDEX IF NOT EXISTS idx_market_maker_orders_status ON market_maker_orders (status);
CREATE INDEX IF NOT EXISTS idx_market_maker_trades_bot_id ON market_maker_trades (bot_id);
CREATE INDEX IF NOT EXISTS idx_market_maker_trades_created_at ON market_maker_trades (created_at DESC);
CREATE INDEX IF NOT EXISTS idx_market_maker_inventory_bot_id ON market_maker_inventory (bot_id);
CREATE INDEX IF NOT EXISTS idx_market_maker_health_bot_id ON market_maker_health_checks (bot_id);
CREATE INDEX IF NOT EXISTS idx_market_maker_health_checked_at ON market_maker_health_checks (checked_at DESC);
