package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/artifact"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/testutil"
)

type deliveryPolicyRouterResponse struct {
	ProjectID     string  `json:"project_id"`
	Revision      int64   `json:"revision"`
	PolicyVersion string  `json:"policy_version"`
	Mode          string  `json:"mode"`
	Stage         string  `json:"stage"`
	Reason        string  `json:"reason"`
	CreatedBy     *string `json:"created_by"`
	CreatedAt     *string `json:"created_at"`
}

type deliveryEvaluationRouterResponse struct {
	EvaluationID        string                         `json:"evaluation_id"`
	RequestKey          string                         `json:"request_key"`
	EvaluatorUserID     string                         `json:"evaluator_user_id"`
	PolicyRevision      int64                          `json:"policy_revision"`
	Stage               string                         `json:"stage"`
	Observation         deliveryEvidenceRouterResponse `json:"observation"`
	UsableForCompletion *bool                          `json:"usable_for_completion"`
	Applicability       struct {
		Status    string `json:"status"`
		Reason    string `json:"reason"`
		CheckedAt string `json:"checked_at"`
	} `json:"applicability"`
}

type deliveryPolicyRouterFixture struct {
	*deliveryEvidenceRouterFixture
	project string
}

func newDeliveryPolicyRouterFixture(t *testing.T, mode string, execute bool) *deliveryPolicyRouterFixture {
	t.Helper()
	f := &deliveryPolicyRouterFixture{deliveryEvidenceRouterFixture: newDeliveryEvidenceRouterFixture(t, mode, execute)}
	f.project = f.fx.Project(t, "Durable delivery policy")
	f.fx.Exec(t, `UPDATE issue SET project_id=$1,revision=revision+1 WHERE id=$2`, f.project, f.request.ResourceID)
	var issueRevision, projectRevision int64
	f.fx.QueryRow(t, `SELECT revision FROM issue WHERE id=$1`, f.request.ResourceID).Scan(&issueRevision)
	f.fx.QueryRow(t, `SELECT revision FROM project WHERE id=$1`, f.project).Scan(&projectRevision)
	f.body["expected_issue_revision"] = issueRevision
	f.body["expected_project_id"] = f.project
	f.body["expected_project_revision"] = projectRevision
	return f
}

func (f *deliveryPolicyRouterFixture) policyPath() string {
	return "/api/projects/" + f.project + "/delivery-policy"
}

func (f *deliveryPolicyRouterFixture) evaluationsPath() string {
	return "/api/issues/" + f.request.ResourceID + "/delivery-evidence/evaluations"
}

func (f *deliveryPolicyRouterFixture) policyBody(revision int64, mode string) map[string]any {
	return map[string]any{"expected_policy_revision": revision, "mode": mode, "policy_version": "delivery-artifact-observe-v1", "stage": "done", "reason": "Review the retained artifact evidence"}
}

func (f *deliveryPolicyRouterFixture) evaluationBody(revision int64) map[string]any {
	body := make(map[string]any, len(f.body)+1)
	for key, value := range f.body {
		body[key] = value
	}
	body["expected_policy_revision"] = revision
	return body
}

func (f *deliveryPolicyRouterFixture) enablePolicy(t *testing.T) {
	t.Helper()
	f.fx.Exec(t, `UPDATE member SET role='admin' WHERE workspace_id=$1 AND user_id=$2`, testWorkspaceID, f.user)
	f.call(t, f.token, "PUT", f.policyPath(), f.policyBody(0, "observe")).Want(200)
	f.fx.Exec(t, `UPDATE member SET role='member' WHERE workspace_id=$1 AND user_id=$2`, testWorkspaceID, f.user)
}

func (f *deliveryPolicyRouterFixture) evaluationRows(t *testing.T) string {
	t.Helper()
	var rows string
	f.fx.QueryRow(t, `SELECT COALESCE(jsonb_agg(to_jsonb(e) ORDER BY e.evaluation_id),'[]'::jsonb)::text FROM delivery_evidence_evaluation e WHERE issue_id=$1`, f.request.ResourceID).Scan(&rows)
	return rows
}

func TestDeliveryPolicyRouterManagementAndHistory(t *testing.T) {
	f := newDeliveryPolicyRouterFixture(t, "legacy", false)
	var beforeRevision int64
	f.fx.QueryRow(t, `SELECT revision FROM project WHERE id=$1`, f.project).Scan(&beforeRevision)
	var initial deliveryPolicyRouterResponse
	response := f.call(t, f.token, "GET", f.policyPath(), nil).Want(200)
	response.JSON(&initial)
	if initial.ProjectID != f.project || initial.Revision != 0 || initial.Mode != "off" || initial.PolicyVersion != "delivery-artifact-observe-v1" || initial.Stage != "done" || initial.CreatedBy != nil || initial.CreatedAt != nil || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("incorrect virtual policy: %+v", initial)
	}
	if f.fx.Count(t, `SELECT count(*) FROM project_delivery_policy WHERE project_id=$1`, f.project) != 0 {
		t.Fatal("reading the unconfigured policy created history")
	}
	for _, role := range []string{"member", "project_lead"} {
		t.Run(role, func(t *testing.T) {
			if role == "project_lead" {
				f.fx.Exec(t, `UPDATE project SET lead_type='member',lead_id=$1 WHERE id=$2`, f.user, f.project)
			}
			f.call(t, f.token, "PUT", f.policyPath(), f.policyBody(0, "observe")).Want(403)
			if f.fx.Count(t, `SELECT count(*) FROM project_delivery_policy WHERE project_id=$1`, f.project) != 0 {
				t.Fatal("nonadministrator changed project policy")
			}
		})
	}
	for i, role := range []string{"admin", "owner"} {
		t.Run(role, func(t *testing.T) {
			f.fx.Exec(t, `UPDATE member SET role=$1 WHERE workspace_id=$2 AND user_id=$3`, role, testWorkspaceID, f.user)
			var policy deliveryPolicyRouterResponse
			body := f.policyBody(int64(i), "observe")
			f.call(t, f.token, "PUT", f.policyPath(), body).Want(200).JSON(&policy)
			if policy.Revision != int64(i+1) || policy.Mode != "observe" || policy.ProjectID != f.project || policy.CreatedBy == nil || *policy.CreatedBy != f.user || policy.CreatedAt == nil || policy.Reason != body["reason"] {
				t.Fatalf("policy append lost audited identity: %+v", policy)
			}
			f.call(t, f.token, "PUT", f.policyPath(), body).Want(409)
		})
	}
	var actualRevision, histories int64
	f.fx.QueryRow(t, `SELECT revision FROM project WHERE id=$1`, f.project).Scan(&actualRevision)
	f.fx.QueryRow(t, `SELECT count(*) FROM project_delivery_policy WHERE project_id=$1 AND created_by=$2 AND mode='observe' AND revision IN (1,2)`, f.project, f.user).Scan(&histories)
	if actualRevision != beforeRevision || histories != 2 {
		t.Fatalf("policy history altered project revision or omitted same-value append: project %d/%d history %d", actualRevision, beforeRevision, histories)
	}
}

func TestDeliveryPolicyRouterRefusesMalformedAndNonhumanWrites(t *testing.T) {
	f := newDeliveryPolicyRouterFixture(t, "legacy", false)
	f.fx.Exec(t, `UPDATE member SET role='admin' WHERE workspace_id=$1 AND user_id=$2`, testWorkspaceID, f.user)
	f.call(t, f.taskToken, "PUT", f.policyPath(), f.policyBody(0, "observe"), "X-Actor-Source", "member").Want(403)
	f.call(t, f.taskToken, "GET", f.policyPath(), nil, "X-Actor-Source", "member").Want(403)
	f.call(t, f.token, "PUT", f.policyPath(), f.policyBody(0, "observe"), "X-Agent-ID", f.request.AgentID).Want(403)
	for _, tc := range []struct {
		name, field string
		value       any
	}{
		{"hint", "mode", "hint"}, {"enforce", "mode", "enforce"}, {"skip", "mode", "skip"},
		{"wrong_stage", "stage", "in_review"}, {"wrong_algorithm", "policy_version", "future-policy"},
		{"missing_reason", "reason", "   "}, {"oversized_reason", "reason", strings.Repeat("x", 1001)},
		{"negative_revision", "expected_policy_revision", -1}, {"claimed_authority", "created_by", f.user},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := f.policyBody(0, "observe")
			body[tc.field] = tc.value
			f.call(t, f.token, "PUT", f.policyPath(), body).Want(400)
		})
	}
	raw, err := json.Marshal(f.policyBody(0, "observe"))
	if err != nil {
		t.Fatal(err)
	}
	for _, malformed := range []string{string(raw[:len(raw)-1]) + `,"mode":"observe"}`, string(raw) + `{}`, strings.Replace(string(raw), `"mode"`, `"Mode"`, 1)} {
		f.call(t, f.token, "PUT", f.policyPath(), malformed).Want(400)
	}
	if f.fx.Count(t, `SELECT count(*) FROM project_delivery_policy WHERE project_id=$1`, f.project) != 0 {
		t.Fatal("malformed or machine policy operation wrote history")
	}
}

func TestDeliveryEvaluationRouterRealPackageRestartAndLegacyCompatibility(t *testing.T) {
	for _, mode := range []string{"legacy", "password"} {
		t.Run(mode, func(t *testing.T) {
			f := newDeliveryPolicyRouterFixture(t, mode, true)
			f.enablePolicy(t)
			archive, before := f.archive(t), f.snapshot(t)
			key, body := uuid.NewString(), f.evaluationBody(1)
			var saved deliveryEvaluationRouterResponse
			response := f.call(t, f.token, "POST", f.evaluationsPath(), body, "Idempotency-Key", key).Want(200)
			response.JSON(&saved)
			if saved.EvaluationID == "" || saved.RequestKey != key || saved.EvaluatorUserID != f.user || saved.PolicyRevision != 1 || saved.Stage != "done" || saved.Observation.Verdict != "pass" || saved.Applicability.Status != "current" || saved.UsableForCompletion == nil || *saved.UsableForCompletion {
				t.Fatalf("unbound durable evaluation: %+v", saved)
			}
			if saved.Observation.ExecutionID != f.approval.ExecutionID || saved.Observation.ReceiptDigest != f.approval.ReceiptDigest || saved.Observation.ProducerTaskID != f.request.TaskID || saved.Observation.ProducerAgentID != f.request.AgentID || saved.Observation.ProducerTaskGeneration != 1 || saved.Observation.SourceRevision != f.request.SourceRevision || saved.Observation.ArtifactDigest != f.body["expected_artifact_digest"] || saved.Observation.SpecDigest != f.approval.SpecDigest || saved.Observation.ProjectID == nil || *saved.Observation.ProjectID != f.project || saved.Observation.RevisionProvenance != "database_observed_pr_head_not_remote_verified" {
				t.Fatalf("durable evaluation lost actual package provenance: %+v", saved)
			}
			if _, err := time.Parse(time.RFC3339Nano, saved.Applicability.CheckedAt); err != nil {
				t.Fatal("applicability omitted its observation time")
			}
			if response.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("durable evaluation response is cacheable")
			}
			for _, private := range []string{f.output, f.request.StagingID, "candidate.txt"} {
				if strings.Contains(response.Text(), private) {
					t.Fatal("evaluation disclosed private packaging inputs")
				}
			}
			var retained deliveryEvidenceRouterResponse
			var raw []byte
			var evaluator, requestKey, evaluationID string
			f.fx.QueryRow(t, `SELECT evaluator_user_id::text,request_key::text,evaluation_id::text,observation FROM delivery_evidence_evaluation WHERE workspace_id=$1 AND issue_id=$2`, testWorkspaceID, f.request.ResourceID).Scan(&evaluator, &requestKey, &evaluationID, &raw)
			if err := json.Unmarshal(raw, &retained); err != nil {
				t.Fatal(err)
			}
			if evaluator != f.user || requestKey != key || evaluationID != saved.EvaluationID || !reflect.DeepEqual(retained, saved.Observation) {
				t.Fatal("returned evaluation does not match the committed identity and observation")
			}
			rows := f.evaluationRows(t)
			// Reconstruct the real router and authenticate a newly issued JWT. No
			// in-memory request state is available to the recovery request.
			f.router = controlledArtifactRouter(t)
			version := int64(0)
			if mode == "password" {
				version = 1
			}
			current := controlledArtifactJWT(t, f.user, version, time.Now().Add(2*time.Hour))
			for _, method := range []string{"GET", "POST"} {
				path, payload := f.evaluationsPath(), any(body)
				if method == "GET" {
					path, payload = path+"/"+key, nil
				}
				var recovered deliveryEvaluationRouterResponse
				f.call(t, current, method, path, payload, "Idempotency-Key", key).Want(200).JSON(&recovered)
				if recovered.EvaluationID != saved.EvaluationID || !reflect.DeepEqual(recovered.Observation, saved.Observation) || recovered.Applicability.Status != "current" || recovered.UsableForCompletion == nil || *recovered.UsableForCompletion {
					t.Fatalf("restart recovery replaced or authorized original evaluation: %+v", recovered)
				}
			}
			if f.evaluationRows(t) != rows || f.snapshot(t) != before || !bytes.Equal(f.archive(t), archive) {
				t.Fatal("evaluation or recovery mutated historical rows, lifecycle, queue or physical artifact")
			}
			// The original stateless observer and the installed client's ordinary
			// completion payload retain their earlier contracts.
			f.call(t, current, "POST", f.path(), f.body).Want(200)
			if f.evaluationRows(t) != rows {
				t.Fatal("old stateless observation started writing durable evaluations")
			}
			f.call(t, current, "PUT", "/api/issues/"+f.request.ResourceID, map[string]string{"status": "done"}).Want(200)
			var status string
			f.fx.QueryRow(t, `SELECT status FROM issue WHERE id=$1`, f.request.ResourceID).Scan(&status)
			if status != "done" {
				t.Fatal("observe policy blocked an existing completion writer")
			}
		})
	}
}

func TestDeliveryEvaluationRouterIdentityAndRequestConflicts(t *testing.T) {
	f := newDeliveryPolicyRouterFixture(t, "legacy", true)
	f.enablePolicy(t)
	key, body := uuid.NewString(), f.evaluationBody(1)
	f.call(t, f.token, "POST", f.evaluationsPath(), body, "Idempotency-Key", key).Want(200)
	rows := f.evaluationRows(t)
	for _, field := range []string{"expected_source_revision", "expected_artifact_digest", "expected_policy_revision"} {
		t.Run(field, func(t *testing.T) {
			changed := f.evaluationBody(1)
			switch field {
			case "expected_source_revision":
				changed[field] = strings.Repeat("b", 40)
			case "expected_artifact_digest":
				changed[field] = strings.Repeat("b", 64)
			default:
				changed[field] = 2
			}
			f.call(t, f.token, "POST", f.evaluationsPath(), changed, "Idempotency-Key", key).Want(409)
		})
	}
	f.call(t, f.taskToken, "GET", f.evaluationsPath()+"/"+key, nil, "X-Actor-Source", "member").Want(403)
	f.call(t, f.taskToken, "POST", f.evaluationsPath(), body, "Idempotency-Key", uuid.NewString(), "X-Actor-Source", "member").Want(403)
	f.call(t, f.token, "GET", f.evaluationsPath()+"/"+key, nil, "X-Agent-ID", f.request.AgentID).Want(403)
	unauthenticated := testutil.WithHeaders(testutil.JSONRequest("GET", f.evaluationsPath()+"/"+key, nil), "X-User-ID", f.user, "X-Workspace-ID", testWorkspaceID)
	testutil.Call(t, f.router.ServeHTTP, unauthenticated).Want(401)
	otherUser := f.fx.User(t, "Other policy evaluator", "policy-other-"+uuid.NewString()+"@example.test")
	f.fx.Member(t, testWorkspaceID, otherUser, "admin")
	otherToken := controlledArtifactJWT(t, otherUser, 0, time.Now().Add(time.Hour))
	f.call(t, otherToken, "GET", f.evaluationsPath()+"/"+key, nil).Want(404)
	foreign := f.fx.Workspace(t, "Other policy workspace", "policy-other-"+uuid.NewString())
	f.fx.Member(t, foreign, f.user, "owner")
	f.call(t, f.token, "GET", f.evaluationsPath()+"/"+key, nil, "X-Workspace-ID", foreign).Want(404)
	f.call(t, f.token, "GET", f.policyPath(), nil, "X-Workspace-ID", foreign).Want(404)
	otherIssue := f.fx.Issue(t, "Other policy issue", testutil.Cols{"project_id": f.project})
	otherPath := "/api/issues/" + otherIssue + "/delivery-evidence/evaluations"
	f.call(t, f.token, "GET", otherPath+"/"+key, nil).Want(409)
	f.call(t, f.token, "POST", otherPath, body, "Idempotency-Key", key).Want(409)
	if f.evaluationRows(t) != rows || f.fx.Count(t, `SELECT count(*) FROM delivery_evidence_evaluation WHERE issue_id=$1`, otherIssue) != 0 {
		t.Fatal("rejected identity or conflicting request changed durable history")
	}
}

func TestDeliveryEvaluationRouterAdmissionAndMalformedContract(t *testing.T) {
	f := newDeliveryPolicyRouterFixture(t, "legacy", true)
	body := f.evaluationBody(0)
	f.call(t, f.token, "POST", f.evaluationsPath(), body).Want(409)
	f.enablePolicy(t)
	f.call(t, f.token, "POST", f.evaluationsPath(), body).Want(409)
	body = f.evaluationBody(1)
	for _, key := range []string{"", "not-a-uuid", "00000000-0000-0000-0000-000000000000", "ABCDEFAB-CDEF-4ABC-8DEF-ABCDEFABCDEF"} {
		f.call(t, f.token, "POST", f.evaluationsPath(), body, "Idempotency-Key", key).Want(400)
	}
	missing := testutil.WithHeaders(testutil.JSONRequest("POST", f.evaluationsPath(), body), "Authorization", "Bearer "+f.token, "X-Workspace-ID", testWorkspaceID)
	testutil.Call(t, f.router.ServeHTTP, missing).Want(400)
	duplicate := testutil.WithHeaders(testutil.JSONRequest("POST", f.evaluationsPath(), body), "Authorization", "Bearer "+f.token, "X-Workspace-ID", testWorkspaceID, "Idempotency-Key", uuid.NewString())
	duplicate.Header.Add("Idempotency-Key", uuid.NewString())
	testutil.Call(t, f.router.ServeHTTP, duplicate).Want(400)
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	for _, malformed := range []string{
		string(raw[:len(raw)-1]) + `,"verdict":"pass"}`,
		string(raw[:len(raw)-1]) + `,"skip":true}`,
		string(raw[:len(raw)-1]) + `,"expected_policy_revision":1}`,
		strings.Replace(string(raw), `"expected_policy_revision"`, `"Expected_Policy_Revision"`, 1),
		strings.Replace(string(raw), `"expected_policy_revision":1`, `"expected_policy_revision":null`, 1),
		string(raw) + `{}`,
	} {
		f.call(t, f.token, "POST", f.evaluationsPath(), malformed).Want(400)
	}
	if f.fx.Count(t, `SELECT count(*) FROM delivery_evidence_evaluation WHERE issue_id=$1`, f.request.ResourceID) != 0 {
		t.Fatal("invalid admission created an evaluation")
	}
	f.fx.Exec(t, `UPDATE issue SET project_id=NULL,revision=revision+1 WHERE id=$1`, f.request.ResourceID)
	f.call(t, f.token, "POST", f.evaluationsPath(), body).Want(409)
	if f.fx.Count(t, `SELECT count(*) FROM delivery_evidence_evaluation WHERE issue_id=$1`, f.request.ResourceID) != 0 {
		t.Fatal("projectless issue created an evaluation")
	}
}

func TestDeliveryEvaluationRouterCurrentApplicabilityAndPolicyRecovery(t *testing.T) {
	for _, change := range []string{"policy_revision", "policy_off", "policy_aba", "issue_revision", "project_revision", "project_move", "project_detach", "head_changed", "head_missing"} {
		t.Run(change, func(t *testing.T) {
			f := newDeliveryPolicyRouterFixture(t, "legacy", true)
			f.enablePolicy(t)
			key, body := uuid.NewString(), f.evaluationBody(1)
			var saved deliveryEvaluationRouterResponse
			f.call(t, f.token, "POST", f.evaluationsPath(), body, "Idempotency-Key", key).Want(200).JSON(&saved)
			if saved.Observation.Verdict != "pass" || saved.Applicability.Status != "current" {
				t.Fatalf("invalid passing control: %+v", saved)
			}
			rows := f.evaluationRows(t)
			switch change {
			case "policy_revision", "policy_off", "policy_aba":
				f.fx.Exec(t, `UPDATE member SET role='admin' WHERE workspace_id=$1 AND user_id=$2`, testWorkspaceID, f.user)
				mode := "observe"
				if change != "policy_revision" {
					mode = "off"
				}
				f.call(t, f.token, "PUT", f.policyPath(), f.policyBody(1, mode)).Want(200)
				if change == "policy_aba" {
					f.call(t, f.token, "PUT", f.policyPath(), f.policyBody(2, "observe")).Want(200)
				}
			case "issue_revision":
				f.fx.Exec(t, `UPDATE issue SET revision=revision+1 WHERE id=$1`, f.request.ResourceID)
			case "project_revision":
				f.fx.Exec(t, `UPDATE project SET revision=revision+1 WHERE id=$1`, f.project)
			case "project_move":
				other := f.fx.Project(t, "Moved durable evaluation")
				f.fx.Exec(t, `UPDATE issue SET project_id=$1,revision=revision+1 WHERE id=$2`, other, f.request.ResourceID)
			case "project_detach":
				f.fx.Exec(t, `UPDATE issue SET project_id=NULL,revision=revision+1 WHERE id=$1`, f.request.ResourceID)
			case "head_changed":
				f.fx.Exec(t, `UPDATE github_pull_request SET head_sha=$1 WHERE id=$2`, strings.Repeat("b", 40), f.pr)
			case "head_missing":
				f.fx.Exec(t, `UPDATE issue_pull_request SET reference_only=true WHERE issue_id=$1 AND pull_request_id=$2`, f.request.ResourceID, f.pr)
			}
			want := "stale"
			if change == "head_missing" {
				want = "unknown"
			}
			for _, method := range []string{"GET", "POST"} {
				path, payload := f.evaluationsPath(), any(body)
				if method == "GET" {
					path, payload = path+"/"+key, nil
				}
				var recovered deliveryEvaluationRouterResponse
				f.call(t, f.token, method, path, payload, "Idempotency-Key", key).Want(200).JSON(&recovered)
				if recovered.EvaluationID != saved.EvaluationID || !reflect.DeepEqual(recovered.Observation, saved.Observation) || recovered.Applicability.Status != want || recovered.UsableForCompletion == nil || *recovered.UsableForCompletion {
					t.Fatalf("%s %s did not retain original and invalidate applicability: %+v", change, method, recovered)
				}
			}
			if f.evaluationRows(t) != rows {
				t.Fatal("current applicability rewrote historical evaluation")
			}
		})
	}
}

func TestDeliveryEvaluationRouterNonpassIsNeverPromoted(t *testing.T) {
	for _, original := range []string{"unknown", "hold"} {
		t.Run(original, func(t *testing.T) {
			f := newDeliveryPolicyRouterFixture(t, "legacy", true)
			f.enablePolicy(t)
			if original == "unknown" {
				f.fx.Exec(t, `UPDATE issue_pull_request SET reference_only=true WHERE issue_id=$1 AND pull_request_id=$2`, f.request.ResourceID, f.pr)
			} else {
				f.fx.Exec(t, `UPDATE github_pull_request SET head_sha=$1 WHERE id=$2`, strings.Repeat("b", 40), f.pr)
			}
			key, body := uuid.NewString(), f.evaluationBody(1)
			var saved deliveryEvaluationRouterResponse
			f.call(t, f.token, "POST", f.evaluationsPath(), body, "Idempotency-Key", key).Want(200).JSON(&saved)
			if saved.Observation.Verdict != original {
				t.Fatalf("missing/mismatched head produced wrong saved verdict: %+v", saved)
			}
			rows := f.evaluationRows(t)
			f.fx.Exec(t, `UPDATE issue_pull_request SET reference_only=false WHERE issue_id=$1 AND pull_request_id=$2`, f.request.ResourceID, f.pr)
			f.fx.Exec(t, `UPDATE github_pull_request SET head_sha=$1 WHERE id=$2`, f.request.SourceRevision, f.pr)
			var recovered deliveryEvaluationRouterResponse
			f.call(t, f.token, "POST", f.evaluationsPath(), body, "Idempotency-Key", key).Want(200).JSON(&recovered)
			if recovered.EvaluationID != saved.EvaluationID || !reflect.DeepEqual(recovered.Observation, saved.Observation) || recovered.Applicability.Status == "current" || recovered.UsableForCompletion == nil || *recovered.UsableForCompletion || f.evaluationRows(t) != rows {
				t.Fatalf("same-key recovery promoted or rewrote original %s: %+v", original, recovered)
			}
			var fresh deliveryEvaluationRouterResponse
			f.call(t, f.token, "POST", f.evaluationsPath(), body, "Idempotency-Key", uuid.NewString()).Want(200).JSON(&fresh)
			if fresh.EvaluationID == saved.EvaluationID || fresh.Observation.Verdict != "pass" || fresh.Applicability.Status != "current" {
				t.Fatalf("new deliberate observation did not evaluate repaired evidence: %+v", fresh)
			}
		})
	}
}

func TestDeliveryPolicyRouterRoleRevocationAfterAuthentication(t *testing.T) {
	f := newDeliveryPolicyRouterFixture(t, "legacy", false)
	f.fx.Exec(t, `UPDATE member SET role='admin' WHERE workspace_id=$1 AND user_id=$2`, testWorkspaceID, f.user)
	req := testutil.WithHeaders(testutil.JSONRequest("PUT", f.policyPath(), f.policyBody(0, "observe")), "Authorization", "Bearer "+f.token, "X-Workspace-ID", testWorkspaceID)
	revoked := false
	req.Body = &artifactRevokeOnRead{ReadCloser: req.Body, revoke: func() {
		f.fx.Exec(t, `UPDATE member SET role='member' WHERE workspace_id=$1 AND user_id=$2`, testWorkspaceID, f.user)
		revoked = true
	}}
	testutil.Call(t, f.router.ServeHTTP, req).Want(403)
	if !revoked || f.fx.Count(t, `SELECT count(*) FROM project_delivery_policy WHERE project_id=$1`, f.project) != 0 {
		t.Fatal("policy PUT used pre-revocation authority or appended a refused revision")
	}
}

func TestDeliveryEvaluationRouterStaleVersionStillChecksApproverAuthority(t *testing.T) {
	f := newDeliveryPolicyRouterFixture(t, "legacy", false)
	approver := f.fx.User(t, "Independent policy approver", "policy-approver-"+uuid.NewString()+"@example.test")
	f.fx.Member(t, testWorkspaceID, approver, "admin")
	f.fx.Exec(t, `UPDATE agent SET permission_mode='public_to' WHERE id=$1`, f.request.AgentID)
	f.fx.Insert(t, "agent_invocation_target", testutil.Cols{"agent_id": f.request.AgentID, "target_type": "member", "target_id": approver})
	approverToken := controlledArtifactJWT(t, approver, 0, time.Now().Add(time.Hour))
	base := "/api/agent-approvals/" + f.approval.Approval.ID
	f.call(t, approverToken, "POST", base+"/decision", map[string]string{"decision": "approve", "expected_spec_digest": f.approval.SpecDigest}).Want(200)
	f.call(t, f.taskToken, "POST", base+"/artifact-package/execution", map[string]string{"expected_spec_digest": f.approval.SpecDigest}).Want(200).JSON(&f.approval)
	var digest string
	f.fx.QueryRow(t, `SELECT output_artifact_digest FROM agent_artifact_execution WHERE approval_id=$1`, f.approval.Approval.ID).Scan(&digest)
	f.body["expected_artifact_digest"] = digest
	f.enablePolicy(t)
	key, body := uuid.NewString(), f.evaluationBody(1)
	var saved deliveryEvaluationRouterResponse
	f.call(t, f.token, "POST", f.evaluationsPath(), body, "Idempotency-Key", key).Want(200).JSON(&saved)
	if saved.Observation.Verdict != "pass" || f.approval.Approval.DecidedBy == nil || *f.approval.Approval.DecidedBy != approver {
		t.Fatal("control did not bind distinct authenticated approver")
	}
	rows := f.evaluationRows(t)
	f.fx.Exec(t, `UPDATE issue SET revision=revision+1 WHERE id=$1`, f.request.ResourceID)
	f.fx.Exec(t, `DELETE FROM member WHERE workspace_id=$1 AND user_id=$2`, testWorkspaceID, approver)
	f.call(t, f.token, "GET", f.evaluationsPath()+"/"+key, nil).Want(403)
	f.call(t, f.token, "POST", f.evaluationsPath(), body, "Idempotency-Key", key).Want(403)
	if f.evaluationRows(t) != rows {
		t.Fatal("revocation altered the retained original evaluation")
	}
}

func TestDeliveryEvaluationRouterCredentialRevocationAfterAuthentication(t *testing.T) {
	for _, kind := range []string{"password", "pat", "membership"} {
		t.Run(kind, func(t *testing.T) {
			mode := "legacy"
			if kind == "password" {
				mode = "password"
			}
			f := newDeliveryPolicyRouterFixture(t, mode, true)
			f.enablePolicy(t)
			key, body := uuid.NewString(), f.evaluationBody(1)
			token := f.token
			patID := ""
			revoke := func() {
				f.fx.Exec(t, `DELETE FROM member WHERE workspace_id=$1 AND user_id=$2`, testWorkspaceID, f.user)
			}
			if kind == "password" {
				revoke = func() {
					f.fx.Exec(t, `UPDATE user_password_credential SET session_version=session_version+1 WHERE user_id=$1`, f.user)
				}
			}
			if kind == "pat" {
				var err error
				token, err = auth.GeneratePATToken()
				if err != nil {
					t.Fatal(err)
				}
				patID = f.fx.Insert(t, "personal_access_token", testutil.Cols{"user_id": f.user, "name": "Durable evaluation PAT", "token_hash": auth.HashToken(token), "token_prefix": token[:8], "expires_at": time.Now().Add(time.Hour)})
				revoke = func() { f.fx.Exec(t, `UPDATE personal_access_token SET revoked=true WHERE id=$1`, patID) }
			}
			control := testutil.WithHeaders(testutil.JSONRequest("POST", f.evaluationsPath(), body), "Authorization", "Bearer "+token, "X-Workspace-ID", testWorkspaceID, "Idempotency-Key", key)
			if patID != "" {
				control.Body = &artifactRevokeOnRead{ReadCloser: control.Body, revoke: func() {
					ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
					defer cancel()
					ticker := time.NewTicker(time.Millisecond)
					defer ticker.Stop()
					for {
						var used bool
						if err := testPool.QueryRow(ctx, `SELECT last_used_at IS NOT NULL FROM personal_access_token WHERE id=$1`, patID).Scan(&used); err != nil {
							t.Fatalf("observe real PAT authentication commit: %v", err)
						}
						if used {
							return
						}
						select {
						case <-ctx.Done():
							t.Fatal("real PAT authentication last-used write did not commit")
						case <-ticker.C:
						}
					}
				}}
			}
			var saved deliveryEvaluationRouterResponse
			testutil.Call(t, f.router.ServeHTTP, control).Want(200).JSON(&saved)
			if saved.Observation.Verdict != "pass" || saved.Applicability.Status != "current" {
				t.Fatalf("valid authenticated positive control did not persist pass: %+v", saved)
			}
			rows := f.evaluationRows(t)
			req := testutil.WithHeaders(testutil.JSONRequest("POST", f.evaluationsPath(), body), "Authorization", "Bearer "+token, "X-Workspace-ID", testWorkspaceID, "Idempotency-Key", key)
			revoked := false
			req.Body = &artifactRevokeOnRead{ReadCloser: req.Body, revoke: func() { revoke(); revoked = true }}
			want := http.StatusForbidden
			if kind == "pat" {
				want = http.StatusNotFound
			}
			testutil.Call(t, f.router.ServeHTTP, req).Want(want)
			if !revoked || f.evaluationRows(t) != rows {
				t.Fatalf("%s revocation bypassed the real-auth transaction fence or changed history", kind)
			}
		})
	}
}

func TestDeliveryEvaluationRouterProjectDeletionRetainsHistory(t *testing.T) {
	f := newDeliveryPolicyRouterFixture(t, "legacy", true)
	f.enablePolicy(t)
	key := uuid.NewString()
	var saved deliveryEvaluationRouterResponse
	f.call(t, f.token, "POST", f.evaluationsPath(), f.evaluationBody(1), "Idempotency-Key", key).Want(200).JSON(&saved)
	rows := f.evaluationRows(t)
	f.fx.Exec(t, `UPDATE member SET role='admin' WHERE workspace_id=$1 AND user_id=$2`, testWorkspaceID, f.user)
	var revision int64
	f.fx.QueryRow(t, `SELECT revision FROM project WHERE id=$1`, f.project).Scan(&revision)
	f.call(t, f.token, "DELETE", "/api/projects/"+f.project, map[string]int64{"expected_revision": revision}).Want(204)
	f.call(t, f.token, "GET", f.policyPath(), nil).Want(404)
	var recovered deliveryEvaluationRouterResponse
	f.call(t, f.token, "GET", f.evaluationsPath()+"/"+key, nil).Want(200).JSON(&recovered)
	if recovered.EvaluationID != saved.EvaluationID || !reflect.DeepEqual(recovered.Observation, saved.Observation) || recovered.Applicability.Status != "stale" || f.evaluationRows(t) != rows || f.fx.Count(t, `SELECT count(*) FROM project_delivery_policy WHERE project_id=$1`, f.project) != 1 {
		t.Fatal("project deletion erased history or permitted stale evaluation reuse")
	}
	var project *string
	f.fx.QueryRow(t, `SELECT project_id::text FROM issue WHERE id=$1`, f.request.ResourceID).Scan(&project)
	if project != nil {
		t.Fatal("project deletion did not detach its surviving issue")
	}
}

func TestDeliveryEvaluationRouterConcurrentKeyHasOneCommittedObservation(t *testing.T) {
	f := newDeliveryPolicyRouterFixture(t, "legacy", true)
	f.enablePolicy(t)
	key, body := uuid.NewString(), f.evaluationBody(1)
	before, archive := f.snapshot(t), f.archive(t)
	const callers = 4
	responses := make([]*httptest.ResponseRecorder, callers)
	start := make(chan struct{})
	var workers sync.WaitGroup
	for i := range callers {
		workers.Add(1)
		go func(i int) {
			defer workers.Done()
			request := testutil.WithHeaders(testutil.JSONRequest("POST", f.evaluationsPath(), body), "Authorization", "Bearer "+f.token, "X-Workspace-ID", testWorkspaceID, "Idempotency-Key", key)
			responses[i] = httptest.NewRecorder()
			<-start
			f.router.ServeHTTP(responses[i], request)
		}(i)
	}
	close(start)
	workers.Wait()
	passed := 0
	var committed deliveryEvaluationRouterResponse
	for i, response := range responses {
		if response.Code == 409 {
			if !strings.Contains(response.Body.String(), "artifact_authority_busy") {
				t.Fatalf("concurrent caller %d had an unexpected conflict: %s", i, response.Body.String())
			}
			continue
		}
		if response.Code != 200 {
			t.Fatalf("concurrent caller %d returned %d: %s", i, response.Code, response.Body.String())
		}
		var current deliveryEvaluationRouterResponse
		if err := json.Unmarshal(response.Body.Bytes(), &current); err != nil {
			t.Fatal(err)
		}
		if current.Observation.Verdict != "pass" || current.Applicability.Status != "current" || current.UsableForCompletion == nil || *current.UsableForCompletion {
			t.Fatalf("concurrent caller %d returned an invalid observation: %+v", i, current)
		}
		if passed > 0 && (current.EvaluationID != committed.EvaluationID || !reflect.DeepEqual(current.Observation, committed.Observation)) {
			t.Fatal("concurrent successful responses have distinct committed observations")
		}
		committed = current
		passed++
	}
	if passed == 0 || f.fx.Count(t, `SELECT count(*) FROM delivery_evidence_evaluation WHERE issue_id=$1 AND request_key=$2`, f.request.ResourceID, key) != 1 {
		t.Fatal("concurrent requests did not commit exactly one durable observation")
	}
	var recovered deliveryEvaluationRouterResponse
	f.call(t, f.token, "GET", f.evaluationsPath()+"/"+key, nil).Want(200).JSON(&recovered)
	if recovered.EvaluationID != committed.EvaluationID || !reflect.DeepEqual(recovered.Observation, committed.Observation) || f.snapshot(t) != before || !bytes.Equal(f.archive(t), archive) {
		t.Fatal("concurrent evaluation altered delivery or recovery identity")
	}
}

func TestDeliveryEvaluationRouterOriginalPassExpiresWithoutRenewal(t *testing.T) {
	f := newDeliveryPolicyRouterFixture(t, "legacy", false)
	f.enablePolicy(t)
	// Stage an equivalent fixture specification near its real 15-minute
	// deadline before approval or execution. The public decision and packager
	// then consume that exact frozen specification and produce the real receipt.
	var raw []byte
	f.fx.QueryRow(t, `SELECT canonical_spec FROM agent_artifact_execution WHERE approval_id=$1`, f.approval.Approval.ID).Scan(&raw)
	var spec artifact.Spec
	if err := json.Unmarshal(raw, &spec); err != nil {
		t.Fatal(err)
	}
	var now time.Time
	f.fx.QueryRow(t, `SELECT clock_timestamp()`).Scan(&now)
	expires := now.UTC().Add(6 * time.Second).Truncate(time.Microsecond)
	created := expires.Add(-15 * time.Minute)
	spec.CreatedAt = created.Format("2006-01-02T15:04:05.000000Z")
	spec.ExpiresAt = expires.Format("2006-01-02T15:04:05.000000Z")
	canonical, digest, err := artifact.CanonicalSpec(spec)
	if err != nil {
		t.Fatal(err)
	}
	f.fx.Exec(t, `UPDATE agent_artifact_execution SET created_at=$1,expires_at=$2,canonical_spec=$3,spec_digest=$4 WHERE approval_id=$5`, created, expires, canonical, digest, f.approval.Approval.ID)
	f.approval.SpecDigest = digest
	f.body["expected_spec_digest"] = digest
	base := "/api/agent-approvals/" + f.approval.Approval.ID
	f.call(t, f.token, "POST", base+"/decision", map[string]string{"decision": "approve", "expected_spec_digest": digest}).Want(200)
	f.call(t, f.taskToken, "POST", base+"/artifact-package/execution", map[string]string{"expected_spec_digest": digest}).Want(200).JSON(&f.approval)
	var artifactDigest string
	f.fx.QueryRow(t, `SELECT output_artifact_digest FROM agent_artifact_execution WHERE approval_id=$1`, f.approval.Approval.ID).Scan(&artifactDigest)
	f.body["expected_artifact_digest"] = artifactDigest
	archive := f.archive(t)
	key, body := uuid.NewString(), f.evaluationBody(1)
	var saved deliveryEvaluationRouterResponse
	f.call(t, f.token, "POST", f.evaluationsPath(), body, "Idempotency-Key", key).Want(200).JSON(&saved)
	if saved.Observation.Verdict != "pass" || saved.Applicability.Status != "current" {
		t.Fatalf("near-expiry positive control was not genuinely valid: %+v", saved)
	}
	rows := f.evaluationRows(t)
	deadline, err := time.Parse(time.RFC3339Nano, saved.Observation.ValidUntil)
	if err != nil || !deadline.Equal(expires) {
		t.Fatalf("observation did not preserve exact admission horizon: %s", saved.Observation.ValidUntil)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		var expired bool
		if err := testPool.QueryRow(ctx, `SELECT clock_timestamp() >= $1`, expires).Scan(&expired); err != nil {
			t.Fatal(err)
		}
		if expired {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal("fixture evidence deadline was not reached")
		case <-ticker.C:
		}
	}
	for _, method := range []string{"GET", "POST"} {
		path, payload := f.evaluationsPath(), any(body)
		if method == "GET" {
			path, payload = path+"/"+key, nil
		}
		var recovered deliveryEvaluationRouterResponse
		f.call(t, f.token, method, path, payload, "Idempotency-Key", key).Want(200).JSON(&recovered)
		if recovered.EvaluationID != saved.EvaluationID || !reflect.DeepEqual(recovered.Observation, saved.Observation) || recovered.Applicability.Status != "expired" || recovered.UsableForCompletion == nil || *recovered.UsableForCompletion {
			t.Fatalf("%s renewed or reused the expired original observation: %+v", method, recovered)
		}
	}
	if f.evaluationRows(t) != rows || !bytes.Equal(f.archive(t), archive) {
		t.Fatal("expiry recovery rewrote history or reran packaging")
	}
}

func TestDeliveryEvaluationRouterMissingRetainedBindingRefusesHistory(t *testing.T) {
	f := newDeliveryPolicyRouterFixture(t, "legacy", true)
	f.enablePolicy(t)
	key, body := uuid.NewString(), f.evaluationBody(1)
	var saved deliveryEvaluationRouterResponse
	f.call(t, f.token, "POST", f.evaluationsPath(), body, "Idempotency-Key", key).Want(200).JSON(&saved)
	if saved.Observation.Verdict != "pass" || saved.Observation.ExecutionID == "" {
		t.Fatal("control did not retain an actual execution binding")
	}
	rows := f.evaluationRows(t)
	f.fx.Exec(t, `DELETE FROM agent_artifact_execution WHERE approval_id=$1`, f.approval.Approval.ID)
	for _, method := range []string{"GET", "POST"} {
		path, payload := f.evaluationsPath(), any(body)
		if method == "GET" {
			path, payload = path+"/"+key, nil
		}
		response := f.call(t, f.token, method, path, payload, "Idempotency-Key", key).Want(409)
		if !strings.Contains(response.Text(), "delivery_retained_binding_missing") || strings.Contains(response.Text(), saved.EvaluationID) || strings.Contains(response.Text(), "observation") {
			t.Fatal("missing original authority binding exposed retained evaluation content")
		}
	}
	if f.evaluationRows(t) != rows {
		t.Fatal("missing binding refusal erased original evaluation")
	}
}
