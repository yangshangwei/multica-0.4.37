package main

import (
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/multica-ai/multica/server/internal/testutil"
	"strings"
	"testing"
)

func TestMaterialRepositoryMigrationRetainsOrphanHistory(t *testing.T) {
	ctx, fx, _, opts := switchMigrationFixture(t)
	fx.Exec(t, `CREATE TABLE git_material_job (id uuid,material_id uuid NOT NULL,revision_id uuid NOT NULL,kind text)`)
	fx.Exec(t, `CREATE TABLE git_material_operation (id uuid,material_id uuid NOT NULL,revision_id uuid NOT NULL)`)
	versions := []string{"914_material_repository", "915_material_repository_id", "916_material_repository_project", "917_material_repository_revision_id", "918_material_repository_decision_id", "919_material_repository_job"}
	opts.Files = realMigrationFiles(t, versions, "up")
	if err := runMigrations(ctx, fx.Pool, opts); err != nil {
		t.Fatal(err)
	}
	if fx.Count(t, `SELECT count(*) FROM pg_constraint WHERE conrelid IN ('project_material_repository'::regclass,'material_repository_revision'::regclass,'material_repository_decision'::regclass) AND contype='f'`) != 0 {
		t.Fatal("repository introduced foreign keys")
	}
	fx.Insert(t, "material_repository_revision", testutil.Cols{"id": uuid.NewString(), "repository_id": uuid.NewString(), "workspace_id": uuid.NewString(), "project_id": uuid.NewString(), "generation": 1, "payload": `{"proof":"retained"}`})
	for _, v := range versions {
		down := opts
		down.Direction = "down"
		down.Hooks = hooksForDirection("down")
		down.Files = realMigrationFiles(t, []string{v}, "down")
		var failure *pgconn.PgError
		if err := runMigrations(ctx, fx.Pool, down); !errors.As(err, &failure) || failure.Code != "P0001" || !strings.Contains(err.Error(), "retained") {
			t.Fatalf("orphan evidence did not guard %s: %v", v, err)
		}
	}
	if fx.Count(t, `SELECT count(*) FROM material_repository_revision`) != 1 || fx.Count(t, `SELECT count(*) FROM schema_migrations`) != len(versions) {
		t.Fatal("refused down altered history")
	}
}
