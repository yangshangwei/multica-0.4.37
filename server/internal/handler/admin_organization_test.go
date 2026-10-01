package handler

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestAdminOrganizationAssignedWithWorkspaceCreation(t *testing.T) {
	passwordTestSetup(t)
	login := passwordRegister(t, fmt.Sprintf("orgowner%d", time.Now().UnixNano()))
	var ws WorkspaceResponse
	passwordCall(t, "POST", "/api/workspaces", map[string]string{"name": "Organization assignment", "slug": fmt.Sprintf("org-%d", time.Now().UnixNano())}, login.Token, testHandler.CreateWorkspace).Want(201).JSON(&ws)
	dbfx.Cleanup(t, `DELETE FROM workspace WHERE id=$1`, ws.ID)
	dbfx.Cleanup(t, `DELETE FROM member WHERE workspace_id=$1`, ws.ID)
	dbfx.Cleanup(t, `DELETE FROM issue_status WHERE workspace_id=$1`, ws.ID)
	dbfx.Cleanup(t, `DELETE FROM organization_workspace WHERE workspace_id=$1`, ws.ID)
	if dbfx.Count(t, `SELECT count(*) FROM organization_workspace ow JOIN organization o ON o.id=ow.organization_id WHERE ow.workspace_id=$1 AND o.internal AND o.state='active'`, ws.ID) != 1 {
		t.Fatal("new workspace lacks an internal organization")
	}
	dbfx.InsertNoID(t, "platform_role_binding", testutil.Cols{"user_id": login.User.ID, "role": "platform_observer"}, "user_id=$1", login.User.ID)
	org, err := testHandler.Queries.GetInternalOrganization(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	audit := dbfx.Insert(t, "admin_audit_event", testutil.Cols{"organization_id": org.ID, "actor_kind": "deployment_operator", "target_kind": "user", "target_id": login.User.ID, "action": "retention-test", "phase": "applied", "request_id": uuid.NewString(), "reason": "Workspace deletion retains platform history", "result_code": "applied"})
	router := chi.NewRouter()
	router.Delete("/api/workspaces/{id}", testHandler.DeleteWorkspace)
	req := testutil.JSONRequest("DELETE", "/api/workspaces/"+ws.ID, nil)
	req.Header.Set("X-User-ID", login.User.ID)
	testutil.Call(t, router.ServeHTTP, req).Want(204)
	if dbfx.Count(t, "SELECT count(*) FROM organization_workspace WHERE workspace_id=$1", ws.ID) != 0 {
		t.Fatal("deleted workspace retained organization mapping")
	}
	if dbfx.Count(t, "SELECT count(*) FROM platform_role_binding WHERE user_id=$1", login.User.ID) != 1 || dbfx.Count(t, "SELECT count(*) FROM admin_audit_event WHERE id=$1", audit) != 1 {
		t.Fatal("workspace deletion removed platform role or audit")
	}
}

func TestAdminWorkspaceOrganizationFailureRollsBackCreation(t *testing.T) {
	for _, mode := range []string{"inactive", "assignment failure"} {
		t.Run(mode, func(t *testing.T) {
			ctx := context.Background()
			schema := "admin_workspace_" + strings.ReplaceAll(uuid.NewString(), "-", "")
			quoted := pgx.Identifier{schema}.Sanitize()
			dbfx.Exec(t, "CREATE SCHEMA "+quoted)
			t.Cleanup(func() { _, _ = testPool.Exec(ctx, "DROP SCHEMA "+quoted+" CASCADE") })
			for _, table := range []string{"user", "workspace", "organization", "organization_workspace", "member", "issue_status"} {
				dbfx.Exec(t, "CREATE TABLE "+pgx.Identifier{schema, table}.Sanitize()+" (LIKE "+pgx.Identifier{"public", table}.Sanitize()+" INCLUDING ALL)")
			}
			cfg := testPool.Config().Copy()
			cfg.ConnConfig.RuntimeParams["search_path"] = schema + ",public"
			pool, err := pgxpool.NewWithConfig(ctx, cfg)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(pool.Close)
			fx := testutil.New(pool, "", "")
			id := fx.User(t, "Organization owner", "owner@example.invalid")
			state := "active"
			if mode == "inactive" {
				state = "inactive"
			}
			fx.Insert(t, "organization", testutil.Cols{"name": "Internal test", "state": state})
			if mode == "assignment failure" {
				fx.Exec(t, `CREATE FUNCTION fail_assignment() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected assignment failure'; END $$; CREATE TRIGGER reject_assignment BEFORE INSERT ON organization_workspace FOR EACH ROW EXECUTE FUNCTION fail_assignment()`)
			}
			h := *testHandler
			h.Queries = db.New(pool)
			h.TxStarter = pool
			req := testutil.JSONRequest("POST", "/api/workspaces", map[string]string{"name": "Should roll back", "slug": "rollback-org"})
			req.Header.Set("X-User-ID", id)
			testutil.Call(t, h.CreateWorkspace, req).Want(503)
			for _, table := range []string{"workspace", "member", "organization_workspace", "issue_status"} {
				if fx.Count(t, "SELECT count(*) FROM "+pgx.Identifier{table}.Sanitize()) != 0 {
					t.Fatalf("%s retained a partial creation", table)
				}
			}
		})
	}
}
