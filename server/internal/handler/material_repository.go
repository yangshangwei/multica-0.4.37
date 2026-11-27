package handler

import (
	"github.com/go-chi/chi/v5"
	"github.com/multica-ai/multica/server/internal/material"
	"net/http"
)

func (h *Handler) GetMaterialInitialization(w http.ResponseWriter, r *http.Request) {
	a, ok := h.materialAccess(w, r)
	if !ok {
		return
	}
	out, err := h.materialService().Initialization(r.Context(), a)
	materialRespond(w, out, err)
}
func (h *Handler) CommandMaterialInitialization(w http.ResponseWriter, r *http.Request) {
	a, ok := h.materialAccess(w, r)
	if !ok {
		return
	}
	var in material.InitializationCommand
	if !decodeMaterial(w, r, &in) {
		return
	}
	out, err := h.materialService().InitializationCommand(r.Context(), a, chi.URLParam(r, "initializationCommand"), in)
	materialRespond(w, out, err)
}
func (h *Handler) DecideMaterialInitialization(w http.ResponseWriter, r *http.Request) {
	a, ok := h.materialAccess(w, r)
	if !ok {
		return
	}
	var in material.InitializationDecisionInput
	if !decodeMaterial(w, r, &in) {
		return
	}
	out, err := h.materialService().InitializationDecide(r.Context(), a, in)
	materialRespond(w, out, err)
}
