package migrations

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestPlatformAdminSchemaAndRollback(t *testing.T) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("requires task-owned DATABASE_URL")
	}
	ctx := context.Background()
	c, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close(ctx)
	schema := pgx.Identifier{fmt.Sprintf("admin_migration_%d", time.Now().UnixNano())}.Sanitize()
	if _, err = c.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	defer c.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE")
	if _, err = c.Exec(ctx, "SET search_path TO "+schema); err != nil {
		t.Fatal(err)
	}
	if _, err = c.Exec(ctx, `CREATE TABLE "user" (id uuid NOT NULL); CREATE TABLE workspace (id uuid NOT NULL);`); err != nil {
		t.Fatal(err)
	}
	files := []string{"465_platform_admin", "466_organization_id_unique", "467_organization_internal_unique", "468_organization_workspace_unique", "469_platform_role_user_unique", "470_admin_operation_id_unique", "471_admin_operation_actor_key_unique", "472_admin_audit_id_unique", "473_admin_audit_scope_time", "474_organization_workspace_scope"}
	for _, name := range files {
		applyMigrationFile(t, ctx, c, name+".up.sql")
	}
	if _, err = c.Exec(ctx, `INSERT INTO workspace (id) SELECT gen_random_uuid() FROM generate_series(1,501)`); err != nil {
		t.Fatal(err)
	}
	q := db.New(c)
	if _, err = q.EnsureInternalOrganization(ctx); err != nil {
		t.Fatal(err)
	}
	for _, want := range []int{500, 1, 0} {
		rows, err := q.BackfillWorkspaceOrganizations(ctx)
		if err != nil || len(rows) != want {
			t.Fatalf("bounded organization backfill: got %d want %d, %v", len(rows), want, err)
		}
	}
	if missing, err := q.CountUnassignedWorkspaces(ctx); err != nil || missing != 0 {
		t.Fatalf("unassigned workspace count: %d %v", missing, err)
	}
	var n int
	if err = c.QueryRow(ctx, `SELECT count(*) FROM pg_constraint WHERE connamespace=current_schema()::regnamespace AND contype='f'`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("foreign keys: %d %v", n, err)
	}
	if err = c.QueryRow(ctx, `SELECT count(*) FROM pg_index i JOIN pg_class c ON c.oid=i.indexrelid WHERE c.relnamespace=current_schema()::regnamespace AND NOT indisvalid`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("invalid indexes: %d %v", n, err)
	}
	if _, err = c.Exec(ctx, `INSERT INTO admin_audit_event (organization_id,actor_kind,target_kind,target_id,action,phase,request_id,reason,before_state,after_state,result_code) VALUES (gen_random_uuid(),'deployment_operator','user',gen_random_uuid(),'role_change','committed','migration-test','bootstrap','{}','{}','applied')`); err != nil {
		t.Fatal(err)
	}
	for i := len(files) - 1; i >= 0; i-- {
		_, err = c.Exec(ctx, readMigrationFile(t, files[i]+".down.sql"))
		if err == nil || !strings.Contains(err.Error(), "platform administration data exists") {
			t.Fatalf("%s should preserve security data: %v", files[i], err)
		}
	}
	if err = c.QueryRow(ctx, `SELECT count(*) FROM admin_audit_event`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("lost audit: %d %v", n, err)
	}
}
