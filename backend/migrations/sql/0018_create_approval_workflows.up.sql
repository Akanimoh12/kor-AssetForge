CREATE TABLE approval_workflows (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL UNIQUE,
    asset_type VARCHAR(50),
    minimum_amount BIGINT NOT NULL DEFAULT 0,
    timeout_hours INTEGER NOT NULL DEFAULT 72 CHECK (timeout_hours > 0),
    active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE TABLE approval_steps (
    id BIGSERIAL PRIMARY KEY,
    workflow_id BIGINT NOT NULL REFERENCES approval_workflows(id) ON DELETE CASCADE,
    step_order INTEGER NOT NULL CHECK (step_order > 0),
    required_role VARCHAR(32),
    approver_user_id BIGINT REFERENCES users(id),
    required_approvals INTEGER NOT NULL DEFAULT 1 CHECK (required_approvals > 0),
    allow_delegation BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (workflow_id, step_order),
    CHECK (required_role IS NOT NULL OR approver_user_id IS NOT NULL)
);

CREATE TABLE approval_requests (
    id BIGSERIAL PRIMARY KEY,
    workflow_id BIGINT NOT NULL REFERENCES approval_workflows(id),
    transaction_id BIGINT NOT NULL UNIQUE REFERENCES transactions(id),
    requester_user_id BIGINT NOT NULL REFERENCES users(id),
    asset_id BIGINT NOT NULL REFERENCES assets(id),
    from_address VARCHAR(56) NOT NULL,
    to_address VARCHAR(56) NOT NULL,
    amount BIGINT NOT NULL CHECK (amount > 0),
    status VARCHAR(20) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'approved', 'rejected', 'expired')),
    current_step INTEGER NOT NULL DEFAULT 1,
    expires_at TIMESTAMPTZ NOT NULL,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE TABLE approval_actions (
    id BIGSERIAL PRIMARY KEY,
    approval_request_id BIGINT NOT NULL REFERENCES approval_requests(id) ON DELETE CASCADE,
    step_order INTEGER NOT NULL,
    approver_user_id BIGINT NOT NULL REFERENCES users(id),
    delegated_from_id BIGINT REFERENCES users(id),
    action VARCHAR(20) NOT NULL CHECK (action IN ('approved', 'rejected', 'delegated', 'expired')),
    comment TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_approval_workflows_active ON approval_workflows(active);
CREATE INDEX idx_approval_requests_status_expiry ON approval_requests(status, expires_at);
CREATE INDEX idx_approval_requests_requester ON approval_requests(requester_user_id);
CREATE INDEX idx_approval_actions_request_step ON approval_actions(approval_request_id, step_order);
