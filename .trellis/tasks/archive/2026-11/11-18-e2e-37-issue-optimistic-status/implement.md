# #37 执行顺序（已执行）

以下保留规划时的步骤；实际根因、补充时序和各阶段结果见[执行记录](./research/execution-20261119.md)。

1. 按[共用约束](../../../11-18-e2e-1118-followup/implement.md)核对隔离环境，记录原 `e2e/issue-mutation-branches.spec.ts` 中 `failed status update rolls back the optimistic value before a successful retry` 的当前结果与失败点。不把原 503 当环境故障或自动重试。
2. **先补红测，不改产品。** 在 `packages/core/issues/mutations.test.tsx` 或紧邻的路由/详情集成测试中使用真实 QueryClient、`useCanonicalIssue`、详情 observer、`useUpdateIssue`；只替换 API 边界。用合法 UUID 和可读编号分别进入，允许初始 Todo 成功，启动单独的 UUID 后台详情读并扣住它（不是扣住冷启动读取）。选择状态、扣住首笔 PUT；读取真实详情键/订阅值，先后放行**旧 Todo GET**、**503 PUT**，分别断言状态。加一组无重叠读取的对照。若 Core 测试不红而原按钮仍红，将同一时序接到真实详情/选择器；不能以绿的单独缓存测试代替修前失败证据。保留原浏览器用例的服务端回读、提示、刷新和请求数断言。
3. 在修改前留存稳定 RED 的精确断言、事件次序和退出码；只有其体现原缺陷，才按 `design.md` 选中一处最小修改。拒绝只等写后 refetch、广泛取消其他事项的读取或复用绑定完成分支。对照未返回的 bound completion 不提前变成 done，失败/拒绝不绕授权。
4. 原红测转绿后，用新报告目录依次跑：

   ```sh
   pnpm --filter @multica/core exec vitest run issues/mutations.test.tsx issues/canonical-id.test.tsx issues/cache-coordinator.test.ts issues/completion.test.ts
   pnpm --filter @multica/views exec vitest run issues/components/issue-detail.test.tsx issues/actions/completion-hint-gate.test.tsx modals/issue-completion-authorization.test.tsx
   ```

   按父任务命令先跑原失败标题，再跑完整 `e2e/issue-mutation-branches.spec.ts`（含相邻优先级回滚）。同时跑实际新增回归与受影响包的 `lint`、`typecheck`；若变更涉及共享路由，再联跑 #38。未覆盖的部分照实记录，不将环境失败或红测未复现写成已修。
