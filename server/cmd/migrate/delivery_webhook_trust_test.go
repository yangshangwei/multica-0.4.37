package main

import (
	"context"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/multica-ai/multica/server/internal/testutil"
)

var deliveryWebhookTrustVersions = []string{"792_delivery_github_trust", "793_delivery_vcs_trust"}

func deliveryWebhookTrustFixture(t *testing.T) (context.Context, *testutil.Fixture, string, runOptions) {
	t.Helper()
	ctx, fx, schema, opts := deliveryCompletionMigrationFixture(t)
	if err := runMigrations(ctx, fx.Pool, opts); err != nil {
		t.Fatal(err)
	}
	// Only the pre-existing integration columns relevant to these ALTERs are
	// required here. These private tables are schema fixtures, not integrations.
	fx.Exec(t, `CREATE TABLE github_installation (id uuid NOT NULL, fixture_label text NOT NULL)`)
	fx.Exec(t, `CREATE TABLE vcs_connection (id uuid NOT NULL, fixture_label text NOT NULL)`)
	for _, table := range []string{"github_installation", "vcs_connection"} {
		fx.Insert(t, table, testutil.Cols{"id": artifactUUID(), "fixture_label": "existing row"})
	}
	opts.Files = realMigrationFiles(t, deliveryWebhookTrustVersions, "up")
	return ctx, fx, schema, opts
}

func deliveryWebhookTrustDown(t *testing.T, opts runOptions) runOptions {
	t.Helper()
	versions := slices.Clone(deliveryWebhookTrustVersions)
	slices.Reverse(versions)
	opts.Direction = "down"
	opts.Hooks = hooksForDirection("down")
	opts.Files = realMigrationFiles(t, versions, "down")
	return opts
}

func deliveryWebhookTrustState(t *testing.T, ctx context.Context, fx *testutil.Fixture, schema string) string {
	t.Helper()
	var state string
	fx.QueryRow(t, `SELECT jsonb_build_object(
		'catalog',$1::text,
		'ledger',(SELECT jsonb_agg(to_jsonb(m) ORDER BY version) FROM schema_migrations m),
		'github',(SELECT jsonb_agg(to_jsonb(i) ORDER BY id) FROM github_installation i),
		'vcs',(SELECT jsonb_agg(to_jsonb(c) ORDER BY id) FROM vcs_connection c),
		'authorizations',(SELECT jsonb_agg(to_jsonb(a) ORDER BY id) FROM delivery_completion_authorization a))::text`,
		iterationRecoveryCatalog(t, ctx, fx.Pool, schema)).Scan(&state)
	return state
}

func TestDeliveryWebhookTrustMigrationsUpgradeEmptyRollbackAndReupgrade(t *testing.T) {
	ctx, fx, _, opts := deliveryWebhookTrustFixture(t)
	if err := runMigrations(ctx, fx.Pool, opts); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"github_installation", "vcs_connection"} {
		fx.Insert(t, table, testutil.Cols{"id": artifactUUID(), "fixture_label": "new row"})
		t.Run(table, func(t *testing.T) {
			name := pgx.Identifier{table}.Sanitize()
			if fx.Count(t, `SELECT count(*) FROM `+name+` WHERE fixture_label='existing row' AND trust_revision=1`) != 1 {
				t.Fatal("upgrade lost the existing row or did not supply initial trust")
			}
			if fx.Count(t, `SELECT count(*) FROM `+name+` WHERE fixture_label='new row' AND trust_revision=1`) != 1 {
				t.Fatal("post-upgrade insert did not use the positive trust default")
			}
			for _, value := range []string{"0", "-1", "NULL"} {
				_, err := fx.Pool.Exec(ctx, `UPDATE `+name+` SET trust_revision=`+value)
				code := "23514"
				if value == "NULL" {
					code = "23502"
				}
				artifactPGCode(t, err, code)
			}
			fx.Exec(t, `UPDATE `+name+` SET trust_revision=2`)
		})
	}
	if err := runMigrations(ctx, fx.Pool, deliveryWebhookTrustDown(t, opts)); err != nil {
		t.Fatal(err)
	}
	if fx.Count(t, `SELECT count(*) FROM pg_catalog.pg_attribute WHERE attrelid IN ('github_installation'::regclass,'vcs_connection'::regclass) AND attname='trust_revision' AND NOT attisdropped`) != 0 {
		t.Fatal("empty downgrade retained a trust column")
	}
	if fx.Count(t, `SELECT count(*) FROM github_installation`) != 2 || fx.Count(t, `SELECT count(*) FROM vcs_connection`) != 2 {
		t.Fatal("empty downgrade removed integration rows")
	}
	if err := runMigrations(ctx, fx.Pool, opts); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"github_installation", "vcs_connection"} {
		if fx.Count(t, `SELECT count(*) FROM `+pgx.Identifier{table}.Sanitize()+` WHERE trust_revision=1`) != 2 {
			t.Fatal("re-upgrade without retained grants did not preserve rows/default")
		}
	}
}

// This INSERT creates only a retained schema fixture. It is deliberately not a
// successful artifact/authorization API chain or proof of product completion.
func deliveryWebhookTrustInsertGrant(t *testing.T, ctx context.Context, tx pgx.Tx, kind string) {
	t.Helper()
	var installation any
	var config, provider, instance any
	if kind == "github" {
		installation, config = int64(19), strings.Repeat("e", 64)
	} else {
		provider, instance = "forgejo", "https://migration.example.invalid"
	}
	_, err := tx.Exec(ctx, `INSERT INTO delivery_completion_authorization
		(id,workspace_id,issuer_user_id,request_key,operation_id,contract_version,request_digest,request_json,created_at,expires_at,reason,
		 target_kind,issue_id,issue_revision,expected_status,target_status,project_id,project_revision,policy_revision,
		 evidence_issue_id,evidence_issue_revision,approval_id,execution_id,producer_agent_id,producer_task_id,producer_generation,
		 spec_digest,artifact_digest,receipt_digest,source_revision,evidence_expires_at,observation,
		 consumer_kind,integration_id,integration_trust_revision,github_installation_id,vcs_provider,vcs_instance_url,webhook_config_identity,
		 pull_request_id,repo_owner,repo_name,pr_number,pr_head_sha,
		 issuer_credential_kind,issuer_credential_expires_at,issuer_password_mode)
		VALUES ($1,$2,$3,$4,$5,'delivery-completion-enforcement-v1',$6,'{}',clock_timestamp(),clock_timestamp()+interval '1 minute','retention schema fixture',
		 'ordinary',$7,0,'in_review','done',$8,0,1,$7,0,$9,$10,$11,$12,1,
		 $6,$6,$6,$13,clock_timestamp()+interval '2 minutes','{}',
		 $14,$15,1,$16,$17,$18,$19,$20,'fixture-owner','fixture-repo',1,$13,
		 'jwt',clock_timestamp()+interval '1 hour',false)`,
		artifactUUID(), artifactUUID(), artifactUUID(), artifactUUID(), artifactUUID(), strings.Repeat("a", 64),
		artifactUUID(), artifactUUID(), artifactUUID(), artifactUUID(), artifactUUID(), artifactUUID(), strings.Repeat("b", 40),
		kind, artifactUUID(), installation, provider, instance, config, artifactUUID())
	if err != nil {
		t.Fatal(err)
	}
}

func TestDeliveryWebhookTrustMigrationsRetainEitherConsumerWithoutPartialRollback(t *testing.T) {
	for _, kind := range []string{"github", "vcs"} {
		t.Run(kind, func(t *testing.T) {
			ctx, fx, schema, opts := deliveryWebhookTrustFixture(t)
			if err := runMigrations(ctx, fx.Pool, opts); err != nil {
				t.Fatal(err)
			}
			tx, err := fx.Pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(ctx)
			deliveryWebhookTrustInsertGrant(t, ctx, tx, kind)
			if err = tx.Commit(ctx); err != nil {
				t.Fatal(err)
			}
			before := deliveryWebhookTrustState(t, ctx, fx, schema)
			artifactPGCode(t, runMigrations(ctx, fx.Pool, deliveryWebhookTrustDown(t, opts)), "P0001")
			for _, version := range deliveryWebhookTrustVersions {
				body, err := os.ReadFile(realMigrationFiles(t, []string{version}, "down")[0])
				if err != nil {
					t.Fatal(err)
				}
				_, err = fx.Pool.Exec(ctx, string(body))
				artifactPGCode(t, err, "P0001")
			}
			if after := deliveryWebhookTrustState(t, ctx, fx, schema); after != before {
				t.Fatal("refused downgrade changed integration, grant, catalog or migration ledger")
			}
		})
	}
}

func TestDeliveryWebhookTrustMigrationsFenceUncommittedGrant(t *testing.T) {
	ctx, fx, schema, opts := deliveryWebhookTrustFixture(t)
	if err := runMigrations(ctx, fx.Pool, opts); err != nil {
		t.Fatal(err)
	}
	tx, err := fx.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	deliveryWebhookTrustInsertGrant(t, ctx, tx, "github")
	before := iterationRecoveryCatalog(t, ctx, fx.Pool, schema)
	// NOWAIT must see the in-flight insert even though its row is not visible to
	// this connection. A short statement timeout makes accidental blocking fail.
	for _, version := range deliveryWebhookTrustVersions {
		body, err := os.ReadFile(realMigrationFiles(t, []string{version}, "down")[0])
		if err != nil {
			t.Fatal(err)
		}
		probe, err := fx.Pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = probe.Exec(ctx, "SET LOCAL statement_timeout='500ms'"); err != nil {
			t.Fatal(err)
		}
		_, err = probe.Exec(ctx, string(body))
		artifactPGCode(t, err, "55P03")
		if err = probe.Rollback(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if iterationRecoveryCatalog(t, ctx, fx.Pool, schema) != before {
		t.Fatal("concurrent grant race partially removed trust columns")
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	artifactPGCode(t, runMigrations(ctx, fx.Pool, deliveryWebhookTrustDown(t, opts)), "P0001")
	if fx.Count(t, `SELECT count(*) FROM delivery_completion_authorization`) != 1 {
		t.Fatal("concurrently committed grant was removed")
	}
}
