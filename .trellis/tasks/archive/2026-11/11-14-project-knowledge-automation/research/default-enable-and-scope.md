# 默认启用与本次需求边界

核对日期：2026-11-14。来源为当前工作区，分支 `codex/cross-workspace-collaboration`，HEAD `8e824b303`；工作区已有其他未提交改动。以下是规划证据，不是实现或运行验收。

## 已有控制点

| 位置 | 当前行为 | 本任务计划 |
| --- | --- | --- |
| `server/cmd/server/router.go:433` | 只有环境变量字面值 `true` 才设置 `KnowledgeContextEnabled` | 未配置时默认启用，显式关闭继续有效；确定空值/无效值的兼容语义并覆盖测试 |
| `server/cmd/server/task_knowledge_config_test.go:11` | 当前测试确认空值、`false`、`1` 均关闭 | 修改默认值回归，区分未设置、空值、`true`、`false` 和无效值 |
| `docker-compose.selfhost.yml:124` | `${MULTICA_KNOWLEDGE_CONTEXT_ENABLED:-false}` | 同步自托管启动默认值 |
| `.env.example:667` | opt-in 注释，模板为 `false`，注释仍只列 Claude | 默认值与 Claude/Codex 实际支持范围同步 |
| `scripts/check.sh:116`、`scripts/agent-critical-path.mjs:67` | 隔离测试环境显式关闭 | 保留受控测试的显式关闭；新增专项测试显式验证开启路径 |
| `scripts/check.test.sh:125` | 验证 check 环境强制关闭 | 保留该隔离语义，不能批量把所有 false 改成 true |
| `docs/collaboration-knowledge-runtime.md:3` | 启用及回滚文档 | 改写默认行为，保留会话、授权、来源和回滚保护 |
| `docs/offline-upgrade.zh-CN.md:58` | 升级表写默认 false | 更新新部署默认值，并明确保留已有显式配置 |
| `apps/docs/content/docs/environment-variables.zh.mdx:46`、`collaboration-knowledge.zh.mdx:85` | 用户文档写默认关闭 | 同步文档及仓库要求的内嵌文档生成产物 |
| `.trellis/spec/server/collaboration-knowledge-execution.md:36` | 默认 false 的执行契约 | 实施时更新默认值条款；其余运行保护不因默认值变化而移除 |

## 已确认用户意图

用户在本会话明确要求：默认使用知识；符合明确规则时自动采纳；无项目任务忽略；把两个需求合并创建一个 Trellis 任务并进入规划。此前“暂不启动任务”已被本次明确的任务创建授权取代，但没有产品实施授权。

人工采纳与知识注入的旧契约来自已经阶段归档的 `10-30-collaboration-knowledge-promotion`：

- `.trellis/tasks/archive/2026-11/10-30-collaboration-knowledge-promotion/prd.md` 明确人工确认和独立知识集合。
- `.trellis/tasks/11-12-collaboration-real-runtime-acceptance/prd.md:16` 保留真实模型/CLI 验收待办及原默认关闭事实。
- `.trellis/tasks/11-12-agent-squad-followup-roadmap/prd.md:21` 涉及后续知识治理，但不等于自动采纳或本次默认值变更已实现。

新需求替代旧的默认关闭/只能人工采纳的产品方向；不能改写旧验收记录，也不能据此声称已有真实模型验收通过。本任务保持独立，不修改这些并行任务的状态和证据。

## 实施范围建议

- 自动生产/采纳从现有协作任务的正式交付验收链路开始；普通任务的知识消费照旧，普通 `done` 状态不能直接接到该生产触发器。
- 知识默认使用、自动整理和自动采纳属于三个不同的控制维度。默认使用不能暗中启用一个尚未配置或授权的整理资源。
- 未配置与显式 `false` 按用户要求处理；推荐空值也按默认开启，与 Compose 的 `:-` 行为一致。对于非空无效值保留不启用语义，避免误拼写被当成授权，最终配置表须精确写入设计并测试。
- 不改写已部署环境中的显式 `false`，不重启其他会话的服务，不把自动采纳扩散为工作空间范围。
- 不强制扩展 Provider 白名单，不取消 prepare/authorize、来源/受众检查、历史会话保护、人工流程或独立目标 revision。

## 后续验证要点

1. 同一张配置矩阵验证直接 API 启动和 Compose 模板默认值，包含显式关闭及隔离测试环境。
2. 开关开启仍需真实运行适配支持，且只有 eligible 的已采纳版本进入实际模型输入。
3. 开关关闭后保留已有知识会话的限制和停止/清理记录；关闭不等于将私有历史变成普通历史。
4. 本轮只校验任务与文档，没有运行产品测试、模型、迁移或部署。
