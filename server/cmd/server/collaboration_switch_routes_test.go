package main

import (
	"encoding/json"
	"net/http"
	"reflect"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestCollaborationAtomicSwitchRoutesPreserveHeldInputAndOriginalReceipt(t *testing.T) {
	f := newCollaborationRouteFixture(t)
	base := "/api/collaboration/issues/" + f.issue
	path := base + "/targets/" + f.target + "/switch-proposals"
	q := db.New(f.managed.pool)
	targets, err := q.ListCollaborationTargets(t.Context(), util.MustParseUUID(f.session))
	if err != nil || len(targets) != 1 {
		t.Fatalf("registered fixture target: %v", err)
	}
	before := targets[0]
	input := f.comment(t, true, "Input survives staged Switch")
	authority := func() map[string]any {
		return map[string]any{"expected_lifecycle_epoch": "1", "expected_participant_epoch": "1", "expected_account_epoch": "1"}
	}
	body := authority()
	body["expected_target_epoch"], body["expected_config_version"], body["roster"] = "1", "1", json.RawMessage(before.Roster)
	var proposed service.CollaborationQueueReceipt
	f.managed.request(t, f.ownerToken, false, http.MethodPost, path, body, nil).Want(http.StatusCreated).JSON(&proposed)
	if !proposed.ProposalID.Valid {
		t.Fatal("proposal omitted its immutable identity")
	}
	proposalPath := path + "/" + util.UUIDToString(proposed.ProposalID)
	commit := authority()
	commit["control"] = service.CollaborationTargetControl{TargetID: before.ID, ExpectedTargetEpoch: before.TargetEpoch,
		ExpectedConfigVersion: before.ConfigVersion, ExpectedHoldVersion: before.HoldVersion, ExpectedControlVersion: before.ControlVersion,
		ObservedGroupID: before.ActiveGroupID, ObservedGroupGeneration: before.GroupGeneration}
	key := uuid.NewString()
	var denied service.CollaborationQueueReceipt
	f.managed.request(t, f.ownerToken, false, http.MethodPost, proposalPath+"/commit", commit, map[string]string{"Idempotency-Key": key}).Want(http.StatusConflict).JSON(&denied)
	if denied.Code != "switch_consent_required" {
		t.Fatalf("live grants were accepted for a proposed configuration: %+v", denied)
	}
	consent := authority()
	consent["agent_id"] = f.agent
	consent["identity"] = map[string]any{"kind": "managed", "installation_id": f.installationID, "binding_id": f.binding.BindingID, "binding_epoch": f.binding.BindingEpoch}
	f.managed.request(t, f.contributorToken, false, http.MethodPost, proposalPath+"/consents", consent, nil).Want(http.StatusForbidden)
	f.managed.request(t, f.ownerToken, false, http.MethodPost, proposalPath+"/consents", consent, nil).Want(http.StatusCreated)
	var ready service.CollaborationSwitchProposalView
	f.managed.request(t, f.ownerToken, false, http.MethodGet, proposalPath, nil, nil).Want(http.StatusOK).JSON(&ready)
	if ready.Status != "consent_ready" || len(ready.Resources) != 1 || ready.Resources[0].GrantID != nil {
		t.Fatalf("staged consent became a current grant: %+v", ready)
	}
	var recovered service.CollaborationQueueReceipt
	f.managed.request(t, f.ownerToken, false, http.MethodGet, base+"/switch/operations/"+key, nil, nil).Want(http.StatusOK).JSON(&recovered)
	if !reflect.DeepEqual(denied, recovered) {
		t.Fatal("late consent changed the original refusal")
	}
	var switched service.CollaborationQueueReceipt
	f.managed.request(t, f.ownerToken, false, http.MethodPost, proposalPath+"/commit", commit, nil).Want(http.StatusOK).JSON(&switched)
	var detail struct {
		Execution service.CollaborationExecutionView `json:"execution"`
	}
	f.managed.request(t, f.ownerToken, false, http.MethodGet, base, nil, nil).Want(http.StatusOK).JSON(&detail)
	if switched.Code != "target_switched" || len(detail.Execution.Targets) != 1 || len(detail.Execution.Commands) != 1 || len(detail.Execution.Runs) != 0 {
		t.Fatalf("Switch changed input or began execution: %+v", detail.Execution)
	}
	after, command := detail.Execution.Targets[0], detail.Execution.Commands[0]
	if after.ID != f.target || after.TargetEpoch != "2" || after.ConfigVersion != "2" || after.HoldReason == nil || *after.HoldReason != "switch_hold" ||
		after.Control.QueueState != "paused" || command.ID != util.UUIDToString(input.Targets[0].CommandID) || command.State != "waiting" {
		t.Fatalf("Switch lost the target, explicit Resume hold or waiting input: target=%+v command=%+v", after, command)
	}
	var committed service.CollaborationSwitchProposalView
	f.managed.request(t, f.ownerToken, false, http.MethodGet, proposalPath, nil, nil).Want(http.StatusOK).JSON(&committed)
	if committed.Status != "committed" || committed.Resources[0].GrantID == nil {
		t.Fatal("owner could not identify the grant for explicit withdrawal")
	}
	// The old pre-consent mutation must never remain a public alternative.
	f.managed.request(t, f.ownerToken, false, http.MethodPost, base+"/targets/"+f.target+"/runtime-binding", body, nil).Want(http.StatusNotFound)
}
