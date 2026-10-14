package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestCollaborationAccountReconciliationRotationSurvivesWorkerRestart(t *testing.T) {
	f := newAccountReconciliationFixture(t)
	independent := f.issue(t)
	blocked := f.submit(t, f.first, f.target, 1)
	waiting := f.submit(t, independent, f.other, 1)
	f.change(t, f.target, "disable", 1)
	f.change(t, f.other, "disable", 1)
	holder, err := f.pool.Begin(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer holder.Rollback(context.Background())
	if _, err = holder.Exec(t.Context(), `SELECT id FROM issue WHERE id=$1 FOR NO KEY UPDATE`, f.first); err != nil {
		t.Fatal(err)
	}
	if n, err := f.worker.Reconcile(t.Context(), 1); n != 0 || !collaborationErrorCode(err, "collaboration_busy") {
		t.Fatalf("expected the earlier source to remain busy: %d, %v", n, err)
	}
	// A fresh process has no failed-account memory. The database must still
	// give the later account its turn with a one-step budget.
	f.worker = NewCollaborationAccountReconciler(f.svc.Queries, f.pool)
	if n, err := f.worker.Reconcile(t.Context(), 1); n != 1 || err != nil {
		t.Fatalf("restart retried the failing prefix instead of independent work: %d, %v", n, err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=$1 AND state='cancelled'`, waiting.CommandID) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=$1 AND state='waiting'`, blocked.CommandID) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_account_invalidation WHERE user_id=$1 AND last_issue_id IS NULL AND reconciled_at IS NULL AND last_attempted_at IS NOT NULL`, f.target) != 1 {
		t.Fatal("attempt scheduling changed failed work or lost the independent cancellation")
	}
}

type failedReconciliationScheduleDB struct{ db.DBTX }

func (q failedReconciliationScheduleDB) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if strings.Contains(sql, "SET last_attempted_at = clock_timestamp()") {
		return pgconn.CommandTag{}, errors.New("injected reconciliation scheduling failure")
	}
	return q.DBTX.Exec(ctx, sql, args...)
}

func TestCollaborationAccountReconciliationReportsSchedulingFailure(t *testing.T) {
	f := newAccountReconciliationFixture(t)
	blocked := f.submit(t, f.first, f.target, 1)
	f.change(t, f.target, "disable", 1)
	holder, err := f.pool.Begin(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer holder.Rollback(context.Background())
	if _, err = holder.Exec(t.Context(), `SELECT id FROM issue WHERE id=$1 FOR NO KEY UPDATE`, f.first); err != nil {
		t.Fatal(err)
	}
	f.worker.Queries = db.New(failedReconciliationScheduleDB{DBTX: f.pool})
	n, err := f.worker.Reconcile(t.Context(), 1)
	if n != 0 || !collaborationErrorCode(err, "collaboration_busy") || !strings.Contains(err.Error(), "injected reconciliation scheduling failure") {
		t.Fatalf("scheduling failure hid the original failure or claimed success: %d, %v", n, err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=$1 AND state='waiting'`, blocked.CommandID) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_account_invalidation WHERE user_id=$1 AND last_issue_id IS NULL AND reconciled_at IS NULL AND last_attempted_at IS NULL`, f.target) != 1 {
		t.Fatal("failed scheduling changed source effects or progress")
	}
}
