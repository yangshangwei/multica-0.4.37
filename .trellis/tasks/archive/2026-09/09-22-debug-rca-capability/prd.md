# 补齐调试/根因分析(RCA)研发能力

## Goal / 用户价值

为 Multica 内置研发智能体体系补齐「深度调试 / 根因分析(RCA)」能力——面对一个可观察的失败,系统性地定位其真正原因。当前 8 个内置研发工作技能覆盖需求→设计→实现→测试→审查→文档→发布,唯独没有「定位失败根因」的纪律:bug-fix 小队从「最小复现」直接跳到「实现工程师修复」、两次尝试失败才升级;incident 小队明确把根因「另建 issue」却无人承接。价值:让 RCA 成为可交付、可核验、可委派的能力,agent 以「复现→最小化→假设→插桩/二分→定位→带证据结论」取代猜测式修复。

## Key decisions / 已定决策

- **D1 形态** = 技能 + 新「诊断工程师(`diagnostician`)」角色;**不**改动 bug-fix/incident 现有路由(接入小队作为后续独立任务)。
- **D2 自主级别** = `contributor`:可在隔离分支自行复现、加临时插桩、写「无修复即失败」的测试锁定根因;**不**提交正式修复(交实现工程师)。
- **D3 一角色一技能**(roster 的 `RoleSkills ≤ 1` 硬约束):`diagnostician` → `multica-debugging`。

## Dependency / 依赖状态（2026-09-23 更新）

`09-22-skill-lifecycle-taxonomy` 已提交并归档，原阻塞已解除：功能提交为 `28c91e13d`，归档提交为 `2350999ce`。技能主分类已由 6 类扩到 8 类，code-review / security-review / test-report 归入 `quality`，ADR 归入 `design`。

本任务已基于该状态实现。`multica-debugging` 使用 `quality` 分类和 `microscope` 图标，`PresentationDefaults` 测试与新分类一致。新增角色、skill、注册表和测试均已在工作区，不再处于暂停实现状态。

## Current status / 当前状态

实现、完整验收、提交与任务归档均已完成。两处文本契约已修正：疑似或未查明的调查允许如实交付缺失证据；涉及数据丢失、安全缺陷或凭据泄露时，报告后停止并交由人工处理。最终验证证据记录在 `verification.md`，独立文本评审记录在 `text-review.md`，工作提交为 `e348b009a`。本任务未发布。

## Background / 已确认事实(读源码)

- 角色技能装配 `server/internal/service/builtin_role_skills.go`:`//go:embed builtin_role_skills`;技能必须登记进 `builtinRoleSkillVersions`,否则 `RoleSkillTemplateByName` 返回 false;frontmatter 经 `skill.ParseSkillFrontmatterMeta` 取 description/category/icon。现有 8 个技能均无 `references/` 子目录 → 本技能也不需要 source-map(CLAUDE.md 的 source-map 规则针对 `builtin_skills/*` 操作技能)。
- 角色模板 `builtin_agent_templates.go`(`AgentRoleTemplate`)+ `builtin_agent_templates_roster.go`;角色提示词是独立文件 `builtin_agent_templates/<Key>/INSTRUCTIONS.md`(简体中文,创建时拷贝进 `agent.instructions`)。autonomy 阶梯 observer(1) < contributor(2) < coordinator(3) < operator(4)。四语言 `en/zh/ko/ja`。
- 相邻行为锚点:`builtin_squad_templates/bug-fix/INSTRUCTIONS.md`(分诊→最小复现→修复+回归→验证;两次失败升级)、`.../incident/INSTRUCTIONS.md`(先止血、回滚优先、根因另开 issue)。

## Requirements / 需求

- **R1** 新增 `builtin_role_skills/multica-debugging/SKILL.md`（简体中文，`category: quality`、`icon: microscope`）。承载 RCA 方法：何时使用 / 提交调查前先读什么 / 复现与最小化 / 形成并验证假设（插桩、日志、二分、对照）/ 结论必须有证据 / 输出格式 / 以下情况停止并询问人工。
- **R2** 边界写入正文：`multica-test-report` 验证变更是否可用（包括修复后的回归验证）；`multica-code-review` 静态读 diff 找潜在缺陷；本 skill 在修复前定位已观察失败的原因，填补 bug-fix 流程中“最小复现 → 修复”之间的系统性调查。
- **R3** 证据契约:每个根因结论可追溯到复现/观测/插桩输出;未证实写为假设,不冒充定论(对齐 code-review/security-review「无证据不算发现」)。
- **R4** 新增 `diagnostician` 角色:`builtin_agent_templates/diagnostician/INSTRUCTIONS.md`(含测试要求的 `## 职责` `## 输入` `## 交付格式` `## 不负责的事项` `## 完成标准` `## 需要人工介入的情况`;无 `{{` 占位符)+ roster 条目(Listed、contributor、`RoleSkills:["multica-debugging"]`、四语言、emoji、MaxConcurrentTasks)。
- **R5** 登记 `builtinRoleSkillVersions["multica-debugging"]=1` 并同步所有被计数/清单 pin 的测试(见 AC2)。

## Acceptance Criteria / 验收

- [x] **AC1** `cd server && go build ./...` 通过。
- [x] **AC2** 完整 service/handler 测试通过，数据库用例实际执行；记录原有跳过项。当前计数和清单测试为 `TestAgentRoleTemplates_ListedRosterIsTen`、`DefaultRoleSkills`、`AutonomyDefaults`（operator 仍为 1）、`TestRoleSkillTemplates_RosterIsNine`、`PresentationDefaults`。
- [x] **AC3** `RoleSkillTemplateByName("multica-debugging")` 返回内容;排序位在 `multica-code-review` 之后、`multica-documentation-change` 之前。
- [x] **AC4** `AgentRoleTemplateByKey("diagnostician")`:Listed、Autonomy=contributor、RoleSkills=["multica-debugging"]、四语言完整、emoji 非空且不与现有 9 个重复、MaxConcurrentTasks≥1。
- [x] **AC5** 角色 INSTRUCTIONS.md 通过 `TestAgentRoleTemplates_InstructionsCarryTheContract`(六段标题 + 无占位符)。
- [x] **AC6** `microscope` 通过 `skill.IsIconName`、`quality` 通过 `skill.IsCategory`。
- [x] **AC7** 文本语义评审：skill 与角色分工清晰、与 test-report/code-review 边界明确；R2/R3 契约在文本中可见。由独立审查代理复核，并记录证据和局限。
- [x] **AC8** 新增 `TestDiagnostician_EvidenceContract`(仿 `TestProgressReporter_EvidenceAndCloseoutContract`),把 R2/R3 关键词钉进 CI。

## Out of scope / 暂不包含

- 接入 bug-fix/incident 小队路由、或新增 RCA 小队模板(后续独立任务;squads skill 要求这类改动单独确认)。
- 其他 Tier-1 缺口(性能、可观测性、CI/CD & IaC、大型重构)。
- release-v0-4-47 的任何改动。
- 新增前端页面或路由。角色 picker 继续读取模板 API；工作区并发补充的 debugging skill 识别与四语言展示属于现有展示表同步，验证范围见 `verification.md`。
