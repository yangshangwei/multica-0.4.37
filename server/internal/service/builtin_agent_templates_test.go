package service

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// The registry is content, and content is what this feature is. These tests pin the
// invariants a release must not break: the roster's size and shape, the sections
// every role prompt has to carry, and the fact that every skill and leader a
// template names actually exists in this binary.

// TestAgentRoleTemplates_ListedRosterIsEight pins the product decision. Eight was
// chosen over a per-stack explosion of near-identical prompts; a ninth listed role
// is a product change and should have to edit this number deliberately.
func TestAgentRoleTemplates_ListedRosterIsEight(t *testing.T) {
	listed := AgentRoleTemplates()
	if len(listed) != 8 {
		names := make([]string, 0, len(listed))
		for _, template := range listed {
			names = append(names, template.Key)
		}
		t.Fatalf("listed roster = %d templates (%s), want 8", len(listed), strings.Join(names, ", "))
	}
	want := []string{
		"product-analyst", "architect", "implementer", "qa-engineer",
		"code-reviewer", "security-reviewer", "release-engineer", "technical-writer",
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
		"code-reviewer":     AutonomyObserver,
		"security-reviewer": AutonomyObserver,
		"release-engineer":  AutonomyOperator,
		"technical-writer":  AutonomyContributor,
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
		"code-reviewer":         {"multica-code-review"},
		"security-reviewer":     {"multica-security-review"},
		"release-engineer":      {"multica-release-check"},
		"technical-writer":      {"multica-documentation-change"},
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

// A linked reference must travel with the template that agents receive, not
// merely exist beside SKILL.md in the source checkout.
func TestRequirementClarificationSkillIncludesCSVReference(t *testing.T) {
	const referencePath = "references/csv-export-safety.md"
	skill, ok := RoleSkillTemplateByName("multica-requirement-clarification")
	if !ok {
		t.Fatal("requirement clarification must be a registered role skill")
	}
	if !strings.Contains(skill.Content, "]("+referencePath+")") {
		t.Fatal("requirement clarification must link its CSV safety reference")
	}
	for _, file := range skill.Files {
		if file.Path != referencePath {
			continue
		}
		want, err := os.ReadFile(filepath.Join("builtin_role_skills", skill.Name, referencePath))
		if err != nil {
			t.Fatalf("read source reference: %v", err)
		}
		if strings.TrimSpace(file.Content) == "" || file.Content != string(want) {
			t.Fatal("template must deliver the complete CSV reference unchanged")
		}
		return
	}
	t.Fatal("template must include the linked CSV reference as a supporting file")
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
