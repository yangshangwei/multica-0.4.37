# #37、#38 首批交付与验收

首批两项已按 #37 → #38 完成稳定 RED、最小修复、原整文件回归和最终联验，独立审查无剩余发现。仅接受这两个原编号。首批交付时父任务保留以跟踪其他编号；2026-11-20 用户要求归档父任务，见[归档说明](archive-20261120.md)。

| 编号 | 交付结果 | 详细记录 |
| --- | --- | --- |
| #37 | UUID/可读编号入口在 PUT 待响应时保持 In Progress；更早的 Todo GET 不能覆盖；503 回到 Todo，一次显式重试、刷新后仍为 In Progress | [#37 执行记录](../../11-18-e2e-37-issue-optimistic-status/research/execution-20261119.md) |
| #38 | 在 UUID 上展开历史后，即使历史响应未返回，换到同事项可读编号仍展开；正确迭代链接可打开；从展开 A 站内切 B 后默认折叠，不提前读 B 历史 | [#38 执行记录](../../11-18-e2e-38-iteration-history/research/execution-20261119.md) |

## 修改清单

#37 产品改动只有 `packages/core/issues/mutations.ts`，沿用同步缓存协调器、默认取消、回滚和受控完成授权。新增/更新 `packages/core/issues/mutations.test.tsx` 与 `e2e/issue-optimistic-status.spec.ts`；更新 Core `state-management.md` 及索引。

#38 产品修改 `packages/core/issues/stores/iteration-participation-store.ts`、`issues/stores/index.ts`、`platform/session-cleanup.ts`，以及 Views 的 `navigation/context.tsx`、`navigation/index.ts`、`issues/components/issue-detail-route.tsx`、`issue-detail.tsx`、`iterations/iteration-participation.tsx`。只有当前事项的一条临时展开意图，历史行继续归 Query，非路由详情保留本地状态。

#38 测试修改/新增 `packages/core/issues/stores/iteration-participation-store.test.ts`、`packages/core/platform/session-cleanup.test.ts`、`packages/views/issues/components/issue-detail-route.iteration-history.test.tsx`、`e2e/iteration-history-navigation.spec.ts`、`e2e/iteration-history-navigation-desktop.spec.ts`；更新 Core `iteration-operations.md`、Views `state-management.md` 与索引。没有新增依赖。

## 验证与保留

- 最终生产 Web 四整文件联验 **14/14**：两个原 E2E 和两个冻结新 Web 回归。
- 真实 Electron 渲染器 **6/6**：已有迭代两整文件 5 项，加新增 A→B 1 项。
- #37 Core/Views 相关整文件 **80/80、108/108**；#38 Core/Views 相关整文件 **26/26、53/53**，两端导航 **15/15、38/38**。数字各自按文件记录，不重复相加。
- Core/Views lint、typecheck，两端生产构建和三份新 E2E 严格类型检查均退出 0。Views 的 27 条既有 lint 警告及两条 highlight CSS 优化警告保留。
- Playwright 单 worker、零自动重试，0 skipped/unexpected/flaky；没有放宽超时或断言。原两文件的 SHA 与原版一致，所有输入/结果/trace 与构建指纹保留。

运行号 `20261119-002358-fc3f5ec4`，全新私有库 `e2e_fix_37_38_20261119_002358_fc3f5ec4`，API/Web 54150/54151。最终 Web 构建 `C2Kji8hfWmojMSLxyjsnu`；9014 个源码输入审计通过。总证据：[verification-summary.json](../../../../../../.artifacts/e2e-fix-20261119-002358-fc3f5ec4/joint/verification-summary.json)。只停止本轮自建进程，保留新库、私有源码、构建和报告。

本批未跑原 52 标题的全量 E2E、打包后的 Electron main/安装器或真实智能体。原保留库、KEEP-1、原报告/trace、导出和备份未操作。两条收集错误仍归 PAY-241 S3，不补占位 schema；#51/#52 仍与 `11-17-quick-create-blocked-feedback` 核对，#51 不作为跨区串数据证据。

调试方法和防复发契约见 [debug retrospective](first-batch-debug-retrospective.md)。工作提交与归档后的任务路径记录在各 task.json 和本轮开发者 journal；没有 push。
