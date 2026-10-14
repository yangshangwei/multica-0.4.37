package handler

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/redis/go-redis/v9"
)

func newRecipientHandler(t *testing.T) *Handler {
	t.Helper()
	t.Setenv("MULTICA_PASSWORD_LIMITER_MODE", "single")
	h := *testHandler
	h.PasswordLimiter = auth.NewPasswordLimiter(nil)
	return &h
}

func recipientRequest(body string, principal auth.AuthenticatedPrincipal) *http.Request {
	r := newRequest(http.MethodPost, "/api/collaboration/recipients/resolve", nil)
	r.Body = io.NopCloser(strings.NewReader(body))
	r.ContentLength = int64(len(body))
	return r.WithContext(auth.WithAuthenticatedPrincipal(r.Context(), principal))
}

func TestResolveCollaborationRecipientStrictBody(t *testing.T) {
	id := uuid.NewString()
	principal := auth.AuthenticatedPrincipal{UserID: testUserID, Kind: auth.CredentialJWT}
	for _, scenario := range []struct{ name, body, code string }{
		{"missing_context", `{"kind":"user_id","value":"` + id + `"}`, "invalid_recipient_context"},
		{"null_issue", `{"kind":"user_id","value":"` + id + `","issue_id":null}`, "invalid_recipient_context"},
		{"null_source", `{"kind":"user_id","value":"` + id + `","source_workspace_id":null}`, "invalid_recipient_context"},
		{"both_contexts", `{"kind":"user_id","value":"` + id + `","issue_id":"` + id + `","source_workspace_id":"` + id + `"}`, "invalid_recipient_context"},
		{"second_null_context", `{"kind":"user_id","value":"` + id + `","issue_id":"` + id + `","source_workspace_id":null}`, "invalid_recipient_context"},
		{"first_null_context", `{"kind":"user_id","value":"` + id + `","issue_id":null,"source_workspace_id":"` + id + `"}`, "invalid_recipient_context"},
		{"empty_context", `{"kind":"user_id","value":"` + id + `","issue_id":""}`, "invalid_recipient_context"},
		{"malformed_context", `{"kind":"user_id","value":"` + id + `","issue_id":"MUL-123"}`, "invalid_recipient_context"},
		{"wrong_context_type", `{"kind":"user_id","value":"` + id + `","source_workspace_id":3}`, "invalid_recipient_context"},
		{"unknown_field", `{"kind":"user_id","value":"` + id + `","issue_id":"` + id + `","actor_user_id":"` + id + `"}`, "invalid_recipient_query"},
		{"missing_kind", `{"value":"` + id + `","issue_id":"` + id + `"}`, "invalid_recipient_query"},
		{"missing_value", `{"kind":"user_id","issue_id":"` + id + `"}`, "invalid_recipient_query"},
		{"null_value", `{"kind":"user_id","value":null,"issue_id":"` + id + `"}`, "invalid_recipient_query"},
		{"null_kind", `{"kind":null,"value":"` + id + `","issue_id":"` + id + `"}`, "invalid_recipient_query"},
		{"wrong_value_type", `{"kind":"user_id","value":[],"issue_id":"` + id + `"}`, "invalid_recipient_query"},
		{"duplicate_field", `{"kind":"user_id","kind":"email","value":"` + id + `","issue_id":"` + id + `"}`, "invalid_recipient_query"},
		{"duplicate_alias", `{"kind":"user_id","Kind":"email","value":"` + id + `","issue_id":"` + id + `"}`, "invalid_recipient_query"},
		{"trailing_json", `{"kind":"user_id","value":"` + id + `","issue_id":"` + id + `"} {}`, "invalid_recipient_query"},
		{"malformed_json", `{"kind":`, "invalid_recipient_query"},
		{"array", `[]`, "invalid_recipient_query"},
		{"null", `null`, "invalid_recipient_query"},
		{"oversized", `{"kind":"email","value":"` + strings.Repeat("x", 4096) + `","issue_id":"` + id + `"}`, "invalid_recipient_query"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			h := newRecipientHandler(t)
			h.Queries = nil
			r := recipientRequest(scenario.body, principal)
			r.Header.Set("X-Workspace-Slug", "ambient-space")
			response := testutil.Call(t, h.ResolveCollaborationRecipient, r).Want(http.StatusBadRequest)
			if response.Map()["code"] != scenario.code {
				t.Fatalf("wrong strict-body error: %s", response.Body.String())
			}
			if response.Header().Get("Cache-Control") != "no-store" || r.Header.Get("X-Workspace-ID") != "" || r.Header.Get("X-Workspace-Slug") != "" {
				t.Fatal("invalid lookup kept ambient headers or cacheable output")
			}
		})
	}
}

func TestResolveCollaborationRecipientHumanBackstop(t *testing.T) {
	h := &Handler{}
	for _, principal := range []auth.AuthenticatedPrincipal{
		{}, {UserID: testUserID, Kind: auth.CredentialTask}, {UserID: testUserID, Kind: auth.CredentialCloud},
		{UserID: testUserID, Kind: "plugin"}, {UserID: testUserID, Kind: "unknown"},
		{UserID: testUserID, Kind: auth.CredentialJWT, Restricted: true},
		{UserID: testUserID, Kind: auth.CredentialJWT, AgentActor: true},
		{UserID: testUserID, Kind: auth.CredentialPAT, AgentActor: true},
	} {
		r := recipientRequest(`{"not_a_query":true}`, principal)
		response := testutil.Call(t, h.ResolveCollaborationRecipient, r).Want(http.StatusForbidden)
		if response.Map()["code"] != "forbidden" || response.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("untrusted principal escaped the private backstop: %s", response.Body.String())
		}
	}
	for _, hint := range []string{"X-Agent-ID", "X-Task-ID"} {
		r := recipientRequest(`{}`, auth.AuthenticatedPrincipal{UserID: testUserID, Kind: auth.CredentialJWT})
		r.Header.Set(hint, uuid.NewString())
		testutil.Call(t, h.ResolveCollaborationRecipient, r).Want(http.StatusForbidden)
	}
}

func TestResolveCollaborationRecipientRequiresJSON(t *testing.T) {
	h := newRecipientHandler(t)
	h.Queries = nil
	for _, contentType := range []string{"", "text/plain", "application/x-www-form-urlencoded"} {
		r := recipientRequest(`{}`, auth.AuthenticatedPrincipal{UserID: testUserID, Kind: auth.CredentialJWT})
		r.Header.Set("Content-Type", contentType)
		response := testutil.Call(t, h.ResolveCollaborationRecipient, r).Want(http.StatusUnsupportedMediaType)
		if response.Map()["code"] != "json_required" || response.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("non-JSON query escaped the boundary: %s", response.Body.String())
		}
	}
}

func TestResolveCollaborationRecipientPrivateProjectionAndContext(t *testing.T) {
	h := newRecipientHandler(t)
	issue := collaborationBoundaryIssue(t, testWorkspaceID)
	privateEmail := "private-recipient-" + uuid.NewString() + "@example.test"
	recipient := dbfx.User(t, "Recipient display name", privateEmail, testutil.Cols{"profile_description": "private profile sentinel"})
	body := fmt.Sprintf(`{"kind":"user_id","value":%q,"issue_id":%q}`, strings.ToUpper(recipient), strings.ToUpper(issue))
	r := recipientRequest(body, auth.AuthenticatedPrincipal{UserID: testUserID, Kind: auth.CredentialPAT})
	r.Header.Set("X-User-ID", recipient)
	r.Header.Set("X-Workspace-ID", "not-the-source")
	r.Header.Set("X-Workspace-Slug", "not-the-source")
	response := testutil.Call(t, h.ResolveCollaborationRecipient, r).Want(http.StatusOK)
	var out CollaborationRecipientResponse
	response.JSON(&out)
	if out.IssueID == nil || *out.IssueID != issue || out.SourceWorkspaceID != testWorkspaceID ||
		out.Recipient.UserID != recipient || out.Recipient.Name != "Recipient display name" || !out.Recipient.External || out.Recipient.AvatarURL != "" {
		t.Fatalf("wrong confirmed recipient/context: %+v", out)
	}
	for _, private := range []string{privateEmail, "private profile", `"username"`, `"email"`, `"disabled_at"`, `"available"`, `"password_hash"`, `"session_version"`, `"confirm_token"`} {
		if strings.Contains(response.Body.String(), private) {
			t.Fatalf("recipient projection included private field %q", private)
		}
	}
	if response.Header().Get("Cache-Control") != "no-store" || r.Header.Get("X-Workspace-ID") != "" || r.Header.Get("X-Workspace-Slug") != "" {
		t.Fatal("recipient response kept ambient scope or allowed caching")
	}
	if dbfx.Count(t, `SELECT count(*) FROM issue_participant WHERE issue_id=$1 AND user_id=$2`, issue, recipient) != 0 ||
		dbfx.Count(t, `SELECT count(*) FROM member WHERE workspace_id=$1 AND user_id=$2`, testWorkspaceID, recipient) != 0 {
		t.Fatal("lookup invited or enrolled its recipient")
	}

	var creationEnabled bool
	dbfx.QueryRow(t, `SELECT creation_enabled FROM collaboration_execution_gate WHERE id=1`).Scan(&creationEnabled)
	dbfx.Cleanup(t, `UPDATE collaboration_execution_gate SET creation_enabled=$1 WHERE id=1`, creationEnabled)
	dbfx.Exec(t, `UPDATE collaboration_execution_gate SET creation_enabled=true WHERE id=1`)
	body = fmt.Sprintf(`{"kind":"user_id","value":%q,"source_workspace_id":%q}`, recipient, strings.ToUpper(testWorkspaceID))
	response = testutil.Call(t, h.ResolveCollaborationRecipient, recipientRequest(body, auth.AuthenticatedPrincipal{UserID: testUserID, Kind: auth.CredentialJWT})).Want(http.StatusOK)
	response.JSON(&out)
	if value, present := response.Map()["issue_id"]; !present || value != nil || out.SourceWorkspaceID != testWorkspaceID {
		t.Fatalf("creation projection must return explicit null issue_id and its source: %s", response.Body.String())
	}
}

func TestResolveCollaborationRecipientRateBudgetCannotRotateQueryOrHeaders(t *testing.T) {
	h := newRecipientHandler(t)
	issue := collaborationBoundaryIssue(t, testWorkspaceID)
	principal := auth.AuthenticatedPrincipal{UserID: testUserID, Kind: auth.CredentialJWT}
	limited := false
	for attempt := 0; attempt < 50; attempt++ {
		kind, value := "user_id", uuid.NewString()
		if attempt%3 == 1 {
			kind, value = "email", value+"@example.test"
		} else if attempt%3 == 2 {
			kind, value = "username", "absent_"+strings.ReplaceAll(value, "-", "")[:16]
		}
		r := recipientRequest(fmt.Sprintf(`{"kind":%q,"value":%q,"issue_id":%q}`, kind, value, issue), principal)
		r.Header.Set("X-User-ID", uuid.NewString())
		r.Header.Set("X-Forwarded-For", fmt.Sprintf("203.0.113.%d", attempt+1))
		response := testutil.Call(t, h.ResolveCollaborationRecipient, r)
		if response.Code == http.StatusTooManyRequests {
			wait, err := strconv.Atoi(response.Header().Get("Retry-After"))
			if err != nil || wait < 1 || response.Map()["code"] != "rate_limited" || response.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("invalid rate refusal: headers %v, body %s", response.Header(), response.Body.String())
			}
			limited = true
			break
		}
		response.Want(http.StatusNotFound)
	}
	if !limited {
		t.Fatal("recipient lookups bypassed the actor budget by changing query kind/value or headers")
	}
}

func TestResolveCollaborationRecipientIPBudgetCannotRotateAccounts(t *testing.T) {
	h := newRecipientHandler(t)
	h.Queries = nil
	t.Setenv("MULTICA_TRUSTED_PROXIES", "")
	limited := false
	for attempt := 0; attempt < 125; attempt++ {
		body := fmt.Sprintf(`{"kind":"user_id","value":%q,"issue_id":%q}`, uuid.NewString(), uuid.NewString())
		r := recipientRequest(body, auth.AuthenticatedPrincipal{UserID: uuid.NewString(), Kind: auth.CredentialJWT})
		r.Header.Set("X-Forwarded-For", fmt.Sprintf("203.0.113.%d", attempt+1))
		response := testutil.Call(t, h.ResolveCollaborationRecipient, r)
		if response.Code == http.StatusTooManyRequests {
			if response.Map()["code"] != "rate_limited" || response.Header().Get("Retry-After") == "" {
				t.Fatalf("invalid IP rate refusal: %s", response.Body.String())
			}
			limited = true
			break
		}
		// The dependency is intentionally absent: this tests the IP admission
		// gate before any candidate or source data can be read.
		response.Want(http.StatusServiceUnavailable)
	}
	if !limited {
		t.Fatal("account rotation or untrusted forwarding bypassed the IP lookup budget")
	}
}

func TestResolveCollaborationRecipientLimiterFailureIsUnavailable(t *testing.T) {
	for _, backend := range []string{"missing_limiter", "missing_shared_backend", "failed_shared_backend"} {
		t.Run(backend, func(t *testing.T) {
			h := newRecipientHandler(t)
			h.Queries = nil
			switch backend {
			case "missing_limiter":
				h.PasswordLimiter = nil
			case "missing_shared_backend":
				t.Setenv("MULTICA_PASSWORD_LIMITER_MODE", "shared")
			case "failed_shared_backend":
				t.Setenv("MULTICA_PASSWORD_LIMITER_MODE", "shared")
				rdb := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"})
				if err := rdb.Close(); err != nil {
					t.Fatal(err)
				}
				h.PasswordLimiter = auth.NewPasswordLimiter(rdb)
			}
			body := fmt.Sprintf(`{"kind":"user_id","value":%q,"issue_id":%q}`, uuid.NewString(), uuid.NewString())
			response := testutil.Call(t, h.ResolveCollaborationRecipient, recipientRequest(body, auth.AuthenticatedPrincipal{UserID: testUserID, Kind: auth.CredentialJWT})).Want(http.StatusServiceUnavailable)
			if response.Map()["code"] != "collaboration_unavailable" || response.Header().Get("Retry-After") != "1" || response.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("limiter error did not fail closed: %s", response.Body.String())
			}
		})
	}
}

type recipientHandlerFailureDB struct{ db.DBTX }

func (p recipientHandlerFailureDB) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if strings.Contains(sql, "-- name: FindCollaborationRecipients ") {
		return nil, errors.New("private-recipient-value@example.test: credential storage failure")
	}
	return p.DBTX.Query(ctx, sql, args...)
}

func TestResolveCollaborationRecipientDatabaseFailureDoesNotLeakPrivateValues(t *testing.T) {
	h := newRecipientHandler(t)
	h.Queries = db.New(recipientHandlerFailureDB{DBTX: testPool})
	issue := collaborationBoundaryIssue(t, testWorkspaceID)
	body := fmt.Sprintf(`{"kind":"email","value":"private-recipient-value@example.test","issue_id":%q}`, issue)
	response := testutil.Call(t, h.ResolveCollaborationRecipient, recipientRequest(body, auth.AuthenticatedPrincipal{UserID: testUserID, Kind: auth.CredentialJWT})).Want(http.StatusServiceUnavailable)
	if response.Map()["code"] != "collaboration_unavailable" || strings.Contains(response.Body.String(), "private-recipient") || strings.Contains(response.Body.String(), "credential storage") {
		t.Fatalf("DB failure was hidden or exposed private query information: %s", response.Body.String())
	}
}
