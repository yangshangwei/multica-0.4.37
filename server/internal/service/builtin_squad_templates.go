package service

import (
	"embed"
	"path"
	"strings"
)

// Built-in squad templates: a leader, a roster, and the routing policy that makes
// the two mean something together.
//
// A squad template creates ordinary squad rows. It does not introduce a new
// execution model: the leader still receives the work and still dispatches by
// @mention, exactly as a hand-built squad does. What the template supplies is the
// part teams get wrong — which roles belong in the roster, which member handles
// what, and when the leader must stop and ask a human.
//
// The leader's own instructions come from an unlisted role template
// (feature-delivery-lead / bug-fix-lead). The squad's instructions, embedded here,
// are the leader-only briefing: this roster's routing table. Both are copied at
// creation and owned by the workspace afterwards.

//go:embed builtin_squad_templates
var builtinSquadTemplatesFS embed.FS

const builtinSquadTemplatesRoot = "builtin_squad_templates"

// SquadRoleSlot is one non-leader seat in a template's roster.
type SquadRoleSlot struct {
	// TemplateKey is the role template the seat is filled from. Provisioning
	// reuses an existing agent created from that template when the workspace
	// already has one, so staffing both templates does not produce two
	// Implementers.
	TemplateKey string
	// Roles is the localized `squad_member.role` note. It is context for the
	// leader's roster and a label in the UI — it grants nothing.
	Roles map[string]string
}

// SquadTemplate is one entry in the built-in squad roster.
type SquadTemplate struct {
	Key         string
	Version     int32
	DefaultName string
	AvatarEmoji string
	// LeaderTemplateKey names the unlisted coordinator template the leader is
	// created from.
	LeaderTemplateKey string
	Members           []SquadRoleSlot
	Titles            map[string]string
	Descriptions      map[string]string
}

// Instructions returns the leader briefing copied into squad.instructions.
func (t SquadTemplate) Instructions() string {
	body, err := builtinSquadTemplatesFS.ReadFile(path.Join(builtinSquadTemplatesRoot, t.Key, "INSTRUCTIONS.md"))
	if err != nil {
		return ""
	}
	return strings.TrimRight(string(body), "\n")
}

// Title returns the localized squad name suggestion, falling back to English.
func (t SquadTemplate) Title(language string) string {
	return localizedTemplateString(t.Titles, language, t.DefaultName)
}

// Description returns the localized picker description, falling back to English.
func (t SquadTemplate) Description(language string) string {
	return localizedTemplateString(t.Descriptions, language, "")
}

// TemplateKeys returns every role template this squad needs, leader first. The
// order is the provisioning order and also the order the roster renders in.
func (t SquadTemplate) TemplateKeys() []string {
	keys := make([]string, 0, len(t.Members)+1)
	keys = append(keys, t.LeaderTemplateKey)
	for _, member := range t.Members {
		keys = append(keys, member.TemplateKey)
	}
	return keys
}

// MaxAutonomy returns the highest level any agent in this roster would be created
// at, which is what an autonomy ceiling has to be checked against: staffing one
// squad can mint several agents, and the most privileged one decides whether the
// call is an escalation. Unrecognised keys contribute nothing.
func (t SquadTemplate) MaxAutonomy() AutonomyLevel {
	highest := AutonomyLevel("")
	for _, key := range t.TemplateKeys() {
		role, ok := AgentRoleTemplateByKey(key)
		if !ok || !IsKnownAutonomyLevel(string(role.Autonomy)) {
			continue
		}
		if highest == "" || !AutonomyAtLeast(string(highest), role.Autonomy) {
			highest = role.Autonomy
		}
	}
	return highest
}

var builtinSquadTemplates = []SquadTemplate{
	{
		Key:               "feature-delivery",
		Version:           1,
		DefaultName:       "Feature Delivery Squad",
		AvatarEmoji:       "🚀",
		LeaderTemplateKey: "feature-delivery-lead",
		Members: []SquadRoleSlot{
			{TemplateKey: "product-analyst", Roles: map[string]string{
				"en": "Clarifies the request and writes the acceptance criteria",
				"zh": "澄清需求并给出验收标准",
			}},
			{TemplateKey: "architect", Roles: map[string]string{
				"en": "Decides the approach, boundaries and compatibility",
				"zh": "确定技术方案、边界和兼容性",
			}},
			{TemplateKey: "implementer", Roles: map[string]string{
				"en": "Writes the change and its tests on an isolated branch",
				"zh": "在隔离分支上实现改动和测试",
			}},
			{TemplateKey: "qa-engineer", Roles: map[string]string{
				"en": "Proves the change works and reports the numbers",
				"zh": "验证改动可用并给出真实数据",
			}},
			{TemplateKey: "code-reviewer", Roles: map[string]string{
				"en": "Reads the finished diff and reports must-fix findings",
				"zh": "审查最终 diff 并给出必须修复的问题",
			}},
		},
		Titles: map[string]string{
			"en": "Feature Delivery Squad",
			"zh": "特性交付小队",
		},
		Descriptions: map[string]string{
			"en": "Analysis, design, implementation, testing and review — routed one stage at a time by a lead.",
			"zh": "澄清、设计、实现、测试、审查，由 leader 按阶段逐个派活。",
		},
	},
	{
		Key:               "bug-fix",
		Version:           2,
		DefaultName:       "Bug Fix Squad",
		AvatarEmoji:       "🐞",
		LeaderTemplateKey: "bug-fix-lead",
		Members: []SquadRoleSlot{
			{TemplateKey: "product-analyst", Roles: map[string]string{
				"en": "Triages the report: reproduction, expected vs observed, severity",
				"zh": "定性缺陷报告：复现步骤、预期与实际、严重级别",
			}},
			{TemplateKey: "qa-engineer", Roles: map[string]string{
				"en": "Reproduces the failure before a fix and verifies the repaired behavior",
				"zh": "在修复前复现失败，并验证修复后的行为",
			}},
			{TemplateKey: "diagnostician", Roles: map[string]string{
				"en": "Finds the evidence-backed cause when the failure is not yet explained",
				"zh": "失败原因未知时定位带证据的根因",
			}},
			{TemplateKey: "implementer", Roles: map[string]string{
				"en": "Fixes the defect and adds the regression test",
				"zh": "修复缺陷并补上回归测试",
			}},
		},
		Titles: map[string]string{
			"en": "Bug Fix Squad",
			"zh": "缺陷修复小队",
		},
		Descriptions: map[string]string{
			"en": "Triage, reproduce, diagnose unknown causes, fix with a regression test, verify — with severity deciding what escalates.",
			"zh": "定性、复现；原因不明时先做带证据的诊断，再带回归测试修复、验证；严重级别决定何时上报。",
		},
	},
	// The remaining six use the listed working roles. What differs is the
	// routing policy, which is the point: a squad is an orchestration of roles, so
	// two squads sharing a roster but disagreeing about sequencing and about what
	// escalates are genuinely different squads. Incident and Bug Fix are the clearest
	// pair — same people, opposite priorities.
	//
	// Order is product-controlled and runs low-privilege to high: the two rosters
	// carrying a Release Engineer (Operator) sit last, so the cost of a mis-click in
	// the picker increases monotonically down the list.
	{
		Key:               "review-gate",
		Version:           3,
		DefaultName:       "Review Gate Squad",
		AvatarEmoji:       "🚧",
		LeaderTemplateKey: "review-gate-lead",
		Members: []SquadRoleSlot{
			{TemplateKey: "code-reviewer", Roles: map[string]string{
				"en": "Reads the diff for correctness and reports must-fix findings",
				"zh": "从正确性角度审查 diff，给出必须修复的问题",
			}},
			{TemplateKey: "security-reviewer", Roles: map[string]string{
				"en": "Reviews authorization, secrets, injection and data exposure",
				"zh": "审查权限、密钥、注入和数据暴露",
			}},
			{TemplateKey: "qa-engineer", Roles: map[string]string{
				"en": "Checks the change is covered by tests that would catch its regression",
				"zh": "检查改动是否有能抓住回归的测试覆盖",
			}},
			{TemplateKey: "agent-evaluator", Roles: map[string]string{
				"en": "Evaluates Agent, Skill and MCP changes with versioned cases before the gate closes",
				"zh": "在门禁关闭前用版本化用例评测 Agent、Skill 和 MCP 改动",
			}},
			{TemplateKey: "experience-validation-engineer", Roles: map[string]string{
				"en": "Validates critical browser/client journeys and accessibility evidence before the gate closes",
				"zh": "门禁关闭前验证关键浏览器/客户端路径和可访问性证据",
			}},
			{TemplateKey: "migration-reviewer", Roles: map[string]string{
				"en": "Reviews migration compatibility, validation and recovery evidence before the gate closes",
				"zh": "门禁关闭前审查迁移兼容性、校验和恢复证据",
			}},
		},
		Titles: map[string]string{
			"en": "Review Gate Squad",
			"zh": "合并门禁小队",
		},
		Descriptions: map[string]string{
			"en": "Reads a finished change for correctness, security and test coverage, then reports one verdict.",
			"zh": "从正确性、安全性和测试覆盖三个角度读完一份改动，给出统一结论。",
		},
	},
	{
		Key:               "discovery",
		Version:           1,
		DefaultName:       "Discovery Squad",
		AvatarEmoji:       "🔭",
		LeaderTemplateKey: "discovery-lead",
		Members: []SquadRoleSlot{
			{TemplateKey: "product-analyst", Roles: map[string]string{
				"en": "Establishes what is being asked for and what would make it decidable",
				"zh": "厘清到底在要什么，以及怎样才算可判断",
			}},
			{TemplateKey: "architect", Roles: map[string]string{
				"en": "Answers whether it is feasible here, and at what cost",
				"zh": "回答在这个系统里可不可行、代价是什么",
			}},
		},
		Titles: map[string]string{
			"en": "Discovery Squad",
			"zh": "需求预研小队",
		},
		Descriptions: map[string]string{
			"en": "Turns \"can we do this?\" into a decision a human can make — no code, just the answer and its cost.",
			"zh": "把「这个能不能做」变成人可以拍板的输入：不写代码，只给结论和代价。",
		},
	},
	{
		Key:               "docs",
		Version:           1,
		DefaultName:       "Docs Squad",
		AvatarEmoji:       "📚",
		LeaderTemplateKey: "docs-lead",
		Members: []SquadRoleSlot{
			{TemplateKey: "technical-writer", Roles: map[string]string{
				"en": "Writes the changelog, API docs and runbook the change requires",
				"zh": "写出这次改动需要的变更日志、API 文档和运维手册",
			}},
			{TemplateKey: "code-reviewer", Roles: map[string]string{
				"en": "Checks the documentation against the code it describes",
				"zh": "对照被描述的代码核对文档准确性",
			}},
		},
		Titles: map[string]string{
			"en": "Docs Squad",
			"zh": "文档同步小队",
		},
		Descriptions: map[string]string{
			"en": "Documents a change that already shipped, and checks the prose against the code.",
			"zh": "为已经落地的改动补文档，并对照代码核准确性。",
		},
	},
	{
		Key:               "maintenance",
		Version:           3,
		DefaultName:       "Maintenance Squad",
		AvatarEmoji:       "🧹",
		LeaderTemplateKey: "maintenance-lead",
		Members: []SquadRoleSlot{
			{TemplateKey: "security-reviewer", Roles: map[string]string{
				"en": "Confirms an advisory applies here and that the fix closes it",
				"zh": "确认安全公告在本项目是否成立，以及修复是否真的闭合",
			}},
			{TemplateKey: "diagnostician", Roles: map[string]string{
				"en": "Finds why a flaky test or upgrade failure occurs when the cause is unknown",
				"zh": "不稳定测试或升级失败原因未知时定位根因",
			}},
			{TemplateKey: "implementer", Roles: map[string]string{
				"en": "Makes one upgrade or cleanup at a time, with its tests",
				"zh": "一次只做一个升级或清理，并带上测试",
			}},
			{TemplateKey: "qa-engineer", Roles: map[string]string{
				"en": "Proves the upgrade changed nothing that was working",
				"zh": "验证升级没有改变任何本来正常的行为",
			}},
			{TemplateKey: "migration-reviewer", Roles: map[string]string{
				"en": "Reviews compatibility and recovery evidence when an upgrade changes a contract or stored data",
				"zh": "升级改变契约或存储数据时审查兼容性和恢复证据",
			}},
		},
		Titles: map[string]string{
			"en": "Maintenance Squad",
			"zh": "例行维护小队",
		},
		Descriptions: map[string]string{
			"en": "Dependency upgrades, advisories and flaky tests — diagnose unknown failures, then change one item at a time.",
			"zh": "依赖升级、安全公告和不稳定测试：原因未知时先诊断，再一次处理一项，并确保可回滚。",
		},
	},
	{
		Key:               "release",
		Version:           4,
		DefaultName:       "Release Squad",
		AvatarEmoji:       "📦",
		LeaderTemplateKey: "release-lead",
		Members: []SquadRoleSlot{
			{TemplateKey: "qa-engineer", Roles: map[string]string{
				"en": "Verifies the build that will actually ship",
				"zh": "验证真正会发出去的那个构建",
			}},
			{TemplateKey: "technical-writer", Roles: map[string]string{
				"en": "Writes the release notes and the upgrade steps",
				"zh": "写发布说明和升级步骤",
			}},
			{TemplateKey: "release-engineer", Roles: map[string]string{
				"en": "Prepares the release and rollback, and asks a human before touching production",
				"zh": "准备发布与回滚方案；触及生产前先向人类申请审批",
			}},
			{TemplateKey: "reliability-engineer", Roles: map[string]string{
				"en": "Records the same-artifact baseline, observation window and recovery signals after release",
				"zh": "发布后记录同一产物的基线、观察窗口和恢复信号",
			}},
			{TemplateKey: "agent-evaluator", Roles: map[string]string{
				"en": "Checks Agent/Skill/MCP evaluation evidence and drift before the release decision",
				"zh": "发布决策前核对 Agent、Skill、MCP 的评测证据和漂移",
			}},
			{TemplateKey: "experience-validation-engineer", Roles: map[string]string{
				"en": "Validates critical journeys and accessibility on the exact artifact before approval",
				"zh": "审批前在同一产物上验证关键路径和可访问性",
			}},
			{TemplateKey: "migration-reviewer", Roles: map[string]string{
				"en": "Checks old-client compatibility, migration validation and rollback limits before approval",
				"zh": "审批前核对旧客户端兼容性、迁移校验和回滚限制",
			}},
		},
		Titles: map[string]string{
			"en": "Release Squad",
			"zh": "发布小队",
		},
		Descriptions: map[string]string{
			"en": "Verify, document, then ship behind a human approval — with the rollback written before the release runs.",
			"zh": "先验证、再写文档，然后在人工审批后发布；回滚方案先于发布动作存在。",
		},
	},
	{
		Key:               "incident",
		Version:           3,
		DefaultName:       "Incident Response Squad",
		AvatarEmoji:       "🚨",
		LeaderTemplateKey: "incident-lead",
		Members: []SquadRoleSlot{
			{TemplateKey: "release-engineer", Roles: map[string]string{
				"en": "Stops the bleeding: rollback, revert or flag, under human approval",
				"zh": "先止血：在人工审批下回滚、撤销或关开关",
			}},
			{TemplateKey: "product-analyst", Roles: map[string]string{
				"en": "Establishes blast radius: who is affected, since when, how badly",
				"zh": "确定影响面：谁受影响、从什么时候开始、有多严重",
			}},
			{TemplateKey: "implementer", Roles: map[string]string{
				"en": "Writes the mitigation, and the real fix only after service is restored",
				"zh": "先写缓解措施；真正的修复等服务恢复后再做",
			}},
			{TemplateKey: "qa-engineer", Roles: map[string]string{
				"en": "Confirms recovery from the user's side, not from the logs alone",
				"zh": "从用户侧确认已恢复，而不是只看日志",
			}},
			{TemplateKey: "diagnostician", Roles: map[string]string{
				"en": "Owns the separate root-cause follow-up after recovery, without delaying mitigation",
				"zh": "恢复后承接独立的根因调查，不让诊断拖延止血",
			}},
			{TemplateKey: "reliability-engineer", Roles: map[string]string{
				"en": "Measures recovery, SLO impact and missing signals, then proposes prevention work",
				"zh": "量化恢复、SLO 影响和缺失信号，再提出预防工作",
			}},
		},
		Titles: map[string]string{
			"en": "Incident Response Squad",
			"zh": "事故响应小队",
		},
		Descriptions: map[string]string{
			"en": "Restore service first, then hand a separate root-cause follow-up to a diagnostician — rollback beats a fix.",
			"zh": "先恢复服务，再把独立的根因后续任务交给诊断工程师；回滚优于修复。",
		},
	},
}

// SquadTemplates returns the built-in squad roster in product order.
func SquadTemplates() []SquadTemplate {
	out := make([]SquadTemplate, len(builtinSquadTemplates))
	copy(out, builtinSquadTemplates)
	return out
}

// SquadTemplateByKey looks a squad template up by its stable key.
func SquadTemplateByKey(key string) (SquadTemplate, bool) {
	for _, template := range builtinSquadTemplates {
		if template.Key == key {
			return template, true
		}
	}
	return SquadTemplate{}, false
}
