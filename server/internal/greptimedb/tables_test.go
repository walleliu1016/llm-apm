package greptimedb

import (
	"strings"
	"testing"
)

func TestCreateTablesSQL(t *testing.T) {
	sqls := CreateTablesSQL()

	if len(sqls) == 0 {
		t.Error("expected non-empty SQL statements")
	}

	// Check for expected tables
	expectedTables := []string{
		"apm_hook_events",
		"apm_messages",
		"apm_turns",
	}

	for _, table := range expectedTables {
		found := false
		for _, sql := range sqls {
			if strings.Contains(sql, table) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected table %s not found in SQL", table)
		}
	}
}