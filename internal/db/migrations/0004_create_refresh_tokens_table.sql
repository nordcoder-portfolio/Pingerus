-- +goose Up
CREATE TABLE refresh_tokens (
                                id           SERIAL PRIMARY KEY,
                                user_id      INT     NOT NULL REFERENCES users(id) ON DELETE CASCADE,
                                token_hash   TEXT    NOT NULL,
                                issued_at    TIMESTAMP WITH TIME ZONE DEFAULT now() NOT NULL,
                                expires_at   TIMESTAMP WITH TIME ZONE NOT NULL,
                                revoked      BOOLEAN DEFAULT FALSE NOT NULL
);
CREATE INDEX idx_refresh_tokens_user   ON refresh_tokens(user_id);
CREATE INDEX idx_refresh_tokens_hash   ON refresh_tokens(token_hash);
CREATE INDEX idx_refresh_tokens_expires ON refresh_tokens(expires_at);
-- +goose Down
DROP TABLE IF EXISTS refresh_tokens;