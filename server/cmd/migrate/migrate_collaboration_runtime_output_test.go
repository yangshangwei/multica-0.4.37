package main

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/multica-ai/multica/server/internal/testutil"
)

func runtimeOutputMigrationFixture(t *testing.T) (context.Context, *testutil.Fixture, runOptions) {
	t.Helper()
	ctx, fx, _, opts := switchMigrationFixture(t)
	// Exercise the actual original table and encrypted-delivery upgrade in an
	// owned schema, independently of the application schema's current version.
	fx.Exec(t, `CREATE TABLE comment (id uuid)`)
	opts.Files = realMigrationFiles(t, []string{"571_collaboration_schema", "727_collaboration_attachment_delivery"}, "up")
	if err := runMigrations(ctx, fx.Pool, opts); err != nil {
		t.Fatal(err)
	}
	opts.Files = realMigrationFiles(t, []string{"868_collaboration_runtime_output"}, "up")
	return ctx, fx, opts
}

func TestCollaborationRuntimeOutputMigrationPreservesHumanAndAgentIdentity(t *testing.T) {
	ctx, fx, opts := runtimeOutputMigrationFixture(t)
	human := fx.Insert(t, "collaboration_attachment", testutil.Cols{"draft_id": uuid.NewString(), "uploader_user_id": uuid.NewString(), "source_workspace_id": uuid.NewString(),
		"upload_request_id": uuid.NewString(), "storage_key": "private/collaboration/human", "content_sha256": strings.Repeat("a", 64), "encryption_key": make([]byte, 32), "name": "human.txt", "content_type": "text/plain", "size_bytes": 1})
	if err := runMigrations(ctx, fx.Pool, opts); err != nil {
		t.Fatal(err)
	}
	if fx.Count(t, `SELECT count(*) FROM collaboration_attachment WHERE id=$1 AND uploader_user_id IS NOT NULL AND draft_id IS NOT NULL AND uploader_agent_id IS NULL AND run_id IS NULL AND attempt_seq IS NULL`, human) != 1 {
		t.Fatal("upgrade changed an existing human draft's identity")
	}
	agent := fx.Insert(t, "collaboration_attachment", testutil.Cols{"issue_id": uuid.NewString(), "source_workspace_id": uuid.NewString(), "uploader_agent_id": uuid.NewString(),
		"run_id": uuid.NewString(), "attempt_seq": 1, "delivered_at": "2026-11-19T01:02:03Z", "storage_key": "private/collaboration/agent", "content_sha256": strings.Repeat("b", 64),
		"encryption_key": make([]byte, 32), "name": "agent.txt", "content_type": "text/plain", "size_bytes": 2})
	for _, change := range []string{"uploader_user_id=gen_random_uuid()", "draft_id=gen_random_uuid()", "run_id=NULL", "uploader_agent_id=NULL", "attempt_seq=NULL", "attempt_seq=0", "issue_id=NULL", "delivered_at=NULL", "source_workspace_id=NULL", "encryption_key=NULL", "discarded_at=now()", "upload_request_id='fake-human-upload'", "storage_key='public/escaped'"} {
		t.Run(change, func(t *testing.T) {
			_, err := fx.Pool.Exec(ctx, `UPDATE collaboration_attachment SET `+change+` WHERE id=$1`, agent)
			var pg *pgconn.PgError
			if !errors.As(err, &pg) || pg.Code != "23514" {
				t.Fatalf("invalid Agent identity escaped the real CHECK: %s: %v", change, err)
			}
		})
	}
	if fx.Count(t, `SELECT count(*) FROM pg_constraint WHERE conrelid='collaboration_attachment'::regclass AND contype='f'`) != 0 {
		t.Fatal("runtime output added database relationship authority")
	}
	down := opts
	down.Direction, down.Hooks = "down", hooksForDirection("down")
	down.Files = realMigrationFiles(t, []string{"868_collaboration_runtime_output"}, "down")
	if err := runMigrations(ctx, fx.Pool, down); err == nil {
		t.Fatal("downgrade removed retained Agent output attribution")
	}
	if fx.Count(t, `SELECT count(*) FROM collaboration_attachment WHERE id=$1 AND uploader_user_id IS NULL AND uploader_agent_id IS NOT NULL AND run_id IS NOT NULL AND attempt_seq=1`, agent) != 1 {
		t.Fatal("refused downgrade changed the original Agent output")
	}
	fx.Exec(t, `DELETE FROM collaboration_attachment WHERE id=$1`, agent)
	if err := runMigrations(ctx, fx.Pool, down); err != nil {
		t.Fatalf("empty-output downgrade could not preserve the existing human draft: %v", err)
	}
	if fx.Count(t, `SELECT count(*) FROM collaboration_attachment WHERE id=$1 AND uploader_user_id IS NOT NULL AND draft_id IS NOT NULL`, human) != 1 ||
		fx.Count(t, `SELECT count(*) FROM pg_attribute WHERE attrelid='collaboration_attachment'::regclass AND attname='uploader_user_id' AND attnotnull`) != 1 {
		t.Fatal("downgrade did not restore the original human constraint")
	}
}

func TestCollaborationRuntimeOutputMigrationRefusesUncommittedWriter(t *testing.T) {
	ctx, fx, opts := runtimeOutputMigrationFixture(t)
	if err := runMigrations(ctx, fx.Pool, opts); err != nil {
		t.Fatal(err)
	}
	writer, err := fx.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Rollback(ctx)
	_, err = writer.Exec(ctx, `INSERT INTO collaboration_attachment (issue_id,source_workspace_id,uploader_agent_id,run_id,attempt_seq,delivered_at,storage_key,content_sha256,encryption_key,name,content_type,size_bytes)
		VALUES ($1,$2,$3,$4,1,now(),'private/collaboration/uncommitted',$5,$6,'agent.txt','text/plain',1)`, uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString(), strings.Repeat("c", 64), make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	down := opts
	down.Direction, down.Hooks = "down", hooksForDirection("down")
	down.Files = realMigrationFiles(t, []string{"868_collaboration_runtime_output"}, "down")
	if err = runMigrations(ctx, fx.Pool, down); err == nil {
		t.Fatal("downgrade waited out or erased an uncommitted first output")
	}
	if err = writer.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if fx.Count(t, `SELECT count(*) FROM collaboration_attachment WHERE uploader_agent_id IS NOT NULL AND attempt_seq=1`) != 1 {
		t.Fatal("busy-table refusal lost the writer's original output")
	}
	if err = runMigrations(ctx, fx.Pool, down); err == nil {
		t.Fatal("committed first output did not retain its rollback refusal")
	}
}
