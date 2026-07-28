package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	mysqlDriver "github.com/go-sql-driver/mysql"
)

// sqlExecutor is satisfied by *sql.DB and *sql.Conn, so a batch can be run
// either on the pool or on one pinned connection.
type sqlExecutor interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// ApplyMigrations executes all *.up.sql files in lexical order.
//
// Every file runs on a single pinned connection, so a migration that depends on
// session state (a user variable, a prepared statement, a session setting)
// behaves the same as it would in a mysql client. Handing statements to the
// pool would scatter them across connections and any session state would be
// silently lost.
func ApplyMigrations(ctx context.Context, db *sql.DB, migrationsDir string) error {
	files, err := filepath.Glob(filepath.Join(migrationsDir, "*.up.sql"))
	if err != nil {
		return fmt.Errorf("glob migrations: %w", err)
	}
	sort.Strings(files)

	conn, err := db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("acquire migration connection: %w", err)
	}
	defer conn.Close()

	for _, file := range files {
		content, readErr := os.ReadFile(file)
		if readErr != nil {
			return fmt.Errorf("read migration %s: %w", file, readErr)
		}
		if execErr := executeSQLBatch(ctx, conn, string(content)); execErr != nil {
			return fmt.Errorf("execute migration %s: %w", file, execErr)
		}
	}
	return nil
}

// executeSQLBatch runs the statements in sqlText in order.
func executeSQLBatch(ctx context.Context, exec sqlExecutor, sqlText string) error {
	for _, stmt := range SplitStatements(sqlText) {
		if _, err := exec.ExecContext(ctx, stmt); err != nil {
			if isIgnorableMigrationError(err) {
				continue
			}
			return err
		}
	}
	return nil
}

// isIgnorableMigrationError reports whether err is the specific "column already
// exists" failure that makes a re-run of an additive migration a no-op.
//
// MySQL has no `ALTER TABLE ... ADD COLUMN IF NOT EXISTS` (that syntax is a
// MariaDB extension), so the compatibility migration issues plain ADD COLUMN
// statements and relies on this narrow allowance for idempotency. Nothing else
// is tolerated: every other error still aborts startup.
func isIgnorableMigrationError(err error) bool {
	var mysqlErr *mysqlDriver.MySQLError
	const errDupFieldName = 1060
	return errors.As(err, &mysqlErr) && mysqlErr.Number == errDupFieldName
}

// SplitStatements splits a SQL batch into individual statements.
//
// It is quote- and comment-aware: only semicolons outside string literals,
// quoted identifiers and comments terminate a statement, and `--`/`#` line
// comments plus `/* */` block comments are stripped. A naive strings.Split on
// ";" silently mangles any migration whose comment prose or string literal
// happens to contain a semicolon, which is easy to introduce and produces a
// baffling syntax error at startup.
func SplitStatements(sqlText string) []string {
	var (
		statements []string
		current    strings.Builder
		inSingle   bool
		inDouble   bool
		inBacktick bool
	)

	flush := func() {
		if stmt := strings.TrimSpace(current.String()); stmt != "" {
			statements = append(statements, stmt)
		}
		current.Reset()
	}

	for i := 0; i < len(sqlText); i++ {
		c := sqlText[i]
		inQuotes := inSingle || inDouble || inBacktick

		if !inQuotes {
			// Line comments run to the end of the line.
			if c == '#' || (c == '-' && i+1 < len(sqlText) && sqlText[i+1] == '-') {
				for i < len(sqlText) && sqlText[i] != '\n' {
					i++
				}
				current.WriteByte('\n')
				continue
			}
			// Block comments run to the closing marker.
			if c == '/' && i+1 < len(sqlText) && sqlText[i+1] == '*' {
				i += 2
				for i+1 < len(sqlText) && !(sqlText[i] == '*' && sqlText[i+1] == '/') {
					i++
				}
				i++ // land on '/'
				current.WriteByte(' ')
				continue
			}
			if c == ';' {
				flush()
				continue
			}
		}

		switch c {
		case '\'':
			if !inDouble && !inBacktick {
				inSingle = !inSingle
			}
		case '"':
			if !inSingle && !inBacktick {
				inDouble = !inDouble
			}
		case '`':
			if !inSingle && !inDouble {
				inBacktick = !inBacktick
			}
		}
		current.WriteByte(c)
	}
	flush()
	return statements
}
