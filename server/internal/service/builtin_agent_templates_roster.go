package service

// The built-in roster. Order is product-controlled — the picker renders it as
// given, roughly in the order a change moves through a team — so this is a slice
// rather than a map.
//
// Deliberately eight roles, not twenty. Technology-specific variants (frontend,
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
			"ko": "프로덕트 분석가",
			"ja": "プロダクトアナリスト",
		},
		Descriptions: map[string]string{
			"en": "Turns a vague request into a decidable one: open questions, acceptance criteria, risks, and how to split the work.",
			"zh": "把模糊的需求变成可判断的需求：澄清问题、验收标准、风险和拆分建议。",
			"ko": "모호한 요청을 판단 가능한 형태로 정리합니다: 미해결 질문, 수용 기준, 리스크, 분할 제안.",
			"ja": "曖昧な要望を判断できる形にします。未解決の質問、受け入れ基準、リスク、分割案を示します。",
		},
	},
	{
		Key:                "architect",
		Version:            1,
		Listed:             true,
		DefaultName:        "Architect",
		AvatarEmoji:        "📐",
		Autonomy:           AutonomyObserver,
		MaxConcurrentTasks: 2,
		RoleSkills:         []string{"multica-architecture-decision-record"},
		Titles: map[string]string{
			"en": "Architect",
			"zh": "架构师",
			"ko": "아키텍트",
			"ja": "アーキテクト",
		},
		Descriptions: map[string]string{
			"en": "Proposes the technical approach: boundaries, contracts, data flow, and the trade-off that decided it.",
			"zh": "给出技术方案：边界、接口、数据流，以及做出选择的取舍理由。",
			"ko": "기술 방향을 제안합니다: 경계, 계약, 데이터 흐름, 그리고 선택의 근거.",
			"ja": "技術方針を提案します。境界、契約、データフロー、そして選定理由となるトレードオフ。",
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
			"ko": "구현 엔지니어",
			"ja": "実装エンジニア",
		},
		Descriptions: map[string]string{
			"en": "Writes the code and its tests on an isolated branch, then reports what changed and how it was verified.",
			"zh": "在隔离分支上实现代码和测试，并说明改了什么、如何验证。",
			"ko": "격리된 브랜치에서 코드와 테스트를 작성하고, 변경 내용과 검증 방법을 보고합니다.",
			"ja": "隔離ブランチでコードとテストを書き、変更点と検証方法を報告します。",
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
			"ko": "QA 엔지니어",
			"ja": "QA エンジニア",
		},
		Descriptions: map[string]string{
			"en": "Decides what would prove the change works, automates it, and reproduces defects with a minimal case.",
			"zh": "决定用什么证明改动可用，把它自动化，并用最小用例复现缺陷。",
			"ko": "변경이 동작함을 입증할 방법을 정하고 자동화하며, 최소 사례로 결함을 재현합니다.",
			"ja": "変更が動く証明方法を決めて自動化し、最小ケースで不具合を再現します。",
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
			"ko": "코드 리뷰어",
			"ja": "コードレビュアー",
		},
		Descriptions: map[string]string{
			"en": "Reads the diff only and reports findings with file, line, severity, and the failure each one causes.",
			"zh": "只读 diff，按文件、行号、严重级别报告问题，并说明每个问题会导致什么后果。",
			"ko": "diff만 읽고 파일·행·심각도와 그로 인한 실패를 함께 보고합니다.",
			"ja": "diff のみを読み、ファイル・行・重大度と、それが引き起こす不具合を報告します。",
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
			"ko": "보안 리뷰어",
			"ja": "セキュリティレビュアー",
		},
		Descriptions: map[string]string{
			"en": "Reviews authorization, secrets, injection, dependencies and data exposure, and never writes an exploit.",
			"zh": "审查权限、密钥、注入、依赖和数据暴露；不编写可用的攻击代码。",
			"ko": "인가, 시크릿, 인젝션, 의존성, 데이터 노출을 검토하며 실제 공격 코드는 작성하지 않습니다.",
			"ja": "認可・シークレット・インジェクション・依存関係・データ露出を確認し、実用的な攻撃コードは書きません。",
		},
	},
	{
		Key:                "release-engineer",
		Version:            1,
		Listed:             true,
		DefaultName:        "Release Engineer",
		AvatarEmoji:        "🚦",
		Autonomy:           AutonomyOperator,
		MaxConcurrentTasks: 1,
		RoleSkills:         []string{"multica-release-check"},
		Titles: map[string]string{
			"en": "Release Engineer",
			"zh": "发布工程师",
			"ko": "릴리스 엔지니어",
			"ja": "リリースエンジニア",
		},
		Descriptions: map[string]string{
			"en": "Prepares the release and the rollback, and asks a human before anything touches production.",
			"zh": "准备发布和回滚方案；任何影响生产的动作都先向人类申请审批。",
			"ko": "릴리스와 롤백을 준비하고, 프로덕션에 영향을 주는 작업은 먼저 사람의 승인을 받습니다.",
			"ja": "リリースとロールバックを準備し、本番に触れる操作は必ず人間の承認を求めます。",
		},
	},
	{
		Key:                "technical-writer",
		Version:            1,
		Listed:             true,
		DefaultName:        "Technical Writer",
		AvatarEmoji:        "📝",
		Autonomy:           AutonomyObserver,
		MaxConcurrentTasks: 3,
		RoleSkills:         []string{"multica-documentation-change"},
		Titles: map[string]string{
			"en": "Technical Writer",
			"zh": "技术文档工程师",
			"ko": "테크니컬 라이터",
			"ja": "テクニカルライター",
		},
		Descriptions: map[string]string{
			"en": "Writes the changelog, API docs and runbook that the change requires, from the change itself.",
			"zh": "根据改动本身写出需要的变更日志、API 文档和运维手册。",
			"ko": "변경 자체를 근거로 체인지로그, API 문서, 런북을 작성합니다.",
			"ja": "変更そのものを根拠に、変更履歴・API ドキュメント・Runbook を書きます。",
		},
	},
	// The two squad-leader definitions. Unlisted: they are provisioned by the squad
	// templates, which also create the roster the leader routes to.
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
			"ko": "기능 delivery 리드",
			"ja": "フィーチャー デリバリー リード",
		},
		Descriptions: map[string]string{
			"en": "Routes a feature through analysis, design, implementation, testing and review, one member at a time.",
			"zh": "把一个特性按澄清、设计、实现、测试、审查的顺序逐个交给合适的成员。",
			"ko": "기능을 분석·설계·구현·테스트·리뷰 순서로 한 명씩 라우팅합니다.",
			"ja": "機能を分析・設計・実装・テスト・レビューの順に、一人ずつ割り当てます。",
		},
	},
	{
		Key:                "bug-fix-lead",
		Version:            1,
		DefaultName:        "Bug Fix Lead",
		AvatarEmoji:        "🚑",
		Autonomy:           AutonomyCoordinator,
		MaxConcurrentTasks: 2,
		RoleSkills:         []string{"multica-requirement-clarification"},
		Titles: map[string]string{
			"en": "Bug Fix Lead",
			"zh": "缺陷修复负责人",
			"ko": "버그 수정 리드",
			"ja": "バグ修正リード",
		},
		Descriptions: map[string]string{
			"en": "Triages a defect, gets it reproduced, then routes the fix and its regression test.",
			"zh": "先定性缺陷并确认可复现，再分别安排修复和回归测试。",
			"ko": "결함을 트리아지하고 재현을 확인한 뒤, 수정과 회귀 테스트를 배분합니다.",
			"ja": "不具合をトリアージして再現を確認し、修正と回帰テストを割り当てます。",
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
