package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
)

func (f *collaborationStageFixture) githubPR(t *testing.T, issue, head string, referenceOnly bool) string {
	t.Helper()
	now := time.Now()
	id := f.fx.Insert(t, "github_pull_request", testutil.Cols{"workspace_id": f.workspace, "installation_id": int64(1), "repo_owner": "stage", "repo_name": uuid.NewString(),
		"pr_number": 1, "title": "Private pull request title", "state": "open", "html_url": "https://example.invalid/private", "pr_created_at": now, "pr_updated_at": now, "head_sha": head})
	f.fx.InsertNoID(t, "issue_pull_request", testutil.Cols{"issue_id": issue, "pull_request_id": id, "reference_only": referenceOnly}, "issue_id=$1 AND pull_request_id=$2", issue, id)
	return id
}

func (f *collaborationStageFixture) vcsPR(t *testing.T, issue, head string) string {
	t.Helper()
	now := time.Now()
	id := f.fx.Insert(t, "vcs_pull_request", testutil.Cols{"workspace_id": f.workspace, "connection_id": uuid.NewString(), "provider": "gitlab", "repo_owner": "stage", "repo_name": uuid.NewString(),
		"pr_number": 1, "title": "Private merge request title", "state": "open", "html_url": "https://example.invalid/private", "pr_created_at": now, "pr_updated_at": now, "head_sha": head})
	f.fx.InsertNoID(t, "issue_vcs_pull_request", testutil.Cols{"issue_id": issue, "pull_request_id": id}, "issue_id=$1 AND pull_request_id=$2", issue, id)
	return id
}

func stageChildView(t *testing.T, view CollaborationStageView, issue string) CollaborationStageChildView {
	t.Helper()
	for _, item := range view.Children.Items {
		if item.IssueID == issue {
			return item
		}
	}
	t.Fatalf("child %s is not in the stage view", issue)
	return CollaborationStageChildView{}
}

func stageReadableDigest(t *testing.T, child CollaborationStageChildView) string {
	t.Helper()
	if !child.CanOpen || child.ContentDigest == nil {
		t.Fatal("fixture child is not source-readable")
	}
	return *child.ContentDigest
}

// On first enablement or a reopen the product approver confirms each adopted
// child together with the code version it carries forward. The server accepts
// only a pull request it finds linked to that child with exactly that head.
func TestStageScopeConfirmationBindsVerifiedCodeVersion(t *testing.T) {
	f := newCollaborationStageFixture(t)
	root := f.issue.Issue.ID
	headA, headB, headC := strings.Repeat("a", 40), strings.Repeat("b", 40), strings.Repeat("c", 64)
	withCode := f.fx.Issue(t, "Child with code", testutil.Cols{"parent_issue_id": root, "project_id": f.project})
	withMerge := f.fx.Issue(t, "Child with a merge request", testutil.Cols{"parent_issue_id": root, "project_id": f.project})
	plain := f.fx.Issue(t, "Child without code", testutil.Cols{"parent_issue_id": root, "project_id": f.project})
	other := f.fx.Issue(t, "Unrelated issue with code")
	f.syncCounter(t)
	linked := f.githubPR(t, withCode, headA, false)
	mentioned := f.githubPR(t, withCode, headB, true)
	foreign := f.githubPR(t, other, headA, false)
	merge := f.vcsPR(t, withMerge, headC)
	f.revise(t, true)
	f.configure(t)
	view := f.view(t)
	item := stageChildView(t, view, withCode)
	if len(item.PullRequests) != 1 || item.PullRequests[0].Provider != "github" || item.PullRequests[0].PullRequestID != linked || item.PullRequests[0].HeadSHA != headA ||
		item.Code != nil || item.CodeState != "none" {
		t.Fatalf("the view must offer exactly the really linked pull request and no confirmed code yet: %+v", item)
	}
	if merged := stageChildView(t, view, withMerge); len(merged.PullRequests) != 1 || merged.PullRequests[0].Provider != "vcs" || merged.PullRequests[0].HeadSHA != headC {
		t.Fatalf("a linked merge request is not offered: %+v", merged)
	}
	if none := stageChildView(t, view, plain); len(none.PullRequests) != 0 || none.PullRequests == nil {
		t.Fatalf("a child without code must list an empty set: %+v", none)
	}
	child := func(id string, code *CollaborationStageCodeRef) CollaborationStageScopeChild {
		return CollaborationStageScopeChild{IssueID: util.MustParseUUID(id), ContentDigest: stageReadableDigest(t, stageChildView(t, view, id)), Code: code}
	}
	ref := func(provider, id, head string) *CollaborationStageCodeRef {
		return &CollaborationStageCodeRef{Provider: provider, PullRequestID: util.MustParseUUID(id), HeadSHA: head}
	}
	confirm := func(children ...CollaborationStageScopeChild) (CollaborationWorkflowReceipt, error) {
		return f.svc.ConfirmStageScope(f.ctx, CollaborationStageScopeParams{CollaborationWorkflowRequest: f.req(t, f.ctx), ExpectedPolicyRevision: f.policyRevision,
			ExpectedRoleRevision:  f.stageInt(t, `SELECT role_revision FROM collaboration_stage_task WHERE issue_id=$1`),
			ExpectedScopeRevision: f.stageInt(t, `SELECT COALESCE((SELECT scope_revision FROM collaboration_stage_task WHERE issue_id=$1),0)`), Children: children})
	}
	before := f.stageRows(t)
	for name, tc := range map[string]struct {
		children []CollaborationStageScopeChild
		code     string
	}{
		"unknown_pull_request":   {[]CollaborationStageScopeChild{child(withCode, ref("github", uuid.NewString(), headA))}, "stage_scope_code_unavailable"},
		"reference_only_link":    {[]CollaborationStageScopeChild{child(withCode, ref("github", mentioned, headB))}, "stage_scope_code_unavailable"},
		"another_issue_s_code":   {[]CollaborationStageScopeChild{child(withCode, ref("github", foreign, headA))}, "stage_scope_code_unavailable"},
		"wrong_provider":         {[]CollaborationStageScopeChild{child(withCode, ref("vcs", linked, headA))}, "stage_scope_code_unavailable"},
		"child_without_code":     {[]CollaborationStageScopeChild{child(plain, ref("github", linked, headA))}, "stage_scope_code_unavailable"},
		"stale_head":             {[]CollaborationStageScopeChild{child(withCode, ref("github", linked, headB))}, "stage_scope_changed"},
		"one_bad_among_valid":    {[]CollaborationStageScopeChild{child(plain, nil), child(withCode, ref("github", linked, headB))}, "stage_scope_changed"},
		"malformed_head":         {[]CollaborationStageScopeChild{child(withCode, ref("github", linked, "HEAD"))}, "invalid_stage_request"},
		"client_stated_provider": {[]CollaborationStageScopeChild{child(withCode, ref("local", linked, headA))}, "invalid_stage_request"},
	} {
		if _, err := confirm(tc.children...); !collaborationErrorCode(err, tc.code) {
			t.Fatalf("%s: want %s, got %v", name, tc.code, err)
		}
	}
	if after := f.stageRows(t); after != before {
		t.Fatalf("an unverifiable code reference wrote stage facts:\n%s\n%s", before, after)
	}
	confirmed, err := confirm(child(withCode, ref("github", linked, headA)), child(withMerge, ref("vcs", merge, headC)), child(plain, nil))
	if err != nil || confirmed.Code != "stage_scope_confirmed" {
		t.Fatalf("confirm with verified code: %+v %v", confirmed, err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_stage_child WHERE child_issue_id=$1 AND code_provider='github' AND code_pull_request_id=$2 AND code_head_sha=$3`, withCode, linked, headA) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_stage_child WHERE child_issue_id=$1 AND code_provider='vcs' AND code_pull_request_id=$2 AND code_head_sha=$3`, withMerge, merge, headC) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_stage_child WHERE child_issue_id=$1 AND code_provider IS NULL AND code_pull_request_id IS NULL AND code_head_sha IS NULL`, plain) != 1 {
		t.Fatal("the mapping did not record exactly the confirmed code versions")
	}
	var payload string
	f.fx.QueryRow(t, `SELECT payload::text FROM collaboration_stage_decision WHERE decision_id=$1`, confirmed.StageConfirmation.DecisionID).Scan(&payload)
	if !strings.Contains(payload, headA) || !strings.Contains(payload, linked) || !strings.Contains(payload, headC) || strings.Contains(payload, "Private") {
		t.Fatalf("the immutable scope decision does not bind the code versions, or carries private text: %s", payload)
	}
	view = f.view(t)
	if item = stageChildView(t, view, withCode); item.Code == nil || item.Code.PullRequest.PullRequestID != linked || item.Code.PullRequest.HeadSHA != headA || item.CodeState != "current" {
		t.Fatalf("confirmed code in the view: %+v", item)
	}
	if none := stageChildView(t, view, plain); none.Code != nil || none.CodeState != "none" {
		t.Fatalf("an explicit no-code confirmation: %+v", none)
	}
	// A new head makes the confirmed code version non-current. It is evidence
	// state only: the scope stays confirmed and the child keeps working.
	f.fx.Exec(t, `UPDATE github_pull_request SET head_sha=$2 WHERE id=$1`, linked, headB)
	view = f.view(t)
	if item = stageChildView(t, view, withCode); item.CodeState != "changed" || item.Code.PullRequest.HeadSHA != headA || item.PullRequests[0].HeadSHA != headB ||
		!item.ApprovalCurrent || item.ExecutionGated || view.ScopeState != "current" || view.NeedsConfirmation {
		t.Fatalf("a moved head must only mark the confirmed code version as changed: %+v %s", item, view.ScopeState)
	}
	f.fx.Exec(t, `UPDATE issue_pull_request SET reference_only=true WHERE issue_id=$1 AND pull_request_id=$2`, withCode, linked)
	if item = stageChildView(t, f.view(t), withCode); item.CodeState != "unavailable" || len(item.PullRequests) != 0 {
		t.Fatalf("an unlinked pull request must make the confirmed code version unavailable: %+v", item)
	}
	f.fx.Exec(t, `UPDATE issue_pull_request SET reference_only=false WHERE issue_id=$1 AND pull_request_id=$2`, withCode, linked)
	// Reconfirming at the new head replaces the binding; an explicit null clears it.
	view = f.view(t)
	if _, err = confirm(child(withCode, ref("github", linked, headB))); err != nil {
		t.Fatal(err)
	}
	if item = stageChildView(t, f.view(t), withCode); item.CodeState != "current" || item.Code.PullRequest.HeadSHA != headB {
		t.Fatalf("reconfirmed code: %+v", item)
	}
	view = f.view(t)
	if _, err = confirm(child(withCode, nil)); err != nil {
		t.Fatal(err)
	}
	if item = stageChildView(t, f.view(t), withCode); item.Code != nil || item.CodeState != "none" {
		t.Fatalf("an explicit null did not clear the carried code version: %+v", item)
	}
}

type stageScopeCodeProbeStarter struct {
	TxStarter
	beforeDecision func()
	needle         string
}

type stageScopeCodeProbeTx struct {
	pgx.Tx
	beforeDecision func()
	needle         string
}

func (s stageScopeCodeProbeStarter) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return nil, err
	}
	needle := s.needle
	if needle == "" {
		needle = "-- name: CreateCollaborationStageDecision "
	}
	return &stageScopeCodeProbeTx{Tx: tx, beforeDecision: s.beforeDecision, needle: needle}, nil
}

func (t *stageScopeCodeProbeTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if strings.Contains(sql, t.needle) {
		t.beforeDecision()
	}
	return t.Tx.QueryRow(ctx, sql, args...)
}

// A connector mutation after the observed head was read must not commit before
// the confirmation does. Both PR rows and association rows need the same fence
// because existing connector writers do not share the child's issue lock.
func TestStageScopeConfirmationFencesCodeUntilCommit(t *testing.T) {
	for _, source := range []string{"head", "link"} {
		t.Run(source, func(t *testing.T) {
			f := newCollaborationStageFixture(t)
			child := f.fx.Issue(t, "Synthetic code-fence child", testutil.Cols{"parent_issue_id": f.issue.Issue.ID})
			head := strings.Repeat("a", 40)
			pr := f.githubPR(t, child, head, false)
			f.revise(t, true)
			f.configure(t)
			item := stageChildView(t, f.view(t), child)
			observed, writerCode := false, ""
			f.svc.TxStarter = stageScopeCodeProbeStarter{TxStarter: f.pool, beforeDecision: func() {
				observed = true
				connector, err := f.pool.Begin(f.ctx)
				if err != nil {
					t.Fatal(err)
				}
				defer connector.Rollback(f.ctx)
				if _, err = connector.Exec(f.ctx, `SET LOCAL lock_timeout='150ms'`); err != nil {
					t.Fatal(err)
				}
				if source == "head" {
					_, err = connector.Exec(f.ctx, `UPDATE github_pull_request SET head_sha=$2 WHERE id=$1`, pr, strings.Repeat("b", 40))
				} else {
					_, err = connector.Exec(f.ctx, `UPDATE issue_pull_request SET reference_only=true WHERE issue_id=$1 AND pull_request_id=$2`, child, pr)
				}
				writerCode = stagePgCode(err)
				if err == nil {
					if err = connector.Commit(f.ctx); err != nil {
						t.Fatal(err)
					}
				}
			}}
			f.confirmScope(t, CollaborationStageScopeChild{IssueID: util.MustParseUUID(child), ContentDigest: stageReadableDigest(t, item),
				Code: &CollaborationStageCodeRef{Provider: "github", PullRequestID: util.MustParseUUID(pr), HeadSHA: head}})
			f.svc.TxStarter = f.pool
			if !observed || writerCode != "55P03" {
				t.Fatalf("connector %s changed after observation but before confirmation commit: observed=%v writer SQLSTATE=%q", source, observed, writerCode)
			}
			if current := stageChildView(t, f.view(t), child); current.CodeState != "current" || current.Code.PullRequest.HeadSHA != head {
				t.Fatalf("confirmation committed a stale code reference: %+v", current)
			}
		})
	}
}
