package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestCommentMutationActorVerifiedIdentity(t *testing.T) {
	userID, workspaceID, agentID, taskID := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	token := db.TaskToken{ID: parseUUID(uuid.NewString()), UserID: parseUUID(userID), WorkspaceID: parseUUID(workspaceID), AgentID: parseUUID(agentID), TaskID: parseUUID(taskID)}
	for _, scenario := range []struct {
		name           string
		kind           auth.CredentialKind
		verified       bool
		wrongUser      bool
		wrongWorkspace bool
		missingAgent   bool
		stampedTask    bool
		allowed        bool
		actorType      string
	}{
		{name: "human ignores agent hints", kind: auth.CredentialJWT, allowed: true, actorType: "member"},
		{name: "PAT ignores agent hints", kind: auth.CredentialPAT, allowed: true, actorType: "member"},
		{name: "cloud retains member identity", kind: auth.CredentialCloud, allowed: true, actorType: "member"},
		{name: "verified task retains agent", kind: auth.CredentialTask, verified: true, stampedTask: true, allowed: true, actorType: "agent"},
		{name: "task without verified binding", kind: auth.CredentialTask, stampedTask: true},
		{name: "stamped task without context", kind: auth.CredentialJWT, stampedTask: true},
		{name: "binding for other user", kind: auth.CredentialTask, verified: true, wrongUser: true},
		{name: "binding for other workspace", kind: auth.CredentialTask, verified: true, wrongWorkspace: true},
		{name: "binding lacks agent", kind: auth.CredentialTask, verified: true, missingAgent: true},
		{name: "task binding contradicts human credential", kind: auth.CredentialJWT, verified: true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPut, "/api/comments/local-fixture", nil)
			request.Header.Set("X-Agent-ID", uuid.NewString())
			request.Header.Set("X-Task-ID", uuid.NewString())
			if scenario.stampedTask {
				request.Header.Set("X-Actor-Source", "task_token")
			}
			ctx := auth.WithAuthenticatedPrincipal(context.Background(), auth.AuthenticatedPrincipal{UserID: userID, Kind: scenario.kind})
			if scenario.verified {
				binding := token
				if scenario.wrongUser {
					binding.UserID = parseUUID(uuid.NewString())
				}
				if scenario.wrongWorkspace {
					binding.WorkspaceID = parseUUID(uuid.NewString())
				}
				if scenario.missingAgent {
					binding.AgentID.Valid = false
				}
				ctx = auth.WithVerifiedTaskToken(ctx, binding)
			}
			actorType, actorID, allowed := commentMutationActor(request.WithContext(ctx), userID, workspaceID)
			if allowed != scenario.allowed || actorType != scenario.actorType {
				t.Fatalf("actor type=%q allowed=%v", actorType, allowed)
			}
			if allowed {
				wantID := userID
				if actorType == "agent" {
					wantID = agentID
				}
				if actorID != wantID {
					t.Fatal("mutation actor does not match verified credential")
				}
			}
		})
	}
}

// Auth stamps these sources in production. Direct calls prove that moving the
// routes cannot silently drop the credential gate inside either handler.
func TestCommentManagementMachineHandlerBackstop(t *testing.T) {
	for _, source := range []string{"task_token", "cloud_pat"} {
		for _, role := range []string{"owner", "admin"} {
			t.Run(source+"/"+role, func(t *testing.T) {
				ws := dbfx.Workspace(t, "Comment backstop", "comment-backstop-"+uuid.NewString())
				dbfx.Member(t, ws, testUserID, role)
				fx := testutil.New(testPool, ws, testUserID)
				other := fx.User(t, "Other author", uuid.NewString()+"@example.test")
				issue := fx.Issue(t, "Comment backstop")
				agent := fx.Agent(t, "Machine author", "")
				h := *testHandler
				h.Bus = events.New()
				h.Bus.SubscribeAll(func(events.Event) { t.Error("denied management published an event") })
				for _, operation := range []string{"body", "attachments", "delete"} {
					t.Run(operation, func(t *testing.T) {
						comment := fx.Comment(t, issue, "Original body", testutil.Cols{"author_id": other})
						method := http.MethodPut
						fn := h.UpdateComment
						body := map[string]any{"content": "Changed"}
						if operation == "attachments" {
							body["content"] = "Original body"
							body["attachment_ids"] = []string{}
						}
						if operation == "delete" {
							method = http.MethodDelete
							fn = h.DeleteComment
						}
						req := testutil.WithURLParams(testutil.JSONRequest(method, "/api/comments/"+comment, body), "commentId", comment)
						testutil.WithHeaders(req, "X-User-ID", testUserID, "X-Workspace-ID", ws, "X-Actor-Source", source, "X-Agent-ID", agent)
						var before string
						fx.QueryRow(t, `SELECT to_jsonb(c)::text FROM comment c WHERE id=$1`, comment).Scan(&before)
						testutil.Call(t, fn, req).Want(http.StatusForbidden)
						var after string
						fx.QueryRow(t, `SELECT to_jsonb(c)::text FROM comment c WHERE id=$1`, comment).Scan(&after)
						if before != after {
							t.Fatal("handler backstop changed comment")
						}
					})
				}
			})
		}
	}
}
