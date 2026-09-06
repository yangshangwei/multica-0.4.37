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
