-- +goose Up
CREATE TABLE notifications
(
    id        SERIAL PRIMARY KEY,
    check_id  INT         NOT NULL REFERENCES checks (id) ON DELETE CASCADE,
    user_id   INT         NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    type      TEXT        NOT NULL,
    sent_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    payload   TEXT        NOT NULL
);

CREATE INDEX idx_notifications_user_time
    ON notifications (user_id, sent_at DESC);

-- +goose Down
DROP TABLE IF EXISTS notifications


