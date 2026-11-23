package service

import (
	"context"
	"strconv"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// newSharedRuntimeSquadFixture places a squad (leader+member) and an unrelated
// direct-agent lane on one managed runtime, so both lanes share a namespace.
func newSharedRuntimeSquadFixture(t *testing.T) (*managedCollaborationFixture, pgtype.UUID, pgtype.UUID) {
	t.Helper()
	f, member := newManagedSquadFixture(t)
	session, err := f.svc.Queries.GetCollaborationSession(f.ctx, f.issue.Issue.ID)
	if err != nil {
		t.Fatal(err)
	}
	agent := f.fx.Agent(t, "Other lane agent "+uuid.NewString(), util.UUIDToString(f.runtime.ID), testutil.Cols{"owner_id": f.creator, "permission_mode": "private"})
	registered, err := f.svc.RegisterTarget(f.ctx, CollaborationRegisterTargetParams{CollaborationQueueRequest: f.request(), Kind: collaboration.TargetAgent,
		ResourceID: util.MustParseUUID(agent), ExpectedTargetSetVersion: session.TargetSetVersion,
		Roster: []collaboration.TargetResource{{Role: "agent", AgentID: agent, RuntimeID: util.UUIDToString(f.runtime.ID), ExecutionWorkspaceID: f.workspace, OwnerUserID: f.creator}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.svc.ConsentTargetResource(f.ctx, CollaborationConsentParams{CollaborationQueueRequest: f.request(), TargetID: registered.TargetID,
		AgentID: util.MustParseUUID(agent), ExpectedTargetEpoch: 1, ExpectedConfigVersion: 1,
		Identity: CollaborationResourceIdentity{Kind: "managed", InstallationID: f.installation.ID, BindingID: f.binding.ID, BindingEpoch: 1}}); err != nil {
		t.Fatal(err)
	}
	f.resumeManaged(t)
	return f, member, registered.TargetID
}

func (f *managedCollaborationFixture) submitManagedTo(t *testing.T, targetID pgtype.UUID, body string) {
	t.Helper()
	target := queueTarget(t, f.collaborationQueueFixture, targetID)
	if _, err := f.svc.SubmitCommand(f.ctx, CollaborationSubmitCommandParams{CollaborationQueueRequest: f.request(), TargetID: targetID, InputEventID: util.MustParseUUID(uuid.NewString()),
		ExpectedTargetEpoch: target.TargetEpoch, ExpectedConfigVersion: target.ConfigVersion, Content: body}); err != nil {
		t.Fatal(err)
	}
}

func (f *managedCollaborationFixture) managedClaimRequest() protocol.CollaborationRunClaimRequest {
	return protocol.CollaborationRunClaimRequest{ProtocolVersion: protocol.CollaborationRuntimeProtocolVersion, DeploymentID: auth.ManagedDeploymentID(),
		InstallationID: util.UUIDToString(f.installation.ID), RuntimeID: util.UUIDToString(f.runtime.ID), BindingID: util.UUIDToString(f.binding.ID),
		BindingEpoch: strconv.FormatInt(f.binding.BindingEpoch, 10), Containment: protocol.CollaborationContainmentLinuxPIDNamespace}
}

func (f *managedCollaborationFixture) listRuns(t *testing.T) []pgtype.UUID {
	t.Helper()
	list, err := f.svc.ListRuntimeRuns(f.daemonCtx, f.runtime.ID)
	if err != nil {
		t.Fatal(err)
	}
	ids := make([]pgtype.UUID, 0, len(list.RunIDs))
	for _, id := range list.RunIDs {
		ids = append(ids, util.MustParseUUID(id))
	}
	return ids
}

func (f *managedCollaborationFixture) runAndFinish(t *testing.T, run pgtype.UUID) {
	t.Helper()
	claim, err := f.svc.ClaimRuntimeRun(f.daemonCtx, run, f.managedClaimRequest())
	if err != nil {
		t.Fatal(err)
	}
	token, nonce := permitSquadRun(t, f, run, claim)
	finishSquadRun(t, f, run, claim, token, nonce, "success")
}

// startSquadWithPendingMember leaves the squad leader's first turn ended, its
// member dispatch prepared but unreserved, and other-lane input waiting.
func startSquadWithPendingMember(t *testing.T, f *managedCollaborationFixture, member, other pgtype.UUID) pgtype.UUID {
	t.Helper()
	f.submitManaged(t, f.creator, "squad instruction")
	root, _, claim := f.claimManaged(t)
	token, nonce := permitSquadRun(t, f, root, claim)
	receipt, err := f.svc.CreateRuntimeComment(context.Background(), root, token, CollaborationRuntimeCommentParams{CommentID: util.MustParseUUID(uuid.NewString()),
		Content: "[@member](mention://agent/" + util.UUIDToString(member) + ") implement"})
	if err != nil || len(receipt.RunIDs) != 1 {
		t.Fatalf("member dispatch: %+v %v", receipt, err)
	}
	f.submitManagedTo(t, other, "other lane instruction")
	finishSquadRun(t, f, root, claim, token, nonce, "success")
	return receipt.RunIDs[0]
}

func (f *managedCollaborationFixture) groupCount(t *testing.T, target pgtype.UUID) int {
	t.Helper()
	return f.fx.Count(t, `SELECT count(*) FROM collaboration_execution_group WHERE target_id=$1`, target)
}

// The leader's turn ended while its member waits for the shared namespace. A
// new root in another lane must not take the namespace first (R23/E01).
func TestCollaborationPendingDerivedRunDefersOtherLaneRoot(t *testing.T) {
	f, member, other := newSharedRuntimeSquadFixture(t)
	memberRun := startSquadWithPendingMember(t, f, member, other)
	if runs := f.listRuns(t); len(runs) != 1 || runs[0] != memberRun {
		t.Fatalf("member must be the only offered run: %v", runs)
	}
	if f.groupCount(t, other) != 0 {
		t.Fatal("other lane reserved the namespace ahead of the pending member")
	}
	f.runAndFinish(t, memberRun)
	runs := f.listRuns(t)
	if len(runs) != 1 {
		t.Fatalf("expected only the leader continuation: %v", runs)
	}
	continuation, err := f.svc.Queries.GetCollaborationRuntimeRun(f.ctx, runs[0])
	if err != nil || continuation.StepKind.String != "continuation" || f.groupCount(t, other) != 0 {
		t.Fatalf("continuation lost the namespace to another lane: %+v %v", continuation, err)
	}
	f.runAndFinish(t, continuation.ID)
	runs = f.listRuns(t)
	if len(runs) != 1 {
		t.Fatalf("other lane was not promoted after the squad settled: %v", runs)
	}
	stored, err := f.svc.Queries.GetCollaborationRuntimeRun(f.ctx, runs[0])
	if err != nil || stored.TargetID != other {
		t.Fatalf("expected the other lane root: %+v %v", stored, err)
	}
	f.runAndFinish(t, stored.ID)
}

// A reserved root and an older unreserved member can coexist, e.g. when the
// member is committed concurrently after the deferral read. The reserved run
// must be offered first, or the daemon retries runtime_busy forever.
func TestCollaborationReservedCandidateIsOfferedBeforeOlderDerivedRun(t *testing.T) {
	f, member, other := newSharedRuntimeSquadFixture(t)
	memberRun := startSquadWithPendingMember(t, f, member, other)
	// Fault injection: hide the member from promotion's deferral read only.
	f.fx.Exec(t, `UPDATE collaboration_target SET hold_reason='grant_required' WHERE id=$1`, f.targetID)
	runs := f.listRuns(t)
	if len(runs) != 1 || f.groupCount(t, other) != 1 {
		t.Fatalf("a held derived run must not defer promotion: %v", runs)
	}
	otherRoot := runs[0]
	f.fx.Exec(t, `UPDATE collaboration_target SET hold_reason=NULL WHERE id=$1`, f.targetID)
	runs = f.listRuns(t)
	if len(runs) != 1 || runs[0] != otherRoot {
		t.Fatalf("only the reserved tier may be offered ahead of the older unreserved member: %v", runs)
	}
	if _, err := f.svc.ClaimRuntimeRun(f.daemonCtx, memberRun, f.managedClaimRequest()); !collaborationErrorCode(err, "runtime_busy") {
		t.Fatalf("member claimed a namespace reserved by another run: %v", err)
	}
	f.runAndFinish(t, otherRoot)
	f.runAndFinish(t, memberRun)
}

// A revoked derived run is filtered from candidates and must not keep other
// lanes deferred.
func TestCollaborationRevokedDerivedRunDoesNotDeferPromotion(t *testing.T) {
	f, member, other := newSharedRuntimeSquadFixture(t)
	startSquadWithPendingMember(t, f, member, other)
	grants, err := f.svc.Queries.ListCollaborationTargetGrants(f.ctx, f.targetID)
	if err != nil {
		t.Fatal(err)
	}
	revoked := false
	for _, g := range grants {
		if g.AgentID == member && !g.RevokedAt.Valid {
			if _, err = f.svc.RevokeGrant(f.ctx, CollaborationRevokeGrantParams{CollaborationQueueRequest: f.request(), GrantID: g.ID, ExpectedGrantEpoch: g.Epoch}); err != nil {
				t.Fatal(err)
			}
			revoked = true
		}
	}
	if !revoked {
		t.Fatal("member grant missing")
	}
	runs := f.listRuns(t)
	if len(runs) != 1 || f.groupCount(t, other) != 1 {
		t.Fatalf("revoked member kept the other lane deferred: %v", runs)
	}
	f.runAndFinish(t, runs[0])
}

func nonlaunchProof(identity protocol.CollaborationRunIdentity, seq, nonce string) protocol.CollaborationRunProof {
	return protocol.CollaborationRunProof{ReceiptID: uuid.NewString(), Identity: identity, LastAuthorizedAttemptSeq: seq, SealedAttemptSeq: seq,
		AttemptNonce: nonce, State: "never_launched_proven", Containment: protocol.CollaborationContainmentLinuxPIDNamespace}
}

// A Stop settles a claimed, unpermitted run as nonlaunched (sealed 0) while the
// supervisor is between Claim and Permit. Its own proof must be accepted
// idempotently, or the daemon could never release that record.
func TestCollaborationStopSettledRunAcceptsSupervisorNonlaunchProof(t *testing.T) {
	for _, tc := range []struct{ name, seq, nonce string }{
		{"permit_requested", "1", uuid.NewString()},
		{"permit_unrequested", "0", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newManagedCollaborationFixture(t)
			f.submitManaged(t, f.creator, "instruction")
			f.resumeManaged(t)
			runID, _, claim := f.claimManaged(t)
			run, err := f.svc.Queries.GetCollaborationRuntimeRun(f.ctx, runID)
			if err != nil {
				t.Fatal(err)
			}
			target := queueTarget(t, f.collaborationQueueFixture, f.targetID)
			stopped, err := f.svc.StopQueue(f.ctx, f.stopParams(run, target.ControlVersion))
			if err != nil || !stopped.GroupSettled {
				t.Fatalf("stop before permit: %+v %v", stopped, err)
			}
			proof := nonlaunchProof(claim.Identity, tc.seq, tc.nonce)
			if _, err = f.svc.AcceptRuntimeProof(context.Background(), runID, claim.TerminationToken, proof); err != nil {
				t.Fatalf("supervisor nonlaunch proof after Stop: %v", err)
			}
			if _, err = f.svc.AcceptRuntimeProof(context.Background(), runID, claim.TerminationToken, proof); err != nil {
				t.Fatalf("proof replay: %v", err)
			}
			if f.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE id=$1 AND proof_state='never_launched_proven' AND sealed_attempt_seq=0 AND authorized_attempt_seq=0`, runID) != 1 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_termination WHERE run_id=$1 AND confirmed_at IS NOT NULL`, runID) != 1 {
				t.Fatal("proof rewrote the settled run or did not confirm termination")
			}
			permit := protocol.CollaborationRunPermitRequest{Identity: claim.Identity, AttemptSeq: "1", AttemptNonce: uuid.NewString(), Containment: protocol.CollaborationContainmentLinuxPIDNamespace}
			if _, err = f.svc.PermitRuntimeRun(f.daemonCtx, runID, permit); err == nil {
				t.Fatal("late permit authorized a settled run")
			}
		})
	}
}

// A lost or exhausted permit where the server never committed: the sealed
// nonlaunch proof settles the run and frees the reservation. Exit proofs for
// an attempt the server never authorized remain stale.
func TestCollaborationUnpermittedNonlaunchProofSettlesRun(t *testing.T) {
	f := newManagedCollaborationFixture(t)
	command := f.submitManaged(t, f.creator, "instruction")
	f.resumeManaged(t)
	runID, _, claim := f.claimManaged(t)
	nonce := uuid.NewString()
	if _, err := f.svc.AcceptRuntimeProof(context.Background(), runID, claim.TerminationToken, managedProof(claim.Identity, nonce)); !collaborationErrorCode(err, "stale_attempt_proof") {
		t.Fatalf("exit proof for an unauthorized attempt: %v", err)
	}
	if _, err := f.svc.AcceptRuntimeProof(context.Background(), runID, claim.TerminationToken, nonlaunchProof(claim.Identity, "1", nonce)); err != nil {
		t.Fatalf("unpermitted nonlaunch proof: %v", err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_runtime_reservation WHERE run_id=$1 AND released_at IS NOT NULL`, runID) != 1 {
		t.Fatal("nonlaunch proof did not release the reservation")
	}
	permit := protocol.CollaborationRunPermitRequest{Identity: claim.Identity, AttemptSeq: "1", AttemptNonce: nonce, Containment: protocol.CollaborationContainmentLinuxPIDNamespace}
	if _, err := f.svc.PermitRuntimeRun(f.daemonCtx, runID, permit); err == nil {
		t.Fatal("late permit replay authorized a sealed run")
	}
	// Existing nonlaunch semantics (unchanged here): the input returns to waiting.
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=$1 AND state='waiting'`, command.CommandID) != 1 {
		t.Fatal("never-launched input did not return to waiting")
	}
}
