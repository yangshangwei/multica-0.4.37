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
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/realtime"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func newKnowledgeInvalidationFixture(t *testing.T) (*collaborationQueueFixture, *CollaborationKnowledgeInvalidation) {
	t.Helper()
	f := newCollaborationMemoryFixture(t)
	knowledgeInvalidationCleanup(t, f)
	worker := &CollaborationKnowledgeInvalidation{Queries: f.svc.Queries, TxStarter: f.pool, Publisher: &collaborationPublisher{}}
	// Pending rows intentionally survive fixture parent deletion. Finish those
	// older jobs through the real drainer before observing this test's frames.
	finishKnowledgeTestScans(t, f, worker)
	for {
		n, err := worker.DeliverPending(f.ctx, 100)
		if err != nil {
			t.Fatal(err)
		}
		if n == 0 {
			break
		}
	}
	worker.Publisher = &collaborationPublisher{}
	return f, worker
}

func knowledgeInvalidationCleanup(t *testing.T, f *collaborationQueueFixture) {
	t.Helper()
	f.fx.Cleanup(t, `DELETE FROM collaboration_knowledge_target WHERE workspace_id=$1`, f.workspace)
	f.fx.Cleanup(t, `DELETE FROM collaboration_knowledge_invalidation WHERE state <> 'pending' AND (target_workspace_id=$1 OR scope_id IN ($1,$2))`, f.workspace, f.issue.Issue.ID)
}

// Generated storage operations seed immutable history. Behavioral assertions
// below drive the real purge/authority/worker transaction owners.
func seedKnowledgeInvalidationCandidate(t *testing.T, f *collaborationQueueFixture, published bool, project pgtype.UUID) db.CollaborationKnowledgeCandidate {
	t.Helper()
	q := f.svc.Queries
	kind, target := "workspace", util.MustParseUUID(f.workspace)
	if project.Valid {
		kind, target = "project", project
	}
	row, err := q.CreateKnowledgeCandidate(f.ctx, db.CreateKnowledgeCandidateParams{
		ID: pgtype.UUID{Bytes: uuid.New(), Valid: true}, SourceWorkspaceID: util.MustParseUUID(f.workspace), SourceIssueID: f.issue.Issue.ID,
		TargetKind: kind, TargetWorkspaceID: util.MustParseUUID(f.workspace), TargetProjectID: project,
		Content: "PRIVATE_DERIVED_BODY", Summary: "PRIVATE_SUMMARY", ContentSha256: protocol.CollaborationKnowledgeHash("PRIVATE_DERIVED_BODY"),
		Sources: []byte(`[{"kind":"memory"}]`), Evidence: []byte(`[]`), Applicability: []byte(`{"surfaces":["issue"]}`), ProposedByUserID: util.MustParseUUID(f.creator),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !published {
		return row
	}
	if _, err = q.InitializeKnowledgeTarget(f.ctx, db.InitializeKnowledgeTargetParams{TargetKind: kind, TargetID: target, WorkspaceID: util.MustParseUUID(f.workspace)}); err != nil {
		t.Fatal(err)
	}
	current, err := q.GetKnowledgeTarget(f.ctx, db.GetKnowledgeTargetParams{TargetKind: kind, TargetID: target})
	if err != nil {
		t.Fatal(err)
	}
	advanced, err := q.AdvanceKnowledgeTarget(f.ctx, db.AdvanceKnowledgeTargetParams{TargetKind: kind, TargetID: target, KnowledgeRevision: current.KnowledgeRevision})
	if err != nil {
		t.Fatal(err)
	}
	row, err = q.ApproveKnowledgeCandidate(f.ctx, db.ApproveKnowledgeCandidateParams{ID: row.ID, ActorID: util.MustParseUUID(f.creator), ExpectedVersion: 1, ExpectedStateRevision: 1, ExpectedContentSha256: row.ContentSha256, TargetRevision: pgtype.Int8{Int64: advanced.KnowledgeRevision, Valid: true}})
	if err != nil {
		t.Fatal(err)
	}
	return row
}

func enqueueKnowledgeTestScan(t *testing.T, f *collaborationQueueFixture, scope string, scopeID pgtype.UUID) pgtype.UUID {
	t.Helper()
	cause := pgtype.UUID{Bytes: uuid.New(), Valid: true}
	if err := EnqueueKnowledgeSourceInvalidation(f.ctx, f.svc.Queries, cause, scope, scopeID); err != nil {
		t.Fatal(err)
	}
	f.fx.Cleanup(t, `DELETE FROM collaboration_knowledge_invalidation WHERE state <> 'pending' AND cause_id=$1`, cause)
	return cause
}

func finishKnowledgeTestScans(t *testing.T, f *collaborationQueueFixture, worker *CollaborationKnowledgeInvalidation) {
	t.Helper()
	for range 100 {
		n, err := worker.ScanPending(f.ctx, 100)
		if err != nil {
			t.Fatal(err)
		}
		if n == 0 {
			return
		}
	}
	t.Fatal("bounded scans never completed")
}

func TestKnowledgePendingChangesDoNotSignalTarget(t *testing.T) {
	f, worker := newKnowledgeInvalidationFixture(t)
	seedKnowledgeInvalidationCandidate(t, f, false, pgtype.UUID{})
	cause := enqueueKnowledgeTestScan(t, f, "source_issue", f.issue.Issue.ID)
	finishKnowledgeTestScans(t, f, worker)
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_invalidation WHERE cause_id=$1 AND state='done'`, cause) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_invalidation WHERE target_workspace_id=$1`, f.workspace) != 0 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_target WHERE workspace_id=$1`, f.workspace) != 0 {
		t.Fatal("pending-only source activity disclosed a target or failed to complete")
	}
}

func TestKnowledgeSourcePurgeAppendsIntentWithoutTargetLocks(t *testing.T) {
	for _, admin := range []bool{false, true} {
		t.Run(map[bool]string{false: "human", true: "admin"}[admin], func(t *testing.T) {
			var f *collaborationQueueFixture
			var af *adminCollaborationFixture
			if admin {
				af = newAdminCollaborationFixture(t, func(f *collaborationQueueFixture) {
					f.memoryUpdate(t, f.creator, 0, "PRIVATE_ORIGINAL")
					f.memoryUpdate(t, f.creator, 1, "replacement")
				})
				f = af.collaborationQueueFixture
				knowledgeInvalidationCleanup(t, f)
				f.fx.Exec(t, `UPDATE "user" SET disabled_at=now() WHERE id=$1`, f.creator)
			} else {
				f, _ = newKnowledgeInvalidationFixture(t)
				f.memoryUpdate(t, f.creator, 0, "PRIVATE_ORIGINAL")
				f.memoryUpdate(t, f.creator, 1, "replacement")
			}
			seedKnowledgeInvalidationCandidate(t, f, true, pgtype.UUID{})
			lock, err := f.pool.Begin(f.ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer lock.Rollback(context.Background())
			if _, err = f.svc.Queries.WithTx(lock).LockKnowledgeTargetForWrite(f.ctx, db.LockKnowledgeTargetForWriteParams{TargetKind: "workspace", TargetID: util.MustParseUUID(f.workspace)}); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(f.ctx, 2*time.Second)
			defer cancel()
			if admin {
				ctx = adminCollaborationContext(af.admin, 1)
				ctx, cancel = context.WithTimeout(ctx, 2*time.Second)
				defer cancel()
				r, err := af.actions.PurgeMemoryVersion(ctx, AdminCollaborationMemoryPurgeParams{OrganizationID: af.org, IssueID: f.issue.Issue.ID, IdempotencyKey: pgtype.UUID{Bytes: uuid.New(), Valid: true}, Version: 1, Reason: "Dispose reported content", RequestID: uuid.NewString()})
				if err != nil || r.MemoryResult == nil || r.MemoryResult.Code != "memory_purged" {
					t.Fatalf("admin purge waited for target: %+v %v", r, err)
				}
			} else {
				r, err := f.svc.PurgeMemoryVersion(ctx, CollaborationMemoryPurgeParams{IssueID: f.issue.Issue.ID, RequestID: uuid.NewString(), Version: 1, Reason: "Dispose reported content"})
				if err != nil || r.Code != "memory_purged" {
					t.Fatalf("human purge waited for target: %+v %v", r, err)
				}
			}
			if f.fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_invalidation k JOIN collaboration_event e ON e.id=k.cause_id WHERE k.scope_id=$1 AND e.kind='memory_changed' AND e.payload->>'action'='purge'`, f.issue.Issue.ID) != 1 {
				t.Fatal("purge did not atomically retain exactly one original-event scan")
			}
			worker := &CollaborationKnowledgeInvalidation{Queries: f.svc.Queries, TxStarter: f.pool}
			if _, err = worker.ScanPending(f.ctx, 100); err == nil {
				t.Fatal("busy target reported success")
			}
			if f.fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_invalidation WHERE scope_id=$1 AND state='pending' AND scan_cursor IS NULL AND scan_started_at IS NOT NULL`, f.issue.Issue.ID) != 1 {
				t.Fatal("busy scan skipped its candidate or forgot its fixed boundary")
			}
			if err = lock.Rollback(f.ctx); err != nil {
				t.Fatal(err)
			}
			finishKnowledgeTestScans(t, f, worker)
			if f.fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_target WHERE workspace_id=$1 AND knowledge_revision=2`, f.workspace) != 1 {
				t.Fatal("purged published target was not invalidated on retry")
			}
		})
	}
}

func TestKnowledgeFanoutNeverWaitsOnSourceWriter(t *testing.T) {
	f, worker := newKnowledgeInvalidationFixture(t)
	candidate := seedKnowledgeInvalidationCandidate(t, f, true, pgtype.UUID{})
	enqueueKnowledgeTestScan(t, f, "source_issue", f.issue.Issue.ID)
	lock, err := f.pool.Begin(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Rollback(context.Background())
	for _, query := range []string{`SELECT id FROM issue WHERE id=$1 FOR UPDATE`, `SELECT issue_id FROM issue_collaboration WHERE issue_id=$1 FOR UPDATE`} {
		if _, err = lock.Exec(f.ctx, query, f.issue.Issue.ID); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = lock.Exec(f.ctx, `SELECT id FROM collaboration_knowledge_candidate WHERE id=$1 FOR UPDATE`, candidate.ID); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(f.ctx, 2*time.Second)
	defer cancel()
	if _, err = worker.ScanPending(ctx, 100); err != nil {
		t.Fatalf("scanner acquired a reverse source/candidate lock: %v", err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_target WHERE workspace_id=$1 AND knowledge_revision=2`, f.workspace) != 1 {
		t.Fatal("no target invalidation while source writer was held")
	}
}

func TestKnowledgeSourceDeletionRetainsInvalidationRouting(t *testing.T) {
	f, worker := newKnowledgeInvalidationFixture(t)
	seedKnowledgeInvalidationCandidate(t, f, true, pgtype.UUID{})
	cause := enqueueKnowledgeTestScan(t, f, "source_issue", f.issue.Issue.ID)
	// Source hard deletion is currently refused by the human API. Simulate
	// retained-history maintenance to prove delivery has no live source join.
	f.fx.Exec(t, `DELETE FROM issue_collaboration WHERE issue_id=$1`, f.issue.Issue.ID)
	f.fx.Exec(t, `DELETE FROM issue WHERE id=$1`, f.issue.Issue.ID)
	finishKnowledgeTestScans(t, f, worker)
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_invalidation WHERE cause_id=$1 AND kind='target_hint'`, cause) != 1 {
		t.Fatal("source deletion lost retained candidate routing")
	}
	if _, err := worker.DeliverPending(f.ctx, 100); err != nil {
		t.Fatal(err)
	}
}

func TestKnowledgeDeletedTargetIsNotRecreatedByPendingFanout(t *testing.T) {
	f, worker := newKnowledgeInvalidationFixture(t)
	project := util.MustParseUUID(f.fx.Project(t, "Target project"))
	seedKnowledgeInvalidationCandidate(t, f, true, project)
	cause := enqueueKnowledgeTestScan(t, f, "source_issue", f.issue.Issue.ID)
	deleted := pgtype.UUID{Bytes: uuid.New(), Valid: true}
	tx, err := f.pool.Begin(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	if _, err = tx.Exec(f.ctx, `SELECT id FROM project WHERE id=$1 FOR UPDATE`, project); err != nil {
		t.Fatal(err)
	}
	if err = DeleteKnowledgeTargetsInTx(f.ctx, f.svc.Queries.WithTx(tx), deleted, util.MustParseUUID(f.workspace), project); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(f.ctx, `DELETE FROM project WHERE id=$1`, project); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(f.ctx); err != nil {
		t.Fatal(err)
	}
	finishKnowledgeTestScans(t, f, worker)
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_target WHERE target_id=$1`, project) != 0 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_invalidation WHERE cause_id=$1 AND kind='target_hint' AND knowledge_revision=2`, deleted) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_invalidation WHERE cause_id=$1 AND state='done'`, cause) != 1 {
		t.Fatal("deleted target was recreated, lost its final hint or stranded its scan")
	}
	if _, err = worker.DeliverPending(f.ctx, 100); err != nil {
		t.Fatal(err)
	}
	publisher := worker.Publisher.(*collaborationPublisher)
	if len(publisher.frames) == 0 || publisher.frames[len(publisher.frames)-1].id != f.workspace {
		t.Fatal("deleted project hint did not use retained workspace routing")
	}
}

type knowledgeCommitFaultStarter struct {
	TxStarter
	commit bool
}

func (s knowledgeCommitFaultStarter) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := s.TxStarter.Begin(ctx)
	return knowledgeCommitFaultTx{Tx: tx, commit: s.commit}, err
}

type knowledgeCommitFaultTx struct {
	pgx.Tx
	commit bool
}

func (tx knowledgeCommitFaultTx) Commit(ctx context.Context) error {
	if tx.commit {
		if err := tx.Tx.Commit(ctx); err != nil {
			return err
		}
	} else if err := tx.Tx.Rollback(ctx); err != nil {
		return err
	}
	return errors.New("commit acknowledgement unavailable")
}

func TestKnowledgeFanoutCrashAndReplayPreserveCursorAndRevision(t *testing.T) {
	f, worker := newKnowledgeInvalidationFixture(t)
	for range 3 {
		seedKnowledgeInvalidationCandidate(t, f, true, pgtype.UUID{})
	}
	cause := enqueueKnowledgeTestScan(t, f, "source_issue", f.issue.Issue.ID)
	// Persist the boundary, then interrupt one actual candidate transaction.
	if _, err := worker.ScanPending(f.ctx, 1); err != nil {
		t.Fatal(err)
	}
	worker.TxStarter = knowledgeCommitFaultStarter{TxStarter: f.pool}
	if _, err := worker.ScanPending(f.ctx, 1); err == nil {
		t.Fatal("rolled-back scan reported success")
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_target WHERE workspace_id=$1 AND knowledge_revision=3`, f.workspace) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_invalidation WHERE cause_id=$1 AND scan_cursor IS NOT NULL`, cause) != 0 {
		t.Fatal("rollback committed a revision/hint/cursor prefix")
	}
	worker.TxStarter = knowledgeCommitFaultStarter{TxStarter: f.pool, commit: true}
	if _, err := worker.ScanPending(f.ctx, 1); err == nil {
		t.Fatal("lost commit acknowledgement reported success")
	}
	// Two restarted workers may race on the job/target. The database owns the
	// dedupe and cursor; no in-memory ownership survives the restart.
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w := &CollaborationKnowledgeInvalidation{Queries: f.svc.Queries, TxStarter: f.pool}
			_, _ = w.ScanPending(f.ctx, 100)
		}()
	}
	wg.Wait()
	worker.TxStarter = f.pool
	finishKnowledgeTestScans(t, f, worker)
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_target WHERE workspace_id=$1 AND knowledge_revision=4`, f.workspace) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_invalidation WHERE cause_id=$1 AND kind='target_hint'`, cause) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_invalidation WHERE cause_id=$1 AND state='done' AND scan_cursor IS NOT NULL`, cause) != 1 {
		t.Fatal("restart or competing worker repeated/lost target invalidation")
	}
}

func TestKnowledgeTargetHintRelayFailureRetainsStableID(t *testing.T) {
	for _, committed := range []bool{false, true} {
		t.Run(map[bool]string{false: "rollback", true: "lost_ack"}[committed], func(t *testing.T) {
			f, worker := newKnowledgeInvalidationFixture(t)
			seedKnowledgeInvalidationCandidate(t, f, true, pgtype.UUID{})
			cause := enqueueKnowledgeTestScan(t, f, "source_issue", f.issue.Issue.ID)
			finishKnowledgeTestScans(t, f, worker)
			publisher := worker.Publisher.(*collaborationPublisher)
			publisher.fail = true
			if _, err := worker.DeliverPending(f.ctx, 100); err == nil {
				t.Fatal("relay failure reported success")
			}
			original := publisher.frames[len(publisher.frames)-1]
			if f.fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_invalidation WHERE cause_id=$1 AND kind='target_hint' AND state='pending'`, cause) != 1 {
				t.Fatal("failed publish marked delivered")
			}
			publisher.fail = false
			worker.TxStarter = knowledgeCommitFaultStarter{TxStarter: f.pool, commit: committed}
			if _, err := worker.DeliverPending(f.ctx, 100); err == nil {
				t.Fatal("unknown commit reported acknowledged delivery")
			}
			worker.TxStarter = f.pool
			if _, err := worker.DeliverPending(f.ctx, 100); err != nil {
				t.Fatal(err)
			}
			for _, frame := range publisher.frames {
				if frame.event != original.event || string(frame.data) != string(original.data) {
					t.Fatal("retry changed the stable hint ID or revision")
				}
			}
			if f.fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_invalidation WHERE cause_id=$1 AND kind='target_hint' AND state='delivered'`, cause) != 1 {
				t.Fatal("restarted delivery did not settle the original hint")
			}
		})
	}
}

func TestKnowledgeTargetHintHasOnlyPublicTargetFields(t *testing.T) {
	f, worker := newKnowledgeInvalidationFixture(t)
	candidate := seedKnowledgeInvalidationCandidate(t, f, true, pgtype.UUID{})
	cause := enqueueKnowledgeTestScan(t, f, "source_issue", f.issue.Issue.ID)
	finishKnowledgeTestScans(t, f, worker)
	if _, err := worker.DeliverPending(f.ctx, 100); err != nil {
		t.Fatal(err)
	}
	frames := worker.Publisher.(*collaborationPublisher).frames
	if len(frames) != 1 {
		t.Fatalf("expected actual target publication, got %d frames", len(frames))
	}
	frame := frames[0]
	var decoded struct {
		Type    string            `json:"type"`
		Payload map[string]string `json:"payload"`
	}
	if err := json.Unmarshal(frame.data, &decoded); err != nil || decoded.Type != protocol.EventCollaborationKnowledgeChanged || len(decoded.Payload) != 3 ||
		decoded.Payload["target_kind"] != "workspace" || decoded.Payload["target_id"] != f.workspace || decoded.Payload["knowledge_revision"] != "2" || frame.scope != realtime.ScopeWorkspace || frame.id != f.workspace || !strings.HasSuffix(frame.event, ":target") {
		t.Fatalf("incorrect target frame: %+v %v", decoded, err)
	}
	for _, private := range []string{"PRIVATE_", util.UUIDToString(candidate.ID), util.UUIDToString(cause), util.UUIDToString(f.issue.Issue.ID), f.creator, candidate.ContentSha256} {
		if strings.Contains(string(frame.data), private) {
			t.Fatal("target payload disclosed private provenance")
		}
	}
}

func TestKnowledgeAuthorityChangesFanOutWithoutCurrentMembershipJoin(t *testing.T) {
	f, worker := newKnowledgeInvalidationFixture(t)
	seedKnowledgeInvalidationCandidate(t, f, true, pgtype.UUID{})
	r, err := f.svc.RemoveParticipant(f.ctx, CollaborationParticipantParams{CollaborationWorkflowRequest: CollaborationWorkflowRequest{CollaborationQueueRequest: f.request(), ExpectedCollaborationRevision: f.issue.Revision}, TargetUserID: util.MustParseUUID(f.handler)})
	if err != nil || r.Code != "participant_removed" {
		t.Fatalf("participant removal: %+v %v", r, err)
	}
	finishKnowledgeTestScans(t, f, worker)
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_target WHERE workspace_id=$1 AND knowledge_revision=2`, f.workspace) != 1 {
		t.Fatal("revoked participant disappeared from historical target fanout")
	}
	// Authoritative account changes share this transaction-borrowing helper.
	// The account has no membership in the source; no current ACL join is legal.
	tx, err := f.pool.Begin(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	if _, err = tx.Exec(f.ctx, `UPDATE "user" SET disabled_at=now() WHERE id=$1`, f.handler); err != nil {
		t.Fatal(err)
	}
	if _, err = auth.RevokePasswordCredentials(f.ctx, tx, util.MustParseUUID(f.handler), "admin:"+uuid.NewString()); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(f.ctx); err != nil {
		t.Fatal(err)
	}
	f.fx.Cleanup(t, `DELETE FROM collaboration_account_invalidation WHERE user_id=$1`, f.handler)
	finishKnowledgeTestScans(t, f, worker)
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_target WHERE workspace_id=$1 AND knowledge_revision=3`, f.workspace) != 1 {
		t.Fatal("disabled account failed to invalidate historical targets")
	}
}

func TestKnowledgeMissingCounterDoesNotReportEmptySuccess(t *testing.T) {
	f, worker := newKnowledgeInvalidationFixture(t)
	seedKnowledgeInvalidationCandidate(t, f, true, pgtype.UUID{})
	cause := enqueueKnowledgeTestScan(t, f, "source_issue", f.issue.Issue.ID)
	f.fx.Exec(t, `DELETE FROM collaboration_knowledge_target WHERE workspace_id=$1`, f.workspace)
	if _, err := worker.ScanPending(f.ctx, 100); err == nil {
		t.Fatal("missing live counter masqueraded as an empty target")
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_invalidation WHERE cause_id=$1 AND state='pending' AND scan_cursor IS NULL`, cause) != 1 {
		t.Fatal("corrupt live target advanced the cursor")
	}
}

func TestKnowledgeWorkspaceAuthorizationRequiresCurrentMembership(t *testing.T) {
	f, _ := newKnowledgeInvalidationFixture(t)
	// A target administrator does not need source Read to receive the target
	// identity. This account is deliberately unrelated to the source issue.
	target := f.fx.Workspace(t, "Independent target", "knowledge-independent-"+uuid.NewString())
	f.fx.Member(t, target, f.outsider, "admin")
	if allowed, err := f.svc.AuthorizeKnowledgeWorkspace(collaborationContext(f.outsider), f.outsider, target); err != nil || !allowed {
		t.Fatalf("target-only administrator denied: %v %v", allowed, err)
	}
	if _, err := f.svc.ResolveIssueAccess(collaborationContext(f.outsider), nil, f.issue.Issue.ID); err == nil {
		t.Fatal("target-only administrator unexpectedly gained source Read")
	}
	if allowed, err := f.svc.AuthorizeKnowledgeWorkspace(f.ctx, f.creator, f.workspace); err != nil || !allowed {
		t.Fatalf("current target member denied: %v %v", allowed, err)
	}
	// Source participation alone is not target workspace authority.
	f.fx.Exec(t, `DELETE FROM member WHERE user_id=$1 AND workspace_id=$2`, f.creator, f.workspace)
	if allowed, err := f.svc.AuthorizeKnowledgeWorkspace(f.ctx, f.creator, f.workspace); err != nil || allowed {
		t.Fatalf("departed target member retained access: %v %v", allowed, err)
	}
	f.fx.Member(t, f.workspace, f.creator, "owner")
	f.fx.Exec(t, `UPDATE "user" SET disabled_at=now() WHERE id=$1`, f.creator)
	if allowed, err := f.svc.AuthorizeKnowledgeWorkspace(f.ctx, f.creator, f.workspace); err != nil || allowed {
		t.Fatalf("disabled account retained target access: %v %v", allowed, err)
	}
}

func TestKnowledgeBusyTargetYieldsToOtherScans(t *testing.T) {
	f, worker := newKnowledgeInvalidationFixture(t)
	other, _ := newKnowledgeInvalidationFixture(t)
	seedKnowledgeInvalidationCandidate(t, f, true, pgtype.UUID{})
	seedKnowledgeInvalidationCandidate(t, other, true, pgtype.UUID{})
	cause := enqueueKnowledgeTestScan(t, f, "source_issue", f.issue.Issue.ID)
	enqueueKnowledgeTestScan(t, other, "source_issue", other.issue.Issue.ID)
	lock, err := f.pool.Begin(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Rollback(context.Background())
	if _, err = f.svc.Queries.WithTx(lock).LockKnowledgeTargetForWrite(f.ctx, db.LockKnowledgeTargetForWriteParams{TargetKind: "workspace", TargetID: util.MustParseUUID(f.workspace)}); err != nil {
		t.Fatal(err)
	}
	if _, err = worker.ScanPending(f.ctx, 100); err == nil {
		t.Fatal("busy target reported success")
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_invalidation WHERE cause_id=$1 AND state='pending' AND scan_cursor IS NULL`, cause) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_target WHERE workspace_id=$1 AND knowledge_revision=2`, other.workspace) != 1 {
		t.Fatal("busy target lost its cursor or starved an independent scan")
	}
	if err = lock.Rollback(f.ctx); err != nil {
		t.Fatal(err)
	}
	finishKnowledgeTestScans(t, f, worker)
}

func TestKnowledgeScanBoundaryAndConcurrentApprovalHaveIndependentHints(t *testing.T) {
	f, worker := newKnowledgeInvalidationFixture(t)
	proposal := knowledgeServiceProposal(t, f)
	if _, err := f.svc.ProposeKnowledge(f.ctx, f.issue.Issue.ID, proposal); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.ApproveKnowledge(f.ctx, util.MustParseUUID(proposal.CandidateID), protocol.CollaborationKnowledgeApproveRequest{ProtocolVersion: 1, RequestID: uuid.NewString(), ExpectedStateRevision: "1", ExpectedVersion: "1", ExpectedContentSHA256: protocol.CollaborationKnowledgeHash(proposal.Content)}); err != nil {
		t.Fatal(err)
	}
	cause := enqueueKnowledgeTestScan(t, f, "source_issue", f.issue.Issue.ID)
	if _, err := worker.ScanPending(f.ctx, 1); err != nil {
		t.Fatal(err)
	}
	var before, after pgtype.UUID
	f.fx.QueryRow(t, `SELECT scan_upper_bound FROM collaboration_knowledge_invalidation WHERE cause_id=$1 AND kind='scan'`, cause).Scan(&before)
	proposal.CandidateID, proposal.RequestID = uuid.NewString(), uuid.NewString()
	if _, err := f.svc.ProposeKnowledge(f.ctx, f.issue.Issue.ID, proposal); err != nil {
		t.Fatal(err)
	}
	approved, err := f.svc.ApproveKnowledge(f.ctx, util.MustParseUUID(proposal.CandidateID), protocol.CollaborationKnowledgeApproveRequest{ProtocolVersion: 1, RequestID: uuid.NewString(), ExpectedStateRevision: "1", ExpectedVersion: "1", ExpectedContentSHA256: protocol.CollaborationKnowledgeHash(proposal.Content)})
	if err != nil || approved.Code != "knowledge_approved" {
		t.Fatalf("concurrent approval: %+v %v", approved, err)
	}
	finishKnowledgeTestScans(t, f, worker)
	f.fx.QueryRow(t, `SELECT scan_upper_bound FROM collaboration_knowledge_invalidation WHERE cause_id=$1 AND kind='scan'`, cause).Scan(&after)
	if before != after || f.fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_invalidation WHERE target_workspace_id=$1 AND kind='target_hint'`, f.workspace) != 3 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_target WHERE workspace_id=$1 AND knowledge_revision=3`, f.workspace) != 1 {
		t.Fatal("scan moved its boundary or missed/doubled the direct new approval hint")
	}
}

func TestKnowledgeSourceMutationRollsBackWhenScanAppendFails(t *testing.T) {
	f, _ := newKnowledgeInvalidationFixture(t)
	f.memoryUpdate(t, f.creator, 0, "PRIVATE_REQUIRED_SOURCE")
	f.memoryUpdate(t, f.creator, 1, "current")
	request := CollaborationMemoryPurgeParams{IssueID: f.issue.Issue.ID, RequestID: uuid.NewString(), Version: 1, Reason: "dispose"}
	f.svc.TxStarter = workflowFaultStarter{TxStarter: f.pool, needle: "INSERT INTO collaboration_knowledge_invalidation"}
	if _, err := f.svc.PurgeMemoryVersion(f.ctx, request); err == nil {
		t.Fatal("failed durable append permitted purge")
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_memory_version WHERE issue_id=$1 AND version=1 AND content='PRIVATE_REQUIRED_SOURCE' AND purged_at IS NULL`, f.issue.Issue.ID) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1 AND payload->>'action'='purge'`, f.issue.Issue.ID) != 0 {
		t.Fatal("failed scan append committed a purge/event prefix")
	}
	f.svc.TxStarter = f.pool
	r, err := f.svc.PurgeMemoryVersion(f.ctx, request)
	if err != nil || r.Code != "memory_purged" {
		t.Fatalf("same-key retry failed: %+v %v", r, err)
	}
	replay, err := f.svc.PurgeMemoryVersion(f.ctx, request)
	if err != nil || replay.Code != r.Code || f.fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_invalidation WHERE scope_id=$1`, f.issue.Issue.ID) != 1 {
		t.Fatal("original purge retry lost or duplicated its scan")
	}
}

func TestKnowledgeCommentEditRetainsSourceUntilDelete(t *testing.T) {
	f, worker := newKnowledgeInvalidationFixture(t)
	created, err := f.svc.CreateComment(f.ctx, CollaborationCreateCommentParams{CollaborationQueueRequest: f.request(), CollaborationCommentInput: CollaborationCommentInput{Content: "PRIVATE_COMMENT_V1"}})
	if err != nil || created.Code != "comment_created" {
		t.Fatalf("create comment: %+v %v", created, err)
	}
	ref := []protocol.CollaborationKnowledgeSource{{Kind: "comment", IssueID: util.UUIDToString(f.issue.Issue.ID), CommentID: util.UUIDToString(created.CommentID), Version: "1", SHA256: protocol.CollaborationKnowledgeHash("PRIVATE_COMMENT_V1")}}
	edit := CollaborationEditCommentParams{IssueID: f.issue.Issue.ID, CommentID: created.CommentID, RequestID: uuid.NewString(), ExpectedCommentRevision: 1, Content: "v2"}
	edited, err := f.svc.EditComment(f.ctx, edit)
	if err != nil || edited.Code != "comment_edited" {
		t.Fatalf("edit: %+v %v", edited, err)
	}
	available, err := f.svc.KnowledgeSourceAvailability(f.ctx, f.svc.Queries, f.issue.Issue.ID, ref)
	if err != nil || !available || f.fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_invalidation WHERE scope_id=$1`, f.issue.Issue.ID) != 0 {
		t.Fatal("ordinary retained edit incorrectly invalidated source availability")
	}
	seedKnowledgeInvalidationCandidate(t, f, true, pgtype.UUID{})
	edit.RequestID, edit.Content, edit.ExpectedCommentRevision = uuid.NewString(), "", 2
	deleted, err := f.svc.DeleteComment(f.ctx, edit)
	if err != nil || deleted.Code != "comment_deleted" {
		t.Fatalf("delete: %+v %v", deleted, err)
	}
	available, err = f.svc.KnowledgeSourceAvailability(f.ctx, f.svc.Queries, f.issue.Issue.ID, ref)
	if err != nil || available || f.fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_invalidation WHERE cause_id=$1 AND scope_id=$2`, deleted.EventID, f.issue.Issue.ID) != 1 {
		t.Fatal("comment tombstone failed to suppress history or append source intent")
	}
	finishKnowledgeTestScans(t, f, worker)
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_target WHERE workspace_id=$1 AND knowledge_revision=2`, f.workspace) != 1 {
		t.Fatal("comment deletion did not reach published target")
	}
}

type knowledgeSignalPublisher struct {
	signal chan struct{}
	once   sync.Once
}

func (p *knowledgeSignalPublisher) PublishWithID(_, _, _ string, _ []byte, _ string) error {
	p.once.Do(func() { close(p.signal) })
	return nil
}

func TestKnowledgeInvalidationRunSchedulesScannerAndDrainer(t *testing.T) {
	f, worker := newKnowledgeInvalidationFixture(t)
	seedKnowledgeInvalidationCandidate(t, f, true, pgtype.UUID{})
	cause := enqueueKnowledgeTestScan(t, f, "source_issue", f.issue.Issue.ID)
	publisher := &knowledgeSignalPublisher{signal: make(chan struct{})}
	worker.Publisher = publisher
	ctx, cancel := context.WithCancel(f.ctx)
	defer cancel()
	done := make(chan struct{})
	go func() { defer close(done); worker.Run(ctx) }()
	select {
	case <-publisher.signal:
	case <-time.After(3 * time.Second):
		t.Fatal("registered worker lifecycle did not scan and publish")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("worker did not honor shutdown")
	}
	// Cancellation can race the delivery commit: either original pending or
	// delivered is safe, while the scanner must have durably produced the hint.
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_invalidation WHERE cause_id=$1 AND kind='target_hint'`, cause) != 1 {
		t.Fatal("lifecycle publication had no durable target hint")
	}
}

func TestKnowledgeAdministrativeDisableAppendsBeforeReconciliation(t *testing.T) {
	f := newAdminCollaborationFixture(t)
	knowledgeInvalidationCleanup(t, f.collaborationQueueFixture)
	seedKnowledgeInvalidationCandidate(t, f.collaborationQueueFixture, true, pgtype.UUID{})
	p := PlatformAccountChangeParams{OrganizationID: f.org, TargetUserID: util.MustParseUUID(f.handler), IdempotencyKey: pgtype.UUID{Bytes: uuid.New(), Valid: true}, Action: "disable", ExpectedAuthVersion: 1, Reason: "Disable departed account", Password: platformAdminTestPassword, RequestID: uuid.NewString()}
	svc := NewPlatformAdminService(f.svc.Queries, f.pool)
	result, err := svc.ChangeAccount(f.adminCtx, p)
	if err != nil || result.Operation.ResultCode != "account_disabled" {
		t.Fatalf("admin disable: %+v %v", result, err)
	}
	cause := uuid.NewSHA1(uuid.UUID(p.TargetUserID.Bytes), []byte("collaboration-knowledge:password:admin:"+util.UUIDToString(result.Operation.ID)))
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_invalidation WHERE cause_id=$1 AND scope_kind='all_sources' AND state='pending'`, cause.String()) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_account_invalidation WHERE user_id=$1 AND reconciled_at IS NULL`, p.TargetUserID) != 1 {
		t.Fatal("account change waited for reconciliation or omitted durable global scan")
	}
	if _, err = svc.ChangeAccount(f.adminCtx, p); err != nil {
		t.Fatal(err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_invalidation WHERE cause_id=$1`, cause.String()) != 1 {
		t.Fatal("account operation replay duplicated its cause")
	}
	worker := &CollaborationKnowledgeInvalidation{Queries: f.svc.Queries, TxStarter: f.pool}
	finishKnowledgeTestScans(t, f.collaborationQueueFixture, worker)
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_invalidation WHERE cause_id=$1 AND target_workspace_id=$2`, cause.String(), f.workspace) != 1 {
		t.Fatal("admin disable did not fan out before per-issue reconciliation")
	}
}

// knowledgeScriptedPublisher accepts or refuses each publish by call order, so
// the assertions hold whichever pending hint the queue offers first.
type knowledgeScriptedPublisher struct {
	stall  map[int]time.Duration
	fail   map[int]bool
	events []string
	frames [][]byte
}

func (p *knowledgeScriptedPublisher) PublishWithID(_, _, _ string, frame []byte, event string) error {
	call := len(p.events)
	p.events = append(p.events, event)
	p.frames = append(p.frames, append([]byte(nil), frame...))
	if d := p.stall[call]; d > 0 {
		time.Sleep(d) // The relay accepted the frame; its acknowledgement outlives the hint budget.
	}
	if p.fail[call] {
		return errors.New("relay unavailable")
	}
	return nil
}

// seedKnowledgeTestHints leaves n pending target hints for this fixture's
// workspace, one per source cause, each created after the previous one.
func seedKnowledgeTestHints(t *testing.T, f *collaborationQueueFixture, worker *CollaborationKnowledgeInvalidation, n int) {
	t.Helper()
	seedKnowledgeInvalidationCandidate(t, f, true, pgtype.UUID{})
	for range n {
		enqueueKnowledgeTestScan(t, f, "source_issue", f.issue.Issue.ID)
		finishKnowledgeTestScans(t, f, worker)
	}
	if got := f.fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_invalidation WHERE kind='target_hint' AND state='pending' AND target_workspace_id=$1`, f.workspace); got != n {
		t.Fatalf("seeded %d pending hints, want %d", got, n)
	}
}

func knowledgeHintIDFromEvent(t *testing.T, event string) string {
	t.Helper()
	id, ok := strings.CutSuffix(event, ":target")
	if !ok {
		t.Fatalf("unexpected hint event id %q", event)
	}
	return id
}

func TestKnowledgeTimedOutHintDoesNotBlockLaterHints(t *testing.T) {
	f, worker := newKnowledgeInvalidationFixture(t)
	seedKnowledgeTestHints(t, f, worker, 2)
	worker.hintTimeout = 200 * time.Millisecond
	publisher := &knowledgeScriptedPublisher{stall: map[int]time.Duration{0: 600 * time.Millisecond}}
	worker.Publisher = publisher
	delivered, err := worker.DeliverPending(f.ctx, 100)
	if err == nil {
		t.Fatal("hint that outlived its delivery budget reported success")
	}
	if len(publisher.events) != 2 || delivered != 1 {
		t.Fatalf("hint behind the timed-out hint was not delivered in the same pass: delivered=%d events=%v err=%v", delivered, publisher.events, err)
	}
	stalled, behind := knowledgeHintIDFromEvent(t, publisher.events[0]), knowledgeHintIDFromEvent(t, publisher.events[1])
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_invalidation WHERE id=$1 AND state='delivered'`, behind) != 1 {
		t.Fatal("hint behind the timed-out hint stayed pending")
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_invalidation WHERE id=$1 AND state='pending' AND last_attempted_at IS NOT NULL`, stalled) != 1 {
		t.Fatal("timed-out hint did not record its attempt, so it keeps the head of the queue")
	}
	// The retry republishes the identical event; the Hub and clients dedup by event_id.
	publisher.stall = nil
	if _, err = worker.DeliverPending(f.ctx, 100); err != nil {
		t.Fatal(err)
	}
	if len(publisher.events) != 3 || publisher.events[2] != publisher.events[0] || string(publisher.frames[2]) != string(publisher.frames[0]) {
		t.Fatalf("retry changed the stable hint event or repeated a delivered hint: %v", publisher.events)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_invalidation WHERE id IN ($1,$2) AND state='delivered'`, stalled, behind) != 2 {
		t.Fatal("retry did not settle the timed-out hint")
	}
}

func TestKnowledgeHintDeliverySkipsFailuresUntilConsecutiveLimit(t *testing.T) {
	t.Run("skips_failed_hint", func(t *testing.T) {
		f, worker := newKnowledgeInvalidationFixture(t)
		seedKnowledgeTestHints(t, f, worker, 2)
		publisher := &knowledgeScriptedPublisher{fail: map[int]bool{0: true}}
		worker.Publisher = publisher
		delivered, err := worker.DeliverPending(f.ctx, 100)
		if err == nil || delivered != 1 || len(publisher.events) != 2 || publisher.events[0] == publisher.events[1] {
			t.Fatalf("failed hint stopped the pass: delivered=%d events=%v err=%v", delivered, publisher.events, err)
		}
		failed := knowledgeHintIDFromEvent(t, publisher.events[0])
		if f.fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_invalidation WHERE id=$1 AND state='pending' AND last_attempted_at IS NOT NULL`, failed) != 1 {
			t.Fatal("failed hint did not record its attempt")
		}
	})
	t.Run("each_hint_once_per_pass", func(t *testing.T) {
		f, worker := newKnowledgeInvalidationFixture(t)
		seedKnowledgeTestHints(t, f, worker, 2)
		publisher := &knowledgeScriptedPublisher{fail: map[int]bool{0: true, 1: true, 2: true}}
		worker.Publisher = publisher
		delivered, err := worker.DeliverPending(f.ctx, 100)
		if err == nil || delivered != 0 || len(publisher.events) != 2 || publisher.events[0] == publisher.events[1] {
			t.Fatalf("failed hints were not each attempted exactly once: delivered=%d events=%v err=%v", delivered, publisher.events, err)
		}
	})
	t.Run("stops_after_three_consecutive_failures", func(t *testing.T) {
		f, worker := newKnowledgeInvalidationFixture(t)
		seedKnowledgeTestHints(t, f, worker, 5)
		// A dead relay fails every hint; the design caps a pass at three in a row.
		publisher := &knowledgeScriptedPublisher{fail: map[int]bool{0: true, 1: true, 2: true, 3: true, 4: true}}
		worker.Publisher = publisher
		delivered, err := worker.DeliverPending(f.ctx, 100)
		if err == nil || delivered != 0 || len(publisher.events) != 3 {
			t.Fatalf("dead relay pass: delivered=%d events=%v err=%v", delivered, publisher.events, err)
		}
		seen := map[string]bool{}
		for _, event := range publisher.events {
			if seen[event] {
				t.Fatalf("one pass retried the same failed hint: %v", publisher.events)
			}
			seen[event] = true
		}
		if f.fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_invalidation WHERE target_workspace_id=$1 AND kind='target_hint' AND state='pending' AND last_attempted_at IS NULL`, f.workspace) != 2 {
			t.Fatal("pass touched hints beyond the consecutive failure limit")
		}
	})
	t.Run("success_resets_the_limit", func(t *testing.T) {
		f, worker := newKnowledgeInvalidationFixture(t)
		seedKnowledgeTestHints(t, f, worker, 6)
		publisher := &knowledgeScriptedPublisher{fail: map[int]bool{0: true, 1: true, 3: true, 4: true, 5: true}}
		worker.Publisher = publisher
		delivered, err := worker.DeliverPending(f.ctx, 100)
		if err == nil || delivered != 1 || len(publisher.events) != 6 {
			t.Fatalf("limit counted non-consecutive failures: delivered=%d events=%v err=%v", delivered, publisher.events, err)
		}
	})
}
