package handler

import (
	"encoding/json"
	"net/http"
)

// RawMessage distinguishes an omitted field from explicit null. Until the
// confirmed lifecycle writer is available, no generic endpoint may silently
// accept an iteration assignment, clear, or client-authored rollover counter.
func rejectUnconfirmedIterationWrite(w http.ResponseWriter, fields ...json.RawMessage) bool {
	for _, field := range fields {
		if len(field) > 0 {
			writeIterationAPIError(w, iterationAPIError(http.StatusPreconditionRequired, "iteration_confirmation_required", "Iteration changes require a confirmed operation"))
			return true
		}
	}
	return false
}
