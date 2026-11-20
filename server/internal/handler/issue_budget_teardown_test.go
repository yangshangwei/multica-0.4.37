package handler

import (
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/multica-ai/multica/server/internal/budget"
	"github.com/multica-ai/multica/server/internal/testutil"
)

// Workspace teardown classifies the budget tables as retain_audit. It must
// neither write nor lock a budget row: the row versions and lock markers of
// every budget row are identical before and after the workspace is deleted.
func TestWorkspaceDeletionLeavesBudgetRowsUntouched(t *testing.T) {
	t.Setenv("MULTICA_AUTH_MODE", "legacy")
	workspace := dbfx.Workspace(t, "Budget teardown workspace", "budget-teardown-"+uuid.NewString())
	dbfx.Member(t, workspace, testUserID, "owner")
	parent := dbfx.Issue(t, "Budget teardown root", testutil.Cols{"workspace_id": workspace})
	child := dbfx.Issue(t, "Budget teardown member", testutil.Cols{"workspace_id": workspace, "parent_issue_id": parent})
	in := func(cols testutil.Cols) testutil.Cols {
		cols["workspace_id"] = workspace
		return cols
	}

	root := dbfx.Insert(t, "issue_budget_root", in(issueBudgetRootCols(parent)))
	insertIssueBudgetMember(t, in(issueBudgetMemberCols(root, parent, "")))
	insertIssueBudgetMember(t, in(issueBudgetMemberCols(root, child, parent)))
	attempt := dbfx.Insert(t, "issue_budget_attempt", in(issueBudgetAttemptCols(root, uuid.NewString(), child)))
	reserve := budget.Entry{ID: "reserve-1", Kind: budget.KindReserve, ReservationID: attempt, ReservedTicks: 50}
	_, receipts, err := budget.Reduce([]budget.Entry{reserve})
	if err != nil {
		t.Fatalf("reduce fixture entry: %v", err)
	}
	insertIssueBudgetEntry(t, in(issueBudgetEntryCols(t, root, 1, reserve, receipts[0])))
	dbfx.Insert(t, "issue_budget_operation", testutil.Cols{
		"id": uuid.NewString(), "workspace_id": workspace, "budget_root_id": root, "scope_kind": "issue", "scope_id": parent,
		"operation_key": "activate-1", "operation_kind": "activate", "intent_digest": "0000000000000000000000000000000000000000000000000000000000000000",
		"actor_type": "member", "actor_id": testUserID, "result": `{"budget_root_id":"` + root + `"}`,
	})
	dbfx.Insert(t, "issue_budget_observation", testutil.Cols{
		"id": uuid.NewString(), "workspace_id": workspace, "budget_root_id": root,
		"kind": "coverage_gap", "reason": "legacy_protocol", "attempt_id": attempt,
	})

	// xmin changes when a row is rewritten; xmax is set by an update, a delete
	// and by every row-level lock, including the key-share lock a foreign key
	// check would take.
	snapshot := func() map[string]string {
		out := map[string]string{}
		for _, table := range issueBudgetTables {
			var rows string
			dbfx.QueryRow(t, `SELECT count(*)::text || ' rows' || coalesce(string_agg(
					E'\n' || t.xmin::text || ' ' || t.xmax::text || ' ' || t.ctid::text || ' ' || to_jsonb(t)::text, '' ORDER BY t.ctid), '')
				FROM `+table+` t WHERE t.workspace_id = $1`, workspace).Scan(&rows)
			out[table] = rows
		}
		return out
	}
	before := snapshot()
	for table, want := range map[string]string{
		"issue_budget_root": "1 rows", "issue_budget_member": "2 rows", "issue_budget_attempt": "1 rows",
		"issue_budget_entry": "1 rows", "issue_budget_operation": "1 rows", "issue_budget_observation": "1 rows",
	} {
		if got := before[table]; len(got) < len(want) || got[:len(want)] != want {
			t.Fatalf("%s fixture: %q, want %s", table, got, want)
		}
	}

	r := newRequest(http.MethodDelete, "/api/workspaces/"+workspace, nil)
	r.Header.Set("X-Workspace-ID", workspace)
	testutil.Call(t, testHandler.DeleteWorkspace, withURLParam(r, "id", workspace)).Want(http.StatusNoContent)

	if got := dbfx.Count(t, `SELECT count(*) FROM workspace WHERE id = $1`, workspace); got != 0 {
		t.Fatalf("workspace rows after teardown = %d, want 0", got)
	}
	if got := dbfx.Count(t, `SELECT count(*) FROM issue WHERE id = ANY($1::uuid[])`, []string{parent, child}); got != 0 {
		t.Fatalf("issue rows after teardown = %d, want 0", got)
	}
	after := snapshot()
	for _, table := range issueBudgetTables {
		if after[table] != before[table] {
			t.Errorf("%s was written or locked by workspace teardown:\n before %s\n after  %s", table, before[table], after[table])
		}
	}
}
