# 后端续接验收结果

日期：2026-11-25。候选：`103e2bc329a9fb845d9f45c20b2a9bb3ebb54625`。执行环境为 macOS arm64、Go 1.27.0、Asia/Shanghai；UTC 另有独立记录。

**后端完整验收尚未通过。** 普通测试完整通过；race 完整执行后有一个场景在准备授权时失败；安全扫描退出 3，报告 14 项可达漏洞。后续选定诊断通过没有覆盖或改写原失败。产品源码、现有断言、重试次数及超时均未修改。

## 原始证据独立复核

本轮重新解析两种模式全部 172 个包的原始 Go JSON，核对实际 frozen inventory、包与命名终端、原始 stdout/stderr SHA256，并逐项比较原 `summary.json`。172 个包均完整；原始日志、静态检查日志和已完成诊断的摘要均匹配。当前 32,724 个 tracked 路径、字节与权限和额外六个已登记非可执行 support 文件均无漂移。

机器账本：`artifacts/qa/main-post-merge-e2e-20261124-080504/backend/continuation-offline-audit-01.json`。该账本包含所有包的原始文件路径/hash、全部失败和跳过身份、静态入口及漏洞 ID。它是证据复核，未再次执行任何产品测试。

| 范围 | 包 | 顶层 PASS / FAIL / SKIP | 命名 PASS / FAIL / SKIP | 叶 PASS / FAIL / SKIP | 结论 |
| --- | ---: | ---: | ---: | ---: | --- |
| 普通 Go | 86/86 | 11,743 / 0 / 23 | 25,580 / 0 / 24 | 23,265 / 0 / 24 | PASS；明确保留条件跳过 |
| Go race | 86/86 | 11,742 / 1 / 23 | 25,578 / 2 / 24 | 23,264 / 1 / 24 | FAIL；两个命名失败是同一父子场景 |
| UTC 关键回归 | 3/3 | 41 / 0 / 0 | 93 / 0 / 0 | 76 / 0 / 0 | PASS；独立新库及 UTC 进程 |

每种完整 Go 模式另有九个没有命名测试的编译包，不能计作业务用例。普通与 race 的每个包结束时记录数据库客户端连接为 0。UTC 回执及三个 raw stream 的原有记录位于 `qa/utc/`，具体选取见 `utc-regression-results.md`。

## 原始 race 失败

唯一失败场景是：

`TestStageCompletionIndependentConsumerAuthorityAndMinimalReceipt/wrong_consumer`

原始证据：`.omx/private/qa-work/main-post-merge-e2e-20261124-080504/backend/runs/race/986dfe9cbf73/test.jsonl`，2026-11-24 10:49:02 +08:00：

```text
collaboration_stage_completion_tx_test.go:1272: actual M1 authorization: delivery_completion_conflict
```

执行路径为 `stageCompletionM1ForLifecycle` → `stageCompletionM1Grant` → `CreateDeliveryCompletionAuthorizationInTx`。错误发生在 fixture 创建合法 M1 授权时，先于该用例切换为错误消费者，以及后续拒绝和副作用断言。原 run 中 `agent_actor`、`issuer_revoked`、`consumer_revoked` 三个相邻子用例通过。

现有两次全新数据库诊断 `backend-diagnostics/stage-normal-01` 与 `stage-race-01` 各为 1 顶层、5 命名、4 叶 PASS、退出 0、结束客户端 0，且保持原四个子用例和断言。它们证明该选定矩阵在独立执行时通过，不能替代整包原失败。

本轮恢复原 PostgreSQL 容器后，从保留日志取回精确时间窗口，找到了错误的实际来源：

```text
2026-11-24 02:49:02.224 UTC [16586] ERROR: could not obtain lock on relation "github_pull_request"
STATEMENT: LOCK TABLE github_pull_request, issue_pull_request, issue_vcs_pull_request, vcs_pull_request IN SHARE MODE NOWAIT
```

这与 Go 失败的 02:49:02.225 UTC 对应，将失败定位到 `delivery_evidence.go:132` 的非等待表级观察锁。`delivery_completion.go:88` 把该数据库锁拒绝映射为 `delivery_completion_conflict`。提取命令、raw SHA 和精确窗口保存在 `backend-diagnostics/stage-original-pg-window-01.{json,log}`，相邻窗口另外保留；仅做只读 Docker 日志检索，没有补写历史记录或修改数据库。

原 JSON 未出现 `WARNING: DATA RACE`。失败时只有该父测试与其子测试处于运行状态，其他并行测试尚在 paused 状态。保留日志没有竞争持锁进程身份，不能推定是 autovacuum、其他测试、应用进程或某个特定 SQL 的竞争；也没有证明错误消费者获得授权。已经确认的是合法授权准备被真实数据库表锁竞争拒绝，目标负向断言因而未执行。

**保留为未解决的 P1 验收失败。** 后续诊断/修复应捕获实际竞争持锁者，在新库复现 owning service 场景，再验证完整包；不能降低非等待授权守卫或延长超时取得绿灯。本验收任务没有修改该产品或 fixture。

## 静态检查及依赖安全

原入口已全部执行，回执位于私有 `backend/static-results.json` 与 `backend/logs/`。本轮核对所有日志摘要，没有重复运行已完成检查。

| 实际命令 | 退出码 | 结果 |
| --- | ---: | --- |
| 固定 Go `build ./...` | 0 | PASS |
| 固定 Go `vet -p 2 ./...` | 0 | PASS |
| `bash scripts/test-go.test.sh` | 0 | PASS |
| 固定 Go `tool govulncheck ./...` | 3 | FAIL，14 项可达漏洞 |

所有命令固定当前源码、工具、Git 外 HOME/TMP/GOTMPDIR 和本地模块缓存。Go 编译与 vet 构成此后端范围的类型及静态检查；没有声称存在另外的 Python 类型检查器。原全仓 TS/mobile lint/typecheck 归前端验收记录，本轮没有修改对应代码或重复计数。

| 扫描 ID | 工具报告的组件及版本 | 修复版本 |
| --- | --- | --- |
| GO-2026-6859 | `net/http@go1.27` | Go 1.27.4；已保存官方公告另列 1.26.11 |
| GO-2026-6809 | `crypto/x509@go1.27` | Go 1.27.3 |
| GO-2026-6617 | HTTP/2；stdlib 及 `golang.org/x/net@v0.57.0` 条目 | Go 1.27.2 / x/net v0.60.0 |
| GO-2026-6613 | `net/http@go1.27` | Go 1.27.2 |
| GO-2026-6612 | HTTP/2；stdlib 及 x/net v0.57.0 条目 | Go 1.27.2 / x/net v0.60.0 |
| GO-2026-6611 | HTTP/2；stdlib 及 x/net v0.57.0 条目 | Go 1.27.2 / x/net v0.60.0 |
| GO-2026-6610 | HTTP/2；stdlib 及 x/net v0.57.0 条目 | Go 1.27.2 / x/net v0.60.0 |
| GO-2026-6609 | `net/http@go1.27`，Range 头规模 | Go 1.27.2 |
| GO-2026-6608 | MIME / multipart / textproto | Go 1.27.2 |
| GO-2026-6607 | `crypto/tls@go1.27` | Go 1.27.2 |
| GO-2026-6605 | `net/http@go1.27`，CONNECT | Go 1.27.2 |
| GO-2026-6604 | `os@go1.27`，工具明确标记 Windows | Go 1.27.2 |
| GO-2026-6603 | HTTP/2；stdlib 及 x/net v0.57.0 条目 | Go 1.27.2 / x/net v0.60.0 |
| GO-2026-6278 | `github.com/gorilla/websocket@v1.5.3` | v1.5.4-0.20240701034025-d67f41855da4 |

该表保留工具输出，不是 14 个可利用性复现。工具尾部给出的汇总是“14 vulnerabilities from 1 module and the Go standard library”；一些条目同时展示 x/net 与标准库，而实际示例调用链包含标准库内部 HTTP/2，不能把重复展示另算漏洞。另有四个 require 级别漏洞未显示被代码调用，不计入上述 14 项。

本机固定 Go 1.27.0 与 CI 要求的最新 Go 1.26 补丁不同；本机扫描不能证明当前生产二进制或 CI 修复版状态。`server/go.mod` 仍声明 gorilla/websocket v1.5.3 和 x/net v0.57.0，第三方依赖风险不能全部归为本机编译器差异。应以支持的修复工具链重新构建/扫描，并在独立修复范围评估依赖升级。本任务没有升级工具链或依赖。

## 跳过处置与补充诊断

完整模式各自原样保留 24 个命名 SKIP。完整身份及原 skip 理由见机器账本；同一身份在 normal/race 执行两次不代表两个产品功能。

| 类别 | 原命名数 | 当前处置 |
| --- | ---: | --- |
| 明确 Linux namespace / `/proc` / signal-reap | 9 | 本机 macOS 未证明；需真实 Linux lane |
| Windows 行为 | 2 | 当前平台不可执行 |
| root-only sudo preflight 子用例 | 1 | 非 root 当前用户不可执行，不提升权限 |
| pg_bigm 真 opclass | 1 | 当前 PG 镜像无 pg_bigm；不加依赖 |
| Shared Redis limiter | 1 | 已用专用 Redis DB 补 normal/race，各 1 PASS |
| I1 P95 两项、Triage 规模 | 3 | 本轮已在新库实际补跑，全部 PASS，原阈值和样本不变 |
| skills.sh 公共 GitHub 集成 | 1 | 已补跑但 FAIL；实际 GitHub tree HTTP 403，原始日志保留 |
| 专用 stage harness / M1 browser fixture / wire 导出 | 3 | 必须由对应 browser/协议 lane 配置；普通 suite 不直接开启 |
| 明确未实现/空目录边界 | 3 | CLI 未知字段不保留、webhook token lookup 故障占位、catalog 无 HTTP recipe；不计 PASS |

`skills-public-01` 的实际错误为读取 `anthropics/skills` 仓库 tree 返回 HTTP 403；程序报告外部源暂时不可用及可能限流。未经响应限流头证据，不能断言一定是 GitHub rate limit。没有借用用户 GitHub token 或重试隐藏该失败。

## 本轮性能补测

执行既有已准备 `backend-diagnostics/run_selected.py iteration-perf-01` 和 `triage-perf-01`，每组使用单独全新 migrated clone，顺序执行、固定 Go 和 Agent CLI guard。原完整测试中的 SKIP 记录仍保留并指向这些补充结果。

| 实际用例 | 原始完整规模 | P95，毫秒 | 结果 |
| --- | --- | --- | --- |
| `TestIterationI1P95` | 1,000 任务；10 项目；5 次 warmup + 30 次测量；真实 start/close/carryover | detail 58.649；preview 119.215；closure 536.588 | PASS |
| `TestIterationI1DisableP95` | 51 个计划、1,530 成员；5 次 warmup + 30 次测量 | preview 242.006；apply 463.499 | PASS |
| `TestTriageQueueScaleBaseline` | 10,000 正式任务、2,000 pending；每种操作 10 次 | list 66.671；filter 59.430；action 161.068 | PASS |

三个用例的冻结门槛均为 2,000ms，两组命令退出 0、完整命名 inventory、结束数据库客户端 0。所有原始 samples、JSON、断言输出和退出码保留在 `qa/backend-diagnostics/{iteration-perf-01,triage-perf-01}/`；本轮机器报告重新验证了样本数量并从原样本计算 P95。

这是本机 Go handler + 独立 PostgreSQL 的性能证据，未控制整台主机的所有其他负载，也未清 OS/PG cache；不等于浏览器网络延迟或 L10 的 100 用户实时传播性能。

## 本轮资源恢复与终审

续接时 Docker CLI 与原 socket 存在，但 Docker daemon 未运行；初始容器检查在连接阶段拒绝，没有 SQL 或测试写入。主会话启动 Docker.app 后，本 lane 验证原 container ID/name/image/owner、实际 PG volume 与挂载均一致，只启动两个既有自有容器，未重跑 setup、重建数据库或调整 PG 参数。

PostgreSQL 的临时 loopback 端口由 49359 改为 **50172**，容器内 PID 为 2566；Redis 由 49362 改为 **50173**，PID 为 2625。旧 allocation 原字节保留为私有 `backend/allocation-before-continuation-20261125.json`，新 allocation 仅更新这两个易变字段；输入/守卫/source 不变。两版 allocation SHA、启动命令、完整 before/after 归属位于 `backend/continuation-recovery-{before,after}-01.json`。原运行仍引用其原端口和原 allocation digest。

新 `run_backend.py audit` 已完成并退出 0，逐包重算 normal/race，完整性均通过、race 业务结果仍失败。私有审计为 `backend/audit-e0f69472493f.json`；原审计器的退出 0 是审计完成，不是后端业务全绿。本轮追加整个自有 cluster 的只读连接检查：185 个非 template 数据库（含 postgres、原模板、normal/race、UTC、诊断和 seed 证据），除审计自身连接外客户端 **0**。最大连接 90，当前总 backend 6，其中客户端 1 为审计连接；未终止任何连接。

最终机器报告为 `artifacts/qa/main-post-merge-e2e-20261124-080504/backend/continuation-result-01.json`，包含精确漏洞清单、三项缺陷、原计数、全部新性能样本、当前资源归属及严格 `backend_release_green=false`。源文件仍是同一 32,724 路径清单。失败数据库与证据继续保留，两个自有容器留给主会话统一停止；本报告不宣称最终容器关闭完成。

## 变更和风险

本轮仅新增本研究报告、backend/diagnostics 的审计/恢复/性能记录，以及保留旧版后更新自有资源 allocation，复用了既有受审 source 验证器、Go JSON parser 和选定诊断入口；没有新增编排框架、产品变更或依赖。未解决风险包括完整 race 场景失败、安全扫描失败、外部 skills 导入失败和明确的平台/功能条件缺口。三个本机性能缺口及 live 连接审计已完成，最终容器停止由主会话负责。后端结果不能据此给出发版绿灯。
