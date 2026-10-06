package handler

import (
	"context"
	"errors"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/multica-ai/multica/server/internal/iteration"
	"github.com/multica-ai/multica/server/internal/testutil"
)

func TestDeleteIssueIterationRetainsDeletionFacts(t *testing.T) {
	for _, status := range []string{"active", "planned"} {
		t.Run(status, func(t *testing.T) {
			issueID, iterationID := iterationIssueFixture(t)
			dbfx.Exec(t, `UPDATE iteration SET status=$2 WHERE id=$1`, iterationID, status)
			dbfx.Exec(t, `UPDATE iteration_participation SET has_started_current_participation=true WHERE issue_id=$1`, issueID)
			child := dbfx.Issue(t, "Detached child", testutil.Cols{"parent_issue_id": issueID, "stage": 2})
			historical := dbfx.Insert(t, "iteration", testutil.Cols{"workspace_id": testWorkspaceID, "name": "Frozen deletion history", "timezone": "UTC", "start_date": "2026-09-01", "end_date": "2026-09-14", "status": "completed", "created_by": testUserID})
			dbfx.InsertNoID(t, "iteration_snapshot", testutil.Cols{"workspace_id": testWorkspaceID, "iteration_id": historical, "operation_id": uuid.NewString(), "created_at": testutil.Raw("clock_timestamp()"), "body": `{"title":"Frozen deletion history"}`}, "iteration_id=$1", historical)
			testutil.Call(t, testHandler.DeleteIssue, withURLParam(newRequest("DELETE", "/api/issues/"+issueID, nil), "id", issueID)).Want(204)
			if n := dbfx.Count(t, `SELECT count(*) FROM iteration_event WHERE iteration_id=$1 AND issue_id=$2 AND kind='delete' AND before_facts->>'title'='Original commitment' AND after_facts='null'::jsonb AND actor->>'type'='member' AND actor->>'user_id'=$3`, iterationID, issueID, testUserID); n != 1 {
				t.Fatalf("deletion must retain one delete/leave fact, got %d", n)
			}
			if n := dbfx.Count(t, `SELECT count(*) FROM iteration_participation WHERE iteration_id=$1 AND issue_id=$2 AND current_joined_at IS NULL AND NOT has_started_current_participation AND last_left_at IS NOT NULL AND in_original AND original_facts->>'title'='Original commitment'`, iterationID, issueID); n != 1 {
				t.Fatal("delete lost original facts or did not release current participation")
			}
			if n := dbfx.Count(t, `SELECT count(*) FROM iteration WHERE id=$1 AND scope_revision=2`, iterationID); n != 1 {
				t.Fatal("delete did not advance scope revision")
			}
			if n := dbfx.Count(t, `SELECT count(*) FROM issue WHERE id=$1`, issueID); n != 0 {
				t.Fatal("deleted issue remains")
			}
			if n := dbfx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND parent_issue_id IS NULL AND stage IS NULL AND revision=2`, child); n != 1 {
				t.Fatal("delete did not preserve and detach direct child")
			}
			if n := dbfx.Count(t, `SELECT count(*) FROM iteration_snapshot WHERE iteration_id=$1 AND body='{"title":"Frozen deletion history"}'::jsonb`, historical); n != 1 {
				t.Fatal("delete changed historical snapshot")
			}
		})
	}
}

func TestBatchDeleteIssueIterationKeepsUnselectedChildrenAndHistory(t *testing.T) {
	parent, active := iterationIssueFixture(t)
	planned := dbfx.Insert(t, "iteration", testutil.Cols{"workspace_id": testWorkspaceID, "name": "Planned deletion", "timezone": "UTC", "start_date": "2026-10-15", "end_date": "2026-10-28", "created_by": testUserID})
	child := dbfx.Issue(t, "Selected child", testutil.Cols{"parent_issue_id": parent, "stage": 2, "current_iteration_id": planned})
	dbfx.InsertNoID(t, "iteration_participation", testutil.Cols{"workspace_id": testWorkspaceID, "iteration_id": planned, "issue_id": child, "first_joined_at": testutil.Raw("clock_timestamp()"), "current_joined_at": testutil.Raw("clock_timestamp()")}, "iteration_id=$1 AND issue_id=$2", planned, child)
	dbfx.Cleanup(t, `DELETE FROM iteration_event WHERE iteration_id=$1`, planned)
	grandchild := dbfx.Issue(t, "Unselected grandchild", testutil.Cols{"parent_issue_id": child, "stage": 3, "current_iteration_id": active})
	dbfx.InsertNoID(t, "iteration_participation", testutil.Cols{"workspace_id": testWorkspaceID, "iteration_id": active, "issue_id": grandchild, "first_joined_at": testutil.Raw("clock_timestamp()"), "current_joined_at": testutil.Raw("clock_timestamp()")}, "iteration_id=$1 AND issue_id=$2", active, grandchild)
	former := dbfx.Issue(t, "Former participant")
	dbfx.InsertNoID(t, "iteration_participation", testutil.Cols{"workspace_id": testWorkspaceID, "iteration_id": active, "issue_id": former, "first_joined_at": testutil.Raw("clock_timestamp()"), "last_left_at": testutil.Raw("clock_timestamp()"), "in_original": true, "original_facts": `{"title":"Former commitment"}`}, "iteration_id=$1 AND issue_id=$2", active, former)
	pending := dbfx.Issue(t, "Pending deletion", testutil.Cols{"admission_status": "pending"})
	var formerBefore, formerAfter string
	dbfx.QueryRow(t, `SELECT row_to_json(p)::text FROM iteration_participation p WHERE issue_id=$1`, former).Scan(&formerBefore)
	var response struct {
		Deleted int `json:"deleted"`
	}
	testutil.Call(t, testHandler.BatchDeleteIssues, newRequest("POST", "/api/issues/batch-delete", map[string]any{"issue_ids": []string{child, parent, child, former, pending, "invalid", uuid.NewString()}})).Want(200).JSON(&response)
	if response.Deleted != 4 {
		t.Fatalf("batch count=%d want=4", response.Deleted)
	}
	var count, samples, operations int
	dbfx.QueryRow(t, `SELECT count(*),count(DISTINCT sampled_at),count(DISTINCT operation_id) FROM iteration_event WHERE iteration_id=ANY($1::uuid[])`, []string{active, planned}).Scan(&count, &samples, &operations)
	if count != 2 || samples != 1 || operations != 1 {
		t.Fatalf("batch delete facts/time/identity=%d/%d/%d want 2/1/1", count, samples, operations)
	}
	if dbfx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND parent_issue_id IS NULL AND stage IS NULL AND revision=2 AND current_iteration_id=$2`, grandchild, active) != 1 {
		t.Fatal("grandchild detach changed membership or revision unexpectedly")
	}
	if dbfx.Count(t, `SELECT count(*) FROM iteration_participation WHERE issue_id=$1 AND current_joined_at IS NOT NULL`, grandchild) != 1 {
		t.Fatal("unselected child's participation was released")
	}
	dbfx.QueryRow(t, `SELECT row_to_json(p)::text FROM iteration_participation p WHERE issue_id=$1`, former).Scan(&formerAfter)
	if formerBefore != formerAfter {
		t.Fatal("deletion of an unassociated issue rewrote former participation")
	}
}

type issueDeleteFailureStarter struct{ inner txStarter }

func (s issueDeleteFailureStarter) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := s.inner.Begin(ctx)
	return &issueDeleteFailureTx{Tx: tx}, err
}

type issueDeleteFailureTx struct {
	pgx.Tx
	deletes int
}

func (tx *issueDeleteFailureTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if strings.HasPrefix(sql, "-- name: DeleteIssue :exec") {
		tx.deletes++
		if tx.deletes == 2 {
			return pgconn.CommandTag{}, errors.New("injected second issue delete failure")
		}
	}
	return tx.Tx.Exec(ctx, sql, args...)
}

func TestBatchDeleteIssueIterationRollsBackEarlierDelete(t *testing.T) {
	first, iterationID := iterationIssueFixture(t)
	second := dbfx.Issue(t, "Second deletion", testutil.Cols{"current_iteration_id": iterationID})
	dbfx.InsertNoID(t, "iteration_participation", testutil.Cols{"workspace_id": testWorkspaceID, "iteration_id": iterationID, "issue_id": second, "first_joined_at": testutil.Raw("clock_timestamp()"), "current_joined_at": testutil.Raw("clock_timestamp()")}, "iteration_id=$1 AND issue_id=$2", iterationID, second)
	child := dbfx.Issue(t, "Child rollback", testutil.Cols{"parent_issue_id": first, "stage": 2})
	attachment := dbfx.Insert(t, "attachment", testutil.Cols{"workspace_id": testWorkspaceID, "issue_id": first, "uploader_type": "member", "uploader_id": testUserID, "filename": "delete.txt", "url": "/delete.txt", "content_type": "text/plain", "size_bytes": 1})
	h := *testHandler
	h.TxStarter = issueDeleteFailureStarter{inner: h.TxStarter}
	testutil.Call(t, h.BatchDeleteIssues, newRequest("POST", "/api/issues/batch-delete", map[string]any{"issue_ids": []string{first, second}})).Want(500)
	if dbfx.Count(t, `SELECT count(*) FROM issue WHERE id=ANY($1::uuid[]) AND revision=1`, []string{first, second}) != 2 || dbfx.Count(t, `SELECT count(*) FROM attachment WHERE id=$1`, attachment) != 1 {
		t.Fatal("earlier issue or attachment deletion escaped rollback")
	}
	if dbfx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND parent_issue_id=$2 AND stage=2 AND revision=1`, child, first) != 1 {
		t.Fatal("child detach escaped rollback")
	}
	if dbfx.Count(t, `SELECT count(*) FROM iteration_event WHERE iteration_id=$1`, iterationID) != 0 || dbfx.Count(t, `SELECT count(*) FROM iteration WHERE id=$1 AND scope_revision=1`, iterationID) != 1 || dbfx.Count(t, `SELECT count(*) FROM iteration_participation WHERE iteration_id=$1 AND current_joined_at IS NOT NULL AND last_left_at IS NULL`, iterationID) != 2 {
		t.Fatal("delete history or participation escaped rollback")
	}
}

func TestDeleteIssueIterationSeesJoinAfterFenceWait(t *testing.T) {
	issueID, iterationID := iterationIssueFixture(t)
	dbfx.Exec(t, `UPDATE issue SET current_iteration_id=NULL WHERE id=$1`, issueID)
	dbfx.Exec(t, `UPDATE iteration_participation SET current_joined_at=NULL WHERE issue_id=$1`, issueID)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	tx, err := testPool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	if err = iteration.LockWorkspace(ctx, tx, parseUUID(testWorkspaceID)); err != nil {
		t.Fatal(err)
	}
	h := *testHandler
	reached := make(chan struct{}, 1)
	h.TxStarter = iterationFenceBarrierStarter{inner: h.TxStarter, reached: reached}
	result := make(chan *testutil.Response, 1)
	go func() {
		result <- testutil.Call(t, h.DeleteIssue, withURLParam(newRequest("DELETE", "/api/issues/"+issueID, nil).WithContext(ctx), "id", issueID))
	}()
	waitIterationBarrier(t, reached)
	if _, err = tx.Exec(ctx, `SELECT id FROM issue WHERE id=$1 FOR UPDATE NOWAIT`, issueID); err != nil {
		t.Fatalf("issue lock precedes fence: %v", err)
	}
	if _, err = tx.Exec(ctx, `UPDATE issue SET current_iteration_id=$1 WHERE id=$2`, iterationID, issueID); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, `UPDATE iteration_participation SET current_joined_at=clock_timestamp() WHERE issue_id=$1`, issueID); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	(<-result).Want(204)
	if dbfx.Count(t, `SELECT count(*) FROM iteration_event WHERE iteration_id=$1 AND issue_id=$2 AND kind='delete'`, iterationID, issueID) != 1 {
		t.Fatal("late join deletion fact was lost")
	}
}

func TestDeleteIssueIterationRevalidatesAuthority(t *testing.T) {
	for _, change := range []string{"member_revoked", "agent_autonomy", "agent_archived", "task_deleted"} {
		t.Run(change, func(t *testing.T) {
			issueID, iterationID := iterationIssueFixture(t)
			userID := dbfx.User(t, "Current deleting member", "delete-member-"+uuid.NewString()+"@test.invalid")
			dbfx.Member(t, testWorkspaceID, userID, "member")
			r := withURLParam(newRequest("DELETE", "/api/issues/"+issueID, nil), "id", issueID)
			r.Header.Set("X-User-ID", userID)
			var agentID, taskID string
			if change != "member_revoked" {
				agentID = dbfx.Agent(t, "Current deleting agent", handlerTestRuntimeID(t), testutil.Cols{"autonomy_level": "contributor"})
				taskID = dbfx.Task(t, agentID, testutil.Cols{"runtime_id": handlerTestRuntimeID(t), "status": "running"})
				r = asAgent(r, agentID, taskID)
			}
			ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
			defer cancel()
			reached, release := make(chan struct{}, 1), make(chan struct{})
			var once sync.Once
			defer once.Do(func() { close(release) })
			h := *testHandler
			h.TxStarter = iterationBeginBarrier{inner: h.TxStarter, reached: reached, release: release}
			result := make(chan *testutil.Response, 1)
			go func() { result <- testutil.Call(t, h.DeleteIssue, r.WithContext(ctx)) }()
			select {
			case <-reached:
			case response := <-result:
				t.Fatalf("delete stopped before its transaction: %d %s", response.Code, response.Body.String())
			case <-ctx.Done():
				t.Fatal("delete did not reach its transaction")
			}
			switch change {
			case "member_revoked":
				dbfx.Exec(t, `DELETE FROM member WHERE workspace_id=$1 AND user_id=$2`, testWorkspaceID, userID)
			case "agent_autonomy":
				dbfx.Exec(t, `UPDATE agent SET autonomy_level='observer' WHERE id=$1`, agentID)
			case "agent_archived":
				dbfx.Exec(t, `UPDATE agent SET archived_at=clock_timestamp() WHERE id=$1`, agentID)
			case "task_deleted":
				dbfx.Exec(t, `DELETE FROM agent_task_queue WHERE id=$1`, taskID)
			}
			once.Do(func() { close(release) })
			(<-result).Want(403)
			if dbfx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND revision=1`, issueID) != 1 || dbfx.Count(t, `SELECT count(*) FROM iteration_event WHERE iteration_id=$1`, iterationID) != 0 || dbfx.Count(t, `SELECT count(*) FROM iteration_participation WHERE issue_id=$1 AND current_joined_at IS NOT NULL`, issueID) != 1 {
				t.Fatal("revoked delete authority changed issue or iteration facts")
			}
		})
	}
}

func TestDeleteIssueIterationRetainsAgentActor(t *testing.T) {
	issueID, iterationID := iterationIssueFixture(t)
	agentID := dbfx.Agent(t, "Deleting contributor", handlerTestRuntimeID(t), testutil.Cols{"autonomy_level": "contributor"})
	taskID := dbfx.Task(t, agentID, testutil.Cols{"runtime_id": handlerTestRuntimeID(t), "status": "running"})
	r := asAgent(withURLParam(newRequest("DELETE", "/api/issues/"+issueID, nil), "id", issueID), agentID, taskID)
	testutil.Call(t, testHandler.DeleteIssue, r).Want(204)
	if dbfx.Count(t, `SELECT count(*) FROM iteration_event WHERE iteration_id=$1 AND actor->>'type'='agent' AND actor->>'id'=$2 AND actor->>'user_id'=$3`, iterationID, agentID, testUserID) != 1 {
		t.Fatal("delete lost stable agent actor identity")
	}
}

func TestDeleteIssueIterationClampsParticipationLeaveTime(t *testing.T) {
	issueID, iterationID := iterationIssueFixture(t)
	future := time.Now().UTC().Add(24 * time.Hour).Truncate(time.Microsecond)
	dbfx.Insert(t, "iteration_event", testutil.Cols{"workspace_id": testWorkspaceID, "iteration_id": iterationID, "issue_id": issueID, "operation_id": uuid.NewString(), "sequence": 1, "kind": "issue_changed", "actor": `{"type":"system","id":null,"user_id":null,"source":"test_clock"}`, "occurred_at": future, "sampled_at": future})
	testutil.Call(t, testHandler.DeleteIssue, withURLParam(newRequest("DELETE", "/api/issues/"+issueID, nil), "id", issueID)).Want(204)
	var occurred, sampled, left time.Time
	dbfx.QueryRow(t, `SELECT occurred_at,sampled_at FROM iteration_event WHERE iteration_id=$1 AND sequence=2`, iterationID).Scan(&occurred, &sampled)
	dbfx.QueryRow(t, `SELECT last_left_at FROM iteration_participation WHERE issue_id=$1`, issueID).Scan(&left)
	if !occurred.Equal(future) || !left.Equal(occurred) || !sampled.Before(occurred) {
		t.Fatalf("delete/leave timestamp drift: occurred=%s sampled=%s left=%s", occurred, sampled, left)
	}
}

func TestBatchDeleteIssueIterationSamplesAfterAllParticipationLocks(t *testing.T) {
	first, iterationID := iterationIssueFixture(t)
	second := dbfx.Issue(t, "Last locked deletion", testutil.Cols{"current_iteration_id": iterationID})
	dbfx.InsertNoID(t, "iteration_participation", testutil.Cols{"workspace_id": testWorkspaceID, "iteration_id": iterationID, "issue_id": second, "first_joined_at": testutil.Raw("clock_timestamp()"), "current_joined_at": testutil.Raw("clock_timestamp()")}, "iteration_id=$1 AND issue_id=$2", iterationID, second)
	ids := []string{first, second}
	sort.Strings(ids)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	blocker, err := testPool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer blocker.Rollback(context.Background())
	if _, err = blocker.Exec(ctx, `SELECT issue_id FROM iteration_participation WHERE issue_id=$1 FOR UPDATE`, ids[1]); err != nil {
		t.Fatal(err)
	}
	reached := make(chan uint32, 2)
	h := *testHandler
	h.TxStarter = squadIterationObservedStarter{inner: h.TxStarter, statement: "LockIssueIterationParticipation", reached: reached}
	result := make(chan *testutil.Response, 1)
	go func() {
		result <- testutil.Call(t, h.BatchDeleteIssues, newRequest("POST", "/api/issues/batch-delete", map[string]any{"issue_ids": ids}).WithContext(ctx))
	}()
	squadIterationWaitBlocked(t, ctx, reached, result)
	var cutoff time.Time
	dbfx.QueryRow(t, `SELECT clock_timestamp()`).Scan(&cutoff)
	if err = blocker.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	(<-result).Want(200)
	var minimum, maximum time.Time
	dbfx.QueryRow(t, `SELECT min(sampled_at),max(sampled_at) FROM iteration_event WHERE iteration_id=$1`, iterationID).Scan(&minimum, &maximum)
	if minimum.Before(cutoff) || !minimum.Equal(maximum) {
		t.Fatalf("deletion batch sampled before final lock: min=%s max=%s cutoff=%s", minimum, maximum, cutoff)
	}
}
