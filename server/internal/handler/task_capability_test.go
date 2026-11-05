package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"net/http"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/middleware"
	"github.com/multica-ai/multica/server/internal/testutil"
)

type taskCapabilityFixture struct {
	agent, runtime, task, issue, project, token string
	response                                    AgentTaskResponse
}

func newTaskCapabilityFixture(t *testing.T, key string, version int, batch bool) taskCapabilityFixture {
	t.Helper()
	f := taskCapabilityFixture{}
	f.runtime = dbfx.Runtime(t, "Capability runtime")
	f.agent = dbfx.Agent(t, "Capability agent", f.runtime, testutil.Cols{"template_key": key, "template_version": version, "autonomy_level": "observer"})
	f.project = dbfx.Project(t, "Capability project")
	f.issue = dbfx.Issue(t, "Capability issue", testutil.Cols{"project_id": f.project})
	f.task = dbfx.Task(t, f.agent, testutil.Cols{"runtime_id": f.runtime, "issue_id": f.issue, "originator_user_id": testUserID, "accountable_user_id": testUserID})
	if batch {
		req := newDaemonTokenRequest("POST", "/api/daemon/tasks/claim", map[string]any{"runtime_ids": []string{f.runtime}, "max_tasks": 1, "daemon_id": batchClaimTestDaemonID}, testWorkspaceID, batchClaimTestDaemonID)
		var out struct {
			Tasks []AgentTaskResponse `json:"tasks"`
		}
		testutil.Call(t, testHandler.ClaimTasksByRuntime, req).Want(200).JSON(&out)
		if len(out.Tasks) != 1 {
			t.Fatalf("expected one protected claim, got %d", len(out.Tasks))
		}
		f.response = out.Tasks[0]
	} else {
		req := withURLParam(newDaemonTokenRequest("POST", "/api/daemon/runtimes/"+f.runtime+"/tasks/claim", nil, testWorkspaceID, "capability-test"), "runtimeId", f.runtime)
		var out struct {
			Task *AgentTaskResponse `json:"task"`
		}
		testutil.Call(t, testHandler.ClaimTaskByRuntime, req).Want(200).JSON(&out)
		if out.Task == nil {
			t.Fatal("expected protected claim")
		}
		f.response = *out.Task
	}
	f.token = f.response.AuthToken
	if f.token == "" {
		t.Fatal("claim omitted token")
	}
	dbfx.Cleanup(t, `DELETE FROM task_token WHERE task_id=$1`, f.task)
	return f
}

func taskCapabilityRouter() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Auth(testHandler.Queries, nil, nil))
	r.Use(testHandler.TaskCapabilityGuard)
	r.Use(middleware.RequireWorkspaceMember(testHandler.Queries))
	r.Get("/api/issues", testHandler.ListIssues)
	r.Get("/api/issues/{id}", testHandler.GetIssue)
	r.Get("/api/issues/{id}/comments", testHandler.ListComments)
	r.Get("/api/issues/{id}/timeline", testHandler.ListTimeline)
	r.Post("/api/issues/{id}/comments", testHandler.CreateComment)
	r.Put("/api/issues/{id}", testHandler.UpdateIssue)
	r.Post("/api/issues/batch-update", testHandler.BatchUpdateIssues)
	r.Get("/api/issues/search", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
	r.Get("/api/unknown", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
	return r
}
func capabilityCall(t *testing.T, token, method, path string, body any) *testutil.Response {
	t.Helper()
	req := testutil.JSONRequest(method, path, body)
	req.Header.Set("Authorization", "Bearer "+token)
	// Forged actor and workspace headers must never affect the verified binding.
	req.Header.Set("X-Actor-Source", "member")
	req.Header.Set("X-Workspace-Slug", "different-workspace")
	return testutil.Call(t, taskCapabilityRouter().ServeHTTP, req)
}

func TestTaskCapabilityClaimAndResourceMatrix(t *testing.T) {
	for _, tc := range []struct {
		key     string
		version int
		batch   bool
	}{{"code-reviewer", 2, false}, {"progress-reporter", 2, true}, {"security-reviewer", 2, false}, {"migration-reviewer", 3, true}} {
		t.Run(tc.key, func(t *testing.T) {
			f := newTaskCapabilityFixture(t, tc.key, tc.version, tc.batch)
			if f.response.RemoteMCPDaemonToken != "" || len(f.response.RemoteMCPConnections) > 0 || len(f.response.PluginHookTools) > 0 || f.response.PriorSessionID != "" || len(f.response.ActiveSiblingRuns) > 0 {
				t.Fatal("protected claim exposes broader tools or context")
			}
			if !strings.Contains(f.response.Agent.Instructions, "Task API capability (system") || !strings.Contains(f.response.Agent.Instructions, "including in_review and done") {
				t.Fatal("claim lacks authoritative capability briefing")
			}
			var raw []byte
			dbfx.QueryRow(t, `SELECT api_capability FROM task_token WHERE task_id=$1`, f.task).Scan(&raw)
			c, err := auth.ParseTaskCapability(raw)
			if err != nil {
				t.Fatal(err)
			}
			if c.IssueID != f.issue || c.ProjectID != f.project || c.OriginatorID != testUserID || c.ClaimGeneration < 1 {
				t.Fatalf("wrong frozen capability: %+v", c)
			}
			other := dbfx.Issue(t, "Other same project", testutil.Cols{"project_id": f.project})
			outside := dbfx.Issue(t, "Outside project")
			for _, suffix := range []string{"", "/comments", "/timeline"} {
				capabilityCall(t, f.token, "GET", "/api/issues/"+f.issue+suffix, nil).Want(200)
			}
			var number int
			dbfx.QueryRow(t, `SELECT number FROM issue WHERE id=$1`, f.issue).Scan(&number)
			var prefix string
			dbfx.QueryRow(t, `SELECT issue_prefix FROM workspace WHERE id=$1`, testWorkspaceID).Scan(&prefix)
			capabilityCall(t, f.token, "GET", "/api/issues/"+prefix+"-"+fmt.Sprint(number), nil).Want(200)
			want := 403
			if tc.key == "progress-reporter" {
				want = 200
			}
			capabilityCall(t, f.token, "GET", "/api/issues/"+other, nil).Want(want)
			capabilityCall(t, f.token, "GET", "/api/issues?project_id="+f.project, nil).Want(want)
			for _, path := range []string{"/api/issues/" + outside, "/api/issues", "/api/issues/search", "/api/unknown", "/api/issues?project_id=" + f.project + "&project_id=" + f.project} {
				capabilityCall(t, f.token, "GET", path, nil).Want(403)
			}
			capabilityCall(t, f.token, "PUT", "/api/issues/"+f.issue, map[string]any{"title": "denied"}).Want(403)
			capabilityCall(t, f.token, "POST", "/api/issues/batch-update", map[string]any{"issue_ids": []string{f.issue}, "updates": map[string]any{"status": "done"}}).Want(403)
			capabilityCall(t, f.token, "POST", "/api/issues/"+other+"/comments", map[string]any{"content": "out of scope"}).Want(403)
			capabilityCall(t, f.token, "POST", "/api/issues/"+f.issue+"/comments", map[string]any{"content": "review result"}).Want(201)
		})
	}
}

func TestTaskCapabilityRevocationAndMalformedPolicy(t *testing.T) {
	cases := []struct {
		name, sql string
		arg       any
	}{
		{"terminal", `UPDATE agent_task_queue SET status='cancelled' WHERE id=$1`, nil},
		{"old_generation", `UPDATE agent_task_queue SET claim_generation=claim_generation+1 WHERE id=$1`, nil},
		{"moved_project", `UPDATE issue SET project_id=NULL WHERE id=$1`, nil},
		{"revoked_token", `DELETE FROM task_token WHERE task_id=$1`, nil},
		{"expired", `UPDATE task_token SET expires_at=now()-interval '1 second' WHERE task_id=$1`, nil},
		{"unknown_policy", `UPDATE task_token SET api_capability='{"version":99}' WHERE task_id=$1`, nil},
		{"missing_policy", `UPDATE task_token SET api_capability=NULL WHERE task_id=$1`, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newTaskCapabilityFixture(t, "code-reviewer", 2, false)
			id := f.task
			if tc.name == "moved_project" {
				id = f.issue
			}
			dbfx.Exec(t, tc.sql, id)
			want := 403
			if tc.name == "revoked_token" || tc.name == "expired" {
				want = 401
			}
			capabilityCall(t, f.token, "GET", "/api/issues/"+f.issue, nil).Want(want)
		})
	}
}

func TestTaskCapabilityCommentsStayInertAfterTokenCleanup(t *testing.T) {
	f := newTaskCapabilityFixture(t, "code-reviewer", 2, false)
	target := dbfx.Agent(t, "Mention target", handlerTestRuntimeID(t))
	squad := dbfx.Squad(t, "Return target", target)
	dbfx.Exec(t, `UPDATE issue SET assignee_type='squad',assignee_id=$2 WHERE id=$1`, f.issue, squad)
	var out CommentResponse
	content := "[@target](mention://agent/" + target + ") review result"
	capabilityCall(t, f.token, "POST", "/api/issues/"+f.issue+"/comments", map[string]any{"content": content}).Want(201).JSON(&out)
	if len(out.TriggerOutcomes) != 1 || out.TriggerOutcomes[0].Status != DispatchBlocked || out.TriggerOutcomes[0].ReasonCode != ReasonInvocationNotAllowed {
		t.Fatalf("not an honest blocked outcome: %+v", out.TriggerOutcomes)
	}
	var eligible bool
	dbfx.QueryRow(t, `SELECT dispatch_eligible FROM comment WHERE id=$1`, out.ID).Scan(&eligible)
	if eligible {
		t.Fatal("comment will dispatch on replay")
	}
	before := dbfx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE agent_id=$1`, target)
	dbfx.Exec(t, `DELETE FROM task_token WHERE task_id=$1`, f.task)
	source, err := testHandler.Queries.GetAgentTask(context.Background(), parseUUID(f.task))
	if err != nil {
		t.Fatal(err)
	}
	htarget := source
	htarget.AgentID = parseUUID(target)
	htarget.DeliveredCommentIds = nil
	testHandler.reconcileCommentsOnCompletion(context.Background(), &htarget)
	if after := dbfx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE agent_id=$1`, target); after != before {
		t.Fatal("reconciliation dispatched a protected comment")
	}
}

func TestTaskCapabilityCompletionCommentStaysInert(t *testing.T) {
	f := newTaskCapabilityFixture(t, "progress-reporter", 2, false)
	target := dbfx.Agent(t, "Completion target", handlerTestRuntimeID(t))
	dbfx.Exec(t, `UPDATE agent_task_queue SET status='running',started_at=now() WHERE id=$1`, f.task)
	result, _ := json.Marshal(map[string]any{"output": "[@target](mention://agent/" + target + ") report complete"})
	if _, err := testHandler.TaskService.CompleteTask(context.Background(), parseUUID(f.task), result, "", "", "", false, "", ""); err != nil {
		t.Fatal(err)
	}
	var eligible bool
	dbfx.QueryRow(t, `SELECT dispatch_eligible FROM comment WHERE source_task_id=$1 AND author_type='agent'`, f.task).Scan(&eligible)
	if eligible {
		t.Fatal("completion generated an executable comment")
	}
}

func TestTaskCapabilityPasswordAuthentication(t *testing.T) {
	f := newTaskCapabilityFixture(t, "code-reviewer", 2, false)
	dbfx.InsertNoID(t, "user_password_credential", testutil.Cols{"user_id": testUserID, "username": "capability-password", "password_hash": "unused-synthetic-hash", "session_version": 1}, "user_id=$1", testUserID)
	dbfx.Exec(t, `UPDATE task_token SET auth_version=1 WHERE task_id=$1`, f.task)
	passwordTestSetup(t)
	capabilityCall(t, f.token, "GET", "/api/issues/"+f.issue, nil).Want(200)
	capabilityCall(t, f.token, "PUT", "/api/issues/"+f.issue, map[string]any{"title": "denied"}).Want(403)
}

func TestTaskCapabilityLegacyRoleUnchanged(t *testing.T) {
	f := newTaskCapabilityFixture(t, "code-reviewer", 1, false)
	other := dbfx.Issue(t, "Legacy other issue")
	capabilityCall(t, f.token, "GET", "/api/issues/"+other, nil).Want(200)
	capabilityCall(t, f.token, "PUT", "/api/issues/"+other, map[string]any{"title": "legacy edit"}).Want(200)
}

func TestTaskCapabilityCurrentOriginatorAuthority(t *testing.T) {
	for _, change := range []string{"membership", "disabled", "invoke"} {
		t.Run(change, func(t *testing.T) {
			f := newTaskCapabilityFixture(t, "code-reviewer", 2, false)
			origin := dbfx.User(t, "Capability originator", "cap-origin-"+f.agent+"@example.test")
			dbfx.Member(t, testWorkspaceID, origin, "member")
			dbfx.Exec(t, `UPDATE agent SET permission_mode='public_to' WHERE id=$1`, f.agent)
			target := dbfx.Insert(t, "agent_invocation_target", testutil.Cols{"agent_id": f.agent, "target_type": "member", "target_id": origin})
			dbfx.Exec(t, `UPDATE agent_task_queue SET originator_user_id=$2,accountable_user_id=$2 WHERE id=$1`, f.task, origin)
			dbfx.Exec(t, `UPDATE task_token SET api_capability=jsonb_set(api_capability,'{originator_id}',to_jsonb($2::text)) WHERE task_id=$1`, f.task, origin)
			capabilityCall(t, f.token, "GET", "/api/issues/"+f.issue, nil).Want(200)
			switch change {
			case "membership":
				dbfx.Exec(t, `DELETE FROM member WHERE user_id=$1 AND workspace_id=$2`, origin, testWorkspaceID)
			case "disabled":
				dbfx.Exec(t, `UPDATE "user" SET disabled_at=now() WHERE id=$1`, origin)
			case "invoke":
				dbfx.Exec(t, `DELETE FROM agent_invocation_target WHERE id=$1`, target)
			}
			capabilityCall(t, f.token, "GET", "/api/issues/"+f.issue, nil).Want(403)
			capabilityCall(t, f.token, "POST", "/api/issues/"+f.issue+"/comments", map[string]any{"content": "must not persist"}).Want(403)
			if n := dbfx.Count(t, `SELECT count(*) FROM comment WHERE issue_id=$1`, f.issue); n != 0 {
				t.Fatal("revoked principal wrote comment")
			}
		})
	}
}

func TestTaskCapabilityCommentDoesNotMutateThreadOrEscalation(t *testing.T) {
	f := newTaskCapabilityFixture(t, "code-reviewer", 2, false)
	parent := dbfx.Insert(t, "comment", testutil.Cols{"issue_id": f.issue, "workspace_id": testWorkspaceID, "author_type": "member", "author_id": testUserID, "content": "Resolved request", "type": "comment", "resolved_at": testutil.Raw("now()"), "resolved_by_type": "member", "resolved_by_id": testUserID})
	fallbackAgent := dbfx.Agent(t, "Deferred escalation", handlerTestRuntimeID(t))
	deferred := dbfx.Task(t, fallbackAgent, testutil.Cols{"status": "deferred", "issue_id": f.issue, "escalation_for_task_id": f.task, "runtime_id": handlerTestRuntimeID(t)})
	capabilityCall(t, f.token, "POST", "/api/issues/"+f.issue+"/comments", map[string]any{"content": "review with attachment", "parent_id": parent, "attachment_ids": []string{"00000000-0000-4000-8000-000000000123"}}).Want(403)
	if n := dbfx.Count(t, `SELECT count(*) FROM comment WHERE issue_id=$1`, f.issue); n != 1 {
		t.Fatal("attachment refusal persisted a comment")
	}
	capabilityCall(t, f.token, "POST", "/api/issues/"+f.issue+"/comments", map[string]any{"content": "plain review", "parent_id": parent}).Want(201)
	var resolved bool
	dbfx.QueryRow(t, `SELECT resolved_at IS NOT NULL FROM comment WHERE id=$1`, parent).Scan(&resolved)
	if !resolved {
		t.Fatal("restricted reply reopened thread")
	}
	var status string
	dbfx.QueryRow(t, `SELECT status FROM agent_task_queue WHERE id=$1`, deferred).Scan(&status)
	if status != "deferred" {
		t.Fatal("restricted reply changed deferred escalation")
	}
}

func TestTaskCapabilityUnsupportedToolsAndNonIssueClaims(t *testing.T) {
	for _, kind := range []string{"mcp", "non_issue"} {
		for _, batch := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/batch_%v", kind, batch), func(t *testing.T) {
				rt := dbfx.Runtime(t, "Unsupported capability runtime")
				cols := testutil.Cols{"template_key": "code-reviewer", "template_version": 2}
				if kind == "mcp" {
					cols["mcp_config"] = []byte(`{"mcpServers":{"unsafe":{"command":"unused"}}}`)
				}
				agent := dbfx.Agent(t, "Unsupported capability agent", rt, cols)
				taskCols := testutil.Cols{"runtime_id": rt, "originator_user_id": testUserID, "accountable_user_id": testUserID}
				if kind != "non_issue" {
					taskCols["issue_id"] = dbfx.Issue(t, "Unsupported tool issue")
				}
				task := dbfx.Task(t, agent, taskCols)
				if batch {
					req := newDaemonTokenRequest("POST", "/api/daemon/tasks/claim", map[string]any{"runtime_ids": []string{rt}, "daemon_id": batchClaimTestDaemonID, "max_tasks": 1}, testWorkspaceID, batchClaimTestDaemonID)
					var out struct {
						Tasks []AgentTaskResponse `json:"tasks"`
					}
					testutil.Call(t, testHandler.ClaimTasksByRuntime, req).Want(200).JSON(&out)
					if len(out.Tasks) != 0 {
						t.Fatal("unsupported claim delivered")
					}
				} else {
					req := withURLParam(newDaemonTokenRequest("POST", "/api/daemon/runtimes/"+rt+"/tasks/claim", nil, testWorkspaceID, "capability-test"), "runtimeId", rt)
					testutil.Call(t, testHandler.ClaimTaskByRuntime, req).Want(403)
				}
				var status string
				dbfx.QueryRow(t, `SELECT status FROM agent_task_queue WHERE id=$1`, task).Scan(&status)
				if status != "failed" && status != "cancelled" {
					t.Fatalf("unsupported claim may loop: %s", status)
				}
				if n := dbfx.Count(t, `SELECT count(*) FROM task_token WHERE task_id=$1`, task); n != 0 {
					t.Fatal("unsupported claim minted credential")
				}
			})
		}
	}
}

func TestTaskCapabilityFinalizationRejectsChangedScopeAndBrokerCredential(t *testing.T) {
	f := newTaskCapabilityFixture(t, "progress-reporter", 2, false)
	ctx := context.Background()
	task, err := testHandler.Queries.GetAgentTask(ctx, parseUUID(f.task))
	if err != nil {
		t.Fatal(err)
	}
	token, err := testHandler.Queries.GetTaskTokenByHash(ctx, auth.HashToken(f.token))
	if err != nil {
		t.Fatal(err)
	}
	params := db.CreateTaskTokenParams{TokenHash: auth.HashToken("mat_synthetic_second_claim"), TaskID: task.ID, AgentID: task.AgentID, WorkspaceID: token.WorkspaceID, UserID: token.UserID, ExpiresAt: token.ExpiresAt, ApiCapability: token.ApiCapability}
	t.Run("broker", func(t *testing.T) {
		_, err := testHandler.TaskService.FinalizeTaskClaim(ctx, task, params, nil, false, nil, db.CreateDaemonTokenParams{})
		if err == nil {
			t.Fatal("restricted task accepted extra daemon credential")
		}
	})
	t.Run("project_move", func(t *testing.T) {
		dbfx.Exec(t, `UPDATE issue SET project_id=NULL WHERE id=$1`, f.issue)
		_, err := testHandler.TaskService.FinalizeTaskClaim(ctx, task, params, nil, false, nil)
		if err == nil {
			t.Fatal("payload scope changed before finalization")
		}
	})
	if n := dbfx.Count(t, `SELECT count(*) FROM task_token WHERE task_id=$1`, f.task); n != 1 {
		t.Fatal("refusal changed token inventory")
	}
}

// Snapshot validation is tied to the handler data. The move occurs after the
// snapshot has read authorization and before it reads the source issue.
func TestTaskCapabilityReadSnapshotAndWriteRecheck(t *testing.T) {
	f := newTaskCapabilityFixture(t, "progress-reporter", 2, false)
	ctx := context.Background()
	token, err := testHandler.Queries.GetTaskTokenByHash(ctx, auth.HashToken(f.token))
	if err != nil {
		t.Fatal(err)
	}
	req := testutil.JSONRequest("GET", "/api/issues/"+f.issue, nil)
	req.Header.Set("X-User-ID", testUserID)
	req.Header.Set("X-Workspace-ID", testWorkspaceID)
	req.Header.Set("X-Actor-Source", "task_token")
	req = req.WithContext(auth.WithVerifiedTaskToken(req.Context(), token))
	testutil.Call(t, func(w http.ResponseWriter, r *http.Request) {
		testHandler.taskCapabilityRead(w, r, func(scoped *Handler, w http.ResponseWriter, r *http.Request) {
			dbfx.Exec(t, `UPDATE issue SET project_id=NULL WHERE id=$1`, f.issue)
			issue, ok := scoped.loadIssueForUser(w, r, f.issue)
			if !ok {
				return
			}
			if uuidToString(issue.ProjectID) != f.project {
				t.Fatal("read consumed resource outside authorized snapshot")
			}
			writeJSON(w, 200, map[string]any{"scope": "original"})
		})
	}, req).Want(200)
	_, err = testHandler.createCapabilityComment(ctx, token, db.CreateCommentParams{IssueID: parseUUID(f.issue), WorkspaceID: parseUUID(testWorkspaceID), AuthorID: parseUUID(f.agent), AuthorType: "agent", Type: "comment", Content: "must be refused", SourceTaskID: parseUUID(f.task)})
	if !errors.Is(err, auth.ErrTaskCapability) {
		t.Fatalf("write failed to recheck moved resource: %v", err)
	}
}

func TestTaskCapabilityNoProjectReporterCannotListWorkspace(t *testing.T) {
	f := newTaskCapabilityFixture(t, "progress-reporter", 2, false)
	dbfx.Exec(t, `UPDATE issue SET project_id=NULL WHERE id=$1`, f.issue)
	dbfx.Exec(t, `UPDATE task_token SET api_capability=jsonb_set(api_capability,'{project_id}','""') WHERE task_id=$1`, f.task)
	capabilityCall(t, f.token, "GET", "/api/issues/"+f.issue, nil).Want(200)
	capabilityCall(t, f.token, "GET", "/api/issues", nil).Want(403)
	capabilityCall(t, f.token, "GET", "/api/issues?project_id="+f.project, nil).Want(403)
}

func TestTaskCapabilityDaemonToolEndpointsCannotBypass(t *testing.T) {
	f := newTaskCapabilityFixture(t, "code-reviewer", 2, false)
	withPluginsV1Flag(t, testHandler, true)
	for _, tc := range []struct {
		method, path string
		handler      http.HandlerFunc
	}{{"POST", "/api/daemon/tasks/" + f.task + "/plugin-hooks", testHandler.InvokeAgentPluginHook}, {"GET", "/api/daemon/tasks/" + f.task + "/plugin-mcp/example/credential", testHandler.ResolvePluginMCPCredential}} {
		req := withURLParam(newDaemonTokenRequest(tc.method, tc.path, map[string]any{"installation_id": "unused", "hook_key": "unused"}, testWorkspaceID, "capability-test"), "id", f.task)
		testutil.Call(t, tc.handler, req).Want(403)
	}
}

func TestTaskCapabilityConcurrentMoveCannotWriteComment(t *testing.T) {
	f := newTaskCapabilityFixture(t, "code-reviewer", 2, false)
	tx, err := testPool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	if _, err = tx.Exec(context.Background(), `UPDATE issue SET project_id=NULL WHERE id=$1`, f.issue); err != nil {
		t.Fatal(err)
	}
	// Auth reads the committed old project, but the actual write must not use
	// that stale decision while another transaction owns the resource move.
	capabilityCall(t, f.token, "POST", "/api/issues/"+f.issue+"/comments", map[string]any{"content": "racing write"}).Want(503)
	if err = tx.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	capabilityCall(t, f.token, "POST", "/api/issues/"+f.issue+"/comments", map[string]any{"content": "stale write"}).Want(403)
	if n := dbfx.Count(t, `SELECT count(*) FROM comment WHERE issue_id=$1`, f.issue); n != 0 {
		t.Fatal("concurrent scope move admitted stale write")
	}
}
