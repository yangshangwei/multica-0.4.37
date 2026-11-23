package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func informationTestRequest(t *testing.T, svc *CollaborationService, id pgtype.UUID, ctx context.Context) CollaborationWorkflowRequest {
	t.Helper()
	access, err := svc.ResolveIssueAccess(ctx, nil, id)
	if err != nil {
		t.Fatal(err)
	}
	return CollaborationWorkflowRequest{CollaborationQueueRequest: CollaborationQueueRequest{IssueID: id, RequestID: uuid.NewString(),
		ExpectedLifecycleEpoch: access.LifecycleEpoch, ExpectedParticipantEpoch: access.ParticipantEpoch.Int64, ExpectedAccountEpoch: access.AccountEpoch.Int64},
		ExpectedCollaborationRevision: access.Revision}
}

func informationTestAsk(t *testing.T, svc *CollaborationService, id pgtype.UUID, ctx context.Context, target string) CollaborationInformationRequestParams {
	t.Helper()
	options, err := svc.StatusOptions(ctx, id, "request_information")
	if err != nil {
		t.Fatal(err)
	}
	condition, estimate := "Supply the original callback identifier.", informationTime(time.Now().Add(time.Hour).Truncate(time.Microsecond))
	return CollaborationInformationRequestParams{CollaborationTransferParams: CollaborationTransferParams{CollaborationParticipantParams: CollaborationParticipantParams{
		CollaborationWorkflowRequest: informationTestRequest(t, svc, id, ctx), TargetUserID: util.MustParseUUID(target)}, Note: "The original callback evidence is missing."},
		ExpectedStatus: options.DefaultStatus, StatusOptionsVersion: options.StatusOptionsVersion, UnblockCondition: &condition, EstimatedResolutionAt: &estimate}
}

func informationTestRepair(t *testing.T, svc *CollaborationService, id pgtype.UUID, ctx context.Context, target, status string, historical bool) CollaborationInformationRepairParams {
	t.Helper()
	options, err := svc.StatusOptions(ctx, id, "repair_information")
	if err != nil || options.InformationRepairContext == nil {
		t.Fatalf("repair options prerequisite: %+v %v", options, err)
	}
	basis := CollaborationInformationRepairBasis{Kind: "human_selected"}
	if historical {
		origin := options.InformationRepairContext.HistoricalOrigin
		if origin == nil {
			t.Fatal("historical basis prerequisite is absent")
		}
		basis = CollaborationInformationRepairBasis{Kind: "historical_information", OriginRequestID: &origin.OriginRequestID, OriginEventID: &origin.OriginEventID}
	}
	return CollaborationInformationRepairParams{CollaborationTransferParams: CollaborationTransferParams{CollaborationParticipantParams: CollaborationParticipantParams{
		CollaborationWorkflowRequest: informationTestRequest(t, svc, id, ctx), TargetUserID: util.MustParseUUID(target)}, Note: "Explicitly repair the unavailable information path."},
		ExpectedStatus: status, StatusOptionsVersion: options.StatusOptionsVersion,
		InformationRepair: CollaborationInformationRepairSelection{ContextVersion: options.InformationRepairContext.ContextVersion,
			ActiveRequestID: options.InformationRepairContext.ActiveRequestID, Basis: basis}}
}

func informationTestReturn(t *testing.T, svc *CollaborationService, id pgtype.UUID, ctx context.Context, request pgtype.UUID) CollaborationInformationReturnParams {
	t.Helper()
	options, err := svc.StatusOptions(ctx, id, "return_information")
	if err != nil {
		t.Fatal(err)
	}
	return CollaborationInformationReturnParams{CollaborationWorkflowRequest: informationTestRequest(t, svc, id, ctx), InformationRequestID: request,
		ExpectedStatus: options.DefaultStatus, StatusOptionsVersion: options.StatusOptionsVersion, Note: "The requested evidence is now available."}
}

func TestCollaborationInformationManagedMetadataAndLegacyRecovery(t *testing.T) {
	t.Setenv("MULTICA_AUTH_MODE", "legacy")
	f := newCollaborationStageFixture(t)
	id := f.issue.Issue.ID
	legacy := informationTestAsk(t, f.svc, id, f.handlerCtx, f.creator)
	legacy.Note, legacy.UnblockCondition, legacy.EstimatedResolutionAt = "", nil, nil
	oldBody, err := json.Marshal(struct {
		CollaborationTransferParams
		ExpectedStatus       string
		StatusOptionsVersion string
	}{legacy.CollaborationTransferParams, legacy.ExpectedStatus, legacy.StatusOptionsVersion})
	if err != nil {
		t.Fatal(err)
	}
	accepted, err := f.svc.RequestInformation(f.handlerCtx, legacy)
	if err != nil || accepted.Code != "information_requested" {
		t.Fatalf("old unmanaged shape: %+v %v", accepted, err)
	}
	operation, err := f.svc.Queries.GetCollaborationOperation(f.ctx, db.GetCollaborationOperationParams{ScopeKind: "issue", ScopeID: id,
		ActorUserID: util.MustParseUUID(f.handler), RequestID: legacy.RequestID})
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(oldBody)
	if operation.PayloadHash != hex.EncodeToString(digest[:]) {
		t.Fatal("omitted metadata changed the old request hash")
	}
	f.revise(t, true)
	before := sourceRoleBusinessSnapshot(t, f.collaborationQueueFixture)
	if replay, err := f.svc.RequestInformation(f.handlerCtx, legacy); err != nil || !reflect.DeepEqual(replay, accepted) ||
		before != sourceRoleBusinessSnapshot(t, f.collaborationQueueFixture) {
		t.Fatalf("old key was revalidated as a new managed request: %+v %v", replay, err)
	}
	request := activeInformationRequest(t, f.collaborationQueueFixture)
	row, err := f.svc.Queries.GetCollaborationInformationRequest(f.ctx, db.GetCollaborationInformationRequestParams{IssueID: id, RequestID: request})
	if err != nil || row.UnblockCondition.Valid || row.EstimatedResolutionAt.Valid || informationBlock(row).State != "legacy" {
		t.Fatalf("legacy metadata was fabricated: %+v %v", row, err)
	}
	missing := informationTestAsk(t, f.svc, id, f.ctx, f.handler)
	missing.UnblockCondition, missing.EstimatedResolutionAt = nil, nil
	refused, err := f.svc.RequestInformation(f.ctx, missing)
	if !collaborationErrorCode(err, "information_metadata_required") || refused.Status != 400 {
		t.Fatalf("managed omission was accepted: %+v %v", refused, err)
	}
	repair := informationTestRepair(t, f.svc, id, f.ctx, f.creator, "todo", false)
	if result, err := f.svc.RepairInformation(f.ctx, repair); err != nil || result.InformationRepairConfirmation == nil || result.InformationRepairConfirmation.SupersededCount != "1" {
		t.Fatalf("legacy repair: %+v %v", result, err)
	}
	if replay, err := f.svc.RequestInformation(f.ctx, missing); !collaborationErrorCode(err, "information_metadata_required") || !reflect.DeepEqual(replay, refused) {
		t.Fatalf("old omission refusal recomputed after repair: %+v %v", replay, err)
	}
}

func TestCollaborationInformationRepairRestoresReturnableContext(t *testing.T) {
	for _, scenario := range []string{"creator_replacement", "recipient_removed", "legacy_metadata", "partial_metadata", "cycle", "missing_parent", "requester_removed", "requester_disabled"} {
		t.Run(scenario, func(t *testing.T) {
			t.Setenv("MULTICA_AUTH_MODE", "legacy")
			f := newCollaborationStageFixture(t)
			f.revise(t, true)
			id := f.issue.Issue.ID
			target, recipientCtx := f.creator, f.ctx
			if scenario == "creator_replacement" || scenario == "recipient_removed" {
				target, recipientCtx = f.qa, f.qaCtx
			}
			ask := informationTestAsk(t, f.svc, id, f.handlerCtx, target)
			if _, err := f.svc.RequestInformation(f.handlerCtx, ask); err != nil {
				t.Fatal(err)
			}
			request := activeInformationRequest(t, f.collaborationQueueFixture)
			if scenario == "creator_replacement" {
				current := mustAccess(t, f.collaborationQueueFixture, f.ctx)
				if current.Issue.AssigneeID != util.MustParseUUID(f.qa) || current.Issue.AssigneeID == util.MustParseUUID(f.creator) ||
					current.StatusCategory != "blocked" || !current.Capabilities.Transfer {
					t.Fatal("creator replacement must start with a blocked request addressed to another current handler")
				}
			}
			var priorReturn *CollaborationInformationReturnParams
			if scenario == "requester_removed" || scenario == "requester_disabled" {
				value := informationTestReturn(t, f.svc, id, recipientCtx, request)
				priorReturn = &value
			}
			switch scenario {
			case "recipient_removed":
				if _, err := f.svc.RemoveParticipant(f.ctx, CollaborationParticipantParams{CollaborationWorkflowRequest: f.req(t, f.ctx), TargetUserID: util.MustParseUUID(f.qa)}); err != nil {
					t.Fatal(err)
				}
			case "legacy_metadata":
				f.fx.Exec(t, `UPDATE issue_handoff SET unblock_condition=NULL,estimated_resolution_at=NULL WHERE id=$1`, request)
			case "partial_metadata":
				f.fx.Exec(t, `UPDATE issue_handoff SET estimated_resolution_at=NULL WHERE id=$1`, request)
			case "cycle":
				f.fx.Exec(t, `UPDATE issue_handoff SET parent_request_id=id,previous_status='blocked' WHERE id=$1`, request)
			case "missing_parent":
				f.fx.Exec(t, `UPDATE issue_handoff SET parent_request_id=$2,previous_status='blocked' WHERE id=$1`, request, uuid.NewString())
			case "requester_removed":
				if _, err := f.svc.RemoveParticipant(f.ctx, CollaborationParticipantParams{CollaborationWorkflowRequest: f.req(t, f.ctx), TargetUserID: util.MustParseUUID(f.handler)}); err != nil {
					t.Fatal(err)
				}
			case "requester_disabled":
				f.fx.Exec(t, `UPDATE "user" SET disabled_at=now() WHERE id=$1`, f.handler)
			}
			if priorReturn != nil {
				priorReturn.CollaborationWorkflowRequest = f.req(t, recipientCtx)
				if r, err := f.svc.ReturnInformation(recipientCtx, *priorReturn); !collaborationErrorCode(err, "return_target_unavailable") {
					t.Fatalf("unavailable requester was reenrolled: %+v %v", r, err)
				}
			}
			beforeRequest := sourceRoleBusinessSnapshot(t, f.collaborationQueueFixture)
			if scenario != "requester_removed" && scenario != "requester_disabled" {
				probe := informationTestAsk(t, f.svc, id, f.ctx, f.handler)
				if r, err := f.svc.RequestInformation(f.ctx, probe); !collaborationErrorCode(err, "information_repair_required") || beforeRequest != sourceRoleBusinessSnapshot(t, f.collaborationQueueFixture) {
					t.Fatalf("blocked replacement/nest acquired a false return path: %+v %v", r, err)
				}
			}
			beforeAI := sourceRoleExecutionSnapshot(t, f.collaborationQueueFixture)
			beforeInputs := sourceAuthorityExecutionState(t, f.collaborationQueueFixture)
			repair := informationTestRepair(t, f.svc, id, f.ctx, f.creator, "todo", scenario == "creator_replacement" || scenario == "requester_removed" || scenario == "requester_disabled")
			result, err := f.svc.RepairInformation(f.ctx, repair)
			if err != nil || result.Code != "information_repaired" || result.InformationRepairConfirmation == nil {
				t.Fatalf("explicit repair failed: %+v %v", result, err)
			}
			confirmation := result.InformationRepairConfirmation
			count := "1"
			if scenario == "recipient_removed" {
				count = "0"
			}
			if confirmation.SupersededCount != count || confirmation.HandlerUserID != util.MustParseUUID(f.creator) || confirmation.Status != "todo" ||
				beforeAI != sourceRoleExecutionSnapshot(t, f.collaborationQueueFixture) || beforeInputs != sourceAuthorityExecutionState(t, f.collaborationQueueFixture) {
				t.Fatal("repair changed execution or credited old security supersession")
			}
			current := mustAccess(t, f.collaborationQueueFixture, f.ctx)
			if current.StatusCategory != "todo" || current.InformationRequestID.Valid || f.fx.Count(t, `SELECT count(*) FROM collaboration_stage_task WHERE issue_id=$1`, id) != 0 {
				t.Fatal("repair did not commit standalone nonblocked state or created a stage head")
			}
			repaired, err := f.svc.Queries.GetCollaborationHandoffByID(f.ctx, db.GetCollaborationHandoffByIDParams{IssueID: id, ID: confirmation.RepairHandoffID})
			expectedFrom := util.MustParseUUID(f.creator)
			if scenario == "creator_replacement" {
				expectedFrom = util.MustParseUUID(f.qa)
			}
			if err != nil || repaired.Purpose != "transfer" || repaired.PreviousStatus != "blocked" || repaired.FromUserID != expectedFrom || repaired.ToUserID != util.MustParseUUID(f.creator) {
				t.Fatalf("repair fabricated its actual from/to handler or prior status: %+v %v", repaired, err)
			}
			t.Logf("information_repair_scenario=%s prior_handler=%s restored_handler=%s prior_status=%s", scenario, util.UUIDToString(repaired.FromUserID), util.UUIDToString(repaired.ToUserID), repaired.PreviousStatus)
			old, err := f.svc.Queries.GetCollaborationInformationRequest(f.ctx, db.GetCollaborationInformationRequestParams{IssueID: id, RequestID: request})
			if err != nil || old.Resolution != "superseded" || (scenario == "recipient_removed" && old.SupersededByRepairHandoffID.Valid) ||
				(scenario != "recipient_removed" && old.SupersededByRepairHandoffID != confirmation.RepairHandoffID) {
				t.Fatalf("repair supersession relation: %+v %v", old, err)
			}
			if scenario == "requester_removed" || scenario == "requester_disabled" {
				if f.fx.Count(t, `SELECT count(*) FROM issue_participant WHERE issue_id=$1 AND user_id=$2 AND revoked_at IS NULL`, id, f.handler) != map[string]int{"requester_removed": 0, "requester_disabled": 1}[scenario] {
					t.Fatal("repair reenrolled the old requester")
				}
			}
			// B2 is a separate explicit action. A usable target is picked after
			// requester-disable instead of restoring that unavailable account.
			next := informationTestAsk(t, f.svc, id, f.ctx, f.release)
			if next.RequestID == repair.RequestID {
				t.Fatal("B2 reused the repair request identity")
			}
			if _, err := f.svc.RequestInformation(f.ctx, next); err != nil {
				t.Fatal(err)
			}
			nextID := activeInformationRequest(t, f.collaborationQueueFixture)
			row, err := f.svc.Queries.GetCollaborationInformationRequest(f.ctx, db.GetCollaborationInformationRequestParams{IssueID: id, RequestID: nextID})
			if err != nil || row.PreviousStatus != "todo" || row.RequesterUserID != util.MustParseUUID(f.creator) || row.ParentRequestID.Valid {
				t.Fatalf("B2 did not save actual repaired state: %+v %v", row, err)
			}
			if _, err := f.svc.ReturnInformation(f.releaseCtx, informationTestReturn(t, f.svc, id, f.releaseCtx, nextID)); err != nil {
				t.Fatal(err)
			}
			if replay, err := f.svc.RepairInformation(f.ctx, repair); err != nil || !reflect.DeepEqual(replay, result) {
				t.Fatalf("original repair changed after independent B2/return: %+v %v", replay, err)
			}
		})
	}
}

func TestCollaborationInformationCompleteNestedReturns(t *testing.T) {
	t.Setenv("MULTICA_AUTH_MODE", "legacy")
	f := newCollaborationStageFixture(t)
	f.revise(t, true)
	id := f.issue.Issue.ID
	if _, err := f.svc.RequestInformation(f.handlerCtx, informationTestAsk(t, f.svc, id, f.handlerCtx, f.creator)); err != nil {
		t.Fatal(err)
	}
	first := activeInformationRequest(t, f.collaborationQueueFixture)
	if _, err := f.svc.RequestInformation(f.ctx, informationTestAsk(t, f.svc, id, f.ctx, f.qa)); err != nil {
		t.Fatal(err)
	}
	second := activeInformationRequest(t, f.collaborationQueueFixture)
	if _, err := f.svc.ReturnInformation(f.qaCtx, informationTestReturn(t, f.svc, id, f.qaCtx, second)); err != nil {
		t.Fatal(err)
	}
	if activeInformationRequest(t, f.collaborationQueueFixture) != first || mustAccess(t, f.collaborationQueueFixture, f.ctx).StatusCategory != "blocked" {
		t.Fatal("nested return did not reactivate the exact parent")
	}
	if _, err := f.svc.ReturnInformation(f.ctx, informationTestReturn(t, f.svc, id, f.ctx, first)); err != nil {
		t.Fatal(err)
	}
	if activeInformationRequest(t, f.collaborationQueueFixture).Valid || mustAccess(t, f.collaborationQueueFixture, f.ctx).StatusCategory != "todo" ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1 AND kind='information_repaired'`, id) != 0 {
		t.Fatal("normal nested chain used repair or failed to restore the original status")
	}
}

func TestCollaborationInformationLegacyBlockedPolicyAndAssociation(t *testing.T) {
	for _, mode := range []string{"policy_activation", "project_association"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("MULTICA_AUTH_MODE", "legacy")
			f := newCollaborationStageFixture(t)
			id := f.issue.Issue.ID
			options, err := f.svc.StatusOptions(f.ctx, id, "change_status")
			if err != nil {
				t.Fatal(err)
			}
			if _, err = f.svc.ChangeStatus(f.ctx, CollaborationStatusParams{CollaborationWorkflowRequest: f.req(t, f.ctx),
				TargetStatus: "blocked", StatusOptionsVersion: options.StatusOptionsVersion}); err != nil {
				t.Fatal(err)
			}
			if mode == "policy_activation" {
				f.revise(t, true)
			} else {
				project := util.MustParseUUID(f.fx.Project(t, "New managed association"))
				if _, err = f.svc.ReviseStagePolicy(f.ctx, StagePolicyRevisionParams{WorkspaceID: util.MustParseUUID(f.workspace), ProjectID: project,
					RequestID: uuid.NewString(), Enabled: true, Reason: "Managed association fixture", Credential: stageCredential(f.creator)}); err != nil {
					t.Fatal(err)
				}
				if _, err = f.svc.EditIssue(f.ctx, CollaborationEditParams{CollaborationWorkflowRequest: f.req(t, f.ctx), ProjectID: &project,
					ProjectChangeReason: "Move the legacy root into current governance"}); err != nil {
					t.Fatal(err)
				}
			}
			view, err := f.svc.InformationWorkflowView(f.ctx, mustAccess(t, f.collaborationQueueFixture, f.ctx))
			if err != nil || view == nil || !view.Managed || view.RecordState != "none" || view.ActiveRequestID.Valid || !view.Actions.Repair.Available || view.Actions.Request.Available {
				t.Fatalf("legacy standalone blocked context was invented or not repairable: %+v %v", view, err)
			}
			repair := informationTestRepair(t, f.svc, id, f.ctx, f.creator, "todo", false)
			if result, err := f.svc.RepairInformation(f.ctx, repair); err != nil || result.InformationRepairConfirmation == nil || result.InformationRepairConfirmation.SupersededCount != "0" {
				t.Fatalf("legacy standalone repair: %+v %v", result, err)
			}
			if f.fx.Count(t, `SELECT count(*) FROM issue_handoff WHERE issue_id=$1 AND purpose='information'`, id) != 0 {
				t.Fatal("repair fabricated an information request")
			}
		})
	}
}

func TestCollaborationInformationCurrentActionabilityAndRawPointer(t *testing.T) {
	t.Setenv("MULTICA_AUTH_MODE", "legacy")
	f := newCollaborationStageFixture(t)
	f.revise(t, true)
	id := f.issue.Issue.ID
	if _, err := f.svc.RequestInformation(f.handlerCtx, informationTestAsk(t, f.svc, id, f.handlerCtx, f.creator)); err != nil {
		t.Fatal(err)
	}
	request := activeInformationRequest(t, f.collaborationQueueFixture)
	f.fx.Exec(t, `UPDATE "user" SET disabled_at=now() WHERE id=$1`, f.handler)
	view, err := f.svc.InformationWorkflowView(f.ctx, mustAccess(t, f.collaborationQueueFixture, f.ctx))
	if err != nil || view == nil || view.RecordState != "complete" || view.Actions.Return.Available || !view.Actions.Repair.Available ||
		view.Actions.Return.Code == nil || *view.Actions.Return.Code != "return_target_unavailable" {
		t.Fatalf("complete metadata was confused with current actionability: %+v %v", view, err)
	}
	f.fx.Exec(t, `UPDATE "user" SET disabled_at=NULL WHERE id=$1`, f.handler)
	dangling := util.MustParseUUID(uuid.NewString())
	f.fx.Exec(t, `UPDATE issue_collaboration SET active_information_request_id=$2 WHERE issue_id=$1`, id, dangling)
	access := mustAccess(t, f.collaborationQueueFixture, f.ctx)
	if access.InformationRequestID.Valid {
		t.Fatal("legacy joined capability unexpectedly trusted a dangling pointer")
	}
	view, err = f.svc.InformationWorkflowView(f.ctx, access)
	if err != nil || view == nil || view.ActiveRequestID != dangling || view.RecordState != "unavailable" {
		t.Fatalf("real dangling pointer was collapsed to none: %+v %v", view, err)
	}
	f.fx.Exec(t, `UPDATE issue_collaboration SET active_information_request_id=$2 WHERE issue_id=$1`, id, request)
}
