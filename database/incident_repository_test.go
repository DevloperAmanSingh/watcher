package database

import (
	"context"
	"sync"
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

// TestIncidentOpenResolveLifecycle covers the open/resolve cycle and, more
// importantly, the boolean each returns. That value is what the alerting path
// uses to decide whether to notify, so a wrong answer means either a missed
// page or a duplicate one.
func TestIncidentOpenResolveLifecycle(t *testing.T) {
	pool := newTestPool(t)
	ctx := context.Background()
	repo := NewIncidentRepository(pool)

	urlID := insertURL(t, pool, "https://flaky.example")

	opened, err := repo.Open(ctx, urlID)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if !opened {
		t.Fatal("Open reported no incident created on a URL with none open")
	}
	if n := countOpenIncidents(t, pool, urlID); n != 1 {
		t.Fatalf("open incidents after Open = %d, want 1", n)
	}

	// Opening again must be a silent no-op reporting false, so a replayed
	// unhealthy result cannot page twice.
	opened, err = repo.Open(ctx, urlID)
	if err != nil {
		t.Fatalf("second Open: %v", err)
	}
	if opened {
		t.Error("second Open reported an incident created; it would send a duplicate alert")
	}
	if n := countOpenIncidents(t, pool, urlID); n != 1 {
		t.Errorf("open incidents after second Open = %d, want 1", n)
	}

	resolved, err := repo.Resolve(ctx, urlID)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if !resolved {
		t.Error("Resolve reported nothing closed while an incident was open")
	}
	if n := countOpenIncidents(t, pool, urlID); n != 0 {
		t.Errorf("open incidents after Resolve = %d, want 0", n)
	}

	// Resolving again must report false rather than erroring or reopening.
	resolved, err = repo.Resolve(ctx, urlID)
	if err != nil {
		t.Fatalf("second Resolve: %v", err)
	}
	if resolved {
		t.Error("second Resolve reported a closure; it would send a duplicate recovery alert")
	}

	// A subsequent outage is a new incident, not a reopening of the old one.
	opened, err = repo.Open(ctx, urlID)
	if err != nil {
		t.Fatalf("Open after resolve: %v", err)
	}
	if !opened {
		t.Error("Open after a resolved incident reported nothing created")
	}

	var total int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM incidents WHERE url_id = $1`, urlID).Scan(&total); err != nil {
		t.Fatalf("counting incidents: %v", err)
	}
	if total != 2 {
		t.Errorf("total incidents = %d, want 2", total)
	}
}

// TestIncidentOpenIsConcurrencySafe is the test that backs the exactly-once
// claim. Many goroutines report the same URL unhealthy at once, exactly as the
// event bus does when it dispatches each handler independently. Exactly one
// must be told it opened the incident, because exactly one alert may be sent.
func TestIncidentOpenIsConcurrencySafe(t *testing.T) {
	pool := newTestPool(t)
	ctx := context.Background()
	repo := NewIncidentRepository(pool)

	urlID := insertURL(t, pool, "https://stampede.example")

	const racers = 24

	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		openings int
		errs     []error
	)

	start := make(chan struct{})
	for i := 0; i < racers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start // release everyone at once to maximize overlap
			opened, err := repo.Open(ctx, urlID)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errs = append(errs, err)
				return
			}
			if opened {
				openings++
			}
		}()
	}
	close(start)
	wg.Wait()

	for _, err := range errs {
		t.Errorf("Open returned an error under contention: %v", err)
	}
	if openings != 1 {
		t.Errorf("%d of %d concurrent Open calls reported success, want exactly 1", openings, racers)
	}
	if n := countOpenIncidents(t, pool, urlID); n != 1 {
		t.Errorf("open incidents = %d, want 1", n)
	}
}

// TestIncidentResolveIsConcurrencySafe is the recovery-side equivalent.
func TestIncidentResolveIsConcurrencySafe(t *testing.T) {
	pool := newTestPool(t)
	ctx := context.Background()
	repo := NewIncidentRepository(pool)

	urlID := insertURL(t, pool, "https://recovery.example")
	if _, err := repo.Open(ctx, urlID); err != nil {
		t.Fatalf("Open: %v", err)
	}

	const racers = 24

	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		closures int
		errs     []error
	)

	start := make(chan struct{})
	for i := 0; i < racers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			resolved, err := repo.Resolve(ctx, urlID)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errs = append(errs, err)
				return
			}
			if resolved {
				closures++
			}
		}()
	}
	close(start)
	wg.Wait()

	for _, err := range errs {
		t.Errorf("Resolve returned an error under contention: %v", err)
	}
	if closures != 1 {
		t.Errorf("%d of %d concurrent Resolve calls reported success, want exactly 1", closures, racers)
	}
}

// TestOnlyOneOpenIncidentPerURL pins the partial unique index. A second
// unresolved incident for the same URL must be rejected, while a URL may
// accumulate any number of resolved ones.
func TestOnlyOneOpenIncidentPerURL(t *testing.T) {
	pool := newTestPool(t)
	ctx := context.Background()

	urlID := insertURL(t, pool, "https://constrained.example")

	// Historical, resolved incidents are unconstrained.
	insertIncidentAt(t, pool, urlID, 10*day)
	insertIncidentAt(t, pool, urlID, 5*day)

	insertOpenIncidentAt(t, pool, urlID, time.Hour)

	_, err := pool.Exec(ctx, `INSERT INTO incidents (url_id) VALUES ($1)`, urlID)
	if err == nil {
		t.Fatal("a second unresolved incident was accepted; the unique index is not in effect")
	}

	// The same insert with ON CONFLICT DO NOTHING must be a silent no-op,
	// since that is what makes opening an incident idempotent.
	tag, err := pool.Exec(ctx,
		`INSERT INTO incidents (url_id) VALUES ($1) ON CONFLICT DO NOTHING`, urlID)
	if err != nil {
		t.Fatalf("ON CONFLICT DO NOTHING returned an error: %v", err)
	}
	if n := tag.RowsAffected(); n != 0 {
		t.Errorf("ON CONFLICT DO NOTHING inserted %d rows, want 0", n)
	}

	if open := countOpenIncidents(t, pool, urlID); open != 1 {
		t.Errorf("open incidents = %d, want 1", open)
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
