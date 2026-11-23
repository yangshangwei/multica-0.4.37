package budget

import (
	"errors"
	"reflect"
	"testing"
)

// b25 builds one cumulative scope whose running total went 100 then 150.
func b25(t *testing.T) (*Ledger, []Receipt) {
	t.Helper()
	l := NewLedger()
	receipts := apply(t, l,
		cumulative("v1", "", "scope-1", 1, reported(100)),
		cumulative("v2", "", "scope-1", 2, reported(150)),
	)
	return l, receipts
}

// B15: child calls 40 and 60, a proven parent rollup of 100, and the parent's
// own call of 15 give A = 115. The rollup is a comparison record only.
func TestB15ProvenRollupContributesNothing(t *testing.T) {
	rollup := delta("rollup-1", "", "parent-rollup", reported(100))
	rollup.Usage.Overlap = OverlapRollup
	l := NewLedger()
	receipts := apply(t, l,
		delta("s1", "", "child-1", reported(40)),
		delta("s2", "", "child-2", reported(60)),
		rollup,
		delta("p", "", "parent-own", reported(15)),
	)
	if r := receipts[2]; r.Disposition != DispositionComparison || r.ActualAdded != 0 || r.Reason != "" {
		t.Fatalf("rollup receipt = %+v", r)
	}
	p := l.Projection()
	wantFigures(t, p, 115, 0, 0, 0, 0)
	wantUnknown(t, p)
}

func TestB15UnprovenRollupIsAmbiguousNotActual(t *testing.T) {
	unproven := delta("rollup-1", "", "parent-rollup", reported(100))
	unproven.Usage.Overlap = OverlapUnproven
	l := NewLedger()
	receipts := apply(t, l,
		delta("s1", "", "child-1", reported(40)),
		delta("s2", "", "child-2", reported(60)),
		unproven,
		delta("p", "", "parent-own", reported(15)),
	)
	if r := receipts[2]; r.Disposition != DispositionQuarantined || r.ActualAdded != 0 || r.Reason != ReasonScopeAmbiguity {
		t.Fatalf("unproven rollup receipt = %+v", r)
	}
	p := l.Projection()
	wantFigures(t, p, 115, 0, 0, 0, 0) // not 215
	wantUnknown(t, p, ReasonScopeAmbiguity)
	if p.Unknown[0].EntryID != "rollup-1" || p.Unknown[0].ScopeID != "parent-rollup" {
		t.Fatalf("unknown item = %+v", p.Unknown[0])
	}
}

// B21: c1 has 100 tokens and no cost; c2 has 50 tokens and cost 30. A = 30 and
// c1 stays in the unknown set; the positive cost does not cover it.
func TestB21MissingCostStaysUnknownBesideAKnownCost(t *testing.T) {
	c1 := delta("c1", "", "call-1", Cost{})
	c1.Usage.Tokens = TokenCounts{Input: 100}
	c2 := delta("c2", "", "call-2", reported(30))
	c2.Usage.Tokens = TokenCounts{Input: 50}
	l := NewLedger()
	receipts := apply(t, l, c1, c2)
	if receipts[0].CostState != CostStateUnknown || receipts[0].Reason != ReasonCostAbsent || receipts[1].CostState != CostStateActual {
		t.Fatalf("receipts = %+v", receipts)
	}
	p := l.Projection()
	wantFigures(t, p, 30, 0, 0, 0, 0)
	wantUnknown(t, p, ReasonCostAbsent)
	if p.Unknown[0] != (UnknownItem{Reason: ReasonCostAbsent, EntryID: "c1", ScopeID: "call-1"}) {
		t.Fatalf("unknown item = %+v", p.Unknown[0])
	}
	if p.TotalKnown() {
		t.Fatal("projection claims every call was costed")
	}
	wantRemaining(t, p, 250, nil)
}

// B22: absent, a real zero, a zero without evidence, a negative amount and a
// non-USD amount are five different outcomes.
func TestB22CostPresenceStatesStayDistinct(t *testing.T) {
	l := NewLedger()
	apply(t, l, delta("paid", "", "scope-paid", reported(7)))

	absent := apply(t, l, delta("absent", "", "scope-absent", Cost{}))[0]
	if absent.CostState != CostStateUnknown || absent.Reason != ReasonCostAbsent || absent.ActualAdded != 0 {
		t.Fatalf("absent = %+v", absent)
	}

	knownZero := apply(t, l, delta("free", "", "scope-free", Cost{Basis: CostReported, Currency: "USD", ZeroBasis: "provider-free-tier"}))[0]
	if knownZero.CostState != CostStateKnownZero || knownZero.Reason != "" || knownZero.Disposition != DispositionAccepted || knownZero.ActualAdded != 0 {
		t.Fatalf("known zero = %+v", knownZero)
	}

	bareZero := apply(t, l, delta("bare-zero", "", "scope-bare", Cost{Basis: CostReported, Currency: "USD"}))[0]
	if bareZero.CostState != CostStateUnknown || bareZero.Reason != ReasonZeroWithoutBasis || bareZero.ActualAdded != 0 {
		t.Fatalf("zero without basis = %+v", bareZero)
	}

	before := l.Projection()
	if _, err := l.apply(delta("negative", "", "scope-negative", Cost{Basis: CostReported, Ticks: -5, Currency: "USD"})); !errors.Is(err, ErrNegativeAmount) {
		t.Fatalf("negative error = %v, want ErrNegativeAmount", err)
	}
	if after := l.Projection(); !reflect.DeepEqual(after, before) {
		t.Fatalf("a rejected negative amount changed the projection to %+v", after)
	}

	foreign := apply(t, l, delta("eur", "", "scope-eur", Cost{Basis: CostReported, Ticks: 500, Currency: "EUR"}))[0]
	if foreign.CostState != CostStateUnknown || foreign.Reason != ReasonUnsupportedCurrency || foreign.ActualAdded != 0 {
		t.Fatalf("non-USD = %+v", foreign)
	}

	p := l.Projection()
	wantFigures(t, p, 7, 0, 0, 0, 0)
	wantUnknown(t, p, ReasonCostAbsent, ReasonZeroWithoutBasis, ReasonUnsupportedCurrency)
	counts := p.UnknownCounts()
	if len(counts) != 3 || counts[ReasonCostAbsent] != 1 || counts[ReasonZeroWithoutBasis] != 1 || counts[ReasonUnsupportedCurrency] != 1 {
		t.Fatalf("unknown counts = %v", counts)
	}
}

// B25: running totals 100 then 150 contribute 100 and then only the 50 more.
func TestB25CumulativeAppendsOnlyTheDifference(t *testing.T) {
	l, receipts := b25(t)
	if receipts[0].ActualAdded != 100 || receipts[1].ActualAdded != 50 || receipts[1].ActualRemoved != 0 {
		t.Fatalf("receipts = %+v", receipts)
	}
	wantFigures(t, l.Projection(), 150, 0, 0, 0, 0)
	// Both raw observations are still held, with their own values.
	if l.entries["v1"].Usage.Cost.Ticks != 100 || l.entries["v2"].Usage.Cost.Ticks != 150 {
		t.Fatalf("raw observations = %+v, %+v", l.entries["v1"], l.entries["v2"])
	}
}

// B26: resending v2 returns its original receipt; a late v1 changes nothing.
func TestB26DuplicateAndLateOlderValueChangeNothing(t *testing.T) {
	l, receipts := b25(t)
	again := apply(t, l, cumulative("v2", "", "scope-1", 2, reported(150)))[0]
	if !again.Replayed {
		t.Fatalf("resend was applied again: %+v", again)
	}
	again.Replayed = false
	if again != receipts[1] {
		t.Fatalf("resend receipt = %+v, want the original %+v", again, receipts[1])
	}
	wantFigures(t, l.Projection(), 150, 0, 0, 0, 0)

	late := apply(t, l, cumulative("v1", "", "scope-1", 1, reported(100)))[0]
	if !late.Replayed || late.ActualAdded != 100 {
		t.Fatalf("late redelivery = %+v", late)
	}
	p := l.Projection()
	wantFigures(t, p, 150, 0, 0, 0, 0)
	wantUnknown(t, p)
}

// B26: order is by source sequence, not arrival. v2 arrives first; v1 arrives
// afterwards for the first time and is kept as stale without subtracting.
func TestB26LateOlderValueIsNotOrderedByArrival(t *testing.T) {
	l := NewLedger()
	apply(t, l, cumulative("v2", "", "scope-1", 2, reported(150)))
	late := apply(t, l, cumulative("v1", "", "scope-1", 1, reported(100)))[0]
	if late.Disposition != DispositionStale || late.ActualAdded != 0 || late.ActualRemoved != 0 || late.Reason != "" {
		t.Fatalf("late older value = %+v", late)
	}
	p := l.Projection()
	wantFigures(t, p, 150, 0, 0, 0, 0)
	wantUnknown(t, p)
	if l.entries["v1"].Usage.Cost.Ticks != 100 {
		t.Fatal("the late observation was not kept")
	}
	// A second claimant of an occupied source sequence is quarantined.
	odd := apply(t, l, cumulative("v1-other", "", "scope-1", 1, reported(100)))[0]
	if odd.Disposition != DispositionQuarantined || odd.Reason != ReasonSequenceConflict {
		t.Fatalf("second claimant of sequence 1 = %+v", odd)
	}
}

// B27: a running total that drops to 130 with no correction basis is
// quarantined. A stays 150 and the unknown set grows.
func TestB27UnexplainedRegressionIsQuarantined(t *testing.T) {
	l, _ := b25(t)
	drop := apply(t, l, cumulative("v3", "", "scope-1", 3, reported(130)))[0]
	if drop.Disposition != DispositionQuarantined || drop.Reason != ReasonSequenceRegression || drop.ActualAdded != 0 || drop.ActualRemoved != 0 {
		t.Fatalf("regression receipt = %+v", drop)
	}
	p := l.Projection()
	wantFigures(t, p, 150, 0, 0, 0, 0)
	wantUnknown(t, p, ReasonSequenceRegression)
	wantRemaining(t, p, 250, nil)

	// The quarantined value never becomes the comparison point: 180 is 30 more
	// than the accepted 150, not 50 more than 130.
	next := apply(t, l, cumulative("v4", "", "scope-1", 4, reported(180)))[0]
	if next.ActualAdded != 30 {
		t.Fatalf("next receipt = %+v", next)
	}
	wantFigures(t, l.Projection(), 180, 0, 0, 0, 0)
}

func TestLateOlderValueAboveTheHeadIsQuarantined(t *testing.T) {
	l := NewLedger()
	apply(t, l, cumulative("v2", "", "scope-1", 2, reported(150)))
	odd := apply(t, l, cumulative("v1", "", "scope-1", 1, reported(170)))[0]
	if odd.Disposition != DispositionQuarantined || odd.Reason != ReasonSequenceRegression {
		t.Fatalf("receipt = %+v", odd)
	}
	p := l.Projection()
	wantFigures(t, p, 150, 0, 0, 0, 0)
	wantUnknown(t, p, ReasonSequenceRegression)
}

// B28: an authorised correction that supersedes v2 lowers the net to 130. The
// originals stay readable and replaying the correction subtracts nothing more.
func TestB28ExplicitCorrectionLowersNetAndKeepsOriginals(t *testing.T) {
	l, _ := b25(t)
	fix := cumulative("v3", "", "scope-1", 3, reported(130))
	fix.Usage.Supersedes = "v2"
	fix.Usage.CorrectionAuthorized = true

	receipt := apply(t, l, fix)[0]
	if receipt.Disposition != DispositionAccepted || receipt.ActualRemoved != 20 || receipt.ActualAdded != 0 {
		t.Fatalf("correction receipt = %+v", receipt)
	}
	p := l.Projection()
	wantFigures(t, p, 130, 0, 0, 0, 0)
	wantUnknown(t, p)
	if l.entries["v1"].Usage.Cost.Ticks != 100 || l.entries["v2"].Usage.Cost.Ticks != 150 {
		t.Fatal("the corrected originals were not kept")
	}

	again := apply(t, l, fix)[0]
	if !again.Replayed || again.ActualRemoved != 20 {
		t.Fatalf("correction replay = %+v", again)
	}
	wantFigures(t, l.Projection(), 130, 0, 0, 0, 0)
}

func TestCorrectionWithoutAuthorityOrChainIsQuarantined(t *testing.T) {
	for name, mutate := range map[string]func(*Usage){
		"no authority":       func(u *Usage) { u.Supersedes = "v2" },
		"not the head":       func(u *Usage) { u.Supersedes, u.CorrectionAuthorized = "v1", true },
		"unknown original":   func(u *Usage) { u.Supersedes, u.CorrectionAuthorized = "v9", true },
		"other scope's head": func(u *Usage) { u.ScopeID, u.Supersedes, u.CorrectionAuthorized = "scope-2", "v2", true },
	} {
		l, _ := b25(t)
		fix := cumulative("v3", "", "scope-1", 3, reported(130))
		mutate(&fix.Usage)
		receipt := apply(t, l, fix)[0]
		if receipt.Disposition != DispositionQuarantined || receipt.Reason != ReasonCorrectionUnresolved || receipt.ActualRemoved != 0 {
			t.Errorf("%s: receipt = %+v", name, receipt)
		}
		p := l.Projection()
		wantFigures(t, p, 150, 0, 0, 0, 0)
		wantUnknown(t, p, ReasonCorrectionUnresolved)
	}
}

// B29: deltas 40 and 60 with 40 resent give 100. A cumulative event in the
// same scope is rejected because the mode is fixed per scope.
func TestB29DeltaCountsEachEventOnceAndModeIsFixed(t *testing.T) {
	l := NewLedger()
	d1 := delta("d1", "", "scope-1", reported(40))
	receipts := apply(t, l, d1, delta("d2", "", "scope-1", reported(60)), d1)
	if receipts[0].ActualAdded != 40 || receipts[1].ActualAdded != 60 || !receipts[2].Replayed {
		t.Fatalf("receipts = %+v", receipts)
	}
	wantFigures(t, l.Projection(), 100, 0, 0, 0, 0)

	mixed := cumulative("c1", "", "scope-1", 1, reported(500))
	if _, err := l.apply(mixed); !errors.Is(err, ErrModeMismatch) {
		t.Fatalf("mixed mode error = %v, want ErrModeMismatch", err)
	}
	p := l.Projection()
	wantFigures(t, p, 100, 0, 0, 0, 0)
	wantUnknown(t, p)
	// The rejected event left no record: its ID is free for a valid event.
	if r := apply(t, l, delta("c1", "", "scope-1", reported(5)))[0]; r.Replayed || r.ActualAdded != 5 {
		t.Fatalf("receipt after the rejection = %+v", r)
	}
	wantFigures(t, l.Projection(), 105, 0, 0, 0, 0)
}

func TestDeltaCorrectionReplacesItsOriginalOnce(t *testing.T) {
	l := NewLedger()
	apply(t, l, delta("d1", "", "scope-1", reported(40)), delta("d2", "", "scope-1", reported(60)))
	fix := delta("d3", "", "scope-1", reported(25))
	fix.Usage.Supersedes, fix.Usage.CorrectionAuthorized = "d1", true
	receipt := apply(t, l, fix)[0]
	if receipt.ActualRemoved != 15 || receipt.ActualAdded != 0 {
		t.Fatalf("correction receipt = %+v", receipt)
	}
	wantFigures(t, l.Projection(), 85, 0, 0, 0, 0)

	// d1 is already superseded; a second correction of it has no valid chain.
	second := delta("d4", "", "scope-1", reported(1))
	second.Usage.Supersedes, second.Usage.CorrectionAuthorized = "d1", true
	if r := apply(t, l, second)[0]; r.Disposition != DispositionQuarantined || r.Reason != ReasonCorrectionUnresolved {
		t.Fatalf("second correction = %+v", r)
	}
	wantFigures(t, l.Projection(), 85, 0, 0, 0, 0)

	// Correcting the current version of the chain works.
	third := delta("d5", "", "scope-1", reported(30))
	third.Usage.Supersedes, third.Usage.CorrectionAuthorized = "d3", true
	if r := apply(t, l, third)[0]; r.ActualAdded != 5 || r.ActualRemoved != 0 {
		t.Fatalf("third correction = %+v", r)
	}
	wantFigures(t, l.Projection(), 90, 0, 0, 0, 0)
}

func TestDeltaCorrectionResolvesTheOriginalsUnknown(t *testing.T) {
	l := NewLedger()
	apply(t, l, delta("d1", "", "scope-1", Cost{}))
	wantUnknown(t, l.Projection(), ReasonCostAbsent)
	fix := delta("d2", "", "scope-1", reported(12))
	fix.Usage.Supersedes, fix.Usage.CorrectionAuthorized = "d1", true
	apply(t, l, fix)
	p := l.Projection()
	wantFigures(t, p, 12, 0, 0, 0, 0)
	wantUnknown(t, p)
}

// b30 builds a provider session metered cumulatively across two invocations:
// 80 at the end of the first, 200 at the end of the second.
func b30(t *testing.T, baseline string) (*Ledger, []Receipt) {
	t.Helper()
	first := cumulative("e1", "r1", "session-1", 1, reported(80))
	first.Usage.InvocationID = "i1"
	second := cumulative("e2", "r2", "session-1", 2, reported(200))
	second.Usage.InvocationID = "i2"
	second.Usage.BaselineEventID = baseline
	l := NewLedger()
	receipts := apply(t, l, reserveEntry("reserve-1", "r1", 100), reserveEntry("reserve-2", "r2", 150), first, second)
	return l, receipts[2:]
}

// B30 with a provable baseline: i1 = 80, i2 = 120, and 200 is counted once.
func TestB30SessionAcrossInvocationsWithBaseline(t *testing.T) {
	l, receipts := b30(t, "e1")
	if receipts[0].ActualAdded != 80 || receipts[1].ActualAdded != 120 || receipts[1].Disposition != DispositionAccepted {
		t.Fatalf("receipts = %+v", receipts)
	}
	p := l.Projection()
	// r1: 100 - 80 = 20 left; r2: 150 - 120 = 30 left.
	wantFigures(t, p, 200, 0, 50, 0, 0)
	wantUnknown(t, p)
}

// B30 without a baseline: i2 is ambiguous. A holds only the 80; the 200 is
// neither added on top nor split by guesswork.
func TestB30SessionAcrossInvocationsWithoutBaseline(t *testing.T) {
	l, receipts := b30(t, "")
	if receipts[0].ActualAdded != 80 || receipts[1].Disposition != DispositionQuarantined || receipts[1].Reason != ReasonScopeAmbiguity || receipts[1].ActualAdded != 0 {
		t.Fatalf("receipts = %+v", receipts)
	}
	p := l.Projection()
	// r1: 100 - 80 = 20 left; r2 untouched at 150.
	wantFigures(t, p, 80, 0, 170, 0, 0)
	wantUnknown(t, p, ReasonScopeAmbiguity)
	if p.Unknown[0].EntryID != "e2" || p.Unknown[0].ReservationID != "r2" {
		t.Fatalf("unknown item = %+v", p.Unknown[0])
	}
}

func TestBaselineThatIsNotTheScopeHeadIsAmbiguous(t *testing.T) {
	// A session whose history started somewhere this ledger never saw.
	foreign := cumulative("e1", "", "session-1", 1, reported(200))
	foreign.Usage.BaselineEventID = "event-from-another-root"
	l := NewLedger()
	if r := apply(t, l, foreign)[0]; r.Disposition != DispositionQuarantined || r.Reason != ReasonScopeAmbiguity {
		t.Fatalf("receipt = %+v", r)
	}
	wantFigures(t, l.Projection(), 0, 0, 0, 0, 0)

	l, _ = b30(t, "e1")
	stale := cumulative("e3", "r2", "session-1", 3, reported(260))
	stale.Usage.InvocationID = "i3"
	stale.Usage.BaselineEventID = "e1" // the head is e2 by now
	if r := apply(t, l, stale)[0]; r.Disposition != DispositionQuarantined || r.Reason != ReasonScopeAmbiguity {
		t.Fatalf("receipt = %+v", r)
	}
	wantFigures(t, l.Projection(), 200, 0, 50, 0, 0)
}

func TestCumulativeTokenRegressionIsQuarantined(t *testing.T) {
	first := cumulative("v1", "", "scope-1", 1, reported(100))
	first.Usage.Tokens = TokenCounts{Input: 500, Output: 40}
	second := cumulative("v2", "", "scope-1", 2, reported(150))
	second.Usage.Tokens = TokenCounts{Input: 400, Output: 60}
	l := NewLedger()
	receipts := apply(t, l, first, second)
	if receipts[1].Disposition != DispositionQuarantined || receipts[1].Reason != ReasonSequenceRegression {
		t.Fatalf("receipt = %+v", receipts[1])
	}
	wantFigures(t, l.Projection(), 100, 0, 0, 0, 0)
}

// A later running total with a reported cost covers the scope from its start,
// so an earlier cost-less observation of the same scope is no longer unknown.
func TestCumulativeCostCoversEarlierCostlessObservations(t *testing.T) {
	l := NewLedger()
	apply(t, l, cumulative("v1", "", "scope-1", 1, Cost{}))
	wantUnknown(t, l.Projection(), ReasonCostAbsent)
	apply(t, l, cumulative("v2", "", "scope-1", 2, reported(90)))
	p := l.Projection()
	wantFigures(t, p, 90, 0, 0, 0, 0)
	wantUnknown(t, p)

	// The reverse does not hold: a cost-less later observation leaves the
	// known part in place and marks its own part unknown.
	apply(t, l, cumulative("v3", "", "scope-1", 3, Cost{}))
	p = l.Projection()
	wantFigures(t, p, 90, 0, 0, 0, 0)
	wantUnknown(t, p, ReasonCostAbsent)
}

func TestEstimatedCostNeverBecomesActual(t *testing.T) {
	estimated := func(v Ticks) Cost { return Cost{Basis: CostEstimated, Ticks: v, Currency: "USD"} }
	l := NewLedger()
	receipts := apply(t, l,
		reserveEntry("reserve-1", "r1", 100),
		cumulative("v1", "r1", "scope-1", 1, estimated(30)),
		cumulative("v2", "r1", "scope-1", 2, estimated(45)),
	)
	if receipts[1].EstimatedAdded != 30 || receipts[2].EstimatedAdded != 15 || receipts[2].ActualAdded != 0 || receipts[2].CostState != CostStateEstimated {
		t.Fatalf("receipts = %+v", receipts)
	}
	p := l.Projection()
	// An estimate does not draw down the reservation.
	wantFigures(t, p, 0, 45, 100, 0, 0)
	wantUnknown(t, p, ReasonEstimateOnly, ReasonEstimateOnly)
	wantRemaining(t, p, 250, nil)
}

func TestCorrectionCannotMakeAReservationNegative(t *testing.T) {
	l, _ := b30(t, "e1")
	fix := cumulative("e3", "r2", "session-1", 3, reported(60))
	fix.Usage.InvocationID = "i2"
	fix.Usage.Supersedes, fix.Usage.CorrectionAuthorized = "e2", true
	// e2 gave r2 120. Lowering the session to 60 would take 140 back from r2.
	if r := apply(t, l, fix)[0]; r.Disposition != DispositionQuarantined || r.Reason != ReasonCorrectionUnresolved {
		t.Fatalf("receipt = %+v", r)
	}
	wantFigures(t, l.Projection(), 200, 0, 50, 0, 0)
}
