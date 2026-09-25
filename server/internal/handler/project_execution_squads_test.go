package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/multica-ai/multica/server/internal/service"

	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type executionSquadsTestProject struct {
	ProjectResponse
	ExecutionSquads []ProjectExecutionSquad `json:"execution_squads"`
}

func TestProjectExecutionSquads_CreateRetainsOrderedChoices(t *testing.T) {
	f := projectExecutionFixture(t)
	var out executionSquadsTestProject
	testutil.Call(t, testHandler.CreateProject, projectExecutionRequest(f, "POST", "", map[string]any{
		"title": "Multiple project squads", "execution_squads": []map[string]any{
			{"template_key": "feature-delivery"}, {"template_key": "bug-fix"},
		},
	})).Want(http.StatusCreated).JSON(&out)
	if len(out.ExecutionSquads) != 2 || out.ExecutionSquads[0].TemplateKey != "feature-delivery" || out.ExecutionSquads[1].TemplateKey != "bug-fix" {
		t.Fatalf("create lost selected order: %+v", out.ExecutionSquads)
	}
	if out.ExecutionSquad != out.ExecutionSquads[0] {
		t.Fatalf("legacy default must expose first selection: %+v", out)
	}
	var stored string
	f.QueryRow(t, `SELECT jsonb_typeof(execution_squad) FROM project WHERE id = $1`, out.ID).Scan(&stored)
	if stored != "array" {
		t.Fatalf("new selections must use array storage, got %s", stored)
	}
}

func TestProjectExecutionSquads_RejectsInvalidBatchBeforeCreation(t *testing.T) {
	f := projectExecutionFixture(t)
	for name, body := range map[string]map[string]any{
		"mixed fields":     {"execution_squad": map[string]any{}, "execution_squads": []any{}},
		"bad later choice": {"execution_squads": []any{map[string]any{"template_key": "feature-delivery"}, map[string]any{"squad_id": "bad"}}},
		"duplicate choice": {"execution_squads": []any{map[string]any{"template_key": "feature-delivery"}, map[string]any{"template_key": "feature-delivery"}}},
		"empty entry":      {"execution_squads": []any{map[string]any{}}},
		"null list":        {"execution_squads": nil},
		"null entry":       {"execution_squads": []any{nil}},
	} {
		t.Run(name, func(t *testing.T) {
			body["title"] = name
			testutil.Call(t, testHandler.CreateProject, projectExecutionRequest(f, "POST", "", body)).Want(http.StatusBadRequest)
		})
	}
	if count := f.Count(t, `SELECT count(*) FROM project WHERE workspace_id = $1`, f.WorkspaceID); count != 0 {
		t.Fatalf("invalid batch created %d projects", count)
	}
}

func TestProjectExecutionSquads_NormalizesLegacyAndMalformedStorage(t *testing.T) {
	for _, tc := range []struct {
		raw   string
		count int
	}{
		{`{"state":"needs_runtime","template_key":"feature-delivery","selection_revision":"internal"}`, 1},
		{`[{"state":"needs_runtime","template_key":"feature-delivery"},{"state":"needs_runtime","template_key":"bug-fix"}]`, 2},
		{`[]`, 0}, {`{}`, 0}, {`null`, 0},
		{`[{"state":"configured","squad_id":"bad"}]`, 0},
		{`[{"state":"needs_runtime","template_key":"feature-delivery"},null]`, 0},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			encoded, err := json.Marshal(projectToResponse(db.Project{ExecutionSquad: []byte(tc.raw)}))
			if err != nil {
				t.Fatal(err)
			}
			var out executionSquadsTestProject
			if err := json.Unmarshal(encoded, &out); err != nil {
				t.Fatal(err)
			}
			if out.ExecutionSquads == nil || len(out.ExecutionSquads) != tc.count {
				t.Fatalf("storage normalized incorrectly: %s", encoded)
			}
			if tc.count > 0 && out.ExecutionSquad != out.ExecutionSquads[0] {
				t.Fatalf("legacy default differs from first selection: %s", encoded)
			}
		})
	}
}

func TestProjectExecutionSquads_TemplateAndExistingAliasAppearOnce(t *testing.T) {
	f := projectExecutionFixture(t)
	runtimeID := f.Runtime(t, "Alias runtime")
	projectID := f.Project(t, "Alias project")
	choice := map[string]any{"template_key": "feature-delivery", "runtime_id": runtimeID}
	var first ProjectResponse
	testutil.Call(t, testHandler.ConfigureProjectSquad, projectExecutionRequest(f, "PUT", projectID, choice)).Want(http.StatusOK).JSON(&first)
	var out ProjectResponse
	testutil.Call(t, testHandler.ConfigureProjectSquads, projectExecutionRequest(f, "PUT", projectID, map[string]any{
		"squads": []map[string]any{choice, {"squad_id": first.ExecutionSquad.SquadID}},
	})).Want(http.StatusOK).JSON(&out)
	if len(out.ExecutionSquads) != 1 || out.ExecutionSquads[0] != first.ExecutionSquad {
		t.Fatalf("template and workspace instance alias were not collapsed in order: %+v", out.ExecutionSquads)
	}
}

func TestProjectExecutionSquads_ReplaceReorderAndLegacyClear(t *testing.T) {
	f := projectExecutionFixture(t)
	runtimeID := f.Runtime(t, "Batch runtime")
	projectID := f.Project(t, "Batch project")
	choices := []map[string]any{{"template_key": "feature-delivery", "runtime_id": runtimeID}, {"template_key": "docs", "runtime_id": runtimeID}}
	put := func(choices []map[string]any) ProjectResponse {
		t.Helper()
		var out ProjectResponse
		testutil.Call(t, testHandler.ConfigureProjectSquads, projectExecutionRequest(f, "PUT", projectID, map[string]any{"squads": choices})).Want(http.StatusOK).JSON(&out)
		return out
	}
	first := put(choices)
	if len(first.ExecutionSquads) != 2 {
		t.Fatalf("batch was not saved: %+v", first)
	}
	for _, squad := range first.ExecutionSquads {
		if squad.State != "configured" {
			t.Fatalf("batch not configured: %+v", first.ExecutionSquads)
		}
	}
	f.Exec(t, `UPDATE agent SET instructions = 'batch customization' WHERE workspace_id = $1`, f.WorkspaceID)
	again := put(choices)
	if !slices.Equal(first.ExecutionSquads, again.ExecutionSquads) || first.UpdatedAt != again.UpdatedAt {
		t.Fatalf("identical replacement changed saved choices: first=%+v again=%+v", first, again)
	}
	reordered := put([]map[string]any{choices[1], choices[0]})
	if reordered.ExecutionSquads[0] != first.ExecutionSquads[1] || reordered.ExecutionSquad != first.ExecutionSquads[1] {
		t.Fatalf("reordering lost instance identity or legacy default: %+v", reordered)
	}
	if count := f.Count(t, `SELECT count(*) FROM agent WHERE workspace_id = $1 AND instructions <> 'batch customization'`, f.WorkspaceID); count != 0 {
		t.Fatalf("batch retry overwrote %d customized agents", count)
	}
	var legacy ProjectResponse
	testutil.Call(t, testHandler.ConfigureProjectSquad, projectExecutionRequest(f, "PUT", projectID, choices[0])).Want(http.StatusOK).JSON(&legacy)
	if len(legacy.ExecutionSquads) != 1 || legacy.ExecutionSquads[0] != first.ExecutionSquads[0] {
		t.Fatalf("legacy PUT must replace entire list: %+v", legacy)
	}
	cleared := put([]map[string]any{})
	if len(cleared.ExecutionSquads) != 0 || cleared.ExecutionSquad.State != "none" {
		t.Fatalf("clear retained choices: %+v", cleared)
	}
	if count := f.Count(t, `SELECT count(*) FROM squad WHERE workspace_id = $1`, f.WorkspaceID); count != 2 {
		t.Fatalf("clear deleted squads or retry duplicated them: %d", count)
	}
}

type failDocsSquadBegin struct{ inner txStarter }

func (s failDocsSquadBegin) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := s.inner.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return failDocsSquadTx{tx}, nil
}

type failDocsSquadTx struct{ pgx.Tx }

func (tx failDocsSquadTx) Begin(ctx context.Context) (pgx.Tx, error) {
	nested, err := tx.Tx.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return failDocsSquadTx{nested}, nil
}

func (tx failDocsSquadTx) QueryRow(ctx context.Context, query string, args ...any) pgx.Row {
	if strings.Contains(query, "-- name: CreateSquadFromTemplate :") && args[1] == "Docs Squad" {
		return tx.Tx.QueryRow(ctx, "SELECT 1 / 0")
	}
	return tx.Tx.QueryRow(ctx, query, args...)
}

func TestProjectExecutionSquads_SQLFailureRollsBackOnlyItsChoice(t *testing.T) {
	f := projectExecutionFixture(t)
	runtimeID := f.Runtime(t, "Partial batch runtime")
	choices := []map[string]any{{"template_key": "feature-delivery", "runtime_id": runtimeID}, {"template_key": "docs", "runtime_id": runtimeID}, {"template_key": "bug-fix", "runtime_id": runtimeID}}
	h := *testHandler
	h.TxStarter = failDocsSquadBegin{testPool}
	var out ProjectResponse
	testutil.Call(t, h.CreateProject, projectExecutionRequest(f, "POST", "", map[string]any{"title": "Partial success", "execution_squads": choices})).Want(http.StatusCreated).JSON(&out)
	if len(out.ExecutionSquads) != 3 || out.ExecutionSquads[0].State != "configured" || out.ExecutionSquads[1].ErrorCode != "preparation_failed" || out.ExecutionSquads[2].State != "configured" {
		t.Fatalf("one SQL failure corrupted the batch: %+v", out.ExecutionSquads)
	}
	if count := f.Count(t, `SELECT count(*) FROM agent WHERE workspace_id = $1 AND template_key IN ('docs-lead', 'technical-writer')`, f.WorkspaceID); count != 0 {
		t.Fatalf("failed savepoint leaked %d role agents", count)
	}
	if count := f.Count(t, `SELECT count(*) FROM squad WHERE workspace_id = $1`, f.WorkspaceID); count != 2 {
		t.Fatalf("successful squads did not commit independently: %d", count)
	}
	var recovered ProjectResponse
	testutil.Call(t, testHandler.ConfigureProjectSquads, projectExecutionRequest(f, "PUT", out.ID, map[string]any{"squads": choices})).Want(http.StatusOK).JSON(&recovered)
	if len(recovered.ExecutionSquads) != 3 || recovered.ExecutionSquads[1].State != "configured" || recovered.ExecutionSquads[0] != out.ExecutionSquads[0] || recovered.ExecutionSquads[2] != out.ExecutionSquads[2] {
		t.Fatalf("retry failed or replaced other choices: %+v", recovered.ExecutionSquads)
	}
}

func TestProjectExecutionSquads_StaleCreateCannotRestoreRemovedLaterChoice(t *testing.T) {
	f := projectExecutionFixture(t)
	runtimeID := f.Runtime(t, "Stale batch runtime")
	reached, release := make(chan struct{}, 1), make(chan struct{})
	h := *testHandler
	h.TxStarter = pauseProjectSquadBegin{inner: testPool, reached: reached, release: release}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	done := make(chan *testutil.Response, 1)
	go func() {
		done <- testutil.Call(t, h.CreateProject, projectExecutionRequest(f, "POST", "", map[string]any{
			"title": "Stale batch", "execution_squads": []map[string]any{{"template_key": "feature-delivery"}, {"template_key": "bug-fix", "runtime_id": runtimeID}},
		}).WithContext(ctx))
	}()
	select {
	case <-reached:
	case <-ctx.Done():
		t.Fatal("create did not reach preparation gap")
	}
	var projectID string
	f.QueryRow(t, `SELECT id::text FROM project WHERE workspace_id = $1`, f.WorkspaceID).Scan(&projectID)
	var newer ProjectResponse
	testutil.Call(t, testHandler.ConfigureProjectSquads, projectExecutionRequest(f, "PUT", projectID, map[string]any{
		"squads": []map[string]any{{"template_key": "feature-delivery"}, {"template_key": "docs"}},
	})).Want(http.StatusOK).JSON(&newer)
	close(release)
	var out ProjectResponse
	(<-done).Want(http.StatusCreated).JSON(&out)
	if !slices.Equal(out.ExecutionSquads, newer.ExecutionSquads) {
		t.Fatalf("stale create restored replaced choice: %+v", out.ExecutionSquads)
	}
	if count := f.Count(t, `SELECT count(*) FROM squad WHERE workspace_id = $1`, f.WorkspaceID); count != 0 {
		t.Fatalf("stale batch created %d orphan squads", count)
	}
}

func TestProjectExecutionSquads_ConcurrentOppositeOrdersReuseEachTemplate(t *testing.T) {
	f := projectExecutionFixture(t)
	runtimeID := f.Runtime(t, "Concurrent batch runtime")
	forward := []map[string]any{}
	for _, template := range service.SquadTemplates() {
		forward = append(forward, map[string]any{"template_key": template.Key, "runtime_id": runtimeID})
	}
	reverse := slices.Clone(forward)
	slices.Reverse(reverse)
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	responses := make(chan *testutil.Response, 3)
	for i, choices := range [][]map[string]any{forward, reverse} {
		projectID := f.Project(t, fmt.Sprintf("Concurrent batch %d", i))
		go func() {
			responses <- testutil.Call(t, testHandler.ConfigureProjectSquads, testutil.WithURLParams(projectExecutionRequest(f, "PUT", projectID, map[string]any{"squads": choices}).WithContext(ctx), "id", projectID))
		}()
	}
	singleID := f.Project(t, "Concurrent single selection")
	go func() {
		responses <- testutil.Call(t, testHandler.ConfigureProjectSquad, testutil.WithURLParams(projectExecutionRequest(f, "PUT", singleID, forward[len(forward)-1]).WithContext(ctx), "id", singleID))
	}()
	ids := map[string]string{}
	for range 3 {
		var out ProjectResponse
		(<-responses).Want(http.StatusOK).JSON(&out)
		for _, squad := range out.ExecutionSquads {
			if squad.State != "configured" {
				t.Fatalf("concurrent materialization failed: %+v", squad)
			}
			if previous := ids[squad.TemplateKey]; previous != "" && previous != squad.SquadID {
				t.Fatalf("template %s materialized twice", squad.TemplateKey)
			}
			ids[squad.TemplateKey] = squad.SquadID
		}
	}
	if len(ids) != len(forward) {
		t.Fatalf("missing templates: got %d want %d", len(ids), len(forward))
	}
	if count := f.Count(t, `SELECT count(*) FROM squad WHERE workspace_id = $1`, f.WorkspaceID); count != len(forward) {
		t.Fatalf("concurrent batches created %d squads for %d templates", count, len(forward))
	}
}

func TestProjectExecutionSquads_RejectsMalformedReplaceWithoutChangingSelection(t *testing.T) {
	f := projectExecutionFixture(t)
	projectID := f.Project(t, "Protected choices")
	var initial ProjectResponse
	testutil.Call(t, testHandler.ConfigureProjectSquad, projectExecutionRequest(f, "PUT", projectID, map[string]any{"template_key": "docs"})).Want(http.StatusOK).JSON(&initial)
	for _, body := range []string{`{}`, `null`, `[]`, `{"squads":null}`, `{"squads":{}}`, `{"squads":[null]}`, `{"squads":[{}]}`, `{"squads":[],"typo":true}`, `{"squads":[{"template_key":"docs"},{"template_key":"unknown"}]}`} {
		testutil.Call(t, testHandler.ConfigureProjectSquads, projectExecutionRequest(f, "PUT", projectID, body)).Want(http.StatusBadRequest)
	}
	var stored ProjectResponse
	testutil.Call(t, testHandler.GetProject, projectExecutionRequest(f, "GET", projectID, nil)).Want(http.StatusOK).JSON(&stored)
	if !slices.Equal(initial.ExecutionSquads, stored.ExecutionSquads) {
		t.Fatalf("invalid replacement changed project: %+v", stored.ExecutionSquads)
	}
}

func TestProjectExecutionSquads_FinalLinkFailureRollsBackWholeBatch(t *testing.T) {
	f := projectExecutionFixture(t)
	runtimeID := f.Runtime(t, "Atomic batch runtime")
	h := *testHandler
	h.TxStarter = failProjectSquadLinkBegin{inner: testPool}
	var out ProjectResponse
	testutil.Call(t, h.CreateProject, projectExecutionRequest(f, "POST", "", map[string]any{
		"title": "Retained after batch link failure", "execution_squads": []map[string]any{
			{"template_key": "feature-delivery", "runtime_id": runtimeID}, {"template_key": "docs", "runtime_id": runtimeID},
		},
	})).Want(http.StatusCreated).JSON(&out)
	if len(out.ExecutionSquads) != 2 {
		t.Fatalf("link failure lost retry choices: %+v", out.ExecutionSquads)
	}
	for _, squad := range out.ExecutionSquads {
		if squad.State != "failed" || squad.ErrorCode != "preparation_failed" {
			t.Fatalf("rolled-back choice reported as configured: %+v", squad)
		}
	}
	for _, table := range []string{"squad", "agent", "skill"} {
		if count := f.Count(t, "SELECT count(*) FROM "+table+" WHERE workspace_id = $1", f.WorkspaceID); count != 0 {
			t.Fatalf("batch rollback left %d %s rows", count, table)
		}
	}
	if count := f.Count(t, `SELECT count(*) FROM project WHERE workspace_id = $1`, f.WorkspaceID); count != 1 {
		t.Fatalf("saved project was lost: %d", count)
	}
}

func TestProjectExecutionSquads_LaterForbiddenChoiceRejectsWholeCreate(t *testing.T) {
	f := projectExecutionFixture(t)
	runtimeID := f.Runtime(t, "Allowed batch runtime")
	other := projectExecutionFixture(t)
	foreignRuntime := other.Runtime(t, "Foreign batch runtime")
	testutil.Call(t, testHandler.CreateProject, projectExecutionRequest(f, "POST", "", map[string]any{
		"title": "Rejected foreign batch", "execution_squads": []map[string]any{
			{"template_key": "feature-delivery", "runtime_id": runtimeID}, {"template_key": "docs", "runtime_id": foreignRuntime},
		},
	})).Want(http.StatusBadRequest)
	for _, table := range []string{"project", "squad", "agent", "skill"} {
		if count := f.Count(t, "SELECT count(*) FROM "+table+" WHERE workspace_id = $1", f.WorkspaceID); count != 0 {
			t.Fatalf("late rejected choice left %d %s rows", count, table)
		}
	}
}
