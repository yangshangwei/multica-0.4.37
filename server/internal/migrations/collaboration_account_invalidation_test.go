package migrations

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/multica-ai/multica/server/internal/testutil"
)

func TestCollaborationAccountInvalidationMigrationsPreserveRevocation(t *testing.T) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("requires task-owned DATABASE_URL")
	}
	ctx := context.Background()
	base, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = base.Close(ctx) })
	schema := "collaboration_invalidation_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	quoted := pgx.Identifier{schema}.Sanitize()
	if _, err = base.Exec(ctx, "CREATE SCHEMA "+quoted); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = base.Exec(ctx, "DROP SCHEMA "+quoted+" CASCADE") })
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	fx := testutil.New(pool, "", "")
	names := []string{"613_collaboration_account_invalidation", "614_collaboration_account_invalidation_credential", "615_collaboration_account_invalidation_epoch", "616_collaboration_account_invalidation_pending"}
	read := func(name, direction string) string {
		t.Helper()
		data, err := os.ReadFile(filepath.Join("..", "..", "migrations", name+"."+direction+".sql"))
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	for _, name := range names {
		fx.Exec(t, read(name, "up"))
	}
	if fx.Count(t, `SELECT count(*) FROM pg_index i JOIN pg_class c ON c.oid=i.indexrelid WHERE c.relnamespace=current_schema()::regnamespace AND i.indisvalid`) != 3 {
		t.Fatal("revocation indexes not all valid")
	}
	if fx.Count(t, `SELECT count(*) FROM pg_constraint WHERE connamespace=current_schema()::regnamespace AND contype='f'`) != 0 {
		t.Fatal("migration added foreign keys")
	}
	user := uuid.NewString()
	insert := `INSERT INTO collaboration_account_invalidation(user_id,account_epoch,revocation_key,credential_version,reason) VALUES($1,$2,$3,$4,'credentials_revoked')`
	// A legacy account has no password credential version, but its two real
	// administrative operations must retain distinct durable invalidations.
	fx.Exec(t, insert, user, 2, "admin:disable", nil)
	fx.Exec(t, insert, user, 3, "admin:restore", nil)
	for _, args := range [][]any{{user, 4, "admin:disable", nil}, {user, 3, "admin:other", nil}} {
		if _, err = pool.Exec(ctx, insert, args...); err == nil {
			t.Fatal("duplicate revocation key or account epoch was accepted")
		}
	}
	fx.Exec(t, `UPDATE collaboration_account_invalidation SET reconciled_at=now()`)
	for _, name := range names {
		if _, err = pool.Exec(ctx, read(name, "down")); err == nil || !strings.Contains(err.Error(), "retained collaboration account invalidation") {
			t.Fatalf("%s did not retain processed revocation: %v", name, err)
		}
	}
	fx.Exec(t, `DELETE FROM collaboration_account_invalidation WHERE user_id=$1`, user)
	holder, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer holder.Rollback(ctx)
	if _, err = holder.Exec(ctx, insert, user, 4, "admin:in_flight", nil); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, read(names[3], "down")); err == nil || !strings.Contains(err.Error(), "55P03") {
		t.Fatalf("rollback ignored uncommitted first invalidation: %v", err)
	}
	if err = holder.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	for i := len(names) - 1; i >= 0; i-- {
		fx.Exec(t, read(names[i], "down"))
	}
	if fx.Count(t, `SELECT count(*) FROM pg_class WHERE relnamespace=current_schema()::regnamespace AND relname='collaboration_account_invalidation'`) != 0 {
		t.Fatal("empty fixture schema did not roll back")
	}
}
