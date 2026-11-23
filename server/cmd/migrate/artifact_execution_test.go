package main

import (
	"context"
	"errors"
	"os"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

var artifactMigrationVersions = []string{"766_agent_artifact_execution", "767_agent_artifact_execution_id", "768_agent_artifact_execution_approval", "769_agent_artifact_execution_idempotency", "770_agent_artifact_execution_ready"}

func artifactMigrationFixture(t *testing.T) (context.Context, *testutil.Fixture, string, runOptions) {
	t.Helper()
	ctx, fx, schema, opts := switchMigrationFixture(t)
	opts.Files = realMigrationFiles(t, artifactMigrationVersions, "up")
	return ctx, fx, schema, opts
}

func artifactUUID() pgtype.UUID             { return pgtype.UUID{Bytes: uuid.New(), Valid: true} }
func artifactText(value string) pgtype.Text { return pgtype.Text{String: value, Valid: true} }
func artifactParams(t *testing.T, q *db.Queries) db.CreateAgentArtifactExecutionParams {
	t.Helper()
	now, err := q.ArtifactExecutionDatabaseTime(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	return db.CreateAgentArtifactExecutionParams{
		ExecutionID: artifactUUID(), ApprovalID: artifactUUID(), WorkspaceID: artifactUUID(), InitiatorKind: "human", InitiatorID: artifactUUID(), OriginatorUserID: artifactUUID(),
		AgentID: artifactUUID(), TaskID: artifactUUID(), RuntimeID: artifactUUID(), TaskGeneration: 1, ResourceKind: "issue", ResourceID: artifactUUID(), Action: "package_release_candidate",
		IdempotencyKey: artifactUUID(), RequestDigest: strings.Repeat("a", 64), CanonicalSpec: []byte(`{"version":1}`), SpecDigest: strings.Repeat("b", 64), SourceRevision: strings.Repeat("c", 40),
		StagingID: "fixture", StagingManifestDigest: strings.Repeat("d", 64), InputManifestDigest: strings.Repeat("e", 64), InvocationDigest: strings.Repeat("f", 64), PolicyVersion: "artifact-tar-v1", TargetEnvironment: "local-synthetic", RootConfigIdentity: "fixture-root",
		CreatedAt: now, ExpiresAt: pgtype.Timestamptz{Time: now.Time.Add(15 * time.Minute), Valid: true}, UpdatedAt: now,
	}
}
func artifactReady(t *testing.T, ctx context.Context, q *db.Queries, want bool) {
	t.Helper()
	got, err := q.ArtifactExecutionSchemaReady(ctx)
	if err != nil || got != want {
		t.Fatalf("schema ready=%v err=%v want=%v", got, err, want)
	}
}
func artifactClaim(row db.AgentArtifactExecution) db.ClaimAgentArtifactExecutionParams {
	return db.ClaimAgentArtifactExecutionParams{ExecutionID: row.ExecutionID, SpecDigest: row.SpecDigest, ClaimedKind: artifactText("human"), ClaimedBy: row.InitiatorID, ClaimNonce: artifactUUID()}
}
func artifactFinal(row db.AgentArtifactExecution, nonce pgtype.UUID, status string) db.FinalizeAgentArtifactExecutionParams {
	p := db.FinalizeAgentArtifactExecutionParams{ExecutionID: row.ExecutionID, SpecDigest: row.SpecDigest, ClaimNonce: nonce, ResultStatus: artifactText(status), ResultCode: "fixture_result", ReceiptJson: []byte(`{"version":1}`), ReceiptDigest: strings.Repeat("a", 64)}
	if status == "succeeded" {
		p.OutputArtifactID = row.ExecutionID
		p.OutputArtifactDigest = strings.Repeat("b", 64)
		p.OutputSize = pgtype.Int8{Int64: 1024, Valid: true}
	}
	return p
}
func artifactPGCode(t *testing.T, err error, code string) {
	t.Helper()
	var p *pgconn.PgError
	if !errors.As(err, &p) || p.Code != code {
		t.Fatalf("expected PostgreSQL %s, got %v", code, err)
	}
}

func TestArtifactExecutionMigrationsPreserveEvidenceAndRollbackOrder(t *testing.T) {
	ctx, fx, _, opts := artifactMigrationFixture(t)
	q := db.New(fx.Pool)
	artifactReady(t, ctx, q, false)
	first := opts
	first.Files = first.Files[:1]
	if err := runMigrations(ctx, fx.Pool, first); err != nil {
		t.Fatal(err)
	}
	if fx.Count(t, `SELECT count(*) FROM pg_index WHERE indrelid='agent_artifact_execution'::regclass`) != 0 {
		t.Fatal("table migration created implicit index")
	}
	artifactReady(t, ctx, q, false)
	if err := runMigrations(ctx, fx.Pool, opts); err != nil {
		t.Fatal(err)
	}
	artifactReady(t, ctx, q, true)
	if fx.Count(t, `SELECT count(*) FROM pg_constraint WHERE conrelid='agent_artifact_execution'::regclass AND contype IN ('f','p','u')`) != 0 {
		t.Fatal("foreign key or implicit uniqueness")
	}
	params := artifactParams(t, q)
	if _, err := q.CreateAgentArtifactExecution(ctx, params); err != nil {
		t.Fatal(err)
	}
	down := opts
	down.Direction = "down"
	down.Hooks = hooksForDirection("down")
	reverse := slices.Clone(artifactMigrationVersions)
	slices.Reverse(reverse)
	down.Files = realMigrationFiles(t, reverse, "down")
	artifactPGCode(t, runMigrations(ctx, fx.Pool, down), "P0001")
	artifactReady(t, ctx, q, true)
	if fx.Count(t, `SELECT count(*) FROM schema_migrations`) != 5 || fx.Count(t, `SELECT count(*) FROM agent_artifact_execution`) != 1 {
		t.Fatal("refused rollback changed ledger or evidence")
	}
	fx.Exec(t, `DELETE FROM agent_artifact_execution`)
	if err := runMigrations(ctx, fx.Pool, down); err != nil {
		t.Fatal(err)
	}
	artifactReady(t, ctx, q, false)
	if fx.Count(t, `SELECT count(*) FROM schema_migrations`) != 0 {
		t.Fatal("empty maintenance rollback retained ledger")
	}
}

func TestArtifactExecutionSchemaReadinessRejectsWrongIndex(t *testing.T) {
	ctx, fx, _, opts := artifactMigrationFixture(t)
	q := db.New(fx.Pool)
	if err := runMigrations(ctx, fx.Pool, opts); err != nil {
		t.Fatal(err)
	}
	for _, sql := range []string{
		`CREATE INDEX CONCURRENTLY agent_artifact_execution_idempotency_idx ON agent_artifact_execution(workspace_id,initiator_kind,initiator_id,idempotency_key)`,
		`CREATE UNIQUE INDEX CONCURRENTLY agent_artifact_execution_idempotency_idx ON agent_artifact_execution(workspace_id,initiator_id,initiator_kind,idempotency_key)`,
		`CREATE UNIQUE INDEX CONCURRENTLY agent_artifact_execution_idempotency_idx ON agent_artifact_execution(workspace_id,initiator_kind,initiator_id,idempotency_key) WHERE claimed_at IS NULL`,
		`CREATE UNIQUE INDEX CONCURRENTLY agent_artifact_execution_idempotency_idx ON agent_artifact_execution(workspace_id,initiator_kind,initiator_id,idempotency_key) INCLUDE(spec_digest)`,
	} {
		fx.Exec(t, `DROP INDEX CONCURRENTLY agent_artifact_execution_idempotency_idx`)
		artifactReady(t, ctx, q, false)
		fx.Exec(t, sql)
		artifactReady(t, ctx, q, false)
		body, err := os.ReadFile(realMigrationFiles(t, []string{artifactMigrationVersions[4]}, "up")[0])
		if err != nil {
			t.Fatal(err)
		}
		_, err = fx.Pool.Exec(ctx, string(body))
		artifactPGCode(t, err, "P0001")
	}
	fx.Exec(t, `DROP INDEX CONCURRENTLY agent_artifact_execution_idempotency_idx`)
	body, err := os.ReadFile(realMigrationFiles(t, []string{artifactMigrationVersions[3]}, "up")[0])
	if err != nil {
		t.Fatal(err)
	}
	fx.Exec(t, string(body))
	artifactReady(t, ctx, q, true)
}

func TestArtifactExecutionClaimAndReceiptCAS(t *testing.T) {
	ctx, fx, _, opts := artifactMigrationFixture(t)
	q := db.New(fx.Pool)
	if err := runMigrations(ctx, fx.Pool, opts); err != nil {
		t.Fatal(err)
	}
	for _, status := range []string{"succeeded", "failed", "unknown"} {
		t.Run(status, func(t *testing.T) {
			row, err := q.CreateAgentArtifactExecution(ctx, artifactParams(t, q))
			if err != nil {
				t.Fatal(err)
			}
			claim := artifactClaim(row)
			wrong := claim
			wrong.SpecDigest = strings.Repeat("f", 64)
			if _, err = q.ClaimAgentArtifactExecution(ctx, wrong); !errors.Is(err, pgx.ErrNoRows) {
				t.Fatalf("changed digest admitted: %v", err)
			}
			claimed, err := q.ClaimAgentArtifactExecution(ctx, claim)
			if err != nil {
				t.Fatal(err)
			}
			if claimed.ResultStatus.String != "unknown" || !claimed.ClaimedAt.Valid || claimed.FinishedAt.Valid {
				t.Fatal("claim did not durably reserve unknown")
			}
			if _, err = q.ClaimAgentArtifactExecution(ctx, claim); !errors.Is(err, pgx.ErrNoRows) {
				t.Fatalf("claim replay admitted: %v", err)
			}
			final := artifactFinal(row, artifactUUID(), status)
			if _, err = q.FinalizeAgentArtifactExecution(ctx, final); !errors.Is(err, pgx.ErrNoRows) {
				t.Fatalf("wrong nonce finalized: %v", err)
			}
			final.ClaimNonce = claim.ClaimNonce
			done, err := q.FinalizeAgentArtifactExecution(ctx, final)
			if err != nil {
				t.Fatal(err)
			}
			if done.ResultStatus.String != status || !done.FinishedAt.Valid {
				t.Fatal("wrong terminal receipt")
			}
			if _, err = q.FinalizeAgentArtifactExecution(ctx, final); !errors.Is(err, pgx.ErrNoRows) {
				t.Fatalf("terminal receipt overwritten: %v", err)
			}
			if _, err = q.ClaimAgentArtifactExecution(ctx, claim); !errors.Is(err, pgx.ErrNoRows) {
				t.Fatalf("terminal execution admitted: %v", err)
			}
		})
	}
	expired := artifactParams(t, q)
	expired.CreatedAt.Time = expired.CreatedAt.Time.Add(-time.Hour)
	expired.ExpiresAt.Time = expired.ExpiresAt.Time.Add(-time.Hour)
	row, err := q.CreateAgentArtifactExecution(ctx, expired)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = q.ClaimAgentArtifactExecution(ctx, artifactClaim(row)); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("expired claim admitted: %v", err)
	}
}

func TestArtifactExecutionConcurrentClaimAndIdempotency(t *testing.T) {
	ctx, fx, _, opts := artifactMigrationFixture(t)
	q := db.New(fx.Pool)
	if err := runMigrations(ctx, fx.Pool, opts); err != nil {
		t.Fatal(err)
	}
	p := artifactParams(t, q)
	var created atomic.Int32
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			copy := p
			copy.ExecutionID = artifactUUID()
			copy.ApprovalID = artifactUUID()
			if _, err := q.CreateAgentArtifactExecution(ctx, copy); err == nil {
				created.Add(1)
			} else if !errors.Is(err, pgx.ErrNoRows) {
				errs <- err
			}
		}()
	}
	wg.Wait()
	if created.Load() != 1 {
		t.Fatalf("idempotent creation winners=%d", created.Load())
	}
	row, err := q.GetAgentArtifactExecutionByIdempotency(ctx, db.GetAgentArtifactExecutionByIdempotencyParams{WorkspaceID: p.WorkspaceID, InitiatorKind: p.InitiatorKind, InitiatorID: p.InitiatorID, IdempotencyKey: p.IdempotencyKey})
	if err != nil {
		t.Fatal(err)
	}
	var admitted atomic.Int32
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := q.ClaimAgentArtifactExecution(ctx, artifactClaim(row)); err == nil {
				admitted.Add(1)
			} else if !errors.Is(err, pgx.ErrNoRows) {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	if admitted.Load() != 1 {
		t.Fatalf("claim winners=%d", admitted.Load())
	}
	retained, err := q.GetAgentArtifactExecutionByApproval(ctx, row.ApprovalID)
	if err != nil {
		t.Fatal(err)
	}
	if retained.ResultStatus.String != "unknown" {
		t.Fatal("winner did not retain unknown")
	}
	// This proves admission cardinality; API tests separately count real adapter calls.
}

func TestArtifactExecutionRollbackBusyAndTerminalEvidence(t *testing.T) {
	ctx, fx, _, opts := artifactMigrationFixture(t)
	q := db.New(fx.Pool)
	if err := runMigrations(ctx, fx.Pool, opts); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(realMigrationFiles(t, []string{artifactMigrationVersions[4]}, "down")[0])
	if err != nil {
		t.Fatal(err)
	}
	tx, err := fx.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	params := artifactParams(t, q)
	row, err := db.New(tx).CreateAgentArtifactExecution(ctx, params)
	if err != nil {
		t.Fatal(err)
	}
	_, err = fx.Pool.Exec(ctx, string(body))
	artifactPGCode(t, err, "55P03")
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	_, err = fx.Pool.Exec(ctx, string(body))
	artifactPGCode(t, err, "P0001")
	claim := artifactClaim(row)
	if _, err = q.ClaimAgentArtifactExecution(ctx, claim); err != nil {
		t.Fatal(err)
	}
	for _, stem := range []string{artifactMigrationVersions[4], artifactMigrationVersions[0]} {
		b, err := os.ReadFile(realMigrationFiles(t, []string{stem}, "down")[0])
		if err != nil {
			t.Fatal(err)
		}
		_, err = fx.Pool.Exec(ctx, string(b))
		artifactPGCode(t, err, "P0001")
	}
	artifactReady(t, ctx, q, true)
}

func TestArtifactExecutionConstraintsRejectIncompleteAndInvalidStates(t *testing.T) {
	ctx, fx, _, opts := artifactMigrationFixture(t)
	q := db.New(fx.Pool)
	if err := runMigrations(ctx, fx.Pool, opts); err != nil {
		t.Fatal(err)
	}
	row, err := q.CreateAgentArtifactExecution(ctx, artifactParams(t, q))
	if err != nil {
		t.Fatal(err)
	}
	invalid := []string{
		"initiator_kind='runtime'", "task_generation=-1", "resource_kind='workspace'", "action='shell'", "canonical_spec=''::bytea", "request_digest='bad'", "spec_digest='bad'", "source_revision='main'", "staging_id=''", "staging_manifest_digest='bad'", "input_manifest_digest='bad'", "invocation_digest='bad'", "target_environment='production'", "policy_version='unknown'", "root_config_identity=''", "expires_at=created_at", "result_status='unexpected'", "claimed_at=now()", "output_artifact_id=execution_id", "output_size=1", "output_artifact_digest=repeat('a',64)",
		"claimed_kind='human',claimed_by=initiator_id,claim_nonce=gen_random_uuid(),claimed_at=now(),result_status='unknown'",
		"receipt_json='[]'::jsonb", "receipt_json=jsonb_build_object('x',repeat('a',4096))",
	}
	for _, set := range invalid {
		_, err = fx.Pool.Exec(ctx, "UPDATE agent_artifact_execution SET "+set+" WHERE execution_id=$1", row.ExecutionID)
		artifactPGCode(t, err, "23514")
	}
	claim := artifactClaim(row)
	if _, err = q.ClaimAgentArtifactExecution(ctx, claim); err != nil {
		t.Fatal(err)
	}
	for _, set := range []string{"result_status='succeeded'", "result_status='failed'", "finished_at=now()", "output_artifact_id=execution_id"} {
		_, err = fx.Pool.Exec(ctx, "UPDATE agent_artifact_execution SET "+set+" WHERE execution_id=$1", row.ExecutionID)
		artifactPGCode(t, err, "23514")
	}
	final := artifactFinal(row, claim.ClaimNonce, "succeeded")
	if _, err = q.FinalizeAgentArtifactExecution(ctx, final); err != nil {
		t.Fatal(err)
	}
}

func TestArtifactExecutionConcurrentIndexRetryRecoversInvalidRelation(t *testing.T) {
	ctx, fx, _, opts := artifactMigrationFixture(t)
	first := opts
	first.Files = first.Files[:1]
	if err := runMigrations(ctx, fx.Pool, first); err != nil {
		t.Fatal(err)
	}
	q := db.New(fx.Pool)
	p := artifactParams(t, q)
	cols := testutil.Cols{
		"execution_id": p.ExecutionID, "approval_id": p.ApprovalID, "workspace_id": p.WorkspaceID, "initiator_kind": p.InitiatorKind, "initiator_id": p.InitiatorID, "originator_user_id": p.OriginatorUserID,
		"agent_id": p.AgentID, "task_id": p.TaskID, "runtime_id": p.RuntimeID, "task_generation": p.TaskGeneration, "resource_kind": p.ResourceKind, "resource_id": p.ResourceID, "action": p.Action,
		"idempotency_key": p.IdempotencyKey, "request_digest": p.RequestDigest, "canonical_spec": p.CanonicalSpec, "spec_digest": p.SpecDigest, "source_revision": p.SourceRevision, "staging_id": p.StagingID,
		"staging_manifest_digest": p.StagingManifestDigest, "input_manifest_digest": p.InputManifestDigest, "invocation_digest": p.InvocationDigest, "policy_version": p.PolicyVersion, "target_environment": p.TargetEnvironment,
		"root_config_identity": p.RootConfigIdentity, "created_at": p.CreatedAt, "expires_at": p.ExpiresAt, "updated_at": p.UpdatedAt,
	}
	fx.InsertNoID(t, "agent_artifact_execution", cols, "execution_id=$1", p.ExecutionID)
	duplicateApproval := artifactUUID()
	cols["approval_id"] = duplicateApproval
	cols["idempotency_key"] = artifactUUID()
	fx.InsertNoID(t, "agent_artifact_execution", cols, "approval_id=$1", duplicateApproval)
	index := opts
	index.Files = realMigrationFiles(t, []string{artifactMigrationVersions[1]}, "up")
	artifactPGCode(t, runMigrations(ctx, fx.Pool, index), "23505")
	if fx.Count(t, `SELECT count(*) FROM pg_index WHERE indexrelid=to_regclass('agent_artifact_execution_id_idx') AND NOT indisvalid`) != 1 {
		t.Fatal("expected real failed concurrent index")
	}
	fx.Exec(t, `DELETE FROM agent_artifact_execution WHERE approval_id=$1`, duplicateApproval)
	if err := runMigrations(ctx, fx.Pool, index); err != nil {
		t.Fatal(err)
	}
	if err := runMigrations(ctx, fx.Pool, opts); err != nil {
		t.Fatal(err)
	}
	artifactReady(t, ctx, q, true)
	if fx.Count(t, `SELECT count(*) FROM agent_artifact_execution`) != 1 {
		t.Fatal("retry cleanup changed surviving evidence")
	}
}
