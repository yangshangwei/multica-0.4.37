package migrations

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// Migration fixtures intentionally use the pre-P1 schema instead of dbfx's
// current-schema inserts. No migration or destructive cleanup touches public.
const projectP1BeforeSchema = `
CREATE TABLE workspace(id uuid, name text, settings jsonb, updated_at timestamptz DEFAULT now());
CREATE TABLE project(id uuid, workspace_id uuid, title text, description text, icon text, status text,
 lead_type text, lead_id uuid, priority text, start_date date, due_date date,
 execution_squad jsonb, created_at timestamptz, updated_at timestamptz);
CREATE TABLE issue(id uuid, workspace_id uuid, project_id uuid, status text, admission_status text);
CREATE TABLE project_resource(id uuid, workspace_id uuid, project_id uuid, resource_type text,
 resource_ref jsonb, label text, position int, created_by uuid, created_at timestamptz);`

var projectP1Tables = []string{"project_state_change", "project_update", "project_update_revision", "project_update_request", "project_update_notification"}

type projectP1Fixture struct {
	ctx                 context.Context
	conn, writer        *pgx.Conn
	schema, databaseURL string
}

func newProjectP1Fixture(t *testing.T) *projectP1Fixture {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("requires local PostgreSQL at DATABASE_URL")
	}
	config, err := pgx.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	if config.Host != "localhost" && config.Host != "127.0.0.1" && config.Host != "::1" && !strings.HasPrefix(config.Host, "/") {
		t.Fatal("P1 destructive migration fixtures require a local PostgreSQL connection")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)
	conn, err := pgx.ConnectConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	schema := fmt.Sprintf("projects_p1_verify_%d", time.Now().UnixNano())
	quoted := pgx.Identifier{schema}.Sanitize()
	if _, err = conn.Exec(ctx, "CREATE SCHEMA "+quoted); err != nil {
		conn.Close(ctx)
		t.Fatal(err)
	}
	f := &projectP1Fixture{ctx: ctx, conn: conn, schema: schema}
	t.Cleanup(func() {
		cleanup, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		if f.writer != nil {
			f.writer.Close(cleanup)
		}
		if t.Failed() {
			t.Logf("preserved failed migration evidence in schema %s", schema)
		} else if _, err := conn.Exec(cleanup, "DROP SCHEMA "+quoted+" CASCADE"); err != nil {
			t.Errorf("cleanup own migration schema: %v", err)
		}
		conn.Close(cleanup)
	})
	if _, err = conn.Exec(ctx, "SET search_path TO "+quoted); err != nil {
		t.Fatal(err)
	}
	config.RuntimeParams["search_path"] = schema
	f.writer, err = pgx.ConnectConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(dsn)
	if err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") {
		t.Fatal("P1 runner fixture requires a postgres URL")
	}
	query := u.Query()
	query.Set("search_path", schema)
	u.RawQuery = query.Encode()
	f.databaseURL = u.String()
	projectP1Exec(t, f, projectP1BeforeSchema)
	return f
}

func projectP1Exec(t *testing.T, f *projectP1Fixture, sql string, args ...any) {
	t.Helper()
	if _, err := f.conn.Exec(f.ctx, sql, args...); err != nil {
		t.Fatal(err)
	}
}

func projectP1Files(t *testing.T, direction string) []string {
	t.Helper()
	var result []string
	for _, path := range migrationFilesForLint(t, "*."+direction+".sql") {
		number, err := strconv.Atoi(strings.SplitN(filepath.Base(path), "_", 2)[0])
		if err != nil {
			t.Fatal(err)
		}
		if number >= 536 && number <= 549 {
			result = append(result, filepath.Base(path))
		}
	}
	if len(result) != 14 {
		t.Fatalf("P1 migration coverage changed: got %d, want 14", len(result))
	}
	return result
}

func projectP1Up(t *testing.T, f *projectP1Fixture) {
	t.Helper()
	for _, file := range projectP1Files(t, "up") {
		applyMigrationFile(t, f.ctx, f.conn, file)
	}
}

func projectP1WantState(t *testing.T, err error, code string) {
	t.Helper()
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != code {
		t.Fatalf("want SQLSTATE %s, got %v", code, err)
	}
}

func projectP1Snapshot(t *testing.T, f *projectP1Fixture, table string, exclude ...string) string {
	t.Helper()
	var result string
	if exclude == nil {
		exclude = []string{}
	}
	if err := f.conn.QueryRow(f.ctx, "SELECT coalesce(jsonb_agg(row ORDER BY row::text), '[]')::text FROM (SELECT to_jsonb(t)-$1::text[] AS row FROM "+pgx.Identifier{table}.Sanitize()+" t) s", exclude).Scan(&result); err != nil {
		t.Fatal(err)
	}
	return result
}

func TestProjectP1UpgradePreservesLegacyRowsAndFormalCounts(t *testing.T) {
	f := newProjectP1Fixture(t)
	projectP1Exec(t, f, `INSERT INTO workspace(id,name,settings) VALUES('10000000-0000-0000-0000-000000000001','Legacy','{"arbitrary":"unchanged"}');
INSERT INTO project(id,workspace_id,title,description,icon,status,lead_type,lead_id,priority,start_date,due_date,execution_squad,created_at,updated_at)
SELECT gen_random_uuid(),'10000000-0000-0000-0000-000000000001','旧项目 ' || status,CASE status WHEN 'planned' THEN NULL WHEN 'paused' THEN '' ELSE E'  # 原始描述\r\n\n**空格**\t🙂  ' END,'🧪',status,'member',gen_random_uuid(),'high','2020-01-02','2030-12-31','{"template_id":"retained"}','2020-01-01','2021-01-01'
FROM unnest(ARRAY['planned','in_progress','paused','completed','cancelled']) status;
INSERT INTO issue SELECT gen_random_uuid(),workspace_id,id,issue_status,admission FROM project CROSS JOIN unnest(ARRAY['done','cancelled','in_progress']) issue_status CROSS JOIN unnest(ARRAY['not_required','accepted','pending','rejected']) admission;
INSERT INTO project_resource SELECT gen_random_uuid(),workspace_id,id,'github_repo','{"full_name":"legacy/repo"}','保留资源',4,lead_id,'2020-01-01' FROM project;`)
	before := map[string]string{}
	for _, table := range []string{"project", "workspace", "issue", "project_resource"} {
		before[table] = projectP1Snapshot(t, f, table)
	}
	var started time.Time
	if err := f.conn.QueryRow(f.ctx, "SELECT clock_timestamp()").Scan(&started); err != nil {
		t.Fatal(err)
	}
	projectP1Up(t, f)
	for _, table := range []string{"project", "workspace", "issue", "project_resource"} {
		exclude := []string{}
		if table == "project" {
			exclude = []string{"revision", "description_revision", "in_progress_since", "in_progress_since_source"}
		}
		if table == "workspace" {
			exclude = []string{"planning_timezone"}
		}
		if got := projectP1Snapshot(t, f, table, exclude...); got != before[table] {
			t.Errorf("legacy %s changed: before=%s after=%s", table, before[table], got)
		}
	}
	var baseline bool
	if err := f.conn.QueryRow(f.ctx, `SELECT bool_and(revision=1 AND description_revision=1 AND CASE WHEN status='in_progress' THEN in_progress_since >= $1 AND in_progress_since <= clock_timestamp() AND in_progress_since_source='migration' ELSE in_progress_since IS NULL AND in_progress_since_source IS NULL END) FROM project`, started).Scan(&baseline); err != nil || !baseline {
		t.Fatalf("invalid migration clock/revision baseline: %v", err)
	}
	var timezoneUnset bool
	if err := f.conn.QueryRow(f.ctx, `SELECT bool_and(planning_timezone IS NULL) FROM workspace`).Scan(&timezoneUnset); err != nil || !timezoneUnset {
		t.Fatalf("migration silently configured timezone: %v", err)
	}
	var ws pgtype.UUID
	if err := ws.Scan("10000000-0000-0000-0000-000000000001"); err != nil {
		t.Fatal(err)
	}
	rows, err := f.conn.Query(f.ctx, "SELECT id FROM project")
	if err != nil {
		t.Fatal(err)
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[pgtype.UUID])
	if err != nil {
		t.Fatal(err)
	}
	stats, err := db.New(f.conn).GetProjectIssueStats(f.ctx, db.GetProjectIssueStatsParams{WorkspaceID: ws, ProjectIds: ids, TerminalStatusKeys: []string{"done", "cancelled"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(stats) != 5 {
		t.Fatalf("stats returned %d projects", len(stats))
	}
	for _, stat := range stats {
		if stat.TotalCount != 6 || stat.DoneCount != 4 {
			t.Errorf("legacy count must include formal done+cancelled only: %+v", stat)
		}
	}
}

func TestProjectP1EmptyMaintenanceRollbackAndCatalog(t *testing.T) {
	f := newProjectP1Fixture(t)
	projectP1Up(t, f)
	var indexes, invalid, foreignKeys int
	if err := f.conn.QueryRow(f.ctx, `SELECT count(*),count(*) FILTER(WHERE NOT (i.indisvalid AND i.indisready AND i.indislive)) FROM pg_index i JOIN pg_class c ON c.oid=i.indrelid JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname=$1`, f.schema).Scan(&indexes, &invalid); err != nil {
		t.Fatal(err)
	}
	if indexes != 13 || invalid != 0 {
		t.Fatalf("P1 catalog indexes=%d invalid=%d, want exactly 13 valid explicit indexes", indexes, invalid)
	}
	if err := f.conn.QueryRow(f.ctx, `SELECT count(*) FROM pg_constraint c JOIN pg_namespace n ON n.oid=c.connamespace WHERE n.nspname=$1 AND c.contype='f'`, f.schema).Scan(&foreignKeys); err != nil || foreignKeys != 0 {
		t.Fatalf("P1 added foreign keys: %d (%v)", foreignKeys, err)
	}
	files := projectP1Files(t, "down")
	for i := len(files) - 1; i >= 0; i-- {
		applyMigrationFile(t, f.ctx, f.conn, files[i])
	}
	var remaining int
	if err := f.conn.QueryRow(f.ctx, `SELECT count(*) FROM information_schema.columns WHERE table_schema=$1 AND (table_name=ANY($2::text[]) OR column_name IN ('revision','description_revision','planning_timezone','in_progress_since','in_progress_since_source'))`, f.schema, projectP1Tables).Scan(&remaining); err != nil || remaining != 0 {
		t.Fatalf("empty downgrade left P1 schema: %d (%v)", remaining, err)
	}
}

var projectP1ProtectedWrites = []struct{ name, table, write string }{
	{"state audit", "project_state_change", `INSERT INTO project_state_change(workspace_id,project_id,actor_type,actor_id,from_status,to_status,project_revision) VALUES(gen_random_uuid(),gen_random_uuid(),'member',gen_random_uuid(),'planned','completed',2)`},
	{"progress", "project_update", `INSERT INTO project_update(workspace_id,project_id,author_user_id) VALUES(gen_random_uuid(),gen_random_uuid(),gen_random_uuid())`},
	{"revision", "project_update_revision", `INSERT INTO project_update_revision(workspace_id,project_id,update_id,revision,editor_user_id,kind,body) VALUES(gen_random_uuid(),gen_random_uuid(),gen_random_uuid(),1,gen_random_uuid(),'progress','retained')`},
	{"request tombstone", "project_update_request", `INSERT INTO project_update_request(workspace_id,project_id,actor_user_id,request_id,operation,payload_hash,update_id,result_revision) VALUES(gen_random_uuid(),gen_random_uuid(),gen_random_uuid(),gen_random_uuid(),'create','retained',gen_random_uuid(),1)`},
	{"outbox", "project_update_notification", `INSERT INTO project_update_notification(workspace_id,project_id,update_id,recipient_user_id,source_revision) VALUES(gen_random_uuid(),gen_random_uuid(),gen_random_uuid(),gen_random_uuid(),1)`},
	{"project version", "project", `INSERT INTO project(id,revision) VALUES(gen_random_uuid(),2)`},
	{"description version", "project", `INSERT INTO project(id,description_revision) VALUES(gen_random_uuid(),2)`},
	{"transition clock", "project", `INSERT INTO project(id,in_progress_since,in_progress_since_source) VALUES(gen_random_uuid(),now(),'transition')`},
	{"planning timezone", "workspace", `INSERT INTO workspace(id,planning_timezone) VALUES(gen_random_uuid(),'Asia/Shanghai')`},
}

func TestProjectP1EveryDownPreservesProtectedData(t *testing.T) {
	for _, scenario := range projectP1ProtectedWrites {
		t.Run(scenario.name, func(t *testing.T) {
			f := newProjectP1Fixture(t)
			projectP1Up(t, f)
			projectP1Exec(t, f, scenario.write)
			before := projectP1Snapshot(t, f, scenario.table)
			for _, file := range projectP1Files(t, "down") {
				t.Run(file, func(t *testing.T) {
					_, err := f.conn.Exec(f.ctx, readMigrationFile(t, file))
					projectP1WantState(t, err, "P0001")
					if got := projectP1Snapshot(t, f, scenario.table); got != before {
						t.Fatal("refused downgrade changed retained evidence")
					}
				})
			}
			var indexes int
			if err := f.conn.QueryRow(f.ctx, `SELECT count(*) FROM pg_index i JOIN pg_class c ON c.oid=i.indrelid WHERE c.relnamespace=$1::regnamespace`, f.schema).Scan(&indexes); err != nil || indexes != 13 {
				t.Fatalf("refused downgrade removed an index: %d (%v)", indexes, err)
			}
		})
	}
}

func TestProjectP1EveryDownRefusesConcurrentWriterWithoutWaiting(t *testing.T) {
	for _, scenario := range projectP1ProtectedWrites {
		t.Run(scenario.name, func(t *testing.T) {
			f := newProjectP1Fixture(t)
			projectP1Up(t, f)
			tx, err := f.writer.Begin(f.ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(context.Background())
			if _, err = tx.Exec(f.ctx, scenario.write); err != nil {
				t.Fatal(err)
			}
			for _, file := range projectP1Files(t, "down") {
				t.Run(file, func(t *testing.T) {
					ctx, cancel := context.WithTimeout(f.ctx, time.Second)
					defer cancel()
					_, err := f.conn.Exec(ctx, readMigrationFile(t, file))
					projectP1WantState(t, err, "55P03")
				})
			}
			if err = tx.Commit(f.ctx); err != nil {
				t.Fatal(err)
			}
			if got := projectP1Snapshot(t, f, scenario.table); got == "[]" {
				t.Fatal("concurrent committed evidence was lost")
			}
			_, err = f.conn.Exec(f.ctx, readMigrationFile(t, "536_project_p1_tables.down.sql"))
			projectP1WantState(t, err, "P0001")
		})
	}
}

func TestProjectP1DownFirstCannotAcknowledgeLateWriter(t *testing.T) {
	f := newProjectP1Fixture(t)
	projectP1Up(t, f)
	tx, err := f.conn.Begin(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	if _, err = tx.Exec(f.ctx, readMigrationFile(t, "536_project_p1_tables.down.sql")); err != nil {
		t.Fatal(err)
	}
	result := make(chan error, 1)
	go func() { _, err := f.writer.Exec(f.ctx, projectP1ProtectedWrites[1].write); result <- err }()
	// Observe a database lock barrier, not an assumed scheduling delay.
	for {
		var waiting bool
		if err := tx.QueryRow(f.ctx, `SELECT EXISTS(SELECT 1 FROM pg_locks WHERE pid=$1 AND NOT granted)`, f.writer.PgConn().PID()).Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if waiting {
			break
		}
		select {
		case err := <-result:
			t.Fatalf("late writer did not wait for downgrade: %v", err)
		case <-f.ctx.Done():
			t.Fatal(f.ctx.Err())
		case <-time.After(5 * time.Millisecond):
		}
	}
	if err = tx.Commit(f.ctx); err != nil {
		t.Fatal(err)
	}
	projectP1WantState(t, <-result, "42P01")
}

func TestProjectP1IndexesAreExplicitConcurrentSingleStatements(t *testing.T) {
	pattern := regexp.MustCompile(`(?is)^CREATE (?:UNIQUE )?INDEX CONCURRENTLY IF NOT EXISTS [a-z_]+ ON [a-z_]+ .+;$`)
	for _, file := range projectP1Files(t, "up") {
		sql := strings.TrimSpace(readMigrationFile(t, file))
		if strings.HasPrefix(file, "536_") {
			if regexp.MustCompile(`(?i)\b(PRIMARY KEY|UNIQUE|REFERENCES|FOREIGN KEY|CASCADE|CREATE INDEX)\b`).MatchString(sql) {
				t.Fatal("P1 table migration introduces implicit index or database relationship")
			}
			continue
		}
		if !pattern.MatchString(sql) || strings.Count(sql, ";") != 1 {
			t.Errorf("%s is not one concurrent index statement", file)
		}
	}
}

func projectP1Runner(t *testing.T, files []string) func(*projectP1Fixture, string) ([]byte, error) {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "migrate")
	build := exec.Command("go", "build", "-o", bin, "./cmd/migrate")
	build.Dir = filepath.Dir(realMigrationsDir(t))
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build production migration runner: %v\n%s", err, output)
	}
	if err := os.Mkdir(filepath.Join(dir, "migrations"), 0700); err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		if err := os.WriteFile(filepath.Join(dir, "migrations", file), []byte(readMigrationFile(t, file)), 0600); err != nil {
			t.Fatal(err)
		}
	}
	return func(f *projectP1Fixture, direction string) ([]byte, error) {
		cmd := exec.CommandContext(f.ctx, bin, direction)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "DATABASE_URL="+f.databaseURL)
		return cmd.CombinedOutput()
	}
}

func TestProjectP1RunnerRecoversFailedConcurrentIndexAndLedger(t *testing.T) {
	f := newProjectP1Fixture(t)
	run := projectP1Runner(t, projectP1Files(t, "up"))
	applyMigrationFile(t, f.ctx, f.conn, "536_project_p1_tables.up.sql")
	projectP1Exec(t, f, `INSERT INTO project_state_change(id,workspace_id,project_id,actor_type,actor_id,from_status,to_status,project_revision)
SELECT '20000000-0000-0000-0000-000000000001',gen_random_uuid(),gen_random_uuid(),'member',gen_random_uuid(),'planned','completed',2 FROM generate_series(1,2)`)
	output, err := run(f, "up")
	if err == nil || !strings.Contains(string(output), "23505") {
		t.Fatalf("expected real concurrent unique-build failure: %v\n%s", err, output)
	}
	var valid, recorded bool
	if err = f.conn.QueryRow(f.ctx, `SELECT indisvalid FROM pg_index WHERE indexrelid='project_state_change_id'::regclass`).Scan(&valid); err != nil || valid {
		t.Fatalf("failed build did not leave INVALID index: %t (%v)", valid, err)
	}
	if err = f.conn.QueryRow(f.ctx, `SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version='537_project_state_change_id')`).Scan(&recorded); err != nil || recorded {
		t.Fatalf("failed migration must not advance ledger: %t (%v)", recorded, err)
	}
	projectP1Exec(t, f, `DELETE FROM project_state_change WHERE ctid=(SELECT max(ctid) FROM project_state_change)`)
	output, err = run(f, "up")
	if err != nil {
		t.Fatalf("retry failed: %v\n%s", err, output)
	}
	if err = f.conn.QueryRow(f.ctx, `SELECT indisvalid AND indisready AND indislive FROM pg_index WHERE indexrelid='project_state_change_id'::regclass`).Scan(&valid); err != nil || !valid {
		t.Fatalf("runner recorded success without repairing INVALID index: %t (%v)\n%s", valid, err, output)
	}
	if err = f.conn.QueryRow(f.ctx, `SELECT count(*)=14 FROM schema_migrations`).Scan(&recorded); err != nil || !recorded {
		t.Fatalf("incomplete P1 ledger: %t (%v)", recorded, err)
	}
	before := projectP1Snapshot(t, f, "project_state_change")
	projectP1Exec(t, f, `DROP INDEX CONCURRENTLY project_state_change_id`)
	// A recorded version alone cannot heal a missing index. Recovery must reset
	// only its ledger row, then run the real up migration and verify the catalog.
	output, err = run(f, "up")
	if err != nil {
		t.Fatalf("recorded migration rerun: %v\n%s", err, output)
	}
	var absent bool
	if err = f.conn.QueryRow(f.ctx, `SELECT to_regclass('project_state_change_id') IS NULL`).Scan(&absent); err != nil || !absent {
		t.Fatalf("missing-index drill expected ledger-only skip: %v", err)
	}
	projectP1Exec(t, f, `DELETE FROM schema_migrations WHERE version='537_project_state_change_id'`)
	output, err = run(f, "up")
	if err != nil {
		t.Fatalf("targeted ledger recovery: %v\n%s", err, output)
	}
	if err = f.conn.QueryRow(f.ctx, `SELECT indisvalid AND indisready AND indislive FROM pg_index WHERE indexrelid='project_state_change_id'::regclass`).Scan(&valid); err != nil || !valid {
		t.Fatalf("targeted recovery did not restore usable index: %v", err)
	}
	if got := projectP1Snapshot(t, f, "project_state_change"); got != before {
		t.Fatal("index recovery changed retained audit data")
	}
}

func TestProjectP1RunnerMaintenanceDownAndPartialRecovery(t *testing.T) {
	f := newProjectP1Fixture(t)
	run := projectP1Runner(t, append(projectP1Files(t, "up"), projectP1Files(t, "down")...))
	if output, err := run(f, "up"); err != nil {
		t.Fatalf("empty-schema runner up: %v\n%s", err, output)
	}
	// Model a process stopping after the first successful down step. The next
	// step really fails with a concurrent writer; up must restore both catalog
	// and ledger rather than silently accepting the partially removed schema.
	applyMigrationFile(t, f.ctx, f.conn, "549_project_update_notification_project.down.sql")
	projectP1Exec(t, f, "DELETE FROM schema_migrations WHERE version='549_project_update_notification_project'")
	tx, err := f.writer.Begin(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	if _, err = tx.Exec(f.ctx, "INSERT INTO workspace(id) VALUES(gen_random_uuid())"); err != nil {
		t.Fatal(err)
	}
	output, err := run(f, "down")
	if err == nil || !strings.Contains(string(output), "55P03") {
		t.Fatalf("partial down must refuse a concurrent writer: %v\n%s", err, output)
	}
	if err = tx.Rollback(f.ctx); err != nil {
		t.Fatal(err)
	}
	output, err = run(f, "up")
	if err != nil {
		t.Fatalf("partial-down recovery: %v\n%s", err, output)
	}
	var intact bool
	if err = f.conn.QueryRow(f.ctx, `SELECT (SELECT count(*)=14 FROM schema_migrations) AND (SELECT indisvalid AND indisready AND indislive FROM pg_index WHERE indexrelid='project_update_notification_project'::regclass)`).Scan(&intact); err != nil || !intact {
		t.Fatalf("partial recovery must restore catalog and ledger: %v", err)
	}
	output, err = run(f, "down")
	if err != nil {
		t.Fatalf("maintenance down: %v\n%s", err, output)
	}
	if err = f.conn.QueryRow(f.ctx, `SELECT count(*)=0 AND to_regclass('project_update') IS NULL FROM schema_migrations`).Scan(&intact); err != nil || !intact {
		t.Fatalf("maintenance down must remove both P1 objects and ledger: %v", err)
	}
}

func TestProjectP1WorkspaceDeletionClearsAllNewTables(t *testing.T) {
	f := newProjectP1Fixture(t)
	projectP1Up(t, f)
	// These unrelated empty relation shapes allow the real generated workspace
	// deletion query to execute; only P1-owned data is under assertion here.
	for _, table := range []string{"channel_installation", "chat_session", "agent", "skill", "dingtalk_group_route", "channel_user_binding", "channel_binding_token", "issue_property", "quick_action", "workspace_mcp_server", "github_pending_check_suite", "github_pull_request", "vcs_pull_request", "vcs_connection", "client_usage_daily"} {
		projectP1Exec(t, f, "CREATE TABLE "+pgx.Identifier{table}.Sanitize()+"(id uuid,workspace_id uuid)")
	}
	for _, table := range []string{"channel_task_delivery", "channel_outbound_message", "channel_inbound_message_dedup", "dingtalk_group_presence", "dingtalk_bot_identity", "channel_inbound_audit"} {
		projectP1Exec(t, f, "CREATE TABLE "+pgx.Identifier{table}.Sanitize()+"(installation_id uuid)")
	}
	for _, table := range []string{"channel_chat_context_generation", "channel_outbound_card_message", "chat_draft_restore"} {
		projectP1Exec(t, f, "CREATE TABLE "+pgx.Identifier{table}.Sanitize()+"(chat_session_id uuid)")
	}
	projectP1Exec(t, f, `CREATE TABLE channel_chat_session_binding(installation_id uuid,chat_session_id uuid);
CREATE TABLE agent_to_label(agent_id uuid); CREATE TABLE skill_to_label(skill_id uuid);
CREATE TABLE agent_mcp_server(server_id uuid,agent_id uuid); CREATE TABLE github_pull_request_check_run(pr_id uuid);
CREATE TABLE issue_vcs_pull_request(pull_request_id uuid); CREATE TABLE vcs_commit_status(connection_id uuid);
INSERT INTO workspace(id) VALUES('10000000-0000-0000-0000-000000000001'),('10000000-0000-0000-0000-000000000002');`)
	var ws pgtype.UUID
	if err := ws.Scan("10000000-0000-0000-0000-000000000001"); err != nil {
		t.Fatal(err)
	}
	other := map[string]string{}
	for _, scenario := range projectP1ProtectedWrites[:5] {
		projectP1Exec(t, f, scenario.write)
		projectP1Exec(t, f, "UPDATE "+pgx.Identifier{scenario.table}.Sanitize()+" SET workspace_id='10000000-0000-0000-0000-000000000002'")
		other[scenario.table] = projectP1Snapshot(t, f, scenario.table)
		projectP1Exec(t, f, scenario.write)
		projectP1Exec(t, f, "UPDATE "+pgx.Identifier{scenario.table}.Sanitize()+" SET workspace_id=$1 WHERE workspace_id <> '10000000-0000-0000-0000-000000000002'", ws)
	}
	before := map[string]string{}
	for _, table := range projectP1Tables {
		before[table] = projectP1Snapshot(t, f, table)
	}
	projectP1Exec(t, f, `CREATE FUNCTION refuse_workspace_delete() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected workspace failure'; END $$;
CREATE TRIGGER refuse_workspace_delete BEFORE DELETE ON workspace FOR EACH ROW EXECUTE FUNCTION refuse_workspace_delete();`)
	err := db.New(f.conn).DeleteWorkspace(f.ctx, ws)
	projectP1WantState(t, err, "P0001")
	for _, table := range projectP1Tables {
		if got := projectP1Snapshot(t, f, table); got != before[table] {
			t.Errorf("failed workspace deletion partially removed %s", table)
		}
	}
	projectP1Exec(t, f, "DROP TRIGGER refuse_workspace_delete ON workspace")
	if err := db.New(f.conn).DeleteWorkspace(f.ctx, ws); err != nil {
		t.Fatal(err)
	}
	for _, table := range projectP1Tables {
		if got := projectP1Snapshot(t, f, table); got != other[table] {
			t.Errorf("workspace cleanup must remove all own %s and preserve other workspace: %s", table, got)
		}
	}
}
