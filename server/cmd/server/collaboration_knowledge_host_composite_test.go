//go:build knowledgeintegration

package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/multica-ai/multica/server/internal/analytics"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/cli"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/daemon/collaborationrun"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/handler"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	"github.com/multica-ai/multica/server/pkg/agent"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

const hostCompositePrivate = "PRIVATE_HOST_COMPOSITE_APPROVED"

type hostCompositeCapture struct {
	RunID              string   `json:"run_id"`
	Args               []string `json:"args"`
	Cwd                string   `json:"cwd"`
	Input              string   `json:"input"`
	BusinessCredential bool     `json:"business_credential"`
	BroadCredential    bool     `json:"broad_credential"`
}

// Children exit before the real cmd/server TestMain can initialize any fixture.
// This is a copy of the test binary, never an installed provider executable.
func init() {
	path := os.Getenv("COLLAB_HOST_COMPOSITE_CAPTURE")
	if path == "" {
		// A stripped marker must not turn our copied provider binary into a
		// cmd/server test process that initializes database fixtures.
		if strings.TrimSuffix(filepath.Base(os.Args[0]), ".exe") == "host-composite-fake" {
			os.Exit(26)
		}
		return
	}
	if !slices.Contains(os.Args[1:], "-p") {
		os.Exit(25)
	}
	input, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		os.Exit(21)
	}
	run, err := cli.ReadCollaborationContext()
	if err != nil || run == nil {
		os.Exit(22)
	}
	cwd, err := os.Getwd()
	if err != nil {
		os.Exit(23)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		os.Exit(24)
	}
	_ = json.NewEncoder(f).Encode(hostCompositeCapture{RunID: run.Identity.RunID, Args: os.Args[1:], Cwd: cwd, Input: input, BusinessCredential: strings.HasPrefix(run.Token, "mcb_"), BroadCredential: os.Getenv("MULTICA_TOKEN") != "" || os.Getenv("MULTICA_DAEMON_TOKEN") != ""})
	_ = f.Close()
	// Echo approved private stdin as a real child might do on failure. The
	// production private-input policy must retain only bounded run evidence.
	_, _ = fmt.Fprintln(os.Stderr, input)
	if os.Getenv("COLLAB_HOST_COMPOSITE_FAIL") == "1" {
		_, _ = fmt.Fprintln(os.Stderr, strings.Repeat("old-private-diagnostic ", 400)+hostCompositePrivate)
		os.Exit(2)
	}
	native := uuid.NewString()
	_, _ = fmt.Fprintf(os.Stdout, "{\"type\":\"system\",\"subtype\":\"init\",\"session_id\":%q}\n", native)
	_, _ = fmt.Fprintln(os.Stdout, `{"type":"assistant","message":{"model":"composite-fixture","content":[{"type":"text","text":"Composite public result"}],"usage":{"input_tokens":17,"output_tokens":4}}}`)
	_, _ = fmt.Fprintf(os.Stdout, "{\"type\":\"result\",\"is_error\":false,\"session_id\":%q,\"result\":\"Composite public result\"}\n", native)
	os.Exit(0)
}

func TestCollaborationKnowledgeHostCompositeLostMarkerCannotEnterTestMain(t *testing.T) {
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(self)
	if err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(t.TempDir(), "host-composite-fake")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	if err = os.WriteFile(bin, raw, 0700); err != nil {
		t.Fatal(err)
	}
	cmd := exec.CommandContext(t.Context(), bin, "-p")
	// The negative regression itself remains safe if its basename guard is
	// accidentally removed: TestMain's independent empty-DB guard exits first.
	cmd.Env = append(os.Environ(), "COLLAB_HOST_COMPOSITE_CAPTURE=", "DATABASE_URL=")
	output, err := cmd.CombinedOutput()
	exit, ok := err.(*exec.ExitError)
	if !ok || exit.ExitCode() != 26 || len(output) != 0 {
		t.Fatalf("lost marker entered test initialization: exit=%v output_bytes=%d", err, len(output))
	}
}

type hostCompositeLog struct {
	mu sync.Mutex
	bytes.Buffer
}

func (l *hostCompositeLog) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.Buffer.Write(p)
}
func (l *hostCompositeLog) text() string { l.mu.Lock(); defer l.mu.Unlock(); return l.Buffer.String() }

type hostCompositeCall struct {
	Path, Query, Credential string
	Request, Response       []byte
	Status                  int
}
type hostCompositeTransport struct {
	mu       sync.Mutex
	calls    []hostCompositeCall
	before   func(string, []byte)
	after    func(string, []byte)
	dropPath string
	dropped  bool
}

func (x *hostCompositeTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	var request []byte
	if r.Body != nil {
		var err error
		request, err = io.ReadAll(r.Body)
		if err != nil {
			return nil, err
		}
		r.Body = io.NopCloser(bytes.NewReader(request))
	}
	x.mu.Lock()
	before := x.before
	x.mu.Unlock()
	if before != nil {
		before(r.URL.Path, request)
	}
	response, err := http.DefaultTransport.RoundTrip(r)
	if err != nil {
		return nil, err
	}
	raw, err := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if err != nil {
		return nil, err
	}
	response.Body = io.NopCloser(bytes.NewReader(raw))
	credential, _, _ := strings.Cut(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "), "_")
	x.mu.Lock()
	x.calls = append(x.calls, hostCompositeCall{Path: r.URL.Path, Query: r.URL.RawQuery, Credential: credential, Request: request, Response: raw, Status: response.StatusCode})
	after := x.after
	drop := response.StatusCode == 200 && x.dropPath == r.URL.Path && !x.dropped
	if drop {
		x.dropped = true
	}
	x.mu.Unlock()
	if after != nil {
		after(r.URL.Path, raw)
	}
	if drop {
		return nil, io.ErrUnexpectedEOF
	}
	return response, nil
}
func (x *hostCompositeTransport) snapshot() []hostCompositeCall {
	x.mu.Lock()
	defer x.mu.Unlock()
	return append([]hostCompositeCall(nil), x.calls...)
}

type hostCompositeFixture struct {
	base               *collaborationRouteFixture
	h                  *handler.Handler
	svc                *service.CollaborationService
	q                  *db.Queries
	owner, contributor context.Context
	host               *collaborationrun.HostExecutor
	runner             *collaborationrun.Runner
	transport          *hostCompositeTransport
	request            protocol.CollaborationRunClaimRequest
	proposals          []protocol.CollaborationKnowledgeProposal
	inputs             []string
	capture            string
	log                *hostCompositeLog
}

func newHostCompositeFixture(t *testing.T, ttl ...time.Duration) *hostCompositeFixture {
	t.Helper()
	cfg, err := pgxpool.ParseConfig(strings.TrimSpace(os.Getenv("DATABASE_URL")))
	if err != nil || !strings.HasPrefix(cfg.ConnConfig.Database, "multica_knowledge_host_composite_20261101_") {
		t.Fatal("host composite requires its explicit owned database")
	}
	var database string
	if testPool == nil || testPool.QueryRow(t.Context(), "SELECT current_database()").Scan(&database) != nil || database != cfg.ConnConfig.Database {
		t.Fatal("host composite database identity mismatch")
	}
	t.Logf("database=%s", database)
	b := newCollaborationRouteFixture(t)
	// The shared router fixture shadows workspace; keep project in that same
	// schema so its legacy public-table FK cannot target an unrelated fixture.
	b.managed.fx.Exec(t, `CREATE TABLE project (LIKE public.project INCLUDING ALL)`)
	cloneHostCompositeTriggers(t, b.managed)
	// dbfx's initial issue does not allocate through the production counter.
	// Subsequent real Create calls must start after that existing fixture row.
	b.managed.fx.Exec(t, `UPDATE workspace SET issue_counter=(SELECT COALESCE(max(number),0) FROM issue WHERE workspace_id=workspace.id) WHERE id=$1`, b.managed.fx.WorkspaceID)
	router, h := NewRouterWithOptions(b.managed.pool, nil, events.New(), analytics.NoopClient{}, nil, RouterOptions{})
	h.TaskService.KnowledgeContextEnabled = true
	b.managed.router = router
	b.server.Config.Handler = router
	f := &hostCompositeFixture{base: b, h: h, q: db.New(b.managed.pool), transport: &hostCompositeTransport{}, capture: filepath.Join(t.TempDir(), "stdin.jsonl"), log: &hostCompositeLog{}}
	f.svc = service.NewCollaborationService(f.q, b.managed.pool, h.IssueService)
	f.svc.KnowledgeContextEnabled = true
	actor := func(id string) context.Context {
		return auth.WithPasswordSession(auth.WithAuthenticatedPrincipal(context.Background(), auth.AuthenticatedPrincipal{UserID: id, Kind: auth.CredentialJWT}), auth.PasswordSession{UserID: id, Version: 1, Kind: "jwt"})
	}
	f.owner, f.contributor = actor(b.owner), actor(b.contributor)
	b.managed.fx.Exec(t, `UPDATE collaboration_execution_gate SET creation_enabled=true,host_execution_enabled=true,epoch=epoch+1 WHERE id=1`)
	installationOS := runtime.GOOS
	if installationOS == "darwin" {
		installationOS = "macos"
	}
	b.managed.fx.Exec(t, `UPDATE managed_installation SET os=$2 WHERE id=$1`, b.installationID, installationOS)
	project := b.managed.fx.Project(t, "Composite host project", testutil.Cols{"description": "Independent project base"})
	source, err := f.svc.Create(f.owner, service.CollaborationCreateParams{SourceWorkspaceID: util.MustParseUUID(b.managed.fx.WorkspaceID), HandlerID: util.MustParseUUID(b.contributor), ProjectID: util.MustParseUUID(project), RequestID: uuid.NewString(), Title: "Host composite accepted source"})
	if err != nil {
		t.Fatal(err)
	}
	b.issue, b.session = util.UUIDToString(source.Issue.ID), util.UUIDToString(source.SessionID)
	oldRuntime, err := f.q.GetAgentRuntime(t.Context(), util.MustParseUUID(b.runtime))
	if err != nil {
		t.Fatal(err)
	}
	b.runtime = b.managed.fx.Runtime(t, "Composite built-in Claude", testutil.Cols{"owner_id": b.owner, "provider": "claude", "runtime_mode": "local", "daemon_id": oldRuntime.DaemonID.String, "status": "online"})
	b.agent = b.managed.fx.Agent(t, "Composite Claude agent", b.runtime, testutil.Cols{"owner_id": b.owner, "permission_mode": "private", "instructions": "Follow the admitted assignment."})
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(self)
	if err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(t.TempDir(), "host-composite-fake")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	if err = os.WriteFile(bin, raw, 0700); err != nil {
		t.Fatal(err)
	}
	hostConfig := &collaborationrun.HostConfig{Provider: "claude", ProviderVersion: "2.1.200", ProviderMode: "local", AccountSource: "test-owned-host-composite", Executable: bin, Limits: protocol.CollaborationPreparationLimits{FileCount: 32, PerFileBytes: 100 << 20, TotalInputBytes: 256 << 20, WorkspaceBytes: 512 << 20, GitExpandedBytes: 256 << 20, EntryCount: 4096, TimeoutSeconds: 60}}
	caps, err := hostConfig.Capabilities()
	if err != nil {
		t.Fatal(err)
	}
	runtimeRow, err := f.q.GetAgentRuntime(t.Context(), util.MustParseUUID(b.runtime))
	if err != nil {
		t.Fatal(err)
	}
	registration := protocol.CollaborationRuntimeCapabilitiesRequest{ProtocolVersion: 3, DeploymentID: b.deployment, InstallationID: b.installationID, RuntimeID: b.runtime, BindingID: b.binding.BindingID, BindingEpoch: b.binding.BindingEpoch, ExpectedRuntimeConfigEpoch: strconv.FormatInt(runtimeRow.CollaborationConfigEpoch, 10), Capabilities: caps}
	b.managed.request(t, b.binding.DaemonToken, false, "POST", "/api/collaboration/runtime/capabilities", registration, nil).Want(200)
	bindingEpoch, err := strconv.ParseInt(b.binding.BindingEpoch, 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	queue := f.queueRequest(b.issue)
	target, err := f.svc.RegisterTarget(f.owner, service.CollaborationRegisterTargetParams{CollaborationQueueRequest: queue, Kind: collaboration.TargetAgent, ResourceID: util.MustParseUUID(b.agent), ExpectedTargetSetVersion: 1, Roster: []collaboration.TargetResource{{Role: "agent", AgentID: b.agent, RuntimeID: b.runtime, OwnerUserID: b.owner, ExecutionWorkspaceID: b.managed.fx.WorkspaceID}}})
	if err != nil {
		t.Fatal(err)
	}
	b.target = util.UUIDToString(target.TargetID)
	_, err = f.svc.ConsentTargetResource(f.owner, service.CollaborationConsentParams{CollaborationQueueRequest: f.queueRequest(b.issue), TargetID: target.TargetID, AgentID: util.MustParseUUID(b.agent), ExpectedTargetEpoch: 1, ExpectedConfigVersion: 1, Identity: service.CollaborationResourceIdentity{Kind: "managed", InstallationID: util.MustParseUUID(b.installationID), BindingID: util.MustParseUUID(b.binding.BindingID), BindingEpoch: bindingEpoch}})
	if err != nil {
		t.Fatal(err)
	}
	for i, kind := range []string{"project", "workspace"} {
		knowledgeSource, err := f.svc.Create(f.owner, service.CollaborationCreateParams{SourceWorkspaceID: util.MustParseUUID(b.managed.fx.WorkspaceID), HandlerID: util.MustParseUUID(b.contributor), ProjectID: util.MustParseUUID(project), RequestID: uuid.NewString(), Title: "Approved composite knowledge source"})
		if err != nil {
			t.Fatal(err)
		}
		text := fmt.Sprintf("Host composite source version %d", i)
		if _, err = f.svc.UpdateMemory(f.owner, service.CollaborationMemoryUpdateParams{IssueID: knowledgeSource.Issue.ID, RequestID: uuid.NewString(), ExpectedVersion: 0, Content: text}); err != nil {
			t.Fatal(err)
		}
		id := uuid.NewString()
		if i == 0 {
			id = "f" + id[1:]
		} else {
			id = "0" + id[1:]
		}
		p := protocol.CollaborationKnowledgeProposal{ProtocolVersion: 1, CandidateID: id, RequestID: uuid.NewString(), Target: protocol.CollaborationKnowledgeTarget{Kind: kind, WorkspaceID: b.managed.fx.WorkspaceID}, Summary: "Composite approved summary", Content: hostCompositePrivate + "_" + kind, Sources: []protocol.CollaborationKnowledgeSource{{Kind: "memory", IssueID: util.UUIDToString(knowledgeSource.Issue.ID), Version: "1", SHA256: protocol.CollaborationKnowledgeHash(text)}}, Evidence: []protocol.CollaborationKnowledgeEvidence{}, Applicability: protocol.CollaborationKnowledgeApplicability{Surfaces: []string{"collaboration"}}}
		if kind == "project" {
			p.Target.ProjectID = project
		}
		if i == 0 && len(ttl) > 0 {
			expiry := time.Now().UTC().Add(ttl[0])
			p.ExpiresAt = &expiry
		}
		if _, err = f.svc.ProposeKnowledge(f.owner, knowledgeSource.Issue.ID, p); err != nil {
			t.Fatal(err)
		}
		if _, err = f.svc.ApproveKnowledge(f.owner, util.MustParseUUID(id), protocol.CollaborationKnowledgeApproveRequest{ProtocolVersion: 1, RequestID: uuid.NewString(), ExpectedStateRevision: "1", ExpectedVersion: "1", ExpectedContentSHA256: protocol.CollaborationKnowledgeHash(p.Content)}); err != nil {
			t.Fatal(err)
		}
		f.proposals = append(f.proposals, p)
	}
	f.inputs = []string{"Original owner input", "Original external contributor input"}
	f.submit(t, f.owner, f.inputs[0])
	f.submit(t, f.contributor, f.inputs[1])
	session, err := f.q.GetCollaborationSession(t.Context(), source.Issue.ID)
	if err != nil {
		t.Fatal(err)
	}
	targets, err := f.q.ListCollaborationTargets(t.Context(), source.SessionID)
	if err != nil || len(targets) != 1 {
		t.Fatal("unexpected target fixture")
	}
	current := targets[0]
	gate, err := f.q.GetCollaborationGate(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.svc.ResumeQueue(f.owner, service.CollaborationResumeQueueParams{CollaborationStopQueueParams: service.CollaborationStopQueueParams{CollaborationQueueRequest: f.queueRequest(b.issue), Scope: collaboration.ControlIssue, ExpectedControlVersion: session.ControlVersion, ExpectedTargetSetVersion: session.TargetSetVersion, Targets: []service.CollaborationTargetControl{{TargetID: current.ID, ExpectedTargetEpoch: current.TargetEpoch, ExpectedConfigVersion: current.ConfigVersion, ExpectedControlVersion: current.ControlVersion, ExpectedHoldVersion: current.HoldVersion, ObservedGroupID: current.ActiveGroupID, ObservedGroupGeneration: current.GroupGeneration}}}, ExpectedExecutionEpoch: gate.Epoch})
	if err != nil {
		t.Fatal(err)
	}
	digest, err := protocol.CollaborationRuntimeCapabilityDigest(caps)
	if err != nil {
		t.Fatal(err)
	}
	f.request = protocol.CollaborationRunClaimRequest{ProtocolVersion: 3, DeploymentID: b.deployment, InstallationID: b.installationID, RuntimeID: b.runtime, BindingID: b.binding.BindingID, BindingEpoch: b.binding.BindingEpoch, Containment: protocol.CollaborationContainmentHostProcessSupervision, PreparationVersion: 2, CapabilityDigest: digest}
	client, err := collaborationrun.NewClient(b.server.URL, b.binding.DaemonToken, f.transport)
	if err != nil {
		t.Fatal(err)
	}
	ledger, err := collaborationrun.OpenLedger(filepath.Join(t.TempDir(), "ledger"))
	if err != nil {
		t.Fatal(err)
	}
	f.host = &collaborationrun.HostExecutor{Config: hostConfig, Directory: filepath.Join(t.TempDir(), "host-work"), CLIExecutable: bin, Logger: slog.New(slog.NewTextHandler(f.log, &slog.HandlerOptions{Level: slog.LevelDebug})), Options: agent.ExecOptions{Timeout: 15 * time.Second}}
	f.runner = &collaborationrun.Runner{Ledger: ledger, Client: client, Host: f.host, StopPollInterval: 20 * time.Millisecond, PermitAttempts: 2, PermitBackoff: time.Millisecond}
	t.Setenv("COLLAB_HOST_COMPOSITE_CAPTURE", f.capture)
	t.Setenv("MULTICA_TOKEN", "ordinary-broad-credential-must-not-enter-child")
	t.Setenv("MULTICA_DAEMON_TOKEN", "managed-broad-credential-must-not-enter-child")
	return f
}

// LIKE INCLUDING ALL preserves indexes/checks, but not triggers. Replay the
// actual migrated definitions so capability epochs and immutable knowledge
// behavior in this composite are the same as the production tables.
func cloneHostCompositeTriggers(t *testing.T, f *platformAdminRouterFixture) {
	t.Helper()
	rows, err := f.pool.Query(t.Context(), `SELECT replace(pg_get_triggerdef(t.oid),
		' ON public.'||quote_ident(c.relname)||' ',
		' ON '||quote_ident(current_schema())||'.'||quote_ident(c.relname)||' ')
		FROM pg_trigger t JOIN pg_class c ON c.oid=t.tgrelid JOIN pg_namespace n ON n.oid=c.relnamespace
		WHERE n.nspname='public' AND NOT t.tgisinternal
		AND to_regclass(format('%I.%I',current_schema(),c.relname)) IS NOT NULL
		AND NOT EXISTS(SELECT 1 FROM pg_trigger present WHERE present.tgrelid=to_regclass(format('%I.%I',current_schema(),c.relname)) AND present.tgname=t.tgname)
		ORDER BY c.relname,t.tgname`)
	if err != nil {
		t.Fatal(err)
	}
	var statements []string
	for rows.Next() {
		var sql string
		if err = rows.Scan(&sql); err != nil {
			rows.Close()
			t.Fatal(err)
		}
		statements = append(statements, sql)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		t.Fatal(err)
	}
	rows.Close()
	for _, sql := range statements {
		f.fx.Exec(t, sql)
	}
	if len(statements) == 0 {
		t.Fatal("composite did not install migrated production triggers")
	}
}

func (f *hostCompositeFixture) queueRequest(issue string) service.CollaborationQueueRequest {
	return service.CollaborationQueueRequest{ExecutionViewVersion: 6, IssueID: util.MustParseUUID(issue), RequestID: uuid.NewString(), ExpectedLifecycleEpoch: 1, ExpectedParticipantEpoch: 1, ExpectedAccountEpoch: 1}
}
func (f *hostCompositeFixture) submit(t *testing.T, actor context.Context, body string) {
	t.Helper()
	_, err := f.svc.SubmitCommand(actor, service.CollaborationSubmitCommandParams{CollaborationQueueRequest: f.queueRequest(f.base.issue), TargetID: util.MustParseUUID(f.base.target), ExpectedTargetEpoch: 1, ExpectedConfigVersion: 1, InputEventID: util.MustParseUUID(uuid.NewString()), Content: body})
	if err != nil {
		t.Fatal(err)
	}
}
func (f *hostCompositeFixture) nextRun(t *testing.T) string {
	t.Helper()
	list, err := f.runner.Client.List(t.Context(), f.base.runtime)
	if err != nil || len(list.RunIDs) != 1 {
		var state string
		f.base.managed.fx.QueryRow(t, `SELECT jsonb_build_object('session_queue',s.queue_state,'target_queue',t.queue_state,'hold',t.hold_reason,'commands',(SELECT jsonb_agg(jsonb_build_object('state',c.state,'reason',c.reason)) FROM collaboration_command c WHERE c.session_id=s.id))::text FROM collaboration_session s JOIN collaboration_target t ON t.session_id=s.id WHERE s.id=$1 AND t.id=$2`, f.base.session, f.base.target).Scan(&state)
		t.Logf("pending admission metadata=%s", state)
		t.Fatalf("real pending run count=%d err=%v", len(list.RunIDs), err)
	}
	return list.RunIDs[0]
}
func (f *hostCompositeFixture) captures(t *testing.T) []hostCompositeCapture {
	t.Helper()
	raw, err := os.ReadFile(f.capture)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	var out []hostCompositeCapture
	scan := bufio.NewScanner(bytes.NewReader(raw))
	scan.Buffer(make([]byte, 4096), 4<<20)
	for scan.Scan() {
		var c hostCompositeCapture
		if err = json.Unmarshal(scan.Bytes(), &c); err != nil {
			t.Fatal(err)
		}
		out = append(out, c)
	}
	if err = scan.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}
func hostCompositePrompt(t *testing.T, c hostCompositeCapture) string {
	t.Helper()
	var frame struct {
		Message struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"message"`
	}
	if json.Unmarshal([]byte(c.Input), &frame) != nil || len(frame.Message.Content) != 1 {
		t.Fatal("fake child did not read real stream-json stdin")
	}
	return frame.Message.Content[0].Text
}

func TestCollaborationKnowledgeHostCompositeHappyPath(t *testing.T) {
	f := newHostCompositeFixture(t)
	run := f.nextRun(t)
	if err := f.runner.Run(t.Context(), run, f.request); err != nil {
		t.Fatal(err)
	}
	starts := f.captures(t)
	if len(starts) != 1 {
		t.Fatalf("actual physical starts=%d", len(starts))
	}
	start := starts[0]
	prompt := hostCompositePrompt(t, start)
	if start.RunID != run || !start.BusinessCredential || start.BroadCredential || !slices.Contains(start.Args, "--no-session-persistence") || slices.Contains(start.Args, "--resume") {
		t.Fatal("real host invocation lost run/credential/freshness contract")
	}
	for _, p := range f.proposals {
		if !strings.Contains(prompt, p.Content) {
			t.Fatal("approved canonical contribution missing from actual stdin")
		}
	}
	for _, input := range f.inputs {
		if !strings.Contains(prompt, input) {
			t.Fatal("knowledge displaced original accepted input")
		}
	}
	if !strings.Contains(prompt, f.base.owner) || !strings.Contains(prompt, f.base.contributor) {
		t.Fatal("accepted author attribution lost")
	}
	var claim protocol.CollaborationRunClaimResponse
	var bundle protocol.CollaborationRunKnowledgeBundle
	var launch protocol.CollaborationRunKnowledgeAuthorization
	for _, call := range f.transport.snapshot() {
		if strings.HasSuffix(call.Path, "/claim") {
			if call.Status != 200 || call.Credential != "mdt" || !strings.Contains(call.Query, "knowledge_version=1") {
				t.Fatal("real claim did not negotiate managed knowledge")
			}
			if json.Unmarshal(call.Response, &claim) != nil {
				t.Fatal("claim response malformed")
			}
		}
		if strings.HasSuffix(call.Path, "/knowledge-context/prepare") {
			if json.Unmarshal(call.Response, &bundle) != nil {
				t.Fatal("private prepare response malformed")
			}
		}
		if strings.HasSuffix(call.Path, "/knowledge-context/authorize") {
			if call.Status != 200 || json.Unmarshal(call.Response, &launch) != nil {
				t.Fatal("real launch authorization failed")
			}
		}
	}
	if claim.KnowledgeContext == nil || claim.KnowledgeContext.Namespace != run || len(bundle.Contributions) != 2 || bundle.Contributions[0].CandidateID != f.proposals[1].CandidateID || launch.PromptSHA256 != protocol.CollaborationHandoffHash(prompt) {
		t.Fatal("real claim/prepare/launch did not bind canonical actual input")
	}
	state, err := f.q.GetCollaborationRuntimeRun(t.Context(), util.MustParseUUID(run))
	if err != nil || state.ProgressState != "settled" || state.ProofState != "host_exit_observed" || state.BusinessOutcome.String != "success" {
		t.Fatalf("real run did not settle with physical proof: %v %s %s", err, state.ProgressState, state.ProofState)
	}
	brief, err := os.ReadFile(filepath.Join(start.Cwd, "CLAUDE.md"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(brief), hostCompositePrivate) || strings.Contains(f.log.text(), hostCompositePrivate) {
		t.Fatal("private stdin escaped into stable context or generic debug logs")
	}
	if f.base.managed.fx.Count(t, `SELECT count(*) FROM collaboration_run_event WHERE run_id=$1 AND kind='knowledge_preparation'`, run) != 1 || f.base.managed.fx.Count(t, `SELECT count(*) FROM collaboration_run_event WHERE run_id=$1 AND kind='knowledge_authorization'`, run) != 1 {
		t.Fatal("real private ledger facts missing")
	}
}

func (f *hostCompositeFixture) claimFor(t *testing.T, run string) protocol.CollaborationRunClaimResponse {
	t.Helper()
	for _, call := range f.transport.snapshot() {
		if call.Path == "/api/collaboration/runtime/runs/"+run+"/claim" && call.Status == 200 {
			var claim protocol.CollaborationRunClaimResponse
			if json.Unmarshal(call.Response, &claim) != nil {
				t.Fatal("invalid actual claim")
			}
			return claim
		}
	}
	t.Fatal("no actual managed claim recorded")
	return protocol.CollaborationRunClaimResponse{}
}

func (f *hostCompositeFixture) invalidate(kind string) error {
	p := f.proposals[0]
	source := util.MustParseUUID(p.Sources[0].IssueID)
	switch kind {
	case "withdraw":
		_, err := f.svc.WithdrawKnowledge(f.owner, util.MustParseUUID(p.CandidateID), protocol.CollaborationKnowledgeDecisionRequest{ProtocolVersion: 1, RequestID: uuid.NewString(), ExpectedStateRevision: "2", Reason: "Composite withdrawal before physical launch"})
		return err
	case "purge":
		// The product preserves the current memory version. Replace that base
		// first, then purge the historical version this candidate still needs.
		if _, err := f.svc.UpdateMemory(f.owner, service.CollaborationMemoryUpdateParams{IssueID: source, RequestID: uuid.NewString(), ExpectedVersion: 1, Content: "Replacement source base before historical purge"}); err != nil {
			return err
		}
		_, err := f.svc.PurgeMemoryVersion(f.owner, service.CollaborationMemoryPurgeParams{IssueID: source, RequestID: uuid.NewString(), Version: 1, Reason: "Composite source removal"})
		return err
	case "expiry":
		if p.ExpiresAt == nil {
			return fmt.Errorf("expiry fixture missing deadline")
		}
		if wait := time.Until(*p.ExpiresAt) + 10*time.Millisecond; wait > 0 {
			time.Sleep(wait)
		}
		return nil
	case "source_access":
		access, err := f.svc.ResolveIssueAccess(f.owner, f.q, source)
		if err != nil {
			return err
		}
		result, err := f.svc.RemoveParticipant(f.owner, service.CollaborationParticipantParams{CollaborationWorkflowRequest: service.CollaborationWorkflowRequest{CollaborationQueueRequest: f.queueRequest(p.Sources[0].IssueID), ExpectedCollaborationRevision: access.Revision}, TargetUserID: util.MustParseUUID(f.base.contributor)})
		if err == nil && result.Status >= 400 {
			return fmt.Errorf("source removal refused: %s", result.Code)
		}
		return err
	default:
		return fmt.Errorf("unknown invalidation fixture")
	}
}

func TestCollaborationKnowledgeHostCompositeInvalidationBeforeLaterRunLaunch(t *testing.T) {
	for _, kind := range []string{"withdraw", "purge", "expiry", "source_access"} {
		t.Run(kind, func(t *testing.T) {
			var ttl []time.Duration
			if kind == "expiry" {
				ttl = []time.Duration{5 * time.Second}
			}
			f := newHostCompositeFixture(t, ttl...)
			first := f.nextRun(t)
			if err := f.runner.Run(t.Context(), first, f.request); err != nil {
				t.Fatal(err)
			}
			f.submit(t, f.owner, "Second explicit accepted input")
			second := f.nextRun(t)
			var once sync.Once
			var mutationErr error
			f.transport.mu.Lock()
			f.transport.before = func(path string, _ []byte) {
				if path == "/api/collaboration/runtime/runs/"+second+"/knowledge-context/authorize" {
					once.Do(func() { mutationErr = f.invalidate(kind) })
				}
			}
			f.transport.mu.Unlock()
			err := f.runner.Run(t.Context(), second, f.request)
			if mutationErr != nil {
				t.Fatalf("real source mutation failed: %v", mutationErr)
			}
			if err == nil {
				t.Fatal("changed real source authorized stale private launch")
			}
			if len(f.captures(t)) != 1 {
				t.Fatal("refused later run physically replayed accepted input")
			}
			state, e := f.q.GetCollaborationRuntimeRun(t.Context(), util.MustParseUUID(second))
			if e != nil || state.ProgressState != "settled" || state.ProofState != "never_launched_proven" {
				t.Fatalf("refusal stranded original proof: %v %s %s", e, state.ProgressState, state.ProofState)
			}
			firstState, e := f.q.GetCollaborationRuntimeRun(t.Context(), util.MustParseUUID(first))
			if e != nil || firstState.ProofState != "host_exit_observed" || firstState.BusinessOutcome.String != "success" {
				t.Fatal("later refusal changed earlier completed effects")
			}
			var refused bool
			for _, call := range f.transport.snapshot() {
				if call.Path == "/api/collaboration/runtime/runs/"+second+"/knowledge-context/authorize" {
					refused = call.Status == 409
					if strings.Contains(string(call.Response), hostCompositePrivate) {
						t.Fatal("refusal redisclosed private body")
					}
				}
			}
			if !refused {
				t.Fatal("actual knowledge authorization route was not exercised")
			}
			claim := f.claimFor(t, second)
			f.h.TaskService.KnowledgeContextEnabled = false
			status := f.base.managed.request(t, claim.TerminationToken, false, "GET", "/api/collaboration/runtime/runs/"+second+"/knowledge-context/status", nil, nil).Want(200)
			if status.Map()["prepared"] != true || strings.Contains(status.Text(), hostCompositePrivate) || strings.Contains(status.Text(), f.proposals[0].CandidateID) {
				t.Fatal("gate/source loss broke content-free operation recovery")
			}
			before := len(f.transport.snapshot())
			if err = f.runner.Run(t.Context(), second, f.request); err != nil {
				t.Fatal(err)
			}
			for _, call := range f.transport.snapshot()[before:] {
				if strings.Contains(call.Path, "/knowledge-context/prepare") || strings.HasSuffix(call.Path, "/claim") {
					t.Fatal("proof recovery obtained a new input or knowledge snapshot")
				}
			}
		})
	}
}

func TestCollaborationKnowledgeHostCompositeLostResponsesKeepOriginalIdentity(t *testing.T) {
	for _, action := range []string{"prepare", "authorize"} {
		t.Run(action, func(t *testing.T) {
			f := newHostCompositeFixture(t)
			run := f.nextRun(t)
			path := "/api/collaboration/runtime/runs/" + run + "/knowledge-context/" + action
			f.transport.mu.Lock()
			f.transport.dropPath = path
			f.transport.mu.Unlock()
			if err := f.runner.Run(t.Context(), run, f.request); err != nil {
				t.Fatal(err)
			}
			var calls []hostCompositeCall
			for _, call := range f.transport.snapshot() {
				if call.Path == path {
					calls = append(calls, call)
				}
			}
			if len(calls) != 2 || calls[0].Status != 200 || calls[1].Status != 200 || !bytes.Equal(calls[0].Request, calls[1].Request) || !bytes.Equal(calls[0].Response, calls[1].Response) || len(f.captures(t)) != 1 {
				t.Fatal("lost real response replaced the original request/bundle/attempt or replayed input")
			}
			if f.base.managed.fx.Count(t, `SELECT count(*) FROM collaboration_run_event WHERE run_id=$1 AND kind='knowledge_preparation'`, run) != 1 || f.base.managed.fx.Count(t, `SELECT count(*) FROM collaboration_run_event WHERE run_id=$1 AND kind='knowledge_authorization'`, run) != 1 {
				t.Fatal("real response recovery duplicated disclosure/authorization")
			}
		})
	}
}

func TestCollaborationKnowledgeHostCompositeLostResponseRechecksWithdrawnSource(t *testing.T) {
	f := newHostCompositeFixture(t)
	run := f.nextRun(t)
	path := "/api/collaboration/runtime/runs/" + run + "/knowledge-context/prepare"
	var once sync.Once
	var mutationErr error
	f.transport.mu.Lock()
	f.transport.dropPath = path
	f.transport.after = func(p string, raw []byte) {
		if p == path && bytes.Contains(raw, []byte(`"contributions"`)) {
			once.Do(func() { mutationErr = f.invalidate("withdraw") })
		}
	}
	f.transport.mu.Unlock()
	if err := f.runner.Run(t.Context(), run, f.request); err == nil {
		t.Fatal("lost response bypassed current source authorization")
	}
	if mutationErr != nil {
		t.Fatal(mutationErr)
	}
	var calls []hostCompositeCall
	for _, call := range f.transport.snapshot() {
		if call.Path == path {
			calls = append(calls, call)
		}
	}
	if len(calls) != 2 || calls[0].Status != 200 || calls[1].Status != 409 || !bytes.Equal(calls[0].Request, calls[1].Request) || strings.Contains(string(calls[1].Response), hostCompositePrivate) || len(f.captures(t)) != 0 {
		t.Fatal("original-key recovery redisclosed withdrawn source or started input")
	}
	state, err := f.q.GetCollaborationRuntimeRun(t.Context(), util.MustParseUUID(run))
	if err != nil || state.ProofState != "never_launched_proven" {
		t.Fatal("lost private reply stranded termination proof")
	}
}

func TestCollaborationKnowledgeHostCompositeFreshRunsUseDifferentNativeNamespaces(t *testing.T) {
	f := newHostCompositeFixture(t)
	first := f.nextRun(t)
	if err := f.runner.Run(t.Context(), first, f.request); err != nil {
		t.Fatal(err)
	}
	f.submit(t, f.owner, "A new explicit run, never a replay")
	second := f.nextRun(t)
	if err := f.runner.Run(t.Context(), second, f.request); err != nil {
		t.Fatal(err)
	}
	starts := f.captures(t)
	if len(starts) != 2 || starts[0].Cwd == starts[1].Cwd || f.claimFor(t, first).KnowledgeContext.Namespace == f.claimFor(t, second).KnowledgeContext.Namespace {
		t.Fatal("admitted runs reused a native namespace")
	}
	for _, start := range starts {
		if !slices.Contains(start.Args, "--no-session-persistence") || slices.Contains(start.Args, "--resume") || slices.Contains(start.Args, "--continue") {
			t.Fatal("native persistence or resume reached a knowledge invocation")
		}
	}
	claim := f.claimFor(t, second)
	if len(claim.Inputs) != 1 || claim.Inputs[0].Body != "A new explicit run, never a replay" {
		t.Fatal("later run reclaimed previously executed inputs")
	}
}

func TestCollaborationKnowledgeHostCompositeUnnegotiatedAndDowngradedClaimsCannotDisclose(t *testing.T) {
	for _, negotiated := range []bool{false, true} {
		t.Run(map[bool]string{false: "unnegotiated", true: "downgrade"}[negotiated], func(t *testing.T) {
			f := newHostCompositeFixture(t)
			run := f.nextRun(t)
			request := f.request
			if negotiated {
				request.KnowledgeVersion = 1
			}
			claim, err := f.runner.Client.Claim(t.Context(), run, request)
			if err != nil {
				t.Fatal(err)
			}
			if negotiated {
				if claim.KnowledgeContext == nil {
					t.Fatal("knowledge claim was not actually admitted")
				}
				if _, err = f.runner.Client.Claim(t.Context(), run, f.request); err == nil {
					t.Fatal("old daemon downgraded admitted private run")
				}
			} else {
				if claim.KnowledgeContext != nil {
					t.Fatal("unnegotiated claim obtained a private descriptor")
				}
				p := protocol.CollaborationRunKnowledgePrepare{ProtocolVersion: 1, Identity: claim.Identity, RequestID: uuid.NewString(), HandoffID: uuid.NewString(), InvocationFingerprint: strings.Repeat("a", 64)}
				if _, err = f.runner.Client.PrepareKnowledge(t.Context(), p); err == nil {
					t.Fatal("unnegotiated business run obtained private knowledge")
				}
			}
			if err = f.runner.Run(t.Context(), run, f.request); err != nil {
				t.Fatal(err)
			}
			starts := f.captures(t)
			if len(starts) != 1 {
				t.Fatal("claim compatibility replayed physical input")
			}
			contains := strings.Contains(hostCompositePrompt(t, starts[0]), hostCompositePrivate)
			if contains != negotiated {
				t.Fatal("frozen claim negotiation was silently upgraded or downgraded")
			}
		})
	}
}

func TestCollaborationKnowledgeHostCompositePrivateStderrKeepsOnlyBoundedRunEvidence(t *testing.T) {
	f := newHostCompositeFixture(t)
	t.Setenv("COLLAB_HOST_COMPOSITE_FAIL", "1")
	run := f.nextRun(t)
	_ = f.runner.Run(t.Context(), run, f.request)
	if len(f.captures(t)) != 1 || strings.Contains(f.log.text(), hostCompositePrivate) {
		t.Fatal("actual private stderr reached generic logger or fixture did not launch")
	}
	state, err := f.q.GetCollaborationRuntimeRun(t.Context(), util.MustParseUUID(run))
	if err != nil || state.ProofState != "host_exit_observed" || state.BusinessOutcome.String != "failure" {
		t.Fatal("private diagnostic failure lost physical proof/outcome")
	}
	event, err := f.q.GetCollaborationRunRecordMetadata(t.Context(), db.GetCollaborationRunRecordMetadataParams{IssueID: util.MustParseUUID(f.base.issue), RunID: util.MustParseUUID(run)})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range event {
		if e.Kind == "outcome" {
			var out protocol.CollaborationRunOutcome
			if json.Unmarshal(e.Payload, &out) != nil {
				t.Fatal("invalid retained outcome")
			}
			found = strings.Contains(out.Body, hostCompositePrivate)
			if len(out.Body) > 4096 {
				t.Fatal("stderr diagnostic exceeded bounded execution evidence")
			}
		}
	}
	if !found {
		t.Fatal("private stderr was erased from authorized bounded run evidence")
	}
}
