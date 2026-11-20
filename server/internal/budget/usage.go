package budget

import (
	"fmt"
	"maps"
)

// scope is the state of one meter scope. The cumulative fields are unused in
// delta mode, where each event stands on its own.
type scope struct {
	mode MeterMode
	// The head is the accepted observation with the highest source sequence.
	headID         string
	headSeq        int64
	headInvocation string
	headTokens     TokenCounts
	// sequences holds every source sequence an entry has claimed, whatever
	// became of the entry.
	sequences map[int64]struct{}
	// actual is the latest accepted reported running total.
	hasActual bool
	actual    int64
	// estimateTotal is the latest accepted estimated running total that no
	// reported total has covered since.
	estimateTotal int64
	// actualBy attributes the scope's actual to reservations ("" for none), so
	// a session that spans attempts charges each one its own part.
	actualBy map[string]int64
}

func (sc *scope) clone() *scope {
	next := *sc
	next.sequences = maps.Clone(sc.sequences)
	next.actualBy = maps.Clone(sc.actualBy)
	return &next
}

// estimated is the part of the estimated running total beyond known actual,
// so the same usage is never in both figures.
func (sc *scope) estimated() int64 {
	if sc.estimateTotal <= sc.actual {
		return 0
	}
	return sc.estimateTotal - sc.actual
}

// contribution is what one accepted delta event currently adds.
type contribution struct {
	scopeID       string
	reservationID string
	actual        int64
	estimated     int64
	superseded    bool
}

func validateUsage(e Entry) error {
	u := e.Usage
	invalid := func(why string) error {
		return fmt.Errorf("%w: entry %q: %s", ErrInvalidEntry, e.ID, why)
	}
	for _, v := range [6]int64{
		int64(u.Tokens.Input), int64(u.Tokens.Output), int64(u.Tokens.CacheRead), int64(u.Tokens.CacheWrite),
		int64(u.Sequence), int64(u.Cost.Ticks),
	} {
		if v < 0 {
			return fmt.Errorf("%w: entry %q", ErrNegativeAmount, e.ID)
		}
	}
	if u.ScopeID == "" || u.InvocationID == "" {
		return invalid("usage needs a meter scope and an invocation")
	}
	switch u.Mode {
	case ModeCumulative:
		if u.Sequence == 0 {
			return invalid("a cumulative observation needs a source sequence")
		}
	case ModeDelta:
		if u.BaselineEventID != "" {
			return invalid("a baseline applies to cumulative observations only")
		}
	default:
		return invalid("unknown measurement mode")
	}
	switch u.Overlap {
	case OverlapNone:
	case OverlapRollup, OverlapUnproven:
		if u.Supersedes != "" || u.BaselineEventID != "" {
			return invalid("an overlapping observation cannot carry a baseline or a correction")
		}
	default:
		return invalid("unknown overlap")
	}
	if u.Supersedes == e.ID || (u.CorrectionAuthorized && u.Supersedes == "") {
		return invalid("a correction names another entry")
	}
	switch c := u.Cost; c.Basis {
	case CostAbsent:
		if c != (Cost{}) {
			return invalid("an absent cost carries no amount")
		}
	case CostReported:
		if c.PriceVersion != "" || c.PriceDigest != "" {
			return invalid("a reported cost has no price snapshot")
		}
		if c.Ticks > 0 && c.ZeroBasis != "" {
			return invalid("zero basis on a non-zero cost")
		}
	case CostEstimated:
		if c.ZeroBasis != "" {
			return invalid("zero basis on an estimate")
		}
	default:
		return invalid("unknown cost basis")
	}
	return nil
}

// classifyCost keeps the presence states apart: absent, a zero without
// evidence and an unsupported currency are unknown, never zero.
func classifyCost(c Cost) (CostState, UnknownReason) {
	switch {
	case c.Basis == CostAbsent:
		return CostStateUnknown, ReasonCostAbsent
	case c.Currency != CurrencyUSD:
		return CostStateUnknown, ReasonUnsupportedCurrency
	case c.Basis == CostEstimated:
		return CostStateEstimated, ReasonEstimateOnly
	case c.Ticks > 0:
		return CostStateActual, ""
	case c.ZeroBasis != "":
		return CostStateKnownZero, ""
	}
	return CostStateUnknown, ReasonZeroWithoutBasis
}

func isCostQualityReason(reason UnknownReason) bool {
	switch reason {
	case ReasonCostAbsent, ReasonZeroWithoutBasis, ReasonUnsupportedCurrency, ReasonEstimateOnly:
		return true
	}
	return false
}

func (l *Ledger) applyUsage(e Entry) (Receipt, error) {
	u := e.Usage
	state, reason := classifyCost(u.Cost)
	switch u.Overlap {
	case OverlapRollup:
		// A proven summary of other scopes is kept for comparison only.
		return Receipt{Disposition: DispositionComparison, CostState: state}, nil
	case OverlapUnproven:
		return l.quarantine(e, state, ReasonScopeAmbiguity), nil
	}
	sc, ok := l.scopes[u.ScopeID]
	if !ok {
		sc = &scope{mode: u.Mode, sequences: map[int64]struct{}{}, actualBy: map[string]int64{}}
		l.scopes[u.ScopeID] = sc
	} else if sc.mode != u.Mode {
		return Receipt{}, fmt.Errorf("%w: scope %q is %s, entry %q is %s", ErrModeMismatch, u.ScopeID, sc.mode, e.ID, u.Mode)
	}
	if u.Mode == ModeDelta {
		return l.applyDelta(e, state, reason)
	}
	return l.applyCumulative(sc, e, state, reason)
}

// quarantine keeps the entry as a record that contributes nothing and adds
// an unknown item.
func (l *Ledger) quarantine(e Entry, state CostState, reason UnknownReason) Receipt {
	l.addUnknown(UnknownItem{Reason: reason, EntryID: e.ID, ReservationID: e.ReservationID, ScopeID: e.Usage.ScopeID})
	return Receipt{Disposition: DispositionQuarantined, CostState: state, Reason: reason}
}

func (l *Ledger) addCostUnknown(e Entry, reason UnknownReason) {
	l.addUnknown(UnknownItem{Reason: reason, EntryID: e.ID, ReservationID: e.ReservationID, ScopeID: e.Usage.ScopeID})
}

func tokensBelow(a, b TokenCounts) bool {
	return a.Input < b.Input || a.Output < b.Output || a.CacheRead < b.CacheRead || a.CacheWrite < b.CacheWrite
}

func isReported(state CostState) bool {
	return state == CostStateActual || state == CostStateKnownZero
}

func (l *Ledger) applyCumulative(sc *scope, e Entry, state CostState, reason UnknownReason) (Receipt, error) {
	u := e.Usage
	seq, value := int64(u.Sequence), int64(u.Cost.Ticks)
	correction := u.Supersedes != ""

	if _, taken := sc.sequences[seq]; taken {
		return l.quarantine(e, state, ReasonSequenceConflict), nil
	}
	sc.sequences[seq] = struct{}{}

	switch {
	case seq < sc.headSeq:
		// Order is the source sequence, never arrival. An older value is a
		// record only, unless it is larger than what came after it.
		if correction {
			return l.quarantine(e, state, ReasonCorrectionUnresolved), nil
		}
		above := tokensBelow(sc.headTokens, u.Tokens) ||
			(isReported(state) && sc.hasActual && value > sc.actual) ||
			(state == CostStateEstimated && sc.estimateTotal > 0 && value > sc.estimateTotal)
		if above {
			return l.quarantine(e, state, ReasonSequenceRegression), nil
		}
		return Receipt{Disposition: DispositionStale, CostState: state}, nil
	case correction:
		usable := isReported(state) || (state == CostStateEstimated && !sc.hasActual)
		if !u.CorrectionAuthorized || sc.headID == "" || u.Supersedes != sc.headID || !usable {
			return l.quarantine(e, state, ReasonCorrectionUnresolved), nil
		}
		// A correction is never applied to another attempt: it must come
		// under the reservation its original charged.
		if l.entries[sc.headID].ReservationID != e.ReservationID {
			return l.quarantine(e, state, ReasonCorrectionUnresolved), nil
		}
		// Lowering the scope takes the difference back from that
		// reservation, which cannot go below what the scope gave it.
		if isReported(state) && sc.actualBy[e.ReservationID]+value-sc.actual < 0 {
			return l.quarantine(e, state, ReasonCorrectionUnresolved), nil
		}
	default:
		// A running total continues the head. Across invocations that needs
		// the adapter's proof of which observation it starts from.
		if u.BaselineEventID != "" && u.BaselineEventID != sc.headID {
			return l.quarantine(e, state, ReasonScopeAmbiguity), nil
		}
		if u.BaselineEventID == "" && sc.headID != "" && sc.headInvocation != u.InvocationID {
			return l.quarantine(e, state, ReasonScopeAmbiguity), nil
		}
		below := tokensBelow(u.Tokens, sc.headTokens) ||
			(isReported(state) && value < sc.actual) ||
			(state == CostStateEstimated && value < sc.estimateTotal)
		if below {
			return l.quarantine(e, state, ReasonSequenceRegression), nil
		}
	}

	receipt := Receipt{Disposition: DispositionAccepted, CostState: state}
	estimatedBefore := sc.estimated()
	switch {
	case isReported(state):
		change := value - sc.actual
		if err := l.addActual(e.ReservationID, change); err != nil {
			return Receipt{}, err
		}
		sc.actualBy[e.ReservationID] += change
		sc.actual, sc.hasActual, sc.estimateTotal = value, true, 0
		// A reported running total covers the scope from its start.
		l.dropUnknown(func(item UnknownItem) bool {
			return item.ScopeID == u.ScopeID && isCostQualityReason(item.Reason)
		})
		receipt.ActualAdded, receipt.ActualRemoved = net(change)
	case state == CostStateEstimated:
		if correction {
			l.dropUnknown(func(item UnknownItem) bool { return item.EntryID == sc.headID })
		}
		sc.estimateTotal = value
		l.addCostUnknown(e, reason)
		receipt.Reason = reason
	default:
		l.addCostUnknown(e, reason)
		receipt.Reason = reason
	}
	estimatedChange := sc.estimated() - estimatedBefore
	if err := l.addEstimated(estimatedChange); err != nil {
		return Receipt{}, err
	}
	receipt.EstimatedAdded, receipt.EstimatedRemoved = net(estimatedChange)
	sc.headID, sc.headSeq, sc.headInvocation, sc.headTokens = e.ID, seq, u.InvocationID, u.Tokens
	return receipt, nil
}

func (l *Ledger) applyDelta(e Entry, state CostState, reason UnknownReason) (Receipt, error) {
	u := e.Usage
	var replaced contribution
	if u.Supersedes != "" {
		target, ok := l.contributions[u.Supersedes]
		// A correction stays in its original's scope and under the
		// reservation its original charged; it never moves an amount to
		// another attempt.
		if !u.CorrectionAuthorized || !ok || target.scopeID != u.ScopeID || target.reservationID != e.ReservationID || target.superseded {
			return l.quarantine(e, state, ReasonCorrectionUnresolved), nil
		}
		// A correction replaces everything its original contributed, once.
		if err := l.addActual(target.reservationID, -target.actual); err != nil {
			return Receipt{}, err
		}
		if err := l.addEstimated(-target.estimated); err != nil {
			return Receipt{}, err
		}
		l.dropUnknown(func(item UnknownItem) bool { return item.EntryID == u.Supersedes })
		replaced = target
		target.superseded = true
		l.contributions[u.Supersedes] = target
	}

	receipt := Receipt{Disposition: DispositionAccepted, CostState: state}
	added := contribution{scopeID: u.ScopeID, reservationID: e.ReservationID}
	switch state {
	case CostStateActual:
		added.actual = int64(u.Cost.Ticks)
		if err := l.addActual(e.ReservationID, added.actual); err != nil {
			return Receipt{}, err
		}
	case CostStateKnownZero:
	case CostStateEstimated:
		added.estimated = int64(u.Cost.Ticks)
		if err := l.addEstimated(added.estimated); err != nil {
			return Receipt{}, err
		}
		l.addCostUnknown(e, reason)
		receipt.Reason = reason
	default:
		l.addCostUnknown(e, reason)
		receipt.Reason = reason
	}
	l.contributions[e.ID] = added
	receipt.ActualAdded, receipt.ActualRemoved = net(added.actual - replaced.actual)
	receipt.EstimatedAdded, receipt.EstimatedRemoved = net(added.estimated - replaced.estimated)
	return receipt, nil
}

// net splits a signed change into the added and removed parts of a receipt.
func net(change int64) (added, removed Ticks) {
	if change < 0 {
		return 0, Ticks(-change)
	}
	return Ticks(change), 0
}
