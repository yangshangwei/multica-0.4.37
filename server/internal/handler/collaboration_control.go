package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/service"
)

type collaborationControlBody struct {
	collaborationResourceRequestBody
	Action                   string                     `json:"action"`
	Scope                    collaboration.ControlScope `json:"scope"`
	ExpectedControlVersion   json.RawMessage            `json:"expected_control_version"`
	ExpectedTargetSetVersion json.RawMessage            `json:"expected_target_set_version"`
	ExpectedExecutionEpoch   json.RawMessage            `json:"expected_execution_epoch"`
	Targets                  []struct {
		TargetID                string          `json:"target_id"`
		ExpectedTargetEpoch     string          `json:"expected_target_epoch"`
		ExpectedConfigVersion   string          `json:"expected_config_version"`
		ExpectedControlVersion  string          `json:"expected_control_version"`
		ExpectedHoldVersion     json.RawMessage `json:"expected_hold_version"`
		ObservedGroupID         json.RawMessage `json:"observed_group_id"`
		ObservedGroupGeneration string          `json:"observed_group_generation"`
	} `json:"targets"`
}

func (h *Handler) ControlCollaborationIssue(w http.ResponseWriter, r *http.Request) {
	prepareCollaborationContentRequest(w, r)
	var body collaborationControlBody
	if !decodeCollaborationWorkflowBody(w, r, &body) {
		return
	}
	if (body.Action != "stop" && body.Action != "resume") || body.Targets == nil || len(body.Targets) > 256 ||
		(body.Scope != collaboration.ControlIssue && body.Scope != collaboration.ControlTarget) ||
		(body.Scope == collaboration.ControlTarget && len(body.Targets) != 1) ||
		(body.Action == "stop" && len(body.ExpectedExecutionEpoch) != 0) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_control_request", "error": "Invalid control request"})
		return
	}
	request, ok := collaborationResourceRequest(w, r, body.collaborationResourceRequestBody)
	if !ok {
		return
	}
	p := service.CollaborationStopQueueParams{CollaborationQueueRequest: request, Scope: body.Scope,
		Targets: make([]service.CollaborationTargetControl, 0, len(body.Targets))}
	for _, value := range []struct {
		input json.RawMessage
		out   *int64
	}{
		{body.ExpectedControlVersion, &p.ExpectedControlVersion}, {body.ExpectedTargetSetVersion, &p.ExpectedTargetSetVersion},
	} {
		if body.Scope == collaboration.ControlTarget && len(value.input) != 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_control_request", "error": "Target control does not accept issue versions"})
			return
		}
		if body.Scope == collaboration.ControlIssue {
			if len(value.input) == 0 {
				writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_version", "error": "Issue control version required"})
				return
			}
			version, valid := collaborationControlVersion(value.input)
			if !valid {
				writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_version", "error": "Invalid control version"})
				return
			}
			*value.out = version
		}
	}
	for _, observed := range body.Targets {
		id, valid := parseUUIDOrBadRequest(w, observed.TargetID, "target_id")
		if !valid {
			return
		}
		target := service.CollaborationTargetControl{TargetID: id}
		if body.Action == "stop" && len(observed.ExpectedHoldVersion) != 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_control_request", "error": "Stop does not accept hold versions"})
			return
		}
		if body.Action == "resume" {
			version, valid := collaborationControlVersion(observed.ExpectedHoldVersion)
			if !valid {
				writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_version", "error": "Target hold version required"})
				return
			}
			target.ExpectedHoldVersion = version
		}
		for _, value := range []struct {
			input string
			out   *int64
		}{
			{observed.ExpectedTargetEpoch, &target.ExpectedTargetEpoch},
			{observed.ExpectedConfigVersion, &target.ExpectedConfigVersion},
			{observed.ExpectedControlVersion, &target.ExpectedControlVersion},
		} {
			version, valid := collaborationPositiveVersion(value.input)
			if !valid {
				writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_version", "error": "Invalid target control version"})
				return
			}
			*value.out = version
		}
		if observed.ObservedGroupGeneration != "0" {
			generation, valid := collaborationPositiveVersion(observed.ObservedGroupGeneration)
			if !valid {
				writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_version", "error": "Invalid group generation"})
				return
			}
			target.ObservedGroupGeneration = generation
		}
		var observedGroupID *string
		if len(observed.ObservedGroupID) == 0 || json.Unmarshal(observed.ObservedGroupID, &observedGroupID) != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_control_request", "error": "Observed group identity required"})
			return
		}
		if observedGroupID != nil {
			group, valid := parseUUIDOrBadRequest(w, *observedGroupID, "observed_group_id")
			if !valid {
				return
			}
			target.ObservedGroupID = group
		}
		p.Targets = append(p.Targets, target)
	}
	svc := service.NewCollaborationService(h.Queries, h.TxStarter, h.IssueService)
	var receipt service.CollaborationQueueReceipt
	var err error
	if body.Action == "stop" {
		receipt, err = svc.StopQueue(r.Context(), p)
	} else {
		if len(body.ExpectedExecutionEpoch) == 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_version", "error": "Execution epoch required"})
			return
		}
		epoch, valid := collaborationControlVersion(body.ExpectedExecutionEpoch)
		if !valid {
			writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_version", "error": "Invalid execution epoch"})
			return
		}
		receipt, err = svc.ResumeQueue(r.Context(), service.CollaborationResumeQueueParams{CollaborationStopQueueParams: p, ExpectedExecutionEpoch: epoch})
	}
	writeCollaborationResourceResponse(w, receipt, err)
}

func collaborationControlVersion(raw json.RawMessage) (int64, bool) {
	var value string
	if json.Unmarshal(raw, &value) != nil {
		return 0, false
	}
	return collaborationPositiveVersion(value)
}

// Recovery returns the original logical status and decisions inside HTTP 200.
func (h *Handler) GetCollaborationControlOperation(w http.ResponseWriter, r *http.Request) {
	prepareCollaborationContentRequest(w, r)
	issue, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "issue_id")
	if !ok {
		return
	}
	svc := service.NewCollaborationService(h.Queries, h.TxStarter, h.IssueService)
	receipt, err := svc.GetControlOperation(r.Context(), issue, chi.URLParam(r, "requestId"))
	if err != nil {
		writeCollaborationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, receipt)
}
