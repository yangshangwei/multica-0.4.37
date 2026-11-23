package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
)

func collaborationFieldsReadRequest(issue, actor, suffix string) *http.Request {
	r := withURLParam(newRequest(http.MethodGet, "/api/collaboration/issues/"+issue+suffix, nil), "id", issue)
	r.Header.Set("X-Workspace-ID", "unrelated-workspace")
	r.Header.Set("X-Workspace-Slug", "unrelated-workspace")
	return r.WithContext(auth.WithAuthenticatedPrincipal(r.Context(), auth.AuthenticatedPrincipal{UserID: actor, Kind: auth.CredentialJWT}))
}

func TestCollaborationFieldsHTTPOptionsAndDetail(t *testing.T) {
	issue := collaborationEditFixture(t)
	external := dbfx.User(t, "External field viewer", "private-field-"+uuid.NewString()+"@example.test")
	dbfx.InsertNoID(t, "issue_participant", testutil.Cols{"issue_id": issue, "user_id": external}, "issue_id=$1 AND user_id=$2", issue, external)
	dbfx.Exec(t, `UPDATE issue SET assignee_id=$2 WHERE id=$1`, issue, external)
	dbfx.Exec(t, `UPDATE issue_collaboration SET revision=9007199254740993 WHERE issue_id=$1`, issue)
	label := dbfx.Insert(t, "issue_label", testutil.Cols{"workspace_id": testWorkspaceID, "resource_type": "issue", "name": "Source field label " + uuid.NewString(), "description": "private-management-description", "color": "#123456"})
	dbfx.InsertNoID(t, "issue_to_label", testutil.Cols{"issue_id": issue, "label_id": label}, "issue_id=$1 AND label_id=$2", issue, label)
	active := dbfx.Insert(t, "issue_property", testutil.Cols{"workspace_id": testWorkspaceID, "name": "Active definition " + uuid.NewString(), "type": "checkbox", "description": "private-management-description"})
	archived := dbfx.Insert(t, "issue_property", testutil.Cols{"workspace_id": testWorkspaceID, "name": "Archived actor " + uuid.NewString(), "type": "actor", "archived_at": time.Now(), "description": "private-management-description"})
	unreferenced := dbfx.Insert(t, "issue_property", testutil.Cols{"workspace_id": testWorkspaceID, "name": "Unreferenced definition " + uuid.NewString(), "type": "text"})
	dbfx.Insert(t, "issue_property", testutil.Cols{"workspace_id": uuid.NewString(), "name": "private-foreign-definition", "type": "text"})
	dbfx.Insert(t, "issue_label", testutil.Cols{"workspace_id": testWorkspaceID, "resource_type": "agent", "name": "private-agent-label", "color": "#123456"})
	values, err := json.Marshal(map[string]any{active: false, archived: "member:" + uuid.NewString()})
	if err != nil {
		t.Fatal(err)
	}
	dbfx.Exec(t, `UPDATE issue SET properties=$2::jsonb WHERE id=$1`, issue, values)
	r := collaborationFieldsReadRequest(issue, external, "/field-options")
	response := testutil.Call(t, testHandler.GetCollaborationFieldOptions, r).Want(http.StatusOK)
	var options service.CollaborationFieldOptions
	response.JSON(&options)
	if options.IssueID != issue || options.SourceWorkspaceID != testWorkspaceID || options.CollaborationRevision != 9007199254740993 || len(options.Labels) != 1 || options.Labels[0].ID != label || len(options.Properties) != 2 {
		t.Fatalf("field options are not scoped to the source issue: %+v", options)
	}
	for _, definition := range options.Properties {
		if definition.Archived || (definition.ID != active && definition.ID != unreferenced) {
			t.Fatalf("editing options included an unavailable definition: %+v", definition)
		}
	}
	encoded := response.Map()
	if len(encoded) != 5 || len(encoded["labels"].([]any)[0].(map[string]any)) != 3 || len(encoded["properties"].([]any)[0].(map[string]any)) != 6 {
		t.Fatalf("options contain more than the minimum field summaries: %+v", encoded)
	}
	if response.Header().Get("Cache-Control") != "no-store" || r.Header.Get("X-Workspace-ID") != "" || r.Header.Get("X-Workspace-Slug") != "" {
		t.Fatal("field options retained cacheability or ambient workspace authority")
	}
	for _, private := range []string{"private-", "usage_count", "position", "updated_at", "created_at", "email", "username"} {
		if strings.Contains(response.Body.String(), private) {
			t.Fatalf("options exposed management or directory data: %s", private)
		}
	}
	var detail CollaborationIssueResponse
	detailResponse := testutil.Call(t, testHandler.GetCollaborationIssue, collaborationFieldsReadRequest(issue, external, "")).Want(http.StatusOK)
	detailResponse.JSON(&detail)
	if len(detail.Labels) != 1 || detail.Labels[0].ID != label || len(detail.Properties) != 2 || len(detail.PropertyDefinitions) != 2 || detail.Revision != "9007199254740993" {
		t.Fatalf("detail lost selected field values/definitions: %+v", detail.CollaborationIssueFields)
	}
	archivedFound := false
	for _, definition := range detail.PropertyDefinitions {
		if definition.ID == unreferenced {
			t.Fatal("detail exposed an unreferenced source definition")
		}
		archivedFound = archivedFound || definition.ID == archived && definition.Archived
	}
	if !archivedFound || strings.Contains(detailResponse.Body.String(), "private-") || strings.Contains(detailResponse.Body.String(), "usage_count") {
		t.Fatal("archived values are unreadable or detail leaked management metadata")
	}
	dbfx.Exec(t, `UPDATE issue SET assignee_id=$2 WHERE id=$1`, issue, testUserID)
	testutil.Call(t, testHandler.GetCollaborationFieldOptions, collaborationFieldsReadRequest(issue, external, "/field-options")).Want(http.StatusForbidden)
	// A former handler still reads this issue's selected values through its
	// participation, but cannot use the catalog as an editing directory.
	testutil.Call(t, testHandler.GetCollaborationIssue, collaborationFieldsReadRequest(issue, external, "")).Want(http.StatusOK)
	dbfx.Exec(t, `UPDATE issue_participant SET revoked_at=now() WHERE issue_id=$1 AND user_id=$2`, issue, external)
	testutil.Call(t, testHandler.GetCollaborationFieldOptions, collaborationFieldsReadRequest(issue, external, "/field-options")).Want(http.StatusNotFound)
	testutil.Call(t, testHandler.GetCollaborationIssue, collaborationFieldsReadRequest(issue, external, "")).Want(http.StatusNotFound)
}

func TestCollaborationFieldsHTTPPreserveAndClear(t *testing.T) {
	for _, tc := range []struct {
		name  string
		patch map[string]any
		clear bool
	}{
		{"omitted", map[string]any{}, false},
		{"whole_null", map[string]any{"priority": nil, "label_ids": nil, "properties": nil}, false},
		{"empty_properties", map[string]any{"properties": map[string]any{}}, false},
		{"clear", map[string]any{"label_ids": []string{}}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			issue := collaborationEditFixture(t)
			label := dbfx.Insert(t, "issue_label", testutil.Cols{"workspace_id": testWorkspaceID, "resource_type": "issue", "name": "Kept label " + uuid.NewString(), "color": "#123456"})
			property := dbfx.Insert(t, "issue_property", testutil.Cols{"workspace_id": testWorkspaceID, "name": "Clearable field " + uuid.NewString(), "type": "checkbox", "archived_at": time.Now()})
			dbfx.InsertNoID(t, "issue_to_label", testutil.Cols{"issue_id": issue, "label_id": label}, "issue_id=$1 AND label_id=$2", issue, label)
			dbfx.Exec(t, `UPDATE issue SET priority='high',properties=jsonb_build_object($2::text,true) WHERE id=$1`, issue, property)
			body := collaborationEditBodyValues()
			for key, value := range tc.patch {
				body[key] = value
			}
			if tc.clear {
				body["properties"] = map[string]any{property: nil}
			}
			out := testutil.Call(t, testHandler.UpdateCollaborationIssue, collaborationEditRequest(issue, testUserID, uuid.NewString(), body)).Want(http.StatusOK).Map()
			code, revision, labels := "unchanged", "1", 1
			if tc.clear {
				code, revision, labels = "issue_edited", "2", 0
			}
			if out["code"] != code || out["collaboration_revision"] != revision || dbfx.Count(t, `SELECT count(*) FROM issue_to_label WHERE issue_id=$1`, issue) != labels {
				t.Fatalf("null/omission/clear was collapsed: %+v", out)
			}
			if tc.clear {
				if dbfx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND priority='high' AND properties='{}'::jsonb`, issue) != 1 {
					t.Fatal("clear modified priority or retained property")
				}
			} else if dbfx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND priority='high' AND properties=jsonb_build_object($2::text,true)`, issue, property) != 1 {
				t.Fatal("preserve removed existing fields")
			}
		})
	}
}

func TestCollaborationFieldsHTTPRejectsMalformedFields(t *testing.T) {
	issue := collaborationEditFixture(t)
	for _, fields := range []map[string]any{
		{"priority": 1}, {"priority": ""}, {"label_ids": "private-invalid-id"}, {"label_ids": []any{nil}},
		{"label_ids": []string{"private-invalid-id"}}, {"properties": []any{}}, {"properties": "private-invalid-value"},
		{"properties": map[string]any{"private-invalid-id": true}},
	} {
		body := collaborationEditBodyValues()
		body["title"] = "Must not be saved"
		for key, value := range fields {
			body[key] = value
		}
		response := testutil.Call(t, testHandler.UpdateCollaborationIssue, collaborationEditRequest(issue, testUserID, uuid.NewString(), body)).Want(http.StatusBadRequest)
		if strings.Contains(response.Body.String(), "private-") || dbfx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND title='Protected collaboration' AND revision=1`, issue) != 1 {
			t.Fatal("malformed fields leaked their values or partially changed metadata")
		}
	}
	for _, principal := range []auth.AuthenticatedPrincipal{{UserID: testUserID, Kind: auth.CredentialTask}, {UserID: testUserID, Kind: auth.CredentialCloud}, {UserID: testUserID}, {UserID: testUserID, Kind: auth.CredentialPAT, AgentActor: true}, {UserID: testUserID, Kind: auth.CredentialJWT, Restricted: true}} {
		r := collaborationFieldsReadRequest(uuid.NewString(), testUserID, "/field-options")
		r = r.WithContext(auth.WithAuthenticatedPrincipal(r.Context(), principal))
		testutil.Call(t, (&Handler{}).GetCollaborationFieldOptions, r).Want(http.StatusNotFound)
	}
}

func TestCollaborationFieldsHTTPBrokenDefinitionIsUnavailable(t *testing.T) {
	issue := collaborationEditFixture(t)
	property := dbfx.Insert(t, "issue_property", testutil.Cols{"workspace_id": testWorkspaceID, "name": "private-broken-config", "type": "select", "config": json.RawMessage(`{"options":"private-malformed-options"}`)})
	dbfx.Exec(t, `UPDATE issue SET properties=jsonb_build_object($2::text,'private-option') WHERE id=$1`, issue, property)
	for _, endpoint := range []struct {
		suffix  string
		handler http.HandlerFunc
	}{{"/field-options", testHandler.GetCollaborationFieldOptions}, {"", testHandler.GetCollaborationIssue}} {
		response := testutil.Call(t, endpoint.handler, collaborationFieldsReadRequest(issue, testUserID, endpoint.suffix)).Want(http.StatusServiceUnavailable)
		if response.Map()["code"] != "collaboration_unavailable" || strings.Contains(response.Body.String(), "private-") {
			t.Fatal("broken definition was silently downgraded or leaked parser details")
		}
	}
}

type collaborationFieldsSnapshotStarter struct {
	service.TxStarter
	ready, release chan struct{}
	once           sync.Once
}

type collaborationFieldsSnapshotTx struct {
	pgx.Tx
	gate *collaborationFieldsSnapshotStarter
}

func (s *collaborationFieldsSnapshotStarter) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &collaborationFieldsSnapshotTx{Tx: tx, gate: s}, nil
}

func (tx *collaborationFieldsSnapshotTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if strings.Contains(sql, "-- name: ListLabelsByIssue") {
		tx.gate.once.Do(func() { close(tx.gate.ready) })
		select {
		case <-tx.gate.release:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return tx.Tx.Query(ctx, sql, args...)
}

func TestCollaborationFieldsHTTPDetailUsesOneSnapshot(t *testing.T) {
	issue := collaborationEditFixture(t)
	property := dbfx.Insert(t, "issue_property", testutil.Cols{"workspace_id": testWorkspaceID, "name": "Before " + uuid.NewString(), "type": "number"})
	beforeLabel := dbfx.Insert(t, "issue_label", testutil.Cols{"workspace_id": testWorkspaceID, "resource_type": "issue", "name": "Before " + uuid.NewString(), "color": "#123456"})
	afterLabel := dbfx.Insert(t, "issue_label", testutil.Cols{"workspace_id": testWorkspaceID, "resource_type": "issue", "name": "After " + uuid.NewString(), "color": "#123456"})
	dbfx.InsertNoID(t, "issue_to_label", testutil.Cols{"issue_id": issue, "label_id": beforeLabel}, "issue_id=$1 AND label_id=$2", issue, beforeLabel)
	dbfx.Cleanup(t, `DELETE FROM issue_to_label WHERE issue_id=$1`, issue)
	dbfx.Exec(t, `UPDATE issue SET priority='low',properties=jsonb_build_object($2::text,1) WHERE id=$1`, issue, property)
	gate := &collaborationFieldsSnapshotStarter{TxStarter: testHandler.TxStarter, ready: make(chan struct{}), release: make(chan struct{})}
	var release sync.Once
	unblock := func() { release.Do(func() { close(gate.release) }) }
	defer unblock()
	scoped := *testHandler
	scoped.TxStarter = gate
	r := collaborationFieldsReadRequest(issue, testUserID, "")
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	r = r.WithContext(ctx)
	result := make(chan *testutil.Response, 1)
	go func() { result <- testutil.Call(t, scoped.GetCollaborationIssue, r) }()
	select {
	case <-gate.ready:
	case early := <-result:
		t.Fatalf("detail ended before the field-read barrier: %d %s", early.Code, early.Body.String())
	case <-ctx.Done():
		t.Fatal("detail did not reach the field-read barrier")
	}
	body := collaborationEditBodyValues()
	body["priority"], body["label_ids"], body["properties"] = "high", []string{afterLabel}, map[string]any{property: 2}
	testutil.Call(t, testHandler.UpdateCollaborationIssue, collaborationEditRequest(issue, testUserID, uuid.NewString(), body)).Want(http.StatusOK)
	dbfx.Exec(t, `UPDATE issue_property SET name='After concurrent edit' WHERE id=$1`, property)
	unblock()
	var before CollaborationIssueResponse
	(<-result).Want(http.StatusOK).JSON(&before)
	if before.Revision != "1" || before.Priority != "low" || len(before.Labels) != 1 || before.Labels[0].ID != beforeLabel || string(before.Properties[property]) != "1" || len(before.PropertyDefinitions) != 1 || !strings.HasPrefix(before.PropertyDefinitions[0].Name, "Before ") {
		t.Fatalf("one detail spliced field values/definitions from different revisions: %+v", before)
	}
	var after CollaborationIssueResponse
	testutil.Call(t, testHandler.GetCollaborationIssue, collaborationFieldsReadRequest(issue, testUserID, "")).Want(http.StatusOK).JSON(&after)
	if after.Revision != "2" || after.Priority != "high" || len(after.Labels) != 1 || after.Labels[0].ID != afterLabel || string(after.Properties[property]) != "2" || after.PropertyDefinitions[0].Name != "After concurrent edit" {
		t.Fatalf("fresh detail did not show the committed field edit: %+v", after)
	}
}
