package service

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type ordinaryCollaborationCapacityFixture struct {
	fx             *testutil.Fixture
	q              *db.Queries
	svc            *TaskService
	agent, runtime pgtype.UUID
	task, daemon   string
}

func newOrdinaryCollaborationCapacityFixture(t *testing.T) ordinaryCollaborationCapacityFixture {
	t.Helper()
	if os.Getenv("DATABASE_URL") == "" {
		t.Skip("requires task-owned DATABASE_URL")
	}
	pool := newTaskClaimRacePool(t)
	fx := testutil.New(pool, "", "")
	suffix := uuid.NewString()
	fx.UserID = fx.User(t, "Shared capacity owner", "capacity-"+suffix+"@example.test")
	fx.WorkspaceID = fx.Workspace(t, "Shared capacity", "capacity-"+suffix)
	fx.Member(t, fx.WorkspaceID, fx.UserID, "owner")
	daemon := "capacity-" + suffix
	runtime := fx.Runtime(t, "Ordinary runtime", testutil.Cols{"daemon_id": daemon})
	agent := fx.Agent(t, "Ordinary agent", runtime, testutil.Cols{"max_concurrent_tasks": 5})
	issue := fx.Issue(t, "Ordinary task remains ordinary")
	task := fx.Task(t, agent, testutil.Cols{"runtime_id": runtime, "issue_id": issue})
	q := db.New(pool)
	return ordinaryCollaborationCapacityFixture{fx: fx, q: q, svc: NewTaskService(q, pool, nil, events.New()),
		agent: util.MustParseUUID(agent), runtime: util.MustParseUUID(runtime), task: task, daemon: daemon}
}

func (f ordinaryCollaborationCapacityFixture) reserve(t *testing.T, agent, daemon string) string {
	t.Helper()
	run := uuid.NewString()
	// Deliberately no matching run/runtime/grant row: retained physical
	// occupancy must not disappear when those resource rows are cleaned up.
	f.fx.InsertNoID(t, "collaboration_runtime_reservation", testutil.Cols{
		"run_id": run, "execution_workspace_id": f.fx.WorkspaceID,
		"agent_id": agent, "runtime_id": uuid.NewString(), "daemon_id": daemon,
		"installation_id": uuid.NewString(), "binding_id": uuid.NewString(),
	}, "run_id=$1", run)
	return run
}

func TestOrdinaryCapacityCountsRetainedCollaborationReservation(t *testing.T) {
	f := newOrdinaryCollaborationCapacityFixture(t)
	run := f.reserve(t, util.UUIDToString(f.agent), f.daemon)
	occupied, err := f.q.CountRunningTasks(t.Context(), f.agent)
	if err != nil || occupied != 1 {
		t.Fatalf("retained collaboration occupancy = %d, %v; want 1 even without live resource rows", occupied, err)
	}
	f.fx.Exec(t, `UPDATE collaboration_runtime_reservation SET released_at=now() WHERE run_id=$1`, run)
	occupied, err = f.q.CountRunningTasks(t.Context(), f.agent)
	if err != nil || occupied != 0 {
		t.Fatalf("released collaboration occupancy = %d, %v; want 0", occupied, err)
	}
}

func TestOrdinaryClaimsWaitForCollaborationNamespaceProof(t *testing.T) {
	for _, mode := range []string{"agent", "runtime", "batch", "sql"} {
		t.Run(mode, func(t *testing.T) {
			f := newOrdinaryCollaborationCapacityFixture(t)
			run := f.reserve(t, uuid.NewString(), f.daemon)
			claim := func() []db.AgentTaskQueue {
				t.Helper()
				var rows []db.AgentTaskQueue
				var task *db.AgentTaskQueue
				var err error
				switch mode {
				case "agent":
					task, err = f.svc.ClaimTask(t.Context(), f.agent)
				case "runtime":
					task, err = f.svc.ClaimTaskForRuntime(t.Context(), f.runtime)
				case "batch":
					rows, err = f.svc.ClaimTasksForRuntimes(t.Context(), []pgtype.UUID{f.runtime}, 1)
				case "sql":
					var row db.AgentTaskQueue
					row, err = f.q.ClaimAgentTask(t.Context(), db.ClaimAgentTaskParams{
						AgentID: f.agent, RuntimeID: f.runtime, PrepareLeaseSecs: 30, RuntimeStaleSecs: RuntimeClaimFreshnessSeconds})
					if err == nil {
						task = &row
					} else if errors.Is(err, pgx.ErrNoRows) {
						err = nil
					}
				}
				if err != nil {
					t.Fatalf("%s claim: %v", mode, err)
				}
				if task != nil {
					rows = append(rows, *task)
				}
				return rows
			}
			if rows := claim(); len(rows) != 0 {
				t.Fatalf("%s claimed ordinary work while another agent in its namespace has no termination proof", mode)
			}
			f.fx.Exec(t, `UPDATE collaboration_runtime_reservation SET released_at=now() WHERE run_id=$1`, run)
			rows := claim()
			if len(rows) != 1 || util.UUIDToString(rows[0].ID) != f.task {
				t.Fatalf("%s did not release the original queued task after proof: %+v", mode, rows)
			}
		})
	}
}

func TestOrdinaryClaimIgnoresAnotherCollaborationNamespace(t *testing.T) {
	f := newOrdinaryCollaborationCapacityFixture(t)
	f.reserve(t, uuid.NewString(), "different-"+f.daemon)
	task, err := f.svc.ClaimTask(t.Context(), f.agent)
	if err != nil || task == nil || util.UUIDToString(task.ID) != f.task {
		t.Fatalf("another namespace blocked ordinary work: %v, %v", task, err)
	}
}

func TestOrdinaryClaimRechecksReservationAfterNamespaceLock(t *testing.T) {
	f := newOrdinaryCollaborationCapacityFixture(t)
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	tx, err := f.fx.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	if err = f.q.WithTx(tx).LockInstallationNamespace(ctx, installationNamespace(util.MustParseUUID(f.fx.WorkspaceID), f.daemon)); err != nil {
		t.Fatal(err)
	}
	var blocker int
	if err = tx.QueryRow(ctx, "SELECT pg_backend_pid()").Scan(&blocker); err != nil {
		t.Fatal(err)
	}
	type result struct {
		task *db.AgentTaskQueue
		err  error
	}
	done := make(chan result, 1)
	go func() { task, err := f.svc.ClaimTask(ctx, f.agent); done <- result{task, err} }()
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	for {
		var blocked bool
		if err = f.fx.Pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pg_stat_activity WHERE $1::int=ANY(pg_blocking_pids(pid)))`, blocker).Scan(&blocked); err != nil {
			t.Fatal(err)
		}
		if blocked {
			break
		}
		select {
		case result := <-done:
			t.Fatalf("claim bypassed the namespace fence: %+v", result)
		case <-ctx.Done():
			t.Fatal("claim never waited for the namespace lock")
		case <-ticker.C:
		}
	}
	f.reserve(t, uuid.NewString(), f.daemon)
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case result := <-done:
		if result.err != nil || result.task != nil {
			t.Fatalf("claim did not recheck committed reservation after waiting: %+v", result)
		}
	case <-ctx.Done():
		t.Fatal("claim did not return after namespace release")
	}
}
