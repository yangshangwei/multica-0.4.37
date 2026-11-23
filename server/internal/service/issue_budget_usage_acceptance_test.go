package service

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/budget"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func usageAcceptanceAttempt(t *testing.T, f *issueBudgetFixture, task db.AgentTaskQueue) db.IssueBudgetAttempt {
	t.Helper()
	id := f.mustFinalize(t, f.claims(f.pool), task, TaskClaimBudgetProtocolV1)
	attempt, err := f.q.GetIssueBudgetUsageAttempt(context.Background(), util.MustParseUUID(id))
	if err != nil {
		t.Fatal(err)
	}
	return attempt
}

func usageAcceptanceReport(t *testing.T, f *issueBudgetFixture, attempt db.IssueBudgetAttempt, ticks budget.Ticks) []byte {
	t.Helper()
	batch := usageOneBatch(t)
	batch.Invocations[0].Usage[0].CostTicks = &ticks
	raw := usageBytes(t, batch)
	response, err := usageActualService(f, f.pool).Report(context.Background(), attempt, raw, time.Now())
	requireUsageRecorded(t, response, err, false)
	return raw
}

// B07: the retry is a real CreateRetryTask clone, not merely another unrelated
// fixture task. Both reports are admitted by the actual usage service.
func TestIssueBudgetUsageAcceptanceRetryKeepsActualAndLimit(t *testing.T) {
	f := newIssueBudgetFixture(t)
	ctx := context.Background()
	enabled, _, child := f.budgetTree(t, 500, 100)
	root := enabled.BudgetRootID
	agent, runtime := f.agent(t)
	first := f.claimed(t, agent, runtime, child, 1)
	firstAttempt := usageAcceptanceAttempt(t, f, first)
	usageAcceptanceReport(t, f, firstAttempt, 40)
	f.requireRootLedger(t, root, 2, issueBudgetLedgerFigures{ActualKnownTicks: 40, ReservedTicks: 60}, "first actual 40")
	f.endTask(t, first, "failed", "agent_error")
	f.reconcile(t, root, "failed original")
	retry, err := f.q.CreateRetryTask(ctx, db.CreateRetryTaskParams{ID: first.ID})
	if err != nil {
		t.Fatal(err)
	}
	f.fx.Cleanup(t, `DELETE FROM agent_task_queue WHERE id=$1`, util.UUIDToString(retry.ID))
	if retry.ID == first.ID || retry.RetryOfTaskID != first.ID || retry.IssueID != first.IssueID {
		t.Fatalf("retry lost new identity/lineage: %+v", retry)
	}
	retry = f.claimedAgain(t, retry)
	retryAttempt := usageAcceptanceAttempt(t, f, retry)
	if retryAttempt.ID == firstAttempt.ID || retryAttempt.BudgetRootID != firstAttempt.BudgetRootID {
		t.Fatal("retry replaced attempt or root")
	}
	raw := usageAcceptanceReport(t, f, retryAttempt, 25)
	f.requireRootLedger(t, root, 5, issueBudgetLedgerFigures{ActualKnownTicks: 65, ReservedTicks: 75, UnknownHoldTicks: 60, UnknownCount: 1}, "40 plus retry 25")
	row := f.rootRow(t, root)
	if !row.LimitTicks.Valid || row.LimitTicks.Int64 != 500 || row.PolicyRevision != 1 {
		t.Fatal("retry reset or changed original limit")
	}
	before := f.snapshot(t)
	replay, err := usageActualService(f, f.pool).Report(ctx, retryAttempt, raw, time.Now())
	requireUsageRecorded(t, replay, err, true)
	f.requireUnchanged(t, before, "retry report replay")
	requireUsageProjection(t, f, root)
}

// B38: observe both root-lock orders with two real writers queued behind a
// held root. Their lock waits, not sleeps, determine the order of commits.
func TestIssueBudgetUsageAcceptanceUsageAndCancellationBothOrders(t *testing.T) {
	for _, usageFirst := range []bool{true, false} {
		t.Run(map[bool]string{true: "usage_then_cancel_hold", false: "cancel_hold_then_usage"}[usageFirst], func(t *testing.T) {
			f, root, attempt, task := usageScene(t)
			f.endTask(t, task, "cancelled", nil)
			_, release := f.holdRoot(t, root)
			defer release()
			raw := usageBytes(t, usageOneBatch(t))
			var usage usageWriter
			var lifecycle *issueBudgetLifecycleWriter
			if usageFirst {
				usage = startUsageWriter(t, f, attempt, raw)
				f.waitsBehind(t, usage.pid, "usage before cancellation reconciliation")
				lifecycle = f.startReconcile(t, root)
				f.waitsBehind(t, lifecycle.pid, "cancellation reconciliation second")
			} else {
				lifecycle = f.startReconcile(t, root)
				f.waitsBehind(t, lifecycle.pid, "cancellation reconciliation first")
				usage = startUsageWriter(t, f, attempt, raw)
				f.waitsBehind(t, usage.pid, "late usage second")
			}
			release()
			reported := usage.outcome(t)
			requireUsageRecorded(t, reported.response, reported.err, false)
			closed := lifecycle.ended(t, "cancellation reconciliation")
			if len(closed.result.Closed) != 1 || closed.result.Closed[0].AttemptID != attempt.ID || closed.result.Closed[0].State != "cancelled" || !closed.result.Closed[0].Held {
				t.Fatalf("cancellation result: %+v", closed.result)
			}
			f.requireRootLedger(t, root, 3, issueBudgetLedgerFigures{ActualKnownTicks: 30, UnknownHoldTicks: 70, UnknownCount: 1}, "cancel preserves actual30 plus held70")
			entries := f.entryRows(t, root)
			wantKinds := []string{"reserve", "usage", "hold"}
			if !usageFirst {
				wantKinds = []string{"reserve", "hold", "usage"}
			}
			for i, row := range entries {
				if row.Kind != wantKinds[i] {
					t.Fatalf("commit order %d = %s, want %s", i, row.Kind, wantKinds[i])
				}
			}
			if n := f.fx.Count(t, `SELECT count(*) FROM issue_budget_entry WHERE attempt_id=$1 AND kind='hold'`, attempt.ID); n != 1 {
				t.Fatalf("terminal hold count = %d", n)
			}
			if n := f.fx.Count(t, `SELECT count(*) FROM issue_budget_operation WHERE scope_kind='attempt' AND scope_id=$1 AND operation_kind='usage_batch'`, attempt.ID); n != 1 {
				t.Fatalf("usage receipt count = %d", n)
			}
			before := f.snapshot(t)
			again := f.reconcile(t, root, "duplicate cancellation reconciliation")
			if len(again.result.Closed) != 0 {
				t.Fatal("duplicate cancellation closed attempt again")
			}
			replay, err := usageActualService(f, f.pool).Report(context.Background(), attempt, raw, time.Now())
			requireUsageRecorded(t, replay, err, true)
			f.requireUnchanged(t, before, "duplicate cancellation and consumption")
			requireUsageProjection(t, f, root)
		})
	}
}

// B10/B11/B57: mutation uses the real update/binding and detach/delete seams;
// each later execution then passes finalization and the usage endpoint.
func TestIssueBudgetUsageAcceptanceMovesAndParentDeletionKeepOriginalRoot(t *testing.T) {
	f := newIssueBudgetFixture(t)
	ctx := context.Background()
	first, parent, child := f.budgetTree(t, 1000, 100)
	root := first.BudgetRootID
	second, otherParent, _ := f.budgetTree(t, 1000, 100)
	rootless := f.issue(t, "Rootless destination", "")
	project := f.fx.Project(t, "Different project")
	original, bound := f.member(t, child)
	if !bound {
		t.Fatal("missing initial binding")
	}
	charge := func(issue string, amount, total budget.Ticks) {
		agent, runtime := f.agent(t)
		task := f.claimed(t, agent, runtime, issue, 1)
		attempt := usageAcceptanceAttempt(t, f, task)
		if util.UUIDToString(attempt.BudgetRootID) != root {
			t.Fatalf("later execution escaped original root: %+v", attempt)
		}
		usageAcceptanceReport(t, f, attempt, amount)
		if got := f.rootRow(t, root).ActualKnownTicks; got != int64(total) {
			t.Fatalf("actual after %d = %d, want %d", amount, got, total)
		}
		other := f.rootRow(t, second.BudgetRootID)
		if other.ActualKnownTicks != 0 || other.LedgerRevision != 0 {
			t.Fatal("destination root acquired original member spend")
		}
		requireUsageProjection(t, f, root)
	}
	charge(child, 10, 10)
	move := func(parent, project string) {
		f.inBinding(t, func(q *db.Queries) error {
			old, err := q.GetIssueInWorkspace(ctx, db.GetIssueInWorkspaceParams{WorkspaceID: f.ws, ID: util.MustParseUUID(child)})
			if err != nil {
				return err
			}
			params := db.UpdateIssueParams{ID: old.ID, AssigneeType: old.AssigneeType, AssigneeID: old.AssigneeID, StartDate: old.StartDate, DueDate: old.DueDate, Stage: old.Stage}
			if parent != "" {
				params.ParentIssueID = util.MustParseUUID(parent)
			}
			if project != "" {
				params.ProjectID = util.MustParseUUID(project)
			}
			updated, err := q.UpdateIssue(ctx, params)
			if err != nil {
				return err
			}
			if updated.ParentIssueID != old.ParentIssueID {
				return BindIssueBudgetOnReparent(ctx, q, updated)
			}
			return nil
		})
		now, ok := f.member(t, child)
		if !ok || !reflect.DeepEqual(now, original) {
			t.Fatalf("move rewrote member: %+v, want %+v", now, original)
		}
	}
	move(otherParent, "")
	charge(child, 20, 30)
	if n := f.fx.Count(t, `SELECT count(*) FROM issue_budget_observation WHERE budget_root_id=$1 AND issue_id=$2 AND kind='would_refuse' AND reason='root_conflict'`, root, child); n != 1 {
		t.Fatalf("cross-root move recorded %d conflicts", n)
	}
	move(rootless, "")
	charge(child, 30, 60)
	move("", "")
	charge(child, 40, 100)
	move("", project)
	charge(child, 50, 150)
	current, err := f.q.GetIssueInWorkspace(ctx, db.GetIssueInWorkspaceParams{WorkspaceID: f.ws, ID: util.MustParseUUID(child)})
	if err != nil {
		t.Fatal(err)
	}
	if current.ParentIssueID.Valid || util.UUIDToString(current.ProjectID) != project {
		t.Fatal("parent clearing/project move did not occur")
	}
	retained := f.createChild(t, "Survives parent deletion", parent)
	retainedID := util.UUIDToString(retained.ID)
	beforeDetach, ok := f.member(t, retainedID)
	if !ok {
		t.Fatal("retained child unbound")
	}
	f.inBinding(t, func(q *db.Queries) error {
		children, err := q.DetachDirectChildIssues(ctx, db.DetachDirectChildIssuesParams{WorkspaceID: f.ws, ParentIssueID: util.MustParseUUID(parent), ExcludedIssueIds: []pgtype.UUID{}})
		if err != nil {
			return err
		}
		if len(children) != 1 || children[0].ID != retained.ID || children[0].ParentIssueID.Valid {
			t.Fatalf("detach did not preserve expected child: %+v", children)
		}
		if err := TombstoneIssueBudgetMember(ctx, q, f.ws, util.MustParseUUID(parent)); err != nil {
			return err
		}
		return q.DeleteIssue(ctx, db.DeleteIssueParams{ID: util.MustParseUUID(parent), WorkspaceID: f.ws})
	})
	afterDetach, ok := f.member(t, retainedID)
	if !ok || !reflect.DeepEqual(afterDetach, beforeDetach) || afterDetach.BoundParent != parent {
		t.Fatal("parent deletion changed retained attribution")
	}
	charge(retainedID, 60, 210)
	if n := f.fx.Count(t, `SELECT count(*) FROM issue_budget_observation WHERE budget_root_id=$1 AND issue_id=$2 AND reason='root_deleted'`, root, retainedID); n != 1 {
		t.Fatalf("retained child claim missing deleted-root observation: %d", n)
	}
	if limit := f.rootRow(t, root).LimitTicks; !limit.Valid || limit.Int64 != 1000 {
		t.Fatal("moves reset limit")
	}
}

// B45: hand-calculated boundary A90 + reserve10 is exactly L100, while the
// second reserve10 reaches110. Observation records both successful claims.
func TestIssueBudgetUsageAcceptanceConcurrentReservationsPastLimit(t *testing.T) {
	f := newIssueBudgetFixture(t)
	ctx := context.Background()
	enabled, _, child := f.budgetTree(t, 100, 90)
	root := enabled.BudgetRootID
	agent, runtime := f.agent(t)
	seed := f.claimed(t, agent, runtime, child, 1)
	usageAcceptanceReport(t, f, usageAcceptanceAttempt(t, f, seed), 90)
	f.requireRootLedger(t, root, 2, issueBudgetLedgerFigures{ActualKnownTicks: 90}, "known actual90")
	limit, reserve := budget.Ticks(100), budget.Ticks(10)
	if _, err := f.svc.SetLimit(ctx, f.manage(root, 1), &limit, &reserve); err != nil {
		t.Fatal(err)
	}
	a1, r1 := f.agent(t)
	a2, r2 := f.agent(t)
	first := f.claimed(t, a1, r1, child, 1)
	second := f.claimed(t, a2, r2, child, 1)
	_, release := f.holdRoot(t, root)
	defer release()
	one := f.startFinalize(t, first, TaskClaimBudgetProtocolV1)
	f.waitsBehind(t, one.pid, "first ten-tick reservation")
	two := f.startFinalize(t, second, TaskClaimBudgetProtocolV1)
	f.waitsBehind(t, two.pid, "second ten-tick reservation")
	release()
	out1 := one.outcome(t, "first claim at limit")
	out2 := two.outcome(t, "second claim above limit")
	if out1.err != nil || out2.err != nil || !out1.finalized.BudgetAttemptID.Valid || !out2.finalized.BudgetAttemptID.Valid {
		t.Fatalf("observe blocked claims: %+v %+v", out1, out2)
	}
	f.requireRootLedger(t, root, 4, issueBudgetLedgerFigures{ActualKnownTicks: 90, ReservedTicks: 20}, "both ten-tick reservations")
	observations := f.claimObservations(t, root)
	if len(observations) != 1 || observations[0].Kind != "would_refuse" || observations[0].Reason != "limit" || observations[0].Attempt != util.UUIDToString(out2.finalized.BudgetAttemptID) {
		t.Fatalf("expected only second limit refusal: %+v", observations)
	}
	want := map[string]string{"actual_known_ticks": "90", "reserved_ticks": "20", "unknown_hold_ticks": "0", "limit_ticks": "100", "this_reservation": "sized", "this_reservation_ticks": "10"}
	if !reflect.DeepEqual(observations[0].Detail, want) {
		t.Fatalf("limit arithmetic detail = %v, want %v", observations[0].Detail, want)
	}
	// Compare execution-facing fields with a claim under a never-enabled issue.
	controlIssue := f.issue(t, "No budget control", "")
	ac, rc := f.agent(t)
	control := f.claimed(t, ac, rc, controlIssue, 1)
	controlResult, err := f.finalize(t, f.claims(f.pool), control, TaskClaimBudgetProtocolV1)
	if err != nil || controlResult.BudgetAttemptID.Valid {
		t.Fatalf("control finalization = %+v %v", controlResult, err)
	}
	for _, task := range []db.AgentTaskQueue{first, second, control} {
		current := f.taskRow(t, util.UUIDToString(task.ID))
		if current.Status != "dispatched" || current.ClaimGeneration != 1 || current.Priority != task.Priority || !current.DispatchedAt.Time.Equal(task.DispatchedAt.Time) || f.tokens(t, task) != 1 {
			t.Fatalf("budget changed execution-facing claim state: %+v", current)
		}
	}
	requireUsageProjection(t, f, root)
}
