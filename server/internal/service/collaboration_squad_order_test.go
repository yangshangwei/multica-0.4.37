package service

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/multica-ai/multica/server/internal/util"
)

type collaborationDelayedBegin struct {
	TxStarter
	ready, release chan struct{}
	once           sync.Once
}

func (s *collaborationDelayedBegin) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return nil, err
	}
	s.once.Do(func() {
		close(s.ready)
		select {
		case <-s.release:
		case <-ctx.Done():
		}
	})
	if err := ctx.Err(); err != nil {
		_ = tx.Rollback(context.Background())
		return nil, err
	}
	return tx, nil
}

func TestCollaborationSquadFinalLeaderFollowsSerializedTurnOrder(t *testing.T) {
	f, memberRuntime, member := newManagedSquadResourcesFixture(t, true)
	f.submitManaged(t, f.creator, "root instruction")
	f.resumeManaged(t)
	root, request, claim := f.claimManaged(t)
	token, nonce := permitSquadRun(t, f, root, claim)
	if _, err := f.svc.CreateRuntimeComment(context.Background(), root, token, CollaborationRuntimeCommentParams{
		CommentID: util.MustParseUUID(uuid.NewString()), Content: "[@member](mention://agent/" + util.UUIDToString(member) + ") evaluate the result",
	}); err != nil {
		t.Fatal(err)
	}
	finishSquadRun(t, f, root, claim, token, nonce, "success")
	memberRun, _, memberClaim := memberRuntime.claimManaged(t)
	memberToken, memberNonce := permitSquadRun(t, memberRuntime, memberRun, memberClaim)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	gate := &collaborationDelayedBegin{TxStarter: f.pool, ready: make(chan struct{}), release: make(chan struct{})}
	delayed := *f.svc
	delayed.TxStarter = gate
	type result struct {
		receipt CollaborationRuntimeCommentReceipt
		err     error
	}
	done := make(chan result, 1)
	go func() {
		receipt, err := delayed.CreateRuntimeComment(ctx, memberRun, memberToken, CollaborationRuntimeCommentParams{CommentID: util.MustParseUUID(uuid.NewString()), Content: "Second leader turn must reject the result"})
		done <- result{receipt, err}
	}()
	select {
	case <-gate.ready:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	first, err := f.svc.CreateRuntimeComment(ctx, memberRun, memberToken, CollaborationRuntimeCommentParams{CommentID: util.MustParseUUID(uuid.NewString()), Content: "First leader turn accepts the result"})
	if err != nil {
		t.Fatal(err)
	}
	firstClaim, err := f.svc.ClaimRuntimeRun(f.daemonCtx, first.ContinuationRunID, request)
	if err != nil {
		t.Fatal(err)
	}
	firstToken, firstNonce := permitSquadRun(t, f, first.ContinuationRunID, firstClaim)
	finishSquadRun(t, f, first.ContinuationRunID, firstClaim, firstToken, firstNonce, "success")
	close(gate.release)
	var second result
	select {
	case second = <-done:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	if second.err != nil {
		t.Fatal(second.err)
	}
	one, err := f.svc.Queries.GetCollaborationRuntimeRun(ctx, first.ContinuationRunID)
	if err != nil {
		t.Fatal(err)
	}
	two, err := f.svc.Queries.GetCollaborationRuntimeRun(ctx, second.receipt.ContinuationRunID)
	if err != nil || two.ID == one.ID || !two.CreatedAt.Time.Before(one.CreatedAt.Time) {
		t.Fatalf("delayed transaction did not establish reversed timestamps: first=%+v second=%+v err=%v", one.CreatedAt, two.CreatedAt, err)
	}
	finishSquadRun(t, memberRuntime, memberRun, memberClaim, memberToken, memberNonce, "success")
	last, _, lastClaim := f.claimManaged(t)
	if last != two.ID {
		t.Fatal("member completion replaced the queued final leader")
	}
	lastToken, lastNonce := permitSquadRun(t, f, last, lastClaim)
	finishSquadRun(t, f, last, lastClaim, lastToken, lastNonce, "failure")
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_execution_group WHERE id=$1 AND state='failed' AND settled_at IS NOT NULL`, two.GroupID) != 1 {
		t.Fatal("transaction timestamps selected an earlier leader's successful result")
	}
}
