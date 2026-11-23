package handler

import (
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/pkg/protocol"
	"net/http"
)

func (h *Handler) RecordCollaborationRuntimeCheckpoint(w http.ResponseWriter, r *http.Request) {
	token, ok := collaborationNarrowBearer(w, r, service.CollaborationBusinessTokenPrefix)
	if !ok {
		return
	}
	var p protocol.CollaborationRecordingCheckpoint
	if !decodeCollaborationRuntimeBodyLimit(w, r, &p, protocol.CollaborationCheckpointJSONLimit) {
		return
	}
	run, ok := collaborationRuntimeRunID(w, r, &p.Identity.RunID)
	if !ok {
		return
	}
	svc := service.NewCollaborationService(h.Queries, h.TxStarter, h.IssueService)
	out, err := svc.RecordRuntimeCheckpoint(r.Context(), run, token, p)
	if err != nil {
		writeCollaborationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) SealCollaborationRuntimeRecording(w http.ResponseWriter, r *http.Request) {
	token, ok := collaborationNarrowBearer(w, r, service.CollaborationTerminationTokenPrefix)
	if !ok {
		return
	}
	var p protocol.CollaborationRecordingSeal
	if !decodeCollaborationRuntimeBodyLimit(w, r, &p, 16<<10) {
		return
	}
	run, ok := collaborationRuntimeRunID(w, r, &p.Identity.RunID)
	if !ok {
		return
	}
	svc := service.NewCollaborationService(h.Queries, h.TxStarter, h.IssueService)
	out, err := svc.SealRuntimeRecording(r.Context(), run, token, p)
	if err != nil {
		writeCollaborationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}
