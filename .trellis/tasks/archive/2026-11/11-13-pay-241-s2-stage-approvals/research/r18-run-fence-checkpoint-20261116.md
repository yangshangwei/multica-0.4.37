# R18 站立审批读取回归与 run 栅栏检查点

Active task: `.trellis/tasks/11-13-pay-241-s2-stage-approvals`

本文接续 `r17-original-four-failure-and-after-20261116.md`。当前最新原四场景仍是 **1 PASS、1 FAIL、2 NOT_RUN**；R18 尚未执行新原四，未更新 A01–A12／父矩阵，未声明 S2 接受。

R18 外部根：

`/Volumes/artisan/.multica-acceptance/pay241-20261115-continuation/resume-r05/r18-stage-execution-read-lock-fix-r01`

## 已实际完成的检查

新增规范回归 `collaboration_stage_execution_read_concurrency_test.go` 215 行，SHA256 `16f61b61fff07fb4cf609bf27a141093d67d46afe96f2448a9605a7c7709a75c`。冻结 RED 共 5,043 文件，仅新增此文件；既有 R17 回归 SHA256 `2053aa790974aea910a908634a18953748279f9028659c2dac4d1b28e74630f6` 保持。

原三个顶层／11 命名／8 叶用例包括真正进入有效 QA／release／unknown intent 的 R4/R4、R4/T4 配对、四个 plain／marked completion writer 控制和两个原 QA 签字人控制。同步使用实际两连接与不同 PID，第一读完成两个 collaboration／session SQL 后在 `ListStageExecutionIntents` 持锁；未模拟 SQL 返回。Go 5 分钟期限、第一 reader 5 秒界限沿用 R17；原浏览器 480 秒／20 秒保持。

独立测试边界收据 `check/standing-read-regression-boundary-review-r01.json`，SHA256 `d8ce725de992c410a131f4499fdf869a63427cb575c114e866ade37d525ab473`。

DB 准备 r01 在任何 CREATE DATABASE 前因 SQL NULL 空输出解释错误退出，未创建克隆、未跑 Go；原脚本与只读 donor 证据保留。r02 将空值明确为 JSON `[]`，三个新空克隆的 donor／ledger／index／全表计数与可信 HOME／TempDir 已绑定。新 recorder 只兼容有效 Go mixed build／空 ImportPath 事件；重解析旧 raw 后 terminal set 相等，未重跑旧 Go。独立输入收据 SHA256 `ce182d8f7002930a9759e5095b545e5f3e6586dbcc940fccb93e9de9265ff699`。

| 实际轮次 | 结果 | 证据 |
| --- | --- | --- |
| 唯一 RED | 3／11／8 全执行、1 叶 FAIL＋7 PASS、零 skip；合法第二 R4 读取遇到 `issue_collaboration` 的 `55P03` | `implement/database-r02/red-r01/result.json` SHA256 `d50a15eeb83a5ceebb05f19d08f5019925286c57a14a6b08816b91ce653014fe` |
| 首 FIX 静态边界 | 仅 completion 与 execution QA chain 两个生产文件；原 writer 两个 UPDATE NOWAIT 字节相同、raw writer 恒 false，即便 marked 输入；private reader 在该链选择共享锁 | manifest SHA256 `129e1c52d7a903ae8ce438cefc0d98cabee49fb1261ed8196e9e3ce0fafd9b8f`；独立边界 SHA256 `f1937cbb781d610999ce3c78d56e8abb0168db336b927400223b2169e4bad54f` |
| 首 GREEN | **仍 FAIL**，3／11／8 全执行、同一 R4 叶 `55P03` 已前进到 `collaboration_run`；其余 7 叶 PASS；零 source／DB drift、after 连接与 fixture 表归零 | `implement/database-r02/green-r01/result.json` SHA256 `41bfd193b200da1c7e31e8607d069f157c4f640cd91fc5c0f72bde43abd8f6cc`；独立失败收据 SHA256 `2f05b1ab686405d660d6d629727c9d3d04e6a269a696dc2361d23298d1e114a7` |

RED 独立 raw 复核收据 SHA256 `c66b0b8e02c14ed9c34fd1b2b954e8f57cbc3487fdc1248b6aaf951b314912c4`。首两锁的静态接受不能当作整个修复闭合；race、vet 与新产物构建尚未执行。

## 当前 run 边界与限定接续

首 GREEN raw 已证明 `collaboration_run` 的 `55P03`，当前唯一可达排他源路径为：

```text
StageCodeEvidence → stageExecutionReviewState
→ stageExecutionApprovalChain(release → qa)
→ stageQAReportAs(lock=true):56
→ LockCollaborationStageTestRun → QueryRow.Scan
→ collaboration_run WHERE id=$1 FOR UPDATE NOWAIT
```

该 SQL 源码 SHA256 `1801c9fc7bf4c6cf8394807cc496600fa9c290ace93715ae065c324b521d9444`。截至此检查点，这是 source 关联，尚非 driver SQL 采集；不可混写为已捕获 driver SQL。边界记录 `implement/green-failure-run-fence-boundary-r01.json`，SHA256 `86fc0029fe86f15409f76ca6786c1127bab17e313fb7e3cfdb7bd42d4ac4c9b9`。

ROOT 已授权现有 implement 代理先做一次只覆盖失败 R4/r4 的私有被动诊断，采首 PgError 的 QueryRow.Scan／Query／Exec、SQL／hash／PID／frames，不记参数或凭据，不改规范 215 行、断言、期限。诊断验收增量 0；新输出不覆盖首 GREEN。

随后在新的 `fix-source-r02` 内限定补齐同一私有读取链：允许 `collaboration_stage_qa.go`、`collaboration_stage_execution_approvals.go`，保留首 FIX `delivery_completion_collaboration.go` 字节。原 `stageQAReportAs(lock=true)` writer 恒 false，即便 marked 输入仍取原排他锁；只在原 R4 execution QA 链选择私有 read entry。

共享 run 栅栏优先采用与原 id-only writer 谓词一致的 `SELECT id FROM collaboration_run WHERE id=$1 FOR SHARE NOWAIT`，确认一行后复用原 `GetCollaborationRecordedRun`，保持原适用性、主体、source 与日志检查。`LockKnowledgeRunSources` 带 session JOIN／issue 额外谓词，不能在未证明等价时引入额外读取前提；也不能用裸 `lock=false` 无锁读取求通过。预期不需要 SQLC、DDL、迁移或新依赖。

新增独立 run 控制文件可覆盖：plain／marked 原 QA writer 对独立 Share 栅栏仍 `55P03`；writer-first UPDATE 使真实 R4 拒绝且结果为空；reader-held 真实 R4 阻止原 run writer，释放后同 writer 成功。`recording_incomplete` 未被当前 QA applicability／proven-ended 判断读取，原建议的“改此字段后 release 必须失效”断言已撤销，不为该虚构断言新增产品行为或修改原断言。

新 GREEN／race 仅执行新增 run 控制及因当前 run 改动受影响的 Standing／RetainedIssuer 和两项执行链控制；首 FIX 不变的四项 completion writer 控制以已接受 raw／source 绑定复用。准确分母待新 manifest，不进行动态 Go discovery，不以旧候选失败结果冒充 GREEN。

## 保留的运行资源与新入口准备

运行库仍为 `multica_pay241_20261116_s2_runtime_r11`，ROOT API／bridge 已停止；其 9 条历史队列、6 个成功 tar、`unknown` 发布意图、门禁布尔与 epoch 16 保留。peer Web 39744／renderer 39748 保留，未信号。PAY 至多一个 Go DB 套件、知识至多一个，全局至多两个；独立检查始终只读文件、零 Go／DB／API／浏览器。

R18 已准备八个全新空 generation，所有旧 sealed／unused 都保留且不复用；新 runtime 环境仍明确待 FIX／产物／入口，不是许可。准备检查 SHA256 `1093f51989523eba1788c0f9878348f478ff2d0cb45363481bcaf28807cb7fc0`。

测试依赖元数据证明 daemon test 含 service，migrate 不含；若最终只改 service，构建四项 `server`／`multica`／`stage-router.test`／`host-daemon.test`，复用 R17 migrate 与未变 Provider／Web／renderer。至此未实际构建，若最终 generated／依赖有变，按实际差分重定，不预先扩大构建。

没有本会话 Git commit、新依赖、真实模型、资金或生产部署。接续使用现有两名 Trellis 原生代理，不新建任务、不重载全部历史。
