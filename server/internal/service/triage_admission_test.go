package service

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func triageBoundaryFixture(t *testing.T) (*testutil.Fixture, *TaskService, pgtype.UUID, pgtype.UUID) {
	t.Helper()
	pool := newTaskClaimRacePool(t)
	fx := testutil.New(pool, "", "")
	suffix := uuid.NewString()
	fx.UserID = fx.User(t, "Triage admission", suffix+"@test.invalid")
	fx.WorkspaceID = fx.Workspace(t, "Triage admission", suffix)
	fx.Member(t, fx.WorkspaceID, fx.UserID, "owner")
	runtime := util.MustParseUUID(fx.Runtime(t, "Triage fake runtime"))
	agent := util.MustParseUUID(fx.Agent(t, "Triage fake agent", util.UUIDToString(runtime)))
	return fx, NewTaskService(db.New(pool), pool, nil, events.New()), agent, runtime
}

func TestTriageAdmissionEnqueueAndRuntimeBoundaries(t *testing.T) {
	for _, state := range []string{"pending", "rejected", "duplicate", "not_required", "accepted"} {
		for _, path := range []string{"enqueue", "claim", "runtime", "batch", "start", "retry", "promote", "rerun"} {
			t.Run(state+"/"+path, func(t *testing.T) {
				fx, s, agent, runtime := triageBoundaryFixture(t)
				issueID := fx.Issue(t, "Triage boundary", testutil.Cols{"admission_status": state, "assignee_type": "agent", "assignee_id": agent})
				issue, err := s.Queries.GetIssue(t.Context(), util.MustParseUUID(issueID))
				if err != nil {
					t.Fatal(err)
				}
				allowed := state == "not_required" || state == "accepted"
				var executed bool
				switch path {
				case "enqueue":
					_, err = s.EnqueueTaskForIssue(t.Context(), issue, pgtype.UUID{})
					executed = err == nil
				case "rerun":
					_, err = s.RerunIssue(t.Context(), issue.ID, pgtype.UUID{}, pgtype.UUID{}, util.MustParseUUID(fx.UserID), func(db.Agent) bool { return true })
					executed = err == nil
				default:
					status := "queued"
					if path == "start" {
						status = "dispatched"
					}
					if path == "retry" {
						status = "failed"
					}
					if path == "promote" {
						status = "deferred"
					}
					taskID := util.MustParseUUID(fx.Task(t, util.UUIDToString(agent), testutil.Cols{"issue_id": issue.ID, "runtime_id": runtime, "status": status, "fire_at": testutil.Raw("now() - interval '1 second'")}))
					switch path {
					case "claim":
						task, e := s.ClaimTask(t.Context(), agent)
						err = e
						executed = task != nil
					case "runtime":
						task, e := s.ClaimTaskForRuntime(t.Context(), runtime)
						err = e
						executed = task != nil
					case "batch":
						tasks, e := s.ClaimTasksForRuntimes(t.Context(), []pgtype.UUID{runtime}, 1)
						err = e
						executed = len(tasks) > 0
					case "start":
						_, err = s.Queries.StartAgentTask(t.Context(), taskID)
						executed = err == nil
					case "retry":
						_, err = s.Queries.CreateRetryTask(t.Context(), db.CreateRetryTaskParams{ID: taskID})
						executed = err == nil
					case "promote":
						_, err = s.Queries.PromoteDeferredChannelIssueTask(t.Context(), taskID)
						executed = err == nil
					}
				}
				if executed != allowed {
					t.Fatalf("admission=%s path=%s executed=%v allowed=%v err=%v", state, path, executed, allowed, err)
				}
				if allowed && err != nil {
					t.Fatal(err)
				}
				if !allowed && err != nil && !errors.Is(err, pgx.ErrNoRows) && path != "enqueue" && path != "rerun" {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestTriagePendingCommentNeverDispatchesAfterAcceptance(t *testing.T) {
	fx, s, agent, runtime := triageBoundaryFixture(t)
	issueID := util.MustParseUUID(fx.Issue(t, "Pending comment", testutil.Cols{"admission_status": "pending", "assignee_type": "agent", "assignee_id": agent}))
	c, err := s.Queries.CreateComment(t.Context(), db.CreateCommentParams{IssueID: issueID, WorkspaceID: util.MustParseUUID(fx.WorkspaceID), AuthorType: "member", AuthorID: util.MustParseUUID(fx.UserID), Content: "Please execute", Type: "comment"})
	if err != nil {
		t.Fatal(err)
	}
	fx.Exec(t, "UPDATE issue SET admission_status='accepted' WHERE id=$1", issueID)
	issue, err := s.Queries.GetIssue(t.Context(), issueID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.EnqueueTaskForIssue(t.Context(), issue, c.ID)
	if err == nil {
		t.Fatal("pending-era comment executed after acceptance")
	}
	taskID := util.MustParseUUID(fx.Task(t, util.UUIDToString(agent), testutil.Cols{"issue_id": issueID, "runtime_id": runtime}))
	_, err = s.Queries.MergeCommentIntoPendingTask(t.Context(), db.MergeCommentIntoPendingTaskParams{IssueID: issueID, AgentID: agent, NewTriggerCommentID: c.ID})
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("pending-era merge error=%v", err)
	}
	fx.Exec(t, "UPDATE agent_task_queue SET status='running' WHERE id=$1", taskID)
	_, err = s.Queries.RegisterPlannedCommentForActiveTask(t.Context(), db.RegisterPlannedCommentForActiveTaskParams{IssueID: issueID, AgentID: agent, CommentID: c.ID})
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("pending-era obligation error=%v", err)
	}
	if got := fx.Count(t, "SELECT cardinality(coalesced_comment_ids) FROM agent_task_queue WHERE id=$1", taskID); got != 0 {
		t.Fatal(fmt.Sprintf("hidden obligation retained: %d", got))
	}
}

func TestTriageClaimFinalizationCannotMintExecutionCredentials(t *testing.T) {
	fx, s, agent, runtime := triageBoundaryFixture(t)
	issueID := util.MustParseUUID(fx.Issue(t, "Unreviewed delivery", testutil.Cols{"admission_status": "pending"}))
	taskID := util.MustParseUUID(fx.Task(t, util.UUIDToString(agent), testutil.Cols{"issue_id": issueID, "status": "dispatched", "runtime_id": runtime, "dispatched_at": testutil.Raw("now()")}))
	task, err := s.Queries.GetAgentTask(t.Context(), taskID)
	if err != nil {
		t.Fatal(err)
	}
	hash := "triage-test-" + uuid.NewString()
	_, err = s.FinalizeTaskClaim(t.Context(), task, db.CreateTaskTokenParams{TokenHash: hash, TaskID: task.ID, AgentID: agent, WorkspaceID: util.MustParseUUID(fx.WorkspaceID), UserID: util.MustParseUUID(fx.UserID), ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(time.Hour), Valid: true}}, nil, false)
	fx.Cleanup(t, "DELETE FROM task_token WHERE token_hash=$1", hash)
	if err == nil {
		t.Fatal("nonformal claim minted execution credentials without a comment receipt")
	}
	if n := fx.Count(t, "SELECT count(*) FROM task_token WHERE token_hash=$1", hash); n != 0 {
		t.Fatal("blocked claim retained execution credentials")
	}
}

func TestTriageTextAndDecisionEventsCannotTriggerPlugins(t *testing.T) {
	seen := 0
	bus := events.New()
	SubscribePluginEvents(bus, recordingSink(func(string) { seen++ }))
	for _, kind := range []string{protocol.EventIssueUpdated, protocol.EventIssueCreated, protocol.EventCommentCreated} {
		bus.Publish(events.Event{Type: kind, Payload: map[string]any{"suppress_execution": true, "status_changed": true}})
	}
	if seen != 0 {
		t.Fatalf("triage text/review events invoked %d execution-capable plugin hooks", seen)
	}
	bus.Publish(events.Event{Type: protocol.EventIssueUpdated, Payload: map[string]any{"status_changed": true}})
	if seen != 2 {
		t.Fatalf("formal status hooks changed: %d", seen)
	}
}

func TestTriageAdmissionSQLReplayDeferredAndLink(t *testing.T) {
	for _, state := range []string{"pending", "rejected", "duplicate", "accepted", "not_required"} {
		for _, path := range []string{"insert", "deferred", "replay", "batch_replay", "due", "batch_due", "link", "waiting_start", "quick_parent"} {
			t.Run(state+"/"+path, func(t *testing.T) {
				fx, s, agent, runtime := triageBoundaryFixture(t)
				issue := util.MustParseUUID(fx.Issue(t, "SQL admission", testutil.Cols{"admission_status": state}))
				allowed := state == "accepted" || state == "not_required"
				var err error
				var executed bool
				switch path {
				case "insert":
					_, err = s.Queries.CreateAgentTask(t.Context(), db.CreateAgentTaskParams{AgentID: agent, RuntimeID: runtime, IssueID: issue})
					executed = err == nil
				case "deferred":
					_, err = s.Queries.CreateDeferredAgentTask(t.Context(), db.CreateDeferredAgentTaskParams{AgentID: agent, RuntimeID: runtime, IssueID: issue, FireAt: pgtype.Timestamptz{Time: time.Now(), Valid: true}})
					executed = err == nil
				case "quick_parent":
					raw := []byte(fmt.Sprintf(`{"type":"quick_create","parent_issue_id":%q}`, util.UUIDToString(issue)))
					_, err = s.Queries.CreateQuickCreateTask(t.Context(), db.CreateQuickCreateTaskParams{AgentID: agent, RuntimeID: runtime, Context: raw})
					executed = err == nil
				default:
					status := "dispatched"
					if path == "due" || path == "batch_due" {
						status = "deferred"
					}
					if path == "waiting_start" {
						status = "waiting_local_directory"
					}
					cols := testutil.Cols{"issue_id": issue, "runtime_id": runtime, "status": status, "dispatched_at": testutil.Raw("now() - interval '2 minutes'"), "fire_at": testutil.Raw("now() - interval '2 minutes'")}
					if path == "link" {
						cols["issue_id"] = nil
					}
					task := util.MustParseUUID(fx.Task(t, util.UUIDToString(agent), cols))
					switch path {
					case "replay":
						_, err = s.Queries.ReclaimStaleDispatchedTaskForRuntime(t.Context(), db.ReclaimStaleDispatchedTaskForRuntimeParams{RuntimeID: runtime, PrepareLeaseSecs: 60, ClaimRecoverySecs: 1, RuntimeStaleSecs: RuntimeClaimFreshnessSeconds})
						executed = err == nil
					case "batch_replay":
						rows, e := s.Queries.ReclaimStaleDispatchedTasksForRuntimes(t.Context(), db.ReclaimStaleDispatchedTasksForRuntimesParams{RuntimeIds: []pgtype.UUID{runtime}, MaxTasks: 10, PrepareLeaseSecs: 60, ClaimRecoverySecs: 1, RuntimeStaleSecs: RuntimeClaimFreshnessSeconds})
						err = e
						executed = len(rows) > 0
					case "due":
						rows, e := s.Queries.PromoteDueDeferredTasksForRuntime(t.Context(), db.PromoteDueDeferredTasksForRuntimeParams{RuntimeID: runtime, RuntimeStaleSecs: RuntimeClaimFreshnessSeconds})
						err = e
						executed = len(rows) > 0
					case "batch_due":
						rows, e := s.Queries.PromoteDueDeferredTasksForRuntimes(t.Context(), db.PromoteDueDeferredTasksForRuntimesParams{RuntimeIds: []pgtype.UUID{runtime}, RuntimeStaleSecs: RuntimeClaimFreshnessSeconds})
						err = e
						executed = len(rows) > 0
					case "link":
						err = s.Queries.LinkTaskToIssue(t.Context(), db.LinkTaskToIssueParams{ID: task, IssueID: issue})
						executed = fx.Count(t, "SELECT count(*) FROM agent_task_queue WHERE id=$1 AND issue_id=$2", task, issue) == 1
					case "waiting_start":
						_, err = s.Queries.StartAgentTask(t.Context(), task)
						executed = err == nil
					}
				}
				if err != nil && !errors.Is(err, pgx.ErrNoRows) {
					t.Fatal(err)
				}
				if executed != allowed {
					t.Fatalf("state=%s path=%s executed=%v allowed=%v error=%v", state, path, executed, allowed, err)
				}
			})
		}
	}
}

func TestTriageStaleContentEditPreservesAcceptedAssignment(t *testing.T) {
	fx, s, agent, _ := triageBoundaryFixture(t)
	id := util.MustParseUUID(fx.Issue(t, "Before review", testutil.Cols{"admission_status": "pending"}))
	stale, err := s.Queries.GetIssue(t.Context(), id)
	if err != nil {
		t.Fatal(err)
	}
	fx.Exec(t, "UPDATE issue SET admission_status='accepted',assignee_type='agent',assignee_id=$2,status='todo' WHERE id=$1", id, agent)
	title := "Edited content"
	updated, err := (&IssueService{Queries: s.Queries}).UpdateContent(t.Context(), stale, IssueContentPatch{Title: &title})
	if err != nil {
		t.Fatal(err)
	}
	if updated.AssigneeID != agent || updated.AssigneeType.String != "agent" || updated.AdmissionStatus != "accepted" || updated.Status != "todo" {
		t.Fatalf("stale content restored old review fields: %+v", updated)
	}
}

func TestTriageFailureSettlementSurvivesRetryRefusal(t *testing.T) {
	fx, s, agent, runtime := triageBoundaryFixture(t)
	issue := util.MustParseUUID(fx.Issue(t, "Retry refusal", testutil.Cols{"admission_status": "pending"}))
	taskID := util.MustParseUUID(fx.Task(t, util.UUIDToString(agent), testutil.Cols{"issue_id": issue, "runtime_id": runtime, "status": "running", "attempt": 1, "max_attempts": 3}))
	task, err := s.FailTask(t.Context(), taskID, "temporary network failure", "", "", "", "provider_network", false, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if task.Status != "failed" {
		t.Fatalf("failure settlement rolled back: %s", task.Status)
	}
	if n := fx.Count(t, "SELECT count(*) FROM agent_task_queue WHERE parent_task_id=$1", taskID); n != 0 {
		t.Fatal("nonformal failure queued retry")
	}
}

func TestTriageSourceContextCannotHidePendingExecutionBehindNullIssueID(t *testing.T) {
	for _, state := range []string{"pending", "rejected", "duplicate", "accepted", "not_required"} {
		t.Run(state, func(t *testing.T) {
			fx, s, agent, runtime := triageBoundaryFixture(t)
			issue := fx.Issue(t, "Captured review input", testutil.Cols{"admission_status": state})
			comment := fx.Comment(t, issue, "Branch context")
			source := fx.Insert(t, "issue_source_context", testutil.Cols{"id": uuid.NewString(), "workspace_id": fx.WorkspaceID, "source_issue_id": issue, "anchor_comment_id": comment, "captured_by_user_id": fx.UserID, "snapshot_version": 1, "snapshot": "{}", "capture_digest": "test", "state": "abandoned"})
			raw := []byte(fmt.Sprintf(`{"type":"quick_create","source_context_id":%q}`, source))
			_, err := s.Queries.CreateQuickCreateTask(t.Context(), db.CreateQuickCreateTaskParams{AgentID: agent, RuntimeID: runtime, Context: raw})
			allowed := state == "accepted" || state == "not_required"
			if (err == nil) != allowed {
				t.Fatalf("source %s admitted=%v err=%v", state, err == nil, err)
			}
		})
	}
}

func TestTriageChildProgressCountsOnlyFormalWork(t *testing.T) {
	fx, s, _, _ := triageBoundaryFixture(t)
	parent := fx.Issue(t, "Formal parent progress")
	for _, state := range []string{"pending", "rejected", "duplicate", "accepted", "not_required"} {
		fx.Issue(t, state, testutil.Cols{"parent_issue_id": parent, "admission_status": state, "status": "done"})
	}
	rows, err := s.Queries.ChildIssueProgress(t.Context(), db.ChildIssueProgressParams{WorkspaceID: util.MustParseUUID(fx.WorkspaceID), TerminalStatusKeys: []string{"done", "cancelled"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Total != 2 || rows[0].Done != 2 {
		t.Fatalf("progress includes review inputs: %+v", rows)
	}
}
