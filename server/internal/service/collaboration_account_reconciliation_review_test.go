package service

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/testutil"
)

// A bounded polling pass must not retry the same failing prefix forever. This
// uses the real source lock and credential writers, with no process-exit claim.
func TestCollaborationAccountReconciliationMakesProgressBeyondFailedPass(t *testing.T) {
	f := newAccountReconciliationFixture(t)
	later := f.user(t, "")
	collaborationAccountFence(t, f.platformAdminFixture, later)
	independent := f.issue(t)
	f.fx.InsertNoID(t, "issue_participant", testutil.Cols{"issue_id": independent, "user_id": later},
		"issue_id=$1 AND user_id=$2", independent, later)
	for _, author := range []pgtype.UUID{f.target, f.other} {
		f.submit(t, f.first, author, 1)
	}
	run := f.run(t, independent, f.submit(t, independent, later, 1), f.grant(t, independent, later, 1, true), 1)
	waiting := f.submit(t, independent, later, 1)
	for _, account := range []pgtype.UUID{f.target, f.other, later} {
		f.change(t, account, "disable", 1)
	}

	tx, err := f.pool.Begin(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	if _, err = tx.Exec(t.Context(), `SELECT id FROM issue WHERE id=$1 FOR NO KEY UPDATE`, f.first); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	processed := 0
	for range 3 {
		n, err := f.worker.Reconcile(ctx, 2)
		if err != nil && !collaborationErrorCode(err, "collaboration_busy") {
			t.Fatalf("unexpected reconciliation failure: %v", err)
		}
		processed += n
	}
	if ctx.Err() != nil {
		t.Fatal("bounded passes did not complete while a source was busy")
	}
	if processed < 2 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=$1 AND state='cancelled'`, waiting.CommandID) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE id=$1 AND forced_disposition='stopped' AND proof_state='unknown' AND settled_at IS NULL`, run.ID) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_account_invalidation WHERE user_id=$1 AND reconciled_at IS NOT NULL`, later) != 1 {
		t.Fatalf("repeated bounded passes starved the later account behind a failing prefix: processed=%d", processed)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_account_invalidation WHERE user_id=ANY($1::uuid[]) AND last_issue_id IS NULL AND reconciled_at IS NULL`, []pgtype.UUID{f.target, f.other}) != 2 {
		t.Fatal("fair polling advanced the busy accounts' durable cursors")
	}
}
