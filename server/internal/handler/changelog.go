package handler

import (
	"log/slog"
	"net/http"
	"strconv"
)

// GetChangelog serves the deployment's validated history. Authentication is
// enforced by the API route group; the source is independent of workspace.
func (h *Handler) GetChangelog(w http.ResponseWriter, r *http.Request) {
	resource, err := h.changelogReader.Read()
	if err != nil {
		slog.WarnContext(r.Context(), "changelog source refresh failed", "error", err)
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "private, no-cache")
	w.Header().Set("ETag", resource.ETag())
	if etagMatches(r.Header.Get("If-None-Match"), resource.ETag()) {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	body := resource.Content()
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}
