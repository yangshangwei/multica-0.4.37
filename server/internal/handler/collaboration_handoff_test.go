package handler

import (
	"net/http"
	"testing"

	"github.com/multica-ai/multica/server/internal/testutil"
)

func TestCollaborationTransferRejectsDirectedStatusFields(t *testing.T) {
	for _, field := range []string{"expected_status", "status_options_version"} {
		for _, value := range []any{nil, "", "confirmed"} {
			t.Run(field, func(t *testing.T) {
				req := testutil.JSONRequest(http.MethodPost, "/api/collaboration/issues/unused/handoffs", map[string]any{
					"purpose": "transfer", "target_user_id": "unused", field: value,
				})
				r := testutil.Call(t, (&Handler{}).TransferCollaborationIssue, req).Want(http.StatusBadRequest).Map()
				if r["code"] != "invalid_request" {
					t.Fatalf("transfer ignored a directed confirmation field: %v", r)
				}
			})
		}
	}
}

func TestCollaborationInformationRequestRejectsNonStringStatusConfirmation(t *testing.T) {
	for _, field := range []string{"expected_status", "status_options_version"} {
		req := testutil.JSONRequest(http.MethodPost, "/api/collaboration/issues/unused/handoffs", map[string]any{
			"purpose": "information", "target_user_id": "unused", field: 1,
		})
		r := testutil.Call(t, (&Handler{}).TransferCollaborationIssue, req).Want(http.StatusBadRequest).Map()
		if r["code"] != "invalid_status_request" {
			t.Fatalf("nonstring confirmation reached the service: %v", r)
		}
	}
}
