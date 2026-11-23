package budget

import (
	"encoding/json"
	"reflect"
	"testing"
)

// The tests in this file are about Ledger.Reservation, the reading of one
// reservation. That the readings of a ledger add up to its projection is
// checked on every ledger checkLedger sees (invariants_test.go).

// wantReading fails unless the ledger reads the reservation as want.
func wantReading(t *testing.T, l *Ledger, id string, want ReservationProjection) {
	t.Helper()
	got, ok := l.Reservation(id)
	if !ok || got != want {
		t.Fatalf("Reservation(%q) = %+v, %v; want %+v", id, got, ok, want)
	}
	if !l.HasReservation(id) {
		t.Fatalf("HasReservation(%q) is false for a reservation that is read", id)
	}
}

// wantNoReading fails unless the ledger has no reservation with this ID.
func wantNoReading(t *testing.T, l *Ledger, id string) {
	t.Helper()
	if got, ok := l.Reservation(id); ok || got != (ReservationProjection{}) {
		t.Fatalf("Reservation(%q) = %+v, %v; want nothing", id, got, ok)
	}
	if l.HasReservation(id) {
		t.Fatalf("HasReservation(%q) is true for a reservation that is not read", id)
	}
}

// One ledger with a reservation in each status, with and without an amount,
// that spent less than the amount, all of it and more. In no row are two
// amounts equal unless they have to be, so a reading that hands out the wrong
// one is seen.
func TestReservationReadsEachStatusSizedAndUnsized(t *testing.T) {
	spend := func(reservation string, ticks Ticks) Entry {
		return delta("usage-"+reservation, reservation, "scope-"+reservation, reported(ticks))
	}
	l := NewLedger()
	apply(t, l,
		reserveEntry("reserve-fresh", "fresh", 100),
		reserveEntry("reserve-open", "open", 100), spend("open", 30),
		reserveEntry("reserve-spent", "spent", 100), spend("spent", 100),
		reserveEntry("reserve-over", "over", 100), spend("over", 130),
		reserveEntry("reserve-zero", "zero", 0), spend("zero", 20),

		reserveEntry("reserve-held", "held", 90), spend("held", 25),
		holdEntry("hold-held", "held", HoldTimeout),
		reserveEntry("reserve-held-over", "held-over", 50), spend("held-over", 85),
		holdEntry("hold-held-over", "held-over", HoldCancelled),

		reserveEntry("reserve-settled", "settled", 80), spend("settled", 35),
		settleEntry("settle-settled", "settled"),
		reserveEntry("reserve-settled-over", "settled-over", 10), spend("settled-over", 25),
		settleEntry("settle-settled-over", "settled-over"),
		reserveEntry("reserve-held-settled", "held-settled", 70), spend("held-settled", 45),
		holdEntry("hold-held-settled", "held-settled", HoldLostContact), settleEntry("settle-held-settled", "held-settled"),

		unsizedReserve("reserve-unsized-fresh", "unsized-fresh"),
		unsizedReserve("reserve-unsized", "unsized"), spend("unsized", 40),
		unsizedReserve("reserve-unsized-held", "unsized-held"), spend("unsized-held", 55),
		holdEntry("hold-unsized-held", "unsized-held", HoldProcessExit),
		unsizedReserve("reserve-unsized-settled", "unsized-settled"), spend("unsized-settled", 65),
		settleEntry("settle-unsized-settled", "unsized-settled"),

		// Usage that names no reservation is in no reading.
		delta("usage-none", "", "scope-none", reported(7)),
	)

	for id, want := range map[string]ReservationProjection{
		"fresh": {Status: ReservationOpen, Reserved: 100, Unspent: 100},
		"open":  {Status: ReservationOpen, Reserved: 100, ActualKnown: 30, Unspent: 70},
		"spent": {Status: ReservationOpen, Reserved: 100, ActualKnown: 100},
		"over":  {Status: ReservationOpen, Reserved: 100, ActualKnown: 130, Overrun: 30},
		// A reservation of zero has an amount: all it spends is beyond it.
		"zero": {Status: ReservationOpen, ActualKnown: 20, Overrun: 20},

		// A hold keeps what is unspent occupied.
		"held":      {Status: ReservationHeld, Reserved: 90, ActualKnown: 25, Unspent: 65},
		"held-over": {Status: ReservationHeld, Reserved: 50, ActualKnown: 85, Overrun: 35},

		// A settlement releases it. What was spent beyond the amount stays.
		"settled":      {Status: ReservationSettled, Reserved: 80, ActualKnown: 35},
		"settled-over": {Status: ReservationSettled, Reserved: 10, ActualKnown: 25, Overrun: 15},
		"held-settled": {Status: ReservationSettled, Reserved: 70, ActualKnown: 45},

		// Without an amount there is nothing to leave unspent or to exceed.
		"unsized-fresh":   {Status: ReservationOpen, Unsized: true},
		"unsized":         {Status: ReservationOpen, Unsized: true, ActualKnown: 40},
		"unsized-held":    {Status: ReservationHeld, Unsized: true, ActualKnown: 55},
		"unsized-settled": {Status: ReservationSettled, Unsized: true, ActualKnown: 65},
	} {
		wantReading(t, l, id, want)
	}

	// An ID that is not a reservation's: none, one never seen, one in another
	// letter case or with a space, and the IDs of entries and scopes.
	for _, id := range []string{"", "nope", "OPEN", "open ", "reserve-open", "usage-open", "scope-open", "settle-settled"} {
		wantNoReading(t, l, id)
	}
	// A ledger with no reservation, the zero Ledger among them.
	var zero Ledger
	wantNoReading(t, &zero, "open")
	wantNoReading(t, &zero, "")
	wantNoReading(t, NewLedger(), "open")

	// The same ledger as its projection has it: the unspent of the open
	// reservations is R, that of the held ones is H.
	wantFigures(t, checkLedger(t, l, "a reservation in each status"), 662, 0, 170, 65, 100)
}

// One reservation, read after each entry of its life. A ledger keeps the
// reading it had when a later one is derived from it.
func TestReservationReadingFollowsTheEntries(t *testing.T) {
	estimate := delta("d2", "r1", "scope-2", estimated(500))
	costed := delta("d2-fix", "r1", "scope-2", reported(5))
	costed.Usage.Supersedes, costed.Usage.CorrectionAuthorized = "d2", true
	lowered := delta("late-fix", "r1", "scope-late", reported(15))
	lowered.Usage.Supersedes, lowered.Usage.CorrectionAuthorized = "late", true

	open := func(actual, unspent Ticks) ReservationProjection {
		return ReservationProjection{Status: ReservationOpen, Reserved: 100, ActualKnown: actual, Unspent: unspent}
	}
	held := func(actual, unspent Ticks) ReservationProjection {
		return ReservationProjection{Status: ReservationHeld, Reserved: 100, ActualKnown: actual, Unspent: unspent}
	}
	settled := func(actual, overrun Ticks) ReservationProjection {
		return ReservationProjection{Status: ReservationSettled, Reserved: 100, ActualKnown: actual, Overrun: overrun}
	}
	steps := []struct {
		name  string
		entry Entry
		want  ReservationProjection
	}{
		{"opened", reserveEntry("reserve-1", "r1", 100), open(0, 100)},
		{"known spend", delta("d1", "r1", "scope-1", reported(30)), open(30, 70)},
		{"an estimate is not known spend", estimate, open(30, 70)},
		{"the estimate replaced by a reported cost", costed, open(35, 65)},
		{"another reservation opened", reserveEntry("reserve-2", "r2", 40), open(35, 65)},
		{"spend of the other reservation", delta("other", "r2", "scope-other", reported(60)), open(35, 65)},
		{"spend that names no reservation", delta("none", "", "scope-none", reported(9)), open(35, 65)},
		{"held", holdEntry("hold-1", "r1", HoldTimeout), held(35, 65)},
		{"held for a second reason", holdEntry("hold-2", "r1", HoldLostContact), held(35, 65)},
		{"spend while held", delta("d3", "r1", "scope-3", reported(20)), held(55, 45)},
		{"settled", settleEntry("settle-1", "r1"), settled(55, 0)},
		{"spend after the settlement", delta("late", "r1", "scope-late", reported(70)), settled(125, 25)},
		{"a hold after the settlement", holdEntry("hold-3", "r1", HoldCancelled), settled(125, 25)},
		{"the late spend lowered", lowered, settled(70, 0)},
		{"settled again", settleEntry("settle-2", "r1"), settled(70, 0)},
	}

	l := NewLedger()
	wantNoReading(t, l, "r1")
	ledgers := make([]*Ledger, len(steps))
	for i, step := range steps {
		next, _, err := l.With(step.entry)
		if err != nil {
			t.Fatalf("%s: %v", step.name, err)
		}
		got, ok := next.Reservation("r1")
		if !ok || got != step.want {
			t.Fatalf("%s: Reservation = %+v, %v; want %+v", step.name, got, ok, step.want)
		}
		checkLedger(t, next, step.name)
		ledgers[i], l = next, next
	}
	for i, step := range steps {
		if got, ok := ledgers[i].Reservation("r1"); !ok || got != step.want {
			t.Errorf("the ledger of %q now reads %+v, %v; want %+v", step.name, got, ok, step.want)
		}
	}
	wantReading(t, l, "r2", ReservationProjection{Status: ReservationOpen, Reserved: 40, ActualKnown: 60, Overrun: 20})
	wantNoReading(t, ledgers[3], "r2")
}

// A reading is a value. No field of it can refer to anything, so no reading
// is a part of the ledger it was taken from, and two readings compare with ==.
func TestReservationReadingHoldsNoReference(t *testing.T) {
	typ := reflect.TypeOf(ReservationProjection{})
	if typ.NumField() != 6 {
		t.Fatalf("a reading has %d fields", typ.NumField())
	}
	for i := 0; i < typ.NumField(); i++ {
		switch field := typ.Field(i); field.Type.Kind() {
		case reflect.String, reflect.Bool, reflect.Int64:
		default:
			t.Errorf("the field %s is a %s", field.Name, field.Type.Kind())
		}
	}

	// The names it goes by outside the package.
	if ReservationOpen != "open" || ReservationHeld != "held" || ReservationSettled != "settled" {
		t.Fatalf("the statuses are %q, %q and %q", ReservationOpen, ReservationHeld, ReservationSettled)
	}
	raw, err := json.Marshal(ReservationProjection{Status: ReservationHeld, Unsized: true, Reserved: 1, ActualKnown: 2, Unspent: 3, Overrun: 4})
	const want = `{"status":"held","unsized":true,"reserved_ticks":"1","actual_known_ticks":"2","unspent_ticks":"3","overrun_ticks":"4"}`
	if err != nil || string(raw) != want {
		t.Fatalf("JSON = %s, %v; want %s", raw, err, want)
	}
}
