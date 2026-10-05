package service

// Built-in MCP server templates: the product's shortlist of mainstream MCP
// servers a workspace can add with one click.
//
// Templates create ordinary, write-only workspace entries through a trusted
// key/version request and validated inputs. The version identifies the recipe, not the executable
// package release. Renames preserve provenance; replacing config clears it.
//
// Config is public, credential-free content, so it is served in full — the
// opposite of the write-only workspace library. The keyless invariant is pinned
// by builtin_mcp_templates_test.go. Credentials are collected separately as
// write-only inputs and never appear in the public recipe.

// McpServerTemplate is one entry in the built-in MCP catalog.
type McpServerTemplate struct {
	// Key is the stable identity and the default server name. It must match the
	// name grammar enforced by the server and add form (^[A-Za-z0-9_-]+$).
	//
	// For "chrome-devtools" and "playwright" the key AND the args below are load
	// bearing: server/pkg/agent/browser_mcp_config.go keys its Windows browser
	// fallback on exactly these names / package args. Renaming a key or changing
	// the package token silently disables that fallback.
	Key string
	// Source and Transport identify catalog provenance without exposing a recipe.
	Source    string
	Transport string
	// Version identifies the configuration recipe, independently of package versions.
	Version          string
	Category         string
	DocumentationURL string
	Requirements     map[string][]string
	Inputs           []McpTemplateInput
	// Config is the raw MCP server entry previewed publicly and copied by the
	// trusted creation handler. It must contain "command" or "url" and must
	// never carry credential material.
	Config map[string]any
	// Titles / Descriptions are the localized picker copy (en/zh), falling
	// back to English then the key. Unlike the config, this is display-only.
	Titles       map[string]string
	Descriptions map[string]string
}

// Title returns the localized catalog label, falling back to English then Key.
func (t McpServerTemplate) Title(language string) string {
	return localizedTemplateString(t.Titles, language, t.Key)
}

// Description returns the localized catalog description, falling back to English.
func (t McpServerTemplate) Description(language string) string {
	return localizedTemplateString(t.Descriptions, language, "")
}

// RequirementLabels returns runtime prerequisites in the requested language.
func (t McpServerTemplate) RequirementLabels(language string) []string {
	if requirements, ok := t.Requirements[language]; ok {
		return requirements
	}
	return t.Requirements["en"]
}

// McpServerTemplates returns the built-in roster in product-controlled order.
//
// A fresh map per entry per call keeps the exported config immutable: a handler
// that marshals these must never be able to mutate the shared roster.
func McpServerTemplates() []McpServerTemplate {
	return append([]McpServerTemplate{
		{
			Key:              "chrome-devtools",
			Version:          "1",
			Category:         "browser",
			DocumentationURL: "https://github.com/ChromeDevTools/chrome-devtools-mcp",
			Requirements: map[string][]string{
				"en": {"Node.js and npx on the agent runtime", "Google Chrome installed on the agent runtime"},
				"zh": {"智能体运行时需安装 Node.js 和 npx", "智能体运行时需安装 Google Chrome"},
			},
			// Official README standard config. `-y` keeps npx from prompting in
			// a runtime without a TTY. Keyless.
			Config: map[string]any{
				"command": "npx",
				"args":    []any{"-y", "chrome-devtools-mcp@latest"},
			},
			Titles: map[string]string{
				"en": "Chrome DevTools",
				"zh": "Chrome DevTools",
			},
			Descriptions: map[string]string{
				"en": "Inspect pages and debug Chrome: examine the DOM, run scripts, and troubleshoot page behavior through DevTools.",
				"zh": "检查页面、调试浏览器：通过 DevTools 查看 DOM、执行脚本、排查页面行为。",
			},
		},
		{
			Key:              "playwright",
			Version:          "1",
			Category:         "browser",
			DocumentationURL: "https://github.com/microsoft/playwright-mcp",
			Requirements: map[string][]string{
				"en": {"Node.js and npx on the agent runtime", "Browser binaries available to the agent runtime"},
				"zh": {"智能体运行时需安装 Node.js 和 npx", "智能体运行时需具备可用的浏览器程序"},
			},
			// Upstream shows `npx @playwright/mcp@latest`; we add `-y` because the
			// agent runtime has no TTY and npx would otherwise block on a prompt.
			// Keyless.
			Config: map[string]any{
				"command": "npx",
				"args":    []any{"-y", "@playwright/mcp@latest"},
			},
			Titles: map[string]string{
				"en": "Playwright",
				"zh": "Playwright",
			},
			Descriptions: map[string]string{
				"en": "Automate web interactions: use Playwright to navigate pages, fill forms, and capture snapshots for the agent.",
				"zh": "自动操作网页：用 Playwright 打开页面、填写表单、抓取快照供智能体处理。",
			},
		},
		{
			Key:              "sequential-thinking",
			Version:          "1",
			Category:         "reasoning",
			DocumentationURL: "https://github.com/modelcontextprotocol/servers/tree/main/src/sequentialthinking",
			Requirements: map[string][]string{
				"en": {"Node.js and npx on the agent runtime"},
				"zh": {"智能体运行时需安装 Node.js 和 npx"},
			},
			// Official README config, verbatim. Keyless; runs entirely locally.
			Config: map[string]any{
				"command": "npx",
				"args":    []any{"-y", "@modelcontextprotocol/server-sequential-thinking"},
			},
			Titles: map[string]string{
				"en": "Sequential Thinking",
				"zh": "顺序思考",
			},
			Descriptions: map[string]string{
				"en": "A tool for breaking a problem into a revisable chain of thoughts, useful for planning and multi-step reasoning.",
				"zh": "把问题拆成可修正的思考链，适合规划和多步推理。",
			},
		},
		{
			Key: "serena", Version: "1", Category: "coding",
			DocumentationURL: "https://github.com/oraios/serena",
			Requirements: map[string][]string{
				"en": {"uv/uvx and Python 3.13 on the agent runtime; uv can download Python", "A local project directory on the agent runtime; some languages require a language server", "Package download access is needed on first launch"},
				"zh": {"智能体运行环境需安装 uv/uvx，并提供 Python 3.13；uv 可自动下载 Python", "需填写智能体运行环境上的项目目录；部分语言需要语言服务器", "首次启动需要联网下载软件包"},
			},
			Config: map[string]any{"command": "uvx", "args": []any{"--python", "3.13", "--from", "serena-agent", "serena", "start-mcp-server"}},
			Inputs: []McpTemplateInput{{
				Key: "project_path", Required: true, argument: "--project",
				Labels:       map[string]string{"en": "Project directory", "zh": "项目目录"},
				Descriptions: map[string]string{"en": "Enter the project directory on the machine running the agent, such as /work/my-project or C:\\work\\my-project.", "zh": "填写智能体所在机器上的项目目录，例如 /work/my-project 或 C:\\work\\my-project。"},
			}},
			Titles:       map[string]string{"en": "Serena", "zh": "Serena"},
			Descriptions: map[string]string{"en": "Understand and edit code through symbols, references, and language-aware navigation in a local project.", "zh": "按符号和引用理解本地项目，定位代码并进行精准编辑。"},
		},
		{
			Key: "codebase-memory", Version: "1", Category: "coding",
			DocumentationURL: "https://github.com/DeusData/codebase-memory-mcp",
			Requirements: map[string][]string{
				"en": {"Install the official codebase-memory-mcp release and its runtime assets on the agent machine, and make the executable available on PATH", "Supports macOS, Linux, and Windows; needs access to the local project files"},
				"zh": {"需在智能体所在机器安装官方 codebase-memory-mcp 程序及配套运行资源，并将程序加入 PATH", "支持 macOS、Linux 和 Windows；需能访问本地项目文件"},
			},
			Config:       map[string]any{"command": "codebase-memory-mcp", "args": []any{}},
			Titles:       map[string]string{"en": "Codebase Memory MCP", "zh": "Codebase Memory MCP"},
			Descriptions: map[string]string{"en": "Build a local code knowledge graph to inspect call chains, dependencies, and the impact of changes.", "zh": "建立本地代码知识图谱，分析调用链、模块依赖和改动影响。"},
		},
		{
			Key: "repomix", Version: "1", Category: "coding",
			DocumentationURL: "https://github.com/yamadashy/repomix",
			Requirements: map[string][]string{
				"en": {"Node.js 22 or later and npx on the agent runtime", "Local repository access; package download access is needed on first launch"},
				"zh": {"智能体运行环境需安装 Node.js 22 或更高版本及 npx", "需能访问本地仓库；首次启动需要联网下载软件包"},
			},
			Config:       map[string]any{"command": "npx", "args": []any{"-y", "repomix", "--mcp"}},
			Titles:       map[string]string{"en": "Repomix", "zh": "Repomix"},
			Descriptions: map[string]string{"en": "Package and search a repository to give the agent structured project context.", "zh": "整理、打包和检索代码仓库，为智能体提供完整的项目上下文。"},
		},
		{
			Key: "markitdown", Version: "1", Category: "documentation",
			DocumentationURL: "https://github.com/microsoft/markitdown/tree/main/packages/markitdown-mcp",
			Requirements: map[string][]string{
				"en": {"Python 3.10 or later and uv/uvx on the agent runtime", "Local document access; package download access is needed on first launch"},
				"zh": {"智能体运行环境需安装 Python 3.10 或更高版本及 uv/uvx", "需能访问本地文档；首次启动需要联网下载软件包"},
			},
			Config:       map[string]any{"command": "uvx", "args": []any{"markitdown-mcp"}},
			Titles:       map[string]string{"en": "MarkItDown MCP", "zh": "MarkItDown MCP"},
			Descriptions: map[string]string{"en": "Convert PDF, Word, Excel, and PowerPoint files to Markdown for requirements and technical-document analysis.", "zh": "将 PDF、Word、Excel 和 PPT 转成 Markdown，便于读取需求和技术资料。"},
		},
		{
			Key: "dbhub", Version: "1", Category: "database",
			DocumentationURL: "https://github.com/bytebase/dbhub",
			Requirements: map[string][]string{
				"en": {"Node.js 22.5 or later and npx on the agent runtime", "A database connection URL accessible from the agent runtime; use a read-only account when only querying", "Package download access is needed on first launch"},
				"zh": {"智能体运行环境需安装 Node.js 22.5 或更高版本及 npx", "需填写智能体运行环境可访问的数据库连接地址；仅查询时建议使用只读账号", "首次启动需要联网下载软件包"},
			},
			Config:       map[string]any{"command": "npx", "args": []any{"-y", "@bytebase/dbhub@latest", "--transport", "stdio"}},
			Inputs:       []McpTemplateInput{databaseMcpTemplateInput("DSN")},
			Titles:       map[string]string{"en": "DBHub", "zh": "DBHub"},
			Descriptions: map[string]string{"en": "Inspect schemas, run SQL, and analyze query plans across databases including PostgreSQL, MySQL, and SQLite.", "zh": "查看表结构、执行 SQL 和分析执行计划，支持 PostgreSQL、MySQL、SQLite 等数据库。"},
		},
		{
			Key: "postgres-mcp", Version: "2", Category: "database",
			DocumentationURL: "https://github.com/crystaldba/postgres-mcp",
			Requirements: map[string][]string{
				"en": {"Python 3.12 or later and uv/uvx on the agent runtime", "A PostgreSQL connection URL accessible from the agent runtime; this template uses restricted, read-only mode", "Package download access is needed on first launch"},
				"zh": {"智能体运行环境需安装 Python 3.12 或更高版本及 uv/uvx", "需填写智能体运行环境可访问的 PostgreSQL 连接地址；此模板使用受限只读模式", "首次启动需要联网下载软件包"},
			},
			// postgres-mcp imports mcp.server.fastmcp, which MCP SDK v2 removed.
			// Keep the compatible SDK major until upstream supports the new API.
			Config:       map[string]any{"command": "uvx", "args": []any{"--with", "mcp<2", "postgres-mcp", "--access-mode=restricted"}},
			Inputs:       []McpTemplateInput{databaseMcpTemplateInput("DATABASE_URI")},
			Titles:       map[string]string{"en": "Postgres MCP Pro", "zh": "Postgres MCP Pro"},
			Descriptions: map[string]string{"en": "Check PostgreSQL health and investigate slow queries, execution plans, and index recommendations in read-only mode.", "zh": "以只读模式检查 PostgreSQL 健康状况，分析慢查询、执行计划和索引优化建议。"},
		},
	}, builtinIntranetMcpTemplates()...)
}
