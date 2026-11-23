package migrations

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/multica-ai/multica/server/internal/testutil"
)

var collaborationStageMigrations = []string{
	"805_collaboration_stage", "806_project_stage_policy_revision", "807_project_stage_policy_request", "808_collaboration_stage_task_issue",
	"809_collaboration_stage_role_revision", "810_collaboration_stage_candidate_id", "811_collaboration_stage_candidate_version",
	"812_collaboration_stage_decision_id", "813_collaboration_stage_decision_issue", "814_collaboration_stage_decision_candidate",
	"815_collaboration_stage_decision_accept", "816_collaboration_stage_child_issue", "817_collaboration_stage_child_root",
	"818_collaboration_stage_child_materialization", "819_collaboration_stage_ready", "820_collaboration_stage_child_code",
}

var collaborationStageTables = []string{"project_stage_policy", "collaboration_stage_task", "collaboration_stage_role",
	"collaboration_stage_candidate", "collaboration_stage_decision", "collaboration_stage_child"}

func collaborationStageMigration(t *testing.T, stem, direction string) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("..", "..", "migrations", stem+"."+direction+".sql"))
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

// The stage tables are standalone, so the actual migration files run in a
// private schema of the task database without touching application tables.
func newCollaborationStageMigrationFixture(t *testing.T) (*testutil.Fixture, func(string) error) {
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
	schema := "collaboration_stage_" + strings.ReplaceAll(uuid.NewString(), "-", "")
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
	config.MaxConns, config.MinConns = 3, 0
	config.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	// Migration files are executed like the runner does: outside an explicit
	// transaction, one file per simple-protocol command string.
	run := func(sql string) error {
		_, err := pool.Exec(ctx, sql, pgx.QueryExecModeSimpleProtocol)
		return err
	}
	return testutil.New(pool, "", ""), run
}

func stagePgError(err error) (string, string) {
	var pgerr *pgconn.PgError
	if errors.As(err, &pgerr) {
		return pgerr.Code, pgerr.Message
	}
	return "", ""
}

func TestCollaborationStageMigrationsShapeGuardsAndRollback(t *testing.T) {
	ctx := context.Background()
	fx, run := newCollaborationStageMigrationFixture(t)
	tables := collaborationStageMigration(t, collaborationStageMigrations[0], "up")
	statements := tables[strings.Index(tables, "CREATE TABLE"):]
	for _, forbidden := range []string{"FOREIGN KEY", "REFERENCES ", "PRIMARY KEY", " UNIQUE", "CASCADE", "CREATE INDEX"} {
		if strings.Contains(statements, forbidden) {
			t.Fatalf("table migration contains %q", forbidden)
		}
	}
	for _, stem := range collaborationStageMigrations[1:14] {
		up := strings.TrimSpace(collaborationStageMigration(t, stem, "up"))
		if strings.Count(up, ";") != 1 || !strings.HasSuffix(up, ";") || (!strings.HasPrefix(up, "CREATE UNIQUE INDEX CONCURRENTLY ") && !strings.HasPrefix(up, "CREATE INDEX CONCURRENTLY ")) {
			t.Fatalf("%s is not one concurrent index statement: %s", stem, up)
		}
		if down := strings.TrimSpace(collaborationStageMigration(t, stem, "down")); !strings.HasPrefix(down, "DROP INDEX CONCURRENTLY IF EXISTS ") || strings.Count(down, ";") != 1 {
			t.Fatalf("%s down is not one concurrent index drop: %s", stem, down)
		}
	}
	if err := run(tables); err != nil {
		t.Fatal(err)
	}
	for _, table := range collaborationStageTables {
		if fx.Count(t, `SELECT count(*) FROM pg_index WHERE indrelid=to_regclass($1)`, table) != 0 ||
			fx.Count(t, `SELECT count(*) FROM pg_constraint WHERE conrelid=to_regclass($1) AND contype IN ('f','u','p')`, table) != 0 {
			t.Fatalf("%s introduced an implicit index, key or foreign key", table)
		}
	}
	ready := collaborationStageMigration(t, "819_collaboration_stage_ready", "up")
	if code, message := stagePgError(run(ready)); code != "P0001" || !strings.Contains(message, "collaboration stage schema unavailable") {
		t.Fatalf("readiness passed before the indexes existed: %s %s", code, message)
	}
	for _, stem := range collaborationStageMigrations[1:14] {
		if err := run(collaborationStageMigration(t, stem, "up")); err != nil {
			t.Fatalf("%s: %v", stem, err)
		}
	}
	if err := run(ready); err != nil {
		t.Fatalf("readiness refused the complete schema: %v", err)
	}
	if fx.Count(t, `SELECT count(*) FROM pg_index i JOIN pg_class c ON c.oid=i.indexrelid WHERE c.relnamespace=current_schema()::regnamespace AND i.indisvalid AND i.indisready AND i.indislive`) != 13 ||
		fx.Count(t, `SELECT count(*) FROM pg_index i JOIN pg_class c ON c.oid=i.indexrelid WHERE c.relnamespace=current_schema()::regnamespace AND i.indisunique`) != 11 {
		t.Fatal("unexpected stage index set")
	}
	// An invalid index must fail readiness even though its name exists.
	fx.Exec(t, `UPDATE pg_index SET indisvalid=false WHERE indexrelid=to_regclass('collaboration_stage_child_issue_idx')`)
	if code, _ := stagePgError(run(ready)); code != "P0001" {
		t.Fatal("readiness accepted an invalid index")
	}
	fx.Exec(t, `UPDATE pg_index SET indisvalid=true WHERE indexrelid=to_regclass('collaboration_stage_child_issue_idx')`)

	ws, project, issue, user, other := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	digest := strings.Repeat("a", 64)
	policy := `INSERT INTO project_stage_policy (workspace_id,project_id,revision,contract,enabled,reason,created_by,request_id,request_digest) VALUES ($1,$2,$3,$4,true,$5,$6,$7,$8)`
	fx.Exec(t, policy, ws, project, 1, "collaboration-stage-v1", "enable", user, "key-1", digest)
	role := `INSERT INTO collaboration_stage_role (issue_id,workspace_id,revision,business_owner_user_id,implementer_user_id,product_approver_user_id,code_reviewer_user_id,qa_approver_user_id,release_approver_user_id,policy_revision,lifecycle_epoch,configured_by,operation_id)
 VALUES ($1,$2,$3,$4,$4,$4,$5,$4,$6,1,1,$4,$7)`
	fx.Exec(t, role, issue, ws, 1, user, other, uuid.NewString(), uuid.NewString())
	candidate, proposal := uuid.NewString(), uuid.NewString()
	candidateSQL := `INSERT INTO collaboration_stage_candidate (candidate_id,issue_id,workspace_id,kind,proposal_id,version,parent_candidate_id,content,content_digest,source,lifecycle_epoch,submitted_by,operation_id)
 VALUES ($1,$2,$3,'proposal',$4,$5,$6,'{"summary":"","children":[]}',$7,'{"kind":"manual"}',1,$8,$9)`
	fx.Exec(t, candidateSQL, candidate, issue, ws, proposal, 1, nil, digest, user, uuid.NewString())
	decisionSQL := `INSERT INTO collaboration_stage_decision (decision_id,issue_id,workspace_id,step,decision,proposal_id,candidate_id,candidate_version,content_digest,role,role_revision,policy_revision,lifecycle_epoch,actor_user_id,participant_epoch,account_epoch,credential_kind,reason,payload,operation_id)
 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,'product_approver',1,1,1,$10,1,1,'jwt','','{}',$11)`
	accepted := uuid.NewString()
	fx.Exec(t, decisionSQL, accepted, issue, ws, "proposal", "accepted", proposal, candidate, 1, digest, user, uuid.NewString())
	childSQL := `INSERT INTO collaboration_stage_child (child_issue_id,root_issue_id,workspace_id,origin,proposal_id,candidate_id,ordinal,decision_id,lifecycle_epoch) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,1)`
	child := uuid.NewString()
	fx.Exec(t, childSQL, child, issue, ws, "materialized", proposal, candidate, 1, accepted)
	fx.Exec(t, `INSERT INTO collaboration_stage_task (issue_id,workspace_id) VALUES ($1,$2)`, issue, ws)

	second := uuid.NewString()
	for name, tc := range map[string]struct {
		sql  string
		args []any
		code string
	}{
		"policy_revision_reused": {policy, []any{ws, project, 1, "collaboration-stage-v1", "again", user, "key-2", digest}, "23505"},
		"policy_key_reused":      {policy, []any{ws, project, 2, "collaboration-stage-v1", "again", user, "key-1", digest}, "23505"},
		"policy_contract":        {policy, []any{ws, project, 2, "another-contract", "again", user, "key-3", digest}, "23514"},
		"policy_blank_reason":    {policy, []any{ws, project, 2, "collaboration-stage-v1", "  ", user, "key-4", digest}, "23514"},
		"role_self_review":       {role, []any{issue, ws, 2, user, user, uuid.NewString(), uuid.NewString()}, "23514"},
		"role_qa_release":        {role, []any{issue, ws, 2, user, other, user, uuid.NewString()}, "23514"},
		"role_revision_reused":   {role, []any{issue, ws, 1, user, other, uuid.NewString(), uuid.NewString()}, "23505"},
		"candidate_version":      {candidateSQL, []any{uuid.NewString(), issue, ws, proposal, 1, nil, digest, user, uuid.NewString()}, "23505"},
		"candidate_parent":       {candidateSQL, []any{uuid.NewString(), issue, ws, proposal, 2, nil, digest, user, uuid.NewString()}, "23514"},
		"candidate_digest":       {candidateSQL, []any{uuid.NewString(), issue, ws, uuid.NewString(), 1, nil, "not-a-digest", user, uuid.NewString()}, "23514"},
		"second_decision":        {decisionSQL, []any{uuid.NewString(), issue, ws, "proposal", "rejected", proposal, candidate, 1, digest, user, uuid.NewString()}, "23505"},
		"scope_with_candidate":   {decisionSQL, []any{uuid.NewString(), issue, ws, "scope", "confirmed", nil, candidate, nil, nil, user, uuid.NewString()}, "23514"},
		"proposal_without_ref":   {decisionSQL, []any{uuid.NewString(), issue, ws, "proposal", "accepted", proposal, nil, nil, nil, user, uuid.NewString()}, "23514"},
		"child_mapped_twice":     {childSQL, []any{child, issue, ws, "confirmed", nil, nil, nil, accepted}, "23505"},
		"ordinal_reused":         {childSQL, []any{second, issue, ws, "materialized", proposal, candidate, 1, accepted}, "23505"},
		"ordinal_out_of_range":   {childSQL, []any{second, issue, ws, "materialized", proposal, candidate, 21, accepted}, "23514"},
		"confirmed_with_ordinal": {childSQL, []any{second, issue, ws, "confirmed", nil, nil, 1, accepted}, "23514"},
		"child_is_root":          {childSQL, []any{issue, issue, ws, "confirmed", nil, nil, nil, accepted}, "23514"},
		"task_reused":            {`INSERT INTO collaboration_stage_task (issue_id,workspace_id) VALUES ($1,$2)`, []any{issue, ws}, "23505"},
	} {
		if _, err := fx.Pool.Exec(ctx, tc.sql, tc.args...); func() bool { code, _ := stagePgError(err); return code != tc.code }() {
			t.Fatalf("%s: want %s, got %v", name, tc.code, err)
		}
	}
	// A second accepted decision for the same proposal is refused even on
	// another candidate version.
	next := uuid.NewString()
	fx.Exec(t, candidateSQL, next, issue, ws, proposal, 2, candidate, digest, user, uuid.NewString())
	if _, err := fx.Pool.Exec(ctx, decisionSQL, uuid.NewString(), issue, ws, "proposal", "accepted", proposal, next, 2, digest, user, uuid.NewString()); func() bool { code, _ := stagePgError(err); return code != "23505" }() {
		t.Fatalf("a proposal was accepted twice: %v", err)
	}
	for _, statement := range []string{
		`UPDATE project_stage_policy SET enabled=false`, `UPDATE collaboration_stage_role SET revision=revision`,
		`UPDATE collaboration_stage_candidate SET content='{}'`, `UPDATE collaboration_stage_decision SET reason='rewritten'`,
	} {
		if _, err := fx.Pool.Exec(ctx, statement); func() bool { code, _ := stagePgError(err); return code != "23514" }() {
			t.Fatalf("immutable fact was rewritten by %q: %v", statement, err)
		}
	}
	fx.Exec(t, `UPDATE collaboration_stage_child SET semantic_version=semantic_version+1 WHERE child_issue_id=$1`, child)
	// 820 adds the confirmed code version to the retained mapping row in place.
	code := collaborationStageMigration(t, "820_collaboration_stage_child_code", "up")
	for _, forbidden := range []string{"FOREIGN KEY", "REFERENCES ", "CREATE INDEX", " UNIQUE", "CASCADE"} {
		if strings.Contains(code[strings.Index(code, "ALTER TABLE"):], forbidden) {
			t.Fatalf("code version migration contains %q", forbidden)
		}
	}
	if err := run(code); err != nil {
		t.Fatal(err)
	}
	if fx.Count(t, `SELECT count(*) FROM collaboration_stage_child WHERE child_issue_id=$1 AND semantic_version=2 AND code_provider IS NULL AND code_head_sha IS NULL`, child) != 1 {
		t.Fatal("the code version migration changed a retained mapping")
	}
	head := strings.Repeat("b", 40)
	for name, tc := range map[string][]any{
		"partial":       {"github", nil, nil},
		"provider":      {"local", uuid.NewString(), head},
		"short_head":    {"github", uuid.NewString(), head[:12]},
		"upper_head":    {"vcs", uuid.NewString(), strings.ToUpper(head)},
		"head_only":     {nil, nil, head},
		"missing_head":  {"github", uuid.NewString(), nil},
		"symbolic_head": {"github", uuid.NewString(), "refs/heads/main"},
	} {
		if _, err := fx.Pool.Exec(ctx, `UPDATE collaboration_stage_child SET code_provider=$2, code_pull_request_id=$3, code_head_sha=$4 WHERE child_issue_id=$1`, append([]any{child}, tc...)...); func() bool { code, _ := stagePgError(err); return code != "23514" }() {
			t.Fatalf("invalid code version %s accepted: %v", name, err)
		}
	}
	fx.Exec(t, `UPDATE collaboration_stage_child SET code_provider='vcs', code_pull_request_id=$2, code_head_sha=$3 WHERE child_issue_id=$1`, child, uuid.NewString(), strings.Repeat("c", 64))

	// Rollback starts at 820 and refuses before any column or index is dropped.
	snapshot := func() string {
		var out string
		fx.QueryRow(t, `SELECT (SELECT count(*) FROM project_stage_policy)::text||'/'||(SELECT count(*) FROM collaboration_stage_role)::text||'/'||
 (SELECT count(*) FROM collaboration_stage_candidate)::text||'/'||(SELECT count(*) FROM collaboration_stage_decision)::text||'/'||
 (SELECT count(*) FROM collaboration_stage_child)::text||'/'||(SELECT count(*) FROM collaboration_stage_task)::text||'/'||
 (SELECT count(*) FROM pg_index i JOIN pg_class c ON c.oid=i.indexrelid WHERE c.relnamespace=current_schema()::regnamespace AND i.indisvalid)::text||'/'||
 (SELECT count(*) FROM information_schema.columns WHERE table_schema=current_schema() AND table_name='collaboration_stage_child' AND column_name LIKE 'code_%')::text||'/'||
 (SELECT count(*) FROM collaboration_stage_child WHERE code_head_sha IS NOT NULL)::text`).Scan(&out)
		return out
	}
	before := snapshot()
	for _, stem := range []string{"820_collaboration_stage_child_code", "819_collaboration_stage_ready", "805_collaboration_stage"} {
		if code, message := stagePgError(run(collaborationStageMigration(t, stem, "down"))); code != "P0001" || !strings.Contains(message, "cannot roll back retained collaboration stage records") {
			t.Fatalf("%s down did not refuse retained history: %s %s", stem, code, message)
		}
	}
	if snapshot() != before {
		t.Fatal("a refused rollback changed retained stage history or indexes")
	}
	// An uncommitted writer is refused by NOWAIT before the emptiness check.
	writer, err := fx.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = writer.Exec(ctx, policy, ws, project, 2, "collaboration-stage-v1", "in flight", user, "key-5", digest); err != nil {
		t.Fatal(err)
	}
	for _, stem := range []string{"820_collaboration_stage_child_code", "819_collaboration_stage_ready"} {
		if busy, _ := stagePgError(run(collaborationStageMigration(t, stem, "down"))); busy != "55P03" {
			t.Fatalf("%s rollback waited for or ignored a busy writer: %s", stem, busy)
		}
	}
	_ = writer.Rollback(ctx)
	if snapshot() != before {
		t.Fatal("a rollback refused by a busy writer changed retained stage history")
	}
	// Explicit maintenance: only an empty schema rolls back, then upgrades again.
	for _, table := range collaborationStageTables {
		fx.Exec(t, "DELETE FROM "+table)
	}
	reverse := slices.Clone(collaborationStageMigrations)
	slices.Reverse(reverse)
	for _, stem := range reverse {
		if err = run(collaborationStageMigration(t, stem, "down")); err != nil {
			t.Fatalf("%s down: %v", stem, err)
		}
	}
	if fx.Count(t, `SELECT count(*) FROM pg_class WHERE relnamespace=current_schema()::regnamespace`) != 0 ||
		fx.Count(t, `SELECT count(*) FROM pg_proc WHERE pronamespace=current_schema()::regnamespace`) != 0 {
		t.Fatal("empty rollback left stage objects behind")
	}
	// Replaying the empty rollback is harmless.
	for _, stem := range reverse {
		if err = run(collaborationStageMigration(t, stem, "down")); err != nil {
			t.Fatalf("%s down replay: %v", stem, err)
		}
	}
	for _, stem := range collaborationStageMigrations {
		if err = run(collaborationStageMigration(t, stem, "up")); err != nil {
			t.Fatalf("%s re-upgrade: %v", stem, err)
		}
	}
	if fx.Count(t, `SELECT count(*) FROM pg_index i JOIN pg_class c ON c.oid=i.indexrelid WHERE c.relnamespace=current_schema()::regnamespace AND i.indisvalid`) != 13 {
		t.Fatal("re-upgrade did not rebuild the stage indexes")
	}
}
