package service

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
)

func TestProjectDeletionStopsStaleAutopilotDispatch(t *testing.T) {
	for _, mode := range []string{"create_issue", "run_only"} {
		for _, reason := range []string{"project_deleted", "ordinary_pause"} {
			t.Run(mode+"/"+reason, func(t *testing.T) {
				fx, tasks, agentID, _ := triageBoundaryFixture(t)
				projectID := fx.Project(t, "Automation project")
				autopilotID := util.MustParseUUID(fx.Insert(t, "autopilot", testutil.Cols{"workspace_id": fx.WorkspaceID, "title": "Automation deletion race", "assignee_type": "agent", "assignee_id": agentID, "status": "active", "execution_mode": mode, "created_by_type": "member", "created_by_id": fx.UserID, "project_id": projectID}))
				fx.Cleanup(t, `DELETE FROM issue WHERE workspace_id=$1`, fx.WorkspaceID)
				fx.Cleanup(t, `DELETE FROM autopilot_run WHERE autopilot_id=$1`, autopilotID)
				fx.Cleanup(t, `DELETE FROM agent_task_queue WHERE agent_id=$1`, agentID)
				ap, err := tasks.Queries.GetAutopilot(t.Context(), autopilotID)
				if err != nil {
					t.Fatal(err)
				}
				// Preserve the caller's active snapshot while changing authoritative state,
				// just as a scheduler batch that was loaded before project deletion does.
				if reason == "project_deleted" {
					fx.Exec(t, `UPDATE autopilot SET project_id=NULL,status='paused',pause_reason='project_deleted' WHERE id=$1`, autopilotID)
					fx.Exec(t, `DELETE FROM project WHERE id=$1`, projectID)
				} else {
					fx.Exec(t, `UPDATE autopilot SET status='paused' WHERE id=$1`, autopilotID)
				}
				runID := util.MustParseUUID(fx.Insert(t, "autopilot_run", testutil.Cols{"autopilot_id": autopilotID, "source": "manual", "status": "running"}))
				run, err := tasks.Queries.GetAutopilotRun(t.Context(), runID)
				if err != nil {
					t.Fatal(err)
				}
				svc := NewAutopilotService(tasks.Queries, fx.Pool, tasks.Bus, tasks)
				if mode == "create_issue" {
					err = svc.dispatchCreateIssue(t.Context(), ap, &run, "UTC", util.MustParseUUID(fx.UserID))
				} else {
					err = svc.dispatchRunOnly(t.Context(), ap, &run, util.MustParseUUID(fx.UserID))
				}
				if reason == "project_deleted" {
					var skip *errDispatchSkipped
					if !errors.As(err, &skip) {
						t.Errorf("deleted project's stale dispatch must skip before creating work, got %v", err)
					}
					if n := fx.Count(t, `SELECT count(*) FROM issue WHERE workspace_id=$1`, fx.WorkspaceID); n != 0 {
						t.Errorf("stale dispatch created %d issues", n)
					}
					if n := fx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE agent_id=$1`, agentID); n != 0 {
						t.Errorf("stale dispatch created %d tasks", n)
					}
				} else if err != nil {
					t.Fatalf("ordinary paused automation must retain manual dispatch: %v", err)
				}
			})
		}
	}
}

// The final transaction, including run_only's task insertion, owns the project
// fence until commit. A row that merely passed preflight does not satisfy this.
type projectDispatchCommitStarter struct {
	base    TxStarter
	reached chan struct{}
	release chan struct{}
}

func (s projectDispatchCommitStarter) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := s.base.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &projectDispatchCommitTx{Tx: tx, reached: s.reached, release: s.release}, nil
}

type projectDispatchCommitTx struct {
	pgx.Tx
	reached chan struct{}
	release chan struct{}
}

func (tx *projectDispatchCommitTx) Commit(ctx context.Context) error {
	select {
	case tx.reached <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	}
	select {
	case <-tx.release:
	case <-ctx.Done():
		return ctx.Err()
	}
	return tx.Tx.Commit(ctx)
}
func TestProjectAutopilotDispatchRetainsAssociationFence(t *testing.T) {
	for _, mode := range []string{"create_issue", "run_only"} {
		t.Run(mode, func(t *testing.T) {
			fx, tasks, agentID, _ := triageBoundaryFixture(t)
			if mode == "run_only" {
				owner := fx.User(t, "Shared agent owner", util.UUIDToString(agentID)+"@grant.invalid")
				fx.Exec(t, `UPDATE agent SET owner_id=$2,permission_mode='public_to' WHERE id=$1`, agentID, owner)
				fx.InsertNoID(t, "agent_invocation_target", testutil.Cols{"agent_id": agentID, "target_type": "member", "target_id": fx.UserID}, "agent_id=$1", agentID)
			}

			projectID := fx.Project(t, "Dispatch fence")
			autopilotID := util.MustParseUUID(fx.Insert(t, "autopilot", testutil.Cols{"workspace_id": fx.WorkspaceID, "title": "Fence automation", "assignee_type": "agent", "assignee_id": agentID, "status": "active", "execution_mode": mode, "created_by_type": "member", "created_by_id": fx.UserID, "project_id": projectID}))
			fx.Cleanup(t, `DELETE FROM issue WHERE workspace_id=$1`, fx.WorkspaceID)
			fx.Cleanup(t, `DELETE FROM autopilot_run WHERE autopilot_id=$1`, autopilotID)
			fx.Cleanup(t, `DELETE FROM agent_task_queue WHERE agent_id=$1`, agentID)
			ap, err := tasks.Queries.GetAutopilot(t.Context(), autopilotID)
			if err != nil {
				t.Fatal(err)
			}
			runID := util.MustParseUUID(fx.Insert(t, "autopilot_run", testutil.Cols{"autopilot_id": autopilotID, "source": "manual", "status": "running"}))
			run, err := tasks.Queries.GetAutopilotRun(t.Context(), runID)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			barrier := projectDispatchCommitStarter{base: fx.Pool, reached: make(chan struct{}, 1), release: make(chan struct{})}
			defer func() {
				select {
				case <-barrier.release:
				default:
					close(barrier.release)
				}
			}()
			svc := NewAutopilotService(tasks.Queries, barrier, tasks.Bus, tasks)
			done := make(chan error, 1)
			go func() {
				if mode == "create_issue" {
					done <- svc.dispatchCreateIssue(ctx, ap, &run, "UTC", util.MustParseUUID(fx.UserID))
				} else {
					done <- svc.dispatchRunOnly(ctx, ap, &run, util.MustParseUUID(fx.UserID))
				}
			}()
			select {
			case <-barrier.reached:
			case err := <-done:
				t.Fatalf("dispatch stopped before commit: %v", err)
			case <-ctx.Done():
				t.Fatal("dispatch never reached commit")
			}
			tx, err := fx.Pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			_, err = tx.Exec(ctx, `SELECT id FROM project WHERE id=$1 FOR NO KEY UPDATE NOWAIT`, projectID)
			_ = tx.Rollback(context.Background())
			var pgerr *pgconn.PgError
			if !errors.As(err, &pgerr) || pgerr.Code != "55P03" {
				t.Errorf("dispatch did not hold association fence: %v", err)
			}
			if mode == "run_only" {
				for _, probe := range []struct {
					sql  string
					args []any
				}{{`SELECT agent_id FROM agent_invocation_target WHERE agent_id=$1 FOR UPDATE NOWAIT`, []any{agentID}}, {`SELECT id FROM member WHERE workspace_id=$1 AND user_id=$2 FOR UPDATE NOWAIT`, []any{fx.WorkspaceID, fx.UserID}}} {
					check, e := fx.Pool.Begin(ctx)
					if e != nil {
						t.Fatal(e)
					}
					_, e = check.Exec(ctx, probe.sql, probe.args...)
					_ = check.Rollback(context.Background())
					var busy *pgconn.PgError
					if !errors.As(e, &busy) || busy.Code != "55P03" {
						t.Errorf("dispatch did not stabilize invocation authority: %v", e)
					}
				}
			}

			close(barrier.release)
			select {
			case err := <-done:
				if err != nil {
					t.Fatal(err)
				}
			case <-ctx.Done():
				t.Fatal("dispatch did not finish")
			}
		})
	}
}

// Hold the actual automation row lock just before dispatch attempts owner locks.
// TeardownRuntime runs on a second connection with its normal runtime/agent locks.
type projectDispatchAutopilotBarrier struct {
	base    TxStarter
	reached chan struct{}
	release chan struct{}
	once    *sync.Once
}

func (s projectDispatchAutopilotBarrier) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := s.base.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &projectDispatchAutopilotTx{Tx: tx, reached: s.reached, release: s.release, once: s.once}, nil
}

type projectDispatchAutopilotTx struct {
	pgx.Tx
	reached chan struct{}
	release chan struct{}
	once    *sync.Once
}

func (tx *projectDispatchAutopilotTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	row := tx.Tx.QueryRow(ctx, sql, args...)
	if strings.Contains(sql, "-- name: LockAutopilotForUpdate ") {
		return projectDispatchLockedRow{Row: row, ctx: ctx, reached: tx.reached, release: tx.release, once: tx.once}
	}
	return row
}

type projectDispatchLockedRow struct {
	pgx.Row
	ctx     context.Context
	reached chan struct{}
	release chan struct{}
	once    *sync.Once
}

func (r projectDispatchLockedRow) Scan(dest ...any) error {
	if err := r.Row.Scan(dest...); err != nil {
		return err
	}
	r.once.Do(func() {
		select {
		case r.reached <- struct{}{}:
		case <-r.ctx.Done():
			return
		}
		select {
		case <-r.release:
		case <-r.ctx.Done():
		}
	})
	return nil
}
func TestProjectAutopilotRunOnlyRacesRealRuntimeTeardown(t *testing.T) {
	for _, bound := range []bool{true, false} {
		name := "without_project"
		if bound {
			name = "with_project"
		}
		t.Run(name, func(t *testing.T) { testProjectAutopilotRunOnlyRuntimeTeardown(t, bound) })
	}
}

func testProjectAutopilotRunOnlyRuntimeTeardown(t *testing.T, withProject bool) {
	fx, tasks, agentID, runtimeID := triageBoundaryFixture(t)
	projectID := fx.Project(t, "Teardown race project")
	autopilotID := util.MustParseUUID(fx.Insert(t, "autopilot", testutil.Cols{"workspace_id": fx.WorkspaceID, "title": "Runtime teardown race", "assignee_type": "agent", "assignee_id": agentID, "status": "active", "execution_mode": "run_only", "created_by_type": "member", "created_by_id": fx.UserID, "project_id": projectID}))
	fx.Cleanup(t, `DELETE FROM autopilot_run WHERE autopilot_id=$1`, autopilotID)
	fx.Cleanup(t, `DELETE FROM agent_task_queue WHERE agent_id=$1`, agentID)
	if !withProject {
		fx.Exec(t, `UPDATE autopilot SET project_id=NULL WHERE id=$1`, autopilotID)
	}

	ap, err := tasks.Queries.GetAutopilot(t.Context(), autopilotID)
	if err != nil {
		t.Fatal(err)
	}
	runID := util.MustParseUUID(fx.Insert(t, "autopilot_run", testutil.Cols{"autopilot_id": autopilotID, "source": "manual", "status": "running"}))
	run, err := tasks.Queries.GetAutopilotRun(t.Context(), runID)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 8*time.Second)
	defer cancel()
	barrier := projectDispatchAutopilotBarrier{base: fx.Pool, reached: make(chan struct{}, 1), release: make(chan struct{}), once: &sync.Once{}}
	defer func() {
		select {
		case <-barrier.release:
		default:
			close(barrier.release)
		}
	}()
	svc := NewAutopilotService(tasks.Queries, barrier, tasks.Bus, tasks)
	dispatched := make(chan error, 1)
	go func() { dispatched <- svc.dispatchRunOnly(ctx, ap, &run, util.MustParseUUID(fx.UserID)) }()
	select {
	case <-barrier.reached:
	case err := <-dispatched:
		t.Fatalf("dispatch did not lock automation: %v", err)
	case <-ctx.Done():
		t.Fatal("dispatch did not reach barrier")
	}
	teardownTx, err := fx.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer teardownTx.Rollback(context.Background())
	q := tasks.Queries.WithTx(teardownTx)
	if _, err = q.LockAgentRuntime(ctx, runtimeID); err != nil {
		t.Fatal(err)
	}
	if _, err = q.ListUserAgentsByRuntimeForUpdate(ctx, runtimeID); err != nil {
		t.Fatal(err)
	}
	tornDown := make(chan error, 1)
	go func() {
		_, e := TeardownRuntime(ctx, q, runtimeID, RuntimeTeardownOptions{CancelNonTerminalTasks: true})
		if e == nil {
			_, e = teardownTx.Exec(ctx, `DELETE FROM agent_runtime WHERE id=$1`, runtimeID)
		}
		if e == nil {
			e = teardownTx.Commit(ctx)
		} else {
			_ = teardownTx.Rollback(context.Background())
		}
		tornDown <- e
	}()
	pid := int32(teardownTx.Conn().PgConn().PID())
	blocked := false
	for !blocked {
		if err = fx.Pool.QueryRow(ctx, `SELECT cardinality(pg_blocking_pids($1))>0`, pid).Scan(&blocked); err != nil {
			t.Fatal(err)
		}
		select {
		case e := <-tornDown:
			t.Fatalf("teardown unexpectedly completed before dispatch release: %v", e)
		default:
		}
		if !blocked {
			time.Sleep(time.Millisecond)
		}
	}
	close(barrier.release)
	var dispatchErr, teardownErr error
	select {
	case dispatchErr = <-dispatched:
	case <-ctx.Done():
		t.Fatal("dispatch timed out")
	}
	select {
	case teardownErr = <-tornDown:
	case <-ctx.Done():
		t.Fatal("teardown timed out")
	}
	var skipped *errDispatchSkipped
	if dispatchErr != nil && !errors.As(dispatchErr, &skipped) {
		t.Errorf("dispatch must complete or skip without deadlock: %v", dispatchErr)
	}
	if teardownErr != nil {
		t.Errorf("real teardown must finish without deadlock: %v", teardownErr)
	}
	if n := fx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE runtime_id=$1`, runtimeID); n != 0 {
		t.Errorf("%d tasks reference the deleted runtime", n)
	}
	if n := fx.Count(t, `SELECT count(*) FROM agent_runtime WHERE id=$1`, runtimeID); n != 0 {
		t.Errorf("runtime teardown did not finish")
	}
}

type projectDispatchObservedStarter struct {
	base TxStarter
	pid  chan int32
}

func (s projectDispatchObservedStarter) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := s.base.Begin(ctx)
	if err != nil {
		return nil, err
	}
	s.pid <- int32(tx.Conn().PgConn().PID())
	return tx, nil
}
func TestProjectAutopilotTeardownCommitOrders(t *testing.T) {
	for _, order := range []string{"dispatch_first", "teardown_first"} {
		t.Run(order, func(t *testing.T) {
			fx, tasks, agentID, runtimeID := triageBoundaryFixture(t)
			projectID := fx.Project(t, "Runtime commit ordering")
			apID := util.MustParseUUID(fx.Insert(t, "autopilot", testutil.Cols{"workspace_id": fx.WorkspaceID, "title": "Commit order", "assignee_type": "agent", "assignee_id": agentID, "status": "active", "execution_mode": "run_only", "created_by_type": "member", "created_by_id": fx.UserID, "project_id": projectID}))
			fx.Cleanup(t, `DELETE FROM autopilot_run WHERE autopilot_id=$1`, apID)
			fx.Cleanup(t, `DELETE FROM agent_task_queue WHERE agent_id=$1`, agentID)
			ap, err := tasks.Queries.GetAutopilot(t.Context(), apID)
			if err != nil {
				t.Fatal(err)
			}
			runID := util.MustParseUUID(fx.Insert(t, "autopilot_run", testutil.Cols{"autopilot_id": apID, "source": "manual", "status": "running"}))
			run, err := tasks.Queries.GetAutopilotRun(t.Context(), runID)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(t.Context(), 8*time.Second)
			defer cancel()
			tx, err := fx.Pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(context.Background())
			teardown := func() error {
				if _, e := TeardownRuntime(ctx, tasks.Queries.WithTx(tx), runtimeID, RuntimeTeardownOptions{CancelNonTerminalTasks: true}); e != nil {
					return e
				}
				_, e := tx.Exec(ctx, `DELETE FROM agent_runtime WHERE id=$1`, runtimeID)
				return e
			}
			dispatched := make(chan error, 1)
			waitBlocked := func(pid int32) {
				t.Helper()
				for {
					var blocked bool
					if e := fx.Pool.QueryRow(ctx, `SELECT cardinality(pg_blocking_pids($1))>0`, pid).Scan(&blocked); e != nil {
						t.Fatal(e)
					}
					if blocked {
						return
					}
					time.Sleep(time.Millisecond)
				}
			}
			if order == "dispatch_first" {
				barrier := projectDispatchCommitStarter{base: fx.Pool, reached: make(chan struct{}, 1), release: make(chan struct{})}
				defer func() {
					select {
					case <-barrier.release:
					default:
						close(barrier.release)
					}
				}()
				svc := NewAutopilotService(tasks.Queries, barrier, tasks.Bus, tasks)
				go func() { dispatched <- svc.dispatchRunOnly(ctx, ap, &run, util.MustParseUUID(fx.UserID)) }()
				select {
				case <-barrier.reached:
				case e := <-dispatched:
					t.Fatalf("dispatch stopped: %v", e)
				case <-ctx.Done():
					t.Fatal("commit barrier timed out")
				}
				teardownPID := int32(tx.Conn().PgConn().PID())
				tornDown := make(chan error, 1)
				go func() {
					e := teardown()
					if e == nil {
						e = tx.Commit(ctx)
					}
					tornDown <- e
				}()
				waitBlocked(teardownPID)
				close(barrier.release)
				select {
				case e := <-dispatched:
					if e != nil {
						t.Fatal(e)
					}
				case <-ctx.Done():
					t.Fatal("dispatch timed out")
				}
				select {
				case e := <-tornDown:
					if e != nil {
						t.Fatal(e)
					}
				case <-ctx.Done():
					t.Fatal("teardown timed out")
				}
				if n := fx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE agent_id=$1 AND status='cancelled' AND runtime_id IS NULL`, agentID); n != 1 {
					t.Errorf("teardown should preserve cancelled task history, got %d", n)
				}
			} else {
				if err = teardown(); err != nil {
					t.Fatal(err)
				}
				pid := make(chan int32, 1)
				svc := NewAutopilotService(tasks.Queries, projectDispatchObservedStarter{base: fx.Pool, pid: pid}, tasks.Bus, tasks)
				go func() { dispatched <- svc.dispatchRunOnly(ctx, ap, &run, util.MustParseUUID(fx.UserID)) }()
				var dispatchPID int32
				select {
				case dispatchPID = <-pid:
				case e := <-dispatched:
					t.Fatalf("dispatch did not reach stale final transaction: %v", e)
				case <-ctx.Done():
					t.Fatal("dispatch timed out")
				}
				waitBlocked(dispatchPID)
				if err = tx.Commit(ctx); err != nil {
					t.Fatal(err)
				}
				select {
				case e := <-dispatched:
					var skipped *errDispatchSkipped
					if !errors.As(e, &skipped) {
						t.Fatalf("late dispatch should skip after teardown: %v", e)
					}
				case <-ctx.Done():
					t.Fatal("dispatch timed out")
				}
				if n := fx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE agent_id=$1`, agentID); n != 0 {
					t.Errorf("late dispatch inserted %d tasks", n)
				}
			}
			if n := fx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE runtime_id=$1`, runtimeID); n != 0 {
				t.Errorf("%d tasks retain deleted runtime", n)
			}
			if n := fx.Count(t, `SELECT count(*) FROM agent_runtime WHERE id=$1`, runtimeID); n != 0 {
				t.Error("runtime not deleted")
			}
		})
	}
}

func TestProjectAutopilotRunOnlyRevalidatesLockedOwners(t *testing.T) {
	for _, change := range []string{"runtime_offline", "agent_permission", "runtime_binding", "member_removed"} {
		t.Run(change, func(t *testing.T) {
			fx, tasks, agentID, runtimeID := triageBoundaryFixture(t)
			other := fx.User(t, "Changed owner", util.UUIDToString(agentID)+"@owner.invalid")
			apID := util.MustParseUUID(fx.Insert(t, "autopilot", testutil.Cols{"workspace_id": fx.WorkspaceID, "title": "Locked validation", "assignee_type": "agent", "assignee_id": agentID, "status": "active", "execution_mode": "run_only", "created_by_type": "member", "created_by_id": fx.UserID}))
			fx.Cleanup(t, `DELETE FROM autopilot_run WHERE autopilot_id=$1`, apID)
			fx.Cleanup(t, `DELETE FROM agent_task_queue WHERE agent_id=$1`, agentID)
			ap, err := tasks.Queries.GetAutopilot(t.Context(), apID)
			if err != nil {
				t.Fatal(err)
			}
			runID := util.MustParseUUID(fx.Insert(t, "autopilot_run", testutil.Cols{"autopilot_id": apID, "source": "manual", "status": "running"}))
			run, err := tasks.Queries.GetAutopilotRun(t.Context(), runID)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			barrier := projectDispatchAutopilotBarrier{base: fx.Pool, reached: make(chan struct{}, 1), release: make(chan struct{}), once: &sync.Once{}}
			defer func() {
				select {
				case <-barrier.release:
				default:
					close(barrier.release)
				}
			}()
			svc := NewAutopilotService(tasks.Queries, barrier, tasks.Bus, tasks)
			done := make(chan error, 1)
			go func() { done <- svc.dispatchRunOnly(ctx, ap, &run, util.MustParseUUID(fx.UserID)) }()
			select {
			case <-barrier.reached:
			case e := <-done:
				t.Fatalf("dispatch stopped early: %v", e)
			case <-ctx.Done():
				t.Fatal("barrier timed out")
			}
			switch change {
			case "runtime_offline":
				fx.Exec(t, `UPDATE agent_runtime SET status='offline' WHERE id=$1`, runtimeID)
			case "agent_permission":
				fx.Exec(t, `UPDATE agent SET owner_id=$2,permission_mode='private' WHERE id=$1`, agentID, other)
			case "runtime_binding":
				fx.Exec(t, `UPDATE agent SET runtime_id=NULL WHERE id=$1`, agentID)
			case "member_removed":
				fx.Exec(t, `DELETE FROM member WHERE workspace_id=$1 AND user_id=$2`, fx.WorkspaceID, fx.UserID)
			}
			close(barrier.release)
			select {
			case e := <-done:
				var skipped *errDispatchSkipped
				if !errors.As(e, &skipped) {
					t.Errorf("changed owner state must skip: %v", e)
				}
			case <-ctx.Done():
				t.Fatal("dispatch timed out")
			}
			if n := fx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE agent_id=$1`, agentID); n != 0 {
				t.Errorf("changed owner state inserted %d tasks", n)
			}
		})
	}
}
