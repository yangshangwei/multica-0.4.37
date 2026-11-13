// Package reportsnapshot freezes report facts and computes deterministic results.
// It performs no database, network, clock, permission, or provider operations.
package reportsnapshot

import (
	"encoding/json"
	"time"
)

const (
	ProtocolVersion  = 1
	SchemaVersion    = 1
	SemanticsVersion = "cohort-as-of-v1"
	CoverageVersion  = "coverage-v1"
	CollectorVersion = "postgres-facts-v1"
	MaxSnapshotBytes = 16 << 20
	MaxIssues        = 10000
	MaxActivities    = 100000
)

// Identity binds one immutable seal. RequestKey is an identifier, never a secret.
type Identity struct {
	SnapshotID     string  `json:"snapshot_id"`
	WorkspaceID    string  `json:"workspace_id"`
	ReportIssueID  string  `json:"report_issue_id"`
	ProjectID      *string `json:"project_id"`
	ReportRevision int64   `json:"report_revision_at_capture"`
	Sequence       int64   `json:"sequence"`
	CreatedBy      string  `json:"created_by"`
	RequestKey     string  `json:"request_key"`
	CorrectionOf   *string `json:"correction_of"`
}

type Window struct {
	Start      time.Time `json:"start"`
	End        time.Time `json:"end"`
	Timezone   string    `json:"timezone"`
	PeriodMode string    `json:"period_mode"`
}

// Input is trusted collector output, not a public capture request.
type Input struct {
	Identity
	Window    Window     `json:"window"`
	AsOf      time.Time  `json:"as_of"`
	ReadCut   string     `json:"read_cut"`
	Facts     Facts      `json:"facts"`
	TimeBasis *TimeBasis `json:"time_basis"`
}

type Facts struct {
	Health     *HealthFacts   `json:"health"`
	Issues     []IssueFact    `json:"issues"`
	Activities []ActivityFact `json:"activities"`
	Sources    []SourceFact   `json:"sources"`
	Gaps       []Gap          `json:"gaps"`
}

// HealthFacts retains only the scalars needed to replay project health. The
// unrestricted projecthealth.Reference.Facts payload is intentionally absent.
type HealthFacts struct {
	Project        ProjectFact     `json:"project"`
	Statuses       []StatusFact    `json:"statuses"`
	References     []ReferenceFact `json:"references"`
	LatestUpdateAt *time.Time      `json:"latest_update_at"`
}

type ProjectFact struct {
	WorkspaceID           string     `json:"workspace_id"`
	ID                    string     `json:"id"`
	Revision              int64      `json:"revision"`
	Status                string     `json:"status"`
	DueDate               *string    `json:"due_date"`
	LeadType              string     `json:"lead_type"`
	LeadID                string     `json:"lead_id"`
	InProgressSince       *time.Time `json:"in_progress_since"`
	InProgressSinceSource string     `json:"in_progress_since_source"`
}

type StatusFact struct {
	Key      string `json:"key"`
	Category string `json:"category"`
}

// TimeBasis saves resolved offsets so reading an old seal does not consult
// today's timezone database. Every date calculation uses these frozen offsets.
type TimeBasis struct {
	RulesVersion                 string `json:"rules_version"`
	ReferenceDate                string `json:"reference_date"`
	StartOffsetSeconds           int    `json:"start_offset_seconds"`
	EndOffsetSeconds             int    `json:"end_offset_seconds"`
	AsOfOffsetSeconds            int    `json:"as_of_offset_seconds"`
	LatestUpdateOffsetSeconds    *int   `json:"latest_update_offset_seconds"`
	InProgressSinceOffsetSeconds *int   `json:"in_progress_since_offset_seconds"`
}

type ReferenceFact struct {
	Kind                   string `json:"kind"`
	ID                     string `json:"id"`
	Valid                  bool   `json:"valid"`
	EnvironmentUnavailable bool   `json:"environment_unavailable"`
}

type IssueFact struct {
	ID                   string    `json:"id"`
	Revision             int64     `json:"revision"`
	ProjectID            *string   `json:"project_id"`
	Status               string    `json:"status"`
	AdmissionStatus      string    `json:"admission_status"`
	CreatedAt            time.Time `json:"created_at"`
	DueDate              *string   `json:"due_date"`
	AssigneeType         string    `json:"assignee_type"`
	AssigneeID           string    `json:"assignee_id"`
	OriginType           string    `json:"origin_type"`
	OriginID             *string   `json:"origin_id"`
	Classification       string    `json:"classification"`
	ClassificationReason string    `json:"classification_reason"`
	// These are proven source capabilities, not assumptions based on current rows.
	CreationProjectKnown bool       `json:"creation_project_known"`
	HistoryComplete      bool       `json:"history_complete"`
	BaselineAt           *time.Time `json:"baseline_at"`
	BaselineStatus       string     `json:"baseline_status"`
	BaselineCategory     string     `json:"baseline_category"`
}

type ActivityFact struct {
	ID            string     `json:"id"`
	IssueID       string     `json:"issue_id"`
	Action        string     `json:"action"`
	RecordedAt    time.Time  `json:"recorded_at"`
	BusinessAt    *time.Time `json:"business_at"`
	Sequence      *int64     `json:"sequence"`
	FromStatus    string     `json:"from_status"`
	ToStatus      string     `json:"to_status"`
	FromCategory  string     `json:"from_category"`
	ToCategory    string     `json:"to_category"`
	ProjectID     *string    `json:"project_id"`
	ProjectKnown  bool       `json:"project_known"`
	SourceVersion string     `json:"source_version"`
}

// ReadComplete only certifies the visible scan. ProducerComplete separately
// certifies that all relevant facts were recorded by that source.
type SourceIdentity struct {
	ID       string `json:"id"`
	Revision string `json:"revision"`
}

type SourceFact struct {
	Filters          []string         `json:"filters"`
	Identities       []SourceIdentity `json:"identities"`
	ContentDigest    string           `json:"content_digest"`
	Kind             string           `json:"kind"`
	Version          string           `json:"version"`
	Ordering         string           `json:"ordering"`
	ReadComplete     bool             `json:"read_complete"`
	ProducerComplete bool             `json:"producer_complete"`
	Watermark        *string          `json:"watermark"`
	Cursor           *string          `json:"cursor"`
	Count            *int64           `json:"count"`
}

type Gap struct {
	Code      string   `json:"code"`
	Source    string   `json:"source"`
	MetricIDs []string `json:"metric_ids"`
	IssueIDs  []string `json:"issue_ids"`
}

type Rational struct {
	Numerator   int64 `json:"numerator"`
	Denominator int64 `json:"denominator"`
}

type UnknownCoverage struct {
	Count         *int64   `json:"count"`
	KnownCount    int64    `json:"known_count"`
	ExtentUnknown bool     `json:"extent_unknown"`
	Reasons       []string `json:"reasons"`
}

type Coverage struct {
	Version                string          `json:"version"`
	Unit                   string          `json:"unit"`
	Eligible               *int64          `json:"eligible"`
	Included               int64           `json:"included"`
	Excluded               int64           `json:"excluded"`
	Unknown                UnknownCoverage `json:"unknown"`
	ObservedCandidateCount int64           `json:"observed_candidate_count"`
	Ratio                  *Rational       `json:"ratio"`
	IncludedIDs            []string        `json:"included_ids"`
	ExcludedIDs            []string        `json:"excluded_ids"`
	UnknownIDs             []string        `json:"unknown_ids"`
}

// Value contains exactly the JSON scalar/rational identified by ValueKind.
// Constructors and Validate prevent arbitrary JSON objects or lossy numbers.
type Metric struct {
	ID          string          `json:"id"`
	ValueKind   string          `json:"value_kind"`
	Value       json.RawMessage `json:"value"`
	Quality     string          `json:"quality"`
	LowerBound  *int64          `json:"lower_bound"`
	VerifiedIDs []string        `json:"verified_ids"`
	Coverage    Coverage        `json:"coverage"`
}

type Snapshot struct {
	FormalScope string `json:"formal_scope"`
	Input
	ProtocolVersion  int      `json:"protocol_version"`
	SchemaVersion    int      `json:"schema_version"`
	SemanticsVersion string   `json:"semantics_version"`
	CoverageVersion  string   `json:"coverage_version"`
	CollectorVersion string   `json:"collector_version"`
	TimeRulesVersion string   `json:"time_rules_version"`
	ReferenceDate    string   `json:"reference_date"`
	State            string   `json:"state"`
	Complete         bool     `json:"complete"`
	Metrics          []Metric `json:"metrics"`
	FactsDigest      string   `json:"facts_digest"`
	ResultDigest     string   `json:"result_digest"`
	SnapshotDigest   string   `json:"snapshot_digest"`
}
