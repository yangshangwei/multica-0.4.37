package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/multica-ai/multica/server/internal/iteration"
)

// This is a protocol proof for a proposed optimization, not an alternate
// production writer. Keep statements separate: a single SQL CTE would capture
// one pre-wait snapshot and would not satisfy the same contract.
func TestIterationPipelinePostFenceSnapshot(t *testing.T) {
	issueID, iterationID := iterationIssueFixture(t)
	dbfx.Exec(t, "UPDATE issue SET current_iteration_id=NULL WHERE id=$1", issueID)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	blocker, err := testPool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer blocker.Rollback(context.Background())
	if err := iteration.LockWorkspace(ctx, blocker, parseUUID(testWorkspaceID)); err != nil {
		t.Fatal(err)
	}
	writer, err := testPool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Rollback(context.Background())
	pid := int32(writer.Conn().PgConn().PID())
	type result struct {
		iteration, title string
		err              error
	}
	done := make(chan result, 1)
	go func() {
		batch := &pgx.Batch{}
		batch.Queue(issueWriteFenceSQL[0], parseUUID(testWorkspaceID))
		batch.Queue(issueWriteFenceSQL[1], parseUUID(testWorkspaceID), parseUUID(testUserID))
		batch.Queue(issueWriteFenceSQL[2], parseUUID(testUserID), parseUUID(testWorkspaceID))
		batch.Queue(issueWriteFenceSQL[3], parseUUID(testWorkspaceID))
		batch.Queue(issueWriteFenceSQL[4], parseUUID(testWorkspaceID))
		batch.Queue(`SELECT i.id FROM iteration i JOIN issue x
		 ON x.workspace_id=i.workspace_id AND x.current_iteration_id=i.id
		 WHERE x.workspace_id=$1 AND x.id=$2 FOR UPDATE OF i`, parseUUID(testWorkspaceID), parseUUID(issueID))
		batch.Queue(`SELECT title FROM issue WHERE workspace_id=$1 AND id=$2 FOR UPDATE`, parseUUID(testWorkspaceID), parseUUID(issueID))
		rows := writer.SendBatch(ctx, batch)
		defer rows.Close()
		var out result
		for i := 0; i < 5; i++ {
			if _, out.err = rows.Exec(); out.err != nil {
				done <- out
				return
			}
		}
		if out.err = rows.QueryRow().Scan(&out.iteration); out.err == nil {
			out.err = rows.QueryRow().Scan(&out.title)
		}
		if err := rows.Close(); out.err == nil {
			out.err = err
		}
		done <- out
	}()
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	observed := false
	for !observed {
		select {
		case <-ctx.Done():
			t.Fatal("pipeline did not wait behind the I1 fence")
		case out := <-done:
			t.Fatalf("pipeline completed before blocker release: %+v", out)
		case <-ticker.C:
			err := testPool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity
			 WHERE pid=$1 AND wait_event_type='Lock' AND wait_event='advisory'
			 AND $2::integer=ANY(pg_blocking_pids(pid)))`, pid, int32(blocker.Conn().PgConn().PID())).Scan(&observed)
			if err != nil {
				t.Fatal(err)
			}
		}
	}
	if _, err := blocker.Exec(ctx, "SELECT id FROM issue WHERE id=$1 FOR UPDATE NOWAIT", issueID); err != nil {
		t.Fatalf("pipeline locked issue before waiting fence: %v", err)
	}
	if _, err := blocker.Exec(ctx, "UPDATE issue SET current_iteration_id=$1,title='Joined after queued batch' WHERE id=$2", iterationID, issueID); err != nil {
		t.Fatal(err)
	}
	if err := blocker.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case out := <-done:
		if out.err != nil || out.iteration != iterationID || out.title != "Joined after queued batch" {
			t.Fatalf("post-fence statements used stale membership/facts: %+v", out)
		}
	case <-ctx.Done():
		t.Fatal("pipeline did not finish after blocker committed")
	}
	t.Log("Seven separate statements in one pgx batch: post-fence iteration and issue queries saw the holder's commit; issue lock remained available before release")
}

func TestIssueReadLockSQLMatchesAuthoritativeQueries(t *testing.T) {
	normalize := func(sql string) string {
		var lines []string
		for _, line := range strings.Split(sql, "\n") {
			if !strings.HasPrefix(strings.TrimSpace(line), "--") {
				lines = append(lines, line)
			}
		}
		return strings.TrimSuffix(strings.Join(strings.Fields(strings.Join(lines, " ")), ""), ";")
	}
	for i, query := range []struct{ file, name string }{
		{"iteration.sql", "LockIssueIteration"},
		{"issue.sql", "LockIssueForDescriptionUpdate"},
	} {
		contents, err := os.ReadFile("../../pkg/db/queries/" + query.file)
		if err != nil {
			t.Fatal(err)
		}
		_, sql, found := strings.Cut(string(contents), "-- name: "+query.name+" ")
		if !found {
			t.Fatalf("missing authoritative %s", query.name)
		}
		_, sql, _ = strings.Cut(sql, "\n")
		sql, _, _ = strings.Cut(sql, "\n-- name:")
		if normalize(sql) != normalize(issueReadLockSQL[i]) {
			t.Fatalf("read pipeline %s drifted from %s", query.name, query.file)
		}
	}
}

func TestUpdateIssueIterationReadPipelineLockOrder(t *testing.T) {
	for _, stage := range []string{"join_behind_fence", "iteration_before_issue"} {
		t.Run(stage, func(t *testing.T) {
			issueID, iterationID := iterationIssueFixture(t)
			if stage == "join_behind_fence" {
				dbfx.Exec(t, "UPDATE issue SET current_iteration_id=NULL WHERE id=$1", issueID)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			blocker, err := testPool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer blocker.Rollback(context.Background())
			if stage == "join_behind_fence" {
				err = iteration.LockWorkspace(ctx, blocker, parseUUID(testWorkspaceID))
			} else {
				_, err = blocker.Exec(ctx, "SELECT id FROM iteration WHERE id=$1 FOR UPDATE", iterationID)
			}
			if err != nil {
				t.Fatal(err)
			}
			application := "i1-read-pipeline-" + stage
			h, _ := iterationCapacityHandler(t, application, &iterationCapacityTracer{})
			done := make(chan *httptest.ResponseRecorder, 1)
			go func() {
				response := httptest.NewRecorder()
				request := capacityTitleRequest(testWorkspaceID, issueID, "Read after the lock")
				h.UpdateIssue(response, withURLParam(request.WithContext(ctx), "id", issueID))
				done <- response
			}()
			ticker := time.NewTicker(time.Millisecond)
			defer ticker.Stop()
			observed := false
			for !observed {
				select {
				case <-ctx.Done():
					t.Fatal("HTTP read pipeline did not reach its expected blocker")
				case response := <-done:
					t.Fatalf("HTTP pipeline bypassed blocker: %d %s", response.Code, response.Body.String())
				case <-ticker.C:
					err := testPool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity
					 WHERE application_name=$1 AND wait_event_type='Lock'
					 AND $2::integer=ANY(pg_blocking_pids(pid)))`, application, int32(blocker.Conn().PgConn().PID())).Scan(&observed)
					if err != nil {
						t.Fatal(err)
					}
				}
			}
			if _, err := blocker.Exec(ctx, "SELECT id FROM issue WHERE id=$1 FOR UPDATE NOWAIT", issueID); err != nil {
				t.Fatalf("HTTP pipeline locked issue before its %s lock: %v", stage, err)
			}
			if _, err := blocker.Exec(ctx, "UPDATE issue SET current_iteration_id=$1,title='Committed before read batch' WHERE id=$2", iterationID, issueID); err != nil {
				t.Fatal(err)
			}
			if err := blocker.Commit(ctx); err != nil {
				t.Fatal(err)
			}
			select {
			case response := <-done:
				if response.Code != http.StatusOK {
					t.Fatalf("HTTP pipeline: %d %s", response.Code, response.Body.String())
				}
			case <-ctx.Done():
				t.Fatal("HTTP pipeline did not complete after lock release")
			}
			var count int
			var before string
			dbfx.QueryRow(t, "SELECT count(*),min(before_facts->>'title') FROM iteration_event WHERE iteration_id=$1", iterationID).Scan(&count, &before)
			if count != 1 || before != "Committed before read batch" {
				t.Fatalf("HTTP pipeline used stale before facts: events=%d before=%q", count, before)
			}
		})
	}
}
