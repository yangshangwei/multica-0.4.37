package migrations

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/testutil"
)

func collaborationLeaderTurnMigration(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "migrations", "725_collaboration_leader_turn.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func collaborationLeaderTurnHistory(t *testing.T, settled, siblings bool) (*testutil.Fixture, string, []string) {
	t.Helper()
	f := newCollaborationChannelMigrationFixture(t)
	f.Exec(t, "ALTER TABLE collaboration_execution_group DROP COLUMN IF EXISTS next_leader_turn")
	f.Exec(t, "ALTER TABLE collaboration_run DROP COLUMN IF EXISTS leader_turn")
	session, target, command, channel, batch := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	cols := testutil.Cols{"session_id": session, "target_id": target, "command_id": command, "channel_id": channel, "batch_id": batch,
		"generation": 1, "target_epoch": 1, "config_version": 1, "roster": "[]"}
	if settled {
		cols["settled_at"], cols["expansion_sealed_at"], cols["state"] = testutil.Raw("now()"), testutil.Raw("now()"), "failed"
	}
	group := f.Insert(t, "collaboration_execution_group", cols)
	var runs []string
	for i, kind := range []string{"leader", "continuation", "continuation"} {
		row := testutil.Cols{"session_id": session, "target_id": target, "group_id": group, "command_id": command,
			"channel_id": channel, "batch_id": batch, "step_key": uuid.NewString(), "step_kind": kind,
			"agent_id": uuid.NewString(), "target_epoch": 1, "config_version": 1, "grant_id": uuid.NewString(), "grant_epoch": 1,
			"resource_epoch": 1, "execution_workspace_id": uuid.NewString(), "runtime_id": uuid.NewString(),
			"installation_id": uuid.NewString(), "binding_id": uuid.NewString(), "binding_epoch": 1}
		if i > 0 {
			row["parent_run_id"] = runs[i-1]
			if siblings {
				row["parent_run_id"] = runs[0]
			}
		}
		runs = append(runs, f.Insert(t, "collaboration_run", row))
	}
	return f, group, runs
}

func TestCollaborationLeaderTurnMigrationUsesCausalityAndPreservesSettledHistory(t *testing.T) {
	for _, settled := range []bool{false, true} {
		t.Run(map[bool]string{false: "active_causal_chain", true: "settled_ambiguous_history"}[settled], func(t *testing.T) {
			f, group, runs := collaborationLeaderTurnHistory(t, settled, settled)
			f.Exec(t, collaborationLeaderTurnMigration(t))
			if settled {
				if f.Count(t, `SELECT count(*) FROM collaboration_run WHERE group_id=$1 AND leader_turn IS NULL`, group) != 3 ||
					f.Count(t, `SELECT count(*) FROM collaboration_execution_group WHERE id=$1 AND state='failed' AND settled_at IS NOT NULL`, group) != 1 {
					t.Fatal("migration invented ordering or rewrote settled history")
				}
				return
			}
			for i, run := range runs {
				if f.Count(t, `SELECT count(*) FROM collaboration_run WHERE id=$1 AND leader_turn=$2`, run, i+1) != 1 {
					t.Fatal("migration did not preserve the unique causal leader order")
				}
			}
			if f.Count(t, `SELECT count(*) FROM collaboration_execution_group WHERE id=$1 AND next_leader_turn=4`, group) != 1 {
				t.Fatal("next turn could reuse an existing leader identity")
			}
			if _, err := f.Pool.Exec(t.Context(), `UPDATE collaboration_run SET leader_turn=99 WHERE id=$1`, runs[2]); err == nil {
				t.Fatal("leader turn identity remained mutable")
			}
		})
	}
}

func TestCollaborationLeaderTurnMigrationRefusesAmbiguousActiveHistory(t *testing.T) {
	f, group, _ := collaborationLeaderTurnHistory(t, false, true)
	conn, err := f.Pool.Acquire(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Release()
	_, err = conn.Exec(t.Context(), collaborationLeaderTurnMigration(t))
	if err == nil || !strings.Contains(err.Error(), "leader order is ambiguous") {
		t.Fatalf("ambiguous active order was guessed from timestamps: %v", err)
	}
	if _, err = conn.Exec(t.Context(), "ROLLBACK"); err != nil {
		t.Fatal(err)
	}
	var count int
	if err = conn.QueryRow(t.Context(), `SELECT count(*) FROM collaboration_run WHERE group_id=$1`, group).Scan(&count); err != nil || count != 3 {
		t.Fatalf("refused migration altered retained runs: count=%d err=%v", count, err)
	}
}
