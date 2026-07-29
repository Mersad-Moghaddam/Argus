// Command migrate-route-secrets performs the operator-controlled route-header
// migration. Plaintext migration resumes because successful rows are cleared.
// Ciphertext rotation uses a durable, key-fingerprinted checkpoint that is
// committed atomically with each rewrapped batch.
package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"argus/internal/secrets"
	_ "github.com/go-sql-driver/mysql"
)

func main() {
	dsn := flag.String("dsn", os.Getenv("DATABASE_DSN"), "MySQL DSN")
	key := flag.String("key", os.Getenv("ROUTE_SECRET_ENCRYPTION_KEY"), "base64url destination key")
	oldKey := flag.String("old-key", "", "base64url source key; required for --rotate")
	rotate := flag.Bool("rotate", false, "rewrap existing ciphertext")
	rotationID := flag.String("rotation-id", "", "durable identifier required for --rotate (1-100 characters)")
	dry := flag.Bool("dry-run", false, "report without writing")
	batch := flag.Int("batch-size", 200, "rows per batch (1-1000)")
	flag.Parse()
	if *dsn == "" || *key == "" || *batch < 1 || *batch > 1000 {
		log.Fatal("dsn, destination key, and batch size 1-1000 are required")
	}
	if *rotate && (*rotationID == "" || len(*rotationID) > 100) {
		log.Fatal("--rotation-id must be 1-100 characters when --rotate is set")
	}
	if !*rotate && *rotationID != "" {
		log.Fatal("--rotation-id may only be used with --rotate")
	}
	to, err := secrets.ParseKey(*key)
	if err != nil {
		log.Fatal(err)
	}
	var from []byte
	if *rotate {
		if *oldKey == "" {
			log.Fatal("--old-key is required for --rotate")
		}
		from, err = secrets.ParseKey(*oldKey)
		if err != nil {
			log.Fatal(err)
		}
	}
	db, err := sql.Open("mysql", *dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	if *rotate {
		if err := rotateCiphertext(ctx, db, *rotationID, from, to, *batch, *dry); err != nil {
			log.Fatal(err)
		}
		return
	}
	if err := migratePlaintext(ctx, db, to, *batch, *dry); err != nil {
		log.Fatal(err)
	}
}

func migratePlaintext(ctx context.Context, db *sql.DB, key []byte, batch int, dry bool) error {
	for {
		rows, err := db.QueryContext(ctx, `SELECT id, headers FROM api_routes WHERE headers IS NOT NULL AND headers<>'' ORDER BY id LIMIT ?`, batch)
		if err != nil {
			return err
		}
		count := 0
		for rows.Next() {
			var id int64
			var plain string
			if err = rows.Scan(&id, &plain); err != nil {
				rows.Close()
				return err
			}
			next, err := secrets.Seal(key, plain)
			if err != nil {
				rows.Close()
				return fmt.Errorf("route %d: %w", id, err)
			}
			if !dry {
				if _, err = db.ExecContext(ctx, "UPDATE api_routes SET headers=NULL, headers_encrypted=?, updated_at=NOW() WHERE id=?", next, id); err != nil {
					rows.Close()
					return err
				}
			}
			count++
		}
		if err = rows.Close(); err != nil {
			return err
		}
		fmt.Printf("processed=%d dry_run=%t rotate=false\n", count, dry)
		if count < batch {
			return nil
		}
	}
}

type encryptedRoute struct {
	id     int64
	cipher string
}

func rotateCiphertext(ctx context.Context, db *sql.DB, rotationID string, from, to []byte, batch int, dry bool) error {
	sourceFingerprint := sha256.Sum256(from)
	destinationFingerprint := sha256.Sum256(to)
	if dry {
		return dryRunRotation(ctx, db, rotationID, from, to, sourceFingerprint, destinationFingerprint, batch)
	}
	if _, err := db.ExecContext(ctx, `INSERT IGNORE INTO route_header_secret_rotations
		(rotation_id, source_key_fingerprint, destination_key_fingerprint, last_route_id)
		VALUES (?, ?, ?, 0)`, rotationID, sourceFingerprint[:], destinationFingerprint[:]); err != nil {
		return err
	}
	for {
		processed, lastID, complete, err := rotateBatch(ctx, db, rotationID, from, to, sourceFingerprint, destinationFingerprint, batch)
		if err != nil {
			return err
		}
		fmt.Printf("rotation_id=%s processed=%d last_route_id=%d complete=%t dry_run=false\n", rotationID, processed, lastID, complete)
		if complete {
			return nil
		}
	}
}

func dryRunRotation(ctx context.Context, db *sql.DB, rotationID string, from, to []byte, sourceFingerprint, destinationFingerprint [sha256.Size]byte, batch int) error {
	lastID, complete, err := rotationCheckpoint(ctx, db, rotationID, sourceFingerprint, destinationFingerprint)
	if err != nil {
		return err
	}
	if complete {
		fmt.Printf("rotation_id=%s processed=0 last_route_id=%d complete=true dry_run=true\n", rotationID, lastID)
		return nil
	}
	for {
		routes, err := loadEncryptedRoutes(ctx, db, lastID, batch, false)
		if err != nil {
			return err
		}
		for _, route := range routes {
			if _, err := secrets.Rewrap(from, to, route.cipher); err != nil {
				return fmt.Errorf("route %d: %w", route.id, err)
			}
			lastID = route.id
		}
		complete = len(routes) < batch
		fmt.Printf("rotation_id=%s processed=%d last_route_id=%d complete=%t dry_run=true\n", rotationID, len(routes), lastID, complete)
		if complete {
			return nil
		}
	}
}

func rotateBatch(ctx context.Context, db *sql.DB, rotationID string, from, to []byte, sourceFingerprint, destinationFingerprint [sha256.Size]byte, batch int) (int, int64, bool, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, 0, false, err
	}
	defer tx.Rollback()
	lastID, complete, err := rotationCheckpointTx(ctx, tx, rotationID, sourceFingerprint, destinationFingerprint)
	if err != nil || complete {
		if err != nil {
			return 0, 0, false, err
		}
		return 0, lastID, true, tx.Commit()
	}
	routes, err := loadEncryptedRoutes(ctx, tx, lastID, batch, true)
	if err != nil {
		return 0, 0, false, err
	}
	if len(routes) == 0 {
		if _, err := tx.ExecContext(ctx, `UPDATE route_header_secret_rotations SET completed_at=NOW() WHERE rotation_id=?`, rotationID); err != nil {
			return 0, 0, false, err
		}
		return 0, lastID, true, tx.Commit()
	}
	for _, route := range routes {
		next, err := secrets.Rewrap(from, to, route.cipher)
		if err != nil {
			return 0, 0, false, fmt.Errorf("route %d: %w", route.id, err)
		}
		if _, err = tx.ExecContext(ctx, "UPDATE api_routes SET headers_encrypted=?, updated_at=NOW() WHERE id=?", next, route.id); err != nil {
			return 0, 0, false, err
		}
		lastID = route.id
	}
	complete = len(routes) < batch
	if complete {
		_, err = tx.ExecContext(ctx, `UPDATE route_header_secret_rotations SET last_route_id=?, completed_at=NOW() WHERE rotation_id=?`, lastID, rotationID)
	} else {
		_, err = tx.ExecContext(ctx, `UPDATE route_header_secret_rotations SET last_route_id=? WHERE rotation_id=?`, lastID, rotationID)
	}
	if err != nil {
		return 0, 0, false, err
	}
	return len(routes), lastID, complete, tx.Commit()
}

type queryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func loadEncryptedRoutes(ctx context.Context, q queryer, afterID int64, batch int, forUpdate bool) ([]encryptedRoute, error) {
	query := `SELECT id, headers_encrypted FROM api_routes WHERE headers_encrypted IS NOT NULL AND headers_encrypted<>'' AND id>? ORDER BY id LIMIT ?`
	if forUpdate {
		query += " FOR UPDATE"
	}
	rows, err := q.QueryContext(ctx, query, afterID, batch)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var routes []encryptedRoute
	for rows.Next() {
		var route encryptedRoute
		if err := rows.Scan(&route.id, &route.cipher); err != nil {
			return nil, err
		}
		routes = append(routes, route)
	}
	return routes, rows.Err()
}

func rotationCheckpoint(ctx context.Context, db *sql.DB, rotationID string, sourceFingerprint, destinationFingerprint [sha256.Size]byte) (int64, bool, error) {
	var source, destination []byte
	var lastID int64
	var completedAt sql.NullTime
	err := db.QueryRowContext(ctx, `SELECT source_key_fingerprint, destination_key_fingerprint, last_route_id, completed_at
		FROM route_header_secret_rotations WHERE rotation_id=?`, rotationID).Scan(&source, &destination, &lastID, &completedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	if string(source) != string(sourceFingerprint[:]) || string(destination) != string(destinationFingerprint[:]) {
		return 0, false, errors.New("rotation id is bound to different source or destination key material")
	}
	return lastID, completedAt.Valid, nil
}

func rotationCheckpointTx(ctx context.Context, tx *sql.Tx, rotationID string, sourceFingerprint, destinationFingerprint [sha256.Size]byte) (int64, bool, error) {
	var source, destination []byte
	var lastID int64
	var completedAt sql.NullTime
	err := tx.QueryRowContext(ctx, `SELECT source_key_fingerprint, destination_key_fingerprint, last_route_id, completed_at
		FROM route_header_secret_rotations WHERE rotation_id=? FOR UPDATE`, rotationID).Scan(&source, &destination, &lastID, &completedAt)
	if err != nil {
		return 0, false, err
	}
	if string(source) != string(sourceFingerprint[:]) || string(destination) != string(destinationFingerprint[:]) {
		return 0, false, errors.New("rotation id is bound to different source or destination key material")
	}
	return lastID, completedAt.Valid, nil
}
