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

func TestCollaborationTargetMigrationPreservesUnresolvedWork(t *testing.T) {
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
	schema := "collaboration_targets_" + strings.ReplaceAll(uuid.NewString(), "-", "")
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
	files, err := filepath.Glob(filepath.Join("..", "..", "migrations", "*.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	apply := func(first, last string) []string {
		t.Helper()
		var selected []string
		for _, path := range files {
			name := filepath.Base(path)
			if name < first || name >= last {
				continue
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = pool.Exec(ctx, string(data)); err != nil {
				t.Fatalf("apply %s: %v", name, err)
			}
			selected = append(selected, path)
		}
		return selected
	}
	apply("571_", "620_")
	issue, actor := uuid.NewString(), uuid.NewString()
	session := fx.Insert(t, "collaboration_session", testutil.Cols{"issue_id": issue, "queue_state": "active", "next_command_seq": 11})
	commandCols := testutil.Cols{"session_id": session, "seq": 9, "author_user_id": actor, "participant_epoch": 1, "author_auth_epoch": 1, "lifecycle_epoch": 1, "content": "legacy private input"}
	waiting := fx.Insert(t, "collaboration_command", commandCols)
	commandCols["seq"], commandCols["state"] = 10, "in_flight"
	inflight := fx.Insert(t, "collaboration_command", commandCols)
	grant := fx.Insert(t, "collaboration_grant", testutil.Cols{"issue_id": issue, "owner_user_id": actor, "owner_auth_epoch": 1,
		"execution_workspace_id": uuid.NewString(), "agent_id": uuid.NewString(), "runtime_id": uuid.NewString(),
		"identity_kind": "enrolled", "installation_id": uuid.NewString(), "binding_id": uuid.NewString(), "binding_epoch": 1})
	nonce := uuid.NewString()
	runCols := testutil.Cols{"session_id": session, "command_id": inflight, "grant_id": grant, "grant_epoch": 1, "resource_epoch": 1,
		"execution_workspace_id": uuid.NewString(), "runtime_id": uuid.NewString(), "installation_id": uuid.NewString(), "binding_id": uuid.NewString(), "binding_epoch": 1,
		"launch_nonce": nonce, "fence": 7, "claim_generation": 5, "authorized_attempt_seq": 3, "progress_state": "ending", "forced_disposition": "stopped", "disposition_reason": "original_stop"}
	run := fx.Insert(t, "collaboration_run", runCols)
	fx.Exec(t, `UPDATE collaboration_session SET active_run_id=$2, active_grant_id=$3 WHERE id=$1`, session, run, grant)
	fx.Exec(t, `UPDATE collaboration_execution_gate SET enabled=true, creation_enabled=true WHERE id=1`)
	applied := apply("620_", "643_")
	if len(applied) != 23 {
		t.Fatalf("expected target schema and 22 separate indexes, got %d", len(applied))
	}
	if fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=ANY($1::uuid[]) AND legacy_unresolved AND target_id IS NULL AND input_event_id IS NULL AND session_id=legacy_session_id AND content='legacy private input'`, []string{waiting, inflight}) != 2 ||
		fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE id=$1 AND legacy_unresolved AND group_id IS NULL AND target_id IS NULL AND launch_nonce=$2 AND fence=7 AND claim_generation=5 AND authorized_attempt_seq=3 AND proof_state='unknown' AND settled_at IS NULL AND forced_disposition='stopped' AND disposition_reason='original_stop' AND session_id=legacy_session_id AND command_id=legacy_command_id`, run, nonce) != 1 ||
		fx.Count(t, `SELECT count(*) FROM collaboration_session WHERE id=$1 AND queue_state='paused' AND active_run_id=$2 AND active_grant_id=$3 AND next_command_seq=11 AND control_version=2`, session, run, grant) != 1 ||
		fx.Count(t, `SELECT count(*) FROM collaboration_execution_gate WHERE NOT enabled AND NOT creation_enabled AND epoch=2`) != 1 {
		t.Fatal("upgrade guessed a target, lost retained identity or enabled execution")
	}
	refuse := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err == nil {
			t.Fatalf("unsafe historical write succeeded: %s", sql)
		}
	}
	refuse(`INSERT INTO collaboration_command (session_id,seq,author_user_id,participant_epoch,author_auth_epoch,lifecycle_epoch,content) VALUES ($1,11,$2,1,1,1,'old writer')`, session, actor)
	refuse(`INSERT INTO collaboration_command (session_id,legacy_session_id,legacy_unresolved,seq,author_user_id,participant_epoch,author_auth_epoch,lifecycle_epoch,content) VALUES ($1,$1,true,11,$2,1,1,1,'forged legacy')`, session, actor)
	refuse(`UPDATE collaboration_command SET state='in_flight' WHERE id=$1`, waiting)
	refuse(`UPDATE collaboration_run SET authorized_attempt_seq=4 WHERE id=$1`, run)
	refuse(`UPDATE collaboration_run SET settled_at=now() WHERE id=$1`, run)
	refuse(`INSERT INTO collaboration_run (session_id,command_id,grant_id,grant_epoch,resource_epoch,execution_workspace_id,runtime_id,installation_id,binding_id,binding_epoch) SELECT session_id,gen_random_uuid(),grant_id,grant_epoch,resource_epoch,execution_workspace_id,runtime_id,installation_id,binding_id,binding_epoch FROM collaboration_run WHERE id=$1`, run)
	fx.Exec(t, `UPDATE collaboration_command SET state='cancelled',reason='participant_removed' WHERE id=$1`, waiting)
	if fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=$1 AND seq=9 AND legacy_unresolved AND state='cancelled'`, waiting) != 1 {
		t.Fatal("legacy waiting cancellation lost history")
	}

	resource := uuid.NewString()
	a := fx.Insert(t, "collaboration_target", testutil.Cols{"session_id": session, "kind": "agent", "resource_id": resource, "roster": "[]"})
	b := fx.Insert(t, "collaboration_target", testutil.Cols{"session_id": session, "kind": "squad", "resource_id": resource, "roster": "[]"})
	refuse(`INSERT INTO collaboration_target (session_id,kind,resource_id,roster) VALUES ($1,'agent',$2,'[]')`, session, resource)
	input := uuid.NewString()
	newCommand := func(target string) string {
		t.Helper()
		return fx.Insert(t, "collaboration_command", testutil.Cols{"session_id": session, "target_id": target, "input_event_id": input, "seq": 1,
			"author_user_id": actor, "participant_epoch": 1, "author_auth_epoch": 1, "lifecycle_epoch": 1, "content": "fixed target input", "state": "in_flight"})
	}
	ca, cb := newCommand(a), newCommand(b)
	refuse(`INSERT INTO collaboration_command (session_id,target_id,input_event_id,seq,author_user_id,participant_epoch,author_auth_epoch,lifecycle_epoch,content) SELECT session_id,target_id,input_event_id,2,author_user_id,participant_epoch,author_auth_epoch,lifecycle_epoch,content FROM collaboration_command WHERE id=$1`, ca)
	refuse(`UPDATE collaboration_command SET target_id=$2 WHERE id=$1`, ca, b)
	refuse(`UPDATE collaboration_command SET content='edited comment overwrite' WHERE id=$1`, ca)
	group := func(target, command string) string {
		t.Helper()
		return fx.Insert(t, "collaboration_execution_group", testutil.Cols{"session_id": session, "target_id": target, "command_id": command, "generation": 1, "target_epoch": 1, "config_version": 1, "roster": "[]"})
	}
	ga, gb := group(a, ca), group(b, cb)
	refuse(`INSERT INTO collaboration_execution_group (session_id,target_id,command_id,generation,target_epoch,config_version,roster) VALUES ($1,$2,gen_random_uuid(),2,1,1,'[]')`, session, a)
	newRun := func(target, group, command, step, kind string, parent any) string {
		t.Helper()
		return fx.Insert(t, "collaboration_run", testutil.Cols{"session_id": session, "target_id": target, "group_id": group, "command_id": command,
			"parent_run_id": parent, "step_key": step, "step_kind": kind, "agent_id": resource, "target_epoch": 1, "config_version": 1,
			"grant_id": grant, "grant_epoch": 1, "resource_epoch": 1, "execution_workspace_id": uuid.NewString(), "runtime_id": uuid.NewString(),
			"installation_id": uuid.NewString(), "binding_id": uuid.NewString(), "binding_epoch": 1})
	}
	ra := newRun(a, ga, ca, "root", "agent", nil)
	newRun(a, ga, ca, "member", "member", ra)
	newRun(b, gb, cb, "root", "leader", nil)
	refuse(`UPDATE collaboration_run SET parent_run_id=id WHERE id=$1`, ra)
	refuse(`INSERT INTO collaboration_run (session_id,target_id,group_id,command_id,step_key,step_kind,agent_id,target_epoch,config_version,grant_id,grant_epoch,resource_epoch,execution_workspace_id,runtime_id,installation_id,binding_id,binding_epoch) SELECT session_id,target_id,group_id,command_id,step_key,step_kind,agent_id,target_epoch,config_version,grant_id,grant_epoch,resource_epoch,execution_workspace_id,runtime_id,installation_id,binding_id,binding_epoch FROM collaboration_run WHERE id=$1`, ra)
	if fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE session_id=$1 AND NOT legacy_unresolved AND settled_at IS NULL`, session) != 3 {
		t.Fatal("independent targets or descendant runs still share a global execution slot")
	}
	if fx.Count(t, `SELECT count(*) FROM pg_constraint WHERE connamespace=current_schema()::regnamespace AND contype='f'`) != 0 ||
		fx.Count(t, `SELECT count(*) FROM pg_index i JOIN pg_class c ON c.oid=i.indexrelid WHERE c.relnamespace=current_schema()::regnamespace AND NOT i.indisvalid`) != 0 {
		t.Fatal("target migration added foreign keys or invalid indexes")
	}

	for _, path := range applied {
		data, err := os.ReadFile(strings.TrimSuffix(path, ".up.sql") + ".down.sql")
		if err != nil {
			t.Fatal(err)
		}
		if _, err = pool.Exec(ctx, string(data)); err == nil || !strings.Contains(err.Error(), "retained collaboration target state") {
			t.Fatalf("populated down %s: %v", path, err)
		}
	}
	for _, table := range []string{"collaboration_termination", "collaboration_run", "collaboration_execution_group", "collaboration_delegation", "collaboration_command", "collaboration_grant", "collaboration_target", "collaboration_session"} {
		fx.Exec(t, "DELETE FROM "+pgx.Identifier{table}.Sanitize())
	}
	holder, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer holder.Rollback(ctx)
	if _, err = holder.Exec(ctx, `INSERT INTO collaboration_target (session_id,kind,resource_id,roster) VALUES ($1,'agent',$2,'[]')`, session, resource); err != nil {
		t.Fatal(err)
	}
	for _, path := range applied {
		data, err := os.ReadFile(strings.TrimSuffix(path, ".up.sql") + ".down.sql")
		if err != nil {
			t.Fatal(err)
		}
		if _, err = pool.Exec(ctx, string(data)); err == nil || !strings.Contains(err.Error(), "55P03") {
			t.Fatalf("busy-writer down %s: %v", path, err)
		}
	}
	if err = holder.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	for i := len(applied) - 1; i >= 0; i-- {
		path := strings.TrimSuffix(applied[i], ".up.sql") + ".down.sql"
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = pool.Exec(ctx, string(data)); err != nil {
			t.Fatalf("empty maintenance rollback %s: %v", path, err)
		}
	}
	if fx.Count(t, `SELECT count(*) FROM information_schema.columns WHERE table_schema=current_schema() AND column_name IN ('legacy_session_id','target_id','group_id')`) != 0 {
		t.Fatal("empty down did not restore original column identities")
	}
	if fx.Count(t, `SELECT count(*) FROM pg_indexes WHERE schemaname=current_schema() AND indexname IN ('collaboration_command_sequence_idx','collaboration_run_command_idx','collaboration_run_unsettled_idx')`) != 3 {
		t.Fatal("rollback lost the original unique indexes")
	}
}
