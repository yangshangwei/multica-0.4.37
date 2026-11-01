# 可靠上下文交接与召回：当前进度

状态：**实现已交付，确定性与已覆盖平台验收完成，待真实模型验收**。任务保持 `in_progress`。

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

## 下一步顺序

1. **新 runtime/知识真实验收已登记为 RK1，尚不可运行。** [RK1 登记](research/real-model-rk1/README.md)（`d2fe54ab0`）固定源码指纹、CLI 2.1.316、请求模型、网关、场景顺序与停止规则，调用上限为 0。登记中的 RK1-H1 不可评分：启动交接包的 `recording` 只含当前 run 的保留计数，启动时为空，checkpoint 正文不是运行时来源（`collaboration_runtime_handoff.go:251-255`、`collaboration_checkpoints.go:102`）；修正由会话 -87 以 RK1 旁的独立补充文件提交，研究见 [框架研究](research/real-model-runtime-framework-research.md)。缺口：用户尚未分配预算（参考值按修正前场景计算：最小集 14 次，不含 K3 的完整集 29 次）；HEAD 没有授予 handoff/recording/knowledge 的真实 Provider 框架，现有 `TestHostProviderAccountSmoke` 的 claim 不设置 `HandoffVersion`，只证明 CLI/生命周期；H5 需要第二个自控账号。
2. **预算到位后先建框架再运行 RK1-E0。** 框架须先用 fake executable 证明实际调用 handoff/recording/recall/knowledge 接口，独立复核后登记 SHA-256。然后在独占串行窗口按 RK1 顺序运行，首个不完整结果停止后续运行，保留原结果。
3. **原实验系列已续跑一轮，再次停在不完整结果。** 2026-11-01 12:02 起由会话 multica-0-4-37-54 在独占窗口执行（登记 [prerun.json](../10-30-collaboration-fix-closure/research/experiments-2026-11-01-e2a-resume3/prerun.json)，记录在同目录各场景子目录，由该会话提交）。E2a trial 3、E2b 1–3、E3 baseline 1–3 共 7 次框架判定通过，均读取了场景上下文；E3 after trial 1 读取 6 次上下文后在 300.9 秒 CLI 超时，记录为 incomplete，按停止规则后续全部停止。本轮消耗 8 次，**下一入口 E3 after trial 2，剩余上限 14 次**，其后 E4 → E5 → E6。新系列不挪用原槽位，也不把新版本结果填入旧样本。
4. **汇总模型结论后关闭本任务必要验收门。** 父任务继续追踪 Windows 原生、安装/升级组合、多账号与实际额度来源等关联门禁；知识交付的确定性结果已有独立证据，生产开关仍关闭。

## 保留的验收缺口

- 新 runtime 正式 E0、E5/E6 与真实读取/理解/接力证据尚未运行。
- 原 E1–E6 系列尚未完成：E3 after、E4、E5、E6 未完成；有效、无效、已消耗和未运行样本分别记录。E4/E6 需语义裁决，E5 canary 被执行即阻塞审查。
- Windows、安装包组合、多账号/额度及发布验收见父矩阵；本轮 Web/macOS fixture 结果不外推这些范围。

本页由会话 multica-0-4-37-0b 更新；原系列执行与语义判定归会话 -54，本会话未发起任何 Provider 调用。RK1 仍未消耗槽位。确定性通过不等于模型理解，任务暂不归档。
