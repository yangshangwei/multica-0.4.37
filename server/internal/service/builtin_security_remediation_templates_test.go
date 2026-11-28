package service

import (
	"regexp"
	"slices"
	"strings"
	"testing"
)

func TestAgentRoleTemplates_SecurityRemediationLead(t *testing.T) {
	leader, ok := AgentRoleTemplateByKey("security-remediation-lead")
	if !ok {
		t.Fatal("security-remediation-lead is missing")
	}
	if leader.Listed || leader.Version != 3 || leader.Autonomy != AutonomyCoordinator || leader.MaxConcurrentTasks != 6 {
		t.Errorf("leader defaults = listed:%t version:%d autonomy:%s concurrency:%d, want false/3/coordinator/6",
			leader.Listed, leader.Version, leader.Autonomy, leader.MaxConcurrentTasks)
	}
	if len(leader.RoleSkills) != 0 {
		t.Errorf("leader skills = %v, want none: working members own security methods", leader.RoleSkills)
	}
	if leader.DefaultName != "Security Remediation Lead" || leader.Title("zh") != "安全漏洞修复负责人" {
		t.Errorf("leader names = %q / %q", leader.DefaultName, leader.Title("zh"))
	}
}

func TestSquadTemplates_SecurityRemediationRoster(t *testing.T) {
	squad, ok := SquadTemplateByKey("security-remediation")
	if !ok {
		t.Fatal("security-remediation is missing")
	}
	wantKeys := []string{
		"security-remediation-lead", "product-analyst", "security-reviewer",
		"diagnostician", "implementer", "qa-engineer", "migration-reviewer", "code-reviewer",
	}
	if !slices.Equal(squad.TemplateKeys(), wantKeys) {
		t.Errorf("roster = %v, want %v", squad.TemplateKeys(), wantKeys)
	}
	if squad.Version != 3 || squad.MaxAutonomy() != AutonomyCoordinator {
		t.Errorf("squad version/ceiling = %d/%s, want 3/coordinator", squad.Version, squad.MaxAutonomy())
	}
	if squad.DefaultName != "Security Remediation Squad" || squad.Title("zh") != "安全漏洞修复小队" || squad.AvatarEmoji != "🛡️" {
		t.Errorf("squad presentation = %q / %q / %q", squad.DefaultName, squad.Title("zh"), squad.AvatarEmoji)
	}
	for _, language := range TemplateLanguages {
		if squad.Titles[language] == "" || squad.Descriptions[language] == "" {
			t.Errorf("squad has no %s catalog copy", language)
		}
	}
	for _, language := range []string{"ja", "ko", "unknown"} {
		if squad.Title(language) != squad.Title("en") || squad.Description(language) != squad.Description("en") {
			t.Errorf("%s must fall back to English", language)
		}
	}
}

// The instructions are the behavior added by this template. Pin the safety and
// handoff requirements in each body; concatenating them could conceal a conflict
// or omission in the leader's persisted instructions or its squad briefing.
func TestSquadTemplates_SecurityRemediationInstructionContracts(t *testing.T) {
	squad, ok := SquadTemplateByKey("security-remediation")
	if !ok {
		t.Fatal("security-remediation is missing")
	}
	leader, ok := AgentRoleTemplateByKey(squad.LeaderTemplateKey)
	if !ok {
		t.Fatal("security-remediation leader is missing")
	}
	contracts := map[string][]string{
		"triage": {
			"type: dependency | code | other", "source:", "affected_surface:",
			"affected_versions:", "reachability:", "affected_assets:",
			"severity: critical | high | medium | low | unknown",
			"exploitability: confirmed | suspected | unknown | not-applicable",
			"evidence_state: confirmed | suspected | unknown",
			"production_impact: yes | no | unknown",
			"route: maintenance | bug-fix | review-gate | incident | human",
			"blocked_actions:", "next_owner:",
		},
		"routing": {
			"dependency", "适用性", "锁文件", "来源", "迁移审查员", "schema", "API", "已安装客户端",
			"code", "原因未知", "诊断工程师", "实现工程师", "一次只修一个",
			"other", "信任边界", "hold", "unknown", "补证", "负责人",
			"测试工程师", "代码审查员", "安全审查员", "不允许修改者自审",
		},
		"handoff": {
			"当前花名册", "[@名称](mention://agent/<UUID>)", "只派一个下一步",
			"`action`", "退出本轮", "source_task_id", "author_id", "delegated_from_task_id",
			"`delegation`", "触发评论", "成员本人", "`no_action`", "`in_review`",
			"绝不将父任务标记为 `done`",
		},
		"stop": {
			"当前生产可利用", "凭据泄露", "客户数据暴露", "停止本地执行", "人工",
			"`incident`", "`release`", "目标不存在或不可调用", "逐项人工审批",
			"不编写漏洞利用", "不读取真实 secret", "生产发布", "线上迁移", "外部通知", "不可逆删除",
			"不覆盖成员的停止规则", "停止自动派活并交人工", "不扩大 Access", "不提供沙箱",
		},
		"evidence": {
			"真实命令", "退出码", "测试计数", "无修复时失败", "回滚限制", "残余风险",
			"未运行", "模拟", "真实模型",
		},
	}
	fixedUUID := regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)
	for name, body := range map[string]string{"leader": leader.Instructions(), "squad": squad.Instructions()} {
		t.Run(name, func(t *testing.T) {
			for group, markers := range contracts {
				t.Run(group, func(t *testing.T) {
					for _, marker := range markers {
						if !strings.Contains(body, marker) {
							t.Errorf("missing %q", marker)
						}
					}
				})
			}
			if fixedUUID.MatchString(body) {
				t.Error("instructions contain a fixed UUID instead of the current roster identity")
			}
		})
	}
}

func TestSquadTemplates_SecurityRemediationParentStatusOwnership(t *testing.T) {
	squad, ok := SquadTemplateByKey("security-remediation")
	if !ok {
		t.Fatal("security-remediation is missing")
	}
	leader, ok := AgentRoleTemplateByKey(squad.LeaderTemplateKey)
	if !ok {
		t.Fatal("security-remediation leader is missing")
	}
	for name, body := range map[string]string{"leader": leader.Instructions(), "squad": squad.Instructions()} {
		t.Run(name, func(t *testing.T) {
			_, section, found := strings.Cut(body, "## 父任务状态\n")
			if !found {
				t.Fatal("parent-status section is missing")
			}
			section, _, _ = strings.Cut(section, "\n## ")
			permission := "只有任务实际分配给本小队，且注入的 Squad Operating Protocol 授予父任务状态管理权时"
			for _, contract := range []string{
				permission, "Agent Identity 的禁止规则优先", "来宾 @squad", "`quick-create`",
				"保持当前负责人和状态不变", "只交付评论和活动记录", "不得自行改派任务来取得状态管理权",
				"绝不将父任务标记为 `done`",
			} {
				if !strings.Contains(section, contract) {
					t.Errorf("parent-status section is missing ownership contract %q", contract)
				}
			}
			for _, state := range []string{"`in_progress`", "`in_review`"} {
				position := strings.Index(section, state)
				if position < 0 || strings.Index(section, permission) < 0 || strings.Index(section, permission) > position {
					t.Errorf("%s transition must follow the actual-assignment and injected-ownership condition", state)
				}
			}
		})
	}
}

// securityTriageV1Block is the triage block exactly as security-remediation v1
// shipped it. Workspaces hold copies of it, so a later version may only append.
const securityTriageV1Block = `## Security triage
- type: dependency | code | other
- source: <公告、报告或文件来源>
- affected_surface: <组件、代码路径、端点或部署>
- affected_versions: <版本、锁文件或部署标识；未知则 unknown>
- reachability: <可达条件及其证据；未知则 unknown>
- affected_assets: <受影响资产及数据类别，不含原始敏感数据>
- severity: critical | high | medium | low | unknown
- exploitability: confirmed | suspected | unknown | not-applicable
- evidence_state: confirmed | suspected | unknown
- production_impact: yes | no | unknown
- route: maintenance | bug-fix | review-gate | incident | human
- blocked_actions: <禁止的生产、凭据、外部通知或破坏性动作>
- next_owner: <实际成员或人工负责人及下一步>`

// securityRemediationInstructions returns both instruction bodies, keyed by the
// name the subtests report. A template upgrade never rewrites the copies a
// workspace already holds. Mixed saved versions are possible, so every
// current contract has to hold in each new body on its own.
func securityRemediationInstructions(t *testing.T) map[string]string {
	t.Helper()
	squad, ok := SquadTemplateByKey("security-remediation")
	if !ok {
		t.Fatal("security-remediation is missing")
	}
	leader, ok := AgentRoleTemplateByKey(squad.LeaderTemplateKey)
	if !ok {
		t.Fatal("security-remediation leader is missing")
	}
	return map[string]string{"leader": leader.Instructions(), "squad": squad.Instructions()}
}

// markdownHeadingLevel reports the level of an ATX heading line, or 0.
func markdownHeadingLevel(line string) int {
	level := len(line) - len(strings.TrimLeft(line, "#"))
	if level == 0 || level > 6 || !strings.HasPrefix(line[level:], " ") {
		return 0
	}
	return level
}

// markdownSections returns the text under every line equal to heading, each
// running to the next heading of the same or a higher level. Headings inside
// fenced code do not count: the triage block opens with one.
func markdownSections(body, heading string) []string {
	level := markdownHeadingLevel(heading)
	var sections []string
	var current []string
	open, fenced := false, false
	closeSection := func() {
		if open {
			sections = append(sections, strings.TrimRight(strings.Join(current, "\n"), "\n"))
			current, open = nil, false
		}
	}
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			fenced = !fenced
		}
		if lineLevel := markdownHeadingLevel(line); !fenced && lineLevel > 0 {
			if lineLevel <= level {
				closeSection()
			}
			if line == heading {
				open = true
				continue
			}
		}
		if open {
			current = append(current, line)
		}
	}
	closeSection()
	return sections
}

func soleMarkdownSection(t *testing.T, name, body, heading string) string {
	t.Helper()
	sections := markdownSections(body, heading)
	if len(sections) != 1 {
		t.Fatalf("%s instructions have %d %q sections, want exactly 1", name, len(sections), heading)
	}
	return sections[0]
}

// firstDifferentLine returns the 1-based number of the first line on which the
// two texts differ, with that line from each.
func firstDifferentLine(a, b string) (int, string, string) {
	left, right := strings.Split(a, "\n"), strings.Split(b, "\n")
	for i := 0; i < len(left) || i < len(right); i++ {
		var l, r string
		if i < len(left) {
			l = left[i]
		}
		if i < len(right) {
			r = right[i]
		}
		if l != r {
			return i + 1, l, r
		}
	}
	return 0, "", ""
}

// V3 replaces mandatory human gates with evidence checkpoints. Test each saved
// body independently so a stale copy cannot silently restore the prerequisite.
func TestSquadTemplates_SecurityRemediationV3InstructionContracts(t *testing.T) {
	contracts := map[string][]string{
		"automatic_checkpoints": {"G1", "G2", "不要求人类创建者", "不要求小队里有人类安全负责人", "无需人工放行评论", "当前批次", "缺失或失败不算通过"},
		"restricted_return":     {"审查员的结果评论不会唤醒 leader", "平台", "真实委派", "执行结束", "原队长", "成功、失败或缺少结果", "通知不是审查结论", "不通过 @ 或回复启动其他智能体"},
		"result_provenance":     {"source_task_id", "delegated_from_task_id", "author_id", "`type` 为 `comment`", "`system`", "`progress_update`", "最近一次委派", "G1 的结果、退回前的结果都不能充当这一次的结果", "不带 `--compact`"},
		"triage_fields":         {"finding_id:", "cwe:", "severity_basis:", "exploit_maturity:", "disposition:", "not_affected_reason:", "reproduction:", "reproduction_ref:", "fix_ref:", "backport_range:", "due:", "disclosure:"},
		"triage_updates":        {"更新时在本轮的新评论里重写完整分诊块，不编辑以前的评论", "需要派活时与本轮唯一的派活评论合并", "只在 G2 的必需结果齐备并通过之后填", "引用那条评论"},
		"deadlines":             {"自 G1 确认定级起算", "| critical | 7 天 |", "| high | 30 天 |", "| medium | 90 天 |", "| low | 随常规版本 |", "`severity: unknown` 没有默认期限", "不写入任务的截止日期字段", "两份指令的表不一致时以小队指令为准"},
		"decision_integrity":    {"身份不明或评论被修改（`revision` 大于 1）时要求重新确认", "不能把任意评论作者当作批准者", "不采信智能体转述的批准", "不能覆盖成员自己的权限和停止规则"},
		"bounded_recovery":      {"multica issue runs <任务 ID> --output json", "最多重派一次", "不重置", "不得跳过必需审查", "最多返修两轮", "不重复派发"},
		"reproduction":          {"最小安全复现", "测试工程师", "合成数据", "只用本地测试夹具", "不出站", "不接触真实凭据、生产或第三方系统", "可达且可触发即停", "不做载荷串联、数据提取或持久化", "内存破坏与远程执行类停在触发证据", "可脱离测试框架复用的攻击脚本", "不得修改复现测试", "不是同一漏洞的补丁作者"},
		"human_exceptions":      {"风险接受", "不修复", "复查日期", "author_type", "`member`", "原停止条件", "不扩大 Access", "不修改小队指令", "不修改本小队任何智能体（含自己）的指令、技能或配置"},
		"evidence":              {"旧结论作废", "每项结论列出来源评论、作者和所针对的提交", "不出具通过", "以原始输出为准", "`disclosure: need-to-know`", "派生任务的标题和描述", "只引用 `finding_id`", "没有任务级可见性控制"},
	}
	forbidden := []string{"## 人工闸门", "创建者也不是人类时不发闸门", "小队缺少安全负责人", "负责人在 G1 选定", "修复前的安全复现不派给诊断工程师或测试工程师", "只有安全负责人按上述放行方式新发评论"}
	for name, body := range securityRemediationInstructions(t) {
		t.Run(name, func(t *testing.T) {
			for group, markers := range contracts {
				t.Run(group, func(t *testing.T) {
					for _, marker := range markers {
						if !strings.Contains(body, marker) {
							t.Errorf("missing %q", marker)
						}
					}
				})
			}
			for _, old := range forbidden {
				if strings.Contains(body, old) {
					t.Errorf("obsolete human-gate rule %q remains", old)
				}
			}
		})
	}
}

// Evidence checks remain mandatory even though no human release is required.
func TestSquadTemplates_SecurityRemediationAutomaticCheckpointDecisions(t *testing.T) {
	for name, body := range securityRemediationInstructions(t) {
		t.Run(name, func(t *testing.T) {
			checkpoint := soleMarkdownSection(t, name, body, "## 自动检查点")
			for _, marker := range []string{"G1：复现和修复之前", "G2：收口之前", "结果齐备", "同一个修复提交", "失败通知", "不能代替", "不得跳过必需审查"} {
				if !strings.Contains(checkpoint, marker) {
					t.Errorf("checkpoint missing %q", marker)
				}
			}
		})
	}
}

// The injected Squad Operating Protocol owns receipts, activity outcomes and
// retries. v1 restated them in its own words, in places at odds with the
// protocol, and told the leader to wait for a member even when that member's
// result never wakes it. None of that wording may come back.
func TestSquadTemplates_SecurityRemediationLeavesReceiptsToTheProtocol(t *testing.T) {
	removed := []string{
		"等待成员本人返回", "写入结果不明时先回读", "确认写入后记录", "确认委派写入后记录",
		// The closing turn posts a summary and may move the parent, which the
		// protocol records as `action`; v1 fixed it to `no_action`.
		"再记录 `no_action` 并退出",
	}
	for name, body := range securityRemediationInstructions(t) {
		t.Run(name, func(t *testing.T) {
			for _, phrase := range removed {
				if strings.Contains(body, phrase) {
					t.Errorf("v1 wording %q is back", phrase)
				}
			}
		})
	}
}

// Four sections say the same thing in both bodies. They are compared byte for
// byte so that an edit to one copy cannot leave the leader reading two versions
// of a gate, a triage field or a reproduction limit.
func TestSquadTemplates_SecurityRemediationSharedSections(t *testing.T) {
	bodies := securityRemediationInstructions(t)
	for _, heading := range []string{"## 分诊记录", "## 自动检查点", "## 最小安全复现", "## 保密与披露"} {
		t.Run(heading, func(t *testing.T) {
			leader := soleMarkdownSection(t, "leader", bodies["leader"], heading)
			squad := soleMarkdownSection(t, "squad", bodies["squad"], heading)
			if strings.TrimSpace(leader) == "" {
				t.Fatal("section is empty")
			}
			if leader != squad {
				line, inLeader, inSquad := firstDifferentLine(leader, squad)
				t.Errorf("section differs between the bodies at line %d:\nleader: %q\nsquad:  %q", line, inLeader, inSquad)
			}
		})
	}
}

// Preserve the established triage prefix across template upgrades so recorded
// findings keep the same field meanings.
func TestSquadTemplates_SecurityRemediationTriageBlockExtendsV1(t *testing.T) {
	for name, body := range securityRemediationInstructions(t) {
		t.Run(name, func(t *testing.T) {
			section := soleMarkdownSection(t, name, body, "## 分诊记录")
			_, block, found := strings.Cut(section, "```text\n")
			if !found {
				t.Fatal("triage section has no fenced block")
			}
			block, _, found = strings.Cut(block, "\n```")
			if !found {
				t.Fatal("triage block is not closed")
			}
			if !strings.HasPrefix(block, securityTriageV1Block+"\n") {
				t.Error("triage block must open with the unchanged v1 block and continue after it")
			}
		})
	}
}

// A patch written before the reproduction cannot show that the test fails
// without it. The squad body carries the step order, so the code route has to
// introduce the reproduction before it names the implementer.
func TestSquadTemplates_SecurityRemediationCodeRouteReproducesBeforeFixing(t *testing.T) {
	section := soleMarkdownSection(t, "squad", securityRemediationInstructions(t)["squad"], "### code：程序代码漏洞")
	reproduction := strings.Index(section, "最小安全复现")
	implementer := strings.Index(section, "实现工程师")
	if reproduction < 0 || implementer < 0 {
		t.Fatalf("code route names the reproduction at %d and the implementer at %d, want both present", reproduction, implementer)
	}
	if reproduction > implementer {
		t.Error("code route names the implementer before the minimal security reproduction")
	}
}

// Two kinds of turn leave the leader without its whole brief. A human who
// replies to the leader's own comment, or mentions the leader agent directly,
// starts an ordinary task: no Squad Operating Protocol, no roster and no squad
// instructions. A runtime that reads only the head of the context file keeps
// the protocol and drops the roster or the end of the squad instructions. The
// role body is the one text present on both, so each rule lives there and
// nowhere else. The squad body carries a check of its own for the one case the
// role body cannot cover; see the preamble test below.
func TestAgentRoleTemplates_SecurityRemediationLeadWithoutSquadContext(t *testing.T) {
	rules := []string{
		"看不到 Squad Operating Protocol：这不是 leader 任务，不派活、不放行、不改状态",
		`看得到 Squad Operating Protocol，却看不到花名册或小队指令的"最终证据"小节：本轮指令被运行时截断`,
		// On that turn the issue read gives the assignee's id, not the squad's name.
		"multica squad get <assignee_id> --output json",
		// How far a cut brief reaches into the protocol depends on the autonomy
		// policy printed before it; the record step is not always inside.
		"协议的记录步骤也看不到时，这条评论就是记录",
	}
	bodies := securityRemediationInstructions(t)
	for _, rule := range rules {
		if !strings.Contains(bodies["leader"], rule) {
			t.Errorf("leader instructions are missing %q", rule)
		}
		if strings.Contains(bodies["squad"], rule) {
			t.Errorf("squad instructions carry %q, a rule for a turn on which they are missing or cut", rule)
		}
	}
	// The reply a person looks for when a runtime has cut the brief.
	if !strings.Contains(bodies["leader"], "本轮指令不完整") {
		t.Error(`leader instructions are missing "本轮指令不完整"`)
	}
}

// A workspace that already has a v1 leader reuses it for a new squad, and that
// role body has no truncation rule. A runtime that reads only the first 32 KiB
// of the context file then cuts the v2 squad instructions in the middle of the
// gate section, dropping the release checks and every section after them. The
// squad body therefore names its own closing section in its preamble, the one
// part of it such a cut still delivers.
func TestSquadTemplates_SecurityRemediationPreambleDetectsTruncation(t *testing.T) {
	squad := securityRemediationInstructions(t)["squad"]
	preamble, _, found := strings.Cut(squad, "\n## ")
	if !found {
		t.Fatal("squad instructions have no level-2 section")
	}
	for _, marker := range []string{
		`本指令以"最终证据"一节结尾`,
		"不派活、不放行、不改状态",
		"本轮指令不完整",
	} {
		if !strings.Contains(preamble, marker) {
			t.Errorf("squad preamble is missing %q", marker)
		}
	}
}

// The leader's truncation check anchors on this section: it tells a cut brief
// from a whole one by whether it can still see the closing section of the squad
// instructions. The section therefore has to stay last; a cut that removed only
// what came after it would go unnoticed. The v1 squad body ends with the same
// heading, so the anchor also holds for a workspace that still has that copy.
func TestSquadTemplates_SecurityRemediationEndsWithFinalEvidence(t *testing.T) {
	const heading = "## 最终证据"
	squad := securityRemediationInstructions(t)["squad"]
	section := soleMarkdownSection(t, "squad", squad, heading)
	if strings.TrimSpace(section) == "" {
		t.Fatal("section is empty")
	}
	if !strings.HasSuffix(strings.TrimRight(squad, "\n"), heading+"\n"+section) {
		t.Errorf("%q must be the last level-2 section of the squad instructions", heading)
	}
}

// The instructions editor escapes bare markup characters on save. Keep
// placeholders inside fenced blocks or code spans so saved instructions retain
// usable command examples and mention guidance.
func TestSquadTemplates_SecurityRemediationSquadBodySurvivesTheInstructionsEditor(t *testing.T) {
	squad := securityRemediationInstructions(t)["squad"]
	fenced := false
	for number, line := range strings.Split(squad, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			fenced = !fenced
			continue
		}
		if fenced {
			continue
		}
		// Code spans in the body never contain a backtick, so every second
		// piece of a line split on backticks is code.
		for i, piece := range strings.Split(line, "`") {
			if i%2 == 0 && strings.ContainsAny(piece, "<&") {
				t.Errorf("squad instructions line %d uses \"<\" or \"&\" outside code: %q", number+1, piece)
			}
		}
	}
}

// The handler suite counts this sentence across the leader's instructions and
// its briefing together, which cannot tell one copy in each body from two in
// one. New sections that mention status ownership must not repeat it.
func TestSquadTemplates_SecurityRemediationStatesStatusConditionOnce(t *testing.T) {
	const permission = "只有任务实际分配给本小队，且注入的 Squad Operating Protocol 授予父任务状态管理权时"
	for name, body := range securityRemediationInstructions(t) {
		if count := strings.Count(body, permission); count != 1 {
			t.Errorf("%s instructions state the status-ownership condition %d times, want 1", name, count)
		}
	}
}
