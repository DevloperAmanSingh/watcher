-- Converts incidents from a hypertable to an ordinary table.
--
-- A hypertable cannot carry a unique index that omits its partitioning column,
-- so `UNIQUE (url_id) WHERE resolved_at IS NULL` — the constraint that makes
-- opening an incident idempotent — is rejected outright while incidents is
-- partitioned on time.
--
-- Partitioning bought nothing here. incidents holds one row per outage, while
-- url_statuses holds one per check; the two differ by orders of magnitude, and
-- only the latter needs a hypertable. Trading time partitioning for a
-- uniqueness guarantee is the right side of that exchange.

-- +goose Up
-- +goose StatementBegin
CREATE TABLE incidents_regular
(
    id          BIGSERIAL PRIMARY KEY,
    url_id      BIGINT      NOT NULL REFERENCES urls (id) ON DELETE CASCADE,
    time        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    resolved_at TIMESTAMPTZ DEFAULT NULL
);

INSERT INTO incidents_regular (url_id, time, resolved_at)
SELECT url_id, time, resolved_at
FROM incidents;

DROP TABLE incidents;

ALTER TABLE incidents_regular RENAME TO incidents;
ALTER SEQUENCE incidents_regular_id_seq RENAME TO incidents_id_seq;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
CREATE TABLE incidents_hyper
(
    time        TIMESTAMPTZ NOT NULL,
    url_id      BIGINT      NOT NULL REFERENCES urls (id) ON DELETE CASCADE,
    resolved_at TIMESTAMPTZ DEFAULT NULL
);

SELECT create_hypertable('incidents_hyper', 'time');

INSERT INTO incidents_hyper (time, url_id, resolved_at)
SELECT time, url_id, resolved_at
FROM incidents;

DROP TABLE incidents;

ALTER TABLE incidents_hyper RENAME TO incidents;
-- +goose StatementEnd
