package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/analytics"
	"github.com/multica-ai/multica/server/internal/logger"
	obsmetrics "github.com/multica-ai/multica/server/internal/metrics"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// Creating a standing automation from a built-in autopilot template.
//
// The result is an ordinary autopilot plus an ordinary schedule trigger. The
// scheduler, the dispatch path and every edit surface treat them exactly as they
// treat hand-built rows, because they ARE the same rows written by the same
// in-transaction bodies (createAutopilotInTx / createScheduleTriggerInTx). What
// the template contributes is content and provenance: the prompt, the cadence,
// the execution mode, and the template_key / template_version stamped on the
// autopilot so a later release can tell where the copy came from.
//
// The one thing this endpoint adds over calling POST /api/autopilots and then
// POST /api/autopilots/{id}/triggers is atomicity: a template produces an
// automation that either exists with its schedule or does not exist at all,
// instead of leaving a client to clean up an autopilot that never got a trigger.

// AutopilotTemplateResponse is one entry in the template picker.
//
// Prompt is included in full for the same reason the role-template picker ships
// Instructions: a person is about to adopt a prompt into their workspace and
// should be able to read it first, and shipping the exact text is what makes the
// copy honest — this is byte-for-byte what lands on the autopilot row.
type AutopilotTemplateResponse struct {
	Key     string `json:"key"`
	Version int32  `json:"version"`
	// Category is the stable slug; CategoryLabel is the localized copy. They are
	// separate so a client can group by category without keying off translated
	// text.
	Category       string `json:"category"`
	CategoryLabel  string `json:"category_label"`
	Title          string `json:"title"`
	Description    string `json:"description"`
	CronExpression string `json:"cron_expression"`
	ExecutionMode  string `json:"execution_mode"`
	AvatarEmoji    string `json:"avatar_emoji"`
	Prompt         string `json:"prompt"`
}

// ListAutopilotTemplates returns the built-in roster for the picker.
//
// Read-only and workspace-independent: templates ship with the binary, so this
// answers the same for every workspace on this server. It still lives behind the
// workspace-scoped API group so the client needs no special call shape.
func (h *Handler) ListAutopilotTemplates(w http.ResponseWriter, r *http.Request) {
	language := templateLanguageFromRequest(r.URL.Query().Get("language"))
	templates := service.AutopilotTemplates()
	out := make([]AutopilotTemplateResponse, 0, len(templates))
	for _, template := range templates {
		out = append(out, autopilotTemplateToResponse(template, language))
	}
	writeJSON(w, http.StatusOK, map[string]any{"templates": out})
}

func autopilotTemplateToResponse(template service.AutopilotTemplate, language string) AutopilotTemplateResponse {
	return AutopilotTemplateResponse{
		Key:            template.Key,
		Version:        template.Version,
		Category:       template.Category,
		CategoryLabel:  template.CategoryLabel(language),
		Title:          template.Title(language),
		Description:    template.Description(language),
		CronExpression: template.CronExpression,
		ExecutionMode:  template.ExecutionMode,
		AvatarEmoji:    template.AvatarEmoji,
		Prompt:         template.Prompt(),
	}
}

// CreateAutopilotFromTemplateRequest is the creation input. Everything the
// template decides is absent from it on purpose — prompt, cadence and execution
// mode come from the server, so a client cannot claim a template's provenance
// while supplying its own prompt. The workspace can edit all three afterwards
// through the ordinary autopilot endpoints; template_key then records where the
// copy started, not what it still says.
type CreateAutopilotFromTemplateRequest struct {
	TemplateKey string `json:"template_key"`
	// AssigneeID is required: a template cannot know which agent a workspace
	// owns, which is why adopting one is a two-step flow rather than one click.
	AssigneeID string `json:"assignee_id"`
	// AssigneeType is optional and defaults to "agent", matching
	// CreateAutopilotRequest.
	AssigneeType *string `json:"assignee_type"`
	ProjectID    *string `json:"project_id"`
	// Timezone is the schedule trigger's timezone. Absent means UTC, the same
	// fallback the scheduler applies to a trigger that has none.
	Timezone *string `json:"timezone"`
	// Language selects the localized title stored on the row. The prompt stays
	// English, matching every other agent-harness text in this product.
	Language    string            `json:"language"`
	Subscribers []SubscriberInput `json:"subscribers"`
}

// CreateAutopilotFromTemplateResponse returns both rows the call wrote. The
// trigger is not a detail the caller can look up separately without a second
// round trip, and the picker wants to show the next run time it carries.
type CreateAutopilotFromTemplateResponse struct {
	Autopilot AutopilotResponse        `json:"autopilot"`
	Trigger   AutopilotTriggerResponse `json:"trigger"`
}

// CreateAutopilotFromTemplate provisions an autopilot and its schedule trigger
// from a built-in template, in ONE transaction.
//
// The transaction is the point of the endpoint. It reproduces, in order, every
// substantive side effect the two hand-built endpoints have: the autonomy gate,
// the subscriber locks, the assignee readiness check, the autopilot row, its
// rule version, the subscribers, the trigger and the trigger's rule version.
// Dropping any of them would produce a row that looks like an autopilot and
// behaves differently at dispatch time — an autopilot with no rule version, for
// instance, has no accountable human when it fires.
func (h *Handler) CreateAutopilotFromTemplate(w http.ResponseWriter, r *http.Request) {
	workspaceID := h.resolveWorkspaceID(r)

	// Standing automation outlives the turn that created it and can wake other
	// agents on a schedule. That is coordination, not contribution, so the same
	// gate CreateAutopilot carries applies here — otherwise an agent refused by
	// POST /api/autopilots could stand one up through the template instead.
	if !h.requireAgentAutonomy(w, r, workspaceID, service.AutonomyCoordinator, "create automation") {
		return
	}

	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	// The trigger's created_by is the immutable authorization principal every
	// later dispatch acts as (MUL-6951), so it follows the delegation chain
	// rather than the credential. published_by below keeps the acting identity.
	// Resolved before the transaction opens: it writes its own 401/403.
	principalID, ok := h.requireAutomationPrincipal(w, r, workspaceID)
	if !ok {
		return
	}

	var req CreateAutopilotFromTemplateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	template, ok := service.AutopilotTemplateByKey(strings.TrimSpace(req.TemplateKey))
	if !ok || !template.Listed {
		// An unlisted template is one that ships dark. Offered here it would let
		// a client adopt a prompt the picker deliberately does not show.
		writeError(w, http.StatusBadRequest, "unknown template_key")
		return
	}
	if req.AssigneeID == "" {
		writeError(w, http.StatusBadRequest, "assignee_id is required")
		return
	}
	assigneeType := "agent"
	if req.AssigneeType != nil && *req.AssigneeType != "" {
		assigneeType = *req.AssigneeType
	}
	if !isValidAutopilotAssigneeType(assigneeType) {
		writeError(w, http.StatusBadRequest, "assignee_type must be agent or squad")
		return
	}

	assigneeUUID, ok := parseUUIDOrBadRequest(w, req.AssigneeID, "assignee_id")
	if !ok {
		return
	}
	wsUUID, ok := parseUUIDOrBadRequest(w, workspaceID, "workspace id")
	if !ok {
		return
	}
	projectID, ok := h.parseAutopilotProjectID(w, r, req.ProjectID, wsUUID)
	if !ok {
		return
	}

	timezone := service.DefaultAutopilotTriggerTimezone
	if req.Timezone != nil && *req.Timezone != "" {
		if err := service.ValidateTimezone(*req.Timezone); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		timezone = *req.Timezone
	}
	// The cron is the server's own, and the timezone has just been validated, so
	// a failure here is a malformed template rather than a bad request — the
	// registry test pins that every template's cron parses.
	nextRun, err := computeNextRun(template.CronExpression, timezone)
	if err != nil {
		slog.Error("create autopilot from template: template cron did not parse",
			append(logger.RequestAttrs(r), "error", err, "template_key", template.Key)...)
		writeError(w, http.StatusInternalServerError, "failed to create autopilot")
		return
	}

	// Parse before insert so a malformed payload doesn't open a transaction.
	subscribers, ok := parseAutopilotSubscribers(w, req.Subscribers)
	if !ok {
		return
	}

	language := templateLanguageFromRequest(req.Language)
	creatorID := parseUUID(userID)

	tx, err := h.TxStarter.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create autopilot")
		return
	}
	defer tx.Rollback(r.Context())
	qtx := h.Queries.WithTx(tx)

	// This must be the first lock family in the transaction, for the reason
	// CreateAutopilot documents: member revocation takes the same
	// per-(workspace, user) locks before deleting member rows, so whichever
	// commits first, the other sees a consistent membership.
	if !h.lockAndValidateAutopilotSubscribers(w, r, qtx, subscribers, wsUUID) {
		return
	}
	// Save-time readiness validation belongs in the same transaction as the
	// insert: the assignment lock serializes this path with runtime teardown, so
	// an active autopilot cannot slip in after teardown's pause sweep.
	if !h.validateAutopilotAssigneeForSave(w, r, qtx, assigneeType, assigneeUUID, wsUUID, true) {
		return
	}

	autopilot, err := h.createAutopilotInTx(r.Context(), qtx, createAutopilotInTxInput{
		WorkspaceID:  wsUUID,
		Title:        template.Title(language),
		AssigneeType: assigneeType,
		AssigneeID:   assigneeUUID,
		// The autopilot has no separate prompt column: description IS the brief.
		// buildIssueDescription writes it into the issue a create_issue run
		// opens, and a run_only dispatch hands the same text to the agent. The
		// localized card Description is picker copy and must never land here.
		Description:     pgtype.Text{String: template.Prompt(), Valid: true},
		ExecutionMode:   template.ExecutionMode,
		CreatedByID:     creatorID,
		ProjectID:       projectID,
		PublishedByID:   creatorID,
		Subscribers:     subscribers,
		TemplateKey:     template.Key,
		TemplateVersion: template.Version,
	})
	if err != nil {
		if errors.Is(err, errAutopilotSubscriberInsert) {
			writeError(w, http.StatusInternalServerError, "failed to add autopilot subscriber")
			return
		}
		slog.Warn("create autopilot from template: autopilot insert failed",
			append(logger.RequestAttrs(r), "error", err, "template_key", template.Key)...)
		writeError(w, http.StatusInternalServerError, "failed to create autopilot")
		return
	}

	trigger, err := h.createScheduleTriggerInTx(r.Context(), qtx, autopilot, createScheduleTriggerInTxInput{
		CronExpression: pgtype.Text{String: template.CronExpression, Valid: true},
		Timezone:       pgtype.Text{String: timezone, Valid: true},
		NextRunAt:      pgtype.Timestamptz{Time: nextRun, Valid: true},
		PublishedByID:  creatorID,
		PrincipalID:    principalID,
	})
	if err != nil {
		// The autopilot written above goes with it. That is the whole reason this
		// endpoint exists: no half-created automation for a client to clean up.
		slog.Warn("create autopilot from template: trigger insert failed",
			append(logger.RequestAttrs(r), "error", err, "template_key", template.Key)...)
		writeError(w, http.StatusInternalServerError, "failed to create autopilot")
		return
	}

	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create autopilot")
		return
	}

	subs, err := h.Queries.ListAutopilotSubscribers(r.Context(), autopilot.ID)
	if err != nil {
		subs = nil
	}
	autopilotResp := autopilotToResponse(autopilot, subs)
	triggerResp := h.triggerToResponse(trigger)

	// Both events the hand-built pair would have published, in the same order a
	// client that called the two endpoints in sequence would have seen them.
	h.publish(protocol.EventAutopilotCreated, workspaceID, "member", userID, map[string]any{"autopilot": autopilotResp})
	h.publish(protocol.EventAutopilotUpdated, workspaceID, "member", userID, map[string]any{
		"autopilot_id": uuidToString(autopilot.ID),
		"trigger":      triggerResp,
	})
	obsmetrics.RecordEvent(h.Analytics, h.Metrics, analytics.AutopilotCreated(
		userID,
		workspaceID,
		uuidToString(autopilot.ID),
		// Trigger kind stands in for cadence, the same proxy the run events use
		// (see the cadence note in metrics/labels_pr3.go). Unlike the manual
		// create path this one always lands a schedule.
		"schedule",
		"schedule",
	))

	writeJSON(w, http.StatusCreated, CreateAutopilotFromTemplateResponse{
		Autopilot: autopilotResp,
		Trigger:   triggerResp,
	})
}
