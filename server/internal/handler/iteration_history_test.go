package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/multica-ai/multica/server/internal/iteration"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type historyFixture struct {
	id       string
	issues   map[string]string
	start    time.Time
	sequence int64
}

func newHistoryFixture(t *testing.T) *historyFixture {
	t.Helper()
	if testHandler == nil {
		t.Skip("database unavailable")
	}
	f := &historyFixture{issues: map[string]string{}, start: time.Date(2026, 10, 5, 15, 0, 0, 0, time.UTC)}
	f.id = dbfx.Insert(t, "iteration", testutil.Cols{"workspace_id": testWorkspaceID, "name": "HG history", "timezone": "Asia/Shanghai", "start_date": "2026-10-05", "end_date": "2026-10-14", "status": "active", "created_by": testUserID, "started_by": testUserID, "started_at": f.start})
	dbfx.Cleanup(t, "DELETE FROM iteration_event WHERE iteration_id=$1", f.id)
	dbfx.Cleanup(t, "DELETE FROM iteration_snapshot WHERE iteration_id=$1", f.id)
	// Planned activity at exactly the start timestamp must not enter the counters.
	f.event(t, "planned_activity", "", nil, f.start)
	f.event(t, "start", "", nil, f.start)
	for _, label := range []string{"A", "B", "C", "D", "E", "F", "G", "H"} {
		f.join(t, label, true, "todo", f.start)
	}
	return f
}

func (f *historyFixture) event(t *testing.T, kind, label string, after any, at time.Time) {
	t.Helper()
	f.sequence++
	raw, err := json.Marshal(after)
	if err != nil {
		t.Fatal(err)
	}
	var issueID any
	if label != "" {
		issueID = f.issues[label]
	}
	dbfx.Insert(t, "iteration_event", testutil.Cols{"workspace_id": testWorkspaceID, "iteration_id": f.id, "sequence": f.sequence, "operation_id": uuid.NewString(), "issue_id": issueID, "kind": kind, "actor": `{"type":"system","id":null,"user_id":null,"source":"hg_fixture"}`, "occurred_at": at, "sampled_at": at, "after_facts": raw})
}

func (f *historyFixture) join(t *testing.T, label string, original bool, status string, at time.Time) {
	t.Helper()
	id := dbfx.Issue(t, label, testutil.Cols{"current_iteration_id": f.id, "status": status})
	f.issues[label] = id
	historical := iteration.HistoricalIssue{IssueID: id, Identifier: "HG-" + label, Title: label, StatusKey: status, StatusCategory: status}
	raw, err := json.Marshal(iteration.OriginalFacts{HistoricalIssue: historical})
	if err != nil {
		t.Fatal(err)
	}
	var originalJSON any
	if original {
		originalJSON = raw
	}
	dbfx.InsertNoID(t, "iteration_participation", testutil.Cols{"workspace_id": testWorkspaceID, "iteration_id": f.id, "issue_id": id, "first_joined_at": at, "current_joined_at": at, "in_original": original, "original_facts": originalJSON}, "iteration_id=$1 AND issue_id=$2", f.id, id)
	kind := "join"
	if original {
		kind = "baseline"
	}
	f.event(t, kind, label, iteration.OriginalFacts{HistoricalIssue: historical}, at)
}

func (f *historyFixture) change(t *testing.T, label, status string, at time.Time) {
	t.Helper()
	if status == "" {
		dbfx.Exec(t, "UPDATE issue SET current_iteration_id=NULL WHERE id=$1", f.issues[label])
		dbfx.Exec(t, "UPDATE iteration_participation SET current_joined_at=NULL,has_started_current_participation=false WHERE iteration_id=$1 AND issue_id=$2", f.id, f.issues[label])
		f.event(t, "leave", label, nil, at)
		return
	}
	started := status == "done" || status == "in_progress" || status == "in_review"
	dbfx.Exec(t, "UPDATE issue SET status=$2 WHERE id=$1", f.issues[label], status)
	dbfx.Exec(t, "UPDATE iteration_participation SET has_started_current_participation=has_started_current_participation OR $3 WHERE iteration_id=$1 AND issue_id=$2", f.id, f.issues[label], started)
	f.event(t, "status", label, iteration.IssueFacts{IssueID: parseUUID(f.issues[label]), Title: label, StatusKey: status, StatusCategory: status, HasStarted: started}, at)
}

func loadHistoryFixture(t *testing.T, f *historyFixture, at time.Time) iteration.History {
	t.Helper()
	tx, err := testPool.BeginTx(context.Background(), pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	history, err := iteration.LoadHistory(context.Background(), tx, parseUUID(testWorkspaceID), parseUUID(f.id), at)
	if err != nil {
		t.Fatal(err)
	}
	return history
}

func TestIterationHistoryPersistedCanonicalFacts(t *testing.T) {
	f := newHistoryFixture(t)
	// A planned deletion and the later baseline can share a clamped timestamp.
	// Only the start sequence determines whether that deletion affects scope.
	dbfx.Exec(t, "UPDATE iteration_event SET kind='delete',issue_id=$2 WHERE iteration_id=$1 AND sequence=1", f.id, f.issues["A"])
	at := f.start.Add(time.Hour)
	for _, label := range []string{"A", "B", "C"} {
		f.change(t, label, "done", at)
	}
	f.join(t, "I", false, "todo", at)
	f.join(t, "J", false, "todo", at)
	f.change(t, "E", "cancelled", at)
	f.change(t, "F", "", at)
	f.change(t, "G", "done", at)
	f.change(t, "I", "done", at)
	dbfx.Exec(t, "DELETE FROM issue WHERE id=$1", f.issues["F"])
	history := loadHistoryFixture(t, f, at)
	s := history.Statistics
	if s.Original != 8 || s.Current != 9 || s.Effective != 8 || s.Completed != 5 || s.OriginalCompleted != 4 || s.Remaining != 3 || s.AddedUnique != 2 || s.RemovedEvents != 1 || s.Started != 5 {
		t.Fatalf("A–J projection: %+v", s)
	}
	if s.EffectiveRatio == nil || *s.EffectiveRatio != .625 || s.OriginalRatio == nil || *s.OriginalRatio != .5 {
		t.Fatalf("ratios: %+v", s)
	}
	if len(s.Chart) != 2 || s.Chart[0].Date != "2026-10-05" || s.Chart[0].Completed != 0 || s.Chart[1].Completed != 5 {
		t.Fatalf("local midnight chart: %+v", s.Chart)
	}
	if len(history.Original) != 8 {
		t.Fatalf("deletion shrank original: %+v", history.Original)
	}
}

func TestIterationHistorySnapshotUsesFrozenDisplayAndNeverLiveFallback(t *testing.T) {
	f := newHistoryFixture(t)
	at := f.start.Add(time.Hour)
	project := dbfx.Project(t, "Original project")
	dbfx.Exec(t, "UPDATE issue SET project_id=$2,assignee_type='member',assignee_id=$3 WHERE id=$1", f.issues["A"], project, testUserID)
	tx, err := testPool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	q := db.New(tx)
	issue, err := q.GetIssue(context.Background(), parseUUID(f.issues["A"]))
	if err != nil {
		t.Fatal(err)
	}
	captured, err := iteration.CaptureHistoricalIssues(context.Background(), tx, parseUUID(testWorkspaceID), []db.Issue{issue}, true)
	if err != nil {
		t.Fatal(err)
	}
	if captured[0].ProjectName == nil || *captured[0].ProjectName != "Original project" || captured[0].AssigneeName == nil || *captured[0].AssigneeName != handlerTestName {
		t.Fatalf("capture: %+v", captured)
	}
	originalJSON, err := json.Marshal(iteration.OriginalFacts{HistoricalIssue: captured[0]})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(context.Background(), "UPDATE iteration_participation SET original_facts=$3 WHERE iteration_id=$1 AND issue_id=$2", f.id, issue.ID, originalJSON); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	dbfx.Exec(t, "UPDATE project SET title='Changed project' WHERE id=$1", project)
	setWorkspaceIssuePrefixForTest(t, "RENAMED")
	dbfx.Cleanup(t, `UPDATE "user" SET name=$2 WHERE id=$1`, testUserID, handlerTestName)
	dbfx.Exec(t, `UPDATE "user" SET name='Changed assignee' WHERE id=$1`, testUserID)
	history := loadHistoryFixture(t, f, at)
	destinations := make([]iteration.Destination, 0, len(history.Scope))
	for _, item := range history.Scope {
		destinations = append(destinations, iteration.Destination{IssueID: item.IssueID, RolloverCountBefore: item.RolloverCount, RolloverCountAfter: item.RolloverCount})
	}
	snapshot, err := iteration.BuildSnapshot(history, uuid.NewString(), "completed", "Completed", at, at, destinations)
	if err != nil {
		t.Fatal(err)
	}
	originalName := ""
	for _, row := range snapshot.Original {
		if row.IssueID == f.issues["A"] && row.ProjectName != nil {
			originalName = *row.ProjectName
			if row.Identifier != captured[0].Identifier || row.AssigneeName == nil || *row.AssigneeName != handlerTestName {
				t.Fatalf("frozen identifier or assignee drifted: %+v", row)
			}
		}
	}
	if originalName != "Original project" {
		t.Fatalf("original drifted: %q", originalName)
	}
	body, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	dbfx.InsertNoID(t, "iteration_snapshot", testutil.Cols{"workspace_id": testWorkspaceID, "iteration_id": f.id, "operation_id": snapshot.OperationID, "body": body, "created_at": at}, "iteration_id=$1", f.id)
	dbfx.Exec(t, "UPDATE iteration SET status='completed',logical_ended_at=$2,processed_at=$2 WHERE id=$1", f.id, at)
	dbfx.Exec(t, "UPDATE issue SET title='Live drift',status='cancelled' WHERE current_iteration_id=$1", f.id)
	dbfx.Exec(t, "UPDATE issue SET project_id=NULL WHERE project_id=$1", project)
	dbfx.Exec(t, "DELETE FROM project WHERE id=$1", project)
	dbfx.Exec(t, "DELETE FROM iteration_participation WHERE iteration_id=$1", f.id)
	frozen := loadHistoryFixture(t, f, at.Add(time.Hour))
	if frozen.Snapshot == nil || frozen.Statistics.Current != 8 || !frozen.Statistics.CalculatedAt.Equal(at) {
		t.Fatalf("closed history recomputed: %+v", frozen)
	}
	f.event(t, "edit", "", map[string]any{"name": "Corrected history name"}, at.Add(time.Minute))
	dbfx.Exec(t, "UPDATE iteration SET name='Corrected history name',revision=revision+1 WHERE id=$1", f.id)
	var audit struct {
		Items []iteration.Event `json:"items"`
	}
	testutil.Call(t, testHandler.ListIterationEvents, historyRequest(f.id, "?after_sequence="+strconv.FormatInt(f.sequence-1, 10))).Want(200).JSON(&audit)
	if len(audit.Items) != 1 || audit.Items[0].Kind != "edit" {
		t.Fatalf("closed correction audit missing: %+v", audit)
	}
	if got := loadHistoryFixture(t, f, at.Add(time.Hour)); len(got.Snapshot.Events) != len(snapshot.Events) || got.Statistics.Current != 8 {
		t.Fatal("post-close metadata changed frozen history")
	}
	dbfx.Exec(t, "UPDATE iteration_snapshot SET body='{}'::jsonb WHERE iteration_id=$1", f.id)
	req := historyRequest(f.id, "")
	testutil.Call(t, testHandler.GetIteration, req).Want(http.StatusServiceUnavailable)
}

func historyRequest(id, query string) *http.Request {
	r := withURLParam(newRequest("GET", "/api/workspaces/"+testWorkspaceID+"/iterations/"+id+query, nil), "id", testWorkspaceID)
	chi.RouteContext(r.Context()).URLParams.Add("iterationID", id)
	return r
}

func historyRequestContext(ctx context.Context, id, query string) *http.Request {
	r := historyRequest(id, query)
	return r.WithContext(context.WithValue(ctx, chi.RouteCtxKey, chi.RouteContext(r.Context())))
}

func TestIterationHistoryPaginationBindsScopeAndCurrentAuthorization(t *testing.T) {
	f := newHistoryFixture(t)
	type issuePage struct {
		Items []iteration.HistoricalIssue `json:"items"`
		Total int                         `json:"total"`
		Next  *string                     `json:"next_cursor"`
	}
	var first, second issuePage
	testutil.Call(t, testHandler.ListIterationIssues, historyRequest(f.id, "?limit=2")).Want(200).JSON(&first)
	if len(first.Items) != 2 || first.Total != 8 || first.Next == nil {
		t.Fatalf("first page: %+v", first)
	}
	testutil.Call(t, testHandler.ListIterationIssues, historyRequest(f.id, "?limit=2&cursor="+*first.Next)).Want(200).JSON(&second)
	if len(second.Items) != 2 || second.Total != 8 || second.Items[0].IssueID <= first.Items[1].IssueID {
		t.Fatalf("unstable keyset page: %+v", second)
	}
	testutil.Call(t, testHandler.ListIterationIssues, historyRequest(f.id, "?limit=2&scope=original&cursor="+*first.Next)).Want(409)
	testutil.Call(t, testHandler.ListIterationEvents, historyRequest(f.id, "?cursor="+*first.Next)).Want(409)
	dbfx.Exec(t, "UPDATE iteration SET scope_revision=scope_revision+1 WHERE id=$1", f.id)
	testutil.Call(t, testHandler.ListIterationIssues, historyRequest(f.id, "?cursor="+*first.Next)).Want(409)
	testutil.Call(t, testHandler.ListIterationIssues, historyRequest(f.id, "?limit=101")).Want(400)
	var eventPage struct {
		Items []iteration.Event `json:"items"`
		Next  *string           `json:"next_cursor"`
	}
	testutil.Call(t, testHandler.ListIterationEvents, historyRequest(f.id, "?limit=2&after_sequence=1")).Want(200).JSON(&eventPage)
	if len(eventPage.Items) != 2 || eventPage.Items[0].Sequence != 2 || eventPage.Next == nil {
		t.Fatalf("events first page: %+v", eventPage)
	}
	cursor := *eventPage.Next
	testutil.Call(t, testHandler.ListIterationEvents, historyRequest(f.id, "?limit=2&after_sequence=1&cursor="+cursor)).Want(200).JSON(&eventPage)
	if len(eventPage.Items) != 2 || eventPage.Items[0].Sequence != 4 {
		t.Fatalf("events keyset page: %+v", eventPage)
	}
	user := dbfx.User(t, "History reader", uuid.NewString()+"@example.invalid")
	member := dbfx.Member(t, testWorkspaceID, user, "member")
	request := historyRequest(f.id, "")
	request.Header.Set("X-User-ID", user)
	testutil.Call(t, testHandler.GetIteration, request).Want(200)
	dbfx.Exec(t, "DELETE FROM member WHERE id=$1", member)
	for _, handler := range []http.HandlerFunc{testHandler.GetIteration, testHandler.ListIterationIssues, testHandler.ListIterationEvents} {
		testutil.Call(t, handler, request).Want(403)
	}
	testutil.Call(t, testHandler.GetIteration, historyRequest(uuid.NewString(), "")).Want(404)
	request = historyRequest(f.id, "")
	request.Header.Set("X-Workspace-ID", uuid.NewString())
	testutil.Call(t, testHandler.GetIteration, request).Want(403)
}

func TestIterationHistoryStartedReentryAndUnknownStatus(t *testing.T) {
	f := newHistoryFixture(t)
	at := f.start.Add(time.Minute)
	f.change(t, "B", "blocked", at)
	dbfx.Exec(t, "UPDATE iteration_participation SET has_started_current_participation=true WHERE iteration_id=$1 AND issue_id=$2", f.id, f.issues["B"])
	f.event(t, "execution_started", "B", iteration.IssueFacts{IssueID: parseUUID(f.issues["B"]), Title: "B", StatusKey: "blocked", StatusCategory: "blocked", HasStarted: true}, at)
	if got := loadHistoryFixture(t, f, at).Statistics.Started; got != 1 {
		t.Fatalf("execution start on blocked missing: %d", got)
	}
	f.change(t, "B", "", at)
	dbfx.Exec(t, "UPDATE issue SET current_iteration_id=$2 WHERE id=$1", f.issues["B"], f.id)
	dbfx.Exec(t, "UPDATE iteration_participation SET current_joined_at=$3,has_started_current_participation=false WHERE iteration_id=$1 AND issue_id=$2", f.id, f.issues["B"], at)
	f.event(t, "reenter", "B", iteration.IssueFacts{IssueID: parseUUID(f.issues["B"]), Title: "B", StatusKey: "blocked", StatusCategory: "blocked"}, at)
	stats := loadHistoryFixture(t, f, at).Statistics
	if stats.Started != 0 || stats.ReentryEvents != 1 || stats.Original != 8 || stats.AddedUnique != 0 {
		t.Fatalf("participation did not reset: %+v", stats)
	}
	dbfx.Exec(t, "UPDATE iteration_event SET after_facts=jsonb_set(after_facts,'{status_category}','\"mystery\"') WHERE iteration_id=$1 AND sequence=$2", f.id, f.sequence)
	testutil.Call(t, testHandler.GetIteration, historyRequest(f.id, "")).Want(503)
}

func TestIterationHistoryClockRollbackUsesLogicalCutoff(t *testing.T) {
	f := newHistoryFixture(t)
	f.change(t, "A", "done", f.start.Add(time.Hour))
	history := loadHistoryFixture(t, f, f.start.Add(-time.Hour))
	if !history.Statistics.CalculatedAt.Equal(f.start.Add(time.Hour)) || history.Statistics.Completed != 1 || len(history.Statistics.Chart) != 2 || history.Statistics.Chart[1].Completed != 1 {
		t.Fatalf("clock rollback broke history: %+v", history.Statistics)
	}
}

func TestIterationHistoryCancelledPlanHasNoSnapshot(t *testing.T) {
	id := dbfx.Insert(t, "iteration", testutil.Cols{"workspace_id": testWorkspaceID, "name": "Cancelled planned", "timezone": "UTC", "start_date": "2026-10-05", "end_date": "2026-10-14", "status": "cancelled", "created_by": testUserID})
	var response struct {
		Snapshot   *iteration.Snapshot  `json:"snapshot"`
		Statistics iteration.Statistics `json:"statistics"`
	}
	testutil.Call(t, testHandler.GetIteration, historyRequest(id, "")).Want(200).JSON(&response)
	if response.Snapshot != nil || response.Statistics.Original != 0 || response.Statistics.Current != 0 || len(response.Statistics.Chart) != 0 {
		t.Fatalf("planned cancellation fabricated history: %+v", response)
	}
}

type historyReadPauseStarter struct {
	inner   txStarter
	reached chan<- struct{}
	release <-chan struct{}
}

func (s historyReadPauseStarter) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := s.inner.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &historyReadPauseTx{Tx: tx, reached: s.reached, release: s.release}, nil
}

type historyReadPauseTx struct {
	pgx.Tx
	reached chan<- struct{}
	release <-chan struct{}
}

func (tx *historyReadPauseTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if strings.Contains(sql, "-- name: ListIterationParticipations") {
		tx.reached <- struct{}{}
		select {
		case <-tx.release:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return tx.Tx.Query(ctx, sql, args...)
}

func TestIterationHistoryReadKeepsCoherentSnapshot(t *testing.T) {
	f := newHistoryFixture(t)
	writer := dbfx.User(t, "Concurrent history writer", uuid.NewString()+"@example.invalid")
	dbfx.Member(t, testWorkspaceID, writer, "member")
	reached := make(chan struct{}, 1)
	release := make(chan struct{})
	h := *testHandler
	h.TxStarter = historyReadPauseStarter{inner: h.TxStarter, reached: reached, release: release}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	result := make(chan *testutil.Response, 1)
	go func() { result <- testutil.Call(t, h.GetIteration, historyRequestContext(ctx, f.id, "")) }()
	waitIterationBarrier(t, reached)
	write := withURLParam(newRequest("PUT", "/api/issues/"+f.issues["A"], map[string]any{"status": "done"}).WithContext(ctx), "id", f.issues["A"])
	write.Header.Set("X-User-ID", writer)
	testutil.Call(t, testHandler.UpdateIssue, write).Want(200)
	close(release)
	var old struct {
		Statistics iteration.Statistics `json:"statistics"`
	}
	select {
	case response := <-result:
		response.Want(200).JSON(&old)
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	if old.Statistics.Completed != 0 {
		t.Fatalf("read combined versions across queries: %+v", old)
	}
	testutil.Call(t, testHandler.GetIteration, historyRequest(f.id, "")).Want(200).JSON(&old)
	if old.Statistics.Completed != 1 {
		t.Fatalf("new read missed committed completion: %+v", old)
	}
}

func TestIterationHistoryReadRetriesRevocationSnapshot(t *testing.T) {
	f := newHistoryFixture(t)
	user := dbfx.User(t, "Revoked history reader", uuid.NewString()+"@example.invalid")
	member := dbfx.Member(t, testWorkspaceID, user, "member")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	owner, err := testPool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Rollback(context.Background())
	if err = db.New(owner).LockSubscriberWrites(ctx, db.LockSubscriberWritesParams{WorkspaceID: parseUUID(testWorkspaceID), UserID: parseUUID(user)}); err != nil {
		t.Fatal(err)
	}
	if _, err = owner.Exec(ctx, "DELETE FROM member WHERE id=$1", member); err != nil {
		t.Fatal(err)
	}
	began := make(chan uint32, 3)
	h := *testHandler
	h.TxStarter = operationReadBarrierStarter{inner: h.TxStarter, began: began}
	request := historyRequestContext(ctx, f.id, "")
	request.Header.Set("X-User-ID", user)
	result := make(chan *testutil.Response, 1)
	go func() { result <- testutil.Call(t, h.GetIteration, request) }()
	var pid uint32
	select {
	case pid = <-began:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	waitForOperationReadBlock(t, ctx, pid, owner.Conn().PgConn().PID())
	if err = owner.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case response := <-result:
		response.Want(403)
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
}

func TestIterationHistoryCaptureLocksDisplayReferences(t *testing.T) {
	project := dbfx.Project(t, "Protected project")
	issueID := dbfx.Issue(t, "Capture references", testutil.Cols{"project_id": project, "assignee_type": "member", "assignee_id": testUserID})
	for _, kind := range []string{"workspace", "project", "user"} {
		t.Run(kind, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			holder, err := testPool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer holder.Rollback(context.Background())
			switch kind {
			case "workspace":
				_, err = holder.Exec(ctx, "UPDATE workspace SET issue_prefix='LOCKED' WHERE id=$1", testWorkspaceID)
			case "project":
				_, err = holder.Exec(ctx, "UPDATE project SET title='Locked project' WHERE id=$1", project)
			case "user":
				_, err = holder.Exec(ctx, "UPDATE \"user\" SET name='Locked name' WHERE id=$1", testUserID)
			}
			if err != nil {
				t.Fatal(err)
			}
			tx, err := testPool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(context.Background())
			issue, err := db.New(tx).GetIssue(ctx, parseUUID(issueID))
			if err != nil {
				t.Fatal(err)
			}
			_, err = iteration.CaptureHistoricalIssues(ctx, tx, parseUUID(testWorkspaceID), []db.Issue{issue}, true)
			var conflict *pgconn.PgError
			if !errors.As(err, &conflict) || conflict.Code != "55P03" {
				t.Fatalf("display capture did not use NOWAIT protection: %v", err)
			}
		})
	}
}

func TestIterationHistoryCanonicalThroughLifecycleAndIssueWriters(t *testing.T) {
	ctx := context.Background()
	ws, actor := parseUUID(testWorkspaceID), parseUUID(testUserID)
	dbfx.Exec(t, "UPDATE workspace SET planning_timezone='UTC' WHERE id=$1", testWorkspaceID)
	dbfx.Cleanup(t, "UPDATE workspace SET planning_timezone=NULL WHERE id=$1", testWorkspaceID)
	dbfx.InsertNoID(t, "workspace_iteration_settings", testutil.Cols{"workspace_id": testWorkspaceID, "enabled": true}, "workspace_id=$1", testWorkspaceID)
	for _, table := range []string{"iteration", "iteration_event", "iteration_participation", "iteration_operation"} {
		dbfx.Cleanup(t, "DELETE FROM "+table+" WHERE workspace_id=$1", testWorkspaceID)
	}
	authorize := func(ctx context.Context, tx pgx.Tx) error {
		_, err := db.New(tx).GetMemberByUserAndWorkspace(ctx, db.GetMemberByUserAndWorkspaceParams{WorkspaceID: ws, UserID: actor})
		return err
	}
	svc := service.IterationService{TxStarter: testPool, Available: func(context.Context) bool { return true }, AuthorizeIssues: func(_ context.Context, _ pgx.Tx, issues []db.Issue) error {
		for _, issue := range issues {
			if issue.WorkspaceID != ws {
				return errors.New("foreign issue")
			}
		}
		return nil
	}}
	created, err := svc.Create(ctx, ws, actor, service.CreateIterationInput{RequestID: uuid.NewString(), Name: "HG real lifecycle", StartDate: time.Now().UTC().Format(time.DateOnly), EndDate: time.Now().UTC().AddDate(0, 0, 13).Format(time.DateOnly), ConfirmedTimezone: "UTC"}, authorize)
	if err != nil {
		t.Fatal(err)
	}
	id := created.IterationIDs[0]
	apply := func(draft iteration.Draft) {
		t.Helper()
		preview, err := svc.Preview(ctx, ws, actor, draft, authorize)
		if err != nil {
			t.Fatal(err)
		}
		if len(preview.InvalidItems) != 0 {
			t.Fatalf("invalid lifecycle preview: %+v", preview.InvalidItems)
		}
		if _, err = svc.Apply(ctx, ws, actor, service.ApplyIterationInput{RequestID: uuid.NewString(), Draft: draft, PreviewHash: preview.PreviewHash}, authorize); err != nil {
			t.Fatal(err)
		}
	}
	issues := map[string]string{}
	join := func(labels []string) {
		t.Helper()
		moves := []iteration.Move{}
		for _, label := range labels {
			fields := testutil.Cols{}
			if label == "B" {
				// AC-13: a real parent and child remain two independent IDs.
				fields["parent_issue_id"] = issues["A"]
			}
			issueID := dbfx.Issue(t, label, fields)
			issues[label] = issueID
			moves = append(moves, iteration.Move{IssueID: issueID, ExpectedIssueRevision: 1, TargetID: &id})
		}
		apply(iteration.Draft{Operation: "move", ExpectedSettingsRevision: 1, Moves: moves})
	}
	join([]string{"A", "B", "C", "D", "E", "F", "G", "H"})
	row, err := db.New(testPool).GetIteration(ctx, db.GetIterationParams{WorkspaceID: ws, ID: parseUUID(id)})
	if err != nil {
		t.Fatal(err)
	}
	apply(iteration.Draft{Operation: "start", IterationID: &id, ExpectedIterationRevision: &row.Revision, ExpectedScopeRevision: &row.ScopeRevision, ExpectedSettingsRevision: 1, Moves: []iteration.Move{}, Start: &iteration.StartDraft{TargetID: id, Mode: "scheduled", TerminalChoices: []iteration.TerminalChoice{}}})
	join([]string{"I", "J"})
	for _, label := range []string{"A", "B", "C", "G", "I"} {
		testutil.Call(t, testHandler.UpdateIssue, withURLParam(newRequest("PUT", "/api/issues/"+issues[label], map[string]any{"status": "done"}), "id", issues[label])).Want(200)
	}
	testutil.Call(t, testHandler.UpdateIssue, withURLParam(newRequest("PUT", "/api/issues/"+issues["E"], map[string]any{"status": "cancelled"}), "id", issues["E"])).Want(200)
	removed, err := db.New(testPool).GetIssue(ctx, parseUUID(issues["F"]))
	if err != nil {
		t.Fatal(err)
	}
	reason := "Work moved outside this commitment"
	apply(iteration.Draft{Operation: "move", ExpectedSettingsRevision: 1, Reason: &reason, Moves: []iteration.Move{{IssueID: issues["F"], ExpectedIssueRevision: removed.Revision, ExpectedSourceID: &id, TargetID: nil}}})
	history := loadHistoryFixture(t, &historyFixture{id: id}, time.Now().UTC())
	stats := history.Statistics
	if stats.Original != 8 || stats.Current != 9 || stats.Effective != 8 || stats.Completed != 5 || stats.OriginalCompleted != 4 || stats.AddedUnique != 2 || stats.RemovedEvents != 1 || stats.EffectiveRatio == nil || *stats.EffectiveRatio != .625 || stats.OriginalRatio == nil || *stats.OriginalRatio != .5 {
		t.Fatalf("production lifecycle/writer projection: %+v", stats)
	}
	for _, members := range [][]iteration.HistoricalIssue{history.Original, history.Scope} {
		for _, label := range []string{"A", "B"} {
			count := 0
			for _, member := range members {
				if member.IssueID == issues[label] {
					count++
				}
			}
			if count != 1 {
				t.Fatalf("parent/child %s counted %d times", label, count)
			}
		}
	}
	// AC-12: completing F through the real ordinary writer after it left must
	// not manufacture a source-period event or increase original completion.
	eventsBefore := dbfx.Count(t, "SELECT count(*) FROM iteration_event WHERE iteration_id=$1", id)
	testutil.Call(t, testHandler.UpdateIssue, withURLParam(newRequest("PUT", "/api/issues/"+issues["F"], map[string]any{"status": "done"}), "id", issues["F"])).Want(200)
	history = loadHistoryFixture(t, &historyFixture{id: id}, time.Now().UTC())
	if history.Statistics.OriginalCompleted != 4 || history.Statistics.Completed != 5 || dbfx.Count(t, "SELECT count(*) FROM iteration_event WHERE iteration_id=$1", id) != eventsBefore {
		t.Fatal("external completion rewrote the source commitment")
	}
	if dbfx.Count(t, "SELECT count(*) FROM issue WHERE id=$1 AND status='done' AND current_iteration_id IS NULL", issues["F"]) != 1 {
		t.Fatal("external completion fixture did not complete the unassigned issue")
	}
	testutil.Call(t, testHandler.DeleteIssue, withURLParam(newRequest("DELETE", "/api/issues/"+issues["F"], nil), "id", issues["F"])).Want(204)
	history = loadHistoryFixture(t, &historyFixture{id: id}, time.Now().UTC())
	foundDeleted := false
	for _, original := range history.Original {
		if original.IssueID == issues["F"] {
			foundDeleted = true
			if original.Title != "F" || original.Identifier == "" {
				t.Fatalf("deleted original display missing: %+v", original)
			}
		}
	}
	if !foundDeleted {
		t.Fatal("deleted original lost")
	}
}

func TestIterationHistoryCaptureResolvesArchivedStatusAndTenantReferences(t *testing.T) {
	dbfx.Insert(t, "issue_status", testutil.Cols{"workspace_id": testWorkspaceID, "key": "hg_archived", "name": "Archived review", "category": "in_review", "color": "#ff0000", "position": 20, "archived_at": testutil.Raw("clock_timestamp()")})
	otherWorkspace := dbfx.Workspace(t, "Foreign history", uuid.NewString())
	otherProject := dbfx.Project(t, "Foreign project secret", testutil.Cols{"workspace_id": otherWorkspace})
	id := dbfx.Issue(t, "Archived work", testutil.Cols{"status": "hg_archived", "project_id": otherProject})
	read := func(lock bool) ([]iteration.HistoricalIssue, error) {
		t.Helper()
		tx, err := testPool.Begin(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(context.Background())
		issue, err := db.New(tx).GetIssue(context.Background(), parseUUID(id))
		if err != nil {
			t.Fatal(err)
		}
		return iteration.CaptureHistoricalIssues(context.Background(), tx, parseUUID(testWorkspaceID), []db.Issue{issue}, lock)
	}
	items, err := read(false)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].StatusCategory != "in_review" || items[0].StatusKey != "hg_archived" || items[0].ProjectName != nil {
		t.Fatalf("status resolution or tenant boundary failed: %+v", items)
	}
	if _, err = read(true); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("writer captured an unavailable foreign project: %v", err)
	}
	dbfx.Exec(t, "UPDATE issue SET project_id=NULL,status='hg_unknown' WHERE id=$1", id)
	if _, err = read(false); err == nil {
		t.Fatal("unknown live status accepted")
	}
}

func TestIterationHistoryLegacyIdentifierCapture(t *testing.T) {
	for _, tc := range []struct{ name, prefix, want string }{{"My Team", "", "MYT-7"}, {"前端团队", "", "WS-7"}, {"My Team", "CUSTOM", "CUSTOM-7"}} {
		t.Run(tc.want, func(t *testing.T) {
			ws := dbfx.Workspace(t, tc.name, uuid.NewString(), testutil.Cols{"issue_prefix": tc.prefix})
			fx := testutil.New(testPool, ws, testUserID)
			id := fx.Issue(t, "Frozen identifier", testutil.Cols{"number": 7})
			tx, err := testPool.Begin(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(context.Background())
			issue, err := db.New(tx).GetIssue(context.Background(), parseUUID(id))
			if err != nil {
				t.Fatal(err)
			}
			items, err := iteration.CaptureHistoricalIssues(context.Background(), tx, parseUUID(ws), []db.Issue{issue}, true)
			if err != nil {
				t.Fatal(err)
			}
			if len(items) != 1 || items[0].Identifier != tc.want {
				t.Fatalf("legacy identifier capture=%+v want %s", items, tc.want)
			}
		})
	}
}
