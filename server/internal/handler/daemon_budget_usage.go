package handler

import (
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/multica-ai/multica/server/internal/service"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// ReportBudgetAttemptUsage accepts one immutable v1 batch for a retained
// attempt. The route belongs inside the DaemonAuth group, never task auth.
func (h *Handler) ReportBudgetAttemptUsage(w http.ResponseWriter, r *http.Request) {
	h.reportBudgetAttemptUsage(w, r, time.Now(), service.NewIssueBudgetUsageService(h.Queries, h.TxStarter))
}

func (h *Handler) reportBudgetAttemptUsage(w http.ResponseWriter, r *http.Request, receivedAt time.Time, usage *service.IssueBudgetUsageService) {
	attempt, ok := h.requireDaemonBudgetAttempt(w, r)
	if !ok {
		return
	}
	raw, err := io.ReadAll(io.LimitReader(r.Body, service.IssueBudgetUsageMaxBytes+1))
	if err != nil {
		writeErrorCode(w, http.StatusBadRequest, "invalid_request", "failed to read usage batch")
		return
	}
	if len(raw) > service.IssueBudgetUsageMaxBytes {
		writeErrorCode(w, http.StatusRequestEntityTooLarge, "request_too_large", "usage batch exceeds 256 KiB")
		return
	}
	result, err := usage.Report(r.Context(), attempt, raw, receivedAt)
	if err != nil {
		writeDaemonBudgetUsageError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) GetBudgetAttemptUsage(w http.ResponseWriter, r *http.Request) {
	attempt, ok := h.requireDaemonBudgetAttempt(w, r)
	if !ok {
		return
	}
	usage := service.NewIssueBudgetUsageService(h.Queries, h.TxStarter)
	result, err := usage.Query(r.Context(), attempt, chi.URLParam(r, "batchKey"))
	if err != nil {
		writeDaemonBudgetUsageError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) requireDaemonBudgetAttempt(w http.ResponseWriter, r *http.Request) (db.IssueBudgetAttempt, bool) {
	id, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "attemptId"), "attempt_id")
	if !ok {
		return db.IssueBudgetAttempt{}, false
	}
	attempt, err := h.Queries.GetIssueBudgetUsageAttempt(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "not found")
		return db.IssueBudgetAttempt{}, false
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load budget attempt")
		return db.IssueBudgetAttempt{}, false
	}
	if !h.requireDaemonWorkspaceAccess(w, r, uuidToString(attempt.WorkspaceID)) {
		return db.IssueBudgetAttempt{}, false
	}
	if _, ok := h.requireDaemonRuntimeAccess(w, r, uuidToString(attempt.RuntimeID)); !ok {
		return db.IssueBudgetAttempt{}, false
	}
	return attempt, true
}

func writeDaemonBudgetUsageError(w http.ResponseWriter, err error) {
	if errors.Is(err, service.ErrIssueBudgetUsageTooLarge) {
		writeErrorCode(w, http.StatusRequestEntityTooLarge, "request_too_large", "usage batch exceeds 256 KiB")
		return
	}
	var invalid *service.IssueBudgetUsageInvalidError
	if errors.As(err, &invalid) {
		writeErrorCode(w, http.StatusBadRequest, "invalid_request", invalid.Error())
		return
	}
	switch code := service.IssueBudgetErrorCode(err); code {
	case "protocol_mismatch", "operation_conflict", "batch_conflict":
		writeErrorCode(w, http.StatusConflict, code, "budget usage conflict")
		return
	}
	if errors.Is(err, service.ErrIssueBudgetLedgerUnavailable) {
		writeErrorCode(w, http.StatusServiceUnavailable, "ledger_unavailable", "budget ledger unavailable")
		return
	}
	writeError(w, http.StatusInternalServerError, "failed to record budget usage")
}
