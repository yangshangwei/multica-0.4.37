package handler

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// A well-formed source UUID that names nothing this run may read is a wrong
// reference, not an outage. The model must be told not to retry it.
func TestCollaborationRuntimeHandoffSourceWrongReferenceIsNotFound(t *testing.T) {
	f := runtimeCredentialHostFixture(t)
	runID := f.claim.Identity.RunID
	const body = "wrong-reference control note"
	handoffID := dbfx.Insert(t, "issue_handoff", testutil.Cols{"issue_id": f.issue, "actor_user_id": testUserID,
		"from_user_id": testUserID, "to_user_id": testUserID, "purpose": "transfer", "previous_status": "todo", "note": body})
	dbfx.Cleanup(t, `DELETE FROM issue_handoff WHERE id=$1`, handoffID)
	router := chi.NewRouter()
	router.Get("/api/collaboration/runtime/runs/{runId}/handoff/sources/{sourceId}", f.handler.GetCollaborationRuntimeHandoffSource)
	call := func(t *testing.T, sourceID string, query url.Values) *testutil.Response {
		t.Helper()
		r := testutil.JSONRequest(http.MethodGet, "/api/collaboration/runtime/runs/"+runID+"/handoff/sources/"+sourceID+"?"+query.Encode(), nil)
		r.Header.Set("Authorization", "Bearer "+f.permit.BusinessToken)
		return testutil.Call(t, router.ServeHTTP, r.WithContext(f.ctx))
	}
	digest := protocol.CollaborationHandoffHash(body)
	call(t, handoffID, url.Values{"kind": {"handoff"}, "sha256": {digest}}).Want(http.StatusOK)

	cases := []struct {
		name, sourceID string
		query          url.Values
	}{
		{"unknown_handoff", uuid.NewString(), url.Values{"kind": {"handoff"}, "sha256": {digest}}},
		// A run ID copied into the source position is the reported mistake.
		{"run_id_as_result", runID, url.Values{"kind": {"provider_result"}, "source_run_id": {runID}, "version": {"1"}, "sha256": {digest}}},
		{"run_id_as_delegation", runID, url.Values{"kind": {"delegation"}, "source_run_id": {runID}, "version": {"1"}, "sha256": {digest}}},
		{"unknown_squad_activity", uuid.NewString(), url.Values{"kind": {"squad_activity"}, "source_run_id": {runID}, "version": {"1"}, "sha256": {digest}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			response := call(t, tc.sourceID, tc.query).Want(http.StatusNotFound)
			if response.Map()["code"] != "handoff_source_not_found" || strings.Contains(response.Text(), body) {
				t.Fatalf("wrong reference was not reported as not found: %s", response.Text())
			}
		})
	}
}
