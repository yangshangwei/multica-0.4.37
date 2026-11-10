package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/service"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

const deliveryPolicyStage = service.DeliveryPolicyStage

type deliveryPolicyRequest struct {
	ExpectedRevision int64  `json:"expected_policy_revision"`
	Mode             string `json:"mode"`
	PolicyVersion    string `json:"policy_version"`
	Stage            string `json:"stage"`
	Reason           string `json:"reason"`
}

type deliveryPolicyResponse = service.DeliveryPolicy

func decodeDeliveryPolicy(w http.ResponseWriter, r *http.Request) (deliveryPolicyRequest, bool) {
	var req deliveryPolicyRequest
	var fields map[string]json.RawMessage
	if !decodeCollaborationWorkflowBodyLimit(w, r, &fields, 8192) {
		return req, false
	}
	valid := len(fields) == 5
	for _, key := range []string{"expected_policy_revision", "mode", "policy_version", "stage", "reason"} {
		value, ok := fields[key]
		valid = valid && ok && !bytes.Equal(value, []byte("null"))
	}
	raw, _ := json.Marshal(fields)
	if json.Unmarshal(raw, &req) != nil {
		valid = false
	}
	req.Reason = strings.TrimSpace(req.Reason)
	valid = valid && req.ExpectedRevision >= 0 && req.ExpectedRevision < math.MaxInt64 && (req.Mode == "off" || req.Mode == "observe" || req.Mode == "enforce") && req.PolicyVersion == deliveryEvidencePolicy && req.Stage == deliveryPolicyStage && utf8.ValidString(req.Reason) && utf8.RuneCountInString(req.Reason) >= 1 && utf8.RuneCountInString(req.Reason) <= 1000
	if !valid {
		writeError(w, 400, "invalid_delivery_policy_request")
	}
	return req, valid
}

func (h *Handler) deliveryPolicyTx(ctx context.Context) (pgx.Tx, *db.Queries, error) {
	tx, q, err := h.artifactTx(ctx)
	if err != nil {
		return nil, nil, err
	}
	if err = service.CheckDeliveryPolicySchema(ctx, tx); err != nil {
		tx.Rollback(ctx)
		return nil, nil, artifactCoreError(err)
	}
	return tx, q, nil
}

func currentDeliveryPolicy(ctx context.Context, tx pgx.Tx, ws, project pgtype.UUID) (deliveryPolicyResponse, error) {
	policy, err := service.CurrentDeliveryPolicyInTx(ctx, tx, ws, project)
	return policy, artifactCoreError(err)
}

func (h *Handler) GetDeliveryPolicy(w http.ResponseWriter, r *http.Request) {
	h.deliveryPolicy(w, r, false)
}
func (h *Handler) UpdateDeliveryPolicy(w http.ResponseWriter, r *http.Request) {
	h.deliveryPolicy(w, r, true)
}

func (h *Handler) deliveryPolicy(w http.ResponseWriter, r *http.Request, write bool) {
	w.Header().Set("Cache-Control", "no-store")
	p, kind, _, err := artifactPrincipal(r)
	if err != nil || kind != "human" {
		writeArtifactError(w, artifactDeny())
		return
	}
	ws, ok := parseUUIDOrBadRequest(w, h.resolveWorkspaceID(r), "workspace_id")
	if !ok {
		return
	}
	projectID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "project_id")
	if !ok {
		return
	}
	var req deliveryPolicyRequest
	if write {
		req, ok = decodeDeliveryPolicy(w, r)
		if !ok {
			return
		}
	}
	tx, q, err := h.deliveryPolicyTx(r.Context())
	if err != nil {
		writeArtifactError(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	out, err := h.deliveryPolicyInTx(r, tx, q, p, ws, projectID, req, write)
	if err == nil {
		err = tx.Commit(r.Context())
	}
	if err != nil {
		writeArtifactError(w, err)
		return
	}
	writeJSON(w, 200, out)
}

func (h *Handler) deliveryPolicyInTx(r *http.Request, tx pgx.Tx, q *db.Queries, p auth.AuthenticatedPrincipal, ws, projectID pgtype.UUID, req deliveryPolicyRequest, write bool) (deliveryPolicyResponse, error) {
	ctx := r.Context()
	var out deliveryPolicyResponse
	if err := artifactFence(ctx, q, ws); err != nil {
		return out, err
	}
	members, err := artifactUsers(ctx, q, ws, p.UserID)
	if err != nil {
		return out, err
	}
	if err = artifactCredential(ctx, q, tx, p); err != nil {
		return out, err
	}
	if write {
		if role := members[p.UserID].Role; role != "owner" && role != "admin" {
			return out, artifactFailure{403, "delivery_policy_management_denied"}
		}
		_, err = q.LockDeliveryPolicyProject(ctx, db.LockDeliveryPolicyProjectParams{ID: projectID, WorkspaceID: ws})
	} else {
		_, err = q.LockProjectForAssociationNowait(ctx, db.LockProjectForAssociationNowaitParams{ID: projectID, WorkspaceID: ws})
	}
	if err != nil {
		return out, err
	}
	out, err = currentDeliveryPolicy(ctx, tx, ws, projectID)
	if err != nil {
		return out, err
	}
	if write {
		if out.Revision != req.ExpectedRevision {
			return out, artifactFailure{409, "delivery_policy_revision_conflict"}
		}
		if req.Mode == "enforce" {
			if err = service.CheckDeliveryEnforcementActivationSchema(ctx, tx); err != nil {
				return out, artifactCoreError(err)
			}
		}
		_, err = q.CreateDeliveryPolicy(ctx, db.CreateDeliveryPolicyParams{WorkspaceID: ws, ProjectID: projectID, Revision: out.Revision + 1, PolicyVersion: req.PolicyVersion, Mode: req.Mode, Stage: req.Stage, Reason: req.Reason, CreatedBy: parseUUID(p.UserID)})
		if err != nil {
			return out, err
		}
		out, err = currentDeliveryPolicy(ctx, tx, ws, projectID)
		if err != nil {
			return out, err
		}
	}
	// A credential may expire while the transaction is building its response.
	if err = artifactCredential(ctx, q, tx, p); err != nil {
		return out, err
	}
	return out, nil
}
