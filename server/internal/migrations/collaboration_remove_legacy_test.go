package migrations

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/multica-ai/multica/server/internal/testutil"
)

func TestCollaborationRemoveLegacyExecutionPreservesCurrentState(t *testing.T) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("requires task-owned DATABASE_URL")
	}
	ctx := context.Background()
	base, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = base.Close(ctx) })
	schema := "collaboration_remove_legacy_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	quoted := pgx.Identifier{schema}.Sanitize()
	if _, err = base.Exec(ctx, "CREATE SCHEMA "+quoted); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = base.Exec(ctx, "DROP SCHEMA "+quoted+" CASCADE") })
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
	fx.Exec(t, "CREATE TABLE comment (id uuid, revision bigint NOT NULL DEFAULT 1)")
	paths, err := filepath.Glob(filepath.Join("..", "..", "migrations", "*.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	apply := func(first, last string) {
		t.Helper()
		for _, path := range paths {
			name := filepath.Base(path)
			if name < first || name >= last {
				continue
			}
			sql, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = pool.Exec(ctx, string(sql)); err != nil {
				t.Fatalf("apply %s: %v", name, err)
			}
		}
	}
	apply("571_", "620_")
	issue, actor, workspace, agent, runtime, installation, binding := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	session := fx.Insert(t, "collaboration_session", testutil.Cols{"issue_id": issue, "queue_state": "paused"})
	legacyCommand := fx.Insert(t, "collaboration_command", testutil.Cols{"session_id": session, "seq": 1, "author_user_id": actor,
		"participant_epoch": 1, "author_auth_epoch": 1, "lifecycle_epoch": 1, "content": "obsolete single-session input"})
	legacyGrant := fx.Insert(t, "collaboration_grant", testutil.Cols{"issue_id": issue, "owner_user_id": actor, "owner_auth_epoch": 1,
		"execution_workspace_id": workspace, "agent_id": agent, "runtime_id": runtime, "identity_kind": "enrolled", "installation_id": installation, "binding_id": binding, "binding_epoch": 1})
	legacyRun := fx.Insert(t, "collaboration_run", testutil.Cols{"session_id": session, "command_id": legacyCommand, "grant_id": legacyGrant,
		"grant_epoch": 1, "resource_epoch": 1, "execution_workspace_id": workspace, "runtime_id": runtime, "installation_id": installation, "binding_id": binding, "binding_epoch": 1})
	legacyEvent := fx.Insert(t, "collaboration_run_event", testutil.Cols{"issue_id": issue, "run_id": legacyRun, "producer_event_id": "legacy", "seq": 1, "kind": "output", "payload": "{}"})
	fx.InsertNoID(t, "issue_participant", testutil.Cols{"issue_id": issue, "user_id": actor}, "issue_id=$1 AND user_id=$2", issue, actor)
	fx.InsertNoID(t, "collaboration_account_fence", testutil.Cols{"user_id": actor}, "user_id=$1", actor)
	event := fx.Insert(t, "collaboration_event", testutil.Cols{"issue_id": issue, "event_seq": 1, "actor_user_id": actor, "kind": "invited"})
	notification := fx.Insert(t, "collaboration_notification", testutil.Cols{"issue_id": issue, "event_seq": 1, "recipient_user_id": actor, "kind": "invited"})
	apply("620_", "643_")
	target := fx.Insert(t, "collaboration_target", testutil.Cols{"session_id": session, "kind": "agent", "resource_id": agent, "roster": "[]"})
	command := fx.Insert(t, "collaboration_command", testutil.Cols{"session_id": session, "target_id": target, "input_event_id": uuid.NewString(),
		"seq": 1, "author_user_id": actor, "participant_epoch": 1, "author_auth_epoch": 1, "lifecycle_epoch": 1, "content": "current immutable input", "state": "in_flight"})
	grant := fx.Insert(t, "collaboration_grant", testutil.Cols{"issue_id": issue, "target_id": target, "config_version": 1, "role": "agent", "owner_user_id": actor,
		"owner_auth_epoch": 1, "execution_workspace_id": workspace, "agent_id": agent, "runtime_id": runtime, "identity_kind": "enrolled", "installation_id": installation, "binding_id": binding, "binding_epoch": 1})
	group := fx.Insert(t, "collaboration_execution_group", testutil.Cols{"session_id": session, "target_id": target, "command_id": command, "generation": 1, "target_epoch": 1, "config_version": 1, "roster": "[]"})
	nonce := uuid.NewString()
	run := fx.Insert(t, "collaboration_run", testutil.Cols{"session_id": session, "target_id": target, "group_id": group, "command_id": command,
		"step_key": "root", "step_kind": "agent", "agent_id": agent, "target_epoch": 1, "config_version": 1, "grant_id": grant, "grant_epoch": 1, "resource_epoch": 1,
		"execution_workspace_id": workspace, "runtime_id": runtime, "installation_id": installation, "binding_id": binding, "binding_epoch": 1,
		"launch_nonce": nonce, "fence": 4, "authorized_attempt_seq": 2, "progress_state": "ending", "forced_disposition": "stopped", "disposition_reason": "retained_reason"})
	fx.Exec(t, `UPDATE collaboration_target SET active_group_id=$2,group_generation=1 WHERE id=$1`, target, group)
	// Establish the destructive migration's preservation baseline before applying it.
	if fx.Count(t, `SELECT count(*) FROM collaboration_command`) != 2 || fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE proof_state='unknown'`) != 2 {
		t.Fatal("legacy/current baseline setup failed")
	}
	path := filepath.Join("..", "..", "migrations", "680_collaboration_remove_legacy_execution.up.sql")
	sql, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, string(sql)); err != nil {
		t.Fatalf("remove obsolete execution: %v", err)
	}
	if fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=$1`, legacyCommand) != 0 ||
		fx.Count(t, `SELECT count(*) FROM collaboration_grant WHERE id=$1`, legacyGrant) != 0 ||
		fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE id=$1`, legacyRun) != 0 ||
		fx.Count(t, `SELECT count(*) FROM collaboration_run_event WHERE id=$1`, legacyEvent) != 0 {
		t.Fatal("obsolete execution survived the explicit removal")
	}
	if fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=$1 AND content='current immutable input'`, command) != 1 ||
		fx.Count(t, `SELECT count(*) FROM collaboration_grant WHERE id=$1`, grant) != 1 ||
		fx.Count(t, `SELECT count(*) FROM collaboration_execution_group WHERE id=$1 AND settled_at IS NULL`, group) != 1 ||
		fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE id=$1 AND launch_nonce=$2 AND fence=4 AND authorized_attempt_seq=2 AND proof_state='unknown' AND settled_at IS NULL AND disposition_reason='retained_reason'`, run, nonce) != 1 ||
		fx.Count(t, `SELECT count(*) FROM issue_participant WHERE issue_id=$1 AND user_id=$2`, issue, actor) != 1 ||
		fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE id=$1`, event) != 1 ||
		fx.Count(t, `SELECT count(*) FROM collaboration_notification WHERE id=$1`, notification) != 1 ||
		fx.Count(t, `SELECT count(*) FROM collaboration_account_fence WHERE user_id=$1 AND epoch=1`, actor) != 1 {
		t.Fatal("removing legacy execution changed current proof, immutable input or human/account history")
	}
	if fx.Count(t, `SELECT count(*) FROM information_schema.columns WHERE table_schema=current_schema() AND column_name IN ('legacy_unresolved','legacy_session_id','legacy_command_id','active_run_id','active_grant_id')`) != 0 {
		t.Fatal("obsolete execution mapping columns remain")
	}
}
