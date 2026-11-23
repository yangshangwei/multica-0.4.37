# #38 展开状态由谁持有？

原 `v2_collab/runs/acceptance` 的 trace：`call@674` 的真实点击发生在 UUID 地址，点击后 `<details open>` 显示 `Loading…`；等链接的 `call@676` 结束时已是同一事项 `P1AF-1`，面板关闭。`GET …/iterations?issue_id=<UUID>` 被取消，并非服务端返回“无历史”。指向编号的页面请求在用户点击**之前已经发起**，只在之后提交了地址替换；仅在调用 `replace()` 的一刻记下展开值，会漏掉这次点击。

`packages/views/issues/components/issue-detail-route.tsx:40` 调 `navigation.replace` 保留规范地址／片段；Web 一般路径切换经 `apps/web/platform/navigation.tsx:99` 走 Next `router.replace`。`packages/core/issues/canonical-id.ts:90` 会填编号别名缓存，切换时未必再发 `/api/issues/{编号}`。详情 `issue-detail.tsx:2427` **已有** `workspace:issueUUID` 组件键。`packages/views/iterations/iteration-participation.tsx:20` 的本地 `open` 初始为 false，`:29` 原生 `<details>` 没有受控 `open`，`onToggle` 决定是否启用按事项查询。这个键能区分不同事项，却不能跨父路由卸载保存状态。

**候选修法，以红测确定边界。** 分别观察页面段、详情、参与历史的挂载/卸载，以及原生 `open` 和查询启用值。若是已解析的同一事项在替换时走了不必要的骨架/子树重建，优先在不绕过 401/403/404 处理的前提下保留该子树。若动态路由段按设计会重挂载，只把当前详情的展开意图交给能跨这次替换的最近稳定持有者，以工作区 + 真实 UUID 和当前导航表面划定生命周期；在替换提交前也接收点击。让原生 `open` 与查询开关一致，异事项或离开时不沿用，不缓存所有访问过的事项。历史数据继续归 React Query，保留惰性读取、分页、重取和请求取消。不要用本地存储、全局布尔值、延迟替换，或只给 `<details>` 加 `open={open}` 来掩盖上层重挂载。
