package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/logger"
	"github.com/multica-ai/multica/server/internal/service"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// Creating an agent from a built-in role template.
//
// The result is an ordinary workspace agent. Everything about it — Access, runtime
// binding, task lifecycle, archiving — behaves exactly as a hand-built agent does,
// because it is created through the same code path
// (createAgentFromRequest). What the template contributes is the content: the
// role's instructions, its default autonomy level, and the role skills it needs.

// AgentRoleTemplateResponse is one entry in the template picker.
//
// Instructions are included in full: a person is about to adopt a prompt into
// their workspace and should be able to read it first. It is also what makes the
// copy honest — the text shown here is byte-for-byte what lands on the row.
type AgentRoleTemplateResponse struct {
	Key     string `json:"key"`
	Version int32  `json:"version"`
	// Name is the default agent name; Title is the localized role label. They are
	// separate because the name is stored (and renameable) while the title is
	// picker copy that follows the reader's language.
	Name               string   `json:"name"`
	Title              string   `json:"title"`
	Description        string   `json:"description"`
	AutonomyLevel      string   `json:"autonomy_level"`
	AvatarEmoji        string   `json:"avatar_emoji"`
	MaxConcurrentTasks int32    `json:"max_concurrent_tasks"`
	SkillNames         []string `json:"skill_names"`
	Instructions       string   `json:"instructions"`
}

// ListAgentRoleTemplates returns the built-in roster for the picker.
//
// Read-only and workspace-independent: templates ship with the binary, so this
// answers the same for every workspace on this server. It still lives behind the
// workspace-scoped API group so the client needs no special call shape.
func (h *Handler) ListAgentRoleTemplates(w http.ResponseWriter, r *http.Request) {
	language := templateLanguageFromRequest(r.URL.Query().Get("language"))
	templates := service.AgentRoleTemplates()
	out := make([]AgentRoleTemplateResponse, 0, len(templates))
	for _, template := range templates {
		out = append(out, agentRoleTemplateToResponse(template, language))
	}
	writeJSON(w, http.StatusOK, map[string]any{"templates": out})
}

func agentRoleTemplateToResponse(template service.AgentRoleTemplate, language string) AgentRoleTemplateResponse {
	skills := template.RoleSkills
	if skills == nil {
		skills = []string{}
	}
	return AgentRoleTemplateResponse{
		Key:                template.Key,
		Version:            template.Version,
		Name:               template.DefaultName,
		Title:              template.Title(language),
		Description:        template.Description(language),
		AutonomyLevel:      string(template.Autonomy),
		AvatarEmoji:        template.AvatarEmoji,
		MaxConcurrentTasks: template.MaxConcurrentTasks,
		SkillNames:         skills,
		Instructions:       template.Instructions(),
	}
}

// templateLanguageFromRequest normalises the requested locale. An unknown or
// absent value falls back to English rather than failing: picker copy is not worth
// a 400, and the template registry falls back the same way.
func templateLanguageFromRequest(language string) string {
	trimmed := strings.TrimSpace(language)
	if service.IsSupportedTemplateLanguage(trimmed) {
		return trimmed
	}
	return "en"
}

// CreateAgentFromTemplateRequest is the creation input. Everything the template
// decides is absent from it on purpose — instructions, autonomy level and skills
// come from the server, so a client cannot claim a role's provenance while
// supplying its own prompt.
type CreateAgentFromTemplateRequest struct {
	TemplateKey string `json:"template_key"`
	RuntimeID   string `json:"runtime_id"`
	// Name overrides the template's default. Useful when a workspace wants two
	// implementers, or names roles in its own language.
	Name          string `json:"name"`
	Model         string `json:"model"`
	ThinkingLevel string `json:"thinking_level"`
	ServiceTier   string `json:"service_tier"`
	// PermissionMode and InvocationTargets are passed through to the ordinary
	// create path unchanged. Absent means private, the same default a blank agent
	// gets.
	PermissionMode    *string                    `json:"permission_mode"`
	InvocationTargets []AgentInvocationTargetDTO `json:"invocation_targets"`
	// Language selects the localized description stored on the row. Instructions
	// stay English, matching every other agent-harness text in this product.
	Language string `json:"language"`
}

// CreateAgentFromTemplate provisions a workspace agent from a built-in role
// template.
func (h *Handler) CreateAgentFromTemplate(w http.ResponseWriter, r *http.Request) {
	workspaceID := h.resolveWorkspaceID(r)
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}

	var req CreateAgentFromTemplateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	template, ok := service.AgentRoleTemplateByKey(strings.TrimSpace(req.TemplateKey))
	if !ok || !template.Listed {
		// Unlisted keys are the squad-leader definitions. They are provisioned by
		// the squad-template flow, which also creates the roster that makes a
		// coordinator meaningful; offered here they would produce a leader with
		// nobody to lead.
		writeError(w, http.StatusBadRequest, "unknown template_key")
		return
	}
	if req.RuntimeID == "" {
		writeError(w, http.StatusBadRequest, "runtime_id is required")
		return
	}

	// Staffing a role is a coordination decision, so it carries the same
	// requirement CreateSquad does.
	if !h.requireAgentAutonomy(w, r, workspaceID, service.AutonomyCoordinator, "create agents") {
		return
	}
	// And it may not hand out a level above the caller's own: this route decides
	// autonomy_level server-side, which is what makes it an escalation path that
	// UpdateAgent's guard cannot see.
	if !h.requireAgentMayGrantAutonomy(w, r, workspaceID, template.Autonomy) {
		return
	}

	wsUUID, ok := parseUUIDOrBadRequest(w, workspaceID, "workspace id")
	if !ok {
		return
	}

	// Skills are materialized BEFORE the agent so the ids can ride along in the
	// same create transaction the ordinary path already uses — an agent never
	// becomes visible with half its role attached.
	//
	// A create that fails afterwards (a name conflict, say) therefore leaves the
	// skills behind. That is the right way round: a role skill is a workspace
	// asset, materialization is idempotent, and the retry reuses what is already
	// there. Rolling it back would delete a row another agent may have just
	// attached.
	skillIDs, err := h.materializeRoleSkills(r.Context(), wsUUID, parseUUID(userID), template.RoleSkills)
	if err != nil {
		slog.Warn("create agent from template: materialize role skills failed",
			append(logger.RequestAttrs(r), "error", err, "template_key", template.Key)...)
		writeError(w, http.StatusInternalServerError, "failed to prepare the role's skills")
		return
	}

	language := templateLanguageFromRequest(req.Language)
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = template.DefaultName
	}
	avatar := agentEmojiAvatarPrefix + template.AvatarEmoji

	create := CreateAgentRequest{
		Name:               name,
		Description:        template.Description(language),
		Instructions:       template.Instructions(),
		AvatarURL:          &avatar,
		RuntimeID:          strings.TrimSpace(req.RuntimeID),
		Visibility:         "private",
		PermissionMode:     req.PermissionMode,
		InvocationTargets:  req.InvocationTargets,
		MaxConcurrentTasks: template.MaxConcurrentTasks,
		Model:              strings.TrimSpace(req.Model),
		ThinkingLevel:      strings.TrimSpace(req.ThinkingLevel),
		ServiceTier:        strings.TrimSpace(req.ServiceTier),
		SkillIDs:           skillIDs,
		// Creation-source attribution for the `agent_created` analytics event, so a
		// template's adoption is measurable separately from the AI builder's.
		Template: "role_template:" + template.Key,
	}

	h.createAgentFromRequest(w, r, create, agentTemplateRawFields(req, template), agentTemplateProvenance{
		Key:      template.Key,
		Version:  template.Version,
		Autonomy: string(template.Autonomy),
	})
}

// agentTemplateRawFields synthesizes the "which keys did the client send" map that
// createAgentFromRequest consults.
//
// Three fields are read from it rather than from the struct, because for each one
// "absent" and "zero" mean different things. Building it here keeps the shared
// create path unaware of who called it.
func agentTemplateRawFields(req CreateAgentFromTemplateRequest, template service.AgentRoleTemplate) map[string]json.RawMessage {
	fields := map[string]json.RawMessage{
		// Present so the template's own cap is honoured instead of being replaced by
		// the global default.
		"max_concurrent_tasks": json.RawMessage(strconv.FormatInt(int64(template.MaxConcurrentTasks), 10)),
	}
	if req.PermissionMode != nil && len(req.InvocationTargets) > 0 {
		// Only signals presence; the values themselves are read from the struct.
		fields["invocation_targets"] = json.RawMessage("[]")
	}
	return fields
}

// materializeRoleSkills turns the template's role skills into workspace skill rows
// and returns their ids, in the template's order.
//
// Opens its own transaction so the single-agent template flow takes the same
// per-workspace lock the squad flow does. See materializeRoleSkillsInTx for the
// reuse rule that matters.
func (h *Handler) materializeRoleSkills(ctx context.Context, wsUUID, creatorID pgtype.UUID, names []string) ([]string, error) {
	if len(names) == 0 {
		return []string{}, nil
	}
	tx, err := h.TxStarter.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	qtx := h.Queries.WithTx(tx)
	if err := lockRoleSkillMaterialization(ctx, tx, wsUUID); err != nil {
		return nil, err
	}
	skillIDs, err := h.materializeRoleSkillsInTx(ctx, qtx, wsUUID, creatorID, names)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(skillIDs))
	for _, id := range skillIDs {
		ids = append(ids, uuidToString(id))
	}
	return ids, nil
}

// materializeRoleSkillsInTx is the reuse-or-create body, on a caller-owned
// transaction.
//
// Reuse-or-create, keyed on name:
//
//   - A workspace that already has a skill by that name gets that skill, exactly as
//     it stands. This is the rule that keeps a workspace admin's edits safe — there
//     is no update path here at all, so no release can rewrite tuned prose, and a
//     second agent of the same role attaches the same skill rather than a divergent
//     copy.
//   - Otherwise the embedded copy is written, tagged with config.origin so its
//     provenance and version stay visible. The origin type is deliberately one
//     `refreshableOriginSource` does not recognise: a role skill is upgraded through
//     its template, never re-fetched from a URL.
//
// Callers must hold the lock from lockRoleSkillMaterialization. Inside a
// transaction a unique-violation cannot be recovered from — the whole transaction
// is poisoned — so the lock, not a retry, is what makes concurrent staffing safe.
func (h *Handler) materializeRoleSkillsInTx(
	ctx context.Context,
	qtx *db.Queries,
	wsUUID, creatorID pgtype.UUID,
	names []string,
) ([]pgtype.UUID, error) {
	ids := make([]pgtype.UUID, 0, len(names))
	for _, name := range names {
		existing, err := qtx.GetSkillByWorkspaceAndName(ctx, db.GetSkillByWorkspaceAndNameParams{
			WorkspaceID: wsUUID,
			Name:        name,
		})
		if err == nil {
			ids = append(ids, existing.ID)
			continue
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}

		roleSkill, ok := service.RoleSkillTemplateByName(name)
		if !ok {
			// A template naming a skill this binary does not ship is a build-time
			// mistake, not a user error. Fail rather than create an agent whose role is
			// quietly missing half its method. A registry test pins that every template
			// names a skill that exists.
			return nil, fmt.Errorf("role skill %q is not embedded in this server", name)
		}
		files := make([]CreateSkillFileRequest, 0, len(roleSkill.Files))
		for _, file := range roleSkill.Files {
			files = append(files, CreateSkillFileRequest{Path: file.Path, Content: file.Content})
		}
		created, err := createSkillWithFilesInTx(ctx, qtx, skillCreateInput{
			WorkspaceID: wsUUID,
			CreatorID:   creatorID,
			Name:        roleSkill.Name,
			Description: roleSkill.Description,
			Content:     roleSkill.Content,
			Config: map[string]any{
				"origin": map[string]any{
					"type":    roleSkillOriginType,
					"name":    roleSkill.Name,
					"version": roleSkill.Version,
				},
			},
			Files: files,
		})
		if err != nil {
			return nil, err
		}
		ids = append(ids, parseUUID(created.ID))
	}
	return ids, nil
}

// lockRoleSkillMaterialization serializes role-skill creation within a workspace.
//
// Needed because different templates can name the same role skill — every built-in
// squad lead carries multica-requirement-clarification, and the eight rosters draw
// on the same eight working roles. Staffing two of them at once would otherwise have
// both transactions miss the reuse lookup and collide on skill's unique
// (workspace_id, name) index. The squad-template lock does not cover this — it is
// keyed per template, and these are different templates.
func lockRoleSkillMaterialization(ctx context.Context, tx pgx.Tx, wsUUID pgtype.UUID) error {
	_, err := tx.Exec(ctx,
		"SELECT pg_advisory_xact_lock(hashtextextended($1, 0))",
		"role-skill:"+uuidToString(wsUUID),
	)
	return err
}

// roleSkillOriginType tags a materialized role skill's provenance in skill.config.
// Unrecognised by refreshableOriginSource on purpose — "update from source" must
// not offer to re-download a skill that came from the server binary.
const roleSkillOriginType = "builtin_role_skill"
