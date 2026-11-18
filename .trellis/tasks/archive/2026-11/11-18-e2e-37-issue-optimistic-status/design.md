# #37 读写的是哪个键？

| 环节 | 当前工作树 |
|---|---|
| 路由解析 | `packages/core/issues/canonical-id.ts:60`：UUID 直接使用；编号通过 `issueDetailOptions(wsId, routeId)` 解析 `.id`，再订阅 UUID 详情。UUID → 编号时会给编号键填一次初始值（`:92`），这不是普通详情渲染时的权威状态。 |
| 详情读取 | `packages/views/issues/components/issue-detail-route.tsx:86` 传 `canonicalId`；`issue-detail.tsx:1183` 的外层查询在挂载时强制重取，`:1415` 的普通详情也观察同一键，`:2414` 直接把 `issue.status` 传给选择器。`packages/core/issues/queries.ts:442` 使用 `issueKeys.detail(wsId, id)`，即 `["issues", wsId, "detail", UUID]`。 |
| 普通写入 | `packages/views/issues/actions/use-issue-actions.ts:70` 取 `issue.id`；`packages/core/issues/mutations.ts:127` 使用当前工作区，`:169` 取消多个列表等查询，但未取消目标详情查询。`:176` 取 UUID 详情快照；`cache-coordinator.ts:524` 仅给存在的该键合入乐观值，失败由 `mutations.ts:235` 回滚。有 `completion` 的命令在 `:132`、`:145` 走独立路径。 |

**现有证据强度。** 原 `v2_business/runs/acceptance` 的 #37 trace 中，以 UUID 进入，点状态菜单时已显示 `MUTA-1`；点击后测试扣住的请求体是 `status: in_progress`。同一浏览器的 API 日志先记录 `14:34:26.179 → PUT`，随后 `14:34:26.204 ← 200`（同一 UUID 详情 GET），到 `14:34:31.367` 才记录 503。网络接收与 JS 处理不必同时发生。**源码中的键一致**，且有旧详情读晚于写入派发的线索，但 trace 没记录 Query 数据的每一次变化，尚未证明就是该读覆盖了界面。

**修改决策。** 在同一真实 QueryClient 中依次观察解析出的 UUID、reader/writer 的键和值、旧 GET 完成后的值以及已关闭菜单的详情按钮。若证明旧读覆盖同键，优先在普通 `onMutate` 写前精确取消/隔离**这一工作区这一 UUID**的进行中详情读，保持当前同步 patch、快照回滚、服务端版本调和和受控完成分支。以旧结果不能再发布为准，不强求 HTTP 传输一定中断。若键/QueryClient 实例确实不同，修正实际身份传递；若键值正确而按钮仍旧，只修 Views 的消费路径。不双写编号副本，不新建 UI 状态来冒充服务器状态，不关闭进页权限重验。
