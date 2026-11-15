package service

import (
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

type CollaborationStageArtifactSelection struct{ ApprovalID pgtype.UUID }

type CollaborationStageCodeParams struct {
	CollaborationWorkflowRequest
	ExpectedPolicyRevision int64
	ExpectedRoleRevision   int64
	ChildIssueID           pgtype.UUID
	Supersedes             *CollaborationStageCandidateRef
	PullRequest            CollaborationStageCodeRef
	Artifact               *CollaborationStageArtifactSelection
}

type CollaborationStageBuild struct {
	ArtifactDigest string `json:"artifact_digest"`
	Environment    string `json:"environment"`
	TargetDigest   string `json:"target_digest,omitempty"`
}

type CollaborationStageReviewEvidence struct {
	CIDigest             *string                        `json:"ci_digest"`
	TestReportID         *string                        `json:"test_report_id"`
	Build                *CollaborationStageBuild       `json:"build"`
	DeploymentID         *string                        `json:"deployment_id"`
	RollbackTargetDigest *string                        `json:"rollback_target_digest"`
	RollbackObservation  *CollaborationStageRollbackRef `json:"rollback_observation,omitempty"`
}

type CollaborationStageReviewParams struct {
	CollaborationWorkflowRequest
	ExpectedPolicyRevision int64
	ExpectedRoleRevision   int64
	ExpectedStageRevision  int64
	Step                   string
	Decision               string
	Candidate              CollaborationStageCandidateRef
	Evidence               CollaborationStageReviewEvidence
	DependsOn              []string
	Reason                 string
	ExpiresAt              *time.Time
}

type CollaborationStageRevocationParams struct {
	CollaborationWorkflowRequest
	ExpectedPolicyRevision int64
	ExpectedRoleRevision   int64
	DecisionID             pgtype.UUID
	Reason                 string
}

type CollaborationStageAdvanceParams struct {
	CollaborationWorkflowRequest
	ExpectedPolicyRevision int64
	ExpectedRoleRevision   int64
	ExpectedStageRevision  int64
	TargetStage            string
	Candidate              CollaborationStageCandidateRef
	DecisionIDs            []string
}

type CollaborationStageCodePullRequest struct {
	Provider      string `json:"provider"`
	PullRequestID string `json:"pull_request_id"`
	HeadSHA       string `json:"head_sha"`
	TrustRevision string `json:"trust_revision"`
}

type CollaborationStageArtifactBaseline struct {
	ApprovalID     string `json:"approval_id"`
	ArtifactDigest string `json:"artifact_digest"`
}

type CollaborationStageCodeBaseline struct {
	RequirementDigest    string                              `json:"requirement_digest"`
	ChildIssueID         string                              `json:"child_issue_id"`
	ChildSemanticVersion string                              `json:"child_semantic_version"`
	PullRequest          CollaborationStageCodePullRequest   `json:"pull_request"`
	Artifact             *CollaborationStageArtifactBaseline `json:"artifact"`
}

type CollaborationStageCodeCandidateView struct {
	CandidateID       string                          `json:"candidate_id"`
	Version           string                          `json:"version"`
	ContentDigest     string                          `json:"content_digest"`
	ParentCandidateID *string                         `json:"parent_candidate_id"`
	LifecycleEpoch    string                          `json:"lifecycle_epoch"`
	State             string                          `json:"state"`
	SubmittedBy       string                          `json:"submitted_by"`
	ImplementerUserID string                          `json:"implementer_user_id"`
	RoleRevision      string                          `json:"role_revision"`
	CreatedAt         time.Time                       `json:"created_at"`
	Baseline          *CollaborationStageCodeBaseline `json:"baseline"`
}

type CollaborationStageCodeCandidateSummary struct {
	CandidateID   string `json:"candidate_id"`
	Version       string `json:"version"`
	ContentDigest string `json:"content_digest"`
	State         string `json:"state"`
}

type CollaborationStageCodeCandidates struct {
	Items     []CollaborationStageCodeCandidateSummary `json:"items"`
	Truncated bool                                     `json:"truncated"`
}

type CollaborationStageObservedPR struct {
	State   string  `json:"state"`
	HeadSHA *string `json:"head_sha"`
}

type CollaborationStageObservedCI struct {
	State      string     `json:"state"`
	CIDigest   *string    `json:"ci_digest"`
	HeadSHA    *string    `json:"head_sha"`
	ObservedAt *time.Time `json:"observed_at"`
	CheckCount int        `json:"check_count"`
}

type CollaborationStageObserved struct {
	PullRequest CollaborationStageObservedPR `json:"pull_request"`
	CI          CollaborationStageObservedCI `json:"ci"`
}

type CollaborationStageBlocker struct {
	Code         string `json:"code"`
	Stage        string `json:"stage"`
	CandidateID  string `json:"candidate_id,omitempty"`
	DecisionID   string `json:"decision_id,omitempty"`
	ChildIssueID string `json:"child_issue_id,omitempty"`
}

type CollaborationStageSignable struct {
	Step         string                      `json:"step"`
	Allowed      bool                        `json:"allowed"`
	MaxExpiresAt *time.Time                  `json:"max_expires_at"`
	Blockers     []CollaborationStageBlocker `json:"blockers"`
}

type CollaborationStageRevocationView struct {
	RevocationID string    `json:"revocation_id"`
	ActorUserID  string    `json:"actor_user_id"`
	CreatedAt    time.Time `json:"created_at"`
	Reason       string    `json:"reason"`
}

type CollaborationStageReviewView struct {
	DecisionID     string                            `json:"decision_id"`
	Step           string                            `json:"step"`
	Decision       string                            `json:"decision"`
	State          string                            `json:"state"`
	ActorUserID    string                            `json:"actor_user_id"`
	Role           string                            `json:"role"`
	RoleRevision   string                            `json:"role_revision"`
	PolicyRevision string                            `json:"policy_revision"`
	CreatedAt      time.Time                         `json:"created_at"`
	ExpiresAt      *time.Time                        `json:"expires_at"`
	Reason         string                            `json:"reason"`
	Evidence       CollaborationStageReviewEvidence  `json:"evidence"`
	DependsOn      []string                          `json:"depends_on"`
	Revocation     *CollaborationStageRevocationView `json:"revocation"`
}

type CollaborationStageCodeEvidenceView struct {
	ProtocolVersion      int                                         `json:"protocol_version"`
	IssueID              string                                      `json:"issue_id"`
	CheckedAt            time.Time                                   `json:"checked_at"`
	Candidate            CollaborationStageCodeCandidateView         `json:"candidate"`
	Observed             CollaborationStageObserved                  `json:"observed"`
	TestReports          []CollaborationStageTestReportSummary       `json:"test_reports"`
	Decisions            []CollaborationStageReviewView              `json:"decisions"`
	ExecutionFacts       []CollaborationStageExecutionFactView       `json:"execution_facts"`
	Signable             []CollaborationStageSignable                `json:"signable"`
	BlockedEvidence      int                                         `json:"blocked_evidence"`
	ExecutionTargets     []CollaborationStageExecutionTargetView     `json:"execution_targets"`
	ExecutionOperations  []CollaborationStageExecutionOperationView  `json:"execution_operations"`
	RollbackObservations []CollaborationStageRollbackObservationView `json:"rollback_observations"`
}

// Populated by increments 2b/2c. Keep the frozen wire types explicit while their
// producer/consumer gates are unavailable; 2a returns empty arrays only.
type CollaborationStageTestReportSummary struct {
	ReportID        string    `json:"report_id"`
	RunID           string    `json:"run_id"`
	Attempt         string    `json:"attempt"`
	SourceKind      string    `json:"source_kind"`
	ReportedResult  string    `json:"reported_result"`
	Applicability   string    `json:"applicability"`
	CaseCount       int       `json:"case_count"`
	FailedCaseCount int       `json:"failed_case_count"`
	ReceivedAt      time.Time `json:"received_at"`
}

type CollaborationStageExecutionFactView struct {
	FactID                 string    `json:"fact_id"`
	IntentID               string    `json:"intent_id"`
	TargetDigest           string    `json:"target_digest"`
	Kind                   string    `json:"kind"`
	SourceKind             string    `json:"source_kind"`
	Environment            string    `json:"environment"`
	OperationID            string    `json:"operation_id"`
	Result                 string    `json:"result"`
	ArtifactDigest         string    `json:"artifact_digest"`
	PreviousArtifactDigest *string   `json:"previous_artifact_digest"`
	ApprovalDecisionID     string    `json:"approval_decision_id"`
	ObservedAt             time.Time `json:"observed_at"`
}
