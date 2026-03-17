package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"argus/internal/models"
)

// WebsiteRepository declares persistence operations for websites.
type WebsiteRepository interface {
	Create(ctx context.Context, website models.Website) (int64, error)
	List(ctx context.Context) ([]models.Website, error)
	Delete(ctx context.Context, id int64) error
	ListDue(ctx context.Context, now time.Time) ([]models.Website, error)
	MarkChecked(ctx context.Context, id int64, status string, statusCode int, checkedAt, nextCheckAt time.Time) error
}

// MySQLWebsiteRepository is a MySQL-backed website repository.
type MySQLWebsiteRepository struct {
	db *sql.DB
}

// NewMySQLWebsiteRepository creates a MySQLWebsiteRepository.
func NewMySQLWebsiteRepository(db *sql.DB) *MySQLWebsiteRepository {
	return &MySQLWebsiteRepository{db: db}
}

// Create inserts a website row.
func (r *MySQLWebsiteRepository) Create(ctx context.Context, website models.Website) (int64, error) {
	const query = `
INSERT INTO websites (url, check_interval_seconds, status, next_check_at, last_status_code)
VALUES (?, ?, ?, ?, ?)`
	result, err := r.db.ExecContext(ctx, query, website.URL, website.CheckInterval, website.Status, website.NextCheckAt, website.LastStatusCode)
	if err != nil {
		return 0, fmt.Errorf("insert website: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("get website last insert id: %w", err)
	}
	return id, nil
}

// List returns all websites sorted by id desc.
func (r *MySQLWebsiteRepository) List(ctx context.Context) ([]models.Website, error) {
	const query = `
SELECT id, url, check_interval_seconds, status, last_checked_at, next_check_at, last_status_code, created_at, updated_at
FROM websites
ORDER BY id DESC`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query websites: %w", err)
	}
	defer rows.Close()

	items := make([]models.Website, 0)
	for rows.Next() {
		var w models.Website
		err = rows.Scan(
			&w.ID,
			&w.URL,
			&w.CheckInterval,
			&w.Status,
			&w.LastCheckedAt,
			&w.NextCheckAt,
			&w.LastStatusCode,
			&w.CreatedAt,
			&w.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan website row: %w", err)
		}
		items = append(items, w)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate website rows: %w", err)
	}
	return items, nil
}

// Delete removes a website by id.
func (r *MySQLWebsiteRepository) Delete(ctx context.Context, id int64) error {
	const query = `DELETE FROM websites WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete website: %w", err)
	}
	return nil
}

// ListDue returns websites that should be checked now.
func (r *MySQLWebsiteRepository) ListDue(ctx context.Context, now time.Time) ([]models.Website, error) {
	const query = `
SELECT id, url, check_interval_seconds, status, last_checked_at, next_check_at, last_status_code, created_at, updated_at
FROM websites
WHERE next_check_at <= ?
ORDER BY next_check_at ASC`

	rows, err := r.db.QueryContext(ctx, query, now)
	if err != nil {
		return nil, fmt.Errorf("query due websites: %w", err)
	}
	defer rows.Close()

	items := make([]models.Website, 0)
	for rows.Next() {
		var w models.Website
		err = rows.Scan(
			&w.ID,
			&w.URL,
			&w.CheckInterval,
			&w.Status,
			&w.LastCheckedAt,
			&w.NextCheckAt,
			&w.LastStatusCode,
			&w.CreatedAt,
			&w.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan due website row: %w", err)
		}
		items = append(items, w)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate due website rows: %w", err)
	}
	return items, nil
}

// MarkChecked updates status, status code and next check time after execution.
func (r *MySQLWebsiteRepository) MarkChecked(ctx context.Context, id int64, status string, statusCode int, checkedAt, nextCheckAt time.Time) error {
	const query = `
UPDATE websites
SET status = ?, last_status_code = ?, last_checked_at = ?, next_check_at = ?, updated_at = NOW()
WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, status, statusCode, checkedAt, nextCheckAt, id)
	if err != nil {
		return fmt.Errorf("update checked website: %w", err)
	}
	return nil
}
