package main

import (
	"slices"
	"testing"
)

var issueBudgetMigrationVersions = []string{
	"869_issue_budget",
	"870_issue_budget_root_id",
	"871_issue_budget_root_issue",
	"872_issue_budget_member_issue",
	"873_issue_budget_member_root",
	"874_issue_budget_attempt_id",
	"875_issue_budget_attempt_claim",
	"876_issue_budget_attempt_root",
	"877_issue_budget_entry_seq",
	"878_issue_budget_entry_identity",
	"879_issue_budget_entry_attempt",
	"880_issue_budget_operation_id",
	"881_issue_budget_operation_scope",
	"882_issue_budget_observation_root",
	"883_issue_budget_observation_gap",
	"884_issue_budget_attempt_open",
	"885_issue_budget_ready",
}

// issueBudgetIndexTotal is the number of indexes the budget migrations build.
const issueBudgetIndexTotal = 15

const (
	issueBudgetTableCount = `SELECT count(*) FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = current_schema() AND c.relkind = 'r' AND c.relname LIKE 'issue\_budget\_%'`
	issueBudgetIndexCount = `SELECT count(*) FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = current_schema() AND c.relkind = 'i' AND c.relname LIKE 'issue\_budget\_%'`
)

// The budget migrations reference no other table, so they run on their own in
// a private schema: up, down through exactly these files, and up again.
func TestIssueBudgetMigrationsRunUpDownUp(t *testing.T) {
	ctx, fx, _, opts := switchMigrationFixture(t)
	opts.Files = realMigrationFiles(t, issueBudgetMigrationVersions, "up")

	tablesOnly := opts
	tablesOnly.Files = tablesOnly.Files[:1]
	if err := runMigrations(ctx, fx.Pool, tablesOnly); err != nil {
		t.Fatalf("create tables: %v", err)
	}
	if got := fx.Count(t, issueBudgetTableCount); got != 6 {
		t.Fatalf("table migration created %d budget tables, want 6", got)
	}
	// No PRIMARY KEY or UNIQUE in the table migration: either one would build
	// an index inside the table's own statement instead of concurrently.
	if got := fx.Count(t, issueBudgetIndexCount); got != 0 {
		t.Fatalf("table migration created %d implicit indexes, want 0", got)
	}

	if err := runMigrations(ctx, fx.Pool, opts); err != nil {
		t.Fatalf("up: %v", err)
	}
	if got := fx.Count(t, issueBudgetIndexCount); got != issueBudgetIndexTotal {
		t.Fatalf("up left %d budget indexes, want %d", got, issueBudgetIndexTotal)
	}
	// Two of them are partial: the index of the coverage gaps (design 10.12)
	// and the index of the open attempts (design 10.3).
	if got := fx.Count(t, `SELECT count(*) FROM pg_index i JOIN pg_class c ON c.oid = i.indexrelid
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = current_schema() AND c.relname = 'issue_budget_observation_gap_idx'
		AND pg_get_expr(i.indpred, i.indrelid) LIKE '%coverage_gap%'`); got != 1 {
		t.Fatalf("up left %d partial indexes of the coverage gaps, want 1", got)
	}
	if got := fx.Count(t, `SELECT count(*) FROM pg_index i JOIN pg_class c ON c.oid = i.indexrelid
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = current_schema() AND c.relname = 'issue_budget_attempt_open_idx'
		AND pg_get_expr(i.indpred, i.indrelid) LIKE '%open%'`); got != 1 {
		t.Fatalf("up left %d partial indexes of the open attempts, want 1", got)
	}
	if got := fx.Count(t, `SELECT count(*) FROM pg_index i JOIN pg_class c ON c.oid = i.indexrelid
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = current_schema() AND c.relname LIKE 'issue\_budget\_%' AND i.indpred IS NOT NULL`); got != 2 {
		t.Fatalf("up left %d partial budget indexes, want 2", got)
	}
	if got := fx.Count(t, `SELECT count(*) FROM pg_index i JOIN pg_class c ON c.oid = i.indexrelid
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = current_schema() AND c.relname LIKE 'issue\_budget\_%'
		AND NOT (i.indisvalid AND i.indisready AND i.indislive)`); got != 0 {
		t.Fatalf("%d budget indexes are not valid", got)
	}

	down := opts
	down.Direction = "down"
	down.Hooks = hooksForDirection("down")
	reversed := slices.Clone(issueBudgetMigrationVersions)
	slices.Reverse(reversed)
	down.Files = realMigrationFiles(t, reversed, "down")
	if err := runMigrations(ctx, fx.Pool, down); err != nil {
		t.Fatalf("down on empty tables: %v", err)
	}
	if tables, indexes := fx.Count(t, issueBudgetTableCount), fx.Count(t, issueBudgetIndexCount); tables != 0 || indexes != 0 {
		t.Fatalf("down left %d tables and %d indexes", tables, indexes)
	}
	if got := fx.Count(t, `SELECT count(*) FROM schema_migrations`); got != 0 {
		t.Fatalf("down left %d ledger rows", got)
	}

	if err := runMigrations(ctx, fx.Pool, opts); err != nil {
		t.Fatalf("second up: %v", err)
	}
	if tables, indexes := fx.Count(t, issueBudgetTableCount), fx.Count(t, issueBudgetIndexCount); tables != 6 || indexes != issueBudgetIndexTotal {
		t.Fatalf("second up left %d tables and %d indexes, want 6 and %d", tables, indexes, issueBudgetIndexTotal)
	}
}

// A ledger is retained evidence: rollback refuses at its first step while any
// budget table holds a row, before a single index is dropped.
func TestIssueBudgetRollbackRefusesWhileLedgerRowsExist(t *testing.T) {
	ctx, fx, _, opts := switchMigrationFixture(t)
	opts.Files = realMigrationFiles(t, issueBudgetMigrationVersions, "up")
	if err := runMigrations(ctx, fx.Pool, opts); err != nil {
		t.Fatalf("up: %v", err)
	}
	down := opts
	down.Direction = "down"
	down.Hooks = hooksForDirection("down")
	reversed := slices.Clone(issueBudgetMigrationVersions)
	slices.Reverse(reversed)
	down.Files = realMigrationFiles(t, reversed, "down")

	// The observation table is the last one the guard reads, and a row there
	// is not money: it must hold the rollback back all the same.
	fx.Exec(t, `INSERT INTO issue_budget_observation (id, workspace_id, budget_root_id, kind, reason)
		VALUES (gen_random_uuid(), gen_random_uuid(), gen_random_uuid(), 'coverage_gap', 'legacy_protocol')`)
	artifactPGCode(t, runMigrations(ctx, fx.Pool, down), "P0001")
	if tables, indexes := fx.Count(t, issueBudgetTableCount), fx.Count(t, issueBudgetIndexCount); tables != 6 || indexes != issueBudgetIndexTotal {
		t.Fatalf("refused rollback left %d tables and %d indexes, want 6 and %d", tables, indexes, issueBudgetIndexTotal)
	}
	if got := fx.Count(t, `SELECT count(*) FROM schema_migrations`); got != len(issueBudgetMigrationVersions) {
		t.Fatalf("refused rollback left %d ledger rows, want %d", got, len(issueBudgetMigrationVersions))
	}
	if got := fx.Count(t, `SELECT count(*) FROM issue_budget_observation`); got != 1 {
		t.Fatalf("refused rollback left %d observation rows, want 1", got)
	}

	// The table drop carries the same guard, so skipping the first step does
	// not get past it.
	tableDown := down
	tableDown.Files = down.Files[len(down.Files)-1:]
	artifactPGCode(t, runMigrations(ctx, fx.Pool, tableDown), "P0001")
	if got := fx.Count(t, issueBudgetTableCount); got != 6 {
		t.Fatalf("guarded table drop left %d tables, want 6", got)
	}
}

// The ready migration is the last up step. It fails when a required unique
// index is missing or was rebuilt without uniqueness.
func TestIssueBudgetReadyMigrationRejectsAWeakenedIndex(t *testing.T) {
	ctx, fx, _, opts := switchMigrationFixture(t)
	versions := issueBudgetMigrationVersions[:len(issueBudgetMigrationVersions)-1]
	opts.Files = realMigrationFiles(t, versions, "up")
	if err := runMigrations(ctx, fx.Pool, opts); err != nil {
		t.Fatalf("up without ready: %v", err)
	}
	fx.Exec(t, `DROP INDEX CONCURRENTLY issue_budget_attempt_claim_key`)
	fx.Exec(t, `CREATE INDEX CONCURRENTLY issue_budget_attempt_claim_key ON issue_budget_attempt (task_id, claim_generation)`)

	ready := opts
	ready.Files = realMigrationFiles(t, issueBudgetMigrationVersions[len(issueBudgetMigrationVersions)-1:], "up")
	artifactPGCode(t, runMigrations(ctx, fx.Pool, ready), "P0001")

	fx.Exec(t, `DROP INDEX CONCURRENTLY issue_budget_attempt_claim_key`)
	fx.Exec(t, `CREATE UNIQUE INDEX CONCURRENTLY issue_budget_attempt_claim_key ON issue_budget_attempt (task_id, claim_generation)`)
	if err := runMigrations(ctx, fx.Pool, ready); err != nil {
		t.Fatalf("ready after restoring the unique index: %v", err)
	}
}

// The index of the coverage gaps is one of the two partial indexes of the
// budget tables. IF NOT EXISTS accepts any index of its name, so the readiness check
// compares the predicate too: it refuses the name on an index without a
// predicate, with another predicate and with another key, and accepts the
// predicate however it was spelled when the index was built.
func TestIssueBudgetReadyMigrationRejectsAnotherGapIndex(t *testing.T) {
	ctx, fx, _, opts := switchMigrationFixture(t)
	last := len(issueBudgetMigrationVersions) - 1
	opts.Files = realMigrationFiles(t, issueBudgetMigrationVersions[:last], "up")
	if err := runMigrations(ctx, fx.Pool, opts); err != nil {
		t.Fatalf("up without ready: %v", err)
	}
	ready := opts
	ready.Files = realMigrationFiles(t, issueBudgetMigrationVersions[last:], "up")
	const name = "issue_budget_observation_gap_idx"
	rebuild := func(definition string) {
		t.Helper()
		fx.Exec(t, `DROP INDEX CONCURRENTLY `+name)
		fx.Exec(t, `CREATE INDEX CONCURRENTLY `+name+` ON issue_budget_observation `+definition)
	}
	recorded := func() int {
		t.Helper()
		return fx.Count(t, `SELECT count(*) FROM schema_migrations WHERE version = $1`, issueBudgetMigrationVersions[last])
	}

	for _, tc := range []struct{ what, definition string }{
		{"without a predicate", `(budget_root_id, reason)`},
		{"with another predicate", `(budget_root_id, reason) WHERE kind = 'would_refuse'`},
		{"with a predicate that excludes instead of naming", `(budget_root_id, reason) WHERE kind <> 'would_refuse'`},
		{"with a second condition", `(budget_root_id, reason) WHERE kind = 'coverage_gap' AND reason = 'legacy_protocol'`},
		{"with a condition that is always true beside it", `(budget_root_id, reason) WHERE kind = 'coverage_gap' AND true`},
		{"on the root alone", `(budget_root_id) WHERE kind = 'coverage_gap'`},
		{"on the root alone without a predicate", `(budget_root_id)`},
		{"with the key the other way round", `(reason, budget_root_id) WHERE kind = 'coverage_gap'`},
		{"with an included column", `(budget_root_id, reason) INCLUDE (kind) WHERE kind = 'coverage_gap'`},
	} {
		rebuild(tc.definition)
		artifactPGCode(t, runMigrations(ctx, fx.Pool, ready), "P0001")
		if got := recorded(); got != 0 {
			t.Fatalf("the readiness check refused the gap index %s and was recorded %d times", tc.what, got)
		}
	}

	// The same index under another name does not stand in for it.
	fx.Exec(t, `DROP INDEX CONCURRENTLY `+name)
	fx.Exec(t, `CREATE INDEX CONCURRENTLY issue_budget_observation_gap_other_idx ON issue_budget_observation (budget_root_id, reason) WHERE kind = 'coverage_gap'`)
	artifactPGCode(t, runMigrations(ctx, fx.Pool, ready), "P0001")
	fx.Exec(t, `DROP INDEX CONCURRENTLY issue_budget_observation_gap_other_idx`)
	fx.Exec(t, `CREATE INDEX CONCURRENTLY `+name+` ON issue_budget_observation (budget_root_id)`)

	// Spellings of the same predicate are one predicate.
	for _, definition := range []string{
		`(budget_root_id, reason) WHERE kind = 'coverage_gap'::text`,
		`(budget_root_id, reason) WHERE (kind IN ('coverage_gap'))`,
		`(budget_root_id, reason) WHERE kind = 'coverage_gap'`,
	} {
		rebuild(definition)
		if err := runMigrations(ctx, fx.Pool, ready); err != nil {
			t.Fatalf("ready with the gap index built as %s: %v", definition, err)
		}
		if got := recorded(); got != 1 {
			t.Fatalf("ready with the gap index built as %s was recorded %d times, want 1", definition, got)
		}
		fx.Exec(t, `DELETE FROM schema_migrations WHERE version = $1`, issueBudgetMigrationVersions[last])
	}
}

// The index of the open attempts is the other partial index. The periodic
// reconciliation finds its roots through it, so an index of that name over
// every attempt, or over the attempts of another state, would let the query
// read the whole table on every tick without anybody noticing. The readiness
// check refuses each of them and accepts the predicate however it was spelled.
func TestIssueBudgetReadyMigrationRejectsAnotherOpenAttemptIndex(t *testing.T) {
	ctx, fx, _, opts := switchMigrationFixture(t)
	last := len(issueBudgetMigrationVersions) - 1
	opts.Files = realMigrationFiles(t, issueBudgetMigrationVersions[:last], "up")
	if err := runMigrations(ctx, fx.Pool, opts); err != nil {
		t.Fatalf("up without ready: %v", err)
	}
	ready := opts
	ready.Files = realMigrationFiles(t, issueBudgetMigrationVersions[last:], "up")
	const name = "issue_budget_attempt_open_idx"
	rebuild := func(definition string, unique bool) {
		t.Helper()
		create := `CREATE INDEX CONCURRENTLY `
		if unique {
			create = `CREATE UNIQUE INDEX CONCURRENTLY `
		}
		fx.Exec(t, `DROP INDEX CONCURRENTLY `+name)
		fx.Exec(t, create+name+` ON issue_budget_attempt `+definition)
	}
	recorded := func() int {
		t.Helper()
		return fx.Count(t, `SELECT count(*) FROM schema_migrations WHERE version = $1`, issueBudgetMigrationVersions[last])
	}

	for _, tc := range []struct {
		what, definition string
		unique           bool
	}{
		{"without a predicate", `(budget_root_id)`, false},
		{"of another state", `(budget_root_id) WHERE state = 'settled'`, false},
		{"of every state but open", `(budget_root_id) WHERE state <> 'open'`, false},
		{"with a second condition", `(budget_root_id) WHERE state = 'open' AND reserved_ticks IS NOT NULL`, false},
		{"with a condition that is always true beside it", `(budget_root_id) WHERE state = 'open' AND true`, false},
		{"on the task", `(task_id) WHERE state = 'open'`, false},
		{"with a second key column", `(budget_root_id, task_id) WHERE state = 'open'`, false},
		{"with an included column", `(budget_root_id) INCLUDE (task_id) WHERE state = 'open'`, false},
		{"that is unique", `(budget_root_id) WHERE state = 'open'`, true},
	} {
		rebuild(tc.definition, tc.unique)
		artifactPGCode(t, runMigrations(ctx, fx.Pool, ready), "P0001")
		if got := recorded(); got != 0 {
			t.Fatalf("the readiness check refused the open-attempt index %s and was recorded %d times", tc.what, got)
		}
	}

	// The same index under another name does not stand in for it.
	fx.Exec(t, `DROP INDEX CONCURRENTLY `+name)
	fx.Exec(t, `CREATE INDEX CONCURRENTLY issue_budget_attempt_open_other_idx ON issue_budget_attempt (budget_root_id) WHERE state = 'open'`)
	artifactPGCode(t, runMigrations(ctx, fx.Pool, ready), "P0001")
	fx.Exec(t, `DROP INDEX CONCURRENTLY issue_budget_attempt_open_other_idx`)
	fx.Exec(t, `CREATE INDEX CONCURRENTLY `+name+` ON issue_budget_attempt (budget_root_id)`)

	// Spellings of the same predicate are one predicate.
	for _, definition := range []string{
		`(budget_root_id) WHERE state = 'open'::text`,
		`(budget_root_id) WHERE (state IN ('open'))`,
		`(budget_root_id) WHERE state = 'open'`,
	} {
		rebuild(definition, false)
		if err := runMigrations(ctx, fx.Pool, ready); err != nil {
			t.Fatalf("ready with the open-attempt index built as %s: %v", definition, err)
		}
		if got := recorded(); got != 1 {
			t.Fatalf("ready with the open-attempt index built as %s was recorded %d times, want 1", definition, got)
		}
		fx.Exec(t, `DELETE FROM schema_migrations WHERE version = $1`, issueBudgetMigrationVersions[last])
	}
}

// An index over every row must not carry a predicate: the readiness check
// refuses a partial index under the name of a plain one.
func TestIssueBudgetReadyMigrationRejectsAPredicateOnAPlainIndex(t *testing.T) {
	ctx, fx, _, opts := switchMigrationFixture(t)
	last := len(issueBudgetMigrationVersions) - 1
	opts.Files = realMigrationFiles(t, issueBudgetMigrationVersions[:last], "up")
	if err := runMigrations(ctx, fx.Pool, opts); err != nil {
		t.Fatalf("up without ready: %v", err)
	}
	ready := opts
	ready.Files = realMigrationFiles(t, issueBudgetMigrationVersions[last:], "up")

	fx.Exec(t, `DROP INDEX CONCURRENTLY issue_budget_observation_root_idx`)
	fx.Exec(t, `CREATE INDEX CONCURRENTLY issue_budget_observation_root_idx ON issue_budget_observation (budget_root_id) WHERE kind = 'coverage_gap'`)
	artifactPGCode(t, runMigrations(ctx, fx.Pool, ready), "P0001")

	fx.Exec(t, `DROP INDEX CONCURRENTLY issue_budget_observation_root_idx`)
	fx.Exec(t, `CREATE INDEX CONCURRENTLY issue_budget_observation_root_idx ON issue_budget_observation (budget_root_id)`)
	if err := runMigrations(ctx, fx.Pool, ready); err != nil {
		t.Fatalf("ready after restoring the plain index: %v", err)
	}
}
