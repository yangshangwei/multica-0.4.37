package handler

import (
	"net/http"
	"net/url"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
)

type collaborationNotificationHTTPPage struct {
	Items       []service.CollaborationNotificationView `json:"items"`
	UnreadCount int64                                   `json:"unread_count"`
	NextCursor  *string                                 `json:"next_cursor"`
	View        *string                                 `json:"view"`
}

func TestCollaborationNotificationsHTTPFiltersBeforeCursor(t *testing.T) {
	reader := dbfx.User(t, "Filtered notification reader", "filtered-"+uuid.NewString()+"@example.test")
	other := dbfx.User(t, "Other notification recipient", "other-filtered-"+uuid.NewString()+"@example.test")
	issues := []string{collaborationBoundaryIssue(t, testWorkspaceID), collaborationBoundaryIssue(t, testWorkspaceID)}
	for _, issue := range issues {
		dbfx.InsertNoID(t, "issue_participant", testutil.Cols{"issue_id": issue, "user_id": reader}, "issue_id=$1 AND user_id=$2", issue, reader)
	}
	type rowState struct {
		id             string
		created        time.Time
		read, archived bool
	}
	rows := make([]rowState, 0, 12)
	at := time.Date(2026, time.November, 22, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 12; i++ {
		row := rowState{created: at.Add(-time.Duration(i/4) * time.Minute), read: i%4 == 1 || i%4 == 3, archived: i%4 >= 2}
		cols := testutil.Cols{"recipient_user_id": reader, "issue_id": issues[i%2], "event_seq": i + 1,
			"kind": "assigned", "source_kind": "issue", "created_at": row.created}
		if row.read {
			cols["read_at"] = at
		}
		if row.archived {
			cols["archived_at"] = at
		}
		row.id = dbfx.Insert(t, "collaboration_notification", cols)
		rows = append(rows, row)
		dbfx.Insert(t, "collaboration_notification", testutil.Cols{"recipient_user_id": other, "issue_id": issues[i%2], "event_seq": i + 1,
			"kind": "OTHER_RECIPIENT_SENTINEL", "source_kind": "issue", "created_at": row.created.Add(time.Second)})
	}
	sort.Slice(rows, func(i, j int) bool {
		if !rows[i].created.Equal(rows[j].created) {
			return rows[i].created.After(rows[j].created)
		}
		return rows[i].id > rows[j].id
	})
	for _, selector := range []struct {
		query, view string
		include     func(rowState) bool
	}{
		{"view=unread", "unread", func(row rowState) bool { return !row.read && !row.archived }},
		{"view=all", "all", func(row rowState) bool { return !row.archived }},
		{"view=archived", "archived", func(row rowState) bool { return row.archived }},
		{"", "", func(row rowState) bool { return !row.archived }},
		{"include_archived=false", "", func(row rowState) bool { return !row.archived }},
		{"include_archived=true", "", func(row rowState) bool { return true }},
	} {
		t.Run(selector.query, func(t *testing.T) {
			want := make([]string, 0)
			for _, row := range rows {
				if selector.include(row) {
					want = append(want, row.id)
				}
			}
			for _, limit := range []string{"1", "2"} {
				query, err := url.ParseQuery(selector.query)
				if err != nil {
					t.Fatal(err)
				}
				query.Set("limit", limit)
				got := make([]string, 0)
				for pages := 0; ; pages++ {
					if pages > len(rows) {
						t.Fatal("notification cursor did not terminate")
					}
					r := newRequest(http.MethodGet, "/api/me/collaboration-notifications?"+query.Encode(), nil)
					r.Header.Set("X-Workspace-ID", "invalid-ambient-workspace")
					r = r.WithContext(auth.WithAuthenticatedPrincipal(r.Context(), auth.AuthenticatedPrincipal{UserID: reader, Kind: auth.CredentialJWT}))
					var page collaborationNotificationHTTPPage
					result := testutil.Call(t, testHandler.ListCollaborationNotifications, r).Want(http.StatusOK).JSON(&page)
					if page.UnreadCount != 3 {
						t.Fatalf("view %q changed recipient-global unread count: %+v", selector.view, page)
					}
					if result.Header().Get("Cache-Control") != "no-store" || result.Header().Get("Referrer-Policy") != "no-referrer" {
						t.Fatal("account notifications lost their private-response headers")
					}
					for _, item := range page.Items {
						if item.Kind == "OTHER_RECIPIENT_SENTINEL" || selector.view == "unread" && (item.ReadAt != nil || item.ArchivedAt != nil) ||
							selector.view == "all" && item.ArchivedAt != nil || selector.view == "archived" && item.ArchivedAt == nil {
							t.Fatalf("view %q returned a row outside its state/recipient filter: %+v", selector.view, item)
						}
						got = append(got, uuidToString(item.ID))
					}
					if selector.view == "" && page.View != nil || selector.view != "" && (page.View == nil || *page.View != selector.view) {
						t.Fatalf("response marker does not match request %q: %+v", selector.query, page)
					}
					if page.NextCursor == nil {
						break
					}
					if len(page.Items) == 0 || query.Get("cursor") == *page.NextCursor {
						t.Fatal("notification pagination produced an empty or repeated cursor page")
					}
					query.Set("cursor", *page.NextCursor)
				}
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("limit %s skipped, duplicated, or reordered matching notifications: got %v want %v", limit, got, want)
				}
			}
		})
	}
}

func TestCollaborationNotificationsHTTPRejectsInvalidSelectors(t *testing.T) {
	for _, query := range []string{"view=", "view=future", "view=UNREAD", "view=all&view=unread", "include_archived=invalid",
		"include_archived=false&include_archived=true", "view=unread&include_archived=false", "view=all&include_archived=true", "view=archived&include_archived="} {
		t.Run(query, func(t *testing.T) {
			r := newRequest(http.MethodGet, "/api/me/collaboration-notifications?"+query, nil)
			r = r.WithContext(auth.WithAuthenticatedPrincipal(r.Context(), auth.AuthenticatedPrincipal{UserID: testUserID, Kind: auth.CredentialJWT}))
			var body struct{ Code string }
			testutil.Call(t, testHandler.ListCollaborationNotifications, r).Want(http.StatusBadRequest).JSON(&body)
			if body.Code != "invalid_notification_query" {
				t.Fatalf("invalid notification selectors returned unrelated error: %+v", body)
			}
		})
	}
}

func TestCollaborationNotificationsHTTPFilteredPrivacyKeepsExactTargets(t *testing.T) {
	issue := collaborationBoundaryIssue(t, testWorkspaceID)
	reader := dbfx.User(t, "Notification anchor reader", "anchors-"+uuid.NewString()+"@example.test")
	dbfx.InsertNoID(t, "issue_participant", testutil.Cols{"issue_id": issue, "user_id": reader}, "issue_id=$1 AND user_id=$2", issue, reader)
	comment := dbfx.Comment(t, issue, "PRIVATE_COMMENT_SENTINEL")
	run := uuid.NewString()
	for i, cols := range []testutil.Cols{
		{"kind": "comment_created", "is_mention": true, "source_kind": "comment", "source_id": comment},
		{"kind": "mention_added", "is_mention": true, "source_kind": "body", "source_id": issue},
		{"kind": "ai_completed", "source_kind": "issue", "run_id": run, "archived_at": time.Now()},
		{"kind": "removed", "source_kind": "comment", "source_id": comment, "run_id": run},
	} {
		cols["issue_id"], cols["recipient_user_id"], cols["event_seq"] = issue, reader, i+1
		dbfx.Insert(t, "collaboration_notification", cols)
	}
	request := func(view string) *http.Request {
		r := newRequest(http.MethodGet, "/api/me/collaboration-notifications?view="+view, nil)
		return r.WithContext(auth.WithAuthenticatedPrincipal(r.Context(), auth.AuthenticatedPrincipal{UserID: reader, Kind: auth.CredentialJWT}))
	}
	for _, view := range []string{"unread", "all", "archived"} {
		var page collaborationNotificationHTTPPage
		testutil.Call(t, testHandler.ListCollaborationNotifications, request(view)).Want(http.StatusOK).JSON(&page)
		for _, item := range page.Items {
			switch item.Kind {
			case "comment_created":
				if !item.CanOpen || item.SourceKind != "comment" || uuidToString(item.SourceID) != comment {
					t.Fatalf("filtered comment lost its exact target: %+v", item)
				}
			case "mention_added":
				if !item.CanOpen || item.SourceKind != "body" || uuidToString(item.SourceID) != issue {
					t.Fatalf("filtered body lost its exact target: %+v", item)
				}
			case "ai_completed":
				if !item.CanOpen || item.RunID == nil || *item.RunID != run || item.ReadAt != nil {
					t.Fatalf("archived unread result lost its exact run or became read: %+v", item)
				}
			case "removed":
				if item.CanOpen || item.IssueTitle != nil || item.SourceKind != "issue" || item.SourceID.Valid || item.RunID != nil {
					t.Fatalf("removed notification exposed private content: %+v", item)
				}
			}
		}
	}
	dbfx.Exec(t, `UPDATE issue_participant SET revoked_at=now(),epoch=epoch+1 WHERE issue_id=$1 AND user_id=$2`, issue, reader)
	for _, view := range []string{"unread", "all", "archived"} {
		var page collaborationNotificationHTTPPage
		result := testutil.Call(t, testHandler.ListCollaborationNotifications, request(view)).Want(http.StatusOK).JSON(&page)
		if len(page.Items) == 0 || page.UnreadCount != 3 {
			t.Fatalf("retained filtered notification states vanished after access loss: %+v", page)
		}
		for _, item := range page.Items {
			if item.CanOpen || item.IssueTitle != nil || item.SourceKind != "issue" || item.SourceID.Valid || item.RunID != nil {
				t.Fatalf("filtered retained notification restored private content: %+v", item)
			}
		}
		if strings.Contains(result.Body.String(), comment) || strings.Contains(result.Body.String(), run) || strings.Contains(result.Body.String(), "Protected collaboration") {
			t.Fatal("filtered denied response serialized a private title or anchor")
		}
	}
}

func TestCollaborationNotificationHTTPRechecksRecipientAndObjectAccess(t *testing.T) {
	issue := collaborationBoundaryIssue(t, testWorkspaceID)
	external := dbfx.User(t, "Notification reader", "reader-"+uuid.NewString()+"@example.test")
	dbfx.InsertNoID(t, "issue_participant", testutil.Cols{"issue_id": issue, "user_id": external}, "issue_id=$1 AND user_id=$2", issue, external)
	comment := dbfx.Comment(t, issue, "private comment sentinel")
	notification := dbfx.Insert(t, "collaboration_notification", testutil.Cols{"recipient_user_id": external, "issue_id": issue, "event_seq": 1,
		"kind": "comment_created", "is_mention": true, "source_kind": "comment", "source_id": comment})
	request := func(actor, action string) *http.Request {
		r := newRequest(http.MethodGet, "/api/me/collaboration-notifications", nil)
		if action != "" {
			r.Method = http.MethodPost
			r = withURLParam(r, "notificationId", notification)
		}
		r.Header.Set("X-Workspace-ID", "invalid-ambient-workspace")
		return r.WithContext(auth.WithAuthenticatedPrincipal(r.Context(), auth.AuthenticatedPrincipal{UserID: actor, Kind: auth.CredentialJWT}))
	}
	var page struct {
		Items       []service.CollaborationNotificationView `json:"items"`
		UnreadCount int64                                   `json:"unread_count"`
	}
	testutil.Call(t, testHandler.ListCollaborationNotifications, request(external, "")).Want(http.StatusOK).JSON(&page)
	if len(page.Items) != 1 || !page.Items[0].CanOpen || uuidToString(page.Items[0].SourceID) != comment || page.UnreadCount != 1 {
		t.Fatalf("notification cannot locate the original comment: %+v", page)
	}
	testutil.Call(t, testHandler.ReadCollaborationNotification, request(testUserID, "read")).Want(http.StatusNotFound)
	read := testutil.Call(t, testHandler.ReadCollaborationNotification, request(external, "read")).Want(http.StatusOK)
	if strings.Contains(read.Body.String(), "issue_id") || strings.Contains(read.Body.String(), "private") || strings.Contains(read.Body.String(), "source_id") {
		t.Fatal("notification mutation returned private row content")
	}
	dbfx.Exec(t, `UPDATE issue_participant SET revoked_at=now(),epoch=epoch+1 WHERE issue_id=$1 AND user_id=$2`, issue, external)
	testutil.Call(t, testHandler.ListCollaborationNotifications, request(external, "")).Want(http.StatusOK).JSON(&page)
	if page.Items[0].CanOpen || page.Items[0].IssueTitle != nil || page.Items[0].SourceID.Valid {
		t.Fatalf("old notification restored access: %+v", page.Items[0])
	}
	testutil.Call(t, testHandler.ArchiveCollaborationNotification, request(external, "archive")).Want(http.StatusOK)
	testutil.Call(t, testHandler.ListCollaborationNotifications, request(external, "")).Want(http.StatusOK).JSON(&page)
	if len(page.Items) != 0 || page.UnreadCount != 0 {
		t.Fatalf("archived notification remains unread: %+v", page)
	}
}
