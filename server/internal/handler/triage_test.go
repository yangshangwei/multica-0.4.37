package handler

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/pkg/dbid"
)

func triageEnableForTest(t *testing.T) {
	t.Helper()
	dbfx.Exec(t, `INSERT INTO workspace_triage_settings(workspace_id,enabled) VALUES($1,true) ON CONFLICT(workspace_id) DO UPDATE SET enabled=true, require_priority=false, responsibility_mode='none', responsibility_member_id=NULL`, testWorkspaceID)
	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM inbox_item WHERE workspace_id=$1 AND type='triage'`, testWorkspaceID)
		testPool.Exec(context.Background(), `DELETE FROM triage_notification WHERE workspace_id=$1`, testWorkspaceID)
		testPool.Exec(context.Background(), `DELETE FROM triage_action WHERE workspace_id=$1`, testWorkspaceID)
		testPool.Exec(context.Background(), `DELETE FROM triage_intake_request WHERE workspace_id=$1`, testWorkspaceID)
		testPool.Exec(context.Background(), `DELETE FROM issue_triage WHERE workspace_id=$1`, testWorkspaceID)
		testPool.Exec(context.Background(), `DELETE FROM workspace_triage_settings WHERE workspace_id=$1`, testWorkspaceID)
	})
}
func triageInputForTest() map[string]any {
	return map[string]any{"request_id": uuidToString(dbid.NewV7()), "title": "Incoming triage feedback"}
}
func triageCreateForTest(t *testing.T) TriageItem {
	t.Helper()
	var out TriageItem
	testutil.Call(t, testHandler.CreateTriageItem, newRequest("POST", "/api/triage/items", triageInputForTest())).Want(201).JSON(&out)
	return out
}
func triageActionForTest(t *testing.T, item TriageItem, action string, extra map[string]any, status int) TriageActionResult {
	t.Helper()
	in := map[string]any{"request_id": uuidToString(dbid.NewV7()), "expected_revision": item.Issue.Revision, "action": action}
	for k, v := range extra {
		in[k] = v
	}
	var out TriageActionResult
	c := testutil.Call(t, testHandler.ActOnTriageItem, withURLParam(newRequest("POST", "/api/triage/items/"+item.Issue.ID+"/actions", in), "id", item.Issue.ID)).Want(status)
	if status == 200 {
		c.JSON(&out)
	}
	return out
}
func TestTriageDefaultAndHumanGate(t *testing.T) {
	var settings TriageSettings
	testutil.Call(t, testHandler.GetTriageSettings, newRequest("GET", "/api/triage/settings", nil)).Want(200).JSON(&settings)
	if settings.Enabled || !settings.Supported {
		t.Fatalf("new workspace must be supported and off: %+v", settings)
	}
	testutil.Call(t, testHandler.CreateTriageItem, newRequest("POST", "/api/triage/items", triageInputForTest())).Want(409)
	triageEnableForTest(t)
	item := triageCreateForTest(t)
	r := withURLParam(newRequest("POST", "/api/triage/items/actions", map[string]any{"request_id": uuidToString(dbid.NewV7()), "expected_revision": item.Issue.Revision, "action": "accept"}), "id", item.Issue.ID)
	r.Header.Set("X-Actor-Source", "task_token")
	testutil.Call(t, testHandler.ActOnTriageItem, r).Want(http.StatusForbidden)
}
func TestTriageAtomicAdmissionAndHistory(t *testing.T) {
	triageEnableForTest(t)
	item := triageCreateForTest(t)
	if item.Issue.AdmissionStatus != "pending" || item.Issue.Status != "backlog" || item.Issue.AssigneeID != nil || item.Issue.ProjectID != nil {
		t.Fatalf("intake must be inert: %+v", item)
	}
	triageActionForTest(t, item, "reject", map[string]any{"reason": "   "}, 400)
	rejected := triageActionForTest(t, item, "reject", map[string]any{"reason": "No longer relevant"}, 200)
	if rejected.Item.Issue.AdmissionStatus != "rejected" || rejected.Item.Issue.Status != "cancelled" {
		t.Fatalf("bad rejection %+v", rejected)
	}
	reopened := triageActionForTest(t, rejected.Item, "reopen", map[string]any{"reason": "New information"}, 200)
	if reopened.Item.Round != 2 || reopened.Item.FirstEnteredAt != item.FirstEnteredAt {
		t.Fatal("reopen must retain first arrival and create a round")
	}
	accepted := triageActionForTest(t, reopened.Item, "accept", nil, 200)
	if accepted.Item.Issue.AdmissionStatus != "accepted" || accepted.Action.ExecutionStatus != "not_requested" {
		t.Fatal("ordinary acceptance must not execute")
	}
	var count int
	dbfx.QueryRow(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id=$1`, item.Issue.ID).Scan(&count)
	if count != 0 {
		t.Fatal("ordinary acceptance enqueued work")
	}
	triageActionForTest(t, accepted.Item, "reopen", map[string]any{"reason": "wrong"}, 409)
	var history struct {
		Events []TriageAction `json:"events"`
	}
	testutil.Call(t, testHandler.GetTriageItemHistory, withURLParam(newRequest("GET", "/api/triage/items/history", nil), "id", item.Issue.ID)).Want(200).JSON(&history)
	if len(history.Events) != 3 || history.Events[0].Reason == nil || *history.Events[0].Reason != "No longer relevant" {
		t.Fatalf("history lost prior round: %+v", history)
	}
}
func TestTriageIdempotencyCASAndSnoozeDisable(t *testing.T) {
	triageEnableForTest(t)
	input := triageInputForTest()
	var item, again TriageItem
	testutil.Call(t, testHandler.CreateTriageItem, newRequest("POST", "/api/triage/items", input)).Want(201).JSON(&item)
	testutil.Call(t, testHandler.CreateTriageItem, newRequest("POST", "/api/triage/items", input)).Want(201).JSON(&again)
	if item.Issue.ID != again.Issue.ID {
		t.Fatal("intake retry duplicated issue")
	}
	snoozed := triageActionForTest(t, item, "snooze", map[string]any{"snoozed_until": time.Now().Add(time.Hour).Format(time.RFC3339)}, 200)
	triageActionForTest(t, item, "accept", nil, 409)
	var settings TriageSettings
	testutil.Call(t, testHandler.GetTriageSettings, newRequest("GET", "/api/triage/settings", nil)).Want(200).JSON(&settings)
	testutil.Call(t, testHandler.UpdateTriageSettings, newRequest("PUT", "/api/triage/settings", map[string]any{"enabled": false, "acceptance_status": "todo", "responsibility_mode": "none", "expected_revision": settings.Revision})).Want(409)
	accepted := triageActionForTest(t, snoozed.Item, "accept", nil, 200)
	if accepted.Item.SnoozedUntil != nil {
		t.Fatal("acceptance retained snooze")
	}
}

func TestTriageAcceptExecuteRetryAfterTerminalTask(t *testing.T) {
	triageEnableForTest(t)
	agent := dbfx.Agent(t, "Triage explicit executor", testRuntimeID)
	input := triageInputForTest()
	input["candidate_assignee_type"] = "agent"
	input["candidate_assignee_id"] = agent
	item := testutil.Decode[TriageItem](t, testHandler.CreateTriageItem, newRequest("POST", "/api/triage/items", input), 201)
	result := triageActionForTest(t, item, "accept_and_execute", nil, 200)
	if result.Action.ExecutionStatus != "queued" || result.Action.TaskID == nil {
		t.Fatalf("explicit execution not queued: %+v", result.Action)
	}
	dbfx.Exec(t, `UPDATE agent_task_queue SET status='completed' WHERE id=$1`, *result.Action.TaskID)
	retry := testutil.Decode[TriageActionResult](t, testHandler.RetryTriageExecution, withURLParam(newRequest("POST", "/api/triage/actions/retry", map[string]any{}), "actionId", result.Action.ID), 200)
	if retry.Action.TaskID == nil || *retry.Action.TaskID != *result.Action.TaskID {
		t.Fatal("terminal execution retry changed durable task identity")
	}
	var n int
	dbfx.QueryRow(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id=$1`, item.Issue.ID).Scan(&n)
	if n != 1 {
		t.Fatalf("retry created %d tasks", n)
	}
}
func TestTriageExecutionFailureKeepsAcceptanceAndContext(t *testing.T) {
	triageEnableForTest(t)
	agent := dbfx.Agent(t, "Triage unavailable executor", testRuntimeID)
	input := triageInputForTest()
	input["candidate_assignee_type"] = "agent"
	input["candidate_assignee_id"] = agent
	item := testutil.Decode[TriageItem](t, testHandler.CreateTriageItem, newRequest("POST", "/api/triage/items", input), 201)
	original := testHandler.TxStarter
	testHandler.TxStarter = triageAfterCommitStarter{base: original, after: func() {
		dbfx.Exec(t, `UPDATE agent_runtime SET status='offline',metadata='{"offline_reason":{"code":"not_executable"}}' WHERE id=$1`, testRuntimeID)
	}}
	t.Cleanup(func() { testHandler.TxStarter = original })
	t.Cleanup(func() {
		testPool.Exec(context.Background(), `UPDATE agent_runtime SET status='online',metadata='{}' WHERE id=$1`, testRuntimeID)
	})
	result := triageActionForTest(t, item, "accept_and_execute", nil, 200)
	if result.Item.Issue.AdmissionStatus != "accepted" || result.Action.ExecutionStatus != "failed" {
		t.Fatalf("failed dispatch must preserve admission: %+v", result)
	}
	testHandler.TxStarter = original
	dbfx.Exec(t, `UPDATE agent_runtime SET status='online',metadata='{}' WHERE id=$1`, testRuntimeID)
	dbfx.Exec(t, `UPDATE issue SET title='Changed execution context' WHERE id=$1`, item.Issue.ID)
	testutil.Call(t, testHandler.RetryTriageExecution, withURLParam(newRequest("POST", "/api/triage/actions/retry", map[string]any{}), "actionId", result.Action.ID)).Want(409)
	dbfx.Exec(t, `UPDATE issue SET title=$2 WHERE id=$1`, item.Issue.ID, item.Issue.Title)
	originalAgent, err := testHandler.Queries.GetAgent(context.Background(), parseUUID(agent))
	if err != nil {
		t.Fatal(err)
	}
	for _, mutation := range []string{"mcp", "instructions", "cancelled"} {
		t.Run(mutation, func(t *testing.T) {
			switch mutation {
			case "mcp":
				dbfx.Exec(t, `UPDATE agent SET mcp_config='{"changed":true}' WHERE id=$1`, agent)
			case "instructions":
				dbfx.Exec(t, `UPDATE agent SET instructions='Different run instructions' WHERE id=$1`, agent)
			case "cancelled":
				dbfx.Exec(t, `UPDATE issue SET status='cancelled' WHERE id=$1`, item.Issue.ID)
			}
			testutil.Call(t, testHandler.RetryTriageExecution, withURLParam(newRequest("POST", "/api/triage/actions/retry", map[string]any{}), "actionId", result.Action.ID)).Want(409)
			dbfx.Exec(t, `UPDATE agent SET mcp_config=$2,instructions=$3 WHERE id=$1`, agent, originalAgent.McpConfig, originalAgent.Instructions)
			dbfx.Exec(t, `UPDATE issue SET status='todo' WHERE id=$1`, item.Issue.ID)
		})
	}
	var n int
	dbfx.QueryRow(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id=$1`, item.Issue.ID).Scan(&n)
	if n != 0 {
		t.Fatal("changed context retry enqueued work")
	}

}
func TestTriageActionReplayAndDeletedTombstones(t *testing.T) {
	triageEnableForTest(t)
	input := triageInputForTest()
	item := testutil.Decode[TriageItem](t, testHandler.CreateTriageItem, newRequest("POST", "/api/triage/items", input), 201)
	action := map[string]any{"request_id": uuidToString(dbid.NewV7()), "expected_revision": item.Issue.Revision, "action": "reject", "reason": "Preserve this decision"}
	call := func() *http.Request {
		return withURLParam(newRequest("POST", "/api/triage/items/actions", action), "id", item.Issue.ID)
	}
	first := testutil.Decode[TriageActionResult](t, testHandler.ActOnTriageItem, call(), 200)
	second := testutil.Decode[TriageActionResult](t, testHandler.ActOnTriageItem, call(), 200)
	if first.Action.ID != second.Action.ID {
		t.Fatal("action replay changed identity")
	}
	action["reason"] = "Different decision"
	testutil.Call(t, testHandler.ActOnTriageItem, call()).Want(409)
	action["reason"] = "Preserve this decision"
	testutil.Call(t, testHandler.DeleteIssue, withURLParam(newRequest("DELETE", "/api/issues/"+item.Issue.ID, nil), "id", item.Issue.ID)).Want(204)
	testutil.Call(t, testHandler.ActOnTriageItem, call()).Want(409)
	testutil.Call(t, testHandler.ActOnTriageItem, withURLParam(newRequest("POST", "/api/triage/items/actions", action), "id", item.Issue.Identifier)).Want(409)
	testutil.Call(t, testHandler.CreateTriageItem, newRequest("POST", "/api/triage/items", input)).Want(409)
	var history struct {
		Entries []TriageHistoryEntry `json:"entries"`
	}
	testutil.Call(t, testHandler.GetTriageHistory, newRequest("GET", "/api/triage/history?result=rejected", nil)).Want(200).JSON(&history)
	if len(history.Entries) != 1 || history.Entries[0].Title != item.Issue.Title {
		t.Fatalf("deleted result history lost snapshot: %+v", history)
	}
}
func TestTriageConcurrentReviewOnlyOneDecision(t *testing.T) {
	triageEnableForTest(t)
	item := triageCreateForTest(t)
	start := make(chan struct{})
	done := make(chan error, 2)
	for _, action := range []string{"accept", "reject"} {
		go func(action string) {
			<-start
			_, err := testHandler.actOnTriageItem(newRequest("POST", "/api/triage/items/actions", nil), item.Issue.ID, TriageActionInput{RequestID: uuidToString(dbid.NewV7()), ExpectedRevision: item.Issue.Revision, Action: action, Reason: "Race decision"}, false)
			done <- err
		}(action)
	}
	close(start)
	wins := 0
	for range 2 {
		if <-done == nil {
			wins++
		}
	}
	if wins != 1 {
		t.Fatalf("expected1 committed decision, got%d", wins)
	}
	var n int
	dbfx.QueryRow(t, `SELECT count(*) FROM triage_action WHERE issue_id=$1`, item.Issue.ID).Scan(&n)
	if n != 1 {
		t.Fatalf("race recorded%d decisions", n)
	}
}
func TestTriageBatchAllowlistAndPreviewRollback(t *testing.T) {
	triageEnableForTest(t)
	item := triageCreateForTest(t)
	for _, action := range []string{"accept_and_execute", "duplicate", "reopen", "unsnooze"} {
		testutil.Call(t, testHandler.CommitTriageBatch, newRequest("POST", "/api/triage/batch", map[string]any{"items": []any{map[string]any{"issue_id": item.Issue.ID, "request_id": uuidToString(dbid.NewV7()), "expected_revision": item.Issue.Revision, "action": action}}})).Want(400)
	}
	testutil.Call(t, testHandler.PreviewTriageBatch, newRequest("POST", "/api/triage/batch/preview", map[string]any{"items": []any{map[string]any{"issue_id": item.Issue.ID, "expected_revision": item.Issue.Revision}}, "action": "accept"})).Want(200)
	var state string
	dbfx.QueryRow(t, `SELECT admission_status FROM issue WHERE id=$1`, item.Issue.ID).Scan(&state)
	if state != "pending" {
		t.Fatal("batch preview committed acceptance")
	}
}
func TestTriagePriorityAndCandidateRollback(t *testing.T) {
	triageEnableForTest(t)
	project := dbfx.Project(t, "Triage candidate project")
	input := triageInputForTest()
	input["candidate_project_id"] = project
	item := testutil.Decode[TriageItem](t, testHandler.CreateTriageItem, newRequest("POST", "/api/triage/items", input), 201)
	dbfx.Exec(t, `UPDATE workspace_triage_settings SET require_priority=true WHERE workspace_id=$1`, testWorkspaceID)
	triageActionForTest(t, item, "accept", nil, 400)
	dbfx.Exec(t, `DELETE FROM project WHERE id=$1`, project)
	triageActionForTest(t, item, "accept", map[string]any{"fields": map[string]any{"priority": "high"}}, 400)
	var state, priority string
	dbfx.QueryRow(t, `SELECT admission_status,priority FROM issue WHERE id=$1`, item.Issue.ID).Scan(&state, &priority)
	if state != "pending" || priority != "none" {
		t.Fatal("invalid acceptance partially committed")
	}
	accepted := triageActionForTest(t, item, "accept", map[string]any{"fields": map[string]any{"priority": "high", "project_id": nil}}, 200)
	if accepted.Item.Issue.Priority != "high" || accepted.Item.Issue.ProjectID != nil {
		t.Fatal("explicit candidate clearing failed")
	}
}
func TestTriageDueQueueAndNotificationIdempotency(t *testing.T) {
	triageEnableForTest(t)
	dbfx.Exec(t, `UPDATE workspace_triage_settings SET responsibility_mode='assign',responsibility_member_id=$2 WHERE workspace_id=$1`, testWorkspaceID, testUserID)
	item := triageCreateForTest(t)
	result := triageActionForTest(t, item, "snooze", map[string]any{"snoozed_until": time.Now().Add(time.Hour).Format(time.RFC3339)}, 200)
	past := time.Now().Add(-time.Minute)
	dbfx.Exec(t, `UPDATE issue_triage SET snoozed_until=$2 WHERE issue_id=$1`, item.Issue.ID, past)
	dbfx.Exec(t, `UPDATE triage_notification SET due_at=$2,details=jsonb_set(details,'{snoozed_until}',to_jsonb($3::text)) WHERE event_key=$1`, "snooze:"+result.Action.ID, past, past.Format(time.RFC3339Nano))
	var queue struct {
		Items  []TriageItem `json:"items"`
		Counts struct {
			Pending int `json:"pending"`
			Ready   int `json:"ready"`
			Snoozed int `json:"snoozed"`
		} `json:"counts"`
	}
	testutil.Call(t, testHandler.ListTriageItems, newRequest("GET", "/api/triage/items", nil)).Want(200).JSON(&queue)
	if len(queue.Items) != 1 || queue.Items[0].EnteredAt != item.EnteredAt || queue.Counts.Ready != 1 {
		t.Fatalf("expired snooze did not recover %+v", queue)
	}
	testHandler.deliverTriageNotifications(context.Background(), parseUUID(testWorkspaceID))
	var n int
	dbfx.QueryRow(t, `SELECT count(*) FROM inbox_item WHERE issue_id=$1 AND type='triage'`, item.Issue.ID).Scan(&n)
	if n != 1 {
		t.Fatalf("due reminder must deliver once, got%d", n)
	}
}

// Inject the dispatch failure after admission commit, not before preflight.
type triageAfterCommitStarter struct {
	base  txStarter
	after func()
}

func (s triageAfterCommitStarter) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := s.base.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &triageAfterCommitTx{Tx: tx, after: s.after}, nil
}

type triageAfterCommitTx struct {
	pgx.Tx
	after func()
	once  sync.Once
}

func (t *triageAfterCommitTx) Commit(ctx context.Context) error {
	err := t.Tx.Commit(ctx)
	if err == nil {
		t.once.Do(t.after)
	}
	return err
}

func TestTriageBatchDuplicateSelectionHasNoHiddenMutation(t *testing.T) {
	triageEnableForTest(t)
	item := triageCreateForTest(t)
	testutil.Call(t, testHandler.CommitTriageBatch, newRequest("POST", "/api/triage/batch", map[string]any{"items": []any{
		map[string]any{"issue_id": item.Issue.ID, "request_id": uuidToString(dbid.NewV7()), "expected_revision": item.Issue.Revision, "action": "snooze", "snoozed_until": time.Now().Add(time.Hour).Format(time.RFC3339)},
		map[string]any{"issue_id": item.Issue.ID, "request_id": uuidToString(dbid.NewV7()), "expected_revision": item.Issue.Revision + 1, "action": "assign_reviewer", "reviewer_id": testUserID},
	}})).Want(400)
	var revision int64
	var actions int
	dbfx.QueryRow(t, `SELECT revision FROM issue WHERE id=$1`, item.Issue.ID).Scan(&revision)
	dbfx.QueryRow(t, `SELECT count(*) FROM triage_action WHERE issue_id=$1`, item.Issue.ID).Scan(&actions)
	if revision != item.Issue.Revision || actions != 0 {
		t.Fatalf("invalid duplicate batch wrote revision=%d actions=%d", revision, actions)
	}
}

func TestTriageSettingsRoleAndResolvedAgentGates(t *testing.T) {
	triageEnableForTest(t)
	user := dbfx.User(t, "triage ordinary reviewer", "triage-ordinary@example.test")
	dbfx.Member(t, testWorkspaceID, user, "member")
	settings := testutil.Decode[TriageSettings](t, testHandler.GetTriageSettings, newRequest("GET", "/api/triage/settings", nil), 200)
	req := newRequest("PUT", "/api/triage/settings", map[string]any{"enabled": false, "acceptance_status": "todo", "responsibility_mode": "none", "expected_revision": settings.Revision})
	req.Header.Set("X-User-ID", user)
	testutil.Call(t, testHandler.UpdateTriageSettings, req).Want(403)
	item := triageCreateForTest(t)
	agent := dbfx.Agent(t, "legacy PAT triage agent", testRuntimeID)
	task := dbfx.Task(t, agent, testutil.Cols{"status": "running", "runtime_id": testRuntimeID, "originator_user_id": testUserID, "accountable_user_id": testUserID})
	req = withURLParam(newRequest("POST", "/api/triage/items/actions", map[string]any{"request_id": uuidToString(dbid.NewV7()), "expected_revision": item.Issue.Revision, "action": "accept"}), "id", item.Issue.ID)
	req.Header.Set("X-Agent-ID", agent)
	req.Header.Set("X-Task-ID", task)
	testutil.Call(t, testHandler.ActOnTriageItem, req).Want(403)
	req = withURLParam(newRequest("POST", "/api/triage/items/actions", map[string]any{"request_id": uuidToString(dbid.NewV7()), "expected_revision": item.Issue.Revision, "action": "accept"}), "id", item.Issue.ID)
	req.Header.Set("X-User-ID", user)
	testutil.Call(t, testHandler.ActOnTriageItem, req).Want(200)
}

func TestTriageWriteRechecksMembershipAfterFenceWait(t *testing.T) {
	triageEnableForTest(t)
	user := dbfx.User(t, "revoked triage reviewer", "triage-revoked@example.test")
	membership := dbfx.Member(t, testWorkspaceID, user, "member")
	item := triageCreateForTest(t)
	ctx := context.Background()
	fence, err := testPool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer fence.Rollback(ctx)
	if _, err = fence.Exec(ctx, `SELECT 1 FROM workspace_triage_settings WHERE workspace_id=$1 FOR UPDATE`, testWorkspaceID); err != nil {
		t.Fatal(err)
	}
	reached := make(chan struct{})
	h := *testHandler
	h.TxStarter = triageExecHookStarter{base: testPool, hook: func(sql string) error {
		if strings.Contains(sql, "SELECT 1 FROM workspace_triage_settings") {
			select {
			case <-reached:
			default:
				close(reached)
			}
		}
		return nil
	}}
	req := newRequest("POST", "/api/triage/items/actions", nil)
	req.Header.Set("X-User-ID", user)
	done := make(chan error, 1)
	go func() {
		_, e := h.actOnTriageItem(req, item.Issue.ID, TriageActionInput{RequestID: uuidToString(dbid.NewV7()), ExpectedRevision: item.Issue.Revision, Action: "accept"}, false)
		done <- e
	}()
	select {
	case <-reached:
	case <-time.After(3 * time.Second):
		t.Fatal("review never reached settings fence")
	}
	dbfx.Exec(t, `DELETE FROM member WHERE id=$1`, membership)
	if err = fence.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case err = <-done:
		var denial *triageError
		if !errors.As(err, &denial) || denial.Status != 403 {
			t.Fatalf("revoked reviewer was not denied: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("review blocked after fence released")
	}
	var n int
	dbfx.QueryRow(t, `SELECT count(*) FROM triage_action WHERE issue_id=$1`, item.Issue.ID).Scan(&n)
	if n != 0 {
		t.Fatal("revoked reviewer wrote action")
	}
}
func TestTriageDisableFencesConcurrentIntake(t *testing.T) {
	triageEnableForTest(t)
	ctx := context.Background()
	fence, err := testPool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer fence.Rollback(ctx)
	if _, err = fence.Exec(ctx, `SELECT 1 FROM workspace_triage_settings WHERE workspace_id=$1 FOR UPDATE`, testWorkspaceID); err != nil {
		t.Fatal(err)
	}
	reached := make(chan struct{})
	h := *testHandler
	h.TxStarter = triageExecHookStarter{base: testPool, hook: func(sql string) error {
		if strings.Contains(sql, "SELECT 1 FROM workspace_triage_settings") {
			select {
			case <-reached:
			default:
				close(reached)
			}
		}
		return nil
	}}
	done := make(chan int, 1)
	go func() {
		response := testutil.Call(t, h.CreateTriageItem, newRequest("POST", "/api/triage/items", triageInputForTest()))
		done <- response.Code
	}()
	select {
	case <-reached:
	case <-time.After(3 * time.Second):
		t.Fatal("intake never reached settings fence")
	}
	if _, err = fence.Exec(ctx, `UPDATE workspace_triage_settings SET enabled=false WHERE workspace_id=$1`, testWorkspaceID); err != nil {
		t.Fatal(err)
	}
	if err = fence.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case status := <-done:
		if status != 409 {
			t.Fatalf("intake racing disable returned %d", status)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("intake blocked after disable")
	}
	var n int
	dbfx.QueryRow(t, `SELECT count(*) FROM issue_triage WHERE workspace_id=$1`, testWorkspaceID).Scan(&n)
	if n != 0 {
		t.Fatal("disabled intake created pending work")
	}
}
func TestTriageHistoryFailureRollsBackAdmission(t *testing.T) {
	triageEnableForTest(t)
	item := triageCreateForTest(t)
	h := *testHandler
	h.TxStarter = triageExecHookStarter{base: testPool, hook: func(sql string) error {
		if strings.Contains(sql, "INSERT INTO triage_action") {
			return errors.New("injected history insert failure")
		}
		return nil
	}}
	_, err := h.actOnTriageItem(newRequest("POST", "/api/triage/items/actions", nil), item.Issue.ID, TriageActionInput{RequestID: uuidToString(dbid.NewV7()), ExpectedRevision: item.Issue.Revision, Action: "accept"}, false)
	if err == nil {
		t.Fatal("history fault was ignored")
	}
	var state string
	var revision int64
	dbfx.QueryRow(t, `SELECT admission_status,revision FROM issue WHERE id=$1`, item.Issue.ID).Scan(&state, &revision)
	if state != "pending" || revision != item.Issue.Revision {
		t.Fatal("history failure partially committed admission")
	}
}

type triageExecHookStarter struct {
	base txStarter
	hook func(string) error
}

func (s triageExecHookStarter) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := s.base.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &triageExecHookTx{Tx: tx, hook: s.hook}, nil
}

type triageExecHookTx struct {
	pgx.Tx
	hook func(string) error
}

func (t *triageExecHookTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if err := t.hook(sql); err != nil {
		return pgconn.CommandTag{}, err
	}
	return t.Tx.Exec(ctx, sql, args...)
}

func TestTriageWireFixtures(t *testing.T) {
	path := os.Getenv("TRIAGE_WIRE_FIXTURES")
	if path == "" {
		t.Skip("wire fixture export requested only by cross-package verification")
	}
	triageEnableForTest(t)
	item := triageCreateForTest(t)
	result := triageActionForTest(t, item, "accept", nil, 200)
	settings := testutil.Decode[map[string]any](t, testHandler.GetTriageSettings, newRequest("GET", "/api/triage/settings", nil), 200)
	items := testutil.Decode[map[string]any](t, testHandler.ListTriageItems, newRequest("GET", "/api/triage/items?view=history", nil), 200)
	history := testutil.Decode[map[string]any](t, testHandler.GetTriageHistory, newRequest("GET", "/api/triage/history", nil), 200)
	second := triageCreateForTest(t)
	batchPreview := testutil.Decode[map[string]any](t, testHandler.PreviewTriageBatch, newRequest("POST", "/api/triage/batch/preview", map[string]any{"items": []any{map[string]any{"issue_id": second.Issue.ID, "expected_revision": second.Issue.Revision}}, "action": "accept"}), 200)
	batchResult := testutil.Decode[map[string]any](t, testHandler.CommitTriageBatch, newRequest("POST", "/api/triage/batch", map[string]any{"items": []any{map[string]any{"issue_id": second.Issue.ID, "request_id": uuidToString(dbid.NewV7()), "expected_revision": second.Issue.Revision, "action": "accept"}}}), 200)
	raw, err := json.MarshalIndent(map[string]any{"settings": settings, "item": item, "actionResult": result, "items": items, "history": history, "batchPreview": batchPreview, "batchResult": batchResult}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
}

func TestTriageIntakeHonorsAgentAutonomy(t *testing.T) {
	triageEnableForTest(t)
	for _, level := range []string{"observer", "contributor"} {
		t.Run(level, func(t *testing.T) {
			agent := dbfx.Agent(t, "Triage "+level, testRuntimeID, testutil.Cols{"autonomy_level": level})
			task := dbfx.Task(t, agent, testutil.Cols{"status": "running", "runtime_id": testRuntimeID, "originator_user_id": testUserID, "accountable_user_id": testUserID})
			req := newRequest("POST", "/api/triage/items", triageInputForTest())
			req.Header.Set("X-Agent-ID", agent)
			req.Header.Set("X-Task-ID", task)
			status := 201
			if level == "observer" {
				status = 403
			}
			testutil.Call(t, testHandler.CreateTriageItem, req).Want(status)
		})
	}
}

func TestTriageSnoozeReplayAfterDueTime(t *testing.T) {
	triageEnableForTest(t)
	dbfx.Exec(t, `UPDATE workspace_triage_settings SET responsibility_mode='assign',responsibility_member_id=$2 WHERE workspace_id=$1`, testWorkspaceID, testUserID)
	item := triageCreateForTest(t)
	input := map[string]any{"request_id": uuidToString(dbid.NewV7()), "expected_revision": item.Issue.Revision, "action": "snooze", "snoozed_until": time.Now().Add(time.Second).Format(time.RFC3339Nano)}
	call := func() *http.Request {
		return withURLParam(newRequest("POST", "/api/triage/items/actions", input), "id", item.Issue.ID)
	}
	first := testutil.Decode[TriageActionResult](t, testHandler.ActOnTriageItem, call(), 200)
	time.Sleep(1100 * time.Millisecond)
	replay := testutil.Decode[TriageActionResult](t, testHandler.ActOnTriageItem, call(), 200)
	if replay.Action.ID != first.Action.ID {
		t.Fatal("due snooze replay lost durable action identity")
	}
	testHandler.deliverTriageNotifications(context.Background(), parseUUID(testWorkspaceID))
	var n int
	dbfx.QueryRow(t, `SELECT count(*) FROM inbox_item WHERE issue_id=$1 AND details->>'triage_event'=$2`, item.Issue.ID, "snooze:"+first.Action.ID).Scan(&n)
	if n != 1 {
		t.Fatal("fractional snooze deadline did not produce its due reminder")
	}
}
func TestTriageSnoozeReassignmentDeliversToCurrentReviewer(t *testing.T) {
	triageEnableForTest(t)
	newUser := dbfx.User(t, "New triage reviewer", "triage-reassigned@example.test")
	dbfx.Member(t, testWorkspaceID, newUser, "member")
	dbfx.Exec(t, `UPDATE workspace_triage_settings SET responsibility_mode='assign',responsibility_member_id=$2 WHERE workspace_id=$1`, testWorkspaceID, testUserID)
	item := triageCreateForTest(t)
	snoozed := triageActionForTest(t, item, "snooze", map[string]any{"snoozed_until": time.Now().Add(time.Hour).Format(time.RFC3339Nano)}, 200)
	triageActionForTest(t, snoozed.Item, "assign_reviewer", map[string]any{"reviewer_id": newUser}, 200)
	past := time.Now().Add(-time.Minute)
	dbfx.Exec(t, `UPDATE issue_triage SET snoozed_until=$2 WHERE issue_id=$1`, item.Issue.ID, past)
	dbfx.Exec(t, `UPDATE triage_notification SET due_at=$2,details=jsonb_set(details,'{snoozed_until}',to_jsonb($3::text)) WHERE event_key=$1`, "snooze:"+snoozed.Action.ID, past, past.Format(time.RFC3339Nano))
	testHandler.deliverTriageNotifications(context.Background(), parseUUID(testWorkspaceID))
	var oldCount, newCount int
	dbfx.QueryRow(t, `SELECT count(*) FILTER(WHERE recipient_id=$2),count(*) FILTER(WHERE recipient_id=$3) FROM inbox_item WHERE issue_id=$1 AND details->>'triage_event'=$4`, item.Issue.ID, testUserID, newUser, "snooze:"+snoozed.Action.ID).Scan(&oldCount, &newCount)
	if oldCount != 0 || newCount != 1 {
		t.Fatalf("due notification recipients old=%d current=%d", oldCount, newCount)
	}
}
func TestTriageOutboxDoesNotWaitOnBatchActorGuard(t *testing.T) {
	triageEnableForTest(t)
	ctx := context.Background()
	ws := parseUUID(testWorkspaceID)
	actor := parseUUID(testUserID)
	tx, err := testPool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err = testHandler.queueTriageNotification(ctx, tx, ws, actor, pgtype.UUID{}, pgtype.UUID{}, "review-batch:guard-test", "Batch complete", nil, time.Now()); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	guard, err := testPool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer guard.Rollback(ctx)
	if err = testHandler.Queries.WithTx(guard).LockSubscriberWrites(ctx, db.LockSubscriberWritesParams{WorkspaceID: ws, UserID: actor}); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- testHandler.deliverTriageNotificationsOnce(ctx, ws) }()
	select {
	case err = <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("delivery held outbox while waiting on actor advisory guard")
	}
	if err = guard.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	testHandler.deliverTriageNotifications(ctx, ws)
	var n int
	dbfx.QueryRow(t, `SELECT count(*) FROM inbox_item WHERE workspace_id=$1 AND details->>'triage_event'='review-batch:guard-test'`, ws).Scan(&n)
	if n != 1 {
		t.Fatal("skipped outbox did not recover after guard release")
	}
}
func TestTriageBatchSummaryTransportRetryIsStable(t *testing.T) {
	triageEnableForTest(t)
	item := triageCreateForTest(t)
	in := map[string]any{"items": []any{map[string]any{"issue_id": item.Issue.ID, "request_id": uuidToString(dbid.NewV7()), "expected_revision": item.Issue.Revision, "action": "accept"}}}
	for range 2 {
		testutil.Call(t, testHandler.CommitTriageBatch, newRequest("POST", "/api/triage/batch", in)).Want(200)
	}
	var n int
	dbfx.QueryRow(t, `SELECT count(*) FROM inbox_item WHERE workspace_id=$1 AND details->>'triage_event' LIKE 'review-batch:%'`, testWorkspaceID).Scan(&n)
	if n != 1 {
		t.Fatalf("batch retry created %d summary notifications", n)
	}
}

func TestTriageWorkspaceDeletionOwnsAllData(t *testing.T) {
	triageEnableForTest(t)
	control := triageCreateForTest(t)
	ws := dbfx.Workspace(t, "Triage teardown", "triage-teardown-"+uuidToString(dbid.NewV7()), testutil.Cols{"issue_prefix": "TDT"})
	dbfx.Member(t, ws, testUserID, "owner")
	issue := dbfx.Issue(t, "Triage teardown task", testutil.Cols{"workspace_id": ws, "admission_status": "pending", "status": "backlog"})
	dbfx.InsertNoID(t, "workspace_triage_settings", testutil.Cols{"workspace_id": ws, "enabled": true}, "workspace_id=$1", ws)
	dbfx.InsertNoID(t, "issue_triage", testutil.Cols{"workspace_id": ws, "issue_id": issue}, "workspace_id=$1", ws)
	dbfx.InsertNoID(t, "triage_intake_request", testutil.Cols{"workspace_id": ws, "actor_id": testUserID, "request_id": uuidToString(dbid.NewV7()), "payload_hash": "fixture", "issue_id": issue}, "workspace_id=$1", ws)
	dbfx.Insert(t, "triage_action", testutil.Cols{"id": uuidToString(dbid.NewV7()), "workspace_id": ws, "issue_id": issue, "actor_id": testUserID, "request_id": uuidToString(dbid.NewV7()), "payload_hash": "fixture", "action": "snooze", "round": 1, "before_snapshot": "{}", "after_snapshot": "{}"})
	batch := dbfx.Insert(t, "triage_import_batch", testutil.Cols{"id": uuidToString(dbid.NewV7()), "workspace_id": ws, "actor_id": testUserID, "request_id": uuidToString(dbid.NewV7()), "payload_hash": "fixture", "filename": "deleted.csv"})
	dbfx.InsertNoID(t, "triage_import_row", testutil.Cols{"batch_id": batch, "workspace_id": ws, "row_number": 1, "issue_id": issue, "status": "created"}, "workspace_id=$1", ws)
	dbfx.Insert(t, "triage_notification", testutil.Cols{"id": uuidToString(dbid.NewV7()), "workspace_id": ws, "recipient_id": testUserID, "event_key": "fixture", "title": "Triage deletion", "issue_id": issue, "batch_id": batch})
	testutil.Call(t, testHandler.DeleteWorkspace, withURLParam(newRequest("DELETE", "/api/workspaces/"+ws, nil), "id", ws)).Want(204)
	for _, table := range []string{"workspace_triage_settings", "issue_triage", "triage_intake_request", "triage_action", "triage_import_batch", "triage_import_row", "triage_notification"} {
		var n int
		dbfx.QueryRow(t, "SELECT count(*) FROM "+table+" WHERE workspace_id=$1", ws).Scan(&n)
		if n != 0 {
			t.Errorf("workspace delete left %d rows in %s", n, table)
		}
	}
	var n int
	dbfx.QueryRow(t, `SELECT count(*) FROM issue_triage WHERE issue_id=$1`, control.Issue.ID).Scan(&n)
	if n != 1 {
		t.Fatal("workspace deletion touched another workspace's triage")
	}
}

func TestTriageLostCommitResponseResumesSameExecution(t *testing.T) {
	for _, phase := range []string{"admission", "enqueue"} {
		t.Run(phase, func(t *testing.T) {
			triageEnableForTest(t)
			agent := dbfx.Agent(t, "Triage lost "+phase, testRuntimeID)
			input := triageInputForTest()
			input["candidate_assignee_type"] = "agent"
			input["candidate_assignee_id"] = agent
			item := testutil.Decode[TriageItem](t, testHandler.CreateTriageItem, newRequest("POST", "/api/triage/items", input), 201)
			in := map[string]any{"request_id": uuidToString(dbid.NewV7()), "expected_revision": item.Issue.Revision, "action": "accept_and_execute"}
			h := *testHandler
			h.TxStarter = triageLostCommitStarter{base: testPool, phase: phase, once: &sync.Once{}}
			call := func() *http.Request {
				return withURLParam(newRequest("POST", "/api/triage/items/actions", in), "id", item.Issue.ID)
			}
			if phase == "admission" {
				testutil.Call(t, h.ActOnTriageItem, call()).Want(500)
			}
			first := testutil.Decode[TriageActionResult](t, h.ActOnTriageItem, call(), 200)
			replay := testutil.Decode[TriageActionResult](t, h.ActOnTriageItem, call(), 200)
			if first.Action.TaskID == nil || first.Action.ExecutionStatus != "queued" || replay.Action.ID != first.Action.ID || replay.Action.TaskID == nil || *replay.Action.TaskID != *first.Action.TaskID {
				t.Fatalf("lost commit failed durable recovery: %+v %+v", first.Action, replay.Action)
			}
			var tasks, actions int
			dbfx.QueryRow(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id=$1`, item.Issue.ID).Scan(&tasks)
			dbfx.QueryRow(t, `SELECT count(*) FROM triage_action WHERE issue_id=$1`, item.Issue.ID).Scan(&actions)
			if tasks != 1 || actions != 1 {
				t.Fatalf("lost commit repeated side effects: tasks=%d actions=%d", tasks, actions)
			}
		})
	}
}

type triageLostCommitStarter struct {
	base  txStarter
	phase string
	once  *sync.Once
}

func (s triageLostCommitStarter) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := s.base.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &triageLostCommitTx{Tx: tx, phase: s.phase, once: s.once}, nil
}

type triageLostCommitTx struct {
	pgx.Tx
	phase  string
	once   *sync.Once
	marked bool
}

func (t *triageLostCommitTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if t.phase == "admission" && strings.Contains(sql, "INSERT INTO triage_action") || t.phase == "enqueue" && strings.Contains(sql, "UPDATE triage_action SET task_id=") {
		t.marked = true
	}
	return t.Tx.Exec(ctx, sql, args...)
}
func (t *triageLostCommitTx) Commit(ctx context.Context) error {
	if err := t.Tx.Commit(ctx); err != nil {
		return err
	}
	lost := false
	if t.marked {
		t.once.Do(func() { lost = true })
	}
	if lost {
		return errors.New("injected lost response after durable commit")
	}
	return nil
}

func TestTriageCreatorFollowsCommentsAndResultWithoutDispatch(t *testing.T) {
	triageEnableForTest(t)
	input := triageInputForTest()
	item := testutil.Decode[TriageItem](t, testHandler.CreateTriageItem, newRequest("POST", "/api/triage/items", input), 201)
	var active int
	dbfx.QueryRow(t, `SELECT count(*) FROM issue_subscriber WHERE issue_id=$1 AND user_type='member' AND user_id=$2 AND reason='creator' AND unsubscribed_at IS NULL`, item.Issue.ID, testUserID).Scan(&active)
	if active != 1 {
		t.Fatal("inert intake did not preserve creator's normal comment subscription")
	}
	reviewer := dbfx.User(t, "Triage follow-up reviewer", "triage-comment-reviewer@example.test")
	dbfx.Member(t, testWorkspaceID, reviewer, "member")
	comment := withURLParam(newRequest("POST", "/api/issues/comments", map[string]any{"content": "Please confirm the reproduction steps."}), "id", item.Issue.ID)
	comment.Header.Set("X-User-ID", reviewer)
	testutil.Call(t, testHandler.CreateComment, comment).Want(201)
	current := testutil.Decode[TriageItem](t, testHandler.GetTriageItem, withURLParam(newRequest("GET", "/api/triage/items", nil), "id", item.Issue.ID), 200)
	action := withURLParam(newRequest("POST", "/api/triage/items/actions", map[string]any{"request_id": uuidToString(dbid.NewV7()), "expected_revision": current.Issue.Revision, "action": "reject", "reason": "Confirmed as unsupported"}), "id", item.Issue.ID)
	action.Header.Set("X-User-ID", reviewer)
	testutil.Call(t, testHandler.ActOnTriageItem, action).Want(200)
	var notices, tasks int
	dbfx.QueryRow(t, `SELECT count(*) FROM inbox_item WHERE issue_id=$1 AND recipient_id=$2 AND type='triage'`, item.Issue.ID, testUserID).Scan(&notices)
	dbfx.QueryRow(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id=$1`, item.Issue.ID).Scan(&tasks)
	if notices != 1 || tasks != 0 {
		var pending, delivered, future int
		dbfx.QueryRow(t, `SELECT count(*) FILTER(WHERE delivered_at IS NULL),count(*) FILTER(WHERE delivered_at IS NOT NULL),count(*) FILTER(WHERE due_at>now()) FROM triage_notification WHERE issue_id=$1`, item.Issue.ID).Scan(&pending, &delivered, &future)
		t.Logf("outbox before retry pending=%d delivered=%d future=%d", pending, delivered, future)
		t.Fatalf("creator result/subscriber dedup or inertness broken: notices=%d tasks=%d", notices, tasks)
	}
	dbfx.Exec(t, `UPDATE issue_subscriber SET unsubscribed_at=now() WHERE issue_id=$1 AND user_id=$2`, item.Issue.ID, testUserID)
	testutil.Call(t, testHandler.CreateTriageItem, newRequest("POST", "/api/triage/items", input)).Want(201)
	dbfx.QueryRow(t, `SELECT count(*) FROM issue_subscriber WHERE issue_id=$1 AND user_id=$2 AND unsubscribed_at IS NULL`, item.Issue.ID, testUserID).Scan(&active)
	if active != 0 {
		t.Fatal("intake replay resurrected unsubscribe tombstone")
	}
}

func TestTriageImmediateOutboxUsesTransactionClock(t *testing.T) {
	triageEnableForTest(t)
	ctx := context.Background()
	tx, err := testPool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if err = testHandler.queueTriageNotification(ctx, tx, parseUUID(testWorkspaceID), parseUUID(testUserID), pgtype.UUID{}, pgtype.UUID{}, "clock-proof", "Immediate result", nil, time.Now()); err != nil {
		t.Fatal(err)
	}
	var immediatelyDue bool
	if err = tx.QueryRow(ctx, `SELECT due_at<=created_at FROM triage_notification WHERE event_key='clock-proof' AND workspace_id=$1`, testWorkspaceID).Scan(&immediatelyDue); err != nil {
		t.Fatal(err)
	}
	if !immediatelyDue {
		t.Fatal("immediate notification is scheduled in the future of its database transaction clock")
	}
}

func TestTriageDateFiltersIncludeWholeDayAndExactInstants(t *testing.T) {
	triageEnableForTest(t)
	times := []string{"2026-10-04T00:00:00Z", "2026-10-04T23:59:59Z", "2026-10-05T00:00:00Z"}
	items := []TriageItem{}
	for _, at := range times {
		item := triageCreateForTest(t)
		dbfx.Exec(t, `UPDATE issue_triage SET entered_at=$2::timestamptz WHERE issue_id=$1`, item.Issue.ID, at)
		items = append(items, item)
	}
	for _, check := range []struct {
		query string
		total int
	}{{"entered_after=2026-10-04&entered_before=2026-10-04", 2}, {"entered_before=2026-10-04T00:00:00Z", 1}, {"entered_before=2026-10-04T23:59:59Z", 2}} {
		out := testutil.Decode[struct {
			Total int `json:"total"`
		}](t, testHandler.ListTriageItems, newRequest("GET", "/api/triage/items?view=all&"+check.query, nil), 200)
		if out.Total != check.total {
			t.Errorf("%s total=%d want%d", check.query, out.Total, check.total)
		}
	}
	for i, item := range items {
		result := triageActionForTest(t, item, "reject", map[string]any{"reason": "Historical date boundary fixture"}, 200)
		dbfx.Exec(t, `UPDATE triage_action SET created_at=$2::timestamptz WHERE id=$1`, result.Action.ID, times[i])
	}
	for _, check := range []struct {
		query string
		total int
	}{{"processed_after=2026-10-04&processed_before=2026-10-04", 2}, {"processed_before=2026-10-04T00:00:00Z", 1}} {
		out := testutil.Decode[struct {
			Total int `json:"total"`
		}](t, testHandler.GetTriageHistory, newRequest("GET", "/api/triage/history?"+check.query, nil), 200)
		if out.Total != check.total {
			t.Errorf("history%s total=%d want%d", check.query, out.Total, check.total)
		}
		out = testutil.Decode[struct {
			Total int `json:"total"`
		}](t, testHandler.ListTriageItems, newRequest("GET", "/api/triage/items?view=history&"+check.query, nil), 200)
		if out.Total != check.total {
			t.Errorf("items%s total=%d want%d", check.query, out.Total, check.total)
		}
	}
}

func TestTriageDuplicateTargetsAreScopedFormalAndUnchanged(t *testing.T) {
	triageEnableForTest(t)
	item := triageCreateForTest(t)
	foreignWS := dbfx.Workspace(t, "Foreign duplicate targets", "triage-duplicate-foreign-"+uuidToString(dbid.NewV7()))
	foreign := dbfx.Issue(t, "Secret foreign duplicate title", testutil.Cols{"workspace_id": foreignWS})
	pending := triageCreateForTest(t)
	rejected := dbfx.Issue(t, "Rejected target", testutil.Cols{"admission_status": "rejected", "status": "cancelled"})
	deleted := dbfx.Issue(t, "Deleted target")
	dbfx.Exec(t, `DELETE FROM issue WHERE id=$1`, deleted)
	for name, target := range map[string]string{"self": item.Issue.ID, "foreign": foreign, "pending": pending.Issue.ID, "rejected": rejected, "deleted": deleted} {
		t.Run(name, func(t *testing.T) {
			req := withURLParam(newRequest("POST", "/api/triage/items/actions", map[string]any{"request_id": uuidToString(dbid.NewV7()), "expected_revision": item.Issue.Revision, "action": "duplicate", "duplicate_issue_id": target}), "id", item.Issue.ID)
			response := testutil.Call(t, testHandler.ActOnTriageItem, req).Want(400)
			if strings.Contains(response.Body.String(), "Secret foreign") {
				t.Fatal("duplicate rejection leaked foreign target")
			}
		})
	}
	target := dbfx.Issue(t, "Closed formal duplicate target", testutil.Cols{"status": "done"})
	dbfx.Comment(t, item.Issue.ID, "Preserved source comment")
	dbfx.Comment(t, target, "Preserved target comment")
	before, err := testHandler.Queries.GetIssueInWorkspace(context.Background(), db.GetIssueInWorkspaceParams{ID: parseUUID(target), WorkspaceID: parseUUID(testWorkspaceID)})
	if err != nil {
		t.Fatal(err)
	}
	result := triageActionForTest(t, item, "duplicate", map[string]any{"duplicate_issue_id": target, "reason": "Same request"}, 200)
	if result.Item.Issue.AdmissionStatus != "duplicate" || result.Item.DuplicateIssueID == nil || *result.Item.DuplicateIssueID != target {
		t.Fatalf("closed formal target rejected %+v", result.Item)
	}
	after, err := testHandler.Queries.GetIssueInWorkspace(context.Background(), db.GetIssueInWorkspaceParams{ID: parseUUID(target), WorkspaceID: parseUUID(testWorkspaceID)})
	if err != nil {
		t.Fatal(err)
	}
	a, _ := json.Marshal(before)
	b, _ := json.Marshal(after)
	if string(a) != string(b) {
		t.Fatal("duplicate action mutated formal target")
	}
	for _, id := range []string{item.Issue.ID, target} {
		var n int
		dbfx.QueryRow(t, `SELECT count(*) FROM comment WHERE issue_id=$1`, id).Scan(&n)
		if n != 1 {
			t.Fatal("duplicate action moved or copied comments")
		}
	}
	identifier := result.Item.DuplicateIdentifier
	dbfx.Exec(t, `DELETE FROM issue WHERE id=$1`, target)
	live := testutil.Decode[TriageItem](t, testHandler.GetTriageItem, withURLParam(newRequest("GET", "/api/triage/items", nil), "id", item.Issue.ID), 200)
	if live.DuplicateIdentifier == nil || identifier == nil || *live.DuplicateIdentifier != *identifier {
		t.Fatal("deleted duplicate target lost historical identifier")
	}
}

func TestTriageOrdinaryAcceptanceAdmitsProjectWithoutExecutionOrInheritance(t *testing.T) {
	triageEnableForTest(t)
	agent := dbfx.Agent(t, "Explicit triage candidate", testRuntimeID)
	leader := dbfx.Agent(t, "Project default triage leader", testRuntimeID)
	squad := dbfx.Squad(t, "Project default triage squad", leader)
	defaults, _ := json.Marshal([]map[string]any{{"state": "configured", "squad_id": squad}})
	project := dbfx.Project(t, "Triage formal count project", testutil.Cols{"execution_squad": defaults})
	for index, withAgent := range []bool{true, false} {
		input := triageInputForTest()
		input["candidate_project_id"] = project
		if withAgent {
			input["candidate_assignee_type"] = "agent"
			input["candidate_assignee_id"] = agent
		}
		item := testutil.Decode[TriageItem](t, testHandler.CreateTriageItem, newRequest("POST", "/api/triage/items", input), 201)
		before := testutil.Decode[ProjectResponse](t, testHandler.GetProject, withURLParam(newRequest("GET", "/api/projects", nil), "id", project), 200)
		if before.IssueCount != int64(index) {
			t.Fatal("candidate input entered project count before acceptance")
		}
		accepted := triageActionForTest(t, item, "accept", nil, 200)
		if accepted.Item.Issue.ProjectID == nil || *accepted.Item.Issue.ProjectID != project {
			t.Fatal("candidate project not admitted")
		}
		if withAgent && (accepted.Item.Issue.AssigneeID == nil || *accepted.Item.Issue.AssigneeID != agent) {
			t.Fatal("explicit agent candidate not admitted")
		}
		if !withAgent && accepted.Item.Issue.AssigneeID != nil {
			t.Fatal("ordinary acceptance inherited project execution squad")
		}
		after := testutil.Decode[ProjectResponse](t, testHandler.GetProject, withURLParam(newRequest("GET", "/api/projects", nil), "id", project), 200)
		if after.IssueCount != int64(index+1) {
			t.Fatal("formal project count did not increase exactly once")
		}
		var n int
		dbfx.QueryRow(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id=$1`, item.Issue.ID).Scan(&n)
		if n != 0 {
			t.Fatal("ordinary acceptance enqueued explicit agent or project squad")
		}
	}
}
