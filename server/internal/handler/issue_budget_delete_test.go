package handler

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/budget"
	"github.com/multica-ai/multica/server/internal/testutil"
)

// B56: deleting a member issue physically removes its tasks, usage rows and
// task tokens through the existing foreign-key cascade. The budget rows hold
// ID copies and no foreign key, so the root, the member binding, the attempt
// and the ledger entries all stay, and the root still shows the 30 ticks the
// deleted issue spent. Cancelling the task inside the delete releases nothing.
// The one thing the delete writes is the tombstone time on the deleted
// issue's own member row.
func TestDeleteMemberIssueKeepsBudgetRows(t *testing.T) {
	parent := dbfx.Issue(t, "Budget root parent")
	child := dbfx.Issue(t, "Budget member to delete", testutil.Cols{"parent_issue_id": parent})
	runtime := dbfx.Runtime(t, "Budget delete runtime")
	agent := dbfx.Agent(t, "Budget delete agent", runtime)
	task := dbfx.Task(t, agent, testutil.Cols{"issue_id": child, "runtime_id": runtime, "status": "running"})
	dbfx.Insert(t, "task_usage", testutil.Cols{
		"task_id": task, "provider": "fixture", "model": "fixture", "input_tokens": 10, "output_tokens": 2,
		"cache_read_tokens": 0, "cache_write_tokens": 0, "cost_usd_ticks": 30,
	})
	token, err := auth.GenerateAgentTaskToken()
	if err != nil {
		t.Fatal(err)
	}
	dbfx.Insert(t, "task_token", testutil.Cols{
		"token_hash": auth.HashToken(token), "task_id": task, "agent_id": agent,
		"workspace_id": testWorkspaceID, "user_id": testUserID, "expires_at": time.Now().Add(time.Hour),
	})

	rootCols := issueBudgetRootCols(parent)
	rootCols["ledger_revision"] = 2
	rootCols["actual_known_ticks"] = 30
	rootCols["reserved_ticks"] = 20
	root := dbfx.Insert(t, "issue_budget_root", rootCols)
	insertIssueBudgetMember(t, issueBudgetMemberCols(root, parent, ""))
	insertIssueBudgetMember(t, issueBudgetMemberCols(root, child, parent))
	attemptCols := issueBudgetAttemptCols(root, task, child)
	attemptCols["runtime_id"], attemptCols["agent_id"] = runtime, agent
	attempt := dbfx.Insert(t, "issue_budget_attempt", attemptCols)

	entries := []budget.Entry{
		{ID: "reserve-1", Kind: budget.KindReserve, ReservationID: attempt, ReservedTicks: 50},
		{ID: "usage-1", Kind: budget.KindUsage, ReservationID: attempt, Usage: budget.Usage{
			ScopeID: "request-1", InvocationID: "invocation-1", Mode: budget.ModeDelta,
			Cost: budget.Cost{Basis: budget.CostReported, Ticks: 30, Currency: budget.CurrencyUSD},
		}},
	}
	projection, receipts, err := budget.Reduce(entries)
	if err != nil {
		t.Fatalf("reduce fixture ledger: %v", err)
	}
	// The fixture's stored projection is what the metering package computes
	// for these two entries, not a number chosen to match the assertion below.
	if projection.ActualKnown != 30 || projection.Reserved != 20 || projection.UnknownHold != 0 {
		t.Fatalf("fixture ledger projects A=%d R=%d H=%d, want 30/20/0", projection.ActualKnown, projection.Reserved, projection.UnknownHold)
	}
	for i, entry := range entries {
		insertIssueBudgetEntry(t, issueBudgetEntryCols(t, root, i+1, entry, receipts[i]))
	}

	snapshot := func() map[string]string {
		out := map[string]string{}
		for name, query := range map[string]string{
			"root":          `SELECT to_jsonb(r)::text FROM issue_budget_root r WHERE r.id = $1`,
			"root member":   `SELECT to_jsonb(m)::text FROM issue_budget_member m WHERE m.budget_root_id = $1 AND m.bind_source = 'root'`,
			"child member":  `SELECT (to_jsonb(m) - 'tombstoned_at')::text FROM issue_budget_member m WHERE m.budget_root_id = $1 AND m.bind_source = 'create'`,
			"attempt":       `SELECT to_jsonb(a)::text FROM issue_budget_attempt a WHERE a.budget_root_id = $1`,
			"reserve entry": `SELECT to_jsonb(e)::text FROM issue_budget_entry e WHERE e.budget_root_id = $1 AND e.seq = 1`,
			"usage entry":   `SELECT to_jsonb(e)::text FROM issue_budget_entry e WHERE e.budget_root_id = $1 AND e.seq = 2`,
		} {
			// A row the delete removed reads as "gone" instead of aborting the
			// test, so every lost row is reported.
			var row string
			dbfx.QueryRow(t, `SELECT coalesce((`+query+`), 'gone')`, root).Scan(&row)
			out[name] = row
		}
		return out
	}
	before := snapshot()
	for name, row := range before {
		if row == "gone" {
			t.Fatalf("fixture has no %s row", name)
		}
	}
	tombstoned := func(issue string) bool {
		var marked bool
		dbfx.QueryRow(t, `SELECT tombstoned_at IS NOT NULL FROM issue_budget_member WHERE budget_root_id = $1 AND issue_id = $2`, root, issue).Scan(&marked)
		return marked
	}
	if tombstoned(child) || tombstoned(parent) {
		t.Fatal("fixture member rows are already tombstoned")
	}

	testutil.Call(t, testHandler.DeleteIssue,
		withURLParam(newRequest(http.MethodDelete, "/api/issues/"+child, nil), "id", child)).Want(http.StatusNoContent)

	// The cascade really ran: the issue and everything keyed to its task are gone.
	for name, query := range map[string]string{
		"issue":            `SELECT count(*) FROM issue WHERE id = $1`,
		"agent_task_queue": `SELECT count(*) FROM agent_task_queue WHERE issue_id = $1`,
	} {
		if got := dbfx.Count(t, query, child); got != 0 {
			t.Errorf("%s rows of the deleted issue = %d, want 0", name, got)
		}
	}
	for name, query := range map[string]string{
		"agent_task_queue": `SELECT count(*) FROM agent_task_queue WHERE id = $1`,
		"task_usage":       `SELECT count(*) FROM task_usage WHERE task_id = $1`,
		"task_token":       `SELECT count(*) FROM task_token WHERE task_id = $1`,
	} {
		if got := dbfx.Count(t, query, task); got != 0 {
			t.Errorf("%s rows of the deleted issue's task = %d, want 0", name, got)
		}
	}
	if got := dbfx.Count(t, `SELECT count(*) FROM issue WHERE id = $1`, parent); got != 1 {
		t.Fatalf("parent issue rows = %d, want 1", got)
	}

	// Every budget row is still there with the values it had. The deleted
	// issue's member row differs in its tombstone time and in nothing else;
	// the root issue's member row is not marked.
	after := snapshot()
	for name, want := range before {
		if after[name] != want {
			t.Errorf("%s changed across the delete:\n before %s\n after  %s", name, want, after[name])
		}
	}
	if !tombstoned(child) {
		t.Error("the deleted issue's member row carries no tombstone time")
	}
	if tombstoned(parent) {
		t.Error("the delete tombstoned the root issue's member row")
	}
	for name, c := range map[string]struct {
		query string
		want  int
	}{
		"members":      {`SELECT count(*) FROM issue_budget_member WHERE budget_root_id = $1`, 2},
		"attempts":     {`SELECT count(*) FROM issue_budget_attempt WHERE budget_root_id = $1`, 1},
		"entries":      {`SELECT count(*) FROM issue_budget_entry WHERE budget_root_id = $1`, 2},
		"observations": {`SELECT count(*) FROM issue_budget_observation WHERE budget_root_id = $1`, 0},
	} {
		if got := dbfx.Count(t, c.query, root); got != c.want {
			t.Errorf("budget %s after the delete = %d, want %d", name, got, c.want)
		}
	}

	// The root's stored actual is still 30 and the cancelled task's
	// reservation is still held: nothing was released.
	var actual, reserved, revision int64
	var attemptState string
	dbfx.QueryRow(t, `SELECT actual_known_ticks, reserved_ticks, ledger_revision FROM issue_budget_root WHERE id = $1`, root).Scan(&actual, &reserved, &revision)
	dbfx.QueryRow(t, `SELECT state FROM issue_budget_attempt WHERE id = $1`, attempt).Scan(&attemptState)
	if actual != 30 || reserved != 20 || revision != 2 || attemptState != "open" {
		t.Errorf("root A=%d R=%d revision=%d, attempt state %q; want 30, 20, 2, open", actual, reserved, revision, attemptState)
	}

	// The surviving entries still replay to the same amounts.
	rows, err := testPool.Query(t.Context(), `SELECT entry_json FROM issue_budget_entry WHERE budget_root_id = $1 ORDER BY seq`, root)
	if err != nil {
		t.Fatalf("read surviving entries: %v", err)
	}
	defer rows.Close()
	var stored []budget.Entry
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			t.Fatalf("scan entry: %v", err)
		}
		var entry budget.Entry
		if err := json.Unmarshal(raw, &entry); err != nil {
			t.Fatalf("decode stored entry: %v", err)
		}
		stored = append(stored, entry)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate entries: %v", err)
	}
	replayed, _, err := budget.Reduce(stored)
	if err != nil {
		t.Fatalf("replay surviving entries: %v", err)
	}
	if replayed.ActualKnown != 30 || replayed.Reserved != 20 || replayed.UnknownHold != 0 || len(replayed.Unknown) != 0 {
		t.Errorf("surviving entries replay to A=%d R=%d H=%d unknown=%d, want 30/20/0/0",
			replayed.ActualKnown, replayed.Reserved, replayed.UnknownHold, len(replayed.Unknown))
	}
}
