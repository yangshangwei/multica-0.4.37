package service

import (
	"embed"
	"io/fs"
	"path"
	"strings"

	"github.com/multica-ai/multica/server/internal/skill"
)

// Role skills: the reusable "how" that role templates attach on creation.
//
// These are deliberately NOT part of BuiltinSkills(). That set is handed to every
// agent on every claim, so anything added there lengthens every prompt in every
// workspace — the exact failure mode this feature's risk register calls out. A
// role skill is instead MATERIALIZED as an ordinary workspace skill the first time
// a template needs it, and attached to that agent:
//
//   - it shows up on the Skills page, where an admin can read and edit it;
//   - it can be attached to agents the template did not create;
//   - a later release does not overwrite the workspace's edited copy — the
//     materializer reuses a same-named skill as-is and never rewrites it.
//
// Version travels in the skill's config JSON, so a future upgrade path can tell a
// pristine copy from an edited one without a migration.

//go:embed builtin_role_skills
var builtinRoleSkillsFS embed.FS

const builtinRoleSkillsRoot = "builtin_role_skills"

// RoleSkillTemplate is one embedded role skill, ready to be written into a
// workspace.
type RoleSkillTemplate struct {
	// Name is both the embedded directory and the workspace skill name. The
	// "multica-" prefix keeps it from colliding with a skill a user authored, the
	// same convention BuiltinSkills() uses.
	Name string
	// Version is bumped when the content changes materially. Recorded on the
	// materialized row; never used to overwrite one.
	Version int32
	// Description mirrors the SKILL.md frontmatter so the workspace row and the
	// file agree without a second source of truth.
	Description string
	// Category and Icon are the presentation defaults declared in the SKILL.md
	// frontmatter `metadata` block. Written into config.presentation when the
	// skill is materialized; empty for a mounted template that declares none.
	Category string
	Icon     string
	Content  string
	Files    []AgentSkillFileData
}

// builtinRoleSkillVersions is the release-side version of each role skill.
// Declared here rather than parsed out of the file because it is a statement
// about a release, not about the prose: an editorial fix that changes no
// behaviour should not invalidate every workspace's copy.
var builtinRoleSkillVersions = map[string]int32{
	"multica-requirement-clarification":    3,
	"multica-architecture-decision-record": 3,
	"multica-test-report":                  1,
	"multica-code-review":                  1,
	"multica-debugging":                    1,
	"multica-security-review":              1,
	"multica-release-check":                2,
	"multica-documentation-change":         2,
	"multica-progress-report":              1,
}

// RoleSkillTemplateByName loads one role skill from the binary.
func RoleSkillTemplateByName(name string) (RoleSkillTemplate, bool) {
	version, known := builtinRoleSkillVersions[name]
	if !known {
		// Not in the version map means not a role skill this binary ships. Refuse
		// rather than materializing an unversioned skill whose provenance nothing
		// could later explain.
		return RoleSkillTemplate{}, false
	}
	dir := path.Join(builtinRoleSkillsRoot, name)
	content, err := fs.ReadFile(builtinRoleSkillsFS, path.Join(dir, "SKILL.md"))
	if err != nil {
		return RoleSkillTemplate{}, false
	}
	fm := skill.ParseSkillFrontmatterMeta(string(content))
	loaded := RoleSkillTemplate{
		Name:        name,
		Version:     version,
		Description: fm.Description,
		Category:    fm.Category,
		Icon:        fm.Icon,
		Content:     string(content),
	}
	// Supporting files keep their relative path so nested references survive,
	// matching loadBuiltinSkill's behaviour.
	_ = fs.WalkDir(builtinRoleSkillsFS, dir, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil || d.IsDir() {
			return walkErr
		}
		rel := strings.TrimPrefix(p, dir+"/")
		if rel == "SKILL.md" {
			return nil
		}
		data, readErr := fs.ReadFile(builtinRoleSkillsFS, p)
		if readErr != nil {
			return nil
		}
		loaded.Files = append(loaded.Files, AgentSkillFileData{Path: rel, Content: string(data)})
		return nil
	})
	return loaded, true
}

// RoleSkillTemplates returns every role skill this binary ships, in a stable
// order so tests and listings do not depend on map iteration.
func RoleSkillTemplates() []RoleSkillTemplate {
	names := make([]string, 0, len(builtinRoleSkillVersions))
	for name := range builtinRoleSkillVersions {
		names = append(names, name)
	}
	sortStrings(names)
	out := make([]RoleSkillTemplate, 0, len(names))
	for _, name := range names {
		if loaded, ok := RoleSkillTemplateByName(name); ok {
			out = append(out, loaded)
		}
	}
	return out
}

// sortStrings is a local insertion sort to keep this file free of a sort import
// for one call; the slice is nine entries long.
func sortStrings(values []string) {
	for i := 1; i < len(values); i++ {
		for j := i; j > 0 && values[j] < values[j-1]; j-- {
			values[j], values[j-1] = values[j-1], values[j]
		}
	}
}
