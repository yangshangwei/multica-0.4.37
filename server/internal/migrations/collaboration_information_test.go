package migrations

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/testutil"
)

func TestCollaborationInformationMetadataMigrationAndRetention(t *testing.T) {
	fx, run := newCollaborationStageMigrationFixture(t)
	if err := run(`CREATE TABLE issue_handoff (id uuid NOT NULL DEFAULT gen_random_uuid(), issue_id uuid NOT NULL,
		purpose text NOT NULL DEFAULT 'information', note text NOT NULL DEFAULT '', previous_status text NOT NULL DEFAULT 'todo',
		resolution text NOT NULL DEFAULT 'open'); CREATE TABLE collaboration_event (id uuid NOT NULL DEFAULT gen_random_uuid(),kind text NOT NULL);`); err != nil {
		t.Fatal(err)
	}
	file := func(stem, direction string) string {
		t.Helper()
		body, err := os.ReadFile(filepath.Join("..", "..", "migrations", stem+"."+direction+".sql"))
		if err != nil {
			t.Fatal(err)
		}
		return string(body)
	}
	columns := file("866_collaboration_information_metadata", "up")
	ready := file("867_collaboration_information_ready", "up")
	guardDown := file("867_collaboration_information_ready", "down")
	columnsDown := file("866_collaboration_information_metadata", "down")
	for _, forbidden := range []string{"FOREIGN KEY", " REFERENCES ", "CREATE INDEX", "PRIMARY KEY", " UNIQUE", " DEFAULT "} {
		if strings.Contains(strings.ToUpper(columns), forbidden) {
			t.Fatalf("new metadata migration contains %q", forbidden)
		}
	}
	legacy := fx.Insert(t, "issue_handoff", testutil.Cols{"issue_id": uuid.NewString(), "note": "Legacy exact note"})
	if err := run(columns); err != nil {
		t.Fatal(err)
	}
	if err := run(columns); err != nil {
		t.Fatalf("idempotent metadata installation failed: %v", err)
	}
	if err := run(ready); err != nil {
		t.Fatal(err)
	}
	if fx.Count(t, `SELECT count(*) FROM issue_handoff WHERE id=$1 AND note='Legacy exact note' AND unblock_condition IS NULL AND estimated_resolution_at IS NULL AND superseded_by_repair_handoff_id IS NULL`, legacy) != 1 {
		t.Fatal("metadata installation backfilled legacy information")
	}
	for _, item := range []string{"unblock_condition", "estimated_resolution_at", "superseded_by_repair_handoff_id", "repair_event"} {
		t.Run(item, func(t *testing.T) {
			switch item {
			case "unblock_condition":
				fx.Exec(t, `UPDATE issue_handoff SET unblock_condition='new condition' WHERE id=$1`, legacy)
			case "estimated_resolution_at":
				fx.Exec(t, `UPDATE issue_handoff SET estimated_resolution_at=now() WHERE id=$1`, legacy)
			case "superseded_by_repair_handoff_id":
				fx.Exec(t, `UPDATE issue_handoff SET superseded_by_repair_handoff_id=$2 WHERE id=$1`, legacy, uuid.NewString())
			case "repair_event":
				fx.Insert(t, "collaboration_event", testutil.Cols{"kind": "information_repaired"})
			}
			if code, _ := stagePgError(run(guardDown)); code != "23514" {
				t.Fatalf("highest guard did not retain %s: code=%s", item, code)
			}
			if code, _ := stagePgError(run(columnsDown)); code != "23514" {
				t.Fatalf("direct supporting downgrade bypassed retention: code=%s", code)
			}
			if fx.Count(t, `SELECT count(*) FROM pg_catalog.pg_attribute WHERE attrelid=to_regclass('issue_handoff') AND attname IN ('unblock_condition','estimated_resolution_at','superseded_by_repair_handoff_id') AND NOT attisdropped AND NOT attnotnull`) != 3 {
				t.Fatal("failed downgrade removed supporting columns")
			}
			t.Logf("retained_information_migration_case=%s guard=23514 supporting_columns=3", item)
			fx.Exec(t, `UPDATE issue_handoff SET unblock_condition=NULL,estimated_resolution_at=NULL,superseded_by_repair_handoff_id=NULL WHERE id=$1`, legacy)
			fx.Exec(t, `DELETE FROM collaboration_event WHERE kind='information_repaired'`)
		})
	}
	if err := run(guardDown); err != nil {
		t.Fatal(err)
	}
	if err := run(columnsDown); err != nil {
		t.Fatal(err)
	}
	if fx.Count(t, `SELECT count(*) FROM issue_handoff WHERE id=$1 AND note='Legacy exact note'`, legacy) != 1 {
		t.Fatal("safe metadata rollback deleted old retained data")
	}
}
