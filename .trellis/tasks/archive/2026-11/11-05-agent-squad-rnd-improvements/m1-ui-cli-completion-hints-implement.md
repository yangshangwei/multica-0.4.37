# M1 UI/CLI 自动完成提示实施计划

这是批准后的实施顺序草案。当前会话只完成规划；在明确批准前不得改产品源码、不得运行 `task.py start`。

## P0：冻结行为与失败测试

- [ ] 在 `packages/core/api` 增加 completion-hint 请求/响应的 malformed、未知枚举、布尔值、404/405 和 abort 测试。
- [ ] 在 `packages/core/issues` 为 category 判断、status-only intent、custom done、done→done、缺 revision/category、混合字段和重复触发写失败测试。
- [ ] 在 `packages/views/issues` 为共享 gate 写失败测试：attention 取消不 PUT、确认后只 PUT 一次、hint unknown 不成功、旧端点 bypass、stale revision 不写入、实际 mutation 失败不调用 `onDone`。
- [ ] 为详情顶部按钮、`SubIssueRow`、surface/table/status picker 入口各补一个 canonical behavior test；不要把同一纯函数矩阵复制到每个组件。
- [ ] 在 `server/cmd/multica/cmd_issue_test.go` 用 httptest fake API 写 CLI 失败测试：attention warning、matching warning、refusal、unknown、legacy 404/405、custom done target、status-only update、实际 PUT failure。

## P1：Core API 最小实现

- [ ] 添加 `packages/core/types/api.ts` typed DTO。
- [ ] 添加 `packages/core/api/schemas.ts` 严格响应 schema，并接入 `packages/core/api/client.ts` 的 `parseWithFallback`。
- [ ] 添加纯分类 helper 与 node 环境单测；不引入 React、UI、localStorage 或新依赖。
- [ ] 将 `evaluation_key` 固定为显式 `null` 的调用约束写入类型/测试，禁止调用方偷传 key。

## P2：Shared Web/Desktop gate

- [ ] 实现一个可复用的 views gate/continuation，保证 hint 完成前不触发 optimistic mutation；处理 single-flight、AbortSignal、Retry/Cancel 和 legacy bypass。
- [ ] 接入 `useIssueActions` 与 `useIssueSurfaceActions`，覆盖 detail/status menu/list/board/table/swimlane 的 status-only PUT。
- [ ] 将 `SubIssueRow` 接入共享 gate；确认普通 child status picker 不会绕过提示。
- [ ] 将 detail `onDone` 改为 mutation success 回调；取消、拒绝、unknown 和失败均保留当前页面/草稿。
- [ ] 如采用 modal registry，更新 `packages/core/modals/store.ts`、`packages/views/modals/registry.tsx` 及新 modal 测试；transient continuation 不持久化。
- [ ] 更新 `packages/views/locales/en/issues.json`、`packages/views/locales/zh-Hans/issues.json`，校验双语 key parity。Batch toolbar、drag/move、create/comment-child 不接入。

## P3：CLI 最小实现

- [ ] 在 CLI helper 中读取 issue revision，调用 completion-hint，严格区分 unsupported/attention/refusal/unknown。
- [ ] 接入 `runIssueStatus` 与 status-only `runIssueUpdate`；保持旧 PUT body、`--no-start`、table/json stdout/stderr 约定。
- [ ] custom done 只以服务器 hint 的 `target_category` 为准；不在 CLI 本地硬编码 custom status membership。
- [ ] 更新 `multica-working-on-issues` skill 与 source-map 的命令行为、提示非授权语义和旧 server fallback；保持 `multica-creating-agents` hint API 边界准确。

## P4：验证与审查门

- [ ] 运行 core/views 定向 Vitest；至少覆盖 API schema/client、pure helper、actions、detail、table/surface、modal。
- [ ] 运行 CLI/Go 定向测试、`gofmt`、`go vet`；本轮若只改 CLI，不扩大到无关 server integration。
- [ ] 运行受影响 workspace 的 `pnpm typecheck` 和 `pnpm lint`；既有 warnings 只能如实记录，不能压制。
- [ ] 运行 `git diff --check`，核对没有 completion binding、skip、迁移、真实 provider、生产配置或完成门禁 writer 变更。
- [ ] 独立 review 检查所有清单入口、旧客户端降级、unknown 不成功、custom done、私有 key 不泄漏和实际 mutation 单一提交。
- [ ] 最终把本轮状态写成“UI/CLI 提示增量完成/待验收”，绝不写成完整 M1、v1 gate 或真实供应商验收完成。

## 预计文件所有权

Core API/helper：`packages/core/api/client.ts`、`packages/core/api/schemas.ts`、`packages/core/types/api.ts`、`packages/core/issues/*completion-hint*`及对应测试。

Shared views：`packages/views/issues/actions/*`、`packages/views/issues/surface/use-issue-surface-actions.ts`、`packages/views/issues/components/issue-detail.tsx`、必要的 modal registry/store、locales 和对应测试。不得修改 batch/move/create writer 逻辑来假装覆盖 out-of-scope。

CLI/docs：`server/cmd/multica/cmd_issue.go`、`cmd_issue_test.go`、必要的 CLI helper、`multica-working-on-issues` 与 `multica-creating-agents` builtin skill/source-map 文档及对应静态合同测试。

## 回滚点

- Core API 与 CLI hint consumer 可独立回滚；旧 status/update 请求体不变。
- Views gate 可整体关闭为 legacy bypass，但服务端 v1 writer 与提示 API 不回滚、不改写历史。
- 不删除或修改 evaluation、authorization、terminal、policy、artifact 记录，不新增迁移或依赖。
