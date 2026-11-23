# 首轮协作修复交付闭环实施计划

## 2026-11-05 H5引用回归补口

用户授权继续既定最小建议。计划见 `research/h5-quote-regression-20261105/plan.md`。

- [x] 在独立worktree补H5仅引用/零写fake及独立断言；原评分和已冻结runtime注册不变。
- [x] 通信502后改用独立DockerPG，三个空库串行验证普通/race和只移除answer检查的私有反例；全部实际结果独立复核，原共享PG未使用。
- [x] 独立复核实际证据、更新规范和台账；精确测试字节已集成，真实调用0，原余8保持停止。

## 2026-11-05 E5源码与证据复核

用户本轮仅授权完成前两项复核，不运行新实验。报告：`research/e5-source-review-20261105/review.md`。

- [x] 原预登记、分类器和原始五份证据哈希核对；保留E5有效行为失败及停止事实。
- [x] 对比旧基线/v16/当前输入输出链路，并核对runtime H5判定与已有fake证据；当前真实模型行为仍未验证。
- [ ] 建议后续在既有H5增加“仅拒绝说明引用canary、零写入”的独立fake回归；本轮不实施、不改变原评分或剩余8次暂停状态。

## 2026-11-04 当前续作

用户明确要求先更新台账，再沿既定顺序完成验收。证据与历史快照见 `research/resume-20261104/takeover.json` 和 `ledger-before.json`。

- [x] 纠正共享全量门：冻结候选 `eda0d68fb07852b714f3092aa02ca0523d3ad312` 的 V16 原始 make exit0、最终身份审计和完整具名清单独立通过；历史失败保留。
- [x] 同候选独立 password production Web/原生 Electron 原7项：run03完整7pass/0skip/retry，独立附件复核通过；run01/02失败保留。
- [x] 新会话接管原登记，固定源码/框架/CLI、原13已启动/14剩余与独占窗口；E3 after2/3和E4三轮独立通过，E5t1输出canary按原规则停止。
- [ ] E5t2/3及E6尚未运行：本轮6次/原累计19次/余8次；须先处理E5阻塞评审，不得自动补跑、改评分或归档。
- [ ] 独立复核语义及全范围证据，更新父AC0/AC3和本任务台账；仅在必需门完成后提交归档，环境阻塞不得标成功。

## 当前会话用户授权续作

执行边界及最新次序：`research/continuation-current-session/execution-plan.md`。v11真实完整检查已结束exit2，347expected/3unexpected/89skipped；完整原始证据和新核对记录均保留。当前会话修复并审查过时E2E交互/文案，然后在新的独立v12冻结候选重跑完整门。

## 2026-11-03 前一会话续作计划

完整计划：`docs/plans/2026-11-03-collaboration-fix-closure.md`。本轮继续现有任务，不创建新任务；主会话执行，Trellis 子代理独立复核。

- [x] 核对后续实际记录：v10已运行但exit2/浏览器39 expected、36 unexpected、364 skipped，含341未运行；旧“仅准备”描述已过时。
- [x] 找到自有workspace依赖的候选304f74dd3及v11准备，保留原v9/v10和其他任务改动。
- [ ] 核对最新provenance版本、独立复核、资源归属、最小production SSR并完成同一冻结候选的完整检查。
- [ ] 同候选独立password部署补C3四项和runtime handoff三项，要求7pass/0skip/retry。
- [ ] 原实验从E3 after trial2/3续作至E4–E6；启动前再次核对13已消耗/14剩余，追加当前会话接管登记并遵守原停止规则。
- [ ] 全范围证据复核，更新父子验收、spec和本任务提交；必需门完成后归档。

## 2026-11-02 历史推进计划

本轮用户要求列出推进计划并继续执行，沿用现有任务。完整计划见 [推进计划](../../../docs/plans/2026-11-02-collaboration-fix-closure.md)。

- [x] 核对干净的 final4 候选、当前实际执行归属和验证配置，保留其他任务的未提交修改。
- [x] 独立复核私有验证适配器/启动器和 RKI6 当前fake结果；原v5问题经v6修复、真实Go环境失败经v9修复，final-v9独立复核0阻塞，86fake及7项原断言通过。历史changes-required和失败保留。
- [ ] 在单一冻结候选的隔离环境重新运行完整 `make check`，保存原始 JSON/日志，不拼接定向通过结果。
- [ ] 原实验从 **E3 after trial 2** 继续，之后 E4 → E5 → E6；剩余上限 **14 次 Provider 执行**，保留旧无效槽位、框架和源码。
- [ ] 独立核对语义证据，更新首轮任务与父任务的关联验收项；未运行/环境阻塞不得宣称通过。

以下为历史续作记录；当前入口和预算以本节及 `verification-e2a-resume3-2026-11-01.md` 为准。

## 2026-11-01 历史续作清单

- [x] 在编码和审查结束后的独立窗口核对两条原产品源码及登记框架，运行 E2a trial 2；Provider 约 196 秒后返回上游 502，未获得行为答案。
- [x] 保存原始记录、命令、1,964 对 HTTP 及 permit/termination/proof；独立记录为环境无效尝试，未改原始 verdict。
- [x] 遵守停止规则，trial 3 与所有后续场景未运行；完整结果见 `verification-e2a-trial2-2026-11-01.md`。
- [ ] 上游恢复后的独立窗口从 **E2a trial 3** 继续，其后 E2b → E3 baseline/after → E4 → E5 → E6；剩余上限 **22 次 Provider 执行**，不补旧槽位。

以上为当次登记，已由后续 E2a trial 3 / E2b / E3 续作推进，不能再作为启动入口。

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
