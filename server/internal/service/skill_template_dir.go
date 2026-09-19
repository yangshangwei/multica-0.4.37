package service

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"sort"

	"github.com/multica-ai/multica/server/internal/skill"
)

// Mounted skill templates: templates an operator drops into a server directory
// (MULTICA_SKILL_TEMPLATE_DIR) so an intranet deployment can offer its own
// starting content in "new skill → start from a template" without a code
// change, a rebuild, or a migration. The layout mirrors builtin_role_skills:
// one first-level directory per template, holding a SKILL.md and optional
// supporting files.
//
// A mounted template is still just pre-fill content copied into an ordinary
// workspace skill on creation — never a runtime built-in, never a new entity
// kind, and never platform-official provenance. It is intentionally kept out of
// BuiltinSkills() for the same reason the embedded role skills are.

const (
	// mountedSkillTemplateVersion marks a mounted template as non-release
	// content. Unlike an embedded role skill, nothing about a file an operator
	// dropped in corresponds to a shipped version, so it carries the zero value.
	mountedSkillTemplateVersion int32 = 0

	// Bundle caps mirror the URL/archive import limits (handler.maxImport*): a
	// single oversized or file-heavy template is skipped rather than allowed to
	// bloat every catalog read.
	maxSkillTemplateFileSize  = 1 << 20 // 1 MiB per file
	maxSkillTemplateTotalSize = 8 << 20 // 8 MiB summed across supporting files
	maxSkillTemplateFileCount = 256     // supporting files per template
)

// skillTemplateNamePattern is the skill-name grammar the add form enforces. It
// also forbids the separators and dot-prefix a directory-traversal name would
// need, so validating against it is both the name check and the first line of
// path-safety defense.
var skillTemplateNamePattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

// SkillTemplates returns the templates offered in "new skill → start from a
// template": every embedded role skill first, then any well-formed template
// found in the mounted directory. When SkillTemplateDir is empty the result is
// byte-for-byte the embedded catalog, so an unconfigured deployment behaves
// exactly as before.
func (s *TaskService) SkillTemplates() []RoleSkillTemplate {
	embed := RoleSkillTemplates()
	if s == nil || s.SkillTemplateDir == "" {
		return embed
	}
	embedNames := make(map[string]struct{}, len(embed))
	for _, template := range embed {
		embedNames[template.Name] = struct{}{}
	}
	mounted := scanSkillTemplateDir(s.SkillTemplateDir, embedNames)
	// Embedded catalog stays first; mounted entries follow in their own stable
	// order. A caller relying on the leading run of platform templates keeps it.
	return append(embed, mounted...)
}

// scanSkillTemplateDir reads mounted skill templates from dir, in stable name
// order. It is defensive by construction: any single malformed entry is skipped
// with a warning instead of failing the whole scan, so one bad folder cannot
// take down the template catalog. embedNames carries the names the embedded
// registry already ships; a mounted entry that collides is skipped so platform
// content always wins.
func scanSkillTemplateDir(dir string, embedNames map[string]struct{}) []RoleSkillTemplate {
	if dir == "" {
		return nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		// A missing or empty directory is a valid "nothing mounted" state, not an
		// error to surface. Anything else is worth a warning but still non-fatal.
		if !errors.Is(err, os.ErrNotExist) {
			slog.Warn("skill templates: mounted directory could not be read", "dir", dir, "error", err)
		}
		return nil
	}
	out := make([]RoleSkillTemplate, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if !skillTemplateNamePattern.MatchString(name) {
			slog.Warn("skill templates: skipping entry with an invalid name", "dir", dir, "name", name)
			continue
		}
		// Refuse a symlinked top-level entry outright. entry.Type() reports the
		// link's own mode without following it, so a symlink pointing at an
		// external directory never has its SKILL.md read — the same "no symlinks"
		// stance collectMountedTemplateFiles applies to supporting files (R-1).
		if entry.Type()&os.ModeSymlink != 0 {
			slog.Warn("skill templates: skipping symlinked entry", "dir", dir, "name", name)
			continue
		}
		if !entry.IsDir() {
			slog.Warn("skill templates: skipping non-directory entry", "dir", dir, "name", name)
			continue
		}
		if _, clash := embedNames[name]; clash {
			// D4: embed wins. An operator who wants to change a platform template
			// ships it under a different name; a same-named mount cannot shadow it.
			slog.Warn("skill templates: mounted entry shadows a built-in role skill and was skipped", "name", name)
			continue
		}
		template, ok := loadMountedSkillTemplate(dir, name)
		if !ok {
			continue
		}
		out = append(out, template)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// loadMountedSkillTemplate reads one template directory into a RoleSkillTemplate,
// returning ok=false (with a warning) for any malformed entry.
func loadMountedSkillTemplate(dir, name string) (RoleSkillTemplate, bool) {
	root := filepath.Join(dir, name)
	content, err := os.ReadFile(filepath.Join(root, skill.ContentFilename))
	if err != nil {
		slog.Warn("skill templates: entry is missing a readable SKILL.md", "name", name, "error", err)
		return RoleSkillTemplate{}, false
	}
	if len(content) > maxSkillTemplateFileSize {
		slog.Warn("skill templates: SKILL.md exceeds the per-file size limit", "name", name, "bytes", len(content))
		return RoleSkillTemplate{}, false
	}
	fmName, description := skill.ParseSkillFrontmatter(string(content))
	if fmName == "" {
		// The directory name is authoritative for the template name, but an entry
		// whose frontmatter carries no name is malformed content and is skipped.
		slog.Warn("skill templates: SKILL.md frontmatter has no name", "name", name)
		return RoleSkillTemplate{}, false
	}
	files, ok := collectMountedTemplateFiles(root, name)
	if !ok {
		return RoleSkillTemplate{}, false
	}
	return RoleSkillTemplate{
		Name:        name, // directory name is authoritative, matching plugin skills
		Version:     mountedSkillTemplateVersion,
		Description: description,
		Content:     string(content),
		Files:       files,
	}, true
}

// collectMountedTemplateFiles gathers every supporting file under root (all but
// SKILL.md), keeping each file's path relative to root so nested references
// survive. It enforces the per-file, total, and file-count caps and refuses
// symlinks, which are the one traversal vector filepath.Rel cannot rule out.
func collectMountedTemplateFiles(root, name string) ([]AgentSkillFileData, bool) {
	var files []AgentSkillFileData
	var bundleSize int64
	walkErr := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		// A symlink surfaces here as a non-directory entry; WalkDir does not
		// descend it. Refuse both symlinked files and symlinked directories so a
		// link cannot pull content in from outside the template directory.
		if d.Type()&fs.ModeSymlink != 0 {
			slog.Warn("skill templates: skipping symlinked path", "name", name, "path", p)
			return nil
		}
		if d.IsDir() {
			return nil
		}
		rel, relErr := filepath.Rel(root, p)
		if relErr != nil {
			return relErr
		}
		rel = filepath.ToSlash(rel)
		if skill.IsReservedContentPath(rel) {
			// SKILL.md is the primary content, carried on Content, not a file.
			return nil
		}
		info, infoErr := d.Info()
		if infoErr != nil {
			return infoErr
		}
		if info.Size() > maxSkillTemplateFileSize {
			return fmt.Errorf("supporting file %q is %d bytes, over the %d byte per-file limit", rel, info.Size(), maxSkillTemplateFileSize)
		}
		if len(files) >= maxSkillTemplateFileCount {
			return fmt.Errorf("template has more than %d supporting files", maxSkillTemplateFileCount)
		}
		bundleSize += info.Size()
		if bundleSize > maxSkillTemplateTotalSize {
			return fmt.Errorf("supporting files exceed the %d byte bundle limit", maxSkillTemplateTotalSize)
		}
		data, readErr := os.ReadFile(p)
		if readErr != nil {
			return readErr
		}
		files = append(files, AgentSkillFileData{Path: rel, Content: string(data)})
		return nil
	})
	if walkErr != nil {
		slog.Warn("skill templates: skipping entry with unreadable or oversized supporting files", "name", name, "error", walkErr)
		return nil, false
	}
	// Stable order so the listing and its tests do not depend on directory walk
	// order across platforms.
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return files, true
}
