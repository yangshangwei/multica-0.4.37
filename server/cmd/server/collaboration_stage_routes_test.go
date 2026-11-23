package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"net/http"
	"reflect"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// The service and handler suites own the stage matrices. This test proves the
// production router: real JWT authentication, the account route group, the
// project route group with its human-actor guard, actual policy activation and
// retained original-key recovery. The composed M1 policy-cycle test is separate.
func TestCollaborationStageRoutesActivateReadyPolicyAndPreserveOriginalKey(t *testing.T) {
	if testPool == nil || testServer == nil {
		t.Fatal("isolated real-router fixture required")
	}
	base := testutil.New(testPool, testWorkspaceID, testUserID)
	source := base.Workspace(t, "Stage source", "collaboration-stage-"+uuid.NewString())
	base.Member(t, source, testUserID, "owner")
	fx := testutil.New(testPool, source, testUserID)
	project := fx.Project(t, "Stage governed project")
	issue := fx.Issue(t, "Stage governed root", testutil.Cols{"assignee_type": "member", "assignee_id": testUserID, "project_id": project})
	fx.InsertNoID(t, "issue_collaboration", testutil.Cols{"issue_id": issue}, "issue_id=$1", issue)
	fx.Insert(t, "collaboration_session", testutil.Cols{"issue_id": issue})
	fx.InsertNoID(t, "issue_participant", testutil.Cols{"issue_id": issue, "user_id": testUserID}, "issue_id=$1 AND user_id=$2", issue, testUserID)
	ensureCollaborationRouteAccountFence(t, fx, testUserID)
	for _, table := range []string{"issue_participant", "collaboration_event", "collaboration_notification"} {
		fx.Cleanup(t, "DELETE FROM "+table+" WHERE issue_id=$1", issue)
	}
	fx.Cleanup(t, `DELETE FROM collaboration_operation WHERE scope_kind='issue' AND scope_id=$1`, issue)
	for _, table := range []string{"collaboration_stage_child", "collaboration_stage_decision", "collaboration_stage_candidate", "collaboration_stage_role", "collaboration_stage_task", "project_stage_policy"} {
		fx.Cleanup(t, `DELETE FROM `+table+` WHERE workspace_id=$1`, source)
	}
	call := func(token, method, path, key string, body any, headers ...string) *testutil.Response {
		t.Helper()
		req := testutil.JSONRequest(method, path, body)
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		req.Header.Set("X-Workspace-ID", source)
		if key != "" {
			req.Header.Set("Idempotency-Key", key)
		}
		testutil.WithHeaders(req, headers...)
		return testutil.Call(t, testServer.Config.Handler.ServeHTTP, req)
	}
	stage, policy := "/api/collaboration/issues/"+issue+"/stage", "/api/projects/"+project+"/stage-policy"
	rows := func() int {
		return fx.Count(t, `SELECT count(*) FROM project_stage_policy WHERE project_id=$1`, project)
	}
	observations := func() map[string]any {
		t.Helper()
		current := call(testToken, http.MethodGet, "/api/collaboration/issues/"+issue, "", nil).Want(http.StatusOK).Map()
		write := current["write_context"].(map[string]any)
		return map[string]any{"expected_collaboration_revision": current["collaboration_revision"], "expected_lifecycle_epoch": current["lifecycle_epoch"],
			"expected_participant_epoch": write["participant_epoch"], "expected_account_epoch": write["account_epoch"]}
	}
	with := func(fields map[string]any) map[string]any {
		body := observations()
		for key, value := range fields {
			body[key] = value
		}
		return body
	}

	call("", http.MethodGet, stage, "", nil).Want(http.StatusUnauthorized)
	view := call(testToken, http.MethodGet, stage, "", nil).Want(http.StatusOK).Map()
	if view["managed"] != false || view["issue_id"] != issue || view["policy"].(map[string]any)["state"] != "none" {
		t.Fatalf("unmanaged stage view through the router: %v", view)
	}
	// An agent-actor hint on a human token is not a human reader or writer.
	call(testToken, http.MethodGet, stage, "", nil, "X-Agent-ID", uuid.NewString()).WantOneOf(http.StatusForbidden, http.StatusNotFound)

	enable := map[string]any{"expected_policy_revision": "0", "enabled": true, "reason": "switch the stage gates on"}
	call("", http.MethodPut, policy, uuid.NewString(), enable).Want(http.StatusUnauthorized)
	call(testToken, http.MethodPut, policy, uuid.NewString(), enable, "X-Agent-ID", uuid.NewString(), "X-Task-ID", uuid.NewString()).Want(http.StatusForbidden)
	policyRead := call(testToken, http.MethodGet, policy, "", nil).Want(http.StatusOK).Map()
	if policyRead["policy_revision"] != "0" || policyRead["enabled"] != false || policyRead["contract"] != "collaboration-stage-v1" {
		t.Fatalf("virtual policy read through the router: %v", policyRead)
	}
	if rows() != 0 {
		t.Fatal("a refused or read request wrote a policy revision")
	}

	// Issue-level stage routes exist and refuse while the project is unmanaged.
	key := uuid.NewString()
	roles := map[string]any{"business_owner_user_id": testUserID, "implementer_user_id": testUserID, "product_approver_user_id": testUserID,
		"code_reviewer_user_id": uuid.NewString(), "qa_approver_user_id": testUserID, "release_approver_user_id": uuid.NewString()}
	refusedBody := with(map[string]any{"expected_policy_revision": "1", "expected_role_revision": "0", "roles": roles})
	refused := call(testToken, http.MethodPost, stage+"/roles", key, refusedBody).Want(http.StatusConflict).Map()
	if refused["code"] != "stage_policy_disabled" || refused["status"] != float64(http.StatusConflict) {
		t.Fatalf("stored refusal through the router: %v", refused)
	}
	if recovered := call(testToken, http.MethodGet, "/api/collaboration/issues/"+issue+"/operations/"+key, "", nil).Want(http.StatusOK).Map(); recovered["code"] != "stage_policy_disabled" || recovered["status"] != float64(http.StatusConflict) {
		t.Fatalf("original operation recovery through the router: %v", recovered)
	}
	proposal := uuid.NewString()
	for _, path := range []string{"/proposals", "/proposals/" + proposal + "/versions", "/proposals/" + proposal + "/decisions", "/scope-confirmations"} {
		if out := call(testToken, http.MethodPost, stage+path, uuid.NewString(), `{"unknown":true}`).Want(http.StatusBadRequest).Map(); out["code"] != "invalid_request" {
			t.Fatalf("stage route %s is not registered with the strict decoder: %v", path, out)
		}
	}
	call(testToken, http.MethodGet, stage+"/proposals", "", nil).Want(http.StatusOK)
	call(testToken, http.MethodGet, stage+"/proposals/"+proposal, "", nil).Want(http.StatusNotFound)

	q := db.New(testPool)
	executionReady, err := q.StageExecutionSchemaReady(t.Context())
	if err != nil || !executionReady.Valid || !executionReady.Bool {
		t.Fatalf("actual execution schema required: ready=%+v error=%T", executionReady, err)
	}
	submissionReady, err := q.CollaborationStageSubmissionReady(t.Context())
	if err != nil || !submissionReady.Valid || !submissionReady.Bool || fx.Count(t, `SELECT count(*) FROM schema_migrations WHERE version='864_collaboration_stage_submission_ready'`) != 1 || fx.Count(t, `SELECT count(*) FROM member WHERE workspace_id=$1 AND user_id=$2 AND role IN ('owner','admin')`, source, testUserID) != 1 {
		t.Fatalf("exact864/submission schema/current owner required: ready=%+v error=%T", submissionReady, err)
	}
	business := func() string {
		t.Helper()
		var snapshot string
		fx.QueryRow(t, `SELECT jsonb_build_object(
 'roles',(SELECT count(*) FROM collaboration_stage_role WHERE issue_id=$1),
 'candidates',(SELECT count(*) FROM collaboration_stage_candidate WHERE issue_id=$1),
 'decisions',(SELECT count(*) FROM collaboration_stage_decision WHERE issue_id=$1),
 'stage_tasks',(SELECT count(*) FROM collaboration_stage_task WHERE issue_id=$1),
 'scope_children',(SELECT count(*) FROM collaboration_stage_child WHERE root_issue_id=$1),
 'submissions',(SELECT count(*) FROM collaboration_stage_submission WHERE issue_id=$1),
 'children',(SELECT count(*) FROM issue WHERE parent_issue_id=$1),
 'tasks',(SELECT jsonb_agg(jsonb_build_object('id',id,'status',status) ORDER BY id) FROM agent_task_queue WHERE issue_id=$1 OR issue_id IN (SELECT id FROM issue WHERE parent_issue_id=$1)),
 'runs',(SELECT count(*) FROM collaboration_run WHERE session_id IN (SELECT id FROM collaboration_session WHERE issue_id=$1)),
 'events',(SELECT count(*) FROM collaboration_event WHERE issue_id=$1),
 'notifications',(SELECT count(*) FROM collaboration_notification WHERE issue_id=$1))::text`, issue).Scan(&snapshot)
		return snapshot
	}
	beforeEnable := business()
	enableKey := uuid.NewString()
	enableCall := call(testToken, http.MethodPut, policy, enableKey, enable)
	enabled := enableCall.Map()
	// Capture the actual authenticated PUT before checking the desired 200;
	// the initial false/503 RED must not stop at GET activation_available.
	t.Logf("P2 real-JWT healthy activation PUT status=%d response=%v policy_rows=%d execution_ready=%t submission_ready=%t", enableCall.Code, enabled, rows(), executionReady.Bool, submissionReady.Bool)
	enableCall.Want(http.StatusOK)
	confirmation, _ := enabled["stage_confirmation"].(map[string]any)
	if enabled["code"] != "stage_policy_revised" || confirmation["action"] != "revise_policy" || confirmation["project_id"] != project || confirmation["policy_revision"] != "1" || confirmation["enabled"] != true || len(confirmation) != 4 || rows() != 1 || business() != beforeEnable || policyRead["activation_available"] != true {
		t.Fatalf("actual router activation changed unrelated facts or confirmation: %v rows=%d", enabled, rows())
	}
	view = call(testToken, http.MethodGet, stage, "", nil).Want(http.StatusOK).Map()
	missing, _ := view["missing"].([]any)
	if view["managed"] != true || view["needs_confirmation"] != true || len(missing) != 2 || view["stage"].(map[string]any)["current"] != "development" {
		t.Fatalf("managed stage view through the router: %v", view)
	}
	created := call(testToken, http.MethodPost, stage+"/proposals", uuid.NewString(), with(map[string]any{"expected_policy_revision": "1",
		"content": map[string]any{"summary": "", "children": []any{map[string]any{"title": "Router child", "description": "", "priority": "none"}}},
		"source":  map[string]any{"kind": "manual"}})).Want(http.StatusOK).Map()
	if confirmation, _ := created["stage_confirmation"].(map[string]any); created["code"] != "stage_proposal_created" || confirmation["action"] != "create_proposal" ||
		fx.Count(t, `SELECT count(*) FROM issue WHERE parent_issue_id=$1`, issue) != 0 {
		t.Fatalf("proposal creation through the router: %v", created)
	}
	// The explicit rollback path stays available, with its own original-key read.
	beforeCycle := business()
	disableKey := uuid.NewString()
	disable := map[string]any{"expected_policy_revision": "1", "enabled": false, "reason": "explicit rollback"}
	disabled := call(testToken, http.MethodPut, policy, disableKey, disable).Want(http.StatusOK).Map()
	if confirmation, _ := disabled["stage_confirmation"].(map[string]any); disabled["code"] != "stage_policy_revised" || confirmation["action"] != "revise_policy" || confirmation["project_id"] != project || confirmation["policy_revision"] != "2" || confirmation["enabled"] != false || len(confirmation) != 4 {
		t.Fatalf("disable through the router: %v", disabled)
	}
	againKey := uuid.NewString()
	again := map[string]any{"expected_policy_revision": "2", "enabled": true, "reason": "explicit re-enable"}
	reenabled := call(testToken, http.MethodPut, policy, againKey, again).Want(http.StatusOK).Map()
	if confirmation, _ := reenabled["stage_confirmation"].(map[string]any); reenabled["code"] != "stage_policy_revised" || confirmation["action"] != "revise_policy" || confirmation["project_id"] != project || confirmation["policy_revision"] != "3" || confirmation["enabled"] != true || len(confirmation) != 4 || rows() != 3 || business() != beforeCycle {
		t.Fatalf("re-enable through the router changed facts: %v rows=%d", reenabled, rows())
	}
	for _, original := range []struct {
		key     string
		body    map[string]any
		receipt map[string]any
	}{{enableKey, enable, enabled}, {disableKey, disable, disabled}, {againKey, again, reenabled}} {
		recovered := call(testToken, http.MethodGet, policy+"/operations/"+original.key, "", nil).Want(http.StatusOK).Map()
		replayed := call(testToken, http.MethodPut, policy, original.key, original.body).Want(http.StatusOK).Map()
		if !reflect.DeepEqual(recovered, original.receipt) || !reflect.DeepEqual(replayed, original.receipt) || rows() != 3 || business() != beforeCycle {
			t.Fatalf("router original key adopted new state/effects: recovered=%v replayed=%v", recovered, replayed)
		}
	}
	changed := map[string]any{"expected_policy_revision": "0", "enabled": true, "reason": "changed original intent"}
	if out := call(testToken, http.MethodPut, policy, enableKey, changed).Want(http.StatusConflict).Map(); out["code"] != "idempotency_conflict" || rows() != 3 {
		t.Fatalf("router accepted a changed original body: %v", out)
	}
	if out := call(testToken, http.MethodPut, policy, uuid.NewString(), disable).Want(http.StatusConflict).Map(); out["code"] != "stage_policy_revision_conflict" || rows() != 3 {
		t.Fatalf("router accepted stale policy CAS: %v", out)
	}
	unknownKey := uuid.NewString()
	if out := call(testToken, http.MethodGet, policy+"/operations/"+unknownKey, "", nil).Want(http.StatusNotFound).Map(); out["code"] != "stage_policy_operation_not_found" {
		t.Fatalf("router inferred an unknown policy key: %v", out)
	}
	for _, result := range []map[string]any{
		call(testToken, http.MethodGet, "/api/collaboration/issues/"+issue+"/operations/"+key, "", nil).Want(http.StatusOK).Map(),
		call(testToken, http.MethodPost, stage+"/roles", key, refusedBody).Want(http.StatusConflict).Map(),
	} {
		if result["code"] != refused["code"] || result["status"] != refused["status"] || business() != beforeCycle || rows() != 3 {
			t.Fatalf("policy cycle revived an original workflow refusal: %v", result)
		}
	}
}

// This is one real-JWT/router composition: a legacy stage-off completion
// consumes its actual enforce grant before the first public stage activation.
// It does not claim an initial stage-enabled completion or external deployment.
func TestCollaborationStagePolicyActivationPreservesConsumedCompletionHistory(t *testing.T) {
	f := newCompletionCollaborationRouterFixture(t, "self")
	f.fx.Cleanup(t, `DELETE FROM project_stage_policy WHERE project_id=$1`, f.project)
	f.fx.Cleanup(t, `DELETE FROM collaboration_stage_task WHERE issue_id=$1`, f.parent)
	policyPath := "/api/projects/" + f.project + "/stage-policy"
	policy := f.call(t, f.token, http.MethodGet, policyPath, nil).Want(http.StatusOK).Map()
	if policy["policy_revision"] != "0" || policy["enabled"] != false || f.fx.Count(t, `SELECT count(*) FROM project_stage_policy WHERE project_id=$1`, f.project) != 0 {
		t.Fatalf("legacy completion must start with virtual stage-off: %v", policy)
	}
	if f.initiator == f.user || f.fx.Count(t, `SELECT count(*) FROM member WHERE workspace_id=$1 AND user_id=$2`, testWorkspaceID, f.initiator) != 0 {
		t.Fatal("completion consumer must remain distinct from the source policy issuer")
	}
	req, body := f.authorization(t, "self")
	grant := f.grant(t, req)
	unusedReq, _ := f.authorization(t, "self")
	unusedGrant := f.grant(t, unusedReq)
	if grant.AuthorizationID == unusedGrant.AuthorizationID || req.OperationID == unusedReq.OperationID || grant.State != "issued" || unusedGrant.State != "issued" {
		t.Fatal("actual independent consumed/pending enforce grants were not issued")
	}
	body["completion"] = service.DeliveryCompletionBinding{AuthorizationID: grant.AuthorizationID, OperationID: req.OperationID, ExpectedIssueRevision: req.Target.ExpectedIssueRevision}
	originalRaw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	originalBodyHash := sha256.Sum256(originalRaw)
	archive := f.archive(t)
	completed := f.call(t, f.initiatorToken, http.MethodPost, f.pathFor("/lifecycle"), originalRaw, "Idempotency-Key", req.OperationID).Want(http.StatusOK).Map()
	originalWorkflowGet := f.call(t, f.initiatorToken, http.MethodGet, f.pathFor("/operations/"+req.OperationID), nil).Want(http.StatusOK)
	originalWorkflow := originalWorkflowGet.Map()
	t.Logf("K06 actual public workflow GET status=%d response=%s", originalWorkflowGet.Code, originalWorkflowGet.Text())
	if originalWorkflowGet.Header().Get("Cache-Control") != "no-store" || originalWorkflow["status"] != nil || originalWorkflow["code"] != "completed" || originalWorkflow["code"] != completed["code"] || originalWorkflow["collaboration_revision"] != completed["collaboration_revision"] || !reflect.DeepEqual(originalWorkflow["delivery_confirmation"], completed["delivery_confirmation"]) {
		t.Fatalf("actual minimal workflow GET differs from its completed result: %v", originalWorkflow)
	}
	var originalAuthorization service.DeliveryCompletionAuthorizationView
	f.call(t, f.initiatorToken, http.MethodGet, completionAuthorizationRouterPath+"/"+grant.AuthorizationID, nil).Want(http.StatusOK).JSON(&originalAuthorization)
	var beforeRevision, completedRevision int64
	var operationID, actorID, collaborationOperationID, submissionID string
	f.fx.QueryRow(t, `SELECT issue_revision_before,issue_revision_after,operation_id::text,actor_user_id::text,
 COALESCE(collaboration_operation_id::text,''),COALESCE(result_submission_id::text,'')
 FROM delivery_completion_terminal WHERE authorization_id=$1 AND kind='consumed'`, grant.AuthorizationID).Scan(&beforeRevision, &completedRevision, &operationID, &actorID, &collaborationOperationID, &submissionID)
	if beforeRevision != req.Target.ExpectedIssueRevision || completedRevision <= beforeRevision || operationID != req.OperationID || actorID != f.initiator || collaborationOperationID == "" || submissionID == "" || originalAuthorization.State != "consumed" || originalAuthorization.CompletionReceipt == nil || originalAuthorization.CompletionReceipt.AuthorizationID != grant.AuthorizationID || originalAuthorization.CompletionReceipt.OperationID != req.OperationID || originalAuthorization.CompletionReceipt.IssueID != f.parent || originalAuthorization.CompletionReceipt.CompletedRevision != completedRevision || originalAuthorization.CompletionReceipt.TargetStatus != "done" {
		t.Fatal("actual completed result, issuer/consumer and consumed receipt identities differ")
	}
	originalConfirmation, _ := originalWorkflow["delivery_confirmation"].(map[string]any)
	if originalConfirmation["submission_id"] != submissionID {
		t.Fatal("actual public workflow GET names a different result submission")
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_operation WHERE id=$1 AND scope_id=$2 AND request_id=$3`, collaborationOperationID, f.parent, req.OperationID) != 1 || f.fx.Count(t, `SELECT count(*) FROM delivery_completion_terminal WHERE authorization_id=$1`, grant.AuthorizationID) != 1 || f.fx.Count(t, `SELECT count(*) FROM delivery_completion_terminal WHERE authorization_id=$1`, unusedGrant.AuthorizationID) != 0 || f.fx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND status='done'`, f.parent) != 1 {
		t.Fatal("legacy actual completion did not consume exactly its original grant")
	}
	t.Logf("K06 legacy consumed operation=%s authorization=%s issue_before=%d completed=%d original_body_sha256=%x", req.OperationID, grant.AuthorizationID, beforeRevision, completedRevision, originalBodyHash)
	for _, step := range []struct {
		expected string
		revision string
		enabled  bool
	}{
		{"0", "1", true},
		{"1", "2", false},
		{"2", "3", true},
	} {
		key := uuid.NewString()
		request := map[string]any{"expected_policy_revision": step.expected, "enabled": step.enabled, "reason": "explicit policy cycle after consumed legacy completion"}
		raw, encodeErr := json.Marshal(request)
		if encodeErr != nil {
			t.Fatal(encodeErr)
		}
		response := f.call(t, f.token, http.MethodPut, policyPath, raw, "Idempotency-Key", key)
		out := response.Map()
		t.Logf("K06 actual source-issuer stage policy PUT status=%d expected=%s enabled=%t response=%v", response.Code, step.expected, step.enabled, out)
		response.Want(http.StatusOK)
		confirmation, _ := out["stage_confirmation"].(map[string]any)
		if out["code"] != "stage_policy_revised" || confirmation["action"] != "revise_policy" || confirmation["project_id"] != f.project || confirmation["policy_revision"] != step.revision || confirmation["enabled"] != step.enabled || len(confirmation) != 4 {
			t.Fatalf("actual P2 policy cycle confirmation: %v", out)
		}
	}
	if f.fx.Count(t, `SELECT count(*) FROM member WHERE workspace_id=$1 AND user_id=$2`, testWorkspaceID, f.initiator) != 0 || f.fx.Count(t, `SELECT count(*) FROM project_stage_policy WHERE project_id=$1`, f.project) != 3 {
		t.Fatal("policy cycle changed consumer authority or its own revision count")
	}
	reopenKey := uuid.NewString()
	reopenBody := f.workflowBody(t, f.initiatorToken, "reopen")
	var beforeReopenLifecycle int64
	f.fx.QueryRow(t, `SELECT lifecycle_epoch FROM issue_collaboration WHERE issue_id=$1`, f.parent).Scan(&beforeReopenLifecycle)
	beforeReopenRuns := f.fx.Count(t, `SELECT count(*) FROM collaboration_run r JOIN collaboration_session s ON s.id=r.session_id WHERE s.issue_id=$1`, f.parent)
	beforeReopenCommands := f.fx.Count(t, `SELECT count(*) FROM collaboration_command c JOIN collaboration_session s ON s.id=c.session_id WHERE s.issue_id=$1`, f.parent)
	beforeReopenTasks := f.fx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id=$1`, f.parent)
	beforeReopenConsumed := f.fx.Count(t, `SELECT count(*) FROM delivery_completion_terminal WHERE authorization_id IN ($1,$2) AND kind='consumed'`, grant.AuthorizationID, unusedGrant.AuthorizationID)
	f.call(t, f.initiatorToken, http.MethodPost, f.pathFor("/lifecycle"), reopenBody, "Idempotency-Key", reopenKey).Want(http.StatusOK)
	view := f.call(t, f.initiatorToken, http.MethodGet, f.pathFor("/stage"), nil).Want(http.StatusOK).Map()
	stage, _ := view["stage"].(map[string]any)
	if view["managed"] != true || view["needs_confirmation"] != true || stage["current"] != "development" || f.fx.Count(t, `SELECT count(*) FROM collaboration_stage_role WHERE issue_id=$1`, f.parent) != 0 || f.fx.Count(t, `SELECT count(*) FROM collaboration_stage_decision WHERE issue_id=$1`, f.parent) != 0 || f.fx.Count(t, `SELECT count(*) FROM collaboration_stage_task WHERE issue_id=$1 AND (scope_revision<>0 OR scope_decision_id IS NOT NULL)`, f.parent) != 0 {
		t.Fatalf("activation/reopen inferred a role, scope or approval: %v", view)
	}
	var reopenedLifecycle int64
	var queueState, pauseReason string
	f.fx.QueryRow(t, `SELECT c.lifecycle_epoch,s.queue_state,COALESCE(s.pause_reason,'') FROM issue_collaboration c JOIN collaboration_session s ON s.issue_id=c.issue_id WHERE c.issue_id=$1`, f.parent).Scan(&reopenedLifecycle, &queueState, &pauseReason)
	reopenedRuns := f.fx.Count(t, `SELECT count(*) FROM collaboration_run r JOIN collaboration_session s ON s.id=r.session_id WHERE s.issue_id=$1`, f.parent)
	reopenedCommands := f.fx.Count(t, `SELECT count(*) FROM collaboration_command c JOIN collaboration_session s ON s.id=c.session_id WHERE s.issue_id=$1`, f.parent)
	reopenedTasks := f.fx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id=$1`, f.parent)
	reopenedConsumed := f.fx.Count(t, `SELECT count(*) FROM delivery_completion_terminal WHERE authorization_id IN ($1,$2) AND kind='consumed'`, grant.AuthorizationID, unusedGrant.AuthorizationID)
	t.Logf("K06 actual reopen lifecycle_before=%d lifecycle_after=%d queue=%s pause_reason=%s runs_before=%d runs_after=%d commands_before=%d commands_after=%d tasks_before=%d tasks_after=%d consumed_before=%d consumed_after=%d", beforeReopenLifecycle, reopenedLifecycle, queueState, pauseReason, beforeReopenRuns, reopenedRuns, beforeReopenCommands, reopenedCommands, beforeReopenTasks, reopenedTasks, beforeReopenConsumed, reopenedConsumed)
	if f.fx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND status='done'`, f.parent) != 0 || reopenedLifecycle != beforeReopenLifecycle+1 || queueState != "paused" || pauseReason != "issue_reopened" || reopenedRuns != beforeReopenRuns || reopenedCommands != beforeReopenCommands || reopenedTasks != beforeReopenTasks || reopenedConsumed != beforeReopenConsumed {
		t.Fatal("actual reopen did not preserve its paused new lifecycle and existing execution facts")
	}
	baseline := collaborationCompletionReadRows(t, f)
	workflowGet := f.call(t, f.initiatorToken, http.MethodGet, f.pathFor("/operations/"+req.OperationID), nil).Want(http.StatusOK)
	recoveredWorkflow := workflowGet.Map()
	if !reflect.DeepEqual(recoveredWorkflow, originalWorkflow) || collaborationCompletionReadRows(t, f) != baseline {
		t.Fatal("current stage state changed the original workflow GET or its effects")
	}
	var recoveredAuthorization service.DeliveryCompletionAuthorizationView
	authorizationGet := f.call(t, f.initiatorToken, http.MethodGet, completionAuthorizationRouterPath+"/"+grant.AuthorizationID, nil).Want(http.StatusOK)
	authorizationGet.JSON(&recoveredAuthorization)
	if authorizationGet.Header().Get("Cache-Control") != "no-store" || !reflect.DeepEqual(recoveredAuthorization, originalAuthorization) || collaborationCompletionReadRows(t, f) != baseline {
		t.Fatal("current stage state changed the original minimal consumed receipt")
	}
	replayed := f.call(t, f.initiatorToken, http.MethodPost, f.pathFor("/lifecycle"), originalRaw, "Idempotency-Key", req.OperationID).Want(http.StatusOK).Map()
	if !reflect.DeepEqual(replayed, completed) || sha256.Sum256(originalRaw) != originalBodyHash || collaborationCompletionReadRows(t, f) != baseline || f.fx.Count(t, `SELECT count(*) FROM delivery_completion_terminal WHERE authorization_id=$1`, grant.AuthorizationID) != 1 || !bytes.Equal(archive, f.archive(t)) {
		t.Fatal("old completion raw body/key consumed again or adopted the reopened state")
	}
	// This operation is unused by the workflow. Its real old authorization must
	// not acquire current completion authority after activation and reopen.
	freshKey := unusedReq.OperationID
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_operation WHERE scope_id=$1 AND request_id=$2`, f.parent, freshKey) != 0 {
		t.Fatal("pending authorization already has a workflow operation")
	}
	freshBody := f.workflowBody(t, f.initiatorToken, "complete")
	freshBody["note"], freshBody["basis"] = unusedReq.Target.Result, unusedReq.Target.Basis
	freshBody["completion"] = service.DeliveryCompletionBinding{AuthorizationID: unusedGrant.AuthorizationID, OperationID: freshKey, ExpectedIssueRevision: unusedReq.Target.ExpectedIssueRevision}
	freshRaw, err := json.Marshal(freshBody)
	if err != nil {
		t.Fatal(err)
	}
	beforeFresh := collaborationCompletionReadRows(t, f)
	freshResponse := f.call(t, f.initiatorToken, http.MethodPost, f.pathFor("/lifecycle"), freshRaw, "Idempotency-Key", freshKey)
	refused := freshResponse.Map()
	t.Logf("K06 fresh current completion with unused old grant status=%d code=%v", freshResponse.Code, refused["code"])
	freshResponse.Want(http.StatusConflict)
	if refused["code"] != "delivery_completion_conflict" || refused["status"] != float64(http.StatusConflict) || refused["delivery_confirmation"] != nil || refused["stage_confirmation"] != nil || f.fx.Count(t, `SELECT count(*) FROM delivery_completion_terminal WHERE authorization_id=$1`, unusedGrant.AuthorizationID) != 0 || f.fx.Count(t, `SELECT count(*) FROM delivery_completion_terminal WHERE authorization_id=$1`, grant.AuthorizationID) != 1 {
		t.Fatalf("old unused approval revived or fresh completion was not durably refused: %v", refused)
	}
	var beforeState, afterState map[string]any
	if err = json.Unmarshal([]byte(beforeFresh), &beforeState); err != nil {
		t.Fatal(err)
	}
	afterFresh := collaborationCompletionReadRows(t, f)
	if err = json.Unmarshal([]byte(afterFresh), &afterState); err != nil {
		t.Fatal(err)
	}
	operations, _ := afterState["operations"].([]any)
	kept := make([]any, 0, len(operations))
	newRefusals := 0
	for _, value := range operations {
		operation, _ := value.(map[string]any)
		if operation["request_id"] == freshKey {
			newRefusals++
			result, _ := operation["result"].(map[string]any)
			if operation["kind"] != "complete" || operation["result_code"] != "delivery_completion_conflict" || operation["completed_at"] == nil || result["code"] != "delivery_completion_conflict" || result["status"] != float64(http.StatusConflict) {
				t.Fatal("fresh refusal operation is not its exact bounded terminal")
			}
			continue
		}
		kept = append(kept, value)
	}
	afterState["operations"] = kept
	if newRefusals != 1 || !reflect.DeepEqual(beforeState, afterState) {
		t.Fatal("fresh completion changed business facts beyond its one refusal operation")
	}
	var recoveredRefusal service.CollaborationWorkflowOperationReceipt
	f.call(t, f.initiatorToken, http.MethodGet, f.pathFor("/operations/"+freshKey), nil).Want(http.StatusOK).JSON(&recoveredRefusal)
	if recoveredRefusal.Status != http.StatusConflict || recoveredRefusal.Code != "delivery_completion_conflict" || collaborationCompletionReadRows(t, f) != afterFresh {
		t.Fatal("fresh refusal original GET changed its result or business facts")
	}
	refusalReplay := f.call(t, f.initiatorToken, http.MethodPost, f.pathFor("/lifecycle"), freshRaw, "Idempotency-Key", freshKey).Want(http.StatusConflict).Map()
	if !reflect.DeepEqual(refusalReplay, refused) || collaborationCompletionReadRows(t, f) != afterFresh || !bytes.Equal(archive, f.archive(t)) || f.fx.Count(t, `SELECT count(*) FROM member WHERE workspace_id=$1 AND user_id=$2`, testWorkspaceID, f.initiator) != 0 {
		t.Fatal("fresh refusal replay changed history, source bytes or consumer authority")
	}
	t.Logf("K06 history and refusal replay stable before_sha256=%x after_sha256=%x allowed_fresh_refusal_operations=%d pending_terminal_count=%d source_consumer_membership=%d", sha256.Sum256([]byte(afterFresh)), sha256.Sum256([]byte(collaborationCompletionReadRows(t, f))), newRefusals, f.fx.Count(t, `SELECT count(*) FROM delivery_completion_terminal WHERE authorization_id=$1`, unusedGrant.AuthorizationID), f.fx.Count(t, `SELECT count(*) FROM member WHERE workspace_id=$1 AND user_id=$2`, testWorkspaceID, f.initiator))
}
