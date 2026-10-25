package service

import (
	"context"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func hostQueueRequest(f *managedCollaborationFixture) CollaborationQueueRequest {
	p := f.request()
	p.ExecutionViewVersion = 6
	return p
}

func newHostCollaborationFixture(t *testing.T) (*managedCollaborationFixture, protocol.CollaborationRuntimeCapabilities) {
	t.Helper()
	f := newManagedCollaborationFixture(t, "local")
	p := hostCapabilityRegistration(f)
	if _, err := f.svc.RegisterRuntimeCapabilities(f.daemonCtx, p); err != nil {
		t.Fatal(err)
	}
	var err error
	f.runtime, err = f.svc.Queries.GetAgentRuntime(f.ctx, f.runtime.ID)
	if err != nil {
		t.Fatal(err)
	}
	renewHostConsent(t, f)
	gate, err := f.svc.Queries.GetCollaborationGate(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	f.fx.Cleanup(t, `UPDATE collaboration_execution_gate SET host_execution_enabled=$1 WHERE id=1`, gate.HostExecutionEnabled)
	f.fx.Exec(t, `UPDATE collaboration_execution_gate SET host_execution_enabled=true,epoch=epoch+1 WHERE id=1`)
	f.gateEpoch = gate.Epoch + 1
	return f, p.Capabilities
}

func hostCapabilityRegistration(f *managedCollaborationFixture) protocol.CollaborationRuntimeCapabilitiesRequest {
	p := preparationRegistrationRequest(f)
	p.ProtocolVersion = protocol.CollaborationHostRuntimeProtocolVersion
	p.Capabilities.Version, p.Capabilities.PreparationVersion = 2, 2
	p.Capabilities.Containment = protocol.CollaborationContainmentHostProcessSupervision
	p.Capabilities.Provider.AdapterID, p.Capabilities.Provider.BundleDigest = "ordinary_backend_v1", ""
	p.Capabilities.Provider.Resume, p.Capabilities.Provider.Fallback = "isolated", true
	p.Capabilities.Host = &protocol.CollaborationHostCapability{ProviderVersion: "test-cli-1", ProviderMode: "local",
		ConfigurationDigest: strings.Repeat("d", 64), SupervisionScopes: []string{"direct_process", "process_group"}}
	return p
}

func renewHostConsent(t *testing.T, f *managedCollaborationFixture) {
	t.Helper()
	old := queueTarget(t, f.collaborationQueueFixture, f.targetID)
	roster, err := decodeCollaborationRoster[collaboration.TargetResource](old.Roster)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.svc.ReconfigureTarget(f.ctx, CollaborationReconfigureTargetParams{CollaborationQueueRequest: hostQueueRequest(f), Control: targetControl(old), Roster: roster}); err != nil {
		t.Fatal(err)
	}
	target := queueTarget(t, f.collaborationQueueFixture, f.targetID)
	consented, err := f.svc.ConsentTargetResource(f.ctx, CollaborationConsentParams{CollaborationQueueRequest: hostQueueRequest(f),
		TargetID: f.targetID, AgentID: f.grant.AgentID, ExpectedTargetEpoch: target.TargetEpoch, ExpectedConfigVersion: target.ConfigVersion,
		Identity: CollaborationResourceIdentity{Kind: "managed", InstallationID: f.installation.ID, BindingID: f.binding.ID, BindingEpoch: f.binding.BindingEpoch}})
	if err != nil {
		t.Fatal(err)
	}
	f.grant, err = f.svc.Queries.GetCollaborationGrant(f.ctx, consented.GrantID)
	if err != nil {
		t.Fatal(err)
	}
}

func submitHostCommand(t *testing.T, f *managedCollaborationFixture, author, content string) CollaborationQueueReceipt {
	t.Helper()
	target := queueTarget(t, f.collaborationQueueFixture, f.targetID)
	receipt, err := f.svc.SubmitCommand(collaborationPasswordContext(util.MustParseUUID(author), 1), CollaborationSubmitCommandParams{
		CollaborationQueueRequest: hostQueueRequest(f), TargetID: f.targetID, InputEventID: util.MustParseUUID(uuid.NewString()),
		ExpectedTargetEpoch: target.TargetEpoch, ExpectedConfigVersion: target.ConfigVersion, Content: content})
	if err != nil {
		t.Fatal(err)
	}
	return receipt
}

func claimHostRun(t *testing.T, f *managedCollaborationFixture, caps protocol.CollaborationRuntimeCapabilities) (pgtype.UUID, protocol.CollaborationRunClaimResponse) {
	t.Helper()
	resume := f.issueControl(t)
	resume.ExecutionViewVersion = 6
	if _, err := f.svc.ResumeQueue(f.ctx, CollaborationResumeQueueParams{CollaborationStopQueueParams: resume, ExpectedExecutionEpoch: f.gateEpoch}); err != nil {
		t.Fatal(err)
	}
	listed, err := f.svc.ListRuntimeRuns(f.daemonCtx, f.runtime.ID)
	if err != nil || len(listed.RunIDs) != 1 {
		t.Fatalf("host candidate: %+v %v", listed, err)
	}
	id := util.MustParseUUID(listed.RunIDs[0])
	request := hostClaimRequest(t, f, caps)
	claim, err := f.svc.ClaimRuntimeRun(f.daemonCtx, id, request)
	if err != nil {
		t.Fatal(err)
	}
	if claim.HostContext == nil || claim.HostContext.AgentOwnerUserID != f.creator || claim.HostContext.RuntimeID != util.UUIDToString(f.runtime.ID) ||
		claim.ProtocolVersion != 3 || claim.Preparation == nil || claim.Preparation.Version != 2 {
		t.Fatalf("host claim did not retain owner/context/preparation: %+v", claim.HostContext)
	}
	return id, claim
}

func hostClaimRequest(t *testing.T, f *managedCollaborationFixture, caps protocol.CollaborationRuntimeCapabilities) protocol.CollaborationRunClaimRequest {
	t.Helper()
	digest, err := protocol.CollaborationRuntimeCapabilityDigest(caps)
	if err != nil {
		t.Fatal(err)
	}
	request := preparationClaimRequest(f)
	request.ProtocolVersion, request.Containment = protocol.CollaborationHostRuntimeProtocolVersion, protocol.CollaborationContainmentHostProcessSupervision
	request.PreparationVersion, request.BundleDigest, request.CapabilityDigest = 2, "", digest
	return request
}

func hostPermitRequest(claim protocol.CollaborationRunClaimResponse, seq int, kind string) protocol.CollaborationRunPermitRequest {
	return protocol.CollaborationRunPermitRequest{ProtocolVersion: 3, Identity: claim.Identity, Containment: protocol.CollaborationContainmentHostProcessSupervision,
		AttemptSeq: strconv.Itoa(seq), AttemptNonce: uuid.NewString(), LaunchDescriptor: &protocol.CollaborationLaunchDescriptor{Kind: kind,
			CapabilityDigest: claim.Preparation.CapabilityDigest, ConfigurationDigest: claim.Preparation.Host.ConfigurationDigest}}
}

func hostLaunchEnded(p protocol.CollaborationRunPermitRequest, started bool) protocol.CollaborationLaunchTermination {
	observation := protocol.CollaborationLaunchTermination{ProtocolVersion: 3, Identity: p.Identity, Containment: p.Containment, AttemptSeq: p.AttemptSeq,
		AttemptNonce: p.AttemptNonce, LaunchID: p.AttemptNonce, EventID: uuid.NewString(), Kind: "not_started",
		Cleanup: protocol.CollaborationCleanupObservation{State: "unverified", Scope: "direct_process"}}
	if started {
		exit := 0
		observation.Kind, observation.ExitCode = "managed_launch_ended", &exit
	}
	return observation
}

func hostFinalProof(t *testing.T, claim protocol.CollaborationRunClaimResponse, launches ...protocol.CollaborationLaunchTermination) protocol.CollaborationRunProof {
	t.Helper()
	proof := protocol.CollaborationRunProof{ProtocolVersion: 3, Identity: claim.Identity, ReceiptID: uuid.NewString(),
		Containment: protocol.CollaborationContainmentHostProcessSupervision, LastAuthorizedAttemptSeq: strconv.Itoa(len(launches)),
		SealedAttemptSeq: strconv.Itoa(len(launches)), State: "host_exit_observed", Launches: launches}
	if len(launches) == 0 {
		proof.State = "never_launched_proven"
	} else {
		proof.AttemptNonce = launches[len(launches)-1].AttemptNonce
	}
	var err error
	proof.Cleanup, err = protocol.SummarizeCollaborationCleanup(launches)
	if err != nil {
		t.Fatal(err)
	}
	return proof
}

func TestCollaborationHostLaunchesNeedIndependentReceiptsAndFinalSeal(t *testing.T) {
	f, caps := newHostCollaborationFixture(t)
	submitHostCommand(t, f, f.creator, "first host author")
	submitHostCommand(t, f, f.handler, "second host author")
	id, claim := claimHostRun(t, f, caps)
	if len(claim.Inputs) != 2 || claim.Inputs[0].AuthorUserID == claim.Inputs[1].AuthorUserID {
		t.Fatal("host batch lost original authors")
	}
	first := hostPermitRequest(claim, 1, "preparation")
	permit, err := f.svc.PermitRuntimeRun(f.daemonCtx, id, first)
	if err != nil || permit.ProtocolVersion != 3 {
		t.Fatalf("first host permit: %v", err)
	}
	replay, err := f.svc.PermitRuntimeRun(f.daemonCtx, id, first)
	if err != nil || replay != permit {
		t.Fatalf("permit replay changed original credential: %v", err)
	}
	second := hostPermitRequest(claim, 2, "provider")
	if _, err = f.svc.PermitRuntimeRun(f.daemonCtx, id, second); !collaborationErrorCode(err, "prior_launch_unconfirmed") {
		t.Fatalf("fallback passed unconfirmed previous launch: %v", err)
	}
	endFirst := hostLaunchEnded(first, false)
	accepted, err := f.svc.AcceptRuntimeLaunchTermination(context.Background(), id, claim.TerminationToken, endFirst)
	if err != nil || !accepted.Authorized {
		t.Fatalf("first launch receipt: %v", err)
	}
	got, err := f.svc.AcceptRuntimeLaunchTermination(context.Background(), id, claim.TerminationToken, endFirst)
	if err != nil || got != accepted {
		t.Fatalf("launch receipt replay changed identity: %v", err)
	}
	changed := endFirst
	changed.EventID = uuid.NewString()
	if _, err = f.svc.AcceptRuntimeLaunchTermination(context.Background(), id, claim.TerminationToken, changed); !collaborationErrorCode(err, "launch_termination_conflict") {
		t.Fatalf("changed receipt overwrote old observation: %v", err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE id=$1 AND settled_at IS NULL AND authorized_attempt_seq=1 AND sealed_attempt_seq IS NULL`, id) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_runtime_reservation WHERE run_id=$1 AND released_at IS NULL`, id) != 1 {
		t.Fatal("per-launch observation sealed or released the run")
	}
	if _, err = f.svc.PermitRuntimeRun(f.daemonCtx, id, second); err != nil {
		t.Fatalf("observed preparation prevented next provider launch: %v", err)
	}
	observation := protocol.CollaborationPreparationObservation{ProtocolVersion: 3, Containment: second.Containment, Identity: claim.Identity,
		EventID: uuid.NewString(), AttemptSeq: "2", AttemptNonce: second.AttemptNonce, ManifestDigest: claim.Preparation.ManifestDigest, Stage: "workspace", State: "prepared"}
	secondPermit, err := f.svc.PermitRuntimeRun(f.daemonCtx, id, second)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.svc.RecordRuntimePreparation(context.Background(), id, secondPermit.BusinessToken, observation); err != nil {
		t.Fatalf("host helper preparation retained a hard-coded first attempt: %v", err)
	}
	endSecond := hostLaunchEnded(second, true)
	if _, err = f.svc.AcceptRuntimeLaunchTermination(context.Background(), id, claim.TerminationToken, endSecond); err != nil {
		t.Fatal(err)
	}
	partial := hostFinalProof(t, claim, endFirst)
	if _, err = f.svc.AcceptRuntimeProof(context.Background(), id, claim.TerminationToken, partial); err == nil {
		t.Fatal("final seal skipped an issued permit")
	}
	proof := hostFinalProof(t, claim, endFirst, endSecond)
	receipt, err := f.svc.AcceptRuntimeProof(context.Background(), id, claim.TerminationToken, proof)
	if err != nil || receipt.ProtocolVersion != 3 || !receipt.Accepted {
		t.Fatalf("truthful final host observation rejected: %v", err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE id=$1 AND proof_state='host_exit_observed' AND settled_at IS NOT NULL AND business_outcome IS NULL AND recording_incomplete`, id) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_runtime_reservation WHERE run_id=$1 AND released_at IS NOT NULL`, id) != 1 {
		t.Fatal("host termination fabricated success or retained completed capacity")
	}
	again, err := f.svc.AcceptRuntimeProof(context.Background(), id, claim.TerminationToken, proof)
	if err != nil || again != receipt {
		t.Fatalf("final receipt replay was not exact: %v", err)
	}
}

func TestCollaborationHostUnissuedPermitTombstoneStopsLateLaunch(t *testing.T) {
	f, caps := newHostCollaborationFixture(t)
	submitHostCommand(t, f, f.creator, "request may never reach server")
	id, claim := claimHostRun(t, f, caps)
	pending := hostPermitRequest(claim, 1, "provider")
	observation := hostLaunchEnded(pending, false)
	receipt, err := f.svc.AcceptRuntimeLaunchTermination(context.Background(), id, claim.TerminationToken, observation)
	if err != nil || receipt.Authorized {
		t.Fatalf("missing permit was converted to authorization: %+v %v", receipt, err)
	}
	if _, err = f.svc.PermitRuntimeRun(f.daemonCtx, id, pending); err == nil {
		t.Fatal("late original permit crossed a durable nonlaunch tombstone")
	}
	pending.AttemptNonce = uuid.NewString()
	if _, err = f.svc.PermitRuntimeRun(f.daemonCtx, id, pending); err == nil {
		t.Fatal("a new nonce replaced the unissued tombstone")
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE id=$1 AND authorized_attempt_seq=0 AND settled_at IS NULL`, id) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_business_capability WHERE run_id=$1`, id) != 0 {
		t.Fatal("tombstone minted authority or settled execution")
	}
	if _, err = f.svc.AcceptRuntimeProof(context.Background(), id, claim.TerminationToken, hostFinalProof(t, claim)); err != nil {
		t.Fatalf("zero-permit tombstone failed to converge: %v", err)
	}
}

func TestCollaborationHostStopRejectsFallbackAndAcceptsOriginalCleanup(t *testing.T) {
	f, caps := newHostCollaborationFixture(t)
	submitHostCommand(t, f, f.creator, "stop before provider fallback")
	id, claim := claimHostRun(t, f, caps)
	first := hostPermitRequest(claim, 1, "provider")
	permit, err := f.svc.PermitRuntimeRun(f.daemonCtx, id, first)
	if err != nil {
		t.Fatal(err)
	}
	end := hostLaunchEnded(first, true)
	if _, err = f.svc.AcceptRuntimeLaunchTermination(context.Background(), id, claim.TerminationToken, end); err != nil {
		t.Fatal(err)
	}
	stop := f.issueControl(t)
	stop.ExecutionViewVersion = 6
	if _, err = f.svc.StopQueue(f.ctx, stop); err != nil {
		t.Fatal(err)
	}
	if _, err = f.svc.PermitRuntimeRun(f.daemonCtx, id, hostPermitRequest(claim, 2, "provider")); err == nil {
		t.Fatal("stopped execution was allowed a fallback")
	}
	if err = f.svc.RecordRuntimeOutcome(context.Background(), id, permit.BusinessToken, protocol.CollaborationRunOutcome{ProtocolVersion: 3,
		Containment: first.Containment, Identity: claim.Identity, AttemptSeq: first.AttemptSeq, AttemptNonce: first.AttemptNonce,
		EventID: uuid.NewString(), Outcome: "success", Body: "late business output"}); err == nil {
		t.Fatal("stopped business capability published late output")
	}
	resume := f.issueControl(t)
	resume.ExecutionViewVersion = 6
	if _, err = f.svc.ResumeQueue(f.ctx, CollaborationResumeQueueParams{CollaborationStopQueueParams: resume, ExpectedExecutionEpoch: f.gateEpoch}); !collaborationErrorCode(err, "partially_resumed") && !collaborationErrorCode(err, "stop_unconfirmed") {
		// Issue Resume can return a successful partial receipt; inspect its
		// retained barrier below rather than assuming a single HTTP status.
		if err != nil {
			t.Fatal(err)
		}
	}
	before := queueTarget(t, f.collaborationQueueFixture, f.targetID)
	if _, err = f.svc.AcceptRuntimeProof(context.Background(), id, claim.TerminationToken, hostFinalProof(t, claim, end)); err != nil {
		t.Fatalf("Stop stranded original host cleanup: %v", err)
	}
	after := queueTarget(t, f.collaborationQueueFixture, f.targetID)
	if before.QueueState != "paused" || after.QueueState != "paused" || after.ControlVersion != before.ControlVersion ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_attempt WHERE run_id=$1`, id) != 1 {
		t.Fatal("proof arrival resumed stopped input or created a fallback")
	}
	oldMode := managedProof(claim.Identity, first.AttemptNonce)
	if _, err = f.svc.AcceptRuntimeProof(context.Background(), id, claim.TerminationToken, oldMode); err == nil {
		t.Fatal("legacy namespace proof settled a host run")
	}
}

func TestCollaborationHostGrantWithdrawalKeepsOriginalTerminalAuthority(t *testing.T) {
	f, caps := newHostCollaborationFixture(t)
	submitHostCommand(t, f, f.handler, "peer uses owner's authorized agent")
	id, claim := claimHostRun(t, f, caps)
	first := hostPermitRequest(claim, 1, "provider")
	if _, err := f.svc.PermitRuntimeRun(f.daemonCtx, id, first); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.RevokeGrant(f.ctx, CollaborationRevokeGrantParams{CollaborationQueueRequest: hostQueueRequest(f), GrantID: f.grant.ID, ExpectedGrantEpoch: f.grant.Epoch}); err != nil {
		t.Fatal(err)
	}
	end := hostLaunchEnded(first, true)
	if _, err := f.svc.AcceptRuntimeLaunchTermination(context.Background(), id, claim.TerminationToken, end); err != nil {
		t.Fatalf("revocation blocked narrow launch receipt: %v", err)
	}
	if _, err := f.svc.PermitRuntimeRun(f.daemonCtx, id, hostPermitRequest(claim, 2, "provider")); err == nil {
		t.Fatal("revoked grant allowed fallback")
	}
	if _, err := f.svc.AcceptRuntimeProof(context.Background(), id, claim.TerminationToken, hostFinalProof(t, claim, end)); err != nil {
		t.Fatalf("revocation stranded minimal final receipt: %v", err)
	}
	target := queueTarget(t, f.collaborationQueueFixture, f.targetID)
	if !target.HoldReason.Valid || target.HoldReason.String != "grant_required" {
		t.Fatal("termination restored revoked authorization")
	}
	current, err := f.svc.Queries.GetCollaborationRuntimeRun(f.ctx, id)
	if err != nil || current.ResourceOwnerUserID != f.runtime.OwnerID || claim.Inputs[0].AuthorUserID != f.handler {
		t.Fatal("cleanup replaced actual author/resource provenance")
	}
}

func TestCollaborationHostSQLAndGoTerminalPredicatesAgree(t *testing.T) {
	f := newCollaborationFixture(t)
	for _, mode := range []string{"", protocol.CollaborationContainmentLinuxPIDNamespace, protocol.CollaborationContainmentHostProcessSupervision, "future"} {
		for _, proof := range []string{"unknown", "exited_proven", "host_exit_observed", "never_launched_proven"} {
			for _, progress := range []string{"running", "settled"} {
				run := db.CollaborationRun{Containment: pgtype.Text{String: mode, Valid: mode != ""}, ProofState: proof, ProgressState: progress,
					AuthorizedAttemptSeq: 1, SealedAttemptSeq: pgtype.Int8{Int64: 1, Valid: true}, SettledAt: pgtype.Timestamptz{Time: time.Now(), Valid: true}}
				var sqlEnded bool
				err := f.pool.QueryRow(t.Context(), `SELECT collaboration_managed_execution_ended($1,$2,$3,$4,$5,$6,$7,$8)`,
					run.Containment, run.ProofState, run.ProgressState, run.SettledAt, run.AuthorizedAttemptSeq, run.SealedAttemptSeq, run.StartedAt, run.BusinessOutcome).Scan(&sqlEnded)
				if err != nil || !reflect.DeepEqual(sqlEnded, collaborationRunProvenEnded(run)) {
					t.Fatalf("SQL/Go terminal mismatch for %s/%s/%s: %t %v", mode, proof, progress, sqlEnded, err)
				}
			}
		}
	}
}

func TestCollaborationHostSquadCapacityOneKeepsEachRunIdentity(t *testing.T) {
	f, caps := newHostCollaborationFixture(t)
	leader := util.UUIDToString(f.grant.AgentID)
	member := f.fx.Agent(t, "Host squad member "+uuid.NewString(), util.UUIDToString(f.runtime.ID), testutil.Cols{"owner_id": f.creator, "permission_mode": "private"})
	squad := f.fx.Squad(t, "Host collaboration squad", leader)
	f.fx.SquadMember(t, squad, "agent", member)
	session, err := f.svc.Queries.GetCollaborationSession(f.ctx, f.issue.Issue.ID)
	if err != nil {
		t.Fatal(err)
	}
	registered, err := f.svc.RegisterTarget(f.ctx, CollaborationRegisterTargetParams{CollaborationQueueRequest: hostQueueRequest(f),
		Kind: collaboration.TargetSquad, ResourceID: util.MustParseUUID(squad), ExpectedTargetSetVersion: session.TargetSetVersion,
		Roster: []collaboration.TargetResource{
			{Role: "leader", AgentID: leader, RuntimeID: util.UUIDToString(f.runtime.ID), ExecutionWorkspaceID: f.workspace, OwnerUserID: f.creator},
			{Role: "member", AgentID: member, RuntimeID: util.UUIDToString(f.runtime.ID), ExecutionWorkspaceID: f.workspace, OwnerUserID: f.creator},
		}})
	if err != nil {
		t.Fatal(err)
	}
	f.targetID = registered.TargetID
	for _, agentID := range []string{leader, member} {
		_, err = f.svc.ConsentTargetResource(f.ctx, CollaborationConsentParams{CollaborationQueueRequest: hostQueueRequest(f), TargetID: f.targetID,
			AgentID: util.MustParseUUID(agentID), ExpectedTargetEpoch: 1, ExpectedConfigVersion: 1,
			Identity: CollaborationResourceIdentity{Kind: "managed", InstallationID: f.installation.ID, BindingID: f.binding.ID, BindingEpoch: f.binding.BindingEpoch}})
		if err != nil {
			t.Fatal(err)
		}
	}
	f.fx.Cleanup(t, `DELETE FROM collaboration_delegation WHERE group_id IN (SELECT id FROM collaboration_execution_group WHERE session_id=$1)`, f.issue.SessionID)
	submitHostCommand(t, f, f.handler, "collaborator directs the squad")
	root, rootClaim := claimHostRun(t, f, caps)
	rootRequest := hostPermitRequest(rootClaim, 1, "provider")
	rootPermit, err := f.svc.PermitRuntimeRun(f.daemonCtx, root, rootRequest)
	if err != nil {
		t.Fatal(err)
	}
	comment := CollaborationRuntimeCommentParams{CommentID: util.MustParseUUID(uuid.NewString()), Content: "[@member](mention://agent/" + member + ") do the assigned work"}
	dispatch, err := f.svc.CreateRuntimeComment(context.Background(), root, rootPermit.BusinessToken, comment)
	if err != nil || len(dispatch.RunIDs) != 1 {
		t.Fatalf("host leader did not create one member: %+v %v", dispatch, err)
	}
	memberRun := dispatch.RunIDs[0]
	request := hostClaimRequest(t, f, caps)
	if _, err = f.svc.ClaimRuntimeRun(f.daemonCtx, memberRun, request); !collaborationErrorCode(err, "runtime_busy") {
		t.Fatalf("member borrowed leader's occupied slot: %v", err)
	}
	finish := func(id pgtype.UUID, claim protocol.CollaborationRunClaimResponse, launch protocol.CollaborationRunPermitRequest, permit protocol.CollaborationRunPermitResponse) {
		t.Helper()
		if err := f.svc.RecordRuntimeOutcome(context.Background(), id, permit.BusinessToken, protocol.CollaborationRunOutcome{ProtocolVersion: 3, Containment: launch.Containment,
			Identity: claim.Identity, AttemptSeq: launch.AttemptSeq, AttemptNonce: launch.AttemptNonce, EventID: uuid.NewString(), Outcome: "success", Body: "turn complete"}); err != nil {
			t.Fatal(err)
		}
		end := hostLaunchEnded(launch, true)
		if _, err := f.svc.AcceptRuntimeLaunchTermination(context.Background(), id, claim.TerminationToken, end); err != nil {
			t.Fatal(err)
		}
		if _, err := f.svc.AcceptRuntimeProof(context.Background(), id, claim.TerminationToken, hostFinalProof(t, claim, end)); err != nil {
			t.Fatal(err)
		}
	}
	finish(root, rootClaim, rootRequest, rootPermit)
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE id=$1 AND settled_at IS NULL`, memberRun) != 1 {
		t.Fatal("leader terminal observation substituted for the member")
	}
	memberClaim, err := f.svc.ClaimRuntimeRun(f.daemonCtx, memberRun, request)
	if err != nil {
		t.Fatal(err)
	}
	if memberClaim.HostContext == nil || memberClaim.HostContext.Role != "member" || memberClaim.HostContext.ParentRunID == nil || *memberClaim.HostContext.ParentRunID != util.UUIDToString(root) ||
		memberClaim.Inputs[0].AuthorUserID != f.handler || memberClaim.Identity.LaunchNonce == rootClaim.Identity.LaunchNonce {
		t.Fatal("member lost root author/parent or reused launch identity")
	}
	memberRequest := hostPermitRequest(memberClaim, 1, "provider")
	memberPermit, err := f.svc.PermitRuntimeRun(f.daemonCtx, memberRun, memberRequest)
	if err != nil {
		t.Fatal(err)
	}
	finish(memberRun, memberClaim, memberRequest, memberPermit)
	listed, err := f.svc.ListRuntimeRuns(f.daemonCtx, f.runtime.ID)
	if err != nil || len(listed.RunIDs) != 1 {
		t.Fatalf("leader continuation was not scheduled: %+v %v", listed, err)
	}
	continuation := util.MustParseUUID(listed.RunIDs[0])
	continued, err := f.svc.ClaimRuntimeRun(f.daemonCtx, continuation, request)
	if err != nil {
		t.Fatal(err)
	}
	if continued.HostContext == nil || continued.HostContext.Role != "continuation" || continued.Inputs[0].AuthorUserID != f.handler || continuation == root || continuation == memberRun {
		t.Fatal("continuation did not retain independent identity/original author")
	}
	continuedRequest := hostPermitRequest(continued, 1, "provider")
	continuedPermit, err := f.svc.PermitRuntimeRun(f.daemonCtx, continuation, continuedRequest)
	if err != nil {
		t.Fatal(err)
	}
	finish(continuation, continued, continuedRequest, continuedPermit)
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE session_id=$1 AND proof_state='host_exit_observed' AND settled_at IS NOT NULL`, f.issue.SessionID) != 3 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_execution_group WHERE session_id=$1 AND state='succeeded' AND settled_at IS NOT NULL`, f.issue.SessionID) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_runtime_reservation WHERE run_id IN ($1,$2,$3) AND released_at IS NULL`, root, memberRun, continuation) != 0 {
		t.Fatal("capacity-one squad did not settle all three independent runs")
	}
}

func TestCollaborationHostUpgradeRequiresFreshOwnerConsentAndKeepsLegacyProof(t *testing.T) {
	f := newManagedCollaborationFixture(t, "local")
	f.submitManaged(t, f.handler, "already running in the original Linux mode")
	f.resumeManaged(t)
	id, _, claim := f.claimManaged(t)
	p := protocol.CollaborationRunPermitRequest{Identity: claim.Identity, Containment: protocol.CollaborationContainmentLinuxPIDNamespace, AttemptSeq: "1", AttemptNonce: uuid.NewString()}
	if _, err := f.svc.PermitRuntimeRun(f.daemonCtx, id, p); err != nil {
		t.Fatal(err)
	}
	beforeGrant := f.grant
	registration := hostCapabilityRegistration(f)
	if _, err := f.svc.RegisterRuntimeCapabilities(f.daemonCtx, registration); err != nil {
		t.Fatal(err)
	}
	target := queueTarget(t, f.collaborationQueueFixture, f.targetID)
	_, err := f.svc.SubmitCommand(f.ctx, CollaborationSubmitCommandParams{CollaborationQueueRequest: hostQueueRequest(f), TargetID: f.targetID,
		InputEventID: util.MustParseUUID(uuid.NewString()), ExpectedTargetEpoch: target.TargetEpoch, ExpectedConfigVersion: target.ConfigVersion, Content: "old consent must not expand access"})
	if err == nil {
		t.Fatal("Linux owner consent silently expanded to ordinary host access")
	}
	grant, err := f.svc.Queries.GetCollaborationGrant(f.ctx, beforeGrant.ID)
	if err != nil || grant.ConfigurationFingerprint != beforeGrant.ConfigurationFingerprint || grant.ConfigVersion != beforeGrant.ConfigVersion {
		t.Fatal("upgrade rewrote old consent history")
	}
	if _, err = f.svc.AcceptRuntimeProof(context.Background(), id, claim.TerminationToken, managedProof(claim.Identity, p.AttemptNonce)); err != nil {
		t.Fatalf("new host capability stranded original Linux proof: %v", err)
	}
	old, err := f.svc.Queries.GetCollaborationRuntimeRun(f.ctx, id)
	if err != nil || old.Containment.String != protocol.CollaborationContainmentLinuxPIDNamespace || old.ProofState != "exited_proven" {
		t.Fatal("upgrade reinterpreted the old execution mode")
	}
}

func TestCollaborationHostPermissionChangeRetainsWaitingInputsAndRequiresResume(t *testing.T) {
	f, caps := newHostCollaborationFixture(t)
	waiting := submitHostCommand(t, f, f.handler, "retained while owner confirms permissions")
	request := hostCapabilityRegistration(f)
	request.Capabilities.Host.ConfigurationDigest = strings.Repeat("e", 64)
	if _, err := f.svc.RegisterRuntimeCapabilities(f.daemonCtx, request); err != nil {
		t.Fatal(err)
	}
	var err error
	f.runtime, err = f.svc.Queries.GetAgentRuntime(f.ctx, f.runtime.ID)
	if err != nil {
		t.Fatal(err)
	}
	if caps.Host.ConfigurationDigest == request.Capabilities.Host.ConfigurationDigest || f.runtime.CollaborationConfigEpoch != 3 {
		t.Fatal("permission expansion did not invalidate its exact registration")
	}
	renewHostConsent(t, f)
	target := queueTarget(t, f.collaborationQueueFixture, f.targetID)
	if !target.HoldReason.Valid || target.QueueState != "paused" ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=$1 AND state='waiting'`, waiting.CommandID) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE session_id=$1`, f.issue.SessionID) != 0 {
		t.Fatal("renewed authorization consumed or discarded waiting input")
	}
	listed, err := f.svc.ListRuntimeRuns(f.daemonCtx, f.runtime.ID)
	if err != nil || len(listed.RunIDs) != 0 {
		t.Fatalf("reconsent automatically resumed execution: %+v %v", listed, err)
	}
	id, claim := claimHostRun(t, f, request.Capabilities)
	if claim.HostContext.SessionGeneration != strconv.FormatInt(target.ConfigVersion, 10)+":3" || len(claim.Inputs) != 1 || claim.Inputs[0].CommandID != util.UUIDToString(waiting.CommandID) {
		t.Fatal("fresh explicit Resume lost the retained input or configuration generation")
	}
	if _, err = f.svc.AcceptRuntimeProof(context.Background(), id, claim.TerminationToken, hostFinalProof(t, claim)); err != nil {
		t.Fatal(err)
	}
}

func TestCollaborationHostGateStopsNewAttemptsButPreservesCleanup(t *testing.T) {
	f, caps := newHostCollaborationFixture(t)
	submitHostCommand(t, f, f.creator, "host admission can be rolled back independently")
	id, claim := claimHostRun(t, f, caps)
	first := hostPermitRequest(claim, 1, "provider")
	if _, err := f.svc.PermitRuntimeRun(f.daemonCtx, id, first); err != nil {
		t.Fatal(err)
	}
	end := hostLaunchEnded(first, true)
	f.fx.Exec(t, `UPDATE collaboration_execution_gate SET host_execution_enabled=false,epoch=epoch+1 WHERE id=1`)
	if _, err := f.svc.PermitRuntimeRun(f.daemonCtx, id, hostPermitRequest(claim, 2, "provider")); !collaborationErrorCode(err, "execution_mode_unsupported") {
		t.Fatalf("closing host gate allowed a new launch: %v", err)
	}
	if _, err := f.svc.AcceptRuntimeLaunchTermination(context.Background(), id, claim.TerminationToken, end); err != nil {
		t.Fatalf("closed host gate blocked original launch cleanup: %v", err)
	}
	if _, err := f.svc.AcceptRuntimeProof(context.Background(), id, claim.TerminationToken, hostFinalProof(t, claim, end)); err != nil {
		t.Fatalf("closed host gate blocked original final cleanup: %v", err)
	}
}

func TestCollaborationHostConcurrentPermitsNeverAuthorizeTwoLaunchesAtOneSequence(t *testing.T) {
	f, caps := newHostCollaborationFixture(t)
	submitHostCommand(t, f, f.creator, "two daemon deliveries contend for one launch")
	id, claim := claimHostRun(t, f, caps)
	requests := []protocol.CollaborationRunPermitRequest{hostPermitRequest(claim, 1, "provider"), hostPermitRequest(claim, 1, "provider")}
	start := make(chan struct{})
	type result struct {
		index int
		err   error
	}
	results := make(chan result, 2)
	for index, request := range requests {
		go func() {
			<-start
			_, err := f.svc.PermitRuntimeRun(f.daemonCtx, id, request)
			results <- result{index, err}
		}()
	}
	close(start)
	winner, accepted := -1, 0
	for range requests {
		got := <-results
		if got.err == nil {
			winner, accepted = got.index, accepted+1
		} else if !collaborationErrorCode(got.err, "attempt_conflict") {
			t.Fatalf("unexpected concurrent permit failure: %v", got.err)
		}
	}
	if accepted != 1 || f.fx.Count(t, `SELECT count(*) FROM collaboration_attempt WHERE run_id=$1`, id) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_business_capability WHERE run_id=$1`, id) != 1 {
		t.Fatal("two different launch identities acquired one sequence")
	}
	end := hostLaunchEnded(requests[winner], true)
	if _, err := f.svc.AcceptRuntimeLaunchTermination(context.Background(), id, claim.TerminationToken, end); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.AcceptRuntimeProof(context.Background(), id, claim.TerminationToken, hostFinalProof(t, claim, end)); err != nil {
		t.Fatal(err)
	}
}

func TestCollaborationHostUnissuedTombstoneCannotBecomeAPermitInStorage(t *testing.T) {
	f, caps := newHostCollaborationFixture(t)
	submitHostCommand(t, f, f.creator, "retain original unissued launch identity")
	id, claim := claimHostRun(t, f, caps)
	pending := hostPermitRequest(claim, 1, "provider")
	if _, err := f.svc.AcceptRuntimeLaunchTermination(context.Background(), id, claim.TerminationToken, hostLaunchEnded(pending, false)); err != nil {
		t.Fatal(err)
	}
	_, err := f.pool.Exec(t.Context(), `UPDATE collaboration_attempt SET authorized_at=now(), launch_descriptor=jsonb_build_object('kind','provider','capability_digest',$2::text,'configuration_digest',$3::text) WHERE run_id=$1 AND attempt_seq=1`,
		id, pending.LaunchDescriptor.CapabilityDigest, pending.LaunchDescriptor.ConfigurationDigest)
	if err == nil {
		t.Fatal("storage allowed an acknowledged unissued launch to become an authorized permit")
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_attempt WHERE run_id=$1 AND authorized_at IS NULL AND launch_descriptor IS NULL`, id) != 1 {
		t.Fatal("failed tombstone mutation changed retained launch authority")
	}
}

func TestCollaborationHostCapabilityCannotReclassifyRemoteRuntimeAsLocal(t *testing.T) {
	f := newManagedCollaborationFixture(t)
	f.fx.Exec(t, `UPDATE agent_runtime SET runtime_mode='cloud' WHERE id=$1`, f.runtime.ID)
	var err error
	f.runtime, err = f.svc.Queries.GetAgentRuntime(f.ctx, f.runtime.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.svc.RegisterRuntimeCapabilities(f.daemonCtx, hostCapabilityRegistration(f)); !collaborationErrorCode(err, "unsupported_runtime_configuration") {
		t.Fatalf("local lifecycle claim was accepted for remote runtime: %v", err)
	}
}

func TestCollaborationHostClaimKeepsAgentOwnerSeparateFromRuntimeOwner(t *testing.T) {
	f, caps := newHostCollaborationFixture(t)
	f.fx.Member(t, f.workspace, f.handler, "member")
	f.fx.Exec(t, `UPDATE agent SET owner_id=$2,permission_mode='public_to' WHERE id=$1`, f.grant.AgentID, f.handler)
	f.fx.Insert(t, "agent_invocation_target", testutil.Cols{"agent_id": f.grant.AgentID, "target_type": "member", "target_id": f.creator, "created_by": f.handler})
	renewHostConsent(t, f)
	submitHostCommand(t, f, f.handler, "agent owner submits through another owner's runtime")
	resume := f.issueControl(t)
	resume.ExecutionViewVersion = 6
	if _, err := f.svc.ResumeQueue(f.ctx, CollaborationResumeQueueParams{CollaborationStopQueueParams: resume, ExpectedExecutionEpoch: f.gateEpoch}); err != nil {
		t.Fatal(err)
	}
	listed, err := f.svc.ListRuntimeRuns(f.daemonCtx, f.runtime.ID)
	if err != nil || len(listed.RunIDs) != 1 {
		t.Fatalf("separately owned agent was not admitted: %+v %v", listed, err)
	}
	id := util.MustParseUUID(listed.RunIDs[0])
	claim, err := f.svc.ClaimRuntimeRun(f.daemonCtx, id, hostClaimRequest(t, f, caps))
	if err != nil || claim.HostContext == nil || claim.HostContext.AgentOwnerUserID != f.handler || claim.ResourceOwnerID != f.creator ||
		claim.Preparation.Resource.AgentOwnerUserID != f.handler || len(claim.Inputs) != 1 || claim.Inputs[0].AuthorUserID != f.handler {
		t.Fatalf("host claim collapsed author, agent owner and runtime owner: %v", err)
	}
	if _, err = f.svc.AcceptRuntimeProof(context.Background(), id, claim.TerminationToken, hostFinalProof(t, claim)); err != nil {
		t.Fatal(err)
	}
}
