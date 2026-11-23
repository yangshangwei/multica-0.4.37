package handler

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func TestCollaborationClaimRejectsInvalidExtensionBeforeMutation(t *testing.T) {
	for _, tc := range []struct {
		query   string
		version int
	}{
		{"handoff_version=2", protocol.CollaborationHostRuntimeProtocolVersion},
		{"recording_version=0", protocol.CollaborationHostRuntimeProtocolVersion},
		{"handoff_version=1&handoff_version=1", protocol.CollaborationHostRuntimeProtocolVersion},
		{"recording_version=1", protocol.CollaborationRuntimeProtocolVersion},
	} {
		t.Run(tc.query, func(t *testing.T) {
			runID := uuid.NewString()
			request := withURLParam(newRequest(http.MethodPost, "/api/collaboration/runtime/runs/"+runID+"/claim?"+tc.query,
				protocol.CollaborationRunClaimRequest{ProtocolVersion: tc.version}), "runId", runID)
			request.Header.Set("Authorization", "Bearer mdt_fixture")
			request = request.WithContext(auth.WithPasswordSession(request.Context(), auth.PasswordSession{
				Kind: "daemon_token", UserID: uuid.NewString(), Version: 1, BindingID: uuid.NewString(), BindingEpoch: 1,
				WorkspaceID: uuid.NewString(), DaemonID: "fixture",
			}))
			var response struct {
				Code string `json:"code"`
			}
			// A bare handler has no write dependencies: reaching the claim service
			// would fail instead of silently reserving work for an unknown contract.
			testutil.Call(t, (&Handler{}).ClaimCollaborationRuntimeRun, request).Want(http.StatusBadRequest).JSON(&response)
			if response.Code != "unsupported_runtime_read_version" {
				t.Fatalf("wrong refusal: %s", response.Code)
			}
		})
	}
}
