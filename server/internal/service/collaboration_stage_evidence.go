package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

const stageCIHorizon = 30 * time.Minute
const stageApprovalHorizon = 168 * time.Hour

// Private frozen source identity, never an HTTP DTO. CI is intentionally not
// part of candidate identity: each decision binds its exact observed check set.
type stageCodeSource struct {
	IntegrationID       string               `json:"integration_id"`
	RepoOwner           string               `json:"repo_owner"`
	RepoName            string               `json:"repo_name"`
	ArtifactObservation *DeliveryObservation `json:"artifact_observation,omitempty"`
}

type stageCodeContent struct {
	Baseline          CollaborationStageCodeBaseline `json:"baseline"`
	ImplementerUserID string                         `json:"implementer_user_id"`
	RoleRevision      int64                          `json:"role_revision"`
	Source            stageCodeSource                `json:"source"`
}

func stageJSONDigest(value any) ([]byte, string, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, "", err
	}
	hash := sha256.Sum256(raw)
	return raw, hex.EncodeToString(hash[:]), nil
}

func decodeStageCode(candidate db.CollaborationStageCandidate) (stageCodeContent, error) {
	var content stageCodeContent
	if candidate.Kind != "code" || json.Unmarshal(candidate.Content, &content) != nil ||
		!stageUUID(content.Baseline.ChildIssueID) || !stagePositiveVersion(content.Baseline.ChildSemanticVersion) ||
		!stageDigest(content.Baseline.RequirementDigest) || !stageUUID(content.ImplementerUserID) || content.RoleRevision < 1 ||
		!stageUUID(content.Baseline.PullRequest.PullRequestID) || !stageHeadSHA(content.Baseline.PullRequest.HeadSHA) ||
		!stagePositiveVersion(content.Baseline.PullRequest.TrustRevision) || !stageUUID(content.Source.IntegrationID) ||
		(content.Baseline.PullRequest.Provider != "github" && content.Baseline.PullRequest.Provider != "vcs") {
		return content, errors.New("invalid retained stage code candidate")
	}
	if a := content.Baseline.Artifact; a != nil && (!stageUUID(a.ApprovalID) || !stageDigest(a.ArtifactDigest)) {
		return content, errors.New("invalid retained stage artifact")
	}
	return content, nil
}

func lockStageCodeEvidence(ctx context.Context, tx pgx.Tx) error {
	// Same PR/link fence as M1, extended to current checks and integration trust.
	// NOWAIT preserves root-before-child workflow ordering without a reverse wait.
	_, err := tx.Exec(ctx, `LOCK TABLE github_pull_request, issue_pull_request, issue_vcs_pull_request, vcs_pull_request,
 github_pull_request_check_run, github_installation, vcs_connection IN SHARE MODE NOWAIT`)
	return err
}

func loadStageCodeSource(ctx context.Context, q *db.Queries, workspace, child pgtype.UUID, ref CollaborationStageCodeRef, now time.Time) (CollaborationStageCodePullRequest, stageCodeSource, CollaborationStageObservedCI, error) {
	pr := CollaborationStageCodePullRequest{Provider: ref.Provider, PullRequestID: util.UUIDToString(ref.PullRequestID)}
	source := stageCodeSource{}
	ci := CollaborationStageObservedCI{State: "unsupported"}
	if ref.Provider == "vcs" {
		row, err := q.GetCollaborationStageVCSSource(ctx, db.GetCollaborationStageVCSSourceParams{ID: ref.PullRequestID, WorkspaceID: workspace, IssueID: child})
		if err != nil {
			return pr, source, ci, err
		}
		pr.HeadSHA, pr.TrustRevision = row.HeadSha, stageVersion(row.TrustRevision)
		source.IntegrationID, source.RepoOwner, source.RepoName = util.UUIDToString(row.ConnectionID), row.RepoOwner, row.RepoName
		return pr, source, ci, nil
	}
	row, err := q.GetCollaborationStageGitHubSource(ctx, db.GetCollaborationStageGitHubSourceParams{ID: ref.PullRequestID, WorkspaceID: workspace, IssueID: child})
	if err != nil {
		return pr, source, ci, err
	}
	pr.HeadSHA, pr.TrustRevision = row.HeadSha, stageVersion(row.TrustRevision)
	source.IntegrationID, source.RepoOwner, source.RepoName = util.UUIDToString(row.IntegrationID), row.RepoOwner, row.RepoName
	checks, err := q.ListCollaborationStageGitHubChecks(ctx, row.ID)
	if err != nil {
		return pr, source, ci, err
	}
	ci, err = stageGitHubCI(row, checks, now)
	return pr, source, ci, err
}

func stageGitHubCI(pr db.GetCollaborationStageGitHubSourceRow, checks []db.GithubPullRequestCheckRun, now time.Time) (CollaborationStageObservedCI, error) {
	ci := CollaborationStageObservedCI{State: "missing"}
	if !pr.SnapshotComplete || !pr.SnapshotFetchedAt.Valid || pr.SnapshotHeadSha != pr.HeadSha || len(checks) == 0 {
		return ci, nil
	}
	head, observed := pr.SnapshotHeadSha, pr.SnapshotFetchedAt.Time.UTC()
	ci.HeadSHA, ci.ObservedAt, ci.CheckCount = &head, &observed, len(checks)
	type checkIdentity struct {
		Name, Status, Conclusion string
		Legacy                   bool
	}
	identities := make([]checkIdentity, 0, len(checks))
	passed := true
	for index, check := range checks {
		if check.HeadSha != head || check.Ordinal != int32(index) || check.Name == "" {
			return CollaborationStageObservedCI{State: "missing"}, nil
		}
		identities = append(identities, checkIdentity{check.Name, check.Status, check.Conclusion.String, check.IsStatusContext})
		passed = passed && check.Status == "completed" && check.Conclusion.Valid && check.Conclusion.String == "success"
	}
	sort.Slice(identities, func(i, j int) bool {
		a, b := identities[i], identities[j]
		if a.Name != b.Name {
			return a.Name < b.Name
		}
		if a.Legacy != b.Legacy {
			return !a.Legacy
		}
		if a.Status != b.Status {
			return a.Status < b.Status
		}
		return a.Conclusion < b.Conclusion
	})
	_, digest, err := stageJSONDigest(struct {
		Provider, PR, Head, Integration string
		Trust                           int64
		Checks                          []checkIdentity
	}{"github", util.UUIDToString(pr.ID), head, util.UUIDToString(pr.IntegrationID), pr.TrustRevision, identities})
	if err != nil {
		return ci, err
	}
	ci.CIDigest = &digest
	switch {
	case observed.After(now) || !now.Before(observed.Add(stageCIHorizon)):
		ci.State = "stale"
	case !passed:
		ci.State = "failed"
	default:
		ci.State = "passed"
	}
	return ci, nil
}

func stageCIRefusal(ci CollaborationStageObservedCI) string {
	switch ci.State {
	case "passed":
		return ""
	case "failed":
		return "stage_ci_failed"
	case "stale":
		return "stage_ci_stale"
	case "unsupported":
		return "stage_ci_unsupported"
	case "unavailable":
		return "stage_evidence_unavailable"
	default:
		return "stage_ci_missing"
	}
}

func stageCodeCurrent(ctx context.Context, q *db.Queries, access CollaborationIssueAccess, state *collaborationStageState, candidate db.CollaborationStageCandidate, content stageCodeContent, now time.Time) (CollaborationStageObserved, string, error) {
	observed := CollaborationStageObserved{PullRequest: CollaborationStageObservedPR{State: "unavailable"}, CI: CollaborationStageObservedCI{State: "unavailable"}}
	if candidate.LifecycleEpoch != access.LifecycleEpoch || content.RoleRevision != state.task.RoleRevision ||
		content.Baseline.RequirementDigest != collaboration.StageChildContentDigest(access.Issue.Title, access.Issue.Description.String) {
		return observed, "stage_candidate_stale", nil
	}
	childID := util.MustParseUUID(content.Baseline.ChildIssueID)
	child, err := q.GetIssue(ctx, childID)
	if errors.Is(err, pgx.ErrNoRows) {
		return observed, "stage_evidence_unavailable", nil
	}
	if err != nil {
		return observed, "", err
	}
	mapping, err := q.GetCollaborationStageChild(ctx, childID)
	if errors.Is(err, pgx.ErrNoRows) {
		return observed, "stage_candidate_stale", nil
	}
	if err != nil {
		return observed, "", err
	}
	if child.WorkspaceID != access.Issue.WorkspaceID || mapping.RootIssueID != access.Issue.ID || mapping.LifecycleEpoch != access.LifecycleEpoch || stageVersion(mapping.SemanticVersion) != content.Baseline.ChildSemanticVersion {
		return observed, "stage_candidate_stale", nil
	}
	ref := content.Baseline.PullRequest
	pr, source, ci, err := loadStageCodeSource(ctx, q, child.WorkspaceID, child.ID, CollaborationStageCodeRef{Provider: ref.Provider, PullRequestID: util.MustParseUUID(ref.PullRequestID), HeadSHA: ref.HeadSHA}, now)
	if errors.Is(err, pgx.ErrNoRows) {
		observed.PullRequest.State = "unlinked"
		return observed, "stage_pull_request_unavailable", nil
	}
	if err != nil {
		return observed, "", err
	}
	observed.PullRequest = CollaborationStageObservedPR{State: "current", HeadSHA: &pr.HeadSHA}
	observed.CI = ci
	if pr.HeadSHA != ref.HeadSHA {
		observed.PullRequest.State = "head_changed"
		return observed, "stage_head_changed", nil
	}
	if pr.TrustRevision != ref.TrustRevision || source.IntegrationID != content.Source.IntegrationID || source.RepoOwner != content.Source.RepoOwner || source.RepoName != content.Source.RepoName {
		observed.PullRequest.State = "trust_changed"
		return observed, "stage_trust_changed", nil
	}
	if artifact := content.Baseline.Artifact; artifact != nil {
		execution, e := q.GetAgentArtifactExecutionByApproval(ctx, util.MustParseUUID(artifact.ApprovalID))
		if errors.Is(e, pgx.ErrNoRows) {
			return observed, "stage_evidence_unavailable", nil
		}
		if e != nil {
			return observed, "", e
		}
		frozen := content.Source.ArtifactObservation
		if frozen == nil || execution.ResourceID != child.ID || execution.WorkspaceID != child.WorkspaceID || execution.ResultStatus.String != "succeeded" ||
			execution.OutputArtifactDigest != artifact.ArtifactDigest || util.UUIDToString(execution.ExecutionID) != frozen.ExecutionID || execution.ReceiptDigest != frozen.ReceiptDigest || execution.SpecDigest != frozen.SpecDigest {
			return observed, "stage_evidence_changed", nil
		}
	}
	return observed, "", nil
}

func stageArtifactObservation(ctx context.Context, tx pgx.Tx, child db.Issue, approval pgtype.UUID, head string) (DeliveryObservation, error) {
	return stageArtifactObservationAs(ctx, tx, child, approval, head, completionRequestCredential(ctx))
}

func stageArtifactObservationAs(ctx context.Context, tx pgx.Tx, child db.Issue, approval pgtype.UUID, head string, credential ArtifactCredentialSnapshot) (DeliveryObservation, error) {
	q := db.New(tx)
	execution, err := q.GetAgentArtifactExecutionByApproval(ctx, approval)
	if err != nil {
		return DeliveryObservation{}, err
	}
	request := DeliveryEvidenceRequest{PolicyVersion: DeliveryEvidencePolicy, IssueRevision: child.Revision, ApprovalID: util.UUIDToString(approval), SourceRevision: head, SpecDigest: execution.SpecDigest, ArtifactDigest: execution.OutputArtifactDigest}
	if child.ProjectID.Valid {
		project, err := q.GetProjectInWorkspace(ctx, db.GetProjectInWorkspaceParams{ID: child.ProjectID, WorkspaceID: child.WorkspaceID})
		if err != nil {
			return DeliveryObservation{}, err
		}
		id, revision := util.UUIDToString(project.ID), project.Revision
		request.ProjectID, request.ProjectRevision = &id, &revision
	}
	access := ArtifactAccess{Credential: credential, RequestedWorkspaceID: util.UUIDToString(child.WorkspaceID)}
	if _, readProjection := tx.(collaborationStageEvidenceReadTx); readProjection {
		return evaluateDeliveryEvidenceReadInTx(ctx, tx, access, child, request)
	}
	return EvaluateDeliveryEvidenceInTx(ctx, tx, access, child, request)
}

func checkStageArtifactCurrent(ctx context.Context, tx pgx.Tx, content stageCodeContent) (string, error) {
	a := content.Baseline.Artifact
	if a == nil {
		return "", nil
	}
	child, err := db.New(tx).GetIssue(ctx, util.MustParseUUID(content.Baseline.ChildIssueID))
	if errors.Is(err, pgx.ErrNoRows) {
		return "stage_evidence_unavailable", nil
	}
	if err != nil {
		return "", err
	}
	observation, err := stageArtifactObservation(ctx, tx, child, util.MustParseUUID(a.ApprovalID), content.Baseline.PullRequest.HeadSHA)
	if stageArtifactUnavailable(err) {
		return "stage_evidence_unavailable", nil
	}
	if err != nil {
		return "", err
	}
	frozen := content.Source.ArtifactObservation
	if observation.Verdict != "pass" || frozen == nil || observation.ArtifactDigest != a.ArtifactDigest || observation.ExecutionID != frozen.ExecutionID || observation.ReceiptDigest != frozen.ReceiptDigest || observation.ProducerTaskGeneration != frozen.ProducerTaskGeneration {
		return "stage_evidence_changed", nil
	}
	return "", nil
}
