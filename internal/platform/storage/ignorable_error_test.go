package storage

import (
	"errors"
	"fmt"
	"testing"

	mysqlDriver "github.com/go-sql-driver/mysql"
)

func TestIsIgnorableMigrationError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "duplicate column",
			err:  &mysqlDriver.MySQLError{Number: 1060, Message: "Duplicate column name"},
			want: true,
		},
		{
			name: "wrapped duplicate column",
			err:  fmt.Errorf("apply column: %w", &mysqlDriver.MySQLError{Number: 1060, Message: "Duplicate column name"}),
			want: true,
		},
		{
			name: "other mysql error",
			err:  &mysqlDriver.MySQLError{Number: 1064, Message: "Syntax error"},
			want: false,
		},
		{
			name: "non mysql error",
			err:  errors.New("connection failed"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := isIgnorableMigrationError(tt.err); got != tt.want {
				t.Fatalf("isIgnorableMigrationError() = %v, want %v", got, tt.want)
			}
		})
	}
}
