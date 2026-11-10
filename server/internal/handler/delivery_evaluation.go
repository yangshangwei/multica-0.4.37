package handler

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/auth"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type deliveryEvaluationRequest struct {
	deliveryEvidenceRequest
	PolicyRevision int64 `json:"expected_policy_revision"`
}

type deliveryApplicability struct {
	Status    string    `json:"status"`
	Reason    string    `json:"reason"`
	CheckedAt time.Time `json:"checked_at"`
}

type deliveryEvaluationResponse struct {
	EvaluationID        string                   `json:"evaluation_id"`
	RequestKey          string                   `json:"request_key"`
	EvaluatorUserID     string                   `json:"evaluator_user_id"`
	PolicyRevision      int64                    `json:"policy_revision"`
	Stage               string                   `json:"stage"`
	Observation         deliveryEvidenceResponse `json:"observation"`
	Applicability       deliveryApplicability    `json:"applicability"`
	UsableForCompletion bool                     `json:"usable_for_completion"`
}

func decodeDeliveryEvaluation(w http.ResponseWriter, r *http.Request) (deliveryEvaluationRequest, bool) {
	var req deliveryEvaluationRequest
	var fields map[string]json.RawMessage
	if !decodeCollaborationWorkflowBodyLimit(w, r, &fields, 4096) {
		return req, false
	}
	raw, ok := fields["expected_policy_revision"]
	valid := ok && !bytes.Equal(raw, []byte("null")) && json.Unmarshal(raw, &req.PolicyRevision) == nil && req.PolicyRevision >= 0
	delete(fields, "expected_policy_revision")
	req.deliveryEvidenceRequest, ok = deliveryEvidenceFields(fields)
	valid = valid && ok
	if !valid {
		writeError(w, 400, "invalid_delivery_evaluation_request")
	}
	return req, valid
}

func deliveryEvaluationDigest(issueID pgtype.UUID, req deliveryEvaluationRequest) string {
	raw, _ := json.Marshal(struct {
		IssueID string                    `json:"issue_id"`
		Request deliveryEvaluationRequest `json:"request"`
	}{uuidToString(issueID), req})
	hash := sha256.Sum256(append([]byte("multica-delivery-evaluation-v1\x00"), raw...))
	return hex.EncodeToString(hash[:])
}

func (h *Handler) CreateDeliveryEvaluation(w http.ResponseWriter, r *http.Request) {
	h.deliveryEvaluation(w, r, true)
}
func (h *Handler) GetDeliveryEvaluation(w http.ResponseWriter, r *http.Request) {
	h.deliveryEvaluation(w, r, false)
}

func (h *Handler) deliveryEvaluation(w http.ResponseWriter, r *http.Request, create bool) {
	w.Header().Set("Cache-Control", "no-store")
	p, kind, _, err := artifactPrincipal(r)
	if err != nil || kind != "human" {
		writeArtifactError(w, artifactDeny())
		return
	}
	var req deliveryEvaluationRequest
	keyText := chi.URLParam(r, "key")
	if create {
		keys := r.Header.Values("Idempotency-Key")
		if len(keys) != 1 {
			writeError(w, 400, "invalid_delivery_evaluation_key")
			return
		}
		keyText = keys[0]
	}
	key, err := delegationKey(keyText)
	if err != nil {
		writeError(w, 400, "invalid_delivery_evaluation_key")
		return
	}
	if create {
		var ok bool
		req, ok = decodeDeliveryEvaluation(w, r)
		if !ok {
			return
		}
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
	out, err := h.deliveryEvaluationInTx(r, tx, q, p, issue, key, req, create)
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

func (h *Handler) deliveryEvaluationInTx(r *http.Request, tx pgx.Tx, q *db.Queries, p auth.AuthenticatedPrincipal, initial db.Issue, key pgtype.UUID, req deliveryEvaluationRequest, create bool) (deliveryEvaluationResponse, error) {
	ctx := r.Context()
	var out deliveryEvaluationResponse
	if err := artifactFence(ctx, q, initial.WorkspaceID); err != nil {
		return out, err
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
	var project db.Project
	if issue.ProjectID.Valid {
		project, err = q.LockProjectForAssociationNowait(ctx, db.LockProjectForAssociationNowaitParams{ID: issue.ProjectID, WorkspaceID: issue.WorkspaceID})
		if err != nil {
			return out, err
		}
	}
	lookup := db.GetDeliveryEvaluationParams{WorkspaceID: issue.WorkspaceID, EvaluatorUserID: parseUUID(p.UserID), RequestKey: key}
	row, err := q.GetDeliveryEvaluation(ctx, lookup)
	if err == nil {
		if row.IssueID != issue.ID || (create && row.RequestDigest != deliveryEvaluationDigest(issue.ID, req)) {
			return out, artifactFailure{409, "delivery_evaluation_key_conflict"}
		}
		return h.recoverDeliveryEvaluation(r, tx, q, p, issue, project, row)
	}
	if !errors.Is(err, pgx.ErrNoRows) || !create {
		return out, err
	}
	if !issue.ProjectID.Valid {
		return out, artifactFailure{409, "delivery_project_required"}
	}
	policy, err := currentDeliveryPolicy(ctx, tx, issue.WorkspaceID, issue.ProjectID)
	if err != nil {
		return out, err
	}
	if policy.Revision != req.PolicyRevision {
		return out, artifactFailure{409, "delivery_policy_revision_conflict"}
	}
	if (policy.Mode != "observe" && policy.Mode != "enforce") || policy.PolicyVersion != deliveryEvidencePolicy || policy.Stage != deliveryPolicyStage {
		return out, artifactFailure{409, "delivery_policy_not_observing"}
	}
	observation, err := h.evaluateDeliveryEvidence(r, tx, q, p, issue, req.deliveryEvidenceRequest)
	if err != nil {
		return out, err
	}
	rawRequest, err := json.Marshal(req)
	if err != nil {
		return out, err
	}
	rawObservation, err := json.Marshal(observation)
	if err != nil {
		return out, err
	}
	id, err := uuid.NewV7()
	if err != nil {
		return out, err
	}
	row, err = q.CreateDeliveryEvaluation(ctx, db.CreateDeliveryEvaluationParams{WorkspaceID: issue.WorkspaceID, EvaluatorUserID: lookup.EvaluatorUserID, RequestKey: key, EvaluationID: parseUUID(id.String()), IssueID: issue.ID, ProjectID: issue.ProjectID, PolicyRevision: policy.Revision, RequestDigest: deliveryEvaluationDigest(issue.ID, req), RequestJson: rawRequest, Observation: rawObservation})
	if errors.Is(err, pgx.ErrNoRows) {
		return out, artifactFailure{409, "delivery_evaluation_key_conflict"}
	}
	if err != nil {
		return out, err
	}
	return deliveryEvaluationResult(row, observation, deliveryEvaluationApplicability(observation, observation, policy, req.PolicyRevision)), nil
}

func (h *Handler) recoverDeliveryEvaluation(r *http.Request, tx pgx.Tx, q *db.Queries, p auth.AuthenticatedPrincipal, issue db.Issue, project db.Project, row db.DeliveryEvidenceEvaluation) (deliveryEvaluationResponse, error) {
	var out deliveryEvaluationResponse
	var req deliveryEvaluationRequest
	var saved deliveryEvidenceResponse
	if json.Unmarshal(row.RequestJson, &req) != nil || json.Unmarshal(row.Observation, &saved) != nil || row.RequestDigest != deliveryEvaluationDigest(issue.ID, req) || saved.IssueID != uuidToString(row.IssueID) || saved.ProjectID == nil || *saved.ProjectID != uuidToString(row.ProjectID) || row.PolicyRevision != req.PolicyRevision {
		return out, artifactFailure{409, "delivery_evaluation_binding_invalid"}
	}
	// Versions change applicability, never permission. Feed current locked versions
	// to the existing observer so stale requests still traverse artifact authority.
	liveRequest := req.deliveryEvidenceRequest
	liveRequest.IssueRevision = issue.Revision
	liveRequest.ProjectID = nil
	liveRequest.ProjectRevision = nil
	policy := deliveryPolicyResponse{Mode: "off"}
	if issue.ProjectID.Valid {
		id, revision := uuidToString(project.ID), project.Revision
		liveRequest.ProjectID = &id
		liveRequest.ProjectRevision = &revision
		var err error
		policy, err = currentDeliveryPolicy(r.Context(), tx, issue.WorkspaceID, issue.ProjectID)
		if err != nil {
			return out, err
		}
	}
	live, err := h.evaluateDeliveryEvidence(r, tx, q, p, issue, liveRequest)
	if err != nil {
		return out, err
	}
	if saved.ExecutionID != "" && live.ExecutionID == "" {
		return out, artifactFailure{409, "delivery_retained_binding_missing"}
	}
	return deliveryEvaluationResult(row, saved, deliveryEvaluationApplicability(saved, live, policy, row.PolicyRevision)), nil
}

func deliveryEvaluationResult(row db.DeliveryEvidenceEvaluation, observation deliveryEvidenceResponse, applicability deliveryApplicability) deliveryEvaluationResponse {
	return deliveryEvaluationResponse{EvaluationID: uuidToString(row.EvaluationID), RequestKey: uuidToString(row.RequestKey), EvaluatorUserID: uuidToString(row.EvaluatorUserID), PolicyRevision: row.PolicyRevision, Stage: deliveryPolicyStage, Observation: observation, Applicability: applicability}
}

func deliveryEvaluationApplicability(saved, live deliveryEvidenceResponse, policy deliveryPolicyResponse, revision int64) deliveryApplicability {
	out := deliveryApplicability{Status: "stale", Reason: "observation_not_passed", CheckedAt: live.ObservedAt}
	switch {
	case saved.IssueRevision != live.IssueRevision || !sameDeliveryPointer(saved.ProjectID, live.ProjectID) || !sameDeliveryPointer(saved.ProjectRevision, live.ProjectRevision):
		out.Reason = "resource_revision_changed"
	case policy.Revision != revision || (policy.Mode != "observe" && policy.Mode != "enforce") || policy.PolicyVersion != deliveryEvidencePolicy || policy.Stage != deliveryPolicyStage:
		out.Reason = "policy_changed"
	case saved.ValidUntil != nil && !live.ObservedAt.Before(*saved.ValidUntil):
		out.Status = "expired"
		out.Reason = "evidence_expired"
	case saved.Verdict == "unknown":
		out.Status = "unknown"
		out.Reason = "original_observation_unknown"
	case live.Verdict == "unknown":
		out.Status = "unknown"
		out.Reason = live.Reason
	case saved.Verdict != "pass":
		out.Reason = saved.Reason
	case live.Verdict != "pass":
		out.Reason = live.Reason
	case saved.ExecutionID != live.ExecutionID || saved.ApprovalID != live.ApprovalID || saved.SourceRevision != live.SourceRevision || saved.SpecDigest != live.SpecDigest || saved.ArtifactDigest != live.ArtifactDigest || saved.ReceiptDigest != live.ReceiptDigest || saved.ProducerAgentID != live.ProducerAgentID || saved.ProducerTaskID != live.ProducerTaskID || saved.ProducerTaskGeneration != live.ProducerTaskGeneration || saved.ObservedHeadSHA != live.ObservedHeadSHA || !sameDeliveryTime(saved.ValidUntil, live.ValidUntil):
		out.Reason = "evidence_binding_changed"
	default:
		out.Status = "current"
		out.Reason = "observation_still_matches"
	}
	return out
}

func sameDeliveryPointer[T comparable](a, b *T) bool {
	return a == nil && b == nil || a != nil && b != nil && *a == *b
}
func sameDeliveryTime(a, b *time.Time) bool {
	return a == nil && b == nil || a != nil && b != nil && a.Equal(*b)
}
