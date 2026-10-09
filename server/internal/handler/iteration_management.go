package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/iteration"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// No new machine maintenance grant is implied by issue assignment. Existing
// human workspace members may maintain periods; settings retains its admin gate.
func (h *Handler) iterationManagementScope(r *http.Request) (pgtype.UUID, pgtype.UUID, *service.IterationService, func(context.Context, pgx.Tx) error, error) {
	ws, actor, err := iterationWorkspaceScope(r)
	if err != nil {
		return ws, actor, nil, nil, err
	}
	initialType, initialID := h.resolveActor(r, requestUserID(r), uuidToString(ws))
	if isMachineCredentialActor(r) || initialType != "member" || initialID != uuidToString(actor) {
		return ws, actor, nil, nil, iterationAPIError(403, "forbidden", "Iteration maintenance requires a human workspace member")
	}
	authorize := func(ctx context.Context, tx pgx.Tx) error {
		scoped := *h
		scoped.Queries, scoped.DB = h.Queries.WithTx(tx), tx
		kind, id := scoped.resolveActor(r, requestUserID(r), uuidToString(ws))
		if kind != initialType || id != initialID {
			return iterationAPIError(403, "forbidden", "Actor authority changed")
		}
		return nil
	}
	svc := &service.IterationService{TxStarter: h.TxStarter, AuthorizeIssues: func(_ context.Context, _ pgx.Tx, issues []db.Issue) error {
		// Current membership is held by the owner. Existing issue maintenance is
		// workspace-wide for human members; planning does not invoke an assignee.
		for _, issue := range issues {
			if issue.WorkspaceID != ws {
				return iterationAPIError(403, "forbidden", "Issue is outside this workspace")
			}
		}
		return nil
	}}
	return ws, actor, svc, authorize, nil
}
func decodeIterationBody(w http.ResponseWriter, r *http.Request, target any) bool {
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 4<<20))
	if err != nil {
		var large *http.MaxBytesError
		if errors.As(err, &large) {
			writeIterationAPIError(w, iterationAPIError(413, "iteration_operation_too_large", "Operation exceeds the request limit"))
		} else {
			writeIterationAPIError(w, iterationAPIError(400, "invalid_request", "Invalid iteration request"))
		}
		return false
	}
	if err = iteration.ValidateObjectJSON(raw); err != nil {
		writeIterationAPIError(w, iterationAPIError(400, "invalid_request", "Invalid or ambiguous iteration request"))
		return false
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(target); err != nil {
		writeIterationAPIError(w, iterationAPIError(400, "invalid_request", "Invalid iteration request"))
		return false
	}
	return true
}

func decodeIterationDraft(w http.ResponseWriter, raw json.RawMessage) (iteration.Draft, bool) {
	var draft iteration.Draft
	canonical, err := iteration.CanonicalDraftJSON(raw)
	if err != nil || json.Unmarshal(canonical, &draft) != nil {
		writeIterationAPIError(w, iterationAPIError(400, "invalid_request", "Invalid iteration draft"))
		return draft, false
	}
	return draft, true
}
func (h *Handler) writeIterationResult(w http.ResponseWriter, r *http.Request, result iteration.WriteResult, created bool) {
	if !result.Replayed {
		h.publish(protocol.EventIterationUpdated, result.WorkspaceID, "member", requestUserID(r), map[string]any{"iteration_ids": result.IterationIDs, "operation_id": result.OperationID, "request_id": result.RequestID})
	}
	status := 200
	if created && !result.Replayed {
		status = 201
	}
	writeJSON(w, status, result)
}
func (h *Handler) CreateIteration(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	ws, actor, svc, authorize, err := h.iterationManagementScope(r)
	if err != nil {
		writeIterationAPIError(w, err)
		return
	}
	var input service.CreateIterationInput
	if !decodeIterationBody(w, r, &input) {
		return
	}
	result, err := svc.Create(r.Context(), ws, actor, input, authorize)
	if err != nil {
		writeIterationAPIError(w, err)
		return
	}
	h.writeIterationResult(w, r, result, true)
}
func (h *Handler) UpdateIteration(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	ws, actor, svc, authorize, err := h.iterationManagementScope(r)
	if err != nil {
		writeIterationAPIError(w, err)
		return
	}
	id, err := util.ParseUUID(chi.URLParam(r, "iterationID"))
	if err != nil || id.Bytes == [16]byte{} {
		writeIterationAPIError(w, iterationAPIError(400, "invalid_request", "Invalid iteration ID"))
		return
	}
	var input service.EditIterationInput
	if !decodeIterationBody(w, r, &input) {
		return
	}
	result, err := svc.Edit(r.Context(), ws, actor, id, input, authorize)
	if err != nil {
		writeIterationAPIError(w, err)
		return
	}
	h.writeIterationResult(w, r, result, false)
}
func (h *Handler) PreviewIterationOperation(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	ws, actor, svc, authorize, err := h.iterationManagementScope(r)
	if err != nil {
		writeIterationAPIError(w, err)
		return
	}
	var raw json.RawMessage
	if !decodeIterationBody(w, r, &raw) {
		return
	}
	draft, ok := decodeIterationDraft(w, raw)
	if !ok {
		return
	}
	preview, err := svc.Preview(r.Context(), ws, actor, draft, authorize)
	if err != nil {
		writeIterationAPIError(w, err)
		return
	}
	writeJSON(w, 200, preview)
}
func (h *Handler) ApplyIterationOperation(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	ws, actor, svc, authorize, err := h.iterationManagementScope(r)
	if err != nil {
		writeIterationAPIError(w, err)
		return
	}
	var envelope struct {
		RequestID   string          `json:"request_id"`
		Draft       json.RawMessage `json:"draft"`
		PreviewHash string          `json:"preview_hash"`
	}
	if !decodeIterationBody(w, r, &envelope) {
		return
	}
	draft, ok := decodeIterationDraft(w, envelope.Draft)
	if !ok {
		return
	}
	result, err := svc.Apply(r.Context(), ws, actor, service.ApplyIterationInput{RequestID: envelope.RequestID, Draft: draft, PreviewHash: envelope.PreviewHash}, authorize)
	if err != nil {
		writeIterationAPIError(w, err)
		return
	}
	h.writeIterationResult(w, r, result, false)
}
