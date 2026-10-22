package service

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"slices"
	"strconv"
	"strings"
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

// This combined service regression uses real persisted grants and distinct
// managed daemon principals. Its proof payloads do not observe provider processes.
func TestCollaborationCatalogueAuthorityMatrix(t *testing.T) {
	f := newManagedCollaborationFixture(t)
	direct := queueTarget(t, f.collaborationQueueFixture, f.targetID)
	f.fx.Cleanup(t, `DELETE FROM comment WHERE issue_id=$1`, f.issue.Issue.ID)
	for _, owner := range []string{f.handler, f.outsider} {
		f.fx.Member(t, f.workspace, owner, "member")
	}
	f.fx.InsertNoID(t, "user_password_credential", testutil.Cols{"user_id": f.outsider, "username": "catalogue" + uuid.NewString()[:8],
		"password_hash": "fixture-not-used-for-login", "must_change_password": false}, "user_id=$1", f.outsider)
	leader := ownerSwitchNamespace(t, f, f.creator, "Catalogue leader")
	memberA := ownerSwitchNamespace(t, f, f.handler, "Catalogue member A")
	memberB := ownerSwitchNamespace(t, f, f.outsider, "Catalogue member B")
	squadID := util.MustParseUUID(f.fx.Squad(t, "Three-owner catalogue squad", util.UUIDToString(leader.grant.AgentID)))
	for _, member := range []*managedCollaborationFixture{memberA, memberB} {
		f.fx.SquadMember(t, util.UUIDToString(squadID), "agent", util.UUIDToString(member.grant.AgentID))
	}
	f.fx.Cleanup(t, `DELETE FROM collaboration_delegation WHERE group_id IN (SELECT id FROM collaboration_execution_group WHERE session_id=$1)`, f.issue.SessionID)
	roster := []collaboration.TargetResource{ownerSwitchResource(leader, "leader"), ownerSwitchResource(memberA, "member"), ownerSwitchResource(memberB, "member")}
	session, err := f.svc.Queries.GetCollaborationSession(f.ctx, f.issue.Issue.ID)
	if err != nil {
		t.Fatal(err)
	}
	registration := CollaborationRegisterTargetParams{CollaborationQueueRequest: f.request(), Kind: collaboration.TargetSquad,
		ResourceID: squadID, ExpectedTargetSetVersion: session.TargetSetVersion, Roster: roster}
	workflow := func(ctx context.Context) CollaborationWorkflowRequest {
		t.Helper()
		access, err := f.svc.ResolveIssueAccess(ctx, nil, f.issue.Issue.ID)
		if err != nil {
			t.Fatal(err)
		}
		request := f.request()
		request.ExpectedLifecycleEpoch, request.ExpectedParticipantEpoch, request.ExpectedAccountEpoch = access.LifecycleEpoch, access.ParticipantEpoch.Int64, access.AccountEpoch.Int64
		return CollaborationWorkflowRequest{CollaborationQueueRequest: request, ExpectedCollaborationRevision: access.Revision}
	}
	mentions := func(ctx context.Context) map[pgtype.UUID]CollaborationMentionTarget {
		t.Helper()
		items := make(map[pgtype.UUID]CollaborationMentionTarget)
		for _, item := range authorityCatalogue(t, f.collaborationQueueFixture, ctx) {
			if _, duplicate := items[item.TargetID]; duplicate {
				t.Fatal("catalogue repeated a typed target")
			}
			items[item.TargetID] = item
		}
		return items
	}
	before := controlStateSnapshot(t, f.collaborationQueueFixture)
	if _, err = f.svc.RegisterTarget(f.ctx, registration); !collaborationErrorCode(err, "resource_owner_not_participant") || before != controlStateSnapshot(t, f.collaborationQueueFixture) {
		t.Fatalf("creator organized a squad before its actual member owner joined: %v", err)
	}
	outsiderCtx := collaborationPasswordContext(util.MustParseUUID(f.outsider), 1)
	if _, err = f.svc.ConsentTargetResource(outsiderCtx, CollaborationConsentParams{CollaborationQueueRequest: f.request(),
		TargetID: direct.ID, AgentID: memberB.grant.AgentID, ExpectedTargetEpoch: 1, ExpectedConfigVersion: 1,
		Identity: CollaborationResourceIdentity{Kind: "managed", InstallationID: memberB.installation.ID, BindingID: memberB.binding.ID, BindingEpoch: 1}}); !collaborationErrorCode(err, "forbidden") || before != controlStateSnapshot(t, f.collaborationQueueFixture) {
		t.Fatalf("uninvited resource owner acquired per-issue consent: %v", err)
	}
	if _, err = f.svc.InviteParticipant(f.ctx, CollaborationParticipantParams{CollaborationWorkflowRequest: workflow(f.ctx), TargetUserID: util.MustParseUUID(f.outsider)}); err != nil {
		t.Fatal(err)
	}
	registration.RequestID = uuid.NewString()
	if _, err = f.svc.RegisterTarget(outsiderCtx, registration); !collaborationErrorCode(err, "forbidden") {
		t.Fatalf("full ordinary participant acquired organization rights: %v", err)
	}
	handlerCtx := collaborationPasswordContext(util.MustParseUUID(f.handler), 1)
	registration.RequestID = uuid.NewString()
	registered, err := f.svc.RegisterTarget(handlerCtx, registration)
	if err != nil || registered.Code != "target_registered" {
		t.Fatalf("current handler could not organize the exact three-owner squad: %v", err)
	}
	squad := queueTarget(t, f.collaborationQueueFixture, registered.TargetID)
	for _, resource := range []*managedCollaborationFixture{leader, memberA, memberB} {
		resource.targetID = squad.ID
	}
	if len(mentions(handlerCtx)) != 1 || f.fx.Count(t, `SELECT count(*) FROM collaboration_grant WHERE target_id=$1`, squad.ID) != 0 {
		t.Fatal("registration granted consent or admitted an incomplete squad")
	}
	before = controlStateSnapshot(t, f.collaborationQueueFixture)
	if _, err = f.svc.SubmitCommand(f.ctx, CollaborationSubmitCommandParams{CollaborationQueueRequest: f.request(), TargetID: squad.ID,
		InputEventID: util.MustParseUUID(uuid.NewString()), ExpectedTargetEpoch: squad.TargetEpoch, ExpectedConfigVersion: squad.ConfigVersion,
		Content: "Incomplete owner consent must reject an old explicit selection"}); !collaborationErrorCode(err, "grant_required") || before != controlStateSnapshot(t, f.collaborationQueueFixture) {
		t.Fatalf("an incomplete squad accepted input outside the authorized catalogue: %v", err)
	}
	leader.grant = ownerSwitchConsent(t, leader, squad)
	if len(mentions(f.ctx)) != 1 {
		t.Fatal("leader consent authorized the two other actual resource owners")
	}
	wrongOwner := CollaborationConsentParams{CollaborationQueueRequest: f.request(), TargetID: squad.ID, AgentID: memberA.grant.AgentID,
		ExpectedTargetEpoch: squad.TargetEpoch, ExpectedConfigVersion: squad.ConfigVersion,
		Identity: CollaborationResourceIdentity{Kind: "managed", InstallationID: memberA.installation.ID, BindingID: memberA.binding.ID, BindingEpoch: 1}}
	before = controlStateSnapshot(t, f.collaborationQueueFixture)
	if _, err = f.svc.ConsentTargetResource(f.ctx, wrongOwner); !collaborationErrorCode(err, "forbidden") || before != controlStateSnapshot(t, f.collaborationQueueFixture) {
		t.Fatalf("organizer signed another owner's grant: %v", err)
	}
	wrongBinding := wrongOwner
	wrongBinding.RequestID = uuid.NewString()
	wrongBinding.Identity.BindingID = leader.binding.ID
	if _, err = f.svc.ConsentTargetResource(handlerCtx, wrongBinding); !collaborationErrorCode(err, "runtime_registration_required") || before != controlStateSnapshot(t, f.collaborationQueueFixture) {
		t.Fatalf("actual member owner borrowed the leader's environment binding: %v", err)
	}
	memberA.grant = ownerSwitchConsent(t, memberA, squad)
	if len(mentions(outsiderCtx)) != 1 {
		t.Fatal("two owner confirmations admitted an unconsented third member")
	}
	memberB.grant = ownerSwitchConsent(t, memberB, squad)
	items := mentions(outsiderCtx)
	if len(items) != 2 || items[squad.ID].TargetID != squad.ID || items[squad.ID].Kind != collaboration.TargetSquad || items[squad.ID].WaitingReason != "issue_paused" {
		t.Fatal("complete owner consent failed to admit only the held typed squad")
	}
	memberDirect := ownerSwitchRegister(t, memberA, collaboration.TargetAgent, memberA.grant.AgentID, []collaboration.TargetResource{ownerSwitchResource(memberA, "agent")})
	memberDirectGrant := ownerSwitchConsent(t, memberA, memberDirect)
	memberA.targetID = squad.ID
	if _, err = f.svc.Transfer(f.ctx, CollaborationTransferParams{CollaborationParticipantParams: CollaborationParticipantParams{
		CollaborationWorkflowRequest: workflow(f.ctx), TargetUserID: util.MustParseUUID(f.creator)}, Note: "Creator takes current responsibility"}); err != nil {
		t.Fatal(err)
	}
	registration.RequestID = uuid.NewString()
	if _, err = f.svc.RegisterTarget(handlerCtx, registration); !collaborationErrorCode(err, "forbidden") {
		t.Fatalf("former handler retained organization rights after transfer: %v", err)
	}

	newcomer := f.fx.User(t, "New catalogue participant", "catalogue-new-"+uuid.NewString()+"@example.test")
	f.fx.InsertNoID(t, "user_password_credential", testutil.Cols{"user_id": newcomer, "username": "cataloguenew" + uuid.NewString()[:8],
		"password_hash": "fixture-not-used-for-login", "must_change_password": false}, "user_id=$1", newcomer)
	f.fx.Cleanup(t, `DELETE FROM collaboration_account_fence WHERE user_id=$1`, newcomer)
	newCtx := collaborationPasswordContext(util.MustParseUUID(newcomer), 1)
	grantsBefore, err := f.svc.Queries.ListCollaborationTargetGrants(f.ctx, squad.ID)
	if err != nil || len(grantsBefore) != 3 {
		t.Fatalf("three original owner grants missing: %v", err)
	}
	if _, err = f.svc.InviteParticipant(f.ctx, CollaborationParticipantParams{CollaborationWorkflowRequest: workflow(f.ctx), TargetUserID: util.MustParseUUID(newcomer)}); err != nil {
		t.Fatal(err)
	}
	grantsAfter, err := f.svc.Queries.ListCollaborationTargetGrants(newCtx, squad.ID)
	if err != nil || !reflect.DeepEqual(grantsBefore, grantsAfter) || len(mentions(newCtx)) != 3 || mentions(newCtx)[squad.ID].WaitingReason != "issue_paused" ||
		f.fx.Count(t, `SELECT count(*) FROM member WHERE workspace_id=$1 AND user_id=$2`, f.workspace, newcomer) != 0 {
		t.Fatalf("new participant required new owner consent or acquired source membership: %v", err)
	}
	submit := func(body string) CollaborationQueueReceipt {
		t.Helper()
		return submitManagedControlInput(t, f, queueTarget(t, f.collaborationQueueFixture, squad.ID), newcomer, body)
	}
	inputs := []CollaborationQueueReceipt{submit("Original new-participant input while the issue is paused")}
	if inputs[0].Code != "command_queued" || inputs[0].QueueState != "paused" || f.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE session_id=$1`, f.issue.SessionID) != 0 {
		t.Fatal("new participant's valid paused selection launched work")
	}
	f.resumeManaged(t)
	f.fx.Exec(t, `UPDATE agent_runtime SET status='offline' WHERE id=$1`, leader.runtime.ID)
	if mentions(newCtx)[squad.ID].WaitingReason != "runtime_offline" {
		t.Fatal("offline valid squad disappeared or reported the wrong waiting reason")
	}
	inputs = append(inputs, submit("Wait for the actual leader runtime to reconnect"))
	assertNoLaunch := func(stage string) {
		t.Helper()
		before := controlStateSnapshot(t, f.collaborationQueueFixture)
		list, err := f.svc.ListRuntimeRuns(leader.daemonCtx, leader.runtime.ID)
		if err != nil || len(list.RunIDs) != 0 || before != controlStateSnapshot(t, f.collaborationQueueFixture) {
			t.Fatalf("valid %s selection consumed input or changed authority instead of waiting: %v", stage, err)
		}
	}
	assertNoLaunch("offline")
	f.fx.Exec(t, `UPDATE agent_runtime SET status='online' WHERE id=$1`, leader.runtime.ID)
	busy := f.fx.Task(t, util.UUIDToString(leader.grant.AgentID), testutil.Cols{"runtime_id": leader.runtime.ID, "status": "running"})
	if item, ok := mentions(newCtx)[squad.ID]; !ok || item.ExecutionAvailable {
		t.Fatal("busy actual namespace removed the valid squad or claimed immediate execution")
	}
	inputs = append(inputs, submit("Wait behind actual ordinary runtime occupancy"))
	assertNoLaunch("busy")
	f.fx.Exec(t, `UPDATE agent_task_queue SET status='completed' WHERE id=$1`, busy)
	if _, err = f.svc.StopQueue(newCtx, managedTargetResume(t, f, squad.ID).CollaborationStopQueueParams); err != nil {
		t.Fatal(err)
	}
	if mentions(newCtx)[squad.ID].WaitingReason != "target_paused" {
		t.Fatal("paused valid channel lost catalogue selection or its precise reason")
	}
	inputs = append(inputs, submit("Wait for a fresh explicit target Resume"))
	assertNoLaunch("target_paused")
	if _, err = f.svc.ResumeQueue(newCtx, managedTargetResume(t, f, squad.ID)); err != nil {
		t.Fatal(err)
	}
	root, request, rootClaim := leader.claimManaged(t)
	if len(rootClaim.Inputs) != len(inputs) || rootClaim.ResourceOwnerID != f.creator || rootClaim.AgentID != util.UUIDToString(leader.grant.AgentID) {
		t.Fatal("root claim changed the actual owner or lost a temporarily waiting input")
	}
	for i, input := range rootClaim.Inputs {
		if input.CommandID != util.UUIDToString(inputs[i].CommandID) || input.AuthorUserID != newcomer ||
			f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=$1 AND seq=$2 AND author_user_id=$3 AND state='in_flight'`, inputs[i].CommandID, inputs[i].CommandSequence, newcomer) != 1 {
			t.Fatal("temporary waiting reordered input or substituted the new participant's identity")
		}
	}
	rootRun, err := f.svc.Queries.GetCollaborationRuntimeRun(f.ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	dependencies, err := f.svc.Queries.ListCollaborationGroupDependencies(f.ctx, rootRun.GroupID)
	if err != nil || len(dependencies) != len(inputs) {
		t.Fatalf("every consumed input requires its own complete resource provenance: %v", err)
	}
	expectedGrants := make(map[string]collaborationFrozenGrant)
	for _, grant := range []db.CollaborationGrant{leader.grant, memberA.grant, memberB.grant} {
		expectedGrants[util.UUIDToString(grant.ID)] = freezeCollaborationGrant(grant)
	}
	for _, dependency := range dependencies {
		frozen, err := decodeCollaborationRoster[collaborationFrozenGrant](dependency.Grants)
		if err != nil || len(frozen) != 3 {
			t.Fatalf("input dependency omitted a real owner: %v", err)
		}
		for _, grant := range frozen {
			if expected, ok := expectedGrants[grant.GrantID]; !ok || !reflect.DeepEqual(grant, expected) {
				t.Fatal("dependency did not freeze the actual per-issue owner/configuration/binding grant")
			}
		}
	}
	rootToken, rootNonce := permitSquadRun(t, leader, root, rootClaim)
	if mentions(newCtx)[squad.ID].WaitingReason != "target_busy" {
		t.Fatal("live valid squad disappeared from the catalogue")
	}
	before = controlStateSnapshot(t, f.collaborationQueueFixture)
	live, err := f.svc.ListRuntimeRuns(leader.daemonCtx, leader.runtime.ID)
	if err != nil || !reflect.DeepEqual(live.RunIDs, []string{util.UUIDToString(root)}) || before != controlStateSnapshot(t, f.collaborationQueueFixture) ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE session_id=$1`, f.issue.SessionID) != 1 {
		t.Fatalf("busy catalogue/polling duplicated the original live root: %v", err)
	}
	replay, err := f.svc.ClaimRuntimeRun(leader.daemonCtx, root, request)
	if err != nil || !reflect.DeepEqual(replay.Inputs, rootClaim.Inputs) {
		t.Fatalf("temporary-wait root replay changed original inputs: %v", err)
	}
	dispatch, err := f.svc.CreateRuntimeComment(context.Background(), root, rootToken, CollaborationRuntimeCommentParams{
		CommentID: util.MustParseUUID(uuid.NewString()), Content: "[@A](mention://agent/" + util.UUIDToString(memberA.grant.AgentID) + ") [@B](mention://agent/" + util.UUIDToString(memberB.grant.AgentID) + ") [@outside-roster](mention://agent/" + util.UUIDToString(f.grant.AgentID) + ") review"})
	if err != nil || len(dispatch.RunIDs) != 2 || dispatch.ContinuationRunID.Valid {
		t.Fatalf("leader dispatch escaped its complete frozen member roster: %v", err)
	}
	type memberExecution struct {
		fixture *managedCollaborationFixture
		run     pgtype.UUID
		request protocol.CollaborationRunClaimRequest
		claim   protocol.CollaborationRunClaimResponse
		token   string
		nonce   string
	}
	derived := make([]memberExecution, 0, 2)
	for _, member := range []*managedCollaborationFixture{memberA, memberB} {
		runID, claimRequest, claim := member.claimManaged(t)
		run, err := f.svc.Queries.GetCollaborationRuntimeRun(f.ctx, runID)
		if err != nil || !slices.Contains(dispatch.RunIDs, runID) || run.ParentRunID != root || run.GroupID != rootRun.GroupID || run.CommandID != rootRun.CommandID ||
			run.GrantID != member.grant.ID || run.GrantEpoch != member.grant.Epoch || run.RuntimeID != member.runtime.ID || run.BindingID != member.binding.ID ||
			run.BindingEpoch != member.binding.BindingEpoch || run.InstallationID != member.installation.ID || run.ResourceOwnerUserID != member.runtime.OwnerID ||
			run.StepKind.String != "member" || claim.ResourceOwnerID != util.UUIDToString(member.grant.OwnerUserID) || !reflect.DeepEqual(claim.Inputs, rootClaim.Inputs) {
			t.Fatalf("derived run substituted its leader for actual grant/owner/runtime/prompt provenance: %v", err)
		}
		before = controlStateSnapshot(t, f.collaborationQueueFixture)
		if _, err = f.svc.ClaimRuntimeRun(leader.daemonCtx, runID, claimRequest); err == nil || before != controlStateSnapshot(t, f.collaborationQueueFixture) {
			t.Fatalf("leader daemon credential acquired another owner's member run: %v", err)
		}
		if _, err = f.svc.RuntimeContext(context.Background(), runID, rootToken); !collaborationErrorCode(err, "business_capability_required") || before != controlStateSnapshot(t, f.collaborationQueueFixture) {
			t.Fatalf("leader business credential expanded member rights: %v", err)
		}
		token, nonce := permitSquadRun(t, member, runID, claim)
		own, err := f.svc.RuntimeContext(context.Background(), runID, token)
		if err != nil || own.AgentID != member.grant.AgentID || own.Identity != claim.Identity {
			t.Fatalf("actual member could not use only its own run capability: %v", err)
		}
		derived = append(derived, memberExecution{fixture: member, run: runID, request: claimRequest, claim: claim, token: token, nonce: nonce})
	}

	oldSelection := CollaborationSubmitCommandParams{CollaborationQueueRequest: f.request(), TargetID: squad.ID,
		InputEventID: util.MustParseUUID(uuid.NewString()), ExpectedTargetEpoch: squad.TargetEpoch, ExpectedConfigVersion: squad.ConfigVersion, Content: "Old selected squad must be refused"}
	memberAgent, err := f.svc.Queries.GetAgent(f.ctx, memberB.grant.AgentID)
	if err != nil {
		t.Fatal(err)
	}
	f.fx.Exec(t, `UPDATE agent SET instructions='Changed member execution configuration' WHERE id=$1`, memberB.grant.AgentID)
	if _, visible := mentions(newCtx)[squad.ID]; visible {
		t.Fatal("a changed member configuration left the squad selectable")
	}
	before = controlStateSnapshot(t, f.collaborationQueueFixture)
	if _, err = f.svc.SubmitCommand(newCtx, oldSelection); !collaborationErrorCode(err, "target_configuration_changed") || before != controlStateSnapshot(t, f.collaborationQueueFixture) {
		t.Fatalf("old selection bypassed current member configuration: %v", err)
	}
	for _, current := range derived {
		if _, err = f.svc.ClaimRuntimeRun(current.fixture.daemonCtx, current.run, current.request); err == nil || before != controlStateSnapshot(t, f.collaborationQueueFixture) {
			t.Fatalf("derived claim replay skipped a changed resource owner's configuration: %v", err)
		}
		if _, err = f.svc.PermitRuntimeRun(current.fixture.daemonCtx, current.run, protocol.CollaborationRunPermitRequest{
			Identity: current.claim.Identity, AttemptSeq: "1", AttemptNonce: current.nonce, Containment: protocol.CollaborationContainmentLinuxPIDNamespace}); err == nil || before != controlStateSnapshot(t, f.collaborationQueueFixture) {
			t.Fatalf("old member permit bypassed the complete frozen-resource recheck: %v", err)
		}
		if _, err = f.svc.RuntimeContext(context.Background(), current.run, current.token); err == nil || before != controlStateSnapshot(t, f.collaborationQueueFixture) {
			t.Fatalf("old member business token retained rights after configuration drift: %v", err)
		}
	}
	if _, err = f.svc.RuntimeContext(context.Background(), root, rootToken); err == nil {
		t.Fatal("leader business token ignored another member's configuration change")
	}
	f.fx.Exec(t, `UPDATE agent SET instructions=$2 WHERE id=$1`, memberB.grant.AgentID, memberAgent.Instructions)
	if _, visible := mentions(newCtx)[squad.ID]; visible {
		t.Fatal("restoring the old configuration value revived its obsolete consent")
	}
	f.fx.Exec(t, `UPDATE installation_daemon_binding SET binding_epoch=binding_epoch+1 WHERE id=$1`, memberB.binding.ID)
	if _, err = f.svc.ListRuntimeRuns(memberB.daemonCtx, memberB.runtime.ID); !collaborationErrorCode(err, "managed_runtime_required") {
		t.Fatalf("old member binding credential survived its epoch change: %v", err)
	}
	if _, err = f.svc.RevokeGrant(handlerCtx, CollaborationRevokeGrantParams{CollaborationQueueRequest: f.request(), GrantID: memberA.grant.ID, ExpectedGrantEpoch: memberA.grant.Epoch}); err != nil {
		t.Fatal(err)
	}
	for _, id := range []pgtype.UUID{root, derived[0].run, derived[1].run} {
		if f.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE id=$1 AND forced_disposition='stopped' AND settled_at IS NULL AND proof_state='unknown'`, id) != 1 ||
			f.fx.Count(t, `SELECT count(*) FROM collaboration_runtime_reservation WHERE run_id=$1 AND released_at IS NULL`, id) != 1 {
			t.Fatal("member withdrawal released another potentially live namespace or invented proof")
		}
	}
	if _, err = f.svc.Leave(handlerCtx, workflow(handlerCtx)); err != nil {
		t.Fatal(err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_grant WHERE issue_id=$1 AND owner_user_id=$2`, f.issue.Issue.ID, f.handler) != 2 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_grant WHERE issue_id=$1 AND owner_user_id=$2 AND revoked_at IS NULL`, f.issue.Issue.ID, f.handler) != 0 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_grant WHERE id=$1 AND revoked_at IS NOT NULL AND epoch=$2`, memberDirectGrant.ID, memberDirectGrant.Epoch+1) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM issue_participant WHERE issue_id=$1 AND user_id=$2 AND revoked_at IS NOT NULL`, f.issue.Issue.ID, f.handler) != 1 {
		t.Fatal("owner departure failed to retract every direct and squad resource grant")
	}
	items = mentions(newCtx)
	if len(items) != 1 || items[direct.ID].TargetID != direct.ID {
		t.Fatal("stale squad or departed owner's direct target remained selectable")
	}
	registration.RequestID = uuid.NewString()
	if _, err = f.svc.RegisterTarget(handlerCtx, registration); !collaborationErrorCode(err, "forbidden") {
		t.Fatalf("source-only former participant acquired organization rights: %v", err)
	}
	oldSelection.RequestID = uuid.NewString()
	if _, err = f.svc.SubmitCommand(newCtx, oldSelection); err == nil {
		t.Fatal("resource withdrawal/departure left an old selection executable")
	}
	if _, err = f.svc.StopQueue(newCtx, f.issueControl(t)); err != nil {
		t.Fatalf("current new participant lost legitimate Stop when resource grants changed: %v", err)
	}
	for _, current := range derived {
		if _, err = f.svc.RuntimeContext(context.Background(), current.run, current.token); err == nil {
			t.Fatal("withdrawn derived execution retained business authority")
		}
		page, err := f.svc.RunRecords(newCtx, f.issue.Issue.ID, current.run, CollaborationRunRecordsQuery{Limit: 100})
		if err != nil || page.Resource.GrantID != util.UUIDToString(current.fixture.grant.ID) || page.Resource.ResourceOwnerUserID != util.UUIDToString(current.fixture.grant.OwnerUserID) ||
			page.Run.ID != util.UUIDToString(current.run) || page.Run.ProofState != "unknown" {
			t.Fatalf("current participant lost readable original member provenance after withdrawal: %v", err)
		}
		if _, err = f.svc.AcceptRuntimeProof(context.Background(), current.run, current.claim.TerminationToken, managedProof(current.claim.Identity, current.nonce)); err != nil {
			t.Fatalf("old member's exact termination authority was stranded by withdrawal/configuration: %v", err)
		}
	}
	if _, err = f.svc.AcceptRuntimeProof(context.Background(), root, rootClaim.TerminationToken, managedProof(rootClaim.Identity, rootNonce)); err != nil {
		t.Fatal(err)
	}
	access, err := f.svc.ResolveIssueAccess(newCtx, nil, f.issue.Issue.ID)
	if err != nil {
		t.Fatal(err)
	}
	history, err := f.svc.ExecutionView(newCtx, access)
	if err != nil || len(history.Commands) != len(inputs) || f.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE group_id=$1 AND settled_at IS NULL`, rootRun.GroupID) != 0 {
		t.Fatalf("original command history or complete proof settlement disappeared: %v", err)
	}
	stored, err := f.svc.Queries.ListCollaborationBatchInputs(newCtx, rootRun.BatchID)
	if err != nil || len(stored) != len(inputs) {
		t.Fatalf("original batch input history missing: %v", err)
	}
	rootRecords, err := f.svc.RunRecords(newCtx, f.issue.Issue.ID, root, CollaborationRunRecordsQuery{Limit: 100})
	if err != nil || !rootRecords.Recording.InputsRecorded {
		t.Fatalf("current participant lost the original consumed-input record: %v", err)
	}
	for i, input := range inputs {
		index := slices.IndexFunc(history.Commands, func(item CollaborationExecutionCommandView) bool {
			return item.ID == util.UUIDToString(input.CommandID)
		})
		if index < 0 {
			t.Fatal("readable history omitted an original temporarily waiting input")
		}
		item := history.Commands[index]
		if item.AuthorUserID != newcomer || item.TargetID != util.UUIDToString(squad.ID) || item.ChannelID != util.UUIDToString(input.ChannelID) ||
			item.Sequence != strconv.FormatInt(input.CommandSequence, 10) || item.State != "stopped" ||
			stored[i].ID != input.CommandID || stored[i].Content != rootClaim.Inputs[i].Body || stored[i].Seq != input.CommandSequence || stored[i].State != "stopped" {
			t.Fatal("readable history rewrote an original author, typed target, sequence or body after withdrawal")
		}
		record := slices.IndexFunc(rootRecords.Items, func(item CollaborationRunRecordItem) bool {
			return item.Kind == "input" && item.CommandID == util.UUIDToString(input.CommandID)
		})
		if record < 0 || rootRecords.Items[record].Body != rootClaim.Inputs[i].Body || rootRecords.Items[record].AuthorUserID != newcomer {
			t.Fatal("public records omitted or rewrote an original consumed input after withdrawal")
		}
	}

	child := f.fx.Issue(t, "Ordinary catalogue child", testutil.Cols{"parent_issue_id": f.issue.Issue.ID, "description": "ordinary-child-only-body"})
	anchor := f.fx.Comment(t, child, "ordinary-child-only-comment")
	f.fx.Cleanup(t, `DELETE FROM agent_task_queue WHERE issue_id=$1`, child)
	build, err := BuildSourceContext(context.Background(), f.svc.Queries, util.MustParseUUID(f.workspace), util.MustParseUUID(anchor))
	encoded, encodeErr := json.Marshal(build.Snapshot)
	if err != nil || encodeErr != nil || build.Snapshot.SourceIssue.ID != child || !strings.Contains(string(encoded), "ordinary-child-only-body") ||
		strings.Contains(string(encoded), f.issue.Issue.Description.String) || strings.Contains(string(encoded), util.UUIDToString(squadID)) {
		t.Fatalf("ordinary child source capture inherited private shared-parent content: %v", err)
	}
	if _, err = f.svc.Queries.CreateAgentTask(context.Background(), db.CreateAgentTaskParams{IssueID: util.MustParseUUID(child),
		AgentID: f.grant.AgentID, RuntimeID: f.runtime.ID, TriggerCommentID: dispatch.CommentID}); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("ordinary child execution inherited a collaboration comment from its parent: %v", err)
	}
	for _, principal := range []auth.AuthenticatedPrincipal{
		{UserID: f.creator, Kind: auth.CredentialTask},
		{UserID: f.creator, Kind: auth.CredentialCloud},
		{UserID: f.creator, Kind: auth.CredentialPAT, AgentActor: true},
	} {
		machine := auth.WithAuthenticatedPrincipal(f.ctx, principal)
		p, _ := collaboration.PrincipalFromContext(machine)
		if caps := collaboration.CapabilitiesFor(p, collaboration.AccessState{AccountActive: true, SourceMember: true, Participant: true, Creator: true, StatusCategory: "todo", QueueState: "paused"}); caps != (collaboration.Capabilities{Reference: true}) {
			t.Fatal("machine source/member identity acquired more than minimal reference authority")
		}
		if _, err = f.svc.ResolveIssueAccess(machine, nil, f.issue.Issue.ID); !collaborationErrorCode(err, "not_found") {
			t.Fatalf("unauthorized machine read shared-parent content: %v", err)
		}
		if _, err = f.svc.RunRecords(machine, f.issue.Issue.ID, root, CollaborationRunRecordsQuery{Limit: 100}); !collaborationErrorCode(err, "not_found") {
			t.Fatalf("unauthorized machine read shared command history: %v", err)
		}
		if _, err = f.svc.StopQueue(machine, f.issueControl(t)); !collaborationErrorCode(err, "forbidden") {
			t.Fatalf("machine reference authority inherited a human control entry: %v", err)
		}
	}
	t.Log("AC96 service matrix: current organization, complete three-owner consent, new participant reuse, paused/offline/busy waiting, derived provenance/recheck, owner-wide withdrawal, history/Stop/proof and ordinary/machine boundaries verified")
}
