# PAY-241 推进计划（2026-11-18）

继续现有任务，按已接受的 S2 候选进入 S3。主会话负责进度、环境和整合，原生 Trellis 实施与检查代理负责明确的文件范围。

## 当前进度

| 阶段 | 已有结果 | 剩余交付 |
| --- | --- | --- |
| S1 身份与闭环基线 | 已检查、提交并归档 | 保留原证据 |
| S2 角色审批与阶段门禁 | R33 完整代码及合成验收独立接受；12 A、18 W、12 Q 全部核对；原四旅程 4 PASS / 0 FAIL / 0 NOT_RUN、零重试 | 提交与归档待处理；真实外部证据属于 S4 |
| S3 执行与协作恢复 | timeout-proof 已归档；v2/r05 规划已通过；字段并发首增量及 TestMain 清理已独立接受，normal/race 与 ROOT 检查通过 | 当前 API/Web/Desktop 并发与批次补读、混合通知、上传恢复和阻塞四项验收 |
| S4 外部集成 | 接入清单与受控合同已准备 | I02–I09 的真实沙箱、仓库/CI、非生产部署/回滚、指标及人员输入 |
| S5 性能与总验收 | r02 工具演练已检查 | S2–S4 最终候选上的正式性能采样及业务签字 |

父矩阵为 **2 PASS / 12 PARTIAL / 10 NOT_RUN**，完整 24 条分母保留。TC-05、TC-08 已通过；TC-16 本轮以 service/HTTP 并发和原回执证据推进为 PARTIAL，当前客户端组合仍待验证。S2 合成签字和受控外部事实不替代支付、真实模型或业务验收。R31/R32 及更早失败仍在历史记录中，S2 结论以 R33 最终报告为准。

## 推进顺序与验收门槛

1. **校正进度并绑定 S3 前置。**核对 R33 完整 S2 与运行后报告、源码 manifest 和既有 v2/r05 规划；更新父任务的旧等待字段与 S3 的 implement/check 上下文。已完成最终源码全量哈希核对并建立独立验证副本，原运行库、历史与成功请求保持。
2. **S3 首增量：优先级与处理人并发。**增加 `server/internal/service/collaboration_recovery_cas_test.go` 和 `server/internal/handler/collaboration_child_recovery_cas_test.go`，并修正本轮真实复现的 `server/internal/handler/handler_test.go` 退出清理缺项。复用真实 PostgreSQL 提交/PID 屏障，验证同键同载荷、同键不同优先级、不同键同版本竞争、处理人转交与优先级两种提交顺序、原 GET 回执恢复及普通关联子任务的显式 numeric CAS。每个 package/mode 使用新专库，检查精确字段、版本、原请求和副作用；通过 normal/race、gofmt、编译、vet 和独立审查后记为局部证据。
3. **S3 批次恢复与通知。**复用 canonical E2E 和现有 outbox/Query 恢复，覆盖 55 条跨页离线评论、实际 WebSocket 重认证、撤权、人员/智能体混合提及和原事件通知去重。分别核对评论、运行、通知及附件的 ID/计数；API、生产 Web 和匹配 Desktop 取得实际证据后才推进对应父例。
4. **S3 阻塞与解除。**单一服务端 owner 按已通过的 v2/r05 合同实现原因、责任人、解除条件与预计解除时间，以及原事务 repair/return、精确历史和 pending submission 保护。冻结 DTO/read/query 后再让 Core/Views owner 接线，覆盖旧请求 hash、nullable authority、分页/HMAC/预算和原键恢复。完成 R01–R09 全范围独立检查后才关闭 S3。
5. **S4 真实非生产接入。**按既有 I02–I09 清单登记实际支付/履约、远端仓库/CI、45 秒回调/重复/乱序/MQ、部署/回滚、指标与审核身份；输入齐备后执行 E01–E07，缺少输入的条目继续 NOT_RUN。
6. **S5 最终采样与父验收。**候选稳定后运行 100 个独立用户 × 100 个状态事件，保留全部 10,000 个传播样本和 100 次回写。检查传播 p95≤2 秒、回写 p95≤5 秒及未收敛样本，再逐条核销 24 个用例和产品/研发/QA/项目经理签字。

## 本轮实施边界

首增量补现有行为的字段与回执证明。转交先提交时，原非创建人处理人的旧修改为 403；优先级先提交时，旧版本转交为 409。普通子任务仅证明显式版本合同，保留旧客户端可选版本语义。若出现真实失败，先保留原运行并诊断，再限定产品修复。

实际执行已复现公共 TestMain 留下一个无外键的调用目标关联，零测试体的独立 RED 也复现。修复范围仅扩到该已有测试夹具的退出清理，不在子测试中提前撤掉共享种子权限。原失败库/源码/输出保留，修正后用新库证明清理和 child normal/race；详情见 S3 `research/testmain-cleanup-diagnosis-20261118.md`。

本轮实际结果：service 普通/竞态各 7 名、handler 修正后普通/竞态各 3 名全部通过，名称集合一致、零失败/跳过/缺失/重复；各自 30 类夹具记录与连接均清理。三个 Go 文件已精确整合回 ROOT，额外两包编译、vet 和三文件格式检查通过。原 handler 3 名通过但清理失败的整轮保持不接受，TestMain-only RED/GREEN 单独保留；不增加业务用例分母。

不新增恢复系统、依赖或数据库迁移；本轮代码与规范已按用户指令提交96a2bffe3，交接记录单独提交；完整S3、S4、S5及归档仍开放。真实模型账户、资金或生产部署继续依各自授权范围执行。

证据入口：父任务 `session-progress.md`、S2 `research/r33-full-s2-a01-a12-independent-check-20261118.md`、S3 `research/s3-source-binding-20261118.md` 和父任务 `research/s3-cas-intake-20261118.md`。

新会话入口：`.trellis/tasks/11-13-pay-241-s3-recovery/research/new-session-handoff-20261118.md`。本次代码提交仅含三个Go文件和两个spec；此前S2、父任务历史与其他工作区WIP保留，后续先核对归属和当前来源。

当前会话已接续S3，细化计划见 `.trellis/tasks/11-13-pay-241-s3-recovery/research/current-clients-plan-20261118.md`。共享工作树按完整文件视图保存388条基线，原index为空；已接受R04持有源码无漂移且ROOT生产路径一致。新证据/状态根单独分配，由主会话核对配置、真实专库和构建身份后执行。下一增量先补当前API/Web/Electron字段并发与原回执，后续以完整55条评论分页和真实WS重认证推进，不沿用单条评论旧证据判PASS。

## Current-client field acceptance R06 — 2026-11-18

The complete fresh password API/production Web/matching Electron four-case field run is independently accepted:4tops/8orderings, retry0, source/config/raw/persistent IDs and owned gate/PID/native closure reconciled. Original operations and both current-authority403/stale409 results remain exact through fresh authorized continuation. Ordinary numeric API CAS is separately labelled from actual optional-version UI writes. TC-16 remainsPARTIAL; parent24 stays2PASS/12PARTIAL/10NOT_RUN.

Existing notification/upload/revocation/proven-result contracts were revalidated on four separate normal/race databases: service13tops/18names each, handler3 each,42named executions;30business relations0, protected history retained, originalgate restored andconn0. This does not accept current extra/browser/runtime compositions. Firstfive client failures, native noauth transport probe and all prior failures remain retained. Next: separate fresh55-comment/realWS profile, extras and structured blocking/repair. Blockserver R01's first16DBtops/43names has3failingtopcontracts; candidate/source/DB/raw frozen before R02 repairs. FullS3 remainsinprogress and unarchived.
