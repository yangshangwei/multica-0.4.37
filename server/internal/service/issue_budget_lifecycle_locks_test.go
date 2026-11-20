package service

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/internal/budget"
	"github.com/multica-ai/multica/server/internal/iteration"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// The lifecycle reconciliation against other transactions (design section 5
// and 10.9): the locks it takes, the locks it does not wait for, and what two
// writers of one root leave when they queue on its row.
//
// As in issue_budget_finalize_locks_test.go, every step is ordered by lock
// state read from the server. Nothing is ordered by a sleep; a deadline only
// ends a run that would otherwise hang.
//
// The lock modes and the statements expected here are written from the
// design: one row lock on the root, plain reads of the task rows, and writes
// to the attempt, the entry and the root. They are not copied from a run.

// issueBudgetLifecycleWriter is a reconciliation running on another
// goroutine.
type issueBudgetLifecycleWriter struct {
	*issueBudgetWriter
	result *IssueBudgetLifecycleResult
}

// startReconcile runs the reconciliation of a root on another goroutine and
// returns once its transaction has begun.
func (f *issueBudgetFixture) startReconcile(t *testing.T, root string) *issueBudgetLifecycleWriter {
	t.Helper()
	result := new(IssueBudgetLifecycleResult)
	w := f.startWriter(t, "the reconciliation", func(tx TxStarter) issueBudgetWriterOutcome {
		got, err := NewIssueBudgetService(f.q, tx).ReconcileLifecycle(context.Background(), f.ws, util.MustParseUUID(root))
		*result = got
		return issueBudgetWriterOutcome{err: err}
	})
	return &issueBudgetLifecycleWriter{issueBudgetWriter: w, result: result}
}

// ended waits for the reconciliation, which has to commit, and returns what
// it reports and what it sent apart from the load of the ledger.
func (w *issueBudgetLifecycleWriter) ended(t *testing.T, what string) issueBudgetLifecycleRun {
	t.Helper()
	if out := w.outcome(t, what); out.err != nil {
		t.Fatalf("%s: %v", what, out.err)
	}
	run := issueBudgetLifecycleRun{result: *w.result}
	run.sent, run.loads = issueBudgetLifecycleSent(t, w.log.names(), what)
	return run
}

// issueBudgetTableLock is one lock on a table of the public schema.
type issueBudgetTableLock struct {
	Table, Mode string
}

// tableLocks reads the table locks a backend holds, in the order of the table
// and the mode names.
func (f *issueBudgetFixture) tableLocks(t *testing.T, pid uint32) []issueBudgetTableLock {
	t.Helper()
	rows, err := f.pool.Query(context.Background(), `
		SELECT c.relname, l.mode
		FROM pg_locks l
		JOIN pg_class c ON c.oid = l.relation
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE l.pid = $1 AND l.locktype = 'relation' AND l.granted
		  AND n.nspname = 'public' AND c.relkind IN ('r', 'p')
		ORDER BY c.relname, l.mode`, int32(pid))
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var locks []issueBudgetTableLock
	for rows.Next() {
		var lock issueBudgetTableLock
		if err := rows.Scan(&lock.Table, &lock.Mode); err != nil {
			t.Fatal(err)
		}
		locks = append(locks, lock)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return locks
}

// lockedNow asks, in a transaction of its own and without waiting, for the
// row locks of the rows a query names, and lets go at once. It returns how
// many rows it locked, or the error of a row somebody else holds.
func (f *issueBudgetFixture) lockedNow(t *testing.T, rows string, args ...any) (int, error) {
	t.Helper()
	ctx := context.Background()
	tx, err := f.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	var n int
	err = tx.QueryRow(ctx, `SELECT count(*) FROM (`+rows+` FOR UPDATE NOWAIT) locked`, args...).Scan(&n)
	return n, err
}

// L20 (section 5). The reconciliation takes the root lock and no other. Seen
// from the server with its transaction still open, after every statement of a
// run that had a hold to write:
//
//   - the task table is only read: the one table lock is the lock of a plain
//     SELECT, so no row of it is locked in any strength;
//   - the issue table, the member table, the operation table and the
//     observation table are not touched at all;
//   - no advisory lock is held, neither the workspace fence nor the binding
//     lock;
//   - the root row is locked, and the task row and the issue rows are free
//     for anybody who asks.
//
// The attempt and entry rows it wrote are locked by the write, as every
// written row is. They are only ever written under the root lock, so nobody
// waits for one of them without waiting for the root first.
func TestIssueBudgetLifecycleTakesTheRootLockAndNoOther(t *testing.T) {
	f := newIssueBudgetFixture(t)
	ctx := context.Background()
	agent, runtime := f.agent(t)
	c := f.lifecycleClaim(t, agent, runtime, "the locks of a run")
	f.endTask(t, c.task, "cancelled", nil)
	// Not served from the cache: the run is to send every statement a run can
	// send, the load of the ledger included.
	issueBudgetLedgers.forget(c.rootID, nil)

	observed := false
	log := &issueBudgetStatementLog{TxStarter: f.pool}
	log.beforeCommit = func(pid uint32) {
		observed = true
		wantTables := []issueBudgetTableLock{
			{"agent_task_queue", "AccessShareLock"},
			{"issue_budget_attempt", "AccessShareLock"},
			{"issue_budget_attempt", "RowExclusiveLock"},
			{"issue_budget_entry", "AccessShareLock"},
			{"issue_budget_entry", "RowExclusiveLock"},
			{"issue_budget_root", "RowExclusiveLock"},
			{"issue_budget_root", "RowShareLock"},
		}
		if got := f.tableLocks(t, pid); !reflect.DeepEqual(got, wantTables) {
			t.Errorf("table locks of the reconciliation = %+v, want %+v", got, wantTables)
		}
		if got := f.advisoryLocks(t, pid); len(got) != 0 {
			t.Errorf("advisory locks of the reconciliation = %+v, want none", got)
		}
		if n, err := f.lockedNow(t, `SELECT id FROM agent_task_queue WHERE id=$1`, util.UUIDToString(c.task.ID)); err != nil || n != 1 {
			t.Errorf("the task row could not be locked while the reconciliation was open: %d rows, %v", n, err)
		}
		if n, err := f.lockedNow(t, `SELECT id FROM issue WHERE id IN ($1, $2)`, c.parent, c.child); err != nil || n != 2 {
			t.Errorf("the issue rows could not be locked while the reconciliation was open: %d rows, %v", n, err)
		}
		if n, err := f.lockedNow(t, `SELECT issue_id FROM issue_budget_member WHERE budget_root_id=$1`, c.root); err != nil || n != 2 {
			t.Errorf("the member rows could not be locked while the reconciliation was open: %d rows, %v", n, err)
		}
		_, err := f.lockedNow(t, `SELECT id FROM issue_budget_root WHERE id=$1`, c.root)
		requireIssueBudgetLockNotAvailable(t, err, "the root row while the reconciliation is open")
	}
	result, err := NewIssueBudgetService(f.q, log).ReconcileLifecycle(ctx, f.ws, c.rootID)
	if err != nil {
		t.Fatalf("reconcile the root: %v", err)
	}
	if !observed {
		t.Fatal("the transaction of the reconciliation was never observed")
	}
	requireIssueBudgetLifecycleResult(t, result, "", []issueBudgetWantClosed{
		{Attempt: c.attempt, Task: util.UUIDToString(c.task.ID), State: "cancelled", Held: true},
	}, "the locks of a run")
	sent, loads := issueBudgetLifecycleSent(t, log.names(), "the locks of a run")
	if loads == 0 {
		t.Fatal("premise: the run was served the ledger from the cache and sent no load")
	}
	if want := []string{
		issueBudgetLevelStatement,
		"LockIssueBudgetRoot",
		"ListIssueBudgetOpenAttemptsOfRoot",
		"CloseIssueBudgetAttempt",
		"InsertIssueBudgetEntry",
		"UpdateIssueBudgetRootLedger",
	}; !reflect.DeepEqual(sent, want) {
		t.Fatalf("the reconciliation sent %v, want %v", sent, want)
	}
	f.requireLifecycleHeld(t, c, "cancelled", "the locks of a run")
}

// L21 (section 5). The reconciliation waits for nothing but the root. With the
// task row, the issue rows, the workspace fence and the binding lock all held
// by a transaction that does not end, it runs to its commit and writes the
// hold. This is what keeps it out of every lock cycle: a finalization holds a
// task row and the shared binding lock when it comes to the root, an issue
// writer holds the fence and issue rows when a binding comes to it.
func TestIssueBudgetLifecycleDoesNotWaitForATaskAnIssueOrABinder(t *testing.T) {
	f := newIssueBudgetFixture(t)
	ctx := context.Background()
	agent, runtime := f.agent(t)
	c := f.lifecycleClaim(t, agent, runtime, "beside the holder")
	f.endTask(t, c.task, "cancelled", nil)

	holder, err := f.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer holder.Rollback(ctx)
	holderPID := holder.Conn().PgConn().PID()
	if err := iteration.LockWorkspace(ctx, holder, f.ws); err != nil {
		t.Fatalf("take the workspace fence: %v", err)
	}
	if err := db.New(holder).LockIssueBudgetBinding(ctx, f.ws); err != nil {
		t.Fatalf("take the binding lock: %v", err)
	}
	var rows int
	if err := holder.QueryRow(ctx, `SELECT count(*) FROM (SELECT id FROM agent_task_queue WHERE id=$1 FOR UPDATE) locked`, util.UUIDToString(c.task.ID)).Scan(&rows); err != nil || rows != 1 {
		t.Fatalf("lock the task row: %d rows, %v", rows, err)
	}
	if err := holder.QueryRow(ctx, `SELECT count(*) FROM (SELECT id FROM issue WHERE id IN ($1, $2) FOR UPDATE) locked`, c.parent, c.child).Scan(&rows); err != nil || rows != 2 {
		t.Fatalf("lock the issue rows: %d rows, %v", rows, err)
	}
	held := []issueBudgetAdvisoryLock{issueBudgetBindingHeld, issueBudgetFenceHeld}
	if got := f.advisoryLocks(t, holderPID); !reflect.DeepEqual(got, held) {
		t.Fatalf("premise: the holder has the advisory locks %+v, want %+v", got, held)
	}

	// The deadline only ends a run that waits; the run is not timed.
	runCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	result, err := f.svc.ReconcileLifecycle(runCtx, f.ws, c.rootID)
	if err != nil {
		t.Fatalf("the reconciliation beside a holder of the task row, the issue rows, the fence and the binding lock: %v", err)
	}
	requireIssueBudgetLifecycleResult(t, result, "", []issueBudgetWantClosed{
		{Attempt: c.attempt, Task: util.UUIDToString(c.task.ID), State: "cancelled", Held: true},
	}, "beside the holder")

	// The holder had everything until the run was over.
	if got := f.advisoryLocks(t, holderPID); !reflect.DeepEqual(got, held) {
		t.Fatalf("after the run the holder has the advisory locks %+v, want %+v", got, held)
	}
	_, err = f.lockedNow(t, `SELECT id FROM agent_task_queue WHERE id=$1`, util.UUIDToString(c.task.ID))
	requireIssueBudgetLockNotAvailable(t, err, "the task row the holder locked, after the run")
	_, err = f.lockedNow(t, `SELECT id FROM issue WHERE id IN ($1, $2)`, c.parent, c.child)
	requireIssueBudgetLockNotAvailable(t, err, "the issue rows the holder locked, after the run")
	if err := holder.Rollback(ctx); err != nil {
		t.Fatalf("let go: %v", err)
	}
	f.requireLifecycleHeld(t, c, "cancelled", "beside the holder")
}

// L22 (B38; 10.9, "a finalization and the reconciliation at once: one hold
// for an attempt"), on the path a claim really takes. A task is claimed a
// second time while its first attempt is open; the finalization of that claim
// and a reconciliation of the root queue on the root row, in either order.
//
// The finalization holds the task row when it comes to the root. The
// reconciliation that got the root first reads that row without a lock, so it
// neither waits for the finalization nor is waited for in a cycle; it finds a
// task that is not final and writes nothing. Either way the first attempt gets
// one hold, written by the finalization that superseded it. When the task
// then ends, the next reconciliation holds what the second attempt reserved,
// once.
func TestIssueBudgetLifecycleAndTheFinalizationOfTheNextClaimLeaveOneHoldEach(t *testing.T) {
	f := newIssueBudgetFixture(t)
	agent, runtime := f.agent(t)
	for _, tc := range []struct {
		name           string
		reconcileFirst bool
	}{
		{name: "the reconciliation locks the root first", reconcileFirst: true},
		{name: "the finalization locks the root first"},
	} {
		func() {
			c := f.lifecycleClaim(t, agent, runtime, tc.name)
			second := f.claimedAgain(t, c.task)

			holder, release := f.holdRoot(t, c.root)
			defer release()
			var run *issueBudgetLifecycleWriter
			var claim *issueBudgetWriter
			if tc.reconcileFirst {
				run = f.startReconcile(t, c.root)
				if got := f.waitsBehind(t, run.pid, "the reconciliation"); !reflect.DeepEqual(got, []uint32{holder}) {
					t.Fatalf("%s: the reconciliation waits behind backends %v, want the holder of the root row, %d", tc.name, got, holder)
				}
				claim = f.startFinalize(t, second, TaskClaimBudgetProtocolV1)
				if got := f.waitsBehind(t, claim.pid, "the finalization"); !reflect.DeepEqual(got, []uint32{run.pid}) {
					t.Fatalf("%s: the finalization waits behind backends %v, want the reconciliation, %d", tc.name, got, run.pid)
				}
			} else {
				claim = f.startFinalize(t, second, TaskClaimBudgetProtocolV1)
				if got := f.waitsBehind(t, claim.pid, "the finalization"); !reflect.DeepEqual(got, []uint32{holder}) {
					t.Fatalf("%s: the finalization waits behind backends %v, want the holder of the root row, %d", tc.name, got, holder)
				}
				run = f.startReconcile(t, c.root)
				if got := f.waitsBehind(t, run.pid, "the reconciliation"); !reflect.DeepEqual(got, []uint32{claim.pid}) {
					t.Fatalf("%s: the reconciliation waits behind backends %v, want the finalization, %d", tc.name, got, claim.pid)
				}
			}
			run.requireStoppedAt(t, "LockIssueBudgetRoot", tc.name+": the reconciliation")
			claim.requireStoppedAt(t, "LockIssueBudgetRoot", tc.name+": the finalization")
			// The finalization holds the row of the task whose attempt the
			// reconciliation will read.
			_, err := f.lockedNow(t, `SELECT id FROM agent_task_queue WHERE id=$1`, util.UUIDToString(c.task.ID))
			requireIssueBudgetLockNotAvailable(t, err, tc.name+": the task row while its finalization waits for the root")

			release()
			reconciled := run.ended(t, tc.name+": the reconciliation")
			claimed := claim.outcome(t, tc.name+": the finalization")
			if claimed.err != nil || !claimed.finalized.BudgetAttemptID.Valid {
				t.Fatalf("%s: the finalization ended with %v and attempt %v; want it committed with an attempt", tc.name, claimed.err, claimed.finalized.BudgetAttemptID)
			}
			a2 := util.UUIDToString(claimed.finalized.BudgetAttemptID)
			// The task was claimed, not ended: nothing for the reconciliation.
			requireIssueBudgetLifecycleResult(t, reconciled.result, "", nil, tc.name)
			requireIssueBudgetLifecycleSent(t, reconciled, []string{issueBudgetLevelStatement, "LockIssueBudgetRoot", "ListIssueBudgetOpenAttemptsOfRoot"}, tc.name)

			f.requireRootLedger(t, c.root, 3, issueBudgetLedgerFigures{ReservedTicks: 100, UnknownHoldTicks: 100, UnknownCount: 1}, tc.name)
			if holds := f.holdsOf(t, c.root, c.attempt); !reflect.DeepEqual(holds, []string{"hold/" + c.attempt + "/superseded_unproven"}) {
				t.Fatalf("%s: the first attempt has the holds %v, want exactly the one of the claim that superseded it", tc.name, holds)
			}
			rows := f.attempts(t, c.task)
			if len(rows) != 2 || rows[0].ID != c.attempt || rows[0].State != "superseded_unproven" || rows[1].ID != a2 || rows[1].State != "open" {
				t.Fatalf("%s: the task has the attempts %+v, want %s superseded and %s open", tc.name, rows, c.attempt, a2)
			}

			// The task ends at its second claim.
			f.endTask(t, second, "failed", "agent_error")
			after := f.reconcile(t, c.root, tc.name+": after the task ended")
			requireIssueBudgetLifecycleResult(t, after.result, "", []issueBudgetWantClosed{
				{Attempt: a2, Task: util.UUIDToString(c.task.ID), State: "process_exit", Held: true},
			}, tc.name+": after the task ended")
			f.requireTwoHolds(t, c, a2, tc.name+": after the task ended")
		}()
	}
}

// requireTwoHolds fails unless the root of c is what is left when both
// attempts of its task have ended: the first superseded, the second by a
// process exit, each with exactly one hold, both amounts held and nothing
// released.
func (f *issueBudgetFixture) requireTwoHolds(t *testing.T, c issueBudgetLifecycleClaim, a2, what string) {
	t.Helper()
	f.requireRootLedger(t, c.root, 4, issueBudgetLedgerFigures{UnknownHoldTicks: 200, UnknownCount: 2}, what)
	f.requireNothingReleased(t, c.root, what)
	entries := f.entryRows(t, c.root)
	if len(entries) != 4 {
		t.Fatalf("%s: the root has %d entry rows, want two reservations and two holds", what, len(entries))
	}
	if holds := f.holdsOf(t, c.root, c.attempt); !reflect.DeepEqual(holds, []string{"hold/" + c.attempt + "/superseded_unproven"}) {
		t.Fatalf("%s: the first attempt has the holds %v, want exactly one", what, holds)
	}
	if holds := f.holdsOf(t, c.root, a2); !reflect.DeepEqual(holds, []string{"hold/" + a2 + "/process_exit"}) {
		t.Fatalf("%s: the second attempt has the holds %v, want exactly one", what, holds)
	}
	want := map[budget.UnknownReason]int{"superseded_unproven": 1, "process_exit": 1}
	if counts := f.replayedLedger(t, c.root).Projection().UnknownCounts(); !reflect.DeepEqual(counts, want) {
		t.Fatalf("%s: a replay of the root's entries counts the unknown items %v, want %v", what, counts, want)
	}
	rows := f.attempts(t, c.task)
	if len(rows) != 2 || rows[0].ID != c.attempt || rows[0].State != "superseded_unproven" || rows[1].ID != a2 || rows[1].State != "process_exit" {
		t.Fatalf("%s: the task has the attempts %+v, want %s superseded and %s ended by a process exit", what, rows, c.attempt, a2)
	}
	// And a run more changes nothing.
	before := f.snapshot(t)
	again := f.reconcile(t, c.root, what+": one run more")
	requireIssueBudgetLifecycleResult(t, again.result, "", nil, what+": one run more")
	f.requireUnchanged(t, before, what+": one run more")
}

// L23 (B38; 10.9), the case in which both writers want the same attempt. The
// row of a task ended at its second claim, its attempt of the first claim is
// open, and the budget step of that second claim runs against the
// reconciliation. Both would write hold/<attempt>/superseded_unproven with the
// same content.
//
// The case is built by a direct call of the budget step because a
// finalization cannot produce it: FinalizeTaskClaim holds the task row and
// goes on only while that row is dispatched, so the row cannot be final while
// the step runs. It is here because it is the only way two writers meet on one
// attempt, and the answer must not depend on the task lock: whoever gets the
// root second lists the attempts after the lock, does not find the first one
// open any more and writes no second hold.
func TestIssueBudgetLifecycleAndABudgetStepThatWantTheSameAttemptWriteOneHold(t *testing.T) {
	f := newIssueBudgetFixture(t)
	agent, runtime := f.agent(t)
	for _, tc := range []struct {
		name           string
		reconcileFirst bool
	}{
		{name: "the reconciliation locks the root first", reconcileFirst: true},
		{name: "the budget step locks the root first"},
	} {
		func() {
			c := f.lifecycleClaim(t, agent, runtime, tc.name)
			second := f.claimedAgain(t, c.task)
			f.endTask(t, second, "failed", "agent_error")
			startStep := func() *issueBudgetWriter {
				return f.startWriter(t, "the budget step", func(tx TxStarter) issueBudgetWriterOutcome {
					ctx := context.Background()
					stepTx, err := tx.Begin(ctx)
					if err != nil {
						return issueBudgetWriterOutcome{err: err}
					}
					defer stepTx.Rollback(ctx)
					claim, err := issueBudgetLedgers.finalizeTaskBudget(ctx, db.New(stepTx), true, second, TaskClaimBudgetProtocolV1)
					if err != nil {
						return issueBudgetWriterOutcome{err: err}
					}
					if err := stepTx.Commit(ctx); err != nil {
						return issueBudgetWriterOutcome{err: err}
					}
					claim.Committed()
					return issueBudgetWriterOutcome{finalized: TaskClaimFinalization{BudgetAttemptID: claim.AttemptID}}
				})
			}

			holder, release := f.holdRoot(t, c.root)
			defer release()
			var run *issueBudgetLifecycleWriter
			var step *issueBudgetWriter
			if tc.reconcileFirst {
				run = f.startReconcile(t, c.root)
				if got := f.waitsBehind(t, run.pid, "the reconciliation"); !reflect.DeepEqual(got, []uint32{holder}) {
					t.Fatalf("%s: the reconciliation waits behind backends %v, want the holder of the root row, %d", tc.name, got, holder)
				}
				step = startStep()
				if got := f.waitsBehind(t, step.pid, "the budget step"); !reflect.DeepEqual(got, []uint32{run.pid}) {
					t.Fatalf("%s: the budget step waits behind backends %v, want the reconciliation, %d", tc.name, got, run.pid)
				}
			} else {
				step = startStep()
				if got := f.waitsBehind(t, step.pid, "the budget step"); !reflect.DeepEqual(got, []uint32{holder}) {
					t.Fatalf("%s: the budget step waits behind backends %v, want the holder of the root row, %d", tc.name, got, holder)
				}
				run = f.startReconcile(t, c.root)
				if got := f.waitsBehind(t, run.pid, "the reconciliation"); !reflect.DeepEqual(got, []uint32{step.pid}) {
					t.Fatalf("%s: the reconciliation waits behind backends %v, want the budget step, %d", tc.name, got, step.pid)
				}
			}
			run.requireStoppedAt(t, "LockIssueBudgetRoot", tc.name+": the reconciliation")
			step.requireStoppedAt(t, "LockIssueBudgetRoot", tc.name+": the budget step")

			release()
			reconciled := run.ended(t, tc.name+": the reconciliation")
			stepped := step.outcome(t, tc.name+": the budget step")
			if stepped.err != nil || !stepped.finalized.BudgetAttemptID.Valid {
				t.Fatalf("%s: the budget step ended with %v and attempt %v; want it committed with an attempt", tc.name, stepped.err, stepped.finalized.BudgetAttemptID)
			}
			a2 := util.UUIDToString(stepped.finalized.BudgetAttemptID)

			if holds := f.holdsOf(t, c.root, c.attempt); !reflect.DeepEqual(holds, []string{"hold/" + c.attempt + "/superseded_unproven"}) {
				t.Fatalf("%s: the attempt both writers wanted has the holds %v, want exactly one", tc.name, holds)
			}
			task := util.UUIDToString(c.task.ID)
			if tc.reconcileFirst {
				// The reconciliation superseded the first attempt; the step
				// found it closed and only opened its own, which is open on a
				// task that has ended, for the next run.
				requireIssueBudgetLifecycleResult(t, reconciled.result, "", []issueBudgetWantClosed{
					{Attempt: c.attempt, Task: task, State: "superseded_unproven", Held: true},
				}, tc.name)
				entries := f.entryRows(t, c.root)
				if len(entries) != 3 {
					t.Fatalf("%s: the root has %d entry rows, want the reservation, the hold and the second reservation", tc.name, len(entries))
				}
				f.requireLifecycleHold(t, entries[1], 2, c.attempt, "superseded_unproven", tc.name)
				f.requireRootLedger(t, c.root, 3, issueBudgetLedgerFigures{ReservedTicks: 100, UnknownHoldTicks: 100, UnknownCount: 1}, tc.name)
				next := f.reconcile(t, c.root, tc.name+": the next run")
				requireIssueBudgetLifecycleResult(t, next.result, "", []issueBudgetWantClosed{
					{Attempt: a2, Task: task, State: "process_exit", Held: true},
				}, tc.name+": the next run")
			} else {
				// The step superseded the first attempt and opened its own;
				// the reconciliation found only that one open, on a task that
				// has ended.
				requireIssueBudgetLifecycleResult(t, reconciled.result, "", []issueBudgetWantClosed{
					{Attempt: a2, Task: task, State: "process_exit", Held: true},
				}, tc.name)
			}
			// The same in both orders.
			f.requireTwoHolds(t, c, a2, tc.name)
		}()
	}
}

// L24 (B38). Two reconciliations of one root on two connections queue on the
// root row. The first holds what the ended attempt reserved; the second lists
// the attempts once it has the lock, finds none open on an ended task and
// writes nothing.
func TestIssueBudgetLifecycleRunOnTwoConnectionsWritesOnce(t *testing.T) {
	f := newIssueBudgetFixture(t)
	agent, runtime := f.agent(t)
	c := f.lifecycleClaim(t, agent, runtime, "two runs")
	f.endTask(t, c.task, "cancelled", nil)

	holder, release := f.holdRoot(t, c.root)
	defer release()
	first := f.startReconcile(t, c.root)
	if got := f.waitsBehind(t, first.pid, "the first run"); !reflect.DeepEqual(got, []uint32{holder}) {
		t.Fatalf("the first run waits behind backends %v, want the holder of the root row, %d", got, holder)
	}
	second := f.startReconcile(t, c.root)
	if got := f.waitsBehind(t, second.pid, "the second run"); !reflect.DeepEqual(got, []uint32{first.pid}) {
		t.Fatalf("the second run waits behind backends %v, want the first run, %d", got, first.pid)
	}
	first.requireStoppedAt(t, "LockIssueBudgetRoot", "the first run")
	second.requireStoppedAt(t, "LockIssueBudgetRoot", "the second run")

	release()
	wrote := first.ended(t, "the first run")
	found := second.ended(t, "the second run")
	requireIssueBudgetLifecycleResult(t, wrote.result, "", []issueBudgetWantClosed{
		{Attempt: c.attempt, Task: util.UUIDToString(c.task.ID), State: "cancelled", Held: true},
	}, "the first run")
	requireIssueBudgetLifecycleSent(t, wrote, []string{
		issueBudgetLevelStatement, "LockIssueBudgetRoot", "ListIssueBudgetOpenAttemptsOfRoot",
		"CloseIssueBudgetAttempt", "InsertIssueBudgetEntry", "UpdateIssueBudgetRootLedger",
	}, "the first run")
	requireIssueBudgetLifecycleResult(t, found.result, "", nil, "the second run")
	requireIssueBudgetLifecycleSent(t, found, []string{issueBudgetLevelStatement, "LockIssueBudgetRoot", "ListIssueBudgetOpenAttemptsOfRoot"}, "the second run")
	f.requireLifecycleHeld(t, c, "cancelled", "two runs")
}
