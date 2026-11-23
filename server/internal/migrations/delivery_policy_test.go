package migrations

import (
	"strings"
	"testing"
)

func TestDeliveryPolicyMigrationContract(t *testing.T) {
	up := strings.ToUpper(readMigrationFile(t, "776_delivery_policy_evaluation.up.sql"))
	for _, forbidden := range []string{"FOREIGN KEY", "REFERENCES ", "PRIMARY KEY", " UNIQUE", "CASCADE"} {
		if strings.Contains(up, forbidden) {
			t.Fatalf("delivery history tables introduced %s", forbidden)
		}
	}
	for _, stem := range []string{"777_project_delivery_policy_revision", "778_delivery_evaluation_key", "779_delivery_evaluation_id"} {
		sql := readMigrationFile(t, stem+".up.sql")
		if strings.Count(sql, ";") != 1 || !strings.Contains(sql, "CREATE UNIQUE INDEX CONCURRENTLY") {
			t.Fatalf("%s must contain one concurrent unique index statement", stem)
		}
		down := readMigrationFile(t, stem+".down.sql")
		if strings.Count(down, ";") != 1 || !strings.Contains(down, "DROP INDEX CONCURRENTLY") {
			t.Fatalf("%s must drop its index in one concurrent statement", stem)
		}
	}
	for _, stem := range []string{"780_delivery_policy_evaluation_ready", "776_delivery_policy_evaluation"} {
		down := readMigrationFile(t, stem+".down.sql")
		if strings.Count(down, "DO $$") != 1 || strings.Count(down, "END; $$;") != 1 {
			t.Fatalf("%s must keep its fence and retention refusal in one atomic DO block", stem)
		}
		for _, table := range []string{"delivery_evidence_evaluation", "project_delivery_policy"} {
			lock := strings.Index(down, "LOCK TABLE "+table+" IN ACCESS EXCLUSIVE MODE NOWAIT")
			check := strings.Index(down, "IF EXISTS (SELECT 1 FROM "+table+")")
			if lock < 0 || check < lock {
				t.Fatalf("%s must lock %s before checking for retained history", stem, table)
			}
			if drop := strings.Index(down, "DROP TABLE IF EXISTS "+table); drop >= 0 && drop < check {
				t.Fatalf("%s drops %s before its retention guard", stem, table)
			}
		}
		if strings.Index(down, "LOCK TABLE delivery_evidence_evaluation") > strings.Index(down, "LOCK TABLE project_delivery_policy") {
			t.Fatalf("%s changes the common rollback lock order", stem)
		}
		upper := strings.ToUpper(down)
		if strings.Contains(upper, "DELETE FROM") || strings.Contains(upper, "TRUNCATE") || strings.Contains(upper, "CASCADE") {
			t.Fatalf("%s removes retained evidence instead of refusing rollback", stem)
		}
	}
}
