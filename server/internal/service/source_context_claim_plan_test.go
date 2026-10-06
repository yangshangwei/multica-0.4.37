package service

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/dbid"
)

func TestSourceContextClaimKeepsSingleLeaseWithStaleStatistics(t *testing.T) {
	pool := newResolveOriginatorPool(t)
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	// A transaction-local copy isolates both rows and planner statistics. The
	// production sqlc query resolves this table without touching any real intent.
	if _, err = tx.Exec(ctx, `CREATE TEMP TABLE issue_source_context_object_intent (LIKE public.issue_source_context_object_intent INCLUDING ALL) ON COMMIT DROP`); err != nil {
		t.Fatal(err)
	}
	q := db.New(tx)
	ws, source := dbid.NewV7(), dbid.NewV7()
	for i := 0; i < 3; i++ {
		_, err = q.RecordSourceContextDeletionObjectIntent(ctx, db.RecordSourceContextDeletionObjectIntentParams{StorageKey: fmt.Sprintf("claim-%d", i), WorkspaceID: ws, SourceContextID: source, AttachmentID: dbid.NewV7(), ObjectUrl: fmt.Sprintf("local://claim-%d", i)})
		if err != nil {
			t.Fatal(err)
		}
		if _, err = tx.Exec(ctx, `UPDATE issue_source_context_object_intent SET created_at=now()-interval '2 hours',next_attempt_at='-infinity'::timestamptz`); err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			// Estimate one row, then add two without ANALYZE. PostgreSQL can now
			// choose a Nested Loop that rescans a nonmaterialized LIMIT subquery.
			if _, err = tx.Exec(ctx, `ANALYZE issue_source_context_object_intent`); err != nil {
				t.Fatal(err)
			}
		}
	}
	seen := map[string]bool{}
	for i := 0; i < 3; i++ {
		lease := dbid.NewV7()
		row, err := q.ClaimSourceContextObjectIntentForCleanup(ctx, lease)
		if err != nil {
			t.Fatalf("claim %d: %v", i, err)
		}
		if seen[row.StorageKey] {
			t.Fatal("reclaimed a still-leased intent")
		}
		seen[row.StorageKey] = true
		var leased, total int
		if err = tx.QueryRow(ctx, `SELECT count(*) FILTER(WHERE lease_token=$1),count(*) FILTER(WHERE state='deleting') FROM issue_source_context_object_intent`, lease).Scan(&leased, &total); err != nil {
			t.Fatal(err)
		}
		if leased != 1 || total != i+1 {
			t.Fatalf("one claim must lease one row: leased=%d total=%d after claim %d", leased, total, i+1)
		}
	}
	if _, err = q.ClaimSourceContextObjectIntentForCleanup(ctx, dbid.NewV7()); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("active leases became claimable: %v", err)
	}
}
