package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type executionSquadTestProject struct {
	ProjectResponse
	ExecutionSquad struct {
		State       string `json:"state"`
		TemplateKey string `json:"template_key"`
		SquadID     string `json:"squad_id"`
		RuntimeID   string `json:"runtime_id"`
		ErrorCode   string `json:"error_code"`
	} `json:"execution_squad"`
	Resources []ProjectResourceResponse `json:"resources"`
}

func projectExecutionFixture(t *testing.T) *testutil.Fixture {
	t.Helper()
	workspaceID := dbfx.Workspace(t, "Project squad tests", fmt.Sprintf("project-squad-%d", time.Now().UnixNano()))
	dbfx.Member(t, workspaceID, testUserID, "owner")
	f := testutil.New(testPool, workspaceID, testUserID)
	f.Cleanup(t, `DELETE FROM skill WHERE workspace_id = $1`, workspaceID)
	f.Cleanup(t, `DELETE FROM agent WHERE workspace_id = $1`, workspaceID)
	f.Cleanup(t, `DELETE FROM squad WHERE workspace_id = $1`, workspaceID)
	f.Cleanup(t, `DELETE FROM project WHERE workspace_id = $1`, workspaceID)
	return f
}

func projectExecutionRequest(f *testutil.Fixture, method, projectID string, body any) *http.Request {
	path := "/api/projects"
	if projectID != "" {
		path += "/" + projectID + "/execution-squad"
	}
	req := testutil.WithHeaders(testutil.JSONRequest(method, path, body),
		"X-User-ID", f.UserID, "X-Workspace-ID", f.WorkspaceID)
	return testutil.WithURLParams(req, "id", projectID, "workspaceId", f.WorkspaceID)
}

func TestProjectExecutionSquad_CreateRetainsTemplateWithoutRuntime(t *testing.T) {
	f := projectExecutionFixture(t)
	var out executionSquadTestProject
	testutil.Call(t, testHandler.CreateProject, projectExecutionRequest(f, "POST", "", map[string]any{
		"title": "Pending squad", "execution_squad": map[string]any{"template_key": "feature-delivery"},
		"resources": []map[string]any{{"resource_type": "github_repo", "resource_ref": map[string]any{"url": "https://github.com/example/project"}}},
	})).Want(http.StatusCreated).JSON(&out)
	if out.ExecutionSquad.State != "needs_runtime" || out.ExecutionSquad.TemplateKey != "feature-delivery" {
		t.Fatalf("execution squad = %+v; want retained feature-delivery selection needing a runtime", out.ExecutionSquad)
	}
	if out.ResourceCount != 1 || len(out.Resources) != 1 {
		t.Fatalf("bundled create lost resources: count=%d resources=%v", out.ResourceCount, out.Resources)
	}
	if count := f.Count(t, `SELECT count(*) FROM squad WHERE workspace_id = $1`, f.WorkspaceID); count != 0 {
		t.Fatalf("template without a runtime created %d squads", count)
	}
	var stored executionSquadTestProject
	testutil.Call(t, testHandler.GetProject, projectExecutionRequest(f, "GET", out.ID, nil)).Want(http.StatusOK).JSON(&stored)
	if stored.ExecutionSquad != out.ExecutionSquad {
		t.Fatalf("selection did not survive a project read: got %+v, want %+v", stored.ExecutionSquad, out.ExecutionSquad)
	}
}

func TestProjectExecutionSquad_CreateLegacyAndNone(t *testing.T) {
	f := projectExecutionFixture(t)
	for _, body := range []map[string]any{
		{"title": "Legacy project"},
		{"title": "No default", "execution_squad": map[string]any{}},
	} {
		var out executionSquadTestProject
		testutil.Call(t, testHandler.CreateProject, projectExecutionRequest(f, "POST", "", body)).Want(http.StatusCreated).JSON(&out)
		if out.ExecutionSquad.State != "none" || out.ExecutionSquad.SquadID != "" {
			t.Fatalf("no-selection project = %+v, want state none", out.ExecutionSquad)
		}
	}
}

func TestProjectExecutionSquad_CreateUsesExistingSquad(t *testing.T) {
	f := projectExecutionFixture(t)
	runtimeID := f.Runtime(t, "Existing squad runtime")
	leaderID := f.Agent(t, "Existing leader", runtimeID)
	squadID := f.Squad(t, "Existing squad", leaderID)
	f.SquadMember(t, squadID, "agent", leaderID, testutil.Cols{"role": "leader"})
	var out executionSquadTestProject
	testutil.Call(t, testHandler.CreateProject, projectExecutionRequest(f, "POST", "", map[string]any{
		"title": "Existing default", "execution_squad": map[string]any{"squad_id": squadID},
	})).Want(http.StatusCreated).JSON(&out)
	if out.ExecutionSquad.State != "configured" || out.ExecutionSquad.SquadID != squadID {
		t.Fatalf("execution squad = %+v, want existing squad %s", out.ExecutionSquad, squadID)
	}
	if count := f.Count(t, `SELECT count(*) FROM squad WHERE workspace_id = $1`, f.WorkspaceID); count != 1 {
		t.Fatalf("existing selection duplicated its squad: count=%d", count)
	}
}

func TestProjectExecutionSquad_CreateRejectsInvalidSelection(t *testing.T) {
	f := projectExecutionFixture(t)
	for name, selection := range map[string]any{
		"two choices":       map[string]any{"template_key": "feature-delivery", "squad_id": "00000000-0000-0000-0000-000000000001"},
		"unknown template":  map[string]any{"template_key": "missing-template"},
		"bad runtime UUID":  map[string]any{"template_key": "feature-delivery", "runtime_id": "bad"},
		"runtime only":      map[string]any{"runtime_id": "00000000-0000-0000-0000-000000000001"},
		"bad squad UUID":    map[string]any{"squad_id": "bad"},
		"invalid language":  map[string]any{"template_key": "feature-delivery", "language": "invalid"},
		"invalid structure": []string{"feature-delivery"},
	} {
		t.Run(name, func(t *testing.T) {
			testutil.Call(t, testHandler.CreateProject, projectExecutionRequest(f, "POST", "", map[string]any{
				"title": "Rejected default", "execution_squad": selection,
			})).Want(http.StatusBadRequest)
		})
	}
	if count := f.Count(t, `SELECT count(*) FROM project WHERE workspace_id = $1`, f.WorkspaceID); count != 0 {
		t.Fatalf("invalid choices left %d projects behind", count)
	}
}

func TestProjectExecutionSquad_CreatePreparationFailureRetainsProject(t *testing.T) {
	f := projectExecutionFixture(t)
	runtimeID := f.Runtime(t, "Preparation runtime")
	f.Agent(t, "Implementer", runtimeID)
	var out executionSquadTestProject
	testutil.Call(t, testHandler.CreateProject, projectExecutionRequest(f, "POST", "", map[string]any{
		"title": "Retryable project", "execution_squad": map[string]any{"template_key": "feature-delivery", "runtime_id": runtimeID},
	})).Want(http.StatusCreated).JSON(&out)
	if out.ID == "" || out.ExecutionSquad.State != "failed" || out.ExecutionSquad.ErrorCode != "agent_name_conflict" {
		t.Fatalf("preparation failure = %+v, want a saved project with safe agent_name_conflict", out)
	}
	if out.ExecutionSquad.TemplateKey != "feature-delivery" || out.ExecutionSquad.RuntimeID != runtimeID {
		t.Fatalf("failed preparation lost retry input: %+v", out.ExecutionSquad)
	}
	if count := f.Count(t, `SELECT count(*) FROM agent WHERE workspace_id = $1`, f.WorkspaceID); count != 1 {
		t.Fatalf("failed preparation left partial role agents: %d", count)
	}
	if count := f.Count(t, `SELECT count(*) FROM skill WHERE workspace_id = $1`, f.WorkspaceID); count != 0 {
		t.Fatalf("failed preparation left partial role skills: %d", count)
	}
	if count := f.Count(t, `SELECT count(*) FROM squad WHERE workspace_id = $1`, f.WorkspaceID); count != 0 {
		t.Fatalf("failed preparation left partial squads: %d", count)
	}
}

func TestProjectExecutionSquad_PutRetryReuseAndClear(t *testing.T) {
	f := projectExecutionFixture(t)
	runtimeID := f.Runtime(t, "Retry runtime")
	projectID := f.Project(t, "Retry project")
	selection := map[string]any{"template_key": "feature-delivery", "runtime_id": runtimeID}
	var first executionSquadTestProject
	testutil.Call(t, testHandler.ConfigureProjectSquad, projectExecutionRequest(f, "PUT", projectID, selection)).Want(http.StatusOK).JSON(&first)
	if first.ExecutionSquad.State != "configured" || first.ExecutionSquad.SquadID == "" {
		t.Fatalf("first configuration = %+v", first.ExecutionSquad)
	}
	var updatedAt string
	f.QueryRow(t, `SELECT updated_at::text FROM project WHERE id = $1`, projectID).Scan(&updatedAt)
	var again executionSquadTestProject
	testutil.Call(t, testHandler.ConfigureProjectSquad, projectExecutionRequest(f, "PUT", projectID, selection)).Want(http.StatusOK).JSON(&again)
	if again.ExecutionSquad != first.ExecutionSquad {
		t.Fatalf("identical retry changed selection: first=%+v second=%+v", first.ExecutionSquad, again.ExecutionSquad)
	}
	var retryUpdatedAt string
	f.QueryRow(t, `SELECT updated_at::text FROM project WHERE id = $1`, projectID).Scan(&retryUpdatedAt)
	if retryUpdatedAt != updatedAt {
		t.Fatalf("idempotent retry updated the project: %s -> %s", updatedAt, retryUpdatedAt)
	}
	f.Exec(t, `UPDATE agent SET instructions = 'custom role instructions' WHERE workspace_id = $1 AND template_key = 'implementer'`, f.WorkspaceID)
	f.Exec(t, `UPDATE skill SET content = 'custom role skill' WHERE workspace_id = $1`, f.WorkspaceID)
	secondProjectID := f.Project(t, "Shared squad project")
	var reused executionSquadTestProject
	testutil.Call(t, testHandler.ConfigureProjectSquad, projectExecutionRequest(f, "PUT", secondProjectID, selection)).Want(http.StatusOK).JSON(&reused)
	if reused.ExecutionSquad.SquadID != first.ExecutionSquad.SquadID {
		t.Fatalf("second project did not reuse the invocable template squad: %+v", reused.ExecutionSquad)
	}
	var instructions string
	f.QueryRow(t, `SELECT instructions FROM agent WHERE workspace_id = $1 AND template_key = 'implementer'`, f.WorkspaceID).Scan(&instructions)
	if instructions != "custom role instructions" {
		t.Fatalf("reused role customization was overwritten: %q", instructions)
	}
	if count := f.Count(t, `SELECT count(*) FROM skill WHERE workspace_id = $1 AND content <> 'custom role skill'`, f.WorkspaceID); count != 0 {
		t.Fatalf("%d customized skills were overwritten", count)
	}
	var cleared executionSquadTestProject
	testutil.Call(t, testHandler.ConfigureProjectSquad, projectExecutionRequest(f, "PUT", projectID, map[string]any{})).Want(http.StatusOK).JSON(&cleared)
	if cleared.ExecutionSquad.State != "none" || cleared.ExecutionSquad.SquadID != "" {
		t.Fatalf("clear = %+v", cleared.ExecutionSquad)
	}
	if count := f.Count(t, `SELECT count(*) FROM squad WHERE workspace_id = $1`, f.WorkspaceID); count != 1 {
		t.Fatalf("clear removed a shared squad or retry duplicated it: count=%d", count)
	}
}

func TestProjectExecutionSquad_RetryAfterConflict(t *testing.T) {
	f := projectExecutionFixture(t)
	runtimeID := f.Runtime(t, "Recoverable runtime")
	conflictingID := f.Agent(t, "Implementer", runtimeID)
	projectID := f.Project(t, "Recoverable project")
	selection := map[string]any{"template_key": "feature-delivery", "runtime_id": runtimeID}
	var failed executionSquadTestProject
	testutil.Call(t, testHandler.ConfigureProjectSquad, projectExecutionRequest(f, "PUT", projectID, selection)).Want(http.StatusOK).JSON(&failed)
	if failed.ExecutionSquad.ErrorCode != "agent_name_conflict" {
		t.Fatalf("conflict = %+v", failed.ExecutionSquad)
	}
	f.Exec(t, `UPDATE agent SET name = 'Custom implementer' WHERE id = $1`, conflictingID)
	var retried executionSquadTestProject
	testutil.Call(t, testHandler.ConfigureProjectSquad, projectExecutionRequest(f, "PUT", projectID, selection)).Want(http.StatusOK).JSON(&retried)
	if retried.ExecutionSquad.State != "configured" || retried.ExecutionSquad.ErrorCode != "" {
		t.Fatalf("retry did not recover after conflict resolution: %+v", retried.ExecutionSquad)
	}
	if count := f.Count(t, `SELECT count(*) FROM project WHERE workspace_id = $1`, f.WorkspaceID); count != 1 {
		t.Fatalf("retry duplicated the project: count=%d", count)
	}
}

func TestProjectExecutionSquad_ConcurrentProjectsShareOneSquad(t *testing.T) {
	f := projectExecutionFixture(t)
	runtimeID := f.Runtime(t, "Concurrent runtime")
	ids := []string{f.Project(t, "First concurrent project"), f.Project(t, "Second concurrent project")}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	responses := make(chan *testutil.Response, len(ids))
	for _, id := range ids {
		go func(projectID string) {
			req := projectExecutionRequest(f, "PUT", projectID, map[string]any{"template_key": "feature-delivery", "runtime_id": runtimeID}).WithContext(ctx)
			req = testutil.WithURLParams(req, "id", projectID, "workspaceId", f.WorkspaceID)
			responses <- testutil.Call(t, testHandler.ConfigureProjectSquad, req)
		}(id)
	}
	var squadID string
	for range ids {
		var out executionSquadTestProject
		(<-responses).Want(http.StatusOK).JSON(&out)
		if out.ExecutionSquad.State != "configured" {
			t.Fatalf("concurrent preparation = %+v", out.ExecutionSquad)
		}
		if squadID != "" && squadID != out.ExecutionSquad.SquadID {
			t.Fatalf("concurrent projects created different squads: %s / %s", squadID, out.ExecutionSquad.SquadID)
		}
		squadID = out.ExecutionSquad.SquadID
	}
	if count := f.Count(t, `SELECT count(*) FROM squad WHERE workspace_id = $1`, f.WorkspaceID); count != 1 {
		t.Fatalf("concurrent materialization produced %d squads", count)
	}
}

func TestProjectExecutionSquad_PrivateAgentCannotBeAdoptedByAdmin(t *testing.T) {
	f := projectExecutionFixture(t)
	otherID := f.User(t, "Private agent owner", uuid.NewString()+"@example.test")
	f.Member(t, f.WorkspaceID, otherID, "member")
	runtimeID := f.Runtime(t, "Admin runtime")
	template, _ := service.SquadTemplateByKey("feature-delivery")
	leaderID := f.Agent(t, "Private feature lead", runtimeID, testutil.Cols{"owner_id": otherID, "template_key": template.LeaderTemplateKey})
	squadID := f.Squad(t, "Private leader squad", leaderID, testutil.Cols{"template_key": template.Key, "creator_id": otherID})
	projectID := f.Project(t, "No admin invocation bypass")
	testutil.Call(t, testHandler.ConfigureProjectSquad, projectExecutionRequest(f, "PUT", projectID, map[string]any{"squad_id": squadID})).Want(http.StatusForbidden)
	var out executionSquadTestProject
	testutil.Call(t, testHandler.ConfigureProjectSquad, projectExecutionRequest(f, "PUT", projectID, map[string]any{
		"template_key": "feature-delivery", "runtime_id": runtimeID,
	})).Want(http.StatusOK).JSON(&out)
	if out.ExecutionSquad.State != "failed" || out.ExecutionSquad.ErrorCode != "agent_access_denied" {
		t.Fatalf("admin adopted a private role: %+v", out.ExecutionSquad)
	}
	if count := f.Count(t, `SELECT count(*) FROM agent WHERE workspace_id = $1`, f.WorkspaceID); count != 1 {
		t.Fatalf("private-role refusal left partial agents: count=%d", count)
	}
}

func TestProjectExecutionSquad_RuntimeMismatchNeverRebindsReusedRole(t *testing.T) {
	f := projectExecutionFixture(t)
	runtimeID := f.Runtime(t, "Requested runtime")
	actualRuntimeID := f.Runtime(t, "Customized role runtime")
	roleID := f.Agent(t, "Customized implementer", actualRuntimeID, testutil.Cols{"template_key": "implementer", "instructions": "keep my role"})
	var out executionSquadTestProject
	testutil.Call(t, testHandler.CreateProject, projectExecutionRequest(f, "POST", "", map[string]any{
		"title": "Runtime mismatch", "execution_squad": map[string]any{"template_key": "feature-delivery", "runtime_id": runtimeID},
	})).Want(http.StatusCreated).JSON(&out)
	if out.ExecutionSquad.State != "failed" || out.ExecutionSquad.ErrorCode != "runtime_mismatch" {
		t.Fatalf("mismatched reused role = %+v", out.ExecutionSquad)
	}
	var actual, instructions string
	f.QueryRow(t, `SELECT runtime_id::text, instructions FROM agent WHERE id = $1`, roleID).Scan(&actual, &instructions)
	if actual != actualRuntimeID || instructions != "keep my role" {
		t.Fatalf("reused role changed: runtime=%s instructions=%q", actual, instructions)
	}
	if count := f.Count(t, `SELECT count(*) FROM agent WHERE workspace_id = $1`, f.WorkspaceID); count != 1 {
		t.Fatalf("mismatch left partial agents: count=%d", count)
	}
}

func TestProjectExecutionSquad_ExistingSquadNeedsDirectoriesOnActualMachines(t *testing.T) {
	f := projectExecutionFixture(t)
	firstDaemon, secondDaemon := uuid.NewString(), uuid.NewString()
	firstRuntime := f.Runtime(t, "First machine", testutil.Cols{"daemon_id": firstDaemon})
	secondRuntime := f.Runtime(t, "Second machine", testutil.Cols{"daemon_id": secondDaemon})
	leader := f.Agent(t, "First machine leader", firstRuntime)
	worker := f.Agent(t, "Second machine worker", secondRuntime)
	squadID := f.Squad(t, "Two machine squad", leader)
	f.SquadMember(t, squadID, "agent", leader, testutil.Cols{"role": "leader"})
	f.SquadMember(t, squadID, "agent", worker)
	projectID := f.Project(t, "Per-machine directories")
	addDirectory := func(daemon string) {
		f.Insert(t, "project_resource", testutil.Cols{
			"project_id": projectID, "workspace_id": f.WorkspaceID, "resource_type": "local_directory",
			"resource_ref": fmt.Sprintf(`{"daemon_id":%q,"local_path":"/projects/test"}`, daemon), "created_by": f.UserID,
		})
	}
	addDirectory(firstDaemon)
	var missing executionSquadTestProject
	testutil.Call(t, testHandler.ConfigureProjectSquad, projectExecutionRequest(f, "PUT", projectID, map[string]any{"squad_id": squadID})).Want(http.StatusOK).JSON(&missing)
	if missing.ExecutionSquad.ErrorCode != "runtime_mismatch" {
		t.Fatalf("worker machine was ignored: %+v", missing.ExecutionSquad)
	}
	addDirectory(secondDaemon)
	var configured executionSquadTestProject
	testutil.Call(t, testHandler.ConfigureProjectSquad, projectExecutionRequest(f, "PUT", projectID, map[string]any{"squad_id": squadID})).Want(http.StatusOK).JSON(&configured)
	if configured.ExecutionSquad.State != "configured" || configured.ResourceCount != 2 {
		t.Fatalf("per-daemon resources did not permit recovery: %+v", configured)
	}
}

func TestProjectExecutionSquad_ForeignAndPrivateRuntimeBoundaries(t *testing.T) {
	f := projectExecutionFixture(t)
	other := projectExecutionFixture(t)
	foreignRuntime := other.Runtime(t, "Foreign runtime")
	foreignLeader := other.Agent(t, "Foreign leader", foreignRuntime)
	foreignSquad := other.Squad(t, "Foreign squad", foreignLeader)
	otherUser := f.User(t, "Private runtime owner", uuid.NewString()+"@example.test")
	privateRuntime := f.Runtime(t, "Private runtime", testutil.Cols{"owner_id": otherUser})
	for _, tc := range []struct {
		name      string
		selection map[string]any
		status    int
	}{
		{"foreign runtime", map[string]any{"template_key": "feature-delivery", "runtime_id": foreignRuntime}, http.StatusBadRequest},
		{"private runtime", map[string]any{"template_key": "feature-delivery", "runtime_id": privateRuntime}, http.StatusForbidden},
		{"foreign squad", map[string]any{"squad_id": foreignSquad}, http.StatusBadRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			testutil.Call(t, testHandler.CreateProject, projectExecutionRequest(f, "POST", "", map[string]any{"title": tc.name, "execution_squad": tc.selection})).Want(tc.status)
		})
	}
	if count := f.Count(t, `SELECT count(*) FROM project WHERE workspace_id = $1`, f.WorkspaceID); count != 0 {
		t.Fatalf("rejected selections left %d projects", count)
	}
	foreignProject := other.Project(t, "Foreign project")
	testutil.Call(t, testHandler.ConfigureProjectSquad, projectExecutionRequest(f, "PUT", foreignProject, map[string]any{})).Want(http.StatusNotFound)
}

func TestProjectExecutionSquad_AgentAutonomyGatesBothEntryPoints(t *testing.T) {
	f := projectExecutionFixture(t)
	runtimeID := f.Runtime(t, "Autonomy runtime")
	projectID := f.Project(t, "Autonomy project")
	for _, tc := range []struct{ level, template string }{
		{"observer", "feature-delivery"},
		{"contributor", "feature-delivery"},
		{"coordinator", "release"},
	} {
		t.Run(tc.level, func(t *testing.T) {
			actorID := f.Agent(t, "Acting "+tc.level, runtimeID, testutil.Cols{"autonomy_level": tc.level})
			selection := map[string]any{"template_key": tc.template, "runtime_id": runtimeID}
			post := projectExecutionRequest(f, "POST", "", map[string]any{"title": "Denied machine setup", "execution_squad": selection})
			put := projectExecutionRequest(f, "PUT", projectID, selection)
			for _, entry := range []struct {
				req     *http.Request
				handler http.HandlerFunc
			}{{post, testHandler.CreateProject}, {put, testHandler.ConfigureProjectSquad}} {
				testutil.WithHeaders(entry.req, "X-Actor-Source", "task_token", "X-Agent-ID", actorID)
				testutil.Call(t, entry.handler, entry.req).Want(http.StatusForbidden)
			}
		})
	}
}

type pauseProjectSquadBegin struct {
	inner   txStarter
	reached chan<- struct{}
	release <-chan struct{}
}

func (s pauseProjectSquadBegin) Begin(ctx context.Context) (pgx.Tx, error) {
	s.reached <- struct{}{}
	select {
	case <-s.release:
		return s.inner.Begin(ctx)
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func TestProjectExecutionSquad_StaleCreatePreparationCannotOverwriteNewChoice(t *testing.T) {
	f := projectExecutionFixture(t)
	runtimeID := f.Runtime(t, "Delayed preparation")
	f.Agent(t, "Implementer", runtimeID)
	reached, release := make(chan struct{}, 1), make(chan struct{})
	h := *testHandler
	h.TxStarter = pauseProjectSquadBegin{inner: testPool, reached: reached, release: release}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	done := make(chan *testutil.Response, 1)
	go func() {
		done <- testutil.Call(t, h.CreateProject, projectExecutionRequest(f, "POST", "", map[string]any{
			"title": "Delayed project", "execution_squad": map[string]any{"template_key": "feature-delivery", "runtime_id": runtimeID},
		}).WithContext(ctx))
	}()
	select {
	case <-reached:
	case <-ctx.Done():
		t.Fatal("create did not reach the preparation gap")
	}
	var projectID string
	f.QueryRow(t, `SELECT id::text FROM project WHERE workspace_id = $1`, f.WorkspaceID).Scan(&projectID)
	testutil.Call(t, testHandler.ConfigureProjectSquad, projectExecutionRequest(f, "PUT", projectID, map[string]any{})).Want(http.StatusOK)
	close(release)
	var out executionSquadTestProject
	(<-done).Want(http.StatusCreated).JSON(&out)
	if out.ExecutionSquad.State != "none" {
		t.Fatalf("stale failure overwrote newer clear: %+v", out.ExecutionSquad)
	}
	if count := f.Count(t, `SELECT count(*) FROM squad WHERE workspace_id = $1`, f.WorkspaceID); count != 0 {
		t.Fatalf("stale preparation created %d orphan squads", count)
	}
}

func TestProjectExecutionSquad_DeleteWinsBeforePreparation(t *testing.T) {
	f := projectExecutionFixture(t)
	runtimeID := f.Runtime(t, "Delete-race runtime")
	reached, release := make(chan struct{}, 1), make(chan struct{})
	h := *testHandler
	h.TxStarter = pauseProjectSquadBegin{inner: testPool, reached: reached, release: release}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	done := make(chan *testutil.Response, 1)
	go func() {
		done <- testutil.Call(t, h.CreateProject, projectExecutionRequest(f, "POST", "", map[string]any{
			"title": "Deleted during setup", "execution_squad": map[string]any{"template_key": "feature-delivery", "runtime_id": runtimeID},
		}).WithContext(ctx))
	}()
	select {
	case <-reached:
	case <-ctx.Done():
		t.Fatal("create did not reach the preparation gap")
	}
	var projectID string
	f.QueryRow(t, `SELECT id::text FROM project WHERE workspace_id = $1`, f.WorkspaceID).Scan(&projectID)
	testutil.Call(t, testHandler.DeleteProject, projectExecutionRequest(f, "DELETE", projectID, nil)).Want(http.StatusNoContent)
	close(release)
	(<-done).Want(http.StatusCreated)
	if count := f.Count(t, `SELECT count(*) FROM project WHERE workspace_id = $1`, f.WorkspaceID); count != 0 {
		t.Fatalf("preparation resurrected a deleted project: count=%d", count)
	}
	if count := f.Count(t, `SELECT count(*) FROM squad WHERE workspace_id = $1`, f.WorkspaceID); count != 0 {
		t.Fatalf("deleted project left %d orphan squads", count)
	}
}

type failProjectSquadLinkBegin struct{ inner txStarter }

func (s failProjectSquadLinkBegin) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := s.inner.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return failProjectSquadLinkTx{Tx: tx}, nil
}

type failProjectSquadLinkTx struct{ pgx.Tx }

func (tx failProjectSquadLinkTx) QueryRow(ctx context.Context, query string, args ...any) pgx.Row {
	if strings.Contains(query, "-- name: UpdateProjectExecutionSquad") {
		return failedProjectSquadLinkRow{}
	}
	return tx.Tx.QueryRow(ctx, query, args...)
}

type failedProjectSquadLinkRow struct{}

func (failedProjectSquadLinkRow) Scan(...any) error {
	return errors.New("injected project link SQL failure")
}

func TestProjectExecutionSquad_ProjectLinkFailureRollsBackMaterialization(t *testing.T) {
	f := projectExecutionFixture(t)
	runtimeID := f.Runtime(t, "Rollback runtime")
	h := *testHandler
	h.TxStarter = failProjectSquadLinkBegin{inner: testPool}
	var out executionSquadTestProject
	testutil.Call(t, h.CreateProject, projectExecutionRequest(f, "POST", "", map[string]any{
		"title": "Saved despite link failure", "execution_squad": map[string]any{"template_key": "feature-delivery", "runtime_id": runtimeID},
	})).Want(http.StatusCreated).JSON(&out)
	if out.ExecutionSquad.State != "failed" || out.ExecutionSquad.ErrorCode != "preparation_failed" {
		t.Fatalf("unexpected failure response: %+v", out.ExecutionSquad)
	}
	for _, table := range []string{"squad", "agent", "skill"} {
		if count := f.Count(t, "SELECT count(*) FROM "+table+" WHERE workspace_id = $1", f.WorkspaceID); count != 0 {
			t.Errorf("project link rollback left %d %s rows", count, table)
		}
	}
	if count := f.Count(t, `SELECT count(*) FROM project WHERE workspace_id = $1`, f.WorkspaceID); count != 1 {
		t.Fatalf("preparation failure lost the project: count=%d", count)
	}
}

func TestProjectExecutionSquad_NormalizesMalformedStorage(t *testing.T) {
	for _, raw := range []string{
		`{}`, `null`, `[]`, `{"state":"future"}`, `{"state":"configured","squad_id":42}`,
		`{"state":"configured","squad_id":"bad-id"}`, `{"state":"needs_runtime","template_key":42}`,
	} {
		t.Run(raw, func(t *testing.T) {
			response := projectToResponse(db.Project{ExecutionSquad: []byte(raw)})
			if response.ExecutionSquad.State != "none" {
				t.Errorf("malformed selection %s normalized to %+v", raw, response.ExecutionSquad)
			}
		})
	}
	response := projectToResponse(db.Project{ExecutionSquad: []byte(`{"state":"needs_runtime","template_key":"feature-delivery","selection_revision":"internal","language":"zh"}`)})
	encoded, err := json.Marshal(response)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "selection_revision") || strings.Contains(string(encoded), `"language"`) {
		t.Fatalf("internal preparation metadata leaked: %s", encoded)
	}
}

func TestProjectExecutionSquad_ArchivedAndDeletedDefaultsRequireExplicitRecovery(t *testing.T) {
	for _, removal := range []string{"archive", "delete"} {
		t.Run(removal, func(t *testing.T) {
			f := projectExecutionFixture(t)
			runtimeID := f.Runtime(t, "Recover stale default")
			projectID := f.Project(t, "Stale default")
			selection := map[string]any{"template_key": "feature-delivery", "runtime_id": runtimeID}
			var first executionSquadTestProject
			testutil.Call(t, testHandler.ConfigureProjectSquad, projectExecutionRequest(f, "PUT", projectID, selection)).Want(http.StatusOK).JSON(&first)
			oldSquadID := first.ExecutionSquad.SquadID
			if removal == "archive" {
				f.Exec(t, `UPDATE squad SET archived_at = now() WHERE id = $1`, oldSquadID)
			} else {
				f.Exec(t, `DELETE FROM squad WHERE id = $1`, oldSquadID)
			}
			before := f.Count(t, `SELECT count(*) FROM squad WHERE workspace_id = $1`, f.WorkspaceID)
			var read executionSquadTestProject
			testutil.Call(t, testHandler.GetProject, projectExecutionRequest(f, "GET", projectID, nil)).Want(http.StatusOK).JSON(&read)
			if read.ExecutionSquad.SquadID != oldSquadID {
				t.Fatalf("GET mutated a stale default: %+v", read.ExecutionSquad)
			}
			if count := f.Count(t, `SELECT count(*) FROM squad WHERE workspace_id = $1`, f.WorkspaceID); count != before {
				t.Fatalf("GET materialized a replacement squad: %d -> %d", before, count)
			}
			testutil.Call(t, testHandler.ConfigureProjectSquad, projectExecutionRequest(f, "PUT", projectID, map[string]any{"squad_id": oldSquadID})).Want(http.StatusBadRequest)
			var recovered executionSquadTestProject
			testutil.Call(t, testHandler.ConfigureProjectSquad, projectExecutionRequest(f, "PUT", projectID, selection)).Want(http.StatusOK).JSON(&recovered)
			if recovered.ExecutionSquad.State != "configured" || recovered.ExecutionSquad.SquadID == oldSquadID {
				t.Fatalf("explicit template retry did not recover: %+v", recovered.ExecutionSquad)
			}
			template, _ := service.SquadTemplateByKey("feature-delivery")
			if count := f.Count(t, `SELECT count(*) FROM agent WHERE workspace_id = $1`, f.WorkspaceID); count != len(template.TemplateKeys()) {
				t.Fatalf("recovery duplicated role agents: got=%d want=%d", count, len(template.TemplateKeys()))
			}
		})
	}
}

func TestProjectExecutionSquad_UnboundAndOfflineRuntime(t *testing.T) {
	f := projectExecutionFixture(t)
	leaderID := f.Agent(t, "Unbound leader", "")
	squadID := f.Squad(t, "Unbound squad", leaderID)
	projectID := f.Project(t, "Runtime recovery")
	selection := map[string]any{"squad_id": squadID}
	var unavailable executionSquadTestProject
	testutil.Call(t, testHandler.ConfigureProjectSquad, projectExecutionRequest(f, "PUT", projectID, selection)).Want(http.StatusOK).JSON(&unavailable)
	if unavailable.ExecutionSquad.State != "failed" || unavailable.ExecutionSquad.ErrorCode != "runtime_unavailable" {
		t.Fatalf("unbound leader was considered available: %+v", unavailable.ExecutionSquad)
	}
	runtimeID := f.Runtime(t, "Offline bound runtime", testutil.Cols{"status": "offline"})
	f.Exec(t, `UPDATE agent SET runtime_id = $1 WHERE id = $2`, runtimeID, leaderID)
	var configured executionSquadTestProject
	testutil.Call(t, testHandler.ConfigureProjectSquad, projectExecutionRequest(f, "PUT", projectID, selection)).Want(http.StatusOK).JSON(&configured)
	if configured.ExecutionSquad.State != "configured" {
		t.Fatalf("configuration incorrectly required an online machine: %+v", configured.ExecutionSquad)
	}
	f.Exec(t, `UPDATE agent SET archived_at = now() WHERE id = $1`, leaderID)
	testutil.Call(t, testHandler.ConfigureProjectSquad, projectExecutionRequest(f, "PUT", projectID, selection)).Want(http.StatusBadRequest)
}

func TestProjectExecutionSquad_ResponsePreservesCountsInGetListSearchAndPut(t *testing.T) {
	f := projectExecutionFixture(t)
	projectID := f.Project(t, "Find execution squad")
	f.Issue(t, "Open project issue", testutil.Cols{"project_id": projectID, "status": "todo"})
	f.Issue(t, "Done project issue", testutil.Cols{"project_id": projectID, "status": "done"})
	f.Insert(t, "project_resource", testutil.Cols{
		"project_id": projectID, "workspace_id": f.WorkspaceID, "resource_type": "github_repo",
		"resource_ref": `{"url":"https://github.com/example/project"}`, "created_by": f.UserID,
	})
	var put executionSquadTestProject
	testutil.Call(t, testHandler.ConfigureProjectSquad, projectExecutionRequest(f, "PUT", projectID, map[string]any{"template_key": "feature-delivery"})).Want(http.StatusOK).JSON(&put)
	var get executionSquadTestProject
	testutil.Call(t, testHandler.GetProject, projectExecutionRequest(f, "GET", projectID, nil)).Want(http.StatusOK).JSON(&get)
	var list, search struct {
		Projects []executionSquadTestProject `json:"projects"`
	}
	testutil.Call(t, testHandler.ListProjects, projectExecutionRequest(f, "GET", "", nil)).Want(http.StatusOK).JSON(&list)
	searchReq := projectExecutionRequest(f, "GET", "", nil)
	searchReq.URL.RawQuery = "q=Find"
	testutil.Call(t, testHandler.SearchProjects, searchReq).Want(http.StatusOK).JSON(&search)
	if len(list.Projects) != 1 || len(search.Projects) != 1 {
		t.Fatalf("project missing from reads: list=%d search=%d", len(list.Projects), len(search.Projects))
	}
	for _, response := range []executionSquadTestProject{put, get, list.Projects[0], search.Projects[0]} {
		if response.ExecutionSquad.State != "needs_runtime" || response.IssueCount != 2 || response.DoneCount != 1 || response.ResourceCount != 1 {
			t.Errorf("project response lost selection or counts: %+v", response)
		}
	}
}

func TestProjectExecutionSquad_DelegatedActorUsesOriginatorInvocationRights(t *testing.T) {
	f := projectExecutionFixture(t)
	runtimeID := f.Runtime(t, "Delegated runtime")
	originatorID := f.User(t, "Different invoking human", uuid.NewString()+"@example.test")
	f.Member(t, f.WorkspaceID, originatorID, "member")
	actorID := f.Agent(t, "Delegated coordinator", runtimeID, testutil.Cols{"autonomy_level": "coordinator"})
	taskID := f.Task(t, actorID, testutil.Cols{
		"runtime_id": runtimeID, "status": "running", "originator_user_id": originatorID,
		"accountable_user_id": originatorID, "originator_source": "direct_human",
	})
	leaderID := f.Agent(t, "Runtime owner's private leader", runtimeID)
	squadID := f.Squad(t, "Runtime owner's private squad", leaderID)
	projectID := f.Project(t, "Originator permission")
	req := projectExecutionRequest(f, "PUT", projectID, map[string]any{"squad_id": squadID})
	testutil.WithHeaders(req, "X-Actor-Source", "task_token", "X-Agent-ID", actorID, "X-Task-ID", taskID)
	testutil.Call(t, testHandler.ConfigureProjectSquad, req).Want(http.StatusForbidden)
}

func TestProjectExecutionSquad_PublicReuseWorksWithOneConnection(t *testing.T) {
	f := projectExecutionFixture(t)
	otherID := f.User(t, "Shared agent owner", uuid.NewString()+"@example.test")
	runtimeID := f.Runtime(t, "Shared runtime")
	leaderID := f.Agent(t, "Shared leader", runtimeID, testutil.Cols{"owner_id": otherID, "permission_mode": "public_to"})
	f.InsertNoID(t, "agent_invocation_target", testutil.Cols{
		"agent_id": leaderID, "target_type": "workspace", "target_id": f.WorkspaceID,
	}, "agent_id = $1", leaderID)
	squadID := f.Squad(t, "Shared squad", leaderID, testutil.Cols{"template_key": "feature-delivery"})
	projectID := f.Project(t, "Single connection")
	config := testPool.Config().Copy()
	config.MaxConns, config.MinConns = 1, 0
	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	h := *testHandler
	h.Queries, h.TxStarter = db.New(pool), pool
	req := projectExecutionRequest(f, "PUT", projectID, map[string]any{"template_key": "feature-delivery", "runtime_id": runtimeID})
	ctx, cancel := context.WithTimeout(req.Context(), 3*time.Second)
	defer cancel()
	var out executionSquadTestProject
	testutil.Call(t, h.ConfigureProjectSquad, req.WithContext(ctx)).Want(http.StatusOK).JSON(&out)
	if out.ExecutionSquad.State != "configured" || out.ExecutionSquad.SquadID != squadID {
		t.Fatalf("public reuse did not stay on the transaction connection: %+v", out.ExecutionSquad)
	}
}

func TestProjectExecutionSquad_IgnoresForeignWorkspaceResourceRows(t *testing.T) {
	f := projectExecutionFixture(t)
	other := projectExecutionFixture(t)
	runtimeID := f.Runtime(t, "Scoped runtime")
	leaderID := f.Agent(t, "Scoped leader", runtimeID)
	squadID := f.Squad(t, "Scoped squad", leaderID)
	projectID := f.Project(t, "Scoped resources")
	other.Insert(t, "project_resource", testutil.Cols{
		"project_id": projectID, "workspace_id": other.WorkspaceID, "resource_type": "local_directory",
		"resource_ref": fmt.Sprintf(`{"daemon_id":%q,"local_path":"/foreign/project"}`, uuid.NewString()), "created_by": f.UserID,
	})
	var out executionSquadTestProject
	testutil.Call(t, testHandler.ConfigureProjectSquad, projectExecutionRequest(f, "PUT", projectID, map[string]any{"squad_id": squadID})).Want(http.StatusOK).JSON(&out)
	if out.ExecutionSquad.State != "configured" {
		t.Fatalf("foreign workspace resource affected configuration: %+v", out.ExecutionSquad)
	}
}
