// Command backfill-endpoint-identity performs the restartable canonical-route
// migration after the application has applied migration 0006. It intentionally
// does not run during web-process startup: legacy data conversion is an
// operator-visible action with a dry-run mode and conflict ledger.
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"argus/internal/domain"

	_ "github.com/go-sql-driver/mysql"
)

type legacyRoute struct {
	id, projectID         int64
	method, path, baseURL string
}

type results struct{ migrated, duplicates, invalid int }

func main() {
	dsn := flag.String("dsn", os.Getenv("DATABASE_DSN"), "MySQL DSN (or DATABASE_DSN)")
	batchSize := flag.Int("batch-size", 200, "routes processed per batch (1-1000)")
	dryRun := flag.Bool("dry-run", false, "report actions without writing data")
	flag.Parse()
	if *dsn == "" {
		log.Fatal("a MySQL DSN is required via -dsn or DATABASE_DSN")
	}
	if *batchSize < 1 || *batchSize > 1000 {
		log.Fatal("batch-size must be between 1 and 1000")
	}

	db, err := sql.Open("mysql", *dsn)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("ping database: %v", err)
	}

	var total results
	for {
		rows, err := loadBatch(ctx, db, *batchSize)
		if err != nil {
			log.Fatalf("load batch: %v", err)
		}
		if len(rows) == 0 {
			break
		}
		for _, route := range rows {
			outcome, err := processRoute(ctx, db, route, *dryRun)
			if err != nil {
				log.Fatalf("route %d: %v", route.id, err)
			}
			switch outcome {
			case "migrated":
				total.migrated++
			case "duplicate":
				total.duplicates++
			case "invalid":
				total.invalid++
			}
		}
		log.Printf("processed batch: migrated=%d duplicates=%d invalid=%d", total.migrated, total.duplicates, total.invalid)
	}
	fmt.Printf("canonical identity backfill complete: migrated=%d duplicates=%d invalid=%d dry_run=%t\n", total.migrated, total.duplicates, total.invalid, *dryRun)
}

func loadBatch(ctx context.Context, db *sql.DB, limit int) ([]legacyRoute, error) {
	rows, err := db.QueryContext(ctx, `SELECT r.id, r.project_id, r.method, r.path, r.base_url
		FROM api_routes r
		WHERE (r.canonical_identity IS NULL OR r.canonical_hash IS NULL)
		  AND NOT EXISTS (
				SELECT 1 FROM route_canonicalization_conflicts c
				WHERE c.route_id=r.id AND c.conflict_type='invalid_legacy_value'
			  )
		ORDER BY r.id ASC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []legacyRoute
	for rows.Next() {
		var r legacyRoute
		if err := rows.Scan(&r.id, &r.projectID, &r.method, &r.path, &r.baseURL); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func processRoute(ctx context.Context, db *sql.DB, route legacyRoute, dryRun bool) (string, error) {
	normalized, err := domain.NormalizeEndpoint(route.method, route.baseURL, route.path)
	if err != nil {
		if !dryRun {
			_, writeErr := db.ExecContext(ctx, `INSERT IGNORE INTO route_canonicalization_conflicts
				(project_id, route_id, conflict_type, details) VALUES (?, ?, 'invalid_legacy_value', ?)`, route.projectID, route.id, domain.ValidationCode(err))
			if writeErr != nil {
				return "", writeErr
			}
		}
		return "invalid", nil
	}
	hash := domain.CanonicalHash(normalized.CanonicalIdentity)
	var conflictingID int64
	err = db.QueryRowContext(ctx, `SELECT id FROM api_routes WHERE project_id=? AND canonical_hash=? AND canonical_identity=? AND id<>? LIMIT 1`, route.projectID, hash, normalized.CanonicalIdentity, route.id).Scan(&conflictingID)
	if err != nil && err != sql.ErrNoRows {
		return "", err
	}
	if dryRun {
		if err == nil {
			return "duplicate", nil
		}
		return "migrated", nil
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `UPDATE api_routes SET canonical_identity=?, canonical_hash=?, canonical_version=1 WHERE id=?`, normalized.CanonicalIdentity, hash, route.id); err != nil {
		return "", err
	}
	if err == nil {
		return "migrated", tx.Commit()
	}
	_, err = tx.ExecContext(ctx, `INSERT IGNORE INTO route_canonicalization_conflicts
		(project_id, route_id, conflicting_route_id, conflict_type, details) VALUES (?, ?, ?, 'exact_duplicate', 'same canonical identity after legacy normalization')`, route.projectID, route.id, conflictingID)
	if err != nil {
		return "", err
	}
	return "duplicate", tx.Commit()
}
