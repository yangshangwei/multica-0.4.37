package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/issuestatus"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type deliveryCompletionHintRequest struct {
	IssueRevision int64   `json:"expected_issue_revision"`
	TargetStatus  string  `json:"target_status"`
	EvaluationKey *string `json:"evaluation_key"`
}

type deliveryCompletionHintResponse struct {
	IssueID             string                      `json:"issue_id"`
	IssueRevision       int64                       `json:"issue_revision"`
	TargetStatus        string                      `json:"target_status"`
	TargetCategory      string                      `json:"target_category"`
	ProjectID           *string                     `json:"project_id"`
	PolicyRevision      int64                       `json:"policy_revision"`
	PolicyMode          string                      `json:"policy_mode"`
	Hint                string                      `json:"hint"`
	Reason              string                      `json:"reason"`
	CheckedAt           time.Time                   `json:"checked_at"`
	Enforced            bool                        `json:"enforced"`
	UsableForCompletion bool                        `json:"usable_for_completion"`
	Evaluation          *deliveryEvaluationResponse `json:"evaluation,omitempty"`
}

func decodeDeliveryCompletionHint(w http.ResponseWriter, r *http.Request) (deliveryCompletionHintRequest, bool) {
	var req deliveryCompletionHintRequest
	var fields map[string]json.RawMessage
	if !decodeCollaborationWorkflowBodyLimit(w, r, &fields, 4096) {
		return req, false
	}
	valid := len(fields) == 3
	for _, key := range []string{"expected_issue_revision", "target_status", "evaluation_key"} {
		value, ok := fields[key]
		valid = valid && ok && (key == "evaluation_key" || !bytes.Equal(value, []byte("null")))
	}
	raw, _ := json.Marshal(fields)
	if json.Unmarshal(raw, &req) != nil {
		valid = false
	}
	req.TargetStatus = strings.ToLower(strings.TrimSpace(req.TargetStatus))
	valid = valid && req.IssueRevision >= 0 && req.TargetStatus != ""
	if req.EvaluationKey != nil {
		_, err := delegationKey(*req.EvaluationKey)
		valid = valid && err == nil
	}
	if !valid {
		writeError(w, 400, "invalid_delivery_completion_hint_request")
		return req, false
	}
	if !issuestatus.IsBuiltIn(req.TargetStatus) {
		if _, err := issuestatus.ValidateKey(req.TargetStatus); err != nil {
			writeError(w, 400, "invalid_delivery_completion_status")
			return req, false
		}
	}
	return req, true
}

// GetDeliveryCompletionHint describes evidence at this read transaction only.
// It never authorizes or performs a subsequent lifecycle transition.
func (h *Handler) GetDeliveryCompletionHint(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	p, kind, _, err := artifactPrincipal(r)
	if err != nil || kind != "human" {
		writeArtifactError(w, artifactDeny())
		return
	}
	req, ok := decodeDeliveryCompletionHint(w, r)
	if !ok {
		return
	}
	issue, ok := h.loadIssueForUser(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	tx, q, err := h.deliveryPolicyTx(r.Context())
	if err != nil {
		writeArtifactError(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	out, err := h.deliveryCompletionHintInTx(r, tx, q, p, issue, req)
	if err == nil {
		err = artifactCredential(r.Context(), q, tx, p)
	}
	if err == nil {
		err = tx.Commit(r.Context())
	}
	if err != nil {
		writeArtifactError(w, err)
		return
	}
	writeJSON(w, 200, out)
}

func (h *Handler) deliveryCompletionHintInTx(r *http.Request, tx pgx.Tx, q *db.Queries, p auth.AuthenticatedPrincipal, initial db.Issue, req deliveryCompletionHintRequest) (deliveryCompletionHintResponse, error) {
	ctx := r.Context()
	var out deliveryCompletionHintResponse
	if err := artifactFence(ctx, q, initial.WorkspaceID); err != nil {
		return out, err
	}
	// Keep the existing catalog-before-issue order. Built-ins cannot be archived.
	if !issuestatus.IsBuiltIn(req.TargetStatus) {
		if err := q.LockIssueStatusCatalogShared(ctx, initial.WorkspaceID); err != nil {
			return out, err
		}
	}
	issue, err := q.LockLifecycleIssue(ctx, db.LockLifecycleIssueParams{ID: initial.ID, WorkspaceID: initial.WorkspaceID})
	if err != nil {
		return out, err
	}
	if _, err = artifactUsers(ctx, q, issue.WorkspaceID, p.UserID); err != nil {
		return out, err
	}
	if err = artifactCredential(ctx, q, tx, p); err != nil {
		return out, err
	}
	if issue.Revision != req.IssueRevision {
		return out, artifactFailure{409, "delivery_issue_revision_conflict"}
	}
	status, err := issuestatus.Resolve(ctx, q, issue.WorkspaceID, req.TargetStatus)
	if errors.Is(err, issuestatus.ErrUnknownStatus) {
		return out, artifactFailure{400, "invalid_delivery_completion_status"}
	}
	if err != nil {
		return out, err
	}
	out = deliveryCompletionHintResponse{IssueID: uuidToString(issue.ID), IssueRevision: issue.Revision, TargetStatus: status.Key, TargetCategory: status.Category, PolicyMode: "off", Hint: "not_applicable"}
	policy := deliveryPolicyResponse{Mode: "off"}
	if issue.ProjectID.Valid {
		project, err := q.LockProjectForAssociationNowait(ctx, db.LockProjectForAssociationNowaitParams{ID: issue.ProjectID, WorkspaceID: issue.WorkspaceID})
		if err != nil {
			return out, err
		}
		id := uuidToString(project.ID)
		out.ProjectID = &id
		policy, err = currentDeliveryPolicy(ctx, tx, issue.WorkspaceID, issue.ProjectID)
		if err != nil {
			return out, err
		}
		out.PolicyRevision, out.PolicyMode = policy.Revision, policy.Mode
	}
	switch {
	case status.Category != deliveryPolicyStage:
		out.Reason = "target_not_done"
	case !issue.ProjectID.Valid:
		out.Reason = "no_project"
	case policy.Mode == "off":
		out.Reason = "policy_off"
	case policy.Mode != "observe" || policy.PolicyVersion != deliveryEvidencePolicy || policy.Stage != deliveryPolicyStage:
		return out, artifactFailure{409, "delivery_policy_not_observing"}
	case req.EvaluationKey == nil:
		out.Hint, out.Reason = "attention", "evaluation_required"
	default:
		// The original evaluator's recovery path owns all artifact authorization
		// and applicability checks. This endpoint neither selects nor creates evidence.
		evaluation, err := h.deliveryEvaluationInTx(r, tx, q, p, issue, parseUUID(*req.EvaluationKey), deliveryEvaluationRequest{}, false)
		if err != nil {
			return out, err
		}
		out.Evaluation = &evaluation
		out.CheckedAt = evaluation.Applicability.CheckedAt
		out.Hint, out.Reason = "attention", evaluation.Applicability.Reason
		if evaluation.Observation.Verdict == "pass" && evaluation.Applicability.Status == "current" {
			out.Hint = "matching"
		}
		return out, nil
	}
	now, err := q.ArtifactExecutionDatabaseTime(ctx)
	if err != nil {
		return out, err
	}
	out.CheckedAt = now.Time.UTC()
	return out, nil
}
