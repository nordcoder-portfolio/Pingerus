-- +goose Up
CREATE TABLE runs (
                      id         SERIAL PRIMARY KEY,
                      check_id   INT     NOT NULL REFERENCES checks(id) ON DELETE CASCADE,
                      ts         TIMESTAMP WITH TIME ZONE DEFAULT now() NOT NULL,
                      status     BOOLEAN NOT NULL,
                      latency_ms INT,
                      code       INT
);
-- +goose Down
DROP TABLE IF EXISTS runs;