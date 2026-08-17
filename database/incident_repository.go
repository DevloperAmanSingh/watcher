package database

import (
	"context"
	"fmt"
	"github.com/DevloperAmanSingh/watcher/enums"
	"github.com/jackc/pgx/v5/pgxpool"
)

type IncidentRepository interface {
	Add(ctx context.Context, urlId int) error
	Resolve(ctx context.Context, urlId int) error
	// Count reports how many incidents were opened for urlId within the
	// trailing window of `amount` dateType units ending now.
	Count(ctx context.Context, urlId int, amount int, dateType enums.DateType) (int, error)
}

type incidentRepository struct {
	pool *pgxpool.Pool
}

func (inc incidentRepository) Add(ctx context.Context, urlId int) error {
	sql := "INSERT INTO incidents (time, url_id) VALUES (NOW(), $1)"

	_, err := inc.pool.Exec(ctx, sql, urlId)
	if err != nil {
		return err
	}
	return nil
}

func (inc incidentRepository) Resolve(ctx context.Context, urlId int) error {
	sql := "UPDATE incidents SET resolved_at=NOW() WHERE url_id=$1 AND resolved_at IS NULL"

	_, err := inc.pool.Exec(ctx, sql, urlId)
	if err != nil {
		return err
	}
	return nil
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
