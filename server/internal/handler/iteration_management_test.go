package handler

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/iteration"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func lifecycleHTTPHandler(t *testing.T) *Handler {
	t.Helper()
	h := iterationSettingsHandler(t)
	// Existing lifecycle scenarios use UTC dates independently of the workspace default.
	dbfx.Exec(t, "UPDATE workspace SET planning_timezone='UTC' WHERE id=$1", testWorkspaceID)
	dbfx.Cleanup(t, "UPDATE workspace SET planning_timezone=NULL WHERE id=$1", testWorkspaceID)
	dbfx.Cleanup(t, `DELETE FROM iteration_event WHERE workspace_id=$1`, testWorkspaceID)
	dbfx.Cleanup(t, `DELETE FROM iteration_participation WHERE workspace_id=$1`, testWorkspaceID)
	dbfx.Cleanup(t, `DELETE FROM iteration_snapshot WHERE workspace_id=$1`, testWorkspaceID)
	dbfx.Cleanup(t, `DELETE FROM iteration WHERE workspace_id=$1`, testWorkspaceID)
	body := enableIterationBody(uuid.NewString())
	body["confirmed_timezone"] = "UTC"
	testutil.Call(t, h.EnableIterationSettings, iterationSettingsRequest("POST", "iteration-settings/enable", body)).Want(200)
	return h
}
func lifecycleHTTPRequest(method, path, id string, body any) *http.Request {
	r := iterationSettingsRequest(method, path, body)
	if id != "" {
		chi.RouteContext(r.Context()).URLParams.Add("iterationID", id)
	}
	return r
}
func lifecycleHTTPCreate(t *testing.T, h *Handler, name string) string {
	t.Helper()
	today := time.Now().UTC()
	var result iteration.WriteResult
	body := service.CreateIterationInput{RequestID: uuid.NewString(), Name: name, StartDate: today.Format(time.DateOnly), EndDate: today.AddDate(0, 0, 13).Format(time.DateOnly), ConfirmedTimezone: "UTC"}
	testutil.Call(t, h.CreateIteration, lifecycleHTTPRequest("POST", "iterations", "", body)).Want(201).JSON(&result)
	if len(result.IterationIDs) != 1 {
		t.Fatalf("create result=%+v", result)
	}
	return result.IterationIDs[0]
}
func lifecycleHTTPApply(t *testing.T, h *Handler, draft iteration.Draft) (iteration.Preview, iteration.WriteResult) {
	t.Helper()
	if draft.Moves == nil {
		draft.Moves = []iteration.Move{}
	}
	var preview iteration.Preview
	var result iteration.WriteResult
	testutil.Call(t, h.PreviewIterationOperation, lifecycleHTTPRequest("POST", "iteration-previews", "", draft)).Want(200).JSON(&preview)
	if !preview.Complete || preview.PreviewHash == "" {
		t.Fatalf("incomplete preview=%+v", preview)
	}
	testutil.Call(t, h.ApplyIterationOperation, lifecycleHTTPRequest("POST", "iteration-operations", "", service.ApplyIterationInput{RequestID: uuid.NewString(), Draft: preview.Draft, PreviewHash: preview.PreviewHash})).Want(200).JSON(&result)
	return preview, result
}
func TestIterationLifecycleHTTPManualFlow(t *testing.T) {
	h := lifecycleHTTPHandler(t)
	first := lifecycleHTTPCreate(t, h, "Current manual cycle")
	next := lifecycleHTTPCreate(t, h, "Next manual cycle")
	issue := dbfx.Issue(t, "HTTP commitment")
	lifecycleHTTPApply(t, h, iteration.Draft{Operation: "move", ExpectedSettingsRevision: 2, Moves: []iteration.Move{{IssueID: issue, ExpectedIssueRevision: 1, TargetID: &first}}})
	row, err := h.Queries.GetIteration(context.Background(), db.GetIterationParams{WorkspaceID: parseUUID(testWorkspaceID), ID: parseUUID(first)})
	if err != nil {
		t.Fatal(err)
	}
	lifecycleHTTPApply(t, h, iteration.Draft{Operation: "start", IterationID: &first, ExpectedIterationRevision: &row.Revision, ExpectedScopeRevision: &row.ScopeRevision, ExpectedSettingsRevision: 2, Start: &iteration.StartDraft{TargetID: first, Mode: "scheduled", TerminalChoices: []iteration.TerminalChoice{}}})
	testutil.Call(t, h.UpdateIssue, withURLParam(newRequest("PUT", "/api/issues/"+issue, map[string]any{"status": "done"}), "id", issue)).Want(200)
	var detail struct {
		Statistics iteration.Statistics `json:"statistics"`
	}
	testutil.Call(t, h.GetIteration, lifecycleHTTPRequest("GET", "iterations/"+first, first, nil)).Want(200).JSON(&detail)
	if detail.Statistics.Original != 1 || detail.Statistics.Completed != 1 || detail.Statistics.OriginalCompleted != 1 {
		t.Fatalf("real API projection=%+v", detail.Statistics)
	}
	testutil.Call(t, h.UpdateIssue, withURLParam(newRequest("PUT", "/api/issues/"+issue, map[string]any{"status": "todo"}), "id", issue)).Want(200)
	current, err := h.Queries.GetIssueInWorkspace(context.Background(), db.GetIssueInWorkspaceParams{WorkspaceID: parseUUID(testWorkspaceID), ID: parseUUID(issue)})
	if err != nil {
		t.Fatal(err)
	}
	reason := "Plan for next cycle"
	lifecycleHTTPApply(t, h, iteration.Draft{Operation: "move", ExpectedSettingsRevision: 2, Reason: &reason, Moves: []iteration.Move{{IssueID: issue, ExpectedIssueRevision: current.Revision, ExpectedSourceID: &first, TargetID: &next}}})
	row, err = h.Queries.GetIteration(context.Background(), db.GetIterationParams{WorkspaceID: parseUUID(testWorkspaceID), ID: parseUUID(next)})
	if err != nil {
		t.Fatal(err)
	}
	lifecycleHTTPApply(t, h, iteration.Draft{Operation: "cancel", IterationID: &next, ExpectedIterationRevision: &row.Revision, ExpectedScopeRevision: &row.ScopeRevision, ExpectedSettingsRevision: 2, Reason: &reason})
	if n := dbfx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND current_iteration_id IS NULL AND iteration_rollover_count=0`, issue); n != 1 {
		t.Fatal("planned cancellation changed rollover or retained pointer")
	}
	testutil.Call(t, h.GetIteration, lifecycleHTTPRequest("GET", "iterations/"+first, first, nil)).Want(200).JSON(&detail)
	if detail.Statistics.Original != 1 || detail.Statistics.Current != 0 {
		t.Fatal("moving out erased original commitment")
	}
}
func TestIterationLifecycleHTTPReleaseAndRawValidation(t *testing.T) {
	body := service.CreateIterationInput{RequestID: uuid.NewString(), Name: "Closed release", StartDate: "2026-10-06", EndDate: "2026-10-19", ConfirmedTimezone: "UTC"}
	testutil.Call(t, testHandler.CreateIteration, lifecycleHTTPRequest("POST", "iterations", "", body)).Want(422)
	h := lifecycleHTTPHandler(t)
	bad := map[string]any{"operation": "move", "expected_settings_revision": 9007199254740992.0, "moves": []any{}}
	testutil.Call(t, h.PreviewIterationOperation, lifecycleHTTPRequest("POST", "iteration-previews", "", bad)).Want(400)
	request := lifecycleHTTPRequest("POST", "iterations", "", body)
	request.Header.Set("X-Actor-Source", "cloud_pat")
	testutil.Call(t, h.CreateIteration, request).Want(403)
}

func TestIterationLifecycleHTTPRejectsDuplicateEnvelopeFields(t *testing.T) {
	h := lifecycleHTTPHandler(t)
	for _, field := range []string{"name", "NAME"} {
		raw := fmt.Sprintf(`{"request_id":%q,"name":"First",%q:"Second","start_date":"2026-10-06","end_date":"2026-10-19","confirmed_timezone":"UTC"}`, uuid.NewString(), field)
		request := lifecycleHTTPRequest("POST", "iterations", "", nil)
		request.Body = io.NopCloser(strings.NewReader(raw))
		request.ContentLength = int64(len(raw))
		testutil.Call(t, h.CreateIteration, request).Want(400)
	}
	period := lifecycleHTTPCreate(t, h, "Strict edit")
	raw := fmt.Sprintf(`{"request_id":%q,"expected_revision":1,"fields":{"name":"One","name":"Two"}}`, uuid.NewString())
	request := lifecycleHTTPRequest("PUT", "iterations/"+period, period, nil)
	request.Body = io.NopCloser(strings.NewReader(raw))
	request.ContentLength = int64(len(raw))
	testutil.Call(t, h.UpdateIteration, request).Want(400)
	if n := dbfx.Count(t, `SELECT count(*) FROM iteration WHERE id=$1 AND name='Strict edit' AND revision=1`, period); n != 1 {
		t.Fatal("duplicate fields mutated iteration")
	}
}

func TestIterationLifecycleHTTPRejectsUnicodeFieldAliases(t *testing.T) {
	h := lifecycleHTTPHandler(t)
	raw := fmt.Sprintf(`{"request_id":%q,"name":"Ambiguous calendar","start_date":"2026-10-06","ſtart_date":"2026-10-07","end_date":"2026-10-19","confirmed_timezone":"UTC"}`, uuid.NewString())
	request := lifecycleHTTPRequest("POST", "iterations", "", nil)
	request.Body = io.NopCloser(strings.NewReader(raw))
	request.ContentLength = int64(len(raw))
	testutil.Call(t, h.CreateIteration, request).Want(400)
}
