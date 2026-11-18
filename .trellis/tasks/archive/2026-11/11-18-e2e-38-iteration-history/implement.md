# #38 执行顺序（已执行）

以下保留规划时的步骤；真实路由重挂载、最小修复和两端结果见[执行记录](./research/execution-20261119.md)。

1. 在[新隔离环境](../../../11-18-e2e-1118-followup/implement.md)按原失败标题运行 `e2e/iterations-i1-history.spec.ts`，新旧报告独立。保留原例先展开再点链接的顺序及完整后续断言。
2. **先补能失败的时序回归，不改产品。** 新浏览器上下文、唯一事项 A；在导航前设置两个独立的放行点。放过首次 UUID 详情和能力查询，使历史可点；只扣住指向 A **可读编号页面**的同源导航响应（原 trace 为带 `rsc: 1` 的 GET），以及真实的 `GET …/iterations?issue_id=A.uuid`。不要扣住初始详情或依赖可能不发出的编号 API 查询。若生产 Next 已有预取使页面请求不可截获，在路由测试中让 NavigationAdapter 暂缓**提交**替换，维持同一 QueryClient，按真实页面段的边界重挂载；真实浏览器仍须证实点击时在 UUID 地址，不能以 mock 的 replace 调用代验收。
3. 在实际 `summary` 点击事件记录 pathname，断言是 A 的 UUID，面板为 `open` 且 A 的历史读已开始。**先放行页面，仍扣住历史**：断言地址和标题对应 A，当前页面上的 `<details>` 仍展开；再放行历史，确认正确链接可见且能打开目标迭代。若原读取取消，允许新订阅读取同一 A；不要把“请求从未取消”当产品要求。用 `finally` 放行并收妥路由处理器，不增加生产延迟或固定 sleep。
4. 另做负对照：从**仍展开**的 A 经现有站内链接切普通事项 B（例如造一个父事项作为链接目标）；不靠 `page.goto` 或整页刷新。断言同一页面会话内到达 B 的标题/编号，B 可用的历史面板关闭，B 在用户未展开时不发参与历史读。不要从已打开的迭代页面返回 B 冒充 A→B 的对照。
5. 留下修前同一失败点的证据后才按 `design.md` 修一处。修后原样跑红测、原失败标题及完整文件（本文件原只有这个标题）。按父任务命令绑定新的报告目录；跑相关组件/路由回归：

   ```sh
   pnpm --filter @multica/views exec vitest run iterations/iteration-details.test.tsx iterations/iteration-assignment.test.tsx iterations/iteration-history.test.tsx issues/components/issue-detail-route.test.tsx
   pnpm --filter @multica/core exec vitest run issues/canonical-id.test.tsx
   ```

   如改导航接线，加跑对应 Web/Desktop 的 `platform/navigation.test.tsx` 及实际受影响客户端；保留单次 replace、哈希锚点、可读直达、不可访问事项拒绝读取的现有覆盖。跑受影响包 `lint`、`typecheck` 与 #37 原 E2E 联验，记录各自结果。
