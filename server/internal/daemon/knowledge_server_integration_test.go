//go:build knowledgeintegration

package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/daemonws"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/handler"
	"github.com/multica-ai/multica/server/internal/middleware"
	"github.com/multica-ai/multica/server/internal/realtime"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	"github.com/multica-ai/multica/server/pkg/agent"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// The build tag keeps default daemon tests DB-free. This acceptance fixture
// never falls back, skips, or connects before validating explicit ownership.
func compositeKnowledgePool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	raw := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if raw == "" {
		t.Fatal("knowledgeintegration requires explicit task-owned DATABASE_URL")
	}
	cfg, err := pgxpool.ParseConfig(raw)
	if err != nil {
		t.Fatal("invalid composite database configuration")
	}
	if !strings.HasPrefix(cfg.ConnConfig.Database, "multica_knowledge_composite_20261101_") {
		t.Fatal("knowledgeintegration refuses a database outside its owned prefix")
	}
	pool, err := pgxpool.NewWithConfig(t.Context(), cfg)
	if err != nil {
		t.Fatal("cannot create composite database pool")
	}
	t.Cleanup(pool.Close)
	var identity string
	if err := pool.QueryRow(t.Context(), "select current_database()").Scan(&identity); err != nil {
		t.Fatal("composite database unavailable")
	}
	if identity != cfg.ConnConfig.Database {
		t.Fatal("composite database identity mismatch")
	}
	t.Logf("database=%s", identity)
	return pool
}

type knowledgeServerCall struct {
	path              string
	request, response []byte
	status            int
}
type knowledgeServerFixture struct {
	*knowledgeDaemonFixture
	pool                                                                           *pgxpool.Pool
	fx                                                                             *testutil.Fixture
	h                                                                              *handler.Handler
	svc                                                                            *service.CollaborationService
	human                                                                          context.Context
	workspace, user, project, runtimeID, agentID, daemonID, token, issueID, chatID string
	proposals                                                                      []protocol.CollaborationKnowledgeProposal
	traffic                                                                        []knowledgeServerCall
	before                                                                         func(string, []byte)
	after                                                                          func(string, []byte)
	drop                                                                           string
	dropped                                                                        bool
}

func newKnowledgeServerFixture(t *testing.T, surface string, ttl ...time.Duration) *knowledgeServerFixture {
	t.Helper()
	t.Setenv("MULTICA_AUTH_MODE", "legacy")
	f := &knowledgeServerFixture{knowledgeDaemonFixture: newKnowledgeDaemonFixture(t), pool: compositeKnowledgePool(t)}
	control := filepath.Join(filepath.Dir(f.bin), "native-control")
	if err := os.WriteFile(control, []byte(f.native), 0600); err != nil {
		t.Fatal(err)
	}
	script, err := os.ReadFile(f.bin)
	if err != nil {
		t.Fatal(err)
	}
	script = bytes.Replace(script, []byte("#!/bin/sh\n"), []byte("#!/bin/sh\nFAKE_NATIVE=$(cat \"$NATIVE_CONTROL\")\n"), 1)
	if err := os.WriteFile(f.bin, script, 0700); err != nil {
		t.Fatal(err)
	}
	f.task.Agent.CustomEnv["NATIVE_CONTROL"] = control
	f.fx = testutil.New(f.pool, "", "")
	suffix := uuid.NewString()
	f.user = f.fx.User(t, "Composite owner", suffix+"@example.test")
	f.workspace = f.fx.Workspace(t, "Composite workspace", "composite-"+suffix, testutil.Cols{"context": "independent workspace base"})
	f.fx.Member(t, f.workspace, f.user, "owner")
	f.fx.UserID, f.fx.WorkspaceID = f.user, f.workspace
	f.project = f.fx.Project(t, "Composite project", testutil.Cols{"description": "independent project base"})
	f.human = auth.WithAuthenticatedPrincipal(context.Background(), auth.AuthenticatedPrincipal{UserID: f.user, Kind: auth.CredentialJWT})
	q := db.New(f.pool)
	f.svc = service.NewCollaborationService(q, f.pool, service.NewIssueService(q, f.pool, nil, nil, nil))
	f.fx.Exec(t, `UPDATE collaboration_execution_gate SET creation_enabled=true WHERE id=1`)
	f.fx.Cleanup(t, `UPDATE collaboration_execution_gate SET creation_enabled=false WHERE id=1`)
	for _, table := range []string{"issue", "issue_collaboration", "issue_participant", "collaboration_session", "collaboration_event", "collaboration_notification", "collaboration_memory_version"} {
		if table == "issue" {
			f.fx.Cleanup(t, `DELETE FROM issue WHERE workspace_id=$1`, f.workspace)
		} else {
			f.fx.Cleanup(t, `DELETE FROM `+table+` WHERE issue_id IN (SELECT id FROM issue WHERE workspace_id=$1)`, f.workspace)
		}
	}
	f.fx.Cleanup(t, `DELETE FROM collaboration_operation WHERE scope_id=$1 OR scope_id IN (SELECT id FROM issue WHERE workspace_id=$1)`, f.workspace)
	// Candidate history and pending invalidations are retained by schema.
	// This exclusive test database intentionally keeps those immutable facts.
	f.fx.Cleanup(t, `DELETE FROM collaboration_knowledge_target WHERE workspace_id=$1`, f.workspace)
	f.fx.Cleanup(t, `DELETE FROM collaboration_account_fence WHERE user_id=$1`, f.user)
	// Project first, workspace second, with deliberately reversed lexical IDs.
	// First disclosure and original-key replay must have the same canonical order.
	for i, kind := range []string{"project", "workspace"} {
		source, err := f.svc.Create(f.human, service.CollaborationCreateParams{SourceWorkspaceID: util.MustParseUUID(f.workspace), HandlerID: util.MustParseUUID(f.user), RequestID: uuid.NewString(), Title: fmt.Sprintf("Composite source %d", i)})
		if err != nil {
			t.Fatal(err)
		}
		text := fmt.Sprintf("composite source material %d", i)
		if _, err = f.svc.UpdateMemory(f.human, service.CollaborationMemoryUpdateParams{IssueID: source.Issue.ID, RequestID: uuid.NewString(), ExpectedVersion: 0, Content: text}); err != nil {
			t.Fatal(err)
		}
		id := uuid.NewString()
		if i == 0 {
			id = "f" + id[1:]
		} else {
			id = "0" + id[1:]
		}
		target := protocol.CollaborationKnowledgeTarget{Kind: kind, WorkspaceID: f.workspace}
		if kind == "project" {
			target.ProjectID = f.project
		}
		p := protocol.CollaborationKnowledgeProposal{ProtocolVersion: 1, CandidateID: id, RequestID: uuid.NewString(), Target: target, Summary: fmt.Sprintf("composite private summary %d", i), Content: fmt.Sprintf("%s-%d", privateKnowledgeTestBody, i), Sources: []protocol.CollaborationKnowledgeSource{{Kind: "memory", IssueID: util.UUIDToString(source.Issue.ID), Version: "1", SHA256: protocol.CollaborationKnowledgeHash(text)}}, Evidence: []protocol.CollaborationKnowledgeEvidence{}, Applicability: protocol.CollaborationKnowledgeApplicability{Surfaces: []string{"issue", "chat", "quick_create", "autopilot", "collaboration"}}}
		if i == 0 && len(ttl) > 0 {
			expiry := time.Now().UTC().Add(ttl[0])
			p.ExpiresAt = &expiry
		}
		if _, err = f.svc.ProposeKnowledge(f.human, source.Issue.ID, p); err != nil {
			t.Fatal(err)
		}
		if _, err = f.svc.ApproveKnowledge(f.human, util.MustParseUUID(id), protocol.CollaborationKnowledgeApproveRequest{ProtocolVersion: 1, RequestID: uuid.NewString(), ExpectedStateRevision: "1", ExpectedVersion: "1", ExpectedContentSHA256: protocol.CollaborationKnowledgeHash(p.Content)}); err != nil {
			t.Fatal(err)
		}
		f.proposals = append(f.proposals, p)
	}
	f.daemonID = "composite-" + suffix
	f.token = "mdt_" + uuid.NewString()
	f.runtimeID = f.fx.Runtime(t, "Composite runtime", testutil.Cols{"runtime_mode": "local", "provider": "claude", "daemon_id": f.daemonID})
	env, _ := json.Marshal(f.task.Agent.CustomEnv)
	f.agentID = f.fx.Agent(t, "Composite agent", f.runtimeID, testutil.Cols{"runtime_mode": "local", "custom_env": env, "max_concurrent_tasks": 3})
	f.fx.Insert(t, "daemon_token", testutil.Cols{"token_hash": auth.HashToken(f.token), "workspace_id": f.workspace, "daemon_id": f.daemonID, "user_id": f.user, "expires_at": testutil.Raw("now()+interval '1 hour'")})
	hub := realtime.NewHub()
	go hub.Run()
	daemonHub := daemonws.NewHub()
	f.h = handler.New(q, f.pool, hub, events.New(), service.NewEmailService(), nil, nil, nil, handler.Config{MessagingIntegrationsDisabled: true}, daemonHub)
	daemonHub.SetHeartbeatHandler(f.h.HandleDaemonWSHeartbeat)
	daemonHub.SetRuntimeAuthorizer(f.h.AuthorizeDaemonConnection)
	daemonHub.SetRPCHandler(f.h.DaemonRPCHandler)
	daemonHub.SetPasswordQueries(q)
	f.h.TaskService.KnowledgeContextEnabled = true
	router := chi.NewRouter()
	router.Use(middleware.DaemonAuth(q, nil, nil, nil))
	router.Post("/api/daemon/runtimes/{runtimeId}/tasks/claim", f.h.ClaimTaskByRuntime)
	router.Post("/api/daemon/tasks/claim", f.h.ClaimTasksByRuntime)
	router.Get("/api/daemon/ws", f.h.DaemonWebSocket)
	router.Post("/api/daemon/runtimes/{runtimeId}/tasks/{taskId}/prepare-lease", f.h.ExtendTaskPrepareLease)
	router.Post("/api/daemon/runtimes/{runtimeId}/tasks/{taskId}/skill-bundles/resolve", f.h.ResolveTaskSkillBundles)
	router.Post("/api/daemon/tasks/{taskId}/knowledge-context/prepare", f.h.PrepareTaskKnowledge)
	router.Post("/api/daemon/tasks/{taskId}/knowledge-context/authorize", f.h.AuthorizeTaskKnowledgeLaunch)
	router.Post("/api/daemon/tasks/{taskId}/knowledge-context/observations", f.h.ObserveTaskKnowledgeLaunch)
	router.Post("/api/daemon/tasks/{taskId}/start", f.h.StartTask)
	router.Post("/api/daemon/tasks/{taskId}/complete", f.h.CompleteTask)
	router.Post("/api/daemon/tasks/{taskId}/fail", f.h.FailTask)
	router.Post("/api/daemon/tasks/{taskId}/progress", f.h.ReportTaskProgress)
	router.Post("/api/daemon/tasks/{taskId}/messages", f.h.ReportTaskMessages)
	router.Post("/api/daemon/tasks/{taskId}/usage", f.h.ReportTaskUsage)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/daemon/ws" {
			router.ServeHTTP(w, r)
			return
		}
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
			w.WriteHeader(400)
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(raw))
		action := filepath.Base(r.URL.Path)
		if f.before != nil {
			f.before(action, raw)
		}
		response := testutil.Call(t, router.ServeHTTP, r)
		body := append([]byte(nil), response.Body.Bytes()...)
		f.mu.Lock()
		f.traffic = append(f.traffic, knowledgeServerCall{r.URL.Path, raw, body, response.Code})
		f.mu.Unlock()
		if f.after != nil {
			f.after(action, body)
		}
		if f.drop == action && !f.dropped {
			f.dropped = true
			conn, _, err := w.(http.Hijacker).Hijack()
			if err != nil {
				t.Error(err)
				return
			}
			_ = conn.Close()
			return
		}
		for k, v := range response.Header() {
			w.Header()[k] = v
		}
		w.WriteHeader(response.Code)
		_, _ = w.Write(body)
	}))
	t.Cleanup(server.Close)
	f.d.cfg.ServerBaseURL = server.URL
	f.d.cfg.DaemonID = f.daemonID
	f.d.client.baseURL = server.URL
	f.d.client.SetToken(f.token)
	f.d.runtimeIndex = map[string]Runtime{f.runtimeID: {ID: f.runtimeID, Provider: "claude"}}
	f.enqueue(t, surface)
	return f
}

func (f *knowledgeServerFixture) enqueue(t *testing.T, surface string) string {
	t.Helper()
	cols := testutil.Cols{"runtime_id": f.runtimeID, "originator_user_id": f.user, "accountable_user_id": f.user, "max_attempts": 3}
	switch surface {
	case "issue", "comment":
		if f.issueID == "" {
			f.issueID = f.fx.Issue(t, "Composite ordinary input", testutil.Cols{"assignee_type": "agent", "assignee_id": f.agentID, "project_id": f.project})
		}
		cols["issue_id"] = f.issueID
		if surface == "comment" {
			cols["trigger_comment_id"] = f.fx.Comment(t, f.issueID, "composite explicit follow-up")
		}
	case "chat":
		if f.chatID == "" {
			f.chatID = f.fx.ChatSession(t, f.agentID, testutil.Cols{"runtime_id": f.runtimeID, "project_id": f.project})
		}
		cols["chat_session_id"] = f.chatID
	case "quick_create":
		raw, _ := json.Marshal(service.QuickCreateContext{Type: service.QuickCreateContextType, WorkspaceID: f.workspace, RequesterID: f.user, Prompt: "Create the composite requested issue"})
		cols["context"] = raw
	case "autopilot":
		id := f.fx.Insert(t, "autopilot", testutil.Cols{"workspace_id": f.workspace, "title": "Composite automation", "assignee_id": f.agentID, "execution_mode": "run_only", "created_by_type": "member", "created_by_id": f.user})
		cols["autopilot_run_id"] = f.fx.Insert(t, "autopilot_run", testutil.Cols{"autopilot_id": id, "source": "manual", "status": "running"})
	default:
		t.Fatal("invalid composite surface")
	}
	id := f.fx.Task(t, f.agentID, cols)
	if surface == "chat" {
		f.fx.Exec(t, `UPDATE agent_task_queue SET chat_input_task_id=id WHERE id=$1`, id)
		f.fx.Insert(t, "chat_message", testutil.Cols{"chat_session_id": f.chatID, "role": "user", "content": "composite private chat request", "task_id": id})
	}
	return id
}
func (f *knowledgeServerFixture) claim(t *testing.T, batch bool) Task {
	t.Helper()
	var task *Task
	var err error
	if batch {
		var tasks []*Task
		tasks, err = f.d.client.ClaimTasks(t.Context(), f.daemonID, []string{f.runtimeID}, 1)
		if len(tasks) == 1 {
			task = tasks[0]
		}
	} else {
		task, err = f.d.client.ClaimTask(t.Context(), f.runtimeID)
	}
	if err != nil {
		t.Fatal(err)
	}
	if task == nil || task.KnowledgeContext == nil {
		t.Fatal("real handler did not finalize negotiated task")
	}
	f.task = *task
	row, err := f.h.Queries.GetAgentTask(t.Context(), util.MustParseUUID(task.ID))
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range f.proposals {
		if bytes.Contains(row.KnowledgeDelivery, []byte(p.CandidateID)) || bytes.Contains(row.KnowledgeDelivery, []byte(p.Content)) {
			t.Fatal("private material persisted before separate disclosure")
		}
	}
	for _, call := range f.calls("claim") {
		for _, p := range f.proposals {
			if bytes.Contains(call.response, []byte(p.CandidateID)) || bytes.Contains(call.response, []byte(p.Content)) {
				t.Fatal("claim disclosed private knowledge")
			}
		}
	}
	return *task
}
func (f *knowledgeServerFixture) calls(action string) []knowledgeServerCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []knowledgeServerCall
	for _, c := range f.traffic {
		if filepath.Base(c.path) == action {
			out = append(out, c)
		}
	}
	return out
}
func (f *knowledgeServerFixture) execute(t *testing.T) TaskResult {
	t.Helper()
	result, err := f.d.runTask(t.Context(), f.task, "claude", 0, f.d.logger)
	if err != nil {
		t.Fatal(err)
	}
	f.d.reportTaskResult(t.Context(), f.task.ID, result, f.d.logger)
	return result
}
func (f *knowledgeServerFixture) prompt(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(f.capture)
	if err != nil {
		t.Fatal(err)
	}
	var wire struct {
		Message struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"message"`
	}
	if json.Unmarshal(raw, &wire) != nil || len(wire.Message.Content) != 1 {
		t.Fatal("fake executable did not capture exact stream-json input")
	}
	return wire.Message.Content[0].Text
}
func (f *knowledgeServerFixture) assertStableFiles(t *testing.T) {
	t.Helper()
	err := filepath.WalkDir(f.d.cfg.WorkspacesRoot, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, p := range f.proposals {
			for _, private := range []string{p.Content, p.Summary, p.CandidateID, p.Sources[0].IssueID} {
				if bytes.Contains(raw, []byte(private)) {
					t.Errorf("private knowledge persisted in stable file %s", filepath.Base(path))
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
func TestKnowledgeServerDaemonCompositeHappyPath(t *testing.T) {
	f := newKnowledgeServerFixture(t, "issue")
	f.claim(t, false)
	result := f.execute(t)
	if result.Status != "completed" || result.SessionID != f.native {
		t.Fatalf("real daemon outcome %s/%s", result.Status, result.FailureReason)
	}
	prompt := f.prompt(t)
	for _, p := range f.proposals {
		if !strings.Contains(prompt, p.Content) {
			t.Fatal("real approved contribution absent from stdin")
		}
	}
	prepares := f.calls("prepare")
	if len(prepares) != 1 || prepares[0].status != 200 {
		t.Fatal("real preparation failed")
	}
	var prepared protocol.TaskKnowledgePrepareResponse
	if json.Unmarshal(prepares[0].response, &prepared) != nil {
		t.Fatal("decode preparation")
	}
	if len(prepared.Contributions) != 2 || prepared.Contributions[0].CandidateID != f.proposals[1].CandidateID {
		t.Fatal("multiple target bundle not in canonical order")
	}
	var authorized protocol.TaskKnowledgeAuthorizeRequest
	if json.Unmarshal(f.calls("authorize")[0].request, &authorized) != nil {
		t.Fatal("decode authorization")
	}
	if authorized.PromptSHA256 != protocol.CollaborationKnowledgeHash(prompt) || authorized.ManifestSHA256 != prepared.ManifestSHA256 || authorized.InvocationFingerprint != prepared.InvocationFingerprint {
		t.Fatal("real authorization does not bind actual stdin and manifest")
	}
	row, err := f.h.Queries.GetAgentTask(t.Context(), util.MustParseUUID(f.task.ID))
	if err != nil {
		t.Fatal(err)
	}
	if row.Status != "completed" || row.SessionID.String != f.native || !bytes.Contains(row.KnowledgeDelivery, []byte(prepared.ManifestSHA256)) || !bytes.Contains(row.KnowledgeDelivery, []byte(f.native)) {
		t.Fatal("terminal pointer and retained exposure disagree")
	}
	observations := f.calls("observations")
	if len(observations) != 3 {
		t.Fatalf("actual process observation count=%d", len(observations))
	}
	for _, c := range observations {
		if c.status != 200 {
			t.Fatal("real process observation was refused")
		}
	}
	credential, err := os.ReadFile(f.credentials)
	if err != nil || string(credential) != f.task.AuthToken || !strings.HasPrefix(string(credential), "mat_") {
		t.Fatal("actual child environment did not preserve its scoped task credential")
	}
	for _, p := range f.proposals {
		if bytes.Contains(row.KnowledgeDelivery, []byte(p.Content)) || bytes.Contains(row.KnowledgeDelivery, []byte(p.Summary)) {
			t.Fatal("private body or summary persisted in exposure metadata")
		}
	}
	if strings.Contains(f.logs.String(), privateKnowledgeTestBody) {
		t.Fatal("private provider output escaped into generic logs")
	}
	var workspaceBase, projectBase string
	f.fx.QueryRow(t, `SELECT context,(SELECT description FROM project WHERE id=$2) FROM workspace WHERE id=$1`, f.workspace, f.project).Scan(&workspaceBase, &projectBase)
	if workspaceBase != "independent workspace base" || projectBase != "independent project base" {
		t.Fatal("private knowledge changed stable base context")
	}
	f.assertStableFiles(t)
	t.Logf("source_sha256=%s,%s manifest_sha256=%s prompt_sha256=%s physical_starts=1 native_pointer_bound=true", f.proposals[0].Sources[0].SHA256, f.proposals[1].Sources[0].SHA256, prepared.ManifestSHA256, authorized.PromptSHA256)
}

func (f *knowledgeServerFixture) withdraw(t *testing.T, index int) {
	t.Helper()
	_, err := f.svc.WithdrawKnowledge(f.human, util.MustParseUUID(f.proposals[index].CandidateID), protocol.CollaborationKnowledgeDecisionRequest{ProtocolVersion: 1, RequestID: uuid.NewString(), ExpectedStateRevision: "2", Reason: "Composite source withdrawal"})
	if err != nil {
		t.Fatal(err)
	}
}
func (f *knowledgeServerFixture) startCount(t *testing.T) int {
	t.Helper()
	raw, err := os.ReadFile(f.argv)
	if errors.Is(err, os.ErrNotExist) {
		return 0
	}
	if err != nil {
		t.Fatal(err)
	}
	return strings.Count(string(raw), "PHYSICAL_START")
}
func TestKnowledgeServerDaemonCompositeOrdinarySurfaces(t *testing.T) {
	for _, surface := range []string{"comment", "chat", "quick_create", "autopilot"} {
		t.Run(surface, func(t *testing.T) {
			f := newKnowledgeServerFixture(t, surface)
			f.claim(t, true)
			result := f.execute(t)
			if result.Status != "completed" {
				t.Fatalf("surface %s failed: %s", surface, result.FailureReason)
			}
			prompt := f.prompt(t)
			if !strings.Contains(prompt, f.proposals[1].Content) {
				t.Fatal("workspace contribution did not reach actual stdin")
			}
			for _, call := range f.calls("prepare") {
				for _, key := range []string{"originator_user_id", "accountable_user_id", "audience", "runtime_owner"} {
					if bytes.Contains(call.request, []byte(key)) {
						t.Fatal("client supplied authority instead of server consumer facts")
					}
				}
			}
			row, err := f.h.Queries.GetAgentTask(t.Context(), util.MustParseUUID(f.task.ID))
			if err != nil || row.Status != "completed" || row.SessionID.String != f.native {
				t.Fatal("surface lost terminal native pointer")
			}
			f.assertStableFiles(t)
			t.Logf("surface=%s prompt_sha256=%s physical_starts=%d", surface, protocol.CollaborationKnowledgeHash(prompt), f.startCount(t))
		})
	}
}
func TestKnowledgeServerDaemonCompositeLiveWebSocketClaim(t *testing.T) {
	f := newKnowledgeServerFixture(t, "issue")
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = f.d.runTaskWakeupConnection(ctx, []string{f.runtimeID}, make(chan taskWakeup, 8), make(chan struct{}))
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("live daemon WebSocket did not stop")
		}
	})
	deadline := time.Now().Add(5 * time.Second)
	for !f.d.wsRPC.supportsRPCV1() && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if !f.d.wsRPC.supportsRPCV1() {
		t.Fatal("actual WebSocket never negotiated RPC support")
	}
	tasks, err := f.d.ClaimTasksWSFirst(t.Context(), f.daemonID, []string{f.runtimeID}, 1)
	if err != nil || len(tasks) != 1 || tasks[0].KnowledgeContext == nil {
		t.Fatal("live authenticated WS claim failed")
	}
	f.task = *tasks[0]
	if len(f.calls("claim")) != 0 {
		t.Fatal("WebSocket claim silently fell back to HTTP")
	}
	if result := f.execute(t); result.Status != "completed" {
		t.Fatal("WebSocket claimed task failed real execution")
	}
	if !strings.Contains(f.prompt(t), f.proposals[0].Content) || f.startCount(t) != 1 {
		t.Fatal("WS claim failed exact private input boundary")
	}
	cancel()
	<-done
	t.Log("live_ws_rpc=true http_claim_fallback=false actual_stdin_verified=true")
}
func TestKnowledgeServerDaemonCompositeLostCommittedResponses(t *testing.T) {
	for _, action := range []string{"prepare", "authorize", "observations"} {
		t.Run(action, func(t *testing.T) {
			f := newKnowledgeServerFixture(t, "issue")
			f.claim(t, false)
			f.drop = action
			if result := f.execute(t); result.Status != "completed" {
				t.Fatalf("real committed response recovery: %s", result.FailureReason)
			}
			calls := f.calls(action)
			if len(calls) < 2 || !bytes.Equal(calls[0].request, calls[1].request) || !bytes.Equal(calls[0].response, calls[1].response) {
				t.Fatal("committed replay changed original request or receipt")
			}
			if f.startCount(t) != 1 {
				t.Fatal("response recovery replayed business input")
			}
			if action == "prepare" {
				var first protocol.TaskKnowledgePrepareResponse
				if json.Unmarshal(calls[0].response, &first) != nil || len(first.Contributions) != 2 || first.Contributions[0].CandidateID != f.proposals[1].CandidateID {
					t.Fatal("multi-target response replay lost canonical contribution order")
				}
			}
			t.Logf("lost_response=%s original_key_and_receipt_equal=true physical_starts=1", action)
		})
	}
}
func TestKnowledgeServerDaemonCompositeRefusesChangedSourceBeforeLaunch(t *testing.T) {
	for _, change := range []string{"withdraw", "purge", "expiry", "gate"} {
		t.Run(change, func(t *testing.T) {
			var ttl []time.Duration
			if change == "expiry" {
				ttl = []time.Duration{2 * time.Second}
			}
			f := newKnowledgeServerFixture(t, "issue", ttl...)
			f.claim(t, false)
			f.before = func(action string, _ []byte) {
				if action != "authorize" {
					return
				}
				f.before = nil
				switch change {
				case "withdraw":
					f.withdraw(t, 0)
				case "purge":
					if _, err := f.svc.UpdateMemory(f.human, service.CollaborationMemoryUpdateParams{IssueID: util.MustParseUUID(f.proposals[0].Sources[0].IssueID), RequestID: uuid.NewString(), ExpectedVersion: 1, Content: "replacement source version"}); err != nil {
						t.Error(err)
					}
					_, err := f.svc.PurgeMemoryVersion(f.human, service.CollaborationMemoryPurgeParams{IssueID: util.MustParseUUID(f.proposals[0].Sources[0].IssueID), RequestID: uuid.NewString(), Version: 1, Reason: "Composite unavailable source"})
					if err != nil {
						t.Error(err)
					}
				case "expiry":
					time.Sleep(time.Until(*f.proposals[0].ExpiresAt) + 10*time.Millisecond)
				case "gate":
					f.h.TaskService.KnowledgeContextEnabled = false
				}
			}
			result := f.execute(t)
			if result.Status != "blocked" || !knowledgeFailureCode(result.FailureReason) || f.startCount(t) != 0 {
				t.Fatalf("%s did not refuse first physical launch: %s/%s", change, result.Status, result.FailureReason)
			}
			row, err := f.h.Queries.GetAgentTask(t.Context(), util.MustParseUUID(f.task.ID))
			if err != nil {
				t.Fatal(err)
			}
			if row.Status != "failed" || row.SessionID.Valid || row.Attempt > 1 {
				t.Fatal("refusal fabricated a pointer or automatic task retry")
			}
			if len(f.calls("prepare")) != 1 || len(f.calls("observations")) != 0 {
				t.Fatal("first refusal replayed preparation or invented process facts")
			}
			f.assertStableFiles(t)
			t.Logf("change=%s reason=%s physical_starts=0 terminal_failed=true", change, result.FailureReason)
		})
	}
}
func TestKnowledgeServerDaemonCompositeLostDisclosureThenWithdrawal(t *testing.T) {
	f := newKnowledgeServerFixture(t, "issue")
	f.claim(t, false)
	f.drop = "prepare"
	f.after = func(action string, _ []byte) {
		if action == "prepare" {
			f.after = nil
			f.withdraw(t, 0)
		}
	}
	result := f.execute(t)
	calls := f.calls("prepare")
	if result.Status != "blocked" || f.startCount(t) != 0 || len(calls) != 2 || calls[0].status != 200 || calls[1].status < 400 || !bytes.Equal(calls[0].request, calls[1].request) {
		t.Fatal("lost committed disclosure bypassed current source withdrawal")
	}
	for _, p := range f.proposals {
		if bytes.Contains(calls[1].response, []byte(p.Content)) {
			t.Fatal("refused replay disclosed private body")
		}
	}
}
func TestKnowledgeServerDaemonCompositeGateClosureKeepsObservationCleanup(t *testing.T) {
	f := newKnowledgeServerFixture(t, "issue")
	f.claim(t, false)
	f.after = func(action string, _ []byte) {
		if action == "authorize" {
			f.after = nil
			f.h.TaskService.KnowledgeContextEnabled = false
		}
	}
	result := f.execute(t)
	if result.Status != "completed" || f.startCount(t) != 1 {
		t.Fatal("gate closure stranded an admitted process")
	}
	observations := f.calls("observations")
	if len(observations) != 3 {
		t.Fatal("gate closure lost process observation")
	}
	for _, call := range observations {
		if call.status != 200 {
			t.Fatal("gate closure refused admitted cleanup")
		}
	}
	row, err := f.h.Queries.GetAgentTask(t.Context(), util.MustParseUUID(f.task.ID))
	if err != nil || row.Status != "completed" || row.SessionID.String != f.native {
		t.Fatal("gate closure lost terminal exposure/pointer")
	}
}
func TestKnowledgeServerDaemonCompositeZeroNativeStaleFence(t *testing.T) {
	f := newKnowledgeServerFixture(t, "issue")
	task := f.claim(t, false)
	f.fx.Exec(t, `UPDATE agent_task_queue SET claim_generation=claim_generation+1 WHERE id=$1`, task.ID)
	_, err := f.d.runTask(t.Context(), task, "claude", 0, f.d.logger)
	if err == nil || taskRunFailureReason(err) != agent.KnowledgeContextChanged {
		t.Fatal("stale StartTask fence lost typed refusal")
	}
	if f.startCount(t) != 0 || len(f.calls("prepare")) != 0 {
		t.Fatal("stale zero-native claim reached private disclosure or executable")
	}
	err = f.d.client.FailTask(t.Context(), task.ID, "knowledge_context_changed", "", "", "", agent.KnowledgeContextChanged, false, "", "", taskKnowledgeFence(task))
	if err == nil {
		t.Fatal("stale zero-native terminal writer was accepted")
	}
	row, err := f.h.Queries.GetAgentTask(t.Context(), util.MustParseUUID(task.ID))
	if err != nil || row.Status != "dispatched" || row.SessionID.Valid {
		t.Fatal("stale writer changed replacement task")
	}
}
func TestKnowledgeServerDaemonCompositeLaterLaunchRefusal(t *testing.T) {
	f := newKnowledgeServerFixture(t, "issue")
	f.claim(t, false)
	// The controlled backend asks the real provider backend for a second
	// physical command; both commands use the same production lifecycle.
	if err := f.d.client.StartTask(t.Context(), f.task.ID, taskKnowledgeFence(f.task)); err != nil {
		t.Fatal(err)
	}
	cfg := agent.Config{ExecutablePath: f.bin, BuiltinRuntime: true, Logger: f.d.logger, Env: f.task.Agent.CustomEnv}
	opts := agent.ExecOptions{Cwd: t.TempDir(), Timeout: 5 * time.Second}
	prompt, err := f.d.prepareKnowledgePrompt(t.Context(), f.task, "claude", cfg, &opts, nil)
	if err != nil {
		t.Fatal(err)
	}
	inner, err := agent.ResolveBackend("claude", cfg)
	if err != nil {
		t.Fatal(err)
	}
	backend := knowledgeTwoLaunchBackend{inner: inner, afterFirst: func() { f.withdraw(t, 0) }}
	var sequence atomic.Int32
	result, _, err := f.d.executeAndDrain(t.Context(), backend, prompt, opts, f.d.logger, f.task.ID, "", &sequence)
	if err != nil || result.KnowledgeFailure != agent.KnowledgeContextChanged || result.Usage["test-model"].InputTokens != 12 || result.KnowledgeOutput != privateKnowledgeTestBody || result.Output != "" {
		t.Fatal("later real authorization refusal lost original usage/evidence")
	}
	if f.startCount(t) != 1 || len(f.calls("authorize")) != 2 || len(f.calls("observations")) != 3 {
		t.Fatal("later refusal replayed input or lost original process termination")
	}
	if err = f.d.client.FailTask(t.Context(), f.task.ID, agent.KnowledgeContextChanged, f.native, "", "", agent.KnowledgeContextChanged, false, "", "", taskKnowledgeFence(f.task)); err != nil {
		t.Fatal(err)
	}
	row, err := f.h.Queries.GetAgentTask(t.Context(), util.MustParseUUID(f.task.ID))
	if err != nil || row.Status != "failed" || row.Attempt > 1 {
		t.Fatal("later refusal entered automatic task retry")
	}
}

func TestKnowledgeServerDaemonCompositeExactResumeThenFreshReplacement(t *testing.T) {
	f := newKnowledgeServerFixture(t, "issue")
	first := f.claim(t, false)
	if result := f.execute(t); result.Status != "completed" {
		t.Fatal("first native exposure failed")
	}
	f.enqueue(t, "comment")
	second := f.claim(t, true)
	if second.PriorSessionID != f.native || second.KnowledgeContext.Session.NativeSessionID != f.native || second.KnowledgeContext.Session.Generation != first.KnowledgeContext.Session.Generation {
		t.Fatal("real claim did not select exact cumulative session binding")
	}
	if result := f.execute(t); result.Status != "completed" || result.SessionID != f.native {
		t.Fatalf("real follow-up failed: %s", result.FailureReason)
	}
	argv, _ := os.ReadFile(f.argv)
	if !strings.Contains(string(argv), "--resume\n"+f.native+"\n") {
		t.Fatal("exact selected session did not reach actual argv")
	}
	// The old native remains in both completed task exposures. Withdrawing any
	// earlier contribution must retire that entire exact native history.
	f.withdraw(t, 0)
	f.enqueue(t, "comment")
	third := f.claim(t, false)
	if third.PriorSessionID != "" || third.KnowledgeContext.Session.NativeSessionID != "" || third.KnowledgeContext.Session.Generation == first.KnowledgeContext.Session.Generation {
		t.Fatal("withdrawn cumulative exposure remained resumable")
	}
	newNative := "composite-replacement-native"
	if err := os.WriteFile(f.task.Agent.CustomEnv["NATIVE_CONTROL"], []byte(newNative), 0600); err != nil {
		t.Fatal(err)
	}
	if result := f.execute(t); result.Status != "completed" || result.SessionID != newNative {
		t.Fatalf("explicit fresh follow-up failed: %s", result.FailureReason)
	}
	prompt := f.prompt(t)
	if strings.Contains(prompt, f.proposals[0].Content) || !strings.Contains(prompt, f.proposals[1].Content) {
		t.Fatal("replacement input reused withdrawn body or lost current approved body")
	}
	argv, _ = os.ReadFile(f.argv)
	if strings.Count(string(argv), "--resume\n") != 1 || f.startCount(t) != 3 {
		t.Fatal("replacement launch reused old native or replayed input")
	}
	for _, task := range []Task{first, second} {
		row, err := f.h.Queries.GetAgentTask(t.Context(), util.MustParseUUID(task.ID))
		if err != nil || !bytes.Contains(row.KnowledgeDelivery, []byte(f.proposals[0].CandidateID)) {
			t.Fatal("fresh replacement erased historical exposure")
		}
	}
	f.assertStableFiles(t)
	t.Log("completed_turns=3 cumulative_exposure_retained=true resume_launches=1 fresh_replacement_launches=1 withdrawn_stdin_absent=true")
}
func TestKnowledgeServerDaemonCompositeLegacyCannotReuseExposure(t *testing.T) {
	f := newKnowledgeServerFixture(t, "issue")
	f.claim(t, false)
	if result := f.execute(t); result.Status != "completed" {
		t.Fatal("initial exposure failed")
	}
	f.withdraw(t, 0)
	f.enqueue(t, "comment")
	// A runtime unknown to the capability selector represents an installed old
	// client. Its authenticated HTTP claim still traverses history safety.
	delete(f.d.runtimeIndex, f.runtimeID)
	task, err := f.d.client.ClaimTask(t.Context(), f.runtimeID)
	if err == nil || task != nil || knowledgeErrorReason(err) != agent.KnowledgeInvocationUnsupported {
		t.Fatalf("legacy client bypassed managed history: task=%t reason=%s", task != nil, knowledgeErrorReason(err))
	}
	if f.startCount(t) != 1 {
		t.Fatal("legacy denial launched input")
	}
}
func TestKnowledgeServerDaemonCompositeAccountabilityIsNotOriginator(t *testing.T) {
	f := newKnowledgeServerFixture(t, "autopilot")
	f.fx.Exec(t, `UPDATE agent_task_queue SET originator_user_id=NULL WHERE agent_id=$1 AND status='queued'`, f.agentID)
	f.claim(t, false)
	if result := f.execute(t); result.Status != "completed" {
		t.Fatalf("safe base automation failed: %s", result.FailureReason)
	}
	prompt := f.prompt(t)
	for _, p := range f.proposals {
		if strings.Contains(prompt, p.Content) {
			t.Fatal("accountable user or installer supplied missing trusted originator")
		}
	}
	var bundle protocol.TaskKnowledgePrepareResponse
	if json.Unmarshal(f.calls("prepare")[0].response, &bundle) != nil || len(bundle.Contributions) != 0 {
		t.Fatal("missing trusted originator disclosed private material")
	}
	if len(f.calls("authorize")) != 1 || f.startCount(t) != 1 {
		t.Fatal("empty authorized bundle bypassed real launch fence")
	}
}

func TestKnowledgeServerDaemonCompositeSourceParticipationRevoked(t *testing.T) {
	f := newKnowledgeServerFixture(t, "chat")
	owner := f.fx.User(t, "External source owner", uuid.NewString()+"@example.test")
	workspace := f.fx.Workspace(t, "External source workspace", "external-"+uuid.NewString())
	f.fx.Member(t, workspace, owner, "owner")
	ownerCtx := auth.WithAuthenticatedPrincipal(t.Context(), auth.AuthenticatedPrincipal{UserID: owner, Kind: auth.CredentialJWT})
	source, err := f.svc.Create(ownerCtx, service.CollaborationCreateParams{SourceWorkspaceID: util.MustParseUUID(workspace), HandlerID: util.MustParseUUID(f.user), RequestID: uuid.NewString(), Title: "Externally shared source"})
	if err != nil {
		t.Fatal(err)
	}
	f.fx.Cleanup(t, `DELETE FROM issue WHERE id=$1`, source.Issue.ID)
	for _, table := range []string{"issue_collaboration", "issue_participant", "collaboration_session", "collaboration_event", "collaboration_notification", "collaboration_memory_version", "issue_handoff"} {
		f.fx.Cleanup(t, `DELETE FROM `+table+` WHERE issue_id=$1`, source.Issue.ID)
	}
	f.fx.Cleanup(t, `DELETE FROM collaboration_operation WHERE scope_id=$1 OR scope_id=$2`, source.Issue.ID, workspace)
	f.fx.Cleanup(t, `DELETE FROM collaboration_account_fence WHERE user_id=$1`, owner)
	if _, err = f.svc.UpdateMemory(ownerCtx, service.CollaborationMemoryUpdateParams{IssueID: source.Issue.ID, RequestID: uuid.NewString(), ExpectedVersion: 0, Content: "external source material"}); err != nil {
		t.Fatal(err)
	}
	p := protocol.CollaborationKnowledgeProposal{ProtocolVersion: 1, CandidateID: uuid.NewString(), RequestID: uuid.NewString(), Target: protocol.CollaborationKnowledgeTarget{Kind: "workspace", WorkspaceID: f.workspace}, Summary: "External attributed contribution", Content: "external-private-approved-sentinel", Sources: []protocol.CollaborationKnowledgeSource{{Kind: "memory", IssueID: util.UUIDToString(source.Issue.ID), Version: "1", SHA256: protocol.CollaborationKnowledgeHash("external source material")}}, Evidence: []protocol.CollaborationKnowledgeEvidence{}, Applicability: protocol.CollaborationKnowledgeApplicability{Surfaces: []string{"chat"}}}
	if _, err = f.svc.ProposeKnowledge(f.human, source.Issue.ID, p); err != nil {
		t.Fatal(err)
	}
	if _, err = f.svc.ApproveKnowledge(f.human, util.MustParseUUID(p.CandidateID), protocol.CollaborationKnowledgeApproveRequest{ProtocolVersion: 1, RequestID: uuid.NewString(), ExpectedVersion: "1", ExpectedStateRevision: "1", ExpectedContentSHA256: protocol.CollaborationKnowledgeHash(p.Content)}); err != nil {
		t.Fatal(err)
	}
	f.proposals = append(f.proposals, p)
	f.claim(t, false)
	f.before = func(action string, _ []byte) {
		if action != "authorize" {
			return
		}
		f.before = nil
		var bundle protocol.TaskKnowledgePrepareResponse
		calls := f.calls("prepare")
		if len(calls) != 1 || json.Unmarshal(calls[0].response, &bundle) != nil {
			t.Error("missing real disclosure")
			return
		}
		found := false
		for _, item := range bundle.Contributions {
			found = found || item.CandidateID == p.CandidateID
		}
		if !found {
			t.Error("external source was not initially eligible for private chat")
			return
		}
		access, err := f.svc.ResolveIssueAccess(f.human, f.svc.Queries, source.Issue.ID)
		if err != nil {
			t.Error(err)
			return
		}
		req := service.CollaborationWorkflowRequest{CollaborationQueueRequest: service.CollaborationQueueRequest{IssueID: source.Issue.ID, RequestID: uuid.NewString(), ExpectedLifecycleEpoch: access.LifecycleEpoch, ExpectedParticipantEpoch: access.ParticipantEpoch.Int64, ExpectedAccountEpoch: access.AccountEpoch.Int64}, ExpectedCollaborationRevision: access.Revision}
		transfer, err := f.svc.Transfer(f.human, service.CollaborationTransferParams{CollaborationParticipantParams: service.CollaborationParticipantParams{CollaborationWorkflowRequest: req, TargetUserID: util.MustParseUUID(owner)}})
		if err != nil {
			t.Error(err)
			return
		}
		req.RequestID = uuid.NewString()
		req.ExpectedCollaborationRevision = transfer.CollaborationRevision
		leave, err := f.svc.Leave(f.human, req)
		if err != nil || leave.Code != "participant_left" {
			t.Errorf("source participation revocation failed: %s %v", leave.Code, err)
		}
	}
	result := f.execute(t)
	if result.Status != "blocked" || f.startCount(t) != 0 {
		t.Fatalf("source ACL revocation failed launch fence: %s/%s", result.Status, result.FailureReason)
	}
	if f.fx.Count(t, `SELECT count(*) FROM member WHERE workspace_id=$1 AND user_id=$2`, f.workspace, f.user) != 1 {
		t.Fatal("fixture revoked target membership instead of source read")
	}
	if _, err = f.svc.ResolveIssueAccess(f.human, f.svc.Queries, source.Issue.ID); err == nil {
		t.Fatal("former external participant retained source read")
	}
	f.assertStableFiles(t)
}

func TestKnowledgeServerDaemonCompositeDatabaseGuard(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ name, value, want string }{{"absent", "", "requires explicit task-owned DATABASE_URL"}, {"unowned", "postgres:///multica?host=/definitely-nonexistent-composite-socket", "refuses a database outside its owned prefix"}} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, executable, "-test.run=^TestKnowledgeServerDaemonCompositeHappyPath$", "-test.count=1")
			for _, value := range os.Environ() {
				if !strings.HasPrefix(value, "DATABASE_URL=") {
					cmd.Env = append(cmd.Env, value)
				}
			}
			if tc.value != "" {
				cmd.Env = append(cmd.Env, "DATABASE_URL="+tc.value)
			}
			output, err := cmd.CombinedOutput()
			if err == nil || !strings.Contains(string(output), tc.want) || bytes.Contains(output, []byte("database=multica_")) {
				t.Fatal("tagged composite guard did not refuse before connecting")
			}
		})
	}
}
