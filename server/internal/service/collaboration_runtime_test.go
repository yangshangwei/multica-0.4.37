package service

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"strconv"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/installation"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

type managedCollaborationFixture struct {
	*collaborationQueueFixture
	runtime      db.AgentRuntime
	grant        db.CollaborationGrant
	binding      db.InstallationDaemonBinding
	installation db.ManagedInstallation
	daemonCtx    context.Context
	gateEpoch    int64
}

// This fixture establishes real persisted managed/owner-consent prerequisites.
// The tests below exercise server protocol state only; native containment is
// proven separately by collaborationrun tests and the real-router Linux loop.
func newManagedCollaborationFixture(t *testing.T) *managedCollaborationFixture {
	t.Helper()
	t.Setenv("MULTICA_AUTH_MODE", "legacy")
	t.Setenv("MULTICA_DEPLOYMENT_ID", "68150177-594d-45d8-8a84-bc979b498b04")
	base := newCollaborationFixture(t)
	issue, err := base.svc.Create(base.ctx, base.params(t))
	if err != nil {
		t.Fatal(err)
	}
	f := &managedCollaborationFixture{collaborationQueueFixture: &collaborationQueueFixture{collaborationFixture: base, issue: issue}}
	for _, table := range []string{"collaboration_target", "collaboration_grant", "collaboration_channel", "collaboration_batch", "collaboration_command", "collaboration_execution_group", "collaboration_run"} {
		switch table {
		case "collaboration_grant":
			f.fx.Cleanup(t, `DELETE FROM collaboration_grant WHERE issue_id=$1`, issue.Issue.ID)
		default:
			f.fx.Cleanup(t, `DELETE FROM `+table+` WHERE session_id=$1`, issue.SessionID)
		}
	}
	for _, table := range []string{"collaboration_runtime_reservation", "collaboration_attempt", "collaboration_business_capability", "collaboration_termination", "collaboration_run_event"} {
		f.fx.Cleanup(t, `DELETE FROM `+table+` WHERE run_id IN (SELECT id FROM collaboration_run WHERE session_id=$1)`, issue.SessionID)
	}
	f.fx.Cleanup(t, `DELETE FROM collaboration_operation WHERE scope_kind='issue' AND scope_id=$1`, issue.Issue.ID)
	for _, user := range []string{f.creator, f.handler} {
		f.fx.InsertNoID(t, "user_password_credential", testutil.Cols{"user_id": user, "username": "runtime" + uuid.NewString()[:8], "password_hash": "fixture-not-used-for-login", "must_change_password": false}, "user_id=$1", user)
	}
	org, err := f.svc.Queries.EnsureInternalOrganization(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	f.fx.InsertNoID(t, "organization_workspace", testutil.Cols{"workspace_id": f.workspace, "organization_id": org.ID}, "workspace_id=$1", f.workspace)
	public, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	inst := f.fx.Insert(t, "managed_installation", testutil.Cols{"deployment_id": auth.ManagedDeploymentID(), "organization_id": org.ID, "public_key": []byte(public), "key_fingerprint": installation.Digest(public), "responsible_user_id": f.creator, "os": "linux"})
	daemon := uuid.NewString()
	runtimeID := f.fx.Runtime(t, "Managed collaboration fake runtime", testutil.Cols{"owner_id": f.creator, "daemon_id": daemon, "provider": "codex", "status": "online"})
	agentID := f.fx.Agent(t, "Managed collaboration fake agent "+uuid.NewString(), runtimeID, testutil.Cols{"owner_id": f.creator, "permission_mode": "private"})
	bindingID := f.fx.Insert(t, "installation_daemon_binding", testutil.Cols{"installation_id": inst, "workspace_id": f.workspace, "daemon_id": daemon, "principal_user_id": f.creator, "auth_version": 1, "binding_epoch": 1})
	t.Setenv("MULTICA_AUTH_MODE", "password")
	f.ctx = collaborationPasswordContext(util.MustParseUUID(f.creator), 1)
	f.daemonCtx = auth.WithPasswordSession(context.Background(), auth.PasswordSession{UserID: f.creator, Version: 1, Kind: "daemon_token", WorkspaceID: f.workspace, DaemonID: daemon, BindingID: bindingID, BindingEpoch: 1})
	registered, err := f.svc.RegisterTarget(f.ctx, CollaborationRegisterTargetParams{CollaborationQueueRequest: f.request(), Kind: collaboration.TargetAgent, ResourceID: util.MustParseUUID(agentID), ExpectedTargetSetVersion: 1,
		Roster: []collaboration.TargetResource{{Role: "agent", AgentID: agentID, RuntimeID: runtimeID, ExecutionWorkspaceID: f.workspace, OwnerUserID: f.creator}}})
	if err != nil {
		t.Fatal(err)
	}
	f.targetID = registered.TargetID
	consented, err := f.svc.ConsentTargetResource(f.ctx, CollaborationConsentParams{CollaborationQueueRequest: f.request(), TargetID: f.targetID, AgentID: util.MustParseUUID(agentID), ExpectedTargetEpoch: 1, ExpectedConfigVersion: 1,
		Identity: CollaborationResourceIdentity{Kind: "managed", InstallationID: util.MustParseUUID(inst), BindingID: util.MustParseUUID(bindingID), BindingEpoch: 1}})
	if err != nil {
		t.Fatal(err)
	}
	f.grant, err = f.svc.Queries.GetCollaborationGrant(f.ctx, consented.GrantID)
	if err != nil {
		t.Fatal(err)
	}
	f.runtime, err = f.svc.Queries.GetAgentRuntime(f.ctx, util.MustParseUUID(runtimeID))
	if err != nil {
		t.Fatal(err)
	}
	f.binding, err = f.svc.Queries.GetInstallationBinding(f.ctx, util.MustParseUUID(bindingID))
	if err != nil {
		t.Fatal(err)
	}
	f.installation, err = f.svc.Queries.GetManagedInstallation(f.ctx, util.MustParseUUID(inst))
	if err != nil {
		t.Fatal(err)
	}
	gate, err := f.svc.Queries.LockCollaborationGateShared(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	f.gateEpoch = gate.Epoch + 1
	f.fx.Exec(t, `UPDATE collaboration_execution_gate SET enabled=true,epoch=$1 WHERE id=1`, f.gateEpoch)
	f.fx.Cleanup(t, `UPDATE collaboration_execution_gate SET enabled=$1,epoch=$2 WHERE id=1`, gate.Enabled, gate.Epoch)
	return f
}

func (f *managedCollaborationFixture) submitManaged(t *testing.T, user, body string) CollaborationQueueReceipt {
	t.Helper()
	target := queueTarget(t, f.collaborationQueueFixture, f.targetID)
	r, err := f.svc.SubmitCommand(collaborationPasswordContext(util.MustParseUUID(user), 1), CollaborationSubmitCommandParams{CollaborationQueueRequest: f.request(), TargetID: f.targetID, InputEventID: util.MustParseUUID(uuid.NewString()), ExpectedTargetEpoch: target.TargetEpoch, ExpectedConfigVersion: target.ConfigVersion, Content: body})
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func (f *managedCollaborationFixture) resumeManaged(t *testing.T) {
	t.Helper()
	_, err := f.svc.ResumeQueue(f.ctx, CollaborationResumeQueueParams{CollaborationStopQueueParams: f.issueControl(t), ExpectedExecutionEpoch: f.gateEpoch})
	if err != nil {
		t.Fatal(err)
	}
}

func (f *managedCollaborationFixture) claimManaged(t *testing.T) (pgtype.UUID, protocol.CollaborationRunClaimRequest, protocol.CollaborationRunClaimResponse) {
	t.Helper()
	list, err := f.svc.ListRuntimeRuns(f.daemonCtx, f.runtime.ID)
	if err != nil || len(list.RunIDs) != 1 {
		t.Fatalf("one admitted runtime candidate: %+v %v", list, err)
	}
	id := util.MustParseUUID(list.RunIDs[0])
	p := protocol.CollaborationRunClaimRequest{ProtocolVersion: protocol.CollaborationRuntimeProtocolVersion, DeploymentID: auth.ManagedDeploymentID(), InstallationID: util.UUIDToString(f.installation.ID), RuntimeID: util.UUIDToString(f.runtime.ID), BindingID: util.UUIDToString(f.binding.ID), BindingEpoch: strconv.FormatInt(f.binding.BindingEpoch, 10), Containment: protocol.CollaborationContainmentLinuxPIDNamespace}
	claim, err := f.svc.ClaimRuntimeRun(f.daemonCtx, id, p)
	if err != nil {
		t.Fatal(err)
	}
	return id, p, claim
}

func managedProof(identity protocol.CollaborationRunIdentity, nonce string) protocol.CollaborationRunProof {
	zero := 0
	return protocol.CollaborationRunProof{ReceiptID: uuid.NewString(), Identity: identity, LastAuthorizedAttemptSeq: "1", SealedAttemptSeq: "1", AttemptNonce: nonce, State: "exited_proven", Containment: protocol.CollaborationContainmentLinuxPIDNamespace, ExitCode: &zero}
}

func TestCollaborationManagedRuntimeBatchProofAndExactReplay(t *testing.T) {
	f := newManagedCollaborationFixture(t)
	one := f.submitManaged(t, f.creator, "first author")
	two := f.submitManaged(t, f.handler, "second author")
	f.resumeManaged(t)
	runID, request, claim := f.claimManaged(t)
	if len(claim.Inputs) != 2 || claim.Inputs[0].AuthorUserID == claim.Inputs[1].AuthorUserID || one.BatchID != two.BatchID {
		t.Fatal("batch authorization lost a human author")
	}
	replay, err := f.svc.ClaimRuntimeRun(f.daemonCtx, runID, request)
	if err != nil || replay.TerminationToken != claim.TerminationToken || replay.Identity != claim.Identity {
		t.Fatalf("claim replay changed launch/proof capability: %v", err)
	}
	p := protocol.CollaborationRunPermitRequest{Identity: claim.Identity, AttemptSeq: "1", AttemptNonce: uuid.NewString(), Containment: protocol.CollaborationContainmentLinuxPIDNamespace}
	permit, err := f.svc.PermitRuntimeRun(f.daemonCtx, runID, p)
	if err != nil {
		t.Fatal(err)
	}
	permitReplay, err := f.svc.PermitRuntimeRun(f.daemonCtx, runID, p)
	if err != nil || permitReplay.BusinessToken != permit.BusinessToken {
		t.Fatalf("permit replay rotated its capability: %v", err)
	}
	replay, err = f.svc.ClaimRuntimeRun(f.daemonCtx, runID, request)
	if err != nil || replay.TerminationToken != claim.TerminationToken {
		t.Fatalf("claim replay after permit stranded termination: %v", err)
	}
	if err = f.svc.RecordRuntimeOutcome(context.Background(), runID, permit.BusinessToken, protocol.CollaborationRunOutcome{Identity: claim.Identity, EventID: uuid.NewString(), AttemptSeq: "1", AttemptNonce: p.AttemptNonce, Outcome: "success", Body: "private batch result"}); err != nil {
		t.Fatal(err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_runtime_reservation WHERE run_id=$1 AND released_at IS NULL`, runID) != 1 || f.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE id=$1 AND settled_at IS NULL`, runID) != 1 {
		t.Fatal("business outcome released a live process reservation")
	}
	proof := managedProof(claim.Identity, p.AttemptNonce)
	accepted, err := f.svc.AcceptRuntimeProof(context.Background(), runID, claim.TerminationToken, proof)
	if err != nil || !accepted.Accepted {
		t.Fatalf("sealed proof: %+v %v", accepted, err)
	}
	if _, err = f.svc.AcceptRuntimeProof(context.Background(), runID, claim.TerminationToken, proof); err != nil {
		t.Fatal(err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE batch_id=$1 AND state='succeeded'`, one.BatchID) != 2 || f.fx.Count(t, `SELECT count(*) FROM collaboration_runtime_reservation WHERE run_id=$1 AND released_at IS NOT NULL`, runID) != 1 {
		t.Fatal("proof did not atomically settle the whole batch and release its reservation")
	}
	stored, err := f.svc.Queries.GetCollaborationTermination(f.ctx, runID)
	if err != nil {
		t.Fatal(err)
	}
	if string(stored.TokenCiphertext) == claim.TerminationToken || json.Valid(stored.TokenCiphertext) {
		t.Fatal("termination replay material was stored as plaintext")
	}
}

func TestCollaborationManagedStopProofPreservesFreshResumeIntent(t *testing.T) {
	f := newManagedCollaborationFixture(t)
	f.submitManaged(t, f.creator, "first batch")
	f.resumeManaged(t)
	runID, _, claim := f.claimManaged(t)
	p := protocol.CollaborationRunPermitRequest{Identity: claim.Identity, AttemptSeq: "1", AttemptNonce: uuid.NewString(), Containment: protocol.CollaborationContainmentLinuxPIDNamespace}
	if _, err := f.svc.PermitRuntimeRun(f.daemonCtx, runID, p); err != nil {
		t.Fatal(err)
	}
	next := f.submitManaged(t, f.handler, "queued followup")
	run, err := f.svc.Queries.GetCollaborationRuntimeRun(f.ctx, runID)
	if err != nil {
		t.Fatal(err)
	}
	target := queueTarget(t, f.collaborationQueueFixture, f.targetID)
	if _, err = f.svc.StopQueue(f.ctx, f.stopParams(run, target.ControlVersion)); err != nil {
		t.Fatal(err)
	}
	intent, err := f.svc.RuntimeStopIntent(context.Background(), runID, claim.TerminationToken)
	if err != nil || !intent.StopRequested || intent.Identity != claim.Identity {
		t.Fatalf("stop intent lost its historical launch identity: %+v %v", intent, err)
	}
	resume := CollaborationResumeQueueParams{CollaborationStopQueueParams: CollaborationStopQueueParams{CollaborationQueueRequest: f.request(), Scope: collaboration.ControlTarget, Targets: []CollaborationTargetControl{targetControl(queueTarget(t, f.collaborationQueueFixture, f.targetID))}}, ExpectedExecutionEpoch: f.gateEpoch}
	if _, err = f.svc.ResumeQueue(f.ctx, resume); !collaborationErrorCode(err, "stop_unconfirmed") {
		t.Fatalf("resume skipped proof: %v", err)
	}
	if _, err = f.svc.AcceptRuntimeProof(context.Background(), runID, claim.TerminationToken, managedProof(claim.Identity, p.AttemptNonce)); err != nil {
		t.Fatal(err)
	}
	if _, err = f.svc.ResumeQueue(f.ctx, resume); !collaborationErrorCode(err, "stop_unconfirmed") {
		t.Fatalf("old refused request became a deferred resume: %v", err)
	}
	resume.RequestID = uuid.NewString()
	resume.Targets = []CollaborationTargetControl{targetControl(queueTarget(t, f.collaborationQueueFixture, f.targetID))}
	if _, err = f.svc.ResumeQueue(f.ctx, resume); err != nil {
		t.Fatal(err)
	}
	newID, _, newClaim := f.claimManaged(t)
	if newID == runID || newClaim.Identity.LaunchNonce == claim.Identity.LaunchNonce || newClaim.Identity.BatchID != util.UUIDToString(next.BatchID) {
		t.Fatal("explicit resume revived the stopped run instead of starting its waiting batch")
	}
}

func TestCollaborationTerminationSurvivesOwnerAndRuntimeRemoval(t *testing.T) {
	f := newManagedCollaborationFixture(t)
	f.submitManaged(t, f.creator, "must retain narrow proof authority")
	f.resumeManaged(t)
	runID, _, claim := f.claimManaged(t)
	p := protocol.CollaborationRunPermitRequest{Identity: claim.Identity, AttemptSeq: "1", AttemptNonce: uuid.NewString(), Containment: protocol.CollaborationContainmentLinuxPIDNamespace}
	permit, err := f.svc.PermitRuntimeRun(f.daemonCtx, runID, p)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.svc.RevokeGrant(f.ctx, CollaborationRevokeGrantParams{CollaborationQueueRequest: f.request(), GrantID: f.grant.ID, ExpectedGrantEpoch: 1}); err != nil {
		t.Fatal(err)
	}
	f.fx.Exec(t, `UPDATE "user" SET disabled_at=now() WHERE id=$1`, f.creator)
	f.fx.Exec(t, `UPDATE agent SET runtime_id=NULL WHERE runtime_id=$1`, f.runtime.ID)
	f.fx.Exec(t, `DELETE FROM agent_runtime WHERE id=$1`, f.runtime.ID)
	f.fx.Exec(t, `DELETE FROM collaboration_grant WHERE id=$1`, f.grant.ID)
	if _, err = f.svc.PermitRuntimeRun(f.daemonCtx, runID, p); err == nil {
		t.Fatal("revoked runtime obtained another permit")
	}
	if err = f.svc.RecordRuntimeOutcome(context.Background(), runID, permit.BusinessToken, protocol.CollaborationRunOutcome{Identity: claim.Identity, EventID: uuid.NewString(), AttemptSeq: "1", AttemptNonce: p.AttemptNonce, Outcome: "success", Body: "late private output"}); err == nil {
		t.Fatal("revocation allowed late private output")
	}
	intent, err := f.svc.RuntimeStopIntent(context.Background(), runID, claim.TerminationToken)
	if err != nil || !intent.StopRequested {
		t.Fatalf("resource cleanup removed narrow stop authority: %v", err)
	}
	if _, err = f.svc.AcceptRuntimeProof(context.Background(), runID, claim.TerminationToken, managedProof(claim.Identity, p.AttemptNonce)); err != nil {
		t.Fatalf("revoked owner/runtime stranded a sealed proof: %v", err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE id=$1 AND forced_disposition='stopped' AND proof_state='exited_proven' AND recording_incomplete AND settled_at IS NOT NULL`, runID) != 1 {
		t.Fatal("late proof lost forced disposition or pretended missing business output was success")
	}
}

func TestCollaborationRemovalOfNonRootBatchAuthorStopsRun(t *testing.T) {
	f := newManagedCollaborationFixture(t)
	f.submitManaged(t, f.creator, "root author")
	f.submitManaged(t, f.handler, "second author")
	f.resumeManaged(t)
	runID, _, claim := f.claimManaged(t)
	if _, err := f.svc.PermitRuntimeRun(f.daemonCtx, runID, protocol.CollaborationRunPermitRequest{Identity: claim.Identity, AttemptSeq: "1", AttemptNonce: uuid.NewString(), Containment: protocol.CollaborationContainmentLinuxPIDNamespace}); err != nil {
		t.Fatal(err)
	}
	_, err := f.svc.RemoveParticipant(f.ctx, CollaborationParticipantParams{CollaborationWorkflowRequest: CollaborationWorkflowRequest{CollaborationQueueRequest: f.request(), ExpectedCollaborationRevision: 1}, TargetUserID: util.MustParseUUID(f.handler)})
	if err != nil {
		t.Fatal(err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE id=$1 AND forced_disposition='stopped' AND settled_at IS NULL`, runID) != 1 {
		t.Fatal("removing a non-root batch author left their accepted input executing")
	}
}
