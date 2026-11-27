package material

import "time"

// Initialization is a project/root workflow, independent of requirement review.
type Initialization struct {
	History                 []RepositoryReadiness      `json:"history"`
	WorkspaceID             string                     `json:"workspace_id"`
	ProjectID               string                     `json:"project_id"`
	ProtocolVersion         int                        `json:"protocol_version"`
	Version                 int64                      `json:"version"`
	SettingsVersion         int64                      `json:"settings_version"`
	BindingGeneration       int64                      `json:"binding_generation"`
	Status                  string                     `json:"status"`
	Blocker                 *string                    `json:"blocker"`
	Readiness               RepositoryReadiness        `json:"readiness"`
	Operation               *Operation                 `json:"operation"`
	Candidate               *InitializationCandidate   `json:"candidate"`
	Decisions               []InitializationDecision   `json:"decisions"`
	AvailableBundleVersions []string                   `json:"available_bundle_versions"`
	Capabilities            InitializationCapabilities `json:"capabilities"`
}
type RepositoryReadiness struct {
	PublishAllowed bool       `json:"publish_allowed"`
	VerifiedCommit *string    `json:"verified_commit"`
	ManifestSHA256 *string    `json:"manifest_sha256"`
	SchemaVersion  *int       `json:"schema_version"`
	BundleVersion  *string    `json:"bundle_version"`
	VerifiedAt     *time.Time `json:"verified_at"`
}
type InitializationCapabilities struct {
	CanCheck    bool `json:"can_check"`
	CanReview   bool `json:"can_review"`
	CanRetry    bool `json:"can_retry"`
	CanMaintain bool `json:"can_maintain"`
}
type InitializationCandidate struct {
	ID                string               `json:"id"`
	Kind              string               `json:"kind"`
	Version           int64                `json:"version"`
	BindingGeneration int64                `json:"binding_generation"`
	BundleVersion     string               `json:"bundle_version"`
	ManifestSHA256    string               `json:"manifest_sha256"`
	BaseCommit        string               `json:"base_commit"`
	HeadSHA           *string              `json:"head_sha"`
	ReviewURL         *string              `json:"review_url"`
	PublicationStatus string               `json:"publication_status"`
	Files             []InitializationFile `json:"files"`
}
type InitializationFile struct {
	Path          string  `json:"path"`
	Action        string  `json:"action"`
	BeforeSHA256  *string `json:"before_sha256"`
	AfterSHA256   *string `json:"after_sha256"`
	BeforeContent *string `json:"before_content"`
	AfterContent  *string `json:"after_content"`
	Explanation   *string `json:"explanation"`
}
type InitializationDecision struct {
	SettingsVersion   int64     `json:"settings_version"`
	BindingGeneration int64     `json:"binding_generation"`
	ID                string    `json:"id"`
	CandidateID       string    `json:"candidate_id"`
	ActorID           string    `json:"actor_id"`
	Decision          string    `json:"decision"`
	Reason            string    `json:"reason"`
	HeadSHA           string    `json:"head_sha"`
	ManifestSHA256    string    `json:"manifest_sha256"`
	CreatedAt         time.Time `json:"created_at"`
}
type InitializationCommand struct {
	RequestID               string `json:"request_id"`
	ExpectedVersion         int64  `json:"expected_version"`
	ExpectedSettingsVersion int64  `json:"expected_settings_version"`
	TargetBundleVersion     string `json:"target_bundle_version,omitempty"`
}
type InitializationDecisionInput struct {
	RequestID               string `json:"request_id"`
	ExpectedVersion         int64  `json:"expected_version"`
	ExpectedSettingsVersion int64  `json:"expected_settings_version"`
	CandidateID             string `json:"candidate_id"`
	HeadSHA                 string `json:"head_sha"`
	ManifestSHA256          string `json:"manifest_sha256"`
	Decision                string `json:"decision"`
	Reason                  string `json:"reason"`
}
