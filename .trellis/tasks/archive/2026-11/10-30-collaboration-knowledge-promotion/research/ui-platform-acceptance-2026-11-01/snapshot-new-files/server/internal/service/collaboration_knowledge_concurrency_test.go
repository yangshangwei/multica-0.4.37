package service

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/multica-ai/multica/server/internal/util"
)

type knowledgeBeforeSourceStarter struct {
	TxStarter
	once   *sync.Once
	before func()
}

func (s knowledgeBeforeSourceStarter) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return knowledgeBeforeSourceTx{Tx: tx, once: s.once, before: s.before}, nil
}

type knowledgeBeforeSourceTx struct {
	pgx.Tx
	once   *sync.Once
	before func()
}

func (tx knowledgeBeforeSourceTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	// Workflow edits lock every affected participant's user row. Interleave
	// before that shared actor fence, where the real editor can actually win.
	if strings.Contains(sql, "-- name: LockCollaborationUsers ") {
		tx.once.Do(tx.before)
	}
	return tx.Tx.Query(ctx, sql, args...)
}

type knowledgeCommitGate struct {
	TxStarter
	entered chan uint32
	release chan struct{}
	once    *sync.Once
}

func (s knowledgeCommitGate) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return knowledgeCommitGateTx{Tx: tx, gate: s}, nil
}

type knowledgeCommitGateTx struct {
	pgx.Tx
	gate knowledgeCommitGate
}

func (tx knowledgeCommitGateTx) Commit(ctx context.Context) error {
	tx.gate.once.Do(func() {
		tx.gate.entered <- tx.Conn().PgConn().PID()
		select {
		case <-tx.gate.release:
		case <-ctx.Done():
		}
	})
	return tx.Tx.Commit(ctx)
}
func newKnowledgeCommitGate(t *testing.T, pool *pgxpool.Pool) knowledgeCommitGate {
	t.Helper()
	g := knowledgeCommitGate{TxStarter: pool, entered: make(chan uint32, 1), release: make(chan struct{}), once: &sync.Once{}}
	t.Cleanup(func() {
		select {
		case <-g.release:
		default:
			close(g.release)
		}
	})
	return g
}
func knowledgeWaitForBlock(t *testing.T, pool *pgxpool.Pool, holder uint32) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		var blocked bool
		err := pool.QueryRow(context.Background(), `SELECT EXISTS (SELECT 1 FROM pg_stat_activity WHERE datname=current_database() AND $1::integer=ANY(pg_blocking_pids(pid)))`, int32(holder)).Scan(&blocked)
		if err != nil {
			t.Fatal(err)
		}
		if blocked {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("competing transaction never reached the observed PostgreSQL lock barrier")
}

func TestKnowledgeSourceProjectProposalRevalidatesCurrentRelationship(t *testing.T) {
	f := newCollaborationMemoryFixture(t)
	p := knowledgeServiceProposal(t, f)
	oldProject := util.MustParseUUID(f.fx.Project(t, "Original project"))
	newProject := util.MustParseUUID(f.fx.Project(t, "New project"))
	f.fx.Exec(t, `UPDATE issue SET project_id=$2 WHERE id=$1`, f.issue.Issue.ID, oldProject)
	p.Target.Kind = "project"
	p.Target.ProjectID = util.UUIDToString(oldProject)
	editor := NewCollaborationService(f.svc.Queries, f.pool, f.svc.Issues)
	f.svc.TxStarter = knowledgeBeforeSourceStarter{TxStarter: f.pool, once: &sync.Once{}, before: func() {
		if _, err := editor.EditIssue(f.ctx, CollaborationEditParams{CollaborationWorkflowRequest: workflowRequest(f, 1), ProjectID: &newProject}); err != nil {
			t.Fatalf("actual source edit before proposal actor fence: %v", err)
		}
	}}
	if _, err := f.svc.ProposeKnowledge(collaborationContext(f.handler), f.issue.Issue.ID, p); !collaborationErrorCode(err, "knowledge_target_not_found") {
		t.Fatalf("stale source-project reference was accepted: %v", err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_candidate WHERE id=$1`, p.CandidateID) != 0 {
		t.Fatal("stale relationship persisted a proposal")
	}
}

func TestKnowledgeSourceProjectEditAfterProposalKeepsOriginalTarget(t *testing.T) {
	f := newCollaborationMemoryFixture(t)
	p := knowledgeServiceProposal(t, f)
	oldProject := util.MustParseUUID(f.fx.Project(t, "Original project"))
	newProject := util.MustParseUUID(f.fx.Project(t, "New project"))
	f.fx.Exec(t, `UPDATE issue SET project_id=$2 WHERE id=$1`, f.issue.Issue.ID, oldProject)
	p.Target.Kind = "project"
	p.Target.ProjectID = util.UUIDToString(oldProject)
	gate := newKnowledgeCommitGate(t, f.pool)
	f.svc.TxStarter = gate
	proposed := make(chan error, 1)
	go func() {
		_, err := f.svc.ProposeKnowledge(collaborationContext(f.handler), f.issue.Issue.ID, p)
		proposed <- err
	}()
	var holder uint32
	select {
	case holder = <-gate.entered:
	case err := <-proposed:
		t.Fatalf("proposal before barrier: %v", err)
	case <-time.After(3 * time.Second):
		t.Fatal("proposal did not reach commit")
	}
	editor := NewCollaborationService(f.svc.Queries, f.pool, f.svc.Issues)
	edited := make(chan error, 1)
	go func() {
		_, err := editor.EditIssue(f.ctx, CollaborationEditParams{CollaborationWorkflowRequest: workflowRequest(f, 1), ProjectID: &newProject})
		edited <- err
	}()
	knowledgeWaitForBlock(t, f.pool, holder)
	close(gate.release)
	if err := <-proposed; err != nil {
		t.Fatal(err)
	}
	if err := <-edited; err != nil {
		t.Fatal(err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_candidate WHERE id=$1 AND target_project_id=$2`, p.CandidateID, oldProject) != 1 {
		t.Fatal("later source edit rewrote the immutable proposal target")
	}
}

func TestKnowledgeApprovalAndParticipantRemovalBothCommitOrders(t *testing.T) {
	for _, approvalFirst := range []bool{true, false} {
		t.Run(map[bool]string{true: "approval_first", false: "removal_first"}[approvalFirst], func(t *testing.T) {
			f := newCollaborationMemoryFixture(t)
			p := knowledgeServiceProposal(t, f)
			target := f.fx.Workspace(t, "Approval target", "knowledge-race-"+uuid.NewString())
			f.fx.Member(t, target, f.creator, "member")
			f.fx.Member(t, target, f.handler, "owner")
			p.Target.WorkspaceID = target
			if _, err := f.svc.ProposeKnowledge(f.ctx, f.issue.Issue.ID, p); err != nil {
				t.Fatal(err)
			}
			gate := newKnowledgeCommitGate(t, f.pool)
			first := NewCollaborationService(f.svc.Queries, gate, f.svc.Issues)
			second := NewCollaborationService(f.svc.Queries, f.pool, f.svc.Issues)
			approve := func(s *CollaborationService) error {
				_, err := s.ApproveKnowledge(collaborationContext(f.handler), util.MustParseUUID(p.CandidateID), knowledgeApproveRequest(p))
				return err
			}
			remove := func(s *CollaborationService) error {
				_, err := s.RemoveParticipant(f.ctx, CollaborationParticipantParams{CollaborationWorkflowRequest: workflowRequest(f, 1), TargetUserID: util.MustParseUUID(f.handler)})
				return err
			}
			firstResult := make(chan error, 1)
			secondResult := make(chan error, 1)
			go func() {
				if approvalFirst {
					firstResult <- approve(first)
				} else {
					firstResult <- remove(first)
				}
			}()
			var holder uint32
			select {
			case holder = <-gate.entered:
			case err := <-firstResult:
				t.Fatalf("first writer before barrier: %v", err)
			case <-time.After(3 * time.Second):
				t.Fatal("first writer did not reach commit")
			}
			go func() {
				if approvalFirst {
					secondResult <- remove(second)
				} else {
					secondResult <- approve(second)
				}
			}()
			knowledgeWaitForBlock(t, f.pool, holder)
			close(gate.release)
			if err := <-firstResult; err != nil {
				t.Fatal(err)
			}
			err := <-secondResult
			if approvalFirst && err != nil || !approvalFirst && !collaborationErrorCode(err, "knowledge_permission_denied") {
				t.Fatalf("second writer: %v", err)
			}
			want := 0
			if approvalFirst {
				want = 1
			}
			if f.fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_candidate WHERE id=$1 AND state='approved'`, p.CandidateID) != want {
				t.Fatal("approval ignored source authority linearization")
			}
			view, err := second.KnowledgeCandidate(collaborationContext(f.handler), util.MustParseUUID(p.CandidateID))
			if err != nil || view.Candidate.Availability != "restricted" || view.Candidate.Content != nil {
				t.Fatalf("removed actor retained source body: %+v %v", view, err)
			}
		})
	}
}

func TestKnowledgeCorrectionApprovalCannotReplaceNewerApprovedVersion(t *testing.T) {
	f := newCollaborationMemoryFixture(t)
	p := knowledgeServiceProposal(t, f)
	if _, err := f.svc.ProposeKnowledge(f.ctx, f.issue.Issue.ID, p); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.ApproveKnowledge(f.ctx, util.MustParseUUID(p.CandidateID), knowledgeApproveRequest(p)); err != nil {
		t.Fatal(err)
	}
	second := p
	second.CandidateID = uuid.NewString()
	second.RequestID = uuid.NewString()
	second.SupersedesCandidateID = &p.CandidateID
	if _, err := f.svc.ProposeKnowledge(f.ctx, f.issue.Issue.ID, second); err != nil {
		t.Fatal(err)
	}
	third := second
	third.CandidateID = uuid.NewString()
	third.RequestID = uuid.NewString()
	third.SupersedesCandidateID = &second.CandidateID
	if _, err := f.svc.ProposeKnowledge(f.ctx, f.issue.Issue.ID, third); err != nil {
		t.Fatal(err)
	}
	request := knowledgeApproveRequest(third)
	request.ExpectedVersion = "3"
	if _, err := f.svc.ApproveKnowledge(f.ctx, util.MustParseUUID(third.CandidateID), request); err != nil {
		t.Fatal(err)
	}
	request = knowledgeApproveRequest(second)
	request.ExpectedVersion = "2"
	if _, err := f.svc.ApproveKnowledge(f.ctx, util.MustParseUUID(second.CandidateID), request); !collaborationErrorCode(err, "knowledge_state_conflict") {
		t.Fatalf("older pending correction replaced the newer approved version: %v", err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_candidate WHERE lineage_id=$1 AND state='approved' AND version=3`, p.CandidateID) != 1 {
		t.Fatal("lineage approval regressed")
	}
}

func TestKnowledgeOwnerApprovalDoesNotContendWithProjectDescriptionUpdate(t *testing.T) {
	f := newCollaborationMemoryFixture(t)
	p := knowledgeServiceProposal(t, f)
	project := f.fx.Project(t, "Independent project edit")
	p.Target.Kind = "project"
	p.Target.ProjectID = project
	if _, err := f.svc.ProposeKnowledge(f.ctx, f.issue.Issue.ID, p); err != nil {
		t.Fatal(err)
	}
	gate := newKnowledgeCommitGate(t, f.pool)
	f.svc.TxStarter = gate
	approved := make(chan error, 1)
	go func() {
		_, err := f.svc.ApproveKnowledge(f.ctx, util.MustParseUUID(p.CandidateID), knowledgeApproveRequest(p))
		approved <- err
	}()
	select {
	case <-gate.entered:
	case err := <-approved:
		t.Fatalf("approval before commit gate: %v", err)
	case <-time.After(3 * time.Second):
		t.Fatal("approval missed gate")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, editErr := f.pool.Exec(ctx, `UPDATE project SET description='ordinary concurrent edit',revision=revision+1 WHERE id=$1`, project)
	close(gate.release)
	if err := <-approved; err != nil {
		t.Fatal(err)
	}
	if editErr != nil {
		t.Fatalf("owner approval held an unnecessary lead/description lock: %v", editErr)
	}
}
