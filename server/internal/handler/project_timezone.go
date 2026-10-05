package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

type ProjectPlanningTimezone struct {
	WorkspaceID       string  `json:"workspace_id"`
	PlanningTimezone  *string `json:"planning_timezone"`
	EffectiveTimezone string  `json:"effective_timezone"`
	Configured        bool    `json:"configured"`
}

func projectPlanningTimezone(ws db.Workspace) ProjectPlanningTimezone {
	out := ProjectPlanningTimezone{WorkspaceID: uuidToString(ws.ID), PlanningTimezone: textToPtr(ws.PlanningTimezone), EffectiveTimezone: "UTC", Configured: ws.PlanningTimezone.Valid}
	if ws.PlanningTimezone.Valid {
		out.EffectiveTimezone = ws.PlanningTimezone.String
	}
	return out
}
func (h *Handler) projectWorkspaceScope(r *http.Request) (pgtype.UUID, pgtype.UUID, error) {
	ws, err := util.ParseUUID(workspaceIDFromURL(r, "id"))
	if err != nil {
		return ws, pgtype.UUID{}, projectErr(400, "invalid_request", "invalid workspace id")
	}
	user, err := util.ParseUUID(requestUserID(r))
	if err != nil {
		return ws, user, projectErr(401, "unauthenticated", "user not authenticated")
	}
	// URL scoped routes may have no selected workspace header; when present it must agree.
	if selected := r.Header.Get("X-Workspace-ID"); selected != "" && selected != uuidToString(ws) {
		return ws, user, projectErr(403, "forbidden", "workspace scope mismatch")
	}
	return ws, user, nil
}
func (h *Handler) GetProjectCapabilities(w http.ResponseWriter, r *http.Request) {
	ws, actor, err := h.projectWorkspaceScope(r)
	if err == nil {
		err = h.runProjectTransaction(r.Context(), ws, actor, func(pgx.Tx, *db.Queries) error { return nil })
	}
	if err != nil {
		writeProjectAPIError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"workspace_id": uuidToString(ws), "schema_version": 1, "overview": false, "updates": false, "description_cas": true, "planning_timezone": true})
}
func (h *Handler) GetProjectPlanningTimezone(w http.ResponseWriter, r *http.Request) {
	ws, actor, err := h.projectWorkspaceScope(r)
	var out ProjectPlanningTimezone
	if err == nil {
		err = h.runProjectTransaction(r.Context(), ws, actor, func(_ pgx.Tx, q *db.Queries) error {
			row, e := q.GetWorkspace(r.Context(), ws)
			if e != nil {
				return e
			}
			out = projectPlanningTimezone(row)
			return nil
		})
	}
	if err != nil {
		writeProjectAPIError(w, err)
		return
	}
	writeJSON(w, 200, out)
}
func (h *Handler) UpdateProjectPlanningTimezone(w http.ResponseWriter, r *http.Request) {
	ws, _, err := h.projectWorkspaceScope(r)
	if err != nil {
		writeProjectAPIError(w, err)
		return
	}
	actor, err := h.projectHumanActor(r, uuidToString(ws))
	if err != nil {
		writeProjectAPIError(w, err)
		return
	}
	var raw map[string]json.RawMessage
	if err = json.NewDecoder(r.Body).Decode(&raw); err != nil {
		writeProjectAPIError(w, projectErr(400, "invalid_request", "invalid request body"))
		return
	}
	value, present := raw["planning_timezone"]
	var zone *string
	if !present || json.Unmarshal(value, &zone) != nil {
		writeProjectAPIError(w, projectErr(400, "invalid_request", "planning_timezone must be a string or null"))
		return
	}
	if zone != nil {
		_, e := time.LoadLocation(*zone)
		if e != nil || *zone == "" || *zone == "Local" || strings.HasPrefix(*zone, "/") || strings.Contains(*zone, "\\") {
			writeProjectAPIError(w, &projectAPIError{Status: 422, Code: "validation_failed", Message: "invalid IANA planning timezone", FieldErrors: []map[string]string{{"field": "planning_timezone", "message": "use an IANA timezone name"}}})
			return
		}
	}
	var out ProjectPlanningTimezone
	err = h.runProjectTransaction(r.Context(), ws, actor, func(_ pgx.Tx, q *db.Queries) error {
		member, e := q.GetMemberByUserAndWorkspace(r.Context(), db.GetMemberByUserAndWorkspaceParams{WorkspaceID: ws, UserID: actor})
		if e != nil {
			return e
		}
		if member.Role != "owner" && member.Role != "admin" {
			return projectErr(403, "forbidden", "only owners and administrators may set the planning timezone")
		}
		row, e := q.UpdateWorkspacePlanningTimezone(r.Context(), db.UpdateWorkspacePlanningTimezoneParams{ID: ws, PlanningTimezone: ptrToText(zone)})
		if e != nil {
			if errors.Is(e, pgx.ErrNoRows) {
				return projectErr(404, "project_not_found", "workspace not found")
			}
			return e
		}
		out = projectPlanningTimezone(row)
		return nil
	})
	if err != nil {
		writeProjectAPIError(w, err)
		return
	}
	h.publish(protocol.EventWorkspaceUpdated, uuidToString(ws), "member", uuidToString(actor), map[string]any{"workspace_id": uuidToString(ws), "planning_timezone": out.PlanningTimezone})
	writeJSON(w, 200, out)
}
