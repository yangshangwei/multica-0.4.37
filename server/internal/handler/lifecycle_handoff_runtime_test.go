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

func postLifecycleHandoffForTest(t *testing.T, issueID string, body map[string]any) (int, lifecycleHandoffResponse) {
	t.Helper()
	w := httptest.NewRecorder()
	r := withURLParam(newRequest(http.MethodPost, "/api/issues/"+issueID+"/lifecycle-handoffs", body), "id", issueID)
	testHandler.CreateLifecycleHandoff(w, r)
	var response lifecycleHandoffResponse
	if w.Code == http.StatusCreated {
		if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
			t.Fatalf("decode lifecycle handoff response: %v", err)
		}
	}
	return w.Code, response
}

func TestCreateLifecycleHandoffMaintenanceCreatesDiagnosticFollowUp(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	diagnosticianID := createHandlerTestAgent(t, "Runtime Maintenance Diagnostician", nil)
	sourceID := createCommentTriggerPreviewIssue(t, "runtime maintenance source", "", "")
	title := "diagnose runtime maintenance unknown cause"

	code, response := postLifecycleHandoffForTest(t, sourceID, map[string]any{
		"kind": "rca", "route": "maintenance", "cause_state": "unknown",
		"follow_up_title": title, "assignee_type": "agent", "assignee_id": diagnosticianID,
		"handoff_note": "reproduce before proposing a fix",
	})
	if code != http.StatusCreated {
		t.Fatalf("lifecycle handoff: expected 201, got %d", code)
	}
	if response.Decision != string(service.LifecycleDecisionDiagnosis) || !response.FollowUpCreated {
		t.Fatalf("response = %+v, want diagnosis + created follow-up", response)
	}
	if response.FollowUpIssueID == "" || response.QueuedTaskID == "" {
		t.Fatalf("response = %+v, want follow-up and queued task", response)
	}

	var parentID, taskStatus string
	if err := testPool.QueryRow(context.Background(), `
		SELECT i.parent_issue_id::text, atq.status
		FROM issue i
		JOIN agent_task_queue atq ON atq.issue_id = i.id AND atq.agent_id = $2
		WHERE i.id = $1
	`, response.FollowUpIssueID, diagnosticianID).Scan(&parentID, &taskStatus); err != nil {
		t.Fatalf("read diagnostic follow-up: %v", err)
	}
	if parentID != sourceID || taskStatus != "queued" {
		t.Fatalf("follow-up parent/status = %q/%q, want %q/queued", parentID, taskStatus, sourceID)
	}

	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM agent_task_queue WHERE issue_id = $1`, response.FollowUpIssueID)
		testPool.Exec(context.Background(), `DELETE FROM comment WHERE issue_id = $1`, response.FollowUpIssueID)
		testPool.Exec(context.Background(), `DELETE FROM issue WHERE id = $1`, response.FollowUpIssueID)
	})
}

func TestCreateLifecycleHandoffBugFixKnownAndUnknownRoutes(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	sourceID := createCommentTriggerPreviewIssue(t, "runtime bug fix source", "", "")
	knownCode, known := postLifecycleHandoffForTest(t, sourceID, map[string]any{
		"kind": "rca", "route": "bug-fix", "cause_state": "known",
		"reason": "regression is isolated to the parser branch", "follow_up_title": "fix parser branch",
	})
	if knownCode != http.StatusCreated || known.Decision != string(service.LifecycleDecisionDirectRepair) {
		t.Fatalf("known bug-fix response: status=%d response=%+v", knownCode, known)
	}
	unknownCode, unknown := postLifecycleHandoffForTest(t, sourceID, map[string]any{
		"kind": "rca", "route": "bug-fix", "cause_state": "unknown",
		"follow_up_title": "diagnose parser regression",
	})
	if unknownCode != http.StatusCreated || unknown.Decision != string(service.LifecycleDecisionDiagnosis) {
		t.Fatalf("unknown bug-fix response: status=%d response=%+v", unknownCode, unknown)
	}
	t.Cleanup(func() {
		for _, issueID := range []string{known.FollowUpIssueID, unknown.FollowUpIssueID} {
			testPool.Exec(context.Background(), `DELETE FROM issue WHERE id = $1`, issueID)
		}
	})
}

func TestCreateLifecycleHandoffPersistsRCARepairEvidence(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	sourceID := createCommentTriggerPreviewIssue(t, "runtime RCA evidence source", "", "")
	diagnosisRef := sourceID + "#comment-rca-runtime"
	regressionTest := "TestRetryPolicyDoesNotDropAcknowledgements"
	code, response := postLifecycleHandoffForTest(t, sourceID, map[string]any{
		"kind": "rca", "route": "bug-fix", "cause_state": "known",
		"reason":        "the redacted retry branch and regression test identify the cause",
		"conclusion":    "confirmed",
		"evidence":      []string{"retry branch stops acknowledging after the policy change"},
		"unknowns":      []string{"dependency saturation interval"},
		"diagnosis_ref": diagnosisRef, "regression_test": regressionTest,
		"follow_up_title": "repair retry acknowledgement regression",
	})
	if code != http.StatusCreated || response.Decision != string(service.LifecycleDecisionDirectRepair) || response.FollowUpIssueID == "" {
		t.Fatalf("RCA evidence handoff: status=%d response=%+v", code, response)
	}
	var diagnosis, regression, conclusion string
	var evidence, unknowns []string
	if err := testPool.QueryRow(context.Background(), `
		SELECT metadata->>'lifecycle_diagnosis_ref',
		       metadata->>'lifecycle_regression_test',
		       metadata->>'lifecycle_rca_conclusion',
		       ARRAY(SELECT jsonb_array_elements_text(metadata->'lifecycle_rca_evidence')),
		       ARRAY(SELECT jsonb_array_elements_text(metadata->'lifecycle_rca_unknowns'))
		FROM issue WHERE id = $1
	`, response.FollowUpIssueID).Scan(&diagnosis, &regression, &conclusion, &evidence, &unknowns); err != nil {
		t.Fatalf("read RCA repair evidence: %v", err)
	}
	if diagnosis != diagnosisRef || regression != regressionTest || conclusion != "confirmed" || len(evidence) != 1 || len(unknowns) != 1 {
		t.Fatalf("repair evidence = ref=%q regression=%q conclusion=%q evidence=%v unknowns=%v", diagnosis, regression, conclusion, evidence, unknowns)
	}
	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM comment WHERE issue_id = $1`, response.FollowUpIssueID)
		testPool.Exec(context.Background(), `DELETE FROM issue WHERE id = $1`, response.FollowUpIssueID)
	})
}

func TestCreateLifecycleHandoffIncidentRequiresMitigation(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	sourceID := createCommentTriggerPreviewIssue(t, "runtime incident source", "", "")
	code, _ := postLifecycleHandoffForTest(t, sourceID, map[string]any{
		"kind": "rca", "route": "incident", "cause_state": "unknown",
		"follow_up_title": "post recovery RCA", "separate_follow_up": true,
	})
	if code != http.StatusBadRequest {
		t.Fatalf("incident without mitigation: status = %d, want 400", code)
	}
}

func TestCreateLifecycleHandoffIncidentLearningIsIdempotent(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	sourceID := createCommentTriggerPreviewIssue(t, "runtime incident learning source", "", "")
	body := map[string]any{
		"kind": "incident-learning", "facts": []string{"queue exceeded budget"},
		"inferences": []string{"retry storm amplified the queue"}, "unknowns": []string{"customer impact window"},
		"follow_up_title": "add queue retry guard", "prevention": map[string]any{
			"title": "add queue retry guard", "owner": "reliability", "priority": "high",
			"acceptance_signal": "queue depth stays below baseline", "related_incident": sourceID,
		},
	}
	code, first := postLifecycleHandoffForTest(t, sourceID, body)
	if code != http.StatusCreated || first.FollowUpIssueID == "" {
		t.Fatalf("first incident-learning handoff: status=%d response=%+v", code, first)
	}
	code, second := postLifecycleHandoffForTest(t, sourceID, body)
	if code != http.StatusCreated || second.FollowUpIssueID != first.FollowUpIssueID || !second.FollowUpReused {
		t.Fatalf("second incident-learning handoff: status=%d response=%+v, first=%+v", code, second, first)
	}

	var owner, signal string
	if err := testPool.QueryRow(context.Background(), `
		SELECT metadata->>'lifecycle_owner', metadata->>'lifecycle_acceptance_signal' FROM issue WHERE id = $1
	`, first.FollowUpIssueID).Scan(&owner, &signal); err != nil {
		t.Fatalf("read prevention metadata: %v", err)
	}
	if owner != "reliability" || signal == "" {
		t.Fatalf("prevention metadata = %q/%q", owner, signal)
	}
	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM comment WHERE issue_id = $1`, first.FollowUpIssueID)
		testPool.Exec(context.Background(), `DELETE FROM issue WHERE id = $1`, first.FollowUpIssueID)
	})
}

func TestCreateLifecycleHandoffIncidentLearningLinksExistingPreventionTask(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	sourceID := createCommentTriggerPreviewIssue(t, "runtime existing prevention source", "", "")
	preventionID := createCommentTriggerPreviewIssue(t, "existing prevention task", "", "")
	code, response := postLifecycleHandoffForTest(t, sourceID, map[string]any{
		"kind": "incident-learning", "facts": []string{"alert fired"},
		"inferences": []string{"retry amplified impact"}, "unknowns": []string{"impact window"},
		"existing_prevention_tasks": []string{preventionID},
	})
	if code != http.StatusCreated || response.Decision != string(service.LifecycleDecisionContinue) || response.FollowUpIssueID != "" {
		t.Fatalf("existing prevention handoff: status=%d response=%+v", code, response)
	}
	handoff, ok := response.Metadata["lifecycle_handoff"].(map[string]any)
	if !ok {
		t.Fatalf("response lifecycle handoff = %#v", response.Metadata["lifecycle_handoff"])
	}
	evidence, ok := handoff["evidence"].(map[string]any)
	if !ok {
		t.Fatalf("response evidence = %#v", handoff["evidence"])
	}
	linked, ok := evidence["existing_prevention_tasks"].([]any)
	if !ok || len(linked) != 1 {
		t.Fatalf("existing prevention evidence = %#v", evidence["existing_prevention_tasks"])
	}
	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM issue WHERE id = $1`, preventionID)
	})
}

func TestCreateLifecycleHandoffRolloutMissingEvidenceIsUnknown(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	sourceID := createCommentTriggerPreviewIssue(t, "runtime rollout source", "", "")
	code, response := postLifecycleHandoffForTest(t, sourceID, map[string]any{
		"kind": "rollout", "rollout": map[string]any{
			"approved_digest": "sha256:a", "artifact_digest": "sha256:b",
			"baseline": map[string]float64{"error_rate": 0}, "observation_window": map[string]any{
				"started_at": "2026-09-24T10:00:00Z", "ended_at": "2026-09-24T10:15:00Z", "complete": true,
			},
			"signals": []map[string]any{{"name": "error_rate", "value": 0, "threshold": 1}},
		},
	})
	if code != http.StatusCreated || response.Decision != string(service.LifecycleDecisionUnknown) {
		t.Fatalf("rollout response: status=%d response=%+v", code, response)
	}
	var decision string
	if err := testPool.QueryRow(context.Background(), `SELECT metadata->'lifecycle_handoff'->>'decision' FROM issue WHERE id = $1`, sourceID).Scan(&decision); err != nil {
		t.Fatalf("read rollout decision: %v", err)
	}
	if decision != string(service.LifecycleDecisionUnknown) {
		t.Fatalf("persisted rollout decision = %q, want unknown", decision)
	}
}

func TestCreateLifecycleHandoffGovernanceFailsClosed(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	sourceID := createCommentTriggerPreviewIssue(t, "runtime governance source", "", "")
	complete := map[string]any{
		"kind": "governance", "governance": map[string]any{
			"capability": "contract-compatibility", "signals": []map[string]any{
				{"name": "old-client-matrix", "status": "pass", "detail": "desktop and plugin matrix checked"},
				{"name": "plugin-boundary", "status": "pass"},
				{"name": "parse-with-fallback", "status": "pass"},
			},
		},
	}
	code, response := postLifecycleHandoffForTest(t, sourceID, complete)
	if code != http.StatusCreated || response.Decision != string(service.LifecycleDecisionPass) {
		t.Fatalf("complete governance handoff: status=%d response=%+v", code, response)
	}
	missing := map[string]any{
		"kind": "governance", "governance": map[string]any{
			"capability": "contract-compatibility", "signals": []map[string]any{
				{"name": "old-client-matrix", "status": "pass"},
				{"name": "plugin-boundary", "status": "unknown"},
			},
		},
	}
	code, response = postLifecycleHandoffForTest(t, sourceID, missing)
	if code != http.StatusCreated || response.Decision != string(service.LifecycleDecisionUnknown) {
		t.Fatalf("incomplete governance handoff: status=%d response=%+v", code, response)
	}
	failed := map[string]any{
		"kind": "governance", "governance": map[string]any{
			"capability": "contract-compatibility", "signals": []map[string]any{
				{"name": "old-client-matrix", "status": "fail"},
				{"name": "plugin-boundary", "status": "pass"},
				{"name": "parse-with-fallback", "status": "pass"},
			},
		},
	}
	code, response = postLifecycleHandoffForTest(t, sourceID, failed)
	if code != http.StatusCreated || response.Decision != string(service.LifecycleDecisionHold) {
		t.Fatalf("failed governance handoff: status=%d response=%+v", code, response)
	}
}

func TestCreateLifecycleHandoffPreservesBoundedHistoryAcrossStages(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	sourceID := createCommentTriggerPreviewIssue(t, "runtime cross-stage source", "", "")
	code, rca := postLifecycleHandoffForTest(t, sourceID, map[string]any{
		"kind": "rca", "route": "bug-fix", "cause_state": "known",
		"reason": "regression is isolated", "follow_up_title": "repair cross-stage source",
	})
	if code != http.StatusCreated || rca.Decision != string(service.LifecycleDecisionDirectRepair) {
		t.Fatalf("RCA handoff: status=%d response=%+v", code, rca)
	}
	code, rollout := postLifecycleHandoffForTest(t, sourceID, map[string]any{
		"kind": "rollout", "rollout": map[string]any{
			"approved_digest": "sha256:stage-1", "artifact_digest": "sha256:stage-1",
			"baseline": map[string]float64{"error_rate": 0.1}, "observation_window": map[string]any{
				"started_at": "2026-09-24T11:00:00Z", "ended_at": "2026-09-24T11:15:00Z", "complete": true,
			},
			"signals": []map[string]any{{"name": "error_rate", "value": 0.1, "threshold": 1}},
		},
	})
	if code != http.StatusCreated || rollout.Decision != string(service.LifecycleDecisionContinue) {
		t.Fatalf("rollout handoff: status=%d response=%+v", code, rollout)
	}
	var latest, historyLength int
	if err := testPool.QueryRow(context.Background(), `
		SELECT jsonb_array_length(metadata->'lifecycle_handoff_history'),
		       CASE WHEN metadata->'lifecycle_handoff'->>'kind' = 'rollout' THEN 1 ELSE 0 END
		FROM issue WHERE id = $1
	`, sourceID).Scan(&historyLength, &latest); err != nil {
		t.Fatalf("read lifecycle history: %v", err)
	}
	if historyLength != 2 || latest != 1 {
		t.Fatalf("lifecycle history/latest = %d/%d, want 2/1", historyLength, latest)
	}
	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM issue WHERE id = $1`, rca.FollowUpIssueID)
	})
}

func TestCreateLifecycleHandoffAgentEvaluationQueuesEvaluatorFollowUp(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	evaluatorID := createHandlerTestAgent(t, "Runtime Agent Evaluator", nil)
	sourceID := createCommentTriggerPreviewIssue(t, "runtime agent change source", "", "")
	cases := make([]map[string]any, 0, 6)
	for _, category := range []string{"correctness", "tool-failure", "safety", "cost", "latency", "drift"} {
		result := "pass"
		if category == "safety" {
			result = "blocked"
		}
		cases = append(cases, map[string]any{
			"category": category, "trace": []string{"input", "tool", "decision"},
			"stop_reason": "case-complete", "result": result,
		})
	}
	code, response := postLifecycleHandoffForTest(t, sourceID, map[string]any{
		"kind": "agent-evaluation", "follow_up_title": "evaluate candidate agent release",
		"assignee_type": "agent", "assignee_id": evaluatorID,
		"agent_evaluation": map[string]any{
			"artifact_digest": "sha256:eval-2", "baseline_version": "agent-1", "candidate_version": "agent-2",
			"skill_version": "skill-4", "mcp_version": "mcp-3", "cases": cases,
		},
	})
	if code != http.StatusCreated || response.Decision != string(service.LifecycleDecisionPass) || response.QueuedTaskID == "" {
		t.Fatalf("agent evaluation response: status=%d response=%+v", code, response)
	}
	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM agent_task_queue WHERE issue_id = $1`, response.FollowUpIssueID)
		testPool.Exec(context.Background(), `DELETE FROM issue WHERE id = $1`, response.FollowUpIssueID)
	})
}

func TestCreateLifecycleHandoffRejectsUnrelatedFollowUp(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	sourceID := createCommentTriggerPreviewIssue(t, "runtime unrelated source", "", "")
	otherID := createCommentTriggerPreviewIssue(t, "runtime unrelated target", "", "")
	code, _ := postLifecycleHandoffForTest(t, sourceID, map[string]any{
		"kind": "rca", "route": "bug-fix", "cause_state": "unknown",
		"follow_up_issue_id": otherID,
	})
	if code != http.StatusBadRequest {
		t.Fatalf("unrelated follow-up status=%d, want 400", code)
	}
	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM issue WHERE id = $1`, otherID)
	})
}
