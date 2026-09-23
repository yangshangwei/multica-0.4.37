package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/multica-ai/multica/server/internal/service"
)

// TestLifecycleHandoffCommentReachesTaskQueue proves the runtime part of the
// lifecycle contract. The validator decides the route, while the existing
// comment trigger path owns persistence, permissions and task deduplication.
func TestLifecycleHandoffCommentReachesTaskQueue(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}

	if decision, err := service.ValidateRCARoute(service.RCARouteInput{
		Route: "maintenance", CauseState: "unknown", DownstreamRepairIssue: "DIAG-RUNTIME-1",
	}); err != nil || decision != service.LifecycleDecisionDiagnosis {
		t.Fatalf("maintenance handoff contract = %q, %v", decision, err)
	}

	diagnosticianID := createHandlerTestAgent(t, "Lifecycle Diagnostician", nil)
	issueID := createCommentTriggerPreviewIssue(t, "runtime lifecycle handoff", "", "")
	content := fmt.Sprintf(
		"lifecycle-handoff route=maintenance cause=unknown follow-up=DIAG-RUNTIME-1 [@Diagnostician](mention://agent/%s)",
		diagnosticianID,
	)

	w := httptest.NewRecorder()
	r := withURLParam(newRequest(http.MethodPost, "/api/issues/"+issueID+"/comments", map[string]any{
		"content": content,
	}), "id", issueID)
	testHandler.CreateComment(w, r)
	if w.Code != http.StatusCreated {
		t.Fatalf("CreateComment: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var response CommentResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("decode handoff comment: %v", err)
	}
	if response.ID == "" {
		t.Fatal("handoff comment has no id")
	}
	if outcome := findCommentOutcome(t, response.TriggerOutcomes, diagnosticianID); outcome.Status != DispatchQueued {
		t.Fatalf("diagnostician outcome = %+v, want queued", outcome)
	}

	var queued int
	var triggerCommentID string
	if err := testPool.QueryRow(context.Background(), `
		SELECT count(*), COALESCE(max(trigger_comment_id::text), '')
		FROM agent_task_queue
		WHERE issue_id = $1 AND agent_id = $2 AND status = 'queued'
	`, issueID, diagnosticianID).Scan(&queued, &triggerCommentID); err != nil {
		t.Fatalf("read lifecycle handoff task: %v", err)
	}
	if queued != 1 {
		t.Fatalf("lifecycle handoff queued tasks = %d, want 1", queued)
	}
	if triggerCommentID != response.ID {
		t.Fatalf("task trigger_comment_id = %q, want handoff comment %q", triggerCommentID, response.ID)
	}
}
