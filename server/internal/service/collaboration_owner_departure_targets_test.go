package service

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// Independent managed namespaces are test prerequisites. Claims/permits below
// use the real service; sealed proof payloads do not observe a kernel process.
func ownerSwitchNamespace(t *testing.T, f *managedCollaborationFixture, owner, name string) *managedCollaborationFixture {
	t.Helper()
	other := *f
	queue := *f.collaborationQueueFixture
	other.collaborationQueueFixture = &queue
	daemon := uuid.NewString()
	runtime := f.fx.Runtime(t, name+" runtime", testutil.Cols{"owner_id": owner, "daemon_id": daemon, "provider": "codex", "status": "online"})
	other.runtime = mustRuntime(t, f.svc.Queries, util.MustParseUUID(runtime))
	binding := f.fx.Insert(t, "installation_daemon_binding", testutil.Cols{"installation_id": f.installation.ID, "workspace_id": f.workspace,
		"daemon_id": daemon, "principal_user_id": owner, "auth_version": 1, "binding_epoch": 1})
	var err error
	other.binding, err = f.svc.Queries.GetInstallationBinding(f.ctx, util.MustParseUUID(binding))
	if err != nil {
		t.Fatal(err)
	}
	other.daemonCtx = auth.WithPasswordSession(context.Background(), auth.PasswordSession{UserID: owner, Version: 1, Kind: "daemon_token",
		WorkspaceID: f.workspace, DaemonID: daemon, BindingID: binding, BindingEpoch: 1})
	agent := f.fx.Agent(t, name+" agent", runtime, testutil.Cols{"owner_id": owner, "permission_mode": "private"})
	other.grant = db.CollaborationGrant{AgentID: util.MustParseUUID(agent)}
	f.fx.Cleanup(t, `DELETE FROM collaboration_runtime_scan_cursor WHERE runtime_id=$1`, other.runtime.ID)
	return &other
}

func mustRuntime(t *testing.T, q *db.Queries, id pgtype.UUID) db.AgentRuntime {
	t.Helper()
	runtime, err := q.GetAgentRuntime(t.Context(), id)
	if err != nil {
		t.Fatal(err)
	}
	return runtime
}

func ownerSwitchResource(f *managedCollaborationFixture, role string) collaboration.TargetResource {
	return collaboration.TargetResource{Role: role, AgentID: util.UUIDToString(f.grant.AgentID), RuntimeID: util.UUIDToString(f.runtime.ID),
		ExecutionWorkspaceID: f.workspace, OwnerUserID: util.UUIDToString(f.runtime.OwnerID)}
}

func ownerSwitchConsent(t *testing.T, f *managedCollaborationFixture, target db.CollaborationTarget) db.CollaborationGrant {
	t.Helper()
	owner := collaborationPasswordContext(f.runtime.OwnerID, 1)
	consented, err := f.svc.ConsentTargetResource(owner, CollaborationConsentParams{CollaborationQueueRequest: f.request(),
		TargetID: target.ID, AgentID: f.grant.AgentID, ExpectedTargetEpoch: target.TargetEpoch, ExpectedConfigVersion: target.ConfigVersion,
		Identity: CollaborationResourceIdentity{Kind: "managed", InstallationID: f.installation.ID, BindingID: f.binding.ID, BindingEpoch: f.binding.BindingEpoch}})
	if err != nil {
		t.Fatal(err)
	}
	grant, err := f.svc.Queries.GetCollaborationGrant(owner, consented.GrantID)
	if err != nil {
		t.Fatal(err)
	}
	return grant
}

func ownerSwitchRegister(t *testing.T, f *managedCollaborationFixture, kind collaboration.TargetKind, resource pgtype.UUID, roster []collaboration.TargetResource) db.CollaborationTarget {
	t.Helper()
	session, err := f.svc.Queries.GetCollaborationSession(f.ctx, f.issue.Issue.ID)
	if err != nil {
		t.Fatal(err)
	}
	registered, err := f.svc.RegisterTarget(f.ctx, CollaborationRegisterTargetParams{CollaborationQueueRequest: f.request(), Kind: kind,
		ResourceID: resource, ExpectedTargetSetVersion: session.TargetSetVersion, Roster: roster})
	if err != nil {
		t.Fatal(err)
	}
	f.targetID = registered.TargetID
	return queueTarget(t, f.collaborationQueueFixture, registered.TargetID)
}

type ownerSwitchPermittedRun struct {
	run    pgtype.UUID
	claim  protocol.CollaborationRunClaimResponse
	permit protocol.CollaborationRunPermitResponse
}

func ownerSwitchClaimPermit(t *testing.T, f *managedCollaborationFixture, input CollaborationQueueReceipt) ownerSwitchPermittedRun {
	t.Helper()
	list, err := f.svc.ListRuntimeRuns(f.daemonCtx, f.runtime.ID)
	if err != nil || len(list.RunIDs) != 1 {
		target := queueTarget(t, f.collaborationQueueFixture, input.TargetID)
		promotion, promoteErr := collaborationRuntimeMutation(f.daemonCtx, f.svc, func(tx pgx.Tx) (collaborationRuntimePromotion, error) {
			return f.svc.promoteCollaborationRuntimeBatch(f.daemonCtx, tx, target.SessionID, target.ID, f.runtime.ID)
		})
		t.Fatalf("exact namespace candidate unavailable name=%s runtime=%s target=%s hold=%v list=%+v err=%v promotion=%+v promoteErr=%v", f.runtime.Name, util.UUIDToString(f.runtime.ID), util.UUIDToString(target.ID), target.HoldReason, list, err, promotion, promoteErr)
	}
	run, _, claim := f.claimManaged(t)
	if len(claim.Inputs) != 1 || claim.Inputs[0].CommandID != util.UUIDToString(input.CommandID) ||
		claim.Identity.TargetID != util.UUIDToString(input.TargetID) ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE id=$1 AND runtime_id=$2`, run, f.runtime.ID) != 1 {
		t.Fatal("managed claim did not consume the original target input in its actual namespace")
	}
	permit, err := f.svc.PermitRuntimeRun(f.daemonCtx, run, protocol.CollaborationRunPermitRequest{Identity: claim.Identity, AttemptSeq: "1",
		AttemptNonce: uuid.NewString(), Containment: protocol.CollaborationContainmentLinuxPIDNamespace})
	if err != nil {
		t.Fatal(err)
	}
	return ownerSwitchPermittedRun{run: run, claim: claim, permit: permit}
}

func ownerSwitchTargetSnapshot(t *testing.T, f *managedCollaborationFixture, target pgtype.UUID) string {
	t.Helper()
	var snapshot string
	f.fx.QueryRow(t, `SELECT jsonb_build_object(
		'target',(SELECT to_jsonb(t) FROM collaboration_target t WHERE t.id=$1),
		'channel',(SELECT to_jsonb(c) FROM collaboration_channel c JOIN collaboration_target t ON t.channel_id=c.id WHERE t.id=$1),
		'grants',(SELECT jsonb_agg(to_jsonb(g) ORDER BY g.id) FROM collaboration_grant g WHERE g.target_id=$1),
		'commands',(SELECT jsonb_agg(to_jsonb(c) ORDER BY c.id) FROM collaboration_command c WHERE c.target_id=$1),
		'groups',(SELECT jsonb_agg(to_jsonb(g) ORDER BY g.id) FROM collaboration_execution_group g WHERE g.target_id=$1),
		'runs',(SELECT jsonb_agg(to_jsonb(r) ORDER BY r.id) FROM collaboration_run r WHERE r.target_id=$1),
		'attempts',(SELECT jsonb_agg(to_jsonb(a) ORDER BY a.run_id,a.attempt_seq) FROM collaboration_attempt a
			WHERE a.run_id IN (SELECT id FROM collaboration_run WHERE target_id=$1)),
		'proofs',(SELECT jsonb_agg(to_jsonb(p) ORDER BY p.run_id) FROM collaboration_termination p
			WHERE p.run_id IN (SELECT id FROM collaboration_run WHERE target_id=$1)),
		'reservations',(SELECT jsonb_agg(to_jsonb(r) ORDER BY r.run_id) FROM collaboration_runtime_reservation r
			WHERE r.run_id IN (SELECT id FROM collaboration_run WHERE target_id=$1)),
		'records',(SELECT jsonb_agg(to_jsonb(e) ORDER BY e.run_id,e.seq) FROM collaboration_run_event e
			WHERE e.run_id IN (SELECT id FROM collaboration_run WHERE target_id=$1))
	)::text`, target).Scan(&snapshot)
	return snapshot
}

func TestCollaborationOwnerDepartureRetractsMultipleTargetsAndSquadMember(t *testing.T) {
	f := newManagedCollaborationFixture(t)
	f.fx.Cleanup(t, `DELETE FROM collaboration_delegation WHERE group_id IN (SELECT id FROM collaboration_execution_group WHERE session_id=$1)`, f.issue.SessionID)
	d := f.outsider
	f.fx.Member(t, f.workspace, d, "member")
	f.fx.InsertNoID(t, "user_password_credential", testutil.Cols{"user_id": d, "username": "multitarget" + uuid.NewString()[:8],
		"password_hash": "fixture-not-used-for-login", "must_change_password": false}, "user_id=$1", d)
	if _, err := f.svc.InviteParticipant(f.ctx, CollaborationParticipantParams{CollaborationWorkflowRequest: workflowRequest(f.collaborationQueueFixture, 1), TargetUserID: util.MustParseUUID(d)}); err != nil {
		t.Fatal(err)
	}
	d1, d2, member := ownerSwitchNamespace(t, f, d, "D first"), ownerSwitchNamespace(t, f, d, "D second"), ownerSwitchNamespace(t, f, d, "D member")
	leader := ownerSwitchNamespace(t, f, f.creator, "Independent squad leader")
	for _, direct := range []*managedCollaborationFixture{d1, d2} {
		target := ownerSwitchRegister(t, direct, collaboration.TargetAgent, direct.grant.AgentID, []collaboration.TargetResource{ownerSwitchResource(direct, "agent")})
		direct.grant = ownerSwitchConsent(t, direct, target)
	}
	squadID := util.MustParseUUID(f.fx.Squad(t, "Multi-owner departure squad", util.UUIDToString(leader.grant.AgentID)))
	f.fx.SquadMember(t, util.UUIDToString(squadID), "agent", util.UUIDToString(member.grant.AgentID))
	squad := ownerSwitchRegister(t, leader, collaboration.TargetSquad, squadID, []collaboration.TargetResource{ownerSwitchResource(leader, "leader"), ownerSwitchResource(member, "member")})
	leader.grant = ownerSwitchConsent(t, leader, squad)
	member.targetID, member.grant = squad.ID, ownerSwitchConsent(t, member, squad)
	if len(authorityCatalogue(t, f.collaborationQueueFixture, f.ctx)) != 4 {
		t.Fatal("fully consented multi-owner targets did not become selectable")
	}
	f.resumeManaged(t)
	historyInput := f.submitManaged(t, f.creator, "Unrelated C completed record")
	history := ownerSwitchClaimPermit(t, f, historyInput)
	if err := f.svc.RecordRuntimeOutcome(context.Background(), history.run, history.permit.BusinessToken, recordedOutcome(history.claim, history.permit, 1)); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.AcceptRuntimeProof(context.Background(), history.run, history.claim.TerminationToken, managedProof(history.claim.Identity, history.permit.AttemptNonce)); err != nil {
		t.Fatal(err)
	}
	independentInput := f.submitManaged(t, f.creator, "Unrelated C current input")
	independent := ownerSwitchClaimPermit(t, f, independentInput)
	f.submitManaged(t, f.creator, "Unrelated C future input")
	active := []ownerSwitchPermittedRun{}
	for _, direct := range []*managedCollaborationFixture{d1, d2, leader} {
		input := submitManagedControlInput(t, direct, queueTarget(t, direct.collaborationQueueFixture, direct.targetID), f.creator, "Current resource-owned input")
		active = append(active, ownerSwitchClaimPermit(t, direct, input))
	}
	delegated, err := leader.svc.CreateRuntimeComment(context.Background(), active[2].run, active[2].permit.BusinessToken, CollaborationRuntimeCommentParams{
		CommentID: util.MustParseUUID(uuid.NewString()), Content: "[@Dmember](mention://agent/" + util.UUIDToString(member.grant.AgentID) + ") delegated work"})
	if err != nil || len(delegated.RunIDs) != 1 {
		t.Fatalf("actual squad member delegation: %+v %v", delegated, err)
	}
	_, memberRequest, memberClaim := member.claimManaged(t)
	if memberClaim.Identity.RunID != util.UUIDToString(delegated.RunIDs[0]) || memberClaim.ResourceOwnerID != d ||
		memberClaim.AgentID != util.UUIDToString(member.grant.AgentID) || memberClaim.ExecutionWorkspaceID != f.workspace {
		t.Fatal("derived member did not claim its original delegated identity")
	}
	memberRun, err := f.svc.Queries.GetCollaborationRuntimeRun(f.ctx, delegated.RunIDs[0])
	if err != nil || memberRun.ParentRunID != active[2].run || memberRun.GrantID != member.grant.ID || memberRun.GrantEpoch != member.grant.Epoch ||
		memberRun.RuntimeID != member.runtime.ID || memberRun.BindingID != member.binding.ID || memberRun.BindingEpoch != member.binding.BindingEpoch ||
		memberRun.InstallationID != member.installation.ID || memberRun.ResourceOwnerUserID != member.runtime.OwnerID || memberRun.StepKind.String != "member" {
		t.Fatalf("derived member borrowed the leader's resource/consent/runtime identity: %v", err)
	}
	beforeMember := controlStateSnapshot(t, f.collaborationQueueFixture)
	if _, err := f.svc.ClaimRuntimeRun(leader.daemonCtx, memberRun.ID, memberRequest); err == nil || beforeMember != controlStateSnapshot(t, f.collaborationQueueFixture) {
		t.Fatalf("leader credential acquired another owner's member execution: %v", err)
	}
	if _, err := f.svc.RuntimeContext(context.Background(), memberRun.ID, active[2].permit.BusinessToken); err == nil || beforeMember != controlStateSnapshot(t, f.collaborationQueueFixture) {
		t.Fatalf("leader business token expanded into member capability: %v", err)
	}
	memberPermit, err := member.svc.PermitRuntimeRun(member.daemonCtx, delegated.RunIDs[0], protocol.CollaborationRunPermitRequest{Identity: memberClaim.Identity,
		AttemptSeq: "1", AttemptNonce: uuid.NewString(), Containment: protocol.CollaborationContainmentLinuxPIDNamespace})
	if err != nil {
		t.Fatal(err)
	}
	active = append(active, ownerSwitchPermittedRun{run: delegated.RunIDs[0], claim: memberClaim, permit: memberPermit})
	var cancelled, retained []pgtype.UUID
	for _, target := range []*managedCollaborationFixture{d1, d2, leader} {
		current := queueTarget(t, target.collaborationQueueFixture, target.targetID)
		cancelled = append(cancelled, submitManagedControlInput(t, target, current, d, "D waiting before owner departure").CommandID)
		retained = append(retained, submitManagedControlInput(t, target, current, f.creator, "C waiting through owner departure").CommandID)
	}
	beforeIndependent := ownerSwitchTargetSnapshot(t, f, f.targetID)
	var immutable string
	f.fx.QueryRow(t, `SELECT jsonb_build_object(
		'inputs',(SELECT jsonb_agg(to_jsonb(i) ORDER BY i.group_id,i.command_id) FROM collaboration_group_input i
			WHERE i.group_id IN (SELECT id FROM collaboration_execution_group WHERE session_id=$1)),
		'consumed_commands',(SELECT jsonb_agg(to_jsonb(c) ORDER BY c.id) FROM collaboration_command c WHERE c.session_id=$1 AND c.state<>'waiting'),
		'dependencies',(SELECT jsonb_agg(to_jsonb(d)-'detached_at'-'detach_reason' ORDER BY d.group_id,d.command_id) FROM collaboration_group_dependency d
			WHERE d.group_id IN (SELECT id FROM collaboration_execution_group WHERE session_id=$1)),
		'run_identity',(SELECT jsonb_agg(jsonb_build_object('id',r.id,'target',r.target_id,'group',r.group_id,'command',r.command_id,
			'agent',r.agent_id,'runtime',r.runtime_id,'owner',r.resource_owner_user_id,'grant',r.grant_id,'grant_epoch',r.grant_epoch,
			'binding',r.binding_id,'binding_epoch',r.binding_epoch,'installation',r.installation_id,'target_epoch',r.target_epoch,
			'config_version',r.config_version,'channel',r.channel_id,'batch',r.batch_id,'parent',r.parent_run_id,
			'fence',r.fence,'launch_nonce',r.launch_nonce,'prompt',r.prompt_snapshot) ORDER BY r.id)
			FROM collaboration_run r WHERE r.session_id=$1),
		'proof_identity',(SELECT jsonb_agg(to_jsonb(p)-'stop_requested_at'-'confirmed_at'-'proof' ORDER BY p.run_id) FROM collaboration_termination p
			WHERE p.run_id IN (SELECT id FROM collaboration_run WHERE session_id=$1)),
		'attempts',(SELECT jsonb_agg(to_jsonb(a) ORDER BY a.run_id,a.attempt_seq) FROM collaboration_attempt a
			WHERE a.run_id IN (SELECT id FROM collaboration_run WHERE session_id=$1)),
		'records',(SELECT jsonb_agg(to_jsonb(e) ORDER BY e.run_id,e.seq) FROM collaboration_run_event e WHERE e.issue_id=$2)
	)::text`, f.issue.SessionID, f.issue.Issue.ID).Scan(&immutable)
	ownerCtx := collaborationPasswordContext(util.MustParseUUID(d), 1)
	leaveRequest := workflowRequest(f.collaborationQueueFixture, 2)
	left, err := f.svc.Leave(ownerCtx, leaveRequest)
	if err != nil || left.Code != "participant_left" || left.CollaborationRevision != 3 {
		t.Fatalf("actual multi-target resource owner departure: %+v %v", left, err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_grant WHERE issue_id=$1 AND owner_user_id=$2 AND revoked_at IS NOT NULL AND epoch=2 AND revocation_reason='participant_left'`, f.issue.Issue.ID, d) != 3 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_grant WHERE issue_id=$1 AND owner_user_id=$2 AND revoked_at IS NULL`, f.issue.Issue.ID, d) != 0 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=ANY($1::uuid[]) AND state='cancelled' AND reason='participant_left'`, cancelled) != len(cancelled) ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=ANY($1::uuid[]) AND state='waiting'`, retained) != len(retained) {
		t.Fatal("owner departure missed a direct/member grant, retained D input or cancelled another author")
	}
	for _, target := range []*managedCollaborationFixture{d1, d2, leader} {
		current := queueTarget(t, target.collaborationQueueFixture, target.targetID)
		if current.HoldReason.String != "grant_required" {
			t.Fatalf("affected target remained eligible: %+v", current)
		}
	}
	for _, old := range active {
		if f.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE id=$1 AND forced_disposition='stopped' AND proof_state='unknown' AND settled_at IS NULL
			AND claim_generation=$2::bigint+1`, old.run, old.claim.Identity.ClaimGeneration) != 1 ||
			f.fx.Count(t, `SELECT count(*) FROM collaboration_termination WHERE run_id=$1 AND stop_requested_at IS NOT NULL AND confirmed_at IS NULL AND proof IS NULL`, old.run) != 1 ||
			f.fx.Count(t, `SELECT count(*) FROM collaboration_runtime_reservation WHERE run_id=$1 AND released_at IS NULL`, old.run) != 1 {
			t.Fatal("owner departure fabricated proof or released a permitted direct/squad execution")
		}
	}
	items := authorityCatalogue(t, f.collaborationQueueFixture, f.ctx)
	if len(items) != 1 || items[0].TargetID != f.targetID || beforeIndependent != ownerSwitchTargetSnapshot(t, f, f.targetID) {
		t.Fatal("departed owner remained selectable or affected unrelated other-owner target/history")
	}
	for _, namespace := range []*managedCollaborationFixture{d1, d2, member, leader} {
		if list, err := f.svc.ListRuntimeRuns(namespace.daemonCtx, namespace.runtime.ID); err != nil || len(list.RunIDs) != 0 {
			t.Fatalf("owner departure left an affected runtime candidate: %+v %v", list, err)
		}
	}
	if _, err := member.svc.ClaimRuntimeRun(member.daemonCtx, active[3].run, memberRequest); err == nil {
		t.Fatal("old delegated claim reused withdrawn member authority")
	}
	var afterImmutable string
	f.fx.QueryRow(t, `SELECT jsonb_build_object(
		'inputs',(SELECT jsonb_agg(to_jsonb(i) ORDER BY i.group_id,i.command_id) FROM collaboration_group_input i
			WHERE i.group_id IN (SELECT id FROM collaboration_execution_group WHERE session_id=$1)),
		'consumed_commands',(SELECT jsonb_agg(to_jsonb(c) ORDER BY c.id) FROM collaboration_command c WHERE c.session_id=$1 AND c.id NOT IN
			(SELECT unnest($3::uuid[])) AND c.state<>'waiting'),
		'dependencies',(SELECT jsonb_agg(to_jsonb(d)-'detached_at'-'detach_reason' ORDER BY d.group_id,d.command_id) FROM collaboration_group_dependency d
			WHERE d.group_id IN (SELECT id FROM collaboration_execution_group WHERE session_id=$1)),
		'run_identity',(SELECT jsonb_agg(jsonb_build_object('id',r.id,'target',r.target_id,'group',r.group_id,'command',r.command_id,
			'agent',r.agent_id,'runtime',r.runtime_id,'owner',r.resource_owner_user_id,'grant',r.grant_id,'grant_epoch',r.grant_epoch,
			'binding',r.binding_id,'binding_epoch',r.binding_epoch,'installation',r.installation_id,'target_epoch',r.target_epoch,
			'config_version',r.config_version,'channel',r.channel_id,'batch',r.batch_id,'parent',r.parent_run_id,
			'fence',r.fence,'launch_nonce',r.launch_nonce,'prompt',r.prompt_snapshot) ORDER BY r.id)
			FROM collaboration_run r WHERE r.session_id=$1),
		'proof_identity',(SELECT jsonb_agg(to_jsonb(p)-'stop_requested_at'-'confirmed_at'-'proof' ORDER BY p.run_id) FROM collaboration_termination p
			WHERE p.run_id IN (SELECT id FROM collaboration_run WHERE session_id=$1)),
		'attempts',(SELECT jsonb_agg(to_jsonb(a) ORDER BY a.run_id,a.attempt_seq) FROM collaboration_attempt a
			WHERE a.run_id IN (SELECT id FROM collaboration_run WHERE session_id=$1)),
		'records',(SELECT jsonb_agg(to_jsonb(e) ORDER BY e.run_id,e.seq) FROM collaboration_run_event e WHERE e.issue_id=$2)
	)::text`, f.issue.SessionID, f.issue.Issue.ID, cancelled).Scan(&afterImmutable)
	if afterImmutable != immutable {
		var beforeFields, afterFields map[string]json.RawMessage
		if err := json.Unmarshal([]byte(immutable), &beforeFields); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal([]byte(afterImmutable), &afterFields); err != nil {
			t.Fatal(err)
		}
		for key, before := range beforeFields {
			if !reflect.DeepEqual(before, afterFields[key]) {
				t.Fatalf("owner departure changed retained %s facts", key)
			}
		}
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_session WHERE id=$1 AND queue_state='active' AND control_version=2`, f.issue.SessionID) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE id=$1 AND forced_disposition IS NULL AND settled_at IS NULL`, independent.run) != 1 {
		t.Fatal("owner departure erased immutable history or changed whole-issue/other-owner execution")
	}
	after := controlStateSnapshot(t, f.collaborationQueueFixture)
	recovered, err := f.svc.GetWorkflowOperation(ownerCtx, f.issue.Issue.ID, leaveRequest.RequestID)
	if err != nil || recovered.Code != left.Code || recovered.CollaborationRevision != left.CollaborationRevision || controlStateSnapshot(t, f.collaborationQueueFixture) != after {
		t.Fatalf("owner's own departure recovery mutated retained data: %+v %v", recovered, err)
	}
	ownerAccess, err := f.svc.ResolveIssueAccess(ownerCtx, nil, f.issue.Issue.ID)
	if err != nil {
		// D still belongs to the source workspace, but that does not restore the
		// withdrawn issue participation/resource grants.
		t.Fatal(err)
	}
	if !ownerAccess.SourceMember || ownerAccess.Capabilities.ControlAI {
		t.Fatal("source membership silently restored departed collaboration control")
	}
}
