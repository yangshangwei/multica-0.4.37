package migrations

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func knowledgeUUID() pgtype.UUID { return pgtype.UUID{Bytes: uuid.New(), Valid: true} }

func knowledgeQueryProposal() db.CreateKnowledgeCandidateParams {
	return db.CreateKnowledgeCandidateParams{ID: knowledgeUUID(), SourceWorkspaceID: knowledgeUUID(), SourceIssueID: knowledgeUUID(), TargetKind: "workspace", TargetWorkspaceID: knowledgeUUID(), Content: "retained private material", Summary: "summary", ContentSha256: protocol.CollaborationKnowledgeHash("retained private material"), Sources: []byte(`[{"kind":"memory"}]`), Evidence: []byte(`[]`), Applicability: []byte(`{"surfaces":["issue"]}`), ProposedByUserID: knowledgeUUID()}
}

func knowledgeTx(t *testing.T, pool *pgxpool.Pool) pgx.Tx {
	t.Helper()
	tx, err := pool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback(context.Background()) })
	return tx
}

func TestKnowledgeQueriesCorrectionCASAndAtomicReplacement(t *testing.T) {
	pool, fx := knowledgeSchema(t)
	q := db.New(pool)
	ctx := context.Background()
	p := knowledgeQueryProposal()
	first, err := q.CreateKnowledgeCandidate(ctx, p)
	if err != nil {
		t.Fatal(err)
	}
	if first.Version != 1 || first.State != "pending" || first.LineageID != p.ID {
		t.Fatalf("unexpected first version: %+v", first)
	}
	approve := db.ApproveKnowledgeCandidateParams{ID: p.ID, ActorID: p.ProposedByUserID, TargetRevision: pgtype.Int8{Int64: 1, Valid: true}, ExpectedVersion: 1, ExpectedStateRevision: 1, ExpectedContentSha256: p.ContentSha256}
	bad := approve
	bad.ExpectedContentSha256 = protocol.CollaborationKnowledgeHash("changed")
	if _, err = q.ApproveKnowledgeCandidate(ctx, bad); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("wrong reviewed content approved: %v", err)
	}
	if _, err = q.ApproveKnowledgeCandidate(ctx, approve); err != nil {
		t.Fatal(err)
	}
	if _, err = q.ApproveKnowledgeCandidate(ctx, approve); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("stale CAS approved again: %v", err)
	}
	correction := p
	correction.ID = knowledgeUUID()
	correction.SupersedesCandidateID = p.ID
	correction.ExpectedPreviousVersion = 1
	wrong := correction
	wrong.TargetWorkspaceID = knowledgeUUID()
	if _, err = q.CreateKnowledgeCandidate(ctx, wrong); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("lineage moved target: %v", err)
	}
	wrong = correction
	wrong.SourceIssueID = knowledgeUUID()
	if _, err = q.CreateKnowledgeCandidate(ctx, wrong); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("lineage moved source: %v", err)
	}
	second, err := q.CreateKnowledgeCandidate(ctx, correction)
	if err != nil {
		t.Fatal(err)
	}
	if second.Version != 2 || second.LineageID != p.ID || second.State != "pending" {
		t.Fatalf("correction identity: %+v", second)
	}
	if fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_candidate WHERE state='approved' AND id=$1`, p.ID) != 1 {
		t.Fatal("pending correction replaced active contribution")
	}
	wrong = correction
	wrong.ID = knowledgeUUID()
	if _, err = q.CreateKnowledgeCandidate(ctx, wrong); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("stale previous version allocated another correction: %v", err)
	}
	tx := knowledgeTx(t, pool)
	qt := db.New(tx)
	if _, err = qt.SupersedeKnowledgeCandidate(ctx, db.SupersedeKnowledgeCandidateParams{ID: p.ID, ActorID: p.ProposedByUserID, ExpectedStateRevision: 2}); err != nil {
		t.Fatal(err)
	}
	approve.ID = second.ID
	approve.ExpectedVersion = 2
	approve.TargetRevision.Int64 = 2
	if _, err = qt.ApproveKnowledgeCandidate(ctx, approve); err != nil {
		t.Fatal(err)
	}
	if err = tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_candidate WHERE state='approved' AND id=$1`, p.ID) != 1 {
		t.Fatal("rollback lost original approval")
	}
	tx = knowledgeTx(t, pool)
	qt = db.New(tx)
	if _, err = qt.SupersedeKnowledgeCandidate(ctx, db.SupersedeKnowledgeCandidateParams{ID: p.ID, ActorID: p.ProposedByUserID, ExpectedStateRevision: 2}); err != nil {
		t.Fatal(err)
	}
	if _, err = qt.ApproveKnowledgeCandidate(ctx, approve); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_candidate WHERE state='approved' AND id=$1`, second.ID) != 1 {
		t.Fatal("atomic replacement missing")
	}
	if fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_candidate WHERE id=$1 AND state='superseded' AND published_target_revision=1 AND approved_at IS NOT NULL`, p.ID) != 1 {
		t.Fatal("old approval facts were erased")
	}
}

func TestKnowledgeQueriesTargetInitializationIsBoundedAndCannotResurrect(t *testing.T) {
	pool, fx := knowledgeSchema(t)
	ctx := context.Background()
	ws := knowledgeUUID()
	fx.InsertNoID(t, "workspace", testutil.Cols{"id": ws}, "id=$1", ws)
	init := db.InitializeKnowledgeTargetParams{TargetKind: "workspace", TargetID: ws, WorkspaceID: ws}
	tx := knowledgeTx(t, pool)
	q := db.New(tx)
	if _, err := q.LockKnowledgeWorkspace(ctx, ws); err != nil {
		t.Fatal(err)
	}
	if n, err := q.InitializeKnowledgeTarget(ctx, init); err != nil || n != 1 {
		t.Fatalf("initial insert: %d %v", n, err)
	}
	other := knowledgeTx(t, pool)
	qo := db.New(other)
	if _, err := qo.LockKnowledgeWorkspace(ctx, ws); err != nil {
		t.Fatal(err)
	}
	if _, err := qo.SetKnowledgeLockTimeout(ctx, "25ms"); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	_, err := qo.InitializeKnowledgeTarget(ctx, init)
	var pgerr *pgconn.PgError
	if !errors.As(err, &pgerr) || pgerr.Code != "55P03" || time.Since(start) > time.Second {
		t.Fatalf("initialization was not bounded: %v after %v", err, time.Since(start))
	}
	if err = other.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	q = db.New(pool)
	if n, err := q.InitializeKnowledgeTarget(ctx, init); err != nil || n != 0 {
		t.Fatalf("duplicate counter: %d %v", n, err)
	}
	if fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_target`) != 1 {
		t.Fatal("multiple counters")
	}
	fx.Exec(t, `DELETE FROM workspace WHERE id=$1`, ws)
	if _, err = q.DeleteKnowledgeTarget(ctx, db.DeleteKnowledgeTargetParams{TargetKind: "workspace", TargetID: ws, WorkspaceID: ws}); err != nil {
		t.Fatal(err)
	}
	if n, err := q.InitializeKnowledgeTarget(ctx, init); err != nil || n != 0 {
		t.Fatalf("deleted parent resurrected: %d %v", n, err)
	}
	fx.Exec(t, `INSERT INTO workspace (id) VALUES ($1)`, ws)
	p := knowledgeQueryProposal()
	p.TargetWorkspaceID = ws
	if _, err = q.CreateKnowledgeCandidate(ctx, p); err != nil {
		t.Fatal(err)
	}
	if _, err = q.ApproveKnowledgeCandidate(ctx, db.ApproveKnowledgeCandidateParams{ID: p.ID, ActorID: p.ProposedByUserID, TargetRevision: pgtype.Int8{Int64: 1, Valid: true}, ExpectedVersion: 1, ExpectedStateRevision: 1, ExpectedContentSha256: p.ContentSha256}); err != nil {
		t.Fatal(err)
	}
	if n, err := q.InitializeKnowledgeTarget(ctx, init); err != nil || n != 0 {
		t.Fatalf("missing counter with published history became empty: %d %v", n, err)
	}
}

func TestKnowledgeQueriesParentLocksKeepOrdinaryEditsIndependent(t *testing.T) {
	pool, fx := knowledgeSchema(t)
	ctx := context.Background()
	ws, project := knowledgeUUID(), knowledgeUUID()
	fx.InsertNoID(t, "workspace", testutil.Cols{"id": ws}, "id=$1", ws)
	fx.InsertNoID(t, "project", testutil.Cols{"id": project, "workspace_id": ws}, "id=$1", project)
	tx := knowledgeTx(t, pool)
	if _, err := tx.Exec(ctx, `UPDATE workspace SET issue_counter=issue_counter+1 WHERE id=$1`, ws); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `UPDATE project SET description='ordinary edit',revision=revision+1 WHERE id=$1`, project); err != nil {
		t.Fatal(err)
	}
	other := knowledgeTx(t, pool)
	q := db.New(other)
	if _, err := q.LockKnowledgeWorkspace(ctx, ws); err != nil {
		t.Fatalf("knowledge blocks ordinary issue allocation: %v", err)
	}
	if _, err := q.LockKnowledgeProject(ctx, db.LockKnowledgeProjectParams{ID: project, WorkspaceID: ws}); err != nil {
		t.Fatalf("knowledge blocks description edit: %v", err)
	}
	if err := other.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	tx = knowledgeTx(t, pool)
	if _, err := tx.Exec(ctx, `SELECT id FROM workspace WHERE id=$1 FOR UPDATE`, ws); err != nil {
		t.Fatal(err)
	}
	other = knowledgeTx(t, pool)
	q = db.New(other)
	_, err := q.LockKnowledgeWorkspace(ctx, ws)
	var pgerr *pgconn.PgError
	if !errors.As(err, &pgerr) || pgerr.Code != "55P03" {
		t.Fatalf("delete parent fence missed: %v", err)
	}
}

func TestKnowledgeQueriesScanAndHintsRetainRoutingAcrossRollback(t *testing.T) {
	pool, fx := knowledgeSchema(t)
	ctx := context.Background()
	q := db.New(pool)
	p := knowledgeQueryProposal()
	if _, err := q.CreateKnowledgeCandidate(ctx, p); err != nil {
		t.Fatal(err)
	}
	pending := knowledgeQueryProposal()
	if _, err := q.CreateKnowledgeCandidate(ctx, pending); err != nil {
		t.Fatal(err)
	}
	if _, err := q.ApproveKnowledgeCandidate(ctx, db.ApproveKnowledgeCandidateParams{ID: p.ID, ActorID: p.ProposedByUserID, TargetRevision: pgtype.Int8{Int64: 1, Valid: true}, ExpectedVersion: 1, ExpectedStateRevision: 1, ExpectedContentSha256: p.ContentSha256}); err != nil {
		t.Fatal(err)
	}
	fx.InsertNoID(t, "collaboration_knowledge_target", testutil.Cols{"target_kind": "workspace", "target_id": p.TargetWorkspaceID, "workspace_id": p.TargetWorkspaceID, "knowledge_revision": 1}, "target_id=$1", p.TargetWorkspaceID)
	holder := knowledgeTx(t, pool)
	if _, err := db.New(holder).LockKnowledgeTargetForWrite(ctx, db.LockKnowledgeTargetForWriteParams{TargetKind: "workspace", TargetID: p.TargetWorkspaceID}); err != nil {
		t.Fatal(err)
	}
	scan := db.EnqueueKnowledgeScanParams{ID: knowledgeUUID(), CauseID: knowledgeUUID(), ScopeKind: pgtype.Text{String: "all_sources", Valid: true}}
	// No source tables exist in this private schema. Neither intent insertion
	// nor scanning needs a source JOIN or a candidate/target row lock.
	short, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	if n, err := q.EnqueueKnowledgeScan(short, scan); err != nil || n != 1 {
		t.Fatalf("source intent waited on target: %d %v", n, err)
	}
	if n, err := q.EnqueueKnowledgeScan(ctx, scan); err != nil || n != 0 {
		t.Fatalf("scan replay duplicated cause: %d %v", n, err)
	}
	if err := holder.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	tx := knowledgeTx(t, pool)
	qt := db.New(tx)
	job, err := qt.LockNextKnowledgeScan(ctx, []pgtype.UUID{})
	if err != nil || job.ID != scan.ID {
		t.Fatalf("scan claim: %v", err)
	}
	job, err = qt.StartKnowledgeScan(ctx, job.ID)
	if err != nil {
		t.Fatal(err)
	}
	row, err := qt.NextKnowledgeScanCandidate(ctx, db.NextKnowledgeScanCandidateParams{UpperBound: job.ScanUpperBound, ScopeKind: "all_sources"})
	if err != nil || row.ID != p.ID {
		t.Fatalf("published routing: %+v %v", row, err)
	}
	if _, err = qt.NextKnowledgeScanCandidate(ctx, db.NextKnowledgeScanCandidateParams{UpperBound: job.ScanUpperBound, AfterID: row.ID, ScopeKind: "all_sources"}); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("pending-only target entered scan: %v", err)
	}
	hint := db.EnqueueKnowledgeTargetHintParams{ID: knowledgeUUID(), CauseID: scan.CauseID, TargetKind: pgtype.Text{String: "workspace", Valid: true}, TargetID: p.TargetWorkspaceID, TargetWorkspaceID: p.TargetWorkspaceID, KnowledgeRevision: pgtype.Int8{Int64: 2, Valid: true}}
	if _, err = qt.AdvanceKnowledgeTarget(ctx, db.AdvanceKnowledgeTargetParams{TargetKind: "workspace", TargetID: p.TargetWorkspaceID, KnowledgeRevision: 1}); err != nil {
		t.Fatal(err)
	}
	if n, err := qt.EnqueueKnowledgeTargetHint(ctx, hint); err != nil || n != 1 {
		t.Fatalf("hint insert: %d %v", n, err)
	}
	if _, err = qt.AdvanceKnowledgeScan(ctx, db.AdvanceKnowledgeScanParams{ID: scan.ID, Cursor: p.ID}); err != nil {
		t.Fatal(err)
	}
	if err = tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_target WHERE knowledge_revision=1`) != 1 || fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_invalidation WHERE scan_started_at IS NOT NULL OR kind='target_hint'`) != 0 {
		t.Fatal("failed scan leaked revision/hint/cursor")
	}
	if n, err := q.EnqueueKnowledgeTargetHint(ctx, hint); err != nil || n != 1 {
		t.Fatalf("retry hint: %d %v", n, err)
	}
	if n, err := q.EnqueueKnowledgeTargetHint(ctx, hint); err != nil || n != 0 {
		t.Fatalf("hint replay duplicate: %d %v", n, err)
	}
	tx = knowledgeTx(t, pool)
	qt = db.New(tx)
	retained, err := qt.LockNextKnowledgeTargetHint(ctx, []pgtype.UUID{})
	if err != nil || retained.ID != hint.ID {
		t.Fatalf("stable hint identity: %v", err)
	}
	if _, err = qt.MarkKnowledgeTargetHintDelivered(ctx, hint.ID); err != nil {
		t.Fatal(err)
	}
	if err = tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	tx = knowledgeTx(t, pool)
	retained, err = db.New(tx).LockNextKnowledgeTargetHint(ctx, []pgtype.UUID{})
	if err != nil || retained.ID != hint.ID {
		t.Fatalf("failed delivery lost stable retry: %v", err)
	}
	if err = tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	knowledgeError(t, pool, "23514", `UPDATE collaboration_knowledge_invalidation SET target_id=gen_random_uuid() WHERE id=$1`, hint.ID)
	knowledgeError(t, pool, "23514", `DELETE FROM collaboration_knowledge_invalidation WHERE id=$1`, hint.ID)
}
