package handler

import (
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
)

func switchHTTPBody() map[string]any {
	return map[string]any{"expected_lifecycle_epoch": "1", "expected_participant_epoch": "1", "expected_account_epoch": "1"}
}

func TestCollaborationSwitchHTTPRejectsMalformedObservations(t *testing.T) {
	bare := &Handler{}
	for _, scenario := range []struct {
		name  string
		patch func(map[string]any, map[string]any)
	}{
		{"missing_control", func(body, _ map[string]any) { delete(body, "control") }},
		{"missing_hold", func(_, control map[string]any) { delete(control, "expected_hold_version") }},
		{"null_hold", func(_, control map[string]any) { control["expected_hold_version"] = nil }},
		{"numeric_hold", func(_, control map[string]any) { control["expected_hold_version"] = 1 }},
		{"padded_hold", func(_, control map[string]any) { control["expected_hold_version"] = "01" }},
		{"overflow_hold", func(_, control map[string]any) { control["expected_hold_version"] = "9223372036854775808" }},
		{"missing_group", func(_, control map[string]any) { delete(control, "observed_group_id") }},
		{"missing_generation", func(_, control map[string]any) { delete(control, "observed_group_generation") }},
		{"scientific_generation", func(_, control map[string]any) { control["observed_group_generation"] = "1e1" }},
		{"wrong_target", func(_, control map[string]any) { control["target_id"] = uuid.NewString() }},
		{"unknown_field", func(_, control map[string]any) { control["resume"] = true }},
		{"bad_authority", func(body, _ map[string]any) { body["expected_participant_epoch"] = "1.0" }},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			target := uuid.NewString()
			control := map[string]any{"target_id": target, "expected_target_epoch": "1", "expected_config_version": "1", "expected_control_version": "1",
				"expected_hold_version": "9007199254740993", "observed_group_id": nil, "observed_group_generation": "0"}
			body := switchHTTPBody()
			body["control"] = control
			scenario.patch(body, control)
			r := withURLParams(collaborationContentRequest(http.MethodPost, uuid.NewString(), "", testUserID, uuid.NewString(), body), "targetId", target, "proposalId", uuid.NewString())
			response := testutil.Call(t, bare.CommitCollaborationTargetSwitch, r).Want(http.StatusBadRequest)
			if response.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("Switch validation denial can be cached")
			}
		})
	}
	for _, handler := range []http.HandlerFunc{bare.ProposeCollaborationTargetSwitch, bare.ConsentCollaborationTargetSwitch, bare.CommitCollaborationTargetSwitch, bare.RevokeCollaborationTargetSwitchConsent} {
		for _, body := range []json.RawMessage{json.RawMessage(`{"expected_account_epoch":"1","expected_account_epoch":"2"}`), json.RawMessage(`{} {}`)} {
			r := collaborationContentRequest(http.MethodPost, uuid.NewString(), "", testUserID, uuid.NewString(), body)
			testutil.Call(t, handler, r).Want(http.StatusBadRequest)
		}
	}
}

func TestCollaborationSwitchHTTPMachineCannotBorrowOwnerConsent(t *testing.T) {
	bare := &Handler{}
	body := switchHTTPBody()
	body["agent_id"] = uuid.NewString()
	body["identity"] = map[string]any{"kind": "enrolled", "installation_id": uuid.NewString(), "binding_id": uuid.NewString(), "binding_epoch": "1"}
	r := withURLParams(collaborationContentRequest(http.MethodPost, uuid.NewString(), "", testUserID, uuid.NewString(), body), "targetId", uuid.NewString(), "proposalId", uuid.NewString())
	r = r.WithContext(auth.WithAuthenticatedPrincipal(r.Context(), auth.AuthenticatedPrincipal{UserID: testUserID, Kind: auth.CredentialTask}))
	testutil.Call(t, bare.ConsentCollaborationTargetSwitch, r).Want(http.StatusForbidden)
}

func TestCollaborationSwitchHTTPStagedConsentCommitAndRecovery(t *testing.T) {
	f := newConfigHTTPFixture(t)
	t.Setenv("MULTICA_DEPLOYMENT_ID", "68150177-594d-45d8-8a84-bc979b498b04")
	target := f.register(t, collaboration.TargetAgent, f.agent)
	dbfx.Cleanup(t, `DELETE FROM collaboration_switch_proposal WHERE issue_id=$1`, f.issue)
	dbfx.Cleanup(t, `DELETE FROM collaboration_switch_consent WHERE proposal_id IN (SELECT id FROM collaboration_switch_proposal WHERE issue_id=$1)`, f.issue)
	dbfx.Cleanup(t, `DELETE FROM collaboration_grant WHERE issue_id=$1`, f.issue)
	call := func(handler http.HandlerFunc, method, key, proposal string, body any, status int, out any) *testutil.Response {
		t.Helper()
		r := withURLParams(collaborationContentRequest(method, f.issue, "", testUserID, key, body), "targetId", uuidToString(target.ID), "proposalId", proposal, "requestId", key)
		r.Header.Set("X-Workspace-ID", "unrelated-workspace")
		result := testutil.Call(t, handler, r).Want(status)
		result.JSON(out)
		return result
	}
	body := switchHTTPBody()
	body["expected_target_epoch"], body["expected_config_version"] = "1", "1"
	body["roster"] = json.RawMessage(target.Roster)
	var proposed service.CollaborationQueueReceipt
	key := uuid.NewString()
	call(testHandler.ProposeCollaborationTargetSwitch, http.MethodPost, key, "", body, http.StatusCreated, &proposed)
	if !proposed.ProposalID.Valid || !reflect.DeepEqual(target, f.target(t, target.ID)) {
		t.Fatal("HTTP proposal changed live configuration")
	}
	installation := uuid.NewString()
	binding := dbfx.Insert(t, "collaboration_runtime_enrollment", testutil.Cols{"deployment_id": auth.ManagedDeploymentID(), "owner_user_id": testUserID,
		"execution_workspace_id": testWorkspaceID, "runtime_id": f.runtime, "installation_id": installation, "public_key": make([]byte, 32)})
	consent := switchHTTPBody()
	consent["agent_id"] = f.agent
	consent["identity"] = map[string]any{"kind": "enrolled", "installation_id": installation, "binding_id": binding, "binding_epoch": "1"}
	var consented service.CollaborationQueueReceipt
	call(testHandler.ConsentCollaborationTargetSwitch, http.MethodPost, uuid.NewString(), uuidToString(proposed.ProposalID), consent, http.StatusCreated, &consented)
	var view service.CollaborationSwitchProposalView
	response := call(testHandler.GetCollaborationTargetSwitchProposal, http.MethodGet, "", uuidToString(proposed.ProposalID), nil, http.StatusOK, &view)
	if view.Status != "consent_ready" || strings.Contains(response.Body.String(), "fingerprint") || strings.Contains(response.Body.String(), installation) ||
		strings.Contains(response.Body.String(), binding) || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("proposal view leaked private registration or lost readiness: %s", response.Body.String())
	}
	commit := switchHTTPBody()
	commit["control"] = map[string]any{"target_id": uuidToString(target.ID), "expected_target_epoch": "1", "expected_config_version": "1",
		"expected_control_version": "1", "expected_hold_version": "1", "observed_group_id": nil, "observed_group_generation": "0"}
	commitKey := uuid.NewString()
	var switched service.CollaborationQueueReceipt
	call(testHandler.CommitCollaborationTargetSwitch, http.MethodPost, commitKey, uuidToString(proposed.ProposalID), commit, http.StatusOK, &switched)
	if switched.Code != "target_switched" || f.target(t, target.ID).HoldReason.String != "switch_hold" {
		t.Fatal("HTTP Switch did not preserve the explicit Resume hold")
	}
	var recovered service.CollaborationQueueReceipt
	call(testHandler.GetCollaborationSwitchOperation, http.MethodGet, commitKey, "", nil, http.StatusOK, &recovered)
	if !reflect.DeepEqual(switched, recovered) {
		t.Fatal("GET recovery changed the historical Switch receipt")
	}
}
