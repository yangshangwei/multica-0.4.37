package handler

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/testutil"
)

func TestStageHTTPCodeReviewContractAndProtectedEvidence(t *testing.T) {
	s := stageHTTPFixture(t)
	s.revise(t, true)
	reviewer := s.fx.User(t, "Synthetic stage reviewer", "stage-review-"+uuid.NewString()+"@example.test")
	release := s.fx.User(t, "Synthetic stage releaser", "stage-release-"+uuid.NewString()+"@example.test")
	s.fx.Cleanup(t, `DELETE FROM collaboration_account_fence WHERE user_id=ANY($1::uuid[])`, []string{reviewer, release})
	for _, user := range []string{reviewer, release} {
		testutil.Call(t, s.h.InviteCollaborationParticipant, s.request(http.MethodPost, "/participants", s.actor, uuid.NewString(), s.body(t, s.actor, map[string]any{"target_user_id": user}))).Want(http.StatusOK)
	}
	s.fx.Member(t, s.fx.WorkspaceID, reviewer, "member")
	roles := s.roles()
	roles["code_reviewer_user_id"], roles["qa_approver_user_id"], roles["release_approver_user_id"] = reviewer, s.actor, release
	testutil.Call(t, s.h.ConfigureCollaborationStageRoles, s.request(http.MethodPost, "/stage/roles", s.actor, uuid.NewString(), s.body(t, s.actor, map[string]any{"expected_policy_revision": "1", "expected_role_revision": "0", "roles": roles}))).Want(http.StatusOK)
	child := s.fx.Issue(t, "Synthetic HTTP code child", testutil.Cols{"parent_issue_id": s.issue, "project_id": s.project})
	view := testutil.Call(t, s.h.GetCollaborationStage, s.request(http.MethodGet, "/stage", s.actor, "", nil)).Want(http.StatusOK).Map()
	item := view["children"].(map[string]any)["items"].([]any)[0].(map[string]any)
	testutil.Call(t, s.h.ConfirmCollaborationStageScope, s.request(http.MethodPost, "/stage/scope-confirmations", s.actor, uuid.NewString(), s.body(t, s.actor, map[string]any{
		"expected_policy_revision": "1", "expected_role_revision": "1", "expected_scope_revision": "0", "children": []any{map[string]any{"issue_id": child, "content_digest": item["content_digest"], "code": nil}}, "reason": ""}))).Want(http.StatusOK)
	head := strings.Repeat("b", 40)
	now := time.Now()
	s.fx.Insert(t, "github_installation", testutil.Cols{"workspace_id": s.fx.WorkspaceID, "installation_id": int64(1), "account_login": "synthetic-http", "account_type": "User"})
	pr := s.fx.Insert(t, "github_pull_request", testutil.Cols{"workspace_id": s.fx.WorkspaceID, "installation_id": int64(1), "repo_owner": "synthetic", "repo_name": uuid.NewString(), "pr_number": 1,
		"title": "Private synthetic PR title", "state": "open", "html_url": "https://example.invalid/private", "pr_created_at": now, "pr_updated_at": now, "head_sha": head,
		"snapshot_head_sha": head, "snapshot_fetched_at": now.Add(-time.Minute), "snapshot_complete": true})
	s.fx.InsertNoID(t, "issue_pull_request", testutil.Cols{"issue_id": child, "pull_request_id": pr}, "issue_id=$1 AND pull_request_id=$2", child, pr)
	s.fx.InsertNoID(t, "github_pull_request_check_run", testutil.Cols{"pr_id": pr, "head_sha": head, "ordinal": int32(0), "name": "Synthetic passing CI", "status": "completed", "conclusion": "success"}, "pr_id=$1", pr)
	codeFields := map[string]any{"expected_policy_revision": "1", "expected_role_revision": "1", "child_issue_id": child, "supersedes": nil, "pull_request": map[string]any{"provider": "github", "pull_request_id": pr, "head_sha": head}, "artifact": nil}
	key := uuid.NewString()
	codeBody := s.body(t, s.actor, codeFields)
	created := testutil.Call(t, s.h.CreateCollaborationStageCodeCandidate, s.request(http.MethodPost, "/stage/code-candidates", s.actor, key, codeBody)).Want(http.StatusOK).Map()
	confirmation := created["stage_confirmation"].(map[string]any)
	if confirmation["action"] != "create_code_candidate" || confirmation["parent_candidate_id"] != nil {
		t.Fatalf("A6 confirmation: %v", confirmation)
	}
	if _, ok := confirmation["parent_candidate_id"]; !ok {
		t.Fatal("A6 omitted required null parent_candidate_id")
	}
	candidate := map[string]any{"candidate_id": confirmation["candidate_id"], "version": confirmation["version"], "content_digest": confirmation["content_digest"]}
	candidateID := confirmation["candidate_id"].(string)
	read := func(actor string) *testutil.Response {
		return testutil.Call(t, s.h.GetCollaborationStageCodeEvidence, s.request(http.MethodGet, "/stage/code-candidates/"+candidateID, actor, "", nil, "candidateId", candidateID))
	}
	redacted := read(s.recipient).Want(http.StatusOK)
	private := redacted.Map()
	if private["candidate"].(map[string]any)["baseline"] != nil || private["blocked_evidence"] != float64(1) || strings.Contains(redacted.Text(), head) || strings.Contains(redacted.Text(), pr) || strings.Contains(redacted.Text(), child) {
		t.Fatalf("R4 leaked source evidence: %s", redacted.Text())
	}
	for _, raw := range private["signable"].([]any) {
		hint := raw.(map[string]any)
		if hint["allowed"] != false || hint["max_expires_at"] != nil {
			t.Fatalf("R4 redacted signable: %v", hint)
		}
		for _, raw := range hint["blockers"].([]any) {
			blocker := raw.(map[string]any)
			switch blocker["stage"] {
			case "development", "code_review", "qa", "release", "verification":
			default:
				t.Fatalf("invalid blocker stage: %v", blocker)
			}
		}
	}
	advance := s.body(t, s.actor, map[string]any{"expected_policy_revision": "1", "expected_role_revision": "1", "expected_stage_revision": "1", "target_stage": "code_review", "candidate": candidate, "decision_ids": []string{}})
	testutil.Call(t, s.h.AdvanceCollaborationStage, s.request(http.MethodPost, "/stage/advance", s.actor, uuid.NewString(), advance)).Want(http.StatusOK)
	evidence := read(reviewer).Want(http.StatusOK).Map()
	ci := evidence["observed"].(map[string]any)["ci"].(map[string]any)
	if ci["state"] != "passed" || evidence["signable"].([]any)[0].(map[string]any)["allowed"] != true {
		t.Fatalf("R4 review capability: %v", evidence)
	}
	decisionFields := map[string]any{"expected_policy_revision": "1", "expected_role_revision": "1", "expected_stage_revision": "2", "step": "code_review", "decision": "approve", "candidate": candidate,
		"evidence": map[string]any{"ci_digest": ci["ci_digest"], "test_report_id": nil, "build": nil, "deployment_id": nil, "rollback_target_digest": nil}, "depends_on": []string{}, "reason": "Synthetic human approval",
		"expires_at": time.Now().UTC().Add(5 * time.Minute).Truncate(time.Microsecond).Format(time.RFC3339Nano)}
	decisionBody := s.body(t, reviewer, decisionFields)
	decisionKey := uuid.NewString()
	decided := testutil.Call(t, s.h.DecideCollaborationStage, s.request(http.MethodPost, "/stage/decisions", reviewer, decisionKey, decisionBody)).Want(http.StatusOK).Map()["stage_confirmation"].(map[string]any)
	if decided["decision"] != "approved" || decided["expires_at"] == nil {
		t.Fatalf("A7 confirmation: %v", decided)
	}
	recovered := testutil.Call(t, s.h.GetCollaborationOperation, s.request(http.MethodGet, "/operations/"+decisionKey, reviewer, decisionKey, nil)).Want(http.StatusOK).Map()
	if recovered["stage_confirmation"].(map[string]any)["decision_id"] != decided["decision_id"] {
		t.Fatalf("O1 recovery: %v", recovered)
	}
	decisionID := decided["decision_id"].(string)
	revoke := s.body(t, reviewer, map[string]any{"expected_policy_revision": "1", "expected_role_revision": "1", "reason": "Synthetic human withdrawal"})
	testutil.Call(t, s.h.RevokeCollaborationStageDecision, s.request(http.MethodPost, "/stage/decisions/"+decisionID+"/revocations", reviewer, uuid.NewString(), revoke, "decisionId", decisionID)).Want(http.StatusOK)
	if state := read(reviewer).Want(http.StatusOK).Map()["decisions"].([]any)[0].(map[string]any)["state"]; state != "revoked" {
		t.Fatalf("R4 revocation state: %v", state)
	}
	// Current read authority is rechecked; a remembered R4 URL grants nothing.
	s.fx.Exec(t, `DELETE FROM member WHERE workspace_id=$1 AND user_id=$2`, s.fx.WorkspaceID, reviewer)
	s.fx.Exec(t, `UPDATE issue_participant SET revoked_at=clock_timestamp(),epoch=epoch+1 WHERE issue_id=$1 AND user_id=$2`, s.issue, reviewer)
	read(reviewer).Want(http.StatusNotFound)
}

func TestStageHTTPReviewStrictBodies(t *testing.T) {
	s := stageHTTPFixture(t)
	base := map[string]any{"expected_policy_revision": "1", "expected_role_revision": "1", "child_issue_id": uuid.NewString(), "supersedes": nil,
		"pull_request": map[string]any{"provider": "github", "pull_request_id": uuid.NewString(), "head_sha": strings.Repeat("b", 40)}, "artifact": nil}
	for _, name := range []string{"absent_supersedes", "absent_artifact", "upper_alias", "unknown_field", "short_head"} {
		t.Run(name, func(t *testing.T) {
			fields := map[string]any{}
			for k, v := range base {
				fields[k] = v
			}
			switch name {
			case "absent_supersedes":
				delete(fields, "supersedes")
			case "absent_artifact":
				delete(fields, "artifact")
			case "upper_alias":
				fields["Child_issue_id"] = fields["child_issue_id"]
				delete(fields, "child_issue_id")
			case "unknown_field":
				fields["verified"] = true
			case "short_head":
				fields["pull_request"] = map[string]any{"provider": "github", "pull_request_id": uuid.NewString(), "head_sha": "short"}
			}
			testutil.Call(t, s.h.CreateCollaborationStageCodeCandidate, s.request(http.MethodPost, "/stage/code-candidates", s.actor, uuid.NewString(), s.body(t, s.actor, fields))).Want(http.StatusBadRequest)
		})
	}
}

func TestStageHTTPExecutionEvidenceExtensions(t *testing.T) {
	s := stageHTTPFixture(t)
	candidate := map[string]any{"candidate_id": uuid.NewString(), "version": "1", "content_digest": strings.Repeat("a", 64)}
	evidence := func() map[string]any {
		return map[string]any{"ci_digest": nil, "test_report_id": nil, "build": map[string]any{"artifact_digest": strings.Repeat("b", 64), "environment": "synthetic", "target_digest": strings.Repeat("c", 64)}, "deployment_id": nil, "rollback_target_digest": nil}
	}
	fields := func(step string, e map[string]any) map[string]any {
		return s.body(t, s.actor, map[string]any{"expected_policy_revision": "1", "expected_role_revision": "1", "expected_stage_revision": "1", "step": step, "decision": "approve", "candidate": candidate, "evidence": e, "depends_on": []string{uuid.NewString()}, "reason": "Synthetic transport review", "expires_at": time.Now().UTC().Add(time.Minute).Truncate(time.Microsecond).Format(time.RFC3339Nano)})
	}
	valid := evidence()
	out := testutil.Call(t, s.h.DecideCollaborationStage, s.request(http.MethodPost, "/stage/decisions", s.actor, uuid.NewString(), fields("release", valid))).Want(409).Map()
	if out["code"] != "stage_policy_disabled" {
		t.Fatalf("valid2c fields failed before actual current policy: %v", out)
	}
	for _, kind := range []string{"missing_target", "qa_target", "bad_metric", "foreign_field"} {
		t.Run(kind, func(t *testing.T) {
			e := evidence()
			step := "release"
			switch kind {
			case "missing_target":
				delete(e["build"].(map[string]any), "target_digest")
			case "qa_target":
				step = "qa"
			case "bad_metric":
				step = "rollback"
				e["build"] = nil
				e["deployment_id"] = uuid.NewString()
				e["rollback_target_digest"] = strings.Repeat("b", 64)
				e["rollback_observation"] = map[string]any{"observation_id": uuid.NewString(), "observation_digest": "bad"}
			case "foreign_field":
				e["producer_proof"] = "PRIVATE_SENTINEL"
			}
			reply := testutil.Call(t, s.h.DecideCollaborationStage, s.request(http.MethodPost, "/stage/decisions", s.actor, uuid.NewString(), fields(step, e))).Want(400)
			if strings.Contains(reply.Text(), "PRIVATE_SENTINEL") {
				t.Fatal("malformed source proof echoed")
			}
		})
	}
}
