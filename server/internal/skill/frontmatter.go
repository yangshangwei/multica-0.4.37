// Package skill provides shared utilities for working with SKILL.md files.
package skill

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Keeping the trailing newline inside group 1 matters: yaml.v3's `|` clip
// chomping only preserves a final newline when the input itself contains one.
var frontmatterPattern = regexp.MustCompile(`(?s)\A---\r?\n(.*?\r?\n)---`)

// ParseSkillFrontmatter extracts name and description from the YAML frontmatter
// block of a SKILL.md file. Returns empty strings when the frontmatter is
// absent or malformed so callers can keep treating missing metadata as a
// non-fatal condition, matching the behaviour of the legacy line-based parser.
//
// Values are decoded into a generic map and coerced per key (scalars via their
// literal form, sequences/mappings via JSON) rather than unmarshalled into a
// string struct. This means a structured value in one field never discards a
// valid sibling key, and the coercion mirrors the TS parseFrontmatter in
// packages/core/skills/frontmatter.ts so both sides agree on the same input.
func ParseSkillFrontmatter(content string) (name, description string) {
	fm := ParseSkillFrontmatterMeta(content)
	return fm.Name, fm.Description
}

// Frontmatter is the structured view of a SKILL.md header. Name and
// Description come from the top-level keys; Category and Icon come from the
// optional `metadata:` mapping, which is where author-supplied presentation
// hints live so they never collide with the reserved top-level keys. Values
// are passed through as written — validation against the category/icon
// whitelists happens in presentation.go. A `metadata.tags` entry is ignored:
// skill labels are workspace labels, never frontmatter.
type Frontmatter struct {
	Name        string
	Description string
	Category    string
	Icon        string
}

// ParseSkillFrontmatterMeta parses the frontmatter block into a Frontmatter.
// Absent or malformed frontmatter yields the zero value.
func ParseSkillFrontmatterMeta(content string) Frontmatter {
	if !strings.HasPrefix(content, "---") {
		return Frontmatter{}
	}
	match := frontmatterPattern.FindStringSubmatch(content)
	if match == nil {
		return Frontmatter{}
	}

	var fm map[string]any
	if err := yaml.Unmarshal([]byte(match[1]), &fm); err != nil {
		return Frontmatter{}
	}
	// Trimmed because both fields are single-line labels wherever they are
	// consumed, while YAML block scalars (`description: |`, `description: >`)
	// carry a trailing newline by clip chomping. Storing that newline made the
	// imported skill differ from its own trimmed form, which the skill detail
	// page read as an unsaved edit (MUL-5645). Normalize at the parse seam so
	// no import path has to remember to.
	out := Frontmatter{
		Name:        strings.TrimSpace(coerceFrontmatterValue(fm["name"])),
		Description: strings.TrimSpace(coerceFrontmatterValue(fm["description"])),
	}
	meta, ok := fm["metadata"].(map[string]any)
	if !ok {
		return out
	}
	if s, isStr := meta["category"].(string); isStr {
		out.Category = strings.TrimSpace(s)
	}
	if s, isStr := meta["icon"].(string); isStr {
		out.Icon = strings.TrimSpace(s)
	}
	return out
}

// coerceFrontmatterValue renders a decoded YAML value as a string, mirroring the
// TS side: nil becomes empty, strings pass through, other scalars use their
// literal form, and structured values (sequences/mappings) are JSON-encoded.
func coerceFrontmatterValue(v any) string {
	switch val := v.(type) {
	case nil:
		return ""
	case string:
		return val
	case bool:
		return strconv.FormatBool(val)
	case int:
		return strconv.Itoa(val)
	case int64:
		return strconv.FormatInt(val, 10)
	case uint64:
		return strconv.FormatUint(val, 10)
	case float64:
		return strconv.FormatFloat(val, 'g', -1, 64)
	default:
		encoded, err := json.Marshal(val)
		if err != nil {
			return ""
		}
		return string(encoded)
	}
}
