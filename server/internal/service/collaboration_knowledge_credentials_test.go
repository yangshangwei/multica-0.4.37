package service

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func knowledgePATFixture(t *testing.T, f *collaborationQueueFixture) (context.Context, string, time.Time) {
	t.Helper()
	token, err := auth.GeneratePATToken()
	if err != nil {
		t.Fatal(err)
	}
	hash := auth.HashToken(token)
	expiry := time.Now().Add(10 * time.Second).UTC()
	id := f.fx.Insert(t, "personal_access_token", testutil.Cols{"user_id": f.creator, "name": "Knowledge test credential", "token_hash": hash, "token_prefix": "mul_test", "expires_at": expiry, "auth_version": 1})
	ctx := auth.WithAuthenticatedPrincipal(context.Background(), auth.AuthenticatedPrincipal{UserID: f.creator, Kind: auth.CredentialPAT, PATTokenHash: hash})
	if auth.PasswordMode() {
		ctx = auth.WithPasswordSession(ctx, auth.PasswordSession{UserID: f.creator, Version: 1, Kind: "pat"})
	}
	return ctx, id, expiry
}

func TestKnowledgePATReadsWritesAndRecoveryRecheckOriginalCredential(t *testing.T) {
	f := newCollaborationMemoryFixture(t)
	p := knowledgeServiceProposal(t, f)
	ctx, tokenID, expiry := knowledgePATFixture(t, f)
	if _, err := f.svc.ProposeKnowledge(ctx, f.issue.Issue.ID, p); err != nil {
		t.Fatal(err)
	}
	id := util.MustParseUUID(p.CandidateID)
	view, err := f.svc.KnowledgeCandidate(ctx, id)
	if err != nil || view.ValidUntil.After(expiry) {
		t.Fatalf("PAT read lease outlived its credential: %v %v", view.ValidUntil, err)
	}
	if _, err = f.svc.GetKnowledgeOperation(ctx, id, p.RequestID); err != nil {
		t.Fatal(err)
	}
	f.fx.Exec(t, `UPDATE personal_access_token SET revoked=true WHERE id=$1`, tokenID)
	for name, call := range map[string]func() error{
		"detail": func() error { _, err := f.svc.KnowledgeCandidate(ctx, id); return err },
		"source": func() error {
			_, err := f.svc.ListKnowledgeCandidates(ctx, f.issue.Issue.ID, KnowledgePageQuery{})
			return err
		},
		"target":   func() error { _, err := f.svc.ListTargetKnowledge(ctx, p.Target, KnowledgePageQuery{}); return err },
		"recovery": func() error { _, err := f.svc.GetKnowledgeOperation(ctx, id, p.RequestID); return err },
		"approval": func() error { _, err := f.svc.ApproveKnowledge(ctx, id, knowledgeApproveRequest(p)); return err },
	} {
		t.Run(name, func(t *testing.T) {
			if err := call(); !collaborationErrorCode(err, "knowledge_permission_denied") {
				t.Fatalf("cached PAT identity survived revocation: %v", err)
			}
		})
	}
	if _, err = f.svc.KnowledgeCandidate(f.ctx, id); err != nil {
		t.Fatal("PAT revocation incorrectly revoked the independent JWT", err)
	}
	f.fx.Exec(t, `UPDATE personal_access_token SET revoked=false,expires_at=now()-interval '1 second' WHERE id=$1`, tokenID)
	if _, err = f.svc.KnowledgeCandidate(ctx, id); !collaborationErrorCode(err, "knowledge_permission_denied") {
		t.Fatalf("expired PAT read: %v", err)
	}
	for _, principal := range []auth.AuthenticatedPrincipal{{UserID: f.creator, Kind: auth.CredentialPAT}, {UserID: f.handler, Kind: auth.CredentialPAT, PATTokenHash: auth.HashToken("unknown")}} {
		if _, err = f.svc.KnowledgeCandidate(auth.WithAuthenticatedPrincipal(context.Background(), principal), id); !collaborationErrorCode(err, "knowledge_permission_denied") {
			t.Fatalf("missing/wrong PAT provenance accepted: %v", err)
		}
	}
}

func TestKnowledgePATWriteHoldsCredentialUntilPublication(t *testing.T) {
	f := newCollaborationMemoryFixture(t)
	p := knowledgeServiceProposal(t, f)
	ctx, tokenID, _ := knowledgePATFixture(t, f)
	gate := newKnowledgeCommitGate(t, f.pool)
	f.svc.TxStarter = gate
	published := make(chan error, 1)
	go func() { _, err := f.svc.ProposeKnowledge(ctx, f.issue.Issue.ID, p); published <- err }()
	var holder uint32
	select {
	case holder = <-gate.entered:
	case err := <-published:
		t.Fatalf("PAT publication before commit gate: %v", err)
	case <-time.After(3 * time.Second):
		t.Fatal("PAT publication missed gate")
	}
	revoked := make(chan error, 1)
	go func() {
		_, err := f.svc.Queries.RevokePersonalAccessToken(f.ctx, db.RevokePersonalAccessTokenParams{ID: util.MustParseUUID(tokenID), UserID: util.MustParseUUID(f.creator)})
		revoked <- err
	}()
	knowledgeWaitForBlock(t, f.pool, holder)
	close(gate.release)
	if err := <-published; err != nil {
		t.Fatal(err)
	}
	if err := <-revoked; err != nil {
		t.Fatal(err)
	}
	p.CandidateID = uuid.NewString()
	p.RequestID = uuid.NewString()
	if _, err := f.svc.ProposeKnowledge(ctx, f.issue.Issue.ID, p); !collaborationErrorCode(err, "knowledge_permission_denied") {
		t.Fatalf("later PAT write survived revocation: %v", err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_candidate WHERE id=$1`, p.CandidateID) != 0 {
		t.Fatal("revoked PAT persisted a new candidate")
	}
}

// knowledgeWaitBehindLock reports whether the knowledge write queued on the
// holder's row lock. A write that finishes first did not wait.
func knowledgeWaitBehindLock(t *testing.T, f *collaborationQueueFixture, holder uint32, published <-chan error) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case err := <-published:
			t.Fatalf("knowledge write finished without waiting for the PAT row lock: %v", err)
		default:
		}
		var blocked bool
		if err := f.pool.QueryRow(context.Background(), `SELECT EXISTS (SELECT 1 FROM pg_stat_activity WHERE datname=current_database() AND $1::integer=ANY(pg_blocking_pids(pid)))`, int32(holder)).Scan(&blocked); err != nil {
			t.Fatal(err)
		}
		if blocked {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("knowledge write never queued on the PAT row lock")
}

// The middleware refreshes last_used_at asynchronously with the same row-lock
// mode as revocation. A knowledge write must wait that short update out
// instead of failing with 503 knowledge_busy.
func TestKnowledgePATWriteWaitsOutLastUsedUpdate(t *testing.T) {
	f := newCollaborationMemoryFixture(t)
	p := knowledgeServiceProposal(t, f)
	ctx, tokenID, _ := knowledgePATFixture(t, f)
	tx, err := f.pool.Begin(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err = f.svc.Queries.WithTx(tx).UpdatePersonalAccessTokenLastUsed(f.ctx, util.MustParseUUID(tokenID)); err != nil {
		t.Fatal(err)
	}
	published := make(chan error, 1)
	go func() { _, err := f.svc.ProposeKnowledge(ctx, f.issue.Issue.ID, p); published <- err }()
	knowledgeWaitBehindLock(t, f, tx.Conn().PgConn().PID(), published)
	if err = tx.Commit(f.ctx); err != nil {
		t.Fatal(err)
	}
	if err = <-published; err != nil {
		t.Fatalf("knowledge write failed behind a last_used_at update: %v", err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_candidate WHERE id=$1`, p.CandidateID) != 1 {
		t.Fatal("knowledge write behind a last_used_at update was not persisted")
	}
}

// Revocation that holds the PAT row first still serializes the knowledge
// write: the write waits for the commit, then fails closed on the revoked row.
func TestKnowledgePATWriteWaitsForConcurrentRevocation(t *testing.T) {
	f := newCollaborationMemoryFixture(t)
	p := knowledgeServiceProposal(t, f)
	ctx, tokenID, _ := knowledgePATFixture(t, f)
	tx, err := f.pool.Begin(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err = f.svc.Queries.WithTx(tx).RevokePersonalAccessToken(f.ctx, db.RevokePersonalAccessTokenParams{ID: util.MustParseUUID(tokenID), UserID: util.MustParseUUID(f.creator)}); err != nil {
		t.Fatal(err)
	}
	published := make(chan error, 1)
	go func() { _, err := f.svc.ProposeKnowledge(ctx, f.issue.Issue.ID, p); published <- err }()
	knowledgeWaitBehindLock(t, f, tx.Conn().PgConn().PID(), published)
	if err = tx.Commit(f.ctx); err != nil {
		t.Fatal(err)
	}
	if err = <-published; !collaborationErrorCode(err, "knowledge_permission_denied") {
		t.Fatalf("knowledge write behind a committed revocation: %v", err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_candidate WHERE id=$1`, p.CandidateID) != 0 {
		t.Fatal("revoked PAT persisted a candidate")
	}
}

// The PAT wait is bounded: a holder that never commits ends the write with the
// retryable busy refusal, and nothing is persisted.
func TestKnowledgePATWriteLockWaitIsBounded(t *testing.T) {
	f := newCollaborationMemoryFixture(t)
	p := knowledgeServiceProposal(t, f)
	ctx, tokenID, _ := knowledgePATFixture(t, f)
	tx, err := f.pool.Begin(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err = f.svc.Queries.WithTx(tx).UpdatePersonalAccessTokenLastUsed(f.ctx, util.MustParseUUID(tokenID)); err != nil {
		t.Fatal(err)
	}
	published := make(chan error, 1)
	go func() { _, err := f.svc.ProposeKnowledge(ctx, f.issue.Issue.ID, p); published <- err }()
	select {
	case err = <-published:
	case <-time.After(5 * time.Second):
		t.Fatal("PAT lock wait was not bounded")
	}
	if !collaborationErrorCode(err, "knowledge_busy") {
		t.Fatalf("held PAT row lock: %v", err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_candidate WHERE id=$1`, p.CandidateID) != 0 {
		t.Fatal("busy PAT write persisted a candidate")
	}
}

// The short PAT lock_timeout must not leak into later locks of the same
// transaction, whether the token is live or revoked.
func TestKnowledgePATLockRestoresLockTimeout(t *testing.T) {
	f := newCollaborationMemoryFixture(t)
	ctx, tokenID, _ := knowledgePATFixture(t, f)
	for _, revoked := range []bool{false, true} {
		f.fx.Exec(t, `UPDATE personal_access_token SET revoked=$2 WHERE id=$1`, tokenID, revoked)
		tx, err := f.pool.Begin(f.ctx)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = tx.Exec(f.ctx, `SET LOCAL lock_timeout = '7s'`); err != nil {
			t.Fatal(err)
		}
		q := f.svc.Queries.WithTx(tx)
		_, err = knowledgePAT(ctx, q, util.MustParseUUID(f.creator), true)
		if revoked != collaborationErrorCode(err, "knowledge_permission_denied") || !revoked && err != nil {
			t.Fatalf("revoked=%v: %v", revoked, err)
		}
		current, err := q.GetKnowledgeLockTimeout(f.ctx)
		_ = tx.Rollback(context.Background())
		if err != nil || current != "7s" {
			t.Fatalf("revoked=%v: lock_timeout after PAT lock = %q %v, want 7s", revoked, current, err)
		}
	}
}
