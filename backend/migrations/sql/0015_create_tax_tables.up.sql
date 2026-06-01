CREATE TABLE IF NOT EXISTS tax_records (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id),
    transaction_id BIGINT REFERENCES transactions(id),
    asset_id BIGINT NOT NULL REFERENCES assets(id),
    transaction_type VARCHAR(50) NOT NULL,
    quantity BIGINT NOT NULL,
    cost_basis_stroops NUMERIC(20, 8) NOT NULL,
    sale_price_stroops NUMERIC(20, 8),
    capital_gain_loss_stroops NUMERIC(20, 8),
    transaction_date TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS tax_reports (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id),
    tax_year INT NOT NULL,
    period_start TIMESTAMPTZ NOT NULL,
    period_end TIMESTAMPTZ NOT NULL,
    total_gain_stroops NUMERIC(20, 8) NOT NULL DEFAULT 0,
    total_loss_stroops NUMERIC(20, 8) NOT NULL DEFAULT 0,
    net_gain_loss_stroops NUMERIC(20, 8) NOT NULL DEFAULT 0,
    total_income_stroops NUMERIC(20, 8) NOT NULL DEFAULT 0,
    total_withheld_tax_stroops NUMERIC(20, 8) NOT NULL DEFAULT 0,
    transaction_count BIGINT NOT NULL DEFAULT 0,
    report_status VARCHAR(20) NOT NULL DEFAULT 'draft',
    generated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    exported_at TIMESTAMPTZ,
    UNIQUE(user_id, tax_year)
);

CREATE TABLE IF NOT EXISTS tax_1099_forms (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id),
    tax_report_id BIGINT NOT NULL REFERENCES tax_reports(id),
    form_type VARCHAR(20) NOT NULL DEFAULT '1099-NEC',
    filer_name VARCHAR(255) NOT NULL,
    filer_tin VARCHAR(20) NOT NULL,
    recipient_address TEXT NOT NULL,
    total_income NUMERIC(20, 8) NOT NULL,
    form_data TEXT NOT NULL,
    form_status VARCHAR(20) NOT NULL DEFAULT 'draft',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    signed_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS tax_withholdings (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id),
    transaction_id BIGINT REFERENCES transactions(id),
    withholding_amount_stroops NUMERIC(20, 8) NOT NULL,
    withholding_rate NUMERIC(5, 2) NOT NULL,
    withholding_date TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    status VARCHAR(20) NOT NULL DEFAULT 'completed'
);

CREATE TABLE IF NOT EXISTS tax_exports (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id),
    tax_report_id BIGINT REFERENCES tax_reports(id),
    export_type VARCHAR(50) NOT NULL,
    file_name VARCHAR(255) NOT NULL,
    file_path TEXT NOT NULL,
    file_format VARCHAR(20) NOT NULL DEFAULT 'PDF',
    exported_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_tax_records_user_id ON tax_records (user_id);
CREATE INDEX IF NOT EXISTS idx_tax_records_transaction_date ON tax_records (transaction_date DESC);
CREATE INDEX IF NOT EXISTS idx_tax_reports_user_id ON tax_reports (user_id);
CREATE INDEX IF NOT EXISTS idx_tax_reports_tax_year ON tax_reports (tax_year);
CREATE INDEX IF NOT EXISTS idx_tax_1099_user_id ON tax_1099_forms (user_id);
CREATE INDEX IF NOT EXISTS idx_tax_1099_status ON tax_1099_forms (form_status);
CREATE INDEX IF NOT EXISTS idx_tax_withholdings_user_id ON tax_withholdings (user_id);
CREATE INDEX IF NOT EXISTS idx_tax_exports_user_id ON tax_exports (user_id);
