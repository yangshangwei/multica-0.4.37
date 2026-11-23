package budget

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"reflect"
	"strings"
	"testing"
)

// unsizedReserve opens a reservation with no amount: the root policy sets no
// per-attempt reservation, so nothing bounds what the attempt may spend.
func unsizedReserve(id, reservation string) Entry {
	return Entry{ID: id, Kind: KindReserve, ReservationID: reservation, Unsized: true}
}

// An unsized reservation occupies nothing, so it shows in no figure. While it
// is unsettled the total has no upper bound: it is one item of the unknown set
// and there is no remaining amount.
func TestUnsizedReservationIsUnknownUntilItSettles(t *testing.T) {
	l := NewLedger()
	receipt := apply(t, l, unsizedReserve("reserve-1", "r1"))[0]
	if want := (Receipt{EntryID: "reserve-1", Kind: KindReserve, Disposition: DispositionAccepted, Reason: "reservation_unsized"}); receipt != want {
		t.Fatalf("reserve receipt = %+v, want %+v", receipt, want)
	}
	p := l.Projection()
	wantFigures(t, p, 0, 0, 0, 0, 0)
	wantUnknown(t, p, "reservation_unsized")
	if p.Unknown[0] != (UnknownItem{Reason: ReasonReservationUnsized, EntryID: "reserve-1", ReservationID: "r1"}) {
		t.Fatalf("unknown item = %+v", p.Unknown[0])
	}
	wantRemaining(t, p, 250, nil)
	if p.Committed() != 0 || p.Coverage.IncludedAttempts != 1 {
		t.Fatalf("committed = %d, coverage = %+v", p.Committed(), p.Coverage)
	}
	if counts := p.UnknownCounts(); len(counts) != 1 || counts[ReasonReservationUnsized] != 1 {
		t.Fatalf("unknown counts = %v", counts)
	}

	// Spend is known actual. It does not supply the missing bound.
	if r := apply(t, l, cumulative("e1", "r1", "scope-1", 1, reported(40)))[0]; r.ActualAdded != 40 || r.Reason != "" {
		t.Fatalf("usage receipt = %+v", r)
	}
	p = l.Projection()
	wantFigures(t, p, 40, 0, 0, 0, 0)
	wantUnknown(t, p, ReasonReservationUnsized)
	wantRemaining(t, p, 250, nil)

	settled := apply(t, l, settleEntry("settle-1", "r1"))[0]
	if settled.Disposition != DispositionAccepted || settled.Released != 0 || settled.Reason != "" {
		t.Fatalf("settle receipt = %+v", settled)
	}
	p = l.Projection()
	wantFigures(t, p, 40, 0, 0, 0, 0)
	wantUnknown(t, p)
	wantRemaining(t, p, 250, ticks(210))
}

func TestSettleOfAnUnsizedReservationNeedsEveryProof(t *testing.T) {
	for name, proof := range map[string]ReleaseProof{
		"none":              {},
		"no final usage":    {ScopeComplete: true, NoFurtherExecution: true},
		"scope incomplete":  {FinalUsage: true, NoFurtherExecution: true},
		"may still execute": {FinalUsage: true, ScopeComplete: true},
	} {
		l := NewLedger()
		apply(t, l, unsizedReserve("reserve-1", "r1"))
		if _, err := l.apply(Entry{ID: "settle-1", Kind: KindSettle, ReservationID: "r1", Proof: proof}); !errors.Is(err, ErrReleaseProofIncomplete) {
			t.Errorf("%s: error = %v, want ErrReleaseProofIncomplete", name, err)
		}
		p := l.Projection()
		wantUnknown(t, p, ReasonReservationUnsized)
		wantRemaining(t, p, 250, nil)
	}
}

// A lifecycle fact adds its own reason. It proves nothing about the missing
// bound, so the unsized item stays until a settlement.
func TestLifecycleHoldKeepsTheUnsizedItem(t *testing.T) {
	for _, reason := range []HoldReason{HoldCancelled, HoldTimeout, HoldProcessExit, HoldLostContact, HoldSupersededUnproven} {
		l := NewLedger()
		apply(t, l, unsizedReserve("reserve-1", "r1"), cumulative("e1", "r1", "scope-1", 1, reported(30)))
		held := apply(t, l, holdEntry("fact-1", "r1", reason))[0]
		if held.Disposition != DispositionAccepted || held.Reason != UnknownReason(reason) || held.Released != 0 {
			t.Fatalf("%s: hold receipt = %+v", reason, held)
		}
		p := l.Projection()
		// Nothing was reserved, so the hold has no unspent part to carry.
		wantFigures(t, p, 30, 0, 0, 0, 0)
		wantUnknown(t, p, ReasonReservationUnsized, UnknownReason(reason))
		if p.Unknown[0].ReservationID != "r1" || p.Unknown[0].EntryID != "reserve-1" {
			t.Fatalf("%s: unsized item = %+v", reason, p.Unknown[0])
		}
		wantRemaining(t, p, 250, nil)

		if r := apply(t, l, settleEntry("settle-1", "r1"))[0]; r.Disposition != DispositionAccepted || r.Released != 0 {
			t.Fatalf("%s: settle receipt = %+v", reason, r)
		}
		p = l.Projection()
		wantFigures(t, p, 30, 0, 0, 0, 0)
		wantUnknown(t, p)
		wantRemaining(t, p, 250, ticks(220))
	}

	// Several facts, a repeated one, and usage that arrives between them.
	l := NewLedger()
	apply(t, l, unsizedReserve("reserve-1", "r1"), holdEntry("fact-1", "r1", HoldTimeout), holdEntry("fact-2", "r1", HoldProcessExit))
	if r := apply(t, l, holdEntry("fact-3", "r1", HoldTimeout))[0]; r.Disposition != DispositionStale {
		t.Fatalf("repeated reason receipt = %+v", r)
	}
	apply(t, l, delta("d1", "r1", "scope-1", reported(75)))
	p := l.Projection()
	wantFigures(t, p, 75, 0, 0, 0, 0)
	wantUnknown(t, p, ReasonReservationUnsized, "timeout", "process_exit")

	// A lifecycle fact after the settlement is stale and brings nothing back.
	apply(t, l, settleEntry("settle-1", "r1"))
	if r := apply(t, l, holdEntry("fact-4", "r1", HoldCancelled))[0]; r.Disposition != DispositionStale || r.Reason != "" {
		t.Fatalf("hold after settlement = %+v", r)
	}
	p = l.Projection()
	wantFigures(t, p, 75, 0, 0, 0, 0)
	wantUnknown(t, p)
}

// Unsized is a field of a reserve entry that carries no amount. With an
// amount, or on any other kind, the entry is refused like any other entry
// whose fields do not fit its kind.
func TestUnsizedIsValidOnlyOnAReserveWithNoAmount(t *testing.T) {
	l := NewLedger()
	apply(t, l, reserveEntry("reserve-1", "r1", 10))
	before := l.clone()

	for _, amount := range []Ticks{1, 5, 9_223_372_036_854_775_807, -1} {
		e := unsizedReserve("reserve-2", "r2")
		e.ReservedTicks = amount
		if rs, err := l.apply(e); !errors.Is(err, ErrInvalidEntry) || rs != nil {
			t.Errorf("unsized reserve of %d: apply = %+v, %v; want ErrInvalidEntry", amount, rs, err)
		}
	}

	for kind, e := range map[EntryKind]Entry{
		KindUsage:     cumulative("e1", "r1", "scope-1", 1, reported(5)),
		KindHold:      holdEntry("e1", "r1", HoldTimeout),
		KindSettle:    settleEntry("e1", "r1"),
		KindGap:       {ID: "e1", Kind: KindGap, ReservationID: "r1", GapReason: ReasonCoverageGap},
		KindExclusion: {ID: "e1", Kind: KindExclusion, Detail: "autopilot_quota"},
	} {
		// The fixture is valid as it stands, so the flag is the only reason
		// left to refuse it.
		if _, err := l.clone().apply(e); err != nil {
			t.Fatalf("%s fixture is not valid: %v", kind, err)
		}
		e.Unsized = true
		if rs, err := l.apply(e); !errors.Is(err, ErrInvalidEntry) || rs != nil {
			t.Errorf("%s entry marked unsized: apply = %+v, %v; want ErrInvalidEntry", kind, rs, err)
		}
	}
	if !reflect.DeepEqual(l, before) {
		t.Fatal("a rejected entry changed the ledger")
	}

	// The refused identities are free, and the flag with no amount is accepted.
	if r := apply(t, l, unsizedReserve("reserve-2", "r2"))[0]; r.Replayed || r.Reason != ReasonReservationUnsized {
		t.Fatalf("receipt after the rejections = %+v", r)
	}
	p := l.Projection()
	wantFigures(t, p, 0, 0, 10, 0, 0)
	wantUnknown(t, p, ReasonReservationUnsized)
}

// Design section 10.1: actual spend on an unsized reservation is never an
// overrun, because there was no amount to exceed. A sized reservation of zero
// is a different fact: its amount is known to be zero, so every tick spent on
// it is overrun and the total stays known.
func TestSpendOnAnUnsizedReservationIsNeverOverrun(t *testing.T) {
	l := NewLedger()
	apply(t, l, unsizedReserve("reserve-1", "r1"), cumulative("e1", "r1", "scope-1", 1, reported(500)))
	wantFigures(t, l.Projection(), 500, 0, 0, 0, 0)
	apply(t, l, holdEntry("lost-1", "r1", HoldLostContact), cumulative("e2", "r1", "scope-1", 2, reported(800)))
	wantFigures(t, l.Projection(), 800, 0, 0, 0, 0)
	apply(t, l, settleEntry("settle-1", "r1"))
	wantFigures(t, l.Projection(), 800, 0, 0, 0, 0)
	// Usage that arrives after the settlement still counts, and still not as
	// overrun.
	apply(t, l, cumulative("e3", "r1", "scope-1", 3, reported(900)))
	p := l.Projection()
	wantFigures(t, p, 900, 0, 0, 0, 0)
	wantUnknown(t, p)
	wantRemaining(t, p, 1000, ticks(100))
	if p.Committed() != 900 {
		t.Fatalf("committed = %d, want 900", p.Committed())
	}

	zero := NewLedger()
	if r := apply(t, zero, reserveEntry("reserve-1", "r1", 0))[0]; r.Reason != "" {
		t.Fatalf("sized zero reserve receipt = %+v", r)
	}
	apply(t, zero, cumulative("e1", "r1", "scope-1", 1, reported(500)))
	p = zero.Projection()
	wantFigures(t, p, 500, 0, 0, 0, 500)
	wantUnknown(t, p)
	wantRemaining(t, p, 1000, ticks(500))
}

// r1 is sized at 100 and r2 is unsized. Each keeps its own figures and its own
// unknown items, whatever happens to the other.
func TestSizedAndUnsizedReservationsDoNotAffectEachOther(t *testing.T) {
	l := NewLedger()
	apply(t, l, reserveEntry("reserve-1", "r1", 100), unsizedReserve("reserve-2", "r2"))
	p := l.Projection()
	wantFigures(t, p, 0, 0, 100, 0, 0)
	wantUnknown(t, p, ReasonReservationUnsized)
	if p.Unknown[0].ReservationID != "r2" || p.Coverage.IncludedAttempts != 2 {
		t.Fatalf("unknown item = %+v, coverage = %+v", p.Unknown[0], p.Coverage)
	}

	// 30 on r1 draws its reservation down; 500 on r2 draws nothing down.
	apply(t, l, delta("d1", "r1", "scope-1", reported(30)), delta("d2", "r2", "scope-2", reported(500)))
	p = l.Projection()
	wantFigures(t, p, 530, 0, 70, 0, 0)
	if p.Committed() != 600 {
		t.Fatalf("committed = %d, want 600", p.Committed())
	}

	// Losing contact with r1 holds its 70 and leaves r2 as it was.
	apply(t, l, holdEntry("lost-1", "r1", HoldLostContact))
	p = l.Projection()
	wantFigures(t, p, 530, 0, 0, 70, 0)
	wantUnknown(t, p, ReasonReservationUnsized, "lost_contact")

	// Settling r2 removes r2's item only and releases nothing of r1.
	if r := apply(t, l, settleEntry("settle-2", "r2"))[0]; r.Released != 0 {
		t.Fatalf("settle of r2 = %+v", r)
	}
	p = l.Projection()
	wantFigures(t, p, 530, 0, 0, 70, 0)
	wantUnknown(t, p, "lost_contact")
	if p.Unknown[0].ReservationID != "r1" {
		t.Fatalf("remaining unknown item = %+v", p.Unknown[0])
	}
	wantRemaining(t, p, 1000, nil)

	if r := apply(t, l, settleEntry("settle-1", "r1"))[0]; r.Released != 70 {
		t.Fatalf("settle of r1 = %+v", r)
	}
	p = l.Projection()
	wantFigures(t, p, 530, 0, 0, 0, 0)
	wantUnknown(t, p)
	wantRemaining(t, p, 1000, ticks(470))

	// The other order: r1 overran by 20 and settled first. Only r1's excess is
	// overrun, and r2's item is still there.
	other := NewLedger()
	apply(t, other,
		reserveEntry("reserve-1", "r1", 100), unsizedReserve("reserve-2", "r2"),
		delta("d1", "r1", "scope-1", reported(120)), delta("d2", "r2", "scope-2", reported(500)),
		settleEntry("settle-1", "r1"),
	)
	p = other.Projection()
	wantFigures(t, p, 620, 0, 0, 0, 20)
	wantUnknown(t, p, ReasonReservationUnsized)
	wantRemaining(t, p, 1000, nil)
}

// The settle rule ignores the unsized item itself. Usage of the same
// reservation whose cost is unknown still stands in the way.
func TestSettleOfAnUnsizedReservationStillRefusesUnknownUsage(t *testing.T) {
	l := NewLedger()
	apply(t, l, unsizedReserve("reserve-1", "r1"), delta("d1", "r1", "scope-1", Cost{}))
	if _, err := l.apply(settleEntry("settle-1", "r1")); !errors.Is(err, ErrReleaseProofIncomplete) {
		t.Fatalf("settle error = %v, want ErrReleaseProofIncomplete", err)
	}
	wantUnknown(t, l.Projection(), ReasonReservationUnsized, ReasonCostAbsent)

	// The correction resolves d1 and nothing else.
	fix := delta("d2", "r1", "scope-1", reported(12))
	fix.Usage.Supersedes, fix.Usage.CorrectionAuthorized = "d1", true
	apply(t, l, fix)
	wantUnknown(t, l.Projection(), ReasonReservationUnsized)

	apply(t, l, settleEntry("settle-1", "r1"))
	p := l.Projection()
	wantFigures(t, p, 12, 0, 0, 0, 0)
	wantUnknown(t, p)
}

// r1 unsized: 30, contact lost, 80 in the end, settled. r2 sized at 100 and
// spent 120. r3 unsized and still open. A = 80 + 120, overrun = 20, and the
// only unknown left is r3.
func TestUnsizedReservationsReplayToTheSameProjection(t *testing.T) {
	entries := []Entry{
		unsizedReserve("reserve-1", "r1"),
		cumulative("e1", "r1", "scope-1", 1, reported(30)),
		reserveEntry("reserve-2", "r2", 100),
		holdEntry("lost-1", "r1", HoldLostContact),
		delta("d1", "r2", "scope-2", reported(120)),
		unsizedReserve("reserve-3", "r3"),
		cumulative("e2", "r1", "scope-1", 2, reported(80)),
		settleEntry("settle-1", "r1"),
	}
	want, receipts, err := Reduce(entries)
	if err != nil || len(receipts) != len(entries) {
		t.Fatalf("Reduce = %v, %d receipts", err, len(receipts))
	}
	wantFigures(t, want, 200, 0, 0, 0, 20)
	wantUnknown(t, want, ReasonReservationUnsized)
	raw, err := json.Marshal(want)
	const wantJSON = `{"actual_known_ticks":"200","estimated_ticks":"0","reserved_ticks":"0","unknown_hold_ticks":"0","overrun_ticks":"20","unknown":[{"reason":"reservation_unsized","entry_id":"reserve-3","reservation_id":"r3","scope_id":"","detail":""}],"coverage":{"included_attempts":3,"not_applicable":[]}}`
	if err != nil || string(raw) != wantJSON {
		t.Fatalf("projection JSON = %s, %v", raw, err)
	}

	again, _, err := Reduce(entries)
	if err != nil || !reflect.DeepEqual(again, want) {
		t.Fatalf("second Reduce = %+v, %v", again, err)
	}
	step, batch := NewLedger(), NewLedger()
	for _, e := range entries {
		apply(t, step, e)
	}
	apply(t, batch, entries...)
	if !reflect.DeepEqual(step, batch) || !reflect.DeepEqual(step.Projection(), want) {
		t.Fatalf("entry-by-entry = %+v, want %+v", step.Projection(), want)
	}

	// Appending every entry again replays every receipt. The settled r1 does
	// not get its item back and the open r3 does not get a second one.
	doubled, replayed, err := Reduce(append(append([]Entry{}, entries...), entries...))
	if err != nil || !reflect.DeepEqual(doubled, want) {
		t.Fatalf("doubled = %+v, %v", doubled, err)
	}
	for i, r := range replayed[len(entries):] {
		original := receipts[i]
		original.Replayed = true
		if r != original {
			t.Fatalf("replay of entry %d = %+v, want %+v", i, r, original)
		}
	}
	state := step.clone()
	for _, e := range []Entry{entries[0], entries[5]} {
		if r := apply(t, step, e)[0]; !r.Replayed || r.Reason != ReasonReservationUnsized {
			t.Fatalf("replay of %s = %+v", e.ID, r)
		}
	}
	if !reflect.DeepEqual(step, state) {
		t.Fatal("replaying an unsized reserve changed the ledger")
	}
}

// The flag is content: the same ID once unsized and once sized at zero is two
// different entries, with two stored forms and two digests.
func TestUnsizedFlagIsPartOfTheEntryIdentity(t *testing.T) {
	unsized, sized := unsizedReserve("reserve-1", "r1"), reserveEntry("reserve-1", "r1", 0)

	l := NewLedger()
	apply(t, l, unsized)
	if _, err := l.apply(sized); !errors.Is(err, ErrConflict) {
		t.Fatalf("sized after unsized: error = %v, want ErrConflict", err)
	}
	l = NewLedger()
	apply(t, l, sized)
	if _, err := l.apply(unsized); !errors.Is(err, ErrConflict) {
		t.Fatalf("unsized after sized: error = %v, want ErrConflict", err)
	}
	wantUnknown(t, l.Projection())
	// A reservation is opened once, sized or not.
	if _, err := l.apply(unsizedReserve("reserve-2", "r1")); !errors.Is(err, ErrConflict) {
		t.Fatalf("second reserve of r1: error = %v, want ErrConflict", err)
	}

	raw, encodedDigest, err := EncodeEntry(unsized)
	const want = `{"id":"reserve-1","kind":"reserve","reservation_id":"r1","reserved_ticks":"0","unsized":true,"hold_reason":"","proof":{"final_usage":false,"scope_complete":false,"no_further_execution":false},"gap_reason":"","detail":"","usage":{"scope_id":"","invocation_id":"","mode":"","sequence":"0","overlap":"","baseline_event_id":"","supersedes":"","correction_authorized":false,"tokens":{"input":"0","output":"0","cache_read":"0","cache_write":"0"},"cost":{"basis":"","ticks":"0","currency":"","zero_basis":"","price_version":"","price_digest":""}}}`
	if err != nil || string(raw) != want {
		t.Fatalf("stored form = %s, %v", raw, err)
	}
	back, err := DecodeEntry(EntryFormVersion, raw)
	if err != nil || back != unsized {
		t.Fatalf("round trip = %+v, %v", back, err)
	}
	// SHA-256 of "budget-entry-v1\n" + the stored form, computed outside Go.
	// The sized twin differs in the one byte sequence "unsized":false.
	const (
		wantDigest      = "a9fd918db913b78298cf666b0d4c14bee8eb40a53741e63eb2a30ec9b6c1dd7b"
		wantSizedDigest = "4ea6c1daffd840c9a0b2ae92534e52aa6b03a103b363adade4d2d8bb728d4c2c"
	)
	if d, err := unsized.Digest(); err != nil || d != wantDigest || encodedDigest != wantDigest {
		t.Fatalf("digest = %q and %q, %v; want %q", d, encodedDigest, err, wantDigest)
	}
	sizedRaw, sizedDigest, err := EncodeEntry(sized)
	if err != nil || string(sizedRaw) != strings.Replace(want, `"unsized":true`, `"unsized":false`, 1) {
		t.Fatalf("sized stored form = %s, %v", sizedRaw, err)
	}
	if d, err := sized.Digest(); err != nil || d != wantSizedDigest || sizedDigest != wantSizedDigest {
		t.Fatalf("sized digest = %q and %q, %v; want %q", d, sizedDigest, err, wantSizedDigest)
	}
}

// randomEntryWithUnsized is randomEntry with about half of the reservations
// unsized, and now and then the flag where it is not valid: on a reserve that
// keeps its amount, or on another kind. Half of the entries get an ID no
// accepted entry has, so fewer draws end as an identity conflict, and more of
// them are settlements, so the histories grow long enough to hold, spend on
// and settle what they reserve.
func randomEntryWithUnsized(rng *rand.Rand, n int) Entry {
	e := randomEntry(rng, n)
	if rng.Intn(2) == 0 {
		e.ID = fmt.Sprintf("u%d", n)
	}
	switch roll := rng.Intn(20); {
	case e.Kind == KindReserve && roll < 10:
		e.ReservedTicks, e.Unsized = 0, true
	case e.Kind != KindReserve && roll < 3:
		e = settleEntry(e.ID, fmt.Sprintf("r%d", rng.Intn(4)))
	case roll == 19:
		e.Unsized = true
	}
	return e
}

// Random interleavings with unsized reservations among the sized ones. Each
// accepted entry keeps the invariants checkLedger asserts, a misplaced flag is
// always refused without a trace, and the history replays to the same state.
func TestRandomInterleavingsWithUnsizedReservationsKeepTheInvariants(t *testing.T) {
	// How often the generator reached each case this test is about: an unsized
	// reservation opened, spent on, held and settled, and a misplaced flag.
	var seen struct{ unsized, spent, held, settled, misplaced int }
	for seed := int64(1); seed <= 300; seed++ {
		rng := rand.New(rand.NewSource(seed))
		l := NewLedger()
		var accepted []Entry
		var receipts []Receipt
		for step := 0; step < 60; step++ {
			e := randomEntryWithUnsized(rng, len(accepted))
			context := fmt.Sprintf("seed %d step %d entry %+v", seed, step, e)
			misplaced := e.Unsized && (e.Kind != KindReserve || e.ReservedTicks != 0)
			before, beforeP := l.clone(), l.Projection()
			got, err := l.apply(e)
			if misplaced {
				seen.misplaced++
				if !errors.Is(err, ErrInvalidEntry) {
					t.Fatalf("%s: error = %v, want ErrInvalidEntry", context, err)
				}
			}
			if err != nil {
				if !reflect.DeepEqual(l, before) {
					t.Fatalf("%s: rejected with %v but changed the ledger", context, err)
				}
				continue
			}
			p := checkLedger(t, l, context)
			if got[0].Replayed {
				if !reflect.DeepEqual(l, before) {
					t.Fatalf("%s: a replay changed the ledger", context)
				}
				continue
			}
			accepted, receipts = append(accepted, e), append(receipts, got[0])
			if e.Kind == KindReserve {
				grew := len(p.Unknown) - len(beforeP.Unknown)
				if p.Reserved-beforeP.Reserved != e.ReservedTicks || p.Committed()-beforeP.Committed() != e.ReservedTicks || p.Overrun != beforeP.Overrun {
					t.Fatalf("%s: reserve moved %+v -> %+v", context, beforeP, p)
				}
				if e.Unsized {
					seen.unsized++
					item := UnknownItem{Reason: ReasonReservationUnsized, EntryID: e.ID, ReservationID: e.ReservationID}
					if grew != 1 || p.Unknown[len(p.Unknown)-1] != item || got[0].Reason != ReasonReservationUnsized {
						t.Fatalf("%s: unsized reserve gave %+v and unknown %+v", context, got[0], p.Unknown)
					}
				} else if grew != 0 || got[0].Reason != "" {
					t.Fatalf("%s: sized reserve gave %+v and unknown %+v", context, got[0], p.Unknown)
				}
			}
			if r, ok := l.reservations[e.ReservationID]; ok && r.unsized && got[0].Disposition == DispositionAccepted {
				switch {
				case e.Kind == KindUsage && got[0].ActualAdded > 0:
					seen.spent++
				case e.Kind == KindHold:
					seen.held++
				case e.Kind == KindSettle:
					seen.settled++
				}
			}
		}
		final := l.Projection()
		p, rs, err := Reduce(accepted)
		if err != nil || !reflect.DeepEqual(p, final) || !reflect.DeepEqual(rs, receipts) {
			t.Fatalf("seed %d: Reduce = %+v, %v; want %+v", seed, p, err, final)
		}
		fresh := NewLedger()
		if _, err := fresh.apply(accepted...); err != nil || !reflect.DeepEqual(fresh, l) {
			t.Fatalf("seed %d: one batch and entry-by-entry reach different states (%v)", seed, err)
		}
		cut := rng.Intn(len(accepted) + 1)
		repeated := append(append(append([]Entry{}, accepted[:cut]...), accepted[:cut]...), accepted[cut:]...)
		if p, _, err = Reduce(repeated); err != nil || !reflect.DeepEqual(p, final) {
			t.Fatalf("seed %d: prefix %d replayed = %+v, %v; want %+v", seed, cut, p, err, final)
		}
	}
	// The generator is seeded, so these counts are the same on every run: 520
	// unsized reservations, 244 spends on one, 128 holds, 201 settlements and
	// 959 misplaced flags. The floors fail the test if a change to the
	// generator, or to the rules, stops one of the cases from being reached.
	t.Logf("reached: %+v", seen)
	if seen.unsized < 300 || seen.spent < 150 || seen.held < 80 || seen.settled < 120 || seen.misplaced < 500 {
		t.Fatalf("the generator no longer reaches every case: %+v", seen)
	}
}
