package service

// Built-in MCP server templates: the product's shortlist of mainstream MCP
// servers a workspace can add with one click.
//
// Templates create ordinary, write-only workspace entries through a trusted
// key/version request. The version identifies the recipe, not the executable
// package release. Renames preserve provenance; replacing config clears it.
//
// Config is public, credential-free content, so it is served in full — the
// opposite of the write-only workspace library. The keyless invariant is pinned
// by builtin_mcp_templates_test.go; adding a secret-bearing template must
// change that test first, on purpose.

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
	// Version identifies the configuration recipe, independently of package versions.
	Version          string
	Category         string
	DocumentationURL string
	Requirements     map[string][]string
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
	return []McpServerTemplate{
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
			Key:              "microsoft-learn",
			Version:          "1",
			Category:         "documentation",
			DocumentationURL: "https://learn.microsoft.com/en-us/training/support/mcp",
			Requirements: map[string][]string{
				"en": {"An agent runtime with Streamable HTTP support and network access to learn.microsoft.com", "Public Microsoft documentation only; training and personal profile information are not included"},
				"zh": {"智能体运行时需支持 Streamable HTTP，并能访问 learn.microsoft.com", "仅提供 Microsoft 公开文档，不包含培训内容或个人资料"},
			},
			Config: map[string]any{
				"type": "http",
				"url":  "https://learn.microsoft.com/api/mcp",
			},
			Titles: map[string]string{
				"en": "Microsoft Learn",
				"zh": "Microsoft Learn",
			},
			Descriptions: map[string]string{
				"en": "Find official Microsoft documentation and code examples for Azure, .NET, Microsoft 365, and other products. No API key required.",
				"zh": "查找 Azure、.NET、Microsoft 365 等产品的 Microsoft 官方文档和代码示例，无需 API 密钥。",
			},
		},
		{
			Key:              "deepwiki",
			Version:          "1",
			Category:         "documentation",
			DocumentationURL: "https://docs.devin.ai/work-with-devin/deepwiki-mcp",
			Requirements: map[string][]string{
				"en": {"An agent runtime with Streamable HTTP support and network access to mcp.deepwiki.com", "Public GitHub repositories indexed by DeepWiki only; private repositories are not supported"},
				"zh": {"智能体运行时需支持 Streamable HTTP，并能访问 mcp.deepwiki.com", "仅支持 DeepWiki 已索引的公共 GitHub 仓库，不支持私有仓库"},
			},
			Config: map[string]any{
				"type": "http",
				"url":  "https://mcp.deepwiki.com/mcp",
			},
			Titles: map[string]string{
				"en": "DeepWiki",
				"zh": "DeepWiki",
			},
			Descriptions: map[string]string{
				"en": "Explore documentation and ask questions about public GitHub repositories indexed by DeepWiki. No API key required.",
				"zh": "阅读 DeepWiki 已索引的公共 GitHub 仓库文档，了解项目结构并提问，无需 API 密钥。",
			},
		},
	}
}
