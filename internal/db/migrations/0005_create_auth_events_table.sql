-- +goose Up
CREATE TABLE auth_events (
                             id          BIGSERIAL PRIMARY KEY,
                             user_id     INT       REFERENCES users(id),
                             event_type  TEXT      NOT NULL,
                             ip_address  INET,
                             user_agent  TEXT,
                             created_at  TIMESTAMP WITH TIME ZONE DEFAULT now() NOT NULL
);
CREATE INDEX idx_auth_events_user_time ON auth_events(user_id, created_at);
-- +goose Down
DROP TABLE IF EXISTS auth_events;