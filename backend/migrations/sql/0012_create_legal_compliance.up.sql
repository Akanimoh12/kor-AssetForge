-- Versioned legal documents (ToS, Privacy Policy, Cookie Policy)
CREATE TABLE IF NOT EXISTS legal_documents (
    id           BIGSERIAL PRIMARY KEY,
    type         VARCHAR(64)  NOT NULL,
    version      VARCHAR(20)  NOT NULL,
    content      TEXT         NOT NULL,
    effective_at TIMESTAMPTZ  NOT NULL,
    active       BOOLEAN      NOT NULL DEFAULT FALSE,
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_legal_documents_type ON legal_documents(type);

-- User consent records (auditable)
CREATE TABLE IF NOT EXISTS user_consents (
    id          BIGSERIAL PRIMARY KEY,
    user_id     BIGINT      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    document_id BIGINT      NOT NULL REFERENCES legal_documents(id),
    doc_type    VARCHAR(64) NOT NULL,
    version     VARCHAR(20) NOT NULL,
    ip_address  VARCHAR(45),
    user_agent  TEXT,
    accepted_at TIMESTAMPTZ NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at  TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_user_consents_user_id ON user_consents(user_id);

-- GDPR data export requests
CREATE TABLE IF NOT EXISTS data_export_requests (
    id           BIGSERIAL PRIMARY KEY,
    user_id      BIGINT      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status       VARCHAR(20) NOT NULL DEFAULT 'pending',
    download_url TEXT,
    expires_at   TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at   TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_data_export_requests_user_id ON data_export_requests(user_id);

-- Seed current active documents so the API returns something useful
INSERT INTO legal_documents (type, version, content, effective_at, active)
VALUES
  ('terms_of_service', '1.0',
   'These Terms of Service govern your use of the kor-AssetForge platform. By using the platform you agree to these terms.',
   NOW(), TRUE),
  ('privacy_policy', '1.0',
   'This Privacy Policy describes how kor-AssetForge collects, uses, and protects your personal information.',
   NOW(), TRUE),
  ('cookie_policy', '1.0',
   'This Cookie Policy explains how kor-AssetForge uses cookies and similar tracking technologies.',
   NOW(), TRUE)
ON CONFLICT DO NOTHING;
