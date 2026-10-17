package service

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/multica-ai/multica/server/internal/attribution"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type legacyExecutionFixture struct {
	fx                      *testutil.Fixture
	svc                     *TaskService
	agent, runtime          pgtype.UUID
	shared, ordinary, child pgtype.UUID
}

func newLegacyExecutionFixture(t *testing.T) legacyExecutionFixture {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("requires task-owned DATABASE_URL")
	}
	pool, err := pgxpool.New(t.Context(), url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	fx := testutil.New(pool, "", "")
	fx.UserID = fx.User(t, "Legacy execution actor", "legacy-execution-"+uuid.NewString()+"@example.test")
	fx.WorkspaceID = fx.Workspace(t, "Legacy execution workspace", "legacy-execution-"+uuid.NewString())
	fx.Member(t, fx.WorkspaceID, fx.UserID, "owner")
	runtime := fx.Runtime(t, "Boundary fake runtime")
	agent := fx.Agent(t, "Boundary fake agent", runtime)
	shared := fx.Issue(t, "Shared source", testutil.Cols{"assignee_type": "member", "assignee_id": fx.UserID})
	fx.InsertNoID(t, "issue_collaboration", testutil.Cols{"issue_id": shared}, "issue_id=$1", shared)
	fx.Insert(t, "collaboration_session", testutil.Cols{"issue_id": shared})
	ordinary := fx.Issue(t, "Ordinary source", testutil.Cols{"assignee_type": "agent", "assignee_id": agent})
	child := fx.Issue(t, "Ordinary child", testutil.Cols{"parent_issue_id": shared, "assignee_type": "agent", "assignee_id": agent})
	fx.Cleanup(t, "DELETE FROM agent_task_queue WHERE agent_id=$1", agent)
	fx.Cleanup(t, "DELETE FROM task_token WHERE agent_id=$1", agent)
	fx.Cleanup(t, "DELETE FROM daemon_token WHERE workspace_id=$1", fx.WorkspaceID)
	return legacyExecutionFixture{
		fx: fx, svc: NewTaskService(db.New(pool), pool, nil, events.New()),
		agent: util.MustParseUUID(agent), runtime: util.MustParseUUID(runtime),
		shared: util.MustParseUUID(shared), ordinary: util.MustParseUUID(ordinary), child: util.MustParseUUID(child),
	}
}

func TestLegacyExecutionCollaborationEnqueueBoundary(t *testing.T) {
	for _, path := range []string{"assignment", "mention", "thread", "squad", "deferred", "channel"} {
		t.Run(path, func(t *testing.T) {
			f := newLegacyExecutionFixture(t)
			issue, err := f.svc.Queries.GetIssue(t.Context(), f.shared)
			if err != nil {
				t.Fatal(err)
			}
			// A stale or internal caller-supplied assignee cannot turn the
			// persisted collaboration issue into an ordinary execution source.
			issue.AssigneeType = pgtype.Text{String: "agent", Valid: true}
			issue.AssigneeID = f.agent
			comment := util.MustParseUUID(f.fx.Comment(t, util.UUIDToString(f.shared), "Private shared input"))
			published := 0
			f.svc.Bus.SubscribeAll(func(events.Event) { published++ })
			switch path {
			case "assignment":
				_, err = f.svc.EnqueueTaskForIssue(t.Context(), issue, comment)
			case "mention":
				_, err = f.svc.EnqueueTaskForMention(t.Context(), issue, f.agent, comment)
			case "thread":
				_, err = f.svc.EnqueueTaskForThreadParent(t.Context(), issue, f.agent, comment)
			case "squad":
				squad := util.MustParseUUID(f.fx.Squad(t, "Boundary squad", util.UUIDToString(f.agent)))
				_, err = f.svc.EnqueueTaskForSquadLeader(t.Context(), issue, f.agent, squad, comment)
			case "deferred":
				_, err = f.svc.EnqueueDeferredAssigneeFallback(t.Context(), issue, f.agent, pgtype.UUID{}, pgtype.UUID{}, comment, time.Now())
			case "channel":
				_, err = f.svc.EnqueueDeferredChannelIssueTask(t.Context(), issue, time.Now())
			}
			if !collaborationErrorCode(err, "unsupported_collaboration_action") {
				t.Errorf("ordinary %s must reject a collaboration source, got %v", path, err)
			}
			if got := f.fx.Count(t, "SELECT count(*) FROM agent_task_queue WHERE agent_id=$1", f.agent); got != 0 || published != 0 {
				t.Fatalf("rejected enqueue left %d tasks and published %d execution events", got, published)
			}
		})
	}
}

func TestLegacyExecutionPreparedHandoffCannotReuseCollaborationTask(t *testing.T) {
	for _, reuse := range []bool{false, true} {
		name := "insert"
		if reuse {
			name = "reuse"
		}
		t.Run(name, func(t *testing.T) {
			f := newLegacyExecutionFixture(t)
			actor := util.MustParseUUID(f.fx.UserID)
			attr := attribution.Result{UserID: actor, AccountableUserID: actor, Source: attribution.SourceDirectHuman}
			prepared, err := f.svc.PrepareIssueTaskEnqueue(t.Context(), f.agent, attr)
			if err != nil {
				t.Fatal(err)
			}
			if reuse {
				f.fx.Task(t, util.UUIDToString(f.agent), testutil.Cols{
					"issue_id": f.shared, "runtime_id": f.runtime,
					"originator_user_id": actor, "accountable_user_id": actor, "originator_source": "direct_human",
				})
			}
			issue, err := f.svc.Queries.GetIssue(t.Context(), f.shared)
			if err != nil {
				t.Fatal(err)
			}
			tx, err := f.fx.Pool.Begin(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(context.Background())
			task, reused, err := f.svc.EnqueuePreparedIssueTaskInTx(t.Context(), tx, issue, f.agent, pgtype.UUID{}, "", attr, prepared)
			if !collaborationErrorCode(err, "unsupported_collaboration_action") || task.ID.Valid || reused {
				t.Fatalf("prepared handoff admitted collaboration task: id=%v reused=%v err=%v", task.ID, reused, err)
			}
		})
	}
}

func TestLegacyExecutionSQLBoundaryAndOrdinaryChild(t *testing.T) {
	for _, kind := range []string{"collaboration", "ordinary", "ordinary_child"} {
		for _, path := range []string{"insert", "deferred", "claim", "runtime", "batch", "start", "sql_start", "retry", "promote", "reclaim", "link"} {
			t.Run(kind+"/"+path, func(t *testing.T) {
				f := newLegacyExecutionFixture(t)
				issue := f.shared
				if kind == "ordinary" {
					issue = f.ordinary
				} else if kind == "ordinary_child" {
					issue = f.child
				}
				allowed := kind != "collaboration"
				var err error
				var executed bool
				switch path {
				case "insert":
					_, err = f.svc.Queries.CreateAgentTask(t.Context(), db.CreateAgentTaskParams{AgentID: f.agent, RuntimeID: f.runtime, IssueID: issue})
					executed = err == nil
				case "deferred":
					_, err = f.svc.Queries.CreateDeferredAgentTask(t.Context(), db.CreateDeferredAgentTaskParams{AgentID: f.agent, RuntimeID: f.runtime, IssueID: issue, FireAt: pgtype.Timestamptz{Time: time.Now(), Valid: true}})
					executed = err == nil
				default:
					status := "queued"
					switch path {
					case "start", "sql_start", "reclaim":
						status = "dispatched"
					case "retry":
						status = "failed"
					case "promote":
						status = "deferred"
					}
					cols := testutil.Cols{"issue_id": issue, "runtime_id": f.runtime, "status": status, "fire_at": testutil.Raw("now() - interval '2 minutes'"), "dispatched_at": testutil.Raw("now() - interval '2 minutes'")}
					if path == "link" {
						cols["issue_id"] = nil
					}
					// Raw fixture rows model previously queued ordinary work. No
					// production ordinary-to-collaboration conversion is introduced.
					taskID := util.MustParseUUID(f.fx.Task(t, util.UUIDToString(f.agent), cols))
					switch path {
					case "claim":
						task, e := f.svc.ClaimTask(t.Context(), f.agent)
						err, executed = e, task != nil
					case "runtime":
						task, e := f.svc.ClaimTaskForRuntime(t.Context(), f.runtime)
						err, executed = e, task != nil
					case "batch":
						tasks, e := f.svc.ClaimTasksForRuntimes(t.Context(), []pgtype.UUID{f.runtime}, 1)
						err, executed = e, len(tasks) != 0
					case "start":
						task, e := f.svc.StartTask(t.Context(), taskID)
						err, executed = e, task != nil
					case "sql_start":
						_, err = f.svc.Queries.StartAgentTask(t.Context(), taskID)
						executed = err == nil
					case "retry":
						_, err = f.svc.Queries.CreateRetryTask(t.Context(), db.CreateRetryTaskParams{ID: taskID})
						executed = err == nil
					case "promote":
						_, err = f.svc.Queries.PromoteDeferredChannelIssueTask(t.Context(), taskID)
						executed = err == nil
					case "reclaim":
						_, err = f.svc.Queries.ReclaimStaleDispatchedTaskForRuntime(t.Context(), db.ReclaimStaleDispatchedTaskForRuntimeParams{RuntimeID: f.runtime, PrepareLeaseSecs: 60, ClaimRecoverySecs: 1, RuntimeStaleSecs: RuntimeClaimFreshnessSeconds})
						executed = err == nil
					case "link":
						err = f.svc.Queries.LinkTaskToIssue(t.Context(), db.LinkTaskToIssueParams{ID: taskID, IssueID: issue})
						executed = f.fx.Count(t, "SELECT count(*) FROM agent_task_queue WHERE id=$1 AND issue_id=$2", taskID, issue) == 1
					}
				}
				if executed != allowed {
					t.Fatalf("%s %s executed=%v allowed=%v err=%v", kind, path, executed, allowed, err)
				}
				if err != nil && !errors.Is(err, pgx.ErrNoRows) && !(path == "start" && !allowed && collaborationErrorCode(err, "unsupported_collaboration_action")) {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestLegacyExecutionFinalizeCannotMintCollaborationCredentials(t *testing.T) {
	for _, kind := range []string{"collaboration", "stale_source", "missing_runtime_snapshot", "stale_capture", "stale_coalesced", "ordinary", "ordinary_child", "late_ordinary_comment"} {
		t.Run(kind, func(t *testing.T) {
			f := newLegacyExecutionFixture(t)
			issue := f.shared
			if kind == "ordinary" || kind == "stale_source" || kind == "missing_runtime_snapshot" || kind == "stale_coalesced" || kind == "late_ordinary_comment" {
				issue = f.ordinary
			} else if kind == "ordinary_child" {
				issue = f.child
			} else if kind == "stale_capture" {
				issue = pgtype.UUID{}
			}
			cols := testutil.Cols{"issue_id": issue, "runtime_id": f.runtime, "status": "dispatched", "dispatched_at": testutil.Raw("now()")}
			taskID := util.MustParseUUID(f.fx.Task(t, util.UUIDToString(f.agent), cols))
			task, err := f.svc.Queries.GetAgentTask(t.Context(), taskID)
			if err != nil {
				t.Fatal(err)
			}
			if kind == "stale_source" {
				f.fx.Exec(t, "UPDATE agent_task_queue SET issue_id=$2 WHERE id=$1", task.ID, f.shared)
			} else if kind == "missing_runtime_snapshot" {
				f.fx.Exec(t, "UPDATE agent_task_queue SET issue_id=$2 WHERE id=$1", task.ID, f.shared)
				task.RuntimeID = pgtype.UUID{}
			} else if kind == "stale_capture" {
				comment := f.fx.Comment(t, util.UUIDToString(f.shared), "Captured private input")
				capture := f.fx.Insert(t, "issue_source_context", testutil.Cols{
					"id": uuid.NewString(), "workspace_id": f.fx.WorkspaceID, "source_issue_id": f.shared, "anchor_comment_id": comment,
					"captured_by_user_id": f.fx.UserID, "snapshot_version": 1, "snapshot": "{}", "capture_digest": "boundary-finalize", "state": "abandoned",
				})
				f.fx.Exec(t, "UPDATE agent_task_queue SET context=jsonb_build_object('type','quick_create','source_context_id',$2::text) WHERE id=$1", task.ID, capture)
			} else if kind == "stale_coalesced" || kind == "late_ordinary_comment" {
				source := f.shared
				if kind == "late_ordinary_comment" {
					source = f.ordinary
				}
				comment := util.MustParseUUID(f.fx.Comment(t, util.UUIDToString(source), "Late input"))
				f.fx.Exec(t, "UPDATE agent_task_queue SET coalesced_comment_ids=$2 WHERE id=$1", task.ID, []pgtype.UUID{comment})
			}
			hash := "legacy-boundary-" + uuid.NewString()
			expires := pgtype.Timestamptz{Time: time.Now().Add(time.Hour), Valid: true}
			_, err = f.svc.FinalizeTaskClaim(t.Context(), task, db.CreateTaskTokenParams{
				TokenHash: hash, TaskID: task.ID, AgentID: f.agent, WorkspaceID: util.MustParseUUID(f.fx.WorkspaceID), UserID: util.MustParseUUID(f.fx.UserID), ExpiresAt: expires,
			}, nil, false, db.CreateDaemonTokenParams{TokenHash: hash + "-daemon", WorkspaceID: util.MustParseUUID(f.fx.WorkspaceID), DaemonID: "boundary-fake-daemon", UserID: util.MustParseUUID(f.fx.UserID), ExpiresAt: expires})
			allowed := kind == "ordinary" || kind == "ordinary_child" || kind == "late_ordinary_comment"
			if (err == nil) != allowed {
				t.Errorf("claim finalization allowed=%v err=%v", allowed, err)
			}
			want := 0
			if allowed {
				want = 1
			}
			if got := f.fx.Count(t, "SELECT count(*) FROM task_token WHERE token_hash=$1", hash); got != want {
				t.Errorf("task credentials=%d want=%d", got, want)
			}
			if got := f.fx.Count(t, "SELECT count(*) FROM daemon_token WHERE token_hash=$1", hash+"-daemon"); got != want {
				t.Errorf("broker credentials=%d want=%d", got, want)
			}
		})
	}
}

type legacyExecutionLookupFailure struct {
	db.DBTX
	agentReads int
}

func (p *legacyExecutionLookupFailure) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if strings.Contains(sql, "-- name: IsCollaborationIssue") {
		return legacyExecutionErrorRow{}
	}
	if strings.Contains(sql, "-- name: GetAgent ") {
		p.agentReads++
	}
	return p.DBTX.QueryRow(ctx, sql, args...)
}

type legacyExecutionErrorRow struct{}

func (legacyExecutionErrorRow) Scan(...any) error { return errors.New("authority lookup failed") }

func TestLegacyExecutionUnavailableAuthorityStopsBeforeExecutionPreparation(t *testing.T) {
	f := newLegacyExecutionFixture(t)
	issue, err := f.svc.Queries.GetIssue(t.Context(), f.ordinary)
	if err != nil {
		t.Fatal(err)
	}
	probe := &legacyExecutionLookupFailure{DBTX: f.fx.Pool}
	f.svc.Queries = db.New(probe)
	_, err = f.svc.EnqueueTaskForMention(t.Context(), issue, f.agent, pgtype.UUID{})
	if !collaborationErrorCode(err, "collaboration_unavailable") || probe.agentReads != 0 {
		t.Fatalf("failed authority lookup reached execution preparation: reads=%d err=%v", probe.agentReads, err)
	}
	if f.fx.Count(t, "SELECT count(*) FROM agent_task_queue WHERE agent_id=$1", f.agent) != 0 {
		t.Fatal("failed authority lookup persisted execution")
	}
}

func TestLegacyExecutionCannotInheritCollaborationComments(t *testing.T) {
	for _, path := range []string{"service_trigger", "service_coalesced", "sql_trigger", "sql_coalesced", "claim"} {
		t.Run(path, func(t *testing.T) {
			f := newLegacyExecutionFixture(t)
			comment := util.MustParseUUID(f.fx.Comment(t, util.UUIDToString(f.shared), "Private shared comment"))
			issue, err := f.svc.Queries.GetIssue(t.Context(), f.child)
			if err != nil {
				t.Fatal(err)
			}
			switch path {
			case "service_trigger":
				_, err = f.svc.EnqueueTaskForMention(t.Context(), issue, f.agent, comment)
			case "service_coalesced":
				_, err = f.svc.enqueueMentionTaskWithCommentPlan(t.Context(), issue, f.agent, pgtype.UUID{}, []pgtype.UUID{comment}, false, pgtype.UUID{}, false, "", pgtype.UUID{}, pgtype.UUID{})
			case "sql_trigger":
				_, err = f.svc.Queries.CreateAgentTask(t.Context(), db.CreateAgentTaskParams{AgentID: f.agent, RuntimeID: f.runtime, IssueID: f.child, TriggerCommentID: comment})
			case "sql_coalesced":
				_, err = f.svc.Queries.CreateAgentTask(t.Context(), db.CreateAgentTaskParams{AgentID: f.agent, RuntimeID: f.runtime, IssueID: f.child, CoalescedCommentIds: []pgtype.UUID{comment}})
			case "claim":
				id := f.fx.Task(t, util.UUIDToString(f.agent), testutil.Cols{"issue_id": f.child, "runtime_id": f.runtime, "coalesced_comment_ids": []pgtype.UUID{comment}})
				task, err := f.svc.ClaimTask(t.Context(), f.agent)
				if err != nil || task != nil {
					t.Fatalf("legacy coalesced shared input became claimable: %v %v", task, err)
				}
				if f.fx.Count(t, "SELECT count(*) FROM agent_task_queue WHERE id=$1 AND status='queued'", id) != 1 {
					t.Fatal("refused legacy task changed state")
				}
				return
			}
			if err == nil {
				t.Fatal("ordinary execution accepted a collaboration comment as input")
			}
			if !errors.Is(err, pgx.ErrNoRows) && !collaborationErrorCode(err, "unsupported_collaboration_action") {
				t.Fatal(err)
			}
			if f.fx.Count(t, "SELECT count(*) FROM agent_task_queue WHERE agent_id=$1", f.agent) != 0 {
				t.Fatal("refused input persisted an ordinary task")
			}
		})
	}
}

func TestLegacyExecutionQuickCreateReferenceAndCapturedSource(t *testing.T) {
	for _, kind := range []string{"parent_reference", "collaboration_capture", "ordinary_child_capture"} {
		t.Run(kind, func(t *testing.T) {
			f := newLegacyExecutionFixture(t)
			payload := QuickCreateContext{Type: QuickCreateContextType, ParentIssueID: util.UUIDToString(f.shared)}
			if kind != "parent_reference" {
				source := f.shared
				if kind == "ordinary_child_capture" {
					source = f.child
				}
				comment := f.fx.Comment(t, util.UUIDToString(source), "Captured input")
				payload.SourceContextID = f.fx.Insert(t, "issue_source_context", testutil.Cols{
					"id": uuid.NewString(), "workspace_id": f.fx.WorkspaceID, "source_issue_id": source, "anchor_comment_id": comment,
					"captured_by_user_id": f.fx.UserID, "snapshot_version": 1, "snapshot": "{}", "capture_digest": "boundary-test", "state": "abandoned",
				})
			}
			raw, err := json.Marshal(payload)
			if err != nil {
				t.Fatal(err)
			}
			_, err = f.svc.Queries.CreateQuickCreateTask(t.Context(), db.CreateQuickCreateTaskParams{AgentID: f.agent, RuntimeID: f.runtime, Context: raw})
			allowed := kind != "collaboration_capture"
			if (err == nil) != allowed {
				t.Fatalf("quick-create reference=%s allowed=%v err=%v", kind, allowed, err)
			}
			if err != nil && !errors.Is(err, pgx.ErrNoRows) {
				t.Fatal(err)
			}
		})
	}
}

func TestLegacyExecutionUnrelatedChatAndQuickCreateStillStart(t *testing.T) {
	for _, kind := range []string{"chat", "quick_create"} {
		t.Run(kind, func(t *testing.T) {
			f := newLegacyExecutionFixture(t)
			actor := util.MustParseUUID(f.fx.UserID)
			var queued db.AgentTaskQueue
			var err error
			if kind == "chat" {
				chat := util.MustParseUUID(f.fx.ChatSession(t, util.UUIDToString(f.agent)))
				queued, err = f.svc.Queries.CreateChatTask(t.Context(), db.CreateChatTaskParams{
					AgentID: f.agent, RuntimeID: f.runtime, ChatSessionID: chat, InitiatorUserID: actor, OriginatorUserID: actor, AccountableUserID: actor,
				})
			} else {
				queued, err = f.svc.EnqueueQuickCreateTask(t.Context(), util.MustParseUUID(f.fx.WorkspaceID), actor, f.agent, pgtype.UUID{}, "New ordinary child", "none", "", pgtype.UUID{}, f.shared, nil)
			}
			if err != nil {
				t.Fatal(err)
			}
			task, err := f.svc.ClaimTask(t.Context(), f.agent)
			if err != nil || task == nil || task.ID != queued.ID {
				t.Fatalf("ordinary %s could not claim its task: %v %v", kind, task, err)
			}
			hash := "ordinary-boundary-control-" + uuid.NewString()
			_, err = f.svc.FinalizeTaskClaim(t.Context(), *task, db.CreateTaskTokenParams{
				TokenHash: hash, TaskID: task.ID, AgentID: f.agent, WorkspaceID: util.MustParseUUID(f.fx.WorkspaceID), UserID: actor,
				ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(time.Hour), Valid: true},
			}, nil, false)
			if err != nil {
				t.Fatalf("ordinary %s claim could not finalize: %v", kind, err)
			}
			started, err := f.svc.StartTask(t.Context(), task.ID)
			if err != nil || started == nil || started.Status != "running" || started.IssueID.Valid {
				t.Fatalf("ordinary %s could not start independently of collaboration: %v %v", kind, started, err)
			}
			if f.fx.Count(t, "SELECT count(*) FROM task_token WHERE token_hash=$1", hash) != 1 {
				t.Fatal("ordinary claim did not retain its execution credential")
			}
		})
	}
}
