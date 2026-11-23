package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func copyOutcomeReceipt(t *testing.T, outcome protocol.CollaborationRunOutcome) protocol.CollaborationRunOutcome {
	t.Helper()
	raw, err := json.Marshal(outcome)
	if err != nil {
		t.Fatal(err)
	}
	var replay protocol.CollaborationRunOutcome
	if err = json.Unmarshal(raw, &replay); err != nil {
		t.Fatal(err)
	}
	return replay
}

func TestCollaborationHostRecordedOutcomeACKSurvivesFinalProof(t *testing.T) {
	f, caps := newHostCollaborationFixture(t)
	submitHostCommand(t, f, f.handler, "recorded outcome reply may be lost")
	id, claim := claimHostRun(t, f, caps)
	launch := hostPermitRequest(claim, 1, "provider")
	permit, err := f.svc.PermitRuntimeRun(f.daemonCtx, id, launch)
	if err != nil {
		t.Fatal(err)
	}
	outcome := recordedOutcome(claim, permit, 2)
	outcome.ProtocolVersion, outcome.Containment = 3, launch.Containment
	if err = f.svc.RecordRuntimeOutcome(context.Background(), id, permit.BusinessToken, outcome); err != nil {
		t.Fatal(err)
	}
	// The client did not receive that outcome reply. Managed termination is
	// reported independently; its final receipt revokes business credentials.
	end := hostLaunchEnded(launch, true)
	if _, err = f.svc.AcceptRuntimeLaunchTermination(context.Background(), id, claim.TerminationToken, end); err != nil {
		t.Fatal(err)
	}
	if _, err = f.svc.AcceptRuntimeProof(context.Background(), id, claim.TerminationToken, hostFinalProof(t, claim, end)); err != nil {
		t.Fatal(err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_business_capability WHERE run_id=$1 AND revoked_at IS NOT NULL`, id) != 1 {
		t.Fatal("final proof did not revoke its original business capability")
	}
	before := controlStateSnapshot(t, f.collaborationQueueFixture)
	for range 2 {
		if err = f.svc.RecordRuntimeOutcome(context.Background(), id, permit.BusinessToken, copyOutcomeReceipt(t, outcome)); err != nil {
			t.Fatalf("original recorded outcome could not recover its ACK after settlement: %v", err)
		}
	}
	if before != controlStateSnapshot(t, f.collaborationQueueFixture) ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_run_event WHERE run_id=$1 AND kind='outcome'`, id) != 1 {
		t.Fatal("outcome ACK replay changed settled history or duplicated recording/usage")
	}
	changed := copyOutcomeReceipt(t, outcome)
	changed.Body = "new body after settlement"
	if err = f.svc.RecordRuntimeOutcome(context.Background(), id, permit.BusinessToken, changed); !collaborationErrorCode(err, "outcome_conflict") {
		t.Fatalf("a changed late outcome was not rejected as a receipt conflict: %v", err)
	}
	changed = copyOutcomeReceipt(t, outcome)
	changed.EventID = uuid.NewString()
	if err = f.svc.RecordRuntimeOutcome(context.Background(), id, permit.BusinessToken, changed); !collaborationErrorCode(err, "business_capability_required") {
		t.Fatalf("an unrecorded late outcome borrowed original receipt authority: %v", err)
	}
	if before != controlStateSnapshot(t, f.collaborationQueueFixture) {
		t.Fatal("rejected late result modified the settled run/group")
	}
}

func TestCollaborationHostRevokedOutcomeACKOnlyConfirmsOriginalPayload(t *testing.T) {
	f, caps := newHostCollaborationFixture(t)
	submitHostCommand(t, f, f.handler, "resource owner may revoke before outcome ACK")
	id, claim := claimHostRun(t, f, caps)
	launch := hostPermitRequest(claim, 1, "provider")
	permit, err := f.svc.PermitRuntimeRun(f.daemonCtx, id, launch)
	if err != nil {
		t.Fatal(err)
	}
	outcome := recordedOutcome(claim, permit, 2)
	outcome.ProtocolVersion, outcome.Containment = 3, launch.Containment
	if err = f.svc.RecordRuntimeOutcome(context.Background(), id, permit.BusinessToken, outcome); err != nil {
		t.Fatal(err)
	}
	if _, err = f.svc.RevokeGrant(f.ctx, CollaborationRevokeGrantParams{CollaborationQueueRequest: hostQueueRequest(f), GrantID: f.grant.ID, ExpectedGrantEpoch: f.grant.Epoch}); err != nil {
		t.Fatal(err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE id=$1 AND claim_generation>1 AND settled_at IS NULL`, id) != 1 {
		t.Fatal("revocation did not invalidate live admission while retaining execution")
	}
	before := controlStateSnapshot(t, f.collaborationQueueFixture)
	if err = f.svc.RecordRuntimeOutcome(context.Background(), id, permit.BusinessToken, copyOutcomeReceipt(t, outcome)); err != nil {
		t.Fatalf("revocation blocked confirmation of the already recorded original result: %v", err)
	}
	for name, test := range map[string]struct {
		mutate func(*protocol.CollaborationRunOutcome)
		code   string
	}{
		"body":                 {func(p *protocol.CollaborationRunOutcome) { p.Body = "replacement result" }, "outcome_conflict"},
		"usage":                {func(p *protocol.CollaborationRunOutcome) { n := "2"; p.Record.Usage.InputTokens = &n }, "outcome_conflict"},
		"new event":            {func(p *protocol.CollaborationRunOutcome) { p.EventID = uuid.NewString() }, "business_capability_required"},
		"another attempt":      {func(p *protocol.CollaborationRunOutcome) { p.AttemptSeq = "2" }, "business_capability_required"},
		"another nonce":        {func(p *protocol.CollaborationRunOutcome) { p.AttemptNonce = uuid.NewString() }, "stale_execution"},
		"another issue":        {func(p *protocol.CollaborationRunOutcome) { p.Identity.IssueID = uuid.NewString() }, "stale_execution"},
		"new claim generation": {func(p *protocol.CollaborationRunOutcome) { p.Identity.ClaimGeneration = "2" }, "stale_execution"},
		"legacy mode":          {func(p *protocol.CollaborationRunOutcome) { p.ProtocolVersion, p.Containment = 0, "" }, "execution_identity_mismatch"},
	} {
		t.Run(name, func(t *testing.T) {
			changed := copyOutcomeReceipt(t, outcome)
			test.mutate(&changed)
			if err := f.svc.RecordRuntimeOutcome(context.Background(), id, permit.BusinessToken, changed); !collaborationErrorCode(err, test.code) {
				t.Fatalf("altered original receipt returned wrong refusal: %v", err)
			}
		})
	}
	if err = f.svc.RecordRuntimeOutcome(context.Background(), id, claim.TerminationToken, outcome); !collaborationErrorCode(err, "business_capability_required") {
		t.Fatalf("termination identity borrowed business receipt authority: %v", err)
	}
	if before != controlStateSnapshot(t, f.collaborationQueueFixture) {
		t.Fatal("revoked ACK or a rejected payload changed execution/history/controls")
	}
}

func TestCollaborationHostUnrecordedOutcomeCannotResurrectSettledResult(t *testing.T) {
	f, caps := newHostCollaborationFixture(t)
	submitHostCommand(t, f, f.handler, "outcome upload never reached the server")
	id, claim := claimHostRun(t, f, caps)
	launch := hostPermitRequest(claim, 1, "provider")
	permit, err := f.svc.PermitRuntimeRun(f.daemonCtx, id, launch)
	if err != nil {
		t.Fatal(err)
	}
	end := hostLaunchEnded(launch, true)
	if _, err = f.svc.AcceptRuntimeLaunchTermination(context.Background(), id, claim.TerminationToken, end); err != nil {
		t.Fatal(err)
	}
	if _, err = f.svc.AcceptRuntimeProof(context.Background(), id, claim.TerminationToken, hostFinalProof(t, claim, end)); err != nil {
		t.Fatal(err)
	}
	before := controlStateSnapshot(t, f.collaborationQueueFixture)
	outcome := recordedOutcome(claim, permit, 2)
	outcome.ProtocolVersion, outcome.Containment = 3, launch.Containment
	if err = f.svc.RecordRuntimeOutcome(context.Background(), id, permit.BusinessToken, outcome); !collaborationErrorCode(err, "business_capability_required") {
		t.Fatalf("new result after final termination was accepted: %v", err)
	}
	if before != controlStateSnapshot(t, f.collaborationQueueFixture) ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE id=$1 AND settled_at IS NOT NULL AND recording_incomplete AND business_outcome IS NULL`, id) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_run_event WHERE run_id=$1 AND kind='outcome'`, id) != 0 {
		t.Fatal("late unrecorded outcome resurrected business success or history")
	}
}

func TestCollaborationLegacyRecordedOutcomeACKKeepsOriginalProtocol(t *testing.T) {
	f := newManagedCollaborationFixture(t)
	f.submitManaged(t, f.handler, "legacy outcome reply may also be lost")
	f.resumeManaged(t)
	id, _, claim := f.claimManaged(t)
	permit, err := f.svc.PermitRuntimeRun(f.daemonCtx, id, protocol.CollaborationRunPermitRequest{Identity: claim.Identity,
		Containment: protocol.CollaborationContainmentLinuxPIDNamespace, AttemptSeq: "1", AttemptNonce: uuid.NewString()})
	if err != nil {
		t.Fatal(err)
	}
	outcome := recordedOutcome(claim, permit, 1)
	if err = f.svc.RecordRuntimeOutcome(context.Background(), id, permit.BusinessToken, outcome); err != nil {
		t.Fatal(err)
	}
	if _, err = f.svc.AcceptRuntimeProof(context.Background(), id, claim.TerminationToken, managedProof(claim.Identity, permit.AttemptNonce)); err != nil {
		t.Fatal(err)
	}
	before := controlStateSnapshot(t, f.collaborationQueueFixture)
	if err = f.svc.RecordRuntimeOutcome(context.Background(), id, permit.BusinessToken, copyOutcomeReceipt(t, outcome)); err != nil {
		t.Fatalf("legacy original outcome receipt lost its ACK: %v", err)
	}
	if before != controlStateSnapshot(t, f.collaborationQueueFixture) {
		t.Fatal("legacy receipt replay changed namespace proof or recorded history")
	}
}
