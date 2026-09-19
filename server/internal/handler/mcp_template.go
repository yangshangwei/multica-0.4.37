package handler

import (
	"net/http"

	"github.com/multica-ai/multica/server/internal/service"
)

// McpServerTemplateResponse is one entry in the built-in MCP catalog.
//
// Unlike WorkspaceMcpServerResponse (which is intentionally write-only and
// never carries url / command / args / headers / env), Config is served in
// full: these templates are public, credential-free content the workspace has
// not yet adopted, so there is nothing to protect. The keyless invariant is
// enforced by builtin_mcp_templates_test.go.
//
// No transport is sent: the client derives it from Config via mcpTransport, so
// echoing a server-computed value would just be a second source of truth.
type McpServerTemplateResponse struct {
	Key         string         `json:"key"`
	Title       string         `json:"title"`
	Description string         `json:"description"`
	Config      map[string]any `json:"config"`
}

// ListMcpServerTemplates returns the built-in MCP catalog in the requested
// language. Read-only and workspace-independent: templates ship with the
// binary, so this answers the same for every workspace. It lives behind the
// workspace-scoped API group so the client needs no special call shape, and it
// is member-visible for the same reason as the workspace MCP library — an agent
// owner needs to see what they can add.
func (h *Handler) ListMcpServerTemplates(w http.ResponseWriter, r *http.Request) {
	language := templateLanguageFromRequest(r.URL.Query().Get("language"))
	templates := service.McpServerTemplates()
	out := make([]McpServerTemplateResponse, 0, len(templates))
	for _, template := range templates {
		out = append(out, McpServerTemplateResponse{
			Key:         template.Key,
			Title:       template.Title(language),
			Description: template.Description(language),
			Config:      template.Config,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"templates": out})
}
