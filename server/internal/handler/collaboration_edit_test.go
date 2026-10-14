package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
)

func collaborationEditFixture(t *testing.T) string {
	t.Helper()
	issue := collaborationBoundaryIssue(t, testWorkspaceID)
	dbfx.InsertNoID(t, "collaboration_account_fence", testutil.Cols{"user_id": testUserID}, "user_id=$1", testUserID)
	dbfx.Cleanup(t, `DELETE FROM collaboration_operation WHERE scope_kind='issue' AND scope_id=$1`, issue)
	dbfx.Cleanup(t, `DELETE FROM collaboration_event WHERE issue_id=$1`, issue)
	return issue
}

func collaborationEditRequest(issue, actor, key string, body any) *http.Request {
	r := withURLParam(newRequest(http.MethodPatch, "/api/collaboration/issues/"+issue, body), "id", issue)
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Idempotency-Key", key)
	r.Header.Set("X-Workspace-ID", "unrelated-ambient-workspace")
	r.Header.Set("X-Workspace-Slug", "unrelated-ambient-workspace")
	return r.WithContext(auth.WithAuthenticatedPrincipal(r.Context(), auth.AuthenticatedPrincipal{UserID: actor, Kind: auth.CredentialJWT}))
}

func collaborationEditBodyValues() map[string]any {
	return map[string]any{"expected_collaboration_revision": "1", "expected_lifecycle_epoch": "1", "expected_participant_epoch": "1", "expected_account_epoch": "1"}
}

func TestCollaborationEditHTTPExternalHandlerAndExactVersions(t *testing.T) {
	issue := collaborationEditFixture(t)
	external := dbfx.User(t, "External editor", "private-editor-"+uuid.NewString()+"@example.test")
	dbfx.InsertNoID(t, "issue_participant", testutil.Cols{"issue_id": issue, "user_id": external, "epoch": int64(9007199254740993)}, "issue_id=$1 AND user_id=$2", issue, external)
	dbfx.InsertNoID(t, "collaboration_account_fence", testutil.Cols{"user_id": external, "epoch": int64(9007199254740993)}, "user_id=$1", external)
	dbfx.Exec(t, `UPDATE issue SET assignee_id=$2 WHERE id=$1`, issue, external)
	dbfx.Exec(t, `UPDATE issue_collaboration SET revision=9007199254740993,lifecycle_epoch=9007199254740993 WHERE issue_id=$1`, issue)
	body := collaborationEditBodyValues()
	for field := range body {
		body[field] = "9007199254740993"
	}
	body["title"], body["description"], body["start_date"], body["due_date"] = "External edit", "New private body", "2026-10-14", "2026-11-01T00:00:00Z"
	key := uuid.NewString()
	r := collaborationEditRequest(issue, external, key, body)
	response := testutil.Call(t, testHandler.UpdateCollaborationIssue, r).Want(http.StatusOK)
	out := response.Map()
	if out["code"] != "issue_edited" || out["collaboration_revision"] != "9007199254740994" || len(out) != 2 {
		t.Fatalf("editing response is not a minimal exact-version receipt: %+v", out)
	}
	if response.Header().Get("Cache-Control") != "no-store" || r.Header.Get("X-Workspace-ID") != "" || r.Header.Get("X-Workspace-Slug") != "" {
		t.Fatal("private account operation retained ambient workspace or caching")
	}
	if strings.Contains(response.Body.String(), "private") || strings.Contains(response.Body.String(), "email") {
		t.Fatal("edit response leaked private body or account fields")
	}
	if dbfx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND title='External edit' AND description='New private body' AND start_date='2026-10-14' AND due_date='2026-11-01'`, issue) != 1 ||
		dbfx.Count(t, `SELECT count(*) FROM member WHERE workspace_id=$1 AND user_id=$2`, testWorkspaceID, external) != 0 {
		t.Fatal("external edit failed or invented source membership")
	}
	testutil.Call(t, testHandler.UpdateCollaborationIssue, collaborationEditRequest(issue, external, key, body)).Want(http.StatusOK)
	if dbfx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1 AND kind='issue_edited'`, issue) != 1 {
		t.Fatal("replayed HTTP edit duplicated history")
	}
}

func TestCollaborationEditHTTPSourceFields(t *testing.T) {
	issue := collaborationEditFixture(t)
	external := dbfx.User(t, "External field editor", "field-editor-"+uuid.NewString()+"@example.test")
	dbfx.InsertNoID(t, "issue_participant", testutil.Cols{"issue_id": issue, "user_id": external}, "issue_id=$1 AND user_id=$2", issue, external)
	dbfx.InsertNoID(t, "collaboration_account_fence", testutil.Cols{"user_id": external}, "user_id=$1", external)
	dbfx.Exec(t, `UPDATE issue SET assignee_id=$2 WHERE id=$1`, issue, external)
	label := dbfx.Insert(t, "issue_label", testutil.Cols{"workspace_id": testWorkspaceID, "resource_type": "issue", "name": "Collaboration " + uuid.NewString(), "color": "#123456"})
	property := dbfx.Insert(t, "issue_property", testutil.Cols{"workspace_id": testWorkspaceID, "name": "Field " + uuid.NewString(), "type": "number"})
	dbfx.Cleanup(t, `DELETE FROM issue_to_label WHERE issue_id=$1`, issue)
	body := collaborationEditBodyValues()
	body["priority"] = "urgent"
	body["label_ids"] = []string{strings.ToUpper(label), label}
	body["properties"] = map[string]any{strings.ToUpper(property): 42}
	key := uuid.NewString()
	out := testutil.Call(t, testHandler.UpdateCollaborationIssue, collaborationEditRequest(issue, external, key, body)).Want(http.StatusOK).Map()
	if out["code"] != "issue_edited" || out["collaboration_revision"] != "2" || len(out) != 2 {
		t.Fatalf("source field edit did not return the existing minimal receipt: %+v", out)
	}
	if dbfx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND priority='urgent' AND properties=jsonb_build_object($2::text,42) AND revision=2`, issue, property) != 1 ||
		dbfx.Count(t, `SELECT count(*) FROM issue_to_label WHERE issue_id=$1 AND label_id=$2`, issue, label) != 1 ||
		dbfx.Count(t, `SELECT count(*) FROM member WHERE workspace_id=$1 AND user_id=$2`, testWorkspaceID, external) != 0 {
		t.Fatal("source fields did not commit canonically or invented membership")
	}
	testutil.Call(t, testHandler.UpdateCollaborationIssue, collaborationEditRequest(issue, external, key, body)).Want(http.StatusOK)
	if dbfx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1 AND kind='issue_edited'`, issue) != 1 {
		t.Fatal("field replay duplicated history")
	}
}

func TestCollaborationEditHTTPPresenceAndClearing(t *testing.T) {
	for _, tc := range []struct {
		name               string
		fields             map[string]any
		title, description string
		start, due         *string
		code               string
	}{
		{"omitted", nil, "Original title", "Original body", strPtr("2026-10-01"), strPtr("2026-10-31"), "unchanged"},
		{"text_null_is_untouched", map[string]any{"title": nil, "description": nil}, "Original title", "Original body", strPtr("2026-10-01"), strPtr("2026-10-31"), "unchanged"},
		{"empty_text_is_saved", map[string]any{"title": "", "description": ""}, "", "", strPtr("2026-10-01"), strPtr("2026-10-31"), "issue_edited"},
		{"date_null_clears", map[string]any{"start_date": nil}, "Original title", "Original body", nil, strPtr("2026-10-31"), "issue_edited"},
		{"empty_date_clears", map[string]any{"due_date": ""}, "Original title", "Original body", strPtr("2026-10-01"), nil, "issue_edited"},
		{"legacy_midnight", map[string]any{"due_date": "2026-11-01T08:00:00+08:00"}, "Original title", "Original body", strPtr("2026-10-01"), strPtr("2026-11-01"), "issue_edited"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			issue := collaborationEditFixture(t)
			dbfx.Exec(t, `UPDATE issue SET title='Original title',description='Original body',start_date='2026-10-01',due_date='2026-10-31' WHERE id=$1`, issue)
			body := collaborationEditBodyValues()
			for key, value := range tc.fields {
				body[key] = value
			}
			out := testutil.Call(t, testHandler.UpdateCollaborationIssue, collaborationEditRequest(issue, testUserID, uuid.NewString(), body)).Want(http.StatusOK).Map()
			if out["code"] != tc.code {
				t.Fatalf("wrong edit result: %+v", out)
			}
			if dbfx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND title=$2 AND description=$3 AND start_date IS NOT DISTINCT FROM $4::date AND due_date IS NOT DISTINCT FROM $5::date`, issue, tc.title, tc.description, tc.start, tc.due) != 1 {
				t.Fatal("PATCH collapsed omitted/null/empty values or changed an untouched field")
			}
			if tc.code == "unchanged" && (out["collaboration_revision"] != "1" || dbfx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1`, issue) != 0) {
				t.Fatal("HTTP no-op manufactured revision/history")
			}
		})
	}
}

func TestCollaborationEditHTTPStrictFieldsAndVersions(t *testing.T) {
	for _, tc := range []struct {
		name, field string
		value       any
	}{
		{"assignee", "assignee_id", nil}, {"project", "project_id", uuid.NewString()}, {"priority", "priority", "private-invalid-priority"},
		{"properties", "properties", map[string]any{"custom": "x"}}, {"labels", "labels", []string{"x"}},
		{"position", "position", 1}, {"iteration", "current_iteration_id", nil}, {"stage", "stage", 2},
		{"title_object", "title", map[string]any{}}, {"description_array", "description", []string{}},
		{"numeric_date", "start_date", 42}, {"bad_date", "due_date", "private-unparseable-date"},
		{"numeric_version", "expected_collaboration_revision", 1}, {"overflow", "expected_lifecycle_epoch", "9223372036854775808"},
		{"leading_zero", "expected_participant_epoch", "01"}, {"fraction", "expected_account_epoch", "1.2"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			issue := collaborationEditFixture(t)
			body := collaborationEditBodyValues()
			body["title"], body[tc.field] = "Should not be written", tc.value
			response := testutil.Call(t, testHandler.UpdateCollaborationIssue, collaborationEditRequest(issue, testUserID, uuid.NewString(), body)).Want(http.StatusBadRequest)
			if strings.Contains(response.Body.String(), "private-unparseable-date") {
				t.Fatal("parse error leaked the supplied value")
			}
			if dbfx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND title='Protected collaboration' AND revision=1`, issue) != 1 ||
				dbfx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1`, issue) != 0 {
				t.Fatal("invalid mixed request partially edited the issue")
			}
		})
	}
	for _, raw := range []string{
		`{"expected_collaboration_revision":"1","expected_collaboration_revision":"2"}`,
		`{"expected_collaboration_revision":"1","EXPECTED_COLLABORATION_REVISION":"2"}`,
		`{"title":"a","TITLE":"b"}`, `{"start_date":null,"ſtart_date":"2026-10-01"}`, `{} {}`, `null`, `[]`,
	} {
		r := collaborationEditRequest(uuid.NewString(), testUserID, uuid.NewString(), json.RawMessage(raw))
		testutil.Call(t, (&Handler{}).UpdateCollaborationIssue, r).Want(http.StatusBadRequest)
	}
}

func TestCollaborationEditHTTPRejectsMachineBeforeDependencies(t *testing.T) {
	body := collaborationEditBodyValues()
	body["title"] = "Machine edit"
	for _, principal := range []auth.AuthenticatedPrincipal{
		{UserID: testUserID, Kind: auth.CredentialTask}, {UserID: testUserID, Kind: auth.CredentialCloud},
		{UserID: testUserID, Kind: auth.CredentialJWT, AgentActor: true}, {UserID: testUserID, Kind: auth.CredentialPAT, AgentActor: true},
		{UserID: testUserID, Kind: auth.CredentialJWT, Restricted: true}, {UserID: testUserID},
	} {
		r := collaborationEditRequest(uuid.NewString(), testUserID, uuid.NewString(), body)
		r = r.WithContext(auth.WithAuthenticatedPrincipal(r.Context(), principal))
		testutil.Call(t, (&Handler{}).UpdateCollaborationIssue, r).Want(http.StatusForbidden)
	}
}

func TestCollaborationEditHTTPPreservesStatusPatchAndRejectsMixedIntent(t *testing.T) {
	issue := collaborationEditFixture(t)
	if err := testHandler.Queries.SeedIssueStatusEntries(context.Background(), parseUUID(testWorkspaceID)); err != nil {
		t.Fatal(err)
	}
	dbfx.Cleanup(t, `DELETE FROM issue_status WHERE workspace_id=$1`, testWorkspaceID)
	ctx := auth.WithAuthenticatedPrincipal(context.Background(), auth.AuthenticatedPrincipal{UserID: testUserID, Kind: auth.CredentialJWT})
	svc := service.NewCollaborationService(testHandler.Queries, testHandler.TxStarter, testHandler.IssueService)
	options, err := svc.StatusOptions(ctx, parseUUID(issue), "change_status")
	if err != nil {
		t.Fatal(err)
	}
	for _, fields := range []map[string]any{
		{"title": "Mixed", "status": "in_progress", "status_options_version": options.StatusOptionsVersion},
		{"description": nil, "status": "in_progress", "status_options_version": options.StatusOptionsVersion},
		{"start_date": nil, "status_options_version": options.StatusOptionsVersion},
		{"due_date": "2026-10-31", "status": nil},
		{"priority": nil, "status": "in_progress"},
		{"label_ids": nil, "status_options_version": options.StatusOptionsVersion},
		{"properties": nil, "status": nil},
	} {
		body := collaborationEditBodyValues()
		for key, value := range fields {
			body[key] = value
		}
		testutil.Call(t, testHandler.UpdateCollaborationIssue, collaborationEditRequest(issue, testUserID, uuid.NewString(), body)).Want(http.StatusBadRequest)
	}
	if dbfx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND status='todo' AND title='Protected collaboration' AND start_date IS NULL AND due_date IS NULL`, issue) != 1 ||
		dbfx.Count(t, `SELECT count(*) FROM collaboration_operation WHERE scope_id=$1`, issue) != 0 {
		t.Fatal("mixed status/metadata request had side effects")
	}
	body := collaborationEditBodyValues()
	body["status"], body["status_options_version"] = "in_progress", options.StatusOptionsVersion
	key := uuid.NewString()
	out := testutil.Call(t, testHandler.UpdateCollaborationIssue, collaborationEditRequest(issue, testUserID, key, body)).Want(http.StatusOK).Map()
	if out["code"] != "status_changed" || out["collaboration_revision"] != "2" {
		t.Fatalf("existing status PATCH contract changed: %+v", out)
	}
	// The compatibility name delegates to the same production parser/service.
	testutil.Call(t, testHandler.ChangeCollaborationStatus, collaborationEditRequest(issue, testUserID, key, body)).Want(http.StatusOK)
	if dbfx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1 AND kind='status_changed'`, issue) != 1 ||
		dbfx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1 AND kind='issue_edited'`, issue) != 0 {
		t.Fatal("status compatibility path duplicated or misclassified an event")
	}
}
