# 首轮协作修复交付闭环实施计划

## 2026-11-01 当前续作清单

- [x] 在编码和审查结束后的独立窗口核对两条原产品源码及登记框架，运行 E2a trial 2；Provider 约 196 秒后返回上游 502，未获得行为答案。
- [x] 保存原始记录、命令、1,964 对 HTTP 及 permit/termination/proof；独立记录为环境无效尝试，未改原始 verdict。
- [x] 遵守停止规则，trial 3 与所有后续场景未运行；完整结果见 `verification-e2a-trial2-2026-11-01.md`。
- [ ] 上游恢复后的独立窗口从 **E2a trial 3** 继续，其后 E2b → E3 baseline/after → E4 → E5 → E6；剩余上限 **22 次 Provider 执行**，不补旧槽位。

以下为历史续作记录；下一入口和预算以本节为准。

## 2026-10-31 续作清单

### 第三次续作：从 E2a 接续

- [x] 核对原始证据 37 个 hash、两个隔离源码目录及框架 hash；继续使用已复核的 `9673ab3bf` 测试框架，E1 已消耗的三个槽位不重跑。
- [x] 从 E2a trial 1 开始实际运行；该轮在上游 429 后五分钟超时，已按第一处不完整结果停止后续。没有扩大调用次数或重跑旧槽位。
- [x] 保留每轮原始证据与独立于原始 verdict 的诊断，更新已消耗槽位、有效/无效样本及下一次入口。本轮运行前登记位于 `research/experiments-e2-resume-2026-10-31/prerun.json`，结果见 `verification-e2-resume-2026-10-31.md`。
- [x] trial 2 已于 2026-11-01 接续并因上游 502 停止；下一入口和预算见本页当前续作清单。

### 第二次续作：保留已消耗的 trial

- E0 已通过；E1 trial 1 的 429/503 无效尝试继续占据原编号，不覆盖、不补跑成成功样本。剩余预算为 E1 trial 2、3 及 E2–E6，共最多 26 次 Provider 调用。
- [x] 给 tagged 测试框架增加可选 `MULTICA_CONTEXT_EXPERIMENT_START_TRIAL`（未设置默认 1，显式空值拒绝，范围 1–3）；总轮数仍固定为 3。参数在 Provider 探测前校验，续跑保留原 trial/hop 编号。
- [x] 用假 Provider 验证剩余调用数、原编号、异常停止及接力隔离；独立复审后将同一框架用于两条源码 arm。最终框架 SHA-256 为 `f6f62e55888fa2d3e733ff12dec6054e07f877828363ad0d503ada6db07aa0bd`。
- [x] 记录新的框架 hash 与续作登记，从 E1 trial 2 开始串行执行。第 2 轮事实回答正确但未回写，第 3 轮 6 次 429 后超时，后续停止；不扩大超时，不改场景/模型/评分。
- [x] 按实际记录更新有效样本、无效尝试、未运行项及后续入口；见 `verification-resume-2026-10-31.md`。E1 为 1 个有效样本、2 次无效尝试；下一次从 E2a trial 1 开始，剩余 24 次调用。任务保持 `in_progress`。

### 首次续作记录（历史）

基线 `4c833e737` 已合入下述原计划第 1–5 项及第 6 项的 Go/TS 检查；历史证据保留，不将旧检查冒充新 HEAD 的完整验收。独立实施目录为 `/Volumes/artisan/code/2026/mc-context-resume-20261031`。

- [x] 在任务独占数据库补 745 → 746 → down → up → reapply，保存命令、HEAD、schema 断言与退出码。
- [x] 用生产 Web 和 Electron 接收方页面补交接验收；环境故障按实际边界记录。
- [x] 承接 `32b741d0a` 的实验测试框架，补完整 prompt/HTTP/生命周期证据和假 Provider 回归；只改测试文件，不改变预登记场景和预测。
- [x] 正式 E0 单次续作通过；随后 E1 trial 1 遭上游 429/503 并超时，已保存无效尝试并停止后续调用。
- [x] 后续已完成 E1 剩余槽位及 E2a 两次尝试；E1 有 1 个有效样本，实验整体仍未完成，详见本页当前清单。
- [x] 独立检查代理复核新增测试和全部当前差异；更新问题映射、父任务状态及 runtime 子任务可执行设计。

## 原实施顺序（历史基线）

1. 创建独立 worktree，记录 85b98c3bb、8 个修复链、工具版本和主工作区状态。
2. 将 8 个修复合入实施分支；解决只影响集成 spec/生成文件的冲突，保留 85b98c3bb 的 core action changes。
3. 修复并验证迁移唯一前缀；运行 sqlc 和 generated diff。
4. 先写附件预算边界失败测试，再实现引用必保/可选历史尽力补位；覆盖小 FileCount、PerFile、Total、Git 保留、引用超限、引用丢失和小队不同 runtime。
5. 修正文案动作映射；补接收方 handoff e2e 可见性和正常状态选择断言；补 comment 64 KiB+1 服务层不落库回归。
6. 在最终 HEAD 运行 migration、sqlc、Go vet/build/targeted tests、pnpm lint/typecheck/test 和相关生产 Web e2e。
7. 硬化实验框架：固定 source/framework/model/CLI metadata、trials=3、完整 prompt/tool/outcome/proof、durable verdict；用 fake provider 验证。
8. 串行运行正式 E0，成功后按 E1–E6 预登记顺序运行并保存原始证据。
9. 更新问题→修复→验证→限制矩阵和首轮验收报告。

首轮合入门槛：迁移 lint 与 sqlc 无漂移；附件矩阵通过；B/C 证据补口通过；最终全栈检查有版本化日志；正式 E0 通过，或环境故障被完整记录且明确不宣称产品通过；E1–E6 结果不被环境失败伪装成成功。
