package service

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestMcpServerTemplates_IntranetIgnoresUnsafeInheritedSettings(t *testing.T) {
	for _, template := range McpServerTemplates() {
		env, _ := template.Config["env"].(map[string]any)
		switch template.Key {
		case "kubernetes":
			if value, present := env["K8S_MCP_CONFIG_PATH"]; !present || value != "" {
				t.Error("Kubernetes must not load an inherited TOML that changes transport or telemetry")
			}
		case "redis":
			if env["REDIS_SSL_CERT_REQS"] != "required" {
				t.Error("Redis TLS must verify certificates even when the parent environment disables verification")
			}
		}
	}
}

var intranetStaticEnv = map[string]map[string]any{
	"gitlab":     {"GITLAB_PERMISSION_MODE": "readonly", "GITLAB_DISABLE_VERSION_CHECK": "true", "STREAMABLE_HTTP": "false", "SSE": "false"},
	"atlassian":  {"JIRA_URL": "", "JIRA_PERSONAL_TOKEN": "", "CONFLUENCE_URL": "", "CONFLUENCE_PERSONAL_TOKEN": "", "MCP_LOGGING_STDOUT": "false", "MCP_ATLASSIAN_USE_SYSTEM_TRUSTSTORE": "true"},
	"grafana":    {"OTEL_EXPORTER_OTLP_ENDPOINT": "", "OTEL_EXPORTER_OTLP_TRACES_ENDPOINT": "", "OTEL_EXPORTER_OTLP_LOGS_ENDPOINT": "", "OTEL_LOGS_EXPORTER": "none"},
	"kubernetes": {"K8S_MCP_CONFIG_PATH": "", "OTEL_EXPORTER_OTLP_ENDPOINT": "", "OTEL_EXPORTER_OTLP_TRACES_ENDPOINT": "", "OTEL_EXPORTER_OTLP_LOGS_ENDPOINT": ""},
	"mongodb":    {"MDB_MCP_TRANSPORT": "stdio", "MDB_MCP_READ_ONLY": "true", "MDB_MCP_TELEMETRY": "disabled", "MDB_MCP_DISABLED_TOOLS": "atlas,atlas-local,assistant,connect"},
	"redis":      {"REDIS_USERNAME": "default", "REDIS_PWD": "", "MCP_DOCS_SEARCH_URL": "", "REDIS_SSL_CERT_REQS": "required"},
	"clickhouse": {"CLICKHOUSE_MCP_SERVER_TRANSPORT": "stdio", "CLICKHOUSE_VERIFY": "true", "CLICKHOUSE_ALLOW_WRITE_ACCESS": "false", "CLICKHOUSE_ALLOW_DROP": "false", "CHDB_ENABLED": "false", "CLICKHOUSE_PASSWORD": "", "CLICKHOUSE_CA_CERT": ""},
}

var intranetInputExamples = map[string]map[string]string{
	"gitlab":     {"gitlab_api_url": "https://gitlab.internal/api/v4", "gitlab_token": "private-gitlab"},
	"atlassian":  {"jira_url": "https://jira.internal", "jira_token": "private-jira"},
	"grafana":    {"grafana_url": "https://grafana.internal/grafana", "grafana_token": "private-grafana"},
	"kubernetes": {"kubeconfig_path": `C:\Users\agent\.kube\config`},
	"mongodb":    {"mongodb_uri": "mongodb://reader:private-mongodb@mongo1.internal:27017,mongo2.internal:27018/app?replicaSet=rs0&authSource=admin"},
	"redis":      {"redis_url": "rediss://redis.internal:6379/0", "redis_username": "private-reader", "redis_password": "private-redis"},
	"clickhouse": {"clickhouse_url": "https://clickhouse.internal:8443", "clickhouse_username": "reader", "clickhouse_password": "private-clickhouse", "clickhouse_ca_path": "/etc/certs/ca.pem"},
}

func TestMcpServerTemplates_IntranetRoster(t *testing.T) {
	want := map[string]string{
		"gitlab": "collaboration", "atlassian": "collaboration",
		"grafana": "operations", "kubernetes": "operations",
		"mongodb": "database", "redis": "database", "clickhouse": "database",
	}
	for _, template := range McpServerTemplates() {
		category, ok := want[template.Key]
		if !ok {
			continue
		}
		if template.Category != category || template.Version != "1" {
			t.Errorf("%s has wrong category/version", template.Key)
		}
		delete(want, template.Key)
	}
	for key := range want {
		t.Errorf("missing requested intranet template: %s", key)
	}
}

func TestMcpServerTemplates_IntranetResolvedRecipes(t *testing.T) {
	for key, tc := range map[string]struct {
		command string
		args    []any
		env     map[string]any
	}{
		"gitlab":     {"zereight-mcp-gitlab", []any{}, map[string]any{"GITLAB_API_URL": "https://gitlab.internal/api/v4", "GITLAB_PERSONAL_ACCESS_TOKEN": "private-gitlab"}},
		"atlassian":  {"mcp-atlassian", []any{"--transport", "stdio", "--read-only", "--jira-ssl-verify", "--confluence-ssl-verify"}, map[string]any{"JIRA_URL": "https://jira.internal", "JIRA_PERSONAL_TOKEN": "private-jira"}},
		"grafana":    {"mcp-grafana", []any{"--transport", "stdio", "--disable-write", "--disable-docs", "--usage-stats=disabled", "--enabled-tools=search,datasource,dashboard,prometheus,loki,alerting"}, map[string]any{"GRAFANA_URL": "https://grafana.internal/grafana", "GRAFANA_SERVICE_ACCOUNT_TOKEN": "private-grafana"}},
		"kubernetes": {"kubernetes-mcp-server", []any{"--read-only", "--cluster-provider", "kubeconfig", "--kubeconfig", `C:\Users\agent\.kube\config`}, nil},
		"mongodb":    {"mongodb-mcp-server", []any{}, map[string]any{"MDB_MCP_CONNECTION_STRING": intranetInputExamples["mongodb"]["mongodb_uri"]}},
		"redis":      {"redis-mcp-server", []any{"--url", "rediss://redis.internal:6379/0"}, map[string]any{"REDIS_USERNAME": "private-reader", "REDIS_PWD": "private-redis"}},
		"clickhouse": {"mcp-clickhouse", []any{}, map[string]any{"CLICKHOUSE_HOST": "clickhouse.internal", "CLICKHOUSE_PORT": "8443", "CLICKHOUSE_SECURE": "true", "CLICKHOUSE_USER": "reader", "CLICKHOUSE_PASSWORD": "private-clickhouse", "CLICKHOUSE_CA_CERT": "/etc/certs/ca.pem"}},
	} {
		t.Run(key, func(t *testing.T) {
			config, err := ResolveMcpServerTemplate(key, "1", intranetInputExamples[key])
			if err != nil {
				t.Fatal(err)
			}
			wantEnv := map[string]any{}
			for name, value := range intranetStaticEnv[key] {
				wantEnv[name] = value
			}
			for name, value := range tc.env {
				wantEnv[name] = value
			}
			want := map[string]any{"command": tc.command, "args": tc.args, "env": wantEnv}
			if !reflect.DeepEqual(config, want) {
				t.Fatal("resolved recipe differs from the reviewed local command and environment")
			}
			args, err := json.Marshal(config["args"])
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(args), "private-") {
				t.Fatal("credential escaped into process arguments")
			}
			config["env"].(map[string]any)["MUTATED"] = "private"
			again, err := ResolveMcpServerTemplate(key, "1", intranetInputExamples[key])
			if err != nil || !reflect.DeepEqual(again, want) {
				t.Fatal("resolution mutated the catalog")
			}
		})
	}
}

func TestMcpServerTemplates_IntranetOptionalInputs(t *testing.T) {
	for _, tc := range []struct {
		key    string
		inputs map[string]string
		env    map[string]any
	}{
		{"atlassian", map[string]string{"confluence_url": "http://confluence.internal/wiki", "confluence_token": "private-token"}, map[string]any{"JIRA_URL": "", "JIRA_PERSONAL_TOKEN": "", "CONFLUENCE_URL": "http://confluence.internal/wiki"}},
		{"atlassian", map[string]string{"jira_url": "https://jira.internal", "jira_token": "private-jira", "confluence_url": "https://confluence.internal", "confluence_token": "private-confluence"}, map[string]any{"JIRA_PERSONAL_TOKEN": "private-jira", "CONFLUENCE_PERSONAL_TOKEN": "private-confluence"}},
		{"redis", map[string]string{"redis_url": "redis://redis.internal"}, map[string]any{"REDIS_USERNAME": "default", "REDIS_PWD": "", "MCP_DOCS_SEARCH_URL": ""}},
		{"redis", map[string]string{"redis_url": "redis://redis.internal/2", "redis_password": ""}, map[string]any{"REDIS_PWD": ""}},
		{"clickhouse", map[string]string{"clickhouse_url": "http://clickhouse.internal:8123", "clickhouse_username": "default"}, map[string]any{"CLICKHOUSE_PORT": "8123", "CLICKHOUSE_SECURE": "false", "CLICKHOUSE_PASSWORD": ""}},
		{"clickhouse", map[string]string{"clickhouse_url": "http://clickhouse.internal", "clickhouse_username": "default", "clickhouse_password": ""}, map[string]any{"CLICKHOUSE_PORT": "80", "CLICKHOUSE_PASSWORD": ""}},
		{"clickhouse", map[string]string{"clickhouse_url": "https://[::1]", "clickhouse_username": "default"}, map[string]any{"CLICKHOUSE_HOST": "::1", "CLICKHOUSE_PORT": "443", "CLICKHOUSE_SECURE": "true"}},
	} {
		config, err := ResolveMcpServerTemplate(tc.key, "1", tc.inputs)
		if err != nil {
			t.Fatal(err)
		}
		encoded, err := json.Marshal(config)
		if err != nil {
			t.Fatal(err)
		}
		var roundTrip map[string]any
		if err := json.Unmarshal(encoded, &roundTrip); err != nil {
			t.Fatal(err)
		}
		env := roundTrip["env"].(map[string]any)
		for key, want := range tc.env {
			if got, exists := env[key]; !exists || got != want {
				t.Errorf("%s: expected explicit environment value for %s", tc.key, key)
			}
		}
	}
}

func TestMcpServerTemplates_IntranetRejectsInvalidInputs(t *testing.T) {
	for _, tc := range []struct{ name, key, field, value string }{
		{"gitlab path", "gitlab", "gitlab_api_url", "https://private.internal"},
		{"gitlab credentials", "gitlab", "gitlab_api_url", "https://private@host/api/v4"},
		{"gitlab query", "gitlab", "gitlab_api_url", "https://host/api/v4?private=1"},
		{"gitlab fragment", "gitlab", "gitlab_api_url", "https://host/api/v4#private"},
		{"gitlab whitespace token", "gitlab", "gitlab_token", "   "},
		{"grafana non-http", "grafana", "grafana_url", "file:///private"},
		{"grafana invalid port", "grafana", "grafana_url", "https://private.internal:99999"},
		{"kube relative path", "kubernetes", "kubeconfig_path", "private/config"},
		{"atlassian missing token", "atlassian", "jira_token", ""},
		{"atlassian token only", "atlassian", "jira_url", ""},
		{"atlassian partial second pair", "atlassian", "confluence_token", "private-token"},
		{"mongodb empty authority", "mongodb", "mongodb_uri", "mongodb:///private"},
		{"mongodb wrong scheme", "mongodb", "mongodb_uri", "https://private.internal"},
		{"mongodb bad port", "mongodb", "mongodb_uri", "mongodb://private.internal:99999"},
		{"mongodb SRV port", "mongodb", "mongodb_uri", "mongodb+srv://private.internal:27017/db"},
		{"mongodb SRV multi-host", "mongodb", "mongodb_uri", "mongodb+srv://private.internal,other.internal/db"},
		{"mongodb malformed credential", "mongodb", "mongodb_uri", "mongodb://private%zz@mongo.internal/db"},
		{"mongodb insecure TLS", "mongodb", "mongodb_uri", "mongodb://mongo.internal/private?tlsInsecure=true"},
		{"mongodb invalid certs", "mongodb", "mongodb_uri", "mongodb://mongo.internal/private?tlsAllowInvalidCertificates=true"},
		{"redis userinfo", "redis", "redis_url", "redis://private@redis.internal/0"},
		{"redis query", "redis", "redis_url", "rediss://redis.internal/0?ssl_cert_reqs=private"},
		{"redis negative database", "redis", "redis_url", "redis://redis.internal/-1"},
		{"redis invalid database", "redis", "redis_url", "redis://redis.internal/private"},
		{"clickhouse path", "clickhouse", "clickhouse_url", "https://host/private"},
		{"clickhouse native scheme", "clickhouse", "clickhouse_url", "clickhouse://private.internal:9000"},
		{"clickhouse HTTP CA", "clickhouse", "clickhouse_url", "http://private.internal:8123"},
		{"clickhouse relative CA", "clickhouse", "clickhouse_ca_path", "private.pem"},
		{"blank optional secret", "redis", "redis_password", "   "},
		{"control char", "redis", "redis_password", "private\nvalue"},
		{"long secret", "redis", "redis_password", strings.Repeat("private", 2048)},
		{"unknown key", "redis", "private_unknown", "private"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			inputs := map[string]string{}
			for key, value := range intranetInputExamples[tc.key] {
				inputs[key] = value
			}
			inputs[tc.field] = tc.value
			config, err := ResolveMcpServerTemplate(tc.key, "1", inputs)
			if err == nil || config != nil {
				t.Fatal("invalid input was accepted")
			}
			if strings.Contains(err.Error(), "private") || strings.Contains(err.Error(), "outdated recipe") {
				t.Fatal("input error leaked user content or bypassed validation")
			}
		})
	}
	if _, err := ResolveMcpServerTemplate("atlassian", "1", nil); err == nil {
		t.Fatal("at least one complete Atlassian pair is required")
	}
}

func TestMcpServerTemplates_IntranetMongoVariants(t *testing.T) {
	for _, uri := range []string{
		"mongodb://host/db", "mongodb://host1:27017,host2:27018/db?replicaSet=rs0",
		"mongodb://user:p%40ss@host/db?authSource=admin&authMechanism=SCRAM-SHA-256",
		"mongodb://[::1]:27017,[::2]:27018/db", "mongodb+srv://cluster.internal/db?authSource=admin",
		"mongodb://host/db?tls=true&tlsCAFile=%2Fetc%2Fca.pem&tlsAllowInvalidCertificates=false",
	} {
		if _, err := ResolveMcpServerTemplate("mongodb", "1", map[string]string{"mongodb_uri": uri}); err != nil {
			t.Fatalf("valid MongoDB connection shape rejected: %v", err)
		}
	}
}

func TestMcpServerTemplates_IntranetMetadata(t *testing.T) {
	wantInputs := map[string]map[string][2]bool{
		"gitlab":     {"gitlab_api_url": {true, false}, "gitlab_token": {true, true}},
		"atlassian":  {"jira_url": {false, false}, "jira_token": {false, true}, "confluence_url": {false, false}, "confluence_token": {false, true}},
		"grafana":    {"grafana_url": {true, false}, "grafana_token": {true, true}},
		"kubernetes": {"kubeconfig_path": {true, false}},
		"mongodb":    {"mongodb_uri": {true, true}},
		"redis":      {"redis_url": {true, false}, "redis_username": {false, true}, "redis_password": {false, true}},
		"clickhouse": {"clickhouse_url": {true, false}, "clickhouse_username": {true, false}, "clickhouse_password": {false, true}, "clickhouse_ca_path": {false, false}},
	}
	for _, template := range McpServerTemplates() {
		if _, ok := intranetInputExamples[template.Key]; !ok {
			continue
		}
		if !reflect.DeepEqual(template.Config["env"], intranetStaticEnv[template.Key]) {
			t.Errorf("%s static safety environment differs", template.Key)
		}
		if len(template.Inputs) != len(wantInputs[template.Key]) {
			t.Errorf("%s has an unexpected input count", template.Key)
		}
		for _, input := range template.Inputs {
			if want, exists := wantInputs[template.Key][input.Key]; !exists || input.Required != want[0] || input.Secret != want[1] {
				t.Errorf("%s has an unexpected input contract for %s", template.Key, input.Key)
			}
			for _, language := range TemplateLanguages {
				if input.Labels[language] == "" || input.Descriptions[language] == "" {
					t.Errorf("%s missing %s input metadata", template.Key, language)
				}
			}
		}
	}
}

func TestMcpServerTemplates_IntranetValidationDoesNotAffectDeploymentRecipes(t *testing.T) {
	template := McpServerTemplate{
		Source: "deployment", Key: "clickhouse", Config: map[string]any{"command": "custom-tool", "args": []any{}},
		Inputs: []McpTemplateInput{{Key: "clickhouse_url", environment: "CUSTOM_ENDPOINT"}},
	}
	config, err := resolveMcpTemplateInputs(template, map[string]string{"clickhouse_url": "custom:not-http"})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(config["env"], map[string]any{"CUSTOM_ENDPOINT": "custom:not-http"}) {
		t.Fatal("builtin identity altered a deployment recipe")
	}
}
