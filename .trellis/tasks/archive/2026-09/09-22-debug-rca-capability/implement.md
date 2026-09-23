# 执行计划:调试/RCA 能力

## 有序清单

1. **定展示元数据**：采用白名单中的 `microscope` 图标和已落地的 `quality` 分类。
2. **写技能** `server/internal/service/builtin_role_skills/multica-debugging/SKILL.md`(简体中文,frontmatter + R1 章节 + R2 边界 + R3 证据契约)。
3. **注册技能**:`builtin_role_skills.go` 的 `builtinRoleSkillVersions` 加 `"multica-debugging": 1`。
4. **写角色提示词** `server/internal/service/builtin_agent_templates/diagnostician/INSTRUCTIONS.md`(含 `## 职责` `## 输入` `## 交付格式` `## 不负责的事项` `## 完成标准` `## 需要人工介入的情况`;无 `{{`)。
5. **加 roster 条目**:`builtin_agent_templates_roster.go` 在 qa-engineer 之后、code-reviewer 之前插入 diagnostician(字段见 design.md);选不与现有 9 个重复的 emoji。
6. **同步计数和清单测试**：`ListedRosterIsTen` 及顺序、`DefaultRoleSkills`、`AutonomyDefaults`、`RoleSkillTemplates_RosterIsNine`、`PresentationDefaults`；同步 handler 列表断言。
7. **新增** `TestDiagnostician_EvidenceContract`(仿 `TestProgressReporter_EvidenceAndCloseoutContract`)。
8. **构建 + 测试**(下方命令),迭代到全绿。
9. **独立文本语义评审**角色/skill（AC7）：边界、证据契约、停止条件及 house style 一致性，记录于 `text-review.md`。

## 验证命令

```bash
cd server && go build ./...
cd server && go test ./internal/service/... -run 'RoleSkill|AgentRoleTemplate|Diagnostician' -count=1
cd server && go test ./internal/service/... ./internal/handler/... -count=1
gofmt -l server/internal/service   # 期望无输出
```

内置模板/skill 测试是纯内容测试；完整 service/handler 测试必须设置指向已迁移隔离测试库的 `DATABASE_URL`。handler 的 `TestMain` 在数据库不可达时可能以退出码 0 提前返回，必须通过 `go test -json` 检查实际执行、通过和跳过数量。

完整验收额外启用 `-race -count=1`、独立 `REDIS_TEST_URL`，并通过 `scripts/go-test-with-agent-cli-guard.sh` 阻止意外执行本机真实 agent CLI。数据库、Redis 均由本次验证创建，结束后清理。最终命令和结果见 `verification.md`。

## 风险文件 / 回滚点

- `builtin_agent_templates_roster.go` 与 5 个计数/清单测试是「一处漏改就红」的耦合面——改完立刻跑第二条(`-run` 过滤)命令快速反馈。
- 纯增量、无迁移；工作提交 `e348b009a` 已按 Lore 协议记录增加 RCA 能力的原因和验证证据。代码回滚不删除已物化的工作区角色/skill 副本。

## 执行状态（2026-09-23）

- 实现步骤 1–7 已完成；角色数 9→10、角色 skill 数 8→9，handler 两处列表断言已同步。
- `implement.jsonl` / `check.jsonl` 均已具备真实 spec 条目，任务状态为 `completed` 并已归档。
- 分类改造依赖已提交并归档，步骤 8–9 已完成：完整数据库与 Redis race 测试、独立文本评审均通过，详见 `verification.md`。
- `drafts/` 保留 9 月 22 日规划快照，不作为当前契约；最终内容以 `server/internal/service/` 下的角色和 skill 文件为准。
- 验证、文本评审、任务记录同步、Lore 工作提交与归档均已完成；发布不在本任务范围内。
