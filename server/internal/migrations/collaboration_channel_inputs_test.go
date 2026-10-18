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

func newCollaborationChannelMigrationFixture(t *testing.T) *testutil.Fixture {
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
	schema := "collaboration_channel_inputs_" + strings.ReplaceAll(uuid.NewString(), "-", "")
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
	for _, table := range []string{"collaboration_target", "collaboration_channel", "collaboration_batch", "collaboration_command", "collaboration_execution_group", "collaboration_run", "collaboration_session", "issue_collaboration", "issue_participant", "collaboration_account_fence", "user"} {
		name := pgx.Identifier{table}.Sanitize()
		fx.Exec(t, "CREATE TABLE "+name+" (LIKE public."+name+" INCLUDING DEFAULTS INCLUDING CONSTRAINTS)")
	}
	fx.Exec(t, "ALTER TABLE collaboration_target ADD COLUMN IF NOT EXISTS next_command_seq bigint NOT NULL DEFAULT 1")
	fx.Exec(t, "ALTER TABLE collaboration_channel DROP COLUMN IF EXISTS next_command_seq")
	fx.Exec(t, "ALTER TABLE collaboration_command DROP COLUMN IF EXISTS group_id")
	fx.Exec(t, `CREATE FUNCTION old_input_identity() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
		IF NEW.seq<>OLD.seq THEN RAISE EXCEPTION 'old sequence is immutable'; END IF; RETURN NEW; END; $$`)
	fx.Exec(t, `CREATE TRIGGER collaboration_command_identity BEFORE UPDATE ON collaboration_command FOR EACH ROW EXECUTE FUNCTION old_input_identity()`)
	fx.Exec(t, `CREATE TRIGGER collaboration_command_batch_identity BEFORE UPDATE ON collaboration_command FOR EACH ROW EXECUTE FUNCTION old_input_identity()`)
	fx.Exec(t, `CREATE UNIQUE INDEX idx_collaboration_command_target_sequence ON collaboration_command(target_id,seq)`)
	fx.Exec(t, `CREATE UNIQUE INDEX idx_collaboration_batch_waiting_target ON collaboration_batch(channel_id,effective_target_id) WHERE state='waiting'`)
	fx.Exec(t, `CREATE UNIQUE INDEX idx_collaboration_group_command ON collaboration_execution_group(command_id)`)
	return fx
}

func channelInputMigrationSQL(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "migrations", "699_collaboration_channel_inputs.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestCollaborationChannelInputMigrationPreservesConsumedHistory(t *testing.T) {
	fx := newCollaborationChannelMigrationFixture(t)
	ctx := context.Background()
	issue, author, rootAgent := uuid.NewString(), uuid.NewString(), uuid.NewString()
	session := fx.Insert(t, "collaboration_session", testutil.Cols{"issue_id": issue})
	channel := fx.Insert(t, "collaboration_channel", testutil.Cols{"session_id": session, "root_agent_id": rootAgent, "control_version": 7, "group_generation": 1})
	target := fx.Insert(t, "collaboration_target", testutil.Cols{"session_id": session, "kind": "agent", "resource_id": rootAgent, "roster": "[]", "channel_id": channel, "next_command_seq": 5})
	squad := fx.Insert(t, "collaboration_target", testutil.Cols{"session_id": session, "kind": "squad", "resource_id": uuid.NewString(), "roster": "[]", "channel_id": channel, "next_command_seq": 2})
	batch := fx.Insert(t, "collaboration_batch", testutil.Cols{"session_id": session, "channel_id": channel, "effective_target_id": target, "seq": 1, "state": "in_flight", "started_at": testutil.Raw("now()")})
	waiting := fx.Insert(t, "collaboration_batch", testutil.Cols{"session_id": session, "channel_id": channel, "effective_target_id": squad, "seq": 2})
	input := func(targetID, batchID string, seq int, state, reason string) string {
		return fx.Insert(t, "collaboration_command", testutil.Cols{"session_id": session, "target_id": targetID, "input_event_id": uuid.NewString(), "channel_id": channel, "batch_id": batchID,
			"seq": seq, "author_user_id": author, "participant_epoch": 1, "author_auth_epoch": 1, "lifecycle_epoch": 1, "content": "retained original text", "state": state, "reason": reason})
	}
	cancelled := input(target, batch, 1, "cancelled", "cancelled_by_participant")
	root := input(target, batch, 2, "in_flight", "")
	other := input(target, batch, 3, "in_flight", "")
	next := input(target, waiting, 4, "waiting", "")
	held := input(squad, waiting, 1, "waiting", "")
	group := fx.Insert(t, "collaboration_execution_group", testutil.Cols{"session_id": session, "target_id": target, "command_id": root, "channel_id": channel, "batch_id": batch,
		"generation": 1, "target_epoch": 1, "config_version": 1, "roster": "[]"})
	run := fx.Insert(t, "collaboration_run", testutil.Cols{"session_id": session, "command_id": root, "target_id": target, "group_id": group, "channel_id": channel, "batch_id": batch,
		"step_key": "root", "step_kind": "agent", "agent_id": rootAgent, "target_epoch": 1, "config_version": 1, "grant_id": uuid.NewString(), "grant_epoch": 1, "resource_epoch": 1,
		"execution_workspace_id": uuid.NewString(), "runtime_id": uuid.NewString(), "installation_id": uuid.NewString(), "binding_id": uuid.NewString(), "binding_epoch": 1,
		"authorized_attempt_seq": 1, "progress_state": "ending", "proof_state": "unknown"})
	fx.Exec(t, `UPDATE collaboration_batch SET root_command_id=$2,group_id=$3 WHERE id=$1`, batch, root, group)
	fx.Exec(t, `UPDATE collaboration_channel SET active_group_id=$2 WHERE id=$1`, channel, group)
	var before, after string
	fx.QueryRow(t, `SELECT jsonb_build_object('run',(SELECT to_jsonb(r) FROM collaboration_run r WHERE id=$1),
		'group',(SELECT to_jsonb(g) FROM collaboration_execution_group g WHERE id=$2),
		'inputs',(SELECT jsonb_agg(to_jsonb(c)-'seq' ORDER BY id) FROM collaboration_command c))::text`, run, group).Scan(&before)
	fx.Exec(t, channelInputMigrationSQL(t))
	fx.QueryRow(t, `SELECT jsonb_build_object('run',(SELECT to_jsonb(r) FROM collaboration_run r WHERE id=$1),
		'group',(SELECT to_jsonb(g) FROM collaboration_execution_group g WHERE id=$2),
		'inputs',(SELECT jsonb_agg(to_jsonb(c)-'seq'-'group_id' ORDER BY id) FROM collaboration_command c))::text`, run, group).Scan(&after)
	if before != after {
		t.Fatal("channel upgrade rewrote source inputs or historical execution/proof identities")
	}
	if fx.Count(t, `SELECT count(*) FROM collaboration_group_input WHERE group_id=$1 AND consuming_run_id=$2 AND command_id=ANY($3::uuid[])`, group, run, []string{root, other}) != 2 ||
		fx.Count(t, `SELECT count(*) FROM collaboration_group_input WHERE command_id=$1`, cancelled) != 0 {
		t.Fatal("upgrade did not distinguish consumed input from a cancelled waiting tombstone")
	}
	if fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=ANY($1::uuid[]) AND group_id IS NULL`, []string{cancelled, next, held}) != 3 ||
		fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE group_id=$1`, group) != 2 {
		t.Fatal("upgrade assigned a waiting or cancelled input to a running group")
	}
	if fx.Count(t, `SELECT count(DISTINCT seq) FROM collaboration_command WHERE channel_id=$1`, channel) != 5 ||
		fx.Count(t, `SELECT count(*) FROM collaboration_channel WHERE id=$1 AND next_command_seq=6 AND control_version=7 AND active_group_id=$2`, channel, group) != 1 {
		t.Fatal("channel sequence allocation lost uniqueness, control intent or active proof barrier")
	}
	var order []string
	fx.QueryRow(t, `SELECT array_agg(id::text ORDER BY seq) FROM collaboration_command WHERE target_id=$1`, target).Scan(&order)
	if strings.Join(order, ",") != strings.Join([]string{cancelled, root, other, next}, ",") {
		t.Fatal("upgrade reversed an existing target's authoritative input order")
	}
	newBatch := fx.Insert(t, "collaboration_batch", testutil.Cols{"session_id": session, "channel_id": channel, "effective_target_id": target, "seq": 3})
	fx.Exec(t, `UPDATE collaboration_command SET batch_id=$2 WHERE id=$1`, next, newBatch)
	for _, mutation := range []struct {
		sql  string
		args []any
	}{
		{`UPDATE collaboration_command SET content='rewritten' WHERE id=$1`, []any{next}},
		{`UPDATE collaboration_command SET seq=seq+100 WHERE id=$1`, []any{next}},
		{`UPDATE collaboration_command SET state='waiting',group_id=NULL,batch_id=$2 WHERE id=$1`, []any{root, newBatch}},
		{`DELETE FROM collaboration_group_input WHERE command_id=$1`, []any{root}},
		{`UPDATE collaboration_group_input SET seq=seq+1 WHERE command_id=$1`, []any{root}},
	} {
		if _, err := fx.Pool.Exec(ctx, mutation.sql, mutation.args...); err == nil {
			t.Fatalf("unsafe historical input mutation accepted: %s", mutation.sql)
		}
	}
	fx.QueryRow(t, `SELECT jsonb_build_object('inputs',(SELECT jsonb_agg(to_jsonb(c) ORDER BY id) FROM collaboration_command c),
		'history',(SELECT jsonb_agg(to_jsonb(i) ORDER BY command_id) FROM collaboration_group_input i),
		'channels',(SELECT jsonb_agg(to_jsonb(ch) ORDER BY id) FROM collaboration_channel ch))::text`).Scan(&before)
	fx.Exec(t, channelInputMigrationSQL(t))
	fx.QueryRow(t, `SELECT jsonb_build_object('inputs',(SELECT jsonb_agg(to_jsonb(c) ORDER BY id) FROM collaboration_command c),
		'history',(SELECT jsonb_agg(to_jsonb(i) ORDER BY command_id) FROM collaboration_group_input i),
		'channels',(SELECT jsonb_agg(to_jsonb(ch) ORDER BY id) FROM collaboration_channel ch))::text`).Scan(&after)
	if before != after {
		t.Fatal("replaying a committed migration rewrote queue placement or historical consumption")
	}
}

func TestCollaborationChannelInputMigrationRefusesMissingConsumptionEvidence(t *testing.T) {
	fx := newCollaborationChannelMigrationFixture(t)
	session, channel, target, batch := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	fx.Insert(t, "collaboration_execution_group", testutil.Cols{"session_id": session, "target_id": target, "command_id": uuid.NewString(), "channel_id": channel,
		"batch_id": batch, "generation": 1, "target_epoch": 1, "config_version": 1, "roster": "[]"})
	conn, err := fx.Pool.Acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Release()
	if _, err = conn.Exec(context.Background(), channelInputMigrationSQL(t)); err == nil || !strings.Contains(err.Error(), "unique consuming root") {
		t.Fatalf("upgrade guessed consumption without a root run: %v", err)
	}
	if _, err = conn.Exec(context.Background(), "ROLLBACK"); err != nil {
		t.Fatal(err)
	}
	if fx.Count(t, `SELECT count(*) FROM information_schema.columns WHERE table_schema=current_schema() AND table_name='collaboration_command' AND column_name='group_id'`) != 0 {
		t.Fatal("failed upgrade left a partial new association schema")
	}
}

func TestCollaborationChannelInputMigrationRefusesAmbiguousCancellation(t *testing.T) {
	fx := newCollaborationChannelMigrationFixture(t)
	session, channel, target, batch := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	fx.Insert(t, "collaboration_execution_group", testutil.Cols{"session_id": session, "target_id": target, "command_id": uuid.NewString(), "channel_id": channel,
		"batch_id": batch, "generation": 1, "target_epoch": 1, "config_version": 1, "roster": "[]", "state": "cancelled", "expansion_sealed_at": testutil.Raw("now()"), "settled_at": testutil.Raw("now()")})
	fx.Insert(t, "collaboration_command", testutil.Cols{"session_id": session, "target_id": target, "input_event_id": uuid.NewString(), "channel_id": channel, "batch_id": batch,
		"seq": 1, "author_user_id": uuid.NewString(), "participant_epoch": 1, "author_auth_epoch": 1, "lifecycle_epoch": 1, "content": "retain ambiguous history", "state": "cancelled", "reason": "participant_removed"})
	conn, err := fx.Pool.Acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Release()
	if _, err = conn.Exec(context.Background(), channelInputMigrationSQL(t)); err == nil || !strings.Contains(err.Error(), "ambiguous retained collaboration input") {
		t.Fatalf("upgrade invented consumption for an ambiguous cancelled input: %v", err)
	}
	if _, err = conn.Exec(context.Background(), "ROLLBACK"); err != nil {
		t.Fatal(err)
	}
	if fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE content='retain ambiguous history'`) != 1 {
		t.Fatal("refused upgrade removed the history needed for reconciliation")
	}
}

func TestCollaborationChannelInputMigrationDoesNotWaitThroughWriter(t *testing.T) {
	fx := newCollaborationChannelMigrationFixture(t)
	ctx := context.Background()
	holder, err := fx.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer holder.Rollback(ctx)
	if _, err = holder.Exec(ctx, `SELECT id FROM collaboration_command FOR UPDATE`); err != nil {
		t.Fatal(err)
	}
	conn, err := fx.Pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Release()
	if _, err = conn.Exec(ctx, channelInputMigrationSQL(t)); err == nil || !strings.Contains(err.Error(), "55P03") {
		t.Fatalf("migration did not refuse the concurrent source writer: %v", err)
	}
	if _, err = conn.Exec(ctx, "ROLLBACK"); err != nil {
		t.Fatal(err)
	}
}
