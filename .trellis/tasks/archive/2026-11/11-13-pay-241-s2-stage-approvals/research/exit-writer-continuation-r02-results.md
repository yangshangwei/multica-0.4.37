# PAY-241 S2 exit / writer continuation r02

**结论：四个新增测试切片已在精确的中间产品输入上验证并合入 LIVE；独立最终服务端整合/复跑仍由 root 负责。** 当前不接受 A01–A12，也不改变父 24 例分母。

## 来源和范围

私有根：`/Users/artisan/.multica/acceptance/pay241-20261115-continuation/exit-writer-tests-r02`。最终测试源为 `source-product-r02`，4,961 文件 inventory SHA256 `222456cd0cf3a77920bdc20cd3e13cfbc31c953a33539bcdc0682dffc1d3b7e9`；基线 green-r03 manifest `3e4bc0270a1e675ebb0a9cae4e479853393c6d796266d049f4e07f1728a3e7b3`。

只消费 owner 明确提供的 **15 个中间产品路径**，manifest SHA256 `22421f02d6a7ca2688eeb95c1c056e04d72086cfac07f7d2ed076977e01ee24e`。没有复制动态 LIVE 产品，也没有包含 owner 后续 R38 blocker-envelope 六行或最终产品集合。原知识删除合同保持；没有新依赖、DDL/迁移、数据库分配/删除或 Git 操作。

## 验证

| 切片 | 顶层 | normal named PASS | race named PASS |
| --- | ---: | ---: | ---: |
| service exit | 4 | 17 | 17 |
| handler exit | 5 | 30 | 30 |
| service writer | 4 | 16 | 16 |
| handler writer | 7 | 32 | 32 |
| 合计 | 20 | 95 | 95 |

每种模式均 0 FAIL / 0 SKIP。named 包括父及子测试记录；normal/race 分别记账，不相加成业务用例。八条最终命令及前后源码快照都与同一 4,961 文件 manifest 相等。`static-vet-r03`（service/handler）、`static-build-r02`（完整 Go backend）、`static-gofmt-r02`（四文件）通过。完整顶层/named inventory、真实 argv、命令/日志/退出码/源码 SHA 与全部原失败尝试见伴随 JSON，raw 在私有 `logs/`。

所有 DB 测试使用原 `pay241-20261113-01/run.mjs`，八个已分配 exact config 分别绑定包/normal/race；只运行 `./internal/service` 或 `./internal/handler` 及新增独有前缀 `^TestStageExitContinuation` / `^TestStageWriterContinuation`。执行 HOME/TMP 均在仓库外。前后审计核对身份、835 ledger/末864；最终八库其他连接均0。

## 覆盖和产品红例

- W11：实际 EditIssue move/clear × stage enabled × M1 off/observe/enforce；原键拒绝恢复、显式关 stage 后新意图、M1 reason/done/source 权限原限制。
- W12：实际 impact/delete 三模式；关 stage 后独立 M1/协作项目限制、无政策空项目 virtual-off 正例。
- W13：实际 root/mixed/普通父删除，在取消、token、quota、附件之前拒绝；真实预检查后协作挂载提交；三处必需写故障让 task/token/仍 reserved run_only quota、period、autopilot 和附件全回滚，0任务取消/issue删除事件。
- Q10：两个真实 backend/goroutine，观测锁等待后让首事务真正 commit/rollback，再收等待 writer 结果。覆盖 policy replacement/complete、root clear/complete、project delete/submit；政策 writer 原 NOWAIT 拒绝单列控制。
- W05–W09：实际 PUT/旧 payload/Agent fixture/Move/root mixed batch；普通 child 的现有 M1 grant、明确 TargetPosition、批量 committed/rejected/not_attempted 前缀及重复 authorization/operation 拒绝；Create/CreateInTx、custom done、人工/comment-to-child 与已批准机器物化；实际 WriteIssueStatus、advanceIssueToDone 和签名 GitHub/Forgejo webhook root 拒绝。
- W14 补充：普通 child 的语义编辑/parent/project/delete 保留 mapping 并使旧候选不可消费；仅 status/position 不使其代码自己失效。删除后的 R4 仍可有 current head 引用，但隐藏私有 baseline 且真实 A9 拒绝。

**EXIT-R02-C01**：旧实际删除在事务复查前已 Cancel/Fail；最后 DeleteIssue 写失败时还留下 token/quota 变化和一个取消事件。`exit-handler-red-r01/r02` 与独立 `exit-handler-postquota-red-r01` 保留。post-quota 是同一根因的补证，不算第三个产品缺陷。owner 将副作用移入原删除事务后 normal/race 转绿。

**EXIT-R02-C02**：P1 明确 200 virtual-off 的普通空项目，实际 delete validator 因忽略 `found=false` 而返回 `*CollaborationError stage_schema_unavailable`，DELETE503。失败在原 knowledge hook 之前；早期知识/peer DDL 猜测已撤回。owner 保持知识合同并修正 absent 与未知已存 contract 的区分，三模式204转绿。

夹具/控制失败也全部保留：密码凭据 provenance、原等待测试 SIGQUIT、handler 格式漂移、重复 agent 名、首次空 A5、workspace counter、M1 exact position、严格 A9 body、terminal 查询和 Go vet 锁复制。最后只修新测试/私有输入，没有改产品断言或现有 helper。after-audit 旧 `execution_epoch` 只读42703保留，改查实际 `epoch` 的新审计通过。

## 四文件交付

- `server/internal/service/collaboration_stage_exit_continuation_test.go` — 305行；SHA256 `d0321fd11ba88c42cc9eb373b015c0c5682dc4f82262a18692821c4688088deb`。
- `server/internal/handler/collaboration_stage_exit_continuation_test.go` — 568行；SHA256 `8c14dd6c3bcef49c2e52962ba8ea9f29e9dc19d9f6037bfbaf3b892a13ad4c95`。
- `server/internal/service/collaboration_stage_writer_continuation_test.go` — 146行；SHA256 `188dad13be50f366bcfc6ed4d94eeca0a0a63c03699ebdf1b34a758281b4168f`。
- `server/internal/handler/collaboration_stage_writer_continuation_test.go` — 468行；SHA256 `7e9323fe9387e831c112d340d6613ada15a93a4ee53a4247ef409394e99f695b`。

root 曾明确冻结 ROOT，本 slice 期间只在私有源推进；收到明确释放后，用 `apply_patch` **仅新增四文件**，合入前确认不存在，合入后字节与已测试源一致。产品、既有 tests/helper、SQL/sqlc、其他 peer 差异未改。两个 patch 都保留在私有 `evidence/tests-r02.patch` / `tests-r02.apply_patch`。

复用 `internal/testutil` 的 row builders、Call/Want/JSON、既有 stage/M1/Issue fixtures 和真实 writer。没有新完成器、自然语言审批解析、重复 SQL INSERT/httptest quartet 或新依赖。

## 限制与交接

- root 继续整合最终 server manifest、独立 Trellis 检查及最终精确包/模式复跑；当前只是这15路径中间源的证据。
- 人类 principal、PR/CI 和本地受控制品都是明确测试夹具；没有真实账号登录、远端 CI、部署、支付、模型或 production 验收。
- 原 SIGQUIT 的 exit_service 库故意保留1 policy/1 role/1 candidate/4 decisions，execution enabled=true、epoch3、0连接；未删受保护历史或猜测恢复。其余七库 stage facts 为0。root 应用已分配的干净独立最终库。
- 没有开启 P2、API/browser/Provider 进程或改父矩阵；A01–A12/父24例保持由主任务记账。
