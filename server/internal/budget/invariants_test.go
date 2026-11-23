package budget

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"reflect"
	"testing"
)

// The tests in this file use seeded generators, so every run sees the same
// entries. They check contract section 4.2 over interleavings the row tests
// do not spell out.

// randomEntry draws from small ID spaces so that replays, conflicts, unknown
// references, corrections and invalid shapes all occur.
func randomEntry(rng *rand.Rand, n int) Entry {
	id := fmt.Sprintf("e%d", rng.Intn(n+4))
	res := fmt.Sprintf("r%d", rng.Intn(4))
	if rng.Intn(5) == 0 {
		res = ""
	}
	costs := []Cost{
		{},
		reported(Ticks(rng.Intn(200))),
		reported(Ticks(rng.Intn(200))),
		{Basis: CostReported, Currency: "USD", ZeroBasis: "free"},
		{Basis: CostEstimated, Ticks: Ticks(rng.Intn(200)), Currency: "USD"},
		{Basis: CostReported, Ticks: Ticks(rng.Intn(200)), Currency: "EUR"},
	}
	switch rng.Intn(12) {
	case 0, 1:
		return reserveEntry(id, res, Ticks(rng.Intn(150)))
	case 2:
		reasons := []HoldReason{HoldCancelled, HoldTimeout, HoldProcessExit, HoldLostContact, HoldSupersededUnproven}
		return holdEntry(id, res, reasons[rng.Intn(len(reasons))])
	case 3:
		e := settleEntry(id, res)
		if rng.Intn(6) == 0 {
			e.Proof.ScopeComplete = false
		}
		return e
	case 4:
		return Entry{ID: id, Kind: KindGap, ReservationID: res, GapReason: ReasonCoverageGap}
	case 5:
		return Entry{ID: id, Kind: KindExclusion, Detail: fmt.Sprintf("m%d", rng.Intn(3))}
	case 6, 7, 8:
		e := cumulative(id, res, fmt.Sprintf("c%d", rng.Intn(3)), Count(1+rng.Intn(12)), costs[rng.Intn(len(costs))])
		e.Usage.InvocationID = fmt.Sprintf("i%d", rng.Intn(2))
		e.Usage.Tokens = TokenCounts{Input: Count(rng.Intn(3) * 100)}
		switch rng.Intn(6) {
		case 0:
			e.Usage.BaselineEventID = fmt.Sprintf("e%d", rng.Intn(n+4))
		case 1:
			e.Usage.Supersedes, e.Usage.CorrectionAuthorized = fmt.Sprintf("e%d", rng.Intn(n+4)), rng.Intn(4) != 0
		case 2:
			e.Usage.Overlap = []Overlap{OverlapRollup, OverlapUnproven}[rng.Intn(2)]
		}
		return e
	default:
		e := delta(id, res, fmt.Sprintf("d%d", rng.Intn(3)), costs[rng.Intn(len(costs))])
		if rng.Intn(4) == 0 {
			e.Usage.Supersedes, e.Usage.CorrectionAuthorized = fmt.Sprintf("e%d", rng.Intn(n+4)), rng.Intn(4) != 0
		}
		return e
	}
}

// checkLedger asserts the structural invariants of contract section 4.2 on
// the internal state and the projection derived from it.
func checkLedger(t *testing.T, l *Ledger, context string) Projection {
	t.Helper()
	p := l.Projection()
	if p.ActualKnown < 0 || p.Estimated < 0 || p.Reserved < 0 || p.UnknownHold < 0 || p.Overrun < 0 {
		t.Fatalf("%s: negative figure in %+v", context, p)
	}
	if got := int64(p.ActualKnown) + int64(p.Reserved) + int64(p.UnknownHold); got != l.committed || p.Committed() != Ticks(l.committed) {
		t.Fatalf("%s: A+R+H = %d, internal committed = %d", context, got, l.committed)
	}
	var open, held, overrun int64
	actual := l.unreserved
	if l.unreserved < 0 {
		t.Fatalf("%s: unreserved actual is %d", context, l.unreserved)
	}
	for id, r := range l.reservations {
		if r.actual < 0 || r.reserved < 0 {
			t.Fatalf("%s: reservation %s = %+v", context, id, r)
		}
		actual += r.actual
		// An unsized reservation had no amount to exceed. It is in the unknown
		// set exactly once from its reserve entry until it settles; a sized
		// reservation never is.
		var unsizedItems int
		for _, item := range l.unknown {
			if item.ReservationID == id && item.Reason == ReasonReservationUnsized {
				unsizedItems++
			}
		}
		wantItems := 0
		if r.unsized && r.state != reservationSettled {
			wantItems = 1
		}
		if unsizedItems != wantItems || (r.unsized && r.reserved != 0) {
			t.Fatalf("%s: reservation %s = %+v carries %d unsized items, want %d", context, id, r, unsizedItems, wantItems)
		}
		if !r.unsized && r.actual > r.reserved {
			overrun += r.actual - r.reserved
		}
		rest := r.reserved - r.actual
		if rest < 0 {
			rest = 0
		}
		switch r.state {
		case reservationOpen:
			open += rest
		case reservationHeld:
			held += rest
			// A held reservation always has a lifecycle reason in the unknown set.
			found := false
			for _, item := range l.unknown {
				found = found || (item.ReservationID == id && isLifecycleReason(item.Reason))
			}
			if !found {
				t.Fatalf("%s: held reservation %s has no lifecycle unknown item", context, id)
			}
		case reservationSettled:
			for _, item := range l.unknown {
				if item.ReservationID == id && isLifecycleReason(item.Reason) {
					t.Fatalf("%s: settled reservation %s still carries %s", context, id, item.Reason)
				}
			}
		}
	}
	if int64(p.ActualKnown) != actual || int64(p.Reserved) != open || int64(p.UnknownHold) != held || int64(p.Overrun) != overrun {
		t.Fatalf("%s: projection %+v, recomputed A=%d R=%d H=%d overrun=%d", context, p, actual, open, held, overrun)
	}
	checkReadings(t, l, p, context)
	var scopeActual, scopeEstimated int64
	for id, sc := range l.scopes {
		var by int64
		for res, v := range sc.actualBy {
			if v < 0 {
				t.Fatalf("%s: scope %s attributes %d to %q", context, id, v, res)
			}
			by += v
		}
		if by != sc.actual {
			t.Fatalf("%s: scope %s actual %d, attributed %d", context, id, sc.actual, by)
		}
		scopeActual += sc.actual
		scopeEstimated += sc.estimated()
	}
	for id, c := range l.contributions {
		if c.actual < 0 || c.estimated < 0 {
			t.Fatalf("%s: contribution %s = %+v", context, id, c)
		}
		if !c.superseded {
			scopeActual += c.actual
			scopeEstimated += c.estimated
		}
	}
	if scopeActual != actual || scopeEstimated != int64(p.Estimated) {
		t.Fatalf("%s: scopes give A=%d E=%d, projection %+v", context, scopeActual, scopeEstimated, p)
	}
	if (len(p.Unknown) > 0) != (p.Remaining(1_000_000) == nil) {
		t.Fatalf("%s: remaining and the unknown set disagree: %+v", context, p)
	}
	if len(l.entries) != len(l.receipts) {
		t.Fatalf("%s: %d entries, %d receipts", context, len(l.entries), len(l.receipts))
	}
	return p
}

// checkReadings holds what Reservation reads of every reservation against
// what the ledger keeps of it, and the readings together against the
// projection: the unspent of the open reservations is R and that of the held
// ones is H, the overruns are the overrun, and the actuals with the actual
// that names no reservation are A.
func checkReadings(t *testing.T, l *Ledger, p Projection, context string) {
	t.Helper()
	statuses := map[reservationState]ReservationStatus{
		reservationOpen: ReservationOpen, reservationHeld: ReservationHeld, reservationSettled: ReservationSettled,
	}
	var open, held, overrun, actual, occupied int64
	for id, r := range l.reservations {
		got, ok := l.Reservation(id)
		// Until it settles, a reservation occupies what it has not spent. One
		// with an amount is over it by what it spent beyond.
		unspent, over := r.reserved-r.actual, r.actual-r.reserved
		if unspent < 0 || r.state == reservationSettled {
			unspent = 0
		}
		if over < 0 || r.unsized {
			over = 0
		}
		want := ReservationProjection{
			Status: statuses[r.state], Unsized: r.unsized,
			Reserved: Ticks(r.reserved), ActualKnown: Ticks(r.actual), Unspent: Ticks(unspent), Overrun: Ticks(over),
		}
		if !ok || got != want || want.Status == "" || !l.HasReservation(id) {
			t.Fatalf("%s: Reservation(%q) = %+v, %v; the ledger keeps %+v, which reads %+v", context, id, got, ok, r, want)
		}
		if got.Unsized && (got.Reserved != 0 || got.Unspent != 0 || got.Overrun != 0) {
			t.Fatalf("%s: the unsized reservation %q reads %+v", context, id, got)
		}
		switch got.Status {
		case ReservationOpen:
			open += int64(got.Unspent)
		case ReservationHeld:
			held += int64(got.Unspent)
		}
		overrun += int64(got.Overrun)
		actual += int64(got.ActualKnown)
		occupied += int64(got.ActualKnown) + int64(got.Unspent)
	}
	if open != int64(p.Reserved) || held != int64(p.UnknownHold) || overrun != int64(p.Overrun) || actual+l.unreserved != int64(p.ActualKnown) {
		t.Fatalf("%s: the readings give R=%d H=%d overrun=%d and A=%d with %d unreserved; the projection is %+v",
			context, open, held, overrun, actual, l.unreserved, p)
	}
	// A settled reservation occupies what it spent and no more, so actual and
	// unspent over every reading are A + R + H.
	if occupied+l.unreserved != l.committed {
		t.Fatalf("%s: the readings occupy %d with %d unreserved, the ledger has %d committed", context, occupied, l.unreserved, l.committed)
	}
	if got, ok := l.Reservation("no reservation has this ID"); ok || got != (ReservationProjection{}) {
		t.Fatalf("%s: a reservation that does not exist reads %+v, %v", context, got, ok)
	}
}

// Random interleavings of every entry kind, valid and invalid: each accepted
// entry keeps the invariants, each rejected entry leaves no trace, and a
// replay of any earlier entry or of the whole history is a no-op.
func TestRandomInterleavingsKeepTheInvariants(t *testing.T) {
	for seed := int64(1); seed <= 400; seed++ {
		rng := rand.New(rand.NewSource(seed))
		l := NewLedger()
		var accepted []Entry
		var receipts []Receipt
		for step := 0; step < 60; step++ {
			e := randomEntry(rng, len(accepted))
			context := fmt.Sprintf("seed %d step %d entry %+v", seed, step, e)
			before := l.clone()
			beforeP := l.Projection()
			got, err := l.apply(e)
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
			accepted = append(accepted, e)
			receipts = append(receipts, got[0])
			r := got[0]
			if int64(p.ActualKnown)-int64(beforeP.ActualKnown) != int64(r.ActualAdded)-int64(r.ActualRemoved) ||
				int64(p.Estimated)-int64(beforeP.Estimated) != int64(r.EstimatedAdded)-int64(r.EstimatedRemoved) {
				t.Fatalf("%s: receipt %+v does not explain %+v -> %+v", context, r, beforeP, p)
			}
			switch e.Kind {
			case KindHold:
				// Moving R to H frees nothing and changes no known figure.
				if p.Committed() != beforeP.Committed() || p.ActualKnown != beforeP.ActualKnown ||
					beforeP.Reserved-p.Reserved != p.UnknownHold-beforeP.UnknownHold || p.Reserved > beforeP.Reserved || r.Released != 0 {
					t.Fatalf("%s: hold moved %+v -> %+v", context, beforeP, p)
				}
			case KindSettle:
				if beforeP.Committed()-p.Committed() != r.Released || p.ActualKnown != beforeP.ActualKnown {
					t.Fatalf("%s: settle released %d, %+v -> %+v", context, r.Released, beforeP, p)
				}
			case KindReserve:
				if p.Reserved-beforeP.Reserved != e.ReservedTicks || p.ActualKnown != beforeP.ActualKnown || p.UnknownHold != beforeP.UnknownHold {
					t.Fatalf("%s: reserve moved %+v -> %+v", context, beforeP, p)
				}
			case KindGap, KindExclusion:
				if p.Committed() != beforeP.Committed() || p.Reserved != beforeP.Reserved {
					t.Fatalf("%s: %s moved %+v -> %+v", context, e.Kind, beforeP, p)
				}
			case KindUsage:
				if r.Released != 0 {
					t.Fatalf("%s: usage released allowance: %+v", context, r)
				}
			}
			// Replaying any single earlier entry returns its receipt unchanged.
			k := rng.Intn(len(accepted))
			state := l.clone()
			again, err := l.apply(accepted[k])
			want := receipts[k]
			want.Replayed = true
			if err != nil || again[0] != want || !reflect.DeepEqual(l, state) {
				t.Fatalf("%s: replay of %+v = %+v, %v; want %+v with no change", context, accepted[k], again, err, want)
			}
		}
		// The history replays to the same state, alone, twice over, and with
		// any prefix repeated in front of the rest.
		final := l.Projection()
		rawFinal, err := json.Marshal(final)
		if err != nil {
			t.Fatal(err)
		}
		p, rs, err := Reduce(accepted)
		if err != nil || !reflect.DeepEqual(p, final) || len(rs) != len(receipts) {
			t.Fatalf("seed %d: Reduce = %+v, %v; want %+v", seed, p, err, final)
		}
		for i := range rs {
			if rs[i] != receipts[i] {
				t.Fatalf("seed %d: receipt %d = %+v, want %+v", seed, i, rs[i], receipts[i])
			}
		}
		if raw, _ := json.Marshal(p); string(raw) != string(rawFinal) {
			t.Fatalf("seed %d: projection JSON differs:\n%s\n%s", seed, raw, rawFinal)
		}
		cut := rng.Intn(len(accepted) + 1)
		repeated := append(append(append([]Entry{}, accepted[:cut]...), accepted[:cut]...), accepted[cut:]...)
		p, _, err = Reduce(repeated)
		if err != nil || !reflect.DeepEqual(p, final) {
			t.Fatalf("seed %d: prefix %d replayed = %+v, %v; want %+v", seed, cut, p, err, final)
		}
		fresh := NewLedger()
		if _, err := fresh.apply(accepted...); err != nil || !reflect.DeepEqual(fresh, l) {
			t.Fatalf("seed %d: one batch and entry-by-entry reach different states (%v)", seed, err)
		}
		// A batch that ends in an invalid entry keeps nothing of its valid part.
		state := l.clone()
		extra := []Entry{reserveEntry("tail-reserve", "tail-r", 5), delta("tail-usage", "tail-r", "tail-scope", reported(3)), {ID: "tail-bad", Kind: "refund"}}
		if rs, err := l.apply(extra...); !errors.Is(err, ErrInvalidEntry) || rs != nil || !reflect.DeepEqual(l, state) {
			t.Fatalf("seed %d: failed batch = %+v, %v, or left a trace", seed, rs, err)
		}
	}
}

// naiveReservation is an independent model of contract section 4.2 for the
// delta-only grammar: it keeps the list of live amounts and derives the
// figures from scratch every time.
type naiveReservation struct {
	reserved int64
	live     map[string]int64
	state    string
}

func TestRandomDeltaLedgersMatchANaiveModel(t *testing.T) {
	for seed := int64(1); seed <= 300; seed++ {
		rng := rand.New(rand.NewSource(seed))
		l := NewLedger()
		model := map[string]*naiveReservation{}
		owner := map[string]string{}
		var liveIDs []string
		for step := 0; step < 50; step++ {
			res := fmt.Sprintf("r%d", rng.Intn(3))
			id := fmt.Sprintf("s%d-%d", seed, step)
			m := model[res]
			switch op := rng.Intn(6); {
			case m == nil:
				amount := int64(rng.Intn(120))
				apply(t, l, reserveEntry(id, res, Ticks(amount)))
				model[res] = &naiveReservation{reserved: amount, live: map[string]int64{}, state: "open"}
			case op <= 2:
				amount := int64(rng.Intn(90))
				apply(t, l, delta(id, res, "scope-"+id, reported(Ticks(amount)+1)))
				m.live[id] = amount + 1
				owner[id] = res
				liveIDs = append(liveIDs, id)
			case op == 3 && len(liveIDs) > 0:
				// An authorised correction replaces one live amount, under the
				// reservation and in the scope of its original.
				k := rng.Intn(len(liveIDs))
				old := liveIDs[k]
				amount := int64(rng.Intn(90))
				fix := delta(id, owner[old], l.entries[old].Usage.ScopeID, reported(Ticks(amount)+1))
				fix.Usage.Supersedes, fix.Usage.CorrectionAuthorized = old, true
				apply(t, l, fix)
				delete(model[owner[old]].live, old)
				model[owner[old]].live[id] = amount + 1
				owner[id] = owner[old]
				liveIDs[k] = id
			case op == 4:
				apply(t, l, holdEntry(id, res, HoldLostContact))
				if m.state == "open" {
					m.state = "held"
				}
			default:
				apply(t, l, settleEntry(id, res))
				m.state = "settled"
			}
			var a, r, h, over int64
			for _, m := range model {
				var spent int64
				for _, v := range m.live {
					spent += v
				}
				a += spent
				rest := m.reserved - spent
				if rest < 0 {
					over -= rest
					rest = 0
				}
				switch m.state {
				case "open":
					r += rest
				case "held":
					h += rest
				}
			}
			p := checkLedger(t, l, fmt.Sprintf("seed %d step %d", seed, step))
			if int64(p.ActualKnown) != a || int64(p.Reserved) != r || int64(p.UnknownHold) != h || int64(p.Overrun) != over || p.Estimated != 0 {
				t.Fatalf("seed %d step %d: ledger %+v, model A=%d R=%d H=%d overrun=%d", seed, step, p, a, r, h, over)
			}
		}
	}
}

// One reservation, one cumulative scope, one invocation, random sequence
// numbers and running totals. The model keeps only "highest sequence seen" and
// "current total": a value counts only if it is newer and not lower.
func TestRandomCumulativeScopesMatchANaiveModel(t *testing.T) {
	for seed := int64(1); seed <= 300; seed++ {
		rng := rand.New(rand.NewSource(seed))
		l := NewLedger()
		apply(t, l, reserveEntry("reserve", "r1", 500))
		var head, total int64
		seen := map[int64]bool{}
		for step := 0; step < 40; step++ {
			seq, value := int64(1+rng.Intn(25)), int64(rng.Intn(900))+1
			r := apply(t, l, cumulative(fmt.Sprintf("u%d", step), "r1", "scope", Count(seq), reported(Ticks(value))))[0]
			var added int64
			if !seen[seq] && seq > head && value >= total {
				added, head, total = value-total, seq, value
			}
			seen[seq] = true
			if int64(r.ActualAdded) != added || r.ActualRemoved != 0 {
				t.Fatalf("seed %d step %d: seq %d value %d receipt %+v, model added %d", seed, step, seq, value, r, added)
			}
			p := checkLedger(t, l, fmt.Sprintf("seed %d step %d", seed, step))
			rest, over := 500-total, int64(0)
			if rest < 0 {
				rest, over = 0, -rest
			}
			if int64(p.ActualKnown) != total || int64(p.Reserved) != rest || int64(p.Overrun) != over || p.UnknownHold != 0 {
				t.Fatalf("seed %d step %d: ledger %+v, model total %d", seed, step, p, total)
			}
		}
	}
}
