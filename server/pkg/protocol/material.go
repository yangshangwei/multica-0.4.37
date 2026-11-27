package protocol

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"
)

const DaemonCapabilityMaterialOperationsV1 = "material-operations-v1"
const MaterialExecutionVersion = 1
const DaemonCapabilityMaterialRepositoryV1 = "material-repository-v1"

// One complete snapshot may carry JSON-escaped 32 MiB source data plus metadata.
const MaxMaterialObservationBytes = 200 << 20

type MaterialRepositoryExecution struct {
	SystemID      string          `json:"system_id"`
	RootPath      string          `json:"root_path"`
	BundleVersion string          `json:"bundle_version"`
	CatalogDigest string          `json:"catalog_digest"`
	BaseCommit    string          `json:"base_commit,omitempty"`
	Candidate     json.RawMessage `json:"candidate,omitempty"`
	Maintenance   json.RawMessage `json:"maintenance,omitempty"`
	ReadyProof    json.RawMessage `json:"ready_proof,omitempty"`
}

// MaterialExecution is the immutable, credential-free expectation fixed before enqueue.
// Its digest excludes both ExpectedDigest itself and the mutable host-only claim binding.
type MaterialExecution struct {
	TargetKind           string                       `json:"target_kind,omitempty"`
	RepositoryID         string                       `json:"repository_id,omitempty"`
	RepositoryRevisionID string                       `json:"repository_revision_id,omitempty"`
	RepositoryGeneration int64                        `json:"repository_generation,omitempty"`
	RepositoryExecution  *MaterialRepositoryExecution `json:"repository_execution,omitempty"`
	ProtocolVersion      int                          `json:"protocol_version"`
	OperationID          string                       `json:"operation_id"`
	JobID                string                       `json:"job_id"`
	MaterialID           string                       `json:"material_id"`
	RevisionID           string                       `json:"revision_id"`
	ProjectID            string                       `json:"project_id"`
	WorkspaceID          string                       `json:"workspace_id"`
	TaskID               string                       `json:"task_id"`
	Action               string                       `json:"action"`
	ReconcileAction      string                       `json:"reconcile_action,omitempty"`
	OperationKey         string                       `json:"operation_key"`
	ConfigVersion        int64                        `json:"config_version"`
	ResourceID           string                       `json:"resource_id"`
	Provider             string                       `json:"provider"`
	InstanceURL          string                       `json:"instance_url"`
	Repository           string                       `json:"repository"`
	BaseBranch           string                       `json:"base_branch"`
	SourceBranch         string                       `json:"source_branch"`
	RuntimeID            string                       `json:"runtime_id"`
	DaemonID             string                       `json:"daemon_id"`
	ProfileID            string                       `json:"profile_id"`
	ConfigDigest         string                       `json:"config_digest"`
	DecisionID           string                       `json:"decision_id,omitempty"`
	ApprovedHead         string                       `json:"approved_head,omitempty"`
	BaselineID           string                       `json:"baseline_id,omitempty"`
	BaselineCommit       string                       `json:"baseline_commit,omitempty"`
	ReviewNumber         int64                        `json:"review_number,omitempty"`
	ReviewURL            string                       `json:"review_url,omitempty"`
	Title                string                       `json:"title,omitempty"`
	Body                 string                       `json:"body,omitempty"`
	AuditNote            *MaterialAuditNote           `json:"audit_note,omitempty"`
	Files                []MaterialFile               `json:"files"`
	ExpectedDigest       string                       `json:"expected_digest"`
}

func (e MaterialExecution) Digest() string {
	e.ExpectedDigest = ""
	if e.RepositoryExecution != nil {
		copy := *e.RepositoryExecution
		canonical := func(raw json.RawMessage) json.RawMessage {
			if len(raw) == 0 {
				return nil
			}
			var v any
			d := json.NewDecoder(bytes.NewReader(raw))
			d.UseNumber()
			if d.Decode(&v) != nil {
				return raw
			}
			out, err := json.Marshal(v)
			if err != nil {
				return raw
			}
			return out
		}
		copy.Candidate = canonical(copy.Candidate)
		copy.Maintenance = canonical(copy.Maintenance)
		copy.ReadyProof = canonical(copy.ReadyProof)
		e.RepositoryExecution = &copy
	}
	raw, _ := json.Marshal(e)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

// MaterialClaimBinding is delivered only on the authenticated claim transport.
// Never expose this value in prompts, task credentials, tool output or agent env.
type MaterialClaimBinding struct {
	ClaimGeneration int64  `json:"claim_generation"`
	HostChallenge   string `json:"host_challenge"`
}

type MaterialOperationResponse struct {
	Execution       MaterialExecution `json:"execution"`
	ClaimGeneration int64             `json:"claim_generation"`
}

type MaterialFile struct {
	Path    string `json:"path"`
	Mode    string `json:"mode"`
	Content string `json:"content"`
	SHA256  string `json:"sha256"`
	Commit  string `json:"commit,omitempty"`
}

type MaterialChange struct {
	Path         string `json:"path"`
	PreviousPath string `json:"previous_path,omitempty"`
	Status       string `json:"status"`
	Mode         string `json:"mode"`
}

type MaterialReview struct {
	Number       int64            `json:"number"`
	URL          string           `json:"url"`
	SourceBranch string           `json:"source_branch"`
	BaseBranch   string           `json:"base_branch"`
	HeadSHA      string           `json:"head_sha"`
	State        string           `json:"state"`
	MergeSHA     string           `json:"merge_sha"`
	MergeActor   string           `json:"merge_actor"`
	MergedAt     *time.Time       `json:"merged_at,omitempty"`
	GatesPassed  bool             `json:"gates_passed"`
	BlockReason  string           `json:"block_reason,omitempty"`
	Changes      []MaterialChange `json:"changes"`
}

type MaterialAuditNote struct {
	Marker string `json:"marker"`
	Body   string `json:"body"`
	Actor  string `json:"actor"`
}

type MaterialObservation struct {
	RepositorySnapshot    json.RawMessage    `json:"repository_snapshot,omitempty"`
	RepositoryPublication json.RawMessage    `json:"repository_publication,omitempty"`
	RulesSnapshot         json.RawMessage    `json:"rules_snapshot,omitempty"`
	ProtocolVersion       int                `json:"protocol_version"`
	OperationID           string             `json:"operation_id"`
	TaskID                string             `json:"task_id"`
	RuntimeID             string             `json:"runtime_id"`
	ClaimGeneration       int64              `json:"claim_generation"`
	ExpectedDigest        string             `json:"expected_digest"`
	ObservationID         string             `json:"observation_id"`
	HostChallenge         string             `json:"host_challenge"`
	ObservedAt            time.Time          `json:"observed_at"`
	Outcome               string             `json:"outcome"`
	SourceComplete        bool               `json:"source_complete"`
	RepositoryIdentity    string             `json:"repository_identity"`
	Review                *MaterialReview    `json:"review,omitempty"`
	Files                 []MaterialFile     `json:"files"`
	AuditNote             *MaterialAuditNote `json:"audit_note,omitempty"`
	Blocker               string             `json:"blocker,omitempty"`
}

type MaterialPermitInput struct {
	RequestID      string `json:"request_id"`
	ExpectedDigest string `json:"expected_digest"`
}

type MaterialPermit struct {
	OperationID    string    `json:"operation_id"`
	Action         string    `json:"action"`
	ExpectedDigest string    `json:"expected_digest"`
	HeadSHA        string    `json:"head_sha"`
	Paths          []string  `json:"paths"`
	ExpiresAt      time.Time `json:"expires_at"`
}
