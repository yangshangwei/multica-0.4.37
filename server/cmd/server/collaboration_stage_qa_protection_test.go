package main

import (
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/handler"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// The actual router protects a valid reason-only QA return. The service suite
// separately supplies the settled report for a fully signable opposite approval.
func TestStageQAReturnProtectedThroughClaimedTaskRouter(t *testing.T) {
	t.Setenv("MULTICA_AUTH_MODE", "legacy")
	base := testutil.New(testPool, testWorkspaceID, testUserID)
	source := base.Workspace(t, "QA protection", "qa-protection-"+uuid.NewString())
	base.Member(t, source, testUserID, "owner")
	fx := testutil.New(testPool, source, testUserID)
	q := db.New(testPool)
	if err := q.SeedIssueStatusEntries(t.Context(), util.MustParseUUID(source)); err != nil {
		t.Fatal(err)
	}
	fx.Cleanup(t, `DELETE FROM issue_status WHERE workspace_id=$1`, source)
	project := fx.Project(t, "QA protection source")
	issue := fx.Issue(t, "Human QA is authoritative", testutil.Cols{"project_id": project, "assignee_type": "member", "assignee_id": testUserID})
	fx.InsertNoID(t, "issue_collaboration", testutil.Cols{"issue_id": issue}, "issue_id=$1", issue)
	fx.Insert(t, "collaboration_session", testutil.Cols{"issue_id": issue})
	fx.InsertNoID(t, "issue_participant", testutil.Cols{"issue_id": issue, "user_id": testUserID}, "issue_id=$1 AND user_id=$2", issue, testUserID)
	ensureCollaborationRouteAccountFence(t, fx, testUserID)
	for _, table := range []string{"issue_participant", "collaboration_event", "collaboration_notification", "issue_handoff"} {
		fx.Cleanup(t, "DELETE FROM "+table+" WHERE issue_id=$1", issue)
	}
	fx.Cleanup(t, `DELETE FROM collaboration_operation WHERE scope_kind='issue' AND scope_id=$1`, issue)
	for _, table := range []string{"collaboration_stage_child", "collaboration_stage_revocation", "collaboration_stage_decision", "collaboration_stage_candidate", "collaboration_stage_role", "collaboration_stage_task", "project_stage_policy"} {
		fx.Cleanup(t, "DELETE FROM "+table+" WHERE workspace_id=$1", source)
	}
	call := func(t *testing.T, token, method, path, key string, body any, headers ...string) *testutil.Response {
		t.Helper()
		r := testutil.JSONRequest(method, path, body)
		testutil.WithHeaders(r, "Authorization", "Bearer "+token, "X-Workspace-ID", source)
		if key != "" {
			r.Header.Set("Idempotency-Key", key)
		}
		testutil.WithHeaders(r, headers...)
		return testutil.Call(t, testServer.Config.Handler.ServeHTTP, r)
	}
	root, stage := "/api/collaboration/issues/"+issue, "/api/collaboration/issues/"+issue+"/stage"
	observe := func(t *testing.T, token string, fields map[string]any) map[string]any {
		t.Helper()
		current := call(t, token, http.MethodGet, root, "", nil).Want(http.StatusOK).Map()
		write := current["write_context"].(map[string]any)
		body := map[string]any{"expected_collaboration_revision": current["collaboration_revision"], "expected_lifecycle_epoch": current["lifecycle_epoch"],
			"expected_participant_epoch": write["participant_epoch"], "expected_account_epoch": write["account_epoch"]}
		for key, value := range fields {
			body[key] = value
		}
		return body
	}
	stageView := func(t *testing.T) service.CollaborationStageView {
		t.Helper()
		var out service.CollaborationStageView
		call(t, testToken, http.MethodGet, stage, "", nil).Want(http.StatusOK).JSON(&out)
		return out
	}
	reviewer := fx.User(t, "QA protection reviewer", "qa-reviewer-"+uuid.NewString()+"@example.test")
	releaser := fx.User(t, "QA protection releaser", "qa-releaser-"+uuid.NewString()+"@example.test")
	for _, user := range []string{reviewer, releaser} {
		fx.Member(t, source, user, "member")
		fx.Cleanup(t, `DELETE FROM collaboration_account_fence WHERE user_id=$1`, user)
		call(t, testToken, http.MethodPost, root+"/participants", uuid.NewString(), observe(t, testToken, map[string]any{"target_user_id": user})).Want(http.StatusOK)
	}
	reviewerToken, err := generateTestJWT(reviewer, "qa-reviewer@example.test", "QA protection reviewer")
	if err != nil {
		t.Fatal(err)
	}
	call(t, testToken, http.MethodPut, "/api/projects/"+project+"/stage-policy", uuid.NewString(), map[string]any{"expected_policy_revision": "0", "enabled": true, "reason": "Verify human QA protection"}).Want(http.StatusOK)
	roles := map[string]any{"business_owner_user_id": testUserID, "implementer_user_id": testUserID, "product_approver_user_id": testUserID,
		"code_reviewer_user_id": reviewer, "qa_approver_user_id": testUserID, "release_approver_user_id": releaser}
	call(t, testToken, http.MethodPost, stage+"/roles", uuid.NewString(), observe(t, testToken, map[string]any{"expected_policy_revision": "1", "expected_role_revision": "0", "roles": roles})).Want(http.StatusOK)
	child := fx.Issue(t, "Reviewed child", testutil.Cols{"parent_issue_id": issue, "project_id": project})
	view := stageView(t)
	if len(view.Children.Items) != 1 || view.Children.Items[0].IssueID != child || view.Children.Items[0].ContentDigest == nil {
		t.Fatalf("actual scope child not readable: %+v", view.Children)
	}
	call(t, testToken, http.MethodPost, stage+"/scope-confirmations", uuid.NewString(), observe(t, testToken, map[string]any{
		"expected_policy_revision": "1", "expected_role_revision": view.RoleRevision, "expected_scope_revision": view.ScopeRevision,
		"children": []any{map[string]any{"issue_id": child, "content_digest": *view.Children.Items[0].ContentDigest, "code": nil}}, "reason": "Explicit human scope"})).Want(http.StatusOK)
	head := strings.Repeat("b", 40)
	now := time.Now().UTC()
	fx.Insert(t, "github_installation", testutil.Cols{"workspace_id": source, "installation_id": int64(1), "account_login": "qa-protection", "account_type": "User"})
	pr := fx.Insert(t, "github_pull_request", testutil.Cols{"workspace_id": source, "installation_id": int64(1), "repo_owner": "synthetic", "repo_name": uuid.NewString(), "pr_number": 1,
		"title": "Synthetic checked PR", "state": "open", "html_url": "https://example.invalid/qa", "pr_created_at": now, "pr_updated_at": now, "head_sha": head,
		"snapshot_head_sha": head, "snapshot_fetched_at": now.Add(-time.Minute), "snapshot_complete": true})
	fx.InsertNoID(t, "issue_pull_request", testutil.Cols{"issue_id": child, "pull_request_id": pr}, "issue_id=$1 AND pull_request_id=$2", child, pr)
	fx.InsertNoID(t, "github_pull_request_check_run", testutil.Cols{"pr_id": pr, "head_sha": head, "ordinal": int32(0), "name": "Synthetic passing CI", "status": "completed", "conclusion": "success"}, "pr_id=$1", pr)
	created := call(t, testToken, http.MethodPost, stage+"/code-candidates", uuid.NewString(), observe(t, testToken, map[string]any{
		"expected_policy_revision": "1", "expected_role_revision": "1", "child_issue_id": child, "supersedes": nil,
		"pull_request": map[string]any{"provider": "github", "pull_request_id": pr, "head_sha": head}, "artifact": nil})).Want(http.StatusOK).Map()
	confirmation := created["stage_confirmation"].(map[string]any)
	candidateID := confirmation["candidate_id"].(string)
	candidate := map[string]any{"candidate_id": candidateID, "version": confirmation["version"], "content_digest": confirmation["content_digest"]}
	advance := func(target string, decisions []string) {
		v := stageView(t)
		call(t, testToken, http.MethodPost, stage+"/advance", uuid.NewString(), observe(t, testToken, map[string]any{"expected_policy_revision": "1", "expected_role_revision": v.RoleRevision,
			"expected_stage_revision": v.Stage.StageRevision, "target_stage": target, "candidate": candidate, "decision_ids": decisions})).Want(http.StatusOK)
	}
	advance("code_review", []string{})
	var codeEvidence service.CollaborationStageCodeEvidenceView
	call(t, reviewerToken, http.MethodGet, stage+"/code-candidates/"+candidateID, "", nil).Want(http.StatusOK).JSON(&codeEvidence)
	if codeEvidence.Observed.CI.CIDigest == nil || !codeEvidence.Signable[0].Allowed {
		t.Fatal("real code review evidence must be signable")
	}
	nullEvidence := func() map[string]any {
		return map[string]any{"ci_digest": nil, "test_report_id": nil, "build": nil, "deployment_id": nil, "rollback_target_digest": nil}
	}
	evidence := nullEvidence()
	evidence["ci_digest"] = *codeEvidence.Observed.CI.CIDigest
	v := stageView(t)
	code := call(t, reviewerToken, http.MethodPost, stage+"/decisions", uuid.NewString(), observe(t, reviewerToken, map[string]any{"expected_policy_revision": "1", "expected_role_revision": v.RoleRevision,
		"expected_stage_revision": v.Stage.StageRevision, "step": "code_review", "decision": "approve", "candidate": candidate, "evidence": evidence,
		"depends_on": []string{}, "reason": "Human code review", "expires_at": now.Add(3 * time.Minute).Truncate(time.Microsecond).Format(time.RFC3339Nano)})).Want(http.StatusOK).Map()
	codeID := code["stage_confirmation"].(map[string]any)["decision_id"].(string)
	advance("qa", []string{codeID})
	v = stageView(t)
	qaKey := uuid.NewString()
	qaBody := observe(t, testToken, map[string]any{"expected_policy_revision": "1", "expected_role_revision": v.RoleRevision, "expected_stage_revision": v.Stage.StageRevision,
		"step": "qa", "decision": "return", "candidate": candidate, "evidence": nullEvidence(), "depends_on": []string{}, "reason": "Human QA corrects the automated assessment", "expires_at": nil})
	human := call(t, testToken, http.MethodPost, stage+"/decisions", qaKey, qaBody).Want(http.StatusOK).Map()
	decision := human["stage_confirmation"].(map[string]any)
	decisionID := decision["decision_id"].(string)
	if decision["decision"] != "returned" {
		t.Fatalf("real human QA return missing: %v", decision)
	}
	originalReceipt := call(t, testToken, http.MethodGet, root+"/operations/"+qaKey, "", nil).Want(http.StatusOK).Map()
	beforeStage := stageView(t)
	if beforeStage.Stage.Current != "qa" {
		t.Fatal("QA correction must remain current before the Agent attempt")
	}
	claims := []struct {
		name, token, agent string
		restricted         bool
	}{}
	for _, restricted := range []bool{false, true} {
		runtime := fx.Runtime(t, "QA protection task runtime")
		cols := testutil.Cols{"autonomy_level": "observer"}
		name := "generic_claimed_task"
		if restricted {
			name = "restricted_claimed_task"
			cols["template_key"], cols["template_version"] = "code-reviewer", 2
		}
		agent := fx.Agent(t, name, runtime, cols)
		ordinary := fx.Issue(t, "Permitted task read", testutil.Cols{"project_id": project})
		task := fx.Task(t, agent, testutil.Cols{"runtime_id": runtime, "issue_id": ordinary, "originator_user_id": testUserID, "accountable_user_id": testUserID})
		fx.Cleanup(t, `DELETE FROM task_token WHERE task_id=$1`, task)
		var claimed struct {
			Task *handler.AgentTaskResponse `json:"task"`
		}
		call(t, testToken, http.MethodPost, "/api/daemon/runtimes/"+runtime+"/tasks/claim", "", nil).Want(http.StatusOK).JSON(&claimed)
		if claimed.Task == nil || claimed.Task.ID != task || claimed.Task.AgentID != agent || claimed.Task.IssueID != ordinary || !strings.HasPrefix(claimed.Task.AuthToken, "mat_") {
			t.Fatal("actual daemon claim did not issue the exact task credential")
		}
		token, readErr := q.GetTaskTokenByHash(t.Context(), auth.HashToken(claimed.Task.AuthToken))
		if readErr != nil || token.UserID != util.MustParseUUID(testUserID) || token.TaskID != util.MustParseUUID(task) || token.AgentID != util.MustParseUUID(agent) || token.WorkspaceID != util.MustParseUUID(source) {
			t.Fatalf("claimed credential binding: error=%v", readErr)
		}
		if restricted {
			capability, parseErr := auth.ParseTaskCapability(token.ApiCapability)
			if parseErr != nil || capability.Profile != auth.TaskCapabilityReviewer || capability.IssueID != ordinary || capability.RuntimeID != runtime || capability.ClaimGeneration < 1 {
				t.Fatalf("restricted claim has no exact frozen capability: %+v %v", capability, parseErr)
			}
		} else if len(token.ApiCapability) != 0 {
			t.Fatal("generic claim cannot be mistaken for restricted route denial")
		}
		allowed := call(t, claimed.Task.AuthToken, http.MethodGet, "/api/issues/"+ordinary, "", nil).Want(http.StatusOK).Map()
		if allowed["id"] != ordinary || allowed["workspace_id"] != source {
			t.Fatal("same issued token did not perform its positive ordinary read")
		}
		claims = append(claims, struct {
			name, token, agent string
			restricted         bool
		}{name, claimed.Task.AuthToken, agent, restricted})
	}
	claims = append(claims, struct {
		name, token, agent string
		restricted         bool
	}{"human_with_agent_hint", testToken, claims[0].agent, false})
	baseline := tc13RouterQAFacts(t, fx, source)
	var beforeEvidence service.CollaborationStageCodeEvidenceView
	call(t, testToken, http.MethodGet, stage+"/code-candidates/"+candidateID, "", nil).Want(http.StatusOK).JSON(&beforeEvidence)
	for _, actor := range claims {
		for _, action := range []string{"opposite_decision", "exact_revocation"} {
			t.Run(actor.name+"/"+action, func(t *testing.T) {
				current := stageView(t)
				fields := map[string]any{"expected_policy_revision": "1", "expected_role_revision": current.RoleRevision, "reason": "Agent attempts to replace the human QA decision"}
				path := stage + "/decisions/" + decisionID + "/revocations"
				if action == "opposite_decision" {
					path = stage + "/decisions"
					fields["expected_stage_revision"], fields["step"], fields["decision"], fields["candidate"] = current.Stage.StageRevision, "qa", "approve", candidate
					fields["evidence"], fields["depends_on"] = nullEvidence(), []string{codeID}
					fields["expires_at"] = time.Now().UTC().Add(time.Minute).Truncate(time.Microsecond).Format(time.RFC3339Nano)
				}
				key := uuid.NewString()
				body := observe(t, testToken, fields)
				headers := []string{}
				if actor.name == "human_with_agent_hint" {
					headers = []string{"X-Agent-ID", actor.agent}
				}
				denied := call(t, actor.token, http.MethodPost, path, key, body, headers...).Want(http.StatusForbidden).Map()
				if actor.restricted {
					if denied["error"] != "task capability does not authorize this action or resource" {
						t.Fatalf("restricted task missed the capability guard: %v", denied)
					}
				} else if denied["code"] != "forbidden" {
					t.Fatalf("valid generic task/hinted JWT missed the human-only stage guard: %v", denied)
				}
				if !reflect.DeepEqual(tc13RouterQAFacts(t, fx, source), baseline) || !reflect.DeepEqual(stageView(t), beforeStage) {
					t.Fatal("refused Agent request changed QA history, stage or business facts")
				}
				var currentEvidence service.CollaborationStageCodeEvidenceView
				call(t, testToken, http.MethodGet, stage+"/code-candidates/"+candidateID, "", nil).Want(http.StatusOK).JSON(&currentEvidence)
				currentEvidence.CheckedAt, beforeEvidence.CheckedAt = time.Time{}, time.Time{}
				for i := range currentEvidence.Signable {
					currentEvidence.Signable[i].MaxExpiresAt = nil
				}
				for i := range beforeEvidence.Signable {
					beforeEvidence.Signable[i].MaxExpiresAt = nil
				}
				if !reflect.DeepEqual(currentEvidence, beforeEvidence) {
					t.Fatal("refused Agent request changed the current QA evidence projection")
				}
				recovered := call(t, testToken, http.MethodGet, root+"/operations/"+qaKey, "", nil).Want(http.StatusOK).Map()
				if !reflect.DeepEqual(recovered, originalReceipt) || fx.Count(t, `SELECT count(*) FROM collaboration_operation WHERE scope_kind='issue' AND scope_id=$1 AND request_id=$2`, issue, key) != 0 {
					t.Fatal("Agent refusal changed the human receipt or persisted a workflow operation")
				}
			})
		}
	}
	// The same opposite envelope without a report reaches the business validator
	// as a human. Its distinct 409 separates auth proof from QA admissibility.
	current := stageView(t)
	humanOpposite := observe(t, testToken, map[string]any{"expected_policy_revision": "1", "expected_role_revision": current.RoleRevision,
		"expected_stage_revision": current.Stage.StageRevision, "step": "qa", "decision": "approve", "candidate": candidate,
		"evidence": nullEvidence(), "depends_on": []string{codeID}, "reason": "Human negative control requires a genuine report",
		"expires_at": time.Now().UTC().Add(time.Minute).Truncate(time.Microsecond).Format(time.RFC3339Nano)})
	control := call(t, testToken, http.MethodPost, stage+"/decisions", uuid.NewString(), humanOpposite).Want(http.StatusConflict).Map()
	if control["code"] != "stage_test_report_unusable" {
		t.Fatalf("human control must reach the missing-report business gate: %v", control)
	}
}

func tc13RouterQAFacts(t *testing.T, fx *testutil.Fixture, workspace string) map[string]json.RawMessage {
	t.Helper()
	result := map[string]json.RawMessage{}
	for table, predicate := range map[string]string{
		"issue": "workspace_id=$1", "issue_collaboration": "issue_id IN (SELECT id FROM issue WHERE workspace_id=$1)",
		"collaboration_stage_task": "workspace_id=$1", "collaboration_stage_role": "workspace_id=$1", "collaboration_stage_child": "workspace_id=$1",
		"collaboration_stage_candidate": "workspace_id=$1", "collaboration_stage_decision": "workspace_id=$1", "collaboration_stage_revocation": "workspace_id=$1",
		"collaboration_operation": "scope_kind='issue' AND scope_id IN (SELECT id FROM issue WHERE workspace_id=$1)",
		"collaboration_event":     "issue_id IN (SELECT id FROM issue WHERE workspace_id=$1)", "collaboration_notification": "issue_id IN (SELECT id FROM issue WHERE workspace_id=$1)",
		"collaboration_session": "issue_id IN (SELECT id FROM issue WHERE workspace_id=$1)", "comment": "workspace_id=$1",
		"collaboration_command":   "session_id IN (SELECT s.id FROM collaboration_session s JOIN issue i ON i.id=s.issue_id WHERE i.workspace_id=$1)",
		"collaboration_run":       "session_id IN (SELECT s.id FROM collaboration_session s JOIN issue i ON i.id=s.issue_id WHERE i.workspace_id=$1)",
		"collaboration_run_event": "run_id IN (SELECT r.id FROM collaboration_run r JOIN collaboration_session s ON s.id=r.session_id JOIN issue i ON i.id=s.issue_id WHERE i.workspace_id=$1)",
		"agent_task_queue":        "agent_id IN (SELECT id FROM agent WHERE workspace_id=$1)",
	} {
		var raw string
		fx.QueryRow(t, `SELECT COALESCE(jsonb_agg(to_jsonb(x) ORDER BY to_jsonb(x)::text),'[]'::jsonb)::text FROM `+table+` x WHERE `+predicate, workspace).Scan(&raw)
		result[table] = json.RawMessage(raw)
	}
	return result
}
