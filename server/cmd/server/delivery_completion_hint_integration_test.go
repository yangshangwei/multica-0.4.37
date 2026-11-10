package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/artifact"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/testutil"
)

// Handler tests own the validation and receipt-state matrices. These tests
// exercise real authentication, packaging, persistence and route compatibility.
type deliveryCompletionHintRouterResponse struct {
	IssueID             string                            `json:"issue_id"`
	IssueRevision       int64                             `json:"issue_revision"`
	TargetStatus        string                            `json:"target_status"`
	TargetCategory      string                            `json:"target_category"`
	ProjectID           *string                           `json:"project_id"`
	PolicyRevision      int64                             `json:"policy_revision"`
	PolicyMode          string                            `json:"policy_mode"`
	Hint                string                            `json:"hint"`
	Reason              string                            `json:"reason"`
	CheckedAt           string                            `json:"checked_at"`
	Enforced            *bool                             `json:"enforced"`
	UsableForCompletion *bool                             `json:"usable_for_completion"`
	Evaluation          *deliveryEvaluationRouterResponse `json:"evaluation"`
}

func (f *deliveryPolicyRouterFixture) completionHintPath() string {
	return "/api/issues/" + f.request.ResourceID + "/delivery-evidence/completion-hint"
}

func (f *deliveryPolicyRouterFixture) completionHintBody(t *testing.T, status string, key *string) map[string]any {
	t.Helper()
	var revision int64
	f.fx.QueryRow(t, `SELECT revision FROM issue WHERE id=$1`, f.request.ResourceID).Scan(&revision)
	return map[string]any{"expected_issue_revision": revision, "target_status": status, "evaluation_key": key}
}

func (f *deliveryPolicyRouterFixture) completionHintSnapshot(t *testing.T) string {
	t.Helper()
	var snapshot string
	f.fx.QueryRow(t, `SELECT jsonb_build_object(
		'delivery',$2::jsonb,
		'executions',(SELECT COALESCE(jsonb_agg(to_jsonb(e) ORDER BY e.execution_id),'[]'::jsonb) FROM agent_artifact_execution e WHERE e.resource_id=$1),
		'approvals',(SELECT COALESCE(jsonb_agg(to_jsonb(a) ORDER BY a.id),'[]'::jsonb) FROM agent_approval_request a WHERE a.task_id=$4),
		'evaluations',(SELECT COALESCE(jsonb_agg(to_jsonb(e) ORDER BY e.evaluation_id),'[]'::jsonb) FROM delivery_evidence_evaluation e WHERE e.issue_id=$1),
		'tasks',(SELECT COALESCE(jsonb_agg(to_jsonb(t) ORDER BY t.id),'[]'::jsonb) FROM agent_task_queue t WHERE t.issue_id=$1),
		'comments',(SELECT COALESCE(jsonb_agg(to_jsonb(c) ORDER BY c.id),'[]'::jsonb) FROM comment c WHERE c.issue_id=$1),
		'policies',(SELECT COALESCE(jsonb_agg(to_jsonb(p) ORDER BY p.revision),'[]'::jsonb) FROM project_delivery_policy p WHERE p.project_id=$3)
	)::text`, f.request.ResourceID, f.snapshot(t), f.project, f.request.TaskID).Scan(&snapshot)
	return snapshot
}

func TestDeliveryCompletionHintRouterRealPackageAndRestart(t *testing.T) {
	for _, mode := range []string{"legacy", "password"} {
		t.Run(mode, func(t *testing.T) {
			f := newDeliveryPolicyRouterFixture(t, mode, true)
			f.enablePolicy(t)
			key := uuid.NewString()
			var saved deliveryEvaluationRouterResponse
			f.call(t, f.token, "POST", f.evaluationsPath(), f.evaluationBody(1), "Idempotency-Key", key).Want(200).JSON(&saved)
			archive, before := f.archive(t), f.completionHintSnapshot(t)
			var retained deliveryEvidenceRouterResponse
			var raw []byte
			var actor, retainedKey, retainedID string
			f.fx.QueryRow(t, `SELECT evaluator_user_id::text,request_key::text,evaluation_id::text,observation FROM delivery_evidence_evaluation WHERE issue_id=$1 AND request_key=$2`, f.request.ResourceID, key).Scan(&actor, &retainedKey, &retainedID, &raw)
			if err := json.Unmarshal(raw, &retained); err != nil {
				t.Fatal(err)
			}
			if actor != f.user || retainedKey != key || retainedID != saved.EvaluationID || !reflect.DeepEqual(retained, saved.Observation) || retained.ExecutionID != f.approval.ExecutionID || retained.ProducerTaskID != f.request.TaskID || retained.ReceiptDigest != f.approval.ReceiptDigest || retained.ArtifactDigest != f.body["expected_artifact_digest"] {
				t.Fatal("hint fixture lacks exact persisted packaging provenance")
			}
			for _, restart := range []bool{false, true} {
				if restart {
					f.router = controlledArtifactRouter(t)
					version := int64(0)
					if mode == "password" {
						version = 1
					}
					f.token = controlledArtifactJWT(t, f.user, version, time.Now().Add(2*time.Hour))
				}
				var hint deliveryCompletionHintRouterResponse
				body := f.completionHintBody(t, "done", &key)
				response := f.call(t, f.token, "POST", f.completionHintPath(), body).Want(200)
				response.JSON(&hint)
				if hint.Hint != "matching" || hint.Reason != "observation_still_matches" || hint.IssueID != f.request.ResourceID || hint.IssueRevision != body["expected_issue_revision"] || hint.TargetStatus != "done" || hint.TargetCategory != "done" || hint.PolicyMode != "observe" || hint.PolicyRevision != 1 || hint.ProjectID == nil || *hint.ProjectID != f.project {
					t.Fatalf("hint lost current policy or issue context after restart=%v: %+v", restart, hint)
				}
				if hint.Enforced == nil || *hint.Enforced || hint.UsableForCompletion == nil || *hint.UsableForCompletion || hint.Evaluation == nil || hint.Evaluation.UsableForCompletion == nil || *hint.Evaluation.UsableForCompletion {
					t.Fatalf("hint implied completion authority or omitted its explicit boundary: %+v", hint)
				}
				if hint.Evaluation.EvaluationID != saved.EvaluationID || hint.Evaluation.RequestKey != key || hint.Evaluation.EvaluatorUserID != f.user || !reflect.DeepEqual(hint.Evaluation.Observation, retained) || hint.Evaluation.Applicability.Status != "current" || hint.CheckedAt != hint.Evaluation.Applicability.CheckedAt {
					t.Fatalf("hint replaced the original evaluation or changed its recheck time: %+v", hint)
				}
				if _, err := time.Parse(time.RFC3339Nano, hint.CheckedAt); err != nil {
					t.Fatal("hint omitted a valid database check time")
				}
				if response.Header().Get("Cache-Control") != "no-store" {
					t.Fatal("completion hint is cacheable")
				}
				for _, private := range []string{f.output, f.request.StagingID, "candidate.txt"} {
					if strings.Contains(response.Text(), private) {
						t.Fatal("completion hint disclosed private packaging inputs")
					}
				}
			}
			if f.completionHintSnapshot(t) != before || !bytes.Equal(f.archive(t), archive) {
				t.Fatal("hint or restart recovery mutated policy, evaluation, lifecycle, tasks, comments or artifact bytes")
			}
		})
	}
}

func TestDeliveryCompletionHintRouterTargetAndPolicyBranches(t *testing.T) {
	f := newDeliveryPolicyRouterFixture(t, "legacy", true)
	f.enablePolicy(t)
	key := uuid.NewString()
	f.call(t, f.token, "POST", f.evaluationsPath(), f.evaluationBody(1), "Idempotency-Key", key).Want(200)
	custom := "accepted_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:16]
	f.fx.Insert(t, "issue_status", testutil.Cols{"workspace_id": testWorkspaceID, "key": custom, "name": "Accepted", "category": "done", "color": "#334455"})
	missing := uuid.NewString()
	for _, tc := range []struct {
		name, target, hint, reason string
		key                        *string
	}{
		{"custom_done", " " + strings.ToUpper(custom) + " ", "matching", "observation_still_matches", &key},
		{"missing_selection", "done", "attention", "evaluation_required", nil},
		{"non_done_ignores_unread_key", "in_review", "not_applicable", "target_not_done", &missing},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := f.completionHintSnapshot(t)
			var out deliveryCompletionHintRouterResponse
			f.call(t, f.token, "POST", f.completionHintPath(), f.completionHintBody(t, tc.target, tc.key)).Want(200).JSON(&out)
			if out.Hint != tc.hint || out.Reason != tc.reason || out.TargetStatus != strings.ToLower(strings.TrimSpace(tc.target)) || out.Enforced == nil || *out.Enforced || out.UsableForCompletion == nil || *out.UsableForCompletion {
				t.Fatalf("wrong target hint: %+v", out)
			}
			if tc.name == "custom_done" && (out.TargetCategory != "done" || out.Evaluation == nil) {
				t.Fatal("custom done failed to consume existing evaluation")
			}
			if tc.name != "custom_done" && out.Evaluation != nil {
				t.Fatal("inapplicable or unselected hint disclosed an evaluation")
			}
			if f.completionHintSnapshot(t) != before {
				t.Fatal("target hint changed durable facts")
			}
		})
	}
	f.fx.Exec(t, `UPDATE member SET role='admin' WHERE workspace_id=$1 AND user_id=$2`, testWorkspaceID, f.user)
	f.call(t, f.token, "PUT", f.policyPath(), f.policyBody(1, "off")).Want(200)
	for _, noProject := range []bool{false, true} {
		if noProject {
			f.fx.Exec(t, `UPDATE issue SET project_id=NULL,revision=revision+1 WHERE id=$1`, f.request.ResourceID)
		}
		before := f.completionHintSnapshot(t)
		var out deliveryCompletionHintRouterResponse
		f.call(t, f.token, "POST", f.completionHintPath(), f.completionHintBody(t, "done", &missing)).Want(200).JSON(&out)
		wantReason, wantRevision := "policy_off", int64(2)
		if noProject {
			wantReason, wantRevision = "no_project", 0
		}
		if out.Hint != "not_applicable" || out.Reason != wantReason || out.PolicyMode != "off" || out.PolicyRevision != wantRevision || out.Evaluation != nil || (noProject && out.ProjectID != nil) || f.completionHintSnapshot(t) != before {
			t.Fatalf("disabled policy leaked or consumed evidence: %+v", out)
		}
	}
}

func TestDeliveryCompletionHintRouterIdentityAndPrivateKeyBoundaries(t *testing.T) {
	f := newDeliveryPolicyRouterFixture(t, "legacy", true)
	f.enablePolicy(t)
	key := uuid.NewString()
	var saved deliveryEvaluationRouterResponse
	f.call(t, f.token, "POST", f.evaluationsPath(), f.evaluationBody(1), "Idempotency-Key", key).Want(200).JSON(&saved)
	body := f.completionHintBody(t, "done", &key)
	before := f.completionHintSnapshot(t)
	f.call(t, f.taskToken, "POST", f.completionHintPath(), body, "X-Actor-Source", "member", "X-User-ID", f.user).Want(403)
	f.call(t, f.token, "POST", f.completionHintPath(), body, "X-Agent-ID", f.request.AgentID).Want(403)
	unauthenticated := testutil.WithHeaders(testutil.JSONRequest("POST", f.completionHintPath(), body), "X-User-ID", f.user, "X-Workspace-ID", testWorkspaceID)
	testutil.Call(t, f.router.ServeHTTP, unauthenticated).Want(401)
	other := f.fx.User(t, "Other hint reader", "hint-other-"+uuid.NewString()+"@example.test")
	f.fx.Member(t, testWorkspaceID, other, "admin")
	otherToken := controlledArtifactJWT(t, other, 0, time.Now().Add(time.Hour))
	response := f.call(t, otherToken, "POST", f.completionHintPath(), body).Want(404)
	if strings.Contains(response.Text(), saved.EvaluationID) || strings.Contains(response.Text(), "observation") {
		t.Fatal("other human discovered a private evaluation")
	}
	missing := uuid.NewString()
	f.call(t, f.token, "POST", f.completionHintPath(), f.completionHintBody(t, "done", &missing)).Want(404)
	second := f.fx.Issue(t, "Other hinted issue", testutil.Cols{"project_id": f.project})
	var revision int64
	f.fx.QueryRow(t, `SELECT revision FROM issue WHERE id=$1`, second).Scan(&revision)
	f.call(t, f.token, "POST", "/api/issues/"+second+"/delivery-evidence/completion-hint", map[string]any{"expected_issue_revision": revision, "target_status": "done", "evaluation_key": key}).Want(409)
	foreign := f.fx.Workspace(t, "Foreign hint workspace", "hint-foreign-"+uuid.NewString())
	f.fx.Member(t, foreign, f.user, "owner")
	f.call(t, f.token, "POST", f.completionHintPath(), body, "X-Workspace-ID", foreign).Want(404)
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	for _, malformed := range []string{string(raw[:len(raw)-1]) + `,"verdict":"pass"}`, string(raw[:len(raw)-1]) + `,"target_status":"done"}`, string(raw) + `{}`} {
		f.call(t, f.token, "POST", f.completionHintPath(), malformed).Want(400)
	}
	f.call(t, f.token, "POST", f.completionHintPath(), strings.Repeat(" ", 4097)).Want(413)
	if f.completionHintSnapshot(t) != before {
		t.Fatal("identity or contract refusal changed evidence or lifecycle")
	}
}

func TestDeliveryCompletionHintRouterRechecksCredentialAfterAuthentication(t *testing.T) {
	for _, kind := range []string{"password", "pat", "membership"} {
		t.Run(kind, func(t *testing.T) {
			mode := "legacy"
			if kind == "password" {
				mode = "password"
			}
			f := newDeliveryPolicyRouterFixture(t, mode, true)
			f.enablePolicy(t)
			key := uuid.NewString()
			f.call(t, f.token, "POST", f.evaluationsPath(), f.evaluationBody(1), "Idempotency-Key", key).Want(200)
			body := f.completionHintBody(t, "done", &key)
			token, patID := f.token, ""
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
				patID = f.fx.Insert(t, "personal_access_token", testutil.Cols{"user_id": f.user, "name": "Completion hint PAT", "token_hash": auth.HashToken(token), "token_prefix": token[:8], "expires_at": time.Now().Add(time.Hour)})
				revoke = func() { f.fx.Exec(t, `UPDATE personal_access_token SET revoked=true WHERE id=$1`, patID) }
			}
			request := testutil.WithHeaders(testutil.JSONRequest("POST", f.completionHintPath(), body), "Authorization", "Bearer "+token, "X-Workspace-ID", testWorkspaceID)
			if patID != "" {
				// Synchronize the real middleware's asynchronous last-used write;
				// otherwise its NOWAIT conflict is a legitimate refusal, not pass.
				request.Body = &artifactRevokeOnRead{ReadCloser: request.Body, revoke: func() {
					ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
					defer cancel()
					ticker := time.NewTicker(time.Millisecond)
					defer ticker.Stop()
					for {
						var used bool
						if err := testPool.QueryRow(ctx, `SELECT last_used_at IS NOT NULL FROM personal_access_token WHERE id=$1`, patID).Scan(&used); err != nil {
							t.Fatal(err)
						}
						if used {
							return
						}
						select {
						case <-ctx.Done():
							t.Fatal("authenticated PAT last-used write did not commit")
						case <-ticker.C:
						}
					}
				}}
			}
			var positive deliveryCompletionHintRouterResponse
			testutil.Call(t, f.router.ServeHTTP, request).Want(200).JSON(&positive)
			if positive.Hint != "matching" || positive.Evaluation == nil || positive.Evaluation.EvaluatorUserID != f.user {
				t.Fatal("valid current credential did not recover its human's original evaluation")
			}
			before := f.completionHintSnapshot(t)
			request = testutil.WithHeaders(testutil.JSONRequest("POST", f.completionHintPath(), body), "Authorization", "Bearer "+token, "X-Workspace-ID", testWorkspaceID)
			revoked := false
			request.Body = &artifactRevokeOnRead{ReadCloser: request.Body, revoke: func() { revoke(); revoked = true }}
			want := http.StatusForbidden
			if kind == "pat" {
				want = http.StatusNotFound
			}
			response := testutil.Call(t, f.router.ServeHTTP, request).Want(want)
			if !revoked || strings.Contains(response.Text(), "observation") || f.completionHintSnapshot(t) != before {
				t.Fatalf("%s revocation returned protected evidence or altered durable facts", kind)
			}
		})
	}
}

func TestDeliveryCompletionHintRouterStaleAndOriginalUnknown(t *testing.T) {
	for _, change := range []string{"policy_revision", "issue_revision", "pr_head", "original_unknown"} {
		t.Run(change, func(t *testing.T) {
			f := newDeliveryPolicyRouterFixture(t, "legacy", true)
			f.enablePolicy(t)
			if change == "original_unknown" {
				f.fx.Exec(t, `UPDATE issue_pull_request SET reference_only=true WHERE issue_id=$1 AND pull_request_id=$2`, f.request.ResourceID, f.pr)
			}
			key := uuid.NewString()
			var saved deliveryEvaluationRouterResponse
			f.call(t, f.token, "POST", f.evaluationsPath(), f.evaluationBody(1), "Idempotency-Key", key).Want(200).JSON(&saved)
			body := f.completionHintBody(t, "done", &key)
			wantReason := "policy_changed"
			switch change {
			case "policy_revision":
				f.fx.Exec(t, `UPDATE member SET role='admin' WHERE workspace_id=$1 AND user_id=$2`, testWorkspaceID, f.user)
				f.call(t, f.token, "PUT", f.policyPath(), f.policyBody(1, "observe")).Want(200)
			case "issue_revision":
				f.fx.Exec(t, `UPDATE issue SET revision=revision+1 WHERE id=$1`, f.request.ResourceID)
				f.call(t, f.token, "POST", f.completionHintPath(), body).Want(409)
				body = f.completionHintBody(t, "done", &key)
				wantReason = "resource_revision_changed"
			case "pr_head":
				f.fx.Exec(t, `UPDATE github_pull_request SET head_sha=$1 WHERE id=$2`, strings.Repeat("b", 40), f.pr)
				wantReason = "source_revision_mismatch"
			case "original_unknown":
				if saved.Observation.Verdict != "unknown" {
					t.Fatal("missing observed head was not retained as unknown")
				}
				f.fx.Exec(t, `UPDATE issue_pull_request SET reference_only=false WHERE issue_id=$1 AND pull_request_id=$2`, f.request.ResourceID, f.pr)
				wantReason = "original_observation_unknown"
			}
			before := f.completionHintSnapshot(t)
			var out deliveryCompletionHintRouterResponse
			f.call(t, f.token, "POST", f.completionHintPath(), body).Want(200).JSON(&out)
			if out.Hint != "attention" || out.Reason != wantReason || out.Evaluation == nil || out.Evaluation.EvaluationID != saved.EvaluationID || !reflect.DeepEqual(out.Evaluation.Observation, saved.Observation) || out.Evaluation.Applicability.Status == "current" || out.CheckedAt != out.Evaluation.Applicability.CheckedAt || f.completionHintSnapshot(t) != before {
				t.Fatalf("%s promoted or changed original evidence: %+v", change, out)
			}
		})
	}
}

func TestDeliveryCompletionHintRouterActualExpiryDoesNotRenewEvidence(t *testing.T) {
	f := newDeliveryPolicyRouterFixture(t, "legacy", false)
	f.enablePolicy(t)
	// Move this synthetic fixture's entire frozen 15-minute window before
	// public approval and actual packaging, then wait for real database expiry.
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
	spec.CreatedAt, spec.ExpiresAt = created.Format("2006-01-02T15:04:05.000000Z"), expires.Format("2006-01-02T15:04:05.000000Z")
	canonical, digest, err := artifact.CanonicalSpec(spec)
	if err != nil {
		t.Fatal(err)
	}
	f.fx.Exec(t, `UPDATE agent_artifact_execution SET created_at=$1,expires_at=$2,canonical_spec=$3,spec_digest=$4 WHERE approval_id=$5`, created, expires, canonical, digest, f.approval.Approval.ID)
	f.approval.SpecDigest, f.body["expected_spec_digest"] = digest, digest
	base := "/api/agent-approvals/" + f.approval.Approval.ID
	f.call(t, f.token, "POST", base+"/decision", map[string]string{"decision": "approve", "expected_spec_digest": digest}).Want(200)
	f.call(t, f.taskToken, "POST", base+"/artifact-package/execution", map[string]string{"expected_spec_digest": digest}).Want(200).JSON(&f.approval)
	var artifactDigest string
	f.fx.QueryRow(t, `SELECT output_artifact_digest FROM agent_artifact_execution WHERE approval_id=$1`, f.approval.Approval.ID).Scan(&artifactDigest)
	f.body["expected_artifact_digest"] = artifactDigest
	key := uuid.NewString()
	var saved deliveryEvaluationRouterResponse
	f.call(t, f.token, "POST", f.evaluationsPath(), f.evaluationBody(1), "Idempotency-Key", key).Want(200).JSON(&saved)
	var positive deliveryCompletionHintRouterResponse
	f.call(t, f.token, "POST", f.completionHintPath(), f.completionHintBody(t, "done", &key)).Want(200).JSON(&positive)
	if saved.Observation.Verdict != "pass" || positive.Hint != "matching" {
		t.Fatal("near-expiry control did not use an actual currently valid pass")
	}
	archive, before := f.archive(t), f.completionHintSnapshot(t)
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
	var out deliveryCompletionHintRouterResponse
	f.call(t, f.token, "POST", f.completionHintPath(), f.completionHintBody(t, "done", &key)).Want(200).JSON(&out)
	if out.Hint != "attention" || out.Reason != "evidence_expired" || out.Evaluation == nil || out.Evaluation.Applicability.Status != "expired" || !reflect.DeepEqual(out.Evaluation.Observation, saved.Observation) || out.CheckedAt != out.Evaluation.Applicability.CheckedAt || f.completionHintSnapshot(t) != before || !bytes.Equal(f.archive(t), archive) {
		t.Fatalf("hint renewed or consumed an expired receipt: %+v", out)
	}
}

func TestDeliveryCompletionHintRouterPolicyAndStatusLockContention(t *testing.T) {
	f := newDeliveryPolicyRouterFixture(t, "legacy", true)
	f.enablePolicy(t)
	key := uuid.NewString()
	f.call(t, f.token, "POST", f.evaluationsPath(), f.evaluationBody(1), "Idempotency-Key", key).Want(200)
	custom := "accepted_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:16]
	f.fx.Insert(t, "issue_status", testutil.Cols{"workspace_id": testWorkspaceID, "key": custom, "name": "Accepted", "category": "done", "color": "#334455"})
	for _, lock := range []string{"project", "custom_status_catalog"} {
		t.Run(lock, func(t *testing.T) {
			before := f.completionHintSnapshot(t)
			tx, err := testPool.Begin(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(context.Background())
			if lock == "project" {
				_, err = tx.Exec(t.Context(), `SELECT id FROM project WHERE id=$1 FOR UPDATE`, f.project)
			} else {
				_, err = tx.Exec(t.Context(), `SELECT pg_advisory_xact_lock(hashtextextended($1::uuid::text || ':issue_status',0))`, testWorkspaceID)
			}
			if err != nil {
				t.Fatal(err)
			}
			f.call(t, f.token, "POST", f.completionHintPath(), f.completionHintBody(t, custom, &key)).Want(409)
			if err := tx.Rollback(t.Context()); err != nil {
				t.Fatal(err)
			}
			var out deliveryCompletionHintRouterResponse
			f.call(t, f.token, "POST", f.completionHintPath(), f.completionHintBody(t, custom, &key)).Want(200).JSON(&out)
			if out.Hint != "matching" || f.completionHintSnapshot(t) != before {
				t.Fatal("lock refusal wrote partial state or prevented a later explicit hint")
			}
		})
	}
}

func TestDeliveryCompletionHintRouterLegacyCompletionsRemainIndependent(t *testing.T) {
	f := newDeliveryPolicyRouterFixture(t, "legacy", false)
	f.enablePolicy(t)
	var missing deliveryCompletionHintRouterResponse
	f.call(t, f.token, "POST", f.completionHintPath(), f.completionHintBody(t, "done", nil)).Want(200).JSON(&missing)
	if missing.Hint != "attention" || missing.Reason != "evaluation_required" {
		t.Fatal("legacy completion control did not first observe missing evidence")
	}
	custom := "accepted_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:16]
	f.fx.Insert(t, "issue_status", testutil.Cols{"workspace_id": testWorkspaceID, "key": custom, "name": "Accepted", "category": "done", "color": "#334455"})
	f.call(t, f.token, "PUT", "/api/issues/"+f.request.ResourceID, map[string]string{"status": custom}).Want(200)
	second := f.fx.Issue(t, "Batch completion with no evaluation", testutil.Cols{"project_id": f.project})
	batch := f.call(t, f.token, "POST", "/api/issues/batch-update", map[string]any{"issue_ids": []string{second}, "updates": map[string]string{"status": "done"}}).Want(200).Map()
	if batch["updated"] != float64(1) {
		t.Fatal("hint was used as a batch completion condition")
	}
	title := "Create completed without a hint " + uuid.NewString()
	f.fx.Cleanup(t, `DELETE FROM issue WHERE workspace_id=$1 AND title=$2`, testWorkspaceID, title)
	// Fixture.Issue allocates MAX(number)+1 without consuming the product's
	// workspace counter. Align that fixture state before the real create path.
	f.fx.Exec(t, `UPDATE workspace SET issue_counter=GREATEST(issue_counter,(SELECT COALESCE(max(number),0) FROM issue WHERE workspace_id=workspace.id)) WHERE id=$1`, testWorkspaceID)
	created := f.call(t, f.token, "POST", "/api/issues", map[string]string{"title": title, "project_id": f.project, "status": custom}).Want(201).Map()
	createdID, ok := created["id"].(string)
	if !ok || createdID == "" {
		t.Fatal("create-as-done did not return an issue")
	}
	for id, want := range map[string]string{f.request.ResourceID: custom, second: "done", createdID: custom} {
		var status string
		f.fx.QueryRow(t, `SELECT status FROM issue WHERE id=$1`, id).Scan(&status)
		if status != want {
			t.Fatalf("legacy completion did not persist %s for %s: %s", want, id, status)
		}
	}
	if f.fx.Count(t, `SELECT count(*) FROM delivery_evidence_evaluation WHERE project_id=$1`, f.project) != 0 {
		t.Fatal("legacy completion fabricated a persistent evaluation")
	}
	f.noClaim(t, f.approval.Approval.ID)
}

func TestDeliveryCompletionHintRouterRejectsIndependentCollaboration(t *testing.T) {
	f := newDeliveryPolicyRouterFixture(t, "legacy", false)
	f.enablePolicy(t)
	f.fx.InsertNoID(t, "issue_collaboration", testutil.Cols{"issue_id": f.request.ResourceID}, "issue_id=$1", f.request.ResourceID)
	f.fx.Insert(t, "collaboration_session", testutil.Cols{"issue_id": f.request.ResourceID})
	f.fx.InsertNoID(t, "issue_participant", testutil.Cols{"issue_id": f.request.ResourceID, "user_id": f.user}, "issue_id=$1 AND user_id=$2", f.request.ResourceID, f.user)
	before := f.completionHintSnapshot(t)
	response := f.call(t, f.token, "POST", f.completionHintPath(), f.completionHintBody(t, "done", nil)).Want(409)
	if response.Map()["code"] != "unsupported_collaboration_action" || strings.Contains(response.Text(), "observation") || f.completionHintSnapshot(t) != before {
		t.Fatal("ordinary completion hint bypassed the independent collaboration workflow")
	}
}
