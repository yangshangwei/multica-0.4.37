package service

// The built-in roster. Order is product-controlled — the picker renders it as
// given, roughly in the order a change moves through a team — so this is a slice
// rather than a map.
//
// Deliberately nine roles, not twenty. Technology-specific variants (frontend,
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
			"ko": "진행 보고 담당자",
			"ja": "進捗レポーター",
		},
		Descriptions: map[string]string{
			"en": "Writes daily and weekly progress reports from real issue history, with traceable counts, blockers and data gaps.",
			"zh": "根据任务和状态变更历史生成日报、周报，说明进展、阻塞、可核对的统计和数据缺口。",
			"ko": "실제 태스크와 상태 변경 이력을 바탕으로 일일·주간 보고서를 작성하고, 진행 상황·차단 요인·검증 가능한 집계·누락 데이터를 정리합니다.",
			"ja": "タスクと状態変更の履歴から日報・週報を作成し、進捗、ブロッカー、確認可能な集計、データの不足を示します。",
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
		Version:            2,
		DefaultName:        "Bug Fix Lead",
		AvatarEmoji:        "🚑",
		Autonomy:           AutonomyCoordinator,
		MaxConcurrentTasks: 2,
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
			"ko": "리뷰 게이트 리드",
			"ja": "レビューゲート リード",
		},
		Descriptions: map[string]string{
			"en": "Routes a finished change through code, security and verification review, then reports one verdict.",
			"zh": "把已完成的改动依次交给代码审查、安全审查和验证，最后给出一个结论。",
			"ko": "완성된 변경을 코드·보안·검증 리뷰로 라우팅한 뒤 하나의 결론을 보고합니다.",
			"ja": "完成した変更をコード・セキュリティ・検証レビューに回し、一つの結論を報告します。",
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
			"ko": "디스커버리 리드",
			"ja": "ディスカバリー リード",
		},
		Descriptions: map[string]string{
			"en": "Turns an open question into a decidable one — criteria and an approach, with no code written.",
			"zh": "把一个开放问题变成可决策的输入：验收标准和技术方案，不写任何代码。",
			"ko": "열린 질문을 판단 가능한 형태로 바꿉니다: 기준과 접근 방식, 코드는 작성하지 않습니다.",
			"ja": "未確定の問いを判断可能にします。基準と方針を示し、コードは書きません。",
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
			"ko": "문서 리드",
			"ja": "ドキュメント リード",
		},
		Descriptions: map[string]string{
			"en": "Routes documentation that a shipped change requires, and has its accuracy checked against the diff.",
			"zh": "安排已合并改动所需的文档，并对照 diff 核对准确性。",
			"ko": "배포된 변경에 필요한 문서를 배분하고, diff와 대조해 정확성을 검증합니다.",
			"ja": "リリース済みの変更に必要なドキュメントを割り当て、diff と照合して正確性を確認します。",
		},
	},
	{
		Key:                "maintenance-lead",
		Version:            2,
		DefaultName:        "Maintenance Lead",
		AvatarEmoji:        "🧹",
		Autonomy:           AutonomyCoordinator,
		MaxConcurrentTasks: 2,
		Titles: map[string]string{
			"en": "Maintenance Lead",
			"zh": "例行维护负责人",
			"ko": "유지보수 리드",
			"ja": "メンテナンス リード",
		},
		Descriptions: map[string]string{
			"en": "Routes upgrades, CVE patches and flaky-test cleanup one item at a time, never as a batch.",
			"zh": "把依赖升级、CVE 修补和不稳定测试清理逐项安排，绝不打包。",
			"ko": "업그레이드, CVE 패치, 불안정 테스트 정리를 한 건씩 배분하며 묶어 처리하지 않습니다.",
			"ja": "アップグレード・CVE 対応・不安定テストの整理を一件ずつ割り当て、まとめて扱いません。",
		},
	},
	{
		Key:                "release-lead",
		Version:            2,
		DefaultName:        "Release Lead",
		AvatarEmoji:        "📦",
		Autonomy:           AutonomyCoordinator,
		MaxConcurrentTasks: 2,
		Titles: map[string]string{
			"en": "Release Lead",
			"zh": "发布负责人",
			"ko": "릴리스 리드",
			"ja": "リリース リード",
		},
		Descriptions: map[string]string{
			"en": "Gets a release verified, documented and rollback-ready before a human approves the production step.",
			"zh": "在人类批准生产动作之前，先让发布通过验证、文档齐备、回滚方案就绪。",
			"ko": "사람이 프로덕션 단계를 승인하기 전에 릴리스의 검증·문서·롤백 준비를 마칩니다.",
			"ja": "人間が本番作業を承認する前に、リリースの検証・文書・ロールバック準備を整えます。",
		},
	},
	{
		Key:                "incident-lead",
		Version:            2,
		DefaultName:        "Incident Lead",
		AvatarEmoji:        "🚨",
		Autonomy:           AutonomyCoordinator,
		MaxConcurrentTasks: 2,
		Titles: map[string]string{
			"en": "Incident Lead",
			"zh": "事故响应负责人",
			"ko": "인시던트 리드",
			"ja": "インシデント リード",
		},
		Descriptions: map[string]string{
			"en": "Stops the bleeding first: mitigation over diagnosis, rollback over a fix, root cause in a separate issue.",
			"zh": "先止血：缓解优先于定性，回滚优先于修复，根因另开 issue。",
			"ko": "먼저 출혈을 막습니다: 진단보다 완화, 수정보다 롤백, 근본 원인은 별도 이슈로.",
			"ja": "まず止血します。診断より緩和、修正よりロールバック、根本原因は別 issue へ。",
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
