package service

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/budget"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
)

// B39/B41: the transaction rechecks membership before fences, root locks and
// original-key replays. System lifecycle transactions do not use this gate.
func TestIssueBudgetManagementMembership(t *testing.T) {
	f := newIssueBudgetFixture(t)
	ctx := context.Background()
	parent := f.issue(t, "managed", "")
	p := f.enableParams(parent)
	initial, err := f.svc.Enable(ctx, p)
	if err != nil {
		t.Fatal(err)
	}
	unbound := f.issue(t, "unbound", "")
	for _, role := range []string{"member", "missing"} {
		if role == "missing" {
			f.fx.Exec(t, `DELETE FROM member WHERE workspace_id=$1 AND user_id=$2`, f.workspace, f.user)
		} else {
			f.fx.Exec(t, `UPDATE member SET role=$3 WHERE workspace_id=$1 AND user_id=$2`, f.workspace, f.user, role)
		}
		before := f.snapshot(t)
		for _, op := range f.operations(unbound) {
			log := &issueBudgetStatementLog{TxStarter: f.pool}
			_, err := op.run(ctx, NewIssueBudgetService(f.q, log), f.manage(initial.BudgetRootID, 1))
			if IssueBudgetErrorCode(err) != IssueBudgetForbidden {
				t.Fatalf("%s by %s: %v", op.name, role, err)
			}
			requireIssueBudgetStatements(t, log.names(), []string{issueBudgetIsolationSQL, "GetMemberByUserAndWorkspace"}, "forbidden "+op.name)
			f.requireUnchanged(t, before, "forbidden "+op.name)
		}
		if _, err := f.svc.Enable(ctx, p); IssueBudgetErrorCode(err) != IssueBudgetForbidden {
			t.Fatalf("demoted original-key replay escaped membership check: %v", err)
		}
	}
	f.fx.Member(t, f.workspace, f.user, "admin")
	if replay, err := f.svc.Enable(ctx, p); err != nil || !replay.Replayed {
		t.Fatalf("admin replay = %+v %v", replay, err)
	}
	for i, op := range f.operations(unbound)[1:] {
		if _, err := op.run(ctx, f.svc, f.manage(initial.BudgetRootID, int64(i+1))); err != nil {
			t.Fatalf("admin %s: %v", op.name, err)
		}
	}
	if _, err := f.svc.Enable(ctx, f.enableParams(unbound)); err != nil {
		t.Fatalf("admin enable: %v", err)
	}

	// A concurrent lock of the member row must not turn this ordinary read
	// into another edge in the workspace/budget lock graph.
	holder, err := f.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer holder.Rollback(ctx)
	if _, err := holder.Exec(ctx, `SELECT 1 FROM member WHERE workspace_id=$1 AND user_id=$2 FOR UPDATE`, f.ws, f.actor); err != nil {
		t.Fatal(err)
	}
	bounded, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if _, err := f.svc.Close(bounded, f.manage(initial.BudgetRootID, 5)); err != nil {
		t.Fatalf("membership read waited for a row lock: %v", err)
	}
}

func TestIssueBudgetReadOperationRecovery(t *testing.T) {
	f := newIssueBudgetFixture(t)
	ctx := context.Background()
	issue := f.issue(t, "original-key recovery", "")
	p := f.enableParams(issue)
	enabled, err := f.svc.Enable(ctx, p)
	if err != nil {
		t.Fatal(err)
	}
	root := util.MustParseUUID(enabled.BudgetRootID)
	lookup, err := f.svc.LookupOperation(ctx, f.ws, "issue", p.IssueID, p.OperationKey)
	if err != nil || lookup == nil || lookup.Stale || !reflect.DeepEqual(lookup.Result, enabled) {
		t.Fatalf("committed result = %+v %v", lookup, err)
	}
	closeParams := f.manage(enabled.BudgetRootID, 1)
	closed, err := f.svc.Close(ctx, closeParams)
	if err != nil {
		t.Fatal(err)
	}
	lookup, err = f.svc.LookupOperation(ctx, f.ws, "issue", p.IssueID, p.OperationKey)
	if err != nil || lookup == nil || !lookup.Stale || !reflect.DeepEqual(lookup.Result, enabled) {
		t.Fatalf("stale receipt changed: %+v %v", lookup, err)
	}
	lookup, err = f.svc.LookupOperation(ctx, f.ws, "root", root, closeParams.OperationKey)
	if err != nil || lookup == nil || lookup.Stale || !reflect.DeepEqual(lookup.Result, closed) {
		t.Fatalf("current receipt = %+v %v", lookup, err)
	}
	for _, tc := range []struct {
		ws    pgtype.UUID
		scope string
		id    pgtype.UUID
		key   string
	}{
		{f.ws, "issue", p.IssueID, uuid.NewString()}, {util.MustParseUUID(uuid.NewString()), "issue", p.IssueID, p.OperationKey},
		{f.ws, "root", root, p.OperationKey},
	} {
		if got, err := f.svc.LookupOperation(ctx, tc.ws, tc.scope, tc.id, tc.key); err != nil || got != nil {
			t.Fatalf("unknown = %+v %v", got, err)
		}
	}
	if _, err := f.svc.LookupOperation(ctx, f.ws, "attempt", root, p.OperationKey); IssueBudgetErrorCode(err) != IssueBudgetInvalidRequest {
		t.Fatalf("attempt receipt accessible: %v", err)
	}
}

func TestIssueBudgetReadSnapshotAndUnknowns(t *testing.T) {
	f := newIssueBudgetFixture(t)
	ctx := context.Background()
	root := f.ledgerRoot(t)
	entries, figures, ids := issueBudgetMixedLedger()
	root.write(t, figures[len(entries)], entries...)
	rootRow := root.row(t)
	created := time.Date(2026, 11, 19, 1, 2, 3, 456789000, time.UTC)
	f.fx.Insert(t, "issue_budget_attempt", testutil.Cols{
		"id": ids[0], "workspace_id": f.workspace, "budget_root_id": util.UUIDToString(root.id),
		"task_id": uuid.NewString(), "claim_generation": int64(1), "runtime_id": uuid.NewString(), "agent_id": uuid.NewString(),
		"issue_id": util.UUIDToString(rootRow.RootIssueID), "created_at": created, "reserved_ticks": int64(100),
	})
	var task string
	f.fx.QueryRow(t, `SELECT task_id::text FROM issue_budget_attempt WHERE id=$1`, ids[0]).Scan(&task)
	for i, reason := range []string{"in_flight_legacy", "in_flight_at_bind", "observation_off", "lost_contact", "future_gap"} {
		cols := testutil.Cols{"id": uuid.NewString(), "workspace_id": f.workspace, "budget_root_id": util.UUIDToString(root.id),
			"kind": "coverage_gap", "reason": reason, "created_at": created.Add(time.Duration(i) * time.Microsecond)}
		if reason == "in_flight_at_bind" {
			cols["task_id"], cols["claim_generation"] = task, int64(1)
		}
		f.fx.Insert(t, "issue_budget_observation", cols)
	}
	before := f.snapshot(t)
	log := &issueBudgetStatementLog{TxStarter: f.pool}
	svc := NewIssueBudgetService(f.q, log)
	out, err := svc.Show(ctx, f.ws, root.id)
	if err != nil {
		t.Fatal(err)
	}
	requireIssueBudgetStatements(t, log.names(), []string{issueBudgetReadIsolationSQL, "GetIssueBudgetRoot", "GetIssueBudgetMember", "ListIssueBudgetEntryFormsAfter", "CountIssueBudgetUncoveredGaps"}, "show")
	if out.Unknown.PreActivation != 1 || out.TotalKnown || out.RemainingTicks != nil || out.Amounts == nil || out.Amounts.ActualKnown != budget.Ticks(figures[len(entries)].ActualKnownTicks) {
		t.Fatalf("show = %+v", out)
	}
	for _, reason := range out.Unknown.ByReason {
		if reason.Reason == "in_flight_at_bind" || reason.Reason == "in_flight_legacy" {
			t.Fatalf("covered/preactivation mixed into reasons: %+v", out.Unknown)
		}
	}
	if _, _, cached := issueBudgetLedgers.lookup(root.id, rootRow.LedgerRevision, false); cached {
		t.Fatal("read populated ledger cache")
	}
	ledger, _, err := budget.NewLedger().With(entries...)
	if err != nil {
		t.Fatal(err)
	}
	issueBudgetLedgers.publish(root.id, rootRow.LedgerRevision, ledger)
	t.Cleanup(func() { issueBudgetLedgers.forget(root.id, nil) })
	if _, err := svc.Show(ctx, f.ws, root.id); err != nil {
		t.Fatal(err)
	}
	if got, _, cached := issueBudgetLedgers.lookup(root.id, rootRow.LedgerRevision, false); !cached || got != ledger {
		t.Fatal("read changed a valid cached state")
	}
	f.requireUnchanged(t, before, "consistent reads")
	page, err := svc.PageObservations(ctx, f.ws, root.id, IssueBudgetPageParams{Limit: 100})
	if err != nil || len(page.Observations) != 5 {
		t.Fatalf("observation page = %+v %v", page, err)
	}
	covered := 0
	for _, o := range page.Observations {
		if o.Covered {
			covered++
			if o.Reason != "in_flight_at_bind" {
				t.Fatalf("wrong observation covered: %+v", o)
			}
		}
	}
	if covered != 1 {
		t.Fatalf("covered = %d", covered)
	}
	// Force a durable projection disagreement and clear only this fixture's
	// cache; unavailable responses may never publish the root's raw numbers.
	issueBudgetLedgers.forget(root.id, nil)
	f.fx.Exec(t, `UPDATE issue_budget_root SET actual_known_ticks=actual_known_ticks+1 WHERE id=$1`, root.id)
	out, err = svc.Show(ctx, f.ws, root.id)
	if err != nil || out.LedgerState != "unavailable" || out.Amounts != nil || out.Coverage.IncludedAttempts != nil || out.TotalKnown || out.Unknown.PreActivation != 1 {
		t.Fatalf("unavailable show = %+v %v", out, err)
	}
	attempts, err := svc.PageAttempts(ctx, f.ws, root.id, IssueBudgetPageParams{Limit: 100})
	if err != nil || attempts.LedgerState != "unavailable" || attempts.Attempts[0].Reservation != nil {
		t.Fatalf("unavailable attempts = %+v %v", attempts, err)
	}
}

// Every row shares one timestamp, so a time-only keyset loses rows here.
func TestIssueBudgetReadPages(t *testing.T) {
	f := newIssueBudgetFixture(t)
	ctx := context.Background()
	root := f.ledgerRoot(t)
	other := f.ledgerRoot(t)
	created := time.Date(2026, 11, 19, 1, 2, 3, 123456000, time.UTC)
	for i := range 251 {
		target := root
		if i == 250 {
			target = other
		}
		id := fmt.Sprintf("00000000-0000-4000-8000-%012d", i+1)
		f.fx.Insert(t, "issue_budget_attempt", testutil.Cols{
			"id": id, "workspace_id": f.workspace, "budget_root_id": util.UUIDToString(target.id),
			"task_id": uuid.NewString(), "claim_generation": int64(1), "runtime_id": uuid.NewString(), "agent_id": uuid.NewString(),
			"issue_id": uuid.NewString(), "created_at": created,
		})
		f.fx.Insert(t, "issue_budget_observation", testutil.Cols{
			"id": id, "workspace_id": f.workspace, "budget_root_id": util.UUIDToString(target.id),
			"kind": "would_refuse", "reason": "over_limit", "created_at": created,
		})
	}
	for _, attempts := range []bool{false, true} {
		seen := map[string]bool{}
		cursor, previous := "", "ffffffff-ffff-ffff-ffff-ffffffffffff"
		for pageNumber := 0; pageNumber < 3; pageNumber++ {
			var ids []string
			var next *string
			if attempts {
				page, err := f.svc.PageAttempts(ctx, f.ws, root.id, IssueBudgetPageParams{Limit: 100, Cursor: cursor})
				if err != nil {
					t.Fatal(err)
				}
				for _, row := range page.Attempts {
					ids = append(ids, row.AttemptID)
				}
				next = page.NextCursor
			} else {
				page, err := f.svc.PageObservations(ctx, f.ws, root.id, IssueBudgetPageParams{Limit: 100, Cursor: cursor})
				if err != nil {
					t.Fatal(err)
				}
				for _, row := range page.Observations {
					ids = append(ids, row.ID)
				}
				next = page.NextCursor
			}
			want := 100
			if pageNumber == 2 {
				want = 50
			}
			if len(ids) != want || (next == nil) != (pageNumber == 2) {
				t.Fatalf("attempts=%v page=%d size=%d next=%v", attempts, pageNumber, len(ids), next)
			}
			for _, id := range ids {
				if seen[id] || id >= previous || strings.HasSuffix(id, "000000000251") {
					t.Fatalf("wrong tuple boundary/root: %s after %s", id, previous)
				}
				seen[id], previous = true, id
			}
			if next != nil {
				cursor = *next
			}
		}
		if len(seen) != 250 {
			t.Fatalf("read %d rows", len(seen))
		}
	}
	// An exact-size page must not offer a nonexistent next page.
	page, err := f.svc.PageAttempts(ctx, f.ws, other.id, IssueBudgetPageParams{Limit: 1})
	if err != nil || len(page.Attempts) != 1 || page.NextCursor != nil {
		t.Fatalf("exact-size page = %+v %v", page, err)
	}
}

// A reader retains the root, ledger and gaps from one snapshot even when a
// writer commits after the first SELECT. The hook is test-only and synchronous.
type issueBudgetSnapshotStarter struct {
	TxStarter
	afterRoot func()
}

func (s issueBudgetSnapshotStarter) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return issueBudgetSnapshotTx{Tx: tx, afterRoot: s.afterRoot}, nil
}

type issueBudgetSnapshotTx struct {
	pgx.Tx
	afterRoot func()
}

func (t issueBudgetSnapshotTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	row := t.Tx.QueryRow(ctx, sql, args...)
	if strings.HasPrefix(sql, "-- name: GetIssueBudgetRoot :one") {
		return issueBudgetSnapshotRow{Row: row, after: t.afterRoot}
	}
	return row
}

type issueBudgetSnapshotRow struct {
	pgx.Row
	after func()
}

func (r issueBudgetSnapshotRow) Scan(dest ...any) error {
	err := r.Row.Scan(dest...)
	if err == nil {
		r.after()
	}
	return err
}

func TestIssueBudgetReadConcurrentSnapshot(t *testing.T) {
	f := newIssueBudgetFixture(t)
	root := f.ledgerRoot(t)
	svc := NewIssueBudgetService(f.q, issueBudgetSnapshotStarter{TxStarter: f.pool, afterRoot: func() {
		f.fx.Exec(t, `UPDATE issue_budget_root SET mode='off', policy_revision=policy_revision+1 WHERE id=$1`, root.id)
		f.fx.Insert(t, "issue_budget_observation", testutil.Cols{"id": uuid.NewString(), "workspace_id": f.workspace,
			"budget_root_id": util.UUIDToString(root.id), "kind": "coverage_gap", "reason": "observation_off"})
	}})
	first, err := svc.Show(context.Background(), f.ws, root.id)
	if err != nil || first.Mode != "observe" || first.PolicyRevision != 1 || !first.TotalKnown {
		t.Fatalf("read mixed snapshots: %+v %v", first, err)
	}
	second, err := f.svc.Show(context.Background(), f.ws, root.id)
	if err != nil || second.Mode != "off" || second.PolicyRevision != 2 || second.TotalKnown {
		t.Fatalf("next snapshot missed commit: %+v %v", second, err)
	}
}
