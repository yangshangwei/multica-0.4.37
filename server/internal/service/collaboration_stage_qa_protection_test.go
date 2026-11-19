package service

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// Both cases stay in QA so the opposite human decision is otherwise available.
// Actual report, claim, permit and settled outcome owners create the evidence.
func TestStageQAReturnProtectedFromAgentOverride(t *testing.T) {
	tc13ProtectQAJudgment(t, "return")
}

func TestStageQAApprovalProtectedFromAgentOverride(t *testing.T) {
	tc13ProtectQAJudgment(t, "approve")
}

func tc13ProtectQAJudgment(t *testing.T, decision string) {
	t.Helper()
	f := newStageRuntimeFixture(t, true)
	run, claim, permit := startStageReportRuntime(t, f)
	subject, err := f.svc.RuntimeTestSubject(context.Background(), run, permit.BusinessToken)
	if err != nil {
		t.Fatal(err)
	}
	report := stageRuntimeReport(subject.Subject, claim.Identity)
	report.Report.Cases[0].Outcome = "passed"
	ack, err := f.svc.RecordRuntimeTestReport(context.Background(), run, permit.BusinessToken, report)
	if err != nil || !ack.Accepted {
		t.Fatalf("real passing report: accepted=%t error=%v", ack.Accepted, err)
	}
	finishSquadRun(t, f.managedCollaborationFixture, run, claim, permit.BusinessToken, permit.AttemptNonce, "success")
	code, err := f.svc.Queries.GetCollaborationStageReviewHead(f.ctx, db.GetCollaborationStageReviewHeadParams{IssueID: f.issue.Issue.ID, CandidateID: f.candidate.CandidateID, Step: "code_review"})
	if err != nil {
		t.Fatal(err)
	}
	params := func(kind string) CollaborationStageReviewParams {
		p := CollaborationStageReviewParams{CollaborationWorkflowRequest: f.stage.req(t, f.stage.qaCtx), ExpectedPolicyRevision: f.stage.policyRevision,
			ExpectedRoleRevision:  f.stage.stageInt(t, `SELECT role_revision FROM collaboration_stage_task WHERE issue_id=$1`),
			ExpectedStageRevision: f.stage.stageInt(t, `SELECT stage_revision FROM collaboration_stage_task WHERE issue_id=$1`),
			Step:                  "qa", Decision: kind, Candidate: f.candidate, DependsOn: []string{}, Reason: "Human QA correction remains authoritative"}
		if kind == "approve" {
			view := f.stage.evidence(t, f.stage.qaCtx, f.candidate)
			if len(view.Signable) < 2 || view.Signable[1].Step != "qa" || !view.Signable[1].Allowed || len(view.TestReports) != 1 || view.TestReports[0].ReportID != ack.ReportID || view.TestReports[0].Applicability != "usable" {
				t.Fatalf("opposite approval must have current usable evidence: %+v", view)
			}
			expiry := view.CheckedAt.Add(time.Minute).Truncate(time.Microsecond)
			p.ExpiresAt, p.Evidence.TestReportID, p.DependsOn = &expiry, &ack.ReportID, []string{util.UUIDToString(code.DecisionID)}
		}
		return p
	}
	human := params(decision)
	receipt, err := f.svc.DecideStage(f.stage.qaCtx, human)
	if err != nil || receipt.StageConfirmation == nil || receipt.StageConfirmation.DecisionID == "" {
		t.Fatalf("actual human QA %s: %+v %v", decision, receipt, err)
	}
	decisionID := util.MustParseUUID(receipt.StageConfirmation.DecisionID)
	original, err := f.svc.Queries.GetCollaborationStageDecision(f.ctx, db.GetCollaborationStageDecisionParams{IssueID: f.issue.Issue.ID, DecisionID: decisionID})
	if err != nil || original.ActorUserID != util.MustParseUUID(f.stage.qa) || original.Step != "qa" || original.CreatedAt.Time.IsZero() {
		t.Fatalf("missing exact human judgment: %+v %v", original, err)
	}
	wantDecision := "returned"
	opposite := "approve"
	if decision == "approve" {
		wantDecision, opposite = "approved", "return"
	}
	if original.Decision != wantDecision {
		t.Fatalf("human decision=%s want=%s", original.Decision, wantDecision)
	}
	oppositeParams := params(opposite)
	originalReceipt, err := f.svc.GetWorkflowOperation(f.stage.qaCtx, f.issue.Issue.ID, human.RequestID)
	if err != nil || originalReceipt.StageConfirmation == nil || originalReceipt.StageConfirmation.DecisionID != receipt.StageConfirmation.DecisionID {
		t.Fatalf("original human recovery receipt: %+v %v", originalReceipt, err)
	}
	before := tc13QAFacts(t, f)
	beforeView := f.stage.evidence(t, f.stage.qaCtx, f.candidate)
	beforeStage := f.stage.view(t)
	if beforeStage.Stage.Current != "qa" {
		t.Fatal("refusals must run while the same QA stage is current")
	}
	head, err := f.svc.Queries.GetCollaborationStageReviewHead(f.ctx, db.GetCollaborationStageReviewHeadParams{IssueID: f.issue.Issue.ID, CandidateID: f.candidate.CandidateID, Step: "qa"})
	if err != nil || head.DecisionID != decisionID {
		t.Fatalf("human judgment is not current QA head: %+v %v", head, err)
	}
	for _, actor := range []struct {
		name string
		kind auth.CredentialKind
		hint bool
	}{{"task_principal", auth.CredentialTask, false}, {"human_with_agent_actor", auth.CredentialJWT, true}} {
		for _, action := range []string{"opposite_decision", "exact_revocation"} {
			t.Run(actor.name+"/"+action, func(t *testing.T) {
				ctx := auth.WithAuthenticatedPrincipal(f.stage.qaCtx, auth.AuthenticatedPrincipal{UserID: f.stage.qa, Kind: actor.kind, AgentActor: actor.hint, ExpiresAt: time.Now().Add(time.Hour)})
				request := f.stage.req(t, f.stage.qaCtx)
				if action == "opposite_decision" {
					p := oppositeParams
					p.CollaborationWorkflowRequest = request
					_, err = f.svc.DecideStage(ctx, p)
				} else {
					_, err = f.svc.RevokeStageDecision(ctx, CollaborationStageRevocationParams{CollaborationWorkflowRequest: request,
						ExpectedPolicyRevision: f.stage.policyRevision, ExpectedRoleRevision: human.ExpectedRoleRevision, DecisionID: decisionID, Reason: "Agent attempts to revoke the exact human QA judgment"})
				}
				if !collaborationErrorCode(err, "forbidden") {
					t.Fatalf("authenticated %s %s must hit human-only denial: %v", actor.name, action, err)
				}
				if got := tc13QAFacts(t, f); !reflect.DeepEqual(got, before) {
					t.Fatalf("denied %s changed immutable QA/runtime/notification facts", action)
				}
				view := f.stage.evidence(t, f.stage.qaCtx, f.candidate)
				// CheckedAt and signability horizons are read-time observations;
				// decision expiry, evidence and all business fields remain exact.
				view.CheckedAt, beforeView.CheckedAt = time.Time{}, time.Time{}
				for i := range view.Signable {
					view.Signable[i].MaxExpiresAt = nil
				}
				for i := range beforeView.Signable {
					beforeView.Signable[i].MaxExpiresAt = nil
				}
				if !reflect.DeepEqual(view, beforeView) || !reflect.DeepEqual(f.stage.view(t), beforeStage) {
					t.Fatal("Agent refusal changed the authoritative QA/candidate/stage projection")
				}
				recovered, readErr := f.svc.GetWorkflowOperation(f.stage.qaCtx, f.issue.Issue.ID, human.RequestID)
				if readErr != nil || !reflect.DeepEqual(recovered, originalReceipt) {
					t.Fatalf("original human receipt changed: %+v %v", recovered, readErr)
				}
				if f.fx.Count(t, `SELECT count(*) FROM collaboration_operation WHERE scope_kind='issue' AND scope_id=$1 AND request_id=$2`, f.issue.Issue.ID, request.RequestID) != 0 {
					t.Fatal("human-only refusal persisted an operation")
				}
			})
		}
	}
	// This distinct human request demonstrates the opposite envelope is not
	// stale, malformed, missing evidence or otherwise unusable after refusals.
	oppositeParams.CollaborationWorkflowRequest = f.stage.req(t, f.stage.qaCtx)
	oppositeParams.RequestID = uuid.NewString()
	control, err := f.svc.DecideStage(f.stage.qaCtx, oppositeParams)
	if err != nil || control.StageConfirmation == nil || control.StageConfirmation.DecisionID == receipt.StageConfirmation.DecisionID {
		t.Fatalf("fresh legitimate opposite human control: %+v %v", control, err)
	}
	retained, err := f.svc.Queries.GetCollaborationStageDecision(f.ctx, db.GetCollaborationStageDecisionParams{IssueID: f.issue.Issue.ID, DecisionID: decisionID})
	if err != nil || !reflect.DeepEqual(retained, original) {
		t.Fatalf("new human decision rewrote original immutable QA judgment: %+v %v", retained, err)
	}
}

func tc13QAFacts(t *testing.T, f *stageRuntimeFixture) map[string]string {
	t.Helper()
	out := map[string]string{}
	for table, predicate := range map[string]string{
		"issue": "id=$1", "issue_collaboration": "issue_id=$1", "collaboration_session": "issue_id=$1",
		"collaboration_stage_task": "issue_id=$1", "collaboration_stage_decision": "issue_id=$1", "collaboration_stage_revocation": "issue_id=$1",
		"collaboration_stage_candidate": "issue_id=$1", "collaboration_operation": "scope_kind='issue' AND scope_id=$1",
		"collaboration_event": "issue_id=$1", "collaboration_notification": "issue_id=$1", "comment": "issue_id=$1",
		"collaboration_command":         "session_id IN (SELECT id FROM collaboration_session WHERE issue_id=$1)",
		"collaboration_execution_group": "session_id IN (SELECT id FROM collaboration_session WHERE issue_id=$1)",
		"collaboration_run":             "session_id IN (SELECT id FROM collaboration_session WHERE issue_id=$1)",
		"collaboration_run_event":       "run_id IN (SELECT id FROM collaboration_run WHERE session_id IN (SELECT id FROM collaboration_session WHERE issue_id=$1))",
		"collaboration_test_subject":    "command_id IN (SELECT id FROM collaboration_command WHERE session_id IN (SELECT id FROM collaboration_session WHERE issue_id=$1))",
		"agent_task_queue":              "issue_id=$1 OR issue_id IN (SELECT id FROM issue WHERE parent_issue_id=$1)",
	} {
		var rows string
		f.fx.QueryRow(t, `SELECT COALESCE(jsonb_agg(to_jsonb(x) ORDER BY to_jsonb(x)::text),'[]'::jsonb)::text FROM `+table+` x WHERE `+predicate, f.issue.Issue.ID).Scan(&rows)
		out[table] = rows
	}
	return out
}
