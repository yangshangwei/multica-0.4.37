package handler

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/multica-ai/multica/server/internal/iteration"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/internal/testutil"
)

func iterationIssueFixture(t *testing.T) (string, string) {
	t.Helper()
	if testHandler == nil {
		t.Skip("database not available")
	}
	iterationID := dbfx.Insert(t, "iteration", testutil.Cols{"workspace_id": testWorkspaceID, "name": "S1 recorder", "timezone": "UTC", "start_date": "2026-10-01", "end_date": "2026-10-14", "status": "active", "created_by": testUserID, "started_by": testUserID, "started_at": testutil.Raw("clock_timestamp()")})
	issueID := dbfx.Issue(t, "Original commitment", testutil.Cols{"status": "todo", "current_iteration_id": iterationID, "iteration_rollover_count": 2})
	dbfx.InsertNoID(t, "iteration_participation", testutil.Cols{"workspace_id": testWorkspaceID, "iteration_id": iterationID, "issue_id": issueID, "first_joined_at": testutil.Raw("clock_timestamp()"), "current_joined_at": testutil.Raw("clock_timestamp()"), "in_original": true, "original_facts": `{"title":"Original commitment","status_category":"todo"}`}, "iteration_id=$1 AND issue_id=$2", iterationID, issueID)
	dbfx.Cleanup(t, "DELETE FROM iteration_event WHERE iteration_id=$1", iterationID)
	return issueID, iterationID
}

func TestUpdateIssueIterationFacts(t *testing.T) {
	issueID, iterationID := iterationIssueFixture(t)
	projectID := dbfx.Project(t, "New project")
	request := withURLParam(newRequest("PUT", "/api/issues/"+issueID, map[string]any{"title": "Changed commitment", "status": "in_progress", "project_id": projectID, "assignee_type": "member", "assignee_id": testUserID, "expected_revision": 1}), "id", issueID)
	testutil.Call(t, testHandler.UpdateIssue, request).Want(http.StatusOK)
	var count, scope, revision, rollover int64
	var before, after []byte
	var original, started bool
	var pointer string
	dbfx.QueryRow(t, `SELECT count(*) FROM iteration_event WHERE iteration_id=$1`, iterationID).Scan(&count)
	if count != 1 {
		t.Fatalf("one factual update must append exactly one event, got %d", count)
	}
	dbfx.QueryRow(t, `SELECT before_facts,after_facts FROM iteration_event WHERE iteration_id=$1`, iterationID).Scan(&before, &after)
	var old, next map[string]any
	if err := json.Unmarshal(before, &old); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(after, &next); err != nil {
		t.Fatal(err)
	}
	if old["title"] != "Original commitment" || next["title"] != "Changed commitment" || old["status_category"] != "todo" || next["status_category"] != "in_progress" || next["project_id"] != projectID || next["assignee_id"] != testUserID {
		t.Fatalf("incorrect before/after facts: %s -> %s", before, after)
	}
	dbfx.QueryRow(t, `SELECT scope_revision FROM iteration WHERE id=$1`, iterationID).Scan(&scope)
	dbfx.QueryRow(t, `SELECT in_original,has_started_current_participation FROM iteration_participation WHERE iteration_id=$1 AND issue_id=$2`, iterationID, issueID).Scan(&original, &started)
	dbfx.QueryRow(t, `SELECT revision,current_iteration_id,iteration_rollover_count FROM issue WHERE id=$1`, issueID).Scan(&revision, &pointer, &rollover)
	if scope != 2 || revision != 2 || !original || !started || pointer != iterationID || rollover != 2 {
		t.Fatalf("atomic versions/participation: scope=%d issue=%d original=%v started=%v pointer=%s rollover=%d", scope, revision, original, started, pointer, rollover)
	}
}

func TestUpdateIssueIterationNoFactChange(t *testing.T) {
	for _, body := range []map[string]any{{"title": "Original commitment"}, {"description": "Only description"}, {"priority": "high"}} {
		t.Run("non_scope", func(t *testing.T) {
			issueID, iterationID := iterationIssueFixture(t)
			testutil.Call(t, testHandler.UpdateIssue, withURLParam(newRequest("PUT", "/api/issues/"+issueID, body), "id", issueID)).Want(http.StatusOK)
			var count, scope int64
			dbfx.QueryRow(t, `SELECT count(*) FROM iteration_event WHERE iteration_id=$1`, iterationID).Scan(&count)
			dbfx.QueryRow(t, `SELECT scope_revision FROM iteration WHERE id=$1`, iterationID).Scan(&scope)
			if count != 0 || scope != 1 {
				t.Fatalf("non-factual write manufactured scope activity: events=%d scope=%d", count, scope)
			}
		})
	}
}

type iterationEventFailureStarter struct{ inner txStarter }

func (s iterationEventFailureStarter) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := s.inner.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &iterationEventFailureTx{Tx: tx}, nil
}

type iterationEventFailureTx struct{ pgx.Tx }

func (tx *iterationEventFailureTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if strings.Contains(sql, "AppendIterationIssueEvent") {
		return pgconn.CommandTag{}, errors.New("injected event append failure")
	}
	return tx.Tx.Exec(ctx, sql, args...)
}

func TestUpdateIssueIterationAtomicFailure(t *testing.T) {
	for _, failure := range []string{"event", "attachment"} {
		t.Run(failure, func(t *testing.T) {
			issueID, iterationID := iterationIssueFixture(t)
			attachmentID := dbfx.Insert(t, "attachment", testutil.Cols{"workspace_id": testWorkspaceID, "uploader_type": "member", "uploader_id": testUserID, "filename": "atomic.txt", "url": "/atomic.txt", "content_type": "text/plain", "size_bytes": 1})
			h := *testHandler
			if failure == "event" {
				h.TxStarter = iterationEventFailureStarter{inner: h.TxStarter}
			} else {
				h.TxStarter = failIssueAttachmentLinkTxStarter{inner: h.TxStarter}
			}
			testutil.Call(t, h.UpdateIssue, withURLParam(newRequest("PUT", "/api/issues/"+issueID, map[string]any{"title": "Must roll back", "status": "in_progress", "attachment_ids": []string{attachmentID}}), "id", issueID)).Want(http.StatusInternalServerError)
			var title string
			var revision, scope, count int64
			var linked, started bool
			dbfx.QueryRow(t, `SELECT title,revision FROM issue WHERE id=$1`, issueID).Scan(&title, &revision)
			dbfx.QueryRow(t, `SELECT scope_revision FROM iteration WHERE id=$1`, iterationID).Scan(&scope)
			dbfx.QueryRow(t, `SELECT count(*) FROM iteration_event WHERE iteration_id=$1`, iterationID).Scan(&count)
			dbfx.QueryRow(t, `SELECT has_started_current_participation FROM iteration_participation WHERE iteration_id=$1 AND issue_id=$2`, iterationID, issueID).Scan(&started)
			dbfx.QueryRow(t, `SELECT issue_id IS NOT NULL FROM attachment WHERE id=$1`, attachmentID).Scan(&linked)
			if title != "Original commitment" || revision != 1 || scope != 1 || count != 0 || started || linked {
				t.Fatalf("partial commit title=%q revision=%d scope=%d events=%d started=%v linked=%v", title, revision, scope, count, started, linked)
			}
		})
	}
}

func TestUpdateIssueIterationTransitions(t *testing.T) {
	issueID, iterationID := iterationIssueFixture(t)
	for _, status := range []string{"cancelled", "todo", "done", "blocked"} {
		testutil.Call(t, testHandler.UpdateIssue, withURLParam(newRequest("PUT", "/api/issues/"+issueID, map[string]any{"status": status}), "id", issueID)).Want(http.StatusOK)
	}
	rows, err := testPool.Query(context.Background(), `SELECT sequence,kind,after_facts->>'has_started' FROM iteration_event WHERE iteration_id=$1 ORDER BY sequence`, iterationID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	want := []string{"cancel", "reopen", "status", "reopen"}
	i := 0
	for rows.Next() {
		var sequence int64
		var kind, started string
		if err = rows.Scan(&sequence, &kind, &started); err != nil {
			t.Fatal(err)
		}
		if i >= len(want) || sequence != int64(i+1) || kind != want[i] || (i >= 2 && started != "true") {
			t.Fatalf("unexpected transition sequence=%d kind=%s started=%s", sequence, kind, started)
		}
		i++
	}
	if err = rows.Err(); err != nil {
		t.Fatal(err)
	}
	if i != 4 {
		t.Fatalf("events=%d", i)
	}
	var original []byte
	dbfx.QueryRow(t, `SELECT original_facts FROM iteration_participation WHERE iteration_id=$1 AND issue_id=$2`, iterationID, issueID).Scan(&original)
	var facts map[string]any
	if err = json.Unmarshal(original, &facts); err != nil {
		t.Fatal(err)
	}
	if facts["title"] != "Original commitment" || facts["status_category"] != "todo" {
		t.Fatalf("original facts drifted: %s", original)
	}
}

type iterationBeginBarrier struct {
	inner   txStarter
	reached chan<- struct{}
	release <-chan struct{}
}

func (s iterationBeginBarrier) Begin(ctx context.Context) (pgx.Tx, error) {
	s.reached <- struct{}{}
	select {
	case <-s.release:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	return s.inner.Begin(ctx)
}
func waitIterationBarrier(t *testing.T, reached <-chan struct{}) {
	t.Helper()
	select {
	case <-reached:
	case <-time.After(5 * time.Second):
		t.Fatal("writer did not reach barrier")
	}
}

func TestUpdateIssueIterationConcurrentRevision(t *testing.T) {
	issueID, iterationID := iterationIssueFixture(t)
	reached, release := make(chan struct{}, 2), make(chan struct{})
	h := *testHandler
	h.TxStarter = iterationBeginBarrier{h.TxStarter, reached, release}
	results := make(chan *testutil.Response, 2)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for _, title := range []string{"Winner A", "Winner B"} {
		go func(title string) {
			results <- testutil.Call(t, h.UpdateIssue, withURLParam(newRequest("PUT", "/api/issues/"+issueID, map[string]any{"title": title, "expected_revision": 1}).WithContext(ctx), "id", issueID))
		}(title)
	}
	waitIterationBarrier(t, reached)
	waitIterationBarrier(t, reached)
	close(release)
	a, b := <-results, <-results
	if !((a.Code == 200 && b.Code == 409) || (a.Code == 409 && b.Code == 200)) {
		t.Fatalf("concurrent revision statuses=%d/%d: %s / %s", a.Code, b.Code, a.Body, b.Body)
	}
	var count, scope, revision int64
	dbfx.QueryRow(t, `SELECT count(*) FROM iteration_event WHERE iteration_id=$1`, iterationID).Scan(&count)
	dbfx.QueryRow(t, `SELECT scope_revision FROM iteration WHERE id=$1`, iterationID).Scan(&scope)
	dbfx.QueryRow(t, `SELECT revision FROM issue WHERE id=$1`, issueID).Scan(&revision)
	if count != 1 || scope != 2 || revision != 2 {
		t.Fatalf("loser leaked facts: events=%d scope=%d revision=%d", count, scope, revision)
	}
}
func TestUpdateIssueIterationRevokedBeforeTransaction(t *testing.T) {
	issueID, iterationID := iterationIssueFixture(t)
	reached, release := make(chan struct{}, 1), make(chan struct{})
	h := *testHandler
	h.TxStarter = iterationBeginBarrier{h.TxStarter, reached, release}
	result := make(chan *testutil.Response, 1)
	go func() {
		result <- testutil.Call(t, h.UpdateIssue, withURLParam(newRequest("PUT", "/api/issues/"+issueID, map[string]any{"title": "Unauthorized", "expected_revision": 1}), "id", issueID))
	}()
	waitIterationBarrier(t, reached)
	var memberJSON []byte
	dbfx.QueryRow(t, `SELECT row_to_json(member) FROM member WHERE workspace_id=$1 AND user_id=$2`, testWorkspaceID, testUserID).Scan(&memberJSON)
	dbfx.Exec(t, `DELETE FROM member WHERE workspace_id=$1 AND user_id=$2`, testWorkspaceID, testUserID)
	t.Cleanup(func() {
		dbfx.Exec(t, `INSERT INTO member SELECT * FROM json_populate_record(NULL::member,$1::json)`, string(memberJSON))
	})
	close(release)
	response := <-result
	response.Want(http.StatusForbidden)
	if strings.Contains(response.Body.String(), "current_revision") {
		t.Fatal("revoked response leaked current revision")
	}
	var count int
	dbfx.QueryRow(t, `SELECT count(*) FROM iteration_event WHERE iteration_id=$1`, iterationID).Scan(&count)
	if count != 0 {
		t.Fatal("revoked actor wrote an event")
	}
}

type iterationFenceBarrierStarter struct {
	inner   txStarter
	reached chan<- struct{}
}

func (s iterationFenceBarrierStarter) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := s.inner.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &iterationFenceBarrierTx{Tx: tx, reached: s.reached}, nil
}

type iterationFenceBarrierTx struct {
	pgx.Tx
	reached chan<- struct{}
}

func (tx *iterationFenceBarrierTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if strings.Contains(sql, "'iteration:'") {
		tx.reached <- struct{}{}
	}
	return tx.Tx.Exec(ctx, sql, args...)
}

func (tx *iterationFenceBarrierTx) SendBatch(ctx context.Context, batch *pgx.Batch) pgx.BatchResults {
	for _, query := range batch.QueuedQueries {
		if strings.Contains(query.SQL, "'iteration:'") {
			tx.reached <- struct{}{}
		}
	}
	return tx.Tx.SendBatch(ctx, batch)
}

func TestUpdateIssueIterationJoinBeforeWriter(t *testing.T) {
	issueID, iterationID := iterationIssueFixture(t)
	dbfx.Exec(t, `UPDATE issue SET current_iteration_id=NULL WHERE id=$1`, issueID)
	dbfx.Exec(t, `UPDATE iteration_participation SET current_joined_at=NULL WHERE iteration_id=$1 AND issue_id=$2`, iterationID, issueID)
	attachmentID := dbfx.Insert(t, "attachment", testutil.Cols{"workspace_id": testWorkspaceID, "uploader_type": "member", "uploader_id": testUserID, "filename": "join.txt", "url": "/join.txt", "content_type": "text/plain", "size_bytes": 1})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	tx, err := testPool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if err = iteration.LockWorkspace(ctx, tx, parseUUID(testWorkspaceID)); err != nil {
		t.Fatal(err)
	}
	reached := make(chan struct{}, 1)
	h := *testHandler
	h.TxStarter = iterationFenceBarrierStarter{h.TxStarter, reached}
	result := make(chan *testutil.Response, 1)
	go func() {
		result <- testutil.Call(t, h.UpdateIssue, withURLParam(newRequest("PUT", "/api/issues/"+issueID, map[string]any{"title": "Joined then changed", "attachment_ids": []string{attachmentID}}).WithContext(ctx), "id", issueID))
	}()
	waitIterationBarrier(t, reached)
	// A writer waiting on the I1 fence must own neither attachment nor issue.
	if _, err = tx.Exec(ctx, `SELECT id FROM attachment WHERE id=$1 FOR UPDATE NOWAIT`, attachmentID); err != nil {
		t.Fatalf("attachment locked before iteration fence: %v", err)
	}
	if _, err = tx.Exec(ctx, `SELECT id FROM issue WHERE id=$1 FOR UPDATE NOWAIT`, issueID); err != nil {
		t.Fatalf("issue locked before iteration fence: %v", err)
	}
	if _, err = tx.Exec(ctx, `UPDATE issue SET current_iteration_id=$1 WHERE id=$2`, iterationID, issueID); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, `UPDATE iteration_participation SET current_joined_at=clock_timestamp() WHERE iteration_id=$1 AND issue_id=$2`, iterationID, issueID); err != nil {
		t.Fatal(err)
	}
	var joinedAt time.Time
	if err = tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&joinedAt); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	(<-result).Want(http.StatusOK)
	var count int
	var sampled time.Time
	dbfx.QueryRow(t, `SELECT count(*),min(sampled_at) FROM iteration_event WHERE iteration_id=$1`, iterationID).Scan(&count, &sampled)
	if count != 1 || sampled.Before(joinedAt) {
		t.Fatalf("concurrent join lost facts or used prelock time count=%d sampled=%s joined=%s", count, sampled, joinedAt)
	}
}

func TestUpdateIssueIterationArchivedCustomAndNullFacts(t *testing.T) {
	issueID, iterationID := iterationIssueFixture(t)
	dbfx.Insert(t, "issue_status", testutil.Cols{"workspace_id": testWorkspaceID, "key": "s1_archived", "name": "Archived review", "category": "in_review", "color": "#ff0000", "position": 20, "archived_at": testutil.Raw("clock_timestamp()")})
	projectID := dbfx.Project(t, "Old project")
	dbfx.Exec(t, `UPDATE issue SET status='s1_archived',project_id=$1,assignee_type='member',assignee_id=$2 WHERE id=$3`, projectID, testUserID, issueID)
	dbfx.Exec(t, `UPDATE iteration_participation SET has_started_current_participation=true WHERE iteration_id=$1`, iterationID)
	testutil.Call(t, testHandler.UpdateIssue, withURLParam(newRequest("PUT", "/api/issues/"+issueID, map[string]any{"title": "Custom facts", "project_id": nil, "assignee_type": nil, "assignee_id": nil}), "id", issueID)).Want(200)
	var raw []byte
	dbfx.QueryRow(t, `SELECT after_facts FROM iteration_event WHERE iteration_id=$1`, iterationID).Scan(&raw)
	var facts map[string]any
	if err := json.Unmarshal(raw, &facts); err != nil {
		t.Fatal(err)
	}
	if facts["status_category"] != "in_review" || facts["project_id"] != nil || facts["assignee_id"] != nil || facts["assignee_type"] != nil {
		t.Fatalf("custom/null facts=%s", raw)
	}
}

func TestUpdateIssueIterationSequenceClampsClock(t *testing.T) {
	issueID, iterationID := iterationIssueFixture(t)
	future := time.Now().UTC().Add(time.Hour)
	dbfx.Insert(t, "iteration_event", testutil.Cols{"workspace_id": testWorkspaceID, "iteration_id": iterationID, "issue_id": issueID, "sequence": 7, "operation_id": uuid.NewString(), "kind": "status", "actor": `{"type":"member"}`, "occurred_at": future, "sampled_at": future})
	testutil.Call(t, testHandler.UpdateIssue, withURLParam(newRequest("PUT", "/api/issues/"+issueID, map[string]any{"title": "After clock rollback"}), "id", issueID)).Want(200)
	var sequence int64
	var occurred, sampled time.Time
	dbfx.QueryRow(t, `SELECT sequence,occurred_at,sampled_at FROM iteration_event WHERE iteration_id=$1 ORDER BY sequence DESC LIMIT 1`, iterationID).Scan(&sequence, &occurred, &sampled)
	if sequence != 8 || occurred.Before(future.Truncate(time.Microsecond)) || !sampled.Before(occurred) {
		t.Fatalf("clock clamp sequence=%d occurred=%s sampled=%s", sequence, occurred, sampled)
	}
}

func TestUpdateIssueIterationAttachmentOnlyAndBatch(t *testing.T) {
	issueID, iterationID := iterationIssueFixture(t)
	attachmentID := dbfx.Insert(t, "attachment", testutil.Cols{"workspace_id": testWorkspaceID, "uploader_type": "member", "uploader_id": testUserID, "filename": "content.txt", "url": "/content.txt", "content_type": "text/plain", "size_bytes": 1})
	for i := 0; i < 2; i++ {
		testutil.Call(t, testHandler.UpdateIssue, withURLParam(newRequest("PUT", "/api/issues/"+issueID, map[string]any{"attachment_ids": []string{attachmentID}}), "id", issueID)).Want(200)
	}
	var count, revision int64
	dbfx.QueryRow(t, `SELECT count(*) FROM iteration_event WHERE iteration_id=$1`, iterationID).Scan(&count)
	dbfx.QueryRow(t, `SELECT revision FROM issue WHERE id=$1`, issueID).Scan(&revision)
	if count != 0 || revision != 2 {
		t.Fatalf("attachment-only/duplicate events=%d revision=%d", count, revision)
	}
	testutil.Call(t, testHandler.BatchUpdateIssues, newRequest("POST", "/api/issues/batch-update", map[string]any{"issue_ids": []string{issueID}, "updates": map[string]any{"status": "blocked"}})).Want(200)
	dbfx.QueryRow(t, `SELECT count(*) FROM iteration_event WHERE iteration_id=$1`, iterationID).Scan(&count)
	var started bool
	dbfx.QueryRow(t, `SELECT has_started_current_participation FROM iteration_participation WHERE iteration_id=$1`, iterationID).Scan(&started)
	if count != 1 || started {
		t.Fatalf("batch event=%d blocked incorrectly started=%v", count, started)
	}
}

func TestUpdateIssueIterationDoesNotChangeExecution(t *testing.T) {
	issueID, iterationID := iterationIssueFixture(t)
	runtimeID := handlerTestRuntimeID(t)
	agentID := dbfx.Agent(t, "I1 active execution", runtimeID)
	dbfx.Exec(t, `UPDATE issue SET assignee_type='agent',assignee_id=$1 WHERE id=$2`, agentID, issueID)
	taskID := dbfx.Task(t, agentID, testutil.Cols{"runtime_id": runtimeID, "issue_id": issueID, "status": "running", "started_at": testutil.Raw("clock_timestamp()")})
	var before []byte
	dbfx.QueryRow(t, `SELECT row_to_json(agent_task_queue) FROM agent_task_queue WHERE id=$1`, taskID).Scan(&before)
	testutil.Call(t, testHandler.UpdateIssue, withURLParam(newRequest("PUT", "/api/issues/"+issueID, map[string]any{"title": "Still executing", "status": "cancelled", "assignee_type": "member", "assignee_id": testUserID}), "id", issueID)).Want(200)
	var after []byte
	var tasks, events int
	dbfx.QueryRow(t, `SELECT row_to_json(agent_task_queue) FROM agent_task_queue WHERE id=$1`, taskID).Scan(&after)
	dbfx.QueryRow(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id=$1`, issueID).Scan(&tasks)
	dbfx.QueryRow(t, `SELECT count(*) FROM iteration_event WHERE iteration_id=$1`, iterationID).Scan(&events)
	if string(before) != string(after) || tasks != 1 || events != 1 {
		t.Fatalf("execution drift tasks=%d events=%d before=%s after=%s", tasks, events, before, after)
	}
}

type iterationRetryStarter struct {
	inner        txStarter
	operationIDs []any
	actors       []string
}

func (s *iterationRetryStarter) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := s.inner.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &iterationRetryTx{Tx: tx, owner: s}, nil
}

type iterationRetryTx struct {
	pgx.Tx
	owner *iterationRetryStarter
}

func (tx *iterationRetryTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if strings.Contains(sql, "AppendIterationIssueEvent") {
		tx.owner.operationIDs = append(tx.owner.operationIDs, args[2])
		tx.owner.actors = append(tx.owner.actors, string(args[5].([]byte)))
		if len(tx.owner.operationIDs) == 1 {
			return pgconn.CommandTag{}, &pgconn.PgError{Code: "55P03", Message: "injected recorder lock conflict"}
		}
	}
	return tx.Tx.Exec(ctx, sql, args...)
}
func TestUpdateIssueIterationRetryIdentity(t *testing.T) {
	issueID, iterationID := iterationIssueFixture(t)
	h := *testHandler
	starter := &iterationRetryStarter{inner: h.TxStarter}
	h.TxStarter = starter
	testutil.Call(t, h.UpdateIssue, withURLParam(newRequest("PUT", "/api/issues/"+issueID, map[string]any{"status": "in_progress"}), "id", issueID)).Want(200)
	if len(starter.operationIDs) != 2 || starter.operationIDs[0] != starter.operationIDs[1] || starter.actors[0] != starter.actors[1] {
		t.Fatalf("retry identity drift: %#v %#v", starter.operationIDs, starter.actors)
	}
	var count, scope, revision int64
	dbfx.QueryRow(t, `SELECT count(*) FROM iteration_event WHERE iteration_id=$1`, iterationID).Scan(&count)
	dbfx.QueryRow(t, `SELECT scope_revision FROM iteration WHERE id=$1`, iterationID).Scan(&scope)
	dbfx.QueryRow(t, `SELECT revision FROM issue WHERE id=$1`, issueID).Scan(&revision)
	if count != 1 || scope != 2 || revision != 2 {
		t.Fatalf("retry duplicate events=%d scope=%d revision=%d", count, scope, revision)
	}
}

func TestUpdateIssueIterationActorTaskFence(t *testing.T) {
	issueID, iterationID := iterationIssueFixture(t)
	runtimeID := handlerTestRuntimeID(t)
	actorID := dbfx.Agent(t, "I1 author", runtimeID)
	targetID := dbfx.Agent(t, "I1 private target", runtimeID, testutil.Cols{"permission_mode": "private"})
	taskID := dbfx.Task(t, actorID, testutil.Cols{"runtime_id": runtimeID, "issue_id": issueID, "status": "running", "originator_user_id": testUserID, "accountable_user_id": testUserID, "originator_source": "trigger_owner"})
	request := func(body map[string]any) *http.Request {
		r := withURLParam(newRequest("PUT", "/api/issues/"+issueID, body), "id", issueID)
		r.Header.Set("X-Agent-ID", actorID)
		r.Header.Set("X-Task-ID", taskID)
		return r
	}
	tx, err := testPool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	if _, err = tx.Exec(context.Background(), `SELECT id FROM agent_task_queue WHERE id=$1 FOR UPDATE`, taskID); err != nil {
		t.Fatal(err)
	}
	testutil.Call(t, testHandler.UpdateIssue, request(map[string]any{"assignee_type": "agent", "assignee_id": targetID})).Want(http.StatusConflict)
	var count int
	dbfx.QueryRow(t, `SELECT count(*) FROM iteration_event WHERE iteration_id=$1`, iterationID).Scan(&count)
	if count != 0 {
		t.Fatal("busy actor task leaked facts")
	}
	if _, err = tx.Exec(context.Background(), `UPDATE agent_task_queue SET status='completed' WHERE id=$1`, taskID); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	testutil.Call(t, testHandler.UpdateIssue, request(map[string]any{"assignee_type": "agent", "assignee_id": targetID})).Want(http.StatusForbidden)
	// Terminal tasks can still edit ordinary content, but lend no invoke grant.
	testutil.Call(t, testHandler.UpdateIssue, request(map[string]any{"title": "Terminal actor content"})).Want(http.StatusOK)
	dbfx.QueryRow(t, `SELECT count(*) FROM iteration_event WHERE iteration_id=$1`, iterationID).Scan(&count)
	if count != 1 {
		t.Fatalf("ordinary terminal content event count=%d", count)
	}
}

func TestIssueWriteFenceSQLMatchesAuthoritativeQueries(t *testing.T) {
	definitions := []struct {
		file, name string
		args       map[string]string
	}{
		{"workspace.sql", "LockWorkspaceForChatSessionCreate", nil},
		{"subscriber.sql", "LockSubscriberWrites", map[string]string{"workspace_id": "$1", "user_id": "$2"}},
		{"subscriber.sql", "LockActiveMember", nil},
		{"issue_status.sql", "LockIssueStatusCatalogShared", map[string]string{"workspace_id": "$1"}},
	}
	normalize := func(s string) string {
		lines := strings.Split(s, "\n")
		var sql string
		for _, line := range lines {
			if !strings.HasPrefix(strings.TrimSpace(line), "--") {
				sql += line
			}
		}
		return strings.TrimSuffix(strings.Join(strings.Fields(sql), ""), ";")
	}
	for i, definition := range definitions {
		contents, err := os.ReadFile("../../pkg/db/queries/" + definition.file)
		if err != nil {
			t.Fatal(err)
		}
		_, section, ok := strings.Cut(string(contents), "-- name: "+definition.name+" ")
		if !ok {
			t.Fatalf("missing query %s", definition.name)
		}
		_, section, _ = strings.Cut(section, "\n")
		section, _, _ = strings.Cut(section, "\n-- name:")
		for name, value := range definition.args {
			section = strings.ReplaceAll(section, "sqlc.arg('"+name+"')", value)
			section = strings.ReplaceAll(section, "sqlc.arg("+name+")", value)
		}
		if normalize(section) != normalize(issueWriteFenceSQL[i]) {
			t.Fatalf("batched %s drifted from %s: %s vs %s", definition.name, definition.file, normalize(issueWriteFenceSQL[i]), normalize(section))
		}
	}
}

func TestUpdateIssueIterationMovePath(t *testing.T) {
	issueID, iterationID := iterationIssueFixture(t)
	testutil.Call(t, testHandler.MoveIssue, withURLParam(newRequest("POST", "/api/issues/"+issueID+"/move", map[string]any{"before_id": nil, "after_id": nil, "status": "blocked"}), "id", issueID)).Want(200)
	var count int
	var category string
	dbfx.QueryRow(t, `SELECT count(*),min(after_facts->>'status_category') FROM iteration_event WHERE iteration_id=$1`, iterationID).Scan(&count, &category)
	if count != 1 || category != "blocked" {
		t.Fatalf("move recorder events=%d category=%s", count, category)
	}
}
