package handler

import (
	"context"
	"net/http"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type configHTTPFixture struct {
	issue, runtime, agent, session string
	owned                          []string
	svc                            *service.CollaborationService
	ctx                            context.Context
}

func newConfigHTTPFixture(t *testing.T) *configHTTPFixture {
	t.Helper()
	if testHandler == nil || testPool == nil {
		t.Skip("requires task-owned DATABASE_URL")
	}
	t.Setenv("MULTICA_AUTH_MODE", "legacy")
	f := &configHTTPFixture{}
	// Actual fixture cleanup deletes resources and emits tombstones. This
	// final cleanup is scoped to these synthetic resource IDs only.
	t.Cleanup(func() {
		if _, err := testPool.Exec(context.Background(), `DELETE FROM collaboration_config_invalidation WHERE resource_id=ANY($1::uuid[])`, f.owned); err != nil {
			t.Errorf("cleanup HTTP configuration intents: %v", err)
		}
	})
	f.issue = collaborationEditFixture(t)
	f.runtime = dbfx.Runtime(t, "HTTP config runtime "+uuid.NewString())
	f.agent = dbfx.Agent(t, "HTTP config agent "+uuid.NewString(), f.runtime)
	f.owned = []string{f.runtime, f.agent}
	f.svc = service.NewCollaborationService(testHandler.Queries, testHandler.TxStarter, testHandler.IssueService)
	f.ctx = auth.WithAuthenticatedPrincipal(t.Context(), auth.AuthenticatedPrincipal{UserID: testUserID, Kind: auth.CredentialJWT})
	session, err := testHandler.Queries.GetCollaborationSession(f.ctx, parseUUID(f.issue))
	if err != nil {
		t.Fatal(err)
	}
	f.session = uuidToString(session.ID)
	dbfx.Cleanup(t, `DELETE FROM collaboration_channel WHERE session_id=$1`, f.session)
	dbfx.Cleanup(t, `DELETE FROM collaboration_target WHERE session_id=$1`, f.session)
	return f
}

func (f *configHTTPFixture) register(t *testing.T, kind collaboration.TargetKind, resource string) db.CollaborationTarget {
	t.Helper()
	session, err := testHandler.Queries.GetCollaborationSession(f.ctx, parseUUID(f.issue))
	if err != nil {
		t.Fatal(err)
	}
	role := "agent"
	if kind == collaboration.TargetSquad {
		role = "leader"
	}
	r, err := f.svc.RegisterTarget(f.ctx, service.CollaborationRegisterTargetParams{
		CollaborationQueueRequest: service.CollaborationQueueRequest{IssueID: parseUUID(f.issue), RequestID: uuid.NewString(), ExpectedLifecycleEpoch: 1, ExpectedParticipantEpoch: 1, ExpectedAccountEpoch: 1},
		Kind:                      kind, ResourceID: parseUUID(resource), ExpectedTargetSetVersion: session.TargetSetVersion,
		Roster: []collaboration.TargetResource{{Role: role, AgentID: f.agent, RuntimeID: f.runtime, OwnerUserID: testUserID, ExecutionWorkspaceID: testWorkspaceID}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return f.target(t, r.TargetID)
}

func (f *configHTTPFixture) target(t *testing.T, id pgtype.UUID) db.CollaborationTarget {
	t.Helper()
	targets, err := testHandler.Queries.ListCollaborationTargets(f.ctx, parseUUID(f.session))
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range targets {
		if target.ID == id {
			return target
		}
	}
	t.Fatal("registered HTTP configuration target missing")
	return db.CollaborationTarget{}
}

func (f *configHTTPFixture) drain(t *testing.T) {
	t.Helper()
	worker := service.NewCollaborationConfigReconciler(testHandler.Queries, testHandler.TxStarter)
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	// Other handler tests also emit durable intents. Exercise the real fair
	// consumer, but wait only for these resources instead of global emptiness.
	for {
		var pending int
		err := testPool.QueryRow(ctx, `SELECT count(*) FROM collaboration_config_invalidation
			WHERE resource_id=ANY($1::uuid[]) AND reconciled_at IS NULL`, f.owned).Scan(&pending)
		if err != nil {
			t.Fatal(err)
		}
		if pending == 0 {
			return
		}
		processed, err := worker.Reconcile(ctx, 128)
		if err != nil {
			t.Fatal(err)
		}
		if processed == 0 {
			t.Fatalf("HTTP writer still has %d pending configuration intents without consumer progress", pending)
		}
	}
}

func TestCollaborationConfigurationAgentHTTPDisplayEchoPreservesAuthority(t *testing.T) {
	f := newConfigHTTPFixture(t)
	permission := map[string]any{"permission_mode": "public_to", "invocation_targets": []map[string]string{{"target_type": "workspace", "target_id": testWorkspaceID}}}
	testutil.Call(t, testHandler.UpdateAgent, withURLParam(newRequest(http.MethodPut, "/api/agents/"+f.agent, permission), "id", f.agent)).Want(http.StatusOK)
	target := f.register(t, collaboration.TargetAgent, f.agent)
	f.drain(t)
	before, err := testHandler.Queries.GetAgent(f.ctx, parseUUID(f.agent))
	if err != nil {
		t.Fatal(err)
	}
	beforeTargets, err := testHandler.Queries.ListAgentInvocationTargets(f.ctx, before.ID)
	if err != nil {
		t.Fatal(err)
	}
	intents := dbfx.Count(t, `SELECT count(*) FROM collaboration_config_invalidation WHERE resource_id=$1`, f.agent)
	permission["name"] = "Renamed HTTP configuration agent " + uuid.NewString()
	result := testutil.Call(t, testHandler.UpdateAgent, withURLParam(newRequest(http.MethodPut, "/api/agents/"+f.agent, permission), "id", f.agent)).Want(http.StatusOK).Map()
	if result["name"] != permission["name"] {
		t.Fatal("ordinary display update did not succeed")
	}
	after, err := testHandler.Queries.GetAgent(f.ctx, before.ID)
	if err != nil {
		t.Fatal(err)
	}
	afterTargets, err := testHandler.Queries.ListAgentInvocationTargets(f.ctx, before.ID)
	if err != nil {
		t.Fatal(err)
	}
	f.drain(t)
	if before.CollaborationConfigEpoch != after.CollaborationConfigEpoch || !reflect.DeepEqual(beforeTargets, afterTargets) ||
		!reflect.DeepEqual(target, f.target(t, target.ID)) || dbfx.Count(t, `SELECT count(*) FROM collaboration_config_invalidation WHERE resource_id=$1`, f.agent) != intents {
		t.Fatal("a display rename with identical permission echo revoked or rotated consent identity")
	}
	testutil.Call(t, testHandler.UpdateAgent, withURLParam(newRequest(http.MethodPut, "/api/agents/"+f.agent, map[string]any{"instructions": "changed execution policy"}), "id", f.agent)).Want(http.StatusOK)
	f.drain(t)
	if current := f.target(t, target.ID); current.HoldReason.String != "config_invalid" || current.ControlVersion != target.ControlVersion {
		t.Fatalf("real agent update failed to hold the stale target independently: %+v", current)
	}
}

func TestCollaborationConfigurationAgentHTTPUpdateIsAtomic(t *testing.T) {
	for _, failure := range []string{"clear", "commit"} {
		t.Run(failure, func(t *testing.T) {
			f := newConfigHTTPFixture(t)
			dbfx.Exec(t, `UPDATE agent SET mcp_config='{"configured":true}' WHERE id=$1`, f.agent)
			f.register(t, collaboration.TargetAgent, f.agent)
			f.drain(t)
			var before, after string
			dbfx.QueryRow(t, `SELECT to_jsonb(a)::text FROM agent a WHERE id=$1`, f.agent).Scan(&before)
			intents := dbfx.Count(t, `SELECT count(*) FROM collaboration_config_invalidation WHERE resource_id=$1`, f.agent)
			copyHandler := *testHandler
			fault := lifecycleFaultStarter{base: testHandler.TxStarter, commit: failure == "commit"}
			if failure == "clear" {
				fault.query = "ClearAgentMcpConfig"
			}
			copyHandler.TxStarter = fault
			body := map[string]any{"name": "must roll back " + uuid.NewString(), "mcp_config": nil, "instructions": "must roll back",
				"permission_mode": "public_to", "invocation_targets": []map[string]string{{"target_type": "workspace", "target_id": testWorkspaceID}}}
			testutil.Call(t, copyHandler.UpdateAgent, withURLParam(newRequest(http.MethodPut, "/api/agents/"+f.agent, body), "id", f.agent)).Want(http.StatusInternalServerError)
			dbfx.QueryRow(t, `SELECT to_jsonb(a)::text FROM agent a WHERE id=$1`, f.agent).Scan(&after)
			if before != after || dbfx.Count(t, `SELECT count(*) FROM collaboration_config_invalidation WHERE resource_id=$1`, f.agent) != intents ||
				dbfx.Count(t, `SELECT count(*) FROM agent_invocation_target WHERE agent_id=$1`, f.agent) != 0 {
				t.Fatal("failed HTTP update committed row, nullable clear, invoke membership or invalidation separately")
			}
		})
	}
}

func TestCollaborationConfigurationSquadHTTPMembershipPersistsInvalidation(t *testing.T) {
	f := newConfigHTTPFixture(t)
	squad := dbfx.Squad(t, "HTTP configuration squad "+uuid.NewString(), f.agent)
	f.owned = append(f.owned, squad)
	target := f.register(t, collaboration.TargetSquad, squad)
	request := func(method string, body any) *http.Request {
		return withURLParams(newRequest(method, "/api/squads/"+squad, body), "workspaceId", testWorkspaceID, "id", squad)
	}
	testutil.Call(t, testHandler.UpdateSquad, request(http.MethodPatch, map[string]any{"name": "renamed squad " + uuid.NewString()})).Want(http.StatusOK)
	if dbfx.Count(t, `SELECT count(*) FROM collaboration_config_invalidation WHERE resource_id=$1`, squad) != 0 {
		t.Fatal("squad display name invalidated its execution configuration")
	}
	member := dbfx.Agent(t, "HTTP new member "+uuid.NewString(), f.runtime)
	f.owned = append(f.owned, member)
	dbfx.Cleanup(t, `DELETE FROM squad_member WHERE squad_id=$1`, squad)
	testutil.Call(t, testHandler.AddSquadMember, request(http.MethodPost, map[string]any{"member_type": "agent", "member_id": member, "role": "developer"})).Want(http.StatusCreated)
	f.drain(t)
	if dbfx.Count(t, `SELECT count(*) FROM squad_member WHERE squad_id=$1 AND member_id=$2`, squad, member) != 1 || f.target(t, target.ID).HoldReason.String != "config_invalid" {
		t.Fatal("real membership HTTP writer did not preserve its ordinary row and hold the stale collaboration target")
	}
}

func TestCollaborationConfigurationSkillHTTPEnabledChangePersistsInvalidation(t *testing.T) {
	f := newConfigHTTPFixture(t)
	skill := dbfx.Insert(t, "skill", testutil.Cols{"workspace_id": testWorkspaceID, "name": "HTTP configuration skill " + uuid.NewString(), "content": "test fixture", "created_by": testUserID})
	if err := testHandler.Queries.AddAgentSkill(f.ctx, db.AddAgentSkillParams{AgentID: parseUUID(f.agent), SkillID: parseUUID(skill)}); err != nil {
		t.Fatal(err)
	}
	dbfx.Cleanup(t, `DELETE FROM agent_skill WHERE agent_id=$1`, f.agent)
	target := f.register(t, collaboration.TargetAgent, f.agent)
	f.drain(t)
	request := withURLParams(newRequest(http.MethodPatch, "/api/agents/"+f.agent+"/skills/"+skill, map[string]bool{"enabled": false}), "id", f.agent, "skillId", skill)
	testutil.Call(t, testHandler.SetAgentSkillEnabled, request).Want(http.StatusOK)
	f.drain(t)
	if dbfx.Count(t, `SELECT count(*) FROM agent_skill WHERE agent_id=$1 AND skill_id=$2 AND enabled=false`, f.agent, skill) != 1 || f.target(t, target.ID).HoldReason.String != "config_invalid" {
		t.Fatal("skill HTTP writer did not persist disabled state and collaboration invalidation")
	}
}
