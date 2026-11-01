package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

type preparationExternalFixture struct {
	*taskKnowledgeFixture
	source   pgtype.UUID
	proposal protocol.CollaborationKnowledgeProposal
}

func newPreparationExternalFixture(t *testing.T) *preparationExternalFixture {
	t.Helper()
	f := newTaskKnowledgeFixture(t)
	f.withdraw(t, f.candidate.CandidateID)
	chat := f.fx.ChatSession(t, util.UUIDToString(f.task.AgentID), testutil.Cols{"runtime_id": f.task.RuntimeID})
	f.fx.Exec(t, `UPDATE agent_task_queue SET issue_id=NULL,chat_session_id=$2 WHERE id=$1`, f.task.ID, chat)
	var err error
	f.task, err = f.tasks.Queries.GetAgentTask(f.ctx, f.task.ID)
	if err != nil {
		t.Fatal(err)
	}
	owner := f.fx.User(t, "External source owner", "prep-external-"+uuid.NewString()+"@example.test")
	workspace := f.fx.Workspace(t, "External preparation source", "prep-external-"+uuid.NewString())
	f.fx.Member(t, workspace, owner, "owner")
	ownerCtx := collaborationContext(owner)
	source, err := f.svc.Create(ownerCtx, CollaborationCreateParams{SourceWorkspaceID: util.MustParseUUID(workspace), HandlerID: util.MustParseUUID(f.creator), RequestID: uuid.NewString(), Title: "Preparation source read grant"})
	if err != nil {
		t.Fatal(err)
	}
	// This source is outside f.workspace, so the base fixture cannot clean its
	// service-owned rows. Remove them before parent teardown, retaining knowledge history.
	f.fx.Cleanup(t, `DELETE FROM issue WHERE id=$1`, source.Issue.ID)
	for _, table := range []string{"issue_collaboration", "issue_participant", "collaboration_session", "issue_handoff", "collaboration_event", "collaboration_notification", "collaboration_memory_version"} {
		f.fx.Cleanup(t, `DELETE FROM `+table+` WHERE issue_id=$1`, source.Issue.ID)
	}
	f.fx.Cleanup(t, `DELETE FROM collaboration_operation WHERE scope_kind='issue' AND scope_id=$1`, source.Issue.ID)
	f.fx.Cleanup(t, `DELETE FROM collaboration_operation WHERE scope_kind='create' AND scope_id=$1`, workspace)
	f.fx.Cleanup(t, `DELETE FROM collaboration_account_fence WHERE user_id=$1`, owner)
	if _, err = f.svc.UpdateMemory(ownerCtx, CollaborationMemoryUpdateParams{IssueID: source.Issue.ID, RequestID: uuid.NewString(), ExpectedVersion: 0, Content: "EXTERNAL_PREPARATION_PRIVATE_SOURCE"}); err != nil {
		t.Fatal(err)
	}
	p := protocol.CollaborationKnowledgeProposal{ProtocolVersion: 1, CandidateID: uuid.NewString(), RequestID: uuid.NewString(), Target: protocol.CollaborationKnowledgeTarget{Kind: "workspace", WorkspaceID: f.workspace}, Summary: "EXTERNAL_PRIVATE_SUMMARY", Content: "EXTERNAL_PRIVATE_DISCLOSURE", Sources: []protocol.CollaborationKnowledgeSource{{Kind: "memory", IssueID: util.UUIDToString(source.Issue.ID), Version: "1", SHA256: protocol.CollaborationKnowledgeHash("EXTERNAL_PREPARATION_PRIVATE_SOURCE")}}, Evidence: []protocol.CollaborationKnowledgeEvidence{}, Applicability: protocol.CollaborationKnowledgeApplicability{Surfaces: []string{"chat"}}}
	if _, err = f.svc.ProposeKnowledge(f.ctx, source.Issue.ID, p); err != nil {
		t.Fatal(err)
	}
	if _, err = f.svc.ApproveKnowledge(f.ctx, util.MustParseUUID(p.CandidateID), knowledgeApproveRequest(p)); err != nil {
		t.Fatal(err)
	}
	result := &preparationExternalFixture{taskKnowledgeFixture: f, source: source.Issue.ID, proposal: p}
	// A former handler can leave. Transfer changes responsibility but retains
	// the participant read basis that the actual revocation below removes.
	if _, err = f.svc.Transfer(f.ctx, CollaborationTransferParams{CollaborationParticipantParams: CollaborationParticipantParams{CollaborationWorkflowRequest: result.sourceRequest(t), TargetUserID: util.MustParseUUID(owner)}}); err != nil {
		t.Fatal(err)
	}
	access, err := f.svc.ResolveIssueAccess(f.ctx, f.svc.Queries, source.Issue.ID)
	if err != nil || access.SourceMember || !access.ParticipantEpoch.Valid {
		t.Fatalf("fixture must rely on source participation only: %+v %v", access, err)
	}
	return result
}

func (f *preparationExternalFixture) sourceRequest(t *testing.T) CollaborationWorkflowRequest {
	t.Helper()
	access, err := f.svc.ResolveIssueAccess(f.ctx, f.svc.Queries, f.source)
	if err != nil {
		t.Fatal(err)
	}
	return CollaborationWorkflowRequest{CollaborationQueueRequest: CollaborationQueueRequest{IssueID: f.source, RequestID: uuid.NewString(), ExpectedLifecycleEpoch: access.LifecycleEpoch, ExpectedParticipantEpoch: access.ParticipantEpoch.Int64, ExpectedAccountEpoch: access.AccountEpoch.Int64}, ExpectedCollaborationRevision: access.Revision}
}

func (f *preparationExternalFixture) assertSourceRevokedOnly(t *testing.T) {
	t.Helper()
	if _, err := f.svc.ResolveIssueAccess(f.ctx, f.svc.Queries, f.source); err == nil {
		t.Fatal("source participation revocation retained read access")
	}
	if f.fx.Count(t, `SELECT count(*) FROM member WHERE workspace_id=$1 AND user_id=$2`, f.workspace, f.creator) != 1 {
		t.Fatal("revocation changed task workspace authority")
	}
}

func assertNoExternalPreparationBody(t *testing.T, out protocol.TaskKnowledgePrepareResponse, p protocol.CollaborationKnowledgeProposal) {
	t.Helper()
	raw, err := json.Marshal(out)
	if err != nil {
		t.Fatal(err)
	}
	for _, private := range []string{p.Content, p.Summary, p.CandidateID, p.Sources[0].IssueID} {
		if strings.Contains(string(raw), private) {
			t.Fatal("unavailable external source entered preparation response")
		}
	}
}

func TestKnowledgePreparationSourceRevocationBothCommitOrders(t *testing.T) {
	for _, first := range []string{"source_revocation", "disclosure"} {
		t.Run(first, func(t *testing.T) {
			f := newPreparationExternalFixture(t)
			d := f.finalize(t, "")
			req := f.prepareRequest(d)
			facts, _, _, err := f.tasks.taskKnowledgeFacts(f.ctx, f.tasks.Queries, f.task)
			if err != nil {
				t.Fatal(err)
			}
			eligible, err := f.svc.ResolveApprovedKnowledge(f.ctx, f.svc.Queries, facts)
			if err != nil || len(eligible.Contributions) != 1 || eligible.Contributions[0].CandidateID != f.proposal.CandidateID {
				t.Fatalf("external source was not initially eligible: %v", err)
			}
			leave := f.sourceRequest(t)
			gate := newKnowledgeCommitGate(t, f.pool)
			ctx, cancel := context.WithTimeout(f.ctx, 8*time.Second)
			defer cancel()
			if first == "source_revocation" {
				revoker := NewCollaborationService(f.svc.Queries, gate, f.svc.Issues)
				done := make(chan error, 1)
				go func() {
					receipt, err := revoker.Leave(ctx, leave)
					if err == nil && receipt.Code != "participant_left" {
						err = knowledgeFailure(409, receipt.Code)
					}
					done <- err
				}()
				select {
				case <-gate.entered:
				case err := <-done:
					t.Fatalf("source writer before barrier: %v", err)
				case <-ctx.Done():
					t.Fatal("source writer missed barrier")
				}
				blocked, err := f.tasks.PrepareTaskKnowledge(ctx, f.caller, f.task.ID, req)
				if !collaborationErrorCode(err, "knowledge_busy") {
					t.Fatalf("preparation crossed uncommitted source revocation: %v", err)
				}
				assertNoExternalPreparationBody(t, blocked, f.proposal)
				close(gate.release)
				if err = <-done; err != nil {
					t.Fatal(err)
				}
				f.assertSourceRevokedOnly(t)
				fresh, err := f.tasks.PrepareTaskKnowledge(ctx, f.caller, f.task.ID, req)
				if err != nil {
					t.Fatal(err)
				}
				assertNoExternalPreparationBody(t, fresh, f.proposal)
			} else {
				f.tasks.TxStarter = gate
				type preparedResult struct {
					out protocol.TaskKnowledgePrepareResponse
					err error
				}
				prepared := make(chan preparedResult, 1)
				go func() {
					out, err := f.tasks.PrepareTaskKnowledge(ctx, f.caller, f.task.ID, req)
					prepared <- preparedResult{out, err}
				}()
				var holder uint32
				select {
				case holder = <-gate.entered:
				case result := <-prepared:
					t.Fatalf("preparation before barrier: %v", result.err)
				case <-ctx.Done():
					t.Fatal("preparation missed barrier")
				}
				revoked := make(chan error, 1)
				go func() {
					receipt, err := f.svc.Leave(ctx, leave)
					if err == nil && receipt.Code != "participant_left" {
						err = knowledgeFailure(409, receipt.Code)
					}
					revoked <- err
				}()
				knowledgeWaitForBlock(t, f.pool, holder)
				select {
				case <-prepared:
					t.Fatal("private body returned before commit")
				default:
				}
				close(gate.release)
				result := <-prepared
				if result.err != nil || len(result.out.Contributions) != 1 {
					t.Fatalf("authorized earlier disclosure failed: %v", result.err)
				}
				if err = <-revoked; err != nil {
					t.Fatal(err)
				}
				f.tasks.TxStarter = f.pool
				f.assertSourceRevokedOnly(t)
				replay, err := f.tasks.PrepareTaskKnowledge(ctx, f.caller, f.task.ID, req)
				if !collaborationErrorCode(err, "knowledge_context_changed") {
					t.Fatalf("exact original manifest replay survived source revocation: %v", err)
				}
				assertNoExternalPreparationBody(t, replay, f.proposal)
			}
		})
	}
}

func TestKnowledgeSourceAuthorityChangesBetweenClaimBuildAndFinalize(t *testing.T) {
	f := newPreparationExternalFixture(t)
	d, _, err := f.tasks.BuildTaskKnowledgeClaim(f.ctx, f.task, &f.inv, "")
	if err != nil {
		t.Fatal(err)
	}
	if receipt, err := f.svc.Leave(f.ctx, f.sourceRequest(t)); err != nil || receipt.Code != "participant_left" {
		t.Fatalf("real source revocation failed: %s %v", receipt.Code, err)
	}
	f.assertSourceRevokedOnly(t)
	_, err = f.tasks.FinalizeTaskClaim(f.ctx, f.task, db.CreateTaskTokenParams{TokenHash: "matrix-" + uuid.NewString(), TaskID: f.task.ID, AgentID: f.task.AgentID, WorkspaceID: util.MustParseUUID(f.workspace), UserID: util.MustParseUUID(f.creator), ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(time.Hour), Valid: true}}, nil, false, d)
	if err != nil {
		t.Fatalf("source-free finalization imported private source authority: %v", err)
	}
	raw, _ := json.Marshal(d)
	for _, private := range []string{f.proposal.Content, f.proposal.Summary, f.proposal.CandidateID, util.UUIDToString(f.source)} {
		if strings.Contains(string(raw), private) {
			t.Fatal("claim finalization disclosed private source")
		}
	}
	if _, err = f.tasks.StartTask(f.ctx, f.task.ID, nil); !collaborationErrorCode(err, "knowledge_context_changed") {
		t.Fatalf("missing claim fence accepted: %v", err)
	}
	fence := &protocol.TaskKnowledgeFence{ClaimGeneration: d.ClaimGeneration, DispatchedAt: d.DispatchedAt}
	if _, err = f.tasks.StartTask(f.ctx, f.task.ID, fence); err != nil {
		t.Fatalf("current target-authorized task could not start: %v", err)
	}
	out, err := f.tasks.PrepareTaskKnowledge(f.ctx, f.caller, f.task.ID, f.prepareRequest(d))
	if err != nil {
		t.Fatal(err)
	}
	assertNoExternalPreparationBody(t, out, f.proposal)
}
