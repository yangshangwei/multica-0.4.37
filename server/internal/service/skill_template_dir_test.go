package service

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeTemplateFile writes a file under dir, creating parent directories.
func writeTemplateFile(t *testing.T, dir, rel, content string) {
	t.Helper()
	full := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(full), err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", full, err)
	}
}

func skillMD(name, description, body string) string {
	return "---\nname: " + name + "\ndescription: " + description + "\n---\n" + body
}

func templateByName(templates []RoleSkillTemplate, name string) (RoleSkillTemplate, bool) {
	for _, tpl := range templates {
		if tpl.Name == name {
			return tpl, true
		}
	}
	return RoleSkillTemplate{}, false
}

// TestSkillTemplates_EmbedOnlyWhenDirUnset pins AC3: an unconfigured deployment
// returns exactly the embedded catalog, in the same order.
func TestSkillTemplates_EmbedOnlyWhenDirUnset(t *testing.T) {
	embed := RoleSkillTemplates()

	for _, svc := range []*TaskService{nil, {}, {SkillTemplateDir: ""}} {
		got := svc.SkillTemplates()
		if len(got) != len(embed) {
			t.Fatalf("SkillTemplates() = %d entries, want %d embedded", len(got), len(embed))
		}
		for i := range embed {
			if got[i].Name != embed[i].Name || got[i].Version != embed[i].Version || got[i].Content != embed[i].Content {
				t.Fatalf("entry %d = %q, want embedded %q verbatim", i, got[i].Name, embed[i].Name)
			}
		}
	}
}

// TestSkillTemplates_MissingDirIsEmbedOnly pins AC3 for a configured-but-absent
// directory: it must not error, just fall back to the embedded catalog.
func TestSkillTemplates_MissingDirIsEmbedOnly(t *testing.T) {
	svc := &TaskService{SkillTemplateDir: filepath.Join(t.TempDir(), "does-not-exist")}
	if len(svc.SkillTemplates()) != len(RoleSkillTemplates()) {
		t.Fatal("a missing mounted directory must yield the embedded catalog only")
	}
}

// TestSkillTemplates_MountedEntryWithFiles pins AC1 and AC5: a well-formed
// mounted template appears in the catalog with its frontmatter description,
// version 0, and its supporting files.
func TestSkillTemplates_MountedEntryWithFiles(t *testing.T) {
	dir := t.TempDir()
	writeTemplateFile(t, dir, "my-debug-helper/SKILL.md", skillMD("my-debug-helper", "Help debug tricky failures", "# Debug\nSteps here."))
	writeTemplateFile(t, dir, "my-debug-helper/references/foo.md", "reference body")
	writeTemplateFile(t, dir, "my-debug-helper/references/nested/bar.txt", "nested body")

	svc := &TaskService{SkillTemplateDir: dir}
	got := svc.SkillTemplates()

	if len(got) != len(RoleSkillTemplates())+1 {
		t.Fatalf("expected embed catalog + 1 mounted entry, got %d", len(got))
	}
	tpl, ok := templateByName(got, "my-debug-helper")
	if !ok {
		t.Fatal("mounted template my-debug-helper is missing from the catalog")
	}
	if tpl.Version != 0 {
		t.Errorf("mounted template version = %d, want 0 (non-release content)", tpl.Version)
	}
	if tpl.Description != "Help debug tricky failures" {
		t.Errorf("description = %q, want the frontmatter value", tpl.Description)
	}
	if !strings.Contains(tpl.Content, "# Debug") {
		t.Errorf("content must be the raw SKILL.md, got %q", tpl.Content)
	}
	if len(tpl.Files) != 2 {
		t.Fatalf("files = %d, want 2 supporting files", len(tpl.Files))
	}
	// Files are sorted by relative path, forward-slash normalized.
	if tpl.Files[0].Path != "references/foo.md" || tpl.Files[1].Path != "references/nested/bar.txt" {
		t.Errorf("supporting file paths = %q, %q; want normalized relative paths", tpl.Files[0].Path, tpl.Files[1].Path)
	}
}

// TestSkillTemplates_MalformedEntriesSkipped pins AC4: each class of malformed
// entry is skipped while the well-formed sibling still lists.
func TestSkillTemplates_MalformedEntriesSkipped(t *testing.T) {
	dir := t.TempDir()

	// Well-formed entry that must survive alongside the malformed ones.
	writeTemplateFile(t, dir, "good-one/SKILL.md", skillMD("good-one", "A fine template", "body"))

	// Missing SKILL.md: a directory with only a supporting file.
	writeTemplateFile(t, dir, "no-skill-md/references/only.md", "orphan")

	// Frontmatter with no name.
	writeTemplateFile(t, dir, "no-name/SKILL.md", "---\ndescription: nameless\n---\nbody")

	// Illegal directory name (contains a dot, rejected by the grammar).
	writeTemplateFile(t, dir, "bad.name/SKILL.md", skillMD("bad-name", "illegal dir", "body"))

	// A plain file at the top level, not a directory.
	writeTemplateFile(t, dir, "loose-file.md", "not a template")

	svc := &TaskService{SkillTemplateDir: dir}
	got := svc.SkillTemplates()

	if _, ok := templateByName(got, "good-one"); !ok {
		t.Fatal("the well-formed entry must still be listed")
	}
	for _, bad := range []string{"no-skill-md", "no-name", "bad.name", "bad-name", "loose-file", "loose-file.md"} {
		if _, ok := templateByName(got, bad); ok {
			t.Errorf("malformed entry %q must be skipped", bad)
		}
	}
	if len(got) != len(RoleSkillTemplates())+1 {
		t.Fatalf("only the one well-formed entry should be added, got %d total", len(got))
	}
}

// TestSkillTemplates_PathEscapeSkipped pins AC4 for traversal: an entry whose
// only supporting file is a symlink escaping the directory is skipped, and its
// content never leaks in.
func TestSkillTemplates_PathEscapeSkipped(t *testing.T) {
	dir := t.TempDir()
	secretDir := t.TempDir()
	secret := filepath.Join(secretDir, "secret.txt")
	if err := os.WriteFile(secret, []byte("top secret"), 0o644); err != nil {
		t.Fatalf("write secret: %v", err)
	}

	writeTemplateFile(t, dir, "escaper/SKILL.md", skillMD("escaper", "tries to escape", "body"))
	link := filepath.Join(dir, "escaper", "references", "leak.txt")
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.Symlink(secret, link); err != nil {
		t.Skipf("symlinks unavailable on this platform: %v", err)
	}

	svc := &TaskService{SkillTemplateDir: dir}
	got := svc.SkillTemplates()

	tpl, ok := templateByName(got, "escaper")
	if !ok {
		// Refusing the symlink is fine; leaking its content is not. The template
		// may still list with no files, so both "skipped entirely" and "listed
		// without the symlinked file" are acceptable — only the leak is a bug.
		return
	}
	for _, f := range tpl.Files {
		if strings.Contains(f.Content, "top secret") {
			t.Fatalf("symlinked file %q leaked out-of-tree content", f.Path)
		}
	}
}

// TestSkillTemplates_TopLevelSymlinkSkipped pins R-1 for the top-level entry: a
// <name> that is itself a symlink to an external template directory is refused
// outright, so not even its SKILL.md body is read. Mirrors the "no symlinks"
// stance applied to supporting files.
func TestSkillTemplates_TopLevelSymlinkSkipped(t *testing.T) {
	dir := t.TempDir()
	external := t.TempDir()
	writeTemplateFile(t, external, "SKILL.md", skillMD("sneaky", "external template", "leaked body"))

	link := filepath.Join(dir, "sneaky")
	if err := os.Symlink(external, link); err != nil {
		t.Skipf("symlinks unavailable on this platform: %v", err)
	}

	svc := &TaskService{SkillTemplateDir: dir}
	got := svc.SkillTemplates()

	if tpl, ok := templateByName(got, "sneaky"); ok {
		t.Fatalf("symlinked top-level entry was listed and leaked content: %q", tpl.Content)
	}
}

// TestSkillTemplates_EmbedWinsOnNameClash pins AC6/D4: a mounted entry named
// like an embedded role skill is skipped, and the embedded content is returned.
func TestSkillTemplates_EmbedWinsOnNameClash(t *testing.T) {
	const clash = "multica-code-review"
	embedTpl, ok := RoleSkillTemplateByName(clash)
	if !ok {
		t.Fatalf("%s must exist in the embedded registry for this test", clash)
	}

	dir := t.TempDir()
	writeTemplateFile(t, dir, clash+"/SKILL.md", skillMD(clash, "IMPOSTOR description", "impostor body"))

	svc := &TaskService{SkillTemplateDir: dir}
	got := svc.SkillTemplates()

	tpl, found := templateByName(got, clash)
	if !found {
		t.Fatalf("%s must still be present from the embedded catalog", clash)
	}
	if tpl.Content != embedTpl.Content || tpl.Description != embedTpl.Description {
		t.Error("embedded content must win over a same-named mounted entry")
	}
	// Exactly one entry with the clashing name — the mounted impostor is gone.
	count := 0
	for _, e := range got {
		if e.Name == clash {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected a single %s entry, got %d", clash, count)
	}
	if len(got) != len(RoleSkillTemplates()) {
		t.Fatalf("the mounted impostor must not add an entry, total = %d", len(got))
	}
}

// TestSkillTemplates_OversizedSkillMdSkipped pins the per-file cap: a SKILL.md
// over the limit is skipped rather than served.
func TestSkillTemplates_OversizedSkillMdSkipped(t *testing.T) {
	dir := t.TempDir()
	huge := skillMD("too-big", "oversized", strings.Repeat("a", maxSkillTemplateFileSize+1))
	writeTemplateFile(t, dir, "too-big/SKILL.md", huge)

	svc := &TaskService{SkillTemplateDir: dir}
	if _, ok := templateByName(svc.SkillTemplates(), "too-big"); ok {
		t.Fatal("an oversized SKILL.md must be skipped")
	}
}
