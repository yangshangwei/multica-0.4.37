# RKI6 当前 normal/race 结果独立复核

结论：**pass-with-notes；未关闭的阻塞问题 0**。复核仅确认当前固定源码上的 fake/sim 执行与证据完整性，不关闭完整 `make check`、真实 Provider 或父任务发布门。

复核人：原生 `trellis-check` 子代理 `/root/rki6_result_review`。2026-11-02；依据主会话的当前任务 dispatch，未启动其他代理、服务或数据库测试。

## 核实结果

| 范围 | 实际复核结果 |
| --- | --- |
| 产品源码 | final3 `639bf6ef85ad894553b244873c2ed8d9f9dd29b2` 与 prepared final4 `64a9994ec549e8dff73ab14456dc9a9f4b63f7c7` 均为 7,960 个文件，产品根目录干净，指纹均为 `9f72fc214b8e81f6fc3c1969be5e3304b7f158f60a6f7305f1531d55574d80ee` |
| RK1 框架 | 两工作树均为 17 个 `rk1_*_test.go`，重算 `5eaea44f480f8e94a9a90d35025ca2d68d76c895c5bf70fa902da37c5bed94c9`；与登记相同 |
| normal 原始执行 | 原 JSON/JSONL 与汇总 SHA 完全一致；具名 run/pass 各 62，fail/skip 各 0，进程与记录退出码均 0 |
| race 原始执行 | 原 JSON/JSONL 与汇总 SHA 完全一致；具名 run/pass 各 62，fail/skip 各 0，退出码均 0；无 DATA RACE 标记 |
| 每轮样本与启动 | 19 个夹具类别、22 个 public trial、32 个 fake/sim 物理启动；没有缺失或额外 trial，原编号保留 |
| 样本原始证据 | 两轮共 44 份 public、44 份 trace 的哈希均匹配；64 个 launch 的实际 stdin、launch metadata 与 stdout 已核对，permit/bind/原始 prompt 哈希/termination/proof 一致 |
| 暴露与判分 | H1 停止前记录不完整且结果缺失；H2b 真码不在启动 source 前缀、由 CLI source 读取且进入 tool_result；H3a/H3b 按实际 CLI/模型可见性复判；H5 canary 暴露与写入判据一致 |
| 故障/停止规则 | 错误但完整答复继续；crash 从 trial 2 保留编号并停止 trial 3；未暴露与 Provider 报错为 incomplete；canary 被执行为 blocking_review；隐藏预览为 fail_not_visible |
| H6 | 两独立目录/ledger，hop1 memory PUT 和 hop2 memory GET 各 1；事实只经 memory 到达 hop2，原始答案包含登记的决定、两个被拒绝方案和迁移风险；保留 manual_review_required |
| 历史失败保留 | 38 个已有 tracked fake-proof 文件与入口提交 `7af9ab990` 逐字节一致，未改判或覆盖 |

可复跑的纯证据核查：

```sh
python3 .trellis/tasks/10-30-collaboration-context-handoff-runtime/research/continuation-20261102/rki6-review/verify_evidence.py
```

本轮实际输出：1,794 项跨记录一致性检查通过，0 失败；退出码 0；数据库操作 0，真实 Provider 调用 0。完整哈希、具名测试和逐项核查见 `verification.json`。这些一致性检查不是额外的产品测试数。

normal 原 JSON SHA：`d9a61875e2c0dcf308e2070d4afc9addcad23db8a21e5ab49d2f08fba0eb27d9`；JSONL SHA：`fd95c62e1f6a17897a41eaa5aa9bb020f31ba8298ed5eb602b5fd1671352cf5a`。

race 原 JSON SHA：`bf8dadd829eef7c3f4d5ce4445f17e7faa1bd63b569760c26c6bf7593aa1fc0d`；JSONL SHA：`c5395236ed41215d5741f19579a13bee42697fb4bd615aff5018fdbe5b3dffd2`。

## 未修改的范围与说明

- **RKI6-N1（低，非阻塞）：H6 fake 输出不验证三行格式遵从。** 两轮 fake relay_read 均直接返回六行 memory 内容；问题正文要求最多三行，而登记的 H6 scorer 只保留语义材料与 `manual_review_required`。这不是一份真实模型答复或已通过的行为样本，不据此修改登记、冻结框架或追溯改变判分。后续真实 H6 必须继续手工复核；若要把行数升级为独立验收判据，应在相应实验前明确登记。
- **H2 的当前范围明确排除了第二结果页。** 当前 `runtime-addendum.json` 的 `product_fix.measured_by` 明确写明 H2 只测 member result/readers/wake source，H2b 测截断 source 全文，H3b 测旧评论分页；超过 32 个 result/cause 的分页不由任何 RK1 场景测量。这是当前批准范围，未作为新增阻塞项。
- 真实 Provider、原生 resume、安装版/多账号、全量候选验收均未由此证明。fake 每场景的一轮或特定三轮不能代替登记的 25 次真实调用预算。
- `quiet-preflight.json` 的历史进程/共享网关静止声明只能核对记录和时间；本次不重建当时外部进程状态，也不据 fake 结果给真实串行窗口放行。
- DB 事件计数及 H5 保留行计数来自已完成时的原 public 观察。复核结合原 trace/stdin/outcome，未重连数据库重放当时状态。

## Verification

- 证据核查脚本：pass，exit 0；Python AST 语法检查：pass。
- Lint / TypeCheck：未运行，限定为只读证据复核且无产品或 TypeScript 修改；主会话的新一轮全量验收负责这两个门。
- Go normal/race：核实原始完整执行与每个具名 run/pass，未重复执行数据库测试。
- 产品、实验框架、adapter 和共享任务台账：本代理未修改。新文件只在分配的 `rki6-review/` 目录内。

主会话可按本结论关闭 **当前 RKI6 fake/race 执行结果的独立复核**；最终 paid source 绑定和 `runnable` 仍须等待其他必要门与授权条件。
