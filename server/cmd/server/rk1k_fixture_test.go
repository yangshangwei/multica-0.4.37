//go:build agentintegration && !windows

package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/multica-ai/multica/server/internal/analytics"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/daemon"
	"github.com/multica-ai/multica/server/internal/daemon/collaborationrun"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

const rk1kDatabasePrefix = "multica_rk1k_"

type rk1kFixture struct {
	*rk1Fixture
	allTrace                           *rk1kTrace
	project, source, sourceWorkspace   string
	sourceToken, memory, memoryVersion string
	ordinaryRuntime, ordinaryAgent     string
	ordinaryDaemon, ordinaryToken      string
	nativeStore                        string
	driver                             *daemon.RK1KnowledgeDriver
	transport                          *rk1kTransport
}

func rk1kRequireDatabase(t *testing.T) {
	t.Helper()
	cfg, err := pgxpool.ParseConfig(strings.TrimSpace(os.Getenv("DATABASE_URL")))
	if err != nil || !strings.HasPrefix(cfg.ConnConfig.Database, rk1kDatabasePrefix) {
		t.Fatal("knowledge fake proof requires its explicit migrated " + rk1kDatabasePrefix + "* database")
	}
	var actual string
	if testPool == nil || testPool.QueryRow(t.Context(), "SELECT current_database()").Scan(&actual) != nil ||
		actual != cfg.ConnConfig.Database {
		t.Fatal("knowledge fake database identity mismatch")
	}
	if os.Getenv("MULTICA_RUN_REAL_AGENT_SMOKE") != "" {
		t.Fatal("fake proof refuses a real-agent gate")
	}
}

func newRK1KnowledgeFixture(t *testing.T, dir string, crossSource bool) *rk1kFixture {
	t.Helper()
	rk1kRequireDatabase(t)
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MULTICA_KNOWLEDGE_CONTEXT_ENABLED", "true")
	t.Setenv("MULTICA_RESOURCE_PUBLISH_DIR", "")
	t.Setenv("MULTICA_LLM_API_KEY", "")
	t.Setenv("MULTICA_LLM_BASE_URL", "")
	home := filepath.Join(dir, "home")
	if err := os.MkdirAll(home, 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(home, ".claude"))
	b := newCollaborationRouteFixture(t)
	// Add only the ordinary producer/output tables used by this sibling. The
	// collaboration/managed fixtures already shadow their own table sets.
	for _, table := range []string{"project", "chat_session", "chat_message", "autopilot", "autopilot_trigger",
		"autopilot_run", "autopilot_rule_version", "autopilot_subscriber", "autopilot_collaborator",
		"autopilot_quota_period", "autopilot_quota_reservation", "task_message", "task_usage"} {
		var local bool
		if err := b.managed.pool.QueryRow(t.Context(),
			"SELECT EXISTS(SELECT 1 FROM information_schema.tables WHERE table_schema=current_schema() AND table_name=$1)", table).Scan(&local); err != nil {
			t.Fatal(err)
		}
		if !local {
			name := pgx.Identifier{table}.Sanitize()
			b.managed.fx.Exec(t, "CREATE TABLE "+name+" (LIKE public."+name+" INCLUDING ALL)")
		}
	}
	rk1CloneTriggers(t, b.managed)
	b.managed.fx.Exec(t, `UPDATE workspace SET issue_counter=(SELECT COALESCE(max(number),0) FROM issue WHERE workspace_id=workspace.id) WHERE id=$1`, b.managed.fx.WorkspaceID)
	b.managed.fx.Member(t, b.managed.fx.WorkspaceID, b.contributor, "member")
	router, h := NewRouterWithOptions(b.managed.pool, nil, events.New(), analytics.NoopClient{}, nil, RouterOptions{})
	if !h.TaskService.KnowledgeContextEnabled {
		t.Fatal("router did not construct a knowledge-enabled TaskService")
	}
	plans := filepath.Join(dir, "provider")
	if err := os.Mkdir(plans, 0700); err != nil {
		t.Fatal(err)
	}
	binDir := filepath.Join(dir, "bin")
	if err := os.Mkdir(binDir, 0700); err != nil {
		t.Fatal(err)
	}
	wrapper := rk1kLinkSelf(t, binDir, rk1kProviderName)
	sourceCLI := os.Getenv("RK1K_SOURCE_CLI")
	expectedCLI := os.Getenv("RK1K_SOURCE_CLI_SHA256")
	if !filepath.IsAbs(sourceCLI) || len(expectedCLI) != 64 {
		t.Fatal("fake fixture requires an explicitly hash-bound task-owned Multica CLI")
	}
	raw, err := os.ReadFile(sourceCLI)
	if err != nil {
		t.Fatal("task-owned Multica CLI unavailable")
	}
	digest := sha256.Sum256(raw)
	if hex.EncodeToString(digest[:]) != expectedCLI {
		t.Fatal("task-owned Multica CLI hash changed")
	}
	cliPath := filepath.Join(binDir, "multica")
	if err := os.WriteFile(cliPath, raw, 0700); err != nil {
		t.Fatal(err)
	}
	env := &rk1Env{version: rk1kFakeVersion, model: "rk1k-requested-model", wrapper: wrapper, cli: cliPath}
	tr := &rk1kTrace{rk1Trace: &rk1Trace{next: router}}
	f := &rk1kFixture{rk1Fixture: &rk1Fixture{t: t, env: env, base: b, h: h, q: db.New(b.managed.pool),
		trace: tr.rk1Trace, dir: dir, plans: plans, targetSet: 1, targets: map[string]string{}},
		allTrace: tr, transport: &rk1kTransport{}, nativeStore: filepath.Join(home, ".claude", "rk1k-fake-native")}
	b.managed.router, b.server.Config.Handler = tr, tr
	f.svc = service.NewCollaborationService(f.q, b.managed.pool, h.IssueService)
	f.svc.KnowledgeContextEnabled = h.TaskService.KnowledgeContextEnabled
	actor := func(id string) context.Context {
		return auth.WithPasswordSession(auth.WithAuthenticatedPrincipal(context.Background(),
			auth.AuthenticatedPrincipal{UserID: id, Kind: auth.CredentialJWT}),
			auth.PasswordSession{UserID: id, Version: 1, Kind: "jwt"})
	}
	f.owner, f.contributor = actor(b.owner), actor(b.contributor)
	b.managed.fx.Exec(t, `UPDATE collaboration_execution_gate SET creation_enabled=true,host_execution_enabled=true,epoch=epoch+1 WHERE id=1`)
	installationOS := runtime.GOOS
	if installationOS == "darwin" {
		installationOS = "macos"
	}
	b.managed.fx.Exec(t, `UPDATE managed_installation SET os=$2 WHERE id=$1`, b.installationID, installationOS)
	f.project = b.managed.fx.Project(t, "RK1K independent project", testutil.Cols{"description": "Independent base; no approved policy"})
	old, err := f.q.GetAgentRuntime(t.Context(), util.MustParseUUID(b.runtime))
	if err != nil {
		t.Fatal(err)
	}
	metadata, err := json.Marshal(map[string]string{"cli_version": "0.4.37"})
	if err != nil {
		t.Fatal(err)
	}
	b.runtime = b.managed.fx.Runtime(t, "RK1K Claude host", testutil.Cols{"owner_id": b.owner,
		"provider": "claude", "runtime_mode": "local", "daemon_id": old.DaemonID.String, "status": "online", "metadata": metadata})
	f.host = &collaborationrun.HostConfig{Provider: "claude", ProviderVersion: env.version, ProviderMode: "local",
		AccountSource: "rk1k-test-runtime", Executable: wrapper,
		Limits: protocol.CollaborationPreparationLimits{FileCount: 32, PerFileBytes: 100 << 20, TotalInputBytes: 256 << 20,
			WorkspaceBytes: 512 << 20, GitExpandedBytes: 256 << 20, EntryCount: 4096, TimeoutSeconds: 60}}
	caps, err := f.host.Capabilities()
	if err != nil {
		t.Fatal(err)
	}
	runtimeRow, err := f.q.GetAgentRuntime(t.Context(), util.MustParseUUID(b.runtime))
	if err != nil {
		t.Fatal(err)
	}
	b.managed.request(t, b.binding.DaemonToken, false, "POST", "/api/collaboration/runtime/capabilities",
		protocol.CollaborationRuntimeCapabilitiesRequest{ProtocolVersion: 3, DeploymentID: b.deployment,
			InstallationID: b.installationID, RuntimeID: b.runtime, BindingID: b.binding.BindingID,
			BindingEpoch: b.binding.BindingEpoch, ExpectedRuntimeConfigEpoch: strconv.FormatInt(runtimeRow.CollaborationConfigEpoch, 10),
			Capabilities: caps}, nil).Want(200)
	capDigest, err := protocol.CollaborationRuntimeCapabilityDigest(caps)
	if err != nil {
		t.Fatal(err)
	}
	f.request = protocol.CollaborationRunClaimRequest{ProtocolVersion: 3, DeploymentID: b.deployment,
		InstallationID: b.installationID, RuntimeID: b.runtime, BindingID: b.binding.BindingID, BindingEpoch: b.binding.BindingEpoch,
		Containment: protocol.CollaborationContainmentHostProcessSupervision, PreparationVersion: 2, CapabilityDigest: capDigest}
	if f.client, err = collaborationrun.NewClient(b.server.URL, b.binding.DaemonToken, f.transport); err != nil {
		t.Fatal(err)
	}
	t.Setenv(rk1kPlanDirEnv, plans)
	f.sourceWorkspace, f.sourceToken = b.managed.fx.WorkspaceID, b.ownerToken
	sourceProject, sourceUser := f.project, b.owner
	if crossSource {
		f.sourceWorkspace = b.managed.fx.Workspace(t, "RK1K isolated source", "rk1k-source-"+uuid.NewString())
		b.managed.fx.Member(t, f.sourceWorkspace, b.contributor, "owner")
		sourceFX := testutil.New(b.managed.pool, f.sourceWorkspace, b.contributor)
		sourceProject = sourceFX.Project(t, "RK1K isolated source project")
		f.sourceToken, sourceUser = b.contributorToken, b.contributor
	}
	var source struct {
		ID string `json:"id"`
	}
	f.human(f.sourceToken, "POST", "/api/collaboration/issues", map[string]any{
		"source_workspace_id": f.sourceWorkspace, "handler_user_id": sourceUser, "title": "RK1K policy source",
		"description": "Human-owned knowledge source", "project_id": sourceProject}, nil).Want(201).JSON(&source)
	f.source, f.memoryVersion = source.ID, "0"
	if crossSource {
		body := f.sourceMutation()
		body["target_user_id"] = b.owner
		f.human(f.sourceToken, "POST", "/api/collaboration/issues/"+f.source+"/participants", body, nil).Want(200)
	}
	return f
}

func (f *rk1kFixture) human(token, method, path string, body any, headers map[string]string) *testutil.Response {
	f.t.Helper()
	if headers == nil {
		headers = map[string]string{}
	}
	headers["X-Workspace-ID"] = f.base.managed.fx.WorkspaceID
	return f.base.managed.request(f.t, token, false, method, path, body, headers)
}

// nextRunForIssue keeps one consumer's finite schedule isolated when several
// fixtures share the same runtime. The daemon list endpoint is runtime-scoped
// and can surface a still-prepared continuation from an earlier issue.
func (f *rk1Fixture) nextRunForIssue() string {
	f.t.Helper()
	session, err := f.q.GetCollaborationSession(f.t.Context(), util.MustParseUUID(f.issue))
	if err != nil {
		f.t.Fatal(err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for {
		list, err := f.client.List(f.t.Context(), f.base.runtime)
		if err != nil {
			f.t.Fatalf("runtime list: %v", err)
		}
		eligible := make(map[string]bool, len(list.RunIDs))
		for _, id := range list.RunIDs {
			eligible[id] = true
		}
		runs, err := f.q.ListCollaborationRuns(f.t.Context(), session.ID)
		if err != nil {
			f.t.Fatal(err)
		}
		pending := make([]db.CollaborationRun, 0, len(runs))
		for _, run := range runs {
			if !run.SettledAt.Valid && eligible[util.UUIDToString(run.ID)] {
				pending = append(pending, run)
			}
		}
		if len(pending) == 1 {
			return util.UUIDToString(pending[0].ID)
		}
		if len(pending) > 1 || time.Now().After(deadline) {
			ids := make([]string, 0, len(pending))
			for _, run := range pending {
				ids = append(ids, util.UUIDToString(run.ID)+":"+run.StepKind.String)
			}
			f.t.Fatalf("expected one pending run for issue %s, got %d (%s)", f.issue, len(pending), strings.Join(ids, ","))
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func (f *rk1kFixture) sourceMutation() map[string]any {
	var detail map[string]any
	f.human(f.sourceToken, "GET", "/api/collaboration/issues/"+f.source, nil, nil).Want(200).JSON(&detail)
	write, ok := detail["write_context"].(map[string]any)
	if !ok {
		f.t.Fatal("source has no actual write context")
	}
	return map[string]any{"expected_collaboration_revision": detail["collaboration_revision"],
		"expected_lifecycle_epoch": detail["lifecycle_epoch"], "expected_participant_epoch": write["participant_epoch"],
		"expected_account_epoch": write["account_epoch"]}
}

func (f *rk1kFixture) updateMemory(text string) {
	var receipt struct {
		Version string `json:"version"`
	}
	f.human(f.sourceToken, "PUT", "/api/collaboration/issues/"+f.source+"/memory",
		map[string]any{"expected_version": f.memoryVersion, "content": text}, nil).Want(200).JSON(&receipt)
	if receipt.Version == "" {
		f.t.Fatal("source memory version missing")
	}
	f.memory, f.memoryVersion = text, receipt.Version
}

func (f *rk1kFixture) propose(content string, supersedes *string) (protocol.CollaborationKnowledgeProposal, protocol.CollaborationKnowledgeReceipt) {
	p := protocol.CollaborationKnowledgeProposal{ProtocolVersion: 1, CandidateID: uuid.NewString(), RequestID: uuid.NewString(),
		Target:  protocol.CollaborationKnowledgeTarget{Kind: "project", WorkspaceID: f.base.managed.fx.WorkspaceID, ProjectID: f.project},
		Summary: "Human-reviewed registered policy", Content: content, SupersedesCandidateID: supersedes,
		Sources: []protocol.CollaborationKnowledgeSource{{Kind: "memory", IssueID: f.source, Version: f.memoryVersion,
			SHA256: protocol.CollaborationKnowledgeHash(f.memory)}}, Evidence: []protocol.CollaborationKnowledgeEvidence{},
		Applicability: protocol.CollaborationKnowledgeApplicability{Surfaces: []string{"issue", "chat", "quick_create", "autopilot", "collaboration"}}}
	var receipt protocol.CollaborationKnowledgeReceipt
	f.human(f.sourceToken, "POST", "/api/collaboration/issues/"+f.source+"/knowledge-candidates", p, nil).Want(201).JSON(&receipt)
	return p, receipt
}

func (f *rk1kFixture) approve(p protocol.CollaborationKnowledgeProposal, pending protocol.CollaborationKnowledgeReceipt) (protocol.CollaborationKnowledgeApproveRequest, protocol.CollaborationKnowledgeReceipt) {
	request := protocol.CollaborationKnowledgeApproveRequest{ProtocolVersion: 1, RequestID: uuid.NewString(),
		ExpectedStateRevision: pending.StateRevision, ExpectedVersion: pending.Version,
		ExpectedContentSHA256: protocol.CollaborationKnowledgeHash(p.Content)}
	var receipt protocol.CollaborationKnowledgeReceipt
	f.human(f.base.ownerToken, "POST", "/api/collaboration/knowledge-candidates/"+p.CandidateID+"/approve", request, nil).Want(200).JSON(&receipt)
	return request, receipt
}

func (f *rk1kFixture) approveLostResponse(p protocol.CollaborationKnowledgeProposal, pending protocol.CollaborationKnowledgeReceipt) (protocol.CollaborationKnowledgeApproveRequest, protocol.CollaborationKnowledgeReceipt) {
	request := protocol.CollaborationKnowledgeApproveRequest{ProtocolVersion: 1, RequestID: uuid.NewString(),
		ExpectedStateRevision: pending.StateRevision, ExpectedVersion: pending.Version,
		ExpectedContentSHA256: protocol.CollaborationKnowledgeHash(p.Content)}
	path := "/api/collaboration/knowledge-candidates/" + p.CandidateID + "/approve"
	f.transport.mu.Lock()
	f.transport.dropPath, f.transport.dropped = path, false
	f.transport.mu.Unlock()
	req := testutil.JSONRequest("POST", f.base.server.URL+path, request)
	req = req.WithContext(f.t.Context())
	// JSONRequest is also used by the in-process handler helper and therefore
	// carries RequestURI. A real http.Client must send an origin-form request
	// and rejects that server-only field before RoundTrip, which would bypass
	// the deliberate lost-response transport control entirely.
	req.RequestURI = ""
	req.Header.Set("Authorization", "Bearer "+f.base.ownerToken)
	client := &http.Client{Transport: f.transport}
	response, err := client.Do(req)
	if response != nil {
		_ = response.Body.Close()
	}
	f.transport.mu.Lock()
	dropped := f.transport.dropped
	dropPath := f.transport.dropPath
	f.transport.mu.Unlock()
	if err == nil || !dropped {
		status := 0
		if response != nil {
			status = response.StatusCode
		}
		f.t.Fatalf("approval control did not lose one committed successful response: err=%v status=%d dropped=%v drop_path=%q request_path=%q", err, status, dropped, dropPath, path)
	}
	var receipt protocol.CollaborationKnowledgeReceipt
	f.human(f.base.ownerToken, "GET", "/api/collaboration/knowledge-candidates/"+p.CandidateID+"/operations/"+request.RequestID,
		nil, nil).Want(200).JSON(&receipt)
	if receipt.RequestID != request.RequestID {
		f.t.Fatal("lost approval response was not recovered by its original body request ID")
	}
	return request, receipt
}

func (f *rk1kFixture) withdraw(p protocol.CollaborationKnowledgeProposal, approved protocol.CollaborationKnowledgeReceipt) {
	request := protocol.CollaborationKnowledgeDecisionRequest{ProtocolVersion: 1, RequestID: uuid.NewString(),
		ExpectedStateRevision: approved.StateRevision, Reason: "Registered withdrawal"}
	f.human(f.sourceToken, "POST", "/api/collaboration/knowledge-candidates/"+p.CandidateID+"/withdraw", request, nil).Want(200)
}

func (f *rk1kFixture) ordinaryDriver() *daemon.RK1KnowledgeDriver {
	if f.driver != nil {
		return f.driver
	}
	t := f.t
	f.ordinaryDaemon = "rk1k-" + uuid.NewString()
	f.ordinaryToken = "mdt_" + uuid.NewString()
	f.ordinaryRuntime = f.base.managed.fx.Runtime(t, "RK1K ordinary Claude", testutil.Cols{"owner_id": f.base.owner,
		"provider": "claude", "runtime_mode": "local", "daemon_id": f.ordinaryDaemon, "status": "online",
		"metadata": `{"cli_version":"0.4.37"}`})
	f.ordinaryAgent = f.base.managed.fx.Agent(t, "RK1K ordinary agent", f.ordinaryRuntime, testutil.Cols{
		"owner_id": f.base.owner, "autonomy_level": "contributor", "permission_mode": "public_to", "visibility": "workspace",
		"instructions": "Report active approved policy from the delivered knowledge data, or NONE."})
	f.base.managed.fx.Insert(t, "agent_invocation_target", testutil.Cols{"agent_id": f.ordinaryAgent,
		"target_type": "workspace", "target_id": f.base.managed.fx.WorkspaceID})
	f.base.managed.fx.Insert(t, "daemon_token", testutil.Cols{"token_hash": auth.HashToken(f.ordinaryToken),
		"workspace_id": f.base.managed.fx.WorkspaceID, "daemon_id": f.ordinaryDaemon, "user_id": f.base.owner,
		"auth_version": int64(1), "expires_at": time.Now().Add(time.Hour)})
	var err error
	f.driver, err = daemon.NewRK1KnowledgeDriver(daemon.RK1KnowledgeDriverOptions{Config: daemon.Config{
		ServerBaseURL: f.base.server.URL, DaemonID: f.ordinaryDaemon, WorkspacesRoot: filepath.Join(f.dir, "workspaces"),
		Profile: "rk1k", CLIVersion: "0.4.37", AgentTimeout: time.Minute, KeepEnvAfterTask: true,
		Agents: map[string]daemon.AgentEntry{"claude": {Path: f.env.wrapper}}},
		Token: f.ordinaryToken, RuntimeID: f.ordinaryRuntime, CLIPath: f.env.cli,
		HTTPClient: &http.Client{Transport: f.transport}}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(f.driver.Close)
	return f.driver
}

func (f *rk1kFixture) chat(token string) string {
	f.ordinaryDriver()
	var session struct {
		ID string `json:"id"`
	}
	f.human(token, "POST", "/api/chat/sessions", map[string]any{"agent_id": f.ordinaryAgent,
		"title": "RK1K private chat " + uuid.NewString(), "project_id": f.project}, nil).Want(201).JSON(&session)
	return session.ID
}

func (f *rk1kFixture) chatMessage(token, chat string) string {
	var out struct {
		Task string `json:"task_id"`
	}
	f.human(token, "POST", "/api/chat/sessions/"+chat+"/messages", map[string]any{
		"content": "Report the active approved policy for this assignment.", "attachment_ids": []string{}}, nil).Want(201).JSON(&out)
	return out.Task
}

func (f *rk1kFixture) actualTaskIDs(where string, args ...any) []string {
	f.t.Helper()
	rows, err := f.base.managed.pool.Query(f.t.Context(), "SELECT id::text FROM agent_task_queue WHERE "+where+" ORDER BY created_at,id", args...)
	if err != nil {
		f.t.Fatal(err)
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			f.t.Fatal(err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		f.t.Fatal(err)
	}
	return ids
}

func (f *rk1kFixture) producer(surface string) []string {
	f.ordinaryDriver()
	switch surface {
	case "chat":
		return []string{f.chatMessage(f.base.contributorToken, f.chat(f.base.contributorToken))}
	case "quick_create":
		var out struct {
			Task string `json:"task_id"`
		}
		f.human(f.base.contributorToken, "POST", "/api/issues/quick-create", map[string]any{"agent_id": f.ordinaryAgent,
			"project_id": f.project, "prompt": "Create one inert backlog issue in the current project; report the active approved policy."}, nil).Want(202).JSON(&out)
		return []string{out.Task}
	case "autopilot":
		var definition struct {
			ID string `json:"id"`
		}
		f.human(f.base.contributorToken, "POST", "/api/autopilots", map[string]any{"title": "RK1K manual automation " + uuid.NewString(),
			"description": "Report the active approved policy.", "assignee_type": "agent", "assignee_id": f.ordinaryAgent,
			"execution_mode": "run_only", "project_id": f.project, "subscribers": []string{}}, nil).Want(201).JSON(&definition)
		var run struct {
			Task *string `json:"task_id"`
		}
		f.human(f.base.contributorToken, "POST", "/api/autopilots/"+definition.ID+"/trigger", nil,
			map[string]string{"Idempotency-Key": uuid.NewString()}).Want(200).JSON(&run)
		if run.Task == nil {
			return nil
		}
		return []string{*run.Task}
	case "issue", "comment":
		body := map[string]any{"title": "RK1K " + surface + " " + uuid.NewString(),
			"description": "Report the active approved policy.", "status": "todo", "priority": "none", "project_id": f.project}
		if surface == "issue" {
			body["assignee_type"], body["assignee_id"] = "agent", f.ordinaryAgent
		}
		var issue struct {
			ID string `json:"id"`
		}
		f.human(f.base.contributorToken, "POST", "/api/issues", body, nil).Want(201).JSON(&issue)
		if surface == "issue" {
			return f.actualTaskIDs("issue_id=$1 AND agent_id=$2", issue.ID, f.ordinaryAgent)
		}
		var comment struct {
			ID string `json:"id"`
		}
		f.human(f.base.contributorToken, "POST", "/api/issues/"+issue.ID+"/comments",
			map[string]any{"content": "[@Agent](mention://agent/" + f.ordinaryAgent + ") Report the active approved policy.",
				"type": "comment", "attachment_ids": []string{}, "suppress_agent_ids": []string{}}, nil).Want(201).JSON(&comment)
		return f.actualTaskIDs("trigger_comment_id=$1 AND agent_id=$2", comment.ID, f.ordinaryAgent)
	}
	f.t.Fatal("unknown registered ordinary producer")
	return nil
}

func (f *rk1kFixture) newConsumer(label string, squad bool) (*rk1Fixture, string, string, string) {
	t := f.t
	var issue struct {
		ID string `json:"id"`
	}
	f.human(f.base.ownerToken, "POST", "/api/collaboration/issues", map[string]any{
		"source_workspace_id": f.base.managed.fx.WorkspaceID, "handler_user_id": f.base.contributor,
		"title": "RK1K consumer " + label, "description": "Report the active approved policy.", "project_id": f.project}, nil).Want(201).JSON(&issue)
	copy := *f.rk1Fixture
	copy.issue, copy.targets, copy.targetSet = issue.ID, map[string]string{}, 1
	copy.dir = filepath.Join(f.dir, "consumer-"+label)
	if err := os.Mkdir(copy.dir, 0700); err != nil {
		t.Fatal(err)
	}
	target, leader, member := "", "", ""
	if squad {
		target, leader, member = copy.squadTarget(label)
	} else {
		target, leader = copy.agentTarget(label)
	}
	return &copy, target, leader, member
}

func (f *rk1kFixture) submitConsumer(h *rk1Fixture, target, kind, resource string) {
	var detail map[string]any
	f.human(f.base.contributorToken, "GET", "/api/collaboration/issues/"+h.issue, nil, nil).Want(200).JSON(&detail)
	write, ok := detail["write_context"].(map[string]any)
	if !ok {
		f.t.Fatal("consumer write context missing")
	}
	targetRow := f.consumerTarget(h, target)
	f.human(f.base.contributorToken, "POST", "/api/collaboration/issues/"+h.issue+"/comments", map[string]any{
		"execution_view_version": 6, "content": "[@Resource](mention://" + kind + "/" + resource + ") Report the active approved policy.",
		"suppressed_target_ids": []string{}, "expected_lifecycle_epoch": detail["lifecycle_epoch"],
		"expected_participant_epoch": write["participant_epoch"], "expected_account_epoch": write["account_epoch"],
		"target_versions": []map[string]string{{"target_id": target,
			"expected_target_epoch":   strconv.FormatInt(targetRow.TargetEpoch, 10),
			"expected_config_version": strconv.FormatInt(targetRow.ConfigVersion, 10)}}}, nil).Want(201)
	h.control(false)
}

func (f *rk1kFixture) consumerTarget(h *rk1Fixture, id string) db.CollaborationTarget {
	f.t.Helper()
	session, err := f.q.GetCollaborationSession(f.t.Context(), util.MustParseUUID(h.issue))
	if err != nil {
		f.t.Fatal(err)
	}
	targets, err := f.q.ListCollaborationTargets(f.t.Context(), session.ID)
	if err != nil {
		f.t.Fatal(err)
	}
	for _, target := range targets {
		if util.UUIDToString(target.ID) == id {
			return target
		}
	}
	f.t.Fatal("consumer's actual target is unavailable")
	return db.CollaborationTarget{}
}

type rk1kTransport struct {
	mu       sync.Mutex
	before   func(*http.Request, []byte)
	after    func(*http.Request, []byte, int)
	dropPath string
	dropped  bool
}

func (x *rk1kTransport) hooks(before func(*http.Request, []byte), after func(*http.Request, []byte, int)) {
	x.mu.Lock()
	defer x.mu.Unlock()
	x.before, x.after = before, after
}

func (x *rk1kTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	var body []byte
	if request.Body != nil {
		var err error
		body, err = io.ReadAll(request.Body)
		if err != nil {
			return nil, err
		}
		request.Body = io.NopCloser(bytes.NewReader(body))
	}
	x.mu.Lock()
	before := x.before
	x.mu.Unlock()
	if before != nil {
		before(request, body)
	}
	response, err := http.DefaultTransport.RoundTrip(request)
	if err != nil {
		return nil, err
	}
	raw, err := io.ReadAll(response.Body)
	closeErr := response.Body.Close()
	if err != nil || closeErr != nil {
		return nil, errors.Join(err, closeErr)
	}
	response.Body = io.NopCloser(bytes.NewReader(raw))
	x.mu.Lock()
	after := x.after
	drop := response.StatusCode >= 200 && response.StatusCode < 300 && x.dropPath == request.URL.Path && !x.dropped
	if drop {
		x.dropped = true
	}
	x.mu.Unlock()
	if after != nil {
		after(request, raw, response.StatusCode)
	}
	if drop {
		return nil, io.ErrUnexpectedEOF
	}
	return response, nil
}
