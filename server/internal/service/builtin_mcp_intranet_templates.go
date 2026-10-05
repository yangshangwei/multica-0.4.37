package service

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// These recipes launch preinstalled, reviewed releases. They never bootstrap
// packages or images, and all service addresses come from write-only inputs.
func builtinIntranetMcpTemplates() []McpServerTemplate {
	return []McpServerTemplate{
		{
			Key: "gitlab", Version: "1", Category: "collaboration",
			DocumentationURL: "https://github.com/zereight/gitlab-mcp/tree/v2.1.69",
			Titles:           map[string]string{"en": "GitLab MCP", "zh": "GitLab MCP"},
			Descriptions:     map[string]string{"en": "Read repositories, merge requests, and issues from your self-hosted GitLab.", "zh": "读取自建 GitLab 的代码仓库、合并请求和议题。"},
			Requirements: map[string][]string{
				"en": {"Preinstall @zereight/mcp-gitlab 2.1.69 and Node.js 18.17 or later; zereight-mcp-gitlab must be on PATH", "Use the full intranet GitLab API URL ending in /api/v4 and a read-only personal access token", "Install private CA certificates in the runtime trust store; version checks are disabled"},
				"zh": {"预装 @zereight/mcp-gitlab 2.1.69 和 Node.js 18.17 或更高版本，并将 zereight-mcp-gitlab 加入 PATH", "填写以 /api/v4 结尾的内网 GitLab API 地址，并使用只读个人访问令牌", "私有 CA 证书需加入运行环境的信任库；已关闭版本检查"},
			},
			Config: map[string]any{"command": "zereight-mcp-gitlab", "args": []any{}, "env": map[string]any{
				"GITLAB_PERMISSION_MODE": "readonly", "GITLAB_DISABLE_VERSION_CHECK": "true", "STREAMABLE_HTTP": "false", "SSE": "false",
			}},
			Inputs: []McpTemplateInput{
				{Key: "gitlab_api_url", Required: true, environment: "GITLAB_API_URL", Labels: map[string]string{"en": "GitLab API URL", "zh": "GitLab API 地址"}, Descriptions: map[string]string{"en": "Full HTTP(S) API base, for example https://gitlab.intranet/api/v4.", "zh": "完整 HTTP(S) API 地址，例如 https://gitlab.intranet/api/v4。"}},
				{Key: "gitlab_token", Required: true, Secret: true, environment: "GITLAB_PERSONAL_ACCESS_TOKEN", Labels: map[string]string{"en": "GitLab personal access token", "zh": "GitLab 个人访问令牌"}, Descriptions: map[string]string{"en": "A token with read-only access to the required repositories and issues.", "zh": "使用仅能读取目标仓库和议题的访问令牌。"}},
			},
		},
		{
			Key: "atlassian", Version: "1", Category: "collaboration",
			DocumentationURL: "https://github.com/sooperset/mcp-atlassian/tree/v0.23.1",
			Titles:           map[string]string{"en": "Atlassian MCP", "zh": "Atlassian MCP"},
			Descriptions:     map[string]string{"en": "Search and read self-hosted Jira issues and Confluence pages using personal access tokens.", "zh": "使用个人访问令牌搜索和读取自建 Jira 议题及 Confluence 页面。"},
			Requirements: map[string][]string{
				"en": {"Preinstall mcp-atlassian 0.23.1 and Python 3.10 or later; mcp-atlassian must be on PATH", "Configure at least one complete URL/token pair for Jira or Confluence Server/Data Center", "Launch from a directory without unrelated .env files; this release loads .env with override enabled", "TLS verification and read-only mode are enabled; install private CA certificates in the system trust store"},
				"zh": {"预装 mcp-atlassian 0.23.1 和 Python 3.10 或更高版本，并将 mcp-atlassian 加入 PATH", "至少完整填写一组 Jira 或 Confluence Server/Data Center 地址及令牌", "启动目录不能包含无关的 .env 文件；此版本会读取该文件并覆盖环境变量", "已启用 TLS 证书验证和只读模式；私有 CA 证书需加入系统信任库"},
			},
			// Explicit verification flags take precedence over a discovered .env file.
			Config: map[string]any{"command": "mcp-atlassian", "args": []any{"--transport", "stdio", "--read-only", "--jira-ssl-verify", "--confluence-ssl-verify"}, "env": map[string]any{
				"JIRA_URL": "", "JIRA_PERSONAL_TOKEN": "", "CONFLUENCE_URL": "", "CONFLUENCE_PERSONAL_TOKEN": "", "MCP_LOGGING_STDOUT": "false", "MCP_ATLASSIAN_USE_SYSTEM_TRUSTSTORE": "true",
			}},
			Inputs: []McpTemplateInput{
				{Key: "jira_url", environment: "JIRA_URL", Labels: map[string]string{"en": "Jira URL", "zh": "Jira 地址"}, Descriptions: map[string]string{"en": "Self-hosted Jira HTTP(S) base URL; fill this together with its personal token.", "zh": "自建 Jira 的 HTTP(S) 地址，需与个人令牌一起填写。"}},
				{Key: "jira_token", Secret: true, environment: "JIRA_PERSONAL_TOKEN", Labels: map[string]string{"en": "Jira personal access token", "zh": "Jira 个人访问令牌"}, Descriptions: map[string]string{"en": "Required when a Jira URL is supplied; leave both blank to use only Confluence.", "zh": "填写 Jira 地址时必须填写；仅使用 Confluence 时请将两项留空。"}},
				{Key: "confluence_url", environment: "CONFLUENCE_URL", Labels: map[string]string{"en": "Confluence URL", "zh": "Confluence 地址"}, Descriptions: map[string]string{"en": "Self-hosted Confluence HTTP(S) base URL; fill this together with its personal token.", "zh": "自建 Confluence 的 HTTP(S) 地址，需与个人令牌一起填写。"}},
				{Key: "confluence_token", Secret: true, environment: "CONFLUENCE_PERSONAL_TOKEN", Labels: map[string]string{"en": "Confluence personal access token", "zh": "Confluence 个人访问令牌"}, Descriptions: map[string]string{"en": "Required when a Confluence URL is supplied; leave both blank to use only Jira.", "zh": "填写 Confluence 地址时必须填写；仅使用 Jira 时请将两项留空。"}},
			},
		},
		{
			Key: "grafana", Version: "1", Category: "operations",
			DocumentationURL: "https://github.com/grafana/mcp-grafana/tree/v2.0.0",
			Titles:           map[string]string{"en": "Grafana MCP", "zh": "Grafana MCP"},
			Descriptions:     map[string]string{"en": "Inspect on-premises dashboards, metrics, logs, and alerts with read-only Grafana tools.", "zh": "通过只读工具查看内网 Grafana 仪表盘、指标、日志和告警。"},
			Requirements: map[string][]string{
				"en": {"Preinstall the mcp-grafana v2.0.0 executable on PATH", "Provide an intranet Grafana URL and preferably a Viewer service-account token", "Cloud product tools, public documentation search, usage reporting, and writes are disabled; PromQL and LogQL remain available"},
				"zh": {"预装 mcp-grafana v2.0.0 可执行程序并加入 PATH", "填写内网 Grafana 地址，建议使用 Viewer 服务账号令牌", "已关闭云产品工具、公共文档搜索、用量上报和写入；仍可使用 PromQL 和 LogQL"},
			},
			Config: map[string]any{"command": "mcp-grafana", "args": []any{"--transport", "stdio", "--disable-write", "--disable-docs", "--usage-stats=disabled", "--enabled-tools=search,datasource,dashboard,prometheus,loki,alerting"}, "env": map[string]any{
				"OTEL_EXPORTER_OTLP_ENDPOINT": "", "OTEL_EXPORTER_OTLP_TRACES_ENDPOINT": "", "OTEL_EXPORTER_OTLP_LOGS_ENDPOINT": "", "OTEL_LOGS_EXPORTER": "none",
			}},
			Inputs: []McpTemplateInput{
				{Key: "grafana_url", Required: true, environment: "GRAFANA_URL", Labels: map[string]string{"en": "Grafana URL", "zh": "Grafana 地址"}, Descriptions: map[string]string{"en": "HTTP(S) base URL of your intranet Grafana, including any deployment subpath.", "zh": "内网 Grafana 的 HTTP(S) 地址，可包含部署子路径。"}},
				{Key: "grafana_token", Required: true, Secret: true, environment: "GRAFANA_SERVICE_ACCOUNT_TOKEN", Labels: map[string]string{"en": "Grafana service-account token", "zh": "Grafana 服务账号令牌"}, Descriptions: map[string]string{"en": "Use a Viewer token with access to the dashboards and data sources you need.", "zh": "使用有权访问所需仪表盘和数据源的 Viewer 令牌。"}},
			},
		},
		{
			Key: "kubernetes", Version: "1", Category: "operations",
			DocumentationURL: "https://github.com/containers/kubernetes-mcp-server/tree/v0.0.67",
			Titles:           map[string]string{"en": "Kubernetes MCP", "zh": "Kubernetes MCP"},
			Descriptions:     map[string]string{"en": "Read cluster resources, events, and logs using a local kubeconfig and read-only tools.", "zh": "通过本地 kubeconfig 和只读工具查看集群资源、事件及日志。"},
			Requirements: map[string][]string{
				"en": {"Preinstall kubernetes-mcp-server v0.0.67 on PATH; this recipe is verified only for this release", "Provide an absolute kubeconfig path on the agent machine; preinstall any exec authentication plugins referenced by it", "Use read-only cluster RBAC and an intranet cluster endpoint; no extra server configuration is loaded"},
				"zh": {"预装 kubernetes-mcp-server v0.0.67 并加入 PATH；此模板仅针对该版本核验", "填写智能体机器上的 kubeconfig 绝对路径；预装其中引用的 exec 认证插件", "使用只读集群 RBAC 和内网集群地址；不加载额外的服务器配置文件"},
			},
			// v0.0.67 enables telemetry only when an endpoint exists. No TOML is
			// loaded here, and empty variables prevent inherited collector URLs.
			Config: map[string]any{"command": "kubernetes-mcp-server", "args": []any{"--read-only", "--cluster-provider", "kubeconfig"}, "env": map[string]any{
				"K8S_MCP_CONFIG_PATH":         "",
				"OTEL_EXPORTER_OTLP_ENDPOINT": "", "OTEL_EXPORTER_OTLP_TRACES_ENDPOINT": "", "OTEL_EXPORTER_OTLP_LOGS_ENDPOINT": "",
			}},
			Inputs: []McpTemplateInput{
				{Key: "kubeconfig_path", Required: true, argument: "--kubeconfig", validator: "absolute_path", Labels: map[string]string{"en": "Kubeconfig path", "zh": "Kubeconfig 路径"}, Descriptions: map[string]string{"en": "Absolute path on the agent machine, such as /home/user/.kube/config or C:\\Users\\user\\.kube\\config.", "zh": "智能体机器上的绝对路径，例如 /home/user/.kube/config 或 C:\\Users\\user\\.kube\\config。"}},
			},
		},
		{
			Key: "mongodb", Version: "1", Category: "database",
			DocumentationURL: "https://github.com/mongodb-js/mongodb-mcp-server/tree/v3.0.5",
			Titles:           map[string]string{"en": "MongoDB MCP", "zh": "MongoDB MCP"},
			Descriptions:     map[string]string{"en": "Explore a self-hosted MongoDB database with read-only tools, without Atlas or online assistant tools.", "zh": "使用只读工具查询自建 MongoDB，不启用 Atlas 或在线助手工具。"},
			Requirements: map[string][]string{
				"en": {"Preinstall mongodb-mcp-server 3.0.5 on PATH and a supported Node.js version: 20.19+, 22.13+, or 24+", "Use a read-only database principal and an intranet mongodb:// URI; mongodb+srv:// requires internal SRV DNS", "Telemetry, Atlas, local Atlas deployment, online assistant, and connection-switching tools are disabled"},
				"zh": {"预装 mongodb-mcp-server 3.0.5 并加入 PATH；Node.js 需为 20.19+、22.13+ 或 24+", "使用只读数据库账号及内网 mongodb:// 地址；mongodb+srv:// 需要内网 SRV DNS", "已关闭遥测、Atlas、本地 Atlas 部署、在线助手和切换连接工具"},
			},
			Config: map[string]any{"command": "mongodb-mcp-server", "args": []any{}, "env": map[string]any{
				"MDB_MCP_TRANSPORT": "stdio", "MDB_MCP_READ_ONLY": "true", "MDB_MCP_TELEMETRY": "disabled", "MDB_MCP_DISABLED_TOOLS": "atlas,atlas-local,assistant,connect",
			}},
			Inputs: []McpTemplateInput{
				{Key: "mongodb_uri", Required: true, Secret: true, environment: "MDB_MCP_CONNECTION_STRING", Labels: map[string]string{"en": "MongoDB connection URI", "zh": "MongoDB 连接 URI"}, Descriptions: map[string]string{"en": "mongodb:// or mongodb+srv:// URI; credentials, replica-set hosts, and authentication options may be included. TLS verification must stay enabled.", "zh": "填写 mongodb:// 或 mongodb+srv:// URI，可包含凭证、副本集主机和认证选项；不得关闭 TLS 证书验证。"}},
			},
		},
		{
			Key: "redis", Version: "1", Category: "database",
			DocumentationURL: "https://github.com/redis/mcp-redis/tree/0.5.1",
			Titles:           map[string]string{"en": "Redis MCP", "zh": "Redis MCP"},
			Descriptions:     map[string]string{"en": "Inspect your intranet Redis through a configured ACL account; available operations follow that account's permissions.", "zh": "通过指定 ACL 账号访问内网 Redis，可执行操作由账号权限决定。"},
			Requirements: map[string][]string{
				"en": {"Preinstall redis-mcp-server 0.5.1 and Python 3.10 or later; redis-mcp-server must be on PATH", "Redis MCP has no read-only switch; use a restricted Redis ACL account for read-only access", "The online documentation tool remains visible but its endpoint is disabled; URL query options are not supported by this template", "Remove REDIS_ENTRAID_AUTH_FLOW from the startup environment and .env; an empty value does not disable cloud authentication"},
				"zh": {"预装 redis-mcp-server 0.5.1 和 Python 3.10 或更高版本，并将 redis-mcp-server 加入 PATH", "Redis MCP 没有只读开关；只读访问需使用受限 Redis ACL 账号", "在线文档工具仍可见，但其联网地址已禁用；此模板不支持 URL 查询参数", "启动环境及 .env 中不得设置 REDIS_ENTRAID_AUTH_FLOW；置空也不能禁用云认证"},
			},
			Config: map[string]any{"command": "redis-mcp-server", "args": []any{}, "env": map[string]any{
				"REDIS_USERNAME": "default", "REDIS_PWD": "", "MCP_DOCS_SEARCH_URL": "", "REDIS_SSL_CERT_REQS": "required",
			}},
			Inputs: []McpTemplateInput{
				{Key: "redis_url", Required: true, argument: "--url", Labels: map[string]string{"en": "Redis URL", "zh": "Redis 地址"}, Descriptions: map[string]string{"en": "redis:// or rediss:// URL with an optional numeric database, for example rediss://redis.intranet:6379/0. Do not include credentials or query parameters.", "zh": "填写 redis:// 或 rediss:// 地址，可附数字库编号，例如 rediss://redis.intranet:6379/0。不要包含凭证或查询参数。"}},
				{Key: "redis_username", Secret: true, environment: "REDIS_USERNAME", Labels: map[string]string{"en": "Redis ACL username", "zh": "Redis ACL 用户名"}, Descriptions: map[string]string{"en": "Optional ACL username; leave blank to use default.", "zh": "可选 ACL 用户名，留空使用 default。"}},
				{Key: "redis_password", Secret: true, environment: "REDIS_PWD", Labels: map[string]string{"en": "Redis password", "zh": "Redis 密码"}, Descriptions: map[string]string{"en": "Optional password. Blank explicitly means no password; it does not inherit the parent process password.", "zh": "可选密码。留空表示不使用密码，不会继承父进程中的密码。"}},
			},
		},
		{
			Key: "clickhouse", Version: "1", Category: "database",
			DocumentationURL: "https://github.com/ClickHouse/mcp-clickhouse/tree/v0.7.0",
			Titles:           map[string]string{"en": "ClickHouse MCP", "zh": "ClickHouse MCP"},
			Descriptions:     map[string]string{"en": "Query an intranet ClickHouse HTTP endpoint in read-only mode with TLS verification enabled.", "zh": "以只读模式查询内网 ClickHouse HTTP 接口，并保留 TLS 证书验证。"},
			Requirements: map[string][]string{
				"en": {"Preinstall mcp-clickhouse 0.7.0 and Python 3.10 or later; mcp-clickhouse must be on PATH", "Use the HTTP interface, such as http://clickhouse.intranet:8123 or https://clickhouse.intranet:8443; native TCP ports are not supported", "Use a read-only database principal. TLS certificate verification stays enabled; an optional local CA file can be supplied for HTTPS"},
				"zh": {"预装 mcp-clickhouse 0.7.0 和 Python 3.10 或更高版本，并将 mcp-clickhouse 加入 PATH", "使用 HTTP 接口，例如 http://clickhouse.intranet:8123 或 https://clickhouse.intranet:8443；不支持原生 TCP 端口", "使用只读数据库账号。TLS 证书验证保持开启；HTTPS 可提供本地 CA 文件"},
			},
			Config: map[string]any{"command": "mcp-clickhouse", "args": []any{}, "env": map[string]any{
				"CLICKHOUSE_MCP_SERVER_TRANSPORT": "stdio", "CLICKHOUSE_VERIFY": "true", "CLICKHOUSE_ALLOW_WRITE_ACCESS": "false", "CLICKHOUSE_ALLOW_DROP": "false", "CHDB_ENABLED": "false", "CLICKHOUSE_PASSWORD": "", "CLICKHOUSE_CA_CERT": "",
			}},
			Inputs: []McpTemplateInput{
				// The resolver replaces this URL with its host and derives port/TLS fields.
				{Key: "clickhouse_url", Required: true, environment: "CLICKHOUSE_HOST", Labels: map[string]string{"en": "ClickHouse HTTP URL", "zh": "ClickHouse HTTP 地址"}, Descriptions: map[string]string{"en": "HTTP(S) origin only, for example http://clickhouse.intranet:8123. Omitted ports use HTTP 80 or HTTPS 443.", "zh": "只填写 HTTP(S) 服务地址，例如 http://clickhouse.intranet:8123。省略端口时使用 HTTP 80 或 HTTPS 443。"}},
				{Key: "clickhouse_username", Required: true, environment: "CLICKHOUSE_USER", Labels: map[string]string{"en": "ClickHouse username", "zh": "ClickHouse 用户名"}, Descriptions: map[string]string{"en": "Database username, for example default; prefer a read-only principal.", "zh": "数据库用户名，例如 default；建议使用只读账号。"}},
				{Key: "clickhouse_password", Secret: true, environment: "CLICKHOUSE_PASSWORD", Labels: map[string]string{"en": "ClickHouse password", "zh": "ClickHouse 密码"}, Descriptions: map[string]string{"en": "May be blank for an account without a password; blank is passed explicitly.", "zh": "无密码账号可留空；空密码会被明确传入。"}},
				{Key: "clickhouse_ca_path", environment: "CLICKHOUSE_CA_CERT", validator: "absolute_path", Labels: map[string]string{"en": "ClickHouse CA certificate path", "zh": "ClickHouse CA 证书路径"}, Descriptions: map[string]string{"en": "Optional absolute CA certificate path on the agent machine, for HTTPS only.", "zh": "可选，填写智能体机器上的 CA 证书绝对路径，仅用于 HTTPS。"}},
			},
		},
	}
}

// Only built-in identities reach this hook. Deployment recipes retain their
// declared validators and may legitimately use the same keys for other tools.
func resolveIntranetMcpInputs(template McpServerTemplate, inputs map[string]string, config map[string]any) (map[string]any, error) {
	switch template.Key {
	case "gitlab", "atlassian", "grafana", "kubernetes", "mongodb", "redis", "clickhouse":
	default:
		return config, nil
	}
	for _, input := range template.Inputs {
		if value := inputs[input.Key]; value != "" && strings.TrimSpace(value) == "" {
			return nil, fmt.Errorf("%s must not contain only whitespace", input.Key)
		}
	}
	validateHTTP := func(key string) (*url.URL, error) {
		parsed, valid := intranetHTTPURL(inputs[key])
		if !valid {
			return nil, fmt.Errorf("%s must be an HTTP(S) URL without credentials, query parameters, or a fragment", key)
		}
		return parsed, nil
	}
	switch template.Key {
	case "gitlab":
		parsed, err := validateHTTP("gitlab_api_url")
		if err != nil {
			return nil, err
		}
		if !strings.HasSuffix(strings.TrimSuffix(parsed.Path, "/"), "/api/v4") {
			return nil, errors.New("gitlab_api_url must end in /api/v4")
		}
	case "atlassian":
		configured := false
		for _, pair := range [][2]string{{"jira_url", "jira_token"}, {"confluence_url", "confluence_token"}} {
			hasURL, hasToken := inputs[pair[0]] != "", inputs[pair[1]] != ""
			if hasURL != hasToken {
				return nil, fmt.Errorf("%s and %s must be supplied together", pair[0], pair[1])
			}
			if hasURL {
				if _, err := validateHTTP(pair[0]); err != nil {
					return nil, err
				}
				configured = true
			}
		}
		if !configured {
			return nil, errors.New("provide a complete Jira or Confluence URL and token pair")
		}
	case "grafana":
		if _, err := validateHTTP("grafana_url"); err != nil {
			return nil, err
		}
	case "mongodb":
		if !validIntranetMongoURI(inputs["mongodb_uri"]) {
			return nil, errors.New("mongodb_uri must be a valid mongodb:// or mongodb+srv:// URI with TLS certificate verification enabled")
		}
	case "redis":
		parsed, err := url.Parse(inputs["redis_url"])
		if err != nil || (parsed.Scheme != "redis" && parsed.Scheme != "rediss") || !intranetURLAuthority(parsed) || parsed.User != nil || strings.ContainsAny(inputs["redis_url"], "?#") {
			return nil, errors.New("redis_url must be a redis:// or rediss:// URL without credentials, query parameters, or a fragment")
		}
		if parsed.Path != "" && parsed.Path != "/" {
			if !strings.HasPrefix(parsed.Path, "/") || !decimalDigits(strings.TrimPrefix(parsed.Path, "/")) {
				return nil, errors.New("redis_url database must be a non-negative integer")
			}
		}
	case "clickhouse":
		parsed, err := validateHTTP("clickhouse_url")
		if err != nil {
			return nil, err
		}
		if parsed.Path != "" && parsed.Path != "/" {
			return nil, errors.New("clickhouse_url must contain only the HTTP(S) origin, without a path")
		}
		if inputs["clickhouse_ca_path"] != "" && parsed.Scheme != "https" {
			return nil, errors.New("clickhouse_ca_path requires an HTTPS clickhouse_url")
		}
		port := parsed.Port()
		if port == "" {
			port = "80"
			if parsed.Scheme == "https" {
				port = "443"
			}
		}
		env := config["env"].(map[string]any)
		env["CLICKHOUSE_HOST"] = parsed.Hostname()
		env["CLICKHOUSE_PORT"] = port
		env["CLICKHOUSE_SECURE"] = strconv.FormatBool(parsed.Scheme == "https")
	}
	return config, nil
}

func intranetHTTPURL(value string) (*url.URL, bool) {
	parsed, err := url.Parse(value)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || !intranetURLAuthority(parsed) || parsed.User != nil || strings.ContainsAny(value, "?#") {
		return nil, false
	}
	return parsed, true
}

func intranetURLAuthority(parsed *url.URL) bool {
	if parsed.Opaque != "" || parsed.Hostname() == "" || strings.ContainsAny(parsed.Hostname(), " ,\\") {
		return false
	}
	if port := parsed.Port(); port != "" {
		number, err := strconv.Atoi(port)
		return err == nil && number > 0 && number <= 65535
	}
	return !strings.HasSuffix(parsed.Host, ":")
}

func decimalDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, char := range value {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}

// A MongoDB authority may contain several host:port pairs, which must be
// validated separately instead of treating the complete list as one URL host.
func validIntranetMongoURI(value string) bool {
	scheme, rest, ok := strings.Cut(value, "://")
	if !ok || (scheme != "mongodb" && scheme != "mongodb+srv") || strings.Contains(rest, "#") {
		return false
	}
	authority := rest
	suffix := ""
	if end := strings.IndexAny(rest, "/?"); end >= 0 {
		authority, suffix = rest[:end], rest[end:]
	}
	if at := strings.LastIndex(authority, "@"); at >= 0 {
		credentials, err := url.Parse("mongodb://" + authority[:at] + "@localhost")
		if err != nil || credentials.User == nil {
			return false
		}
		authority = authority[at+1:]
	}
	hosts := strings.Split(authority, ",")
	if scheme == "mongodb+srv" && len(hosts) != 1 {
		return false
	}
	for _, host := range hosts {
		parsed, err := url.Parse("mongodb://" + host)
		if err != nil || !intranetURLAuthority(parsed) || parsed.User != nil || (scheme == "mongodb+srv" && parsed.Port() != "") {
			return false
		}
	}
	parsed, err := url.Parse("mongodb://localhost" + suffix)
	if err != nil {
		return false
	}
	query, err := url.ParseQuery(parsed.RawQuery)
	if err != nil {
		return false
	}
	for key, values := range query {
		for _, option := range values {
			switch strings.ToLower(key) {
			case "tlsinsecure", "tlsallowinvalidcertificates", "tlsallowinvalidhostnames":
				if !strings.EqualFold(option, "false") {
					return false
				}
			case "sslvalidate":
				if !strings.EqualFold(option, "true") {
					return false
				}
			}
		}
	}
	return true
}
