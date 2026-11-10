package main

import (
	"context"
	"os"
	"slices"
	"testing"

	"github.com/multica-ai/multica/server/internal/testutil"
)

var deliveryProjectChangeVersions = []string{"789_delivery_project_change", "790_delivery_project_change_operation", "791_delivery_project_change_ready"}

func deliveryProjectChangeFixture(t *testing.T) (context.Context, *testutil.Fixture, string, runOptions) {
	t.Helper()
	ctx, fx, schema, opts := switchMigrationFixture(t)
	opts.Files = realMigrationFiles(t, deliveryProjectChangeVersions, "up")
	return ctx, fx, schema, opts
}
func deliveryProjectChangeDown(t *testing.T, opts runOptions) runOptions {
	t.Helper()
	versions := slices.Clone(deliveryProjectChangeVersions)
	slices.Reverse(versions)
	opts.Direction = "down"
	opts.Hooks = hooksForDirection("down")
	opts.Files = realMigrationFiles(t, versions, "down")
	return opts
}

const projectChangeInsert = `INSERT INTO delivery_project_change
 (workspace_id,operation_id,issue_id,issue_revision,actor_kind,actor_id,actor_user_id,from_project_id,to_project_id,from_project_revision,to_project_revision,from_policy_revision,to_policy_revision,reason)
 VALUES($1,$2,$3,2,'human',$4,$4,$5,NULL,1,0,1,0,'controlled scope adjustment')`

func TestDeliveryProjectChangeMigrationsUpgradeAndEmptyRollback(t *testing.T) {
	ctx, fx, schema, opts := deliveryProjectChangeFixture(t)
	table := opts
	table.Files = table.Files[:1]
	if err := runMigrations(ctx, fx.Pool, table); err != nil {
		t.Fatal(err)
	}
	if n := fx.Count(t, `SELECT count(*) FROM pg_index WHERE indrelid='delivery_project_change'::regclass`); n != 0 {
		t.Fatal("table introduced implicit indexes")
	}
	if n := fx.Count(t, `SELECT count(*) FROM pg_constraint WHERE conrelid='delivery_project_change'::regclass AND contype IN ('f','u','p')`); n != 0 {
		t.Fatal("table introduced foreign keys or implicit uniqueness")
	}
	if err := runMigrations(ctx, fx.Pool, opts); err != nil {
		t.Fatal(err)
	}
	if n := fx.Count(t, `SELECT count(*) FROM pg_index WHERE indrelid='delivery_project_change'::regclass AND indisunique AND indisvalid AND indisready AND indislive`); n != 1 {
		t.Fatal("missing actual usable concurrent index")
	}
	before := iterationRecoveryCatalog(t, ctx, fx.Pool, schema)
	if err := runMigrations(ctx, fx.Pool, opts); err != nil {
		t.Fatal(err)
	}
	if after := iterationRecoveryCatalog(t, ctx, fx.Pool, schema); after != before {
		t.Fatal("upgrade replay changed catalog")
	}
	if err := runMigrations(ctx, fx.Pool, deliveryProjectChangeDown(t, opts)); err != nil {
		t.Fatal(err)
	}
	if fx.Count(t, `SELECT count(*) FROM pg_class WHERE oid=to_regclass('delivery_project_change')`) != 0 {
		t.Fatal("empty downgrade retained table")
	}
}
func TestDeliveryProjectChangeMigrationsRetainAdjustment(t *testing.T) {
	ctx, fx, schema, opts := deliveryProjectChangeFixture(t)
	if err := runMigrations(ctx, fx.Pool, opts); err != nil {
		t.Fatal(err)
	}
	fx.InsertNoID(t, "delivery_project_change", testutil.Cols{"workspace_id": artifactUUID(), "operation_id": artifactUUID(), "issue_id": artifactUUID(), "issue_revision": int64(2), "actor_kind": "human", "actor_id": artifactUUID(), "actor_user_id": artifactUUID(), "from_project_id": artifactUUID(), "from_project_revision": int64(1), "to_project_revision": int64(0), "from_policy_revision": int64(1), "to_policy_revision": int64(0), "reason": "retained adjustment"}, "true")
	before := iterationRecoveryCatalog(t, ctx, fx.Pool, schema)
	artifactPGCode(t, runMigrations(ctx, fx.Pool, deliveryProjectChangeDown(t, opts)), "P0001")
	for _, version := range []string{deliveryProjectChangeVersions[0], deliveryProjectChangeVersions[2]} {
		body, err := os.ReadFile(realMigrationFiles(t, []string{version}, "down")[0])
		if err != nil {
			t.Fatal(err)
		}
		_, err = fx.Pool.Exec(ctx, string(body))
		artifactPGCode(t, err, "P0001")
	}
	if fx.Count(t, "SELECT count(*) FROM delivery_project_change") != 1 || iterationRecoveryCatalog(t, ctx, fx.Pool, schema) != before {
		t.Fatal("refused downgrade changed retained adjustment or catalog")
	}
}
func TestDeliveryProjectChangeMigrationsFenceConcurrentAdjustment(t *testing.T) {
	ctx, fx, _, opts := deliveryProjectChangeFixture(t)
	if err := runMigrations(ctx, fx.Pool, opts); err != nil {
		t.Fatal(err)
	}
	tx, err := fx.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, projectChangeInsert, artifactUUID(), artifactUUID(), artifactUUID(), artifactUUID(), artifactUUID()); err != nil {
		t.Fatal(err)
	}
	for _, version := range []string{deliveryProjectChangeVersions[0], deliveryProjectChangeVersions[2]} {
		body, err := os.ReadFile(realMigrationFiles(t, []string{version}, "down")[0])
		if err != nil {
			t.Fatal(err)
		}
		_, err = fx.Pool.Exec(ctx, string(body))
		artifactPGCode(t, err, "55P03")
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	artifactPGCode(t, runMigrations(ctx, fx.Pool, deliveryProjectChangeDown(t, opts)), "P0001")
	if fx.Count(t, "SELECT count(*) FROM delivery_project_change") != 1 {
		t.Fatal("concurrent retained fact lost")
	}
}
func TestDeliveryProjectChangeMigrationsRejectMalformedIndex(t *testing.T) {
	ctx, fx, _, opts := deliveryProjectChangeFixture(t)
	if err := runMigrations(ctx, fx.Pool, opts); err != nil {
		t.Fatal(err)
	}
	if _, err := fx.Pool.Exec(ctx, "DROP INDEX delivery_project_change_operation_idx"); err != nil {
		t.Fatal(err)
	}
	if _, err := fx.Pool.Exec(ctx, "CREATE UNIQUE INDEX delivery_project_change_operation_idx ON delivery_project_change(workspace_id,issue_id,operation_id)"); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(realMigrationFiles(t, []string{deliveryProjectChangeVersions[2]}, "up")[0])
	if err != nil {
		t.Fatal(err)
	}
	_, err = fx.Pool.Exec(ctx, string(body))
	artifactPGCode(t, err, "P0001")
}
