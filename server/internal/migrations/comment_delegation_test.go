package migrations

import (
	"strings"
	"testing"
)

func TestDelegationReceiptMigrationContract(t *testing.T) {
	up := strings.ToUpper(readMigrationFile(t, "771_comment_delegation_receipt.up.sql"))
	for _, bad := range []string{"FOREIGN KEY", "REFERENCES ", "PRIMARY KEY", " UNIQUE"} {
		if strings.Contains(up, bad) {
			t.Fatalf("table contains %s", bad)
		}
	}
	idx := readMigrationFile(t, "772_comment_delegation_receipt_key.up.sql")
	if strings.Count(idx, ";") != 1 || !strings.Contains(idx, "CREATE UNIQUE INDEX CONCURRENTLY") {
		t.Fatal("index must be independent concurrent statement")
	}
	for _, stem := range []string{"771_comment_delegation_receipt", "773_comment_delegation_receipt_ready", "775_comment_delegation_replay_ready"} {
		down := readMigrationFile(t, stem+".down.sql")
		if !strings.Contains(down, "ACCESS EXCLUSIVE MODE NOWAIT") || !strings.Contains(down, "IF EXISTS (SELECT 1 FROM comment_delegation_receipt)") {
			t.Fatal("rollback may lose receipts")
		}
	}
}
