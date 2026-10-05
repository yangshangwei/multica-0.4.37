package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/pkg/dbid"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

type triageAcceptanceList struct {
	Items  []TriageItem `json:"items"`
	Total  int          `json:"total"`
	Counts struct {
		Pending int `json:"pending"`
		Ready   int `json:"ready"`
		Snoozed int `json:"snoozed"`
	} `json:"counts"`
}

type triageAcceptanceHistory struct {
	Entries []TriageHistoryEntry `json:"entries"`
	Total   int                  `json:"total"`
}

func TestTriageAcceptanceEnableDoesNotBackfillOrdinaryProjectWork(t *testing.T) {
	settings := testutil.Decode[TriageSettings](t, testHandler.GetTriageSettings, newRequest("GET", "/api/triage/settings", nil), 200)
	if settings.Enabled {
		t.Fatal("fresh workspace is enabled")
	}
	project := dbfx.Project(t, "Ordinary project before and after triage")
	create := func(title string) IssueResponse {
		issue := testutil.Decode[IssueResponse](t, testHandler.CreateIssue, newRequest("POST", "/api/issues", map[string]any{"title": title, "project_id": project, "status": "todo", "assignee_type": "member", "assignee_id": testUserID}), 201)
		t.Cleanup(func() { deleteTestIssue(t, issue.ID) })
		return issue
	}
	before := create("Ordinary work before triage")
	// Register the shared cleanup before exercising the real administrative API.
	triageEnableForTest(t)
	dbfx.Exec(t, `UPDATE workspace_triage_settings SET enabled=false WHERE workspace_id=$1`, testWorkspaceID)
	admin := dbfx.User(t, "Triage enabling admin", "triage-enabling-admin@example.test")
	dbfx.Member(t, testWorkspaceID, admin, "admin")
	settings = testutil.Decode[TriageSettings](t, testHandler.GetTriageSettings, newRequest("GET", "/api/triage/settings", nil), 200)
	req := newRequest("PUT", "/api/triage/settings", map[string]any{"enabled": true, "acceptance_status": "todo", "responsibility_mode": "none", "expected_revision": settings.Revision})
	req.Header.Set("X-User-ID", admin)
	enabled := testutil.Decode[TriageSettings](t, testHandler.UpdateTriageSettings, req, 200)
	if !enabled.Enabled {
		t.Fatal("admin enable did not persist")
	}
	after := create("Ordinary work after triage")
	for _, issue := range []IssueResponse{before, after} {
		live := testutil.Decode[IssueResponse](t, testHandler.GetIssue, withURLParam(newRequest("GET", "/api/issues/"+issue.ID, nil), "id", issue.ID), 200)
		if live.AdmissionStatus != "not_required" || live.Status != "todo" || live.ProjectID == nil || *live.ProjectID != project || live.AssigneeID == nil || *live.AssigneeID != testUserID {
			t.Fatalf("ordinary project workflow changed: %+v", live)
		}
		if n := dbfx.Count(t, `SELECT count(*) FROM issue_triage WHERE issue_id=$1`, issue.ID); n != 0 {
			t.Fatal("ordinary issue was backfilled into triage")
		}
	}
	queue := testutil.Decode[triageAcceptanceList](t, testHandler.ListTriageItems, newRequest("GET", "/api/triage/items?view=all", nil), 200)
	if queue.Total != 0 || queue.Counts.Pending != 0 {
		t.Fatalf("enabling triage backfilled ordinary work: %+v", queue)
	}
}

func TestTriageAcceptanceSnoozedCommentKeepsSharedReadinessAndExecutionFence(t *testing.T) {
	triageEnableForTest(t)
	reviewer := dbfx.User(t, "Snoozed comment author", "triage-snoozed-comment@example.test")
	dbfx.Member(t, testWorkspaceID, reviewer, "member")
	dbfx.Exec(t, `UPDATE workspace_triage_settings SET responsibility_mode='assign',responsibility_member_id=$2 WHERE workspace_id=$1`, testWorkspaceID, testUserID)
	input := triageInputForTest()
	input["due_date"] = "2026-12-20"
	item := testutil.Decode[TriageItem](t, testHandler.CreateTriageItem, newRequest("POST", "/api/triage/items", input), 201)
	snoozed := triageActionForTest(t, item, "snooze", map[string]any{"snoozed_until": time.Now().Add(2 * time.Hour).Format(time.RFC3339Nano)}, 200)
	h := *testHandler
	h.Bus = events.New()
	var comments []events.Event
	h.Bus.Subscribe(protocol.EventCommentCreated, func(e events.Event) { comments = append(comments, e) })
	agent := dbfx.Agent(t, "Snoozed mention cannot execute", testRuntimeID)
	content := "More detail [@Agent](mention://agent/" + agent + ")"
	req := withURLParam(newRequest("POST", "/api/issues/comments", map[string]any{"content": content}), "id", item.Issue.ID)
	req.Header.Set("X-User-ID", reviewer)
	comment := testutil.Decode[CommentResponse](t, h.CreateComment, req, 201)
	if comment.Content != content || len(comments) != 1 || comments[0].ActorID != reviewer {
		t.Fatalf("ordinary comment event lost: %+v %+v", comment, comments)
	}
	var payload struct {
		SuppressExecution bool `json:"suppress_execution"`
	}
	raw, err := json.Marshal(comments[0].Payload)
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &payload); err != nil || !payload.SuppressExecution {
		t.Fatalf("pending comment did not retain downstream execution fence: %s, %v", raw, err)
	}
	for _, user := range []string{testUserID, reviewer} {
		for _, view := range []string{"ready", "all", "snoozed"} {
			req = newRequest("GET", "/api/triage/items?view="+view, nil)
			req.Header.Set("X-User-ID", user)
			list := testutil.Decode[triageAcceptanceList](t, testHandler.ListTriageItems, req, 200)
			want := 1
			if view == "ready" {
				want = 0
			}
			if list.Total != want || len(list.Items) != want || list.Counts.Pending != 1 || list.Counts.Ready != 0 || list.Counts.Snoozed != 1 {
				t.Fatalf("member %s view %s disagrees on global snooze: %+v", user, view, list)
			}
		}
	}
	live := testutil.Decode[TriageItem](t, testHandler.GetTriageItem, withURLParam(newRequest("GET", "/api/triage/items", nil), "id", item.Issue.ID), 200)
	if live.Issue.AdmissionStatus != "pending" || live.SnoozedUntil == nil || *live.SnoozedUntil != *snoozed.Item.SnoozedUntil || live.EnteredAt != item.EnteredAt || !reflect.DeepEqual(live.Issue.DueDate, item.Issue.DueDate) {
		t.Fatalf("comment changed admission, snooze or deadline: %+v", live)
	}
	if n := dbfx.Count(t, `SELECT count(*) FROM comment WHERE id=$1 AND dispatch_eligible=false`, comment.ID); n != 1 {
		t.Fatal("pending-era comment was not durably ineligible for delayed execution")
	}
	if n := dbfx.Count(t, `SELECT count(*) FROM inbox_item WHERE issue_id=$1 AND type='triage'`, item.Issue.ID); n != 0 {
		t.Fatal("comment or future snooze generated a premature review notification")
	}
	if n := dbfx.Count(t, `SELECT count(*) FROM triage_action WHERE issue_id=$1`, item.Issue.ID); n != 1 {
		t.Fatal("comment generated another review action")
	}
	if n := dbfx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id=$1`, item.Issue.ID); n != 0 {
		t.Fatal("snoozed comment dispatched agent execution")
	}
}

func TestTriageAcceptanceInboxStateIsIndependent(t *testing.T) {
	triageEnableForTest(t)
	reviewer := dbfx.User(t, "Inbox triage reviewer", "triage-inbox-reviewer@example.test")
	dbfx.Member(t, testWorkspaceID, reviewer, "member")
	dbfx.Exec(t, `UPDATE workspace_triage_settings SET responsibility_mode='assign',responsibility_member_id=$2 WHERE workspace_id=$1`, testWorkspaceID, reviewer)
	item := triageCreateForTest(t)
	snoozed := triageActionForTest(t, item, "snooze", map[string]any{"snoozed_until": time.Now().Add(time.Hour).Format(time.RFC3339Nano)}, 200)
	var notification string
	dbfx.QueryRow(t, `SELECT id FROM inbox_item WHERE issue_id=$1 AND recipient_id=$2 AND type='triage'`, item.Issue.ID, reviewer).Scan(&notification)
	for _, operation := range []struct {
		name           string
		handler        http.HandlerFunc
		read, archived bool
	}{{"read", testHandler.MarkInboxRead, true, false}, {"archive", testHandler.ArchiveInboxItem, true, true}, {"unread", testHandler.MarkInboxUnread, false, true}, {"unarchive", testHandler.UnarchiveInboxItem, false, false}} {
		t.Run(operation.name, func(t *testing.T) {
			req := withURLParam(newRequest("POST", "/api/inbox/"+notification+"/"+operation.name, nil), "id", notification)
			req.Header.Set("X-User-ID", reviewer)
			notice := testutil.Decode[InboxItemResponse](t, inboxWorkspaceHandler(operation.handler), req, 200)
			if notice.Read != operation.read || notice.Archived != operation.archived {
				t.Fatalf("inbox operation did not take effect: %+v", notice)
			}
			live := testutil.Decode[TriageItem](t, testHandler.GetTriageItem, withURLParam(newRequest("GET", "/api/triage/items", nil), "id", item.Issue.ID), 200)
			if !reflect.DeepEqual(live, snoozed.Item) {
				t.Fatalf("%s changed persisted triage: before=%+v after=%+v", operation.name, snoozed.Item, live)
			}
		})
	}
}

func TestTriageAcceptanceResponsibilityModesAndDeparture(t *testing.T) {
	triageEnableForTest(t)
	reviewer := dbfx.User(t, "Departing triage reviewer", "triage-departing-reviewer@example.test")
	member := dbfx.Member(t, testWorkspaceID, reviewer, "member")
	other := dbfx.User(t, "Replacement triage reviewer", "triage-replacement-reviewer@example.test")
	dbfx.Member(t, testWorkspaceID, other, "member")
	agent := dbfx.Agent(t, "Independent proposed executor", testRuntimeID)
	var assigned TriageItem
	for _, mode := range []string{"none", "notify", "assign"} {
		dbfx.Exec(t, `UPDATE workspace_triage_settings SET responsibility_mode=$2,responsibility_member_id=$3 WHERE workspace_id=$1`, testWorkspaceID, mode, reviewer)
		input := triageInputForTest()
		input["candidate_assignee_type"], input["candidate_assignee_id"] = "agent", agent
		item := testutil.Decode[TriageItem](t, testHandler.CreateTriageItem, newRequest("POST", "/api/triage/items", input), 201)
		if (item.ReviewerID != nil) != (mode == "assign") || item.Issue.AssigneeID != nil || item.CandidateAssigneeID == nil || *item.CandidateAssigneeID != agent {
			t.Fatalf("%s conflated follow-up and execution responsibility: %+v", mode, item)
		}
		want := 1
		if mode == "none" {
			want = 0
		}
		if n := dbfx.Count(t, `SELECT count(*) FROM inbox_item WHERE issue_id=$1 AND recipient_id=$2 AND type='triage'`, item.Issue.ID, reviewer); n != want {
			t.Fatalf("mode %s notifications=%d want %d", mode, n, want)
		}
		if mode == "assign" {
			assigned = item
		}
	}
	dbfx.Exec(t, `UPDATE workspace_triage_settings SET responsibility_mode='assign',responsibility_member_id=$2 WHERE workspace_id=$1`, testWorkspaceID, other)
	live := testutil.Decode[TriageItem](t, testHandler.GetTriageItem, withURLParam(newRequest("GET", "/api/triage/items", nil), "id", assigned.Issue.ID), 200)
	if live.ReviewerID == nil || *live.ReviewerID != reviewer {
		t.Fatal("configuration change rewrote an existing round's reviewer")
	}
	testutil.Call(t, testHandler.DeleteMember, testutil.WithURLParams(newRequest("DELETE", "/api/workspaces/members", nil), "id", testWorkspaceID, "memberId", member)).Want(204)
	live = testutil.Decode[TriageItem](t, testHandler.GetTriageItem, withURLParam(newRequest("GET", "/api/triage/items", nil), "id", assigned.Issue.ID), 200)
	if live.ReviewerID == nil || *live.ReviewerID != reviewer || live.ReviewerValid || live.Issue.AdmissionStatus != "pending" || live.CandidateAssigneeID == nil || *live.CandidateAssigneeID != agent {
		t.Fatalf("departure destroyed review state or executor proposal: %+v", live)
	}
	req := withURLParam(newRequest("POST", "/api/triage/items/actions", map[string]any{"request_id": uuidToString(dbid.NewV7()), "expected_revision": live.Issue.Revision, "action": "assign_reviewer", "reviewer_id": other}), "id", live.Issue.ID)
	req.Header.Set("X-User-ID", other)
	reassigned := testutil.Decode[TriageActionResult](t, testHandler.ActOnTriageItem, req, 200)
	if reassigned.Item.ReviewerID == nil || *reassigned.Item.ReviewerID != other || !reassigned.Item.ReviewerValid || reassigned.Item.Issue.AssigneeID != nil || reassigned.Item.CandidateAssigneeID == nil || *reassigned.Item.CandidateAssigneeID != agent {
		t.Fatalf("ordinary member could not reassign independently: %+v", reassigned)
	}
	req = withURLParam(newRequest("POST", "/api/triage/items/actions", map[string]any{"request_id": uuidToString(dbid.NewV7()), "expected_revision": reassigned.Item.Issue.Revision, "action": "reject", "reason": "Member can finish review after departure"}), "id", live.Issue.ID)
	req.Header.Set("X-User-ID", other)
	result := testutil.Decode[TriageActionResult](t, testHandler.ActOnTriageItem, req, 200)
	if result.Item.Issue.AdmissionStatus != "rejected" {
		t.Fatal("reviewer departure blocked another ordinary member's review")
	}
}

func TestTriageAcceptanceBatchReportsMixedOutcomesAndRetriesOnlySelectedFailures(t *testing.T) {
	triageEnableForTest(t)
	member := dbfx.User(t, "Batch ordinary reviewer", "triage-batch-ordinary@example.test")
	dbfx.Member(t, testWorkspaceID, member, "member")
	agent := dbfx.Agent(t, "Revoked batch candidate", testRuntimeID, testutil.Cols{"permission_mode": "public_to", "visibility": "workspace"})
	success, conflict, invalid, untouched := triageCreateForTest(t), triageCreateForTest(t), triageCreateForTest(t), triageCreateForTest(t)
	input := triageInputForTest()
	input["candidate_assignee_type"], input["candidate_assignee_id"] = "agent", agent
	forbidden := testutil.Decode[TriageItem](t, testHandler.CreateTriageItem, newRequest("POST", "/api/triage/items", input), 201)
	makeRow := func(item TriageItem) map[string]any {
		return map[string]any{"issue_id": item.Issue.ID, "request_id": uuidToString(dbid.NewV7()), "expected_revision": item.Issue.Revision, "action": "accept"}
	}
	rows := []map[string]any{makeRow(success), makeRow(conflict), makeRow(forbidden), makeRow(invalid)}
	rows[3]["fields"] = map[string]any{"due_date": "not-a-date"}
	// Preview uses only selection identities/revisions, not per-item action payloads.
	preview := map[string]any{"action": "accept", "items": []any{map[string]any{"issue_id": success.Issue.ID, "expected_revision": success.Issue.Revision}, map[string]any{"issue_id": conflict.Issue.ID, "expected_revision": conflict.Issue.Revision}}}
	req := newRequest("POST", "/api/triage/batch/preview", preview)
	req.Header.Set("X-User-ID", member)
	preflight := testutil.Decode[struct {
		Valid int `json:"valid_count"`
	}](t, testHandler.PreviewTriageBatch, req, 200)
	if preflight.Valid != 2 {
		t.Fatal("valid selections did not pass preflight")
	}
	changed := triageActionForTest(t, conflict, "assign_reviewer", map[string]any{"reviewer_id": member}, 200)
	dbfx.Exec(t, `UPDATE agent SET permission_mode='private',visibility='private' WHERE id=$1`, agent)
	type batchResponse struct {
		Results []struct {
			IssueID string              `json:"issue_id"`
			Status  string              `json:"status"`
			Result  *TriageActionResult `json:"result"`
			Error   string              `json:"error"`
		} `json:"results"`
		Success int `json:"success_count"`
	}
	call := func(selected []map[string]any) batchResponse {
		req := newRequest("POST", "/api/triage/batch", map[string]any{"items": selected})
		req.Header.Set("X-User-ID", member)
		return testutil.Decode[batchResponse](t, testHandler.CommitTriageBatch, req, 200)
	}
	first := call(rows)
	if first.Success != 1 || len(first.Results) != 4 {
		t.Fatalf("bad mixed result: %+v", first)
	}
	for i, status := range []string{"success", "conflict", "forbidden", "invalid"} {
		if first.Results[i].Status != status || first.Results[i].IssueID != rows[i]["issue_id"] {
			t.Fatalf("row %d should be %s: %+v", i, status, first.Results[i])
		}
		if status != "success" && first.Results[i].Error == "" {
			t.Fatal("failed row has no actionable result")
		}
	}
	if first.Results[0].Result == nil {
		t.Fatal("successful row lost result receipt")
	}
	for _, item := range []TriageItem{conflict, forbidden, invalid, untouched} {
		if n := dbfx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND admission_status='pending'`, item.Issue.ID); n != 1 {
			t.Fatal("failure or unselected issue partially admitted")
		}
	}
	// Preserve the failed intention's request ID; only refresh its CAS revision.
	rows[1]["expected_revision"] = changed.Item.Issue.Revision
	retry := call(rows[1:2])
	if retry.Success != 1 || len(retry.Results) != 1 || retry.Results[0].IssueID != conflict.Issue.ID {
		t.Fatalf("selected-only retry failed: %+v", retry)
	}
	for _, item := range []TriageItem{forbidden, invalid, untouched} {
		if n := dbfx.Count(t, `SELECT count(*) FROM triage_action WHERE issue_id=$1`, item.Issue.ID); n != 0 {
			t.Fatal("retry expanded beyond selected failure")
		}
	}
	if n := dbfx.Count(t, `SELECT count(*) FROM triage_action WHERE issue_id=$1 AND id=$2`, success.Issue.ID, first.Results[0].Result.Action.ID); n != 1 {
		t.Fatal("retry replaced prior successful receipt")
	}
	if n := dbfx.Count(t, `SELECT count(*) FROM triage_action WHERE issue_id=$1`, success.Issue.ID); n != 1 {
		t.Fatal("retry repeated prior successful action")
	}
	if n := dbfx.Count(t, `SELECT count(*) FROM triage_action WHERE issue_id=$1 AND request_id=$2`, conflict.Issue.ID, rows[1]["request_id"]); n != 1 {
		t.Fatal("retry lost original failed request identity")
	}
}

func TestTriageAcceptanceRevokedMemberCannotOpenNotificationTargetsOrAttachments(t *testing.T) {
	triageEnableForTest(t)
	user := dbfx.User(t, "Revoked triage submitter", "triage-revoked-submitter@example.test")
	member := dbfx.Member(t, testWorkspaceID, user, "member")
	req := newRequest("POST", "/api/triage/items", triageInputForTest())
	req.Header.Set("X-User-ID", user)
	item := testutil.Decode[TriageItem](t, testHandler.CreateTriageItem, req, 201)
	target := dbfx.Issue(t, "Confidential formal duplicate target")
	duplicate := triageActionForTest(t, item, "duplicate", map[string]any{"duplicate_issue_id": target}, 200)
	var notification string
	dbfx.QueryRow(t, `SELECT id FROM inbox_item WHERE issue_id=$1 AND recipient_id=$2 AND type='triage'`, item.Issue.ID, user).Scan(&notification)
	store := &mockStorage{}
	content := []byte("Confidential triage attachment body")
	attachmentURL, err := store.Upload(context.Background(), "triage-confidential.txt", content, "text/plain", "triage-confidential.txt")
	if err != nil {
		t.Fatal(err)
	}
	attachment := dbfx.Insert(t, "attachment", testutil.Cols{"workspace_id": testWorkspaceID, "issue_id": item.Issue.ID, "uploader_type": "member", "uploader_id": testUserID, "filename": "triage-confidential.txt", "url": attachmentURL, "content_type": "text/plain", "size_bytes": len(content)})
	h := *testHandler
	h.Storage = store
	h.cfg.AttachmentDownloadMode = "proxy"
	h.CFSigner = nil
	req = withURLParam(newRequest("GET", "/api/attachments/"+attachment+"/download", nil), "id", attachment)
	req.Header.Set("X-User-ID", user)
	if body := testutil.Call(t, h.DownloadAttachment, req).Want(200).Body.String(); body != string(content) {
		t.Fatal("authorized attachment control failed")
	}
	testutil.Call(t, testHandler.DeleteMember, testutil.WithURLParams(newRequest("DELETE", "/api/workspaces/members", nil), "id", testWorkspaceID, "memberId", member)).Want(204)
	for _, check := range []struct {
		name    string
		handler http.HandlerFunc
		id      string
		status  int
	}{
		{"notification", inboxWorkspaceHandler(testHandler.MarkInboxRead), notification, 404},
		{"inbox", inboxWorkspaceHandler(testHandler.ListInbox), "", 404},
		{"unread-count", inboxWorkspaceHandler(testHandler.CountUnreadInbox), "", 404},
		{"triage-detail", testHandler.GetTriageItem, item.Issue.ID, 403},
		{"triage-history", testHandler.GetTriageItemHistory, item.Issue.ID, 403},
		{"triage-queue", testHandler.ListTriageItems, "", 403},
		{"global-history", testHandler.GetTriageHistory, "", 403},
		{"original-issue", inboxWorkspaceHandler(testHandler.GetIssue), item.Issue.ID, 404},
		{"duplicate-target", inboxWorkspaceHandler(testHandler.GetIssue), *duplicate.Item.DuplicateIssueID, 404},
		{"attachment-metadata", inboxWorkspaceHandler(h.GetAttachmentByID), attachment, 404},
		{"attachment-download", h.DownloadAttachment, attachment, 404},
	} {
		t.Run(check.name, func(t *testing.T) {
			req := withURLParam(newRequest("GET", "/api/"+check.name, nil), "id", check.id)
			req.Header.Set("X-User-ID", user)
			body := testutil.Call(t, check.handler, req).Want(check.status).Body.String()
			for _, secret := range []string{item.Issue.Title, "Confidential formal duplicate target", attachmentURL, string(content), "\"counts\"", "\"entries\""} {
				if strings.Contains(body, secret) {
					t.Fatalf("revoked response disclosed %q: %s", secret, body)
				}
			}
		})
	}
	if store.getReaderCalls != 1 {
		t.Fatal("revoked attachment request reached object storage")
	}
}

func TestTriageAcceptanceFinishedSnoozeCannotNotifyOrReenter(t *testing.T) {
	triageEnableForTest(t)
	dbfx.Exec(t, `UPDATE workspace_triage_settings SET responsibility_mode='assign',responsibility_member_id=$2 WHERE workspace_id=$1`, testWorkspaceID, testUserID)
	item := triageCreateForTest(t)
	snoozed := triageActionForTest(t, item, "snooze", map[string]any{"snoozed_until": time.Now().Add(time.Hour).Format(time.RFC3339Nano)}, 200)
	accepted := triageActionForTest(t, snoozed.Item, "accept", nil, 200)
	dbfx.Exec(t, `UPDATE triage_notification SET due_at=now()-interval '1 minute' WHERE event_key=$1`, "snooze:"+snoozed.Action.ID)
	for range 2 {
		testHandler.deliverTriageNotifications(context.Background(), parseUUID(testWorkspaceID))
	}
	queue := testutil.Decode[triageAcceptanceList](t, testHandler.ListTriageItems, newRequest("GET", "/api/triage/items?view=all", nil), 200)
	if accepted.Item.SnoozedUntil != nil || queue.Total != 0 || queue.Counts.Pending != 0 {
		t.Fatalf("accepted snooze reentered: %+v %+v", accepted.Item, queue)
	}
	if n := dbfx.Count(t, `SELECT count(*) FROM inbox_item WHERE issue_id=$1 AND details->>'triage_event'=$2`, item.Issue.ID, "snooze:"+snoozed.Action.ID); n != 0 {
		t.Fatal("finished snooze delivered stale review reminder")
	}
	if n := dbfx.Count(t, `SELECT count(*) FROM triage_action WHERE issue_id=$1`, item.Issue.ID); n != 2 {
		t.Fatal("due delivery manufactured a review action")
	}
}

func TestTriageAcceptanceTwoMembersRaceHasOneDecisionAndOneResultNotice(t *testing.T) {
	triageEnableForTest(t)
	item := triageCreateForTest(t)
	users := []string{dbfx.User(t, "First racing reviewer", "triage-racer-one@example.test"), dbfx.User(t, "Second racing reviewer", "triage-racer-two@example.test")}
	for _, user := range users {
		dbfx.Member(t, testWorkspaceID, user, "member")
	}
	type outcome struct {
		user   string
		result TriageActionResult
		err    error
	}
	start, done := make(chan struct{}), make(chan outcome, 2)
	for i, user := range users {
		go func(i int, user string) {
			<-start
			req := newRequest("POST", "/api/triage/items/actions", nil)
			req.Header.Set("X-User-ID", user)
			result, err := testHandler.actOnTriageItem(req, item.Issue.ID, TriageActionInput{RequestID: uuidToString(dbid.NewV7()), ExpectedRevision: item.Issue.Revision, Action: []string{"accept", "reject"}[i], Reason: "Independent concurrent review"}, false)
			done <- outcome{user, result, err}
		}(i, user)
	}
	close(start)
	wins := 0
	var winning outcome
	for range 2 {
		o := <-done
		if o.err == nil {
			wins++
			winning = o
			continue
		}
		var e *triageError
		if !errors.As(o.err, &e) || e.Status != 409 {
			t.Fatalf("loser should receive revision conflict: %v", o.err)
		}
	}
	if wins != 1 {
		t.Fatalf("concurrent members committed %d decisions", wins)
	}
	testHandler.deliverTriageNotifications(context.Background(), parseUUID(testWorkspaceID))
	if n := dbfx.Count(t, `SELECT count(*) FROM triage_action WHERE issue_id=$1`, item.Issue.ID); n != 1 {
		t.Fatal("race recorded duplicate decisions")
	}
	if n := dbfx.Count(t, `SELECT count(*) FROM inbox_item WHERE issue_id=$1 AND type='triage' AND recipient_id=$2 AND details->>'actor_id'=$3`, item.Issue.ID, testUserID, winning.user); n != 1 {
		t.Fatal("creator did not receive exactly the winning member's result")
	}
	if n := dbfx.Count(t, `SELECT count(*) FROM inbox_item WHERE issue_id=$1 AND type='triage'`, item.Issue.ID); n != 1 {
		t.Fatal("race sent extra result notices")
	}
	if n := dbfx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id=$1`, item.Issue.ID); n != 0 {
		t.Fatal("race implicitly dispatched execution")
	}
	current := testutil.Decode[TriageItem](t, testHandler.GetTriageItem, withURLParam(newRequest("GET", "/api/triage/items", nil), "id", item.Issue.ID), 200)
	if current.Issue.AdmissionStatus != winning.result.Item.Issue.AdmissionStatus {
		t.Fatal("losing decision overwrote winner")
	}
}

func TestTriageAcceptanceDisabledReopenKeepsRoundAndReadableHistory(t *testing.T) {
	// A private workspace makes the disable precondition independent of other
	// tests' unresolved inputs in the suite's shared workspace.
	workspace := dbfx.Workspace(t, "Disabled triage history", "triage-disabled-history-"+uuidToString(dbid.NewV7()))
	dbfx.Member(t, workspace, testUserID, "owner")
	t.Cleanup(func() {
		testutil.Call(t, testHandler.DeleteWorkspace, withURLParam(newRequest("DELETE", "/api/workspaces/"+workspace, nil), "id", workspace)).Want(204)
	})
	scoped := func(handler http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			r.Header.Set("X-Workspace-ID", workspace)
			handler(w, r)
		}
	}
	settings := testutil.Decode[TriageSettings](t, scoped(testHandler.GetTriageSettings), newRequest("GET", "/api/triage/settings", nil), 200)
	testutil.Call(t, scoped(testHandler.UpdateTriageSettings), newRequest("PUT", "/api/triage/settings", map[string]any{"enabled": true, "acceptance_status": "todo", "responsibility_mode": "none", "expected_revision": settings.Revision})).Want(200)
	first := testutil.Decode[TriageItem](t, scoped(testHandler.CreateTriageItem), newRequest("POST", "/api/triage/items", triageInputForTest()), 201)
	second := testutil.Decode[TriageItem](t, scoped(testHandler.CreateTriageItem), newRequest("POST", "/api/triage/items", triageInputForTest()), 201)
	target := dbfx.Issue(t, "Formal disabled-reopen duplicate target", testutil.Cols{"workspace_id": workspace})
	rejected := testutil.Decode[TriageActionResult](t, scoped(testHandler.ActOnTriageItem), withURLParam(newRequest("POST", "/api/triage/items/actions", map[string]any{"request_id": uuidToString(dbid.NewV7()), "expected_revision": first.Issue.Revision, "action": "reject", "reason": "Retain first rejection"}), "id", first.Issue.ID), 200)
	duplicate := testutil.Decode[TriageActionResult](t, scoped(testHandler.ActOnTriageItem), withURLParam(newRequest("POST", "/api/triage/items/actions", map[string]any{"request_id": uuidToString(dbid.NewV7()), "expected_revision": second.Issue.Revision, "action": "duplicate", "duplicate_issue_id": target}), "id", second.Issue.ID), 200)
	settings = testutil.Decode[TriageSettings](t, scoped(testHandler.GetTriageSettings), newRequest("GET", "/api/triage/settings", nil), 200)
	testutil.Call(t, scoped(testHandler.UpdateTriageSettings), newRequest("PUT", "/api/triage/settings", map[string]any{"enabled": false, "acceptance_status": "todo", "responsibility_mode": "none", "expected_revision": settings.Revision})).Want(200)
	for _, result := range []TriageActionResult{rejected, duplicate} {
		request := map[string]any{"request_id": uuidToString(dbid.NewV7()), "expected_revision": result.Item.Issue.Revision, "action": "reopen", "reason": "Reopen must await enable"}
		for range 2 {
			body := testutil.Call(t, scoped(testHandler.ActOnTriageItem), withURLParam(newRequest("POST", "/api/triage/items/actions", request), "id", result.Item.Issue.ID)).Want(409).Body.String()
			if !strings.Contains(body, "enable triage") {
				t.Fatalf("disabled reopen lacks recovery instruction: %s", body)
			}
		}
		live := testutil.Decode[TriageItem](t, scoped(testHandler.GetTriageItem), withURLParam(newRequest("GET", "/api/triage/items", nil), "id", result.Item.Issue.ID), 200)
		if !reflect.DeepEqual(live, result.Item) {
			t.Fatal("disabled reopen changed result, round or revision")
		}
		history := testutil.Decode[struct {
			Events []TriageAction `json:"events"`
		}](t, scoped(testHandler.GetTriageItemHistory), withURLParam(newRequest("GET", "/api/triage/items/history", nil), "id", result.Item.Issue.ID), 200)
		if len(history.Events) != 1 || history.Events[0].ID != result.Action.ID {
			t.Fatal("disabled history lost original decision or recorded denied reopen")
		}
	}
	history := testutil.Decode[triageAcceptanceHistory](t, scoped(testHandler.GetTriageHistory), newRequest("GET", "/api/triage/history", nil), 200)
	if history.Total != 2 || len(history.Entries) != 2 {
		t.Fatal("disabled global history is unavailable")
	}
}

func TestTriageAcceptanceTitleSimilarityWarnsWithoutDroppingInputs(t *testing.T) {
	triageImportSetup(t)
	target := testutil.Decode[IssueResponse](t, testHandler.CreateIssue, newRequest("POST", "/api/issues", map[string]any{"title": "Similar feedback retained"}), 201)
	t.Cleanup(func() { deleteTestIssue(t, target.ID) })
	preview := triageImportPreviewForTest(t, "title,external_id\nSimilar feedback retained,one\nSimilar feedback retained,two\n")
	if len(preview.Rows) != 2 {
		t.Fatal("same title dropped a preview row")
	}
	for _, row := range preview.Rows {
		if row.Duplicate || !slices.Contains(row.SimilarIssueIDs, target.ID) || len(row.Warnings) == 0 {
			t.Fatalf("similar title was not a nonblocking candidate: %+v", row)
		}
	}
	result := triageImportCommitForTest(t, preview.BatchID, map[string]any{"row_number": 1}, map[string]any{"row_number": 2})
	if result.Created != 2 || result.Skipped != 0 || result.Failed != 0 || len(result.Results) != 2 || result.Results[0].IssueID == nil || result.Results[1].IssueID == nil || *result.Results[0].IssueID == *result.Results[1].IssueID {
		t.Fatalf("same-title inputs did not both survive: %+v", result)
	}
	if n := dbfx.Count(t, `SELECT count(*) FROM issue WHERE workspace_id=$1 AND title='Similar feedback retained'`, testWorkspaceID); n != 3 {
		t.Fatal("similarity silently deduplicated an input or replaced formal target")
	}
	history := testutil.Decode[triageAcceptanceHistory](t, testHandler.GetTriageHistory, newRequest("GET", "/api/triage/history?result=import&source=csv&processed_by="+testUserID+"&q=incoming.csv", nil), 200)
	if history.Total != 1 || len(history.Entries) != 1 || history.Entries[0].BatchID == nil || *history.Entries[0].BatchID != preview.BatchID || history.Entries[0].Counts["created"] != 2 {
		t.Fatalf("CSV summary is missing from filtered global history: %+v", history)
	}
}

func TestTriageAcceptanceQueueFiltersStableSortAndGlobalCounts(t *testing.T) {
	triageEnableForTest(t)
	project := dbfx.Project(t, "Queue filter project")
	reviewer := dbfx.User(t, "Queue filter reviewer", "triage-filter-reviewer@example.test")
	dbfx.Member(t, testWorkspaceID, reviewer, "member")
	label := dbfx.Insert(t, "issue_label", testutil.Cols{"workspace_id": testWorkspaceID, "name": "Triage queue label", "color": "#123456", "resource_type": "issue"})
	var items []TriageItem
	for i, priority := range []string{"low", "urgent", "urgent"} {
		input := triageInputForTest()
		input["title"], input["priority"] = "Queue filter needle "+string(rune('A'+i)), priority
		if i == 0 {
			input["candidate_project_id"], input["label_ids"] = project, []string{label}
		}
		req := newRequest("POST", "/api/triage/items", input)
		if i == 0 {
			req.Header.Set("X-User-ID", reviewer)
		}
		item := testutil.Decode[TriageItem](t, testHandler.CreateTriageItem, req, 201)
		dbfx.Exec(t, `UPDATE issue_triage SET entered_at='2026-10-01T12:00:00Z',reviewer_id=$2 WHERE issue_id=$1`, item.Issue.ID, reviewer)
		items = append(items, item)
	}
	dbfx.Exec(t, `UPDATE issue_triage SET source='csv',reviewer_id=NULL,snoozed_until=now()+interval '1 day' WHERE issue_id=$1`, items[2].Issue.ID)
	foreignWS := dbfx.Workspace(t, "Foreign filter scope", "foreign-filter-"+uuidToString(dbid.NewV7()))
	foreign := dbfx.Issue(t, "Queue filter needle foreign", testutil.Cols{"workspace_id": foreignWS, "admission_status": "pending"})
	dbfx.InsertNoID(t, "issue_triage", testutil.Cols{"workspace_id": foreignWS, "issue_id": foreign}, "issue_id=$1", foreign)
	checks := []struct {
		query string
		want  []string
	}{
		{"source=csv", []string{items[2].Issue.ID}}, {"priority=low", []string{items[0].Issue.ID}}, {"project_id=" + project, []string{items[0].Issue.ID}},
		{"project_id=none", []string{items[1].Issue.ID, items[2].Issue.ID}}, {"label_id=" + label, []string{items[0].Issue.ID}}, {"reviewer_id=" + reviewer, []string{items[0].Issue.ID, items[1].Issue.ID}},
		{"reviewer_id=none", []string{items[2].Issue.ID}}, {"creator_id=" + reviewer, []string{items[0].Issue.ID}}, {"q=" + url.QueryEscape(items[1].Issue.Identifier), []string{items[1].Issue.ID}},
		{"q=needle&source=manual&priority=low&project_id=" + project + "&label_id=" + label + "&reviewer_id=" + reviewer + "&creator_id=" + reviewer, []string{items[0].Issue.ID}},
		{"q=missing", []string{}}, {"entered_after=2026-10-02", []string{}},
	}
	for _, check := range checks {
		list := testutil.Decode[triageAcceptanceList](t, testHandler.ListTriageItems, newRequest("GET", "/api/triage/items?view=all&"+check.query, nil), 200)
		ids := []string{}
		for _, item := range list.Items {
			ids = append(ids, item.Issue.ID)
		}
		if !reflect.DeepEqual(ids, check.want) || list.Total != len(check.want) {
			t.Fatalf("filter %s: got %v total %d, want %v", check.query, ids, list.Total, check.want)
		}
		if list.Counts.Pending != 3 || list.Counts.Ready != 2 || list.Counts.Snoozed != 1 {
			t.Fatalf("filter %s changed global counts or leaked foreign workspace: %+v", check.query, list.Counts)
		}
	}
	for _, check := range []struct {
		sort string
		ids  []string
	}{{"oldest", []string{items[0].Issue.ID, items[1].Issue.ID, items[2].Issue.ID}}, {"newest", []string{items[2].Issue.ID, items[1].Issue.ID, items[0].Issue.ID}}, {"priority", []string{items[1].Issue.ID, items[2].Issue.ID, items[0].Issue.ID}}} {
		for page, want := range check.ids {
			list := testutil.Decode[triageAcceptanceList](t, testHandler.ListTriageItems, newRequest("GET", "/api/triage/items?view=all&sort="+check.sort+"&limit=1&offset="+strconv.Itoa(page), nil), 200)
			if list.Total != 3 || len(list.Items) != 1 || list.Items[0].Issue.ID != want {
				t.Fatalf("%s tie pagination %d unstable: %+v", check.sort, page, list)
			}
		}
	}
}

func TestTriageAcceptanceHistoricalFiltersUseDecisionSnapshotsAfterReopen(t *testing.T) {
	triageEnableForTest(t)
	reviewer := dbfx.User(t, "Historical reviewer", "triage-historical-reviewer@example.test")
	dbfx.Member(t, testWorkspaceID, reviewer, "member")
	item := triageCreateForTest(t)
	req := withURLParam(newRequest("POST", "/api/triage/items/actions", map[string]any{"request_id": uuidToString(dbid.NewV7()), "expected_revision": item.Issue.Revision, "action": "reject", "reason": "Round one rejection remains attributable"}), "id", item.Issue.ID)
	req.Header.Set("X-User-ID", reviewer)
	rejected := testutil.Decode[TriageActionResult](t, testHandler.ActOnTriageItem, req, 200)
	reopened := triageActionForTest(t, rejected.Item, "reopen", map[string]any{"reason": "New evidence for round two"}, 200)
	accepted := triageActionForTest(t, reopened.Item, "accept", nil, 200)
	dbfx.Exec(t, `UPDATE triage_action SET created_at='2026-10-01T12:00:00Z' WHERE id=$1`, rejected.Action.ID)
	dbfx.Exec(t, `UPDATE triage_action SET created_at='2026-10-02T12:00:00Z' WHERE id=$1 OR id=$2`, reopened.Action.ID, accepted.Action.ID)
	dbfx.Exec(t, `UPDATE issue SET title='Current title after review' WHERE id=$1`, item.Issue.ID)
	query := "result=rejected&processed_by=" + reviewer + "&source=manual&processed_after=2026-10-01&processed_before=2026-10-01&q=" + url.QueryEscape(item.Issue.Title)
	history := testutil.Decode[triageAcceptanceHistory](t, testHandler.GetTriageHistory, newRequest("GET", "/api/triage/history?"+query, nil), 200)
	if history.Total != 1 || len(history.Entries) != 1 {
		t.Fatalf("old-round filters consulted current result/title: %+v", history)
	}
	entry := history.Entries[0]
	var after TriageItem
	if err := json.Unmarshal(entry.After, &after); err != nil {
		t.Fatal(err)
	}
	if entry.ID != rejected.Action.ID || entry.ActorID != reviewer || entry.Title != item.Issue.Title || entry.Reason == nil || *entry.Reason != "Round one rejection remains attributable" || after.Round != 1 || after.Issue.AdmissionStatus != "rejected" {
		t.Fatalf("history changed prior decision snapshot: %+v %+v", entry, after)
	}
	for _, filter := range []string{"result=rejected&processed_by=" + testUserID, "source=csv", "processed_before=2026-09-30"} {
		out := testutil.Decode[triageAcceptanceHistory](t, testHandler.GetTriageHistory, newRequest("GET", "/api/triage/history?"+filter, nil), 200)
		if out.Total != 0 || len(out.Entries) != 0 {
			t.Fatalf("history ignored filter %s: %+v", filter, out)
		}
	}
	// Reopen and acceptance share a timestamp: stable action identity breaks ties.
	for page, want := range []string{accepted.Action.ID, reopened.Action.ID, rejected.Action.ID} {
		out := testutil.Decode[triageAcceptanceHistory](t, testHandler.GetTriageHistory, newRequest("GET", "/api/triage/history?limit=1&offset="+strconv.Itoa(page), nil), 200)
		if out.Total != 3 || len(out.Entries) != 1 || out.Entries[0].ID != want {
			t.Fatalf("history tie pagination %d unstable: %+v", page, out)
		}
	}
}

func TestTriageAcceptanceRescheduleAndCancelSnoozePreserveAuditAndDates(t *testing.T) {
	triageEnableForTest(t)
	dbfx.Exec(t, `UPDATE workspace_triage_settings SET responsibility_mode='assign',responsibility_member_id=$2 WHERE workspace_id=$1`, testWorkspaceID, testUserID)
	input := triageInputForTest()
	input["due_date"] = "2026-12-20"
	item := testutil.Decode[TriageItem](t, testHandler.CreateTriageItem, newRequest("POST", "/api/triage/items", input), 201)
	for _, invalid := range []string{"invalid", time.Now().Add(-time.Minute).Format(time.RFC3339), time.Now().Add(91 * 24 * time.Hour).Format(time.RFC3339)} {
		triageActionForTest(t, item, "snooze", map[string]any{"snoozed_until": invalid}, 400)
	}
	if n := dbfx.Count(t, `SELECT count(*) FROM triage_action WHERE issue_id=$1`, item.Issue.ID); n != 0 {
		t.Fatal("invalid snooze recorded an action")
	}
	first := triageActionForTest(t, item, "snooze", map[string]any{"snoozed_until": time.Now().Add(time.Hour).Format(time.RFC3339Nano)}, 200)
	reset := triageActionForTest(t, first.Item, "snooze", map[string]any{"snoozed_until": time.Now().Add(2 * time.Hour).Format(time.RFC3339Nano)}, 200)
	if reset.Item.SnoozedUntil == nil || *reset.Item.SnoozedUntil == *first.Item.SnoozedUntil {
		t.Fatal("rescheduling did not replace deadline")
	}
	restored := triageActionForTest(t, reset.Item, "unsnooze", nil, 200)
	if restored.Item.SnoozedUntil != nil || restored.Item.Issue.AdmissionStatus != "pending" || restored.Item.EnteredAt != item.EnteredAt || !reflect.DeepEqual(restored.Item.Issue.DueDate, item.Issue.DueDate) {
		t.Fatalf("cancel changed issue instead of readiness: %+v", restored.Item)
	}
	dbfx.Exec(t, `UPDATE triage_notification SET due_at=now()-interval '1 minute' WHERE issue_id=$1 AND event_key LIKE 'snooze:%'`, item.Issue.ID)
	testHandler.deliverTriageNotifications(context.Background(), parseUUID(testWorkspaceID))
	if n := dbfx.Count(t, `SELECT count(*) FROM inbox_item WHERE issue_id=$1 AND details->>'triage_event' LIKE 'snooze:%'`, item.Issue.ID); n != 0 {
		t.Fatal("cancelled or superseded snooze delivered a reminder")
	}
	history := testutil.Decode[struct {
		Events []TriageAction `json:"events"`
	}](t, testHandler.GetTriageItemHistory, withURLParam(newRequest("GET", "/api/triage/items/history", nil), "id", item.Issue.ID), 200)
	if len(history.Events) != 3 {
		t.Fatalf("snooze audit chain incomplete: %+v", history)
	}
	for index, action := range []string{"snooze", "snooze", "unsnooze"} {
		if history.Events[index].Action != action || history.Events[index].ActorID != testUserID || history.Events[index].CreatedAt == "" {
			t.Fatal("snooze audit lost who, when or transition")
		}
	}
	ready := testutil.Decode[triageAcceptanceList](t, testHandler.ListTriageItems, newRequest("GET", "/api/triage/items?view=ready", nil), 200)
	if ready.Total != 1 || len(ready.Items) != 1 || ready.Items[0].Issue.ID != item.Issue.ID {
		t.Fatal("unsnooze failed to restore original issue")
	}
}

func TestTriageAcceptanceResultNotifiesOnlyActiveCurrentRecipientsOnce(t *testing.T) {
	triageEnableForTest(t)
	item := triageCreateForTest(t)
	actor := dbfx.User(t, "Result actor", "triage-result-actor@example.test")
	watcher := dbfx.User(t, "Result active watcher", "triage-result-active@example.test")
	unsubscribed := dbfx.User(t, "Result former watcher", "triage-result-unsubscribed@example.test")
	departed := dbfx.User(t, "Result departed watcher", "triage-result-departed@example.test")
	for _, user := range []string{actor, watcher, unsubscribed, departed} {
		dbfx.Member(t, testWorkspaceID, user, "member")
	}
	for _, user := range []string{actor, watcher, unsubscribed, departed} {
		dbfx.InsertNoID(t, "issue_subscriber", testutil.Cols{"issue_id": item.Issue.ID, "user_type": "member", "user_id": user, "reason": "manual"}, "issue_id=$1 AND user_id=$2", item.Issue.ID, user)
	}
	dbfx.Exec(t, `UPDATE issue_subscriber SET unsubscribed_at=now() WHERE issue_id=$1 AND user_id=$2`, item.Issue.ID, unsubscribed)
	dbfx.Exec(t, `DELETE FROM member WHERE workspace_id=$1 AND user_id=$2`, testWorkspaceID, departed)
	req := withURLParam(newRequest("POST", "/api/triage/items/actions", map[string]any{"request_id": uuidToString(dbid.NewV7()), "expected_revision": item.Issue.Revision, "action": "reject", "reason": "Notify current followers only"}), "id", item.Issue.ID)
	req.Header.Set("X-User-ID", actor)
	result := testutil.Decode[TriageActionResult](t, testHandler.ActOnTriageItem, req, 200)
	for range 2 {
		testHandler.deliverTriageNotifications(context.Background(), parseUUID(testWorkspaceID))
	}
	for _, user := range []string{testUserID, watcher, actor, unsubscribed, departed} {
		want := 0
		if user == testUserID || user == watcher {
			want = 1
		}
		if n := dbfx.Count(t, `SELECT count(*) FROM inbox_item WHERE issue_id=$1 AND recipient_id=$2 AND details->>'triage_event'=$3`, item.Issue.ID, user, "action:"+result.Action.ID); n != want {
			t.Fatalf("recipient %s result notices=%d want %d", user, n, want)
		}
	}
}
