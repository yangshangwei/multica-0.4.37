package service

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"testing"
	"time"

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
