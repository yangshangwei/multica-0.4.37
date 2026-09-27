package service

// The built-in roster. Order is product-controlled — the picker renders it as
// given, roughly in the order a change moves through a team — so this is a slice
// rather than a map.
//
// Deliberately fourteen roles, not twenty. Technology-specific variants (frontend,
// backend, mobile, data) are NOT separate templates: they are the same
// Implementer with different skills and project resources attached. Splitting by
// stack multiplies prompts that all say the same thing and leaves a team picking
// between agents that differ in one paragraph.
var builtinAgentRoleTemplates = []AgentRoleTemplate{
	{
		Key:                "product-analyst",
		Version:            1,
		Listed:             true,
		DefaultName:        "Product Analyst",
		AvatarEmoji:        "🔍",
		Autonomy:           AutonomyObserver,
		MaxConcurrentTasks: 3,
		RoleSkills:         []string{"multica-requirement-clarification"},
		Titles: map[string]string{
			"en": "Product Analyst",
			"zh": "产品分析师",
		},
		Descriptions: map[string]string{
			"en": "Turns a vague request into a decidable one: open questions, acceptance criteria, risks, and how to split the work.",
			"zh": "把模糊的需求变成可判断的需求：澄清问题、验收标准、风险和拆分建议。",
		},
	},
	{
		Key:                "architect",
		Version:            2,
		Listed:             true,
		DefaultName:        "Architect",
		AvatarEmoji:        "📐",
		Autonomy:           AutonomyObserver,
		MaxConcurrentTasks: 2,
		RoleSkills:         []string{"multica-architecture-decision-record"},
		Titles: map[string]string{
			"en": "Architect",
			"zh": "架构师",
		},
		Descriptions: map[string]string{
			"en": "Proposes the technical approach: boundaries, contracts, data flow, and the trade-off that decided it.",
			"zh": "给出技术方案：边界、接口、数据流，以及做出选择的取舍理由。",
		},
	},
	{
		Key:                "implementer",
		Version:            1,
		Listed:             true,
		DefaultName:        "Implementer",
		AvatarEmoji:        "🛠️",
		Autonomy:           AutonomyContributor,
		MaxConcurrentTasks: 1,
		RoleSkills:         []string{"multica-test-report"},
		Titles: map[string]string{
			"en": "Implementer",
			"zh": "实现工程师",
		},
		Descriptions: map[string]string{
			"en": "Writes the code and its tests on an isolated branch, then reports what changed and how it was verified.",
			"zh": "在隔离分支上实现代码和测试，并说明改了什么、如何验证。",
		},
	},
	{
		Key:                "qa-engineer",
		Version:            1,
		Listed:             true,
		DefaultName:        "QA Engineer",
		AvatarEmoji:        "🧪",
		Autonomy:           AutonomyContributor,
		MaxConcurrentTasks: 2,
		RoleSkills:         []string{"multica-test-report"},
		Titles: map[string]string{
			"en": "QA Engineer",
			"zh": "测试工程师",
		},
		Descriptions: map[string]string{
			"en": "Decides what would prove the change works, automates it, and reproduces defects with a minimal case.",
			"zh": "决定用什么证明改动可用，把它自动化，并用最小用例复现缺陷。",
		},
	},
	{
		Key:                "diagnostician",
		Version:            1,
		Listed:             true,
		DefaultName:        "Diagnostician",
		AvatarEmoji:        "🩺",
		Autonomy:           AutonomyContributor,
		MaxConcurrentTasks: 1,
		RoleSkills:         []string{"multica-debugging"},
		Titles: map[string]string{
			"en": "Diagnostician",
			"zh": "诊断工程师",
		},
		Descriptions: map[string]string{
			"en": "Turns an observed failure into an evidence-backed root cause: reproduces and minimizes it, tests hypotheses, and hands the fix direction to the implementer.",
			"zh": "把一个可观察的失败查成带证据的根因结论：复现并最小化，提出并验证假设，给出修复方向；不提交正式修复。",
		},
	},
	{
		Key:                "code-reviewer",
		Version:            1,
		Listed:             true,
		DefaultName:        "Code Reviewer",
		AvatarEmoji:        "🔬",
		Autonomy:           AutonomyObserver,
		MaxConcurrentTasks: 3,
		RoleSkills:         []string{"multica-code-review"},
		Titles: map[string]string{
			"en": "Code Reviewer",
			"zh": "代码审查员",
		},
		Descriptions: map[string]string{
			"en": "Reads the diff only and reports findings with file, line, severity, and the failure each one causes.",
			"zh": "只读 diff，按文件、行号、严重级别报告问题，并说明每个问题会导致什么后果。",
		},
	},
	{
		Key:                "security-reviewer",
		Version:            1,
		Listed:             true,
		DefaultName:        "Security Reviewer",
		AvatarEmoji:        "🛡️",
		Autonomy:           AutonomyObserver,
		MaxConcurrentTasks: 2,
		RoleSkills:         []string{"multica-security-review"},
		Titles: map[string]string{
			"en": "Security Reviewer",
			"zh": "安全审查员",
		},
		Descriptions: map[string]string{
			"en": "Reviews authorization, secrets, injection, dependencies and data exposure, and never writes an exploit.",
			"zh": "审查权限、密钥、注入、依赖和数据暴露；不编写可用的攻击代码。",
		},
	},
	{
		Key:                "release-engineer",
		Version:            2,
		Listed:             true,
		DefaultName:        "Release Engineer",
		AvatarEmoji:        "🚦",
		Autonomy:           AutonomyOperator,
		MaxConcurrentTasks: 1,
		RoleSkills:         []string{"multica-release-check"},
		Titles: map[string]string{
			"en": "Release Engineer",
			"zh": "发布工程师",
		},
		Descriptions: map[string]string{
			"en": "Prepares the release and the rollback, and asks a human before anything touches production.",
			"zh": "准备发布和回滚方案；任何影响生产的动作都先向人类申请审批。",
		},
	},
	{
		Key:                "technical-writer",
		Version:            2,
		Listed:             true,
		DefaultName:        "Technical Writer",
		AvatarEmoji:        "📝",
		Autonomy:           AutonomyContributor,
		MaxConcurrentTasks: 3,
		RoleSkills:         []string{"multica-documentation-change"},
		Titles: map[string]string{
			"en": "Technical Writer",
			"zh": "技术文档工程师",
		},
		Descriptions: map[string]string{
			"en": "Writes the changelog, API docs and runbook that the change requires, from the change itself.",
			"zh": "根据改动本身写出需要的变更日志、API 文档和运维手册。",
		},
	},
	{
		Key:                "progress-reporter",
		Version:            1,
		Listed:             true,
		DefaultName:        "Progress Reporter",
		AvatarEmoji:        "📊",
		Autonomy:           AutonomyContributor,
		MaxConcurrentTasks: 1,
		RoleSkills:         []string{"multica-progress-report"},
		Titles: map[string]string{
			"en": "Progress Reporter",
			"zh": "进展报告员",
		},
		Descriptions: map[string]string{
			"en": "Writes daily and weekly progress reports from real issue history, with traceable counts, blockers and data gaps.",
			"zh": "根据任务和状态变更历史生成日报、周报，说明进展、阻塞、可核对的统计和数据缺口。",
		},
	},
	{
		Key:                "reliability-engineer",
		Version:            1,
		Listed:             true,
		DefaultName:        "Reliability Engineer",
		AvatarEmoji:        "📡",
		Autonomy:           AutonomyContributor,
		MaxConcurrentTasks: 1,
		RoleSkills:         []string{"multica-reliability-engineering"},
		Titles: map[string]string{
			"en": "Reliability Engineer",
			"zh": "可靠性工程师",
		},
		Descriptions: map[string]string{
			"en": "Turns runtime signals into SLO, error-budget, capacity and recovery evidence without taking unapproved production actions.",
			"zh": "把运行信号整理成 SLO、错误预算、容量和恢复证据；不执行未经批准的生产操作。",
		},
	},
	{
		Key:                "agent-evaluator",
		Version:            1,
		Listed:             true,
		DefaultName:        "Agent Evaluator",
		AvatarEmoji:        "🤖",
		Autonomy:           AutonomyObserver,
		MaxConcurrentTasks: 2,
		RoleSkills:         []string{"multica-agent-evaluation"},
		Titles: map[string]string{
			"en": "Agent Evaluator",
			"zh": "智能体评测工程师",
		},
		Descriptions: map[string]string{
			"en": "Evaluates Agent, Skill and MCP changes with versioned cases for correctness, safety, cost, latency and drift.",
			"zh": "用版本化用例评测 Agent、Skill 和 MCP 变更的正确性、安全性、成本、延迟与漂移。",
		},
	},
	{
		Key:                "experience-validation-engineer",
		Version:            1,
		Listed:             true,
		DefaultName:        "Experience Validation Engineer",
		AvatarEmoji:        "🖥️",
		Autonomy:           AutonomyContributor,
		MaxConcurrentTasks: 2,
		RoleSkills:         []string{"multica-experience-validation"},
		Titles: map[string]string{
			"en": "Experience Validation Engineer",
			"zh": "体验验证工程师",
		},
		Descriptions: map[string]string{
			"en": "Validates critical user journeys across browsers and clients with accessibility evidence and a concrete outcome risk.",
			"zh": "用浏览器、跨端和可访问性证据验证关键用户路径，并说明结果风险。",
		},
	},
	{
		Key:                "migration-reviewer",
		Version:            2,
		Listed:             true,
		DefaultName:        "Migration Reviewer",
		AvatarEmoji:        "🗃️",
		Autonomy:           AutonomyObserver,
		MaxConcurrentTasks: 2,
		RoleSkills:         []string{"multica-migration-review"},
		Titles: map[string]string{
			"en": "Migration Reviewer",
			"zh": "迁移审查员",
		},
		Descriptions: map[string]string{
			"en": "Reviews schema, API and client migrations for compatibility windows, validation, retry and rollback evidence.",
			"zh": "审查 schema、API 和客户端迁移的兼容窗口、校验、重试和回滚证据。",
		},
	},
	// The squad-leader definitions, one per built-in squad template. Unlisted: they
	// are provisioned by the squad templates, which also create the roster the leader
	// routes to.
	//
	// Coordinator rather than Observer, because routing IS the work: a leader has to
	// mention members and, on the issues its own squad owns, move the parent forward.
	// The Squad Operating Protocol injected at claim time supplies the mechanics; these
	// instructions supply the judgement about who gets what.
	{
		Key:                "feature-delivery-lead",
		Version:            1,
		DefaultName:        "Feature Delivery Lead",
		AvatarEmoji:        "🧭",
		Autonomy:           AutonomyCoordinator,
		MaxConcurrentTasks: 2,
		RoleSkills:         []string{"multica-requirement-clarification"},
		Titles: map[string]string{
			"en": "Feature Delivery Lead",
			"zh": "特性交付负责人",
		},
		Descriptions: map[string]string{
			"en": "Routes a feature through analysis, design, implementation, testing and review, one member at a time.",
			"zh": "把一个特性按澄清、设计、实现、测试、审查的顺序逐个交给合适的成员。",
		},
	},
	{
		Key:                "bug-fix-lead",
		Version:            3,
		DefaultName:        "Bug Fix Lead",
		AvatarEmoji:        "🚑",
		Autonomy:           AutonomyCoordinator,
		MaxConcurrentTasks: 2,
		Titles: map[string]string{
			"en": "Bug Fix Lead",
			"zh": "缺陷修复负责人",
		},
		Descriptions: map[string]string{
			"en": "Triages a defect, gets it reproduced, diagnoses unknown causes, then routes the fix and its regression test.",
			"zh": "先定性缺陷并确认可复现；原因未知时先安排诊断，再分别安排修复和回归测试。",
		},
	},
	{
		Key:                "review-gate-lead",
		Version:            2,
		DefaultName:        "Review Gate Lead",
		AvatarEmoji:        "🚧",
		Autonomy:           AutonomyCoordinator,
		MaxConcurrentTasks: 2,
		Titles: map[string]string{
			"en": "Review Gate Lead",
			"zh": "合并门禁负责人",
		},
		Descriptions: map[string]string{
			"en": "Routes a finished change through code, security and verification review, then reports one verdict.",
			"zh": "把已完成的改动依次交给代码审查、安全审查和验证，最后给出一个结论。",
		},
	},
	{
		Key:                "discovery-lead",
		Version:            1,
		DefaultName:        "Discovery Lead",
		AvatarEmoji:        "🔭",
		Autonomy:           AutonomyCoordinator,
		MaxConcurrentTasks: 2,
		RoleSkills:         []string{"multica-requirement-clarification"},
		Titles: map[string]string{
			"en": "Discovery Lead",
			"zh": "需求预研负责人",
		},
		Descriptions: map[string]string{
			"en": "Turns an open question into a decidable one — criteria and an approach, with no code written.",
			"zh": "把一个开放问题变成可决策的输入：验收标准和技术方案，不写任何代码。",
		},
	},
	{
		Key:                "docs-lead",
		Version:            2,
		DefaultName:        "Docs Lead",
		AvatarEmoji:        "📚",
		Autonomy:           AutonomyCoordinator,
		MaxConcurrentTasks: 2,
		Titles: map[string]string{
			"en": "Docs Lead",
			"zh": "文档同步负责人",
		},
		Descriptions: map[string]string{
			"en": "Routes documentation that a shipped change requires, and has its accuracy checked against the diff.",
			"zh": "安排已合并改动所需的文档，并对照 diff 核对准确性。",
		},
	},
	{
		Key:                "maintenance-lead",
		Version:            3,
		DefaultName:        "Maintenance Lead",
		AvatarEmoji:        "🧹",
		Autonomy:           AutonomyCoordinator,
		MaxConcurrentTasks: 2,
		Titles: map[string]string{
			"en": "Maintenance Lead",
			"zh": "例行维护负责人",
		},
		Descriptions: map[string]string{
			"en": "Routes upgrades, CVE patches and flaky-test cleanup one item at a time, diagnosing unknown failures before changes.",
			"zh": "把依赖升级、CVE 修补和不稳定测试清理逐项安排；原因未知时先诊断，绝不打包。",
		},
	},
	{
		Key:                "release-lead",
		Version:            3,
		DefaultName:        "Release Lead",
		AvatarEmoji:        "📦",
		Autonomy:           AutonomyCoordinator,
		MaxConcurrentTasks: 2,
		RoleSkills:         []string{"multica-rollout-and-canary-verification"},
		Titles: map[string]string{
			"en": "Release Lead",
			"zh": "发布负责人",
		},
		Descriptions: map[string]string{
			"en": "Gets a release verified, documented and rollback-ready before a human approves the production step.",
			"zh": "在人类批准生产动作之前，先让发布通过验证、文档齐备、回滚方案就绪。",
		},
	},
	{
		Key:                "incident-lead",
		Version:            4,
		DefaultName:        "Incident Lead",
		AvatarEmoji:        "🚨",
		Autonomy:           AutonomyCoordinator,
		MaxConcurrentTasks: 2,
		RoleSkills:         []string{"multica-incident-learning"},
		Titles: map[string]string{
			"en": "Incident Lead",
			"zh": "事故响应负责人",
		},
		Descriptions: map[string]string{
			"en": "Stops the bleeding first: mitigation over diagnosis, rollback over a fix, then hands a separate root-cause follow-up to a diagnostician.",
			"zh": "先止血：缓解优先于定性，回滚优先于修复；恢复后把独立根因后续任务交给诊断工程师。",
		},
	},
}

// AgentRoleTemplates returns the templates a person may pick from, in product
// order. Squad-leader definitions are excluded — see Listed.
func AgentRoleTemplates() []AgentRoleTemplate {
	out := make([]AgentRoleTemplate, 0, len(builtinAgentRoleTemplates))
	for _, template := range builtinAgentRoleTemplates {
		if template.Listed {
			out = append(out, template)
		}
	}
	return out
}

// AllAgentRoleTemplates returns every template including the unlisted squad
// leaders. For provisioning and for tests that must cover the whole registry.
func AllAgentRoleTemplates() []AgentRoleTemplate {
	out := make([]AgentRoleTemplate, len(builtinAgentRoleTemplates))
	copy(out, builtinAgentRoleTemplates)
	return out
}

// AgentRoleTemplateByKey looks a template up by its stable key.
func AgentRoleTemplateByKey(key string) (AgentRoleTemplate, bool) {
	for _, template := range builtinAgentRoleTemplates {
		if template.Key == key {
			return template, true
		}
	}
	return AgentRoleTemplate{}, false
}
