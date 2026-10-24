package service

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// Pause the first scan immediately before its cursor write, after runtime
// authorization. This reproduces deletion after the old unlocked read.
type runtimeScanEnsureBarrierStarter struct {
	TxStarter
	reached chan int
	release <-chan struct{}
	once    sync.Once
}

type runtimeScanEnsureBarrierTx struct {
	pgx.Tx
	starter *runtimeScanEnsureBarrierStarter
}

func (s *runtimeScanEnsureBarrierStarter) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &runtimeScanEnsureBarrierTx{Tx: tx, starter: s}, nil
}

func (tx *runtimeScanEnsureBarrierTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if strings.Contains(sql, "-- name: EnsureCollaborationRuntimeScanCursor ") {
		pause := false
		tx.starter.once.Do(func() { pause = true })
		if pause {
			tx.starter.reached <- int(tx.Conn().PgConn().PID())
			select {
			case <-tx.starter.release:
			case <-ctx.Done():
				return pgconn.CommandTag{}, ctx.Err()
			}
		}
	}
	return tx.Tx.Exec(ctx, sql, args...)
}

// Return whether the competing operation finished before reaching the actual
// database lock. An early result is retained so the test can still commit both
// sides and assert the persisted orphan count, rather than just timing out.
func runtimeScanWaitForBlockOrResult(t *testing.T, ctx context.Context, f *managedCollaborationFixture, blocker int, done <-chan error) (bool, error) {
	t.Helper()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		var blocked bool
		if err := f.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE $1::int=ANY(pg_blocking_pids(pid)))`, blocker).Scan(&blocked); err != nil {
			t.Fatal(err)
		}
		if blocked {
			return false, nil
		}
		select {
		case err := <-done:
			return true, err
		case <-ctx.Done():
			t.Fatal("competing operation neither completed nor reached the runtime fence")
		case <-ticker.C:
		}
	}
}

func deleteRuntimeScanResources(ctx context.Context, q *db.Queries, f *managedCollaborationFixture, scope string) error {
	if scope == "workspace" {
		if err := q.LockWorkspaceTaskOwnerRuntimes(ctx, f.runtime.WorkspaceID); err != nil {
			return err
		}
	}
	// The actual teardown locks the runtime before agents, retained execution
	// invalidation and the final runtime/workspace deletion statement.
	if _, err := TeardownRuntime(ctx, q, f.runtime.ID, RuntimeTeardownOptions{CancelNonTerminalTasks: true}); err != nil {
		return err
	}
	if scope == "workspace" {
		return q.DeleteWorkspaceRuntimesAndProjects(ctx, f.runtime.WorkspaceID)
	}
	return q.DeleteAgentRuntime(ctx, f.runtime.ID)
}

func TestCollaborationRuntimeScanSerializesWithDeletion(t *testing.T) {
	for _, scope := range []string{"runtime", "workspace"} {
		for _, first := range []string{"scan", "delete"} {
			t.Run(scope+"/"+first+"_first", func(t *testing.T) {
				f := newManagedCollaborationFixture(t)
				ctx, cancel := context.WithTimeout(f.daemonCtx, 10*time.Second)
				defer cancel()
				if first == "delete" {
					tx, err := f.pool.Begin(ctx)
					if err != nil {
						t.Fatal(err)
					}
					defer tx.Rollback(context.Background())
					if err = deleteRuntimeScanResources(ctx, f.svc.Queries.WithTx(tx), f, scope); err != nil {
						t.Fatal(err)
					}
					done := make(chan error, 1)
					go func() {
						_, err := f.svc.ListRuntimeRuns(ctx, f.runtime.ID)
						done <- err
					}()
					early, scanErr := runtimeScanWaitForBlockOrResult(t, ctx, f, int(tx.Conn().PgConn().PID()), done)
					if err = tx.Commit(ctx); err != nil {
						t.Fatal(err)
					}
					if !early {
						scanErr = <-done
					}
					orphans := f.fx.Count(t, `SELECT count(*) FROM collaboration_runtime_scan_cursor WHERE runtime_id=$1`, f.runtime.ID)
					if early || !collaborationErrorCode(scanErr, "runtime_not_found") || orphans != 0 {
						t.Fatalf("scan bypassed deletion or recreated its cursor: early=%t err=%v orphan_cursors=%d", early, scanErr, orphans)
					}
					return
				}

				release := make(chan struct{})
				var once sync.Once
				unblock := func() { once.Do(func() { close(release) }) }
				defer unblock()
				barrier := &runtimeScanEnsureBarrierStarter{TxStarter: f.pool, reached: make(chan int, 1), release: release}
				svc := *f.svc
				svc.TxStarter = barrier
				scanDone := make(chan error, 1)
				go func() {
					_, err := svc.ListRuntimeRuns(ctx, f.runtime.ID)
					scanDone <- err
				}()
				var blocker int
				select {
				case blocker = <-barrier.reached:
				case err := <-scanDone:
					t.Fatalf("scan failed before its cursor boundary: %v", err)
				case <-ctx.Done():
					t.Fatal("scan did not reach its cursor boundary")
				}
				deleteDone := make(chan error, 1)
				go func() {
					tx, err := f.pool.Begin(ctx)
					if err != nil {
						deleteDone <- err
						return
					}
					defer tx.Rollback(context.Background())
					if err = deleteRuntimeScanResources(ctx, f.svc.Queries.WithTx(tx), f, scope); err == nil {
						err = tx.Commit(ctx)
					}
					deleteDone <- err
				}()
				early, deleteErr := runtimeScanWaitForBlockOrResult(t, ctx, f, blocker, deleteDone)
				unblock()
				scanErr := <-scanDone
				if !early {
					deleteErr = <-deleteDone
				}
				if deleteErr != nil || scanErr != nil && !collaborationErrorCode(scanErr, "runtime_not_found") {
					t.Fatalf("serialized scan/deletion failed: scan=%v delete=%v", scanErr, deleteErr)
				}
				orphans := f.fx.Count(t, `SELECT count(*) FROM collaboration_runtime_scan_cursor WHERE runtime_id=$1`, f.runtime.ID)
				if early || orphans != 0 {
					t.Fatalf("deletion passed the paused authorized scan: early=%t orphan_cursors=%d", early, orphans)
				}
			})
		}
	}
}

func TestCollaborationRuntimeScanReauthorizesAfterRuntimeFence(t *testing.T) {
	f := newManagedCollaborationFixture(t)
	ctx, cancel := context.WithTimeout(f.daemonCtx, 10*time.Second)
	defer cancel()
	tx, err := f.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	if _, err = f.svc.Queries.WithTx(tx).LockAgentRuntime(ctx, f.runtime.ID); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := f.svc.ListRuntimeRuns(ctx, f.runtime.ID)
		done <- err
	}()
	early, scanErr := runtimeScanWaitForBlockOrResult(t, ctx, f, int(tx.Conn().PgConn().PID()), done)
	if _, err = tx.Exec(ctx, `UPDATE installation_daemon_binding SET binding_epoch=binding_epoch+1 WHERE id=$1`, f.binding.ID); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if !early {
		scanErr = <-done
	}
	if early || !collaborationErrorCode(scanErr, "managed_runtime_required") || f.fx.Count(t, `SELECT count(*) FROM collaboration_runtime_scan_cursor WHERE runtime_id=$1`, f.runtime.ID) != 0 {
		t.Fatalf("scan retained authority from before the runtime fence: early=%t err=%v", early, scanErr)
	}
}
