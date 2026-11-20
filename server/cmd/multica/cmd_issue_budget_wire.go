package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"

	"github.com/multica-ai/multica/server/internal/budget"
)

// This file is the reading side of the budget HTTP shape (design 12.9): the
// structs an answer is read into, and the rule that decides whether an answer
// was understood.
//
// encoding/json leaves a field untouched when its key is missing, and for most
// types also when its value is null, so a struct decoded the plain way cannot
// tell "absent" from zero. Every field here is therefore one of three wrappers
// that record what the answer held. understandBudgetAnswer checks each of them
// afterwards: a required key that is missing, a null where 12.9 gives none, a
// value in another form or a key that is there twice makes the answer "not
// understood". No field is ever given a default.

// budgetSlot is what the three kinds of field share. Its UnmarshalJSON only
// records what the answer held. The value is read afterwards, by settle, so
// that the caller's choice between ignoring and refusing unknown keys reaches
// every depth of the answer.
type budgetSlot[T any] struct {
	value   T
	raw     json.RawMessage
	present bool // the key was in the answer
	null    bool // and its value was null
	twice   bool // the key was in the answer more than once
}

func (s *budgetSlot[T]) UnmarshalJSON(data []byte) error {
	if s.present {
		s.twice = true
		return nil
	}
	s.present = true
	s.null = string(data) == "null"
	s.raw = append(s.raw[:0], data...)
	return nil
}

// budgetFieldRule is what 12.9 says about the presence of one field.
type budgetFieldRule int

const (
	budgetFieldRequired budgetFieldRule = iota // the key is there and its value is not null
	budgetFieldNullable                        // the key is there; its value may be null
	budgetFieldOptional                        // the key may be left out; it is never null
)

// settleAs reads the recorded value. It returns what is wrong with the field,
// or "" when the answer may be used; inner is the value to check further, nil
// when the answer gave none.
func (s *budgetSlot[T]) settleAs(rule budgetFieldRule, strict bool) (inner any, problem string, cause error) {
	switch {
	case s.twice:
		return nil, "is there twice", nil
	case !s.present && rule == budgetFieldOptional:
		return nil, "", nil
	case !s.present:
		return nil, "is missing", nil
	case s.null && rule == budgetFieldNullable:
		return nil, "", nil
	case s.null:
		return nil, "is null", nil
	}
	decoder := json.NewDecoder(bytes.NewReader(s.raw))
	if strict {
		decoder.DisallowUnknownFields()
	}
	if err := decoder.Decode(&s.value); err != nil {
		return nil, "is not in the form this command reads", err
	}
	return &s.value, "", nil
}

// budgetRequired is a field 12.9 always gives, with a value.
type budgetRequired[T any] struct{ budgetSlot[T] }

func (f *budgetRequired[T]) settle(strict bool) (any, string, error) {
	return f.settleAs(budgetFieldRequired, strict)
}

// budgetNullable is a field 12.9 always gives, as a value or as null.
type budgetNullable[T any] struct{ budgetSlot[T] }

func (f *budgetNullable[T]) settle(strict bool) (any, string, error) {
	return f.settleAs(budgetFieldNullable, strict)
}

// pointer is the value, or nil when the answer gave null.
func (f *budgetNullable[T]) pointer() *T {
	if f.null {
		return nil
	}
	return &f.value
}

// budgetOptional is a field 12.9 leaves out in some answers. Whoever reads it
// asks present first.
type budgetOptional[T any] struct{ budgetSlot[T] }

func (f *budgetOptional[T]) settle(strict bool) (any, string, error) {
	return f.settleAs(budgetFieldOptional, strict)
}

// budgetField is implemented by the three wrappers.
type budgetField interface {
	settle(strict bool) (inner any, problem string, cause error)
}

// budgetAnswerError says why an answer was not understood. Its text is this
// command's own. cause, when there is one, is the decoder's error: its wording
// differs between Go versions, so it is kept for a developer and left out of
// what the operator reads.
type budgetAnswerError struct {
	field   string
	problem string
	cause   error
}

func (e *budgetAnswerError) Error() string {
	if e.field == "" {
		return "it " + e.problem
	}
	return fmt.Sprintf("field %q %s", e.field, e.problem)
}

func (e *budgetAnswerError) Unwrap() error { return e.cause }

// settleBudgetValue checks one value of an answer and everything inside it.
// v is addressable; name is the path of the value in the answer.
func settleBudgetValue(v reflect.Value, name string, strict bool) error {
	if field, ok := v.Addr().Interface().(budgetField); ok {
		inner, problem, cause := field.settle(strict)
		if problem != "" {
			return &budgetAnswerError{field: name, problem: problem, cause: cause}
		}
		if inner == nil {
			return nil
		}
		return settleBudgetValue(reflect.ValueOf(inner).Elem(), name, strict)
	}
	switch v.Kind() {
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			key, _, _ := strings.Cut(v.Type().Field(i).Tag.Get("json"), ",")
			if _, ok := v.Field(i).Addr().Interface().(budgetField); !ok || key == "" {
				// A plain field would be left at its zero value by a missing
				// key. That is a defect of this file, not of the answer.
				return fmt.Errorf("budget wire struct %s: field %s is not a named budget field", v.Type(), v.Type().Field(i).Name)
			}
			if name != "" {
				key = name + "." + key
			}
			if err := settleBudgetValue(v.Field(i), key, strict); err != nil {
				return err
			}
		}
	case reflect.Slice:
		if v.Type().Elem().Kind() == reflect.Uint8 {
			return nil // a JSON value kept as its bytes
		}
		for i := 0; i < v.Len(); i++ {
			if err := settleBudgetValue(v.Index(i), fmt.Sprintf("%s[%d]", name, i), strict); err != nil {
				return err
			}
		}
	}
	return nil
}

// understandBudgetAnswer reads the body of an answer into the struct made for
// it, the way the commands read: a key the struct does not know is ignored.
// An error means the answer was not understood and nothing in the struct may
// be used.
func understandBudgetAnswer(data []byte, answer any) error {
	return readBudgetAnswer(data, answer, false)
}

// readBudgetAnswer is understandBudgetAnswer with a choice: strict also refuses
// a key the struct does not know, at every depth. The test of the fixture
// files reads that way, to prove the structs know every field of 12.9.
func readBudgetAnswer(data []byte, answer any, strict bool) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	if strict {
		decoder.DisallowUnknownFields()
	}
	if err := decoder.Decode(answer); err != nil {
		return &budgetAnswerError{problem: "is not one JSON object in the form this command reads", cause: err}
	}
	if _, err := decoder.Token(); err != io.EOF {
		return &budgetAnswerError{problem: "is followed by something that is not part of it", cause: err}
	}
	return settleBudgetValue(reflect.ValueOf(answer).Elem(), "", strict)
}

// budgetNumber is a count that 12.9 spells as a JSON number: digits only, no
// sign, no fraction, no exponent.
type budgetNumber int64

func (n *budgetNumber) UnmarshalJSON(data []byte) error {
	value, err := budget.ParseDecimal(string(data))
	if err != nil {
		return err
	}
	*n = budgetNumber(value)
	return nil
}

// budgetJSONObject is a JSON object that is shown as it came.
type budgetJSONObject []byte

func (o *budgetJSONObject) UnmarshalJSON(data []byte) error {
	if len(data) == 0 || data[0] != '{' {
		return errors.New("not a JSON object")
	}
	*o = append((*o)[:0], data...)
	return nil
}

// The answers of design 12.9, one struct per shape. The order of the fields is
// the order of 12.9.

// budgetWriteAnswer is the answer to a write that committed.
type budgetWriteAnswer struct {
	Replayed budgetRequired[bool]                  `json:"replayed"`
	Result   budgetRequired[budgetOperationResult] `json:"result"`
}

// budgetOperationResult is the result of one committed operation.
type budgetOperationResult struct {
	OperationID         budgetRequired[string]       `json:"operation_id"`
	OperationKind       budgetRequired[string]       `json:"operation_kind"`
	BudgetRootID        budgetRequired[string]       `json:"budget_root_id"`
	RootIssueID         budgetRequired[string]       `json:"root_issue_id"`
	AdmissionState      budgetRequired[string]       `json:"admission_state"`
	Mode                budgetRequired[string]       `json:"mode"`
	PolicyRevision      budgetRequired[budget.Count] `json:"policy_revision"`
	PolicyEpoch         budgetRequired[budget.Count] `json:"policy_epoch"`
	LimitTicks          budgetNullable[budget.Ticks] `json:"limit_ticks"`
	AttemptReserveTicks budgetNullable[budget.Ticks] `json:"attempt_reserve_ticks"`
	Reason              budgetRequired[string]       `json:"reason"`
	// The three fields below are only in the result of an enable, and the two
	// counts are left out when they are zero.
	ActivatedAt      budgetOptional[string]       `json:"activated_at"`
	MemberCount      budgetOptional[budgetNumber] `json:"member_count"`
	InFlightGapCount budgetOptional[budgetNumber] `json:"in_flight_gap_count"`
}

// budgetOperationState is the one field every answer to a query by key has.
// It decides which of the two structs below the answer is read into.
type budgetOperationState struct {
	State budgetRequired[string] `json:"state"`
}

// budgetOperationCommitted is the answer to a query by key when the operation
// committed.
type budgetOperationCommitted struct {
	State  budgetRequired[string]                `json:"state"`
	Stale  budgetRequired[bool]                  `json:"stale"`
	Result budgetRequired[budgetOperationResult] `json:"result"`
}

// budgetOperationUnknown is the answer to a query by key when no committed
// operation is visible under the key.
type budgetOperationUnknown struct {
	State     budgetRequired[string] `json:"state"`
	RetryPath budgetRequired[string] `json:"retry_path"`
}

// budgetIssueBinding says which budget root an issue belongs to.
type budgetIssueBinding struct {
	IssueID      budgetRequired[string] `json:"issue_id"`
	BudgetRootID budgetRequired[string] `json:"budget_root_id"`
	IsRootIssue  budgetRequired[bool]   `json:"is_root_issue"`
	BindSource   budgetRequired[string] `json:"bind_source"`
	BoundAt      budgetRequired[string] `json:"bound_at"`
}

// budgetShow is one budget root as `show` answers it.
type budgetShow struct {
	BudgetRootID        budgetRequired[string]             `json:"budget_root_id"`
	RootIssueID         budgetRequired[string]             `json:"root_issue_id"`
	AdmissionState      budgetRequired[string]             `json:"admission_state"`
	Mode                budgetRequired[string]             `json:"mode"`
	PolicyRevision      budgetRequired[budget.Count]       `json:"policy_revision"`
	PolicyEpoch         budgetRequired[budget.Count]       `json:"policy_epoch"`
	LedgerRevision      budgetRequired[budget.Count]       `json:"ledger_revision"`
	ActivatedAt         budgetRequired[string]             `json:"activated_at"`
	SettledAsOfRevision budgetNullable[budget.Count]       `json:"settled_as_of_revision"`
	LimitTicks          budgetNullable[budget.Ticks]       `json:"limit_ticks"`
	AttemptReserveTicks budgetNullable[budget.Ticks]       `json:"attempt_reserve_ticks"`
	LedgerState         budgetRequired[string]             `json:"ledger_state"`
	Amounts             budgetNullable[budgetShowAmounts]  `json:"amounts"`
	Unknown             budgetRequired[budgetShowUnknown]  `json:"unknown"`
	TotalKnown          budgetRequired[bool]               `json:"total_known"`
	RemainingTicks      budgetNullable[budget.Ticks]       `json:"remaining_ticks"`
	Coverage            budgetRequired[budgetShowCoverage] `json:"coverage"`
}

// budgetShowAmounts is the five amounts of a root. The whole object is null
// when the ledger cannot be read.
type budgetShowAmounts struct {
	ActualKnownTicks budgetRequired[budget.Ticks] `json:"actual_known_ticks"`
	EstimatedTicks   budgetRequired[budget.Ticks] `json:"estimated_ticks"`
	ReservedTicks    budgetRequired[budget.Ticks] `json:"reserved_ticks"`
	UnknownHoldTicks budgetRequired[budget.Ticks] `json:"unknown_hold_ticks"`
	OverrunTicks     budgetRequired[budget.Ticks] `json:"overrun_ticks"`
}

type budgetShowUnknown struct {
	ByReason      budgetRequired[[]budgetShowUnknownReason] `json:"by_reason"`
	PreActivation budgetRequired[budgetNumber]              `json:"pre_activation"`
}

type budgetShowUnknownReason struct {
	Reason budgetRequired[string]       `json:"reason"`
	Count  budgetRequired[budgetNumber] `json:"count"`
}

type budgetShowCoverage struct {
	IncludedAttempts budgetNullable[budgetNumber]             `json:"included_attempts"`
	NotApplicable    budgetRequired[[]budgetRequired[string]] `json:"not_applicable"`
}

// budgetObservationPage is one page of the observations of a root.
type budgetObservationPage struct {
	Observations budgetRequired[[]budgetObservation] `json:"observations"`
	NextCursor   budgetNullable[string]              `json:"next_cursor"`
}

type budgetObservation struct {
	ID              budgetRequired[string]           `json:"id"`
	Kind            budgetRequired[string]           `json:"kind"`
	Reason          budgetRequired[string]           `json:"reason"`
	IssueID         budgetNullable[string]           `json:"issue_id"`
	TaskID          budgetNullable[string]           `json:"task_id"`
	ClaimGeneration budgetNullable[budget.Count]     `json:"claim_generation"`
	AttemptID       budgetNullable[string]           `json:"attempt_id"`
	Covered         budgetRequired[bool]             `json:"covered"`
	Detail          budgetRequired[budgetJSONObject] `json:"detail"`
	CreatedAt       budgetRequired[string]           `json:"created_at"`
}

// budgetAttemptPage is one page of the executions of a root.
type budgetAttemptPage struct {
	LedgerState budgetRequired[string]          `json:"ledger_state"`
	Attempts    budgetRequired[[]budgetAttempt] `json:"attempts"`
	NextCursor  budgetNullable[string]          `json:"next_cursor"`
}

type budgetAttempt struct {
	AttemptID            budgetRequired[string]            `json:"attempt_id"`
	TaskID               budgetRequired[string]            `json:"task_id"`
	ClaimGeneration      budgetRequired[budget.Count]      `json:"claim_generation"`
	IssueID              budgetNullable[string]            `json:"issue_id"`
	ContextParentIssueID budgetNullable[string]            `json:"context_parent_issue_id"`
	AgentID              budgetRequired[string]            `json:"agent_id"`
	RuntimeID            budgetRequired[string]            `json:"runtime_id"`
	State                budgetRequired[string]            `json:"state"`
	ReservedTicks        budgetNullable[budget.Ticks]      `json:"reserved_ticks"`
	ProtocolVersion      budgetNullable[budgetNumber]      `json:"protocol_version"`
	CreatedAt            budgetRequired[string]            `json:"created_at"`
	Reservation          budgetNullable[budgetReservation] `json:"reservation"`
}

// budgetReservation is what the ledger holds under the reservation of one
// execution.
type budgetReservation struct {
	Status           budgetRequired[string]       `json:"status"`
	Unsized          budgetRequired[bool]         `json:"unsized"`
	ReservedTicks    budgetRequired[budget.Ticks] `json:"reserved_ticks"`
	ActualKnownTicks budgetRequired[budget.Ticks] `json:"actual_known_ticks"`
	UnspentTicks     budgetRequired[budget.Ticks] `json:"unspent_ticks"`
	OverrunTicks     budgetRequired[budget.Ticks] `json:"overrun_ticks"`
}

// budgetErrorBody is the body of a refusal by the budget handlers: one code,
// given under two names.
type budgetErrorBody struct {
	Error budgetRequired[string] `json:"error"`
	Code  budgetRequired[string] `json:"code"`
}

// The requests of design 12.9. They are encoded, never decoded, so their fields
// are plain. The order of the fields is the order on the wire, and a nil
// amount is sent as null: no key is ever left out.

// budgetEnableRequest is the body of POST /api/issues/{id}/budget.
type budgetEnableRequest struct {
	RequestKey          string        `json:"request_key"`
	Reason              string        `json:"reason"`
	LimitTicks          *budget.Ticks `json:"limit_ticks"`
	AttemptReserveTicks *budget.Ticks `json:"attempt_reserve_ticks"`
}

// budgetOperationRequest is the body of POST
// /api/issue-budgets/{rootId}/operations for close and reopen, and the first
// four fields of the two bodies below.
type budgetOperationRequest struct {
	Kind                   string       `json:"kind"`
	RequestKey             string       `json:"request_key"`
	Reason                 string       `json:"reason"`
	ExpectedPolicyRevision budget.Count `json:"expected_policy_revision"`
}

type budgetSetLimitRequest struct {
	budgetOperationRequest
	LimitTicks          *budget.Ticks `json:"limit_ticks"`
	AttemptReserveTicks *budget.Ticks `json:"attempt_reserve_ticks"`
}

type budgetSetModeRequest struct {
	budgetOperationRequest
	Mode string `json:"mode"`
}
