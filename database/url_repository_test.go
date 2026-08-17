package database

import (
	"context"
	"sync"
	"testing"

	"github.com/DevloperAmanSingh/watcher/enums"
)

// TestRecordFailureCountsUp checks the counter advances one per call and that
// a reset returns it to zero.
func TestRecordFailureCountsUp(t *testing.T) {
	pool := newTestPool(t)
	ctx := context.Background()
	repo := NewUrlRepository(pool)

	urlID := insertURL(t, pool, "https://counter.example")

	for want := 1; want <= 3; want++ {
		got, err := repo.RecordFailure(ctx, urlID)
		if err != nil {
			t.Fatalf("RecordFailure: %v", err)
		}
		if got != want {
			t.Fatalf("RecordFailure returned %d, want %d", got, want)
		}
	}

	reset, err := repo.ResetFailures(ctx, urlID)
	if err != nil {
		t.Fatalf("ResetFailures: %v", err)
	}
	if !reset {
		t.Error("ResetFailures reported no change while the counter was non-zero")
	}

	// Resetting an already-zero counter reports no change, so a steady healthy
	// run is distinguishable from a recovery.
	if reset, err = repo.ResetFailures(ctx, urlID); err != nil {
		t.Fatalf("second ResetFailures: %v", err)
	}
	if reset {
		t.Error("ResetFailures reported a change on an already-zero counter")
	}

	got, err := repo.RecordFailure(ctx, urlID)
	if err != nil {
		t.Fatalf("RecordFailure after reset: %v", err)
	}
	if got != 1 {
		t.Errorf("RecordFailure after reset returned %d, want 1", got)
	}
}

// TestRecordFailureIsAtomic is what lets the alerting path trust the counter.
// Concurrent unhealthy results for one URL must each receive a distinct value,
// so exactly one observes the alert threshold. A read-modify-write would hand
// the same number to several callers and page more than once.
func TestRecordFailureIsAtomic(t *testing.T) {
	pool := newTestPool(t)
	ctx := context.Background()
	repo := NewUrlRepository(pool)

	urlID := insertURL(t, pool, "https://atomic.example")

	const racers = 24

	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		seen = make(map[int]int)
		errs []error
	)

	start := make(chan struct{})
	for i := 0; i < racers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			n, err := repo.RecordFailure(ctx, urlID)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errs = append(errs, err)
				return
			}
			seen[n]++
		}()
	}
	close(start)
	wg.Wait()

	for _, err := range errs {
		t.Errorf("RecordFailure errored under contention: %v", err)
	}

	for value, count := range seen {
		if count != 1 {
			t.Errorf("value %d was returned to %d callers, want 1", value, count)
		}
	}
	if len(seen) != racers {
		t.Errorf("saw %d distinct values across %d calls, want %d", len(seen), racers, racers)
	}

	// Exactly one caller can have observed any given threshold.
	if seen[3] != 1 {
		t.Errorf("threshold value 3 was observed %d times, want exactly 1", seen[3])
	}
}

// TestUrlEachPagesBeyondOnePage covers the paging added when the hardcoded
// ten-URL limit was removed. The page size is larger than any sensible fixture,
// so this asserts the callback sees every row rather than the first page.
func TestUrlEachVisitsEveryRow(t *testing.T) {
	pool := newTestPool(t)
	ctx := context.Background()
	repo := NewUrlRepository(pool)

	const total = 25 // more than the old hardcoded limit of 10
	for i := 0; i < total; i++ {
		insertURL(t, pool, "https://each.example/"+string(rune('a'+i%26))+itoa(i))
	}

	var seen int
	err := repo.Each(ctx, UrlQueryFilter{}, func(Url) error {
		seen++
		return nil
	})
	if err != nil {
		t.Fatalf("Each: %v", err)
	}
	if seen != total {
		t.Errorf("Each visited %d urls, want %d", seen, total)
	}
}

// TestUrlEachAppliesFilter checks the filter reaches the query rather than
// being silently dropped.
func TestUrlEachAppliesFilter(t *testing.T) {
	pool := newTestPool(t)
	ctx := context.Background()
	repo := NewUrlRepository(pool)

	insertURL(t, pool, "https://filter.example/one")
	insertURL(t, pool, "https://filter.example/two")

	id := insertURL(t, pool, "https://filter.example/three")
	if err := repo.UpdateStatus(ctx, id, enums.Healthy); err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}

	var seen int
	err := repo.Each(ctx, UrlQueryFilter{Status: enums.Healthy}, func(u Url) error {
		seen++
		if u.Id != id {
			t.Errorf("Each yielded url %d, want only %d", u.Id, id)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Each: %v", err)
	}
	if seen != 1 {
		t.Errorf("Each visited %d urls, want 1", seen)
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}
