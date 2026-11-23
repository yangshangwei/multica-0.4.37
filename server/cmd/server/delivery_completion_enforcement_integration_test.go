package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/testutil"
)

// P4 uses the real owner/admin policy API. Unlike internal P1/P3 fixtures, no
// enforce policy or successful artifact/authorization/terminal row is inserted.
func activateCompletionPolicy(t *testing.T, f *deliveryPolicyRouterFixture, expected int64) {
	t.Helper()
	f.fx.Exec(t, `UPDATE member SET role='admin' WHERE workspace_id=$1 AND user_id=$2`, testWorkspaceID, f.user)
	var response map[string]any
	f.call(t, f.token, "PUT", f.policyPath(), f.policyBody(expected, "enforce")).Want(200).JSON(&response)
	if response["mode"] != "enforce" || response["completion_contract_version"] != "delivery-completion-enforcement-v1" || response["skip_supported"] != false {
		t.Fatalf("activation omitted fixed completion capabilities: %+v", response)
	}
	checks, ok := response["waivable_checks"].([]any)
	if !ok || len(checks) != 0 {
		t.Fatal("policy exposed a nonexistent skip engine")
	}
	f.fx.Exec(t, `UPDATE member SET role='member' WHERE workspace_id=$1 AND user_id=$2`, testWorkspaceID, f.user)
}
func publicCompletionGrant(t *testing.T, f *deliveryPolicyRouterFixture, status string) completionAuthorizationRouterResponse {
	t.Helper()
	body := completionAuthorizationRouterBody(t, f, 1, map[string]any{"kind": "human", "user_id": f.user})
	body["target"].(map[string]any)["target_status"] = status
	var grant completionAuthorizationRouterResponse
	f.call(t, f.token, "POST", completionAuthorizationRouterPath, body, "Idempotency-Key", uuid.NewString()).Want(201).JSON(&grant)
	return grant
}
func publicCompletionBody(grant completionAuthorizationRouterResponse, status string) map[string]any {
	return map[string]any{"status": status, "expected_revision": grant.IssueRevision, "completion": map[string]any{"authorization_id": grant.AuthorizationID, "operation_id": grant.OperationID, "expected_issue_revision": grant.IssueRevision}}
}
func assertPublicCompletion(t *testing.T, f *deliveryPolicyRouterFixture, grant completionAuthorizationRouterResponse, status string) {
	t.Helper()
	var state, operation, stored string
	var rev int64
	f.fx.QueryRow(t, `SELECT i.status,i.revision,t.operation_id::text,t.status_after FROM issue i JOIN delivery_completion_authorization a ON a.issue_id=i.id JOIN delivery_completion_terminal t ON t.authorization_id=a.id WHERE a.id=$1 AND t.kind='consumed'`, grant.AuthorizationID).Scan(&state, &rev, &operation, &stored)
	if state != status || stored != status || operation != grant.OperationID || rev <= grant.IssueRevision {
		t.Fatal("public writer omitted the exact atomic completion receipt")
	}
}

func TestDeliveryCompletionEnforcementRouterPublicActivation(t *testing.T) {
	for _, role := range []string{"owner", "admin"} {
		t.Run(role, func(t *testing.T) {
			f := newDeliveryPolicyRouterFixture(t, "password", false)
			f.fx.Exec(t, `UPDATE member SET role=$1 WHERE workspace_id=$2 AND user_id=$3`, role, testWorkspaceID, f.user)
			var policy deliveryPolicyRouterResponse
			f.call(t, f.token, "PUT", f.policyPath(), f.policyBody(0, "enforce")).Want(200).JSON(&policy)
			if policy.Mode != "enforce" || policy.Revision != 1 {
				t.Fatal(policy)
			}
		})
	}
}
func TestDeliveryCompletionEnforcementRouterOrdinaryAndMove(t *testing.T) {
	for _, writer := range []string{"put", "move", "custom_done"} {
		t.Run(writer, func(t *testing.T) {
			f := newDeliveryPolicyRouterFixture(t, "password", true)
			archive := f.archive(t)
			activateCompletionPolicy(t, f, 0)
			status := "done"
			if writer == "custom_done" {
				status = "accepted_custom"
				f.fx.Insert(t, "issue_status", testutil.Cols{"workspace_id": testWorkspaceID, "key": status, "name": "Accepted custom", "category": "done", "color": "#000000", "is_system": false, "position": 100})
			}
			var grant completionAuthorizationRouterResponse
			if writer == "move" {
				grantBody := completionAuthorizationRouterBody(t, f, 1, map[string]any{"kind": "human", "user_id": f.user})
				var position float64
				f.fx.QueryRow(t, `SELECT position FROM issue WHERE id=$1`, f.request.ResourceID).Scan(&position)
				grantBody["target"].(map[string]any)["target_position"] = position
				f.call(t, f.token, "POST", completionAuthorizationRouterPath, grantBody, "Idempotency-Key", uuid.NewString()).Want(201).JSON(&grant)
			} else {
				grant = publicCompletionGrant(t, f, status)
			}
			method, path := "PUT", "/api/issues/"+f.request.ResourceID
			completionBody := publicCompletionBody(grant, status)
			if writer == "move" {
				method, path = "POST", path+"/move"
				completionBody["before_id"] = nil
				completionBody["after_id"] = nil
			}
			f.call(t, f.token, method, path, completionBody).Want(200)
			assertPublicCompletion(t, f, grant, status)
			if !bytes.Equal(archive, f.archive(t)) {
				t.Fatal("completion recreated or changed producer archive")
			}
			// Once committed, the original retry recovers; it must not consume twice.
			f.call(t, f.token, method, path, completionBody).Want(200)
			if f.fx.Count(t, `SELECT count(*) FROM delivery_completion_terminal WHERE authorization_id=$1`, grant.AuthorizationID) != 1 {
				t.Fatal("retry duplicated terminal fact")
			}
		})
	}
}
func TestDeliveryCompletionEnforcementRouterBatchCommittedPrefix(t *testing.T) {
	f := newDeliveryPolicyRouterFixture(t, "password", true)
	activateCompletionPolicy(t, f, 0)
	grant := publicCompletionGrant(t, f, "done")
	second := f.fx.Issue(t, "Second requires its own evidence", testutil.Cols{"project_id": f.project})
	third := f.fx.Issue(t, "Third not attempted", testutil.Cols{"project_id": f.project})
	var body struct {
		Updated int    `json:"updated"`
		Code    string `json:"code"`
		Results []struct {
			Index   int    `json:"index"`
			IssueID string `json:"issue_id"`
			State   string `json:"state"`
			Code    string `json:"code"`
		} `json:"results"`
	}
	f.call(t, f.token, "POST", "/api/issues/batch-update", map[string]any{"issue_ids": []string{f.request.ResourceID, second, third}, "updates": map[string]any{"status": "done"}, "completion_bindings": []map[string]any{{"issue_id": f.request.ResourceID, "authorization_id": grant.AuthorizationID, "operation_id": grant.OperationID, "expected_issue_revision": grant.IssueRevision}}}).Want(http.StatusPreconditionRequired).JSON(&body)
	if body.Updated != 1 || body.Code != "batch_update_incomplete" || len(body.Results) != 3 || body.Results[0].State != "committed" || body.Results[1].State != "rejected" || body.Results[2].State != "not_attempted" {
		t.Fatalf("batch lost per-item commit facts: %+v", body)
	}
	assertPublicCompletion(t, f, grant, "done")
	if f.fx.Count(t, `SELECT count(*) FROM issue WHERE id=ANY($1::uuid[]) AND status='done'`, []string{second, third}) != 0 {
		t.Fatal("rejected/unattempted prefix changed")
	}
}
func TestDeliveryCompletionEnforcementRouterCreateAndCommentChild(t *testing.T) {
	f := newDeliveryPolicyRouterFixture(t, "password", true)
	activateCompletionPolicy(t, f, 0)
	// The shared artifact fixture inserts numbered rows directly. Align its
	// workspace counter before exercising the real allocation path.
	f.fx.Exec(t, `UPDATE workspace SET issue_counter=GREATEST(issue_counter,COALESCE((SELECT max(number) FROM issue WHERE workspace_id=$1),0)) WHERE id=$1`, testWorkspaceID)
	for _, status := range []string{"done", "custom_create_done"} {
		t.Run(status, func(t *testing.T) {
			if status != "done" {
				f.fx.Insert(t, "issue_status", testutil.Cols{"workspace_id": testWorkspaceID, "key": status, "name": "Custom creation done", "category": "done", "color": "#000000", "is_system": false, "position": 100})
			}
			title := "Rejected completed child " + uuid.NewString()
			f.call(t, f.token, "POST", "/api/issues", map[string]any{"title": title, "status": status, "parent_issue_id": f.request.ResourceID}).Want(409)
			if f.fx.Count(t, `SELECT count(*) FROM issue WHERE title=$1`, title) != 0 {
				t.Fatal("refused creation left a task")
			}
		})
	}
	comment := f.fx.Comment(t, f.request.ResourceID, "Convert this discussion into a child")
	var preview struct {
		CaptureToken string `json:"capture_token"`
	}
	f.call(t, f.token, "GET", "/api/comments/"+comment+"/sub-issue-preview", nil).Want(200).JSON(&preview)
	title := "Rejected comment child " + uuid.NewString()
	f.call(t, f.token, "POST", "/api/comments/"+comment+"/sub-issues", map[string]any{"mode": "manual", "capture_token": preview.CaptureToken, "issue": map[string]any{"title": title, "status": "done"}}).Want(409)
	if f.fx.Count(t, `SELECT count(*) FROM issue WHERE title=$1`, title) != 0 || f.fx.Count(t, `SELECT count(*) FROM issue_source_context WHERE anchor_comment_id=$1`, comment) != 0 {
		t.Fatal("refused comment child left partial creation")
	}
	var child struct {
		ID string `json:"id"`
	}
	f.call(t, f.token, "POST", "/api/issues", map[string]any{"title": "Allowed draft child " + uuid.NewString(), "status": "todo", "parent_issue_id": f.request.ResourceID}).Want(201).JSON(&child)
	f.fx.Cleanup(t, `DELETE FROM issue WHERE id=$1`, child.ID)
}
func TestDeliveryCompletionEnforcementRouterPrivateObservationAndABA(t *testing.T) {
	f := newDeliveryPolicyRouterFixture(t, "password", true)
	f.enablePolicy(t)
	key := uuid.NewString()
	var old deliveryEvaluationRouterResponse
	f.call(t, f.token, "POST", f.evaluationsPath(), f.evaluationBody(1), "Idempotency-Key", key).Want(200).JSON(&old)
	retained := f.evaluationRows(t)
	activateCompletionPolicy(t, f, 1)
	var stale deliveryEvaluationRouterResponse
	f.call(t, f.token, "GET", f.evaluationsPath()+"/"+key, nil).Want(200).JSON(&stale)
	if stale.Applicability.Status != "stale" || stale.Applicability.Reason != "policy_changed" || f.evaluationRows(t) != retained {
		t.Fatal("policy change rewrote historical evaluation")
	}
	newKey := uuid.NewString()
	var fresh deliveryEvaluationRouterResponse
	f.call(t, f.token, "POST", f.evaluationsPath(), f.evaluationBody(2), "Idempotency-Key", newKey).Want(200).JSON(&fresh)
	if fresh.Observation.Verdict != "pass" || fresh.Applicability.Status != "current" || fresh.UsableForCompletion == nil || *fresh.UsableForCompletion {
		t.Fatal("fresh private observation became permission")
	}
	var hint deliveryCompletionHintRouterResponse
	f.call(t, f.token, "POST", f.completionHintPath(), f.completionHintBody(t, "done", &newKey)).Want(200).JSON(&hint)
	if hint.Hint != "matching" || hint.PolicyMode != "enforce" || hint.Enforced == nil || *hint.Enforced || hint.UsableForCompletion == nil || *hint.UsableForCompletion {
		t.Fatal("matching hint authorized a future writer")
	}
	f.call(t, f.token, "PUT", "/api/issues/"+f.request.ResourceID, map[string]any{"status": "done"}).Want(428)
	other := f.fx.User(t, "Other private evaluation reader", "other-evaluation-"+uuid.NewString()+"@example.invalid")
	f.fx.Member(t, testWorkspaceID, other, "member")
	f.fx.InsertNoID(t, "user_password_credential", testutil.Cols{"user_id": other, "username": "other-" + uuid.NewString(), "password_hash": "unused", "session_version": 1, "must_change_password": false}, "user_id=$1", other)
	otherToken := controlledArtifactJWT(t, other, 1, time.Now().Add(time.Hour))
	f.call(t, otherToken, "GET", f.evaluationsPath()+"/"+newKey, nil).Want(404)
	body := completionAuthorizationRouterBody(t, f, 2, map[string]any{"kind": "human", "user_id": f.user})
	var grant completionAuthorizationRouterResponse
	f.call(t, f.token, "POST", completionAuthorizationRouterPath, body, "Idempotency-Key", uuid.NewString()).Want(201).JSON(&grant)
	f.fx.Exec(t, `UPDATE member SET role='admin' WHERE workspace_id=$1 AND user_id=$2`, testWorkspaceID, f.user)
	f.call(t, f.token, "PUT", f.policyPath(), f.policyBody(2, "off")).Want(200)
	f.call(t, f.token, "PUT", f.policyPath(), f.policyBody(3, "enforce")).Want(200)
	f.call(t, f.token, "PUT", "/api/issues/"+f.request.ResourceID, publicCompletionBody(grant, "done")).Want(409)
	if f.fx.Count(t, `SELECT count(*) FROM delivery_completion_terminal WHERE authorization_id=$1`, grant.AuthorizationID) != 0 {
		t.Fatal("policy ABA consumed old authorization")
	}
}
func TestDeliveryCompletionEnforcementRouterRevokedGrantAndRollback(t *testing.T) {
	f := newDeliveryPolicyRouterFixture(t, "password", true)
	activateCompletionPolicy(t, f, 0)
	grant := publicCompletionGrant(t, f, "done")
	f.call(t, f.token, "POST", completionAuthorizationRouterPath+"/"+grant.AuthorizationID+"/revocations", map[string]string{"reason": "withdraw exact acceptance"}, "Idempotency-Key", uuid.NewString()).Want(200)
	f.call(t, f.token, "PUT", "/api/issues/"+f.request.ResourceID, publicCompletionBody(grant, "done")).Want(409)
	f.fx.Exec(t, `UPDATE member SET role='admin' WHERE workspace_id=$1 AND user_id=$2`, testWorkspaceID, f.user)
	f.call(t, f.token, "PUT", f.policyPath(), f.policyBody(1, "off")).Want(200)
	f.call(t, f.token, "PUT", "/api/issues/"+f.request.ResourceID, map[string]any{"status": "done"}).Want(200)
	var state string
	f.fx.QueryRow(t, `SELECT kind FROM delivery_completion_terminal WHERE authorization_id=$1`, grant.AuthorizationID).Scan(&state)
	if state != "revoked" {
		t.Fatal("policy rollback rewrote historical revocation")
	}
	raw, _ := json.Marshal(f.policyBody(2, "enforce"))
	bad := strings.TrimSuffix(string(raw), "}") + `,"skip":true}`
	f.call(t, f.token, "PUT", f.policyPath(), bad).Want(400)
}
