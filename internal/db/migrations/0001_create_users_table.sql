-- +goose Up
CREATE TABLE users (
                       id            SERIAL PRIMARY KEY,
                       email         TEXT    UNIQUE   NOT NULL,
                       password_hash TEXT    NOT NULL,
                       is_active     BOOLEAN DEFAULT TRUE NOT NULL,
                       created_at    TIMESTAMP WITH TIME ZONE DEFAULT now() NOT NULL,
                       updated_at    TIMESTAMP WITH TIME ZONE DEFAULT now() NOT NULL
);
-- +goose Down
DROP TABLE IF EXISTS users;