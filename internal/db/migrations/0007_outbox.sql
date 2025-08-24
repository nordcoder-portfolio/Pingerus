-- +goose Up
CREATE TYPE outbox_status AS ENUM ('CREATED', 'IN_PROGRESS', 'SUCCESS');

CREATE TABLE IF NOT EXISTS outbox
(
    idempotency_key TEXT PRIMARY KEY,
    data            JSONB                                  NOT NULL,
    status          outbox_status                          NOT NULL,
    kind            INT                                    NOT NULL,
    created_at      TIMESTAMP WITH TIME ZONE DEFAULT now() NOT NULL,
    updated_at      TIMESTAMP WITH TIME ZONE DEFAULT now() NOT NULL,
    traceparent     TEXT,
    tracestate      TEXT,
    baggage         TEXT
);

CREATE INDEX IF NOT EXISTS idx_outbox_status_created_at
    ON outbox (status, created_at);

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION update_outbox_timestamp() RETURNS TRIGGER AS
$$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd


CREATE OR REPLACE TRIGGER trigger_update_outbox_timestamp
    BEFORE UPDATE
    ON outbox
    FOR EACH ROW
EXECUTE FUNCTION update_outbox_timestamp();

-- +goose Down
DROP TRIGGER IF EXISTS trg_outbox_updated_at ON outbox;
DROP FUNCTION IF EXISTS set_updated_at();
DROP TABLE IF EXISTS outbox;
DROP TYPE IF EXISTS outbox_status;