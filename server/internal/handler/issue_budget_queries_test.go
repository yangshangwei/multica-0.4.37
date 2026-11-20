package handler

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/multica-ai/multica/server/internal/budget"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// The (budget_root_id, seq) key can serve a root's ledger already ordered:
// with whole-table and bitmap scans ruled out, the plan of the sqlc statement
// itself walks that index and sorts nothing. On a large table the planner is
// free to fetch the root's rows through the same leading column and sort them.
func TestIssueBudgetLedgerLoadIsOrderedByItsKey(t *testing.T) {
	ctx := context.Background()
	source, err := os.ReadFile(filepath.Join("..", "..", "pkg", "db", "queries", "issue_budget.sql"))
	if err != nil {
		t.Fatalf("read budget queries: %v", err)
	}
	_, statement, found := strings.Cut(string(source), "-- name: ListIssueBudgetEntries :many\n")
	if !found {
		t.Fatal("pkg/db/queries/issue_budget.sql has no ListIssueBudgetEntries query")
	}
	// The statement runs to the next query; its comment lines may hold a
	// semicolon of their own.
	statement, _, _ = strings.Cut(statement, "\n-- name:")
	var lines []string
	for _, line := range strings.Split(statement, "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), "--") {
			lines = append(lines, line)
		}
	}
	statement = strings.TrimSuffix(strings.TrimSpace(strings.Join(lines, "\n")), ";")
	if !strings.Contains(statement, "@workspace_id") || !strings.Contains(statement, "@budget_root_id") {
		t.Fatalf("ledger load is not scoped by workspace and root:\n%s", statement)
	}
	id := "'" + uuid.NewString() + "'::uuid"
	statement = strings.NewReplacer("@workspace_id", id, "@budget_root_id", id).Replace(statement)

	tx, err := testPool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	// The question is whether the key can serve the order, not which plan
	// is cheapest for the rows this database happens to hold.
	if _, err := tx.Exec(ctx, `SET LOCAL enable_seqscan = off`); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `SET LOCAL enable_bitmapscan = off`); err != nil {
		t.Fatal(err)
	}
	rows, err := tx.Query(ctx, "EXPLAIN (COSTS OFF) "+statement)
	if err != nil {
		t.Fatalf("explain ledger load: %v", err)
	}
	defer rows.Close()
	var plan []string
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			t.Fatalf("scan plan: %v", err)
		}
		plan = append(plan, line)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("read plan: %v", err)
	}
	got := strings.Join(plan, "\n")
	if !strings.Contains(got, "Index Scan using issue_budget_entry_seq_key on issue_budget_entry") || strings.Contains(got, "Sort") {
		t.Fatalf("ledger load is not an ordered scan of issue_budget_entry_seq_key:\n%s", got)
	}
}

// The generated budget queries against the migrated schema: each row kind can
// be inserted, the root and member lookups are scoped by workspace, and the
// root lock is a real row lock.
func TestIssueBudgetQueriesRoundTrip(t *testing.T) {
	ctx := context.Background()
	q := testHandler.Queries
	workspace := parseUUID(testWorkspaceID)
	newID := func() pgtype.UUID { return parseUUID(uuid.NewString()) }
	rootID, rootIssue, memberIssue, attemptID, task, operationID := newID(), newID(), newID(), newID(), newID(), newID()
	bareRootID := newID()
	for _, table := range issueBudgetTables {
		for _, id := range []pgtype.UUID{rootID, bareRootID} {
			dbfx.Cleanup(t, `DELETE FROM `+table+` WHERE workspace_id = $1 AND `+
				map[bool]string{true: "id", false: "budget_root_id"}[table == "issue_budget_root"]+` = $2`, workspace, id)
		}
	}

	root, err := q.InsertIssueBudgetRoot(ctx, db.InsertIssueBudgetRootParams{
		ID: rootID, WorkspaceID: workspace, RootIssueID: rootIssue,
		LimitTicks:          pgtype.Int8{Int64: 100, Valid: true},
		AttemptReserveTicks: pgtype.Int8{Int64: 10, Valid: true},
		ActivatedAt:         pgtype.Timestamptz{Time: time.Now(), Valid: true},
	})
	if err != nil {
		t.Fatalf("insert root: %v", err)
	}
	if root.ID != rootID || root.AdmissionState != "open" || root.Mode != "observe" || root.PolicyRevision != 1 ||
		root.PolicyEpoch != 1 || root.LedgerRevision != 0 || root.ActualKnownTicks != 0 || root.ReservedTicks != 0 ||
		root.SettledAsOfRevision.Valid || root.LimitTicks.Int64 != 100 || root.AttemptReserveTicks != (pgtype.Int8{Int64: 10, Valid: true}) {
		t.Fatalf("new root = %+v", root)
	}
	// A root enabled with neither value has neither: both columns are NULL,
	// not zero.
	bare, err := q.InsertIssueBudgetRoot(ctx, db.InsertIssueBudgetRootParams{
		ID: bareRootID, WorkspaceID: workspace, RootIssueID: newID(),
		ActivatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
	})
	if err != nil || bare.LimitTicks.Valid || bare.AttemptReserveTicks.Valid {
		t.Fatalf("root without a limit or a declared reserve = %+v, %v", bare, err)
	}
	// The policy update replaces both values as a whole and counts one
	// revision; it leaves the amounts alone.
	changed, err := q.UpdateIssueBudgetRootPolicy(ctx, db.UpdateIssueBudgetRootPolicyParams{
		AdmissionState: bare.AdmissionState, Mode: bare.Mode, PolicyEpoch: bare.PolicyEpoch,
		AttemptReserveTicks: pgtype.Int8{Int64: 7, Valid: true},
		WorkspaceID:         workspace, ID: bareRootID,
	})
	if err != nil || changed.LimitTicks.Valid || changed.AttemptReserveTicks != (pgtype.Int8{Int64: 7, Valid: true}) ||
		changed.PolicyRevision != bare.PolicyRevision+1 || changed.LedgerRevision != 0 || changed.ReservedTicks != 0 {
		t.Fatalf("root after a declared reserve was set = %+v, %v", changed, err)
	}
	found, err := q.GetIssueBudgetRootByIssue(ctx, db.GetIssueBudgetRootByIssueParams{WorkspaceID: workspace, RootIssueID: rootIssue})
	if err != nil || found.ID != rootID {
		t.Fatalf("get root by issue = %v, %v", found.ID, err)
	}
	if _, err := q.GetIssueBudgetRootByIssue(ctx, db.GetIssueBudgetRootByIssueParams{WorkspaceID: newID(), RootIssueID: rootIssue}); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("root read through another workspace: %v, want no rows", err)
	}

	for _, m := range []db.InsertIssueBudgetMemberParams{
		{WorkspaceID: workspace, IssueID: rootIssue, BudgetRootID: rootID, BindSource: "root", BindingVersion: 1},
		{WorkspaceID: workspace, IssueID: memberIssue, BudgetRootID: rootID, BoundParentIssueID: rootIssue, BindSource: "create", BindingVersion: 1},
	} {
		if _, err := q.InsertIssueBudgetMember(ctx, m); err != nil {
			t.Fatalf("insert %s member: %v", m.BindSource, err)
		}
	}
	member, err := q.GetIssueBudgetMember(ctx, db.GetIssueBudgetMemberParams{WorkspaceID: workspace, IssueID: memberIssue})
	if err != nil || member.BudgetRootID != rootID || member.BoundParentIssueID != rootIssue || member.TombstonedAt.Valid {
		t.Fatalf("get member = %+v, %v", member, err)
	}
	if _, err := q.GetIssueBudgetMember(ctx, db.GetIssueBudgetMemberParams{WorkspaceID: workspace, IssueID: newID()}); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("member of an unbound issue: %v, want no rows", err)
	}

	attempt, err := q.InsertIssueBudgetAttempt(ctx, db.InsertIssueBudgetAttemptParams{
		ID: attemptID, WorkspaceID: workspace, BudgetRootID: rootID, TaskID: task, ClaimGeneration: 1,
		RuntimeID: newID(), AgentID: newID(), IssueID: memberIssue, ReservedTicks: pgtype.Int8{Int64: 50, Valid: true},
		PolicyRevision: pgtype.Int8{Int64: root.PolicyRevision, Valid: true}, PolicyEpoch: pgtype.Int8{Int64: root.PolicyEpoch, Valid: true},
	})
	if err != nil || attempt.State != "open" || attempt.ReservedTicks != (pgtype.Int8{Int64: 50, Valid: true}) || attempt.ProtocolVersion.Valid ||
		attempt.PolicyRevision.Int64 != 1 || attempt.PolicyEpoch.Int64 != 1 {
		t.Fatalf("insert attempt = %+v, %v", attempt, err)
	}
	// The next claim of the same task, on a root that declared no reserve: the
	// reservation is unsized and the amount is NULL, not zero.
	unsized, err := q.InsertIssueBudgetAttempt(ctx, db.InsertIssueBudgetAttemptParams{
		ID: newID(), WorkspaceID: workspace, BudgetRootID: rootID, TaskID: task, ClaimGeneration: 2,
		RuntimeID: attempt.RuntimeID, AgentID: attempt.AgentID, IssueID: memberIssue,
		ProtocolVersion: pgtype.Int4{Int32: 1, Valid: true},
	})
	if err != nil || unsized.State != "open" || unsized.ReservedTicks.Valid || unsized.ProtocolVersion.Int32 != 1 {
		t.Fatalf("insert unsized attempt = %+v, %v", unsized, err)
	}

	entry := budget.Entry{ID: "reserve-1", Kind: budget.KindReserve, ReservationID: uuid.UUID(attemptID.Bytes).String(), ReservedTicks: 50}
	_, receipts, err := budget.Reduce([]budget.Entry{entry})
	if err != nil {
		t.Fatalf("reduce entry: %v", err)
	}
	usage := budget.Entry{ID: "usage-1", Kind: budget.KindUsage, ReservationID: entry.ReservationID, Usage: budget.Usage{
		ScopeID: "request-1", InvocationID: "invocation-1", Mode: budget.ModeDelta,
		Cost: budget.Cost{Basis: budget.CostReported, Ticks: 30, Currency: budget.CurrencyUSD},
	}}
	_, usageReceipts, err := budget.Reduce([]budget.Entry{entry, usage})
	if err != nil {
		t.Fatalf("reduce usage: %v", err)
	}
	raw, _ := json.Marshal(entry)
	digest, _ := entry.Digest()
	receipt, _ := json.Marshal(receipts[0])
	usageRaw, _ := json.Marshal(usage)
	usageDigest, _ := usage.Digest()
	usageReceipt, _ := json.Marshal(usageReceipts[1])
	received := time.Now().UTC().Truncate(time.Microsecond)
	text := func(s string) pgtype.Text { return pgtype.Text{String: s, Valid: true} }
	at := func(d time.Duration) pgtype.Timestamptz {
		return pgtype.Timestamptz{Time: received.Add(d), Valid: true}
	}
	// The reported entry is written first with the higher seq, so the list
	// below is ordered by seq and not by insertion.
	reported, err := q.InsertIssueBudgetEntry(ctx, db.InsertIssueBudgetEntryParams{
		WorkspaceID: workspace, BudgetRootID: rootID, Seq: 2, EntryID: usage.ID, Kind: string(usage.Kind),
		AttemptID: attemptID, InvocationID: text(usage.Usage.InvocationID), ScopeID: text(usage.Usage.ScopeID),
		EntryJson: usageRaw, EntryDigest: usageDigest, Receipt: usageReceipt,
		Source: "daemon", SchemaVersion: 1, AdapterVersion: text("claude-code/1.2.3"), NormalizerVersion: text("1"),
		ProducerEpoch: text("epoch-1"), PayloadDigest: text(strings.Repeat("ef", 32)),
		ObservedAt: at(-2 * time.Second), ReportedAt: at(-time.Second), ReceivedAt: at(0), Completeness: text("final"),
		OperationID: operationID,
	})
	if err != nil || reported.AttemptID != attemptID || reported.Source != "daemon" || reported.SchemaVersion != 1 ||
		reported.AdapterVersion.String != "claude-code/1.2.3" || reported.NormalizerVersion.String != "1" ||
		reported.ProducerEpoch.String != "epoch-1" || reported.PayloadDigest.String != strings.Repeat("ef", 32) ||
		!reported.ObservedAt.Time.Equal(received.Add(-2*time.Second)) || !reported.ReportedAt.Time.Equal(received.Add(-time.Second)) ||
		!reported.ReceivedAt.Time.Equal(received) || reported.Completeness.String != "final" ||
		reported.OperationID != operationID {
		t.Fatalf("insert reported entry = %+v, %v", reported, err)
	}
	stored, err := q.InsertIssueBudgetEntry(ctx, db.InsertIssueBudgetEntryParams{
		WorkspaceID: workspace, BudgetRootID: rootID, Seq: 1, EntryID: entry.ID, Kind: string(entry.Kind),
		AttemptID: attemptID, EntryJson: raw, EntryDigest: digest, Receipt: receipt, Source: "server", SchemaVersion: 1,
	})
	// A server-written entry has no reporting envelope.
	if err != nil || string(stored.EntryJson) != string(raw) || stored.AdapterVersion.Valid || stored.NormalizerVersion.Valid ||
		stored.ProducerEpoch.Valid || stored.PayloadDigest.Valid || stored.ObservedAt.Valid || stored.ReportedAt.Valid ||
		stored.ReceivedAt.Valid || stored.Completeness.Valid || stored.OperationID.Valid {
		t.Fatalf("insert entry = %+v, %v", stored, err)
	}
	// Neither a reservation nor a usage entry that reported its own cost was
	// priced by the server: no snapshot on either row.
	if stored.PriceSnapshot != nil || stored.PriceDigest.Valid || reported.PriceSnapshot != nil || reported.PriceDigest.Valid {
		t.Fatalf("price snapshot on an entry that was not estimated: stored %v/%v, reported %v/%v",
			stored.PriceSnapshot != nil, stored.PriceDigest.Valid, reported.PriceSnapshot != nil, reported.PriceDigest.Valid)
	}
	// An estimate carries the snapshot it was priced with, and its cost cites
	// the digest the row stores. Rolled back: the ledger read below is of the
	// two entries above.
	priceRaw, priceDigest := issueBudgetPriceSnapshot(t)
	estimate := budget.Entry{ID: "usage-2", Kind: budget.KindUsage, ReservationID: entry.ReservationID, Usage: budget.Usage{
		ScopeID: "request-2", InvocationID: "invocation-1", Mode: budget.ModeDelta,
		Cost: budget.Cost{Basis: budget.CostEstimated, Ticks: 12, Currency: budget.CurrencyUSD, PriceVersion: "2026-11-01", PriceDigest: priceDigest},
	}}
	estimateRaw, estimateDigest, err := budget.EncodeEntry(estimate)
	if err != nil {
		t.Fatalf("encode estimate: %v", err)
	}
	pricing, err := testPool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer pricing.Rollback(ctx)
	priced, err := q.WithTx(pricing).InsertIssueBudgetEntry(ctx, db.InsertIssueBudgetEntryParams{
		WorkspaceID: workspace, BudgetRootID: rootID, Seq: 3, EntryID: estimate.ID, Kind: string(estimate.Kind),
		AttemptID: attemptID, InvocationID: text(estimate.Usage.InvocationID), ScopeID: text(estimate.Usage.ScopeID),
		EntryJson: estimateRaw, EntryDigest: estimateDigest, Receipt: []byte(`{}`),
		PriceSnapshot: priceRaw, PriceDigest: text(priceDigest),
		Source: "server", SchemaVersion: budget.EntryFormVersion,
	})
	if err != nil || string(priced.PriceSnapshot) != string(priceRaw) || priced.PriceDigest.String != priceDigest ||
		priced.PriceDigest.String != estimate.Usage.Cost.PriceDigest {
		t.Fatalf("insert priced entry = snapshot %d bytes, digest %q, %v", len(priced.PriceSnapshot), priced.PriceDigest.String, err)
	}
	if err := pricing.Rollback(ctx); err != nil {
		t.Fatal(err)
	}

	// A write replays the whole ledger of its root: every entry, in seq order,
	// and nothing through another workspace or of another root.
	ledger, err := q.ListIssueBudgetEntries(ctx, db.ListIssueBudgetEntriesParams{WorkspaceID: workspace, BudgetRootID: rootID})
	if err != nil || len(ledger) != 2 || ledger[0].Seq != 1 || ledger[1].Seq != 2 {
		t.Fatalf("ledger of the root = %d entries, %v; want seq 1, 2", len(ledger), err)
	}
	var replay []budget.Entry
	for _, row := range ledger {
		var e budget.Entry
		if err := json.Unmarshal(row.EntryJson, &e); err != nil {
			t.Fatalf("decode entry %d: %v", row.Seq, err)
		}
		replay = append(replay, e)
	}
	if projection, _, err := budget.Reduce(replay); err != nil || projection.ActualKnown != 30 || projection.Reserved != 20 {
		t.Fatalf("replayed ledger = A %d R %d, %v; want 30, 20", projection.ActualKnown, projection.Reserved, err)
	}
	for name, params := range map[string]db.ListIssueBudgetEntriesParams{
		"another workspace": {WorkspaceID: newID(), BudgetRootID: rootID},
		"another root":      {WorkspaceID: workspace, BudgetRootID: newID()},
	} {
		if got, err := q.ListIssueBudgetEntries(ctx, params); err != nil || len(got) != 0 {
			t.Fatalf("ledger read through %s = %d entries, %v; want none", name, len(got), err)
		}
	}
	// The receipt comes back as the same document the metering package made.
	var got budget.Receipt
	if err := json.Unmarshal(stored.Receipt, &got); err != nil || got != receipts[0] {
		t.Fatalf("stored receipt = %+v, %v; want %+v", got, err, receipts[0])
	}

	// The usage batch the reported entry cites, by id.
	batch, err := q.InsertIssueBudgetOperation(ctx, db.InsertIssueBudgetOperationParams{
		ID: operationID, WorkspaceID: workspace, BudgetRootID: rootID, ScopeKind: "attempt", ScopeID: attemptID, OperationKey: "batch-1",
		OperationKind: "report_usage", IntentDigest: strings.Repeat("ab", 32), ActorType: "runtime", ActorID: attempt.RuntimeID,
		Result: []byte(`{"accepted":"1"}`),
	})
	if err != nil || batch.ID != operationID || batch.ID != ledger[1].OperationID {
		t.Fatalf("insert operation = %+v, %v", batch, err)
	}
	observation, err := q.InsertIssueBudgetObservation(ctx, db.InsertIssueBudgetObservationParams{
		ID: newID(), WorkspaceID: workspace, BudgetRootID: rootID, Kind: "would_refuse", Reason: "root_conflict",
		IssueID: memberIssue, Detail: []byte(`{}`),
	})
	if err != nil || observation.AttemptID.Valid || observation.TaskID.Valid || observation.ClaimGeneration.Valid {
		t.Fatalf("insert observation = %+v, %v", observation, err)
	}
	// An in-flight gap names the claim it could not attribute.
	gap, err := q.InsertIssueBudgetObservation(ctx, db.InsertIssueBudgetObservationParams{
		ID: newID(), WorkspaceID: workspace, BudgetRootID: rootID, Kind: "coverage_gap", Reason: "in_flight_legacy",
		IssueID: memberIssue, TaskID: task, ClaimGeneration: pgtype.Int8{Int64: 0, Valid: true}, Detail: []byte(`{}`),
	})
	if err != nil || gap.TaskID != task || !gap.ClaimGeneration.Valid || gap.ClaimGeneration.Int64 != 0 {
		t.Fatalf("insert gap observation = %+v, %v", gap, err)
	}

	// The root lock is held until its transaction ends: a second locker with a
	// short lock timeout is refused, and gets through once the first is done.
	holder, err := testPool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer holder.Rollback(ctx)
	locked, err := q.WithTx(holder).LockIssueBudgetRoot(ctx, db.LockIssueBudgetRootParams{WorkspaceID: workspace, ID: rootID})
	if err != nil || locked.ID != rootID {
		t.Fatalf("lock root = %v, %v", locked.ID, err)
	}
	if _, err := q.WithTx(holder).LockIssueBudgetRoot(ctx, db.LockIssueBudgetRootParams{WorkspaceID: newID(), ID: rootID}); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("root locked through another workspace: %v, want no rows", err)
	}
	waiter, err := testPool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer waiter.Rollback(ctx)
	if _, err := waiter.Exec(ctx, `SET LOCAL lock_timeout = '200ms'`); err != nil {
		t.Fatal(err)
	}
	if _, err := q.WithTx(waiter).LockIssueBudgetRoot(ctx, db.LockIssueBudgetRootParams{WorkspaceID: workspace, ID: rootID}); !issueBudgetIsPGCode(err, "55P03") {
		t.Fatalf("second root lock while the first is held: %v, want SQLSTATE 55P03", err)
	}
	if err := waiter.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if err := holder.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := q.LockIssueBudgetRoot(ctx, db.LockIssueBudgetRootParams{WorkspaceID: workspace, ID: rootID}); err != nil {
		t.Fatalf("root lock after release: %v", err)
	}
}
