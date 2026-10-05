package service

import (
	"errors"
	"strings"
)

var (
	ErrMcpCatalogUnavailable  = errors.New("MCP catalog is unavailable")
	ErrMcpTemplateChanged     = errors.New("MCP template has changed")
	ErrMcpTemplateUnavailable = errors.New("MCP template is unavailable")
)

// McpCatalog combines immutable builtins with a freshly read deployment directory.
// It never executes commands, probes endpoints, or retains submitted inputs.
type McpCatalog struct {
	Directory string
	AllowHTTP bool
}

func (c McpCatalog) List() ([]McpServerTemplate, error) {
	deployed, err := c.readDeploymentTemplates()
	if err != nil {
		return nil, err
	}
	result := McpServerTemplates()
	for i := range result {
		result[i].Source = "builtin"
		result[i].Transport = "stdio"
		if _, ok := result[i].Config["url"]; ok {
			result[i].Transport = "http"
		}
	}
	for _, template := range deployed {
		// Deployment recipes and input destinations are never a public preview.
		template.Config = nil
		result = append(result, template)
	}
	return result, nil
}

func (c McpCatalog) Resolve(source, key, version string, inputs map[string]string) (map[string]any, error) {
	switch source {
	case "", "builtin":
		return ResolveMcpServerTemplate(key, version, inputs)
	case "deployment":
		if !mcpTemplateName.MatchString(key) || len(key) > 128 || strings.TrimSpace(version) == "" {
			return nil, errors.New("invalid MCP template identity")
		}
		deployed, err := c.readDeploymentTemplates()
		if err != nil {
			return nil, err
		}
		for _, template := range deployed {
			if template.Key != key {
				continue
			}
			if template.Version != version {
				return nil, ErrMcpTemplateChanged
			}
			// Compare and resolve the same validated snapshot; do not reread here.
			return resolveMcpTemplateInputs(template, inputs)
		}
		return nil, ErrMcpTemplateUnavailable
	default:
		return nil, errors.New("unknown MCP template source")
	}
}
