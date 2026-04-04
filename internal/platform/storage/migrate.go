package storage

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ApplyMigrations executes all *.up.sql files in lexical order.
func ApplyMigrations(ctx context.Context, db *sql.DB, migrationsDir string) error {
	files, err := filepath.Glob(filepath.Join(migrationsDir, "*.up.sql"))
	if err != nil {
		return fmt.Errorf("glob migrations: %w", err)
	}
	sort.Strings(files)

	for _, file := range files {
		content, readErr := os.ReadFile(file)
		if readErr != nil {
			return fmt.Errorf("read migration %s: %w", file, readErr)
		}
		if execErr := executeSQLBatch(ctx, db, string(content)); execErr != nil {
			return fmt.Errorf("execute migration %s: %w", file, execErr)
		}
	}
	return nil
}

func executeSQLBatch(ctx context.Context, db *sql.DB, sqlText string) error {
	statements := strings.Split(sqlText, ";")
	for _, stmt := range statements {
		q := strings.TrimSpace(stmt)
		if q == "" || strings.HasPrefix(q, "--") {
			continue
		}
		if _, err := db.ExecContext(ctx, q); err != nil {
			return err
		}
	}
	return nil
}
