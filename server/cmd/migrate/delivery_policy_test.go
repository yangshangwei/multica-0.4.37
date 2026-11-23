package main

import (
	"context"
	"errors"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

var deliveryPolicyMigrationVersions = []string{
	"776_delivery_policy_evaluation",
	"777_project_delivery_policy_revision",
	"778_delivery_evaluation_key",
	"779_delivery_evaluation_id",
	"780_delivery_policy_evaluation_ready",
}

func deliveryPolicyMigrationFixture(t *testing.T) (context.Context, *testutil.Fixture, string, runOptions) {
	t.Helper()
	ctx, fx, schema, opts := switchMigrationFixture(t)
	opts.Files = realMigrationFiles(t, deliveryPolicyMigrationVersions, "up")
	return ctx, fx, schema, opts
}

func deliveryPolicyMigrationParams() db.CreateDeliveryPolicyParams {
	return db.CreateDeliveryPolicyParams{
		WorkspaceID: artifactUUID(), ProjectID: artifactUUID(), Revision: 1,
		PolicyVersion: "delivery-artifact-observe-v1", Mode: "observe", Stage: "done",
		Reason: "Retain an observation policy", CreatedBy: artifactUUID(),
	}
}

func deliveryEvaluationMigrationParams() db.CreateDeliveryEvaluationParams {
	return db.CreateDeliveryEvaluationParams{
		WorkspaceID: artifactUUID(), EvaluatorUserID: artifactUUID(), RequestKey: artifactUUID(),
		EvaluationID: artifactUUID(), IssueID: artifactUUID(), ProjectID: artifactUUID(), PolicyRevision: 1,
		RequestDigest: strings.Repeat("a", 64), RequestJson: []byte(`{"policy_version":"delivery-artifact-observe-v1"}`),
		Observation: []byte(`{"verdict":"unknown","reason":"evidence_missing"}`),
	}
}

func deliveryPolicyMigrationReady(t *testing.T, ctx context.Context, q *db.Queries, want bool) {
	t.Helper()
	ready, err := q.DeliveryPolicySchemaReady(ctx)
	if err != nil || !ready.Valid || ready.Bool != want {
		t.Fatalf("delivery schema ready=%+v err=%v want=%v", ready, err, want)
	}
}

func deliveryPolicyMigrationState(t *testing.T, ctx context.Context, fx *testutil.Fixture, schema string) string {
	t.Helper()
	var state string
	fx.QueryRow(t, `SELECT jsonb_build_object(
		'catalog',$1::text,
		'policies',(SELECT jsonb_agg(to_jsonb(p) ORDER BY workspace_id,project_id,revision) FROM project_delivery_policy p),
		'evaluations',(SELECT jsonb_agg(to_jsonb(e) ORDER BY evaluation_id) FROM delivery_evidence_evaluation e))::text`,
		iterationRecoveryCatalog(t, ctx, fx.Pool, schema)).Scan(&state)
	return state
}

func deliveryPolicyMigrationDown(t *testing.T, opts runOptions) runOptions {
	t.Helper()
	versions := slices.Clone(deliveryPolicyMigrationVersions)
	slices.Reverse(versions)
	opts.Direction = "down"
	opts.Hooks = hooksForDirection("down")
	opts.Files = realMigrationFiles(t, versions, "down")
	return opts
}

func TestDeliveryPolicyMigrationsUpgradeAndEmptyRollback(t *testing.T) {
	ctx, fx, schema, opts := deliveryPolicyMigrationFixture(t)
	q := db.New(fx.Pool)
	deliveryPolicyMigrationReady(t, ctx, q, false)
	first := opts
	first.Files = first.Files[:1]
	if err := runMigrations(ctx, fx.Pool, first); err != nil {
		t.Fatal(err)
	}
	if n := fx.Count(t, `SELECT count(*) FROM pg_index WHERE indrelid IN ('project_delivery_policy'::regclass,'delivery_evidence_evaluation'::regclass)`); n != 0 {
		t.Fatalf("table migration created %d implicit indexes", n)
	}
	if n := fx.Count(t, `SELECT count(*) FROM pg_constraint WHERE conrelid IN ('project_delivery_policy'::regclass,'delivery_evidence_evaluation'::regclass) AND contype IN ('f','p','u')`); n != 0 {
		t.Fatalf("tables introduced %d foreign-key or implicit uniqueness constraints", n)
	}
	deliveryPolicyMigrationReady(t, ctx, q, false)
	if err := runMigrations(ctx, fx.Pool, opts); err != nil {
		t.Fatal(err)
	}
	deliveryPolicyMigrationReady(t, ctx, q, true)
	if n := fx.Count(t, `SELECT count(*) FROM pg_index WHERE indrelid IN ('project_delivery_policy'::regclass,'delivery_evidence_evaluation'::regclass) AND indisvalid AND indisready AND indislive`); n != 3 {
		t.Fatalf("expected three usable concurrent indexes, got %d", n)
	}
	if fx.Count(t, `SELECT count(*) FROM schema_migrations`) != len(deliveryPolicyMigrationVersions) {
		t.Fatal("upgrade did not record every migration")
	}
	before := deliveryPolicyMigrationState(t, ctx, fx, schema)
	if err := runMigrations(ctx, fx.Pool, opts); err != nil {
		t.Fatal(err)
	}
	if after := deliveryPolicyMigrationState(t, ctx, fx, schema); after != before {
		t.Fatal("upgrade replay changed rows, index identity or migration ledger")
	}
	down := deliveryPolicyMigrationDown(t, opts)
	if err := runMigrations(ctx, fx.Pool, down); err != nil {
		t.Fatalf("empty maintenance rollback failed: %v", err)
	}
	deliveryPolicyMigrationReady(t, ctx, q, false)
	if fx.Count(t, `SELECT count(*) FROM pg_class WHERE oid IN (to_regclass('project_delivery_policy'),to_regclass('delivery_evidence_evaluation'))`) != 0 || fx.Count(t, `SELECT count(*) FROM schema_migrations`) != 0 {
		t.Fatal("empty rollback left a table or a ledger entry")
	}
}

func TestDeliveryPolicyMigrationsRetainEachHistory(t *testing.T) {
	for _, table := range []string{"project_delivery_policy", "delivery_evidence_evaluation"} {
		t.Run(table, func(t *testing.T) {
			ctx, fx, schema, opts := deliveryPolicyMigrationFixture(t)
			if err := runMigrations(ctx, fx.Pool, opts); err != nil {
				t.Fatal(err)
			}
			q := db.New(fx.Pool)
			if table == "project_delivery_policy" {
				if _, err := q.CreateDeliveryPolicy(ctx, deliveryPolicyMigrationParams()); err != nil {
					t.Fatal(err)
				}
			} else if _, err := q.CreateDeliveryEvaluation(ctx, deliveryEvaluationMigrationParams()); err != nil {
				t.Fatal(err)
			}
			before := deliveryPolicyMigrationState(t, ctx, fx, schema)
			down := deliveryPolicyMigrationDown(t, opts)
			artifactPGCode(t, runMigrations(ctx, fx.Pool, down), "P0001")
			if after := deliveryPolicyMigrationState(t, ctx, fx, schema); after != before {
				t.Fatal("highest down removed retained rows, indexes or ledger")
			}
			// The final table-drop step must independently retain the same facts.
			down.Files = realMigrationFiles(t, deliveryPolicyMigrationVersions[:1], "down")
			artifactPGCode(t, runMigrations(ctx, fx.Pool, down), "P0001")
			if after := deliveryPolicyMigrationState(t, ctx, fx, schema); after != before {
				t.Fatal("table-drop guard changed retained rows, indexes or ledger")
			}
			deliveryPolicyMigrationReady(t, ctx, q, true)
		})
	}
}

func TestDeliveryPolicyMigrationsFenceConcurrentHistory(t *testing.T) {
	for _, table := range []string{"project_delivery_policy", "delivery_evidence_evaluation"} {
		t.Run(table, func(t *testing.T) {
			ctx, fx, schema, opts := deliveryPolicyMigrationFixture(t)
			if err := runMigrations(ctx, fx.Pool, opts); err != nil {
				t.Fatal(err)
			}
			tx, err := fx.Pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(context.Background())
			q := db.New(tx)
			if table == "project_delivery_policy" {
				_, err = q.CreateDeliveryPolicy(ctx, deliveryPolicyMigrationParams())
			} else {
				_, err = q.CreateDeliveryEvaluation(ctx, deliveryEvaluationMigrationParams())
			}
			if err != nil {
				t.Fatal(err)
			}
			for _, stem := range []string{deliveryPolicyMigrationVersions[4], deliveryPolicyMigrationVersions[0]} {
				body, err := os.ReadFile(realMigrationFiles(t, []string{stem}, "down")[0])
				if err != nil {
					t.Fatal(err)
				}
				_, err = fx.Pool.Exec(ctx, string(body))
				artifactPGCode(t, err, "55P03")
			}
			if err := tx.Commit(ctx); err != nil {
				t.Fatal(err)
			}
			if fx.Count(t, "SELECT count(*) FROM "+table) != 1 {
				t.Fatal("rollback contention lost the writer's committed history")
			}
			before := deliveryPolicyMigrationState(t, ctx, fx, schema)
			down := deliveryPolicyMigrationDown(t, opts)
			down.Files = realMigrationFiles(t, []string{deliveryPolicyMigrationVersions[4]}, "down")
			artifactPGCode(t, runMigrations(ctx, fx.Pool, down), "P0001")
			if after := deliveryPolicyMigrationState(t, ctx, fx, schema); after != before {
				t.Fatal("committed history was lost after initial lock refusal")
			}
		})
	}
}

func TestDeliveryPolicyMigrationsRejectIncorrectIndexShapes(t *testing.T) {
	indexes := []struct {
		stem, name, table, columns, wrongColumns, include, predicate string
	}{
		{deliveryPolicyMigrationVersions[1], "project_delivery_policy_revision_idx", "project_delivery_policy", "workspace_id,project_id,revision", "project_id,workspace_id,revision", "mode", "mode='observe'"},
		{deliveryPolicyMigrationVersions[2], "delivery_evaluation_key_idx", "delivery_evidence_evaluation", "workspace_id,evaluator_user_id,request_key", "evaluator_user_id,workspace_id,request_key", "issue_id", "policy_revision=1"},
		{deliveryPolicyMigrationVersions[3], "delivery_evaluation_id_idx", "delivery_evidence_evaluation", "evaluation_id", "request_key", "issue_id", "policy_revision=1"},
	}
	for _, index := range indexes {
		t.Run(index.name, func(t *testing.T) {
			ctx, fx, _, opts := deliveryPolicyMigrationFixture(t)
			if err := runMigrations(ctx, fx.Pool, opts); err != nil {
				t.Fatal(err)
			}
			q := db.New(fx.Pool)
			readySQL, err := os.ReadFile(realMigrationFiles(t, []string{deliveryPolicyMigrationVersions[4]}, "up")[0])
			if err != nil {
				t.Fatal(err)
			}
			for _, bad := range []struct{ name, sql string }{
				{"missing", ""},
				{"nonunique", "CREATE INDEX CONCURRENTLY " + index.name + " ON " + index.table + " (" + index.columns + ")"},
				{"wrong_columns", "CREATE UNIQUE INDEX CONCURRENTLY " + index.name + " ON " + index.table + " (" + index.wrongColumns + ")"},
				{"partial", "CREATE UNIQUE INDEX CONCURRENTLY " + index.name + " ON " + index.table + " (" + index.columns + ") WHERE " + index.predicate},
				{"included_column", "CREATE UNIQUE INDEX CONCURRENTLY " + index.name + " ON " + index.table + " (" + index.columns + ") INCLUDE (" + index.include + ")"},
				{"descending", "CREATE UNIQUE INDEX CONCURRENTLY " + index.name + " ON " + index.table + " (" + index.columns + " DESC)"},
			} {
				t.Run(bad.name, func(t *testing.T) {
					fx.Exec(t, "DROP INDEX CONCURRENTLY "+index.name)
					if bad.sql != "" {
						fx.Exec(t, bad.sql)
					}
					deliveryPolicyMigrationReady(t, ctx, q, false)
					_, err := fx.Pool.Exec(ctx, string(readySQL))
					artifactPGCode(t, err, "P0001")
					fx.Exec(t, "DROP INDEX CONCURRENTLY IF EXISTS "+index.name)
					repair, err := os.ReadFile(realMigrationFiles(t, []string{index.stem}, "up")[0])
					if err != nil {
						t.Fatal(err)
					}
					fx.Exec(t, string(repair))
					deliveryPolicyMigrationReady(t, ctx, q, true)
				})
			}
		})
	}
}

func TestDeliveryPolicyMigrationsRecoverRealInvalidIndexes(t *testing.T) {
	for _, index := range []struct{ stem, name string }{
		{deliveryPolicyMigrationVersions[1], "project_delivery_policy_revision_idx"},
		{deliveryPolicyMigrationVersions[2], "delivery_evaluation_key_idx"},
		{deliveryPolicyMigrationVersions[3], "delivery_evaluation_id_idx"},
	} {
		t.Run(index.name, func(t *testing.T) {
			if got := concurrentIndexCleanups[index.stem]; got != index.name || hooksForDirection("up")[index.stem] == nil {
				t.Fatalf("missing cleanup registration: %s -> %q", index.stem, got)
			}
			ctx, fx, schema, opts := deliveryPolicyMigrationFixture(t)
			first := opts
			first.Files = first.Files[:1]
			if err := runMigrations(ctx, fx.Pool, first); err != nil {
				t.Fatal(err)
			}
			q := db.New(fx.Pool)
			var reconcile func()
			if index.stem == deliveryPolicyMigrationVersions[1] {
				p := deliveryPolicyMigrationParams()
				if _, err := q.CreateDeliveryPolicy(ctx, p); err != nil {
					t.Fatal(err)
				}
				p.Reason = "Deliberate private duplicate"
				if _, err := q.CreateDeliveryPolicy(ctx, p); err != nil {
					t.Fatal(err)
				}
				reconcile = func() { fx.Exec(t, `DELETE FROM project_delivery_policy WHERE reason=$1`, p.Reason) }
			} else {
				p := deliveryEvaluationMigrationParams()
				if _, err := q.CreateDeliveryEvaluation(ctx, p); err != nil {
					t.Fatal(err)
				}
				p.RequestDigest = strings.Repeat("b", 64)
				if index.stem == deliveryPolicyMigrationVersions[2] {
					p.EvaluationID = artifactUUID()
				} else {
					p.RequestKey = artifactUUID()
				}
				if _, err := q.CreateDeliveryEvaluation(ctx, p); err != nil {
					t.Fatal(err)
				}
				reconcile = func() {
					fx.Exec(t, `DELETE FROM delivery_evidence_evaluation WHERE request_digest=$1`, p.RequestDigest)
				}
			}
			build := opts
			build.Files = realMigrationFiles(t, []string{index.stem}, "up")
			artifactPGCode(t, runMigrations(ctx, fx.Pool, build), "23505")
			assertIndexValidity(t, fx.Pool, schema, index.name, false)
			if fx.Count(t, `SELECT count(*) FROM schema_migrations`) != 1 {
				t.Fatal("failed unique index build was recorded as migrated")
			}
			deliveryPolicyMigrationReady(t, ctx, q, false)
			// Only the deliberately duplicated private fixture is reconciled.
			// Retention refusal tests never delete history to make rollback pass.
			reconcile()
			if err := runMigrations(ctx, fx.Pool, build); err != nil {
				t.Fatalf("registered cleanup did not repair the real failed index: %v", err)
			}
			assertIndexValidity(t, fx.Pool, schema, index.name, true)
			if err := runMigrations(ctx, fx.Pool, opts); err != nil {
				t.Fatal(err)
			}
			deliveryPolicyMigrationReady(t, ctx, q, true)
			if fx.Count(t, `SELECT (SELECT count(*) FROM project_delivery_policy)+(SELECT count(*) FROM delivery_evidence_evaluation)`) != 1 || fx.Count(t, `SELECT count(*) FROM schema_migrations`) != len(deliveryPolicyMigrationVersions) {
				t.Fatal("index recovery lost surviving history or migration coverage")
			}
		})
	}
}

func TestDeliveryPolicyMigrationsUniqueIdentities(t *testing.T) {
	ctx, fx, schema, opts := deliveryPolicyMigrationFixture(t)
	if err := runMigrations(ctx, fx.Pool, opts); err != nil {
		t.Fatal(err)
	}
	q := db.New(fx.Pool)
	p := deliveryPolicyMigrationParams()
	if _, err := q.CreateDeliveryPolicy(ctx, p); err != nil {
		t.Fatal(err)
	}
	if _, err := q.CreateDeliveryPolicy(ctx, p); err == nil {
		t.Fatal("duplicate policy revision was accepted")
	} else {
		artifactPGCode(t, err, "23505")
	}
	p.Revision++
	if _, err := q.CreateDeliveryPolicy(ctx, p); err != nil {
		t.Fatalf("new policy revision was refused: %v", err)
	}
	e := deliveryEvaluationMigrationParams()
	if _, err := q.CreateDeliveryEvaluation(ctx, e); err != nil {
		t.Fatal(err)
	}
	before := deliveryPolicyMigrationState(t, ctx, fx, schema)
	changedIssue := e
	changedIssue.EvaluationID, changedIssue.IssueID = artifactUUID(), artifactUUID()
	if _, err := q.CreateDeliveryEvaluation(ctx, changedIssue); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("same actor/key should return no inserted row, got %v", err)
	}
	if after := deliveryPolicyMigrationState(t, ctx, fx, schema); after != before {
		t.Fatal("same actor/key conflict altered retained rows or schema")
	}
	changedKey := e
	changedKey.RequestKey = artifactUUID()
	if _, err := q.CreateDeliveryEvaluation(ctx, changedKey); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("duplicate evaluation identity should return no inserted row, got %v", err)
	}
	if after := deliveryPolicyMigrationState(t, ctx, fx, schema); after != before {
		t.Fatal("duplicate evaluation identity altered retained rows or schema")
	}
	otherActor := e
	otherActor.EvaluationID, otherActor.EvaluatorUserID = artifactUUID(), artifactUUID()
	if _, err := q.CreateDeliveryEvaluation(ctx, otherActor); err != nil {
		t.Fatalf("another evaluator's separate key namespace was refused: %v", err)
	}
	otherWorkspace := e
	otherWorkspace.EvaluationID, otherWorkspace.WorkspaceID = artifactUUID(), artifactUUID()
	if _, err := q.CreateDeliveryEvaluation(ctx, otherWorkspace); err != nil {
		t.Fatalf("another workspace's separate key namespace was refused: %v", err)
	}
	if fx.Count(t, `SELECT count(*) FROM project_delivery_policy`) != 2 || fx.Count(t, `SELECT count(*) FROM delivery_evidence_evaluation`) != 3 {
		t.Fatal("identity refusal inserted or removed an unexpected history row")
	}
}
