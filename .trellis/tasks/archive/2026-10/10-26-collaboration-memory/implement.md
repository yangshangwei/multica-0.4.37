# 实施计划：协作单共享的 AI 工作上下文

> 2026-10-26，planning；2026-10-27 继续执行。依据 [PRD](prd.md) 与 [技术设计](design.md)。M1–M6 已实现并完成主要验证，Playwright 人工与 runtime 验收已通过。

## 实施原则

- 服务端先行，daemon 最后发布；每一步都能单独回退，不依赖后续步骤才能保持正确。
- 只在协作层扩展，不改普通任务路径，不重构 host_runner 或 daemon.go。
- 新增的跨端字段一律可选，旧客户端和旧 daemon 不因此失败。
- 不新增依赖。
- 普通本机执行路径的文件可能有其他会话在改，开始每一步前先确认工作树状态并读最新代码，不按本文的行号盲改。

## 顺序与验收归属

### M1. 存储与配置

- [x] 迁移：`collaboration_memory_version` 表；`collaboration_run` 增加 `memory_mode`、`memory_base_version`；三个并发唯一索引各占一个文件，并登记到 `concurrentIndexCleanups`；down 文件拒绝回滚。编号在实施时顺延。
- [x] sqlc 查询：当前版本、按版本取、倒序分页、插入、清除、run 写入幂等查找、run 最大写入版本；`make sqlc`。
- [x] `MULTICA_COLLABORATION_MEMORY_MAX_BYTES` 解析，越界时启动失败；注入 service、handler 与 `AdminReadConfig`。
- 主验收：CA05 的配置部分。

### M2. 人的接口与服务

- [x] 服务方法：读当前、列版本、取版本、更新、恢复、清除，写入流程按 design"写入语义"；用 `mutateCollaborationContent` 风格的独立 revision 与 `collaboration_operation` 幂等，kind 为 `memory_update`/`memory_restore`/`memory_purge`，并加入原键恢复的允许列表。
- [x] 路由与处理器；写接口的请求体读取上限按配置计算。
- [x] `memory_changed` 事件：`deliverNext` 跳过工作台广播；活动流及其他 `collaboration_event` 读取方排除该类型（全量检索读取方）。
- [x] R38 代管清除：`purge_memory` 动作、双账本、审计；管理后台用的版本元数据接口。
- 主验收：CA06、CA07、CA08、CA09、CA10。

### M3. 运行时服务端

- [x] claim 请求 `memory_version`；首次 claim 事务内写 `memory_mode`。
- [x] runtime 路由 `memory/base`、`GET memory`、`PUT memory`：门禁、`memory_unsupported`、`(run_id, request_id)` 幂等、单独的请求体上限；AI 写入经 `redact.Text` 后再判断上限、计算幂等哈希并存储（C30）。
- [x] run 记录投影增加可选 `memory` 字段。
- 主验收：CA03、CA04、CA11（服务端部分）、CA17。

### M4. daemon、CLI 与文档

- [x] host claim 填 `memory_version=1`；许可后取基线并放入 `hostPrompt`；拉取失败按 `not_started` 收尾；fresh-session 重试沿用基线。
- [x] 修改 `hostPrompt` 中的 "shared context" 一句；`hostBrief` 加固定说明。
- [x] `multica collaboration memory show/update`：根命令、白名单、参数、stdin/文件输入、`guardLocalPathLinks`、错误提示。
- [x] 更新 `multica-working-on-issues` 的 `SKILL.md`、`references/collaboration-workflow.md`、`references/working-on-issues-source-map.md`，以及 `.trellis/spec/server/collaboration-runtime-cli.md`、`collaboration-host-execution.md`。
- 主验收：CA01、CA02（运行时部分）、CA15。

### M5. Web/Desktop 与管理后台

- [x] core：`memory-schema.ts`、`memory-queries.ts`、`memory-actions.ts` 及 `ApiClient` 方法；run 投影的可选 `memory`；node 环境测试，含畸形响应。
- [x] views：`memory-panel.tsx` 挂进 `CollaborationDetail`；冲突、超限、清除确认；`account.ts` 的 `memory`/`memory-v{n}` 深链；`run-records.tsx` 标记行；en/zh-Hans 文案与现有 parity 测试。
- [x] 管理后台：设置页的上限只读行；协作单详情的代管清除入口。
- 主验收：CA06–CA08、CA11、CA13、CA14。

### M6. 文档、端到端与收尾

- [x] 环境变量文档：`.env.example`、`docker-compose.selfhost.yml`、`apps/docs/content/docs/environment-variables{,.zh}.mdx`、`pnpm generate:docs-bundle`、`docs/offline-upgrade.zh-CN.md`（写明先升级服务端再升级 daemon）。
- [x] 术语表：`apps/docs/content/docs/developers/conventions{,.zh}.mdx` 增加"协作上下文 / Shared context"。
- [x] Playwright：换机接力、共享上下文读取、版本、幂等重放、冲突、详情面板、恢复、清除和终态只读场景。
- [x] 实现阶段已更新 `.trellis/spec/server/` 的协作 runtime/host 规范。
- [x] 确认改动不涉及 `apps/mobile`（CA12）。
- 主验收：CA01、CA12、CA13、CA16，以及全部 CA 回归。

## 检查命令

从仓库根目录运行。默认测试只用测试创建的假智能体可执行文件；DB 测试用克隆的迁移模板库，否则 `internal/handler` 的 `ok` 不代表测试真正执行过。

~~~sh
git diff --check
python3 .trellis/scripts/task.py validate .trellis/tasks/10-26-collaboration-memory
make sqlc
pnpm typecheck
pnpm lint
pnpm test
make test
make check
~~~

Go 迭代检查（在 `server/` 下）：`./internal/service`、`./internal/handler`、`./cmd/server`、`./cmd/multica`、`./internal/cli`、`./internal/daemon/collaborationrun`、`./pkg/protocol`、`./cmd/migrate`。

## 风险与回退

- 迁移号可能被其他会话占用：实施时再取号，提交前再核对一次。
- `host_runner.go` 由 10-24 持续修改：M4 开始前先同步 10-24 的最新状态，改动尽量集中在取基线和两段文案上。
- `collaboration_event` 读取方遗漏会让活动流出现空行：M2 用测试固定"写入上下文后活动流条目数不变"。
- 回退：daemon 退回旧版本只会让新 run 标 `unsupported`；服务端迁移不可回滚，与现有协作迁移一致。
