package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func failedIssueIterationFixture(t *testing.T) (*testutil.Fixture, *TaskService, db.Issue, db.AgentTaskQueue, string) {
	t.Helper()
	fx, s, agent, runtime := triageBoundaryFixture(t)
	iterationID := fx.Insert(t, "iteration", testutil.Cols{"workspace_id": fx.WorkspaceID, "name": "Failed task iteration", "timezone": "UTC", "start_date": "2026-10-01", "end_date": "2026-10-14", "status": "active", "created_by": fx.UserID, "started_at": testutil.Raw("clock_timestamp()")})
	issueID := fx.Issue(t, "In progress commitment", testutil.Cols{"status": "in_progress", "current_iteration_id": iterationID, "iteration_rollover_count": 2})
	fx.InsertNoID(t, "iteration_participation", testutil.Cols{"workspace_id": fx.WorkspaceID, "iteration_id": iterationID, "issue_id": issueID, "first_joined_at": testutil.Raw("clock_timestamp()"), "current_joined_at": testutil.Raw("clock_timestamp()"), "has_started_current_participation": true, "in_original": true, "original_facts": `{"status_category":"in_progress"}`}, "iteration_id=$1 AND issue_id=$2", iterationID, issueID)
	fx.Cleanup(t, `DELETE FROM iteration_event WHERE iteration_id=$1`, iterationID)
	taskID := fx.Task(t, util.UUIDToString(agent), testutil.Cols{"runtime_id": runtime, "issue_id": issueID, "status": "failed", "attempt": 1, "max_attempts": 1, "error": "failed", "failure_reason": "agent_error.process_failure"})
	issue, err := s.Queries.GetIssue(t.Context(), util.MustParseUUID(issueID))
	if err != nil {
		t.Fatal(err)
	}
	task, err := s.Queries.GetAgentTask(t.Context(), util.MustParseUUID(taskID))
	if err != nil {
		t.Fatal(err)
	}
	return fx, s, issue, task, iterationID
}

func TestHandleFailedTasksIterationFacts(t *testing.T) {
	fx, s, issue, task, iterationID := failedIssueIterationFixture(t)
	if retries := s.HandleFailedTasks(t.Context(), []db.AgentTaskQueue{task, task}); retries != 0 {
		t.Fatalf("exhausted task retried %d times", retries)
	}
	s.HandleFailedTasks(t.Context(), []db.AgentTaskQueue{task})
	updated, err := s.Queries.GetIssue(t.Context(), issue.ID)
	if err != nil {
		t.Fatal(err)
	}
	events := fx.Count(t, `SELECT count(*) FROM iteration_event WHERE iteration_id=$1`, iterationID)
	var scope int64
	fx.QueryRow(t, `SELECT scope_revision FROM iteration WHERE id=$1`, iterationID).Scan(&scope)
	if updated.Status != "todo" || updated.Revision != issue.Revision+1 || events != 1 || scope != 2 {
		t.Fatalf("reset facts not atomic status=%s revision=%d events=%d scope=%d", updated.Status, updated.Revision, events, scope)
	}
	if updated.CurrentIterationID != issue.CurrentIterationID || updated.IterationRolloverCount != 2 {
		t.Fatal("failure reset lost iteration fields")
	}
	var actorJSON, originalJSON []byte
	fx.QueryRow(t, `SELECT actor FROM iteration_event WHERE iteration_id=$1`, iterationID).Scan(&actorJSON)
	fx.QueryRow(t, `SELECT original_facts FROM iteration_participation WHERE iteration_id=$1`, iterationID).Scan(&originalJSON)
	var actor, originalFacts map[string]any
	if err = json.Unmarshal(actorJSON, &actor); err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(originalJSON, &originalFacts); err != nil {
		t.Fatal(err)
	}
	if actor["type"] != "system" || actor["source"] != "failed_task_reset" || actor["id"] != nil || actor["user_id"] != nil || originalFacts["status_category"] != "in_progress" {
		t.Fatalf("system actor or frozen original facts drift: %s / %s", actorJSON, originalJSON)
	}

	var started, original bool
	fx.QueryRow(t, `SELECT has_started_current_participation,in_original FROM iteration_participation WHERE iteration_id=$1`, iterationID).Scan(&started, &original)
	if !started || !original {
		t.Fatal("failure reset erased original/started history")
	}
}

func TestHandleFailedTasksIterationExclusions(t *testing.T) {
	for _, scenario := range []string{"custom_review", "blocked", "custom_working", "active", "retry"} {
		t.Run(scenario, func(t *testing.T) {
			fx, s, issue, task, iterationID := failedIssueIterationFixture(t)
			wantStatus := "in_progress"
			wantEvents := 0
			switch scenario {
			case "custom_review", "custom_working":
				category := "in_review"
				if scenario == "custom_working" {
					category = "in_progress"
					wantEvents = 1
				}
				fx.Insert(t, "issue_status", testutil.Cols{"workspace_id": fx.WorkspaceID, "key": scenario, "name": scenario, "category": category, "color": "#ff0000", "position": 10})
				fx.Exec(t, `UPDATE issue SET status=$1 WHERE id=$2`, scenario, issue.ID)
				wantStatus = scenario
				if wantEvents == 1 {
					wantStatus = "todo"
				}
			case "blocked":
				fx.Exec(t, `UPDATE issue SET status='blocked' WHERE id=$1`, issue.ID)
				wantStatus = "blocked"
			case "active":
				fx.Task(t, util.UUIDToString(task.AgentID), testutil.Cols{"runtime_id": task.RuntimeID, "issue_id": issue.ID, "status": "running"})
			case "retry":
				fx.Exec(t, `UPDATE agent_task_queue SET failure_reason='timeout',max_attempts=2 WHERE id=$1`, task.ID)
				var err error
				task, err = s.Queries.GetAgentTask(t.Context(), task.ID)
				if err != nil {
					t.Fatal(err)
				}
			}
			retries := s.HandleFailedTasks(t.Context(), []db.AgentTaskQueue{task})
			if scenario == "retry" && retries != 1 {
				t.Fatalf("retry-pending fixture retried=%d", retries)
			}
			updated, err := s.Queries.GetIssue(t.Context(), issue.ID)
			if err != nil {
				t.Fatal(err)
			}
			events := fx.Count(t, `SELECT count(*) FROM iteration_event WHERE iteration_id=$1`, iterationID)
			if updated.Status != wantStatus || events != wantEvents {
				t.Fatalf("scenario=%s status=%s events=%d", scenario, updated.Status, events)
			}
		})
	}
}

type statusEventFailStarter struct{ inner TxStarter }

func (s statusEventFailStarter) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := s.inner.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &statusEventFailTx{Tx: tx}, nil
}

type statusEventFailTx struct{ pgx.Tx }

func (tx *statusEventFailTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if strings.Contains(sql, "AppendIterationIssueEvent") {
		return pgconn.CommandTag{}, errors.New("injected status event failure")
	}
	return tx.Tx.Exec(ctx, sql, args...)
}
func TestHandleFailedTasksIterationRecorderRollback(t *testing.T) {
	fx, s, issue, task, iterationID := failedIssueIterationFixture(t)
	var updates atomic.Int32
	s.Bus.Subscribe(protocol.EventIssueUpdated, func(events.Event) { updates.Add(1) })
	s.TxStarter = statusEventFailStarter{s.TxStarter}
	s.HandleFailedTasks(t.Context(), []db.AgentTaskQueue{task})
	if updates.Load() != 0 {
		t.Fatal("failed reset published issue:updated")
	}
	updated, err := s.Queries.GetIssue(t.Context(), issue.ID)
	if err != nil {
		t.Fatal(err)
	}
	failed, err := s.Queries.GetAgentTask(t.Context(), task.ID)
	if err != nil {
		t.Fatal(err)
	}
	var scope int64
	fx.QueryRow(t, `SELECT scope_revision FROM iteration WHERE id=$1`, iterationID).Scan(&scope)
	if updated.Status != issue.Status || updated.Revision != issue.Revision || failed.Status != "failed" || scope != 1 || fx.Count(t, `SELECT count(*) FROM iteration_event WHERE iteration_id=$1`, iterationID) != 0 {
		t.Fatalf("reset failure partial write issue=%s revision=%d task=%s scope=%d", updated.Status, updated.Revision, failed.Status, scope)
	}
}

type statusWriteBarrierStarter struct {
	inner      TxStarter
	locked     chan<- uint32
	commit     chan<- struct{}
	release    <-chan struct{}
	repeatable bool
}

func (s statusWriteBarrierStarter) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := s.inner.Begin(ctx)
	if err != nil {
		return nil, err
	}
	if s.repeatable {
		if _, err = tx.Exec(ctx, "SET TRANSACTION ISOLATION LEVEL REPEATABLE READ"); err != nil {
			tx.Rollback(ctx)
			return nil, err
		}
	}
	return &statusWriteBarrierTx{Tx: tx, owner: s}, nil
}

type statusWriteBarrierTx struct {
	pgx.Tx
	owner       statusWriteBarrierStarter
	wroteStatus bool
}

func (tx *statusWriteBarrierTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if strings.Contains(sql, "UpdateIssueStatus") {
		tx.wroteStatus = true
	}
	if tx.owner.locked != nil && strings.Contains(sql, "LockIssueForDescriptionUpdate") {
		tx.owner.locked <- tx.Conn().PgConn().PID()
	}
	return tx.Tx.QueryRow(ctx, sql, args...)
}
func (tx *statusWriteBarrierTx) Commit(ctx context.Context) error {
	if tx.owner.commit != nil && tx.wroteStatus {
		tx.owner.commit <- struct{}{}
		select {
		case <-tx.owner.release:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return tx.Tx.Commit(ctx)
}

func TestHandleFailedTasksIterationExecutionLockOrders(t *testing.T) {
	t.Run("enqueue_first_forces_fresh_RC_read", func(t *testing.T) {
		fx, s, issue, task, iterationID := failedIssueIterationFixture(t)
		ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
		defer cancel()
		enqueue, err := fx.Pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer enqueue.Rollback(ctx)
		created, err := db.New(enqueue).CreateAgentTask(ctx, db.CreateAgentTaskParams{AgentID: task.AgentID, RuntimeID: task.RuntimeID, IssueID: issue.ID})
		if err != nil {
			t.Fatal(err)
		}
		fx.Cleanup(t, `DELETE FROM agent_task_queue WHERE id=$1`, created.ID)
		reached := make(chan uint32, 1)
		s.TxStarter = statusWriteBarrierStarter{inner: s.TxStarter, locked: reached, repeatable: true}
		done := make(chan struct{})
		go func() { s.HandleFailedTasks(ctx, []db.AgentTaskQueue{task}); close(done) }()
		var pid uint32
		select {
		case pid = <-reached:
		case <-ctx.Done():
			t.Fatal("reset did not reach issue lock")
		}
		waiting := false
		for !waiting && ctx.Err() == nil {
			if err = fx.Pool.QueryRow(ctx, `SELECT $1::int=ANY(pg_blocking_pids($2::int))`, enqueue.Conn().PgConn().PID(), pid).Scan(&waiting); err != nil {
				t.Fatal(err)
			}
			if !waiting {
				time.Sleep(time.Millisecond)
			}
		}
		if !waiting {
			t.Fatal("reset did not actually wait on enqueue KEY SHARE")
		}
		if err = enqueue.Commit(ctx); err != nil {
			t.Fatal(err)
		}
		select {
		case <-done:
		case <-ctx.Done():
			t.Fatal("reset did not finish")
		}
		updated, err := s.Queries.GetIssue(ctx, issue.ID)
		if err != nil {
			t.Fatal(err)
		}
		if updated.Status != "in_progress" || fx.Count(t, `SELECT count(*) FROM iteration_event WHERE iteration_id=$1`, iterationID) != 0 {
			t.Fatal("reset used stale snapshot and missed committed active enqueue")
		}
	})
	t.Run("reset_first_excludes_enqueue", func(t *testing.T) {
		fx, s, issue, task, iterationID := failedIssueIterationFixture(t)
		var updates atomic.Int32
		s.Bus.Subscribe(protocol.EventIssueUpdated, func(events.Event) { updates.Add(1) })
		ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
		defer cancel()
		reached, release := make(chan struct{}, 1), make(chan struct{})
		locked := make(chan uint32, 1)
		s.TxStarter = statusWriteBarrierStarter{inner: s.TxStarter, commit: reached, release: release, locked: locked}
		done := make(chan struct{})
		go func() { s.HandleFailedTasks(ctx, []db.AgentTaskQueue{task}); close(done) }()
		select {
		case <-reached:
		case <-ctx.Done():
			t.Fatal("reset did not reach commit")
		}
		if updates.Load() != 0 {
			t.Fatal("reset published before commit")
		}
		var resetPID uint32
		select {
		case resetPID = <-locked:
		case <-ctx.Done():
			t.Fatal("reset issue lock hook missing")
		}
		var admitted bool
		if err := fx.Pool.QueryRow(ctx, `SELECT lock_issue_execution($1::uuid)`, issue.ID).Scan(&admitted); err != nil {
			t.Fatal(err)
		}
		if admitted {
			t.Fatal("execution admission passed reset FOR UPDATE lock")
		}
		enqueue, err := fx.Pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer enqueue.Rollback(ctx)
		queued := make(chan error, 1)
		go func() {
			created, e := db.New(enqueue).CreateAgentTask(ctx, db.CreateAgentTaskParams{AgentID: task.AgentID, RuntimeID: task.RuntimeID, IssueID: issue.ID})
			if e == nil {
				fx.Cleanup(t, `DELETE FROM agent_task_queue WHERE id=$1`, created.ID)
				e = enqueue.Commit(ctx)
			}
			queued <- e
		}()
		waiting := false
		for !waiting && ctx.Err() == nil {
			if err = fx.Pool.QueryRow(ctx, `SELECT $1::int=ANY(pg_blocking_pids($2::int))`, resetPID, enqueue.Conn().PgConn().PID()).Scan(&waiting); err != nil {
				t.Fatal(err)
			}
			if !waiting {
				time.Sleep(time.Millisecond)
			}
		}
		if !waiting {
			t.Fatal("actual enqueue did not wait for reset commit")
		}

		close(release)
		select {
		case <-done:
		case <-ctx.Done():
			t.Fatal("reset did not finish")
		}
		select {
		case err = <-queued:
			if err != nil {
				t.Fatalf("enqueue failed after reset released: %v", err)
			}
		case <-ctx.Done():
			t.Fatal("enqueue did not finish after reset")
		}
		if updates.Load() != 1 {
			t.Fatalf("committed reset publication count=%d", updates.Load())
		}
		updated, err := s.Queries.GetIssue(ctx, issue.ID)
		if err != nil {
			t.Fatal(err)
		}
		if updated.Status != "todo" || fx.Count(t, `SELECT count(*) FROM iteration_event WHERE iteration_id=$1`, iterationID) != 1 {
			t.Fatal("reset did not commit atomic facts")
		}
	})
}
