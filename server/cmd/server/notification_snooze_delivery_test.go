package main

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/handler"
	"github.com/multica-ai/multica/server/internal/inboxstate"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// The state transition matrix belongs to internal/inboxstate. These tests drive
// the actual listener with saved source identities and inspect retained rows,
// deadlines and broadcast eligibility rather than repeating its pure matrix.
func snoozeDeliveryGroup(t *testing.T, f *commentReplayFixture, issue, recipient string) inboxstate.State {
	t.Helper()
	g := inboxstate.Group{WorkspaceID: util.MustParseUUID(f.fx.WorkspaceID), RecipientType: "member", RecipientID: util.MustParseUUID(recipient), Kind: "issue", ID: util.MustParseUUID(issue)}
	tx, err := testPool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
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
	if err = tx.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	f.fx.Cleanup(t, `DELETE FROM inbox_snooze WHERE workspace_id=$1 AND group_kind='issue' AND group_id=$2`, f.fx.WorkspaceID, issue)
	f.fx.Cleanup(t, `DELETE FROM inbox_critical_source WHERE workspace_id=$1 AND group_kind='issue' AND group_id=$2`, f.fx.WorkspaceID, issue)
	return state
}

func retainedDeliverySeed(t *testing.T, f *commentReplayFixture, issue, recipient, kind string) {
	t.Helper()
	f.fx.Insert(t, "inbox_item", testutil.Cols{
		"workspace_id": f.fx.WorkspaceID, "recipient_type": "member", "recipient_id": recipient,
		"issue_id": issue, "type": kind, "title": "Original postponed notification", "read": true,
	})
}

func readDeliveryState(t *testing.T, f *commentReplayFixture, issue, recipient string) inboxstate.State {
	t.Helper()
	state, err := inboxstate.Read(context.Background(), testPool, inboxstate.Group{
		WorkspaceID: util.MustParseUUID(f.fx.WorkspaceID), RecipientType: "member", RecipientID: util.MustParseUUID(recipient), Kind: "issue", ID: util.MustParseUUID(issue),
	})
	if err != nil {
		t.Fatal(err)
	}
	return state
}

func TestNotificationSnoozedOrdinaryCommentRetainsGroup(t *testing.T) {
	f := newCommentReplayFixture(t)
	issue := f.issue(t, "Ordinary postponed source")
	watcher := f.member(t, "Postponed watcher")
	f.subscribe(t, issue, "member", watcher, "manual")
	retainedDeliverySeed(t, f, issue, watcher, "new_comment")
	before := snoozeDeliveryGroup(t, f, issue, watcher)
	comment := f.fx.Comment(t, issue, "Ordinary update stays postponed")
	collector := &commentReplayCollector{}
	bus := collector.bus(f.queries)
	var updates []events.Event
	bus.Subscribe(protocol.EventInboxUpdated, func(e events.Event) { updates = append(updates, e) })
	bus.Publish(f.event(t, comment))
	if notices := len(collector.snapshot()); notices != 0 {
		t.Errorf("ordinary postponed source emitted %d banner-eligible inbox:new events, want 0", notices)
	}
	after := readDeliveryState(t, f, issue, watcher)
	if !after.SnoozedUntil.Valid || !after.SnoozedUntil.Time.Equal(before.SnoozedUntil.Time) {
		t.Error("ordinary comment changed the saved snooze deadline")
	}
	rows := f.items(t, issue)
	if len(rows) != 2 {
		t.Errorf("ordinary postponed delivery retained %d rows, want original plus new comment", len(rows))
	}
	if len(updates) != 1 {
		t.Errorf("ordinary postponed delivery emitted %d state invalidations, want 1", len(updates))
	} else {
		payload, ok := updates[0].Payload.(map[string]any)
		if !ok || payload["workspace_id"] != f.fx.WorkspaceID || payload["recipient_id"] != watcher || payload["group_id"] != issue || payload["item"] != nil {
			t.Error("postponed invalidation lost source/recipient identity or exposed item content")
		}
	}
}

func TestNotificationSnoozedDirectMentionWakesOnce(t *testing.T) {
	f := newCommentReplayFixture(t)
	issue := f.issue(t, "Direct postponed source")
	watcher := f.member(t, "Direct target")
	f.subscribe(t, issue, "member", watcher, "manual")
	retainedDeliverySeed(t, f, issue, watcher, "new_comment")
	snoozeDeliveryGroup(t, f, issue, watcher)
	comment := f.fx.Comment(t, issue, "Please review [@Target](mention://member/"+watcher+").")
	event := f.event(t, comment)
	collector := &commentReplayCollector{}
	bus := collector.bus(f.queries)
	bus.Publish(event)
	state := readDeliveryState(t, f, issue, watcher)
	if state.SnoozedUntil.Valid {
		t.Error("a saved direct mention did not wake its postponed recipient")
	}
	if count := len(collector.snapshot()); count != 1 {
		t.Errorf("direct mention emitted %d banner-eligible events, want one direct mention only", count)
	}
	rows := f.items(t, issue)
	var directID string
	for _, row := range rows {
		if row.Type != "mentioned" {
			continue
		}
		var details map[string]string
		if err := json.Unmarshal(row.Details, &details); err != nil {
			t.Fatal(err)
		}
		if details["mention_scope"] != "member" || details["comment_id"] != comment || row.Read {
			t.Error("direct mention lost original member/source identity or unread state")
		}
		directID = util.UUIDToString(row.ID)
	}
	if directID == "" {
		t.Fatal("no retained direct mention")
	}
	snoozeDeliveryGroup(t, f, issue, watcher)
	before := f.items(t, issue)
	collector.bus(f.queries).Publish(event)
	if !readDeliveryState(t, f, issue, watcher).SnoozedUntil.Valid {
		t.Error("replayed original direct mention woke a later snooze")
	}
	if after := f.items(t, issue); !reflect.DeepEqual(before, after) {
		t.Error("original direct mention replay duplicated or rewrote retained notifications")
	}
	if count := len(collector.snapshot()); count != 1 {
		t.Errorf("direct mention replay emitted another banner: total=%d", count)
	}
}

func TestNotificationSnoozedMentionScope(t *testing.T) {
	for _, name := range []string{"all_only", "all_then_direct", "direct_then_all"} {
		t.Run(name, func(t *testing.T) {
			f := newCommentReplayFixture(t)
			issue := f.issue(t, "Mention scope source")
			watcher := f.member(t, "Mention scope target")
			retainedDeliverySeed(t, f, issue, watcher, "new_comment")
			snoozeDeliveryGroup(t, f, issue, watcher)
			comment := f.fx.Comment(t, issue, "Saved source for direct and broad mentions")
			e := f.event(t, comment)
			details, _ := json.Marshal(map[string]string{"comment_id": comment})
			mentions := []mention{{Type: "all", ID: "all"}}
			if name == "all_then_direct" {
				mentions = append(mentions, mention{Type: "member", ID: watcher})
			} else if name == "direct_then_all" {
				mentions = append([]mention{{Type: "member", ID: watcher}}, mentions...)
			}
			bus := events.New()
			var notices []events.Event
			bus.Subscribe(protocol.EventInboxNew, func(e events.Event) { notices = append(notices, e) })
			notifyMentionedMembers(bus, f.queries, testPool, e, mentions, issue, "Scope source", "todo", "Scope source", nil, details)
			state := readDeliveryState(t, f, issue, watcher)
			wantFuture := name == "all_only"
			if state.SnoozedUntil.Valid != wantFuture {
				t.Errorf("%s future snooze=%t, want %t", name, state.SnoozedUntil.Valid, wantFuture)
			}
			for _, row := range f.items(t, issue) {
				if row.Type == "mentioned" && util.UUIDToString(row.RecipientID) == watcher {
					var actual map[string]string
					if err := json.Unmarshal(row.Details, &actual); err != nil {
						t.Fatal(err)
					}
					wantScope := "member"
					if wantFuture {
						wantScope = "all"
					}
					if actual["mention_scope"] != wantScope {
						t.Errorf("%s scope=%q, want %q", name, actual["mention_scope"], wantScope)
					}
				}
			}
			for _, notice := range notices {
				payload := notice.Payload.(map[string]any)["item"].(map[string]any)
				if wantFuture && payload["recipient_id"] == watcher {
					t.Error("@all alone emitted a banner to the postponed member")
				}
			}
		})
	}
}

func TestNotificationSnoozedAssignmentUsesOriginalRevision(t *testing.T) {
	f := newCommentReplayFixture(t)
	issue := f.issue(t, "Assignment occurrence")
	watcher := f.member(t, "Assigned target")
	retainedDeliverySeed(t, f, issue, watcher, "new_comment")
	snoozeDeliveryGroup(t, f, issue, watcher)
	f.fx.Exec(t, `UPDATE issue SET assignee_type='member',assignee_id=$2,revision=revision+1 WHERE id=$1`, issue, watcher)
	row, err := f.queries.GetIssue(context.Background(), util.MustParseUUID(issue))
	if err != nil {
		t.Fatal(err)
	}
	kind := "member"
	event := events.Event{Type: protocol.EventIssueUpdated, WorkspaceID: f.fx.WorkspaceID, ActorType: "member", ActorID: f.fx.UserID,
		Payload: map[string]any{"issue": handler.IssueResponse{ID: issue, WorkspaceID: f.fx.WorkspaceID, Title: row.Title, Status: row.Status, Revision: row.Revision, AssigneeType: &kind, AssigneeID: &watcher}, "assignee_changed": true}}
	// A later unrelated edit cannot redefine this already committed occurrence.
	f.fx.Exec(t, `UPDATE issue SET revision=revision+1 WHERE id=$1`, issue)
	collector := &commentReplayCollector{}
	bus := collector.bus(f.queries)
	bus.Publish(event)
	if readDeliveryState(t, f, issue, watcher).SnoozedUntil.Valid {
		t.Error("new actual assignment did not wake its recipient")
	}
	if n := f.fx.Count(t, `SELECT count(*) FROM inbox_critical_source WHERE workspace_id=$1 AND group_id=$2 AND source_kind='issue_assignment' AND source_revision=$3`, f.fx.WorkspaceID, issue, row.Revision); n != 1 {
		t.Errorf("original assignment revision has %d persisted claims, want 1", n)
	}
	snoozeDeliveryGroup(t, f, issue, watcher)
	before := f.items(t, issue)
	bus.Publish(event)
	if !readDeliveryState(t, f, issue, watcher).SnoozedUntil.Valid || !reflect.DeepEqual(before, f.items(t, issue)) {
		t.Error("old committed assignment replay changed a later snooze or notification")
	}
}

func TestNotificationSnoozedDescriptionMentionUsesOriginalRevision(t *testing.T) {
	f := newCommentReplayFixture(t)
	issue := f.issue(t, "Description occurrence")
	watcher := f.member(t, "Description target")
	retainedDeliverySeed(t, f, issue, watcher, "new_comment")
	snoozeDeliveryGroup(t, f, issue, watcher)
	content := "Review with [@Target](mention://member/" + watcher + ")."
	f.fx.Exec(t, `UPDATE issue SET description=$2,revision=revision+1 WHERE id=$1`, issue, content)
	row, err := f.queries.GetIssue(context.Background(), util.MustParseUUID(issue))
	if err != nil {
		t.Fatal(err)
	}
	event := events.Event{Type: protocol.EventIssueUpdated, WorkspaceID: f.fx.WorkspaceID, ActorType: "member", ActorID: f.fx.UserID,
		Payload: map[string]any{"issue": handler.IssueResponse{ID: issue, WorkspaceID: f.fx.WorkspaceID, Title: row.Title, Status: row.Status, Revision: row.Revision, Description: &content}, "description_changed": true}}
	collector := &commentReplayCollector{}
	bus := collector.bus(f.queries)
	bus.Publish(event)
	if readDeliveryState(t, f, issue, watcher).SnoozedUntil.Valid {
		t.Error("new saved description direct mention did not wake its recipient")
	}
	snoozeDeliveryGroup(t, f, issue, watcher)
	before := f.items(t, issue)
	bus.Publish(event)
	if !readDeliveryState(t, f, issue, watcher).SnoozedUntil.Valid || !reflect.DeepEqual(before, f.items(t, issue)) {
		t.Error("old description mention replay changed a later snooze or notification")
	}
}

func TestNotificationSnoozedFailureUsesOriginalTask(t *testing.T) {
	f := newCommentReplayFixture(t)
	issue := f.issue(t, "Failure occurrence")
	watcher := f.member(t, "Failure watcher")
	f.subscribe(t, issue, "member", watcher, "manual")
	retainedDeliverySeed(t, f, issue, watcher, "new_comment")
	snoozeDeliveryGroup(t, f, issue, watcher)
	agent := f.fx.Agent(t, "Failure source agent", "")
	task := f.fx.Task(t, agent, testutil.Cols{"issue_id": issue, "status": "failed", "completed_at": testutil.Raw("clock_timestamp()")})
	event := events.Event{Type: protocol.EventTaskFailed, WorkspaceID: f.fx.WorkspaceID, ActorType: "agent", ActorID: agent, TaskID: task,
		Payload: map[string]any{"task_id": task, "agent_id": agent, "issue_id": issue}}
	collector := &commentReplayCollector{}
	bus := collector.bus(f.queries)
	bus.Publish(event)
	if readDeliveryState(t, f, issue, watcher).SnoozedUntil.Valid {
		t.Error("actual failed task did not wake its postponed watcher")
	}
	if n := f.fx.Count(t, `SELECT count(*) FROM inbox_critical_source WHERE workspace_id=$1 AND group_id=$2 AND source_kind='task_failure' AND source_id=$3`, f.fx.WorkspaceID, issue, task); n != 1 {
		t.Errorf("original failed task has %d durable claims, want 1", n)
	}
	snoozeDeliveryGroup(t, f, issue, watcher)
	before := f.items(t, issue)
	bus.Publish(event)
	if !readDeliveryState(t, f, issue, watcher).SnoozedUntil.Valid || !reflect.DeepEqual(before, f.items(t, issue)) {
		t.Error("original failure replay changed a later snooze or retained notifications")
	}
}

func TestNotificationSnoozedResolvedFailureCleanup(t *testing.T) {
	for _, keepOrdinary := range []bool{false, true} {
		name := "last_failure"
		if keepOrdinary {
			name = "ordinary_sibling_remains"
		}
		t.Run(name, func(t *testing.T) {
			f := newCommentReplayFixture(t)
			issue := f.issue(t, "Resolved postponed failure")
			watcher := f.member(t, "Resolved failure watcher")
			retainedDeliverySeed(t, f, issue, watcher, "task_failed")
			if keepOrdinary {
				retainedDeliverySeed(t, f, issue, watcher, "new_comment")
			}
			before := snoozeDeliveryGroup(t, f, issue, watcher)
			bus := events.New()
			archiveStaleTaskFailedInbox(context.Background(), f.queries, testPool, bus, f.fx.WorkspaceID, issue)
			after := readDeliveryState(t, f, issue, watcher)
			if keepOrdinary && (!after.SnoozedUntil.Valid || !after.SnoozedUntil.Time.Equal(before.SnoozedUntil.Time)) {
				t.Error("failure retirement erased the still-active ordinary group's schedule")
			}
			if !keepOrdinary && after.SnoozedUntil.Valid {
				t.Error("last failed notification left a ghost snooze after retirement")
			}
			for _, row := range f.items(t, issue) {
				if row.Type == "task_failed" && !row.Archived {
					t.Error("resolved failure remained active")
				}
			}
		})
	}
}
