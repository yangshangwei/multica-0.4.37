package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/multica-ai/multica/server/internal/service"
)

type collaborationCreateCommentBody struct {
	collaborationCommentInputBody
	ExpectedLifecycleEpoch   string  `json:"expected_lifecycle_epoch"`
	ExpectedParticipantEpoch *string `json:"expected_participant_epoch"`
	ExpectedAccountEpoch     *string `json:"expected_account_epoch"`
	TargetVersions           []struct {
		TargetID              string `json:"target_id"`
		ExpectedTargetEpoch   string `json:"expected_target_epoch"`
		ExpectedConfigVersion string `json:"expected_config_version"`
	} `json:"target_versions"`
}

func (h *Handler) CreateCollaborationComment(w http.ResponseWriter, r *http.Request) {
	prepareCollaborationContentRequest(w, r)
	var body collaborationCreateCommentBody
	if !decodeCollaborationWorkflowBody(w, r, &body) {
		return
	}
	viewVersion, validVersion := collaborationExecutionWriteVersion(w, body.ExecutionViewVersion)
	if !validVersion {
		return
	}
	issue, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "issue_id")
	if !ok {
		return
	}
	input, ok := collaborationCommentInput(w, body.collaborationCommentInputBody)
	if !ok {
		return
	}
	p := service.CollaborationCreateCommentParams{CollaborationCommentInput: input,
		CollaborationQueueRequest: service.CollaborationQueueRequest{IssueID: issue, RequestID: r.Header.Get("Idempotency-Key"), ExecutionViewVersion: viewVersion},
		TargetVersions:            make([]service.CollaborationCommentTargetVersion, 0, len(body.TargetVersions))}
	for _, version := range []struct {
		value *string
		out   *int64
	}{
		{&body.ExpectedLifecycleEpoch, &p.ExpectedLifecycleEpoch},
		{body.ExpectedParticipantEpoch, &p.ExpectedParticipantEpoch},
		{body.ExpectedAccountEpoch, &p.ExpectedAccountEpoch},
	} {
		if version.value == nil {
			continue
		}
		parsed, ok := collaborationPositiveVersion(*version.value)
		if !ok {
			writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_version", "error": "Invalid collaboration version"})
			return
		}
		*version.out = parsed
	}
	for _, target := range body.TargetVersions {
		id, ok := parseUUIDOrBadRequest(w, target.TargetID, "target_id")
		if !ok {
			return
		}
		epoch, validEpoch := collaborationPositiveVersion(target.ExpectedTargetEpoch)
		config, validConfig := collaborationPositiveVersion(target.ExpectedConfigVersion)
		if !validEpoch || !validConfig {
			writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_version", "error": "Invalid target version"})
			return
		}
		p.TargetVersions = append(p.TargetVersions, service.CollaborationCommentTargetVersion{TargetID: id, ExpectedTargetEpoch: epoch, ExpectedConfigVersion: config})
	}
	svc := service.NewCollaborationService(h.Queries, h.TxStarter, h.IssueService)
	receipt, err := svc.CreateComment(r.Context(), p)
	writeCollaborationCommentReceipt(w, receipt, err)
}

func (h *Handler) GetCollaborationCommentOperation(w http.ResponseWriter, r *http.Request) {
	prepareCollaborationContentRequest(w, r)
	issue, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "issue_id")
	if !ok {
		return
	}
	svc := service.NewCollaborationService(h.Queries, h.TxStarter, h.IssueService)
	receipt, err := svc.GetCommentOperation(r.Context(), issue, chi.URLParam(r, "requestId"))
	if err != nil {
		writeCollaborationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, receipt)
}
