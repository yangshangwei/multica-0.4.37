# 七个角色 skill 正文中文化技术方案

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 将七个角色 skill 的正文改为准确、自然的简体中文，并让目标工作区的现有副本生效。

**Architecture:** 保持现有的源码内嵌、工作区副本、智能体绑定及内容 hash 下发链路。源码提供未来首次创建的中文默认正文；现有工作区通过已有 skill 更新接口逐项更新，不增加自动覆盖或数据库迁移。

**Tech Stack:** Markdown、Go embed、现有 Multica CLI/API、Go tests、Vitest。

## 范围与约束

用户已在本次对话明确要求制定方案并执行。实施范围为以下七项的 Markdown 正文：

| 调用名称 | 中文标题 |
|---|---|
| `multica-release-check` | 发布检查 |
| `multica-documentation-change` | 文档更新 |
| `multica-security-review` | 安全审查 |
| `multica-architecture-decision-record` | 架构决策记录 |
| `multica-code-review` | 代码审查 |
| `multica-requirement-clarification` | 需求澄清 |
| `multica-test-report` | 测试报告 |

- 保留各文件原始 YAML frontmatter，包括 `name`、英文 `description`、`user-invocable: false`。
- 保留命令、参数、路径、字段、枚举及约定值，例如 `operator`、`production_release`、`Status: proposed`、`must-fix`。
- 翻译标题、说明、自然语言占位符和报告模板中的人类可读部分。业务术语遵循 `apps/docs/content/docs/developers/conventions.zh.mdx`。
- 按原文保留必须、禁止、适用条件、审批边界和失败处理。性能数值等示例仍然只是示例，不新增产品要求。
- 本次仅改变正文语言；角色职责、权限、调用策略及模型配置保持原有行为。角色指令的英文模板仍可能影响实际输出语言，不将本次修改表述为全局中文输出保证。
- 不新增依赖，不建立双语运行时选择器，不自动覆盖其他工作区，不把当前仓库技术栈写入通用 skill。
- 版本号现有契约用于实质行为变更；本次语义等价翻译保持版本号，实际文件内容变化由现有 bundle hash 识别。

## 已核实的技术事实

1. `server/internal/service/builtin_role_skills.go` 将完整 `SKILL.md` 内嵌到服务端。
2. `server/internal/handler/agent_template.go` 首次创建副本后按名称复用，源码变化不会升级既有副本。
3. `multica skill update <UUID> --content-file <file>` 使用既有 `PUT /api/skills/<UUID>`。仅传入 `content` 时保留 DB 名称、描述、config、ID、标签、绑定及附属文件。
4. API 不从正文重新同步 metadata；工作区原 frontmatter 必须原样保留，不能将 v2 文件头误盖到 v1 副本。
5. 下一次领取任务重新读取正文并计算 bundle hash，daemon 自然获取新内容；正在执行的模型不会因此即时重读。
6. 更新接口没有原子条件写入。写前重新读取并比较、写后核验可以发现大部分冲突，但不能消除检查和写入之间的竞争窗口。

## 任务 1：备份与工作区定位

**涉及文件：** `.omx/artifacts/role-skill-bodies-zh/` 下的本地备份和核验记录（gitignored）。

1. 保存七份源码的原始字节及 SHA-256。
2. 从当前 checkout 对应的本地服务和已配置登录态定位目标工作区；只读取相关身份和 skill 数据，不输出凭证。
3. 按完整 canonical name 和来源信息筛选七项，导出完整正文、metadata、附属文件，并记录标签和绑定。
4. 比较工作区副本与当前、历史默认正文。当前相同正文复用中文译文；历史正文或定制内容单独保留语义翻译，不能借中文化静默升级业务规则。

## 任务 2：修改七份源码正文

**修改文件：** `server/internal/service/builtin_role_skills/<上述名称>/SKILL.md`。

1. 用简体中文重述原有每一项工作要求，保留章节组织和有效的无问题/无需修改结果。
2. 发布检查保留逐项审批、适用项未验证即失败、独立高风险动作的检查范围。
3. ADR 保留 `observer` 的任务评论交付方式及 `proposed` → 人工 `accepted` 的边界。
4. 审查、需求、文档和测试报告保留证据标准、职责分工与停止条件。
5. 做独立语义审查，逐项检查遗漏、弱化、擅自增强要求和术语漂移；评审只读取文件，不触发生产动作或真实智能体 CLI。

## 任务 3：调整既有验证与维护约定

**修改文件：**
- `server/internal/handler/agent_template_test.go`
- `.trellis/spec/server/builtin-templates.md`

将依赖 `# Code review` 英文标题的物化断言替换为“数据库正文与内嵌模板完全一致”。保留定制副本不覆盖、多个智能体共用同一副本的既有测试。更新规范，说明七份正文使用中文、metadata 保持稳定及既有副本采用显式更新。

不为可逆的措辞改动增加逐字匹配测试；复用现有解析、物化、CLI 文件输入和缓存验证。

## 任务 4：验证

先确认测试工具和隔离数据库可用。DB 测试仅在为本任务创建的独立数据库运行，不能对用户工作区库执行测试初始化或迁移。

```bash
bash scripts/go-test-with-agent-cli-guard.sh -- go -C server test ./cmd/multica -run '^TestRunSkill' -count=1
bash scripts/go-test-with-agent-cli-guard.sh -- go -C server test ./internal/service -run 'TestAgentRoleTemplates|TestRoleSkillTemplates' -count=1
bash scripts/go-test-with-agent-cli-guard.sh -- go -C server test ./internal/handler -run 'TestCreateAgentFromTemplate|TestUpdateSkillSkipsSkillMdFile' -count=1 -v
pnpm --filter @multica/views test skills/lib/skill-presentation.test.ts skills/components/skill-detail-page.test.tsx agents/components/tabs/skills-tab.test.tsx
pnpm typecheck
pnpm lint
git diff --check
```

同时执行适用的 Go 静态检查，检查原 frontmatter、命令和状态值均保留。必须确认 DB 目标测试实际执行，不能把 `Skipping tests` 加退出码 0 当作通过。独立评审覆盖发布审批、ADR 只读交付、审查无发现、需求已清楚、文档无需更新、测试未运行等场景。

## 任务 5：更新现有工作区并核验

源文件和译文审查通过后，再写入工作区：

1. 为每项生成“原 frontmatter + 对应中文正文”的 UTF-8 文件。
2. 写前再次 GET，与备份中的正文、metadata、更新时间比较。出现变化先重新评估，不覆盖并发修改。
3. 使用显式 profile、workspace ID、skill UUID 调用现有 `skill update --content-file`；不发送 `files` 或其他 metadata。
4. 每项写后 GET，逐字核对正文，并比较身份、描述、config、标签、绑定、附属文件。记录成功项及 hash。
5. 七次更新各自事务提交，记录逐项进度；无需清除 daemon 缓存。

## 回滚与交付

- 源码由本次 Git diff 回退。
- 工作区通过同一更新接口写回对应原文；回滚前确认仍是本次写入的内容，避免覆盖后来修改。`updated_at` 正常推进，不承诺恢复原时间戳。
- 不自动重新运行用户智能体，也不为测试读取真实凭证内容、调用生产动作或发送消息。
- 交付技术方案、七份中文源文件、测试结果、工作区更新/备份记录和明确的未验证范围。若目标工作区无法确定或登录态不可用，完成源码和验证，并精确报告仍需补充的信息。
