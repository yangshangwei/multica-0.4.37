package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/multica-ai/multica/server/internal/budget"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/taskfailure"
)

// The lifecycle reconciliation of a budget root (design 10.3), in two files.
// This one holds the helpers and what a reconciliation on one connection
// leaves; issue_budget_lifecycle_locks_test.go holds the cases that need a
// second connection.
//
// Rows of the F3 acceptance matrix covered by the two: B36 (the half without a
// settlement), B37 and B38.
//
// The expectations are written from the design: the entry a reconciliation
// appends, its identity, the state an attempt ends in and the statements a run
// sends are literals here and not taken from the code under test. The stored
// form of a hold is compared with the bytes the metering package marshals for
// it; that package is not under test here.

// endTask moves a task row to a final status the way the statements that end
// a task leave it: the status, the failure reason and the completion time.
// failure is a string, or nil for a row without a reason.
func (f *issueBudgetFixture) endTask(t *testing.T, task db.AgentTaskQueue, status string, failure any) {
	t.Helper()
	f.endTaskWithError(t, task, status, failure, nil)
}

// endTaskWithError is endTask for a row that also carries an error text.
// errText is a string, or nil for a row without one.
func (f *issueBudgetFixture) endTaskWithError(t *testing.T, task db.AgentTaskQueue, status string, failure, errText any) {
	t.Helper()
	f.fx.Exec(t, `UPDATE agent_task_queue SET status=$2, failure_reason=$3, error=$4, completed_at=now() WHERE id=$1`,
		util.UUIDToString(task.ID), status, failure, errText)
}

// issueBudgetLifecycleClaim is a tree whose root declares a reserve of 100,
// and one finalized claim of a task of its child: an open attempt that
// reserved 100.
type issueBudgetLifecycleClaim struct {
	enabled       IssueBudgetOperationResult
	root          string
	rootID        pgtype.UUID
	parent, child string
	task          db.AgentTaskQueue
	attempt       string
}

func (f *issueBudgetFixture) lifecycleClaim(t *testing.T, agent, runtime, what string) issueBudgetLifecycleClaim {
	t.Helper()
	enabled, parent, child := f.budgetTree(t, 0, 100)
	c := issueBudgetLifecycleClaim{
		enabled: enabled, root: enabled.BudgetRootID, rootID: util.MustParseUUID(enabled.BudgetRootID),
		parent: parent, child: child,
	}
	c.task = f.claimed(t, agent, runtime, child, 1)
	c.attempt = f.mustFinalize(t, f.claims(f.pool), c.task, TaskClaimBudgetProtocolV1)
	if c.attempt == "" {
		t.Fatalf("%s: premise: the claim got no attempt", what)
	}
	f.requireRootLedger(t, c.root, 1, issueBudgetLedgerFigures{ReservedTicks: 100}, what+": premise: after the claim")
	return c
}

// issueBudgetLifecycleRun is one reconciliation and what its transaction
// sent.
type issueBudgetLifecycleRun struct {
	result IssueBudgetLifecycleResult
	// sent is every statement of the run apart from the load of the ledger.
	sent []string
	// loads counts the statements of the load.
	loads int
}

// reconcile is one lifecycle reconciliation of a root of the fixture
// workspace, which has to commit.
func (f *issueBudgetFixture) reconcile(t *testing.T, root, what string) issueBudgetLifecycleRun {
	t.Helper()
	log := &issueBudgetStatementLog{TxStarter: f.pool}
	result, err := NewIssueBudgetService(f.q, log).ReconcileLifecycle(context.Background(), f.ws, util.MustParseUUID(root))
	if err != nil {
		t.Fatalf("%s: reconcile the root: %v", what, err)
	}
	if log.transactions() != 1 {
		t.Fatalf("%s: the reconciliation opened %d transactions, want 1", what, log.transactions())
	}
	run := issueBudgetLifecycleRun{result: result}
	run.sent, run.loads = issueBudgetLifecycleSent(t, log.names(), what)
	return run
}

// issueBudgetLifecycleSent takes the load of the ledger out of what a
// reconciliation sent. The load stands directly behind the root lock and
// nowhere else; how many statements it is depends on what the process cache
// holds.
func issueBudgetLifecycleSent(t *testing.T, names []string, what string) (sent []string, loads int) {
	t.Helper()
	for _, name := range names {
		if name != "ListIssueBudgetEntryFormsAfter" {
			sent = append(sent, name)
			continue
		}
		if len(sent) == 0 || sent[len(sent)-1] != "LockIssueBudgetRoot" {
			t.Fatalf("%s: the ledger was loaded somewhere else than directly behind the root lock:\n%s", what, strings.Join(names, "\n"))
		}
		loads++
	}
	return sent, loads
}

func requireIssueBudgetLifecycleSent(t *testing.T, run issueBudgetLifecycleRun, want []string, what string) {
	t.Helper()
	if !reflect.DeepEqual(run.sent, want) {
		t.Fatalf("%s: the reconciliation sent\n%s\nwant\n%s", what, strings.Join(run.sent, "\n"), strings.Join(want, "\n"))
	}
}

// issueBudgetWantClosed is one attempt a reconciliation must report as
// closed.
type issueBudgetWantClosed struct {
	Attempt, Task, State string
	Held                 bool
}

// requireIssueBudgetLifecycleResult compares what a reconciliation reports.
// The attempts it closed are compared as a set.
func requireIssueBudgetLifecycleResult(t *testing.T, got IssueBudgetLifecycleResult, skipped string, want []issueBudgetWantClosed, what string) {
	t.Helper()
	closed := make([]issueBudgetWantClosed, 0, len(got.Closed))
	for _, c := range got.Closed {
		closed = append(closed, issueBudgetWantClosed{
			Attempt: util.UUIDToString(c.AttemptID), Task: util.UUIDToString(c.TaskID), State: c.State, Held: c.Held,
		})
	}
	want = append([]issueBudgetWantClosed{}, want...)
	for _, rows := range [][]issueBudgetWantClosed{closed, want} {
		sort.Slice(rows, func(i, j int) bool { return rows[i].Attempt < rows[j].Attempt })
	}
	if got.Skipped != skipped || !reflect.DeepEqual(closed, want) {
		t.Fatalf("%s: the reconciliation reports skipped %q and closed %+v, want skipped %q and closed %+v", what, got.Skipped, closed, skipped, want)
	}
}

// requireLifecycleHold compares one entry row a reconciliation appended with
// the entry design 10.3 gives it: its place in the sequence, the identity
// hold/<attempt>/<reason>, the attempt, the stored form (a hold of that
// reservation with that reason and nothing else, no detail), the receipt, the
// source "lifecycle" and no operation.
func (f *issueBudgetFixture) requireLifecycleHold(t *testing.T, got db.IssueBudgetEntry, seq int64, attempt, reason, what string) {
	t.Helper()
	id := "hold/" + attempt + "/" + reason
	if got.Seq != seq || got.EntryID != id || got.Kind != "hold" || util.UUIDToString(got.AttemptID) != attempt ||
		got.Source != "lifecycle" || got.OperationID.Valid || util.UUIDToString(got.WorkspaceID) != f.workspace {
		t.Fatalf("%s: entry row seq %d %q kind %q attempt %s source %q operation %s workspace %s, want seq %d %q kind \"hold\" attempt %s source \"lifecycle\", no operation, workspace %s",
			what, got.Seq, got.EntryID, got.Kind, util.UUIDToString(got.AttemptID), got.Source, util.UUIDToString(got.OperationID), util.UUIDToString(got.WorkspaceID),
			seq, id, attempt, f.workspace)
	}
	if got.InvocationID.Valid || got.ScopeID.Valid || got.ReceivedAt.Valid || got.PriceSnapshot != nil {
		t.Fatalf("%s: entry row %q carries an invocation, a scope, an arrival time or a price; a hold the server wrote has none", what, id)
	}
	want := budget.Entry{ID: id, Kind: budget.KindHold, ReservationID: attempt, HoldReason: budget.HoldReason(reason)}
	raw, digest, err := budget.EncodeEntry(want)
	if err != nil {
		t.Fatal(err)
	}
	// The same attempt and reason give the same bytes, whoever writes the
	// hold: this is what makes a second writer a replay.
	if got.SchemaVersion != 1 || !bytes.Equal(got.EntryJson, raw) {
		t.Fatalf("%s: stored form of %q (version %d)\n got: %s\nwant: %s (version 1)", what, id, got.SchemaVersion, got.EntryJson, raw)
	}
	if got.EntryDigest != digest || got.EntryDigest != issueBudgetStoredDigest(got.EntryJson) {
		t.Fatalf("%s: entry row %q has digest %s, which is not the digest of its stored form (%s)", what, id, got.EntryDigest, digest)
	}
	var committed budget.Receipt
	if err := json.Unmarshal(got.Receipt, &committed); err != nil {
		t.Fatalf("%s: receipt of %q: %v", what, id, err)
	}
	receipt := budget.Receipt{EntryID: id, Kind: budget.KindHold, Disposition: budget.DispositionAccepted, Reason: budget.UnknownReason(reason)}
	if committed != receipt {
		t.Fatalf("%s: receipt of %q\n got: %+v\nwant: %+v", what, id, committed, receipt)
	}
}

// requireNothingReleased fails when an entry row of the root settles a
// reservation or says it released an amount.
func (f *issueBudgetFixture) requireNothingReleased(t *testing.T, root, what string) {
	t.Helper()
	for _, row := range f.entryRows(t, root) {
		if row.Kind == "settle" {
			t.Fatalf("%s: entry %d %q settles a reservation; a lifecycle fact never does", what, row.Seq, row.EntryID)
		}
		var receipt budget.Receipt
		if err := json.Unmarshal(row.Receipt, &receipt); err != nil {
			t.Fatalf("%s: receipt of %q: %v", what, row.EntryID, err)
		}
		if receipt.Released != 0 {
			t.Fatalf("%s: entry %d %q released %d ticks; a lifecycle fact releases nothing", what, row.Seq, row.EntryID, receipt.Released)
		}
	}
}

// holdsOf counts the hold entries of a root that name the attempt.
func (f *issueBudgetFixture) holdsOf(t *testing.T, root, attempt string) (ids []string) {
	t.Helper()
	for _, row := range f.entryRows(t, root) {
		if row.Kind == "hold" && util.UUIDToString(row.AttemptID) == attempt {
			ids = append(ids, row.EntryID)
		}
	}
	return ids
}

// requireLifecycleAttempt fails unless the task of c has one attempt row: the
// one its claim left, in the state given.
func (f *issueBudgetFixture) requireLifecycleAttempt(t *testing.T, c issueBudgetLifecycleClaim, state, what string) {
	t.Helper()
	rows := f.attempts(t, c.task)
	if len(rows) != 1 {
		t.Fatalf("%s: the task has %d attempt rows, want 1", what, len(rows))
	}
	f.requireAttempt(t, rows[0], c.attempt, issueBudgetWantAttempt{
		Root: c.enabled, Task: c.task, Issue: c.child, State: state,
		Reserved: issueBudgetInt64(100), Protocol: issueBudgetInt32(1),
	}, what)
}

// requireLifecycleHeld fails unless the root of c is what one reconciliation
// leaves of its one claim: the reservation and one hold with the reason
// given, the amount moved from reserved to held, one unknown item of that
// reason, nothing released, and the attempt in the state of the same name.
func (f *issueBudgetFixture) requireLifecycleHeld(t *testing.T, c issueBudgetLifecycleClaim, reason, what string) {
	t.Helper()
	entries := f.entryRows(t, c.root)
	if len(entries) != 2 {
		t.Fatalf("%s: the root has %d entry rows, want the reservation and one hold", what, len(entries))
	}
	f.requireLifecycleHold(t, entries[1], 2, c.attempt, reason, what)
	f.requireRootLedger(t, c.root, 2, issueBudgetLedgerFigures{UnknownHoldTicks: 100, UnknownCount: 1}, what)
	f.requireNothingReleased(t, c.root, what)
	want := map[budget.UnknownReason]int{budget.UnknownReason(reason): 1}
	if counts := f.replayedLedger(t, c.root).Projection().UnknownCounts(); !reflect.DeepEqual(counts, want) {
		t.Fatalf("%s: a replay of the root's entries counts the unknown items %v, want %v", what, counts, want)
	}
	f.requireLifecycleAttempt(t, c, reason, what)
}

// requireIssueBudgetCachedFigures fails unless the process cache holds the
// root at the revision given, with the figures given.
func requireIssueBudgetCachedFigures(t *testing.T, rootID pgtype.UUID, revision int64, want issueBudgetLedgerFigures, what string) {
	t.Helper()
	got, ledger, ok := issueBudgetLedgers.peek(rootID)
	if !ok {
		t.Fatalf("%s: the cache holds nothing of the root, want revision %d", what, revision)
	}
	if figures := issueBudgetLedgerFiguresOf(ledger); got != revision || figures != want {
		t.Fatalf("%s: the cache holds revision %d with %+v, want revision %d with %+v", what, got, figures, revision, want)
	}
}

// L01 (B36). A task that was cancelled, that timed out or whose process
// exited leaves a lifecycle fact and no usage. The reconciliation turns what
// its attempt reserved into a hold with that reason: the amount moves from
// reserved to held, the unknown set gains one item of the reason, and nothing
// is released. The attempt leaves 'open' for the same word. Nothing else is
// written: a hold has no operation row and is no observation.
func TestIssueBudgetLifecycleHoldsWhatAnEndedAttemptReserved(t *testing.T) {
	f := newIssueBudgetFixture(t)
	agent, runtime := f.agent(t)
	for _, tc := range []struct {
		name    string
		status  string
		failure any
		reason  string
	}{
		{name: "a cancelled task", status: "cancelled", reason: "cancelled"},
		{name: "a task that timed out", status: "failed", failure: "agent_error.agent_timeout", reason: "timeout"},
		{name: "a task whose process exited", status: "completed", reason: "process_exit"},
	} {
		c := f.lifecycleClaim(t, agent, runtime, tc.name)
		if counts := f.replayedLedger(t, c.root).Projection().UnknownCounts(); len(counts) != 0 {
			t.Fatalf("%s: premise: the root has unknown items before the task ended: %v", tc.name, counts)
		}
		f.endTask(t, c.task, tc.status, tc.failure)
		before := f.snapshot(t)

		run := f.reconcile(t, c.root, tc.name)
		requireIssueBudgetLifecycleResult(t, run.result, "", []issueBudgetWantClosed{
			{Attempt: c.attempt, Task: util.UUIDToString(c.task.ID), State: tc.reason, Held: true},
		}, tc.name)
		f.requireLifecycleHeld(t, c, tc.reason, tc.name)
		requireIssueBudgetLifecycleSent(t, run, []string{
			issueBudgetLevelStatement,
			"LockIssueBudgetRoot",
			"ListIssueBudgetOpenAttemptsOfRoot",
			"CloseIssueBudgetAttempt",
			"InsertIssueBudgetEntry",
			"UpdateIssueBudgetRootLedger",
		}, tc.name)
		after := f.snapshot(t)
		for _, table := range []string{"issue_budget_member", "issue_budget_operation", "issue_budget_observation"} {
			if before[table] != after[table] {
				t.Fatalf("%s: the reconciliation changed %s\nbefore: %s\nafter:  %s", tc.name, table, before[table], after[table])
			}
		}
		// Published once the commit is confirmed: the next writer of the root
		// is served the ledger with the hold in it.
		requireIssueBudgetCachedFigures(t, c.rootID, 2, issueBudgetLedgerFigures{UnknownHoldTicks: 100, UnknownCount: 1}, tc.name)
	}
}

// L02 (B37). Five facts that are not usage: the queue row is gone, the HTTP
// request of the claim was aborted, a TTL ran out, nothing is running any
// more, and the task was claimed again. None of them proves what the agent
// spent, so none releases a tick: what the attempt reserved stays in the root,
// reserved while the task can still run and held once it cannot.
//
// Three of the five touch no budget code at all. An aborted request, a lease
// that ran out and a task that went back to the queue each only leave a task
// row in some state, so what is pinned for them is what the reconciliation
// does with that state: nothing while the task is not final, and a hold, never
// a release, once it is.
func TestIssueBudgetLifecycleReleasesNothingForAFactThatIsNotUsage(t *testing.T) {
	f := newIssueBudgetFixture(t)
	ctx := context.Background()
	agent, runtime := f.agent(t)

	// requireUntouched: the reconciliation looked and wrote nothing.
	requireUntouched := func(c issueBudgetLifecycleClaim, what string) {
		t.Helper()
		before := f.snapshot(t)
		run := f.reconcile(t, c.root, what)
		requireIssueBudgetLifecycleResult(t, run.result, "", nil, what)
		requireIssueBudgetLifecycleSent(t, run, []string{issueBudgetLevelStatement, "LockIssueBudgetRoot", "ListIssueBudgetOpenAttemptsOfRoot"}, what)
		f.requireUnchanged(t, before, what)
		f.requireNothingReleased(t, c.root, what)
	}

	t.Run("the queue row is gone", func(t *testing.T) {
		c := f.lifecycleClaim(t, agent, runtime, "queue row gone")
		f.fx.Exec(t, `DELETE FROM agent_task_queue WHERE id=$1`, util.UUIDToString(c.task.ID))
		run := f.reconcile(t, c.root, "queue row gone")
		requireIssueBudgetLifecycleResult(t, run.result, "", []issueBudgetWantClosed{
			{Attempt: c.attempt, Task: util.UUIDToString(c.task.ID), State: "lost_contact", Held: true},
		}, "queue row gone")
		// Held, with the weakest reason there is: nothing is left to say how
		// the task ended.
		f.requireLifecycleHeld(t, c, "lost_contact", "queue row gone")
	})

	t.Run("the HTTP request of the claim was aborted", func(t *testing.T) {
		// The finalization committed and the daemon never read the answer.
		// No statement runs for that: the row stays dispatched at its
		// generation until a lease runs out or the daemon comes back.
		c := f.lifecycleClaim(t, agent, runtime, "HTTP abort")
		if row := f.taskRow(t, util.UUIDToString(c.task.ID)); row.Status != "dispatched" || row.ClaimGeneration != 1 {
			t.Fatalf("premise: the task is %s at generation %d", row.Status, row.ClaimGeneration)
		}
		requireUntouched(c, "HTTP abort")
		f.requireRootLedger(t, c.root, 1, issueBudgetLedgerFigures{ReservedTicks: 100}, "HTTP abort")
		f.requireLifecycleAttempt(t, c, "open", "HTTP abort")
	})

	t.Run("the TTL ran out", func(t *testing.T) {
		// The sweeper's own statement fails the task: a dispatched row whose
		// prepare lease was never renewed. The statement fails every stale
		// task of the database, so the row is made twenty years old and the
		// thresholds ten: no row another test wrote is that old.
		c := f.lifecycleClaim(t, agent, runtime, "TTL")
		task := util.UUIDToString(c.task.ID)
		f.fx.Exec(t, `UPDATE agent_task_queue SET dispatched_at=now()-interval '20 years', prepare_lease_expires_at=NULL WHERE id=$1`, task)
		const tenYears = 10 * 365 * 24 * 3600
		failed, err := f.q.FailStaleTasks(ctx, db.FailStaleTasksParams{
			DispatchTimeoutSecs: tenYears, RunningTimeoutSecs: tenYears,
			// Whatever the runtime of the fixture says about itself.
			RuntimeStaleSecs: tenYears, RuntimeReconnectGraceSecs: 0,
		})
		if err != nil {
			t.Fatalf("FailStaleTasks: %v", err)
		}
		found := false
		for _, row := range failed {
			found = found || util.UUIDToString(row.ID) == task
		}
		if !found {
			t.Fatalf("premise: FailStaleTasks failed %d tasks and not the one of this test", len(failed))
		}
		if row := f.taskRow(t, task); row.Status != "failed" || !row.FailureReason.Valid || row.FailureReason.String != "timeout" ||
			!row.Error.Valid || row.Error.String != "task timed out" || row.ClaimGeneration != 1 {
			t.Fatalf("premise: FailStaleTasks left the task %s with reason %+v and error %+v at generation %d", row.Status, row.FailureReason, row.Error, row.ClaimGeneration)
		}
		// The reason on the row is 'timeout', and the hold is not: nobody saw
		// this task time out, its daemon only stopped proving it was alive.
		run := f.reconcile(t, c.root, "TTL")
		requireIssueBudgetLifecycleResult(t, run.result, "", []issueBudgetWantClosed{
			{Attempt: c.attempt, Task: util.UUIDToString(c.task.ID), State: "lost_contact", Held: true},
		}, "TTL")
		f.requireLifecycleHeld(t, c, "lost_contact", "TTL")
	})

	t.Run("nothing is running any more", func(t *testing.T) {
		// The claim went back to the queue at its generation, through the
		// statement that does that. The tree has no dispatched or running task
		// now, and its one attempt is as open as before.
		c := f.lifecycleClaim(t, agent, runtime, "running count zero")
		if _, err := f.q.RequeueAgentTaskAfterClaimFailure(ctx, db.RequeueAgentTaskAfterClaimFailureParams{
			TaskID: c.task.ID, RuntimeID: c.task.RuntimeID, DispatchedAt: c.task.DispatchedAt, ClaimGeneration: c.task.ClaimGeneration,
		}); err != nil {
			t.Fatalf("requeue the claim: %v", err)
		}
		if row := f.taskRow(t, util.UUIDToString(c.task.ID)); row.Status != "queued" || row.ClaimGeneration != 1 {
			t.Fatalf("premise: the requeued task is %s at generation %d", row.Status, row.ClaimGeneration)
		}
		if running := f.fx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id IN ($1, $2) AND status IN ('dispatched', 'running', 'waiting_local_directory')`, c.parent, c.child); running != 0 {
			t.Fatalf("premise: %d tasks of the tree are dispatched or running", running)
		}
		requireUntouched(c, "running count zero")
		f.requireRootLedger(t, c.root, 1, issueBudgetLedgerFigures{ReservedTicks: 100}, "running count zero")
		f.requireLifecycleAttempt(t, c, "open", "running count zero")
	})

	t.Run("the task was claimed again", func(t *testing.T) {
		c := f.lifecycleClaim(t, agent, runtime, "re-claim")
		second := f.claimedAgain(t, c.task)
		// Between the claim statement and its finalization: the attempt is of
		// an earlier generation, and the task is not final. It is left to the
		// finalization of the new claim.
		requireUntouched(c, "re-claim, before the finalization")
		f.requireRootLedger(t, c.root, 1, issueBudgetLedgerFigures{ReservedTicks: 100}, "re-claim, before the finalization")

		a2 := f.mustFinalize(t, f.claims(f.pool), second, TaskClaimBudgetProtocolV1)
		rows := f.attempts(t, c.task)
		if len(rows) != 2 || rows[0].ID != c.attempt || rows[0].State != "superseded_unproven" || rows[1].ID != a2 || rows[1].State != "open" {
			t.Fatalf("after the second claim the task has attempts %+v, want %s superseded and %s open", rows, c.attempt, a2)
		}
		// The new claim reserved again, and what the first reserved is held.
		f.requireRootLedger(t, c.root, 3, issueBudgetLedgerFigures{ReservedTicks: 100, UnknownHoldTicks: 100, UnknownCount: 1}, "re-claim, after the finalization")
		requireUntouched(c, "re-claim, after the finalization")
	})
}

// L03 (B38, one connection). A reservation gets one terminal contribution and
// one receipt. A second reconciliation of the same root finds no open attempt
// of an ended task and writes nothing; the hold sent again by hand, with the
// same identity and content, is answered as a replay and writes nothing
// either.
func TestIssueBudgetLifecycleRunTwiceWritesOnce(t *testing.T) {
	f := newIssueBudgetFixture(t)
	ctx := context.Background()
	agent, runtime := f.agent(t)
	c := f.lifecycleClaim(t, agent, runtime, "first run")
	f.endTask(t, c.task, "cancelled", nil)
	first := f.reconcile(t, c.root, "first run")
	requireIssueBudgetLifecycleResult(t, first.result, "", []issueBudgetWantClosed{
		{Attempt: c.attempt, Task: util.UUIDToString(c.task.ID), State: "cancelled", Held: true},
	}, "first run")
	f.requireLifecycleHeld(t, c, "cancelled", "first run")

	before := f.snapshot(t)
	for _, what := range []string{"second run", "third run"} {
		run := f.reconcile(t, c.root, what)
		requireIssueBudgetLifecycleResult(t, run.result, "", nil, what)
		requireIssueBudgetLifecycleSent(t, run, []string{issueBudgetLevelStatement, "LockIssueBudgetRoot", "ListIssueBudgetOpenAttemptsOfRoot"}, what)
		f.requireUnchanged(t, before, what)
	}
	if holds := f.holdsOf(t, c.root, c.attempt); !reflect.DeepEqual(holds, []string{"hold/" + c.attempt + "/cancelled"}) {
		t.Fatalf("the reservation has the holds %v, want exactly one", holds)
	}

	// The same hold again, as a writer that did not see the first would send
	// it.
	sent := (&issueBudgetLedgerRoot{f: f, id: c.rootID}).locked(t, true, func(q *db.Queries, root db.IssueBudgetRoot) {
		ledger, err := issueBudgetLedgers.loadUnderRootLock(ctx, q, root)
		if err != nil {
			t.Fatalf("load the ledger: %v", err)
		}
		appended, err := issueBudgetLedgers.appendEntries(ctx, q, root, ledger, []issueBudgetLedgerEntry{
			{Entry: issueBudgetHoldEntry(c.attempt, budget.HoldCancelled), Source: issueBudgetSourceLifecycle},
		})
		if err != nil {
			t.Fatalf("send the hold again: %v", err)
		}
		if len(appended.Receipts) != 1 || !appended.Receipts[0].Replayed || appended.Receipts[0].Disposition != budget.DispositionAccepted ||
			appended.Receipts[0].Reason != "cancelled" || appended.Receipts[0].Released != 0 {
			t.Fatalf("the hold sent again got the receipts %+v, want the first receipt marked as a replay", appended.Receipts)
		}
	})
	for _, name := range sent {
		if name == "InsertIssueBudgetEntry" || name == "UpdateIssueBudgetRootLedger" {
			t.Fatalf("the hold sent again wrote: %v", sent)
		}
	}
	f.requireUnchanged(t, before, "the hold sent again")
}

// L04. An attempt that was claimed while the ledger could not be loaded has no
// reservation. When its task ends there is nothing to hold: the attempt only
// leaves 'open', no entry is appended and the root row is not written.
func TestIssueBudgetLifecycleClosesAnAttemptWithoutAReservationWithoutAHold(t *testing.T) {
	f := newIssueBudgetFixture(t)
	agent, runtime := f.agent(t)

	t.Run("the only attempt of the root", func(t *testing.T) {
		enabled, _, child := f.budgetTree(t, 0, 100)
		root := enabled.BudgetRootID
		ledger := &issueBudgetLedgerRoot{f: f, id: util.MustParseUUID(root)}
		// Figures no empty ledger adds up to: the load fails.
		ledger.set(t, 0, issueBudgetLedgerFigures{ReservedTicks: 1})
		task := f.claimed(t, agent, runtime, child, 1)
		attempt := f.mustFinalize(t, f.claims(f.pool), task, TaskClaimBudgetProtocolV1)
		rows := f.attempts(t, task)
		if len(rows) != 1 || rows[0].ID != attempt || rows[0].ReservedTicks != nil || rows[0].State != "open" {
			t.Fatalf("premise: the claim on the ledger that does not load left %+v, want one open attempt without a reservation", rows)
		}
		ledger.set(t, 0, issueBudgetLedgerFigures{})
		f.endTask(t, task, "completed", nil)
		before := f.snapshot(t)

		run := f.reconcile(t, root, "the only attempt")
		requireIssueBudgetLifecycleResult(t, run.result, "", []issueBudgetWantClosed{
			{Attempt: attempt, Task: util.UUIDToString(task.ID), State: "process_exit", Held: false},
		}, "the only attempt")
		requireIssueBudgetLifecycleSent(t, run, []string{
			issueBudgetLevelStatement, "LockIssueBudgetRoot", "ListIssueBudgetOpenAttemptsOfRoot", "CloseIssueBudgetAttempt",
		}, "the only attempt")
		after := f.snapshot(t)
		for _, table := range issueBudgetTables {
			if table != "issue_budget_attempt" && before[table] != after[table] {
				t.Fatalf("closing an attempt without a reservation changed %s\nbefore: %s\nafter:  %s", table, before[table], after[table])
			}
		}
		f.requireAttempt(t, f.attempts(t, task)[0], attempt, issueBudgetWantAttempt{
			Root: enabled, Task: task, Issue: child, State: "process_exit", Protocol: issueBudgetInt32(1),
		}, "the only attempt")
		f.requireRootLedger(t, root, 0, issueBudgetLedgerFigures{}, "the only attempt")
	})

	t.Run("beside an attempt that has one", func(t *testing.T) {
		c := f.lifecycleClaim(t, agent, runtime, "two attempts")
		ledger := &issueBudgetLedgerRoot{f: f, id: c.rootID}
		ledger.set(t, 1, issueBudgetLedgerFigures{ReservedTicks: 101})
		second := f.claimedAgain(t, c.task)
		a2 := f.mustFinalize(t, f.claims(f.pool), second, TaskClaimBudgetProtocolV1)
		rows := f.attempts(t, c.task)
		if len(rows) != 2 || rows[0].State != "open" || rows[1].ID != a2 || rows[1].State != "open" || rows[1].ReservedTicks != nil {
			t.Fatalf("premise: the claim on the ledger that does not load left %+v, want the first attempt open and a second without a reservation", rows)
		}
		ledger.set(t, 1, issueBudgetLedgerFigures{ReservedTicks: 100})
		// The row ended at its second claim.
		f.endTask(t, second, "failed", "agent_error")

		run := f.reconcile(t, c.root, "two attempts")
		// The first attempt belongs to an earlier claim, so it was superseded;
		// the second is the claim the final status speaks about.
		requireIssueBudgetLifecycleResult(t, run.result, "", []issueBudgetWantClosed{
			{Attempt: c.attempt, Task: util.UUIDToString(c.task.ID), State: "superseded_unproven", Held: true},
			{Attempt: a2, Task: util.UUIDToString(c.task.ID), State: "process_exit", Held: false},
		}, "two attempts")
		requireIssueBudgetLifecycleSent(t, run, []string{
			issueBudgetLevelStatement, "LockIssueBudgetRoot", "ListIssueBudgetOpenAttemptsOfRoot",
			"CloseIssueBudgetAttempt", "CloseIssueBudgetAttempt",
			"InsertIssueBudgetEntry", "UpdateIssueBudgetRootLedger",
		}, "two attempts")
		entries := f.entryRows(t, c.root)
		if len(entries) != 2 {
			t.Fatalf("the root has %d entry rows, want the reservation and one hold", len(entries))
		}
		f.requireLifecycleHold(t, entries[1], 2, c.attempt, "superseded_unproven", "two attempts")
		f.requireRootLedger(t, c.root, 2, issueBudgetLedgerFigures{UnknownHoldTicks: 100, UnknownCount: 1}, "two attempts")
		rows = f.attempts(t, c.task)
		if len(rows) != 2 || rows[0].State != "superseded_unproven" || rows[1].State != "process_exit" {
			t.Fatalf("the attempts ended as %+v, want superseded_unproven and process_exit", rows)
		}
		if holds := f.holdsOf(t, c.root, a2); len(holds) != 0 {
			t.Fatalf("the attempt without a reservation has the holds %v", holds)
		}
	})
}

// issueBudgetLifecycleDamages are four ways the ledger of a root at revision
// 1, with one reservation of 100, stops loading, by the cause the load names.
var issueBudgetLifecycleDamages = map[string]func(t *testing.T, r *issueBudgetLedgerRoot){
	// A second row in a form version this build does not read.
	"stored_form": func(t *testing.T, r *issueBudgetLedgerRoot) {
		e := issueBudgetReserveEntry(uuid.NewString(), 300)
		raw, _, err := budget.EncodeEntry(e)
		if err != nil {
			t.Fatal(err)
		}
		r.insertForm(t, 2, e.ID, e, raw, 2)
		r.set(t, 2, issueBudgetLedgerFigures{ReservedTicks: 400})
	},
	// A hold on a reservation no entry opened.
	"replay": func(t *testing.T, r *issueBudgetLedgerRoot) {
		r.insert(t, 2, issueBudgetHoldEntry(uuid.NewString(), budget.HoldTimeout))
		r.set(t, 2, issueBudgetLedgerFigures{ReservedTicks: 100})
	},
	// The root row is at revision 3 and there is no row at seq 2.
	"sequence": func(t *testing.T, r *issueBudgetLedgerRoot) {
		r.insert(t, 3, issueBudgetReserveEntry(uuid.NewString(), 300))
		r.set(t, 3, issueBudgetLedgerFigures{ReservedTicks: 400})
	},
	// The figures on the root row are not what its entries add up to.
	"projection": func(t *testing.T, r *issueBudgetLedgerRoot) {
		r.set(t, 1, issueBudgetLedgerFigures{ReservedTicks: 101})
	},
}

// L05 (10.5). A root whose ledger cannot be loaded is left exactly as it is:
// whether an attempt has a reservation to hold cannot be asked, so no attempt
// is closed. The run says so at error level, once, and ends its transaction
// without an error, so the sweep goes on to the next root. After the repair
// the next run writes the hold.
func TestIssueBudgetLifecycleLeavesARootWhoseLedgerDoesNotLoad(t *testing.T) {
	f := newIssueBudgetFixture(t)
	agent, runtime := f.agent(t)
	logs := issueBudgetClaimLog(t)
	for _, cause := range []string{"stored_form", "replay", "sequence", "projection"} {
		c := f.lifecycleClaim(t, agent, runtime, cause)
		ledger := &issueBudgetLedgerRoot{f: f, id: c.rootID}
		f.endTask(t, c.task, "cancelled", nil)
		issueBudgetLifecycleDamages[cause](t, ledger)
		logs()
		before := f.snapshot(t)

		run := f.reconcile(t, c.root, cause)
		requireIssueBudgetLifecycleResult(t, run.result, IssueBudgetLifecycleSkippedLedger, nil, cause)
		f.requireUnchanged(t, before, cause+": the run on the ledger that does not load")
		// The lock and the load, and then nothing: the attempts are not even
		// listed.
		requireIssueBudgetLifecycleSent(t, run, []string{issueBudgetLevelStatement, "LockIssueBudgetRoot"}, cause)
		if run.loads == 0 {
			t.Fatalf("%s: premise: the run did not try to load the ledger", cause)
		}
		var errorLines []string
		for _, line := range logs() {
			if strings.Contains(line, "level=ERROR") {
				errorLines = append(errorLines, line)
			}
		}
		requireIssueBudgetLogLine(t, errorLines, "ERROR", []string{"budget_root_id=" + c.root}, cause)
		if !strings.Contains(errorLines[0], ": "+cause+": ") {
			t.Fatalf("%s: premise: the ledger was to be unavailable for cause %q: %s", cause, cause, errorLines[0])
		}
		issueBudgetLedgers.requireNotCached(t, c.rootID, cause+": a root whose ledger does not load")

		// Repaired: the one good entry and its figures.
		f.fx.Exec(t, `DELETE FROM issue_budget_entry WHERE budget_root_id=$1 AND seq>1`, c.root)
		ledger.set(t, 1, issueBudgetLedgerFigures{ReservedTicks: 100})
		repaired := f.reconcile(t, c.root, cause+": after the repair")
		requireIssueBudgetLifecycleResult(t, repaired.result, "", []issueBudgetWantClosed{
			{Attempt: c.attempt, Task: util.UUIDToString(c.task.ID), State: "cancelled", Held: true},
		}, cause+": after the repair")
		f.requireLifecycleHeld(t, c, "cancelled", cause+": after the repair")
		for _, line := range logs() {
			if strings.Contains(line, "level=ERROR") {
				t.Fatalf("%s: the run after the repair logged %s", cause, line)
			}
		}
	}
}

// L06 (10.6, the part of the reconciliation). A root whose switch is off is
// not reconciled: the run takes the root lock, reads the mode and ends. It
// does not load the ledger and does not list an attempt. Once the root
// observes again, the next run writes the hold of the task that ended in
// between.
func TestIssueBudgetLifecycleLeavesARootThatIsOff(t *testing.T) {
	f := newIssueBudgetFixture(t)
	ctx := context.Background()
	agent, runtime := f.agent(t)
	c := f.lifecycleClaim(t, agent, runtime, "off")
	f.endTask(t, c.task, "completed", nil)
	off, err := f.svc.SetMode(ctx, f.manage(c.root, 1), IssueBudgetModeOff)
	if err != nil {
		t.Fatalf("switch the root off: %v", err)
	}
	before := f.snapshot(t)

	run := f.reconcile(t, c.root, "off")
	requireIssueBudgetLifecycleResult(t, run.result, IssueBudgetLifecycleSkippedOff, nil, "off")
	requireIssueBudgetLifecycleSent(t, run, []string{issueBudgetLevelStatement, "LockIssueBudgetRoot"}, "off")
	if run.loads != 0 {
		t.Fatalf("the run on a root that is off loaded its ledger (%d statements)", run.loads)
	}
	f.requireUnchanged(t, before, "the run on a root that is off")

	if _, err := f.svc.SetMode(ctx, f.manage(c.root, int64(off.PolicyRevision)), IssueBudgetModeObserve); err != nil {
		t.Fatalf("switch the root back to observe: %v", err)
	}
	again := f.reconcile(t, c.root, "observing again")
	requireIssueBudgetLifecycleResult(t, again.result, "", []issueBudgetWantClosed{
		{Attempt: c.attempt, Task: util.UUIDToString(c.task.ID), State: "process_exit", Held: true},
	}, "observing again")
	// The attempt keeps the policy it was judged under at its claim.
	f.requireLifecycleHeld(t, c, "process_exit", "observing again")
}

// L07. A root that does not exist, and a root of another workspace, are not
// found. Nothing is written.
func TestIssueBudgetLifecycleOfAnUnknownRootIsNotFound(t *testing.T) {
	f := newIssueBudgetFixture(t)
	ctx := context.Background()
	agent, runtime := f.agent(t)
	c := f.lifecycleClaim(t, agent, runtime, "not found")
	f.endTask(t, c.task, "completed", nil)
	before := f.snapshot(t)
	for name, ids := range map[string][2]pgtype.UUID{
		"a root that does not exist":  {f.ws, util.MustParseUUID(uuid.NewString())},
		"a root of another workspace": {util.MustParseUUID(uuid.NewString()), c.rootID},
	} {
		_, err := f.svc.ReconcileLifecycle(ctx, ids[0], ids[1])
		requireIssueBudgetCode(t, err, IssueBudgetNotFound, name)
	}
	f.requireUnchanged(t, before, "reconciliations that found no root")
}

// L08 (10.4). The ledger a reconciliation appended to is published once the
// commit of its transaction is confirmed, and not before. A run that went to
// the end and then lost its transaction leaves the rows and the cache with
// what is committed, and the next run writes the hold as a first run does.
func TestIssueBudgetLifecycleThatDoesNotCommitPublishesNothing(t *testing.T) {
	f := newIssueBudgetFixture(t)
	ctx := context.Background()
	agent, runtime := f.agent(t)
	logs := issueBudgetClaimLog(t)
	c := f.lifecycleClaim(t, agent, runtime, "lost commit")
	f.endTask(t, c.task, "cancelled", nil)
	before := f.snapshot(t)
	requireIssueBudgetCachedFigures(t, c.rootID, 1, issueBudgetLedgerFigures{ReservedTicks: 100}, "premise: the claim published its ledger")

	_, err := NewIssueBudgetService(f.q, issueBudgetLostCommit{TxStarter: f.pool}).ReconcileLifecycle(ctx, f.ws, c.rootID)
	if !errors.Is(err, errIssueBudgetLostCommit) {
		t.Fatalf("the run whose commit did not go through ended with %v, want the commit error", err)
	}
	f.requireUnchanged(t, before, "the run whose commit did not go through")
	requireIssueBudgetCachedFigures(t, c.rootID, 1, issueBudgetLedgerFigures{ReservedTicks: 100}, "the run whose commit did not go through")

	run := f.reconcile(t, c.root, "the run after the lost commit")
	requireIssueBudgetLifecycleResult(t, run.result, "", []issueBudgetWantClosed{
		{Attempt: c.attempt, Task: util.UUIDToString(c.task.ID), State: "cancelled", Held: true},
	}, "the run after the lost commit")
	f.requireLifecycleHeld(t, c, "cancelled", "the run after the lost commit")
	// Served from the cache: a state published ahead of the root row would
	// have been dropped here with a warning.
	if run.loads != 0 {
		t.Fatalf("the run after the lost commit loaded the ledger (%d statements); the cache held the committed revision", run.loads)
	}
	if lines := logs(); len(lines) != 0 {
		t.Fatalf("the runs logged %q", lines)
	}
}

// issueBudgetLifecycleRows is the reason table of the lifecycle
// reconciliation: every final status a task row can have and every failure
// reason the code writes, with the state the attempt of that claim ends in.
// row names the row of the table in the step 5c report. want is empty for a
// status that is not final: the attempt stays open.
var issueBudgetLifecycleRows = []struct {
	row     string
	status  string
	failure any // nil: the column is NULL
	want    string
	errText any // the error column; nil: NULL
}{
	{"3: completed", "completed", nil, "process_exit", nil},
	{"3: completed, with a reason left on the row", "completed", "timeout", "process_exit", nil},
	{"4: cancelled", "cancelled", nil, "cancelled", nil},
	{"4: cancelled, with the text of the caller", "cancelled", "the operator stopped the run", "cancelled", nil},
	{"4: cancelled, with a text that is a failure reason", "cancelled", "runtime_offline", "cancelled", nil},
	{"5: failed, reported as cancelled by the daemon", "failed", "cancelled", "cancelled", nil},
	{"6: failed, agent timeout", "failed", "agent_error.agent_timeout", "timeout", nil},
	{"6: failed, runtime CLI timeout", "failed", "runtime_cli_timeout", "timeout", nil},
	{"6: failed, idle watchdog", "failed", "idle_watchdog", "timeout", nil},
	{"6: failed, Codex semantic inactivity", "failed", "codex_semantic_inactivity", "timeout", nil},
	{"7: failed, runtime offline", "failed", "runtime_offline", "lost_contact", nil},
	{"7: failed, runtime recovery", "failed", "runtime_recovery", "lost_contact", nil},
	{"7: failed, runtime reconnect timeout", "failed", "runtime_reconnect_timeout", "lost_contact", nil},
	{"8: failed, a timeout a daemon reported", "failed", "timeout", "timeout", "agent execution timed out after 2h0m0s"},
	{"8: failed, a timeout without an error text", "failed", "timeout", "timeout", nil},
	{"8: failed, a timeout with an empty error text", "failed", "timeout", "timeout", ""},
	{"8: failed, a timeout whose text is the sweeper's in another case", "failed", "timeout", "timeout", "Task timed out"},
	{"8: failed, a timeout whose text begins with the sweeper's", "failed", "timeout", "timeout", "task timed out after 30m"},
	{"8: failed, the timeout of the sweeper", "failed", "timeout", "lost_contact", "task timed out"},
	{"10: failed, the sweeper's text beside another reason", "failed", "agent_error", "process_exit", "task timed out"},
	{"3: completed, with the sweeper's reason and text left on the row", "completed", "timeout", "process_exit", "task timed out"},
	{"9: failed, expired in the queue", "failed", "queued_expired", "lost_contact", nil},
	{"10: failed, the default of FailAgentTask", "failed", "agent_error", "process_exit", nil},
	{"10: failed, provider auth", "failed", "agent_error.provider_auth_or_access", "process_exit", nil},
	{"10: failed, provider quota", "failed", "agent_error.provider_quota_limit", "process_exit", nil},
	{"10: failed, provider capacity", "failed", "agent_error.provider_capacity_or_rate_limit", "process_exit", nil},
	{"10: failed, provider server error", "failed", "agent_error.provider_server_error", "process_exit", nil},
	{"10: failed, provider network", "failed", "agent_error.provider_network", "process_exit", nil},
	{"10: failed, process failure", "failed", "agent_error.process_failure", "process_exit", nil},
	{"10: failed, empty output", "failed", "agent_error.empty_or_unparseable_output", "process_exit", nil},
	{"10: failed, context overflow", "failed", "agent_error.context_overflow", "process_exit", nil},
	{"10: failed, missing config", "failed", "agent_error.missing_config", "process_exit", nil},
	{"10: failed, model not found", "failed", "agent_error.model_not_found_or_unavailable", "process_exit", nil},
	{"10: failed, runtime version", "failed", "agent_error.runtime_version_unsupported", "process_exit", nil},
	{"10: failed, missing executable", "failed", "agent_error.runtime_missing_executable", "process_exit", nil},
	{"10: failed, unknown agent error", "failed", "agent_error.unknown", "process_exit", nil},
	{"10: failed, iteration limit", "failed", "iteration_limit", "process_exit", nil},
	{"10: failed, agent blocked", "failed", "agent_blocked", "process_exit", nil},
	{"10: failed, invalid API request", "failed", "api_invalid_request", "process_exit", nil},
	{"10: failed, skill bundle unavailable", "failed", "skill_bundle_unavailable", "process_exit", nil},
	{"10: failed, invalid task identity", "failed", "invalid_task_identity", "process_exit", nil},
	{"10: failed, knowledge context changed", "failed", "knowledge_context_changed", "process_exit", nil},
	{"10: failed, knowledge authorization denied", "failed", "knowledge_authorization_denied", "process_exit", nil},
	{"10: failed, knowledge unavailable", "failed", "knowledge_unavailable", "process_exit", nil},
	{"10: failed, knowledge busy", "failed", "knowledge_busy", "process_exit", nil},
	{"10: failed, knowledge invocation unsupported", "failed", "knowledge_invocation_unsupported", "process_exit", nil},
	{"10: failed, local directory error", "failed", "local_directory_error", "process_exit", nil},
	{"10: failed, agent fallback message", "failed", "agent_fallback_message", "process_exit", nil},
	{"10: failed, Codex resume oversized", "failed", "codex_resume_oversized", "process_exit", nil},
	{"11: failed, no reason", "failed", nil, "lost_contact", nil},
	{"11: failed, an empty reason", "failed", "", "lost_contact", nil},
	{"11: failed, a text no build knows", "failed", "a reason no build of the server knows", "lost_contact", nil},
	{"11: failed, a known reason in another case", "failed", "Agent_Error", "lost_contact", nil},
	{"not final: queued", "queued", nil, "", nil},
	{"not final: dispatched", "dispatched", nil, "", nil},
	{"not final: running", "running", nil, "", nil},
	{"not final: waiting for a local directory", "waiting_local_directory", nil, "", nil},
	{"not final: deferred", "deferred", nil, "", nil},
}

// L09. Every row of the reason table. The attempt of the claim a final status
// speaks about ends in the state the table gives, with a hold of the same
// reason; the attempt of a task that is not final stays open.
//
// Two guards keep the table whole. Every status the CHECK of the task table
// allows is named by a row, and every reason of the taskfailure taxonomy is
// named by a row and placed in the product's map, so a status or a reason
// added later fails here until somebody decides how its attempt ends.
func TestIssueBudgetLifecycleReasonTable(t *testing.T) {
	f := newIssueBudgetFixture(t)
	agent, runtime := f.agent(t)

	statuses, failures := issueBudgetLifecycleRowSets()
	var check string
	f.fx.QueryRow(t, `SELECT pg_get_constraintdef(oid) FROM pg_constraint WHERE conrelid='agent_task_queue'::regclass AND conname='agent_task_queue_status_check'`).Scan(&check)
	allowed := map[string]bool{}
	for _, m := range regexp.MustCompile(`'([a-z_]+)'::text`).FindAllStringSubmatch(check, -1) {
		allowed[m[1]] = true
	}
	if len(allowed) == 0 || !reflect.DeepEqual(allowed, statuses) {
		t.Fatalf("the task table allows the statuses %v and the reason table names %v; a status has to be final with a state, or not final\nCHECK: %s", allowed, statuses, check)
	}
	for _, reason := range taskfailure.AllReasons() {
		if !failures[string(reason)] {
			t.Errorf("failure reason %q of the taskfailure taxonomy has no row in the reason table", reason)
		}
		if _, ok := issueBudgetFailureEnds[string(reason)]; !ok {
			t.Errorf("failure reason %q of the taskfailure taxonomy is not placed in issueBudgetFailureEnds", reason)
		}
	}
	for reason := range issueBudgetFailureEnds {
		if !failures[reason] {
			t.Errorf("failure reason %q is placed in issueBudgetFailureEnds and has no row in the reason table", reason)
		}
	}
	if t.Failed() {
		t.FailNow()
	}

	for _, tc := range issueBudgetLifecycleRows {
		c := f.lifecycleClaim(t, agent, runtime, tc.row)
		if tc.want == "" {
			f.fx.Exec(t, `UPDATE agent_task_queue SET status=$2 WHERE id=$1`, util.UUIDToString(c.task.ID), tc.status)
			before := f.snapshot(t)
			run := f.reconcile(t, c.root, tc.row)
			requireIssueBudgetLifecycleResult(t, run.result, "", nil, tc.row)
			f.requireUnchanged(t, before, "row "+tc.row)
			// Out of the way of the next row's task: one issue and agent
			// have one queued or dispatched task.
			f.endTask(t, c.task, "cancelled", nil)
			continue
		}
		f.endTaskWithError(t, c.task, tc.status, tc.failure, tc.errText)
		run := f.reconcile(t, c.root, tc.row)
		requireIssueBudgetLifecycleResult(t, run.result, "", []issueBudgetWantClosed{
			{Attempt: c.attempt, Task: util.UUIDToString(c.task.ID), State: tc.want, Held: true},
		}, "row "+tc.row)
		f.requireLifecycleHeld(t, c, tc.want, "row "+tc.row)
	}
}

// L10 (10.3, the supplement of 2026-11-19; row 2 of the reason table). A final
// status speaks about the last claim of its task. An open attempt of an
// earlier claim was superseded by a claim that wrote no attempt of its own, so
// it ends as superseded_unproven whatever the status says: the fact, the
// state and the entry identity the finalization of the next claim would have
// written. The attempt of the claim the row ended at ends by the table.
func TestIssueBudgetLifecycleSupersedesTheAttemptOfAnEarlierClaim(t *testing.T) {
	f := newIssueBudgetFixture(t)
	ctx := context.Background()
	agent, runtime := f.agent(t)

	t.Run("the root was off at the second claim", func(t *testing.T) {
		c := f.lifecycleClaim(t, agent, runtime, "off at the second claim")
		off, err := f.svc.SetMode(ctx, f.manage(c.root, 1), IssueBudgetModeOff)
		if err != nil {
			t.Fatalf("switch the root off: %v", err)
		}
		second := f.claimedAgain(t, c.task)
		if got := f.mustFinalize(t, f.claims(f.pool), second, TaskClaimBudgetProtocolV1); got != "" {
			t.Fatalf("premise: the claim on the root that is off got attempt %s", got)
		}
		f.requireLifecycleAttempt(t, c, "open", "premise: after the claim on the root that is off")
		if _, err := f.svc.SetMode(ctx, f.manage(c.root, int64(off.PolicyRevision)), IssueBudgetModeObserve); err != nil {
			t.Fatalf("switch the root back to observe: %v", err)
		}
		f.endTask(t, second, "failed", "agent_error")
		if row := f.taskRow(t, util.UUIDToString(c.task.ID)); row.Status != "failed" || row.ClaimGeneration != 2 {
			t.Fatalf("premise: the task is %s at generation %d, want failed at 2", row.Status, row.ClaimGeneration)
		}

		run := f.reconcile(t, c.root, "off at the second claim")
		requireIssueBudgetLifecycleResult(t, run.result, "", []issueBudgetWantClosed{
			{Attempt: c.attempt, Task: util.UUIDToString(c.task.ID), State: "superseded_unproven", Held: true},
		}, "off at the second claim")
		f.requireLifecycleHeld(t, c, "superseded_unproven", "off at the second claim")
	})

	t.Run("the same attempt when the row ended at its own claim", func(t *testing.T) {
		c := f.lifecycleClaim(t, agent, runtime, "ended at generation 1")
		f.endTask(t, c.task, "failed", "agent_error")
		run := f.reconcile(t, c.root, "ended at generation 1")
		requireIssueBudgetLifecycleResult(t, run.result, "", []issueBudgetWantClosed{
			{Attempt: c.attempt, Task: util.UUIDToString(c.task.ID), State: "process_exit", Held: true},
		}, "ended at generation 1")
		f.requireLifecycleHeld(t, c, "process_exit", "ended at generation 1")
	})

	// The second claim never got an attempt because its finalization failed
	// with its transaction; the row then ended without being claimed again.
	// Whatever it ended as, the first attempt was superseded.
	for _, end := range []struct {
		name    string
		status  string
		failure any
	}{
		{"completed", "completed", nil},
		{"cancelled", "cancelled", nil},
		{"failed for a timeout", "failed", "agent_error.agent_timeout"},
		{"failed for a runtime that went offline", "failed", "runtime_offline"},
		{"failed for a reason no build knows", "failed", "something else"},
	} {
		t.Run("a later claim without an attempt, then "+end.name, func(t *testing.T) {
			c := f.lifecycleClaim(t, agent, runtime, end.name)
			second := f.claimedAgain(t, c.task)
			f.endTask(t, second, end.status, end.failure)
			run := f.reconcile(t, c.root, end.name)
			requireIssueBudgetLifecycleResult(t, run.result, "", []issueBudgetWantClosed{
				{Attempt: c.attempt, Task: util.UUIDToString(c.task.ID), State: "superseded_unproven", Held: true},
			}, end.name)
			f.requireLifecycleHeld(t, c, "superseded_unproven", end.name)
		})
	}
}

// L11. One run holds every ended attempt of the root in one batch: one close
// for each, one entry for each hold, and one update of the root, whose figures
// are the projection of all of them. An attempt whose task still runs is left
// open in the same run.
func TestIssueBudgetLifecycleHoldsEveryEndedAttemptOfARootInOneBatch(t *testing.T) {
	f := newIssueBudgetFixture(t)
	agent, runtime := f.agent(t)
	enabled, _, child := f.budgetTree(t, 0, 100)
	root := enabled.BudgetRootID
	type claim struct {
		task    db.AgentTaskQueue
		attempt string
		state   string
	}
	var claims []claim
	for _, end := range []struct {
		status  string
		failure any
		state   string
	}{
		{"cancelled", nil, "cancelled"},
		{"failed", "idle_watchdog", "timeout"},
		{"completed", nil, "process_exit"},
		{"failed", "runtime_offline", "lost_contact"},
		{"dispatched", nil, "open"},
	} {
		task := f.claimed(t, agent, runtime, child, 1)
		attempt := f.mustFinalize(t, f.claims(f.pool), task, TaskClaimBudgetProtocolV1)
		if end.state != "open" {
			f.endTask(t, task, end.status, end.failure)
		}
		claims = append(claims, claim{task: task, attempt: attempt, state: end.state})
	}
	f.requireRootLedger(t, root, 5, issueBudgetLedgerFigures{ReservedTicks: 500}, "premise: five claims")

	run := f.reconcile(t, root, "the batch")
	var want []issueBudgetWantClosed
	for _, c := range claims[:4] {
		want = append(want, issueBudgetWantClosed{Attempt: c.attempt, Task: util.UUIDToString(c.task.ID), State: c.state, Held: true})
	}
	requireIssueBudgetLifecycleResult(t, run.result, "", want, "the batch")
	requireIssueBudgetLifecycleSent(t, run, []string{
		issueBudgetLevelStatement, "LockIssueBudgetRoot", "ListIssueBudgetOpenAttemptsOfRoot",
		"CloseIssueBudgetAttempt", "CloseIssueBudgetAttempt", "CloseIssueBudgetAttempt", "CloseIssueBudgetAttempt",
		"InsertIssueBudgetEntry", "InsertIssueBudgetEntry", "InsertIssueBudgetEntry", "InsertIssueBudgetEntry",
		"UpdateIssueBudgetRootLedger",
	}, "the batch")
	f.requireRootLedger(t, root, 9, issueBudgetLedgerFigures{ReservedTicks: 100, UnknownHoldTicks: 400, UnknownCount: 4}, "the batch")
	f.requireNothingReleased(t, root, "the batch")
	counts := f.replayedLedger(t, root).Projection().UnknownCounts()
	if wantCounts := (map[budget.UnknownReason]int{"cancelled": 1, "timeout": 1, "process_exit": 1, "lost_contact": 1}); !reflect.DeepEqual(counts, wantCounts) {
		t.Fatalf("a replay of the root's entries counts the unknown items %v, want %v", counts, wantCounts)
	}
	for _, c := range claims {
		rows := f.attempts(t, c.task)
		if len(rows) != 1 || rows[0].ID != c.attempt || rows[0].State != c.state {
			t.Fatalf("the attempt of the task that ended as %s is %+v, want state %s", c.state, rows, c.state)
		}
		holds := f.holdsOf(t, root, c.attempt)
		if c.state == "open" && len(holds) != 0 || c.state != "open" && !reflect.DeepEqual(holds, []string{"hold/" + c.attempt + "/" + c.state}) {
			t.Fatalf("the attempt in state %s has the holds %v", c.state, holds)
		}
	}
	requireIssueBudgetCachedFigures(t, util.MustParseUUID(root), 9, issueBudgetLedgerFigures{ReservedTicks: 100, UnknownHoldTicks: 400, UnknownCount: 4}, "the batch")
}

// L12 (B38, "a cancellation does not overwrite a fact that was consumed"). The
// reconciliation looks at open attempts and at no other. An attempt that left
// 'open' before, settled by a usage report or closed with a hold reason, keeps
// its state when its task ends afterwards and gets no hold, although the
// ledger still has its reservation.
//
// The state is set on the row alone. No code of this slice settles an
// attempt; the writer that will does it under the root lock, which is all the
// reconciliation relies on.
func TestIssueBudgetLifecycleLeavesAnAttemptThatIsNotOpen(t *testing.T) {
	f := newIssueBudgetFixture(t)
	agent, runtime := f.agent(t)
	for _, state := range []string{"settled", "cancelled", "timeout", "process_exit", "lost_contact", "superseded_unproven"} {
		c := f.lifecycleClaim(t, agent, runtime, state)
		f.fx.Exec(t, `UPDATE issue_budget_attempt SET state=$2 WHERE id=$1`, c.attempt, state)
		f.endTask(t, c.task, "failed", "agent_error.agent_timeout")
		before := f.snapshot(t)

		run := f.reconcile(t, c.root, state)
		requireIssueBudgetLifecycleResult(t, run.result, "", nil, "an attempt that is "+state)
		requireIssueBudgetLifecycleSent(t, run, []string{issueBudgetLevelStatement, "LockIssueBudgetRoot", "ListIssueBudgetOpenAttemptsOfRoot"}, "an attempt that is "+state)
		f.requireUnchanged(t, before, "the run over an attempt that is "+state)
		if holds := f.holdsOf(t, c.root, c.attempt); len(holds) != 0 {
			t.Fatalf("an attempt that is %s got the holds %v", state, holds)
		}
	}
}

// issueBudgetLifecycleRowSets are the statuses the reason table names, and the
// failure reasons it names for a failed task.
func issueBudgetLifecycleRowSets() (statuses, failures map[string]bool) {
	statuses, failures = map[string]bool{}, map[string]bool{}
	for _, tc := range issueBudgetLifecycleRows {
		statuses[tc.status] = true
		if reason, ok := tc.failure.(string); ok && tc.status == "failed" {
			failures[reason] = true
		}
	}
	return statuses, failures
}

// issueBudgetTaskUpdate is one UPDATE of the task table in the query files:
// the statement it stands in and what its SET clause assigns to the three
// columns the reason table reads, as the text of the query file.
type issueBudgetTaskUpdate struct {
	Statement             string
	Status, Reason, Error string
}

// issueBudgetTaskUpdates reads every UPDATE of agent_task_queue out of the
// sqlc query files. A column the statement does not assign is empty.
func issueBudgetTaskUpdates(t *testing.T) []issueBudgetTaskUpdate {
	t.Helper()
	files, err := filepath.Glob(filepath.Join("..", "..", "pkg", "db", "queries", "*.sql"))
	if err != nil || len(files) == 0 {
		t.Fatalf("the sqlc query files were not found: %v", err)
	}
	var (
		header     = regexp.MustCompile(`(?m)^-- name: (\w+) :\w+[ \t]*$`)
		comment    = regexp.MustCompile(`(?m)^[ \t]*--.*$`)
		update     = regexp.MustCompile(`(?is)\bUPDATE\s+agent_task_queue\b`)
		setClause  = regexp.MustCompile(`(?is)\bSET\b(.*?)(?:\n[ \t]*(?:FROM|WHERE|RETURNING)\b|\z)`)
		assignment = regexp.MustCompile(`(?s)^(\w+)\s*=\s*(.*)$`)
		blanks     = regexp.MustCompile(`\s+`)
	)
	// assignments splits a SET clause at the commas that stand outside every
	// parenthesis and every quoted text, and returns what each column is set
	// to, with runs of white space made one blank.
	assignments := func(clause string) map[string]string {
		out := map[string]string{}
		depth, quoted, from := 0, false, 0
		cut := func(to int) {
			if m := assignment.FindStringSubmatch(strings.TrimSpace(clause[from:to])); m != nil {
				out[m[1]] = blanks.ReplaceAllString(strings.TrimSpace(m[2]), " ")
			}
			from = to + 1
		}
		for i, r := range clause {
			switch {
			case r == '\'':
				quoted = !quoted
			case quoted:
			case r == '(':
				depth++
			case r == ')':
				depth--
			case r == ',' && depth == 0:
				cut(i)
			}
		}
		cut(len(clause))
		return out
	}
	var updates []issueBudgetTaskUpdate
	for _, file := range files {
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		text := string(raw)
		heads := header.FindAllStringSubmatchIndex(text, -1)
		for i, head := range heads {
			end := len(text)
			if i+1 < len(heads) {
				end = heads[i+1][0]
			}
			name := text[head[2]:head[3]]
			body := comment.ReplaceAllString(text[head[1]:end], "")
			starts := update.FindAllStringIndex(body, -1)
			for j, start := range starts {
				stop := len(body)
				if j+1 < len(starts) {
					stop = starts[j+1][0]
				}
				set := setClause.FindStringSubmatch(body[start[1]:stop])
				if set == nil {
					t.Fatalf("%s: an UPDATE of agent_task_queue without a SET clause this test can read", name)
				}
				set1 := assignments(set[1])
				updates = append(updates, issueBudgetTaskUpdate{
					Statement: name,
					Status:    set1["status"], Reason: set1["failure_reason"], Error: set1["error"],
				})
			}
		}
	}
	if len(updates) < 20 {
		t.Fatalf("this test read %d UPDATEs of agent_task_queue out of %d query files; it no longer understands them", len(updates), len(files))
	}
	return updates
}

// L13 (10.3, the supplement of 2026-11-20; needs no database). Two guards on
// what the server itself writes into a task row, read from the query files:
//
// Every statement that fails a task writes a failure reason the reason table
// places, so a statement added later that fails a task turns this test red
// until its reason has a row. The status of a task row is only ever written
// as a literal, so no statement fails a task through a parameter this test
// cannot see.
//
// The failure reason 'timeout' is written by one statement, FailStaleTasks,
// beside the error text "task timed out". That text is all that tells the
// sweeper's row from a timeout a daemon reported (row 8), so a change of it
// has to turn a test red: this one for the query file, and the case of the
// five facts that runs the statement for what the reconciliation makes of it.
func TestIssueBudgetLifecycleReasonTablePlacesEveryReasonTheServerWrites(t *testing.T) {
	_, failures := issueBudgetLifecycleRowSets()
	literal := regexp.MustCompile(`^'([^']*)'$`)
	fails := map[string]issueBudgetTaskUpdate{}
	for _, u := range issueBudgetTaskUpdates(t) {
		if u.Status == "" {
			continue
		}
		m := literal.FindStringSubmatch(u.Status)
		if m == nil {
			t.Errorf("%s writes the status of a task as %s; a status that is not a literal can end a task where this test does not see it", u.Statement, u.Status)
			continue
		}
		if m[1] != "failed" {
			continue
		}
		if _, twice := fails[u.Statement]; twice {
			t.Errorf("%s fails a task in two UPDATEs; this test reads one", u.Statement)
		}
		fails[u.Statement] = u
	}
	// The reason each statement writes, as the query file spells it. For
	// FailAgentTask it is the text a daemon sent, and this default without
	// one.
	want := map[string]string{
		"FailAgentTask":                      "COALESCE(sqlc.narg('failure_reason'), 'agent_error')",
		"RecoverOrphanedTasksForRuntime":     "'runtime_recovery'",
		"FailStaleTasks":                     "'timeout'",
		"ExpireStaleQueuedTasks":             "'queued_expired'",
		"FailExpiredRuntimeReconnectRetries": "'runtime_reconnect_timeout'",
		"FailTasksForOfflineRuntimes":        "'runtime_offline'",
	}
	got := map[string]string{}
	for name, u := range fails {
		got[name] = u.Reason
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("the statements that fail a task, with the failure reason each writes:\n got: %v\nwant: %v\na new one needs a row in the reason table and a line here", got, want)
	}
	quoted := regexp.MustCompile(`'([^']*)'`)
	for name, reason := range want {
		for _, m := range quoted.FindAllStringSubmatch(reason, -1) {
			if m[1] == "failure_reason" {
				continue // the name of the sqlc argument
			}
			if !failures[m[1]] {
				t.Errorf("%s writes the failure reason %q, which has no row in the reason table", name, m[1])
			}
			if _, ok := issueBudgetFailureEnds[m[1]]; !ok {
				t.Errorf("%s writes the failure reason %q, which is not placed in issueBudgetFailureEnds", name, m[1])
			}
		}
	}

	if got := fails["FailStaleTasks"].Error; got != "'task timed out'" {
		t.Errorf("FailStaleTasks writes the error text %s, want 'task timed out': row 8 of the reason table tells the sweeper's timeout from a daemon's by exactly this text", got)
	}
	for _, u := range issueBudgetTaskUpdates(t) {
		if strings.Contains(u.Reason, "'timeout'") && u.Statement != "FailStaleTasks" {
			t.Errorf("%s writes the failure reason 'timeout' as well; row 8 of the reason table knows one such statement and tells it by its error text", u.Statement)
		}
	}
}

// requireIssueBudgetLockNotAvailable fails unless err is the answer of a
// NOWAIT lock request that found the row locked.
func requireIssueBudgetLockNotAvailable(t *testing.T, err error, what string) {
	t.Helper()
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "55P03" {
		t.Fatalf("%s: got %v, want lock_not_available (55P03)", what, err)
	}
}
