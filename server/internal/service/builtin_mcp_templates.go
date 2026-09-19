package service

// Built-in MCP server templates: the product's shortlist of mainstream MCP
// servers a workspace can add with one click.
//
// A template is NOT a new kind of entity. Selecting one pre-fills the ordinary
// "add MCP server" form; saving produces an ordinary workspace MCP server row —
// write-only, renameable, replaceable, deletable, and still unassigned to any
// agent until an owner attaches it. The template's only contribution is the
// starting configuration and its localized picker copy.
//
// Unlike the agent / autopilot registries, these carry NO Version and record NO
// template_key on the created row. Provenance would be dishonest here: the add
// form deliberately lets a user edit the config before saving, so a stored
// "came from template X" stamp could describe a server that runs something else.
// Not recording it also avoids a migration. See design notes in
// .trellis/tasks/09-19-builtin-mcp-presets/design.md.
//
// Config is public, credential-free content, so it is served in full — the
// opposite of the write-only workspace library. The keyless invariant is pinned
// by builtin_mcp_templates_test.go; adding a secret-bearing template must
// change that test first, on purpose.

// McpServerTemplate is one entry in the built-in MCP catalog.
type McpServerTemplate struct {
	// Key is the stable identity and the default server name. It must match the
	// name grammar the add form enforces (^[A-Za-z0-9_-]+$) or a save from the
	// pre-filled form would be rejected client-side.
	//
	// For "chrome-devtools" and "playwright" the key AND the args below are load
	// bearing: server/pkg/agent/browser_mcp_config.go keys its Windows browser
	// fallback on exactly these names / package args. Renaming a key or changing
	// the package token silently disables that fallback.
	Key string
	// Config is the raw MCP server entry copied verbatim into the add form. It
	// must contain "command" or "url" (the form's missing-target rule) and must
	// never carry credential material.
	Config map[string]any
	// Titles / Descriptions are the localized picker copy (en/zh/ko/ja), falling
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

// McpServerTemplates returns the built-in roster in product-controlled order.
//
// A fresh map per entry per call keeps the exported config immutable: a handler
// that marshals these must never be able to mutate the shared roster.
func McpServerTemplates() []McpServerTemplate {
	return []McpServerTemplate{
		{
			Key: "chrome-devtools",
			// Official README standard config. `-y` keeps npx from prompting in
			// a runtime without a TTY. Keyless.
			Config: map[string]any{
				"command": "npx",
				"args":    []any{"-y", "chrome-devtools-mcp@latest"},
			},
			Titles: map[string]string{
				"en": "Chrome DevTools",
				"zh": "Chrome DevTools",
				"ja": "Chrome DevTools",
				"ko": "Chrome DevTools",
			},
			Descriptions: map[string]string{
				"en": "Drive a real Chrome through the DevTools protocol: inspect the DOM, run scripts, and debug page behavior.",
				"zh": "通过 DevTools 协议操控真实 Chrome：检查 DOM、执行脚本、调试页面行为。",
				"ja": "DevTools プロトコルで実際の Chrome を操作します。DOM の確認、スクリプト実行、ページ挙動のデバッグ。",
				"ko": "DevTools 프로토콜로 실제 Chrome을 제어합니다: DOM 검사, 스크립트 실행, 페이지 동작 디버깅.",
			},
		},
		{
			Key: "playwright",
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
				"ja": "Playwright",
				"ko": "Playwright",
			},
			Descriptions: map[string]string{
				"en": "Automate a browser with Playwright: navigate pages, fill forms, and capture snapshots for the agent to act on.",
				"zh": "用 Playwright 自动化浏览器：打开页面、填写表单、抓取快照供智能体处理。",
				"ja": "Playwright でブラウザを自動化します。ページ遷移、フォーム入力、スナップショット取得。",
				"ko": "Playwright로 브라우저를 자동화합니다: 페이지 이동, 폼 입력, 스냅샷 캡처.",
			},
		},
		{
			Key: "sequential-thinking",
			// Official README config, verbatim. Keyless; runs entirely locally.
			Config: map[string]any{
				"command": "npx",
				"args":    []any{"-y", "@modelcontextprotocol/server-sequential-thinking"},
			},
			Titles: map[string]string{
				"en": "Sequential Thinking",
				"zh": "顺序思考",
				"ja": "順序思考",
				"ko": "순차적 사고",
			},
			Descriptions: map[string]string{
				"en": "A tool for breaking a problem into a revisable chain of thoughts, useful for planning and multi-step reasoning.",
				"zh": "把问题拆成可修正的思考链，适合规划和多步推理。",
				"ja": "問題を見直し可能な思考の連なりに分解します。計画立案や多段階の推論に有用です。",
				"ko": "문제를 수정 가능한 사고의 연쇄로 나눕니다. 계획 수립과 다단계 추론에 유용합니다.",
			},
		},
	}
}
