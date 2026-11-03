package handler

import (
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
)

func deliveryHTTPRequest(method, issue, suffix, actor, key string, body any) *http.Request {
	r := testutil.WithURLParams(testutil.JSONRequest(method, "/api/collaboration/issues/"+issue+suffix, body), "id", issue, "requestId", key)
	testutil.WithHeaders(r, "Idempotency-Key", key, "X-Workspace-ID", "unrelated-ambient-workspace", "X-Workspace-Slug", "unrelated-ambient-workspace")
	if actor != "" {
		r = r.WithContext(auth.WithAuthenticatedPrincipal(r.Context(), auth.AuthenticatedPrincipal{UserID: actor, Kind: auth.CredentialJWT}))
	}
	return r
}

func deliveryHTTPFixture(t *testing.T) (*Handler, *testutil.Fixture, string, string, string) {
	t.Helper()
	t.Setenv("MULTICA_AUTH_MODE", "legacy")
	h, fx, actor, recipient := collaborationCreationHTTPFixture(t)
	if err := h.Queries.SeedIssueStatusEntries(t.Context(), parseUUID(fx.WorkspaceID)); err != nil {
		t.Fatal(err)
	}
	fx.Cleanup(t, `DELETE FROM issue_status WHERE workspace_id=$1`, fx.WorkspaceID)
	fx.Cleanup(t, `DELETE FROM issue_handoff WHERE issue_id IN (SELECT id FROM issue WHERE workspace_id=$1)`, fx.WorkspaceID)
	fx.Cleanup(t, `DELETE FROM collaboration_operation WHERE scope_kind='issue' AND scope_id IN (SELECT id FROM issue WHERE workspace_id=$1)`, fx.WorkspaceID)
	created := testutil.Call(t, h.CreateCollaborationIssue, collaborationCreationHTTPRequest(http.MethodPost, fx.WorkspaceID, uuid.NewString(), actor,
		map[string]any{"source_workspace_id": fx.WorkspaceID, "handler_user_id": recipient, "title": "HTTP result boundary", "description": "Human delivery without execution."}, false)).Want(http.StatusCreated).Map()
	issue, ok := created["id"].(string)
	if !ok {
		t.Fatal("creation returned no issue identity")
	}
	return h, fx, actor, recipient, issue
}

func deliveryHTTPSubmissionBody(t *testing.T, h *Handler, issue, actor, note, basis string) map[string]any {
	t.Helper()
	var options service.CollaborationStatusOptions
	testutil.Call(t, h.GetCollaborationStatusOptions, deliveryHTTPRequest(http.MethodGet, issue, "/status-options?action=submit_acceptance", actor, "", nil)).Want(http.StatusOK).JSON(&options)
	body := collaborationEditBodyValues()
	body["note"], body["basis"] = note, basis
	body["expected_status"], body["status_options_version"] = options.DefaultStatus, options.StatusOptionsVersion
	return body
}

func TestCollaborationDeliveryHTTPStrictBodiesAndPublicLimits(t *testing.T) {
	for _, endpoint := range []struct {
		name, field string
		handler     http.HandlerFunc
	}{
		{"submit", "note", (&Handler{}).SubmitCollaborationAcceptance},
		{"complete", "note", (&Handler{}).ChangeCollaborationLifecycle},
		{"return", "note", (&Handler{}).ReturnCollaborationAcceptance},
		{"policy", "ai_policy", (&Handler{}).UpdateCollaborationAIPolicy},
	} {
		t.Run(endpoint.name, func(t *testing.T) {
			for _, body := range []string{
				`{"unknown":"private unexpected value"}`,
				`{"` + endpoint.field + `":"first","` + endpoint.field + `":"second"}`,
				`{} {}`,
				`{"` + endpoint.field + `":{}}`,
				`null`,
			} {
				out := testutil.Call(t, endpoint.handler, testutil.JSONRequest(http.MethodPost, "/api/collaboration/issues/unused", body)).Want(http.StatusBadRequest).Map()
				if out["code"] != "invalid_request" {
					t.Fatalf("strict JSON reached workflow parsing: %v", out)
				}
			}
			request := testutil.JSONRequest(http.MethodPost, "/api/collaboration/issues/unused", `{}`)
			request.Header.Set("Content-Type", "text/plain")
			if out := testutil.Call(t, endpoint.handler, request).Want(http.StatusUnsupportedMediaType).Map(); out["code"] != "json_required" {
				t.Fatal("workflow accepted non-JSON media type")
			}
		})
	}
	for _, endpoint := range []http.HandlerFunc{(&Handler{}).SubmitCollaborationAcceptance, (&Handler{}).ChangeCollaborationLifecycle} {
		if out := testutil.Call(t, endpoint, testutil.JSONRequest(http.MethodPost, "/api/collaboration/issues/unused", strings.Repeat(" ", (1<<20)+1))).Want(http.StatusRequestEntityTooLarge).Map(); out["code"] != "request_too_large" {
			t.Fatal("result route did not bound the combined public request")
		}
	}
}

func TestCollaborationDeliveryHTTPRejectsMalformedOptionalSelection(t *testing.T) {
	for _, endpoint := range []http.HandlerFunc{(&Handler{}).SubmitCollaborationAcceptance, (&Handler{}).ChangeCollaborationLifecycle} {
		for _, selection := range []map[string]any{
			{"draft_id": "invalid"}, {"source_comment_id": "invalid"}, {"attachment_ids": []string{"invalid"}},
			{"attachment_ids": []int{1}}, {"draft_id": map[string]any{}}, {"source_comment_id": map[string]any{}},
		} {
			body := collaborationEditBodyValues()
			for field, value := range selection {
				body[field] = value
			}
			testutil.Call(t, endpoint, deliveryHTTPRequest(http.MethodPost, uuid.NewString(), "", "", uuid.NewString(), body)).Want(http.StatusBadRequest)
		}
	}
}

func TestCollaborationDeliveryHTTPAcceptsBothEscapedByteLimitsAndOwnConfirmation(t *testing.T) {
	h, fx, actor, recipient, issue := deliveryHTTPFixture(t)
	// Each decoded prose field is exactly 64KiB, with a six-byte JSON escape
	// for each control byte. The public limit must accommodate both fields.
	prose := strings.Repeat("\x01", (64<<10)-1) + "x"
	body := deliveryHTTPSubmissionBody(t, h, issue, recipient, prose, prose)
	key := uuid.NewString()
	request := deliveryHTTPRequest(http.MethodPost, issue, "/acceptance-submissions", recipient, key, body)
	response := testutil.Call(t, h.SubmitCollaborationAcceptance, request).Want(http.StatusOK)
	out := response.Map()
	confirmation, ok := out["delivery_confirmation"].(map[string]any)
	if !ok || len(confirmation) != 2 || out["code"] != "acceptance_submitted" || out["collaboration_revision"] != "2" {
		t.Fatalf("submission did not return its narrow exact confirmation: %v", out)
	}
	ids, ok := confirmation["attachment_ids"].([]any)
	submissionID, validID := confirmation["submission_id"].(string)
	if !ok || len(ids) != 0 || !validID || uuid.Validate(submissionID) != nil {
		t.Fatal("text-only result did not confirm its submission with an empty file list")
	}
	if response.Header().Get("Cache-Control") != "no-store" || request.Header.Get("X-Workspace-ID") != "" || request.Header.Get("X-Workspace-Slug") != "" ||
		fx.Count(t, `SELECT count(*) FROM member WHERE workspace_id=$1 AND user_id=$2`, fx.WorkspaceID, recipient) != 0 ||
		fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE session_id IN (SELECT id FROM collaboration_session WHERE issue_id=$1)`, issue) != 0 {
		t.Fatal("delivery retained ambient scope, expanded source membership or created AI input")
	}
	replay := testutil.Call(t, h.SubmitCollaborationAcceptance, deliveryHTTPRequest(http.MethodPost, issue, "/acceptance-submissions", recipient, key, body)).Want(http.StatusOK).Map()
	if !reflect.DeepEqual(replay, out) {
		t.Fatal("submission replay changed its original confirmation")
	}
	remove := collaborationEditBodyValues()
	remove["expected_collaboration_revision"] = "2"
	removeRequest := deliveryHTTPRequest(http.MethodPost, issue, "/participants/"+recipient+"/remove", actor, uuid.NewString(), remove)
	removeRequest = testutil.WithURLParams(removeRequest, "id", issue, "userID", recipient)
	testutil.Call(t, h.RemoveCollaborationParticipant, removeRequest).Want(http.StatusOK)
	testutil.Call(t, h.GetCollaborationIssue, deliveryHTTPRequest(http.MethodGet, issue, "", recipient, "", nil)).Want(http.StatusNotFound)
	testutil.Call(t, h.ListCollaborationAcceptanceSubmissions, deliveryHTTPRequest(http.MethodGet, issue, "/acceptance-submissions", recipient, "", nil)).Want(http.StatusNotFound)
	recovery := testutil.Call(t, h.GetCollaborationOperation, deliveryHTTPRequest(http.MethodGet, issue, "/operations/"+key, recipient, key, nil)).Want(http.StatusOK)
	result := recovery.Map()
	if len(result) != 3 || !reflect.DeepEqual(result["delivery_confirmation"], confirmation) || result["code"] != "acceptance_submitted" || strings.Contains(recovery.Text(), "basis") || strings.Contains(recovery.Text(), "actor_user_id") {
		t.Fatal("own recovery lost correlation or disclosed protected content after access loss")
	}
}

func TestCollaborationDeliveryHTTPRefusalHasCommittedStatusAndOriginalKey(t *testing.T) {
	h, _, _, recipient, issue := deliveryHTTPFixture(t)
	body := deliveryHTTPSubmissionBody(t, h, issue, recipient, "The result remains editable", " \n\t")
	key := uuid.NewString()
	refused := testutil.Call(t, h.SubmitCollaborationAcceptance, deliveryHTTPRequest(http.MethodPost, issue, "/acceptance-submissions", recipient, key, body)).Want(http.StatusBadRequest).Map()
	if refused["code"] != "basis_required" || refused["status"] != float64(http.StatusBadRequest) || refused["collaboration_revision"] != "1" {
		t.Fatalf("known refusal omitted its committed status: %v", refused)
	}
	result := testutil.Call(t, h.GetCollaborationOperation, deliveryHTTPRequest(http.MethodGet, issue, "/operations/"+key, recipient, key, nil)).Want(http.StatusOK).Map()
	if result["code"] != "basis_required" || result["status"] != float64(http.StatusBadRequest) || result["delivery_confirmation"] != nil {
		t.Fatal("refusal recovery became success or invented result confirmation")
	}
	body["basis"] = "Corrected supporting checks"
	if changed := testutil.Call(t, h.SubmitCollaborationAcceptance, deliveryHTTPRequest(http.MethodPost, issue, "/acceptance-submissions", recipient, key, body)).Want(http.StatusConflict).Map(); changed["code"] != "idempotency_conflict" {
		t.Fatal("changed result reused a refused original key")
	}
	testutil.Call(t, h.SubmitCollaborationAcceptance, deliveryHTTPRequest(http.MethodPost, issue, "/acceptance-submissions", recipient, uuid.NewString(), body)).Want(http.StatusOK)
}

func TestCollaborationAIPolicyHTTPRejectsInvalidPolicyWithDurableRefusal(t *testing.T) {
	h, fx, actor, recipient, issue := deliveryHTTPFixture(t)
	for i, policy := range []any{"unexpected", "", nil} {
		body := collaborationEditBodyValues()
		body["ai_policy"] = policy
		key := uuid.NewString()
		out := testutil.Call(t, h.UpdateCollaborationAIPolicy, deliveryHTTPRequest(http.MethodPost, issue, "/ai-policy", actor, key, body)).Want(http.StatusBadRequest).Map()
		if out["code"] != "invalid_ai_policy" || out["status"] != float64(http.StatusBadRequest) || out["collaboration_revision"] != "1" ||
			fx.Count(t, `SELECT count(*) FROM issue_collaboration WHERE issue_id=$1 AND ai_policy='assisted' AND revision=1`, issue) != 1 {
			t.Fatalf("invalid policy case %d changed state or lost a final refusal: %v", i, out)
		}
		result := testutil.Call(t, h.GetCollaborationOperation, deliveryHTTPRequest(http.MethodGet, issue, "/operations/"+key, actor, key, nil)).Want(http.StatusOK).Map()
		if result["status"] != float64(http.StatusBadRequest) || result["code"] != "invalid_ai_policy" {
			t.Fatal("original policy refusal was not recoverable")
		}
	}
	body := collaborationEditBodyValues()
	body["ai_policy"] = "manual"
	if out := testutil.Call(t, h.UpdateCollaborationAIPolicy, deliveryHTTPRequest(http.MethodPost, issue, "/ai-policy", recipient, uuid.NewString(), body)).Want(http.StatusForbidden).Map(); out["code"] != "forbidden" {
		t.Fatal("external handler changed initiator-owned policy")
	}
	changed := testutil.Call(t, h.UpdateCollaborationAIPolicy, deliveryHTTPRequest(http.MethodPost, issue, "/ai-policy", actor, uuid.NewString(), body)).Want(http.StatusOK).Map()
	if changed["code"] != "ai_policy_changed" || changed["collaboration_revision"] != "2" {
		t.Fatal("valid initiator policy operation was refused")
	}
}

func TestCollaborationCreationPolicyHTTPPersistsExplicitManualAndDefaultAssisted(t *testing.T) {
	h, fx, actor, recipient := collaborationCreationHTTPFixture(t)
	for _, policy := range []string{"", "assisted", "manual"} {
		body := map[string]any{"source_workspace_id": fx.WorkspaceID, "handler_user_id": recipient,
			"title": "Policy creation " + policy, "description": "An explicit task policy, without agent execution."}
		if policy != "" {
			body["ai_policy"] = policy
		}
		key := uuid.NewString()
		request := collaborationCreationHTTPRequest(http.MethodPost, fx.WorkspaceID, key, actor, body, false)
		created := testutil.Call(t, h.CreateCollaborationIssue, request).Want(http.StatusCreated).Map()
		want := policy
		if want == "" {
			want = "assisted"
		}
		if created["ai_policy"] != want {
			t.Fatalf("created task policy = %v; want %s", created["ai_policy"], want)
		}
		if fx.Count(t, `SELECT count(*) FROM issue_collaboration WHERE issue_id=$1 AND ai_policy=$2`, created["id"], want) != 1 ||
			fx.Count(t, `SELECT count(*) FROM member WHERE workspace_id=$1 AND user_id=$2`, fx.WorkspaceID, recipient) != 0 ||
			fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE session_id IN (SELECT id FROM collaboration_session WHERE issue_id=$1)`, created["id"]) != 0 {
			t.Fatal("creation lost its policy, enrolled the external account or created AI input")
		}
		replay := testutil.Call(t, h.CreateCollaborationIssue,
			collaborationCreationHTTPRequest(http.MethodPost, fx.WorkspaceID, key, actor, body, false)).Want(http.StatusCreated).Map()
		if replay["id"] != created["id"] || replay["ai_policy"] != want {
			t.Fatal("original creation replay changed its identity or task policy")
		}
	}
}

func TestCollaborationCreationPolicyHTTPRejectsPresentInvalidEnums(t *testing.T) {
	for _, tc := range []struct {
		name   string
		policy any
	}{
		{"null", nil}, {"empty", ""}, {"unknown", "unexpected"}, {"number", 1}, {"object", map[string]any{}}, {"array", []string{"manual"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, fx, actor, recipient := collaborationCreationHTTPFixture(t)
			body := map[string]any{"source_workspace_id": fx.WorkspaceID, "handler_user_id": recipient,
				"title": "Invalid policy does not create a task", "ai_policy": tc.policy}
			out := testutil.Call(t, h.CreateCollaborationIssue, collaborationCreationHTTPRequest(http.MethodPost, fx.WorkspaceID, uuid.NewString(), actor, body, false)).Want(http.StatusBadRequest).Map()
			if out["code"] != "invalid_ai_policy" || fx.Count(t, `SELECT count(*) FROM issue WHERE workspace_id=$1`, fx.WorkspaceID) != 0 ||
				fx.Count(t, `SELECT count(*) FROM collaboration_operation WHERE scope_kind='create' AND scope_id=$1`, fx.WorkspaceID) != 0 {
				t.Fatalf("present invalid policy %s defaulted or wrote creation state: %v", tc.name, out)
			}
		})
	}
}
