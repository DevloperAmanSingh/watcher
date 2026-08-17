-- +goose Up
-- +goose StatementBegin
CREATE TABLE url_statuses
(
    time     TIMESTAMPTZ NOT NULL,
    url_id   BIGINT      NOT NULL REFERENCES urls(id) ON DELETE CASCADE,
    status   BOOLEAN     NOT NULL
);

SELECT create_hypertable('url_statuses', 'time');

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE url_statuses;
-- +goose StatementEnd
