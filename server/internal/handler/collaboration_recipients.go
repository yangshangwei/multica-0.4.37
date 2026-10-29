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

	"github.com/jackc/pgx/v5/pgtype"
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
	var body collaborationRecipientBody
	if !decodeCollaborationRecipientBody(w, r, &body) {
		return out, false
	}
	out.Kind, out.Value = body.Kind, body.Value
	var ok bool
	out.IssueID, out.SourceWorkspaceID, ok = decodeCollaborationRecipientContext(w, body.IssueID, body.SourceWorkspaceID)
	return out, ok
}

// decodeCollaborationRecipientBody applies the strict JSON rules shared by
// recipient resolve and search.
func decodeCollaborationRecipientBody(w http.ResponseWriter, r *http.Request, dst any) bool {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		writeErrorCode(w, http.StatusUnsupportedMediaType, "json_required", "JSON request required")
		return false
	}
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 4096))
	if err != nil || !utf8.Valid(raw) || iteration.ValidateObjectJSON(raw) != nil {
		writeErrorCode(w, http.StatusBadRequest, "invalid_recipient_query", "Invalid recipient query")
		return false
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(dst) != nil {
		writeErrorCode(w, http.StatusBadRequest, "invalid_recipient_query", "Invalid recipient query")
		return false
	}
	return true
}

func decodeCollaborationRecipientContext(w http.ResponseWriter, issueRaw, sourceRaw json.RawMessage) (pgtype.UUID, pgtype.UUID, bool) {
	var issueID, sourceID pgtype.UUID
	// Presence matters: explicit null cannot act as an omitted second context.
	if (len(issueRaw) > 0) == (len(sourceRaw) > 0) {
		writeErrorCode(w, http.StatusBadRequest, "invalid_recipient_context", "Exactly one recipient context is required")
		return issueID, sourceID, false
	}
	contextValue := issueRaw
	if len(sourceRaw) > 0 {
		contextValue = sourceRaw
	}
	var value string
	if json.Unmarshal(contextValue, &value) != nil {
		writeErrorCode(w, http.StatusBadRequest, "invalid_recipient_context", "Invalid recipient context")
		return issueID, sourceID, false
	}
	id, err := util.ParseUUID(value)
	if err != nil {
		writeErrorCode(w, http.StatusBadRequest, "invalid_recipient_context", "Invalid recipient context")
		return issueID, sourceID, false
	}
	if len(issueRaw) > 0 {
		return id, sourceID, true
	}
	return issueID, id, true
}

// CollaborationRecipientAccount is the picker projection: no email, username
// or workspace. External is null when the caller is not a source member.
type CollaborationRecipientAccount struct {
	UserID    string `json:"user_id"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatar_url"`
	External  *bool  `json:"external"`
}

type CollaborationRecipientSearchResponse struct {
	IssueID           *string                         `json:"issue_id"`
	SourceWorkspaceID string                          `json:"source_workspace_id"`
	Scope             string                          `json:"scope"`
	SourceMember      bool                            `json:"source_member"`
	Recent            []CollaborationRecipientAccount `json:"recent"`
	Recipients        []CollaborationRecipientAccount `json:"recipients"`
	NextCursor        *string                         `json:"next_cursor"`
}

type collaborationRecipientSearchBody struct {
	IssueID           json.RawMessage `json:"issue_id"`
	SourceWorkspaceID json.RawMessage `json:"source_workspace_id"`
	Q                 string          `json:"q"`
	Cursor            string          `json:"cursor"`
	Limit             *int            `json:"limit"`
	MembersOnly       bool            `json:"members_only"`
}

func (h *Handler) SearchCollaborationRecipients(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	r.Header.Del("X-Workspace-ID")
	r.Header.Del("X-Workspace-Slug")
	principal, ok := collaboration.PrincipalFromContext(r.Context())
	if !ok || !principal.Human() || r.Header.Get("X-Agent-ID") != "" || r.Header.Get("X-Task-ID") != "" {
		writeErrorCode(w, http.StatusForbidden, "forbidden", "Recipient search requires a human account")
		return
	}
	actor, err := util.ParseUUID(principal.UserID)
	if err != nil {
		writeErrorCode(w, http.StatusForbidden, "forbidden", "Recipient search requires a human account")
		return
	}
	var body collaborationRecipientSearchBody
	if !decodeCollaborationRecipientBody(w, r, &body) {
		return
	}
	issueID, sourceID, ok := decodeCollaborationRecipientContext(w, body.IssueID, body.SourceWorkspaceID)
	if !ok {
		return
	}
	// Typing and paging spend their own budget, so the picker cannot use up the
	// exact lookup's budget or the reverse. Limiter failure rejects, as resolve.
	wait, err := h.PasswordLimiter.Allow(r.Context(),
		auth.PasswordLimit{Key: "collaboration-recipient-search:actor:" + auth.HashToken(util.UUIDToString(actor)), Count: 120, Window: time.Minute},
		auth.PasswordLimit{Key: "collaboration-recipient-search:ip:" + auth.HashToken(auth.PasswordClientIP(r)), Count: 300, Window: time.Minute},
	)
	if err != nil {
		w.Header().Set("Retry-After", "1")
		writeErrorCode(w, http.StatusServiceUnavailable, "collaboration_unavailable", "Recipient search is unavailable")
		return
	}
	if wait > 0 {
		w.Header().Set("Retry-After", strconv.Itoa(max(1, int(math.Ceil(wait.Seconds())))))
		writeErrorCode(w, http.StatusTooManyRequests, "rate_limited", "Too many recipient searches")
		return
	}
	svc := service.NewCollaborationService(h.Queries, h.TxStarter, h.IssueService)
	result, err := svc.SearchRecipients(r.Context(), service.CollaborationRecipientSearchQuery{
		IssueID: issueID, SourceWorkspaceID: sourceID, Query: body.Q, Cursor: body.Cursor, Limit: body.Limit,
		MembersOnly: body.MembersOnly, Directory: h.cfg.CollaborationDirectoryEnabled,
	})
	if err != nil {
		writeCollaborationError(w, err)
		return
	}
	var next *string
	if result.NextCursor != "" {
		next = &result.NextCursor
	}
	writeJSON(w, http.StatusOK, CollaborationRecipientSearchResponse{
		IssueID: util.UUIDToPtr(result.IssueID), SourceWorkspaceID: util.UUIDToString(result.SourceWorkspaceID),
		Scope: result.Scope, SourceMember: result.SourceMember, Recent: collaborationRecipientAccounts(result.Recent),
		Recipients: collaborationRecipientAccounts(result.Recipients), NextCursor: next,
	})
}

func collaborationRecipientAccounts(items []service.CollaborationRecipientCandidate) []CollaborationRecipientAccount {
	out := make([]CollaborationRecipientAccount, 0, len(items))
	for _, item := range items {
		out = append(out, CollaborationRecipientAccount{UserID: util.UUIDToString(item.UserID), Name: item.Name, AvatarURL: item.AvatarURL, External: item.External})
	}
	return out
}
