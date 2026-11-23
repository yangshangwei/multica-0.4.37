package service

import (
	"context"
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// This test proves real DB/service composition for each squad role. Termination
// observations are synthetic service inputs, not claims of a physical process.
// Actual member stdin/argv and the real direct HTTP/HostExecutor chain have
// separate tests in collaborationrun and the knowledgeintegration suite.
func TestCollaborationKnowledgeSquadRootMemberContinuation(t *testing.T) {
	f, _ := newHostCollaborationFixture(t)
	f.fx.Exec(t, `UPDATE agent_runtime SET provider='claude' WHERE id=$1`, f.runtime.ID)
	var err error
	f.runtime, err = f.svc.Queries.GetAgentRuntime(f.ctx, f.runtime.ID)
	if err != nil {
		t.Fatal(err)
	}
	registration := hostCapabilityRegistration(f)
	if _, err = f.svc.RegisterRuntimeCapabilities(f.daemonCtx, registration); err != nil {
		t.Fatal(err)
	}
	f.runtime, err = f.svc.Queries.GetAgentRuntime(f.ctx, f.runtime.ID)
	if err != nil {
		t.Fatal(err)
	}
	renewHostConsent(t, f)
	f.svc.KnowledgeContextEnabled = true
	f.fx.Cleanup(t, `DELETE FROM collaboration_memory_version WHERE issue_id=$1`, f.issue.Issue.ID)
	f.fx.Cleanup(t, `DELETE FROM collaboration_knowledge_candidate WHERE source_issue_id=$1`, f.issue.Issue.ID)
	f.fx.Cleanup(t, `DELETE FROM collaboration_knowledge_target WHERE workspace_id=$1`, f.workspace)
	f.fx.Cleanup(t, `DELETE FROM collaboration_knowledge_invalidation WHERE target_workspace_id=$1`, f.workspace)
	proposal := knowledgeServiceProposal(t, f.collaborationQueueFixture)
	if _, err = f.svc.ProposeKnowledge(f.ctx, f.issue.Issue.ID, proposal); err != nil {
		t.Fatal(err)
	}
	if _, err = f.svc.ApproveKnowledge(f.ctx, util.MustParseUUID(proposal.CandidateID), knowledgeApproveRequest(proposal)); err != nil {
		t.Fatal(err)
	}
	credential := auth.HashToken("mdt_" + uuid.NewString())
	f.fx.Insert(t, "daemon_token", testutil.Cols{"token_hash": credential, "workspace_id": f.workspace, "daemon_id": f.runtime.DaemonID.String, "user_id": f.creator, "auth_version": 1, "expires_at": time.Now().Add(time.Hour)})

	leader := util.UUIDToString(f.grant.AgentID)
	member := f.fx.Agent(t, "Knowledge squad member "+uuid.NewString(), util.UUIDToString(f.runtime.ID), testutil.Cols{"owner_id": f.creator, "permission_mode": "private"})
	squad := f.fx.Squad(t, "Knowledge service-boundary squad", leader)
	f.fx.SquadMember(t, squad, "agent", member)
	session, err := f.svc.Queries.GetCollaborationSession(f.ctx, f.issue.Issue.ID)
	if err != nil {
		t.Fatal(err)
	}
	target, err := f.svc.RegisterTarget(f.ctx, CollaborationRegisterTargetParams{CollaborationQueueRequest: hostQueueRequest(f), Kind: collaboration.TargetSquad,
		ResourceID: util.MustParseUUID(squad), ExpectedTargetSetVersion: session.TargetSetVersion, Roster: []collaboration.TargetResource{
			{Role: "leader", AgentID: leader, RuntimeID: util.UUIDToString(f.runtime.ID), ExecutionWorkspaceID: f.workspace, OwnerUserID: f.creator},
			{Role: "member", AgentID: member, RuntimeID: util.UUIDToString(f.runtime.ID), ExecutionWorkspaceID: f.workspace, OwnerUserID: f.creator},
		}})
	if err != nil {
		t.Fatal(err)
	}
	f.targetID = target.TargetID
	for _, agentID := range []string{leader, member} {
		_, err = f.svc.ConsentTargetResource(f.ctx, CollaborationConsentParams{CollaborationQueueRequest: hostQueueRequest(f), TargetID: target.TargetID,
			AgentID: util.MustParseUUID(agentID), ExpectedTargetEpoch: 1, ExpectedConfigVersion: 1,
			Identity: CollaborationResourceIdentity{Kind: "managed", InstallationID: f.installation.ID, BindingID: f.binding.ID, BindingEpoch: f.binding.BindingEpoch}})
		if err != nil {
			t.Fatal(err)
		}
	}
	f.fx.Cleanup(t, `DELETE FROM collaboration_delegation WHERE group_id IN (SELECT id FROM collaboration_execution_group WHERE session_id=$1)`, f.issue.SessionID)
	submitHostCommand(t, f, f.creator, "Owner's original squad objective")
	submitHostCommand(t, f, f.handler, "External collaborator's accepted constraint")
	resume := f.issueControl(t)
	resume.ExecutionViewVersion = 6
	if _, err = f.svc.ResumeQueue(f.ctx, CollaborationResumeQueueParams{CollaborationStopQueueParams: resume, ExpectedExecutionEpoch: f.gateEpoch}); err != nil {
		t.Fatal(err)
	}
	request := hostClaimRequest(t, f, registration.Capabilities)
	request.KnowledgeVersion, request.MemoryVersion = 1, protocol.CollaborationMemoryVersion

	type admittedStage struct {
		run     pgtype.UUID
		claim   protocol.CollaborationRunClaimResponse
		launch  protocol.CollaborationRunPermitRequest
		permit  protocol.CollaborationRunPermitResponse
		handoff protocol.CollaborationHandoffEnvelope
	}
	namespaces := map[string]bool{}
	authors := []string{f.creator, f.handler}
	admit := func(run pgtype.UUID, role, agentID string) admittedStage {
		t.Helper()
		claim, err := f.svc.ClaimRuntimeRun(f.daemonCtx, run, request)
		if err != nil {
			t.Fatalf("%s claim: %v", role, err)
		}
		id := util.UUIDToString(run)
		if claim.KnowledgeContext == nil || claim.KnowledgeContext.ProtocolVersion != 1 || claim.KnowledgeContext.Identity != claim.Identity || claim.KnowledgeContext.Namespace != id || namespaces[id] {
			t.Fatalf("%s did not receive its independent knowledge run identity", role)
		}
		namespaces[id] = true
		if claim.HostContext == nil || claim.HostContext.Role != role || claim.AgentID != agentID || claim.ResourceOwnerID != f.creator || len(claim.Inputs) != 2 {
			t.Fatalf("%s changed role, resource owner, agent or accepted input count", role)
		}
		for i, input := range claim.Inputs {
			if input.AuthorUserID != authors[i] {
				t.Fatalf("%s changed original author %d", role, i)
			}
		}
		wire, _ := json.Marshal(claim)
		if strings.Contains(string(wire), proposal.Content) || strings.Contains(string(wire), proposal.CandidateID) {
			t.Fatalf("%s claim disclosed private knowledge", role)
		}
		launch := hostPermitRequest(claim, 1, "provider")
		permit, err := f.svc.PermitRuntimeRun(f.daemonCtx, run, launch)
		if err != nil {
			t.Fatalf("%s permit: %v", role, err)
		}
		memory, err := f.svc.RuntimeMemoryBase(context.Background(), run, permit.BusinessToken)
		if err != nil {
			t.Fatalf("%s memory: %v", role, err)
		}
		handoff, err := f.svc.RuntimeHandoffSnapshot(context.Background(), run, permit.BusinessToken, protocol.CollaborationHandoffSnapshotRequest{RequestID: uuid.NewString(), MemoryVersion: memory.Version})
		if err != nil {
			t.Fatalf("%s handoff: %v", role, err)
		}
		prepare := protocol.CollaborationRunKnowledgePrepare{ProtocolVersion: 1, Identity: claim.Identity, RequestID: uuid.NewString(), HandoffID: handoff.SnapshotID,
			InvocationFingerprint: protocol.CollaborationKnowledgeHash("synthetic service-boundary invocation; actual argv tested separately")}
		bundle, err := f.svc.PrepareRuntimeKnowledge(f.daemonCtx, run, credential, prepare)
		if err != nil {
			t.Fatalf("%s knowledge prepare: %v", role, err)
		}
		if len(bundle.Contributions) != 1 || len(bundle.Manifest.Entries) != 1 || bundle.Contributions[0].CandidateID != proposal.CandidateID || bundle.Contributions[0].Content != proposal.Content || bundle.Manifest.Entries[0].ContentSHA256 != protocol.CollaborationKnowledgeHash(proposal.Content) {
			t.Fatalf("%s did not receive the nonempty exact approved contribution", role)
		}
		replay, err := f.svc.PrepareRuntimeKnowledge(f.daemonCtx, run, credential, prepare)
		if err != nil || !reflect.DeepEqual(bundle, replay) {
			t.Fatalf("%s preparation replay changed: %v", role, err)
		}
		// This digest binds a service test input, not an asserted physical stdin.
		prompt, err := json.Marshal(struct {
			Inputs    []protocol.CollaborationRunInput
			Handoff   protocol.CollaborationHandoffEnvelope
			Knowledge []protocol.CollaborationKnowledgeContribution
		}{claim.Inputs, handoff, bundle.Contributions})
		if err != nil {
			t.Fatal(err)
		}
		binding := protocol.CollaborationHandoffBinding{SnapshotID: handoff.SnapshotID, PromptSHA256: protocol.CollaborationHandoffHash(string(prompt)), AttemptSeq: launch.AttemptSeq, AttemptNonce: launch.AttemptNonce}
		if _, err = f.svc.BindRuntimeHandoff(context.Background(), run, permit.BusinessToken, binding); err != nil {
			t.Fatalf("%s handoff binding: %v", role, err)
		}
		authorize := protocol.CollaborationRunKnowledgeAuthorization{CollaborationRunKnowledgePrepare: prepare, ManifestSHA256: bundle.ManifestSHA256, PromptSHA256: binding.PromptSHA256, AttemptSeq: launch.AttemptSeq, AttemptNonce: launch.AttemptNonce}
		accepted, err := f.svc.AuthorizeRuntimeKnowledge(f.daemonCtx, run, credential, authorize)
		if err != nil || accepted != authorize {
			t.Fatalf("%s knowledge authorization: %v", role, err)
		}
		accepted, err = f.svc.AuthorizeRuntimeKnowledge(f.daemonCtx, run, credential, authorize)
		if err != nil || accepted != authorize {
			t.Fatalf("%s original authorization replay: %v", role, err)
		}
		if f.fx.Count(t, `SELECT count(*) FROM collaboration_run_event WHERE run_id=$1 AND kind='knowledge_preparation'`, run) != 1 || f.fx.Count(t, `SELECT count(*) FROM collaboration_run_event WHERE run_id=$1 AND kind='knowledge_authorization'`, run) != 1 {
			t.Fatalf("%s duplicated private authorization facts", role)
		}
		t.Logf("role=%s knowledge_version=1 approved_items=1 accepted_authors=2 run=%s synthetic_process_proof=true", role, id)
		return admittedStage{run, claim, launch, permit, handoff}
	}
	finish := func(stage admittedStage) {
		t.Helper()
		if err := f.svc.RecordRuntimeOutcome(context.Background(), stage.run, stage.permit.BusinessToken, protocol.CollaborationRunOutcome{ProtocolVersion: 3, Containment: stage.launch.Containment,
			Identity: stage.claim.Identity, AttemptSeq: stage.launch.AttemptSeq, AttemptNonce: stage.launch.AttemptNonce, EventID: uuid.NewString(), Outcome: "success", Body: "Synthetic squad service turn completed"}); err != nil {
			t.Fatal(err)
		}
		end := hostLaunchEnded(stage.launch, true)
		if _, err := f.svc.AcceptRuntimeLaunchTermination(context.Background(), stage.run, stage.claim.TerminationToken, end); err != nil {
			t.Fatal(err)
		}
		if _, err := f.svc.AcceptRuntimeProof(context.Background(), stage.run, stage.claim.TerminationToken, hostFinalProof(t, stage.claim, end)); err != nil {
			t.Fatal(err)
		}
	}
	list, err := f.svc.ListRuntimeRuns(f.daemonCtx, f.runtime.ID)
	if err != nil || len(list.RunIDs) != 1 {
		t.Fatalf("root candidate: %v %v", list, err)
	}
	root := admit(util.MustParseUUID(list.RunIDs[0]), "leader", leader)
	dispatchInput := CollaborationRuntimeCommentParams{CommentID: util.MustParseUUID(uuid.NewString()), Content: "[@member](mention://agent/" + member + ") Only perform the delegated parser check."}
	dispatch, err := f.svc.CreateRuntimeComment(context.Background(), root.run, root.permit.BusinessToken, dispatchInput)
	if err != nil || len(dispatch.RunIDs) != 1 {
		t.Fatalf("knowledge root delegation: %v %v", dispatch, err)
	}
	duplicate, err := f.svc.CreateRuntimeComment(context.Background(), root.run, root.permit.BusinessToken, dispatchInput)
	if err != nil || !reflect.DeepEqual(dispatch, duplicate) {
		t.Fatalf("delegation replay changed member identity: %v", err)
	}
	memberRun := dispatch.RunIDs[0]
	if _, err = f.svc.ClaimRuntimeRun(f.daemonCtx, memberRun, request); !collaborationErrorCode(err, "runtime_busy") {
		t.Fatalf("knowledge member bypassed capacity one: %v", err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_run_event WHERE run_id=$1 AND kind='knowledge_claim'`, memberRun) != 0 {
		t.Fatal("unclaimed member received knowledge negotiation")
	}
	finish(root)
	memberStage := admit(memberRun, "member", member)
	if memberStage.claim.HostContext.ParentRunID == nil || *memberStage.claim.HostContext.ParentRunID != root.claim.Identity.RunID || memberStage.handoff.Assignment.ParentRunID != root.claim.Identity.RunID {
		t.Fatal("member lost its actual root/delegation parent")
	}
	delegated := false
	for _, source := range memberStage.handoff.Sources {
		if source.Kind == "delegation" && source.Trust == "admitted_assignment" && source.Content == dispatchInput.Content && source.RunID == root.claim.Identity.RunID {
			delegated = true
		}
	}
	if !delegated || len(memberStage.handoff.Inputs.Items) != 2 {
		t.Fatal("knowledge member lost complete admitted delegation or input readers")
	}
	for _, input := range memberStage.handoff.Inputs.Items {
		if !slices.Contains(input.ReaderRunIDs, root.claim.Identity.RunID) || !slices.Contains(input.ReaderRunIDs, memberStage.claim.Identity.RunID) {
			t.Fatal("knowledge member changed actual original-input readers")
		}
	}
	for i := 0; i < 2; i++ {
		if _, err = f.svc.CreateRuntimeComment(context.Background(), memberRun, memberStage.permit.BusinessToken, CollaborationRuntimeCommentParams{CommentID: util.MustParseUUID(uuid.NewString()), Content: "Member evidence for the final leader"}); err != nil {
			t.Fatal(err)
		}
	}
	finish(memberStage)
	list, err = f.svc.ListRuntimeRuns(f.daemonCtx, f.runtime.ID)
	if err != nil || len(list.RunIDs) != 1 {
		t.Fatalf("member wakes did not coalesce to one continuation: %v %v", list, err)
	}
	continuation := admit(util.MustParseUUID(list.RunIDs[0]), "continuation", leader)
	causes := map[string]int{}
	for _, cause := range continuation.handoff.WakeupCauses.Items {
		causes[cause.SourceKind]++
	}
	if causes["comment"] != 2 || causes["proof"] != 1 {
		t.Fatalf("knowledge continuation lost coalesced member causes: %v", causes)
	}
	if continuation.run == root.run || continuation.run == memberRun {
		t.Fatal("knowledge continuation reused an earlier run")
	}
	finish(continuation)
	if len(namespaces) != 3 || f.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE session_id=$1 AND proof_state='host_exit_observed' AND settled_at IS NOT NULL`, f.issue.SessionID) != 3 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_execution_group WHERE session_id=$1 AND state='succeeded' AND settled_at IS NOT NULL`, f.issue.SessionID) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_runtime_reservation WHERE run_id IN ($1,$2,$3) AND released_at IS NULL`, root.run, memberRun, continuation.run) != 0 {
		t.Fatal("knowledge squad lost independent proofs, final settlement or capacity release")
	}
}
