package storage

import (
	"reflect"
	"strings"
	"testing"
)

func TestSplitStatements(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []string
	}{
		{
			name: "simple batch",
			in:   "SELECT 1; SELECT 2;",
			want: []string{"SELECT 1", "SELECT 2"},
		},
		{
			name: "trailing statement without a semicolon",
			in:   "SELECT 1;\nSELECT 2",
			want: []string{"SELECT 1", "SELECT 2"},
		},
		{
			name: "empty statements are dropped",
			in:   ";;\nSELECT 1;;\n\n;",
			want: []string{"SELECT 1"},
		},
		{
			name: "line comments are stripped entirely",
			in:   "-- a leading note\nSELECT 1;\n-- another note\nSELECT 2;",
			want: []string{"SELECT 1", "SELECT 2"},
		},
		{
			name: "hash comments are stripped",
			in:   "# note\nSELECT 1;",
			want: []string{"SELECT 1"},
		},
		{
			name: "a comment containing a semicolon does not split the statement",
			in:   "-- run these as-is; then continue\nSELECT 1;",
			want: []string{"SELECT 1"},
		},
		{
			name: "a comment-only file yields no statements",
			in:   "-- nothing to do here\n-- really nothing\n",
			want: nil,
		},
		{
			name: "block comments are stripped",
			in:   "/* a note; with a semicolon */ SELECT 1;",
			want: []string{"SELECT 1"},
		},
		{
			name: "a semicolon inside a single-quoted literal does not split",
			in:   "INSERT INTO t (v) VALUES ('a;b');",
			want: []string{"INSERT INTO t (v) VALUES ('a;b')"},
		},
		{
			name: "doubled quotes inside a literal are handled",
			in:   "SET @d = 'DEFAULT ''http_status''; ok'; SELECT 1;",
			want: []string{"SET @d = 'DEFAULT ''http_status''; ok'", "SELECT 1"},
		},
		{
			name: "a semicolon inside a backquoted identifier does not split",
			in:   "SELECT `we;ird` FROM t;",
			want: []string{"SELECT `we;ird` FROM t"},
		},
		{
			name: "a double-quoted literal is respected",
			in:   `SELECT "a;b" AS v;`,
			want: []string{`SELECT "a;b" AS v`},
		},
		{
			name: "a comment marker inside a literal is not a comment",
			in:   "INSERT INTO t (v) VALUES ('a -- b'); SELECT 1;",
			want: []string{"INSERT INTO t (v) VALUES ('a -- b')", "SELECT 1"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := SplitStatements(tc.in)
			if len(got) != len(tc.want) {
				t.Fatalf("got %d statements %q, want %d %q", len(got), got, len(tc.want), tc.want)
			}
			if tc.want == nil {
				return
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// TestSplitStatementsOnRealMigrations guards the actual files: every statement
// must be non-empty and free of a stray leading comment fragment.
func TestSplitStatementsOnRealMigrations(t *testing.T) {
	dir := migrationsDir(t)
	for _, pattern := range []string{"*.up.sql", "*.down.sql"} {
		files := globOrFail(t, dir, pattern)
		for _, file := range files {
			content := readFileOrFail(t, file)
			for i, stmt := range SplitStatements(content) {
				if strings.TrimSpace(stmt) == "" {
					t.Errorf("%s: statement %d is empty", file, i)
				}
				if strings.HasPrefix(strings.TrimSpace(stmt), "--") {
					t.Errorf("%s: statement %d starts with a comment marker: %q", file, i, stmt)
				}
			}
		}
	}
}
