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
				"ko": "요청을 명확히 하고 수용 기준을 작성",
				"ja": "要望を明確化し受け入れ基準を作成",
			}},
			{TemplateKey: "architect", Roles: map[string]string{
				"en": "Decides the approach, boundaries and compatibility",
				"zh": "确定技术方案、边界和兼容性",
				"ko": "접근 방식, 경계, 호환성을 결정",
				"ja": "方針・境界・互換性を決定",
			}},
			{TemplateKey: "implementer", Roles: map[string]string{
				"en": "Writes the change and its tests on an isolated branch",
				"zh": "在隔离分支上实现改动和测试",
				"ko": "격리 브랜치에서 변경과 테스트를 작성",
				"ja": "隔離ブランチで変更とテストを実装",
			}},
			{TemplateKey: "qa-engineer", Roles: map[string]string{
				"en": "Proves the change works and reports the numbers",
				"zh": "验证改动可用并给出真实数据",
				"ko": "변경이 동작함을 검증하고 수치를 보고",
				"ja": "変更の動作を検証し実測値を報告",
			}},
			{TemplateKey: "code-reviewer", Roles: map[string]string{
				"en": "Reads the finished diff and reports must-fix findings",
				"zh": "审查最终 diff 并给出必须修复的问题",
				"ko": "최종 diff를 읽고 필수 수정 사항을 보고",
				"ja": "最終 diff を読み、必須修正点を報告",
			}},
		},
		Titles: map[string]string{
			"en": "Feature Delivery Squad",
			"zh": "特性交付小队",
			"ko": "기능 delivery 스쿼드",
			"ja": "フィーチャー デリバリー スクワッド",
		},
		Descriptions: map[string]string{
			"en": "Analysis, design, implementation, testing and review — routed one stage at a time by a lead.",
			"zh": "澄清、设计、实现、测试、审查，由 leader 按阶段逐个派活。",
			"ko": "분석·설계·구현·테스트·리뷰를 리드가 단계별로 라우팅합니다.",
			"ja": "分析・設計・実装・テスト・レビューを、リードが段階ごとに割り当てます。",
		},
	},
	{
		Key:               "bug-fix",
		Version:           1,
		DefaultName:       "Bug Fix Squad",
		AvatarEmoji:       "🐞",
		LeaderTemplateKey: "bug-fix-lead",
		Members: []SquadRoleSlot{
			{TemplateKey: "product-analyst", Roles: map[string]string{
				"en": "Triages the report: reproduction, expected vs observed, severity",
				"zh": "定性缺陷报告：复现步骤、预期与实际、严重级别",
				"ko": "리포트 트리아지: 재현, 기대 대비 실제, 심각도",
				"ja": "報告のトリアージ：再現手順、期待と実際、重大度",
			}},
			{TemplateKey: "implementer", Roles: map[string]string{
				"en": "Fixes the defect and adds the regression test",
				"zh": "修复缺陷并补上回归测试",
				"ko": "결함을 수정하고 회귀 테스트를 추가",
				"ja": "不具合を修正し回帰テストを追加",
			}},
			{TemplateKey: "qa-engineer", Roles: map[string]string{
				"en": "Reproduces the defect and verifies the fix",
				"zh": "复现缺陷并验证修复结果",
				"ko": "결함을 재현하고 수정을 검증",
				"ja": "不具合を再現し修正を検証",
			}},
		},
		Titles: map[string]string{
			"en": "Bug Fix Squad",
			"zh": "缺陷修复小队",
			"ko": "버그 수정 스쿼드",
			"ja": "バグ修正スクワッド",
		},
		Descriptions: map[string]string{
			"en": "Triage, reproduce, fix with a regression test, verify — with severity deciding what escalates.",
			"zh": "定性、复现、带回归测试地修复、验证；严重级别决定何时上报。",
			"ko": "트리아지·재현·회귀 테스트 포함 수정·검증, 심각도에 따라 에스컬레이션.",
			"ja": "トリアージ・再現・回帰テスト付き修正・検証。重大度に応じて人へエスカレーション。",
		},
	},
	// The remaining six reuse the same eight working roles. What differs is the
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
		Version:           1,
		DefaultName:       "Review Gate Squad",
		AvatarEmoji:       "🚧",
		LeaderTemplateKey: "review-gate-lead",
		Members: []SquadRoleSlot{
			{TemplateKey: "code-reviewer", Roles: map[string]string{
				"en": "Reads the diff for correctness and reports must-fix findings",
				"zh": "从正确性角度审查 diff，给出必须修复的问题",
				"ko": "정확성 관점에서 diff를 읽고 필수 수정 사항을 보고",
				"ja": "正しさの観点で diff を読み、必須修正点を報告",
			}},
			{TemplateKey: "security-reviewer", Roles: map[string]string{
				"en": "Reviews authorization, secrets, injection and data exposure",
				"zh": "审查权限、密钥、注入和数据暴露",
				"ko": "인가, 시크릿, 인젝션, 데이터 노출을 검토",
				"ja": "認可・シークレット・インジェクション・データ露出を確認",
			}},
			{TemplateKey: "qa-engineer", Roles: map[string]string{
				"en": "Checks the change is covered by tests that would catch its regression",
				"zh": "检查改动是否有能抓住回归的测试覆盖",
				"ko": "회귀를 잡아낼 테스트로 변경이 커버되는지 확인",
				"ja": "回帰を捉えるテストで変更がカバーされているか確認",
			}},
		},
		Titles: map[string]string{
			"en": "Review Gate Squad",
			"zh": "合并门禁小队",
			"ko": "리뷰 게이트 스쿼드",
			"ja": "レビューゲート スクワッド",
		},
		Descriptions: map[string]string{
			"en": "Reads a finished change for correctness, security and test coverage, then reports one verdict.",
			"zh": "从正确性、安全性和测试覆盖三个角度读完一份改动，给出统一结论。",
			"ko": "완성된 변경을 정확성·보안·테스트 커버리지 관점에서 읽고 하나의 결론을 냅니다.",
			"ja": "完成した変更を正しさ・セキュリティ・テスト網羅の観点で読み、一つの結論を出します。",
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
				"ko": "무엇을 요구하는지, 무엇이 판단 가능하게 만드는지 정리",
				"ja": "何が求められているか、何があれば判断できるかを整理",
			}},
			{TemplateKey: "architect", Roles: map[string]string{
				"en": "Answers whether it is feasible here, and at what cost",
				"zh": "回答在这个系统里可不可行、代价是什么",
				"ko": "이 시스템에서 실현 가능한지, 비용이 무엇인지 답변",
				"ja": "このシステムで実現可能か、代償は何かを回答",
			}},
		},
		Titles: map[string]string{
			"en": "Discovery Squad",
			"zh": "需求预研小队",
			"ko": "디스커버리 스쿼드",
			"ja": "ディスカバリー スクワッド",
		},
		Descriptions: map[string]string{
			"en": "Turns \"can we do this?\" into a decision a human can make — no code, just the answer and its cost.",
			"zh": "把「这个能不能做」变成人可以拍板的输入：不写代码，只给结论和代价。",
			"ko": "\"이걸 할 수 있나?\"를 사람이 결정할 수 있는 입력으로 바꿉니다. 코드 없이 결론과 비용만.",
			"ja": "「これはできるか」を人が判断できる材料に変えます。コードは書かず、結論と代償だけ。",
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
				"ko": "변경에 필요한 체인지로그, API 문서, 런북을 작성",
				"ja": "変更に必要な変更履歴・API ドキュメント・Runbook を作成",
			}},
			{TemplateKey: "code-reviewer", Roles: map[string]string{
				"en": "Checks the documentation against the code it describes",
				"zh": "对照被描述的代码核对文档准确性",
				"ko": "문서를 그것이 설명하는 코드와 대조해 확인",
				"ja": "ドキュメントを、それが説明するコードと突き合わせて確認",
			}},
		},
		Titles: map[string]string{
			"en": "Docs Squad",
			"zh": "文档同步小队",
			"ko": "문서 스쿼드",
			"ja": "ドキュメント スクワッド",
		},
		Descriptions: map[string]string{
			"en": "Documents a change that already shipped, and checks the prose against the code.",
			"zh": "为已经落地的改动补文档，并对照代码核准确性。",
			"ko": "이미 반영된 변경을 문서화하고, 문장을 코드와 대조합니다.",
			"ja": "すでに入った変更を文書化し、記述をコードと照合します。",
		},
	},
	{
		Key:               "maintenance",
		Version:           1,
		DefaultName:       "Maintenance Squad",
		AvatarEmoji:       "🧹",
		LeaderTemplateKey: "maintenance-lead",
		Members: []SquadRoleSlot{
			{TemplateKey: "implementer", Roles: map[string]string{
				"en": "Makes one upgrade or cleanup at a time, with its tests",
				"zh": "一次只做一个升级或清理，并带上测试",
				"ko": "한 번에 하나의 업그레이드나 정리를, 테스트와 함께 수행",
				"ja": "一度に一つのアップグレードや整理を、テストとともに実施",
			}},
			{TemplateKey: "qa-engineer", Roles: map[string]string{
				"en": "Proves the upgrade changed nothing that was working",
				"zh": "验证升级没有改变任何本来正常的行为",
				"ko": "업그레이드가 정상 동작을 바꾸지 않았음을 검증",
				"ja": "アップグレードが正常な挙動を変えていないことを検証",
			}},
			{TemplateKey: "security-reviewer", Roles: map[string]string{
				"en": "Confirms an advisory applies here and that the fix closes it",
				"zh": "确认安全公告在本项目是否成立，以及修复是否真的闭合",
				"ko": "권고가 여기에 해당하는지, 수정이 실제로 닫는지 확인",
				"ja": "アドバイザリが自プロジェクトに該当するか、修正が実際に閉じるかを確認",
			}},
		},
		Titles: map[string]string{
			"en": "Maintenance Squad",
			"zh": "例行维护小队",
			"ko": "메인테넌스 스쿼드",
			"ja": "メンテナンス スクワッド",
		},
		Descriptions: map[string]string{
			"en": "Dependency upgrades, advisories and flaky tests — one at a time, each separately revertable.",
			"zh": "依赖升级、安全公告和不稳定测试：一次一件，每件都能单独回滚。",
			"ko": "의존성 업그레이드·권고·플레이키 테스트를 하나씩, 각각 되돌릴 수 있게 처리합니다.",
			"ja": "依存アップグレード・アドバイザリ・不安定テストを一件ずつ、個別に戻せる形で処理します。",
		},
	},
	{
		Key:               "release",
		Version:           1,
		DefaultName:       "Release Squad",
		AvatarEmoji:       "📦",
		LeaderTemplateKey: "release-lead",
		Members: []SquadRoleSlot{
			{TemplateKey: "qa-engineer", Roles: map[string]string{
				"en": "Verifies the build that will actually ship",
				"zh": "验证真正会发出去的那个构建",
				"ko": "실제로 배포될 빌드를 검증",
				"ja": "実際に出荷されるビルドを検証",
			}},
			{TemplateKey: "technical-writer", Roles: map[string]string{
				"en": "Writes the release notes and the upgrade steps",
				"zh": "写发布说明和升级步骤",
				"ko": "릴리스 노트와 업그레이드 절차를 작성",
				"ja": "リリースノートとアップグレード手順を作成",
			}},
			{TemplateKey: "release-engineer", Roles: map[string]string{
				"en": "Prepares the release and rollback, and asks a human before touching production",
				"zh": "准备发布与回滚方案；触及生产前先向人类申请审批",
				"ko": "릴리스와 롤백을 준비하고, 프로덕션에 손대기 전 사람의 승인을 요청",
				"ja": "リリースとロールバックを準備し、本番に触れる前に人間の承認を求める",
			}},
		},
		Titles: map[string]string{
			"en": "Release Squad",
			"zh": "发布小队",
			"ko": "릴리스 스쿼드",
			"ja": "リリース スクワッド",
		},
		Descriptions: map[string]string{
			"en": "Verify, document, then ship behind a human approval — with the rollback written before the release runs.",
			"zh": "先验证、再写文档，然后在人工审批后发布；回滚方案先于发布动作存在。",
			"ko": "검증·문서화 후 사람의 승인을 받아 배포합니다. 롤백은 릴리스 실행 전에 작성됩니다.",
			"ja": "検証・文書化のうえ、人間の承認を得て出荷します。ロールバックはリリース実行より先に用意します。",
		},
	},
	{
		Key:               "incident",
		Version:           1,
		DefaultName:       "Incident Response Squad",
		AvatarEmoji:       "🚨",
		LeaderTemplateKey: "incident-lead",
		Members: []SquadRoleSlot{
			{TemplateKey: "release-engineer", Roles: map[string]string{
				"en": "Stops the bleeding: rollback, revert or flag, under human approval",
				"zh": "先止血：在人工审批下回滚、撤销或关开关",
				"ko": "출혈을 멈춥니다: 사람의 승인 아래 롤백·되돌리기·플래그 차단",
				"ja": "まず止血：人間の承認のもとロールバック・巻き戻し・フラグ停止",
			}},
			{TemplateKey: "product-analyst", Roles: map[string]string{
				"en": "Establishes blast radius: who is affected, since when, how badly",
				"zh": "确定影响面：谁受影响、从什么时候开始、有多严重",
				"ko": "영향 범위를 확정: 누가, 언제부터, 얼마나 심각하게 영향받는지",
				"ja": "影響範囲を確定：誰が、いつから、どの程度影響を受けているか",
			}},
			{TemplateKey: "implementer", Roles: map[string]string{
				"en": "Writes the mitigation, and the real fix only after service is restored",
				"zh": "先写缓解措施；真正的修复等服务恢复后再做",
				"ko": "완화 조치를 작성하고, 실제 수정은 서비스 복구 후에 진행",
				"ja": "緩和策を実装し、本当の修正は復旧後に着手",
			}},
			{TemplateKey: "qa-engineer", Roles: map[string]string{
				"en": "Confirms recovery from the user's side, not from the logs alone",
				"zh": "从用户侧确认已恢复，而不是只看日志",
				"ko": "로그만이 아니라 사용자 관점에서 복구를 확인",
				"ja": "ログだけでなくユーザー側から復旧を確認",
			}},
		},
		Titles: map[string]string{
			"en": "Incident Response Squad",
			"zh": "事故响应小队",
			"ko": "인시던트 대응 스쿼드",
			"ja": "インシデント対応 スクワッド",
		},
		Descriptions: map[string]string{
			"en": "Restore service first, diagnose second — rollback beats a fix, and the post-mortem is a separate issue.",
			"zh": "先恢复服务，再查原因：回滚优于修复，复盘另开 issue。",
			"ko": "먼저 서비스를 복구하고 그 다음 원인을 봅니다. 롤백이 수정보다 우선이며, 회고는 별도 이슈입니다.",
			"ja": "まず復旧、次に原因。ロールバックが修正に優先し、振り返りは別 issue にします。",
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
