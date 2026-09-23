package service

import (
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/internal/skill"
)

// The registry is content, and content is what this feature is. These tests pin the
// invariants a release must not break: the roster's size and shape, the sections
// every role prompt has to carry, and the fact that every skill and leader a
// template names actually exists in this binary.

// TestAgentRoleTemplates_ListedRosterIsTen pins the product decision. New listed
// roles must address distinct work rather than duplicate a role for each stack.
func TestAgentRoleTemplates_ListedRosterIsTen(t *testing.T) {
	listed := AgentRoleTemplates()
	if len(listed) != 10 {
		names := make([]string, 0, len(listed))
		for _, template := range listed {
			names = append(names, template.Key)
		}
		t.Fatalf("listed roster = %d templates (%s), want 10", len(listed), strings.Join(names, ", "))
	}
	want := []string{
		"product-analyst", "architect", "implementer", "qa-engineer",
		"diagnostician", "code-reviewer", "security-reviewer", "release-engineer",
		"technical-writer", "progress-reporter",
	}
	for i, key := range want {
		if listed[i].Key != key {
			t.Errorf("listed[%d].Key = %q, want %q (order is product-controlled)", i, listed[i].Key, key)
		}
	}
}

// TestAgentRoleTemplates_UnlistedAreSquadLeaders keeps the picker and the squad
// flow from drifting: an unlisted template exists only because a squad template
// names it as a leader, and leading requires Coordinator.
func TestAgentRoleTemplates_UnlistedAreSquadLeaders(t *testing.T) {
	leaderKeys := map[string]bool{}
	for _, squad := range SquadTemplates() {
		leaderKeys[squad.LeaderTemplateKey] = true
	}
	for _, template := range AllAgentRoleTemplates() {
		if template.Listed {
			continue
		}
		if !leaderKeys[template.Key] {
			t.Errorf("template %q is unlisted but no squad template leads with it — it would be unreachable", template.Key)
		}
		if template.Autonomy != AutonomyCoordinator {
			t.Errorf("squad leader %q autonomy = %q, want coordinator: routing means creating sub-issues and mentioning members", template.Key, template.Autonomy)
		}
	}
}

// TestAgentRoleTemplates_AutonomyDefaults pins the roster's policy mapping. These
// defaults are the product's claim about which roles can act unsupervised, so a
// change here should be a deliberate edit rather than a side effect.
func TestAgentRoleTemplates_AutonomyDefaults(t *testing.T) {
	want := map[string]AutonomyLevel{
		"product-analyst":   AutonomyObserver,
		"architect":         AutonomyObserver,
		"implementer":       AutonomyContributor,
		"qa-engineer":       AutonomyContributor,
		"diagnostician":     AutonomyContributor,
		"code-reviewer":     AutonomyObserver,
		"security-reviewer": AutonomyObserver,
		"release-engineer":  AutonomyOperator,
		"technical-writer":  AutonomyContributor,
		"progress-reporter": AutonomyContributor,
	}
	for key, expected := range want {
		template, ok := AgentRoleTemplateByKey(key)
		if !ok {
			t.Fatalf("template %q is missing from the roster", key)
		}
		if template.Autonomy != expected {
			t.Errorf("%s autonomy = %q, want %q", key, template.Autonomy, expected)
		}
	}
	// Exactly one Operator in the first roster: production access is the privilege
	// this feature is most likely to over-grant by accident.
	operators := 0
	for _, template := range AllAgentRoleTemplates() {
		if template.Autonomy == AutonomyOperator {
			operators++
		}
	}
	if operators != 1 {
		t.Errorf("%d templates default to operator, want exactly 1 (release-engineer)", operators)
	}
}

// TestAgentRoleTemplates_InstructionsCarryTheContract is the content check. Each
// role prompt has to answer the six questions the plan requires of a template —
// what it does, what it must not do, what it reads, what it emits, what "done"
// means, and when to escalate. A prompt missing one of those is why teams end up
// rewriting an agent from scratch.
func TestAgentRoleTemplates_InstructionsCarryTheContract(t *testing.T) {
	required := []string{
		"## 不负责的事项",
		"## 完成标准",
		"## 需要人工介入的情况",
	}
	// A working role lists responsibilities; a squad leader's equivalent is its
	// routing table. Either satisfies "this prompt says what the role does" — an
	// exact heading match here would only push the leaders into a section name that
	// reads wrong for them.
	statesWhatItDoes := []string{"## 职责", "## 如何分配工作"}
	for _, template := range AllAgentRoleTemplates() {
		instructions := template.Instructions()
		if strings.TrimSpace(instructions) == "" {
			t.Errorf("%s: INSTRUCTIONS.md is empty or unreadable", template.Key)
			continue
		}
		for _, heading := range required {
			if !strings.Contains(instructions, heading) {
				t.Errorf("%s: instructions are missing %q", template.Key, heading)
			}
		}
		if !containsAny(instructions, statesWhatItDoes) {
			t.Errorf("%s: instructions have neither %q nor %q", template.Key, statesWhatItDoes[0], statesWhatItDoes[1])
		}
		// Squad leaders describe inputs and output through the routing table and the
		// squad's own policy, so only the working roles carry these two.
		if template.Listed {
			for _, heading := range []string{"## 输入", "## 交付格式"} {
				if !strings.Contains(instructions, heading) {
					t.Errorf("%s: instructions are missing %q", template.Key, heading)
				}
			}
		}
		if strings.Contains(instructions, "{{") {
			t.Errorf("%s: instructions contain an unsubstituted placeholder; role prompts are copied verbatim", template.Key)
		}
	}
}

// TestAgentRoleTemplates_RoleSkillsExist catches the failure that would otherwise
// only surface as a 500 during agent creation: a template naming a skill this
// binary does not embed.
func TestAgentRoleTemplates_RoleSkillsExist(t *testing.T) {
	for _, template := range AllAgentRoleTemplates() {
		if len(template.RoleSkills) > 1 {
			// The stated mitigation for prompt bloat. Two default skills per role is a
			// product decision, not an oversight to discover in production.
			t.Errorf("%s attaches %d default skills; the roster caps this at 1", template.Key, len(template.RoleSkills))
		}
		for _, name := range template.RoleSkills {
			skill, ok := RoleSkillTemplateByName(name)
			if !ok {
				t.Errorf("%s names role skill %q, which is not embedded", template.Key, name)
				continue
			}
			if !strings.HasPrefix(skill.Name, "multica-") {
				t.Errorf("role skill %q must keep the multica- prefix so it cannot collide with a user's own skill", skill.Name)
			}
			if strings.TrimSpace(skill.Description) == "" {
				t.Errorf("role skill %q has no frontmatter description; the workspace row would show an empty summary", skill.Name)
			}
			if skill.Version < 1 {
				t.Errorf("role skill %q version = %d, want >= 1", skill.Name, skill.Version)
			}
		}
	}
}

func TestAgentRoleTemplates_DefaultRoleSkills(t *testing.T) {
	want := map[string][]string{
		"product-analyst":       {"multica-requirement-clarification"},
		"architect":             {"multica-architecture-decision-record"},
		"implementer":           {"multica-test-report"},
		"qa-engineer":           {"multica-test-report"},
		"diagnostician":         {"multica-debugging"},
		"code-reviewer":         {"multica-code-review"},
		"security-reviewer":     {"multica-security-review"},
		"release-engineer":      {"multica-release-check"},
		"technical-writer":      {"multica-documentation-change"},
		"progress-reporter":     {"multica-progress-report"},
		"feature-delivery-lead": {"multica-requirement-clarification"},
		"discovery-lead":        {"multica-requirement-clarification"},
		"bug-fix-lead":          nil,
		"review-gate-lead":      nil,
		"docs-lead":             nil,
		"maintenance-lead":      nil,
		"release-lead":          nil,
		"incident-lead":         nil,
	}
	templates := AllAgentRoleTemplates()
	if len(templates) != len(want) {
		t.Fatalf("roster has %d templates, skill mapping covers %d", len(templates), len(want))
	}
	for _, template := range templates {
		expected, ok := want[template.Key]
		if !ok {
			t.Errorf("%s has no expected default skill mapping", template.Key)
			continue
		}
		if !slices.Equal(template.RoleSkills, expected) {
			t.Errorf("%s role skills = %v, want %v", template.Key, template.RoleSkills, expected)
		}
	}
}

func TestAgentRoleTemplate_ProgressReporterDefaults(t *testing.T) {
	template, ok := AgentRoleTemplateByKey("progress-reporter")
	if !ok {
		t.Fatal("progress-reporter template missing from the roster")
	}
	if !template.Listed || template.Version != 1 || template.MaxConcurrentTasks != 1 {
		t.Errorf("reporter defaults = listed:%t version:%d concurrency:%d, want true, 1, 1", template.Listed, template.Version, template.MaxConcurrentTasks)
	}
	if template.DefaultName != "Progress Reporter" || template.Title("zh") != "进展报告员" {
		t.Errorf("reporter names = %q / %q, want Progress Reporter / 进展报告员", template.DefaultName, template.Title("zh"))
	}
}

func TestRoleSkillTemplates_RosterIsNine(t *testing.T) {
	want := []string{
		"multica-architecture-decision-record", "multica-code-review",
		"multica-debugging", "multica-documentation-change", "multica-progress-report",
		"multica-release-check", "multica-requirement-clarification",
		"multica-security-review", "multica-test-report",
	}
	templates := RoleSkillTemplates()
	if len(templates) != len(want) {
		t.Fatalf("role skill roster = %d, want %d", len(templates), len(want))
	}
	for i, name := range want {
		if templates[i].Name != name {
			t.Errorf("role skill[%d] = %q, want %q", i, templates[i].Name, name)
		}
	}
}

// Reporting content is the feature's executable policy. Keep the role and its
// reusable method aligned on evidence, scope, and permitted writes.
func TestProgressReporter_EvidenceAndCloseoutContract(t *testing.T) {
	role, ok := AgentRoleTemplateByKey("progress-reporter")
	if !ok {
		t.Fatal("progress-reporter template missing from the roster")
	}
	skill, ok := RoleSkillTemplateByName("multica-progress-report")
	if !ok {
		t.Fatal("multica-progress-report skill missing from the registry")
	}
	if skill.Version != 1 {
		t.Errorf("progress report skill version = %d, want 1", skill.Version)
	}
	for name, body := range map[string]string{"role": role.Instructions(), "skill": skill.Content} {
		t.Run(name, func(t *testing.T) {
			for _, contract := range []string{
				"日报", "周报", "时区", "截止时间", "分页", "状态变更历史",
				"updated_at", "done", "关闭", "报告任务", "只读", "证据不足",
				"缺失", "评论发表成功", "in_review", "不另建报告任务",
				"临时评论正文文件", "工作目录", "UTF-8", "--content-file",
				"不覆盖已有文件", "删除本次创建的临时文件",
			} {
				if !strings.Contains(body, contract) {
					t.Errorf("missing reporting contract %q", contract)
				}
			}
		})
	}
}

// Debugging content is the feature's executable policy. Keep the role and its
// reusable method aligned on evidence grading and the no-fix boundary, and keep
// the skill explicit about where it stops relative to its neighbours.
func TestDiagnostician_EvidenceContract(t *testing.T) {
	role, ok := AgentRoleTemplateByKey("diagnostician")
	if !ok {
		t.Fatal("diagnostician template missing from the roster")
	}
	if !role.Listed || role.Version != 1 || role.MaxConcurrentTasks != 1 {
		t.Errorf("diagnostician defaults = listed:%t version:%d concurrency:%d, want true, 1, 1", role.Listed, role.Version, role.MaxConcurrentTasks)
	}
	for _, other := range AgentRoleTemplates() {
		if other.Key != role.Key && other.AvatarEmoji == role.AvatarEmoji {
			t.Errorf("diagnostician avatar must differ from %s", other.Key)
		}
	}
	skill, ok := RoleSkillTemplateByName("multica-debugging")
	if !ok {
		t.Fatal("multica-debugging skill missing from the registry")
	}
	if skill.Version != 1 {
		t.Errorf("debugging skill version = %d, want 1", skill.Version)
	}
	// Method and evidence grading travel with both the role and the skill:
	// reproduce, minimize, hypothesize, verify, and grade every conclusion as
	// confirmed, suspected, or not found — while shipping no fix and touching
	// no production data.
	shared := []string{
		"复现", "最小用例", "假设", "插桩", "二分",
		"已确认根因", "疑似", "未查明", "证据",
		"实现工程师", "不负责提交修复", "contributor", "生产",
		"隔离分支", "无修复时失败", "退出码", "已有改动",
	}
	for name, body := range map[string]string{"role": role.Instructions(), "skill": skill.Content} {
		t.Run(name, func(t *testing.T) {
			for _, contract := range shared {
				if !strings.Contains(body, contract) {
					t.Errorf("missing debugging contract %q", contract)
				}
			}
		})
	}
	// R2 lives in the reusable method: the skill itself must draw the line
	// against the verification and review skills it is most easily confused with.
	for _, boundary := range []string{"multica-test-report", "multica-code-review"} {
		if !strings.Contains(skill.Content, boundary) {
			t.Errorf("debugging skill missing neighbour boundary %q", boundary)
		}
	}
}

// Compare complete bundles with their sources without requiring a particular
// role to carry domain-specific reference material.
func TestRoleSkillTemplates_FilesMatchSource(t *testing.T) {
	for _, skill := range RoleSkillTemplates() {
		t.Run(skill.Name, func(t *testing.T) {
			dir := filepath.Join("builtin_role_skills", skill.Name)
			want := make(map[string]string)
			err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				relative, err := filepath.Rel(dir, path)
				if err != nil {
					return err
				}
				content, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				want[filepath.ToSlash(relative)] = string(content)
				return nil
			})
			if err != nil {
				t.Fatalf("read skill source bundle: %v", err)
			}
			if skill.Content != want["SKILL.md"] {
				t.Error("template content differs from source SKILL.md")
			}
			delete(want, "SKILL.md")
			if len(skill.Files) != len(want) {
				t.Errorf("template has %d supporting files, source has %d", len(skill.Files), len(want))
			}
			for _, file := range skill.Files {
				content, ok := want[file.Path]
				if !ok {
					t.Errorf("unexpected or duplicate supporting file %q", file.Path)
					continue
				}
				if file.Content != content {
					t.Errorf("supporting file %q differs from source", file.Path)
				}
				delete(want, file.Path)
			}
			for path := range want {
				t.Errorf("template is missing supporting file %q", path)
			}
		})
	}
}

// TestRoleSkillTemplates_EveryEmbeddedSkillIsRegistered walks the embedded
// directory rather than the version map, so a skill added on disk without a version
// fails here instead of silently never materializing.
func TestRoleSkillTemplates_EveryEmbeddedSkillIsRegistered(t *testing.T) {
	entries, err := os.ReadDir(filepath.Join("builtin_role_skills"))
	if err != nil {
		t.Fatalf("read builtin_role_skills: %v", err)
	}
	onDisk := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		onDisk++
		if _, ok := RoleSkillTemplateByName(entry.Name()); !ok {
			t.Errorf("builtin_role_skills/%s is embedded but has no entry in builtinRoleSkillVersions, so nothing can attach it", entry.Name())
		}
	}
	if loaded := len(RoleSkillTemplates()); loaded != onDisk {
		t.Errorf("RoleSkillTemplates() returned %d skills, %d directories on disk", loaded, onDisk)
	}
}

// TestAgentRoleTemplates_LocalizedCopyIsComplete keeps the picker from showing an
// English label inside an otherwise translated screen.
func TestAgentRoleTemplates_LocalizedCopyIsComplete(t *testing.T) {
	for _, template := range AllAgentRoleTemplates() {
		for _, language := range TemplateLanguages {
			if strings.TrimSpace(template.Titles[language]) == "" {
				t.Errorf("%s has no %s title", template.Key, language)
			}
			if strings.TrimSpace(template.Descriptions[language]) == "" {
				t.Errorf("%s has no %s description", template.Key, language)
			}
		}
		if template.MaxConcurrentTasks < 1 {
			t.Errorf("%s max_concurrent_tasks = %d, want >= 1", template.Key, template.MaxConcurrentTasks)
		}
		if strings.TrimSpace(template.AvatarEmoji) == "" {
			t.Errorf("%s has no avatar emoji", template.Key)
		}
	}
}

// TestAgentRoleTemplate_FallsBackToEnglish pins the unsupported-locale behaviour the
// API relies on instead of returning a 400 for picker copy.
func TestAgentRoleTemplate_FallsBackToEnglish(t *testing.T) {
	template, ok := AgentRoleTemplateByKey("implementer")
	if !ok {
		t.Fatal("implementer template missing")
	}
	if got := template.Title("de"); got != template.Titles["en"] {
		t.Errorf("Title(\"de\") = %q, want the English title %q", got, template.Titles["en"])
	}
	if got := template.Title("zh"); got != template.Titles["zh"] {
		t.Errorf("Title(\"zh\") = %q, want %q", got, template.Titles["zh"])
	}
}

// TestAgentRoleTemplateByKey_RejectsUnknown covers the API's validation path.
func TestAgentRoleTemplateByKey_RejectsUnknown(t *testing.T) {
	if _, ok := AgentRoleTemplateByKey("frontend-engineer"); ok {
		t.Error("AgentRoleTemplateByKey accepted a key that is not in the roster")
	}
	if _, ok := AgentRoleTemplateByKey(""); ok {
		t.Error("AgentRoleTemplateByKey accepted an empty key")
	}
}

func containsAny(body string, candidates []string) bool {
	for _, candidate := range candidates {
		if strings.Contains(body, candidate) {
			return true
		}
	}
	return false
}

// TestRoleSkillTemplates_PresentationDefaults pins the category/icon each role
// skill declares in its frontmatter `metadata` block. These seed
// config.presentation on materialization, so a missing or misspelled value
// would silently file a role skill under "other" with the generic icon.
func TestRoleSkillTemplates_PresentationDefaults(t *testing.T) {
	want := map[string][2]string{
		"multica-requirement-clarification":    {"research", "message-circle-question"},
		"multica-architecture-decision-record": {"design", "landmark"},
		"multica-code-review":                  {"quality", "git-pull-request"},
		"multica-security-review":              {"quality", "shield-check"},
		"multica-test-report":                  {"quality", "flask-conical"},
		"multica-debugging":                    {"quality", "microscope"},
		"multica-release-check":                {"operations", "rocket"},
		"multica-documentation-change":         {"writing", "book-open"},
		"multica-progress-report":              {"writing", "chart-no-axes-column"},
	}
	templates := RoleSkillTemplates()
	if len(templates) != len(want) {
		t.Fatalf("got %d role skills, want %d", len(templates), len(want))
	}
	for _, tpl := range templates {
		exp, ok := want[tpl.Name]
		if !ok {
			t.Errorf("unexpected role skill %q", tpl.Name)
			continue
		}
		if tpl.Category != exp[0] || tpl.Icon != exp[1] {
			t.Errorf("%s: category/icon = %q/%q, want %q/%q", tpl.Name, tpl.Category, tpl.Icon, exp[0], exp[1])
		}
		if !skill.IsCategory(tpl.Category) || !skill.IsIconName(tpl.Icon) {
			t.Errorf("%s: category %q or icon %q is not on the whitelist", tpl.Name, tpl.Category, tpl.Icon)
		}
	}
}
