package migrations

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/multica-ai/multica/server/internal/testutil"
)

func knowledgeMigrationFiles(t *testing.T) []string {
	t.Helper()
	files, err := filepath.Glob("../../migrations/*_collaboration_knowledge*.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) < 12 {
		t.Fatalf("knowledge schema/index migrations missing: %d", len(files))
	}
	return files
}

func knowledgeSchema(t *testing.T) (*pgxpool.Pool, *testutil.Fixture) {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("requires task-owned DATABASE_URL")
	}
	ctx := context.Background()
	base, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	schema := "knowledge_schema_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err = base.Exec(ctx, "CREATE SCHEMA "+pgx.Identifier{schema}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = base.Exec(ctx, "DROP SCHEMA "+pgx.Identifier{schema}.Sanitize()+" CASCADE")
		_ = base.Close(ctx)
	})
	config, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	config.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	fx := testutil.New(pool, "", "")
	fx.Exec(t, `CREATE TABLE agent_task_queue (
		id uuid NOT NULL DEFAULT gen_random_uuid(), session_id text,
		agent_id uuid, runtime_id uuid, created_at timestamptz NOT NULL DEFAULT now())`)
	fx.Insert(t, "agent_task_queue", testutil.Cols{"session_id": "legacy-session"})
	fx.Exec(t, `CREATE TABLE workspace (id uuid NOT NULL, issue_counter bigint DEFAULT 0)`)
	fx.Exec(t, `CREATE TABLE project (id uuid NOT NULL, workspace_id uuid NOT NULL, lead_id uuid, lead_type text, description text, revision bigint DEFAULT 1)`)
	for _, path := range knowledgeMigrationFiles(t) {
		knowledgeApply(t, pool, path)
	}
	return pool, fx
}

func knowledgeApply(t *testing.T, pool *pgxpool.Pool, path string) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(context.Background(), string(raw)); err != nil {
		t.Fatalf("%s: %v", filepath.Base(path), err)
	}
}

func knowledgeError(t *testing.T, pool *pgxpool.Pool, code, sql string, args ...any) {
	t.Helper()
	_, err := pool.Exec(context.Background(), sql, args...)
	var pgerr *pgconn.PgError
	if !errors.As(err, &pgerr) || pgerr.Code != code {
		t.Fatalf("want SQLSTATE %s, got %v", code, err)
	}
}

func knowledgeCandidate(t *testing.T, fx *testutil.Fixture) string {
	t.Helper()
	id := uuid.NewString()
	return fx.Insert(t, "collaboration_knowledge_candidate", testutil.Cols{
		"id": id, "lineage_id": id, "version": 1, "source_workspace_id": uuid.NewString(), "source_issue_id": uuid.NewString(),
		"target_kind": "workspace", "target_workspace_id": uuid.NewString(), "content": "private immutable body", "summary": "private summary",
		"content_sha256": fmt.Sprintf("%x", sha256.Sum256([]byte("private immutable body"))), "sources": `[{"kind":"memory"}]`, "evidence": `[]`, "applicability": `{"surfaces":["issue"]}`, "proposed_by_user_id": uuid.NewString(),
	})
}

func TestKnowledgeSchemaFreshReapplyAndEmptyDown(t *testing.T) {
	pool, fx := knowledgeSchema(t)
	if fx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE session_id='legacy-session' AND knowledge_context_version IS NULL AND knowledge_delivery IS NULL`) != 1 {
		t.Fatal("upgrade changed legacy session or invented managed exposure")
	}
	if fx.Count(t, `SELECT count(*) FROM pg_constraint WHERE connamespace=current_schema()::regnamespace AND contype IN ('f','p','u')`) != 0 {
		t.Fatal("foreign key or inline unique/primary key created")
	}
	if fx.Count(t, `SELECT count(*) FROM pg_index JOIN pg_class ON pg_class.oid=indexrelid WHERE relnamespace=current_schema()::regnamespace AND NOT indisvalid`) != 0 {
		t.Fatal("invalid concurrent index")
	}
	for _, name := range []string{"collaboration_knowledge_agent_history_idx", "collaboration_knowledge_runtime_history_idx", "collaboration_knowledge_delivery_history_idx"} {
		if fx.Count(t, `SELECT count(*) FROM pg_indexes WHERE schemaname=current_schema() AND indexname=$1`, name) != 1 {
			t.Fatalf("knowledge history index %s was not created", name)
		}
	}
	files := knowledgeMigrationFiles(t)
	for _, path := range files {
		knowledgeApply(t, pool, path)
	}
	for i := len(files) - 1; i >= 0; i-- {
		knowledgeApply(t, pool, strings.TrimSuffix(files[i], ".up.sql")+".down.sql")
	}
	if fx.Count(t, `SELECT count(*) FROM pg_class WHERE relnamespace=current_schema()::regnamespace AND relname LIKE 'collaboration_knowledge%'`) != 0 {
		t.Fatal("empty down left knowledge objects")
	}
	for _, path := range files {
		knowledgeApply(t, pool, path)
	}
}

func TestKnowledgeSchemaImmutableVersionsAndSingleApproval(t *testing.T) {
	pool, fx := knowledgeSchema(t)
	id := knowledgeCandidate(t, fx)
	for _, field := range []string{"content='changed'", "summary='changed'", "source_issue_id=gen_random_uuid()", "target_workspace_id=gen_random_uuid()", "lineage_id=gen_random_uuid()", "version=2", "sources='[]'::jsonb", "expires_at=now()+interval '1 day'"} {
		knowledgeError(t, pool, "23514", `UPDATE collaboration_knowledge_candidate SET `+field+` WHERE id=$1`, id)
	}
	knowledgeError(t, pool, "23514", `DELETE FROM collaboration_knowledge_candidate WHERE id=$1`, id)
	fx.Exec(t, `UPDATE collaboration_knowledge_candidate SET state='approved',state_revision=2,approved_by_user_id=proposed_by_user_id,approved_at=now(),published_target_revision=1,last_decided_by_user_id=proposed_by_user_id,last_decided_at=now() WHERE id=$1`, id)
	knowledgeError(t, pool, "23514", `UPDATE collaboration_knowledge_candidate SET state='withdrawn',last_decision_reason='withdraw' WHERE id=$1`, id)
	knowledgeError(t, pool, "23505", `INSERT INTO collaboration_knowledge_candidate SELECT id,lineage_id,version,source_workspace_id,source_issue_id,target_kind,target_workspace_id,target_project_id,content,summary,content_sha256,sources,evidence,applicability,proposed_by_user_id,proposed_at,expires_at,supersedes_candidate_id,derived_from_candidate_id,state,state_revision,approved_by_user_id,approved_at,last_decided_by_user_id,last_decided_at,last_decision_reason,published_target_revision FROM collaboration_knowledge_candidate WHERE id=$1`, id)
	copySQL := `INSERT INTO collaboration_knowledge_candidate SELECT gen_random_uuid(),lineage_id,2,source_workspace_id,source_issue_id,target_kind,target_workspace_id,target_project_id,content,summary,content_sha256,sources,evidence,applicability,proposed_by_user_id,proposed_at,expires_at,id,NULL,'approved',1,approved_by_user_id,approved_at,last_decided_by_user_id,last_decided_at,last_decision_reason,published_target_revision FROM collaboration_knowledge_candidate WHERE id=$1`
	knowledgeError(t, pool, "23505", copySQL, id)
	fx.Exec(t, `UPDATE collaboration_knowledge_candidate SET state='superseded',state_revision=3,last_decided_by_user_id=proposed_by_user_id,last_decided_at=now() WHERE id=$1`, id)
	fx.Exec(t, copySQL, id)
	knowledgeError(t, pool, "23514", `UPDATE collaboration_knowledge_candidate SET approved_by_user_id=gen_random_uuid() WHERE id=$1`, id)
	knowledgeError(t, pool, "23514", `UPDATE collaboration_knowledge_candidate SET state='approved',state_revision=4 WHERE id=$1`, id)
	for _, path := range knowledgeMigrationFiles(t) {
		knowledgeApply(t, pool, path)
		down := strings.TrimSuffix(path, ".up.sql") + ".down.sql"
		raw, err := os.ReadFile(down)
		if err != nil {
			t.Fatal(err)
		}
		knowledgeError(t, pool, "55000", string(raw))
	}
}

func TestKnowledgeSchemaDownProtectsTaskAndPendingInvalidation(t *testing.T) {
	for _, kind := range []string{"negotiation", "delivery", "scan", "hint", "uncommitted"} {
		t.Run(kind, func(t *testing.T) {
			pool, fx := knowledgeSchema(t)
			switch kind {
			case "negotiation":
				fx.Insert(t, "agent_task_queue", testutil.Cols{"knowledge_context_version": 1})
			case "delivery":
				fx.Insert(t, "agent_task_queue", testutil.Cols{"knowledge_context_version": 1, "knowledge_delivery": `{"phase":"claimed"}`})
			case "scan":
				fx.Insert(t, "collaboration_knowledge_invalidation", testutil.Cols{"kind": "scan", "cause_id": uuid.NewString(), "scope_kind": "all_sources"})
			case "hint":
				targetID := uuid.NewString()
				fx.Insert(t, "collaboration_knowledge_invalidation", testutil.Cols{"kind": "target_hint", "cause_id": uuid.NewString(), "target_kind": "workspace", "target_id": targetID, "target_workspace_id": targetID, "knowledge_revision": 1})
			case "uncommitted":
				tx, err := pool.Begin(context.Background())
				if err != nil {
					t.Fatal(err)
				}
				defer tx.Rollback(context.Background())
				if _, err = tx.Exec(context.Background(), `INSERT INTO agent_task_queue (knowledge_context_version) VALUES (1)`); err != nil {
					t.Fatal(err)
				}
			}
			for _, path := range knowledgeMigrationFiles(t) {
				raw, err := os.ReadFile(strings.TrimSuffix(path, ".up.sql") + ".down.sql")
				if err != nil {
					t.Fatal(err)
				}
				code := "55000"
				if kind == "uncommitted" {
					code = "55P03"
				}
				knowledgeError(t, pool, code, string(raw))
			}
		})
	}
}

func TestKnowledgeSchemaTaskMetadataBoundary(t *testing.T) {
	pool, fx := knowledgeSchema(t)
	for _, sql := range []string{
		`INSERT INTO agent_task_queue (knowledge_context_version) VALUES (2)`,
		`INSERT INTO agent_task_queue (knowledge_delivery) VALUES ('{}')`,
		`INSERT INTO agent_task_queue (knowledge_context_version,knowledge_delivery) VALUES (1,'[]')`,
		`INSERT INTO agent_task_queue (knowledge_context_version,knowledge_delivery) VALUES (1,jsonb_build_object('phase',repeat('x',65536)))`,
	} {
		knowledgeError(t, pool, "23514", sql)
	}
	fx.Insert(t, "agent_task_queue", testutil.Cols{"knowledge_context_version": 1})
	if fx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE knowledge_context_version=1 AND knowledge_delivery IS NULL`) != 1 {
		t.Fatal("negotiated unknown history was erased")
	}
}
