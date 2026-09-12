package service

// The built-in autopilot roster. Order is product-controlled — the picker renders
// it as given — so this is a slice rather than a map.
//
// Nine templates in two groups, and the grouping is the ordering intent:
//
//   - The first four (workday-repo-audit, release-readiness, daily-change-review,
//     hourly-queue-check) are the general-purpose set. They cover four cadences
//     (workday, weekly, daily, hourly) and both execution modes, so a workspace
//     with no particular problem in mind finds a reasonable default in the first
//     row of the picker.
//   - The next five are the specific automations: review queue, triage,
//     reporting, dependencies, documentation. They are narrower — a team adopts
//     one because it already has that need — so they sit after the general set
//     rather than pushing it below the fold.
//
// Order within each group is by how often a template is expected to be adopted,
// not by cadence or alphabetically. Appending to the end is the safe edit;
// reordering changes what the picker shows first and is a product decision.
//
// Every template here runs against the workspace's own repository, issues and
// project state. None of them requires reaching the public internet, which is
// what makes the roster adoptable on an air-gapped or intranet deployment — the
// reason the web-search digest that used to sit in the specific group is gone.
//
// Across the roster, run_only templates (workday-repo-audit, hourly-queue-check,
// stale-pr-reminder, dependency-audit, documentation-check) are patrols: they run,
// and only file an issue when there is something real to file. create_issue
// templates (release-readiness, daily-change-review, bug-triage,
// weekly-progress-report) are summaries: every period lands one issue by design,
// and each stamps {{date}} into the issue title so a month of runs produces
// distinguishable issues rather than thirty rows named "Daily Change Review".
var builtinAutopilotTemplates = []AutopilotTemplate{
	{
		Key:            "workday-repo-audit",
		Version:        1,
		Listed:         true,
		CronExpression: "0 9 * * 1-5",
		ExecutionMode:  "run_only",
		AvatarEmoji:    "🩺",
		Category:       "repo-health",
		Categories: map[string]string{
			"en": "Repo Health",
			"zh": "仓库健康",
			"ko": "레포 상태",
			"ja": "リポジトリ健全性",
		},
		Titles: map[string]string{
			"en": "Workday Repo Audit",
			"zh": "工作日仓库巡检",
			"ko": "평일 레포 점검",
			"ja": "平日リポジトリ監査",
		},
		Descriptions: map[string]string{
			"en": "Checks dependencies, failing tests, and risky open changes every workday.",
			"zh": "每个工作日检查依赖风险、测试失败和滞留变更，仅在发现实质问题时创建或补充任务。",
			"ko": "평일마다 의존성, 실패한 테스트, 위험한 열린 변경을 점검합니다.",
			"ja": "平日ごとに依存関係・失敗したテスト・リスクのある未処理の変更を確認します。",
		},
	},
	{
		Key:                "release-readiness",
		Version:            1,
		Listed:             true,
		CronExpression:     "0 17 * * 1",
		ExecutionMode:      "create_issue",
		IssueTitleTemplate: "发布准备检查 — {{date}}",
		AvatarEmoji:        "🚦",
		Category:           "release-prep",
		Categories: map[string]string{
			"en": "Release Prep",
			"zh": "发布准备",
			"ko": "릴리스 준비",
			"ja": "リリース準備",
		},
		Titles: map[string]string{
			"en": "Release Readiness",
			"zh": "发布准备检查",
			"ko": "릴리스 준비 상태",
			"ja": "リリース準備状況",
		},
		Descriptions: map[string]string{
			"en": "Prepares a weekly release-risk summary from the current project state.",
			"zh": "每周核对已完成事项、发布阻塞和验证结果，汇总发布风险与建议。",
			"ko": "현재 프로젝트 상태를 바탕으로 주간 릴리스 리스크 요약을 준비합니다.",
			"ja": "現在のプロジェクト状況から週次のリリースリスク要約を作成します。",
		},
	},
	{
		Key:                "daily-change-review",
		Version:            1,
		Listed:             true,
		CronExpression:     "0 18 * * *",
		ExecutionMode:      "create_issue",
		IssueTitleTemplate: "每日变更回顾 — {{date}}",
		AvatarEmoji:        "🔎",
		Category:           "periodic-review",
		Categories: map[string]string{
			"en": "Periodic Review",
			"zh": "定期评审",
			"ko": "정기 리뷰",
			"ja": "定期レビュー",
		},
		Titles: map[string]string{
			"en": "Daily Change Review",
			"zh": "每日变更回顾",
			"ko": "일일 변경 리뷰",
			"ja": "日次変更レビュー",
		},
		Descriptions: map[string]string{
			"en": "Scans recent work and flags correctness, UX, and test-coverage risks.",
			"zh": "回顾最近 24 小时的变更，指出功能正确性、使用体验和测试覆盖方面的风险。",
			"ko": "최근 작업을 살펴 정확성, UX, 테스트 커버리지 리스크를 짚어냅니다.",
			"ja": "直近の作業を走査し、正確性・UX・テストカバレッジのリスクを指摘します。",
		},
	},
	{
		Key:            "hourly-queue-check",
		Version:        1,
		Listed:         true,
		CronExpression: "0 * * * *",
		ExecutionMode:  "run_only",
		AvatarEmoji:    "🧹",
		Category:       "maintenance",
		Categories: map[string]string{
			"en": "Maintenance",
			"zh": "维护",
			"ko": "유지보수",
			"ja": "メンテナンス",
		},
		Titles: map[string]string{
			"en": "Hourly Queue Check",
			"zh": "每小时队列巡检",
			"ko": "시간별 큐 점검",
			"ja": "毎時キュー点検",
		},
		Descriptions: map[string]string{
			"en": "Finds stuck work, stale generated files, and failing local checks.",
			"zh": "每小时检查停滞任务、未同步的生成文件和持续失败的快速检查，合并报告重复问题。",
			"ko": "정체된 작업, 오래된 생성 파일, 실패한 로컬 검증을 찾아냅니다.",
			"ja": "停滞した作業・古い生成ファイル・失敗するローカル検証を見つけます。",
		},
	},
	{
		Key:            "stale-pr-reminder",
		Version:        1,
		Listed:         true,
		CronExpression: "0 10 * * 1-5",
		ExecutionMode:  "run_only",
		AvatarEmoji:    "🔀",
		Category:       "periodic-review",
		Categories: map[string]string{
			"en": "Periodic Review",
			"zh": "定期评审",
			"ko": "정기 리뷰",
			"ja": "定期レビュー",
		},
		Titles: map[string]string{
			"en": "PR review reminder",
			"zh": "PR 评审提醒",
			"ko": "PR 리뷰 리마인더",
			"ja": "PR レビューのリマインダー",
		},
		Descriptions: map[string]string{
			"en": "Flag stale pull requests that need review",
			"zh": "汇总超过 24 小时未获评审的非草稿 PR，按风险和等待时间提醒团队处理。",
			"ko": "리뷰가 필요한 오래된 Pull request를 표시합니다",
			"ja": "レビューが必要な滞留中の Pull request を表示します",
		},
	},
	{
		Key:                "bug-triage",
		Version:            2,
		Listed:             true,
		CronExpression:     "0 9 * * 1-5",
		ExecutionMode:      "create_issue",
		IssueTitleTemplate: "缺陷分级 — {{date}}",
		AvatarEmoji:        "🐛",
		Category:           "triage",
		Categories: map[string]string{
			"en": "Triage",
			"zh": "缺陷分级",
			"ko": "분류",
			"ja": "トリアージ",
		},
		Titles: map[string]string{
			"en": "Bug triage",
			"zh": "缺陷分级",
			"ko": "버그 분류",
			"ja": "バグのトリアージ",
		},
		Descriptions: map[string]string{
			"en": "Assess and prioritize new bug reports",
			"zh": "评估尚未分级的缺陷，在权限允许时设置优先级，并说明依据和下一步建议。",
			"ko": "새 버그 리포트를 평가하고 우선순위를 정합니다",
			"ja": "新しいバグレポートを評価し、優先順位を付けます",
		},
	},
	{
		Key:                "weekly-progress-report",
		Version:            1,
		Listed:             true,
		CronExpression:     "0 17 * * 1",
		ExecutionMode:      "create_issue",
		IssueTitleTemplate: "每周进展报告 — {{date}}",
		AvatarEmoji:        "📊",
		Category:           "release-prep",
		Categories: map[string]string{
			"en": "Release Prep",
			"zh": "发布准备",
			"ko": "릴리스 준비",
			"ja": "リリース準備",
		},
		Titles: map[string]string{
			"en": "Weekly progress report",
			"zh": "每周进展报告",
			"ko": "주간 진행 보고서",
			"ja": "週次進捗レポート",
		},
		Descriptions: map[string]string{
			"en": "Compile a weekly summary of team progress",
			"zh": "汇总近 7 天完成的任务、当前进展和阻塞，并列出可核对的任务数量。",
			"ko": "팀 진행 상황을 주간 요약으로 정리합니다",
			"ja": "チームの進捗を週次でまとめます",
		},
	},
	{
		Key:            "dependency-audit",
		Version:        1,
		Listed:         true,
		CronExpression: "0 8 * * 1",
		ExecutionMode:  "run_only",
		AvatarEmoji:    "🛡️",
		Category:       "repo-health",
		Categories: map[string]string{
			"en": "Repo Health",
			"zh": "仓库健康",
			"ko": "레포 상태",
			"ja": "リポジトリ健全性",
		},
		Titles: map[string]string{
			"en": "Dependency audit",
			"zh": "依赖审计",
			"ko": "의존성 감사",
			"ja": "依存関係の監査",
		},
		Descriptions: map[string]string{
			"en": "Scan for security vulnerabilities and outdated packages",
			"zh": "每周检查依赖漏洞和严重落后的版本，结合实际影响提出处理建议。",
			"ko": "보안 취약점과 오래된 패키지를 점검합니다",
			"ja": "セキュリティ脆弱性と古いパッケージをスキャンします",
		},
	},
	{
		Key:            "documentation-check",
		Version:        1,
		Listed:         true,
		CronExpression: "0 14 * * 1",
		ExecutionMode:  "run_only",
		AvatarEmoji:    "📄",
		Category:       "periodic-review",
		Categories: map[string]string{
			"en": "Periodic Review",
			"zh": "定期评审",
			"ko": "정기 리뷰",
			"ja": "定期レビュー",
		},
		Titles: map[string]string{
			"en": "Documentation check",
			"zh": "文档同步检查",
			"ko": "문서 점검",
			"ja": "ドキュメントの点検",
		},
		Descriptions: map[string]string{
			"en": "Review recent changes for documentation gaps",
			"zh": "核对近 7 天的代码变更与文档，汇总接口、配置和使用说明中的真实缺口。",
			"ko": "최근 변경사항에서 문서가 빠진 부분을 검토합니다",
			"ja": "最近の変更にドキュメントの不足がないか確認します",
		},
	},
}

// AutopilotTemplates returns the templates a person may pick from, in product
// order. Unlisted templates are excluded — see Listed.
func AutopilotTemplates() []AutopilotTemplate {
	out := make([]AutopilotTemplate, 0, len(builtinAutopilotTemplates))
	for _, template := range builtinAutopilotTemplates {
		if template.Listed {
			out = append(out, template)
		}
	}
	return out
}

// AutopilotTemplateByKey looks a template up by its stable key.
func AutopilotTemplateByKey(key string) (AutopilotTemplate, bool) {
	for _, template := range builtinAutopilotTemplates {
		if template.Key == key {
			return template, true
		}
	}
	return AutopilotTemplate{}, false
}
