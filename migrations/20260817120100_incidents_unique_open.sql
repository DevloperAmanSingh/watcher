-- Guarantees at most one unresolved incident per URL.
--
-- This is the whole basis of idempotent alerting. Check results arrive
-- concurrently — the event bus dispatches each handler in its own goroutine —
-- so two unhealthy results for the same URL can reach the incident path at the
-- same instant. Without a constraint, both read "no open incident", both
-- insert, and the operator is paged twice for one outage.
--
-- The index makes the second insert a no-op the database resolves itself, so
-- correctness does not depend on the application taking a lock or serialising
-- the write path. Under a correlated outage, when every monitored URL fails at
-- once, that is the difference between contention and none.

-- +goose Up
-- +goose StatementBegin
CREATE UNIQUE INDEX incidents_one_open_per_url
    ON incidents (url_id)
    WHERE resolved_at IS NULL;

-- Supports the trailing-window counts the analysis command runs, and the
-- lookup of a URL's most recent incident.
CREATE INDEX incidents_url_id_time_idx
    ON incidents (url_id, time DESC);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS incidents_url_id_time_idx;
DROP INDEX IF EXISTS incidents_one_open_per_url;
-- +goose StatementEnd
