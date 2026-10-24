package main

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/multica-ai/multica/server/internal/testutil"
)

var resourceOfferMigrationVersions = []string{
	"732_collaboration_resource_offers", "733_collaboration_resource_offer_id", "734_collaboration_resource_offer_issue_history",
}

func resourceOfferMigrationFixture(t *testing.T) (context.Context, *testutil.Fixture, string, runOptions) {
	t.Helper()
	// Reuse the owned private-schema fixture and actual migration runner. Only
	// this test's migration file selection differs from the Switch rehearsal.
	ctx, fx, schema, opts := switchMigrationFixture(t)
	opts.Files = realMigrationFiles(t, resourceOfferMigrationVersions, "up")
	return ctx, fx, schema, opts
}

func resourceOfferMigrationCols(issue, id string) testutil.Cols {
	owner, agent, runtime, workspace := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	return testutil.Cols{"id": id, "issue_id": issue, "offered_by": owner, "lifecycle_epoch": 1, "kind": "agent", "resource_id": agent,
		"execution_workspace_id": workspace, "configuration_fingerprint": strings.Repeat("a", 64),
		"roster":             fmt.Sprintf(`[{"role":"agent","agent_id":"%s","runtime_id":"%s","execution_workspace_id":"%s","owner_user_id":"%s"}]`, agent, runtime, workspace, owner),
		"resource_authority": fmt.Sprintf(`[{"agent_id":"%s","owner_user_id":"%s","owner_auth_epoch":"1","owner_participant_epoch":"1","identity":{"kind":"enrolled","installation_id":"%s","binding_id":"%s","binding_epoch":"1"}}]`, agent, owner, uuid.NewString(), uuid.NewString())}
}

func resourceOfferMigrationState(t *testing.T, ctx context.Context, pool *pgxpool.Pool, schema string) string {
	t.Helper()
	var rows string
	if err := pool.QueryRow(ctx, `SELECT COALESCE(jsonb_agg(to_jsonb(o) ORDER BY id),'[]'::jsonb)::text FROM collaboration_resource_offer o`).Scan(&rows); err != nil {
		t.Fatal(err)
	}
	return iterationRecoveryCatalog(t, ctx, pool, schema) + rows
}

func TestCollaborationResourceOfferMigrationsPreserveHistoryAndRefuseDowngrade(t *testing.T) {
	for _, populated := range []bool{false, true} {
		t.Run(fmt.Sprintf("populated_%t", populated), func(t *testing.T) {
			ctx, fx, schema, opts := resourceOfferMigrationFixture(t)
			first := opts
			first.Files = first.Files[:1]
			if err := runMigrations(ctx, fx.Pool, first); err != nil {
				t.Fatal(err)
			}
			if fx.Count(t, `SELECT count(*) FROM pg_index WHERE indrelid='collaboration_resource_offer'::regclass`) != 0 {
				t.Fatal("offer table creation introduced an inline index")
			}
			if populated {
				fx.Insert(t, "collaboration_resource_offer", resourceOfferMigrationCols(uuid.NewString(), uuid.NewString()))
			}
			if err := runMigrations(ctx, fx.Pool, opts); err != nil {
				t.Fatal(err)
			}
			if fx.Count(t, `SELECT count(*) FROM pg_index WHERE indrelid='collaboration_resource_offer'::regclass AND indisvalid AND indisready AND indislive`) != 2 ||
				fx.Count(t, `SELECT count(*) FROM pg_constraint WHERE conrelid='collaboration_resource_offer'::regclass AND contype IN ('f','p')`) != 0 {
				t.Fatal("resource offers lack two usable concurrent indexes or gained an FK/inline primary key")
			}
			before := resourceOfferMigrationState(t, ctx, fx.Pool, schema)
			if err := runMigrations(ctx, fx.Pool, opts); err != nil {
				t.Fatal(err)
			}
			if after := resourceOfferMigrationState(t, ctx, fx.Pool, schema); after != before {
				t.Fatal("repeated up changed offer history, ledger timestamps or index identity")
			}
			reverse := slices.Clone(resourceOfferMigrationVersions)
			slices.Reverse(reverse)
			for _, version := range reverse {
				down := opts
				down.Direction, down.Files, down.Hooks = "down", realMigrationFiles(t, []string{version}, "down"), hooksForDirection("down")
				var refusal *pgconn.PgError
				if err := runMigrations(ctx, fx.Pool, down); !errors.As(err, &refusal) || refusal.Code != "P0001" {
					t.Fatalf("%s downgrade did not refuse, including empty maintenance state: %v", version, err)
				}
				if after := resourceOfferMigrationState(t, ctx, fx.Pool, schema); after != before {
					t.Fatal("refused down changed catalog, ledger or resource offer history")
				}
			}
		})
	}
}

func TestCollaborationResourceOfferMigrationsEnforceStateConstraints(t *testing.T) {
	ctx, fx, schema, opts := resourceOfferMigrationFixture(t)
	if err := runMigrations(ctx, fx.Pool, opts); err != nil {
		t.Fatal(err)
	}
	id := fx.Insert(t, "collaboration_resource_offer", resourceOfferMigrationCols(uuid.NewString(), uuid.NewString()))
	before := resourceOfferMigrationState(t, ctx, fx.Pool, schema)
	for _, invalid := range []string{
		"lifecycle_epoch=0", "version=0", "kind='unknown'", "state='unknown'", "configuration_fingerprint='short'",
		"roster='{}'::jsonb", "roster='[]'::jsonb", "resource_authority='[]'::jsonb", "state='withdrawn'", "state='adopted'",
		"withdrawn_at=now()", "adopted_at=now()", "adopted_by=gen_random_uuid()", "target_id=gen_random_uuid()",
	} {
		var constraint *pgconn.PgError
		_, err := fx.Pool.Exec(ctx, "UPDATE collaboration_resource_offer SET "+invalid+" WHERE id=$1", id)
		// PostgreSQL may evaluate jsonb_array_length before the type guard.
		// Both the CHECK and that array-type refusal must reject this write.
		if !errors.As(err, &constraint) || constraint.Code != "23514" && !(invalid == "roster='{}'::jsonb" && constraint.Code == "22023") {
			t.Fatalf("invalid offer state %s was not rejected by its constraint: %v", invalid, err)
		}
	}
	if _, err := fx.Pool.Exec(ctx, `INSERT INTO collaboration_resource_offer SELECT * FROM collaboration_resource_offer WHERE id=$1`, id); err == nil {
		t.Fatal("offer identity index allowed duplicate IDs")
	}
	if after := resourceOfferMigrationState(t, ctx, fx.Pool, schema); after != before {
		t.Fatal("constraint or type refusals mutated retained offer history")
	}
	fx.Exec(t, "UPDATE collaboration_resource_offer SET state='withdrawn',version=2,withdrawn_at=now() WHERE id=$1", id)
	adopted := fx.Insert(t, "collaboration_resource_offer", resourceOfferMigrationCols(uuid.NewString(), uuid.NewString()))
	fx.Exec(t, "UPDATE collaboration_resource_offer SET state='adopted',version=2,adopted_at=now(),adopted_by=$2,target_id=$3 WHERE id=$1", adopted, uuid.NewString(), uuid.NewString())
	if fx.Count(t, "SELECT count(*) FROM collaboration_resource_offer WHERE state IN ('withdrawn','adopted') AND version=2") != 2 {
		t.Fatal("valid terminal offer facts could not be retained")
	}
}

func TestCollaborationResourceOfferMigrationsRepairInvalidIdentityIndex(t *testing.T) {
	ctx, fx, schema, opts := resourceOfferMigrationFixture(t)
	first := opts
	first.Files = first.Files[:1]
	if err := runMigrations(ctx, fx.Pool, first); err != nil {
		t.Fatal(err)
	}
	id, issue := uuid.NewString(), uuid.NewString()
	for i := 0; i < 2; i++ {
		fx.Insert(t, "collaboration_resource_offer", resourceOfferMigrationCols(issue, id))
	}
	var duplicate *pgconn.PgError
	if err := runMigrations(ctx, fx.Pool, opts); !errors.As(err, &duplicate) || duplicate.Code != "23505" {
		t.Fatalf("duplicate identity did not fail the real concurrent index build: %v", err)
	}
	assertIndexValidity(t, fx.Pool, schema, "idx_collaboration_resource_offer_id", false)
	if fx.Count(t, "SELECT count(*) FROM schema_migrations") != 1 {
		t.Fatal("failed identity migration was recorded as complete")
	}
	// Reconcile only this deliberate duplicate in the owned private schema.
	fx.Exec(t, "DELETE FROM collaboration_resource_offer WHERE ctid=(SELECT max(ctid) FROM collaboration_resource_offer)")
	if err := runMigrations(ctx, fx.Pool, opts); err != nil {
		t.Fatalf("registered identity-index recovery failed: %v", err)
	}
	assertIndexValidity(t, fx.Pool, schema, "idx_collaboration_resource_offer_id", true)
	assertIndexValidity(t, fx.Pool, schema, "idx_collaboration_resource_offer_issue_history", true)
	if fx.Count(t, "SELECT count(*) FROM schema_migrations") != 3 || fx.Count(t, "SELECT count(*) FROM collaboration_resource_offer") != 1 {
		t.Fatal("index recovery lost the retained offer or failed to complete migrations")
	}
}

func TestCollaborationResourceOfferMigrationsRepairInterruptedHistoryIndex(t *testing.T) {
	ctx, fx, schema, opts := resourceOfferMigrationFixture(t)
	first := opts
	first.Files = first.Files[:2]
	if err := runMigrations(ctx, fx.Pool, first); err != nil {
		t.Fatal(err)
	}
	id := fx.Insert(t, "collaboration_resource_offer", resourceOfferMigrationCols(uuid.NewString(), uuid.NewString()))
	writer, err := fx.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Rollback(ctx)
	if _, err := writer.Exec(ctx, "UPDATE collaboration_resource_offer SET created_at=created_at WHERE id=$1", id); err != nil {
		t.Fatal(err)
	}
	blocker := int(writer.Conn().PgConn().PID())
	buildCtx, cancelBuild := context.WithCancel(ctx)
	defer cancelBuild()
	done := make(chan error, 1)
	go func() { done <- runMigrations(buildCtx, fx.Pool, opts) }()
	deadline := time.Now().Add(10 * time.Second)
	for {
		var waiting bool
		if err := fx.Pool.QueryRow(ctx, `SELECT EXISTS (
			SELECT 1 FROM pg_index i JOIN pg_class c ON c.oid=i.indexrelid
			WHERE c.relnamespace=$1::regnamespace AND c.relname='idx_collaboration_resource_offer_issue_history' AND NOT i.indisvalid)
			AND EXISTS (SELECT 1 FROM pg_stat_activity WHERE $2::int=ANY(pg_blocking_pids(pid)))`, schema, blocker).Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if waiting {
			break
		}
		select {
		case err := <-done:
			t.Fatalf("history build did not retain the expected concurrent-index barrier: %v", err)
		default:
		}
		if time.Now().After(deadline) {
			t.Fatal("history index did not reach its real concurrent build wait")
		}
	}
	cancelBuild()
	if err := <-done; err == nil {
		t.Fatal("interrupted concurrent history index was reported as complete")
	}
	if err := writer.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	assertIndexValidity(t, fx.Pool, schema, "idx_collaboration_resource_offer_issue_history", false)
	if fx.Count(t, "SELECT count(*) FROM schema_migrations") != 2 {
		t.Fatal("interrupted history index advanced the migration ledger")
	}
	if err := runMigrations(ctx, fx.Pool, opts); err != nil {
		t.Fatalf("registered cleanup failed to replace the real interrupted index: %v", err)
	}
	assertIndexValidity(t, fx.Pool, schema, "idx_collaboration_resource_offer_issue_history", true)
	if fx.Count(t, "SELECT count(*) FROM schema_migrations") != 3 || fx.Count(t, "SELECT count(*) FROM collaboration_resource_offer") != 1 {
		t.Fatal("history-index retry lost a retained offer or migration")
	}
}
