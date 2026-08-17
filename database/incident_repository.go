package database

import (
	"context"
	"fmt"
	"github.com/DevloperAmanSingh/watcher/enums"
	"github.com/jackc/pgx/v5/pgxpool"
)

// IncidentRepository records outages.
//
// Open and Resolve are idempotent and report whether they changed anything.
// That return value is the signal callers use to decide whether to notify: an
// alert belongs to the caller that actually caused the transition, so replaying
// a result cannot produce a second notification.
type IncidentRepository interface {
	// Open starts an incident for urlId and reports whether one was created.
	// It returns false when an unresolved incident already exists, which the
	// partial unique index makes an ordinary outcome rather than an error.
	Open(ctx context.Context, urlId int) (bool, error)
	// Resolve closes the open incident for urlId and reports whether one was
	// closed. It returns false when there was nothing open.
	Resolve(ctx context.Context, urlId int) (bool, error)
	// Count reports how many incidents were opened for urlId within the
	// trailing window of `amount` dateType units ending now.
	Count(ctx context.Context, urlId int, amount int, dateType enums.DateType) (int, error)
}

type incidentRepository struct {
	pool *pgxpool.Pool
}

func (inc incidentRepository) Open(ctx context.Context, urlId int) (bool, error) {
	// ON CONFLICT defers the race to the database. Concurrent callers all
	// issue this insert; exactly one affects a row, and the rest are told so
	// without an error and without either side taking a lock.
	sql := `INSERT INTO incidents (time, url_id) VALUES (NOW(), $1)
	        ON CONFLICT (url_id) WHERE resolved_at IS NULL DO NOTHING`

	tag, err := inc.pool.Exec(ctx, sql, urlId)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

func (inc incidentRepository) Resolve(ctx context.Context, urlId int) (bool, error) {
	// The resolved_at IS NULL predicate makes this idempotent in the same way:
	// the first caller closes the incident, later ones match no rows.
	sql := "UPDATE incidents SET resolved_at = NOW() WHERE url_id = $1 AND resolved_at IS NULL"

	tag, err := inc.pool.Exec(ctx, sql, urlId)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// Count returns how many incidents were opened for a URL within the trailing
// window ending now, expressed as an amount of dateType units.
func (inc incidentRepository) Count(ctx context.Context, urlId int, amount int, dateType enums.DateType) (int, error) {
	unit := dateType.ToString()
	if unit == "" {
		return 0, fmt.Errorf("invalid date type %q", string(dateType))
	}
	if amount <= 0 {
		return 0, fmt.Errorf("window amount must be positive, got %d", amount)
	}

	// The window has to be applied in the query. Bucketing alone does not
	// restrict which rows are counted, it only groups them.
	sql := `SELECT count(*) FROM incidents WHERE url_id = $1 AND time >= NOW() - $2::interval`

	var count int
	if err := inc.pool.QueryRow(ctx, sql, urlId, fmt.Sprintf("%d %s", amount, unit)).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func NewIncidentRepository(pool *pgxpool.Pool) IncidentRepository {
	return incidentRepository{
		pool: pool,
	}
}
