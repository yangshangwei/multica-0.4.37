package handler

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestCreateSquadFromTemplate_ReuseAccessWithOneConnection(t *testing.T) {
	for _, tc := range []struct {
		name       string
		permission string
		targetType string
		granted    bool
		wantStatus int
	}{
		{"workspace_public", "public_to", "workspace", true, http.StatusCreated},
		{"member_allowlisted", "public_to", "member", true, http.StatusCreated},
		{"private", "private", "", false, http.StatusConflict},
		{"other_member_allowlisted", "public_to", "member", false, http.StatusConflict},
	} {
		t.Run(tc.name, func(t *testing.T) {
			callerID := dbfx.User(t, "Squad staffing member", "squad-staffing-"+tc.name+"@example.test")
			dbfx.Member(t, testWorkspaceID, callerID, "member")
			runtimeID := dbfx.Runtime(t, "Squad staffing runtime", testutil.Cols{
				"owner_id": callerID,
				"status":   "offline",
			})
			agentID := dbfx.Agent(t, "Existing Implementer", handlerTestRuntimeID(t), testutil.Cols{
				"template_key":    "implementer",
				"permission_mode": tc.permission,
			})
			if tc.targetType != "" {
				targetID := testWorkspaceID
				if tc.targetType == "member" {
					targetID = testUserID
					if tc.granted {
						targetID = callerID
					}
				}
				dbfx.InsertNoID(t, "agent_invocation_target", testutil.Cols{
					"agent_id": agentID, "target_type": tc.targetType, "target_id": targetID,
				}, "agent_id = $1", agentID)
			}

			cfg := testPool.Config().Copy()
			cfg.MaxConns = 1
			cfg.MinConns = 0
			pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(pool.Close)
			h := New(db.New(pool), pool, testHandler.Hub, testHandler.Bus, testHandler.EmailService,
				nil, nil, nil, Config{})

			agentsBefore := dbfx.Count(t, `SELECT COUNT(*) FROM agent WHERE workspace_id = $1`, testWorkspaceID)
			squadsBefore := dbfx.Count(t, `SELECT COUNT(*) FROM squad WHERE workspace_id = $1`, testWorkspaceID)
			skillsBefore := dbfx.Count(t, `SELECT COUNT(*) FROM skill WHERE workspace_id = $1`, testWorkspaceID)
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			req := testutil.WithHeaders(testutil.JSONRequest("POST", "/api/squads/from-template", map[string]any{
				"template_key": "bug-fix", "runtime_id": runtimeID,
			}), "X-User-ID", callerID).WithContext(ctx)
			req = testutil.WithURLParams(req, "workspaceId", testWorkspaceID)
			response := testutil.Call(t, h.CreateSquadFromTemplate, req).Want(tc.wantStatus)
			if ctx.Err() != nil {
				t.Fatalf("reuse authorization waited for a second pool connection: %v", ctx.Err())
			}
			if tc.wantStatus == http.StatusCreated {
				var out CreateSquadFromTemplateResponse
				response.JSON(&out)
				cleanupStaffedSquad(t, out.Squad.ID, out.CreatedAgents)
				if len(out.ReusedAgents) != 1 || out.ReusedAgents[0] != agentID {
					t.Fatalf("reused agents = %v, want [%s]", out.ReusedAgents, agentID)
				}
				return
			}

			// The Implementer is resolved after the leader and analyst were inserted;
			// a denied reuse must roll all of those earlier writes back.
			if got := dbfx.Count(t, `SELECT COUNT(*) FROM agent WHERE workspace_id = $1`, testWorkspaceID); got != agentsBefore {
				t.Errorf("agents after denied reuse = %d, want %d", got, agentsBefore)
			}
			if got := dbfx.Count(t, `SELECT COUNT(*) FROM squad WHERE workspace_id = $1`, testWorkspaceID); got != squadsBefore {
				t.Errorf("squads after denied reuse = %d, want %d", got, squadsBefore)
			}
			if got := dbfx.Count(t, `SELECT COUNT(*) FROM skill WHERE workspace_id = $1`, testWorkspaceID); got != skillsBefore {
				t.Errorf("skills after denied reuse = %d, want %d", got, skillsBefore)
			}
		})
	}
}
