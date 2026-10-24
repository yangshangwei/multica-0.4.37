package migrations

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/multica-ai/multica/server/internal/testutil"
)

func cancellationMigrationSQL(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "migrations", name+".sql"))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestCollaborationCancellationMigrationBackfillsOnlyExactSingletonManualEvents(t *testing.T) {
	f := newCollaborationChannelMigrationFixture(t)
	// This fixture has its own command/session tables and old input guard.
	// Clone the event table too: no migration query may resolve to public data.
	f.Exec(t, `CREATE TABLE collaboration_event (LIKE public.collaboration_event INCLUDING DEFAULTS INCLUDING CONSTRAINTS)`)
	f.Exec(t, `ALTER TABLE collaboration_command DROP COLUMN IF EXISTS cancellation_event_id`)
	issue, actor, author := uuid.NewString(), uuid.NewString(), uuid.NewString()
	session := f.Insert(t, "collaboration_session", testutil.Cols{"issue_id": issue})
	var exactCommand, exactEvent string
	for i, scenario := range []string{"exact", "duplicate", "other_issue", "wrong_target", "wrong_sequence", "bulk", "automatic", "not_cancelled", "missing_action", "mixed_evidence"} {
		target, channel, batch := uuid.NewString(), uuid.NewString(), uuid.NewString()
		state, reason := "cancelled", "cancelled_by_participant"
		if scenario == "automatic" || scenario == "bulk" {
			reason = "participant_removed"
		}
		if scenario == "not_cancelled" {
			state = "waiting"
		}
		command := f.Insert(t, "collaboration_command", testutil.Cols{
			"session_id": session, "target_id": target, "channel_id": channel, "batch_id": batch, "input_event_id": uuid.NewString(),
			"seq": 1, "author_user_id": author, "participant_epoch": 1, "author_auth_epoch": 1, "lifecycle_epoch": 1,
			"content": "immutable " + scenario, "attachments": "[]", "state": state, "reason": reason,
		})
		payload := map[string]any{"command_id": command, "target_id": target, "channel_id": channel, "batch_id": batch, "command_sequence": "1", "action": "cancel_command", "code": "command_cancelled"}
		eventIssue, eventKind := issue, "command_cancelled"
		switch scenario {
		case "other_issue":
			eventIssue = uuid.NewString()
		case "wrong_target":
			payload["target_id"] = uuid.NewString()
		case "wrong_sequence":
			payload["command_sequence"] = "2"
		case "bulk":
			payload, eventKind = map[string]any{"cancelled_commands": 1}, "participant_removed"
		case "automatic":
			payload["action"] = "automatic"
		case "missing_action":
			delete(payload, "action")
		}
		encoded, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		cols := testutil.Cols{"issue_id": eventIssue, "event_seq": i*10 + 1, "actor_user_id": actor, "kind": eventKind, "payload": encoded}
		event := f.Insert(t, "collaboration_event", cols)
		if scenario == "duplicate" || scenario == "mixed_evidence" {
			cols["event_seq"] = i*10 + 2
			if scenario == "mixed_evidence" {
				payload["command_sequence"] = "9"
				encoded, err = json.Marshal(payload)
				if err != nil {
					t.Fatal(err)
				}
				cols["payload"] = encoded
			}
			f.Insert(t, "collaboration_event", cols)
		}
		if scenario == "exact" {
			exactCommand, exactEvent = command, event
		}
	}
	var before string
	f.QueryRow(t, `SELECT jsonb_agg(to_jsonb(c) ORDER BY c.id)::text FROM collaboration_command c`).Scan(&before)
	up := cancellationMigrationSQL(t, "735_collaboration_command_cancellation.up")
	f.Exec(t, up)
	if f.Count(t, `SELECT count(*) FROM collaboration_command WHERE cancellation_event_id IS NOT NULL`) != 1 || f.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=$1 AND cancellation_event_id=$2`, exactCommand, exactEvent) != 1 {
		t.Fatal("migration inferred provenance from bulk, duplicate, cross-issue or inexact evidence")
	}
	var after string
	f.QueryRow(t, `SELECT jsonb_agg(to_jsonb(c)-'cancellation_event_id' ORDER BY c.id)::text FROM collaboration_command c`).Scan(&after)
	if before != after {
		t.Fatal("provenance backfill rewrote original commands or placement")
	}
	f.Exec(t, up)
	if f.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=$1 AND cancellation_event_id=$2`, exactCommand, exactEvent) != 1 {
		t.Fatal("migration replay changed the original event association")
	}
	for _, mutation := range []struct {
		sql string
		id  string
	}{
		{`UPDATE collaboration_command SET cancellation_event_id=NULL WHERE id=$1`, exactCommand},
		{`UPDATE collaboration_command SET cancellation_event_id=gen_random_uuid() WHERE id=$1`, exactCommand},
		{`UPDATE collaboration_command SET reason='replacement reason' WHERE id=$1`, exactCommand},
		{`UPDATE collaboration_command SET state='waiting' WHERE id=$1`, exactCommand},
		{`UPDATE collaboration_event SET actor_user_id=gen_random_uuid() WHERE id=$1`, exactEvent},
		{`UPDATE collaboration_event SET payload='{}' WHERE id=$1`, exactEvent},
		{`UPDATE collaboration_event SET created_at=now() WHERE id=$1`, exactEvent},
		{`UPDATE collaboration_event SET kind='other' WHERE id=$1`, exactEvent},
	} {
		_, err := f.Pool.Exec(t.Context(), mutation.sql, mutation.id)
		var violation *pgconn.PgError
		if !errors.As(err, &violation) || violation.Code != "23514" {
			t.Fatalf("cancellation evidence remained mutable: %s: %v", mutation.sql, err)
		}
	}
	f.Exec(t, `UPDATE collaboration_event SET outbox_state='delivered' WHERE id=$1`, exactEvent)
	if f.Count(t, `SELECT count(*) FROM collaboration_event WHERE id=$1 AND outbox_state='delivered'`, exactEvent) != 1 {
		t.Fatal("immutable audit guard blocked outbox acknowledgement")
	}
	if f.Count(t, `SELECT count(*) FROM pg_constraint WHERE connamespace=current_schema()::regnamespace AND contype='f'`) != 0 {
		t.Fatal("cancellation provenance introduced a database foreign key")
	}
	index := cancellationMigrationSQL(t, "736_collaboration_command_history.up")
	if !strings.Contains(index, "CREATE INDEX CONCURRENTLY") || strings.Count(index, ";") != 1 {
		t.Fatal("history index is not its own concurrent migration")
	}
	f.Exec(t, index)
	if f.Count(t, `SELECT count(*) FROM pg_index i JOIN pg_class c ON c.oid=i.indexrelid WHERE c.relnamespace=current_schema()::regnamespace AND c.relname='collaboration_command_history_idx' AND i.indisvalid`) != 1 {
		t.Fatal("history index was not built successfully")
	}
	versions := []string{"736_collaboration_command_history", "735_collaboration_command_cancellation"}
	for _, version := range versions {
		if _, err := f.Pool.Exec(t.Context(), cancellationMigrationSQL(t, version+".down")); err == nil || !strings.Contains(err.Error(), "retained collaboration") {
			t.Fatalf("rollback discarded retained cancellation provenance: %s: %v", version, err)
		}
	}
	holder, err := f.Pool.Begin(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer holder.Rollback(t.Context())
	if _, err = holder.Exec(t.Context(), `UPDATE collaboration_command SET reason=reason WHERE id=$1`, exactCommand); err != nil {
		t.Fatal(err)
	}
	for _, version := range versions {
		_, err = f.Pool.Exec(t.Context(), cancellationMigrationSQL(t, version+".down"))
		var busy *pgconn.PgError
		if !errors.As(err, &busy) || busy.Code != "55P03" {
			t.Fatalf("rollback failed to fence an active writer: %s: %v", version, err)
		}
	}
	if err = holder.Rollback(t.Context()); err != nil {
		t.Fatal(err)
	}
	f.Exec(t, `DELETE FROM collaboration_command`)
	for _, version := range versions {
		f.Exec(t, cancellationMigrationSQL(t, version+".down"))
	}
}
