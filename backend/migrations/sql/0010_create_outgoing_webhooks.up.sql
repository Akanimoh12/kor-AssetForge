-- Outgoing webhook subscriptions
CREATE TABLE IF NOT EXISTS webhook_subscriptions (
    id          BIGSERIAL PRIMARY KEY,
    user_id     BIGINT        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    url         TEXT          NOT NULL,
    secret      TEXT          NOT NULL,
    events      TEXT          NOT NULL,
    description TEXT,
    active      BOOLEAN       NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    deleted_at  TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_webhook_subscriptions_user_id ON webhook_subscriptions(user_id);

-- Delivery log for outgoing webhooks
CREATE TABLE IF NOT EXISTS webhook_delivery_logs (
    id              BIGSERIAL PRIMARY KEY,
    subscription_id BIGINT      NOT NULL REFERENCES webhook_subscriptions(id) ON DELETE CASCADE,
    event_type      VARCHAR(64) NOT NULL,
    payload         TEXT        NOT NULL,
    status          VARCHAR(20) NOT NULL DEFAULT 'pending',
    http_status     INT,
    response_body   TEXT,
    attempt_count   INT         NOT NULL DEFAULT 0,
    next_retry_at   TIMESTAMPTZ,
    delivered_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_webhook_delivery_logs_sub_id  ON webhook_delivery_logs(subscription_id);
CREATE INDEX IF NOT EXISTS idx_webhook_delivery_logs_status  ON webhook_delivery_logs(status);
CREATE INDEX IF NOT EXISTS idx_webhook_delivery_logs_retry   ON webhook_delivery_logs(next_retry_at);
