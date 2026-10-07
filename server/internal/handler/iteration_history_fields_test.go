package handler

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/iteration"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"testing"
	"time"
)

func TestIterationHistoryPriorityLabelsAndCursor(t *testing.T) {
	f := newHistoryFixture(t)
	label := dbfx.Insert(t, "issue_label", testutil.Cols{"workspace_id": testWorkspaceID, "name": "Frozen label", "resource_type": "issue", "color": "#123456"})
	dbfx.InsertNoID(t, "issue_to_label", testutil.Cols{"issue_id": f.issues["A"], "label_id": label}, "issue_id=$1 AND label_id=$2", f.issues["A"], label)
	dbfx.Exec(t, "UPDATE issue SET priority='high' WHERE id=$1", f.issues["A"])
	var page struct {
		Items []iteration.HistoricalIssue `json:"items"`
		Next  *string                     `json:"next_cursor"`
	}
	testutil.Call(t, testHandler.ListIterationIssues, lifecycleHTTPRequest("GET", "iterations/"+f.id+"/issues?priority=high&label_id="+label, f.id, nil)).Want(200).JSON(&page)
	if len(page.Items) != 1 || page.Items[0].Priority == nil || *page.Items[0].Priority != "high" || len(page.Items[0].Labels) != 1 || page.Items[0].Labels[0].Name != "Frozen label" {
		t.Fatalf("filtered capture: %+v", page)
	}
	testutil.Call(t, testHandler.ListIterationIssues, lifecycleHTTPRequest("GET", "iterations/"+f.id+"/issues?limit=1", f.id, nil)).Want(200).JSON(&page)
	if page.Next == nil {
		t.Fatal("missing cursor")
	}
	cursor := *page.Next
	dbfx.Exec(t, "UPDATE issue_label SET name='Renamed' WHERE id=$1", label)
	testutil.Call(t, testHandler.ListIterationIssues, lifecycleHTTPRequest("GET", "iterations/"+f.id+"/issues?limit=1&cursor="+cursor, f.id, nil)).Want(409)
	for _, query := range []string{"priority=bogus", "label_id=null", "label_id=broken"} {
		testutil.Call(t, testHandler.ListIterationIssues, lifecycleHTTPRequest("GET", "iterations/"+f.id+"/issues?"+query, f.id, nil)).Want(400)
	}
	tx, err := testPool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	issue, err := db.New(tx).GetIssue(context.Background(), parseUUID(f.issues["A"]))
	if err != nil {
		t.Fatal(err)
	}
	captured, err := iteration.CaptureHistoricalIssues(context.Background(), tx, parseUUID(testWorkspaceID), []db.Issue{issue}, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(captured[0].Labels) != 1 || captured[0].Labels[0].Name != "Renamed" {
		t.Fatalf("writer capture: %+v", captured)
	}
}

func TestIterationListIssueParticipationRetainsRemoved(t *testing.T) {
	f := newHistoryFixture(t)
	f.join(t, "Late", false, "todo", f.start)
	dbfx.Exec(t, "UPDATE issue SET current_iteration_id=NULL WHERE id=$1", f.issues["Late"])
	dbfx.Exec(t, "UPDATE iteration_participation SET current_joined_at=NULL WHERE issue_id=$1", f.issues["Late"])
	var page struct {
		Items []iteration.Iteration `json:"items"`
	}
	testutil.Call(t, testHandler.ListIterations, lifecycleHTTPRequest("GET", "iterations?issue_id="+f.issues["Late"], "", nil)).Want(200).JSON(&page)
	if len(page.Items) != 1 || page.Items[0].ID != f.id {
		t.Fatalf("removed participation missing: %+v", page)
	}
	other := dbfx.Issue(t, "Never participated", nil)
	testutil.Call(t, testHandler.ListIterations, lifecycleHTTPRequest("GET", "iterations?issue_id="+other, "", nil)).Want(200).JSON(&page)
	if len(page.Items) != 0 {
		t.Fatalf("unrelated periods: %+v", page)
	}
	testutil.Call(t, testHandler.ListIterations, lifecycleHTTPRequest("GET", "iterations?issue_id=broken", "", nil)).Want(400)
}

func TestIterationHistoryFrozenPriorityAndLabelNames(t *testing.T) {
	h, period, _, issue := closureWriterFixture(t)
	label := dbfx.Insert(t, "issue_label", testutil.Cols{"workspace_id": testWorkspaceID, "name": "Closure label", "resource_type": "issue", "color": "#123456"})
	testutil.Call(t, h.AttachLabel, withURLParam(newRequest("POST", "/api/issues/"+issue+"/labels", map[string]any{"label_id": label}), "id", issue)).Want(200)
	testutil.Call(t, h.UpdateIssue, withURLParam(newRequest("PUT", "/api/issues/"+issue, map[string]any{"priority": "high"}), "id", issue)).Want(200)
	testutil.Call(t, h.ApplyIterationOperation, closureWriterRequest(t, h, period, issue, "end", testUserID)).Want(200)
	digest := closureDigest(t, period)
	testutil.Call(t, h.UpdateLabel, withURLParam(newRequest("PUT", "/api/labels/"+label, map[string]any{"name": "Changed"}), "id", label)).Want(200)
	testutil.Call(t, h.DeleteLabel, withURLParam(newRequest("DELETE", "/api/labels/"+label, nil), "id", label)).Want(204)
	testutil.Call(t, h.UpdateIssue, withURLParam(newRequest("PUT", "/api/issues/"+issue, map[string]any{"priority": "low"}), "id", issue)).Want(200)
	testutil.Call(t, h.DeleteIssue, withURLParam(newRequest("DELETE", "/api/issues/"+issue, nil), "id", issue)).Want(204)
	if closureDigest(t, period) != digest {
		t.Fatal("post-close writers changed stored snapshot")
	}
	var page struct {
		Items []iteration.HistoricalIssue `json:"items"`
	}
	testutil.Call(t, h.ListIterationIssues, lifecycleHTTPRequest("GET", "iterations/"+period+"/issues?priority=high&label_id="+label, period, nil)).Want(200).JSON(&page)
	if len(page.Items) != 1 || len(page.Items[0].Labels) != 1 || page.Items[0].Labels[0].Name != "Closure label" {
		t.Fatalf("frozen labels not retained: %+v", page)
	}
	testutil.Call(t, h.ListIterationIssues, lifecycleHTTPRequest("GET", "iterations/"+period+"/issues?scope=original", period, nil)).Want(200).JSON(&page)
	if len(page.Items) != 1 || page.Items[0].Priority == nil || *page.Items[0].Priority != "none" || page.Items[0].Labels == nil || len(page.Items[0].Labels) != 0 {
		t.Fatalf("original was backfilled: %+v", page)
	}
	var periods struct {
		Items []iteration.Iteration `json:"items"`
	}
	testutil.Call(t, h.ListIterations, lifecycleHTTPRequest("GET", "iterations?issue_id="+issue, "", nil)).Want(200).JSON(&periods)
	if len(periods.Items) != 1 || periods.Items[0].ID != period {
		t.Fatalf("deleted participant history lost: %+v", periods)
	}
}

func TestIterationClosureLabelWriterFence(t *testing.T) {
	for _, kind := range []string{"attach", "detach", "rename", "delete", "priority"} {
		for _, writerFirst := range []bool{false, true} {
			name := kind + "/closure_first"
			if writerFirst {
				name = kind + "/writer_first"
			}
			t.Run(name, func(t *testing.T) {
				h, period, _, issue := closureWriterFixture(t)
				label := dbfx.Insert(t, "issue_label", testutil.Cols{"workspace_id": testWorkspaceID, "name": "Concurrent label", "resource_type": "issue", "color": "#123456"})
				if kind != "attach" {
					testutil.Call(t, h.AttachLabel, withURLParam(newRequest("POST", "/api/issues/"+issue+"/labels", map[string]any{"label_id": label}), "id", issue)).Want(200)
				}
				writer := h.AttachLabel
				req := withURLParam(newRequest("POST", "/api/issues/"+issue+"/labels", map[string]any{"label_id": label}), "id", issue)
				status := 200
				switch kind {
				case "detach":
					writer = h.DetachLabel
					req = withURLParams(newRequest("DELETE", "/api/issues/"+issue+"/labels/"+label, nil), "id", issue, "labelId", label)
				case "rename":
					writer = h.UpdateLabel
					req = withURLParam(newRequest("PUT", "/api/labels/"+label, map[string]any{"name": "Renamed concurrent label"}), "id", label)
				case "delete":
					writer = h.DeleteLabel
					req = withURLParam(newRequest("DELETE", "/api/labels/"+label, nil), "id", label)
					status = 204
				case "priority":
					writer = h.UpdateIssue
					req = withURLParam(newRequest("PUT", "/api/issues/"+issue, map[string]any{"priority": "high"}), "id", issue)
				}
				actor := dbfx.User(t, "Closure field actor", uuid.NewString()+"@test.invalid")
				dbfx.Member(t, testWorkspaceID, actor, "admin")
				closeReq := closureWriterRequest(t, h, period, issue, "end", actor)
				ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
				defer cancel()
				owner, err := testPool.Begin(ctx)
				if err != nil {
					t.Fatal(err)
				}
				defer owner.Rollback(context.Background())
				if err = iteration.LockWorkspace(ctx, owner, parseUUID(testWorkspaceID)); err != nil {
					t.Fatal(err)
				}
				pid := int(owner.Conn().PgConn().PID())
				closes, writes := make(chan *testutil.Response, 1), make(chan *testutil.Response, 1)
				startClose := func() {
					go func() { closes <- testutil.Call(t, h.ApplyIterationOperation, closureRequestContext(closeReq, ctx)) }()
				}
				startWrite := func() { go func() { writes <- testutil.Call(t, writer, closureRequestContext(req, ctx)) }() }
				if writerFirst {
					startWrite()
				} else {
					startClose()
				}
				closureWaitBlocked(t, ctx, pid, 1)
				if writerFirst {
					startClose()
				} else {
					startWrite()
				}
				closureWaitBlocked(t, ctx, pid, 2)
				if err = owner.Commit(ctx); err != nil {
					t.Fatal(err)
				}
				written, closed := <-writes, <-closes
				written.Want(status)
				if writerFirst {
					closed.Want(409)
					if dbfx.Count(t, "SELECT count(*) FROM iteration_snapshot WHERE iteration_id=$1", period) != 0 {
						t.Fatal("stale close wrote snapshot")
					}
				} else {
					closed.Want(200)
					var detail struct {
						Snapshot iteration.Snapshot `json:"snapshot"`
					}
					testutil.Call(t, h.GetIteration, lifecycleHTTPRequest("GET", "iterations/"+period, period, nil)).Want(200).JSON(&detail)
					if len(detail.Snapshot.Scope) != 1 {
						t.Fatalf("scope: %+v", detail)
					}
					frozen := detail.Snapshot.Scope[0]
					if frozen.Priority == nil || *frozen.Priority != "none" {
						t.Fatalf("priority crossed closure: %+v", frozen)
					}
					if kind == "attach" {
						if len(frozen.Labels) != 0 {
							t.Fatalf("phantom attachment crossed closure: %+v", frozen)
						}
					} else if len(frozen.Labels) != 1 || frozen.Labels[0].Name != "Concurrent label" {
						t.Fatalf("label mutation crossed closure: %+v", frozen)
					}
				}
			})
		}
	}
}

func TestIterationLabelWritesReauthorizeRevokedMember(t *testing.T) {
	for _, kind := range []string{"attach", "rename", "detach", "delete"} {
		t.Run(kind, func(t *testing.T) {
			user := dbfx.User(t, "Revoked label writer", uuid.NewString()+"@test.invalid")
			member := dbfx.Member(t, testWorkspaceID, user, "member")
			issue := dbfx.Issue(t, "Protected labels", nil)
			label := dbfx.Insert(t, "issue_label", testutil.Cols{"workspace_id": testWorkspaceID, "name": "Protected label", "resource_type": "issue", "color": "#123456"})
			if kind == "detach" {
				dbfx.InsertNoID(t, "issue_to_label", testutil.Cols{"issue_id": issue, "label_id": label}, "issue_id=$1 AND label_id=$2", issue, label)
			}
			handler := testHandler.AttachLabel
			req := withURLParam(newRequest("POST", "/api/issues/"+issue+"/labels", map[string]any{"label_id": label}), "id", issue)
			switch kind {
			case "rename":
				handler = testHandler.UpdateLabel
				req = withURLParam(newRequest("PUT", "/api/labels/"+label, map[string]any{"name": "Forbidden rename"}), "id", label)
			case "detach":
				handler = testHandler.DetachLabel
				req = withURLParams(newRequest("DELETE", "/api/issues/"+issue+"/labels/"+label, nil), "id", issue, "labelId", label)
			case "delete":
				handler = testHandler.DeleteLabel
				req = withURLParam(newRequest("DELETE", "/api/labels/"+label, nil), "id", label)
			}
			req.Header.Set("X-User-ID", user)
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()
			owner, err := testPool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer owner.Rollback(context.Background())
			q := db.New(owner)
			if _, err = q.LockWorkspaceForChatSessionCreate(ctx, parseUUID(testWorkspaceID)); err != nil {
				t.Fatal(err)
			}
			if err = q.LockSubscriberWrites(ctx, db.LockSubscriberWritesParams{WorkspaceID: parseUUID(testWorkspaceID), UserID: parseUUID(user)}); err != nil {
				t.Fatal(err)
			}
			if _, err = owner.Exec(ctx, "DELETE FROM member WHERE id=$1", member); err != nil {
				t.Fatal(err)
			}
			result := make(chan *testutil.Response, 1)
			go func() { result <- testutil.Call(t, handler, closureRequestContext(req, ctx)) }()
			closureWaitBlocked(t, ctx, int(owner.Conn().PgConn().PID()), 1)
			if err = owner.Commit(ctx); err != nil {
				t.Fatal(err)
			}
			select {
			case response := <-result:
				response.Want(403)
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
			if dbfx.Count(t, "SELECT count(*) FROM issue_label WHERE id=$1 AND name='Protected label'", label) != 1 {
				t.Fatal("revoked writer changed label")
			}
			expected := 0
			if kind == "detach" {
				expected = 1
			}
			if dbfx.Count(t, "SELECT count(*) FROM issue_to_label WHERE issue_id=$1 AND label_id=$2", issue, label) != expected {
				t.Fatal("revoked writer changed assignment")
			}
		})
	}
}

func TestIterationLabelWritesKeepBoundMachineIdentity(t *testing.T) {
	for _, change := range []string{"archive", "rebind"} {
		t.Run(change, func(t *testing.T) {
			issue := dbfx.Issue(t, "Machine label target", nil)
			runtime := handlerTestRuntimeID(t)
			agent := dbfx.Agent(t, "Label actor", runtime, testutil.Cols{"autonomy_level": "contributor"})
			other := dbfx.Agent(t, "Other label actor", runtime)
			task := dbfx.Task(t, agent, testutil.Cols{"runtime_id": runtime, "issue_id": issue, "status": "running"})
			label := dbfx.Insert(t, "issue_label", testutil.Cols{"workspace_id": testWorkspaceID, "name": "Bound label", "resource_type": "issue", "color": "#123456"})
			req := withURLParam(newRequest("POST", "/api/issues/"+issue+"/labels", map[string]any{"label_id": label}), "id", issue)
			req.Header.Set("X-Actor-Source", "task_token")
			req.Header.Set("X-Agent-ID", agent)
			req.Header.Set("X-Task-ID", task)
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()
			owner, err := testPool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer owner.Rollback(context.Background())
			if err = iteration.LockWorkspace(ctx, owner, parseUUID(testWorkspaceID)); err != nil {
				t.Fatal(err)
			}
			result := make(chan *testutil.Response, 1)
			go func() { result <- testutil.Call(t, testHandler.AttachLabel, closureRequestContext(req, ctx)) }()
			closureWaitBlocked(t, ctx, int(owner.Conn().PgConn().PID()), 1)
			if change == "archive" {
				_, err = owner.Exec(ctx, "UPDATE agent SET archived_at=clock_timestamp() WHERE id=$1", agent)
			} else {
				_, err = owner.Exec(ctx, "UPDATE agent_task_queue SET agent_id=$2 WHERE id=$1", task, other)
			}
			if err != nil {
				t.Fatal(err)
			}
			if err = owner.Commit(ctx); err != nil {
				t.Fatal(err)
			}
			select {
			case response := <-result:
				response.Want(403)
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
			if dbfx.Count(t, "SELECT count(*) FROM issue_to_label WHERE issue_id=$1 AND label_id=$2", issue, label) != 0 {
				t.Fatal("changed machine identity wrote labels")
			}
		})
	}
}

func TestIterationDeletionEventKeepsMembershipTransition(t *testing.T) {
	h, period, _, issue := closureWriterFixture(t)
	testutil.Call(t, h.DeleteIssue, withURLParam(newRequest("DELETE", "/api/issues/"+issue, nil), "id", issue)).Want(204)
	var before, after []byte
	dbfx.QueryRow(t, "SELECT before_facts,after_facts FROM iteration_event WHERE iteration_id=$1 AND issue_id=$2 AND kind='delete'", period, issue).Scan(&before, &after)
	var facts map[string]json.RawMessage
	if err := json.Unmarshal(before, &facts); err != nil {
		t.Fatal(err)
	}
	want, _ := json.Marshal(period)
	if string(facts["source_iteration_id"]) != string(want) || string(facts["target_iteration_id"]) != "null" || string(after) != "null" {
		t.Fatalf("delete transition before=%s after=%s", before, after)
	}
	var detail struct {
		Statistics iteration.Statistics `json:"statistics"`
	}
	testutil.Call(t, h.GetIteration, lifecycleHTTPRequest("GET", "iterations/"+period, period, nil)).Want(200).JSON(&detail)
	if detail.Statistics.Current != 0 || detail.Statistics.Original != 1 || detail.Statistics.RemovedEvents != 1 {
		t.Fatalf("transition metadata altered reducer: %+v", detail)
	}
}
