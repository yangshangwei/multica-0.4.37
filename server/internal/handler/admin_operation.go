package handler

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func decodeTaskCancellationAck(r *http.Request) (TaskCancelAckRequest, error) {
	var req TaskCancelAckRequest
	invalid := &service.PlatformAdminError{Status: 400, Code: "invalid_cancel_ack", Message: "The cancellation receipt is invalid"}
	const limit = 1 << 20
	if r.Body == nil {
		return req, nil
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, limit+1))
	if err != nil || len(body) > limit {
		return req, invalid
	}
	var fields map[string]json.RawMessage
	objectErr := json.Unmarshal(body, &fields)
	managed := false
	for _, field := range []string{"operation_id", "binding_epoch", "execution_fence", "outcome"} {
		_, present := fields[field]
		// Malformed JSON cannot supply trusted evidence. Detect reserved field
		// names only to reject such a receipt, never to authorize its contents.
		if present || (objectErr != nil && bytes.Contains(body, []byte(`"`+field+`"`))) {
			managed = true
		}
	}
	if !managed {
		// Legacy payloads keep their original permissive decoder and can only
		// perform cleanup; they never establish management confirmation.
		_ = json.NewDecoder(bytes.NewReader(body)).Decode(&req)
		return req, nil
	}
	if objectErr != nil || json.Unmarshal(body, &req) != nil || req.OperationID == "" || req.BindingEpoch == "" || req.ExecutionFence == nil || req.Outcome == "" {
		return req, invalid
	}
	return req, nil
}

func (h *Handler) adminOperationService() *service.AdminOperationService {
	return service.NewAdminOperationService(h.Queries, h.TxStarter, h.TaskService)
}

type adminExecutionFenceRequest struct {
	RuntimeID     *string `json:"runtime_id"`
	DispatchedAt  *string `json:"dispatched_at"`
	TargetVersion string  `json:"target_version"`
}

func parseAdminExecutionFence(body adminExecutionFenceRequest) (service.AdminExecutionFence, error) {
	var fence service.AdminExecutionFence
	var err error
	if body.TargetVersion != "" {
		fence.StateVersion, err = strconv.ParseInt(body.TargetVersion, 10, 64)
		if err != nil || fence.StateVersion < 1 {
			return fence, &service.PlatformAdminError{Status: 400, Code: "invalid_execution_fence", Message: "The target version is invalid"}
		}
	}
	if body.RuntimeID != nil {
		fence.RuntimeID, err = util.ParseUUID(*body.RuntimeID)
		if err != nil {
			return fence, &service.PlatformAdminError{Status: 400, Code: "invalid_execution_fence", Message: "The runtime ID is invalid"}
		}
	}
	if body.DispatchedAt != nil {
		value, e := time.Parse(time.RFC3339Nano, *body.DispatchedAt)
		if e != nil {
			return fence, &service.PlatformAdminError{Status: 400, Code: "invalid_execution_fence", Message: "The dispatch timestamp is invalid"}
		}
		fence.DispatchedAt = pgtype.Timestamptz{Time: value, Valid: true}
	}
	if !fence.DispatchedAt.Valid && fence.StateVersion < 1 {
		return fence, &service.PlatformAdminError{Status: 400, Code: "invalid_execution_fence", Message: "A server-issued queued target version is required"}
	}
	if fence.DispatchedAt.Valid && !fence.RuntimeID.Valid {
		return fence, &service.PlatformAdminError{Status: 400, Code: "invalid_execution_fence", Message: "A runtime ID is required for a dispatched execution"}
	}
	return fence, nil
}
func adminTaskFenceResponse(runtime pgtype.UUID, dispatched pgtype.Timestamptz, version int64) map[string]any {
	var dispatchedAt *string
	if dispatched.Valid {
		value := dispatched.Time.UTC().Format(time.RFC3339Nano)
		dispatchedAt = &value
	}
	return map[string]any{"runtime_id": uuidToPtr(runtime), "dispatched_at": dispatchedAt, "target_version": strconv.FormatInt(version, 10)}
}
func adminCancellationTargetResponse(task db.AgentTaskQueue) map[string]any {
	return map[string]any{"id": uuidToString(task.ID), "status": task.Status, "state_version": strconv.FormatInt(task.StateVersion, 10), "execution_fence": adminTaskFenceResponse(task.RuntimeID, task.DispatchedAt, task.StateVersion)}
}
func (h *Handler) AdminCancelTask(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.requirePlatformAccess(w, r, true)
	if !ok {
		return
	}
	id, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "task id")
	if !ok {
		return
	}
	key, ok := parseUUIDOrBadRequest(w, r.Header.Get("Idempotency-Key"), "idempotency key")
	if !ok {
		return
	}
	var body struct {
		ExpectedExecutionFence adminExecutionFenceRequest `json:"expected_execution_fence"`
		Reason                 string                     `json:"reason"`
	}
	if !passwordDecode(w, r, &body) {
		return
	}
	fence, err := parseAdminExecutionFence(body.ExpectedExecutionFence)
	if err != nil {
		adminServiceError(w, r, err)
		return
	}
	result, err := h.adminOperationService().CancelTask(r.Context(), service.AdminCancelTaskParams{OrganizationID: actor.OrganizationID, TaskID: id, IdempotencyKey: key, Fence: fence, Reason: body.Reason, RequestID: adminRequestID(r)})
	if err != nil {
		adminServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"operation": adminOperationResponse(result.Operation), "target": adminCancellationTargetResponse(result.Task)})
}

func taskCancellationAck(req TaskCancelAckRequest) (service.TaskCancellationAck, error) {
	ack := service.TaskCancellationAck{BranchName: req.BranchName, DurableWorkDir: req.DurableWorkDir, ErrorMessage: req.ErrorMessage, FailureReason: req.FailureReason, Outcome: req.Outcome}
	invalid := &service.PlatformAdminError{Status: 400, Code: "invalid_cancel_ack", Message: "The cancellation receipt is invalid"}
	var err error
	if req.OperationID != "" {
		ack.OperationID, err = util.ParseUUID(req.OperationID)
		if err != nil {
			return ack, invalid
		}
	}
	if req.BindingEpoch != "" {
		ack.BindingEpoch, err = strconv.ParseInt(req.BindingEpoch, 10, 64)
		if err != nil || ack.BindingEpoch < 1 {
			return ack, invalid
		}
	}
	if req.ExecutionFence != nil {
		ack.Fence.RuntimeID, err = util.ParseUUID(req.ExecutionFence.RuntimeID)
		if err != nil {
			return ack, invalid
		}
		value, e := time.Parse(time.RFC3339Nano, req.ExecutionFence.DispatchedAt)
		if e != nil {
			return ack, invalid
		}
		ack.Fence.DispatchedAt = pgtype.Timestamptz{Time: value, Valid: true}
	}
	return ack, nil
}
