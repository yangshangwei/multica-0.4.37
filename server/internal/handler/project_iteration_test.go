package handler

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/multica-ai/multica/server/internal/iteration"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type projectIterationFixtureData struct {
	project                                                                                 string
	issues, iterations                                                                      []string
	unassociated, pending, neighbour, neighbourProject, snapshot, task, automation, trigger string
}

func projectIterationFixture(t *testing.T) projectIterationFixtureData {
	t.Helper()
	first, active := iterationIssueFixture(t)
	project := dbfx.Project(t, "Project iteration detach")
	dbfx.Exec(t, `UPDATE issue SET project_id=$1 WHERE id=$2`, project, first)
	planned := dbfx.Insert(t, "iteration", testutil.Cols{"workspace_id": testWorkspaceID, "name": "Planned detach", "timezone": "UTC", "start_date": "2026-10-15", "end_date": "2026-10-28", "created_by": testUserID})
	data := projectIterationFixtureData{project: project, issues: []string{first}, iterations: []string{active, planned}}
	for _, iterationID := range []string{active, planned} {
		issueID := dbfx.Issue(t, "Another commitment", testutil.Cols{"project_id": project, "current_iteration_id": iterationID, "iteration_rollover_count": 3, "status": "todo"})
		dbfx.InsertNoID(t, "iteration_participation", testutil.Cols{"workspace_id": testWorkspaceID, "iteration_id": iterationID, "issue_id": issueID, "first_joined_at": testutil.Raw("clock_timestamp()"), "current_joined_at": testutil.Raw("clock_timestamp()"), "in_original": iterationID == active, "original_facts": `{"title":"Original project facts"}`}, "iteration_id=$1 AND issue_id=$2", iterationID, issueID)
		dbfx.Cleanup(t, `DELETE FROM iteration_event WHERE iteration_id=$1`, iterationID)
		data.issues = append(data.issues, issueID)
	}
	data.unassociated = dbfx.Issue(t, "Unassociated", testutil.Cols{"project_id": project})
	data.pending = dbfx.Issue(t, "Pending detached too", testutil.Cols{"project_id": project, "admission_status": "pending"})
	foreignWS := dbfx.Workspace(t, "Neighbour iteration workspace", "i1-neighbour-"+uuid.NewString())
	foreign := testutil.New(testPool, foreignWS, testUserID)
	data.neighbourProject = foreign.Project(t, "Project iteration detach")
	data.neighbour = foreign.Issue(t, "Neighbour must be unchanged", testutil.Cols{"project_id": data.neighbourProject})
	historical := dbfx.Insert(t, "iteration", testutil.Cols{"workspace_id": testWorkspaceID, "name": "Frozen previous iteration", "timezone": "UTC", "start_date": "2026-09-01", "end_date": "2026-09-14", "status": "completed", "created_by": testUserID})
	data.snapshot = historical
	dbfx.InsertNoID(t, "iteration_snapshot", testutil.Cols{"workspace_id": testWorkspaceID, "iteration_id": historical, "operation_id": uuid.NewString(), "created_at": testutil.Raw("clock_timestamp()"), "body": `{"project_name":"Frozen project"}`}, "iteration_id=$1", historical)
	agent := dbfx.Agent(t, "Project deletion running agent", handlerTestRuntimeID(t))
	data.task = dbfx.Task(t, agent, testutil.Cols{"issue_id": first, "runtime_id": handlerTestRuntimeID(t), "status": "running", "started_at": testutil.Raw("clock_timestamp()")})
	data.automation = dbfx.Insert(t, "autopilot", testutil.Cols{"workspace_id": testWorkspaceID, "project_id": project, "title": "Project deletion automation", "assignee_id": agent, "created_by_type": "member", "created_by_id": testUserID})
	data.trigger = dbfx.Insert(t, "autopilot_trigger", testutil.Cols{"autopilot_id": data.automation, "kind": "api", "enabled": true})

	return data
}
func TestDeleteProjectIterationFacts(t *testing.T) {
	data := projectIterationFixture(t)
	var taskBefore []byte
	dbfx.QueryRow(t, `SELECT row_to_json(agent_task_queue) FROM agent_task_queue WHERE id=$1`, data.task).Scan(&taskBefore)
	testutil.Call(t, testHandler.DeleteProject, withURLParam(newRequest("DELETE", "/api/projects/"+data.project, map[string]any{"expected_revision": 1}), "id", data.project)).Want(204)
	var events int
	dbfx.QueryRow(t, `SELECT count(*) FROM iteration_event WHERE iteration_id=ANY($1::uuid[])`, data.iterations).Scan(&events)
	if events != len(data.issues) {
		t.Fatalf("project detach must record every current participation, events=%d want=%d", events, len(data.issues))
	}
	if n := dbfx.Count(t, `SELECT count(*) FROM iteration_event WHERE iteration_id=ANY($1::uuid[]) AND before_facts->>'project_id'=$2 AND after_facts->>'project_id' IS NULL`, data.iterations, data.project); n != 3 {
		t.Fatal("detach before/after project facts incomplete")
	}
	for i, id := range data.iterations {
		var scope int64
		dbfx.QueryRow(t, `SELECT scope_revision FROM iteration WHERE id=$1`, id).Scan(&scope)
		want := int64(3)
		if i == 1 {
			want = 2
		}
		if scope != want {
			t.Fatalf("scope revision=%d want=%d", scope, want)
		}
	}
	var originalFacts []byte
	dbfx.QueryRow(t, `SELECT original_facts FROM iteration_participation WHERE iteration_id=$1 AND issue_id=$2`, data.iterations[0], data.issues[0]).Scan(&originalFacts)
	var originalMap map[string]any
	if err := json.Unmarshal(originalFacts, &originalMap); err != nil {
		t.Fatal(err)
	}
	if originalMap["title"] != "Original commitment" || originalMap["status_category"] != "todo" {
		t.Fatalf("frozen original facts changed: %s", originalFacts)
	}

	var times, operations int
	dbfx.QueryRow(t, `SELECT count(DISTINCT sampled_at),count(DISTINCT operation_id) FROM iteration_event WHERE iteration_id=ANY($1::uuid[])`, data.iterations).Scan(&times, &operations)
	if times != 1 || operations != 1 {
		t.Fatalf("batch identity/time split: sampled=%d operations=%d", times, operations)
	}
	for i, issueID := range data.issues {
		var projectID *string
		var pointer string
		var revision, rollover int64
		dbfx.QueryRow(t, `SELECT project_id::text,current_iteration_id::text,revision,iteration_rollover_count FROM issue WHERE id=$1`, issueID).Scan(&projectID, &pointer, &revision, &rollover)
		wantIteration := data.iterations[0]
		wantRollovers := int64(3)
		if i == 0 {
			wantRollovers = 2
		}
		if i == 2 {
			wantIteration = data.iterations[1]
		}
		if projectID != nil || pointer != wantIteration || revision != 2 || rollover != wantRollovers {
			t.Fatalf("detach damaged issue %s: project=%v pointer=%s revision=%d rollover=%d", issueID, projectID, pointer, revision, rollover)
		}
	}
	for _, issueID := range []string{data.unassociated, data.pending} {
		if n := dbfx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND project_id IS NULL AND revision=2`, issueID); n != 1 {
			t.Fatal("unassociated/nonformal project reference not detached")
		}
	}
	if n := dbfx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND project_id=$2 AND revision=1`, data.neighbour, data.neighbourProject); n != 1 {
		t.Fatal("neighbour workspace changed")
	}
	var snapshot, taskAfter, actor []byte
	var original bool
	dbfx.QueryRow(t, `SELECT body FROM iteration_snapshot WHERE iteration_id=$1`, data.snapshot).Scan(&snapshot)
	dbfx.QueryRow(t, `SELECT row_to_json(agent_task_queue) FROM agent_task_queue WHERE id=$1`, data.task).Scan(&taskAfter)
	dbfx.QueryRow(t, `SELECT actor FROM iteration_event WHERE iteration_id=$1 LIMIT 1`, data.iterations[0]).Scan(&actor)
	dbfx.QueryRow(t, `SELECT in_original FROM iteration_participation WHERE iteration_id=$1 AND issue_id=$2`, data.iterations[0], data.issues[0]).Scan(&original)
	var snapshotFacts, identity map[string]any
	if err := json.Unmarshal(snapshot, &snapshotFacts); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(actor, &identity); err != nil {
		t.Fatal(err)
	}
	if snapshotFacts["project_name"] != "Frozen project" || string(taskBefore) != string(taskAfter) || !original || identity["type"] != "member" || identity["id"] != testUserID {
		t.Fatalf("history/execution/actor changed snapshot=%s actor=%s", snapshot, actor)
	}

}

type projectIterationFailureState struct {
	events, samples int
	retry           bool
	operationIDs    []any
	actors          []string
}
type projectIterationFailureStarter struct {
	inner txStarter
	state *projectIterationFailureState
}

func (s projectIterationFailureStarter) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := s.inner.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &projectIterationFailureTx{Tx: tx, state: s.state}, nil
}

type projectIterationFailureTx struct {
	pgx.Tx
	state *projectIterationFailureState
}

func (tx *projectIterationFailureTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if sql == "SELECT clock_timestamp()" {
		tx.state.samples++
	}
	return tx.Tx.QueryRow(ctx, sql, args...)
}
func (tx *projectIterationFailureTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if strings.Contains(sql, "AppendIterationIssueEvent") {
		tx.state.events++
		tx.state.operationIDs = append(tx.state.operationIDs, args[2])
		tx.state.actors = append(tx.state.actors, string(args[5].([]byte)))
		if tx.state.events == 2 {
			if tx.state.retry {
				return pgconn.CommandTag{}, &pgconn.PgError{Code: "40001", Message: "injected second issue conflict"}
			}
			return pgconn.CommandTag{}, errors.New("injected second issue event failure")
		}
	}
	return tx.Tx.Exec(ctx, sql, args...)
}

func TestDeleteProjectIterationSecondEventRollback(t *testing.T) {
	data := projectIterationFixture(t)
	h := *testHandler
	state := &projectIterationFailureState{}
	h.TxStarter = projectIterationFailureStarter{inner: h.TxStarter, state: state}
	testutil.Call(t, h.DeleteProject, withURLParam(newRequest("DELETE", "/api/projects/"+data.project, nil), "id", data.project)).Want(503)
	if state.events != 2 || state.samples != 1 {
		t.Fatalf("failure did not occur inside sampled batch events=%d samples=%d", state.events, state.samples)
	}
	if n := dbfx.Count(t, `SELECT count(*) FROM project WHERE id=$1`, data.project); n != 1 {
		t.Fatal("failed event deleted project")
	}
	for _, id := range append(append([]string{}, data.issues...), data.unassociated, data.pending) {
		if n := dbfx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND project_id=$2 AND revision=1`, id, data.project); n != 1 {
			t.Fatalf("failed batch escaped rollback for issue %s", id)
		}
	}
	if n := dbfx.Count(t, `SELECT count(*) FROM iteration WHERE id=ANY($1::uuid[]) AND scope_revision=1`, data.iterations); n != 2 {
		t.Fatal("scope revisions escaped rollback")
	}
	if n := dbfx.Count(t, `SELECT count(*) FROM iteration_event WHERE iteration_id=ANY($1::uuid[])`, data.iterations); n != 0 {
		t.Fatal("partial iteration events committed")
	}
	if n := dbfx.Count(t, `SELECT count(*) FROM autopilot WHERE id=$1 AND project_id=$2 AND status='active'`, data.automation, data.project); n != 1 {
		t.Fatal("automation pause/detach escaped rollback")
	}
	if n := dbfx.Count(t, `SELECT count(*) FROM autopilot_trigger WHERE id=$1 AND enabled`, data.trigger); n != 1 {
		t.Fatal("automation trigger escaped rollback")
	}
}

func TestDeleteProjectIterationRetryIdentityAndCAS(t *testing.T) {
	data := projectIterationFixture(t)
	testutil.Call(t, testHandler.DeleteProject, withURLParam(newRequest("DELETE", "/api/projects/"+data.project, map[string]any{"expected_revision": 2}), "id", data.project)).Want(409)
	if n := dbfx.Count(t, `SELECT count(*) FROM iteration_event WHERE iteration_id=ANY($1::uuid[])`, data.iterations); n != 0 {
		t.Fatal("stale project revision wrote events")
	}
	h := *testHandler
	state := &projectIterationFailureState{retry: true}
	h.TxStarter = projectIterationFailureStarter{inner: h.TxStarter, state: state}
	testutil.Call(t, h.DeleteProject, withURLParam(newRequest("DELETE", "/api/projects/"+data.project, map[string]any{"expected_revision": 1}), "id", data.project)).Want(204)
	if state.events != 5 || state.samples != 2 {
		t.Fatalf("unexpected retry/sample budget events=%d samples=%d", state.events, state.samples)
	}
	for i := range state.operationIDs {
		if state.operationIDs[i] != state.operationIDs[0] || state.actors[i] != state.actors[0] {
			t.Fatal("operation/actor identity changed on retry")
		}
	}
	if n := dbfx.Count(t, `SELECT count(*) FROM iteration_event WHERE iteration_id=ANY($1::uuid[])`, data.iterations); n != 3 {
		t.Fatalf("retry duplicate facts count=%d", n)
	}
}

func TestDeleteProjectIterationFencePrecedesProjectAndSeesJoin(t *testing.T) {
	data := projectIterationFixture(t)
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
	h.TxStarter = iterationFenceBarrierStarter{inner: h.TxStarter, reached: reached}
	result := make(chan *testutil.Response, 1)
	go func() {
		result <- testutil.Call(t, h.DeleteProject, withURLParam(newRequest("DELETE", "/api/projects/"+data.project, nil).WithContext(ctx), "id", data.project))
	}()
	waitIterationBarrier(t, reached)
	if _, err = tx.Exec(ctx, `SELECT id FROM project WHERE id=$1 FOR UPDATE NOWAIT`, data.project); err != nil {
		t.Fatalf("project lock precedes iteration fence: %v", err)
	}
	if _, err = tx.Exec(ctx, `SELECT id FROM issue WHERE id=$1 FOR UPDATE NOWAIT`, data.unassociated); err != nil {
		t.Fatalf("issue lock precedes iteration fence: %v", err)
	}
	if _, err = tx.Exec(ctx, `UPDATE issue SET current_iteration_id=$1 WHERE id=$2`, data.iterations[0], data.unassociated); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO iteration_participation(workspace_id,iteration_id,issue_id,first_joined_at,current_joined_at) VALUES($1,$2,$3,clock_timestamp(),clock_timestamp())`, testWorkspaceID, data.iterations[0], data.unassociated); err != nil {
		t.Fatal(err)
	}
	dbfx.Cleanup(t, `DELETE FROM iteration_participation WHERE iteration_id=$1 AND issue_id=$2`, data.iterations[0], data.unassociated)
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case response := <-result:
		response.Want(204)
	case <-ctx.Done():
		t.Fatal("deletion failed to resume after join")
	}
	if n := dbfx.Count(t, `SELECT count(*) FROM iteration_event WHERE iteration_id=ANY($1::uuid[])`, data.iterations); n != 4 {
		t.Fatalf("post-fence join lost detach fact count=%d", n)
	}
}

func TestDeleteProjectIterationRefusesForeignReferences(t *testing.T) {
	for _, table := range []string{"issue", "autopilot", "project_resource"} {
		t.Run(table, func(t *testing.T) {
			data := projectIterationFixture(t)
			neighbour, err := testHandler.Queries.GetIssue(context.Background(), parseUUID(data.neighbour))
			if err != nil {
				t.Fatal(err)
			}
			foreignFixture := testutil.New(testPool, uuidToString(neighbour.WorkspaceID), testUserID)
			var foreign string
			switch table {
			case "issue":
				foreign = foreignFixture.Issue(t, "Hidden foreign issue", testutil.Cols{"project_id": data.project})
			case "autopilot":
				agent := foreignFixture.Agent(t, "Foreign automation agent", "")
				foreign = foreignFixture.Insert(t, "autopilot", testutil.Cols{"workspace_id": neighbour.WorkspaceID, "project_id": data.project, "title": "Hidden foreign automation", "assignee_id": agent, "created_by_type": "member", "created_by_id": testUserID})
			case "project_resource":
				foreign = foreignFixture.Insert(t, "project_resource", testutil.Cols{"workspace_id": neighbour.WorkspaceID, "project_id": data.project, "resource_type": "github_repo", "resource_ref": `{"url":"https://example.test/foreign/repo"}`, "label": "Hidden foreign resource"})
			}
			var before []byte
			dbfx.QueryRow(t, "SELECT row_to_json(t) FROM "+table+" t WHERE id=$1", foreign).Scan(&before)
			response := testutil.Call(t, testHandler.DeleteProject, withURLParam(newRequest("DELETE", "/api/projects/"+data.project, nil), "id", data.project)).Want(409)
			if strings.Contains(response.Body.String(), foreign) || strings.Contains(response.Body.String(), "Hidden foreign") {
				t.Fatal("integrity conflict exposed foreign data")
			}
			if dbfx.Count(t, `SELECT count(*) FROM project WHERE id=$1`, data.project) != 1 {
				t.Fatal("foreign reference conflict deleted project")
			}
			var after []byte
			dbfx.QueryRow(t, "SELECT row_to_json(t) FROM "+table+" t WHERE id=$1", foreign).Scan(&after)
			if string(before) != string(after) {
				t.Fatalf("foreign %s row changed despite conflict", table)
			}
			if dbfx.Count(t, `SELECT count(*) FROM iteration_event WHERE iteration_id=ANY($1::uuid[])`, data.iterations) != 0 {
				t.Fatal("integrity conflict wrote facts")
			}
			if dbfx.Count(t, `SELECT count(*) FROM autopilot WHERE id=$1 AND project_id=$2 AND status='active'`, data.automation, data.project) != 1 {
				t.Fatal("integrity conflict changed automation")
			}
		})
	}
}

func TestDeleteProjectIterationExcludesLateAssociation(t *testing.T) {
	data := projectIterationFixture(t)
	incoming := dbfx.Issue(t, "Late project association")
	otherUser := dbfx.User(t, "Another project member", uuid.NewString()+"@test.invalid")
	dbfx.Member(t, testWorkspaceID, otherUser, "member")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	reached, release := make(chan struct{}, 1), make(chan struct{})
	deleting := *testHandler
	deleting.TxStarter = projectAssociationCommitStarter{base: deleting.TxStarter, reached: reached, release: release, once: &sync.Once{}}
	deletion := make(chan *testutil.Response, 1)
	go func() {
		deletion <- testutil.Call(t, deleting.DeleteProject, withURLParam(newRequest("DELETE", "/api/projects/"+data.project, nil).WithContext(ctx), "id", data.project))
	}()
	waitIterationBarrier(t, reached)
	tx, err := testPool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	_, err = testHandler.Queries.WithTx(tx).LockProjectForAssociationNowait(ctx, db.LockProjectForAssociationNowaitParams{ID: parseUUID(data.project), WorkspaceID: parseUUID(testWorkspaceID)})
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "55P03" {
		t.Fatalf("project shared association lock passed deleting project: %v", err)
	}
	if err = tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	waiting := make(chan struct{}, 1)
	writer := *testHandler
	writer.TxStarter = iterationFenceBarrierStarter{inner: writer.TxStarter, reached: waiting}
	request := withURLParam(newRequest("PUT", "/api/issues/"+incoming, map[string]any{"project_id": data.project}).WithContext(ctx), "id", incoming)
	request.Header.Set("X-User-ID", otherUser)
	updated := make(chan *testutil.Response, 1)
	go func() { updated <- testutil.Call(t, writer.UpdateIssue, request) }()
	waitIterationBarrier(t, waiting)
	close(release)
	select {
	case response := <-deletion:
		response.Want(204)
	case <-ctx.Done():
		t.Fatal("project deletion did not finish")
	}
	select {
	case response := <-updated:
		response.Want(400)
	case <-ctx.Done():
		t.Fatal("association did not resume")
	}
	if n := dbfx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND project_id IS NULL AND revision=1`, incoming); n != 1 {
		t.Fatal("late writer restored deleted project")
	}
}
