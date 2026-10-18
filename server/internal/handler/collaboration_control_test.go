package handler

import (
	"encoding/json"
	"net/http"
	"reflect"
	"strconv"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
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

func TestCollaborationControlHTTPRejectsHoldAndScopeAmbiguity(t *testing.T) {
	bare := &Handler{}
	for _, scenario := range []struct {
		name  string
		patch func(map[string]any, map[string]any)
	}{
		{"missing_hold", func(_ map[string]any, target map[string]any) { delete(target, "expected_hold_version") }},
		{"null_hold", func(_ map[string]any, target map[string]any) { target["expected_hold_version"] = nil }},
		{"numeric_hold", func(_ map[string]any, target map[string]any) { target["expected_hold_version"] = 1 }},
		{"zero_hold", func(_ map[string]any, target map[string]any) { target["expected_hold_version"] = "0" }},
		{"overflow_hold", func(_ map[string]any, target map[string]any) { target["expected_hold_version"] = "9223372036854775808" }},
		{"stop_hold", func(body map[string]any, _ map[string]any) {
			body["action"] = "stop"
			delete(body, "expected_execution_epoch")
		}},
		{"stop_null_hold", func(body map[string]any, target map[string]any) {
			body["action"] = "stop"
			delete(body, "expected_execution_epoch")
			target["expected_hold_version"] = nil
		}},
		{"stop_null_gate", func(body map[string]any, target map[string]any) {
			body["action"] = "stop"
			body["expected_execution_epoch"] = nil
			delete(target, "expected_hold_version")
		}},
		{"target_issue_version", func(body map[string]any, _ map[string]any) { body["expected_control_version"] = nil }},
		{"missing_group_observation", func(_ map[string]any, target map[string]any) { delete(target, "observed_group_id") }},
		{"missing_targets", func(body map[string]any, _ map[string]any) { delete(body, "targets") }},
		{"unknown_target_field", func(_ map[string]any, target map[string]any) { target["hold_reason"] = "initial" }},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			target := map[string]any{"target_id": uuid.NewString(), "expected_target_epoch": "1", "expected_config_version": "1", "expected_control_version": "1",
				"expected_hold_version": "9007199254740993", "observed_group_id": nil, "observed_group_generation": "0"}
			body := map[string]any{"action": "resume", "scope": "target", "expected_lifecycle_epoch": "1", "expected_participant_epoch": "1", "expected_account_epoch": "1",
				"expected_execution_epoch": "1", "targets": []any{target}}
			scenario.patch(body, target)
			request := collaborationContentRequest(http.MethodPost, uuid.NewString(), "", testUserID, uuid.NewString(), body)
			testutil.Call(t, bare.ControlCollaborationIssue, request).Want(http.StatusBadRequest)
		})
	}
}

func TestCollaborationControlHTTPRecoversOriginalStructuredRefusal(t *testing.T) {
	ids := collaborationExecutionReadFixture(t)
	dbfx.InsertNoID(t, "collaboration_account_fence", testutil.Cols{"user_id": testUserID}, "user_id=$1", testUserID)
	targets, err := testHandler.Queries.ListCollaborationTargets(t.Context(), parseUUID(ids.session))
	if err != nil {
		t.Fatal(err)
	}
	var target db.CollaborationTarget
	for _, candidate := range targets {
		if uuidToString(candidate.ID) == ids.target {
			target = candidate
		}
	}
	if !target.ID.Valid {
		t.Fatal("fixture target is missing")
	}
	key := uuid.NewString()
	body := map[string]any{"action": "resume", "scope": "target", "expected_lifecycle_epoch": "1", "expected_participant_epoch": "1", "expected_account_epoch": "1",
		"expected_execution_epoch": "1", "targets": []map[string]any{{"target_id": ids.target, "expected_target_epoch": strconv.FormatInt(target.TargetEpoch, 10),
			"expected_config_version": strconv.FormatInt(target.ConfigVersion, 10), "expected_control_version": strconv.FormatInt(target.ControlVersion, 10),
			"expected_hold_version": strconv.FormatInt(target.HoldVersion, 10), "observed_group_id": uuidToString(target.ActiveGroupID), "observed_group_generation": strconv.FormatInt(target.GroupGeneration, 10)}}}
	var original service.CollaborationQueueReceipt
	response := testutil.Call(t, testHandler.ControlCollaborationIssue, collaborationContentRequest(http.MethodPost, ids.issue, "", testUserID, key, body)).Want(http.StatusConflict).JSON(&original)
	if original.Code != "issue_paused" || original.Scope != "target" || original.Action != "resume" || original.RequestID != key || uuidToString(original.IssueID) != ids.issue ||
		original.Channels == nil || original.TargetHolds == nil || len(original.Channels) != 0 || len(original.TargetHolds) != 0 {
		t.Fatalf("target refusal lost its original identity or claimed partial mutation: %s", response.Body.String())
	}
	dbfx.Exec(t, `UPDATE collaboration_session SET queue_state='active',control_version=control_version+1 WHERE id=$1`, ids.session)
	recoveryRequest := withURLParams(collaborationContentRequest(http.MethodGet, ids.issue, "", testUserID, key, nil), "id", ids.issue, "requestId", key)
	var recovered service.CollaborationQueueReceipt
	recovery := testutil.Call(t, testHandler.GetCollaborationControlOperation, recoveryRequest).Want(http.StatusOK).JSON(&recovered)
	if !reflect.DeepEqual(original, recovered) || recovery.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("recovery replaced the original refusal: %+v", recovered)
	}
	var replay service.CollaborationQueueReceipt
	testutil.Call(t, testHandler.ControlCollaborationIssue, collaborationContentRequest(http.MethodPost, ids.issue, "", testUserID, key, body)).Want(http.StatusConflict).JSON(&replay)
	if !reflect.DeepEqual(original, replay) {
		t.Fatal("original refused key became a new resume intention")
	}
	wrongIssue := uuid.NewString()
	testutil.Call(t, testHandler.GetCollaborationControlOperation, withURLParams(recoveryRequest, "id", wrongIssue, "requestId", key)).Want(http.StatusNotFound)
}
