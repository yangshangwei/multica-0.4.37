package service

import (
	"encoding/json"

	"github.com/multica-ai/multica/server/internal/budget"
)

// These projections are the frozen human management HTTP v1 contract. Nullable
// fields are always present, counters and money retain their exact spelling,
// and slice fields are initialized by the reader even when there are no rows.
type IssueBudgetWriteResponse struct {
	Replayed bool                       `json:"replayed"`
	Result   IssueBudgetOperationResult `json:"result"`
}

type IssueBudgetOperationLookup struct {
	State  string                     `json:"state"`
	Stale  bool                       `json:"stale"`
	Result IssueBudgetOperationResult `json:"result"`
}

type IssueBudgetOperationUnknown struct {
	State     string `json:"state"`
	RetryPath string `json:"retry_path"`
}

type IssueBudgetBinding struct {
	IssueID      string `json:"issue_id"`
	BudgetRootID string `json:"budget_root_id"`
	IsRootIssue  bool   `json:"is_root_issue"`
	BindSource   string `json:"bind_source"`
	BoundAt      string `json:"bound_at"`
}

type IssueBudgetShow struct {
	BudgetRootID        string                  `json:"budget_root_id"`
	RootIssueID         string                  `json:"root_issue_id"`
	AdmissionState      string                  `json:"admission_state"`
	Mode                string                  `json:"mode"`
	PolicyRevision      budget.Count            `json:"policy_revision"`
	PolicyEpoch         budget.Count            `json:"policy_epoch"`
	LedgerRevision      budget.Count            `json:"ledger_revision"`
	ActivatedAt         string                  `json:"activated_at"`
	SettledAsOfRevision *budget.Count           `json:"settled_as_of_revision"`
	LimitTicks          *budget.Ticks           `json:"limit_ticks"`
	AttemptReserveTicks *budget.Ticks           `json:"attempt_reserve_ticks"`
	LedgerState         string                  `json:"ledger_state"`
	Amounts             *IssueBudgetAmounts     `json:"amounts"`
	Unknown             IssueBudgetUnknown      `json:"unknown"`
	TotalKnown          bool                    `json:"total_known"`
	RemainingTicks      *budget.Ticks           `json:"remaining_ticks"`
	Coverage            IssueBudgetReadCoverage `json:"coverage"`
}

type IssueBudgetAmounts struct {
	ActualKnown budget.Ticks `json:"actual_known_ticks"`
	Estimated   budget.Ticks `json:"estimated_ticks"`
	Reserved    budget.Ticks `json:"reserved_ticks"`
	UnknownHold budget.Ticks `json:"unknown_hold_ticks"`
	Overrun     budget.Ticks `json:"overrun_ticks"`
}

type IssueBudgetUnknown struct {
	ByReason      []IssueBudgetUnknownCount `json:"by_reason"`
	PreActivation int64                     `json:"pre_activation"`
}

type IssueBudgetUnknownCount struct {
	Reason string `json:"reason"`
	Count  int64  `json:"count"`
}

type IssueBudgetReadCoverage struct {
	IncludedAttempts *int     `json:"included_attempts"`
	NotApplicable    []string `json:"not_applicable"`
}

type IssueBudgetObservationPage struct {
	Observations []IssueBudgetObservationView `json:"observations"`
	NextCursor   *string                      `json:"next_cursor"`
}

type IssueBudgetObservationView struct {
	ID              string          `json:"id"`
	Kind            string          `json:"kind"`
	Reason          string          `json:"reason"`
	IssueID         *string         `json:"issue_id"`
	TaskID          *string         `json:"task_id"`
	ClaimGeneration *budget.Count   `json:"claim_generation"`
	AttemptID       *string         `json:"attempt_id"`
	Covered         bool            `json:"covered"`
	Detail          json.RawMessage `json:"detail"`
	CreatedAt       string          `json:"created_at"`
}

type IssueBudgetAttemptPage struct {
	LedgerState string                   `json:"ledger_state"`
	Attempts    []IssueBudgetAttemptView `json:"attempts"`
	NextCursor  *string                  `json:"next_cursor"`
}

type IssueBudgetAttemptView struct {
	AttemptID            string                        `json:"attempt_id"`
	TaskID               string                        `json:"task_id"`
	ClaimGeneration      budget.Count                  `json:"claim_generation"`
	IssueID              *string                       `json:"issue_id"`
	ContextParentIssueID *string                       `json:"context_parent_issue_id"`
	AgentID              string                        `json:"agent_id"`
	RuntimeID            string                        `json:"runtime_id"`
	State                string                        `json:"state"`
	ReservedTicks        *budget.Ticks                 `json:"reserved_ticks"`
	ProtocolVersion      *int32                        `json:"protocol_version"`
	CreatedAt            string                        `json:"created_at"`
	Reservation          *budget.ReservationProjection `json:"reservation"`
}
