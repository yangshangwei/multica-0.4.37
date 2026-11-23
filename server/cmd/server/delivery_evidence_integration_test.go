package main

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/testutil"
)

// Handler tests own the receipt-state and concurrency matrices. These tests
// prove the public route authenticates real credentials and observes a real
// filesystem execution without advancing any existing lifecycle.
type deliveryEvidenceRouterResponse struct {
	Mode                   string  `json:"mode"`
	PolicyVersion          string  `json:"policy_version"`
	Verdict                string  `json:"verdict"`
	Reason                 string  `json:"reason"`
	IssueID                string  `json:"issue_id"`
	IssueRevision          int64   `json:"issue_revision"`
	ProjectID              *string `json:"project_id"`
	ProjectRevision        *int64  `json:"project_revision"`
	ApprovalID             string  `json:"approval_id"`
	ExecutionID            string  `json:"execution_id"`
	SourceRevision         string  `json:"source_revision"`
	ObservedHeadSHA        string  `json:"observed_head_sha"`
	RevisionProvenance     string  `json:"revision_provenance"`
	SpecDigest             string  `json:"spec_digest"`
	ArtifactDigest         string  `json:"artifact_digest"`
	ReceiptDigest          string  `json:"receipt_digest"`
	ProducerAgentID        string  `json:"producer_agent_id"`
	ProducerTaskID         string  `json:"producer_task_id"`
	ProducerTaskGeneration int64   `json:"producer_task_generation"`
	ObservedAt             string  `json:"observed_at"`
	ValidUntil             string  `json:"valid_until"`
}

type deliveryEvidenceRouterFixture struct {
	*controlledArtifactRouterFixture
	approval controlledArtifactRouterResponse
	body     map[string]any
	pr       string
}

func newDeliveryEvidenceRouterFixture(t *testing.T, mode string, execute bool) *deliveryEvidenceRouterFixture {
	t.Helper()
	f := &deliveryEvidenceRouterFixture{controlledArtifactRouterFixture: newControlledArtifactRouterFixture(t, mode)}
	f.approval = f.create(t, f.taskToken)
	base := "/api/agent-approvals/" + f.approval.Approval.ID
	if execute {
		f.call(t, f.token, "POST", base+"/decision", map[string]string{"decision": "approve", "expected_spec_digest": f.approval.SpecDigest}).Want(200)
		f.call(t, f.taskToken, "POST", base+"/artifact-package/execution", map[string]string{"expected_spec_digest": f.approval.SpecDigest}).Want(200).JSON(&f.approval)
		if f.approval.Status != "succeeded" || f.approval.Approval.Status != "executed" || f.approval.ReceiptDigest == "" {
			t.Fatalf("real packaging did not commit success: %+v", f.approval)
		}
	}
	var revision int64
	var digest string
	f.fx.QueryRow(t, `SELECT revision FROM issue WHERE id=$1`, f.request.ResourceID).Scan(&revision)
	f.fx.QueryRow(t, `SELECT output_artifact_digest FROM agent_artifact_execution WHERE approval_id=$1`, f.approval.Approval.ID).Scan(&digest)
	if !execute {
		digest = strings.Repeat("0", 64)
	}
	f.body = map[string]any{
		"policy_version": "delivery-artifact-observe-v1", "expected_issue_revision": revision,
		"expected_project_id": nil, "expected_project_revision": nil,
		"approval_id": f.approval.Approval.ID, "expected_source_revision": f.request.SourceRevision,
		"expected_spec_digest": f.approval.SpecDigest, "expected_artifact_digest": digest,
	}
	f.pr = f.fx.Insert(t, "github_pull_request", testutil.Cols{
		"workspace_id": testWorkspaceID, "installation_id": 1, "repo_owner": "synthetic",
		"repo_name": "delivery-" + uuid.NewString(), "pr_number": 1, "title": "Observed candidate",
		"state": "open", "html_url": "https://example.invalid/pull/1", "head_sha": f.request.SourceRevision,
		"pr_created_at": time.Now(), "pr_updated_at": time.Now(),
	})
	f.fx.InsertNoID(t, "issue_pull_request", testutil.Cols{"issue_id": f.request.ResourceID, "pull_request_id": f.pr}, "issue_id=$1 AND pull_request_id=$2", f.request.ResourceID, f.pr)
	return f
}

func (f *deliveryEvidenceRouterFixture) path() string {
	return "/api/issues/" + f.request.ResourceID + "/delivery-evidence/evaluate"
}

func (f *deliveryEvidenceRouterFixture) evaluate(t *testing.T, token string) deliveryEvidenceRouterResponse {
	t.Helper()
	var out deliveryEvidenceRouterResponse
	response := f.call(t, token, "POST", f.path(), f.body).Want(http.StatusOK)
	response.JSON(&out)
	if response.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("observation response can be cached")
	}
	for _, private := range []string{f.output, f.request.StagingID, "candidate.txt"} {
		if strings.Contains(response.Text(), private) {
			t.Fatal("observation disclosed private packaging inputs")
		}
	}
	return out
}

func (f *deliveryEvidenceRouterFixture) snapshot(t *testing.T) string {
	t.Helper()
	var snapshot string
	f.fx.QueryRow(t, `SELECT jsonb_build_object('issue',to_jsonb(i),'execution',to_jsonb(e),'approval',to_jsonb(a),'tasks',(SELECT count(*) FROM agent_task_queue WHERE issue_id=i.id),'comments',(SELECT count(*) FROM comment WHERE issue_id=i.id))::text FROM issue i JOIN agent_artifact_execution e ON e.resource_id=i.id JOIN agent_approval_request a ON a.id=e.approval_id WHERE i.id=$1 AND e.approval_id=$2`, f.request.ResourceID, f.approval.Approval.ID).Scan(&snapshot)
	return snapshot
}

func (f *deliveryEvidenceRouterFixture) archive(t *testing.T) []byte {
	t.Helper()
	entries, err := os.ReadDir(f.output)
	if err != nil || len(entries) != 1 || entries[0].Name() != f.approval.ExecutionID {
		t.Fatalf("expected exactly one execution directory, got %v: %v", entries, err)
	}
	path := filepath.Join(f.output, f.approval.ExecutionID, "artifact.tar")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprintf("%x", sha256.Sum256(data)) != f.body["expected_artifact_digest"] {
		t.Fatal("physical archive does not match committed artifact digest")
	}
	r := tar.NewReader(bytes.NewReader(data))
	header, err := r.Next()
	if err != nil || header.Name != "candidate.txt" {
		t.Fatalf("physical archive lacks expected entry: %v", err)
	}
	content, err := io.ReadAll(r)
	if err != nil || string(content) != "candidate" {
		t.Fatalf("physical archive content mismatch: %v", err)
	}
	if _, err = r.Next(); err != io.EOF {
		t.Fatalf("unexpected archive entry: %v", err)
	}
	return data
}

func TestDeliveryEvidenceRouterActualPackageObservation(t *testing.T) {
	for _, mode := range []string{"legacy", "password"} {
		t.Run(mode, func(t *testing.T) {
			f := newDeliveryEvidenceRouterFixture(t, mode, true)
			archive, before := f.archive(t), f.snapshot(t)
			for range 2 {
				out := f.evaluate(t, f.token)
				if out.Verdict != "pass" || out.Reason != "evidence_verified" || out.Mode != "observe" || out.PolicyVersion != f.body["policy_version"] || out.IssueID != f.request.ResourceID || out.IssueRevision != f.body["expected_issue_revision"] || out.ProjectID != nil || out.ProjectRevision != nil {
					t.Fatalf("wrong observation context: %+v", out)
				}
				if out.ApprovalID != f.approval.Approval.ID || out.ExecutionID != f.approval.ExecutionID || out.SpecDigest != f.approval.SpecDigest || out.ArtifactDigest != f.body["expected_artifact_digest"] || out.ReceiptDigest != f.approval.ReceiptDigest || out.ProducerAgentID != f.request.AgentID || out.ProducerTaskID != f.request.TaskID || out.ProducerTaskGeneration != 1 {
					t.Fatalf("observation lost committed evidence identity: %+v", out)
				}
				if out.SourceRevision != f.request.SourceRevision || out.ObservedHeadSHA != f.request.SourceRevision || out.RevisionProvenance != "database_observed_pr_head_not_remote_verified" {
					t.Fatalf("source provenance incorrect: %+v", out)
				}
				observed, err := time.Parse(time.RFC3339Nano, out.ObservedAt)
				until, untilErr := time.Parse(time.RFC3339Nano, out.ValidUntil)
				if err != nil || untilErr != nil || !until.After(observed) {
					t.Fatalf("invalid observation horizon: %+v", out)
				}
			}
			if f.snapshot(t) != before || !bytes.Equal(f.archive(t), archive) {
				t.Fatal("repeated observation changed lifecycle, receipt, queue or archive")
			}
			f.fx.Exec(t, `UPDATE agent_task_queue SET status='completed' WHERE id=$1`, f.request.TaskID)
			if out := f.evaluate(t, f.token); out.Verdict != "pass" {
				t.Fatalf("completed producer task lost valid retained evidence: %+v", out)
			}
		})
	}
}

func TestDeliveryEvidenceRouterCurrentCandidateAndLegacyUpdate(t *testing.T) {
	f := newDeliveryEvidenceRouterFixture(t, "legacy", true)
	before := f.snapshot(t)
	f.fx.Exec(t, `UPDATE github_pull_request SET head_sha=$1 WHERE id=$2`, strings.Repeat("b", 40), f.pr)
	if out := f.evaluate(t, f.token); out.Verdict != "hold" || out.Reason != "source_revision_mismatch" {
		t.Fatalf("old SHA received green observation: %+v", out)
	}
	f.fx.Exec(t, `UPDATE issue_pull_request SET reference_only=true WHERE issue_id=$1 AND pull_request_id=$2`, f.request.ResourceID, f.pr)
	if out := f.evaluate(t, f.token); out.Verdict != "unknown" || out.Reason != "source_head_missing" {
		t.Fatalf("unobserved head received green observation: %+v", out)
	}
	if f.snapshot(t) != before {
		t.Fatal("negative observation changed lifecycle or retained receipt")
	}
	// Observe mode preserves the installed client's old status-update payload.
	f.call(t, f.token, "PUT", "/api/issues/"+f.request.ResourceID, map[string]string{"status": "done"}).Want(200)
	var status string
	f.fx.QueryRow(t, `SELECT status FROM issue WHERE id=$1`, f.request.ResourceID).Scan(&status)
	if status != "done" {
		t.Fatal("observe-only endpoint imposed a new completion policy")
	}
}

func TestDeliveryEvidenceRouterProjectAndIssueRevisionBinding(t *testing.T) {
	f := newDeliveryEvidenceRouterFixture(t, "legacy", true)
	project := f.fx.Project(t, "Delivery evidence project")
	f.fx.Exec(t, `UPDATE issue SET project_id=$1,revision=revision+1 WHERE id=$2`, project, f.request.ResourceID)
	f.call(t, f.token, "POST", f.path(), f.body).Want(409)
	var issueRevision, projectRevision int64
	f.fx.QueryRow(t, `SELECT revision FROM issue WHERE id=$1`, f.request.ResourceID).Scan(&issueRevision)
	f.fx.QueryRow(t, `SELECT revision FROM project WHERE id=$1`, project).Scan(&projectRevision)
	f.body["expected_issue_revision"], f.body["expected_project_id"], f.body["expected_project_revision"] = issueRevision, project, projectRevision
	if out := f.evaluate(t, f.token); out.Verdict != "pass" || out.ProjectID == nil || *out.ProjectID != project || out.ProjectRevision == nil || *out.ProjectRevision != projectRevision {
		t.Fatalf("current project context missing: %+v", out)
	}
	f.fx.Exec(t, `UPDATE project SET revision=revision+1 WHERE id=$1`, project)
	f.call(t, f.token, "POST", f.path(), f.body).Want(409)
}

func TestDeliveryEvidenceRouterRejectsMachineAndForgedIdentity(t *testing.T) {
	f := newDeliveryEvidenceRouterFixture(t, "legacy", true)
	before := f.snapshot(t)
	f.call(t, f.taskToken, "POST", f.path(), f.body, "X-Actor-Source", "member", "X-User-ID", f.user).Want(403)
	f.call(t, f.token, "POST", f.path(), f.body, "X-Agent-ID", f.request.AgentID).Want(403)
	f.call(t, f.token, "POST", f.path(), f.body, "X-Task-ID", f.request.TaskID).Want(403)
	unauthenticated := testutil.WithHeaders(testutil.JSONRequest("POST", f.path(), f.body), "X-User-ID", f.user, "X-Workspace-ID", testWorkspaceID)
	testutil.Call(t, f.router.ServeHTTP, unauthenticated).Want(401)
	outsider := f.fx.User(t, "Ordinary evidence reader", "evidence-"+uuid.NewString()+"@example.test")
	f.fx.Member(t, testWorkspaceID, outsider, "member")
	f.call(t, controlledArtifactJWT(t, outsider, 0, time.Now().Add(time.Hour)), "POST", f.path(), f.body).Want(403)
	foreign := f.fx.Workspace(t, "Foreign evidence", "foreign-"+uuid.NewString())
	f.fx.Member(t, foreign, f.user, "owner")
	f.call(t, f.token, "POST", f.path(), f.body, "X-Workspace-ID", foreign).Want(404)
	if f.snapshot(t) != before {
		t.Fatal("identity refusal changed retained evidence")
	}
}

func TestDeliveryEvidenceRouterCredentialRevocationAfterAuthentication(t *testing.T) {
	for _, kind := range []string{"password", "pat", "membership"} {
		t.Run(kind, func(t *testing.T) {
			mode := "legacy"
			if kind == "password" {
				mode = "password"
			}
			f := newDeliveryEvidenceRouterFixture(t, mode, true)
			token := f.token
			var patID string
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
				patID = f.fx.Insert(t, "personal_access_token", testutil.Cols{"user_id": f.user, "name": "Delivery evidence PAT", "token_hash": auth.HashToken(token), "token_prefix": token[:8], "expires_at": time.Now().Add(time.Hour)})
				revoke = func() { f.fx.Exec(t, `UPDATE personal_access_token SET revoked=true WHERE id=$1`, patID) }
			}
			control := testutil.WithHeaders(testutil.JSONRequest("POST", f.path(), f.body), "Authorization", "Bearer "+token, "X-Workspace-ID", testWorkspaceID)
			if patID != "" {
				// Real legacy authentication asynchronously updates last_used_at.
				// Its row lock legitimately conflicts with the evaluator's NOWAIT
				// authority lock. Observe that one fresh-token write committing
				// before running the positive control; do not retry a 409 as pass.
				control.Body = &artifactRevokeOnRead{ReadCloser: control.Body, revoke: func() {
					ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
					defer cancel()
					ticker := time.NewTicker(time.Millisecond)
					defer ticker.Stop()
					for {
						var used bool
						if err := testPool.QueryRow(ctx, `SELECT last_used_at IS NOT NULL FROM personal_access_token WHERE id=$1`, patID).Scan(&used); err != nil {
							t.Fatalf("observe authenticated PAT last-used commit: %v", err)
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
			var positive deliveryEvidenceRouterResponse
			testutil.Call(t, f.router.ServeHTTP, control).Want(200).JSON(&positive)
			if positive.Verdict != "pass" {
				t.Fatalf("valid credential control did not pass: %+v", positive)
			}
			before := f.snapshot(t)
			req := testutil.WithHeaders(testutil.JSONRequest("POST", f.path(), f.body), "Authorization", "Bearer "+token, "X-Workspace-ID", testWorkspaceID)
			revoked := false
			req.Body = &artifactRevokeOnRead{ReadCloser: req.Body, revoke: func() { revoke(); revoked = true }}
			want := http.StatusForbidden
			if kind == "pat" {
				want = http.StatusNotFound
			}
			testutil.Call(t, f.router.ServeHTTP, req).Want(want)
			if !revoked {
				t.Fatalf("%s probe did not run after real authentication", kind)
			}
			if f.snapshot(t) != before {
				t.Fatal("revoked observation changed lifecycle or receipt")
			}
		})
	}
}

func TestDeliveryEvidenceRouterMissingEvidenceAndMalformedContract(t *testing.T) {
	f := newDeliveryEvidenceRouterFixture(t, "legacy", false)
	before := f.snapshot(t)
	if out := f.evaluate(t, f.token); out.Verdict != "unknown" || out.Reason != "evidence_missing" {
		t.Fatalf("pending approval became evidence: %+v", out)
	}
	raw, err := json.Marshal(f.body)
	if err != nil {
		t.Fatal(err)
	}
	for _, malformed := range []string{
		string(raw[:len(raw)-1]) + `,"verdict":"pass"}`,
		string(raw[:len(raw)-1]) + `,"policy_version":"delivery-artifact-observe-v1"}`,
		strings.Replace(string(raw), `"approval_id"`, `"Approval_ID"`, 1),
		strings.Replace(string(raw), `"expected_project_id":null`, `"expected_project_id":"`+uuid.NewString()+`"`, 1),
		string(raw) + `{}`,
	} {
		f.call(t, f.token, "POST", f.path(), malformed).Want(400)
	}
	if f.snapshot(t) != before {
		t.Fatal("missing or malformed evidence mutated lifecycle")
	}
	f.noClaim(t, f.approval.Approval.ID)
}
