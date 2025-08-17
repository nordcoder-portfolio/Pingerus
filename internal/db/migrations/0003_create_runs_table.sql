-- +goose Up
CREATE TABLE runs (
                      id         BIGSERIAL PRIMARY KEY,
                      check_id   INT     NOT NULL REFERENCES checks(id) ON DELETE CASCADE,
                      ts         TIMESTAMP WITH TIME ZONE DEFAULT now() NOT NULL,
                      status     BOOLEAN NOT NULL,
                      latency_ms BIGINT NOT NULL,
                      code       INT
);
CREATE INDEX IF NOT EXISTS idx_runs_check_time ON runs(check_id, ts DESC);
-- +goose Down
DROP TABLE IF EXISTS runs;