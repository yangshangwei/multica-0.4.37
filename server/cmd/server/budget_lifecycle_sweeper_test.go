package main

import (
	"context"
	"errors"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"

	obsmetrics "github.com/multica-ai/multica/server/internal/metrics"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// The loop that reconciles the lifecycle of budget roots (design 10.3, the
// trigger, with the rulings of 2026-11-20): where it runs, what one round
// reports and what it sends.
//
// What a reconciliation does to one root, and the loop over roots that fail,
// run out of their time or are skipped, is tested in internal/service
// (issue_budget_lifecycle_test.go, issue_budget_lifecycle_round_test.go).
// Here the round runs as the server runs it, over every workspace. The two
// database cases therefore want a database in which no other test has an
// open budget attempt of an ended task at that moment: this package run on
// its own, which is how the repository's test entry point runs packages.

// TestBudgetLifecycleRunsOutsideTheLivenessLoop pins where the step runs. The
// 30-second liveness tick drops its next rounds while one is running, so work
// that takes a root lock and a transaction per root has a loop of its own,
// started by main, with no observer: a failed round never reaches the health
// of the task sweeper.
func TestBudgetLifecycleRunsOutsideTheLivenessLoop(t *testing.T) {
	if budgetLifecycleSweepInterval != 30*time.Second || budgetLifecycleBatchSize != 500 || budgetLifecycleOperationTimeout != 5*time.Second {
		t.Fatalf("budget lifecycle loop: every %s, %d roots a round, %s a root; want 30s, 500, 5s",
			budgetLifecycleSweepInterval, budgetLifecycleBatchSize, budgetLifecycleOperationTimeout)
	}
	read := func(name string) string {
		source, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		return string(source)
	}

	liveness := read("runtime_sweeper.go")
	for _, word := range []string{"ReconcileLifecycles", "sweepBudgetLifecycles", "runBudgetLifecycleSweeper", "RuntimeSweepStageBudgetLifecycle"} {
		if strings.Contains(liveness, word) {
			t.Fatalf("runtime_sweeper.go mentions %s; the budget lifecycle reconciliation belongs on runBudgetLifecycleSweeper", word)
		}
	}

	loop := read("budget_lifecycle_sweeper.go")
	_, signature, ok := strings.Cut(loop, "\nfunc runBudgetLifecycleSweeper(")
	if !ok {
		t.Fatal("budget_lifecycle_sweeper.go has no runBudgetLifecycleSweeper")
	}
	signature, _, _ = strings.Cut(signature, "\n")
	if strings.Contains(signature, "func(") || strings.Contains(signature, "...") {
		t.Fatalf("runBudgetLifecycleSweeper takes an observer: %s", signature)
	}

	main := read("main.go")
	const start = "go runBudgetLifecycleSweeper(sweepCtx, "
	if n := strings.Count(main, start); n != 1 {
		t.Fatalf("main.go starts the budget lifecycle loop %d times, want once", n)
	}
	_, call, _ := strings.Cut(main, start)
	call, _, _ = strings.Cut(call, "\n")
	if strings.Contains(call, "bservation") || strings.Contains(call, ".record") {
		t.Fatalf("main.go hands the budget lifecycle loop an observer: %s", call)
	}
}

// fakeBudgetLifecycles answers a round with what the test gives it.
type fakeBudgetLifecycles struct {
	round    service.IssueBudgetLifecycleRound
	err      error
	calls    int
	maxRoots int32
	each     time.Duration
}

func (f *fakeBudgetLifecycles) ReconcileLifecycles(_ context.Context, maxRoots int32, each time.Duration) (service.IssueBudgetLifecycleRound, error) {
	f.calls++
	f.maxRoots, f.each = maxRoots, each
	return f.round, f.err
}

// budgetLifecycleStageSeries is what the sweeper counters of metrics hold, by
// family and stage label.
func budgetLifecycleStageSeries(t *testing.T, metrics *obsmetrics.BusinessMetrics) map[string]map[string]float64 {
	t.Helper()
	registry := prometheus.NewRegistry()
	registry.MustRegister(metrics.Collectors()...)
	families, err := registry.Gather()
	if err != nil {
		t.Fatalf("gather the sweeper metrics: %v", err)
	}
	series := map[string]map[string]float64{}
	for _, family := range families {
		name := family.GetName()
		if name != "multica_runtime_sweeper_candidate_rows_total" && name != "multica_runtime_sweeper_rows_changed_total" {
			continue
		}
		series[name] = map[string]float64{}
		for _, metric := range family.Metric {
			for _, label := range metric.Label {
				if label.GetName() == "stage" {
					series[name][label.GetValue()] = metric.GetCounter().GetValue()
				}
			}
		}
	}
	return series
}

// TestBudgetLifecycleSweepReportsWhatTheRoundDid: one round asks the service
// once, with the bound on the roots and the time for each. Its candidates are
// the roots the round found and its changes the attempts it closed; skipped
// and failed roots are neither. Both are counted under the stage of the step
// and under no other. A round whose read failed is marked failed and reports
// nothing else.
func TestBudgetLifecycleSweepReportsWhatTheRoundDid(t *testing.T) {
	budgets := &fakeBudgetLifecycles{round: service.IssueBudgetLifecycleRound{Roots: 7, Closed: 9, Skipped: 2, Failed: 1}}
	metrics := obsmetrics.NewBusinessMetrics()
	stats := sweepBudgetLifecycles(context.Background(), budgets, metrics)
	if want := (runtimeSweepStageStats{candidates: 7, changed: 9}); stats != want {
		t.Fatalf("the round reports %+v, want %+v", stats, want)
	}
	if budgets.calls != 1 || budgets.maxRoots != 500 || budgets.each != 5*time.Second {
		t.Fatalf("the round asked the service %d times for %d roots with %s each; want once, 500, 5s", budgets.calls, budgets.maxRoots, budgets.each)
	}
	if got, want := budgetLifecycleStageSeries(t, metrics), (map[string]map[string]float64{
		"multica_runtime_sweeper_candidate_rows_total": {"budget_lifecycle": 7},
		"multica_runtime_sweeper_rows_changed_total":   {"budget_lifecycle": 9},
	}); !reflect.DeepEqual(got, want) {
		t.Fatalf("the round counted %v, want %v", got, want)
	}

	failing := &fakeBudgetLifecycles{
		round: service.IssueBudgetLifecycleRound{Roots: 3, Closed: 3},
		err:   errors.New("test: the read failed"),
	}
	stats = sweepBudgetLifecycles(context.Background(), failing, nil)
	if want := (runtimeSweepStageStats{failed: true}); stats != want {
		t.Fatalf("a round whose read failed reports %+v, want %+v", stats, want)
	}
}

// TestBudgetLifecycleSweeperStopsWithItsContext keeps shutdown clean: the loop
// ends with its context and does not wait out an interval.
func TestBudgetLifecycleSweeperStopsWithItsContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	stopped := make(chan struct{})
	go func() {
		runBudgetLifecycleSweeper(ctx, &fakeBudgetLifecycles{}, nil)
		close(stopped)
	}()
	cancel()
	select {
	case <-stopped:
	case <-time.After(5 * time.Second):
		t.Fatal("the budget lifecycle loop did not stop with its context")
	}
}

// budgetLifecycleStatements is a pool that records every statement sent
// outside a transaction and counts the transactions begun.
type budgetLifecycleStatements struct {
	*pgxpool.Pool
	mu         sync.Mutex
	statements []string
	// left is the time each statement had left when it was sent; zero for a
	// statement sent without a deadline.
	left  []time.Duration
	begun int
}

func (s *budgetLifecycleStatements) record(ctx context.Context, sql string) {
	name := strings.TrimSpace(sql)
	if rest, ok := strings.CutPrefix(name, "-- name: "); ok {
		name, _, _ = strings.Cut(rest, " ")
	}
	var left time.Duration
	if deadline, ok := ctx.Deadline(); ok {
		left = time.Until(deadline)
	}
	s.mu.Lock()
	s.statements = append(s.statements, name)
	s.left = append(s.left, left)
	s.mu.Unlock()
}

func (s *budgetLifecycleStatements) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	s.record(ctx, sql)
	return s.Pool.Exec(ctx, sql, args...)
}

func (s *budgetLifecycleStatements) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	s.record(ctx, sql)
	return s.Pool.Query(ctx, sql, args...)
}

func (s *budgetLifecycleStatements) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	s.record(ctx, sql)
	return s.Pool.QueryRow(ctx, sql, args...)
}

func (s *budgetLifecycleStatements) SendBatch(ctx context.Context, batch *pgx.Batch) pgx.BatchResults {
	s.record(ctx, "SendBatch")
	return s.Pool.SendBatch(ctx, batch)
}

func (s *budgetLifecycleStatements) Begin(ctx context.Context) (pgx.Tx, error) {
	s.mu.Lock()
	s.begun++
	s.mu.Unlock()
	return s.Pool.Begin(ctx)
}

// budgetLifecycleFixture is a workspace of its own with an agent, in which a
// test builds budget roots and attempts.
type budgetLifecycleFixture struct {
	fx        *testutil.Fixture
	svc       *service.IssueBudgetService
	workspace pgtype.UUID
	agent     string
	runtime   string
}

func newBudgetLifecycleFixture(t *testing.T) *budgetLifecycleFixture {
	t.Helper()
	setup := testutil.New(testPool, testWorkspaceID, testUserID)
	suffix := strings.ReplaceAll(uuid.NewString(), "-", "")
	workspace := setup.Workspace(t, "Budget lifecycle sweep", "budget-sweep-"+suffix)
	fx := testutil.New(testPool, workspace, testUserID)
	fx.Member(t, workspace, testUserID, "owner")
	// Budget rows have no foreign key, so nothing else removes them.
	fx.Cleanup(t, `DELETE FROM issue WHERE workspace_id=$1`, workspace)
	for _, table := range []string{
		"issue_budget_root", "issue_budget_member", "issue_budget_attempt",
		"issue_budget_entry", "issue_budget_operation", "issue_budget_observation",
	} {
		fx.Cleanup(t, `DELETE FROM `+table+` WHERE workspace_id=$1`, workspace)
	}
	runtime := fx.Runtime(t, "budget-sweep-runtime-"+suffix)
	return &budgetLifecycleFixture{
		fx:        fx,
		svc:       service.NewIssueBudgetService(db.New(testPool), testPool),
		workspace: util.MustParseUUID(workspace),
		agent:     fx.Agent(t, "budget-sweep-agent-"+suffix, runtime),
		runtime:   runtime,
	}
}

// budgetLifecycleAttempt is a root of its own with one open attempt.
type budgetLifecycleAttempt struct {
	root, attempt, task string
}

// openAttempt enables a root on a new issue and writes one open attempt of a
// task in the status given. The attempt has no reservation in the root's
// ledger, as an attempt claimed while the ledger was unavailable has none: a
// reconciliation closes it and writes no hold.
func (f *budgetLifecycleFixture) openAttempt(t *testing.T, status string) budgetLifecycleAttempt {
	t.Helper()
	issue := f.fx.Issue(t, "Budget lifecycle sweep "+status)
	enabled, err := f.svc.Enable(context.Background(), service.IssueBudgetEnableParams{
		WorkspaceID: f.workspace, IssueID: util.MustParseUUID(issue),
		OperationKey: "enable-" + uuid.NewString(), Reason: "track this parent task",
		ActorUserID: util.MustParseUUID(testUserID),
	})
	if err != nil {
		t.Fatalf("enable a budget root: %v", err)
	}
	task := f.fx.Task(t, f.agent, testutil.Cols{"runtime_id": f.runtime, "issue_id": issue, "status": status, "claim_generation": 1})
	attempt := f.fx.Insert(t, "issue_budget_attempt", testutil.Cols{
		"id": uuid.NewString(), "workspace_id": f.fx.WorkspaceID, "budget_root_id": enabled.BudgetRootID,
		"task_id": task, "claim_generation": 1, "runtime_id": f.runtime, "agent_id": f.agent, "issue_id": issue,
	})
	return budgetLifecycleAttempt{root: enabled.BudgetRootID, attempt: attempt, task: task}
}

func (f *budgetLifecycleFixture) state(t *testing.T, a budgetLifecycleAttempt) string {
	t.Helper()
	var state string
	f.fx.QueryRow(t, `SELECT state FROM issue_budget_attempt WHERE id=$1`, a.attempt).Scan(&state)
	return state
}

// requireNoForeignWork fails unless the roots the read finds are the test's
// own: the two cases below are about a whole round.
func (f *budgetLifecycleFixture) requireNoForeignWork(t *testing.T) {
	t.Helper()
	rows, err := db.New(testPool).ListIssueBudgetRootsWithEndedAttempts(context.Background(), 1<<30)
	if err != nil {
		t.Fatalf("find the roots with ended attempts: %v", err)
	}
	for _, row := range rows {
		if row.WorkspaceID != f.workspace {
			t.Fatalf("the database has a budget root with an ended attempt that this test did not create (%s in workspace %s): run this package on a database of its own",
				util.UUIDToString(row.ID), util.UUIDToString(row.WorkspaceID))
		}
	}
}

// TestBudgetLifecycleSweepWithNothingToReconcileIsOneRead: a round that finds
// no root sends the one read and nothing else. It begins no transaction, so
// it takes no root lock and writes nothing. A root whose attempts all have a
// running task, and a root that is off although its task ended, are such
// nothing: neither is locked to find that out. The read has the time one root
// has and no more.
func TestBudgetLifecycleSweepWithNothingToReconcileIsOneRead(t *testing.T) {
	f := newBudgetLifecycleFixture(t)
	ctx := context.Background()
	running := f.openAttempt(t, "running")
	off := f.openAttempt(t, "completed")
	if _, err := f.svc.SetMode(ctx, service.IssueBudgetManageParams{
		WorkspaceID: f.workspace, BudgetRootID: util.MustParseUUID(off.root),
		OperationKey: "off-" + uuid.NewString(), ExpectedPolicyRevision: 1,
		Reason: "operator decision", ActorUserID: util.MustParseUUID(testUserID),
	}, service.IssueBudgetModeOff); err != nil {
		t.Fatalf("switch the root off: %v", err)
	}
	f.requireNoForeignWork(t)

	sent := &budgetLifecycleStatements{Pool: testPool}
	stats := sweepBudgetLifecycles(ctx, service.NewIssueBudgetService(db.New(sent), sent), nil)
	if stats != (runtimeSweepStageStats{}) {
		t.Fatalf("a round with nothing to reconcile reports %+v, want no candidate, no change and no failure", stats)
	}
	if want := []string{"ListIssueBudgetRootsWithEndedAttempts"}; !reflect.DeepEqual(sent.statements, want) || sent.begun != 0 {
		t.Fatalf("a round with nothing to reconcile sent %v and began %d transactions, want %v and none", sent.statements, sent.begun, want)
	}
	if left := sent.left[0]; left <= 0 || left > budgetLifecycleOperationTimeout {
		t.Fatalf("the read was sent with %s left, want a deadline of at most %s", left, budgetLifecycleOperationTimeout)
	}
	for name, a := range map[string]budgetLifecycleAttempt{"the attempt whose task runs": running, "the attempt of the root that is off": off} {
		if state := f.state(t, a); state != "open" {
			t.Fatalf("%s is %q after the round, want open", name, state)
		}
	}
}

// TestBudgetLifecycleSweepReconcilesTheRootsItFinds: a round closes the open
// attempts whose task ended or whose task row is gone, and leaves the one
// whose task runs. The next round finds nothing. With a bound of one root a
// round takes the first root in root order, and the round after it the next:
// a reconciled root is not found again, which is all the cursor there is.
func TestBudgetLifecycleSweepReconcilesTheRootsItFinds(t *testing.T) {
	f := newBudgetLifecycleFixture(t)
	ctx := context.Background()
	ended := f.openAttempt(t, "completed")
	gone := f.openAttempt(t, "dispatched")
	f.fx.Exec(t, `DELETE FROM agent_task_queue WHERE id=$1`, gone.task)
	running := f.openAttempt(t, "running")
	f.requireNoForeignWork(t)

	stats := sweepBudgetLifecycles(ctx, f.svc, nil)
	if want := (runtimeSweepStageStats{candidates: 2, changed: 2}); stats != want {
		t.Fatalf("the round reports %+v, want %+v", stats, want)
	}
	for name, want := range map[string]struct {
		attempt budgetLifecycleAttempt
		state   string
	}{
		"the attempt of the completed task":       {ended, "process_exit"},
		"the attempt whose task row is gone":      {gone, "lost_contact"},
		"the attempt whose task is still running": {running, "open"},
	} {
		if state := f.state(t, want.attempt); state != want.state {
			t.Fatalf("%s is %q after the round, want %q", name, state, want.state)
		}
	}
	if stats := sweepBudgetLifecycles(ctx, f.svc, nil); stats != (runtimeSweepStageStats{}) {
		t.Fatalf("the round after it reports %+v, want nothing to reconcile", stats)
	}

	first, second := f.openAttempt(t, "cancelled"), f.openAttempt(t, "failed")
	if second.root < first.root {
		first, second = second, first
	}
	for i, want := range []struct {
		round         service.IssueBudgetLifecycleRound
		first, second string
	}{
		{service.IssueBudgetLifecycleRound{Roots: 1, Closed: 1}, "closed", "open"},
		{service.IssueBudgetLifecycleRound{Roots: 1, Closed: 1}, "closed", "closed"},
		{service.IssueBudgetLifecycleRound{}, "closed", "closed"},
	} {
		round, err := f.svc.ReconcileLifecycles(ctx, 1, time.Minute)
		if err != nil || round != want.round {
			t.Fatalf("round %d with a bound of one root reports %+v (%v), want %+v", i+1, round, err, want.round)
		}
		for name, a := range map[string]struct {
			attempt budgetLifecycleAttempt
			want    string
		}{"the first root in root order": {first, want.first}, "the second root in root order": {second, want.second}} {
			if got := f.state(t, a.attempt) != "open"; got != (a.want == "closed") {
				t.Fatalf("round %d with a bound of one root: the attempt of %s is closed: %v, want %s", i+1, name, got, a.want)
			}
		}
	}
}
