package database

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// testDSN builds the connection string for the test database from the same
// DB_* variables the application reads, so `make dev` locally and the service
// containers in CI resolve to the same place.
func testDSN() string {
	get := func(key, fallback string) string {
		if value, ok := os.LookupEnv(key); ok && value != "" {
			return value
		}
		return fallback
	}
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		get("DB_USER", "watcher"),
		get("DB_PASSWORD", "watcher"),
		get("DB_HOST", "127.0.0.1"),
		get("DB_PORT", "5432"),
		get("DB_DATABASE", "watcher"),
	)
}

// newTestPool returns a pool against the test database and truncates every
// table, or skips when the stack is not running. Repository behavior depends on
// TimescaleDB specifics such as hypertables and interval casts that a mock
// would not reproduce, so these tests use a real database or none at all.
func newTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	if testing.Short() {
		t.Skip("skipping database test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, testDSN())
	if err != nil {
		t.Skipf("database unavailable (%v); run `make dev` to start the stack", err)
	}
	if pingErr := pool.Ping(ctx); pingErr != nil {
		pool.Close()
		t.Skipf("database unavailable (%v); run `make dev` to start the stack", pingErr)
	}

	var migrated bool
	err = pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'incidents')`,
	).Scan(&migrated)
	if err != nil || !migrated {
		pool.Close()
		t.Skip("schema not migrated; run `make migrate`")
	}

	t.Cleanup(pool.Close)
	truncateAll(t, pool)
	return pool
}

// truncateAll clears every table up front and again on cleanup, so a failing
// test cannot leak rows into the next one.
func truncateAll(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()

	clear := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_, err := pool.Exec(ctx, `TRUNCATE incidents, url_statuses, urls RESTART IDENTITY CASCADE`)
		if err != nil {
			t.Fatalf("truncating tables: %v", err)
		}
	}

	clear()
	t.Cleanup(clear)
}

// insertURL creates a monitored URL and returns its id.
func insertURL(t *testing.T, pool *pgxpool.Pool, target string) int {
	t.Helper()

	var id int
	err := pool.QueryRow(context.Background(),
		`INSERT INTO urls (url, http_method, contact_email, status, monitoring_frequency)
		 VALUES ($1, 'get', 'alerts@watcher.local', 'pending', 'five_minutes')
		 RETURNING id`, target).Scan(&id)
	if err != nil {
		t.Fatalf("inserting url %q: %v", target, err)
	}
	return id
}

// insertIncidentAt backdates an incident so window boundaries can be exercised
// without waiting for real time to pass.
func insertIncidentAt(t *testing.T, pool *pgxpool.Pool, urlID int, age time.Duration) {
	t.Helper()

	_, err := pool.Exec(context.Background(),
		`INSERT INTO incidents (time, url_id) VALUES (NOW() - $1::interval, $2)`,
		fmt.Sprintf("%d seconds", int(age.Seconds())), urlID)
	if err != nil {
		t.Fatalf("inserting incident aged %s: %v", age, err)
	}
}
