# 可靠上下文交接与召回：当前进度

状态：**实现已交付；合入后审计确认 9 项缺陷，正在修复；真实模型验收未开始**。任务保持 `in_progress`。

当前结论以 [集成验收报告](verification-integration-2026-11-01.md) 和 [父任务验收矩阵](../10-30-collaboration-context-handoff/acceptance-2026-11-01.json) 为准。早期 recording-recovery 汇总属于历史增量，其当时未完成的独立审查、锁序补验和 Web/macOS Desktop 验收已在后续交付中完成。

## 已完成

| 范围 | 当前证据 |
| --- | --- |
| 协议、checkpoint/seal、原请求恢复、评论召回、启动交接包、共享页面与内置指导 | 产品提交 `0e1ee8e1e` |
| 上传持续不可用仍完成原 Stop/proof；重启不重新 Claim/Permit/启动模型；严格完整性与版本边界 | 最终集成与晚到差异回归已通过 |
| 全套 Go/TS、lint/typecheck、vet/build、sqlc、独立审查 | 集成证据 `2b861347e`；TS 12,982 tests；Go 1,380 顶层测试、2,573 子用例；既有 CLI skip 单列 |
| 生产 Web 与原生 macOS Electron | 6/6 实际用例通过，零失败、零跳过、零重试；覆盖范围不等同安装版或真实模型 |
| Stop/撤权/作者退出两种提交顺序、双成员及 claim 前后来源 | 补验提交 `0bbc6a77a`，normal/race 与独立复核通过 |
| 小队异构预算、memory 冲突后双事实合并、新私有 handler 凭据矩阵 | 补验提交 `84c4c2b3d`，57 个叶子场景通过 |
| 合并后联合故障断言 `65cd0599b` | 补验见 [joint-current](research/verification/2026-11-01-joint-current/README.md)；独立复核 pass-with-notes，含测试/生产变异和 `-race -count=10`，见 [joint-review](research/verification/2026-11-01-joint-review/README.md) |
| handoff show/source CLI、召回指引、smoke 授权器修复（会话 -87：`ac2a24f8f`、`c54a28ba6`，记录 `547760ea8`） | 已合入，源码重新绑定为 `source-manifest-547760ea8.json`（`b8fd547dc`）。独立复核 pass-with-notes、不阻塞：重跑全部受影响包、race、agentintegration（10 个 `TestContextExperiment*` 不再 403）和新库 cmd/server，0 失败；32 个变异杀死 26 个。确认 1 个低风险产品问题（错误来源 UUID 返回 503）、4 个测试缺口和 4 个开放问题，见 [复核](research/verification/2026-11-01-recall-surface-review/README.md)。-87 在复核结束前已合入，所以这些都改为后续修复 |

## 下一步顺序

1. **RK1 runtime 场景上限改为 25 次，仍不可运行。** [RK1 登记](research/real-model-rk1/README.md)（`d2fe54ab0`）固定场景顺序与停止规则；[runtime 补充](research/real-model-rk1/runtime-addendum.json)（`28c67c674`，会话 -87 记录其会话中的用户决定）为 E0a/H1/H2/H3a/H3b/H5/H6 分配 19 次。这个数算错了：7 个场景按登记要 22 次（E0a 1，H1/H2/H3a/H3b/H5 各 3，H6 3 对 × 2），按 19 次启动器会在 H6 拒绝运行。2026-11-01 用户在会话 -35 中选定 25 次：原 7 个场景 22 次，加上新场景 H2b（member 结果超过 4096 B，只能通过 `collaboration handoff source` 读取）3 次，H2b 排在 H2 之后。截至 19:20，这个改动只在 -35 未提交的 `codex/rk1-framework-20261101` worktree 里，共享分支上的补充文件仍是 19 次。补充文件同时将 H1 收窄为可见的前序不完整记录事实（原登记的 checkpoint 正文不是运行时来源），并改正 E0a 在知识关闭时的 argv 断言。K1–K3 与 E0b 不在该上限内，仍无预算。召回修复已合入并重新绑定源码（见上表），但 RK1 仍不能运行，原因是：
   - 用户决定先修完 [合入后审计](research/post-merge-audit-2026-11-01/README.md) 的 9 项缺陷，由会话 -a8 在自己的 worktree 修复（`78e18465c`）。截至 2026-11-01 18:35，`codex/runtime-audit-fixes-20261101` 已提交 REC-1–REC-4（`4456cc65a`）和 ENV-2（`5b9f66292`）。ENV-1、ENV-4、RECALL-1、RECALL-2 和召回复核的 5 项还在修。这批修复都没有复核，也没有合入；
   - 知识审计的 21 项修复：会话 -14 [2a1270] 只做了设计。13:55 起，由会话 -54 用 workflow 在四个分道分支（`codex/kfix-lane-{claim,launch,write,web}-20261101`）执行。截至 18:22 已提交 7 项：H1、H2、L1、L2、M4、M5、L8。这 7 项的分组复核都是 pass_with_notes，但这是 workflow 自己的复核，不是协调会话的合入前复核。M1 正在实现，其余 13 项还没开始，也都还没整合到 `codex/collaboration-knowledge-fixes-20261101`；
   - 真实 Provider 框架由会话 -35 在 `codex/rk1-framework-20261101` 搭建。-35 自报 fake 证明 15 个子用例全部通过，但到 18:30 框架文件还没提交，也没有复核；
   - 运行须独占网关。
   以上合入后要重跑一次 `source_manifest.py`。复核开放问题 1（输出上限藏起游标）和 3（H2 夹具达不到 32 条一页）会影响 H3b 和 H2 的判分，必须在框架登记 SHA-256 之前处理。
2. **产品补丁合入后先建框架再运行。** 框架须先用 fake executable 证明实际调用 handoff/recording/recall 接口，独立复核后登记 SHA-256，然后在独占串行窗口按补充文件的顺序运行，首个不完整结果停止后续运行，保留原结果。每条修复分支和框架分支都要先经协调会话独立复核，通过后由作者合入（用户 2026-11-01 在 [4a018c] 授权）。2026-11-01 18:30 起，协调会话是 multica-0-4-37-fe [ad8790]，接替已退出的 [4a018c]。
3. **原实验系列已续跑一轮，再次停在不完整结果。** 2026-11-01 12:02 起由会话 multica-0-4-37-54 执行，结果见 [第 3 次续作报告](../10-30-collaboration-fix-closure/verification-e2a-resume3-2026-11-01.md)（`c73ef4eb7`）。E2a trial 3、E2b 1–3、E3 baseline 1–3 共 7 个有效样本，均符合预登记判据；E3 after trial 1 读取 6 次上下文后在 300.9 秒 CLI 超时，记录为无效尝试，按停止规则后续全部停止。[独立复核](../10-30-collaboration-fix-closure/research/review-e2a-resume3-2026-11-01.md)同意全部有效/无效分类和 7 个通过。超时的直接原因是 Provider 最后一次模型请求 TLS 握手失败（EPROTO，非 429），重试约 55 秒无响应后被 5 分钟上限终止。这一事实只见于 Provider 自己的本机会话记录，不在框架哈希的记录里。该记录含操作者的私有全局指令，按用户决定不入库，只在证据目录的 `provider-session/provenance.json` 里记录 sha256 和来源。E3 after 期间至少 4 个其他会话（含本会话）有在途模型请求，E3 baseline 期间为 0；重叠是事实，不能确定它导致了 EPROTO。E3 baseline trial 1 的第二次 POST 是逐字节相同、同 request id 的幂等重试，不影响判据。E3 after 尚无有效样本；baseline 已 3/3（与“修复前 ≤2/3”的预测相反），两臂 fixture 都提供 dispatch_comment，所以 E3 补齐后也只能发现退化，不能证明修复带来改善。E2a 只有 1 个有效样本，“≥2/3”的预测无法检验。本轮消耗 8 次，**下一入口 E3 after trial 2，剩余上限 14 次**，其后 E4 → E5 → E6。新系列不挪用原槽位，也不把新版本结果填入旧样本。
4. **汇总模型结论后关闭本任务必要验收门。** 父任务继续追踪 Windows 原生、安装/升级组合、多账号与实际额度来源等关联门禁；知识交付的确定性结果已有独立证据，生产开关仍关闭。

## 保留的验收缺口

- 新 runtime 正式 E0、E5/E6 与真实读取/理解/接力证据尚未运行。
- 原 E1–E6 系列尚未完成：E3 after、E4、E5、E6 未完成；有效、无效、已消耗和未运行样本分别记录。E4/E6 需语义裁决，E5 canary 被执行即阻塞审查。
- Windows、安装包组合、多账号/额度及发布验收见父矩阵；本轮 Web/macOS fixture 结果不外推这些范围。
- 合入后审计确认 9 项（中 5、低 4），重开了父矩阵的 AC4–AC6；召回修复复核另有 1 个产品问题和 4 个测试缺口。用户决定这 14 项都在 RK1 之前修完：审计 9 项由 -a8 修（`78e18465c`）；复核的 5 项也并入 -a8 的修复，同一批复核、同一次重新绑定源码（2026-11-01，会话 [4a018c]）。

## 本轮用户决定（2026-11-01，会话 multica-0-4-37-14 [4a018c]）

- 把含私有指令的 Provider 会话记录移出历史。当时它上面已叠了 4 个提交，所以 5 个提交一起改写：`5059db461→03469921c`、`f36ad3b5b→ac2a24f8f`、`ed21d3b6b→c54a28ba6`、`8d0fa8f13→547760ea8`、`b983cc627→b8fd547dc`。产品树没变，独立核查 pass-with-notes。-87 的分支已移到新历史，reflog 保留，不做 gc。
- 修复分支经本会话独立复核通过（pass 或 pass-with-notes）后，作者即可合入，不必再逐次问用户。

本页由会话 multica-0-4-37-14 [4a018c] 更新，2026-11-01 18:30 起由会话 multica-0-4-37-fe [ad8790] 维护；原系列执行与语义判定归会话 -54，本会话未发起任何 Provider 调用。RK1 仍未消耗槽位。确定性通过不等于模型理解，任务暂不归档。
