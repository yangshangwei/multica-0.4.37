package main

import (
	"context"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

var deliveryCompletionMigrationVersions = []string{"781_delivery_completion_authorization", "782_delivery_completion_id", "783_delivery_completion_request", "784_delivery_completion_operation", "785_delivery_completion_terminal_id", "786_delivery_completion_candidate", "787_delivery_completion_policy_mode", "788_delivery_completion_ready"}

func deliveryCompletionMigrationFixture(t *testing.T) (context.Context, *testutil.Fixture, string, runOptions) {
	t.Helper()
	ctx, fx, schema, opts := switchMigrationFixture(t)
	prerequisites := opts
	prerequisites.Files = realMigrationFiles(t, deliveryPolicyMigrationVersions, "up")
	if err := runMigrations(ctx, fx.Pool, prerequisites); err != nil {
		t.Fatal(err)
	}
	opts.Files = realMigrationFiles(t, deliveryCompletionMigrationVersions, "up")
	return ctx, fx, schema, opts
}
func completionMigrationReady(t *testing.T, ctx context.Context, q *db.Queries, want bool) {
	t.Helper()
	ready, err := q.DeliveryCompletionSchemaReady(ctx)
	if err != nil || !ready.Valid || ready.Bool != want {
		t.Fatalf("completion readiness=%+v error=%v want=%v", ready, err, want)
	}
}
func completionMigrationDown(t *testing.T, opts runOptions) runOptions {
	t.Helper()
	v := slices.Clone(deliveryCompletionMigrationVersions)
	slices.Reverse(v)
	opts.Direction = "down"
	opts.Hooks = hooksForDirection("down")
	opts.Files = realMigrationFiles(t, v, "down")
	return opts
}
func TestDeliveryCompletionMigrationsUpgradeAndEmptyRollback(t *testing.T) {
	ctx, fx, _, opts := deliveryCompletionMigrationFixture(t)
	q := db.New(fx.Pool)
	completionMigrationReady(t, ctx, q, false)
	first := opts
	first.Files = first.Files[:1]
	if err := runMigrations(ctx, fx.Pool, first); err != nil {
		t.Fatal(err)
	}
	if fx.Count(t, `SELECT count(*) FROM pg_index WHERE indrelid IN ('delivery_completion_authorization'::regclass,'delivery_completion_terminal'::regclass)`) != 0 {
		t.Fatal("implicit index in table migration")
	}
	if fx.Count(t, `SELECT count(*) FROM pg_constraint WHERE conrelid IN ('delivery_completion_authorization'::regclass,'delivery_completion_terminal'::regclass) AND contype IN ('f','u','p')`) != 0 {
		t.Fatal("foreign key or implicit uniqueness")
	}
	completionMigrationReady(t, ctx, q, false)
	if err := runMigrations(ctx, fx.Pool, opts); err != nil {
		t.Fatal(err)
	}
	completionMigrationReady(t, ctx, q, true)
	if fx.Count(t, `SELECT count(*) FROM pg_index WHERE indrelid IN ('delivery_completion_authorization'::regclass,'delivery_completion_terminal'::regclass) AND indisvalid AND indisready AND indislive`) != 5 {
		t.Fatal("missing exact usable indexes")
	}
	if err := runMigrations(ctx, fx.Pool, completionMigrationDown(t, opts)); err != nil {
		t.Fatal(err)
	}
	completionMigrationReady(t, ctx, q, false)
	if fx.Count(t, `SELECT count(*) FROM pg_class WHERE oid IN(to_regclass('delivery_completion_authorization'),to_regclass('delivery_completion_terminal'))`) != 0 {
		t.Fatal("empty rollback retained tables")
	}
	deliveryPolicyMigrationReady(t, ctx, q, true)
}
func TestDeliveryCompletionMigrationsRetainTerminalAndEnforce(t *testing.T) {
	for _, kind := range []string{"terminal", "enforce"} {
		t.Run(kind, func(t *testing.T) {
			ctx, fx, schema, opts := deliveryCompletionMigrationFixture(t)
			if err := runMigrations(ctx, fx.Pool, opts); err != nil {
				t.Fatal(err)
			}
			if kind == "terminal" {
				actor := artifactUUID()
				fx.InsertNoID(t, "delivery_completion_terminal", testutil.Cols{"authorization_id": artifactUUID(), "kind": "revoked", "actor_kind": "human", "actor_user_id": actor, "revoker_user_id": actor, "revocation_request_key": artifactUUID(), "revocation_reason": "schema retention fixture", "revocation_digest": strings.Repeat("a", 64)}, "true")
			} else {
				p := deliveryPolicyMigrationParams()
				p.Mode = "enforce"
				if _, err := db.New(fx.Pool).CreateDeliveryPolicy(ctx, p); err != nil {
					t.Fatal(err)
				}
			}
			before := iterationRecoveryCatalog(t, ctx, fx.Pool, schema)
			artifactPGCode(t, runMigrations(ctx, fx.Pool, completionMigrationDown(t, opts)), "P0001")
			if after := iterationRecoveryCatalog(t, ctx, fx.Pool, schema); after != before {
				t.Fatal("retained rollback changed schema")
			}
			completionMigrationReady(t, ctx, db.New(fx.Pool), true)
		})
	}
}
func TestDeliveryCompletionMigrationsFenceConcurrentTerminal(t *testing.T) {
	ctx, fx, _, opts := deliveryCompletionMigrationFixture(t)
	if err := runMigrations(ctx, fx.Pool, opts); err != nil {
		t.Fatal(err)
	}
	tx, err := fx.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "LOCK TABLE delivery_completion_terminal IN ROW EXCLUSIVE MODE"); err != nil {
		t.Fatal(err)
	}
	for _, stem := range []string{deliveryCompletionMigrationVersions[0], deliveryCompletionMigrationVersions[7]} {
		body, err := os.ReadFile(realMigrationFiles(t, []string{stem}, "down")[0])
		if err != nil {
			t.Fatal(err)
		}
		_, err = fx.Pool.Exec(ctx, string(body))
		artifactPGCode(t, err, "55P03")
	}
	if err = tx.Rollback(ctx); err != nil && !strings.Contains(err.Error(), pgx.ErrTxClosed.Error()) {
		t.Fatal(err)
	}
}
func TestDeliveryCompletionMigrationsRejectMalformedIndex(t *testing.T) {
	ctx, fx, _, opts := deliveryCompletionMigrationFixture(t)
	if err := runMigrations(ctx, fx.Pool, opts); err != nil {
		t.Fatal(err)
	}
	fx.Exec(t, "DROP INDEX delivery_completion_operation_idx")
	fx.Exec(t, "CREATE INDEX delivery_completion_operation_idx ON delivery_completion_authorization(operation_id,workspace_id)")
	completionMigrationReady(t, ctx, db.New(fx.Pool), false)
}

func TestDeliveryCompletionMigrationsRetainAuthorizationAndRejectInvalidShape(t *testing.T) {
	ctx, fx, schema, opts := deliveryCompletionMigrationFixture(t)
	if err := runMigrations(ctx, fx.Pool, opts); err != nil {
		t.Fatal(err)
	}
	// This row is explicitly a schema/retention fixture, not product evidence.
	issue, project, actor := artifactUUID(), artifactUUID(), artifactUUID()
	id := fx.Insert(t, "delivery_completion_authorization", testutil.Cols{
		"id":           artifactUUID(),
		"workspace_id": artifactUUID(), "issuer_user_id": actor, "request_key": artifactUUID(), "operation_id": artifactUUID(), "contract_version": "delivery-completion-enforcement-v1", "request_digest": strings.Repeat("a", 64), "request_json": `{}`, "created_at": time.Now().UTC(), "expires_at": time.Now().UTC().Add(time.Minute), "reason": "schema retention fixture",
		"target_kind": "ordinary", "issue_id": issue, "issue_revision": 0, "expected_status": "in_review", "target_status": "done", "project_id": project, "project_revision": 0, "policy_revision": 1,
		"evidence_issue_id": issue, "evidence_issue_revision": 0, "approval_id": artifactUUID(), "execution_id": artifactUUID(), "producer_agent_id": artifactUUID(), "producer_task_id": artifactUUID(), "producer_generation": 1, "spec_digest": strings.Repeat("a", 64), "artifact_digest": strings.Repeat("b", 64), "receipt_digest": strings.Repeat("c", 64), "source_revision": strings.Repeat("d", 40), "evidence_expires_at": time.Now().UTC().Add(2 * time.Minute), "observation": `{}`,
		"consumer_kind": "human", "consumer_user_id": actor, "issuer_credential_kind": "jwt", "issuer_credential_expires_at": time.Now().UTC().Add(time.Hour), "issuer_password_mode": false,
	})
	before := iterationRecoveryCatalog(t, ctx, fx.Pool, schema)
	artifactPGCode(t, runMigrations(ctx, fx.Pool, completionMigrationDown(t, opts)), "P0001")
	if after := iterationRecoveryCatalog(t, ctx, fx.Pool, schema); after != before {
		t.Fatal("authorization retention changed schema")
	}
	for _, sql := range []string{`UPDATE delivery_completion_authorization SET expires_at=evidence_expires_at+interval '1 second' WHERE id=$1`, `UPDATE delivery_completion_authorization SET consumer_user_id=NULL WHERE id=$1`, `UPDATE delivery_completion_authorization SET target_kind='collaboration_self' WHERE id=$1`, `UPDATE delivery_completion_authorization SET issuer_password_mode=true WHERE id=$1`} {
		_, err := fx.Pool.Exec(ctx, sql, id)
		artifactPGCode(t, err, "23514")
	}
	// A NULL in a required terminal variant must not pass PostgreSQL's three-valued CHECK.
	_, err := fx.Pool.Exec(ctx, `INSERT INTO delivery_completion_terminal(authorization_id,kind,actor_kind,actor_user_id,revocation_request_key,revocation_reason,revocation_digest) VALUES($1,'revoked','human',$2,$3,'test',$4)`, artifactUUID(), actor, artifactUUID(), strings.Repeat("a", 64))
	artifactPGCode(t, err, "23514")
}

func TestDeliveryCompletionMigrationsRejectWeakenedSchema(t *testing.T) {
	ctx, fx, _, opts := deliveryCompletionMigrationFixture(t)
	if err := runMigrations(ctx, fx.Pool, opts); err != nil {
		t.Fatal(err)
	}
	completionMigrationReady(t, ctx, db.New(fx.Pool), true)
	var canonical string
	fx.QueryRow(t, `SELECT jsonb_build_object(
		'columns', (SELECT jsonb_agg(jsonb_build_array(c.relname,a.attname,format_type(a.atttypid,a.atttypmod),a.attnotnull) ORDER BY c.relname,a.attnum) FROM pg_catalog.pg_attribute a JOIN pg_catalog.pg_class c ON c.oid=a.attrelid WHERE a.attrelid IN ('delivery_completion_authorization'::regclass,'delivery_completion_terminal'::regclass) AND a.attnum>0 AND NOT a.attisdropped),
		'checks', (SELECT jsonb_agg(jsonb_build_array(c.relname,k.conname,pg_get_constraintdef(k.oid)) ORDER BY c.relname,k.conname) FROM pg_catalog.pg_constraint k JOIN pg_catalog.pg_class c ON c.oid=k.conrelid WHERE k.conrelid IN ('delivery_completion_authorization'::regclass,'delivery_completion_terminal'::regclass) AND k.contype='c'))::text`).Scan(&canonical)
	// Preserve the actual PostgreSQL catalog shape before any negative mutation.
	t.Logf("DELIVERY_COMPLETION_SCHEMA_CONTRACT=%s", canonical)
	body, err := os.ReadFile(realMigrationFiles(t, []string{"788_delivery_completion_ready"}, "up")[0])
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		sql  string
	}{
		{"window_same_name_true", `ALTER TABLE delivery_completion_authorization DROP CONSTRAINT delivery_completion_authorization_window; ALTER TABLE delivery_completion_authorization ADD CONSTRAINT delivery_completion_authorization_window CHECK (true)`},
		{"terminal_same_name_true", `ALTER TABLE delivery_completion_terminal DROP CONSTRAINT delivery_completion_terminal_shape; ALTER TABLE delivery_completion_terminal ADD CONSTRAINT delivery_completion_terminal_shape CHECK (true)`},
		{"target_missing", `ALTER TABLE delivery_completion_authorization DROP CONSTRAINT delivery_completion_authorization_target`},
		{"consumer_missing", `ALTER TABLE delivery_completion_authorization DROP CONSTRAINT delivery_completion_authorization_consumer`},
		{"issuer_missing", `ALTER TABLE delivery_completion_authorization DROP CONSTRAINT delivery_completion_authorization_issuer`},
		{"required_column_nullable", `ALTER TABLE delivery_completion_authorization ALTER COLUMN issuer_password_mode DROP NOT NULL`},
		{"column_wrong_type", `ALTER TABLE delivery_completion_authorization ALTER COLUMN issuer_password_kind TYPE varchar(128)`},
		{"terminal_wrong_clock_default", `ALTER TABLE delivery_completion_terminal ALTER COLUMN recorded_at SET DEFAULT now()`},
		{"window_not_validated", `ALTER TABLE delivery_completion_authorization DROP CONSTRAINT delivery_completion_authorization_window; ALTER TABLE delivery_completion_authorization ADD CONSTRAINT delivery_completion_authorization_window CHECK (created_at<expires_at AND expires_at<=created_at+interval '24 hours' AND expires_at<=evidence_expires_at AND (issuer_credential_expires_at IS NULL OR expires_at<=issuer_credential_expires_at)) NOT VALID`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx, err := fx.Pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(ctx)
			if _, err = tx.Exec(ctx, tc.sql); err != nil {
				t.Fatal(err)
			}
			completionMigrationReady(t, ctx, db.New(tx), false)
			_, err = tx.Exec(ctx, string(body))
			artifactPGCode(t, err, "P0001")
		})
	}
	completionMigrationReady(t, ctx, db.New(fx.Pool), true)
}
