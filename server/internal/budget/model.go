// Package budget meters one parent-task budget root: decimal amounts, frozen
// price snapshots, and a ledger that replays immutable entries into a net
// projection. It performs no database, network, clock, permission, or
// provider operations, and uses integer and rational arithmetic only.
package budget

import "errors"

// TicksPerUSD is the amount scale: one tick is 1e-10 USD. It is the unit the
// agent adapters report as agent.CostUSDTicksPerUSD. It is declared here so
// this package stays free of the adapter package's dependencies; scale_test.go
// fails if the two ever differ.
const TicksPerUSD int64 = 10_000_000_000

const CurrencyUSD = "USD"

var (
	ErrInvalidDecimal         = errors.New("budget: not a canonical non-negative decimal string")
	ErrOverflow               = errors.New("budget: amount exceeds the signed 64-bit range")
	ErrNegativeAmount         = errors.New("budget: negative amount")
	ErrInvalidPrice           = errors.New("budget: invalid price snapshot")
	ErrUnsupportedCurrency    = errors.New("budget: unsupported currency")
	ErrInvalidEntry           = errors.New("budget: invalid ledger entry")
	ErrConflict               = errors.New("budget: entry identity reused with different content")
	ErrModeMismatch           = errors.New("budget: measurement mode is fixed within a meter scope")
	ErrUnknownReservation     = errors.New("budget: unknown reservation")
	ErrReleaseProofIncomplete = errors.New("budget: release requires final usage, a complete scope and proof of no further execution")
	ErrInvalidStoredForm      = errors.New("budget: not the stored form of an entry in this version")
)

// Ticks is a non-negative USD amount in ticks. Count is a non-negative
// quantity such as tokens. Both cross the wire as canonical decimal strings.
type (
	Ticks int64
	Count int64
)

// TokenCounts uses the provider's own split; Input excludes cached input.
type TokenCounts struct {
	Input      Count `json:"input"`
	Output     Count `json:"output"`
	CacheRead  Count `json:"cache_read"`
	CacheWrite Count `json:"cache_write"`
}

type EntryKind string

const (
	// KindReserve opens a shadow reservation for one execution attempt, with
	// an amount or, when the entry is Unsized, without one.
	KindReserve EntryKind = "reserve"
	// KindUsage is one metered observation on a meter scope.
	KindUsage EntryKind = "usage"
	// KindHold records a lifecycle fact (cancel, timeout, lost contact). It
	// turns the unspent reservation into an unknown hold and never releases.
	KindHold EntryKind = "hold"
	// KindSettle ends a reservation and releases what is left. It is the only
	// entry that releases allowance and it requires a complete ReleaseProof.
	KindSettle EntryKind = "settle"
	// KindGap records an included execution or period with no usable usage.
	KindGap EntryKind = "gap"
	// KindExclusion names a mechanism outside the root's coverage.
	KindExclusion EntryKind = "exclusion"
)

type MeterMode string

const (
	// ModeCumulative: each event carries the scope's running total.
	ModeCumulative MeterMode = "cumulative"
	// ModeDelta: each event carries its own increment.
	ModeDelta MeterMode = "delta"
)

// Overlap says whether an observation is its own meter scope.
type Overlap string

const (
	OverlapNone Overlap = ""
	// OverlapRollup is a proven summary of other scopes: kept for comparison,
	// contributes nothing.
	OverlapRollup Overlap = "rollup"
	// OverlapUnproven may include other scopes' usage and cannot be split.
	OverlapUnproven Overlap = "unproven"
)

type CostBasis string

const (
	// CostAbsent is the zero value: no amount was reported. It is unknown,
	// never zero.
	CostAbsent    CostBasis = ""
	CostReported  CostBasis = "reported"
	CostEstimated CostBasis = "estimated"
)

// Cost is the amount attached to one observation. ZeroBasis names the evidence
// that a reported zero is a real zero charge. PriceVersion and PriceDigest
// record the frozen snapshot behind a server-computed estimate.
type Cost struct {
	Basis        CostBasis `json:"basis"`
	Ticks        Ticks     `json:"ticks"`
	Currency     string    `json:"currency"`
	ZeroBasis    string    `json:"zero_basis"`
	PriceVersion string    `json:"price_version"`
	PriceDigest  string    `json:"price_digest"`
}

// Usage is one observation. ScopeID is the provider's real metering range
// (request, turn or session); InvocationID is the provider call that produced
// the observation. CorrectionAuthorized is decided by the server from the
// source, never by the reporter.
type Usage struct {
	ScopeID              string      `json:"scope_id"`
	InvocationID         string      `json:"invocation_id"`
	Mode                 MeterMode   `json:"mode"`
	Sequence             Count       `json:"sequence"`
	Overlap              Overlap     `json:"overlap"`
	BaselineEventID      string      `json:"baseline_event_id"`
	Supersedes           string      `json:"supersedes"`
	CorrectionAuthorized bool        `json:"correction_authorized"`
	Tokens               TokenCounts `json:"tokens"`
	Cost                 Cost        `json:"cost"`
}

type HoldReason string

const (
	HoldCancelled          HoldReason = "cancelled"
	HoldTimeout            HoldReason = "timeout"
	HoldProcessExit        HoldReason = "process_exit"
	HoldLostContact        HoldReason = "lost_contact"
	HoldSupersededUnproven HoldReason = "superseded_unproven"
)

// ReleaseProof lists what a settlement must prove. A queue row disappearing,
// an aborted request, an expired TTL or a re-claim proves none of them.
type ReleaseProof struct {
	FinalUsage         bool `json:"final_usage"`
	ScopeComplete      bool `json:"scope_complete"`
	NoFurtherExecution bool `json:"no_further_execution"`
}

// Entry is one immutable ledger record. Only the fields of its Kind may be
// set. Entries are comparable: the same ID with equal content is a replay,
// the same ID with different content is a conflict.
type Entry struct {
	ID            string    `json:"id"`
	Kind          EntryKind `json:"kind"`
	ReservationID string    `json:"reservation_id"`
	ReservedTicks Ticks     `json:"reserved_ticks"`
	// Unsized marks a reserve entry that has no amount: the root policy sets
	// no per-attempt reservation. ReservedTicks is then zero, which stands for
	// "not sized" and is not a reservation of zero.
	Unsized    bool          `json:"unsized"`
	HoldReason HoldReason    `json:"hold_reason"`
	Proof      ReleaseProof  `json:"proof"`
	GapReason  UnknownReason `json:"gap_reason"`
	Detail     string        `json:"detail"`
	Usage      Usage         `json:"usage"`
}

type UnknownReason string

const (
	ReasonCostAbsent           UnknownReason = "cost_absent"
	ReasonZeroWithoutBasis     UnknownReason = "zero_without_basis"
	ReasonUnsupportedCurrency  UnknownReason = "unsupported_currency"
	ReasonEstimateOnly         UnknownReason = "estimate_only"
	ReasonSequenceRegression   UnknownReason = "sequence_regression"
	ReasonSequenceConflict     UnknownReason = "sequence_conflict"
	ReasonCorrectionUnresolved UnknownReason = "correction_unresolved"
	ReasonScopeAmbiguity       UnknownReason = "scope_ambiguity"
	ReasonCoverageGap          UnknownReason = "coverage_gap"
	ReasonPreActivationUnknown UnknownReason = "pre_activation_unknown"
	// ReasonReservationUnsized: a reservation without an amount has not
	// settled. Nothing bounds what its attempt may still spend.
	ReasonReservationUnsized UnknownReason = "reservation_unsized"
	// Lifecycle reasons reuse the HoldReason strings.
)

// UnknownItem is one member of the unknown set. It carries a reason and its
// subject, never an amount: unknown spend has no number.
type UnknownItem struct {
	Reason        UnknownReason `json:"reason"`
	EntryID       string        `json:"entry_id"`
	ReservationID string        `json:"reservation_id"`
	ScopeID       string        `json:"scope_id"`
	Detail        string        `json:"detail"`
}

type Disposition string

const (
	DispositionAccepted Disposition = "accepted"
	// DispositionStale: kept as a record, changes nothing.
	DispositionStale Disposition = "stale"
	// DispositionQuarantined: kept as a record, contributes nothing, adds an
	// unknown item.
	DispositionQuarantined Disposition = "quarantined"
	// DispositionComparison: a proven rollup, kept for comparison only.
	DispositionComparison Disposition = "comparison"
)

type CostState string

const (
	CostStateNone      CostState = ""
	CostStateActual    CostState = "actual"
	CostStateKnownZero CostState = "known_zero"
	CostStateEstimated CostState = "estimated"
	CostStateUnknown   CostState = "unknown"
)

// Receipt is the committed outcome of one entry. Replaying the entry returns
// the same receipt with Replayed set and changes nothing else.
type Receipt struct {
	EntryID          string        `json:"entry_id"`
	Kind             EntryKind     `json:"kind"`
	Disposition      Disposition   `json:"disposition"`
	CostState        CostState     `json:"cost_state"`
	Reason           UnknownReason `json:"reason"`
	ActualAdded      Ticks         `json:"actual_added_ticks"`
	ActualRemoved    Ticks         `json:"actual_removed_ticks"`
	EstimatedAdded   Ticks         `json:"estimated_added_ticks"`
	EstimatedRemoved Ticks         `json:"estimated_removed_ticks"`
	Released         Ticks         `json:"released_ticks"`
	Replayed         bool          `json:"replayed"`
}

// Coverage separates mechanism-level exclusions from included attempts.
// Exclusions are neither in the denominator nor a zero cost.
type Coverage struct {
	IncludedAttempts int      `json:"included_attempts"`
	NotApplicable    []string `json:"not_applicable"`
}

// Projection is the net state of a ledger. One unit of allowance is counted
// in at most one of ActualKnown, Reserved and UnknownHold. UnknownHold is the
// unspent part of reservations whose outcome is unproven; it is not the amount
// of the unknown spend.
type Projection struct {
	ActualKnown Ticks         `json:"actual_known_ticks"`
	Estimated   Ticks         `json:"estimated_ticks"`
	Reserved    Ticks         `json:"reserved_ticks"`
	UnknownHold Ticks         `json:"unknown_hold_ticks"`
	Overrun     Ticks         `json:"overrun_ticks"`
	Unknown     []UnknownItem `json:"unknown"`
	Coverage    Coverage      `json:"coverage"`
}

// ReservationStatus is where a reservation stands.
type ReservationStatus string

const (
	// ReservationOpen: the attempt may still spend. What it has not spent is
	// reserved.
	ReservationOpen ReservationStatus = "open"
	// ReservationHeld: a lifecycle fact ended the attempt without proof of its
	// final usage. What it has not spent is an unknown hold.
	ReservationHeld ReservationStatus = "held"
	// ReservationSettled: a settlement ended the reservation and released what
	// it had not spent.
	ReservationSettled ReservationStatus = "settled"
)

// ReservationProjection is the net state of one reservation, as Projection is
// of the ledger. Over the reservations of a ledger, Unspent adds up to
// Projection.Reserved for the open ones and to Projection.UnknownHold for the
// held ones, Overrun adds up to Projection.Overrun, and ActualKnown, with the
// known actual of usage that names no reservation, to Projection.ActualKnown.
type ReservationProjection struct {
	Status ReservationStatus `json:"status"`
	// Unsized: the reservation was opened without an amount. Reserved, Unspent
	// and Overrun are then zero: there is no amount to leave unspent or to
	// exceed.
	Unsized bool `json:"unsized"`
	// Reserved is the amount of the reserve entry.
	Reserved Ticks `json:"reserved_ticks"`
	// ActualKnown is the known actual spend recorded against the reservation.
	ActualKnown Ticks `json:"actual_known_ticks"`
	// Unspent is the part of Reserved that still occupies allowance: Reserved
	// less ActualKnown, never below zero, until the reservation settles, and
	// zero from then on.
	Unspent Ticks `json:"unspent_ticks"`
	// Overrun is ActualKnown beyond Reserved.
	Overrun Ticks `json:"overrun_ticks"`
}
