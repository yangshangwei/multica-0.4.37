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
	"github.com/multica-ai/multica/server/internal/iteration"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func startIterationFixture(t *testing.T) (*testutil.Fixture, *TaskService, db.Issue, db.AgentTaskQueue, string) {
	t.Helper()
	fx, s, agent, runtime := triageBoundaryFixture(t)
	iid := fx.Insert(t, "iteration", testutil.Cols{"workspace_id": fx.WorkspaceID, "name": "Execution start", "timezone": "UTC", "start_date": "2026-10-01", "end_date": "2026-10-14", "status": "active", "created_by": fx.UserID})
	issueID := fx.Issue(t, "Unstarted commitment", testutil.Cols{"current_iteration_id": iid, "iteration_rollover_count": 2})
	fx.InsertNoID(t, "iteration_participation", testutil.Cols{"workspace_id": fx.WorkspaceID, "iteration_id": iid, "issue_id": issueID, "first_joined_at": testutil.Raw("clock_timestamp()"), "current_joined_at": testutil.Raw("clock_timestamp()"), "in_original": true, "original_facts": `{"title":"Original commitment"}`}, "iteration_id=$1 AND issue_id=$2", iid, issueID)
	fx.Cleanup(t, `DELETE FROM iteration_event WHERE iteration_id=$1`, iid)
	taskID := fx.Task(t, util.UUIDToString(agent), testutil.Cols{"runtime_id": runtime, "issue_id": issueID, "status": "dispatched"})
	issue, err := s.Queries.GetIssue(t.Context(), util.MustParseUUID(issueID))
	if err != nil {
		t.Fatal(err)
	}
	task, err := s.Queries.GetAgentTask(t.Context(), util.MustParseUUID(taskID))
	if err != nil {
		t.Fatal(err)
	}
	return fx, s, issue, task, iid
}

func assertStartIterationFacts(t *testing.T, fx *testutil.Fixture, iid string, wantStarted bool, wantEvents int) {
	t.Helper()
	var started bool
	var scope int64
	var original string
	fx.QueryRow(t, `SELECT has_started_current_participation,original_facts->>'title' FROM iteration_participation WHERE iteration_id=$1`, iid).Scan(&started, &original)
	fx.QueryRow(t, `SELECT scope_revision FROM iteration WHERE id=$1`, iid).Scan(&scope)
	count := fx.Count(t, `SELECT count(*) FROM iteration_event WHERE iteration_id=$1`, iid)
	if started != wantStarted || count != wantEvents || scope != int64(wantEvents+1) || original != "Original commitment" {
		t.Fatalf("start facts: started=%v events=%d scope=%d original=%q; want started=%v events=%d", started, count, scope, original, wantStarted, wantEvents)
	}
}

func TestStartTaskIterationFacts(t *testing.T) {
	for _, state := range []string{"active", "planned"} {
		t.Run(state, func(t *testing.T) {
			fx, s, issue, task, iid := startIterationFixture(t)
			fx.Exec(t, `UPDATE iteration SET status=$1 WHERE id=$2`, state, iid)
			started, err := s.StartTask(t.Context(), task.ID)
			if err != nil {
				t.Fatal(err)
			}
			if started.Status != "running" || !started.StartedAt.Valid {
				t.Fatalf("task did not start: %+v", started)
			}
			assertStartIterationFacts(t, fx, iid, true, 1)
			var kind string
			var before, after, actor map[string]any
			var rawBefore, rawAfter, rawActor []byte
			fx.QueryRow(t, `SELECT kind,before_facts,after_facts,actor FROM iteration_event WHERE iteration_id=$1`, iid).Scan(&kind, &rawBefore, &rawAfter, &rawActor)
			for _, pair := range []struct {
				raw []byte
				dst *map[string]any
			}{{rawBefore, &before}, {rawAfter, &after}, {rawActor, &actor}} {
				if err = json.Unmarshal(pair.raw, pair.dst); err != nil {
					t.Fatal(err)
				}
			}
			wantKind := "execution_started"
			if state == "planned" {
				wantKind = "planned_activity"
			}
			if kind != wantKind || before["has_started"] != false || after["has_started"] != true || actor["type"] != "system" || actor["source"] != "task_start" || actor["id"] != nil || actor["user_id"] != nil {
				t.Fatalf("unexpected start event: %s %s %s %s", kind, rawBefore, rawAfter, rawActor)
			}
			updated, err := s.Queries.GetIssue(t.Context(), issue.ID)
			if err != nil {
				t.Fatal(err)
			}
			if updated.Status != issue.Status || updated.Revision != issue.Revision || updated.CurrentIterationID != issue.CurrentIterationID || updated.IterationRolloverCount != issue.IterationRolloverCount {
				t.Fatal("execution start rewrote issue facts")
			}
			if _, err = s.StartTask(t.Context(), task.ID); !errors.Is(err, pgx.ErrNoRows) {
				t.Fatalf("repeated start changed its error contract: %v", err)
			}
			fx.Exec(t, `UPDATE agent_task_queue SET status='dispatched',started_at=NULL WHERE id=$1`, task.ID)
			if _, err = s.StartTask(t.Context(), task.ID); err != nil {
				t.Fatal(err)
			}
			assertStartIterationFacts(t, fx, iid, true, 1)
		})
	}
}

type startIterationStarter struct {
	inner          TxStarter
	beforeTaskLock chan<- struct{}
	beforeCommit   chan<- struct{}
	release        <-chan struct{}
	failEvent      bool
	pid            chan<- uint32
}

func (s startIterationStarter) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := s.inner.Begin(ctx)
	if err != nil {
		return nil, err
	}
	if s.pid != nil {
		s.pid <- tx.Conn().PgConn().PID()
	}
	return &startIterationTx{Tx: tx, owner: s}, nil
}

type startIterationTx struct {
	pgx.Tx
	owner startIterationStarter
}

func (tx *startIterationTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if strings.Contains(sql, "LockAgentTaskForStart") && tx.owner.beforeTaskLock != nil {
		tx.owner.beforeTaskLock <- struct{}{}
		select {
		case <-tx.owner.release:
		case <-ctx.Done():
		}
	}
	return tx.Tx.QueryRow(ctx, sql, args...)
}

func (tx *startIterationTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if tx.owner.failEvent && strings.Contains(sql, "AppendIterationIssueEvent") {
		return pgconn.CommandTag{}, errors.New("injected start event failure")
	}
	return tx.Tx.Exec(ctx, sql, args...)
}

func (tx *startIterationTx) Commit(ctx context.Context) error {
	if tx.owner.beforeCommit != nil {
		tx.owner.beforeCommit <- struct{}{}
		select {
		case <-tx.owner.release:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return tx.Tx.Commit(ctx)
}

func TestStartTaskIterationRollback(t *testing.T) {
	fx, s, _, task, iid := startIterationFixture(t)
	var published atomic.Int32
	s.Bus.Subscribe(protocol.EventTaskRunning, func(events.Event) { published.Add(1) })
	s.TxStarter = startIterationStarter{inner: s.TxStarter, failEvent: true}
	if _, err := s.StartTask(t.Context(), task.ID); err == nil {
		t.Fatal("start ignored recorder failure")
	}
	current, err := s.Queries.GetAgentTask(t.Context(), task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if current.Status != "dispatched" || current.StartedAt.Valid || published.Load() != 0 {
		t.Fatalf("failed start leaked task or publication: %s %v %d", current.Status, current.StartedAt, published.Load())
	}
	assertStartIterationFacts(t, fx, iid, false, 0)
}

func TestStartTaskIterationPostCommitPublication(t *testing.T) {
	fx, s, _, task, iid := startIterationFixture(t)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	reached, release := make(chan struct{}, 1), make(chan struct{})
	defer close(release)
	s.TxStarter = startIterationStarter{inner: s.TxStarter, beforeCommit: reached, release: release}
	var published atomic.Int32
	s.Bus.Subscribe(protocol.EventTaskRunning, func(events.Event) { published.Add(1) })
	done := make(chan error, 1)
	go func() { _, err := s.StartTask(ctx, task.ID); done <- err }()
	select {
	case <-reached:
	case <-ctx.Done():
		t.Fatal("start did not reach commit")
	}
	assertStartIterationFacts(t, fx, iid, false, 0)
	if published.Load() != 0 {
		t.Fatal("task running published before commit")
	}
	release <- struct{}{}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	assertStartIterationFacts(t, fx, iid, true, 1)
	if published.Load() != 1 {
		t.Fatal("task running was not published after commit")
	}
}

func TestStartTaskIterationWaitsForMembershipFence(t *testing.T) {
	for _, action := range []string{"join", "leave"} {
		t.Run(action, func(t *testing.T) {
			fx, s, issue, task, iid := startIterationFixture(t)
			if action == "join" {
				fx.Exec(t, `UPDATE issue SET current_iteration_id=NULL WHERE id=$1`, issue.ID)
				fx.Exec(t, `UPDATE iteration_participation SET current_joined_at=NULL WHERE iteration_id=$1`, iid)
			}
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			owner, err := fx.Pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer owner.Rollback(ctx)
			if err = iteration.LockWorkspace(ctx, owner, issue.WorkspaceID); err != nil {
				t.Fatal(err)
			}
			pid := make(chan uint32, 1)
			s.TxStarter = startIterationStarter{inner: s.TxStarter, pid: pid}
			done := make(chan error, 1)
			go func() { _, err := s.StartTask(ctx, task.ID); done <- err }()
			var startPID uint32
			select {
			case startPID = <-pid:
			case <-ctx.Done():
				t.Fatal("start did not open transaction")
			}
			blocked := false
			for !blocked && ctx.Err() == nil {
				if err = fx.Pool.QueryRow(ctx, `SELECT $1::int=ANY(pg_blocking_pids($2::int))`, owner.Conn().PgConn().PID(), startPID).Scan(&blocked); err != nil {
					t.Fatal(err)
				}
				if !blocked {
					time.Sleep(time.Millisecond)
				}
			}
			if !blocked {
				t.Fatal("start did not wait for membership fence")
			}
			if _, err = owner.Exec(ctx, `SET LOCAL lock_timeout='200ms'`); err != nil {
				t.Fatal(err)
			}
			if _, err = owner.Exec(ctx, `SELECT id FROM issue WHERE id=$1 FOR UPDATE NOWAIT`, issue.ID); err != nil {
				t.Fatalf("start acquired issue before workspace fence: %v", err)
			}
			if action == "join" {
				_, err = owner.Exec(ctx, `UPDATE issue SET current_iteration_id=$1 WHERE id=$2`, iid, issue.ID)
				if err == nil {
					_, err = owner.Exec(ctx, `UPDATE iteration_participation SET current_joined_at=clock_timestamp() WHERE iteration_id=$1`, iid)
				}
			} else {
				_, err = owner.Exec(ctx, `UPDATE issue SET current_iteration_id=NULL WHERE id=$1`, issue.ID)
				if err == nil {
					_, err = owner.Exec(ctx, `UPDATE iteration_participation SET current_joined_at=NULL WHERE iteration_id=$1`, iid)
				}
			}
			if err != nil {
				t.Fatal(err)
			}
			var releasedAt time.Time
			if err = owner.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&releasedAt); err != nil {
				t.Fatal(err)
			}
			if err = owner.Commit(ctx); err != nil {
				t.Fatal(err)
			}
			select {
			case err = <-done:
				if err != nil {
					t.Fatal(err)
				}
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
			want := 0
			if action == "join" {
				want = 1
			}
			assertStartIterationFacts(t, fx, iid, action == "join", want)
			started, err := s.Queries.GetAgentTask(ctx, task.ID)
			if err != nil {
				t.Fatal(err)
			}
			if !started.StartedAt.Valid || started.StartedAt.Time.Before(releasedAt) {
				t.Fatalf("task started_at predates membership fence release: %v < %v", started.StartedAt.Time, releasedAt)
			}
		})
	}
}

func TestStartTaskIterationUnassociatedTaskBecomesLinked(t *testing.T) {
	fx, s, issue, task, iid := startIterationFixture(t)
	fx.Exec(t, `UPDATE agent_task_queue SET issue_id=NULL WHERE id=$1`, task.ID)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	reached, release := make(chan struct{}, 1), make(chan struct{})
	defer close(release)
	s.TxStarter = startIterationStarter{inner: s.TxStarter, beforeTaskLock: reached, release: release}
	done := make(chan error, 1)
	go func() { _, err := s.StartTask(ctx, task.ID); done <- err }()
	select {
	case <-reached:
	case <-ctx.Done():
		t.Fatal("start did not reach guarded write")
	}
	if err := s.Queries.LinkTaskToIssue(ctx, db.LinkTaskToIssueParams{ID: task.ID, IssueID: issue.ID}); err != nil {
		t.Fatal(err)
	}
	release <- struct{}{}
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("changed ownership started outside its iteration fence")
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	current, err := s.Queries.GetAgentTask(ctx, task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if current.Status != "dispatched" {
		t.Fatalf("changed owner retained running transition: %s", current.Status)
	}
	assertStartIterationFacts(t, fx, iid, false, 0)
	s.TxStarter = fx.Pool
	if _, err = s.StartTask(ctx, task.ID); err != nil {
		t.Fatal(err)
	}
	assertStartIterationFacts(t, fx, iid, true, 1)
}

func TestStartTaskIterationNonStartingPhases(t *testing.T) {
	for _, phase := range []string{"queued", "running", "completed", "cancelled", "failed", "deferred"} {
		t.Run(phase, func(t *testing.T) {
			fx, s, _, task, iid := startIterationFixture(t)
			fx.Exec(t, `UPDATE agent_task_queue SET status=$1 WHERE id=$2`, phase, task.ID)
			if _, err := s.StartTask(t.Context(), task.ID); !errors.Is(err, pgx.ErrNoRows) {
				t.Fatalf("phase=%s start error=%v, want no transition", phase, err)
			}
			assertStartIterationFacts(t, fx, iid, false, 0)
		})
	}
}

func TestStartTaskIterationWaitingAndAlreadyStarted(t *testing.T) {
	for _, alreadyStarted := range []bool{false, true} {
		t.Run(map[bool]string{false: "unstarted", true: "already_started"}[alreadyStarted], func(t *testing.T) {
			fx, s, _, task, iid := startIterationFixture(t)
			fx.Exec(t, `UPDATE iteration_participation SET has_started_current_participation=$1 WHERE iteration_id=$2`, alreadyStarted, iid)
			fx.Exec(t, `UPDATE agent_task_queue SET status='waiting_local_directory',wait_reason='busy directory' WHERE id=$1`, task.ID)
			started, err := s.StartTask(t.Context(), task.ID)
			if err != nil {
				t.Fatal(err)
			}
			if started.Status != "running" || started.WaitReason.Valid {
				t.Fatalf("waiting task transition changed: %+v", started)
			}
			want := 1
			if alreadyStarted {
				want = 0
			}
			assertStartIterationFacts(t, fx, iid, true, want)
		})
	}
}

func TestStartTaskIterationIssueNullPreservesQuickCreate(t *testing.T) {
	fx, s, issue, task, iid := startIterationFixture(t)
	fx.Exec(t, `UPDATE agent_task_queue SET issue_id=NULL,context=jsonb_build_object('type','quick_create','workspace_id',$1::text,'parent_issue_id',$2::text) WHERE id=$3`, fx.WorkspaceID, util.UUIDToString(issue.ID), task.ID)
	// A deleted optional source is a snapshot reference, not task ownership.
	fx.Exec(t, `DELETE FROM issue WHERE id=$1`, issue.ID)
	started, err := s.StartTask(t.Context(), task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if started.Status != "running" || started.IssueID.Valid {
		t.Fatalf("deleted-source quick-create did not remain unassociated: %+v", started)
	}
	assertStartIterationFacts(t, fx, iid, false, 0)
}

func TestStartTaskIterationAdmissionRefusal(t *testing.T) {
	fx, s, issue, task, iid := startIterationFixture(t)
	fx.Exec(t, `UPDATE issue SET admission_status='pending' WHERE id=$1`, issue.ID)
	if _, err := s.StartTask(t.Context(), task.ID); err == nil {
		t.Fatal("pending issue was executed")
	}
	assertStartIterationFacts(t, fx, iid, false, 0)
}

func TestStartTaskIterationConcurrentRequests(t *testing.T) {
	fx, s, _, task, iid := startIterationFixture(t)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	begin := make(chan struct{})
	done := make(chan error, 2)
	for range 2 {
		go func() {
			<-begin
			_, err := s.StartTask(ctx, task.ID)
			done <- err
		}()
	}
	close(begin)
	successes := 0
	for range 2 {
		select {
		case err := <-done:
			if err == nil {
				successes++
			} else if !errors.Is(err, pgx.ErrNoRows) {
				t.Fatal(err)
			}
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
	}
	if successes != 1 {
		t.Fatalf("concurrent start successes=%d, want 1", successes)
	}
	assertStartIterationFacts(t, fx, iid, true, 1)
}

func TestStartTaskIterationNewParticipationAndEventClock(t *testing.T) {
	fx, s, _, task, iid := startIterationFixture(t)
	if _, err := s.StartTask(t.Context(), task.ID); err != nil {
		t.Fatal(err)
	}
	// Model a later participation after leave/reentry without rewriting original
	// commitment history. A previous future timestamp models clock correction.
	fx.Exec(t, `UPDATE iteration_participation SET current_joined_at=clock_timestamp(),has_started_current_participation=false WHERE iteration_id=$1`, iid)
	fx.Exec(t, `UPDATE iteration_event SET occurred_at=clock_timestamp()+interval '1 hour' WHERE iteration_id=$1`, iid)
	fx.Exec(t, `UPDATE agent_task_queue SET status='dispatched',started_at=NULL WHERE id=$1`, task.ID)
	if _, err := s.StartTask(t.Context(), task.ID); err != nil {
		t.Fatal(err)
	}
	assertStartIterationFacts(t, fx, iid, true, 2)
	var ordered, sampled bool
	fx.QueryRow(t, `SELECT b.sequence=a.sequence+1 AND b.occurred_at=a.occurred_at,b.sampled_at<b.occurred_at FROM iteration_event a JOIN iteration_event b ON b.iteration_id=a.iteration_id AND b.sequence=2 WHERE a.iteration_id=$1 AND a.sequence=1`, iid).Scan(&ordered, &sampled)
	if !ordered || !sampled {
		t.Fatal("execution start did not preserve event ordering through wall-clock correction")
	}
}
