package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/multica-ai/multica/server/internal/util"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

type knowledgeCommitBarrier struct {
	TxStarter
	entered chan struct{}
	release chan struct{}
}
type knowledgeBarrierTx struct {
	pgx.Tx
	owner *knowledgeCommitBarrier
}

func (b *knowledgeCommitBarrier) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := b.TxStarter.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &knowledgeBarrierTx{Tx: tx, owner: b}, nil
}
func (tx *knowledgeBarrierTx) Commit(ctx context.Context) error {
	close(tx.owner.entered)
	select {
	case <-tx.owner.release:
	case <-ctx.Done():
		return ctx.Err()
	}
	return tx.Tx.Commit(ctx)
}

func TestKnowledgePreparationVersusWithdrawAndSourceRevoke(t *testing.T) {
	for _, first := range []string{"withdraw", "prepare"} {
		t.Run(first, func(t *testing.T) {
			f := newTaskKnowledgeFixture(t)
			d := f.finalize(t, "")
			req := f.prepareRequest(d)
			if first == "withdraw" {
				f.withdraw(t, f.candidate.CandidateID)
				out, err := f.tasks.PrepareTaskKnowledge(f.ctx, f.caller, f.task.ID, req)
				if err != nil {
					t.Fatal(err)
				}
				if len(out.Contributions) != 0 {
					t.Fatal("withdraw committed first but its body was disclosed")
				}
				return
			}
			barrier := &knowledgeCommitBarrier{TxStarter: f.pool, entered: make(chan struct{}), release: make(chan struct{})}
			f.tasks.TxStarter = barrier
			type outcome struct {
				out protocol.TaskKnowledgePrepareResponse
				err error
			}
			done := make(chan outcome, 1)
			ctx, cancel := context.WithTimeout(f.ctx, 5*time.Second)
			defer cancel()
			go func() {
				out, err := f.tasks.PrepareTaskKnowledge(ctx, f.caller, f.task.ID, req)
				done <- outcome{out, err}
			}()
			select {
			case <-barrier.entered:
			case <-time.After(3 * time.Second):
				t.Fatal("preparation did not reach commit barrier")
			}
			select {
			case value := <-done:
				t.Fatalf("body returned before commit: %+v", value)
			default:
			}
			withdraw := protocol.CollaborationKnowledgeDecisionRequest{ProtocolVersion: 1, RequestID: uuid.NewString(), ExpectedStateRevision: "2", Reason: "concurrent withdrawal"}
			withdrawDone := make(chan error, 1)
			go func() {
				_, err := f.svc.WithdrawKnowledge(ctx, util.MustParseUUID(f.candidate.CandidateID), withdraw)
				withdrawDone <- err
			}()
			retryWithdrawal := false
			// Human writers can wait at their common root user lock. They may
			// also refuse a later NOWAIT conflict; neither may commit early.
			select {
			case err := <-withdrawDone:
				if !collaborationErrorCode(err, "knowledge_busy") {
					t.Fatalf("withdraw crossed held preparation locks: %v", err)
				}
				retryWithdrawal = true
			case <-time.After(50 * time.Millisecond):
			}
			close(barrier.release)
			value := <-done
			if value.err != nil || len(value.out.Contributions) != 1 {
				t.Fatalf("authorized commit lost its disclosed result: %v", value.err)
			}
			f.tasks.TxStarter = f.pool
			var err error
			if retryWithdrawal {
				_, err = f.svc.WithdrawKnowledge(f.ctx, util.MustParseUUID(f.candidate.CandidateID), withdraw)
			} else {
				err = <-withdrawDone
			}
			if err != nil {
				t.Fatalf("withdraw did not complete after disclosure commit: %v", err)
			}
			if _, err = f.tasks.AuthorizeTaskKnowledgeLaunch(f.ctx, f.caller, f.task.ID, f.authorizeRequest(value.out, d)); !collaborationErrorCode(err, "knowledge_context_changed") {
				t.Fatalf("post-disclosure withdrawal permitted a launch: %v", err)
			}
		})
	}
}

func TestKnowledgePreparationCompleteEnvelopeBudgetRollsBackDisclosure(t *testing.T) {
	f := newTaskKnowledgeFixture(t)
	project := f.fx.Project(t, "Knowledge envelope target")
	f.fx.Exec(t, `UPDATE issue SET project_id=$2 WHERE id=ANY($1::uuid[])`, []string{util.UUIDToString(f.task.IssueID), util.UUIDToString(f.issue.Issue.ID)}, project)
	for _, kind := range []string{"workspace", "project"} {
		target := protocol.CollaborationKnowledgeTarget{Kind: kind, WorkspaceID: f.workspace}
		if kind == "project" {
			target.ProjectID = project
		}
		items := make([]protocol.CollaborationKnowledgeContribution, 16)
		proposals := make([]protocol.CollaborationKnowledgeProposal, 16)
		for i := range items {
			p := f.candidate
			p.CandidateID = uuid.NewString()
			p.RequestID = uuid.NewString()
			p.Target = target
			p.Content = ""
			proposals[i] = p
			items[i] = protocol.CollaborationKnowledgeContribution{CandidateID: p.CandidateID, Version: "1", ContentSHA256: strings.Repeat("a", 64), Target: target, SourceIssueID: util.UUIDToString(f.issue.Issue.ID), Summary: p.Summary, Sources: p.Sources, Evidence: p.Evidence, Applicability: p.Applicability}
		}
		start := 0
		if kind == "workspace" {
			row, err := f.svc.Queries.GetKnowledgeCandidate(f.ctx, util.MustParseUUID(f.candidate.CandidateID))
			if err != nil {
				t.Fatal(err)
			}
			items[0], err = knowledgeContribution(row)
			if err != nil {
				t.Fatal(err)
			}
			start = 1
		}
		material, _, err := protocol.CollaborationKnowledgeSizes(items)
		if err != nil {
			t.Fatal(err)
		}
		remaining := protocol.CollaborationKnowledgeMaxTargetBytes - 128 - material
		for i := start; i < len(items); i++ {
			size := remaining / (len(items) - i)
			remaining -= size
			p := proposals[i]
			p.Content = strings.Repeat("x", size)
			if _, err = f.svc.ProposeKnowledge(f.ctx, f.issue.Issue.ID, p); err != nil {
				t.Fatal(err)
			}
			if _, err = f.svc.ApproveKnowledge(f.ctx, util.MustParseUUID(p.CandidateID), protocol.CollaborationKnowledgeApproveRequest{ProtocolVersion: 1, RequestID: uuid.NewString(), ExpectedStateRevision: "1", ExpectedVersion: "1", ExpectedContentSHA256: protocol.CollaborationKnowledgeHash(p.Content)}); err != nil {
				t.Fatal(err)
			}
		}
	}
	var err error
	f.task, err = f.tasks.Queries.GetAgentTask(f.ctx, f.task.ID)
	if err != nil {
		t.Fatal(err)
	}
	d := f.finalize(t, "")
	facts, _, _, err := f.tasks.taskKnowledgeFacts(f.ctx, f.tasks.Queries, f.task)
	if err != nil {
		t.Fatal(err)
	}
	resolution, err := f.svc.ResolveApprovedKnowledge(f.ctx, f.svc.Queries, facts)
	if err != nil {
		t.Fatal(err)
	}
	if len(resolution.Contributions) != 32 || protocol.ValidateCollaborationKnowledgeRuntimeBudget(resolution.Contributions, 0) != nil {
		t.Fatal("fixture must fit the contribution-only budget")
	}
	out, err := f.tasks.PrepareTaskKnowledge(f.ctx, f.caller, f.task.ID, f.prepareRequest(d))
	if !collaborationErrorCode(err, "knowledge_unavailable") || len(out.Contributions) != 0 {
		t.Fatalf("oversized complete response disclosed body: %v", err)
	}
	row, err := f.tasks.Queries.GetAgentTask(f.ctx, f.task.ID)
	if err != nil {
		t.Fatal(err)
	}
	meta, err := decodeTaskKnowledge(row.KnowledgeContextVersion, row.KnowledgeDelivery)
	if err != nil {
		t.Fatal(err)
	}
	if meta.Claims[0].Prepared != nil || meta.Claims[0].Phase != "claimed" {
		t.Fatal("rejected response committed disclosure metadata")
	}
}

func TestKnowledgePreparationHoldsConsumerProjectIdentity(t *testing.T) {
	f := newTaskKnowledgeFixture(t)
	d := f.finalize(t, "")
	tx, err := f.pool.Begin(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(f.ctx)
	if _, err = tx.Exec(f.ctx, `SELECT id FROM issue WHERE id=$1 FOR UPDATE`, f.task.IssueID); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(f.ctx, time.Second)
	defer cancel()
	if _, err = f.tasks.PrepareTaskKnowledge(ctx, f.caller, f.task.ID, f.prepareRequest(d)); !collaborationErrorCode(err, "knowledge_busy") {
		t.Fatalf("consumer scope changed outside the preparation fence: %v", err)
	}
}
