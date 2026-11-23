package budget

import (
	"errors"
	"fmt"
	"math/rand"
	"reflect"
	"sync"
	"testing"
)

// published builds a ledger that holds every kind of state a derived ledger
// could share with it: a sized reservation metered cumulatively and then
// held, an unsized one with a cost-less delta event, a gap and an exclusion.
// A = 30, H = 70, and four unknown items.
func published(t *testing.T) *Ledger {
	t.Helper()
	l := NewLedger()
	apply(t, l,
		reserveEntry("reserve-1", "r1", 100),
		cumulative("e1", "r1", "scope-1", 1, reported(30)),
		unsizedReserve("reserve-2", "r2"),
		delta("d1", "r2", "scope-2", Cost{}),
		holdEntry("lost-1", "r1", HoldLostContact),
		Entry{ID: "gap-1", Kind: KindGap, GapReason: ReasonCoverageGap, Detail: "legacy_protocol"},
		Entry{ID: "x-1", Kind: KindExclusion, Detail: "chat"},
	)
	return l
}

// later is what arrives after the published state. It writes to every part of
// a ledger: it moves the head of scope-1 (A 30 -> 80), replaces d1 by a costed
// event (A + 12), settles both reservations, which takes three items out of
// the unknown list and releases the 20 r1 had left, opens r3 for 40 and names
// a second exclusion. Afterwards A = 92, R = 40 and one unknown item is left.
func later() []Entry {
	fix := delta("d2", "r2", "scope-2", reported(12))
	fix.Usage.Supersedes, fix.Usage.CorrectionAuthorized = "d1", true
	return []Entry{
		cumulative("e2", "r1", "scope-1", 2, reported(80)),
		fix,
		settleEntry("settle-2", "r2"),
		settleEntry("settle-1", "r1"),
		reserveEntry("reserve-3", "r3", 40),
		{ID: "x-2", Kind: KindExclusion, Detail: "autopilot_quota"},
	}
}

// With returns the next state and leaves its receiver as it was, so a
// published ledger can be read while later states are derived from it.
//
// The states are compared with ledgers built again from their entries, not
// with copies the ledger makes of itself: a copy that shares state with its
// origin would change along with it and hide the fault.
func TestWithLeavesTheReceiverUntouched(t *testing.T) {
	l := published(t)
	next, receipts, err := l.With(later()...)
	if err != nil || next == nil || len(receipts) != 6 {
		t.Fatalf("With = %v, %+v, %v", next, receipts, err)
	}
	if !reflect.DeepEqual(l, published(t)) {
		t.Fatal("With changed its receiver")
	}
	p := l.Projection()
	wantFigures(t, p, 30, 0, 0, 70, 0)
	wantUnknown(t, p, ReasonReservationUnsized, ReasonCostAbsent, "lost_contact", ReasonCoverageGap)
	if l.Len() != 7 || l.HasReservation("r3") {
		t.Fatalf("receiver has Len %d and r3 %v", l.Len(), l.HasReservation("r3"))
	}

	p = next.Projection()
	wantFigures(t, p, 92, 0, 40, 0, 0)
	wantUnknown(t, p, ReasonCoverageGap)
	if next.Len() != 13 || !next.HasReservation("r3") {
		t.Fatalf("result has Len %d and r3 %v", next.Len(), next.HasReservation("r3"))
	}
	if receipts[0].ActualAdded != 50 || receipts[1].ActualAdded != 12 || receipts[2].Released != 0 || receipts[3].Released != 20 {
		t.Fatalf("receipts = %+v", receipts)
	}
	for i, r := range receipts {
		if r.Replayed || r.EntryID != later()[i].ID {
			t.Fatalf("receipt %d = %+v", i, r)
		}
	}

	// The result is a ledger of its own. What is appended to it afterwards (a
	// moved scope head, new unknown items, a write to every map) never reaches
	// the ledger it came from.
	more := []Entry{
		cumulative("e3", "r1", "scope-1", 3, reported(95)),
		holdEntry("timeout-3", "r3", HoldTimeout),
		delta("d3", "r3", "scope-2", Cost{}),
		{ID: "x-3", Kind: KindExclusion, Detail: "plugin_external_call"},
	}
	apply(t, next, more...)
	if !reflect.DeepEqual(l, published(t)) {
		t.Fatal("a derived ledger changed the one it came from")
	}

	// Nor does the origin, moving on by itself, reach the derived ledger. Its
	// scope-1 still ends at sequence 1, so sequence 2 is free there and the
	// running total goes from 30 to 31.
	own := apply(t, l,
		cumulative("e2", "r1", "scope-1", 2, reported(31)),
		holdEntry("timeout-2", "r2", HoldTimeout),
		Entry{ID: "x-9", Kind: KindExclusion, Detail: "collaboration_host_run"},
	)
	if own[0].Disposition != DispositionAccepted || own[0].ActualAdded != 1 || own[0].Replayed || own[1].Disposition != DispositionAccepted {
		t.Fatalf("the origin's own entries = %+v", own)
	}
	p = l.Projection()
	wantFigures(t, p, 31, 0, 0, 69, 0)
	wantUnknown(t, p, ReasonReservationUnsized, ReasonCostAbsent, "lost_contact", ReasonCoverageGap, "timeout")

	want := published(t)
	apply(t, want, later()...)
	apply(t, want, more...)
	if !reflect.DeepEqual(next, want) {
		t.Fatal("the origin changed a ledger derived from it")
	}
	p = next.Projection()
	wantFigures(t, p, 107, 0, 0, 40, 0)
	wantUnknown(t, p, ReasonCoverageGap, "timeout", ReasonCostAbsent)
}

// Two ledgers derived from one published state are siblings: a request that
// failed after deriving leaves its ledger behind, and the next one derives
// from the same state again. Both use the same entry IDs, reservation and
// scope with different content, and each must see only its own.
func TestLedgersDerivedFromOneOriginDoNotAffectEachOther(t *testing.T) {
	origin := published(t)
	// Room behind the end of the unknown list is where an append that reused
	// the origin's list would put the items of both siblings.
	origin.unknown = append(make([]UnknownItem, 0, 16), origin.unknown...)

	batch := func(total, reserve Ticks, detail string) []Entry {
		return []Entry{
			cumulative("e2", "r1", "scope-1", 2, reported(total)),
			{ID: "gap-2", Kind: KindGap, ReservationID: "r2", GapReason: ReasonCoverageGap, Detail: detail},
			reserveEntry("reserve-3", "r3", reserve),
			{ID: "x-2", Kind: KindExclusion, Detail: detail},
		}
	}
	check := func(name string, l *Ledger, actual, reserved, hold Ticks, detail string) {
		t.Helper()
		p := l.Projection()
		wantFigures(t, p, actual, 0, reserved, hold, 0)
		wantUnknown(t, p, ReasonReservationUnsized, ReasonCostAbsent, "lost_contact", ReasonCoverageGap, ReasonCoverageGap)
		if last := p.Unknown[4]; last != (UnknownItem{Reason: ReasonCoverageGap, EntryID: "gap-2", ReservationID: "r2", Detail: detail}) {
			t.Fatalf("%s: last unknown item = %+v", name, last)
		}
		if !reflect.DeepEqual(p.Coverage, Coverage{IncludedAttempts: 3, NotApplicable: []string{"chat", detail}}) {
			t.Fatalf("%s: coverage = %+v", name, p.Coverage)
		}
	}

	// r1 holds what is left of its 100: 50 after a total of 50, 30 after 70.
	one, receipts, err := origin.With(batch(50, 10, "one")...)
	if err != nil || receipts[0].ActualAdded != 20 {
		t.Fatalf("first sibling = %+v, %v", receipts, err)
	}
	check("first sibling", one, 50, 10, 50, "one")

	other, receipts, err := origin.With(batch(70, 20, "other")...)
	if err != nil {
		t.Fatalf("second sibling: %v", err)
	}
	for _, r := range receipts {
		if r.Replayed || r.Disposition != DispositionAccepted {
			t.Fatalf("second sibling receipt = %+v", r)
		}
	}
	if receipts[0].ActualAdded != 40 {
		t.Fatalf("second sibling usage receipt = %+v", receipts[0])
	}
	check("second sibling", other, 70, 20, 30, "other")

	// Deriving the second changed neither the first nor the origin.
	check("first sibling, afterwards", one, 50, 10, 50, "one")
	want := published(t)
	apply(t, want, batch(50, 10, "one")...)
	if !reflect.DeepEqual(one, want) || !reflect.DeepEqual(origin, published(t)) {
		t.Fatal("a sibling changed the first one or the origin")
	}
}

// With no entries the result is a separate ledger in the same state, also for
// the zero Ledger.
func TestWithNoEntriesIsAnIndependentCopy(t *testing.T) {
	l := published(t)
	same, receipts, err := l.With()
	if err != nil || len(receipts) != 0 || same == l || !reflect.DeepEqual(same, l) {
		t.Fatalf("With() = %v, %+v, %v", same, receipts, err)
	}
	apply(t, same, later()...)
	if !reflect.DeepEqual(l, published(t)) {
		t.Fatal("writing to the copy changed the original")
	}

	var zero Ledger
	next, _, err := zero.With(reserveEntry("reserve-1", "r1", 100), delta("d1", "r1", "scope-1", reported(30)))
	if err != nil {
		t.Fatal(err)
	}
	wantFigures(t, next.Projection(), 30, 0, 70, 0, 0)
	if !reflect.DeepEqual(zero, Ledger{}) {
		t.Fatal("With changed the zero Ledger")
	}
}

// A batch that fails returns no ledger and no receipts, and nothing of its
// valid part is kept anywhere.
func TestWithOnErrorReturnsNothingAndLeavesTheReceiverUntouched(t *testing.T) {
	l := published(t)
	for name, tc := range map[string]struct {
		last Entry
		want error
	}{
		"invalid entry":       {Entry{ID: "bad", Kind: "refund"}, ErrInvalidEntry},
		"identity conflict":   {reserveEntry("reserve-1", "r1", 101), ErrConflict},
		"unknown reservation": {settleEntry("settle-9", "r9"), ErrUnknownReservation},
		"incomplete proof":    {Entry{ID: "settle-3", Kind: KindSettle, ReservationID: "r3", Proof: ReleaseProof{FinalUsage: true}}, ErrReleaseProofIncomplete},
		"mode mismatch":       {delta("d9", "r1", "scope-1", reported(1)), ErrModeMismatch},
		"overflow":            {reserveEntry("reserve-9", "r9", 9_223_372_036_854_775_807), ErrOverflow},
		"negative amount":     {reserveEntry("reserve-9", "r9", -1), ErrNegativeAmount},
	} {
		next, receipts, err := l.With(append(later(), tc.last)...)
		if !errors.Is(err, tc.want) || next != nil || receipts != nil {
			t.Errorf("%s: With = %v, %+v, %v; want nothing and %v", name, next, receipts, err, tc.want)
		}
		if !reflect.DeepEqual(l, published(t)) {
			t.Fatalf("%s: a failed With changed its receiver", name)
		}
	}
	// The valid part of the failed batches is new when it is sent again.
	next, receipts, err := l.With(later()...)
	if err != nil || next.Len() != l.Len()+6 {
		t.Fatalf("With after the failures = %v, %v", next, err)
	}
	for _, r := range receipts {
		if r.Replayed {
			t.Fatalf("receipt %+v is a replay of a batch that failed", r)
		}
	}
}

// apply is With followed by taking the result: both give the same state and
// the same receipts, for new entries and for replayed ones.
func TestWithGivesWhatApplyGives(t *testing.T) {
	derived, got, err := published(t).With(later()...)
	if err != nil {
		t.Fatal(err)
	}
	inPlace := published(t)
	want := apply(t, inPlace, later()...)
	if !reflect.DeepEqual(derived, inPlace) || !reflect.DeepEqual(got, want) {
		t.Fatalf("With gave %+v, apply gave %+v", got, want)
	}

	again, replayed, err := derived.With(later()...)
	if err != nil || !reflect.DeepEqual(again, derived) {
		t.Fatalf("replay through With changed the state (%v)", err)
	}
	for i, r := range replayed {
		original := want[i]
		original.Replayed = true
		if r != original {
			t.Fatalf("replay of entry %d = %+v, want %+v", i, r, original)
		}
	}
}

// The loader replays a root's whole history when it has nothing cached and
// only the entries after the cached revision when it has. One entry at a
// time, a prefix and then the rest, or everything at once must give the same
// ledger, and no state along the way may be changed by the ones after it.
func TestChainOfWithEqualsAFullReplay(t *testing.T) {
	for seed := int64(1); seed <= 150; seed++ {
		rng := rand.New(rand.NewSource(seed))
		full := NewLedger()
		var accepted []Entry
		var receipts []Receipt
		for step := 0; step < 60; step++ {
			e := randomEntryWithUnsized(rng, len(accepted))
			if rs, err := full.apply(e); err == nil && !rs[0].Replayed {
				accepted, receipts = append(accepted, e), append(receipts, rs[0])
			}
		}

		states := []*Ledger{NewLedger()}
		for i, e := range accepted {
			next, rs, err := states[i].With(e)
			if err != nil || len(rs) != 1 || rs[0] != receipts[i] {
				t.Fatalf("seed %d entry %d: With = %+v, %v; want %+v", seed, i, rs, err, receipts[i])
			}
			if next.Len() != i+1 {
				t.Fatalf("seed %d entry %d: Len = %d", seed, i, next.Len())
			}
			states = append(states, next)
		}
		if chain := states[len(accepted)]; !reflect.DeepEqual(chain, full) {
			t.Fatalf("seed %d: the chain and apply reach different states", seed)
		}
		if p, _, err := Reduce(accepted); err != nil || !reflect.DeepEqual(p, full.Projection()) {
			t.Fatalf("seed %d: Reduce = %+v, %v; want %+v", seed, p, err, full.Projection())
		}

		// A cached prefix and then the rest. The prefix is the state the chain
		// had at that point, still: nothing derived later has changed it.
		for cut := range states {
			prefix, _, err := NewLedger().With(accepted[:cut]...)
			if err != nil || !reflect.DeepEqual(prefix, states[cut]) {
				t.Fatalf("seed %d: the state after %d entries changed after it was derived from (%v)", seed, cut, err)
			}
			if cut%7 != int(seed%7) {
				continue
			}
			whole, rs, err := prefix.With(accepted[cut:]...)
			if err != nil || !reflect.DeepEqual(whole, full) || !reflect.DeepEqual(rs, receipts[cut:]) {
				t.Fatalf("seed %d: prefix %d and then the rest differs from the full replay (%v)", seed, cut, err)
			}
			if !reflect.DeepEqual(prefix, states[cut]) {
				t.Fatalf("seed %d: deriving the rest changed the prefix of %d entries", seed, cut)
			}
		}
	}
}

// The service publishes a ledger and derives from it while other requests read
// it. Under -race this reports a With that writes to its receiver, even if it
// puts everything back, and a derived ledger that shares a map, a scope or the
// unknown list with its origin.
func TestWithIsSafeBesideReadersOfTheOrigin(t *testing.T) {
	origin := published(t)
	wantP := published(t).Projection()

	const readers, derivers, rounds = 4, 4, 150
	var wg sync.WaitGroup
	for g := 0; g < readers; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < rounds; i++ {
				if p := origin.Projection(); !reflect.DeepEqual(p, wantP) {
					t.Errorf("a reader saw %+v, want %+v", p, wantP)
					return
				}
				if origin.Len() != 7 || !origin.HasReservation("r2") || origin.HasReservation("r3") {
					t.Errorf("a reader saw Len %d, r2 %v, r3 %v", origin.Len(), origin.HasReservation("r2"), origin.HasReservation("r3"))
					return
				}
			}
		}()
	}
	for g := 0; g < derivers; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < rounds; i++ {
				next, _, err := origin.With(later()...)
				if err != nil {
					t.Errorf("With: %v", err)
					return
				}
				// Each deriver then writes to its own ledger in place.
				id := fmt.Sprintf("g%d-%d", g, i)
				if _, err := next.apply(
					cumulative(id, "r1", "scope-1", Count(3+i), reported(Ticks(80+i))),
					Entry{ID: id + "-gap", Kind: KindGap, ReservationID: "r3", GapReason: ReasonCoverageGap},
					holdEntry(id+"-hold", "r3", HoldTimeout),
				); err != nil {
					t.Errorf("apply on a derived ledger: %v", err)
					return
				}
				if failed, _, err := origin.With(later()[0], Entry{ID: id, Kind: "refund"}); err == nil || failed != nil {
					t.Errorf("With accepted an invalid batch: %v, %v", failed, err)
					return
				}
			}
		}(g)
	}
	wg.Wait()
	if !reflect.DeepEqual(origin, published(t)) {
		t.Fatal("the origin changed while ledgers were derived from it")
	}
}

// Len is the number of distinct entries the ledger holds. The loader compares
// it with the root's ledger revision.
func TestLenCountsDistinctAcceptedEntries(t *testing.T) {
	var zero Ledger
	if zero.Len() != 0 || NewLedger().Len() != 0 {
		t.Fatalf("empty ledgers have Len %d and %d", zero.Len(), NewLedger().Len())
	}

	l := NewLedger()
	apply(t, l, reserveEntry("reserve-1", "r1", 100), cumulative("v2", "r1", "scope-1", 2, reported(50)))
	if l.Len() != 2 {
		t.Fatalf("Len = %d, want 2", l.Len())
	}

	// An entry that is kept as a record counts, whatever became of it.
	rollup := delta("rollup-1", "", "parent-rollup", reported(100))
	rollup.Usage.Overlap = OverlapRollup
	rs := apply(t, l,
		cumulative("v1", "r1", "scope-1", 1, reported(20)),
		cumulative("v1-other", "r1", "scope-1", 1, reported(20)),
		rollup,
	)
	if rs[0].Disposition != DispositionStale || rs[1].Disposition != DispositionQuarantined || rs[2].Disposition != DispositionComparison {
		t.Fatalf("receipts = %+v", rs)
	}
	if l.Len() != 5 {
		t.Fatalf("Len = %d, want 5", l.Len())
	}

	// A replay is not another entry, alone or beside a new one.
	rs = apply(t, l, reserveEntry("reserve-1", "r1", 100), unsizedReserve("reserve-2", "r2"), reserveEntry("reserve-1", "r1", 100))
	if !rs[0].Replayed || rs[1].Replayed || !rs[2].Replayed {
		t.Fatalf("receipts = %+v", rs)
	}
	if l.Len() != 6 {
		t.Fatalf("Len = %d, want 6", l.Len())
	}

	// A refused entry is not kept, and neither is the valid part of its batch.
	if _, err := l.apply(reserveEntry("reserve-3", "r3", 1), Entry{ID: "bad", Kind: "refund"}); !errors.Is(err, ErrInvalidEntry) {
		t.Fatalf("error = %v, want ErrInvalidEntry", err)
	}
	if _, err := l.apply(reserveEntry("reserve-1", "r1", 101)); !errors.Is(err, ErrConflict) {
		t.Fatalf("error = %v, want ErrConflict", err)
	}
	if l.Len() != 6 {
		t.Fatalf("Len = %d, want 6", l.Len())
	}

	// With counts in its result and not in its receiver.
	next, _, err := l.With(holdEntry("hold-1", "r1", HoldTimeout), holdEntry("hold-1", "r1", HoldTimeout))
	if err != nil || next.Len() != 7 || l.Len() != 6 {
		t.Fatalf("With: %v, Len %d and %d; want 7 and 6", err, next.Len(), l.Len())
	}
}

// HasReservation tells an attempt whose reservation is in the ledger from one
// that never got one (design section 10.5). Settling does not forget it.
func TestHasReservation(t *testing.T) {
	var zero Ledger
	if zero.HasReservation("r1") || NewLedger().HasReservation("r1") || NewLedger().HasReservation("") {
		t.Fatal("an empty ledger has a reservation")
	}

	l := NewLedger()
	apply(t, l, reserveEntry("reserve-1", "r1", 100), unsizedReserve("reserve-2", "r2"), delta("d1", "", "scope-1", reported(5)))
	for id, want := range map[string]bool{
		"r1": true, "r2": true,
		"r3": false, "": false,
		// Only a reservation ID names a reservation.
		"reserve-1": false, "reserve-2": false, "d1": false, "scope-1": false,
	} {
		if got := l.HasReservation(id); got != want {
			t.Errorf("HasReservation(%q) = %v, want %v", id, got, want)
		}
	}

	// A held and a settled reservation were still applied.
	apply(t, l, holdEntry("lost-1", "r1", HoldLostContact))
	if !l.HasReservation("r1") {
		t.Fatal("a held reservation is gone")
	}
	apply(t, l, settleEntry("settle-1", "r1"), settleEntry("settle-2", "r2"))
	if !l.HasReservation("r1") || !l.HasReservation("r2") {
		t.Fatal("a settled reservation is gone")
	}

	// A reserve entry in a refused batch was never applied.
	if _, err := l.apply(reserveEntry("reserve-3", "r3", 1), settleEntry("settle-9", "r9")); !errors.Is(err, ErrUnknownReservation) {
		t.Fatalf("error = %v, want ErrUnknownReservation", err)
	}
	if l.HasReservation("r3") {
		t.Fatal("a refused batch left its reservation behind")
	}

	// With: the result has it, the receiver does not.
	next, _, err := l.With(reserveEntry("reserve-3", "r3", 1))
	if err != nil || !next.HasReservation("r3") || l.HasReservation("r3") {
		t.Fatalf("With: %v, result %v, receiver %v", err, next.HasReservation("r3"), l.HasReservation("r3"))
	}
}
