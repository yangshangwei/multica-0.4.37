// Package iteration defines the manual iteration contract shared by HTTP and
// transactional services. Server state is never reconstructed from client DTOs.
package iteration

import (
	"encoding/json"
	"fmt"
	"time"
)

const SchemaVersion = 1

type Iteration struct {
	ID                string     `json:"id"`
	WorkspaceID       string     `json:"workspace_id"`
	Name              string     `json:"name"`
	Description       *string    `json:"description"`
	CoordinatorUserID *string    `json:"coordinator_user_id"`
	Status            string     `json:"status"`
	Mode              string     `json:"mode"`
	StartDate         string     `json:"start_date"`
	EndDate           string     `json:"end_date"`
	Timezone          string     `json:"timezone"`
	Revision          int64      `json:"revision"`
	ScopeRevision     int64      `json:"scope_revision"`
	StartedAt         *time.Time `json:"started_at"`
	LogicalEndedAt    *time.Time `json:"logical_ended_at"`
	ProcessedAt       *time.Time `json:"processed_at"`
}

type Settings struct {
	WorkspaceID        string  `json:"workspace_id"`
	Enabled            bool    `json:"enabled"`
	Revision           int64   `json:"revision"`
	PlanningTimezone   *string `json:"planning_timezone"`
	EffectiveTimezone  string  `json:"effective_timezone"`
	TimezoneConfigured bool    `json:"timezone_configured"`
}

type Draft struct {
	Operation                 string      `json:"operation"`
	IterationID               *string     `json:"iteration_id"`
	ExpectedIterationRevision *int64      `json:"expected_iteration_revision"`
	ExpectedScopeRevision     *int64      `json:"expected_scope_revision"`
	ExpectedSettingsRevision  int64       `json:"expected_settings_revision"`
	Reason                    *string     `json:"reason"`
	Moves                     []Move      `json:"moves"`
	Start                     *StartDraft `json:"start"`
}

type Move struct {
	IssueID               string  `json:"issue_id"`
	ExpectedIssueRevision int64   `json:"expected_issue_revision"`
	ExpectedSourceID      *string `json:"expected_source_id"`
	TargetID              *string `json:"target_id"`
	AllowCompleted        bool    `json:"allow_completed"`
}

type StartDraft struct {
	TargetID        string           `json:"target_id"`
	Mode            string           `json:"mode"`
	TerminalChoices []TerminalChoice `json:"terminal_choices"`
}

type TerminalChoice struct {
	IssueID string `json:"issue_id"`
	Retain  bool   `json:"retain"`
}

type WriteResult struct {
	WorkspaceID  string       `json:"workspace_id"`
	RequestID    string       `json:"request_id"`
	OperationID  string       `json:"operation_id"`
	Operation    string       `json:"operation"`
	Replayed     bool         `json:"replayed"`
	IterationIDs []string     `json:"iteration_ids"`
	Result       WriteSummary `json:"result"`
	CommittedAt  time.Time    `json:"committed_at"`
}

type WriteSummary struct {
	SnapshotID       *string `json:"snapshot_id"`
	Deleted          bool    `json:"deleted"`
	SettingsRevision int64   `json:"settings_revision"`
	IssueCount       int     `json:"issue_count"`
}

type HistoricalIssue struct {
	IssueID             string  `json:"issue_id"`
	Identifier          string  `json:"identifier"`
	Title               string  `json:"title"`
	ProjectID           *string `json:"project_id"`
	ProjectName         *string `json:"project_name"`
	AssigneeType        *string `json:"assignee_type"`
	AssigneeID          *string `json:"assignee_id"`
	AssigneeName        *string `json:"assignee_name"`
	StatusKey           string  `json:"status_key"`
	StatusCategory      string  `json:"status_category"`
	WasCompletedAtStart bool    `json:"was_completed_at_start"`
	RolloverCount       int     `json:"rollover_count"`
}

type Event struct {
	ID          string          `json:"id"`
	Sequence    int64           `json:"sequence"`
	IterationID string          `json:"iteration_id"`
	IssueID     *string         `json:"issue_id"`
	OperationID string          `json:"operation_id"`
	Kind        string          `json:"kind"`
	Actor       json.RawMessage `json:"actor"`
	OccurredAt  time.Time       `json:"occurred_at"`
	SampledAt   time.Time       `json:"sampled_at"`
	BeforeFacts json.RawMessage `json:"before_facts"`
	AfterFacts  json.RawMessage `json:"after_facts"`
	Reason      *string         `json:"reason"`
}

type Statistics struct {
	ScopeStatistics
	Chart        []ScopeChartPoint `json:"chart"`
	CalculatedAt time.Time         `json:"calculated_at"`
}

type Snapshot struct {
	SchemaVersion  int               `json:"schema_version"`
	WorkspaceID    string            `json:"workspace_id"`
	IterationID    string            `json:"iteration_id"`
	OperationID    string            `json:"operation_id"`
	EndType        string            `json:"end_type"`
	Reason         string            `json:"reason"`
	LogicalEndedAt time.Time         `json:"logical_ended_at"`
	ProcessedAt    time.Time         `json:"processed_at"`
	Original       []HistoricalIssue `json:"original"`
	Scope          []HistoricalIssue `json:"scope"`
	Events         []Event           `json:"events"`
	Statistics     Statistics        `json:"statistics"`
	Destinations   []Destination     `json:"destinations"`
}

type Destination struct {
	IssueID             string  `json:"issue_id"`
	TargetIterationID   *string `json:"target_iteration_id"`
	RolloverCountBefore int     `json:"rollover_count_before"`
	RolloverCountAfter  int     `json:"rollover_count_after"`
}

// OperationError carries only safe machine-readable information. Callers must
// authorize any resource details separately before including them in a response.
type OperationError struct {
	Status    int    `json:"-"`
	Code      string `json:"code"`
	Message   string `json:"error"`
	Retryable bool   `json:"retryable,omitempty"`
}

func (e *OperationError) Error() string { return fmt.Sprintf("%s: %s", e.Code, e.Message) }
