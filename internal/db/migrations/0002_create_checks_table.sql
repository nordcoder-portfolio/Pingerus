-- +goose Up
CREATE TABLE checks (
                        id            SERIAL PRIMARY KEY,
                        user_id       INT     NOT NULL REFERENCES users(id) ON DELETE CASCADE,
                        host          TEXT    NOT NULL,
                        interval_sec  INT     NOT NULL,
                        last_status   BOOLEAN,
                        next_run      TIMESTAMP WITH TIME ZONE,
                        active        BOOLEAN DEFAULT TRUE NOT NULL,
                        created_at    TIMESTAMP WITH TIME ZONE DEFAULT now() NOT NULL,
                        updated_at    TIMESTAMP WITH TIME ZONE DEFAULT now() NOT NULL
);
-- +goose Down
DROP TABLE IF EXISTS checks;