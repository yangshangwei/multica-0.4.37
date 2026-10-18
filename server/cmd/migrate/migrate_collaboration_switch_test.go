package main

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/multica-ai/multica/server/internal/testutil"
)

var switchMigrationVersions = []string{
	"713_collaboration_switch_proposals", "714_collaboration_switch_proposal_id",
	"715_collaboration_switch_consent_resource", "716_collaboration_switch_proposal_target",
}

func switchMigrationFixture(t *testing.T) (context.Context, *testutil.Fixture, string, runOptions) {
	t.Helper()
	if os.Getenv("DATABASE_URL") == "" {
		t.Skip("requires explicit isolated DATABASE_URL")
	}
	admin := openTestPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	t.Cleanup(cancel)
	schema := "migrate_switch_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+pgx.Identifier{schema}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanup, stop := context.WithTimeout(context.Background(), 10*time.Second)
		defer stop()
		if _, err := admin.Exec(cleanup, "DROP SCHEMA "+pgx.Identifier{schema}.Sanitize()+" CASCADE"); err != nil {
			t.Errorf("remove owned Switch migration schema: %v", err)
		}
	})
	pool := openTestPoolWithSearchPath(t, schema)
	fx := testutil.New(pool, "", "")
	opts := runOptions{Direction: "up", Files: realMigrationFiles(t, switchMigrationVersions, "up"),
		SchemaMigrationsTable: schema + ".schema_migrations", AdvisoryLockKey: int64(rand.Uint64()&0x7fffffffffffffff) | 1,
		Hooks: hooksForDirection("up")}
	return ctx, fx, schema, opts
}

func switchMigrationState(t *testing.T, ctx context.Context, pool *pgxpool.Pool, schema string) string {
	t.Helper()
	var state string
	if err := pool.QueryRow(ctx, `SELECT jsonb_build_object(
		'catalog',$1::text,
		'ledger',(SELECT jsonb_agg(to_jsonb(m) ORDER BY version) FROM schema_migrations m),
		'proposals',(SELECT jsonb_agg(to_jsonb(p) ORDER BY id) FROM collaboration_switch_proposal p),
		'consents',(SELECT jsonb_agg(to_jsonb(c) ORDER BY proposal_id,agent_id) FROM collaboration_switch_consent c))::text`,
		iterationRecoveryCatalog(t, ctx, pool, schema)).Scan(&state); err != nil {
		t.Fatal(err)
	}
	return state
}

func TestCollaborationSwitchMigrationsPreserveAndRefuseDowngrade(t *testing.T) {
	ctx, fx, schema, opts := switchMigrationFixture(t)
	opts.Files = opts.Files[:1]
	if err := runMigrations(ctx, fx.Pool, opts); err != nil {
		t.Fatal(err)
	}
	if fx.Count(t, `SELECT count(*) FROM pg_index WHERE indrelid IN ('collaboration_switch_proposal'::regclass,'collaboration_switch_consent'::regclass)`) != 0 {
		t.Fatal("Switch schema created an inline identity index")
	}
	proposal := fx.Insert(t, "collaboration_switch_proposal", testutil.Cols{
		"issue_id": uuid.NewString(), "session_id": uuid.NewString(), "target_id": uuid.NewString(), "proposed_by": uuid.NewString(),
		"lifecycle_epoch": 1, "base_target_epoch": 1, "base_config_version": 1, "roster": "[]", "configuration_fingerprint": strings.Repeat("a", 64),
	})
	frozen := testutil.Cols{"proposal_id": proposal, "agent_id": uuid.NewString(), "owner_user_id": uuid.NewString(),
		"owner_auth_epoch": 1, "owner_participant_epoch": 1, "identity_kind": "enrolled", "installation_id": uuid.NewString(),
		"binding_id": uuid.NewString(), "binding_epoch": 1}
	frozenAgent := frozen["agent_id"]
	fx.InsertNoID(t, "collaboration_switch_consent", frozen, "proposal_id=$1 AND agent_id=$2", proposal, frozenAgent)
	opts.Files = realMigrationFiles(t, switchMigrationVersions, "up")
	if err := runMigrations(ctx, fx.Pool, opts); err != nil {
		t.Fatal(err)
	}
	if fx.Count(t, `SELECT count(*) FROM pg_index WHERE indrelid IN ('collaboration_switch_proposal'::regclass,'collaboration_switch_consent'::regclass) AND indisvalid AND indisready AND indislive`) != 3 ||
		fx.Count(t, `SELECT count(*) FROM pg_constraint WHERE conrelid IN ('collaboration_switch_proposal'::regclass,'collaboration_switch_consent'::regclass) AND contype='f'`) != 0 {
		t.Fatal("Switch upgrade lacks usable concurrent indexes or introduced a foreign key")
	}
	for _, statement := range []string{
		`INSERT INTO collaboration_switch_proposal SELECT * FROM collaboration_switch_proposal`,
		`INSERT INTO collaboration_switch_consent SELECT * FROM collaboration_switch_consent`,
	} {
		if _, err := fx.Pool.Exec(ctx, statement); err == nil {
			t.Fatalf("Switch identity index allowed duplicate retained consent: %s", statement)
		}
	}
	before := switchMigrationState(t, ctx, fx.Pool, schema)
	if err := runMigrations(ctx, fx.Pool, opts); err != nil {
		t.Fatal(err)
	}
	if after := switchMigrationState(t, ctx, fx.Pool, schema); after != before {
		t.Fatal("upgrade replay changed catalog, ledger or staged consent history")
	}
	reverse := slices.Clone(switchMigrationVersions)
	slices.Reverse(reverse)
	for _, version := range reverse {
		t.Run("refuse_"+version, func(t *testing.T) {
			down := opts
			down.Direction, down.Files, down.Hooks = "down", realMigrationFiles(t, []string{version}, "down"), hooksForDirection("down")
			var refusal *pgconn.PgError
			if err := runMigrations(ctx, fx.Pool, down); !errors.As(err, &refusal) || refusal.Code != "P0001" {
				t.Fatalf("Switch downgrade did not refuse old execution semantics: %v", err)
			}
			if after := switchMigrationState(t, ctx, fx.Pool, schema); after != before {
				t.Fatal("refused downgrade removed consent, index or ledger history")
			}
		})
	}
}

func TestCollaborationSwitchMigrationsRepairInvalidConsentIndex(t *testing.T) {
	ctx, fx, schema, opts := switchMigrationFixture(t)
	opts.Files = opts.Files[:2]
	if err := runMigrations(ctx, fx.Pool, opts); err != nil {
		t.Fatal(err)
	}
	proposal, agent := uuid.NewString(), uuid.NewString()
	for range 2 {
		fx.InsertNoID(t, "collaboration_switch_consent", testutil.Cols{
			"proposal_id": proposal, "agent_id": agent, "owner_user_id": uuid.NewString(), "owner_auth_epoch": 1,
			"owner_participant_epoch": 1, "identity_kind": "enrolled", "installation_id": uuid.NewString(), "binding_id": uuid.NewString(), "binding_epoch": 1,
		}, "proposal_id=$1 AND agent_id=$2", proposal, agent)
	}
	opts.Files = realMigrationFiles(t, switchMigrationVersions, "up")
	var duplicate *pgconn.PgError
	if err := runMigrations(ctx, fx.Pool, opts); !errors.As(err, &duplicate) || duplicate.Code != "23505" {
		t.Fatalf("negative control failed to expose duplicate proposed consent: %v", err)
	}
	assertIndexValidity(t, fx.Pool, schema, "idx_collaboration_switch_consent_resource", false)
	if fx.Count(t, `SELECT count(*) FROM schema_migrations`) != 2 {
		t.Fatal("failed consent identity build was recorded in the ledger")
	}
	// Reconcile only this deliberately duplicated private fixture before retry.
	fx.Exec(t, `DELETE FROM collaboration_switch_consent WHERE ctid=(SELECT max(ctid) FROM collaboration_switch_consent)`)
	if err := runMigrations(ctx, fx.Pool, opts); err != nil {
		t.Fatalf("registered consent identity cleanup did not recover the real migration: %v", err)
	}
	assertIndexValidity(t, fx.Pool, schema, "idx_collaboration_switch_consent_resource", true)
	if fx.Count(t, `SELECT count(*) FROM schema_migrations`) != 4 || fx.Count(t, `SELECT count(*) FROM collaboration_switch_consent`) != 1 {
		t.Fatal("index recovery lost retained consent or skipped a migration")
	}
}

func TestCollaborationNewMigrationFilesKeepConcurrentIndexBoundary(t *testing.T) {
	for _, prefix := range []string{"709", "713", "714", "715", "716", "717", "718", "719", "720", "721", "722", "723", "724", "725", "726"} {
		paths, err := filepath.Glob(filepath.Join("..", "..", "migrations", prefix+"_*.up.sql"))
		if err != nil || len(paths) != 1 {
			t.Fatalf("migration %s: paths=%v err=%v", prefix, paths, err)
		}
		body, err := os.ReadFile(paths[0])
		if err != nil {
			t.Fatal(err)
		}
		code := string(stripSQLLineComments(body))
		for _, prohibited := range []string{"REFERENCES", "FOREIGN KEY", "PRIMARY KEY"} {
			if strings.Contains(strings.ToUpper(code), prohibited) {
				t.Errorf("%s introduced %s", paths[0], prohibited)
			}
		}
		if builds := concurrentIndexNamePattern.FindAllStringSubmatch(code, -1); len(builds) != 0 {
			if len(builds) != 1 || strings.Count(code, ";") != 1 {
				t.Errorf("%s concurrent index build must be one standalone statement", paths[0])
			}
			version := strings.TrimSuffix(filepath.Base(paths[0]), ".up.sql")
			if registered := concurrentIndexCleanups[version]; registered != builds[0][1] {
				t.Errorf("%s missing its exact invalid-index recovery hook: %s", version, fmt.Sprint(builds[0][1]))
			}
		}
	}
}
