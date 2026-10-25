package handler

import (
	"encoding/json"
	"net/http"
	"net/url"

	"github.com/multica-ai/multica/server/internal/service"
)

// Negotiate explicitly, before endpoint-specific query validation. Removing
// only this metadata keeps existing narrow resource/cursor query contracts.
func collaborationExecutionRead(w http.ResponseWriter, r *http.Request) bool {
	query, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_query"})
		return false
	}
	version := 5
	if values, supplied := query["execution_view_version"]; supplied {
		if len(values) != 1 || values[0] != "5" && values[0] != "6" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"code": "unsupported_execution_view_version"})
			return false
		}
		if values[0] == "6" {
			version = 6
		}
		query.Del("execution_view_version")
		requestURL := *r.URL
		requestURL.RawQuery = query.Encode()
		r.URL = &requestURL
	}
	*r = *r.WithContext(service.WithCollaborationExecutionViewVersion(r.Context(), version))
	return true
}

func collaborationExecutionWriteVersion(w http.ResponseWriter, raw json.RawMessage) (int, bool) {
	if len(raw) == 0 {
		return 0, true
	}
	var version int
	if json.Unmarshal(raw, &version) != nil || version != 6 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"code": "unsupported_execution_view_version"})
		return 0, false
	}
	return version, true
}
