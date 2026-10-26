package service

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func hostExecutionView(t *testing.T, f *managedCollaborationFixture, version int) (CollaborationExecutionView, error) {
	t.Helper()
	ctx := WithCollaborationExecutionViewVersion(f.ctx, version)
	access, err := f.svc.ResolveIssueAccess(ctx, f.svc.Queries, f.issue.Issue.ID)
	if err != nil {
		return CollaborationExecutionView{}, err
	}
	return f.svc.ExecutionView(ctx, access)
}

func TestCollaborationHostIntegrationStoppedHistoryUsesOriginalIdentity(t *testing.T) {
	f, caps := newHostCollaborationFixture(t)
	submitHostCommand(t, f, f.handler, "immutable caller remains visible after Stop")
	id, claim := claimHostRun(t, f, caps)
	launch := hostPermitRequest(claim, 1, "provider")
	if _, err := f.svc.PermitRuntimeRun(f.daemonCtx, id, launch); err != nil {
		t.Fatal(err)
	}
	stop := f.issueControl(t)
	stop.ExecutionViewVersion = 6
	if _, err := f.svc.StopQueue(f.ctx, stop); err != nil {
		t.Fatal(err)
	}
	end := hostLaunchEnded(launch, true)
	end.Cleanup = protocol.CollaborationCleanupObservation{State: "best_effort", Scope: "process_group", Attempted: true}
	if _, err := f.svc.AcceptRuntimeLaunchTermination(context.Background(), id, claim.TerminationToken, end); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.AcceptRuntimeProof(context.Background(), id, claim.TerminationToken, hostFinalProof(t, claim, end)); err != nil {
		t.Fatal(err)
	}
	// The run's owner provenance remains the original claim even after the
	// current agent changes owner. History does not borrow current resource data.
	f.fx.Member(t, f.workspace, f.handler, "member")
	f.fx.Exec(t, `UPDATE agent SET owner_id=$2 WHERE id=$1`, f.grant.AgentID, f.handler)
	view, err := hostExecutionView(t, f, 6)
	if err != nil {
		t.Fatalf("v6 stopped history lost its original claim identity: %v", err)
	}
	if view.ProtocolVersion != 6 || len(view.Runs) != 1 || view.Runs[0].ExecutionMode != launch.Containment ||
		view.Runs[0].ProofState != "host_exit_observed" || view.Runs[0].LaunchCount == nil || *view.Runs[0].LaunchCount != "1" ||
		view.Runs[0].BusinessOutcome != nil || !view.Runs[0].RecordingIncomplete {
		t.Fatal("v6 collapsed business outcome, termination or launch facts")
	}
	var cleanup protocol.CollaborationCleanupObservation
	if err = json.Unmarshal(view.Runs[0].DescendantCleanup, &cleanup); err != nil || cleanup != end.Cleanup {
		t.Fatalf("v6 changed the actual cleanup scope: %v", err)
	}
	page, err := f.svc.RunRecords(WithCollaborationExecutionViewVersion(f.ctx, 6), f.issue.Issue.ID, id, CollaborationRunRecordsQuery{Limit: 50})
	if err != nil {
		t.Fatalf("stopped run records rejected frozen preparation/provenance: %v", err)
	}
	if page.Resource.ExecutionMode != launch.Containment || page.Resource.AgentOwnerUserID != claim.HostContext.AgentOwnerUserID ||
		page.Resource.ResourceOwnerUserID != claim.ResourceOwnerID || page.Resource.ProviderVersion == nil || *page.Resource.ProviderVersion != caps.Host.ProviderVersion ||
		len(page.Items) == 0 || page.Items[0].Kind != "input" || page.Items[0].AuthorUserID != f.handler {
		t.Fatal("stopped records dropped frozen owners or real author")
	}
	legacy, err := hostExecutionView(t, f, 5)
	if err != nil || legacy.ProtocolVersion != 5 || !legacy.RequiresUpgrade || legacy.ExecutionGate.Enabled ||
		len(legacy.Targets)+len(legacy.Channels)+len(legacy.Commands)+len(legacy.Groups)+len(legacy.Runs) != 0 ||
		legacy.Availability != "unavailable" || legacy.UnavailableReason != "shared_runtime_unavailable" {
		t.Fatalf("legacy host graph was not one consistent unavailable projection: %v", err)
	}
	access, err := f.svc.ResolveIssueAccess(f.ctx, f.svc.Queries, f.issue.Issue.ID)
	if err != nil || !access.Capabilities.Read || !access.Capabilities.Comment {
		t.Fatalf("old execution view removed human access: %v", err)
	}
}

func TestCollaborationHostIntegrationPreparationHistoryUsesEachLaunch(t *testing.T) {
	f, caps := newHostCollaborationFixture(t)
	submitHostCommand(t, f, f.creator, "prepare through two registered launches")
	id, claim := claimHostRun(t, f, caps)
	for seq, stage := range []string{"attachments", "workspace"} {
		launch := hostPermitRequest(claim, seq+1, "preparation")
		permit, err := f.svc.PermitRuntimeRun(f.daemonCtx, id, launch)
		if err != nil {
			t.Fatal(err)
		}
		observation := protocol.CollaborationPreparationObservation{ProtocolVersion: 3, Containment: launch.Containment, Identity: claim.Identity,
			EventID: uuid.NewString(), AttemptSeq: launch.AttemptSeq, AttemptNonce: launch.AttemptNonce, ManifestDigest: claim.Preparation.ManifestDigest,
			Stage: stage, State: "prepared"}
		if _, err = f.svc.RecordRuntimePreparation(context.Background(), id, permit.BusinessToken, observation); err != nil {
			t.Fatal(err)
		}
		if _, err = f.svc.AcceptRuntimeLaunchTermination(context.Background(), id, claim.TerminationToken, hostLaunchEnded(launch, false)); err != nil {
			t.Fatal(err)
		}
	}
	page, err := f.svc.RunRecords(WithCollaborationExecutionViewVersion(f.ctx, 6), f.issue.Issue.ID, id, CollaborationRunRecordsQuery{Limit: 50})
	if err != nil {
		t.Fatalf("host preparation history still assumed a single launch: %v", err)
	}
	if page.Preparation == nil || len(page.Preparation.Stages) != 2 || page.Preparation.Stages[0].Stage != "attachments" ||
		page.Preparation.Stages[1].Stage != "workspace" || page.Run.LaunchCount == nil || *page.Run.LaunchCount != "2" {
		t.Fatal("preparation stages lost per-launch correspondence or launch count")
	}
}

func TestCollaborationHostIntegrationZeroPermitStopRemainsReadable(t *testing.T) {
	f, caps := newHostCollaborationFixture(t)
	submitHostCommand(t, f, f.creator, "Stop before any launch is permitted")
	_, _ = claimHostRun(t, f, caps)
	stop := f.issueControl(t)
	stop.ExecutionViewVersion = 6
	if _, err := f.svc.StopQueue(f.ctx, stop); err != nil {
		t.Fatal(err)
	}
	view, err := hostExecutionView(t, f, 6)
	if err != nil {
		t.Fatalf("server-proven zero-permit settlement required a nonexistent runtime receipt: %v", err)
	}
	if len(view.Runs) != 1 || view.Runs[0].ProofState != "never_launched_proven" || view.Runs[0].LaunchCount == nil || *view.Runs[0].LaunchCount != "0" ||
		string(view.Runs[0].DescendantCleanup) != "null" {
		t.Fatal("zero-permit Stop invented process cleanup")
	}
}

func TestCollaborationHostIntegrationResourceMetadataAndLegacyAvailability(t *testing.T) {
	f, caps := newHostCollaborationFixture(t)
	ctx := WithCollaborationExecutionViewVersion(f.ctx, 6)
	resources, err := f.svc.Resources(ctx, f.issue.Issue.ID)
	if err != nil || len(resources.Targets) != 1 || len(resources.Targets[0].Resources) != 1 {
		t.Fatalf("host resource summary unavailable: %v", err)
	}
	check := func(resource CollaborationResourceView) {
		t.Helper()
		if resource.ExecutionMode != caps.Containment || resource.AgentOwnerUserID != f.creator || resource.OwnerUserID != f.creator ||
			resource.Provider == nil || *resource.Provider != f.runtime.Provider || resource.ProviderVersion == nil || *resource.ProviderVersion != caps.Host.ProviderVersion ||
			resource.AccountSource == nil || *resource.AccountSource != "runtime_configuration" || resource.Runtime == nil || resource.Runtime.ID != f.runtime.ID {
			t.Fatal("host resource metadata did not match the frontend contract")
		}
		raw, err := json.Marshal(resource)
		if err != nil {
			t.Fatal(err)
		}
		for _, private := range []string{"configuration_digest", "capability_digest", "installation_id", "binding_id", "custom_env", "instructions"} {
			if strings.Contains(string(raw), private) {
				t.Fatalf("resource summary exposed private field %s", private)
			}
		}
	}
	check(resources.Targets[0].Resources[0])
	visible := func(_ db.Member, agent db.Agent, _ db.AgentRuntime, _ []db.AgentInvocationTarget) bool {
		return agent.ID == f.grant.AgentID
	}
	options, err := f.svc.ResourceOptions(ctx, f.issue.Issue.ID, f.runtime.WorkspaceID, visible)
	if err != nil || len(options.Items) != 1 || !options.Items[0].Available {
		t.Fatalf("v6 host resource option unavailable: %v", err)
	}
	check(options.Items[0].Resources[0])
	identity, err := f.svc.ResourceIdentity(ctx, f.issue.Issue.ID, f.targetID, f.grant.AgentID, pgtype.UUID{})
	if err != nil || identity.Identity == nil || identity.Reason != "" || identity.Identity.Kind != "managed" ||
		identity.Identity.InstallationID != f.installation.ID || identity.Identity.BindingID != f.binding.ID || identity.Identity.BindingEpoch != f.binding.BindingEpoch {
		t.Fatalf("v6 consent identity did not match current managed authority: %v", err)
	}
	legacy, err := f.svc.Resources(f.ctx, f.issue.Issue.ID)
	if err != nil || legacy.Targets[0].Authorized || legacy.Targets[0].Reason != "collaboration_client_upgrade_required" ||
		legacy.Targets[0].Resources[0].CanConsent || !legacy.Targets[0].Resources[0].CanRevoke {
		t.Fatalf("legacy resource view exposed authorization or lost human revocation: %v", err)
	}
	oldOptions, err := f.svc.ResourceOptions(f.ctx, f.issue.Issue.ID, f.runtime.WorkspaceID, visible)
	if err != nil || len(oldOptions.Items) != 1 || oldOptions.Items[0].Available || oldOptions.Items[0].Reason != "collaboration_client_upgrade_required" {
		t.Fatalf("legacy resource options offered incompatible host execution: %v", err)
	}
	unknown := WithCollaborationExecutionViewVersion(f.ctx, 7)
	if _, err = f.svc.ResourceOptions(unknown, f.issue.Issue.ID, f.runtime.WorkspaceID, visible); !collaborationErrorCode(err, "unsupported_execution_view_version") {
		t.Fatalf("resource options silently accepted unknown read semantics: %v", err)
	}
	if _, err = f.svc.ResourceIdentity(unknown, f.issue.Issue.ID, f.targetID, f.grant.AgentID, pgtype.UUID{}); !collaborationErrorCode(err, "unsupported_execution_view_version") {
		t.Fatalf("resource identity silently accepted unknown read semantics: %v", err)
	}
}

// Regression: the consent form requires the v6 execution summary on switch
// proposal resources too, or nobody can consent to a pending switch.
func TestCollaborationHostIntegrationSwitchProposalCarriesExecutionMetadata(t *testing.T) {
	f, caps := newHostCollaborationFixture(t)
	switchCleanup(t, f.collaborationQueueFixture)
	target := queueTarget(t, f.collaborationQueueFixture, f.targetID)
	proposal, err := f.svc.ProposeTargetSwitch(f.ctx, CollaborationProposeSwitchParams{CollaborationQueueRequest: hostQueueRequest(f),
		TargetID: target.ID, ExpectedTargetEpoch: target.TargetEpoch, ExpectedConfigVersion: target.ConfigVersion, Roster: switchRoster(t, target)})
	if err != nil {
		t.Fatal(err)
	}
	view, err := f.svc.GetTargetSwitchProposal(WithCollaborationExecutionViewVersion(f.ctx, 6), f.issue.Issue.ID, target.ID, proposal.ProposalID)
	if err != nil || len(view.Resources) != 1 {
		t.Fatalf("v6 switch proposal unavailable: %v", err)
	}
	// Decode the wire contract the consent form reads, not the Go struct.
	raw, err := json.Marshal(view.Resources[0])
	if err != nil {
		t.Fatal(err)
	}
	var wire struct {
		ExecutionMode    string  `json:"execution_mode"`
		AgentOwnerUserID string  `json:"agent_owner_user_id"`
		Provider         *string `json:"provider"`
		ProviderVersion  *string `json:"provider_version"`
		AccountSource    *string `json:"account_source"`
	}
	if err := json.Unmarshal(raw, &wire); err != nil {
		t.Fatal(err)
	}
	if wire.ExecutionMode != caps.Containment || wire.AgentOwnerUserID != f.creator ||
		wire.Provider == nil || *wire.Provider != f.runtime.Provider || wire.ProviderVersion == nil || *wire.ProviderVersion != caps.Host.ProviderVersion ||
		wire.AccountSource == nil || *wire.AccountSource != "runtime_configuration" {
		t.Fatalf("switch proposal resource lacks the v6 execution summary: %s", raw)
	}
	for _, private := range []string{"configuration_digest", "capability_digest", "installation_id", "binding_id", "custom_env", "instructions"} {
		if strings.Contains(string(raw), private) {
			t.Fatalf("switch proposal resource exposed private field %s", private)
		}
	}
	legacy, err := f.svc.GetTargetSwitchProposal(f.ctx, f.issue.Issue.ID, target.ID, proposal.ProposalID)
	if err != nil {
		t.Fatal(err)
	}
	if raw, err = json.Marshal(legacy.Resources[0]); err != nil {
		t.Fatal(err)
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"execution_mode", "provider", "provider_version", "account_source", "agent_owner_user_id"} {
		if _, ok := top[field]; ok {
			t.Fatalf("v5 switch proposal gained v6 field %s: %s", field, raw)
		}
	}
}

func TestCollaborationHostIntegrationMixedCommentKeepsHumanEffectsAndOriginalRefusal(t *testing.T) {
	f, _ := newHostCollaborationFixture(t)
	f.fx.Cleanup(t, `DELETE FROM comment WHERE issue_id=$1`, f.issue.Issue.ID)
	legacy := ownerSwitchNamespace(t, f, f.creator, "Compatible legacy target")
	legacyTarget := ownerSwitchRegister(t, legacy, collaboration.TargetAgent, legacy.grant.AgentID, []collaboration.TargetResource{ownerSwitchResource(legacy, "agent")})
	legacy.grant = ownerSwitchConsent(t, legacy, legacyTarget)
	hostTarget := queueTarget(t, f.collaborationQueueFixture, f.targetID)
	content := "[@Host](mention://agent/" + util.UUIDToString(f.grant.AgentID) + ") [@Legacy](mention://agent/" + util.UUIDToString(legacy.grant.AgentID) + ") [@Person](mention://member/" + f.handler + ") keep this discussion"
	input := CollaborationCreateCommentParams{CollaborationQueueRequest: f.request(), CollaborationCommentInput: CollaborationCommentInput{Content: content},
		TargetVersions: []CollaborationCommentTargetVersion{{TargetID: hostTarget.ID, ExpectedTargetEpoch: hostTarget.TargetEpoch, ExpectedConfigVersion: hostTarget.ConfigVersion},
			{TargetID: legacyTarget.ID, ExpectedTargetEpoch: legacyTarget.TargetEpoch, ExpectedConfigVersion: legacyTarget.ConfigVersion}}}
	ctx6 := WithCollaborationExecutionViewVersion(f.ctx, 6)
	preview, err := f.svc.PreviewComment(ctx6, f.issue.Issue.ID, input.CollaborationCommentInput)
	if err != nil || preview.ExecutionViewVersion != 6 || len(preview.Targets) != 2 || preview.Targets[0].State != "waiting" {
		t.Fatalf("v6 preview did not preserve host target and negotiated semantics: %v", err)
	}
	oldPreview, err := f.svc.PreviewComment(f.ctx, f.issue.Issue.ID, input.CollaborationCommentInput)
	if err != nil || oldPreview.ExecutionViewVersion != 0 || len(oldPreview.Targets) != 2 ||
		oldPreview.Targets[0].Code != "collaboration_client_upgrade_required" || oldPreview.Targets[0].State != "blocked" || oldPreview.Targets[0].Grouping != nil || oldPreview.Targets[1].State != "waiting" {
		t.Fatalf("legacy preview did not isolate the incompatible AI target: %v", err)
	}
	created, err := f.svc.CreateComment(f.ctx, input)
	if err != nil || len(created.Targets) != 2 || created.Targets[0].State != "blocked" || created.Targets[0].Code != "collaboration_client_upgrade_required" ||
		created.Targets[0].CommandID.Valid || created.Targets[1].State != "waiting" || !created.Targets[1].CommandID.Valid {
		t.Fatalf("legacy mixed comment lost target-specific results: %v", err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM comment WHERE issue_id=$1 AND id=$2 AND content=$3`, f.issue.Issue.ID, created.CommentID, content) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE target_id=$1`, hostTarget.ID) != 0 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE target_id=$1`, legacyTarget.ID) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_notification WHERE issue_id=$1 AND source_id=$2 AND recipient_user_id=$3 AND is_mention`, f.issue.Issue.ID, created.CommentID, f.handler) != 1 {
		t.Fatal("incompatible host AI swallowed human comment/notification or compatible target")
	}
	replay, err := f.svc.CreateComment(ctx6, input)
	if err != nil || !reflect.DeepEqual(replay, created) {
		t.Fatalf("upgraded reader reinterpreted the original blocked request: %v", err)
	}
	upgraded := input
	upgraded.ExecutionViewVersion = 6
	if _, err = f.svc.CreateComment(ctx6, upgraded); !collaborationErrorCode(err, "idempotency_conflict") {
		t.Fatalf("same key with upgraded AI metadata became new intent: %v", err)
	}
	recovered, err := f.svc.GetCommentOperation(ctx6, f.issue.Issue.ID, input.RequestID)
	if err != nil || !reflect.DeepEqual(recovered, created) {
		t.Fatalf("original receipt recovery changed its refusal: %v", err)
	}
	upgraded.RequestID, upgraded.Content = uuid.NewString(), "[@Host](mention://agent/"+util.UUIDToString(f.grant.AgentID)+") explicit new instruction"
	upgraded.TargetVersions = upgraded.TargetVersions[:1]
	next, err := f.svc.CreateComment(ctx6, upgraded)
	if err != nil || len(next.Targets) != 1 || next.Targets[0].State != "waiting" || !next.Targets[0].CommandID.Valid {
		t.Fatalf("fresh v6 intent could not use the already authorized host target: %v", err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE target_id=$1`, hostTarget.ID) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE target_id=$1`, legacyTarget.ID) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM comment WHERE issue_id=$1`, f.issue.Issue.ID) != 2 {
		t.Fatal("new explicit v6 request or recovery duplicated an old effect")
	}
}

func TestCollaborationHostIntegrationHumanActionsNeedNoAIMetadata(t *testing.T) {
	for _, action := range []string{"complete", "cancel", "leave", "revoke"} {
		t.Run(action, func(t *testing.T) {
			f, caps := newHostCollaborationFixture(t)
			submitHostCommand(t, f, f.handler, "human policy can stop this host execution")
			id, claim := claimHostRun(t, f, caps)
			launch := hostPermitRequest(claim, 1, "provider")
			if _, err := f.svc.PermitRuntimeRun(f.daemonCtx, id, launch); err != nil {
				t.Fatal(err)
			}
			switch action {
			case "complete", "cancel":
				if err := f.svc.Queries.SeedIssueStatusEntries(f.ctx, f.runtime.WorkspaceID); err != nil {
					t.Fatal(err)
				}
				f.fx.Cleanup(t, `DELETE FROM issue_status WHERE workspace_id=$1`, f.workspace)
				status := "done"
				if action == "cancel" {
					status = "cancelled"
				}
				request := lifecycleParams(t, f.collaborationQueueFixture, action, status, 1, 1)
				if request.ExecutionViewVersion != 0 {
					t.Fatal("human lifecycle unexpectedly inherited AI metadata")
				}
				result, err := f.svc.ChangeLifecycle(f.ctx, request)
				if err != nil || result.QueueState != "closed" {
					t.Fatalf("legacy human lifecycle was blocked by host mode: %v", err)
				}
			case "leave":
				transferred, err := f.svc.Transfer(f.ctx, CollaborationTransferParams{CollaborationParticipantParams: CollaborationParticipantParams{
					CollaborationWorkflowRequest: workflowRequest(f.collaborationQueueFixture, 1), TargetUserID: util.MustParseUUID(f.creator)}})
				if err != nil {
					t.Fatal(err)
				}
				request := workflowRequest(f.collaborationQueueFixture, transferred.CollaborationRevision)
				if request.ExecutionViewVersion != 0 {
					t.Fatal("human leave unexpectedly inherited AI metadata")
				}
				result, err := f.svc.Leave(collaborationPasswordContext(util.MustParseUUID(f.handler), 1), request)
				if err != nil || result.Code != "participant_left" {
					t.Fatalf("legacy human leave was blocked by host mode: %v", err)
				}
			case "revoke":
				request := CollaborationRevokeGrantParams{CollaborationQueueRequest: f.request(), GrantID: f.grant.ID, ExpectedGrantEpoch: f.grant.Epoch}
				if request.ExecutionViewVersion != 0 {
					t.Fatal("human revocation unexpectedly inherited AI metadata")
				}
				if _, err := f.svc.RevokeGrant(f.ctx, request); err != nil {
					t.Fatalf("legacy owner revocation was blocked by host mode: %v", err)
				}
			}
			intent, err := f.svc.RuntimeStopIntent(context.Background(), id, claim.TerminationToken)
			if err != nil || !intent.StopRequested {
				t.Fatalf("human action failed to request system cleanup: %v", err)
			}
			end := hostLaunchEnded(launch, true)
			if _, err = f.svc.AcceptRuntimeLaunchTermination(context.Background(), id, claim.TerminationToken, end); err != nil {
				t.Fatal(err)
			}
			if _, err = f.svc.AcceptRuntimeProof(context.Background(), id, claim.TerminationToken, hostFinalProof(t, claim, end)); err != nil {
				t.Fatal(err)
			}
			view, err := hostExecutionView(t, f, 6)
			if err != nil || len(view.Runs) != 1 || view.Runs[0].ProofState != "host_exit_observed" || view.Runs[0].SettledAt == nil {
				t.Fatalf("human policy settlement lost readable host history: %v", err)
			}
		})
	}
}
