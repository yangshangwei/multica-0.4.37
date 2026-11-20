package budget

import (
	"errors"
	"fmt"
	"math/rand"
	"reflect"
	"testing"
)

// The tests in this file describe what a hold entry gets without looking at
// how the ledger remembers it: a lifecycle reason is recorded once for a
// reservation, and what is recorded is what the unknown set shows.

// allHoldReasons lists the lifecycle facts a hold entry can carry.
var allHoldReasons = []HoldReason{HoldCancelled, HoldTimeout, HoldProcessExit, HoldLostContact, HoldSupersededUnproven}

// A reservation records each reason once and each reason by itself: with
// some on record the others are still new facts, and a recorded one stays
// recorded whatever comes after it. The reasons arrive in every rotation of
// their order, on a sized and on an unsized reservation.
func TestEachHoldReasonIsRecordedOnceForAReservation(t *testing.T) {
	for _, unsized := range []bool{false, true} {
		for start := range allHoldReasons {
			name := fmt.Sprintf("unsized %v, starting with %s", unsized, allHoldReasons[start])
			l := NewLedger()
			wantItems := []UnknownItem{}
			if unsized {
				apply(t, l, unsizedReserve("reserve-1", "r1"))
				wantItems = append(wantItems, UnknownItem{Reason: ReasonReservationUnsized, EntryID: "reserve-1", ReservationID: "r1"})
			} else {
				apply(t, l, reserveEntry("reserve-1", "r1", 100))
			}
			var recorded []HoldReason
			for k := range allHoldReasons {
				reason := allHoldReasons[(start+k)%len(allHoldReasons)]
				first := holdEntry("first-"+string(reason), "r1", reason)
				first.Detail = "seen as " + string(reason)
				got := apply(t, l, first)[0]
				if want := (Receipt{EntryID: first.ID, Kind: KindHold, Disposition: DispositionAccepted, Reason: UnknownReason(reason)}); got != want {
					t.Fatalf("%s: first %s = %+v, want %+v", name, reason, got, want)
				}
				recorded = append(recorded, reason)
				wantItems = append(wantItems, UnknownItem{Reason: UnknownReason(reason), EntryID: first.ID, ReservationID: "r1", Detail: first.Detail})
				// Every reason on record is stale when it comes again: the one
				// just recorded and the ones before it. The entry that repeats
				// it differs in everything but the reason.
				for _, again := range recorded {
					id := fmt.Sprintf("%s-again-after-%s", again, reason)
					got := apply(t, l, holdEntry(id, "r1", again))[0]
					if want := (Receipt{EntryID: id, Kind: KindHold, Disposition: DispositionStale}); got != want {
						t.Fatalf("%s: %s again after %s = %+v, want %+v", name, again, reason, got, want)
					}
				}
				if p := l.Projection(); !reflect.DeepEqual(p.Unknown, wantItems) {
					t.Fatalf("%s: unknown after %s = %+v, want %+v", name, reason, p.Unknown, wantItems)
				}
			}
			// Five facts hold the reservation once. Nothing is spent, so a sized
			// one has its whole amount in H and an unsized one has no figure.
			if unsized {
				wantFigures(t, l.Projection(), 0, 0, 0, 0, 0)
			} else {
				wantFigures(t, l.Projection(), 0, 0, 0, 100, 0)
			}
		}
	}
}

// What one reservation has on record says nothing about another, whether the
// other was opened before the fact or after it.
func TestHoldReasonsAreRecordedPerReservation(t *testing.T) {
	l := NewLedger()
	apply(t, l, reserveEntry("reserve-1", "r1", 100), unsizedReserve("reserve-2", "r2"))
	hold := func(reservation string, reason HoldReason, want Disposition) {
		t.Helper()
		id := fmt.Sprintf("%s-%s-%d", reservation, reason, l.Len())
		if got := apply(t, l, holdEntry(id, reservation, reason))[0]; got.Disposition != want {
			t.Fatalf("%s on %s = %+v, want %s", reason, reservation, got, want)
		}
	}
	for _, reason := range allHoldReasons {
		hold("r1", reason, DispositionAccepted)
		// r1 has just recorded it. r2 has not.
		hold("r2", reason, DispositionAccepted)
		hold("r1", reason, DispositionStale)
		hold("r2", reason, DispositionStale)
	}
	// r3 opens when r1 and r2 have every reason on record.
	apply(t, l, reserveEntry("reserve-3", "r3", 40))
	for _, reason := range allHoldReasons {
		hold("r3", reason, DispositionAccepted)
		hold("r3", reason, DispositionStale)
		hold("r1", reason, DispositionStale)
	}

	p := l.Projection()
	wantFigures(t, p, 0, 0, 0, 140, 0)
	counts := p.UnknownCounts()
	if len(p.Unknown) != 16 || len(counts) != 6 || counts[ReasonReservationUnsized] != 1 {
		t.Fatalf("unknown = %+v", p.Unknown)
	}
	for _, reason := range allHoldReasons {
		if counts[UnknownReason(reason)] != 3 {
			t.Fatalf("%s is recorded %d times, want once for each of three reservations", reason, counts[UnknownReason(reason)])
		}
	}
}

// Once a reservation has settled every lifecycle fact about it is stale: a
// reason it recorded before the settlement and one it never recorded. Its
// settlement changes nothing for a reservation that has not settled.
func TestHoldAfterSettlementIsStaleWhateverWasRecorded(t *testing.T) {
	l := NewLedger()
	apply(t, l,
		reserveEntry("reserve-1", "r1", 100), reserveEntry("reserve-2", "r2", 50), unsizedReserve("reserve-3", "r3"),
		holdEntry("timeout-1", "r1", HoldTimeout), holdEntry("timeout-2", "r2", HoldTimeout),
	)
	if rs := apply(t, l, settleEntry("settle-1", "r1"), settleEntry("settle-3", "r3")); rs[0].Released != 100 || rs[1].Released != 0 {
		t.Fatalf("settlements = %+v", rs)
	}
	// r1 recorded a timeout before it settled. r3 recorded nothing.
	for _, reservation := range []string{"r1", "r3"} {
		for _, reason := range allHoldReasons {
			id := fmt.Sprintf("late-%s-%s", reservation, reason)
			got := apply(t, l, holdEntry(id, reservation, reason))[0]
			if want := (Receipt{EntryID: id, Kind: KindHold, Disposition: DispositionStale}); got != want {
				t.Fatalf("%s on the settled %s = %+v, want %+v", reason, reservation, got, want)
			}
		}
	}
	p := l.Projection()
	wantFigures(t, p, 0, 0, 0, 50, 0)
	wantUnknown(t, p, "timeout")
	if p.Unknown[0] != (UnknownItem{Reason: "timeout", EntryID: "timeout-2", ReservationID: "r2"}) {
		t.Fatalf("unknown item = %+v", p.Unknown[0])
	}

	// r2 has not settled: the reason it has is stale, another is a new fact.
	rs := apply(t, l, holdEntry("timeout-3", "r2", HoldTimeout), holdEntry("exit-2", "r2", HoldProcessExit))
	if rs[0].Disposition != DispositionStale || rs[1].Disposition != DispositionAccepted || rs[1].Reason != "process_exit" {
		t.Fatalf("holds on r2 = %+v", rs)
	}
	p = l.Projection()
	wantFigures(t, p, 0, 0, 0, 50, 0)
	wantUnknown(t, p, "timeout", "process_exit")
}

// The record belongs to one ledger state. A ledger derived by With knows what
// its origin recorded. What it records itself reaches neither the origin nor
// a ledger derived from the same origin beside it.
func TestRecordedHoldReasonsFollowWith(t *testing.T) {
	origin := NewLedger()
	apply(t, origin, reserveEntry("reserve-1", "r1", 100), holdEntry("timeout-1", "r1", HoldTimeout))

	derived, rs, err := origin.With(holdEntry("timeout-2", "r1", HoldTimeout), holdEntry("lost-1", "r1", HoldLostContact))
	if err != nil || rs[0].Disposition != DispositionStale || rs[1].Disposition != DispositionAccepted {
		t.Fatalf("derived: %+v, %v", rs, err)
	}
	// The origin has no lost contact on record, so a second ledger derived
	// from it records the reason under its own entry.
	sibling, rs, err := origin.With(holdEntry("lost-2", "r1", HoldLostContact))
	if err != nil || rs[0] != (Receipt{EntryID: "lost-2", Kind: KindHold, Disposition: DispositionAccepted, Reason: "lost_contact"}) {
		t.Fatalf("sibling: %+v, %v", rs, err)
	}
	// One step further down, both reasons are on record and a third is not.
	further, rs, err := derived.With(
		holdEntry("lost-3", "r1", HoldLostContact), holdEntry("timeout-3", "r1", HoldTimeout), holdEntry("cancel-1", "r1", HoldCancelled),
	)
	if err != nil || rs[0].Disposition != DispositionStale || rs[1].Disposition != DispositionStale || rs[2].Disposition != DispositionAccepted {
		t.Fatalf("further: %+v, %v", rs, err)
	}
	// The origin, moving on by itself, still takes the reason as a new fact.
	if r := apply(t, origin, holdEntry("lost-4", "r1", HoldLostContact))[0]; r.Disposition != DispositionAccepted {
		t.Fatalf("origin: %+v", r)
	}

	for name, tc := range map[string]struct {
		l    *Ledger
		want []string
	}{
		"origin":  {origin, []string{"timeout-1", "lost-4"}},
		"derived": {derived, []string{"timeout-1", "lost-1"}},
		"sibling": {sibling, []string{"timeout-1", "lost-2"}},
		"further": {further, []string{"timeout-1", "lost-1", "cancel-1"}},
	} {
		var got []string
		for _, item := range tc.l.Projection().Unknown {
			got = append(got, item.EntryID)
		}
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: the unknown items come from %v, want %v", name, got, tc.want)
		}
	}

	// The zero Ledger records as any other.
	var zero Ledger
	next, rs, err := zero.With(
		reserveEntry("reserve-1", "r1", 100), holdEntry("exit-1", "r1", HoldProcessExit), holdEntry("exit-2", "r1", HoldProcessExit),
	)
	if err != nil || rs[1].Disposition != DispositionAccepted || rs[2].Disposition != DispositionStale {
		t.Fatalf("zero ledger: %+v, %v", rs, err)
	}
	wantUnknown(t, next.Projection(), "process_exit")
}

// A refused batch records nothing, so the reason its hold carried is a new
// fact afterwards. Sending an entry again is a replay of its receipt and not
// a second fact: an accepted hold stays accepted.
func TestRefusedBatchAndReplayLeaveTheRecordAlone(t *testing.T) {
	l := NewLedger()
	apply(t, l, reserveEntry("reserve-1", "r1", 100))
	timeout := holdEntry("timeout-1", "r1", HoldTimeout)
	if _, err := l.apply(timeout, Entry{ID: "bad", Kind: "refund"}); !errors.Is(err, ErrInvalidEntry) {
		t.Fatalf("error = %v, want ErrInvalidEntry", err)
	}
	if next, _, err := l.With(timeout, settleEntry("settle-9", "r9")); !errors.Is(err, ErrUnknownReservation) || next != nil {
		t.Fatalf("With = %v, %v; want nothing and ErrUnknownReservation", next, err)
	}
	p := l.Projection()
	wantFigures(t, p, 0, 0, 100, 0, 0)
	wantUnknown(t, p)

	first := apply(t, l, timeout)[0]
	if first.Disposition != DispositionAccepted || first.Replayed {
		t.Fatalf("the hold after the refused batches = %+v", first)
	}
	repeated := holdEntry("timeout-2", "r1", HoldTimeout)
	second := apply(t, l, repeated)[0]
	if second.Disposition != DispositionStale {
		t.Fatalf("the same reason under another entry = %+v", second)
	}

	again := apply(t, l, timeout, repeated)
	first.Replayed, second.Replayed = true, true
	if again[0] != first || again[1] != second {
		t.Fatalf("replays = %+v, want %+v and %+v", again, first, second)
	}
	p = l.Projection()
	wantFigures(t, p, 0, 0, 0, 100, 0)
	wantUnknown(t, p, "timeout")
}

// Besides a settlement, three places take items out of the unknown set: a
// reported running total, which is also where a reported correction of one
// ends; an estimated correction of a running total; and a correction of a
// delta event. They take what usage left there and never a lifecycle fact.
// In each row r1 reports usage, is held, reports more usage, and then the
// entry under test removes some of what the usage left. The facts of r1 and
// of r2 beside it stay where they were, their reason is stale when it comes
// again and another reason is still a new fact.
func TestUsageRemovesNoLifecycleFact(t *testing.T) {
	const scope = "s1"
	correction := func(e Entry, original string) Entry {
		e.Usage.Supersedes, e.Usage.CorrectionAuthorized = original, true
		return e
	}
	estimate := func(entry string) UnknownItem {
		return UnknownItem{Reason: ReasonEstimateOnly, EntryID: entry, ReservationID: "r1", ScopeID: scope}
	}
	facts := []UnknownItem{
		{Reason: "timeout", EntryID: "timeout-1", ReservationID: "r1"},
		{Reason: "timeout", EntryID: "timeout-2", ReservationID: "r2"},
	}
	for _, row := range []struct {
		name string
		// early and late are the usage of r1 before its fact and after it.
		// Each leaves an estimate item, so the facts stand between the two.
		early, late Entry
		// remover is the entry under test. wantEarly and wantLate are the
		// usage items the set has after it, before the facts and after them.
		remover             Entry
		wantEarly, wantLate []UnknownItem
	}{
		{
			name:    "a reported running total",
			early:   cumulative("u1", "r1", scope, 1, estimated(30)),
			late:    cumulative("u2", "r1", scope, 2, estimated(35)),
			remover: cumulative("u3", "r1", scope, 3, reported(40)),
		},
		{
			name:    "a reported correction of a running total",
			early:   cumulative("u1", "r1", scope, 1, estimated(30)),
			late:    cumulative("u2", "r1", scope, 2, estimated(35)),
			remover: correction(cumulative("u3", "r1", scope, 3, reported(20)), "u2"),
		},
		{
			name:      "an estimated correction of a running total",
			early:     cumulative("u1", "r1", scope, 1, estimated(30)),
			late:      cumulative("u2", "r1", scope, 2, estimated(35)),
			remover:   correction(cumulative("u3", "r1", scope, 3, estimated(20)), "u2"),
			wantEarly: []UnknownItem{estimate("u1")},
			wantLate:  []UnknownItem{estimate("u3")},
		},
		{
			name:     "a reported correction of a delta event",
			early:    delta("u1", "r1", scope, estimated(30)),
			late:     delta("u2", "r1", scope, estimated(5)),
			remover:  correction(delta("u3", "r1", scope, reported(20)), "u1"),
			wantLate: []UnknownItem{estimate("u2")},
		},
		{
			name:      "an estimated correction of a delta event",
			early:     delta("u1", "r1", scope, estimated(30)),
			late:      delta("u2", "r1", scope, estimated(5)),
			remover:   correction(delta("u3", "r1", scope, estimated(20)), "u2"),
			wantEarly: []UnknownItem{estimate("u1")},
			wantLate:  []UnknownItem{estimate("u3")},
		},
	} {
		for _, unsized := range []bool{false, true} {
			name := fmt.Sprintf("%s, unsized %v", row.name, unsized)
			l := NewLedger()
			opened := []UnknownItem{}
			if unsized {
				apply(t, l, unsizedReserve("reserve-1", "r1"))
				opened = append(opened, UnknownItem{Reason: ReasonReservationUnsized, EntryID: "reserve-1", ReservationID: "r1"})
			} else {
				apply(t, l, reserveEntry("reserve-1", "r1", 100))
			}
			list := func(early, late []UnknownItem, more ...UnknownItem) []UnknownItem {
				items := append([]UnknownItem{}, opened...)
				items = append(append(append(items, early...), facts...), late...)
				return append(items, more...)
			}

			rs := apply(t, l,
				reserveEntry("reserve-2", "r2", 50), row.early,
				holdEntry("timeout-1", "r1", HoldTimeout), holdEntry("timeout-2", "r2", HoldTimeout),
				row.late,
			)
			if rs[1].Disposition != DispositionAccepted || rs[4].Disposition != DispositionAccepted {
				t.Fatalf("%s: the usage of r1 = %+v and %+v, want both accepted", name, rs[1], rs[4])
			}
			before := list([]UnknownItem{estimate("u1")}, []UnknownItem{estimate("u2")})
			if p := l.Projection(); !reflect.DeepEqual(p.Unknown, before) {
				t.Fatalf("%s: unknown before the entry = %+v, want %+v", name, p.Unknown, before)
			}

			if r := apply(t, l, row.remover)[0]; r.Disposition != DispositionAccepted {
				t.Fatalf("%s: the entry = %+v, want it accepted", name, r)
			}
			after := list(row.wantEarly, row.wantLate)
			if p := l.Projection(); !reflect.DeepEqual(p.Unknown, after) {
				t.Fatalf("%s: unknown after the entry = %+v, want %+v", name, p.Unknown, after)
			}

			// The reason is on record for both reservations, as a search of
			// the unknown set says too.
			for _, reservation := range []string{"r1", "r2"} {
				again := holdEntry("timeout-again-"+reservation, reservation, HoldTimeout)
				searched, _ := wantHold(again, false, l.Projection().Unknown)
				got := apply(t, l, again)[0]
				if want := (Receipt{EntryID: again.ID, Kind: KindHold, Disposition: DispositionStale}); got != want || searched != want {
					t.Fatalf("%s: timeout again on %s = %+v, a search of the unknown set gives %+v, want %+v", name, reservation, got, searched, want)
				}
			}
			got := apply(t, l, holdEntry("lost-1", "r1", HoldLostContact))[0]
			if want := (Receipt{EntryID: "lost-1", Kind: KindHold, Disposition: DispositionAccepted, Reason: "lost_contact"}); got != want {
				t.Fatalf("%s: another reason on r1 = %+v, want %+v", name, got, want)
			}
			end := list(row.wantEarly, row.wantLate, UnknownItem{Reason: "lost_contact", EntryID: "lost-1", ReservationID: "r1"})
			if p := l.Projection(); !reflect.DeepEqual(p.Unknown, end) {
				t.Fatalf("%s: unknown at the end = %+v, want %+v", name, p.Unknown, end)
			}
		}
	}
}

// A lifecycle fact enters the unknown set through a hold entry and through
// nothing else. A gap entry is the other entry that names its own reason, and
// it cannot name one of these: the fact would be in the set without having
// been recorded for the reservation. Refused, it leaves the reason a new fact.
func TestGapEntryCannotCarryALifecycleReason(t *testing.T) {
	l := NewLedger()
	apply(t, l, reserveEntry("reserve-1", "r1", 100))
	var recorded []UnknownReason
	for _, reason := range allHoldReasons {
		for _, reservation := range []string{"r1", ""} {
			gap := Entry{ID: "gap-" + string(reason), Kind: KindGap, ReservationID: reservation, GapReason: UnknownReason(reason)}
			if _, err := l.apply(gap); !errors.Is(err, ErrInvalidEntry) {
				t.Fatalf("a gap entry with the reason %s for the reservation %q: error = %v, want ErrInvalidEntry", reason, reservation, err)
			}
		}
		id := "hold-" + string(reason)
		got := apply(t, l, holdEntry(id, "r1", reason))[0]
		if want := (Receipt{EntryID: id, Kind: KindHold, Disposition: DispositionAccepted, Reason: UnknownReason(reason)}); got != want {
			t.Fatalf("%s after the refused gap entries = %+v, want %+v", reason, got, want)
		}
		recorded = append(recorded, UnknownReason(reason))
	}
	wantUnknown(t, l.Projection(), recorded...)
}

// randomEntryWithHolds is randomEntryWithUnsized with a third of the entries
// that are not reservations replaced by a lifecycle fact under an ID of its
// own. Four reservations and five reasons leave twenty facts to record, so
// the same reason soon comes back: on the reservation that has it, on its
// neighbours, and after settlements.
func randomEntryWithHolds(rng *rand.Rand, n int) Entry {
	e := randomEntryWithUnsized(rng, n)
	if e.Kind == KindReserve || rng.Intn(3) != 0 {
		return e
	}
	e = holdEntry(fmt.Sprintf("h%d", n), fmt.Sprintf("r%d", rng.Intn(4)), allHoldReasons[rng.Intn(len(allHoldReasons))])
	if rng.Intn(4) == 0 {
		e.Detail = fmt.Sprintf("note %d", rng.Intn(3))
	}
	return e
}

// acceptedFor reports whether the history accepted an entry of this kind for
// the reservation: a reserve entry that opened it, a settle entry that ended
// it.
func acceptedFor(entries []Entry, receipts []Receipt, kind EntryKind, reservation string) bool {
	for i, e := range entries {
		if e.Kind == kind && e.ReservationID == reservation && receipts[i].Disposition == DispositionAccepted {
			return true
		}
	}
	return false
}

// factsOf lists the lifecycle reasons an unknown list has for a reservation,
// in the order of the list.
func factsOf(unknown []UnknownItem, reservation string) []UnknownReason {
	var facts []UnknownReason
	for _, item := range unknown {
		if item.ReservationID == reservation && isLifecycleReason(item.Reason) {
			facts = append(facts, item.Reason)
		}
	}
	return facts
}

// heldReasons lists the lifecycle reasons a history recorded for a
// reservation and no settlement has resolved, in the order they were
// accepted. A settled reservation has none: its settlement resolved those
// before it, and every hold after it is stale.
func heldReasons(entries []Entry, receipts []Receipt, reservation string) []UnknownReason {
	var reasons []UnknownReason
	for i, e := range entries {
		if e.ReservationID != reservation || receipts[i].Disposition != DispositionAccepted {
			continue
		}
		switch e.Kind {
		case KindHold:
			reasons = append(reasons, UnknownReason(e.HoldReason))
		case KindSettle:
			reasons = nil
		}
	}
	return reasons
}

// wantHold is what an unknown list says a new hold entry must get. The entry
// is stale if its reservation has settled or the list already has its reason
// for that reservation. Otherwise it is accepted and adds one item at the end
// of the list.
func wantHold(e Entry, settled bool, unknown []UnknownItem) (Receipt, []UnknownItem) {
	reason := UnknownReason(e.HoldReason)
	recorded := false
	for _, item := range unknown {
		recorded = recorded || (item.ReservationID == e.ReservationID && item.Reason == reason)
	}
	if settled || recorded {
		return Receipt{EntryID: e.ID, Kind: KindHold, Disposition: DispositionStale}, unknown
	}
	item := UnknownItem{Reason: reason, EntryID: e.ID, ReservationID: e.ReservationID, Detail: e.Detail}
	return Receipt{EntryID: e.ID, Kind: KindHold, Disposition: DispositionAccepted, Reason: reason},
		append(append([]UnknownItem{}, unknown...), item)
}

// itemsGone counts the items of before that after no longer has. An entry
// leaves at most one item, so the entry ID tells the items apart.
func itemsGone(before, after []UnknownItem) int {
	kept := map[string]bool{}
	for _, item := range after {
		kept[item.EntryID] = true
	}
	gone := 0
	for _, item := range before {
		if !kept[item.EntryID] {
			gone++
		}
	}
	return gone
}

// Random interleavings of every entry kind, dense in lifecycle facts. Before
// each hold entry the test searches the unknown set the ledger shows for the
// entry's reservation and reason, and the receipt has to agree with what it
// found. After each entry the unknown set has to show, for every reservation,
// the reasons its accepted holds recorded and nothing else: no entry takes a
// fact out but the settlement of its reservation.
//
// Of the entries that remove unknown items, these histories have settlements
// and a few reported running totals. They have no correction that removes one
// beside a fact: TestUsageRemovesNoLifecycleFact is where those are tested.
//
// The ledger is a chain of states derived by With, with a second one applied
// in place beside it. Every state of the chain stays in use: after each step
// one of them, picked at random, is asked about a hold of its own.
func TestHoldDispositionMatchesASearchOfTheUnknownList(t *testing.T) {
	// How often the generator reached each case this test is about.
	var seen struct {
		// first: a reservation's first fact. further: another reason on a
		// reservation that already has one.
		first, further int
		// repeated: stale because the reason is on record. older: the same,
		// and the reservation recorded another reason after that one.
		repeated, older int
		// settled: stale because the reservation has settled. unrecorded: the
		// same, with a reason the reservation never recorded.
		settled, unrecorded int
		// neighbour: accepted although another reservation has the reason.
		neighbour int
		// unsized: accepted on an unsized reservation. unsizedRepeated: stale
		// on one because the reason is on record.
		unsized, unsizedRepeated int
		// earlier: an earlier state of the chain accepted a hold that the
		// newest state would call stale.
		earlier int
		// An entry took items out of the unknown set while the set had a fact
		// that had to stay. settledBeside: a settlement, beside the fact of
		// another reservation. totalBeside: a reported running total.
		// correctedBeside: a correction of usage.
		settledBeside, totalBeside, correctedBeside int
	}
	for seed := int64(1); seed <= 300; seed++ {
		rng := rand.New(rand.NewSource(seed))
		inPlace := NewLedger()
		states := []*Ledger{NewLedger()}
		var accepted []Entry
		var receipts []Receipt
		for step := 0; step < 80; step++ {
			e := randomEntryWithHolds(rng, len(accepted))
			context := fmt.Sprintf("seed %d step %d entry %+v", seed, step, e)
			state := states[len(states)-1]
			before := state.Projection()
			next, rs, err := state.With(e)
			direct, directErr := inPlace.apply(e)
			if (err == nil) != (directErr == nil) || (err == nil && rs[0] != direct[0]) {
				t.Fatalf("%s: With = %+v, %v; apply = %+v, %v", context, rs, err, direct, directErr)
			}
			if err == nil && !rs[0].Replayed {
				checkLedger(t, inPlace, context)
				after := next.Projection()
				if e.Kind == KindHold {
					settled := acceptedFor(accepted, receipts, KindSettle, e.ReservationID)
					want, wantList := wantHold(e, settled, before.Unknown)
					if rs[0] != want || !reflect.DeepEqual(after.Unknown, wantList) {
						t.Fatalf("%s: receipt %+v and unknown %+v;\nthe unknown set before it gives %+v and %+v", context, rs[0], after.Unknown, want, wantList)
					}
					facts := factsOf(before.Unknown, e.ReservationID)
					unsized := false
					for _, opened := range accepted {
						unsized = unsized || (opened.Kind == KindReserve && opened.ReservationID == e.ReservationID && opened.Unsized)
					}
					switch {
					case settled:
						seen.settled++
						recorded := false
						for i, old := range accepted {
							recorded = recorded || (old.Kind == KindHold && old.ReservationID == e.ReservationID &&
								old.HoldReason == e.HoldReason && receipts[i].Disposition == DispositionAccepted)
						}
						if !recorded {
							seen.unrecorded++
						}
					case want.Disposition == DispositionStale:
						seen.repeated++
						if facts[len(facts)-1] != UnknownReason(e.HoldReason) {
							seen.older++
						}
						if unsized {
							seen.unsizedRepeated++
						}
					default:
						if len(facts) == 0 {
							seen.first++
						} else {
							seen.further++
						}
						for _, item := range before.Unknown {
							if item.Reason == UnknownReason(e.HoldReason) {
								seen.neighbour++
								break
							}
						}
						if unsized {
							seen.unsized++
						}
					}
				}
				if itemsGone(before.Unknown, after.Unknown) > 0 {
					toStay := false
					for _, item := range before.Unknown {
						toStay = toStay || (isLifecycleReason(item.Reason) && (e.Kind != KindSettle || item.ReservationID != e.ReservationID))
					}
					switch {
					case !toStay:
					case e.Kind == KindSettle:
						seen.settledBeside++
					case e.Usage.Supersedes != "":
						seen.correctedBeside++
					default:
						seen.totalBeside++
					}
				}
				accepted, receipts, states = append(accepted, e), append(receipts, rs[0]), append(states, next)
				for n := 0; n < 4; n++ {
					reservation := fmt.Sprintf("r%d", n)
					if got, want := factsOf(after.Unknown, reservation), heldReasons(accepted, receipts, reservation); !reflect.DeepEqual(got, want) {
						t.Fatalf("%s: the unknown set has %v for %s, its accepted holds recorded %v", context, got, reservation, want)
					}
				}
			}

			// An earlier state answers from what it holds itself, not from
			// what was derived from it since.
			k := rng.Intn(len(states))
			probe := holdEntry("probe", fmt.Sprintf("r%d", rng.Intn(4)), allHoldReasons[rng.Intn(len(allHoldReasons))])
			earlier := states[k].Projection()
			_, rs, err = states[k].With(probe)
			if !acceptedFor(accepted[:k], receipts[:k], KindReserve, probe.ReservationID) {
				if !errors.Is(err, ErrUnknownReservation) {
					t.Fatalf("%s: probe %+v on state %d = %+v, %v; want ErrUnknownReservation", context, probe, k, rs, err)
				}
			} else {
				want, _ := wantHold(probe, acceptedFor(accepted[:k], receipts[:k], KindSettle, probe.ReservationID), earlier.Unknown)
				if err != nil || rs[0] != want {
					t.Fatalf("%s: probe %+v on state %d = %+v, %v; its unknown set gives %+v", context, probe, k, rs, err, want)
				}
				newest := states[len(states)-1].Projection().Unknown
				now, _ := wantHold(probe, acceptedFor(accepted, receipts, KindSettle, probe.ReservationID), newest)
				if want.Disposition == DispositionAccepted && now.Disposition == DispositionStale {
					seen.earlier++
				}
			}
			if !reflect.DeepEqual(states[k].Projection(), earlier) {
				t.Fatalf("%s: probe %+v changed state %d", context, probe, k)
			}
		}

		// Every state of the chain is still what its entries make it: the rest
		// of the history gets the same receipts from it and ends in the same
		// ledger.
		for cut := range states {
			if cut != 0 && cut%9 != int(seed%9) {
				continue
			}
			whole, rs, err := states[cut].With(accepted[cut:]...)
			if err != nil || len(rs) != len(accepted)-cut || !reflect.DeepEqual(whole, inPlace) {
				t.Fatalf("seed %d: state %d and then the rest differs from the ledger applied in place (%v)", seed, cut, err)
			}
			for i, r := range rs {
				if r != receipts[cut+i] {
					t.Fatalf("seed %d: entry %d applied to state %d = %+v, want %+v", seed, cut+i, cut, r, receipts[cut+i])
				}
			}
		}
	}
	// The generator is seeded, so these counts are the same on every run: 861
	// first facts and 1384 further ones, 1163 of the two on an unsized
	// reservation and 869 with the reason on record for a neighbour; 1075
	// repeated reasons, 601 of them not the latest on record and 571 on an
	// unsized reservation; 1176 holds after a settlement, 992 of them with a
	// reason never recorded; and 2049 earlier states that still take the fact.
	// Items left the unknown set beside a fact that had to stay 188 times by
	// a settlement, 10 times by a reported running total and never by a
	// correction.
	// The floors fail the test if a change to the generator, or to the rules,
	// stops one of the cases from being reached.
	t.Logf("reached: %+v", seen)
	if seen.first < 500 || seen.further < 800 || seen.unsized < 700 || seen.neighbour < 500 ||
		seen.repeated < 600 || seen.older < 350 || seen.unsizedRepeated < 340 ||
		seen.settled < 700 || seen.unrecorded < 600 || seen.earlier < 1200 ||
		seen.settledBeside < 110 || seen.totalBeside < 5 {
		t.Fatalf("the generator no longer reaches every case: %+v", seen)
	}
}
