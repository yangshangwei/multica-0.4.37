package service

import (
	"embed"
	"path"
	"strings"
)

// Built-in role templates: the product's opinion about which engineering roles a
// team should be able to staff in one click.
//
// A template is NOT a new kind of agent. Creating from one produces an ordinary
// workspace agent — same table, same Access, same runtime, same task lifecycle —
// whose instructions were COPIED from the template at creation time. That copy is
// the whole contract:
//
//   - the workspace owns the text and may edit it freely;
//   - a later release that rewrites a template can never overwrite that edit;
//   - agent.template_key / template_version record where the copy came from, so a
//     future upgrade can show a diff instead of silently replacing prose a team
//     has tuned.
//
// This is deliberately the opposite trade-off from Mika (builtin_agents.go),
// whose prompt is layered in from the server binary on every claim and therefore
// hot-updatable but not editable. Mika is one product-owned assistant; these are
// eight starting points a team is expected to make their own.

//go:embed builtin_agent_templates
var builtinAgentTemplatesFS embed.FS

const builtinAgentTemplatesRoot = "builtin_agent_templates"

// AutonomyLevel is the enforceable half of a role: how far an agent may act on
// its own before a human has to be in the loop.
//
// The four levels are cumulative — each one may do everything the level below it
// may do. They are stored on the agent row (agent.autonomy_level), enforced on
// agent-actor API requests (agent_autonomy.go), and injected into every claimed
// task's brief so the agent is told the same rule the server will apply.
type AutonomyLevel string

const (
	// AutonomyObserver reads, analyses and comments. It must not change what the
	// workspace is doing: no status changes, no assignment, no new work items.
	AutonomyObserver AutonomyLevel = "observer"
	// AutonomyContributor does the work: writes code on an isolated branch, runs
	// tests, moves the issues it was given. It must not operate production or
	// create standing automation.
	AutonomyContributor AutonomyLevel = "contributor"
	// AutonomyCoordinator routes work: creates sub-issues, mentions members,
	// drives a squad, manages automation.
	AutonomyCoordinator AutonomyLevel = "coordinator"
	// AutonomyOperator may carry out high-risk operations — but only ones a human
	// has approved, one approval per action (agent_approval.go).
	AutonomyOperator AutonomyLevel = "operator"
)

// autonomyRank orders the levels for "at least this level" comparisons. Absent
// from the map means unrecognised, which callers must treat as no declared
// policy rather than as level zero — see AutonomyAtLeast.
var autonomyRank = map[AutonomyLevel]int{
	AutonomyObserver:    1,
	AutonomyContributor: 2,
	AutonomyCoordinator: 3,
	AutonomyOperator:    4,
}

// IsKnownAutonomyLevel reports whether the value is one of the four levels. The
// empty string is not: it means "no policy declared" and is the value every
// agent created before this feature carries.
func IsKnownAutonomyLevel(level string) bool {
	_, ok := autonomyRank[AutonomyLevel(level)]
	return ok
}

// AutonomyAtLeast reports whether have is at least as permissive as want.
//
// An undeclared level (empty, or a value this binary does not recognise) returns
// true for every want. That is the compatibility rule, and it is deliberate: the
// column is empty for every pre-existing agent, and a downgrade-by-default
// would silently break workspaces whose agents have been doing this work for
// months. Policy applies to agents that opted into a role, and to those a
// workspace explicitly assigns a level.
func AutonomyAtLeast(have string, want AutonomyLevel) bool {
	haveRank, known := autonomyRank[AutonomyLevel(have)]
	if !known {
		return true
	}
	wantRank, known := autonomyRank[want]
	if !known {
		return true
	}
	return haveRank >= wantRank
}

// AgentRoleTemplate is one entry in the built-in roster.
type AgentRoleTemplate struct {
	// Key is the stable identity, recorded on created agents. Never localized,
	// never renamed — a rename would orphan the provenance of every agent
	// already created from it.
	Key string
	// Version is bumped whenever Instructions, RoleSkills or Autonomy change in a
	// way a workspace would want to know about. Stored on the created agent so a
	// later release can offer an upgrade diff.
	Version int32
	// DefaultName seeds the agent's display name. Owners rename freely; nothing
	// server-side keys off it.
	DefaultName string
	// AvatarEmoji is rendered through the same `emoji:` marker every other agent
	// avatar uses, so no surface needs to special-case a template agent.
	AvatarEmoji string
	// Autonomy is the level a fresh copy starts at. A workspace may raise or
	// lower it afterwards; the template only chooses the default.
	Autonomy AutonomyLevel
	// MaxConcurrentTasks is the per-agent scheduler cap. Reviewers and analysts
	// are cheap and bursty; an implementer holding a working tree is not.
	MaxConcurrentTasks int32
	// RoleSkills names built-in role skills (builtin_role_skills/<name>) that are
	// materialized as workspace skills and attached on creation. Kept to one per
	// role on purpose: the failure mode of this feature is prompt bloat, and the
	// instructions already carry the role's own contract.
	RoleSkills []string
	// Titles and Descriptions are the localized picker copy. Instructions stay
	// English-only, matching every other agent-harness text in this repo.
	Titles       map[string]string
	Descriptions map[string]string
	// Listed is false for the squad-leader definitions. They are real templates —
	// same provenance, same copy semantics — but they are provisioned by the squad
	// flow, which supplies the roster that makes a coordinator meaningful. Offering
	// them in the agent picker would hand someone a leader with nobody to lead.
	Listed bool
}

// Instructions returns the role prompt copied into agent.instructions at
// creation. Embedded rather than inlined so the prose is reviewable as prose.
func (t AgentRoleTemplate) Instructions() string {
	body, err := builtinAgentTemplatesFS.ReadFile(path.Join(builtinAgentTemplatesRoot, t.Key, "INSTRUCTIONS.md"))
	if err != nil {
		// Unreachable in a built binary: the embed directive fails the build if
		// the directory is missing, and agentRoleTemplateKeys is covered by a
		// test that reads every file. Returning empty rather than panicking keeps
		// a malformed backport from taking down agent creation entirely.
		return ""
	}
	return strings.TrimRight(string(body), "\n")
}

// Title returns the localized role label, falling back to English.
func (t AgentRoleTemplate) Title(language string) string {
	return localizedTemplateString(t.Titles, language, t.DefaultName)
}

// Description returns the localized picker description, falling back to English.
func (t AgentRoleTemplate) Description(language string) string {
	return localizedTemplateString(t.Descriptions, language, "")
}

func localizedTemplateString(values map[string]string, language, fallback string) string {
	if v, ok := values[language]; ok && v != "" {
		return v
	}
	if v, ok := values["en"]; ok && v != "" {
		return v
	}
	return fallback
}

// TemplateLanguages are the locales the picker copy is translated into. Matches
// Mika's set (mika_agent.go) so the two built-in surfaces stay in step.
var TemplateLanguages = []string{"en", "zh", "ko", "ja"}

// IsSupportedTemplateLanguage reports whether the locale has its own copy. An
// unsupported value is not an error at the API boundary — it falls back to
// English — but the request validator uses this to reject obvious typos.
func IsSupportedTemplateLanguage(language string) bool {
	for _, candidate := range TemplateLanguages {
		if candidate == language {
			return true
		}
	}
	return false
}
