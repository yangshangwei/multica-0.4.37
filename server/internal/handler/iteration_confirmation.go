package handler

import (
	"encoding/json"
	"net/http"
)

// RawMessage distinguishes an omitted field from explicit null. This guard
// covers release-disabled assignment, server-owned rollover fields and generic
// batch/plugin writes; confirmed individual HTTP writes use their own boundary.
func rejectUnconfirmedIterationWrite(w http.ResponseWriter, fields ...json.RawMessage) bool {
	for _, field := range fields {
		if len(field) > 0 {
			writeIterationAPIError(w, iterationAPIError(http.StatusPreconditionRequired, "iteration_confirmation_required", "Iteration changes require a confirmed operation"))
			return true
		}
	}
	return false
}
