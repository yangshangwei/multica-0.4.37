# R17 原四场景结果与后态（2026-11-16）

Active task: `.trellis/tasks/11-13-pay-241-s2-stage-approvals`

沿用 r15 最新交接和本会话 r16/r17 证据，不重新建立任务。本文记录实际检查点；S2、A01–A12 与父 24 例矩阵尚未接受。

外部证据根：

`/Volumes/artisan/.multica-acceptance/pay241-20261115-continuation/resume-r05/r17-stage-read-lock-fix-r01`

## 唯一一次实际运行

独立入口收据 `check/original-four-entry-clearance-r01.json` 已接受，阻挡 0，SHA256 `a8462d4dac05dd5888de7d9950429318d2ed7bab8c6b5aa4f84900e583799fcb`。ROOT 校验 permit、before、live binding、runner、config 后于 `2026-11-16T07:19:02.536Z` 执行一次原四场景，`07:20:50.188Z` 结束，exit 1。

原测试源码、执行顺序、四例分母、480 秒场景期限、20 秒操作期限、workers 1、retries 0、maxFailures 1 均保持。运行使用全新八个 staging generation；没有同键自动重跑，没有复用封存目录。

| 原场景 | 本轮结果 | 实际边界 |
| --- | --- | --- |
| `collaboration-stage-approvals.spec.ts:27` | PASS | 完整 QA 及共享 Web／Desktop 子集，三个受控 Host run，清理无错误 |
| `collaboration-stage-execution.spec.ts:233` | FAIL | 两个真实本地 tar 成功；已走过代码评审、QA、发布批准及原键恢复；在第 278 行 reload 后找不到 `Execution requests` region |
| `collaboration-stage-execution.spec.ts:349` | NOT_RUN | 首失败后停止 |
| `collaboration-stage-execution.spec.ts:398` | NOT_RUN | 首失败后停止 |

最新结果仍为 **1 PASS、1 FAIL、2 NOT_RUN**。本轮结果不能转换为 S2 或全部 24 例通过。

## 新失败的最小证据边界

被动观察器完整保存 3,755 条记录、1,253 个完整 HTTP 正文，未触 4,096 条／单正文 1 MiB 上限。原日志按字节冻结到 `evidence/four-scenarios-read-diagnostics-r01.jsonl`，SHA256 `653678a587bc35591f87bc1d8b87e017f008efbcf4fb27bf3c682939cf520139`；派生摘要 SHA256 `982a680a19450adddafab6fd4d7a16ee9097c861ae523e952f8a1486e3f5f644`。

失败 case2 的 issue 为 `01a1e33a-4605-7c31-b190-37db22986a22`。六次 R4 请求在下列调用点得到首个 PgError：

```text
StageCodeEvidence → stageExecutionReviewState
→ stageExecutionApprovalChain(release → qa)
→ completionCollaborationAccess:53
→ SELECT issue_id FROM issue_collaboration WHERE issue_id=$1 FOR UPDATE NOWAIT
→ SQLSTATE 55P03 → ROLLBACK → HTTP 503
```

该 SQL 的 SHA256 为 `2069277eaf66ff534a1a5b24b6ffdf9bbb2b3ed99024b95e6ba13e1a220bf71b`。六个请求各有完整 82 字节失败正文：

```json
{"code":"collaboration_unavailable","error":"Collaboration data is unavailable."}
```

私有读取事务已经到达这个执行审批链 helper，但它仍无条件使用 writer 的 collaboration 与 session `FOR UPDATE NOWAIT`。页面按现有读取错误保护隐藏 region，是症状；不要改 UI 保护、权限、断言或超时。

独立边界检查收据 `check/original-four-failure-root-boundary-r01.json`，SHA256 `82d3224c33fd0aae24779430a4e7f57def1eb7b63688b6d205d91302ef1e2878`。现有证据不证明精确持锁者，不证明单事务自升级冲突，也不能把全轮 138 个 503 都归于该 helper。全轮首个 55P03 是已 PASS case1 的 `LockArtifactMembers`；全轮另有 120 个无 SQLSTATE 的 query error，按请求与时间窗口分开保留。

普通源成员 M 的 T4 本轮返回原断言要求的 404，QA／发布者存在成功 T4 和决定 200 的实际进展，也另有无 SQLSTATE 的 T4 503；不能称 T4 全部修复。

## 后态与资源交接

只读 DB 后态 `runtime/after-four-database-r01.json`，SHA256 `50763682e5cea1c1c73b3234a1b1e0fb60637f58bfe155ea13cf8304c624f56a`：

- 15 个 Host run 全部 settled，未释放 reservation、准备租约、execution installation／binding 均 0。
- 6 次 artifact execution 完成；本轮新增 2 次，所有真实 tar 与 DB digest／size 相符。
- 门禁布尔恢复 `enabled=false / creation_enabled=true / host_execution_enabled=false`，epoch 从 12 单调增至 16。
- 9 条队列为 4 条既有 failed、4 条既有 deferred retry、1 条本轮 synthetic producer running，保留全部历史。不得删除、改状态或把 deferred／无 binding 称为永久不可执行。
- 1 个 `unknown` 发布意图及 1 个对应 unknown observation 按失败场景保留；不作为 Host 清理泄漏处理，不制造终结事实。

文件／来源后态 `runtime/after-four-files-history-r01.json`，SHA256 `c08374e261f3930583f8eac0791a7afcec01355086fac9f7313c0b9a41b501c1`：94 个原 binding 输入及环境／before 再校验，server 5,043 文件、ROOT 1,458 个生产依赖文件、两套 frontend 各 3,358 文件／1,961 链接均无漂移；历史 inode／mode／文件字节保留；本轮八代为 2 sealed、6 unused，均保留，后续仍必须新分配八代。两平台 fixture 的三次 Host 清理收据均 exit 0、errors 空、active_tasks 0。

ROOT 仅 SIGTERM 自有 API 67700 与 bridge 67695，停机收据 `runtime/stop-owned-api-after-four-r01.json`，SHA256 `0ec0f15a631f976139a702ef2f53001ef9c14e0dd78577619baead117b737c75`。端口 64922 无监听；peer 拥有的 Web 39744／renderer 39748 仍在 64923／64924，未信号。

停机后 DB `runtime/database-after-owned-api-stop-r01.json`，SHA256 `b11e11e20af42e50fb4fe8642b309f063bd68fb0259bc0a18b07b44bfb47c0c0`：运行库 `multica_pay241_20261116_s2_runtime_r11` 其他连接 0，9 条队列、6 条产物、门禁 epoch 16 保留。

## 接续 R18 的限定工作

现有 Trellis implement 代理在新的外部证据目录 `r18-stage-execution-read-lock-fix-r01/implement/` 内负责真实两连接 RED／FIX／定向 race，不直接写 ROOT。新增回归必须触达已批准 QA／release／unknown intent 的完整 R4 投影分支，原 writer 与撤权控制保持。

只修这个私有读取调用到 collaboration／session 两锁的模式。所有原 `completionCollaborationAccess` writer 调用仍默认排他，包括直接传私有 marker 的 writer 调用；显式签字人、权限、RR、NOWAIT、拒绝投影、重试均不变。检查代理独立只读审核；ROOT 再整合与实际验收。

PAY API 已停；implement 获唯一 PAY DB 回归槽 1，知识槽至多 1，全局至多 2。知识协调线程 `01a1df7d-d12f-74b2-a959-72af2045a06e`、平台 peer `01a1e232-4890-7bb0-9671-23089bc4587b` 已收到停机／R18 状态。沿用 source freeze，不修改知识 SOURCE8877／TAG。

没有本会话 Git commit、新依赖、真实模型、资金或生产部署。未更新 A01–A12／父矩阵，未归档任务；新一轮原四必须经新 generation／来源／实际入口绑定，不能复用本轮许可。
