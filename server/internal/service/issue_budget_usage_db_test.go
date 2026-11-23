package service

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/budget"
	"github.com/multica-ai/multica/server/internal/metrics"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func usageScene(t *testing.T) (*issueBudgetFixture, string, db.IssueBudgetAttempt, db.AgentTaskQueue) {
	t.Helper()
	f := newIssueBudgetFixture(t)
	enabled, _, child := f.budgetTree(t, 0, 100)
	agent, runtime := f.agent(t)
	task := f.claimed(t, agent, runtime, child, 1)
	id := f.mustFinalize(t, f.claims(f.pool), task, TaskClaimBudgetProtocolV1)
	attempt, err := f.q.GetIssueBudgetUsageAttempt(context.Background(), util.MustParseUUID(id))
	if err != nil {
		t.Fatal(err)
	}
	return f, enabled.BudgetRootID, attempt, task
}

func usageOneBatch(t *testing.T) issueBudgetUsageBatch {
	t.Helper()
	b := usageBatch(t)
	b.Invocations = b.Invocations[:1]
	b.Invocations[0].Usage = b.Invocations[0].Usage[:1]
	ticks := budget.Ticks(30)
	b.Invocations[0].Usage[0].CostTicks = &ticks
	return b
}
func usageBytes(t *testing.T, b issueBudgetUsageBatch) []byte {
	t.Helper()
	raw, err := json.Marshal(b)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
func usageActualService(f *issueBudgetFixture, tx TxStarter) *IssueBudgetUsageService {
	s := NewIssueBudgetUsageService(f.q, tx)
	s.Evidence = IssueBudgetUsageEvidenceTable{{"claude", "1"}: {FreshOwnUsage: true, ResumedOwnUsage: true, ReportedCost: true}}
	return s
}
func requireUsageProjection(t *testing.T, f *issueBudgetFixture, root string) {
	t.Helper()
	row := f.rootRow(t, root)
	replayed := f.replayedLedger(t, root)
	if issueBudgetRootFigures(row) != issueBudgetLedgerFiguresOf(replayed) || row.LedgerRevision != int64(replayed.Len()) {
		t.Fatal("stored projection/revision differ from replay")
	}
}
func requireUsageRecorded(t *testing.T, response IssueBudgetUsageResponse, err error, replayed bool) {
	t.Helper()
	if err != nil || response.Status != "recorded" || response.Replayed == nil || *response.Replayed != replayed {
		t.Fatalf("usage = %+v, %v", response, err)
	}
}

func TestIssueBudgetUsageDBReplayConflictsAndSwitch(t *testing.T) {
	f, root, attempt, _ := usageScene(t)
	ctx := context.Background()
	b := usageOneBatch(t)
	raw := usageBytes(t, b)
	s := usageActualService(f, f.pool)
	first, err := s.Report(ctx, attempt, raw, time.Now())
	requireUsageRecorded(t, first, err, false)
	requireUsageProjection(t, f, root)
	if f.rootRow(t, root).ActualKnownTicks != 30 {
		t.Fatal("wrong actual")
	}
	before := f.snapshot(t)
	replay, err := s.Report(ctx, attempt, raw, time.Now())
	requireUsageRecorded(t, replay, err, true)
	f.requireUnchanged(t, before, "replay")
	if !reflect.DeepEqual(first.Receipts, replay.Receipts) {
		t.Fatal("replay changed receipts")
	}
	_, err = s.Report(ctx, attempt, append(raw, ' '), time.Now())
	if IssueBudgetErrorCode(err) != "operation_conflict" {
		t.Fatalf("changed bytes: %v", err)
	}
	b.BatchKey = "another_usage_batch_key"
	_, err = s.Report(ctx, attempt, usageBytes(t, b), time.Now())
	if IssueBudgetErrorCode(err) != "batch_conflict" {
		t.Fatalf("new batch: %v", err)
	}
	f.requireUnchanged(t, before, "conflicts")
	if _, err := f.svc.SetMode(ctx, f.manage(root, 1), "off"); err != nil {
		t.Fatal(err)
	}
	// Replay is independent of the switch and of an unreadable ledger.
	f.fx.Exec(t, `UPDATE issue_budget_root SET actual_known_ticks=999 WHERE id=$1`, root)
	before = f.snapshot(t)
	replay, err = s.Report(ctx, attempt, raw, time.Now())
	requireUsageRecorded(t, replay, err, true)
	f.requireUnchanged(t, before, "off/corrupt replay")
	query, err := s.Query(ctx, attempt, usageOneBatch(t).BatchKey)
	requireUsageRecorded(t, query, err, true)
	f.requireUnchanged(t, before, "query")
}

func TestIssueBudgetUsageDBOffAndUnavailableRemainRetryable(t *testing.T) {
	for _, unavailable := range []bool{false, true} {
		t.Run(map[bool]string{true: "unreadable", false: "off"}[unavailable], func(t *testing.T) {
			f, root, attempt, _ := usageScene(t)
			ctx := context.Background()
			s := usageActualService(f, f.pool)
			raw := usageBytes(t, usageOneBatch(t))
			if unavailable {
				f.fx.Exec(t, `UPDATE issue_budget_root SET actual_known_ticks=999 WHERE id=$1`, root)
			} else if _, err := f.svc.SetMode(ctx, f.manage(root, 1), "off"); err != nil {
				t.Fatal(err)
			}
			before := f.snapshot(t)
			response, err := s.Report(ctx, attempt, raw, time.Now())
			if unavailable {
				if !errors.Is(err, ErrIssueBudgetLedgerUnavailable) {
					t.Fatalf("unavailable: %v", err)
				}
			} else if err != nil || response.Status != "not_recorded" || response.Reason != "observation_off" {
				t.Fatalf("off: %+v %v", response, err)
			}
			f.requireUnchanged(t, before, "refused first delivery")
			if unavailable {
				f.fx.Exec(t, `UPDATE issue_budget_root SET actual_known_ticks=0 WHERE id=$1`, root)
			} else if _, err := f.svc.SetMode(ctx, f.manage(root, 2), "observe"); err != nil {
				t.Fatal(err)
			}
			response, err = s.Report(ctx, attempt, raw, time.Now())
			requireUsageRecorded(t, response, err, false)
			requireUsageProjection(t, f, root)
		})
	}
}

func TestIssueBudgetUsageDBAtomicInvalidAndProtocol(t *testing.T) {
	f, root, attempt, _ := usageScene(t)
	ctx := context.Background()
	log := &issueBudgetStatementLog{TxStarter: f.pool}
	s := usageActualService(f, log)
	before := f.snapshot(t)
	b := usageOneBatch(t)
	inv := b.Invocations[0]
	for _, id := range []string{"33333333-3333-4333-8333-333333333333", "44444444-4444-4444-8444-444444444444"} {
		next := inv
		next.InvocationID = id
		b.Invocations = append(b.Invocations, next)
	}
	b.Invocations[1].Ended = "invalid"
	_, err := s.Report(ctx, attempt, usageBytes(t, b), time.Now())
	var invalid *IssueBudgetUsageInvalidError
	if !errors.As(err, &invalid) || log.transactions() != 0 {
		t.Fatalf("invalid decoder opened transaction: %v", err)
	}
	f.requireUnchanged(t, before, "decode refusal")
	b.Invocations[1].Ended = "returned"
	huge := budget.Ticks(math.MaxInt64)
	b.Invocations[0].Usage = []issueBudgetUsageItem{{Model: "one", CostTicks: &huge}}
	_, err = s.Report(ctx, attempt, usageBytes(t, b), time.Now())
	if !errors.As(err, &invalid) || !errors.Is(err, budget.ErrOverflow) {
		t.Fatalf("overflow: %v", err)
	}
	f.requireUnchanged(t, before, "aggregate overflow")
	for _, protocol := range []pgtype.Int4{{}, {Int32: 2, Valid: true}} {
		old := attempt
		old.ProtocolVersion = protocol
		_, err = s.Report(ctx, old, usageBytes(t, usageOneBatch(t)), time.Now())
		if IssueBudgetErrorCode(err) != "protocol_mismatch" {
			t.Fatalf("protocol: %v", err)
		}
	}
	requireUsageProjection(t, f, root)
}

func TestIssueBudgetUsageDBEnvelopeGapsAndFrozenPrices(t *testing.T) {
	f, root, attempt, _ := usageScene(t)
	ctx := context.Background()
	s := NewIssueBudgetUsageService(f.q, f.pool)
	s.Prices = usagePrice
	b := usageBatch(t)
	b.AdapterVersion = ""
	b.Invocations[0].Usage = b.Invocations[0].Usage[1:]
	b.Invocations = append(b.Invocations, issueBudgetUsageInvocation{InvocationID: "33333333-3333-4333-8333-333333333333", Ended: "returned", ObservedAt: b.ReportedAt, Usage: []issueBudgetUsageItem{}})
	raw := usageBytes(t, b)
	received := time.Date(2026, 11, 20, 1, 2, 3, 0, time.UTC)
	response, err := s.Report(ctx, attempt, raw, received)
	requireUsageRecorded(t, response, err, false)
	rows := f.entryRows(t, root)
	if len(rows) != 5 {
		t.Fatalf("rows = %d", len(rows))
	}
	decoded, err := DecodeIssueBudgetUsage(raw)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows[1:] {
		if row.AttemptID != attempt.ID || row.AdapterVersion.String != "claude/unknown" || row.Source != "daemon_report" || row.PayloadDigest.String != decoded.PayloadDigest || !row.ReceivedAt.Time.Equal(received) {
			t.Fatalf("bad envelope: %+v", row)
		}
		if row.Kind == "gap" && (row.InvocationID.Valid || row.ScopeID.Valid || row.Completeness.Valid) {
			t.Fatal("gap gained invocation/scope/completeness")
		}
	}
	if !rows[1].PriceDigest.Valid || len(rows[1].PriceSnapshot) == 0 || rows[2].Completeness.String != "partial" {
		t.Fatal("missing snapshot or partial completeness")
	}
	var stored issueBudgetUsageOperationResult
	op, err := f.q.GetIssueBudgetOperationByKey(ctx, db.GetIssueBudgetOperationByKeyParams{WorkspaceID: f.ws, ScopeKind: "attempt", ScopeID: attempt.ID, OperationKey: b.BatchKey})
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(op.Result, &stored); err != nil {
		t.Fatal(err)
	}
	if len(stored.Invocations) != 4 || !stored.Invocations[1].Resumed || !stored.Invocations[0].Evidence.Matched || stored.Invocations[0].Evidence.ReportedCost {
		t.Fatalf("evidence = %+v", stored.Invocations)
	}
	first := f.rootRow(t, root)
	if first.UnknownCount == 0 || first.ActualKnownTicks != 0 || first.EstimatedTicks == 0 {
		t.Fatalf("projection = %+v", first)
	}
	s.Prices = func(string) (metrics.ModelPrice, bool) {
		return metrics.ModelPrice{Provider: "changed", Model: "changed", InputPerM: 100}, true
	}
	replay, err := s.Report(ctx, attempt, raw, received)
	requireUsageRecorded(t, replay, err, true)
	requireUsageProjection(t, f, root)
	if f.rootRow(t, root).EstimatedTicks != first.EstimatedTicks {
		t.Fatal("price update changed old estimate")
	}
}

func TestIssueBudgetUsageDBEmptyAndUnreserved(t *testing.T) {
	for _, empty := range []bool{false, true} {
		t.Run(map[bool]string{true: "empty", false: "unreserved"}[empty], func(t *testing.T) {
			f, root, attempt, _ := usageScene(t)
			ctx := context.Background()
			s := usageActualService(f, f.pool)
			b := usageOneBatch(t)
			if empty {
				b.Invocations = []issueBudgetUsageInvocation{}
			} else {
				f.fx.Exec(t, `DELETE FROM issue_budget_entry WHERE budget_root_id=$1`, root)
				f.fx.Exec(t, `UPDATE issue_budget_root SET ledger_revision=0,reserved_ticks=0 WHERE id=$1`, root)
			}
			revision := f.rootRow(t, root).LedgerRevision
			response, err := s.Report(ctx, attempt, usageBytes(t, b), time.Now())
			requireUsageRecorded(t, response, err, false)
			if empty {
				if f.rootRow(t, root).LedgerRevision != revision || response.Receipts == nil || len(*response.Receipts) != 0 {
					t.Fatal("empty batch wrote entries")
				}
			} else {
				rows := f.entryRows(t, root)
				if len(rows) != 1 || rows[0].AttemptID != attempt.ID {
					t.Fatal("unreserved usage lost attempt")
				}
				entry, err := budget.DecodeEntry(int(rows[0].SchemaVersion), rows[0].EntryJson)
				if err != nil || entry.ReservationID != "" {
					t.Fatalf("unreserved entry: %+v %v", entry, err)
				}
			}
			requireUsageProjection(t, f, root)
		})
	}
}

func TestIssueBudgetUsageDBLateClosedDeletedSuperseded(t *testing.T) {
	for _, state := range []string{"closed", "deleted", "superseded"} {
		t.Run(state, func(t *testing.T) {
			f, root, attempt, task := usageScene(t)
			ctx := context.Background()
			switch state {
			case "closed":
				if _, err := f.svc.Close(ctx, f.manage(root, 1)); err != nil {
					t.Fatal(err)
				}
			case "deleted":
				rootIssue := util.UUIDToString(f.rootRow(t, root).RootIssueID)
				f.fx.Exec(t, `UPDATE issue_budget_member SET tombstoned_at=now() WHERE workspace_id=$1 AND issue_id=$2`, f.workspace, rootIssue)
				// A retained attempt remains reportable even without its queue row.
				f.fx.Exec(t, `DELETE FROM agent_task_queue WHERE id=$1`, util.UUIDToString(task.ID))
				f.fx.Exec(t, `DELETE FROM issue WHERE id=$1`, rootIssue)
			case "superseded":
				next := f.claimedAgain(t, task)
				f.mustFinalize(t, f.claims(f.pool), next, TaskClaimBudgetProtocolV1)
			}
			response, err := usageActualService(f, f.pool).Report(ctx, attempt, usageBytes(t, usageOneBatch(t)), time.Now())
			requireUsageRecorded(t, response, err, false)
			requireUsageProjection(t, f, root)
			if f.rootRow(t, root).ActualKnownTicks != 30 {
				t.Fatal("late amount lost")
			}
		})
	}
}

func TestIssueBudgetUsageDBStatementsAndUnknownQuery(t *testing.T) {
	f, root, attempt, _ := usageScene(t)
	ctx := context.Background()
	log := &issueBudgetStatementLog{TxStarter: f.pool}
	s := usageActualService(f, log)
	log.beforeCommit = func(pid uint32) {
		if locks := f.advisoryLocks(t, pid); len(locks) != 0 {
			t.Fatalf("usage holds advisory locks: %+v", locks)
		}
		query, err := s.Query(ctx, attempt, usageOneBatch(t).BatchKey)
		if err != nil || query.Status != "unknown" || !strings.HasSuffix(query.QueryPath, "/usage/"+usageOneBatch(t).BatchKey) {
			t.Fatalf("uncommitted query = %+v %v", query, err)
		}
	}
	response, err := s.Report(ctx, attempt, usageBytes(t, usageOneBatch(t)), time.Now())
	requireUsageRecorded(t, response, err, false)
	want := []string{issueBudgetIsolationSQL, "LockIssueBudgetRoot", "GetIssueBudgetOperationByKey", "CountIssueBudgetUsageBatches", "InsertIssueBudgetEntry", "UpdateIssueBudgetRootLedger", "InsertIssueBudgetOperation"}
	if got := log.names(); !reflect.DeepEqual(got, want) {
		t.Fatalf("statements = %v, want %v", got, want)
	}
	before := f.snapshot(t)
	query, err := s.Query(ctx, attempt, usageOneBatch(t).BatchKey)
	requireUsageRecorded(t, query, err, true)
	f.requireUnchanged(t, before, "committed query")
	requireUsageProjection(t, f, root)
}

func TestIssueBudgetUsageDBInternalSettlementAndAttemptConsistency(t *testing.T) {
	f, root, attempt, _ := usageScene(t)
	ctx := context.Background()
	tx, err := f.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	q := f.q.WithTx(tx)
	locked, err := q.LockIssueBudgetRoot(ctx, db.LockIssueBudgetRootParams{WorkspaceID: f.ws, ID: attempt.BudgetRootID})
	if err != nil {
		t.Fatal(err)
	}
	ledger, err := issueBudgetLedgers.loadUnderRootLock(ctx, q, locked)
	if err != nil {
		t.Fatal(err)
	}
	settle := budget.Entry{ID: "settle/" + util.UUIDToString(attempt.ID), Kind: budget.KindSettle, ReservationID: util.UUIDToString(attempt.ID), Proof: budget.ReleaseProof{FinalUsage: true, ScopeComplete: true, NoFurtherExecution: true}}
	_, err = issueBudgetLedgers.appendEntries(ctx, q, locked, ledger, []issueBudgetLedgerEntry{{Entry: settle, Source: "server", AttemptID: util.MustParseUUID("33333333-3333-4333-8333-333333333333")}})
	if !errors.Is(err, budget.ErrInvalidEntry) {
		t.Fatalf("mismatched explicit attempt: %v", err)
	}
	appended, err := issueBudgetLedgers.appendEntries(ctx, q, locked, ledger, []issueBudgetLedgerEntry{{Entry: settle, Source: "server", AttemptID: attempt.ID}})
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	appended.Committed()
	requireUsageProjection(t, f, root)
	row := f.rootRow(t, root)
	if row.LedgerRevision != 2 || row.ReservedTicks != 0 || row.UnknownHoldTicks != 0 || appended.Receipts[0].Released != 100 {
		t.Fatalf("internal proof settlement: %+v", row)
	}
}

func TestIssueBudgetUsageDBUnexpectedEntryReplayRollsBack(t *testing.T) {
	f, root, attempt, _ := usageScene(t)
	ctx := context.Background()
	batch := usageOneBatch(t)
	raw := usageBytes(t, batch)
	decoded, err := DecodeIssueBudgetUsage(raw)
	if err != nil {
		t.Fatal(err)
	}
	s := usageActualService(f, f.pool)
	tx, err := f.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	q := f.q.WithTx(tx)
	locked, err := q.LockIssueBudgetRoot(ctx, db.LockIssueBudgetRootParams{WorkspaceID: f.ws, ID: attempt.BudgetRootID})
	if err != nil {
		t.Fatal(err)
	}
	ledger, err := issueBudgetLedgers.loadUnderRootLock(ctx, q, locked)
	if err != nil {
		t.Fatal(err)
	}
	normalized := normalizeIssueBudgetUsage(decoded, attempt.ID, true, s.Evidence, s.Prices)
	appended, err := issueBudgetLedgers.appendEntries(ctx, q, locked, ledger, normalized.Entries)
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	appended.Committed()
	before := f.snapshot(t)
	_, err = s.Report(ctx, attempt, raw, time.Now())
	var invalid *IssueBudgetUsageInvalidError
	if err == nil || errors.As(err, &invalid) || !strings.Contains(err.Error(), "unexpectedly replayed") {
		t.Fatalf("invariant error: %v", err)
	}
	f.requireUnchanged(t, before, "unexpected entry replay")
	requireUsageProjection(t, f, root)
}
