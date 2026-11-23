package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/testutil"
)

const completionAuthorizationRouterPath = "/api/delivery-completion-authorizations"

type completionAuthorizationRouterResponse struct {
	AuthorizationID string    `json:"authorization_id"`
	OperationID     string    `json:"operation_id"`
	IssueID         string    `json:"issue_id"`
	IssueRevision   int64     `json:"issue_revision"`
	TargetStatus    string    `json:"target_status"`
	ContractVersion string    `json:"contract_version"`
	CreatedAt       time.Time `json:"created_at"`
	ExpiresAt       time.Time `json:"expires_at"`
	State           string    `json:"state"`
	Applicability   string    `json:"applicability"`
	Replayed        bool      `json:"replayed"`
}

// This P1 fixture installs a task-owned enforce row before public policy
// activation is available. It proves the authenticated authorization API, not
// project-policy rollout or completion by any lifecycle writer. Artifact
// creation, approval, execution and archive verification use the existing real
// router/packager chain; no successful artifact or authorization is seeded.
func installCompletionAuthorizationRouterPolicy(t *testing.T, f *deliveryPolicyRouterFixture, revision int64) {
	t.Helper()
	f.fx.InsertNoID(t, "project_delivery_policy", testutil.Cols{
		"workspace_id": testWorkspaceID, "project_id": f.project,
		"revision": revision, "policy_version": "delivery-artifact-observe-v1",
		"mode": "enforce", "stage": "done", "created_by": f.user,
		"reason": "Internal authorization router fixture; public activation is separate",
	}, "workspace_id=$1 AND project_id=$2 AND revision=$3", testWorkspaceID, f.project, revision)
}

func completionAuthorizationRouterBody(t *testing.T, f *deliveryPolicyRouterFixture, policyRevision int64, consumer map[string]any) map[string]any {
	t.Helper()
	var status string
	var expires time.Time
	f.fx.QueryRow(t, `SELECT status FROM issue WHERE id=$1`, f.request.ResourceID).Scan(&status)
	f.fx.QueryRow(t, `SELECT clock_timestamp()+interval '1 minute'`).Scan(&expires)
	return map[string]any{
		"contract_version":    "delivery-completion-enforcement-v1",
		"source_workspace_id": testWorkspaceID, "operation_id": uuid.NewString(),
		"target": map[string]any{
			"kind": "ordinary", "issue_id": f.request.ResourceID,
			"expected_issue_revision": f.body["expected_issue_revision"],
			"expected_status":         status, "target_status": "done", "target_position": nil,
		},
		"project": map[string]any{
			"id": f.project, "expected_revision": f.body["expected_project_revision"],
			"expected_policy_revision": policyRevision,
		},
		"evidence": map[string]any{
			"issue_id": f.request.ResourceID, "expected_issue_revision": f.body["expected_issue_revision"],
			"approval_id": f.approval.Approval.ID, "expected_source_revision": f.request.SourceRevision,
			"expected_spec_digest": f.approval.SpecDigest, "expected_artifact_digest": f.body["expected_artifact_digest"],
		},
		"consumer":   consumer,
		"expires_at": expires.UTC().Truncate(time.Microsecond).Format(time.RFC3339Nano),
		"reason":     "Human acceptance of this exact retained candidate",
	}
}

func completionAuthorizationRouterRows(t *testing.T, f *deliveryPolicyRouterFixture) string {
	t.Helper()
	var rows string
	f.fx.QueryRow(t, `SELECT jsonb_build_object(
		'authorizations',(SELECT COALESCE(jsonb_agg(to_jsonb(a) ORDER BY a.id),'[]'::jsonb) FROM delivery_completion_authorization a WHERE a.issue_id=$1),
		'terminals',(SELECT COALESCE(jsonb_agg(to_jsonb(t) ORDER BY t.authorization_id),'[]'::jsonb) FROM delivery_completion_terminal t JOIN delivery_completion_authorization a ON a.id=t.authorization_id WHERE a.issue_id=$1)
	)::text`, f.request.ResourceID).Scan(&rows)
	return rows
}

func TestDeliveryCompletionAuthorizationRouterRealArtifactAndRestart(t *testing.T) {
	for _, mode := range []string{"legacy", "password"} {
		t.Run(mode, func(t *testing.T) {
			f := newDeliveryPolicyRouterFixture(t, mode, true)
			installCompletionAuthorizationRouterPolicy(t, f, 1)
			archive, before, evaluations := f.archive(t), f.snapshot(t), f.evaluationRows(t)
			var tasksBefore string
			f.fx.QueryRow(t, `SELECT COALESCE(jsonb_agg(to_jsonb(t) ORDER BY t.id),'[]'::jsonb)::text FROM agent_task_queue t WHERE t.issue_id=$1`, f.request.ResourceID).Scan(&tasksBefore)
			body := completionAuthorizationRouterBody(t, f, 1, map[string]any{"kind": "human", "user_id": f.user})
			key := uuid.NewString()
			var saved completionAuthorizationRouterResponse
			response := f.call(t, f.token, "POST", completionAuthorizationRouterPath, body, "Idempotency-Key", key).Want(201).JSON(&saved)
			if saved.AuthorizationID == "" || saved.OperationID != body["operation_id"] || saved.IssueID != f.request.ResourceID || saved.IssueRevision != f.body["expected_issue_revision"] || saved.TargetStatus != "done" || saved.ContractVersion != "delivery-completion-enforcement-v1" || saved.State != "issued" || saved.Applicability != "not_checked" || saved.Replayed || !saved.ExpiresAt.After(saved.CreatedAt) {
				t.Fatalf("authorization did not preserve the accepted target: %+v", saved)
			}
			if response.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("authorization response is cacheable")
			}
			var issuer, operation, approval, execution, artifactDigest, specDigest, receiptDigest, persistedState string
			var persistedExpiry time.Time
			f.fx.QueryRow(t, `SELECT issuer_user_id::text,operation_id::text,approval_id::text,execution_id::text,artifact_digest,spec_digest,receipt_digest,observation->>'verdict',expires_at FROM delivery_completion_authorization WHERE id=$1`, saved.AuthorizationID).Scan(&issuer, &operation, &approval, &execution, &artifactDigest, &specDigest, &receiptDigest, &persistedState, &persistedExpiry)
			if issuer != f.user || operation != saved.OperationID || approval != f.approval.Approval.ID || execution != f.approval.ExecutionID || artifactDigest != f.body["expected_artifact_digest"] || specDigest != f.approval.SpecDigest || receiptDigest != f.approval.ReceiptDigest || persistedState != "pass" || !persistedExpiry.Equal(saved.ExpiresAt) {
				t.Fatal("authorization does not bind the actual committed artifact and issuer")
			}
			rows := completionAuthorizationRouterRows(t, f)
			f.router = controlledArtifactRouter(t)
			version := int64(0)
			if mode == "password" {
				version = 1
			}
			current := controlledArtifactJWT(t, f.user, version, time.Now().Add(2*time.Hour))
			for _, request := range []struct {
				method, path string
				body         any
			}{
				{"GET", completionAuthorizationRouterPath + "/" + saved.AuthorizationID, nil},
				{"GET", completionAuthorizationRouterPath + "/by-request-key/" + key + "?workspace_id=" + testWorkspaceID, nil},
				{"POST", completionAuthorizationRouterPath, body},
			} {
				var restored completionAuthorizationRouterResponse
				f.call(t, current, request.method, request.path, request.body, "Idempotency-Key", key).Want(200).JSON(&restored)
				if restored.AuthorizationID != saved.AuthorizationID || restored.OperationID != saved.OperationID || !restored.ExpiresAt.Equal(saved.ExpiresAt) || restored.State != "issued" || request.method == "POST" && !restored.Replayed {
					t.Fatal("restart or renewed credential replaced the original authorization")
				}
			}
			body["reason"] = "Different acceptance intent"
			f.call(t, current, "POST", completionAuthorizationRouterPath, body, "Idempotency-Key", key).Want(409)
			var tasksAfter string
			f.fx.QueryRow(t, `SELECT COALESCE(jsonb_agg(to_jsonb(t) ORDER BY t.id),'[]'::jsonb)::text FROM agent_task_queue t WHERE t.issue_id=$1`, f.request.ResourceID).Scan(&tasksAfter)
			if tasksAfter != tasksBefore {
				t.Fatal("authorization or recovery mutated an existing producer task")
			}
			if completionAuthorizationRouterRows(t, f) != rows || f.snapshot(t) != before || f.evaluationRows(t) != evaluations || !bytes.Equal(f.archive(t), archive) {
				t.Fatal("authorization or recovery changed the lifecycle, private evaluation, artifact or original grant")
			}
			if f.fx.Count(t, `SELECT count(*) FROM delivery_completion_authorization WHERE issue_id=$1`, f.request.ResourceID) != 1 || f.fx.Count(t, `SELECT count(*) FROM delivery_completion_terminal t JOIN delivery_completion_authorization a ON a.id=t.authorization_id WHERE a.issue_id=$1`, f.request.ResourceID) != 0 {
				t.Fatal("issuance duplicated or prematurely consumed the authorization")
			}
		})
	}
}

func TestDeliveryCompletionAuthorizationRouterRejectsMachineAndSpoofedIdentity(t *testing.T) {
	f := newDeliveryPolicyRouterFixture(t, "legacy", true)
	installCompletionAuthorizationRouterPolicy(t, f, 1)
	body := completionAuthorizationRouterBody(t, f, 1, map[string]any{"kind": "human", "user_id": f.user})
	before := f.snapshot(t)
	missing := testutil.WithHeaders(testutil.JSONRequest("POST", completionAuthorizationRouterPath, body), "X-User-ID", f.user, "X-Workspace-ID", testWorkspaceID, "Idempotency-Key", uuid.NewString())
	testutil.Call(t, f.router.ServeHTTP, missing).Want(401)
	f.call(t, f.taskToken, "POST", completionAuthorizationRouterPath, body, "X-Actor-Source", "member").Want(403)
	f.call(t, f.token, "POST", completionAuthorizationRouterPath, body, "X-Agent-ID", f.request.AgentID).Want(403)
	f.call(t, f.token, "POST", completionAuthorizationRouterPath, body, "X-Task-ID", f.request.TaskID).Want(403)
	if f.fx.Count(t, `SELECT count(*) FROM delivery_completion_authorization WHERE issue_id=$1`, f.request.ResourceID) != 0 || f.snapshot(t) != before {
		t.Fatal("unauthenticated, machine or forged identity wrote an authorization or lifecycle")
	}
}

func TestDeliveryCompletionAuthorizationRouterDesignatedHumanKeepsEvaluationPrivate(t *testing.T) {
	f := newDeliveryPolicyRouterFixture(t, "legacy", true)
	f.enablePolicy(t)
	evaluationKey := uuid.NewString()
	f.call(t, f.token, "POST", f.evaluationsPath(), f.evaluationBody(1), "Idempotency-Key", evaluationKey).Want(200)
	evaluations := f.evaluationRows(t)
	installCompletionAuthorizationRouterPolicy(t, f, 2)
	consumer := f.fx.User(t, "Designated completion consumer", "completion-consumer-"+uuid.NewString()+"@example.test")
	f.fx.Member(t, testWorkspaceID, consumer, "member")
	consumerToken := controlledArtifactJWT(t, consumer, 0, time.Now().Add(time.Hour))
	body := completionAuthorizationRouterBody(t, f, 2, map[string]any{"kind": "human", "user_id": consumer})
	key := uuid.NewString()
	var saved completionAuthorizationRouterResponse
	f.call(t, f.token, "POST", completionAuthorizationRouterPath, body, "Idempotency-Key", key).Want(201).JSON(&saved)
	var projection map[string]json.RawMessage
	response := f.call(t, consumerToken, "GET", completionAuthorizationRouterPath+"/"+saved.AuthorizationID, nil).Want(200).JSON(&projection)
	allowed := map[string]bool{"authorization_id": true, "operation_id": true, "issue_id": true, "issue_revision": true, "target_status": true, "contract_version": true, "created_at": true, "expires_at": true, "state": true, "applicability": true, "replayed": true}
	for field := range projection {
		if !allowed[field] {
			t.Fatalf("issued authorization exposed nonminimal field %q", field)
		}
	}
	for _, private := range []string{f.output, f.request.StagingID, f.token, f.taskToken, f.approval.SpecDigest, f.approval.ReceiptDigest, "candidate.txt", evaluationKey} {
		if strings.Contains(response.Text(), private) {
			t.Fatal("consumer read exposed issuer private evidence or credential material")
		}
	}
	f.call(t, consumerToken, "GET", f.evaluationsPath()+"/"+evaluationKey, nil).Want(404)
	f.call(t, consumerToken, "GET", completionAuthorizationRouterPath+"/by-request-key/"+key+"?workspace_id="+testWorkspaceID, nil).Want(404)
	stranger := f.fx.User(t, "Other current member", "completion-stranger-"+uuid.NewString()+"@example.test")
	f.fx.Member(t, testWorkspaceID, stranger, "member")
	strangerToken := controlledArtifactJWT(t, stranger, 0, time.Now().Add(time.Hour))
	f.call(t, strangerToken, "GET", "/api/issues/"+f.request.ResourceID, nil).Want(200)
	f.call(t, strangerToken, "GET", completionAuthorizationRouterPath+"/"+saved.AuthorizationID, nil).Want(404)
	if f.evaluationRows(t) != evaluations {
		t.Fatal("narrow consumer authorization changed the original private evaluation")
	}
}

func TestDeliveryCompletionAuthorizationRouterDesignatedTaskReadsOriginalGrant(t *testing.T) {
	f := newDeliveryPolicyRouterFixture(t, "legacy", true)
	installCompletionAuthorizationRouterPolicy(t, f, 1)
	body := completionAuthorizationRouterBody(t, f, 1, map[string]any{"kind": "task", "agent_id": f.request.AgentID, "task_id": f.request.TaskID, "task_generation": 1})
	var saved, read completionAuthorizationRouterResponse
	f.call(t, f.token, "POST", completionAuthorizationRouterPath, body).Want(201).JSON(&saved)
	before := completionAuthorizationRouterRows(t, f)
	// The real task token carries the same owner UserID as the issuer. It must
	// still use the explicitly designated task consumer branch, not masquerade
	// as a human issuer because those owner IDs happen to match.
	f.call(t, f.taskToken, "GET", completionAuthorizationRouterPath+"/"+saved.AuthorizationID, nil).Want(200).JSON(&read)
	if read.AuthorizationID != saved.AuthorizationID || read.OperationID != saved.OperationID || read.State != "issued" {
		t.Fatal("designated task did not recover its exact minimal authorization")
	}
	f.call(t, f.taskToken, "POST", completionAuthorizationRouterPath+"/"+saved.AuthorizationID+"/revocations", map[string]string{"reason": "A task cannot revoke as its human owner"}).Want(403)
	if completionAuthorizationRouterRows(t, f) != before {
		t.Fatal("task read or refused human operation changed the original authorization")
	}
}

func TestDeliveryCompletionAuthorizationRouterConflictingWorkspaceSelectors(t *testing.T) {
	f := newDeliveryPolicyRouterFixture(t, "legacy", true)
	installCompletionAuthorizationRouterPolicy(t, f, 1)
	body := completionAuthorizationRouterBody(t, f, 1, map[string]any{"kind": "human", "user_id": f.user})
	for _, headers := range [][]string{{"X-Workspace-ID", uuid.NewString()}, {"X-Workspace-Slug", "another-source"}} {
		f.call(t, f.token, "POST", completionAuthorizationRouterPath, body, headers...).Want(403)
	}
	if f.fx.Count(t, `SELECT count(*) FROM delivery_completion_authorization WHERE issue_id=$1`, f.request.ResourceID) != 0 {
		t.Fatal("conflicting source selector admitted an authorization")
	}
	var saved completionAuthorizationRouterResponse
	f.call(t, f.token, "POST", completionAuthorizationRouterPath, body).Want(201).JSON(&saved)
	before := completionAuthorizationRouterRows(t, f)
	f.call(t, f.token, "GET", completionAuthorizationRouterPath+"/"+saved.AuthorizationID, nil, "X-Workspace-ID", uuid.NewString()).Want(403)
	f.call(t, f.token, "POST", completionAuthorizationRouterPath+"/"+saved.AuthorizationID+"/revocations", map[string]string{"reason": "Must not commit under conflicting scope"}, "X-Workspace-Slug", "another-source").Want(403)
	if completionAuthorizationRouterRows(t, f) != before {
		t.Fatal("selector refusal left a partially committed revocation")
	}
}

func TestDeliveryCompletionAuthorizationRouterCurrentCredentialRecheck(t *testing.T) {
	f := newDeliveryPolicyRouterFixture(t, "password", true)
	installCompletionAuthorizationRouterPolicy(t, f, 1)
	body := completionAuthorizationRouterBody(t, f, 1, map[string]any{"kind": "human", "user_id": f.user})
	req := testutil.WithHeaders(testutil.JSONRequest("POST", completionAuthorizationRouterPath, body), "Authorization", "Bearer "+f.token, "X-Workspace-ID", testWorkspaceID, "Idempotency-Key", uuid.NewString())
	revoked := false
	req.Body = &artifactRevokeOnRead{ReadCloser: req.Body, revoke: func() {
		f.fx.Exec(t, `UPDATE user_password_credential SET session_version=session_version+1 WHERE user_id=$1`, f.user)
		revoked = true
	}}
	denied := testutil.Call(t, f.router.ServeHTTP, req).Want(403)
	if !revoked || !strings.Contains(denied.Text(), "delivery_completion_denied") || f.fx.Count(t, `SELECT count(*) FROM delivery_completion_authorization WHERE issue_id=$1`, f.request.ResourceID) != 0 {
		t.Fatal("issuer credential changed after authentication but still admitted a grant")
	}
	current := controlledArtifactJWT(t, f.user, 2, time.Now().Add(time.Hour))
	var saved completionAuthorizationRouterResponse
	f.call(t, current, "POST", completionAuthorizationRouterPath, body).Want(201).JSON(&saved)
	f.fx.Exec(t, `UPDATE user_password_credential SET session_version=session_version+1 WHERE user_id=$1`, f.user)
	f.call(t, current, "GET", completionAuthorizationRouterPath+"/"+saved.AuthorizationID, nil).Want(401)
	if f.fx.Count(t, `SELECT count(*) FROM delivery_completion_authorization WHERE id=$1`, saved.AuthorizationID) != 1 {
		t.Fatal("credential revocation removed retained acceptance history")
	}
}

func TestDeliveryCompletionAuthorizationRouterExpiryBounds(t *testing.T) {
	f := newDeliveryPolicyRouterFixture(t, "legacy", true)
	installCompletionAuthorizationRouterPolicy(t, f, 1)
	body := completionAuthorizationRouterBody(t, f, 1, map[string]any{"kind": "human", "user_id": f.user})
	var artifactExpiry, now time.Time
	f.fx.QueryRow(t, `SELECT expires_at,clock_timestamp() FROM agent_artifact_execution WHERE approval_id=$1`, f.approval.Approval.ID).Scan(&artifactExpiry, &now)
	body["expires_at"] = artifactExpiry.Add(time.Second).UTC().Format(time.RFC3339Nano)
	response := f.call(t, f.token, "POST", completionAuthorizationRouterPath, body).Want(409)
	if !strings.Contains(response.Text(), "delivery_completion_expiry_invalid") {
		t.Fatal("expiry beyond the bound artifact was not explicitly refused")
	}
	body["expires_at"] = now.Add(-time.Second).UTC().Format(time.RFC3339Nano)
	f.call(t, f.token, "POST", completionAuthorizationRouterPath, body).Want(409)
	body["expires_at"] = now.Add(3 * time.Minute).UTC().Format(time.RFC3339Nano)
	short := controlledArtifactJWT(t, f.user, 0, now.Add(2*time.Minute))
	f.call(t, short, "POST", completionAuthorizationRouterPath, body).Want(409)
	if f.fx.Count(t, `SELECT count(*) FROM delivery_completion_authorization WHERE issue_id=$1`, f.request.ResourceID) != 0 {
		t.Fatal("invalid expiry produced a durable authorization")
	}
}

func TestDeliveryCompletionAuthorizationRouterSkipAndAmbiguousJSON(t *testing.T) {
	f := newDeliveryPolicyRouterFixture(t, "legacy", true)
	installCompletionAuthorizationRouterPolicy(t, f, 1)
	body := completionAuthorizationRouterBody(t, f, 1, map[string]any{"kind": "human", "user_id": f.user})
	body["skip"] = true
	response := f.call(t, f.token, "POST", completionAuthorizationRouterPath, body).Want(409)
	if !strings.Contains(response.Text(), "delivery_skip_not_supported") {
		t.Fatal("unsupported skip was not explicit")
	}
	delete(body, "skip")
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	duplicate := strings.TrimSuffix(string(raw), "}") + `,"reason":"contradictory acceptance"}`
	f.call(t, f.token, "POST", completionAuthorizationRouterPath, duplicate).Want(400)
	f.call(t, f.token, "POST", completionAuthorizationRouterPath, string(raw)+`{}`).Want(400)
	if f.fx.Count(t, `SELECT count(*) FROM delivery_completion_authorization WHERE issue_id=$1`, f.request.ResourceID) != 0 {
		t.Fatal("unsupported or ambiguous intent wrote an authorization")
	}
}

func TestDeliveryCompletionAuthorizationRouterRevocationRecovery(t *testing.T) {
	f := newDeliveryPolicyRouterFixture(t, "legacy", true)
	installCompletionAuthorizationRouterPolicy(t, f, 1)
	consumer := f.fx.User(t, "Completion revocation consumer", "completion-revocation-"+uuid.NewString()+"@example.test")
	f.fx.Member(t, testWorkspaceID, consumer, "member")
	consumerToken := controlledArtifactJWT(t, consumer, 0, time.Now().Add(time.Hour))
	body := completionAuthorizationRouterBody(t, f, 1, map[string]any{"kind": "human", "user_id": consumer})
	var saved, revoked, restored completionAuthorizationRouterResponse
	f.call(t, f.token, "POST", completionAuthorizationRouterPath, body).Want(201).JSON(&saved)
	before, archive := f.snapshot(t), f.archive(t)
	path := completionAuthorizationRouterPath + "/" + saved.AuthorizationID + "/revocations"
	reason := map[string]string{"reason": "Withdraw this exact unused authorization"}
	f.call(t, consumerToken, "POST", path, reason).Want(403)
	key := uuid.NewString()
	f.call(t, f.token, "POST", path, reason, "Idempotency-Key", key).Want(200).JSON(&revoked)
	if revoked.State != "revoked" || revoked.AuthorizationID != saved.AuthorizationID || revoked.Replayed || !revoked.ExpiresAt.Equal(saved.ExpiresAt) {
		t.Fatal("revocation replaced acceptance identity or lifetime")
	}
	rows := completionAuthorizationRouterRows(t, f)
	f.router = controlledArtifactRouter(t)
	f.call(t, f.token, "POST", path, reason, "Idempotency-Key", key).Want(200).JSON(&restored)
	if restored.State != "revoked" || !restored.Replayed {
		t.Fatal("original revocation key did not recover its committed fact")
	}
	f.call(t, f.token, "POST", path, map[string]string{"reason": "Changed original intent"}, "Idempotency-Key", key).Want(409)
	f.call(t, consumerToken, "GET", completionAuthorizationRouterPath+"/"+saved.AuthorizationID, nil).Want(200).JSON(&restored)
	if restored.State != "revoked" || completionAuthorizationRouterRows(t, f) != rows || f.snapshot(t) != before || !bytes.Equal(f.archive(t), archive) {
		t.Fatal("revocation recovery changed state, archive or original acceptance records")
	}
	if f.fx.Count(t, `SELECT count(*) FROM delivery_completion_terminal WHERE authorization_id=$1 AND kind='revoked' AND revoker_user_id=$2 AND revocation_request_key=$3`, saved.AuthorizationID, f.user, key) != 1 {
		t.Fatal("revocation did not retain exactly one original actor/key fact")
	}
}
