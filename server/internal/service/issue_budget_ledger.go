package service

import (
	"container/list"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/multica-ai/multica/server/internal/budget"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// The ledger of a budget root (design 10.4 and 10.5).
//
// The ledger of a root is the replay of its entry rows in seq order. The
// replay is the definition. The cache in this file only saves replaying what
// has not changed since a state was last built.
//
// The cache holds committed entries only. A state gets into it at two moments
// and at no other:
//
//   - a writer that holds the root lock has loaded the ledger. Every entry up
//     to the revision on the root row is committed then, because whoever
//     appended it held that lock until its commit;
//   - a writer's own append, once the commit of its transaction is confirmed.
//
// A state is a *budget.Ledger, and nothing modifies one: the next state is
// another ledger. The cache keeps pointers and replaces pointers. Nothing
// here assigns through one, so any number of goroutines may read a state
// while another is published.
//
// Each process has its own cache. What keeps two processes right is the
// comparison of revisions under the root lock, not the cache.
//
// A cached state never decides that a ledger is unavailable. A state that does
// not go with the rows of its root may be all that is wrong: the database was
// put back, or the rows were repaired, behind a process that still holds the
// state from before. Such a state is dropped, with a warning, and the same
// load replays every row. Only that replay says a ledger is unavailable.

// issueBudgetLedgerCacheLimit is what the ledger cache of a process holds at
// most, as the sum of the weights of the roots in it.
const issueBudgetLedgerCacheLimit = 50_000

// issueBudgetLedgerRootWeight is what a root weighs beyond its entries. A
// ledger has a fixed cost, and on a root of a few entries it is most of what
// the root takes.
const issueBudgetLedgerRootWeight = 2

// issueBudgetLedgerEntryMaxBytes is the longest stored form of an entry that a
// server writes. It is what makes the cache limit a bound on memory: a ledger
// keeps its entries whole, the weight of a root counts entries and not bytes,
// and the metering package lets one entry be budget.MaxStoredEntryBytes long.
// Every entry a server writes goes through appendEntries, which refuses a
// longer one. The largest entry of the first slice, a usage entry with the
// longest values a version 1 report may carry, is below this; the test beside
// the append says by how much.
//
// A load does not apply it. A row that another build or an operator wrote is
// read as it is, up to what the metering package and the table take.
const issueBudgetLedgerEntryMaxBytes = 2048

// issueBudgetLedgers is the ledger cache of this process.
var issueBudgetLedgers = newIssueBudgetLedgerCache(issueBudgetLedgerCacheLimit)

// ErrIssueBudgetLedgerUnavailable says that the entry rows of a root do not
// replay to the ledger its root row records (design 10.5). The root is still
// there and its budget is not switched off: what it has spent cannot be said
// until the rows are repaired.
var ErrIssueBudgetLedgerUnavailable = errors.New("budget ledger unavailable")

// issueBudgetLedgerCause is the check a ledger failed.
type issueBudgetLedgerCause string

const (
	// A row does not hold the stored form of an entry in a version this build
	// reads.
	issueBudgetLedgerStoredForm issueBudgetLedgerCause = "stored_form"
	// The ledger refuses the entries in the order of the rows, or two rows
	// are one entry.
	issueBudgetLedgerReplay issueBudgetLedgerCause = "replay"
	// The rows are not numbered without a gap up to the revision on the root
	// row.
	issueBudgetLedgerSequence issueBudgetLedgerCause = "sequence"
	// The entries replay to other figures than the root row holds.
	issueBudgetLedgerProjection issueBudgetLedgerCause = "projection"
)

// issueBudgetLedgerAheadOfRoot is why a cached state was dropped that failed
// none of the checks above: it holds more entries than the root row its writer
// locked. It is a cause in the warning about a dropped state only, never the
// cause of an unavailable ledger.
const issueBudgetLedgerAheadOfRoot issueBudgetLedgerCause = "ahead_of_root"

// issueBudgetLedgerUnavailableError is ErrIssueBudgetLedgerUnavailable with
// the check that failed. The error of the metering package behind it is kept
// as text: a failed load is not a refused entry, and must not be taken for
// one where the two share a sentinel.
type issueBudgetLedgerUnavailableError struct {
	RootID   pgtype.UUID
	Revision int64
	Cause    issueBudgetLedgerCause
	Detail   string
}

func (e *issueBudgetLedgerUnavailableError) Error() string {
	return fmt.Sprintf("%v: root %s at revision %d: %s: %s",
		ErrIssueBudgetLedgerUnavailable, util.UUIDToString(e.RootID), e.Revision, e.Cause, e.Detail)
}

func (e *issueBudgetLedgerUnavailableError) Is(target error) bool {
	return target == ErrIssueBudgetLedgerUnavailable
}

// issueBudgetLedgerFigures is the projection of a ledger as a root row keeps
// it.
type issueBudgetLedgerFigures struct {
	ActualKnownTicks int64
	EstimatedTicks   int64
	ReservedTicks    int64
	UnknownHoldTicks int64
	OverrunTicks     int64
	UnknownCount     int64
}

// issueBudgetLedgerFiguresOf is what the root row of l has to hold.
func issueBudgetLedgerFiguresOf(l *budget.Ledger) issueBudgetLedgerFigures {
	p := l.Projection()
	return issueBudgetLedgerFigures{
		ActualKnownTicks: int64(p.ActualKnown),
		EstimatedTicks:   int64(p.Estimated),
		ReservedTicks:    int64(p.Reserved),
		UnknownHoldTicks: int64(p.UnknownHold),
		OverrunTicks:     int64(p.Overrun),
		UnknownCount:     int64(len(p.Unknown)),
	}
}

// issueBudgetRootFigures is what a root row holds.
func issueBudgetRootFigures(root db.IssueBudgetRoot) issueBudgetLedgerFigures {
	return issueBudgetLedgerFigures{
		ActualKnownTicks: root.ActualKnownTicks,
		EstimatedTicks:   root.EstimatedTicks,
		ReservedTicks:    root.ReservedTicks,
		UnknownHoldTicks: root.UnknownHoldTicks,
		OverrunTicks:     root.OverrunTicks,
		UnknownCount:     root.UnknownCount,
	}
}

// issueBudgetLedgerCache keeps, for a root, the ledger of a committed prefix
// of its entries and the revision of that prefix.
type issueBudgetLedgerCache struct {
	// limit is the largest sum of weights the cache holds.
	limit int
	// log receives the warnings of the cache: a root that is too large to
	// cache, and a cached state that was dropped because it did not go with
	// the database. Nil is the default logger of the process.
	log *slog.Logger

	mu sync.Mutex
	// roots finds the state of a root. recent holds the same states, the most
	// recently used in front. weight is the sum of their weights.
	roots  map[pgtype.UUID]*list.Element
	recent *list.List
	weight int
}

// issueBudgetCachedLedger is the state of one root in the cache: the ledger
// of its entries up to revision.
type issueBudgetCachedLedger struct {
	rootID   pgtype.UUID
	revision int64
	ledger   *budget.Ledger
}

func newIssueBudgetLedgerCache(limit int) *issueBudgetLedgerCache {
	return &issueBudgetLedgerCache{limit: limit, roots: map[pgtype.UUID]*list.Element{}, recent: list.New()}
}

// issueBudgetLedgerWeight is what a ledger weighs in the cache: one for each
// entry, and the fixed part.
func issueBudgetLedgerWeight(l *budget.Ledger) int {
	return l.Len() + issueBudgetLedgerRootWeight
}

// lookup returns the state the cache has for a root and its revision, for a
// caller whose root row is at revision, and marks the root as used.
//
// A state ahead of the caller's root row is not returned. Under the root lock
// the row is the newest there is, so that state describes a database that is
// gone: it is removed, and the warning about a dropped state says so. Without
// the lock it is a commit after the caller's snapshot: the cache is right,
// stays exactly as it is, and nothing is logged.
func (c *issueBudgetLedgerCache) lookup(rootID pgtype.UUID, revision int64, holdsRootLock bool) (*budget.Ledger, int64, bool) {
	c.mu.Lock()
	element, ok := c.roots[rootID]
	if !ok {
		c.mu.Unlock()
		return nil, 0, false
	}
	// A state in the cache is replaced, never changed, so it is read here
	// after the mutex is released.
	cached := element.Value.(*issueBudgetCachedLedger)
	ahead := cached.revision > revision
	switch {
	case !ahead:
		c.recent.MoveToFront(element)
	case holdsRootLock:
		c.remove(element)
	}
	c.mu.Unlock()
	if !ahead {
		return cached.ledger, cached.revision, true
	}
	if holdsRootLock {
		c.warnDropped(rootID, cached.revision, revision, issueBudgetLedgerAheadOfRoot)
	}
	return nil, 0, false
}

// publish offers the ledger of a root at revision. The caller knows every
// entry of it to be committed. Only a newer state replaces the one that is
// there, so a publication that arrives late does not put back what a later
// one replaced. The roots used least recently make room. A ledger that
// weighs more than the cache holds is not kept, and neither is an older state
// of its root: the memory the cache takes is bounded by its limit.
func (c *issueBudgetLedgerCache) publish(rootID pgtype.UUID, revision int64, ledger *budget.Ledger) {
	weight := issueBudgetLedgerWeight(ledger)
	c.mu.Lock()
	defer c.mu.Unlock()
	element, ok := c.roots[rootID]
	if weight > c.limit {
		if ok {
			c.remove(element)
		}
		return
	}
	state := &issueBudgetCachedLedger{rootID: rootID, revision: revision, ledger: ledger}
	if ok {
		cached := element.Value.(*issueBudgetCachedLedger)
		if revision <= cached.revision {
			return
		}
		c.weight += weight - issueBudgetLedgerWeight(cached.ledger)
		element.Value = state
		c.recent.MoveToFront(element)
	} else {
		c.roots[rootID] = c.recent.PushFront(state)
		c.weight += weight
	}
	// The state just published is in front and fits by itself, so this ends
	// before it reaches it.
	for c.weight > c.limit {
		c.remove(c.recent.Back())
	}
}

// forget removes the state of a root: whatever is there when used is nil,
// and otherwise only if it is still the state used. It reports whether it
// removed one.
func (c *issueBudgetLedgerCache) forget(rootID pgtype.UUID, used *budget.Ledger) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	element, ok := c.roots[rootID]
	if !ok {
		return false
	}
	if used != nil && element.Value.(*issueBudgetCachedLedger).ledger != used {
		return false
	}
	c.remove(element)
	return true
}

// remove takes one state out. The caller holds mu.
func (c *issueBudgetLedgerCache) remove(element *list.Element) {
	cached := c.recent.Remove(element).(*issueBudgetCachedLedger)
	delete(c.roots, cached.rootID)
	c.weight -= issueBudgetLedgerWeight(cached.ledger)
}

func (c *issueBudgetLedgerCache) logger() *slog.Logger {
	if c.log != nil {
		return c.log
	}
	return slog.Default()
}

// warnDropped reports a cached state that was dropped because it did not go
// with the database: the state of cachedRevision entries, met by a caller
// whose root row is at rootRevision, for the cause given. It is the only
// trace of a cache that disagreed with the database, since the load that
// dropped the state replays every row and may well succeed.
func (c *issueBudgetLedgerCache) warnDropped(rootID pgtype.UUID, cachedRevision, rootRevision int64, cause issueBudgetLedgerCause) {
	c.logger().Warn("cached budget ledger does not go with the database and is dropped: the ledger is replayed in full",
		"budget_root_id", util.UUIDToString(rootID),
		"cached_revision", cachedRevision,
		"root_revision", rootRevision,
		"cause", string(cause),
	)
}

// loadUnderRootLock returns the ledger of a root for a writer: root is the
// row LockIssueBudgetRoot returned in the caller's transaction, and the
// transaction has not appended to the root since. Such a caller sees every
// entry there is, all of them committed, and what it loads is published.
//
// A transaction that has appended goes on with the ledger of its append. It
// must not load again: the rows it would read are its own, not committed.
func (c *issueBudgetLedgerCache) loadUnderRootLock(ctx context.Context, q *db.Queries, root db.IssueBudgetRoot) (*budget.Ledger, error) {
	return c.load(ctx, q, root, true)
}

// loadWithoutRootLock returns the ledger of a root as of the root row the
// caller read, for a reader at any isolation level. Entries after that row's
// revision are later commits and are left out; a cached state newer than the
// row is neither used nor removed; nothing is published.
func (c *issueBudgetLedgerCache) loadWithoutRootLock(ctx context.Context, q *db.Queries, root db.IssueBudgetRoot) (*budget.Ledger, error) {
	return c.load(ctx, q, root, false)
}

// load returns the ledger of root as of the revision on the row given. It
// asks for no lock and for no isolation level itself.
//
// With a cached state of an older revision only the entries after it are
// read; with a cached state of the same revision none is read; without one
// the whole ledger is replayed. On each path the entries must be numbered
// without a gap up to the root's revision and the ledger must project to the
// figures on the root row.
//
// A cached state that fails a check says nothing about the rows (design 10.4).
// It is dropped, with a warning, and every row is read and replayed within
// this load. Only that replay decides: when it fails, the error is
// ErrIssueBudgetLedgerUnavailable with the cause it found (design 10.5), and
// when it passes, its ledger is the result like any other. Any other error is
// a failed read, and leaves the cache as it is.
func (c *issueBudgetLedgerCache) load(ctx context.Context, q *db.Queries, root db.IssueBudgetRoot, holdsRootLock bool) (*budget.Ledger, error) {
	if base, baseRevision, cached := c.lookup(root.ID, root.LedgerRevision, holdsRootLock); cached {
		ledger, err := replayIssueBudgetLedgerFrom(ctx, q, root, base, baseRevision, holdsRootLock)
		var failed *issueBudgetLedgerUnavailableError
		if !errors.As(err, &failed) {
			if err != nil {
				return nil, err
			}
			return c.loaded(root, ledger, holdsRootLock), nil
		}
		// Under the root lock whatever is cached goes. A reader drops the
		// state it used and no other: one published since belongs to a later
		// revision than this reader worked with, and stays.
		used := base
		if holdsRootLock {
			used = nil
		}
		if c.forget(root.ID, used) {
			c.warnDropped(root.ID, baseRevision, root.LedgerRevision, failed.Cause)
		}
	}
	ledger, err := replayIssueBudgetLedgerFrom(ctx, q, root, budget.NewLedger(), 0, holdsRootLock)
	if err != nil {
		if holdsRootLock && errors.Is(err, ErrIssueBudgetLedgerUnavailable) {
			// Nothing stays cached of a root whose ledger is unavailable, a
			// state published while the rows were read included.
			c.forget(root.ID, nil)
		}
		return nil, err
	}
	return c.loaded(root, ledger, holdsRootLock), nil
}

// loaded hands out a ledger that passed every check against root, and
// publishes it for a caller that holds the root lock.
func (c *issueBudgetLedgerCache) loaded(root db.IssueBudgetRoot, ledger *budget.Ledger, holdsRootLock bool) *budget.Ledger {
	if issueBudgetLedgerWeight(ledger) > c.limit {
		c.logger().Warn("budget ledger is too large to cache and is replayed in full on every load",
			"budget_root_id", util.UUIDToString(root.ID),
			"entries", ledger.Len(),
			"cache_limit", c.limit,
		)
	}
	if holdsRootLock {
		c.publish(root.ID, root.LedgerRevision, ledger)
	}
	return ledger
}

// replayIssueBudgetLedgerFrom reads the entry rows of root after baseRevision
// and replays them onto base, the ledger at baseRevision. At the root's own
// revision there is nothing to read, and base is checked against the root row
// as it is.
func replayIssueBudgetLedgerFrom(ctx context.Context, q *db.Queries, root db.IssueBudgetRoot, base *budget.Ledger, baseRevision int64, holdsRootLock bool) (*budget.Ledger, error) {
	var rows []db.ListIssueBudgetEntryFormsAfterRow
	if baseRevision < root.LedgerRevision {
		var err error
		rows, err = q.ListIssueBudgetEntryFormsAfter(ctx, db.ListIssueBudgetEntryFormsAfterParams{
			WorkspaceID:  root.WorkspaceID,
			BudgetRootID: root.ID,
			AfterSeq:     baseRevision,
		})
		if err != nil {
			return nil, fmt.Errorf("read budget ledger entries: %w", err)
		}
	}
	return replayIssueBudgetLedger(root, base, baseRevision, rows, holdsRootLock)
}

// replayIssueBudgetLedger applies rows, the entry rows after baseRevision in
// seq order, to base, the ledger at baseRevision, and checks the result
// against the root row. Every error it returns is
// ErrIssueBudgetLedgerUnavailable.
func replayIssueBudgetLedger(root db.IssueBudgetRoot, base *budget.Ledger, baseRevision int64, rows []db.ListIssueBudgetEntryFormsAfterRow, holdsRootLock bool) (*budget.Ledger, error) {
	unavailable := func(cause issueBudgetLedgerCause, format string, args ...any) error {
		return &issueBudgetLedgerUnavailableError{
			RootID:   root.ID,
			Revision: root.LedgerRevision,
			Cause:    cause,
			Detail:   fmt.Sprintf(format, args...),
		}
	}
	if !holdsRootLock {
		// Appended after the root row was read. Under the root lock nothing
		// can be, and such a row is one the root row does not account for.
		for len(rows) > 0 && rows[len(rows)-1].Seq > root.LedgerRevision {
			rows = rows[:len(rows)-1]
		}
	}
	entries := make([]budget.Entry, 0, len(rows))
	for i, row := range rows {
		if want := baseRevision + int64(i) + 1; row.Seq != want {
			return nil, unavailable(issueBudgetLedgerSequence, "seq %d follows seq %d", row.Seq, want-1)
		}
		entry, err := budget.DecodeEntry(int(row.SchemaVersion), row.EntryJson)
		if err != nil {
			return nil, unavailable(issueBudgetLedgerStoredForm, "seq %d: %v", row.Seq, err)
		}
		entries = append(entries, entry)
	}
	if last := baseRevision + int64(len(rows)); last != root.LedgerRevision {
		return nil, unavailable(issueBudgetLedgerSequence, "the entries end at seq %d", last)
	}
	ledger := base
	if len(entries) > 0 {
		// One call for all of them: each call copies the whole state.
		next, _, err := base.With(entries...)
		if err != nil {
			return nil, unavailable(issueBudgetLedgerReplay, "%v", err)
		}
		ledger = next
	}
	if int64(ledger.Len()) != root.LedgerRevision {
		return nil, unavailable(issueBudgetLedgerReplay, "the rows replay to %d entries: a row repeats an entry", ledger.Len())
	}
	if replayed, stored := issueBudgetLedgerFiguresOf(ledger), issueBudgetRootFigures(root); replayed != stored {
		return nil, unavailable(issueBudgetLedgerProjection, "the entries replay to %+v, the root row holds %+v", replayed, stored)
	}
	return ledger, nil
}

// issueBudgetLedgerEntry is an entry the server appends itself, with what its
// row says about how it arrived.
type issueBudgetLedgerEntry struct {
	Entry budget.Entry
	// Source names what produced the entry.
	Source string
	// OperationID is the committed operation that appends the entry.
	OperationID pgtype.UUID
	// AttemptID preserves attribution when the ledger had no reservation. If
	// both are present, it must agree with Entry.ReservationID.
	AttemptID         pgtype.UUID
	AdapterVersion    string
	NormalizerVersion string
	ProducerEpoch     string
	PayloadDigest     string
	ObservedAt        time.Time
	ReportedAt        time.Time
	ReceivedAt        time.Time
	Completeness      string
	PriceSnapshot     []byte
	PriceDigest       string
}

// issueBudgetEntryRow is the part of an entry row that follows from the entry
// alone: its identity, its kind, the attempt, invocation and scope columns,
// the stored form with its digest, and the version of that form. This is the
// one place those columns are derived, so they cannot disagree with the bytes
// beside them.
//
// The reservation of an entry is the attempt it belongs to, written as the
// canonical lowercase text of the attempt's UUID. Another spelling of the
// same UUID is refused: the ledger would know the reservation under a name
// the attempt id does not render to. An entry that names no reservation gets
// no attempt here.
func issueBudgetEntryRow(e budget.Entry) (db.InsertIssueBudgetEntryParams, error) {
	raw, digest, err := budget.EncodeEntry(e)
	if err != nil {
		return db.InsertIssueBudgetEntryParams{}, fmt.Errorf("encode budget entry: %w", err)
	}
	row := db.InsertIssueBudgetEntryParams{
		EntryID:       e.ID,
		Kind:          string(e.Kind),
		EntryJson:     raw,
		EntryDigest:   digest,
		SchemaVersion: int32(budget.EntryFormVersion),
	}
	if e.ReservationID != "" {
		attemptID, err := util.ParseUUID(e.ReservationID)
		if err != nil || util.UUIDToString(attemptID) != e.ReservationID {
			return db.InsertIssueBudgetEntryParams{}, fmt.Errorf("budget entry %q: reservation %q is not the canonical text of an attempt id", e.ID, e.ReservationID)
		}
		row.AttemptID = attemptID
	}
	if e.Kind == budget.KindUsage {
		row.InvocationID = pgtype.Text{String: e.Usage.InvocationID, Valid: true}
		row.ScopeID = pgtype.Text{String: e.Usage.ScopeID, Valid: true}
	}
	return row, nil
}

// issueBudgetLedgerAppend is an append whose transaction has not committed
// yet.
type issueBudgetLedgerAppend struct {
	// Root is the root row as the append left it.
	Root db.IssueBudgetRoot
	// Ledger is the ledger with the entries of the append.
	Ledger *budget.Ledger
	// Receipts holds the receipt of each entry given, in the order given. An
	// entry the ledger already held has Replayed set and got no row.
	Receipts []budget.Receipt

	cache *issueBudgetLedgerCache
}

// appendEntries appends entries to the ledger of a root inside the caller's
// transaction. The transaction holds the root lock; root is the row it locked
// and ledger is what loadUnderRootLock returned for that row, or Root and
// Ledger of the append the transaction made before.
//
// Every entry is encoded before anything is written, and the ledger takes
// them in one batch: an entry that is refused leaves no row. An entry whose
// stored form is longer than issueBudgetLedgerEntryMaxBytes is refused as an
// invalid entry, also when the ledger holds it already and it would get no
// row. Each entry the ledger did not hold gets one row, in the order given,
// numbered on from the root's revision; an entry it held gets none. The root
// row moves to the seq of the last row with the projection of the new ledger.
// The new ledger must hold as many entries as the new revision says, or the
// append fails.
//
// Nothing is published here. The caller calls Committed on the result once
// its commit is confirmed. An error leaves the transaction to be rolled back.
func (c *issueBudgetLedgerCache) appendEntries(ctx context.Context, q *db.Queries, root db.IssueBudgetRoot, ledger *budget.Ledger, entries []issueBudgetLedgerEntry) (issueBudgetLedgerAppend, error) {
	rows := make([]db.InsertIssueBudgetEntryParams, len(entries))
	batch := make([]budget.Entry, len(entries))
	for i, e := range entries {
		row, err := issueBudgetEntryRow(e.Entry)
		if err != nil {
			return issueBudgetLedgerAppend{}, err
		}
		if len(row.EntryJson) > issueBudgetLedgerEntryMaxBytes {
			return issueBudgetLedgerAppend{}, fmt.Errorf("%w: budget entry %q: a stored form of %d bytes, the limit for an entry a server writes is %d",
				budget.ErrInvalidEntry, e.Entry.ID, len(row.EntryJson), issueBudgetLedgerEntryMaxBytes)
		}
		row.WorkspaceID, row.BudgetRootID = root.WorkspaceID, root.ID
		row.Source, row.OperationID = e.Source, e.OperationID
		if e.AttemptID.Valid {
			if row.AttemptID.Valid && row.AttemptID != e.AttemptID {
				return issueBudgetLedgerAppend{}, fmt.Errorf("%w: entry attempt differs from reservation", budget.ErrInvalidEntry)
			}
			row.AttemptID = e.AttemptID
		}
		row.AdapterVersion = pgtype.Text{String: e.AdapterVersion, Valid: e.AdapterVersion != ""}
		row.NormalizerVersion = pgtype.Text{String: e.NormalizerVersion, Valid: e.NormalizerVersion != ""}
		row.ProducerEpoch = pgtype.Text{String: e.ProducerEpoch, Valid: e.ProducerEpoch != ""}
		row.PayloadDigest = pgtype.Text{String: e.PayloadDigest, Valid: e.PayloadDigest != ""}
		row.Completeness = pgtype.Text{String: e.Completeness, Valid: e.Completeness != ""}
		row.ObservedAt = pgtype.Timestamptz{Time: e.ObservedAt, Valid: e.Source == "daemon_report" || !e.ObservedAt.IsZero()}
		row.ReportedAt = pgtype.Timestamptz{Time: e.ReportedAt, Valid: e.Source == "daemon_report" || !e.ReportedAt.IsZero()}
		row.ReceivedAt = pgtype.Timestamptz{Time: e.ReceivedAt, Valid: e.Source == "daemon_report" || !e.ReceivedAt.IsZero()}
		row.PriceSnapshot = e.PriceSnapshot
		row.PriceDigest = pgtype.Text{String: e.PriceDigest, Valid: e.PriceDigest != ""}
		rows[i], batch[i] = row, e.Entry
	}
	next, receipts, err := ledger.With(batch...)
	if err != nil {
		return issueBudgetLedgerAppend{}, fmt.Errorf("append budget entries: %w", err)
	}
	revision := root.LedgerRevision
	for _, receipt := range receipts {
		if !receipt.Replayed {
			revision++
		}
	}
	if int64(next.Len()) != revision {
		return issueBudgetLedgerAppend{}, fmt.Errorf("append budget entries: the ledger holds %d entries where revision %d is due", next.Len(), revision)
	}
	if revision == root.LedgerRevision {
		return issueBudgetLedgerAppend{Root: root, Ledger: ledger, Receipts: receipts, cache: c}, nil
	}
	seq := root.LedgerRevision
	for i, receipt := range receipts {
		if receipt.Replayed {
			continue
		}
		seq++
		committed, err := json.Marshal(receipt)
		if err != nil {
			return issueBudgetLedgerAppend{}, fmt.Errorf("encode budget entry receipt: %w", err)
		}
		rows[i].Seq, rows[i].Receipt = seq, committed
		if _, err := q.InsertIssueBudgetEntry(ctx, rows[i]); err != nil {
			return issueBudgetLedgerAppend{}, fmt.Errorf("insert budget entry: %w", err)
		}
	}
	figures := issueBudgetLedgerFiguresOf(next)
	updated, err := q.UpdateIssueBudgetRootLedger(ctx, db.UpdateIssueBudgetRootLedgerParams{
		LedgerRevision:     revision,
		ActualKnownTicks:   figures.ActualKnownTicks,
		EstimatedTicks:     figures.EstimatedTicks,
		ReservedTicks:      figures.ReservedTicks,
		UnknownHoldTicks:   figures.UnknownHoldTicks,
		OverrunTicks:       figures.OverrunTicks,
		UnknownCount:       figures.UnknownCount,
		WorkspaceID:        root.WorkspaceID,
		ID:                 root.ID,
		FromLedgerRevision: root.LedgerRevision,
	})
	if err != nil {
		return issueBudgetLedgerAppend{}, fmt.Errorf("move budget root to revision %d: %w", revision, err)
	}
	return issueBudgetLedgerAppend{Root: updated, Ledger: next, Receipts: receipts, cache: c}, nil
}

// Committed publishes the ledger of the append. It is for the moment the
// commit of the transaction that made the append is confirmed, and for no
// other: after a rollback, or while the outcome of the commit is not known,
// the append is let go without it, and the next load reads what the database
// holds. Inside a savepoint the commit that counts is the outer one.
func (a issueBudgetLedgerAppend) Committed() {
	if a.cache == nil {
		return
	}
	a.cache.publish(a.Root.ID, a.Root.LedgerRevision, a.Ledger)
}
