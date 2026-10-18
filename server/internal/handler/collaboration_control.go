package handler

import (
	"net/http"

	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/service"
)

type collaborationControlBody struct {
	collaborationResourceRequestBody
	Action                   string                     `json:"action"`
	Scope                    collaboration.ControlScope `json:"scope"`
	ExpectedControlVersion   *string                    `json:"expected_control_version"`
	ExpectedTargetSetVersion *string                    `json:"expected_target_set_version"`
	ExpectedExecutionEpoch   *string                    `json:"expected_execution_epoch"`
	Targets                  []struct {
		TargetID                string  `json:"target_id"`
		ExpectedTargetEpoch     string  `json:"expected_target_epoch"`
		ExpectedConfigVersion   string  `json:"expected_config_version"`
		ExpectedControlVersion  string  `json:"expected_control_version"`
		ObservedGroupID         *string `json:"observed_group_id"`
		ObservedGroupGeneration string  `json:"observed_group_generation"`
	} `json:"targets"`
}

func (h *Handler) ControlCollaborationIssue(w http.ResponseWriter, r *http.Request) {
	prepareCollaborationContentRequest(w, r)
	var body collaborationControlBody
	if !decodeCollaborationWorkflowBody(w, r, &body) {
		return
	}
	if (body.Action != "stop" && body.Action != "resume") || len(body.Targets) > 256 ||
		(body.Scope != collaboration.ControlIssue && body.Scope != collaboration.ControlTarget) ||
		(body.Action == "stop" && body.ExpectedExecutionEpoch != nil) {
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
		input *string
		out   *int64
	}{
		{body.ExpectedControlVersion, &p.ExpectedControlVersion}, {body.ExpectedTargetSetVersion, &p.ExpectedTargetSetVersion},
	} {
		if body.Scope == collaboration.ControlTarget && value.input != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_control_request", "error": "Target control does not accept issue versions"})
			return
		}
		if body.Scope == collaboration.ControlIssue {
			if value.input == nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_version", "error": "Issue control version required"})
				return
			}
			version, valid := collaborationPositiveVersion(*value.input)
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
		if observed.ObservedGroupID != nil {
			group, valid := parseUUIDOrBadRequest(w, *observed.ObservedGroupID, "observed_group_id")
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
		if body.ExpectedExecutionEpoch == nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_version", "error": "Execution epoch required"})
			return
		}
		epoch, valid := collaborationPositiveVersion(*body.ExpectedExecutionEpoch)
		if !valid {
			writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_version", "error": "Invalid execution epoch"})
			return
		}
		receipt, err = svc.ResumeQueue(r.Context(), service.CollaborationResumeQueueParams{CollaborationStopQueueParams: p, ExpectedExecutionEpoch: epoch})
	}
	writeCollaborationResourceResponse(w, receipt, err)
}
