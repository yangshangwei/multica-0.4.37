package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func knowledgeApproveRequest(p protocol.CollaborationKnowledgeProposal) protocol.CollaborationKnowledgeApproveRequest {
	return protocol.CollaborationKnowledgeApproveRequest{ProtocolVersion: 1, RequestID: uuid.NewString(), ExpectedStateRevision: "1", ExpectedVersion: "1", ExpectedContentSHA256: protocol.CollaborationKnowledgeHash(p.Content)}
}

func TestKnowledgeExternalHandlerCanProposeToVisibleCurrentSourceProject(t *testing.T) {
	f := newCollaborationMemoryFixture(t)
	p := knowledgeServiceProposal(t, f)
	project := f.fx.Project(t, "PRIVATE_PROJECT_NAME", testutil.Cols{"description": "PRIVATE_PROJECT_DESCRIPTION"})
	f.fx.Exec(t, `UPDATE issue SET project_id=$2,status='done' WHERE id=$1`, f.issue.Issue.ID, project)
	p.Target.Kind = "project"
	p.Target.ProjectID = project
	if _, err := f.svc.ProposeKnowledge(collaborationContext(f.handler), f.issue.Issue.ID, p); err != nil {
		t.Fatal(err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM member WHERE workspace_id=$1 AND user_id=$2`, f.workspace, f.handler) != 0 || f.fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_target WHERE target_id=$1`, project) != 0 {
		t.Fatal("source reference proposal changed membership/effective collection")
	}
}

func TestKnowledgeProposeRejectsInvisibleArbitraryDestination(t *testing.T) {
	f := newCollaborationMemoryFixture(t)
	p := knowledgeServiceProposal(t, f)
	hidden := f.fx.Workspace(t, "Hidden target", "hidden-"+uuid.NewString())
	for _, target := range []string{hidden, uuid.NewString()} {
		p.CandidateID = uuid.NewString()
		p.RequestID = uuid.NewString()
		p.Target.WorkspaceID = target
		_, err := f.svc.ProposeKnowledge(collaborationContext(f.handler), f.issue.Issue.ID, p)
		if !collaborationErrorCode(err, "knowledge_target_not_found") {
			t.Fatalf("hidden/absent target differentiated: %v", err)
		}
		if f.fx.Count(t, `SELECT count(*) FROM collaboration_operation WHERE scope_id=$1 AND request_id=$2`, f.issue.Issue.ID, p.RequestID) != 0 {
			t.Fatal("invisible proposal left an operation")
		}
	}
}

func TestKnowledgeProposeHidesUnreadableSourceBeforeTargetResolution(t *testing.T) {
	f := newCollaborationMemoryFixture(t)
	p := knowledgeServiceProposal(t, f)
	target := f.fx.Workspace(t, "Caller owned target", "own-target-"+uuid.NewString())
	f.fx.Member(t, target, f.outsider, "owner")
	p.Target.WorkspaceID = target
	for _, source := range []string{util.UUIDToString(f.issue.Issue.ID), uuid.NewString()} {
		p.CandidateID = uuid.NewString()
		p.RequestID = uuid.NewString()
		p.Sources[0].IssueID = source
		if _, err := f.svc.ProposeKnowledge(collaborationContext(f.outsider), util.MustParseUUID(source), p); !collaborationErrorCode(err, "knowledge_candidate_not_found") {
			t.Fatalf("owned destination revealed hidden source existence: %v", err)
		}
	}
}

func TestKnowledgeSourceMemberCreatorWithoutParticipantCanProposeAndWithdraw(t *testing.T) {
	f := newCollaborationMemoryFixture(t)
	p := knowledgeServiceProposal(t, f)
	f.fx.Exec(t, `DELETE FROM issue_participant WHERE issue_id=$1 AND user_id=$2`, f.issue.Issue.ID, f.creator)
	if _, err := f.svc.ProposeKnowledge(f.ctx, f.issue.Issue.ID, p); err != nil {
		t.Fatal(err)
	}
	_, err := f.svc.WithdrawKnowledge(f.ctx, util.MustParseUUID(p.CandidateID), protocol.CollaborationKnowledgeDecisionRequest{ProtocolVersion: 1, RequestID: uuid.NewString(), ExpectedStateRevision: "1", Reason: "source owner withdrawal"})
	if err != nil {
		t.Fatal(err)
	}
	f.fx.Exec(t, `DELETE FROM member WHERE workspace_id=$1 AND user_id=$2`, f.workspace, f.creator)
	p.CandidateID = uuid.NewString()
	p.RequestID = uuid.NewString()
	if _, err = f.svc.ProposeKnowledge(f.ctx, f.issue.Issue.ID, p); err == nil {
		t.Fatal("creator without current read basis proposed")
	}
}

func TestKnowledgeApprovalRequiresCurrentHumanProjectLeadOrAdmin(t *testing.T) {
	f := newCollaborationMemoryFixture(t)
	p := knowledgeServiceProposal(t, f)
	f.fx.Member(t, f.workspace, f.outsider, "member")
	project := f.fx.Project(t, "Project", testutil.Cols{"lead_type": "agent", "lead_id": f.outsider})
	p.Target.Kind = "project"
	p.Target.ProjectID = project
	if _, err := f.svc.ProposeKnowledge(f.ctx, f.issue.Issue.ID, p); err != nil {
		t.Fatal(err)
	}
	request := knowledgeApproveRequest(p)
	id := util.MustParseUUID(p.CandidateID)
	if _, err := f.svc.ApproveKnowledge(collaborationContext(f.outsider), id, request); !collaborationErrorCode(err, "knowledge_permission_denied") {
		t.Fatalf("agent lead inherited human confirmation: %v", err)
	}
	f.fx.Exec(t, `UPDATE project SET lead_type='member' WHERE id=$1`, project)
	r, err := f.svc.ApproveKnowledge(collaborationContext(f.outsider), id, request)
	if err != nil || r.Code != "knowledge_approved" {
		t.Fatalf("human member lead: %+v %v", r, err)
	}
	for _, principal := range []auth.AuthenticatedPrincipal{{UserID: f.creator, Kind: auth.CredentialTask}, {UserID: f.creator, Kind: auth.CredentialPAT, AgentActor: true}, {UserID: f.creator, Kind: auth.CredentialJWT, Restricted: true}} {
		p.CandidateID = uuid.NewString()
		p.RequestID = uuid.NewString()
		if _, err := f.svc.ProposeKnowledge(auth.WithAuthenticatedPrincipal(context.Background(), principal), f.issue.Issue.ID, p); !collaborationErrorCode(err, "knowledge_permission_denied") {
			t.Fatalf("machine/restricted proposal: %v", err)
		}
	}
}

func TestKnowledgeRequestIDConflictsAcrossOperationKinds(t *testing.T) {
	for _, kind := range []string{"memory", "command"} {
		for _, knowledgeFirst := range []bool{false, true} {
			t.Run(kind+"/"+map[bool]string{false: "knowledge_second", true: "knowledge_first"}[knowledgeFirst], func(t *testing.T) {
				f := newCollaborationMemoryFixture(t)
				p := knowledgeServiceProposal(t, f)
				other := func() error {
					if kind == "memory" {
						_, err := f.svc.UpdateMemory(f.ctx, CollaborationMemoryUpdateParams{IssueID: f.issue.Issue.ID, RequestID: p.RequestID, ExpectedVersion: 1, Content: "new memory"})
						return err
					}
					request := f.request()
					request.RequestID = p.RequestID
					_, err := f.svc.SubmitCommand(f.ctx, CollaborationSubmitCommandParams{CollaborationQueueRequest: request, TargetID: f.targetID, InputEventID: util.MustParseUUID(uuid.NewString()), ExpectedTargetEpoch: 1, ExpectedConfigVersion: 1, Content: "one input"})
					return err
				}
				if knowledgeFirst {
					if _, err := f.svc.ProposeKnowledge(f.ctx, f.issue.Issue.ID, p); err != nil {
						t.Fatal(err)
					}
					if err := other(); !collaborationErrorCode(err, "idempotency_conflict") {
						t.Fatalf("other kind reuse: %v", err)
					}
				} else {
					if err := other(); err != nil {
						t.Fatal(err)
					}
					if _, err := f.svc.ProposeKnowledge(f.ctx, f.issue.Issue.ID, p); !collaborationErrorCode(err, "idempotency_conflict") {
						t.Fatalf("knowledge kind reuse: %v", err)
					}
				}
				if f.fx.Count(t, `SELECT count(*) FROM collaboration_operation WHERE scope_id=$1 AND actor_user_id=$2 AND request_id=$3`, f.issue.Issue.ID, f.creator, p.RequestID) != 1 {
					t.Fatal("cross-kind UUID forked the operation namespace")
				}
			})
		}
	}
}

func TestKnowledgeInvalidNewProposalRemainsUnsealedAndCannotFabricateRecovery(t *testing.T) {
	f := newCollaborationMemoryFixture(t)
	p := knowledgeServiceProposal(t, f)
	p.Sources[0].SHA256 = strings.Repeat("a", 64)
	if _, err := f.svc.ProposeKnowledge(f.ctx, f.issue.Issue.ID, p); !collaborationErrorCode(err, "knowledge_source_unavailable") {
		t.Fatalf("wrong source hash: %v", err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_candidate WHERE id=$1`, p.CandidateID) != 0 || f.fx.Count(t, `SELECT count(*) FROM collaboration_operation WHERE scope_id=$1 AND request_id=$2`, f.issue.Issue.ID, p.RequestID) != 0 || f.fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_invalidation WHERE target_workspace_id=$1`, f.workspace) != 0 {
		t.Fatal("invalid new proposal wrote candidate/operation/hint")
	}
	if _, err := f.svc.GetKnowledgeOperation(f.ctx, util.MustParseUUID(p.CandidateID), p.RequestID); !collaborationErrorCode(err, "operation_not_found") {
		t.Fatalf("unsealed failure fabricated a candidate state: %v", err)
	}
	p.Sources[0].SHA256 = protocol.CollaborationKnowledgeHash("PRIVATE_KNOWLEDGE_SOURCE")
	expired := time.Now().Add(-time.Second).UTC()
	p.ExpiresAt = &expired
	if _, err := f.svc.ProposeKnowledge(f.ctx, f.issue.Issue.ID, p); !collaborationErrorCode(err, "invalid_knowledge_request") {
		t.Fatalf("expired new proposal: %v", err)
	}
}

type knowledgeLostCommitStarter struct {
	TxStarter
	once *sync.Once
}

func (s knowledgeLostCommitStarter) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return knowledgeLostCommitTx{Tx: tx, once: s.once}, nil
}

type knowledgeLostCommitTx struct {
	pgx.Tx
	once *sync.Once
}

func (tx knowledgeLostCommitTx) Commit(ctx context.Context) error {
	if err := tx.Tx.Commit(ctx); err != nil {
		return err
	}
	lost := false
	tx.once.Do(func() { lost = true })
	if lost {
		return errors.New("test lost commit acknowledgement")
	}
	return nil
}

func TestKnowledgeCreateLostResponseUsesPreselectedCandidateID(t *testing.T) {
	f := newCollaborationMemoryFixture(t)
	p := knowledgeServiceProposal(t, f)
	f.svc.TxStarter = knowledgeLostCommitStarter{TxStarter: f.pool, once: &sync.Once{}}
	if _, err := f.svc.ProposeKnowledge(f.ctx, f.issue.Issue.ID, p); !collaborationErrorCode(err, "knowledge_unavailable") {
		t.Fatalf("unknown commit was treated as confirmed: %v", err)
	}
	r, err := f.svc.GetKnowledgeOperation(f.ctx, util.MustParseUUID(p.CandidateID), p.RequestID)
	if err != nil || r.Code != "knowledge_proposed" {
		t.Fatalf("lost response recovery: %+v %v", r, err)
	}
	replay, err := f.svc.ProposeKnowledge(f.ctx, f.issue.Issue.ID, p)
	a, _ := json.Marshal(r)
	b, _ := json.Marshal(replay)
	if err != nil || string(a) != string(b) || f.fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_candidate WHERE id=$1`, p.CandidateID) != 1 {
		t.Fatalf("unknown commit duplicated original candidate: %+v %v", replay, err)
	}
}

func TestKnowledgeTargetWithdrawalRecoveryAfterSourceAndTargetRoleLoss(t *testing.T) {
	f := newCollaborationMemoryFixture(t)
	p := knowledgeServiceProposal(t, f)
	target := f.fx.Workspace(t, "Target", "knowledge-target-"+uuid.NewString())
	f.fx.Member(t, target, f.creator, "member")
	f.fx.Member(t, target, f.handler, "owner")
	p.Target.WorkspaceID = target
	if _, err := f.svc.ProposeKnowledge(f.ctx, f.issue.Issue.ID, p); err != nil {
		t.Fatal(err)
	}
	id := util.MustParseUUID(p.CandidateID)
	if _, err := f.svc.ApproveKnowledge(collaborationContext(f.handler), id, knowledgeApproveRequest(p)); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.RemoveParticipant(f.ctx, CollaborationParticipantParams{CollaborationWorkflowRequest: workflowRequest(f, 1), TargetUserID: util.MustParseUUID(f.handler)}); err != nil {
		t.Fatal(err)
	}
	request := protocol.CollaborationKnowledgeDecisionRequest{ProtocolVersion: 1, RequestID: uuid.NewString(), ExpectedStateRevision: "2", Reason: "target no longer needs contribution"}
	r, err := f.svc.WithdrawKnowledge(collaborationContext(f.handler), id, request)
	if err != nil {
		t.Fatal(err)
	}
	f.fx.Exec(t, `DELETE FROM member WHERE workspace_id=$1 AND user_id=$2`, target, f.handler)
	recovered, err := f.svc.GetKnowledgeOperation(collaborationContext(f.handler), id, request.RequestID)
	a, _ := json.Marshal(r)
	b, _ := json.Marshal(recovered)
	if err != nil || string(a) != string(b) || strings.Contains(string(b), "PRIVATE_") || strings.Contains(string(b), f.workspace) {
		t.Fatalf("target-only receipt recovery: %s %v", b, err)
	}
	if _, err = f.svc.GetKnowledgeOperation(f.ctx, id, request.RequestID); !collaborationErrorCode(err, "operation_not_found") {
		t.Fatalf("another actor recovered private operation: %v", err)
	}
}
