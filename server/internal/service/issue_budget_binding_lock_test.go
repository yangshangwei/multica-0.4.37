package service

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/multica-ai/multica/server/internal/budget"
	"github.com/multica-ai/multica/server/internal/iteration"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// Step 5b, round 1: the binder's side of the workspace binding lock (design
// 10.10), the level a binding transaction must have (section 5, rule 5) and
// the in-flight scan as corrected on 2026-11-18 (10.2).
//
// The claim's side of the lock belongs to finalize. Here a second connection
// stands in for a claim and takes the shared side through the query finalize
// will use, so both sides are proven to be one lock.

// issueBudgetAdvisoryLock is one advisory lock of a backend as pg_locks shows
// it.
type issueBudgetAdvisoryLock struct {
	Name    string // fence, binding or other
	Mode    string
	Granted bool
}

// advisoryLocks reads the advisory locks one backend holds or waits for. The
// two keys are spelled out here, not taken from the code under test: the
// binding lock's is the text issue_budget_binding:<workspace id>, and it is
// not the fence's.
func (f *issueBudgetFixture) advisoryLocks(t *testing.T, pid uint32) []issueBudgetAdvisoryLock {
	t.Helper()
	rows, err := f.pool.Query(context.Background(), `
		SELECT CASE (l.classid::bigint << 32) | l.objid::bigint
		         WHEN hashtextextended('iteration:' || $2::uuid::text, 0) THEN 'fence'
		         WHEN hashtextextended('issue_budget_binding:' || $2::uuid::text, 0) THEN 'binding'
		         ELSE 'other' END AS name,
		       l.mode, l.granted
		FROM pg_locks l
		WHERE l.locktype = 'advisory' AND l.objsubid = 1 AND l.pid = $1
		ORDER BY name, l.mode`, int32(pid), f.ws)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var locks []issueBudgetAdvisoryLock
	for rows.Next() {
		var lock issueBudgetAdvisoryLock
		if err := rows.Scan(&lock.Name, &lock.Mode, &lock.Granted); err != nil {
			t.Fatal(err)
		}
		locks = append(locks, lock)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return locks
}

// claimArrives reports whether a claim arriving now would get the shared side
// of a workspace's binding lock. It asks without waiting, as a claim does, and
// lets go at once.
func (f *issueBudgetFixture) claimArrives(t *testing.T, workspace pgtype.UUID) bool {
	t.Helper()
	ctx := context.Background()
	tx, err := f.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	acquired, err := db.New(tx).TryLockIssueBudgetBindingShared(ctx, workspace)
	if err != nil {
		t.Fatal(err)
	}
	return acquired
}

// blockedBy waits until exactly one backend waits for a lock holder has, and
// returns that backend.
func (f *issueBudgetFixture) blockedBy(t *testing.T, holder uint32) uint32 {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		rows, err := f.pool.Query(context.Background(),
			`SELECT pid FROM pg_stat_activity WHERE datname=current_database() AND $1::integer=ANY(pg_blocking_pids(pid))`, int32(holder))
		if err != nil {
			t.Fatal(err)
		}
		pids, err := pgx.CollectRows(rows, pgx.RowTo[int32])
		if err != nil {
			t.Fatal(err)
		}
		if len(pids) == 1 {
			return uint32(pids[0])
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("no transaction came to wait behind backend %d", holder)
	return 0
}

// awaitWriter makes a test that ends early wait for the writer it started on
// another goroutine. A failed check releases the stand-in claim through its
// deferred rollback and so lets the writer run on; the fixture rows must not
// be removed under it.
func (f *issueBudgetFixture) awaitWriter(t *testing.T, finished <-chan struct{}) {
	t.Helper()
	t.Cleanup(func() {
		select {
		case <-finished:
		case <-time.After(30 * time.Second):
			t.Error("a writer started by this test was still running when it ended")
		}
	})
}

// inBindingHeld is inBinding with a look at the transaction before it ends:
// held runs after fn, with the binding's transaction still open.
func (f *issueBudgetFixture) inBindingHeld(t *testing.T, fn func(q *db.Queries) error, held func(pid uint32)) []string {
	t.Helper()
	ctx := context.Background()
	tx, err := f.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if err := iteration.LockWorkspace(ctx, tx, f.ws); err != nil {
		t.Fatal(err)
	}
	log := &issueBudgetStatementLog{}
	if err := fn(db.New(issueBudgetLoggedTx{Tx: tx, log: log})); err != nil {
		t.Fatalf("binding: %v", err)
	}
	held(tx.Conn().PgConn().PID())
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	return log.names()
}

var (
	issueBudgetFenceHeld   = issueBudgetAdvisoryLock{Name: "fence", Mode: "ExclusiveLock", Granted: true}
	issueBudgetBindingHeld = issueBudgetAdvisoryLock{Name: "binding", Mode: "ExclusiveLock", Granted: true}
	issueBudgetBindingWait = issueBudgetAdvisoryLock{Name: "binding", Mode: "ExclusiveLock", Granted: false}
	issueBudgetClaimHeld   = issueBudgetAdvisoryLock{Name: "binding", Mode: "ShareLock", Granted: true}
)

// 10.10, the binder's side in Enable: once nothing can refuse it any more,
// enabling takes the workspace binding lock, exclusive, and holds it until it
// commits. A claim that arrives meanwhile is turned away; a claim in another
// workspace is not.
func TestIssueBudgetEnableHoldsTheBindingLockUntilItCommits(t *testing.T) {
	f := newIssueBudgetFixture(t)
	ctx := context.Background()
	parent := f.issue(t, "Parent", "")
	f.issue(t, "Child", parent)
	elsewhere := util.MustParseUUID(uuid.NewString())

	if !f.claimArrives(t, f.ws) {
		t.Fatal("the binding lock was not free before enabling")
	}
	observed := false
	log := &issueBudgetStatementLog{TxStarter: f.pool}
	log.beforeCommit = func(pid uint32) {
		observed = true
		want := []issueBudgetAdvisoryLock{issueBudgetBindingHeld, issueBudgetFenceHeld}
		if got := f.advisoryLocks(t, pid); !reflect.DeepEqual(got, want) {
			t.Errorf("advisory locks of the enabling transaction = %+v, want %+v", got, want)
		}
		if f.claimArrives(t, f.ws) {
			t.Error("a claim got the shared side of the binding lock while enabling held it")
		}
		if !f.claimArrives(t, elsewhere) {
			t.Error("enabling a root in one workspace turned a claim of another workspace away")
		}
	}
	if _, err := NewIssueBudgetService(f.q, log).Enable(ctx, f.enableParams(parent)); err != nil {
		t.Fatal(err)
	}
	if !observed {
		t.Fatal("the enabling transaction was never observed")
	}
	if !f.claimArrives(t, f.ws) {
		t.Fatal("the binding lock outlived the enabling transaction")
	}
}

// 10.10: the lock comes after every refusal check. An enable that is replayed
// or refused binds nothing, so it ends without the lock and without the scan
// that is only meaningful under it.
func TestIssueBudgetEnableThatBindsNothingTakesNoBindingLock(t *testing.T) {
	f := newIssueBudgetFixture(t)
	ctx := context.Background()
	parent := f.issue(t, "Parent", "")
	child := f.issue(t, "Child", parent)
	first := f.enableParams(parent)
	if _, err := f.svc.Enable(ctx, first); err != nil {
		t.Fatal(err)
	}
	// An issue outside every root with a member of that root below it.
	above := f.issue(t, "Above a member", "")
	f.setParent(t, child, above)
	// Two issues that are each other's parent.
	loopA := f.issue(t, "Loop A", "")
	loopB := f.issue(t, "Loop B", loopA)
	f.setParent(t, loopA, loopB)
	pending := f.issue(t, "Waiting for triage", "", testutil.Cols{"admission_status": "pending"})
	committed := f.snapshot(t)

	otherIntent := first
	limit := budget.Ticks(1)
	otherIntent.LimitTicks = &limit
	for _, tc := range []struct {
		name   string
		params IssueBudgetEnableParams
		code   string
	}{
		{"the committed enable replayed under its key", first, ""},
		{"the same key with another intent", otherIntent, IssueBudgetOperationConflict},
		{"an enabled issue under another key", f.enableParams(parent), IssueBudgetAlreadyEnabled},
		{"a member of a root", f.enableParams(child), IssueBudgetRootConflict},
		{"an issue above a member of a root", f.enableParams(above), IssueBudgetRootConflict},
		{"an issue in a parent loop", f.enableParams(loopA), IssueBudgetParentCycle},
		{"an issue waiting for triage", f.enableParams(pending), IssueBudgetNotEligible},
		{"an issue that does not exist", f.enableParams(uuid.NewString()), IssueBudgetNotFound},
	} {
		log := &issueBudgetStatementLog{TxStarter: f.pool}
		log.beforeCommit = func(pid uint32) {
			// Only the replay gets as far as a commit.
			if got := f.advisoryLocks(t, pid); !reflect.DeepEqual(got, []issueBudgetAdvisoryLock{issueBudgetFenceHeld}) {
				t.Errorf("%s: advisory locks = %+v, want the fence alone", tc.name, got)
			}
		}
		result, err := NewIssueBudgetService(f.q, log).Enable(ctx, tc.params)
		if tc.code == "" {
			if err != nil || !result.Replayed {
				t.Fatalf("%s: got (%+v, %v), want the committed result replayed", tc.name, result, err)
			}
		} else {
			requireIssueBudgetCode(t, err, tc.code, tc.name)
		}
		sent := log.names()
		if len(sent) < 4 || sent[0] != issueBudgetLevelStatement {
			t.Fatalf("%s sent %v, want an operation that at least reached the slot lookup", tc.name, sent)
		}
		for _, name := range sent {
			if name == "LockIssueBudgetBinding" || name == "ListIssueBudgetInFlightTasks" || strings.HasPrefix(name, "Insert") {
				t.Fatalf("%s sent %s; statements: %v", tc.name, name, sent)
			}
		}
	}
	f.requireUnchanged(t, committed, "replayed and refused enables")
}

// 10.10, why the lock is waited for and why the scan comes after it. A claim
// that already read "unbound" under the shared side has not committed its task
// yet. Enabling waits for it; while it waits, later claims are turned away;
// and once the claim has committed, the scan, a statement sent after the lock
// was granted in a READ COMMITTED transaction, finds the task and records it.
// A scan sent before the lock, or from an older snapshot, would miss it, and
// the claim would stay unattributed without a trace.
func TestIssueBudgetEnableWaitsForAClaimThatReadUnbound(t *testing.T) {
	f := newIssueBudgetFixture(t)
	ctx := context.Background()
	parent := f.issue(t, "Parent", "")
	child := f.issue(t, "Child", parent)
	agent, runtime := f.agent(t)
	task := f.task(t, agent, runtime, child, "queued", 0)

	claim, err := f.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer claim.Rollback(ctx)
	claimQ := db.New(claim)
	if acquired, err := claimQ.TryLockIssueBudgetBindingShared(ctx, f.ws); err != nil || !acquired {
		t.Fatalf("the claim's shared try-lock = %v, %v", acquired, err)
	}
	if _, bound, err := issueBudgetMember(ctx, claimQ, f.ws, util.MustParseUUID(child)); err != nil || bound {
		t.Fatalf("the claim read bound = %v, %v; the child has no member row yet", bound, err)
	}
	claimPID := claim.Conn().PgConn().PID()
	if got := f.advisoryLocks(t, claimPID); !reflect.DeepEqual(got, []issueBudgetAdvisoryLock{issueBudgetClaimHeld}) {
		t.Fatalf("advisory locks of the claim = %+v", got)
	}

	type outcome struct {
		result IssueBudgetOperationResult
		err    error
	}
	done := make(chan outcome, 1)
	finished := make(chan struct{})
	log := &issueBudgetStatementLog{TxStarter: f.pool}
	go func() {
		defer close(finished)
		result, err := NewIssueBudgetService(f.q, log).Enable(ctx, f.enableParams(parent))
		done <- outcome{result, err}
	}()
	f.awaitWriter(t, finished)

	enabling := f.blockedBy(t, claimPID)
	if got := f.advisoryLocks(t, enabling); !reflect.DeepEqual(got, []issueBudgetAdvisoryLock{issueBudgetBindingWait, issueBudgetFenceHeld}) {
		t.Fatalf("advisory locks of the waiting enable = %+v, want the fence held and the binding lock asked for", got)
	}
	if sent := log.names(); len(sent) == 0 || sent[len(sent)-1] != "LockIssueBudgetBinding" {
		t.Fatalf("the waiting enable last sent %v, want it stopped at the binding lock", sent)
	}
	if f.claimArrives(t, f.ws) {
		t.Fatal("a claim arriving behind a waiting binder got the shared side of the lock")
	}
	select {
	case early := <-done:
		t.Fatalf("enable finished while a claim held the shared side: %+v, %v", early.result, early.err)
	default:
	}

	// The claim commits what it claimed, and with that lets go of the lock.
	if _, err := claim.Exec(ctx, `UPDATE agent_task_queue SET status='dispatched', claim_generation=1 WHERE id=$1`, task); err != nil {
		t.Fatal(err)
	}
	if err := claim.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	var out outcome
	select {
	case out = <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("enable did not finish after the claim committed")
	}
	if out.err != nil {
		t.Fatalf("enable: %v", out.err)
	}
	if out.result.MemberCount != 2 || out.result.InFlightGapCount != 1 {
		t.Fatalf("enable result = %+v, want 2 members and the claim that committed while it waited as 1 gap", out.result)
	}
	want := []issueBudgetObservationRow{{Kind: "coverage_gap", Reason: "in_flight_legacy", Issue: child, Task: task, Generation: 1}}
	if got := f.observations(t, out.result.BudgetRootID); !reflect.DeepEqual(got, want) {
		t.Fatalf("observations = %+v, want %+v", got, want)
	}
	if !f.claimArrives(t, f.ws) {
		t.Fatal("the binding lock outlived the enabling transaction")
	}
}

// The same for a move that binds existing issues (9.3 rule 2): it waits for
// the claim that read "unbound", turns later claims away, and then finds the
// task that claim committed.
func TestIssueBudgetReparentWaitsForAClaimThatReadUnbound(t *testing.T) {
	f := newIssueBudgetFixture(t)
	ctx := context.Background()
	parent := f.issue(t, "Parent", "")
	enabled := f.enable(t, parent)
	subtree := f.issue(t, "Unbound subtree", "")
	leaf := f.issue(t, "Unbound leaf", subtree)
	agent, runtime := f.agent(t)
	task := f.task(t, agent, runtime, leaf, "queued", 0)

	claim, err := f.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer claim.Rollback(ctx)
	claimQ := db.New(claim)
	if acquired, err := claimQ.TryLockIssueBudgetBindingShared(ctx, f.ws); err != nil || !acquired {
		t.Fatalf("the claim's shared try-lock = %v, %v", acquired, err)
	}
	if _, bound, err := issueBudgetMember(ctx, claimQ, f.ws, util.MustParseUUID(leaf)); err != nil || bound {
		t.Fatalf("the claim read bound = %v, %v; the leaf has no member row yet", bound, err)
	}
	claimPID := claim.Conn().PgConn().PID()

	// The move, as an issue writer makes it: fence, the parent write, the
	// binding, all in one transaction.
	done := make(chan error, 1)
	finished := make(chan struct{})
	log := &issueBudgetStatementLog{}
	f.awaitWriter(t, finished)
	go func() {
		defer close(finished)
		done <- func() error {
			tx, err := f.pool.Begin(ctx)
			if err != nil {
				return err
			}
			defer tx.Rollback(ctx)
			if err := iteration.LockWorkspace(ctx, tx, f.ws); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `UPDATE issue SET parent_issue_id=$2 WHERE id=$1`, subtree, parent); err != nil {
				return err
			}
			moved, err := db.New(tx).GetIssueInWorkspace(ctx, db.GetIssueInWorkspaceParams{ID: util.MustParseUUID(subtree), WorkspaceID: f.ws})
			if err != nil {
				return err
			}
			if err := BindIssueBudgetOnReparent(ctx, db.New(issueBudgetLoggedTx{Tx: tx, log: log}), moved); err != nil {
				return err
			}
			return tx.Commit(ctx)
		}()
	}()

	moving := f.blockedBy(t, claimPID)
	if got := f.advisoryLocks(t, moving); !reflect.DeepEqual(got, []issueBudgetAdvisoryLock{issueBudgetBindingWait, issueBudgetFenceHeld}) {
		t.Fatalf("advisory locks of the waiting move = %+v, want the fence held and the binding lock asked for", got)
	}
	wantSoFar := []string{"GetIssueBudgetMember", "GetIssueBudgetTransactionIsolation", "GetIssueBudgetMember", "ListIssueBudgetDescendants", "LockIssueBudgetBinding"}
	if sent := log.names(); !reflect.DeepEqual(sent, wantSoFar) {
		t.Fatalf("the waiting move sent %v, want %v", sent, wantSoFar)
	}
	if f.claimArrives(t, f.ws) {
		t.Fatal("a claim arriving behind a waiting binder got the shared side of the lock")
	}
	if _, bound := f.member(t, subtree); bound {
		t.Fatal("the move bound an issue before it had the binding lock")
	}

	if _, err := claim.Exec(ctx, `UPDATE agent_task_queue SET status='running', claim_generation=4 WHERE id=$1`, task); err != nil {
		t.Fatal(err)
	}
	if err := claim.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("move: %v", err)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("the move did not finish after the claim committed")
	}

	f.requireMember(t, subtree, issueBudgetMemberRow{Root: enabled.BudgetRootID, BoundParent: parent, Source: "reparent", Version: 1})
	f.requireMember(t, leaf, issueBudgetMemberRow{Root: enabled.BudgetRootID, BoundParent: subtree, Source: "reparent", Version: 1})
	want := []issueBudgetObservationRow{{Kind: "coverage_gap", Reason: "in_flight_at_bind", Issue: leaf, Task: task, Generation: 4}}
	if got := f.observations(t, enabled.BudgetRootID); !reflect.DeepEqual(got, want) {
		t.Fatalf("observations = %+v, want %+v", got, want)
	}
}

// 10.10, path by path, written from the design: only a transaction that gives
// an existing issue its first member row takes the binding lock. It takes it
// after the walk and before the first member row, and the scan follows the
// member rows. Creating a child, moving a member and deleting one take none,
// and a claim gets the shared side while their transactions are open.
func TestIssueBudgetOnlyAFirstBindingTakesTheBindingLock(t *testing.T) {
	f := newIssueBudgetFixture(t)
	ctx := context.Background()
	parent := f.issue(t, "Parent", "")
	member := f.issue(t, "Member", parent)
	otherParent := f.issue(t, "Other root's parent", "")
	foreign := f.issue(t, "Member of the other root", otherParent)
	f.enable(t, parent)
	f.enable(t, otherParent)
	agent, runtime := f.agent(t)

	noLock := func(what string) func(pid uint32) {
		return func(pid uint32) {
			t.Helper()
			if got := f.advisoryLocks(t, pid); !reflect.DeepEqual(got, []issueBudgetAdvisoryLock{issueBudgetFenceHeld}) {
				t.Fatalf("%s: advisory locks = %+v, want the fence alone", what, got)
			}
			if !f.claimArrives(t, f.ws) {
				t.Fatalf("%s turned a claim away", what)
			}
		}
	}
	exclusive := func(what string) func(pid uint32) {
		return func(pid uint32) {
			t.Helper()
			if got := f.advisoryLocks(t, pid); !reflect.DeepEqual(got, []issueBudgetAdvisoryLock{issueBudgetBindingHeld, issueBudgetFenceHeld}) {
				t.Fatalf("%s: advisory locks = %+v, want the fence and the binding lock", what, got)
			}
			if f.claimArrives(t, f.ws) {
				t.Fatalf("%s let a claim in while it held the binding lock", what)
			}
		}
	}
	require := func(what string, got, want []string) {
		t.Helper()
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("%s sent\n%s\nwant\n%s", what, strings.Join(got, "\n"), strings.Join(want, "\n"))
		}
	}

	created := f.issue(t, "Created under a member", member)
	createdRow := f.setParent(t, created, member)
	what := "creating a child of a member"
	require(what, f.inBindingHeld(t, func(q *db.Queries) error { return BindIssueBudgetOnCreate(ctx, q, createdRow, pgtype.UUID{}) }, noLock(what)),
		[]string{"GetIssueBudgetMember", "GetIssueBudgetTransactionIsolation", "InsertIssueBudgetMember"})

	what = "moving a member inside its root"
	inside := f.setParent(t, created, parent)
	require(what, f.inBindingHeld(t, func(q *db.Queries) error { return BindIssueBudgetOnReparent(ctx, q, inside) }, noLock(what)),
		[]string{"GetIssueBudgetMember", "GetIssueBudgetTransactionIsolation", "GetIssueBudgetMember"})

	what = "moving a member under another root"
	across := f.setParent(t, created, otherParent)
	require(what, f.inBindingHeld(t, func(q *db.Queries) error { return BindIssueBudgetOnReparent(ctx, q, across) }, noLock(what)),
		[]string{"GetIssueBudgetMember", "GetIssueBudgetTransactionIsolation", "GetIssueBudgetMember", "GetIssueBudgetRoot", "InsertIssueBudgetObservation"})

	what = "deleting a member"
	require(what, f.inBindingHeld(t, func(q *db.Queries) error {
		return TombstoneIssueBudgetMember(ctx, q, f.ws, util.MustParseUUID(created))
	}, noLock(what)), []string{"TombstoneIssueBudgetMember"})

	what = "moving an unbound subtree under a member"
	subtree := f.issue(t, "Unbound subtree", "")
	f.issue(t, "Unbound leaf", subtree)
	plain := f.setParent(t, subtree, member)
	require(what, f.inBindingHeld(t, func(q *db.Queries) error { return BindIssueBudgetOnReparent(ctx, q, plain) }, exclusive(what)),
		[]string{
			"GetIssueBudgetMember", "GetIssueBudgetTransactionIsolation", "GetIssueBudgetMember",
			"ListIssueBudgetDescendants",
			"LockIssueBudgetBinding",
			"GetIssueBudgetRoot",
			"InsertIssueBudgetMembers",
			"ListIssueBudgetInFlightTasks",
		})
	if !f.claimArrives(t, f.ws) {
		t.Fatal("the binding lock outlived the move that took it")
	}

	// The same move with everything it can meet: a descendant that belongs to
	// another root, and a task a daemon already holds.
	what = "moving an unbound subtree with a foreign member and a held task"
	busy := f.issue(t, "Busy unbound subtree", "")
	f.setParent(t, foreign, busy)
	f.task(t, agent, runtime, busy, "running", 2)
	full := f.setParent(t, busy, member)
	require(what, f.inBindingHeld(t, func(q *db.Queries) error { return BindIssueBudgetOnReparent(ctx, q, full) }, exclusive(what)),
		[]string{
			"GetIssueBudgetMember", "GetIssueBudgetTransactionIsolation", "GetIssueBudgetMember",
			"ListIssueBudgetDescendants",
			"LockIssueBudgetBinding",
			"GetIssueBudgetRoot",
			"InsertIssueBudgetMembers",
			"InsertIssueBudgetObservation",
			"ListIssueBudgetInFlightTasks",
			"InsertIssueBudgetObservation",
		})
}

// 10.10 from the other side: while a binder holds the exclusive side, every
// writer that binds no existing issue still goes through. Child creation is
// driven through the real create path; each of these would hang here if it
// asked for either side of the lock.
func TestIssueBudgetWritersThatBindNoExistingIssueDoNotWaitForABinder(t *testing.T) {
	f := newIssueBudgetFixture(t)
	parent := f.issue(t, "Parent", "")
	member := f.issue(t, "Member", parent)
	otherParent := f.issue(t, "Other root's parent", "")
	enabled := f.enable(t, parent)
	f.enable(t, otherParent)
	f.syncIssueCounter(t)

	binder, err := f.pool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer binder.Rollback(context.Background())
	if err := db.New(binder).LockIssueBudgetBinding(context.Background(), f.ws); err != nil {
		t.Fatal(err)
	}
	if f.claimArrives(t, f.ws) {
		t.Fatal("the stand-in binder does not hold the binding lock")
	}

	// A writer that waited for the lock is cancelled here instead of hanging
	// the run.
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	tx, err := f.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	result, err := f.issues.CreateInTx(ctx, tx, IssueCreateParams{
		WorkspaceID: f.ws, Title: "Created while a binder holds the lock", Status: "todo", Priority: "none",
		CreatorType: "member", CreatorID: f.actor, AllowDuplicate: true, ParentIssueID: util.MustParseUUID(member),
	}, IssueCountPolicy{})
	if err != nil {
		t.Fatalf("create a child of a member while a binder holds the lock: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	created := util.UUIDToString(result.Issue.ID)
	f.requireMember(t, created, issueBudgetMemberRow{Root: enabled.BudgetRootID, BoundParent: member, Source: "create", Version: 1})

	inside := f.setParent(t, created, parent)
	f.inBinding(t, func(q *db.Queries) error { return BindIssueBudgetOnReparent(ctx, q, inside) })
	across := f.setParent(t, created, otherParent)
	f.inBinding(t, func(q *db.Queries) error { return BindIssueBudgetOnReparent(ctx, q, across) })
	f.inBinding(t, func(q *db.Queries) error { return TombstoneIssueBudgetMember(ctx, q, f.ws, result.Issue.ID) })
	f.requireMember(t, created, issueBudgetMemberRow{Root: enabled.BudgetRootID, BoundParent: member, Source: "create", Version: 1, Tombstoned: true})

	// Management locks its root and nothing else of the workspace.
	revision := int64(1)
	for _, op := range f.operations("")[1:] {
		if _, err := op.run(ctx, f.svc, f.manage(enabled.BudgetRootID, revision)); err != nil {
			t.Fatalf("%s while a binder holds the lock: %v", op.name, err)
		}
		revision++
	}

	if f.claimArrives(t, f.ws) {
		t.Fatal("the stand-in binder lost the lock; nothing above was tested against it")
	}
}

// 10.2, supplement of 2026-11-18: a task that has an issue is found through
// the issue, whichever workspace its agent and runtime belong to. Enabling a
// root records such a task as a gap, and so does a move that binds its issue.
func TestIssueBudgetInFlightScanFindsATaskRunFromAnotherWorkspace(t *testing.T) {
	f := newIssueBudgetFixture(t)
	ctx := context.Background()
	parent := f.issue(t, "Parent", "")
	child := f.issue(t, "Child", parent)
	subtree := f.issue(t, "Unbound subtree", "")

	executing := f.fx.Workspace(t, "Executing workspace", "budget-exec-"+strings.ReplaceAll(uuid.NewString(), "-", ""))
	f.fx.Member(t, executing, f.user, "owner")
	issueBudgetCleanWorkspace(t, f.fx, executing)
	foreignRuntime := f.fx.Runtime(t, "budget-foreign-runtime-"+uuid.NewString(), testutil.Cols{"workspace_id": executing})
	foreignAgent := f.fx.Agent(t, "budget-foreign-agent-"+uuid.NewString(), foreignRuntime, testutil.Cols{"workspace_id": executing, "max_concurrent_tasks": 20})
	agent, runtime := f.agent(t)

	foreignTask := f.task(t, foreignAgent, foreignRuntime, child, "running", 3)
	homeTask := f.task(t, agent, runtime, parent, "dispatched", 1)
	lateTask := f.task(t, foreignAgent, foreignRuntime, subtree, "waiting_local_directory", 5)
	// Held by the same foreign agent, for an issue of its own workspace.
	elsewhere := f.fx.Issue(t, "Issue of the executing workspace", testutil.Cols{"workspace_id": executing})
	f.task(t, foreignAgent, foreignRuntime, elsewhere, "running", 7)

	enabled := f.enable(t, parent)
	if enabled.InFlightGapCount != 2 {
		t.Fatalf("enable counted %d in-flight gaps, want the home task and the one run from another workspace", enabled.InFlightGapCount)
	}
	wantEnable := map[issueBudgetObservationRow]bool{
		{Kind: "coverage_gap", Reason: "in_flight_legacy", Issue: child, Task: foreignTask, Generation: 3}: true,
		{Kind: "coverage_gap", Reason: "in_flight_legacy", Issue: parent, Task: homeTask, Generation: 1}:   true,
	}
	got := f.observations(t, enabled.BudgetRootID)
	if len(got) != len(wantEnable) {
		t.Fatalf("observations after enable = %+v, want %d", got, len(wantEnable))
	}
	for _, row := range got {
		if !wantEnable[row] {
			t.Fatalf("unexpected observation after enable: %+v", row)
		}
	}

	moved := f.setParent(t, subtree, child)
	f.inBinding(t, func(q *db.Queries) error { return BindIssueBudgetOnReparent(ctx, q, moved) })
	late := issueBudgetObservationRow{Kind: "coverage_gap", Reason: "in_flight_at_bind", Issue: subtree, Task: lateTask, Generation: 5}
	got = f.observations(t, enabled.BudgetRootID)
	if len(got) != 3 || got[2] != late {
		t.Fatalf("observations after the move = %+v, want the two from enable and %+v", got, late)
	}
	// Nothing of this reaches the executing workspace.
	for _, table := range issueBudgetTables {
		if rows := issueBudgetRows(t, f.fx, table, executing); rows != "[]" {
			t.Fatalf("%s of the executing workspace = %s", table, rows)
		}
	}
}

// 10.2, supplement of 2026-11-18 (check finding N3): the scan and
// parseQuickCreateContext decide by one definition what a quick-create task
// is. The query proposes every task without an issue whose context names the
// parent; a task linked to a chat session or an autopilot run, or one whose
// context does not decode, is not a quick-create task and is no gap.
func TestIssueBudgetInFlightScanDecidesQuickCreateByTheSharedDefinition(t *testing.T) {
	f := newIssueBudgetFixture(t)
	parent := f.issue(t, "Parent", "")
	agent, runtime := f.agent(t)

	plain := f.quickCreateTask(t, agent, runtime, parent, "running", 1)
	upper := f.quickCreateTask(t, agent, runtime, strings.ToUpper(parent), "dispatched", 2)

	chat := f.fx.ChatSession(t, agent)
	f.quickCreateTask(t, agent, runtime, parent, "running", 3, testutil.Cols{"chat_session_id": chat})
	autopilot := f.fx.Insert(t, "autopilot", testutil.Cols{
		"workspace_id": f.workspace, "title": "Budget scan autopilot", "assignee_id": agent,
		"execution_mode": "run_only", "created_by_type": "member", "created_by_id": f.user,
	})
	run := f.fx.Insert(t, "autopilot_run", testutil.Cols{"autopilot_id": autopilot, "source": "manual", "status": "running"})
	f.quickCreateTask(t, agent, runtime, parent, "running", 4, testutil.Cols{"autopilot_run_id": run})
	// The type and the parent are what the query looks at; the prompt is a
	// number, so the context is not a quick-create context.
	f.quickCreateTask(t, agent, runtime, parent, "running", 5, testutil.Cols{
		"context": `{"type":"quick_create","parent_issue_id":"` + parent + `","prompt":7}`,
	})
	f.quickCreateTask(t, agent, runtime, parent, "running", 6, testutil.Cols{
		"context": `{"type":"quick_create_draft","parent_issue_id":"` + parent + `"}`,
	})

	enabled := f.enable(t, parent)
	want := map[issueBudgetObservationRow]bool{
		{Kind: "coverage_gap", Reason: "in_flight_legacy", Issue: parent, Task: plain, Generation: 1}: true,
		{Kind: "coverage_gap", Reason: "in_flight_legacy", Issue: parent, Task: upper, Generation: 2}: true,
	}
	got := f.observations(t, enabled.BudgetRootID)
	if len(got) != len(want) || enabled.InFlightGapCount != len(want) {
		t.Fatalf("observations = %+v (result counts %d), want %d quick-create gaps", got, enabled.InFlightGapCount, len(want))
	}
	for _, row := range got {
		if !want[row] {
			t.Fatalf("unexpected observation %+v", row)
		}
	}
}

// The definition itself, without a database: the task reader and the scan
// call one function and so give one answer for every shape of task.
func TestIssueBudgetQuickCreateDefinitionIsShared(t *testing.T) {
	link := pgtype.UUID{Bytes: uuid.New(), Valid: true}
	quick := []byte(`{"type":"quick_create","prompt":"file it","parent_issue_id":"8a4f2f0e-3c0b-4a43-9f7e-0f0a5a0c9d11"}`)
	for _, tc := range []struct {
		name string
		task db.AgentTaskQueue
		want bool
	}{
		{"a quick-create context and no link", db.AgentTaskQueue{Context: quick}, true},
		{"linked to an issue", db.AgentTaskQueue{IssueID: link, Context: quick}, false},
		{"linked to a chat session", db.AgentTaskQueue{ChatSessionID: link, Context: quick}, false},
		{"linked to an autopilot run", db.AgentTaskQueue{AutopilotRunID: link, Context: quick}, false},
		{"no context", db.AgentTaskQueue{}, false},
		{"a context that does not decode", db.AgentTaskQueue{Context: []byte(`{"type":"quick_create","prompt":7}`)}, false},
		{"a context that is not JSON", db.AgentTaskQueue{Context: []byte(`quick_create`)}, false},
		{"a context of another type", db.AgentTaskQueue{Context: []byte(`{"type":"chat"}`)}, false},
	} {
		byTask, okTask := (&TaskService{}).parseQuickCreateContext(tc.task)
		byColumns, okColumns := quickCreateTaskContext(tc.task.IssueID, tc.task.ChatSessionID, tc.task.AutopilotRunID, tc.task.Context)
		if okTask != tc.want || okColumns != tc.want || !reflect.DeepEqual(byTask, byColumns) {
			t.Fatalf("%s: task reader = (%+v, %v), scan = (%+v, %v), want %v from both", tc.name, byTask, okTask, byColumns, okColumns, tc.want)
		}
		if tc.want && (byTask.Type != QuickCreateContextType || byTask.Prompt != "file it" || byTask.ParentIssueID == "") {
			t.Fatalf("%s: decoded context = %+v", tc.name, byTask)
		}
	}
}

// Section 5, rule 5, in the binding functions. A transaction that reaches a
// member row from REPEATABLE READ or SERIALIZABLE is refused with
// ErrIssueBudgetIsolation before it writes, on the first tree that has a root.
// A tree without a root is never asked, and neither is a delete.
func TestIssueBudgetBindingRefusesATransactionThatIsNotReadCommitted(t *testing.T) {
	f := newIssueBudgetFixture(t)
	ctx := context.Background()
	parent := f.issue(t, "Parent", "")
	member := f.issue(t, "Member", parent)
	otherParent := f.issue(t, "Other root's parent", "")
	f.enable(t, parent)
	f.enable(t, otherParent)

	createdRow := f.setParent(t, f.issue(t, "Created under a member", parent), parent)
	unboundRow := f.setParent(t, f.issue(t, "Unbound issue moved under a member", ""), parent)
	insideRow := f.setParent(t, f.issue(t, "Second member", parent), parent)
	f.inBinding(t, func(q *db.Queries) error { return BindIssueBudgetOnCreate(ctx, q, insideRow, pgtype.UUID{}) })
	acrossRow := f.setParent(t, member, otherParent)
	rootless := f.issue(t, "Rootless parent", "")
	strayRow := f.setParent(t, f.issue(t, "Rootless child", rootless), rootless)
	f.syncIssueCounter(t)
	committed := f.snapshot(t)

	asked := []string{"GetIssueBudgetMember", "GetIssueBudgetTransactionIsolation"}
	for _, level := range []pgx.TxIsoLevel{pgx.RepeatableRead, pgx.Serializable} {
		for _, tc := range []struct {
			name    string
			run     func(q *db.Queries) error
			refused bool
			sent    []string
		}{
			{"creating a child of a member", func(q *db.Queries) error { return BindIssueBudgetOnCreate(ctx, q, createdRow, pgtype.UUID{}) }, true, asked},
			{"moving an unbound issue under a member", func(q *db.Queries) error { return BindIssueBudgetOnReparent(ctx, q, unboundRow) }, true, asked},
			{"moving a member inside its root", func(q *db.Queries) error { return BindIssueBudgetOnReparent(ctx, q, insideRow) }, true, asked},
			{"moving a member under another root", func(q *db.Queries) error { return BindIssueBudgetOnReparent(ctx, q, acrossRow) }, true, asked},
			{"creating a child in a tree without a root", func(q *db.Queries) error { return BindIssueBudgetOnCreate(ctx, q, strayRow, pgtype.UUID{}) }, false, []string{"GetIssueBudgetMember"}},
			{"moving an issue in a tree without a root", func(q *db.Queries) error { return BindIssueBudgetOnReparent(ctx, q, strayRow) }, false, []string{"GetIssueBudgetMember"}},
			{"deleting a member", func(q *db.Queries) error {
				return TombstoneIssueBudgetMember(ctx, q, f.ws, util.MustParseUUID(member))
			}, false, []string{"TombstoneIssueBudgetMember"}},
		} {
			what := tc.name + " from " + string(level)
			tx, err := f.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: level})
			if err != nil {
				t.Fatal(err)
			}
			// Registered before the first check: a failed check must not
			// leave the connection out of the pool the cleanup closes.
			t.Cleanup(func() { _ = tx.Rollback(context.Background()) })
			if err := iteration.LockWorkspace(ctx, tx, f.ws); err != nil {
				t.Fatal(err)
			}
			log := &issueBudgetStatementLog{}
			err = tc.run(db.New(issueBudgetLoggedTx{Tx: tx, log: log}))
			if tc.refused {
				if !errors.Is(err, ErrIssueBudgetIsolation) || !strings.Contains(err.Error(), string(level)) {
					t.Fatalf("%s: got %v, want ErrIssueBudgetIsolation naming the level", what, err)
				}
			} else if err != nil {
				t.Fatalf("%s: %v", what, err)
			}
			if got := log.names(); !reflect.DeepEqual(got, tc.sent) {
				t.Fatalf("%s sent %v, want %v", what, got, tc.sent)
			}
			if err := tx.Rollback(ctx); err != nil {
				t.Fatal(err)
			}
		}

		// Through the real create path the refusal fails the issue write.
		tx, err := f.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: level})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = tx.Rollback(context.Background()) })
		_, err = f.issues.CreateInTx(ctx, tx, IssueCreateParams{
			WorkspaceID: f.ws, Title: "Created from " + string(level), Status: "todo", Priority: "none",
			CreatorType: "member", CreatorID: f.actor, AllowDuplicate: true, ParentIssueID: util.MustParseUUID(parent),
		}, IssueCountPolicy{})
		if !errors.Is(err, ErrIssueBudgetIsolation) {
			t.Fatalf("create under a member from %s: got %v, want ErrIssueBudgetIsolation", level, err)
		}
		if err := tx.Rollback(ctx); err != nil {
			t.Fatal(err)
		}
	}
	f.requireUnchanged(t, committed, "bindings refused for their isolation level")
	var titled int
	f.fx.QueryRow(t, `SELECT count(*) FROM issue WHERE workspace_id=$1 AND title LIKE 'Created from %'`, f.workspace).Scan(&titled)
	if titled != 0 {
		t.Fatalf("%d issues were created by refused writes", titled)
	}
}
