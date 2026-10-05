package handler

import "net/http"

// McpServerTemplateResponse exposes catalog metadata. Builtin config remains
// public for installed clients; deployment config and all input targets stay
// server-side and are resolved only when a human admin adopts the template.
type McpServerTemplateResponse struct {
	Source           string                     `json:"source"`
	Transport        string                     `json:"transport"`
	Version          string                     `json:"version"`
	Category         string                     `json:"category"`
	Requirements     []string                   `json:"requirements"`
	DocumentationURL string                     `json:"documentation_url"`
	Key              string                     `json:"key"`
	Title            string                     `json:"title"`
	Description      string                     `json:"description"`
	Config           map[string]any             `json:"config,omitempty"`
	Inputs           []McpTemplateInputResponse `json:"inputs"`
}

// McpTemplateInputResponse exposes display metadata, never config destinations or saved values.
type McpTemplateInputResponse struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
	Secret      bool   `json:"secret"`
}

// ListMcpServerTemplates returns the deployment-wide catalog in the requested
// language. It lives behind the workspace-scoped API group so the client needs
// no special call shape, and it
// is member-visible for the same reason as the workspace MCP library — an agent
// owner needs to see what they can add.
func (h *Handler) ListMcpServerTemplates(w http.ResponseWriter, r *http.Request) {
	language := templateLanguageFromRequest(r.URL.Query().Get("language"))
	templates, err := h.McpCatalog.List()
	if err != nil {
		writeMcpTemplateError(w, err)
		return
	}
	out := make([]McpServerTemplateResponse, 0, len(templates))
	for _, template := range templates {
		inputs := make([]McpTemplateInputResponse, 0, len(template.Inputs))
		for _, input := range template.Inputs {
			inputs = append(inputs, McpTemplateInputResponse{
				Key: input.Key, Label: input.Label(language), Description: input.Description(language),
				Required: input.Required, Secret: input.Secret,
			})
		}
		response := McpServerTemplateResponse{
			Source:           template.Source,
			Transport:        template.Transport,
			Key:              template.Key,
			Version:          template.Version,
			Category:         template.Category,
			Requirements:     template.RequirementLabels(language),
			DocumentationURL: template.DocumentationURL,
			Title:            template.Title(language),
			Description:      template.Description(language),
			Inputs:           inputs,
		}
		if template.Source == "builtin" {
			response.Config = template.Config
		}
		out = append(out, response)
	}
	writeJSON(w, http.StatusOK, map[string]any{"templates": out})
}
