package handler

import (
	"fmt"
	"io"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/testutil"
)

func recipientSearchRequest(body string, principal auth.AuthenticatedPrincipal) *http.Request {
	r := newRequest(http.MethodPost, "/api/collaboration/recipients/search", nil)
	r.Body = io.NopCloser(strings.NewReader(body))
	r.ContentLength = int64(len(body))
	return r.WithContext(auth.WithAuthenticatedPrincipal(r.Context(), principal))
}

func TestSearchCollaborationRecipientsStrictBody(t *testing.T) {
	id := uuid.NewString()
	principal := auth.AuthenticatedPrincipal{UserID: testUserID, Kind: auth.CredentialJWT}
	nul := `\` + "u0000"
	for _, scenario := range []struct{ name, body, code string }{
		{"missing_context", `{"q":"a"}`, "invalid_recipient_context"},
		{"two_contexts", `{"issue_id":"` + id + `","source_workspace_id":"` + id + `"}`, "invalid_recipient_context"},
		{"null_second_context", `{"issue_id":"` + id + `","source_workspace_id":null}`, "invalid_recipient_context"},
		{"malformed_context", `{"issue_id":"MUL-1"}`, "invalid_recipient_context"},
		{"unknown_field", `{"issue_id":"` + id + `","page":2}`, "invalid_recipient_query"},
		{"duplicate_field", `{"issue_id":"` + id + `","q":"a","q":"b"}`, "invalid_recipient_query"},
		{"wrong_q_type", `{"issue_id":"` + id + `","q":1}`, "invalid_recipient_query"},
		{"fractional_limit", `{"issue_id":"` + id + `","limit":1.5}`, "invalid_recipient_query"},
		{"string_limit", `{"issue_id":"` + id + `","limit":"30"}`, "invalid_recipient_query"},
		{"zero_limit", `{"issue_id":"` + id + `","limit":0}`, "invalid_recipient_query"},
		{"large_limit", `{"issue_id":"` + id + `","limit":51}`, "invalid_recipient_query"},
		{"bad_cursor", `{"issue_id":"` + id + `","cursor":"not a cursor"}`, "invalid_recipient_query"},
		{"cursor_bad_id", `{"issue_id":"` + id + `","cursor":"eyJuIjoieCIsImkiOiJNVUwtMSJ9"}`, "invalid_recipient_query"},
		{"wrong_members_only_type", `{"issue_id":"` + id + `","members_only":"yes"}`, "invalid_recipient_query"},
		{"long_q", `{"issue_id":"` + id + `","q":"` + strings.Repeat("x", 65) + `"}`, "invalid_recipient_query"},
		{"nul_q", `{"issue_id":"` + id + `","q":"a` + nul + `b"}`, "invalid_recipient_query"},
		{"oversized", `{"issue_id":"` + id + `","q":"` + strings.Repeat("x", 4096) + `"}`, "invalid_recipient_query"},
		{"array", `[]`, "invalid_recipient_query"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			h := newRecipientHandler(t)
			h.Queries = nil
			r := recipientSearchRequest(scenario.body, principal)
			r.Header.Set("X-Workspace-Slug", "ambient-space")
			response := testutil.Call(t, h.SearchCollaborationRecipients, r).Want(http.StatusBadRequest)
			if response.Map()["code"] != scenario.code || response.Header().Get("Cache-Control") != "no-store" || r.Header.Get("X-Workspace-Slug") != "" {
				t.Fatalf("wrong strict-body error: %s", response.Body.String())
			}
		})
	}
	h := newRecipientHandler(t)
	h.Queries = nil
	r := recipientSearchRequest(`{}`, principal)
	r.Header.Set("Content-Type", "text/plain")
	testutil.Call(t, h.SearchCollaborationRecipients, r).Want(http.StatusUnsupportedMediaType)
}

func TestSearchCollaborationRecipientsHumanBackstop(t *testing.T) {
	h := &Handler{}
	for _, principal := range []auth.AuthenticatedPrincipal{
		{}, {UserID: testUserID, Kind: auth.CredentialTask}, {UserID: testUserID, Kind: auth.CredentialJWT, AgentActor: true},
		{UserID: testUserID, Kind: auth.CredentialJWT, Restricted: true}, {UserID: "not-a-uuid", Kind: auth.CredentialJWT},
	} {
		response := testutil.Call(t, h.SearchCollaborationRecipients, recipientSearchRequest(`{}`, principal)).Want(http.StatusForbidden)
		if response.Map()["code"] != "forbidden" || response.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("untrusted principal searched recipients: %s", response.Body.String())
		}
	}
	for _, hint := range []string{"X-Agent-ID", "X-Task-ID"} {
		r := recipientSearchRequest(`{}`, auth.AuthenticatedPrincipal{UserID: testUserID, Kind: auth.CredentialJWT})
		r.Header.Set(hint, uuid.NewString())
		testutil.Call(t, h.SearchCollaborationRecipients, r).Want(http.StatusForbidden)
	}
}

func TestSearchCollaborationRecipientsRateBudgetIsSeparateFromResolve(t *testing.T) {
	h := newRecipientHandler(t)
	h.Queries = nil
	principal := auth.AuthenticatedPrincipal{UserID: testUserID, Kind: auth.CredentialJWT}
	allowed, limited := 0, false
	for attempt := 0; attempt < 250 && !limited; attempt++ {
		body := fmt.Sprintf(`{"issue_id":%q,"q":%q}`, uuid.NewString(), uuid.NewString()[:8])
		response := testutil.Call(t, h.SearchCollaborationRecipients, recipientSearchRequest(body, principal))
		if response.Code != http.StatusTooManyRequests {
			response.Want(http.StatusServiceUnavailable)
			allowed++
			continue
		}
		wait, err := strconv.Atoi(response.Header().Get("Retry-After"))
		if err != nil || wait < 1 || response.Map()["code"] != "rate_limited" || response.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("invalid rate refusal: headers %v, body %s", response.Header(), response.Body.String())
		}
		limited = true
	}
	if !limited || allowed < 120 {
		t.Fatalf("actor search budget: limited=%v after %d allowed searches", limited, allowed)
	}
	// The exhausted search budget must not spend the exact lookup's budget.
	body := fmt.Sprintf(`{"kind":"user_id","value":%q,"issue_id":%q}`, uuid.NewString(), uuid.NewString())
	testutil.Call(t, h.ResolveCollaborationRecipient, recipientRequest(body, principal)).Want(http.StatusServiceUnavailable)
}

func TestSearchCollaborationRecipientsIPBudgetCannotRotateAccounts(t *testing.T) {
	h := newRecipientHandler(t)
	h.Queries = nil
	t.Setenv("MULTICA_TRUSTED_PROXIES", "")
	allowed, limited := 0, false
	for attempt := 0; attempt < 650 && !limited; attempt++ {
		r := recipientSearchRequest(fmt.Sprintf(`{"issue_id":%q}`, uuid.NewString()), auth.AuthenticatedPrincipal{UserID: uuid.NewString(), Kind: auth.CredentialJWT})
		r.Header.Set("X-Forwarded-For", fmt.Sprintf("203.0.113.%d", attempt%250+1))
		response := testutil.Call(t, h.SearchCollaborationRecipients, r)
		if limited = response.Code == http.StatusTooManyRequests; !limited {
			response.Want(http.StatusServiceUnavailable)
			allowed++
		} else if response.Map()["code"] != "rate_limited" || response.Header().Get("Retry-After") == "" {
			t.Fatalf("invalid IP rate refusal: %s", response.Body.String())
		}
	}
	if !limited || allowed < 300 {
		t.Fatalf("IP search budget: limited=%v after %d allowed searches", limited, allowed)
	}
}

func TestSearchCollaborationRecipientsLimiterFailureIsUnavailable(t *testing.T) {
	h := newRecipientHandler(t)
	h.Queries, h.PasswordLimiter = nil, nil
	response := testutil.Call(t, h.SearchCollaborationRecipients, recipientSearchRequest(fmt.Sprintf(`{"issue_id":%q}`, uuid.NewString()), auth.AuthenticatedPrincipal{UserID: testUserID, Kind: auth.CredentialJWT})).Want(http.StatusServiceUnavailable)
	if response.Map()["code"] != "collaboration_unavailable" || response.Header().Get("Retry-After") != "1" || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("limiter error did not fail closed: %s", response.Body.String())
	}
}

func recipientSearchKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}

func TestSearchCollaborationRecipientsExactProjection(t *testing.T) {
	h := newRecipientHandler(t)
	h.cfg.CollaborationDirectoryEnabled = true
	principal := auth.AuthenticatedPrincipal{UserID: testUserID, Kind: auth.CredentialJWT}
	suffix := "pick" + strings.ReplaceAll(uuid.NewString(), "-", "")[:12]
	privateEmail := "picker-private-" + uuid.NewString() + "@example.test"
	first := dbfx.User(t, "Picker shape "+suffix, privateEmail, testutil.Cols{"profile_description": "private profile sentinel"})
	second := dbfx.User(t, "Picker shape "+suffix, "picker-"+uuid.NewString()+"@example.test")
	issue := collaborationBoundaryIssue(t, testWorkspaceID)
	topKeys := []string{"issue_id", "next_cursor", "recent", "recipients", "scope", "source_member", "source_workspace_id"}
	accountKeys := []string{"avatar_url", "external", "name", "user_id"}
	search := func(body string) map[string]any {
		t.Helper()
		response := testutil.Call(t, h.SearchCollaborationRecipients, recipientSearchRequest(body, principal)).Want(http.StatusOK)
		out := response.Map()
		if !slices.Equal(recipientSearchKeys(out), topKeys) || response.Header().Get("Cache-Control") != "no-store" ||
			strings.Contains(response.Body.String(), privateEmail) || strings.Contains(response.Body.String(), "private profile") {
			t.Fatalf("search projection is not exact: %s", response.Body.String())
		}
		for _, list := range []string{"recent", "recipients"} {
			items, ok := out[list].([]any)
			if !ok {
				t.Fatalf("%s must be an array: %s", list, response.Body.String())
			}
			for _, item := range items {
				if account, ok := item.(map[string]any); !ok || !slices.Equal(recipientSearchKeys(account), accountKeys) {
					t.Fatalf("%s account keys: %v", list, item)
				}
			}
		}
		return out
	}
	page := search(fmt.Sprintf(`{"issue_id":%q,"q":%q,"limit":1}`, strings.ToUpper(issue), suffix))
	recipients, _ := page["recipients"].([]any)
	cursor, _ := page["next_cursor"].(string)
	if page["issue_id"] != issue || page["source_workspace_id"] != testWorkspaceID || page["scope"] != "directory" || page["source_member"] != true || len(recipients) != 1 || cursor == "" {
		t.Fatalf("first page: %v", page)
	}
	ids := []string{recipients[0].(map[string]any)["user_id"].(string)}
	if recipients[0].(map[string]any)["external"] != true {
		t.Fatalf("a source member sees external=true for non-members: %v", recipients[0])
	}
	page = search(fmt.Sprintf(`{"issue_id":%q,"q":%q,"limit":1,"cursor":%q}`, issue, suffix, cursor))
	recipients, _ = page["recipients"].([]any)
	if len(recipients) != 1 || page["next_cursor"] != nil {
		t.Fatalf("last page: %v", page)
	}
	ids = append(ids, recipients[0].(map[string]any)["user_id"].(string))
	want := []string{first, second}
	slices.Sort(ids)
	slices.Sort(want)
	if !slices.Equal(ids, want) {
		t.Fatalf("pages returned %v, want %v", ids, want)
	}

	var creationEnabled bool
	dbfx.QueryRow(t, `SELECT creation_enabled FROM collaboration_execution_gate WHERE id=1`).Scan(&creationEnabled)
	dbfx.Cleanup(t, `UPDATE collaboration_execution_gate SET creation_enabled=$1 WHERE id=1`, creationEnabled)
	dbfx.Exec(t, `UPDATE collaboration_execution_gate SET creation_enabled=true WHERE id=1`)
	if page = search(fmt.Sprintf(`{"source_workspace_id":%q,"q":%q,"members_only":null,"cursor":null,"limit":null}`, testWorkspaceID, suffix)); page["issue_id"] != nil || len(page["recipients"].([]any)) != 2 {
		t.Fatalf("creation context: %v", page)
	}
	h.cfg.CollaborationDirectoryEnabled = false
	if page = search(fmt.Sprintf(`{"issue_id":%q,"q":%q}`, issue, suffix)); page["scope"] != "related" || len(page["recipients"].([]any)) != 0 {
		t.Fatalf("related scope listed strangers: %v", page)
	}

	// The caller handles an issue from a workspace it is not a member of.
	h.cfg.CollaborationDirectoryEnabled = true
	other := dbfx.Workspace(t, "Picker other source", "picker-"+suffix)
	foreign := collaborationBoundaryIssue(t, other)
	page = search(fmt.Sprintf(`{"issue_id":%q,"q":%q}`, foreign, suffix))
	recipients, _ = page["recipients"].([]any)
	if page["source_member"] != false || len(recipients) != 2 {
		t.Fatalf("non-member context: %v", page)
	}
	for _, item := range recipients {
		if value, present := item.(map[string]any)["external"]; !present || value != nil {
			t.Fatalf("a non-member must see external=null: %v", item)
		}
	}
	page = search(fmt.Sprintf(`{"issue_id":%q,"q":%q,"members_only":true}`, foreign, suffix))
	if len(page["recipients"].([]any)) != 0 || len(page["recent"].([]any)) != 0 || page["next_cursor"] != nil {
		t.Fatalf("members_only for a non-member must be an empty 200: %v", page)
	}
}
