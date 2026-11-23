package handler

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type snoozeFixture struct{ workspace, issue, item string }

func newSnoozeFixture(t *testing.T, read bool) snoozeFixture {
	t.Helper()
	ws := dbfx.Workspace(t, "Inbox snooze", "inbox-snooze-"+uuid.NewString())
	dbfx.Member(t, ws, testUserID, "owner")
	issue := dbfx.Issue(t, "Snoozed issue", testutil.Cols{"workspace_id": ws})
	item := dbfx.Insert(t, "inbox_item", testutil.Cols{"workspace_id": ws, "recipient_type": "member", "recipient_id": testUserID, "issue_id": issue, "type": "status_changed", "severity": "info", "title": "Snoozed issue", "read": read, "created_at": testutil.Raw("now()-interval '3 hours'")})
	dbfx.Cleanup(t, "DELETE FROM inbox_critical_source WHERE workspace_id=$1", ws)
	dbfx.Cleanup(t, "DELETE FROM inbox_snooze WHERE workspace_id=$1", ws)
	return snoozeFixture{ws, issue, item}
}

func (f snoozeFixture) request(action string, body any) *http.Request {
	req := testutil.WithHeaders(testutil.JSONRequest(http.MethodPost, "/api/inbox/"+f.item+"/"+action, body), "X-User-ID", testUserID, "X-Workspace-ID", f.workspace)
	return testutil.WithURLParams(req, "id", f.item)
}

func (f snoozeFixture) snooze(t *testing.T, revision int64) InboxSnoozeReceipt {
	t.Helper()
	var receipt InboxSnoozeReceipt
	// Exercise PostgreSQL's precision boundary regardless of the host clock.
	until := time.Now().UTC().Add(time.Hour).Truncate(time.Microsecond).Add(789 * time.Nanosecond)
	testutil.Call(t, inboxWorkspaceHandler(testHandler.SnoozeInboxItem), f.request("snooze", map[string]any{"snoozed_until": until.Format(time.RFC3339Nano), "expected_revision": revision})).Want(200).JSON(&receipt)
	if receipt.SnoozedUntil == nil || *receipt.SnoozedUntil != until.Truncate(time.Microsecond).Format(time.RFC3339Nano) {
		t.Fatal("snooze receipt did not retain the requested deadline at database precision")
	}
	return receipt
}

func (f snoozeFixture) list(t *testing.T, handler http.HandlerFunc) []InboxItemResponse {
	t.Helper()
	var items []InboxItemResponse
	testutil.Call(t, inboxWorkspaceHandler(handler), inboxRequest(http.MethodGet, "/api/inbox", f.workspace)).Want(200).JSON(&items)
	return items
}

func TestInboxSnoozeGroupAndItemIsolation(t *testing.T) {
	f := newSnoozeFixture(t, true)
	dbfx.Insert(t, "inbox_item", testutil.Cols{"workspace_id": f.workspace, "recipient_type": "member", "recipient_id": testUserID, "issue_id": f.issue, "type": "new_comment", "severity": "info", "title": "Sibling", "read": false})
	other := dbfx.Insert(t, "inbox_item", testutil.Cols{"workspace_id": f.workspace, "recipient_type": "member", "recipient_id": testUserID, "type": "triage", "severity": "info", "title": "Independent notice"})
	receipt := f.snooze(t, 0)
	if receipt.GroupID != f.issue || receipt.GroupKind != "issue" || receipt.Revision != 1 || receipt.SnoozedUntil == nil || receipt.UndoToken == nil {
		t.Fatalf("unexpected snooze receipt: %+v", receipt)
	}
	active := f.list(t, testHandler.ListInbox)
	if len(active) != 1 || active[0].ID != other {
		t.Fatalf("active=%+v, want only independent notification", active)
	}
	snoozed := f.list(t, testHandler.ListSnoozedInbox)
	if len(snoozed) != 2 {
		t.Fatalf("snoozed raw rows=%d, want both siblings", len(snoozed))
	}
	for _, item := range snoozed {
		if item.SnoozedUntil == nil || *item.SnoozedUntil != *receipt.SnoozedUntil || item.SnoozeRevision != 1 || item.Archived {
			t.Fatalf("incomplete group projection: %+v", item)
		}
	}
	if got := dbfx.Count(t, "SELECT count(*) FROM inbox_item WHERE id=$1 AND read=true AND archived=false", f.item); got != 1 {
		t.Fatal("snoozing altered the original read/archived state")
	}
	independent := snoozeFixture{workspace: f.workspace, item: other}
	itemReceipt := independent.snooze(t, 0)
	if itemReceipt.GroupKind != "item" || itemReceipt.GroupID != other {
		t.Fatalf("issue-less group receipt=%+v", itemReceipt)
	}
}

func TestInboxSnoozeValidationAndOwnership(t *testing.T) {
	f := newSnoozeFixture(t, false)
	for _, body := range []any{
		map[string]any{"snoozed_until": "yesterday", "expected_revision": 0},
		map[string]any{"snoozed_until": time.Now().UTC().Add(-time.Hour).Format(time.RFC3339Nano), "expected_revision": 0},
		map[string]any{"snoozed_until": time.Now().UTC().Add(time.Hour).Format(time.RFC3339Nano)},
		map[string]any{"snoozed_until": time.Now().UTC().Add(time.Hour).Format(time.RFC3339Nano), "expected_revision": -1},
		map[string]any{"snoozed_until": time.Now().UTC().Add(time.Hour).Format(time.RFC3339Nano), "expected_revision": 0, "recipient_id": uuid.NewString()},
	} {
		testutil.Call(t, inboxWorkspaceHandler(testHandler.SnoozeInboxItem), f.request("snooze", body)).Want(400)
	}
	other := dbfx.User(t, "Other inbox owner", "other-inbox-"+uuid.NewString()+"@example.test")
	dbfx.Member(t, f.workspace, other, "member")
	req := testutil.WithHeaders(f.request("snooze", map[string]any{"snoozed_until": time.Now().UTC().Add(time.Hour).Format(time.RFC3339Nano), "expected_revision": 0}), "X-User-ID", other)
	testutil.Call(t, inboxWorkspaceHandler(testHandler.SnoozeInboxItem), req).Want(404)
	if got := dbfx.Count(t, "SELECT count(*) FROM inbox_snooze WHERE workspace_id=$1 AND snoozed_until IS NOT NULL", f.workspace); got != 0 {
		t.Fatal("invalid or unauthorized request persisted a snooze")
	}
}

func TestInboxSnoozeUndoAndNewerVersionConflict(t *testing.T) {
	f := newSnoozeFixture(t, true)
	first := f.snooze(t, 0)
	second := f.snooze(t, first.Revision)
	testutil.Call(t, inboxWorkspaceHandler(testHandler.UndoInboxSnooze), f.request("snooze/undo", map[string]any{"undo_token": *first.UndoToken})).Want(409)
	var restored InboxSnoozeReceipt
	testutil.Call(t, inboxWorkspaceHandler(testHandler.UndoInboxSnooze), f.request("snooze/undo", map[string]any{"undo_token": *second.UndoToken})).Want(200).JSON(&restored)
	if restored.Revision != second.Revision+1 || restored.SnoozedUntil == nil || *restored.SnoozedUntil != *first.SnoozedUntil {
		t.Fatalf("undo failed to restore previous deadline: %+v", restored)
	}
	testutil.Call(t, inboxWorkspaceHandler(testHandler.UndoInboxSnooze), f.request("snooze/undo", map[string]any{"undo_token": *second.UndoToken})).Want(409)
	testutil.Call(t, inboxWorkspaceHandler(testHandler.SnoozeInboxItem), f.request("snooze", map[string]any{"snoozed_until": time.Now().UTC().Add(time.Hour).Format(time.RFC3339Nano), "expected_revision": first.Revision})).Want(409)
}

func TestInboxRemindNowUndoPreservesReadState(t *testing.T) {
	f := newSnoozeFixture(t, true)
	first := f.snooze(t, 0)
	var reminded InboxSnoozeReceipt
	testutil.Call(t, inboxWorkspaceHandler(testHandler.UnsnoozeInboxItem), f.request("unsnooze", map[string]any{"expected_revision": first.Revision})).Want(200).JSON(&reminded)
	active := f.list(t, testHandler.ListInbox)
	if len(active) != 1 || active[0].Read || active[0].RemindedAt == nil || reminded.SnoozedUntil != nil {
		t.Fatalf("manual reminder did not return unread: %+v %+v", active, reminded)
	}
	testutil.Call(t, inboxWorkspaceHandler(testHandler.UndoInboxSnooze), f.request("snooze/undo", map[string]any{"undo_token": *reminded.UndoToken})).Want(200)
	snoozed := f.list(t, testHandler.ListSnoozedInbox)
	if len(snoozed) != 1 || !snoozed[0].Read {
		t.Fatalf("undo did not restore original read state: %+v", snoozed)
	}
}

func TestInboxReadChangeInvalidatesSnoozeUndo(t *testing.T) {
	f := newSnoozeFixture(t, false)
	receipt := f.snooze(t, 0)
	testutil.Call(t, inboxWorkspaceHandler(testHandler.MarkInboxRead), f.request("read", nil)).Want(200)
	testutil.Call(t, inboxWorkspaceHandler(testHandler.UndoInboxSnooze), f.request("snooze/undo", map[string]any{"undo_token": *receipt.UndoToken})).Want(409)
	items := f.list(t, testHandler.ListSnoozedInbox)
	if len(items) != 1 || !items[0].Read {
		t.Fatal("stale undo overwrote the newer read state")
	}
}

func TestInboxExpiryProjectionAndSingleConsumption(t *testing.T) {
	f := newSnoozeFixture(t, true)
	receipt := f.snooze(t, 0)
	dbfx.Exec(t, "UPDATE inbox_snooze SET snoozed_until=now()-interval '1 minute' WHERE workspace_id=$1", f.workspace)
	items := f.list(t, testHandler.ListInbox)
	if len(items) != 1 || items[0].Read || items[0].RemindedAt == nil || items[0].ReminderReason == nil || *items[0].ReminderReason != "due" {
		t.Fatalf("paused-sweeper projection=%+v", items)
	}
	restarted := *testHandler
	if err := restarted.expireInboxSnoozesOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	var revision int64
	dbfx.QueryRow(t, "SELECT revision FROM inbox_snooze WHERE workspace_id=$1", f.workspace).Scan(&revision)
	if revision != receipt.Revision+1 {
		t.Fatalf("expiry revision=%d, want one transition", revision)
	}
	if err := restarted.expireInboxSnoozesOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	dbfx.QueryRow(t, "SELECT revision FROM inbox_snooze WHERE workspace_id=$1", f.workspace).Scan(&revision)
	if revision != receipt.Revision+1 {
		t.Fatal("second sweep repeated the reminder")
	}
	if got := dbfx.Count(t, "SELECT count(*) FROM inbox_item WHERE issue_id=$1", f.issue); got != 1 {
		t.Fatal("expiry duplicated notification identity")
	}
}

func TestInboxOverdueBulkReadAndReadArchive(t *testing.T) {
	for _, action := range []string{"read", "archive_read"} {
		t.Run(action, func(t *testing.T) {
			f := newSnoozeFixture(t, true)
			f.snooze(t, 0)
			dbfx.Exec(t, "UPDATE inbox_snooze SET snoozed_until=now()-interval '1 minute' WHERE workspace_id=$1", f.workspace)
			if action == "read" {
				testutil.Call(t, inboxWorkspaceHandler(testHandler.MarkAllInboxRead), inboxRequest(http.MethodPost, "/api/inbox/mark-all-read", f.workspace)).Want(200)
				items := f.list(t, testHandler.ListInbox)
				if len(items) != 1 || !items[0].Read {
					t.Fatalf("bulk read did not consume overdue unread marker: %+v", items)
				}
				if items[0].RemindedAt == nil {
					t.Fatal("bulk acknowledgment discarded the restored reminder ordering")
				}
				if err := testHandler.expireInboxSnoozesOnce(context.Background()); err != nil {
					t.Fatal(err)
				}
				if refreshed := f.list(t, testHandler.ListInbox); len(refreshed) != 1 || !refreshed[0].Read {
					t.Fatal("expired marker reopened an acknowledged group")
				}
			} else {
				testutil.Call(t, inboxWorkspaceHandler(testHandler.ArchiveAllReadInbox), inboxRequest(http.MethodPost, "/api/inbox/archive-all-read", f.workspace)).Want(200)
				items := f.list(t, testHandler.ListInbox)
				if len(items) != 1 || items[0].Read || items[0].Archived {
					t.Fatalf("archive read consumed the newly unread reminder: %+v", items)
				}
			}
		})
	}
}

func TestInboxFutureSnoozesSurviveMainBulkActions(t *testing.T) {
	for _, handler := range []http.HandlerFunc{testHandler.MarkAllInboxRead, testHandler.ArchiveAllInbox, testHandler.ArchiveAllReadInbox, testHandler.ArchiveCompletedInbox} {
		f := newSnoozeFixture(t, false)
		f.snooze(t, 0)
		testutil.Call(t, inboxWorkspaceHandler(handler), inboxRequest(http.MethodPost, "/api/inbox/bulk", f.workspace)).Want(200)
		items := f.list(t, testHandler.ListSnoozedInbox)
		if len(items) != 1 || items[0].Read || items[0].Archived || items[0].SnoozeRevision != 1 {
			t.Fatalf("main bulk action modified future snooze: %+v", items)
		}
	}
}

func TestArchivedSnoozeNeverReappears(t *testing.T) {
	f := newSnoozeFixture(t, false)
	f.snooze(t, 0)
	testutil.Call(t, inboxWorkspaceHandler(testHandler.ArchiveInboxItem), f.request("archive", nil)).Want(200)
	if err := testHandler.expireInboxSnoozesOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if items := f.list(t, testHandler.ListInbox); len(items) != 0 {
		t.Fatalf("archived group reappeared: %+v", items)
	}
	if items := f.list(t, testHandler.ListSnoozedInbox); len(items) != 0 {
		t.Fatal("archive retained an active snooze")
	}
}

func TestInboxSnoozeCleanupFollowsIssueAndMembership(t *testing.T) {
	for _, kind := range []string{"issue", "membership"} {
		t.Run(kind, func(t *testing.T) {
			f := newSnoozeFixture(t, false)
			f.snooze(t, 0)
			comment := dbfx.Comment(t, f.issue, "Critical source", testutil.Cols{"workspace_id": f.workspace})
			dbfx.Insert(t, "inbox_critical_source", testutil.Cols{"workspace_id": f.workspace, "recipient_type": "member", "recipient_id": testUserID, "group_kind": "issue", "group_id": f.issue, "notification_type": "mentioned", "source_kind": "comment_mention", "source_id": comment})
			if kind == "issue" {
				if err := testHandler.Queries.DeleteIssue(context.Background(), db.DeleteIssueParams{ID: parseUUID(f.issue), WorkspaceID: parseUUID(f.workspace)}); err != nil {
					t.Fatal(err)
				}
			} else {
				var member string
				dbfx.QueryRow(t, "SELECT id FROM member WHERE workspace_id=$1 AND user_id=$2", f.workspace, testUserID).Scan(&member)
				if _, err := testHandler.revokeAndRemoveMember(context.Background(), parseUUID(f.workspace), parseUUID(testUserID), parseUUID(member), parseUUID(testUserID)); err != nil {
					t.Fatal(err)
				}
			}
			for _, table := range []string{"inbox_snooze", "inbox_critical_source"} {
				if count := dbfx.Count(t, "SELECT count(*) FROM "+table+" WHERE workspace_id=$1", f.workspace); count != 0 {
					t.Fatalf("%s cleanup left %d %s rows", kind, count, table)
				}
			}
		})
	}
}

func TestInboxNewActivityRisesPastOldReminder(t *testing.T) {
	f := newSnoozeFixture(t, false)
	receipt := f.snooze(t, 0)
	testutil.Call(t, inboxWorkspaceHandler(testHandler.UnsnoozeInboxItem), f.request("unsnooze", map[string]any{"expected_revision": receipt.Revision})).Want(200)
	dbfx.Exec(t, "UPDATE inbox_snooze SET reminded_at=now()-interval '2 hours' WHERE workspace_id=$1", f.workspace)
	other := dbfx.Issue(t, "Other newer group", testutil.Cols{"workspace_id": f.workspace})
	dbfx.Insert(t, "inbox_item", testutil.Cols{"workspace_id": f.workspace, "recipient_type": "member", "recipient_id": testUserID, "issue_id": other, "type": "new_comment", "severity": "info", "title": "Other group", "created_at": testutil.Raw("now()-interval '1 hour'")})
	newest := dbfx.Insert(t, "inbox_item", testutil.Cols{"workspace_id": f.workspace, "recipient_type": "member", "recipient_id": testUserID, "issue_id": f.issue, "type": "new_comment", "severity": "info", "title": "Newest real activity"})
	items := f.list(t, testHandler.ListInbox)
	if len(items) == 0 || items[0].ID != newest {
		t.Fatal("an old reminder timestamp pinned newer notification activity below another group")
	}
}
