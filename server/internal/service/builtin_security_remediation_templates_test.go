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
	if leader.Listed || leader.Version != 1 || leader.Autonomy != AutonomyCoordinator || leader.MaxConcurrentTasks != 6 {
		t.Errorf("leader defaults = listed:%t version:%d autonomy:%s concurrency:%d, want false/1/coordinator/6",
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
	if squad.Version != 1 || squad.MaxAutonomy() != AutonomyCoordinator {
		t.Errorf("squad version/ceiling = %d/%s, want 1/coordinator", squad.Version, squad.MaxAutonomy())
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
