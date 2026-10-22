package service

import (
	"context"
	"reflect"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestCollaborationPublicSquadSwitchWaitsBehindBusyDestinationAndFreshResume(t *testing.T) {
	f := newManagedCollaborationFixture(t)
	switchCleanup(t, f.collaborationQueueFixture)
	squadID := util.MustParseUUID(f.fx.Squad(t, "Public switch original squad", util.UUIDToString(f.grant.AgentID)))
	old := *f
	oldQueue := *f.collaborationQueueFixture
	old.collaborationQueueFixture = &oldQueue
	squad := ownerSwitchRegister(t, &old, collaboration.TargetSquad, squadID, []collaboration.TargetResource{ownerSwitchResource(f, "leader")})
	old.grant = ownerSwitchConsent(t, &old, squad)
	cancelled := submitManagedControlInput(t, &old, squad, f.creator, "Original typed S cancelled input")
	if _, err := f.svc.CancelWaitingCommand(f.ctx, CollaborationCancelCommandParams{CollaborationQueueRequest: f.request(), TargetID: squad.ID,
		CommandID: cancelled.CommandID, ExpectedCollaborationRevision: 1}); err != nil {
		t.Fatal(err)
	}
	f.resumeManaged(t)
	stoppedInput := submitManagedControlInput(t, &old, queueTarget(t, old.collaborationQueueFixture, squad.ID), f.creator, "Original typed S stopped input")
	stopped := ownerSwitchClaimPermit(t, &old, stoppedInput)
	if _, err := f.svc.StopQueue(f.ctx, managedTargetResume(t, &old, squad.ID).CollaborationStopQueueParams); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.AcceptRuntimeProof(context.Background(), stopped.run, stopped.claim.TerminationToken, managedProof(stopped.claim.Identity, stopped.permit.AttemptNonce)); err != nil {
		t.Fatal(err)
	}
	first := submitManagedControlInput(t, &old, queueTarget(t, old.collaborationQueueFixture, squad.ID), f.creator, "Original typed S retained first")
	second := submitManagedControlInput(t, &old, queueTarget(t, old.collaborationQueueFixture, squad.ID), f.handler, "Original typed S retained second")
	third := submitManagedControlInput(t, &old, queueTarget(t, old.collaborationQueueFixture, squad.ID), f.handler, "Original typed S single cancellation after Switch")
	destination := ownerSwitchNamespace(t, f, f.creator, "Busy new leader")
	direct := ownerSwitchRegister(t, destination, collaboration.TargetAgent, destination.grant.AgentID, []collaboration.TargetResource{ownerSwitchResource(destination, "agent")})
	destination.grant = ownerSwitchConsent(t, destination, direct)
	if _, err := f.svc.ResumeQueue(f.ctx, managedTargetResume(t, destination, direct.ID)); err != nil {
		t.Fatal(err)
	}
	busyInput := submitManagedControlInput(t, destination, queueTarget(t, destination.collaborationQueueFixture, direct.ID), f.creator, "Destination direct target current work")
	busy := ownerSwitchClaimPermit(t, destination, busyInput)
	head := submitManagedControlInput(t, destination, queueTarget(t, destination.collaborationQueueFixture, direct.ID), f.creator, "Destination direct target future head")
	beforeDestination := queueTarget(t, destination.collaborationQueueFixture, direct.ID)
	oldChannel, err := f.svc.Queries.GetCollaborationChannel(f.ctx, squad.ChannelID)
	if err != nil {
		t.Fatal(err)
	}
	oldInputs, err := f.svc.Queries.ListCollaborationGroupInputs(f.ctx, util.MustParseUUID(stopped.claim.Identity.GroupID))
	if err != nil {
		t.Fatal(err)
	}
	oldCommands, err := f.svc.Queries.ListCollaborationCommands(f.ctx, f.issue.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	retainedTerminal := []db.CollaborationCommand{}
	for _, input := range oldCommands {
		if input.ID == cancelled.CommandID || input.ID == stoppedInput.CommandID {
			retainedTerminal = append(retainedTerminal, input)
		}
	}
	var oldHistory string
	f.fx.QueryRow(t, `SELECT jsonb_build_object(
		'group',(SELECT to_jsonb(g) FROM collaboration_execution_group g WHERE id=$1),
		'runs',(SELECT jsonb_agg(to_jsonb(r) ORDER BY r.id) FROM collaboration_run r WHERE group_id=$1),
		'inputs',(SELECT jsonb_agg(to_jsonb(i) ORDER BY i.command_id) FROM collaboration_group_input i WHERE group_id=$1),
		'dependencies',(SELECT jsonb_agg(to_jsonb(d) ORDER BY d.command_id) FROM collaboration_group_dependency d WHERE group_id=$1),
		'proofs',(SELECT jsonb_agg(to_jsonb(p) ORDER BY p.run_id) FROM collaboration_termination p WHERE run_id IN (SELECT id FROM collaboration_run WHERE group_id=$1)),
		'attempts',(SELECT jsonb_agg(to_jsonb(a) ORDER BY a.run_id,a.attempt_seq) FROM collaboration_attempt a WHERE run_id IN (SELECT id FROM collaboration_run WHERE group_id=$1))
	)::text`, stopped.claim.Identity.GroupID).Scan(&oldHistory)
	// The resource writer changes the actual squad configuration. Public staged
	// Switch must separately capture and consent that new configuration; it must
	// never use the test-only ReconfigureTarget wrapper as a replacement.
	if _, err := f.svc.Queries.UpdateSquad(f.ctx, db.UpdateSquadParams{ID: squadID, LeaderID: destination.grant.AgentID,
		ExpectedWorkspaceID: util.MustParseUUID(f.workspace)}); err != nil {
		t.Fatal(err)
	}
	beforeSwitch := queueTarget(t, old.collaborationQueueFixture, squad.ID)
	stagedSnapshot := ownerSwitchTargetSnapshot(t, f, squad.ID)
	proposalParams := CollaborationProposeSwitchParams{CollaborationQueueRequest: f.request(), TargetID: squad.ID,
		ExpectedTargetEpoch: beforeSwitch.TargetEpoch, ExpectedConfigVersion: beforeSwitch.ConfigVersion,
		Roster: []collaboration.TargetResource{ownerSwitchResource(destination, "leader")}}
	proposal, err := f.svc.ProposeTargetSwitch(f.ctx, proposalParams)
	if err != nil || proposal.Code != "switch_proposed" || !proposal.ProposalID.Valid {
		t.Fatalf("public busy-destination Switch proposal: %+v %v", proposal, err)
	}
	consent := CollaborationSwitchConsentParams{CollaborationQueueRequest: f.request(), TargetID: squad.ID, ProposalID: proposal.ProposalID,
		AgentID: destination.grant.AgentID, Identity: collaborationGrantIdentity(destination.grant)}
	if _, err := f.svc.ConsentTargetSwitchResource(collaborationPasswordContext(util.MustParseUUID(f.handler), 1), consent); !collaborationErrorCode(err, "forbidden") {
		t.Fatalf("organizer substituted for replacement resource owner: %v", err)
	}
	consent.RequestID = uuid.NewString()
	if _, err := f.svc.ConsentTargetSwitchResource(f.ctx, consent); err != nil {
		t.Fatal(err)
	}
	if stagedSnapshot != ownerSwitchTargetSnapshot(t, f, squad.ID) {
		t.Fatal("public proposal/consent changed live target, waiting input or retained execution")
	}
	commit := CollaborationCommitSwitchParams{CollaborationQueueRequest: f.request(), ProposalID: proposal.ProposalID, Control: targetControl(beforeSwitch)}
	switched, err := f.svc.CommitTargetSwitch(f.ctx, commit)
	if err != nil || switched.Code != "target_switched" {
		t.Fatalf("public Switch to busy new leader: %+v %v", switched, err)
	}
	current := queueTarget(t, old.collaborationQueueFixture, squad.ID)
	if current.ID != squad.ID || current.ResourceID != squadID || current.Kind != "squad" || current.ChannelID != head.ChannelID ||
		current.HoldReason.String != "switch_hold" || current.ConfigVersion != beforeSwitch.ConfigVersion+1 || current.TargetEpoch != beforeSwitch.TargetEpoch+1 ||
		current.ActiveGroupID != busyInput.GroupID && current.ActiveGroupID != util.MustParseUUID(busy.claim.Identity.GroupID) {
		t.Fatalf("public Switch changed typed target or lost independent hold/destination current group: %+v", current)
	}
	for i, input := range []CollaborationQueueReceipt{first, second, third} {
		if f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=$1 AND target_id=$2 AND channel_id=$3 AND state='waiting'
			AND group_id IS NULL AND seq=$4`, input.CommandID, squad.ID, head.ChannelID, head.CommandSequence+int64(i)+1) != 1 {
			t.Fatal("public Switch failed to append typed waiting inputs in original relative order")
		}
	}
	if !reflect.DeepEqual(beforeDestination, queueTarget(t, destination.collaborationQueueFixture, direct.ID)) ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE id=$1 AND forced_disposition IS NULL AND settled_at IS NULL AND proof_state='unknown'`, busy.run) != 1 {
		t.Fatal("public Switch stopped/held/replaced unrelated destination target work")
	}
	if _, err := f.svc.CancelWaitingCommand(f.ctx, CollaborationCancelCommandParams{CollaborationQueueRequest: f.request(), TargetID: squad.ID,
		CommandID: third.CommandID, ExpectedCollaborationRevision: 1}); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.RecordRuntimeOutcome(context.Background(), busy.run, busy.permit.BusinessToken, recordedOutcome(busy.claim, busy.permit, 1)); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.AcceptRuntimeProof(context.Background(), busy.run, busy.claim.TerminationToken, managedProof(busy.claim.Identity, busy.permit.AttemptNonce)); err != nil {
		t.Fatal(err)
	}
	// While S remains held, the destination's earlier direct input must start
	// normally. Switching S cannot block other targets on the new channel.
	next := ownerSwitchClaimPermit(t, destination, head)
	if len(next.claim.Inputs) != 1 || next.claim.Inputs[0].CommandID != util.UUIDToString(head.CommandID) ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=ANY($1::uuid[]) AND state='waiting'`, []pgtype.UUID{first.CommandID, second.CommandID}) != 2 {
		t.Fatal("held switched squad blocked direct head or executed before its explicit Resume")
	}
	if err := f.svc.RecordRuntimeOutcome(context.Background(), next.run, next.permit.BusinessToken, recordedOutcome(next.claim, next.permit, 1)); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.AcceptRuntimeProof(context.Background(), next.run, next.claim.TerminationToken, managedProof(next.claim.Identity, next.permit.AttemptNonce)); err != nil {
		t.Fatal(err)
	}
	if list, err := f.svc.ListRuntimeRuns(destination.daemonCtx, destination.runtime.ID); err != nil || len(list.RunIDs) != 0 || queueTarget(t, old.collaborationQueueFixture, squad.ID).HoldReason.String != "switch_hold" {
		t.Fatalf("proof/poll automatically resumed switched S: %+v %v", list, err)
	}
	squadExecution := *destination
	squadQueue := *destination.collaborationQueueFixture
	squadExecution.collaborationQueueFixture = &squadQueue
	squadExecution.targetID = squad.ID
	if _, err := f.svc.ResumeQueue(f.ctx, managedTargetResume(t, &squadExecution, squad.ID)); err != nil {
		t.Fatal(err)
	}
	runID, _, claim := squadExecution.claimManaged(t)
	if len(claim.Inputs) != 2 || claim.Inputs[0].CommandID != util.UUIDToString(first.CommandID) || claim.Inputs[1].CommandID != util.UUIDToString(second.CommandID) ||
		claim.Identity.TargetID != util.UUIDToString(squad.ID) || claim.Identity.ChannelID != util.UUIDToString(head.ChannelID) || claim.AgentID != util.UUIDToString(destination.grant.AgentID) ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_execution_group WHERE id=$1 AND config_version=$2 AND target_id=$3 AND channel_id=$4`,
			claim.Identity.GroupID, current.ConfigVersion, squad.ID, head.ChannelID) != 1 || claim.Identity.RunID == stopped.claim.Identity.RunID {
		t.Fatal("fresh resumed S did not continue exact original typed inputs in the replacement environment")
	}
	permitToken, permitNonce := permitSquadRun(t, &squadExecution, runID, claim)
	finishSquadRun(t, &squadExecution, runID, claim, permitToken, permitNonce, "success")
	terminalAfter, err := f.svc.Queries.ListCollaborationCommands(f.ctx, f.issue.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	for _, original := range retainedTerminal {
		if !mentionAdmissionRetainsCommand(terminalAfter, original) {
			t.Fatal("public Switch/Resume replayed or moved original cancelled/stopped input")
		}
	}
	inputsAfter, err := f.svc.Queries.ListCollaborationGroupInputs(f.ctx, util.MustParseUUID(stopped.claim.Identity.GroupID))
	if err != nil || !reflect.DeepEqual(inputsAfter, oldInputs) {
		t.Fatalf("public Switch rewrote original group input history: %v", err)
	}
	channelAfter, err := f.svc.Queries.GetCollaborationChannel(f.ctx, squad.ChannelID)
	if err != nil || !reflect.DeepEqual(channelAfter, oldChannel) {
		t.Fatalf("public Switch changed old channel controls/history: %v", err)
	}
	var afterHistory string
	f.fx.QueryRow(t, `SELECT jsonb_build_object(
		'group',(SELECT to_jsonb(g) FROM collaboration_execution_group g WHERE id=$1),
		'runs',(SELECT jsonb_agg(to_jsonb(r) ORDER BY r.id) FROM collaboration_run r WHERE group_id=$1),
		'inputs',(SELECT jsonb_agg(to_jsonb(i) ORDER BY i.command_id) FROM collaboration_group_input i WHERE group_id=$1),
		'dependencies',(SELECT jsonb_agg(to_jsonb(d) ORDER BY d.command_id) FROM collaboration_group_dependency d WHERE group_id=$1),
		'proofs',(SELECT jsonb_agg(to_jsonb(p) ORDER BY p.run_id) FROM collaboration_termination p WHERE run_id IN (SELECT id FROM collaboration_run WHERE group_id=$1)),
		'attempts',(SELECT jsonb_agg(to_jsonb(a) ORDER BY a.run_id,a.attempt_seq) FROM collaboration_attempt a WHERE run_id IN (SELECT id FROM collaboration_run WHERE group_id=$1))
	)::text`, stopped.claim.Identity.GroupID).Scan(&afterHistory)
	if oldHistory != afterHistory {
		t.Fatal("public Switch/Resume changed the original stopped group, grants, attempts or proof history")
	}
	beforeRecovery := controlStateSnapshot(t, f.collaborationQueueFixture)
	recovered, err := f.svc.GetSwitchOperation(f.ctx, f.issue.Issue.ID, commit.RequestID)
	if err != nil || !reflect.DeepEqual(recovered, switched) || beforeRecovery != controlStateSnapshot(t, f.collaborationQueueFixture) {
		t.Fatalf("old public Switch GET replay changed execution: %+v %v", recovered, err)
	}
}
