CREATE TABLE IF NOT EXISTS multi_sig_wallets (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    owner_ids TEXT NOT NULL,
    threshold INT NOT NULL,
    contract_id VARCHAR(255) UNIQUE,
    created_by_id BIGINT NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS multi_sig_proposals (
    id BIGSERIAL PRIMARY KEY,
    wallet_id BIGINT NOT NULL REFERENCES multi_sig_wallets(id),
    proposer_id BIGINT NOT NULL REFERENCES users(id),
    to_address VARCHAR(255) NOT NULL,
    amount BIGINT NOT NULL,
    description TEXT,
    status VARCHAR(32) NOT NULL DEFAULT 'pending',
    sign_count INT NOT NULL DEFAULT 0,
    tx_hash VARCHAR(255),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_multi_sig_proposals_wallet_id ON multi_sig_proposals(wallet_id);
CREATE INDEX IF NOT EXISTS idx_multi_sig_proposals_status ON multi_sig_proposals(status);

CREATE TABLE IF NOT EXISTS multi_sig_signatures (
    id BIGSERIAL PRIMARY KEY,
    proposal_id BIGINT NOT NULL REFERENCES multi_sig_proposals(id),
    signer_id BIGINT NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_multisig_sig UNIQUE (proposal_id, signer_id)
);
