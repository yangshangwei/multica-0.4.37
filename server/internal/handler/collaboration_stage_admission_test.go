package handler

import (
	"net/http"
	"testing"

	"github.com/multica-ai/multica/server/internal/testutil"
)

func TestStageDescendantMoveAndBatchRefuseExactAdmissionCode(t *testing.T) {
	s := stageHTTPFixture(t)
	runtime := s.fx.Runtime(t, "Synthetic stage runtime")
	agent := s.fx.Agent(t, "Synthetic stage agent", runtime)
	task := s.fx.Task(t, agent, testutil.Cols{"runtime_id": runtime, "status": "running", "originator_user_id": s.actor, "accountable_user_id": s.actor})
	parent := s.fx.Issue(t, "Manual child", testutil.Cols{"parent_issue_id": s.issue, "creator_id": s.actor})
	loose := s.fx.Issue(t, "Loose ordinary issue", testutil.Cols{"creator_id": s.actor})
	s.revise(t, true)
	request := func(path string, body any, machine bool) *http.Request {
		r := testutil.JSONRequest(http.MethodPost, path, body)
		testutil.WithHeaders(r, "X-User-ID", s.actor, "X-Workspace-ID", s.fx.WorkspaceID)
		if machine {
			testutil.WithHeaders(r, "X-Agent-ID", agent, "X-Task-ID", task)
		}
		return r
	}
	body := map[string]any{"parent_issue_id": parent, "before_id": nil, "after_id": nil}
	out := testutil.Call(t, s.h.MoveIssue, testutil.WithURLParams(request("/api/issues/"+loose+"/move", body, true), "id", loose)).Want(http.StatusConflict).Map()
	if out["code"] != "stage_proposal_required" {
		t.Fatalf("machine move did not reach stage admission: %v", out)
	}
	var batch BatchUpdateIssuesResponse
	testutil.Call(t, s.h.BatchUpdateIssues, request("/api/issues/batch-update?workspace_id="+s.fx.WorkspaceID,
		map[string]any{"issue_ids": []string{loose}, "updates": map[string]any{"parent_issue_id": parent}}, true)).Want(http.StatusConflict).JSON(&batch)
	if batch.Updated != 0 || len(batch.Results) != 1 || batch.Results[0].State != "rejected" || batch.Results[0].Code != "stage_proposal_required" ||
		s.fx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND parent_issue_id IS NULL`, loose) != 1 {
		t.Fatalf("machine batch did not preserve the refusal and unchanged parent: %+v", batch)
	}
	testutil.Call(t, s.h.MoveIssue, testutil.WithURLParams(request("/api/issues/"+loose+"/move", body, false), "id", loose)).Want(http.StatusOK)
	if s.fx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND parent_issue_id=$2`, loose, parent) != 1 ||
		s.fx.Count(t, `SELECT count(*) FROM collaboration_stage_child WHERE child_issue_id=$1`, loose) != 0 {
		t.Fatal("human move failed or produced an approval mapping")
	}
}

func TestStageDescendantDaemonStartAndWaitRefuseBeforeMutation(t *testing.T) {
	s := stageHTTPFixture(t)
	runtime := s.fx.Runtime(t, "Synthetic stage daemon runtime", testutil.Cols{"daemon_id": "stage-fixture-daemon"})
	agent := s.fx.Agent(t, "Synthetic stage daemon agent", runtime)
	parent := s.fx.Issue(t, "Manual parent", testutil.Cols{"parent_issue_id": s.issue, "creator_id": s.actor})
	s.revise(t, true)
	for _, machine := range []bool{true, false} {
		for _, endpoint := range []string{"start", "waiting-local-directory"} {
			name := "human/" + endpoint
			if machine {
				name = "machine/" + endpoint
			}
			t.Run(name, func(t *testing.T) {
				cols := testutil.Cols{"parent_issue_id": parent, "creator_id": s.actor}
				if machine {
					cols["creator_type"], cols["creator_id"] = "agent", agent
				}
				issue := s.fx.Issue(t, "Synthetic daemon descendant", cols)
				task := s.fx.Task(t, agent, testutil.Cols{"runtime_id": runtime, "issue_id": issue, "status": "dispatched"})
				r := testutil.WithURLParams(newDaemonTokenRequest(http.MethodPost, "/api/daemon/tasks/"+task+"/"+endpoint, nil, s.fx.WorkspaceID, "stage-fixture-daemon"), "taskId", task)
				handler, state := s.h.StartTask, "running"
				if endpoint == "waiting-local-directory" {
					handler, state = s.h.MarkTaskWaitingLocalDirectory, "waiting_local_directory"
				}
				response := testutil.Call(t, handler, r)
				if machine {
					out := response.Want(http.StatusConflict).Map()
					if out["error"] != "stage_child_unapproved" {
						t.Fatalf("wrong stage refusal: %v", out)
					}
					state = "dispatched"
				} else {
					response.Want(http.StatusOK)
				}
				if s.fx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE id=$1 AND status=$2`, task, state) != 1 {
					t.Fatalf("task should remain %s", state)
				}
			})
		}
	}
}
