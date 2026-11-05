package migrations

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Exercise the real migration in a private schema, never on deployment tables.
func TestTaskCapabilityRollbackFence(t *testing.T) {
	u := os.Getenv("DATABASE_URL")
	if u == "" {
		t.Skip("isolated database required")
	}
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, u)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	schema := pgx.Identifier{fmt.Sprintf("capability_rollback_%d", time.Now().UnixNano())}.Sanitize()
	if _, err = conn.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	defer conn.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE")
	if _, err = conn.Exec(ctx, "SET search_path TO "+schema); err != nil {
		t.Fatal(err)
	}
	if _, err = conn.Exec(ctx, `CREATE TABLE task_token(id int)`); err != nil {
		t.Fatal(err)
	}
	applyMigrationFile(t, ctx, conn, "765_task_api_capability.up.sql")
	down := readMigrationFile(t, "765_task_api_capability.down.sql")
	writer, err := pgx.Connect(ctx, u)
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Close(ctx)
	if _, err = writer.Exec(ctx, "SET search_path TO "+schema); err != nil {
		t.Fatal(err)
	}
	tx, err := writer.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `INSERT INTO task_token VALUES(1,'{"version":1}')`); err != nil {
		t.Fatal(err)
	}
	_, err = conn.Exec(ctx, down)
	var pgerr *pgconn.PgError
	if !errors.As(err, &pgerr) || pgerr.Code != "55P03" {
		t.Fatalf("concurrent mint was not fenced: %v", err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	_, err = conn.Exec(ctx, down)
	if !errors.As(err, &pgerr) || pgerr.Code != "P0001" {
		t.Fatalf("persisted scope was not protected: %v", err)
	}
	var count int
	if err = conn.QueryRow(ctx, `SELECT count(*) FROM task_token WHERE api_capability IS NOT NULL`).Scan(&count); err != nil || count != 1 {
		t.Fatal("refusal lost capability", err)
	}
	if _, err = conn.Exec(ctx, `DELETE FROM task_token`); err != nil {
		t.Fatal(err)
	}
	applyMigrationFile(t, ctx, conn, "765_task_api_capability.down.sql")
	applyMigrationFile(t, ctx, conn, "765_task_api_capability.up.sql")
}
