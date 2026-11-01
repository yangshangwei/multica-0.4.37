# 原系列第 3 次续作（E2a trial 3 起）

本次从 E2a trial 3 开始串行运行，到 E3 after trial 1 遇到第一个不完整结果后停止。共 8 次 Provider 执行，7 个有效样本均符合预登记判据，1 次无效尝试。

| 项目 | 证据与结论 |
| --- | --- |
| 固定来源 | baseline `c58f79874cfe2e6318e3dce067d7b6bac24e943f`、after `835e7b046c6841e0945a59326bc4ddcbae4a0df7`；实验框架 `f6f62e55888fa2d3e733ff12dec6054e07f877828363ad0d503ada6db07aa0bd` |
| Provider | Claude CLI 2.1.316；请求 `claude-opus-5-5`，thinking/tier 为 provider_default；实际模型未观察到 |
| E2a | trial 3 有效，答 `2026-11-14`，通过。E2a 三个槽位全部用完：1 个有效且通过，2 个无效 |
| E2b | trial 1–3 全部有效，均答 `2026-11-14`，通过 |
| E3 baseline | trial 1–3 全部有效，均只发 `BETA`，通过。trial 1 有同 request id 的第二次 POST，留待复核 |
| E3 after | trial 1 无效：memory 已写出后，CLI 遇到 EPROTO TLS 握手失败（重试 1/10），约 55 秒无响应，到 5 分钟上限被终止；不是 429。错误只出现在 Provider 本机会话记录里；记录的 sha256 和来源已入库，正文不入库。trial 2、3 未运行 |
| 执行收尾 | 8 个 run 的 HTTP 请求/响应全部成对，每个 run 一个 lifecycle 事件并达到 `proof_acknowledged` |
| 剩余 | 总上限 **14 次**；下一入口 **E3 after trial 2**，随后 E4 → E5 → E6 |

E3 是 baseline 与 after 的对照，after 还没有有效样本，所以本场景尚无对照结论。baseline 已是 3/3，与“修复前 ≤2/3”的预测相反；两臂 fixture 都提供 `dispatch_comment`，补齐 after 只能发现退化。E2a 只有 1 个有效样本，“≥2/3”的预测无法检验。

E3 after 启动时窗口不独占：启动前的检查显示另一会话 29 秒前有写入，启动器没有据此暂停；该 trial 期间至少 4 个其他会话的请求在途，baseline 期间为 0。这支持环境类归因，不能确定原因。语义判定由运行会话写出，已由会话 multica-0-4-37-0b 独立复核（[复核](research/review-e2a-resume3-2026-11-01.md)，`2fc5c962c`），同意全部分类和 7 个通过。

运行前登记、启动器、原始记录、判定与完整性 hash 见 [本轮证据](research/experiments-2026-11-01-e2a-resume3/summary.md)。新 runtime `0e1ee8e1e` 和知识版本的真实验收另行登记（RK1），不继承本系列结论。
