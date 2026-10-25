package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func TestCollaborationExecutionNegotiationPreservesQueryScope(t *testing.T) {
	for _, test := range []struct {
		query           string
		status, version int
	}{
		{"?limit=20", 204, 5}, {"?limit=20&execution_view_version=6", 204, 6}, {"?execution_view_version=5&limit=20", 204, 5},
		{"?execution_view_version=7", 400, 0}, {"?execution_view_version=6&execution_view_version=5", 400, 0}, {"?execution_view_version=06", 400, 0},
	} {
		t.Run(test.query, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/collaboration/issues/test"+test.query, nil)
			testutil.Call(t, func(w http.ResponseWriter, r *http.Request) {
				if !collaborationExecutionRead(w, r) {
					return
				}
				if service.CollaborationExecutionViewVersion(r.Context()) != test.version || r.URL.Query().Get("execution_view_version") != "" || r.URL.Query().Get("limit") != "20" {
					t.Error("read negotiation changed scope or lost explicit version")
				}
				w.WriteHeader(http.StatusNoContent)
			}, req).Want(test.status)
		})
	}
}

func TestCollaborationRuntimeDecoderRejectsNewFieldsOnLegacyMessages(t *testing.T) {
	for _, test := range []struct {
		name   string
		target func() any
		extra  string
	}{
		{"permit version null", func() any { return &protocol.CollaborationRunPermitRequest{} }, `"protocol_version":null`},
		{"permit version zero", func() any { return &protocol.CollaborationRunPermitRequest{} }, `"protocol_version":0`},
		{"permit descriptor", func() any { return &protocol.CollaborationRunPermitRequest{} }, `"launch_descriptor":null`},
		{"proof launches", func() any { return &protocol.CollaborationRunProof{} }, `"launches":null`},
		{"proof cleanup", func() any { return &protocol.CollaborationRunProof{} }, `"cleanup":null`},
		{"outcome containment", func() any { return &protocol.CollaborationRunOutcome{} }, `"containment":""`},
	} {
		t.Run(test.name, func(t *testing.T) {
			raw, _ := json.Marshal(test.target())
			body := strings.TrimSuffix(string(raw), "}") + "," + test.extra + "}"
			req := httptest.NewRequest(http.MethodPost, "/runtime", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			testutil.Call(t, func(w http.ResponseWriter, r *http.Request) {
				if decodeCollaborationRuntimeBody(w, r, test.target()) {
					w.WriteHeader(http.StatusNoContent)
				}
			}, req).Want(http.StatusBadRequest)
		})
	}
}

func TestCollaborationExecutionV6ReadsHostAndV5KeepsHumanAccess(t *testing.T) {
	ids := collaborationExecutionReadFixture(t)
	dbfx.Exec(t, `UPDATE collaboration_run SET containment='host_process_supervision_v1' WHERE target_id=$1`, ids.target)
	request := collaborationFieldsReadRequest(ids.issue, testUserID, "")
	request.URL.RawQuery = "execution_view_version=6"
	var current CollaborationIssueResponse
	testutil.Call(t, testHandler.GetCollaborationIssue, request).Want(http.StatusOK).JSON(&current)
	if current.Execution.ProtocolVersion != 6 || len(current.Execution.Runs) != 3 {
		t.Fatal("new client lost mixed-mode history")
	}
	for _, run := range current.Execution.Runs {
		if run.LaunchCount == nil || len(run.DescendantCleanup) == 0 {
			t.Fatal("v6 dropped independent execution facts")
		}
		if run.TargetID == ids.target && run.ExecutionMode != protocol.CollaborationContainmentHostProcessSupervision {
			t.Fatal("host mode was projected as namespace isolation")
		}
	}
	var legacy CollaborationIssueResponse
	testutil.Call(t, testHandler.GetCollaborationIssue, collaborationFieldsReadRequest(ids.issue, testUserID, "")).Want(http.StatusOK).JSON(&legacy)
	if legacy.Execution.ProtocolVersion != 5 || len(legacy.Execution.Runs) != 0 || len(legacy.Execution.Groups) != 0 || len(legacy.Execution.Channels) != 0 || len(legacy.Execution.Commands) != 0 || len(legacy.Execution.Targets) != 0 {
		t.Fatal("legacy unavailable graph retained dangling/new-mode nodes")
	}
	if !legacy.Capabilities.Comment || legacy.Capabilities.ControlAI || legacy.Capabilities.SubmitCommand {
		t.Fatal("legacy mode compatibility changed human access or exposed AI controls")
	}
}
