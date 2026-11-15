package service

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestKnowledgeAutomationPolicyPermissionsAndOriginalRecovery(t *testing.T) {
	f := newCollaborationFixture(t)
	project := util.MustParseUUID(f.fx.Project(t, "Knowledge policy"))
	workspace := util.MustParseUUID(f.workspace)
	view, err := f.svc.GetProjectKnowledgePolicy(f.ctx, project, workspace)
	if err != nil || view.Revision != "0" || !view.IsEnabled || view.MaxCandidates != 3 || view.Origin != "system" || view.ChangedByUserID != nil {
		t.Fatalf("default system policy: %+v %v", view, err)
	}
	if _, err = f.svc.GetProjectKnowledgePolicy(collaborationContext(f.handler), project, workspace); !knowledgeErrorCode(err, "knowledge_permission_denied") {
		t.Fatalf("external task participant gained policy authority: %v", err)
	}
	f.fx.Member(t, f.workspace, f.outsider, "member")
	if _, err = f.svc.GetProjectKnowledgePolicy(collaborationContext(f.outsider), project, workspace); !knowledgeErrorCode(err, "knowledge_permission_denied") {
		t.Fatalf("ordinary member gained policy authority: %v", err)
	}
	f.fx.Exec(t, `UPDATE project SET lead_id=$1,lead_type='member' WHERE id=$2`, f.outsider, project)
	if _, err = f.svc.GetProjectKnowledgePolicy(collaborationContext(f.outsider), project, workspace); err != nil {
		t.Fatalf("current human project lead denied: %v", err)
	}
	first := ProjectKnowledgePolicyRequest{RequestID: uuid.NewString(), ExpectedRevision: "0", IsEnabled: false, MaxCandidates: 2}
	one, err := f.svc.UpdateProjectKnowledgePolicy(f.ctx, project, workspace, first)
	if err != nil || one.Revision != "1" {
		t.Fatalf("disabled override: %+v %v", one, err)
	}
	second := ProjectKnowledgePolicyRequest{RequestID: uuid.NewString(), ExpectedRevision: "1", IsEnabled: true, MaxCandidates: 1}
	if two, err := f.svc.UpdateProjectKnowledgePolicy(f.ctx, project, workspace, second); err != nil || two.Revision != "2" {
		t.Fatalf("current policy update: %+v %v", two, err)
	}
	if replay, err := f.svc.UpdateProjectKnowledgePolicy(f.ctx, project, workspace, first); err != nil || replay != one {
		t.Fatalf("older original request recomputed latest policy: %+v %v", replay, err)
	}
	stale := first
	stale.RequestID = uuid.NewString()
	refused, err := f.svc.UpdateProjectKnowledgePolicy(f.ctx, project, workspace, stale)
	if !knowledgeErrorCode(err, "knowledge_policy_revision_conflict") || refused.Revision != "2" {
		t.Fatalf("stale policy CAS accepted: %+v %v", refused, err)
	}
	changed := first
	changed.IsEnabled = true
	if _, err = f.svc.UpdateProjectKnowledgePolicy(f.ctx, project, workspace, changed); !knowledgeErrorCode(err, "idempotency_conflict") {
		t.Fatalf("changed original intent accepted: %v", err)
	}
	f.fx.Exec(t, `DELETE FROM member WHERE workspace_id=$1 AND user_id=$2`, f.workspace, f.creator)
	if recovered, err := f.svc.GetProjectKnowledgePolicyOperation(f.ctx, project, first.RequestID); err != nil || recovered != one {
		t.Fatalf("original actor lost metadata-only receipt after role loss: %+v %v", recovered, err)
	}
	if _, err = f.svc.GetProjectKnowledgePolicyOperation(collaborationContext(f.outsider), project, first.RequestID); !knowledgeErrorCode(err, "operation_not_found") {
		t.Fatalf("another actor recovered private original intent: %v", err)
	}
}

func TestKnowledgeAutomationOnlyFormalProjectAcceptanceCreatesIntent(t *testing.T) {
	for _, kind := range []string{"project", "no_project", "manual", "disabled"} {
		t.Run(kind, func(t *testing.T) {
			f := newCollaborationHandoffFixture(t)
			if kind != "no_project" {
				project := f.fx.Project(t, "Accepted knowledge source")
				f.fx.Exec(t, `UPDATE issue SET project_id=$1 WHERE id=$2`, project, f.issue.Issue.ID)
				if kind == "disabled" {
					_, err := f.svc.UpdateProjectKnowledgePolicy(f.ctx, util.MustParseUUID(project), util.MustParseUUID(f.workspace), ProjectKnowledgePolicyRequest{
						RequestID: uuid.NewString(), ExpectedRevision: "0", IsEnabled: false, MaxCandidates: 3})
					if err != nil {
						t.Fatal(err)
					}
				}
			}
			if kind == "manual" {
				f.fx.Exec(t, `UPDATE issue_collaboration SET ai_policy='manual' WHERE issue_id=$1`, f.issue.Issue.ID)
			}
			p := handoffAcceptance(t, f, 1, "Amounts are expressed in cents.")
			p.Basis = "The accepted API contract states cents as its unit."
			submitted, err := f.svc.SubmitAcceptance(collaborationContext(f.handler), p)
			if err != nil {
				t.Fatal(err)
			}
			if f.fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_job WHERE source_issue_id=$1`, f.issue.Issue.ID) != 0 {
				t.Fatal("unaccepted submission created an automatic job")
			}
			id := submitted.DeliveryConfirmation.SubmissionID
			accept := lifecycleParams(t, f, "complete", "done", 2, 1)
			accept.SubmissionID = &id
			if _, err = f.svc.ChangeLifecycle(f.ctx, accept); err != nil {
				t.Fatal(err)
			}
			if _, err = f.svc.ChangeLifecycle(f.ctx, accept); err != nil {
				t.Fatal(err)
			}
			want := 0
			if kind == "project" {
				want = 1
			}
			if got := f.fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_job WHERE source_issue_id=$1`, f.issue.Issue.ID); got != want {
				t.Fatalf("automatic acceptance intents=%d want%d", got, want)
			}
			if f.fx.Count(t, `SELECT count(*) FROM collaboration_session WHERE issue_id=$1 AND queue_state='closed'`, f.issue.Issue.ID) != 1 {
				t.Fatal("knowledge work reopened the completed business queue")
			}
			if kind == "project" {
				jobs, err := f.svc.ListKnowledgeAutomationJobs(f.ctx, f.issue.Issue.ID)
				if err != nil || len(jobs.Items) != 1 || !jobs.Items[0].SourceCurrent || jobs.Items[0].Phase != "queued" {
					t.Fatalf("accepted job projection: %+v %v", jobs, err)
				}
				rows, err := f.svc.Queries.ListKnowledgeAutomationJobs(context.Background(), db.ListKnowledgeAutomationJobsParams{SourceWorkspaceID: util.MustParseUUID(f.workspace), SourceIssueID: f.issue.Issue.ID, Limit: 2})
				if err != nil || rows[0].SourceReaderUserID != util.MustParseUUID(f.creator) || rows[0].SubmissionID != id || rows[0].ValidatedCloseEpoch != 2 || rows[0].ExecutionPolicyRevision != 0 {
					t.Fatalf("job lost actual acceptance binding: %+v %v", rows, err)
				}
			}
		})
	}
}

func TestKnowledgeAutomationRuleUpgradePreservesExplicitPolicy(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		f := newCollaborationFixture(t)
		project := util.MustParseUUID(f.fx.Project(t, "Earlier rule override"))
		workspace := util.MustParseUUID(f.workspace)
		created, err := f.svc.Queries.CreateProjectKnowledgePolicy(f.ctx, db.CreateProjectKnowledgePolicyParams{
			ProjectID: project, WorkspaceID: workspace, IsEnabled: enabled, MaxCandidates: 1,
			BuiltinRuleVersion: "earlier-project-rule", ChangedByUserID: util.MustParseUUID(f.creator)})
		if err != nil {
			t.Fatal(err)
		}
		view, err := f.svc.GetProjectKnowledgePolicy(f.ctx, project, workspace)
		if err != nil || view.IsEnabled != enabled || view.MaxCandidates != 1 || view.Revision != "1" || view.RuleVersion != ProjectKnowledgeRuleVersion {
			t.Fatalf("rule upgrade changed explicit settings or mislabeled execution: %+v %v", view, err)
		}
		retained, err := f.svc.Queries.GetProjectKnowledgePolicy(f.ctx, db.GetProjectKnowledgePolicyParams{ProjectID: project, WorkspaceID: workspace})
		if err != nil || retained.BuiltinRuleVersion != created.BuiltinRuleVersion || retained.Revision != created.Revision || retained.IsEnabled != enabled {
			t.Fatal("effective rule read rewrote historical policy override", retained, err)
		}
	}
}
