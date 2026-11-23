package reportsnapshot

import "time"

const PublicProjectionVersion = 1

// PublicInput is the complete safe input for public metric replay. It deliberately
// does not embed internal Identity, Input, source manifests or directory rows.
type PublicInput struct {
	SnapshotID     string      `json:"snapshot_id"`
	WorkspaceID    string      `json:"workspace_id"`
	ReportIssueID  string      `json:"report_issue_id"`
	ProjectID      *string     `json:"project_id"`
	ReportRevision int64       `json:"report_revision_at_capture"`
	CorrectionOf   *string     `json:"correction_of"`
	Window         Window      `json:"window"`
	AsOf           time.Time   `json:"as_of"`
	TimeBasis      *TimeBasis  `json:"time_basis"`
	Facts          PublicFacts `json:"facts"`
}

type PublicFacts struct {
	Project    *PublicProjectFact `json:"project"`
	Issues     []PublicIssueFact  `json:"issues"`
	Activities []PublicActivity   `json:"activities"`
	Sources    []PublicSource     `json:"sources"`
	Gaps       []Gap              `json:"gaps"`
}

type PublicProjectFact struct {
	ID              string     `json:"id"`
	Revision        int64      `json:"revision"`
	Status          string     `json:"status"`
	DueDate         *string    `json:"due_date"`
	InProgressSince *time.Time `json:"in_progress_since"`
	LatestUpdateAt  *time.Time `json:"latest_update_at"`
	LeadValid       *bool      `json:"lead_valid"`
}

type PublicIssueFact struct {
	ID                     string     `json:"id"`
	Revision               int64      `json:"revision"`
	ProjectID              *string    `json:"project_id"`
	Status                 string     `json:"status"`
	Category               string     `json:"category"`
	AdmissionStatus        string     `json:"admission_status"`
	CreatedAt              time.Time  `json:"created_at"`
	DueDate                *string    `json:"due_date"`
	AssignmentValid        *bool      `json:"assignment_valid"`
	EnvironmentUnavailable *bool      `json:"environment_unavailable"`
	Classification         string     `json:"classification"`
	ClassificationReason   string     `json:"classification_reason"`
	CreationProjectKnown   bool       `json:"creation_project_known"`
	HistoryComplete        bool       `json:"history_complete"`
	BaselineAt             *time.Time `json:"baseline_at"`
	BaselineStatus         string     `json:"baseline_status"`
	BaselineCategory       string     `json:"baseline_category"`
}

// PublicActivity has a snapshot-local identity, never the native activity ID.
// Only facts required by a proven lifecycle are projected.
type PublicActivity struct {
	FactID       string    `json:"fact_id"`
	IssueID      string    `json:"issue_id"`
	BusinessAt   time.Time `json:"business_at"`
	Sequence     int64     `json:"sequence"`
	FromStatus   string    `json:"from_status"`
	ToStatus     string    `json:"to_status"`
	FromCategory string    `json:"from_category"`
	ToCategory   string    `json:"to_category"`
	ProjectID    *string   `json:"project_id"`
}

type PublicSource struct {
	Kind              string `json:"kind"`
	Version           string `json:"version"`
	ReadComplete      bool   `json:"read_complete"`
	ProducerComplete  bool   `json:"producer_complete"`
	RetentionComplete *bool  `json:"retention_complete"`
	ProofVisibility   string `json:"proof_visibility"`
}

type PublicSnapshot struct {
	PublicInput
	FormalScope             string   `json:"formal_scope"`
	ProtocolVersion         int      `json:"protocol_version"`
	PublicProjectionVersion int      `json:"public_projection_version"`
	SemanticsVersion        string   `json:"semantics_version"`
	CoverageVersion         string   `json:"coverage_version"`
	TimeRulesVersion        string   `json:"time_rules_version"`
	ReferenceDate           string   `json:"reference_date"`
	State                   string   `json:"state"`
	Complete                bool     `json:"complete"`
	Metrics                 []Metric `json:"metrics"`
	ProjectReasons          []string `json:"project_reasons"`
	FactsDigest             string   `json:"facts_digest"`
	ResultDigest            string   `json:"result_digest"`
	ProjectionDigest        string   `json:"projection_digest"`
}
