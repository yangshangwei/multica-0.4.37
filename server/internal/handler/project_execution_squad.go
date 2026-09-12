package handler

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"sort"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/service"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// ProjectExecutionSquad is a saved default, not a claim that its machine is
// online. Live squad, agent and runtime availability remains independently read.
type ProjectExecutionSquad struct {
	State       string `json:"state"`
	TemplateKey string `json:"template_key,omitempty"`
	SquadID     string `json:"squad_id,omitempty"`
	RuntimeID   string `json:"runtime_id,omitempty"`
	ErrorCode   string `json:"error_code,omitempty"`
}

type ConfigureProjectSquadRequest struct {
	TemplateKey string `json:"template_key,omitempty"`
	SquadID     string `json:"squad_id,omitempty"`
	RuntimeID   string `json:"runtime_id,omitempty"`
	Language    string `json:"language,omitempty"`
}

func (r *ConfigureProjectSquadRequest) UnmarshalJSON(data []byte) error {
	if len(bytes.TrimSpace(data)) == 0 || bytes.TrimSpace(data)[0] != '{' {
		return errors.New("execution_squad must be an object")
	}
	type request ConfigureProjectSquadRequest
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	return decoder.Decode((*request)(r))
}

// The revision and language are internal: callers cannot supply the revision,
// and project responses expose only ProjectExecutionSquad. The revision bridges
// the committed project create and its separate preparation transaction.
type projectSquadSelection struct {
	ProjectExecutionSquad
	Revision string `json:"selection_revision,omitempty"`
	Language string `json:"language,omitempty"`
}

func readProjectSquadSelection(data []byte) projectSquadSelection {
	var selection projectSquadSelection
	if len(data) == 0 || json.Unmarshal(data, &selection) != nil {
		return projectSquadSelection{ProjectExecutionSquad: ProjectExecutionSquad{State: "none"}}
	}
	for _, id := range []string{selection.SquadID, selection.RuntimeID} {
		var parsed pgtype.UUID
		if id != "" && (parsed.Scan(id) != nil || !parsed.Valid) {
			return projectSquadSelection{ProjectExecutionSquad: ProjectExecutionSquad{State: "none"}}
		}
	}
	switch selection.State {
	case "needs_runtime":
		if selection.TemplateKey != "" {
			return selection
		}
	case "configured":
		if selection.SquadID != "" {
			return selection
		}
	case "failed":
		if selection.TemplateKey != "" || selection.SquadID != "" {
			return selection
		}
	}
	return projectSquadSelection{
		ProjectExecutionSquad: ProjectExecutionSquad{State: "none"},
		Revision:              selection.Revision,
	}
}

type projectSquadInput struct {
	Selection    projectSquadSelection
	Member       db.Member
	ActorType    string
	ActorID      string
	OriginatorID string
}

func (h *Handler) validateProjectSquadChoice(w http.ResponseWriter, r *http.Request, workspaceID pgtype.UUID, req ConfigureProjectSquadRequest) (projectSquadInput, bool) {
	var in projectSquadInput
	ws := uuidToString(workspaceID)
	member, ok := h.requireWorkspaceMember(w, r, ws, "workspace not found")
	if !ok {
		return in, false
	}
	req.TemplateKey = strings.TrimSpace(req.TemplateKey)
	req.SquadID = strings.TrimSpace(req.SquadID)
	req.RuntimeID = strings.TrimSpace(req.RuntimeID)
	if req.TemplateKey != "" && req.SquadID != "" {
		writeError(w, http.StatusBadRequest, "choose template_key or squad_id, not both")
		return in, false
	}
	if req.RuntimeID != "" && req.TemplateKey == "" {
		writeError(w, http.StatusBadRequest, "runtime_id requires template_key")
		return in, false
	}
	switch req.Language {
	case "", "en", "zh", "ja", "ko":
	default:
		writeError(w, http.StatusBadRequest, "language must be en, zh, ja, or ko")
		return in, false
	}
	if !h.requireAgentAutonomy(w, r, ws, service.AutonomyCoordinator, "configure project execution squads") {
		return in, false
	}
	in.Member = member
	in.ActorType, in.ActorID = h.resolveActor(r, uuidToString(member.UserID), ws)
	in.OriginatorID = h.invokeOriginatorFromRequest(r, in.ActorType, in.ActorID)
	in.Selection = projectSquadSelection{
		ProjectExecutionSquad: ProjectExecutionSquad{State: "none"},
		Revision:              rand.Text(),
	}
	if req.TemplateKey != "" {
		template, exists := service.SquadTemplateByKey(req.TemplateKey)
		if !exists {
			writeError(w, http.StatusBadRequest, "unknown template_key")
			return in, false
		}
		if !h.requireAgentMayGrantAutonomy(w, r, ws, template.MaxAutonomy()) {
			return in, false
		}
		in.Selection.TemplateKey = template.Key
		in.Selection.Language = templateLanguageFromRequest(req.Language)
		in.Selection.State = "needs_runtime"
		if req.RuntimeID == "" {
			return in, true
		}
		runtimeID, valid := parseUUIDOrBadRequest(w, req.RuntimeID, "runtime_id")
		if !valid {
			return in, false
		}
		runtime, err := h.Queries.GetAgentRuntimeForWorkspace(r.Context(), db.GetAgentRuntimeForWorkspaceParams{ID: runtimeID, WorkspaceID: workspaceID})
		if err != nil {
			writeError(w, http.StatusBadRequest, "runtime not found in this workspace")
			return in, false
		}
		if !canUseRuntimeForAgent(member, runtime) {
			writeError(w, http.StatusForbidden, "this runtime is private; only its owner can create agents on it")
			return in, false
		}
		in.Selection.RuntimeID = uuidToString(runtime.ID)
	} else if req.SquadID != "" {
		squadID, valid := parseUUIDOrBadRequest(w, req.SquadID, "squad_id")
		if !valid {
			return in, false
		}
		squad, err := h.Queries.GetSquadInWorkspace(r.Context(), db.GetSquadInWorkspaceParams{ID: squadID, WorkspaceID: workspaceID})
		if err != nil || squad.ArchivedAt.Valid {
			writeError(w, http.StatusBadRequest, "squad not found or archived in this workspace")
			return in, false
		}
		leader, err := h.Queries.GetAgentInWorkspace(r.Context(), db.GetAgentInWorkspaceParams{ID: squad.LeaderID, WorkspaceID: workspaceID})
		if err != nil || leader.ArchivedAt.Valid {
			writeError(w, http.StatusBadRequest, "squad leader is unavailable")
			return in, false
		}
		if !in.canInvoke(r.Context(), h.Queries, leader, ws) {
			writeError(w, http.StatusForbidden, "you do not have access to invoke this squad leader")
			return in, false
		}
		in.Selection.SquadID = uuidToString(squad.ID)
	} else {
		return in, true
	}
	// If a preparation transaction cannot even start, the already-created
	// project still contains an honest, retryable choice rather than a false
	// configured state or a 5xx that invites repeating the project POST.
	in.Selection.State = "failed"
	in.Selection.ErrorCode = "preparation_failed"
	return in, true
}

func (in projectSquadInput) canInvoke(ctx context.Context, queries *db.Queries, agent db.Agent, workspaceID string) bool {
	return memberCanWireAgentWithQueries(ctx, queries, in.Member, agent, workspaceID) &&
		invokeAgentDecision(ctx, queries, agent, in.ActorType, in.ActorID, in.OriginatorID, workspaceID)
}

func sameProjectSquadChoice(a, b projectSquadSelection) bool {
	if a.TemplateKey != "" || b.TemplateKey != "" {
		return a.TemplateKey == b.TemplateKey && a.RuntimeID == b.RuntimeID
	}
	return a.SquadID == b.SquadID
}

func (h *Handler) ConfigureProjectSquad(w http.ResponseWriter, r *http.Request) {
	projectID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "project id")
	if !ok {
		return
	}
	workspaceID, ok := parseUUIDOrBadRequest(w, h.resolveWorkspaceID(r), "workspace_id")
	if !ok {
		return
	}
	var req *ConfigureProjectSquadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req == nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	in, ok := h.validateProjectSquadChoice(w, r, workspaceID, *req)
	if !ok {
		return
	}
	project, staged, err := h.configureProjectSquad(r.Context(), projectID, workspaceID, in, "")
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "project not found")
			return
		}
		slog.WarnContext(r.Context(), "configure project execution squad failed", "project_id", uuidToString(projectID), "error", err)
		writeError(w, http.StatusInternalServerError, "failed to save project execution squad")
		return
	}
	h.publishProjectSquadMaterialization(r.Context(), project, in, staged)
	resp := projectToResponse(project)
	resp.IssueCount, resp.DoneCount = h.loadProjectIssueStats(r.Context(), workspaceID, projectID)
	resp.ResourceCount = h.loadProjectResourceCount(r.Context(), projectID)
	h.publish(protocol.EventProjectUpdated, uuidToString(workspaceID), in.ActorType, in.ActorID, map[string]any{"project": resp})
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) configureProjectSquad(ctx context.Context, projectID, workspaceID pgtype.UUID, in projectSquadInput, expectedRevision string) (db.Project, *squadTemplateProvisionResult, error) {
	tx, err := h.TxStarter.Begin(ctx)
	if err != nil {
		return db.Project{}, nil, err
	}
	defer tx.Rollback(ctx)
	qtx := h.Queries.WithTx(tx)
	project, err := qtx.LockProjectForExecutionSquad(ctx, db.LockProjectForExecutionSquadParams{ID: projectID, WorkspaceID: workspaceID})
	if err != nil {
		return db.Project{}, nil, err
	}
	current := readProjectSquadSelection(project.ExecutionSquad)
	if expectedRevision != "" && current.Revision != expectedRevision {
		return project, nil, nil
	}
	selection := in.Selection
	if sameProjectSquadChoice(current, selection) && current.Revision != "" {
		selection.Revision = current.Revision
		selection.Language = current.Language
		selection.SquadID = current.SquadID
	}
	in.Selection = selection
	var staged *squadTemplateProvisionResult
	if selection.State != "none" && selection.State != "needs_runtime" {
		// A failed SQL statement aborts its transaction. A savepoint lets us roll
		// back all roster writes and still commit the selected failure state.
		preparation, err := tx.Begin(ctx)
		if err != nil {
			return db.Project{}, nil, err
		}
		squad, materialized, prepareErr := h.prepareProjectSquadInTx(ctx, preparation, project, in)
		if prepareErr != nil {
			if err := preparation.Rollback(ctx); err != nil {
				return db.Project{}, nil, err
			}
			selection.State = "failed"
			selection.ErrorCode = projectSquadErrorCode(prepareErr)
			slog.WarnContext(ctx, "project execution squad preparation failed", "project_id", uuidToString(projectID), "error_code", selection.ErrorCode, "error", prepareErr)
		} else {
			selection.State = "configured"
			selection.SquadID = uuidToString(squad.ID)
			selection.ErrorCode = ""
			if err := preparation.Commit(ctx); err != nil {
				return db.Project{}, nil, err
			}
			staged = materialized
		}
	}
	if selection != current {
		encoded, err := json.Marshal(selection)
		if err != nil {
			return db.Project{}, nil, err
		}
		project, err = qtx.UpdateProjectExecutionSquad(ctx, db.UpdateProjectExecutionSquadParams{ID: projectID, WorkspaceID: workspaceID, ExecutionSquad: encoded})
		if err != nil {
			return db.Project{}, nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return db.Project{}, nil, err
	}
	return project, staged, nil
}

type projectSquadPreparationError string

func (e projectSquadPreparationError) Error() string { return string(e) }

func projectSquadErrorCode(err error) string {
	var preparation projectSquadPreparationError
	if errors.As(err, &preparation) {
		return string(preparation)
	}
	var conflict *agentNameConflictError
	if errors.As(err, &conflict) {
		return "agent_name_conflict"
	}
	var denied errTemplateAgentNotWireable
	if errors.As(err, &denied) {
		return "agent_access_denied"
	}
	return "preparation_failed"
}

func (h *Handler) prepareProjectSquadInTx(ctx context.Context, tx pgx.Tx, project db.Project, in projectSquadInput) (db.Squad, *squadTemplateProvisionResult, error) {
	qtx := h.Queries.WithTx(tx)
	resources, err := qtx.ListProjectResourcesInWorkspace(ctx, db.ListProjectResourcesInWorkspaceParams{ProjectID: project.ID, WorkspaceID: project.WorkspaceID})
	if err != nil {
		return db.Squad{}, nil, err
	}
	localDaemons := make(map[string]bool)
	for _, resource := range resources {
		if resource.ResourceType == "local_directory" {
			var ref localDirectoryRef
			if err := json.Unmarshal(resource.ResourceRef, &ref); err != nil || ref.DaemonID == "" {
				return db.Squad{}, nil, projectSquadPreparationError("runtime_mismatch")
			}
			localDaemons[ref.DaemonID] = true
		}
	}
	validateAgent := func(ctx context.Context, queries *db.Queries, agent db.Agent) error {
		// Locking the actual agent also protects reused customized bindings from
		// changing between validation and the committed project reference.
		locked, err := queries.LockAgentForAutopilotAssignment(ctx, db.LockAgentForAutopilotAssignmentParams{ID: agent.ID, WorkspaceID: project.WorkspaceID})
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && locked.ArchivedAt.Valid) {
			return projectSquadPreparationError("agent_unavailable")
		}
		if err != nil {
			return err
		}
		if !in.canInvoke(ctx, queries, locked, uuidToString(project.WorkspaceID)) {
			return projectSquadPreparationError("agent_access_denied")
		}
		if !locked.RuntimeID.Valid {
			return projectSquadPreparationError("runtime_unavailable")
		}
		if in.Selection.RuntimeID != "" && uuidToString(locked.RuntimeID) != in.Selection.RuntimeID {
			return projectSquadPreparationError("runtime_mismatch")
		}
		runtime, err := queries.GetAgentRuntimeForWorkspace(ctx, db.GetAgentRuntimeForWorkspaceParams{ID: locked.RuntimeID, WorkspaceID: project.WorkspaceID})
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && !runtime.OwnerID.Valid) {
			return projectSquadPreparationError("runtime_unavailable")
		}
		if err != nil {
			return err
		}
		if len(localDaemons) > 0 && (!runtime.DaemonID.Valid || !localDaemons[runtime.DaemonID.String]) {
			return projectSquadPreparationError("runtime_mismatch")
		}
		return nil
	}
	if in.Selection.TemplateKey == "" {
		squad, err := h.loadInvocableProjectSquad(ctx, qtx, project.WorkspaceID, in.Selection.SquadID, validateAgent)
		return squad, nil, err
	}
	template, ok := service.SquadTemplateByKey(in.Selection.TemplateKey)
	if !ok {
		return db.Squad{}, nil, projectSquadPreparationError("squad_unavailable")
	}
	runtime, err := qtx.LockRuntimeForProjectSquad(ctx, db.LockRuntimeForProjectSquadParams{ID: parseUUID(in.Selection.RuntimeID), WorkspaceID: project.WorkspaceID})
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && !canUseRuntimeForAgent(in.Member, runtime)) {
		return db.Squad{}, nil, projectSquadPreparationError("runtime_unavailable")
	}
	if err != nil {
		return db.Squad{}, nil, err
	}
	provision := squadTemplateProvisionInput{
		Template: template, WorkspaceID: project.WorkspaceID, OwnerID: in.Member.UserID,
		Runtime: runtime, Permission: resolvedPermission{mode: "private"},
		Language: in.Selection.Language, SquadName: template.Title(in.Selection.Language),
		Member: in.Member, WorkspaceIDString: uuidToString(project.WorkspaceID), ValidateAgent: validateAgent,
	}
	if err := lockSquadTemplateProvisioning(ctx, tx, provision); err != nil {
		return db.Squad{}, nil, err
	}
	if in.Selection.SquadID != "" {
		squad, err := h.loadInvocableProjectSquad(ctx, qtx, project.WorkspaceID, in.Selection.SquadID, validateAgent)
		if err == nil {
			return squad, nil, nil
		}
		var unavailable projectSquadPreparationError
		if !errors.As(err, &unavailable) {
			return db.Squad{}, nil, err
		}
	}
	candidates, err := qtx.ListSquadsByTemplateForProject(ctx, db.ListSquadsByTemplateForProjectParams{WorkspaceID: project.WorkspaceID, TemplateKey: template.Key})
	if err != nil {
		return db.Squad{}, nil, err
	}
	for _, candidate := range candidates {
		squad, err := h.loadInvocableProjectSquad(ctx, qtx, project.WorkspaceID, uuidToString(candidate.ID), validateAgent)
		if err == nil {
			return squad, nil, nil
		}
		var unavailable projectSquadPreparationError
		if !errors.As(err, &unavailable) {
			return db.Squad{}, nil, err
		}
	}
	staged, err := h.materializeSquadTemplateInTx(ctx, tx, provision)
	if err != nil {
		return db.Squad{}, nil, err
	}
	return staged.Squad, &staged, nil
}

func (h *Handler) loadInvocableProjectSquad(ctx context.Context, queries *db.Queries, workspaceID pgtype.UUID, squadID string, validateAgent func(context.Context, *db.Queries, db.Agent) error) (db.Squad, error) {
	squad, err := queries.LockSquadForAutopilotAssignment(ctx, db.LockSquadForAutopilotAssignmentParams{ID: parseUUID(squadID), WorkspaceID: workspaceID})
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && squad.ArchivedAt.Valid) {
		return db.Squad{}, projectSquadPreparationError("squad_unavailable")
	}
	if err != nil {
		return db.Squad{}, err
	}
	members, err := queries.ListSquadMembers(ctx, squad.ID)
	if err != nil {
		return db.Squad{}, err
	}
	ids := map[string]pgtype.UUID{uuidToString(squad.LeaderID): squad.LeaderID}
	for _, member := range members {
		if member.MemberType == "agent" {
			ids[uuidToString(member.MemberID)] = member.MemberID
		}
	}
	ordered := make([]string, 0, len(ids))
	for id := range ids {
		ordered = append(ordered, id)
	}
	sort.Strings(ordered)
	for _, id := range ordered {
		if err := validateAgent(ctx, queries, db.Agent{ID: ids[id]}); err != nil {
			return db.Squad{}, err
		}
	}
	return squad, nil
}

func (h *Handler) prepareCreatedProjectSquad(ctx context.Context, project db.Project, in *projectSquadInput) db.Project {
	if in == nil || in.Selection.State == "none" || in.Selection.State == "needs_runtime" {
		return project
	}
	prepared, staged, err := h.configureProjectSquad(ctx, project.ID, project.WorkspaceID, *in, in.Selection.Revision)
	if err != nil {
		slog.WarnContext(ctx, "prepare saved project execution squad failed", "project_id", uuidToString(project.ID), "error", err)
		return project
	}
	h.publishProjectSquadMaterialization(ctx, prepared, *in, staged)
	return prepared
}

func (h *Handler) publishProjectSquadMaterialization(ctx context.Context, project db.Project, in projectSquadInput, staged *squadTemplateProvisionResult) {
	if staged == nil {
		return
	}
	ws := uuidToString(project.WorkspaceID)
	for _, agent := range staged.CreatedAgents {
		if h.TaskService != nil {
			h.TaskService.ReconcileAgentStatus(ctx, agent.ID)
		}
		h.publish(protocol.EventAgentCreated, ws, in.ActorType, in.ActorID, map[string]any{"agent": broadcastAgentResponse(h.agentToResponse(agent))})
	}
	squad, err := h.squadToResponseWithPreview(ctx, staged.Squad)
	if err != nil {
		slog.WarnContext(ctx, "load project execution squad preview failed", "squad_id", uuidToString(staged.Squad.ID), "error", err)
		return
	}
	h.publish(protocol.EventSquadCreated, ws, in.ActorType, in.ActorID, map[string]any{"squad": squad})
}
