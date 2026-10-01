package handler

import (
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestAdminExecutionCursorBindsFiltersActorAndSnapshot(t *testing.T) {
	t.Setenv("JWT_SECRET", "admin-cursor-test-secret-at-least-32-characters")
	actor, org := uuid.NewString(), uuid.NewString()
	now := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	r := httptest.NewRequest("GET", "/api/admin/tasks?status=failed&limit=2", nil)
	p, err := parseAdminQuery(r, "tasks", actor, org, now)
	if err != nil {
		t.Fatal(err)
	}
	token, err := p.Cursor(now.Add(-time.Hour), uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	u := r.URL.Query()
	u.Set("cursor", token)
	next := httptest.NewRequest("GET", "/api/admin/tasks?"+u.Encode(), nil)
	page, err := parseAdminQuery(next, "tasks", actor, org, now.Add(time.Minute))
	if err != nil || !page.AsOf.Equal(now) || !page.From.Equal(p.From) {
		t.Fatalf("snapshot not retained: %+v %v", page, err)
	}
	for _, change := range []string{"actor", "organization", "resource", "status", "limit", "tamper"} {
		t.Run(change, func(t *testing.T) {
			a, o, resource := actor, org, "tasks"
			params, _ := url.ParseQuery(u.Encode())
			switch change {
			case "actor":
				a = uuid.NewString()
			case "organization":
				o = uuid.NewString()
			case "resource":
				resource = "issues"
			case "status":
				params.Set("status", "running")
			case "limit":
				params.Set("limit", "3")
			case "tamper":
				params.Set("cursor", token+"x")
			}
			_, err := parseAdminQuery(httptest.NewRequest("GET", "/api/admin/"+resource+"?"+params.Encode(), nil), resource, a, o, now)
			if err == nil {
				t.Fatal("accepted cursor outside its scope")
			}
		})
	}
}

func TestAdminExecutionQueryRejectsInvalidBoundaries(t *testing.T) {
	now := time.Now().UTC()
	for _, query := range []string{"limit=101", "limit=0", "status=secret", "source=arbitrary", "runtime_id=bad", "time_from=2026-01-01T00:00:00Z&time_to=2026-03-01T00:00:00Z", "timezone=Invalid/Place", "time_from=bad", "status=queued&status=failed", "status=failed;source=issue", "q=%zz", "sort=title", "q=" + url.QueryEscape(string(make([]byte, 130)))} {
		t.Run(query, func(t *testing.T) {
			_, err := parseAdminQuery(httptest.NewRequest("GET", "/api/admin/tasks?"+query, nil), "tasks", uuid.NewString(), uuid.NewString(), now)
			if err == nil {
				t.Fatal("accepted invalid query")
			}
		})
	}
}

func TestAdminExecutionHistoricalWindowAndIssueFilterValidation(t *testing.T) {
	now := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	p, err := parseAdminQuery(httptest.NewRequest("GET", "/api/admin/tasks?time_to=2026-08-01T00:00:00Z", nil), "tasks", uuid.NewString(), uuid.NewString(), now)
	if err != nil || p.To.Sub(p.From) != 31*24*time.Hour {
		t.Fatalf("historical end should select preceding 31 days: %+v %v", p, err)
	}
	for _, query := range []string{"source=chat", "runtime_id=" + uuid.NewString(), "task_id=" + uuid.NewString()} {
		if _, err := parseAdminQuery(httptest.NewRequest("GET", "/api/admin/issues?"+query, nil), "issues", uuid.NewString(), uuid.NewString(), now); err == nil {
			t.Fatal("ignored unsupported business-task filter", query)
		}
	}
}
