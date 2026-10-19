package protocol

// CollaborationRuntimeProtocolVersion is independent of ordinary task claims.
const CollaborationRuntimeProtocolVersion = 2

const CollaborationContainmentLinuxPIDNamespace = "linux_pid_namespace_v1"

// Business outcomes are distinct from command/group terminal states.
const (
	CollaborationBusinessOutcomeSuccess = "success"
	CollaborationBusinessOutcomeFailure = "failure"
)

type CollaborationRunListResponse struct {
	RunIDs []string `json:"run_ids"`
}

// CollaborationRunIdentity is fixed before any provider attempt can start.
// Decimal versions are strings so installed clients cannot round them.
type CollaborationRunIdentity struct {
	DeploymentID    string `json:"deployment_id"`
	InstallationID  string `json:"installation_id"`
	IssueID         string `json:"issue_id"`
	SessionID       string `json:"session_id"`
	TargetID        string `json:"target_id"`
	ChannelID       string `json:"channel_id"`
	BatchID         string `json:"batch_id"`
	GroupID         string `json:"group_id"`
	RunID           string `json:"run_id"`
	Fence           string `json:"fence"`
	ClaimGeneration string `json:"claim_generation"`
	LaunchNonce     string `json:"launch_nonce"`
}

// CollaborationRunInput retains every human authorization in a batch.
type CollaborationRunInput struct {
	CommandID        string   `json:"command_id"`
	AuthorUserID     string   `json:"author_user_id"`
	ParticipantEpoch string   `json:"participant_epoch"`
	AccountEpoch     string   `json:"account_epoch"`
	LifecycleEpoch   string   `json:"lifecycle_epoch"`
	Body             string   `json:"body"`
	AttachmentIDs    []string `json:"attachment_ids"`
}

type CollaborationRunClaimRequest struct {
	ProtocolVersion int    `json:"protocol_version"`
	DeploymentID    string `json:"deployment_id"`
	InstallationID  string `json:"installation_id"`
	RuntimeID       string `json:"runtime_id"`
	BindingID       string `json:"binding_id"`
	BindingEpoch    string `json:"binding_epoch"`
	Containment     string `json:"containment"`
}

// TerminationToken belongs only to the supervisor and its private ledger.
type CollaborationRunClaimResponse struct {
	Identity             CollaborationRunIdentity `json:"identity"`
	Inputs               []CollaborationRunInput  `json:"inputs"`
	AgentID              string                   `json:"agent_id"`
	ResourceOwnerID      string                   `json:"resource_owner_user_id"`
	ExecutionWorkspaceID string                   `json:"execution_workspace_id"`
	TerminationToken     string                   `json:"termination_token"`
}

type CollaborationRunPermitRequest struct {
	Identity     CollaborationRunIdentity `json:"identity"`
	AttemptSeq   string                   `json:"attempt_seq"`
	AttemptNonce string                   `json:"attempt_nonce"`
	Containment  string                   `json:"containment"`
}

type CollaborationRunPermitResponse struct {
	Identity      CollaborationRunIdentity `json:"identity"`
	AttemptSeq    string                   `json:"attempt_seq"`
	AttemptNonce  string                   `json:"attempt_nonce"`
	BusinessToken string                   `json:"business_token"`
}

type CollaborationRunStopIntent struct {
	Identity      CollaborationRunIdentity `json:"identity"`
	StopRequested bool                     `json:"stop_requested"`
}

// Business outcomes are separate from OS exit observations and sealed proofs.
type CollaborationRunOutcome struct {
	Identity     CollaborationRunIdentity `json:"identity"`
	EventID      string                   `json:"event_id"`
	AttemptSeq   string                   `json:"attempt_seq"`
	AttemptNonce string                   `json:"attempt_nonce"`
	Outcome      string                   `json:"outcome"`
	Body         string                   `json:"body"`
	Record       *CollaborationRunRecord  `json:"record,omitempty"`
}

// CollaborationRunProof intentionally cannot carry content, paths or logs.
type CollaborationRunProof struct {
	ReceiptID                string                   `json:"receipt_id"`
	Identity                 CollaborationRunIdentity `json:"identity"`
	LastAuthorizedAttemptSeq string                   `json:"last_authorized_attempt_seq"`
	SealedAttemptSeq         string                   `json:"sealed_attempt_seq"`
	AttemptNonce             string                   `json:"attempt_nonce"`
	State                    string                   `json:"state"`
	Containment              string                   `json:"containment"`
	ExitCode                 *int                     `json:"exit_code,omitempty"`
}

type CollaborationRunProofReceipt struct {
	ReceiptID string                   `json:"receipt_id"`
	Identity  CollaborationRunIdentity `json:"identity"`
	Accepted  bool                     `json:"accepted"`
}
