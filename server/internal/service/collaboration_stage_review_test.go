package service

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
)

type stageReviewFixture struct {
	*collaborationStageFixture
	child, pr, head string
	artifact        *CollaborationStageArtifactSelection
}

func newStageReviewFixture(t *testing.T) *stageReviewFixture {
	t.Helper()
	f := newCollaborationStageFixture(t)
	principal := func(user string) context.Context {
		return auth.WithAuthenticatedPrincipal(context.Background(), auth.AuthenticatedPrincipal{UserID: user, Kind: auth.CredentialJWT, ExpiresAt: time.Now().Add(time.Hour)})
	}
	f.ctx, f.handlerCtx, f.reviewerCtx, f.qaCtx, f.releaseCtx = principal(f.creator), principal(f.handler), principal(f.reviewer), principal(f.qa), principal(f.release)
	// The designated reviewer explicitly has source access. Participation alone
	// is deliberately insufficient to read/approve private source evidence.
	f.fx.Member(t, f.workspace, f.reviewer, "member")
	child := f.fx.Issue(t, "Synthetic reviewed child", testutil.Cols{"parent_issue_id": f.issue.Issue.ID, "project_id": f.project})
	f.revise(t, true)
	f.configure(t)
	item := stageChildView(t, f.view(t), child)
	f.confirmScope(t, CollaborationStageScopeChild{IssueID: util.MustParseUUID(child), ContentDigest: stageReadableDigest(t, item)})
	head := strings.Repeat("b", 40)
	pr := f.githubPR(t, child, head, false)
	f.fx.Insert(t, "github_installation", testutil.Cols{"workspace_id": f.workspace, "installation_id": int64(1), "account_login": "synthetic-stage", "account_type": "User"})
	f.fx.Exec(t, `UPDATE github_pull_request SET snapshot_head_sha=head_sha,snapshot_fetched_at=clock_timestamp()-interval '1 minute',snapshot_complete=true WHERE id=$1`, pr)
	f.fx.InsertNoID(t, "github_pull_request_check_run", testutil.Cols{"pr_id": pr, "head_sha": head, "ordinal": int32(0), "name": "synthetic unit tests", "status": "completed", "conclusion": "success"}, "pr_id=$1", pr)
	return &stageReviewFixture{collaborationStageFixture: f, child: child, pr: pr, head: head}
}

func (f *stageReviewFixture) code(t *testing.T, supersedes *CollaborationStageCandidateRef) CollaborationStageCandidateRef {
	t.Helper()
	r, err := f.svc.CreateStageCodeCandidate(f.ctx, CollaborationStageCodeParams{CollaborationWorkflowRequest: f.req(t, f.ctx), ExpectedPolicyRevision: f.policyRevision,
		ExpectedRoleRevision: f.stageInt(t, `SELECT role_revision FROM collaboration_stage_task WHERE issue_id=$1`), ChildIssueID: util.MustParseUUID(f.child), Supersedes: supersedes,
		PullRequest: CollaborationStageCodeRef{Provider: "github", PullRequestID: util.MustParseUUID(f.pr), HeadSHA: f.head}, Artifact: f.artifact})
	if err != nil || r.StageConfirmation == nil {
		t.Fatalf("create code candidate: %+v %v", r, err)
	}
	return stageRef(*r.StageConfirmation)
}

func (f *stageReviewFixture) advanceParams(t *testing.T, c CollaborationStageCandidateRef, target string, decisions ...string) CollaborationStageAdvanceParams {
	t.Helper()
	return CollaborationStageAdvanceParams{CollaborationWorkflowRequest: f.req(t, f.ctx), ExpectedPolicyRevision: f.policyRevision,
		ExpectedRoleRevision: f.stageInt(t, `SELECT role_revision FROM collaboration_stage_task WHERE issue_id=$1`), ExpectedStageRevision: f.stageInt(t, `SELECT stage_revision FROM collaboration_stage_task WHERE issue_id=$1`),
		TargetStage: target, Candidate: c, DecisionIDs: append([]string{}, decisions...)}
}

func (f *stageReviewFixture) evidence(t *testing.T, ctx context.Context, c CollaborationStageCandidateRef) CollaborationStageCodeEvidenceView {
	t.Helper()
	view, err := f.svc.StageCodeEvidence(ctx, f.issue.Issue.ID, c.CandidateID)
	if err != nil {
		t.Fatal(err)
	}
	return view
}

func (f *stageReviewFixture) reviewParams(t *testing.T, c CollaborationStageCandidateRef, decision string) CollaborationStageReviewParams {
	t.Helper()
	view := f.evidence(t, f.reviewerCtx, c)
	p := CollaborationStageReviewParams{CollaborationWorkflowRequest: f.req(t, f.reviewerCtx), ExpectedPolicyRevision: f.policyRevision, ExpectedRoleRevision: f.stageInt(t, `SELECT role_revision FROM collaboration_stage_task WHERE issue_id=$1`),
		ExpectedStageRevision: f.stageInt(t, `SELECT stage_revision FROM collaboration_stage_task WHERE issue_id=$1`), Step: "code_review", Decision: decision, Candidate: c, DependsOn: []string{}, Reason: "Synthetic human review"}
	if decision == "approve" {
		expires := view.CheckedAt.Add(5 * time.Minute).Truncate(time.Microsecond)
		p.ExpiresAt = &expires
		p.Evidence.CIDigest = view.Observed.CI.CIDigest
	}
	return p
}

func (f *stageReviewFixture) approved(t *testing.T) (CollaborationStageCandidateRef, CollaborationStageReviewParams, CollaborationWorkflowReceipt) {
	t.Helper()
	c := f.code(t, nil)
	if _, err := f.svc.AdvanceStage(f.ctx, f.advanceParams(t, c, "code_review")); err != nil {
		t.Fatal(err)
	}
	p := f.reviewParams(t, c, "approve")
	r, err := f.svc.DecideStage(f.reviewerCtx, p)
	if err != nil {
		t.Fatal(err)
	}
	return c, p, r
}

func TestStageCodeReviewTransitionsAndOriginalKey(t *testing.T) {
	f := newStageReviewFixture(t)
	c := f.code(t, nil)
	discovery := f.view(t).CodeCandidates
	if len(discovery.Items) != 1 || discovery.Items[0].CandidateID != util.UUIDToString(c.CandidateID) || discovery.Items[0].State != "current" {
		t.Fatalf("discovery: %+v", discovery)
	}
	if _, err := f.svc.AdvanceStage(f.ctx, f.advanceParams(t, c, "code_review")); err != nil {
		t.Fatal(err)
	}
	view := f.evidence(t, f.reviewerCtx, c)
	if view.Candidate.Baseline == nil || view.Observed.CI.State != "passed" || view.Observed.CI.CIDigest == nil || !view.Signable[0].Allowed || view.Signable[0].MaxExpiresAt == nil ||
		view.Signable[0].MaxExpiresAt.After(view.Observed.CI.ObservedAt.Add(stageCIHorizon)) {
		t.Fatalf("review evidence/horizon: %+v", view)
	}
	if _, err := f.svc.AdvanceStage(f.ctx, f.advanceParams(t, c, "qa")); !collaborationErrorCode(err, "stage_dependency_invalid") {
		t.Fatalf("QA without human approval: %v", err)
	}
	p := f.reviewParams(t, c, "approve")
	r, err := f.svc.DecideStage(f.reviewerCtx, p)
	if err != nil {
		t.Fatal(err)
	}
	if r.StageConfirmation.Action != "decide_stage" || r.StageConfirmation.Decision != "approved" || r.StageConfirmation.CandidateID != util.UUIDToString(c.CandidateID) {
		t.Fatalf("review receipt: %+v", r)
	}
	advance := f.advanceParams(t, c, "qa", r.StageConfirmation.DecisionID)
	if _, err = f.svc.AdvanceStage(f.ctx, advance); err != nil {
		t.Fatal(err)
	}
	before := f.view(t)
	if replay, err := f.svc.DecideStage(f.reviewerCtx, p); err != nil || replay.StageConfirmation.DecisionID != r.StageConfirmation.DecisionID {
		t.Fatalf("original decision replay: %+v %v", replay, err)
	}
	if replay, err := f.svc.GetWorkflowOperation(f.reviewerCtx, p.IssueID, p.RequestID); err != nil || replay.StageConfirmation.DecisionID != r.StageConfirmation.DecisionID {
		t.Fatalf("original decision recovery: %+v %v", replay, err)
	}
	p.Reason = "Changed intent"
	if _, err = f.svc.DecideStage(f.reviewerCtx, p); !collaborationErrorCode(err, "idempotency_conflict") {
		t.Fatalf("same key changed payload: %v", err)
	}
	if after := f.view(t); after.Stage.Current != "qa" || after.CollaborationRevision != before.CollaborationRevision {
		t.Fatalf("recovery mutated stage: %+v", after)
	}
	cancel := lifecycleParams(t, f.collaborationQueueFixture, "cancel", "cancelled", 0, 1)
	cancel.CollaborationWorkflowRequest = f.req(t, f.ctx)
	if _, err = f.svc.ChangeLifecycle(f.ctx, cancel); err != nil {
		t.Fatal(err)
	}
	reopen := lifecycleParams(t, f.collaborationQueueFixture, "reopen", "todo", 0, 1)
	reopen.CollaborationWorkflowRequest = f.req(t, f.ctx)
	if _, err = f.svc.ChangeLifecycle(f.ctx, reopen); err != nil {
		t.Fatal(err)
	}
	if current := f.view(t); current.Stage.Current != "development" || !current.NeedsConfirmation || current.CodeCandidates.Items[0].State != "stale" {
		t.Fatalf("reopen retained approval applicability: %+v", current)
	}
}

func TestStageCodeReviewStrictCIRefusals(t *testing.T) {
	cases := []struct{ name, sql, code string }{
		{"missing", `DELETE FROM github_pull_request_check_run WHERE pr_id=$1`, "stage_ci_missing"},
		{"legacy_incomplete", `UPDATE github_pull_request SET snapshot_complete=false WHERE id=$1`, "stage_ci_missing"},
		{"old_head", `UPDATE github_pull_request SET snapshot_head_sha='old-head' WHERE id=$1`, "stage_ci_missing"},
		{"stale", `UPDATE github_pull_request SET snapshot_fetched_at=clock_timestamp()-interval '31 minutes' WHERE id=$1`, "stage_ci_stale"},
		{"future", `UPDATE github_pull_request SET snapshot_fetched_at=clock_timestamp()+interval '1 minute' WHERE id=$1`, "stage_ci_stale"},
		{"running", `UPDATE github_pull_request_check_run SET status='in_progress',conclusion=NULL WHERE pr_id=$1`, "stage_ci_failed"},
		{"skipped", `UPDATE github_pull_request_check_run SET conclusion='skipped' WHERE pr_id=$1`, "stage_ci_failed"},
		{"neutral", `UPDATE github_pull_request_check_run SET conclusion='neutral' WHERE pr_id=$1`, "stage_ci_failed"},
		{"failure", `UPDATE github_pull_request_check_run SET conclusion='failure' WHERE pr_id=$1`, "stage_ci_failed"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newStageReviewFixture(t)
			c := f.code(t, nil)
			f.fx.Exec(t, tc.sql, f.pr)
			if _, err := f.svc.AdvanceStage(f.ctx, f.advanceParams(t, c, "code_review")); !collaborationErrorCode(err, tc.code) {
				t.Fatalf("strict CI want %s: %v", tc.code, err)
			}
			if f.view(t).Stage.Current != "development" {
				t.Fatal("refused CI advanced stage")
			}
		})
	}
}

func TestStageCodeReviewDigestAndSourceDrift(t *testing.T) {
	for _, kind := range []string{"head", "checks", "trust", "child", "requirement"} {
		t.Run(kind, func(t *testing.T) {
			f := newStageReviewFixture(t)
			c, _, r := f.approved(t)
			before := f.evidence(t, f.reviewerCtx, c)
			f.fx.Exec(t, `UPDATE github_pull_request SET snapshot_fetched_at=clock_timestamp() WHERE id=$1`, f.pr)
			if refreshed := f.evidence(t, f.reviewerCtx, c); *refreshed.Observed.CI.CIDigest != *before.Observed.CI.CIDigest || refreshed.Decisions[0].State != "valid" {
				t.Fatal("refresh time changed content identity")
			}
			switch kind {
			case "head":
				f.fx.Exec(t, `UPDATE github_pull_request SET head_sha=$2 WHERE id=$1`, f.pr, strings.Repeat("c", 40))
			case "checks":
				f.fx.Exec(t, `UPDATE github_pull_request_check_run SET conclusion='failure' WHERE pr_id=$1`, f.pr)
			case "trust":
				f.fx.Exec(t, `UPDATE github_installation SET trust_revision=trust_revision+1 WHERE workspace_id=$1`, f.workspace)
			case "child":
				f.fx.Exec(t, `UPDATE collaboration_stage_child SET semantic_version=semantic_version+1 WHERE child_issue_id=$1`, f.child)
			case "requirement":
				f.fx.Exec(t, `UPDATE issue SET title='Changed reviewed requirement' WHERE id=$1`, f.issue.Issue.ID)
			}
			if _, err := f.svc.AdvanceStage(f.ctx, f.advanceParams(t, c, "qa", r.StageConfirmation.DecisionID)); !collaborationErrorCode(err, "stage_evidence_changed") {
				t.Fatalf("old approval admitted changed %s: %v", kind, err)
			}
			if f.view(t).Stage.Current != "code_review" {
				t.Fatal("changed evidence advanced QA")
			}
		})
	}
}

func TestStageCodeReviewReturnAndSourcePrivacy(t *testing.T) {
	f := newStageReviewFixture(t)
	c, _, r := f.approved(t)
	f.fx.Exec(t, `UPDATE github_pull_request_check_run SET conclusion='failure' WHERE pr_id=$1`, f.pr)
	view := f.evidence(t, f.reviewerCtx, c)
	if view.Signable[0].Allowed {
		t.Fatal("failed CI remains approvable")
	}
	p := f.reviewParams(t, c, "return")
	p.DependsOn = []string{r.StageConfirmation.DecisionID}
	returned, err := f.svc.DecideStage(f.reviewerCtx, p)
	if err != nil || returned.StageConfirmation.Decision != "returned" || returned.StageConfirmation.ExpiresAt != nil {
		t.Fatalf("human return failed: %+v %v", returned, err)
	}
	if _, err = f.svc.AdvanceStage(f.ctx, f.advanceParams(t, c, "development", returned.StageConfirmation.DecisionID)); err != nil {
		t.Fatal(err)
	}
	private := f.evidence(t, f.handlerCtx, c)
	if private.Candidate.Baseline != nil || private.BlockedEvidence != 1 || len(private.Decisions) != 0 || len(private.TestReports) != 0 || len(private.ExecutionFacts) != 0 || private.Observed.CI.CIDigest != nil || private.Observed.PullRequest.HeadSHA != nil {
		t.Fatalf("external source projection leaked: %+v", private)
	}
	for _, hint := range private.Signable {
		if hint.Allowed || hint.MaxExpiresAt != nil {
			t.Fatalf("external source signable: %+v", hint)
		}
		for _, b := range hint.Blockers {
			if b.CandidateID != "" || b.ChildIssueID != "" || b.DecisionID != "" || !stageTarget(b.Stage) {
				t.Fatalf("private/invalid blocker: %+v", b)
			}
		}
	}
}

func TestStageCodeReviewSignerRevocationAndExpiryBounds(t *testing.T) {
	for _, kind := range []string{"participant", "account", "pat", "password", "source_access"} {
		t.Run(kind, func(t *testing.T) {
			f := newStageReviewFixture(t)
			var pat string
			if kind == "pat" {
				hash := strings.Repeat("c", 64)
				pat = f.fx.Insert(t, "personal_access_token", testutil.Cols{"user_id": f.reviewer, "name": "Synthetic stage PAT", "token_hash": hash, "token_prefix": "synthetic", "expires_at": time.Now().Add(time.Hour)})
				f.reviewerCtx = auth.WithAuthenticatedPrincipal(context.Background(), auth.AuthenticatedPrincipal{UserID: f.reviewer, Kind: auth.CredentialPAT, PATTokenHash: hash})
			}
			if kind == "password" {
				f.fx.InsertNoID(t, "user_password_credential", testutil.Cols{"user_id": f.reviewer, "username": "stage-" + f.reviewer, "password_hash": "synthetic-unused-hash", "session_version": int64(1)}, "user_id=$1", f.reviewer)
				f.reviewerCtx = auth.WithPasswordSession(f.reviewerCtx, auth.PasswordSession{UserID: f.reviewer, Kind: "jwt", Version: 1, ExpiresAt: time.Now().Add(time.Hour)})
			}
			c, _, r := f.approved(t)
			switch kind {
			case "participant":
				f.fx.Exec(t, `UPDATE issue_participant SET epoch=epoch+2 WHERE issue_id=$1 AND user_id=$2`, f.issue.Issue.ID, f.reviewer)
			case "account":
				f.fx.Exec(t, `UPDATE collaboration_account_fence SET epoch=epoch+1 WHERE user_id=$1`, f.reviewer)
			case "pat":
				f.fx.Exec(t, `UPDATE personal_access_token SET revoked=true WHERE id=$1`, pat)
			case "password":
				f.fx.Exec(t, `UPDATE user_password_credential SET session_version=session_version+1 WHERE user_id=$1`, f.reviewer)
			case "source_access":
				f.fx.Exec(t, `DELETE FROM member WHERE workspace_id=$1 AND user_id=$2`, f.workspace, f.reviewer)
			}
			if _, err := f.svc.AdvanceStage(f.ctx, f.advanceParams(t, c, "qa", r.StageConfirmation.DecisionID)); !collaborationErrorCode(err, "stage_signer_changed") {
				t.Fatalf("changed signer admitted: %v", err)
			}
		})
	}
	t.Run("limit_and_expired", func(t *testing.T) {
		f := newStageReviewFixture(t)
		c := f.code(t, nil)
		if _, err := f.svc.AdvanceStage(f.ctx, f.advanceParams(t, c, "code_review")); err != nil {
			t.Fatal(err)
		}
		p := f.reviewParams(t, c, "approve")
		tooLate := time.Now().Add(2 * time.Hour)
		p.ExpiresAt = &tooLate
		if _, err := f.svc.DecideStage(f.reviewerCtx, p); !collaborationErrorCode(err, "stage_expiry_exceeds_limit") {
			t.Fatalf("unbounded approval: %v", err)
		}
		if f.fx.Count(t, `SELECT count(*) FROM collaboration_operation WHERE scope_id=$1 AND request_id=$2`, p.IssueID, p.RequestID) != 0 {
			t.Fatal("invalid expiry was stored")
		}
		p = f.reviewParams(t, c, "approve")
		soon := time.Now().Add(2 * time.Second).UTC().Truncate(time.Microsecond)
		p.ExpiresAt = &soon
		r, err := f.svc.DecideStage(f.reviewerCtx, p)
		if err != nil {
			t.Fatal(err)
		}
		time.Sleep(time.Until(soon) + 20*time.Millisecond)
		if _, err = f.svc.AdvanceStage(f.ctx, f.advanceParams(t, c, "qa", r.StageConfirmation.DecisionID)); !collaborationErrorCode(err, "stage_decision_expired") {
			t.Fatalf("expired approval admitted: %v", err)
		}
	})
}

func TestStageCodeCandidateVersionsAndReviewerIsolation(t *testing.T) {
	f := newStageReviewFixture(t)
	first := f.code(t, nil)
	second := f.code(t, &first)
	if second.Version != first.Version+1 || second.CandidateID == first.CandidateID {
		t.Fatal("code revision reused candidate identity")
	}
	view := f.view(t)
	if len(view.CodeCandidates.Items) != 2 || view.CodeCandidates.Items[0].State != "current" || view.CodeCandidates.Items[1].State != "superseded" {
		t.Fatalf("version discovery: %+v", view.CodeCandidates)
	}
	if _, err := f.svc.AdvanceStage(f.ctx, f.advanceParams(t, first, "code_review")); !collaborationErrorCode(err, "stage_candidate_changed") {
		t.Fatalf("superseded candidate advanced: %v", err)
	}
	if _, err := f.svc.AdvanceStage(f.ctx, f.advanceParams(t, second, "code_review")); err != nil {
		t.Fatal(err)
	}
	p := f.reviewParams(t, second, "approve")
	p.CollaborationWorkflowRequest = f.req(t, f.ctx)
	if _, err := f.svc.DecideStage(f.ctx, p); !collaborationErrorCode(err, "stage_role_required") {
		t.Fatalf("creator inherited code review role: %v", err)
	}
	if _, err := f.svc.DecideStage(f.machine, p); !collaborationErrorCode(err, "forbidden") {
		t.Fatalf("machine signed review: %v", err)
	}
	hinted := auth.WithAuthenticatedPrincipal(context.Background(), auth.AuthenticatedPrincipal{UserID: f.reviewer, Kind: auth.CredentialJWT, ExpiresAt: time.Now().Add(time.Hour), AgentActor: true})
	if _, err := f.svc.DecideStage(hinted, p); !collaborationErrorCode(err, "forbidden") {
		t.Fatalf("agent actor borrowed human review authority: %v", err)
	}
	// A legitimate ordinary status change is not a content identity change.
	f.fx.Exec(t, `UPDATE issue SET status='in_progress',revision=revision+1 WHERE id=$1`, f.child)
	if current := f.evidence(t, f.reviewerCtx, second); current.Candidate.State != "current" {
		t.Fatalf("ordinary status invalidated code: %+v", current.Candidate)
	}
	roles := f.roleSet()
	roles.Implementer = util.MustParseUUID(f.qa)
	roles.CodeReviewer = util.MustParseUUID(f.handler)
	if _, err := f.svc.ConfigureStageRoles(f.ctx, CollaborationStageRolesParams{CollaborationWorkflowRequest: f.req(t, f.ctx), ExpectedPolicyRevision: 1, ExpectedRoleRevision: 1, Roles: roles}); err != nil {
		t.Fatal(err)
	}
	f.fx.Member(t, f.workspace, f.handler, "member")
	p = f.reviewParams(t, second, "return")
	p.CollaborationWorkflowRequest = f.req(t, f.handlerCtx)
	if _, err := f.svc.DecideStage(f.handlerCtx, p); !collaborationErrorCode(err, "stage_self_review_forbidden") {
		t.Fatalf("original implementer reviewed own candidate after reassignment: %v", err)
	}
}

func TestStageCodeCandidateRejectsUnboundInputs(t *testing.T) {
	for _, kind := range []string{"pull_request", "head", "child", "artifact"} {
		t.Run(kind, func(t *testing.T) {
			f := newStageReviewFixture(t)
			p := CollaborationStageCodeParams{CollaborationWorkflowRequest: f.req(t, f.ctx), ExpectedPolicyRevision: 1, ExpectedRoleRevision: 1, ChildIssueID: util.MustParseUUID(f.child),
				PullRequest: CollaborationStageCodeRef{Provider: "github", PullRequestID: util.MustParseUUID(f.pr), HeadSHA: f.head}}
			want := ""
			switch kind {
			case "pull_request":
				p.PullRequest.PullRequestID = p.ChildIssueID
				want = "stage_pull_request_unavailable"
			case "head":
				p.PullRequest.HeadSHA = strings.Repeat("c", 40)
				want = "stage_head_changed"
			case "child":
				p.ChildIssueID = util.MustParseUUID(f.creator)
				want = "stage_child_unconfirmed"
			case "artifact":
				p.Artifact = &CollaborationStageArtifactSelection{ApprovalID: util.MustParseUUID(f.pr)}
				want = "stage_artifact_unavailable"
			}
			if _, err := f.svc.CreateStageCodeCandidate(f.ctx, p); !collaborationErrorCode(err, want) {
				t.Fatalf("unbound %s: want %s, got %v", kind, want, err)
			}
			if len(f.view(t).CodeCandidates.Items) != 0 {
				t.Fatal("refused source created a code candidate")
			}
		})
	}
}

func TestStageCodeVCSWithoutRepositoryCheckIdentityIsUnsupported(t *testing.T) {
	f := newStageReviewFixture(t)
	pr := f.vcsPR(t, f.child, f.head)
	var connection string
	f.fx.QueryRow(t, `SELECT connection_id FROM vcs_pull_request WHERE id=$1`, pr).Scan(&connection)
	f.fx.Insert(t, "vcs_connection", testutil.Cols{"id": connection, "workspace_id": f.workspace, "provider": "gitlab", "instance_url": "https://example.invalid/synthetic", "account_login": "synthetic", "access_token_encrypted": "synthetic-unusable", "webhook_secret_encrypted": "synthetic-unusable"})
	f.fx.InsertNoID(t, "vcs_commit_status", testutil.Cols{"connection_id": connection, "sha": f.head, "context": "synthetic passing status", "state": "success", "updated_at": time.Now()}, "connection_id=$1", connection)
	r, err := f.svc.CreateStageCodeCandidate(f.ctx, CollaborationStageCodeParams{CollaborationWorkflowRequest: f.req(t, f.ctx), ExpectedPolicyRevision: 1, ExpectedRoleRevision: 1, ChildIssueID: util.MustParseUUID(f.child), PullRequest: CollaborationStageCodeRef{Provider: "vcs", PullRequestID: util.MustParseUUID(pr), HeadSHA: f.head}})
	if err != nil {
		t.Fatal(err)
	}
	c := stageRef(*r.StageConfirmation)
	if view := f.evidence(t, f.reviewerCtx, c); view.Observed.CI.State != "unsupported" {
		t.Fatalf("unbound VCS CI appeared usable: %+v", view.Observed)
	}
	if _, err = f.svc.AdvanceStage(f.ctx, f.advanceParams(t, c, "code_review")); !collaborationErrorCode(err, "stage_ci_unsupported") {
		t.Fatalf("VCS aggregate unlocked review: %v", err)
	}
}

func TestStageCodeReviewConcurrentIntentAndLostCommit(t *testing.T) {
	t.Run("concurrent_same_key", func(t *testing.T) {
		f := newStageReviewFixture(t)
		c := f.code(t, nil)
		if _, err := f.svc.AdvanceStage(f.ctx, f.advanceParams(t, c, "code_review")); err != nil {
			t.Fatal(err)
		}
		p := f.reviewParams(t, c, "approve")
		var wg sync.WaitGroup
		receipts := make([]CollaborationWorkflowReceipt, 2)
		errs := make([]error, 2)
		for i := range receipts {
			wg.Add(1)
			go func(i int) { defer wg.Done(); receipts[i], errs[i] = f.svc.DecideStage(f.reviewerCtx, p) }(i)
		}
		wg.Wait()
		for i, err := range errs {
			if err != nil {
				t.Fatalf("concurrent request %d: %v", i, err)
			}
		}
		if receipts[0].StageConfirmation.DecisionID != receipts[1].StageConfirmation.DecisionID || f.fx.Count(t, `SELECT count(*) FROM collaboration_stage_decision WHERE issue_id=$1 AND step='code_review'`, f.issue.Issue.ID) != 1 {
			t.Fatal("same intent produced multiple human decisions")
		}
	})
	t.Run("lost_commit", func(t *testing.T) {
		f := newStageReviewFixture(t)
		c := f.code(t, nil)
		if _, err := f.svc.AdvanceStage(f.ctx, f.advanceParams(t, c, "code_review")); err != nil {
			t.Fatal(err)
		}
		p := f.reviewParams(t, c, "approve")
		fault := &queueFaultStarter{TxStarter: f.pool, lostCommit: true}
		f.svc.TxStarter = fault
		_, err := f.svc.DecideStage(f.reviewerCtx, p)
		f.svc.TxStarter = f.pool
		if err == nil || fault.begins.Load() != 1 {
			t.Fatalf("unknown commit was retried or reported success: %v", err)
		}
		r, err := f.svc.GetWorkflowOperation(f.reviewerCtx, p.IssueID, p.RequestID)
		if err != nil || r.StageConfirmation == nil || r.StageConfirmation.Decision != "approved" {
			t.Fatalf("original-key recovery: %+v %v", r, err)
		}
		if f.fx.Count(t, `SELECT count(*) FROM collaboration_stage_decision WHERE issue_id=$1 AND step='code_review'`, p.IssueID) != 1 {
			t.Fatal("lost response duplicated decision")
		}
	})
}

func TestStageCodeReviewFencesChecksAndTrustThroughConsumption(t *testing.T) {
	for _, phase := range []string{"sign", "advance"} {
		for _, source := range []string{"head", "checks", "trust"} {
			t.Run(phase+"/"+source, func(t *testing.T) {
				f := newStageReviewFixture(t)
				c := f.code(t, nil)
				if _, err := f.svc.AdvanceStage(f.ctx, f.advanceParams(t, c, "code_review")); err != nil {
					t.Fatal(err)
				}
				p := f.reviewParams(t, c, "approve")
				var decision string
				if phase == "advance" {
					r, err := f.svc.DecideStage(f.reviewerCtx, p)
					if err != nil {
						t.Fatal(err)
					}
					decision = r.StageConfirmation.DecisionID
				}
				advance := f.advanceParams(t, c, "qa", decision)
				needle := "-- name: CreateCollaborationStageReviewDecision "
				if phase == "advance" {
					needle = "-- name: AdvanceCollaborationStage "
				}
				writerCode := "not_attempted"
				f.svc.TxStarter = stageScopeCodeProbeStarter{TxStarter: f.pool, needle: needle, beforeDecision: func() {
					tx, err := f.pool.Begin(f.ctx)
					if err != nil {
						t.Fatal(err)
					}
					defer tx.Rollback(f.ctx)
					if _, err = tx.Exec(f.ctx, `SET LOCAL lock_timeout='150ms'`); err != nil {
						t.Fatal(err)
					}
					switch source {
					case "head":
						_, err = tx.Exec(f.ctx, `UPDATE github_pull_request SET head_sha=$2 WHERE id=$1`, f.pr, strings.Repeat("c", 40))
					case "checks":
						_, err = tx.Exec(f.ctx, `UPDATE github_pull_request_check_run SET conclusion='failure' WHERE pr_id=$1`, f.pr)
					case "trust":
						_, err = tx.Exec(f.ctx, `UPDATE github_installation SET trust_revision=trust_revision+1 WHERE workspace_id=$1`, f.workspace)
					}
					writerCode = stagePgCode(err)
					if err == nil {
						if err = tx.Commit(f.ctx); err != nil {
							t.Fatal(err)
						}
					}
				}}
				var err error
				if phase == "sign" {
					_, err = f.svc.DecideStage(f.reviewerCtx, p)
				} else {
					_, err = f.svc.AdvanceStage(f.ctx, advance)
				}
				f.svc.TxStarter = f.pool
				if err != nil || writerCode != "55P03" {
					t.Fatalf("source mutation crossed %s: SQLSTATE=%s err=%v", phase, writerCode, err)
				}
			})
		}
	}
}

func TestStageCodeReviewRevocationIsAppendOnly(t *testing.T) {
	f := newStageReviewFixture(t)
	c, _, r := f.approved(t)
	p := CollaborationStageRevocationParams{CollaborationWorkflowRequest: f.req(t, f.reviewerCtx), ExpectedPolicyRevision: 1, ExpectedRoleRevision: 1, DecisionID: util.MustParseUUID(r.StageConfirmation.DecisionID), Reason: "Synthetic human withdrawal"}
	revoked, err := f.svc.RevokeStageDecision(f.reviewerCtx, p)
	if err != nil || revoked.StageConfirmation.RevocationID == "" {
		t.Fatalf("revoke: %+v %v", revoked, err)
	}
	if replay, err := f.svc.RevokeStageDecision(f.reviewerCtx, p); err != nil || replay.StageConfirmation.RevocationID != revoked.StageConfirmation.RevocationID {
		t.Fatalf("revoke replay: %+v %v", replay, err)
	}
	if _, err = f.svc.AdvanceStage(f.ctx, f.advanceParams(t, c, "qa", r.StageConfirmation.DecisionID)); !collaborationErrorCode(err, "stage_decision_revoked") {
		t.Fatalf("revoked approval admitted: %v", err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_stage_decision WHERE decision_id=$1 AND decision='approved'`, p.DecisionID) != 1 || f.fx.Count(t, `SELECT count(*) FROM collaboration_stage_revocation WHERE decision_id=$1`, p.DecisionID) != 1 {
		t.Fatal("revocation overwrote or duplicated history")
	}
	view := f.evidence(t, f.reviewerCtx, c)
	if len(view.Decisions) != 1 || view.Decisions[0].State != "revoked" || view.Decisions[0].Revocation == nil {
		t.Fatalf("revocation view: %+v", view.Decisions)
	}
}

func TestStageCodeReviewSourceMembershipRejoinDoesNotReviveApproval(t *testing.T) {
	f := newStageReviewFixture(t)
	c, _, approval := f.approved(t)
	var originalMember, restoredMember string
	f.fx.QueryRow(t, `SELECT id FROM member WHERE workspace_id=$1 AND user_id=$2`, f.workspace, f.reviewer).Scan(&originalMember)
	f.fx.Exec(t, `DELETE FROM member WHERE workspace_id=$1 AND user_id=$2`, f.workspace, f.reviewer)
	f.fx.Member(t, f.workspace, f.reviewer, "member")
	f.fx.QueryRow(t, `SELECT id FROM member WHERE workspace_id=$1 AND user_id=$2`, f.workspace, f.reviewer).Scan(&restoredMember)
	if originalMember == restoredMember {
		t.Fatal("fixture did not create a new source grant")
	}
	if _, err := f.svc.AdvanceStage(f.ctx, f.advanceParams(t, c, "qa", approval.StageConfirmation.DecisionID)); !collaborationErrorCode(err, "stage_signer_changed") {
		t.Fatalf("source membership rejoin revived the original approval: %v", err)
	}
	if f.view(t).Stage.Current != "code_review" {
		t.Fatal("old approval advanced the task")
	}
	// Current source membership permits a fresh, explicit human signature.
	fresh, err := f.svc.DecideStage(f.reviewerCtx, f.reviewParams(t, c, "approve"))
	if err != nil {
		t.Fatal(err)
	}
	if fresh.StageConfirmation.DecisionID == approval.StageConfirmation.DecisionID {
		t.Fatal("new signature reused the old approval")
	}
	if _, err = f.svc.AdvanceStage(f.ctx, f.advanceParams(t, c, "qa", fresh.StageConfirmation.DecisionID)); err != nil {
		t.Fatalf("fresh approval after rejoin: %v", err)
	}
}

func TestStageCodeCandidateDiscoveryEmpty(t *testing.T) {
	f := newCollaborationStageFixture(t)
	f.revise(t, true)
	f.configure(t)
	f.confirmScope(t)
	raw, err := json.Marshal(f.view(t))
	if err != nil {
		t.Fatal(err)
	}
	var view map[string]json.RawMessage
	if err = json.Unmarshal(raw, &view); err != nil {
		t.Fatal(err)
	}
	var candidates struct {
		Items     []json.RawMessage `json:"items"`
		Truncated bool              `json:"truncated"`
	}
	if json.Unmarshal(view["code_candidates"], &candidates) != nil || candidates.Items == nil || len(candidates.Items) != 0 || candidates.Truncated {
		t.Fatalf("stage view has no authoritative empty code-candidate discovery: %s", raw)
	}
}

func TestStageScopeRoleRestorationDoesNotReviveApproval(t *testing.T) {
	f := newCollaborationStageFixture(t)
	f.revise(t, true)
	f.configure(t)
	f.confirmScope(t)
	roles := f.roleSet()
	roles.ProductApprover = util.MustParseUUID(f.qa)
	for i, next := range []CollaborationStageRoleSet{roles, f.roleSet()} {
		_, err := f.svc.ConfigureStageRoles(f.ctx, CollaborationStageRolesParams{CollaborationWorkflowRequest: f.req(t, f.ctx), ExpectedPolicyRevision: f.policyRevision,
			ExpectedRoleRevision: int64(i + 1), Roles: next})
		if err != nil {
			t.Fatal(err)
		}
	}
	if view := f.view(t); view.ScopeState != "signer_changed" || !view.NeedsConfirmation {
		t.Fatalf("restoring a role revived a signature from an earlier configuration: %+v", view)
	}
}

func TestStageScopeWriteRequiresCurrentCredentialWithoutRetainedExpiry(t *testing.T) {
	for _, kind := range []string{"expired_jwt", "revoked_pat"} {
		t.Run(kind, func(t *testing.T) {
			f := newStageReviewFixture(t)
			p := CollaborationStageScopeParams{CollaborationWorkflowRequest: f.req(t, f.ctx), ExpectedPolicyRevision: 1, ExpectedRoleRevision: 1, ExpectedScopeRevision: 1, Children: []CollaborationStageScopeChild{}, Reason: "Synthetic scope confirmation"}
			principal := auth.AuthenticatedPrincipal{UserID: f.creator, Kind: auth.CredentialJWT, ExpiresAt: time.Now().Add(-time.Minute)}
			if kind == "revoked_pat" {
				hash := strings.ReplaceAll(f.creator, "-", "") + strings.Repeat("d", 32)
				f.fx.Insert(t, "personal_access_token", testutil.Cols{"user_id": f.creator, "name": "Synthetic revoked scope PAT", "token_hash": hash, "token_prefix": "synthetic", "revoked": true})
				principal.Kind, principal.PATTokenHash = auth.CredentialPAT, hash
			}
			ctx := auth.WithAuthenticatedPrincipal(context.Background(), principal)
			if _, err := f.svc.ConfirmStageScope(ctx, p); !collaborationErrorCode(err, "forbidden") && !(kind == "revoked_pat" && collaborationErrorCode(err, "not_found")) {
				t.Fatalf("inactive credential confirmed scope: %v", err)
			}
			if f.view(t).ScopeRevision != "1" {
				t.Fatal("invalid credential advanced scope")
			}
		})
	}
}
