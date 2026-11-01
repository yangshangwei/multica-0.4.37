//go:build agentintegration && !windows

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/multica-ai/multica/server/internal/analytics"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/daemon/collaborationrun"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/handler"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

const rk1DatabasePrefix = "multica_rk1_"

// rk1Fixture is one trial: a fresh router schema, issue, Claude host runtime
// and trace. Seeding uses the production service; runs use the real Runner.
type rk1Fixture struct {
	t                  *testing.T
	env                *rk1Env
	base               *collaborationRouteFixture
	h                  *handler.Handler
	svc                *service.CollaborationService
	q                  *db.Queries
	owner, contributor context.Context
	issue, session     string
	trace              *rk1Trace
	host               *collaborationrun.HostConfig
	request            protocol.CollaborationRunClaimRequest
	client             *collaborationrun.Client
	dir, plans         string
	sim                string // rk1-sim-claude in plans, only in a sim env
	targetSet          int64
	targets            map[string]string
}

func rk1RequireDatabase(t *testing.T) {
	t.Helper()
	cfg, err := pgxpool.ParseConfig(strings.TrimSpace(os.Getenv("DATABASE_URL")))
	if err != nil || !strings.HasPrefix(cfg.ConnConfig.Database, rk1DatabasePrefix) {
		t.Skip("RK1 runs only against its own migrated database named " + rk1DatabasePrefix + "*")
	}
	var database string
	if testPool == nil || testPool.QueryRow(t.Context(), "SELECT current_database()").Scan(&database) != nil || database != cfg.ConnConfig.Database {
		t.Fatal("RK1 database identity mismatch")
	}
	if os.Getenv("MULTICA_KNOWLEDGE_CONTEXT_ENABLED") != "" {
		t.Fatal("RK1 runtime scenarios run with knowledge off; unset MULTICA_KNOWLEDGE_CONTEXT_ENABLED")
	}
}

func newRK1Fixture(t *testing.T, env *rk1Env, dir string) *rk1Fixture {
	t.Helper()
	b := newCollaborationRouteFixture(t)
	b.managed.fx.Exec(t, `CREATE TABLE project (LIKE public.project INCLUDING ALL)`)
	rk1CloneTriggers(t, b.managed)
	b.managed.fx.Exec(t, `UPDATE workspace SET issue_counter=(SELECT COALESCE(max(number),0) FROM issue WHERE workspace_id=workspace.id) WHERE id=$1`, b.managed.fx.WorkspaceID)
	router, h := NewRouterWithOptions(b.managed.pool, nil, events.New(), analytics.NoopClient{}, nil, RouterOptions{})
	h.TaskService.KnowledgeContextEnabled = false
	f := &rk1Fixture{t: t, env: env, base: b, h: h, q: db.New(b.managed.pool), trace: &rk1Trace{next: router}, dir: dir,
		plans: filepath.Join(dir, "provider"), targetSet: 1, targets: map[string]string{}}
	b.managed.router, b.server.Config.Handler = router, f.trace
	if err := os.MkdirAll(f.plans, 0700); err != nil {
		t.Fatal(err)
	}
	if env.sim {
		f.sim = rk1LinkSim(t, f.plans)
	}
	f.svc = service.NewCollaborationService(f.q, b.managed.pool, h.IssueService)
	f.svc.KnowledgeContextEnabled = false
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
	project := b.managed.fx.Project(t, "RK1 project", testutil.Cols{"description": "RK1 runtime acceptance"})
	created, err := f.svc.Create(f.owner, service.CollaborationCreateParams{SourceWorkspaceID: util.MustParseUUID(b.managed.fx.WorkspaceID),
		HandlerID: util.MustParseUUID(b.contributor), ProjectID: util.MustParseUUID(project), RequestID: uuid.NewString(), Title: "RK1 runtime acceptance issue"})
	if err != nil {
		t.Fatal(err)
	}
	f.issue, f.session = util.UUIDToString(created.Issue.ID), util.UUIDToString(created.SessionID)
	old, err := f.q.GetAgentRuntime(t.Context(), util.MustParseUUID(b.runtime))
	if err != nil {
		t.Fatal(err)
	}
	b.runtime = b.managed.fx.Runtime(t, "RK1 Claude host", testutil.Cols{"owner_id": b.owner, "provider": "claude", "runtime_mode": "local", "daemon_id": old.DaemonID.String, "status": "online"})
	f.host = &collaborationrun.HostConfig{Provider: "claude", ProviderVersion: env.version, ProviderMode: "local", AccountSource: "rk1-operator-runtime", Executable: env.wrapper,
		Limits: protocol.CollaborationPreparationLimits{FileCount: 32, PerFileBytes: 100 << 20, TotalInputBytes: 256 << 20, WorkspaceBytes: 512 << 20, GitExpandedBytes: 256 << 20, EntryCount: 4096, TimeoutSeconds: 60}}
	caps, err := f.host.Capabilities()
	if err != nil {
		t.Fatal(err)
	}
	row, err := f.q.GetAgentRuntime(t.Context(), util.MustParseUUID(b.runtime))
	if err != nil {
		t.Fatal(err)
	}
	b.managed.request(t, b.binding.DaemonToken, false, "POST", "/api/collaboration/runtime/capabilities", protocol.CollaborationRuntimeCapabilitiesRequest{ProtocolVersion: 3,
		DeploymentID: b.deployment, InstallationID: b.installationID, RuntimeID: b.runtime, BindingID: b.binding.BindingID, BindingEpoch: b.binding.BindingEpoch,
		ExpectedRuntimeConfigEpoch: strconv.FormatInt(row.CollaborationConfigEpoch, 10), Capabilities: caps}, nil).Want(200)
	digest, err := protocol.CollaborationRuntimeCapabilityDigest(caps)
	if err != nil {
		t.Fatal(err)
	}
	f.request = protocol.CollaborationRunClaimRequest{ProtocolVersion: 3, DeploymentID: b.deployment, InstallationID: b.installationID, RuntimeID: b.runtime,
		BindingID: b.binding.BindingID, BindingEpoch: b.binding.BindingEpoch, Containment: protocol.CollaborationContainmentHostProcessSupervision, PreparationVersion: 2, CapabilityDigest: digest}
	if f.client, err = collaborationrun.NewClient(b.server.URL, b.binding.DaemonToken, nil); err != nil {
		t.Fatal(err)
	}
	t.Setenv(rk1PlanDirEnv, f.plans)
	return f
}

// rk1LinkSim puts this test binary into the trial's plan directory as
// rk1-sim-claude, where it reads its plans. A hard link avoids copying the
// binary; a copy is the fallback across filesystems. It is not evidence, so it
// is removed when the trial ends.
func rk1LinkSim(t *testing.T, plans string) string {
	t.Helper()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	sim := filepath.Join(plans, rk1SimClaudeName)
	if err = os.Link(self, sim); err != nil {
		raw, readErr := os.ReadFile(self)
		if readErr != nil {
			t.Fatal(readErr)
		}
		if err = os.WriteFile(sim, raw, 0700); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() { _ = os.Remove(sim) })
	return sim
}

func (f *rk1Fixture) agent(name string) string {
	cols := testutil.Cols{"owner_id": f.base.owner, "permission_mode": "private", "instructions": "Follow the admitted collaboration assignment."}
	if f.env.model != "" {
		cols["model"] = f.env.model
	}
	return f.base.managed.fx.Agent(f.t, "RK1 "+name+" "+uuid.NewString()[:8], f.base.runtime, cols)
}

func (f *rk1Fixture) queue() service.CollaborationQueueRequest {
	return service.CollaborationQueueRequest{ExecutionViewVersion: 6, IssueID: util.MustParseUUID(f.issue), RequestID: uuid.NewString(),
		ExpectedLifecycleEpoch: 1, ExpectedParticipantEpoch: 1, ExpectedAccountEpoch: 1}
}

func (f *rk1Fixture) resource(role, agent string) collaboration.TargetResource {
	return collaboration.TargetResource{Role: role, AgentID: agent, RuntimeID: f.base.runtime, OwnerUserID: f.base.owner, ExecutionWorkspaceID: f.base.managed.fx.WorkspaceID}
}

func (f *rk1Fixture) register(label string, kind collaboration.TargetKind, resource string, roster []collaboration.TargetResource) string {
	t := f.t
	t.Helper()
	target, err := f.svc.RegisterTarget(f.owner, service.CollaborationRegisterTargetParams{CollaborationQueueRequest: f.queue(), Kind: kind,
		ResourceID: util.MustParseUUID(resource), ExpectedTargetSetVersion: f.targetSet, Roster: roster})
	if err != nil || !target.TargetID.Valid {
		t.Fatalf("register %s target: %v %+v", label, err, target)
	}
	f.targetSet++
	bindingEpoch, err := strconv.ParseInt(f.base.binding.BindingEpoch, 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range roster {
		if _, err = f.svc.ConsentTargetResource(f.owner, service.CollaborationConsentParams{CollaborationQueueRequest: f.queue(), TargetID: target.TargetID,
			AgentID: util.MustParseUUID(r.AgentID), ExpectedTargetEpoch: 1, ExpectedConfigVersion: 1, Identity: service.CollaborationResourceIdentity{Kind: "managed",
				InstallationID: util.MustParseUUID(f.base.installationID), BindingID: util.MustParseUUID(f.base.binding.BindingID), BindingEpoch: bindingEpoch}}); err != nil {
			t.Fatalf("consent %s target: %v", label, err)
		}
	}
	f.targets[label] = util.UUIDToString(target.TargetID)
	return f.targets[label]
}

func (f *rk1Fixture) agentTarget(label string) (target, agent string) {
	agent = f.agent(label)
	return f.register(label, collaboration.TargetAgent, agent, []collaboration.TargetResource{f.resource("agent", agent)}), agent
}

func (f *rk1Fixture) squadTarget(label string) (target, leader, member string) {
	leader, member = f.agent(label+" leader"), f.agent(label+" member")
	squad := f.base.managed.fx.Squad(f.t, "RK1 squad "+uuid.NewString()[:8], leader)
	f.base.managed.fx.SquadMember(f.t, squad, "agent", member)
	return f.register(label, collaboration.TargetSquad, squad, []collaboration.TargetResource{f.resource("leader", leader), f.resource("member", member)}), leader, member
}

func (f *rk1Fixture) submit(actor context.Context, label, body string) {
	f.t.Helper()
	if _, err := f.svc.SubmitCommand(actor, service.CollaborationSubmitCommandParams{CollaborationQueueRequest: f.queue(), TargetID: util.MustParseUUID(f.targets[label]),
		ExpectedTargetEpoch: 1, ExpectedConfigVersion: 1, InputEventID: util.MustParseUUID(uuid.NewString()), Content: body}); err != nil {
		f.t.Fatalf("submit to %s: %v", label, err)
	}
}

func (f *rk1Fixture) comment(author string, at int, body string) string {
	return f.base.managed.fx.Comment(f.t, f.issue, body, testutil.Cols{"author_id": author, "created_at": rk1CommentTime(at)})
}

func rk1Label(scenario string, trial int, hop string) string {
	label := fmt.Sprintf("%s-t%d", scenario, trial)
	if hop != "" {
		label += "-" + hop
	}
	return label
}
