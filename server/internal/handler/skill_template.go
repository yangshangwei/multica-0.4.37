package handler

import (
	"net/http"

	"github.com/multica-ai/multica/server/internal/service"
)

// SkillTemplateResponse is editable source content, without workspace identity
// or official provenance. Copies are created through the ordinary skill API.
type SkillTemplateResponse struct {
	Name        string                      `json:"name"`
	Version     int32                       `json:"version"`
	Description string                      `json:"description"`
	Content     string                      `json:"content"`
	Files       []SkillTemplateFileResponse `json:"files"`
}

type SkillTemplateFileResponse struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// ListSkillTemplates reads the embedded catalog without materializing workspace
// skills. Authentication and workspace membership are enforced by the router.
func (h *Handler) ListSkillTemplates(w http.ResponseWriter, r *http.Request) {
	templates := service.RoleSkillTemplates()
	out := make([]SkillTemplateResponse, 0, len(templates))
	for _, template := range templates {
		files := make([]SkillTemplateFileResponse, 0, len(template.Files))
		for _, file := range template.Files {
			files = append(files, SkillTemplateFileResponse{Path: file.Path, Content: file.Content})
		}
		out = append(out, SkillTemplateResponse{
			Name:        template.Name,
			Version:     template.Version,
			Description: template.Description,
			Content:     template.Content,
			Files:       files,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"templates": out})
}
