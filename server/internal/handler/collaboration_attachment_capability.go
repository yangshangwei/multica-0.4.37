package handler

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/service"
)

const collaborationAttachmentCapabilityTTL = time.Minute

type collaborationAttachmentCapability struct {
	ProtocolVersion int                                      `json:"protocol_version"`
	IssueID         pgtype.UUID                              `json:"issue_id"`
	AttachmentID    pgtype.UUID                              `json:"attachment_id"`
	Version         int64                                    `json:"version,string"`
	Access          service.CollaborationAttachmentReadStamp `json:"access"`
	PasswordVersion int64                                    `json:"password_version,string"`
	Intent          string                                   `json:"intent"`
	ExpiresAt       int64                                    `json:"expires_at"`
}

func collaborationAttachmentMAC(encoded string) []byte {
	key := sha256.Sum256(append([]byte("collaboration-attachment-read:v1:"), auth.JWTSecret()...))
	mac := hmac.New(sha256.New, key[:])
	_, _ = mac.Write([]byte(encoded))
	return mac.Sum(nil)
}

func encodeCollaborationAttachmentCapability(cap collaborationAttachmentCapability) (string, error) {
	raw, err := json.Marshal(cap)
	if err != nil {
		return "", err
	}
	encoded := base64.RawURLEncoding.EncodeToString(raw)
	return encoded + "." + base64.RawURLEncoding.EncodeToString(collaborationAttachmentMAC(encoded)), nil
}

func decodeCollaborationAttachmentCapability(raw string, now time.Time) (collaborationAttachmentCapability, bool) {
	var cap collaborationAttachmentCapability
	if len(raw) > 4096 {
		return cap, false
	}
	encoded, signature, found := strings.Cut(raw, ".")
	if !found {
		return cap, false
	}
	sig, err := base64.RawURLEncoding.DecodeString(signature)
	if err != nil || !hmac.Equal(sig, collaborationAttachmentMAC(encoded)) {
		return cap, false
	}
	data, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil || json.Unmarshal(data, &cap) != nil || cap.ProtocolVersion != 1 ||
		!cap.IssueID.Valid || !cap.AttachmentID.Valid || !cap.Access.ViewerID.Valid || cap.Version < 1 ||
		cap.Access.AccountEpoch < 1 || cap.Access.ParticipantEpoch < 0 || (!cap.Access.MemberID.Valid && cap.Access.ParticipantEpoch == 0) ||
		(cap.Intent != "content" && cap.Intent != "download") || now.Unix() >= cap.ExpiresAt ||
		cap.ExpiresAt > now.Add(collaborationAttachmentCapabilityTTL).Unix() {
		return cap, false
	}
	return cap, true
}

func (h *Handler) CreateCollaborationAttachmentCapability(w http.ResponseWriter, r *http.Request) {
	prepareCollaborationContentRequest(w, r)
	var body struct {
		Intent string `json:"intent"`
	}
	if !decodeCollaborationWorkflowBody(w, r, &body) {
		return
	}
	if body.Intent != "content" && body.Intent != "download" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_attachment_intent", "error": "invalid_attachment_intent"})
		return
	}
	issue, _, id, ok := collaborationAttachmentPath(w, r)
	if !ok {
		return
	}
	svc := h.collaborationAttachmentService()
	row, err := svc.GetAttachment(r.Context(), issue, pgtype.UUID{}, id)
	if err != nil {
		writeCollaborationError(w, err)
		return
	}
	stamp, err := svc.MintAttachmentReadStamp(r.Context(), issue, id)
	if err != nil {
		writeCollaborationError(w, err)
		return
	}
	cap := collaborationAttachmentCapability{ProtocolVersion: 1, IssueID: issue, AttachmentID: id, Version: row.Version,
		Access: stamp, Intent: body.Intent, ExpiresAt: time.Now().Add(collaborationAttachmentCapabilityTTL).Unix()}
	if session, ok := auth.PasswordSessionFromContext(r.Context()); ok {
		cap.PasswordVersion = session.Version
	}
	token, err := encodeCollaborationAttachmentCapability(cap)
	if err != nil {
		writeCollaborationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"issue_id": issue, "attachment_id": id, "version": strconv.FormatInt(row.Version, 10),
		"url":        "/api/collaboration/issues/" + uuidToString(issue) + "/attachments/" + uuidToString(id) + "/capability?cap=" + token,
		"expires_at": time.Unix(cap.ExpiresAt, 0).UTC(),
	})
}

// The only public route for collaboration files. Its signature authorizes
// reconstruction of this bounded read context, never generic human API access.
func (h *Handler) ReadCollaborationAttachmentCapability(w http.ResponseWriter, r *http.Request) {
	prepareCollaborationContentRequest(w, r)
	values := r.URL.Query()["cap"]
	if len(values) != 1 || len(r.URL.Query()) != 1 {
		writeJSON(w, http.StatusNotFound, map[string]string{"code": "not_found", "error": "not_found"})
		return
	}
	cap, ok := decodeCollaborationAttachmentCapability(values[0], time.Now())
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"code": "not_found", "error": "not_found"})
		return
	}
	issue, _, id, ok := collaborationAttachmentPath(w, r)
	if !ok {
		return
	}
	if issue != cap.IssueID || id != cap.AttachmentID {
		writeJSON(w, http.StatusNotFound, map[string]string{"code": "not_found", "error": "not_found"})
		return
	}
	ctx := auth.WithAuthenticatedPrincipal(r.Context(), auth.AuthenticatedPrincipal{UserID: uuidToString(cap.Access.ViewerID), Kind: auth.CredentialJWT})
	if cap.PasswordVersion > 0 {
		ctx = auth.WithPasswordSession(ctx, auth.PasswordSession{UserID: uuidToString(cap.Access.ViewerID), Version: cap.PasswordVersion, Kind: "jwt"})
	}
	r = r.WithContext(ctx)
	svc := h.collaborationAttachmentService()
	row, err := svc.CheckAttachmentReadStamp(ctx, issue, id, cap.Version, cap.Access)
	if err != nil {
		writeCollaborationError(w, err)
		return
	}
	h.serveCollaborationAttachmentBytes(w, r, svc, row, cap.Intent == "download", func(ctx context.Context) error {
		if _, ok := decodeCollaborationAttachmentCapability(values[0], time.Now()); !ok {
			return &service.CollaborationError{Status: http.StatusNotFound, Code: "not_found"}
		}
		_, err := svc.CheckAttachmentReadStamp(ctx, issue, id, cap.Version, cap.Access)
		return err
	})
}
