# 合并后联合故障验收补验

本轮在主工作区 `93afc3cb0eedb4936e5d62b9f4e4841dc99309d5` 上复核启动注入与 checkpoint 同时启用的故障链路。该版本已包含 runtime 与知识实现；没有重复应用启动补丁、改变生产逻辑或启动真实实验。

## 当前结果

| 检查 | 结果 | 证据 |
| --- | --- | --- |
| 联合故障、checkpoint、recording、ledger 与 proof 恢复，`-race` | 22 个顶层、40 个测试项通过；零失败/跳过 | `joint-race.json`、`joint-race.jsonl` |
| 启动注入与 fake smoke authorizer，`agentintegration` | 23 个顶层、80 个测试项通过；零失败/跳过 | `tagged-launch.json`、`tagged-launch.jsonl` |
| Tagged daemon vet | 退出 0 | `tagged-vet.json` |
| 补强终止事实后的联合回归，`-race` | 3 个顶层、6 个测试项通过；零失败/跳过 | `strengthened-race.json`、`strengthened-race.jsonl` |
| 补强后的 daemon vet | 退出 0 | `strengthened-vet.json` |

这些 suite 的覆盖有重叠，计数不相加。前三项使用相同 HEAD 和 server 源码指纹，检查前后文件集合/哈希保持一致。最后两项记录同一 HEAD 上仅加强测试断言的 dirty diff 和相关源码哈希；检查期间也没有源码变动。准确 argv、时间、退出码和具名测试均在各 JSON 中；`source.json` 与 `manifest.json` 固定证据来源。

## 覆盖与本轮修改

- Snapshot 拒绝发生在 sender 创建前；binding 拒绝发生在进程启动前；fallback 来源漂移发生在第一次 fake 进程结束后。已有测试检查原 Stop、termination/proof ACK 与实际启动次数。
- 原 checkpoint 已保存但 ACK 丢失时，以相同 identity、attempt、key、offset 和 payload 重放；确认后清除待发送正文。
- 上传持续阻塞期间，原 Stop 与 proof ACK 在 fixture 释放上传前完成。
- 重开磁盘 ledger 后，新 Runner 只恢复原 checkpoint/seal；不再 Claim、取 snapshot 或启动 Provider，持久化 ACK 后不保留已确认正文。
- 本轮仅在 `host_handoff_checkpoint_test.go` 补强原 permit identity/attempt/nonce、`not_started` 与 `managed_launch_ended` 的区别，以及 proof 内终止事实的精确一致性；同步 startup spec。复用现有 fixture，没有新增测试框架或依赖。

独立检查代理本轮因路由 503/限流未成功执行；不把这些请求记为通过。原产品独立审查及最终门禁仍以父流程已归档证据为准。

## 交给父流程的状态

Runtime 产品及原全栈验收已由 `0e1ee8e1e` / `2b861347e` 交付，补充矩阵见 `0bbc6a77a` / `84c4c2b3d`。原生产 Web 与原生 macOS Electron 六项验收全部通过，零自动重试，见 `../../../verification-integration-2026-11-01.md` 和 `../../integration-verification/final-browser/summary-final.json`；这些是各自版本的已执行证据，本轮没有再运行浏览器或完整 TS/DB 门禁。

知识工程交付已合入主工作区，完整验收和开放门见相邻知识任务的 `verification-integration-2026-11-01.md`。本补验只确认后续合并保留了原联合故障链路。

本轮没有真实 Provider/account、DB、Windows、安装包或额度操作。原实验独立流程已保存 E2a trial 2 的上游 502 无效结果；下一入口为 **E2a trial 3，剩余上限 22 次**。继续保持停止规则，不重跑旧槽位，不自动探测或重试。新 runtime 的模型验收须另行登记版本与预算，父任务保持 `in_progress`。
