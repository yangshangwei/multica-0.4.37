package service

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// StageExecutionSources is installed by trusted server construction only.
// Implementations authenticate captured evidence under live trust locks; stage
// policy and human authorization remain owned by this service. Nil is unavailable.
type StageExecutionSources interface {
	Targets(context.Context, pgx.Tx, pgtype.UUID) ([]StageExecutionTarget, error)
	AuthorizeExecution(context.Context, pgx.Tx, StageExecutionRequest, StageExecutionTarget, StageExecutionProof) (StageExecutionConsumer, error)
	Execute(context.Context, StageExecutionInvocation) (StageExecutionProof, error)
	ExecutionObservation(context.Context, pgx.Tx, StageExecutionInvocation, StageExecutionProof) (StageExecutionObservation, error)
	MetricObservation(context.Context, pgx.Tx, StageExecutionDeployment, StageExecutionProof) (StageRollbackMetric, error)
	CheckMetricCurrent(context.Context, pgx.Tx, StageExecutionTarget, StageRollbackMetric) error
}

// Proof bytes have no normal JSON representation and no public write endpoint.
// They are interpreted only by the configured, purpose-specific source.
type StageExecutionProof struct {
	Payload        []byte `json:"-"`
	Authentication []byte `json:"-"`
}

type StageIntegrationIdentity struct {
	Adapter        string `json:"adapter"`
	AdapterVersion string `json:"adapter_version"`
	IntegrationID  string `json:"integration_id"`
	TrustRevision  string `json:"trust_revision"`
	ConfigRevision string `json:"config_revision"`
}

type StageExecutionTarget struct {
	WorkspaceID           string                   `json:"workspace_id"`
	Environment           string                   `json:"environment"`
	Source                StageIntegrationIdentity `json:"source"`
	ConcurrencyDomain     string                   `json:"concurrency_domain"`
	RemoteInstance        string                   `json:"remote_instance"`
	RemoteTenant          string                   `json:"remote_tenant"`
	RemoteTarget          string                   `json:"remote_target"`
	SupportsConditional   bool                     `json:"supports_conditional"`
	SupportsOriginalQuery bool                     `json:"supports_original_query"`
	// Current state is bound privately at admission but excluded from the
	// configuration digest: a successful deployment may legitimately change it.
	ConditionalVersion    string  `json:"conditional_version"`
	CurrentArtifactDigest *string `json:"current_artifact_digest"`
}

type StageExecutionRequest struct {
	WorkspaceID        pgtype.UUID
	IssueID            pgtype.UUID
	RequestID          string
	ApprovalDecisionID pgtype.UUID
	Candidate          CollaborationStageCandidateRef
	TargetDigest       string
}

type StageExecutionConsumer struct {
	Subject    string    `json:"subject"`
	ValidUntil time.Time `json:"valid_until"`
}

type StageExecutionInvocation struct {
	IntentID           string               `json:"intent_id"`
	IssueID            string               `json:"issue_id"`
	ApprovalDecisionID string               `json:"approval_decision_id"`
	Kind               string               `json:"kind"`
	Target             StageExecutionTarget `json:"target"`
	TargetDigest       string               `json:"target_digest"`
	ArtifactDigest     string               `json:"artifact_digest"`
	ExternalRequestKey string               `json:"external_request_key"`
	DispatchExpiresAt  time.Time            `json:"dispatch_expires_at"`
	DeploymentID       *string              `json:"deployment_id"`
}

type StageExecutionObservation struct {
	NativeID                string                   `json:"native_id"`
	NativeVersion           string                   `json:"native_version"`
	Source                  StageIntegrationIdentity `json:"source"`
	ExternalRequestKey      string                   `json:"external_request_key"`
	ExternalOperationID     *string                  `json:"external_operation_id"`
	Result                  string                   `json:"result"`
	TargetDigest            string                   `json:"target_digest"`
	ConditionalVersion      string                   `json:"conditional_version"`
	AfterConditionalVersion string                   `json:"after_conditional_version"`
	BeforeArtifactDigest    *string                  `json:"before_artifact_digest"`
	AfterArtifactDigest     *string                  `json:"after_artifact_digest"`
	ObservedAt              time.Time                `json:"observed_at"`
	EvidenceDigest          string                   `json:"evidence_digest"`
	Summary                 string                   `json:"summary"`
}

type StageExecutionDeployment struct {
	FactID      string                    `json:"fact_id"`
	Invocation  StageExecutionInvocation  `json:"invocation"`
	Observation StageExecutionObservation `json:"observation"`
}

type StageRollbackMetric struct {
	StreamIdentity string                   `json:"stream_identity"`
	NativeID       string                   `json:"native_id"`
	NativeVersion  string                   `json:"native_version"`
	Source         StageIntegrationIdentity `json:"source"`
	DeploymentID   string                   `json:"deployment_id"`
	TargetDigest   string                   `json:"target_digest"`
	ArtifactDigest string                   `json:"artifact_digest"`
	QueryID        string                   `json:"query_id"`
	QueryDigest    string                   `json:"query_digest"`
	RuleID         string                   `json:"rule_id"`
	RuleDigest     string                   `json:"rule_digest"`
	WindowStart    time.Time                `json:"window_start"`
	WindowEnd      time.Time                `json:"window_end"`
	ObservedAt     time.Time                `json:"observed_at"`
	ValidUntil     time.Time                `json:"valid_until"`
	Condition      string                   `json:"condition"`
	EvidenceDigest string                   `json:"evidence_digest"`
	Summary        string                   `json:"summary"`
}

type StageExecutionReceipt struct {
	Version            int     `json:"version"`
	IssueID            string  `json:"issue_id"`
	RequestID          string  `json:"request_id"`
	IntentID           string  `json:"intent_id"`
	ApprovalDecisionID string  `json:"approval_decision_id"`
	Kind               string  `json:"kind"`
	State              string  `json:"state"`
	FactID             *string `json:"fact_id"`
	ObservationID      *string `json:"observation_id,omitempty"`
}

type StageRollbackMetricReceipt struct {
	Version            int    `json:"version"`
	IssueID            string `json:"issue_id"`
	ObservationID      string `json:"observation_id"`
	ObservationVersion string `json:"observation_version"`
	ObservationDigest  string `json:"observation_digest"`
}

type CollaborationStageRollbackRef struct {
	ObservationID     string `json:"observation_id"`
	ObservationDigest string `json:"observation_digest"`
}

// Later execution snapshots live with a decision, never in its request digest
// or in the immutable code-candidate content that earlier humans reviewed.
type stageExecutionBinding struct {
	Target         StageExecutionTarget           `json:"target"`
	TargetDigest   string                         `json:"target_digest"`
	ArtifactDigest string                         `json:"artifact_digest"`
	DeploymentID   *string                        `json:"deployment_id"`
	Metric         *CollaborationStageRollbackRef `json:"metric"`
}

type CollaborationStageExecutionTargetView struct {
	Environment  string `json:"environment"`
	TargetDigest string `json:"target_digest"`
	SourceKind   string `json:"source_kind"`
}

type CollaborationStageExecutionOperationView struct {
	IntentID            string    `json:"intent_id"`
	Kind                string    `json:"kind"`
	ApprovalDecisionID  string    `json:"approval_decision_id"`
	Environment         string    `json:"environment"`
	TargetDigest        string    `json:"target_digest"`
	ArtifactDigest      string    `json:"artifact_digest"`
	State               string    `json:"state"`
	ExternalOperationID *string   `json:"external_operation_id"`
	FactID              *string   `json:"fact_id"`
	CreatedAt           time.Time `json:"created_at"`
}

type CollaborationStageRollbackObservationView struct {
	ObservationID     string    `json:"observation_id"`
	Version           string    `json:"version"`
	ObservationDigest string    `json:"observation_digest"`
	DeploymentID      string    `json:"deployment_id"`
	Environment       string    `json:"environment"`
	TargetDigest      string    `json:"target_digest"`
	ArtifactDigest    string    `json:"artifact_digest"`
	Condition         string    `json:"condition"`
	State             string    `json:"state"`
	WindowStart       time.Time `json:"window_start"`
	WindowEnd         time.Time `json:"window_end"`
	ObservedAt        time.Time `json:"observed_at"`
	ValidUntil        time.Time `json:"valid_until"`
	Summary           string    `json:"summary"`
}
