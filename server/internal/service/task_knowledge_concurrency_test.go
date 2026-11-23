package service

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
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

// holdKnowledgeSiblingLock runs lock in its own transaction, the way claim,
// finalize or a delegated-subscriber writer holds it, until release is called.
// It reports the instant just before rollback, so a waiter that acquires the
// key afterwards always observes a later time.
func holdKnowledgeSiblingLock(t *testing.T, starter TxStarter, lock func(pgx.Tx) error) (held <-chan struct{}, release func() time.Time) {
	t.Helper()
	ready, stop, done := make(chan struct{}), make(chan struct{}), make(chan time.Time, 1)
	go func() {
		ctx := context.Background()
		tx, err := starter.Begin(ctx)
		if err != nil {
			t.Error(err)
			close(ready)
			done <- time.Now()
			return
		}
		if err = lock(tx); err != nil {
			t.Error(err)
		}
		close(ready)
		<-stop
		released := time.Now()
		_ = tx.Rollback(ctx)
		done <- released
	}()
	var once sync.Once
	var released time.Time
	release = func() time.Time {
		once.Do(func() {
			close(stop)
			released = <-done
		})
		return released
	}
	t.Cleanup(func() { release() })
	return ready, release
}

func holdManagedRuntimeNamespace(t *testing.T, f *taskKnowledgeFixture) (<-chan struct{}, func() time.Time) {
	t.Helper()
	runtime, err := f.tasks.Queries.GetAgentRuntime(f.ctx, f.task.RuntimeID)
	if err != nil {
		t.Fatal(err)
	}
	return holdKnowledgeSiblingLock(t, f.pool, func(tx pgx.Tx) error {
		_, err := LockManagedRuntime(f.ctx, f.tasks.runtimeLookup(f.tasks.Queries.WithTx(tx)), runtime)
		return err
	})
}

func storedKnowledgeClaim(t *testing.T, f *taskKnowledgeFixture) *taskKnowledgeClaim {
	t.Helper()
	row, err := f.tasks.Queries.GetAgentTask(f.ctx, f.task.ID)
	if err != nil {
		t.Fatal(err)
	}
	meta, err := decodeTaskKnowledge(row.KnowledgeContextVersion, row.KnowledgeDelivery)
	if err != nil {
		t.Fatal(err)
	}
	claim, err := taskKnowledgeClaimAt(meta, row.ClaimGeneration)
	if err != nil {
		t.Fatal(err)
	}
	return claim
}

// Claim and finalize hold the installation namespace with a blocking lock, and
// the delegated-subscriber writer holds the member fence the same way. Prepare
// takes both late as try-locks; a miss must re-run the whole transaction
// instead of ending the task with knowledge_busy.
func TestKnowledgePreparationWaitsOutSiblingNamespaceHolder(t *testing.T) {
	for _, holder := range []string{"installation_namespace", "member_fence"} {
		t.Run(holder, func(t *testing.T) {
			f := newTaskKnowledgeFixture(t)
			d := f.finalize(t, "")
			var held <-chan struct{}
			var release func() time.Time
			if holder == "installation_namespace" {
				held, release = holdManagedRuntimeNamespace(t, f)
			} else {
				held, release = holdKnowledgeSiblingLock(t, f.pool, func(tx pgx.Tx) error {
					return f.tasks.Queries.WithTx(tx).LockSubscriberWrites(f.ctx, db.LockSubscriberWritesParams{WorkspaceID: util.MustParseUUID(f.workspace), UserID: util.MustParseUUID(f.creator)})
				})
			}
			<-held
			type outcome struct {
				out      protocol.TaskKnowledgePrepareResponse
				err      error
				returned time.Time
			}
			done := make(chan outcome, 1)
			ctx, cancel := context.WithTimeout(f.ctx, 5*time.Second)
			defer cancel()
			started := time.Now()
			go func() {
				out, err := f.tasks.PrepareTaskKnowledge(ctx, f.caller, f.task.ID, f.prepareRequest(d))
				done <- outcome{out, err, time.Now()}
			}()
			var got outcome
			select {
			case got = <-done:
				release()
				t.Fatalf("preparation returned while the sibling held the key: err=%v after %s", got.err, got.returned.Sub(started))
			case <-time.After(300 * time.Millisecond):
			}
			released := release()
			select {
			case got = <-done:
			case <-time.After(3 * time.Second):
				t.Fatal("preparation did not finish after the sibling released the key")
			}
			if got.err != nil || len(got.out.Contributions) != 1 {
				t.Fatalf("lock contention ended the preparation: %v", got.err)
			}
			if !got.returned.After(released) || got.returned.Sub(started) > 3*time.Second {
				t.Fatalf("preparation did not wait out the sibling: returned %s after start, released %s after start", got.returned.Sub(started), released.Sub(started))
			}
			if claim := storedKnowledgeClaim(t, f); claim.Phase != "disclosed" || claim.Prepared == nil {
				t.Fatalf("retried preparation did not commit its disclosure: phase=%s", claim.Phase)
			}
		})
	}
}

func TestKnowledgeAuthorizationWaitsOutSiblingNamespaceHolder(t *testing.T) {
	f := newTaskKnowledgeFixture(t)
	d := f.finalize(t, "")
	p, err := f.tasks.PrepareTaskKnowledge(f.ctx, f.caller, f.task.ID, f.prepareRequest(d))
	if err != nil {
		t.Fatal(err)
	}
	held, release := holdManagedRuntimeNamespace(t, f)
	<-held
	type outcome struct {
		out      protocol.TaskKnowledgeAuthorization
		err      error
		returned time.Time
	}
	done := make(chan outcome, 1)
	ctx, cancel := context.WithTimeout(f.ctx, 5*time.Second)
	defer cancel()
	go func() {
		out, err := f.tasks.AuthorizeTaskKnowledgeLaunch(ctx, f.caller, f.task.ID, f.authorizeRequest(p, d))
		done <- outcome{out, err, time.Now()}
	}()
	select {
	case got := <-done:
		release()
		t.Fatalf("authorization returned while the sibling held the namespace: %v", got.err)
	case <-time.After(300 * time.Millisecond):
	}
	released := release()
	var got outcome
	select {
	case got = <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("authorization did not finish after the sibling released the namespace")
	}
	if got.err != nil || got.out.PrepareID != p.PrepareID || !got.returned.After(released) {
		t.Fatalf("lock contention ended the authorization: %v", got.err)
	}
	if attempts := len(storedKnowledgeClaim(t, f).Attempts); attempts != 1 {
		t.Fatalf("retried authorization stored %d attempts, want 1", attempts)
	}
}

type knowledgeBeginCounter struct {
	TxStarter
	begins atomic.Int32
}

func (c *knowledgeBeginCounter) Begin(ctx context.Context) (pgx.Tx, error) {
	c.begins.Add(1)
	return c.TxStarter.Begin(ctx)
}

// Sustained contention still ends in a bounded knowledge_busy: the retry has a
// budget, holds no lock while it waits and commits nothing.
func TestKnowledgePreparationNamespaceBusyBudgetIsBounded(t *testing.T) {
	f := newTaskKnowledgeFixture(t)
	d := f.finalize(t, "")
	counter := &knowledgeBeginCounter{TxStarter: f.pool}
	f.tasks.TxStarter = counter
	held, release := holdManagedRuntimeNamespace(t, f)
	<-held
	ctx, cancel := context.WithTimeout(f.ctx, 5*time.Second)
	defer cancel()
	started := time.Now()
	out, err := f.tasks.PrepareTaskKnowledge(ctx, f.caller, f.task.ID, f.prepareRequest(d))
	elapsed := time.Since(started)
	release()
	if !collaborationErrorCode(err, "knowledge_busy") || len(out.Contributions) != 0 {
		t.Fatalf("exhausted contention was not a bounded knowledge_busy: %v", err)
	}
	if begins := counter.begins.Load(); begins < 2 {
		t.Fatalf("contended preparation ran %d transaction(s), want a retry", begins)
	}
	if elapsed < 1200*time.Millisecond || elapsed > 2500*time.Millisecond {
		t.Fatalf("busy budget took %s, want 1.2s-2.5s", elapsed)
	}
	if claim := storedKnowledgeClaim(t, f); claim.Phase != "claimed" || claim.Prepared != nil {
		t.Fatalf("refused preparation committed disclosure metadata: phase=%s", claim.Phase)
	}
}
