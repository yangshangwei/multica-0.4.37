package handler

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
)

// Rows of the F3 acceptance matrix that go through an HTTP issue write:
// B09-B13, B57 and the handler paths of B60. The binding rules themselves,
// enabling and the management operations are in
// internal/service/issue_budget_test.go; B56 is in
// issue_budget_delete_test.go.

// enableIssueBudget makes a fixture issue a root issue through the service,
// as the management API will, and removes the budget rows afterwards: they
// have no foreign key, so deleting the fixture issue leaves them.
func enableIssueBudget(t *testing.T, workspace, issue string) string {
	t.Helper()
	result, err := service.NewIssueBudgetService(testHandler.Queries, testPool).Enable(context.Background(), service.IssueBudgetEnableParams{
		WorkspaceID: parseUUID(workspace), IssueID: parseUUID(issue),
		OperationKey: "enable-" + uuid.NewString(), Reason: "handler seam test", ActorUserID: parseUUID(testUserID),
	})
	if err != nil {
		t.Fatalf("enable budget root: %v", err)
	}
	dbfx.Cleanup(t, `DELETE FROM issue_budget_root WHERE id=$1`, result.BudgetRootID)
	for _, table := range issueBudgetTables[1:] {
		dbfx.Cleanup(t, `DELETE FROM `+table+` WHERE budget_root_id=$1`, result.BudgetRootID)
	}
	return result.BudgetRootID
}

// issueBudgetRootRows is every budget row of one root, table by table.
func issueBudgetRootRows(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, table := range issueBudgetTables {
		column := "budget_root_id"
		if table == "issue_budget_root" {
			column = "id"
		}
		var rows string
		dbfx.QueryRow(t, `SELECT COALESCE(jsonb_agg(to_jsonb(x) ORDER BY to_jsonb(x)::text), '[]'::jsonb)::text FROM `+table+` x WHERE x.`+column+`=$1`, root).Scan(&rows)
		out[table] = rows
	}
	return out
}

func requireIssueBudgetRootRows(t *testing.T, root string, want map[string]string, what string, except ...string) {
	t.Helper()
	got := issueBudgetRootRows(t, root)
	for _, table := range issueBudgetTables {
		skip := false
		for _, name := range except {
			skip = skip || name == table
		}
		if !skip && got[table] != want[table] {
			t.Fatalf("%s changed %s\nbefore: %s\nafter:  %s", what, table, want[table], got[table])
		}
	}
}

type issueBudgetBinding struct {
	Root, BoundParent, Source string
	Tombstoned                bool
}

// issueBudgetBindingOf reads an issue's member row; ok is false without one.
func issueBudgetBindingOf(t *testing.T, issue string) (row issueBudgetBinding, ok bool) {
	t.Helper()
	var parent *string
	err := testPool.QueryRow(context.Background(), `
		SELECT budget_root_id::text, bound_parent_issue_id::text, bind_source, tombstoned_at IS NOT NULL
		FROM issue_budget_member WHERE issue_id=$1`, issue).Scan(&row.Root, &parent, &row.Source, &row.Tombstoned)
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

func requireIssueBudgetBinding(t *testing.T, what, issue string, want issueBudgetBinding) {
	t.Helper()
	got, ok := issueBudgetBindingOf(t, issue)
	if !ok {
		t.Fatalf("%s: issue %s has no member row, want %+v", what, issue, want)
	}
	if got != want {
		t.Fatalf("%s: member row of %s = %+v, want %+v", what, issue, got, want)
	}
}

func issueBudgetParentOf(t *testing.T, issue string) string {
	t.Helper()
	var parent string
	dbfx.QueryRow(t, `SELECT COALESCE(parent_issue_id::text, '') FROM issue WHERE id=$1`, issue).Scan(&parent)
	return parent
}

func updateIssueForBudget(t *testing.T, issue string, body map[string]any) {
	t.Helper()
	testutil.Call(t, testHandler.UpdateIssue,
		withURLParam(newRequest(http.MethodPut, "/api/issues/"+issue, body), "id", issue)).Want(http.StatusOK)
}

// syncIssueCounterForBudget moves the workspace counter past the numbers
// fixture issues took, so a real creation after them gets a free number.
func syncIssueCounterForBudget(t *testing.T, workspace string) {
	t.Helper()
	dbfx.Exec(t, `UPDATE workspace SET issue_counter = GREATEST(issue_counter, (SELECT COALESCE(max(number), 0) FROM issue WHERE workspace_id = $1)) WHERE id = $1`, workspace)
}

// B09: closing and reopening the business issue is not a budget event. The
// root, its limit and amounts, the members and the operation history are the
// same rows before and after; "reopen" exists only as a management operation.
func TestIssueBudgetBusinessReopenWritesNothing(t *testing.T) {
	parent := dbfx.Issue(t, "Budget parent reopened in business")
	child := dbfx.Issue(t, "Budget member reopened in business", testutil.Cols{"parent_issue_id": parent})
	root := enableIssueBudget(t, testWorkspaceID, parent)
	dbfx.Exec(t, `UPDATE issue_budget_root SET limit_ticks=500, actual_known_ticks=30, reserved_ticks=20, unknown_hold_ticks=7, ledger_revision=3 WHERE id=$1`, root)
	before := issueBudgetRootRows(t, root)

	for _, issue := range []string{child, parent} {
		for _, status := range []string{"done", "todo", "in_progress", "cancelled", "todo"} {
			updateIssueForBudget(t, issue, map[string]any{"status": status})
			var stored string
			dbfx.QueryRow(t, `SELECT status FROM issue WHERE id=$1`, issue).Scan(&stored)
			if stored != status {
				t.Fatalf("issue %s status = %q, want %q", issue, stored, status)
			}
		}
	}
	requireIssueBudgetRootRows(t, root, before, "a business close and reopen")
	if got := dbfx.Count(t, `SELECT count(*) FROM issue_budget_operation WHERE budget_root_id=$1`, root); got != 1 {
		t.Fatalf("%d budget operations, want only the enable", got)
	}
}

// B10: a member that leaves its parent, goes under an issue outside every
// root, or changes project keeps the member row it has. Where the issue is
// now and where its spending goes are two separate facts.
func TestIssueBudgetMoveKeepsTheMemberRow(t *testing.T) {
	parent := dbfx.Issue(t, "Budget parent of a moving member")
	child := dbfx.Issue(t, "Moving budget member", testutil.Cols{"parent_issue_id": parent})
	sibling := dbfx.Issue(t, "Sibling budget member", testutil.Cols{"parent_issue_id": parent})
	rootless := dbfx.Issue(t, "Parent outside every root")
	project := dbfx.Project(t, "Budget move project")
	root := enableIssueBudget(t, testWorkspaceID, parent)
	dbfx.Exec(t, `UPDATE issue_budget_root SET actual_known_ticks=30, ledger_revision=1 WHERE id=$1`, root)
	before := issueBudgetRootRows(t, root)
	want := issueBudgetBinding{Root: root, BoundParent: parent, Source: "activation"}

	for _, move := range []struct {
		name   string
		body   map[string]any
		parent string
	}{
		{"cleared parent", map[string]any{"parent_issue_id": nil}, ""},
		{"under an issue outside every root", map[string]any{"parent_issue_id": rootless}, rootless},
		{"another project", map[string]any{"project_id": project}, rootless},
		{"under another member of its root", map[string]any{"parent_issue_id": sibling}, sibling},
	} {
		updateIssueForBudget(t, child, move.body)
		if got := issueBudgetParentOf(t, child); got != move.parent {
			t.Fatalf("%s: the issue's parent is %q, want %q", move.name, got, move.parent)
		}
		requireIssueBudgetBinding(t, move.name, child, want)
		requireIssueBudgetRootRows(t, root, before, move.name)
	}
	if got := dbfx.Count(t, `SELECT count(*) FROM issue_budget_member WHERE issue_id=$1`, rootless); got != 0 {
		t.Fatal("the issue outside every root was bound by a member moving under it")
	}
}

// B11: moving a member under another root's issue is not blocked in this
// slice. The member stays with its root, the move is recorded on that root as
// one a gate would have refused, and the other root is not touched. A
// management request for the contradictory binding is refused.
func TestIssueBudgetMoveIntoAnotherRootIsRecordedNotRefused(t *testing.T) {
	parent := dbfx.Issue(t, "Budget parent R1")
	child := dbfx.Issue(t, "Member of R1", testutil.Cols{"parent_issue_id": parent})
	otherParent := dbfx.Issue(t, "Budget parent R2")
	root := enableIssueBudget(t, testWorkspaceID, parent)
	other := enableIssueBudget(t, testWorkspaceID, otherParent)
	before, otherBefore := issueBudgetRootRows(t, root), issueBudgetRootRows(t, other)

	updateIssueForBudget(t, child, map[string]any{"parent_issue_id": otherParent})
	if got := issueBudgetParentOf(t, child); got != otherParent {
		t.Fatalf("the move was not applied: parent %q", got)
	}
	requireIssueBudgetBinding(t, "move into another root", child, issueBudgetBinding{Root: root, BoundParent: parent, Source: "activation"})
	requireIssueBudgetRootRows(t, other, otherBefore, "a move into this root")
	requireIssueBudgetRootRows(t, root, before, "a move out of this root", "issue_budget_observation")

	var kind, reason, issue, newParent, newRoot string
	if got := dbfx.Count(t, `SELECT count(*) FROM issue_budget_observation WHERE budget_root_id=$1`, root); got != 1 {
		t.Fatalf("%d observations on the member's own root, want 1", got)
	}
	dbfx.QueryRow(t, `SELECT kind, reason, issue_id::text, detail->>'new_parent_issue_id', detail->>'new_parent_budget_root_id'
		FROM issue_budget_observation WHERE budget_root_id=$1`, root).Scan(&kind, &reason, &issue, &newParent, &newRoot)
	if kind != "would_refuse" || reason != "root_conflict" || issue != child || newParent != otherParent || newRoot != other {
		t.Fatalf("observation = %s:%s on %s (new parent %s, root %s)", kind, reason, issue, newParent, newRoot)
	}

	// 9.3 applies only when the parent changes. Later updates of the issue,
	// one of them naming the parent it already has, record nothing more.
	moved := issueBudgetRootRows(t, root)
	updateIssueForBudget(t, child, map[string]any{"title": "Member of R1, retitled"})
	updateIssueForBudget(t, child, map[string]any{"parent_issue_id": otherParent, "priority": "high"})
	updateIssueForBudget(t, child, map[string]any{"status": "in_progress"})
	if got := issueBudgetParentOf(t, child); got != otherParent {
		t.Fatalf("the later updates moved the issue: parent %q", got)
	}
	requireIssueBudgetRootRows(t, root, moved, "an update that kept the parent")
	requireIssueBudgetRootRows(t, other, otherBefore, "an update that kept the parent")

	_, err := service.NewIssueBudgetService(testHandler.Queries, testPool).Enable(context.Background(), service.IssueBudgetEnableParams{
		WorkspaceID: parseUUID(testWorkspaceID), IssueID: parseUUID(child),
		OperationKey: "enable-" + uuid.NewString(), Reason: "contradictory binding", ActorUserID: parseUUID(testUserID),
	})
	if service.IssueBudgetErrorCode(err) != service.IssueBudgetRootConflict {
		t.Fatalf("enable on a member: %v, want root_conflict", err)
	}
}

// B12: deleting a member leaves its member row, marked. Its own children are
// detached from it by the delete and still belong to the root.
func TestIssueBudgetDeleteMemberLeavesATombstone(t *testing.T) {
	parent := dbfx.Issue(t, "Budget parent of a deleted member")
	child := dbfx.Issue(t, "Budget member to delete", testutil.Cols{"parent_issue_id": parent})
	grandchild := dbfx.Issue(t, "Child of the deleted member", testutil.Cols{"parent_issue_id": child})
	root := enableIssueBudget(t, testWorkspaceID, parent)
	dbfx.Exec(t, `UPDATE issue_budget_root SET actual_known_ticks=30, reserved_ticks=20, ledger_revision=2 WHERE id=$1`, root)
	before := issueBudgetRootRows(t, root)

	testutil.Call(t, testHandler.DeleteIssue,
		withURLParam(newRequest(http.MethodDelete, "/api/issues/"+child, nil), "id", child)).Want(http.StatusNoContent)

	if got := dbfx.Count(t, `SELECT count(*) FROM issue WHERE id=$1`, child); got != 0 {
		t.Fatal("the member issue was not deleted")
	}
	requireIssueBudgetBinding(t, "deleted member", child, issueBudgetBinding{Root: root, BoundParent: parent, Source: "activation", Tombstoned: true})
	requireIssueBudgetBinding(t, "child of the deleted member", grandchild, issueBudgetBinding{Root: root, BoundParent: child, Source: "activation"})
	requireIssueBudgetBinding(t, "root issue", parent, issueBudgetBinding{Root: root, Source: "root"})
	if got := issueBudgetParentOf(t, grandchild); got != "" {
		t.Fatalf("the deleted member's child still has parent %q", got)
	}
	// Nothing but the one tombstone time changed.
	requireIssueBudgetRootRows(t, root, before, "deleting a member", "issue_budget_member")
	if got := dbfx.Count(t, `SELECT count(*) FROM issue_budget_member WHERE budget_root_id=$1`, root); got != 3 {
		t.Fatalf("%d member rows, want 3", got)
	}
	if got := dbfx.Count(t, `SELECT count(*) FROM issue_budget_member WHERE budget_root_id=$1 AND tombstoned_at IS NOT NULL`, root); got != 1 {
		t.Fatalf("%d tombstoned member rows, want only the deleted issue's", got)
	}
}

// B12 through the batch entry: every deleted member is marked in the same
// transaction, and an issue outside every root is deleted as before.
func TestIssueBudgetBatchDeleteTombstonesEveryMember(t *testing.T) {
	parent := dbfx.Issue(t, "Budget parent of a batch delete")
	first := dbfx.Issue(t, "First member in the batch", testutil.Cols{"parent_issue_id": parent})
	second := dbfx.Issue(t, "Second member in the batch", testutil.Cols{"parent_issue_id": first})
	kept := dbfx.Issue(t, "Member outside the batch", testutil.Cols{"parent_issue_id": parent})
	stray := dbfx.Issue(t, "Issue outside every root in the batch")
	root := enableIssueBudget(t, testWorkspaceID, parent)
	before := issueBudgetRootRows(t, root)

	var response struct {
		Deleted int `json:"deleted"`
	}
	testutil.Call(t, testHandler.BatchDeleteIssues, newRequest(http.MethodPost, "/api/issues/batch-delete",
		map[string]any{"issue_ids": []string{first, second, stray}})).Want(http.StatusOK).JSON(&response)
	if response.Deleted != 3 {
		t.Fatalf("batch deleted %d issues, want 3", response.Deleted)
	}
	requireIssueBudgetBinding(t, "first deleted member", first, issueBudgetBinding{Root: root, BoundParent: parent, Source: "activation", Tombstoned: true})
	requireIssueBudgetBinding(t, "second deleted member", second, issueBudgetBinding{Root: root, BoundParent: first, Source: "activation", Tombstoned: true})
	requireIssueBudgetBinding(t, "member outside the batch", kept, issueBudgetBinding{Root: root, BoundParent: parent, Source: "activation"})
	if _, ok := issueBudgetBindingOf(t, stray); ok {
		t.Fatal("deleting an issue outside every root wrote a member row")
	}
	requireIssueBudgetRootRows(t, root, before, "a batch delete", "issue_budget_member")
}

// B13 and B57: deleting the root issue. The root row is not touched; the
// root reads as tombstoned through its root issue's member row. The direct
// children lose their parent pointer and keep their member rows, with the
// parent they were bound through. No survivor can get a root of its own.
func TestIssueBudgetDeleteRootIssueTombstonesTheRoot(t *testing.T) {
	parent := dbfx.Issue(t, "Budget root issue to delete")
	childA := dbfx.Issue(t, "Surviving member A", testutil.Cols{"parent_issue_id": parent})
	childB := dbfx.Issue(t, "Surviving member B", testutil.Cols{"parent_issue_id": parent})
	grandchild := dbfx.Issue(t, "Surviving grandchild", testutil.Cols{"parent_issue_id": childB})
	root := enableIssueBudget(t, testWorkspaceID, parent)
	dbfx.Exec(t, `UPDATE issue_budget_root SET limit_ticks=900, actual_known_ticks=30, reserved_ticks=20, ledger_revision=2 WHERE id=$1`, root)
	before := issueBudgetRootRows(t, root)

	testutil.Call(t, testHandler.DeleteIssue,
		withURLParam(newRequest(http.MethodDelete, "/api/issues/"+parent, nil), "id", parent)).Want(http.StatusNoContent)

	if got := dbfx.Count(t, `SELECT count(*) FROM issue WHERE id=$1`, parent); got != 0 {
		t.Fatal("the root issue was not deleted")
	}
	// An issue write path never writes a root row: the row is the same row.
	requireIssueBudgetRootRows(t, root, before, "deleting the root issue", "issue_budget_member")
	requireIssueBudgetBinding(t, "deleted root issue", parent, issueBudgetBinding{Root: root, Source: "root", Tombstoned: true})
	for _, child := range []string{childA, childB} {
		if got := issueBudgetParentOf(t, child); got != "" {
			t.Fatalf("DetachDirectChildIssues left parent %q on %s", got, child)
		}
		requireIssueBudgetBinding(t, "detached child", child, issueBudgetBinding{Root: root, BoundParent: parent, Source: "activation"})
	}
	if got := issueBudgetParentOf(t, grandchild); got != childB {
		t.Fatalf("the grandchild's parent is %q, want it left under %s", got, childB)
	}
	requireIssueBudgetBinding(t, "grandchild", grandchild, issueBudgetBinding{Root: root, BoundParent: childB, Source: "activation"})

	budgets := service.NewIssueBudgetService(testHandler.Queries, testPool)
	ctx := context.Background()
	state, err := budgets.GetRoot(ctx, parseUUID(testWorkspaceID), parseUUID(root))
	if err != nil || state.Admission() != service.IssueBudgetAdmissionTombstoned || state.Root.AdmissionState != service.IssueBudgetAdmissionOpen {
		t.Fatalf("root after its issue was deleted: %+v %v", state, err)
	}
	afterDelete := issueBudgetRootRows(t, root)
	enable := func(issue string) error {
		_, err := budgets.Enable(ctx, service.IssueBudgetEnableParams{WorkspaceID: parseUUID(testWorkspaceID), IssueID: parseUUID(issue),
			OperationKey: "enable-" + uuid.NewString(), Reason: "a root for a survivor", ActorUserID: parseUUID(testUserID)})
		return err
	}
	if err := enable(parent); service.IssueBudgetErrorCode(err) != service.IssueBudgetAlreadyEnabled {
		t.Fatalf("enable on the deleted root issue: %v, want already_enabled", err)
	}
	for _, survivor := range []string{childA, childB, grandchild} {
		if err := enable(survivor); service.IssueBudgetErrorCode(err) != service.IssueBudgetRootConflict {
			t.Fatalf("enable on a surviving member: %v, want root_conflict", err)
		}
	}
	_, err = budgets.Reopen(ctx, service.IssueBudgetManageParams{WorkspaceID: parseUUID(testWorkspaceID), BudgetRootID: parseUUID(root),
		OperationKey: "reopen-" + uuid.NewString(), ExpectedPolicyRevision: 1, Reason: "reopen a deleted root", ActorUserID: parseUUID(testUserID)})
	if service.IssueBudgetErrorCode(err) != service.IssueBudgetRootDeleted {
		t.Fatalf("reopen of a root whose issue was deleted: %v, want root_deleted", err)
	}
	requireIssueBudgetRootRows(t, root, afterDelete, "refused management requests")
}

// B60: every product path that can put an issue under a parent, or change an
// issue's parent, is asserted by name. The inventory lists five create
// entries that can carry a parent and four statements that write the parent
// column:
//
//	C1  POST /api/issues                            here
//	C2  POST /api/comments/{id}/sub-issues (manual) here
//	C4  lifecycle handoff follow-up                 here
//	C7  CollaborationService.Create                 internal/service, TestIssueBudgetCollaborationWritersBindMembers
//	C8  stage proposal acceptance                   internal/service, same test
//	S3  UpdateIssue                                 here
//	S4  DetachDirectChildIssues                     here
//	S5  UpdateCollaborationIssueStructure           internal/service, same test
//	S6  PrepareWorkspaceDeletionLinks               here
//
// Fixture issues inserted with plain SQL are not product paths: the member
// rows asserted below were written by the path under test.
func TestIssueBudgetEveryParentCapableWriterBinds(t *testing.T) {
	parent := dbfx.Issue(t, "Budget parent for every writer")
	root := enableIssueBudget(t, testWorkspaceID, parent)
	syncIssueCounterForBudget(t, testWorkspaceID)
	dbfx.Cleanup(t, `DELETE FROM issue WHERE workspace_id=$1 AND parent_issue_id=$2`, testWorkspaceID, parent)
	created := func(t *testing.T, id string) {
		t.Helper()
		if id == "" {
			t.Fatal("the path returned no issue id")
		}
		dbfx.Cleanup(t, `DELETE FROM issue WHERE id=$1`, id)
		dbfx.Cleanup(t, `DELETE FROM agent_task_queue WHERE issue_id=$1`, id)
		dbfx.Cleanup(t, `DELETE FROM comment WHERE issue_id=$1`, id)
		if got := issueBudgetParentOf(t, id); got != parent {
			t.Fatalf("the created issue's parent is %q, want %s", got, parent)
		}
	}

	t.Run("C1 POST /api/issues", func(t *testing.T) {
		out := testutil.Call(t, testHandler.CreateIssue, newRequest(http.MethodPost, "/api/issues",
			map[string]any{"title": "Budget child created over HTTP", "parent_issue_id": parent})).Want(http.StatusCreated).Map()
		id, _ := out["id"].(string)
		created(t, id)
		requireIssueBudgetBinding(t, "C1 POST /api/issues", id, issueBudgetBinding{Root: root, BoundParent: parent, Source: "create"})
	})

	t.Run("C2 POST /api/comments/{id}/sub-issues", func(t *testing.T) {
		anchor := dbfx.Comment(t, parent, "Split this out of the budget parent")
		var preview sourceContextPreviewResponse
		testutil.Call(t, testHandler.PreviewCommentSubIssue,
			withURLParam(newRequest(http.MethodGet, "/api/comments/"+anchor+"/sub-issue-preview", nil), "commentId", anchor)).Want(http.StatusOK).JSON(&preview)
		dbfx.Cleanup(t, `DELETE FROM issue_source_context WHERE source_issue_id=$1`, parent)
		out := testutil.Call(t, testHandler.CreateCommentSubIssue,
			withURLParam(newRequest(http.MethodPost, "/api/comments/"+anchor+"/sub-issues", map[string]any{
				"mode": "manual", "capture_token": preview.CaptureToken, "issue": map[string]any{"title": "Budget child from a comment"},
			}), "commentId", anchor)).Want(http.StatusCreated).Map()
		id, _ := out["id"].(string)
		created(t, id)
		requireIssueBudgetBinding(t, "C2 comment sub-issue", id, issueBudgetBinding{Root: root, BoundParent: parent, Source: "create"})
	})

	t.Run("C4 lifecycle handoff follow-up", func(t *testing.T) {
		agent := dbfx.Agent(t, "Budget handoff agent", testRuntimeID)
		dbfx.Cleanup(t, `DELETE FROM comment WHERE issue_id=$1`, parent)
		var result lifecycleHandoffResponse
		testutil.Call(t, testHandler.CreateLifecycleHandoff,
			withURLParam(newRequest(http.MethodPost, "/api/issues/"+parent+"/lifecycle-handoffs", map[string]any{
				"kind": "rca", "route": "maintenance", "cause_state": "unknown",
				"follow_up_title": "Budget child from a handoff", "assignee_type": "agent", "assignee_id": agent,
			}), "id", parent)).Want(http.StatusCreated).JSON(&result)
		if !result.FollowUpCreated {
			t.Fatalf("the handoff created no follow-up: %+v", result)
		}
		created(t, result.FollowUpIssueID)
		requireIssueBudgetBinding(t, "C4 lifecycle handoff", result.FollowUpIssueID, issueBudgetBinding{Root: root, BoundParent: parent, Source: "create"})
	})

	t.Run("S3 UpdateIssue", func(t *testing.T) {
		moved := dbfx.Issue(t, "Unbound issue moved under the budget parent")
		below := dbfx.Issue(t, "Child of the moved issue", testutil.Cols{"parent_issue_id": moved})
		updateIssueForBudget(t, moved, map[string]any{"parent_issue_id": parent})
		requireIssueBudgetBinding(t, "S3 UpdateIssue", moved, issueBudgetBinding{Root: root, BoundParent: parent, Source: "reparent"})
		requireIssueBudgetBinding(t, "S3 UpdateIssue, descendant", below, issueBudgetBinding{Root: root, BoundParent: moved, Source: "reparent"})
		// The batch entry reaches the same statement.
		batched := dbfx.Issue(t, "Unbound issue moved by a batch update")
		testutil.Call(t, testHandler.BatchUpdateIssues, newRequest(http.MethodPost, "/api/issues/batch-update", map[string]any{
			"issue_ids": []string{batched}, "updates": map[string]any{"parent_issue_id": parent},
		})).Want(http.StatusOK)
		requireIssueBudgetBinding(t, "S3 UpdateIssue through the batch entry", batched, issueBudgetBinding{Root: root, BoundParent: parent, Source: "reparent"})
	})

	t.Run("S4 DetachDirectChildIssues", func(t *testing.T) {
		middle := dbfx.Issue(t, "Member whose children are detached")
		updateIssueForBudget(t, middle, map[string]any{"parent_issue_id": parent})
		syncIssueCounterForBudget(t, testWorkspaceID)
		out := testutil.Call(t, testHandler.CreateIssue, newRequest(http.MethodPost, "/api/issues",
			map[string]any{"title": "Child detached by its parent's delete", "parent_issue_id": middle})).Want(http.StatusCreated).Map()
		detached, _ := out["id"].(string)
		dbfx.Cleanup(t, `DELETE FROM issue WHERE id=$1`, detached)
		requireIssueBudgetBinding(t, "before the delete", detached, issueBudgetBinding{Root: root, BoundParent: middle, Source: "create"})

		testutil.Call(t, testHandler.DeleteIssue,
			withURLParam(newRequest(http.MethodDelete, "/api/issues/"+middle, nil), "id", middle)).Want(http.StatusNoContent)
		if got := issueBudgetParentOf(t, detached); got != "" {
			t.Fatalf("the child still has parent %q after its parent was deleted", got)
		}
		requireIssueBudgetBinding(t, "S4 DetachDirectChildIssues", detached, issueBudgetBinding{Root: root, BoundParent: middle, Source: "create"})
		requireIssueBudgetBinding(t, "S4, the deleted parent", middle, issueBudgetBinding{Root: root, BoundParent: parent, Source: "reparent", Tombstoned: true})
	})

	t.Run("S6 PrepareWorkspaceDeletionLinks", func(t *testing.T) {
		t.Setenv("MULTICA_AUTH_MODE", "legacy")
		workspace := dbfx.Workspace(t, "Budget writer teardown", "budget-writers-"+uuid.NewString())
		dbfx.Member(t, workspace, testUserID, "owner")
		top := dbfx.Issue(t, "Teardown budget parent", testutil.Cols{"workspace_id": workspace})
		child := dbfx.Issue(t, "Teardown budget member", testutil.Cols{"workspace_id": workspace, "parent_issue_id": top})
		teardownRoot := enableIssueBudget(t, workspace, top)
		before := issueBudgetRootRows(t, teardownRoot)

		r := newRequest(http.MethodDelete, "/api/workspaces/"+workspace, nil)
		r.Header.Set("X-Workspace-ID", workspace)
		testutil.Call(t, testHandler.DeleteWorkspace, withURLParam(r, "id", workspace)).Want(http.StatusNoContent)
		if got := dbfx.Count(t, `SELECT count(*) FROM issue WHERE id = ANY($1::uuid[])`, []string{top, child}); got != 0 {
			t.Fatalf("%d issues left after the workspace was deleted", got)
		}
		// Teardown clears every parent pointer and then deletes the issues.
		// It is not an issue write path of the budget: no row is marked.
		requireIssueBudgetRootRows(t, teardownRoot, before, "S6 PrepareWorkspaceDeletionLinks")
		requireIssueBudgetBinding(t, "S6 PrepareWorkspaceDeletionLinks", child, issueBudgetBinding{Root: teardownRoot, BoundParent: top, Source: "activation"})
	})
}

// The cost promised to trees without a root: creating, moving and deleting
// issues there leaves the budget tables exactly as they were.
func TestIssueBudgetWritesNothingForATreeWithoutARoot(t *testing.T) {
	parent := dbfx.Issue(t, "Parent outside every root")
	other := dbfx.Issue(t, "Another parent outside every root")
	syncIssueCounterForBudget(t, testWorkspaceID)
	dbfx.Cleanup(t, `DELETE FROM issue WHERE workspace_id=$1 AND parent_issue_id=ANY($2::uuid[])`, testWorkspaceID, []string{parent, other})
	count := func() int {
		total := 0
		for _, table := range issueBudgetTables {
			total += dbfx.Count(t, `SELECT count(*) FROM `+table+` WHERE workspace_id=$1`, testWorkspaceID)
		}
		return total
	}
	before := count()

	out := testutil.Call(t, testHandler.CreateIssue, newRequest(http.MethodPost, "/api/issues",
		map[string]any{"title": "Child outside every root", "parent_issue_id": parent})).Want(http.StatusCreated).Map()
	child, _ := out["id"].(string)
	dbfx.Cleanup(t, `DELETE FROM issue WHERE id=$1`, child)
	updateIssueForBudget(t, child, map[string]any{"parent_issue_id": other})
	updateIssueForBudget(t, child, map[string]any{"parent_issue_id": nil})
	updateIssueForBudget(t, child, map[string]any{"parent_issue_id": parent})
	testutil.Call(t, testHandler.DeleteIssue,
		withURLParam(newRequest(http.MethodDelete, "/api/issues/"+child, nil), "id", child)).Want(http.StatusNoContent)
	testutil.Call(t, testHandler.DeleteIssue,
		withURLParam(newRequest(http.MethodDelete, "/api/issues/"+parent, nil), "id", parent)).Want(http.StatusNoContent)

	if after := count(); after != before {
		t.Fatalf("issue writes outside every root changed the budget tables: %d rows -> %d", before, after)
	}
}
