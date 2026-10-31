package handler

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// Service tests own admission, sources and trimming. This checks that HTTP's
// actual escaping and trailing newline match the shared protocol byte count.
func TestCollaborationHandoffHTTPUsesDeclaredWireBudget(t *testing.T) {
	body := strings.Repeat("<", 64<<10)
	v := protocol.CollaborationHandoffEnvelope{ProtocolVersion: 1, IssueID: uuid.NewString(), RunID: uuid.NewString(), SnapshotID: uuid.NewString(), Mode: "snapshot", EventSequence: "1", Authority: protocol.CollaborationHandoffAuthority{Status: "todo"}, Assignment: protocol.CollaborationHandoffAssignment{Role: "member"}, Memory: protocol.CollaborationHandoffMemory{Version: "0", Availability: "absent"}, Inputs: protocol.CollaborationHandoffInputs{Items: []protocol.CollaborationHandoffInput{}}, Results: protocol.CollaborationHandoffResults{Items: []protocol.CollaborationHandoffResult{}}, WakeupCauses: protocol.CollaborationHandoffCauses{Items: []protocol.CollaborationHandoffCause{}, Completeness: "unknown"}, Sources: []protocol.CollaborationHandoffSource{{Kind: "delegation", ID: uuid.NewString(), RunID: uuid.NewString(), Version: "1", Content: body, SHA256: protocol.CollaborationHandoffHash(body), Availability: "available", Trust: "admitted_assignment"}}, Missing: []string{}}
	response := testutil.Call(t, func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, v)
	}, newRequest(http.MethodGet, "/handoff/snapshots", nil)).Want(http.StatusOK)
	material, wire, err := protocol.CollaborationHandoffSizes(v)
	if err != nil || material > protocol.CollaborationHandoffMaxBytes || wire != response.Body.Len() || wire <= protocol.CollaborationHandoffMaxBytes || wire > protocol.CollaborationHandoffMaxJSONBytes {
		t.Fatalf("HTTP/protocol byte counts differ: material=%d wire=%d HTTP=%d: %v", material, wire, response.Body.Len(), err)
	}
	var restored protocol.CollaborationHandoffEnvelope
	if err := json.Unmarshal(response.Body.Bytes(), &restored); err != nil || restored.Sources[0].Content != body || restored.Sources[0].SHA256 != v.Sources[0].SHA256 {
		t.Fatalf("HTTP changed the admitted assignment: %v", err)
	}
}
