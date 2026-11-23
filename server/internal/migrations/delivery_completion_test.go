package migrations

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDeliveryCompletionMigrationContract(t *testing.T) {
	up := strings.ToUpper(readMigrationFile(t, "781_delivery_completion_authorization.up.sql"))
	for _, forbidden := range []string{"FOREIGN KEY", "REFERENCES ", "PRIMARY KEY", " UNIQUE", "CASCADE"} {
		if strings.Contains(up, forbidden) {
			t.Fatalf("authorization tables introduced %s", forbidden)
		}
	}
	for _, stem := range []string{"782_delivery_completion_id", "783_delivery_completion_request", "784_delivery_completion_operation", "785_delivery_completion_terminal_id", "786_delivery_completion_candidate"} {
		up := readMigrationFile(t, stem+".up.sql")
		down := readMigrationFile(t, stem+".down.sql")
		if strings.Count(up, ";") != 1 || !strings.Contains(up, "INDEX CONCURRENTLY") {
			t.Fatalf("%s is not a single concurrent index migration", stem)
		}
		if strings.Count(down, ";") != 1 || !strings.Contains(down, "DROP INDEX CONCURRENTLY") {
			t.Fatalf("%s is not a concurrent index drop", stem)
		}
	}
	for _, stem := range []string{"781_delivery_completion_authorization", "787_delivery_completion_policy_mode", "788_delivery_completion_ready"} {
		body := readMigrationFile(t, stem+".down.sql")
		for _, table := range []string{"delivery_completion_authorization", "delivery_completion_terminal", "project_delivery_policy"} {
			if !strings.Contains(body, "LOCK TABLE "+table+" IN ACCESS EXCLUSIVE MODE NOWAIT") {
				t.Fatalf("%s omits %s fence", stem, table)
			}
		}
		if !strings.Contains(body, "cannot roll back retained") {
			t.Fatal("missing retained-fact refusal")
		}
		for _, bad := range []string{"DELETE FROM", "TRUNCATE", "CASCADE"} {
			if strings.Contains(strings.ToUpper(body), bad) {
				t.Fatalf("rollback removes protected history: %s", bad)
			}
		}
	}
}

func TestDeliveryCompletionReadinessUsesCanonicalRuntimeQuery(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(realMigrationsDir(t), "..", "pkg", "db", "queries", "delivery_completion.sql"))
	if err != nil {
		t.Fatal(err)
	}
	query, _, ok := strings.Cut(string(raw), "-- name: GetDeliveryCompletionAuthorization")
	if !ok {
		t.Fatal("missing completion readiness query boundary")
	}
	var lines []string
	for _, line := range strings.Split(query, "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), "--") {
			lines = append(lines, line)
		}
	}
	statement := strings.TrimSuffix(strings.TrimSpace(strings.Join(lines, "\n")), ";")
	if !strings.HasPrefix(statement, "SELECT ") || !strings.Contains(readMigrationFile(t, "788_delivery_completion_ready.up.sql"), "IF NOT ("+statement+") THEN") {
		t.Fatal("migration readiness diverged from the runtime admission query")
	}
}
