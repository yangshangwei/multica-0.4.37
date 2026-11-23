package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/service"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func writeDeliveryCompletionError(w http.ResponseWriter, err error, commitAttempted bool) {
	var failure service.DeliveryCompletionError
	var artifact service.ArtifactError
	var pgerr *pgconn.PgError
	status, code := 503, "delivery_completion_unavailable"
	switch {
	case commitAttempted:
		code = "delivery_completion_outcome_unknown"
	case errors.As(err, &failure):
		status, code = failure.Status, failure.Code
	case errors.Is(err, pgx.ErrNoRows):
		status, code = 404, "delivery_completion_not_found"
	case errors.As(err, &artifact):
		status = artifact.Status
		code = "delivery_completion_denied"
		if status == 409 {
			code = "delivery_completion_conflict"
		}
	case errors.Is(err, auth.ErrPasswordSession):
		status, code = 403, "delivery_completion_denied"
	case errors.As(err, &pgerr) && (pgerr.Code == "55P03" || pgerr.Code == "40P01" || pgerr.Code == "40001" || pgerr.Code == "23505"):
		status, code = 409, "delivery_completion_conflict"
	}
	writeJSON(w, status, map[string]any{"code": code, "error": code, "message": code})
}
func (h *Handler) completionAccess(r *http.Request) (service.ArtifactAccess, error) {
	principal, _, _, err := artifactPrincipal(r)
	if err != nil {
		return service.ArtifactAccess{}, service.DeliveryCompletionError{Status: 403, Code: "delivery_completion_denied"}
	}
	return service.ArtifactAccess{
		Credential:                         artifactCredentialSnapshot(r.Context(), principal),
		Metrics:                            h.Metrics,
		GitHubWebhookConfigIdentity:        service.DeliveryWebhookConfigIdentity(githubWebhookSecret()),
		CollaborationAdministrationEnabled: h.cfg.PlatformAdminEnabled,
	}, nil
}
func completionRequestKey(r *http.Request) (pgtype.UUID, error) {
	keys := r.Header.Values("Idempotency-Key")
	if len(keys) != 1 {
		return pgtype.UUID{}, service.DeliveryCompletionError{Status: 400, Code: "invalid_delivery_completion_key"}
	}
	key, err := delegationKey(keys[0])
	if err != nil {
		return key, service.DeliveryCompletionError{Status: 400, Code: "invalid_delivery_completion_key"}
	}
	return key, nil
}

// Auth-only routes still reject conflicting workspace selectors. They never
// delete selection headers or reinterpret an authorization in another workspace.
func completionWorkspaceSelection(r *http.Request, q *db.Queries, workspace pgtype.UUID) error {
	if ids := r.Header.Values("X-Workspace-ID"); len(ids) > 1 || len(ids) == 1 && ids[0] != "" && ids[0] != uuidToString(workspace) {
		return service.DeliveryCompletionError{Status: 403, Code: "delivery_completion_denied"}
	}
	slugs := r.Header.Values("X-Workspace-Slug")
	if len(slugs) > 1 {
		return service.DeliveryCompletionError{Status: 403, Code: "delivery_completion_denied"}
	}
	if len(slugs) == 1 && slugs[0] != "" {
		row, err := q.GetWorkspace(r.Context(), workspace)
		if err != nil {
			return err
		}
		if row.Slug != slugs[0] {
			return service.DeliveryCompletionError{Status: 403, Code: "delivery_completion_denied"}
		}
	}
	return nil
}
func (h *Handler) CreateDeliveryCompletionAuthorization(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	access, err := h.completionAccess(r)
	if err != nil {
		writeDeliveryCompletionError(w, err, false)
		return
	}
	kind, _, err := access.Credential.Actor()
	if err != nil || kind != "human" {
		writeDeliveryCompletionError(w, service.DeliveryCompletionError{Status: 403, Code: "delivery_completion_denied"}, false)
		return
	}
	key, err := completionRequestKey(r)
	if err != nil {
		writeDeliveryCompletionError(w, err, false)
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		writeDeliveryCompletionError(w, service.DeliveryCompletionError{Status: 400, Code: "invalid_delivery_completion_request"}, false)
		return
	}
	req, err := service.ParseDeliveryCompletionAuthorization(body)
	if err != nil {
		writeDeliveryCompletionError(w, err, false)
		return
	}
	if h.TxStarter == nil {
		writeDeliveryCompletionError(w, errors.New("transaction unavailable"), false)
		return
	}
	tx, err := h.TxStarter.Begin(r.Context())
	if err != nil {
		writeDeliveryCompletionError(w, err, false)
		return
	}
	defer tx.Rollback(r.Context())
	if err = completionWorkspaceSelection(r, db.New(tx), parseUUID(req.SourceWorkspaceID)); err != nil {
		writeDeliveryCompletionError(w, err, false)
		return
	}
	out, err := service.CreateDeliveryCompletionAuthorizationInTx(r.Context(), tx, access, key, req)
	if err != nil {
		writeDeliveryCompletionError(w, err, false)
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		writeDeliveryCompletionError(w, err, true)
		return
	}
	status := http.StatusCreated
	if out.Replayed {
		status = http.StatusOK
	}
	writeJSON(w, status, out)
}
func (h *Handler) GetDeliveryCompletionAuthorization(w http.ResponseWriter, r *http.Request) {
	h.deliveryCompletionReadOrRevoke(w, r, false, false)
}
func (h *Handler) GetDeliveryCompletionAuthorizationByRequestKey(w http.ResponseWriter, r *http.Request) {
	h.deliveryCompletionReadOrRevoke(w, r, true, false)
}
func (h *Handler) RevokeDeliveryCompletionAuthorization(w http.ResponseWriter, r *http.Request) {
	h.deliveryCompletionReadOrRevoke(w, r, false, true)
}
func (h *Handler) deliveryCompletionReadOrRevoke(w http.ResponseWriter, r *http.Request, byKey, revoke bool) {
	w.Header().Set("Cache-Control", "no-store")
	access, err := h.completionAccess(r)
	if err != nil {
		writeDeliveryCompletionError(w, err, false)
		return
	}
	var id, key, workspace pgtype.UUID
	var reason string
	if byKey {
		id, err = delegationKey(chi.URLParam(r, "requestKey"))
		values := r.URL.Query()["workspace_id"]
		if err == nil && len(values) == 1 {
			workspace, err = delegationKey(values[0])
		} else {
			err = errors.New("invalid key scope")
		}
	} else {
		id, err = delegationKey(chi.URLParam(r, "authorizationId"))
	}
	if err != nil {
		writeDeliveryCompletionError(w, service.DeliveryCompletionError{Status: 400, Code: "invalid_delivery_completion_request"}, false)
		return
	}
	if revoke {
		key, err = completionRequestKey(r)
		if err != nil {
			writeDeliveryCompletionError(w, err, false)
			return
		}
		var fields map[string]json.RawMessage
		if !decodeCollaborationWorkflowBodyLimit(w, r, &fields, 8192) {
			return
		}
		raw, ok := fields["reason"]
		if !ok || len(fields) != 1 || json.Unmarshal(raw, &reason) != nil {
			writeDeliveryCompletionError(w, service.DeliveryCompletionError{Status: 400, Code: "invalid_delivery_completion_request"}, false)
			return
		}
		reason = strings.TrimSpace(reason)
	}
	if h.TxStarter == nil {
		writeDeliveryCompletionError(w, errors.New("transaction unavailable"), false)
		return
	}
	tx, err := h.TxStarter.Begin(r.Context())
	if err != nil {
		writeDeliveryCompletionError(w, err, false)
		return
	}
	defer tx.Rollback(r.Context())
	var out service.DeliveryCompletionAuthorizationView
	switch {
	case byKey:
		out, err = service.GetDeliveryCompletionByRequestInTx(r.Context(), tx, access, workspace, id)
	case revoke:
		out, err = service.RevokeDeliveryCompletionAuthorizationInTx(r.Context(), tx, access, id, key, reason)
	default:
		out, err = service.GetDeliveryCompletionAuthorizationInTx(r.Context(), tx, access, id)
	}
	if err != nil {
		writeDeliveryCompletionError(w, err, false)
		return
	}
	// Only after the current actor has access may selector validation reveal the
	// target's source. This also prevents a header from changing grant identity.
	issue, err := db.New(tx).GetIssue(r.Context(), parseUUID(out.IssueID))
	if err == nil {
		err = completionWorkspaceSelection(r, db.New(tx), issue.WorkspaceID)
	}
	if err == nil {
		err = service.CheckArtifactCredentialInTx(r.Context(), tx, access.Credential)
	}
	if err != nil {
		writeDeliveryCompletionError(w, err, false)
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		writeDeliveryCompletionError(w, err, true)
		return
	}
	writeJSON(w, 200, out)
}
