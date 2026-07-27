package mysql

import (
	"strings"
	"testing"
)

func TestProjectListColumnsAreQualifiedForMemberJoin(t *testing.T) {
	t.Parallel()
	for _, column := range strings.Split(projectColumnsAliased, ",") {
		column = strings.TrimSpace(column)
		if !strings.HasPrefix(column, "p.") {
			t.Fatalf("joined project column must be qualified with p.: %q", column)
		}
	}
}
