package handler

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/testutil"
)

func TestCollaborationInformationMetadataWirePresence(t *testing.T) {
	for _, body := range []string{
		`{"purpose":"information","unblock_condition":null,"estimated_resolution_at":"2026-11-19T00:00:00Z"}`,
		`{"purpose":"information","unblock_condition":"condition"}`,
		`{"purpose":"information","estimated_resolution_at":"2026-11-19T00:00:00Z"}`,
		`{"purpose":"information","unblock_condition":"condition","estimated_resolution_at":null}`,
		`{"purpose":"information","unblock_condition":1,"estimated_resolution_at":"2026-11-19T00:00:00Z"}`,
		`{"purpose":"information","unblock_condition":"condition","UnblockCondition":"changed","estimated_resolution_at":"2026-11-19T00:00:00Z"}`,
		`{"purpose":"transfer","unblock_condition":null}`,
		`{"purpose":"information","information_repair":null}`,
	} {
		req := testutil.JSONRequest(http.MethodPost, "/api/collaboration/issues/unused/handoffs", json.RawMessage(body))
		req.Header.Set("Content-Type", "application/json")
		testutil.Call(t, (&Handler{}).TransferCollaborationIssue, req).Want(http.StatusBadRequest)
	}
}

func TestCollaborationInformationRepairWireClosedBasis(t *testing.T) {
	base := `{"purpose":"information_repair","target_user_id":"aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa","note":"Explicit repair","expected_collaboration_revision":"1","expected_lifecycle_epoch":"1","expected_participant_epoch":"1","expected_account_epoch":"1","expected_status":"todo","status_options_version":"` + strings.Repeat("a", 64) + `","information_repair":%s}`
	for _, repair := range []string{
		`null`,
		`{"context_version":"` + strings.Repeat("a", 64) + `","basis":{"kind":"human_selected"}}`,
		`{"context_version":"` + strings.Repeat("a", 64) + `","active_request_id":1,"basis":{"kind":"human_selected"}}`,
		`{"context_version":"` + strings.Repeat("a", 64) + `","active_request_id":null,"basis":{"kind":"unknown"}}`,
		`{"context_version":"` + strings.Repeat("a", 64) + `","active_request_id":null,"basis":{"kind":"human_selected","submission_id":null}}`,
		`{"context_version":"` + strings.Repeat("a", 64) + `","active_request_id":null,"basis":{"kind":"pending_submission","submission_id":null,"submission_event_id":null}}`,
		`{"context_version":"` + strings.Repeat("a", 64) + `","active_request_id":null,"basis":{"kind":"historical_information","origin_request_id":"AAAAAAAA-AAAA-4AAA-8AAA-AAAAAAAAAAAA","origin_event_id":"aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"}}`,
	} {
		body := strings.Replace(base, "%s", repair, 1)
		req := testutil.JSONRequest(http.MethodPost, "/api/collaboration/issues/aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa/handoffs", json.RawMessage(body))
		req.Header.Set("Content-Type", "application/json")
		testutil.Call(t, (&Handler{}).TransferCollaborationIssue, req).Want(http.StatusBadRequest)
	}
}

func TestCollaborationInformationHistoryWireAuthorityAndQueries(t *testing.T) {
	t.Setenv("MULTICA_AUTH_MODE", "legacy")
	issue := collaborationBoundaryIssue(t, testWorkspaceID)
	insertHandoff := func(t *testing.T, createdAt time.Time) string {
		t.Helper()
		return dbfx.Insert(t, "issue_handoff", testutil.Cols{"issue_id": issue, "actor_user_id": testUserID,
			"from_user_id": testUserID, "to_user_id": testUserID, "purpose": "information", "note": "PRIVATE_INFORMATION_HISTORY_SENTINEL",
			"requester_user_id": testUserID, "previous_status": "todo", "resolution": "returned", "created_at": createdAt})
	}
	handoff := insertHandoff(t, time.Unix(2, 0).UTC())
	read := func(path string, principal auth.AuthenticatedPrincipal) *http.Request {
		req := testutil.WithURLParams(testutil.JSONRequest(http.MethodGet, path, nil), "id", issue, "handoffID", handoff)
		req.Header.Set("X-Workspace-ID", "foreign-ambient-space")
		return req.WithContext(auth.WithAuthenticatedPrincipal(req.Context(), principal))
	}
	human := auth.AuthenticatedPrincipal{UserID: testUserID, Kind: auth.CredentialJWT, ExpiresAt: time.Now().Add(time.Hour)}
	path := "/api/collaboration/issues/" + issue + "/handoffs"
	response := testutil.Call(t, testHandler.ListCollaborationInformationHistory, read(path+"?limit=1", human)).Want(http.StatusOK)
	out := response.Map()
	if out["issue_id"] != issue || out["protocol_version"] != float64(1) || out["next_cursor"] != nil || out["upper_handoff"] == nil {
		t.Fatalf("history envelope: %v", out)
	}
	items := out["items"].([]any)
	if len(items) != 1 {
		t.Fatal("exact handoff disappeared from the page")
	}
	item := items[0].(map[string]any)
	provenance := item["provenance"].(map[string]any)
	if item["handoff_id"] != handoff || item["kind"] != "information_request" || provenance["state"] != "legacy_unlinked" ||
		provenance["event_id"] != nil || provenance["event_sequence"] != nil || item["note"] != "PRIVATE_INFORMATION_HISTORY_SENTINEL" {
		t.Fatalf("legacy provenance/body was fabricated or omitted: %v", item)
	}
	if response.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("private history became cacheable")
	}
	t.Run("collection_follows_returned_cursor", func(t *testing.T) {
		older := insertHandoff(t, time.Unix(1, 0).UTC())
		first := testutil.Call(t, testHandler.ListCollaborationInformationHistory, read(path+"?limit=1", human)).Want(http.StatusOK).Map()
		cursor, ok := first["next_cursor"].(string)
		if !ok || cursor == "" {
			t.Fatal("expected a cursor for the remaining handoff")
		}
		last := testutil.Call(t, testHandler.ListCollaborationInformationHistory, read(path+"?limit=1&cursor="+cursor, human)).Want(http.StatusOK).Map()
		items := last["items"].([]any)
		if len(items) != 1 || items[0].(map[string]any)["handoff_id"] != older || last["next_cursor"] != nil {
			t.Fatalf("expected only the older handoff on the final page: %v", last)
		}
	})
	for _, query := range []string{"?limit=0", "?limit=51", "?limit=01", "?limit=1&limit=2", "?max_bytes=65536", "?cursor=" + strings.Repeat("a", 2049)} {
		if code := testutil.Call(t, testHandler.ListCollaborationInformationHistory, read(path+query, human)).Want(http.StatusBadRequest).Map()["code"]; code != "invalid_information_query" {
			t.Fatalf("query %q: unexpected code %v", query, code)
		}
		machine := human
		machine.Kind = auth.CredentialTask
		denied := testutil.Call(t, testHandler.ListCollaborationInformationHistory, read(path+query, machine)).Want(http.StatusNotFound)
		if strings.Contains(denied.Body.String(), "PRIVATE_INFORMATION_HISTORY_SENTINEL") {
			t.Fatal("query decoding or machine denial leaked private history")
		}
	}
	detailPath := path + "/" + handoff
	affectedPath := detailPath + "/superseded-requests"
	t.Run("exact_handoff_without_query", func(t *testing.T) {
		out := testutil.Call(t, testHandler.GetCollaborationInformationHandoff, read(detailPath, human)).Want(http.StatusOK).Map()
		if out["item"].(map[string]any)["handoff_id"] != handoff {
			t.Fatalf("expected the requested handoff: %v", out)
		}
	})
	t.Run("affected_valid_limit_on_nonrepair", func(t *testing.T) {
		if code := testutil.Call(t, testHandler.ListCollaborationInformationRepairAffected, read(affectedPath+"?limit=1", human)).Want(http.StatusNotFound).Map()["code"]; code != "not_found" {
			t.Fatalf("expected non-repair lookup denial, got %v", code)
		}
	})
	outsider := human
	outsider.UserID = dbfx.User(t, "Unrelated history reader", "history-reader-"+issue+"@example.test")
	for _, route := range []struct {
		name, path, malformed string
		handler               http.HandlerFunc
	}{
		{"collection", path, "limit=%", testHandler.ListCollaborationInformationHistory},
		{"exact_handoff", detailPath, "label=50%", testHandler.GetCollaborationInformationHandoff},
		{"affected", affectedPath, "cursor=%", testHandler.ListCollaborationInformationRepairAffected},
	} {
		for _, query := range []struct{ name, raw string }{
			{"rejects_malformed_pair", route.malformed},
			{"rejects_partial_query", "limit=1&cursor=%"},
		} {
			t.Run(route.name+"/"+query.name, func(t *testing.T) {
				out := testutil.Call(t, route.handler, read(route.path+"?"+query.raw, human)).Want(http.StatusBadRequest).Map()
				if out["code"] != "invalid_information_query" {
					t.Fatalf("expected invalid_information_query, got %v", out["code"])
				}
			})
		}
		t.Run(route.name+"/checks_issue_authority_before_query", func(t *testing.T) {
			denied := testutil.Call(t, route.handler, read(route.path+"?"+route.malformed, outsider)).Want(http.StatusNotFound)
			if denied.Map()["code"] != "not_found" || strings.Contains(denied.Text(), "PRIVATE_INFORMATION_HISTORY_SENTINEL") {
				t.Fatal("unrelated human did not receive a content-free not_found")
			}
		})
	}
}
