package service

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/multica-ai/multica/server/internal/budget"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// The claims that need a second connection: a finalization against a binder
// (design 10.10), two finalizations on one root, and a finalization against a
// manual change of the same root (10.9).
//
// Every test here orders its steps by lock state it reads from the server: a
// writer is started, seen to wait, and only then is the next one started or
// the lock released. Nothing is ordered by a sleep.

// issueBudgetBackends is a TxStarter that reports the backend of every
// transaction it begins.
type issueBudgetBackends struct {
	TxStarter
	began chan uint32
}

func (s *issueBudgetBackends) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return nil, err
	}
	s.began <- tx.Conn().PgConn().PID()
	return tx, nil
}

// issueBudgetWriterOutcome is what a writer started on another goroutine
// ended with: finalized for a finalization, result for a management call.
type issueBudgetWriterOutcome struct {
	finalized TaskClaimFinalization
	result    IssueBudgetOperationResult
	err       error
}

// issueBudgetWriter is a writer running on another goroutine in one
// transaction, whose backend and statements the test can read.
type issueBudgetWriter struct {
	pid  uint32
	log  *issueBudgetStatementLog
	done <-chan issueBudgetWriterOutcome
}

// startWriter runs write on another goroutine and returns once its
// transaction has begun.
func (f *issueBudgetFixture) startWriter(t *testing.T, what string, write func(tx TxStarter) issueBudgetWriterOutcome) *issueBudgetWriter {
	t.Helper()
	backends := &issueBudgetBackends{TxStarter: f.pool, began: make(chan uint32, 8)}
	log := &issueBudgetStatementLog{TxStarter: backends}
	done := make(chan issueBudgetWriterOutcome, 1)
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		done <- write(log)
	}()
	f.awaitWriter(t, finished)
	select {
	case pid := <-backends.began:
		return &issueBudgetWriter{pid: pid, log: log, done: done}
	case out := <-done:
		t.Fatalf("%s ended before it began a transaction: %+v", what, out)
	case <-time.After(15 * time.Second):
		t.Fatalf("%s never began a transaction", what)
	}
	return nil
}

// startFinalize runs the real finalization of a claim on another goroutine.
func (f *issueBudgetFixture) startFinalize(t *testing.T, task db.AgentTaskQueue, declared TaskClaimBudgetProtocol) *issueBudgetWriter {
	t.Helper()
	token := f.token(t, task, f.workspace)
	return f.startWriter(t, "the finalization", func(tx TxStarter) issueBudgetWriterOutcome {
		finalized, err := f.claims(tx).FinalizeTaskClaim(context.Background(), task, token, nil, false, nil, declared)
		return issueBudgetWriterOutcome{finalized: finalized, err: err}
	})
}

// outcome waits for the writer to end.
func (w *issueBudgetWriter) outcome(t *testing.T, what string) issueBudgetWriterOutcome {
	t.Helper()
	select {
	case out := <-w.done:
		return out
	case <-time.After(30 * time.Second):
		t.Fatalf("%s did not end; it last sent %v", what, w.log.names())
	}
	return issueBudgetWriterOutcome{}
}

// requireStoppedAt fails unless the writer is still running and the last
// statement it sent is the one given.
func (w *issueBudgetWriter) requireStoppedAt(t *testing.T, statement, what string) {
	t.Helper()
	select {
	case out := <-w.done:
		t.Fatalf("%s ended while it was to wait: %+v", what, out)
	default:
	}
	if sent := w.log.names(); len(sent) == 0 || sent[len(sent)-1] != statement {
		t.Fatalf("%s last sent %v, want it stopped at %s", what, sent, statement)
	}
}

// waitsBehind waits until a backend waits for a lock and returns the backends
// in front of it: the holders of what it asks for, and whoever asked before
// it.
func (f *issueBudgetFixture) waitsBehind(t *testing.T, pid uint32, what string) []uint32 {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		var blockers []int32
		if err := f.pool.QueryRow(context.Background(), `SELECT pg_blocking_pids($1::integer)`, int32(pid)).Scan(&blockers); err != nil {
			t.Fatal(err)
		}
		if len(blockers) > 0 {
			out := make([]uint32, len(blockers))
			for i, blocker := range blockers {
				out[i] = uint32(blocker)
			}
			return out
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("%s never came to wait for a lock", what)
	return nil
}

// holdRoot locks a root row in a transaction of its own, as every writer of a
// root does, and returns its backend and the function that lets go. Letting
// go twice is harmless, so a test defers it and calls it.
func (f *issueBudgetFixture) holdRoot(t *testing.T, root string) (pid uint32, release func()) {
	t.Helper()
	ctx := context.Background()
	tx, err := f.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.New(tx).LockIssueBudgetRoot(ctx, db.LockIssueBudgetRootParams{WorkspaceID: f.ws, ID: util.MustParseUUID(root)}); err != nil {
		tx.Rollback(ctx)
		t.Fatalf("lock the root row: %v", err)
	}
	released := false
	return tx.Conn().PgConn().PID(), func() {
		if released {
			return
		}
		released = true
		if err := tx.Rollback(ctx); err != nil {
			t.Errorf("let go of the root row: %v", err)
		}
	}
}

// F05 (B35a), the refusal. While a binder holds the workspace's binding lock
// a claim is turned away at its first statement, with an error the handler
// can tell from a failed write. It leaves no budget row and no task token,
// and publishes nothing. The next claim of the task is a first attempt.
//
// The binder here is a move that puts an issue under a member; a root being
// enabled is the binder of the test of the window below.
func TestIssueBudgetFinalizeRefusedForABindingInProgressLeavesNothing(t *testing.T) {
	f := newIssueBudgetFixture(t)
	ctx := context.Background()
	enabled, parent, child := f.budgetTree(t, 0, 100)
	root := enabled.BudgetRootID
	subtree := f.issue(t, "Unbound subtree", "")
	agent, runtime := f.agent(t)
	task := f.claimed(t, agent, runtime, child, 1)
	quick := f.claimedQuickCreate(t, agent, runtime, parent, 1)
	before := f.snapshot(t)

	moved := f.setParent(t, subtree, child)
	observed := false
	f.inBindingHeld(t, func(q *db.Queries) error { return BindIssueBudgetOnReparent(ctx, q, moved) }, func(pid uint32) {
		observed = true
		if got := f.advisoryLocks(t, pid); !reflect.DeepEqual(got, []issueBudgetAdvisoryLock{issueBudgetBindingHeld, issueBudgetFenceHeld}) {
			t.Fatalf("premise: advisory locks of the binder = %+v, want the binding lock and the fence, both held", got)
		}
		for _, claim := range []struct {
			name string
			task db.AgentTaskQueue
			try  string
		}{
			{"a task of a member issue", task, "TryLockIssueBudgetBindingSharedForIssue"},
			{"a quick-create task under a member issue", quick, "TryLockIssueBudgetBindingShared"},
		} {
			log := &issueBudgetStatementLog{TxStarter: f.pool}
			finalized, err := f.finalize(t, f.claims(log), claim.task, TaskClaimBudgetProtocolV1)
			if !errors.Is(err, ErrIssueBudgetBindingInProgress) {
				t.Fatalf("%s: finalize while a binding is in progress = %+v, %v; want ErrIssueBudgetBindingInProgress", claim.name, finalized, err)
			}
			if finalized.BudgetAttemptID.Valid {
				t.Fatalf("%s: the refused finalization handed back attempt %s", claim.name, util.UUIDToString(finalized.BudgetAttemptID))
			}
			// The try lock is the first and the last statement of the step:
			// the member row is not read.
			requireIssueBudgetStep(t, log.names(), []string{claim.try}, claim.name)
			if got := f.tokens(t, claim.task); got != 0 {
				t.Fatalf("%s: the refused finalization left %d task tokens, want 0", claim.name, got)
			}
		}
		// The binder has not committed, so what is there to read is what was
		// there before it and before the two claims.
		f.requireUnchanged(t, before, "two refused finalizations")
		if revision, _, ok := issueBudgetLedgers.peek(util.MustParseUUID(root)); ok {
			t.Fatalf("a refused finalization left the root in the cache at revision %d; it never read a ledger", revision)
		}
	})
	if !observed {
		t.Fatal("the binder was never observed")
	}
	after := f.snapshot(t)
	for _, table := range []string{"issue_budget_attempt", "issue_budget_entry", "issue_budget_operation", "issue_budget_observation", "issue_budget_root"} {
		if before[table] != after[table] {
			t.Fatalf("after the binder committed %s is not what it was\nbefore: %s\nafter:  %s", table, before[table], after[table])
		}
	}

	// Both tasks go back to the queue and are claimed at the next generation.
	again := f.claimedAgain(t, task)
	attempt := f.mustFinalize(t, f.claims(f.pool), again, TaskClaimBudgetProtocolV1)
	f.requireAttempt(t, f.onlyAttempt(t, again, attempt, "the task of an issue after the binding"), attempt, issueBudgetWantAttempt{
		Root: enabled, Task: again, Issue: child, State: "open", Reserved: issueBudgetInt64(100), Protocol: issueBudgetInt32(1),
	}, "the task of an issue after the binding")
	quickAgain := f.claimedAgain(t, quick)
	quickAttempt := f.mustFinalize(t, f.claims(f.pool), quickAgain, TaskClaimBudgetProtocolV1)
	f.requireAttempt(t, f.onlyAttempt(t, quickAgain, quickAttempt, "the quick-create task after the binding"), quickAttempt, issueBudgetWantAttempt{
		Root: enabled, Task: quickAgain, ContextParent: parent, State: "open", Reserved: issueBudgetInt64(100), Protocol: issueBudgetInt32(1),
	}, "the quick-create task after the binding")
	operations := f.claimOperations(t, root)
	if len(operations) != 2 {
		t.Fatalf("two claims left %d operation rows, want 2", len(operations))
	}
	// Neither supersedes anything: the refused claims left no attempt.
	f.requireClaimOperation(t, operations[0], issueBudgetWantClaim{Root: root, Task: again, Attempt: attempt, Reservation: "sized", Reserved: "100"}, "the task of an issue after the binding")
	f.requireClaimOperation(t, operations[1], issueBudgetWantClaim{Root: root, Task: quickAgain, Attempt: quickAttempt, Reservation: "sized", Reserved: "100"}, "the quick-create task after the binding")
	entries := f.entryRows(t, root)
	if len(entries) != 2 {
		t.Fatalf("two claims left %d entry rows, want 2", len(entries))
	}
	accepted := budget.Receipt{Disposition: budget.DispositionAccepted}
	f.requireClaimEntry(t, entries[0], 1, issueBudgetReserveEntry(attempt, 100), accepted, operations[0].ID, "the first reservation after the binding")
	f.requireClaimEntry(t, entries[1], 2, issueBudgetReserveEntry(quickAttempt, 100), accepted, operations[1].ID, "the second reservation after the binding")
	f.requireRootLedger(t, root, 2, issueBudgetLedgerFigures{ReservedTicks: 200}, "after the two claims that followed the binding")
}

// F06 (B45), two claims at once. The limit is judged under the root lock, so
// two claims that arrive together are judged one after the other, each on
// what the one before it committed. Both wait for a root row a third
// connection holds; the second waits behind the first. Once the row is let
// go both commit: 100 is inside a limit of 150, 200 is over it, and exactly
// one of the two is told so.
func TestIssueBudgetFinalizeJudgesTwoClaimsThatQueueOnTheRootLockOneAfterTheOther(t *testing.T) {
	f := newIssueBudgetFixture(t)
	enabled, parent, child := f.budgetTree(t, 150, 100)
	root := enabled.BudgetRootID
	agent, runtime := f.agent(t)
	otherAgent, otherRuntime := f.agent(t)
	first := f.claimed(t, agent, runtime, child, 1)
	second := f.claimed(t, otherAgent, otherRuntime, parent, 1)

	holder, release := f.holdRoot(t, root)
	defer release()
	one := f.startFinalize(t, first, TaskClaimBudgetProtocolV1)
	if got := f.waitsBehind(t, one.pid, "the first finalization"); !reflect.DeepEqual(got, []uint32{holder}) {
		t.Fatalf("the first finalization waits behind backends %v, want the holder of the root row, %d", got, holder)
	}
	two := f.startFinalize(t, second, TaskClaimBudgetProtocolV1)
	if got := f.waitsBehind(t, two.pid, "the second finalization"); !reflect.DeepEqual(got, []uint32{one.pid}) {
		t.Fatalf("the second finalization waits behind backends %v, want the first finalization, %d", got, one.pid)
	}
	// The root lock is the last lock of a finalization: both stopped there,
	// with every other write of the claim sent and nothing committed.
	one.requireStoppedAt(t, "LockIssueBudgetRoot", "the first finalization")
	two.requireStoppedAt(t, "LockIssueBudgetRoot", "the second finalization")
	if got := f.count(t, "issue_budget_attempt"); got != 0 {
		t.Fatalf("%d attempt rows are committed while both finalizations wait", got)
	}

	release()
	a1, a2 := "", ""
	for _, claim := range []struct {
		name   string
		writer *issueBudgetWriter
		id     *string
	}{{"the first finalization", one, &a1}, {"the second finalization", two, &a2}} {
		out := claim.writer.outcome(t, claim.name)
		if out.err != nil {
			t.Fatalf("%s: %v", claim.name, out.err)
		}
		if !out.finalized.BudgetAttemptID.Valid {
			t.Fatalf("%s got no attempt", claim.name)
		}
		*claim.id = util.UUIDToString(out.finalized.BudgetAttemptID)
	}

	f.requireAttempt(t, f.onlyAttempt(t, first, a1, "the first claim"), a1, issueBudgetWantAttempt{
		Root: enabled, Task: first, Issue: child, State: "open", Reserved: issueBudgetInt64(100), Protocol: issueBudgetInt32(1),
	}, "the first claim")
	f.requireAttempt(t, f.onlyAttempt(t, second, a2, "the second claim"), a2, issueBudgetWantAttempt{
		Root: enabled, Task: second, Issue: parent, State: "open", Reserved: issueBudgetInt64(100), Protocol: issueBudgetInt32(1),
	}, "the second claim")
	// One observation in all: the claim that locked the root second is over
	// the limit with what the first committed.
	requireIssueBudgetClaimObservations(t, f.claimObservations(t, root), []issueBudgetClaimObservation{
		issueBudgetObservationOfClaim("would_refuse", "limit", parent, second, a2, map[string]string{
			"actual_known_ticks": "0", "reserved_ticks": "200", "unknown_hold_ticks": "0",
			"limit_ticks": "150", "this_reservation": "sized", "this_reservation_ticks": "100",
		}),
	}, "two claims that queued on the root lock")
	operations := f.claimOperations(t, root)
	if len(operations) != 2 {
		t.Fatalf("two claims left %d operation rows, want 2", len(operations))
	}
	f.requireClaimOperation(t, operations[0], issueBudgetWantClaim{Root: root, Task: first, Attempt: a1, Reservation: "sized", Reserved: "100"}, "the first claim")
	f.requireClaimOperation(t, operations[1], issueBudgetWantClaim{Root: root, Task: second, Attempt: a2, Reservation: "sized", Reserved: "100", WouldRefuse: []string{"limit"}}, "the second claim")
	entries := f.entryRows(t, root)
	if len(entries) != 2 {
		t.Fatalf("two claims left %d entry rows, want 2", len(entries))
	}
	accepted := budget.Receipt{Disposition: budget.DispositionAccepted}
	f.requireClaimEntry(t, entries[0], 1, issueBudgetReserveEntry(a1, 100), accepted, operations[0].ID, "the first reservation")
	f.requireClaimEntry(t, entries[1], 2, issueBudgetReserveEntry(a2, 100), accepted, operations[1].ID, "the second reservation")
	f.requireRootLedger(t, root, 2, issueBudgetLedgerFigures{ReservedTicks: 200}, "after both claims")
}

// F16 (10.9, the window), a claim that arrives while a root is being enabled.
// Enabling holds the binding lock to its commit, so the claim is refused: it
// cannot read whether its issue is a member. The enabling scan saw the task
// dispatched at the claim generation that was refused and recorded it as a
// gap. The task goes back to the queue and is claimed at the next generation,
// and that claim gets its attempt.
//
// The gap names the earlier generation and the attempt the later one, so the
// gap is not covered (3.3): the execution has an attempt, and the root still
// counts one thing it does not know.
func TestIssueBudgetFinalizeRefusedWhileARootIsEnabledIsAttributedAtItsNextClaim(t *testing.T) {
	f := newIssueBudgetFixture(t)
	ctx := context.Background()
	parent := f.issue(t, "Parent", "")
	child := f.issue(t, "Child", parent)
	agent, runtime := f.agent(t)
	task := f.claimed(t, agent, runtime, child, 1)
	before := f.snapshot(t)

	observed := false
	binder := &issueBudgetStatementLog{TxStarter: f.pool}
	binder.beforeCommit = func(pid uint32) {
		observed = true
		if got := f.advisoryLocks(t, pid); !reflect.DeepEqual(got, []issueBudgetAdvisoryLock{issueBudgetBindingHeld, issueBudgetFenceHeld}) {
			t.Fatalf("premise: advisory locks of the enabling transaction = %+v, want the binding lock and the fence, both held", got)
		}
		log := &issueBudgetStatementLog{TxStarter: f.pool}
		finalized, err := f.finalize(t, f.claims(log), task, TaskClaimBudgetProtocolV1)
		if !errors.Is(err, ErrIssueBudgetBindingInProgress) {
			t.Fatalf("finalize while the root is being enabled = %+v, %v; want ErrIssueBudgetBindingInProgress", finalized, err)
		}
		requireIssueBudgetStep(t, log.names(), []string{"TryLockIssueBudgetBindingSharedForIssue"}, "the claim that arrives while the root is being enabled")
		if got := f.tokens(t, task); got != 0 {
			t.Fatalf("the refused finalization left %d task tokens, want 0", got)
		}
		f.requireUnchanged(t, before, "the refused finalization")
	}
	params := f.enableParams(parent)
	limit, reserve := budget.Ticks(1000), budget.Ticks(100)
	params.LimitTicks, params.AttemptReserveTicks = &limit, &reserve
	enabled, err := NewIssueBudgetService(f.q, binder).Enable(ctx, params)
	if err != nil {
		t.Fatalf("enable: %v", err)
	}
	if !observed {
		t.Fatal("the enabling transaction was never observed")
	}
	root := enabled.BudgetRootID
	gap := issueBudgetObservationRow{Kind: "coverage_gap", Reason: "in_flight_legacy", Issue: child, Task: util.UUIDToString(task.ID), Generation: 1}
	if got := f.observations(t, root); enabled.InFlightGapCount != 1 || !reflect.DeepEqual(got, []issueBudgetObservationRow{gap}) {
		t.Fatalf("enabling counted %d gaps and left observations %+v, want the refused claim as its one gap: %+v", enabled.InFlightGapCount, got, gap)
	}
	if rows := f.attempts(t, task); len(rows) != 0 {
		t.Fatalf("the refused claim has attempt rows: %+v", rows)
	}

	again := f.claimedAgain(t, task)
	if again.ClaimGeneration != 2 {
		t.Fatalf("premise: the task is claimed again at generation %d, want 2", again.ClaimGeneration)
	}
	attempt := f.mustFinalize(t, f.claims(f.pool), again, TaskClaimBudgetProtocolV1)
	f.requireAttempt(t, f.onlyAttempt(t, again, attempt, "the claim after the root was enabled"), attempt, issueBudgetWantAttempt{
		Root: enabled, Task: again, Issue: child, State: "open", Reserved: issueBudgetInt64(100), Protocol: issueBudgetInt32(1),
	}, "the claim after the root was enabled")

	listed, err := f.svc.ListObservations(ctx, f.ws, util.MustParseUUID(root))
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 2 || listed[0].Row.Reason != "in_flight_legacy" || listed[0].Covered {
		t.Fatalf("observations after the next claim = %+v, want the gap of generation 1, not covered, and one judgement", listed)
	}
	// The uncovered gap is part of what the root does not know when the next
	// claim is judged.
	all := f.claimObservations(t, root)
	requireIssueBudgetClaimObservations(t, all[1:], []issueBudgetClaimObservation{
		issueBudgetObservationOfClaim("would_refuse", "unbounded_unknown", child, again, attempt, map[string]string{"observation.in_flight_legacy": "1"}),
	}, "the claim after the root was enabled")
	operations := f.claimOperations(t, root)
	if len(operations) != 1 {
		t.Fatalf("one claim left %d operation rows, want 1", len(operations))
	}
	f.requireClaimOperation(t, operations[0], issueBudgetWantClaim{
		Root: root, Task: again, Attempt: attempt, Reservation: "sized", Reserved: "100", WouldRefuse: []string{"unbounded_unknown"},
	}, "the claim after the root was enabled")
	f.requireRootLedger(t, root, 1, issueBudgetLedgerFigures{ReservedTicks: 100}, "after the claim that followed the enabling")
}

// F16, the other order: the claim is first. It reads that its issue has no
// member row under the shared side of the binding lock and keeps the lock to
// its commit, so a root cannot be enabled between that read and the commit:
// enabling waits. A claim that arrives behind the waiting binder is turned
// away. Once the first claim has committed, without an attempt, the enabling
// scan finds its task and records it as a gap.
//
// In both orders the execution has an attempt or a gap.
func TestIssueBudgetFinalizeThatReadNoMemberMakesEnablingWaitAndIsRecordedAsAGap(t *testing.T) {
	f := newIssueBudgetFixture(t)
	ctx := context.Background()
	parent := f.issue(t, "Parent", "")
	child := f.issue(t, "Child", parent)
	agent, runtime := f.agent(t)
	otherAgent, otherRuntime := f.agent(t)
	task := f.claimed(t, agent, runtime, child, 1)
	late := f.claimed(t, otherAgent, otherRuntime, parent, 4)
	before := f.snapshot(t)

	type outcome struct {
		result IssueBudgetOperationResult
		err    error
	}
	done := make(chan outcome, 1)
	binder := &issueBudgetStatementLog{TxStarter: f.pool}
	observed := false
	claim := &issueBudgetStatementLog{TxStarter: f.pool}
	claim.beforeCommit = func(pid uint32) {
		observed = true
		if got := f.advisoryLocks(t, pid); !reflect.DeepEqual(got, []issueBudgetAdvisoryLock{issueBudgetClaimHeld}) {
			t.Fatalf("advisory locks of the finalization before its commit = %+v, want the shared side of the binding lock and no other", got)
		}
		finished := make(chan struct{})
		go func() {
			defer close(finished)
			result, err := NewIssueBudgetService(f.q, binder).Enable(ctx, f.enableParams(parent))
			done <- outcome{result, err}
		}()
		f.awaitWriter(t, finished)

		enabling := f.blockedBy(t, pid)
		if got := f.advisoryLocks(t, enabling); !reflect.DeepEqual(got, []issueBudgetAdvisoryLock{issueBudgetBindingWait, issueBudgetFenceHeld}) {
			t.Fatalf("advisory locks of the waiting enable = %+v, want the fence held and the binding lock asked for", got)
		}
		if sent := binder.names(); len(sent) == 0 || sent[len(sent)-1] != "LockIssueBudgetBinding" {
			t.Fatalf("the waiting enable last sent %v, want it stopped at the binding lock", sent)
		}
		lateLog := &issueBudgetStatementLog{TxStarter: f.pool}
		if finalized, err := f.finalize(t, f.claims(lateLog), late, TaskClaimBudgetProtocolV1); !errors.Is(err, ErrIssueBudgetBindingInProgress) {
			t.Fatalf("finalize behind a waiting binder = %+v, %v; want ErrIssueBudgetBindingInProgress", finalized, err)
		}
		requireIssueBudgetStep(t, lateLog.names(), []string{"TryLockIssueBudgetBindingSharedForIssue"}, "the claim behind the waiting binder")
		select {
		case early := <-done:
			t.Fatalf("enable finished while a finalization held the shared side: %+v, %v", early.result, early.err)
		default:
		}
		f.requireUnchanged(t, before, "the finalization that read no member row, before it commits")
	}
	finalized, err := f.finalize(t, f.claims(claim), task, TaskClaimBudgetProtocolV1)
	if err != nil {
		t.Fatalf("finalize on a tree without a root: %v", err)
	}
	if !observed {
		t.Fatal("the finalization was never observed before its commit")
	}
	if finalized.BudgetAttemptID.Valid {
		t.Fatalf("the finalization that read no member row handed back attempt %s", util.UUIDToString(finalized.BudgetAttemptID))
	}
	requireIssueBudgetStep(t, claim.names(), []string{"TryLockIssueBudgetBindingSharedForIssue", "GetIssueBudgetMember"}, "the claim that read no member row")

	var out outcome
	select {
	case out = <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("enable did not finish after the finalization committed")
	}
	if out.err != nil {
		t.Fatalf("enable: %v", out.err)
	}
	if out.result.MemberCount != 2 || out.result.InFlightGapCount != 2 {
		t.Fatalf("enable result = %+v, want 2 members and both dispatched tasks as gaps", out.result)
	}
	want := map[issueBudgetObservationRow]bool{
		{Kind: "coverage_gap", Reason: "in_flight_legacy", Issue: child, Task: util.UUIDToString(task.ID), Generation: 1}:  true,
		{Kind: "coverage_gap", Reason: "in_flight_legacy", Issue: parent, Task: util.UUIDToString(late.ID), Generation: 4}: true,
	}
	got := f.observations(t, out.result.BudgetRootID)
	if len(got) != len(want) {
		t.Fatalf("observations after enabling = %+v, want %d gaps", got, len(want))
	}
	for _, row := range got {
		if !want[row] {
			t.Fatalf("unexpected observation after enabling: %+v", row)
		}
	}
	for _, claimed := range []db.AgentTaskQueue{task, late} {
		if rows := f.attempts(t, claimed); len(rows) != 0 {
			t.Fatalf("task %s has attempt rows %+v; its claim is a gap of the root", util.UUIDToString(claimed.ID), rows)
		}
	}
	if got := f.tokens(t, task); got != 1 {
		t.Fatalf("the finalized claim left %d task tokens, want 1", got)
	}
	if got := f.tokens(t, late); got != 0 {
		t.Fatalf("the refused claim left %d task tokens, want 0", got)
	}
}

// F17 (10.9). A claim and a manual change of the limit on one root lock the
// same row, each as the last lock of its transaction, so neither can wait for
// the other while holding what the other needs: there is no deadlock, and
// both commit. What they leave is what their order at the root lock gives. A
// claim that locks first is judged under the policy it found; a claim that
// locks behind the change reads the row again, reserves the new amount, is
// judged on the new limit and records the new policy revision.
func TestIssueBudgetFinalizeAndASetLimitOnOneRootCommitInTheOrderTheyLockIt(t *testing.T) {
	f := newIssueBudgetFixture(t)
	agent, runtime := f.agent(t)
	for _, tc := range []struct {
		name       string
		claimFirst bool
		// What the claim reserved and was judged under.
		reserved int64
		revision int64
		refuse   []string
	}{
		{name: "the claim locks the root first", claimFirst: true, reserved: 100, revision: 1},
		{name: "the change of the limit locks the root first", reserved: 70, revision: 2, refuse: []string{"limit"}},
	} {
		func() {
			enabled, _, child := f.budgetTree(t, 1000, 100)
			root := enabled.BudgetRootID
			task := f.claimed(t, agent, runtime, child, 1)
			startClaim := func() *issueBudgetWriter { return f.startFinalize(t, task, TaskClaimBudgetProtocolV1) }
			startChange := func() *issueBudgetWriter {
				params := f.manage(root, 1)
				limit, reserve := budget.Ticks(50), budget.Ticks(70)
				return f.startWriter(t, "the change of the limit", func(tx TxStarter) issueBudgetWriterOutcome {
					result, err := NewIssueBudgetService(f.q, tx).SetLimit(context.Background(), params, &limit, &reserve)
					return issueBudgetWriterOutcome{result: result, err: err}
				})
			}

			holder, release := f.holdRoot(t, root)
			defer release()
			var claim, change *issueBudgetWriter
			if tc.claimFirst {
				claim = startClaim()
				if got := f.waitsBehind(t, claim.pid, "the finalization"); !reflect.DeepEqual(got, []uint32{holder}) {
					t.Fatalf("%s: the finalization waits behind backends %v, want the holder of the root row, %d", tc.name, got, holder)
				}
				change = startChange()
				if got := f.waitsBehind(t, change.pid, "the change of the limit"); !reflect.DeepEqual(got, []uint32{claim.pid}) {
					t.Fatalf("%s: the change of the limit waits behind backends %v, want the finalization, %d", tc.name, got, claim.pid)
				}
			} else {
				change = startChange()
				if got := f.waitsBehind(t, change.pid, "the change of the limit"); !reflect.DeepEqual(got, []uint32{holder}) {
					t.Fatalf("%s: the change of the limit waits behind backends %v, want the holder of the root row, %d", tc.name, got, holder)
				}
				claim = startClaim()
				if got := f.waitsBehind(t, claim.pid, "the finalization"); !reflect.DeepEqual(got, []uint32{change.pid}) {
					t.Fatalf("%s: the finalization waits behind backends %v, want the change of the limit, %d", tc.name, got, change.pid)
				}
			}
			claim.requireStoppedAt(t, "LockIssueBudgetRoot", tc.name+": the finalization")
			change.requireStoppedAt(t, "LockIssueBudgetRoot", tc.name+": the change of the limit")

			release()
			claimed := claim.outcome(t, tc.name+": the finalization")
			changed := change.outcome(t, tc.name+": the change of the limit")
			if claimed.err != nil || changed.err != nil {
				t.Fatalf("%s: the finalization ended with %v and the change of the limit with %v; want both committed", tc.name, claimed.err, changed.err)
			}
			if !claimed.finalized.BudgetAttemptID.Valid {
				t.Fatalf("%s: the finalization got no attempt", tc.name)
			}
			attempt := util.UUIDToString(claimed.finalized.BudgetAttemptID)

			// The change: one policy revision on, whichever came first.
			if changed.result.PolicyRevision != 2 || changed.result.LimitTicks == nil || *changed.result.LimitTicks != 50 ||
				changed.result.AttemptReserveTicks == nil || *changed.result.AttemptReserveTicks != 70 {
				t.Fatalf("%s: the change of the limit committed %+v, want revision 2 with limit 50 and reserve 70", tc.name, changed.result)
			}
			row := f.rootRow(t, root)
			if row.PolicyRevision != 2 || row.PolicyEpoch != 1 || !row.LimitTicks.Valid || row.LimitTicks.Int64 != 50 ||
				!row.AttemptReserveTicks.Valid || row.AttemptReserveTicks.Int64 != 70 {
				t.Fatalf("%s: root policy = revision %d, epoch %d, limit %+v, reserve %+v; want revision 2, epoch 1, limit 50, reserve 70",
					tc.name, row.PolicyRevision, row.PolicyEpoch, row.LimitTicks, row.AttemptReserveTicks)
			}
			// The claim: one entry, with the reserve of the policy it locked.
			f.requireAttempt(t, f.onlyAttempt(t, task, attempt, tc.name), attempt, issueBudgetWantAttempt{
				Root: enabled, Task: task, Issue: child, State: "open",
				Reserved: issueBudgetInt64(tc.reserved), Protocol: issueBudgetInt32(1), Revision: tc.revision, Epoch: 1,
			}, tc.name)
			f.requireRootLedger(t, root, 1, issueBudgetLedgerFigures{ReservedTicks: tc.reserved}, tc.name)
			var observations []issueBudgetClaimObservation
			if tc.refuse != nil {
				observations = append(observations, issueBudgetObservationOfClaim("would_refuse", "limit", child, task, attempt, map[string]string{
					"actual_known_ticks": "0", "reserved_ticks": "70", "unknown_hold_ticks": "0",
					"limit_ticks": "50", "this_reservation": "sized", "this_reservation_ticks": "70",
				}))
			}
			requireIssueBudgetClaimObservations(t, f.claimObservations(t, root), observations, tc.name)
			operations := f.claimOperations(t, root)
			if len(operations) != 1 {
				t.Fatalf("%s: the claim left %d operation rows, want 1", tc.name, len(operations))
			}
			reserved := "100"
			if tc.reserved == 70 {
				reserved = "70"
			}
			f.requireClaimOperation(t, operations[0], issueBudgetWantClaim{
				Root: root, Task: task, Attempt: attempt, Reservation: "sized", Reserved: reserved, WouldRefuse: tc.refuse,
			}, tc.name)
			entries := f.entryRows(t, root)
			if len(entries) != 1 {
				t.Fatalf("%s: the claim left %d entry rows, want 1", tc.name, len(entries))
			}
			f.requireClaimEntry(t, entries[0], 1, issueBudgetReserveEntry(attempt, budget.Ticks(tc.reserved)),
				budget.Receipt{Disposition: budget.DispositionAccepted}, operations[0].ID, tc.name)
		}()
	}
}
