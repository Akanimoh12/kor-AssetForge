CREATE TABLE IF NOT EXISTS kyc_records (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id),
    provider_record_id VARCHAR(255),
    full_name VARCHAR(255) NOT NULL,
    date_of_birth DATE NOT NULL,
    nationality VARCHAR(100) NOT NULL,
    document_type VARCHAR(50) NOT NULL,
    document_number_hash VARCHAR(64) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    risk_score INTEGER NOT NULL DEFAULT 0,
    aml_cleared BOOLEAN NOT NULL DEFAULT false,
    accredited_investor BOOLEAN NOT NULL DEFAULT false,
    review_notes TEXT,
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_kyc_records_user_id ON kyc_records (user_id);
CREATE INDEX IF NOT EXISTS idx_kyc_records_status ON kyc_records (status);
CREATE INDEX IF NOT EXISTS idx_kyc_records_deleted_at ON kyc_records (deleted_at);

CREATE TABLE IF NOT EXISTS kyc_documents (
    id BIGSERIAL PRIMARY KEY,
    kyc_record_id BIGINT NOT NULL REFERENCES kyc_records(id),
    document_type VARCHAR(50) NOT NULL,
    file_name VARCHAR(255) NOT NULL,
    file_hash VARCHAR(64) NOT NULL,
    storage_path TEXT NOT NULL,
    mime_type VARCHAR(100) NOT NULL,
    size_bytes BIGINT NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_kyc_documents_kyc_record_id ON kyc_documents (kyc_record_id);

CREATE TABLE IF NOT EXISTS aml_screenings (
    id BIGSERIAL PRIMARY KEY,
    kyc_record_id BIGINT NOT NULL REFERENCES kyc_records(id),
    screening_id VARCHAR(255) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    risk_level VARCHAR(20) NOT NULL DEFAULT 'low',
    matches JSONB,
    screened_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_aml_screenings_kyc_record_id ON aml_screenings (kyc_record_id);

CREATE TABLE IF NOT EXISTS compliance_audit_logs (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id),
    action VARCHAR(100) NOT NULL,
    entity_type VARCHAR(50) NOT NULL,
    entity_id VARCHAR(255) NOT NULL,
    details TEXT,
    ip_address VARCHAR(45),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_compliance_audit_logs_user_id ON compliance_audit_logs (user_id);
CREATE INDEX IF NOT EXISTS idx_compliance_audit_logs_created_at ON compliance_audit_logs (created_at DESC);
