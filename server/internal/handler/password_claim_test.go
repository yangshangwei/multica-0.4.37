package handler

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestPasswordBlockedTaskAndDaemonMintRejectsReset(t *testing.T) {
	passwordTestSetup(t)
	registered := passwordRegister(t, fmt.Sprintf("claimfence%d", time.Now().UnixNano()))
	fx := testutil.New(testPool, testWorkspaceID, registered.User.ID)
	runtime := fx.Runtime(t, "Password claim fence", testutil.Cols{"runtime_mode": "local", "provider": "local"})
	agent := fx.Agent(t, "Password claim fence", runtime, testutil.Cols{"runtime_mode": "local"})
	taskID := fx.Task(t, agent, testutil.Cols{"runtime_id": runtime, "status": "dispatched", "dispatched_at": time.Now()})
	fx.Cleanup(t, `DELETE FROM task_token WHERE task_id=$1`, taskID)
	fx.Cleanup(t, `DELETE FROM daemon_token WHERE user_id=$1`, registered.User.ID)
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	task, err := testHandler.Queries.GetAgentTask(ctx, parseUUID(taskID))
	if err != nil {
		t.Fatal(err)
	}
	uid := parseUUID(registered.User.ID)
	source := auth.WithPasswordSession(ctx, auth.PasswordSession{UserID: registered.User.ID, Kind: "jwt", Version: 1})
	token := db.CreateTaskTokenParams{TokenHash: "claim-before-" + taskID, TaskID: task.ID, AgentID: task.AgentID, WorkspaceID: parseUUID(testWorkspaceID), UserID: uid, ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(time.Hour), Valid: true}}
	daemon := db.CreateDaemonTokenParams{TokenHash: "daemon-before-" + taskID, WorkspaceID: parseUUID(testWorkspaceID), DaemonID: "password-claim-fence", ExpiresAt: token.ExpiresAt}
	// The same production finalizer first proves the valid source can mint both
	// credentials and that the daemon issuer is fixed to the runtime owner.
	if _, err = testHandler.TaskService.FinalizeTaskClaim(source, task, token, nil, false, daemon); err != nil {
		t.Fatal(err)
	}
	var versions bool
	fx.QueryRow(t, `SELECT EXISTS(SELECT 1 FROM task_token WHERE token_hash=$1 AND auth_version=1 AND user_id=$3) AND EXISTS(SELECT 1 FROM daemon_token WHERE token_hash=$2 AND auth_version=1 AND user_id=$3)`, token.TokenHash, daemon.TokenHash, registered.User.ID).Scan(&versions)
	if !versions {
		t.Fatal("valid finalizer lost credential source version or owner")
	}
	tx, err := testPool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	q := db.New(tx)
	if _, err = q.LockPasswordUser(ctx, uid); err != nil {
		t.Fatal(err)
	}
	var holder int32
	if err = tx.QueryRow(ctx, "SELECT pg_backend_pid()").Scan(&holder); err != nil {
		t.Fatal(err)
	}
	token.TokenHash = "claim-blocked-" + taskID
	daemon.TokenHash = "daemon-blocked-" + taskID
	finished := make(chan error, 1)
	go func() {
		_, err := testHandler.TaskService.FinalizeTaskClaim(source, task, token, nil, false, daemon)
		finished <- err
	}()
	for {
		var blocked bool
		if err = testPool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE $1=ANY(pg_blocking_pids(pid)))`, holder).Scan(&blocked); err != nil {
			t.Fatal(err)
		}
		if blocked {
			break
		}
		select {
		case err := <-finished:
			t.Fatalf("finalizer returned before row lock: %v", err)
		case <-ctx.Done():
			t.Fatal("finalizer never waited on source user")
		case <-time.After(5 * time.Millisecond):
		}
	}
	if _, err = q.ChangePasswordCredential(ctx, db.ChangePasswordCredentialParams{UserID: uid, PasswordHash: "reset-hash-not-used-for-login", SessionVersion: 1}); err != nil {
		t.Fatal(err)
	}
	if _, err = auth.RevokePasswordCredentials(ctx, tx, uid); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-finished:
		if !errors.Is(err, auth.ErrPasswordSession) {
			t.Fatalf("stale finalizer must reject original version: %v", err)
		}
	case <-ctx.Done():
		t.Fatal("finalizer did not finish after reset")
	}
	var revoked bool
	fx.QueryRow(t, `SELECT NOT EXISTS(SELECT 1 FROM task_token WHERE task_id=$1) AND NOT EXISTS(SELECT 1 FROM daemon_token WHERE user_id=$2) AND EXISTS(SELECT 1 FROM agent_task_queue WHERE id=$1 AND status='cancelled')`, taskID, registered.User.ID).Scan(&revoked)
	if !revoked {
		t.Fatal("reset retained old credentials, minted replacement credentials or replayed cancelled task")
	}
}
