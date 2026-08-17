package database

import (
	"context"
	"fmt"
	"github.com/DevloperAmanSingh/watcher/enums"
	"github.com/jackc/pgx/v5/pgxpool"
	"strings"
)

type UrlQueryFilter struct {
	HttpMethod enums.HttpMethod
	Status     enums.SiteHealth
	Frequency  enums.MonitoringFrequency
}

func NewUrlQueryFilter() UrlQueryFilter {
	return UrlQueryFilter{}
}

// DefaultPageSize is the batch size Each uses when walking the table.
const DefaultPageSize = 500

type UrlRepository interface {
	FetchAll(ctx context.Context, limit int, offset int, filter UrlQueryFilter) ([]Url, error)
	// Each invokes fn once per URL matching filter, paging through the table
	// so callers that need every row never depend on a caller-supplied limit.
	Each(ctx context.Context, filter UrlQueryFilter, fn func(Url) error) error
	Add(ctx context.Context, url string, httpMethod enums.HttpMethod, frequency enums.MonitoringFrequency, contactEmail string) (int, error)
	Delete(ctx context.Context, Id int) error
	FindById(ctx context.Context, Id int) (Url, error)
	UpdateStatus(ctx context.Context, Id int, status enums.SiteHealth) error
	// RecordFailure increments the consecutive failure counter and returns its
	// new value. The increment and the read are one statement, so concurrent
	// callers each observe a distinct count and exactly one sees any given
	// threshold.
	RecordFailure(ctx context.Context, Id int) (int, error)
	// ResetFailures clears the counter and reports whether it had been
	// non-zero, so a recovery can be distinguished from a steady healthy run.
	ResetFailures(ctx context.Context, Id int) (bool, error)
}
type urlRepository struct {
	pool *pgxpool.Pool
}

func (ur urlRepository) FetchAll(ctx context.Context, limit int, offset int, filter UrlQueryFilter) ([]Url, error) {
	sql := "SELECT id,url,http_method,contact_email,status,monitoring_frequency,created_at,updated_at FROM urls"

	var whereClauses []string
	var args []interface{}
	argPosition := 1

	if filter.HttpMethod != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("http_method = $%d", argPosition))
		args = append(args, filter.HttpMethod.ToString())
		argPosition++
	}

	if filter.Status != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("status = $%d", argPosition))
		args = append(args, filter.Status.ToString())
		argPosition++
	}

	if filter.Frequency != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("monitoring_frequency = $%d", argPosition))
		args = append(args, filter.Frequency.ToString())
		argPosition++
	}

	if len(whereClauses) > 0 {
		sql += " WHERE " + strings.Join(whereClauses, " AND ")
	}

	// LIMIT/OFFSET without a total order returns arbitrary rows, so paging
	// could repeat or skip URLs between calls.
	sql += " ORDER BY id"
	sql += fmt.Sprintf(" LIMIT $%d OFFSET $%d", argPosition, argPosition+1)
	args = append(args, limit, offset)

	rows, err := ur.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var urls []Url
	var monitoringFrequency string
	var status string
	var httpMethod string

	for rows.Next() {
		var url Url
		err := rows.Scan(
			&url.Id,
			&url.Url,
			&httpMethod,
			&url.ContactEmail,
			&status,
			&monitoringFrequency,
			&url.CreatedAt,
			&url.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		parsedMonitoringFrequency, err := enums.ParseMonitoringFrequency(monitoringFrequency)
		if err != nil {
			return nil, err
		}

		parsedStatus, err := enums.ParseSiteHealth(status)
		if err != nil {
			return nil, err
		}

		parsedHttpMethod, err := enums.ParseHttpMethod(httpMethod)
		if err != nil {
			return nil, err
		}

		url.MonitoringFrequency = parsedMonitoringFrequency
		url.Status = parsedStatus
		url.HttpMethod = parsedHttpMethod
		urls = append(urls, url)
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("error iterating task rows: %w", err)
		}
	}
	return urls, nil
}

func (ur urlRepository) Each(ctx context.Context, filter UrlQueryFilter, fn func(Url) error) error {
	for offset := 0; ; offset += DefaultPageSize {
		page, err := ur.FetchAll(ctx, DefaultPageSize, offset, filter)
		if err != nil {
			return err
		}

		for _, url := range page {
			if err := fn(url); err != nil {
				return err
			}
		}

		if len(page) < DefaultPageSize {
			return nil
		}
	}
}

func (ur urlRepository) Add(ctx context.Context, url string, httpMethod enums.HttpMethod, frequency enums.MonitoringFrequency, contactEmail string) (int, error) {
	sql := "INSERT INTO urls (url,http_method,contact_email,status,monitoring_frequency) VALUES ($1,$2,$3,$4,$5) RETURNING id"

	var id int
	err := ur.pool.QueryRow(ctx, sql, url, httpMethod, contactEmail, enums.Pending, frequency).Scan(&id)
	if err != nil {
		return 0, err
	}
	return id, nil
}

func (ur urlRepository) FindById(ctx context.Context, id int) (Url, error) {
	sql := "SELECT id,url,http_method,contact_email,status,monitoring_frequency,created_at,updated_at FROM urls WHERE ID=$1"
	var url Url
	var monitoringFrequency string
	var status string
	var httpMethod string
	err := ur.pool.QueryRow(ctx, sql, id).Scan(
		&url.Id,
		&url.Url,
		&httpMethod,
		&url.ContactEmail,
		&status,
		&monitoringFrequency,
		&url.CreatedAt,
		&url.UpdatedAt,
	)

	if err != nil {
		return url, err
	}
	parsedMonitoringFrequency, err := enums.ParseMonitoringFrequency(monitoringFrequency)
	if err != nil {
		return Url{}, err
	}

	parsedStatus, err := enums.ParseSiteHealth(status)
	if err != nil {
		return Url{}, err
	}

	parsedHttpMethod, err := enums.ParseHttpMethod(httpMethod)
	if err != nil {
		return Url{}, err
	}

	url.MonitoringFrequency = parsedMonitoringFrequency
	url.Status = parsedStatus
	url.HttpMethod = parsedHttpMethod

	return url, nil
}

func (ur urlRepository) Delete(ctx context.Context, Id int) error {
	sql := "DELETE FROM urls WHERE id=$1"
	_, err := ur.pool.Exec(ctx, sql, Id)
	if err != nil {
		return err
	}
	return nil
}

func (ur urlRepository) UpdateStatus(ctx context.Context, Id int, status enums.SiteHealth) error {
	sql := "UPDATE urls SET status=$1 WHERE id=$2"
	_, err := ur.pool.Exec(ctx, sql, status, Id)
	if err != nil {
		return err
	}
	return nil
}

func (ur urlRepository) RecordFailure(ctx context.Context, id int) (int, error) {
	sql := `UPDATE urls SET consecutive_failures = consecutive_failures + 1
	        WHERE id = $1 RETURNING consecutive_failures`

	var failures int
	if err := ur.pool.QueryRow(ctx, sql, id).Scan(&failures); err != nil {
		return 0, err
	}
	return failures, nil
}

func (ur urlRepository) ResetFailures(ctx context.Context, id int) (bool, error) {
	sql := `UPDATE urls SET consecutive_failures = 0
	        WHERE id = $1 AND consecutive_failures > 0`

	tag, err := ur.pool.Exec(ctx, sql, id)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

func NewUrlRepository(pool *pgxpool.Pool) UrlRepository {
	return urlRepository{
		pool: pool,
	}
}
