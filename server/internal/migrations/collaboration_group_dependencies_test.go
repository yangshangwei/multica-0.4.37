package migrations

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/testutil"
)

func groupDependencyMigrationSQL(t *testing.T) string {
	t.Helper()
	s, err := os.ReadFile(filepath.Join("..", "..", "migrations", "717_collaboration_group_dependencies.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	return string(s)
}

func dependencyMigrationFixture(t *testing.T, missing bool) (*testutil.Fixture, string, string) {
	t.Helper()
	f := newCollaborationChannelMigrationFixture(t)
	f.Exec(t, channelInputMigrationSQL(t))
	f.Exec(t, "CREATE TABLE collaboration_grant (LIKE public.collaboration_grant INCLUDING DEFAULTS INCLUDING CONSTRAINTS)")
	f.Exec(t, "ALTER TABLE collaboration_run DROP COLUMN prompt_snapshot")
	issue, owner, agent, runtime, workspace, install, binding := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	session := f.Insert(t, "collaboration_session", testutil.Cols{"issue_id": issue})
	channel := f.Insert(t, "collaboration_channel", testutil.Cols{"session_id": session, "root_agent_id": agent})
	target := f.Insert(t, "collaboration_target", testutil.Cols{"session_id": session, "kind": "agent", "resource_id": agent, "channel_id": channel, "roster": "[]"})
	grant := f.Insert(t, "collaboration_grant", testutil.Cols{"issue_id": issue, "target_id": target, "target_epoch": 1, "config_version": 1, "role": "agent", "owner_user_id": owner, "owner_auth_epoch": 4, "owner_participant_epoch": 3,
		"epoch": 2, "revoked_at": testutil.Raw("now()"), "configuration_fingerprint": strings.Repeat("a", 64), "execution_workspace_id": workspace, "agent_id": agent, "runtime_id": runtime, "identity_kind": "managed", "installation_id": install, "binding_id": binding, "binding_epoch": 1})
	roster := `[{"role":"agent","agent_id":"` + agent + `","runtime_id":"` + runtime + `","execution_workspace_id":"` + workspace + `","owner_user_id":"` + owner + `","grant_id":"` + grant + `","grant_epoch":"1"}]`
	if missing {
		roster = strings.TrimSuffix(roster, "]") + `,{"role":"member","agent_id":"` + uuid.NewString() + `","runtime_id":"` + runtime + `","execution_workspace_id":"` + workspace + `","owner_user_id":"` + owner + `","grant_id":"` + uuid.NewString() + `","grant_epoch":"1"}]`
	}
	batch := f.Insert(t, "collaboration_batch", testutil.Cols{"session_id": session, "channel_id": channel, "effective_target_id": target, "seq": 1, "state": "in_flight", "started_at": testutil.Raw("now()")})
	command := f.Insert(t, "collaboration_command", testutil.Cols{"session_id": session, "target_id": target, "input_event_id": uuid.NewString(), "channel_id": channel, "batch_id": batch, "seq": 1, "author_user_id": owner, "participant_epoch": 3, "author_auth_epoch": 4, "lifecycle_epoch": 1, "content": "original human input", "state": "waiting"})
	group := f.Insert(t, "collaboration_execution_group", testutil.Cols{"session_id": session, "target_id": target, "command_id": command, "channel_id": channel, "batch_id": batch, "generation": 1, "target_epoch": 1, "config_version": 1, "roster": roster})
	run := f.Insert(t, "collaboration_run", testutil.Cols{"session_id": session, "target_id": target, "group_id": group, "command_id": command, "channel_id": channel, "batch_id": batch, "step_key": "root", "step_kind": "agent", "agent_id": agent,
		"target_epoch": 1, "config_version": 1, "grant_id": grant, "grant_epoch": 1, "resource_epoch": 1, "execution_workspace_id": workspace, "runtime_id": runtime, "installation_id": install, "binding_id": binding, "binding_epoch": 1,
		"progress_state": "settled", "proof_state": "exited_proven", "authorized_attempt_seq": 1, "sealed_attempt_seq": 1, "settled_at": testutil.Raw("now()"), "forced_disposition": "stopped"})
	f.Exec(t, `UPDATE collaboration_batch SET group_id=$2,root_command_id=$3 WHERE id=$1`, batch, group, command)
	f.Exec(t, `UPDATE collaboration_command SET state='in_flight',group_id=$2 WHERE id=$1`, command, group)
	f.InsertNoID(t, "collaboration_group_input", testutil.Cols{"group_id": group, "command_id": command, "channel_id": channel, "batch_id": batch, "seq": 1, "consuming_run_id": run}, "group_id=$1", group)
	f.Exec(t, `UPDATE collaboration_execution_group SET state='stopped',expansion_sealed_at=now(),settled_at=now(),forced_disposition='stopped' WHERE id=$1`, group)
	return f, group, grant
}

func TestCollaborationGroupDependencyMigrationPreservesRevokedHistory(t *testing.T) {
	f, group, grant := dependencyMigrationFixture(t, false)
	f.Exec(t, groupDependencyMigrationSQL(t))
	if f.Count(t, `SELECT count(*) FROM collaboration_group_dependency WHERE group_id=$1 AND grants->0->>'grant_epoch'='1' AND grants->0->>'owner_auth_epoch'='4' AND grants->0->>'owner_participant_epoch'='3'`, group) != 1 {
		t.Fatal("revoked history substituted current grant epoch or lost immutable consent")
	}
	if f.Count(t, `SELECT count(*) FROM collaboration_grant WHERE id=$1 AND epoch=2 AND revoked_at IS NOT NULL`, grant) != 1 {
		t.Fatal("migration revived a revoked grant")
	}
}

func TestCollaborationGroupDependencyMigrationRefusesIncompleteRoster(t *testing.T) {
	f, _, _ := dependencyMigrationFixture(t, true)
	if _, err := f.Pool.Exec(t.Context(), groupDependencyMigrationSQL(t)); err == nil || !strings.Contains(err.Error(), "exact target consent") {
		t.Fatalf("missing frozen member silently dropped: %v", err)
	}
	// A multi-statement migration error leaves its explicit transaction open.
	// The fixture owns this private schema; rollback before checking table absence.
	conn, err := f.Pool.Acquire(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Release()
	_, _ = conn.Exec(t.Context(), "ROLLBACK")
	var exists bool
	if err = conn.QueryRow(t.Context(), `SELECT to_regclass(current_schema()||'.collaboration_group_dependency') IS NOT NULL`).Scan(&exists); err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Fatal("failed dependency upgrade left partial history")
	}
}
