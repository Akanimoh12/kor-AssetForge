CREATE TABLE IF NOT EXISTS disputes (
    id BIGSERIAL PRIMARY KEY,
    transaction_id BIGINT NOT NULL REFERENCES transactions(id),
    filed_by_address VARCHAR(56) NOT NULL,
    respondent_addr VARCHAR(56) NOT NULL,
    reason TEXT NOT NULL,
    evidence TEXT,
    status VARCHAR(20) NOT NULL DEFAULT 'open',
    resolution VARCHAR(30),
    admin_notes TEXT,
    reviewed_by BIGINT,
    escrow_amount BIGINT NOT NULL DEFAULT 0,
    escrow_released BOOLEAN NOT NULL DEFAULT false,
    resolved_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_disputes_transaction_id ON disputes (transaction_id);
CREATE INDEX IF NOT EXISTS idx_disputes_filed_by_address ON disputes (filed_by_address);
CREATE INDEX IF NOT EXISTS idx_disputes_respondent_addr ON disputes (respondent_addr);
CREATE INDEX IF NOT EXISTS idx_disputes_status ON disputes (status);
CREATE INDEX IF NOT EXISTS idx_disputes_reviewed_by ON disputes (reviewed_by);
CREATE INDEX IF NOT EXISTS idx_disputes_deleted_at ON disputes (deleted_at);

CREATE TABLE IF NOT EXISTS dispute_escrows (
    id BIGSERIAL PRIMARY KEY,
    dispute_id BIGINT NOT NULL UNIQUE REFERENCES disputes(id),
    asset_id BIGINT NOT NULL REFERENCES assets(id),
    amount BIGINT NOT NULL,
    held_from TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    released_at TIMESTAMPTZ,
    released_to VARCHAR(56),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_dispute_escrows_dispute_id ON dispute_escrows (dispute_id);
CREATE INDEX IF NOT EXISTS idx_dispute_escrows_asset_id ON dispute_escrows (asset_id);
