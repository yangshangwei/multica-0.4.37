package service

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5/pgtype"
	"testing"

	"github.com/multica-ai/multica/server/internal/util"
)

func TestStageCompletionRefusesUnverifiedSubmission(t *testing.T) {
	f := newCollaborationStageFixture(t)
	f.revise(t, true)
	options, err := f.svc.StatusOptions(f.handlerCtx, f.issue.Issue.ID, "submit_acceptance")
	if err != nil {
		t.Fatal(err)
	}
	before := f.stageInt(t, `SELECT count(*) FROM issue_handoff WHERE issue_id=$1`)
	p := CollaborationAcceptanceParams{CollaborationWorkflowRequest: f.req(t, f.handlerCtx), ExpectedStatus: options.DefaultStatus, StatusOptionsVersion: options.StatusOptionsVersion, Note: "An exact result", Basis: "Explicit verification basis"}
	receipt, err := f.svc.SubmitAcceptance(f.handlerCtx, p)
	if !collaborationErrorCode(err, "stage_incomplete") {
		t.Fatalf("unverified submission admitted: %+v %v", receipt, err)
	}
	if after := f.stageInt(t, `SELECT count(*) FROM issue_handoff WHERE issue_id=$1`); after != before {
		t.Fatalf("refusal created a handoff: before=%d after=%d", before, after)
	}
}

func TestStageCompletionRefusesUnverifiedSelfCompletion(t *testing.T) {
	f := newCollaborationStageFixture(t)
	_, err := f.svc.Transfer(f.ctx, CollaborationTransferParams{CollaborationParticipantParams: CollaborationParticipantParams{CollaborationWorkflowRequest: f.req(t, f.ctx), TargetUserID: util.MustParseUUID(f.creator)}})
	if err != nil {
		t.Fatal(err)
	}
	f.revise(t, true)
	options, err := f.svc.StatusOptions(f.ctx, f.issue.Issue.ID, "complete")
	if err != nil {
		t.Fatal(err)
	}
	p := CollaborationLifecycleParams{CollaborationWorkflowRequest: f.req(t, f.ctx), Action: "complete", TargetStatus: "done", StatusOptionsVersion: options.StatusOptionsVersion, Note: "Exact self result", Basis: "Verified manually"}
	receipt, err := f.svc.ChangeLifecycle(f.ctx, p)
	if !collaborationErrorCode(err, "stage_incomplete") {
		t.Fatalf("unverified self completion admitted: %+v %v", receipt, err)
	}
}

func newStageCompletionFixture(t *testing.T, setup ...func(*testing.T, *stageReviewFixture)) *stageExecutionFixture {
	t.Helper()
	f := newStageExecutionFixture(t, setup...)
	if err := f.svc.Queries.SeedIssueStatusEntries(f.ctx, util.MustParseUUID(f.workspace)); err != nil {
		t.Fatal(err)
	}
	f.fx.Cleanup(t, `DELETE FROM issue_status WHERE workspace_id=$1`, f.workspace)
	f.fx.Cleanup(t, `DELETE FROM collaboration_stage_submission WHERE workspace_id=$1`, f.workspace)
	request, release := f.release(t)
	receipt, err := f.svc.PrepareStageExecution(context.Background(), request, f.source.consumer(request))
	if err != nil || receipt.State != "succeeded" {
		t.Fatalf("controlled deployment: %+v %v", receipt, err)
	}
	if _, err = f.svc.AdvanceStage(f.ctx, f.stage.advanceParams(t, f.candidate, "verification", release)); err != nil {
		t.Fatal(err)
	}
	return f
}
func TestStageCompletionVerifiedSubmissionAndSelf(t *testing.T) {
	for _, self := range []bool{false, true} {
		t.Run(fmt.Sprint(self), func(t *testing.T) {
			f := newStageCompletionFixture(t)
			var submitted *pgtype.UUID
			if self {
				_, err := f.svc.Transfer(f.ctx, CollaborationTransferParams{CollaborationParticipantParams: CollaborationParticipantParams{CollaborationWorkflowRequest: f.stage.req(t, f.ctx), TargetUserID: util.MustParseUUID(f.creator)}})
				if err != nil {
					t.Fatal(err)
				}
			} else {
				options, err := f.svc.StatusOptions(f.stage.handlerCtx, f.issue.Issue.ID, "submit_acceptance")
				if err != nil {
					t.Fatal(err)
				}
				p := CollaborationAcceptanceParams{CollaborationWorkflowRequest: f.stage.req(t, f.stage.handlerCtx), ExpectedStatus: options.DefaultStatus, StatusOptionsVersion: options.StatusOptionsVersion, Note: "Exact result", Basis: "Controlled facts"}
				r, err := f.svc.SubmitAcceptance(f.stage.handlerCtx, p)
				if err != nil || r.DeliveryConfirmation == nil {
					t.Fatalf("verified submit: %+v %v", r, err)
				}
				submitted = &r.DeliveryConfirmation.SubmissionID
			}
			options, err := f.svc.StatusOptions(f.ctx, f.issue.Issue.ID, "complete")
			if err != nil {
				t.Fatal(err)
			}
			p := CollaborationLifecycleParams{CollaborationWorkflowRequest: f.stage.req(t, f.ctx), Action: "complete", TargetStatus: "done", StatusOptionsVersion: options.StatusOptionsVersion, SubmissionID: submitted, Note: "Accepted exact result"}
			if self {
				p.Note = "Self result"
				p.Basis = "Controlled facts"
			}
			result, err := f.svc.ChangeLifecycle(f.ctx, p)
			if err != nil || result.Code != "completed" {
				t.Fatalf("verified completion: %+v %v", result, err)
			}
			if n := f.fx.Count(t, `SELECT count(*) FROM collaboration_stage_submission WHERE issue_id=$1`, f.issue.Issue.ID); n != 1 {
				t.Fatalf("exact immutable binding count=%d", n)
			}
			replay, err := f.svc.ChangeLifecycle(f.ctx, p)
			if err != nil || replay.Code != "completed" {
				t.Fatalf("historical completion: %+v %v", replay, err)
			}
		})
	}
}
