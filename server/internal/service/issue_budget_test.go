package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/multica-ai/multica/server/internal/budget"
	"github.com/multica-ai/multica/server/internal/iteration"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// Rows of the F3 acceptance matrix covered here: B01-B08 (the parts that need
// no attempt), B14 (service half), B59, the binding rules behind B10-B11, and
// the collaboration paths of B60 (C7, C8, S5). The handler suite owns the
// rows that go through an HTTP write path: B04's wiring, B09-B13, B56, B57 and
// the remaining B60 paths.

var issueBudgetTables = []string{
	"issue_budget_root", "issue_budget_member", "issue_budget_attempt",
	"issue_budget_entry", "issue_budget_operation", "issue_budget_observation",
}

type issueBudgetFixture struct {
	pool      *pgxpool.Pool
	fx        *testutil.Fixture
	q         *db.Queries
	svc       *IssueBudgetService
	issues    *IssueService
	workspace string
	user      string
	ws        pgtype.UUID
	actor     pgtype.UUID
}

func newIssueBudgetFixture(t *testing.T) *issueBudgetFixture {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("requires task-owned DATABASE_URL")
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	fx := testutil.New(pool, "", "")
	suffix := strings.ReplaceAll(uuid.NewString(), "-", "")
	fx.UserID = fx.User(t, "Budget operator", "budget-"+suffix+"@example.test")
	fx.WorkspaceID = fx.Workspace(t, "Budget workspace", "budget-"+suffix)
	fx.Member(t, fx.WorkspaceID, fx.UserID, "owner")
	issueBudgetCleanWorkspace(t, fx, fx.WorkspaceID)
	q := db.New(pool)
	return &issueBudgetFixture{
		pool: pool, fx: fx, q: q,
		svc:       NewIssueBudgetService(q, pool),
		issues:    NewIssueService(q, pool, nil, nil, nil),
		workspace: fx.WorkspaceID, user: fx.UserID,
		ws: util.MustParseUUID(fx.WorkspaceID), actor: util.MustParseUUID(fx.UserID),
	}
}

// issueBudgetCleanWorkspace removes what the services under test wrote for a
// fixture workspace. Budget rows have no foreign key, so nothing else removes
// them; issues created through the real create path are not fixture rows.
func issueBudgetCleanWorkspace(t *testing.T, fx *testutil.Fixture, workspace string) {
	t.Helper()
	fx.Cleanup(t, `DELETE FROM issue WHERE workspace_id=$1`, workspace)
	for _, table := range issueBudgetTables {
		fx.Cleanup(t, `DELETE FROM `+table+` WHERE workspace_id=$1`, workspace)
	}
}

func (f *issueBudgetFixture) issue(t *testing.T, title, parent string, over ...testutil.Cols) string {
	t.Helper()
	cols := testutil.Cols{}
	if parent != "" {
		cols["parent_issue_id"] = parent
	}
	return f.fx.Issue(t, title, append([]testutil.Cols{cols}, over...)...)
}

func (f *issueBudgetFixture) setParent(t *testing.T, issue, parent string) db.Issue {
	t.Helper()
	if parent == "" {
		f.fx.Exec(t, `UPDATE issue SET parent_issue_id=NULL WHERE id=$1`, issue)
	} else {
		f.fx.Exec(t, `UPDATE issue SET parent_issue_id=$2 WHERE id=$1`, issue, parent)
	}
	row, err := f.q.GetIssueInWorkspace(context.Background(), db.GetIssueInWorkspaceParams{ID: util.MustParseUUID(issue), WorkspaceID: f.ws})
	if err != nil {
		t.Fatalf("read moved issue: %v", err)
	}
	return row
}

func (f *issueBudgetFixture) enableParams(issue string) IssueBudgetEnableParams {
	return IssueBudgetEnableParams{
		WorkspaceID: f.ws, IssueID: util.MustParseUUID(issue),
		OperationKey: "enable-" + uuid.NewString(), Reason: "track this parent task", ActorUserID: f.actor,
	}
}

func (f *issueBudgetFixture) enable(t *testing.T, issue string) IssueBudgetOperationResult {
	t.Helper()
	result, err := f.svc.Enable(context.Background(), f.enableParams(issue))
	if err != nil {
		t.Fatalf("enable budget root: %v", err)
	}
	return result
}

func (f *issueBudgetFixture) manage(root string, revision int64) IssueBudgetManageParams {
	return IssueBudgetManageParams{
		WorkspaceID: f.ws, BudgetRootID: util.MustParseUUID(root),
		OperationKey: "manage-" + uuid.NewString(), ExpectedPolicyRevision: revision,
		Reason: "operator decision", ActorUserID: f.actor,
	}
}

// rows is every row of one budget table in the fixture workspace, as one
// stable JSON text.
func issueBudgetRows(t *testing.T, fx *testutil.Fixture, table, workspace string) string {
	t.Helper()
	var out string
	fx.QueryRow(t, `SELECT COALESCE(jsonb_agg(to_jsonb(x) ORDER BY to_jsonb(x)::text), '[]'::jsonb)::text FROM `+table+` x WHERE workspace_id=$1`, workspace).Scan(&out)
	return out
}

// snapshot is every budget row of the fixture workspace.
func (f *issueBudgetFixture) snapshot(t *testing.T) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, table := range issueBudgetTables {
		out[table] = issueBudgetRows(t, f.fx, table, f.workspace)
	}
	return out
}

func (f *issueBudgetFixture) requireUnchanged(t *testing.T, before map[string]string, what string) {
	t.Helper()
	after := f.snapshot(t)
	for _, table := range issueBudgetTables {
		if before[table] != after[table] {
			t.Fatalf("%s changed %s\nbefore: %s\nafter:  %s", what, table, before[table], after[table])
		}
	}
}

func (f *issueBudgetFixture) count(t *testing.T, table string) int {
	t.Helper()
	return f.fx.Count(t, `SELECT count(*) FROM `+table+` WHERE workspace_id=$1`, f.workspace)
}

type issueBudgetMemberRow struct {
	Root, BoundParent, Source string
	Version                   int32
	Tombstoned                bool
}

// member reads an issue's member row; ok is false when it has none.
func (f *issueBudgetFixture) member(t *testing.T, issue string) (row issueBudgetMemberRow, ok bool) {
	t.Helper()
	var parent *string
	err := f.pool.QueryRow(context.Background(), `
		SELECT budget_root_id::text, bound_parent_issue_id::text, bind_source, binding_version, tombstoned_at IS NOT NULL
		FROM issue_budget_member WHERE workspace_id=$1 AND issue_id=$2`, f.workspace, issue).
		Scan(&row.Root, &parent, &row.Source, &row.Version, &row.Tombstoned)
	if errors.Is(err, pgx.ErrNoRows) {
		return row, false
	}
	if err != nil {
		t.Fatalf("read member row: %v", err)
	}
	if parent != nil {
		row.BoundParent = *parent
	}
	return row, true
}

func (f *issueBudgetFixture) requireMember(t *testing.T, issue string, want issueBudgetMemberRow) {
	t.Helper()
	got, ok := f.member(t, issue)
	if !ok {
		t.Fatalf("issue %s has no member row, want %+v", issue, want)
	}
	if got != want {
		t.Fatalf("member row of %s = %+v, want %+v", issue, got, want)
	}
}

type issueBudgetObservationRow struct {
	Kind, Reason, Issue, Task string
	Generation                int64
}

// observations lists a root's observations as comparable values, in the
// order they read back.
func (f *issueBudgetFixture) observations(t *testing.T, root string) []issueBudgetObservationRow {
	t.Helper()
	rows, err := f.svc.ListObservations(context.Background(), f.ws, util.MustParseUUID(root))
	if err != nil {
		t.Fatalf("list observations: %v", err)
	}
	out := make([]issueBudgetObservationRow, 0, len(rows))
	for _, row := range rows {
		item := issueBudgetObservationRow{Kind: row.Row.Kind, Reason: row.Row.Reason, Generation: -1}
		if row.Row.IssueID.Valid {
			item.Issue = util.UUIDToString(row.Row.IssueID)
		}
		if row.Row.TaskID.Valid {
			item.Task = util.UUIDToString(row.Row.TaskID)
		}
		if row.Row.ClaimGeneration.Valid {
			item.Generation = row.Row.ClaimGeneration.Int64
		}
		out = append(out, item)
	}
	return out
}

func requireIssueBudgetCode(t *testing.T, err error, code, what string) {
	t.Helper()
	if IssueBudgetErrorCode(err) != code {
		t.Fatalf("%s: got %v, want refusal %q", what, err, code)
	}
}

// agent returns an agent on an online runtime in the fixture workspace.
func (f *issueBudgetFixture) agent(t *testing.T) (agent, runtime string) {
	t.Helper()
	runtime = f.fx.Runtime(t, "budget-runtime-"+uuid.NewString())
	return f.fx.Agent(t, "budget-agent-"+uuid.NewString(), runtime, testutil.Cols{"max_concurrent_tasks": 20}), runtime
}

func (f *issueBudgetFixture) task(t *testing.T, agent, runtime, issue, status string, generation int64) string {
	t.Helper()
	cols := testutil.Cols{"runtime_id": runtime, "status": status, "claim_generation": generation}
	if issue != "" {
		cols["issue_id"] = issue
	}
	return f.fx.Task(t, agent, cols)
}

func (f *issueBudgetFixture) quickCreateTask(t *testing.T, agent, runtime, parent, status string, generation int64, over ...testutil.Cols) string {
	t.Helper()
	contextJSON, err := json.Marshal(QuickCreateContext{
		Type: QuickCreateContextType, Prompt: "file a sub-issue", RequesterID: f.user,
		WorkspaceID: f.workspace, ParentIssueID: parent,
	})
	if err != nil {
		t.Fatal(err)
	}
	return f.fx.Task(t, agent, append([]testutil.Cols{{
		"runtime_id": runtime, "status": status, "claim_generation": generation, "context": string(contextJSON),
	}}, over...)...)
}

// holdFence takes the workspace fence on its own connection and returns the
// backend that holds it and the function that lets go.
func (f *issueBudgetFixture) holdFence(t *testing.T) (holder uint32, release func()) {
	t.Helper()
	ctx := context.Background()
	tx, err := f.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := iteration.LockWorkspace(ctx, tx, f.ws); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	var once sync.Once
	release = func() { once.Do(func() { _ = tx.Rollback(ctx) }) }
	t.Cleanup(release)
	return tx.Conn().PgConn().PID(), release
}

// waitBlocked returns once n backends wait on a lock held by holder.
func (f *issueBudgetFixture) waitBlocked(t *testing.T, holder uint32, n int) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		var blocked int
		if err := f.pool.QueryRow(context.Background(),
			`SELECT count(*) FROM pg_stat_activity WHERE datname=current_database() AND $1::integer=ANY(pg_blocking_pids(pid))`,
			int32(holder)).Scan(&blocked); err != nil {
			t.Fatal(err)
		}
		if blocked >= n {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("%d competing transactions never reached the workspace fence", n)
}

// issueBudgetStatementLog records every statement a transaction sends, by its
// sqlc name where it has one.
type issueBudgetStatementLog struct {
	TxStarter
	mu         sync.Mutex
	statements []string
	// begun counts the transactions opened through this log.
	begun int
	// beforeCommit, when set, runs with the transaction still open.
	beforeCommit func(pid uint32)
}

func (l *issueBudgetStatementLog) Begin(ctx context.Context) (pgx.Tx, error) {
	l.mu.Lock()
	l.begun++
	l.mu.Unlock()
	tx, err := l.TxStarter.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return issueBudgetLoggedTx{Tx: tx, log: l}, nil
}

func (l *issueBudgetStatementLog) record(sql string) {
	name := strings.TrimSpace(sql)
	if rest, ok := strings.CutPrefix(name, "-- name: "); ok {
		name, _, _ = strings.Cut(rest, " ")
	}
	l.mu.Lock()
	l.statements = append(l.statements, name)
	l.mu.Unlock()
}

func (l *issueBudgetStatementLog) names() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string(nil), l.statements...)
}

func (l *issueBudgetStatementLog) transactions() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.begun
}

type issueBudgetLoggedTx struct {
	pgx.Tx
	log *issueBudgetStatementLog
}

func (tx issueBudgetLoggedTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	tx.log.record(sql)
	return tx.Tx.Exec(ctx, sql, args...)
}

func (tx issueBudgetLoggedTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	tx.log.record(sql)
	return tx.Tx.Query(ctx, sql, args...)
}

func (tx issueBudgetLoggedTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	tx.log.record(sql)
	return tx.Tx.QueryRow(ctx, sql, args...)
}

func (tx issueBudgetLoggedTx) Commit(ctx context.Context) error {
	if tx.log.beforeCommit != nil {
		tx.log.beforeCommit(tx.Tx.Conn().PgConn().PID())
	}
	return tx.Tx.Commit(ctx)
}

// inBinding runs fn the way an issue writer calls the binding functions: in
// its own transaction, under the workspace fence. It returns the statements
// fn sent.
func (f *issueBudgetFixture) inBinding(t *testing.T, fn func(q *db.Queries) error) []string {
	t.Helper()
	ctx := context.Background()
	tx, err := f.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if err := iteration.LockWorkspace(ctx, tx, f.ws); err != nil {
		t.Fatal(err)
	}
	log := &issueBudgetStatementLog{}
	if err := fn(db.New(issueBudgetLoggedTx{Tx: tx, log: log})); err != nil {
		t.Fatalf("binding: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	return log.names()
}

// createChild creates an issue through IssueService.CreateInTx, the one
// function every parent-capable create entry goes through.
func (f *issueBudgetFixture) createChild(t *testing.T, title, parent string) db.Issue {
	t.Helper()
	ctx := context.Background()
	f.syncIssueCounter(t)
	tx, err := f.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	p := IssueCreateParams{WorkspaceID: f.ws, Title: title, Status: "todo", Priority: "none", CreatorType: "member", CreatorID: f.actor, AllowDuplicate: true}
	if parent != "" {
		p.ParentIssueID = util.MustParseUUID(parent)
	}
	result, err := f.issues.CreateInTx(ctx, tx, p, IssueCountPolicy{})
	if err != nil {
		t.Fatalf("create issue: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	return result.Issue
}

// syncIssueCounter moves the workspace counter past the numbers fixture
// issues took: fixtures number themselves and leave the counter alone.
func (f *issueBudgetFixture) syncIssueCounter(t *testing.T) {
	t.Helper()
	f.fx.Exec(t, `UPDATE workspace SET issue_counter=(SELECT COALESCE(MAX(number),0) FROM issue WHERE workspace_id=$1) WHERE id=$1`, f.workspace)
}

// B01: one parent task has at most one root. The same original key with the
// same intent gets the committed result back; anything else is refused.
func TestIssueBudgetEnableIsOnePerIssueAndReplaysByOriginalKey(t *testing.T) {
	f := newIssueBudgetFixture(t)
	ctx := context.Background()
	parent := f.issue(t, "Parent task", "")
	limit := budget.Ticks(5_000_000_000)
	p := f.enableParams(parent)
	p.LimitTicks = &limit

	first, err := f.svc.Enable(ctx, p)
	if err != nil {
		t.Fatalf("enable: %v", err)
	}
	if first.Replayed || first.OperationKind != IssueBudgetOperationEnable || first.RootIssueID != parent ||
		first.AdmissionState != IssueBudgetAdmissionOpen || first.Mode != IssueBudgetModeObserve ||
		first.PolicyRevision != 1 || first.PolicyEpoch != 1 || first.LimitTicks == nil || *first.LimitTicks != limit ||
		first.MemberCount != 1 || first.ActivatedAt == "" {
		t.Fatalf("enable result = %+v", first)
	}
	f.requireMember(t, parent, issueBudgetMemberRow{Root: first.BudgetRootID, Source: "root", Version: 1})
	committed := f.snapshot(t)

	replay, err := f.svc.Enable(ctx, p)
	if err != nil {
		t.Fatalf("replay with the original key and intent: %v", err)
	}
	if !replay.Replayed {
		t.Fatal("a replay must say that it wrote nothing")
	}
	replay.Replayed = false
	if !reflect.DeepEqual(replay, first) {
		t.Fatalf("replay = %+v, want the committed result %+v", replay, first)
	}

	changed := p
	other := budget.Ticks(7)
	changed.LimitTicks = &other
	_, err = f.svc.Enable(ctx, changed)
	requireIssueBudgetCode(t, err, IssueBudgetOperationConflict, "original key with another limit")
	changed = p
	changed.Reason = "a different reason"
	_, err = f.svc.Enable(ctx, changed)
	requireIssueBudgetCode(t, err, IssueBudgetOperationConflict, "original key with another reason")

	fresh := f.enableParams(parent)
	fresh.LimitTicks = &limit
	_, err = f.svc.Enable(ctx, fresh)
	requireIssueBudgetCode(t, err, IssueBudgetAlreadyEnabled, "second enable with a new key")

	f.requireUnchanged(t, committed, "a replayed or refused enable")
	if got := f.count(t, "issue_budget_root"); got != 1 {
		t.Fatalf("%d root rows, want 1", got)
	}
}

// B01: two transactions enabling the same parent at once. Both wait on the
// workspace fence; whichever runs second finds the first one's root.
func TestIssueBudgetEnableConcurrentlyOnOneIssue(t *testing.T) {
	for _, tc := range []struct {
		name      string
		sameKey   bool
		sameLimit bool
		loser     string
	}{
		{name: "same key and intent", sameKey: true, sameLimit: true},
		{name: "same key, other intent", sameKey: true, loser: IssueBudgetOperationConflict},
		{name: "other key", sameLimit: true, loser: IssueBudgetAlreadyEnabled},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newIssueBudgetFixture(t)
			parent := f.issue(t, "Contended parent", "")
			a := f.enableParams(parent)
			b := f.enableParams(parent)
			if tc.sameKey {
				b.OperationKey = a.OperationKey
			}
			if !tc.sameLimit {
				limit := budget.Ticks(9)
				b.LimitTicks = &limit
			}

			holder, release := f.holdFence(t)
			type outcome struct {
				result IssueBudgetOperationResult
				err    error
			}
			outcomes := make(chan outcome, 2)
			for _, p := range []IssueBudgetEnableParams{a, b} {
				go func() {
					result, err := f.svc.Enable(context.Background(), p)
					outcomes <- outcome{result, err}
				}()
			}
			f.waitBlocked(t, holder, 2)
			release()
			first, second := <-outcomes, <-outcomes

			if got := f.count(t, "issue_budget_root"); got != 1 {
				t.Fatalf("%d root rows, want exactly 1", got)
			}
			if got := f.count(t, "issue_budget_member"); got != 1 {
				t.Fatalf("%d member rows, want exactly 1", got)
			}
			if got := f.count(t, "issue_budget_operation"); got != 1 {
				t.Fatalf("%d operation rows, want exactly 1", got)
			}
			if tc.loser == "" {
				if first.err != nil || second.err != nil {
					t.Fatalf("same key and intent: %v / %v", first.err, second.err)
				}
				if first.result.BudgetRootID != second.result.BudgetRootID || first.result.OperationID != second.result.OperationID {
					t.Fatalf("the two callers got different roots: %+v / %+v", first.result, second.result)
				}
				if first.result.Replayed == second.result.Replayed {
					t.Fatalf("exactly one caller must have written: replayed %v / %v", first.result.Replayed, second.result.Replayed)
				}
				return
			}
			winner, loser := first, second
			if winner.err != nil {
				winner, loser = second, first
			}
			if winner.err != nil {
				t.Fatalf("neither caller enabled the root: %v / %v", first.err, second.err)
			}
			requireIssueBudgetCode(t, loser.err, tc.loser, "the caller that ran second")
		})
	}
}

// B02: a parent and its child enabling at once. They serialize on the fence,
// and the one that runs second finds the other's member rows.
func TestIssueBudgetEnableParentAndChildConcurrently(t *testing.T) {
	f := newIssueBudgetFixture(t)
	parent := f.issue(t, "Parent", "")
	child := f.issue(t, "Child", parent)

	holder, release := f.holdFence(t)
	type outcome struct {
		issue  string
		result IssueBudgetOperationResult
		err    error
	}
	outcomes := make(chan outcome, 2)
	for _, issue := range []string{parent, child} {
		go func() {
			result, err := f.svc.Enable(context.Background(), f.enableParams(issue))
			outcomes <- outcome{issue, result, err}
		}()
	}
	f.waitBlocked(t, holder, 2)
	release()
	first, second := <-outcomes, <-outcomes

	winner, loser := first, second
	if winner.err != nil {
		winner, loser = second, first
	}
	if winner.err != nil {
		t.Fatalf("neither enable committed: %v / %v", first.err, second.err)
	}
	requireIssueBudgetCode(t, loser.err, IssueBudgetRootConflict, "the enable that ran second")
	if got := f.count(t, "issue_budget_root"); got != 1 {
		t.Fatalf("%d root rows, want exactly 1", got)
	}
	if got := f.fx.Count(t, `SELECT count(*) FROM issue_budget_member WHERE workspace_id=$1 AND issue_id=$2`, f.workspace, child); got != 1 {
		t.Fatalf("the child has %d member rows, want exactly 1", got)
	}
	member, _ := f.member(t, child)
	if member.Root != winner.result.BudgetRootID {
		t.Fatalf("the child belongs to root %s, want the committed root %s", member.Root, winner.result.BudgetRootID)
	}
}

// B02, sequentially and in both orders: neither a member nor an ancestor of a
// member can become a second root, and a refusal writes nothing.
func TestIssueBudgetEnableRefusesOverlappingRoots(t *testing.T) {
	f := newIssueBudgetFixture(t)
	ctx := context.Background()
	top := f.issue(t, "Top", "")
	middle := f.issue(t, "Middle", top)
	leaf := f.issue(t, "Leaf", middle)

	f.enable(t, middle)
	committed := f.snapshot(t)
	_, err := f.svc.Enable(ctx, f.enableParams(leaf))
	requireIssueBudgetCode(t, err, IssueBudgetRootConflict, "enable on a member of another root")
	_, err = f.svc.Enable(ctx, f.enableParams(top))
	requireIssueBudgetCode(t, err, IssueBudgetRootConflict, "enable on an ancestor of a root")
	f.requireUnchanged(t, committed, "a refused enable")
}

// B03: closing a root does not free the slot. Reopen is the same root with a
// new policy revision; the limit and the amounts stay as they were.
func TestIssueBudgetCloseAndReopenKeepTheSameRoot(t *testing.T) {
	f := newIssueBudgetFixture(t)
	ctx := context.Background()
	parent := f.issue(t, "Parent", "")
	limit := budget.Ticks(1_000)
	p := f.enableParams(parent)
	p.LimitTicks = &limit
	enabled, err := f.svc.Enable(ctx, p)
	if err != nil {
		t.Fatal(err)
	}
	// Amounts a ledger would have projected by now. The rules under test
	// must carry them through untouched.
	f.fx.Exec(t, `UPDATE issue_budget_root SET actual_known_ticks=30, estimated_ticks=4, reserved_ticks=20, unknown_hold_ticks=7, overrun_ticks=1, unknown_count=2, ledger_revision=5 WHERE id=$1`, enabled.BudgetRootID)
	amounts := func() string {
		var out string
		f.fx.QueryRow(t, `SELECT jsonb_build_array(limit_ticks, actual_known_ticks, estimated_ticks, reserved_ticks, unknown_hold_ticks, overrun_ticks, unknown_count, ledger_revision, settled_as_of_revision, activated_at)::text FROM issue_budget_root WHERE id=$1`, enabled.BudgetRootID).Scan(&out)
		return out
	}
	before := amounts()

	closed, err := f.svc.Close(ctx, f.manage(enabled.BudgetRootID, 1))
	if err != nil {
		t.Fatalf("close: %v", err)
	}
	if closed.BudgetRootID != enabled.BudgetRootID || closed.AdmissionState != IssueBudgetAdmissionClosed || closed.PolicyRevision != 2 || closed.PolicyEpoch != 1 {
		t.Fatalf("close result = %+v", closed)
	}
	if amounts() != before {
		t.Fatalf("close moved an amount: %s -> %s", before, amounts())
	}

	_, err = f.svc.Enable(ctx, f.enableParams(parent))
	requireIssueBudgetCode(t, err, IssueBudgetAlreadyEnabled, "enable on a closed root's issue")
	replay, err := f.svc.Enable(ctx, p)
	if err != nil || !replay.Replayed || replay.BudgetRootID != enabled.BudgetRootID || replay.PolicyRevision != 1 {
		t.Fatalf("the original enable key must still return what was committed: %+v %v", replay, err)
	}
	_, err = f.svc.Close(ctx, f.manage(enabled.BudgetRootID, 2))
	requireIssueBudgetCode(t, err, IssueBudgetStateConflict, "close of a closed root")

	reopened, err := f.svc.Reopen(ctx, f.manage(enabled.BudgetRootID, 2))
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if reopened.BudgetRootID != enabled.BudgetRootID || reopened.AdmissionState != IssueBudgetAdmissionOpen ||
		reopened.PolicyRevision != 3 || reopened.PolicyEpoch != 1 || reopened.LimitTicks == nil || *reopened.LimitTicks != limit {
		t.Fatalf("reopen result = %+v", reopened)
	}
	if amounts() != before {
		t.Fatalf("reopen moved an amount: %s -> %s", before, amounts())
	}
	_, err = f.svc.Reopen(ctx, f.manage(enabled.BudgetRootID, 3))
	requireIssueBudgetCode(t, err, IssueBudgetStateConflict, "reopen of an open root")
	if got := f.count(t, "issue_budget_root"); got != 1 {
		t.Fatalf("%d root rows, want 1", got)
	}
	state, err := f.svc.GetRoot(ctx, f.ws, util.MustParseUUID(enabled.BudgetRootID))
	if err != nil || state.Admission() != IssueBudgetAdmissionOpen || state.Root.PolicyRevision != 3 {
		t.Fatalf("root after reopen: %+v %v", state, err)
	}
}

// B03 and B13 at the service: once the root issue is deleted the root reads
// as tombstoned, the slot stays taken, surviving members cannot get a root of
// their own, and reopen is refused without leaving a write.
func TestIssueBudgetDeletedRootIssueKeepsTheSlot(t *testing.T) {
	f := newIssueBudgetFixture(t)
	ctx := context.Background()
	parent := f.issue(t, "Parent", "")
	child := f.issue(t, "Child", parent)
	enabled := f.enable(t, parent)
	rootID := util.MustParseUUID(enabled.BudgetRootID)
	if _, err := f.svc.Close(ctx, f.manage(enabled.BudgetRootID, 1)); err != nil {
		t.Fatal(err)
	}

	// What the delete transaction does: mark the member row, detach the
	// children, delete the issue.
	f.inBinding(t, func(q *db.Queries) error {
		return TombstoneIssueBudgetMember(ctx, q, f.ws, util.MustParseUUID(parent))
	})
	f.fx.Exec(t, `UPDATE issue SET parent_issue_id=NULL WHERE id=$1`, child)
	f.fx.Exec(t, `DELETE FROM issue WHERE id=$1`, parent)

	state, err := f.svc.GetRoot(ctx, f.ws, rootID)
	if err != nil || state.Admission() != IssueBudgetAdmissionTombstoned || !state.TombstonedAt.Valid || state.Root.AdmissionState != IssueBudgetAdmissionClosed {
		t.Fatalf("root after its issue was deleted: %+v %v", state, err)
	}
	byIssue, err := f.svc.GetRootByIssue(ctx, f.ws, util.MustParseUUID(parent))
	if err != nil || byIssue.Root.ID != rootID || byIssue.Admission() != IssueBudgetAdmissionTombstoned {
		t.Fatalf("root by its deleted issue: %+v %v", byIssue, err)
	}
	f.requireMember(t, child, issueBudgetMemberRow{Root: enabled.BudgetRootID, BoundParent: parent, Source: "activation", Version: 1})

	committed := f.snapshot(t)
	_, err = f.svc.Enable(ctx, f.enableParams(parent))
	requireIssueBudgetCode(t, err, IssueBudgetAlreadyEnabled, "enable on the deleted root issue")
	_, err = f.svc.Enable(ctx, f.enableParams(child))
	requireIssueBudgetCode(t, err, IssueBudgetRootConflict, "enable on a surviving member")
	_, err = f.svc.Reopen(ctx, f.manage(enabled.BudgetRootID, 2))
	requireIssueBudgetCode(t, err, IssueBudgetRootDeleted, "reopen of a root whose issue was deleted")
	f.requireUnchanged(t, committed, "a refused enable or reopen")
}

// B04: a child and a grandchild created under a root get their member rows in
// their own create transactions, with the parent they were inherited through.
func TestIssueBudgetCreateInheritsTheParentRoot(t *testing.T) {
	f := newIssueBudgetFixture(t)
	parent := f.issue(t, "Parent", "")
	rootless := f.issue(t, "Rootless parent", "")
	enabled := f.enable(t, parent)

	child := f.createChild(t, "Child", parent)
	grandchild := f.createChild(t, "Grandchild", util.UUIDToString(child.ID))
	f.requireMember(t, util.UUIDToString(child.ID), issueBudgetMemberRow{Root: enabled.BudgetRootID, BoundParent: parent, Source: "create", Version: 1})
	f.requireMember(t, util.UUIDToString(grandchild.ID), issueBudgetMemberRow{Root: enabled.BudgetRootID, BoundParent: util.UUIDToString(child.ID), Source: "create", Version: 1})

	// The binding is the saved row, not the current tree: detaching the
	// child afterwards leaves both rows as written.
	f.fx.Exec(t, `UPDATE issue SET parent_issue_id=NULL WHERE id=$1`, child.ID)
	f.requireMember(t, util.UUIDToString(child.ID), issueBudgetMemberRow{Root: enabled.BudgetRootID, BoundParent: parent, Source: "create", Version: 1})

	committed := f.snapshot(t)
	orphan := f.createChild(t, "No parent", "")
	stray := f.createChild(t, "Under a rootless parent", rootless)
	for _, issue := range []db.Issue{orphan, stray} {
		if _, ok := f.member(t, util.UUIDToString(issue.ID)); ok {
			t.Fatalf("issue %s outside every root got a member row", util.UUIDToString(issue.ID))
		}
	}
	f.requireUnchanged(t, committed, "creating issues outside every root")
}

// B05: enabling a root while a child is being created under it. In either
// order the child ends with exactly one member row on that root.
func TestIssueBudgetEnableRacingChildCreate(t *testing.T) {
	t.Run("create commits first", func(t *testing.T) {
		f := newIssueBudgetFixture(t)
		ctx := context.Background()
		parent := f.issue(t, "Parent", "")
		f.syncIssueCounter(t)

		// The create holds the fence with its insert uncommitted.
		tx, err := f.pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(ctx)
		created, err := f.issues.CreateInTx(ctx, tx, IssueCreateParams{WorkspaceID: f.ws, Title: "Phantom child", Status: "todo", Priority: "none",
			CreatorType: "member", CreatorID: f.actor, ParentIssueID: util.MustParseUUID(parent), AllowDuplicate: true}, IssueCountPolicy{})
		if err != nil {
			t.Fatal(err)
		}
		done := make(chan error, 1)
		var enabled IssueBudgetOperationResult
		go func() {
			var err error
			enabled, err = f.svc.Enable(context.Background(), f.enableParams(parent))
			done <- err
		}()
		f.waitBlocked(t, tx.Conn().PgConn().PID(), 1)
		if err := tx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
		if err := <-done; err != nil {
			t.Fatalf("enable: %v", err)
		}
		f.requireMember(t, util.UUIDToString(created.Issue.ID), issueBudgetMemberRow{Root: enabled.BudgetRootID, BoundParent: parent, Source: "activation", Version: 1})
		if enabled.MemberCount != 2 {
			t.Fatalf("enable bound %d members, want the parent and the new child", enabled.MemberCount)
		}
	})

	t.Run("enable commits first", func(t *testing.T) {
		f := newIssueBudgetFixture(t)
		parent := f.issue(t, "Parent", "")
		f.syncIssueCounter(t)

		// The enable is stopped just before its commit: fence held, root and
		// member rows written but invisible to everyone else.
		reached := make(chan uint32, 1)
		proceed := make(chan struct{})
		var once sync.Once
		release := func() { once.Do(func() { close(proceed) }) }
		ctx, cancel := context.WithCancel(context.Background())
		var racing sync.WaitGroup
		// A failed assertion must not leave the enable parked before its
		// commit: it holds a pooled connection, and closing the pool would wait
		// for it until the package timeout. Registered after the fixture, so it
		// runs before the pool is closed.
		t.Cleanup(func() {
			release()
			cancel()
			racing.Wait()
		})
		gated := NewIssueBudgetService(f.q, &issueBudgetStatementLog{TxStarter: f.pool, beforeCommit: func(pid uint32) {
			reached <- pid
			<-proceed
		}})
		enableDone := make(chan error, 1)
		var enabled IssueBudgetOperationResult
		racing.Go(func() {
			var err error
			enabled, err = gated.Enable(ctx, f.enableParams(parent))
			enableDone <- err
		})
		var holder uint32
		select {
		case holder = <-reached:
		case err := <-enableDone:
			t.Fatalf("enable ended before its commit: %v", err)
		case <-time.After(15 * time.Second):
			t.Fatal("enable never reached its commit")
		}

		createDone := make(chan error, 1)
		var created IssueCreateResult
		racing.Go(func() {
			tx, err := f.pool.Begin(ctx)
			if err != nil {
				createDone <- err
				return
			}
			defer tx.Rollback(ctx)
			created, err = f.issues.CreateInTx(ctx, tx, IssueCreateParams{WorkspaceID: f.ws, Title: "Phantom child", Status: "todo", Priority: "none",
				CreatorType: "member", CreatorID: f.actor, ParentIssueID: util.MustParseUUID(parent), AllowDuplicate: true}, IssueCountPolicy{})
			if err != nil {
				createDone <- err
				return
			}
			createDone <- tx.Commit(ctx)
		})
		f.waitBlocked(t, holder, 1)
		release()
		if err := <-enableDone; err != nil {
			t.Fatalf("enable: %v", err)
		}
		if err := <-createDone; err != nil {
			t.Fatalf("create: %v", err)
		}
		f.requireMember(t, util.UUIDToString(created.Issue.ID), issueBudgetMemberRow{Root: enabled.BudgetRootID, BoundParent: parent, Source: "create", Version: 1})
	})

	// Unforced order, several times: whoever gets the fence first, the
	// child has one member row on the parent's root.
	t.Run("either order", func(t *testing.T) {
		f := newIssueBudgetFixture(t)
		ctx := context.Background()
		for round := 0; round < 6; round++ {
			parent := f.issue(t, fmt.Sprintf("Parent %d", round), "")
			f.syncIssueCounter(t)
			holder, release := f.holdFence(t)
			errs := make(chan error, 2)
			var enabled IssueBudgetOperationResult
			var created IssueCreateResult
			go func() {
				var err error
				enabled, err = f.svc.Enable(context.Background(), f.enableParams(parent))
				errs <- err
			}()
			go func() {
				tx, err := f.pool.Begin(ctx)
				if err != nil {
					errs <- err
					return
				}
				defer tx.Rollback(ctx)
				created, err = f.issues.CreateInTx(ctx, tx, IssueCreateParams{WorkspaceID: f.ws, Title: fmt.Sprintf("Racing child %d", round), Status: "todo", Priority: "none",
					CreatorType: "member", CreatorID: f.actor, ParentIssueID: util.MustParseUUID(parent), AllowDuplicate: true}, IssueCountPolicy{})
				if err != nil {
					errs <- err
					return
				}
				errs <- tx.Commit(ctx)
			}()
			f.waitBlocked(t, holder, 2)
			release()
			for i := 0; i < 2; i++ {
				if err := <-errs; err != nil {
					t.Fatalf("round %d: %v", round, err)
				}
			}
			child := util.UUIDToString(created.Issue.ID)
			if got := f.fx.Count(t, `SELECT count(*) FROM issue_budget_member WHERE workspace_id=$1 AND issue_id=$2`, f.workspace, child); got != 1 {
				t.Fatalf("round %d: the child has %d member rows, want exactly 1", round, got)
			}
			member, _ := f.member(t, child)
			if member.Root != enabled.BudgetRootID || member.BoundParent != parent || (member.Source != "activation" && member.Source != "create") {
				t.Fatalf("round %d: member row %+v, want root %s through parent %s", round, member, enabled.BudgetRootID, parent)
			}
		}
	})
}

// B06: enabling freezes every descendant at once and records the tasks a
// daemon already holds in that range. Nothing else is recorded, and the
// root's amounts start at zero.
func TestIssueBudgetEnableFreezesDescendantsAndRecordsInFlightTasks(t *testing.T) {
	f := newIssueBudgetFixture(t)
	ctx := context.Background()
	parent := f.issue(t, "Parent", "")
	childA := f.issue(t, "Child A", parent)
	grandchild := f.issue(t, "Grandchild", childA)
	childB := f.issue(t, "Child B", parent)
	outside := f.issue(t, "Unrelated issue", "")
	agent, runtime := f.agent(t)

	running := f.task(t, agent, runtime, grandchild, "running", 2)
	dispatched := f.task(t, agent, runtime, parent, "dispatched", 1)
	waiting := f.task(t, agent, runtime, childB, "waiting_local_directory", 3)
	quick := f.quickCreateTask(t, agent, runtime, childA, "running", 4)
	// Not in flight, or not in range.
	f.task(t, agent, runtime, childA, "queued", 0)
	f.task(t, agent, runtime, childB, "completed", 5)
	f.task(t, agent, runtime, outside, "running", 6)
	f.quickCreateTask(t, agent, runtime, outside, "running", 7)
	f.quickCreateTask(t, agent, runtime, childB, "queued", 0)

	enabled := f.enable(t, parent)
	if enabled.MemberCount != 4 || enabled.InFlightGapCount != 4 {
		t.Fatalf("enable result = %+v, want 4 members and 4 in-flight gaps", enabled)
	}
	f.requireMember(t, parent, issueBudgetMemberRow{Root: enabled.BudgetRootID, Source: "root", Version: 1})
	f.requireMember(t, childA, issueBudgetMemberRow{Root: enabled.BudgetRootID, BoundParent: parent, Source: "activation", Version: 1})
	f.requireMember(t, grandchild, issueBudgetMemberRow{Root: enabled.BudgetRootID, BoundParent: childA, Source: "activation", Version: 1})
	f.requireMember(t, childB, issueBudgetMemberRow{Root: enabled.BudgetRootID, BoundParent: parent, Source: "activation", Version: 1})
	if _, ok := f.member(t, outside); ok {
		t.Fatal("an issue outside the tree was bound")
	}
	if got := f.count(t, "issue_budget_member"); got != 4 {
		t.Fatalf("%d member rows, want 4", got)
	}

	want := map[string]issueBudgetObservationRow{
		running:    {Kind: "coverage_gap", Reason: "in_flight_legacy", Issue: grandchild, Task: running, Generation: 2},
		dispatched: {Kind: "coverage_gap", Reason: "in_flight_legacy", Issue: parent, Task: dispatched, Generation: 1},
		waiting:    {Kind: "coverage_gap", Reason: "in_flight_legacy", Issue: childB, Task: waiting, Generation: 3},
		quick:      {Kind: "coverage_gap", Reason: "in_flight_legacy", Issue: childA, Task: quick, Generation: 4},
	}
	got := f.observations(t, enabled.BudgetRootID)
	if len(got) != len(want) {
		t.Fatalf("observations = %+v, want %d in-flight gaps", got, len(want))
	}
	for _, row := range got {
		if want[row.Task] != row {
			t.Fatalf("observation %+v, want %+v", row, want[row.Task])
		}
	}

	var amounts string
	f.fx.QueryRow(t, `SELECT jsonb_build_array(actual_known_ticks, estimated_ticks, reserved_ticks, unknown_hold_ticks, overrun_ticks, unknown_count, ledger_revision)::text FROM issue_budget_root WHERE id=$1`, enabled.BudgetRootID).Scan(&amounts)
	if amounts != "[0, 0, 0, 0, 0, 0, 0]" {
		t.Fatalf("a new root starts from zero, got %s", amounts)
	}
	if got := f.count(t, "issue_budget_attempt") + f.count(t, "issue_budget_entry"); got != 0 {
		t.Fatalf("enable wrote %d attempt or ledger rows", got)
	}

	// 9.5: a gap is covered once an attempt exists for the same task and
	// claim generation. The observation row itself is not touched.
	stored := issueBudgetRows(t, f.fx, "issue_budget_observation", f.workspace)
	attempt := func(task string, generation int64, issue string) {
		t.Helper()
		f.fx.Insert(t, "issue_budget_attempt", testutil.Cols{
			"id": testutil.Raw("gen_random_uuid()"), "workspace_id": f.workspace, "budget_root_id": enabled.BudgetRootID,
			"task_id": task, "claim_generation": generation, "runtime_id": runtime, "agent_id": agent, "issue_id": issue, "reserved_ticks": 0,
		})
	}
	attempt(running, 2, grandchild)
	// The same task under another generation covers nothing.
	attempt(dispatched, 9, parent)
	rows, err := f.svc.ListObservations(ctx, f.ws, util.MustParseUUID(enabled.BudgetRootID))
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if covered := util.UUIDToString(row.Row.TaskID) == running; row.Covered != covered {
			t.Fatalf("observation for task %s covered = %v, want %v", util.UUIDToString(row.Row.TaskID), row.Covered, covered)
		}
	}
	if after := issueBudgetRows(t, f.fx, "issue_budget_observation", f.workspace); after != stored {
		t.Fatalf("judging coverage rewrote an observation\nbefore: %s\nafter:  %s", stored, after)
	}
}

// 9.2, the quick-create half of the in-flight scan. A task without an issue of
// its own is found through the parent saved in its context, in each status a
// daemon can hold it in. A task that has an issue is attributed to that issue
// and to nothing else, whatever its context says.
func TestIssueBudgetInFlightScanUsesTheContextParentOnlyWithoutAnIssue(t *testing.T) {
	f := newIssueBudgetFixture(t)
	parent := f.issue(t, "Parent", "")
	child := f.issue(t, "Child", parent)
	outside := f.issue(t, "Unrelated issue", "")
	agent, runtime := f.agent(t)

	waiting := f.quickCreateTask(t, agent, runtime, child, "waiting_local_directory", 2)
	// Fixture only: the product links a quick-create task to the issue it
	// produced once the task has completed. The scan must not rely on that
	// order to count such a task once.
	linked := f.quickCreateTask(t, agent, runtime, child, "running", 3, testutil.Cols{"issue_id": parent})
	f.quickCreateTask(t, agent, runtime, child, "running", 4, testutil.Cols{"issue_id": outside})

	enabled := f.enable(t, parent)
	want := map[string]issueBudgetObservationRow{
		waiting: {Kind: "coverage_gap", Reason: "in_flight_legacy", Issue: child, Task: waiting, Generation: 2},
		linked:  {Kind: "coverage_gap", Reason: "in_flight_legacy", Issue: parent, Task: linked, Generation: 3},
	}
	got := f.observations(t, enabled.BudgetRootID)
	if len(got) != len(want) || enabled.InFlightGapCount != len(want) {
		t.Fatalf("observations = %+v (result counts %d), want %d in-flight gaps", got, enabled.InFlightGapCount, len(want))
	}
	for _, row := range got {
		if want[row.Task] != row {
			t.Fatalf("observation %+v, want %+v", row, want[row.Task])
		}
	}
	for task, through := range map[string]string{waiting: "quick_create_context_parent", linked: ""} {
		var status, attributed string
		f.fx.QueryRow(t, `SELECT detail->>'task_status', COALESCE(detail->>'attributed_through', '') FROM issue_budget_observation WHERE workspace_id=$1 AND task_id=$2`, f.workspace, task).Scan(&status, &attributed)
		var stored string
		f.fx.QueryRow(t, `SELECT status FROM agent_task_queue WHERE id=$1`, task).Scan(&stored)
		if status != stored || attributed != through {
			t.Fatalf("observation detail of task %s = (%q, %q), want (%q, %q)", task, status, attributed, stored, through)
		}
	}
}

// B07 and B08, as far as they hold without attempts: a binding is by issue.
// A retry under a new task id and a re-claim under a new generation leave the
// member row and the root exactly as they were.
func TestIssueBudgetTaskRetryAndReclaimLeaveTheBindingAlone(t *testing.T) {
	f := newIssueBudgetFixture(t)
	parent := f.issue(t, "Parent", "")
	child := f.issue(t, "Child", parent)
	agent, runtime := f.agent(t)
	first := f.task(t, agent, runtime, child, "running", 1)
	f.enable(t, parent)
	committed := f.snapshot(t)

	f.fx.Exec(t, `UPDATE agent_task_queue SET status='failed', completed_at=now() WHERE id=$1`, first)
	retry := f.task(t, agent, runtime, child, "queued", 0)
	f.fx.Exec(t, `UPDATE agent_task_queue SET status='dispatched', claim_generation=claim_generation+1 WHERE id=$1`, retry)
	f.fx.Exec(t, `UPDATE agent_task_queue SET status='queued' WHERE id=$1`, retry)
	f.fx.Exec(t, `UPDATE agent_task_queue SET status='dispatched', claim_generation=claim_generation+1 WHERE id=$1`, retry)
	f.requireUnchanged(t, committed, "a task retry and a re-claim")
}

// B59: the enable walk has no depth limit, and a parent loop refuses the
// enable without writing a single row.
func TestIssueBudgetEnableWalksPastTenLevelsAndRefusesALoop(t *testing.T) {
	f := newIssueBudgetFixture(t)
	ctx := context.Background()

	t.Run("fifteen levels", func(t *testing.T) {
		parent := f.issue(t, "Deep parent", "")
		chain := []string{parent}
		for depth := 1; depth <= 15; depth++ {
			chain = append(chain, f.issue(t, fmt.Sprintf("Level %d", depth), chain[depth-1]))
		}
		enabled := f.enable(t, parent)
		if enabled.MemberCount != 16 {
			t.Fatalf("enable bound %d members, want the root and 15 descendants", enabled.MemberCount)
		}
		for depth := 1; depth <= 15; depth++ {
			f.requireMember(t, chain[depth], issueBudgetMemberRow{Root: enabled.BudgetRootID, BoundParent: chain[depth-1], Source: "activation", Version: 1})
		}
	})

	t.Run("parent loop", func(t *testing.T) {
		parent := f.issue(t, "Looped parent", "")
		child := f.issue(t, "Looped child", parent)
		grandchild := f.issue(t, "Looped grandchild", child)
		// Fixture only: the product's own move check stops a loop it can
		// see, but it looks ten levels up, outside the transaction.
		f.fx.Exec(t, `UPDATE issue SET parent_issue_id=$2 WHERE id=$1`, parent, grandchild)
		committed := f.snapshot(t)
		_, err := f.svc.Enable(ctx, f.enableParams(parent))
		requireIssueBudgetCode(t, err, IssueBudgetParentCycle, "enable on a parent loop")
		_, err = f.svc.Enable(ctx, f.enableParams(child))
		requireIssueBudgetCode(t, err, IssueBudgetParentCycle, "enable on another issue of the loop")
		f.requireUnchanged(t, committed, "a refused enable")
		f.fx.Exec(t, `UPDATE issue SET parent_issue_id=NULL WHERE id=$1`, parent)
	})

	t.Run("its own parent", func(t *testing.T) {
		self := f.issue(t, "Self parent", "")
		f.fx.Exec(t, `UPDATE issue SET parent_issue_id=id WHERE id=$1`, self)
		committed := f.snapshot(t)
		_, err := f.svc.Enable(ctx, f.enableParams(self))
		requireIssueBudgetCode(t, err, IssueBudgetParentCycle, "enable on an issue that is its own parent")
		f.requireUnchanged(t, committed, "a refused enable")
		f.fx.Exec(t, `UPDATE issue SET parent_issue_id=NULL WHERE id=$1`, self)
	})

	// 9.2: with both a loop and a member of another root below the issue, the
	// answer is the loop. The walk meets the member first, one level down; the
	// loop closes two levels further.
	t.Run("a loop outranks a member below the issue", func(t *testing.T) {
		otherParent := f.issue(t, "Other root's parent", "")
		foreign := f.issue(t, "Member of the other root", otherParent)
		f.enable(t, otherParent)
		looped := f.issue(t, "Looped issue", "")
		f.setParent(t, foreign, looped)
		below := f.issue(t, "Below the member", foreign)
		f.setParent(t, looped, below)
		committed := f.snapshot(t)
		_, err := f.svc.Enable(ctx, f.enableParams(looped))
		requireIssueBudgetCode(t, err, IssueBudgetParentCycle, "enable on a loop that runs through a member of another root")
		f.requireUnchanged(t, committed, "a refused enable")
		f.fx.Exec(t, `UPDATE issue SET parent_issue_id=NULL WHERE id=$1`, looped)

		// Without the loop the same tree is an ordinary conflict.
		_, err = f.svc.Enable(ctx, f.enableParams(looped))
		requireIssueBudgetCode(t, err, IssueBudgetRootConflict, "enable above a member of another root")
		f.requireUnchanged(t, committed, "a refused enable")
	})
}

// Only an ordinary formal issue can be a root issue: not one still waiting
// for triage, and not a collaboration issue.
func TestIssueBudgetEnableRequiresAnOrdinaryFormalIssue(t *testing.T) {
	f := newIssueBudgetFixture(t)
	ctx := context.Background()
	for _, status := range []string{"pending", "rejected", "duplicate"} {
		issue := f.issue(t, "Triage "+status, "", testutil.Cols{"admission_status": status})
		_, err := f.svc.Enable(ctx, f.enableParams(issue))
		requireIssueBudgetCode(t, err, IssueBudgetNotEligible, "enable on a "+status+" issue")
	}
	shared := f.issue(t, "Collaboration issue", "")
	f.fx.InsertNoID(t, "issue_collaboration", testutil.Cols{"issue_id": shared}, "issue_id=$1", shared)
	_, err := f.svc.Enable(ctx, f.enableParams(shared))
	requireIssueBudgetCode(t, err, IssueBudgetNotEligible, "enable on a collaboration issue")

	accepted := f.issue(t, "Accepted from triage", "", testutil.Cols{"admission_status": "accepted"})
	if _, err := f.svc.Enable(ctx, f.enableParams(accepted)); err != nil {
		t.Fatalf("an accepted issue is formal: %v", err)
	}
	if got := f.count(t, "issue_budget_root"); got != 1 {
		t.Fatalf("%d root rows, want only the accepted issue's", got)
	}

	_, err = f.svc.Enable(ctx, f.enableParams(uuid.NewString()))
	requireIssueBudgetCode(t, err, IssueBudgetNotFound, "enable on an issue that does not exist")
	for name, mutate := range map[string]func(*IssueBudgetEnableParams){
		"no key":         func(p *IssueBudgetEnableParams) { p.OperationKey = "" },
		"oversized key":  func(p *IssueBudgetEnableParams) { p.OperationKey = strings.Repeat("k", 201) },
		"no reason":      func(p *IssueBudgetEnableParams) { p.Reason = "  " },
		"negative limit": func(p *IssueBudgetEnableParams) { limit := budget.Ticks(-1); p.LimitTicks = &limit },
		"no operator":    func(p *IssueBudgetEnableParams) { p.ActorUserID = pgtype.UUID{} },
	} {
		p := f.enableParams(f.issue(t, "Invalid request "+name, ""))
		mutate(&p)
		_, err := f.svc.Enable(ctx, p)
		requireIssueBudgetCode(t, err, IssueBudgetInvalidRequest, name)
	}
	if got := f.count(t, "issue_budget_root"); got != 1 {
		t.Fatalf("a refused enable left a root: %d rows", got)
	}
}

// B14, service half: every query carries the workspace. With a root id or an
// issue id of another workspace there is nothing to find and nothing changes.
func TestIssueBudgetIsScopedToTheWorkspace(t *testing.T) {
	f := newIssueBudgetFixture(t)
	ctx := context.Background()
	parent := f.issue(t, "Parent", "")
	child := f.issue(t, "Child", parent)
	agent, runtime := f.agent(t)
	f.task(t, agent, runtime, child, "running", 1)
	enabled := f.enable(t, parent)
	rootID := util.MustParseUUID(enabled.BudgetRootID)

	other := f.fx.Workspace(t, "Other workspace", "budget-other-"+strings.ReplaceAll(uuid.NewString(), "-", ""))
	f.fx.Member(t, other, f.user, "owner")
	issueBudgetCleanWorkspace(t, f.fx, other)
	otherWS := util.MustParseUUID(other)
	committed := f.snapshot(t)

	_, err := f.svc.GetRoot(ctx, otherWS, rootID)
	requireIssueBudgetCode(t, err, IssueBudgetNotFound, "root by id from another workspace")
	_, err = f.svc.GetRootByIssue(ctx, otherWS, util.MustParseUUID(parent))
	requireIssueBudgetCode(t, err, IssueBudgetNotFound, "root by issue from another workspace")
	_, err = f.svc.GetMember(ctx, otherWS, util.MustParseUUID(child))
	requireIssueBudgetCode(t, err, IssueBudgetNotFound, "member from another workspace")
	_, err = f.svc.ListObservations(ctx, otherWS, rootID)
	requireIssueBudgetCode(t, err, IssueBudgetNotFound, "observations from another workspace")

	foreign := f.manage(enabled.BudgetRootID, 1)
	foreign.WorkspaceID = otherWS
	limit := budget.Ticks(1)
	_, err = f.svc.Close(ctx, foreign)
	requireIssueBudgetCode(t, err, IssueBudgetNotFound, "close from another workspace")
	_, err = f.svc.Reopen(ctx, foreign)
	requireIssueBudgetCode(t, err, IssueBudgetNotFound, "reopen from another workspace")
	_, err = f.svc.SetLimit(ctx, foreign, &limit, nil)
	requireIssueBudgetCode(t, err, IssueBudgetNotFound, "limit change from another workspace")
	_, err = f.svc.SetMode(ctx, foreign, IssueBudgetModeOff)
	requireIssueBudgetCode(t, err, IssueBudgetNotFound, "mode change from another workspace")

	enable := f.enableParams(parent)
	enable.WorkspaceID = otherWS
	_, err = f.svc.Enable(ctx, enable)
	requireIssueBudgetCode(t, err, IssueBudgetNotFound, "enable on another workspace's issue")
	enable.WorkspaceID = util.MustParseUUID(uuid.NewString())
	_, err = f.svc.Enable(ctx, enable)
	requireIssueBudgetCode(t, err, IssueBudgetForbidden, "enable without membership in a workspace that does not exist")

	// The delete seam carries the workspace too: under the other workspace
	// this issue id matches no member row.
	f.inBinding(t, func(q *db.Queries) error {
		return TombstoneIssueBudgetMember(ctx, q, otherWS, util.MustParseUUID(child))
	})

	f.requireUnchanged(t, committed, "requests from another workspace")
	for _, table := range issueBudgetTables {
		if got := f.fx.Count(t, `SELECT count(*) FROM `+table+` WHERE workspace_id=$1`, other); got != 0 {
			t.Fatalf("%d rows in %s for the other workspace", got, table)
		}
	}

	// The same reads in the right workspace find the rows.
	if state, err := f.svc.GetRoot(ctx, f.ws, rootID); err != nil || state.Root.RootIssueID != util.MustParseUUID(parent) {
		t.Fatalf("root in its own workspace: %+v %v", state, err)
	}
	if member, err := f.svc.GetMember(ctx, f.ws, util.MustParseUUID(child)); err != nil || member.BudgetRootID != rootID {
		t.Fatalf("member in its own workspace: %+v %v", member, err)
	}
	if rows, err := f.svc.ListObservations(ctx, f.ws, rootID); err != nil || len(rows) != 1 {
		t.Fatalf("observations in their own workspace: %+v %v", rows, err)
	}
}

// 9.4: every management change names the policy revision it was decided
// against. A stale one is refused; the service never retries on the latest.
func TestIssueBudgetManagementRequiresTheCurrentPolicyRevision(t *testing.T) {
	f := newIssueBudgetFixture(t)
	ctx := context.Background()
	enabled := f.enable(t, f.issue(t, "Parent", ""))
	limit := budget.Ticks(250)
	if _, err := f.svc.SetLimit(ctx, f.manage(enabled.BudgetRootID, 1), &limit, nil); err != nil {
		t.Fatalf("set limit: %v", err)
	}
	committed := f.snapshot(t)

	for _, revision := range []int64{0, 1, 3} {
		stale := f.manage(enabled.BudgetRootID, revision)
		_, err := f.svc.Close(ctx, stale)
		requireIssueBudgetCode(t, err, IssueBudgetPolicyRevisionConflict, fmt.Sprintf("close against revision %d", revision))
		_, err = f.svc.SetLimit(ctx, stale, nil, nil)
		requireIssueBudgetCode(t, err, IssueBudgetPolicyRevisionConflict, fmt.Sprintf("limit against revision %d", revision))
		_, err = f.svc.SetMode(ctx, stale, IssueBudgetModeOff)
		requireIssueBudgetCode(t, err, IssueBudgetPolicyRevisionConflict, fmt.Sprintf("mode against revision %d", revision))
	}
	_, err := f.svc.Reopen(ctx, f.manage(enabled.BudgetRootID, 1))
	requireIssueBudgetCode(t, err, IssueBudgetPolicyRevisionConflict, "reopen against a stale revision")
	_, err = f.svc.Close(ctx, f.manage(uuid.NewString(), 1))
	requireIssueBudgetCode(t, err, IssueBudgetNotFound, "close of a root that does not exist")
	f.requireUnchanged(t, committed, "refused management requests")
}

// 9.4: a policy revision is a count. Zero is one, and stale (above); a
// negative number is not a revision at all. It is refused as a malformed
// request before a transaction is opened, and never as an internal error:
// the intent digest cannot even be written for it.
func TestIssueBudgetNegativePolicyRevisionIsAMalformedRequest(t *testing.T) {
	f := newIssueBudgetFixture(t)
	ctx := context.Background()
	enabled := f.enable(t, f.issue(t, "Parent", ""))
	closeKey := f.manage(enabled.BudgetRootID, 1)
	if _, err := f.svc.Close(ctx, closeKey); err != nil {
		t.Fatalf("close: %v", err)
	}
	committed := f.snapshot(t)

	log := &issueBudgetStatementLog{TxStarter: f.pool}
	svc := NewIssueBudgetService(f.q, log)
	limit := budget.Ticks(5)
	for _, revision := range []int64{-1, -2, math.MinInt64} {
		p := f.manage(enabled.BudgetRootID, revision)
		// A key that was committed is no different: it never stood for this.
		replayed := closeKey
		replayed.ExpectedPolicyRevision = revision
		for _, operation := range []struct {
			name string
			run  func() (IssueBudgetOperationResult, error)
		}{
			{"close", func() (IssueBudgetOperationResult, error) { return svc.Close(ctx, p) }},
			{"close under a committed key", func() (IssueBudgetOperationResult, error) { return svc.Close(ctx, replayed) }},
			{"reopen", func() (IssueBudgetOperationResult, error) { return svc.Reopen(ctx, p) }},
			{"limit change", func() (IssueBudgetOperationResult, error) { return svc.SetLimit(ctx, p, &limit, nil) }},
			{"limit removal", func() (IssueBudgetOperationResult, error) { return svc.SetLimit(ctx, p, nil, nil) }},
			{"mode change", func() (IssueBudgetOperationResult, error) { return svc.SetMode(ctx, p, IssueBudgetModeOff) }},
		} {
			_, err := operation.run()
			requireIssueBudgetCode(t, err, IssueBudgetInvalidRequest, fmt.Sprintf("%s against revision %d", operation.name, revision))
		}
	}
	if begun, sent := log.transactions(), log.names(); begun != 0 || len(sent) != 0 {
		t.Fatalf("negative policy revisions opened %d transactions and sent %v", begun, sent)
	}
	f.requireUnchanged(t, committed, "requests with a negative policy revision")
}

// 9.4: a management key replays its committed result, also after the root
// has moved on, and conflicts when it comes back with another intent.
func TestIssueBudgetManagementReplaysByOriginalKey(t *testing.T) {
	f := newIssueBudgetFixture(t)
	ctx := context.Background()
	enabled := f.enable(t, f.issue(t, "Parent", ""))
	limit, reserve := budget.Ticks(400), budget.Ticks(10)
	setLimit := f.manage(enabled.BudgetRootID, 1)

	first, err := f.svc.SetLimit(ctx, setLimit, &limit, &reserve)
	if err != nil {
		t.Fatalf("set limit: %v", err)
	}
	if first.OperationKind != IssueBudgetOperationSetLimit || first.PolicyRevision != 2 || first.Replayed ||
		first.LimitTicks == nil || *first.LimitTicks != limit || first.AttemptReserveTicks == nil || *first.AttemptReserveTicks != reserve {
		t.Fatalf("set limit result = %+v", first)
	}
	if _, err := f.svc.Close(ctx, f.manage(enabled.BudgetRootID, 2)); err != nil {
		t.Fatalf("close: %v", err)
	}
	committed := f.snapshot(t)

	// The root is at revision 3 by now; the original request still answers,
	// with both amounts as they were committed.
	replay, err := f.svc.SetLimit(ctx, setLimit, &limit, &reserve)
	if err != nil || !replay.Replayed {
		t.Fatalf("replay: %+v %v", replay, err)
	}
	replay.Replayed = false
	if !reflect.DeepEqual(replay, first) {
		t.Fatalf("replay = %+v, want %+v", replay, first)
	}

	// The key stands for both amounts (10.1): either one changed or left out
	// is another request.
	otherLimit, otherReserve := budget.Ticks(401), budget.Ticks(11)
	_, err = f.svc.SetLimit(ctx, setLimit, &otherLimit, &reserve)
	requireIssueBudgetCode(t, err, IssueBudgetOperationConflict, "original key with another limit")
	_, err = f.svc.SetLimit(ctx, setLimit, nil, &reserve)
	requireIssueBudgetCode(t, err, IssueBudgetOperationConflict, "original key removing the limit")
	_, err = f.svc.SetLimit(ctx, setLimit, &limit, &otherReserve)
	requireIssueBudgetCode(t, err, IssueBudgetOperationConflict, "original key with another reserve")
	_, err = f.svc.SetLimit(ctx, setLimit, &limit, nil)
	requireIssueBudgetCode(t, err, IssueBudgetOperationConflict, "original key removing the reserve")
	_, err = f.svc.Close(ctx, setLimit)
	requireIssueBudgetCode(t, err, IssueBudgetOperationConflict, "original key for another kind of change")
	moved := setLimit
	moved.ExpectedPolicyRevision = 3
	_, err = f.svc.SetLimit(ctx, moved, &limit, &reserve)
	requireIssueBudgetCode(t, err, IssueBudgetOperationConflict, "original key against another revision")
	f.requireUnchanged(t, committed, "replayed and conflicting management requests")

	cleared, err := f.svc.SetLimit(ctx, f.manage(enabled.BudgetRootID, 3), nil, nil)
	if err != nil || cleared.LimitTicks != nil || cleared.AttemptReserveTicks != nil || cleared.PolicyRevision != 4 {
		t.Fatalf("removing the limit and the reserve: %+v %v", cleared, err)
	}
	negative := budget.Ticks(-5)
	_, err = f.svc.SetLimit(ctx, f.manage(enabled.BudgetRootID, 4), &negative, nil)
	requireIssueBudgetCode(t, err, IssueBudgetInvalidRequest, "a negative limit")
}

// 9.4 and B40: switching observation off and on again. Each return to
// observe is a new policy epoch; an epoch is never used twice.
func TestIssueBudgetModeSwitchIssuesANewEpochOnEveryReturn(t *testing.T) {
	f := newIssueBudgetFixture(t)
	ctx := context.Background()
	enabled := f.enable(t, f.issue(t, "Parent", ""))
	revision := int64(1)
	seen := map[budget.Count]bool{enabled.PolicyEpoch: true}
	epoch := enabled.PolicyEpoch

	for round := 0; round < 3; round++ {
		off, err := f.svc.SetMode(ctx, f.manage(enabled.BudgetRootID, revision), IssueBudgetModeOff)
		if err != nil {
			t.Fatalf("round %d off: %v", round, err)
		}
		revision++
		if off.Mode != IssueBudgetModeOff || off.PolicyEpoch != epoch || int64(off.PolicyRevision) != revision {
			t.Fatalf("round %d off = %+v, want epoch %d kept", round, off, epoch)
		}
		_, err = f.svc.SetMode(ctx, f.manage(enabled.BudgetRootID, revision), IssueBudgetModeOff)
		requireIssueBudgetCode(t, err, IssueBudgetStateConflict, "off on a root that is off")

		on, err := f.svc.SetMode(ctx, f.manage(enabled.BudgetRootID, revision), IssueBudgetModeObserve)
		if err != nil {
			t.Fatalf("round %d observe: %v", round, err)
		}
		revision++
		if on.Mode != IssueBudgetModeObserve || on.PolicyEpoch <= epoch || seen[on.PolicyEpoch] || int64(on.PolicyRevision) != revision {
			t.Fatalf("round %d observe = %+v, want an epoch never used before (had %v)", round, on, seen)
		}
		epoch = on.PolicyEpoch
		seen[epoch] = true
	}
	_, err := f.svc.SetMode(ctx, f.manage(enabled.BudgetRootID, revision), "enforce")
	requireIssueBudgetCode(t, err, IssueBudgetInvalidRequest, "a mode this slice cannot record")
}

// 9.4 with 10.1: a limit change gives the limit and the declared reserve
// together and replaces both. It changes nothing only when both already are
// what the root has; such a request is refused like every other no-op and uses
// up no policy revision. Zero is a limit; no limit is not zero.
func TestIssueBudgetSetLimitToTheCurrentLimitIsRefused(t *testing.T) {
	f := newIssueBudgetFixture(t)
	ctx := context.Background()
	enabled := f.enable(t, f.issue(t, "Parent", ""))
	amounts := func(result IssueBudgetOperationResult) string {
		show := func(v *budget.Ticks) string {
			if v == nil {
				return "none"
			}
			return fmt.Sprint(int64(*v))
		}
		return fmt.Sprintf("limit %s reserve %s revision %d", show(result.LimitTicks), show(result.AttemptReserveTicks), result.PolicyRevision)
	}
	committed := f.snapshot(t)

	_, err := f.svc.SetLimit(ctx, f.manage(enabled.BudgetRootID, 1), nil, nil)
	requireIssueBudgetCode(t, err, IssueBudgetStateConflict, "removing a limit and a reserve the root does not have")
	f.requireUnchanged(t, committed, "a limit change that changes nothing")

	zero := budget.Ticks(0)
	set, err := f.svc.SetLimit(ctx, f.manage(enabled.BudgetRootID, 1), &zero, nil)
	if err != nil || amounts(set) != "limit 0 reserve none revision 2" {
		t.Fatalf("a limit of zero is a limit: %s %v", amounts(set), err)
	}
	committed = f.snapshot(t)
	same := budget.Ticks(0)
	_, err = f.svc.SetLimit(ctx, f.manage(enabled.BudgetRootID, 2), &same, nil)
	requireIssueBudgetCode(t, err, IssueBudgetStateConflict, "the limit the root already has, with no reserve as before")
	f.requireUnchanged(t, committed, "a limit change that changes nothing")

	// The refusals used up no revision: 2 is still the current one. The same
	// limit with a reserve the root does not have is a change.
	reserve := budget.Ticks(10)
	declared, err := f.svc.SetLimit(ctx, f.manage(enabled.BudgetRootID, 2), &zero, &reserve)
	if err != nil || amounts(declared) != "limit 0 reserve 10 revision 3" {
		t.Fatalf("the same limit with a new reserve: %s %v", amounts(declared), err)
	}
	committed = f.snapshot(t)
	sameReserve := budget.Ticks(10)
	_, err = f.svc.SetLimit(ctx, f.manage(enabled.BudgetRootID, 3), &same, &sameReserve)
	requireIssueBudgetCode(t, err, IssueBudgetStateConflict, "the limit and the reserve the root already has")
	f.requireUnchanged(t, committed, "a limit change that changes nothing")

	// Either amount alone is enough to make it a change.
	limit, otherReserve := budget.Ticks(75), budget.Ticks(11)
	changed, err := f.svc.SetLimit(ctx, f.manage(enabled.BudgetRootID, 3), &limit, &reserve)
	if err != nil || amounts(changed) != "limit 75 reserve 10 revision 4" {
		t.Fatalf("another limit with the same reserve: %s %v", amounts(changed), err)
	}
	changed, err = f.svc.SetLimit(ctx, f.manage(enabled.BudgetRootID, 4), &limit, &otherReserve)
	if err != nil || amounts(changed) != "limit 75 reserve 11 revision 5" {
		t.Fatalf("the same limit with another reserve: %s %v", amounts(changed), err)
	}
	changed, err = f.svc.SetLimit(ctx, f.manage(enabled.BudgetRootID, 5), &limit, nil)
	if err != nil || amounts(changed) != "limit 75 reserve none revision 6" {
		t.Fatalf("the same limit with the reserve removed: %s %v", amounts(changed), err)
	}
	committed = f.snapshot(t)
	_, err = f.svc.SetLimit(ctx, f.manage(enabled.BudgetRootID, 6), &limit, nil)
	requireIssueBudgetCode(t, err, IssueBudgetStateConflict, "the limit the root already has, the reserve already removed")
	f.requireUnchanged(t, committed, "a limit change that changes nothing")
}

// 10.1: enabling and the limit change each take the limit and the declared
// reserve, either of them or neither, and store them as given: what is left
// out is stored as absent, not kept from before. Close, reopen and a mode
// change carry both amounts over untouched.
func TestIssueBudgetLimitAndReserveAreSetTogether(t *testing.T) {
	f := newIssueBudgetFixture(t)
	ctx := context.Background()
	limit, reserve := budget.Ticks(1000), budget.Ticks(10)
	type amounts struct{ limit, reserve *budget.Ticks }
	combinations := []struct {
		name string
		amounts
	}{
		{"neither", amounts{nil, nil}},
		{"a limit only", amounts{&limit, nil}},
		{"a reserve only", amounts{nil, &reserve}},
		{"both", amounts{&limit, &reserve}},
	}
	// stored reads the two columns of the root row; -1 stands for NULL.
	stored := func(root string) (limitTicks, reserveTicks int64) {
		t.Helper()
		f.fx.QueryRow(t, `SELECT COALESCE(limit_ticks,-1), COALESCE(attempt_reserve_ticks,-1) FROM issue_budget_root WHERE workspace_id=$1 AND id=$2`,
			f.workspace, root).Scan(&limitTicks, &reserveTicks)
		return limitTicks, reserveTicks
	}
	want := func(v *budget.Ticks) int64 {
		if v == nil {
			return -1
		}
		return int64(*v)
	}
	requireAmounts := func(what string, result IssueBudgetOperationResult, a amounts) {
		t.Helper()
		got := amounts{result.LimitTicks, result.AttemptReserveTicks}
		if !reflect.DeepEqual(got, a) {
			t.Fatalf("%s: result has limit %v reserve %v, want %v %v", what, want(got.limit), want(got.reserve), want(a.limit), want(a.reserve))
		}
		if l, r := stored(result.BudgetRootID); l != want(a.limit) || r != want(a.reserve) {
			t.Fatalf("%s: root row has limit %d reserve %d, want %d %d", what, l, r, want(a.limit), want(a.reserve))
		}
	}

	for _, enableWith := range combinations {
		p := f.enableParams(f.issue(t, "Enabled with "+enableWith.name, ""))
		p.LimitTicks, p.AttemptReserveTicks = enableWith.limit, enableWith.reserve
		enabled, err := f.svc.Enable(ctx, p)
		if err != nil {
			t.Fatalf("enable with %s: %v", enableWith.name, err)
		}
		requireAmounts("enable with "+enableWith.name, enabled, enableWith.amounts)
		replay, err := f.svc.Enable(ctx, p)
		if err != nil || !replay.Replayed {
			t.Fatalf("replay of enable with %s: %+v %v", enableWith.name, replay, err)
		}
		requireAmounts("replay of enable with "+enableWith.name, replay, enableWith.amounts)

		// From what enable stored, every other combination is one change.
		revision := int64(1)
		for _, setTo := range combinations {
			result, err := f.svc.SetLimit(ctx, f.manage(enabled.BudgetRootID, revision), setTo.limit, setTo.reserve)
			if setTo.name == enableWith.name && revision == 1 {
				requireIssueBudgetCode(t, err, IssueBudgetStateConflict, "enabled with "+enableWith.name+", set to the same")
				requireAmounts("after the refused change", enabled, enableWith.amounts)
				continue
			}
			if err != nil {
				t.Fatalf("enabled with %s, set to %s: %v", enableWith.name, setTo.name, err)
			}
			revision++
			if int64(result.PolicyRevision) != revision {
				t.Fatalf("enabled with %s, set to %s: revision %d, want %d", enableWith.name, setTo.name, result.PolicyRevision, revision)
			}
			requireAmounts("enabled with "+enableWith.name+", set to "+setTo.name, result, setTo.amounts)
		}
	}

	// The other three operations write the same row and must leave both alone.
	p := f.enableParams(f.issue(t, "Managed", ""))
	p.LimitTicks, p.AttemptReserveTicks = &limit, &reserve
	enabled, err := f.svc.Enable(ctx, p)
	if err != nil {
		t.Fatalf("enable: %v", err)
	}
	both := amounts{&limit, &reserve}
	for i, change := range []struct {
		name string
		run  func(IssueBudgetManageParams) (IssueBudgetOperationResult, error)
	}{
		{"mode off", func(m IssueBudgetManageParams) (IssueBudgetOperationResult, error) {
			return f.svc.SetMode(ctx, m, IssueBudgetModeOff)
		}},
		{"mode observe", func(m IssueBudgetManageParams) (IssueBudgetOperationResult, error) {
			return f.svc.SetMode(ctx, m, IssueBudgetModeObserve)
		}},
		{"close", func(m IssueBudgetManageParams) (IssueBudgetOperationResult, error) { return f.svc.Close(ctx, m) }},
		{"reopen", func(m IssueBudgetManageParams) (IssueBudgetOperationResult, error) { return f.svc.Reopen(ctx, m) }},
	} {
		result, err := change.run(f.manage(enabled.BudgetRootID, int64(i+1)))
		if err != nil {
			t.Fatalf("%s: %v", change.name, err)
		}
		requireAmounts(change.name, result, both)
	}
}

// 9.4: on a root whose root issue was deleted only reopen is refused, and that
// refusal comes before the "already open" one. Close, the limit and the mode
// still change.
func TestIssueBudgetDeletedRootIssueRefusesOnlyReopen(t *testing.T) {
	f := newIssueBudgetFixture(t)
	ctx := context.Background()
	parent := f.issue(t, "Parent", "")
	enabled := f.enable(t, parent)

	// The root is still open when its issue goes.
	f.inBinding(t, func(q *db.Queries) error {
		return TombstoneIssueBudgetMember(ctx, q, f.ws, util.MustParseUUID(parent))
	})
	f.fx.Exec(t, `DELETE FROM issue WHERE id=$1`, parent)
	committed := f.snapshot(t)

	_, err := f.svc.Reopen(ctx, f.manage(enabled.BudgetRootID, 1))
	requireIssueBudgetCode(t, err, IssueBudgetRootDeleted, "reopen of an open root whose issue was deleted")
	f.requireUnchanged(t, committed, "a refused reopen")

	limit := budget.Ticks(40)
	if result, err := f.svc.SetLimit(ctx, f.manage(enabled.BudgetRootID, 1), &limit, nil); err != nil || result.PolicyRevision != 2 || result.LimitTicks == nil || *result.LimitTicks != limit {
		t.Fatalf("limit change on a root whose issue was deleted: %+v %v", result, err)
	}
	if result, err := f.svc.SetMode(ctx, f.manage(enabled.BudgetRootID, 2), IssueBudgetModeOff); err != nil || result.PolicyRevision != 3 || result.Mode != IssueBudgetModeOff {
		t.Fatalf("mode change on a root whose issue was deleted: %+v %v", result, err)
	}
	if result, err := f.svc.Close(ctx, f.manage(enabled.BudgetRootID, 3)); err != nil || result.PolicyRevision != 4 || result.AdmissionState != IssueBudgetAdmissionClosed {
		t.Fatalf("close of a root whose issue was deleted: %+v %v", result, err)
	}

	committed = f.snapshot(t)
	_, err = f.svc.Reopen(ctx, f.manage(enabled.BudgetRootID, 4))
	requireIssueBudgetCode(t, err, IssueBudgetRootDeleted, "reopen of a closed root whose issue was deleted")
	f.requireUnchanged(t, committed, "a refused reopen")
	state, err := f.svc.GetRoot(ctx, f.ws, util.MustParseUUID(enabled.BudgetRootID))
	if err != nil || state.Admission() != IssueBudgetAdmissionTombstoned || state.Root.PolicyRevision != 4 {
		t.Fatalf("root after its issue was deleted: %+v %v", state, err)
	}
}

// 9.2, 9.4 and 10.1: an original key stands for the whole request: the kind
// of change, the limit, the declared reserve, the mode and the reason. The
// reason is trimmed before it is compared and before it is stored.
func TestIssueBudgetOriginalKeyStandsForTheWholeIntent(t *testing.T) {
	f := newIssueBudgetFixture(t)
	ctx := context.Background()
	storedReason := func(operation string) string {
		t.Helper()
		var reason string
		f.fx.QueryRow(t, `SELECT result->>'reason' FROM issue_budget_operation WHERE workspace_id=$1 AND id=$2`, f.workspace, operation).Scan(&reason)
		return reason
	}

	enable := f.enableParams(f.issue(t, "Parent", ""))
	enable.Reason = "  track this parent task \n"
	enabled, err := f.svc.Enable(ctx, enable)
	if err != nil {
		t.Fatalf("enable: %v", err)
	}
	if enabled.Reason != "track this parent task" || storedReason(enabled.OperationID) != enabled.Reason {
		t.Fatalf("enable reason = %q, stored %q; want it trimmed", enabled.Reason, storedReason(enabled.OperationID))
	}

	closeRequest := f.manage(enabled.BudgetRootID, 1)
	closeRequest.Reason = "\tthe work is done  "
	closed, err := f.svc.Close(ctx, closeRequest)
	if err != nil {
		t.Fatalf("close: %v", err)
	}
	if closed.Reason != "the work is done" || storedReason(closed.OperationID) != closed.Reason {
		t.Fatalf("close reason = %q, stored %q; want it trimmed", closed.Reason, storedReason(closed.OperationID))
	}
	committed := f.snapshot(t)

	// The same request with the blanks already removed is the same request.
	trimmed := closeRequest
	trimmed.Reason = "the work is done"
	replay, err := f.svc.Close(ctx, trimmed)
	if err != nil || !replay.Replayed || replay.OperationID != closed.OperationID {
		t.Fatalf("replay with the reason as it is stored: %+v %v", replay, err)
	}
	reworded := closeRequest
	reworded.Reason = "the work is done."
	_, err = f.svc.Close(ctx, reworded)
	requireIssueBudgetCode(t, err, IssueBudgetOperationConflict, "original key with another reason")
	// Nothing but the kind of change tells these apart from the close.
	_, err = f.svc.Reopen(ctx, closeRequest)
	requireIssueBudgetCode(t, err, IssueBudgetOperationConflict, "a close key presented as a reopen")
	_, err = f.svc.SetLimit(ctx, closeRequest, nil, nil)
	requireIssueBudgetCode(t, err, IssueBudgetOperationConflict, "a close key presented as a limit removal")
	f.requireUnchanged(t, committed, "replayed and conflicting requests")

	off := f.manage(enabled.BudgetRootID, 2)
	if _, err := f.svc.SetMode(ctx, off, IssueBudgetModeOff); err != nil {
		t.Fatalf("mode off: %v", err)
	}
	committed = f.snapshot(t)
	_, err = f.svc.SetMode(ctx, off, IssueBudgetModeObserve)
	requireIssueBudgetCode(t, err, IssueBudgetOperationConflict, "original key with another mode")
	f.requireUnchanged(t, committed, "a conflicting mode request")

	// The two amounts, on the enable key and on a limit change key. Each is
	// compared on its own: present against absent, and one value against
	// another.
	limit, reserve := budget.Ticks(500), budget.Ticks(10)
	otherLimit, otherReserve := budget.Ticks(501), budget.Ticks(11)
	for _, tc := range []struct {
		name           string
		limit, reserve *budget.Ticks
	}{
		{"a limit it did not have", &limit, nil},
		{"a declared reserve it did not have", nil, &reserve},
	} {
		request := enable
		request.LimitTicks, request.AttemptReserveTicks = tc.limit, tc.reserve
		_, err = f.svc.Enable(ctx, request)
		requireIssueBudgetCode(t, err, IssueBudgetOperationConflict, "the enable key with "+tc.name)
	}
	both := f.enableParams(f.issue(t, "Parent with both amounts", ""))
	both.LimitTicks, both.AttemptReserveTicks = &limit, &reserve
	if _, err := f.svc.Enable(ctx, both); err != nil {
		t.Fatalf("enable with both amounts: %v", err)
	}
	setLimit := f.manage(enabled.BudgetRootID, 3)
	if _, err := f.svc.SetLimit(ctx, setLimit, &limit, &reserve); err != nil {
		t.Fatalf("set limit: %v", err)
	}
	committed = f.snapshot(t)
	for _, tc := range []struct {
		name           string
		limit, reserve *budget.Ticks
	}{
		{"another limit", &otherLimit, &reserve},
		{"no limit", nil, &reserve},
		{"another declared reserve", &limit, &otherReserve},
		{"no declared reserve", &limit, nil},
	} {
		request := both
		request.LimitTicks, request.AttemptReserveTicks = tc.limit, tc.reserve
		_, err = f.svc.Enable(ctx, request)
		requireIssueBudgetCode(t, err, IssueBudgetOperationConflict, "the enable key with "+tc.name)
		_, err = f.svc.SetLimit(ctx, setLimit, tc.limit, tc.reserve)
		requireIssueBudgetCode(t, err, IssueBudgetOperationConflict, "the limit change key with "+tc.name)
	}
	if replay, err := f.svc.Enable(ctx, both); err != nil || !replay.Replayed {
		t.Fatalf("the enable key with both amounts as committed: %+v %v", replay, err)
	}
	if replay, err := f.svc.SetLimit(ctx, setLimit, &limit, &reserve); err != nil || !replay.Replayed {
		t.Fatalf("the limit change key with both amounts as committed: %+v %v", replay, err)
	}
	f.requireUnchanged(t, committed, "replayed and conflicting amounts")
}

// The intent digest is what a stored original key is compared against, so its
// input is fixed byte for byte: the domain line, then the intent as JSON with
// every field present and in this order, amounts and counts as decimal strings
// and an absent one as null. A change to any of that makes every committed key
// answer operation_conflict to its own request, and no other test would say
// why. The digests below were computed outside this code (sha256 of the input
// as written).
func TestIssueBudgetIntentDigestGoldenVectors(t *testing.T) {
	const (
		workspace = "11111111-2222-4333-8444-555555555555"
		root      = "66666666-7777-4888-9999-aaaaaaaaaaaa"
		issue     = "bbbbbbbb-cccc-4ddd-8eee-ffffffffffff"
	)
	seven, zero := budget.Count(7), budget.Count(0)
	limit, reserve := budget.Ticks(123456789012), budget.Ticks(10)
	for _, tc := range []struct {
		name   string
		intent issueBudgetIntent
		// input is the byte sequence that is hashed. It is left out where
		// writing it would need the encoder's escapes spelled out.
		input  string
		digest string
	}{
		{
			name: "a limit change with both amounts",
			intent: issueBudgetIntent{
				Kind: IssueBudgetOperationSetLimit, WorkspaceID: workspace, ScopeKind: issueBudgetScopeRoot, ScopeID: root,
				ExpectedPolicyRevision: &seven, LimitTicks: &limit, AttemptReserveTicks: &reserve, Reason: "raise the limit, keep the reserve",
			},
			input:  "budget-operation-intent-v1\n" + `{"kind":"set_limit","workspace_id":"11111111-2222-4333-8444-555555555555","scope_kind":"root","scope_id":"66666666-7777-4888-9999-aaaaaaaaaaaa","expected_policy_revision":"7","limit_ticks":"123456789012","attempt_reserve_ticks":"10","mode":"","reason":"raise the limit, keep the reserve"}`,
			digest: "724b93e56e9bfe16576ce5dd072066eecdfdb631c164dd7fb6ebe9b222cf54ef",
		},
		{
			name: "an enable without amounts",
			intent: issueBudgetIntent{
				Kind: IssueBudgetOperationEnable, WorkspaceID: workspace, ScopeKind: issueBudgetScopeIssue, ScopeID: issue,
				Reason: "track this parent task",
			},
			input:  "budget-operation-intent-v1\n" + `{"kind":"enable","workspace_id":"11111111-2222-4333-8444-555555555555","scope_kind":"issue","scope_id":"bbbbbbbb-cccc-4ddd-8eee-ffffffffffff","expected_policy_revision":null,"limit_ticks":null,"attempt_reserve_ticks":null,"mode":"","reason":"track this parent task"}`,
			digest: "ac597040c384acb7310fffb0cd031d916a9c37caff865817b903c49b7029623e",
		},
		{
			// The reason holds an ampersand, angle brackets, a quote and a
			// letter outside ASCII. The digest was computed over the form
			// encoding/json writes by default: the first three as six-byte
			// escapes, the quote behind a backslash, the letter as UTF-8.
			name: "a mode change whose reason needs escaping",
			intent: issueBudgetIntent{
				Kind: IssueBudgetOperationSetMode, WorkspaceID: workspace, ScopeKind: issueBudgetScopeRoot, ScopeID: root,
				ExpectedPolicyRevision: &zero, Mode: IssueBudgetModeOff, Reason: "R&D <paused> \"" + string(rune(0xe9)) + "\"",
			},
			digest: "14dbc2e97262632b1c8a62b34a9cf104ae00506568b32ce57e9e6f0ca71d147f",
		},
	} {
		got, err := tc.intent.digest()
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if tc.input != "" {
			// The two literals of a vector must agree with each other before
			// either says anything about the code.
			if sum := sha256.Sum256([]byte(tc.input)); hex.EncodeToString(sum[:]) != tc.digest {
				t.Fatalf("%s: the vector is inconsistent: its input hashes to %x", tc.name, sum)
			}
			encoded, err := json.Marshal(tc.intent)
			if err != nil {
				t.Fatalf("%s: %v", tc.name, err)
			}
			if hashed := issueBudgetIntentDomain + string(encoded); hashed != tc.input {
				t.Fatalf("%s: the digest input changed\n got %s\nwant %s", tc.name, hashed, tc.input)
			}
		}
		if got != tc.digest {
			t.Fatalf("%s: digest %s, want %s", tc.name, got, tc.digest)
		}
	}
}

// The operator is recorded on the operation row and is not part of what the
// key stands for: another authorized operator presenting the original request
// reads the committed result, and the row still names who committed it.
func TestIssueBudgetOriginalKeyReplaysForAnotherOperator(t *testing.T) {
	f := newIssueBudgetFixture(t)
	ctx := context.Background()
	enable := f.enableParams(f.issue(t, "Parent", ""))
	enabled, err := f.svc.Enable(ctx, enable)
	if err != nil {
		t.Fatalf("enable: %v", err)
	}
	closeRequest := f.manage(enabled.BudgetRootID, 1)
	closed, err := f.svc.Close(ctx, closeRequest)
	if err != nil {
		t.Fatalf("close: %v", err)
	}
	second := f.fx.User(t, "Second budget operator", "budget-second-"+strings.ReplaceAll(uuid.NewString(), "-", "")+"@example.test")
	f.fx.Member(t, f.workspace, second, "admin")
	committed := f.snapshot(t)

	enable.ActorUserID = util.MustParseUUID(second)
	replay, err := f.svc.Enable(ctx, enable)
	if err != nil || !replay.Replayed || replay.OperationID != enabled.OperationID {
		t.Fatalf("the original enable from another operator: %+v %v", replay, err)
	}
	closeRequest.ActorUserID = util.MustParseUUID(second)
	replay, err = f.svc.Close(ctx, closeRequest)
	if err != nil || !replay.Replayed || replay.OperationID != closed.OperationID {
		t.Fatalf("the original close from another operator: %+v %v", replay, err)
	}
	f.requireUnchanged(t, committed, "a replay by another operator")
	if got := f.fx.Count(t, `SELECT count(*) FROM issue_budget_operation WHERE workspace_id=$1 AND actor_type='member' AND actor_id=$2`, f.workspace, f.user); got != 2 {
		t.Fatalf("%d operations name the operator who committed them, want 2", got)
	}
}

// 9.2 and 9.4: a malformed request is refused before a transaction is opened,
// by enable and by each management operation alike. The limits themselves are
// accepted.
func TestIssueBudgetMalformedRequestsSendNoStatement(t *testing.T) {
	f := newIssueBudgetFixture(t)
	ctx := context.Background()
	enabled := f.enable(t, f.issue(t, "Parent", ""))
	candidate := f.issue(t, "Not a root yet", "")
	committed := f.snapshot(t)

	log := &issueBudgetStatementLog{TxStarter: f.pool}
	svc := NewIssueBudgetService(f.q, log)
	limit := budget.Ticks(5)
	manage := func(key, reason string) IssueBudgetManageParams {
		p := f.manage(enabled.BudgetRootID, 1)
		p.OperationKey, p.Reason = key, reason
		return p
	}
	operations := []struct {
		name string
		run  func(key, reason string) error
	}{
		{"enable", func(key, reason string) error {
			p := f.enableParams(candidate)
			p.OperationKey, p.Reason = key, reason
			_, err := svc.Enable(ctx, p)
			return err
		}},
		{"close", func(key, reason string) error { _, err := svc.Close(ctx, manage(key, reason)); return err }},
		{"reopen", func(key, reason string) error { _, err := svc.Reopen(ctx, manage(key, reason)); return err }},
		{"limit change", func(key, reason string) error {
			_, err := svc.SetLimit(ctx, manage(key, reason), &limit, nil)
			return err
		}},
		{"mode change", func(key, reason string) error {
			_, err := svc.SetMode(ctx, manage(key, reason), IssueBudgetModeOff)
			return err
		}},
	}
	malformed := []struct{ name, key, reason string }{
		{"key that is not UTF-8", "key-\xff", "operator decision"},
		{"key with a NUL byte", "key-\x00-tail", "operator decision"},
		{"key of 201 bytes", strings.Repeat("k", 201), "operator decision"},
		{"reason of 2001 bytes", "key", strings.Repeat("r", 2001)},
		{"reason of 2001 bytes between blanks", "key", " " + strings.Repeat("r", 2001) + " "},
		{"reason that is not UTF-8", "key", "operator \xff decision"},
		{"reason with a NUL byte", "key", "operator \x00 decision"},
		{"reason of blanks only", "key", " \t\n"},
	}
	for _, operation := range operations {
		for _, request := range malformed {
			requireIssueBudgetCode(t, operation.run(request.key, request.reason), IssueBudgetInvalidRequest, operation.name+" with a "+request.name)
		}
	}
	negative := budget.Ticks(-1)
	withLimit := f.enableParams(candidate)
	withLimit.LimitTicks = &negative
	_, err := svc.Enable(ctx, withLimit)
	requireIssueBudgetCode(t, err, IssueBudgetInvalidRequest, "enable with a negative limit")
	_, err = svc.SetLimit(ctx, f.manage(enabled.BudgetRootID, 1), &negative, nil)
	requireIssueBudgetCode(t, err, IssueBudgetInvalidRequest, "a negative limit")
	// 10.1: a declared reserve is a positive amount. Zero is a limit and is
	// not a reserve; the way to declare none is to leave it out.
	for _, tc := range []struct {
		name    string
		reserve budget.Ticks
	}{{"a declared reserve of zero", 0}, {"a negative declared reserve", -1}} {
		withReserve := f.enableParams(candidate)
		withReserve.LimitTicks, withReserve.AttemptReserveTicks = &limit, &tc.reserve
		_, err = svc.Enable(ctx, withReserve)
		requireIssueBudgetCode(t, err, IssueBudgetInvalidRequest, "enable with "+tc.name)
		_, err = svc.SetLimit(ctx, f.manage(enabled.BudgetRootID, 1), &limit, &tc.reserve)
		requireIssueBudgetCode(t, err, IssueBudgetInvalidRequest, "a limit change with "+tc.name)
	}
	_, err = svc.SetMode(ctx, f.manage(enabled.BudgetRootID, 1), "enforce")
	requireIssueBudgetCode(t, err, IssueBudgetInvalidRequest, "a mode this slice cannot record")
	if begun, sent := log.transactions(), log.names(); begun != 0 || len(sent) != 0 {
		t.Fatalf("malformed requests opened %d transactions and sent %v", begun, sent)
	}
	f.requireUnchanged(t, committed, "malformed requests")

	// A key of 200 bytes and a reason of 2000 bytes once the blanks around it
	// are gone are both within the limits. The reason limit counts bytes.
	accepted := f.enableParams(candidate)
	accepted.OperationKey = strings.Repeat("k", 200)
	accepted.Reason = "  " + strings.Repeat("r", 2000) + "\n"
	root, err := f.svc.Enable(ctx, accepted)
	if err != nil || len(root.Reason) != 2000 {
		t.Fatalf("enable at the limits: reason of %d bytes, %v", len(root.Reason), err)
	}
	atLimit := f.manage(root.BudgetRootID, 1)
	atLimit.OperationKey = strings.Repeat("c", 200)
	atLimit.Reason = strings.Repeat("\u00e9", 1000)
	if closed, err := f.svc.Close(ctx, atLimit); err != nil || closed.Reason != atLimit.Reason {
		t.Fatalf("close at the limits: reason of %d bytes, %v", len(closed.Reason), err)
	}
	overLimit := f.manage(root.BudgetRootID, 2)
	overLimit.Reason = strings.Repeat("\u00e9", 1000) + "r"
	_, err = f.svc.Reopen(ctx, overLimit)
	requireIssueBudgetCode(t, err, IssueBudgetInvalidRequest, "a reason of 2001 bytes in 1001 characters")
}

// issueBudgetLevelStatement is the statement design section 5, rule 5, has
// every budget operation open with. It is written out here: the test pins the
// words, not the constant the service happens to send.
const issueBudgetLevelStatement = "SET TRANSACTION ISOLATION LEVEL READ COMMITTED"

// Section 5 of the design, statement by statement. Every operation declares
// its level first. Enabling then takes the workspace fence before it reads,
// the binding lock once nothing can refuse it any more, and only after that
// inserts the root row; management locks the root first and after that touches
// budget tables only.
func TestIssueBudgetTransactionsFollowTheLockOrder(t *testing.T) {
	f := newIssueBudgetFixture(t)
	ctx := context.Background()
	parent := f.issue(t, "Parent", "")
	child := f.issue(t, "Child", parent)
	agent, runtime := f.agent(t)
	f.task(t, agent, runtime, child, "running", 1)

	log := &issueBudgetStatementLog{TxStarter: f.pool}
	svc := NewIssueBudgetService(f.q, log)
	enabled, err := svc.Enable(ctx, f.enableParams(parent))
	if err != nil {
		t.Fatal(err)
	}
	// Written from design 9.2 and 10.10, not from a run: the level first; the
	// membership read, workspace row and the fence; the slot, the issue and every refusal
	// check; the walk; then the binding lock, the in-flight scan under it,
	// the activation time from the statement clock, and only then the rows:
	// root, members, the gap the scan found, the operation.
	wantEnable := []string{
		issueBudgetLevelStatement,
		"GetMemberByUserAndWorkspace",
		"LockWorkspaceForChatSessionCreate",
		iteration.WorkspaceFenceSQL,
		"GetIssueBudgetRootByIssue",
		"GetIssueInWorkspace",
		"IsCollaborationIssue",
		"GetIssueBudgetMember",
		"ListIssueBudgetDescendants",
		"LockIssueBudgetBinding",
		"ListIssueBudgetInFlightTasks",
		"SELECT clock_timestamp()",
		"InsertIssueBudgetRoot",
		"InsertIssueBudgetMember",
		"InsertIssueBudgetMembers",
		"InsertIssueBudgetObservation",
		"InsertIssueBudgetOperation",
	}
	if got := log.names(); !reflect.DeepEqual(got, wantEnable) {
		t.Fatalf("enable sent\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(wantEnable, "\n"))
	}

	// The four management operations: the level and membership, then the root lock as the
	// first and only lock, then budget tables alone. None of them takes the
	// fence or the binding lock.
	limit, reserve := budget.Ticks(10), budget.Ticks(1)
	for _, tc := range []struct {
		name string
		run  func(*IssueBudgetService, IssueBudgetManageParams) (IssueBudgetOperationResult, error)
		want []string
	}{
		{"set limit", func(s *IssueBudgetService, p IssueBudgetManageParams) (IssueBudgetOperationResult, error) {
			return s.SetLimit(ctx, p, &limit, &reserve)
		}, []string{issueBudgetLevelStatement, "GetMemberByUserAndWorkspace", "LockIssueBudgetRoot", "GetIssueBudgetOperationByKey", "UpdateIssueBudgetRootPolicy", "InsertIssueBudgetOperation"}},
		{"mode", func(s *IssueBudgetService, p IssueBudgetManageParams) (IssueBudgetOperationResult, error) {
			return s.SetMode(ctx, p, IssueBudgetModeOff)
		}, []string{issueBudgetLevelStatement, "GetMemberByUserAndWorkspace", "LockIssueBudgetRoot", "GetIssueBudgetOperationByKey", "UpdateIssueBudgetRootPolicy", "InsertIssueBudgetOperation"}},
		{"resume observation", func(s *IssueBudgetService, p IssueBudgetManageParams) (IssueBudgetOperationResult, error) {
			return s.SetMode(ctx, p, IssueBudgetModeObserve)
		}, []string{issueBudgetLevelStatement, "GetMemberByUserAndWorkspace", "LockIssueBudgetRoot", "GetIssueBudgetOperationByKey", "InsertIssueBudgetObservation", "UpdateIssueBudgetRootPolicy", "InsertIssueBudgetOperation"}},
		{"close", func(s *IssueBudgetService, p IssueBudgetManageParams) (IssueBudgetOperationResult, error) {
			return s.Close(ctx, p)
		}, []string{issueBudgetLevelStatement, "GetMemberByUserAndWorkspace", "LockIssueBudgetRoot", "GetIssueBudgetOperationByKey", "UpdateIssueBudgetRootPolicy", "InsertIssueBudgetOperation"}},
		{"reopen", func(s *IssueBudgetService, p IssueBudgetManageParams) (IssueBudgetOperationResult, error) {
			return s.Reopen(ctx, p)
		}, []string{issueBudgetLevelStatement, "GetMemberByUserAndWorkspace", "LockIssueBudgetRoot", "GetIssueBudgetOperationByKey", "GetIssueBudgetMember", "UpdateIssueBudgetRootPolicy", "InsertIssueBudgetOperation"}},
	} {
		log := &issueBudgetStatementLog{TxStarter: f.pool}
		var revision int64
		f.fx.QueryRow(t, `SELECT policy_revision FROM issue_budget_root WHERE id=$1`, enabled.BudgetRootID).Scan(&revision)
		if _, err := tc.run(NewIssueBudgetService(f.q, log), f.manage(enabled.BudgetRootID, revision)); err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if got := log.names(); !reflect.DeepEqual(got, tc.want) {
			t.Fatalf("%s sent\n%s\nwant\n%s", tc.name, strings.Join(got, "\n"), strings.Join(tc.want, "\n"))
		}
	}
}

// issueBudgetOperation is one of the five operations. Enabling ignores the
// management parameters: it enables the issue the list was built for.
type issueBudgetOperation struct {
	name string
	run  func(context.Context, *IssueBudgetService, IssueBudgetManageParams) (IssueBudgetOperationResult, error)
}

func (f *issueBudgetFixture) operations(unbound string) []issueBudgetOperation {
	limit, reserve := budget.Ticks(10), budget.Ticks(1)
	return []issueBudgetOperation{
		{"enable", func(ctx context.Context, s *IssueBudgetService, _ IssueBudgetManageParams) (IssueBudgetOperationResult, error) {
			return s.Enable(ctx, f.enableParams(unbound))
		}},
		{"set limit", func(ctx context.Context, s *IssueBudgetService, p IssueBudgetManageParams) (IssueBudgetOperationResult, error) {
			return s.SetLimit(ctx, p, &limit, &reserve)
		}},
		{"mode", func(ctx context.Context, s *IssueBudgetService, p IssueBudgetManageParams) (IssueBudgetOperationResult, error) {
			return s.SetMode(ctx, p, IssueBudgetModeOff)
		}},
		{"close", func(ctx context.Context, s *IssueBudgetService, p IssueBudgetManageParams) (IssueBudgetOperationResult, error) {
			return s.Close(ctx, p)
		}},
		{"reopen", func(ctx context.Context, s *IssueBudgetService, p IssueBudgetManageParams) (IssueBudgetOperationResult, error) {
			return s.Reopen(ctx, p)
		}},
	}
}

// Section 5, rule 5, where an operation is handed an open transaction as its
// starter: it then runs in a savepoint, and PostgreSQL lets a savepoint name
// the level only when the transaction already has it. An operation started
// from REPEATABLE READ or SERIALIZABLE fails on its first statement, before it
// read or locked anything, whether or not the outer transaction has taken its
// snapshot yet, and leaves that transaction usable.
func TestIssueBudgetOperationsRefuseATransactionAtAnotherLevel(t *testing.T) {
	f := newIssueBudgetFixture(t)
	ctx := context.Background()
	parent := f.issue(t, "Parent", "")
	unbound := f.issue(t, "No root yet", "")
	enabled := f.enable(t, parent)
	committed := f.snapshot(t)

	for _, level := range []pgx.TxIsoLevel{pgx.RepeatableRead, pgx.Serializable} {
		for _, afterQuery := range []bool{false, true} {
			for _, op := range f.operations(unbound) {
				what := fmt.Sprintf("%s from %s (outer snapshot taken: %v)", op.name, level, afterQuery)
				outer, err := f.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: level})
				if err != nil {
					t.Fatal(err)
				}
				// Registered before the first check: a failed check must not
				// leave the connection out of the pool the cleanup closes.
				t.Cleanup(func() { _ = outer.Rollback(context.Background()) })
				if afterQuery {
					var roots int
					if err := outer.QueryRow(ctx, `SELECT count(*) FROM issue_budget_root WHERE workspace_id=$1`, f.ws).Scan(&roots); err != nil || roots != 1 {
						t.Fatalf("%s: outer read = %d, %v", what, roots, err)
					}
				}
				log := &issueBudgetStatementLog{TxStarter: outer}
				_, err = op.run(ctx, NewIssueBudgetService(f.q, log), f.manage(enabled.BudgetRootID, 1))
				var pgErr *pgconn.PgError
				if !errors.As(err, &pgErr) || pgErr.Code != "25001" {
					t.Fatalf("%s: got %v, want SQLSTATE 25001 from the level statement", what, err)
				}
				if code := IssueBudgetErrorCode(err); code != "" {
					t.Fatalf("%s: answered with refusal %q; it is not a budget refusal", what, code)
				}
				if got := log.names(); !reflect.DeepEqual(got, []string{issueBudgetLevelStatement}) {
					t.Fatalf("%s sent %v, want the level statement and nothing after it", what, got)
				}
				// The savepoint was rolled back: the outer transaction goes
				// on at the level it had, and holds none of the operation's
				// locks.
				var current string
				if err := outer.QueryRow(ctx, `SELECT current_setting('transaction_isolation')`).Scan(&current); err != nil || current != string(level) {
					t.Fatalf("%s: outer transaction afterwards = %q, %v; want it usable at %s", what, current, err, level)
				}
				var advisory int
				if err := outer.QueryRow(ctx, `SELECT count(*) FROM pg_locks WHERE pid=pg_backend_pid() AND locktype='advisory'`).Scan(&advisory); err != nil || advisory != 0 {
					t.Fatalf("%s: outer transaction holds %d advisory locks (%v)", what, advisory, err)
				}
				if err := outer.Rollback(ctx); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	f.requireUnchanged(t, committed, "operations refused at the level statement")
}

// The other half of rule 5: handed a READ COMMITTED transaction, an operation
// runs as a savepoint of it. The level statement is accepted and changes
// nothing, also when the outer transaction has already read; what the operation
// wrote is the outer transaction's to commit or to roll back.
func TestIssueBudgetOperationsRunInsideAReadCommittedTransaction(t *testing.T) {
	f := newIssueBudgetFixture(t)
	ctx := context.Background()
	parent := f.issue(t, "Parent", "")
	f.issue(t, "Child", parent)
	empty := f.snapshot(t)

	for _, tc := range []struct {
		name       string
		afterQuery bool
		commit     bool
	}{
		{"an outer transaction that has read, rolled back", true, false},
		{"a fresh outer transaction, rolled back", false, false},
		{"a fresh outer transaction, committed", false, true},
	} {
		outer, err := f.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = outer.Rollback(context.Background()) })
		if tc.afterQuery {
			var roots int
			if err := outer.QueryRow(ctx, `SELECT count(*) FROM issue_budget_root WHERE workspace_id=$1`, f.ws).Scan(&roots); err != nil || roots != 0 {
				t.Fatalf("%s: outer read = %d, %v", tc.name, roots, err)
			}
		}
		log := &issueBudgetStatementLog{TxStarter: outer}
		svc := NewIssueBudgetService(f.q, log)
		enabled, err := svc.Enable(ctx, f.enableParams(parent))
		if err != nil {
			t.Fatalf("%s: enable: %v", tc.name, err)
		}
		if got := log.names(); len(got) == 0 || got[0] != issueBudgetLevelStatement {
			t.Fatalf("%s: enable opened with %v", tc.name, got)
		}
		// Every management operation, each in a savepoint of its own.
		revision := int64(1)
		for _, op := range f.operations("")[1:] {
			result, err := op.run(ctx, svc, f.manage(enabled.BudgetRootID, revision))
			if err != nil {
				t.Fatalf("%s: %s: %v", tc.name, op.name, err)
			}
			revision++
			if result.PolicyRevision != budget.Count(revision) {
				t.Fatalf("%s: %s left revision %d, want %d", tc.name, op.name, result.PolicyRevision, revision)
			}
		}

		// Inside the outer transaction the work is there, and its level is
		// what it was.
		var inside struct {
			level              string
			roots, members     int
			operations, policy int64
		}
		if err := outer.QueryRow(ctx, `SELECT current_setting('transaction_isolation'),
			(SELECT count(*) FROM issue_budget_root WHERE workspace_id=$1),
			(SELECT count(*) FROM issue_budget_member WHERE workspace_id=$1),
			(SELECT count(*) FROM issue_budget_operation WHERE workspace_id=$1),
			(SELECT policy_revision FROM issue_budget_root WHERE workspace_id=$1)`, f.ws).
			Scan(&inside.level, &inside.roots, &inside.members, &inside.operations, &inside.policy); err != nil {
			t.Fatalf("%s: read inside the outer transaction: %v", tc.name, err)
		}
		if inside.level != "read committed" || inside.roots != 1 || inside.members != 2 || inside.operations != 5 || inside.policy != 5 {
			t.Fatalf("%s: inside the outer transaction = %+v, want read committed, 1 root, 2 members, 5 operations at revision 5", tc.name, inside)
		}
		// Nobody else sees any of it before the outer transaction ends.
		f.requireUnchanged(t, empty, tc.name+", before the outer transaction ended")

		if !tc.commit {
			if err := outer.Rollback(ctx); err != nil {
				t.Fatal(err)
			}
			f.requireUnchanged(t, empty, tc.name)
			continue
		}
		if err := outer.Commit(ctx); err != nil {
			t.Fatal(err)
		}
		if roots, members, operations := f.count(t, "issue_budget_root"), f.count(t, "issue_budget_member"), f.count(t, "issue_budget_operation"); roots != 1 || members != 2 || operations != 5 {
			t.Fatalf("%s: committed %d roots, %d members, %d operations; want 1, 2, 5", tc.name, roots, members, operations)
		}
	}
}

// Section 5, rule 1, and the cost promised to trees without a root: the
// binding functions never lock or update a root row, and for an issue outside every
// root they are one lookup, or one update that matches nothing.
func TestIssueBudgetBindingNeverLocksOrUpdatesARootRow(t *testing.T) {
	f := newIssueBudgetFixture(t)
	ctx := context.Background()
	rootless := f.issue(t, "Rootless parent", "")
	stray := f.issue(t, "Rootless child", rootless)
	strayRow := f.setParent(t, stray, rootless)
	empty := f.snapshot(t)

	if got := f.inBinding(t, func(q *db.Queries) error { return BindIssueBudgetOnCreate(ctx, q, strayRow, pgtype.UUID{}) }); !reflect.DeepEqual(got, []string{"GetIssueBudgetMember"}) {
		t.Fatalf("create under a rootless parent sent %v", got)
	}
	if got := f.inBinding(t, func(q *db.Queries) error { return BindIssueBudgetOnReparent(ctx, q, strayRow) }); !reflect.DeepEqual(got, []string{"GetIssueBudgetMember"}) {
		t.Fatalf("move under a rootless parent sent %v", got)
	}
	if got := f.inBinding(t, func(q *db.Queries) error {
		return TombstoneIssueBudgetMember(ctx, q, f.ws, util.MustParseUUID(stray))
	}); !reflect.DeepEqual(got, []string{"TombstoneIssueBudgetMember"}) {
		t.Fatalf("delete of an issue outside every root sent %v", got)
	}
	detached := f.setParent(t, stray, "")
	if got := f.inBinding(t, func(q *db.Queries) error { return BindIssueBudgetOnCreate(ctx, q, detached, pgtype.UUID{}) }); len(got) != 0 {
		t.Fatalf("create without a parent sent %v", got)
	}
	if got := f.inBinding(t, func(q *db.Queries) error { return BindIssueBudgetOnReparent(ctx, q, detached) }); len(got) != 0 {
		t.Fatalf("clearing a parent sent %v", got)
	}
	f.requireUnchanged(t, empty, "issue writes in a tree without a root")

	// With roots in play, every statement the three functions send.
	parent := f.issue(t, "Parent", "")
	other := f.issue(t, "Other parent", "")
	enabled := f.enable(t, parent)
	f.enable(t, other)
	subtree := f.issue(t, "Unbound subtree", "")
	leaf := f.issue(t, "Unbound leaf", subtree)
	foreign := f.issue(t, "Member of the other root", "")
	f.inBinding(t, func(q *db.Queries) error { return BindIssueBudgetOnReparent(ctx, q, f.setParent(t, foreign, other)) })
	f.setParent(t, foreign, subtree)
	agent, runtime := f.agent(t)
	f.task(t, agent, runtime, leaf, "running", 1)
	rootBefore := issueBudgetRows(t, f.fx, "issue_budget_root", f.workspace)

	var sent []string
	sent = append(sent, f.inBinding(t, func(q *db.Queries) error { return BindIssueBudgetOnReparent(ctx, q, f.setParent(t, subtree, parent)) })...)
	created := f.issue(t, "Created under a member", subtree)
	sent = append(sent, f.inBinding(t, func(q *db.Queries) error {
		return BindIssueBudgetOnCreate(ctx, q, f.setParent(t, created, subtree), pgtype.UUID{})
	})...)
	sent = append(sent, f.inBinding(t, func(q *db.Queries) error { return BindIssueBudgetOnReparent(ctx, q, f.setParent(t, created, other)) })...)
	sent = append(sent, f.inBinding(t, func(q *db.Queries) error {
		return TombstoneIssueBudgetMember(ctx, q, f.ws, util.MustParseUUID(created))
	})...)
	// Besides member and observation rows: the question about its own level a
	// binding asks once it found a member row, and the workspace binding lock a
	// move takes before it gives existing issues their first member row
	// (design section 5, rule 5, and 10.10). A plain root read checks the mode;
	// no root is locked or updated.
	allowed := map[string]bool{
		"GetIssueBudgetMember": true, "InsertIssueBudgetMember": true, "InsertIssueBudgetMembers": true,
		"ListIssueBudgetDescendants": true, "ListIssueBudgetInFlightTasks": true,
		"InsertIssueBudgetObservation": true, "TombstoneIssueBudgetMember": true,
		"GetIssueBudgetTransactionIsolation": true, "LockIssueBudgetBinding": true, "GetIssueBudgetRoot": true,
	}
	used := map[string]bool{}
	for _, name := range sent {
		if !allowed[name] {
			t.Fatalf("an issue write path sent %q; it may only read and write member and observation rows", name)
		}
		used[name] = true
	}
	for name := range allowed {
		if !used[name] {
			t.Fatalf("the scenario never exercised %s; sent %v", name, sent)
		}
	}
	if after := issueBudgetRows(t, f.fx, "issue_budget_root", f.workspace); after != rootBefore {
		t.Fatalf("an issue write path changed a root row\nbefore: %s\nafter:  %s", rootBefore, after)
	}
	f.requireMember(t, created, issueBudgetMemberRow{Root: enabled.BudgetRootID, BoundParent: subtree, Source: "create", Version: 1, Tombstoned: true})
}

// 9.3, the rules themselves (B10 and B11 go through the HTTP move in the
// handler suite): a member row is never rewritten, a move into another root
// is recorded on the issue's own root, and nothing else is written.
func TestIssueBudgetReparentNeverRewritesAMemberRow(t *testing.T) {
	f := newIssueBudgetFixture(t)
	ctx := context.Background()
	parent := f.issue(t, "Parent", "")
	child := f.issue(t, "Child", parent)
	sibling := f.issue(t, "Sibling", parent)
	rootless := f.issue(t, "Rootless", "")
	otherParent := f.issue(t, "Other root's parent", "")
	otherChild := f.issue(t, "Other root's child", otherParent)
	enabled := f.enable(t, parent)
	other := f.enable(t, otherParent)
	want := issueBudgetMemberRow{Root: enabled.BudgetRootID, BoundParent: parent, Source: "activation", Version: 1}
	committed := f.snapshot(t)

	move := func(to string) {
		t.Helper()
		row := f.setParent(t, child, to)
		f.inBinding(t, func(q *db.Queries) error { return BindIssueBudgetOnReparent(ctx, q, row) })
	}
	move("")
	move(rootless)
	move(sibling)
	move(parent)
	f.requireUnchanged(t, committed, "moves that stay outside other roots")

	move(otherChild)
	f.requireMember(t, child, want)
	if got := f.observations(t, enabled.BudgetRootID); !reflect.DeepEqual(got, []issueBudgetObservationRow{{Kind: "would_refuse", Reason: "root_conflict", Issue: child, Generation: -1}}) {
		t.Fatalf("observations on the issue's own root = %+v", got)
	}
	if got := f.observations(t, other.BudgetRootID); len(got) != 0 {
		t.Fatalf("the other root got observations: %+v", got)
	}
	after := f.snapshot(t)
	for _, table := range []string{"issue_budget_root", "issue_budget_member", "issue_budget_operation", "issue_budget_attempt", "issue_budget_entry"} {
		if after[table] != committed[table] {
			t.Fatalf("a move into another root changed %s", table)
		}
	}
	var detail string
	f.fx.QueryRow(t, `SELECT detail::text FROM issue_budget_observation WHERE workspace_id=$1`, f.workspace).Scan(&detail)
	var decoded map[string]string
	if err := json.Unmarshal([]byte(detail), &decoded); err != nil || decoded["new_parent_issue_id"] != otherChild || decoded["new_parent_budget_root_id"] != other.BudgetRootID {
		t.Fatalf("observation detail = %s (%v)", detail, err)
	}
}

// 9.3 rule 2: an issue outside every root moved under a member joins that
// root with its unbound descendants. Descendants of another root stay there
// and are recorded; tasks already held by a daemon are recorded as gaps.
func TestIssueBudgetReparentBindsAnUnboundSubtree(t *testing.T) {
	f := newIssueBudgetFixture(t)
	ctx := context.Background()
	parent := f.issue(t, "Parent", "")
	member := f.issue(t, "Member", parent)
	otherParent := f.issue(t, "Other root's parent", "")
	foreign := f.issue(t, "Member of the other root", otherParent)
	enabled := f.enable(t, parent)
	other := f.enable(t, otherParent)

	subtree := f.issue(t, "Unbound subtree", "")
	leafA := f.issue(t, "Unbound leaf A", subtree)
	leafB := f.issue(t, "Unbound leaf B", leafA)
	// A member of the other root that was moved into the unbound subtree,
	// and a member of this root that was.
	f.setParent(t, foreign, leafA)
	own := f.issue(t, "Member moved into the subtree", parent)
	f.inBinding(t, func(q *db.Queries) error {
		return BindIssueBudgetOnCreate(ctx, q, f.setParent(t, own, parent), pgtype.UUID{})
	})
	f.setParent(t, own, subtree)
	agent, runtime := f.agent(t)
	running := f.task(t, agent, runtime, leafB, "running", 3)
	quick := f.quickCreateTask(t, agent, runtime, subtree, "dispatched", 1)
	f.task(t, agent, runtime, leafA, "queued", 0)
	// Held for issues that are not newly bound: no gap for these.
	f.task(t, agent, runtime, foreign, "running", 5)
	f.task(t, agent, runtime, own, "running", 6)
	f.task(t, agent, runtime, member, "running", 7)

	row := f.setParent(t, subtree, member)
	f.inBinding(t, func(q *db.Queries) error { return BindIssueBudgetOnReparent(ctx, q, row) })

	f.requireMember(t, subtree, issueBudgetMemberRow{Root: enabled.BudgetRootID, BoundParent: member, Source: "reparent", Version: 1})
	f.requireMember(t, leafA, issueBudgetMemberRow{Root: enabled.BudgetRootID, BoundParent: subtree, Source: "reparent", Version: 1})
	f.requireMember(t, leafB, issueBudgetMemberRow{Root: enabled.BudgetRootID, BoundParent: leafA, Source: "reparent", Version: 1})
	f.requireMember(t, foreign, issueBudgetMemberRow{Root: other.BudgetRootID, BoundParent: otherParent, Source: "activation", Version: 1})
	f.requireMember(t, own, issueBudgetMemberRow{Root: enabled.BudgetRootID, BoundParent: parent, Source: "create", Version: 1})
	// Two roots with one member each, the member bound by create, and the
	// three issues this move bound.
	if got := f.count(t, "issue_budget_member"); got != 8 {
		t.Fatalf("%d member rows, want 8", got)
	}

	want := map[issueBudgetObservationRow]bool{
		{Kind: "would_refuse", Reason: "root_conflict", Issue: foreign, Generation: -1}:                 true,
		{Kind: "coverage_gap", Reason: "in_flight_at_bind", Issue: leafB, Task: running, Generation: 3}: true,
		{Kind: "coverage_gap", Reason: "in_flight_at_bind", Issue: subtree, Task: quick, Generation: 1}: true,
	}
	got := f.observations(t, enabled.BudgetRootID)
	if len(got) != len(want) {
		t.Fatalf("observations on the new root = %+v, want %d", got, len(want))
	}
	for _, row := range got {
		if !want[row] {
			t.Fatalf("unexpected observation %+v", row)
		}
	}
	if got := f.observations(t, other.BudgetRootID); len(got) != 0 {
		t.Fatalf("the other root got observations: %+v", got)
	}

	// Moving the now bound subtree again is rule 1: nothing is rewritten.
	committed := f.snapshot(t)
	again := f.setParent(t, subtree, parent)
	f.inBinding(t, func(q *db.Queries) error { return BindIssueBudgetOnReparent(ctx, q, again) })
	f.requireUnchanged(t, committed, "moving a bound issue inside its root")
}

// 9.3 rule 2 with a parent loop below the moved issue: the move is not
// refused, what the walk reached is bound, and the loop is recorded.
func TestIssueBudgetReparentThroughAParentLoopStillBinds(t *testing.T) {
	f := newIssueBudgetFixture(t)
	ctx := context.Background()
	parent := f.issue(t, "Parent", "")
	member := f.issue(t, "Member", parent)
	enabled := f.enable(t, parent)

	moved := f.issue(t, "Moved issue", "")
	below := f.issue(t, "Below the moved issue", moved)
	// Fixture only: the new parent is itself below the moved issue, which
	// the product's ten-level check outside the transaction can miss.
	f.setParent(t, member, below)
	row := f.setParent(t, moved, member)
	f.inBinding(t, func(q *db.Queries) error { return BindIssueBudgetOnReparent(ctx, q, row) })

	f.requireMember(t, moved, issueBudgetMemberRow{Root: enabled.BudgetRootID, BoundParent: member, Source: "reparent", Version: 1})
	f.requireMember(t, below, issueBudgetMemberRow{Root: enabled.BudgetRootID, BoundParent: moved, Source: "reparent", Version: 1})
	f.requireMember(t, member, issueBudgetMemberRow{Root: enabled.BudgetRootID, BoundParent: parent, Source: "activation", Version: 1})
	if got := f.observations(t, enabled.BudgetRootID); !reflect.DeepEqual(got, []issueBudgetObservationRow{{Kind: "coverage_gap", Reason: "parent_cycle", Issue: moved, Generation: -1}}) {
		t.Fatalf("observations = %+v, want the recorded loop", got)
	}
	f.fx.Exec(t, `UPDATE issue SET parent_issue_id=NULL WHERE id=$1`, moved)
}

// B60, the collaboration paths, each through its product entry:
//
//	C7  CollaborationService.Create with a parent       (create)
//	C8  an accepted stage proposal materializing children (create)
//	S5  UpdateCollaborationIssueStructure via EditIssue   (parent writer)
//
// The other six paths are asserted in the handler suite.
func TestIssueBudgetCollaborationWritersBindMembers(t *testing.T) {
	f := newCollaborationStageFixture(t)
	issueBudgetCleanWorkspace(t, f.fx, f.workspace)
	ctx := context.Background()
	ws := util.MustParseUUID(f.workspace)
	budgets := NewIssueBudgetService(db.New(f.pool), f.pool)
	memberOf := func(issue pgtype.UUID) (root, parent, source string) {
		t.Helper()
		var boundParent *string
		err := f.pool.QueryRow(ctx, `SELECT budget_root_id::text, bound_parent_issue_id::text, bind_source FROM issue_budget_member WHERE workspace_id=$1 AND issue_id=$2`, f.workspace, issue).Scan(&root, &boundParent, &source)
		if errors.Is(err, pgx.ErrNoRows) {
			return "", "", ""
		}
		if err != nil {
			t.Fatal(err)
		}
		if boundParent != nil {
			parent = *boundParent
		}
		return root, parent, source
	}

	// S5 first: the fixture's collaboration issue is outside every root.
	// Moving it under a member of one binds it.
	shared := f.issue.Issue
	parent := f.fx.Issue(t, "Ordinary budget parent")
	f.syncCounter(t)
	enabled, err := budgets.Enable(ctx, IssueBudgetEnableParams{WorkspaceID: ws, IssueID: util.MustParseUUID(parent),
		OperationKey: "enable-" + uuid.NewString(), Reason: "track the ordinary parent", ActorUserID: util.MustParseUUID(f.creator)})
	if err != nil {
		t.Fatalf("enable: %v", err)
	}
	if root, _, _ := memberOf(shared.ID); root != "" {
		t.Fatal("the collaboration issue is already a member before it was moved")
	}
	parentID := util.MustParseUUID(parent)
	receipt, err := f.svc.EditIssue(f.ctx, CollaborationEditParams{CollaborationWorkflowRequest: f.req(t, f.ctx), ParentIssueID: &parentID})
	if err != nil || receipt.Code != "issue_edited" {
		t.Fatalf("S5 UpdateCollaborationIssueStructure: %+v %v", receipt, err)
	}
	if root, through, source := memberOf(shared.ID); root != enabled.BudgetRootID || through != parent || source != "reparent" {
		t.Fatalf("S5 UpdateCollaborationIssueStructure: member row (%q, %q, %q), want root %s through %s by reparent", root, through, source, enabled.BudgetRootID, parent)
	}

	// C7: a collaboration issue created directly under the ordinary parent.
	p := f.params(t)
	p.ParentIssueID = parentID
	created, err := f.svc.Create(f.ctx, p)
	if err != nil {
		t.Fatalf("C7 collaboration create: %v", err)
	}
	if root, through, source := memberOf(created.Issue.ID); root != enabled.BudgetRootID || through != parent || source != "create" {
		t.Fatalf("C7 collaboration create: member row (%q, %q, %q), want root %s through %s by create", root, through, source, enabled.BudgetRootID, parent)
	}

	// C8: accepting a stage proposal creates ordinary children under the
	// collaboration issue, which is a member by now.
	f.revise(t, true)
	f.configure(t)
	f.confirmScope(t)
	proposal := f.propose(t, f.handlerCtx, "Stage child one", "Stage child two")
	decided, err := f.svc.DecideStageProposal(f.ctx, f.decision(t, f.ctx, proposal, "accept", ""))
	if err != nil || decided.Code != "stage_proposal_accepted" {
		t.Fatalf("C8 stage child: %+v %v", decided, err)
	}
	rows, err := f.pool.Query(ctx, `SELECT id FROM issue WHERE workspace_id=$1 AND parent_issue_id=$2`, f.workspace, shared.ID)
	if err != nil {
		t.Fatal(err)
	}
	staged, err := pgx.CollectRows(rows, pgx.RowTo[pgtype.UUID])
	if err != nil || len(staged) != 2 {
		t.Fatalf("C8 stage child: materialized %d children (%v), want 2", len(staged), err)
	}
	for _, child := range staged {
		if root, through, source := memberOf(child); root != enabled.BudgetRootID || through != util.UUIDToString(shared.ID) || source != "create" {
			t.Fatalf("C8 stage child: member row (%q, %q, %q), want root %s through the collaboration issue by create", root, through, source, enabled.BudgetRootID)
		}
	}
}

// 9.3 through the collaboration writer: only an edit that changes the parent
// reaches the binding rules. A structure edit that keeps the parent, and a
// plain field edit, record nothing.
func TestIssueBudgetCollaborationEditBindsOnlyWhenTheParentChanges(t *testing.T) {
	f := newCollaborationStageFixture(t)
	issueBudgetCleanWorkspace(t, f.fx, f.workspace)
	ctx := context.Background()
	budgets := NewIssueBudgetService(db.New(f.pool), f.pool)
	shared := util.UUIDToString(f.issue.Issue.ID)
	first := f.fx.Issue(t, "First budget parent")
	second := f.fx.Issue(t, "Second budget parent")
	enable := func(issue string) string {
		t.Helper()
		result, err := budgets.Enable(ctx, IssueBudgetEnableParams{WorkspaceID: util.MustParseUUID(f.workspace), IssueID: util.MustParseUUID(issue),
			OperationKey: "enable-" + uuid.NewString(), Reason: "track the ordinary parent", ActorUserID: util.MustParseUUID(f.creator)})
		if err != nil {
			t.Fatalf("enable: %v", err)
		}
		return result.BudgetRootID
	}
	own, other := enable(first), enable(second)
	edit := func(what string, p CollaborationEditParams) {
		t.Helper()
		p.CollaborationWorkflowRequest = f.req(t, f.ctx)
		receipt, err := f.svc.EditIssue(f.ctx, p)
		if err != nil || receipt.Code != "issue_edited" {
			t.Fatalf("%s: %+v %v", what, receipt, err)
		}
	}
	rows := func() map[string]string {
		t.Helper()
		out := map[string]string{}
		for _, table := range issueBudgetTables {
			out[table] = issueBudgetRows(t, f.fx, table, f.workspace)
		}
		return out
	}

	firstID, secondID := util.MustParseUUID(first), util.MustParseUUID(second)
	edit("move under the first root", CollaborationEditParams{ParentIssueID: &firstID})
	edit("move under the second root", CollaborationEditParams{ParentIssueID: &secondID})
	// Rule 1 through this writer: the member row stays as written, and the
	// move is recorded once, on the issue's own root.
	var root, through, source string
	f.fx.QueryRow(t, `SELECT budget_root_id::text, bound_parent_issue_id::text, bind_source FROM issue_budget_member WHERE workspace_id=$1 AND issue_id=$2`, f.workspace, shared).Scan(&root, &through, &source)
	if root != own || through != first || source != "reparent" {
		t.Fatalf("member row (%q, %q, %q), want root %s through %s by reparent", root, through, source, own, first)
	}
	if got := f.fx.Count(t, `SELECT count(*) FROM issue_budget_observation WHERE workspace_id=$1`, f.workspace); got != 1 {
		t.Fatalf("%d observations after the two moves, want 1", got)
	}
	var kind, reason, observedRoot, issue, newParent, newRoot string
	f.fx.QueryRow(t, `SELECT kind, reason, budget_root_id::text, issue_id::text, detail->>'new_parent_issue_id', detail->>'new_parent_budget_root_id'
		FROM issue_budget_observation WHERE workspace_id=$1`, f.workspace).Scan(&kind, &reason, &observedRoot, &issue, &newParent, &newRoot)
	if kind != "would_refuse" || reason != "root_conflict" || observedRoot != own || issue != shared || newParent != second || newRoot != other {
		t.Fatalf("observation = %s:%s on root %s for %s (new parent %s, root %s)", kind, reason, observedRoot, issue, newParent, newRoot)
	}

	committed := rows()
	edit("a stage change under the same parent", CollaborationEditParams{Stage: &pgtype.Int4{Int32: 3, Valid: true}})
	edit("a stage change that names the parent it already has", CollaborationEditParams{ParentIssueID: &secondID, Stage: &pgtype.Int4{Int32: 4, Valid: true}})
	title := "Retitled collaboration issue"
	edit("a title change", CollaborationEditParams{Title: &title})
	after := rows()
	for _, table := range issueBudgetTables {
		if after[table] != committed[table] {
			t.Fatalf("an edit that kept the parent changed %s\nbefore: %s\nafter:  %s", table, committed[table], after[table])
		}
	}
}
