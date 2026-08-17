-- +goose Up
-- +goose StatementBegin
CREATE TABLE incidents
(
    time     TIMESTAMPTZ NOT NULL,
    url_id   BIGINT      NOT NULL REFERENCES urls(id) ON DELETE CASCADE,
    resolved_at TIMESTAMPTZ DEFAULT NULL
);

SELECT create_hypertable('incidents', 'time');

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE incidents;
-- +goose StatementEnd
