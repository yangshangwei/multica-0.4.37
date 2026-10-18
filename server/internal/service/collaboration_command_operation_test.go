package service

import (
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/util"
)

func TestCollaborationCommandOperationRecoveryAndDepartureRedaction(t *testing.T) {
	f := newCollaborationQueueFixture(t)
	input := f.submit(t, f.handler, "cancel and recover")
	actor := collaborationContext(f.handler)
	p := CollaborationCancelCommandParams{CollaborationQueueRequest: f.request(), TargetID: f.targetID, CommandID: input.CommandID, ExpectedCollaborationRevision: 1}
	r, err := f.svc.CancelWaitingCommand(actor, p)
	if err != nil {
		t.Fatal(err)
	}
	recovered, err := f.svc.GetCommandOperation(actor, f.issue.Issue.ID, p.RequestID)
	if err != nil || recovered.Code != "command_cancelled" || recovered.Action != "cancel_command" || recovered.CommandID != input.CommandID || recovered.Redacted {
		t.Fatalf("original command cancellation not recovered: %+v %v", recovered, err)
	}
	if _, err = f.svc.GetCommandOperation(f.ctx, f.issue.Issue.ID, p.RequestID); !collaborationErrorCode(err, "operation_not_found") {
		t.Fatalf("another actor recovered private cancellation: %v", err)
	}
	if _, err = f.svc.GetCommandOperation(actor, f.issue.Issue.ID, uuid.NewString()); !collaborationErrorCode(err, "operation_not_found") {
		t.Fatalf("missing key pretended committed: %v", err)
	}
	if _, err = f.svc.RemoveParticipant(f.ctx, CollaborationParticipantParams{CollaborationWorkflowRequest: workflowRequest(f, 1), TargetUserID: util.MustParseUUID(f.handler)}); err != nil {
		t.Fatal(err)
	}
	recovered, err = f.svc.GetCommandOperation(actor, f.issue.Issue.ID, p.RequestID)
	if err != nil || !recovered.Redacted || recovered.CommandID.Valid || recovered.TargetID.Valid || recovered.GroupID.Valid || recovered.Code != r.Code {
		t.Fatalf("departure leaked cancellation identity or lost original status: %+v %v", recovered, err)
	}
}
