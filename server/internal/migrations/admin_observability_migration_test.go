package migrations

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func TestAdminQueueObservationMigrationTracksOnlyKnownQueueTime(t *testing.T) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("requires task-owned DATABASE_URL")
	}
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	schema := pgx.Identifier{fmt.Sprintf("admin_clock_%d", time.Now().UnixNano())}.Sanitize()
	if _, err = conn.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	defer conn.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE")
	if _, err = conn.Exec(ctx, "SET search_path TO "+schema); err != nil {
		t.Fatal(err)
	}
	if _, err = conn.Exec(ctx, `CREATE TABLE agent_task_queue (id uuid NOT NULL DEFAULT gen_random_uuid(),status text NOT NULL,created_at timestamptz NOT NULL DEFAULT now(),note text); CREATE TABLE admin_audit_event (id uuid NOT NULL); INSERT INTO agent_task_queue(status,created_at) VALUES ('queued','2020-01-01'),('running','2020-01-01')`); err != nil {
		t.Fatal(err)
	}
	var before time.Time
	if err = conn.QueryRow(ctx, "SELECT now()").Scan(&before); err != nil {
		t.Fatal(err)
	}
	applyMigrationFile(t, ctx, conn, "500_admin_observability.up.sql")
	var clock time.Time
	var source string
	if err = conn.QueryRow(ctx, "SELECT queued_at,queued_at_source FROM agent_task_queue WHERE status='queued'").Scan(&clock, &source); err != nil {
		t.Fatal(err)
	}
	if clock.Before(before) || source != "observation" {
		t.Fatal("migration invented an old queue entry time")
	}
	var unknown bool
	if err = conn.QueryRow(ctx, "SELECT queued_at IS NULL AND queued_at_source IS NULL FROM agent_task_queue WHERE status='running'").Scan(&unknown); err != nil || !unknown {
		t.Fatal("migration fabricated running task queue history", err)
	}
	var id string
	var created time.Time
	if err = conn.QueryRow(ctx, "INSERT INTO agent_task_queue(status,created_at) VALUES('queued',now()-interval '1 hour') RETURNING id,created_at,queued_at,queued_at_source").Scan(&id, &created, &clock, &source); err != nil {
		t.Fatal(err)
	}
	if !clock.Equal(created) || source != "transition" {
		t.Fatal("new queue entry lost its exact creation timestamp")
	}
	if _, err = conn.Exec(ctx, "UPDATE agent_task_queue SET status='dispatched' WHERE id=$1", id); err != nil {
		t.Fatal(err)
	}
	if _, err = conn.Exec(ctx, "UPDATE agent_task_queue SET status='queued' WHERE id=$1", id); err != nil {
		t.Fatal(err)
	}
	if err = conn.QueryRow(ctx, "SELECT queued_at,queued_at_source FROM agent_task_queue WHERE id=$1", id).Scan(&clock, &source); err != nil || !clock.After(created) || source != "transition" {
		t.Fatal("requeue reused the prior wait interval", err)
	}
	if _, err = conn.Exec(ctx, readMigrationFile(t, "500_admin_observability.down.sql")); err == nil {
		t.Fatal("rollback discarded retained queue observations")
	}
}
