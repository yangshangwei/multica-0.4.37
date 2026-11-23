package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/multica-ai/multica/server/internal/testutil"
)

func TestCollaborationCommandHistoryMigrationRepairsInterruptedIndex(t *testing.T) {
	ctx, fx, schema, opts := switchMigrationFixture(t)
	fx.Exec(t, `CREATE TABLE collaboration_command (LIKE public.collaboration_command INCLUDING DEFAULTS INCLUDING CONSTRAINTS)`)
	opts.Files = realMigrationFiles(t, []string{"736_collaboration_command_history"}, "up")
	id := fx.Insert(t, "collaboration_command", testutil.Cols{
		"session_id": uuid.NewString(), "target_id": uuid.NewString(), "channel_id": uuid.NewString(), "batch_id": uuid.NewString(), "input_event_id": uuid.NewString(),
		"seq": 1, "author_user_id": uuid.NewString(), "participant_epoch": 1, "author_auth_epoch": 1, "lifecycle_epoch": 1, "content": "retained history input",
	})
	writer, err := fx.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Rollback(ctx)
	if _, err = writer.Exec(ctx, `UPDATE collaboration_command SET created_at=created_at WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}
	blocker := int(writer.Conn().PgConn().PID())
	buildCtx, cancelBuild := context.WithCancel(ctx)
	defer cancelBuild()
	done := make(chan error, 1)
	go func() { done <- runMigrations(buildCtx, fx.Pool, opts) }()
	deadline := time.Now().Add(10 * time.Second)
	var builder int
	for {
		if err = fx.Pool.QueryRow(ctx, `SELECT COALESCE((
			SELECT p.pid FROM pg_stat_progress_create_index p
			JOIN pg_index i ON i.indexrelid=p.index_relid JOIN pg_class c ON c.oid=i.indexrelid
			WHERE c.relnamespace=$1::regnamespace AND c.relname='collaboration_command_history_idx'
			AND NOT i.indisvalid AND $2::int=ANY(pg_blocking_pids(p.pid))), 0)`, schema, blocker).Scan(&builder); err != nil {
			t.Fatal(err)
		}
		if builder != 0 {
			break
		}
		select {
		case err = <-done:
			t.Fatalf("command history index did not reach its concurrent build barrier: %v", err)
		default:
		}
		if time.Now().After(deadline) {
			t.Fatal("command history index did not reach its actual writer wait")
		}
	}
	// Client cancellation can return before PostgreSQL has stopped the build.
	// Keep the writer barrier until this exact backend acknowledges cancellation.
	var cancelled bool
	if err = fx.Pool.QueryRow(ctx, `SELECT pg_cancel_backend($1)`, builder).Scan(&cancelled); err != nil || !cancelled {
		t.Fatalf("cancel owned command history index build: cancelled=%t err=%v", cancelled, err)
	}
	var interrupted *pgconn.PgError
	if err = <-done; !errors.As(err, &interrupted) || interrupted.Code != "57014" {
		t.Fatalf("command history index did not acknowledge server cancellation: %v", err)
	}
	if err = writer.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	assertIndexValidity(t, fx.Pool, schema, "collaboration_command_history_idx", false)
	if fx.Count(t, `SELECT count(*) FROM schema_migrations`) != 0 {
		t.Fatal("interrupted history build advanced the migration ledger")
	}
	if err = runMigrations(ctx, fx.Pool, opts); err != nil {
		t.Fatalf("registered cleanup could not recover the interrupted history index: %v", err)
	}
	assertIndexValidity(t, fx.Pool, schema, "collaboration_command_history_idx", true)
	if fx.Count(t, `SELECT count(*) FROM schema_migrations`) != 1 || fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=$1 AND content='retained history input' AND seq=1`, id) != 1 {
		t.Fatal("history-index recovery lost input data or migration identity")
	}
}
