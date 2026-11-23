package migrations

import (
	"strings"
	"testing"
)

func TestArtifactExecutionMigrationFilesAreFailClosed(t *testing.T) {
	up := readMigrationFile(t, "766_agent_artifact_execution.up.sql")
	for _, forbidden := range []string{"FOREIGN KEY", "REFERENCES ", "PRIMARY KEY", " UNIQUE"} {
		if strings.Contains(strings.ToUpper(up), forbidden) {
			t.Fatalf("table introduced %s", forbidden)
		}
	}
	for _, stem := range []string{"767_agent_artifact_execution_id", "768_agent_artifact_execution_approval", "769_agent_artifact_execution_idempotency"} {
		sql := readMigrationFile(t, stem+".up.sql")
		if strings.Count(sql, ";") != 1 || !strings.Contains(sql, "CREATE UNIQUE INDEX CONCURRENTLY") {
			t.Fatalf("%s must be a standalone concurrent index", stem)
		}
		if !strings.Contains(readMigrationFile(t, stem+".down.sql"), "DROP INDEX CONCURRENTLY") {
			t.Fatalf("%s missing concurrent down", stem)
		}
	}
	for _, stem := range []string{"770_agent_artifact_execution_ready", "766_agent_artifact_execution"} {
		down := readMigrationFile(t, stem+".down.sql")
		if !strings.Contains(down, "ACCESS EXCLUSIVE MODE NOWAIT") || !strings.Contains(down, "IF EXISTS (SELECT 1 FROM agent_artifact_execution)") {
			t.Fatalf("%s must refuse busy or retained receipts", stem)
		}
	}
}
