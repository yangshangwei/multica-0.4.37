package migrations

import (
	"context"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const (
	builtinAgentLegacyDescriptionEN = "Your workspace Chief of Staff. Mika turns goals into issues, coordinates agents, and helps build reusable workflows."
	builtinAgentLegacyDescriptionZH = "你的工作区 Chief of Staff。Mika 会把目标转化为任务、协调智能体，并帮你建立可复用的工作流。"
	builtinAgentDescriptionEN       = "Your workspace Chief of Staff. Turns goals into issues, coordinates agents, and helps build reusable workflows."
	builtinAgentDescriptionZH       = "你的工作区 Chief of Staff。把目标转化为任务、协调智能体，并帮你建立可复用的工作流。"
)

func TestBuiltinAgentDisplayNameMigrationPreservesCustomizations(t *testing.T) {
	ctx := context.Background()
	conn, notices := setupBuiltinAgentDisplayNameMigration(t)
	before := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	cases := []struct {
		label, name, systemKey, description, wantName, wantDescription string
		archived, conflict, archivedConflict, changed                  bool
	}{
		{label: "English default", name: "Mika", systemKey: "mika", description: builtinAgentLegacyDescriptionEN, wantName: "小阿孚", wantDescription: builtinAgentDescriptionEN, changed: true},
		{label: "Chinese default", name: "Mika", systemKey: "mika", description: builtinAgentLegacyDescriptionZH, wantName: "小阿孚", wantDescription: builtinAgentDescriptionZH, changed: true},
		{label: "custom description", name: "Mika", systemKey: "mika", description: "Mika handles our private workflow.", wantName: "小阿孚", wantDescription: "Mika handles our private workflow.", changed: true},
		{label: "empty description", name: "Mika", systemKey: "mika", wantName: "小阿孚", changed: true},
		{label: "custom name and description", name: "小齐", systemKey: "mika", description: "Custom Mika notes", wantName: "小齐", wantDescription: "Custom Mika notes"},
		{label: "custom name with old default description", name: "阿策", systemKey: "mika", description: builtinAgentLegacyDescriptionEN, wantName: "阿策", wantDescription: builtinAgentLegacyDescriptionEN},
		{label: "ordinary Mika", name: "Mika", description: builtinAgentLegacyDescriptionEN, wantName: "Mika", wantDescription: builtinAgentLegacyDescriptionEN},
		{label: "different system identity", name: "Mika", systemKey: "builder", description: builtinAgentLegacyDescriptionZH, wantName: "Mika", wantDescription: builtinAgentLegacyDescriptionZH},
		{label: "already renamed", name: "小阿孚", systemKey: "mika", description: builtinAgentLegacyDescriptionEN, wantName: "小阿孚", wantDescription: builtinAgentDescriptionEN, changed: true},
		{label: "already current", name: "小阿孚", systemKey: "mika", description: builtinAgentDescriptionZH, wantName: "小阿孚", wantDescription: builtinAgentDescriptionZH},
		{label: "already renamed with custom description", name: "小阿孚", systemKey: "mika", description: "Keep Mika in this custom description.", wantName: "小阿孚", wantDescription: "Keep Mika in this custom description."},
		{label: "ordinary new name", name: "小阿孚", description: builtinAgentLegacyDescriptionZH, wantName: "小阿孚", wantDescription: builtinAgentLegacyDescriptionZH},
		{label: "archived built-in", name: "Mika", systemKey: "mika", description: builtinAgentLegacyDescriptionEN, wantName: "小阿孚", wantDescription: builtinAgentDescriptionEN, archived: true, changed: true},
		{label: "active conflict", name: "Mika", systemKey: "mika", description: builtinAgentLegacyDescriptionEN, wantName: "Mika", wantDescription: builtinAgentLegacyDescriptionEN, conflict: true},
		{label: "archived conflict", name: "Mika", systemKey: "mika", description: builtinAgentLegacyDescriptionZH, wantName: "Mika", wantDescription: builtinAgentLegacyDescriptionZH, conflict: true, archivedConflict: true},
	}
	type expectedRow struct {
		label, id, workspaceID, name, description, preserved string
		changed                                              bool
	}
	var agents, sessions []expectedRow
	var conflictIdentities []string
	for _, tc := range cases {
		agentID, workspaceID := uuid.NewString(), uuid.NewString()
		if _, err := conn.Exec(ctx, `
			INSERT INTO agent (id, workspace_id, name, system_key, description, instructions, archived_at, created_at, updated_at)
			VALUES ($1, $2, $3, NULLIF($4, ''), $5, 'Keep these custom Mika instructions.', CASE WHEN $6 THEN $7::timestamptz END, $7, $7)
		`, agentID, workspaceID, tc.name, tc.systemKey, tc.description, tc.archived, before); err != nil {
			t.Fatalf("seed %s: %v", tc.label, err)
		}
		var preserved string
		if err := conn.QueryRow(ctx, `SELECT (to_jsonb(agent) - 'name' - 'description' - 'updated_at')::text FROM agent WHERE id = $1`, agentID).Scan(&preserved); err != nil {
			t.Fatal(err)
		}
		agents = append(agents, expectedRow{label: tc.label, id: agentID, workspaceID: workspaceID, name: tc.wantName, description: tc.wantDescription, preserved: preserved, changed: tc.changed})
		if tc.conflict {
			conflictID := uuid.NewString()
			if _, err := conn.Exec(ctx, `
				INSERT INTO agent (id, workspace_id, name, description, instructions, archived_at, created_at, updated_at)
				VALUES ($1, $2, '小阿孚', 'Existing owner content', 'Keep existing instructions', CASE WHEN $3 THEN $4::timestamptz END, $4, $4)
			`, conflictID, workspaceID, tc.archivedConflict, before); err != nil {
				t.Fatalf("seed %s target: %v", tc.label, err)
			}
			if err := conn.QueryRow(ctx, `SELECT (to_jsonb(agent) - 'name' - 'description' - 'updated_at')::text FROM agent WHERE id = $1`, conflictID).Scan(&preserved); err != nil {
				t.Fatal(err)
			}
			agents = append(agents, expectedRow{label: tc.label + " target", id: conflictID, workspaceID: workspaceID, name: "小阿孚", description: "Existing owner content", preserved: preserved})
			conflictIdentities = append(conflictIdentities, agentID, workspaceID)
		}
		for _, title := range []struct{ old, renamed, status string }{
			{"Getting started with Mika", "Getting started with 小阿孚", "active"},
			{"和 Mika 开始", "开始使用小阿孚", "archived"},
			{"Mika planning notes", "Mika planning notes", "active"},
			{"Getting started with 小阿孚", "Getting started with 小阿孚", "active"},
		} {
			sessionID := uuid.NewString()
			if _, err := conn.Exec(ctx, `INSERT INTO chat_session (id, workspace_id, agent_id, title, status, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $6)`, sessionID, workspaceID, agentID, title.old, title.status, before); err != nil {
				t.Fatalf("seed %s session: %v", tc.label, err)
			}
			wantTitle := title.old
			if tc.systemKey == "mika" && tc.wantName == "小阿孚" {
				wantTitle = title.renamed
			}
			if err := conn.QueryRow(ctx, `SELECT (to_jsonb(chat_session) - 'title' - 'updated_at')::text FROM chat_session WHERE id = $1`, sessionID).Scan(&preserved); err != nil {
				t.Fatal(err)
			}
			sessions = append(sessions, expectedRow{label: tc.label + "/" + title.old, id: sessionID, name: wantTitle, preserved: preserved, changed: wantTitle != title.old})
		}
	}
	if _, err := conn.Exec(ctx, `
		INSERT INTO chat_message (id, chat_session_id, role, content, message_kind) VALUES
			(gen_random_uuid(), $1, 'assistant', 'Hello, I am Mika.', 'message'),
			(gen_random_uuid(), $1, 'user', 'Mika, keep this history.', 'message'),
			(gen_random_uuid(), $1, 'user', 'Mika onboarding context', 'onboarding_kickoff');
	`, sessions[0].id); err != nil {
		t.Fatalf("seed history: %v", err)
	}
	if _, err := conn.Exec(ctx, `INSERT INTO issue (id, workspace_id, title, description, assignee_id) VALUES (gen_random_uuid(), $1, 'Mika follow-up', 'Historical Mika content', $2)`, agents[0].workspaceID, agents[0].id); err != nil {
		t.Fatalf("seed assigned issue: %v", err)
	}
	if _, err := conn.Exec(ctx, `INSERT INTO agent_task_queue (id, agent_id, chat_session_id, prompt) VALUES (gen_random_uuid(), $1, $2, 'Historical Mika prompt')`, agents[0].id, sessions[0].id); err != nil {
		t.Fatalf("seed historical task: %v", err)
	}
	historyBefore := snapshotBuiltinAgentMigrationTables(t, conn, "chat_message", "issue", "agent_task_queue")

	applyMigrationFile(t, ctx, conn, "457_builtin_agent_display_name.up.sql")
	for _, want := range agents {
		t.Run(want.label, func(t *testing.T) {
			var name, description, preserved string
			var updatedAt time.Time
			if err := conn.QueryRow(ctx, `SELECT name, description, updated_at, (to_jsonb(agent) - 'name' - 'description' - 'updated_at')::text FROM agent WHERE id = $1`, want.id).Scan(&name, &description, &updatedAt, &preserved); err != nil {
				t.Fatal(err)
			}
			if name != want.name || description != want.description {
				t.Errorf("content = %q / %q, want %q / %q", name, description, want.name, want.description)
			}
			if preserved != want.preserved {
				t.Errorf("agent identity, instructions or other metadata changed: %s", preserved)
			}
			if want.changed && !updatedAt.After(before) || !want.changed && !updatedAt.Equal(before) {
				t.Errorf("updated_at = %v, content changed = %t, original = %v", updatedAt, want.changed, before)
			}
		})
	}
	for _, want := range sessions {
		var title, preserved string
		var updatedAt time.Time
		if err := conn.QueryRow(ctx, `SELECT title, updated_at, (to_jsonb(chat_session) - 'title' - 'updated_at')::text FROM chat_session WHERE id = $1`, want.id).Scan(&title, &updatedAt, &preserved); err != nil {
			t.Fatal(err)
		}
		if title != want.name || preserved != want.preserved {
			t.Errorf("%s: session title/metadata = %q / %s, want %q / %s", want.label, title, preserved, want.name, want.preserved)
		}
		if want.changed && !updatedAt.After(before) || !want.changed && !updatedAt.Equal(before) {
			t.Errorf("%s: updated_at = %v, content changed = %t, original = %v", want.label, updatedAt, want.changed, before)
		}
	}
	if len(*notices) != 2 {
		t.Errorf("conflict notices = %v, want one per skipped agent", *notices)
	}
	for _, id := range conflictIdentities {
		if !strings.Contains(strings.Join(*notices, "\n"), id) {
			t.Errorf("conflict notice omitted agent/workspace identity %s: %v", id, *notices)
		}
	}
	if got := snapshotBuiltinAgentMigrationTables(t, conn, "chat_message", "issue", "agent_task_queue"); !reflect.DeepEqual(got, historyBefore) {
		t.Errorf("migration changed historical messages, kickoff, assigned issues or task snapshots")
	}

	tables := []string{"agent", "chat_session", "chat_message", "issue", "agent_task_queue"}
	afterUp := snapshotBuiltinAgentMigrationTables(t, conn, tables...)
	applyMigrationFile(t, ctx, conn, "457_builtin_agent_display_name.up.sql")
	if got := snapshotBuiltinAgentMigrationTables(t, conn, tables...); !reflect.DeepEqual(got, afterUp) {
		t.Error("replaying the migration changed content or timestamps")
	}

	// Owners may customize content between release and code rollback. The down
	// migration must preserve those choices as well as already migrated names.
	if _, err := conn.Exec(ctx, `UPDATE agent SET name = '自选名称', description = 'Edited after migration', instructions = 'New owner instructions', updated_at = now() WHERE id = $1`, agents[0].id); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(ctx, `UPDATE chat_session SET title = '自选会话', updated_at = now() WHERE id = $1`, sessions[0].id); err != nil {
		t.Fatal(err)
	}
	beforeDown := snapshotBuiltinAgentMigrationTables(t, conn, tables...)
	applyMigrationFile(t, ctx, conn, "457_builtin_agent_display_name.down.sql")
	if got := snapshotBuiltinAgentMigrationTables(t, conn, tables...); !reflect.DeepEqual(got, beforeDown) {
		t.Error("code rollback reverted editable workspace content")
	}
}

func TestBuiltinAgentDisplayNameMigrationContinuesAfterConcurrentNameConflict(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	conn, notices := setupBuiltinAgentDisplayNameMigration(t)
	blockedID, blockedWorkspace := uuid.NewString(), uuid.NewString()
	otherID, otherWorkspace := uuid.NewString(), uuid.NewString()
	before := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	if _, err := conn.Exec(ctx, `INSERT INTO agent (id, workspace_id, name, system_key, description, updated_at) VALUES ($1, $2, 'Mika', 'mika', $5, $6), ($3, $4, 'Mika', 'mika', $5, $6)`, blockedID, blockedWorkspace, otherID, otherWorkspace, builtinAgentLegacyDescriptionEN, before); err != nil {
		t.Fatal(err)
	}
	blocker, err := pgx.ConnectConfig(ctx, conn.Config().Copy())
	if err != nil {
		t.Fatal(err)
	}
	defer blocker.Close(context.Background())
	tx, err := blocker.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	if _, err := tx.Exec(ctx, `SELECT id FROM agent WHERE id = $1 FOR UPDATE`, blockedID); err != nil {
		t.Fatal(err)
	}

	// Holding the candidate row makes the migration wait after its preflight
	// name check. A second transaction can then claim the name before UPDATE.
	migration := readMigrationFile(t, "457_builtin_agent_display_name.up.sql")
	done := make(chan error, 1)
	go func() {
		_, err := conn.Exec(ctx, migration)
		done <- err
	}()
	finished := false
	defer func() {
		cancel()
		if !finished {
			<-done
		}
	}()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		var blocked bool
		if err := tx.QueryRow(ctx, `SELECT $1::int = ANY(pg_blocking_pids($2::int))`, int32(blocker.PgConn().PID()), int32(conn.PgConn().PID())).Scan(&blocked); err != nil {
			t.Fatalf("observe migration row lock: %v", err)
		}
		if blocked {
			break
		}
		select {
		case err := <-done:
			finished = true
			t.Fatalf("migration ended before the concurrent conflict: %v", err)
		case <-ctx.Done():
			t.Fatal("migration did not wait for the candidate row")
		case <-ticker.C:
		}
	}
	if _, err := tx.Exec(ctx, `INSERT INTO agent (id, workspace_id, name, description) VALUES ($1, $2, '小阿孚', 'Concurrent owner content')`, uuid.NewString(), blockedWorkspace); err != nil {
		t.Fatalf("claim name concurrently: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	err = <-done
	finished = true
	if err != nil {
		t.Fatalf("one concurrent conflict aborted the migration: %v", err)
	}
	var name, description string
	var updatedAt time.Time
	if err := conn.QueryRow(ctx, `SELECT name, description, updated_at FROM agent WHERE id = $1`, blockedID).Scan(&name, &description, &updatedAt); err != nil {
		t.Fatal(err)
	}
	if name != "Mika" || description != builtinAgentLegacyDescriptionEN || !updatedAt.Equal(before) {
		t.Errorf("conflicted agent was modified: %q / %q / %v", name, description, updatedAt)
	}
	if err := conn.QueryRow(ctx, `SELECT name, description FROM agent WHERE id = $1`, otherID).Scan(&name, &description); err != nil {
		t.Fatal(err)
	}
	if name != "小阿孚" || description != builtinAgentDescriptionEN {
		t.Errorf("unrelated workspace was not migrated: %q / %q", name, description)
	}
	if len(*notices) != 1 || !strings.Contains((*notices)[0], blockedID) || !strings.Contains((*notices)[0], blockedWorkspace) {
		t.Errorf("missing concurrent conflict notice with agent/workspace identities: %v", *notices)
	}
}

func setupBuiltinAgentDisplayNameMigration(t *testing.T) (*pgx.Conn, *[]string) {
	t.Helper()
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("integration test requires Postgres at DATABASE_URL")
	}
	config, err := pgx.ParseConfig(dbURL)
	if err != nil {
		t.Fatal(err)
	}
	schema := "builtin_agent_name_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	config.RuntimeParams["search_path"] = schema
	var notices []string
	config.OnNotice = func(_ *pgconn.PgConn, notice *pgconn.Notice) {
		notices = append(notices, notice.Message)
	}
	ctx := context.Background()
	conn, err := pgx.ConnectConfig(ctx, config)
	if err != nil {
		t.Fatalf("connect to Postgres: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close(context.Background()) })
	quotedSchema := pgx.Identifier{schema}.Sanitize()
	if _, err := conn.Exec(ctx, "CREATE SCHEMA "+quotedSchema); err != nil {
		t.Fatalf("create isolated schema: %v", err)
	}
	t.Cleanup(func() {
		if _, err := conn.Exec(context.Background(), "DROP SCHEMA "+quotedSchema+" CASCADE"); err != nil {
			t.Errorf("drop isolated schema: %v", err)
		}
	})
	if _, err := conn.Exec(ctx, `
		CREATE TABLE agent (
			id UUID PRIMARY KEY, workspace_id UUID NOT NULL, name TEXT NOT NULL,
			system_key TEXT, description TEXT NOT NULL DEFAULT '', instructions TEXT NOT NULL DEFAULT '',
			archived_at TIMESTAMPTZ, created_at TIMESTAMPTZ NOT NULL DEFAULT now(), updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			CONSTRAINT agent_workspace_name_unique UNIQUE (workspace_id, name)
		);
		CREATE TABLE chat_session (
			id UUID PRIMARY KEY, workspace_id UUID NOT NULL, agent_id UUID NOT NULL,
			title TEXT NOT NULL, status TEXT NOT NULL DEFAULT 'active',
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(), updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);
		CREATE TABLE chat_message (
			id UUID PRIMARY KEY, chat_session_id UUID NOT NULL, role TEXT NOT NULL,
			content TEXT NOT NULL, message_kind TEXT, task_id UUID,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);
		CREATE TABLE issue (
			id UUID PRIMARY KEY, workspace_id UUID NOT NULL, assignee_id UUID,
			title TEXT NOT NULL, description TEXT NOT NULL, updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);
		CREATE TABLE agent_task_queue (
			id UUID PRIMARY KEY, agent_id UUID NOT NULL, chat_session_id UUID, prompt TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);
	`); err != nil {
		t.Fatalf("create pre-migration tables: %v", err)
	}
	return conn, &notices
}

func snapshotBuiltinAgentMigrationTables(t *testing.T, conn *pgx.Conn, tables ...string) map[string]string {
	t.Helper()
	snapshot := make(map[string]string, len(tables))
	for _, table := range tables {
		var rows string
		if err := conn.QueryRow(context.Background(), `SELECT COALESCE(jsonb_agg(to_jsonb(row) ORDER BY row.id), '[]'::jsonb)::text FROM `+pgx.Identifier{table}.Sanitize()+` AS row`).Scan(&rows); err != nil {
			t.Fatalf("snapshot %s: %v", table, err)
		}
		snapshot[table] = rows
	}
	return snapshot
}
