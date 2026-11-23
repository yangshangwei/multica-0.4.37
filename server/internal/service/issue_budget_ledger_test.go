package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"log/slog"
	"math"
	"os"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/multica-ai/multica/server/internal/budget"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// The ledger load and its cache (design 10.4) and the ledger that cannot be
// loaded (design 10.5), driven directly: no caller of them exists before
// claim finalization is wired in. The entry rows are written by the tests
// through budget.EncodeEntry and the insert query, the way another process
// would have committed them, and the figures on the root row are literals.

// issueBudgetLedgerStatements records the statements sent through it by their
// sqlc name. A read of entry forms is recorded with the seq it reads after.
type issueBudgetLedgerStatements struct {
	db.DBTX
	// before, when set, runs before a statement is passed on, with the name
	// the statement is recorded under.
	before func(name string)
	mu     sync.Mutex
	sent   []string
}

func (s *issueBudgetLedgerStatements) record(sql string, args []any) {
	name := strings.TrimSpace(sql)
	if rest, ok := strings.CutPrefix(name, "-- name: "); ok {
		name, _, _ = strings.Cut(rest, " ")
	}
	if name == "ListIssueBudgetEntryFormsAfter" {
		name = fmt.Sprintf("%s after %d", name, args[2])
	}
	s.mu.Lock()
	s.sent = append(s.sent, name)
	s.mu.Unlock()
	if s.before != nil {
		s.before(name)
	}
}

func (s *issueBudgetLedgerStatements) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	s.record(sql, args)
	return s.DBTX.Exec(ctx, sql, args...)
}

func (s *issueBudgetLedgerStatements) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	s.record(sql, args)
	return s.DBTX.Query(ctx, sql, args...)
}

func (s *issueBudgetLedgerStatements) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	s.record(sql, args)
	return s.DBTX.QueryRow(ctx, sql, args...)
}

func (s *issueBudgetLedgerStatements) names() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string{}, s.sent...)
}

const (
	issueBudgetReadWholeLedger = "ListIssueBudgetEntryFormsAfter after 0"
	issueBudgetEntryDomain     = "budget-entry-v1\n"
)

// issueBudgetStoredDigest is the digest the entry table wants beside a stored
// form, computed here and not by the metering package.
func issueBudgetStoredDigest(raw []byte) string {
	sum := sha256.Sum256(append([]byte(issueBudgetEntryDomain), raw...))
	return hex.EncodeToString(sum[:])
}

// The entries of design 10.2, by their identity format.

func issueBudgetReserveEntry(attempt string, ticks budget.Ticks) budget.Entry {
	return budget.Entry{ID: "reserve/" + attempt, Kind: budget.KindReserve, ReservationID: attempt, ReservedTicks: ticks}
}

func issueBudgetUnsizedReserveEntry(attempt string) budget.Entry {
	return budget.Entry{ID: "reserve/" + attempt, Kind: budget.KindReserve, ReservationID: attempt, Unsized: true}
}

func issueBudgetHoldEntry(attempt string, reason budget.HoldReason) budget.Entry {
	return budget.Entry{ID: "hold/" + attempt + "/" + string(reason), Kind: budget.KindHold, ReservationID: attempt, HoldReason: reason}
}

func issueBudgetUsageEntry(attempt, invocation, model string, cost budget.Cost) budget.Entry {
	return budget.Entry{
		ID: "usage/" + attempt + "/" + invocation + "/" + model, Kind: budget.KindUsage, ReservationID: attempt,
		Usage: budget.Usage{ScopeID: attempt + "/" + invocation, InvocationID: invocation, Mode: budget.ModeDelta, Cost: cost},
	}
}

// issueBudgetPadToForm is e with its detail made longer until the stored form
// of the entry is exactly size bytes.
func issueBudgetPadToForm(t testing.TB, e budget.Entry, size int) budget.Entry {
	t.Helper()
	raw, _, err := budget.EncodeEntry(e)
	if err != nil {
		t.Fatalf("encode entry %q: %v", e.ID, err)
	}
	if len(raw) > size {
		t.Fatalf("entry %q has a stored form of %d bytes before any padding, want %d", e.ID, len(raw), size)
	}
	e.Detail += strings.Repeat("x", size-len(raw))
	if raw, _, err = budget.EncodeEntry(e); err != nil || len(raw) != size {
		t.Fatalf("entry %q: a stored form of %d bytes, want %d: %v", e.ID, len(raw), size, err)
	}
	return e
}

// issueBudgetGapEntry is a coverage gap of an attempt, as design 10.2 names
// it.
func issueBudgetGapEntry(attempt string) budget.Entry {
	return budget.Entry{ID: "gap/" + attempt + "/" + uuid.NewString(), Kind: budget.KindGap, ReservationID: attempt, GapReason: budget.ReasonCoverageGap}
}

// issueBudgetMixedLedger is seven entries after which every figure of the
// projection is another number, and the figures after each of them.
func issueBudgetMixedLedger() ([]budget.Entry, []issueBudgetLedgerFigures, [4]string) {
	attempts := [4]string{uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()}
	a, b, c, d := attempts[0], attempts[1], attempts[2], attempts[3]
	invocation := uuid.NewString()
	entries := []budget.Entry{
		issueBudgetReserveEntry(a, 500),
		issueBudgetUsageEntry(a, invocation, "model-one", budget.Cost{Basis: budget.CostReported, Ticks: 700, Currency: budget.CurrencyUSD}),
		issueBudgetReserveEntry(b, 300),
		issueBudgetReserveEntry(c, 400),
		issueBudgetHoldEntry(c, budget.HoldTimeout),
		issueBudgetUsageEntry(b, invocation, "model-two", budget.Cost{
			Basis: budget.CostEstimated, Ticks: 50, Currency: budget.CurrencyUSD,
			PriceVersion: "prices-1", PriceDigest: strings.Repeat("0", 64),
		}),
		issueBudgetUnsizedReserveEntry(d),
	}
	figures := []issueBudgetLedgerFigures{
		{},
		{ReservedTicks: 500},
		{ActualKnownTicks: 700, OverrunTicks: 200},
		{ActualKnownTicks: 700, ReservedTicks: 300, OverrunTicks: 200},
		{ActualKnownTicks: 700, ReservedTicks: 700, OverrunTicks: 200},
		{ActualKnownTicks: 700, ReservedTicks: 300, UnknownHoldTicks: 400, OverrunTicks: 200, UnknownCount: 1},
		{ActualKnownTicks: 700, EstimatedTicks: 50, ReservedTicks: 300, UnknownHoldTicks: 400, OverrunTicks: 200, UnknownCount: 2},
		{ActualKnownTicks: 700, EstimatedTicks: 50, ReservedTicks: 300, UnknownHoldTicks: 400, OverrunTicks: 200, UnknownCount: 3},
	}
	return entries, figures, attempts
}

// requireIssueBudgetLedger compares a ledger with the number of entries and
// the figures a test expects of it, column by column.
func requireIssueBudgetLedger(t *testing.T, ledger *budget.Ledger, entries int, want issueBudgetLedgerFigures, what string) {
	t.Helper()
	if ledger == nil {
		t.Fatalf("%s: no ledger", what)
	}
	p := ledger.Projection()
	got := issueBudgetLedgerFigures{
		ActualKnownTicks: int64(p.ActualKnown), EstimatedTicks: int64(p.Estimated), ReservedTicks: int64(p.Reserved),
		UnknownHoldTicks: int64(p.UnknownHold), OverrunTicks: int64(p.Overrun), UnknownCount: int64(len(p.Unknown)),
	}
	if ledger.Len() != entries || got != want {
		t.Fatalf("%s: ledger of %d entries with %+v, want %d entries with %+v", what, ledger.Len(), got, entries, want)
	}
}

// peek is what the cache holds for a root. It does not count as a use.
func (c *issueBudgetLedgerCache) peek(rootID pgtype.UUID) (revision int64, ledger *budget.Ledger, ok bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	element, ok := c.roots[rootID]
	if !ok {
		return 0, nil, false
	}
	cached := element.Value.(*issueBudgetCachedLedger)
	return cached.revision, cached.ledger, true
}

// requireCached fails unless the cache holds exactly this state for the root.
func (c *issueBudgetLedgerCache) requireCached(t *testing.T, rootID pgtype.UUID, revision int64, ledger *budget.Ledger, what string) {
	t.Helper()
	gotRevision, got, ok := c.peek(rootID)
	if !ok || gotRevision != revision || got != ledger {
		t.Fatalf("%s: cache holds (%v, revision %d, %p), want revision %d and %p", what, ok, gotRevision, got, revision, ledger)
	}
}

func (c *issueBudgetLedgerCache) requireNotCached(t *testing.T, rootID pgtype.UUID, what string) {
	t.Helper()
	if revision, _, ok := c.peek(rootID); ok {
		t.Fatalf("%s: cache holds revision %d of the root, want nothing", what, revision)
	}
}

// requireWithinLimit checks the cache's own account of what it holds.
func (c *issueBudgetLedgerCache) requireWithinLimit(t *testing.T, what string) {
	t.Helper()
	c.mu.Lock()
	defer c.mu.Unlock()
	sum := 0
	for element := c.recent.Front(); element != nil; element = element.Next() {
		cached := element.Value.(*issueBudgetCachedLedger)
		if int64(cached.ledger.Len()) != cached.revision {
			t.Fatalf("%s: a state of %d entries is cached as revision %d", what, cached.ledger.Len(), cached.revision)
		}
		sum += cached.ledger.Len() + 2
	}
	if sum != c.weight || c.weight > c.limit || len(c.roots) != c.recent.Len() {
		t.Fatalf("%s: cache accounts for weight %d and %d roots, holds weight %d in %d states, limit %d",
			what, c.weight, len(c.roots), sum, c.recent.Len(), c.limit)
	}
}

// issueBudgetLedgerRoot is a budget root whose entry rows a test writes.
type issueBudgetLedgerRoot struct {
	f  *issueBudgetFixture
	id pgtype.UUID
}

func (f *issueBudgetFixture) ledgerRoot(t *testing.T) *issueBudgetLedgerRoot {
	t.Helper()
	result := f.enable(t, f.issue(t, "Budget ledger root", ""))
	return &issueBudgetLedgerRoot{f: f, id: util.MustParseUUID(result.BudgetRootID)}
}

// row is the root row as committed.
func (r *issueBudgetLedgerRoot) row(t *testing.T) db.IssueBudgetRoot {
	t.Helper()
	root, err := r.f.q.GetIssueBudgetRoot(context.Background(), db.GetIssueBudgetRootParams{WorkspaceID: r.f.ws, ID: r.id})
	if err != nil {
		t.Fatalf("read budget root: %v", err)
	}
	return root
}

// entries is every entry row of the root as committed, in seq order.
func (r *issueBudgetLedgerRoot) entries(t *testing.T) []db.IssueBudgetEntry {
	t.Helper()
	rows, err := r.f.q.ListIssueBudgetEntries(context.Background(), db.ListIssueBudgetEntriesParams{WorkspaceID: r.f.ws, BudgetRootID: r.id})
	if err != nil {
		t.Fatalf("read entry rows: %v", err)
	}
	return rows
}

// snapshot opens a REPEATABLE READ read-only transaction, as a reader that
// wants one view of a root does, and returns it with the root row it sees.
// The transaction is rolled back when the test ends.
func (r *issueBudgetLedgerRoot) snapshot(t *testing.T) (pgx.Tx, db.IssueBudgetRoot) {
	t.Helper()
	ctx := context.Background()
	tx, err := r.f.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { tx.Rollback(ctx) })
	root, err := db.New(tx).GetIssueBudgetRoot(ctx, db.GetIssueBudgetRootParams{WorkspaceID: r.f.ws, ID: r.id})
	if err != nil {
		t.Fatalf("read budget root in a snapshot: %v", err)
	}
	return tx, root
}

// insertForm commits one entry row with the bytes given as its stored form.
// The digest is computed here, so the bytes may be anything the table takes.
// The receipt is a placeholder: no load reads it.
func (r *issueBudgetLedgerRoot) insertForm(t *testing.T, seq int64, entryID string, e budget.Entry, raw []byte, version int32) {
	t.Helper()
	row := db.InsertIssueBudgetEntryParams{
		WorkspaceID: r.f.ws, BudgetRootID: r.id, Seq: seq, EntryID: entryID, Kind: string(e.Kind),
		EntryJson: raw, EntryDigest: issueBudgetStoredDigest(raw), Receipt: []byte(`{"written_by":"test"}`),
		Source: "server", SchemaVersion: version,
	}
	if e.ReservationID != "" {
		row.AttemptID = util.MustParseUUID(e.ReservationID)
	}
	if e.Kind == budget.KindUsage {
		row.InvocationID = pgtype.Text{String: e.Usage.InvocationID, Valid: true}
		row.ScopeID = pgtype.Text{String: e.Usage.ScopeID, Valid: true}
	}
	if _, err := r.f.q.InsertIssueBudgetEntry(context.Background(), row); err != nil {
		t.Fatalf("insert entry row %d: %v", seq, err)
	}
}

// insert commits one entry row at seq with the stored form of e.
func (r *issueBudgetLedgerRoot) insert(t *testing.T, seq int64, e budget.Entry) {
	t.Helper()
	raw, digest, err := budget.EncodeEntry(e)
	if err != nil {
		t.Fatalf("encode entry %q: %v", e.ID, err)
	}
	if digest != issueBudgetStoredDigest(raw) {
		t.Fatalf("entry %q: EncodeEntry gives digest %s for bytes that digest to %s", e.ID, digest, issueBudgetStoredDigest(raw))
	}
	r.insertForm(t, seq, e.ID, e, raw, int32(budget.EntryFormVersion))
}

// set commits the ledger columns of the root row.
func (r *issueBudgetLedgerRoot) set(t *testing.T, revision int64, figures issueBudgetLedgerFigures) {
	t.Helper()
	r.f.fx.Exec(t, `UPDATE issue_budget_root
		SET ledger_revision=$2, actual_known_ticks=$3, estimated_ticks=$4, reserved_ticks=$5,
		    unknown_hold_ticks=$6, overrun_ticks=$7, unknown_count=$8
		WHERE id=$1`,
		util.UUIDToString(r.id), revision, figures.ActualKnownTicks, figures.EstimatedTicks, figures.ReservedTicks,
		figures.UnknownHoldTicks, figures.OverrunTicks, figures.UnknownCount)
}

// write commits entries behind the ones the root has and moves the root row
// to the last of them with the figures given.
func (r *issueBudgetLedgerRoot) write(t *testing.T, figures issueBudgetLedgerFigures, entries ...budget.Entry) {
	t.Helper()
	revision := r.row(t).LedgerRevision
	for _, e := range entries {
		revision++
		r.insert(t, revision, e)
	}
	r.set(t, revision, figures)
}

// locked runs fn in a transaction that holds the root lock, with the root row
// the lock returned and queries that record what fn sends, and then commits
// or rolls back. It returns what fn sent.
func (r *issueBudgetLedgerRoot) locked(t *testing.T, commit bool, fn func(q *db.Queries, root db.IssueBudgetRoot)) []string {
	t.Helper()
	ctx := context.Background()
	tx, err := r.f.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	root, err := db.New(tx).LockIssueBudgetRoot(ctx, db.LockIssueBudgetRootParams{WorkspaceID: r.f.ws, ID: r.id})
	if err != nil {
		t.Fatalf("lock budget root: %v", err)
	}
	sent := &issueBudgetLedgerStatements{DBTX: tx}
	fn(db.New(sent), root)
	if commit {
		if err := tx.Commit(ctx); err != nil {
			t.Fatalf("commit: %v", err)
		}
	}
	return sent.names()
}

// load is one load of the ledger by a writer that holds the root lock.
func (r *issueBudgetLedgerRoot) load(t *testing.T, cache *issueBudgetLedgerCache) (*budget.Ledger, []string, error) {
	t.Helper()
	var (
		ledger *budget.Ledger
		err    error
	)
	sent := r.locked(t, true, func(q *db.Queries, root db.IssueBudgetRoot) {
		ledger, err = cache.loadUnderRootLock(context.Background(), q, root)
	})
	return ledger, sent, err
}

// mustLoad is load that has to succeed.
func (r *issueBudgetLedgerRoot) mustLoad(t *testing.T, cache *issueBudgetLedgerCache, what string) (*budget.Ledger, []string) {
	t.Helper()
	ledger, sent, err := r.load(t, cache)
	if err != nil {
		t.Fatalf("%s: %v", what, err)
	}
	return ledger, sent
}

// loadWhile is load under ctx, with before run ahead of each statement the
// load sends: what another goroutine of the process does at that moment, or
// the end of ctx.
func (r *issueBudgetLedgerRoot) loadWhile(ctx context.Context, t *testing.T, cache *issueBudgetLedgerCache, before func(name string)) (*budget.Ledger, []string, error) {
	t.Helper()
	tx, err := r.f.pool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	root, err := db.New(tx).LockIssueBudgetRoot(context.Background(), db.LockIssueBudgetRootParams{WorkspaceID: r.f.ws, ID: r.id})
	if err != nil {
		t.Fatalf("lock budget root: %v", err)
	}
	sent := &issueBudgetLedgerStatements{DBTX: tx, before: before}
	ledger, loadErr := cache.loadUnderRootLock(ctx, db.New(sent), root)
	if err := tx.Commit(context.Background()); err != nil {
		t.Fatalf("commit: %v", err)
	}
	return ledger, sent.names(), loadErr
}

// read is one load by a reader that holds no lock: root is the row it read
// earlier, and db is what it reads the entries through.
func (r *issueBudgetLedgerRoot) read(t *testing.T, cache *issueBudgetLedgerCache, conn db.DBTX, root db.IssueBudgetRoot) (*budget.Ledger, []string, error) {
	t.Helper()
	sent := &issueBudgetLedgerStatements{DBTX: conn}
	ledger, err := cache.loadWithoutRootLock(context.Background(), db.New(sent), root)
	return ledger, sent.names(), err
}

func requireIssueBudgetStatements(t *testing.T, got []string, want []string, what string) {
	t.Helper()
	if len(got) == 0 && len(want) == 0 {
		return
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%s sent %v, want %v", what, got, want)
	}
}

// requireIssueBudgetLedgerUnavailable fails unless err says the ledger of the
// root is unavailable for this cause.
func requireIssueBudgetLedgerUnavailable(t *testing.T, err error, cause issueBudgetLedgerCause, what string) {
	t.Helper()
	var unavailable *issueBudgetLedgerUnavailableError
	if !errors.Is(err, ErrIssueBudgetLedgerUnavailable) || !errors.As(err, &unavailable) {
		t.Fatalf("%s: error %v, want the ledger to be unavailable", what, err)
	}
	if unavailable.Cause != cause {
		t.Fatalf("%s: unavailable for %q (%v), want %q", what, unavailable.Cause, err, cause)
	}
	// A failed load is not a refused entry: the caller that appends must be
	// able to tell the two apart.
	for _, refused := range []error{
		budget.ErrInvalidStoredForm, budget.ErrInvalidEntry, budget.ErrConflict, budget.ErrUnknownReservation,
	} {
		if errors.Is(err, refused) {
			t.Fatalf("%s: the load error passes for %v", what, refused)
		}
	}
}

// issueBudgetLedgerWarnings makes a cache log into memory and returns a
// function that hands over the warnings logged since it was last called.
func issueBudgetLedgerWarnings(cache *issueBudgetLedgerCache) func() []string {
	var logs bytes.Buffer
	cache.log = slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelWarn}))
	return func() []string {
		var lines []string
		for _, line := range strings.Split(logs.String(), "\n") {
			if line != "" {
				lines = append(lines, line)
			}
		}
		logs.Reset()
		return lines
	}
}

// requireIssueBudgetDroppedStateWarning fails unless warnings is the one
// warning of a cached state that was dropped, with its four fields: the root,
// the revision that was cached, the revision on the root row, and the check
// the cached state failed.
func requireIssueBudgetDroppedStateWarning(t *testing.T, warnings []string, rootID pgtype.UUID, cachedRevision, rootRevision int64, cause issueBudgetLedgerCause, what string) {
	t.Helper()
	if len(warnings) != 1 {
		t.Fatalf("%s: %d warnings, want the one of a dropped state: %q", what, len(warnings), warnings)
	}
	fields := strings.Fields(warnings[0])
	for _, want := range []string{
		"level=WARN",
		"budget_root_id=" + util.UUIDToString(rootID),
		fmt.Sprintf("cached_revision=%d", cachedRevision),
		fmt.Sprintf("root_revision=%d", rootRevision),
		"cause=" + string(cause),
	} {
		if !slices.Contains(fields, want) {
			t.Fatalf("%s: the warning lacks %q: %s", what, want, warnings[0])
		}
	}
}

func requireNoIssueBudgetLedgerWarning(t *testing.T, warnings []string, what string) {
	t.Helper()
	if len(warnings) != 0 {
		t.Fatalf("%s: logged %q, want no warning", what, warnings)
	}
}

func TestIssueBudgetLedgerLoadsTheWholeLedgerOnceAndThenTheTail(t *testing.T) {
	f := newIssueBudgetFixture(t)
	entries, figures, attempts := issueBudgetMixedLedger()
	cache := newIssueBudgetLedgerCache(issueBudgetLedgerCacheLimit)

	// A root without entries is loaded without a read.
	empty := f.ledgerRoot(t)
	ledger, sent := empty.mustLoad(t, cache, "load an empty ledger")
	requireIssueBudgetStatements(t, sent, nil, "the load of an empty ledger")
	requireIssueBudgetLedger(t, ledger, 0, issueBudgetLedgerFigures{}, "empty ledger")
	cache.requireCached(t, empty.id, 0, ledger, "after the load of an empty ledger")

	root := f.ledgerRoot(t)
	root.write(t, figures[5], entries[:5]...)

	// Nothing cached: every entry, in one read.
	first, sent := root.mustLoad(t, cache, "first load")
	requireIssueBudgetStatements(t, sent, []string{issueBudgetReadWholeLedger}, "the first load")
	requireIssueBudgetLedger(t, first, 5, figures[5], "first load")
	cache.requireCached(t, root.id, 5, first, "after the first load")

	// Cached at the root's revision: no read, and the same state.
	again, sent := root.mustLoad(t, cache, "second load")
	requireIssueBudgetStatements(t, sent, nil, "a load at the cached revision")
	if again != first {
		t.Fatal("a load at the cached revision built another state")
	}

	// Another process appended two entries: only those are read.
	root.write(t, figures[7], entries[5:]...)
	tail, sent := root.mustLoad(t, cache, "load after two more entries")
	requireIssueBudgetStatements(t, sent, []string{"ListIssueBudgetEntryFormsAfter after 5"}, "the load of a tail")
	requireIssueBudgetLedger(t, tail, 7, figures[7], "cached state with the tail")
	cache.requireCached(t, root.id, 7, tail, "after the tail load")
	if !tail.HasReservation(attempts[3]) || first.HasReservation(attempts[3]) {
		t.Fatal("the tail's reservation is missing from the new state or showed up in the old one")
	}
	// The state that was cached before is still what it was.
	requireIssueBudgetLedger(t, first, 5, figures[5], "the state cached before the tail load")

	// The cached state with its tail is the replay of all the rows.
	whole, sent := root.mustLoad(t, newIssueBudgetLedgerCache(issueBudgetLedgerCacheLimit), "replay of the whole ledger")
	requireIssueBudgetStatements(t, sent, []string{issueBudgetReadWholeLedger}, "a load with an empty cache")
	if !reflect.DeepEqual(whole.Projection(), tail.Projection()) {
		t.Fatalf("cache plus tail projects to %+v, the whole ledger to %+v", tail.Projection(), whole.Projection())
	}
	for _, attempt := range attempts {
		if got, _ := tail.Reservation(attempt); !reflect.DeepEqual(got, mustIssueBudgetReservation(t, whole, attempt)) {
			t.Fatalf("reservation %s differs between cache plus tail and the whole ledger", attempt)
		}
	}
	cache.requireWithinLimit(t, "at the end")
}

func mustIssueBudgetReservation(t *testing.T, ledger *budget.Ledger, attempt string) budget.ReservationProjection {
	t.Helper()
	reservation, ok := ledger.Reservation(attempt)
	if !ok {
		t.Fatalf("no reservation %s", attempt)
	}
	return reservation
}

// issueBudgetLedgerDamage is one way the rows of a root stop being the ledger
// its root row records. The root has three good entries when damage runs:
// a sized reservation of 500, an unsized one, and a hold on the first.
type issueBudgetLedgerDamage struct {
	name   string
	cause  issueBudgetLedgerCause
	damage func(t *testing.T, r *issueBudgetLedgerRoot, attempts [3]string)
	// repair, when set, undoes the damage.
	repair func(t *testing.T, r *issueBudgetLedgerRoot)
	// laterCommit says that a reader without the lock takes the damage for a
	// commit made after it read its root row, and loads the ledger of the row.
	laterCommit bool
}

func issueBudgetLedgerDamages() []issueBudgetLedgerDamage {
	good := issueBudgetLedgerFigures{UnknownHoldTicks: 500, UnknownCount: 2}
	// The figures after a further sized reservation of 300.
	withFourth := issueBudgetLedgerFigures{ReservedTicks: 300, UnknownHoldTicks: 500, UnknownCount: 2}
	fourth := func(attempts [3]string) budget.Entry { return issueBudgetReserveEntry(attempts[2], 300) }
	form := func(t *testing.T, e budget.Entry) []byte {
		t.Helper()
		raw, _, err := budget.EncodeEntry(e)
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	damages := []issueBudgetLedgerDamage{
		{
			name: "an entry in a form version this build does not read", cause: issueBudgetLedgerStoredForm,
			damage: func(t *testing.T, r *issueBudgetLedgerRoot, attempts [3]string) {
				e := fourth(attempts)
				r.insertForm(t, 4, e.ID, e, form(t, e), 2)
				r.set(t, 4, withFourth)
			},
			repair: func(t *testing.T, r *issueBudgetLedgerRoot) {
				r.f.fx.Exec(t, `UPDATE issue_budget_entry SET schema_version=1 WHERE budget_root_id=$1 AND seq=4`, util.UUIDToString(r.id))
			},
		},
		{
			name: "white space behind the stored form", cause: issueBudgetLedgerStoredForm,
			damage: func(t *testing.T, r *issueBudgetLedgerRoot, attempts [3]string) {
				e := fourth(attempts)
				r.insertForm(t, 4, e.ID, e, append(form(t, e), '\n'), 1)
				r.set(t, 4, withFourth)
			},
		},
		{
			name: "a stored form without most of its keys", cause: issueBudgetLedgerStoredForm,
			damage: func(t *testing.T, r *issueBudgetLedgerRoot, attempts [3]string) {
				e := fourth(attempts)
				r.insertForm(t, 4, e.ID, e, []byte(`{"id":"`+e.ID+`","kind":"reserve","reservation_id":"`+attempts[2]+`","reserved_ticks":"300"}`), 1)
				r.set(t, 4, withFourth)
			},
		},
		{
			name: "bytes that are no entry", cause: issueBudgetLedgerStoredForm,
			damage: func(t *testing.T, r *issueBudgetLedgerRoot, attempts [3]string) {
				r.insertForm(t, 4, "no-entry", budget.Entry{Kind: budget.KindGap}, []byte(`[]`), 1)
				r.set(t, 4, good)
			},
		},
		{
			name: "a hold on a reservation no entry opened", cause: issueBudgetLedgerReplay,
			damage: func(t *testing.T, r *issueBudgetLedgerRoot, attempts [3]string) {
				r.insert(t, 4, issueBudgetHoldEntry(attempts[2], budget.HoldTimeout))
				r.set(t, 4, good)
			},
		},
		{
			name: "an identity used again with other content", cause: issueBudgetLedgerReplay,
			damage: func(t *testing.T, r *issueBudgetLedgerRoot, attempts [3]string) {
				e := issueBudgetReserveEntry(attempts[0], 501)
				// The column has to differ: the table keeps one row per identity.
				r.insertForm(t, 4, "under-another-name", e, form(t, e), 1)
				r.set(t, 4, good)
			},
		},
		{
			name: "a row that repeats an entry", cause: issueBudgetLedgerReplay,
			damage: func(t *testing.T, r *issueBudgetLedgerRoot, attempts [3]string) {
				e := issueBudgetReserveEntry(attempts[0], 500)
				r.insertForm(t, 4, "under-another-name", e, form(t, e), 1)
				r.set(t, 4, good)
			},
		},
		{
			name: "a seq is missing", cause: issueBudgetLedgerSequence,
			damage: func(t *testing.T, r *issueBudgetLedgerRoot, attempts [3]string) {
				r.insert(t, 4, fourth(attempts))
				r.insert(t, 6, issueBudgetHoldEntry(attempts[2], budget.HoldTimeout))
				r.set(t, 5, issueBudgetLedgerFigures{UnknownHoldTicks: 800, UnknownCount: 3})
			},
		},
		{
			name: "the entries behind the third start one seq late", cause: issueBudgetLedgerSequence,
			damage: func(t *testing.T, r *issueBudgetLedgerRoot, attempts [3]string) {
				r.insert(t, 5, fourth(attempts))
				r.set(t, 4, withFourth)
			},
		},
		{
			name: "the entries end before the root's revision", cause: issueBudgetLedgerSequence,
			damage: func(t *testing.T, r *issueBudgetLedgerRoot, attempts [3]string) {
				r.insert(t, 4, fourth(attempts))
				r.set(t, 5, withFourth)
			},
		},
		{
			name: "an entry beyond the root's revision", cause: issueBudgetLedgerSequence,
			damage: func(t *testing.T, r *issueBudgetLedgerRoot, attempts [3]string) {
				r.insert(t, 4, fourth(attempts))
				r.insert(t, 5, issueBudgetHoldEntry(attempts[2], budget.HoldTimeout))
				r.set(t, 4, withFourth)
			},
			laterCommit: true,
		},
	}
	// The rows are right and one figure on the root row is off by one.
	for _, column := range []struct {
		name string
		off  func(f *issueBudgetLedgerFigures)
	}{
		{"actual_known_ticks", func(f *issueBudgetLedgerFigures) { f.ActualKnownTicks++ }},
		{"estimated_ticks", func(f *issueBudgetLedgerFigures) { f.EstimatedTicks++ }},
		{"reserved_ticks", func(f *issueBudgetLedgerFigures) { f.ReservedTicks-- }},
		{"unknown_hold_ticks", func(f *issueBudgetLedgerFigures) { f.UnknownHoldTicks++ }},
		{"overrun_ticks", func(f *issueBudgetLedgerFigures) { f.OverrunTicks++ }},
		{"unknown_count", func(f *issueBudgetLedgerFigures) { f.UnknownCount-- }},
	} {
		damages = append(damages, issueBudgetLedgerDamage{
			name: "the root row is off by one in " + column.name, cause: issueBudgetLedgerProjection,
			damage: func(t *testing.T, r *issueBudgetLedgerRoot, attempts [3]string) {
				figures := withFourth
				column.off(&figures)
				r.insert(t, 4, fourth(attempts))
				r.set(t, 4, figures)
			},
			repair: func(t *testing.T, r *issueBudgetLedgerRoot) { r.set(t, 4, withFourth) },
		})
	}
	return damages
}

// Design 10.5 with the last bullet of 10.4: each of the causes makes the
// ledger unavailable, and only a replay of every row says so. A load that went
// on from a cached state drops that state, says so once, and reads every row
// once more; what that replay finds is the answer. A root whose ledger is
// unavailable keeps nothing in the cache.
func TestIssueBudgetLedgerIsUnavailableForEachCause(t *testing.T) {
	f := newIssueBudgetFixture(t)
	good := issueBudgetLedgerFigures{UnknownHoldTicks: 500, UnknownCount: 2}
	readTailThenAll := []string{"ListIssueBudgetEntryFormsAfter after 3", issueBudgetReadWholeLedger}
	for _, damage := range issueBudgetLedgerDamages() {
		t.Run(damage.name, func(t *testing.T) {
			attempts := [3]string{uuid.NewString(), uuid.NewString(), uuid.NewString()}
			root := f.ledgerRoot(t)
			root.write(t, good,
				issueBudgetReserveEntry(attempts[0], 500),
				issueBudgetUnsizedReserveEntry(attempts[1]),
				issueBudgetHoldEntry(attempts[0], budget.HoldLostContact),
			)
			// Two caches hold the three good entries, one for a writer and one
			// for a reader to go on from, and one holds nothing.
			warm, cold := newIssueBudgetLedgerCache(issueBudgetLedgerCacheLimit), newIssueBudgetLedgerCache(issueBudgetLedgerCacheLimit)
			behind := newIssueBudgetLedgerCache(issueBudgetLedgerCacheLimit)
			warmWarnings, coldWarnings, behindWarnings := issueBudgetLedgerWarnings(warm), issueBudgetLedgerWarnings(cold), issueBudgetLedgerWarnings(behind)
			before, _ := root.mustLoad(t, warm, "load before the damage")
			requireIssueBudgetLedger(t, before, 3, good, "before the damage")
			used, _ := root.mustLoad(t, behind, "load before the damage, for the reader")

			damage.damage(t, root, attempts)
			rows := f.snapshot(t)
			revision := root.row(t).LedgerRevision

			// The cached state and its tail fail a check. The state goes and
			// every row is read once more: the damage is in the rows, so the
			// full replay finds it too.
			_, sent, err := root.load(t, warm)
			requireIssueBudgetLedgerUnavailable(t, err, damage.cause, "cached state plus tail")
			requireIssueBudgetStatements(t, sent, readTailThenAll, "the load with a cached state")
			warm.requireNotCached(t, root.id, "after a failed load with a cached state")
			requireIssueBudgetDroppedStateWarning(t, warmWarnings(), root.id, 3, revision, damage.cause, "the load with a cached state")

			_, sent, err = root.load(t, cold)
			requireIssueBudgetLedgerUnavailable(t, err, damage.cause, "replay of every row")
			requireIssueBudgetStatements(t, sent, []string{issueBudgetReadWholeLedger}, "the load without a cached state")
			cold.requireNotCached(t, root.id, "after a failed replay")
			requireNoIssueBudgetLedgerWarning(t, coldWarnings(), "the load without a cached state")

			// The cache that lost its state replays every row the next time,
			// once, and has nothing to say about a cached state.
			_, sent, err = root.load(t, warm)
			requireIssueBudgetLedgerUnavailable(t, err, damage.cause, "the load after the state was dropped")
			requireIssueBudgetStatements(t, sent, []string{issueBudgetReadWholeLedger}, "the load after the state was dropped")
			requireNoIssueBudgetLedgerWarning(t, warmWarnings(), "the load after the state was dropped")

			// A reader without the lock is told the same, unless what it sees
			// is what a commit after its root row looks like.
			read, _, err := root.read(t, newIssueBudgetLedgerCache(issueBudgetLedgerCacheLimit), f.pool, root.row(t))
			if damage.laterCommit {
				if err != nil {
					t.Fatalf("reader without the lock: %v", err)
				}
				requireIssueBudgetLedger(t, read, 4, issueBudgetLedgerFigures{ReservedTicks: 300, UnknownHoldTicks: 500, UnknownCount: 2}, "reader without the lock")
			} else {
				requireIssueBudgetLedgerUnavailable(t, err, damage.cause, "reader without the lock")
			}

			// And a reader that goes on from a cached state: the state it used
			// goes, every row is read once more, and the answer is the same.
			read, sent, err = root.read(t, behind, f.pool, root.row(t))
			if damage.laterCommit {
				if err != nil {
					t.Fatalf("reader with a cached state: %v", err)
				}
				requireIssueBudgetLedger(t, read, 4, issueBudgetLedgerFigures{ReservedTicks: 300, UnknownHoldTicks: 500, UnknownCount: 2}, "reader with a cached state")
				requireIssueBudgetStatements(t, sent, readTailThenAll[:1], "a reader with a cached state and a later commit")
				behind.requireCached(t, root.id, 3, used, "after a reader that saw a later commit")
				requireNoIssueBudgetLedgerWarning(t, behindWarnings(), "a reader that saw a later commit")
			} else {
				requireIssueBudgetLedgerUnavailable(t, err, damage.cause, "reader with a cached state")
				requireIssueBudgetStatements(t, sent, readTailThenAll, "a reader with a cached state")
				behind.requireNotCached(t, root.id, "after a reader that used the cached state failed")
				requireIssueBudgetDroppedStateWarning(t, behindWarnings(), root.id, 3, revision, damage.cause, "a reader with a cached state")
			}

			// No load wrote anything.
			f.requireUnchanged(t, rows, "a load of a damaged ledger")
			// The state handed out before the damage is what it was.
			requireIssueBudgetLedger(t, before, 3, good, "the state loaded before the damage")

			if damage.repair == nil {
				return
			}
			damage.repair(t, root)
			repaired, sent := root.mustLoad(t, warm, "load after the repair")
			requireIssueBudgetStatements(t, sent, []string{issueBudgetReadWholeLedger}, "the load after the repair")
			requireIssueBudgetLedger(t, repaired, 4, issueBudgetLedgerFigures{ReservedTicks: 300, UnknownHoldTicks: 500, UnknownCount: 2}, "after the repair")
			warm.requireCached(t, root.id, 4, repaired, "after the repair")
		})
	}
}

// A ledger whose first row is not seq 1 is unavailable too. The tail load has
// its own form of this case above.
func TestIssueBudgetLedgerMustStartAtTheFirstSeq(t *testing.T) {
	f := newIssueBudgetFixture(t)
	root := f.ledgerRoot(t)
	attempt := uuid.NewString()
	root.insert(t, 2, issueBudgetReserveEntry(attempt, 500))
	root.set(t, 1, issueBudgetLedgerFigures{ReservedTicks: 500})
	cache := newIssueBudgetLedgerCache(issueBudgetLedgerCacheLimit)
	_, _, err := root.load(t, cache)
	requireIssueBudgetLedgerUnavailable(t, err, issueBudgetLedgerSequence, "a ledger that starts at seq 2")
	cache.requireNotCached(t, root.id, "after the failed load")
}

// issueBudgetLedgerOf is a ledger of n entries: n unsized reservations.
func issueBudgetLedgerOf(t *testing.T, n int) *budget.Ledger {
	t.Helper()
	entries := make([]budget.Entry, n)
	for i := range entries {
		entries[i] = issueBudgetUnsizedReserveEntry(uuid.NewString())
	}
	ledger, _, err := budget.NewLedger().With(entries...)
	if err != nil {
		t.Fatal(err)
	}
	return ledger
}

func issueBudgetRootID() pgtype.UUID { return util.MustParseUUID(uuid.NewString()) }

// The cap of design 10.4 is a sum of weights, a root weighs its entries and
// two, and the roots used least recently make room.
func TestIssueBudgetLedgerCacheKeepsWithinItsLimit(t *testing.T) {
	if issueBudgetLedgerCacheLimit != 50_000 || issueBudgetLedgers.limit != 50_000 {
		t.Fatalf("the process cache holds a weight of %d (constant %d), want 50000", issueBudgetLedgers.limit, issueBudgetLedgerCacheLimit)
	}
	if got := issueBudgetLedgerWeight(budget.NewLedger()); got != 2 {
		t.Fatalf("an empty ledger weighs %d, want 2", got)
	}
	if got := issueBudgetLedgerWeight(issueBudgetLedgerOf(t, 7)); got != 9 {
		t.Fatalf("a ledger of 7 entries weighs %d, want 9", got)
	}

	cache := newIssueBudgetLedgerCache(12)
	present := func(what string, want map[pgtype.UUID]bool) {
		t.Helper()
		for id, held := range want {
			if _, _, ok := cache.peek(id); ok != held {
				t.Fatalf("%s: a root is cached=%v, want %v", what, ok, held)
			}
		}
		cache.requireWithinLimit(t, what)
	}
	a, b, c, d := issueBudgetRootID(), issueBudgetRootID(), issueBudgetRootID(), issueBudgetRootID()
	stateA, stateC := issueBudgetLedgerOf(t, 3), issueBudgetLedgerOf(t, 1)
	cache.publish(a, 3, stateA)
	cache.publish(b, 2, issueBudgetLedgerOf(t, 2))
	cache.publish(c, 1, stateC)
	present("three roots of weight 5, 4 and 3", map[pgtype.UUID]bool{a: true, b: true, c: true})
	if cache.weight != 12 {
		t.Fatalf("weight %d, want 12", cache.weight)
	}

	// Using a makes b the root used least recently.
	if got, revision, ok := cache.lookup(a, 3, true); !ok || got != stateA || revision != 3 {
		t.Fatalf("lookup of a: %p at %d, %v", got, revision, ok)
	}
	cache.publish(d, 2, issueBudgetLedgerOf(t, 2))
	present("a fourth root of weight 4", map[pgtype.UUID]bool{a: true, b: false, c: true, d: true})

	// A newer state of c weighs more: a, now the least recently used, goes.
	newerC := issueBudgetLedgerOf(t, 3)
	cache.publish(c, 3, newerC)
	present("c grown to weight 5", map[pgtype.UUID]bool{a: false, c: true, d: true})
	cache.requireCached(t, c, 3, newerC, "after the newer state of c")

	// A publication that arrives late replaces nothing.
	cache.publish(c, 1, stateC)
	cache.requireCached(t, c, 3, newerC, "after an older state of c arrived")
	cache.publish(c, 3, issueBudgetLedgerOf(t, 3))
	cache.requireCached(t, c, 3, newerC, "after another state of the same revision arrived")
	present("late publications", map[pgtype.UUID]bool{c: true, d: true})

	// A root that takes the whole cache leaves no room for another.
	e := issueBudgetRootID()
	stateE := issueBudgetLedgerOf(t, 10)
	cache.publish(e, 10, stateE)
	present("a root of weight 12", map[pgtype.UUID]bool{c: false, d: false, e: true})

	// One that weighs more than the cache holds is not kept, and costs no
	// other root its place.
	g := issueBudgetRootID()
	cache.publish(g, 11, issueBudgetLedgerOf(t, 11))
	present("a root of weight 13", map[pgtype.UUID]bool{e: true, g: false})
	cache.requireCached(t, e, 10, stateE, "after a root too large to cache")

	// A root that grows past the limit loses the state it had.
	cache.publish(e, 11, issueBudgetLedgerOf(t, 11))
	present("the cached root grown to weight 13", map[pgtype.UUID]bool{e: false})
	if cache.weight != 0 {
		t.Fatalf("an empty cache accounts for weight %d", cache.weight)
	}

	// A state ahead of the root row: a reader leaves it, a lock holder
	// removes it. One at or behind the root row is handed out.
	h := issueBudgetRootID()
	stateH := issueBudgetLedgerOf(t, 5)
	cache.publish(h, 5, stateH)
	if _, _, ok := cache.lookup(h, 3, false); ok {
		t.Fatal("a reader was handed a state ahead of its root row")
	}
	cache.requireCached(t, h, 5, stateH, "after a reader met a state ahead of its root row")
	for _, revision := range []int64{5, 7} {
		for _, holdsRootLock := range []bool{false, true} {
			if got, at, ok := cache.lookup(h, revision, holdsRootLock); !ok || got != stateH || at != 5 {
				t.Fatalf("lookup for a root row at %d (lock %v): %p at %d, %v", revision, holdsRootLock, got, at, ok)
			}
		}
	}
	if _, _, ok := cache.lookup(h, 3, true); ok {
		t.Fatal("a lock holder was handed a state ahead of its root row")
	}
	present("after a lock holder met a state ahead of its root row", map[pgtype.UUID]bool{h: false})

	// forget takes out the state a failed load used, or whatever is there.
	cache.publish(h, 5, stateH)
	cache.forget(h, issueBudgetLedgerOf(t, 5))
	cache.requireCached(t, h, 5, stateH, "after forgetting another state")
	cache.forget(h, stateH)
	present("after forgetting the state used", map[pgtype.UUID]bool{h: false})
	cache.publish(h, 5, stateH)
	cache.forget(h, nil)
	cache.forget(h, nil)
	present("after forgetting whatever is there", map[pgtype.UUID]bool{h: false})
}

// A root that weighs more than the cache holds is replayed in full on every
// load and reported each time; one that just fits is cached.
func TestIssueBudgetLedgerTooLargeToCacheIsReplayedOnEveryLoad(t *testing.T) {
	f := newIssueBudgetFixture(t)
	entries, figures, _ := issueBudgetMixedLedger()
	var logs bytes.Buffer
	cache := newIssueBudgetLedgerCache(6)
	cache.log = slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelWarn}))
	warnings := func() int { return strings.Count(logs.String(), "level=WARN") }

	// Four entries weigh six: cached, not reported.
	fits := f.ledgerRoot(t)
	fits.write(t, figures[4], entries[:4]...)
	kept, _ := fits.mustLoad(t, cache, "load a root that just fits")
	cache.requireCached(t, fits.id, 4, kept, "a root of weight 6 in a cache of 6")
	if warnings() != 0 {
		t.Fatalf("a root that fits was reported: %s", logs.String())
	}

	// Five entries weigh seven.
	large := f.ledgerRoot(t)
	large.write(t, figures[5], entries[:5]...)
	for load := 1; load <= 2; load++ {
		ledger, sent := large.mustLoad(t, cache, "load a root too large to cache")
		requireIssueBudgetLedger(t, ledger, 5, figures[5], "a root too large to cache")
		requireIssueBudgetStatements(t, sent, []string{issueBudgetReadWholeLedger}, fmt.Sprintf("load %d of a root too large to cache", load))
		cache.requireNotCached(t, large.id, "a root of weight 7 in a cache of 6")
		if warnings() != load {
			t.Fatalf("load %d: %d warnings, want %d: %s", load, warnings(), load, logs.String())
		}
	}
	for _, want := range []string{"budget_root_id=" + util.UUIDToString(large.id), "entries=5", "cache_limit=6"} {
		if !strings.Contains(logs.String(), want) {
			t.Fatalf("the warning lacks %q: %s", want, logs.String())
		}
	}
	// The root that fits kept its place.
	cache.requireCached(t, fits.id, 4, kept, "after loads of a root too large to cache")

	// A reader pays the same and is reported the same.
	ledger, sent, err := large.read(t, cache, f.pool, large.row(t))
	if err != nil {
		t.Fatal(err)
	}
	requireIssueBudgetLedger(t, ledger, 5, figures[5], "a reader of a root too large to cache")
	requireIssueBudgetStatements(t, sent, []string{issueBudgetReadWholeLedger}, "a reader of a root too large to cache")
	if warnings() != 3 {
		t.Fatalf("%d warnings after the reader, want 3", warnings())
	}

	// The cached root grows past the limit: its state goes when the append is
	// published, and from then on it is replayed in full.
	var appended issueBudgetLedgerAppend
	fits.locked(t, true, func(q *db.Queries, root db.IssueBudgetRoot) {
		ledger, err := cache.loadUnderRootLock(context.Background(), q, root)
		if err != nil {
			t.Fatal(err)
		}
		appended, err = cache.appendEntries(context.Background(), q, root, ledger, []issueBudgetLedgerEntry{{Entry: entries[4], Source: "server"}})
		if err != nil {
			t.Fatal(err)
		}
	})
	appended.Committed()
	cache.requireNotCached(t, fits.id, "a cached root that grew past the limit")
	ledger, sent = fits.mustLoad(t, cache, "load the root that grew past the limit")
	requireIssueBudgetLedger(t, ledger, 5, figures[5], "the root that grew past the limit")
	requireIssueBudgetStatements(t, sent, []string{issueBudgetReadWholeLedger}, "the load of the root that grew past the limit")
	if warnings() != 4 || !strings.Contains(logs.String(), "budget_root_id="+util.UUIDToString(fits.id)) {
		t.Fatalf("the grown root was not reported: %s", logs.String())
	}
	cache.requireWithinLimit(t, "at the end")
}

// A writer that finds the cache ahead of the root row it locked is looking at
// a database that was put back: the cached state goes, with the one warning
// that says so, and every row is read.
func TestIssueBudgetLedgerUnderTheRootLockDropsAStateAheadOfTheDatabase(t *testing.T) {
	f := newIssueBudgetFixture(t)
	entries, figures, _ := issueBudgetMixedLedger()
	cache := newIssueBudgetLedgerCache(issueBudgetLedgerCacheLimit)
	warnings := issueBudgetLedgerWarnings(cache)
	root := f.ledgerRoot(t)
	root.write(t, figures[4], entries[:4]...)
	ahead, _ := root.mustLoad(t, cache, "load four entries")
	cache.requireCached(t, root.id, 4, ahead, "before the database is put back")
	requireNoIssueBudgetLedgerWarning(t, warnings(), "the load of four entries")

	f.fx.Exec(t, `DELETE FROM issue_budget_entry WHERE budget_root_id=$1 AND seq > 2`, util.UUIDToString(root.id))
	root.set(t, 2, figures[2])

	ledger, sent := root.mustLoad(t, cache, "load after the database was put back")
	requireIssueBudgetStatements(t, sent, []string{issueBudgetReadWholeLedger}, "the load after the database was put back")
	requireIssueBudgetLedger(t, ledger, 2, figures[2], "after the database was put back")
	cache.requireCached(t, root.id, 2, ledger, "after the database was put back")
	requireIssueBudgetLedger(t, ahead, 4, figures[4], "the state that was ahead")
	// The drop is reported under a cause of its own, written out here: no
	// check of a load failed, and the name must not be one of theirs.
	requireIssueBudgetDroppedStateWarning(t, warnings(), root.id, 4, 2, "ahead_of_root", "the load after the database was put back")
}

// A reader holds no lock. It gets the ledger of the root row it read, from a
// cached state that is not ahead of that row or from the rows alone, and it
// puts nothing into the cache.
func TestIssueBudgetLedgerReaderWithoutTheRootLockPublishesNothing(t *testing.T) {
	f := newIssueBudgetFixture(t)
	entries, figures, _ := issueBudgetMixedLedger()
	cache := newIssueBudgetLedgerCache(issueBudgetLedgerCacheLimit)
	root := f.ledgerRoot(t)
	root.write(t, figures[3], entries[:3]...)
	read := func(conn db.DBTX, row db.IssueBudgetRoot, what string) (*budget.Ledger, []string) {
		t.Helper()
		ledger, sent, err := root.read(t, cache, conn, row)
		if err != nil {
			t.Fatalf("%s: %v", what, err)
		}
		return ledger, sent
	}

	// Nothing cached: the reader replays every row and leaves the cache empty.
	ledger, sent := read(f.pool, root.row(t), "reader with an empty cache")
	requireIssueBudgetLedger(t, ledger, 3, figures[3], "reader with an empty cache")
	requireIssueBudgetStatements(t, sent, []string{issueBudgetReadWholeLedger}, "a reader with an empty cache")
	cache.requireNotCached(t, root.id, "after a reader loaded")

	// A reader takes its snapshot while the root is at revision 3.
	snapshot, old := root.snapshot(t)
	if old.LedgerRevision != 3 {
		t.Fatalf("the snapshot sees revision %d, want 3", old.LedgerRevision)
	}

	// Two entries are committed and a writer loads: the cache is at 5.
	root.write(t, figures[5], entries[3:5]...)
	newer, _ := root.mustLoad(t, cache, "writer load at revision 5")
	cache.requireCached(t, root.id, 5, newer, "after the writer's load")

	// The reader's snapshot is older than the cache. It replays its snapshot
	// in full, and the cache stays exactly as it is.
	ledger, sent = read(snapshot, old, "reader with a snapshot older than the cache")
	requireIssueBudgetLedger(t, ledger, 3, figures[3], "reader with a snapshot older than the cache")
	requireIssueBudgetStatements(t, sent, []string{issueBudgetReadWholeLedger}, "a reader with a snapshot older than the cache")
	cache.requireCached(t, root.id, 5, newer, "after a reader with an older snapshot")

	// The same root row in the hands of a reader that sees rows as they are
	// committed: the two rows behind its root row are left out.
	ledger, sent = read(f.pool, old, "reader with a root row older than the rows")
	requireIssueBudgetLedger(t, ledger, 3, figures[3], "reader with a root row older than the rows")
	requireIssueBudgetStatements(t, sent, []string{issueBudgetReadWholeLedger}, "a reader with a root row older than the rows")
	cache.requireCached(t, root.id, 5, newer, "after a reader with an older root row")

	// A reader at the cached revision is handed the cached state.
	ledger, sent = read(f.pool, root.row(t), "reader at the cached revision")
	requireIssueBudgetStatements(t, sent, nil, "a reader at the cached revision")
	if ledger != newer {
		t.Fatal("a reader at the cached revision built another state")
	}

	// Another process appends two entries. A reader with the new root row
	// reads the two behind the cached state, and the cache keeps what it has.
	root.write(t, figures[7], entries[5:]...)
	ledger, sent = read(f.pool, root.row(t), "reader ahead of the cache")
	requireIssueBudgetLedger(t, ledger, 7, figures[7], "reader ahead of the cache")
	requireIssueBudgetStatements(t, sent, []string{"ListIssueBudgetEntryFormsAfter after 5"}, "a reader ahead of the cache")
	cache.requireCached(t, root.id, 5, newer, "after a reader read a tail")
	requireIssueBudgetLedger(t, newer, 5, figures[5], "the cached state a reader went on from")

	// The snapshot reads what it read before.
	ledger, _ = read(snapshot, old, "the snapshot again")
	requireIssueBudgetLedger(t, ledger, 3, figures[3], "the snapshot again")
	cache.requireWithinLimit(t, "at the end")
}

// A reader whose cached state fails a check drops that state if it is still
// the one cached, and no other. Whether its load fails is then decided by the
// rows alone, here with a ledger that is damaged.
func TestIssueBudgetLedgerFailedReaderDropsOnlyTheStateItUsed(t *testing.T) {
	f := newIssueBudgetFixture(t)
	ctx := context.Background()
	entries, figures, _ := issueBudgetMixedLedger()
	cache := newIssueBudgetLedgerCache(issueBudgetLedgerCacheLimit)
	warnings := issueBudgetLedgerWarnings(cache)

	// The reader's snapshot holds a root row that is off by one. The row is
	// repaired afterwards and the cache gets ahead of the snapshot.
	root := f.ledgerRoot(t)
	off := figures[3]
	off.ReservedTicks++
	root.write(t, off, entries[:3]...)
	snapshot, damaged := root.snapshot(t)
	root.set(t, 3, figures[3])
	root.write(t, figures[5], entries[3:5]...)
	newer, _ := root.mustLoad(t, cache, "writer load at revision 5")

	_, sent, err := root.read(t, cache, snapshot, damaged)
	requireIssueBudgetLedgerUnavailable(t, err, issueBudgetLedgerProjection, "reader with a damaged snapshot")
	requireIssueBudgetStatements(t, sent, []string{issueBudgetReadWholeLedger}, "a reader with a damaged snapshot")
	cache.requireCached(t, root.id, 5, newer, "after a reader behind the cache failed")
	requireNoIssueBudgetLedgerWarning(t, warnings(), "a reader that used no cached state")

	// The row behind the cached state is in a form this build does not read.
	// The reader went on from the cached state, so that state goes, and the
	// rows are read from the first: they say the same.
	raw, _, err := budget.EncodeEntry(entries[5])
	if err != nil {
		t.Fatal(err)
	}
	root.insertForm(t, 6, entries[5].ID, entries[5], raw, 2)
	root.set(t, 6, figures[6])
	_, sent, err = root.read(t, cache, f.pool, root.row(t))
	requireIssueBudgetLedgerUnavailable(t, err, issueBudgetLedgerStoredForm, "reader ahead of the cache")
	requireIssueBudgetStatements(t, sent, []string{"ListIssueBudgetEntryFormsAfter after 5", issueBudgetReadWholeLedger}, "a reader ahead of the cache")
	cache.requireNotCached(t, root.id, "after a reader that used the cached state failed")
	requireIssueBudgetDroppedStateWarning(t, warnings(), root.id, 5, 6, issueBudgetLedgerStoredForm, "a reader ahead of the cache")
	_, sent, err = root.read(t, cache, f.pool, root.row(t))
	requireIssueBudgetLedgerUnavailable(t, err, issueBudgetLedgerStoredForm, "the next reader")
	requireIssueBudgetStatements(t, sent, []string{issueBudgetReadWholeLedger}, "the next reader")
	requireNoIssueBudgetLedgerWarning(t, warnings(), "the next reader")

	// A reader goes on from the cached state at revision 3 with a root row
	// that is off by one. While it reads, the row is repaired and a writer
	// publishes the state at revision 4. The reader drops nothing: the state
	// it used is no longer the one cached. It reads every row, fails by the
	// row it has, and what the writer published stays.
	raced := f.ledgerRoot(t)
	raced.write(t, figures[3], entries[:3]...)
	used, _ := raced.mustLoad(t, cache, "load three entries")
	off = figures[4]
	off.OverrunTicks++
	raced.write(t, off, entries[3])
	stale := raced.row(t)
	var published *budget.Ledger
	conn := &issueBudgetLedgerStatements{DBTX: f.pool}
	conn.before = func(name string) {
		switch name {
		case "ListIssueBudgetEntryFormsAfter after 3":
			raced.set(t, 4, figures[4])
			published, _ = raced.mustLoad(t, cache, "writer load while the reader reads")
		case issueBudgetReadWholeLedger:
		default:
			t.Fatalf("the reader sent %s", name)
		}
	}
	_, err = cache.loadWithoutRootLock(ctx, db.New(conn), stale)
	requireIssueBudgetLedgerUnavailable(t, err, issueBudgetLedgerProjection, "reader overtaken by a writer")
	requireIssueBudgetStatements(t, conn.names(), []string{"ListIssueBudgetEntryFormsAfter after 3", issueBudgetReadWholeLedger}, "a reader overtaken by a writer")
	if published == nil || published == used {
		t.Fatal("the writer published nothing new while the reader read")
	}
	cache.requireCached(t, raced.id, 4, published, "after a reader that used an older state failed")
	// Nothing was dropped, so nothing is reported as dropped.
	requireNoIssueBudgetLedgerWarning(t, warnings(), "a reader whose state was replaced while it read")
}

// issueBudgetOtherLedger is five entries that share nothing with the mixed
// ledger, and the figures after each of them. They stand for the rows of a
// database that was put back, or of a root that was repaired, behind a process
// that still holds a state of the rows from before.
func issueBudgetOtherLedger() ([]budget.Entry, []issueBudgetLedgerFigures) {
	x, y, z, w := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	entries := []budget.Entry{
		issueBudgetReserveEntry(x, 111),
		issueBudgetReserveEntry(y, 222),
		issueBudgetReserveEntry(z, 333),
		issueBudgetHoldEntry(x, budget.HoldTimeout),
		issueBudgetReserveEntry(w, 444),
	}
	figures := []issueBudgetLedgerFigures{
		{},
		{ReservedTicks: 111},
		{ReservedTicks: 333},
		{ReservedTicks: 666},
		{ReservedTicks: 555, UnknownHoldTicks: 111, UnknownCount: 1},
		{ReservedTicks: 999, UnknownHoldTicks: 111, UnknownCount: 1},
	}
	return entries, figures
}

// replace commits other rows in the place of the ones the root has and moves
// the root row to them.
func (r *issueBudgetLedgerRoot) replace(t *testing.T, figures issueBudgetLedgerFigures, entries ...budget.Entry) {
	t.Helper()
	r.f.fx.Exec(t, `DELETE FROM issue_budget_entry WHERE budget_root_id=$1`, util.UUIDToString(r.id))
	r.set(t, 0, issueBudgetLedgerFigures{})
	r.write(t, figures, entries...)
}

// Design 10.4, last bullet: the replay is the definition and the cache only
// saves work. A cached state that does not go with the rows of its root is not
// a damaged ledger. It is dropped, with one warning, and the same load replays
// every row: what the rows say is the result.
func TestIssueBudgetLedgerCachedStateThatDisagreesGivesWayToTheRows(t *testing.T) {
	f := newIssueBudgetFixture(t)
	ctx := context.Background()
	mixed, mixedFigures, _ := issueBudgetMixedLedger()
	other, otherFigures := issueBudgetOtherLedger()
	const readTail = "ListIssueBudgetEntryFormsAfter after 3"

	// Each case starts with the first three entries of the mixed ledger in the
	// rows and in the cache, and then puts other rows in their place.
	cases := []struct {
		name    string
		rows    []budget.Entry
		figures issueBudgetLedgerFigures
		// cause is the check the cached state fails, and sent is what a load
		// that goes on from it reads.
		cause issueBudgetLedgerCause
		sent  []string
	}{
		{
			name: "a state of the root's revision with other content",
			rows: other[:3], figures: otherFigures[3],
			cause: issueBudgetLedgerProjection, sent: []string{issueBudgetReadWholeLedger},
		},
		{
			// The fourth row holds a reservation the cached state never saw.
			name: "an older state whose tail it cannot take",
			rows: other[:4], figures: otherFigures[4],
			cause: issueBudgetLedgerReplay, sent: []string{readTail, issueBudgetReadWholeLedger},
		},
		{
			// The fourth row is a reservation of its own: the cached state
			// takes it and comes to figures the root row does not hold.
			name: "an older state whose tail brings it to other figures",
			rows: []budget.Entry{other[0], other[1], other[2], other[4]}, figures: issueBudgetLedgerFigures{ReservedTicks: 1110},
			cause: issueBudgetLedgerProjection, sent: []string{readTail, issueBudgetReadWholeLedger},
		},
	}
	for _, c := range cases {
		revision := int64(len(c.rows))
		setup := func(t *testing.T) (*issueBudgetLedgerRoot, *issueBudgetLedgerCache, func() []string, *budget.Ledger) {
			t.Helper()
			root := f.ledgerRoot(t)
			root.write(t, mixedFigures[3], mixed[:3]...)
			cache := newIssueBudgetLedgerCache(issueBudgetLedgerCacheLimit)
			warnings := issueBudgetLedgerWarnings(cache)
			stale, _ := root.mustLoad(t, cache, "load before the rows change")
			cache.requireCached(t, root.id, 3, stale, "before the rows change")
			root.replace(t, c.figures, c.rows...)
			requireNoIssueBudgetLedgerWarning(t, warnings(), "the load before the rows change")
			return root, cache, warnings, stale
		}

		t.Run(c.name+"/under the root lock", func(t *testing.T) {
			root, cache, warnings, stale := setup(t)
			ledger, sent := root.mustLoad(t, cache, "load after the rows changed")
			requireIssueBudgetStatements(t, sent, c.sent, "the load after the rows changed")
			requireIssueBudgetLedger(t, ledger, len(c.rows), c.figures, "the ledger of the rows")
			// The state replayed from the rows is published, the other is gone.
			cache.requireCached(t, root.id, revision, ledger, "after the rows decided")
			requireIssueBudgetDroppedStateWarning(t, warnings(), root.id, 3, revision, c.cause, "the load after the rows changed")
			requireIssueBudgetLedger(t, stale, 3, mixedFigures[3], "the state that was dropped")

			// From here on the root loads like any other.
			again, sent := root.mustLoad(t, cache, "the next load")
			requireIssueBudgetStatements(t, sent, nil, "the next load")
			if again != ledger {
				t.Fatal("the next load built another state")
			}
			requireNoIssueBudgetLedgerWarning(t, warnings(), "the next load")
		})

		t.Run(c.name+"/a reader", func(t *testing.T) {
			root, cache, warnings, _ := setup(t)
			ledger, sent, err := root.read(t, cache, f.pool, root.row(t))
			if err != nil {
				t.Fatalf("reader after the rows changed: %v", err)
			}
			requireIssueBudgetStatements(t, sent, c.sent, "the reader after the rows changed")
			requireIssueBudgetLedger(t, ledger, len(c.rows), c.figures, "the ledger of the rows")
			// The state it used is gone, and a reader publishes nothing.
			cache.requireNotCached(t, root.id, "after a reader's cached state gave way")
			requireIssueBudgetDroppedStateWarning(t, warnings(), root.id, 3, revision, c.cause, "the reader after the rows changed")

			ledger, sent, err = root.read(t, cache, f.pool, root.row(t))
			if err != nil {
				t.Fatalf("the next reader: %v", err)
			}
			requireIssueBudgetStatements(t, sent, []string{issueBudgetReadWholeLedger}, "the next reader")
			requireIssueBudgetLedger(t, ledger, len(c.rows), c.figures, "the next reader")
			cache.requireNotCached(t, root.id, "after the next reader")
			requireNoIssueBudgetLedgerWarning(t, warnings(), "the next reader")
		})

		if c.sent[0] != readTail {
			// Without a read between the lookup and the drop there is no
			// moment at which another state could take the place.
			continue
		}
		t.Run(c.name+"/under the root lock, with a state published meanwhile", func(t *testing.T) {
			root, cache, warnings, _ := setup(t)
			// A state of the root's revision gets into the cache while the
			// writer reads the tail, as that of an append does whose commit is
			// confirmed only then. Under the root lock whatever is cached
			// goes: the state replayed from the rows is the one cached after.
			late := issueBudgetLedgerOf(t, len(c.rows))
			ledger, sent, err := root.loadWhile(ctx, t, cache, func(name string) {
				if name == readTail {
					cache.publish(root.id, revision, late)
					cache.requireCached(t, root.id, revision, late, "while the tail is read")
				}
			})
			if err != nil {
				t.Fatalf("load after the rows changed: %v", err)
			}
			requireIssueBudgetStatements(t, sent, c.sent, "the load after the rows changed")
			requireIssueBudgetLedger(t, ledger, len(c.rows), c.figures, "the ledger of the rows")
			cache.requireCached(t, root.id, revision, ledger, "after the rows decided")
			// The warning is about the state the load went on from.
			requireIssueBudgetDroppedStateWarning(t, warnings(), root.id, 3, revision, c.cause, "the load after the rows changed")
		})
		t.Run(c.name+"/a reader overtaken by a writer", func(t *testing.T) {
			root, cache, warnings, stale := setup(t)
			// While the reader reads the tail, a writer loads: it drops the
			// state, replays the rows and publishes. The reader then finds
			// another state in the place of the one it used and leaves it.
			var published *budget.Ledger
			conn := &issueBudgetLedgerStatements{DBTX: f.pool}
			conn.before = func(name string) {
				if name == readTail {
					published, _ = root.mustLoad(t, cache, "writer load while the reader reads")
				}
			}
			ledger, err := cache.loadWithoutRootLock(ctx, db.New(conn), root.row(t))
			if err != nil {
				t.Fatalf("reader overtaken by a writer: %v", err)
			}
			requireIssueBudgetStatements(t, conn.names(), c.sent, "the reader overtaken by a writer")
			requireIssueBudgetLedger(t, ledger, len(c.rows), c.figures, "the ledger of the rows")
			if published == nil || published == stale {
				t.Fatal("the writer published nothing new while the reader read")
			}
			cache.requireCached(t, root.id, revision, published, "after the reader")
			// One state was dropped, by the writer. The reader dropped none
			// and reports none.
			requireIssueBudgetDroppedStateWarning(t, warnings(), root.id, 3, revision, c.cause, "the writer and the reader together")
		})
	}

	// No load that finds the cache and the rows in agreement says anything,
	// and neither does a reader that meets a state ahead of its root row: that
	// state is a later commit and stays.
	t.Run("loads that agree with the cache say nothing", func(t *testing.T) {
		root := f.ledgerRoot(t)
		root.write(t, mixedFigures[3], mixed[:3]...)
		cache := newIssueBudgetLedgerCache(issueBudgetLedgerCacheLimit)
		warnings := issueBudgetLedgerWarnings(cache)
		root.mustLoad(t, cache, "load every row")
		snapshot, old := root.snapshot(t)
		root.write(t, mixedFigures[5], mixed[3:5]...)
		newer, sent := root.mustLoad(t, cache, "load the tail")
		requireIssueBudgetStatements(t, sent, []string{readTail}, "the load of the tail")
		if _, sent = root.mustLoad(t, cache, "load at the cached revision"); len(sent) != 0 {
			t.Fatalf("the load at the cached revision sent %v", sent)
		}
		if _, _, err := root.read(t, cache, f.pool, root.row(t)); err != nil {
			t.Fatalf("reader at the cached revision: %v", err)
		}
		ledger, sent, err := root.read(t, cache, snapshot, old)
		if err != nil {
			t.Fatalf("reader behind the cache: %v", err)
		}
		requireIssueBudgetLedger(t, ledger, 3, mixedFigures[3], "the reader behind the cache")
		requireIssueBudgetStatements(t, sent, []string{issueBudgetReadWholeLedger}, "the reader behind the cache")
		cache.requireCached(t, root.id, 5, newer, "after a reader behind the cache")
		requireNoIssueBudgetLedgerWarning(t, warnings(), "loads that agree with the cache")
	})
}

// Only the replay of every row says that a ledger is unavailable, and the
// error names what that replay found, not the check a cached state failed
// before it.
func TestIssueBudgetLedgerUnavailableIsDecidedByTheFullReplay(t *testing.T) {
	f := newIssueBudgetFixture(t)
	ctx := context.Background()
	mixed, mixedFigures, _ := issueBudgetMixedLedger()
	other, otherFigures := issueBudgetOtherLedger()
	readTailThenAll := []string{"ListIssueBudgetEntryFormsAfter after 3", issueBudgetReadWholeLedger}

	// The rows were replaced and the root row is off by one against them. The
	// cached state cannot take the fourth row, a hold on a reservation it
	// never saw. The rows by themselves replay, to other figures than the
	// root row holds.
	off := otherFigures[4]
	off.UnknownHoldTicks++
	setup := func(t *testing.T) (*issueBudgetLedgerRoot, *issueBudgetLedgerCache, func() []string) {
		t.Helper()
		root := f.ledgerRoot(t)
		root.write(t, mixedFigures[3], mixed[:3]...)
		cache := newIssueBudgetLedgerCache(issueBudgetLedgerCacheLimit)
		warnings := issueBudgetLedgerWarnings(cache)
		root.mustLoad(t, cache, "load before the rows change")
		root.replace(t, off, other[:4]...)
		return root, cache, warnings
	}
	// late is a state that arrives while the rows are replayed, as the one of
	// an append does whose commit is confirmed only then.
	late := issueBudgetLedgerOf(t, 4)

	t.Run("under the root lock", func(t *testing.T) {
		root, cache, warnings := setup(t)
		_, sent, err := root.load(t, cache)
		requireIssueBudgetLedgerUnavailable(t, err, issueBudgetLedgerProjection, "the load under the root lock")
		requireIssueBudgetStatements(t, sent, readTailThenAll, "the load under the root lock")
		cache.requireNotCached(t, root.id, "after the full replay failed")
		requireIssueBudgetDroppedStateWarning(t, warnings(), root.id, 3, 4, issueBudgetLedgerReplay, "the load under the root lock")
	})

	t.Run("a reader", func(t *testing.T) {
		root, cache, warnings := setup(t)
		_, sent, err := root.read(t, cache, f.pool, root.row(t))
		requireIssueBudgetLedgerUnavailable(t, err, issueBudgetLedgerProjection, "the reader")
		requireIssueBudgetStatements(t, sent, readTailThenAll, "the reader")
		cache.requireNotCached(t, root.id, "after the reader's full replay failed")
		requireIssueBudgetDroppedStateWarning(t, warnings(), root.id, 3, 4, issueBudgetLedgerReplay, "the reader")
	})

	// A root whose ledger is unavailable keeps nothing in the cache: a writer
	// under the root lock also drops a state that got there while it replayed.
	t.Run("under the root lock, with a state published during the replay", func(t *testing.T) {
		root, cache, warnings := setup(t)
		_, sent, err := root.loadWhile(ctx, t, cache, func(name string) {
			if name == issueBudgetReadWholeLedger {
				cache.publish(root.id, 4, late)
				cache.requireCached(t, root.id, 4, late, "while the rows are replayed")
			}
		})
		requireIssueBudgetLedgerUnavailable(t, err, issueBudgetLedgerProjection, "the load under the root lock")
		requireIssueBudgetStatements(t, sent, readTailThenAll, "the load under the root lock")
		cache.requireNotCached(t, root.id, "after the full replay failed")
		// The one warning is that of the state the load went on from.
		requireIssueBudgetDroppedStateWarning(t, warnings(), root.id, 3, 4, issueBudgetLedgerReplay, "the load under the root lock")
	})

	// A reader drops the state it used and no other: what was published while
	// it replayed is a later commit for all it knows.
	t.Run("a reader, with a state published during the replay", func(t *testing.T) {
		root, cache, warnings := setup(t)
		conn := &issueBudgetLedgerStatements{DBTX: f.pool}
		conn.before = func(name string) {
			if name == issueBudgetReadWholeLedger {
				cache.requireNotCached(t, root.id, "while the reader replays the rows")
				cache.publish(root.id, 4, late)
			}
		}
		_, err := cache.loadWithoutRootLock(ctx, db.New(conn), root.row(t))
		requireIssueBudgetLedgerUnavailable(t, err, issueBudgetLedgerProjection, "the reader")
		requireIssueBudgetStatements(t, conn.names(), readTailThenAll, "the reader")
		cache.requireCached(t, root.id, 4, late, "after the reader's full replay failed")
		requireIssueBudgetDroppedStateWarning(t, warnings(), root.id, 3, 4, issueBudgetLedgerReplay, "the reader")
	})
}

// A read that fails is not a ledger that is unavailable. The error is that of
// the read, and the cache is left as it is: whether the rows and the cached
// state go together is not known.
func TestIssueBudgetLedgerFailedReadIsNotAnUnavailableLedger(t *testing.T) {
	f := newIssueBudgetFixture(t)
	mixed, mixedFigures, _ := issueBudgetMixedLedger()
	other, otherFigures := issueBudgetOtherLedger()
	const readTail = "ListIssueBudgetEntryFormsAfter after 3"

	// failing is a context that ends just before the statement named is sent,
	// and the hook that ends it, after doing what else the test asks for.
	failing := func(t *testing.T, name string, also func()) (context.Context, func(string)) {
		ctx, cancel := context.WithCancel(context.Background())
		t.Cleanup(cancel)
		return ctx, func(sent string) {
			if sent != name {
				return
			}
			if also != nil {
				also()
			}
			cancel()
		}
	}
	requireFailedRead := func(t *testing.T, err error, what string) {
		t.Helper()
		if !errors.Is(err, context.Canceled) || errors.Is(err, ErrIssueBudgetLedgerUnavailable) {
			t.Fatalf("%s: %v, want the error of the read and no unavailable ledger", what, err)
		}
	}

	t.Run("the read of the tail", func(t *testing.T) {
		root := f.ledgerRoot(t)
		root.write(t, mixedFigures[3], mixed[:3]...)
		cache := newIssueBudgetLedgerCache(issueBudgetLedgerCacheLimit)
		warnings := issueBudgetLedgerWarnings(cache)
		state, _ := root.mustLoad(t, cache, "load before the tail is written")
		root.write(t, mixedFigures[5], mixed[3:5]...)

		ctx, before := failing(t, readTail, nil)
		_, sent, err := root.loadWhile(ctx, t, cache, before)
		requireFailedRead(t, err, "the load under the root lock")
		requireIssueBudgetStatements(t, sent, []string{readTail}, "the load under the root lock")
		cache.requireCached(t, root.id, 3, state, "after the failed read under the root lock")

		reader := &issueBudgetLedgerStatements{DBTX: f.pool}
		ctx, reader.before = failing(t, readTail, nil)
		_, err = cache.loadWithoutRootLock(ctx, db.New(reader), root.row(t))
		requireFailedRead(t, err, "the reader")
		requireIssueBudgetStatements(t, reader.names(), []string{readTail}, "the reader")
		cache.requireCached(t, root.id, 3, state, "after the reader's failed read")
		requireNoIssueBudgetLedgerWarning(t, warnings(), "the failed reads")

		// The next load goes on from the state as if nothing had happened.
		ledger, sent := root.mustLoad(t, cache, "the next load")
		requireIssueBudgetStatements(t, sent, []string{readTail}, "the next load")
		requireIssueBudgetLedger(t, ledger, 5, mixedFigures[5], "the next load")
	})

	t.Run("the read of every row, with a state published meanwhile", func(t *testing.T) {
		root := f.ledgerRoot(t)
		root.write(t, mixedFigures[3], mixed[:3]...)
		cache := newIssueBudgetLedgerCache(issueBudgetLedgerCacheLimit)
		warnings := issueBudgetLedgerWarnings(cache)
		late := issueBudgetLedgerOf(t, 3)
		ctx, before := failing(t, issueBudgetReadWholeLedger, func() { cache.publish(root.id, 3, late) })
		_, sent, err := root.loadWhile(ctx, t, cache, before)
		requireFailedRead(t, err, "the load under the root lock")
		requireIssueBudgetStatements(t, sent, []string{issueBudgetReadWholeLedger}, "the load under the root lock")
		cache.requireCached(t, root.id, 3, late, "after the failed read under the root lock")
		requireNoIssueBudgetLedgerWarning(t, warnings(), "the failed read")
	})

	// A state that failed its check is dropped and reported whatever comes
	// of the read behind it. The read that failed says nothing about the rows.
	t.Run("the read of every row, after a cached state was dropped", func(t *testing.T) {
		root := f.ledgerRoot(t)
		root.write(t, mixedFigures[3], mixed[:3]...)
		cache := newIssueBudgetLedgerCache(issueBudgetLedgerCacheLimit)
		warnings := issueBudgetLedgerWarnings(cache)
		root.mustLoad(t, cache, "load before the rows change")
		root.replace(t, otherFigures[4], other[:4]...)

		ctx, before := failing(t, issueBudgetReadWholeLedger, nil)
		_, sent, err := root.loadWhile(ctx, t, cache, before)
		requireFailedRead(t, err, "the load under the root lock")
		requireIssueBudgetStatements(t, sent, []string{readTail, issueBudgetReadWholeLedger}, "the load under the root lock")
		cache.requireNotCached(t, root.id, "after the cached state was dropped")
		requireIssueBudgetDroppedStateWarning(t, warnings(), root.id, 3, 4, issueBudgetLedgerReplay, "the load under the root lock")

		ledger, sent := root.mustLoad(t, cache, "the next load")
		requireIssueBudgetStatements(t, sent, []string{issueBudgetReadWholeLedger}, "the next load")
		requireIssueBudgetLedger(t, ledger, 4, otherFigures[4], "the next load")
		cache.requireCached(t, root.id, 4, ledger, "after the next load")
		requireNoIssueBudgetLedgerWarning(t, warnings(), "the next load")
	})
}

// A writer appends under the root lock: one row for each entry the ledger did
// not hold, the root row moved to the last of them, and the cache told only
// when the commit is known.
func TestIssueBudgetLedgerAppendWritesRowsAndIsPublishedOnceCommitted(t *testing.T) {
	f := newIssueBudgetFixture(t)
	ctx := context.Background()
	entries, figures, attempts := issueBudgetMixedLedger()
	cache := newIssueBudgetLedgerCache(issueBudgetLedgerCacheLimit)
	root := f.ledgerRoot(t)
	root.write(t, figures[3], entries[:3]...)
	base, _ := root.mustLoad(t, cache, "load three entries")
	before := root.row(t)
	rows := f.snapshot(t)
	operation := util.MustParseUUID(uuid.NewString())
	appendSent := []string{"InsertIssueBudgetEntry", "InsertIssueBudgetEntry", "UpdateIssueBudgetRootLedger"}

	// An append whose transaction is rolled back.
	var undone issueBudgetLedgerAppend
	sent := root.locked(t, false, func(q *db.Queries, locked db.IssueBudgetRoot) {
		ledger, err := cache.loadUnderRootLock(ctx, q, locked)
		if err != nil {
			t.Fatal(err)
		}
		if ledger != base {
			t.Fatal("the writer was not handed the cached state")
		}
		undone, err = cache.appendEntries(ctx, q, locked, ledger, []issueBudgetLedgerEntry{
			{Entry: entries[3], Source: "server", OperationID: operation},
			{Entry: entries[4], Source: "server", OperationID: operation},
		})
		if err != nil {
			t.Fatal(err)
		}
		cache.requireCached(t, root.id, 3, base, "inside the appending transaction")
	})
	requireIssueBudgetStatements(t, sent, appendSent, "the append that is rolled back")
	requireIssueBudgetLedger(t, undone.Ledger, 5, figures[5], "the ledger of the append that was rolled back")
	if undone.Root.LedgerRevision != 5 {
		t.Fatalf("the append left the root at revision %d in its transaction, want 5", undone.Root.LedgerRevision)
	}
	// It is let go without Committed: the cache holds what is committed.
	cache.requireCached(t, root.id, 3, base, "after the rollback")
	f.requireUnchanged(t, rows, "an append that was rolled back")
	again, sent := root.mustLoad(t, cache, "load after the rollback")
	requireIssueBudgetStatements(t, sent, nil, "the load after the rollback")
	if again != base {
		t.Fatal("the load after a rollback built another state")
	}
	requireIssueBudgetLedger(t, base, 3, figures[3], "the cached state after the rollback")

	// The same two entries with one the ledger holds between them, committed.
	var appended issueBudgetLedgerAppend
	sent = root.locked(t, true, func(q *db.Queries, locked db.IssueBudgetRoot) {
		ledger, err := cache.loadUnderRootLock(ctx, q, locked)
		if err != nil {
			t.Fatal(err)
		}
		appended, err = cache.appendEntries(ctx, q, locked, ledger, []issueBudgetLedgerEntry{
			{Entry: entries[3], Source: "server", OperationID: operation},
			{Entry: entries[0], Source: "server", OperationID: operation},
			{Entry: entries[4], Source: "claim", OperationID: operation},
		})
		if err != nil {
			t.Fatal(err)
		}
	})
	requireIssueBudgetStatements(t, sent, appendSent, "the append of two new entries and a held one")
	cache.requireCached(t, root.id, 3, base, "after the commit, before it is reported")
	appended.Committed()
	cache.requireCached(t, root.id, 5, appended.Ledger, "after the commit was reported")
	requireIssueBudgetLedger(t, appended.Ledger, 5, figures[5], "the ledger of the append")
	requireIssueBudgetLedger(t, base, 3, figures[3], "the state the append started from")

	a, c := attempts[0], attempts[2]
	receipts := []budget.Receipt{
		{EntryID: "reserve/" + c, Kind: "reserve", Disposition: "accepted"},
		{EntryID: "reserve/" + a, Kind: "reserve", Disposition: "accepted", Replayed: true},
		{EntryID: "hold/" + c + "/timeout", Kind: "hold", Disposition: "accepted", Reason: "timeout"},
	}
	if !reflect.DeepEqual(appended.Receipts, receipts) {
		t.Fatalf("receipts %+v, want %+v", appended.Receipts, receipts)
	}

	// The two rows, column by column.
	stored := root.entries(t)
	if len(stored) != 5 {
		t.Fatalf("%d entry rows, want 5", len(stored))
	}
	for i, want := range []struct {
		seq     int64
		entryID string
		kind    string
		entry   budget.Entry
		source  string
		receipt budget.Receipt
	}{
		{4, "reserve/" + c, "reserve", entries[3], "server", receipts[0]},
		{5, "hold/" + c + "/timeout", "hold", entries[4], "claim", receipts[2]},
	} {
		got := stored[3+i]
		raw, err := json.Marshal(want.entry)
		if err != nil {
			t.Fatal(err)
		}
		var receipt budget.Receipt
		if err := json.Unmarshal(got.Receipt, &receipt); err != nil {
			t.Fatalf("row %d: receipt %s: %v", want.seq, got.Receipt, err)
		}
		if receipt != want.receipt {
			t.Fatalf("row %d: receipt %+v, want %+v", want.seq, receipt, want.receipt)
		}
		if !got.CreatedAt.Valid {
			t.Fatalf("row %d has no created_at", want.seq)
		}
		got.Receipt, got.CreatedAt = nil, pgtype.Timestamptz{}
		wantRow := db.IssueBudgetEntry{
			WorkspaceID: f.ws, BudgetRootID: root.id, Seq: want.seq, EntryID: want.entryID, Kind: want.kind,
			AttemptID: util.MustParseUUID(c), EntryJson: raw, EntryDigest: issueBudgetStoredDigest(raw),
			Source: want.source, SchemaVersion: 1, OperationID: operation,
		}
		if !reflect.DeepEqual(got, wantRow) {
			t.Fatalf("row %d:\n got %+v\nwant %+v", want.seq, got, wantRow)
		}
		back, err := budget.DecodeEntry(1, raw)
		if err != nil || back != want.entry {
			t.Fatalf("row %d: the stored form decodes to %+v (%v), want %+v", want.seq, back, err, want.entry)
		}
	}

	// The root row: the ledger columns moved, and nothing else but updated_at.
	after := root.row(t)
	if !reflect.DeepEqual(appended.Root, after) {
		t.Fatalf("the append returned root %+v, committed is %+v", appended.Root, after)
	}
	wantRoot := before
	wantRoot.LedgerRevision = 5
	wantRoot.ActualKnownTicks, wantRoot.EstimatedTicks, wantRoot.ReservedTicks = 700, 0, 300
	wantRoot.UnknownHoldTicks, wantRoot.OverrunTicks, wantRoot.UnknownCount = 400, 200, 1
	wantRoot.UpdatedAt = after.UpdatedAt
	if !reflect.DeepEqual(after, wantRoot) {
		t.Fatalf("root row after the append:\n got %+v\nwant %+v", after, wantRoot)
	}
	if !after.UpdatedAt.Time.After(before.UpdatedAt.Time) {
		t.Fatal("the append left updated_at where it was")
	}

	// The cached state is what the rows replay to, and a load reads nothing.
	whole, _ := root.mustLoad(t, newIssueBudgetLedgerCache(issueBudgetLedgerCacheLimit), "replay after the append")
	if !reflect.DeepEqual(whole.Projection(), appended.Ledger.Projection()) {
		t.Fatalf("the rows replay to %+v, the append published %+v", whole.Projection(), appended.Ledger.Projection())
	}
	again, sent = root.mustLoad(t, cache, "load after the append")
	requireIssueBudgetStatements(t, sent, nil, "the load after the append")
	if again != appended.Ledger {
		t.Fatal("the load after the append built another state")
	}

	// Entries the ledger holds already: nothing is sent, and nothing changes.
	rows = f.snapshot(t)
	var held issueBudgetLedgerAppend
	sent = root.locked(t, true, func(q *db.Queries, locked db.IssueBudgetRoot) {
		ledger, err := cache.loadUnderRootLock(ctx, q, locked)
		if err != nil {
			t.Fatal(err)
		}
		held, err = cache.appendEntries(ctx, q, locked, ledger, []issueBudgetLedgerEntry{
			{Entry: entries[4], Source: "server"}, {Entry: entries[1], Source: "server"},
		})
		if err != nil {
			t.Fatal(err)
		}
		if held.Ledger != ledger || !reflect.DeepEqual(held.Root, locked) {
			t.Fatal("an append of entries the ledger holds returned another ledger or root row")
		}
	})
	requireIssueBudgetStatements(t, sent, nil, "an append of entries the ledger holds")
	if len(held.Receipts) != 2 || !held.Receipts[0].Replayed || !held.Receipts[1].Replayed ||
		held.Receipts[0].EntryID != entries[4].ID || held.Receipts[1].EntryID != entries[1].ID {
		t.Fatalf("receipts of entries the ledger holds: %+v", held.Receipts)
	}
	held.Committed()
	cache.requireCached(t, root.id, 5, appended.Ledger, "after an append of nothing new")
	f.requireUnchanged(t, rows, "an append of entries the ledger holds")

	// A transaction that appends twice goes on from its first append.
	var second issueBudgetLedgerAppend
	sent = root.locked(t, true, func(q *db.Queries, locked db.IssueBudgetRoot) {
		ledger, err := cache.loadUnderRootLock(ctx, q, locked)
		if err != nil {
			t.Fatal(err)
		}
		first, err := cache.appendEntries(ctx, q, locked, ledger, []issueBudgetLedgerEntry{{Entry: entries[5], Source: "server"}})
		if err != nil {
			t.Fatal(err)
		}
		second, err = cache.appendEntries(ctx, q, first.Root, first.Ledger, []issueBudgetLedgerEntry{{Entry: entries[6], Source: "server"}})
		if err != nil {
			t.Fatal(err)
		}
	})
	requireIssueBudgetStatements(t, sent, []string{
		"InsertIssueBudgetEntry", "UpdateIssueBudgetRootLedger", "InsertIssueBudgetEntry", "UpdateIssueBudgetRootLedger",
	}, "two appends in one transaction")
	second.Committed()
	requireIssueBudgetLedger(t, second.Ledger, 7, figures[7], "the ledger of the second append")
	cache.requireCached(t, root.id, 7, second.Ledger, "after two appends in one transaction")
	if last := root.row(t); last.LedgerRevision != 7 || !reflect.DeepEqual(second.Root, last) {
		t.Fatalf("root row after two appends: %+v, the append returned %+v", last, second.Root)
	}
	whole, _ = root.mustLoad(t, newIssueBudgetLedgerCache(issueBudgetLedgerCacheLimit), "replay after two appends")
	requireIssueBudgetLedger(t, whole, 7, figures[7], "the replay after two appends")
	usage := root.entries(t)[5]
	if usage.Kind != "usage" || usage.Seq != 6 || util.UUIDToString(usage.AttemptID) != attempts[1] ||
		usage.InvocationID != (pgtype.Text{String: entries[5].Usage.InvocationID, Valid: true}) ||
		usage.ScopeID != (pgtype.Text{String: attempts[1] + "/" + entries[5].Usage.InvocationID, Valid: true}) {
		t.Fatalf("the usage row: %+v", usage)
	}
	cache.requireWithinLimit(t, "at the end")
}

// An append that is refused sends nothing: every entry is encoded and the
// ledger has taken the whole batch before the first row is written.
func TestIssueBudgetLedgerAppendThatIsRefusedWritesNothing(t *testing.T) {
	f := newIssueBudgetFixture(t)
	ctx := context.Background()
	entries, figures, attempts := issueBudgetMixedLedger()
	cache := newIssueBudgetLedgerCache(issueBudgetLedgerCacheLimit)
	root := f.ledgerRoot(t)
	root.write(t, figures[3], entries[:3]...)
	base, _ := root.mustLoad(t, cache, "load three entries")
	rows := f.snapshot(t)

	good := issueBudgetLedgerEntry{Entry: entries[3], Source: "server"}
	stranger := uuid.NewString()
	long := issueBudgetReserveEntry(stranger, 1)
	long.ID = strings.Repeat("x", 201)
	mixed := issueBudgetReserveEntry(stranger, 1)
	mixed.HoldReason = budget.HoldTimeout
	// One byte less and the ledger would take it: the attempt has a
	// reservation, and a gap needs no more.
	over := issueBudgetPadToForm(t, issueBudgetGapEntry(attempts[0]), issueBudgetLedgerEntryMaxBytes+1)
	for _, tc := range []struct {
		name string
		// ledger is what the append is given; nil is the ledger of the root.
		ledger *budget.Ledger
		behind budget.Entry
		is     error
		text   string
	}{
		{name: "an identity of 201 bytes", behind: long, is: budget.ErrInvalidEntry, text: "201 bytes"},
		{
			name: "a stored form one byte over what a server may write", behind: over, is: budget.ErrInvalidEntry,
			text: fmt.Sprintf("budget entry %q: a stored form of 2049 bytes, the limit for an entry a server writes is 2048", over.ID),
		},
		{name: "fields of another kind", behind: mixed, is: budget.ErrInvalidEntry, text: "fields of another kind"},
		{name: "a reservation in upper case", behind: issueBudgetReserveEntry(strings.ToUpper(stranger), 1), text: "not the canonical text of an attempt id"},
		{name: "a reservation that is no attempt id", behind: issueBudgetReserveEntry("attempt-1", 1), text: "not the canonical text of an attempt id"},
		{name: "a hold on a reservation the ledger does not hold", behind: issueBudgetHoldEntry(stranger, budget.HoldTimeout), is: budget.ErrUnknownReservation},
		{name: "an identity the ledger holds with other content", behind: issueBudgetReserveEntry(attempts[0], 501), is: budget.ErrConflict},
		{name: "a ledger that is not the ledger of the root row", ledger: budget.NewLedger(), behind: issueBudgetUnsizedReserveEntry(stranger), text: "revision 5 is due"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var err error
			sent := root.locked(t, false, func(q *db.Queries, locked db.IssueBudgetRoot) {
				ledger, loadErr := cache.loadUnderRootLock(ctx, q, locked)
				if loadErr != nil {
					t.Fatal(loadErr)
				}
				if tc.ledger != nil {
					ledger = tc.ledger
				}
				var refused issueBudgetLedgerAppend
				refused, err = cache.appendEntries(ctx, q, locked, ledger, []issueBudgetLedgerEntry{good, {Entry: tc.behind, Source: "server"}})
				if !reflect.DeepEqual(refused, issueBudgetLedgerAppend{}) {
					t.Fatalf("a refused append returned %+v", refused)
				}
			})
			if err == nil {
				t.Fatal("the append was accepted")
			}
			if tc.is != nil && !errors.Is(err, tc.is) {
				t.Fatalf("error %v, want %v", err, tc.is)
			}
			if !strings.Contains(err.Error(), tc.text) {
				t.Fatalf("error %q, want it to say %q", err, tc.text)
			}
			if errors.Is(err, ErrIssueBudgetLedgerUnavailable) {
				t.Fatalf("a refused entry passes for an unavailable ledger: %v", err)
			}
			requireIssueBudgetStatements(t, sent, nil, "a refused append")
			cache.requireCached(t, root.id, 3, base, "after a refused append")
			f.requireUnchanged(t, rows, "a refused append")
		})
	}

	// The root row given is not the row in the database. The row of the entry
	// can be written, the root does not move, and the append fails.
	ahead, _, err := base.With(entries[3], entries[4])
	if err != nil {
		t.Fatal(err)
	}
	sent := root.locked(t, false, func(q *db.Queries, locked db.IssueBudgetRoot) {
		locked.LedgerRevision = 5
		_, err = cache.appendEntries(ctx, q, locked, ahead, []issueBudgetLedgerEntry{{Entry: entries[5], Source: "server"}})
	})
	if !errors.Is(err, pgx.ErrNoRows) || !strings.Contains(err.Error(), "move budget root to revision 6") {
		t.Fatalf("append from a root row that is not the stored one: %v", err)
	}
	requireIssueBudgetStatements(t, sent, []string{"InsertIssueBudgetEntry", "UpdateIssueBudgetRootLedger"}, "an append from another root row")
	cache.requireCached(t, root.id, 3, base, "after an append from another root row")
	f.requireUnchanged(t, rows, "an append from another root row")
}

// An entry a server writes has a stored form of at most
// issueBudgetLedgerEntryMaxBytes, and the append is where that is seen to: one
// byte more is refused before a statement is sent. The load applies no such
// limit. A row that another build or an operator wrote is read as it is, up to
// what the table and the metering package take.
func TestIssueBudgetLedgerEntryFormIsCappedWhereItIsWritten(t *testing.T) {
	if issueBudgetLedgerEntryMaxBytes != 2048 {
		t.Fatalf("an entry a server writes may have %d bytes, the design says 2,048", issueBudgetLedgerEntryMaxBytes)
	}
	f := newIssueBudgetFixture(t)
	ctx := context.Background()
	cache := newIssueBudgetLedgerCache(issueBudgetLedgerCacheLimit)
	warnings := issueBudgetLedgerWarnings(cache)
	// appendOne appends e to a root under its lock, behind a load.
	appendOne := func(t *testing.T, root *issueBudgetLedgerRoot, e budget.Entry, commit bool) (issueBudgetLedgerAppend, []string, error) {
		t.Helper()
		var (
			appended issueBudgetLedgerAppend
			err      error
		)
		sent := root.locked(t, commit, func(q *db.Queries, locked db.IssueBudgetRoot) {
			ledger, loadErr := cache.loadUnderRootLock(ctx, q, locked)
			if loadErr != nil {
				t.Fatal(loadErr)
			}
			appended, err = cache.appendEntries(ctx, q, locked, ledger, []issueBudgetLedgerEntry{{Entry: e, Source: "server"}})
		})
		return appended, sent, err
	}
	requireRefused := func(t *testing.T, e budget.Entry, appended issueBudgetLedgerAppend, sent []string, err error, what string) {
		t.Helper()
		if !errors.Is(err, budget.ErrInvalidEntry) {
			t.Fatalf("%s: %v, want a refused entry", what, err)
		}
		if errors.Is(err, ErrIssueBudgetLedgerUnavailable) {
			t.Fatalf("%s: a refused entry passes for an unavailable ledger: %v", what, err)
		}
		raw, _, encodeErr := budget.EncodeEntry(e)
		if encodeErr != nil {
			t.Fatal(encodeErr)
		}
		for _, text := range []string{
			fmt.Sprintf("budget entry %q", e.ID),
			fmt.Sprintf("a stored form of %d bytes", len(raw)),
			"the limit for an entry a server writes is 2048",
		} {
			if !strings.Contains(err.Error(), text) {
				t.Fatalf("%s: error %q, want it to say %q", what, err, text)
			}
		}
		if !reflect.DeepEqual(appended, issueBudgetLedgerAppend{}) {
			t.Fatalf("%s: a refused append returned %+v", what, appended)
		}
		requireIssueBudgetStatements(t, sent, nil, what)
	}

	attempt := uuid.NewString()
	reserved := issueBudgetLedgerFigures{ReservedTicks: 500}
	gapped := issueBudgetLedgerFigures{ReservedTicks: 500, UnknownCount: 1}
	root := f.ledgerRoot(t)
	root.write(t, reserved, issueBudgetReserveEntry(attempt, 500))
	base, _ := root.mustLoad(t, cache, "load the reservation")
	gap := issueBudgetGapEntry(attempt)

	t.Run("one byte over is refused with nothing sent", func(t *testing.T) {
		rows := f.snapshot(t)
		over := issueBudgetPadToForm(t, gap, issueBudgetLedgerEntryMaxBytes+1)
		appended, sent, err := appendOne(t, root, over, false)
		requireRefused(t, over, appended, sent, err, "an entry of 2,049 bytes")
		cache.requireCached(t, root.id, 1, base, "after an entry of 2,049 bytes")
		f.requireUnchanged(t, rows, "an entry of 2,049 bytes")
	})

	atCap := issueBudgetPadToForm(t, gap, issueBudgetLedgerEntryMaxBytes)
	t.Run("exactly the limit is written and read back", func(t *testing.T) {
		appended, sent, err := appendOne(t, root, atCap, true)
		if err != nil {
			t.Fatalf("an entry of 2,048 bytes: %v", err)
		}
		appended.Committed()
		requireIssueBudgetStatements(t, sent, []string{"InsertIssueBudgetEntry", "UpdateIssueBudgetRootLedger"}, "an append of an entry of 2,048 bytes")
		requireIssueBudgetLedger(t, appended.Ledger, 2, gapped, "the ledger with the entry of 2,048 bytes")
		cache.requireCached(t, root.id, 2, appended.Ledger, "after the entry of 2,048 bytes")

		want, _, err := budget.EncodeEntry(atCap)
		if err != nil {
			t.Fatal(err)
		}
		stored := root.entries(t)
		if len(stored) != 2 || len(stored[1].EntryJson) != 2048 || !bytes.Equal(stored[1].EntryJson, want) {
			t.Fatalf("the root has %d rows, the last with a stored form of %d bytes", len(stored), len(stored[len(stored)-1].EntryJson))
		}
		// A process with nothing cached reads it back, as a writer and as a
		// reader, with the detail whole.
		for _, holdsRootLock := range []bool{true, false} {
			fresh := newIssueBudgetLedgerCache(issueBudgetLedgerCacheLimit)
			var (
				ledger *budget.Ledger
				sent   []string
				err    error
			)
			if holdsRootLock {
				ledger, sent, err = root.load(t, fresh)
			} else {
				ledger, sent, err = root.read(t, fresh, f.pool, root.row(t))
			}
			if err != nil {
				t.Fatalf("a load of the entry of 2,048 bytes (root lock %v): %v", holdsRootLock, err)
			}
			requireIssueBudgetStatements(t, sent, []string{issueBudgetReadWholeLedger}, "a load of the entry of 2,048 bytes")
			requireIssueBudgetLedger(t, ledger, 2, gapped, "the entry of 2,048 bytes, read back")
			if unknown := ledger.Projection().Unknown; len(unknown) != 1 || unknown[0].EntryID != atCap.ID || unknown[0].Detail != atCap.Detail {
				t.Fatalf("the entry of 2,048 bytes was read back as %d unknown items (root lock %v)", len(unknown), holdsRootLock)
			}
		}
	})

	// A row of the largest form the table and the metering package take, the
	// way another build or an operator may have written it.
	large := f.ledgerRoot(t)
	largest := issueBudgetPadToForm(t, budget.Entry{ID: "exclusion/" + uuid.NewString(), Kind: budget.KindExclusion, Detail: "plugin-call"}, budget.MaxStoredEntryBytes)
	large.write(t, issueBudgetLedgerFigures{}, largest)
	t.Run("a load reads a row of the largest form", func(t *testing.T) {
		if budget.MaxStoredEntryBytes != 65_536 {
			t.Fatalf("the largest stored form is %d bytes", budget.MaxStoredEntryBytes)
		}
		if stored := large.entries(t); len(stored) != 1 || len(stored[0].EntryJson) != 65_536 {
			t.Fatalf("the root has %d rows", len(stored))
		}
		ledger, sent, err := large.load(t, cache)
		if err != nil {
			t.Fatalf("a load of a row of 65,536 bytes under the root lock: %v", err)
		}
		requireIssueBudgetStatements(t, sent, []string{issueBudgetReadWholeLedger}, "a load of a row of 65,536 bytes")
		requireIssueBudgetLedger(t, ledger, 1, issueBudgetLedgerFigures{}, "a row of 65,536 bytes")
		cache.requireCached(t, large.id, 1, ledger, "after a load of a row of 65,536 bytes")
		reader := newIssueBudgetLedgerCache(issueBudgetLedgerCacheLimit)
		ledger, sent, err = large.read(t, reader, f.pool, large.row(t))
		if err != nil {
			t.Fatalf("a reader of a row of 65,536 bytes: %v", err)
		}
		requireIssueBudgetStatements(t, sent, []string{issueBudgetReadWholeLedger}, "a reader of a row of 65,536 bytes")
		requireIssueBudgetLedger(t, ledger, 1, issueBudgetLedgerFigures{}, "a row of 65,536 bytes, read without the lock")
	})

	// The limit is on what the append is given, not on what the ledger holds.
	// The entry of that row given again would get no row, and is refused all
	// the same: it is an entry no server of this build wrote. A short entry
	// behind the long row is appended.
	t.Run("the ledger goes on behind a longer row", func(t *testing.T) {
		rows := f.snapshot(t)
		held, _ := large.mustLoad(t, cache, "load the row of 65,536 bytes")
		appended, sent, err := appendOne(t, large, largest, false)
		requireRefused(t, largest, appended, sent, err, "the entry of 65,536 bytes given again")
		cache.requireCached(t, large.id, 1, held, "after the entry of 65,536 bytes given again")
		f.requireUnchanged(t, rows, "the entry of 65,536 bytes given again")

		appended, sent, err = appendOne(t, large, issueBudgetReserveEntry(uuid.NewString(), 7), true)
		if err != nil {
			t.Fatalf("an append behind a row of 65,536 bytes: %v", err)
		}
		appended.Committed()
		requireIssueBudgetStatements(t, sent, []string{"InsertIssueBudgetEntry", "UpdateIssueBudgetRootLedger"}, "an append behind a row of 65,536 bytes")
		requireIssueBudgetLedger(t, appended.Ledger, 2, issueBudgetLedgerFigures{ReservedTicks: 7}, "behind a row of 65,536 bytes")
		fresh := newIssueBudgetLedgerCache(issueBudgetLedgerCacheLimit)
		ledger, _ := large.mustLoad(t, fresh, "a full replay behind a row of 65,536 bytes")
		requireIssueBudgetLedger(t, ledger, 2, issueBudgetLedgerFigures{ReservedTicks: 7}, "a full replay behind a row of 65,536 bytes")
	})
	requireNoIssueBudgetLedgerWarning(t, warnings(), "entries at and beyond the limit")
}

// The limit leaves room for every entry a server of the first slice writes.
// The largest of them is the usage entry of a version 1 report with the
// longest values such a report may carry (design 11.5): the identity of 200
// bytes, whose model name of 120 bytes is the one text a reporter chooses and
// is all "<", which the stored form writes as six bytes each; the largest
// number in the four token counts; and an estimated cost of the largest
// amount, with a price version of 64 bytes and a price digest.
//
// This is the test that fails when a later change of the stored form, or of
// the limits of design 11.5, pushes a valid entry over the limit. Then the
// limit, the bound on the cache's memory that follows from it (design 10.4)
// and this entry are looked at together; the entry is not made smaller to
// fit. No database.
func TestIssueBudgetLedgerEntryLimitHoldsTheLargestUsageEntry(t *testing.T) {
	const (
		attempt    = "0b6f7a52-3c1d-4e8f-9a2b-5d6c7e8f9a0b"
		invocation = "7c1e2d3f-4a5b-4c6d-8e7f-9a0b1c2d3e4f"
	)
	largest := func(model string) (budget.Entry, int) {
		e := issueBudgetUsageEntry(attempt, invocation, model, budget.Cost{
			Basis: budget.CostEstimated, Ticks: math.MaxInt64, Currency: budget.CurrencyUSD,
			PriceVersion: strings.Repeat("p", 64), PriceDigest: strings.Repeat("0", 64),
		})
		e.Usage.Tokens = budget.TokenCounts{Input: math.MaxInt64, Output: math.MaxInt64, CacheRead: math.MaxInt64, CacheWrite: math.MaxInt64}
		if len(e.ID) != budget.MaxEntryIDBytes {
			t.Fatalf("a usage identity of %d bytes, the longest is %d", len(e.ID), budget.MaxEntryIDBytes)
		}
		row, err := issueBudgetEntryRow(e)
		if err != nil {
			t.Fatalf("the largest usage entry is not an entry: %v", err)
		}
		return e, len(row.EntryJson)
	}
	entry, size := largest(strings.Repeat("<", 120))
	_, plain := largest(strings.Repeat("m", 120))
	t.Logf("the largest usage entry of a version 1 report has a stored form of %d bytes (%d with a model name of letters); a server may write %d", size, plain, issueBudgetLedgerEntryMaxBytes)
	if size > issueBudgetLedgerEntryMaxBytes {
		t.Fatalf("the largest usage entry of a version 1 report has a stored form of %d bytes, a server may write %d", size, issueBudgetLedgerEntryMaxBytes)
	}
	if size < plain {
		t.Fatalf("a model name of 120 times \"<\" gives %d bytes and one of letters %d: it is not the largest", size, plain)
	}
	// It is an entry a ledger takes, not only bytes that fit.
	reserved, _, err := budget.NewLedger().With(issueBudgetUnsizedReserveEntry(attempt))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := reserved.With(entry); err != nil {
		t.Fatalf("the largest usage entry is refused by the ledger: %v", err)
	}
}

// The columns of an entry row that follow from the entry are derived in one
// place (design section 8). No database.
func TestIssueBudgetEntryRowIsDerivedFromTheEntry(t *testing.T) {
	const (
		attempt    = "0b6f7a52-3c1d-4e8f-9a2b-5d6c7e8f9a0b"
		invocation = "7c1e2d3f-4a5b-4c6d-8e7f-9a0b1c2d3e4f"
	)
	attemptID := pgtype.UUID{Valid: true, Bytes: [16]byte{
		0x0b, 0x6f, 0x7a, 0x52, 0x3c, 0x1d, 0x4e, 0x8f, 0x9a, 0x2b, 0x5d, 0x6c, 0x7e, 0x8f, 0x9a, 0x0b,
	}}
	text := func(s string) pgtype.Text { return pgtype.Text{String: s, Valid: true} }
	cost := budget.Cost{Basis: budget.CostReported, Ticks: 7, Currency: budget.CurrencyUSD}
	// The longest identity of design 10.2: two ids of 36 bytes and a model
	// name of 120.
	longest := issueBudgetUsageEntry(attempt, invocation, strings.Repeat("m", 120), cost)
	if len(longest.ID) != 200 {
		t.Fatalf("the longest identity has %d bytes, want 200", len(longest.ID))
	}
	for _, tc := range []struct {
		name       string
		entry      budget.Entry
		kind       string
		attempt    pgtype.UUID
		invocation pgtype.Text
		scope      pgtype.Text
	}{
		{name: "a sized reservation", entry: issueBudgetReserveEntry(attempt, 500), kind: "reserve", attempt: attemptID},
		{name: "an unsized reservation", entry: issueBudgetUnsizedReserveEntry(attempt), kind: "reserve", attempt: attemptID},
		{name: "a hold", entry: issueBudgetHoldEntry(attempt, budget.HoldProcessExit), kind: "hold", attempt: attemptID},
		{
			name: "a settlement", kind: "settle", attempt: attemptID,
			entry: budget.Entry{
				ID: "settle/" + attempt, Kind: budget.KindSettle, ReservationID: attempt,
				Proof: budget.ReleaseProof{FinalUsage: true, ScopeComplete: true, NoFurtherExecution: true},
			},
		},
		{
			name: "usage with the longest identity", entry: longest, kind: "usage", attempt: attemptID,
			invocation: text(invocation), scope: text(attempt + "/" + invocation),
		},
		{
			name: "a gap of an attempt", kind: "gap", attempt: attemptID,
			entry: budget.Entry{ID: "gap/" + attempt + "/" + invocation, Kind: budget.KindGap, ReservationID: attempt, GapReason: budget.ReasonCoverageGap},
		},
		{
			name: "a gap of no attempt", kind: "gap",
			entry: budget.Entry{ID: "gap/before-activation", Kind: budget.KindGap, GapReason: budget.ReasonPreActivationUnknown},
		},
		{
			name: "an exclusion", kind: "exclusion",
			entry: budget.Entry{ID: "exclusion/one", Kind: budget.KindExclusion, Detail: "mechanism"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			row, err := issueBudgetEntryRow(tc.entry)
			if err != nil {
				t.Fatal(err)
			}
			raw, err := json.Marshal(tc.entry)
			if err != nil {
				t.Fatal(err)
			}
			want := db.InsertIssueBudgetEntryParams{
				EntryID: tc.entry.ID, Kind: tc.kind, AttemptID: tc.attempt, InvocationID: tc.invocation, ScopeID: tc.scope,
				EntryJson: raw, EntryDigest: issueBudgetStoredDigest(raw), SchemaVersion: 1,
			}
			if !reflect.DeepEqual(row, want) {
				t.Fatalf("row\n got %+v\nwant %+v", row, want)
			}
			back, err := budget.DecodeEntry(int(row.SchemaVersion), row.EntryJson)
			if err != nil || back != tc.entry {
				t.Fatalf("the stored form decodes to %+v (%v)", back, err)
			}
		})
	}

	tooLong := issueBudgetUsageEntry(attempt, invocation, strings.Repeat("m", 121), cost)
	withDetail := issueBudgetReserveEntry(attempt, 1)
	withDetail.Detail = "x"
	for _, tc := range []struct {
		name  string
		entry budget.Entry
		is    error
		text  string
	}{
		{name: "a reservation in upper case", entry: issueBudgetReserveEntry(strings.ToUpper(attempt), 1), text: "not the canonical text of an attempt id"},
		{name: "a reservation without dashes", entry: issueBudgetReserveEntry(strings.ReplaceAll(attempt, "-", ""), 1), text: "not the canonical text of an attempt id"},
		{name: "a reservation in braces", entry: issueBudgetReserveEntry("{"+attempt+"}", 1), text: "not the canonical text of an attempt id"},
		{name: "a reservation that is no UUID", entry: issueBudgetReserveEntry("attempt-1", 1), text: "not the canonical text of an attempt id"},
		{name: "a gap whose reservation is no UUID", entry: budget.Entry{ID: "gap/x", Kind: budget.KindGap, ReservationID: "x", GapReason: budget.ReasonCoverageGap}, text: "not the canonical text of an attempt id"},
		{name: "a kind the ledger does not know", entry: budget.Entry{ID: "x", Kind: "other"}, is: budget.ErrInvalidEntry, text: "unknown kind"},
		{name: "a usage identity of 201 bytes", entry: tooLong, is: budget.ErrInvalidEntry, text: "201 bytes"},
		{name: "fields of another kind", entry: withDetail, is: budget.ErrInvalidEntry, text: "fields of another kind"},
		{name: "no identity", entry: budget.Entry{Kind: budget.KindReserve, ReservationID: attempt}, is: budget.ErrInvalidEntry, text: "missing id"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			row, err := issueBudgetEntryRow(tc.entry)
			if err == nil {
				t.Fatalf("accepted as %+v", row)
			}
			if tc.is != nil && !errors.Is(err, tc.is) {
				t.Fatalf("error %v, want %v", err, tc.is)
			}
			if !strings.Contains(err.Error(), tc.text) {
				t.Fatalf("error %q, want it to say %q", err, tc.text)
			}
			if !reflect.DeepEqual(row, db.InsertIssueBudgetEntryParams{}) {
				t.Fatalf("a refused entry left a row: %+v", row)
			}
		})
	}
}

// One With call takes all the entries a load reads. Each call copies the whole
// state, so a call for each row would make a load cost the square of the
// ledger. What a replay allocates grows with the rows it is given. No
// database.
func TestIssueBudgetLedgerReplayCostGrowsWithTheRowsRead(t *testing.T) {
	replay := func(n int) uint64 {
		rows := make([]db.ListIssueBudgetEntryFormsAfterRow, n)
		for i := range rows {
			raw, _, err := budget.EncodeEntry(issueBudgetUnsizedReserveEntry(uuid.NewString()))
			if err != nil {
				t.Fatal(err)
			}
			rows[i] = db.ListIssueBudgetEntryFormsAfterRow{Seq: int64(i + 1), SchemaVersion: 1, EntryJson: raw}
		}
		root := db.IssueBudgetRoot{LedgerRevision: int64(n), UnknownCount: int64(n)}
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		ledger, err := replayIssueBudgetLedger(root, budget.NewLedger(), 0, rows, true)
		runtime.ReadMemStats(&after)
		if err != nil || ledger.Len() != n {
			t.Fatalf("replay of %d rows: %v", n, err)
		}
		return after.TotalAlloc - before.TotalAlloc
	}
	// Four times the rows allocate four times the bytes when the cost grows
	// with the rows, and sixteen times when it grows with their square.
	small, large := replay(200), replay(800)
	if large > 6*small {
		t.Fatalf("replaying 800 rows allocated %d bytes and 200 rows %d: more than six times as much for four times the rows", large, small)
	}
}

// The cache is used by every request of the process at once. This test is
// for the race detector: states are handed out and read while others are
// published and removed.
func TestIssueBudgetLedgerCacheIsSafeForConcurrentUse(t *testing.T) {
	cache := newIssueBudgetLedgerCache(40)
	var roots [6]pgtype.UUID
	for i := range roots {
		roots[i] = issueBudgetRootID()
	}
	// states[n] is a ledger of n entries, published at revision n.
	states := make([]*budget.Ledger, 9)
	for n := range states {
		states[n] = issueBudgetLedgerOf(t, n)
	}
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < 500; i++ {
				root := roots[(g+i)%len(roots)]
				n := (g*7 + i) % len(states)
				switch i % 4 {
				case 0:
					cache.publish(root, int64(n), states[n])
				case 1:
					if ledger, revision, ok := cache.lookup(root, int64(len(states)), g%2 == 0); ok {
						if int64(ledger.Len()) != revision || len(ledger.Projection().Unknown) != ledger.Len() {
							t.Errorf("a state of %d entries was handed out at revision %d", ledger.Len(), revision)
						}
					}
				case 2:
					cache.lookup(root, int64(n), g%2 == 0)
				case 3:
					cache.forget(root, nil)
				}
			}
		}(g)
	}
	wg.Wait()
	cache.requireWithinLimit(t, "after concurrent use")
}

// Writers of one root append one after another under its lock, each from what
// the cache holds and the entries the others committed since, while readers
// load without the lock. No load may fail, and at the end the cache holds the
// ledger the rows replay to.
func TestIssueBudgetLedgerConcurrentWritersAndReadersAgree(t *testing.T) {
	f := newIssueBudgetFixture(t)
	ctx := context.Background()
	cache := newIssueBudgetLedgerCache(issueBudgetLedgerCacheLimit)
	root := f.ledgerRoot(t)
	const writers, appends, total = 4, 6, 24

	appendOne := func() error {
		tx, err := f.pool.Begin(ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback(ctx)
		q := db.New(tx)
		locked, err := q.LockIssueBudgetRoot(ctx, db.LockIssueBudgetRootParams{WorkspaceID: f.ws, ID: root.id})
		if err != nil {
			return err
		}
		ledger, err := cache.loadUnderRootLock(ctx, q, locked)
		if err != nil {
			return err
		}
		appended, err := cache.appendEntries(ctx, q, locked, ledger, []issueBudgetLedgerEntry{
			{Entry: issueBudgetUnsizedReserveEntry(uuid.NewString()), Source: "server"},
		})
		if err != nil {
			return err
		}
		if err := tx.Commit(ctx); err != nil {
			return err
		}
		appended.Committed()
		return nil
	}

	var writing, reading sync.WaitGroup
	done := make(chan struct{})
	for w := 0; w < writers; w++ {
		writing.Add(1)
		go func() {
			defer writing.Done()
			for i := 0; i < appends; i++ {
				if err := appendOne(); err != nil {
					t.Errorf("writer: %v", err)
					return
				}
			}
		}()
	}
	for r := 0; r < 2; r++ {
		reading.Add(1)
		go func() {
			defer reading.Done()
			for {
				select {
				case <-done:
					return
				default:
				}
				row, err := f.q.GetIssueBudgetRoot(ctx, db.GetIssueBudgetRootParams{WorkspaceID: f.ws, ID: root.id})
				if err != nil {
					t.Errorf("reader: %v", err)
					return
				}
				ledger, err := cache.loadWithoutRootLock(ctx, f.q, row)
				if err != nil {
					t.Errorf("reader at revision %d: %v", row.LedgerRevision, err)
					return
				}
				if int64(ledger.Len()) != row.LedgerRevision {
					t.Errorf("reader at revision %d got %d entries", row.LedgerRevision, ledger.Len())
					return
				}
			}
		}()
	}
	writing.Wait()
	close(done)
	reading.Wait()
	if t.Failed() {
		return
	}

	want := issueBudgetLedgerFigures{UnknownCount: total}
	revision, cached, ok := cache.peek(root.id)
	if !ok || revision != total {
		t.Fatalf("the cache holds revision %d (cached %v), want %d", revision, ok, total)
	}
	requireIssueBudgetLedger(t, cached, total, want, "the cached state")
	whole, _ := root.mustLoad(t, newIssueBudgetLedgerCache(issueBudgetLedgerCacheLimit), "replay of every row")
	requireIssueBudgetLedger(t, whole, total, want, "the replay of every row")
	if row := root.row(t); row.LedgerRevision != total || row.UnknownCount != total {
		t.Fatalf("root row at revision %d with %d unknown, want %d and %d", row.LedgerRevision, row.UnknownCount, total, total)
	}
	for i, row := range root.entries(t) {
		if row.Seq != int64(i+1) {
			t.Fatalf("entry row %d has seq %d", i, row.Seq)
		}
	}
}

// Design 10.4: a cached ledger is shared by every request that loads its
// root, so nothing may write through a pointer to one. The files of this
// package that import the metering package hold no assignment through a
// pointer at all: a rule a reader checks by eye and this test by syntax. It
// does not see a ledger handled in a file that never imports the package.
func TestIssueBudgetLedgerStatesAreNeverAssignedThrough(t *testing.T) {
	const meteringPackage = `"github.com/multica-ai/multica/server/internal/budget"`
	files, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	var scanned []string
	for _, entry := range files {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, name, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		imports := false
		for _, spec := range file.Imports {
			if spec.Path.Value == meteringPackage {
				imports = true
			}
		}
		if !imports {
			continue
		}
		scanned = append(scanned, name)
		ast.Inspect(file, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.AssignStmt:
				for _, target := range n.Lhs {
					if _, ok := ast.Unparen(target).(*ast.StarExpr); ok {
						t.Errorf("%s assigns through a pointer in a file that handles budget ledgers", fset.Position(target.Pos()))
					}
				}
			case *ast.IncDecStmt:
				if _, ok := ast.Unparen(n.X).(*ast.StarExpr); ok {
					t.Errorf("%s changes a value through a pointer in a file that handles budget ledgers", fset.Position(n.Pos()))
				}
			}
			return true
		})
	}
	found := false
	for _, name := range scanned {
		if name == "issue_budget_ledger.go" {
			found = true
		}
	}
	if !found {
		t.Fatalf("the scan looked at %v and not at the ledger cache", scanned)
	}
}

// What a cache at its limit keeps in memory (design 10.4): the live heap of a
// cache filled to issueBudgetLedgerCacheLimit, by the unit of weight. Each
// state is loaded the way a process loads it, from stored forms through
// replayIssueBudgetLedger, so its strings are the decoder's own, and the
// figure includes what the cache itself keeps for a root. No database:
//
//	go test ./internal/service/ -run '^$' -bench IssueBudgetLedgerCacheMemory -benchtime 1x
//
// The identities are those of design 10.2, the longest of them 200 bytes. The
// cell "form-limit" is the one that is not such a ledger: its entries have
// the largest stored form EncodeEntry lets through. A ledger keeps its
// entries whole, so that cell shows what the weight does not bound.
//
// The cells "written-limit-..." are the bound that holds whatever a server
// writes: every entry that can be is at issueBudgetLedgerEntryMaxBytes, with
// text the ledger keeps byte for byte, and the cache is filled to its limit.
// An exclusion and a gap that names no attempt need no other entry, so all of
// such a root is at the limit. A usage entry hangs on a reservation, which has
// no text to lengthen: those roots have one reservation, or one in twenty
// entries, below the limit. The usage entries are the dearest a ledger keeps
// (an estimate stays in the unknown list), lengthened by their price version.

// issueBudgetBenchModel is the longest model name a usage identity has room
// for; with it the identity is exactly 200 bytes.
var issueBudgetBenchModel = strings.Repeat("m", 120)

// issueBudgetBenchShape is what the attempts of a root leave in its ledger:
// an unsized reservation each, and the entries behind it.
type issueBudgetBenchShape struct {
	// perAttempt is the entries of one attempt, its reservation included. 0
	// is one attempt for the whole root.
	perAttempt int
	entry      func(attempt string) budget.Entry
	// atWriteLimit lengthens the price version of every entry behind a
	// reservation until its stored form is issueBudgetLedgerEntryMaxBytes.
	atWriteLimit bool
}

// issueBudgetBenchUsage is one request of an attempt, a meter scope of its
// own.
func issueBudgetBenchUsage(attempt string, cost budget.Cost) budget.Entry {
	cost.Ticks, cost.Currency = 1_000_000, budget.CurrencyUSD
	e := issueBudgetUsageEntry(attempt, uuid.NewString(), issueBudgetBenchModel, cost)
	e.Usage.Tokens = budget.TokenCounts{Input: 1_000, Output: 100, CacheRead: 5_000}
	return e
}

// A reported cost is known. An estimated one stays in the unknown list.
func issueBudgetBenchReported(attempt string) budget.Entry {
	return issueBudgetBenchUsage(attempt, budget.Cost{Basis: budget.CostReported})
}

func issueBudgetBenchEstimated(attempt string) budget.Entry {
	return issueBudgetBenchUsage(attempt, budget.Cost{Basis: budget.CostEstimated, PriceVersion: "prices-2026-11", PriceDigest: strings.Repeat("0", 64)})
}

var issueBudgetBenchShapes = map[string]issueBudgetBenchShape{
	// Claimed and not yet over: what a claim writes.
	"claimed": {perAttempt: 1},
	// Claimed and ended without usage.
	"held": {perAttempt: 2, entry: func(attempt string) budget.Entry {
		return issueBudgetHoldEntry(attempt, budget.HoldLostContact)
	}},
	// An invocation that reported nothing usable.
	"gap": {perAttempt: 20, entry: func(attempt string) budget.Entry {
		return budget.Entry{ID: "gap/" + attempt + "/" + uuid.NewString(), Kind: budget.KindGap, ReservationID: attempt, GapReason: budget.ReasonCoverageGap}
	}},
	"reported":      {perAttempt: 20, entry: issueBudgetBenchReported},
	"reported-run":  {entry: issueBudgetBenchReported},
	"estimated":     {perAttempt: 20, entry: issueBudgetBenchEstimated},
	"estimated-run": {entry: issueBudgetBenchEstimated},
	// The same two with every usage entry as long as a server may write.
	"written-limit-estimated":     {perAttempt: 20, entry: issueBudgetBenchEstimated, atWriteLimit: true},
	"written-limit-estimated-run": {entry: issueBudgetBenchEstimated, atWriteLimit: true},
}

// issueBudgetBenchLedger is the entries of a root of n entries in a shape.
func issueBudgetBenchLedger(b *testing.B, name string, n int) []budget.Entry {
	b.Helper()
	switch name {
	case "form-limit":
		return issueBudgetBenchForms(b, n, budget.KindExclusion, budget.MaxStoredEntryBytes)
	case "written-limit-exclusion":
		return issueBudgetBenchForms(b, n, budget.KindExclusion, issueBudgetLedgerEntryMaxBytes)
	case "written-limit-gap":
		return issueBudgetBenchForms(b, n, budget.KindGap, issueBudgetLedgerEntryMaxBytes)
	}
	shape, ok := issueBudgetBenchShapes[name]
	if !ok {
		b.Fatalf("no shape %q", name)
	}
	entries := make([]budget.Entry, 0, n)
	for len(entries) < n {
		attempt := uuid.NewString()
		entries = append(entries, issueBudgetUnsizedReserveEntry(attempt))
		for i := 1; len(entries) < n && (shape.perAttempt == 0 || i < shape.perAttempt); i++ {
			e := shape.entry(attempt)
			if shape.atWriteLimit {
				raw, _, err := budget.EncodeEntry(e)
				if err != nil {
					b.Fatal(err)
				}
				e.Usage.Cost.PriceVersion += strings.Repeat("p", issueBudgetLedgerEntryMaxBytes-len(raw))
				if raw, _, err = budget.EncodeEntry(e); err != nil || len(raw) != issueBudgetLedgerEntryMaxBytes {
					b.Fatalf("a usage entry with a stored form of %d bytes: %v", len(raw), err)
				}
			}
			entries = append(entries, e)
		}
	}
	return entries
}

// issueBudgetBenchForms is n entries whose stored form is exactly size bytes
// each: exclusions, or gaps that name no attempt. Neither needs another entry
// in the ledger, so every entry of the root has that size. What is beyond the
// fixed part of the form is detail, which costs one byte in the form for each
// byte the ledger keeps.
func issueBudgetBenchForms(b *testing.B, n int, kind budget.EntryKind, size int) []budget.Entry {
	b.Helper()
	entries := make([]budget.Entry, n)
	for i := range entries {
		e := budget.Entry{ID: fmt.Sprintf("exclusion/%05d", i), Kind: budget.KindExclusion, Detail: fmt.Sprintf("%05d", i)}
		if kind == budget.KindGap {
			e = budget.Entry{ID: "gap/" + uuid.NewString(), Kind: budget.KindGap, GapReason: budget.ReasonCoverageGap}
		}
		entries[i] = issueBudgetPadToForm(b, e, size)
	}
	return entries
}

// issueBudgetBenchLoad puts a root with these entries into the cache the way
// a load under the root lock does.
func issueBudgetBenchLoad(b *testing.B, cache *issueBudgetLedgerCache, entries []budget.Entry) {
	b.Helper()
	whole, _, err := budget.NewLedger().With(entries...)
	if err != nil {
		b.Fatal(err)
	}
	figures := issueBudgetLedgerFiguresOf(whole)
	root := db.IssueBudgetRoot{
		ID: issueBudgetRootID(), LedgerRevision: int64(len(entries)),
		ActualKnownTicks: figures.ActualKnownTicks, EstimatedTicks: figures.EstimatedTicks, ReservedTicks: figures.ReservedTicks,
		UnknownHoldTicks: figures.UnknownHoldTicks, OverrunTicks: figures.OverrunTicks, UnknownCount: figures.UnknownCount,
	}
	rows := make([]db.ListIssueBudgetEntryFormsAfterRow, len(entries))
	for i, e := range entries {
		raw, _, err := budget.EncodeEntry(e)
		if err != nil {
			b.Fatal(err)
		}
		rows[i] = db.ListIssueBudgetEntryFormsAfterRow{Seq: int64(i + 1), SchemaVersion: budget.EntryFormVersion, EntryJson: raw}
	}
	ledger, err := replayIssueBudgetLedger(root, budget.NewLedger(), 0, rows, true)
	if err != nil {
		b.Fatal(err)
	}
	cache.publish(root.ID, root.LedgerRevision, ledger)
}

func issueBudgetBenchLiveHeap() uint64 {
	var stats runtime.MemStats
	runtime.GC()
	runtime.GC()
	runtime.ReadMemStats(&stats)
	return stats.HeapAlloc
}

func BenchmarkIssueBudgetLedgerCacheMemory(b *testing.B) {
	type cell struct {
		shape   string
		perRoot int
		// roots 0 fills the cache to its limit.
		roots int
	}
	// A map keeps between 7/16 and 7/8 of its slots in use, so what an entry
	// costs moves with the size of its root. The sizes are those where a
	// scan of 1 to 1,100 entries found a shape at its dearest (just behind a
	// doubling: 226 for a run, 237 with twenty entries to an attempt) and
	// some between and beyond them.
	sizes := []int{1, 2, 8, 9, 20, 57, 100, 113, 226, 237, 450, 473, 898, 5_000, issueBudgetLedgerCacheLimit - issueBudgetLedgerRootWeight}
	// A root without an entry is what the cache keeps for a root by itself.
	cells := []cell{{shape: "claimed", perRoot: 0}}
	for _, shape := range []string{"claimed", "held", "gap", "reported", "reported-run", "estimated", "estimated-run"} {
		for _, size := range sizes {
			cells = append(cells, cell{shape: shape, perRoot: size})
		}
	}
	cells = append(cells, cell{shape: "form-limit", perRoot: 100, roots: 4})
	for _, shape := range []string{"written-limit-exclusion", "written-limit-gap", "written-limit-estimated", "written-limit-estimated-run"} {
		for _, size := range sizes {
			cells = append(cells, cell{shape: shape, perRoot: size})
		}
	}

	// What the decoder sets up once is not part of a cached state.
	warm := newIssueBudgetLedgerCache(issueBudgetLedgerCacheLimit)
	for shape := range issueBudgetBenchShapes {
		issueBudgetBenchLoad(b, warm, issueBudgetBenchLedger(b, shape, 20))
	}
	if id := issueBudgetBenchEstimated(uuid.NewString()).ID; len(id) != 200 {
		b.Fatalf("a usage identity of %d bytes", len(id))
	}
	for _, cell := range cells {
		b.Run(fmt.Sprintf("%s/%d", cell.shape, cell.perRoot), func(b *testing.B) {
			roots := cell.roots
			if roots == 0 {
				roots = issueBudgetLedgerCacheLimit / (cell.perRoot + issueBudgetLedgerRootWeight)
			}
			var perWeight float64
			for i := 0; i < b.N; i++ {
				before := issueBudgetBenchLiveHeap()
				cache := newIssueBudgetLedgerCache(issueBudgetLedgerCacheLimit)
				for r := 0; r < roots; r++ {
					issueBudgetBenchLoad(b, cache, issueBudgetBenchLedger(b, cell.shape, cell.perRoot))
				}
				after := issueBudgetBenchLiveHeap()
				cache.mu.Lock()
				kept, weight := len(cache.roots), cache.weight
				cache.mu.Unlock()
				if kept != roots || weight != roots*(cell.perRoot+issueBudgetLedgerRootWeight) {
					b.Fatalf("the cache keeps %d roots of weight %d, loaded %d of %d entries", kept, weight, roots, cell.perRoot)
				}
				perWeight = 0
				if after > before {
					perWeight = float64(after-before) / float64(weight)
				}
				runtime.KeepAlive(cache)
			}
			b.ReportMetric(perWeight, "B/weight")
			b.ReportMetric(perWeight*issueBudgetLedgerCacheLimit/(1<<20), "MiB-at-limit")
		})
	}
}
