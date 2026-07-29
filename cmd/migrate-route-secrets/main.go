// Command migrate-route-secrets performs the operator-controlled route-header
// migration. It is restartable: each successful row clears its plaintext
// column, so a later run resumes with the remaining rows.
package main

import (
	"context"
	"database/sql"
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
	dry := flag.Bool("dry-run", false, "report without writing")
	batch := flag.Int("batch-size", 200, "rows per batch (1-1000)")
	flag.Parse()
	if *dsn == "" || *key == "" || *batch < 1 || *batch > 1000 {
		log.Fatal("dsn, destination key, and batch size 1-1000 are required")
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
	var after int64
	for {
		where := map[bool]string{false: "headers IS NOT NULL AND headers<>''", true: "headers_encrypted IS NOT NULL AND headers_encrypted<>'' AND id>?"}[*rotate]
		args := []any{*batch}
		if *rotate {
			args = []any{after, *batch}
		}
		rows, err := db.QueryContext(ctx, `SELECT id, headers, headers_encrypted FROM api_routes WHERE `+where+` ORDER BY id LIMIT ?`, args...)
		if err != nil {
			log.Fatal(err)
		}
		count := 0
		for rows.Next() {
			var id int64
			var plain, cipher sql.NullString
			if err = rows.Scan(&id, &plain, &cipher); err != nil {
				log.Fatal(err)
			}
			after = id
			count++
			var next string
			if *rotate {
				next, err = secrets.Rewrap(from, to, cipher.String)
			} else {
				next, err = secrets.Seal(to, plain.String)
			}
			if err != nil {
				log.Fatalf("route %d: %v", id, err)
			}
			if !*dry {
				if *rotate {
					_, err = db.ExecContext(ctx, "UPDATE api_routes SET headers_encrypted=?, updated_at=NOW() WHERE id=?", next, id)
				} else {
					_, err = db.ExecContext(ctx, "UPDATE api_routes SET headers=NULL, headers_encrypted=?, updated_at=NOW() WHERE id=?", next, id)
				}
				if err != nil {
					log.Fatal(err)
				}
			}
		}
		rows.Close()
		fmt.Printf("processed=%d dry_run=%t rotate=%t\n", count, *dry, *rotate)
		if count < *batch {
			return
		}
	}
}
