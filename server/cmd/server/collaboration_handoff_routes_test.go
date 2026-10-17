package main

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// AC25/AC26/AC29/AC30 over the production router: D asks P for information, P
// returns it to D, D submits the result to P. None of it completes the issue,
// touches the queue or starts ordinary AI.
func TestCollaborationHandoffRoutesReturnInformationAndSubmitAcceptance(t *testing.T) {
	if testPool == nil || testServer == nil {
		t.Fatal("isolated real-router DB fixture is required")
	}
	baseFX := testutil.New(testPool, testWorkspaceID, testUserID)
	source := baseFX.Workspace(t, "Handoff source", "handoff-router-"+uuid.NewString())
	baseFX.Member(t, source, testUserID, "owner")
	email := "handoff-router-" + uuid.NewString() + "@example.test"
	developer := baseFX.User(t, "External developer", email)
	developerToken, err := generateTestJWT(developer, email, "External developer")
	if err != nil {
		t.Fatal(err)
	}
	fx := testutil.New(testPool, source, testUserID)
	if err = db.New(testPool).SeedIssueStatusEntries(t.Context(), util.MustParseUUID(source)); err != nil {
		t.Fatal(err)
	}
	fx.Cleanup(t, `DELETE FROM issue_status WHERE workspace_id=$1`, source)
	issue := fx.Issue(t, "Handoff route fixture", testutil.Cols{"status": "in_progress", "assignee_type": "member", "assignee_id": developer})
	fx.InsertNoID(t, "issue_collaboration", testutil.Cols{"issue_id": issue}, "issue_id=$1", issue)
	session := fx.Insert(t, "collaboration_session", testutil.Cols{"issue_id": issue})
	for _, actor := range []string{testUserID, developer} {
		fx.InsertNoID(t, "issue_participant", testutil.Cols{"issue_id": issue, "user_id": actor}, "issue_id=$1 AND user_id=$2", issue, actor)
		fx.InsertNoID(t, "collaboration_account_fence", testutil.Cols{"user_id": actor}, "user_id=$1", actor)
	}
	seedCollaborationRouteCommand(t, fx, session, developer, "waiting work")
	fx.Cleanup(t, `DELETE FROM collaboration_operation WHERE scope_kind='issue' AND scope_id=$1`, issue)
	for _, table := range []string{"issue_handoff", "collaboration_event", "collaboration_notification"} {
		fx.Cleanup(t, "DELETE FROM "+table+" WHERE issue_id=$1", issue)
	}
	path := "/api/collaboration/issues/" + issue
	call := func(token, method, url, key string, body any) *testutil.Response {
		t.Helper()
		req := testutil.JSONRequest(method, url, body)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("X-Workspace-ID", "unrelated-ambient-space")
		if key != "" {
			req.Header.Set("Idempotency-Key", key)
		}
		return testutil.Call(t, testServer.Config.Handler.ServeHTTP, req)
	}
	detail := func(token string) map[string]any {
		t.Helper()
		return call(token, http.MethodGet, path, "", nil).Want(http.StatusOK).Map()
	}
	input := func(current map[string]any) map[string]any {
		write := current["write_context"].(map[string]any)
		return map[string]any{"expected_collaboration_revision": current["collaboration_revision"], "expected_lifecycle_epoch": current["lifecycle_epoch"],
			"expected_participant_epoch": write["participant_epoch"], "expected_account_epoch": write["account_epoch"]}
	}
	caps := func(current map[string]any) map[string]any { return current["capabilities"].(map[string]any) }
	confirmStatus := func(token, action string, body map[string]any) {
		t.Helper()
		options := call(token, http.MethodGet, path+"/status-options?action="+action, "", nil).Want(http.StatusOK).Map()
		body["expected_status"], body["status_options_version"] = options["default_status"], options["status_options_version"]
	}

	// Without a purpose the body has no business meaning and is refused.
	ask := input(detail(developerToken))
	ask["target_user_id"] = testUserID
	call(developerToken, http.MethodPost, path+"/handoffs", uuid.NewString(), ask).Want(http.StatusBadRequest)
	ask["purpose"] = "acceptance"
	call(developerToken, http.MethodPost, path+"/handoffs", uuid.NewString(), ask).Want(http.StatusBadRequest)
	ask["purpose"], ask["note"] = "information", "Which locale should win?"
	call(developerToken, http.MethodPost, path+"/handoffs", uuid.NewString(), ask).Want(http.StatusBadRequest)
	confirmStatus(developerToken, "request_information", ask)
	if r := call(developerToken, http.MethodPost, path+"/handoffs", uuid.NewString(), ask).Want(http.StatusOK).Map(); r["code"] != "information_requested" || len(r) != 2 {
		t.Fatalf("information request receipt: %v", r)
	}

	asked := detail(testToken)
	request, ok := asked["current_information_request"].(map[string]any)
	if !ok || asked["status"] != "blocked" || asked["handler"].(map[string]any)["user_id"] != testUserID ||
		request["requester"].(map[string]any)["user_id"] != developer || request["recipient"].(map[string]any)["user_id"] != testUserID ||
		request["return_status"] != "in_progress" || request["return_status_fallback"] != false || request["note"] != "Which locale should win?" {
		t.Fatalf("detail does not name the open request and its return: %v", asked)
	}
	if caps(asked)["return_information"] != true || caps(detail(developerToken))["return_information"] != false {
		t.Fatal("return capability does not follow the request recipient")
	}
	if current := call(testToken, http.MethodGet, "/api/me/workbench?view=current", "", nil).Want(http.StatusOK).Map(); current["total"] != float64(1) {
		t.Fatalf("requested account has no current work: %v", current)
	}

	// The client must send back the restore target it displayed.
	answer := input(asked)
	confirmStatus(testToken, "return_information", answer)
	answer["information_request_id"], answer["expected_status"] = request["id"], "todo"
	if r := call(testToken, http.MethodPost, path+"/information-returns", uuid.NewString(), answer).Want(http.StatusConflict).Map(); r["code"] != "status_options_changed" {
		t.Fatalf("silent status substitution: %v", r)
	}
	answer["information_request_id"] = uuid.NewString()
	call(testToken, http.MethodPost, path+"/information-returns", uuid.NewString(), answer).Want(http.StatusConflict)
	answer = input(detail(testToken))
	confirmStatus(testToken, "return_information", answer)
	answer["information_request_id"], answer["expected_status"] = request["id"], "in_progress"
	returnKey := uuid.NewString()
	if r := call(testToken, http.MethodPost, path+"/information-returns", returnKey, answer).Want(http.StatusOK).Map(); r["code"] != "information_returned" {
		t.Fatalf("information return: %v", r)
	}
	call(testToken, http.MethodPost, path+"/information-returns", returnKey, answer).Want(http.StatusOK)
	returned := detail(developerToken)
	if returned["status"] != "in_progress" || returned["handler"].(map[string]any)["user_id"] != developer || returned["current_information_request"] != nil {
		t.Fatalf("return did not restore the requester and saved status: %v", returned)
	}
	if r := call(testToken, http.MethodGet, path+"/operations/"+returnKey, "", nil).Want(http.StatusOK).Map(); r["code"] != "information_returned" {
		t.Fatalf("return receipt recovery: %v", r)
	}

	submit := input(returned)
	confirmStatus(developerToken, "submit_acceptance", submit)
	submit["note"] = "Ready for review"
	call(testToken, http.MethodPost, path+"/acceptance-submissions", uuid.NewString(), input(detail(testToken))).Want(http.StatusForbidden)
	if r := call(developerToken, http.MethodPost, path+"/acceptance-submissions", uuid.NewString(), submit).Want(http.StatusOK).Map(); r["code"] != "acceptance_submitted" {
		t.Fatalf("acceptance submission: %v", r)
	}
	accepted := detail(testToken)
	if accepted["status"] != "in_review" || accepted["handler"].(map[string]any)["user_id"] != testUserID ||
		accepted["session"].(map[string]any)["queue_state"] != "paused" || caps(accepted)["complete"] != true {
		t.Fatalf("acceptance did not hand the open issue to the creator: %v", accepted)
	}
	// Human participation survives the handoff. The v2 HTTP projection keeps
	// execution controls unavailable until their runtime protocol is delivered.
	if developerView := detail(developerToken); caps(developerView)["comment"] != true || caps(developerView)["control_ai"] != false || caps(developerView)["transfer"] != false {
		t.Fatalf("former handler collaboration rights: %v", developerView["capabilities"])
	}
	if fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE session_id=$1 AND state='waiting'`, session) != 1 ||
		fx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id=$1`, issue) != 0 ||
		fx.Count(t, `SELECT count(*) FROM member WHERE workspace_id=$1 AND user_id=$2`, source, developer) != 0 {
		t.Fatal("handoffs changed queued work, started ordinary AI or fabricated membership")
	}
	if fx.Count(t, `SELECT count(*) FROM collaboration_notification WHERE issue_id=$1 AND recipient_user_id=$2 AND kind='acceptance_submitted'`, issue, testUserID) != 1 ||
		fx.Count(t, `SELECT count(*) FROM collaboration_notification WHERE issue_id=$1 AND recipient_user_id=$2 AND kind='information_returned'`, issue, developer) != 1 {
		t.Fatal("handoff recipients were not notified")
	}
}
