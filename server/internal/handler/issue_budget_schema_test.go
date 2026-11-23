package handler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/multica-ai/multica/server/internal/budget"
	"github.com/multica-ai/multica/server/internal/testutil"
)

var issueBudgetTables = []string{
	"issue_budget_root", "issue_budget_member", "issue_budget_attempt",
	"issue_budget_entry", "issue_budget_operation", "issue_budget_observation",
}

type issueBudgetIndex struct {
	table   string
	unique  bool
	columns string
	// predicate is the WHERE clause of a partial index as its migration file
	// writes it, and empty for an index over every row.
	predicate string
}

// issueBudgetIndexes is the whole index set of the budget tables. The unique
// ones are the keys design section 8 requires; issue_budget_observation has
// none, because one attempt can produce any number of observations. Two
// indexes are partial: the coverage gaps of a root, which a claim counts
// (design 10.12), and the open attempts, which the periodic lifecycle
// reconciliation reads (design 10.3).
var issueBudgetIndexes = map[string]issueBudgetIndex{
	"issue_budget_root_id_key":          {"issue_budget_root", true, "id", ""},
	"issue_budget_root_issue_key":       {"issue_budget_root", true, "workspace_id, root_issue_id", ""},
	"issue_budget_member_issue_key":     {"issue_budget_member", true, "workspace_id, issue_id", ""},
	"issue_budget_member_root_idx":      {"issue_budget_member", false, "budget_root_id", ""},
	"issue_budget_attempt_id_key":       {"issue_budget_attempt", true, "id", ""},
	"issue_budget_attempt_claim_key":    {"issue_budget_attempt", true, "task_id, claim_generation", ""},
	"issue_budget_attempt_root_idx":     {"issue_budget_attempt", false, "budget_root_id", ""},
	"issue_budget_attempt_open_idx":     {"issue_budget_attempt", false, "budget_root_id", "state = 'open'"},
	"issue_budget_entry_seq_key":        {"issue_budget_entry", true, "budget_root_id, seq", ""},
	"issue_budget_entry_identity_key":   {"issue_budget_entry", true, "budget_root_id, entry_id", ""},
	"issue_budget_entry_attempt_idx":    {"issue_budget_entry", false, "attempt_id", ""},
	"issue_budget_operation_id_key":     {"issue_budget_operation", true, "id", ""},
	"issue_budget_operation_scope_key":  {"issue_budget_operation", true, "workspace_id, scope_kind, scope_id, operation_key", ""},
	"issue_budget_observation_root_idx": {"issue_budget_observation", false, "budget_root_id", ""},
	"issue_budget_observation_gap_idx":  {"issue_budget_observation", false, "budget_root_id, reason", "kind = 'coverage_gap'"},
}

// issueBudgetIdentifierBytes is the one length limit of every caller-supplied
// identifier. The metering package accepts identifiers of any length.
const issueBudgetIdentifierBytes = 200

var issueBudgetBoundedIdentifiers = []string{
	"issue_budget_entry.entry_id", "issue_budget_entry.invocation_id", "issue_budget_entry.scope_id",
	"issue_budget_entry.adapter_version", "issue_budget_entry.normalizer_version", "issue_budget_entry.producer_epoch",
	"issue_budget_operation.operation_key",
}

// issueBudgetNamedChecks are the CHECK constraints that span columns or tie a
// stored form to its digest. They carry explicit names so a later migration
// can refer to them; a table-level CHECK left unnamed would be called
// <table>_check, <table>_check1, ... by position.
var issueBudgetNamedChecks = map[string]string{
	"issue_budget_root_settlement_check":        "issue_budget_root",
	"issue_budget_member_root_parent_check":     "issue_budget_member",
	"issue_budget_attempt_attribution_check":    "issue_budget_attempt",
	"issue_budget_attempt_policy_pair_check":    "issue_budget_attempt",
	"issue_budget_entry_digest_check":           "issue_budget_entry",
	"issue_budget_entry_kind_attempt_check":     "issue_budget_entry",
	"issue_budget_entry_usage_invocation_check": "issue_budget_entry",
	"issue_budget_entry_usage_scope_check":      "issue_budget_entry",
	// The price snapshot lives on the entry it priced (design 10.8): both
	// columns or neither, the digest of those very bytes, usage entries only.
	"issue_budget_entry_price_pair_check":   "issue_budget_entry",
	"issue_budget_entry_price_digest_check": "issue_budget_entry",
	"issue_budget_entry_price_usage_check":  "issue_budget_entry",
}

// issueBudgetPositiveAmounts are the amounts that refuse zero as well as
// negatives. A declared reserve of 0 would read as "nothing declared", which
// is what NULL says.
var issueBudgetPositiveAmounts = map[string]bool{
	"issue_budget_root.attempt_reserve_ticks": true,
}

// B51: the budget tables as a migrated database really has them.
func TestIssueBudgetSchemaCatalog(t *testing.T) {
	ctx := context.Background()

	if got := dbfx.Count(t, `SELECT count(*) FROM pg_tables WHERE schemaname = 'public' AND tablename = ANY($1::text[])`, issueBudgetTables); got != len(issueBudgetTables) {
		t.Fatalf("found %d of %d budget tables", got, len(issueBudgetTables))
	}
	if got := dbfx.Count(t, `SELECT count(*) FROM pg_tables WHERE schemaname = 'public' AND tablename LIKE 'issue\_budget\_%'`); got != len(issueBudgetTables) {
		t.Errorf("public schema has %d issue_budget_* tables, want %d", got, len(issueBudgetTables))
	}

	const relations = `(SELECT c.oid FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = 'public' AND c.relname = ANY($1::text[]))`
	// Foreign keys in either direction: none may point out of a budget table
	// and none may point at one. Uniqueness lives in concurrently built
	// indexes, so there is no primary key or unique constraint either.
	if got := dbfx.Count(t, `SELECT count(*) FROM pg_constraint WHERE contype = 'f'
		AND (conrelid IN `+relations+` OR confrelid IN `+relations+`)`, issueBudgetTables); got != 0 {
		t.Errorf("budget tables take part in %d foreign keys, want 0", got)
	}
	if got := dbfx.Count(t, `SELECT count(*) FROM pg_constraint WHERE contype IN ('p', 'u', 'x') AND conrelid IN `+relations, issueBudgetTables); got != 0 {
		t.Errorf("budget tables have %d primary key / unique / exclusion constraints, want 0", got)
	}
	if got := dbfx.Count(t, `SELECT count(*) FROM pg_trigger WHERE tgrelid IN `+relations, issueBudgetTables); got != 0 {
		t.Errorf("budget tables have %d triggers, want 0", got)
	}
	if got := dbfx.Count(t, `SELECT count(*) FROM pg_attribute a WHERE a.attrelid IN `+relations+`
		AND a.attname = 'workspace_id' AND a.atttypid = 'uuid'::regtype AND a.attnotnull AND NOT a.attisdropped`, issueBudgetTables); got != len(issueBudgetTables) {
		t.Errorf("%d of %d budget tables have workspace_id uuid NOT NULL", got, len(issueBudgetTables))
	}
	// State columns are text with a CHECK; no enum, domain or composite type.
	if got := dbfx.Count(t, `SELECT count(*) FROM pg_attribute a JOIN pg_type ty ON ty.oid = a.atttypid
		WHERE a.attrelid IN `+relations+` AND a.attnum > 0 AND NOT a.attisdropped AND ty.typtype <> 'b'`, issueBudgetTables); got != 0 {
		t.Errorf("budget tables have %d columns of a non-base type, want 0", got)
	}

	// Every amount is a bigint that refuses negatives; the declared reserve
	// refuses zero too.
	amounts, err := testPool.Query(ctx, `SELECT c.relname, a.attname, a.atttypid = 'bigint'::regtype,
			EXISTS (SELECT 1 FROM pg_constraint k WHERE k.conrelid = c.oid AND k.contype = 'c'
				AND pg_get_constraintdef(k.oid) = 'CHECK ((' || a.attname || ' >= 0))'),
			EXISTS (SELECT 1 FROM pg_constraint k WHERE k.conrelid = c.oid AND k.contype = 'c'
				AND pg_get_constraintdef(k.oid) = 'CHECK ((' || a.attname || ' > 0))')
		FROM pg_attribute a JOIN pg_class c ON c.oid = a.attrelid
		WHERE c.oid IN `+relations+` AND a.attnum > 0 AND NOT a.attisdropped AND a.attname LIKE '%\_ticks'
		ORDER BY 1, 2`, issueBudgetTables)
	if err != nil {
		t.Fatalf("list amount columns: %v", err)
	}
	defer amounts.Close()
	var amountColumns []string
	for amounts.Next() {
		var table, column string
		var isBigint, nonNegative, positive bool
		if err := amounts.Scan(&table, &column, &isBigint, &nonNegative, &positive); err != nil {
			t.Fatalf("scan amount column: %v", err)
		}
		name := table + "." + column
		amountColumns = append(amountColumns, name)
		wantPositive := issueBudgetPositiveAmounts[name]
		if !isBigint || nonNegative == wantPositive || positive != wantPositive {
			t.Errorf("%s: bigint=%v, CHECK (>= 0)=%v, CHECK (> 0)=%v, want the > 0 form: %v", name, isBigint, nonNegative, positive, wantPositive)
		}
	}
	if err := amounts.Err(); err != nil {
		t.Fatalf("iterate amount columns: %v", err)
	}
	wantAmounts := []string{
		"issue_budget_attempt.reserved_ticks",
		"issue_budget_root.actual_known_ticks", "issue_budget_root.attempt_reserve_ticks", "issue_budget_root.estimated_ticks",
		"issue_budget_root.limit_ticks", "issue_budget_root.overrun_ticks", "issue_budget_root.reserved_ticks",
		"issue_budget_root.unknown_hold_ticks",
	}
	if strings.Join(amountColumns, " ") != strings.Join(wantAmounts, " ") {
		t.Errorf("amount columns = %v, want %v", amountColumns, wantAmounts)
	}

	// Column shapes the later steps rely on. The reporting envelope is columns
	// on the entry row, beside the stored form: the pure entry carries none of
	// them. Only the two facts every entry has are required; a server-written
	// entry leaves the rest empty.
	columnTypes := map[string]string{}
	columns, err := testPool.Query(ctx, `SELECT c.relname || '.' || a.attname, format_type(a.atttypid, a.atttypmod) ||
			CASE WHEN a.attnotnull THEN ' not null' ELSE '' END
		FROM pg_attribute a JOIN pg_class c ON c.oid = a.attrelid
		WHERE c.oid IN `+relations+` AND a.attnum > 0 AND NOT a.attisdropped`, issueBudgetTables)
	if err != nil {
		t.Fatalf("list budget columns: %v", err)
	}
	defer columns.Close()
	for columns.Next() {
		var name, typ string
		if err := columns.Scan(&name, &typ); err != nil {
			t.Fatalf("scan budget column: %v", err)
		}
		columnTypes[name] = typ
	}
	if err := columns.Err(); err != nil {
		t.Fatalf("iterate budget columns: %v", err)
	}
	for name, want := range map[string]string{
		"issue_budget_entry.attempt_id": "uuid", "issue_budget_entry.source": "text not null",
		"issue_budget_entry.schema_version": "integer not null", "issue_budget_entry.adapter_version": "text",
		"issue_budget_entry.normalizer_version": "text", "issue_budget_entry.producer_epoch": "text",
		"issue_budget_entry.payload_digest": "text", "issue_budget_entry.observed_at": "timestamp with time zone",
		"issue_budget_entry.reported_at": "timestamp with time zone", "issue_budget_entry.received_at": "timestamp with time zone",
		"issue_budget_entry.completeness": "text",
		// The batch that appended an entry is cited by the operation's id.
		"issue_budget_entry.operation_id": "uuid", "issue_budget_operation.id": "uuid not null",
		// In-flight gap observations are matched to an attempt by task and
		// claim generation.
		"issue_budget_observation.task_id": "uuid", "issue_budget_observation.claim_generation": "bigint",
		// The policy an attempt was judged under.
		"issue_budget_attempt.policy_revision": "bigint", "issue_budget_attempt.policy_epoch": "bigint",
		// The declared reserve is optional, and an attempt claimed without one
		// has no amount: NULL is the unsized reservation (design 10.1).
		"issue_budget_root.attempt_reserve_ticks": "bigint", "issue_budget_attempt.reserved_ticks": "bigint",
		// The price snapshot is kept on the entry it priced (design 10.8).
		"issue_budget_entry.price_snapshot": "bytea", "issue_budget_entry.price_digest": "text",
	} {
		if got, ok := columnTypes[name]; !ok || got != want {
			t.Errorf("%s = %q (present %v), want %q", name, got, ok, want)
		}
	}
	for _, name := range []string{
		// A deleted root issue is read from its member row; nothing copies it
		// onto the root.
		"issue_budget_root.tombstoned_at",
		// Would-refuse facts are observation rows only.
		"issue_budget_attempt.would_refuse_reason",
		"issue_budget_entry.operation_scope_kind", "issue_budget_entry.operation_scope_id", "issue_budget_entry.operation_key",
		// One attempt can use several models; a snapshot prices one of them.
		"issue_budget_attempt.price_snapshot", "issue_budget_attempt.price_digest",
	} {
		if typ, ok := columnTypes[name]; ok {
			t.Errorf("%s exists (%s), want no such column", name, typ)
		}
	}

	// Constraints a later migration may have to name are named, and no CHECK
	// is left with a positional name.
	for name, table := range issueBudgetNamedChecks {
		if got := dbfx.Count(t, `SELECT count(*) FROM pg_constraint k WHERE k.conrelid = ('public.' || $1)::regclass
			AND k.contype = 'c' AND k.conname = $2`, table, name); got != 1 {
			t.Errorf("%s has %d CHECK constraints named %s, want 1", table, got, name)
		}
	}
	if got := dbfx.Count(t, `SELECT count(*) FROM pg_constraint k JOIN pg_class c ON c.oid = k.conrelid
		WHERE c.oid IN `+relations+` AND k.contype = 'c' AND k.conname ~ ('^' || c.relname || '_check[0-9]*$')`, issueBudgetTables); got != 0 {
		t.Errorf("budget tables have %d CHECK constraints with a positional name, want 0", got)
	}

	// The two envelope columns whose meaning is not in their name say it in
	// the catalog.
	for column, want := range map[string]string{
		"received_at":    "arrived at the server",
		"schema_version": "stored entry form",
	} {
		var comment string
		dbfx.QueryRow(t, `SELECT coalesce(col_description('public.issue_budget_entry'::regclass, a.attnum), '')
			FROM pg_attribute a WHERE a.attrelid = 'public.issue_budget_entry'::regclass AND a.attname = $1`, column).Scan(&comment)
		if !strings.Contains(comment, want) {
			t.Errorf("issue_budget_entry.%s comment = %q, want it to say %q", column, comment, want)
		}
	}

	// Caller-supplied identifiers share one byte limit, written the same way
	// on every column.
	for _, column := range issueBudgetBoundedIdentifiers {
		table, name, _ := strings.Cut(column, ".")
		want := fmt.Sprintf("CHECK (((octet_length(%[1]s) >= 1) AND (octet_length(%[1]s) <= %[2]d)))", name, issueBudgetIdentifierBytes)
		if got := dbfx.Count(t, `SELECT count(*) FROM pg_constraint k WHERE k.conrelid = ('public.' || $1)::regclass
			AND k.contype = 'c' AND pg_get_constraintdef(k.oid) = $2`, table, want); got != 1 {
			t.Errorf("%s has %d constraints %q, want 1", column, got, want)
		}
	}

	// The index set, read from the catalog, against the expected set and
	// against the migration file that built each index.
	// A predicate is read without parentheses, whitespace and text casts: how
	// PostgreSQL prints one differs from how a migration file writes it.
	rows, err := testPool.Query(ctx, `SELECT ic.relname, tc.relname, i.indisunique,
			i.indisvalid AND i.indisready AND i.indislive,
			i.indexprs IS NULL AND i.indnkeyatts = i.indnatts,
			(SELECT string_agg(pg_get_indexdef(i.indexrelid, k, true), ', ' ORDER BY k) FROM generate_series(1, i.indnkeyatts) k),
			COALESCE(regexp_replace(pg_get_expr(i.indpred, i.indrelid), '[()[:space:]]|::text', '', 'g'), '')
		FROM pg_index i JOIN pg_class ic ON ic.oid = i.indexrelid JOIN pg_class tc ON tc.oid = i.indrelid
		WHERE tc.oid IN `+relations+` ORDER BY 1`, issueBudgetTables)
	if err != nil {
		t.Fatalf("list budget indexes: %v", err)
	}
	defer rows.Close()
	migrations := issueBudgetUpMigrations(t)
	seen := map[string]bool{}
	for rows.Next() {
		var name, table, columns, predicate string
		var unique, usable, plain bool
		if err := rows.Scan(&name, &table, &unique, &usable, &plain, &columns, &predicate); err != nil {
			t.Fatalf("scan budget index: %v", err)
		}
		seen[name] = true
		want, ok := issueBudgetIndexes[name]
		if !ok {
			t.Errorf("unexpected index %s on %s (%s) WHERE %q", name, table, columns, predicate)
			continue
		}
		if got := (issueBudgetIndex{table, unique, columns, want.predicate}); got != want {
			t.Errorf("index %s = %+v, want %+v", name, got, want)
		}
		// The predicate the database has is the one the file writes, and an
		// index that is to cover every row has none.
		if wantPredicate := strings.NewReplacer(" ", "", "(", "", ")", "").Replace(want.predicate); predicate != wantPredicate {
			t.Errorf("index %s has the predicate %q, want %q", name, predicate, wantPredicate)
		}
		if !usable || !plain {
			t.Errorf("index %s: valid/ready/live=%v, plain column index=%v", name, usable, plain)
		}
		// The statement is rebuilt from the catalog, so a file that builds a
		// different index than the database has cannot pass.
		statement := "CREATE INDEX CONCURRENTLY IF NOT EXISTS "
		if unique {
			statement = "CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS "
		}
		statement += name + " ON " + table + " (" + columns + ")"
		if want.predicate != "" {
			statement += " WHERE " + want.predicate
		}
		statement += ";"
		var builders []string
		creates := regexp.MustCompile(`(?i)CREATE\s+(?:UNIQUE\s+)?INDEX[^;]*\b` + name + `\b`)
		for file, body := range migrations {
			if creates.MatchString(body) {
				builders = append(builders, file)
			}
		}
		if len(builders) != 1 {
			t.Errorf("index %s is created by %d migration files %v, want exactly 1", name, len(builders), builders)
			continue
		}
		if body := strings.TrimSpace(migrations[builders[0]]); body != statement {
			t.Errorf("%s is not the single concurrent statement for %s:\n got %q\nwant %q", builders[0], name, body, statement)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate budget indexes: %v", err)
	}
	for name := range issueBudgetIndexes {
		if !seen[name] {
			t.Errorf("index %s is missing", name)
		}
	}
}

// The keys and value rules later steps rely on, exercised against real rows.
func TestIssueBudgetSchemaConstraints(t *testing.T) {
	rootIssue, memberIssue, task := uuid.NewString(), uuid.NewString(), uuid.NewString()
	rootCols := issueBudgetRootCols(rootIssue)
	root := dbfx.Insert(t, "issue_budget_root", rootCols)
	insertIssueBudgetMember(t, issueBudgetMemberCols(root, rootIssue, ""))
	insertIssueBudgetMember(t, issueBudgetMemberCols(root, memberIssue, rootIssue))
	attemptCols := issueBudgetAttemptCols(root, task, memberIssue)
	attempt := dbfx.Insert(t, "issue_budget_attempt", attemptCols)

	reserve := budget.Entry{ID: "reserve-1", Kind: budget.KindReserve, ReservationID: attempt, ReservedTicks: 50}
	_, receipts, err := budget.Reduce([]budget.Entry{reserve})
	if err != nil {
		t.Fatalf("reduce fixture entry: %v", err)
	}
	entryCols := issueBudgetEntryCols(t, root, 1, reserve, receipts[0])
	insertIssueBudgetEntry(t, entryCols)
	operationCols := testutil.Cols{
		"id": uuid.NewString(), "workspace_id": testWorkspaceID, "budget_root_id": root,
		"scope_kind": "root", "scope_id": root, "operation_key": "key-1", "operation_kind": "adjust_limit",
		"intent_digest": strings.Repeat("ab", 32), "actor_type": "member", "actor_id": testUserID,
		"result": `{"policy_revision":"2"}`,
	}
	operation := dbfx.Insert(t, "issue_budget_operation", operationCols)

	// V17: one attempt may produce any number of observations.
	observationCols := func() testutil.Cols {
		return testutil.Cols{
			"id": uuid.NewString(), "workspace_id": testWorkspaceID, "budget_root_id": root,
			"kind": "attribution_difference", "reason": "created_outside_context_parent",
			"attempt_id": attempt, "task_id": task, "claim_generation": 1, "issue_id": uuid.NewString(),
		}
	}
	dbfx.Insert(t, "issue_budget_observation", observationCols())
	dbfx.Insert(t, "issue_budget_observation", observationCols())
	if got := dbfx.Count(t, `SELECT count(*) FROM issue_budget_observation WHERE attempt_id = $1`, attempt); got != 2 {
		t.Errorf("observations for one attempt = %d, want 2", got)
	}

	// Each hold reason of the metering package is an attempt state.
	for i, reason := range []budget.HoldReason{
		budget.HoldCancelled, budget.HoldTimeout, budget.HoldProcessExit, budget.HoldLostContact, budget.HoldSupersededUnproven,
	} {
		cols := issueBudgetAttemptCols(root, task, memberIssue)
		cols["claim_generation"] = 10 + i
		cols["state"] = string(reason)
		dbfx.Insert(t, "issue_budget_attempt", cols)
	}

	other := func(cols testutil.Cols, over testutil.Cols) testutil.Cols {
		out := testutil.Cols{}
		for k, v := range cols {
			out[k] = v
		}
		for k, v := range over {
			if v == nil {
				delete(out, k)
				continue
			}
			out[k] = v
		}
		return out
	}
	tampered := append([]byte(nil), entryCols["entry_json"].([]byte)...)
	tampered[len(tampered)-2] ^= 1

	// A usage entry as a daemon reports it, with every envelope column set.
	usage := budget.Entry{ID: "usage-1", Kind: budget.KindUsage, ReservationID: attempt, Usage: budget.Usage{
		ScopeID: "request-1", InvocationID: "invocation-1", Mode: budget.ModeDelta,
		Cost: budget.Cost{Basis: budget.CostReported, Ticks: 30, Currency: budget.CurrencyUSD},
	}}
	_, usageReceipts, err := budget.Reduce([]budget.Entry{reserve, usage})
	if err != nil {
		t.Fatalf("reduce fixture usage: %v", err)
	}
	usageCols := other(issueBudgetEntryCols(t, root, 2, usage, usageReceipts[1]), testutil.Cols{
		"source": "daemon", "schema_version": 1, "adapter_version": "claude-code/1.2.3", "normalizer_version": "1",
		"producer_epoch": "epoch-1", "payload_digest": strings.Repeat("ef", 32),
		"observed_at": testutil.Raw("now() - interval '2 seconds'"), "reported_at": testutil.Raw("now() - interval '1 second'"),
		"received_at": testutil.Raw("now()"), "completeness": "final",
		"operation_id": operation,
	})
	longest := strings.Repeat("k", issueBudgetIdentifierBytes)
	tooLong := longest + "k"
	// 101 characters, 202 bytes: the limit counts bytes.
	wide := strings.Repeat("\u00e9", 101)
	// A second operation row; the key and the id are separate identities.
	nextOperation := func(over testutil.Cols) testutil.Cols {
		cols := other(operationCols, testutil.Cols{"id": uuid.NewString(), "operation_key": "key-2"})
		return other(cols, over)
	}
	nextAttempt := func(over testutil.Cols) testutil.Cols {
		return other(other(attemptCols, testutil.Cols{"id": uuid.NewString(), "claim_generation": 3}), over)
	}
	nextRoot := func(over testutil.Cols) testutil.Cols {
		return other(other(rootCols, testutil.Cols{"id": uuid.NewString(), "root_issue_id": uuid.NewString()}), over)
	}

	// An estimated usage entry with the snapshot it was priced with. The
	// entry's own cost cites the digest the row carries.
	priceRaw, priceDigest := issueBudgetPriceSnapshot(t)
	estimated := budget.Entry{ID: "usage-2", Kind: budget.KindUsage, ReservationID: attempt, Usage: budget.Usage{
		ScopeID: "request-2", InvocationID: "invocation-1", Mode: budget.ModeDelta,
		Cost: budget.Cost{Basis: budget.CostEstimated, Ticks: 12, Currency: budget.CurrencyUSD, PriceVersion: "2026-11-01", PriceDigest: priceDigest},
	}}
	_, estimatedReceipts, err := budget.Reduce([]budget.Entry{reserve, usage, estimated})
	if err != nil {
		t.Fatalf("reduce fixture estimate: %v", err)
	}
	pricedCols := other(issueBudgetEntryCols(t, root, 3, estimated, estimatedReceipts[2]), testutil.Cols{
		"price_snapshot": priceRaw, "price_digest": priceDigest,
	})
	for _, c := range []struct {
		name, table string
		cols        testutil.Cols
	}{
		{"usage entry with a full envelope", "issue_budget_entry", usageCols},
		{"entry id at the byte limit", "issue_budget_entry", other(usageCols, testutil.Cols{"entry_id": longest})},
		{"scope and invocation at the byte limit", "issue_budget_entry", other(usageCols, testutil.Cols{"scope_id": longest, "invocation_id": longest})},
		{"operation key at the byte limit", "issue_budget_operation", nextOperation(testutil.Cols{"operation_key": longest})},
		{"attempt judged under a policy", "issue_budget_attempt", nextAttempt(testutil.Cols{"policy_revision": 2, "policy_epoch": 1})},
		{"attempt without a recorded policy", "issue_budget_attempt", nextAttempt(nil)},
		// Design 10.1: no declared reserve means an unsized reservation, which
		// has no amount; a sized reservation of nothing is another fact.
		{"attempt with an unsized reservation", "issue_budget_attempt", nextAttempt(testutil.Cols{"reserved_ticks": nil})},
		{"attempt with a sized reservation of zero", "issue_budget_attempt", nextAttempt(testutil.Cols{"reserved_ticks": 0})},
		{"root with a declared reserve", "issue_budget_root", nextRoot(testutil.Cols{"attempt_reserve_ticks": 1})},
		{"root with a limit and a declared reserve", "issue_budget_root", nextRoot(testutil.Cols{"limit_ticks": 0, "attempt_reserve_ticks": 10})},
		{"estimated usage entry with its price snapshot", "issue_budget_entry", pricedCols},
		{"observation without a claim generation", "issue_budget_observation", other(observationCols(), testutil.Cols{"claim_generation": nil})},
		{"observation at claim generation zero", "issue_budget_observation", other(observationCols(), testutil.Cols{"claim_generation": 0})},
	} {
		if err := issueBudgetInsertError(c.table, c.cols); err != nil {
			t.Errorf("%s: insert into %s returned %v, want it accepted", c.name, c.table, err)
		}
	}

	// A stored form outside the size limits, with the digest of those very
	// bytes, so the size rule is the only one it breaks. The domain is written
	// out here: taking it from the metering package would compare the schema
	// with the code it constrains.
	sized := func(form []byte) testutil.Cols {
		sum := sha256.Sum256(append([]byte("budget-entry-v1\n"), form...))
		return testutil.Cols{"seq": 2, "entry_id": "reserve-2", "entry_json": form, "entry_digest": hex.EncodeToString(sum[:])}
	}

	// The same for a price snapshot, under its own digest domain.
	sizedPrice := func(form []byte) testutil.Cols {
		sum := sha256.Sum256(append([]byte("budget-price-snapshot-v1\n"), form...))
		return testutil.Cols{"price_snapshot": form, "price_digest": hex.EncodeToString(sum[:])}
	}

	const unique, check, notNull = "23505", "23514", "23502"
	for _, c := range []struct {
		name, code, table string
		cols              testutil.Cols
	}{
		{"second root for one issue", unique, "issue_budget_root", other(rootCols, testutil.Cols{"id": uuid.NewString()})},
		{"reused root id", unique, "issue_budget_root", other(rootCols, testutil.Cols{"root_issue_id": uuid.NewString()})},
		{"issue bound to a second root", unique, "issue_budget_member", issueBudgetMemberCols(uuid.NewString(), memberIssue, rootIssue)},
		{"second attempt for one claim", unique, "issue_budget_attempt", other(attemptCols, testutil.Cols{"id": uuid.NewString()})},
		{"reused attempt id", unique, "issue_budget_attempt", other(attemptCols, testutil.Cols{"claim_generation": 2})},
		{"reused ledger sequence", unique, "issue_budget_entry", other(entryCols, testutil.Cols{"entry_id": "reserve-2"})},
		{"reused entry identity", unique, "issue_budget_entry", other(entryCols, testutil.Cols{"seq": 2})},
		{"reused operation key", unique, "issue_budget_operation", other(operationCols, testutil.Cols{"id": uuid.NewString(), "intent_digest": strings.Repeat("cd", 32)})},
		{"reused operation id", unique, "issue_budget_operation", other(operationCols, testutil.Cols{"operation_key": "key-2"})},
		{"operation without an id", notNull, "issue_budget_operation", nextOperation(testutil.Cols{"id": nil})},

		{"root without workspace", notNull, "issue_budget_root", other(rootCols, testutil.Cols{"id": uuid.NewString(), "root_issue_id": uuid.NewString(), "workspace_id": nil})},
		{"negative actual", check, "issue_budget_root", other(rootCols, testutil.Cols{"id": uuid.NewString(), "root_issue_id": uuid.NewString(), "actual_known_ticks": -1})},
		{"negative estimate", check, "issue_budget_root", other(rootCols, testutil.Cols{"id": uuid.NewString(), "root_issue_id": uuid.NewString(), "estimated_ticks": -1})},
		{"negative reservation on the root", check, "issue_budget_root", other(rootCols, testutil.Cols{"id": uuid.NewString(), "root_issue_id": uuid.NewString(), "reserved_ticks": -1})},
		{"negative unknown hold", check, "issue_budget_root", other(rootCols, testutil.Cols{"id": uuid.NewString(), "root_issue_id": uuid.NewString(), "unknown_hold_ticks": -1})},
		{"negative overrun", check, "issue_budget_root", other(rootCols, testutil.Cols{"id": uuid.NewString(), "root_issue_id": uuid.NewString(), "overrun_ticks": -1})},
		{"negative limit", check, "issue_budget_root", other(rootCols, testutil.Cols{"id": uuid.NewString(), "root_issue_id": uuid.NewString(), "limit_ticks": -1})},
		// NULL is "no reserve declared"; a declared reserve is a positive amount.
		{"declared reserve of zero", check, "issue_budget_root", nextRoot(testutil.Cols{"attempt_reserve_ticks": 0})},
		{"negative declared reserve", check, "issue_budget_root", nextRoot(testutil.Cols{"attempt_reserve_ticks": -1})},
		// The first slice observes only: the schema cannot record enforcement.
		{"enforce mode", check, "issue_budget_root", other(rootCols, testutil.Cols{"id": uuid.NewString(), "root_issue_id": uuid.NewString(), "mode": "enforce"})},
		// tombstoned is derived from the root issue's member row, never stored
		// as an admission state.
		{"tombstoned admission state", check, "issue_budget_root", other(rootCols, testutil.Cols{"id": uuid.NewString(), "root_issue_id": uuid.NewString(), "admission_state": "tombstoned"})},
		{"settled beyond the ledger", check, "issue_budget_root", other(rootCols, testutil.Cols{"id": uuid.NewString(), "root_issue_id": uuid.NewString(), "settled_as_of_revision": 1})},
		{"negative ledger revision", check, "issue_budget_root", other(rootCols, testutil.Cols{"id": uuid.NewString(), "root_issue_id": uuid.NewString(), "ledger_revision": -1})},
		{"policy revision zero", check, "issue_budget_root", other(rootCols, testutil.Cols{"id": uuid.NewString(), "root_issue_id": uuid.NewString(), "policy_revision": 0})},
		{"policy epoch zero", check, "issue_budget_root", other(rootCols, testutil.Cols{"id": uuid.NewString(), "root_issue_id": uuid.NewString(), "policy_epoch": 0})},
		{"negative unknown count", check, "issue_budget_root", other(rootCols, testutil.Cols{"id": uuid.NewString(), "root_issue_id": uuid.NewString(), "unknown_count": -1})},

		{"root member with a parent", check, "issue_budget_member", other(issueBudgetMemberCols(root, uuid.NewString(), rootIssue), testutil.Cols{"bind_source": "root"})},
		{"inherited member without a parent", check, "issue_budget_member", other(issueBudgetMemberCols(root, uuid.NewString(), ""), testutil.Cols{"bind_source": "create"})},
		{"unknown bind source", check, "issue_budget_member", other(issueBudgetMemberCols(root, uuid.NewString(), rootIssue), testutil.Cols{"bind_source": "merge"})},
		{"binding version zero", check, "issue_budget_member", other(issueBudgetMemberCols(root, uuid.NewString(), rootIssue), testutil.Cols{"binding_version": 0})},

		{"unknown attempt state", check, "issue_budget_attempt", other(attemptCols, testutil.Cols{"id": uuid.NewString(), "claim_generation": 3, "state": "released"})},
		{"negative reservation", check, "issue_budget_attempt", other(attemptCols, testutil.Cols{"id": uuid.NewString(), "claim_generation": 3, "reserved_ticks": -1})},
		{"attempt with neither issue nor context parent", check, "issue_budget_attempt", other(attemptCols, testutil.Cols{"id": uuid.NewString(), "claim_generation": 3, "issue_id": nil})},
		{"negative claim generation", check, "issue_budget_attempt", other(attemptCols, testutil.Cols{"id": uuid.NewString(), "claim_generation": -1})},
		{"protocol version zero", check, "issue_budget_attempt", other(attemptCols, testutil.Cols{"id": uuid.NewString(), "claim_generation": 3, "protocol_version": 0})},
		{"attempt policy revision zero", check, "issue_budget_attempt", nextAttempt(testutil.Cols{"policy_revision": 0, "policy_epoch": 1})},
		{"attempt policy epoch zero", check, "issue_budget_attempt", nextAttempt(testutil.Cols{"policy_revision": 1, "policy_epoch": 0})},
		{"attempt policy revision without its epoch", check, "issue_budget_attempt", nextAttempt(testutil.Cols{"policy_revision": 1})},
		{"attempt policy epoch without its revision", check, "issue_budget_attempt", nextAttempt(testutil.Cols{"policy_epoch": 1})},

		{"stored form that does not match its digest", check, "issue_budget_entry", other(entryCols, testutil.Cols{"seq": 2, "entry_id": "reserve-2", "entry_json": tampered})},
		{"unknown entry kind", check, "issue_budget_entry", other(entryCols, testutil.Cols{"seq": 2, "entry_id": "reserve-2", "kind": "refund"})},
		{"reservation entry without an attempt", check, "issue_budget_entry", other(entryCols, testutil.Cols{"seq": 2, "entry_id": "reserve-2", "attempt_id": nil})},
		{"usage entry without scope and invocation", check, "issue_budget_entry", other(entryCols, testutil.Cols{"seq": 2, "entry_id": "usage-1", "kind": "usage"})},
		{"sequence zero", check, "issue_budget_entry", other(entryCols, testutil.Cols{"seq": 0, "entry_id": "reserve-2"})},
		// Scope and invocation are separate rules, each in both directions.
		{"usage entry without a scope", check, "issue_budget_entry", other(usageCols, testutil.Cols{"scope_id": nil})},
		{"usage entry without an invocation", check, "issue_budget_entry", other(usageCols, testutil.Cols{"invocation_id": nil})},
		{"reservation entry with a scope", check, "issue_budget_entry", other(entryCols, testutil.Cols{"seq": 2, "entry_id": "reserve-2", "scope_id": "request-1"})},
		{"reservation entry with an invocation", check, "issue_budget_entry", other(entryCols, testutil.Cols{"seq": 2, "entry_id": "reserve-2", "invocation_id": "invocation-1"})},
		{"stored form shorter than an object", check, "issue_budget_entry", other(entryCols, sized([]byte("1")))},
		{"stored form over 64 KiB", check, "issue_budget_entry", other(entryCols, sized([]byte(strings.Repeat("x", 65537))))},
		{"receipt that is not an object", check, "issue_budget_entry", other(entryCols, testutil.Cols{"seq": 2, "entry_id": "reserve-2", "receipt": `[]`})},

		{"entry without a source", notNull, "issue_budget_entry", other(usageCols, testutil.Cols{"source": nil})},
		{"entry without a schema version", notNull, "issue_budget_entry", other(usageCols, testutil.Cols{"schema_version": nil})},
		{"malformed source", check, "issue_budget_entry", other(usageCols, testutil.Cols{"source": "Daemon Adapter"})},
		{"schema version zero", check, "issue_budget_entry", other(usageCols, testutil.Cols{"schema_version": 0})},
		{"malformed payload digest", check, "issue_budget_entry", other(usageCols, testutil.Cols{"payload_digest": "abc"})},
		{"unknown completeness", check, "issue_budget_entry", other(usageCols, testutil.Cols{"completeness": "done"})},
		{"entry id over the byte limit", check, "issue_budget_entry", other(usageCols, testutil.Cols{"entry_id": tooLong})},
		{"entry id over the limit in bytes only", check, "issue_budget_entry", other(usageCols, testutil.Cols{"entry_id": wide})},
		{"empty entry id", check, "issue_budget_entry", other(usageCols, testutil.Cols{"entry_id": ""})},
		{"scope id over the byte limit", check, "issue_budget_entry", other(usageCols, testutil.Cols{"scope_id": tooLong})},
		{"invocation id over the byte limit", check, "issue_budget_entry", other(usageCols, testutil.Cols{"invocation_id": tooLong})},
		{"adapter version over the byte limit", check, "issue_budget_entry", other(usageCols, testutil.Cols{"adapter_version": tooLong})},
		{"normalizer version over the byte limit", check, "issue_budget_entry", other(usageCols, testutil.Cols{"normalizer_version": tooLong})},
		{"producer epoch over the byte limit", check, "issue_budget_entry", other(usageCols, testutil.Cols{"producer_epoch": tooLong})},
		{"operation key over the byte limit", check, "issue_budget_operation", nextOperation(testutil.Cols{"operation_key": tooLong})},
		{"operation key over the limit in bytes only", check, "issue_budget_operation", nextOperation(testutil.Cols{"operation_key": wide})},

		{"unknown operation scope", check, "issue_budget_operation", nextOperation(testutil.Cols{"scope_kind": "workspace"})},
		{"malformed intent digest", check, "issue_budget_operation", nextOperation(testutil.Cols{"intent_digest": "abc"})},
		{"unknown actor type", check, "issue_budget_operation", nextOperation(testutil.Cols{"actor_type": "agent"})},
		{"malformed operation kind", check, "issue_budget_operation", nextOperation(testutil.Cols{"operation_kind": "Adjust Limit"})},
		{"operation result that is not an object", check, "issue_budget_operation", nextOperation(testutil.Cols{"result": `[]`})},
		{"unknown observation kind", check, "issue_budget_observation", other(observationCols(), testutil.Cols{"kind": "refusal"})},
		{"malformed observation reason", check, "issue_budget_observation", other(observationCols(), testutil.Cols{"reason": "Root Conflict"})},
		{"observation detail that is not an object", check, "issue_budget_observation", other(observationCols(), testutil.Cols{"detail": `[]`})},
		{"observation with a negative claim generation", check, "issue_budget_observation", other(observationCols(), testutil.Cols{"claim_generation": -1})},
	} {
		if err := issueBudgetInsertError(c.table, c.cols); !issueBudgetIsPGCode(err, c.code) {
			t.Errorf("%s: insert into %s returned %v, want SQLSTATE %s", c.name, c.table, err, c.code)
		}
	}

	// The price snapshot rules of design 10.8, each refused by its own
	// constraint. Every case breaks one rule only, and the constraint that
	// refused it is named, so a rule that moved into another constraint or was
	// dropped cannot hide behind a neighbour.
	const sizeCheck = "issue_budget_entry_price_snapshot_check"
	for _, c := range []struct {
		name, constraint string
		cols             testutil.Cols
	}{
		{"price snapshot without its digest", "issue_budget_entry_price_pair_check", other(pricedCols, testutil.Cols{"price_digest": nil})},
		{"price digest without its snapshot", "issue_budget_entry_price_pair_check", other(pricedCols, testutil.Cols{"price_snapshot": nil})},
		{"price digest of another snapshot", "issue_budget_entry_price_digest_check", other(pricedCols, testutil.Cols{"price_digest": strings.Repeat("0", 64)})},
		{"price digest under the entry domain", "issue_budget_entry_price_digest_check", other(pricedCols, testutil.Cols{"price_digest": sized(priceRaw)["entry_digest"]})},
		// A reservation with a whole, well-formed pair: only its kind is wrong.
		{"price snapshot on a reservation entry", "issue_budget_entry_price_usage_check", other(entryCols, testutil.Cols{
			"seq": 2, "entry_id": "reserve-2", "price_snapshot": priceRaw, "price_digest": priceDigest})},
		{"price snapshot shorter than an object", sizeCheck, other(pricedCols, sizedPrice([]byte("1")))},
		{"price snapshot over 64 KiB", sizeCheck, other(pricedCols, sizedPrice([]byte(strings.Repeat("x", 65537))))},
	} {
		err := issueBudgetInsertError("issue_budget_entry", c.cols)
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != check || pgErr.ConstraintName != c.constraint {
			t.Errorf("%s: insert returned %v, want SQLSTATE %s from %s", c.name, err, check, c.constraint)
		}
	}
	// The size limits are inclusive: the smallest and the largest snapshot the
	// column takes, each with the digest of its own bytes.
	for _, form := range [][]byte{[]byte("{}"), []byte(strings.Repeat("x", 65536))} {
		if err := issueBudgetInsertError("issue_budget_entry", other(pricedCols, sizedPrice(form))); err != nil {
			t.Errorf("price snapshot of %d bytes: insert returned %v, want it accepted", len(form), err)
		}
	}
}

// ---- fixtures -------------------------------------------------------------

func issueBudgetRootCols(rootIssueID string) testutil.Cols {
	return testutil.Cols{
		"id": uuid.NewString(), "workspace_id": testWorkspaceID, "root_issue_id": rootIssueID,
		"activated_at": testutil.Raw("now()"),
	}
}

// issueBudgetMemberCols binds issueID to rootID. An empty parentIssueID is the
// root issue's own row.
func issueBudgetMemberCols(rootID, issueID, parentIssueID string) testutil.Cols {
	cols := testutil.Cols{
		"workspace_id": testWorkspaceID, "issue_id": issueID, "budget_root_id": rootID, "bind_source": "root",
	}
	if parentIssueID != "" {
		cols["bind_source"] = "create"
		cols["bound_parent_issue_id"] = parentIssueID
	}
	return cols
}

func insertIssueBudgetMember(t *testing.T, cols testutil.Cols) {
	t.Helper()
	dbfx.InsertNoID(t, "issue_budget_member", cols, "workspace_id = $1 AND issue_id = $2", cols["workspace_id"], cols["issue_id"])
}

func issueBudgetAttemptCols(rootID, taskID, issueID string) testutil.Cols {
	return testutil.Cols{
		"id": uuid.NewString(), "workspace_id": testWorkspaceID, "budget_root_id": rootID,
		"task_id": taskID, "claim_generation": 1, "runtime_id": uuid.NewString(), "agent_id": uuid.NewString(),
		"issue_id": issueID, "reserved_ticks": 50, "protocol_version": 1,
	}
}

// issueBudgetPriceSnapshot is a price snapshot as an estimated usage entry
// freezes it: the bytes the metering package marshals and its digest of them.
func issueBudgetPriceSnapshot(t *testing.T) (raw []byte, digest string) {
	t.Helper()
	rate, err := budget.USDRate(3, 1, 1_000_000)
	if err != nil {
		t.Fatalf("fixture rate: %v", err)
	}
	snapshot := budget.PriceSnapshot{
		Provider: "fixture", Model: "fixture-model", NativeModel: "fixture-model", AdapterVersion: "1",
		NormalizerVersion: "1", PriceVersion: "2026-11-01", Source: "fixture", EffectiveFrom: "2026-11-01T00:00:00Z",
		Currency: budget.CurrencyUSD, RoundingVersion: "half-up-v1", Input: rate, Output: rate, CacheRead: rate, CacheWrite: rate,
	}
	raw, err = json.Marshal(snapshot)
	if err != nil {
		t.Fatalf("marshal fixture snapshot: %v", err)
	}
	digest, err = snapshot.Digest()
	if err != nil {
		t.Fatalf("digest fixture snapshot: %v", err)
	}
	return raw, digest
}

// issueBudgetEntryCols stores entry the way the ledger writer will: the bytes
// the metering package marshals, their digest, and the committed receipt.
func issueBudgetEntryCols(t *testing.T, rootID string, seq int, entry budget.Entry, receipt budget.Receipt) testutil.Cols {
	t.Helper()
	raw, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("marshal entry: %v", err)
	}
	digest, err := entry.Digest()
	if err != nil {
		t.Fatalf("digest entry: %v", err)
	}
	receiptJSON, err := json.Marshal(receipt)
	if err != nil {
		t.Fatalf("marshal receipt: %v", err)
	}
	cols := testutil.Cols{
		"workspace_id": testWorkspaceID, "budget_root_id": rootID, "seq": seq, "entry_id": entry.ID,
		"kind": string(entry.Kind), "entry_json": raw, "entry_digest": digest, "receipt": string(receiptJSON),
		// Written by the server itself; the reporting columns stay empty.
		"source": "server", "schema_version": 1,
	}
	if entry.ReservationID != "" {
		cols["attempt_id"] = entry.ReservationID
	}
	if entry.Kind == budget.KindUsage {
		cols["scope_id"] = entry.Usage.ScopeID
		cols["invocation_id"] = entry.Usage.InvocationID
	}
	return cols
}

func insertIssueBudgetEntry(t *testing.T, cols testutil.Cols) {
	t.Helper()
	dbfx.InsertNoID(t, "issue_budget_entry", cols, "budget_root_id = $1 AND seq = $2", cols["budget_root_id"], cols["seq"])
}

// issueBudgetInsertError runs an insert that is expected to be refused and
// returns its error. It rolls back, so an insert that wrongly succeeds leaves
// nothing behind.
func issueBudgetInsertError(table string, cols testutil.Cols) error {
	ctx := context.Background()
	names := make([]string, 0, len(cols))
	for name := range cols {
		names = append(names, name)
	}
	sort.Strings(names)
	values := make([]string, 0, len(names))
	args := make([]any, 0, len(names))
	for _, name := range names {
		if raw, ok := cols[name].(testutil.Raw); ok {
			values = append(values, string(raw))
			continue
		}
		args = append(args, cols[name])
		values = append(values, fmt.Sprintf("$%d", len(args)))
	}
	tx, err := testPool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	_, err = tx.Exec(ctx, "INSERT INTO "+table+" ("+strings.Join(names, ", ")+") VALUES ("+strings.Join(values, ", ")+")", args...)
	return err
}

func issueBudgetIsPGCode(err error, code string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == code
}

// issueBudgetUpMigrations returns every up migration that mentions a budget
// object, by file name.
func issueBudgetUpMigrations(t *testing.T) map[string]string {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join("..", "..", "migrations", "*.up.sql"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("glob migrations: %d files, %v", len(paths), err)
	}
	out := map[string]string{}
	for _, path := range paths {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if strings.Contains(string(body), "issue_budget_") {
			out[filepath.Base(path)] = string(body)
		}
	}
	return out
}
