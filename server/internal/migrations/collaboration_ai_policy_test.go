package migrations

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/multica-ai/multica/server/internal/testutil"
)

func newCollaborationAIPolicyMigrationFixture(t *testing.T) *testutil.Fixture {
	t.Helper()
	url := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if url == "" {
		t.Skip("requires task-owned DATABASE_URL")
	}
	ctx := context.Background()
	base, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = base.Close(ctx) })
	schema := "collaboration_ai_policy_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	quoted := pgx.Identifier{schema}.Sanitize()
	if _, err = base.Exec(ctx, "CREATE SCHEMA "+quoted); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if t.Failed() {
			t.Logf("retained failed migration schema %s", schema)
			return
		}
		_, _ = base.Exec(ctx, "DROP SCHEMA "+quoted+" CASCADE")
	})
	config, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	config.MaxConns, config.MinConns = 2, 0
	config.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	fx := testutil.New(pool, "", "")
	fx.Exec(t, "CREATE TABLE issue_collaboration (LIKE public.issue_collaboration INCLUDING DEFAULTS INCLUDING CONSTRAINTS)")
	fx.Exec(t, "ALTER TABLE issue_collaboration DROP COLUMN IF EXISTS ai_policy")
	return fx
}

func TestCollaborationAIPolicyMigrationPreservesDefaultAndGuardsDowngrade(t *testing.T) {
	ctx := context.Background()
	fx := newCollaborationAIPolicyMigrationFixture(t)
	read := func(direction string) string {
		t.Helper()
		body, err := os.ReadFile(filepath.Join("..", "..", "migrations", "764_collaboration_ai_policy."+direction+".sql"))
		if err != nil {
			t.Fatal(err)
		}
		return string(body)
	}
	up, down := read("up"), read("down")
	snapshot := func(includePolicy bool) string {
		t.Helper()
		query := `SELECT jsonb_agg(to_jsonb(c) ORDER BY issue_id)::text FROM issue_collaboration c`
		if !includePolicy {
			query = `SELECT jsonb_agg(to_jsonb(c)-'ai_policy' ORDER BY issue_id)::text FROM issue_collaboration c`
		}
		var rows string
		fx.QueryRow(t, query).Scan(&rows)
		return rows
	}
	first, second := uuid.NewString(), uuid.NewString()
	fx.InsertNoID(t, "issue_collaboration", testutil.Cols{"issue_id": first, "revision": int64(9007199254740993), "lifecycle_epoch": 7, "authority_epoch": 11,
		"active_information_request_id": uuid.NewString()}, "issue_id=$1", first)
	fx.InsertNoID(t, "issue_collaboration", testutil.Cols{"issue_id": second}, "issue_id=$1", second)
	beforeUpgrade := snapshot(false)
	fx.Exec(t, up)
	if snapshot(false) != beforeUpgrade || fx.Count(t, `SELECT count(*) FROM issue_collaboration WHERE ai_policy='assisted'`) != 2 {
		t.Fatal("upgrade changed retained collaboration facts or failed to default existing tasks to assisted")
	}
	fx.Exec(t, `UPDATE issue_collaboration SET ai_policy='manual' WHERE issue_id=$1`, first)
	if fx.Count(t, `SELECT count(*) FROM issue_collaboration WHERE issue_id=$1 AND ai_policy='manual'`, first) != 1 {
		t.Fatal("valid explicit manual policy did not persist")
	}
	for _, tc := range []struct {
		name  string
		value any
		code  string
	}{
		{"unknown", "future_policy", "23514"}, {"empty", "", "23514"}, {"null", nil, "23502"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := snapshot(true)
			_, err := fx.Pool.Exec(ctx, `UPDATE issue_collaboration SET ai_policy=$2 WHERE issue_id=$1`, first, tc.value)
			var invalid *pgconn.PgError
			if !errors.As(err, &invalid) || invalid.Code != tc.code || snapshot(true) != before {
				t.Fatalf("invalid policy %s was accepted or changed retained data: %v", tc.name, err)
			}
		})
	}
	beforeRefusal := snapshot(true)
	_, err := fx.Pool.Exec(ctx, down)
	var retained *pgconn.PgError
	if !errors.As(err, &retained) || retained.Code != "P0001" || !strings.Contains(retained.Message, "manual tasks remain") {
		t.Fatalf("actual down migration did not refuse retained manual tasks: %v", err)
	}
	if fx.Count(t, `SELECT count(*) FROM information_schema.columns WHERE table_schema=current_schema() AND table_name='issue_collaboration' AND column_name='ai_policy'`) != 1 ||
		snapshot(true) != beforeRefusal {
		t.Fatal("refused downgrade removed policy schema or changed retained collaboration data")
	}
	// Explicitly move the manual task to the other permitted policy before
	// attempting maintenance downgrade; deleting the task would not prove this.
	fx.Exec(t, `UPDATE issue_collaboration SET ai_policy='assisted' WHERE issue_id=$1`, first)
	beforeDowngrade := snapshot(false)
	holder, err := fx.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer holder.Rollback(ctx)
	if _, err = holder.Exec(ctx, `INSERT INTO issue_collaboration (issue_id, ai_policy) VALUES ($1,'manual')`, uuid.NewString()); err != nil {
		t.Fatal(err)
	}
	bounded, cancel := context.WithTimeout(ctx, 5*time.Second)
	_, err = fx.Pool.Exec(bounded, down)
	cancel()
	var busy *pgconn.PgError
	if !errors.As(err, &busy) || busy.Code != "55P03" {
		t.Fatalf("down migration waited through an uncommitted manual-policy writer: %v", err)
	}
	if err = holder.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if snapshot(false) != beforeDowngrade || fx.Count(t, `SELECT count(*) FROM issue_collaboration WHERE ai_policy='manual'`) != 0 {
		t.Fatal("busy downgrade or rolled-back writer changed retained tasks")
	}
	fx.Exec(t, down)
	if fx.Count(t, `SELECT count(*) FROM information_schema.columns WHERE table_schema=current_schema() AND table_name='issue_collaboration' AND column_name='ai_policy'`) != 0 ||
		snapshot(false) != beforeDowngrade {
		t.Fatal("permitted downgrade left the policy column or changed other collaboration facts")
	}
	fx.Exec(t, up)
	if fx.Count(t, `SELECT count(*) FROM issue_collaboration WHERE ai_policy='assisted'`) != 2 || snapshot(false) != beforeDowngrade {
		t.Fatal("reapplying the actual upgrade failed to restore defaults or changed retained tasks")
	}
	fresh := uuid.NewString()
	fx.InsertNoID(t, "issue_collaboration", testutil.Cols{"issue_id": fresh}, "issue_id=$1", fresh)
	if fx.Count(t, `SELECT count(*) FROM issue_collaboration WHERE issue_id=$1 AND ai_policy='assisted'`, fresh) != 1 {
		t.Fatal("reapplied upgrade lost the default for newly created tasks")
	}
}
