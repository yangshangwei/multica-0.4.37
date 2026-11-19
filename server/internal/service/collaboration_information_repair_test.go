package service

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func informationPendingSnapshot(t *testing.T, f *stageExecutionFixture) string {
	t.Helper()
	var value string
	f.fx.QueryRow(t, `SELECT jsonb_build_object(
		'submissions',(SELECT jsonb_agg(to_jsonb(s) ORDER BY s.submission_id) FROM collaboration_stage_submission s WHERE s.issue_id=$1),
		'events',(SELECT jsonb_agg(to_jsonb(e) ORDER BY e.event_seq) FROM collaboration_event e WHERE e.issue_id=$1 AND e.kind IN ('acceptance_submitted','completed','acceptance_returned','cancelled')),
		'decisions',(SELECT jsonb_agg(to_jsonb(d) ORDER BY d.decision_id) FROM collaboration_stage_decision d WHERE d.issue_id=$1),
		'authorizations',(SELECT jsonb_agg(to_jsonb(a) ORDER BY a.id) FROM delivery_completion_authorization a WHERE a.issue_id=$1),
		'attachments',(SELECT jsonb_agg(to_jsonb(a) ORDER BY a.id) FROM collaboration_attachment a WHERE a.issue_id=$1)
	)::text`, f.issue.Issue.ID).Scan(&value)
	return value
}

func TestCollaborationInformationRepairPreservesPendingSubmission(t *testing.T) {
	t.Setenv("MULTICA_AUTH_MODE", "legacy")
	f := newStageCompletionFixture(t)
	t.Cleanup(func() { f.fx.Exec(t, `DELETE FROM issue_handoff WHERE issue_id=$1`, f.issue.Issue.ID) })
	submission := stageCompletionSubmit(t, f)
	id := f.issue.Issue.ID
	if _, err := f.svc.RequestInformation(f.ctx, informationTestAsk(t, f.svc, id, f.ctx, f.stage.qa)); err != nil {
		t.Fatal(err)
	}
	before := informationPendingSnapshot(t, f)
	options, err := f.svc.StatusOptions(f.stage.qaCtx, id, "repair_information")
	if err != nil || options.InformationRepairContext == nil || options.InformationRepairContext.PendingSubmission == nil ||
		options.DefaultStatus != "" || options.InformationRepairContext.HumanSelectionAllowed || options.InformationRepairContext.HistoricalOrigin != nil {
		t.Fatalf("pending repair context did not take priority: %+v %v", options, err)
	}
	pending := options.InformationRepairContext.PendingSubmission
	if pending.SubmissionID != submission || pending.CreatorUserID != util.MustParseUUID(f.creator) {
		t.Fatal("pending context changed original submission or required creator")
	}
	params := CollaborationInformationRepairParams{CollaborationTransferParams: CollaborationTransferParams{CollaborationParticipantParams: CollaborationParticipantParams{
		CollaborationWorkflowRequest: informationTestRequest(t, f.svc, id, f.stage.qaCtx), TargetUserID: util.MustParseUUID(f.creator)}, Note: "Return the unchanged pending result to its creator."},
		ExpectedStatus: "in_review", StatusOptionsVersion: options.StatusOptionsVersion,
		InformationRepair: CollaborationInformationRepairSelection{ContextVersion: options.InformationRepairContext.ContextVersion,
			ActiveRequestID: options.InformationRepairContext.ActiveRequestID, Basis: CollaborationInformationRepairBasis{Kind: "pending_submission",
				SubmissionID: &pending.SubmissionID, SubmissionEventID: &pending.SubmissionEventID}}}
	for _, mutation := range []string{"human_basis", "wrong_target", "wrong_submission", "wrong_event", "ordinary_work"} {
		t.Run(mutation, func(t *testing.T) {
			probe := params
			probe.CollaborationWorkflowRequest = informationTestRequest(t, f.svc, id, f.stage.qaCtx)
			switch mutation {
			case "human_basis":
				probe.InformationRepair.Basis = CollaborationInformationRepairBasis{Kind: "human_selected"}
			case "wrong_target":
				probe.TargetUserID = util.MustParseUUID(f.stage.qa)
			case "wrong_submission":
				wrong := util.MustParseUUID(uuid.NewString())
				probe.InformationRepair.Basis.SubmissionID = &wrong
			case "wrong_event":
				wrong := util.MustParseUUID(uuid.NewString())
				probe.InformationRepair.Basis.SubmissionEventID = &wrong
			case "ordinary_work":
				probe.ExpectedStatus = "todo"
			}
			if receipt, err := f.svc.RepairInformation(f.stage.qaCtx, probe); err == nil || receipt.Status != 409 || before != informationPendingSnapshot(t, f) {
				t.Fatalf("pending repair bypassed original evidence/creator: %+v %v", receipt, err)
			}
		})
	}
	result, err := f.svc.RepairInformation(f.stage.qaCtx, params)
	if err != nil || result.InformationRepairConfirmation == nil || before != informationPendingSnapshot(t, f) ||
		result.InformationRepairConfirmation.HandlerUserID != util.MustParseUUID(f.creator) || result.InformationRepairConfirmation.Status != "in_review" {
		t.Fatalf("legal pending repair changed evidence or acceptance authority: %+v %v", result, err)
	}
	current := mustAccess(t, f.collaborationQueueFixture, f.ctx)
	if current.StatusCategory != "in_review" || current.InformationRequestID.Valid || current.LifecycleEpoch != 1 {
		t.Fatal("pending repair changed lifecycle or left the block active")
	}
	summary := deliverySummaryForTest(t, f.collaborationQueueFixture, f.ctx)
	if summary.Current == nil || summary.Current.ID != submission || summary.Current.Decision != nil {
		t.Fatal("repair consumed or replaced the pending delivery")
	}
}

func TestCollaborationInformationRepairRefusesUnavailablePendingCreator(t *testing.T) {
	t.Setenv("MULTICA_AUTH_MODE", "legacy")
	f := newStageCompletionFixture(t)
	t.Cleanup(func() { f.fx.Exec(t, `DELETE FROM issue_handoff WHERE issue_id=$1`, f.issue.Issue.ID) })
	stageCompletionSubmit(t, f)
	id := f.issue.Issue.ID
	if _, err := f.svc.RequestInformation(f.ctx, informationTestAsk(t, f.svc, id, f.ctx, f.stage.qa)); err != nil {
		t.Fatal(err)
	}
	options, err := f.svc.StatusOptions(f.stage.qaCtx, id, "repair_information")
	if err != nil || options.InformationRepairContext == nil || options.InformationRepairContext.PendingSubmission == nil {
		t.Fatal("pending prerequisite unavailable")
	}
	pending := options.InformationRepairContext.PendingSubmission
	params := CollaborationInformationRepairParams{CollaborationTransferParams: CollaborationTransferParams{CollaborationParticipantParams: CollaborationParticipantParams{
		CollaborationWorkflowRequest: informationTestRequest(t, f.svc, id, f.stage.qaCtx), TargetUserID: util.MustParseUUID(f.creator)}, Note: "Preserve the required pending creator."},
		ExpectedStatus: "in_review", StatusOptionsVersion: options.StatusOptionsVersion,
		InformationRepair: CollaborationInformationRepairSelection{ContextVersion: options.InformationRepairContext.ContextVersion, ActiveRequestID: options.InformationRepairContext.ActiveRequestID,
			Basis: CollaborationInformationRepairBasis{Kind: "pending_submission", SubmissionID: &pending.SubmissionID, SubmissionEventID: &pending.SubmissionEventID}}}
	f.fx.Exec(t, `UPDATE "user" SET disabled_at=now() WHERE id=$1`, f.creator)
	view, err := f.svc.InformationWorkflowView(f.stage.qaCtx, mustAccess(t, f.collaborationQueueFixture, f.stage.qaCtx))
	if err != nil || view == nil || view.Actions.Repair.Available || view.Actions.Repair.Code == nil || *view.Actions.Repair.Code != "return_target_unavailable" {
		t.Fatalf("unavailable creator offered a proxy repair: %+v %v", view, err)
	}
	params.InformationRepair.ContextVersion = view.ContextVersion
	params.InformationRepair.ActiveRequestID = view.ActiveRequestID
	before := informationPendingSnapshot(t, f)
	if result, err := f.svc.RepairInformation(f.stage.qaCtx, params); !collaborationErrorCode(err, "return_target_unavailable") || before != informationPendingSnapshot(t, f) {
		t.Fatalf("repair substituted a disabled pending creator: %+v %v", result, err)
	}
}

func TestCollaborationInformationRepairOriginalKeysAndUnknownCommit(t *testing.T) {
	t.Setenv("MULTICA_AUTH_MODE", "legacy")
	f := newCollaborationStageFixture(t)
	f.revise(t, true)
	id := f.issue.Issue.ID
	if _, err := f.svc.RequestInformation(f.handlerCtx, informationTestAsk(t, f.svc, id, f.handlerCtx, f.creator)); err != nil {
		t.Fatal(err)
	}
	staleB2 := informationTestAsk(t, f.svc, id, f.ctx, f.release)
	repair := informationTestRepair(t, f.svc, id, f.ctx, f.creator, "todo", true)
	fault := &queueFaultStarter{TxStarter: f.pool, lostCommit: true}
	f.svc.TxStarter = fault
	if _, err := f.svc.RepairInformation(f.ctx, repair); err == nil || fault.begins.Load() != 1 {
		t.Fatalf("lost repair ACK automatically retried: %v", err)
	}
	f.svc.TxStarter = f.pool
	recovered, err := f.svc.GetWorkflowOperation(f.ctx, id, repair.RequestID)
	if err != nil || recovered.Code != "information_repaired" || recovered.InformationRepairConfirmation == nil {
		t.Fatalf("original repair GET did not settle uncertainty: %+v %v", recovered, err)
	}
	if result, err := f.svc.RequestInformation(f.ctx, staleB2); !collaborationErrorCode(err, "stale_revision") || result.Code != "stale_revision" {
		t.Fatalf("separate stale B2 overwrote completed repair: %+v %v", result, err)
	}
	if mustAccess(t, f.collaborationQueueFixture, f.ctx).StatusCategory != "todo" ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1 AND kind='information_repaired'`, id) != 1 {
		t.Fatal("lost ACK recovery or stale B2 duplicated repair")
	}
	freshB2 := informationTestAsk(t, f.svc, id, f.ctx, f.release)
	fault = &queueFaultStarter{TxStarter: f.pool, lostCommit: true}
	f.svc.TxStarter = fault
	if _, err := f.svc.RequestInformation(f.ctx, freshB2); err == nil || fault.begins.Load() != 1 {
		t.Fatal("independent B2 ACK uncertainty was not retained")
	}
	f.svc.TxStarter = f.pool
	if result, err := f.svc.GetWorkflowOperation(f.ctx, id, freshB2.RequestID); err != nil || result.Code != "information_requested" {
		t.Fatalf("B2 original receipt was confused with repair: %+v %v", result, err)
	}
	request := activeInformationRequest(t, f.collaborationQueueFixture)
	if _, err := f.svc.ReturnInformation(f.releaseCtx, informationTestReturn(t, f.svc, id, f.releaseCtx, request)); err != nil {
		t.Fatal(err)
	}
	before := sourceRoleBusinessSnapshot(t, f.collaborationQueueFixture)
	result, err := f.svc.RepairInformation(f.ctx, repair)
	if err != nil || result.Code != recovered.Code || !reflect.DeepEqual(result.InformationRepairConfirmation, recovered.InformationRepairConfirmation) ||
		before != sourceRoleBusinessSnapshot(t, f.collaborationQueueFixture) {
		t.Fatalf("old repair replay changed after fresh B2/return: %+v %v", result, err)
	}
	changed := repair
	changed.Note += " changed"
	if _, err := f.svc.RepairInformation(f.ctx, changed); !collaborationErrorCode(err, "idempotency_conflict") {
		t.Fatalf("same repair key accepted a changed explanation: %v", err)
	}
	operation, err := f.svc.Queries.GetCollaborationOperation(f.ctx, db.GetCollaborationOperationParams{ScopeKind: "issue", ScopeID: id,
		ActorUserID: util.MustParseUUID(f.creator), RequestID: repair.RequestID})
	if err != nil || operation.ResultCode.String != "information_repaired" {
		t.Fatal("original repair operation lost its identity")
	}
	var stored CollaborationWorkflowReceipt
	if json.Unmarshal(operation.Result, &stored) != nil || stored.InformationRepairConfirmation == nil || !reflect.DeepEqual(stored.InformationRepairConfirmation, recovered.InformationRepairConfirmation) {
		t.Fatal("old repair result JSON was overwritten")
	}
}

func TestCollaborationInformationRepairRequiredWritesRollback(t *testing.T) {
	for _, needle := range []string{"INSERT INTO issue_participant", "UPDATE issue AS i SET", "UPDATE issue SET assignee_type", "INSERT INTO issue_handoff",
		"superseded_by_repair_handoff_id =", "UPDATE issue_collaboration SET active_information_request_id = NULL", "UPDATE issue_collaboration SET revision",
		"INSERT INTO collaboration_event", "INSERT INTO collaboration_notification", "UPDATE collaboration_operation"} {
		t.Run(needle, func(t *testing.T) {
			t.Setenv("MULTICA_AUTH_MODE", "legacy")
			f := newCollaborationStageFixture(t)
			f.revise(t, true)
			id := f.issue.Issue.ID
			if _, err := f.svc.RequestInformation(f.handlerCtx, informationTestAsk(t, f.svc, id, f.handlerCtx, f.creator)); err != nil {
				t.Fatal(err)
			}
			repair := informationTestRepair(t, f.svc, id, f.ctx, f.outsider, "todo", false)
			before := sourceRoleBusinessSnapshot(t, f.collaborationQueueFixture)
			beforeAI := sourceRoleExecutionSnapshot(t, f.collaborationQueueFixture)
			f.svc.TxStarter = workflowFaultStarter{TxStarter: f.pool, needle: needle}
			if result, err := f.svc.RepairInformation(f.ctx, repair); err == nil {
				t.Fatalf("required write failed but repair reported success: %+v", result)
			}
			f.svc.TxStarter = f.pool
			if before != sourceRoleBusinessSnapshot(t, f.collaborationQueueFixture) || beforeAI != sourceRoleExecutionSnapshot(t, f.collaborationQueueFixture) ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_operation WHERE scope_id=$1 AND request_id=$2`, id, repair.RequestID) != 0 {
				t.Fatal("required repair failure left partial rows, links, effects or receipt")
			}
			if result, err := f.svc.RepairInformation(f.ctx, repair); err != nil || result.InformationRepairConfirmation == nil || result.InformationRepairConfirmation.SupersededCount != "1" {
				t.Fatalf("original identity could not recover definite rollback: %+v %v", result, err)
			}
		})
	}
}
