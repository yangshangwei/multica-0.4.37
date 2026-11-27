package main

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/multica-ai/multica/server/internal/testutil"
)

func TestMaterialOperationMigrationsRetainExecutionIdentity(t *testing.T) {
	ctx, fx, _, opts := switchMigrationFixture(t)
	versions := []string{"909_git_material_operation", "910_git_material_operation_id", "911_git_material_operation_task", "912_git_material_operation_attempt", "913_git_material_receipt_history"}
	opts.Files = realMigrationFiles(t, versions, "up")
	for _, file := range opts.Files[1:4] {
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(string(raw), "CREATE UNIQUE INDEX CONCURRENTLY ") || strings.Count(string(raw), ";") != 1 {
			t.Fatalf("index is not an isolated concurrent build: %s", file)
		}
	}
	if err := runMigrations(ctx, fx.Pool, opts); err != nil {
		t.Fatal(err)
	}
	if fx.Count(t, `SELECT count(*) FROM pg_index WHERE indrelid='git_material_operation'::regclass AND indisvalid AND indisready AND indisunique`) != 3 || fx.Count(t, `SELECT count(*) FROM pg_constraint WHERE conrelid='git_material_operation'::regclass AND contype='f'`) != 0 {
		t.Fatal("operation identity indexes or no-FK contract missing")
	}
	id := fx.Insert(t, "git_material_operation", testutil.Cols{"id": uuid.NewString(), "job_id": uuid.NewString(), "task_id": uuid.NewString(), "workspace_id": uuid.NewString(), "project_id": uuid.NewString(), "material_id": uuid.NewString(), "revision_id": uuid.NewString(), "runtime_id": uuid.NewString(), "agent_id": uuid.NewString(), "authorizer_id": uuid.NewString(), "attempt_no": 1, "action": "publish", "descriptor": "{}", "expected_digest": strings.Repeat("a", 64), "receipt_history": `{"observation":"digest"}`})
	var before string
	if err := fx.Pool.QueryRow(ctx, `SELECT to_jsonb(o)::text FROM git_material_operation o WHERE id=$1`, id).Scan(&before); err != nil {
		t.Fatal(err)
	}
	for _, version := range versions {
		down := opts
		down.Direction = "down"
		down.Files = realMigrationFiles(t, []string{version}, "down")
		down.Hooks = hooksForDirection("down")
		var refusal *pgconn.PgError
		if err := runMigrations(ctx, fx.Pool, down); !errors.As(err, &refusal) || refusal.Code != "P0001" {
			t.Fatalf("downgrade %s erased retained authority: %v", version, err)
		}
	}
	var after string
	if err := fx.Pool.QueryRow(ctx, `SELECT to_jsonb(o)::text FROM git_material_operation o WHERE id=$1`, id).Scan(&after); err != nil {
		t.Fatal(err)
	}
	if before != after || fx.Count(t, `SELECT count(*) FROM pg_index WHERE indrelid='git_material_operation'::regclass AND indisvalid AND indisready AND indisunique`) != 3 || fx.Count(t, `SELECT count(*) FROM schema_migrations`) != len(versions) {
		t.Fatal("refused downgrade changed execution history, indexes or migration ledger")
	}
}
