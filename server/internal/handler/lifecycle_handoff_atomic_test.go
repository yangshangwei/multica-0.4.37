package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/featureflags"
	"github.com/multica-ai/multica/server/internal/runtimeapps"
	"github.com/multica-ai/multica/server/internal/service"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/dbid"
	"github.com/multica-ai/multica/server/pkg/featureflag"
	"github.com/multica-ai/multica/server/pkg/protocol"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/internal/testutil"
)

// Regression coverage for atomic lifecycle persistence and dispatch.
func lifecycleAtomicSource(t *testing.T, title string) string {
	t.Helper()
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	source := dbfx.Issue(t, title)
	dbfx.Exec(t, `UPDATE workspace SET issue_counter = GREATEST(issue_counter, (SELECT max(number) FROM issue WHERE workspace_id = $1)) WHERE id = $1`, testWorkspaceID)
	dbfx.Cleanup(t, `DELETE FROM issue WHERE parent_issue_id = $1`, source)
	dbfx.Cleanup(t, `DELETE FROM comment WHERE issue_id = $1 OR issue_id IN (SELECT id FROM issue WHERE parent_issue_id = $1)`, source)
	dbfx.Cleanup(t, `DELETE FROM agent_task_queue WHERE issue_id IN (SELECT id FROM issue WHERE parent_issue_id = $1)`, source)
	return source
}

func lifecycleAtomicCall(t *testing.T, source string, body any, headers ...string) *testutil.Response {
	t.Helper()
	r := withURLParam(newRequest(http.MethodPost, "/api/issues/"+source+"/lifecycle-handoffs", body), "id", source)
	testutil.WithHeaders(r, headers...)
	return testutil.Call(t, testHandler.CreateLifecycleHandoff, r)
}

func TestLifecycleAtomicNewFollowUpPreservesNote(t *testing.T) {
	source := lifecycleAtomicSource(t, "review handoff note source")
	agent := dbfx.Agent(t, "review handoff note agent", testRuntimeID)
	const note = "Reproduce TestReviewRegression before changing implementation."
	var result lifecycleHandoffResponse
	lifecycleAtomicCall(t, source, map[string]any{
		"kind": "rca", "route": "maintenance", "cause_state": "unknown",
		"follow_up_title": "review handoff note child", "assignee_type": "agent", "assignee_id": agent,
		"handoff_note": note,
	}).Want(http.StatusCreated).JSON(&result)
	var stored string
	dbfx.QueryRow(t, `SELECT COALESCE(handoff_note, '') FROM agent_task_queue WHERE issue_id = $1 AND agent_id = $2 ORDER BY created_at DESC LIMIT 1`, result.FollowUpIssueID, agent).Scan(&stored)
	t.Logf("HTTP=201 queued_task=%s submitted_note=%q stored_note=%q", result.QueuedTaskID, note, stored)
	if stored != note {
		t.Errorf("new follow-up task lost handoff_note")
	}
}

func TestLifecycleAtomicAgentOriginatorPreserved(t *testing.T) {
	source := lifecycleAtomicSource(t, "review originator source")
	creator := dbfx.Agent(t, "review originator creator", testRuntimeID)
	target := dbfx.Agent(t, "review originator target", testRuntimeID)
	acting := dbfx.Task(t, creator, testutil.Cols{"runtime_id": testRuntimeID, "status": "running", "originator_user_id": testUserID, "accountable_user_id": testUserID})
	var result lifecycleHandoffResponse
	lifecycleAtomicCall(t, source, map[string]any{
		"kind": "rca", "route": "maintenance", "cause_state": "unknown",
		"follow_up_title": "review originator child", "assignee_type": "agent", "assignee_id": target,
	}, "X-Agent-ID", creator, "X-Task-ID", acting).Want(http.StatusCreated).JSON(&result)
	var creatorType, originType, originID string
	dbfx.QueryRow(t, `SELECT creator_type, COALESCE(origin_type, ''), COALESCE(origin_id::text, '') FROM issue WHERE id = $1`, result.FollowUpIssueID).Scan(&creatorType, &originType, &originID)
	var originator string
	dbfx.QueryRow(t, `SELECT COALESCE(originator_user_id::text, '') FROM agent_task_queue WHERE issue_id = $1 AND agent_id = $2 ORDER BY created_at DESC LIMIT 1`, result.FollowUpIssueID, target).Scan(&originator)
	t.Logf("HTTP=201 creator_type=%s origin_type=%q origin_id=%q task_originator=%q expected_human=%s acting_task=%s", creatorType, originType, originID, originator, testUserID, acting)
	if originType != "agent_create" || originID != acting {
		t.Errorf("agent-created lifecycle follow-up lost acting task provenance")
	}
	if originator != testUserID {
		t.Errorf("queued lifecycle follow-up task lost human originator")
	}
}

func TestLifecycleAtomicMetadataBudgetIsAtomic(t *testing.T) {
	source := lifecycleAtomicSource(t, "review metadata budget source")
	body := map[string]any{"kind": "governance", "governance": map[string]any{
		"capability": "threat-modeling", "signals": []map[string]any{
			{"name": "trust-boundaries", "status": "pass", "detail": strings.Repeat("x", 4500)},
			{"name": "abuse-paths", "status": "pass"},
			{"name": "mitigations", "status": "pass"},
		},
	}}
	response := lifecycleAtomicCall(t, source, body)
	var latest, history bool
	var size int
	dbfx.QueryRow(t, `SELECT metadata ? 'lifecycle_handoff', metadata ? 'lifecycle_handoff_history', pg_column_size(metadata) FROM issue WHERE id = $1`, source).Scan(&latest, &history, &size)
	t.Logf("HTTP=%d body=%s latest=%t history=%t persisted_size=%d", response.Code, response.Body.String(), latest, history, size)
	response.Want(http.StatusBadRequest)
	if latest || history {
		t.Errorf("oversized whole metadata must leave source unchanged")
	}
	if latest != history {
		t.Errorf("source metadata partially persisted after failure")
	}
}

func TestLifecycleAtomicOversizeHasNoSideEffects(t *testing.T) {
	source := lifecycleAtomicSource(t, "review oversized handoff source")
	agent := dbfx.Agent(t, "review oversized target", testRuntimeID)
	response := lifecycleAtomicCall(t, source, map[string]any{
		"kind": "rca", "route": "bug-fix", "cause_state": "known", "reason": strings.Repeat("r", 8000),
		"follow_up_title": "review oversized handoff child", "assignee_type": "agent", "assignee_id": agent,
	})
	children := dbfx.Count(t, `SELECT count(*) FROM issue WHERE parent_issue_id = $1`, source)
	tasks := dbfx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id IN (SELECT id FROM issue WHERE parent_issue_id = $1)`, source)
	t.Logf("HTTP=%d body=%s persisted_children=%d queued_tasks=%d", response.Code, response.Body.String(), children, tasks)
	response.Want(http.StatusBadRequest)
	if children != 0 || tasks != 0 {
		t.Errorf("rejected lifecycle evidence created a child and enqueued work")
	}
}

func TestLifecycleAtomicFailedCorrectnessCannotPass(t *testing.T) {
	source := lifecycleAtomicSource(t, "review failed correctness source")
	cases := make([]map[string]any, 0, 6)
	for _, category := range []string{"correctness", "tool-failure", "safety", "cost", "latency", "drift"} {
		result := "pass"
		if category == "safety" {
			result = "blocked"
		}
		if category == "correctness" {
			result = "fail"
		}
		cases = append(cases, map[string]any{"category": category, "trace": []string{"candidate returns wrong answer"}, "stop_reason": "evaluation-complete", "result": result})
	}
	var result lifecycleHandoffResponse
	lifecycleAtomicCall(t, source, map[string]any{"kind": "agent-evaluation", "agent_evaluation": map[string]any{
		"artifact_digest": "sha256:review-candidate", "baseline_version": "agent-1", "candidate_version": "agent-2", "skill_version": "skill-1", "mcp_version": "mcp-1", "cases": cases,
	}}).Want(http.StatusCreated).JSON(&result)
	t.Logf("HTTP=201 correctness=fail decision=%s", result.Decision)
	if result.Decision == "pass" {
		t.Errorf("failed correctness case still produces successful gate")
	}
}

func lifecycleAtomicSnapshot(t *testing.T, source string) string {
	t.Helper()
	var snapshot string
	dbfx.QueryRow(t, `SELECT jsonb_build_object(
	  'source', to_jsonb(i), 'counter', w.issue_counter,
	  'children', (SELECT jsonb_agg(to_jsonb(c) ORDER BY c.id) FROM issue c WHERE c.parent_issue_id = i.id),
	  'comments', (SELECT jsonb_agg(to_jsonb(c) ORDER BY c.id) FROM comment c WHERE c.issue_id = i.id OR c.issue_id IN (SELECT id FROM issue WHERE parent_issue_id = i.id)),
	  'tasks', (SELECT jsonb_agg(to_jsonb(q) ORDER BY q.id) FROM agent_task_queue q WHERE q.issue_id = i.id OR q.issue_id IN (SELECT id FROM issue WHERE parent_issue_id = i.id))
	)::text FROM issue i JOIN workspace w ON w.id = i.workspace_id WHERE i.id = $1`, source).Scan(&snapshot)
	return snapshot
}

func lifecycleAtomicAssignedBody(assigneeType, assignee string) map[string]any {
	return map[string]any{"kind": "rca", "route": "maintenance", "cause_state": "unknown",
		"follow_up_title": "atomic diagnostic", "assignee_type": assigneeType, "assignee_id": assignee, "handoff_note": "Reproduce before changing code."}
}

func TestLifecycleAtomicCompatibleQueueReuse(t *testing.T) {
	for _, kind := range []string{"agent", "squad"} {
		t.Run(kind, func(t *testing.T) {
			source := lifecycleAtomicSource(t, "atomic repeated "+kind)
			agent := dbfx.Agent(t, "atomic repeated target", testRuntimeID)
			assignee := agent
			if kind == "squad" {
				assignee = dbfx.Squad(t, "atomic repeated squad", agent)
			}
			body := lifecycleAtomicAssignedBody(kind, assignee)
			var first, second lifecycleHandoffResponse
			lifecycleAtomicCall(t, source, body).Want(http.StatusCreated).JSON(&first)
			lifecycleAtomicCall(t, source, body).Want(http.StatusCreated).JSON(&second)
			if first.QueuedTaskID == "" || second.QueuedTaskID != first.QueuedTaskID || second.FollowUpIssueID != first.FollowUpIssueID || !second.FollowUpReused {
				t.Fatalf("non-idempotent queue reuse: first=%+v second=%+v", first, second)
			}
			if n := dbfx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id = $1`, first.FollowUpIssueID); n != 1 {
				t.Fatalf("queue count=%d, want1", n)
			}
			before := lifecycleAtomicSnapshot(t, source)
			body["handoff_note"] = "Different instructions must not be silently dropped."
			lifecycleAtomicCall(t, source, body).Want(http.StatusConflict)
			if after := lifecycleAtomicSnapshot(t, source); before != after {
				t.Fatal("incompatible note changed committed handoff state")
			}
		})
	}
}

type lifecycleFaultStarter struct {
	base   txStarter
	query  string
	skip   int
	commit bool
}

func (s lifecycleFaultStarter) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := s.base.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &lifecycleFaultTx{Tx: tx, query: s.query, skip: s.skip, failCommit: s.commit}, nil
}

type lifecycleFaultTx struct {
	pgx.Tx
	query      string
	skip       int
	failCommit bool
}

func (tx *lifecycleFaultTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if tx.query != "" && strings.Contains(sql, "-- name: "+tx.query+" ") {
		if tx.skip == 0 {
			return lifecycleFaultRow{}
		}
		tx.skip--
	}
	return tx.Tx.QueryRow(ctx, sql, args...)
}
func (tx *lifecycleFaultTx) Begin(ctx context.Context) (pgx.Tx, error) {
	sp, err := tx.Tx.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &lifecycleFaultTx{Tx: sp, query: tx.query}, nil
}
func (tx *lifecycleFaultTx) Commit(ctx context.Context) error {
	if tx.failCommit {
		return errors.New("injected confirmed commit failure")
	}
	return tx.Tx.Commit(ctx)
}

type lifecycleFaultRow struct{}

func (lifecycleFaultRow) Scan(...any) error { return errors.New("injected lifecycle SQL failure") }

type lifecycleWakeups struct{ calls atomic.Int32 }

func (w *lifecycleWakeups) NotifyTaskAvailable(string, string) { w.calls.Add(1) }

func lifecycleIsolatedHandler() (*Handler, *lifecycleWakeups, *atomic.Int32) {
	h := *testHandler
	bus := events.New()
	count := &atomic.Int32{}
	bus.SubscribeAll(func(events.Event) { count.Add(1) })
	wakeups := &lifecycleWakeups{}
	h.Bus = bus
	h.TaskService = &service.TaskService{Queries: h.Queries, TxStarter: testHandler.TaskService.TxStarter, Bus: bus, Wakeup: wakeups}
	issueService := *testHandler.IssueService
	issueService.Bus = bus
	issueService.TaskService = h.TaskService
	h.IssueService = &issueService
	return &h, wakeups, count
}

func TestLifecycleAtomicRollbackOnEveryRequiredWrite(t *testing.T) {
	for _, stage := range []string{"child-metadata", "source-metadata", "CreateComment", "CreateAgentTask", "commit"} {
		t.Run(stage, func(t *testing.T) {
			source := lifecycleAtomicSource(t, "atomic fault "+stage)
			agent := dbfx.Agent(t, "atomic fault target", testRuntimeID)
			h, wakeups, eventCount := lifecycleIsolatedHandler()
			query, skip := stage, 0
			if stage == "child-metadata" || stage == "source-metadata" {
				query = "SetLifecycleMetadata"
			}
			if stage == "source-metadata" {
				skip = 1
			}
			h.TxStarter = lifecycleFaultStarter{base: h.TxStarter, query: query, skip: skip, commit: stage == "commit"}
			before := lifecycleAtomicSnapshot(t, source)
			body := lifecycleAtomicAssignedBody("agent", agent)
			body["conclusion"] = "unknown"
			req := withURLParam(newRequest(http.MethodPost, "/api/issues/"+source+"/lifecycle-handoffs", body), "id", source)
			testutil.Call(t, h.CreateLifecycleHandoff, req).Want(http.StatusInternalServerError)
			if after := lifecycleAtomicSnapshot(t, source); after != before {
				t.Fatalf("%s failure left partial DB state", stage)
			}
			if eventCount.Load() != 0 || wakeups.calls.Load() != 0 {
				t.Fatalf("%s failure published events=%d wakeups=%d", stage, eventCount.Load(), wakeups.calls.Load())
			}
		})
	}
}

func TestLifecycleAtomicConcurrentHandoffs(t *testing.T) {
	source := lifecycleAtomicSource(t, "atomic concurrent source")
	agent := dbfx.Agent(t, "atomic concurrent target", testRuntimeID)
	body := lifecycleAtomicAssignedBody("agent", agent)
	const count = 4
	start := make(chan struct{})
	responses := make(chan *testutil.Response, count)
	for i := 0; i < count; i++ {
		go func() { <-start; responses <- lifecycleAtomicCall(t, source, body) }()
	}
	close(start)
	var taskID string
	for i := 0; i < count; i++ {
		var result lifecycleHandoffResponse
		(<-responses).Want(http.StatusCreated).JSON(&result)
		if taskID != "" && result.QueuedTaskID != taskID {
			t.Fatalf("concurrent callers got different task IDs")
		}
		taskID = result.QueuedTaskID
	}
	var history int
	dbfx.QueryRow(t, `SELECT jsonb_array_length(metadata->'lifecycle_handoff_history') FROM issue WHERE id = $1`, source).Scan(&history)
	if history != count {
		t.Fatalf("history=%d, want%d", history, count)
	}
	if children := dbfx.Count(t, `SELECT count(*) FROM issue WHERE parent_issue_id = $1`, source); children != 1 {
		t.Fatalf("children=%d, want1", children)
	}
}

func TestLifecycleAtomicBudgetPreservesWholeState(t *testing.T) {
	for _, size := range []int{400, 4500} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			source := lifecycleAtomicSource(t, "atomic metadata budget")
			dbfx.Exec(t, `UPDATE issue SET metadata = '{"user_key":"保留"}'::jsonb WHERE id = $1`, source)
			before := lifecycleAtomicSnapshot(t, source)
			body := map[string]any{"kind": "governance", "governance": map[string]any{"capability": "threat-modeling", "signals": []map[string]any{
				{"name": "trust-boundaries", "status": "pass", "detail": strings.Repeat("x", size)}, {"name": "abuse-paths", "status": "pass"}, {"name": "mitigations", "status": "pass"},
			}}}
			response := lifecycleAtomicCall(t, source, body)
			if size == 4500 {
				response.Want(http.StatusBadRequest)
				if after := lifecycleAtomicSnapshot(t, source); after != before {
					t.Fatal("over-budget rejection changed DB snapshot")
				}
			} else {
				response.Want(http.StatusCreated)
				var saved string
				dbfx.QueryRow(t, `SELECT metadata->>'user_key' FROM issue WHERE id = $1`, source).Scan(&saved)
				if saved != "保留" {
					t.Fatalf("user metadata lost: %q", saved)
				}
			}
		})
	}
}

type lifecycleHookStarter struct {
	base  txStarter
	after func(string, error)
}

func (s lifecycleHookStarter) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := s.base.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &lifecycleHookTx{Tx: tx, after: s.after}, nil
}

type lifecycleHookTx struct {
	pgx.Tx
	after func(string, error)
}

func (tx *lifecycleHookTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return lifecycleHookRow{Row: tx.Tx.QueryRow(ctx, sql, args...), sql: sql, after: tx.after}
}

type lifecycleHookRow struct {
	pgx.Row
	sql   string
	after func(string, error)
}

func (r lifecycleHookRow) Scan(values ...any) error {
	err := r.Row.Scan(values...)
	r.after(r.sql, err)
	return err
}

func TestLifecycleAtomicConcurrentMetadataWriter(t *testing.T) {
	source := lifecycleAtomicSource(t, "atomic metadata writer")
	h, _, _ := lifecycleIsolatedHandler()
	locked, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	h.TxStarter = lifecycleHookStarter{base: h.TxStarter, after: func(sql string, err error) {
		if strings.Contains(sql, "-- name: LockLifecycleIssue ") && err == nil {
			once.Do(func() { close(locked); <-release })
		}
	}}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	response := make(chan *testutil.Response, 1)
	go func() {
		req := withURLParam(newRequest(http.MethodPost, "/api/issues/"+source+"/lifecycle-handoffs", map[string]any{"kind": "rollout", "rollout": map[string]any{}}), "id", source)
		response <- testutil.Call(t, h.CreateLifecycleHandoff, req)
	}()
	select {
	case <-locked:
	case <-ctx.Done():
		t.Fatal("handoff did not acquire source lock")
	}
	writerStarted := make(chan struct{})
	writerDone := make(chan error, 1)
	go func() {
		close(writerStarted)
		_, err := testHandler.Queries.SetIssueMetadataKey(ctx, db.SetIssueMetadataKeyParams{ID: parseUUID(source), WorkspaceID: parseUUID(testWorkspaceID), Key: "concurrent_integer", Value: []byte("9007199254740993")})
		writerDone <- err
	}()
	<-writerStarted
	close(release)
	(<-response).Want(http.StatusCreated)
	if err := <-writerDone; err != nil {
		t.Fatal(err)
	}
	// A later whole-object lifecycle write must preserve the exact integer too.
	lifecycleAtomicCall(t, source, map[string]any{"kind": "rollout", "rollout": map[string]any{}}).Want(http.StatusCreated)
	var integer string
	var history int
	dbfx.QueryRow(t, `SELECT metadata->>'concurrent_integer', jsonb_array_length(metadata->'lifecycle_handoff_history') FROM issue WHERE id=$1`, source).Scan(&integer, &history)
	if integer != "9007199254740993" || history != 2 {
		t.Fatalf("concurrent metadata lost: %s, history=%d", integer, history)
	}
}

func TestLifecycleAtomicSourceContextCreateLockOrder(t *testing.T) {
	source := lifecycleAtomicSource(t, "atomic source-context source")
	anchor := dbfx.Comment(t, source, "captured diagnostic context")
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	build, err := service.BuildSourceContext(ctx, testHandler.Queries, parseUUID(testWorkspaceID), parseUUID(anchor))
	if err != nil {
		t.Fatal(err)
	}
	capture, err := service.PrepareSourceContextCapture(build, dbid.NewV7(), parseUUID(testWorkspaceID), parseUUID(testUserID), time.Now(), nil)
	if err != nil {
		t.Fatal(err)
	}
	dbfx.Cleanup(t, `DELETE FROM issue_source_context WHERE source_issue_id=$1`, source)
	locked, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	ordinary := *testHandler.IssueService
	ordinary.TxStarter = lifecycleHookStarter{base: ordinary.TxStarter, after: func(sql string, err error) {
		if strings.Contains(sql, "-- name: LockIssueForDescriptionUpdate ") && err == nil {
			once.Do(func() { close(locked); <-release })
		}
	}}
	ordinaryDone := make(chan error, 1)
	go func() {
		_, createErr := ordinary.Create(ctx, service.IssueCreateParams{WorkspaceID: parseUUID(testWorkspaceID), ParentIssueID: parseUUID(source),
			Title: "shared diagnostic", Status: "todo", Priority: "none", CreatorType: "member", CreatorID: parseUUID(testUserID), SourceContext: &capture}, service.IssueCreateOpts{})
		ordinaryDone <- createErr
	}()
	select {
	case <-locked:
	case <-ctx.Done():
		t.Fatal("ordinary create did not acquire source lock")
	}
	h, _, _ := lifecycleIsolatedHandler()
	var releaseOnce sync.Once
	h.TxStarter = lifecycleHookStarter{base: h.TxStarter, after: func(sql string, lockErr error) {
		if strings.Contains(sql, "-- name: LockLifecycleIssue ") && lockErr != nil {
			releaseOnce.Do(func() { close(release) })
		}
	}}
	req := withURLParam(newRequest(http.MethodPost, "/api/issues/"+source+"/lifecycle-handoffs", map[string]any{"kind": "rca", "route": "maintenance", "cause_state": "unknown", "follow_up_title": "shared diagnostic"}), "id", source)
	response := testutil.Call(t, h.CreateLifecycleHandoff, req)
	releaseOnce.Do(func() { close(release) })
	if err := <-ordinaryDone; err != nil {
		t.Fatalf("SourceContext create failed: %v", err)
	}
	response.Want(http.StatusCreated)
	if count := dbfx.Count(t, `SELECT count(*) FROM issue WHERE parent_issue_id=$1`, source); count != 1 {
		t.Fatalf("duplicate children=%d", count)
	}
}

type lifecyclePanicWakeup struct{}

func (lifecyclePanicWakeup) NotifyTaskAvailable(string, string) { panic("test unavailable wakeup") }

func TestLifecycleAtomicDurableBeforeNotificationAndPollRecovery(t *testing.T) {
	source := lifecycleAtomicSource(t, "atomic notify source")
	runtime := dbfx.Runtime(t, "atomic notify runtime")
	agent := dbfx.Agent(t, "atomic notify agent", runtime)
	h, _, _ := lifecycleIsolatedHandler()
	h.TaskService.Wakeup = lifecyclePanicWakeup{}
	var queued atomic.Int32
	h.Bus.Subscribe(protocol.EventTaskQueued, func(event events.Event) {
		queued.Add(1)
		if count := dbfx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE agent_id=$1`, agent); count != 1 {
			t.Errorf("event preceded durable task: count=%d", count)
		}
		if count := dbfx.Count(t, `SELECT count(*) FROM comment WHERE issue_id=$1`, source); count != 1 {
			t.Errorf("event preceded durable audit: count=%d", count)
		}
	})
	body := lifecycleAtomicAssignedBody("agent", agent)
	var first, second lifecycleHandoffResponse
	for _, out := range []*lifecycleHandoffResponse{&first, &second} {
		req := withURLParam(newRequest(http.MethodPost, "/api/issues/"+source+"/lifecycle-handoffs", body), "id", source)
		testutil.Call(t, h.CreateLifecycleHandoff, req).Want(http.StatusCreated).JSON(out)
	}
	if queued.Load() != 1 || first.QueuedTaskID != second.QueuedTaskID {
		t.Fatal("compatible retry notified or queued twice")
	}
	candidates, err := h.Queries.ListQueuedClaimCandidatesByRuntime(context.Background(), parseUUID(runtime))
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 1 || uuidToString(candidates[0].ID) != first.QueuedTaskID {
		t.Fatalf("normal daemon poll failed to discover durable task: %+v", candidates)
	}
}

func TestLifecycleAtomicRejectsUntrustedTaskAndChangedAttribution(t *testing.T) {
	for _, state := range []string{"completed", "wrong-agent"} {
		t.Run(state, func(t *testing.T) {
			source := lifecycleAtomicSource(t, "atomic bad origin")
			actor := dbfx.Agent(t, "atomic bad origin actor", testRuntimeID)
			target := dbfx.Agent(t, "atomic bad origin target", testRuntimeID)
			status := "running"
			owner := actor
			if state == "completed" {
				status = "completed"
			} else {
				owner = target
			}
			task := dbfx.Task(t, owner, testutil.Cols{"runtime_id": testRuntimeID, "status": status, "originator_user_id": testUserID, "accountable_user_id": testUserID})
			before := lifecycleAtomicSnapshot(t, source)
			lifecycleAtomicCall(t, source, lifecycleAtomicAssignedBody("agent", target), "X-Agent-ID", actor, "X-Task-ID", task, "X-Actor-Source", "task_token").Want(http.StatusForbidden)
			if lifecycleAtomicSnapshot(t, source) != before {
				t.Fatal("untrusted task changed DB")
			}
		})
	}
	source := lifecycleAtomicSource(t, "atomic changed origin")
	actor := dbfx.Agent(t, "atomic changed actor", testRuntimeID)
	target := dbfx.Agent(t, "atomic changed target", testRuntimeID)
	body := lifecycleAtomicAssignedBody("agent", target)
	firstTask := dbfx.Task(t, actor, testutil.Cols{"runtime_id": testRuntimeID, "status": "running", "originator_user_id": testUserID, "accountable_user_id": testUserID})
	secondTask := dbfx.Task(t, actor, testutil.Cols{"runtime_id": testRuntimeID, "status": "running", "originator_user_id": testUserID, "accountable_user_id": testUserID})
	lifecycleAtomicCall(t, source, body, "X-Agent-ID", actor, "X-Task-ID", firstTask).Want(http.StatusCreated)
	before := lifecycleAtomicSnapshot(t, source)
	lifecycleAtomicCall(t, source, body, "X-Agent-ID", actor, "X-Task-ID", secondTask).Want(http.StatusConflict)
	if lifecycleAtomicSnapshot(t, source) != before {
		t.Fatal("new delegation overwrote prior task attribution")
	}
}

type lifecycleOverlayBuilder struct {
	build func(context.Context, pgtype.UUID, db.Agent) (runtimeapps.MCPOverlayResult, error)
}

func (b lifecycleOverlayBuilder) BuildTaskOverlay(ctx context.Context, id pgtype.UUID, a db.Agent) (runtimeapps.MCPOverlayResult, error) {
	return b.build(ctx, id, a)
}

func TestLifecycleAtomicPreparesOverlayOutsideLocks(t *testing.T) {
	for _, change := range []bool{false, true} {
		t.Run(fmt.Sprint(change), func(t *testing.T) {
			source := lifecycleAtomicSource(t, "atomic overlay source")
			agent := dbfx.Agent(t, "atomic overlay agent", testRuntimeID)
			h, _, _ := lifecycleIsolatedHandler()
			flags := featureflag.NewStaticProvider()
			flags.Set(featureflags.ComposioMCPApps, featureflag.Rule{Default: true})
			h.TaskService.FeatureFlags = featureflag.NewService(flags)
			h.TaskService.Composio = lifecycleOverlayBuilder{build: func(ctx context.Context, _ pgtype.UUID, a db.Agent) (runtimeapps.MCPOverlayResult, error) {
				tx, err := testPool.Begin(ctx)
				if err != nil {
					return runtimeapps.MCPOverlayResult{}, err
				}
				defer tx.Rollback(ctx)
				for _, statement := range []string{`SELECT id FROM workspace WHERE id=$1 FOR UPDATE NOWAIT`, `SELECT id FROM issue WHERE id=$1 FOR UPDATE NOWAIT`, `SELECT id FROM agent WHERE id=$1 FOR UPDATE NOWAIT`} {
					id := testWorkspaceID
					if strings.Contains(statement, "FROM issue") {
						id = source
					}
					if strings.Contains(statement, "FROM agent") {
						id = agent
					}
					if _, err = tx.Exec(ctx, statement, id); err != nil {
						t.Errorf("overlay built while DB locks held: %v", err)
						return runtimeapps.MCPOverlayResult{}, err
					}
				}
				if change {
					if _, err = tx.Exec(ctx, `UPDATE agent SET composio_toolkit_allowlist=ARRAY['changed'] WHERE id=$1`, agent); err != nil {
						return runtimeapps.MCPOverlayResult{}, err
					}
				}
				if err = tx.Commit(ctx); err != nil {
					return runtimeapps.MCPOverlayResult{}, err
				}
				return runtimeapps.MCPOverlayResult{MCPOverlay: json.RawMessage(`{"test":{"url":"https://example.invalid"}}`)}, nil
			}}
			before := lifecycleAtomicSnapshot(t, source)
			req := withURLParam(newRequest(http.MethodPost, "/api/issues/"+source+"/lifecycle-handoffs", lifecycleAtomicAssignedBody("agent", agent)), "id", source)
			response := testutil.Call(t, h.CreateLifecycleHandoff, req)
			if change {
				response.Want(http.StatusConflict)
				if lifecycleAtomicSnapshot(t, source) != before {
					t.Fatal("stale overlay context committed handoff")
				}
				return
			}
			var out lifecycleHandoffResponse
			response.Want(http.StatusCreated).JSON(&out)
			var mounted bool
			dbfx.QueryRow(t, `SELECT runtime_mcp_overlay ? 'test' FROM agent_task_queue WHERE id=$1`, out.QueuedTaskID).Scan(&mounted)
			if !mounted {
				t.Fatal("task became visible without prepared overlay")
			}
		})
	}
}

func TestLifecycleAtomicPendingInsertRaceUsesSavepoint(t *testing.T) {
	for _, compatible := range []bool{true, false} {
		t.Run(fmt.Sprint(compatible), func(t *testing.T) {
			source := lifecycleAtomicSource(t, "atomic unique race")
			agent := dbfx.Agent(t, "atomic unique target", testRuntimeID)
			child := dbfx.Issue(t, "atomic existing target", testutil.Cols{"parent_issue_id": source, "assignee_type": "agent", "assignee_id": agent})
			h, wakeups, _ := lifecycleIsolatedHandler()
			var taskEvents atomic.Int32
			h.Bus.Subscribe(protocol.EventTaskQueued, func(events.Event) { taskEvents.Add(1) })
			var winner db.AgentTaskQueue
			var winnerErr error
			var winnerSnapshot string
			var once sync.Once
			h.TxStarter = lifecycleHookStarter{base: h.TxStarter, after: func(sql string, err error) {
				if strings.Contains(sql, "-- name: LockPendingLifecycleTask ") && errors.Is(err, pgx.ErrNoRows) {
					once.Do(func() {
						ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
						defer cancel()
						issue, loadErr := h.Queries.GetIssueInWorkspace(ctx, db.GetIssueInWorkspaceParams{ID: parseUUID(child), WorkspaceID: parseUUID(testWorkspaceID)})
						if loadErr != nil {
							winnerErr = loadErr
							return
						}
						competitor := &service.TaskService{Queries: h.Queries, Bus: h.Bus, Wakeup: wakeups}
						note := "same instructions"
						if !compatible {
							note = "incompatible winner instructions"
						}
						winner, winnerErr = competitor.EnqueueTaskForIssueWithHandoff(ctx, issue, note, parseUUID(testUserID))
						winnerSnapshot = lifecycleAtomicSnapshot(t, source)
					})
				}
			}}
			body := map[string]any{"kind": "rca", "route": "maintenance", "cause_state": "unknown", "follow_up_issue_id": child, "handoff_note": "same instructions"}
			req := withURLParam(newRequest(http.MethodPost, "/api/issues/"+source+"/lifecycle-handoffs", body), "id", source)
			response := testutil.Call(t, h.CreateLifecycleHandoff, req)
			if winnerErr != nil {
				t.Fatal(winnerErr)
			}
			if compatible {
				var result lifecycleHandoffResponse
				response.Want(http.StatusCreated).JSON(&result)
				if result.QueuedTaskID != uuidToString(winner.ID) {
					t.Fatalf("lost the concurrent winner: %+v", result)
				}
				if count := dbfx.Count(t, `SELECT count(*) FROM comment WHERE issue_id=$1`, source); count != 1 {
					t.Fatalf("outer transaction did not commit audit: %d", count)
				}
			} else {
				response.Want(http.StatusConflict)
				if lifecycleAtomicSnapshot(t, source) != winnerSnapshot {
					t.Fatal("incompatible race did not roll back outer changes")
				}
			}
			if count := dbfx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id=$1`, child); count != 1 {
				t.Fatalf("task count = %d", count)
			}
			if taskEvents.Load() != 1 || wakeups.calls.Load() != 1 {
				t.Fatalf("race duplicated notifications: events=%d wakeups=%d", taskEvents.Load(), wakeups.calls.Load())
			}
		})
	}
}

func TestLifecycleAtomicNullHumanDelegationRemainsNull(t *testing.T) {
	source := lifecycleAtomicSource(t, "atomic autopilot provenance")
	actor := dbfx.Agent(t, "atomic autopilot actor", testRuntimeID)
	target := dbfx.Agent(t, "atomic workspace-public target", testRuntimeID, testutil.Cols{"permission_mode": "public_to"})
	dbfx.Insert(t, "agent_invocation_target", testutil.Cols{"agent_id": target, "target_type": "workspace", "target_id": testWorkspaceID, "created_by": testUserID})
	acting := dbfx.Task(t, actor, testutil.Cols{"runtime_id": testRuntimeID, "status": "running", "accountable_user_id": testUserID})
	var result lifecycleHandoffResponse
	lifecycleAtomicCall(t, source, lifecycleAtomicAssignedBody("agent", target), "X-Agent-ID", actor, "X-Task-ID", acting).Want(http.StatusCreated).JSON(&result)
	var originator, accountable, delegated string
	dbfx.QueryRow(t, `SELECT COALESCE(originator_user_id::text,''),accountable_user_id::text,delegated_from_task_id::text FROM agent_task_queue WHERE id=$1`, result.QueuedTaskID).Scan(&originator, &accountable, &delegated)
	if originator != "" || accountable != testUserID || delegated != acting {
		t.Fatalf("fabricated human authority: originator=%q accountable=%s delegated=%s", originator, accountable, delegated)
	}
}

func TestLifecycleAtomicWorkspaceDeleteFence(t *testing.T) {
	source := lifecycleAtomicSource(t, "atomic delete fence")
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	deleting, err := testPool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer deleting.Rollback(ctx)
	q := testHandler.Queries.WithTx(deleting)
	if _, err = q.LockWorkspaceForDelete(ctx, parseUUID(testWorkspaceID)); err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{})
	done := make(chan *testutil.Response, 1)
	go func() {
		close(started)
		done <- lifecycleAtomicCall(t, source, map[string]any{"kind": "rollout", "rollout": map[string]any{}})
	}()
	<-started
	// Teardown's next lock must be obtainable while the handoff is waiting
	// for the workspace fence; otherwise the two operations invert ownership.
	if _, err = deleting.Exec(ctx, `SELECT id FROM issue WHERE id=$1 FOR UPDATE NOWAIT`, source); err != nil {
		t.Fatalf("handoff locked source before teardown fence: %v", err)
	}
	if _, err = deleting.Exec(ctx, `DELETE FROM issue WHERE id=$1`, source); err != nil {
		t.Fatal(err)
	}
	if err = deleting.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	response := <-done
	if response.Code != http.StatusNotFound && response.Code != http.StatusBadRequest {
		t.Fatalf("handoff racing deletion returned%d: %s", response.Code, response.Body.String())
	}
	if count := dbfx.Count(t, `SELECT count(*) FROM comment WHERE issue_id=$1`, source); count != 0 {
		t.Fatalf("orphan audit count=%d", count)
	}
}

func TestLifecycleAtomicRechecksImplicitDuplicateAfterLock(t *testing.T) {
	for _, change := range []string{"rename", "close", "project"} {
		t.Run(change, func(t *testing.T) {
			source := lifecycleAtomicSource(t, "atomic duplicate predicate")
			child := dbfx.Issue(t, "original diagnostic", testutil.Cols{"parent_issue_id": source})
			h, _, _ := lifecycleIsolatedHandler()
			var once sync.Once
			h.TxStarter = lifecycleHookStarter{base: h.TxStarter, after: func(sql string, err error) {
				if strings.Contains(sql, "-- name: FindActiveDuplicateIssue ") && err == nil {
					once.Do(func() {
						switch change {
						case "rename":
							dbfx.Exec(t, `UPDATE issue SET title='changed diagnostic' WHERE id=$1`, child)
						case "close":
							dbfx.Exec(t, `UPDATE issue SET status='done' WHERE id=$1`, child)
						case "project":
							project := dbfx.Project(t, "different project")
							dbfx.Exec(t, `UPDATE issue SET project_id=$2 WHERE id=$1`, child, project)
						}
					})
				}
			}}
			body := map[string]any{"kind": "rca", "route": "maintenance", "cause_state": "unknown", "follow_up_title": "original diagnostic"}
			req := withURLParam(newRequest(http.MethodPost, "/api/issues/"+source+"/lifecycle-handoffs", body), "id", source)
			var result lifecycleHandoffResponse
			testutil.Call(t, h.CreateLifecycleHandoff, req).Want(http.StatusCreated).JSON(&result)
			if !result.FollowUpCreated || result.FollowUpIssueID == child {
				t.Fatalf("reused stale duplicate: %+v", result)
			}
			body["follow_up_issue_id"] = child
			lifecycleAtomicCall(t, source, body).Want(http.StatusCreated).JSON(&result)
			if !result.FollowUpReused || result.FollowUpIssueID != child {
				t.Fatalf("explicit-ID semantics changed: %+v", result)
			}
		})
	}
}

func TestLifecycleAtomicPendingCapabilitiesMatchWithoutSessionURLs(t *testing.T) {
	source := lifecycleAtomicSource(t, "atomic capability snapshot")
	agent := dbfx.Agent(t, "atomic capability agent", testRuntimeID, testutil.Cols{"composio_toolkit_allowlist": []string{"github", "gmail"}})
	h, wakeups, _ := lifecycleIsolatedHandler()
	flags := featureflag.NewStaticProvider()
	flags.Set(featureflags.ComposioMCPApps, featureflag.Rule{Default: true})
	h.TaskService.FeatureFlags = featureflag.NewService(flags)
	var session int
	h.TaskService.Composio = lifecycleOverlayBuilder{build: func(_ context.Context, _ pgtype.UUID, a db.Agent) (runtimeapps.MCPOverlayResult, error) {
		session++
		if len(a.ComposioToolkitAllowlist) == 0 {
			return runtimeapps.MCPOverlayResult{}, nil
		}
		apps := make([]runtimeapps.ConnectedApp, 0, len(a.ComposioToolkitAllowlist))
		for _, slug := range a.ComposioToolkitAllowlist {
			apps = append(apps, runtimeapps.ConnectedApp{Provider: "composio", ServerName: "composio", ToolkitSlug: slug, ToolkitName: fmt.Sprint(session)})
		}
		if session%2 == 0 && len(apps) > 1 {
			apps[0], apps[1] = apps[1], apps[0]
		}
		return runtimeapps.MCPOverlayResult{MCPOverlay: json.RawMessage(fmt.Sprintf(`{"mcpServers":{"composio":{"url":"https://example.invalid/session-%d"}}}`, session)), ConnectedApps: apps}, nil
	}}
	body := lifecycleAtomicAssignedBody("agent", agent)
	call := func() *testutil.Response {
		req := withURLParam(newRequest(http.MethodPost, "/api/issues/"+source+"/lifecycle-handoffs", body), "id", source)
		return testutil.Call(t, h.CreateLifecycleHandoff, req)
	}
	var first, second lifecycleHandoffResponse
	call().Want(http.StatusCreated).JSON(&first)
	call().Want(http.StatusCreated).JSON(&second)
	if first.QueuedTaskID != second.QueuedTaskID || wakeups.calls.Load() != 1 {
		t.Fatal("rotated session URL or display/order differences broke compatible reuse")
	}
	for _, allowed := range [][]string{{"github"}, {}} {
		dbfx.Exec(t, `UPDATE agent SET composio_toolkit_allowlist=$2::text[] WHERE id=$1`, agent, allowed)
		before := lifecycleAtomicSnapshot(t, source)
		call().Want(http.StatusConflict)
		if lifecycleAtomicSnapshot(t, source) != before || wakeups.calls.Load() != 1 {
			t.Fatal("changed capability context modified pending work or handoff")
		}
	}
}
