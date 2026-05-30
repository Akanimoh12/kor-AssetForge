-- In-app notifications
CREATE TABLE IF NOT EXISTS notifications (
    id            BIGSERIAL PRIMARY KEY,
    user_id       BIGINT       NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type          VARCHAR(64)  NOT NULL,
    title         VARCHAR(255) NOT NULL,
    body          TEXT         NOT NULL,
    resource_id   BIGINT,
    resource_type VARCHAR(64),
    read          BOOLEAN      NOT NULL DEFAULT FALSE,
    read_at       TIMESTAMPTZ,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    deleted_at    TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_notifications_user_id  ON notifications(user_id);
CREATE INDEX IF NOT EXISTS idx_notifications_unread   ON notifications(user_id, read) WHERE read = FALSE;

-- Per-user notification channel preferences
CREATE TABLE IF NOT EXISTS notification_preferences (
    id                BIGSERIAL PRIMARY KEY,
    user_id           BIGINT      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    notification_type VARCHAR(64) NOT NULL,
    in_app            BOOLEAN     NOT NULL DEFAULT TRUE,
    email             BOOLEAN     NOT NULL DEFAULT TRUE,
    push              BOOLEAN     NOT NULL DEFAULT FALSE,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_user_notif_type UNIQUE (user_id, notification_type)
);
