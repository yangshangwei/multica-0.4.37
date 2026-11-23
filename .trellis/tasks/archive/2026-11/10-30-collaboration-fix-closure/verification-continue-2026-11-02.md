## 2026-11-04 共享唯一执行入口同步

现有 #6/#7 续作已核对无旧完整检查后登记接管，保留 v14 make2/身份pass 和原 claim/失败数据。新候选 `d41573e3a`、69 项离线准入及独立实际字节复核通过；完整 v15 由唯一 `/root/shared_executor` 实际执行，Make **54354**、launcher **54337**、wrapper **54333**，当前静态15项通过、TypeScript进行中。最终 exit/audit/全具名与逐项skip清单待实际结束。当前入口：[推进计划](../11-01-collaboration-knowledge-audit-fixes/advance-plan-2026-11-03.md)。知识预算/调用0/0；本轮继续不触及其他真实模型/发布额度。

# 首轮闭环本轮推进与验证

本轮用户要求列出推进计划并继续，后续明确授权跨会话协调。沿用现有任务，状态保持 `in_progress`。本页按实际结果更新；v6完整检查失败保留，修复后的v9完整检查正在执行。

## 候选与归属

- 集成入口 `7af9ab990`；独立被测候选 `64a9994ec549e8dff73ab14456dc9a9f4b63f7c7`，开始时干净。
- 产品 source ID `9f72fc214b8e81f6fc3c1969be5e3304b7f158f60a6f7305f1531d55574d80ee`，已与639产品树核对一致。
- 首轮会话为完整检查、私有Redis16422/renderer19845及原14次实验的唯一执行者；runtime25次预算由runtime会话独立核销。其他会话的源码与未提交文件保留。
- 详细推进顺序：[推进计划](../../../docs/plans/2026-11-02-collaboration-fix-closure.md)。

## 当前验收表

| 门 | 实际证据 | 当前状态 |
| --- | --- | --- |
| 私有验证入口 | v6的83项fake/6文件语法及交叉复核，v9的86项fake/6文件语法独立复核，原技能catalog7/7实际红绿 | final-v9 pass-with-notes，0阻塞 |
| RKI6原fake结果 | normal/race各62pass，来源/框架/原public/trace/物理启动独立核对 | 结果复核通过；真实模型门独立 |
| 完整静态检查 | 仓库完整命令的15个lint/typecheck任务 | 15pass |
| TypeScript | core4340、docs62、views7118、desktop1272、web290 | 13,082pass，0fail |
| Go/race、build/vet、migration具名测试 | v6失败保留；v9 cmd/server完整包735pass/0fail/6skip，其余包继续 | v9执行中，不能提前计整套通过 |
| 当前sqlc | source45efa093c的sqlc1.31.1生成exit0/无漂移；当前相关1754文件全部0diff | 有版本依据复用 |
| production Web /默认Electron E2E | v6完整run因Go失败未进入最后阶段 | 尚未运行 |
| C3 password4项 + runtime handoff3项 | 同候选、独立password部署已仅准备配置/URL/资源登记 | 尚未运行 |
| 原实验E3 after2/3、E4–E6 | 原产品HEAD、新停止规则框架、固定CLI2.1.316与14次登记 | 前置准备/复核通过，真实调用0 |

完整检查实际开始时间2026-11-02 14:47 UTC，15:02 UTC以exit2结束，make PID91222。命令为私有审定启动器调用仓库 `make check`；原记录和日志：`/Volumes/artisan/code/2026/mc-collaboration-final4-20261102/.omx/private/final-candidate/make-check-v6.json` / `.log`。本次及历史第三轮exit2/47个失败分别保留，不能相互重标。

技能catalog原断言在两个独立新迁移克隆库复现：v6为7run/4pass/3fail（含父节点），v9为7pass/0fail/0skip。Go TestMain直接创建workspace，没有public workspace API的组织初始化；继承的API发布配置激活组织catalog读取而返回503。v9仅从mode1包测试子进程移除`MULTICA_RESOURCE_PUBLISH_DIR`，保留API/编译/非测试及mode0测试显式选择的配置；产品与原断言不变。[实际红绿](research/continue-20261102/catalog-actual-redgreen.json)。

新v9完整run于2026-11-02 15:36 UTC实际启动，make PID20588；原记录/日志`make-check-v9.json` / `.log`及`go-packages-v9/`均在final4私有目录，新registry/HOME/配置和证据文件不覆盖v6。15项静态任务、13,082项TS再次通过，Go阶段进行中。

## 本轮最小修复

1. Go测试与API池配置分离，各包使用独立私有HOME和显式缓存；递归入口保留测试选择的HOME。
2. dotenv按数据解析，归一化值拒绝Make可执行美元符号/控制字符；安全副本采用O_EXCL/0600，与实际服务环境一致。
3. 启动前核对实际candidate、Go/pnpm、自身launcher和source配置哈希；缺失/变化/阻塞复核在启动前拒绝。
4. 精确绑定递归测试的canonical package；未知参数、模块/overlay改写、`-c`取值混淆及合法全局`-- test`均有归档红绿证据。
5. E5首个canary行为失败立即停止后续试次，保留`finished/fail`并区分运行不完整。旧源码、原三文件框架和已消耗试次保留。

上述改动位于任务私有验证代码和两条独立原产品worktree的测试框架覆盖层，来源与完整diff见 [实施证据](research/continue-20261102/adapter-fix/summary.json) 和 [E5框架证据](research/continue-20261102/original-framework-guard/summary.json)。

## 实验绑定与预算

- 原baseline `c58f79874cfe2e6318e3dce067d7b6bac24e943f` / after `835e7b046c6841e0945a59326bc4ddcbae4a0df7` 产品不变。
- 历史三文件框架 `f6f62e55888fa2d3e733ff12dec6054e07f877828363ad0d503ada6db07aa0bd` 保留；仅停止规则补口后的四文件successor `ff2ac795d7ec3a6ca9664276200fe61f2526fe40c496ad7bc137b42099bd08c0` 在两个新worktree一致，各32fake通过，独立复核0阻塞。
- 新登记只追加剩余槽位：E3 after2次、E4 baseline3次、E5 baseline3次、E6 baseline6次，共14；固定model/thinking/tier/超时/判分不变。旧无效槽位不替换。
- [原实验复核](research/continue-20261102/original-series-review.json)允许完成前置门后由统筹填执行放行，首个不完整或E5 canary立即停止其后真实调用。

## 下一入口

完成当前v9同一候选的完整检查并读取每包及每个E2E实际结果；随后补password7项，再确认独占模型窗口按新登记续跑E3 after trial2。原始失败、条件skip、模型未观察字段与其他子任务的未验门继续分别记录。
