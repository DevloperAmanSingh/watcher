-- Tracks how many checks in a row a URL has failed.
--
-- A single failed check is not an outage. Packet loss, a brief deploy, or one
-- slow response all produce one failure, and alerting on it pages a human for
-- something already over by the time they read it.
--
-- The counter lives on the row rather than in the application so it can be
-- incremented atomically:
--
--   UPDATE urls SET consecutive_failures = consecutive_failures + 1
--   WHERE id = $1 RETURNING consecutive_failures
--
-- Concurrent results for one URL are serialized by the row lock the UPDATE
-- already takes, and each caller receives a distinct value. Exactly one
-- observes the threshold, so the decision to open an incident needs no
-- coordination above the database.

-- +goose Up
-- +goose StatementBegin
ALTER TABLE urls
    ADD COLUMN consecutive_failures INTEGER NOT NULL DEFAULT 0;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE urls
    DROP COLUMN consecutive_failures;
-- +goose StatementEnd
