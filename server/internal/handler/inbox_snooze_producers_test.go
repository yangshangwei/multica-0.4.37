package handler

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/inboxstate"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func TestTriageNotificationDeliveryRespectsInboxSnooze(t *testing.T) {
	issue := dbfx.Issue(t, "Postponed triage notification")
	dbfx.Insert(t, "inbox_item", testutil.Cols{"workspace_id": testWorkspaceID, "recipient_type": "member", "recipient_id": testUserID, "issue_id": issue, "type": "new_comment", "title": "Already postponed"})
	dbfx.Cleanup(t, `DELETE FROM inbox_item WHERE workspace_id=$1 AND issue_id=$2`, testWorkspaceID, issue)
	dbfx.Cleanup(t, `DELETE FROM triage_notification WHERE workspace_id=$1 AND issue_id=$2`, testWorkspaceID, issue)
	dbfx.Cleanup(t, `DELETE FROM inbox_snooze WHERE workspace_id=$1 AND group_id=$2`, testWorkspaceID, issue)
	dbfx.Cleanup(t, `DELETE FROM inbox_critical_source WHERE workspace_id=$1 AND group_id=$2`, testWorkspaceID, issue)
	h := *testHandler
	h.Bus = events.New()
	var visible, updated int
	h.Bus.Subscribe(protocol.EventInboxNew, func(events.Event) { visible++ })
	h.Bus.Subscribe(protocol.EventInboxUpdated, func(events.Event) { updated++ })
	tx, err := testPool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	g := inboxstate.Group{WorkspaceID: parseUUID(testWorkspaceID), RecipientType: "member", RecipientID: parseUUID(testUserID), Kind: "issue", ID: parseUUID(issue)}
	state, err := inboxstate.Lock(context.Background(), tx, g)
	if err != nil {
		t.Fatal(err)
	}
	now, err := inboxstate.Now(context.Background(), tx)
	if err != nil {
		t.Fatal(err)
	}
	if err = inboxstate.Snooze(context.Background(), tx, &state, now.Add(time.Hour), state.Revision); err != nil {
		t.Fatal(err)
	}
	if err = h.queueTriageNotification(context.Background(), tx, g.WorkspaceID, g.RecipientID, g.ID, pgtype.UUID{}, "ordinary:postponed-notice", "Triage update", nil, now); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err = h.deliverTriageNotificationsOnce(context.Background(), g.WorkspaceID); err != nil {
		t.Fatal(err)
	}
	if visible != 0 || updated != 1 {
		t.Errorf("triage postponed delivery emitted new=%d updated=%d, want 0/1", visible, updated)
	}
	after, err := inboxstate.Read(context.Background(), testPool, g)
	if err != nil || !after.SnoozedUntil.Valid || !after.SnoozedUntil.Time.Equal(state.SnoozedUntil.Time) {
		t.Errorf("triage delivery changed snooze: %v", err)
	}
	if n := dbfx.Count(t, `SELECT count(*) FROM inbox_item WHERE workspace_id=$1 AND issue_id=$2 AND type='triage'`, testWorkspaceID, issue); n != 1 {
		t.Errorf("triage postponed retained rows=%d, want 1", n)
	}
}
