# 2026-11-23 合并准备基线

## Authorization and observation

用户授权：“行，建一个任务。我还有几个提交在另外个一会话中，你别忘了重新检查或者等待一会再看看”。这要求持续在工作边界复查，不能以首次观察 SHA 为永久候选，也不构成另一会话已经完成的通知。

本任务建立前后已观察：

| Observation | Source HEAD | Meaning |
| --- | --- | --- |
| 初次分支审查 | `648d10fb8ac68aa7c8b793971e507c6311411242` | 两条只读审查轴的固定版本 |
| 上轮结论 | `686e7229331e16b4f1713027619dac1b11420c80` | 已包含侧栏实现与任务归档；当时领先 main 857 个提交 |
| 本次建任务并 fetch 后 | `cb192014b2f27e9bb64112db52073cbb31ce8f04` | 新增侧栏 journal；本地与 origin 功能分支一致，领先 main 858 个提交 |

main 基线仍为 `dd87e0b33c1d6a64d488f7ee9ecd0a65984a68ff`，是当前 source 的祖先。上述数值应在实施和最终报告前刷新。机器可读观察见 `branch-observations.json`。

## Confirmed CI failure

- 入口：`node scripts/check-image-budget.mjs --base origin/main`。
- 实测退出 1，错误 `spawnSync git ENOBUFS`。根因位置 `scripts/check-image-budget.mjs:79–81` 使用 `execFileSync` 默认缓冲收集所有新增/修改路径。
- 固定 `648d10fb8` 的差异列表为 3,002,456 bytes / 23,466 AM 路径；固定 `686e72293` 为 3,004,355 bytes，均超过默认 1,048,576 bytes。`cb192014b` 相同列表仍为 3,004,355 bytes。
- 固定 `648d10fb8` 用 Git blob 大小按脚本同一预算语义核算：121 个新建/增大位图超过 300 KiB，总体积约 49.74 MiB。随后至 `cb192014b` 没有位图变化；最终候选仍应重算。
- CI 在 `.github/workflows/ci.yml:860–879` 启用 image-budget，现有显式格式为 `Oversized image exemption: <reason>`。即使提供说明，当前实现也会先在读取列表时崩溃，因此必须先修读取机制。
- 当前 branch 大量归档并非未跟踪临时文件：初次审查的变更 `.trellis/tasks` blobs 约 1,649.87 MiB。不能通过清理未跟踪目录来消除已经提交的体积。

## Acceptance gaps, not newly reproduced product defects

- 原始报告 `artifacts/qa/core-business-e2e-20261121-kV2VvV/REPORT.md:20–29` 固定 `6360f76ccc907038e77d438c18ec8e5ac596297a`，记录 240/245 通过，五项未闭环。
- 两项 localized squad 出现模板接口/创建页等待问题；S1 outsourcing 的 Knowledge candidates locator 匹配两项；live revoke 与 terminal reopen 在 gate 修正后重新注册 runtime、读取 runs 时出现 409。
- 先前有限搜索没有发现其后闭环记录，且报告引用的部分 `playwright.log` 文件缺失。本任务必须重新核查，不能直接把历史描述说成当前 HEAD 的产品故障。
- `.trellis/tasks/archive/2026-11/11-22-collaboration-notifications-page/research/verification.md:156–162` 明确旧 managed/password/offline 分配仅静态收集，不是 live pass。
- 较早 R3 完整回归的源绑定是 `ddc80d5af18f8e801b925efe6f625ec29a2e1e73`；报告记录 514/514 E2E、14,718 TS、23,691 默认 Go named pass 与 9 skip。见归档 `11-20-full-e2e-regression-20261120/research/r3-final-results.md`。这不是当前候选的通过证据。
- 最新侧栏增量记录 `11-23-sidebar-common-icon-rail/research/acceptance.md` 绑定 `76c61f95d`，含 143 unit、4 原始 Web/实际 Electron 用例与限定静态检查；不覆盖整个累积分支。

## Existing debt and scope limits

只读 Standards 审查检查了 566 个变更 migration 文件和 209 个显式索引创建：未发现新增 FK/cascade、非 CONCURRENTLY 或并发索引多语句违反。未实际运行迁移，不能替代数据库验收。

`server/internal/handler/comment.go` 的 Update/DeleteComment 管理员授权债务在 main 也存在，独立任务为 `.trellis/tasks/11-20-comment-admin-human-authority`；新受限任务 capability 对 PUT/DELETE 有拒绝路径。因此不能称所有机器身份均可越权，也不能称是本分支新增。独立真实模型验收和既存发布边界按其任务处理。

上轮远端查询没有本分支 PR/check runs；用户另一个会话已继续提交并推送，所以此事实同样需要在最终报告前更新。
