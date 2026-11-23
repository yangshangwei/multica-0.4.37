package budget

import (
	"fmt"
	"maps"
	"math"
	"slices"
	"sort"
	"unicode/utf8"
)

type reservationState uint8

const (
	reservationOpen reservationState = iota
	// reservationHeld: a lifecycle fact ended the attempt without proof of
	// its final usage. What is unspent stays occupied as an unknown hold.
	reservationHeld
	reservationSettled
)

// status is the name the state goes by outside the package.
func (s reservationState) status() ReservationStatus {
	switch s {
	case reservationHeld:
		return ReservationHeld
	case reservationSettled:
		return ReservationSettled
	}
	return ReservationOpen
}

type reservation struct {
	reserved int64
	actual   int64
	state    reservationState
	// unsized: opened without an amount. reserved is zero and stays zero.
	unsized bool
	// holds: the reasons of the hold entries accepted for the reservation.
	// Until it settles, a reason is in the set exactly when the unknown set
	// has that reason's item for it: only a hold adds such an item and only
	// the settlement removes it. Anything new that removes one must take the
	// reason out of the set as well.
	holds holdSet
}

// holdSet is a set of hold reasons, one bit each. It is a plain value, so a
// copy of a reservation shares nothing with the original.
type holdSet uint8

// holdBit is the bit of a hold reason. A string that is not one has none.
func holdBit(reason HoldReason) holdSet {
	switch reason {
	case HoldCancelled:
		return 1 << 0
	case HoldTimeout:
		return 1 << 1
	case HoldProcessExit:
		return 1 << 2
	case HoldLostContact:
		return 1 << 3
	case HoldSupersededUnproven:
		return 1 << 4
	}
	return 0
}

// unspent is the part of the reservation not yet turned into actual.
func (r reservation) unspent() int64 {
	if r.actual >= r.reserved {
		return 0
	}
	return r.reserved - r.actual
}

// overrun is the actual beyond the reserved amount. An unsized reservation
// had no amount to exceed.
func (r reservation) overrun() int64 {
	if r.unsized || r.actual <= r.reserved {
		return 0
	}
	return r.actual - r.reserved
}

// weight is what the reservation occupies in A + R + H.
func (r reservation) weight() int64 {
	if r.state == reservationSettled {
		return r.actual
	}
	return r.actual + r.unspent()
}

// Ledger is the replayable state of one budget root. Feeding the same ordered
// entries into an empty Ledger always produces the same state. The zero Ledger
// is empty and ready to use.
//
// Nothing this package exports modifies a Ledger or returns a part of one:
// the fields are unexported, the exported methods only read, and With returns
// a new Ledger and leaves its receiver as it was. So one Ledger may be shared
// and read by any number of goroutines. The one write left to code outside
// the package is to assign a whole Ledger over another (*a = *b); it must not
// be made to a Ledger that is shared.
type Ledger struct {
	entries       map[string]Entry
	receipts      map[string]Receipt
	reservations  map[string]reservation
	scopes        map[string]*scope
	contributions map[string]contribution
	unknown       []UnknownItem
	exclusions    map[string]struct{}
	// unreserved is known actual from usage that names no reservation.
	unreserved int64
	estimated  int64
	// committed is A + R + H, kept so every change is checked against the
	// 64-bit range once.
	committed int64
}

func NewLedger() *Ledger {
	return &Ledger{
		entries:       map[string]Entry{},
		receipts:      map[string]Receipt{},
		reservations:  map[string]reservation{},
		scopes:        map[string]*scope{},
		contributions: map[string]contribution{},
		exclusions:    map[string]struct{}{},
	}
}

// cloneMap copies m. A nil map, as in the zero Ledger, becomes an empty one
// that can be written to.
func cloneMap[K comparable, V any](m map[K]V) map[K]V {
	if m == nil {
		return map[K]V{}
	}
	return maps.Clone(m)
}

func (l *Ledger) clone() *Ledger {
	next := *l
	next.entries = cloneMap(l.entries)
	next.receipts = cloneMap(l.receipts)
	next.reservations = cloneMap(l.reservations)
	next.contributions = cloneMap(l.contributions)
	next.exclusions = cloneMap(l.exclusions)
	next.unknown = slices.Clone(l.unknown)
	next.scopes = make(map[string]*scope, len(l.scopes))
	for id, sc := range l.scopes {
		next.scopes[id] = sc.clone()
	}
	return &next
}

// apply is With that then makes the result the receiver's own state. It
// writes to the ledger, so it is not exported: it is for a ledger that nothing
// else can see yet, such as the one Reduce builds. On any error nothing is
// kept and the receipts are nil.
func (l *Ledger) apply(entries ...Entry) ([]Receipt, error) {
	next, receipts, err := l.With(entries...)
	if err != nil {
		return nil, err
	}
	*l = *next
	return receipts, nil
}

// With returns a new ledger that holds what the receiver holds with entries
// appended in order, and the receipts of entries. The batch is atomic: on any
// error the ledger and the receipts are nil. An entry whose ID is already
// present with equal content is a replay and returns its original receipt
// with Replayed set; the same ID with different content is ErrConflict. The
// receiver is not written to, on success or on error, and the result shares
// no state with it: appending to either never shows in the other.
func (l *Ledger) With(entries ...Entry) (*Ledger, []Receipt, error) {
	next := l.clone()
	receipts := make([]Receipt, 0, len(entries))
	for _, e := range entries {
		r, err := next.applyOne(e)
		if err != nil {
			return nil, nil, err
		}
		receipts = append(receipts, r)
	}
	return next, receipts, nil
}

// Len is the number of entries the ledger holds: each entry it accepted,
// counted once. A replay is not another entry, and nothing of a refused batch
// is held.
func (l *Ledger) Len() int { return len(l.entries) }

// HasReservation reports whether a reserve entry opened a reservation with
// this ID. A held or settled reservation is still reported.
func (l *Ledger) HasReservation(id string) bool {
	_, ok := l.reservations[id]
	return ok
}

// Reservation is the net state of the reservation with this ID. It reports
// false, and nothing else, when no reserve entry opened one.
func (l *Ledger) Reservation(id string) (ReservationProjection, bool) {
	r, ok := l.reservations[id]
	if !ok {
		return ReservationProjection{}, false
	}
	p := ReservationProjection{
		Status:      r.state.status(),
		Unsized:     r.unsized,
		Reserved:    Ticks(r.reserved),
		ActualKnown: Ticks(r.actual),
		Overrun:     Ticks(r.overrun()),
	}
	// A settlement released what was unspent.
	if r.state != reservationSettled {
		p.Unspent = Ticks(r.unspent())
	}
	return p, true
}

// Reduce replays entries from an empty ledger. It is a pure function of the
// ordered entry list.
func Reduce(entries []Entry) (Projection, []Receipt, error) {
	l := NewLedger()
	receipts, err := l.apply(entries...)
	if err != nil {
		return Projection{}, nil, err
	}
	return l.Projection(), receipts, nil
}

func (l *Ledger) applyOne(e Entry) (Receipt, error) {
	if err := validateEntry(e); err != nil {
		return Receipt{}, err
	}
	if prior, ok := l.entries[e.ID]; ok {
		if prior != e {
			return Receipt{}, fmt.Errorf("%w: entry %q", ErrConflict, e.ID)
		}
		r := l.receipts[e.ID]
		r.Replayed = true
		return r, nil
	}
	if e.Kind != KindReserve && e.ReservationID != "" {
		if _, ok := l.reservations[e.ReservationID]; !ok {
			return Receipt{}, fmt.Errorf("%w: %q named by entry %q", ErrUnknownReservation, e.ReservationID, e.ID)
		}
	}
	var (
		r   Receipt
		err error
	)
	switch e.Kind {
	case KindReserve:
		r, err = l.applyReserve(e)
	case KindUsage:
		r, err = l.applyUsage(e)
	case KindHold:
		r = l.applyHold(e)
	case KindSettle:
		r, err = l.applySettle(e)
	case KindGap:
		l.addUnknown(UnknownItem{Reason: e.GapReason, EntryID: e.ID, ReservationID: e.ReservationID, Detail: e.Detail})
		r = Receipt{Disposition: DispositionAccepted, Reason: e.GapReason}
	case KindExclusion:
		l.exclusions[e.Detail] = struct{}{}
		r = Receipt{Disposition: DispositionAccepted}
	}
	if err != nil {
		return Receipt{}, err
	}
	r.EntryID, r.Kind = e.ID, e.Kind
	l.entries[e.ID] = e
	l.receipts[e.ID] = r
	return r, nil
}

// validateEntry checks the entry on its own, before any ledger state. Only
// the fields of the entry's kind may be set.
func validateEntry(e Entry) error {
	if e.ID == "" {
		return fmt.Errorf("%w: missing id", ErrInvalidEntry)
	}
	invalid := func(why string) error {
		return fmt.Errorf("%w: entry %q: %s", ErrInvalidEntry, e.ID, why)
	}
	// The stored form is JSON, which replaces invalid UTF-8: two different
	// identifiers would be stored, and digested, as the same one. The typed
	// string fields are checked against their constants below.
	u := e.Usage
	for _, s := range [...]string{
		e.ID, e.ReservationID, e.Detail,
		u.ScopeID, u.InvocationID, u.BaselineEventID, u.Supersedes,
		u.Cost.Currency, u.Cost.ZeroBasis, u.Cost.PriceVersion, u.Cost.PriceDigest,
	} {
		if !utf8.ValidString(s) {
			return invalid("text is not valid UTF-8")
		}
	}
	var shape Entry
	switch e.Kind {
	case KindReserve:
		// Unsized is the absence of an amount, so it comes with none.
		if e.Unsized && e.ReservedTicks != 0 {
			return invalid("an unsized reservation carries no amount")
		}
		if e.ReservedTicks < 0 {
			return fmt.Errorf("%w: entry %q", ErrNegativeAmount, e.ID)
		}
		shape = Entry{ReservedTicks: e.ReservedTicks, Unsized: e.Unsized}
	case KindHold:
		switch e.HoldReason {
		case HoldCancelled, HoldTimeout, HoldProcessExit, HoldLostContact, HoldSupersededUnproven:
		default:
			return invalid("unknown hold reason")
		}
		shape = Entry{HoldReason: e.HoldReason, Detail: e.Detail}
	case KindSettle:
		shape = Entry{Proof: e.Proof}
	case KindGap:
		if e.GapReason != ReasonCoverageGap && e.GapReason != ReasonPreActivationUnknown {
			return invalid("unknown gap reason")
		}
		shape = Entry{GapReason: e.GapReason, Detail: e.Detail}
	case KindExclusion:
		if e.Detail == "" {
			return invalid("an exclusion names its mechanism in detail")
		}
		shape = Entry{Detail: e.Detail}
	case KindUsage:
		if err := validateUsage(e); err != nil {
			return err
		}
		shape = Entry{Usage: e.Usage}
	default:
		return invalid("unknown kind")
	}
	switch e.Kind {
	case KindReserve, KindHold, KindSettle:
		if e.ReservationID == "" {
			return invalid("missing reservation")
		}
		shape.ReservationID = e.ReservationID
	case KindGap, KindUsage:
		shape.ReservationID = e.ReservationID
	}
	shape.ID, shape.Kind = e.ID, e.Kind
	if e != shape {
		return invalid("fields of another kind are set")
	}
	return nil
}

func (l *Ledger) applyReserve(e Entry) (Receipt, error) {
	if _, ok := l.reservations[e.ReservationID]; ok {
		return Receipt{}, fmt.Errorf("%w: reservation %q is already open", ErrConflict, e.ReservationID)
	}
	committed, err := addChecked(l.committed, int64(e.ReservedTicks))
	if err != nil {
		return Receipt{}, err
	}
	l.committed = committed
	l.reservations[e.ReservationID] = reservation{reserved: int64(e.ReservedTicks), unsized: e.Unsized}
	if !e.Unsized {
		return Receipt{Disposition: DispositionAccepted}, nil
	}
	// Without an amount nothing bounds what the attempt may spend. That is one
	// unknown item until the reservation settles.
	l.addUnknown(UnknownItem{Reason: ReasonReservationUnsized, EntryID: e.ID, ReservationID: e.ReservationID})
	return Receipt{Disposition: DispositionAccepted, Reason: ReasonReservationUnsized}, nil
}

// applyHold records a lifecycle fact. It moves the unspent reservation from
// reserved to unknown hold and never makes allowance available. A reason is
// recorded once for a reservation. Whether it is on record is read from the
// reservation and not searched for in the unknown list, so a hold costs the
// same however many items the ledger has.
func (l *Ledger) applyHold(e Entry) Receipt {
	r := l.reservations[e.ReservationID]
	reason := UnknownReason(e.HoldReason)
	bit := holdBit(e.HoldReason)
	if r.state == reservationSettled || r.holds&bit != 0 {
		return Receipt{Disposition: DispositionStale}
	}
	r.state = reservationHeld
	r.holds |= bit
	l.reservations[e.ReservationID] = r
	l.addUnknown(UnknownItem{Reason: reason, EntryID: e.ID, ReservationID: e.ReservationID, Detail: e.Detail})
	return Receipt{Disposition: DispositionAccepted, Reason: reason}
}

// applySettle ends a reservation and releases what it has not spent. It is
// the only entry that releases allowance.
func (l *Ledger) applySettle(e Entry) (Receipt, error) {
	if !e.Proof.FinalUsage || !e.Proof.ScopeComplete || !e.Proof.NoFurtherExecution {
		return Receipt{}, fmt.Errorf("%w: entry %q", ErrReleaseProofIncomplete, e.ID)
	}
	r := l.reservations[e.ReservationID]
	if r.state == reservationSettled {
		return Receipt{Disposition: DispositionStale}, nil
	}
	// Unknown usage of the reservation stands in the way. Its lifecycle facts
	// and its missing amount do not: they are what a settlement resolves.
	for _, item := range l.unknown {
		if item.ReservationID == e.ReservationID && !isLifecycleReason(item.Reason) && item.Reason != ReasonReservationUnsized {
			return Receipt{}, fmt.Errorf("%w: reservation %q still has unknown usage (%s)", ErrReleaseProofIncomplete, e.ReservationID, item.Reason)
		}
	}
	released := r.unspent()
	r.state = reservationSettled
	l.reservations[e.ReservationID] = r
	l.committed -= released
	// The settlement proves the outcome the lifecycle facts left open, and
	// ends what an unsized reservation left unbounded.
	l.dropUnknown(func(item UnknownItem) bool { return item.ReservationID == e.ReservationID })
	return Receipt{Disposition: DispositionAccepted, Released: Ticks(released)}, nil
}

func isLifecycleReason(reason UnknownReason) bool {
	switch HoldReason(reason) {
	case HoldCancelled, HoldTimeout, HoldProcessExit, HoldLostContact, HoldSupersededUnproven:
		return true
	}
	return false
}

// addActual moves known actual by delta, which may be negative for a
// correction, on a reservation or on the unreserved total.
func (l *Ledger) addActual(reservationID string, delta int64) error {
	if reservationID == "" {
		unreserved, err := addChecked(l.unreserved, delta)
		if err != nil {
			return err
		}
		committed, err := addChecked(l.committed, delta)
		if err != nil {
			return err
		}
		l.unreserved, l.committed = unreserved, committed
		return nil
	}
	r := l.reservations[reservationID]
	before := r.weight()
	actual, err := addChecked(r.actual, delta)
	if err != nil {
		return err
	}
	r.actual = actual
	committed, err := addChecked(l.committed-before, r.weight())
	if err != nil {
		return err
	}
	l.committed = committed
	l.reservations[reservationID] = r
	return nil
}

func (l *Ledger) addEstimated(delta int64) error {
	estimated, err := addChecked(l.estimated, delta)
	if err != nil {
		return err
	}
	l.estimated = estimated
	return nil
}

func (l *Ledger) addUnknown(item UnknownItem) { l.unknown = append(l.unknown, item) }

func (l *Ledger) dropUnknown(match func(UnknownItem) bool) {
	l.unknown = slices.DeleteFunc(l.unknown, match)
}

// Projection derives the net figures. Unknown keeps the order in which its
// items were recorded; Coverage.NotApplicable is sorted.
func (l *Ledger) Projection() Projection {
	p := Projection{
		ActualKnown: Ticks(l.unreserved),
		Estimated:   Ticks(l.estimated),
		Unknown:     append([]UnknownItem{}, l.unknown...),
		Coverage:    Coverage{IncludedAttempts: len(l.reservations), NotApplicable: []string{}},
	}
	// Every sum below is bounded by l.committed, which is range-checked.
	for _, r := range l.reservations {
		p.ActualKnown += Ticks(r.actual)
		p.Overrun += Ticks(r.overrun())
		switch r.state {
		case reservationOpen:
			p.Reserved += Ticks(r.unspent())
		case reservationHeld:
			p.UnknownHold += Ticks(r.unspent())
		}
	}
	for name := range l.exclusions {
		p.Coverage.NotApplicable = append(p.Coverage.NotApplicable, name)
	}
	sort.Strings(p.Coverage.NotApplicable)
	return p
}

// Committed is A + R + H: the allowance the root occupies. A projection that
// did not come from a Ledger may exceed the range; it then saturates.
func (p Projection) Committed() Ticks {
	total := int64(0)
	for _, part := range [3]Ticks{p.ActualKnown, p.Reserved, p.UnknownHold} {
		next, err := addChecked(total, int64(part))
		if err != nil {
			return math.MaxInt64
		}
		total = next
	}
	return Ticks(total)
}

// TotalKnown reports whether the unknown set is empty.
func (p Projection) TotalKnown() bool { return len(p.Unknown) == 0 }

// Remaining is limit minus Committed, never below zero. It is nil whenever
// the unknown set is not empty: without a proven upper bound there is no
// remaining amount, and limit minus known actual must not stand in for one.
func (p Projection) Remaining(limit Ticks) *Ticks {
	if limit < 0 || !p.TotalKnown() {
		return nil
	}
	remaining := Ticks(0)
	if committed := p.Committed(); committed < limit {
		remaining = limit - committed
	}
	return &remaining
}

// UnknownCounts groups the unknown set by reason.
func (p Projection) UnknownCounts() map[UnknownReason]int {
	counts := map[UnknownReason]int{}
	for _, item := range p.Unknown {
		counts[item.Reason]++
	}
	return counts
}
