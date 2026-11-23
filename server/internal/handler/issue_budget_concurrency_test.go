package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
)

// The lock order of design section 5 with the requests racing each other:
// enabling a root, managing it, and creating, moving and deleting issues of
// its tree at once, each in its own transaction on its own connection.
//
// Nothing here can deadlock at this step by construction. An issue write
// takes the workspace fence and never locks a root row; a management
// operation locks its root row and takes nothing else. That statement order
// is pinned in internal/service (TestIssueBudgetTransactionsFollowTheLockOrder
// and TestIssueBudgetBindingNeverLocksOrUpdatesARootRow). This test is the net under
// it: the real handler paths, where a deadlock PostgreSQL breaks comes back as
// a 5xx or a plain error, and where the rows left behind must be the ones a
// serial order of the same requests leaves.
//
// The enable starts after a different number of issue writes in every tree,
// so it lands at the head, in the middle and near the tail of the writers.
func TestIssueBudgetConcurrentWritersKeepEveryTreeBound(t *testing.T) {
	marker := "Budget race " + uuid.NewString()
	// Issues created over HTTP below; fixture issues remove themselves.
	dbfx.Cleanup(t, `DELETE FROM issue WHERE workspace_id=$1 AND title LIKE $2`, testWorkspaceID, marker+"%")
	// A request holds one pooled connection for its transaction and may
	// borrow another; this many at once leaves the pool room for that.
	width := min(max(int(testPool.Config().MaxConns)-4, 2), 8)
	for tree := range 12 {
		raceIssueBudgetTree(t, fmt.Sprintf("%s tree %d", marker, tree), tree%6, tree%2 == 1, width)
		if t.Failed() {
			t.FailNow()
		}
	}
}

// raceIssueBudgetTree builds one tree and releases every request on it at
// once. enableAfter is the number of issue writes that have answered before
// the enable starts. With contested, a second enable targets the kept child:
// whichever commits first is the tree's only root.
func raceIssueBudgetTree(t *testing.T, name string, enableAfter int, contested bool, width int) {
	t.Helper()
	title := func(what string) string { return name + " " + what }
	top := dbfx.Issue(t, title("top"))
	kept := dbfx.Issue(t, title("kept child"), testutil.Cols{"parent_issue_id": top})
	leaf := dbfx.Issue(t, title("deleted leaf"), testutil.Cols{"parent_issue_id": kept})
	doomed := dbfx.Issue(t, title("deleted child"), testutil.Cols{"parent_issue_id": top})
	orphan := dbfx.Issue(t, title("child of the deleted child"), testutil.Cols{"parent_issue_id": doomed})
	loose := dbfx.Issue(t, title("moved in"))
	below := dbfx.Issue(t, title("child of the one moved in"), testutil.Cols{"parent_issue_id": loose})
	visitor := dbfx.Issue(t, title("moved in and out"))
	syncIssueCounterForBudget(t, testWorkspaceID)

	budgets := service.NewIssueBudgetService(testHandler.Queries, testPool)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	var (
		mu       sync.Mutex
		problems []string
		created  []string
		written  atomic.Int32
		open     sync.Once
		workers  sync.WaitGroup
	)
	fail := func(format string, args ...any) {
		mu.Lock()
		defer mu.Unlock()
		problems = append(problems, fmt.Sprintf(format, args...))
	}
	start, gate, slots := make(chan struct{}), make(chan struct{}), make(chan struct{}, width)
	wrote := func() {
		if int(written.Add(1)) >= enableAfter {
			open.Do(func() { close(gate) })
		}
	}
	run := func(after <-chan struct{}, steps func()) {
		workers.Go(func() {
			<-start
			<-after
			slots <- struct{}{}
			defer func() { <-slots }()
			steps()
		})
	}
	// Assertions stay on the test goroutine: a worker only records.
	send := func(what string, h http.HandlerFunc, method, path string, body any, id string, want ...int) *testutil.Response {
		req := newRequest(method, path, body).WithContext(ctx)
		if id != "" {
			req = withURLParam(req, "id", id)
		}
		resp := testutil.Call(t, h, req)
		wrote()
		if !slices.Contains(want, resp.Code) {
			fail("%s: %s %s answered %d: %s", what, method, path, resp.Code, resp.Body.String())
		}
		return resp
	}
	create := func(what, parent string, want ...int) {
		resp := send(what, testHandler.CreateIssue, http.MethodPost, "/api/issues",
			map[string]any{"title": title(what), "parent_issue_id": parent}, "", want...)
		if resp.Code != http.StatusCreated {
			return
		}
		var out struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(resp.Body.Bytes(), &out); err != nil || out.ID == "" {
			fail("%s: no issue id in %s", what, resp.Body.String())
			return
		}
		mu.Lock()
		defer mu.Unlock()
		created = append(created, out.ID)
	}
	move := func(what, issue string, parent any) {
		send(what, testHandler.UpdateIssue, http.MethodPut, "/api/issues/"+issue, map[string]any{"parent_issue_id": parent}, issue, http.StatusOK)
	}
	remove := func(what, issue string) {
		send(what, testHandler.DeleteIssue, http.MethodDelete, "/api/issues/"+issue, nil, issue, http.StatusNoContent)
	}
	type manage func(service.IssueBudgetManageParams) (service.IssueBudgetOperationResult, error)
	enable := func(what, issue string) {
		enabled, err := budgets.Enable(ctx, service.IssueBudgetEnableParams{
			WorkspaceID: parseUUID(testWorkspaceID), IssueID: parseUUID(issue),
			OperationKey: "enable-" + uuid.NewString(), Reason: "concurrent writers", ActorUserID: parseUUID(testUserID),
		})
		if contested && service.IssueBudgetErrorCode(err) == service.IssueBudgetRootConflict {
			return
		}
		if err != nil {
			fail("%s: %v", what, err)
			return
		}
		// The root is managed while the issue writes are still running.
		for revision, step := range []struct {
			name string
			op   manage
		}{
			{"close", func(p service.IssueBudgetManageParams) (service.IssueBudgetOperationResult, error) {
				return budgets.Close(ctx, p)
			}},
			{"reopen", func(p service.IssueBudgetManageParams) (service.IssueBudgetOperationResult, error) {
				return budgets.Reopen(ctx, p)
			}},
			{"mode off", func(p service.IssueBudgetManageParams) (service.IssueBudgetOperationResult, error) {
				return budgets.SetMode(ctx, p, service.IssueBudgetModeOff)
			}},
			{"mode observe", func(p service.IssueBudgetManageParams) (service.IssueBudgetOperationResult, error) {
				return budgets.SetMode(ctx, p, service.IssueBudgetModeObserve)
			}},
		} {
			if _, err := step.op(service.IssueBudgetManageParams{
				WorkspaceID: parseUUID(testWorkspaceID), BudgetRootID: parseUUID(enabled.BudgetRootID),
				OperationKey: step.name + "-" + uuid.NewString(), ExpectedPolicyRevision: int64(revision + 1),
				Reason: "concurrent writers", ActorUserID: parseUUID(testUserID),
			}); err != nil {
				fail("%s, then %s: %v", what, step.name, err)
				return
			}
		}
	}

	run(gate, func() { enable("enable the top issue", top) })
	if contested {
		run(gate, func() { enable("enable the kept child", kept) })
	}
	run(start, func() { create("created under the top issue", top, http.StatusCreated) })
	run(start, func() { create("created under the kept child", kept, http.StatusCreated) })
	// Its parent is being deleted: created and then detached, or refused.
	run(start, func() {
		create("created under the deleted child", doomed, http.StatusCreated, http.StatusBadRequest)
	})
	run(start, func() { move("move under the kept child", loose, kept) })
	run(start, func() {
		move("move under the top issue", visitor, top)
		move("move out of the tree", visitor, nil)
	})
	run(start, func() { remove("delete the leaf", leaf) })
	run(start, func() { remove("delete the child with a child", doomed) })
	if enableAfter == 0 {
		open.Do(func() { close(gate) })
	}
	close(start)
	workers.Wait()

	issues := append([]string{top, kept, leaf, doomed, orphan, loose, below, visitor}, created...)
	var roots []string
	dbfx.QueryRow(t, `SELECT COALESCE(array_agg(id::text ORDER BY id), '{}') FROM issue_budget_root
		WHERE workspace_id=$1 AND root_issue_id = ANY($2::uuid[])`, testWorkspaceID, []string{top, kept}).Scan(&roots)
	// Budget rows have no foreign key: deleting the fixture leaves them.
	for _, root := range roots {
		dbfx.Cleanup(t, `DELETE FROM issue_budget_root WHERE id=$1`, root)
		for _, table := range issueBudgetTables[1:] {
			dbfx.Cleanup(t, `DELETE FROM `+table+` WHERE budget_root_id=$1`, root)
		}
	}
	var sources string
	dbfx.QueryRow(t, `SELECT COALESCE(jsonb_object_agg(bind_source, n), '{}'::jsonb)::text FROM (
		SELECT bind_source, count(*) AS n FROM issue_budget_member WHERE budget_root_id = ANY($1::uuid[]) GROUP BY bind_source) s`, roots).Scan(&sources)
	t.Logf("%s: enable after %d writes, contested %v: member rows %s", name, enableAfter, contested, sources)

	for _, problem := range problems {
		t.Errorf("%s: %s", name, problem)
	}
	if len(roots) != 1 {
		t.Errorf("%s has %d roots, want 1", name, len(roots))
	}
	for _, check := range []struct {
		what string
		want int
		sql  string
		args []any
	}{
		{"issues under a bound parent without a member row of its root", 0, `
			SELECT count(*) FROM issue i
			JOIN issue_budget_member p ON p.workspace_id = i.workspace_id AND p.issue_id = i.parent_issue_id
			LEFT JOIN issue_budget_member m ON m.workspace_id = i.workspace_id AND m.issue_id = i.id
			WHERE i.workspace_id = $1 AND i.id = ANY($2::uuid[]) AND m.budget_root_id IS DISTINCT FROM p.budget_root_id`,
			[]any{testWorkspaceID, issues}},
		{"member rows whose tombstone disagrees with the issue being gone", 0, `
			SELECT count(*) FROM issue_budget_member m
			WHERE m.budget_root_id = ANY($1::uuid[])
			  AND (m.tombstoned_at IS NOT NULL) = EXISTS (SELECT 1 FROM issue i WHERE i.id = m.issue_id)`,
			[]any{roots}},
		{"member rows of this tree under another root", 0, `
			SELECT count(*) FROM issue_budget_member
			WHERE workspace_id = $1 AND issue_id = ANY($2::uuid[]) AND NOT (budget_root_id = ANY($3::uuid[]))`,
			[]any{testWorkspaceID, issues, roots}},
		{"roots with the member row of their root issue", len(roots), `
			SELECT count(*) FROM issue_budget_root r
			JOIN issue_budget_member m ON m.budget_root_id = r.id AND m.issue_id = r.root_issue_id AND m.bind_source = 'root'
			WHERE r.id = ANY($1::uuid[])`,
			[]any{roots}},
		// No move here crosses a root, loops or meets a running task.
		{"other observations", 0, `SELECT count(*) FROM issue_budget_observation WHERE budget_root_id = ANY($1::uuid[]) AND reason <> 'observation_off'`, []any{roots}},
		{"off-period gaps", len(roots), `SELECT count(*) FROM issue_budget_observation WHERE budget_root_id = ANY($1::uuid[]) AND kind='coverage_gap' AND reason='observation_off'`, []any{roots}},
		{"roots left open, observing, at policy revision 5", len(roots), `
			SELECT count(*) FROM issue_budget_root
			WHERE id = ANY($1::uuid[]) AND policy_revision = 5 AND admission_state = 'open' AND mode = 'observe'`,
			[]any{roots}},
		{"operations", 5 * len(roots), `SELECT count(*) FROM issue_budget_operation WHERE budget_root_id = ANY($1::uuid[])`, []any{roots}},
	} {
		if got := dbfx.Count(t, check.sql, check.args...); got != check.want {
			t.Errorf("%s: %d %s, want %d", name, got, check.what, check.want)
		}
	}
}
