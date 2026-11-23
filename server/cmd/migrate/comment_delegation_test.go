package main

import (
	"os"
	"slices"
	"testing"

	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestDelegationReceiptMigrationsRetainEvidence(t *testing.T) {
	ctx, fx, _, opts := switchMigrationFixture(t)
	versions := []string{"771_comment_delegation_receipt", "772_comment_delegation_receipt_key", "773_comment_delegation_receipt_ready", "774_comment_delegation_receipt_comment", "775_comment_delegation_replay_ready"}
	opts.Files = realMigrationFiles(t, versions, "up")
	first := opts
	first.Files = first.Files[:1]
	if err := runMigrations(ctx, fx.Pool, first); err != nil {
		t.Fatal(err)
	}
	if fx.Count(t, `SELECT count(*) FROM pg_index WHERE indrelid='comment_delegation_receipt'::regclass`) != 0 {
		t.Fatal("table creates implicit index")
	}
	if err := runMigrations(ctx, fx.Pool, opts); err != nil {
		t.Fatal(err)
	}
	q := db.New(fx.Pool)
	ready, err := q.DelegationReceiptSchemaReady(ctx)
	if err != nil || !ready {
		t.Fatalf("schema not ready %v %v", ready, err)
	}
	n, err := q.ClaimCommentDelegationReceipt(ctx, db.ClaimCommentDelegationReceiptParams{WorkspaceID: artifactUUID(), ActorType: "member", ActorID: artifactUUID(), OriginatorID: artifactUUID(), RequestKey: artifactUUID(), RequestDigest: "fixture", IssueID: artifactUUID(), CommentID: artifactUUID(), Targets: []byte(`[]`), Response: []byte(`{}`)})
	if err != nil || n != 1 {
		t.Fatalf("claim %d %v", n, err)
	}
	down := opts
	down.Direction = "down"
	down.Hooks = hooksForDirection("down")
	slices.Reverse(versions)
	down.Files = realMigrationFiles(t, versions, "down")
	artifactPGCode(t, runMigrations(ctx, fx.Pool, down), "P0001")
	if fx.Count(t, `SELECT count(*) FROM comment_delegation_receipt`) != 1 || fx.Count(t, `SELECT count(*) FROM schema_migrations`) != 5 {
		t.Fatal("refusal lost evidence or migration ledger")
	}
	fx.Exec(t, `DELETE FROM comment_delegation_receipt`)
	if err = runMigrations(ctx, fx.Pool, down); err != nil {
		t.Fatal(err)
	}
	if fx.Count(t, `SELECT count(*) FROM schema_migrations`) != 0 {
		t.Fatal("empty rollback not complete")
	}
}

func TestDelegationReceiptSchemaAndConcurrentRollback(t *testing.T) {
	ctx, fx, _, opts := switchMigrationFixture(t)
	versions := []string{"771_comment_delegation_receipt", "772_comment_delegation_receipt_key", "773_comment_delegation_receipt_ready", "774_comment_delegation_receipt_comment", "775_comment_delegation_replay_ready"}
	opts.Files = realMigrationFiles(t, versions, "up")
	if err := runMigrations(ctx, fx.Pool, opts); err != nil {
		t.Fatal(err)
	}
	downBody, err := os.ReadFile(realMigrationFiles(t, []string{versions[4]}, "down")[0])
	if err != nil {
		t.Fatal(err)
	}
	tx, err := fx.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "LOCK TABLE comment_delegation_receipt IN ROW EXCLUSIVE MODE"); err != nil {
		t.Fatal(err)
	}
	_, err = fx.Pool.Exec(ctx, string(downBody))
	artifactPGCode(t, err, "55P03")
	if err = tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	fx.Exec(t, "DROP INDEX CONCURRENTLY comment_delegation_receipt_key_idx")
	fx.Exec(t, "CREATE INDEX CONCURRENTLY comment_delegation_receipt_key_idx ON comment_delegation_receipt(workspace_id,actor_type,actor_id,originator_id,request_key)")
	ready, err := db.New(fx.Pool).DelegationReceiptSchemaReady(ctx)
	if err != nil || ready {
		t.Fatalf("nonunique index accepted %v %v", ready, err)
	}
	upBody, err := os.ReadFile(opts.Files[2])
	if err != nil {
		t.Fatal(err)
	}
	_, err = fx.Pool.Exec(ctx, string(upBody))
	artifactPGCode(t, err, "P0001")
}
