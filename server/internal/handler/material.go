package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/multica-ai/multica/server/internal/material"
	"github.com/multica-ai/multica/server/internal/service"
)

func (h *Handler) materialService() *service.MaterialService {
	s := service.NewMaterialService(h.Queries, h.TxStarter, h.IssueService, h.TaskService)
	return s
}
func (h *Handler) RunMaterialWorker(ctx context.Context) { h.materialService().Run(ctx) }
func materialFailure(w http.ResponseWriter, err error) {
	status, code := 503, "material_operation_unavailable"
	var domain service.MaterialError
	var pgerr *pgconn.PgError
	switch {
	case errors.As(err, &domain):
		status, code = domain.Status, domain.Code
	case errors.Is(err, pgx.ErrNoRows):
		status, code = 404, "material_not_found"
	case errors.As(err, &pgerr) && (pgerr.Code == "40001" || pgerr.Code == "40P01" || pgerr.Code == "55P03"):
		status, code = 409, "material_busy"
	}
	writeJSON(w, status, map[string]any{"error": code, "code": code})
}
func (h *Handler) materialAccess(w http.ResponseWriter, r *http.Request) (service.MaterialAccess, bool) {
	w.Header().Set("Cache-Control", "no-store")
	var access service.MaterialAccess
	principal, _, _, err := artifactPrincipal(r)
	if err != nil {
		materialFailure(w, service.MaterialError{Status: 403, Code: "material_permission_denied"})
		return access, false
	}
	ws, ok := parseUUIDOrBadRequest(w, h.resolveWorkspaceID(r), "workspace_id")
	if !ok {
		return access, false
	}
	project, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "project_id")
	if !ok {
		return access, false
	}
	access = service.MaterialAccess{WorkspaceID: ws, ProjectID: project, Credential: artifactCredentialSnapshot(r.Context(), principal)}
	return access, true
}
func decodeMaterial(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 2*material.MaxContent+128*1024)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		writeError(w, 400, "invalid_material_request")
		return false
	}
	var trailing any
	if err := dec.Decode(&trailing); err != io.EOF {
		writeError(w, 400, "invalid_material_request")
		return false
	}
	return true
}
func materialRespond(w http.ResponseWriter, out any, err error) {
	if err != nil {
		materialFailure(w, err)
		return
	}
	writeJSON(w, 200, out)
}
func (h *Handler) GetMaterialSettings(w http.ResponseWriter, r *http.Request) {
	a, ok := h.materialAccess(w, r)
	if !ok {
		return
	}
	out, e := h.materialService().Settings(r.Context(), a)
	materialRespond(w, out, e)
}
func (h *Handler) ConfigureMaterials(w http.ResponseWriter, r *http.Request) {
	a, ok := h.materialAccess(w, r)
	if !ok {
		return
	}
	var in material.SettingsInput
	if !decodeMaterial(w, r, &in) {
		return
	}
	out, e := h.materialService().Configure(r.Context(), a, in)
	materialRespond(w, out, e)
}
func (h *Handler) ListMaterials(w http.ResponseWriter, r *http.Request) {
	a, ok := h.materialAccess(w, r)
	if !ok {
		return
	}
	out, e := h.materialService().List(r.Context(), a)
	materialRespond(w, out, e)
}
func (h *Handler) CreateMaterial(w http.ResponseWriter, r *http.Request) {
	a, ok := h.materialAccess(w, r)
	if !ok {
		return
	}
	var in material.CreateInput
	if !decodeMaterial(w, r, &in) {
		return
	}
	out, e := h.materialService().Create(r.Context(), a, in)
	materialRespond(w, out, e)
}
func (h *Handler) GetMaterial(w http.ResponseWriter, r *http.Request) {
	a, ok := h.materialAccess(w, r)
	if !ok {
		return
	}
	out, e := h.materialService().Detail(r.Context(), a, chi.URLParam(r, "materialID"))
	materialRespond(w, out, e)
}
func (h *Handler) ReviseMaterial(w http.ResponseWriter, r *http.Request) {
	a, ok := h.materialAccess(w, r)
	if !ok {
		return
	}
	var in material.RevisionInput
	if !decodeMaterial(w, r, &in) {
		return
	}
	out, e := h.materialService().Revise(r.Context(), a, chi.URLParam(r, "materialID"), in)
	materialRespond(w, out, e)
}
func (h *Handler) DecideMaterial(w http.ResponseWriter, r *http.Request) {
	a, ok := h.materialAccess(w, r)
	if !ok {
		return
	}
	var in material.DecisionInput
	if !decodeMaterial(w, r, &in) {
		return
	}
	out, e := h.materialService().Decide(r.Context(), a, chi.URLParam(r, "materialID"), in)
	materialRespond(w, out, e)
}
func (h *Handler) CommentMaterial(w http.ResponseWriter, r *http.Request) {
	a, ok := h.materialAccess(w, r)
	if !ok {
		return
	}
	var in material.CommentInput
	if !decodeMaterial(w, r, &in) {
		return
	}
	out, e := h.materialService().Comment(r.Context(), a, chi.URLParam(r, "materialID"), in)
	materialRespond(w, out, e)
}
func (h *Handler) RetryMaterial(w http.ResponseWriter, r *http.Request) {
	h.materialCommand(w, r, "retry")
}
func (h *Handler) StartMaterialPlan(w http.ResponseWriter, r *http.Request) {
	h.materialCommand(w, r, "start-plan")
}
func (h *Handler) materialCommand(w http.ResponseWriter, r *http.Request, command string) {
	a, ok := h.materialAccess(w, r)
	if !ok {
		return
	}
	var in material.CommandInput
	if !decodeMaterial(w, r, &in) {
		return
	}
	out, e := h.materialService().Command(r.Context(), a, chi.URLParam(r, "materialID"), command, in)
	materialRespond(w, out, e)
}
