package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
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
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/daemon/collaborationrun"
	"github.com/multica-ai/multica/server/internal/installation"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

const collaborationRouteFakeArg = "__multica_collaboration_route_fake"

// Fixture children exit before the integration TestMain can open a database or
// start a second server. The executable itself is a fresh test-owned copy.
func init() {
	if len(os.Args) == 2 && os.Args[1] == collaborationrun.NamespaceHelperArg {
		os.Exit(collaborationrun.NamespaceHelperMain())
	}
	if len(os.Args) == 4 && os.Args[1] == collaborationRouteFakeArg {
		os.Exit(collaborationRouteFake(os.Args[2], os.Args[3]))
	}
}

func collaborationRouteFake(mode, address string) int {
	var input collaborationrun.ProviderInput
	if err := json.NewDecoder(os.Stdin).Decode(&input); err != nil || len(input.Inputs) == 0 {
		return 20
	}
	connection, err := net.DialTimeout("tcp", address, 5*time.Second)
	if err != nil {
		return 21
	}
	defer connection.Close()
	if err = json.NewEncoder(connection).Encode(input); err != nil {
		return 22
	}
	if mode == "block" {
		var b [1]byte
		_, _ = connection.Read(b[:])
		return 0
	}
	if mode != "succeed" {
		return 23
	}
	if err := json.NewEncoder(os.Stdout).Encode(map[string]string{"outcome": protocol.CollaborationBusinessOutcomeSuccess, "body": fmt.Sprintf("Processed %d immutable inputs", len(input.Inputs))}); err != nil {
		return 24
	}
	return 0
}

type collaborationRouteFixture struct {
	managed                       *platformAdminRouterFixture
	server                        *httptest.Server
	owner, contributor            string
	ownerToken, contributorToken  string
	issue, session, target, agent string
	runtime, deployment           string
	binding                       service.InstallationBindingResult
	installationID                string
}

func newCollaborationRouteFixture(t *testing.T) *collaborationRouteFixture {
	t.Helper()
	f := newManagedRouterFixture(t)
	// Clone the actual migrated tables into the existing router fixture schema.
	// Current-authority joins and historical proof joins must see the same
	// fixture users/issues, never fall through to another package's rows.
	rows, err := testPool.Query(t.Context(), `SELECT tablename FROM pg_tables WHERE schemaname='public'
		AND (tablename LIKE 'collaboration\_%' ESCAPE '\' OR tablename IN ('issue','issue_collaboration','issue_participant','issue_handoff','comment','issue_status','agent_invocation_target','squad','squad_member')) ORDER BY tablename`)
	if err != nil {
		t.Fatal(err)
	}
	var tables []string
	for rows.Next() {
		var table string
		if err = rows.Scan(&table); err != nil {
			rows.Close()
			t.Fatal(err)
		}
		tables = append(tables, table)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		t.Fatal(err)
	}
	rows.Close()
	for _, table := range tables {
		f.fx.Exec(t, "CREATE TABLE "+pgx.Identifier{table}.Sanitize()+" (LIKE "+pgx.Identifier{"public", table}.Sanitize()+" INCLUDING ALL)")
	}
	// LIKE does not copy triggers. Use the production allocator in this
	// schema so native executions have the same serialized turn identities.
	f.fx.Exec(t, `CREATE TRIGGER collaboration_run_leader_turn BEFORE INSERT OR UPDATE ON collaboration_run
		FOR EACH ROW EXECUTE FUNCTION public.collaboration_assign_leader_turn()`)
	f.fx.InsertNoID(t, "collaboration_execution_gate", testutil.Cols{"id": 1, "enabled": true, "creation_enabled": false}, "id=$1", 1)
	owner, contributor := f.user(t, ""), f.user(t, "")
	f.fx.UserID = owner
	ws := f.fx.Workspace(t, "Collaboration vertical", "collaboration-vertical-"+uuid.NewString())
	f.fx.WorkspaceID = ws
	f.fx.Member(t, ws, owner, "owner")
	var organization string
	f.fx.QueryRow(t, "SELECT id::text FROM organization").Scan(&organization)
	f.fx.InsertNoID(t, "organization_workspace", testutil.Cols{"workspace_id": ws, "organization_id": organization}, "workspace_id=$1", ws)
	ownerToken := platformRouterJWT(t, owner, nil)
	pat, err := auth.GeneratePATToken()
	if err != nil {
		t.Fatal(err)
	}
	f.fx.Insert(t, "personal_access_token", testutil.Cols{"user_id": owner, "name": "Fixture daemon binding", "token_hash": auth.HashToken(pat), "token_prefix": "mul_", "auth_version": 1, "expires_at": time.Now().Add(time.Hour)})
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	key := base64.RawURLEncoding.EncodeToString(public)
	challenge := func(body any) installation.Proof {
		t.Helper()
		var out installation.Challenge
		f.request(t, ownerToken, false, http.MethodPost, "/api/installations/challenges", body, nil).Want(http.StatusOK).JSON(&out)
		payload, err := base64.RawURLEncoding.DecodeString(out.SignaturePayload)
		if err != nil {
			t.Fatal(err)
		}
		return installation.Proof{ChallengeID: out.ChallengeID, SignaturePayload: out.SignaturePayload, BodyPayload: out.BodyPayload,
			Signature: base64.RawURLEncoding.EncodeToString(ed25519.Sign(private, payload))}
	}
	var enrolled service.InstallationEnrollment
	f.request(t, ownerToken, false, http.MethodPost, "/api/installations/enroll", challenge(map[string]any{
		"purpose": "enroll", "public_key": key, "desktop_version": "0.4.40", "os": "linux",
	}), nil).Want(http.StatusOK).JSON(&enrolled)
	daemonID := uuid.NewString()
	var binding service.InstallationBindingResult
	f.request(t, pat, false, http.MethodPost, "/api/daemon/installation-bindings", challenge(map[string]any{
		"purpose": "bind", "public_key": key, "installation_id": enrolled.InstallationID, "workspace_id": ws,
		"daemon_id": daemonID, "expected_binding_epoch": nil,
	}), nil).Want(http.StatusOK).JSON(&binding)
	runtimeID := f.fx.Runtime(t, "Test-owned Linux provider", testutil.Cols{"owner_id": owner, "daemon_id": daemonID,
		"runtime_mode": "local", "provider": "codex", "status": "online", "last_seen_at": time.Now()})
	agent := f.fx.Agent(t, "Test-owned direct agent", runtimeID, testutil.Cols{"owner_id": owner, "kind": "user", "max_concurrent_tasks": 1})
	issue := f.fx.Issue(t, "Comment to sealed proof", testutil.Cols{"assignee_type": "member", "assignee_id": owner})
	f.fx.InsertNoID(t, "issue_collaboration", testutil.Cols{"issue_id": issue}, "issue_id=$1", issue)
	session := f.fx.Insert(t, "collaboration_session", testutil.Cols{"issue_id": issue})
	for _, user := range []string{owner, contributor} {
		f.fx.InsertNoID(t, "issue_participant", testutil.Cols{"issue_id": issue, "user_id": user}, "issue_id=$1 AND user_id=$2", issue, user)
		f.fx.InsertNoID(t, "collaboration_account_fence", testutil.Cols{"user_id": user}, "user_id=$1", user)
	}
	base := "/api/collaboration/issues/" + issue
	var registered service.CollaborationQueueReceipt
	f.request(t, ownerToken, false, http.MethodPost, base+"/targets", map[string]any{
		"expected_lifecycle_epoch": "1", "expected_participant_epoch": "1", "expected_account_epoch": "1",
		"expected_target_set_version": "1", "kind": "agent", "resource_id": agent,
		"roster": []collaboration.TargetResource{{Role: "agent", AgentID: agent, RuntimeID: runtimeID, OwnerUserID: owner, ExecutionWorkspaceID: ws}},
	}, nil).Want(http.StatusCreated).JSON(&registered)
	if !registered.TargetID.Valid {
		t.Fatal("registration omitted the direct target")
	}
	target := util.UUIDToString(registered.TargetID)
	f.request(t, ownerToken, false, http.MethodPost, base+"/targets/"+target+"/consents", map[string]any{
		"expected_lifecycle_epoch": "1", "expected_participant_epoch": "1", "expected_account_epoch": "1",
		"agent_id": agent, "expected_target_epoch": "1", "expected_config_version": "1",
		"identity": map[string]any{"kind": "managed", "installation_id": enrolled.InstallationID, "binding_id": binding.BindingID, "binding_epoch": binding.BindingEpoch},
	}, nil).Want(http.StatusCreated)
	server := httptest.NewServer(f.router)
	t.Cleanup(server.Close)
	return &collaborationRouteFixture{managed: f, server: server, owner: owner, contributor: contributor,
		ownerToken: ownerToken, contributorToken: platformRouterJWT(t, contributor, nil), issue: issue, session: session, target: target,
		agent: agent, runtime: runtimeID, deployment: auth.ManagedDeploymentID(), binding: binding, installationID: enrolled.InstallationID}
}

func (f *collaborationRouteFixture) comment(t *testing.T, contributor bool, text string) service.CollaborationCommentReceipt {
	t.Helper()
	token := f.ownerToken
	if contributor {
		token = f.contributorToken
	}
	var result service.CollaborationCommentReceipt
	f.managed.request(t, token, false, http.MethodPost, "/api/collaboration/issues/"+f.issue+"/comments", map[string]any{
		"content":                  "[@Direct](mention://agent/" + f.agent + ") " + text,
		"expected_lifecycle_epoch": "1", "expected_participant_epoch": "1", "expected_account_epoch": "1",
		"target_versions": []map[string]string{{"target_id": f.target, "expected_target_epoch": "1", "expected_config_version": "1"}},
	}, nil).Want(http.StatusCreated).JSON(&result)
	if len(result.Targets) != 1 || result.Targets[0].State != "waiting" || !result.Targets[0].CommandID.Valid || !result.Targets[0].BatchID.Valid {
		t.Fatalf("comment did not append its immutable waiting input: code=%s", result.Code)
	}
	return result
}

func (f *collaborationRouteFixture) controlBody(t *testing.T, action, scope string) map[string]any {
	t.Helper()
	q := db.New(f.managed.pool)
	session, err := q.GetCollaborationSession(t.Context(), util.MustParseUUID(f.issue))
	if err != nil {
		t.Fatal(err)
	}
	targets, err := q.ListCollaborationTargets(t.Context(), session.ID)
	if err != nil {
		t.Fatal(err)
	}
	observed := make([]service.CollaborationTargetControl, 0, len(targets))
	for _, target := range targets {
		value := service.CollaborationTargetControl{TargetID: target.ID, ExpectedTargetEpoch: target.TargetEpoch,
			ExpectedConfigVersion: target.ConfigVersion, ExpectedControlVersion: target.ControlVersion,
			ObservedGroupID: target.ActiveGroupID, ObservedGroupGeneration: target.GroupGeneration}
		if action == "resume" {
			value.ExpectedHoldVersion = target.HoldVersion
		}
		observed = append(observed, value)
	}
	body := map[string]any{"action": action, "scope": scope, "expected_lifecycle_epoch": "1",
		"expected_participant_epoch": "1", "expected_account_epoch": "1", "targets": observed}
	if scope == "issue" {
		body["expected_control_version"] = strconv.FormatInt(session.ControlVersion, 10)
		body["expected_target_set_version"] = strconv.FormatInt(session.TargetSetVersion, 10)
	}
	if action == "resume" {
		gate, err := q.GetCollaborationGate(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		body["expected_execution_epoch"] = strconv.FormatInt(gate.Epoch, 10)
	}
	return body
}

func (f *collaborationRouteFixture) control(t *testing.T, key string, body map[string]any, status int) service.CollaborationQueueReceipt {
	t.Helper()
	var result service.CollaborationQueueReceipt
	f.managed.request(t, f.ownerToken, false, http.MethodPost, "/api/collaboration/issues/"+f.issue+"/control", body,
		map[string]string{"Idempotency-Key": key}).Want(status).JSON(&result)
	return result
}

type collaborationProofBarrier struct {
	entered chan struct{}
	release chan struct{}
	seen    sync.Once
	opened  sync.Once
}

func (b *collaborationProofBarrier) RoundTrip(r *http.Request) (*http.Response, error) {
	if strings.HasSuffix(r.URL.Path, "/proof") {
		b.seen.Do(func() { close(b.entered) })
		select {
		case <-b.release:
		case <-r.Context().Done():
			return nil, r.Context().Err()
		}
	}
	return http.DefaultTransport.RoundTrip(r)
}

func (b *collaborationProofBarrier) open() { b.opened.Do(func() { close(b.release) }) }

func collaborationRouteController(t *testing.T, mode, address string) *collaborationrun.NamespaceController {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(executable)
	if err != nil {
		t.Fatal(err)
	}
	fake := filepath.Join(t.TempDir(), "fixture-provider")
	if err = os.WriteFile(fake, raw, 0700); err != nil {
		t.Fatal(err)
	}
	controller := &collaborationrun.NamespaceController{Executable: fake, HelperExecutable: executable,
		Directory: filepath.Join(t.TempDir(), "namespaces"), Arguments: []string{collaborationRouteFakeArg, mode, address}}
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	if err = controller.Probe(ctx); err != nil {
		t.Fatalf("opt-in Linux route test requires kernel namespace support and a static test binary: %v", err)
	}
	return controller
}

func collaborationRouteObservation(t *testing.T, listener net.Listener) (net.Conn, collaborationrun.ProviderInput) {
	t.Helper()
	if err := listener.(*net.TCPListener).SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
		t.Fatal(err)
	}
	conn, err := listener.Accept()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	if err = conn.SetReadDeadline(time.Now().Add(10 * time.Second)); err != nil {
		t.Fatal(err)
	}
	var observed collaborationrun.ProviderInput
	if err = json.NewDecoder(conn).Decode(&observed); err != nil {
		t.Fatal(err)
	}
	return conn, observed
}

func TestCollaborationRuntimeRoutesKeepManagedAndNarrowPurposesSeparate(t *testing.T) {
	f := newCollaborationRouteFixture(t)
	path := "/api/collaboration/runtime/runs"
	for _, action := range []string{"", "/" + uuid.NewString() + "/claim", "/" + uuid.NewString() + "/permit"} {
		method, target := http.MethodPost, path+action
		if action == "" {
			method, target = http.MethodGet, path+"?runtime_id="+f.runtime
		}
		for _, cookie := range []bool{false, true} {
			response := f.managed.request(t, f.ownerToken, cookie, method, target, map[string]any{}, nil).Want(http.StatusUnauthorized)
			platformAssertNoStore(t, response)
		}
	}
	for _, action := range []string{"proof", "outcome", "stop-intent"} {
		method := http.MethodPost
		if action == "stop-intent" {
			method = http.MethodGet
		}
		for _, token := range []string{f.ownerToken, f.binding.DaemonToken} {
			response := f.managed.request(t, token, false, method, path+"/"+uuid.NewString()+"/"+action, map[string]any{}, nil).Want(http.StatusUnauthorized)
			platformAssertNoStore(t, response)
		}
	}
	client, err := collaborationrun.NewClient(f.server.URL, f.binding.DaemonToken, nil)
	if err != nil {
		t.Fatal(err)
	}
	listed, err := client.List(t.Context(), f.runtime)
	if err != nil || listed.RunIDs == nil || len(listed.RunIDs) != 0 {
		t.Fatalf("valid managed list was not usable or exposed another namespace: count=%d err=%v", len(listed.RunIDs), err)
	}
}

func TestCollaborationRuntimeOutcomeClientMatchesService(t *testing.T) {
	for _, outcome := range []string{"success", "failure"} {
		t.Run(outcome, func(t *testing.T) {
			f := newCollaborationRouteFixture(t)
			f.comment(t, false, "retain this run until its sealed proof arrives")
			f.control(t, uuid.NewString(), f.controlBody(t, "resume", "issue"), http.StatusOK)
			client, err := collaborationrun.NewClient(f.server.URL, f.binding.DaemonToken, nil)
			if err != nil {
				t.Fatal(err)
			}
			list, err := client.List(t.Context(), f.runtime)
			if err != nil || len(list.RunIDs) != 1 {
				t.Fatalf("managed list did not promote one run: count=%d err=%v", len(list.RunIDs), err)
			}
			runID := list.RunIDs[0]
			claim, err := client.Claim(t.Context(), runID, protocol.CollaborationRunClaimRequest{
				ProtocolVersion: protocol.CollaborationRuntimeProtocolVersion, DeploymentID: f.deployment,
				InstallationID: f.installationID, RuntimeID: f.runtime, BindingID: f.binding.BindingID,
				BindingEpoch: f.binding.BindingEpoch, Containment: protocol.CollaborationContainmentLinuxPIDNamespace,
			})
			if err != nil {
				t.Fatal(err)
			}
			permit, err := client.Permit(t.Context(), protocol.CollaborationRunPermitRequest{
				Identity: claim.Identity, AttemptSeq: "1", AttemptNonce: uuid.NewString(),
				Containment: protocol.CollaborationContainmentLinuxPIDNamespace,
			})
			if err != nil {
				t.Fatal(err)
			}
			record := protocol.CollaborationRunOutcome{Identity: claim.Identity, EventID: uuid.NewString(),
				AttemptSeq: permit.AttemptSeq, AttemptNonce: permit.AttemptNonce, Outcome: outcome, Body: "explicit provider result"}
			for range 2 {
				if err := client.Outcome(t.Context(), record, permit.BusinessToken); err != nil {
					t.Fatalf("client/service rejected business outcome %q: %v", outcome, err)
				}
			}
			if f.managed.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE id=$1 AND business_outcome=$2 AND settled_at IS NULL`, runID, outcome) != 1 ||
				f.managed.fx.Count(t, `SELECT count(*) FROM collaboration_runtime_reservation WHERE run_id=$1 AND released_at IS NULL`, runID) != 1 {
				t.Fatal("business result was lost or released capacity without a sealed proof")
			}
			if f.managed.fx.Count(t, `SELECT count(*) FROM collaboration_run_event WHERE run_id=$1 AND producer_event_id=$2`, runID, record.EventID) != 1 {
				t.Fatal("same outcome replay created another event")
			}
			var detail struct {
				Execution service.CollaborationExecutionView `json:"execution"`
			}
			f.managed.request(t, f.ownerToken, false, http.MethodGet, "/api/collaboration/issues/"+f.issue, nil, nil).Want(http.StatusOK).JSON(&detail)
			execution := detail.Execution
			if execution.ProtocolVersion != 5 || len(execution.Commands) != 1 || len(execution.Groups) != 1 || len(execution.Runs) != 1 ||
				execution.Commands[0].ChannelID != claim.Identity.ChannelID || execution.Commands[0].BatchID != claim.Identity.BatchID ||
				execution.Groups[0].BatchID != claim.Identity.BatchID || execution.Runs[0].BatchID != claim.Identity.BatchID ||
				execution.Runs[0].BusinessOutcome == nil || *execution.Runs[0].BusinessOutcome != outcome || execution.Runs[0].SettledAt != nil {
				t.Fatal("actual runtime result lost v4 batch identity or fabricated terminal proof in the read projection")
			}
		})
	}
}

// This test never treats a macOS mock, direct SQL proof or ProcessExited flag
// as Linux kernel evidence. Run explicitly in a task-owned Linux container
// with CGO_ENABLED=0 and the namespace privileges needed by the controller.
func TestCollaborationSingleAgentRuntimeRoutesLinux(t *testing.T) {
	if runtime.GOOS != "linux" || os.Getenv("MULTICA_RUN_COLLAB_RUNTIME_E2E") != "1" {
		t.Skip("requires explicit Linux namespace integration environment")
	}
	f := newCollaborationRouteFixture(t)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	controller := collaborationRouteController(t, "block", listener.Addr().String())
	ledger, err := collaborationrun.OpenLedger(filepath.Join(t.TempDir(), "ledger"))
	if err != nil {
		t.Fatal(err)
	}
	barrier := &collaborationProofBarrier{entered: make(chan struct{}), release: make(chan struct{})}
	t.Cleanup(barrier.open)
	client, err := collaborationrun.NewClient(f.server.URL, f.binding.DaemonToken, barrier)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 45*time.Second)
	t.Cleanup(cancel)
	first := f.comment(t, false, "first immutable request")
	f.control(t, uuid.NewString(), f.controlBody(t, "resume", "issue"), http.StatusOK)
	list, err := client.List(ctx, f.runtime)
	if err != nil || len(list.RunIDs) != 1 {
		t.Fatalf("managed list did not promote one run: count=%d err=%v", len(list.RunIDs), err)
	}
	runID := list.RunIDs[0]
	request := protocol.CollaborationRunClaimRequest{ProtocolVersion: protocol.CollaborationRuntimeProtocolVersion,
		DeploymentID: f.deployment, InstallationID: f.installationID, RuntimeID: f.runtime,
		BindingID: f.binding.BindingID, BindingEpoch: f.binding.BindingEpoch, Containment: protocol.CollaborationContainmentLinuxPIDNamespace}
	runner := &collaborationrun.Runner{Ledger: ledger, Client: client, Controller: controller, StopPollInterval: 10 * time.Millisecond}
	finished := make(chan error, 1)
	go func() { finished <- runner.Run(ctx, runID, request); close(finished) }()
	t.Cleanup(func() {
		cancel()
		barrier.open()
		select {
		case <-finished:
		case <-time.After(10 * time.Second):
			t.Error("fixture supervisor did not finish cleanup")
		}
	})
	_, observed := collaborationRouteObservation(t, listener)
	if observed.Identity.RunID != runID || len(observed.Inputs) != 1 || observed.Inputs[0].CommandID != util.UUIDToString(first.Targets[0].CommandID) || observed.Inputs[0].AuthorUserID != f.owner {
		t.Fatal("actual provider did not receive the first immutable author/input")
	}
	second, third := f.comment(t, false, "second immutable request"), f.comment(t, true, "third immutable request")
	if second.Targets[0].BatchID != third.Targets[0].BatchID || second.Targets[0].BatchID == first.Targets[0].BatchID {
		t.Fatal("busy comments did not append to exactly one subsequent batch")
	}
	f.control(t, uuid.NewString(), f.controlBody(t, "stop", "target"), http.StatusOK)
	select {
	case <-barrier.entered:
	case <-ctx.Done():
		t.Fatal("real supervisor did not produce a sealed proof after Stop")
	}
	refusedBody, refusedKey := f.controlBody(t, "resume", "target"), uuid.NewString()
	refused := f.control(t, refusedKey, refusedBody, http.StatusConflict)
	if refused.Code != "stop_unconfirmed" {
		t.Fatalf("resume bypassed missing server proof: code=%s", refused.Code)
	}
	barrier.open()
	if err = <-finished; err != nil {
		t.Fatalf("real supervisor/proof failed: %v", err)
	}
	if f.managed.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE id=$1 AND proof_state='exited_proven' AND settled_at IS NOT NULL`, runID) != 1 {
		t.Fatal("kernel proof did not settle the stopped run")
	}
	if f.control(t, refusedKey, refusedBody, http.StatusConflict).Code != "stop_unconfirmed" {
		t.Fatal("an old refused Resume became a deferred resume")
	}
	f.control(t, uuid.NewString(), f.controlBody(t, "resume", "target"), http.StatusOK)
	list, err = client.List(ctx, f.runtime)
	if err != nil || len(list.RunIDs) != 1 || list.RunIDs[0] == runID {
		t.Fatalf("fresh Resume did not promote one subsequent batch: count=%d err=%v", len(list.RunIDs), err)
	}
	nextRun := list.RunIDs[0]
	runner.Controller = collaborationRouteController(t, "succeed", listener.Addr().String())
	if err = runner.Run(ctx, nextRun, request); err != nil {
		t.Fatalf("next batch supervisor failed: %v", err)
	}
	_, observed = collaborationRouteObservation(t, listener)
	if len(observed.Inputs) != 2 || observed.Inputs[0].CommandID != util.UUIDToString(second.Targets[0].CommandID) ||
		observed.Inputs[1].CommandID != util.UUIDToString(third.Targets[0].CommandID) || observed.Inputs[1].AuthorUserID != f.contributor {
		t.Fatal("the next provider run did not retain both ordered inputs/authors")
	}
	if f.managed.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE batch_id=$1 AND state='succeeded'`, second.Targets[0].BatchID) != 2 {
		t.Fatal("one successful batch did not settle both immutable inputs")
	}
	// Stop can prove an unpermitted run never launched. Its command returns to
	// waiting and may execute only after a fresh Resume, with new run identity.
	last := f.comment(t, false, "resume after server-proven nonlaunch")
	list, err = client.List(ctx, f.runtime)
	if err != nil || len(list.RunIDs) != 1 {
		t.Fatalf("never-launched fixture was not promoted: %v", err)
	}
	q := db.New(f.managed.pool)
	neverRun, err := q.GetCollaborationRuntimeRun(ctx, util.MustParseUUID(list.RunIDs[0]))
	if err != nil {
		t.Fatal(err)
	}
	f.control(t, uuid.NewString(), f.controlBody(t, "stop", "issue"), http.StatusOK)
	returned, err := q.GetCollaborationCommandInput(ctx, db.GetCollaborationCommandInputParams{
		TargetID: last.Targets[0].TargetID, InputEventID: last.EventID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if returned.ID != last.Targets[0].CommandID || returned.State != "waiting" || returned.GroupID.Valid ||
		returned.BatchID == neverRun.BatchID || returned.ChannelID != neverRun.ChannelID || returned.Seq != last.Targets[0].CommandSequence ||
		returned.AuthorUserID != util.MustParseUUID(f.owner) ||
		f.managed.fx.Count(t, `SELECT count(*) FROM collaboration_batch WHERE id=$1 AND state='waiting' AND group_id IS NULL AND started_at IS NULL`, returned.BatchID) != 1 {
		t.Fatal("server-proven nonlaunch did not return the same input to an unstarted batch at its original sequence")
	}
	if f.managed.fx.Count(t, `SELECT count(*) FROM collaboration_group_input i
		JOIN collaboration_execution_group g ON g.id=i.group_id
		JOIN collaboration_run r ON r.id=i.consuming_run_id AND r.group_id=g.id
		WHERE i.command_id=$1 AND i.group_id=$2 AND i.channel_id=$3 AND i.batch_id=$4 AND i.seq=$5
		AND r.id=$6 AND r.authorized_attempt_seq=0 AND r.sealed_attempt_seq=0 AND r.started_at IS NULL
		AND r.progress_state='settled' AND r.proof_state='never_launched_proven' AND r.settled_at IS NOT NULL
		AND g.state='stopped' AND g.expansion_sealed_at IS NOT NULL AND g.settled_at IS NOT NULL`,
		returned.ID, neverRun.GroupID, neverRun.ChannelID, neverRun.BatchID, returned.Seq, neverRun.ID) != 1 {
		t.Fatal("returning input lost its original group, consumed membership or sealed nonlaunch proof")
	}
	retainedHistory := func() string {
		var snapshot string
		f.managed.fx.QueryRow(t, `SELECT jsonb_build_object('run',to_jsonb(r),'group',to_jsonb(g),
			'inputs',(SELECT jsonb_agg(to_jsonb(i) ORDER BY i.seq,i.command_id) FROM collaboration_group_input i WHERE i.group_id=g.id))::text
			FROM collaboration_run r JOIN collaboration_execution_group g ON g.id=r.group_id WHERE r.id=$1`, neverRun.ID).Scan(&snapshot)
		return snapshot
	}
	historyBeforeResume := retainedHistory()
	var detail struct {
		Execution service.CollaborationExecutionView `json:"execution"`
	}
	f.managed.request(t, f.ownerToken, false, http.MethodGet, "/api/collaboration/issues/"+f.issue, nil, nil).Want(http.StatusOK).JSON(&detail)
	if detail.Execution.ProtocolVersion != 5 || len(detail.Execution.Groups) != 3 || len(detail.Execution.Runs) != 3 {
		t.Fatal("returned input made retained execution history unreadable")
	}
	list, err = client.List(ctx, f.runtime)
	if err != nil || len(list.RunIDs) != 0 {
		t.Fatalf("nonlaunch proof bypassed the explicit Resume requirement: count=%d err=%v", len(list.RunIDs), err)
	}
	f.control(t, uuid.NewString(), f.controlBody(t, "resume", "issue"), http.StatusOK)
	list, err = client.List(ctx, f.runtime)
	if err != nil || len(list.RunIDs) != 1 || list.RunIDs[0] == util.UUIDToString(neverRun.ID) {
		t.Fatalf("fresh Resume did not create a new run for the returned input: count=%d err=%v", len(list.RunIDs), err)
	}
	resumedRun := list.RunIDs[0]
	if err = runner.Run(ctx, resumedRun, request); err != nil {
		t.Fatalf("returned input supervisor failed: %v", err)
	}
	_, observed = collaborationRouteObservation(t, listener)
	if observed.Identity.RunID != resumedRun || observed.Identity.GroupID == util.UUIDToString(neverRun.GroupID) ||
		observed.Identity.BatchID != util.UUIDToString(returned.BatchID) || len(observed.Inputs) != 1 ||
		observed.Inputs[0].CommandID != util.UUIDToString(returned.ID) || observed.Inputs[0].AuthorUserID != f.owner {
		t.Fatal("fresh real provider run did not consume the same returned input with a new group identity")
	}
	if f.managed.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=$1 AND state='succeeded' AND seq=$2 AND batch_id=$3 AND group_id=$4`,
		returned.ID, returned.Seq, returned.BatchID, observed.Identity.GroupID) != 1 || retainedHistory() != historyBeforeResume {
		t.Fatal("resumed input failed to settle or rewrote the original nonlaunch history")
	}
}

func TestCollaborationHeldSquadAllowsDirectAgentRoutesLinux(t *testing.T) {
	if runtime.GOOS != "linux" || os.Getenv("MULTICA_RUN_COLLAB_RUNTIME_E2E") != "1" {
		t.Skip("requires explicit Linux namespace integration environment")
	}
	f := newCollaborationRouteFixture(t)
	// An already active channel may gain a fully consented squad target that
	// retains its initial hold. Later direct input must skip that squad input.
	f.control(t, uuid.NewString(), f.controlBody(t, "resume", "issue"), http.StatusOK)
	q := db.New(f.managed.pool)
	session, err := q.GetCollaborationSession(t.Context(), util.MustParseUUID(f.issue))
	if err != nil {
		t.Fatal(err)
	}
	squadID := f.managed.fx.Squad(t, "Held squad sharing the direct leader", f.agent)
	var registered service.CollaborationQueueReceipt
	f.managed.request(t, f.ownerToken, false, http.MethodPost, "/api/collaboration/issues/"+f.issue+"/targets", map[string]any{
		"expected_lifecycle_epoch": "1", "expected_participant_epoch": "1", "expected_account_epoch": "1",
		"expected_target_set_version": strconv.FormatInt(session.TargetSetVersion, 10), "kind": "squad", "resource_id": squadID,
		"roster": []collaboration.TargetResource{{Role: "leader", AgentID: f.agent, RuntimeID: f.runtime, OwnerUserID: f.owner, ExecutionWorkspaceID: f.managed.fx.WorkspaceID}},
	}, nil).Want(http.StatusCreated).JSON(&registered)
	if !registered.TargetID.Valid {
		t.Fatal("squad registration omitted the target identity")
	}
	squad := *f
	squad.target = util.UUIDToString(registered.TargetID)
	consentCollaborationRouteTarget(t, &squad)
	var comment service.CollaborationCommentReceipt
	f.managed.request(t, f.contributorToken, false, http.MethodPost, "/api/collaboration/issues/"+f.issue+"/comments", map[string]any{
		"content":                  "[@Squad](mention://squad/" + squadID + ") keep this squad input held",
		"expected_lifecycle_epoch": "1", "expected_participant_epoch": "1", "expected_account_epoch": "1",
		"target_versions": []map[string]string{{"target_id": squad.target, "expected_target_epoch": "1", "expected_config_version": "1"}},
	}, nil).Want(http.StatusCreated).JSON(&comment)
	if len(comment.Targets) != 1 {
		t.Fatalf("squad mention lost its per-target receipt: code=%s targets=%d", comment.Code, len(comment.Targets))
	}
	held := comment.Targets[0]
	holdReason := "missing"
	if held.HoldReason != nil {
		holdReason = string(*held.HoldReason)
	}
	if held.State != "waiting" || held.Code != "command_queued" || !held.CommandID.Valid ||
		holdReason != string(collaboration.HoldInitial) {
		t.Fatalf("squad mention lost its waiting input or initial hold: state=%s code=%s command_valid=%t hold=%s", held.State, held.Code, held.CommandID.Valid, holdReason)
	}
	direct := f.comment(t, false, "only this direct input may reach the provider")
	if held.ChannelID != direct.Targets[0].ChannelID || held.CommandSequence >= direct.Targets[0].CommandSequence {
		t.Fatal("same-leader targets did not share the ordered channel")
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	controller := collaborationRouteController(t, "succeed", listener.Addr().String())
	ledger, err := collaborationrun.OpenLedger(filepath.Join(t.TempDir(), "ledger"))
	if err != nil {
		t.Fatal(err)
	}
	client, err := collaborationrun.NewClient(f.server.URL, f.binding.DaemonToken, nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	list, err := client.List(ctx, f.runtime)
	if err != nil || len(list.RunIDs) != 1 {
		t.Fatalf("held squad blocked the direct agent: count=%d err=%v", len(list.RunIDs), err)
	}
	request := protocol.CollaborationRunClaimRequest{ProtocolVersion: protocol.CollaborationRuntimeProtocolVersion,
		DeploymentID: f.deployment, InstallationID: f.installationID, RuntimeID: f.runtime,
		BindingID: f.binding.BindingID, BindingEpoch: f.binding.BindingEpoch, Containment: protocol.CollaborationContainmentLinuxPIDNamespace}
	runner := &collaborationrun.Runner{Ledger: ledger, Client: client, Controller: controller, StopPollInterval: 10 * time.Millisecond}
	if err := runner.Run(ctx, list.RunIDs[0], request); err != nil {
		t.Fatalf("direct input supervisor failed behind the held squad: %v", err)
	}
	_, observed := collaborationRouteObservation(t, listener)
	if observed.Identity.TargetID != f.target || observed.Identity.RunID != list.RunIDs[0] || len(observed.Inputs) != 1 ||
		observed.Inputs[0].CommandID != util.UUIDToString(direct.Targets[0].CommandID) || observed.Inputs[0].AuthorUserID != f.owner {
		t.Fatal("held squad input reached the real direct provider")
	}
	if f.managed.fx.Count(t, `SELECT count(*) FROM collaboration_command c JOIN collaboration_target t ON t.id=c.target_id
		WHERE c.id=$1 AND c.target_id=$2 AND c.state='waiting' AND c.group_id IS NULL AND c.seq=$3
		AND t.hold_reason='initial' AND t.queue_state='active'`, held.CommandID, registered.TargetID, held.CommandSequence) != 1 ||
		f.managed.fx.Count(t, `SELECT count(*) FROM collaboration_group_input WHERE command_id=$1`, held.CommandID) != 0 ||
		f.managed.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=$1 AND state='succeeded'`, direct.Targets[0].CommandID) != 1 ||
		f.managed.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE session_id=$1`, f.session) != 1 {
		t.Fatal("direct execution consumed held input, released its hold or created another run")
	}
	list, err = client.List(ctx, f.runtime)
	if err != nil || len(list.RunIDs) != 0 {
		t.Fatalf("direct completion released the squad hold: count=%d err=%v", len(list.RunIDs), err)
	}
	var detail struct {
		Execution service.CollaborationExecutionView `json:"execution"`
	}
	f.managed.request(t, f.ownerToken, false, http.MethodGet, "/api/collaboration/issues/"+f.issue, nil, nil).Want(http.StatusOK).JSON(&detail)
	if detail.Execution.ProtocolVersion != 5 || len(detail.Execution.Channels) != 1 || len(detail.Execution.Targets) != 2 ||
		len(detail.Execution.Commands) != 2 || len(detail.Execution.Groups) != 1 || len(detail.Execution.Groups[0].Inputs) != 1 ||
		detail.Execution.Groups[0].Inputs[0].CommandID != util.UUIDToString(direct.Targets[0].CommandID) {
		t.Fatal("held and consumed input history lost its shared channel projection")
	}
}

func TestCollaborationRuntimeProofSurvivesOwnerDisableLinux(t *testing.T) {
	if runtime.GOOS != "linux" || os.Getenv("MULTICA_RUN_COLLAB_RUNTIME_E2E") != "1" {
		t.Skip("requires explicit Linux namespace integration environment")
	}
	f := newCollaborationRouteFixture(t)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	controller := collaborationRouteController(t, "block", listener.Addr().String())
	ledger, err := collaborationrun.OpenLedger(filepath.Join(t.TempDir(), "ledger"))
	if err != nil {
		t.Fatal(err)
	}
	barrier := &collaborationProofBarrier{entered: make(chan struct{}), release: make(chan struct{})}
	t.Cleanup(barrier.open)
	client, err := collaborationrun.NewClient(f.server.URL, f.binding.DaemonToken, barrier)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	t.Cleanup(cancel)
	f.comment(t, false, "prove stopped process after source disable")
	f.control(t, uuid.NewString(), f.controlBody(t, "resume", "issue"), http.StatusOK)
	list, err := client.List(ctx, f.runtime)
	if err != nil || len(list.RunIDs) != 1 {
		t.Fatalf("fixture did not promote one run: %v", err)
	}
	runID := list.RunIDs[0]
	request := protocol.CollaborationRunClaimRequest{ProtocolVersion: protocol.CollaborationRuntimeProtocolVersion,
		DeploymentID: f.deployment, InstallationID: f.installationID, RuntimeID: f.runtime,
		BindingID: f.binding.BindingID, BindingEpoch: f.binding.BindingEpoch, Containment: protocol.CollaborationContainmentLinuxPIDNamespace}
	runner := &collaborationrun.Runner{Ledger: ledger, Client: client, Controller: controller, StopPollInterval: 10 * time.Millisecond}
	finished := make(chan error, 1)
	go func() { finished <- runner.Run(ctx, runID, request); close(finished) }()
	t.Cleanup(func() {
		cancel()
		barrier.open()
		select {
		case <-finished:
		case <-time.After(10 * time.Second):
			t.Error("revoked fixture supervisor did not finish cleanup")
		}
	})
	_, observed := collaborationRouteObservation(t, listener)
	if observed.Identity.RunID != runID {
		t.Fatal("fixture process has wrong run identity")
	}
	f.control(t, uuid.NewString(), f.controlBody(t, "stop", "target"), http.StatusOK)
	select {
	case <-barrier.entered:
	case <-ctx.Done():
		t.Fatal("supervisor did not seal actual process exit")
	}
	// A current source-account denial must affect managed business traffic,
	// while the already issued supervisor capability remains able to settle.
	f.managed.fx.Exec(t, `UPDATE "user" SET disabled_at=now() WHERE id=$1`, f.owner)
	_, err = client.List(ctx, f.runtime)
	var denied *collaborationrun.HTTPError
	if !errors.As(err, &denied) || (denied.Status != http.StatusUnauthorized && denied.Status != http.StatusForbidden) {
		t.Fatalf("disabled owner retained managed runtime access: %v", err)
	}
	barrier.open()
	if err = <-finished; err != nil {
		t.Fatalf("old narrow proof was stranded by source disable: %v", err)
	}
	if f.managed.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE id=$1 AND proof_state='exited_proven' AND settled_at IS NOT NULL`, runID) != 1 {
		t.Fatal("disabled owner's real sealed proof did not settle the run")
	}
}
