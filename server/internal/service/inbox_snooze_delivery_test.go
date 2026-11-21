package service

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/inboxstate"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func postponeProducerIssue(t *testing.T, fx *testutil.Fixture, issue, recipient string) inboxstate.State {
	t.Helper()
	fx.Insert(t, "inbox_item", testutil.Cols{"workspace_id": fx.WorkspaceID, "recipient_type": "member", "recipient_id": recipient, "issue_id": issue, "type": "new_comment", "title": "Original postponed source", "read": true})
	fx.Cleanup(t, `DELETE FROM inbox_item WHERE workspace_id=$1 AND issue_id=$2`, fx.WorkspaceID, issue)
	fx.Cleanup(t, `DELETE FROM inbox_snooze WHERE workspace_id=$1 AND group_id=$2`, fx.WorkspaceID, issue)
	fx.Cleanup(t, `DELETE FROM inbox_critical_source WHERE workspace_id=$1 AND group_id=$2`, fx.WorkspaceID, issue)
	tx, err := fx.Pool.Begin(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(t.Context())
	state, err := inboxstate.Lock(t.Context(), tx, inboxstate.Group{WorkspaceID: util.MustParseUUID(fx.WorkspaceID), RecipientType: "member", RecipientID: util.MustParseUUID(recipient), Kind: "issue", ID: util.MustParseUUID(issue)})
	if err != nil {
		t.Fatal(err)
	}
	now, err := inboxstate.Now(t.Context(), tx)
	if err != nil {
		t.Fatal(err)
	}
	if err = inboxstate.Snooze(t.Context(), tx, &state, now.Add(time.Hour), state.Revision); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(t.Context()); err != nil {
		t.Fatal(err)
	}
	return state
}

func TestAutopilotSubscriberDeliveryRespectsInboxSnooze(t *testing.T) {
	f, owner := newPrincipalFixture(t)
	issueID := f.Issue(t, "Postponed automation issue")
	state := postponeProducerIssue(t, f.Fixture, issueID, owner)
	issue, err := f.q.GetIssue(t.Context(), util.MustParseUUID(issueID))
	if err != nil {
		t.Fatal(err)
	}
	var visible, updated int
	f.svc.Bus.Subscribe(protocol.EventInboxNew, func(events.Event) { visible++ })
	f.svc.Bus.Subscribe(protocol.EventInboxUpdated, func(events.Event) { updated++ })
	f.svc.notifyAutopilotSubscribersOnCreate(t.Context(), db.Autopilot{ID: util.MustParseUUID(uuid.NewString()), WorkspaceID: issue.WorkspaceID}, issue, pgtype.UUID{}, []db.AutopilotSubscriber{{UserType: "member", UserID: util.MustParseUUID(owner)}})
	if visible != 0 || updated != 1 {
		t.Errorf("automation postponed delivery emitted new=%d updated=%d, want 0/1", visible, updated)
	}
	after, err := inboxstate.Read(t.Context(), f.Pool, inboxstate.ForState(state))
	if err != nil || !after.SnoozedUntil.Valid || !after.SnoozedUntil.Time.Equal(state.SnoozedUntil.Time) {
		t.Errorf("automation delivery changed snooze: %v", err)
	}
	if n := f.Count(t, `SELECT count(*) FROM inbox_item WHERE workspace_id=$1 AND issue_id=$2 AND type='issue_subscribed'`, f.WorkspaceID, issueID); n != 1 {
		t.Errorf("automation postponed notification rows=%d, want 1", n)
	}
}

func TestQuickCreateCompletionDeliveryRespectsInboxSnooze(t *testing.T) {
	f, owner := newPrincipalFixture(t)
	agent := f.Agent(t, "Quick create notification agent", "")
	taskID := f.Task(t, agent, testutil.Cols{"status": "completed", "completed_at": testutil.Raw("clock_timestamp()")})
	issueID := f.Issue(t, "Postponed quick-create issue", testutil.Cols{"origin_type": "quick_create", "origin_id": taskID})
	state := postponeProducerIssue(t, f.Fixture, issueID, owner)
	task, err := f.q.GetAgentTask(t.Context(), util.MustParseUUID(taskID))
	if err != nil {
		t.Fatal(err)
	}
	svc := &TaskService{Queries: f.q, TxStarter: f.Pool, Bus: events.New()}
	var visible, updated int
	svc.Bus.Subscribe(protocol.EventInboxNew, func(events.Event) { visible++ })
	svc.Bus.Subscribe(protocol.EventInboxUpdated, func(events.Event) { updated++ })
	svc.notifyQuickCreateCompleted(context.Background(), task, QuickCreateContext{RequesterID: owner, WorkspaceID: f.WorkspaceID, Prompt: "Original prompt"}, nil)
	if visible != 0 || updated != 1 {
		t.Errorf("quick-create postponed delivery emitted new=%d updated=%d, want 0/1", visible, updated)
	}
	after, err := inboxstate.Read(t.Context(), f.Pool, inboxstate.ForState(state))
	if err != nil || !after.SnoozedUntil.Valid || !after.SnoozedUntil.Time.Equal(state.SnoozedUntil.Time) {
		t.Errorf("quick-create delivery changed snooze: %v", err)
	}
	if n := f.Count(t, `SELECT count(*) FROM inbox_item WHERE workspace_id=$1 AND issue_id=$2 AND type='quick_create_done'`, f.WorkspaceID, issueID); n != 1 {
		t.Errorf("quick-create postponed notification rows=%d, want 1", n)
	}
}
