package handler

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/testutil"
)

func TestCollaborationControlHTTPRequiresExplicitScopeAndExactVersions(t *testing.T) {
	bare := &Handler{}
	for _, body := range []string{
		`{"action":"resume","scope":"unknown"}`,
		`{"action":"stop","scope":"issue","expected_execution_epoch":"1"}`,
		`{"action":"stop","scope":"issue","expected_lifecycle_epoch":"1","expected_participant_epoch":"1","expected_account_epoch":"1","expected_control_version":"01","expected_target_set_version":"1","targets":[]}`,
		`{"action":"stop","scope":"target","expected_lifecycle_epoch":"1","expected_participant_epoch":"1","expected_account_epoch":"1","expected_control_version":"1","targets":[]}`,
		`{"action":"stop","action":"resume","scope":"issue"}`,
	} {
		r := collaborationContentRequest(http.MethodPost, uuid.NewString(), "", testUserID, uuid.NewString(), json.RawMessage(body))
		response := testutil.Call(t, bare.ControlCollaborationIssue, r).Want(http.StatusBadRequest)
		if response.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("control validation failure can be cached")
		}
	}
}
