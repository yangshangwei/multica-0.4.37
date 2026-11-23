package inboxstate

import (
	"context"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/dbid"
)

type stateFixture struct {
	pool                   *pgxpool.Pool
	fx                     *testutil.Fixture
	workspace, user, issue string
}

func newStateFixture(t *testing.T) stateFixture {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("requires an explicitly allocated state test database")
	}
	config, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal("invalid state test database configuration")
	}
	config.MaxConns = 5
	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		t.Fatal("state test database unavailable")
	}
	t.Cleanup(pool.Close)
	fx := testutil.New(pool, "", "")
	user := fx.User(t, "Inbox state user", "inbox-state-"+uuid.NewString()+"@example.test")
	ws := fx.Workspace(t, "Inbox state", "state-"+uuid.NewString())
	fx.UserID, fx.WorkspaceID = user, ws
	fx.Member(t, ws, user, "owner")
	issue := fx.Issue(t, "Inbox state target")
	fx.Cleanup(t, "DELETE FROM inbox_critical_source WHERE workspace_id=$1", ws)
	fx.Cleanup(t, "DELETE FROM inbox_snooze WHERE workspace_id=$1", ws)
	return stateFixture{pool, fx, ws, user, issue}
}

func (f stateFixture) params(kind string, details []byte) db.CreateInboxItemParams {
	ws := util.MustParseUUID(f.workspace)
	user := util.MustParseUUID(f.user)
	issue := util.MustParseUUID(f.issue)
	return db.CreateInboxItemParams{ID: dbid.NewV7(), WorkspaceID: ws, RecipientType: "member", RecipientID: user, IssueID: issue, Type: kind, Severity: "info", Title: "State notice", Details: details}
}

func TestOrdinaryDeliveryPreservesNumericDetails(t *testing.T) {
	f := newStateFixture(t)
	raw := []byte(`{"fail_pct":50,"failed_runs":2,"total_runs":4,"reason":"threshold","nested":{"enabled":true}}`)
	out, err := Deliver(context.Background(), f.pool, f.params("autopilot_paused", raw))
	if err != nil {
		t.Fatalf("ordinary numeric details delivery failed: %v", err)
	}
	if !out.Created || out.Suppressed {
		t.Fatalf("ordinary delivery=%+v", out)
	}
	var actual, want map[string]any
	if err = json.Unmarshal(out.Item.Details, &actual); err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &want); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(actual, want) {
		t.Fatalf("ordinary details changed values/types: got %#v want %#v", actual, want)
	}

}

func TestScheduledWritersRetainExistingIdentityAndReadState(t *testing.T) {
	for _, kind := range []string{"project_update", "iteration", "triage"} {
		t.Run(kind, func(t *testing.T) {
			f := newStateFixture(t)
			params := f.params(kind, []byte(`{"event":"one","count":2}`))
			if kind != "triage" {
				params.IssueID = pgtype.UUID{}
			}
			first, err := Deliver(context.Background(), f.pool, params)
			if err != nil || !first.Created {
				t.Fatalf("first scheduled delivery: created=%v err=%v", first.Created, err)
			}
			f.fx.Exec(t, "UPDATE inbox_item SET read=true,archived=true WHERE id=$1", first.Item.ID)
			params.Title = "Updated title"
			params.Details = []byte(`{"event":"two","count":3}`)
			second, err := Deliver(context.Background(), f.pool, params)
			if err != nil {
				t.Fatalf("stable scheduled delivery retry failed: %v", err)
			}
			if kind == "triage" {
				if second.Item.ID != first.Item.ID || !second.Item.Read || !second.Item.Archived || second.Item.Title != "Updated title" {
					t.Fatalf("triage update changed retained state: %+v", second.Item)
				}
			} else if second.Created {
				t.Fatal("immutable scheduled identity was inserted again")
			}
			if count := f.fx.Count(t, "SELECT count(*) FROM inbox_item WHERE id=$1", first.Item.ID); count != 1 {
				t.Fatalf("stable delivery rows=%d, want one", count)
			}
		})
	}
}

func (f stateFixture) snooze(t *testing.T, item db.InboxItem) State {
	t.Helper()
	tx, err := f.pool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	state, err := Lock(context.Background(), tx, ForItem(item))
	if err != nil {
		t.Fatal(err)
	}
	// Keep submicrosecond input even on hosts whose clock only reports microseconds.
	until := time.Now().Add(time.Hour).Truncate(time.Microsecond).Add(789 * time.Nanosecond)
	if err = Snooze(context.Background(), tx, &state, until, state.Revision); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	return state
}

func TestCriticalSourceReplayCannotWakeAnotherSnooze(t *testing.T) {
	f := newStateFixture(t)
	comment := f.fx.Comment(t, f.issue, "Direct mention one")
	raw := []byte(`{"mention_scope":"member","comment_id":"` + comment + `"}`)
	params := f.params("mentioned", raw)
	first, err := Deliver(context.Background(), f.pool, params)
	if err != nil || !first.Created {
		t.Fatalf("first critical delivery: created=%v err=%v", first.Created, err)
	}
	state := f.snooze(t, first.Item)
	params.ID = dbid.NewV7()
	replayed, err := Deliver(context.Background(), f.pool, params)
	if err != nil || replayed.Created || !replayed.State.SnoozedUntil.Valid || replayed.State.Revision != state.Revision {
		t.Fatalf("original source broke later snooze: %+v err=%v", replayed, err)
	}
	newComment := f.fx.Comment(t, f.issue, "Direct mention two")
	next, err := Deliver(context.Background(), f.pool, f.params("mentioned", []byte(`{"mention_scope":"member","comment_id":"`+newComment+`"}`)))
	if err != nil || !next.Created || next.Suppressed || next.State.SnoozedUntil.Valid {
		t.Fatalf("new critical source did not wake: %+v err=%v", next, err)
	}
	state = f.snooze(t, next.Item)
	for _, id := range []string{comment, newComment} {
		out, err := Deliver(context.Background(), f.pool, f.params("mentioned", []byte(`{"mention_scope":"member","comment_id":"`+id+`"}`)))
		if err != nil || out.Created || !out.State.SnoozedUntil.Valid || out.State.Revision != state.Revision {
			t.Fatalf("replayed prior source woke again: %+v err=%v", out, err)
		}
	}
	if count := f.fx.Count(t, "SELECT count(*) FROM inbox_critical_source WHERE workspace_id=$1", f.workspace); count != 2 {
		t.Fatalf("critical source claims=%d, want two actual occurrences", count)
	}
}

func TestLegacyCriticalMentionDeliveryAdoptsRetainedRow(t *testing.T) {
	for _, tc := range []struct {
		name      string
		read      bool
		archived  bool
		uppercase bool
	}{
		{name: "unread_active"},
		{name: "read_active", read: true},
		{name: "unread_archived", archived: true},
		{name: "read_archived", read: true, archived: true},
		{name: "uppercase_legacy", read: true, uppercase: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newStateFixture(t)
			f.fx.Cleanup(t, "DELETE FROM inbox_item WHERE workspace_id=$1", f.workspace)
			comment := f.fx.Comment(t, f.issue, "Original legacy direct mention")
			legacyComment := comment
			if tc.uppercase {
				legacyComment = strings.ToUpper(comment)
			}
			details, err := json.Marshal(map[string]any{
				"comment_id": legacyComment,
				"retained":   "original legacy details",
				"count":      3,
				"nested":     map[string]bool{"unchanged": true},
			})
			if err != nil {
				t.Fatal(err)
			}
			id := f.fx.Insert(t, "inbox_item", testutil.Cols{
				"workspace_id": f.workspace, "recipient_type": "member", "recipient_id": f.user,
				"issue_id": f.issue, "type": "mentioned", "severity": "info",
				"title": "Original legacy title", "body": "Original legacy body",
				"actor_type": "member", "actor_id": f.user, "details": string(details),
				"read": tc.read, "archived": tc.archived,
				"created_at": testutil.Raw("now()-interval '2 days'"),
			})
			q := db.New(f.pool)
			before, err := q.GetInboxItem(t.Context(), util.MustParseUUID(id))
			if err != nil {
				t.Fatal(err)
			}
			assertRetained := func() {
				t.Helper()
				after, err := q.GetInboxItem(t.Context(), before.ID)
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(before, after) {
					t.Fatalf("legacy mention changed ID, read/archive state, payload or original time: before=%+v after=%+v", before, after)
				}
			}
			params := f.params("mentioned", []byte(`{"mention_scope":"member","comment_id":"`+comment+`"}`))
			adopted, err := Deliver(t.Context(), f.pool, params)
			if err != nil {
				t.Fatal(err)
			}
			// Postcommit publishers require Created; adoption must not enter
			// their banner-triggering inbox:new path.
			if adopted.Created || adopted.Suppressed || adopted.State.Revision != 0 {
				t.Fatalf("legacy adoption acquired a new delivery or reminder: %+v", adopted)
			}
			assertRetained()
			if n := f.fx.Count(t, "SELECT count(*) FROM inbox_item WHERE workspace_id=$1 AND issue_id=$2 AND type='mentioned'", f.workspace, f.issue); n != 1 {
				t.Fatalf("legacy adoption retained %d mention rows, want one", n)
			}
			if n := f.fx.Count(t, "SELECT count(*) FROM inbox_critical_source WHERE workspace_id=$1 AND recipient_id=$2 AND group_kind='issue' AND group_id=$3 AND notification_type='mentioned' AND source_kind='comment_mention' AND source_id=$4", f.workspace, f.user, f.issue, comment); n != 1 {
				t.Fatalf("legacy adoption recorded %d original-source receipts, want one", n)
			}
			active := before
			if tc.archived {
				seed, err := Deliver(t.Context(), f.pool, f.params("status_changed", []byte(`{}`)))
				if err != nil || !seed.Created {
					t.Fatalf("new active sibling: created=%v err=%v", seed.Created, err)
				}
				active = seed.Item
			}
			state := f.snooze(t, active)
			params.ID = dbid.NewV7()
			replayed, err := Deliver(t.Context(), f.pool, params)
			if err != nil {
				t.Fatal(err)
			}
			if replayed.Created || !replayed.Suppressed || !replayed.State.SnoozedUntil.Valid || replayed.State.Revision != state.Revision || !replayed.State.SnoozedUntil.Time.Equal(state.SnoozedUntil.Time) {
				t.Fatalf("legacy source replay changed a later snooze: %+v", replayed)
			}
			assertRetained()
			if n := f.fx.Count(t, "SELECT count(*) FROM inbox_item WHERE workspace_id=$1 AND issue_id=$2 AND type='mentioned'", f.workspace, f.issue); n != 1 {
				t.Fatalf("re-snooze replay retained %d mention rows, want one", n)
			}
			if n := f.fx.Count(t, "SELECT count(*) FROM inbox_critical_source WHERE workspace_id=$1 AND recipient_id=$2 AND group_kind='issue' AND group_id=$3 AND notification_type='mentioned' AND source_kind='comment_mention' AND source_id=$4", f.workspace, f.user, f.issue, comment); n != 1 {
				t.Fatalf("re-snooze replay recorded %d original-source receipts, want one", n)
			}
		})
	}
}

func TestBroadMentionsRemainOrdinaryAndSuppressed(t *testing.T) {
	f := newStateFixture(t)
	first, err := Deliver(context.Background(), f.pool, f.params("status_changed", []byte(`{}`)))
	if err != nil {
		t.Fatal(err)
	}
	state := f.snooze(t, first.Item)
	for _, scope := range []string{"all", "squad", ""} {
		comment := f.fx.Comment(t, f.issue, "Broad mention")
		out, err := Deliver(context.Background(), f.pool, f.params("mentioned", []byte(`{"mention_scope":"`+scope+`","comment_id":"`+comment+`"}`)))
		if err != nil || !out.Created || !out.Suppressed || out.State.Revision != state.Revision {
			t.Fatalf("broad mention scope=%q changed snooze: %+v err=%v", scope, out, err)
		}
	}
	if count := f.fx.Count(t, "SELECT count(*) FROM inbox_critical_source WHERE workspace_id=$1", f.workspace); count != 0 {
		t.Fatal("broadcast recipient expansion gained direct wake authority")
	}
}

func TestLegacyFailureAndDescriptionWatermarksSuppressOldSources(t *testing.T) {
	f := newStateFixture(t)
	agent := f.fx.Agent(t, "Failure source", "", testutil.Cols{})
	oldTask := f.fx.Task(t, agent, testutil.Cols{"issue_id": f.issue, "status": "failed", "completed_at": testutil.Raw("now()-interval '1 hour'")})
	f.fx.Exec(t, "UPDATE issue SET revision=4 WHERE id=$1", f.issue)
	seed, err := Deliver(context.Background(), f.pool, f.params("status_changed", []byte(`{}`)))
	if err != nil {
		t.Fatal(err)
	}
	state := f.snooze(t, seed.Item)
	for _, params := range []db.CreateInboxItemParams{
		f.params("task_failed", []byte(`{"task_id":"`+oldTask+`"}`)),
		f.params("issue_assigned", []byte(`{"source_revision":"4"}`)),
		f.params("mentioned", []byte(`{"mention_scope":"member","source_kind":"issue_description_mention","source_revision":"4"}`)),
	} {
		out, err := Deliver(context.Background(), f.pool, params)
		if err != nil || out.Created || out.State.Revision != state.Revision {
			t.Fatalf("old unkeyed source gained new delivery: %+v err=%v", out, err)
		}
	}
	f.fx.Exec(t, "UPDATE issue SET revision=5 WHERE id=$1", f.issue)
	updated, err := Deliver(context.Background(), f.pool, f.params("issue_assigned", []byte(`{"source_revision":"5"}`)))
	if err != nil || !updated.Created || updated.State.SnoozedUntil.Valid {
		t.Fatalf("new persisted assignment occurrence did not wake: %+v err=%v", updated, err)
	}
	state = f.snooze(t, updated.Item)
	newTask := f.fx.Task(t, agent, testutil.Cols{"issue_id": f.issue, "status": "failed", "completed_at": testutil.Raw("now()")})
	failed, err := Deliver(context.Background(), f.pool, f.params("task_failed", []byte(`{"task_id":"`+newTask+`"}`)))
	if err != nil || !failed.Created || failed.State.SnoozedUntil.Valid || failed.State.Revision != state.Revision+1 {
		t.Fatalf("new failure occurrence did not wake: %+v err=%v", failed, err)
	}
}

func TestConcurrentCriticalDeliveryClaimsOnce(t *testing.T) {
	f := newStateFixture(t)
	comment := f.fx.Comment(t, f.issue, "Concurrent direct mention")
	raw := []byte(`{"mention_scope":"member","comment_id":"` + comment + `"}`)
	ready := make(chan struct{})
	type outcome struct {
		delivery Delivery
		err      error
	}
	results := make(chan outcome, 2)
	for range 2 {
		go func() {
			<-ready
			out, err := Deliver(context.Background(), f.pool, f.params("mentioned", raw))
			results <- outcome{out, err}
		}()
	}
	close(ready)
	created := 0
	for range 2 {
		result := <-results
		if result.err != nil {
			t.Fatal(result.err)
		}
		if result.delivery.Created {
			created++
		}
	}
	if created != 1 || f.fx.Count(t, "SELECT count(*) FROM inbox_item WHERE workspace_id=$1", f.workspace) != 1 || f.fx.Count(t, "SELECT count(*) FROM inbox_critical_source WHERE workspace_id=$1", f.workspace) != 1 {
		t.Fatal("concurrent source replay created multiple retained deliveries")
	}
}

func TestFirstSnoozeAndDeliveryFenceOrders(t *testing.T) {
	for _, first := range []string{"delivery", "snooze"} {
		t.Run(first, func(t *testing.T) {
			f := newStateFixture(t)
			f.fx.Exec(t, "UPDATE issue SET revision=2 WHERE id=$1", f.issue)
			seed, err := Deliver(context.Background(), f.pool, f.params("status_changed", []byte(`{}`)))
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			holder, err := f.pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer holder.Rollback(context.Background())
			state, err := Lock(ctx, holder, ForItem(seed.Item))
			if err != nil {
				t.Fatal(err)
			}
			params := f.params("mentioned", []byte(`{"mention_scope":"member","source_kind":"issue_description_mention","source_revision":"3"}`))
			if first == "delivery" {
				if _, err = DeliverTx(ctx, holder, params); err != nil {
					t.Fatal(err)
				}
				f.fx.Exec(t, "UPDATE issue SET revision=3 WHERE id=$1", f.issue)
			} else {
				if err = Snooze(ctx, holder, &state, time.Now().Add(time.Hour), state.Revision); err != nil {
					t.Fatal(err)
				}
				f.fx.Exec(t, "UPDATE issue SET revision=3 WHERE id=$1", f.issue)
			}
			pid := make(chan int32, 1)
			results := make(chan error, 1)
			go func() {
				tx, err := f.pool.Begin(ctx)
				if err != nil {
					results <- err
					return
				}
				defer tx.Rollback(context.Background())
				var backendPID int32
				if err = tx.QueryRow(ctx, "SELECT pg_backend_pid()").Scan(&backendPID); err != nil {
					results <- err
					return
				}
				pid <- backendPID
				if first == "delivery" {
					current, e := Lock(ctx, tx, ForItem(seed.Item))
					err = e
					if err == nil {
						err = Snooze(ctx, tx, &current, time.Now().Add(time.Hour), current.Revision)
					}
				} else {
					_, err = DeliverTx(ctx, tx, params)
				}
				if err == nil {
					err = tx.Commit(ctx)
				}
				results <- err
			}()
			var waitingPID int32
			select {
			case waitingPID = <-pid:
			case err = <-results:
				t.Fatalf("waiter failed before fence: %v", err)
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
			blocked := false
			for deadline := time.Now().Add(3 * time.Second); time.Now().Before(deadline); {
				var n int
				if err = f.pool.QueryRow(ctx, "SELECT cardinality(pg_blocking_pids($1))", waitingPID).Scan(&n); err != nil {
					t.Fatal(err)
				}
				if n > 0 {
					blocked = true
					break
				}
				time.Sleep(10 * time.Millisecond)
			}
			if !blocked {
				t.Fatal("waiter never reached the real transaction fence")
			}
			if err = holder.Commit(ctx); err != nil {
				t.Fatal(err)
			}
			if err = <-results; err != nil {
				t.Fatal(err)
			}
			current, err := Read(ctx, f.pool, ForItem(seed.Item))
			if err != nil {
				t.Fatal(err)
			}
			if first == "delivery" && !current.SnoozedUntil.Valid {
				t.Fatal("earlier delivery ended a later snooze")
			}
			if first == "snooze" && current.SnoozedUntil.Valid {
				t.Fatal("later genuine delivery failed to end the earlier snooze")
			}
		})
	}
}

func TestSourceTransactionBeforeSnoozeCommitRemainsNew(t *testing.T) {
	for _, kind := range []string{"comment", "failure"} {
		t.Run(kind, func(t *testing.T) {
			f := newStateFixture(t)
			seed, err := Deliver(context.Background(), f.pool, f.params("status_changed", []byte(`{}`)))
			if err != nil {
				t.Fatal(err)
			}
			var taskID string
			if kind == "failure" {
				runtime := f.fx.Runtime(t, "Uncommitted failure runtime")
				agent := f.fx.Agent(t, "Uncommitted failure source", runtime)
				taskID = f.fx.Task(t, agent, testutil.Cols{"issue_id": f.issue, "status": "running", "runtime_id": runtime})
			}
			sourceTx, err := f.pool.Begin(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			defer sourceTx.Rollback(context.Background())
			q := db.New(sourceTx)
			params := f.params("mentioned", nil)
			var occurredAt pgtype.Timestamptz
			if kind == "comment" {
				created, err := q.CreateComment(context.Background(), db.CreateCommentParams{ID: dbid.NewV7(), IssueID: params.IssueID, WorkspaceID: params.WorkspaceID, AuthorType: "member", AuthorID: params.RecipientID, Content: "Mention persisted after snooze", Type: "comment"})
				if err != nil {
					t.Fatal(err)
				}
				f.fx.Cleanup(t, "DELETE FROM comment WHERE id=$1", created.ID)
				occurredAt = created.CreatedAt
				params.Details = []byte(`{"mention_scope":"member","comment_id":"` + util.UUIDToString(created.ID) + `"}`)
			} else {
				id := util.MustParseUUID(taskID)
				failed, err := q.FailAgentTask(context.Background(), db.FailAgentTaskParams{ID: id, Error: pgtype.Text{String: "source transaction", Valid: true}})
				if err != nil {
					t.Fatal(err)
				}
				occurredAt = failed.CompletedAt
				params = f.params("task_failed", []byte(`{"task_id":"`+taskID+`"}`))
			}
			state := f.snooze(t, seed.Item)
			if !occurredAt.Valid || !occurredAt.Time.Before(state.SnoozedAt.Time) {
				t.Fatal("source fixture did not retain the earlier transaction-start timestamp")
			}
			if err = sourceTx.Commit(context.Background()); err != nil {
				t.Fatal(err)
			}
			out, err := Deliver(context.Background(), f.pool, params)
			if err != nil || !out.Created || out.State.SnoozedUntil.Valid {
				t.Fatalf("source committed after snooze was misclassified as historical: %+v err=%v", out, err)
			}
		})
	}
}

func TestNewCriticalDeliveryInvalidatesManualReminderUndo(t *testing.T) {
	f := newStateFixture(t)
	seed, err := Deliver(context.Background(), f.pool, f.params("status_changed", []byte(`{}`)))
	if err != nil {
		t.Fatal(err)
	}
	state := f.snooze(t, seed.Item)
	tx, err := f.pool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	state, err = Lock(context.Background(), tx, ForState(state))
	if err != nil {
		t.Fatal(err)
	}
	if err = Unsnooze(context.Background(), tx, &state, state.Revision); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !state.UndoToken.Valid {
		t.Fatal("manual reminder did not produce its reversible token")
	}
	comment := f.fx.Comment(t, f.issue, "Important new mention after manual reminder")
	out, err := Deliver(context.Background(), f.pool, f.params("mentioned", []byte(`{"mention_scope":"member","comment_id":"`+comment+`"}`)))
	if err != nil || !out.Created {
		t.Fatalf("new critical delivery failed: %v", err)
	}
	if out.State.UndoToken.Valid || out.State.Revision <= state.Revision {
		t.Fatal("manual reminder undo can hide the newer important notification")
	}
}
