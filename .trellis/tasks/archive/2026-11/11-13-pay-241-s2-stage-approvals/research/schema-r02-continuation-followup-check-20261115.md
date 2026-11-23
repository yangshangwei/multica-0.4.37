# PAY-241 SCHEMA-R02-E01 窄补充复查（2026-11-15）

结论：**PASS（六项已独立核实的旧 raw controls + 一项 root 新执行的 immutable supplement）**。这不是原第七项运行的原始证据，不能追补、替换或改写原 blocked 报告；不代表 runtime、writer、S2 2d 或 A01–A12 验收。

## Findings (fixed)

- 无需修复产品或校验器；仅新增本 followup 两份研究报告。

## Findings (not fixed)

- 原 `SCHEMA-R02-E01` 第七项 immutable 原运行 raw 仍缺失，这一历史事实不可追补。原 `retention-results.json` 与原独立 blocked md/json 的精确哈希均未改变。新证据已满足本次允许的“6 old + 1 fresh”有限 controls 结论；当前 supplement 没有阻挡项。

## Verification

- 独立文件/事件核对 **39/39**；复算 **13 个文件 SHA256**。本次未重新遍历 4949 源文件、完整 up、分配或旧六项控制；旧六项只继承已封存的原独立 review。
- 新 raw SHA256：`c07b48d36cfd2d12be89a7bd8351f07ff9ad5c0796b434564b0420f8815b5499`，与本次 dispatch 完全匹配。脚本 SHA256：`2752488b2a45a3d5082f86e71b78b8133259873fa661148556dea7da41926407`；实际脚本调用 pg 并记录 SQL、params、服务端 error，close 时间在 `await client.end()` 后生成。
- config SHA256：`0dd6708ff4dc4b73e3c65198449eabbe9913b0f8bc95e2c920e1dafb3b7449e9`；0600，路径、分配库与原独立核对一致；未打印凭据。
- 原 retention SHA256：`2750851b6c4d698fc2b890522685025c18b4022d56cc3d230455889c7ccab3e0`，与原 reviewer 保留证据一致。原 immutable 仍只有结构化 claim，未被补写 raw 字段。
- 新 raw 有 **7 个 SQL events、1 次 UPDATE**：身份 → before → BEGIN → UPDATE error → ROLLBACK → after → 身份。SQL 为 `UPDATE collaboration_stage_submission SET stage_revision=2 WHERE submission_id=$1`，参数只有原 synthetic ID `04ec1bce-2286-4e0f-a5eb-56a23f5ce9d7`；原 retained stage_revision 为 1，不是空命中。
- 服务端 SQLSTATE **23514**，message 为 `collaboration stage fact collaboration_stage_submission is immutable`，where 指向 `collaboration_stage_immutable() line 3 at RAISE`。805 函数体与 861 BEFORE UPDATE trigger 的 frozen/current 源字节哈希匹配，函数体也匹配原独立 catalog。
- before/after 均 **1 行**；MD5 均 `4afe7a9c8e28f10337d371fd24d82129`，同时匹配原 retention 与原独立只读行。分配库始末其他连接均 **0**；BEGIN/ROLLBACK 均完成，root close 时间 `2026-11-15T04:14:13.706Z`。
- Reviewer **0 次数据库连接**，没有重新 UPDATE、DDL、迁移或启动 API；不把 root fresh transcript 描述为 reviewer 的新 live/catalog snapshot。
- 绑定脚本 `node --check` 通过（仅语法，不执行）；followup JSON 交付前解析检查。项目 lint/typecheck 与 service/handler/router suites 未运行。

## 保留与边界

原 blocked 报告、私有旧 evidence/manifest 和 root 原 retention 均保留不动。所有精确哈希、39 项独立断言与旧六项控制的既有引用见同名 JSON；没有声明七项属于同一原始运行，也没有补造 standalone exit 文件。

- 新 raw：`/Users/artisan/.multica/acceptance/pay241-20261115-continuation/schema-r02/logs/immutable-control-supplement.json`。
- 原报告：`.trellis/tasks/11-13-pay-241-s2-stage-approvals/research/schema-r02-continuation-check-20261115.md` / `.trellis/tasks/11-13-pay-241-s2-stage-approvals/research/schema-r02-continuation-check-20261115.json`。
- 本次仅写：`.trellis/tasks/11-13-pay-241-s2-stage-approvals/research/schema-r02-continuation-followup-check-20261115.md` / `.trellis/tasks/11-13-pay-241-s2-stage-approvals/research/schema-r02-continuation-followup-check-20261115.json`。
