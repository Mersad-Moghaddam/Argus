package storage

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// migrationsDir resolves db/migrations relative to this package.
func migrationsDir(t *testing.T) string {
	t.Helper()
	dir := filepath.Join("..", "..", "..", "db", "migrations")
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("migrations directory not found at %s: %v", dir, err)
	}
	return dir
}

func globOrFail(t *testing.T, dir, pattern string) []string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(dir, pattern))
	if err != nil {
		t.Fatalf("glob %s: %v", pattern, err)
	}
	sort.Strings(files)
	return files
}

func readFileOrFail(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(content)
}

// TestMigrationFilesArePairedAndOrdered runs without a database: it checks the
// structural invariants ApplyMigrations depends on, so a missing or misnamed
// file is caught in CI even where MySQL is unavailable.
func TestMigrationFilesArePairedAndOrdered(t *testing.T) {
	dir := migrationsDir(t)

	ups, err := filepath.Glob(filepath.Join(dir, "*.up.sql"))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(ups) == 0 {
		t.Fatal("expected at least one migration")
	}
	sort.Strings(ups)

	for _, up := range ups {
		down := strings.TrimSuffix(up, ".up.sql") + ".down.sql"
		if _, statErr := os.Stat(down); statErr != nil {
			t.Errorf("migration %s has no matching down migration", filepath.Base(up))
		}
		content, readErr := os.ReadFile(up)
		if readErr != nil {
			t.Fatalf("read %s: %v", up, readErr)
		}
		if strings.TrimSpace(string(content)) == "" {
			t.Errorf("migration %s is empty", filepath.Base(up))
		}
		// ApplyMigrations splits on ';', so a stored routine body (which needs
		// DELIMITER and contains embedded semicolons) would be torn apart.
		// Only a statement-leading DELIMITER counts; the word may appear in a
		// comment.
		for _, line := range strings.Split(string(content), "\n") {
			if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(line)), "DELIMITER ") {
				t.Errorf("migration %s uses DELIMITER, which the naive ';' splitter cannot handle", filepath.Base(up))
				break
			}
		}
	}

	// Lexical order must match numeric order, since that is the apply order.
	for i := 1; i < len(ups); i++ {
		if filepath.Base(ups[i-1]) >= filepath.Base(ups[i]) {
			t.Errorf("migrations are not lexically ordered: %s then %s", filepath.Base(ups[i-1]), filepath.Base(ups[i]))
		}
	}
}

// TestApplyMigrationsAgainstMySQL is the real up/down/up smoke test. It is
// opt-in: set MYSQL_TEST_DSN to a DSN pointing at a throwaway schema (the test
// drops every table in it). Without that variable the test skips, so the suite
// stays green on machines and CI runners with no database.
//
//	MYSQL_TEST_DSN='argus:argus@tcp(127.0.0.1:3306)/argus_test?parseTime=true&multiStatements=false' go test ./internal/platform/storage/...
func TestApplyMigrationsAgainstMySQL(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("MYSQL_TEST_DSN"))
	if dsn == "" {
		t.Skip("set MYSQL_TEST_DSN to run the migration smoke test against a throwaway schema")
	}
	dir := migrationsDir(t)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	// Registered first so it runs last: t.Cleanup is LIFO, and a deferred
	// Close() would otherwise fire before the drop-tables cleanup below,
	// leaving it to fail against a closed pool.
	t.Cleanup(func() { _ = db.Close() })
	if err = db.PingContext(ctx); err != nil {
		t.Fatalf("ping %s: %v", dsn, err)
	}

	dropAllTables(ctx, t, db)
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), time.Minute)
		defer cleanupCancel()
		dropAllTables(cleanupCtx, t, db)
	})

	// up
	if err = ApplyMigrations(ctx, db, dir); err != nil {
		t.Fatalf("first up migration failed: %v", err)
	}
	assertTablesExist(ctx, t, db, "websites", "website_checks", "incidents", "outbox_events",
		"users", "auth_tokens", "projects", "project_members", "api_routes", "route_checks",
		"route_incidents", "route_import_jobs")

	// up again — migrations must be idempotent, because ApplyMigrations runs on
	// every process start.
	if err = ApplyMigrations(ctx, db, dir); err != nil {
		t.Fatalf("re-running up migrations must be idempotent: %v", err)
	}

	// down
	if err = applyDownMigrations(ctx, db, dir); err != nil {
		t.Fatalf("down migrations failed: %v", err)
	}
	remaining := listTables(ctx, t, db)
	if len(remaining) != 0 {
		t.Fatalf("down migrations left tables behind: %v", remaining)
	}

	// up once more, proving the schema can be rebuilt from scratch after a
	// full rollback.
	if err = ApplyMigrations(ctx, db, dir); err != nil {
		t.Fatalf("second up migration failed: %v", err)
	}
	assertTablesExist(ctx, t, db, "projects", "api_routes", "route_checks", "route_incidents")
}

// applyDownMigrations runs *.down.sql in reverse lexical order.
func applyDownMigrations(ctx context.Context, db *sql.DB, dir string) error {
	files, err := filepath.Glob(filepath.Join(dir, "*.down.sql"))
	if err != nil {
		return err
	}
	sort.Sort(sort.Reverse(sort.StringSlice(files)))
	for _, file := range files {
		content, readErr := os.ReadFile(file)
		if readErr != nil {
			return readErr
		}
		if execErr := executeSQLBatch(ctx, db, string(content)); execErr != nil {
			return execErr
		}
	}
	return nil
}

func listTables(ctx context.Context, t *testing.T, db *sql.DB) []string {
	t.Helper()
	rows, err := db.QueryContext(ctx, `SELECT table_name FROM information_schema.tables WHERE table_schema = DATABASE()`)
	if err != nil {
		t.Fatalf("list tables: %v", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var name string
		if err = rows.Scan(&name); err != nil {
			t.Fatalf("scan table name: %v", err)
		}
		out = append(out, name)
	}
	if err = rows.Err(); err != nil {
		t.Fatalf("iterate tables: %v", err)
	}
	sort.Strings(out)
	return out
}

func assertTablesExist(ctx context.Context, t *testing.T, db *sql.DB, want ...string) {
	t.Helper()
	present := map[string]bool{}
	for _, name := range listTables(ctx, t, db) {
		present[strings.ToLower(name)] = true
	}
	for _, name := range want {
		if !present[name] {
			t.Errorf("expected table %q to exist after migrating", name)
		}
	}
}

func dropAllTables(ctx context.Context, t *testing.T, db *sql.DB) {
	t.Helper()
	tables := listTables(ctx, t, db)
	if len(tables) == 0 {
		return
	}
	if _, err := db.ExecContext(ctx, `SET FOREIGN_KEY_CHECKS = 0`); err != nil {
		t.Fatalf("disable foreign key checks: %v", err)
	}
	for _, name := range tables {
		if _, err := db.ExecContext(ctx, "DROP TABLE IF EXISTS `"+name+"`"); err != nil {
			t.Fatalf("drop table %s: %v", name, err)
		}
	}
	if _, err := db.ExecContext(ctx, `SET FOREIGN_KEY_CHECKS = 1`); err != nil {
		t.Fatalf("re-enable foreign key checks: %v", err)
	}
}
