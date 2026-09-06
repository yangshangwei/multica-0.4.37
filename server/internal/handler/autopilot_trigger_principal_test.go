package handler

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/middleware"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// These tests pin the delegation chain behind agent-created triggers: a task
// token carries its runtime owner's user id, but the authorization principal a
// trigger records — the human every later dispatch acts as — is the task's
// originator. Requests go through the production Auth and workspace middleware
// so the actor headers (X-Actor-Source, X-Agent-ID, X-Task-ID) are set the way
// the real server sets them; no daemon or agent subprocess is involved.

func autopilotPrincipalRouter() http.Handler {
	r := chi.NewRouter()
	r.Post("/api/webhooks/autopilots/{token}", testHandler.HandleAutopilotWebhook)
	r.Group(func(r chi.Router) {
		r.Use(middleware.Auth(testHandler.Queries, nil, nil))
		r.Use(middleware.RequireWorkspaceMember(testHandler.Queries))
		r.Post("/api/autopilots", testHandler.CreateAutopilot)
		r.Post("/api/autopilots/{id}/triggers", testHandler.CreateAutopilotTrigger)
	})
	return r
}

func autopilotPrincipalRequest(method, path, token, workspaceID string, body any) *http.Request {
	return testutil.WithHeaders(testutil.JSONRequest(method, path, body),
		"Authorization", "Bearer "+token, "X-Workspace-ID", workspaceID)
}

// autopilotPrincipalWorkspace returns an isolated workspace owned by the suite
// user, so dispatch side effects (runs, queued tasks, webhook deliveries) never
// touch the shared handler-test workspace.
func autopilotPrincipalWorkspace(t *testing.T) *testutil.Fixture {
	t.Helper()
	fx := testutil.New(testPool, "", testUserID)
	fx.WorkspaceID = fx.Workspace(t, "Trigger principal test", fmt.Sprintf("trigger-principal-%d", time.Now().UnixNano()))
	fx.Member(t, fx.WorkspaceID, fx.UserID, "owner")
	return fx
}

// autopilotPrincipalTaskTokenCaller seeds a coordinator caller agent plus a
// running task and returns a task token bound to that task. The token's user is
// the runtime owner (userID), exactly as the production daemon binds it — which
// is a different human from originatorID, the task's authorizing user.
func autopilotPrincipalTaskTokenCaller(t *testing.T, fx *testutil.Fixture, runtimeID, originatorID string) (agentID string, token string) {
	t.Helper()
	agentID = fx.Agent(t, "Trigger principal coordinator", runtimeID, testutil.Cols{
		"owner_id": originatorID, "autonomy_level": "coordinator", "visibility": "workspace", "permission_mode": "public_to",
	})
	taskID := fx.Task(t, agentID, testutil.Cols{
		"runtime_id": runtimeID, "status": "running", "originator_user_id": originatorID,
		"accountable_user_id": originatorID, "originator_source": "direct_human",
	})
	var err error
	token, err = auth.GenerateAgentTaskToken()
	if err != nil {
		t.Fatal(err)
	}
	fx.Insert(t, "task_token", testutil.Cols{
		"token_hash": auth.HashToken(token), "task_id": taskID, "agent_id": agentID,
		"workspace_id": fx.WorkspaceID, "user_id": fx.UserID, "expires_at": time.Now().Add(time.Hour),
	})
	return agentID, token
}

// autopilotPrincipalPAT mints a personal access token for userID, the human
// creator control's credential.
func autopilotPrincipalPAT(t *testing.T, fx *testutil.Fixture, userID string) string {
	t.Helper()
	token, err := auth.GeneratePATToken()
	if err != nil {
		t.Fatal(err)
	}
	fx.Insert(t, "personal_access_token", testutil.Cols{
		"user_id": userID, "name": "trigger principal test", "token_hash": auth.HashToken(token), "token_prefix": token[:12],
	})
	return token
}

// autopilotPrincipalCleanupRuns removes every row a dispatch against apID can
// leave behind. The autopilot itself was created over HTTP, so it is not
// covered by the fixture's own cleanup.
func autopilotPrincipalCleanupRuns(t *testing.T, fx *testutil.Fixture, apID string) {
	t.Helper()
	fx.Cleanup(t, `DELETE FROM autopilot WHERE id = $1`, apID)
	fx.Cleanup(t, `DELETE FROM autopilot_trigger WHERE autopilot_id = $1`, apID)
	fx.Cleanup(t, `DELETE FROM autopilot_run WHERE autopilot_id = $1`, apID)
	fx.Cleanup(t, `DELETE FROM issue WHERE origin_type = 'autopilot' AND origin_id = $1`, apID)
	fx.Cleanup(t, `DELETE FROM agent_task_queue WHERE autopilot_run_id IN (SELECT id FROM autopilot_run WHERE autopilot_id = $1) OR issue_id IN (SELECT id FROM issue WHERE origin_type = 'autopilot' AND origin_id = $1)`, apID)
	fx.Cleanup(t, `DELETE FROM webhook_delivery WHERE autopilot_id = $1`, apID)
}

// autopilotPrincipalCreateTarget makes a run_only autopilot over targetID via
// the given token and registers the run cleanup.
func autopilotPrincipalCreateTarget(t *testing.T, router http.Handler, fx *testutil.Fixture, token, targetID, title string) AutopilotResponse {
	t.Helper()
	var ap AutopilotResponse
	testutil.Call(t, router.ServeHTTP, autopilotPrincipalRequest(http.MethodPost, "/api/autopilots", token, fx.WorkspaceID, map[string]any{
		"title": title, "execution_mode": "run_only", "assignee_type": "agent", "assignee_id": targetID,
	})).Want(http.StatusCreated).JSON(&ap)
	autopilotPrincipalCleanupRuns(t, fx, ap.ID)
	return ap
}

func autopilotPrincipalCreateTrigger(t *testing.T, router http.Handler, fx *testutil.Fixture, token, apID, kind string) AutopilotTriggerResponse {
	t.Helper()
	body := map[string]any{"kind": kind}
	if kind == "schedule" {
		body["cron_expression"] = "0 * * * *"
	}
	var trig AutopilotTriggerResponse
	testutil.Call(t, router.ServeHTTP, autopilotPrincipalRequest(http.MethodPost,
		"/api/autopilots/"+apID+"/triggers", token, fx.WorkspaceID, body)).Want(http.StatusCreated).JSON(&trig)
	return trig
}

// autopilotPrincipalDispatch fires the trigger the way production does — the
// scheduler for a schedule trigger, the durable webhook worker for a webhook —
// and returns the run row the firing produced.
func autopilotPrincipalDispatch(t *testing.T, router http.Handler, fx *testutil.Fixture, ap AutopilotResponse, trig AutopilotTriggerResponse) *db.AutopilotRun {
	t.Helper()
	ctx := context.Background()
	if trig.Kind == "schedule" {
		stored, err := testHandler.Queries.GetAutopilot(ctx, parseUUID(ap.ID))
		if err != nil {
			t.Fatal(err)
		}
		run, err := testHandler.AutopilotService.DispatchAutopilotForPlan(ctx, stored, parseUUID(trig.ID), "schedule", nil, time.Now().UTC())
		if err != nil {
			t.Fatalf("scheduled dispatch: %v", err)
		}
		return run
	}
	if trig.WebhookToken == nil {
		t.Fatal("created webhook trigger omitted its token")
	}
	out := testutil.Call(t, router.ServeHTTP, testutil.JSONRequest(http.MethodPost,
		"/api/webhooks/autopilots/"+*trig.WebhookToken, map[string]any{"event": "trigger.principal"})).Want(http.StatusOK).Map()
	runID, _ := out["run_id"].(string)
	if runID == "" {
		t.Fatalf("webhook ingress returned no run_id: %v", out)
	}
	worker := NewWebhookDeliveryWorker(testHandler)
	worked, err := worker.ProcessNext(ctx)
	if err != nil || !worked {
		t.Fatalf("durable webhook dispatch: worked=%v err=%v", worked, err)
	}
	run, err := testHandler.Queries.GetAutopilotRun(ctx, parseUUID(runID))
	if err != nil {
		t.Fatal(err)
	}
	return &run
}

// TestCreateAutopilotTrigger_TaskTokenRecordsOriginatorAsPrincipal pins the
// core finding: a trigger created by a task-token actor must record the task's
// originator — the human who authorized the task — as created_by, never the
// runtime owner whose user id the token carries. It then proves the value is
// load-bearing: automatic dispatch of that trigger enqueues nothing against a
// private target the originator cannot invoke, even though the runtime owner
// (the token's user) owns it.
func TestCreateAutopilotTrigger_TaskTokenRecordsOriginatorAsPrincipal(t *testing.T) {
	for _, kind := range []string{"schedule", "webhook"} {
		t.Run(kind, func(t *testing.T) {
			fx := autopilotPrincipalWorkspace(t)
			router := autopilotPrincipalRouter()
			originator := fx.User(t, "authorizing member", fmt.Sprintf("trigger-originator-%d@multica.test", time.Now().UnixNano()))
			fx.Member(t, fx.WorkspaceID, originator, "member")
			runtimeID := fx.Runtime(t, "shared runtime owned by the token user", testutil.Cols{"owner_id": fx.UserID, "visibility": "public"})
			// A private target the token's user owns but the originator
			// cannot invoke: only its owner may, and the originator is a
			// plain member.
			targetID := fx.Agent(t, "private target of the token user", runtimeID, testutil.Cols{"owner_id": fx.UserID})
			_, token := autopilotPrincipalTaskTokenCaller(t, fx, runtimeID, originator)

			ap := autopilotPrincipalCreateTarget(t, router, fx, token, targetID, "agent-created trigger over a target the originator cannot invoke")
			trig := autopilotPrincipalCreateTrigger(t, router, fx, token, ap.ID, kind)

			var creatorType, creatorID string
			fx.QueryRow(t, `SELECT created_by_type, created_by_id::text FROM autopilot_trigger WHERE id = $1`, trig.ID).Scan(&creatorType, &creatorID)
			if creatorType != "member" || creatorID != originator {
				t.Fatalf("agent-created %s trigger recorded created_by=%s/%s, want member/%s (the task's originator, not the token user %s)",
					kind, creatorType, creatorID, originator, fx.UserID)
			}

			run := autopilotPrincipalDispatch(t, router, fx, ap, trig)
			if tasks := fx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE agent_id = $1`, targetID); tasks != 0 {
				t.Fatalf("automatic %s dispatch of an agent-created trigger enqueued %d task(s) for a private target its originator cannot invoke (run status %s)",
					kind, tasks, run.Status)
			}
		})
	}
}

// TestCreateAutopilotTrigger_OriginatorOwnerTargetDispatchesOneTask is the
// positive counterpart: when the originator owns the private target, the
// agent-created trigger's dispatch is admitted as that human and enqueues
// exactly one task, attributed to them.
func TestCreateAutopilotTrigger_OriginatorOwnerTargetDispatchesOneTask(t *testing.T) {
	for _, kind := range []string{"schedule", "webhook"} {
		t.Run(kind, func(t *testing.T) {
			fx := autopilotPrincipalWorkspace(t)
			router := autopilotPrincipalRouter()
			originator := fx.User(t, "authorizing owner", fmt.Sprintf("trigger-owner-%d@multica.test", time.Now().UnixNano()))
			fx.Member(t, fx.WorkspaceID, originator, "member")
			runtimeID := fx.Runtime(t, "shared runtime owned by the token user", testutil.Cols{"owner_id": fx.UserID, "visibility": "public"})
			targetID := fx.Agent(t, "private target of the originator", runtimeID, testutil.Cols{"owner_id": originator})
			_, token := autopilotPrincipalTaskTokenCaller(t, fx, runtimeID, originator)

			ap := autopilotPrincipalCreateTarget(t, router, fx, token, targetID, "agent-created trigger over the originator's own target")
			trig := autopilotPrincipalCreateTrigger(t, router, fx, token, ap.ID, kind)

			var creatorID string
			fx.QueryRow(t, `SELECT created_by_id::text FROM autopilot_trigger WHERE id = $1`, trig.ID).Scan(&creatorID)
			if creatorID != originator {
				t.Fatalf("agent-created %s trigger recorded created_by=%s, want the originator %s", kind, creatorID, originator)
			}

			run := autopilotPrincipalDispatch(t, router, fx, ap, trig)
			if run.Status != "running" {
				t.Fatalf("originator-authorized %s dispatch status=%s, want running", kind, run.Status)
			}
			if tasks := fx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE autopilot_run_id = $1`, run.ID); tasks != 1 {
				t.Fatalf("originator-authorized %s dispatch enqueued %d task(s), want exactly 1", kind, tasks)
			}
			var taskOriginator string
			fx.QueryRow(t, `SELECT originator_user_id::text FROM agent_task_queue WHERE autopilot_run_id = $1`, run.ID).Scan(&taskOriginator)
			if taskOriginator != originator {
				t.Fatalf("dispatched task originator=%s, want the trigger's principal %s", taskOriginator, originator)
			}
		})
	}
}

// TestCreateAutopilotTrigger_TaskWithoutOriginatorIsRejected covers the chain
// with no human at its top (originator_user_id NULL): there is nobody the
// trigger's runs could act as, so creation must fail closed with 403 and leave
// no trigger row behind.
func TestCreateAutopilotTrigger_TaskWithoutOriginatorIsRejected(t *testing.T) {
	fx := autopilotPrincipalWorkspace(t)
	router := autopilotPrincipalRouter()
	runtimeID := fx.Runtime(t, "originless caller runtime", testutil.Cols{"owner_id": fx.UserID, "visibility": "public"})
	targetID := fx.Agent(t, "originless target", runtimeID, testutil.Cols{"owner_id": fx.UserID})
	agentID := fx.Agent(t, "originless coordinator", runtimeID, testutil.Cols{
		"owner_id": fx.UserID, "autonomy_level": "coordinator", "visibility": "workspace", "permission_mode": "public_to",
	})
	taskID := fx.Task(t, agentID, testutil.Cols{
		"runtime_id": runtimeID, "status": "running", "originator_user_id": nil, "accountable_user_id": fx.UserID,
	})
	token, err := auth.GenerateAgentTaskToken()
	if err != nil {
		t.Fatal(err)
	}
	fx.Insert(t, "task_token", testutil.Cols{
		"token_hash": auth.HashToken(token), "task_id": taskID, "agent_id": agentID,
		"workspace_id": fx.WorkspaceID, "user_id": fx.UserID, "expires_at": time.Now().Add(time.Hour),
	})

	ap := autopilotPrincipalCreateTarget(t, router, fx, token, targetID, "trigger from a task with no originator")
	testutil.Call(t, router.ServeHTTP, autopilotPrincipalRequest(http.MethodPost,
		"/api/autopilots/"+ap.ID+"/triggers", token, fx.WorkspaceID,
		map[string]any{"kind": "schedule", "cron_expression": "0 * * * *"})).Want(http.StatusForbidden)
	if n := fx.Count(t, `SELECT count(*) FROM autopilot_trigger WHERE autopilot_id = $1`, ap.ID); n != 0 {
		t.Fatalf("rejected originless task left %d trigger row(s) behind", n)
	}
}

// TestCreateAutopilotTrigger_MemberCreatorRecordsThemselves is the unchanged
// control: a human member creating a trigger still records themselves as the
// authorization principal, on both create paths.
func TestCreateAutopilotTrigger_MemberCreatorRecordsThemselves(t *testing.T) {
	for _, kind := range []string{"schedule", "webhook"} {
		t.Run(kind, func(t *testing.T) {
			fx := autopilotPrincipalWorkspace(t)
			router := autopilotPrincipalRouter()
			runtimeID := fx.Runtime(t, "member creator runtime")
			targetID := fx.Agent(t, "member creator target", runtimeID)
			token := autopilotPrincipalPAT(t, fx, fx.UserID)

			ap := autopilotPrincipalCreateTarget(t, router, fx, token, targetID, "human-created trigger control")
			trig := autopilotPrincipalCreateTrigger(t, router, fx, token, ap.ID, kind)

			var creatorType, creatorID string
			fx.QueryRow(t, `SELECT created_by_type, created_by_id::text FROM autopilot_trigger WHERE id = $1`, trig.ID).Scan(&creatorType, &creatorID)
			if creatorType != "member" || creatorID != fx.UserID {
				t.Fatalf("member-created %s trigger recorded created_by=%s/%s, want member/%s", kind, creatorType, creatorID, fx.UserID)
			}
		})
	}
}
