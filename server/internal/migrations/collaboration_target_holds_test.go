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

func newCollaborationHoldMigrationFixture(t *testing.T) *testutil.Fixture {
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
	t.Cleanup(func() { _ = base.Close(ctx) })
	schema := "collaboration_holds_" + strings.ReplaceAll(uuid.NewString(), "-", "")
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
	for _, table := range []string{"collaboration_target", "collaboration_channel", "collaboration_execution_group", "collaboration_run", "collaboration_delegation"} {
		fx.Exec(t, "CREATE TABLE "+pgx.Identifier{table}.Sanitize()+" (LIKE public."+pgx.Identifier{table}.Sanitize()+" INCLUDING DEFAULTS INCLUDING CONSTRAINTS)")
	}
	fx.Exec(t, "ALTER TABLE collaboration_target DROP COLUMN IF EXISTS hold_reason, DROP COLUMN IF EXISTS hold_version")
	return fx
}

func TestCollaborationTargetHoldMigrationPreservesIntentAndProof(t *testing.T) {
	ctx := context.Background()
	fx := newCollaborationHoldMigrationFixture(t)
	pool := fx.Pool
	session, group := uuid.NewString(), uuid.NewString()
	var retained []string
	scenarios := []struct{ state, pause, want string }{
		{"active", "", ""}, {"paused", "new_target", "initial"},
		{"paused", "participant_stop", "initial"}, {"paused", "owner_revoked", "grant_required"},
		{"paused", "configuration_changed", "switch_hold"}, {"paused", "target_configuration_changed", "config_invalid"},
	}
	for _, scenario := range scenarios {
		id := fx.Insert(t, "collaboration_target", testutil.Cols{"session_id": session, "kind": "agent", "resource_id": uuid.NewString(), "roster": "[]",
			"queue_state": scenario.state, "pause_reason": scenario.pause, "active_group_id": group, "group_generation": 7, "control_version": 11})
		retained = append(retained, id)
	}
	var before string
	fx.QueryRow(t, `SELECT jsonb_agg(to_jsonb(t) ORDER BY id)::text FROM collaboration_target t`).Scan(&before)
	up, err := os.ReadFile(filepath.Join("..", "..", "migrations", "698_collaboration_target_holds.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, string(up)); err != nil {
		t.Fatal(err)
	}
	var preserved string
	fx.QueryRow(t, `SELECT jsonb_agg(to_jsonb(t)-'hold_reason'-'hold_version' ORDER BY id)::text FROM collaboration_target t`).Scan(&preserved)
	if before != preserved {
		t.Fatal("hold backfill changed existing lane mirrors, input identity, or active proof pointers")
	}
	for i, scenario := range scenarios {
		want := scenario.want
		if fx.Count(t, `SELECT count(*) FROM collaboration_target WHERE id=$1 AND COALESCE(hold_reason,'')=$2 AND hold_version=1`, retained[i], want) != 1 {
			t.Fatalf("backfill did not preserve active intent or classify the retained hold: row %d want %q", i, want)
		}
	}
	fresh := fx.Insert(t, "collaboration_target", testutil.Cols{"session_id": session, "kind": "agent", "resource_id": uuid.NewString(), "roster": "[]"})
	if fx.Count(t, `SELECT count(*) FROM collaboration_target WHERE id=$1 AND hold_reason='initial' AND hold_version=1`, fresh) != 1 {
		t.Fatal("new targets did not default to a positive initial hold")
	}
	fx.Exec(t, `UPDATE collaboration_target SET hold_reason=NULL,hold_version=9007199254740995 WHERE id=$1`, fresh)
	fx.QueryRow(t, `SELECT jsonb_agg(to_jsonb(t) ORDER BY id)::text FROM collaboration_target t`).Scan(&before)
	if _, err = pool.Exec(ctx, string(up)); err != nil {
		t.Fatal(err)
	}
	fx.QueryRow(t, `SELECT jsonb_agg(to_jsonb(t) ORDER BY id)::text FROM collaboration_target t`).Scan(&preserved)
	if before != preserved {
		t.Fatal("replaying the migration rewrote explicit hold releases or hold versions")
	}
	for _, mutation := range []string{
		`UPDATE collaboration_target SET hold_reason='future_hold' WHERE id=$1`,
		`UPDATE collaboration_target SET hold_version=0 WHERE id=$1`,
		`UPDATE collaboration_target SET hold_version=NULL WHERE id=$1`,
	} {
		if _, err = pool.Exec(ctx, mutation, fresh); err == nil {
			t.Fatalf("invalid hold state accepted: %s", mutation)
		}
	}
	down, err := os.ReadFile(filepath.Join("..", "..", "migrations", "698_collaboration_target_holds.down.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, string(down)); err == nil || !strings.Contains(err.Error(), "retained collaboration target hold state") {
		t.Fatalf("populated down removed a retained hold observation: %v", err)
	}
	fx.Exec(t, "DELETE FROM collaboration_target")
	holder, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer holder.Rollback(ctx)
	if _, err = holder.Exec(ctx, `INSERT INTO collaboration_target (session_id,kind,resource_id,roster) VALUES ($1,'agent',$2,'[]')`, session, uuid.NewString()); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, string(down)); err == nil || !strings.Contains(err.Error(), "55P03") {
		t.Fatalf("down migration waited through a concurrent hold writer: %v", err)
	}
	if err = holder.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, string(down)); err != nil {
		t.Fatalf("empty maintenance down failed: %v", err)
	}
	if fx.Count(t, `SELECT count(*) FROM information_schema.columns WHERE table_schema=current_schema() AND table_name='collaboration_target' AND column_name IN ('hold_reason','hold_version')`) != 0 {
		t.Fatal("empty maintenance down kept hold columns")
	}
}

func TestCollaborationTargetHoldMigrationRepairsOnlyLegacyReconfigure(t *testing.T) {
	for _, scenario := range []struct {
		name    string
		repair  bool
		history bool
	}{
		{"idle_active", true, false}, {"idle_paused", true, false},
		{"settled_history", true, true}, {"repeated_history", true, true},
		{"active_pointer", false, false}, {"generation_mismatch", false, false},
		{"impossible_control_delta", false, false}, {"other_pause", false, false},
		{"unadvanced_configuration", false, false}, {"missing_generation_history", false, false},
		{"unsettled_group", false, true}, {"unsettled_run", false, true},
		{"mismatched_run", false, true}, {"orphan_delegation", false, true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			ctx := context.Background()
			fx := newCollaborationHoldMigrationFixture(t)
			pool := fx.Pool
			session, agent, command, batch := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
			generation := 0
			if scenario.history || scenario.name == "missing_generation_history" {
				generation = 1
			}
			channelCols := testutil.Cols{"session_id": session, "root_agent_id": agent, "queue_state": "active", "pause_reason": "", "control_version": 9, "group_generation": generation}
			if scenario.name == "idle_paused" {
				channelCols["queue_state"], channelCols["pause_reason"] = "paused", "participant_stop"
			}
			channel := fx.Insert(t, "collaboration_channel", channelCols)
			targetCols := testutil.Cols{"session_id": session, "kind": "agent", "resource_id": agent, "roster": "[]", "channel_id": channel,
				"target_epoch": 2, "config_version": 2, "queue_state": "paused", "pause_reason": "configuration_changed", "control_version": 10, "group_generation": generation}
			switch scenario.name {
			case "repeated_history":
				targetCols["target_epoch"], targetCols["config_version"], targetCols["control_version"] = 4, 4, 12
			case "active_pointer":
				targetCols["active_group_id"] = uuid.NewString()
			case "generation_mismatch":
				targetCols["group_generation"] = generation + 1
			case "impossible_control_delta":
				targetCols["control_version"] = 11
			case "other_pause":
				targetCols["pause_reason"] = "participant_stop"
			case "unadvanced_configuration":
				targetCols["target_epoch"], targetCols["config_version"] = 1, 1
			}
			target := fx.Insert(t, "collaboration_target", targetCols)
			if scenario.history {
				groupCols := testutil.Cols{"session_id": session, "target_id": target, "command_id": command, "channel_id": channel, "batch_id": batch,
					"generation": generation, "target_epoch": 1, "config_version": 1, "roster": "[]", "state": "stopped", "forced_disposition": "stopped",
					"expansion_sealed_at": testutil.Raw("now()"), "settled_at": testutil.Raw("now()")}
				if scenario.name == "unsettled_group" {
					groupCols["state"], groupCols["settled_at"] = "ending", nil
				}
				group := fx.Insert(t, "collaboration_execution_group", groupCols)
				runCols := testutil.Cols{"session_id": session, "target_id": target, "command_id": command, "group_id": group, "channel_id": channel, "batch_id": batch,
					"step_key": "root", "step_kind": "agent", "agent_id": agent, "target_epoch": 1, "config_version": 1, "grant_id": uuid.NewString(), "grant_epoch": 1, "resource_epoch": 1,
					"execution_workspace_id": uuid.NewString(), "runtime_id": uuid.NewString(), "installation_id": uuid.NewString(), "binding_id": uuid.NewString(), "binding_epoch": 1,
					"forced_disposition": "stopped", "proof_state": "never_launched_proven", "authorized_attempt_seq": 0, "sealed_attempt_seq": 0, "progress_state": "settled", "settled_at": testutil.Raw("now()")}
				if scenario.name == "unsettled_run" {
					runCols["proof_state"], runCols["progress_state"], runCols["settled_at"] = "unknown", "ending", nil
				}
				if scenario.name == "mismatched_run" {
					runCols["command_id"] = uuid.NewString()
				}
				fx.Insert(t, "collaboration_run", runCols)
				if scenario.name == "orphan_delegation" {
					fx.Insert(t, "collaboration_delegation", testutil.Cols{"group_id": group, "parent_run_id": uuid.NewString(), "request_key": "retained", "payload_hash": "retained",
						"phase_key": "retained", "members": "[]", "continuation_key": "retained", "state": "cancelled"})
				}
			}
			const ledgerQuery = `SELECT jsonb_build_object(
				'channels',(SELECT jsonb_agg(to_jsonb(c) ORDER BY id) FROM collaboration_channel c),
				'groups',(SELECT jsonb_agg(to_jsonb(g) ORDER BY id) FROM collaboration_execution_group g),
				'runs',(SELECT jsonb_agg(to_jsonb(r) ORDER BY id) FROM collaboration_run r),
				'delegations',(SELECT jsonb_agg(to_jsonb(d) ORDER BY id) FROM collaboration_delegation d))::text`
			var before, ledgerBefore string
			fx.QueryRow(t, `SELECT to_jsonb(t)::text FROM collaboration_target t WHERE id=$1`, target).Scan(&before)
			fx.QueryRow(t, ledgerQuery).Scan(&ledgerBefore)
			up, err := os.ReadFile(filepath.Join("..", "..", "migrations", "698_collaboration_target_holds.up.sql"))
			if err != nil {
				t.Fatal(err)
			}
			if _, err = pool.Exec(ctx, string(up)); err != nil {
				t.Fatal(err)
			}
			var after, ledgerAfter string
			fx.QueryRow(t, `SELECT (to_jsonb(t)-'hold_reason'-'hold_version')::text FROM collaboration_target t WHERE id=$1`, target).Scan(&after)
			fx.QueryRow(t, ledgerQuery).Scan(&ledgerAfter)
			if ledgerBefore != ledgerAfter {
				t.Fatal("legacy mirror repair changed channel intent or retained execution/proof identity")
			}
			if scenario.repair {
				if fx.Count(t, `SELECT count(*) FROM collaboration_target t JOIN collaboration_channel c ON c.id=t.channel_id
					WHERE t.id=$1 AND t.queue_state=c.queue_state AND t.control_version=c.control_version AND t.pause_reason=c.pause_reason
					AND t.group_generation=c.group_generation AND t.active_group_id IS NULL AND t.hold_reason='switch_hold' AND t.hold_version=1`, target) != 1 {
					t.Fatal("known pre-698 reconfigure footprint remains unreadable or lost its switch hold")
				}
				// Rehearse recovery for a database that applied the initial 698
				// hold backfill before its narrow legacy mirror repair was added.
				fx.Exec(t, `UPDATE collaboration_target SET queue_state='paused',pause_reason='configuration_changed',control_version=$2,hold_version=9007199254740995 WHERE id=$1`, target, targetCols["control_version"])
				if _, err = pool.Exec(ctx, string(up)); err != nil {
					t.Fatal(err)
				}
				if fx.Count(t, `SELECT count(*) FROM collaboration_target t JOIN collaboration_channel c ON c.id=t.channel_id
					WHERE t.id=$1 AND t.queue_state=c.queue_state AND t.control_version=c.control_version AND t.pause_reason=c.pause_reason
					AND t.hold_reason='switch_hold' AND t.hold_version=9007199254740995`, target) != 1 {
					t.Fatal("legacy repair cleared or reset a previously persisted switch hold")
				}
			} else if before != after {
				t.Fatal("migration guessed control state for an unrecognized or unproven ledger")
			}
			fx.QueryRow(t, `SELECT to_jsonb(t)::text FROM collaboration_target t WHERE id=$1`, target).Scan(&before)
			if _, err = pool.Exec(ctx, string(up)); err != nil {
				t.Fatal(err)
			}
			fx.QueryRow(t, `SELECT to_jsonb(t)::text FROM collaboration_target t WHERE id=$1`, target).Scan(&after)
			if before != after {
				t.Fatal("replaying legacy repair rewrote target state")
			}
		})
	}
}
