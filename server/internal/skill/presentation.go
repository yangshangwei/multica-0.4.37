package skill

import (
	"fmt"
)

// Presentation metadata for a workspace skill: category and icon.
//
// Stored in skill.config.presentation (JSONB) next to origin and
// template_source. The TypeScript module packages/core/skills/presentation.ts
// is the source of truth for these lists; presentation_parity_test.go keeps
// this mirror identical so a category or icon the UI can pick is never one the
// server rejects.
//
// Skill labels are NOT part of presentation: they are ordinary workspace
// labels (issue_label rows with resource_type = 'skill') attached through
// skill_to_label, managed by the label handlers.

// Categories is the fixed category set, in display order.
var Categories = []string{"research", "writing", "engineering", "operations", "data", "other"}

// DefaultCategory is what a skill without a valid category falls back to.
const DefaultCategory = "other"

// IconNames is the curated Lucide icon whitelist (kebab-case), sorted.
var IconNames = []string{
	"bar-chart",
	"bell",
	"book-open",
	"book-open-text",
	"bot",
	"braces",
	"brain",
	"bug",
	"calendar",
	"chart-line",
	"chart-no-axes-column",
	"chart-pie",
	"clipboard-check",
	"cloud",
	"code",
	"compass",
	"database",
	"file-spreadsheet",
	"file-text",
	"filter",
	"flask-conical",
	"git-branch",
	"git-pull-request",
	"globe",
	"key",
	"landmark",
	"languages",
	"lightbulb",
	"list-checks",
	"lock",
	"mail",
	"megaphone",
	"message-circle-question",
	"microscope",
	"newspaper",
	"package",
	"palette",
	"pen-line",
	"presentation",
	"receipt",
	"repeat",
	"rocket",
	"search",
	"server",
	"shield-check",
	"sparkles",
	"table",
	"terminal",
	"test-tube",
	"timer",
	"workflow",
	"wrench",
}

// CategoryDefaultIcon is the icon drawn when a skill has no explicit override.
var CategoryDefaultIcon = map[string]string{
	"research":    "microscope",
	"writing":     "pen-line",
	"engineering": "code",
	"operations":  "rocket",
	"data":        "database",
	"other":       "book-open-text",
}

var (
	categorySet = toSet(Categories)
	iconSet     = toSet(IconNames)
)

func toSet(values []string) map[string]struct{} {
	out := make(map[string]struct{}, len(values))
	for _, v := range values {
		out[v] = struct{}{}
	}
	return out
}

// IsCategory reports whether value is a known category.
func IsCategory(value string) bool {
	_, ok := categorySet[value]
	return ok
}

// IsIconName reports whether value is on the icon whitelist.
func IsIconName(value string) bool {
	_, ok := iconSet[value]
	return ok
}

// ValidatePresentation checks config["presentation"] on a write path. A nil
// config or an absent/nil presentation passes. Any wrong type or value outside
// the whitelists is an error naming the offending field, suitable for a 400
// body.
func ValidatePresentation(config map[string]any) error {
	if config == nil {
		return nil
	}
	raw, present := config["presentation"]
	if !present || raw == nil {
		return nil
	}
	pres, ok := raw.(map[string]any)
	if !ok {
		return fmt.Errorf("config.presentation must be an object")
	}
	if v, has := pres["category"]; has && v != nil {
		s, isStr := v.(string)
		if !isStr {
			return fmt.Errorf("config.presentation.category must be a string")
		}
		if !IsCategory(s) {
			return fmt.Errorf("config.presentation.category: unknown value %q", s)
		}
	}
	if v, has := pres["icon"]; has && v != nil {
		s, isStr := v.(string)
		if !isStr {
			return fmt.Errorf("config.presentation.icon must be a string")
		}
		if !IsIconName(s) {
			return fmt.Errorf("config.presentation.icon: unknown value %q", s)
		}
	}
	return nil
}

// NormalizePresentation returns a copy of config whose presentation key is in
// canonical form, mirroring writeSkillPresentationMeta on the TS side:
//
//   - icon dropped when it equals the category default or is not whitelisted;
//   - category dropped when unknown; written whenever anything else is present;
//   - unknown keys (including a legacy `tags` array) dropped;
//   - the whole presentation key dropped when nothing is left.
//
// Sibling keys (origin, template_source, ...) are untouched. The input map is
// not mutated. Call after ValidatePresentation; unknown values are silently
// dropped here so this is also safe for lenient paths such as import seeding.
func NormalizePresentation(config map[string]any) map[string]any {
	if config == nil {
		return nil
	}
	out := make(map[string]any, len(config))
	for k, v := range config {
		if k != "presentation" {
			out[k] = v
		}
	}
	pres, ok := config["presentation"].(map[string]any)
	if !ok {
		return out
	}

	category := DefaultCategory
	if s, isStr := pres["category"].(string); isStr && IsCategory(s) {
		category = s
	}
	icon := ""
	if s, isStr := pres["icon"].(string); isStr && IsIconName(s) && s != CategoryDefaultIcon[category] {
		icon = s
	}

	canonical := map[string]any{}
	if category != DefaultCategory {
		canonical["category"] = category
	}
	if icon != "" {
		canonical["icon"] = icon
	}
	if len(canonical) > 0 {
		// Always self-describing once anything is stored, even for the default
		// bucket, matching the TS writer.
		canonical["category"] = category
		out["presentation"] = canonical
	}
	return out
}

// PresentationFromFrontmatter turns author-supplied frontmatter metadata into
// a presentation map for seeding config on import. Invalid values are dropped
// rather than rejected: an import must not fail because of bad metadata in a
// third-party SKILL.md. Returns nil when nothing valid remains.
func PresentationFromFrontmatter(fm Frontmatter) map[string]any {
	pres := map[string]any{}
	if IsCategory(fm.Category) {
		pres["category"] = fm.Category
	}
	if IsIconName(fm.Icon) {
		pres["icon"] = fm.Icon
	}
	normalized := NormalizePresentation(map[string]any{"presentation": pres})
	out, ok := normalized["presentation"].(map[string]any)
	if !ok {
		return nil
	}
	return out
}
