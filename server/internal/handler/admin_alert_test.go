package handler

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/middleware"
	"github.com/multica-ai/multica/server/internal/testutil"
)

func alertHTTPFixture(t *testing.T) (LoginResponse, http.Handler) {
	t.Helper()
	login := adminHandlerSetup(t)
	router := chi.NewRouter()
	router.Use(middleware.Auth(testHandler.Queries, nil, nil))
	router.Get("/api/admin/alerts", testHandler.AdminAlerts)
	router.Get("/api/admin/alerts/{id}", testHandler.AdminAlert)
	router.Post("/api/admin/alerts/{id}/acknowledge", testHandler.AdminAcknowledgeAlert)
	router.Post("/api/admin/alerts/{id}/assign", testHandler.AdminAssignAlert)
	router.Post("/api/admin/alerts/{id}/close", testHandler.AdminCloseAlert)
	router.Get("/api/admin/operations", testHandler.AdminOperations)
	return login, router
}
func createAlertHTTPRow(t *testing.T, status string, first time.Time) string {
	t.Helper()
	org, err := testHandler.Queries.GetInternalOrganization(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	return dbfx.Insert(t, "admin_alert", testutil.Cols{"organization_id": org.ID, "rule": "queue_timeout", "subject_kind": "task", "subject_id": uuid.NewString(), "fingerprint": "fixture:" + uuid.NewString(), "severity": "warning", "status": status, "first_seen_at": first, "last_seen_at": first, "last_observed_at": first, "condition_active": status == "open" || status == "acknowledged"})
}
func alertHTTPCall(t *testing.T, h http.Handler, token, method, path string, body any, key string) *testutil.Response {
	t.Helper()
	req := testutil.JSONRequest(method, path, body)
	req.Header.Set("Authorization", "Bearer "+token)
	if key != "" {
		req.Header.Set("Idempotency-Key", key)
	}
	return testutil.Call(t, h.ServeHTTP, req)
}

func TestAdminAlertHTTPActiveHistoryCursorAndUnknownDetector(t *testing.T) {
	login, router := alertHTTPFixture(t)
	now := time.Now().UTC()
	old := createAlertHTTPRow(t, "open", now.Add(-60*24*time.Hour))
	recent := createAlertHTTPRow(t, "acknowledged", now.Add(-time.Minute))
	createAlertHTTPRow(t, "closed", now.Add(-60*24*time.Hour))
	closed := createAlertHTTPRow(t, "closed", now.Add(-time.Hour))
	var page struct {
		Items   []map[string]any `json:"items"`
		Cursor  *string          `json:"next_cursor"`
		Quality string           `json:"data_quality"`
	}
	alertHTTPCall(t, router, login.Token, "GET", "/api/admin/alerts?limit=1", nil, "").Want(200).JSON(&page)
	if len(page.Items) != 1 || page.Items[0]["id"] != recent || page.Cursor == nil || page.Quality != "unknown" {
		t.Fatalf("first page=%+v", page)
	}
	firstCursor := *page.Cursor
	dbfx.Exec(t, "UPDATE admin_alert SET last_seen_at=now() WHERE id=$1", old)
	alertHTTPCall(t, router, login.Token, "GET", "/api/admin/alerts?limit=1&cursor="+url.QueryEscape(firstCursor), nil, "").Want(200).JSON(&page)
	if len(page.Items) != 1 || page.Items[0]["id"] != old || page.Cursor != nil {
		t.Fatalf("old active alert hidden or pagination moved: %+v", page)
	}
	alertHTTPCall(t, router, login.Token, "GET", "/api/admin/alerts?limit=2&cursor="+url.QueryEscape(firstCursor), nil, "").Want(400)
	alertHTTPCall(t, router, login.Token, "GET", "/api/admin/alerts?status=&status=closed", nil, "").Want(400)
	alertHTTPCall(t, router, login.Token, "GET", "/api/admin/alerts?status=closed", nil, "").Want(200).JSON(&page)
	if len(page.Items) != 1 || page.Items[0]["id"] != closed {
		t.Fatal("closed history did not use bounded default window")
	}
	from := url.QueryEscape(now.Add(-time.Hour).Format(time.RFC3339Nano))
	alertHTTPCall(t, router, login.Token, "GET", "/api/admin/alerts?time_from="+from, nil, "").Want(200).JSON(&page)
	if len(page.Items) != 1 || page.Items[0]["id"] != recent {
		t.Fatal("explicit active-alert time filter was ignored")
	}
}

func TestAdminAlertHTTPMutationReceiptAndObserverBoundary(t *testing.T) {
	login, router := alertHTTPFixture(t)
	id := createAlertHTTPRow(t, "open", time.Now().Add(-time.Hour))
	body := map[string]any{"expected_version": "1", "reason": "Investigate queued execution"}
	key := uuid.NewString()
	var first struct {
		Operation map[string]any `json:"operation"`
		Target    map[string]any `json:"target"`
	}
	alertHTTPCall(t, router, login.Token, "POST", "/api/admin/alerts/"+id+"/acknowledge", body, key).Want(200).JSON(&first)
	if first.Target["status"] != "acknowledged" || first.Target["condition_active"] != true || first.Target["resolved_at"] != nil || first.Target["version"] != "2" {
		t.Fatalf("acknowledgement claimed recovery: %+v", first)
	}
	opID := first.Operation["id"]
	alertHTTPCall(t, router, login.Token, "POST", "/api/admin/alerts/"+id+"/acknowledge", body, key).Want(200).JSON(&first)
	if first.Operation["id"] != opID {
		t.Fatal("repeat acknowledgement created another operation")
	}
	conflictKey := uuid.NewString()
	alertHTTPCall(t, router, login.Token, "POST", "/api/admin/alerts/"+id+"/acknowledge", body, conflictKey).Want(409)
	var found struct {
		Items []map[string]any `json:"items"`
	}
	alertHTTPCall(t, router, login.Token, "GET", "/api/admin/operations?idempotency_key="+conflictKey, nil, "").Want(200).JSON(&found)
	if len(found.Items) != 1 || found.Items[0]["state"] != "failed" || found.Items[0]["result_code"] != "alert_version_conflict" {
		t.Fatal("conflict lacks original-key terminal receipt")
	}
	observer := passwordRegister(t, "alertobserver"+strings.ReplaceAll(uuid.NewString(), "-", "")[:10])
	dbfx.InsertNoID(t, "platform_role_binding", testutil.Cols{"user_id": observer.User.ID, "role": "platform_observer"}, "user_id=$1", observer.User.ID)
	var detail map[string]any
	alertHTTPCall(t, router, observer.Token, "GET", "/api/admin/alerts/"+id, nil, "").Want(200).JSON(&detail)
	if len(detail["allowed_actions"].([]any)) != 0 {
		t.Fatal("observer received mutation controls")
	}
	for _, action := range []string{"acknowledge", "assign", "close"} {
		alertHTTPCall(t, router, observer.Token, "POST", "/api/admin/alerts/"+id+"/"+action, body, uuid.NewString()).Want(403)
	}
	assign := map[string]any{"expected_version": "2", "reason": "Assign accountable administrator", "assignee_id": observer.User.ID}
	alertHTTPCall(t, router, login.Token, "POST", "/api/admin/alerts/"+id+"/assign", assign, uuid.NewString()).Want(409)
	assign["assignee_id"] = login.User.ID
	alertHTTPCall(t, router, login.Token, "POST", "/api/admin/alerts/"+id+"/assign", assign, uuid.NewString()).Want(200).JSON(&first)
	if first.Target["assignee_id"] != login.User.ID {
		t.Fatal("valid superadmin assignment not recorded")
	}
}

func TestAdminAlertHTTPNeverReturnsUncontrolledResolutionText(t *testing.T) {
	login, router := alertHTTPFixture(t)
	id := createAlertHTTPRow(t, "resolved", time.Now().Add(-time.Hour))
	dbfx.Exec(t, "UPDATE admin_alert SET resolution_code='PRIVATE /home/secret/path' WHERE id=$1", id)
	var response map[string]any
	alertHTTPCall(t, router, login.Token, "GET", "/api/admin/alerts/"+id, nil, "").Want(200).JSON(&response)
	encoded, _ := json.Marshal(response)
	if strings.Contains(string(encoded), "PRIVATE") || strings.Contains(string(encoded), "/home/") {
		t.Fatal("uncontrolled resolution text escaped metadata DTO")
	}
}
