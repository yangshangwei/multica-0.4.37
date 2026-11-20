package budget

import (
	"errors"
	"reflect"
	"testing"
)

func estimated(ticks Ticks) Cost { return Cost{Basis: CostEstimated, Ticks: ticks, Currency: "USD"} }

// crossReservationSession: one provider session metered cumulatively. r1
// spent 180 of its 100 (overrun 80) in invocation i1; r2 continued the session
// from e1 and spent 20 of its 100. A=200, R=80, overrun=80.
func crossReservationSession(t *testing.T) *Ledger {
	t.Helper()
	l := NewLedger()
	apply(t, l, reserveEntry("reserve-1", "r1", 100), reserveEntry("reserve-2", "r2", 100))
	e1 := cumulative("e1", "r1", "session-1", 1, reported(180))
	e1.Usage.InvocationID = "i1"
	e2 := cumulative("e2", "r2", "session-1", 2, reported(200))
	e2.Usage.InvocationID, e2.Usage.BaselineEventID = "i2", "e1"
	apply(t, l, e1, e2)
	wantFigures(t, l.Projection(), 200, 0, 80, 0, 80)
	return l
}

// Contract 5.2: a correction is never applied to another attempt by default.
// e2 charged r2. A correction of e2 sent under r1 would take the 20 back from
// r1, leaving r2 charged for usage the correction removed (R=80 instead of
// 100) and understating r1's overrun (60 instead of 80).
func TestCumulativeCorrectionThroughAnotherReservationIsQuarantined(t *testing.T) {
	l := crossReservationSession(t)
	fix := cumulative("e3", "r1", "session-1", 3, reported(180))
	fix.Usage.InvocationID = "i2"
	fix.Usage.Supersedes, fix.Usage.CorrectionAuthorized = "e2", true
	if r := apply(t, l, fix)[0]; r.Disposition != DispositionQuarantined || r.Reason != ReasonCorrectionUnresolved || r.ActualRemoved != 0 {
		t.Fatalf("receipt = %+v", r)
	}
	p := l.Projection()
	wantFigures(t, p, 200, 0, 80, 0, 80)
	wantUnknown(t, p, ReasonCorrectionUnresolved)

	// The same correction sent under the reservation e2 charged is applied:
	// r2 goes back to 0 of 100, so A=180, R=100, overrun=80.
	own := cumulative("e4", "r2", "session-1", 4, reported(180))
	own.Usage.InvocationID = "i2"
	own.Usage.Supersedes, own.Usage.CorrectionAuthorized = "e2", true
	if r := apply(t, l, own)[0]; r.Disposition != DispositionAccepted || r.ActualRemoved != 20 {
		t.Fatalf("receipt = %+v", r)
	}
	wantFigures(t, l.Projection(), 180, 0, 100, 0, 80)
}

// The delta form of the same rule: d1 charged r1 180 of 100, d2 charged r2 20
// of 100. Replacing d1 by 170 under r2 would move the amount to r2 (R=100,
// overrun=90) although r1 spent it (R=80, overrun=70).
func TestDeltaCorrectionThroughAnotherReservationIsQuarantined(t *testing.T) {
	l := NewLedger()
	apply(t, l, reserveEntry("reserve-1", "r1", 100), reserveEntry("reserve-2", "r2", 100))
	apply(t, l, delta("d1", "r1", "scope-1", reported(180)), delta("d2", "r2", "scope-1", reported(20)))
	wantFigures(t, l.Projection(), 200, 0, 80, 0, 80)

	fix := delta("d3", "r2", "scope-1", reported(170))
	fix.Usage.Supersedes, fix.Usage.CorrectionAuthorized = "d1", true
	if r := apply(t, l, fix)[0]; r.Disposition != DispositionQuarantined || r.Reason != ReasonCorrectionUnresolved || r.ActualRemoved != 0 {
		t.Fatalf("receipt = %+v", r)
	}
	p := l.Projection()
	wantFigures(t, p, 200, 0, 80, 0, 80)
	wantUnknown(t, p, ReasonCorrectionUnresolved)

	own := delta("d4", "r1", "scope-1", reported(170))
	own.Usage.Supersedes, own.Usage.CorrectionAuthorized = "d1", true
	if r := apply(t, l, own)[0]; r.Disposition != DispositionAccepted || r.ActualRemoved != 10 {
		t.Fatalf("receipt = %+v", r)
	}
	wantFigures(t, l.Projection(), 190, 0, 80, 0, 70)
}

// B28/B29: a delta correction needs authority, an original that exists, and
// the original's own scope. Otherwise the original keeps counting.
func TestDeltaCorrectionWithoutAuthorityTargetOrScopeIsQuarantined(t *testing.T) {
	l := NewLedger()
	apply(t, l, delta("d1", "", "scope-1", reported(40)))

	unauthorised := delta("d2", "", "scope-1", reported(10))
	unauthorised.Usage.Supersedes = "d1"
	otherScope := delta("d3", "", "scope-2", reported(10))
	otherScope.Usage.Supersedes, otherScope.Usage.CorrectionAuthorized = "d1", true
	missing := delta("d4", "", "scope-1", reported(10))
	missing.Usage.Supersedes, missing.Usage.CorrectionAuthorized = "never-seen", true

	for _, e := range []Entry{unauthorised, otherScope, missing} {
		r := apply(t, l, e)[0]
		if r.Disposition != DispositionQuarantined || r.Reason != ReasonCorrectionUnresolved || r.ActualAdded != 0 || r.ActualRemoved != 0 {
			t.Fatalf("%s receipt = %+v", e.ID, r)
		}
	}
	p := l.Projection()
	wantFigures(t, p, 40, 0, 0, 0, 0)
	wantUnknown(t, p, ReasonCorrectionUnresolved, ReasonCorrectionUnresolved, ReasonCorrectionUnresolved)
}

// B27 for estimates: an estimated running total that goes down without a
// correction is quarantined and the earlier estimate stands.
func TestEstimatedRunningTotalCannotGoDownUnexplained(t *testing.T) {
	l := NewLedger()
	apply(t, l, cumulative("e1", "", "scope-1", 1, estimated(100)))
	r := apply(t, l, cumulative("e2", "", "scope-1", 2, estimated(60)))[0]
	if r.Disposition != DispositionQuarantined || r.Reason != ReasonSequenceRegression || r.EstimatedRemoved != 0 {
		t.Fatalf("receipt = %+v", r)
	}
	p := l.Projection()
	wantFigures(t, p, 0, 100, 0, 0, 0)
	wantUnknown(t, p, ReasonEstimateOnly, ReasonSequenceRegression)
}

// The same usage is never in both A and E. A reported running total covers
// the scope from its start and replaces the estimate (E 100 -> 0, not 70); a
// later estimated total of 120 counts only the 90 beyond the known 30.
func TestReportedTotalReplacesTheEstimateAndALaterEstimateCountsTheExcess(t *testing.T) {
	l := NewLedger()
	apply(t, l, cumulative("e1", "", "scope-1", 1, estimated(100)))
	p := l.Projection()
	wantFigures(t, p, 0, 100, 0, 0, 0)
	wantUnknown(t, p, ReasonEstimateOnly)

	r := apply(t, l, cumulative("e2", "", "scope-1", 2, reported(30)))[0]
	if r.ActualAdded != 30 || r.EstimatedRemoved != 100 || r.EstimatedAdded != 0 {
		t.Fatalf("reported receipt = %+v", r)
	}
	p = l.Projection()
	wantFigures(t, p, 30, 0, 0, 0, 0)
	wantUnknown(t, p)

	r = apply(t, l, cumulative("e3", "", "scope-1", 3, estimated(120)))[0]
	if r.ActualAdded != 0 || r.EstimatedAdded != 90 {
		t.Fatalf("estimate receipt = %+v", r)
	}
	p = l.Projection()
	wantFigures(t, p, 30, 90, 0, 0, 0)
	wantUnknown(t, p, ReasonEstimateOnly)
}

// A lifecycle reason is recorded once per reservation; a different reason is
// a new fact. Neither changes the figures of B18.
func TestRepeatedHoldReasonIsRecordedOnce(t *testing.T) {
	l := b18(t)
	if r := apply(t, l, holdEntry("lost-2", "r1", HoldLostContact))[0]; r.Disposition != DispositionStale {
		t.Fatalf("repeated reason receipt = %+v", r)
	}
	wantUnknown(t, l.Projection(), UnknownReason(HoldLostContact))
	if r := apply(t, l, holdEntry("timeout-1", "r1", HoldTimeout))[0]; r.Disposition != DispositionAccepted || r.Reason != UnknownReason(HoldTimeout) {
		t.Fatalf("second reason receipt = %+v", r)
	}
	p := l.Projection()
	wantFigures(t, p, 30, 0, 0, 70, 0)
	wantUnknown(t, p, UnknownReason(HoldLostContact), UnknownReason(HoldTimeout))
}

func TestUsageShapesThatAreRejected(t *testing.T) {
	withVersion := delta("u", "", "scope-1", reported(5))
	withVersion.Usage.Cost.PriceVersion = "2026-11-01"
	withDigest := delta("u", "", "scope-1", reported(5))
	withDigest.Usage.Cost.PriceDigest = "af47"
	rollupCorrection := delta("u", "", "scope-1", reported(5))
	rollupCorrection.Usage.Overlap, rollupCorrection.Usage.Supersedes, rollupCorrection.Usage.CorrectionAuthorized = OverlapRollup, "d1", true
	unprovenBaseline := cumulative("u", "", "scope-2", 1, reported(5))
	unprovenBaseline.Usage.Overlap, unprovenBaseline.Usage.BaselineEventID = OverlapUnproven, "d1"
	selfCorrection := delta("u", "", "scope-1", reported(5))
	selfCorrection.Usage.Supersedes, selfCorrection.Usage.CorrectionAuthorized = "u", true
	authorityWithoutTarget := delta("u", "", "scope-1", reported(5))
	authorityWithoutTarget.Usage.CorrectionAuthorized = true

	l := NewLedger()
	apply(t, l, delta("d1", "", "scope-1", reported(40)))
	before := l.clone()
	for name, e := range map[string]Entry{
		"reported cost with a price version":  withVersion,
		"reported cost with a price digest":   withDigest,
		"rollup that is also a correction":    rollupCorrection,
		"unproven overlap with a baseline":    unprovenBaseline,
		"correction of itself":                selfCorrection,
		"correction authority without target": authorityWithoutTarget,
	} {
		if rs, err := l.apply(e); !errors.Is(err, ErrInvalidEntry) || rs != nil {
			t.Errorf("%s: apply = %+v, %v; want ErrInvalidEntry", name, rs, err)
		}
	}
	if !reflect.DeepEqual(l, before) {
		t.Fatal("a rejected entry changed the ledger")
	}
}

// stringFields returns a setter for every string-kinded field under v.
func stringFields(v reflect.Value, path string, out map[string]func(reflect.Value, string)) {
	for i := 0; i < v.NumField(); i++ {
		index, name := i, path+v.Type().Field(i).Name
		switch v.Field(i).Kind() {
		case reflect.String:
			out[name] = func(root reflect.Value, s string) { root.Field(index).SetString(s) }
		case reflect.Struct:
			inner := map[string]func(reflect.Value, string){}
			stringFields(v.Field(i), name+".", inner)
			for innerName, set := range inner {
				out[innerName] = func(root reflect.Value, s string) { set(root.Field(index), s) }
			}
		}
	}
}

// The stored form is JSON, which cannot carry invalid UTF-8: encoding/json
// replaces each bad byte with U+FFFD, so "a\xff" and "a\xfe" would share one
// stored form and one digest, and a replay from storage would see one entry
// where the live ledger saw two. Every string field is covered, including
// ones added later.
func TestInvalidUTF8IsRejectedInEveryStringField(t *testing.T) {
	base := cumulative("e1", "r1", "scope-1", 1, reported(5))
	base.Usage.Cost.ZeroBasis = ""
	fields := map[string]func(reflect.Value, string){}
	stringFields(reflect.ValueOf(base), "", fields)
	if len(fields) < 15 {
		t.Fatalf("found only %d string fields", len(fields))
	}
	for name, set := range fields {
		for _, kind := range []EntryKind{KindReserve, KindUsage, KindHold, KindSettle, KindGap, KindExclusion} {
			e := Entry{ID: "e1", Kind: kind, ReservationID: "r1"}
			switch kind {
			case KindUsage:
				e = base
			case KindHold:
				e.HoldReason = HoldTimeout
			case KindSettle:
				e.Proof = ReleaseProof{FinalUsage: true, ScopeComplete: true, NoFurtherExecution: true}
			case KindGap:
				e.GapReason = ReasonCoverageGap
			case KindExclusion:
				e.ReservationID, e.Detail = "", "autopilot_quota"
			}
			// The fixture is valid as it stands, so the bad byte is the only
			// reason left to refuse it.
			l := NewLedger()
			if kind != KindReserve {
				apply(t, l, reserveEntry("reserve-1", "r1", 10))
			}
			before := l.clone()
			if _, err := l.clone().apply(e); err != nil {
				t.Fatalf("%s fixture is not valid: %v", kind, err)
			}
			set(reflect.ValueOf(&e).Elem(), "a\xff")
			if _, err := l.apply(e); !errors.Is(err, ErrInvalidEntry) {
				t.Errorf("%s entry with invalid UTF-8 in %s: error = %v, want ErrInvalidEntry", kind, name, err)
			}
			if !reflect.DeepEqual(l, before) {
				t.Errorf("%s entry with invalid UTF-8 in %s changed the ledger", kind, name)
			}
			// Nor does it get a stored form.
			if raw, digest, err := EncodeEntry(e); !errors.Is(err, ErrInvalidEntry) || raw != nil || digest != "" {
				t.Errorf("%s entry with invalid UTF-8 in %s: EncodeEntry = %s, %q, %v; want ErrInvalidEntry", kind, name, raw, digest, err)
			}
		}
	}

	// Valid multi-byte identifiers are accepted and survive the stored form.
	e := reserveEntry("预留-1", "r-é", 10)
	apply(t, NewLedger(), e)
	raw, _, err := EncodeEntry(e)
	if err != nil {
		t.Fatal(err)
	}
	if restored, err := DecodeEntry(EntryFormVersion, raw); err != nil || restored != e {
		t.Fatalf("restored = %+v, %v; want %+v", restored, err, e)
	}
}

func TestPriceSnapshotRejectsInvalidUTF8(t *testing.T) {
	fields := map[string]func(reflect.Value, string){}
	stringFields(reflect.ValueOf(priceFixture(t)), "", fields)
	if len(fields) != 11 {
		t.Fatalf("found %d string fields, want 11", len(fields))
	}
	for name, set := range fields {
		s := priceFixture(t)
		set(reflect.ValueOf(&s).Elem(), "a\xff")
		if err := s.Validate(); err == nil {
			t.Errorf("snapshot with invalid UTF-8 in %s is valid", name)
		}
		if got, err := s.Estimate(TokenCounts{Input: 1}, ""); err == nil || got != (Estimate{}) {
			t.Errorf("snapshot with invalid UTF-8 in %s priced tokens: %+v", name, got)
		}
	}
	s := priceFixture(t)
	s.Source = "a\xff"
	if err := s.Validate(); !errors.Is(err, ErrInvalidPrice) {
		t.Fatalf("Validate = %v, want ErrInvalidPrice", err)
	}
}

// The zero Ledger is an empty ledger, not a panic.
func TestZeroValueLedgerIsEmpty(t *testing.T) {
	var l Ledger
	wantFigures(t, l.Projection(), 0, 0, 0, 0, 0)
	apply(t, &l, reserveEntry("reserve-1", "r1", 100), delta("d1", "r1", "scope-1", reported(30)),
		Entry{ID: "x1", Kind: KindExclusion, Detail: "autopilot_quota"})
	wantFigures(t, l.Projection(), 30, 0, 70, 0, 0)

	same := NewLedger()
	apply(t, same, reserveEntry("reserve-1", "r1", 100), delta("d1", "r1", "scope-1", reported(30)),
		Entry{ID: "x1", Kind: KindExclusion, Detail: "autopilot_quota"})
	if !reflect.DeepEqual(l.Projection(), same.Projection()) {
		t.Fatalf("zero ledger = %+v, NewLedger = %+v", l.Projection(), same.Projection())
	}
}

// The overflow bound follows A + R + H after a settled overrun. r1 reserved
// 10 and spent 20, then settled: A=20. A further reservation of
// 9223372036854775788 would make A + R exceed the 64-bit range by one;
// 9223372036854775787 reaches it exactly.
func TestOverflowBoundCountsASettledOverrun(t *testing.T) {
	l := NewLedger()
	apply(t, l, reserveEntry("reserve-1", "r1", 10), delta("d1", "r1", "scope-1", reported(20)), settleEntry("settle-1", "r1"))
	wantFigures(t, l.Projection(), 20, 0, 0, 0, 10)
	if _, err := l.apply(reserveEntry("reserve-2", "r2", 9_223_372_036_854_775_788)); !errors.Is(err, ErrOverflow) {
		t.Fatalf("error = %v, want ErrOverflow", err)
	}
	apply(t, l, reserveEntry("reserve-2", "r2", 9_223_372_036_854_775_787))
	p := l.Projection()
	wantFigures(t, p, 20, 0, 9_223_372_036_854_775_787, 0, 10)
	if p.Committed() != 9_223_372_036_854_775_807 {
		t.Fatalf("committed = %d", p.Committed())
	}
}

// A rejected batch leaves no identity behind: its entry IDs, reservation and
// scope can be used again with different content, and nothing is a replay.
func TestRejectedBatchLeavesItsIdentitiesFree(t *testing.T) {
	l := NewLedger()
	apply(t, l, reserveEntry("reserve-1", "r1", 100))
	before := l.clone()

	batch := []Entry{
		reserveEntry("reserve-2", "r2", 50),
		cumulative("u1", "r2", "scope-1", 1, reported(20)),
		holdEntry("hold-1", "r2", HoldTimeout),
		{ID: "x1", Kind: KindExclusion, Detail: "autopilot_quota"},
		settleEntry("settle-1", "r9"), // names a reservation that does not exist
	}
	if rs, err := l.apply(batch...); !errors.Is(err, ErrUnknownReservation) || rs != nil {
		t.Fatalf("batch = %+v, %v; want ErrUnknownReservation", rs, err)
	}
	if !reflect.DeepEqual(l, before) {
		t.Fatal("the rejected batch changed the ledger")
	}

	// Same IDs, different content: accepted as new, as delta on the same scope.
	rs := apply(t, l, reserveEntry("reserve-2", "r2", 60), delta("u1", "r2", "scope-1", reported(25)))
	if rs[0].Replayed || rs[1].Replayed || rs[1].ActualAdded != 25 {
		t.Fatalf("receipts = %+v", rs)
	}
	p := l.Projection()
	wantFigures(t, p, 25, 0, 135, 0, 0)
	wantUnknown(t, p)
	if len(p.Coverage.NotApplicable) != 0 || p.Coverage.IncludedAttempts != 2 {
		t.Fatalf("coverage = %+v", p.Coverage)
	}
}
