package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/multica-ai/multica/server/internal/artifact"
	"github.com/multica-ai/multica/server/internal/auth"
	obsmetrics "github.com/multica-ai/multica/server/internal/metrics"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// DeliveryEvidencePolicy identifies the unchanged fixed observation algorithm.
const DeliveryEvidencePolicy = "delivery-artifact-observe-v1"

// DeliveryEvidenceRequest carries the exact caller-observed resource and artifact bindings.
type DeliveryEvidenceRequest struct {
	PolicyVersion   string  `json:"policy_version"`
	IssueRevision   int64   `json:"expected_issue_revision"`
	ProjectID       *string `json:"expected_project_id"`
	ProjectRevision *int64  `json:"expected_project_revision"`
	ApprovalID      string  `json:"approval_id"`
	SourceRevision  string  `json:"expected_source_revision"`
	SpecDigest      string  `json:"expected_spec_digest"`
	ArtifactDigest  string  `json:"expected_artifact_digest"`
}

// DeliveryObservation is a snapshot of current facts, never a completion permit.
type DeliveryObservation struct {
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

// DeliveryHex checks the canonical lowercase hexadecimal digest/revision shape.
func DeliveryHex(s string, sizes ...int) bool {
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

// EvaluateDeliveryEvidenceInTx observes current locked facts using the caller
// transaction. It neither owns that transaction nor grants completion authority.
func EvaluateDeliveryEvidenceInTx(ctx context.Context, tx pgx.Tx, access ArtifactAccess, initial db.Issue, req DeliveryEvidenceRequest) (DeliveryObservation, error) {
	return evaluateDeliveryEvidenceInTx(ctx, tx, access, initial, req, false)
}

func evaluateDeliveryEvidenceReadInTx(ctx context.Context, tx pgx.Tx, access ArtifactAccess, initial db.Issue, req DeliveryEvidenceRequest) (DeliveryObservation, error) {
	return evaluateDeliveryEvidenceInTx(ctx, tx, access, initial, req, true)
}

func evaluateDeliveryEvidenceInTx(ctx context.Context, tx pgx.Tx, access ArtifactAccess, initial db.Issue, req DeliveryEvidenceRequest, readProjection bool) (DeliveryObservation, error) {
	q := db.New(tx)
	p := access.Credential.Principal
	out := DeliveryObservation{Mode: "observe", PolicyVersion: DeliveryEvidencePolicy, Verdict: "unknown", Reason: "evidence_missing", ApprovalID: req.ApprovalID, SourceRevision: req.SourceRevision, RevisionProvenance: "database_observed_pr_head_not_remote_verified"}
	if err := LockArtifactWorkspace(ctx, q, initial.WorkspaceID); err != nil {
		return out, err
	}
	var issue db.Issue
	var err error
	if readProjection {
		issue, err = q.LockDeliveryEvidenceIssueRead(ctx, db.LockDeliveryEvidenceIssueReadParams{ID: initial.ID, WorkspaceID: initial.WorkspaceID})
	} else {
		issue, err = q.LockLifecycleIssue(ctx, db.LockLifecycleIssueParams{ID: initial.ID, WorkspaceID: initial.WorkspaceID})
	}
	if err != nil {
		return out, err
	}
	if _, err = LockArtifactUsers(ctx, q, issue.WorkspaceID, p.UserID); err != nil {
		return out, err
	}
	if err = CheckArtifactCredentialInTx(ctx, tx, access.Credential); err != nil {
		return out, err
	}
	out.IssueID = util.UUIDToString(issue.ID)
	out.IssueRevision = issue.Revision
	if issue.Revision != req.IssueRevision {
		return out, ArtifactError{409, "delivery_issue_revision_conflict"}
	}
	if issue.ProjectID.Valid {
		project, e := q.LockProjectForAssociationNowait(ctx, db.LockProjectForAssociationNowaitParams{ID: issue.ProjectID, WorkspaceID: issue.WorkspaceID})
		if e != nil {
			return out, e
		}
		id := util.UUIDToString(project.ID)
		out.ProjectID = &id
		out.ProjectRevision = &project.Revision
		if req.ProjectID == nil || *req.ProjectID != id || *req.ProjectRevision != project.Revision {
			return out, ArtifactError{409, "delivery_project_revision_conflict"}
		}
	} else if req.ProjectID != nil {
		return out, ArtifactError{409, "delivery_project_revision_conflict"}
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
	approvalID, err := util.ParseUUID(req.ApprovalID)
	if err != nil {
		return out, artifactConflict()
	}
	e, err := q.GetAgentArtifactExecutionByApproval(ctx, approvalID)
	if errors.Is(err, pgx.ErrNoRows) {
		now, e := q.ArtifactExecutionDatabaseTime(ctx)
		if e != nil {
			return out, e
		}
		if err = CheckArtifactCredentialInTx(ctx, tx, access.Credential); err != nil {
			return out, err
		}
		out.ObservedAt = now.Time.UTC()
		return out, nil
	}
	if err != nil {
		return out, err
	}
	e, task, approval, err := lockDeliveryArtifactWithReadMode(ctx, tx, access, issue, e, readProjection)
	if err != nil {
		return out, err
	}
	now, err := q.ArtifactExecutionDatabaseTime(ctx)
	if err != nil {
		return out, err
	}
	// Validate expiry with a later database clock so a PAT/JWT cannot be valid
	// at a pre-observation check yet already expired at the reported snapshot.
	if err = CheckArtifactCredentialInTx(ctx, tx, access.Credential); err != nil {
		return out, err
	}
	if p.Kind == auth.CredentialJWT && !now.Time.Before(p.ExpiresAt) {
		return out, artifactDeny()
	}
	out.ObservedAt = now.Time.UTC()
	until := e.ExpiresAt.Time.UTC()
	out.ValidUntil = &until
	out.ExecutionID = util.UUIDToString(e.ExecutionID)
	out.SourceRevision = e.SourceRevision
	out.SpecDigest = e.SpecDigest
	out.ArtifactDigest = e.OutputArtifactDigest
	out.ReceiptDigest = e.ReceiptDigest
	out.ProducerAgentID = util.UUIDToString(e.AgentID)
	out.ProducerTaskID = util.UUIDToString(e.TaskID)
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

// lockDeliveryArtifactInTx shares the observer's retained/current artifact
// authority with candidate discovery. It does not observe PRs or create evidence.
func lockDeliveryArtifactInTx(ctx context.Context, tx pgx.Tx, access ArtifactAccess, issue db.Issue, execution db.AgentArtifactExecution) (e db.AgentArtifactExecution, task db.AgentTaskQueue, approval db.AgentApprovalRequest, err error) {
	return lockDeliveryArtifactWithReadMode(ctx, tx, access, issue, execution, false)
}

func readDeliveryArtifactInTx(ctx context.Context, tx pgx.Tx, access ArtifactAccess, issue db.Issue, execution db.AgentArtifactExecution) (e db.AgentArtifactExecution, task db.AgentTaskQueue, approval db.AgentApprovalRequest, err error) {
	return lockDeliveryArtifactWithReadMode(ctx, tx, access, issue, execution, true)
}

func lockDeliveryArtifactWithReadMode(ctx context.Context, tx pgx.Tx, access ArtifactAccess, issue db.Issue, execution db.AgentArtifactExecution, readProjection bool) (e db.AgentArtifactExecution, task db.AgentTaskQueue, approval db.AgentApprovalRequest, err error) {
	e = execution
	q := db.New(tx)
	if e.WorkspaceID != issue.WorkspaceID || e.ResourceID != issue.ID {
		return e, task, approval, pgx.ErrNoRows
	}
	spec, err := ReadArtifactSpec(e)
	if err != nil {
		return e, task, approval, err
	}
	digest, err := artifact.RequestDigest(spec.Request, spec.Binding)
	if err != nil || digest != e.RequestDigest {
		return e, task, approval, artifactConflict()
	}
	initialApproval, err := q.GetAgentApprovalRequest(ctx, e.ApprovalID)
	if err != nil {
		return e, task, approval, err
	}
	// Historical producer tasks may be completed. Their scope, generation and all
	// current principals remain mandatory; the running-only claim gate is not reused.
	if readProjection {
		_, err = q.LockDeliveryArtifactPrincipalsRead(ctx, e.TaskID)
	} else {
		_, err = q.LockTaskCapabilityPrincipals(ctx, e.TaskID)
	}
	if err != nil {
		return e, task, approval, err
	}
	task, err = q.GetAgentTask(ctx, e.TaskID)
	if err != nil {
		return e, task, approval, err
	}
	if task.AgentID != e.AgentID || task.IssueID != e.ResourceID || task.RuntimeID != e.RuntimeID || task.OriginatorUserID != e.OriginatorUserID || task.ClaimGeneration != e.TaskGeneration {
		return e, task, approval, artifactConflict()
	}
	if _, err = CheckArtifactAuthorityInTx(ctx, tx, access, spec.Request, &e, util.UUIDToString(initialApproval.DecidedBy), false, false); err != nil {
		return e, task, approval, err
	}
	runtime, err := (RuntimeLookup{Queries: q, Metrics: access.Metrics, Source: obsmetrics.RuntimeLookupSourceTask}).Get(ctx, task.RuntimeID)
	if err != nil {
		return e, task, approval, err
	}
	agent, err := q.GetAgent(ctx, e.AgentID)
	if err != nil {
		return e, task, approval, err
	}
	if runtime.WorkspaceID != e.WorkspaceID || agent.RuntimeID != runtime.ID {
		return e, task, approval, artifactConflict()
	}
	if _, err = LockArtifactUsers(ctx, q, e.WorkspaceID, util.UUIDToString(e.OriginatorUserID), util.UUIDToString(runtime.OwnerID)); err != nil {
		return e, task, approval, err
	}
	targets, err := q.LockTaskCapabilityInvocationTargets(ctx, agent.ID)
	if err != nil {
		return e, task, approval, err
	}
	if !auth.AgentInvocationAllowed(agent, targets, util.UUIDToString(e.OriginatorUserID), true, false) {
		return e, task, approval, artifactDeny()
	}
	if e.InitiatorKind == "human" {
		initiator := util.UUIDToString(e.InitiatorID)
		members, e2 := LockArtifactUsers(ctx, q, e.WorkspaceID, initiator)
		if e2 != nil {
			return e, task, approval, e2
		}
		if !artifactManager(agent, initiator, members) || !auth.AgentInvocationAllowed(agent, targets, initiator, true, false) {
			return e, task, approval, artifactDeny()
		}
	}
	if readProjection {
		approval, err = q.LockDeliveryArtifactApprovalRead(ctx, e.ApprovalID)
	} else {
		approval, err = q.LockAgentApprovalRequest(ctx, e.ApprovalID)
	}
	if err != nil {
		return e, task, approval, err
	}
	if approval.DecidedBy != initialApproval.DecidedBy || approval.WorkspaceID != e.WorkspaceID || approval.TaskID != e.TaskID || approval.AgentID != e.AgentID || approval.IssueID != e.ResourceID {
		return e, task, approval, artifactConflict()
	}
	var live db.AgentArtifactExecution
	if readProjection {
		live, err = q.LockDeliveryArtifactExecutionRead(ctx, e.ApprovalID)
	} else {
		live, err = q.LockAgentArtifactExecutionByApproval(ctx, e.ApprovalID)
	}
	if err != nil {
		return e, task, approval, err
	}
	if live.RequestDigest != digest || live.SpecDigest != e.SpecDigest || !bytes.Equal(live.CanonicalSpec, e.CanonicalSpec) {
		return e, task, approval, artifactConflict()
	}
	if _, err = ReadArtifactSpec(live); err != nil {
		return e, task, approval, err
	}
	e = live
	if !AutonomyAtLeast(agent.AutonomyLevel, AutonomyOperator) {
		return e, task, approval, artifactDeny()
	}
	if e.ClaimedKind.Valid {
		switch e.ClaimedKind.String {
		case "agent":
			if e.ClaimedBy != e.AgentID {
				return e, task, approval, artifactConflict()
			}
		case "human":
			claimant := util.UUIDToString(e.ClaimedBy)
			members, e2 := LockArtifactUsers(ctx, q, e.WorkspaceID, claimant)
			if e2 != nil {
				return e, task, approval, e2
			}
			if !artifactManager(agent, claimant, members) || !auth.AgentInvocationAllowed(agent, targets, claimant, true, false) {
				return e, task, approval, artifactDeny()
			}
		default:
			return e, task, approval, artifactConflict()
		}
	}
	return e, task, approval, nil
}

func deliveryReceiptValid(e db.AgentArtifactExecution) bool {
	if e.ResultStatus.String != "succeeded" || !e.FinishedAt.Valid || !e.ClaimedAt.Valid || !e.ClaimNonce.Valid || e.OutputArtifactID != e.ExecutionID || !e.OutputSize.Valid || e.OutputSize.Int64 < 0 || !DeliveryHex(e.OutputArtifactDigest, 64) {
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
	expected := map[string]any{"finished_at": finishedAt.UTC(), "execution_id": util.UUIDToString(e.ExecutionID), "approval_id": util.UUIDToString(e.ApprovalID), "spec_digest": e.SpecDigest, "actor_kind": e.ClaimedKind.String, "actor_id": util.UUIDToString(e.ClaimedBy), "resource_id": util.UUIDToString(e.ResourceID), "action": e.Action, "declared_revision": e.SourceRevision, "input_digest": e.InputManifestDigest, "environment": e.TargetEnvironment, "policy": e.PolicyVersion, "result": e.ResultStatus.String, "code": e.ResultCode, "artifact_id": util.UUIDToString(e.OutputArtifactID), "output_digest": e.OutputArtifactDigest, "output_bytes": e.OutputSize.Int64, "claimed_at": e.ClaimedAt.Time.UTC()}
	canonical, err := json.Marshal(expected)
	return err == nil && bytes.Equal(raw, canonical)
}
