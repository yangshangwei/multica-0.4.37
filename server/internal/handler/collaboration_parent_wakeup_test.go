package handler

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func collaborationParentWakeupFixture(t *testing.T, assigneeType, parentStatus string) (shared, parent, agent string) {
	t.Helper()
	shared = collaborationEditFixture(t)
	if err := testHandler.Queries.SeedIssueStatusEntries(t.Context(), parseUUID(testWorkspaceID)); err != nil {
		t.Fatal(err)
	}
	runtime := dbfx.Runtime(t, "Parent wake runtime")
	agent = dbfx.Agent(t, "Parent wake agent", runtime)
	assignee := agent
	if assigneeType == "squad" {
		assignee = dbfx.Squad(t, "Parent wake squad", agent)
	} else if assigneeType == "member" {
		assignee = testUserID
	}
	parent = dbfx.Issue(t, "Ordinary parent", testutil.Cols{"status": parentStatus, "assignee_type": assigneeType, "assignee_id": assignee})
	dbfx.Exec(t, `UPDATE issue SET parent_issue_id=$2, title='Shared child reference', description='PRIVATE-SHARED-CHILD-BODY' WHERE id=$1`, shared, parent)
	dbfx.Cleanup(t, `DELETE FROM comment WHERE issue_id=$1`, parent)
	dbfx.Cleanup(t, `DELETE FROM agent_task_queue WHERE issue_id=$1`, parent)
	return
}

type parentWakeupCommitStarter struct {
	txStarter
	beforeCommit func()
	failure      string
}

type parentWakeupCommitTx struct {
	pgx.Tx
	starter parentWakeupCommitStarter
}

func (s parentWakeupCommitStarter) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := s.txStarter.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return parentWakeupCommitTx{Tx: tx, starter: s}, nil
}

func (tx parentWakeupCommitTx) Commit(ctx context.Context) error {
	tx.starter.beforeCommit()
	if tx.starter.failure == "rollback" {
		return errors.New("injected lifecycle commit failure")
	}
	err := tx.Tx.Commit(ctx)
	if err == nil && tx.starter.failure == "unknown_commit" {
		return errors.New("injected lost lifecycle commit response")
	}
	return err
}

func TestCollaborationChildWakeupRunsOnlyAfterConfirmedCommit(t *testing.T) {
	for _, failure := range []string{"", "rollback", "unknown_commit"} {
		t.Run("commit_"+failure, func(t *testing.T) {
			shared, parent, agent := collaborationParentWakeupFixture(t, "agent", "in_progress")
			body := collaborationParentLifecycleBody(t, shared, "complete", "done")
			h := *testHandler
			commits := 0
			h.TxStarter = parentWakeupCommitStarter{txStarter: testPool, failure: failure, beforeCommit: func() {
				commits++
				// These reads use a second connection while the close is pending.
				if dbfx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND status='todo'`, shared) != 1 ||
					countSystemCommentsOn(t, parent) != 0 || countPendingTasksForAgent(t, parent, agent) != 0 {
					t.Error("uncommitted child completion emitted parent effects")
				}
			}}
			key := uuid.NewString()
			wantStatus, wantEffects, childStatus := http.StatusOK, 1, "done"
			if failure != "" {
				wantStatus, wantEffects = http.StatusServiceUnavailable, 0
			}
			if failure == "rollback" {
				childStatus = "todo"
			}
			testutil.Call(t, h.ChangeCollaborationLifecycle, collaborationEditRequest(shared, testUserID, key, body)).Want(wantStatus)
			if commits != 1 || countSystemCommentsOn(t, parent) != wantEffects || countPendingTasksForAgent(t, parent, agent) != wantEffects ||
				dbfx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND status=$2`, shared, childStatus) != 1 {
				t.Fatal("lifecycle commit outcome and parent effects diverged")
			}
			if failure == "unknown_commit" {
				// Recover the committed result without rerunning its best-effort hook.
				testutil.Call(t, testHandler.ChangeCollaborationLifecycle, collaborationEditRequest(shared, testUserID, key, body)).Want(http.StatusOK)
				if countSystemCommentsOn(t, parent) != 0 || countPendingTasksForAgent(t, parent, agent) != 0 {
					t.Fatal("unknown-commit replay recreated parent effects")
				}
			}
		})
	}
}

func collaborationParentLifecycleBody(t *testing.T, shared, action, status string) map[string]any {
	t.Helper()
	r := collaborationEditRequest(shared, testUserID, "", nil)
	svc := service.NewCollaborationService(testHandler.Queries, testHandler.TxStarter, testHandler.IssueService)
	options, err := svc.StatusOptions(r.Context(), parseUUID(shared), action)
	if err != nil {
		t.Fatal(err)
	}
	body := collaborationEditBodyValues()
	body["action"], body["status"], body["status_options_version"] = action, status, options.StatusOptionsVersion
	return body
}

func TestCollaborationChildCompletionWakesOrdinaryParentOnce(t *testing.T) {
	for _, kind := range []string{"agent", "squad"} {
		t.Run(kind, func(t *testing.T) {
			shared, parent, agent := collaborationParentWakeupFixture(t, kind, "in_progress")
			body := collaborationParentLifecycleBody(t, shared, "complete", "done")
			key := uuid.NewString()
			call := func() {
				t.Helper()
				out := testutil.Call(t, testHandler.ChangeCollaborationLifecycle, collaborationEditRequest(shared, testUserID, key, body)).Want(http.StatusOK)
				if len(out.Map()) != 2 || strings.Contains(out.Body.String(), parent) || strings.Contains(out.Body.String(), "Shared child reference") {
					t.Fatal("lifecycle receipt disclosed the internal parent transition")
				}
			}
			call()
			if countSystemCommentsOn(t, parent) != 1 || countPendingTasksForAgent(t, parent, agent) != 1 {
				t.Fatal("committed collaboration child did not wake its ordinary parent exactly once")
			}
			comment := parentSystemCommentContent(t, parent)
			if !strings.Contains(comment, "Shared child reference") || strings.Contains(comment, "PRIVATE-SHARED-CHILD-BODY") || strings.Contains(comment, "mention://issue/"+shared) {
				t.Fatalf("parent comment did not preserve the unopenable minimal child reference: %s", comment)
			}
			if dbfx.Count(t, `SELECT count(*) FROM agent_task_queue q JOIN comment c ON c.id=q.trigger_comment_id WHERE q.issue_id=$1 AND q.agent_id=$2 AND c.issue_id=$1 AND q.is_leader_task=$3`, parent, agent, kind == "squad") != 1 {
				t.Fatal("parent wake lost its comment or squad leader provenance")
			}
			var taskJSON string
			dbfx.QueryRow(t, `SELECT to_jsonb(q)::text FROM agent_task_queue q WHERE issue_id=$1`, parent).Scan(&taskJSON)
			if strings.Contains(taskJSON, "PRIVATE-SHARED-CHILD-BODY") || strings.Contains(taskJSON, "mention://issue/"+shared) {
				t.Fatal("parent task context expanded the shared-child reference")
			}
			// A settled task must not make an old lifecycle receipt runnable again.
			dbfx.Exec(t, `UPDATE agent_task_queue SET status='completed' WHERE issue_id=$1`, parent)
			call()
			if countSystemCommentsOn(t, parent) != 1 || dbfx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id=$1`, parent) != 1 {
				t.Fatal("lifecycle replay duplicated parent comment or execution")
			}
		})
	}
}

func TestCollaborationChildCompletionPreservesParentGuards(t *testing.T) {
	for _, tc := range []struct {
		name, parentStatus, assignee, siblingStatus string
		stage, siblingStage                         int
		sharedParent, wantWake                      bool
	}{
		{name: "same_stage_open", parentStatus: "in_progress", assignee: "agent", stage: 1, siblingStage: 1, siblingStatus: "todo"},
		{name: "earlier_stage_open", parentStatus: "in_progress", assignee: "agent", stage: 2, siblingStage: 1, siblingStatus: "todo"},
		{name: "later_stage_parked", parentStatus: "in_progress", assignee: "agent", stage: 1, siblingStage: 2, siblingStatus: "backlog", wantWake: true},
		{name: "unstaged_sibling_open", parentStatus: "in_progress", assignee: "agent", siblingStatus: "todo"},
		{name: "unstaged_in_staged_set", parentStatus: "in_progress", assignee: "agent", siblingStage: 1, siblingStatus: "done"},
		{name: "cancelled_sibling", parentStatus: "in_progress", assignee: "agent", stage: 1, siblingStage: 1, siblingStatus: "cancelled", wantWake: true},
		{name: "backlog_parent", parentStatus: "backlog", assignee: "agent"},
		{name: "terminal_parent", parentStatus: "done", assignee: "agent"},
		{name: "human_parent", parentStatus: "in_progress", assignee: "member"},
		{name: "shared_parent", parentStatus: "in_progress", assignee: "member", sharedParent: true},
		// Even a malformed historical assignee cannot turn a shared parent into
		// an ordinary execution or expose an ordinary comment on its timeline.
		{name: "shared_parent_legacy_agent", parentStatus: "in_progress", assignee: "agent", sharedParent: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			shared, parent, agent := collaborationParentWakeupFixture(t, tc.assignee, tc.parentStatus)
			if tc.sharedParent {
				dbfx.InsertNoID(t, "issue_collaboration", testutil.Cols{"issue_id": parent}, "issue_id=$1", parent)
			}
			if tc.stage != 0 {
				dbfx.Exec(t, `UPDATE issue SET stage=$2 WHERE id=$1`, shared, tc.stage)
			}
			if tc.siblingStatus != "" {
				cols := testutil.Cols{"parent_issue_id": parent, "status": tc.siblingStatus}
				if tc.siblingStage != 0 {
					cols["stage"] = tc.siblingStage
				}
				dbfx.Issue(t, "Sibling", cols)
			}
			body := collaborationParentLifecycleBody(t, shared, "complete", "done")
			testutil.Call(t, testHandler.ChangeCollaborationLifecycle, collaborationEditRequest(shared, testUserID, uuid.NewString(), body)).Want(http.StatusOK)
			want := 0
			if tc.wantWake {
				want = 1
			}
			if got := countSystemCommentsOn(t, parent); got != want {
				t.Fatalf("parent comments = %d, want %d", got, want)
			}
			if got := countPendingTasksForAgent(t, parent, agent); got != want {
				t.Fatalf("parent tasks = %d, want %d", got, want)
			}
		})
	}
}

func TestCollaborationChildTerminalCategoriesUseOrdinaryStageWakeup(t *testing.T) {
	for _, tc := range []struct {
		name, action, category, parentCategory string
		wantWake                               bool
	}{
		{"custom_done", "complete", "done", "in_progress", true},
		{"cancel", "cancel", "cancelled", "in_progress", true},
		{"custom_terminal_parent", "complete", "done", "done", false},
		{"custom_backlog_parent", "cancel", "cancelled", "backlog", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			shared, parent, agent := collaborationParentWakeupFixture(t, "agent", "in_progress")
			childStatus := "child_" + uuid.NewString()[:8]
			parentStatus := "parent_" + uuid.NewString()[:8]
			siblingStatus := "sibling_" + uuid.NewString()[:8]
			for key, category := range map[string]string{childStatus: tc.category, parentStatus: tc.parentCategory, siblingStatus: "cancelled"} {
				dbfx.Insert(t, "issue_status", testutil.Cols{"workspace_id": testWorkspaceID, "key": key, "name": key, "category": category, "color": "#000000"})
			}
			dbfx.Exec(t, `UPDATE issue SET status=$2 WHERE id=$1`, parent, parentStatus)
			dbfx.Exec(t, `UPDATE issue SET stage=1 WHERE id=$1`, shared)
			dbfx.Issue(t, "Custom terminal sibling", testutil.Cols{"parent_issue_id": parent, "stage": 1, "status": siblingStatus})
			body := collaborationParentLifecycleBody(t, shared, tc.action, childStatus)
			testutil.Call(t, testHandler.ChangeCollaborationLifecycle, collaborationEditRequest(shared, testUserID, uuid.NewString(), body)).Want(http.StatusOK)
			want := 0
			if tc.wantWake {
				want = 1
			}
			if countSystemCommentsOn(t, parent) != want || countPendingTasksForAgent(t, parent, agent) != want {
				t.Fatal("custom terminal category changed ordinary stage/parent semantics")
			}
		})
	}
}

func TestCollaborationChildWakeupRechecksParentBeforePublishing(t *testing.T) {
	for _, change := range []string{"terminal", "human", "shared"} {
		t.Run(change, func(t *testing.T) {
			shared, parentID, agent := collaborationParentWakeupFixture(t, "agent", "in_progress")
			parent, err := testHandler.Queries.GetIssue(t.Context(), parseUUID(parentID))
			if err != nil {
				t.Fatal(err)
			}
			dbfx.Exec(t, `UPDATE issue SET status='done' WHERE id=$1`, shared)
			completed, err := testHandler.Queries.GetIssue(t.Context(), parseUUID(shared))
			if err != nil {
				t.Fatal(err)
			}
			switch change {
			case "terminal":
				dbfx.Exec(t, `UPDATE issue SET status='done' WHERE id=$1`, parentID)
			case "human":
				dbfx.Exec(t, `UPDATE issue SET assignee_type='member', assignee_id=$2 WHERE id=$1`, parentID, testUserID)
			case "shared":
				dbfx.InsertNoID(t, "issue_collaboration", testutil.Cols{"issue_id": parentID}, "issue_id=$1", parentID)
			}
			testHandler.postChildDoneComment(t.Context(), parent, completed, []db.Issue{completed}, false, 0, false)
			if countSystemCommentsOn(t, parentID) != 0 || countPendingTasksForAgent(t, parentID, agent) != 0 {
				t.Fatal("stale parent snapshot authorized a child-done comment or task")
			}
		})
	}
}
