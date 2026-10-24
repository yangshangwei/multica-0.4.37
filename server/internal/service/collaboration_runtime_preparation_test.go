package service

import (
	"bytes"
	"context"
	"encoding/json"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/storage"
	"github.com/multica-ai/multica/server/internal/util"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func claimPreparedCandidate(t *testing.T, f *managedCollaborationFixture, runID pgtype.UUID) (protocol.CollaborationRunClaimRequest, protocol.CollaborationRunClaimResponse) {
	t.Helper()
	request := preparationClaimRequest(f)
	request.PreparationVersion, request.BundleDigest = 1, strings.Repeat("a", 64)
	claim, err := f.svc.ClaimRuntimeRun(f.daemonCtx, runID, request)
	if err != nil || claim.Preparation == nil {
		t.Fatalf("negotiated preparation claim failed: %v", err)
	}
	if err = protocol.ValidateCollaborationPreparation(claim.Preparation, claim.Identity, claim.Inputs); err != nil {
		t.Fatalf("claim returned preparation unrelated to its actual prompt: %v", err)
	}
	return request, claim
}

func permitPreparedClaim(t *testing.T, f *managedCollaborationFixture, runID pgtype.UUID, claim protocol.CollaborationRunClaimResponse) (protocol.CollaborationRunPermitRequest, protocol.CollaborationRunPermitResponse) {
	t.Helper()
	p := protocol.CollaborationRunPermitRequest{Identity: claim.Identity, AttemptSeq: "1", AttemptNonce: uuid.NewString(), Containment: protocol.CollaborationContainmentLinuxPIDNamespace}
	permit, err := f.svc.PermitRuntimeRun(f.daemonCtx, runID, p)
	if err != nil {
		t.Fatal(err)
	}
	return p, permit
}

func preparationDeliveredFile(t *testing.T, f *managedCollaborationFixture) CollaborationAttachmentView {
	t.Helper()
	if f.svc.Storage == nil {
		t.Setenv("LOCAL_UPLOAD_DIR", t.TempDir())
		t.Setenv("LOCAL_UPLOAD_BASE_URL", "")
		f.svc.Storage = storage.NewLocalStorageFromEnv()
	}
	p := attachmentUploadParams(f.collaborationFixture)
	p.SourceWorkspaceID, p.IssueID = pgtype.UUID{}, f.issue.Issue.ID
	file, err := f.svc.UploadAttachmentDraft(f.ctx, p)
	if err != nil {
		t.Fatal(err)
	}
	f.fx.Cleanup(t, `DELETE FROM collaboration_attachment WHERE id=$1`, file.ID)
	f.fx.Cleanup(t, `DELETE FROM collaboration_attachment_reference WHERE attachment_id=$1`, file.ID)
	if _, err = f.svc.DeliverAttachments(f.ctx, CollaborationAttachmentDelivery{IssueID: f.issue.Issue.ID, DraftID: &p.DraftID,
		AttachmentIDs: []pgtype.UUID{file.ID}, RequestID: uuid.NewString()}); err != nil {
		t.Fatal(err)
	}
	return file
}

func preparationClaimRequest(f *managedCollaborationFixture) protocol.CollaborationRunClaimRequest {
	return protocol.CollaborationRunClaimRequest{ProtocolVersion: protocol.CollaborationRuntimeProtocolVersion,
		DeploymentID: auth.ManagedDeploymentID(), InstallationID: util.UUIDToString(f.installation.ID),
		RuntimeID: util.UUIDToString(f.runtime.ID), BindingID: util.UUIDToString(f.binding.ID),
		BindingEpoch: strconv.FormatInt(f.binding.BindingEpoch, 10), Containment: protocol.CollaborationContainmentLinuxPIDNamespace}
}

func preparationCandidate(t *testing.T, f *managedCollaborationFixture) pgtype.UUID {
	t.Helper()
	f.submitManaged(t, f.creator, "Use the explicitly delivered file")
	f.resumeManaged(t)
	list, err := f.svc.ListRuntimeRuns(f.daemonCtx, f.runtime.ID)
	if err != nil || len(list.RunIDs) != 1 {
		t.Fatalf("one admitted candidate: %+v %v", list, err)
	}
	return util.MustParseUUID(list.RunIDs[0])
}

func TestCollaborationRuntimePreparationLegacyRejectsFilesWithoutClaiming(t *testing.T) {
	f := preparedRuntimeFixture(t)
	preparationDeliveredFile(t, f)
	runID := preparationCandidate(t, f)
	before, err := f.svc.Queries.GetCollaborationRuntimeRun(f.ctx, runID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.svc.ClaimRuntimeRun(f.daemonCtx, runID, preparationClaimRequest(f))
	if !collaborationErrorCode(err, "runtime_preparation_unsupported") {
		t.Fatalf("legacy claim silently ignored an explicitly delivered file: %v", err)
	}
	after, err := f.svc.Queries.GetCollaborationRuntimeRun(f.ctx, runID)
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatalf("refused legacy claim mutated retained run state: %v", err)
	}
	for _, table := range []string{"collaboration_termination", "collaboration_attempt", "collaboration_run_event"} {
		if f.fx.Count(t, `SELECT count(*) FROM `+table+` WHERE run_id=$1`, runID) != 0 {
			t.Fatalf("refused legacy claim left %s", table)
		}
	}
}

func TestCollaborationRuntimePreparationLegacyNoFileWireStaysUnchanged(t *testing.T) {
	f := newManagedCollaborationFixture(t)
	f.submitManaged(t, f.creator, "legacy text input")
	f.resumeManaged(t)
	_, _, claim := f.claimManaged(t)
	raw, err := json.Marshal(claim)
	if err != nil {
		t.Fatal(err)
	}
	var wire map[string]json.RawMessage
	if err = json.Unmarshal(raw, &wire); err != nil {
		t.Fatal(err)
	}
	if len(wire) != 6 || wire["preparation"] != nil {
		t.Fatalf("strict installed claim decoder received unnegotiated fields: %s", raw)
	}
}

func TestCollaborationRuntimePreparationUnsupportedFilesDoNotReserve(t *testing.T) {
	f := newManagedCollaborationFixture(t)
	preparationDeliveredFile(t, f)
	f.submitManaged(t, f.creator, "requires attachment preparation")
	f.resumeManaged(t)
	list, err := f.svc.ListRuntimeRuns(f.daemonCtx, f.runtime.ID)
	if err != nil || len(list.RunIDs) != 0 {
		t.Fatalf("unsupported file preparation produced a candidate: %+v %v", list, err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE session_id=$1`, f.issue.SessionID) != 0 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_runtime_reservation WHERE runtime_id=$1 AND released_at IS NULL`, f.runtime.ID) != 0 {
		t.Fatal("unsupported preparation consumed the runtime namespace")
	}
	target := queueTarget(t, f.collaborationQueueFixture, f.targetID)
	if target.HoldReason.Valid || target.QueueState != "active" {
		t.Fatal("unsupported preparation invented a permanent hold or paused the channel")
	}
}

func TestCollaborationRuntimePreparationFreezesActualInputsAndDeliveredVersions(t *testing.T) {
	f := preparedRuntimeFixture(t)
	file := preparationDeliveredFile(t, f)
	target := queueTarget(t, f.collaborationQueueFixture, f.targetID)
	input, err := f.svc.SubmitCommand(f.ctx, CollaborationSubmitCommandParams{CollaborationQueueRequest: f.request(), TargetID: f.targetID,
		InputEventID: util.MustParseUUID(uuid.NewString()), ExpectedTargetEpoch: target.TargetEpoch, ExpectedConfigVersion: target.ConfigVersion,
		Content: "explicit immutable file input", AttachmentIDs: []pgtype.UUID{file.ID}})
	if err != nil {
		t.Fatal(err)
	}
	f.resumeManaged(t)
	list, err := f.svc.ListRuntimeRuns(f.daemonCtx, f.runtime.ID)
	if err != nil || len(list.RunIDs) != 1 {
		t.Fatalf("prepared input did not promote: %v", err)
	}
	runID := util.MustParseUUID(list.RunIDs[0])
	request, claim := claimPreparedCandidate(t, f, runID)
	if len(claim.Inputs) != 1 || claim.Inputs[0].CommandID != util.UUIDToString(input.CommandID) ||
		len(claim.Inputs[0].AttachmentIDs) != 1 || claim.Inputs[0].AttachmentIDs[0] != util.UUIDToString(file.ID) ||
		len(claim.Preparation.Attachments) != 1 || claim.Preparation.Attachments[0].Version != strconv.FormatInt(file.Version, 10) {
		t.Fatal("prepared manifest omitted the immutable command's file identity/version")
	}
	late := preparationDeliveredFile(t, f)
	replay, err := f.svc.ClaimRuntimeRun(f.daemonCtx, runID, request)
	if err != nil || replay.Identity != claim.Identity || replay.TerminationToken != claim.TerminationToken ||
		!reflect.DeepEqual(replay.Preparation, claim.Preparation) || !reflect.DeepEqual(replay.Inputs, claim.Inputs) {
		t.Fatalf("first-claim replay changed after later issue delivery: %v", err)
	}
	_, permit := permitPreparedClaim(t, f, runID, claim)
	manifest, err := f.svc.RuntimePreparation(context.Background(), runID, permit.BusinessToken)
	if err != nil || !reflect.DeepEqual(manifest, *claim.Preparation) {
		t.Fatalf("business capability did not return its frozen preparation: %v", err)
	}
	row, err := f.svc.RuntimeAttachment(context.Background(), runID, file.ID, file.Version, permit.BusinessToken)
	if err != nil {
		t.Fatal(err)
	}
	data, err := f.svc.ReadAttachmentBytes(context.Background(), row)
	if err != nil || !bytes.Equal(data, attachmentUploadParams(f.collaborationFixture).Data) {
		t.Fatalf("run-bound retrieval did not return actual encrypted-storage bytes: %v", err)
	}
	if _, err = f.svc.RuntimeAttachment(context.Background(), runID, late.ID, late.Version, permit.BusinessToken); !collaborationErrorCode(err, "attachment_not_found") {
		t.Fatalf("a later delivered file widened the existing run's manifest: %v", err)
	}
	raw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{row.StorageKey, row.ContentSha256.String, "encryption_key", "storage_key", "content_sha256"} {
		if secret != "" && strings.Contains(string(raw), secret) {
			t.Fatal("preparation exposed storage or content-hash material")
		}
	}
	if _, err = f.pool.Exec(f.ctx, `UPDATE collaboration_run SET preparation_snapshot=preparation_snapshot || '{"manifest_digest":"changed"}'::jsonb WHERE id=$1`, runID); err == nil {
		t.Fatal("database allowed a retained manifest to be changed")
	}
}

func TestCollaborationRuntimePreparationScopeAndRevocation(t *testing.T) {
	f := preparedRuntimeFixture(t)
	file := preparationDeliveredFile(t, f)
	runID := preparationCandidate(t, f)
	request, claim := claimPreparedCandidate(t, f, runID)
	p, permit := permitPreparedClaim(t, f, runID, claim)
	for name, token := range map[string]string{"termination": claim.TerminationToken, "human": "mul_" + strings.Repeat("a", 64),
		"task": "mat_" + strings.Repeat("a", 64), "other business": "mcb_" + strings.Repeat("b", 64), "anonymous": ""} {
		t.Run(name, func(t *testing.T) {
			if _, err := f.svc.RuntimePreparation(f.ctx, runID, token); err == nil {
				t.Fatal("unrelated credential read a run manifest")
			}
			if _, err := f.svc.RuntimeAttachment(f.ctx, runID, file.ID, file.Version, token); err == nil {
				t.Fatal("unrelated credential read run attachment bytes")
			}
		})
	}
	if _, err := f.svc.RuntimeAttachment(f.ctx, util.MustParseUUID(uuid.NewString()), file.ID, file.Version, permit.BusinessToken); err == nil {
		t.Fatal("business token escaped its run")
	}
	if _, err := f.svc.RuntimeAttachment(f.ctx, runID, file.ID, file.Version+1, permit.BusinessToken); !collaborationErrorCode(err, "attachment_not_found") {
		t.Fatalf("business token selected another file version: %v", err)
	}
	if _, err := f.svc.RuntimeAttachment(f.ctx, runID, util.MustParseUUID(uuid.NewString()), file.Version, permit.BusinessToken); !collaborationErrorCode(err, "attachment_not_found") {
		t.Fatalf("business token selected an unlisted file: %v", err)
	}
	if _, err := f.svc.RevokeGrant(f.ctx, CollaborationRevokeGrantParams{CollaborationQueueRequest: f.request(), GrantID: f.grant.ID, ExpectedGrantEpoch: f.grant.Epoch}); err != nil {
		t.Fatal(err)
	}
	for _, read := range []func() error{
		func() error { _, err := f.svc.RuntimePreparation(f.ctx, runID, permit.BusinessToken); return err },
		func() error {
			_, err := f.svc.RuntimeAttachment(f.ctx, runID, file.ID, file.Version, permit.BusinessToken)
			return err
		},
		func() error { _, err := f.svc.ClaimRuntimeRun(f.daemonCtx, runID, request); return err },
	} {
		if err := read(); err == nil {
			t.Fatal("revoked resource retained preparation/claim access")
		}
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_runtime_reservation WHERE run_id=$1 AND released_at IS NULL`, runID) != 1 {
		t.Fatal("revocation fabricated a termination fact")
	}
	if _, err := f.svc.AcceptRuntimeProof(context.Background(), runID, claim.TerminationToken, managedProof(claim.Identity, p.AttemptNonce)); err != nil {
		t.Fatalf("revocation stranded the original termination identity: %v", err)
	}
}

func TestCollaborationRuntimePreparationObservationsReplayWithoutOutcomeOrProof(t *testing.T) {
	f := preparedRuntimeFixture(t)
	preparationDeliveredFile(t, f)
	runID := preparationCandidate(t, f)
	_, claim := claimPreparedCandidate(t, f, runID)
	p, permit := permitPreparedClaim(t, f, runID, claim)
	event := protocol.CollaborationPreparationObservation{Identity: claim.Identity, EventID: uuid.NewString(), AttemptSeq: "1", AttemptNonce: p.AttemptNonce,
		ManifestDigest: claim.Preparation.ManifestDigest, Stage: "attachments", State: "prepared"}
	before, err := f.svc.Queries.GetCollaborationRuntimeRun(f.ctx, runID)
	if err != nil {
		t.Fatal(err)
	}
	fault := &queueFaultStarter{TxStarter: f.pool, lostCommit: true}
	f.svc.TxStarter = fault
	if _, err = f.svc.RecordRuntimePreparation(context.Background(), runID, permit.BusinessToken, event); err == nil || fault.begins.Load() != 1 {
		t.Fatalf("lost preparation response manufactured certainty or retried: %v", err)
	}
	f.svc.TxStarter = f.pool
	for range 2 {
		receipt, err := f.svc.RecordRuntimePreparation(context.Background(), runID, permit.BusinessToken, event)
		if err != nil || !receipt.Recorded || receipt.EventID != event.EventID || receipt.ManifestDigest != event.ManifestDigest || receipt.Identity != event.Identity {
			t.Fatalf("same event could not recover original observation: %v", err)
		}
	}
	for name, mutate := range map[string]func(*protocol.CollaborationPreparationObservation){
		"changed same event": func(p *protocol.CollaborationPreparationObservation) {
			p.State, p.Code = "failed", "attachment_unavailable"
		},
		"different attempt":  func(p *protocol.CollaborationPreparationObservation) { p.AttemptNonce = uuid.NewString() },
		"different manifest": func(p *protocol.CollaborationPreparationObservation) { p.ManifestDigest = strings.Repeat("b", 64) },
		"different issue":    func(p *protocol.CollaborationPreparationObservation) { p.Identity.IssueID = uuid.NewString() },
	} {
		t.Run(name, func(t *testing.T) {
			bad := event
			mutate(&bad)
			if _, err := f.svc.RecordRuntimePreparation(context.Background(), runID, permit.BusinessToken, bad); err == nil {
				t.Fatal("mismatched observation changed immutable preparation history")
			}
		})
	}
	after, err := f.svc.Queries.GetCollaborationRuntimeRun(f.ctx, runID)
	if err != nil || !reflect.DeepEqual(before, after) ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_run_event WHERE run_id=$1 AND kind='preparation'`, runID) != 1 {
		t.Fatalf("preparation observation changed run outcome/proof or duplicated its event: %v", err)
	}
	if _, err = f.svc.RecordRuntimePreparation(f.ctx, runID, claim.TerminationToken, event); err == nil {
		t.Fatal("termination credential acquired preparation write authority")
	}
	if _, err = f.svc.AcceptRuntimeProof(context.Background(), runID, claim.TerminationToken, managedProof(claim.Identity, p.AttemptNonce)); err != nil {
		t.Fatal(err)
	}
	if _, err = f.svc.RecordRuntimePreparation(context.Background(), runID, permit.BusinessToken, event); err == nil {
		t.Fatal("historical observation replay bypassed settled-run authorization")
	}
}

func TestCollaborationRuntimePreparationBundleChangeFencesClaimPermitAndBusiness(t *testing.T) {
	f := preparedRuntimeFixture(t)
	file := preparationDeliveredFile(t, f)
	runID := preparationCandidate(t, f)
	request, claim := claimPreparedCandidate(t, f, runID)
	p, permit := permitPreparedClaim(t, f, runID, claim)
	registration := preparationRegistrationRequest(f)
	registration.Capabilities.Provider.BundleDigest = strings.Repeat("b", 64)
	if _, err := f.svc.RegisterRuntimeCapabilities(f.daemonCtx, registration); err != nil {
		t.Fatal(err)
	}
	for name, call := range map[string]func() error{
		"claim":  func() error { _, err := f.svc.ClaimRuntimeRun(f.daemonCtx, runID, request); return err },
		"permit": func() error { _, err := f.svc.PermitRuntimeRun(f.daemonCtx, runID, p); return err },
		"manifest": func() error {
			_, err := f.svc.RuntimePreparation(context.Background(), runID, permit.BusinessToken)
			return err
		},
		"bytes": func() error {
			_, err := f.svc.RuntimeAttachment(context.Background(), runID, file.ID, file.Version, permit.BusinessToken)
			return err
		},
	} {
		t.Run(name, func(t *testing.T) {
			if err := call(); err == nil {
				t.Fatal("changed capability bundle retained previous admission")
			}
		})
	}
	row, err := f.svc.Queries.GetCollaborationRuntimeRun(f.ctx, runID)
	if err != nil || row.SettledAt.Valid || row.ProofState != "unknown" {
		t.Fatalf("capability change fabricated termination: %v", err)
	}
	var frozen protocol.CollaborationPreparation
	if err = json.Unmarshal(row.PreparationSnapshot, &frozen); err != nil || !reflect.DeepEqual(frozen, *claim.Preparation) {
		t.Fatalf("capability change rewrote retained preparation: %v", err)
	}
	if _, err = f.svc.AcceptRuntimeProof(context.Background(), runID, claim.TerminationToken, managedProof(claim.Identity, p.AttemptNonce)); err != nil {
		t.Fatalf("capability change stranded the original sealed proof: %v", err)
	}
}

func TestCollaborationRuntimePreparationCannotUpgradePreviouslyClaimedRun(t *testing.T) {
	f := preparedRuntimeFixture(t)
	f.submitManaged(t, f.creator, "legacy claim with no files")
	f.resumeManaged(t)
	runID, legacy, claim := f.claimManaged(t)
	if claim.Preparation != nil {
		t.Fatal("legacy claim received unnegotiated preparation")
	}
	upgraded := legacy
	upgraded.PreparationVersion, upgraded.BundleDigest = 1, strings.Repeat("a", 64)
	if _, err := f.svc.ClaimRuntimeRun(f.daemonCtx, runID, upgraded); !collaborationErrorCode(err, "runtime_preparation_unsupported") {
		t.Fatalf("later negotiation invented preparation for an old claimed prompt: %v", err)
	}
	if _, err := f.pool.Exec(f.ctx, `UPDATE collaboration_run SET preparation_snapshot='{}'::jsonb WHERE id=$1`, runID); err == nil {
		t.Fatal("database allowed late preparation injection after first claim")
	}
	replay, err := f.svc.ClaimRuntimeRun(f.daemonCtx, runID, legacy)
	if err != nil || replay.TerminationToken != claim.TerminationToken || replay.Identity != claim.Identity || !reflect.DeepEqual(replay.Inputs, claim.Inputs) {
		t.Fatalf("refused upgrade damaged old claim recovery: %v", err)
	}
}

func TestCollaborationRuntimePreparationUnsupportedFilesLeaveHealthyNamespaceAvailable(t *testing.T) {
	f := newManagedCollaborationFixture(t)
	preparationDeliveredFile(t, f)
	f.submitManaged(t, f.creator, "unsupported file-dependent issue")
	f.resumeManaged(t)
	healthy := preparationIndependentIssue(t, f)
	list, err := f.svc.ListRuntimeRuns(f.daemonCtx, f.runtime.ID)
	if err != nil || len(list.RunIDs) != 1 {
		t.Fatalf("unsupported file work blocked an independent legacy text issue: %+v %v", list, err)
	}
	claim, err := f.svc.ClaimRuntimeRun(f.daemonCtx, util.MustParseUUID(list.RunIDs[0]), preparationClaimRequest(f))
	if err != nil || claim.Identity.IssueID != util.UUIDToString(healthy.issue.Issue.ID) || claim.Preparation != nil {
		t.Fatalf("healthy text-only work could not consume the namespace: %v", err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE session_id=$1`, f.issue.SessionID) != 0 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE session_id=$1 AND state='waiting' AND group_id IS NULL`, f.issue.SessionID) != 1 {
		t.Fatal("skipped file work acquired a reservation or lost its waiting input")
	}
}

func preparationIndependentIssue(t *testing.T, f *managedCollaborationFixture) *managedCollaborationFixture {
	t.Helper()
	created, err := f.svc.Create(f.ctx, f.params(t))
	if err != nil {
		t.Fatal(err)
	}
	healthy := *f
	healthy.collaborationQueueFixture = &collaborationQueueFixture{collaborationFixture: f.collaborationFixture, issue: created}
	registered, err := healthy.svc.RegisterTarget(healthy.ctx, CollaborationRegisterTargetParams{CollaborationQueueRequest: healthy.request(),
		Kind: collaboration.TargetAgent, ResourceID: f.grant.AgentID, ExpectedTargetSetVersion: 1,
		Roster: []collaboration.TargetResource{{Role: "agent", AgentID: util.UUIDToString(f.grant.AgentID), RuntimeID: util.UUIDToString(f.runtime.ID),
			OwnerUserID: f.creator, ExecutionWorkspaceID: f.workspace}}})
	if err != nil {
		t.Fatal(err)
	}
	healthy.targetID = registered.TargetID
	healthy.grant = consentManagedControlTarget(t, &healthy, queueTarget(t, healthy.collaborationQueueFixture, healthy.targetID), f.grant.AgentID)
	for _, table := range []string{"collaboration_channel", "collaboration_target", "collaboration_batch", "collaboration_command", "collaboration_execution_group", "collaboration_run"} {
		f.fx.Cleanup(t, `DELETE FROM `+table+` WHERE session_id=$1`, created.SessionID)
	}
	f.fx.Cleanup(t, `DELETE FROM collaboration_grant WHERE issue_id=$1`, created.Issue.ID)
	for _, table := range []string{"collaboration_termination", "collaboration_attempt", "collaboration_business_capability", "collaboration_runtime_reservation", "collaboration_run_event"} {
		f.fx.Cleanup(t, `DELETE FROM `+table+` WHERE run_id IN (SELECT id FROM collaboration_run WHERE session_id=$1)`, created.SessionID)
	}
	f.fx.Cleanup(t, `DELETE FROM collaboration_group_input WHERE channel_id IN (SELECT id FROM collaboration_channel WHERE session_id=$1)`, created.SessionID)
	f.fx.Cleanup(t, `DELETE FROM collaboration_group_dependency WHERE target_id=$1`, healthy.targetID)
	healthy.submitManaged(t, healthy.creator, "healthy text-only issue")
	healthy.resumeManaged(t)
	return &healthy
}

func TestCollaborationRuntimePreparationObservationKeepsInstalledRecordsReadable(t *testing.T) {
	f := preparedRuntimeFixture(t)
	runID := preparationCandidate(t, f)
	_, claim := claimPreparedCandidate(t, f, runID)
	p, permit := permitPreparedClaim(t, f, runID, claim)
	if _, err := f.svc.RecordRuntimePreparation(context.Background(), runID, permit.BusinessToken, protocol.CollaborationPreparationObservation{
		Identity: claim.Identity, EventID: uuid.NewString(), AttemptSeq: "1", AttemptNonce: p.AttemptNonce,
		ManifestDigest: claim.Preparation.ManifestDigest, Stage: "workspace", State: "prepared"}); err != nil {
		t.Fatal(err)
	}
	page, err := f.svc.RunRecords(f.ctx, f.issue.Issue.ID, runID, CollaborationRunRecordsQuery{Limit: 50})
	if err != nil || page.ProtocolVersion != 1 || len(page.Items) != 1 || page.Items[0].Kind != "input" {
		t.Fatalf("typed preparation observation broke the installed record-item contract: %v", err)
	}
}

func TestCollaborationRuntimePreparationRejectsAnotherLiveRunAndIssueFile(t *testing.T) {
	first := preparedRuntimeFixture(t)
	firstFile := preparationDeliveredFile(t, first)
	firstRun := preparationCandidate(t, first)
	_, firstClaim := claimPreparedCandidate(t, first, firstRun)
	_, firstPermit := permitPreparedClaim(t, first, firstRun, firstClaim)
	second := preparedRuntimeFixture(t)
	secondFile := preparationDeliveredFile(t, second)
	secondRun := preparationCandidate(t, second)
	_, secondClaim := claimPreparedCandidate(t, second, secondRun)
	_, secondPermit := permitPreparedClaim(t, second, secondRun, secondClaim)
	if _, err := first.svc.RuntimePreparation(context.Background(), secondRun, firstPermit.BusinessToken); !collaborationErrorCode(err, "business_capability_required") {
		t.Fatalf("one live run read another live run's manifest: %v", err)
	}
	if _, err := first.svc.RuntimeAttachment(context.Background(), secondRun, secondFile.ID, secondFile.Version, firstPermit.BusinessToken); !collaborationErrorCode(err, "business_capability_required") {
		t.Fatalf("one live run read another run's file: %v", err)
	}
	if _, err := first.svc.RuntimeAttachment(context.Background(), firstRun, secondFile.ID, secondFile.Version, firstPermit.BusinessToken); !collaborationErrorCode(err, "attachment_not_found") {
		t.Fatalf("another issue's real file escaped the bound manifest: %v", err)
	}
	if _, err := first.svc.RuntimeAttachment(context.Background(), firstRun, firstFile.ID, firstFile.Version, firstPermit.BusinessToken); err != nil {
		t.Fatalf("cross-run denials damaged the first valid capability: %v", err)
	}
	if _, err := second.svc.RuntimeAttachment(context.Background(), secondRun, secondFile.ID, secondFile.Version, secondPermit.BusinessToken); err != nil {
		t.Fatalf("cross-run denials damaged the second valid capability: %v", err)
	}
}

func TestCollaborationRuntimePreparationFreezesAdmittedCommit(t *testing.T) {
	f := newManagedCollaborationFixture(t)
	registration := preparationRegistrationRequest(f)
	registration.Capabilities.Git = &protocol.CollaborationPreparationGit{ObjectFormat: "sha1", CommitOID: strings.Repeat("b", 40), BundleDigest: strings.Repeat("c", 64)}
	if _, err := f.svc.RegisterRuntimeCapabilities(f.daemonCtx, registration); err != nil {
		t.Fatal(err)
	}
	var err error
	f.runtime, err = f.svc.Queries.GetAgentRuntime(f.ctx, f.runtime.ID)
	if err != nil {
		t.Fatal(err)
	}
	refreshPreparationConsent(t, f)
	runID := preparationCandidate(t, f)
	if _, err := f.svc.ClaimRuntimeRun(f.daemonCtx, runID, preparationClaimRequest(f)); !collaborationErrorCode(err, "runtime_preparation_unsupported") {
		t.Fatalf("legacy client ignored admitted fixed Git source: %v", err)
	}
	_, claim := claimPreparedCandidate(t, f, runID)
	if !reflect.DeepEqual(claim.Preparation.Git, registration.Capabilities.Git) {
		t.Fatal("claim replaced the explicitly admitted bundle/commit with a live ref")
	}
}

func TestCollaborationRuntimePreparationLateDeliveryReturnsNeverClaimedInput(t *testing.T) {
	f := newManagedCollaborationFixture(t)
	runID := preparationCandidate(t, f)
	before, err := f.svc.Queries.GetCollaborationRuntimeRun(f.ctx, runID)
	if err != nil {
		t.Fatal(err)
	}
	targetBefore := queueTarget(t, f.collaborationQueueFixture, f.targetID)
	preparationDeliveredFile(t, f)
	if _, err = f.svc.ClaimRuntimeRun(f.daemonCtx, runID, preparationClaimRequest(f)); !collaborationErrorCode(err, "runtime_preparation_unsupported") {
		t.Fatalf("late delivery did not refuse an unsupported legacy claim: %v", err)
	}
	after, err := f.svc.Queries.GetCollaborationRuntimeRun(f.ctx, runID)
	if err != nil || !after.SettledAt.Valid || after.ProofState != "never_launched_proven" || after.AuthorizedAttemptSeq != 0 || after.BusinessOutcome.Valid ||
		after.Fence != before.Fence || after.LaunchNonce != before.LaunchNonce || after.ClaimGeneration != before.ClaimGeneration {
		t.Fatalf("late unsupported input stranded a never-claimed reservation or changed its launch identity: %v", err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_runtime_reservation WHERE run_id=$1 AND released_at IS NOT NULL`, runID) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=$1 AND state='waiting' AND group_id IS NULL AND seq=1`, before.CommandID) != 1 {
		t.Fatal("late preparation refusal did not return the original input and release only its known nonlaunch reservation")
	}
	targetAfter := queueTarget(t, f.collaborationQueueFixture, f.targetID)
	if targetAfter.ControlVersion != targetBefore.ControlVersion || targetAfter.QueueState != targetBefore.QueueState || targetAfter.HoldVersion != targetBefore.HoldVersion ||
		targetAfter.HoldReason != targetBefore.HoldReason || targetAfter.ActiveGroupID.Valid {
		t.Fatal("preparation refusal changed Stop/Resume/hold intent")
	}
	for _, table := range []string{"collaboration_termination", "collaboration_attempt", "collaboration_business_capability"} {
		if f.fx.Count(t, `SELECT count(*) FROM `+table+` WHERE run_id=$1`, runID) != 0 {
			t.Fatalf("never-claimed preparation refusal invented %s", table)
		}
	}
	list, err := f.svc.ListRuntimeRuns(f.daemonCtx, f.runtime.ID)
	if err != nil || len(list.RunIDs) != 0 || f.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE session_id=$1`, f.issue.SessionID) != 1 {
		t.Fatalf("unsupported returned input created another run: %+v %v", list, err)
	}
}

func TestCollaborationRuntimePreparationLateDeliveryPreservesIssuedClaim(t *testing.T) {
	for _, permitted := range []bool{false, true} {
		t.Run(strconv.FormatBool(permitted), func(t *testing.T) {
			f := newManagedCollaborationFixture(t)
			f.submitManaged(t, f.creator, "already claimed legacy text")
			f.resumeManaged(t)
			runID, request, claim := f.claimManaged(t)
			if permitted {
				permitPreparedClaim(t, f, runID, claim)
			}
			before, err := f.svc.Queries.GetCollaborationRuntimeRun(f.ctx, runID)
			if err != nil {
				t.Fatal(err)
			}
			term, err := f.svc.Queries.GetCollaborationTermination(f.ctx, runID)
			if err != nil {
				t.Fatal(err)
			}
			preparationDeliveredFile(t, f)
			if _, err = f.svc.ClaimRuntimeRun(f.daemonCtx, runID, request); !collaborationErrorCode(err, "runtime_preparation_unsupported") {
				t.Fatalf("legacy replay ignored a preparation requirement: %v", err)
			}
			after, err := f.svc.Queries.GetCollaborationRuntimeRun(f.ctx, runID)
			if err != nil || !reflect.DeepEqual(before, after) ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_runtime_reservation WHERE run_id=$1 AND released_at IS NULL`, runID) != 1 {
				t.Fatalf("late file refusal released a run with issued claim/permit evidence: %v", err)
			}
			retained, err := f.svc.Queries.GetCollaborationTermination(f.ctx, runID)
			if err != nil || !reflect.DeepEqual(term, retained) {
				t.Fatalf("late file refusal changed the original termination capability: %v", err)
			}
		})
	}
}

func TestCollaborationRuntimePreparationLateDeliverySettlementCommitBoundary(t *testing.T) {
	for _, committed := range []bool{false, true} {
		t.Run(strconv.FormatBool(committed), func(t *testing.T) {
			f := newManagedCollaborationFixture(t)
			runID := preparationCandidate(t, f)
			preparationDeliveredFile(t, f)
			fault := &queueFaultStarter{TxStarter: f.pool, eventFailure: !committed, lostCommit: committed}
			f.svc.TxStarter = fault
			if _, err := f.svc.ClaimRuntimeRun(f.daemonCtx, runID, preparationClaimRequest(f)); err == nil || collaborationErrorCode(err, "runtime_preparation_unsupported") || fault.begins.Load() != 1 {
				t.Fatalf("failed/unknown factual refusal was reported as committed or retried: %v", err)
			}
			f.svc.TxStarter = f.pool
			run, err := f.svc.Queries.GetCollaborationRuntimeRun(f.ctx, runID)
			if err != nil || run.SettledAt.Valid != committed {
				t.Fatalf("observed settlement disagrees with the actual commit: %v", err)
			}
			if !committed {
				if f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=$1 AND state='in_flight' AND group_id=$2`, run.CommandID, run.GroupID) != 1 ||
					f.fx.Count(t, `SELECT count(*) FROM collaboration_runtime_reservation WHERE run_id=$1 AND released_at IS NULL`, runID) != 1 {
					t.Fatal("failed required write left a partial input return/reservation release")
				}
				if _, err = f.svc.ClaimRuntimeRun(f.daemonCtx, runID, preparationClaimRequest(f)); !collaborationErrorCode(err, "runtime_preparation_unsupported") {
					t.Fatalf("original run could not reconcile a rolled-back preparation refusal: %v", err)
				}
			}
		})
	}
}
