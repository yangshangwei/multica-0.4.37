package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/analytics"
	"github.com/multica-ai/multica/server/internal/logger"
	obsmetrics "github.com/multica-ai/multica/server/internal/metrics"
	"github.com/multica-ai/multica/server/internal/service"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// Staffing a squad from a built-in template.
//
// This changes nothing about how squads run. The result is an ordinary squad: the
// leader receives the work and dispatches by @mention, exactly as a hand-built one
// does. What the template supplies is the roster and the routing policy — the part
// teams get wrong when they wire five agents together for the first time.
//
// Everything happens in ONE transaction, which is why this path does not create its
// agents by calling the HTTP create handler in a loop: a squad that came up with
// three of its five members would be worse than one that failed outright, and the
// per-request path cannot span the squad row and its roster.

// SquadTemplateResponse is one entry in the squad-template picker.
type SquadTemplateResponse struct {
	Key          string                      `json:"key"`
	Version      int32                       `json:"version"`
	Name         string                      `json:"name"`
	Title        string                      `json:"title"`
	Description  string                      `json:"description"`
	AvatarEmoji  string                      `json:"avatar_emoji"`
	Instructions string                      `json:"instructions"`
	Leader       SquadTemplateRoleResponse   `json:"leader"`
	Members      []SquadTemplateRoleResponse `json:"members"`
}

// SquadTemplateRoleResponse describes one seat: which role fills it and what the
// leader is told it is for.
type SquadTemplateRoleResponse struct {
	TemplateKey   string `json:"template_key"`
	Title         string `json:"title"`
	Name          string `json:"name"`
	AutonomyLevel string `json:"autonomy_level"`
	AvatarEmoji   string `json:"avatar_emoji"`
	Role          string `json:"role"`
}

// ListSquadTemplates returns the built-in squad roster for the picker.
func (h *Handler) ListSquadTemplates(w http.ResponseWriter, r *http.Request) {
	language := templateLanguageFromRequest(r.URL.Query().Get("language"))
	templates := service.SquadTemplates()
	out := make([]SquadTemplateResponse, 0, len(templates))
	for _, template := range templates {
		out = append(out, squadTemplateToResponse(template, language))
	}
	writeJSON(w, http.StatusOK, map[string]any{"templates": out})
}

func squadTemplateToResponse(template service.SquadTemplate, language string) SquadTemplateResponse {
	resp := SquadTemplateResponse{
		Key:          template.Key,
		Version:      template.Version,
		Name:         template.DefaultName,
		Title:        template.Title(language),
		Description:  template.Description(language),
		AvatarEmoji:  template.AvatarEmoji,
		Instructions: template.Instructions(),
		Members:      make([]SquadTemplateRoleResponse, 0, len(template.Members)),
	}
	if leader, ok := service.AgentRoleTemplateByKey(template.LeaderTemplateKey); ok {
		resp.Leader = squadTemplateRoleToResponse(leader, language, "leader")
	}
	for _, slot := range template.Members {
		role, ok := service.AgentRoleTemplateByKey(slot.TemplateKey)
		if !ok {
			continue
		}
		resp.Members = append(resp.Members, squadTemplateRoleToResponse(role, language, localizedSlotRole(slot, language)))
	}
	return resp
}

func squadTemplateRoleToResponse(role service.AgentRoleTemplate, language, roleNote string) SquadTemplateRoleResponse {
	return SquadTemplateRoleResponse{
		TemplateKey:   role.Key,
		Title:         role.Title(language),
		Name:          role.DefaultName,
		AutonomyLevel: string(role.Autonomy),
		AvatarEmoji:   role.AvatarEmoji,
		Role:          roleNote,
	}
}

func localizedSlotRole(slot service.SquadRoleSlot, language string) string {
	if value, ok := slot.Roles[language]; ok && value != "" {
		return value
	}
	return slot.Roles["en"]
}

// CreateSquadFromTemplateRequest is the staffing input.
type CreateSquadFromTemplateRequest struct {
	TemplateKey string `json:"template_key"`
	// RuntimeID binds every agent this call creates. One runtime for the whole
	// squad in this first version: a per-role runtime picker is a real need, but it
	// belongs to the same screen that lets a team review each role first.
	RuntimeID string `json:"runtime_id"`
	// Name overrides the squad's default name.
	Name string `json:"name"`
	// PermissionMode and InvocationTargets apply to every agent this call creates.
	// Absent means private — the same default a hand-built agent gets, so staffing
	// a squad never silently widens who can run something. A squad the whole team
	// will assign work to wants "public_to" with a workspace target.
	PermissionMode    *string                    `json:"permission_mode"`
	InvocationTargets []AgentInvocationTargetDTO `json:"invocation_targets"`
	Language          string                     `json:"language"`
}

// CreateSquadFromTemplateResponse reports the squad plus which agents were created
// versus reused, because "you already had an Implementer, so it is in this roster
// too" is something the person who clicked the button needs to know.
type CreateSquadFromTemplateResponse struct {
	Squad         SquadResponse `json:"squad"`
	CreatedAgents []string      `json:"created_agent_ids"`
	ReusedAgents  []string      `json:"reused_agent_ids"`
}

// CreateSquadFromTemplate stages the roster and the squad in one transaction.
func (h *Handler) CreateSquadFromTemplate(w http.ResponseWriter, r *http.Request) {
	workspaceID := workspaceIDFromURL(r, "workspaceId")
	member, ok := h.requireWorkspaceMember(w, r, workspaceID, "workspace not found")
	if !ok {
		return
	}

	var req CreateSquadFromTemplateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	template, ok := service.SquadTemplateByKey(strings.TrimSpace(req.TemplateKey))
	if !ok {
		writeError(w, http.StatusBadRequest, "unknown template_key")
		return
	}
	if req.RuntimeID == "" {
		writeError(w, http.StatusBadRequest, "runtime_id is required")
		return
	}
	runtimeUUID, ok := parseUUIDOrBadRequest(w, req.RuntimeID, "runtime_id")
	if !ok {
		return
	}
	wsUUID, ok := parseUUIDOrBadRequest(w, workspaceID, "workspace_id")
	if !ok {
		return
	}

	// The runtime is validated once for the whole squad, before anything is
	// written: every agent this call creates binds to it, so a bad runtime must
	// fail the request rather than half of the roster.
	runtime, err := h.Queries.GetAgentRuntimeForWorkspace(r.Context(), db.GetAgentRuntimeForWorkspaceParams{
		ID:          runtimeUUID,
		WorkspaceID: wsUUID,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, "runtime not found in this workspace")
		return
	}
	if !canUseRuntimeForAgent(member, runtime) {
		writeError(w, http.StatusForbidden, "this runtime is private; only its owner can create agents on it")
		return
	}

	language := templateLanguageFromRequest(req.Language)
	// A default of "private" is passed explicitly rather than left to the zero
	// value: parsePermissionInput returns an EMPTY resolvedPermission when neither
	// input is present, and an empty permission_mode is not NULL, so it would
	// survive the column's COALESCE default and land on the row as an invalid mode.
	legacyVisibility := "private"
	perm, _, permErr := parsePermissionInput(
		wsUUID, req.PermissionMode, req.InvocationTargets,
		req.PermissionMode != nil, len(req.InvocationTargets) > 0, &legacyVisibility,
	)
	if permErr != nil {
		writeError(w, http.StatusBadRequest, permErr.Error())
		return
	}

	squadName := strings.TrimSpace(req.Name)
	if squadName == "" {
		squadName = template.Title(language)
	}

	staged, err := h.provisionSquadTemplate(r.Context(), squadTemplateProvisionInput{
		Template:    template,
		WorkspaceID: wsUUID,
		OwnerID:     member.UserID,
		Runtime:     runtime,
		Permission:  perm,
		Language:    language,
		SquadName:   squadName,
	})
	if err != nil {
		var conflict *agentNameConflictError
		if errors.As(err, &conflict) {
			writeError(w, http.StatusConflict, "an agent named \""+conflict.Name+"\" already exists in this workspace but was not created from this template; rename it or create the squad manually")
			return
		}
		slog.Warn("create squad from template failed",
			append(logger.RequestAttrs(r), "error", err, "template_key", template.Key, "workspace_id", workspaceID)...)
		writeError(w, http.StatusInternalServerError, "failed to create the squad from this template")
		return
	}

	// Reconcile freshly created agents against a live runtime the same way the
	// ordinary create path does, so the roster does not render as offline until the
	// next heartbeat.
	if runtime.Status == "online" {
		for _, created := range staged.CreatedAgents {
			h.TaskService.ReconcileAgentStatus(r.Context(), created.ID)
		}
	}

	resp, err := h.squadToResponseWithPreview(r.Context(), staged.Squad)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load squad member preview")
		return
	}
	actorID := uuidToString(member.UserID)
	h.publish(protocol.EventSquadCreated, workspaceID, "member", actorID, map[string]any{"squad": resp})
	for _, created := range staged.CreatedAgents {
		agentResp := h.agentToResponse(created)
		h.publish(protocol.EventAgentCreated, workspaceID, "member", actorID, map[string]any{"agent": broadcastAgentResponse(agentResp)})
		obsmetrics.RecordEvent(h.Analytics, h.Metrics, analytics.AgentCreated(
			actorID, workspaceID, uuidToString(created.ID),
			runtime.Provider, runtime.RuntimeMode,
			"squad_template:"+template.Key, false,
		))
	}
	obsmetrics.RecordEvent(h.Analytics, h.Metrics, analytics.SquadCreated(
		actorID, workspaceID, uuidToString(staged.Squad.ID), len(template.Members)+1,
	))

	writeJSON(w, http.StatusCreated, CreateSquadFromTemplateResponse{
		Squad:         resp,
		CreatedAgents: staged.CreatedAgentIDs,
		ReusedAgents:  staged.ReusedAgentIDs,
	})
}

// agentNameConflictError marks the one failure a person can act on: the workspace
// already has an agent under a role's default name that did not come from that
// template.
type agentNameConflictError struct{ Name string }

func (e *agentNameConflictError) Error() string {
	return "agent name already taken: " + e.Name
}

// squadTemplateProvisionInput is everything the transaction needs, resolved and
// validated by the handler first.
type squadTemplateProvisionInput struct {
	Template    service.SquadTemplate
	WorkspaceID pgtype.UUID
	OwnerID     pgtype.UUID
	Runtime     db.AgentRuntime
	Permission  resolvedPermission
	Language    string
	SquadName   string
}

// squadTemplateProvisionResult reports what the transaction did.
type squadTemplateProvisionResult struct {
	Squad           db.Squad
	CreatedAgents   []db.Agent
	CreatedAgentIDs []string
	ReusedAgentIDs  []string
}

// provisionSquadTemplate creates the missing roster agents, the squad, and its
// membership — all or nothing.
//
// Serialized per (workspace, template) by an advisory lock. Two people clicking
// "staff this squad" at the same moment would otherwise both miss the reuse lookup
// and race on agent.name's unique index, leaving one caller with a 500 and the
// workspace with a partially staffed squad.
func (h *Handler) provisionSquadTemplate(ctx context.Context, in squadTemplateProvisionInput) (squadTemplateProvisionResult, error) {
	var result squadTemplateProvisionResult

	tx, err := h.TxStarter.Begin(ctx)
	if err != nil {
		return result, err
	}
	defer tx.Rollback(ctx)
	qtx := h.Queries.WithTx(tx)

	if _, err := tx.Exec(ctx,
		"SELECT pg_advisory_xact_lock(hashtextextended($1, 0))",
		"squad-template:"+uuidToString(in.WorkspaceID)+":"+in.Template.Key,
	); err != nil {
		return result, err
	}
	// Role skills are shared across templates, so their lock is workspace-wide and
	// separate from the per-template one above. Taken here, once, rather than inside
	// the per-role loop: acquiring it in a fixed order relative to the template lock
	// is what keeps two concurrent stafflings from deadlocking.
	if err := lockRoleSkillMaterialization(ctx, tx, in.WorkspaceID); err != nil {
		return result, err
	}

	// Leader first: it is both the squad's leader_id and its first member, and the
	// squad row cannot be written without it.
	agentsByRole := make(map[string]db.Agent, len(in.Template.Members)+1)
	for _, templateKey := range in.Template.TemplateKeys() {
		if _, done := agentsByRole[templateKey]; done {
			// A template may name the same role twice (the analyst leads the bug-fix
			// squad and also sits in its roster). One agent, one seat.
			continue
		}
		agent, created, err := h.resolveTemplateAgentInTx(ctx, qtx, in, templateKey)
		if err != nil {
			return result, err
		}
		agentsByRole[templateKey] = agent
		if created {
			result.CreatedAgents = append(result.CreatedAgents, agent)
			result.CreatedAgentIDs = append(result.CreatedAgentIDs, uuidToString(agent.ID))
		} else {
			result.ReusedAgentIDs = append(result.ReusedAgentIDs, uuidToString(agent.ID))
		}
	}

	leader, ok := agentsByRole[in.Template.LeaderTemplateKey]
	if !ok {
		return result, errors.New("squad template leader role could not be provisioned")
	}

	avatar := pgtype.Text{String: agentEmojiAvatarPrefix + in.Template.AvatarEmoji, Valid: true}
	squad, err := qtx.CreateSquadFromTemplate(ctx, db.CreateSquadFromTemplateParams{
		WorkspaceID: in.WorkspaceID,
		Name:        in.SquadName,
		Description: in.Template.Description(in.Language),
		LeaderID:    leader.ID,
		CreatorID:   in.OwnerID,
		AvatarUrl:   avatar,
		// The leader briefing is COPIED here, like the role instructions: the squad's
		// routing policy is workspace-owned from this moment on and no release
		// rewrites it.
		Instructions:    in.Template.Instructions(),
		TemplateKey:     in.Template.Key,
		TemplateVersion: in.Template.Version,
	})
	if err != nil {
		return result, err
	}
	result.Squad = squad

	// Leader carries role "leader" — the same literal CreateSquad writes, which the
	// roster renderer and the member-status endpoint both key off.
	if _, err := qtx.AddSquadMember(ctx, db.AddSquadMemberParams{
		SquadID:    squad.ID,
		MemberType: "agent",
		MemberID:   leader.ID,
		Role:       "leader",
	}); err != nil {
		return result, err
	}
	for _, slot := range in.Template.Members {
		agent, ok := agentsByRole[slot.TemplateKey]
		if !ok {
			continue
		}
		if uuidToString(agent.ID) == uuidToString(leader.ID) {
			// Already seated as leader. Adding it again would violate
			// squad_member's (squad_id, member_type, member_id) uniqueness and the
			// roster deliberately never lists the leader twice.
			continue
		}
		if _, err := qtx.AddSquadMember(ctx, db.AddSquadMemberParams{
			SquadID:    squad.ID,
			MemberType: "agent",
			MemberID:   agent.ID,
			Role:       localizedSlotRole(slot, in.Language),
		}); err != nil {
			return result, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return result, err
	}
	return result, nil
}

// resolveTemplateAgentInTx returns the workspace's agent for a role template,
// creating it when there is none. The bool reports whether it was created.
func (h *Handler) resolveTemplateAgentInTx(
	ctx context.Context,
	qtx *db.Queries,
	in squadTemplateProvisionInput,
	templateKey string,
) (db.Agent, bool, error) {
	existing, err := qtx.GetAgentByWorkspaceAndTemplateKey(ctx, db.GetAgentByWorkspaceAndTemplateKeyParams{
		WorkspaceID: in.WorkspaceID,
		TemplateKey: templateKey,
	})
	if err == nil {
		// Reuse as-is. Notably we do NOT re-apply the template: the workspace may
		// have edited this agent's instructions, lowered its autonomy, or moved it to
		// another runtime, and staffing a second squad is not consent to undo that.
		return existing, false, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return db.Agent{}, false, err
	}

	template, ok := service.AgentRoleTemplateByKey(templateKey)
	if !ok {
		return db.Agent{}, false, errors.New("squad template references unknown role template " + templateKey)
	}

	skillIDs, err := h.materializeRoleSkillsInTx(ctx, qtx, in.WorkspaceID, in.OwnerID, template.RoleSkills)
	if err != nil {
		return db.Agent{}, false, err
	}

	created, err := qtx.CreateAgent(ctx, db.CreateAgentParams{
		WorkspaceID:        in.WorkspaceID,
		Name:               template.DefaultName,
		Description:        template.Description(in.Language),
		Instructions:       template.Instructions(),
		AvatarUrl:          pgtype.Text{String: agentEmojiAvatarPrefix + template.AvatarEmoji, Valid: true},
		RuntimeMode:        in.Runtime.RuntimeMode,
		RuntimeConfig:      []byte("{}"),
		RuntimeID:          in.Runtime.ID,
		Visibility:         in.Permission.legacyVisibility(),
		PermissionMode:     in.Permission.mode,
		MaxConcurrentTasks: template.MaxConcurrentTasks,
		OwnerID:            in.OwnerID,
		CustomEnv:          []byte("{}"),
		CustomArgs:         []byte("[]"),
		TemplateKey:        template.Key,
		TemplateVersion:    template.Version,
		AutonomyLevel:      string(template.Autonomy),
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "agent_workspace_name_unique" {
			// Someone already has an agent under this role's name that did not come
			// from the template — a hand-built "Implementer", most likely. Reusing it
			// would silently adopt an agent with unknown instructions into a squad
			// that assumes the role's contract, so this is a decision for a person.
			return db.Agent{}, false, &agentNameConflictError{Name: template.DefaultName}
		}
		return db.Agent{}, false, err
	}
	if err := replaceInvocationTargetsWithQueries(ctx, qtx, created.ID, in.OwnerID, in.Permission.targets); err != nil {
		return db.Agent{}, false, err
	}
	for _, skillID := range skillIDs {
		if err := qtx.AddAgentSkill(ctx, db.AddAgentSkillParams{AgentID: created.ID, SkillID: skillID}); err != nil {
			return db.Agent{}, false, err
		}
	}
	return created, true, nil
}
