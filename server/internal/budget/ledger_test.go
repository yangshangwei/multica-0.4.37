package budget

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"
)

func reported(ticks Ticks) Cost { return Cost{Basis: CostReported, Ticks: ticks, Currency: "USD"} }

func reserveEntry(id, reservation string, ticks Ticks) Entry {
	return Entry{ID: id, Kind: KindReserve, ReservationID: reservation, ReservedTicks: ticks}
}

func holdEntry(id, reservation string, reason HoldReason) Entry {
	return Entry{ID: id, Kind: KindHold, ReservationID: reservation, HoldReason: reason}
}

func settleEntry(id, reservation string) Entry {
	return Entry{ID: id, Kind: KindSettle, ReservationID: reservation,
		Proof: ReleaseProof{FinalUsage: true, ScopeComplete: true, NoFurtherExecution: true}}
}

func cumulative(id, reservation, scope string, sequence Count, cost Cost) Entry {
	return Entry{ID: id, Kind: KindUsage, ReservationID: reservation,
		Usage: Usage{ScopeID: scope, InvocationID: "inv-" + scope, Mode: ModeCumulative, Sequence: sequence, Cost: cost}}
}

func delta(id, reservation, scope string, cost Cost) Entry {
	return Entry{ID: id, Kind: KindUsage, ReservationID: reservation,
		Usage: Usage{ScopeID: scope, InvocationID: "inv-" + scope, Mode: ModeDelta, Cost: cost}}
}

func apply(t *testing.T, l *Ledger, entries ...Entry) []Receipt {
	t.Helper()
	receipts, err := l.apply(entries...)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if len(receipts) != len(entries) {
		t.Fatalf("apply returned %d receipts for %d entries", len(receipts), len(entries))
	}
	return receipts
}

// wantFigures compares A, E, R, H and overrun with hand-computed literals.
func wantFigures(t *testing.T, p Projection, actual, estimated, reserved, hold, overrun Ticks) {
	t.Helper()
	if p.ActualKnown != actual || p.Estimated != estimated || p.Reserved != reserved || p.UnknownHold != hold || p.Overrun != overrun {
		t.Fatalf("A=%d E=%d R=%d H=%d overrun=%d; want A=%d E=%d R=%d H=%d overrun=%d",
			p.ActualKnown, p.Estimated, p.Reserved, p.UnknownHold, p.Overrun, actual, estimated, reserved, hold, overrun)
	}
}

func wantUnknown(t *testing.T, p Projection, reasons ...UnknownReason) {
	t.Helper()
	got := []UnknownReason{}
	for _, item := range p.Unknown {
		got = append(got, item.Reason)
	}
	if len(reasons) == 0 {
		reasons = []UnknownReason{}
	}
	if !reflect.DeepEqual(got, reasons) {
		t.Fatalf("unknown reasons = %v, want %v", got, reasons)
	}
	if p.TotalKnown() != (len(reasons) == 0) {
		t.Fatalf("TotalKnown = %v with unknown %v", p.TotalKnown(), got)
	}
}

func wantRemaining(t *testing.T, p Projection, limit Ticks, want *Ticks) {
	t.Helper()
	got := p.Remaining(limit)
	switch {
	case want == nil && got != nil:
		t.Fatalf("Remaining(%d) = %d, want absent", limit, *got)
	case want != nil && (got == nil || *got != *want):
		t.Fatalf("Remaining(%d) = %v, want %d", limit, got, *want)
	}
}

func ticks(v Ticks) *Ticks { return &v }

// b18 builds: reserve 100, partial actual 30, then contact is lost.
func b18(t *testing.T) *Ledger {
	t.Helper()
	l := NewLedger()
	apply(t, l,
		reserveEntry("reserve-1", "r1", 100),
		cumulative("e1", "r1", "scope-1", 1, reported(30)),
		holdEntry("lost-1", "r1", HoldLostContact),
	)
	return l
}

func TestB17PartialActualMovesReservedToActual(t *testing.T) {
	l := NewLedger()
	receipts := apply(t, l,
		reserveEntry("reserve-1", "r1", 100),
		cumulative("e1", "r1", "scope-1", 1, reported(30)),
	)
	if receipts[1].ActualAdded != 30 || receipts[1].Disposition != DispositionAccepted || receipts[1].CostState != CostStateActual {
		t.Fatalf("usage receipt = %+v", receipts[1])
	}
	p := l.Projection()
	wantFigures(t, p, 30, 0, 70, 0, 0) // not A=30, R=100
	wantUnknown(t, p)
	if p.Committed() != 100 {
		t.Fatalf("committed = %d, want 100", p.Committed())
	}
	wantRemaining(t, p, 250, ticks(150))
}

func TestB18LostContactTurnsReservedIntoUnknownHold(t *testing.T) {
	p := b18(t).Projection()
	wantFigures(t, p, 30, 0, 0, 70, 0)
	wantUnknown(t, p, UnknownReason("lost_contact"))
	if p.Unknown[0].ReservationID != "r1" || p.Unknown[0].EntryID != "lost-1" {
		t.Fatalf("unknown item = %+v", p.Unknown[0])
	}
	// The total is unknown: remaining is absent, not limit - A, and the hold
	// is not reported as known spend.
	wantRemaining(t, p, 250, nil)
	if p.ActualKnown == 100 {
		t.Fatal("hold was reported as known actual")
	}
	if p.Committed() != 100 {
		t.Fatalf("committed = %d, want 100", p.Committed())
	}
}

func TestB19FinalEvidenceSettlesAndReleasesOnce(t *testing.T) {
	l := b18(t)
	final := cumulative("e2", "r1", "scope-1", 2, reported(80))
	settle := settleEntry("settle-1", "r1")

	usage := apply(t, l, final)[0]
	if usage.ActualAdded != 50 || usage.ActualRemoved != 0 {
		t.Fatalf("final usage receipt = %+v", usage)
	}
	wantFigures(t, l.Projection(), 80, 0, 0, 20, 0)

	settled := apply(t, l, settle)[0]
	if settled.Released != 20 || settled.Disposition != DispositionAccepted || settled.Replayed {
		t.Fatalf("settle receipt = %+v", settled)
	}
	p := l.Projection()
	wantFigures(t, p, 80, 0, 0, 0, 0)
	wantUnknown(t, p)
	wantRemaining(t, p, 250, ticks(170))

	// Replaying the same receipts adds nothing and releases nothing more.
	replayed := apply(t, l, final, settle)
	if !replayed[0].Replayed || !replayed[1].Replayed {
		t.Fatalf("replay receipts = %+v", replayed)
	}
	replayed[0].Replayed, replayed[1].Replayed = false, false
	if replayed[0] != usage || replayed[1] != settled {
		t.Fatalf("replay returned %+v, want the original receipts", replayed)
	}
	again := l.Projection()
	wantFigures(t, again, 80, 0, 0, 0, 0)
	wantRemaining(t, again, 250, ticks(170))
}

func TestB20FinalCostAboveReservationIsRecordedAsOverrun(t *testing.T) {
	l := b18(t)
	usage := apply(t, l, cumulative("e2", "r1", "scope-1", 2, reported(120)))[0]
	if usage.ActualAdded != 90 || usage.Disposition != DispositionAccepted {
		t.Fatalf("final usage receipt = %+v", usage)
	}
	wantFigures(t, l.Projection(), 120, 0, 0, 0, 20) // not clipped to 100

	settled := apply(t, l, settleEntry("settle-1", "r1"))[0]
	if settled.Released != 0 {
		t.Fatalf("settle released %d, want 0", settled.Released)
	}
	p := l.Projection()
	wantFigures(t, p, 120, 0, 0, 0, 20)
	wantUnknown(t, p)
	wantRemaining(t, p, 250, ticks(130))
	wantRemaining(t, p, 100, ticks(0))
}

// Contract section 4.2: A, R and H never count the same allowance twice, and
// moving R to H makes no allowance available.
func TestAllowanceIsCountedOnceAndHoldFreesNothing(t *testing.T) {
	l := NewLedger()
	steps := []struct {
		entry                         Entry
		actual, reserved, hold, total Ticks
	}{
		{reserveEntry("reserve-1", "r1", 100), 0, 100, 0, 100},
		{cumulative("e1", "r1", "scope-1", 1, reported(30)), 30, 70, 0, 100},
		{holdEntry("cancel-1", "r1", HoldCancelled), 30, 0, 70, 100},
		{cumulative("e2", "r1", "scope-1", 2, reported(45)), 45, 0, 55, 100},
		{settleEntry("settle-1", "r1"), 45, 0, 0, 45},
	}
	for i, step := range steps {
		apply(t, l, step.entry)
		p := l.Projection()
		wantFigures(t, p, step.actual, 0, step.reserved, step.hold, 0)
		if p.Committed() != step.total {
			t.Fatalf("step %d: committed = %d, want %d", i, p.Committed(), step.total)
		}
	}
}

func TestLifecycleFactsAloneNeverRelease(t *testing.T) {
	for _, reason := range []HoldReason{HoldCancelled, HoldTimeout, HoldProcessExit, HoldLostContact, HoldSupersededUnproven} {
		l := NewLedger()
		receipts := apply(t, l, reserveEntry("reserve-1", "r1", 100), holdEntry("fact-1", "r1", reason))
		if receipts[1].Released != 0 || receipts[1].Reason != UnknownReason(reason) {
			t.Fatalf("%s: receipt = %+v", reason, receipts[1])
		}
		p := l.Projection()
		wantFigures(t, p, 0, 0, 0, 100, 0)
		wantUnknown(t, p, UnknownReason(reason))
		wantRemaining(t, p, 250, nil)
	}
	// A second lifecycle fact adds its reason and still releases nothing.
	l := NewLedger()
	apply(t, l, reserveEntry("reserve-1", "r1", 100), holdEntry("fact-1", "r1", HoldTimeout), holdEntry("fact-2", "r1", HoldProcessExit))
	p := l.Projection()
	wantFigures(t, p, 0, 0, 0, 100, 0)
	wantUnknown(t, p, "timeout", "process_exit")
}

func TestSettleRequiresEveryProof(t *testing.T) {
	for name, proof := range map[string]ReleaseProof{
		"none":                 {},
		"no final usage":       {ScopeComplete: true, NoFurtherExecution: true},
		"scope incomplete":     {FinalUsage: true, NoFurtherExecution: true},
		"may still execute":    {FinalUsage: true, ScopeComplete: true},
		"only lifecycle ended": {NoFurtherExecution: true},
	} {
		l := NewLedger()
		apply(t, l, reserveEntry("reserve-1", "r1", 100))
		_, err := l.apply(Entry{ID: "settle-1", Kind: KindSettle, ReservationID: "r1", Proof: proof})
		if !errors.Is(err, ErrReleaseProofIncomplete) {
			t.Errorf("%s: error = %v, want ErrReleaseProofIncomplete", name, err)
		}
		wantFigures(t, l.Projection(), 0, 0, 100, 0, 0)
	}
}

func TestSettleIsRefusedWhileUsageOfTheReservationIsUnknown(t *testing.T) {
	l := NewLedger()
	apply(t, l, reserveEntry("reserve-1", "r1", 100), delta("d1", "r1", "scope-1", Cost{}))
	if _, err := l.apply(settleEntry("settle-1", "r1")); !errors.Is(err, ErrReleaseProofIncomplete) {
		t.Fatalf("settle error = %v, want ErrReleaseProofIncomplete", err)
	}
	p := l.Projection()
	wantFigures(t, p, 0, 0, 100, 0, 0)
	wantUnknown(t, p, ReasonCostAbsent)
}

// Settlement and cancellation of the same reservation end it exactly once in
// either order; a lifecycle fact never overrides a settlement.
func TestSettleAndHoldEndAReservationOnce(t *testing.T) {
	first := NewLedger()
	apply(t, first, reserveEntry("reserve-1", "r1", 100), cumulative("e1", "r1", "scope-1", 1, reported(40)))
	settled := apply(t, first, settleEntry("settle-1", "r1"))[0]
	late := apply(t, first, holdEntry("cancel-1", "r1", HoldCancelled))[0]
	if settled.Released != 60 || late.Disposition != DispositionStale || late.Released != 0 || late.Reason != "" {
		t.Fatalf("settle then hold = %+v, %+v", settled, late)
	}
	p := first.Projection()
	wantFigures(t, p, 40, 0, 0, 0, 0)
	wantUnknown(t, p)

	second := NewLedger()
	apply(t, second, reserveEntry("reserve-1", "r1", 100), cumulative("e1", "r1", "scope-1", 1, reported(40)))
	apply(t, second, holdEntry("cancel-1", "r1", HoldCancelled))
	wantFigures(t, second.Projection(), 40, 0, 0, 60, 0)
	settled = apply(t, second, settleEntry("settle-1", "r1"))[0]
	again := apply(t, second, settleEntry("settle-2", "r1"))[0]
	if settled.Released != 60 || again.Disposition != DispositionStale || again.Released != 0 {
		t.Fatalf("hold then settle twice = %+v, %+v", settled, again)
	}
	p = second.Projection()
	wantFigures(t, p, 40, 0, 0, 0, 0)
	wantUnknown(t, p)
}

func TestLateUsageAfterSettlementStillCounts(t *testing.T) {
	l := NewLedger()
	apply(t, l, reserveEntry("reserve-1", "r1", 100), cumulative("e1", "r1", "scope-1", 1, reported(40)), settleEntry("settle-1", "r1"))
	late := apply(t, l, cumulative("e2", "r1", "scope-1", 2, reported(130)))[0]
	if late.ActualAdded != 90 {
		t.Fatalf("late receipt = %+v", late)
	}
	wantFigures(t, l.Projection(), 130, 0, 0, 0, 30)
}

func TestUsageWithoutAReservationCountsAsActualOnly(t *testing.T) {
	l := NewLedger()
	apply(t, l, delta("d1", "", "scope-1", reported(25)))
	p := l.Projection()
	wantFigures(t, p, 25, 0, 0, 0, 0)
	if p.Coverage.IncludedAttempts != 0 {
		t.Fatalf("included attempts = %d", p.Coverage.IncludedAttempts)
	}
}

func TestEntryIdentityConflictsAndUnknownReferences(t *testing.T) {
	l := NewLedger()
	apply(t, l, reserveEntry("reserve-1", "r1", 100), delta("d1", "r1", "scope-1", reported(10)))
	before := l.Projection()
	for name, tc := range map[string]struct {
		entry Entry
		want  error
	}{
		"same id, other amount":      {reserveEntry("reserve-1", "r1", 101), ErrConflict},
		"same id, other kind":        {holdEntry("reserve-1", "r1", HoldTimeout), ErrConflict},
		"same usage id, other cost":  {delta("d1", "r1", "scope-1", reported(11)), ErrConflict},
		"reservation opened twice":   {reserveEntry("reserve-2", "r1", 100), ErrConflict},
		"usage on unknown reserve":   {delta("d2", "r9", "scope-1", reported(1)), ErrUnknownReservation},
		"hold on unknown reserve":    {holdEntry("hold-1", "r9", HoldTimeout), ErrUnknownReservation},
		"settle on unknown reserve":  {settleEntry("settle-1", "r9"), ErrUnknownReservation},
		"gap on unknown reserve":     {Entry{ID: "gap-1", Kind: KindGap, ReservationID: "r9", GapReason: ReasonCoverageGap}, ErrUnknownReservation},
		"missing id":                 {reserveEntry("", "r2", 1), ErrInvalidEntry},
		"unknown kind":               {Entry{ID: "x", Kind: "refund"}, ErrInvalidEntry},
		"reserve without target":     {reserveEntry("reserve-3", "", 1), ErrInvalidEntry},
		"negative reservation":       {reserveEntry("reserve-3", "r3", -1), ErrNegativeAmount},
		"reserve with usage fields":  {Entry{ID: "reserve-3", Kind: KindReserve, ReservationID: "r3", Usage: Usage{ScopeID: "s"}}, ErrInvalidEntry},
		"hold with unknown reason":   {holdEntry("hold-1", "r1", "bored"), ErrInvalidEntry},
		"hold carrying an amount":    {Entry{ID: "hold-1", Kind: KindHold, ReservationID: "r1", HoldReason: HoldTimeout, ReservedTicks: 5}, ErrInvalidEntry},
		"gap with lifecycle reason":  {Entry{ID: "gap-1", Kind: KindGap, GapReason: "timeout"}, ErrInvalidEntry},
		"exclusion without name":     {Entry{ID: "x-1", Kind: KindExclusion}, ErrInvalidEntry},
		"usage without scope":        {delta("d2", "r1", "", reported(1)), ErrInvalidEntry},
		"usage without invocation":   {Entry{ID: "d2", Kind: KindUsage, Usage: Usage{ScopeID: "s", Mode: ModeDelta, Cost: reported(1)}}, ErrInvalidEntry},
		"usage without mode":         {Entry{ID: "d2", Kind: KindUsage, Usage: Usage{ScopeID: "s", InvocationID: "i", Cost: reported(1)}}, ErrInvalidEntry},
		"cumulative without order":   {cumulative("c1", "r1", "scope-2", 0, reported(1)), ErrInvalidEntry},
		"negative tokens":            {Entry{ID: "d2", Kind: KindUsage, Usage: Usage{ScopeID: "s", InvocationID: "i", Mode: ModeDelta, Tokens: TokenCounts{Output: -1}}}, ErrNegativeAmount},
		"absent cost with an amount": {delta("d2", "r1", "scope-1", Cost{Ticks: 5}), ErrInvalidEntry},
		"unknown cost basis":         {delta("d2", "r1", "scope-1", Cost{Basis: "guessed", Ticks: 5, Currency: "USD"}), ErrInvalidEntry},
		"zero basis on a charge":     {delta("d2", "r1", "scope-1", Cost{Basis: CostReported, Ticks: 5, Currency: "USD", ZeroBasis: "free-tier"}), ErrInvalidEntry},
		"delta with a baseline":      {Entry{ID: "d2", Kind: KindUsage, Usage: Usage{ScopeID: "s", InvocationID: "i", Mode: ModeDelta, BaselineEventID: "d1", Cost: reported(1)}}, ErrInvalidEntry},
		"unknown overlap":            {Entry{ID: "d2", Kind: KindUsage, Usage: Usage{ScopeID: "s", InvocationID: "i", Mode: ModeDelta, Overlap: "maybe", Cost: reported(1)}}, ErrInvalidEntry},
	} {
		if _, err := l.apply(tc.entry); !errors.Is(err, tc.want) {
			t.Errorf("%s: error = %v, want %v", name, err, tc.want)
		}
		if after := l.Projection(); !reflect.DeepEqual(after, before) {
			t.Fatalf("%s: a rejected entry changed the projection to %+v", name, after)
		}
	}
}

// B24a: an increment that would pass the 64-bit range rejects the whole batch
// and leaves nothing behind.
func TestB24aOverflowRejectsTheWholeBatch(t *testing.T) {
	l := NewLedger()
	apply(t, l, delta("d1", "", "scope-1", reported(9_223_372_036_854_775_800)))
	before := l.Projection()

	fits := delta("d2", "", "scope-2", reported(7))
	spills := delta("d3", "", "scope-3", reported(1))
	if receipts, err := l.apply(fits, spills); !errors.Is(err, ErrOverflow) || receipts != nil {
		t.Fatalf("batch = %+v, %v; want ErrOverflow", receipts, err)
	}
	if after := l.Projection(); !reflect.DeepEqual(after, before) || after.ActualKnown != 9_223_372_036_854_775_800 {
		t.Fatalf("rejected batch changed the projection to %+v", after)
	}
	// The entry that fit was not kept from the rejected batch.
	if r := apply(t, l, fits)[0]; r.Replayed || r.ActualAdded != 7 {
		t.Fatalf("receipt after the rejected batch = %+v", r)
	}
	wantFigures(t, l.Projection(), 9_223_372_036_854_775_807, 0, 0, 0, 0)

	// Reservations and estimates share the same bound.
	if _, err := l.apply(reserveEntry("reserve-1", "r1", 1)); !errors.Is(err, ErrOverflow) {
		t.Fatalf("reservation overflow error = %v", err)
	}
	est := NewLedger()
	apply(t, est, delta("e1", "", "scope-1", Cost{Basis: CostEstimated, Ticks: 9_223_372_036_854_775_807, Currency: "USD"}))
	if _, err := est.apply(delta("e2", "", "scope-2", Cost{Basis: CostEstimated, Ticks: 1, Currency: "USD"})); !errors.Is(err, ErrOverflow) {
		t.Fatalf("estimate overflow error = %v", err)
	}
}

// B24 (pure half): an estimate is frozen in its entry. Replaying the ledger
// takes no price input, so a later price table cannot change the old E, and
// reported actual never depends on prices.
func TestB24ReplayKeepsFrozenEstimateAndActual(t *testing.T) {
	frozen := priceFixture(t)
	est, err := frozen.Estimate(TokenCounts{Input: 1_000_001}, "")
	if err != nil {
		t.Fatal(err)
	}
	estimated := delta("est-1", "", "scope-1", Cost{Basis: CostEstimated, Ticks: est.Ticks, Currency: "USD", PriceVersion: est.PriceVersion, PriceDigest: est.PriceDigest})
	estimated.Usage.Tokens = TokenCounts{Input: 1_000_001}
	entries := []Entry{estimated, delta("act-1", "", "scope-2", reported(500))}

	first, receipts, err := Reduce(entries)
	if err != nil {
		t.Fatal(err)
	}
	wantFigures(t, first, 500, 30_000_030_000, 0, 0, 0)
	wantUnknown(t, first, ReasonEstimateOnly) // an estimate is never known actual
	if receipts[0].CostState != CostStateEstimated || receipts[0].EstimatedAdded != 30_000_030_000 || receipts[0].ActualAdded != 0 {
		t.Fatalf("estimate receipt = %+v", receipts[0])
	}

	// The price table moves on; the stored entries are replayed unchanged.
	newer := priceFixture(t)
	newer.PriceVersion = "2026-12-01"
	newer.Input = usdRate(t, 1, 1, 1_000_000)
	if now, err := newer.Estimate(TokenCounts{Input: 1_000_001}, ""); err != nil || now.Ticks != 10_000_010_000 {
		t.Fatalf("newer table = %+v, %v", now, err)
	}
	second, _, err := Reduce(entries)
	if err != nil || !reflect.DeepEqual(second, first) {
		t.Fatalf("replay = %+v, %v; want %+v", second, err, first)
	}
	wantFigures(t, second, 500, 30_000_030_000, 0, 0, 0)
}

// B48: mechanism-level exclusions are named, stay out of the coverage
// denominator and are not a zero cost.
func TestB48ExclusionsAreNotApplicableNotZeroCost(t *testing.T) {
	l := NewLedger()
	apply(t, l,
		reserveEntry("reserve-1", "r1", 100),
		cumulative("e1", "r1", "scope-1", 1, reported(30)),
		Entry{ID: "x-1", Kind: KindExclusion, Detail: "run_only_autopilot"},
		Entry{ID: "x-2", Kind: KindExclusion, Detail: "plugin_external_call"},
		Entry{ID: "x-3", Kind: KindExclusion, Detail: "collaboration_host_run"},
		Entry{ID: "x-4", Kind: KindExclusion, Detail: "run_only_autopilot"},
	)
	p := l.Projection()
	wantFigures(t, p, 30, 0, 70, 0, 0)
	wantUnknown(t, p)
	want := Coverage{IncludedAttempts: 1, NotApplicable: []string{"collaboration_host_run", "plugin_external_call", "run_only_autopilot"}}
	if !reflect.DeepEqual(p.Coverage, want) {
		t.Fatalf("coverage = %+v, want %+v", p.Coverage, want)
	}
}

func TestGapEntriesJoinTheUnknownSet(t *testing.T) {
	l := NewLedger()
	apply(t, l,
		reserveEntry("reserve-1", "r1", 100),
		Entry{ID: "gap-1", Kind: KindGap, ReservationID: "r1", GapReason: ReasonCoverageGap, Detail: "legacy_protocol"},
		Entry{ID: "gap-2", Kind: KindGap, GapReason: ReasonPreActivationUnknown, Detail: "in_flight_legacy"},
	)
	p := l.Projection()
	wantFigures(t, p, 0, 0, 100, 0, 0)
	wantUnknown(t, p, ReasonCoverageGap, ReasonPreActivationUnknown)
	if p.Unknown[0] != (UnknownItem{Reason: ReasonCoverageGap, EntryID: "gap-1", ReservationID: "r1", Detail: "legacy_protocol"}) {
		t.Fatalf("gap item = %+v", p.Unknown[0])
	}
	wantRemaining(t, p, 250, nil)
	counts := p.UnknownCounts()
	if len(counts) != 2 || counts[ReasonCoverageGap] != 1 || counts[ReasonPreActivationUnknown] != 1 {
		t.Fatalf("unknown counts = %v", counts)
	}
}

func TestReduceIsAPureFunctionOfTheOrderedEntries(t *testing.T) {
	entries := []Entry{
		reserveEntry("reserve-1", "r1", 100),
		cumulative("e1", "r1", "scope-1", 1, reported(30)),
		holdEntry("lost-1", "r1", HoldLostContact),
		cumulative("e2", "r1", "scope-1", 2, reported(80)),
		settleEntry("settle-1", "r1"),
		reserveEntry("reserve-2", "r2", 40),
		delta("d1", "r2", "scope-2", Cost{}),
		{ID: "x-1", Kind: KindExclusion, Detail: "chat"},
	}
	want, receipts, err := Reduce(entries)
	if err != nil || len(receipts) != len(entries) {
		t.Fatalf("Reduce = %v, %d receipts", err, len(receipts))
	}
	wantFigures(t, want, 80, 0, 40, 0, 0)
	wantUnknown(t, want, ReasonCostAbsent)

	again, _, err := Reduce(entries)
	if err != nil || !reflect.DeepEqual(again, want) {
		t.Fatalf("second Reduce = %+v, %v", again, err)
	}
	step := NewLedger()
	for _, e := range entries {
		apply(t, step, e)
	}
	if got := step.Projection(); !reflect.DeepEqual(got, want) {
		t.Fatalf("entry-by-entry = %+v, want %+v", got, want)
	}
	// Appending every entry a second time is a replay of every receipt.
	doubled, replayed, err := Reduce(append(append([]Entry{}, entries...), entries...))
	if err != nil || !reflect.DeepEqual(doubled, want) {
		t.Fatalf("doubled = %+v, %v", doubled, err)
	}
	for i, r := range replayed[len(entries):] {
		if !r.Replayed {
			t.Fatalf("entry %d was applied twice: %+v", i, r)
		}
	}
	// Reduce does not keep or modify the caller's slice.
	if entries[0] != reserveEntry("reserve-1", "r1", 100) {
		t.Fatal("Reduce modified its input")
	}
}

func TestEmptyLedgerProjection(t *testing.T) {
	p := NewLedger().Projection()
	wantFigures(t, p, 0, 0, 0, 0, 0)
	wantUnknown(t, p)
	wantRemaining(t, p, 100, ticks(100))
	wantRemaining(t, p, -1, nil)
	raw, err := json.Marshal(p)
	const want = `{"actual_known_ticks":"0","estimated_ticks":"0","reserved_ticks":"0","unknown_hold_ticks":"0","overrun_ticks":"0","unknown":[],"coverage":{"included_attempts":0,"not_applicable":[]}}`
	if err != nil || string(raw) != want {
		t.Fatalf("projection JSON = %s, %v", raw, err)
	}
}

// pinnedEntry is the entry whose stored form and digest are pinned below.
func pinnedEntry() Entry {
	e := cumulative("e1", "r1", "scope-1", 2, reported(30))
	e.Usage.Tokens = TokenCounts{Input: 1_000_001, Output: 7}
	return e
}

// pinnedStoredForm is the stored form of pinnedEntry in form version 1, byte
// for byte.
const pinnedStoredForm = `{"id":"e1","kind":"usage","reservation_id":"r1","reserved_ticks":"0","unsized":false,"hold_reason":"","proof":{"final_usage":false,"scope_complete":false,"no_further_execution":false},"gap_reason":"","detail":"","usage":{"scope_id":"scope-1","invocation_id":"inv-scope-1","mode":"cumulative","sequence":"2","overlap":"","baseline_event_id":"","supersedes":"","correction_authorized":false,"tokens":{"input":"1000001","output":"7","cache_read":"0","cache_write":"0"},"cost":{"basis":"reported","ticks":"30","currency":"USD","zero_basis":"","price_version":"","price_digest":""}}}`

func TestEntryStoredFormRoundTripsAndDigests(t *testing.T) {
	e := pinnedEntry()
	raw, d, err := EncodeEntry(e)
	if err != nil || string(raw) != pinnedStoredForm {
		t.Fatalf("stored form = %s, %v", raw, err)
	}
	// The stored form is the JSON encoding of the entry.
	if plain, err := json.Marshal(e); err != nil || string(plain) != pinnedStoredForm {
		t.Fatalf("JSON encoding = %s, %v", plain, err)
	}
	back, err := DecodeEntry(EntryFormVersion, raw)
	if err != nil || back != e {
		t.Fatalf("round trip = %+v, %v", back, err)
	}
	// SHA-256 of "budget-entry-v1\n" + the stored form, computed outside Go.
	const wantDigest = "5f07d194587118bd12ebe5834f4d42d2838648c32b4238448948aff5ae79bbc9"
	if d != wantDigest {
		t.Fatalf("digest from EncodeEntry = %q; want %q", d, wantDigest)
	}
	d, err = e.Digest()
	if err != nil || d != wantDigest {
		t.Fatalf("digest = %q, %v; want %q", d, err, wantDigest)
	}
	other := e
	other.Usage.Cost.Ticks = 31
	if od, err := other.Digest(); err != nil || od == d {
		t.Fatalf("changed entry digest = %q, %v", od, err)
	}
	if _, err := reserveEntry("x", "r", -1).Digest(); !errors.Is(err, ErrNegativeAmount) {
		t.Fatalf("negative entry digest error = %v", err)
	}
}
