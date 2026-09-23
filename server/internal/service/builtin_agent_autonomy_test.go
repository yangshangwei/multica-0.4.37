package service

import (
	"os"
	"strings"
	"testing"
)

// TestAutonomyAtLeast_UndeclaredPasses is the compatibility rule, and the most
// important test in this file. Every agent that existed before role templates
// carries an empty level; if that ever started failing a policy check, this feature
// would break working workspaces on upgrade.
func TestAutonomyAtLeast_UndeclaredPasses(t *testing.T) {
	for _, want := range []AutonomyLevel{AutonomyObserver, AutonomyContributor, AutonomyCoordinator, AutonomyOperator} {
		if !AutonomyAtLeast("", want) {
			t.Errorf("AutonomyAtLeast(\"\", %q) = false, want true: an agent with no declared policy must keep behaving as it did", want)
		}
		// A level this binary does not recognise is the same case: a downgrade to
		// "deny" on an unknown string would turn a rollback into an outage.
		if !AutonomyAtLeast("supervisor", want) {
			t.Errorf("AutonomyAtLeast(\"supervisor\", %q) = false, want true", want)
		}
	}
}

func TestAutonomyAtLeast_Ordering(t *testing.T) {
	cases := []struct {
		have string
		want AutonomyLevel
		ok   bool
	}{
		{"observer", AutonomyObserver, true},
		{"observer", AutonomyContributor, false},
		{"observer", AutonomyOperator, false},
		{"contributor", AutonomyObserver, true},
		{"contributor", AutonomyContributor, true},
		{"contributor", AutonomyCoordinator, false},
		{"contributor", AutonomyOperator, false},
		{"coordinator", AutonomyContributor, true},
		{"coordinator", AutonomyCoordinator, true},
		{"coordinator", AutonomyOperator, false},
		{"operator", AutonomyOperator, true},
		{"operator", AutonomyObserver, true},
	}
	for _, tc := range cases {
		if got := AutonomyAtLeast(tc.have, tc.want); got != tc.ok {
			t.Errorf("AutonomyAtLeast(%q, %q) = %v, want %v", tc.have, tc.want, got, tc.ok)
		}
	}
}

func TestIsKnownAutonomyLevel(t *testing.T) {
	for _, level := range []string{"observer", "contributor", "coordinator", "operator"} {
		if !IsKnownAutonomyLevel(level) {
			t.Errorf("IsKnownAutonomyLevel(%q) = false", level)
		}
	}
	// Empty is deliberately NOT a known level: it means "no policy declared", and the
	// update endpoint has to be able to tell that apart from a typo.
	for _, level := range []string{"", "Observer", "admin", "operator "} {
		if IsKnownAutonomyLevel(level) {
			t.Errorf("IsKnownAutonomyLevel(%q) = true, want false", level)
		}
	}
}

// TestAutonomyBriefing_EmptyForUndeclared pins that the claim path adds nothing to
// an agent with no policy. This is what keeps existing prompts byte-identical.
func TestAutonomyBriefing_EmptyForUndeclared(t *testing.T) {
	if got := AutonomyBriefing(""); got != "" {
		t.Errorf("AutonomyBriefing(\"\") = %q, want empty", got)
	}
	if got := AutonomyBriefing("supervisor"); got != "" {
		t.Errorf("AutonomyBriefing(\"supervisor\") = %q, want empty", got)
	}
}

// TestAutonomyBriefing_StatesTheLevelAndTheAlternative checks that each level's text
// tells the agent both what it cannot do and what to do instead. A prohibition with
// no sanctioned alternative is what makes a model invent one.
func TestAutonomyBriefing_StatesTheLevelAndTheAlternative(t *testing.T) {
	cases := map[string]string{
		"observer":    "Your level: Observer",
		"contributor": "Your level: Contributor",
		"coordinator": "Your level: Coordinator",
		"operator":    "Your level: Operator",
	}
	for level, marker := range cases {
		briefing := AutonomyBriefing(level)
		if !strings.Contains(briefing, marker) {
			t.Errorf("%s briefing does not announce the level (%q)", level, marker)
		}
		if !strings.Contains(briefing, "## Autonomy policy (system)") {
			t.Errorf("%s briefing is missing the system header that marks it non-editable", level)
		}
		if level == "operator" {
			if !strings.Contains(briefing, "### The approval protocol") {
				t.Errorf("operator briefing must carry the approval protocol")
			}
			if !strings.Contains(briefing, "never approve your own request") {
				t.Errorf("operator briefing must forbid self-approval")
			}
			continue
		}
		if !strings.Contains(briefing, "### When the work needs a high-risk action") {
			t.Errorf("%s briefing must tell the agent what to do when it hits an Operator action", level)
		}
		if strings.Contains(briefing, "### The approval protocol") {
			t.Errorf("%s briefing must not describe executing an approved action; it cannot execute one", level)
		}
	}
}

// TestAutonomyBriefing_NamesTheCommands keeps the protocol runnable. The briefing
// tells an Operator to file a request and to record execution; if the command
// names drift from cmd_approval.go, the agent is told to run something that does
// not exist and will improvise instead.
func TestAutonomyBriefing_NamesTheCommands(t *testing.T) {
	briefing := AutonomyBriefing("operator")
	for _, command := range []string{
		"multica approval request",
		"multica approval executed",
		"--risk-class",
		"--plan-file",
	} {
		if !strings.Contains(briefing, command) {
			t.Errorf("operator briefing does not mention %q", command)
		}
	}
}

// TestAutonomyBriefing_NamesEveryRiskClass keeps the prose the Operator reads in
// step with the classes the API accepts. A class described but rejected — or
// accepted but never described — is how an agent ends up filing a request nobody
// looks at.
func TestAutonomyBriefing_NamesEveryRiskClass(t *testing.T) {
	briefing := AutonomyBriefing("operator")
	for _, class := range ApprovalRiskClasses {
		if !strings.Contains(briefing, class) {
			t.Errorf("operator briefing does not mention risk class %q", class)
		}
	}
}

// TestApprovalRiskClasses_MatchMigrationCheck reads the migration that constrains
// the column. The Go list and the CHECK have to agree: a class Go accepts and
// Postgres rejects is a 500 at the moment an agent asks for permission.
func TestApprovalRiskClasses_MatchMigrationCheck(t *testing.T) {
	body, err := os.ReadFile("../../migrations/452_agent_approval_request.up.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	migration := string(body)
	for _, class := range ApprovalRiskClasses {
		if !strings.Contains(migration, "'"+class+"'") {
			t.Errorf("risk class %q is accepted by the API but absent from the migration's CHECK constraint", class)
		}
	}
	if !IsKnownApprovalRiskClass("production_release") {
		t.Error("IsKnownApprovalRiskClass rejected a class it must accept")
	}
	for _, invalid := range []string{"", "production", "PRODUCTION_RELEASE"} {
		if IsKnownApprovalRiskClass(invalid) {
			t.Errorf("IsKnownApprovalRiskClass(%q) = true, want false", invalid)
		}
	}
}

// TestSquadTemplates_RosterIsCoherent pins the built-in squads and the invariant
// that makes them provisionable: every seat names a template that exists, the leader
// is one of the unlisted coordinators, and the routing policy is present.
//
// The order is product-controlled and load-bearing: the picker renders the registry
// as given, and the two squads whose roster reaches `operator` (release, incident)
// are deliberately last so permission rises monotonically down the list.
func TestSquadTemplates_RosterIsCoherent(t *testing.T) {
	templates := SquadTemplates()
	wantKeys := []string{
		"feature-delivery", "bug-fix",
		"review-gate", "discovery", "docs", "maintenance",
		"release", "incident",
	}
	if len(templates) != len(wantKeys) {
		t.Fatalf("squad roster = %d templates, want %d (%s)", len(templates), len(wantKeys), strings.Join(wantKeys, ", "))
	}
	for i, key := range wantKeys {
		if templates[i].Key != key {
			t.Errorf("squad[%d].Key = %q, want %q", i, templates[i].Key, key)
		}
	}
	for _, template := range templates {
		leader, ok := AgentRoleTemplateByKey(template.LeaderTemplateKey)
		if !ok {
			t.Errorf("%s: leader template %q does not exist", template.Key, template.LeaderTemplateKey)
			continue
		}
		if leader.Listed {
			t.Errorf("%s: leader %q is listed in the agent picker; leaders are provisioned by the squad flow", template.Key, leader.Key)
		}
		if len(template.Members) == 0 {
			t.Errorf("%s has no members; a leader with an empty roster cannot route", template.Key)
		}
		for _, slot := range template.Members {
			role, ok := AgentRoleTemplateByKey(slot.TemplateKey)
			if !ok {
				t.Errorf("%s: member seat names unknown template %q", template.Key, slot.TemplateKey)
				continue
			}
			if !role.Listed {
				t.Errorf("%s: member seat %q is an unlisted leader template", template.Key, slot.TemplateKey)
			}
			for _, language := range TemplateLanguages {
				if strings.TrimSpace(slot.Roles[language]) == "" {
					t.Errorf("%s: seat %q has no %s role note", template.Key, slot.TemplateKey, language)
				}
			}
		}
		instructions := template.Instructions()
		if strings.TrimSpace(instructions) == "" {
			t.Errorf("%s: routing policy is empty", template.Key)
		}
		// The status rule is the one squads get wrong: dispatching is not delivery,
		// and `done` is a human's call.
		if !strings.Contains(instructions, "## 父任务状态") {
			t.Errorf("%s: routing policy does not state the parent-issue status rule", template.Key)
		}
		if !strings.Contains(instructions, "绝不将父任务标记为 `done`") {
			t.Errorf("%s: routing policy must state that the leader does not mark work done", template.Key)
		}
	}
}

func TestSquadTemplates_DiagnosticianPlacementMatchesFailureWorkflows(t *testing.T) {
	want := map[string][]string{
		"bug-fix":     {"原因未知", "诊断工程师", "实现工程师"},
		"maintenance": {"原因未知", "诊断工程师", "不稳定测试"},
		"incident":    {"独立后续任务", "诊断工程师", "不等待"},
	}

	for _, squad := range SquadTemplates() {
		diagnosticians := 0
		for _, member := range squad.Members {
			if member.TemplateKey == "diagnostician" {
				diagnosticians++
			}
		}

		required, shouldSeat := want[squad.Key]
		if shouldSeat && diagnosticians != 1 {
			t.Errorf("%s seats %d diagnosticians, want exactly 1", squad.Key, diagnosticians)
		}
		if !shouldSeat && diagnosticians != 0 {
			t.Errorf("%s seats %d diagnosticians, want none: RCA is not part of this squad's default workflow", squad.Key, diagnosticians)
		}
		for _, contract := range required {
			if !strings.Contains(squad.Instructions(), contract) {
				t.Errorf("%s routing policy does not explain diagnostician boundary %q", squad.Key, contract)
			}
		}
	}
}

func TestSquadTemplates_LifecycleEvidenceRouting(t *testing.T) {
	want := map[string][]string{
		"review-gate": {"agent-evaluator"},
		"release":     {"reliability-engineer", "agent-evaluator"},
		"incident":    {"reliability-engineer"},
	}
	for _, squad := range SquadTemplates() {
		seen := map[string]bool{}
		for _, member := range squad.Members {
			seen[member.TemplateKey] = true
		}
		for _, role := range want[squad.Key] {
			if !seen[role] {
				t.Errorf("%s does not route lifecycle evidence to %s", squad.Key, role)
			}
		}
	}
	release, _ := SquadTemplateByKey("release")
	if !strings.Contains(release.Instructions(), "观察窗口") || !strings.Contains(release.Instructions(), "同一份产物") {
		t.Error("release routing policy must require a same-artifact observation window")
	}
	incident, _ := SquadTemplateByKey("incident")
	if !strings.Contains(incident.Instructions(), "复盘") || !strings.Contains(incident.Instructions(), "预防") {
		t.Error("incident routing policy must route recovery evidence into learning and prevention")
	}
}

func TestSquadLeads_ExplainDiagnosticianHandoff(t *testing.T) {
	want := map[string][]string{
		"bug-fix-lead":     {"诊断工程师", "原因未知"},
		"maintenance-lead": {"诊断工程师", "不稳定测试"},
		"incident-lead":    {"诊断工程师", "独立后续任务", "不等待"},
	}

	for key, contracts := range want {
		lead, ok := AgentRoleTemplateByKey(key)
		if !ok {
			t.Fatalf("leader template %q is missing", key)
		}
		instructions := lead.Instructions()
		for _, contract := range contracts {
			if !strings.Contains(instructions, contract) {
				t.Errorf("%s instructions do not explain diagnostician handoff %q", key, contract)
			}
		}
	}
}

// TestSquadTemplates_StaffOnlyListedWorkingRoles guards the decision the role
// roster rests on: a new squad is a new ROUTING POLICY over listed roles, never an
// excuse to mint a one-off role. A new unlisted working role would arrive here first
// because a squad seat is the only place it becomes reachable without editing the
// picker's own test.
func TestSquadTemplates_StaffOnlyListedWorkingRoles(t *testing.T) {
	listed := map[string]bool{}
	for _, template := range AgentRoleTemplates() {
		listed[template.Key] = true
	}
	for _, squad := range SquadTemplates() {
		for _, slot := range squad.Members {
			if !listed[slot.TemplateKey] {
				t.Errorf("%s: seat %q is not one of the listed working roles", squad.Key, slot.TemplateKey)
			}
		}
	}
}

// TestSquadTemplates_LeadersAreOneToOne pins that each squad has its own leader
// definition. Sharing one lead between two squads would make the routing policy the
// only difference between them, and the policy lives on the squad row where a
// workspace may edit it — so a shared lead's judgement would drift out of step with
// whichever squad edited last.
func TestSquadTemplates_LeadersAreOneToOne(t *testing.T) {
	seen := map[string]string{}
	for _, squad := range SquadTemplates() {
		if other, ok := seen[squad.LeaderTemplateKey]; ok {
			t.Errorf("squads %q and %q share leader %q; each squad needs its own", other, squad.Key, squad.LeaderTemplateKey)
			continue
		}
		seen[squad.LeaderTemplateKey] = squad.Key
	}
	for _, template := range AllAgentRoleTemplates() {
		if template.Listed {
			continue
		}
		if _, ok := seen[template.Key]; !ok {
			t.Errorf("lead template %q leads no squad, so nothing can provision it", template.Key)
		}
	}
}

// TestSquadTemplates_MaxAutonomyMatchesRoster pins each squad's authorization
// ceiling as a contract rather than a coincidence of its roster. MaxAutonomy is what
// CreateSquadFromTemplate checks the caller against, so a seat added to release or
// incident that quietly raised another squad to `operator` would change who may
// staff it — a permission change that must be a deliberate edit here.
func TestSquadTemplates_MaxAutonomyMatchesRoster(t *testing.T) {
	want := map[string]AutonomyLevel{
		"feature-delivery": AutonomyCoordinator,
		"bug-fix":          AutonomyCoordinator,
		"review-gate":      AutonomyCoordinator,
		"discovery":        AutonomyCoordinator,
		"docs":             AutonomyCoordinator,
		"maintenance":      AutonomyCoordinator,
		// The two rosters that seat the Release Engineer. Staffing either one mints an
		// Operator, so only a caller allowed to grant Operator may do it.
		"release":  AutonomyOperator,
		"incident": AutonomyOperator,
	}
	for _, squad := range SquadTemplates() {
		expected, ok := want[squad.Key]
		if !ok {
			t.Errorf("squad %q has no declared autonomy ceiling in this test; add it deliberately", squad.Key)
			continue
		}
		if got := squad.MaxAutonomy(); got != expected {
			t.Errorf("%s MaxAutonomy() = %q, want %q", squad.Key, got, expected)
		}
	}
}

// TestSquadTemplate_TemplateKeysStartWithLeader pins the provisioning order the
// transaction depends on: the squad row cannot be written before its leader exists.
func TestSquadTemplate_TemplateKeysStartWithLeader(t *testing.T) {
	for _, template := range SquadTemplates() {
		keys := template.TemplateKeys()
		if len(keys) != len(template.Members)+1 {
			t.Errorf("%s: TemplateKeys() = %d entries, want leader + %d members", template.Key, len(keys), len(template.Members))
		}
		if keys[0] != template.LeaderTemplateKey {
			t.Errorf("%s: TemplateKeys()[0] = %q, want the leader %q", template.Key, keys[0], template.LeaderTemplateKey)
		}
	}
}
