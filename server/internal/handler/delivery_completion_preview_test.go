package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
)

func completionPreviewRequest(f *completionFixture, body any) *http.Request {
	id := f.request.Target.IssueID
	return testutil.WithURLParams(f.artifactFixture.request("POST", "/api/issues/"+id+"/delivery-evidence/completion-preview", body), "id", id)
}
func completionPreviewBody(f *completionFixture) map[string]any {
	return map[string]any{"expected_issue_revision": f.request.Target.ExpectedIssueRevision, "target_status": "done", "approval_id": f.request.Evidence.ApprovalID}
}
func completionCandidatesRequest(f *completionFixture, suffix string) *http.Request {
	id := f.request.Target.IssueID
	return testutil.WithURLParams(f.artifactFixture.request("GET", "/api/issues/"+id+"/delivery-evidence/candidates"+suffix, nil), "id", id)
}

func TestDeliveryCompletionPreviewCurrentBindingsWithoutWrites(t *testing.T) {
	f := newCompletionFixture(t)
	before := deliveryHintSnapshot(t, f.deliveryPolicyFixture)
	var out service.DeliveryCompletionPreview
	result := testutil.Call(t, f.h.GetDeliveryCompletionPreview, completionPreviewRequest(f, completionPreviewBody(f))).Want(200).JSON(&out)
	if result.Header().Get("Cache-Control") != "no-store" || out.ProtocolVersion != 1 || out.AuthorizationBinding == nil || out.Observation == nil || out.Observation.Verdict != "pass" || out.MaxExpiresAt == nil || out.Enforced || out.UsableForCompletion || out.SkipSupported {
		t.Fatalf("invalid preview: %+v", out)
	}
	b := out.AuthorizationBinding
	if b.Target.IssueID != f.request.Target.IssueID || b.Target.ExpectedIssueRevision != f.request.Target.ExpectedIssueRevision || b.Target.TargetPosition != nil || b.Project != f.request.Project || b.Evidence != f.request.Evidence || b.Consumer != f.request.Consumer {
		t.Fatalf("incorrect binding: %+v", b)
	}
	if out.MaxExpiresAt.After(*out.Observation.ValidUntil) || !out.MaxExpiresAt.After(out.CheckedAt) {
		t.Fatal("invalid expiry horizon")
	}
	if deliveryHintSnapshot(t, f.deliveryPolicyFixture) != before || dbfx.Count(t, `SELECT count(*) FROM delivery_completion_authorization WHERE issue_id=$1`, f.request.Target.IssueID) != 0 {
		t.Fatal("preview wrote business facts")
	}
}

func TestDeliveryCompletionPreviewBranchesAndCurrentRefusal(t *testing.T) {
	for _, tc := range []string{"custom_done", "custom_archived", "already_done", "non_done", "off", "observe", "no_project", "stale", "missing_head", "different_head", "authority", "credential", "other_issue"} {
		t.Run(tc, func(t *testing.T) {
			f := newCompletionFixture(t)
			body := completionPreviewBody(f)
			status := 200
			reason := "authorization_available"
			switch tc {
			case "custom_done":
				deliveryHintCustomStatus(t, f.deliveryPolicyFixture, "accepted_preview", "done")
				body["target_status"] = " ACCEPTED_PREVIEW "
			case "custom_archived":
				id := deliveryHintCustomStatus(t, f.deliveryPolicyFixture, "archived_preview", "done")
				dbfx.Exec(t, `UPDATE issue_status SET archived_at=clock_timestamp() WHERE id=$1`, id)
				body["target_status"] = "archived_preview"
				status = 400
			case "already_done":
				dbfx.Exec(t, `UPDATE issue SET status='done' WHERE id=$1`, f.request.Target.IssueID)
				reason = "already_done"
			case "non_done":
				body["target_status"] = "in_review"
				reason = "target_not_done"
			case "off", "observe":
				dbfx.Exec(t, `INSERT INTO project_delivery_policy(workspace_id,project_id,revision,policy_version,mode,stage,reason,created_by) VALUES($1,$2,2,$3,$4,'done','preview test',$5)`, f.ws, f.project, deliveryEvidencePolicy, tc, f.actor)
				reason = "policy_" + tc
			case "no_project":
				dbfx.Exec(t, `UPDATE issue SET project_id=NULL WHERE id=$1`, f.request.Target.IssueID)
				reason = "no_project"
			case "stale":
				body["expected_issue_revision"] = f.request.Target.ExpectedIssueRevision + 1
				status = 409
			case "missing_head":
				dbfx.Exec(t, `DELETE FROM issue_pull_request WHERE issue_id=$1`, f.request.Target.IssueID)
				reason = "source_head_missing"
			case "different_head":
				dbfx.Exec(t, `UPDATE github_pull_request SET head_sha=$1 WHERE id=$2`, strings.Repeat("b", 40), f.pr)
				reason = "source_revision_mismatch"
			case "authority":
				dbfx.Exec(t, `UPDATE agent SET autonomy_level='contributor' WHERE id=$1`, f.artifactFixture.req.AgentID)
				status = 403
			case "credential":
				status = 403
			case "other_issue":
				other := newCompletionFixture(t)
				body["approval_id"] = other.request.Evidence.ApprovalID
				status = 404
			}
			req := completionPreviewRequest(f, body)
			if tc == "credential" {
				req = req.WithContext(auth.WithAuthenticatedPrincipal(req.Context(), auth.AuthenticatedPrincipal{UserID: f.actor, Kind: auth.CredentialJWT, ExpiresAt: time.Now().Add(-time.Minute)}))
			}
			var out service.DeliveryCompletionPreview
			testutil.Call(t, f.h.GetDeliveryCompletionPreview, req).Want(status).JSON(&out)
			if status == 200 && (out.Reason != reason || (out.AuthorizationBinding != nil) != (reason == "authorization_available")) {
				t.Fatalf("%s: %+v", tc, out)
			}
		})
	}
}

func TestDeliveryCompletionPreviewCredentialHorizon(t *testing.T) {
	f := newCompletionFixture(t)
	expires := time.Now().UTC().Add(time.Minute).Truncate(time.Microsecond)
	req := completionPreviewRequest(f, completionPreviewBody(f))
	req = req.WithContext(auth.WithAuthenticatedPrincipal(req.Context(), auth.AuthenticatedPrincipal{UserID: f.actor, Kind: auth.CredentialJWT, ExpiresAt: expires}))
	var out service.DeliveryCompletionPreview
	testutil.Call(t, f.h.GetDeliveryCompletionPreview, req).Want(200).JSON(&out)
	if out.MaxExpiresAt == nil || !out.MaxExpiresAt.Equal(expires) {
		t.Fatalf("credential horizon ignored: %+v", out)
	}
	raw, _ := json.Marshal(out)
	for _, private := range []string{"evaluation_key", "evaluator", "issuer_pat", "credential_kind", "password_version", "request_key"} {
		if strings.Contains(string(raw), private) {
			t.Fatal("private metadata exposed", private)
		}
	}
}

func TestDeliveryCompletionCandidatesPaginationAndAuthority(t *testing.T) {
	f := newCompletionFixture(t)
	// Two independently approved, actually packaged executions of the same issue.
	f.artifactFixture.key = uuid.NewString()
	second := f.artifactFixture.create(t)
	f.approve(t, second)
	f.execute(t, second, 200)
	before := deliveryHintSnapshot(t, f.deliveryPolicyFixture)
	var page service.DeliveryEvidenceCandidates
	testutil.Call(t, f.h.ListDeliveryEvidenceCandidates, completionCandidatesRequest(f, "?limit=1")).Want(200).JSON(&page)
	if len(page.Items) != 1 || !page.HasMore || page.NextCursor == nil {
		t.Fatalf("missing complete visible pagination: %+v", page)
	}
	var next service.DeliveryEvidenceCandidates
	testutil.Call(t, f.h.ListDeliveryEvidenceCandidates, completionCandidatesRequest(f, "?limit=1&cursor="+*page.NextCursor)).Want(200).JSON(&next)
	if len(next.Items) != 1 || next.HasMore || next.NextCursor != nil || next.Items[0].ApprovalID == page.Items[0].ApprovalID {
		t.Fatalf("pagination duplicated/lost candidate: %+v", next)
	}
	other := dbfx.User(t, "Preview reader", "preview-reader-"+uuid.NewString()+"@example.invalid")
	dbfx.Member(t, f.ws, other, "member")
	req := completionCandidatesRequest(f, "?limit=1")
	req.Header.Set("X-User-ID", other)
	req = req.WithContext(auth.WithAuthenticatedPrincipal(req.Context(), auth.AuthenticatedPrincipal{UserID: other, Kind: auth.CredentialJWT, ExpiresAt: time.Now().Add(time.Hour)}))
	var invisible service.DeliveryEvidenceCandidates
	response := testutil.Call(t, f.h.ListDeliveryEvidenceCandidates, req).Want(200).JSON(&invisible)
	if len(invisible.Items) != 0 || invisible.HasMore || invisible.NextCursor != nil {
		t.Fatalf("hidden candidates leaked: %+v", invisible)
	}
	if strings.Contains(response.Text(), second.Approval.ID) {
		t.Fatal("hidden ID exposed")
	}
	dbfx.Exec(t, `UPDATE issue SET revision=revision+1 WHERE id=$1`, f.request.Target.IssueID)
	testutil.Call(t, f.h.ListDeliveryEvidenceCandidates, completionCandidatesRequest(f, "?limit=1&cursor="+*page.NextCursor)).Want(409)
	dbfx.Exec(t, `UPDATE issue SET revision=revision-1 WHERE id=$1`, f.request.Target.IssueID)
	if deliveryHintSnapshot(t, f.deliveryPolicyFixture) != before {
		t.Fatal("listing changed business facts")
	}
}

func TestDeliveryCompletionPreviewStrictInputsAndReadCommitFailure(t *testing.T) {
	f := newCompletionFixture(t)
	for _, body := range []any{map[string]any{}, map[string]any{"expected_issue_revision": 0, "target_status": "done", "approval_id": uuid.NewString(), "evaluation_key": nil}, map[string]any{"expected_issue_revision": -1, "target_status": "done", "approval_id": uuid.NewString()}, map[string]any{"expected_issue_revision": 0, "target_status": "done\x00", "approval_id": uuid.NewString()}} {
		testutil.Call(t, f.h.GetDeliveryCompletionPreview, completionPreviewRequest(f, body)).Want(400)
	}
	for _, query := range []string{"?limit=0", "?limit=51", "?limit=1&limit=2", "?cursor=invalid", "?unknown=1"} {
		testutil.Call(t, f.h.ListDeliveryEvidenceCandidates, completionCandidatesRequest(f, query)).Want(400)
	}
	tx, err := testPool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	// The held issue lock must be a refusal, never a successful preview.
	if _, err = tx.Exec(context.Background(), `SELECT id FROM issue WHERE id=$1 FOR UPDATE`, f.request.Target.IssueID); err != nil {
		t.Fatal(err)
	}
	testutil.Call(t, f.h.GetDeliveryCompletionPreview, completionPreviewRequest(f, completionPreviewBody(f))).Want(409)
}

func TestDeliveryCompletionPreviewIncompletePackageIsNotAuthorization(t *testing.T) {
	f := newCompletionFixture(t)
	f.artifactFixture.key = uuid.NewString()
	unclaimed := f.artifactFixture.create(t)
	body := completionPreviewBody(f)
	body["approval_id"] = unclaimed.Approval.ID
	var out service.DeliveryCompletionPreview
	testutil.Call(t, f.h.GetDeliveryCompletionPreview, completionPreviewRequest(f, body)).Want(200).JSON(&out)
	if out.Observation == nil || out.Observation.Verdict != "unknown" || out.Reason != "evidence_missing" || out.AuthorizationBinding != nil || out.MaxExpiresAt != nil {
		t.Fatalf("incomplete package became a grant: %+v", out)
	}
	if out.Observation.ArtifactDigest != "" {
		t.Fatal("fabricated missing artifact digest")
	}
}

func TestDeliveryCompletionCandidatesBoundedHiddenScan(t *testing.T) {
	f := newCompletionFixture(t)
	// Real unclaimed declarations are deliberately invisible to this member;
	// they are not seeded successful artifacts and never invoke a Provider.
	for i := 0; i < 200; i++ {
		f.artifactFixture.key = uuid.NewString()
		f.artifactFixture.create(t)
	}
	other := dbfx.User(t, "Bounded preview reader", "preview-bound-"+uuid.NewString()+"@example.invalid")
	dbfx.Member(t, f.ws, other, "member")
	req := completionCandidatesRequest(f, "?limit=1")
	req.Header.Set("X-User-ID", other)
	req = req.WithContext(auth.WithAuthenticatedPrincipal(req.Context(), auth.AuthenticatedPrincipal{UserID: other, Kind: auth.CredentialJWT, ExpiresAt: time.Now().Add(time.Hour)}))
	response := testutil.Call(t, f.h.ListDeliveryEvidenceCandidates, req).Want(503)
	if strings.Contains(response.Text(), "items") || strings.Contains(response.Text(), "cursor") || strings.Contains(response.Text(), f.request.Evidence.ApprovalID) {
		t.Fatal("scan exhaustion leaked hidden page")
	}
}
