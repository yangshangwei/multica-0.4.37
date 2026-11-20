package migrations

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// issueBudgetIndexMigrations lists every index of the F3 budget tables with
// the migration that builds it. The catalog half of this contract lives in
// internal/handler/issue_budget_schema_test.go, which compares the indexes a
// migrated database really has against the same files.
//
// columns is the key as the migration file writes it. predicate is the WHERE
// clause of a partial index as the file writes it, and empty for an index
// over every row; ready is that predicate in the form the readiness check
// compares, without whitespace, parentheses and casts.
var issueBudgetIndexMigrations = []struct {
	stem, index, table string
	unique             bool
	columns            string
	predicate, ready   string
}{
	{"870_issue_budget_root_id", "issue_budget_root_id_key", "issue_budget_root", true, "id", "", ""},
	{"871_issue_budget_root_issue", "issue_budget_root_issue_key", "issue_budget_root", true, "workspace_id, root_issue_id", "", ""},
	{"872_issue_budget_member_issue", "issue_budget_member_issue_key", "issue_budget_member", true, "workspace_id, issue_id", "", ""},
	{"873_issue_budget_member_root", "issue_budget_member_root_idx", "issue_budget_member", false, "budget_root_id", "", ""},
	{"874_issue_budget_attempt_id", "issue_budget_attempt_id_key", "issue_budget_attempt", true, "id", "", ""},
	{"875_issue_budget_attempt_claim", "issue_budget_attempt_claim_key", "issue_budget_attempt", true, "task_id, claim_generation", "", ""},
	{"876_issue_budget_attempt_root", "issue_budget_attempt_root_idx", "issue_budget_attempt", false, "budget_root_id", "", ""},
	{"877_issue_budget_entry_seq", "issue_budget_entry_seq_key", "issue_budget_entry", true, "budget_root_id, seq", "", ""},
	{"878_issue_budget_entry_identity", "issue_budget_entry_identity_key", "issue_budget_entry", true, "budget_root_id, entry_id", "", ""},
	{"879_issue_budget_entry_attempt", "issue_budget_entry_attempt_idx", "issue_budget_entry", false, "attempt_id", "", ""},
	{"880_issue_budget_operation_id", "issue_budget_operation_id_key", "issue_budget_operation", true, "id", "", ""},
	{"881_issue_budget_operation_scope", "issue_budget_operation_scope_key", "issue_budget_operation", true, "workspace_id, scope_kind, scope_id, operation_key", "", ""},
	{"882_issue_budget_observation_root", "issue_budget_observation_root_idx", "issue_budget_observation", false, "budget_root_id", "", ""},
	{"883_issue_budget_observation_gap", "issue_budget_observation_gap_idx", "issue_budget_observation", false, "budget_root_id, reason", "kind = 'coverage_gap'", "kind=''coverage_gap''"},
	{"884_issue_budget_attempt_open", "issue_budget_attempt_open_idx", "issue_budget_attempt", false, "budget_root_id", "state = 'open'", "state=''open''"},
}

// issueBudgetReadyMigration is the readiness check: the last migration of the
// budget tables, after every index.
const issueBudgetReadyMigration = "885_issue_budget_ready"

var issueBudgetTables = []string{
	"issue_budget_root", "issue_budget_member", "issue_budget_attempt",
	"issue_budget_entry", "issue_budget_operation", "issue_budget_observation",
}

func TestIssueBudgetTableMigrationContract(t *testing.T) {
	raw := readMigrationFile(t, "869_issue_budget.up.sql")
	up := strings.ToUpper(stripIssueBudgetSQLComments(raw))
	// Budget rows keep ID copies so an issue delete cannot cascade into them,
	// and every index is built by its own concurrent migration.
	for _, bad := range []string{
		"FOREIGN KEY", "REFERENCES", "PRIMARY KEY", "UNIQUE", "CASCADE",
		"TRIGGER", "CREATE TYPE", "CREATE INDEX", "CREATE FUNCTION",
	} {
		if strings.Contains(up, bad) {
			t.Errorf("869_issue_budget.up.sql contains %q", bad)
		}
	}
	if got := strings.Count(up, "CREATE TABLE "); got != len(issueBudgetTables) {
		t.Errorf("869_issue_budget.up.sql creates %d tables, want %d", got, len(issueBudgetTables))
	}
	for _, table := range issueBudgetTables {
		body := issueBudgetTableBody(t, raw, table)
		if !regexp.MustCompile(`(?m)^\s*workspace_id uuid NOT NULL,`).MatchString(body) {
			t.Errorf("%s: workspace_id uuid NOT NULL is missing", table)
		}
		// Amounts are ticks in a signed 64-bit column that refuses negatives.
		// The declared reserve also refuses zero: it is a positive amount, and
		// "nothing declared" is NULL (design 10.8).
		for _, m := range regexp.MustCompile(`(?m)^\s*([a-z_]+_ticks)\s+([^\n]*)$`).FindAllStringSubmatch(body, -1) {
			column, rest := m[1], m[2]
			bound := ">= 0"
			if issueBudgetPositiveAmounts[table+"."+column] {
				bound = "> 0"
			}
			if !strings.HasPrefix(rest, "bigint ") || !strings.Contains(rest, "CHECK ("+column+" "+bound+")") {
				t.Errorf("%s.%s: want bigint with CHECK (%s %s), got %q", table, column, column, bound, rest)
			}
		}
	}
	// The exceptions above are columns the file really has.
	for name := range issueBudgetPositiveAmounts {
		table, column, _ := strings.Cut(name, ".")
		if !regexp.MustCompile(`(?m)^\s*` + column + `\s+bigint `).MatchString(issueBudgetTableBody(t, raw, table)) {
			t.Errorf("%s is listed as a positive amount but 869_issue_budget.up.sql has no such bigint column", name)
		}
	}
}

// issueBudgetPositiveAmounts are the amount columns that refuse zero as well
// as negatives.
var issueBudgetPositiveAmounts = map[string]bool{
	"issue_budget_root.attempt_reserve_ticks": true,
}

func TestIssueBudgetIndexMigrationContract(t *testing.T) {
	partial := 0
	for _, m := range issueBudgetIndexMigrations {
		up := strings.TrimSpace(readMigrationFile(t, m.stem+".up.sql"))
		create := "CREATE INDEX CONCURRENTLY IF NOT EXISTS "
		if m.unique {
			create = "CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS "
		}
		// The whole statement: the key and, for a partial index, its predicate.
		want := create + m.index + " ON " + m.table + " (" + m.columns + ")"
		if m.predicate != "" {
			partial++
			want += " WHERE " + m.predicate
		}
		if want += ";"; up != want {
			t.Errorf("%s.up.sql: want %q, got %q", m.stem, want, up)
		}
		if (m.predicate == "") != (m.ready == "") {
			t.Errorf("%s: predicate %q and its readiness form %q do not go together", m.stem, m.predicate, m.ready)
		}
		down := strings.TrimSpace(readMigrationFile(t, m.stem+".down.sql"))
		if want := "DROP INDEX CONCURRENTLY IF EXISTS " + m.index + ";"; down != want {
			t.Errorf("%s.down.sql: want %q, got %q", m.stem, want, down)
		}
	}
	// Two partial indexes: the coverage gaps (design 10.12) and the open
	// attempts the periodic reconciliation reads (design 10.3).
	if partial != 2 {
		t.Errorf("%d partial budget indexes are listed, want 2", partial)
	}
}

// The readiness check names every index with its table, uniqueness, key and
// predicate, one line each, and is the last budget migration.
func TestIssueBudgetReadyMigrationListsEveryIndex(t *testing.T) {
	ready := readMigrationFile(t, issueBudgetReadyMigration+".up.sql")
	for _, m := range issueBudgetIndexMigrations {
		predicate := "NULL"
		if m.ready != "" {
			predicate = "'" + m.ready + "'"
		}
		unique := "false"
		if m.unique {
			unique = "true"
		}
		line := "('" + m.table + "', '" + m.index + "', " + unique + ", '" + m.columns + "', " + predicate + ")"
		if got := strings.Count(ready, line); got != 1 {
			t.Errorf("%s.up.sql has %d lines %s, want 1", issueBudgetReadyMigration, got, line)
		}
		if m.stem >= issueBudgetReadyMigration {
			t.Errorf("%s does not run before the readiness check %s", m.stem, issueBudgetReadyMigration)
		}
	}
	if got := strings.Count(ready, "\n  ('issue_budget_"); got != len(issueBudgetIndexMigrations) {
		t.Errorf("%s.up.sql verifies %d indexes, want %d", issueBudgetReadyMigration, got, len(issueBudgetIndexMigrations))
	}
	// The predicate of every index is compared, not only required to be absent.
	if strings.Contains(ready, "indpred IS NULL") || !strings.Contains(ready, "IS NOT DISTINCT FROM spec.predicate") {
		t.Errorf("%s.up.sql does not compare each index's predicate with the expected one", issueBudgetReadyMigration)
	}
	// Nothing of the budget tables comes after it.
	paths, err := filepath.Glob(filepath.Join(realMigrationsDir(t), "*issue_budget*.up.sql"))
	if err != nil || len(paths) != len(issueBudgetIndexMigrations)+2 {
		t.Fatalf("budget up migrations: %d files, %v; want the table file, %d indexes and the readiness check", len(paths), err, len(issueBudgetIndexMigrations))
	}
	for _, path := range paths {
		if stem := strings.TrimSuffix(filepath.Base(path), ".up.sql"); stem > issueBudgetReadyMigration {
			t.Errorf("%s runs after the readiness check %s", stem, issueBudgetReadyMigration)
		}
	}
}

// Rolling the schema back must not discard a ledger: the first rollback step
// (the highest migration) and the table drop both refuse while any budget row
// exists.
func TestIssueBudgetRollbackKeepsLedgerRows(t *testing.T) {
	for _, stem := range []string{"869_issue_budget", issueBudgetReadyMigration} {
		down := readMigrationFile(t, stem+".down.sql")
		if !strings.Contains(down, "RAISE EXCEPTION") {
			t.Errorf("%s.down.sql: no refusal", stem)
		}
		for _, table := range issueBudgetTables {
			if !strings.Contains(down, "LOCK TABLE "+table+" IN ACCESS EXCLUSIVE MODE NOWAIT") {
				t.Errorf("%s.down.sql: does not lock %s before checking it", stem, table)
			}
			if !strings.Contains(down, "EXISTS (SELECT 1 FROM "+table+")") {
				t.Errorf("%s.down.sql: does not check %s for rows", stem, table)
			}
		}
	}
}

func stripIssueBudgetSQLComments(sql string) string {
	return regexp.MustCompile(`(?m)--.*$`).ReplaceAllString(sql, "")
}

func issueBudgetTableBody(t *testing.T, sql, table string) string {
	t.Helper()
	start := strings.Index(sql, "CREATE TABLE "+table+" (")
	if start < 0 {
		t.Fatalf("869_issue_budget.up.sql does not create %s", table)
	}
	end := strings.Index(sql[start:], "\n);")
	if end < 0 {
		t.Fatalf("869_issue_budget.up.sql: %s has no closing line", table)
	}
	return stripIssueBudgetSQLComments(sql[start : start+end])
}
