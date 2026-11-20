package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func TestCollaborationInformationRuntimeV1SourcesAndRepairPreserveExecution(t *testing.T) {
	f := newManagedCollaborationFixture(t)
	f.ctx = auth.WithAuthenticatedPrincipal(f.ctx, stageCredential(f.creator).Principal)
	handlerCtx := auth.WithAuthenticatedPrincipal(collaborationPasswordContext(util.MustParseUUID(f.handler), 1), stageCredential(f.handler).Principal)
	if err := f.svc.Queries.SeedIssueStatusEntries(f.ctx, util.MustParseUUID(f.workspace)); err != nil {
		t.Fatal(err)
	}
	f.fx.Cleanup(t, `DELETE FROM issue_status WHERE workspace_id=$1`, f.workspace)
	f.fx.Cleanup(t, `DELETE FROM issue_handoff WHERE issue_id=$1`, f.issue.Issue.ID)
	f.submitManaged(t, f.handler, "Original permitted input remains unchanged")
	f.resumeManaged(t)
	runID, _, claim := f.claimManaged(t)
	nonce := uuid.NewString()
	permit, err := f.svc.PermitRuntimeRun(f.daemonCtx, runID, protocol.CollaborationRunPermitRequest{Identity: claim.Identity,
		AttemptSeq: "1", AttemptNonce: nonce, Containment: protocol.CollaborationContainmentLinuxPIDNamespace})
	if err != nil {
		t.Fatal(err)
	}
	f.submitManaged(t, f.creator, "Waiting original input remains unchanged")
	ask := informationTestAsk(t, f.svc, f.issue.Issue.ID, handlerCtx, f.creator)
	ask.Note, ask.UnblockCondition, ask.EstimatedResolutionAt = "Original v1 handoff source bytes", nil, nil
	if _, err = f.svc.RequestInformation(handlerCtx, ask); err != nil {
		t.Fatal(err)
	}
	live, err := f.svc.RuntimeHandoff(context.Background(), runID, permit.BusinessToken, CollaborationHandoffQuery{})
	if err != nil {
		t.Fatal(err)
	}
	marker := false
	for _, missing := range live.Missing {
		marker = marker || missing == "information_block_metadata_unavailable_v1"
	}
	if !marker || live.ProtocolVersion != 1 || live.InformationRequest == nil {
		t.Fatal("new live v1 envelope hid missing structured metadata")
	}
	// Construct a valid pre-marker v1 storage specimen using the actual run,
	// source IDs and original note bytes, rather than rewriting an old archive.
	legacy := live
	legacy.Mode, legacy.SnapshotID = "snapshot", uuid.NewString()
	legacy.Missing = nil
	for _, missing := range live.Missing {
		if missing != "information_block_metadata_unavailable_v1" {
			legacy.Missing = append(legacy.Missing, missing)
		}
	}
	if legacy.Missing == nil {
		legacy.Missing = []string{}
	}
	legacy.Sources = append([]protocol.CollaborationHandoffSource{}, live.Sources...)
	if err = protocol.ValidateCollaborationHandoff(legacy); err != nil {
		t.Fatal(err)
	}
	stored := collaborationStoredHandoff{Identity: claim.Identity, MemoryVersion: "", Envelope: legacy}
	stored.Envelope.Sources = append([]protocol.CollaborationHandoffSource{}, legacy.Sources...)
	for i := range stored.Envelope.Sources {
		stored.Envelope.Sources[i].Content = ""
	}
	payload, err := json.Marshal(stored)
	if err != nil {
		t.Fatal(err)
	}
	originalEvent, err := f.svc.Queries.CreateCollaborationBusinessEvent(f.ctx, db.CreateCollaborationBusinessEventParams{IssueID: f.issue.Issue.ID,
		RunID: runID, EventID: "handoff:" + legacy.SnapshotID, Kind: "handoff_snapshot", Payload: payload})
	if err != nil {
		t.Fatal(err)
	}
	// JSONB canonicalizes the incoming JSON. Preserve the actual persisted
	// bytes before repair, after proving they still represent the supplied v1
	// specimen; later recovery must not rewrite that original database value.
	var supplied, persisted any
	if json.Unmarshal(payload, &supplied) != nil || json.Unmarshal(originalEvent.Payload, &persisted) != nil || !reflect.DeepEqual(supplied, persisted) {
		t.Fatal("stored v1 snapshot does not represent its original supplied specimen")
	}
	originalPayload := append([]byte(nil), originalEvent.Payload...)
	originalHash := sha256.Sum256(originalPayload)
	project := util.MustParseUUID(f.fx.Project(t, "Governed runtime information source"))
	f.fx.Cleanup(t, `DELETE FROM project_stage_policy WHERE workspace_id=$1`, f.workspace)
	f.fx.Exec(t, `UPDATE issue SET project_id=$2 WHERE id=$1`, f.issue.Issue.ID, project)
	if _, err = f.svc.ReviseStagePolicy(f.ctx, StagePolicyRevisionParams{WorkspaceID: util.MustParseUUID(f.workspace), ProjectID: project,
		RequestID: uuid.NewString(), Enabled: true, Reason: "Current runtime recovery applicability", Credential: completionRequestCredential(f.ctx)}); err != nil {
		t.Fatal(err)
	}
	beforeAI, beforeInputs := sourceRoleExecutionSnapshot(t, f.collaborationQueueFixture), sourceAuthorityExecutionState(t, f.collaborationQueueFixture)
	repair := informationTestRepair(t, f.svc, f.issue.Issue.ID, f.ctx, f.creator, "todo", false)
	result, err := f.svc.RepairInformation(f.ctx, repair)
	if err != nil || result.Code != "information_repaired" || beforeAI != sourceRoleExecutionSnapshot(t, f.collaborationQueueFixture) || beforeInputs != sourceAuthorityExecutionState(t, f.collaborationQueueFixture) {
		t.Fatalf("repair changed a live permitted run or original input: %+v %v", result, err)
	}
	if replay, err := f.svc.RepairInformation(f.ctx, repair); err != nil || !reflect.DeepEqual(replay, result) || beforeAI != sourceRoleExecutionSnapshot(t, f.collaborationQueueFixture) {
		t.Fatalf("repair replay created execution effects: %+v %v", replay, err)
	}
	restored, err := f.svc.RuntimeHandoffSnapshot(context.Background(), runID, permit.BusinessToken,
		protocol.CollaborationHandoffSnapshotRequest{RequestID: legacy.SnapshotID})
	if err != nil || !reflect.DeepEqual(restored, legacy) {
		t.Fatalf("old v1 snapshot was decorated or changed: %+v %v", restored, err)
	}
	event, err := f.svc.Queries.GetCollaborationRuntimeEvent(f.ctx, db.GetCollaborationRuntimeEventParams{RunID: runID, ProducerEventID: "handoff:" + legacy.SnapshotID})
	if err != nil || event.ID != originalEvent.ID || event.Seq != originalEvent.Seq || event.RunID != runID ||
		event.ProducerEventID != originalEvent.ProducerEventID || event.Kind != originalEvent.Kind || !bytes.Equal(event.Payload, originalPayload) || sha256.Sum256(event.Payload) != originalHash {
		t.Fatal("original stored snapshot/source identity bytes changed")
	}
	t.Logf("information_runtime_original_snapshot_id=%s input_json_sha256=%x persisted_jsonb_sha256=%x persisted_bytes=%d", legacy.SnapshotID, sha256.Sum256(payload), originalHash, len(originalPayload))
}
