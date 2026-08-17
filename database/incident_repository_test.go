package database

import (
	"context"
	"testing"
	"time"

	"github.com/DevloperAmanSingh/watcher/enums"
	"github.com/jackc/pgx/v5/pgxpool"
)

const day = 24 * time.Hour

// TestIncidentCountRespectsWindow is the regression test for Count ignoring its
// window. The previous implementation bucketed without filtering, so every
// window returned the same figure regardless of how far back it reached.
func TestIncidentCountRespectsWindow(t *testing.T) {
	pool := newTestPool(t)
	ctx := context.Background()
	repo := NewIncidentRepository(pool)

	urlID := insertURL(t, pool, "https://example.com")

	// One incident inside each successively wider window.
	insertIncidentAt(t, pool, urlID, 2*time.Hour) // within 1 day
	insertIncidentAt(t, pool, urlID, 3*day)       // within 7 days
	insertIncidentAt(t, pool, urlID, 20*day)      // within 30 days
	insertIncidentAt(t, pool, urlID, 200*day)     // within 365 days
	insertIncidentAt(t, pool, urlID, 400*day)     // outside every window

	cases := []struct {
		days int
		want int
	}{
		{days: 1, want: 1},
		{days: 7, want: 2},
		{days: 30, want: 3},
		{days: 365, want: 4},
	}

	for _, tc := range cases {
		got, err := repo.Count(ctx, urlID, tc.days, enums.Day)
		if err != nil {
			t.Fatalf("Count over %d days: %v", tc.days, err)
		}
		if got != tc.want {
			t.Errorf("Count over %d days = %d, want %d", tc.days, got, tc.want)
		}
	}
}

// TestIncidentCountIsScopedToURL guards against the count leaking across URLs.
func TestIncidentCountIsScopedToURL(t *testing.T) {
	pool := newTestPool(t)
	ctx := context.Background()
	repo := NewIncidentRepository(pool)

	mine := insertURL(t, pool, "https://mine.example")
	theirs := insertURL(t, pool, "https://theirs.example")

	insertIncidentAt(t, pool, mine, time.Hour)
	insertIncidentAt(t, pool, theirs, time.Hour)
	insertIncidentAt(t, pool, theirs, 2*time.Hour)

	got, err := repo.Count(ctx, mine, 7, enums.Day)
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if got != 1 {
		t.Errorf("Count for url %d = %d, want 1", mine, got)
	}
}

// TestIncidentCountEmptyIsZero checks that a URL with no incidents reports zero
// rather than an error. count(*) always returns a row, so there is no
// no-rows case for the caller to special-case.
func TestIncidentCountEmptyIsZero(t *testing.T) {
	pool := newTestPool(t)
	ctx := context.Background()
	repo := NewIncidentRepository(pool)

	urlID := insertURL(t, pool, "https://quiet.example")

	got, err := repo.Count(ctx, urlID, 30, enums.Day)
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if got != 0 {
		t.Errorf("Count = %d, want 0", got)
	}
}

// TestIncidentCountRejectsBadInput checks the guards on the interval, since the
// window is interpolated into an interval literal.
func TestIncidentCountRejectsBadInput(t *testing.T) {
	pool := newTestPool(t)
	ctx := context.Background()
	repo := NewIncidentRepository(pool)

	urlID := insertURL(t, pool, "https://example.org")

	if _, err := repo.Count(ctx, urlID, 0, enums.Day); err == nil {
		t.Error("Count with a zero window returned no error")
	}
	if _, err := repo.Count(ctx, urlID, 7, enums.DateType("fortnight")); err == nil {
		t.Error("Count with an unknown date type returned no error")
	}
}

// TestIncidentAddResolve covers the open/resolve lifecycle Count reports on.
func TestIncidentAddResolve(t *testing.T) {
	pool := newTestPool(t)
	ctx := context.Background()
	repo := NewIncidentRepository(pool)

	urlID := insertURL(t, pool, "https://flaky.example")

	if err := repo.Add(ctx, urlID); err != nil {
		t.Fatalf("Add: %v", err)
	}

	open := countOpenIncidents(t, pool, urlID)
	if open != 1 {
		t.Fatalf("open incidents after Add = %d, want 1", open)
	}

	if err := repo.Resolve(ctx, urlID); err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	if open = countOpenIncidents(t, pool, urlID); open != 0 {
		t.Errorf("open incidents after Resolve = %d, want 0", open)
	}

	// Resolving again must not fail or reopen anything.
	if err := repo.Resolve(ctx, urlID); err != nil {
		t.Fatalf("second Resolve: %v", err)
	}
	if open = countOpenIncidents(t, pool, urlID); open != 0 {
		t.Errorf("open incidents after second Resolve = %d, want 0", open)
	}
}

func countOpenIncidents(t *testing.T, pool *pgxpool.Pool, urlID int) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM incidents WHERE url_id = $1 AND resolved_at IS NULL`,
		urlID).Scan(&n); err != nil {
		t.Fatalf("counting open incidents: %v", err)
	}
	return n
}
