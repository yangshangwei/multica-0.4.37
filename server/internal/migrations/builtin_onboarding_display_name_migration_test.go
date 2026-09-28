package migrations

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// These are the historical product templates, independent of today's handler.
const legacyOnboardingEN = `Hi — welcome to %s. Multica is a workspace where you and AI agents coordinate real work through issues.

I'm %s, your Chief of Staff here. I shape what needs doing, bring in the right agent for it, and stay your starting point for anything.

Here's how we begin: you name a goal, I turn it into an issue and start it with the right agent — and you watch it run.

Pick one below, or just tell me what you want to get done right now.`

const legacyOnboardingZH = `你好，欢迎来到 %s。Multica 是一个人和 AI 智能体通过任务一起把事情做完的工作区。

我是 %s，这里的 Chief of Staff。我负责把事情理清楚、找到合适的智能体接手，也是你随时可以开口的第一站。

接下来是这样：你说一个目标，我把它变成一个任务，交给合适的智能体开始跑，你能看着它推进。

从下面选一个开始，或者直接告诉我你现在想做成什么。`

func TestBuiltinOnboardingDisplayNameMigrationPreservesHistory(t *testing.T) {
	ctx := context.Background()
	conn, _ := setupBuiltinAgentDisplayNameMigration(t)
	cases := []struct {
		name, template, workspace, agentName, systemKey string
		openingSuffix, kickoffMode, extraRole, task     string
		openingRole, kickoffRole                        string
		change                                          bool
	}{
		{name: "English pristine", change: true},
		{name: "Chinese pristine", template: legacyOnboardingZH, workspace: "海卫七", change: true},
		{name: "workspace contains Mika", workspace: "Mika team", change: true},
		{name: "workspace was renamed", workspace: "Old workspace", change: true},
		{name: "workspace contains self introduction", workspace: "I'm Mika, your Chief of Staff here.", change: true},
		{name: "ordinary new-name agent", systemKey: "ordinary"},
		{name: "ordinary Mika", systemKey: "ordinary", agentName: "Mika"},
		{name: "custom builtin name", agentName: "Our assistant"},
		{name: "unrenamed conflicted builtin", agentName: "Mika"},
		{name: "custom opening", openingSuffix: "\nMika handles special requests."},
		{name: "custom template body", template: strings.Replace(legacyOnboardingEN, "you name a goal", "you name your private goal", 1)},
		{name: "user opening", openingRole: "user"},
		{name: "assistant kickoff", kickoffRole: "assistant"},
		{name: "missing kickoff", kickoffMode: "missing"},
		{name: "mismatched quote", kickoffMode: "mismatch"},
		{name: "missing quote delimiters", kickoffMode: "unquoted"},
		{name: "duplicate quoted opening", kickoffMode: "duplicate"},
		{name: "real user history", extraRole: "user"},
		{name: "real assistant history", extraRole: "assistant"},
		{name: "task owns opening", task: "opening"},
		{name: "task owns kickoff", task: "kickoff"},
		{name: "task history without messages", task: "history"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			template, workspace, name, systemKey := tc.template, tc.workspace, tc.agentName, tc.systemKey
			if template == "" {
				template = legacyOnboardingEN
			}
			if workspace == "" {
				workspace = "My workspace"
			}
			if name == "" {
				name = "小阿孚"
			}
			if systemKey == "" {
				systemKey = "mika"
			}
			opening := fmt.Sprintf(template, workspace, "Mika") + tc.openingSuffix
			kickoff := onboardingMigrationKickoff(opening, workspace)
			switch tc.kickoffMode {
			case "mismatch":
				kickoff = onboardingMigrationKickoff("A different greeting", workspace)
			case "unquoted":
				kickoff = opening
			case "duplicate":
				kickoff += "\n" + kickoff
			}
			sessionID, openingID, kickoffID := seedBuiltinOnboardingMigration(t, conn, name, systemKey, opening, kickoff)
			if tc.kickoffMode == "missing" {
				mustExecOnboardingMigration(t, conn, `DELETE FROM chat_message WHERE id = $1`, kickoffID)
			}
			if tc.openingRole != "" {
				mustExecOnboardingMigration(t, conn, `UPDATE chat_message SET role = $1 WHERE id = $2`, tc.openingRole, openingID)
			}
			if tc.kickoffRole != "" {
				mustExecOnboardingMigration(t, conn, `UPDATE chat_message SET role = $1 WHERE id = $2`, tc.kickoffRole, kickoffID)
			}
			if tc.extraRole != "" {
				mustExecOnboardingMigration(t, conn, `INSERT INTO chat_message (id, chat_session_id, role, content) VALUES ($1, $2, $3, 'Real conversation with Mika')`, uuid.NewString(), sessionID, tc.extraRole)
			}
			if tc.task != "" {
				taskID := uuid.NewString()
				if tc.task == "history" {
					mustExecOnboardingMigration(t, conn, `INSERT INTO agent_task_queue (id, agent_id, chat_session_id, prompt) SELECT $1, agent_id, id, 'Historic Mika task' FROM chat_session WHERE id = $2`, taskID, sessionID)
				} else {
					messageID := openingID
					if tc.task == "kickoff" {
						messageID = kickoffID
					}
					mustExecOnboardingMigration(t, conn, `UPDATE chat_message SET task_id = $1 WHERE id = $2`, taskID, messageID)
				}
			}
			before := snapshotBuiltinAgentMigrationTables(t, conn, "agent", "chat_session", "chat_message", "agent_task_queue")
			applyMigrationFile(t, ctx, conn, "458_builtin_onboarding_display_name.up.sql")
			after := snapshotBuiltinAgentMigrationTables(t, conn, "agent", "chat_session", "chat_message", "agent_task_queue")
			if !tc.change {
				if !reflect.DeepEqual(after, before) {
					t.Fatal("custom content or real history was modified")
				}
				return
			}
			var gotOpening, gotKickoff string
			if err := conn.QueryRow(ctx, `SELECT content FROM chat_message WHERE id = $1`, openingID).Scan(&gotOpening); err != nil {
				t.Fatal(err)
			}
			if err := conn.QueryRow(ctx, `SELECT content FROM chat_message WHERE id = $1`, kickoffID).Scan(&gotKickoff); err != nil {
				t.Fatal(err)
			}
			wantOpening := fmt.Sprintf(template, workspace, "小阿孚")
			if gotOpening != wantOpening || gotKickoff != onboardingMigrationKickoff(wantOpening, workspace) {
				t.Fatalf("pristine opening and quoted kickoff were not renamed consistently: opening=%q kickoff=%q", gotOpening, gotKickoff)
			}
			// Restoring only content must restore the entire database snapshot: IDs,
			// timestamps, ordering, profiles and agent/session fields all survive.
			mustExecOnboardingMigration(t, conn, `UPDATE chat_message SET content = $1 WHERE id = $2`, opening, openingID)
			mustExecOnboardingMigration(t, conn, `UPDATE chat_message SET content = $1 WHERE id = $2`, kickoff, kickoffID)
			if restored := snapshotBuiltinAgentMigrationTables(t, conn, "agent", "chat_session", "chat_message", "agent_task_queue"); !reflect.DeepEqual(restored, before) {
				t.Fatal("migration changed fields other than the two message contents")
			}
			mustExecOnboardingMigration(t, conn, `UPDATE chat_message SET content = $1 WHERE id = $2`, gotOpening, openingID)
			mustExecOnboardingMigration(t, conn, `UPDATE chat_message SET content = $1 WHERE id = $2`, gotKickoff, kickoffID)
		})
	}
	beforeReplay := snapshotBuiltinAgentMigrationTables(t, conn, "agent", "chat_session", "chat_message", "agent_task_queue")
	applyMigrationFile(t, ctx, conn, "458_builtin_onboarding_display_name.up.sql")
	if after := snapshotBuiltinAgentMigrationTables(t, conn, "agent", "chat_session", "chat_message", "agent_task_queue"); !reflect.DeepEqual(after, beforeReplay) {
		t.Fatal("replaying the migration changed data")
	}
	// Down must preserve both repaired defaults and later user edits.
	mustExecOnboardingMigration(t, conn, `UPDATE chat_message SET content = 'Owner edited greeting mentioning Mika' WHERE id = (SELECT id FROM chat_message ORDER BY id LIMIT 1)`)
	beforeDown := snapshotBuiltinAgentMigrationTables(t, conn, "agent", "chat_session", "chat_message", "agent_task_queue")
	applyMigrationFile(t, ctx, conn, "458_builtin_onboarding_display_name.down.sql")
	if after := snapshotBuiltinAgentMigrationTables(t, conn, "agent", "chat_session", "chat_message", "agent_task_queue"); !reflect.DeepEqual(after, beforeDown) {
		t.Fatal("down migration changed user content or repaired defaults")
	}
}

func TestBuiltinOnboardingDisplayNameMigrationRechecksAfterFirstSend(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	conn, _ := setupBuiltinAgentDisplayNameMigration(t)
	opening := fmt.Sprintf(legacyOnboardingEN, "My workspace", "Mika")
	sessionID, _, _ := seedBuiltinOnboardingMigration(t, conn, "小阿孚", "mika", opening, onboardingMigrationKickoff(opening, "My workspace"))
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
	// SendDirectChatMessage takes this same lock before writing a real turn.
	if _, err := tx.Exec(ctx, `SELECT id FROM chat_session WHERE id = $1 FOR UPDATE`, sessionID); err != nil {
		t.Fatal(err)
	}
	migration := readMigrationFile(t, "458_builtin_onboarding_display_name.up.sql")
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
			t.Fatal(err)
		}
		if blocked {
			break
		}
		select {
		case err := <-done:
			finished = true
			t.Fatalf("migration did not wait for the first-send session lock: %v", err)
		case <-ctx.Done():
			t.Fatal("migration never reached the session lock")
		case <-ticker.C:
		}
	}
	mustExecOnboardingMigration(t, blocker, `INSERT INTO chat_message (id, chat_session_id, role, content) VALUES ($1, $2, 'user', 'My first real request')`, uuid.NewString(), sessionID)
	mustExecOnboardingMigration(t, blocker, `INSERT INTO agent_task_queue (id, agent_id, chat_session_id, prompt) SELECT $1, agent_id, id, 'First real task' FROM chat_session WHERE id = $2`, uuid.NewString(), sessionID)
	before := snapshotBuiltinAgentMigrationTables(t, blocker, "agent", "chat_session", "chat_message", "agent_task_queue")
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	err = <-done
	finished = true
	if err != nil {
		t.Fatal(err)
	}
	if after := snapshotBuiltinAgentMigrationTables(t, conn, "agent", "chat_session", "chat_message", "agent_task_queue"); !reflect.DeepEqual(after, before) {
		t.Fatal("migration rewrote history after waiting for a first send to commit")
	}
}

func TestBuiltinOnboardingDisplayNameMigrationIsAtomic(t *testing.T) {
	ctx := context.Background()
	conn, _ := setupBuiltinAgentDisplayNameMigration(t)
	opening := fmt.Sprintf(legacyOnboardingZH, "海卫七", "Mika")
	seedBuiltinOnboardingMigration(t, conn, "小阿孚", "mika", opening, onboardingMigrationKickoff(opening, "海卫七"))
	mustExecOnboardingMigration(t, conn, `
		CREATE FUNCTION reject_kickoff_update() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN RAISE EXCEPTION 'simulated kickoff write failure'; END $$;
		CREATE TRIGGER reject_kickoff_update BEFORE UPDATE OF content ON chat_message
		FOR EACH ROW WHEN (OLD.message_kind = 'onboarding_kickoff') EXECUTE FUNCTION reject_kickoff_update();
	`)
	before := snapshotBuiltinAgentMigrationTables(t, conn, "chat_message")
	_, err := conn.Exec(ctx, readMigrationFile(t, "458_builtin_onboarding_display_name.up.sql"))
	if err == nil || !strings.Contains(err.Error(), "simulated kickoff write failure") {
		t.Fatalf("expected injected write failure, got %v", err)
	}
	if after := snapshotBuiltinAgentMigrationTables(t, conn, "chat_message"); !reflect.DeepEqual(after, before) {
		t.Fatal("failed kickoff write left a partially renamed opening")
	}
}

func onboardingMigrationKickoff(opening, workspace string) string {
	return "Product context\n\n<opening-already-sent>\n" + opening + "\n</opening-already-sent>\n\nWorkspace: " + workspace + "\nMember profile: Mika manages our office."
}

func seedBuiltinOnboardingMigration(t *testing.T, conn *pgx.Conn, name, systemKey, opening, kickoff string) (sessionID, openingID, kickoffID string) {
	t.Helper()
	agentID, workspaceID := uuid.NewString(), uuid.NewString()
	sessionID, openingID, kickoffID = uuid.NewString(), uuid.NewString(), uuid.NewString()
	before := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	mustExecOnboardingMigration(t, conn, `INSERT INTO agent (id, workspace_id, name, system_key, description, instructions, created_at, updated_at) VALUES ($1, $2, $3, $4, 'Owner Mika description', 'Owner Mika instructions', $5, $5)`, agentID, workspaceID, name, systemKey, before)
	mustExecOnboardingMigration(t, conn, `INSERT INTO chat_session (id, workspace_id, agent_id, title, created_at, updated_at) VALUES ($1, $2, $3, 'Owner conversation', $4, $4)`, sessionID, workspaceID, agentID, before)
	mustExecOnboardingMigration(t, conn, `INSERT INTO chat_message (id, chat_session_id, role, content, message_kind, created_at) VALUES ($1, $2, 'assistant', $3, 'onboarding_opening', $4)`, openingID, sessionID, opening, before)
	mustExecOnboardingMigration(t, conn, `INSERT INTO chat_message (id, chat_session_id, role, content, message_kind, created_at) VALUES ($1, $2, 'user', $3, 'onboarding_kickoff', $4)`, kickoffID, sessionID, kickoff, before.Add(time.Microsecond))
	return
}

func mustExecOnboardingMigration(t *testing.T, conn *pgx.Conn, query string, args ...any) {
	t.Helper()
	if _, err := conn.Exec(context.Background(), query, args...); err != nil {
		t.Fatal(err)
	}
}
