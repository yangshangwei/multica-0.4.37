package main

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestPasswordRecoveryPreservesUserAndRequiresChange(t *testing.T) {
	if testPool == nil {
		t.Skip("database unavailable")
	}
	t.Setenv("MULTICA_AUTH_MODE", "password")
	fx := testutil.New(testPool, "", "")
	username := fmt.Sprintf("recover%d", time.Now().UnixNano())
	id := fx.User(t, "Recovery Test", username+"@example.com")
	fx.Cleanup(t, `DELETE FROM user_password_credential WHERE user_id=$1`, id)
	uid, err := util.ParseUUID(id)
	if err != nil {
		t.Fatal(err)
	}
	run := func(args []string) {
		t.Helper()
		reader, writer, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		if _, err = writer.WriteString("temporary correct horse password\n"); err != nil {
			t.Fatal(err)
		}
		writer.Close()
		previous := os.Stdin
		os.Stdin = reader
		err = runPasswordRecovery(args)
		os.Stdin = previous
		reader.Close()
		if err != nil {
			t.Fatal(err)
		}
	}
	run([]string{"--user", id, "--username", username})
	q := db.New(testPool)
	first, err := q.GetPasswordCredential(context.Background(), uid)
	if err != nil {
		t.Fatal(err)
	}
	if !first.MustChangePassword || first.SessionVersion != 1 || first.Username != username {
		t.Fatalf("invalid recovered account: version=%d must_change=%v", first.SessionVersion, first.MustChangePassword)
	}
	valid, err := auth.VerifyPassword(context.Background(), first.PasswordHash, "temporary correct horse password")
	if err != nil || !valid {
		t.Fatal("temporary password does not verify", err)
	}

	scoped := testutil.New(testPool, testWorkspaceID, id)
	memberID := scoped.Member(t, testWorkspaceID, id, "member")
	runtimeID := scoped.Runtime(t, "Recovered owner runtime", testutil.Cols{"runtime_mode": "local", "provider": "local"})
	agentID := scoped.Agent(t, "Recovered owner agent", runtimeID, testutil.Cols{"runtime_mode": "local"})
	taskID := scoped.Task(t, agentID, testutil.Cols{"runtime_id": runtimeID, "status": "running"})
	otherRuntime := scoped.Runtime(t, "Independent owner runtime", testutil.Cols{"owner_id": testUserID})
	patID := scoped.Insert(t, "personal_access_token", testutil.Cols{"user_id": id, "name": "recover PAT", "token_hash": "pat-" + username, "token_prefix": "mul_", "auth_version": int64(1)})
	scoped.Insert(t, "task_token", testutil.Cols{"user_id": id, "task_id": taskID, "agent_id": agentID, "workspace_id": testWorkspaceID, "token_hash": "task-" + username, "expires_at": time.Now().Add(time.Hour), "auth_version": int64(1)})
	scoped.Insert(t, "daemon_token", testutil.Cols{"user_id": id, "workspace_id": testWorkspaceID, "daemon_id": "recover-daemon", "token_hash": "daemon-" + username, "expires_at": time.Now().Add(time.Hour), "auth_version": int64(1)})
	run([]string{"--user", id})
	var revokedAndPreserved bool
	scoped.QueryRow(t, `SELECT EXISTS(SELECT 1 FROM personal_access_token WHERE id=$1 AND revoked) AND NOT EXISTS(SELECT 1 FROM task_token WHERE user_id=$2) AND NOT EXISTS(SELECT 1 FROM daemon_token WHERE user_id=$2) AND EXISTS(SELECT 1 FROM agent_task_queue WHERE id=$3 AND status='cancelled') AND EXISTS(SELECT 1 FROM agent_runtime WHERE id=$4 AND status='offline') AND EXISTS(SELECT 1 FROM agent_runtime WHERE id=$5 AND status='online') AND EXISTS(SELECT 1 FROM member WHERE id=$6 AND user_id=$2) AND EXISTS(SELECT 1 FROM agent WHERE id=$7 AND archived_at IS NULL)`, patID, id, taskID, runtimeID, otherRuntime, memberID, agentID).Scan(&revokedAndPreserved)
	if !revokedAndPreserved {
		t.Fatal("recovery did not revoke personal execution while preserving membership, agent and independent runtime")
	}
	second, err := q.GetPasswordCredential(context.Background(), uid)
	if err != nil {
		t.Fatal(err)
	}
	if second.SessionVersion != 2 || !second.MustChangePassword {
		t.Fatal("recovery did not increment version")
	}
	if _, err = q.GetUser(context.Background(), uid); err != nil {
		t.Fatal("recovery lost user", err)
	}
}

func TestPasswordRecoveryUsernameCollisionRollsBack(t *testing.T) {
	if testPool == nil {
		t.Skip("database unavailable")
	}
	t.Setenv("MULTICA_AUTH_MODE", "password")
	fx := testutil.New(testPool, "", "")
	username := fmt.Sprintf("collision%d", time.Now().UnixNano())
	existing := fx.User(t, "Existing password account", username+"@example.com")
	target := fx.User(t, "Legacy recovery target", "target-"+username+"@example.com")
	fx.InsertNoID(t, "user_password_credential", testutil.Cols{"user_id": existing, "username": username, "password_hash": "existing-hash-must-not-change"}, "user_id=$1", existing)
	// If recovery fails, neither the existing account nor the target's personal
	// credential may be revoked or replaced by a half-completed transaction.
	pat := fx.Insert(t, "personal_access_token", testutil.Cols{"user_id": target, "name": "preserved", "token_hash": username, "token_prefix": "mul_"})
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = writer.WriteString("temporary correct horse password\n"); err != nil {
		t.Fatal(err)
	}
	writer.Close()
	defer reader.Close()
	previous := os.Stdin
	os.Stdin = reader
	defer func() { os.Stdin = previous }()
	if err = runPasswordRecovery([]string{"--user", target, "--username", username}); err == nil {
		t.Fatal("recovery accepted another account's username")
	}
	var preserved bool
	fx.QueryRow(t, `SELECT EXISTS(SELECT 1 FROM "user" WHERE id=$1 AND name='Legacy recovery target') AND NOT EXISTS(SELECT 1 FROM user_password_credential WHERE user_id=$1) AND EXISTS(SELECT 1 FROM user_password_credential WHERE user_id=$2 AND username=$3 AND password_hash='existing-hash-must-not-change' AND session_version=1 AND NOT must_change_password) AND EXISTS(SELECT 1 FROM personal_access_token WHERE id=$4 AND NOT revoked)`, target, existing, username, pat).Scan(&preserved)
	if !preserved {
		t.Fatal("failed recovery changed account identity, credentials or PAT")
	}
}
