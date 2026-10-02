package handler

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/daemonws"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type connectionScopeQueryCounter struct {
	db.DBTX
	calls int
}

func (c *connectionScopeQueryCounter) Exec(ctx context.Context, sql string, args ...interface{}) (pgconn.CommandTag, error) {
	c.calls++
	return c.DBTX.Exec(ctx, sql, args...)
}
func (c *connectionScopeQueryCounter) Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error) {
	c.calls++
	return c.DBTX.Query(ctx, sql, args...)
}

func (c *connectionScopeQueryCounter) QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row {
	c.calls++
	return c.DBTX.QueryRow(ctx, sql, args...)
}

func managedConnectionScopeFixture(t *testing.T) (managedDaemonFixture, context.Context, daemonws.ClientIdentity) {
	t.Helper()
	f := managedDaemonSetup(t)
	ids := []string{f.runtime}
	for _, provider := range []string{"claude", "codex"} {
		ids = append(ids, dbfx.Runtime(t, "Additional managed runtime", testutil.Cols{"workspace_id": f.workspace, "owner_id": f.owner, "daemon_id": f.daemon, "provider": provider, "runtime_mode": "local"}))
	}
	source, err := auth.CheckPasswordToken(t.Context(), f.h.Queries, f.token, true)
	if err != nil {
		t.Fatal(err)
	}
	ctx := auth.WithPasswordSession(t.Context(), source.Session)
	return f, ctx, daemonws.ClientIdentity{UserID: f.owner, DaemonID: f.daemon, WorkspaceID: f.workspace, RuntimeIDs: ids}
}

func TestManagedConnectionScopeRechecksEveryCall(t *testing.T) {
	cases := []struct{ name, sql string }{
		{"runtime owner", "UPDATE agent_runtime SET owner_id=NULL WHERE id=$1"},
		{"runtime daemon", "UPDATE agent_runtime SET daemon_id='different-daemon' WHERE id=$1"},
		{"runtime workspace", "UPDATE agent_runtime SET workspace_id=$5 WHERE id=$1"},
		{"runtime deleted", "DELETE FROM agent_runtime WHERE id=$1"},
		{"binding revoked", "UPDATE installation_daemon_binding SET state='revoked' WHERE id=$2"},
		{"binding epoch", "UPDATE installation_daemon_binding SET binding_epoch=binding_epoch+1 WHERE id=$2"},
		{"binding principal", "UPDATE installation_daemon_binding SET principal_user_id='00000000-0000-4000-8000-000000000001' WHERE id=$2"},
		{"credential version", "UPDATE user_password_credential SET session_version=session_version+1 WHERE user_id=$3"},
		{"password reset", "UPDATE user_password_credential SET must_change_password=true WHERE user_id=$3"},
		{"account disabled", "UPDATE \"user\" SET disabled_at=now() WHERE id=$3"},
		{"membership removed", "DELETE FROM member WHERE user_id=$3 AND workspace_id=$4"},
		{"workspace organization", "UPDATE organization_workspace SET organization_id='00000000-0000-4000-8000-000000000001' WHERE workspace_id=$4"},
		{"installation retired", "UPDATE managed_installation SET lifecycle='retired' WHERE id=(SELECT installation_id FROM installation_daemon_binding WHERE id=$2)"},
		{"installation deployment", "UPDATE managed_installation SET deployment_id='00000000-0000-4000-8000-000000000001' WHERE id=(SELECT installation_id FROM installation_daemon_binding WHERE id=$2)"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f, ctx, identity := managedConnectionScopeFixture(t)
			if err := f.h.AuthorizeDaemonConnection(ctx, identity); err != nil {
				t.Fatal(err)
			}
			// Use a SELECT wrapper to keep parameter types explicit even when a mutation only uses one identity.
			otherWorkspace := dbfx.Workspace(t, "Other runtime scope", "scope-"+uuid.NewString())
			sql := "WITH identity AS (SELECT $1::uuid runtime_id,$2::uuid binding_id,$3::uuid owner_id,$4::uuid workspace_id,$5::uuid other_workspace_id) " + tc.sql
			dbfx.Exec(t, sql, identity.RuntimeIDs[2], f.binding, f.owner, f.workspace, otherWorkspace)
			if err := f.h.AuthorizeDaemonConnection(ctx, identity); err == nil {
				t.Fatal("connection retained authority after persisted scope changed")
			}
		})
	}
}

func TestManagedConnectionScopeQueryCount(t *testing.T) {
	f, ctx, identity := managedConnectionScopeFixture(t)
	counter := &connectionScopeQueryCounter{DBTX: testPool}
	f.h.Queries = db.New(counter)
	if err := f.h.AuthorizeDaemonConnection(ctx, identity); err != nil {
		t.Fatal(err)
	}
	if counter.calls != 11 {
		t.Fatalf("three-runtime connection authorization used %d queries, want 11", counter.calls)
	}
	t.Logf("three-runtime connection authorization query count: %d", counter.calls)
	counter.calls = 0
	if _, err := auth.CheckPasswordToken(ctx, f.h.Queries, f.token, true); err != nil {
		t.Fatal(err)
	}
	if err := f.h.AuthorizeDaemonConnection(ctx, identity); err != nil {
		t.Fatal(err)
	}
	if counter.calls != 19 {
		t.Fatalf("three-runtime token and connection authorization used %d queries, want 19", counter.calls)
	}
	t.Logf("three-runtime token and connection authorization query count: %d", counter.calls)
}

func TestManagedConnectionScopeRejectsOtherIdentity(t *testing.T) {
	f, ctx, identity := managedConnectionScopeFixture(t)
	identity.RuntimeIDs[2] = uuid.NewString()
	if err := f.h.AuthorizeDaemonConnection(ctx, identity); err == nil {
		t.Fatal("missing runtime accepted")
	}
	identity.RuntimeIDs = nil
	identity.WorkspaceIDs = []string{f.workspace, uuid.NewString()}
	if err := f.h.AuthorizeDaemonConnection(ctx, identity); err == nil {
		t.Fatal("extra workspace accepted")
	}
}
