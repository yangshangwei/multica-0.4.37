package service

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// McpTemplateInput describes a value collected when saving a trusted recipe.
// Destination fields are private: clients supply values, never config paths.
type McpTemplateInput struct {
	Key          string
	Required     bool
	Secret       bool
	Labels       map[string]string
	Descriptions map[string]string
	argument     string
	environment  string
	positional   bool
	header       string
	prefix       string
	validator    string
}

func (i McpTemplateInput) Label(language string) string {
	return localizedTemplateString(i.Labels, language, i.Key)
}

func (i McpTemplateInput) Description(language string) string {
	return localizedTemplateString(i.Descriptions, language, "")
}

// ResolveMcpServerTemplate applies write-only inputs to a server-owned recipe.
// Values remain single arguments or environment values, with no shell expansion.
// Errors contain trusted field identities only, never request keys or values.
func ResolveMcpServerTemplate(key, version string, inputs map[string]string) (map[string]any, error) {
	for _, template := range McpServerTemplates() {
		if template.Key != key || template.Version != version {
			continue
		}
		return resolveMcpTemplateInputs(template, inputs)
	}
	return nil, errors.New("unknown MCP template or outdated recipe version")
}

// resolveMcpTemplateInputs is shared by both sources. Always copy the recipe,
// including slices and nested maps, before adding write-only input values.
func resolveMcpTemplateInputs(template McpServerTemplate, inputs map[string]string) (map[string]any, error) {
	known := make(map[string]bool, len(template.Inputs))
	for _, input := range template.Inputs {
		known[input.Key] = true
	}
	total := 0
	for key, value := range inputs {
		if !known[key] {
			return nil, errors.New("unknown MCP template input")
		}
		total += len(key) + len(value)
		if total > 65536 {
			return nil, errors.New("MCP template inputs exceed 65536 bytes")
		}
	}
	config := cloneMcpConfigValue(template.Config).(map[string]any)
	for _, input := range template.Inputs {
		value := inputs[input.Key]
		if input.Required && strings.TrimSpace(value) == "" {
			return nil, fmt.Errorf("%s is required", input.Key)
		}
		if !mcpSingleLine(value, 8192) {
			return nil, fmt.Errorf("%s must be a single line of at most 8192 bytes", input.Key)
		}
		if value == "" {
			continue
		}
		validator := input.validator
		if validator == "" {
			switch input.Key {
			case "project_path":
				validator = "absolute_path"
			case "database_url":
				validator = "database_url"
			}
		}
		switch validator {
		case "absolute_path":
			if !absoluteMcpProjectPath(value) {
				return nil, fmt.Errorf("%s must be an absolute directory on the agent machine", input.Key)
			}
		case "database_url":
			postgresOnly := template.Source != "deployment" && template.Key == "postgres-mcp"
			if !validMcpDatabaseURL(value, postgresOnly) {
				return nil, fmt.Errorf("%s must be a supported database connection URL", input.Key)
			}
		}
		switch {
		case input.argument != "" || input.positional:
			args, _ := config["args"].([]any)
			if input.argument != "" {
				args = append(args, input.argument)
			}
			config["args"] = append(args, value)
		case input.environment != "":
			env, _ := config["env"].(map[string]any)
			if env == nil {
				env = map[string]any{}
				config["env"] = env
			}
			env[input.environment] = value
		case input.header != "":
			headers, _ := config["headers"].(map[string]any)
			if headers == nil {
				headers = map[string]any{}
				config["headers"] = headers
			}
			headers[input.header] = input.prefix + value
		}
	}
	if template.Source != "deployment" {
		return resolveIntranetMcpInputs(template, inputs, config)
	}
	return config, nil
}

func cloneMcpConfigValue(value any) any {
	switch value := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(value))
		for key, item := range value {
			result[key] = cloneMcpConfigValue(item)
		}
		return result
	case []any:
		result := make([]any, len(value))
		for i, item := range value {
			result[i] = cloneMcpConfigValue(item)
		}
		return result
	default:
		return value
	}
}

// Runtime paths may target another OS, so filepath.IsAbs on the server is not sufficient.
func absoluteMcpProjectPath(value string) bool {
	if strings.HasPrefix(value, "/") {
		return true
	}
	if len(value) >= 3 && ((value[0] >= 'A' && value[0] <= 'Z') || (value[0] >= 'a' && value[0] <= 'z')) && value[1] == ':' && (value[2] == '\\' || value[2] == '/') {
		return true
	}
	if strings.HasPrefix(value, `\\`) {
		parts := strings.Split(strings.TrimPrefix(value, `\\`), `\`)
		return len(parts) >= 2 && parts[0] != "" && parts[1] != ""
	}
	return false
}

func validMcpDatabaseURL(value string, postgresOnly bool) bool {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Opaque != "" || parsed.Fragment != "" || !strings.HasPrefix(value, parsed.Scheme+"://") {
		return false
	}
	switch parsed.Scheme {
	case "postgres", "postgresql":
	case "mysql", "mariadb", "sqlserver", "oracle":
		if postgresOnly {
			return false
		}
	case "sqlite":
		return !postgresOnly && parsed.User == nil && (parsed.Host == "" || parsed.Host == ".") && strings.Trim(parsed.Path, "/") != ""
	default:
		return false
	}
	return parsed.Hostname() != ""
}

func databaseMcpTemplateInput(environment string) McpTemplateInput {
	return McpTemplateInput{
		Key: "database_url", Required: true, Secret: true,
		Labels: map[string]string{"en": "Database connection URL", "zh": "数据库连接地址"},
		Descriptions: map[string]string{
			"en": "Enter the connection URL accessible from the agent runtime. Credentials are saved without being shown again.",
			"zh": "填写智能体运行环境可访问的数据库连接地址。连接凭证保存后不会再次显示。",
		},
		environment: environment,
	}
}
