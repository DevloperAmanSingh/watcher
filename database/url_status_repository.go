package database

import (
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
)

type UrlStatusRepository interface {
	Add(ctx context.Context, urlId int, status bool) error
	GetRecentStatus(ctx context.Context, urlId int, status bool) (UrlStatus, error)
	GetLastStatus(ctx context.Context, urlId int) (UrlStatus, error)
}
type urlStatusRepository struct {
	pool *pgxpool.Pool
}

func (ur urlStatusRepository) Add(ctx context.Context, urlId int, status bool) error {
	sql := "INSERT INTO url_statuses (time, url_id,status) VALUES (NOW(), $1,$2)"

	_, err := ur.pool.Exec(ctx, sql, urlId, status)
	if err != nil {
		return err
	}
	return nil
}

func (ur urlStatusRepository) GetRecentStatus(ctx context.Context, urlId int, status bool) (UrlStatus, error) {
	var urlStatus UrlStatus
	sql := "SELECT * FROM url_statuses WHERE url_id=$1 AND STATUS=$2 ORDER BY time DESC LIMIT 1"
	err := ur.pool.QueryRow(ctx, sql, urlId, status).Scan(&urlStatus.Time, &urlStatus.UrlId, &urlStatus.Status)
	return urlStatus, err
}

func (ur urlStatusRepository) GetLastStatus(ctx context.Context, urlId int) (UrlStatus, error) {
	var urlStatus UrlStatus
	sql := "SELECT * FROM url_statuses WHERE url_id=$1 ORDER BY time DESC LIMIT 1"
	err := ur.pool.QueryRow(ctx, sql, urlId).Scan(&urlStatus.Time, &urlStatus.UrlId, &urlStatus.Status)
	return urlStatus, err
}

func NewUrlStatusRepository(pool *pgxpool.Pool) UrlStatusRepository {
	return urlStatusRepository{
		pool: pool,
	}
}
