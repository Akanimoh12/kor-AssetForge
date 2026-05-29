CREATE TABLE IF NOT EXISTS stake_positions (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id),
    asset_id BIGINT NOT NULL REFERENCES assets(id),
    staked_amount BIGINT NOT NULL,
    accrued_rewards BIGINT NOT NULL DEFAULT 0,
    claimed_rewards BIGINT NOT NULL DEFAULT 0,
    staked_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_reward_at TIMESTAMPTZ,
    stellar_address VARCHAR(56) NOT NULL,
    active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_stake_positions_user_id ON stake_positions (user_id);
CREATE INDEX IF NOT EXISTS idx_stake_positions_asset_id ON stake_positions (asset_id);
CREATE INDEX IF NOT EXISTS idx_stake_positions_active ON stake_positions (active);
CREATE INDEX IF NOT EXISTS idx_stake_positions_deleted_at ON stake_positions (deleted_at);

CREATE TABLE IF NOT EXISTS reward_distributions (
    id BIGSERIAL PRIMARY KEY,
    asset_id BIGINT NOT NULL REFERENCES assets(id),
    total_distributed BIGINT NOT NULL,
    staker_count INT NOT NULL,
    apr_basis_points INT NOT NULL,
    period_start TIMESTAMPTZ NOT NULL,
    period_end TIMESTAMPTZ NOT NULL,
    tx_hash VARCHAR(255),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_reward_distributions_asset_id ON reward_distributions (asset_id);
CREATE INDEX IF NOT EXISTS idx_reward_distributions_created_at ON reward_distributions (created_at DESC);

CREATE TABLE IF NOT EXISTS reward_claims (
    id BIGSERIAL PRIMARY KEY,
    stake_id BIGINT NOT NULL REFERENCES stake_positions(id),
    user_id BIGINT NOT NULL REFERENCES users(id),
    asset_id BIGINT NOT NULL REFERENCES assets(id),
    amount BIGINT NOT NULL,
    tx_hash VARCHAR(255),
    claimed_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_reward_claims_stake_id ON reward_claims (stake_id);
CREATE INDEX IF NOT EXISTS idx_reward_claims_user_id ON reward_claims (user_id);
CREATE INDEX IF NOT EXISTS idx_reward_claims_asset_id ON reward_claims (asset_id);
