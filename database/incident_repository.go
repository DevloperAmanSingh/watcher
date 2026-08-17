package database

import (
	"context"
	"fmt"
	"github.com/DevloperAmanSingh/watcher/enums"
	"github.com/jackc/pgx/v5/pgxpool"
	"time"
)

type IncidentRepository interface {
	Add(ctx context.Context, urlId int) error
	Resolve(ctx context.Context, incidentId int) error
	Count(ctx context.Context, urlId int, numberOfDays int, dateType enums.DateType) (time.Time, int, error)
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

func (inc incidentRepository) Count(tx context.Context, urlId int, numberOfDays int, dateType enums.DateType) (time.Time, int, error) {
	var incidentCount int
	var bucket time.Time
	date := fmt.Sprintf("%v %v", numberOfDays, dateType.ToString())

	sql := "SELECT time_bucket($1, time) AS bucket, count(*) AS incident_count FROM incidents WHERE url_id=$2 GROUP BY bucket"
	err := inc.pool.QueryRow(tx, sql, date, urlId).Scan(&bucket, &incidentCount)
	if err != nil {
		return time.Time{}, 0, err
	}
	return bucket, incidentCount, nil
}

func NewIncidentRepository(pool *pgxpool.Pool) IncidentRepository {
	return incidentRepository{
		pool: pool,
	}
}
