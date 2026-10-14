package handler

import (
	"bytes"
	"encoding/json"
	"io"
	"math"
	"mime"
	"net/http"
	"strconv"
	"time"
	"unicode/utf8"

	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/iteration"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/util"
)

type CollaborationRecipientResponse struct {
	IssueID           *string                     `json:"issue_id"`
	SourceWorkspaceID string                      `json:"source_workspace_id"`
	Recipient         CollaborationAccountSummary `json:"recipient"`
}

type collaborationRecipientBody struct {
	Kind              string          `json:"kind"`
	Value             string          `json:"value"`
	IssueID           json.RawMessage `json:"issue_id"`
	SourceWorkspaceID json.RawMessage `json:"source_workspace_id"`
}

func (h *Handler) ResolveCollaborationRecipient(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	r.Header.Del("X-Workspace-ID")
	r.Header.Del("X-Workspace-Slug")
	principal, ok := collaboration.PrincipalFromContext(r.Context())
	if !ok || !principal.Human() || r.Header.Get("X-Agent-ID") != "" || r.Header.Get("X-Task-ID") != "" {
		writeErrorCode(w, http.StatusForbidden, "forbidden", "Recipient lookup requires a human account")
		return
	}
	actor, err := util.ParseUUID(principal.UserID)
	if err != nil {
		writeErrorCode(w, http.StatusForbidden, "forbidden", "Recipient lookup requires a human account")
		return
	}
	query, ok := decodeCollaborationRecipientQuery(w, r)
	if !ok {
		return
	}
	// Reuse the bounded limiter already installed by the router. These keys do
	// not spend authentication budgets or store the searched username/email.
	// Both the memory and configured shared backend must reject on failure.
	wait, err := h.PasswordLimiter.Allow(r.Context(),
		auth.PasswordLimit{Key: "collaboration-recipient:actor:" + auth.HashToken(util.UUIDToString(actor)), Count: 20, Window: time.Minute},
		auth.PasswordLimit{Key: "collaboration-recipient:ip:" + auth.HashToken(auth.PasswordClientIP(r)), Count: 60, Window: time.Minute},
	)
	if err != nil {
		w.Header().Set("Retry-After", "1")
		writeErrorCode(w, http.StatusServiceUnavailable, "collaboration_unavailable", "Recipient lookup is unavailable")
		return
	}
	if wait > 0 {
		w.Header().Set("Retry-After", strconv.Itoa(max(1, int(math.Ceil(wait.Seconds())))))
		writeErrorCode(w, http.StatusTooManyRequests, "rate_limited", "Too many recipient lookups")
		return
	}
	svc := service.NewCollaborationService(h.Queries, h.TxStarter, h.IssueService)
	result, err := svc.ResolveRecipient(r.Context(), query)
	if err != nil {
		writeCollaborationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, CollaborationRecipientResponse{
		IssueID: util.UUIDToPtr(result.IssueID), SourceWorkspaceID: util.UUIDToString(result.SourceWorkspaceID),
		Recipient: CollaborationAccountSummary{UserID: util.UUIDToString(result.Recipient.UserID),
			Name: result.Recipient.Name, AvatarURL: result.Recipient.AvatarURL, External: result.Recipient.External},
	})
}

func decodeCollaborationRecipientQuery(w http.ResponseWriter, r *http.Request) (service.CollaborationRecipientQuery, bool) {
	var out service.CollaborationRecipientQuery
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		writeErrorCode(w, http.StatusUnsupportedMediaType, "json_required", "JSON request required")
		return out, false
	}
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 4096))
	if err != nil || !utf8.Valid(raw) || iteration.ValidateObjectJSON(raw) != nil {
		writeErrorCode(w, http.StatusBadRequest, "invalid_recipient_query", "Invalid recipient query")
		return out, false
	}
	var body collaborationRecipientBody
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&body) != nil {
		writeErrorCode(w, http.StatusBadRequest, "invalid_recipient_query", "Invalid recipient query")
		return out, false
	}
	out.Kind, out.Value = body.Kind, body.Value
	// Presence matters: explicit null cannot act as an omitted second context.
	if (len(body.IssueID) > 0) == (len(body.SourceWorkspaceID) > 0) {
		writeErrorCode(w, http.StatusBadRequest, "invalid_recipient_context", "Exactly one recipient context is required")
		return out, false
	}
	contextValue := body.IssueID
	if len(body.SourceWorkspaceID) > 0 {
		contextValue = body.SourceWorkspaceID
	}
	var value string
	if json.Unmarshal(contextValue, &value) != nil {
		writeErrorCode(w, http.StatusBadRequest, "invalid_recipient_context", "Invalid recipient context")
		return out, false
	}
	id, err := util.ParseUUID(value)
	if err != nil {
		writeErrorCode(w, http.StatusBadRequest, "invalid_recipient_context", "Invalid recipient context")
		return out, false
	}
	if len(body.IssueID) > 0 {
		out.IssueID = id
	} else {
		out.SourceWorkspaceID = id
	}
	return out, true
}
