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
	"github.com/jackc/pgx/v5"
	"github.com/multica-ai/multica/server/internal/artifact"
	"github.com/multica-ai/multica/server/internal/auth"
	obsmetrics "github.com/multica-ai/multica/server/internal/metrics"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

const deliveryEvidencePolicy = "delivery-artifact-observe-v1"

type deliveryEvidenceRequest struct {
	PolicyVersion   string  `json:"policy_version"`
	IssueRevision   int64   `json:"expected_issue_revision"`
	ProjectID       *string `json:"expected_project_id"`
	ProjectRevision *int64  `json:"expected_project_revision"`
	ApprovalID      string  `json:"approval_id"`
	SourceRevision  string  `json:"expected_source_revision"`
	SpecDigest      string  `json:"expected_spec_digest"`
	ArtifactDigest  string  `json:"expected_artifact_digest"`
}

type deliveryEvidenceResponse struct {
	Mode                   string     `json:"mode"`
	PolicyVersion          string     `json:"policy_version"`
	Verdict                string     `json:"verdict"`
	Reason                 string     `json:"reason"`
	IssueID                string     `json:"issue_id"`
	IssueRevision          int64      `json:"issue_revision"`
	ProjectID              *string    `json:"project_id"`
	ProjectRevision        *int64     `json:"project_revision"`
	ApprovalID             string     `json:"approval_id"`
	ExecutionID            string     `json:"execution_id,omitempty"`
	SourceRevision         string     `json:"source_revision"`
	ObservedHeadSHA        string     `json:"observed_head_sha"`
	RevisionProvenance     string     `json:"revision_provenance"`
	SpecDigest             string     `json:"spec_digest,omitempty"`
	ArtifactDigest         string     `json:"artifact_digest,omitempty"`
	ReceiptDigest          string     `json:"receipt_digest,omitempty"`
	ProducerAgentID        string     `json:"producer_agent_id,omitempty"`
	ProducerTaskID         string     `json:"producer_task_id,omitempty"`
	ProducerTaskGeneration int64      `json:"producer_task_generation"`
	ObservedAt             time.Time  `json:"observed_at"`
	ValidUntil             *time.Time `json:"valid_until"`
}

func deliveryHex(s string, sizes ...int) bool {
	valid := false
	for _, n := range sizes {
		valid = valid || len(s) == n
	}
	if !valid {
		return false
	}
	for _, c := range s {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}

func decodeDeliveryEvidence(w http.ResponseWriter, r *http.Request) (deliveryEvidenceRequest, bool) {
	var req deliveryEvidenceRequest
	var fields map[string]json.RawMessage
	if !decodeCollaborationWorkflowBodyLimit(w, r, &fields, 4096) {
		return req, false
	}
	req, valid := deliveryEvidenceFields(fields)
	if !valid {
		writeError(w, 400, "invalid_delivery_evidence_request")
	}
	return req, valid
}

// deliveryEvidenceFields is shared by the stateless and durable boundaries.
// Both require the same exact original observation contract.
func deliveryEvidenceFields(fields map[string]json.RawMessage) (deliveryEvidenceRequest, bool) {
	var req deliveryEvidenceRequest
	keys := []string{"policy_version", "expected_issue_revision", "expected_project_id", "expected_project_revision", "approval_id", "expected_source_revision", "expected_spec_digest", "expected_artifact_digest"}
	valid := len(fields) == len(keys)
	for _, key := range keys {
		value, ok := fields[key]
		valid = valid && ok
		if key != "expected_project_id" && key != "expected_project_revision" && bytes.Equal(value, []byte("null")) {
			valid = false
		}
	}
	raw, _ := json.Marshal(fields)
	if json.Unmarshal(raw, &req) != nil {
		valid = false
	}
	id, err := util.ParseUUID(req.ApprovalID)
	if err != nil || uuidToString(id) != req.ApprovalID || req.PolicyVersion != deliveryEvidencePolicy || req.IssueRevision < 0 || !deliveryHex(req.SourceRevision, 40, 64) || !deliveryHex(req.SpecDigest, 64) || !deliveryHex(req.ArtifactDigest, 64) {
		valid = false
	}
	if (req.ProjectID == nil) != (req.ProjectRevision == nil) {
		valid = false
	}
	if req.ProjectID != nil {
		id, err = util.ParseUUID(*req.ProjectID)
		if err != nil || uuidToString(id) != *req.ProjectID || req.ProjectRevision == nil || *req.ProjectRevision < 0 {
			valid = false
		}
	}
	return req, valid
}

// EvaluateDeliveryEvidence observes a locked snapshot. It never grants a reusable
// completion permission, writes a verdict, or invokes the artifact packager.
func (h *Handler) EvaluateDeliveryEvidence(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	p, kind, _, err := artifactPrincipal(r)
	if err != nil || kind != "human" {
		writeArtifactError(w, artifactDeny())
		return
	}
	req, ok := decodeDeliveryEvidence(w, r)
	if !ok {
		return
	}
	issue, ok := h.loadIssueForUser(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	tx, q, err := h.artifactTx(r.Context())
	if err != nil {
		writeArtifactError(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	out, err := h.evaluateDeliveryEvidence(r, tx, q, p, issue, req)
	if err != nil {
		writeArtifactError(w, err)
		return
	}
	// A failed commit has no usable observation, even though this is read-only.
	if err = tx.Commit(r.Context()); err != nil {
		writeArtifactError(w, err)
		return
	}
	writeJSON(w, 200, out)
}

func (h *Handler) evaluateDeliveryEvidence(r *http.Request, tx pgx.Tx, q *db.Queries, p auth.AuthenticatedPrincipal, initial db.Issue, req deliveryEvidenceRequest) (deliveryEvidenceResponse, error) {
	ctx := r.Context()
	out := deliveryEvidenceResponse{Mode: "observe", PolicyVersion: deliveryEvidencePolicy, Verdict: "unknown", Reason: "evidence_missing", ApprovalID: req.ApprovalID, SourceRevision: req.SourceRevision, RevisionProvenance: "database_observed_pr_head_not_remote_verified"}
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
	out.IssueID = uuidToString(issue.ID)
	out.IssueRevision = issue.Revision
	if issue.Revision != req.IssueRevision {
		return out, artifactFailure{409, "delivery_issue_revision_conflict"}
	}
	if issue.ProjectID.Valid {
		project, e := q.LockProjectForAssociationNowait(ctx, db.LockProjectForAssociationNowaitParams{ID: issue.ProjectID, WorkspaceID: issue.WorkspaceID})
		if e != nil {
			return out, e
		}
		id := uuidToString(project.ID)
		out.ProjectID = &id
		out.ProjectRevision = &project.Revision
		if req.ProjectID == nil || *req.ProjectID != id || *req.ProjectRevision != project.Revision {
			return out, artifactFailure{409, "delivery_project_revision_conflict"}
		}
	} else if req.ProjectID != nil {
		return out, artifactFailure{409, "delivery_project_revision_conflict"}
	}
	// Existing PR writers have no common issue-row/advisory lock. Row locks alone
	// miss newly inserted links. This short, NOWAIT observation fence also covers
	// absent rows and tie changes; contention returns 409 instead of waiting.
	if _, err = tx.Exec(ctx, `LOCK TABLE github_pull_request, issue_pull_request, issue_vcs_pull_request, vcs_pull_request IN SHARE MODE NOWAIT`); err != nil {
		return out, err
	}
	var heads int64
	err = tx.QueryRow(ctx, `WITH candidates AS (
 SELECT pr.head_sha, pr.state, pr.pr_updated_at FROM github_pull_request pr JOIN issue_pull_request link ON link.pull_request_id=pr.id WHERE link.issue_id=$1 AND pr.workspace_id=$2 AND NOT link.reference_only AND pr.head_sha<>''
 UNION ALL
 SELECT pr.head_sha, pr.state, pr.pr_updated_at FROM vcs_pull_request pr JOIN issue_vcs_pull_request link ON link.pull_request_id=pr.id WHERE link.issue_id=$1 AND pr.workspace_id=$2 AND NOT link.reference_only AND pr.head_sha<>''
 ), ranked AS (SELECT head_sha,dense_rank() OVER (ORDER BY (state IN ('open','draft')) DESC,pr_updated_at DESC) AS preference FROM candidates)
 SELECT COALESCE(min(head_sha),''),count(DISTINCT head_sha) FROM ranked WHERE preference=1`, issue.ID, issue.WorkspaceID).Scan(&out.ObservedHeadSHA, &heads)
	if err != nil {
		return out, err
	}
	e, err := q.GetAgentArtifactExecutionByApproval(ctx, parseUUID(req.ApprovalID))
	if errors.Is(err, pgx.ErrNoRows) {
		now, e := q.ArtifactExecutionDatabaseTime(ctx)
		if e != nil {
			return out, e
		}
		if err = artifactCredential(ctx, q, tx, p); err != nil {
			return out, err
		}
		out.ObservedAt = now.Time.UTC()
		return out, nil
	}
	if err != nil {
		return out, err
	}
	if e.WorkspaceID != issue.WorkspaceID || e.ResourceID != issue.ID {
		return out, pgx.ErrNoRows
	}
	spec, err := artifactSpec(e)
	if err != nil {
		return out, err
	}
	digest, err := artifact.RequestDigest(spec.Request, spec.Binding)
	if err != nil || digest != e.RequestDigest {
		return out, artifactConflict()
	}
	initialApproval, err := q.GetAgentApprovalRequest(ctx, e.ApprovalID)
	if err != nil {
		return out, err
	}
	// Historical producer tasks may be completed. Their scope, generation and all
	// current principals remain mandatory; the running-only claim gate is not reused.
	if _, err = q.LockTaskCapabilityPrincipals(ctx, e.TaskID); err != nil {
		return out, err
	}
	task, err := q.GetAgentTask(ctx, e.TaskID)
	if err != nil {
		return out, err
	}
	if task.AgentID != e.AgentID || task.IssueID != e.ResourceID || task.RuntimeID != e.RuntimeID || task.OriginatorUserID != e.OriginatorUserID || task.ClaimGeneration != e.TaskGeneration {
		return out, artifactConflict()
	}
	if _, err = h.artifactAuthority(r, q, tx, spec.Request, &e, uuidToString(initialApproval.DecidedBy), false, false); err != nil {
		return out, err
	}
	runtime, err := (service.RuntimeLookup{Queries: q, Metrics: h.Metrics, Source: obsmetrics.RuntimeLookupSourceTask}).Get(ctx, task.RuntimeID)
	if err != nil {
		return out, err
	}
	agent, err := q.GetAgent(ctx, e.AgentID)
	if err != nil {
		return out, err
	}
	if runtime.WorkspaceID != e.WorkspaceID || agent.RuntimeID != runtime.ID {
		return out, artifactConflict()
	}
	if _, err = artifactUsers(ctx, q, e.WorkspaceID, uuidToString(e.OriginatorUserID), uuidToString(runtime.OwnerID)); err != nil {
		return out, err
	}
	targets, err := q.LockTaskCapabilityInvocationTargets(ctx, agent.ID)
	if err != nil {
		return out, err
	}
	if !auth.AgentInvocationAllowed(agent, targets, uuidToString(e.OriginatorUserID), true, false) {
		return out, artifactDeny()
	}
	if e.InitiatorKind == "human" {
		initiator := uuidToString(e.InitiatorID)
		members, e2 := artifactUsers(ctx, q, e.WorkspaceID, initiator)
		if e2 != nil {
			return out, e2
		}
		if !artifactManager(agent, initiator, members) || !auth.AgentInvocationAllowed(agent, targets, initiator, true, false) {
			return out, artifactDeny()
		}
	}
	approval, err := q.LockAgentApprovalRequest(ctx, e.ApprovalID)
	if err != nil {
		return out, err
	}
	if approval.DecidedBy != initialApproval.DecidedBy || approval.WorkspaceID != e.WorkspaceID || approval.TaskID != e.TaskID || approval.AgentID != e.AgentID || approval.IssueID != e.ResourceID {
		return out, artifactConflict()
	}
	live, err := q.LockAgentArtifactExecutionByApproval(ctx, e.ApprovalID)
	if err != nil {
		return out, err
	}
	if live.RequestDigest != digest || live.SpecDigest != e.SpecDigest || !bytes.Equal(live.CanonicalSpec, e.CanonicalSpec) {
		return out, artifactConflict()
	}
	if _, err = artifactSpec(live); err != nil {
		return out, err
	}
	e = live
	if !service.AutonomyAtLeast(agent.AutonomyLevel, service.AutonomyOperator) {
		return out, artifactDeny()
	}
	if e.ClaimedKind.Valid {
		switch e.ClaimedKind.String {
		case "agent":
			if e.ClaimedBy != e.AgentID {
				return out, artifactConflict()
			}
		case "human":
			claimant := uuidToString(e.ClaimedBy)
			members, e2 := artifactUsers(ctx, q, e.WorkspaceID, claimant)
			if e2 != nil {
				return out, e2
			}
			if !artifactManager(agent, claimant, members) || !auth.AgentInvocationAllowed(agent, targets, claimant, true, false) {
				return out, artifactDeny()
			}
		default:
			return out, artifactConflict()
		}
	}
	now, err := q.ArtifactExecutionDatabaseTime(ctx)
	if err != nil {
		return out, err
	}
	// Validate expiry with a later database clock so a PAT/JWT cannot be valid
	// at a pre-observation check yet already expired at the reported snapshot.
	if err = artifactCredential(ctx, q, tx, p); err != nil {
		return out, err
	}
	if p.Kind == auth.CredentialJWT && !now.Time.Before(p.ExpiresAt) {
		return out, artifactDeny()
	}
	out.ObservedAt = now.Time.UTC()
	until := e.ExpiresAt.Time.UTC()
	out.ValidUntil = &until
	out.ExecutionID = uuidToString(e.ExecutionID)
	out.SourceRevision = e.SourceRevision
	out.SpecDigest = e.SpecDigest
	out.ArtifactDigest = e.OutputArtifactDigest
	out.ReceiptDigest = e.ReceiptDigest
	out.ProducerAgentID = uuidToString(e.AgentID)
	out.ProducerTaskID = uuidToString(e.TaskID)
	out.ProducerTaskGeneration = e.TaskGeneration
	switch {
	case req.SpecDigest != e.SpecDigest:
		out.Verdict = "hold"
		out.Reason = "spec_digest_mismatch"
	case req.SourceRevision != e.SourceRevision:
		out.Verdict = "hold"
		out.Reason = "source_revision_mismatch"
	case !now.Time.Before(e.ExpiresAt.Time):
		out.Verdict = "hold"
		out.Reason = "evidence_expired"
	case task.Status == "cancelled" || task.Status == "failed":
		out.Verdict = "hold"
		out.Reason = "producer_task_failed"
	case !e.ResultStatus.Valid:
		out.Reason = "evidence_missing"
	case e.ResultStatus.String == "unknown":
		out.Reason = "execution_unknown"
	case e.ResultStatus.String == "failed":
		out.Verdict = "hold"
		out.Reason = "execution_failed"
	case approval.Status != "executed" || !approval.DecidedBy.Valid || !approval.DecidedAt.Valid || !approval.ExecutedAt.Valid || !deliveryReceiptValid(e):
		out.Verdict = "hold"
		out.Reason = "receipt_invalid"
	case req.ArtifactDigest != e.OutputArtifactDigest:
		out.Verdict = "hold"
		out.Reason = "artifact_digest_mismatch"
	case heads == 0:
		out.Reason = "source_head_missing"
	case heads != 1:
		out.ObservedHeadSHA = ""
		out.Reason = "source_head_ambiguous"
	case out.ObservedHeadSHA != req.SourceRevision:
		out.Verdict = "hold"
		out.Reason = "source_revision_mismatch"
	default:
		out.Verdict = "pass"
		out.Reason = "evidence_verified"
	}
	return out, nil
}

func deliveryReceiptValid(e db.AgentArtifactExecution) bool {
	if e.ResultStatus.String != "succeeded" || !e.FinishedAt.Valid || !e.ClaimedAt.Valid || !e.ClaimNonce.Valid || e.OutputArtifactID != e.ExecutionID || !e.OutputSize.Valid || e.OutputSize.Int64 < 0 || !deliveryHex(e.OutputArtifactDigest, 64) {
		return false
	}
	// finalizeArtifact marshals a map. Re-encoding with UseNumber restores its
	// canonical bytes from JSONB without rounding integers or hashing JSONB text.
	var receipt map[string]any
	decoder := json.NewDecoder(bytes.NewReader(e.ReceiptJson))
	decoder.UseNumber()
	if decoder.Decode(&receipt) != nil {
		return false
	}
	raw, err := json.Marshal(receipt)
	if err != nil {
		return false
	}
	sum := sha256.Sum256(raw)
	if hex.EncodeToString(sum[:]) != e.ReceiptDigest {
		return false
	}
	finished, ok := receipt["finished_at"].(string)
	if !ok {
		return false
	}
	finishedAt, err := time.Parse(time.RFC3339Nano, finished)
	if err != nil || finishedAt.Before(e.ClaimedAt.Time) || finishedAt.After(e.FinishedAt.Time) {
		return false
	}
	expected := map[string]any{"finished_at": finishedAt.UTC(), "execution_id": uuidToString(e.ExecutionID), "approval_id": uuidToString(e.ApprovalID), "spec_digest": e.SpecDigest, "actor_kind": e.ClaimedKind.String, "actor_id": uuidToString(e.ClaimedBy), "resource_id": uuidToString(e.ResourceID), "action": e.Action, "declared_revision": e.SourceRevision, "input_digest": e.InputManifestDigest, "environment": e.TargetEnvironment, "policy": e.PolicyVersion, "result": e.ResultStatus.String, "code": e.ResultCode, "artifact_id": uuidToString(e.OutputArtifactID), "output_digest": e.OutputArtifactDigest, "output_bytes": e.OutputSize.Int64, "claimed_at": e.ClaimedAt.Time.UTC()}
	canonical, err := json.Marshal(expected)
	return err == nil && bytes.Equal(raw, canonical)
}
