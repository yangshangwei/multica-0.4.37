package service

import (
	"embed"
	"path"
	"strings"
)

// Built-in autopilot templates: the product's opinion about which recurring
// automations a team should be able to stand up in a couple of clicks.
//
// A template is NOT a new kind of entity. Creating from one produces an ordinary
// autopilot plus an ordinary schedule trigger — same tables, same scheduler, same
// dispatch — whose prompt, cadence and execution mode were COPIED from the
// template at creation time. The template's only lasting contribution is content
// and provenance: the starting prompt, the cron expression, the execution mode,
// and the template_key / template_version stamped on the created autopilot row so
// a later release can tell where a copy came from.
//
// This mirrors builtin_agent_templates.go exactly: the workspace owns the copy and
// may edit the prompt or the cadence afterwards; a release that rewrites a template
// never overwrites that edit. The localized picker copy (Titles / Descriptions /
// Categories) is server-rendered; the PROMPT.md bodies stay English-only, matching
// every other agent-harness text in this repo.

//go:embed builtin_autopilot_templates
var builtinAutopilotTemplatesFS embed.FS

const builtinAutopilotTemplatesRoot = "builtin_autopilot_templates"

// AutopilotTemplate is one entry in the built-in autopilot roster. Creating from
// one produces an ordinary autopilot + schedule trigger; the template only
// supplies the starting prompt, cadence and execution mode, plus the
// template_key / template_version provenance recorded on the created row.
type AutopilotTemplate struct {
	// Key is the stable identity, recorded on the created autopilot. Never
	// localized, never renamed — a rename would orphan the provenance of every
	// autopilot already created from it.
	Key string
	// Version is bumped whenever Prompt, CronExpression or ExecutionMode change in
	// a way a workspace would want to know about. Stored on the created autopilot
	// so a later release can offer an upgrade diff.
	Version int32
	// CronExpression is the 5-field standard cron the schedule trigger starts
	// from. Must pass service.ComputeNextRun; a workspace may edit it afterwards.
	CronExpression string
	// ExecutionMode is "create_issue" (a summary every period lands in a fresh
	// issue) or "run_only" (the agent runs and only files an issue when it finds
	// something worth one).
	ExecutionMode string
	// IssueTitleTemplate is the starting issue title for create_issue runs.
	// Non-empty only for create_issue templates — run_only never creates the
	// issue itself, so there is no title to template. Like the prompt and the
	// cron, this is content COPIED into the created autopilot, which the
	// workspace may edit afterwards. `{{date}}` is the only supported
	// placeholder (see ValidateIssueTitleTemplate); without it a daily summary
	// files thirty identically-named issues in a month.
	IssueTitleTemplate string
	// AvatarEmoji decorates the picker card. It is picker-only: the autopilot
	// table has no avatar column, so the created instance does not carry it and
	// no surface outside the template picker ever renders it.
	AvatarEmoji string
	// Category is a stable english slug grouping the picker cards (e.g.
	// "repo-health"). Localized labels live in Categories; nothing keys off the
	// localized text.
	Category string
	// Titles, Descriptions and Categories are the localized picker copy,
	// en/zh/ko/ja. The prompt body stays English-only (see PROMPT.md).
	Titles       map[string]string
	Descriptions map[string]string
	Categories   map[string]string
	// Listed is whether the picker shows the template. Every current template is
	// listed; the field exists so an unfinished template can ship dark.
	Listed bool
}

// Prompt returns the automation prompt copied into the created autopilot at
// creation. Embedded rather than inlined so the prose is reviewable as prose.
func (t AutopilotTemplate) Prompt() string {
	body, err := builtinAutopilotTemplatesFS.ReadFile(path.Join(builtinAutopilotTemplatesRoot, t.Key, "PROMPT.md"))
	if err != nil {
		// Unreachable in a built binary: the embed directive fails the build if
		// the directory is missing, and every Key is covered by a test that reads
		// its PROMPT.md. Returning empty rather than panicking keeps a malformed
		// backport from taking down template listing entirely.
		return ""
	}
	return strings.TrimRight(string(body), "\n")
}

// Title returns the localized card title, falling back to English.
func (t AutopilotTemplate) Title(language string) string {
	return localizedTemplateString(t.Titles, language, t.Key)
}

// Description returns the localized card description, falling back to English.
func (t AutopilotTemplate) Description(language string) string {
	return localizedTemplateString(t.Descriptions, language, "")
}

// CategoryLabel returns the localized category label, falling back to English
// and then to the stable Category slug.
func (t AutopilotTemplate) CategoryLabel(language string) string {
	return localizedTemplateString(t.Categories, language, t.Category)
}
