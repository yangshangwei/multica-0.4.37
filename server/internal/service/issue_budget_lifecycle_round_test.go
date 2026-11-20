package service

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// The round of the lifecycle reconciliation (design 10.3, the trigger): the
// read that finds the roots with an ended attempt, and the loop that
// reconciles them one after the other.
//
// The read is over every workspace. A test here looks only at the roots of
// its own workspace in what the read returns, and runs the loop over the
// roots it names. So it neither depends on nor changes what the tests of
// another package have in the same database at that moment. The round as a
// whole, the read and the loop together and what a round sends when there is
// nothing to reconcile, is tested with the loop of cmd/server.
//
// The expectations are written from the design and the rulings of 2026-11-20
// on the trigger: which roots are found, what a round counts, and that a root
// that fails, runs out of its time or is skipped keeps every row it had.

// issueBudgetEveryRoot is a limit of the read that no test database reaches.
const issueBudgetEveryRoot = math.MaxInt32

// The task statuses by whether a task leaves them again: the three that end
// an attempt, and the ones a task can still leave. Both lists are sorted.
var (
	issueBudgetFinalTaskStatuses = []string{"cancelled", "completed", "failed"}
	issueBudgetLiveTaskStatuses  = []string{"deferred", "dispatched", "queued", "running", "waiting_local_directory"}
)

var issueBudgetQuotedWord = regexp.MustCompile(`'([a-z_]+)'`)

// issueBudgetQuoted is every quoted lower-case word of a piece of SQL, in the
// order it stands there.
func issueBudgetQuoted(sql string) []string {
	var words []string
	for _, m := range issueBudgetQuotedWord.FindAllStringSubmatch(sql, -1) {
		words = append(words, m[1])
	}
	return words
}

// endedRoots is what the read returns of the fixture workspace. It fails
// unless the whole answer is in root order with every root once.
func (f *issueBudgetFixture) endedRoots(t *testing.T, what string) []string {
	t.Helper()
	rows, err := f.q.ListIssueBudgetRootsWithEndedAttempts(context.Background(), issueBudgetEveryRoot)
	if err != nil {
		t.Fatalf("%s: find the roots with an ended attempt: %v", what, err)
	}
	own := []string{}
	last := ""
	for _, row := range rows {
		id := util.UUIDToString(row.ID)
		if id <= last {
			t.Fatalf("%s: the read is not in root order with every root once: %s stands after %s", what, id, last)
		}
		last = id
		if row.WorkspaceID == f.ws {
			own = append(own, id)
		}
	}
	return own
}

// roundRows is what the read would return for the roots of the claims given,
// in the order given.
func (f *issueBudgetFixture) roundRows(claims ...issueBudgetLifecycleClaim) []db.ListIssueBudgetRootsWithEndedAttemptsRow {
	rows := make([]db.ListIssueBudgetRootsWithEndedAttemptsRow, 0, len(claims))
	for _, c := range claims {
		rows = append(rows, db.ListIssueBudgetRootsWithEndedAttemptsRow{ID: c.rootID, WorkspaceID: f.ws})
	}
	return rows
}

// issueBudgetRoundFailures are the lines a round logged about a root it could
// not reconcile.
func issueBudgetRoundFailures(lines []string) []string {
	var failures []string
	for _, line := range lines {
		if strings.Contains(line, "budget lifecycle reconciliation of a root failed") {
			failures = append(failures, line)
		}
	}
	return failures
}

func issueBudgetSorted(ids ...string) []string {
	sort.Strings(ids)
	return ids
}

// R01 (10.3, the trigger; ruling B-2). The read finds a root that has an open
// attempt whose task reached a final status, and a root whose task row is
// gone. It does not find a root whose open attempt has a task that can still
// run, a root whose ended attempt is closed already, or a root that is off.
// The root that is off is found once it observes again. A root with two ended
// attempts is listed once.
func TestIssueBudgetLifecycleRoundFindsTheRootsWithAnEndedAttempt(t *testing.T) {
	f := newIssueBudgetFixture(t)
	ctx := context.Background()
	agent, runtime := f.agent(t)
	final := f.lifecycleClaim(t, agent, runtime, "final")
	gone := f.lifecycleClaim(t, agent, runtime, "gone")
	live := f.lifecycleClaim(t, agent, runtime, "live")
	closed := f.lifecycleClaim(t, agent, runtime, "closed")
	off := f.lifecycleClaim(t, agent, runtime, "off")
	if got := f.endedRoots(t, "premise"); len(got) != 0 {
		t.Fatalf("premise: the read finds %v while every task can still run", got)
	}

	f.endTask(t, final.task, "failed", "agent_error")
	second := f.claimed(t, agent, runtime, final.child, 1)
	f.mustFinalize(t, f.claims(f.pool), second, TaskClaimBudgetProtocolV1)
	f.endTask(t, second, "cancelled", nil)
	f.fx.Exec(t, `DELETE FROM agent_task_queue WHERE id=$1`, util.UUIDToString(gone.task.ID))
	f.endTask(t, closed.task, "completed", nil)
	f.reconcile(t, closed.root, "closed")
	f.endTask(t, off.task, "completed", nil)
	switched, err := f.svc.SetMode(ctx, f.manage(off.root, 1), IssueBudgetModeOff)
	if err != nil {
		t.Fatalf("switch the root off: %v", err)
	}
	f.requireLifecycleAttempt(t, live, "open", "premise: the attempt whose task still runs")

	want := issueBudgetSorted(final.root, gone.root)
	if got := f.endedRoots(t, "one root off"); !reflect.DeepEqual(got, want) {
		t.Fatalf("the read finds %v, want the root with the final tasks and the root whose task row is gone: %v", got, want)
	}

	if _, err := f.svc.SetMode(ctx, f.manage(off.root, int64(switched.PolicyRevision)), IssueBudgetModeObserve); err != nil {
		t.Fatalf("switch the root back to observe: %v", err)
	}
	want = issueBudgetSorted(final.root, gone.root, off.root)
	if got := f.endedRoots(t, "observing again"); !reflect.DeepEqual(got, want) {
		t.Fatalf("with the root observing again the read finds %v, want %v", got, want)
	}
}

// R02 (ruling B-5), on the rows. For every status the task table allows, the
// read finds the root exactly when a reconciliation would close the attempt:
// both ask the same task row. If the two ever differed, a root would be found
// by every round and nothing of it would close. The statuses come from the
// constraint of the table, so a status added later is asked too, and has to
// be placed in one of the two lists above.
func TestIssueBudgetLifecycleRoundFindsExactlyWhatAReconciliationCloses(t *testing.T) {
	f := newIssueBudgetFixture(t)
	ctx := context.Background()
	agent, runtime := f.agent(t)
	c := f.lifecycleClaim(t, agent, runtime, "statuses")
	task := util.UUIDToString(c.task.ID)

	var constraint string
	f.fx.QueryRow(t, `SELECT pg_get_constraintdef(oid) FROM pg_constraint
		WHERE conrelid='agent_task_queue'::regclass AND conname='agent_task_queue_status_check'`).Scan(&constraint)
	statuses := issueBudgetQuoted(constraint)
	if len(statuses) < len(issueBudgetFinalTaskStatuses)+len(issueBudgetLiveTaskStatuses) {
		t.Fatalf("premise: the status constraint of the task table reads %q", constraint)
	}
	// ask says what the read and the reconciliation make of the task row as it
	// is now.
	ask := func(what string) (found, ended bool) {
		open, err := f.q.ListIssueBudgetOpenAttemptsOfRoot(ctx, db.ListIssueBudgetOpenAttemptsOfRootParams{WorkspaceID: f.ws, BudgetRootID: c.rootID})
		if err != nil || len(open) != 1 {
			t.Fatalf("%s: the root has %d open attempts (%v), want 1", what, len(open), err)
		}
		_, ended = issueBudgetAttemptEndOf(open[0])
		return slices.Contains(f.endedRoots(t, what), c.root), ended
	}
	for _, status := range statuses {
		final := slices.Contains(issueBudgetFinalTaskStatuses, status)
		if final == slices.Contains(issueBudgetLiveTaskStatuses, status) {
			t.Fatalf("the task table allows the status %q, which this test does not place: decide whether a task in it has ended, then add it to the find statement and to issueBudgetAttemptEndOf, or to the statuses a task can still leave", status)
		}
		f.fx.Exec(t, `UPDATE agent_task_queue SET status=$2 WHERE id=$1`, task, status)
		if found, ended := ask(status); found != final || ended != final {
			t.Fatalf("task status %q: the read finds the root: %v; a reconciliation would close the attempt: %v; want both %v", status, found, ended, final)
		}
	}
	f.fx.Exec(t, `DELETE FROM agent_task_queue WHERE id=$1`, task)
	if found, ended := ask("the task row gone"); !found || !ended {
		t.Fatalf("the task row gone: the read finds the root: %v; a reconciliation would close the attempt: %v; want both true", found, ended)
	}
}

// issueBudgetQueryStatement is one statement of the budget query file without
// its comment lines, on one line.
func issueBudgetQueryStatement(t *testing.T, name string) string {
	t.Helper()
	source, err := os.ReadFile(filepath.Join("..", "..", "pkg", "db", "queries", "issue_budget.sql"))
	if err != nil {
		t.Fatalf("read the budget query file: %v", err)
	}
	_, rest, ok := strings.Cut(string(source), "-- name: "+name+" :")
	if !ok {
		t.Fatalf("the budget query file has no statement %s", name)
	}
	rest, _, _ = strings.Cut(rest, "\n-- name: ")
	var words []string
	for i, line := range strings.Split(rest, "\n") {
		if i == 0 || strings.HasPrefix(strings.TrimSpace(line), "--") {
			continue
		}
		words = append(words, strings.Fields(line)...)
	}
	return strings.Join(words, " ")
}

// issueBudgetDeclaredTaskStatuses is the status list of the last migration
// that defines the status constraint of the task table. It reads files only;
// R02 asks the catalog of a migrated database for the same list.
func issueBudgetDeclaredTaskStatuses(t *testing.T) []string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join("..", "..", "migrations", "*.up.sql"))
	if err != nil || len(files) == 0 {
		t.Fatalf("list the migrations: %d files, %v", len(files), err)
	}
	version := func(path string) int {
		prefix, _, _ := strings.Cut(filepath.Base(path), "_")
		n, err := strconv.Atoi(prefix)
		if err != nil {
			t.Fatalf("migration %s has no version number", path)
		}
		return n
	}
	sort.SliceStable(files, func(i, j int) bool { return version(files[i]) < version(files[j]) })
	list := regexp.MustCompile(`CHECK\s*\(\s*status\s+IN\s*\(([^)]*)\)`)
	var statuses []string
	for _, file := range files {
		source, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		at := strings.LastIndex(string(source), "ADD CONSTRAINT agent_task_queue_status_check")
		if at < 0 {
			continue
		}
		m := list.FindStringSubmatch(string(source[at:]))
		if m == nil {
			t.Fatalf("%s defines the status constraint of the task table in a form this guard does not read", file)
		}
		statuses = issueBudgetQuoted(m[1])
	}
	if len(statuses) == 0 {
		t.Fatal("no migration defines the status constraint of the task table")
	}
	return statuses
}

// R03 (rulings B-3 and B-5), on the sources; it opens no database. The
// statement that finds the roots is a plain read of open attempts, in root
// order and bounded, and what it takes for ended is what
// issueBudgetAttemptEndOf closes an attempt for, whatever the two claim
// generations are: the three final statuses and the row that is gone, and no
// other status the migrations declare. A status a later migration declares
// has to be placed.
func TestIssueBudgetLifecycleRoundFindStatementEndsWhatAReconciliationEnds(t *testing.T) {
	statement := issueBudgetQueryStatement(t, "ListIssueBudgetRootsWithEndedAttempts")
	if !strings.HasPrefix(statement, "SELECT ") {
		t.Fatalf("the statement is not a read: %s", statement)
	}
	for _, word := range []string{" FOR UPDATE", " FOR NO KEY UPDATE", " FOR SHARE", " FOR KEY SHARE", "INSERT ", "UPDATE ", "DELETE "} {
		if strings.Contains(strings.ToUpper(statement), word) {
			t.Fatalf("the statement locks or writes (%q): %s", strings.TrimSpace(word), statement)
		}
	}
	for _, part := range []string{"a.state = 'open'", "r.mode <> 'off'", "(t.id IS NULL OR t.status IN ("} {
		if !strings.Contains(statement, part) {
			t.Fatalf("the statement does not read %q: %s", part, statement)
		}
	}
	if !strings.HasSuffix(statement, " ORDER BY r.id LIMIT @max_roots;") {
		t.Fatalf("the statement does not end in root order with a bound on the roots: %s", statement)
	}
	lists := regexp.MustCompile(`t\.status IN \(([^)]*)\)`).FindAllStringSubmatch(statement, -1)
	if len(lists) != 1 {
		t.Fatalf("the statement names the final statuses %d times, want once: %s", len(lists), statement)
	}
	final := issueBudgetSorted(issueBudgetQuoted(lists[0][1])...)
	if !reflect.DeepEqual(final, issueBudgetFinalTaskStatuses) {
		t.Fatalf("the statement takes a task in %v for ended, want %v", final, issueBudgetFinalTaskStatuses)
	}

	declared := issueBudgetDeclaredTaskStatuses(t)
	for _, status := range final {
		if !slices.Contains(declared, status) {
			t.Fatalf("the statement names the status %q, which the task table does not allow (%v)", status, declared)
		}
	}
	for _, status := range declared {
		isFinal := slices.Contains(final, status)
		if isFinal == slices.Contains(issueBudgetLiveTaskStatuses, status) {
			t.Fatalf("the migrations declare the task status %q, which this guard does not place: decide whether a task in it has ended, then add it to the find statement and to issueBudgetAttemptEndOf, or to the statuses a task can still leave", status)
		}
		// The task's generation is the attempt's, or a later claim's.
		for _, taskGeneration := range []int64{1, 2} {
			_, ended := issueBudgetAttemptEndOf(db.ListIssueBudgetOpenAttemptsOfRootRow{
				TaskExists: true, TaskStatus: status, ClaimGeneration: 1, TaskClaimGeneration: taskGeneration,
			})
			if ended != isFinal {
				t.Fatalf("task status %q at claim generation %d: the statement takes it for ended: %v; issueBudgetAttemptEndOf: %v", status, taskGeneration, isFinal, ended)
			}
		}
	}
	if _, ended := issueBudgetAttemptEndOf(db.ListIssueBudgetOpenAttemptsOfRootRow{ClaimGeneration: 1}); !ended {
		t.Fatal("the statement finds an attempt whose task row is gone, and issueBudgetAttemptEndOf does not close it")
	}
}

// R04 (10.3, the trigger). The loop reconciles every root it is given in a
// transaction of its own and counts what it did: the attempts it closed over
// all roots, and the roots it left as they were, one that went off after it
// was found and one whose ledger does not load. A skipped root keeps its open
// attempt and is no failure.
func TestIssueBudgetLifecycleRoundReconcilesEachRootInATransactionOfItsOwn(t *testing.T) {
	f := newIssueBudgetFixture(t)
	ctx := context.Background()
	agent, runtime := f.agent(t)
	logs := issueBudgetClaimLog(t)
	one := f.lifecycleClaim(t, agent, runtime, "one")
	f.endTask(t, one.task, "completed", nil)
	off := f.lifecycleClaim(t, agent, runtime, "off")
	f.endTask(t, off.task, "completed", nil)
	if _, err := f.svc.SetMode(ctx, f.manage(off.root, 1), IssueBudgetModeOff); err != nil {
		t.Fatalf("switch the root off: %v", err)
	}
	damaged := f.lifecycleClaim(t, agent, runtime, "damaged")
	f.endTask(t, damaged.task, "completed", nil)
	issueBudgetLifecycleDamages["projection"](t, &issueBudgetLedgerRoot{f: f, id: damaged.rootID})
	two := f.lifecycleClaim(t, agent, runtime, "two")
	f.endTask(t, two.task, "cancelled", nil)
	second := f.claimed(t, agent, runtime, two.child, 1)
	f.mustFinalize(t, f.claims(f.pool), second, TaskClaimBudgetProtocolV1)
	f.endTask(t, second, "failed", "runtime_offline")
	logs()

	log := &issueBudgetStatementLog{TxStarter: f.pool}
	round := NewIssueBudgetService(f.q, log).reconcileLifecycleRoots(ctx, f.roundRows(one, off, damaged, two), time.Minute)
	if want := (IssueBudgetLifecycleRound{Roots: 4, Closed: 3, Skipped: 2}); round != want {
		t.Fatalf("the round reports %+v, want %+v", round, want)
	}
	if log.transactions() != 4 {
		t.Fatalf("the round opened %d transactions for 4 roots", log.transactions())
	}
	f.requireLifecycleHeld(t, one, "process_exit", "the first root")
	f.requireLifecycleAttempt(t, off, "open", "the root that is off")
	f.requireLifecycleAttempt(t, damaged, "open", "the root whose ledger does not load")
	if closed := f.fx.Count(t, `SELECT count(*) FROM issue_budget_attempt WHERE budget_root_id=$1 AND state IN ('cancelled','lost_contact')`, two.root); closed != 2 {
		t.Fatalf("the root with two ended attempts has %d closed attempts, want 2", closed)
	}
	f.requireRootLedger(t, two.root, 4, issueBudgetLedgerFigures{UnknownHoldTicks: 200, UnknownCount: 2}, "the root with two ended attempts")

	lines := logs()
	if failures := issueBudgetRoundFailures(lines); len(failures) != 0 {
		t.Fatalf("the round logged a failure for a root it skipped: %q", failures)
	}
	var errorLines []string
	for _, line := range lines {
		if strings.Contains(line, "level=ERROR") {
			errorLines = append(errorLines, line)
		}
	}
	requireIssueBudgetLogLine(t, errorLines, "ERROR", []string{"budget_root_id=" + damaged.root}, "the root whose ledger does not load")
}

// issueBudgetLoseCommitOf is a TxStarter whose nth transaction is rolled back
// where it was to commit and says so. The others commit.
type issueBudgetLoseCommitOf struct {
	TxStarter
	nth   int
	begun int
}

func (s *issueBudgetLoseCommitOf) Begin(ctx context.Context) (pgx.Tx, error) {
	s.begun++
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil || s.begun != s.nth {
		return tx, err
	}
	return issueBudgetLostCommitTx{Tx: tx}, nil
}

// R05 (10.3, the trigger). A root that fails does not stop the roots after
// it. Here the first root does not exist and the third loses its transaction
// at the commit: both are counted and logged at warn level with the root's
// id, the third keeps every row it had, and the second and the fourth are
// reconciled. The next round closes what the failed one could not.
func TestIssueBudgetLifecycleRoundGoesOnAfterARootThatFails(t *testing.T) {
	f := newIssueBudgetFixture(t)
	ctx := context.Background()
	agent, runtime := f.agent(t)
	logs := issueBudgetClaimLog(t)
	var claims []issueBudgetLifecycleClaim
	for _, name := range []string{"before", "lost", "after"} {
		c := f.lifecycleClaim(t, agent, runtime, name)
		f.endTask(t, c.task, "completed", nil)
		claims = append(claims, c)
	}
	before, lost, after := claims[0], claims[1], claims[2]
	unknown := uuid.NewString()
	rows := append([]db.ListIssueBudgetRootsWithEndedAttemptsRow{{ID: util.MustParseUUID(unknown), WorkspaceID: f.ws}}, f.roundRows(before, lost, after)...)
	logs()

	failing := &issueBudgetLoseCommitOf{TxStarter: f.pool, nth: 3}
	round := NewIssueBudgetService(f.q, failing).reconcileLifecycleRoots(ctx, rows, time.Minute)
	if want := (IssueBudgetLifecycleRound{Roots: 4, Closed: 2, Failed: 2}); round != want {
		t.Fatalf("the round reports %+v, want %+v", round, want)
	}
	if failing.begun != 4 {
		t.Fatalf("the round opened %d transactions for 4 roots", failing.begun)
	}
	f.requireLifecycleHeld(t, before, "process_exit", "the root before the lost transaction")
	f.requireLifecycleHeld(t, after, "process_exit", "the root after the lost transaction")
	f.requireLifecycleAttempt(t, lost, "open", "the root that lost its transaction")
	f.requireRootLedger(t, lost.root, 1, issueBudgetLedgerFigures{ReservedTicks: 100}, "the root that lost its transaction")
	failures := issueBudgetRoundFailures(logs())
	if len(failures) != 2 {
		t.Fatalf("the round logged %d failed roots, want 2: %q", len(failures), failures)
	}
	requireIssueBudgetLogLine(t, failures[:1], "WARN", []string{"budget_root_id=" + unknown}, "the root that does not exist")
	requireIssueBudgetLogLine(t, failures[1:], "WARN", []string{"budget_root_id=" + lost.root}, "the root that lost its transaction")

	// The next round: the two reconciled roots have nothing open, the third
	// closes now.
	again := NewIssueBudgetService(f.q, f.pool).reconcileLifecycleRoots(ctx, f.roundRows(before, lost, after), time.Minute)
	if want := (IssueBudgetLifecycleRound{Roots: 3, Closed: 1}); again != want {
		t.Fatalf("the next round reports %+v, want %+v", again, want)
	}
	f.requireLifecycleHeld(t, lost, "process_exit", "the root that lost its transaction, a round later")
	if failures := issueBudgetRoundFailures(logs()); len(failures) != 0 {
		t.Fatalf("the next round logged %q", failures)
	}
}

// R06 (ruling B-3). Every root has its own time. A root whose lock another
// transaction holds runs out of it, is counted and logged as failed, and the
// root after it is reconciled all the same. Nothing of the first root is
// changed; once its lock is free, a later round closes its attempt.
func TestIssueBudgetLifecycleRoundDoesNotWaitForAContendedRootLock(t *testing.T) {
	f := newIssueBudgetFixture(t)
	ctx := context.Background()
	agent, runtime := f.agent(t)
	logs := issueBudgetClaimLog(t)
	held := f.lifecycleClaim(t, agent, runtime, "held")
	f.endTask(t, held.task, "completed", nil)
	free := f.lifecycleClaim(t, agent, runtime, "free")
	f.endTask(t, free.task, "completed", nil)

	holder, err := f.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer holder.Rollback(ctx)
	if _, err := holder.Exec(ctx, `SELECT id FROM issue_budget_root WHERE id=$1 FOR UPDATE`, held.root); err != nil {
		t.Fatalf("hold the root lock: %v", err)
	}
	logs()

	const each = 2 * time.Second
	started := time.Now()
	round := NewIssueBudgetService(f.q, f.pool).reconcileLifecycleRoots(ctx, f.roundRows(held, free), each)
	if took := time.Since(started); took < each || took > 10*each {
		t.Fatalf("the round took %s: the root whose lock is held was to take its %s and no more", took, each)
	}
	if want := (IssueBudgetLifecycleRound{Roots: 2, Closed: 1, Failed: 1}); round != want {
		t.Fatalf("the round reports %+v, want %+v", round, want)
	}
	f.requireLifecycleHeld(t, free, "process_exit", "the root after the contended one")
	failures := issueBudgetRoundFailures(logs())
	requireIssueBudgetLogLine(t, failures, "WARN", []string{"budget_root_id=" + held.root}, "the root whose lock is held")
	if !strings.Contains(failures[0], "deadline") {
		t.Fatalf("the failure does not say that the root ran out of its time: %s", failures[0])
	}

	if err := holder.Rollback(ctx); err != nil {
		t.Fatalf("release the root lock: %v", err)
	}
	f.requireLifecycleAttempt(t, held, "open", "the root whose lock was held")
	f.requireRootLedger(t, held.root, 1, issueBudgetLedgerFigures{ReservedTicks: 100}, "the root whose lock was held")
	later := NewIssueBudgetService(f.q, f.pool).reconcileLifecycleRoots(ctx, f.roundRows(held), time.Minute)
	if want := (IssueBudgetLifecycleRound{Roots: 1, Closed: 1}); later != want {
		t.Fatalf("the round after the lock was released reports %+v, want %+v", later, want)
	}
	f.requireLifecycleHeld(t, held, "process_exit", "the root whose lock was held, a round later")
}

// R07 (ruling B-3). The loop stops between two roots once its context is
// done. A round that starts stopped opens no transaction. A round that is
// stopped while the first root commits leaves that root as it was, does not
// begin the second, and reports neither as failed: being stopped says nothing
// about a root.
func TestIssueBudgetLifecycleRoundStopsWithItsContext(t *testing.T) {
	f := newIssueBudgetFixture(t)
	agent, runtime := f.agent(t)
	logs := issueBudgetClaimLog(t)
	first := f.lifecycleClaim(t, agent, runtime, "first")
	f.endTask(t, first.task, "completed", nil)
	second := f.lifecycleClaim(t, agent, runtime, "second")
	f.endTask(t, second.task, "completed", nil)
	logs()

	stopped, cancel := context.WithCancel(context.Background())
	cancel()
	log := &issueBudgetStatementLog{TxStarter: f.pool}
	round := NewIssueBudgetService(f.q, log).reconcileLifecycleRoots(stopped, f.roundRows(first, second), time.Minute)
	if want := (IssueBudgetLifecycleRound{Roots: 2}); round != want || log.transactions() != 0 {
		t.Fatalf("a round that starts stopped reports %+v after %d transactions, want %+v and none", round, log.transactions(), want)
	}

	stopping, cancel := context.WithCancel(context.Background())
	defer cancel()
	log = &issueBudgetStatementLog{TxStarter: f.pool, beforeCommit: func(uint32) { cancel() }}
	round = NewIssueBudgetService(f.q, log).reconcileLifecycleRoots(stopping, f.roundRows(first, second), time.Minute)
	if want := (IssueBudgetLifecycleRound{Roots: 2}); round != want || log.transactions() != 1 {
		t.Fatalf("a round stopped at the first commit reports %+v after %d transactions, want %+v and one", round, log.transactions(), want)
	}
	for name, c := range map[string]issueBudgetLifecycleClaim{"the first root": first, "the second root": second} {
		f.requireLifecycleAttempt(t, c, "open", name+" of a round that was stopped")
		f.requireRootLedger(t, c.root, 1, issueBudgetLedgerFigures{ReservedTicks: 100}, name+" of a round that was stopped")
	}
	if failures := issueBudgetRoundFailures(logs()); len(failures) != 0 {
		t.Fatalf("a round that was stopped logged a failed root: %q", failures)
	}
}
