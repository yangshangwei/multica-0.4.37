//go:build agentintegration && !windows

package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/daemon/collaborationrun"
	"github.com/multica-ai/multica/server/internal/installation"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

type ac43Lane struct {
	name, runtime, installation, daemon, agent, target string
	config                                             *collaborationrun.HostConfig
	binding                                            service.InstallationBindingResult
	request                                            protocol.CollaborationRunClaimRequest
	client                                             *collaborationrun.Client
}

type ac43Fixture struct {
	raw                     *rk1Fixture
	creator, handler, owner string
	revision                int64
	lanes                   []*ac43Lane
}

func ac43Actor(id string) context.Context {
	return auth.WithPasswordSession(auth.WithAuthenticatedPrincipal(context.Background(), auth.AuthenticatedPrincipal{UserID: id, Kind: auth.CredentialJWT}), auth.PasswordSession{UserID: id, Version: 1, Kind: "jwt"})
}

func newAC43Fixture(t *testing.T, dir, cli string, configs []*collaborationrun.HostConfig) *ac43Fixture {
	// Reuse the real router/schema/trigger setup; none of RK1's experiment runner,
	// wrapper, budgets or result scoring participates in this acceptance.
	raw := newRK1Fixture(t, &rk1Env{version: configs[0].ProviderVersion, wrapper: configs[0].Executable, cli: cli}, dir)
	b := raw.base
	creator := b.managed.user(t, "")
	b.managed.fx.Member(t, b.managed.fx.WorkspaceID, creator, "member")
	created, err := raw.svc.Create(ac43Actor(creator), service.CollaborationCreateParams{SourceWorkspaceID: util.MustParseUUID(b.managed.fx.WorkspaceID), HandlerID: util.MustParseUUID(b.contributor), RequestID: uuid.NewString(), Title: "AC43 real provider provenance"})
	if err != nil {
		t.Fatal(err)
	}
	raw.issue, raw.session = util.UUIDToString(created.Issue.ID), util.UUIDToString(created.SessionID)
	raw.owner, raw.contributor = ac43Actor(creator), ac43Actor(b.contributor)
	f := &ac43Fixture{raw: raw, creator: creator, handler: b.contributor, owner: b.owner, revision: 1}
	invited, err := raw.svc.InviteParticipant(ac43Actor(creator), service.CollaborationParticipantParams{CollaborationWorkflowRequest: f.workflow(creator), TargetUserID: util.MustParseUUID(b.owner)})
	if err != nil || invited.Code != "participant_added" {
		t.Fatalf("invite resource owner: %v code=%s", err, invited.Code)
	}
	f.revision = invited.CollaborationRevision
	for i, config := range configs {
		lane := f.enroll(fmt.Sprintf("environment-%d", i+1), config)
		lane.agent = b.managed.fx.Agent(t, "AC43 "+lane.name, lane.runtime, testutil.Cols{"owner_id": f.owner, "permission_mode": "private", "model": config.DefaultModel, "instructions": "Follow only the supplied AC43 assignment. Do not inspect credentials or accounts."})
		f.lanes = append(f.lanes, lane)
	}
	for _, lane := range f.lanes {
		lane.target = f.register(collaboration.TargetAgent, lane.agent, []*ac43Lane{lane})
	}
	if f.creator == f.handler || f.creator == f.owner || f.handler == f.owner || f.lanes[0].runtime == f.lanes[1].runtime || f.lanes[0].installation == f.lanes[1].installation {
		t.Fatal("AC43 identities are not distinct")
	}
	return f
}

func (f *ac43Fixture) queue(actor string) service.CollaborationQueueRequest {
	t, b := f.raw.t, f.raw.base
	p := service.CollaborationQueueRequest{ExecutionViewVersion: 6, IssueID: util.MustParseUUID(f.raw.issue), RequestID: uuid.NewString()}
	b.managed.fx.QueryRow(t, `SELECT c.lifecycle_epoch,p.epoch,a.epoch FROM issue_collaboration c JOIN issue_participant p ON p.issue_id=c.issue_id JOIN collaboration_account_fence a ON a.user_id=p.user_id WHERE c.issue_id=$1 AND p.user_id=$2 AND p.revoked_at IS NULL`, f.raw.issue, actor).Scan(&p.ExpectedLifecycleEpoch, &p.ExpectedParticipantEpoch, &p.ExpectedAccountEpoch)
	return p
}
func (f *ac43Fixture) workflow(actor string) service.CollaborationWorkflowRequest {
	f.raw.base.managed.fx.QueryRow(f.raw.t, `SELECT revision FROM issue_collaboration WHERE issue_id=$1`, f.raw.issue).Scan(&f.revision)
	return service.CollaborationWorkflowRequest{CollaborationQueueRequest: f.queue(actor), ExpectedCollaborationRevision: f.revision}
}

func (f *ac43Fixture) enroll(name string, config *collaborationrun.HostConfig) *ac43Lane {
	t, b := f.raw.t, f.raw.base
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	key := base64.RawURLEncoding.EncodeToString(public)
	challenge := func(body any) installation.Proof {
		var out installation.Challenge
		b.managed.request(t, b.ownerToken, false, http.MethodPost, "/api/installations/challenges", body, nil).Want(200).JSON(&out)
		payload, err := base64.RawURLEncoding.DecodeString(out.SignaturePayload)
		if err != nil {
			t.Fatal(err)
		}
		return installation.Proof{ChallengeID: out.ChallengeID, SignaturePayload: out.SignaturePayload, BodyPayload: out.BodyPayload, Signature: base64.RawURLEncoding.EncodeToString(ed25519.Sign(private, payload))}
	}
	osName := runtime.GOOS
	if osName == "darwin" {
		osName = "macos"
	}
	var enrollment service.InstallationEnrollment
	b.managed.request(t, b.ownerToken, false, "POST", "/api/installations/enroll", challenge(map[string]any{"purpose": "enroll", "public_key": key, "desktop_version": "0.6.7", "os": osName}), nil).Want(200).JSON(&enrollment)
	pat, err := auth.GeneratePATToken()
	if err != nil {
		t.Fatal(err)
	}
	b.managed.fx.Insert(t, "personal_access_token", testutil.Cols{"user_id": f.owner, "name": "AC43 test-owned binding", "token_hash": auth.HashToken(pat), "token_prefix": "mul_", "auth_version": 1, "expires_at": time.Now().Add(time.Hour)})
	lane := &ac43Lane{name: name, config: config, installation: enrollment.InstallationID, daemon: uuid.NewString()}
	b.managed.request(t, pat, false, "POST", "/api/daemon/installation-bindings", challenge(map[string]any{"purpose": "bind", "public_key": key, "installation_id": lane.installation, "workspace_id": b.managed.fx.WorkspaceID, "daemon_id": lane.daemon, "expected_binding_epoch": nil}), nil).Want(200).JSON(&lane.binding)
	lane.runtime = b.managed.fx.Runtime(t, "AC43 "+name, testutil.Cols{"owner_id": f.owner, "provider": config.Provider, "runtime_mode": "local", "daemon_id": lane.daemon, "status": "online", "last_seen_at": time.Now()})
	caps, err := config.Capabilities()
	if err != nil {
		t.Fatal(err)
	}
	row, err := f.raw.q.GetAgentRuntime(t.Context(), util.MustParseUUID(lane.runtime))
	if err != nil {
		t.Fatal(err)
	}
	b.managed.request(t, lane.binding.DaemonToken, false, "POST", "/api/collaboration/runtime/capabilities", protocol.CollaborationRuntimeCapabilitiesRequest{ProtocolVersion: 3, DeploymentID: b.deployment, InstallationID: lane.installation, RuntimeID: lane.runtime, BindingID: lane.binding.BindingID, BindingEpoch: lane.binding.BindingEpoch, ExpectedRuntimeConfigEpoch: strconv.FormatInt(row.CollaborationConfigEpoch, 10), Capabilities: caps}, nil).Want(200)
	digest, err := protocol.CollaborationRuntimeCapabilityDigest(caps)
	if err != nil {
		t.Fatal(err)
	}
	lane.request = protocol.CollaborationRunClaimRequest{ProtocolVersion: 3, DeploymentID: b.deployment, InstallationID: lane.installation, RuntimeID: lane.runtime, BindingID: lane.binding.BindingID, BindingEpoch: lane.binding.BindingEpoch, Containment: protocol.CollaborationContainmentHostProcessSupervision, PreparationVersion: 2, CapabilityDigest: digest}
	lane.client, err = collaborationrun.NewClient(b.server.URL, lane.binding.DaemonToken, nil)
	if err != nil {
		t.Fatal(err)
	}
	return lane
}

func (f *ac43Fixture) register(kind collaboration.TargetKind, resource string, lanes []*ac43Lane) string {
	t := f.raw.t
	session, err := f.raw.q.GetCollaborationSession(t.Context(), util.MustParseUUID(f.raw.issue))
	if err != nil {
		t.Fatal(err)
	}
	roster := make([]collaboration.TargetResource, len(lanes))
	for i, lane := range lanes {
		role := "agent"
		if kind == collaboration.TargetSquad {
			role = "member"
			if i == 0 {
				role = "leader"
			}
		}
		roster[i] = collaboration.TargetResource{Role: role, AgentID: lane.agent, RuntimeID: lane.runtime, OwnerUserID: f.owner, ExecutionWorkspaceID: f.raw.base.managed.fx.WorkspaceID}
	}
	target, err := f.raw.svc.RegisterTarget(ac43Actor(f.creator), service.CollaborationRegisterTargetParams{CollaborationQueueRequest: f.queue(f.creator), Kind: kind, ResourceID: util.MustParseUUID(resource), ExpectedTargetSetVersion: session.TargetSetVersion, Roster: roster})
	if err != nil || !target.TargetID.Valid || target.Status >= 400 {
		t.Fatalf("target registration: %v code=%s", err, target.Code)
	}
	for _, lane := range lanes {
		epoch, err := strconv.ParseInt(lane.binding.BindingEpoch, 10, 64)
		if err != nil {
			t.Fatal(err)
		}
		receipt, err := f.raw.svc.ConsentTargetResource(ac43Actor(f.owner), service.CollaborationConsentParams{CollaborationQueueRequest: f.queue(f.owner), TargetID: target.TargetID, AgentID: util.MustParseUUID(lane.agent), ExpectedTargetEpoch: 1, ExpectedConfigVersion: 1, Identity: service.CollaborationResourceIdentity{Kind: "managed", InstallationID: util.MustParseUUID(lane.installation), BindingID: util.MustParseUUID(lane.binding.BindingID), BindingEpoch: epoch}})
		if err != nil || receipt.Status >= 400 {
			t.Fatalf("owner consent: %v code=%s", err, receipt.Code)
		}
	}
	return util.UUIDToString(target.TargetID)
}

func (f *ac43Fixture) submit(actor, target, body string) {
	receipt, err := f.raw.svc.SubmitCommand(ac43Actor(actor), service.CollaborationSubmitCommandParams{CollaborationQueueRequest: f.queue(actor), TargetID: util.MustParseUUID(target), ExpectedTargetEpoch: 1, ExpectedConfigVersion: 1, InputEventID: util.MustParseUUID(uuid.NewString()), Content: body})
	if err != nil || receipt.Status >= 400 {
		f.raw.t.Fatalf("submit: %v code=%s", err, receipt.Code)
	}
}

func (f *ac43Fixture) next(lane *ac43Lane) string {
	t := f.raw.t
	// Refresh only these two positive fixtures, never unrelated stale runtimes.
	for _, item := range f.lanes {
		f.raw.base.managed.fx.Exec(t, `UPDATE agent_runtime SET last_seen_at=now(),status='online' WHERE id=$1`, item.runtime)
	}
	for deadline := time.Now().Add(15 * time.Second); ; time.Sleep(100 * time.Millisecond) {
		pending, err := lane.client.List(t.Context(), lane.runtime)
		if err != nil {
			t.Fatal(err)
		}
		var selected []string
		for _, run := range pending.RunIDs {
			row, err := f.raw.q.GetCollaborationRuntimeRun(t.Context(), util.MustParseUUID(run))
			if err != nil {
				t.Fatal(err)
			}
			if util.UUIDToString(row.SessionID) == f.raw.session && row.ProgressState != "settled" {
				selected = append(selected, run)
			}
		}
		if len(selected) == 1 {
			return selected[0]
		}
		if len(selected) > 1 || time.Now().After(deadline) {
			t.Fatalf("%s expected one pending AC43 run, got %d", lane.name, len(selected))
		}
	}
}

// Remove test-only connection and launcher variables from the real child.
// Pool connections have already been created; no configuration file is edited.
func ac43WithoutTestEnvironment() func() {
	saved := map[string]string{}
	for _, entry := range os.Environ() {
		key, value, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		if key == "DATABASE_URL" || key == "CLAUDECODE" || strings.HasPrefix(key, "PG") || strings.HasPrefix(key, "AC43_") || strings.HasPrefix(key, "RK1_") || key == "MULTICA_RUN_REAL_AGENT_SMOKE" {
			saved[key] = value
			_ = os.Unsetenv(key)
		}
	}
	return func() {
		for key, value := range saved {
			_ = os.Setenv(key, value)
		}
	}
}
