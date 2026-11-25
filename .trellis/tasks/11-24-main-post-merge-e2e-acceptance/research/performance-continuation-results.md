# L10 百人协作性能验收

日期：2026-11-25。源码：`103e2bc329a9fb845d9f45c20b2a9bb3ebb54625`。运行：`main-post-merge-e2e-20261124-080504-performance-01`。

**既定性能门槛通过，三个原始阶段均退出 0。** 100 个真实合成账号通过公开注册和邀请 API 加入同一协作任务；传播采样包含完整 10,000 个正式观察结果；写回采样包含完整 100 次独立正式运行的两类指标。原固定分母、暖身排除、注册窗口、截止时间和阈值均未修改。结果仅证明本机单 API、确定性 Provider 的当前源码性能，不代表模型质量、生产多副本或完整发布验收。

API 日志仍保留九次 HTTP 503；规范采样器没有持久化写回阶段的每个 socket 身份基线。这两项证据限制分别说明如下，未被通过结论隐藏。

## 实际执行与结果

每个阶段只收集并运行一个 canonical 测试，`workers=1`、`retries=0`，没有跳过。顺序为 `prepare` → `propagation` → `writeback`，同一独立数据库和 API 持续服务三个阶段。

| 阶段 | 原始结果 | 实际耗时 | 独立核对 |
| --- | --- | ---: | --- |
| prepare | PASS，100 个不同账号 | 707.28 秒 | 公开注册；100 次注册尝试、100 个成功账号；保留每 300 秒最多 30 次的固定窗口 |
| propagation | PASS | 144.01 秒 | 100 个账号、100 个不同 socket；100 个正式事件 × 100 个观察者，全量 10,000 个样本收敛；另保留并排除 500 个暖身样本 |
| writeback | PASS | 234.98 秒 | 100 次正式运行，每次 business/finalization 各一条；另保留并排除一次暖身运行的两条指标 |

| 指标 | 固定分母 | P95 | 原始门槛 | 缺失 / 未运行 |
| --- | ---: | ---: | ---: | --- |
| 传播：全部观察者汇总 | 10,000 | 665.175 ms | ≤ 2,000 ms | 0 / 0 |
| 传播：每个事件最慢观察者 | 100 | 701.033 ms | ≤ 2,000 ms | 0 / 0 |
| Agent 业务结果可见 | 100 | 107.349 ms | ≤ 5,000 ms | 0 / 0 |
| Agent 最终状态收敛 | 100 | 409.195 ms | ≤ 5,000 ms | 0 / 0 |

独立 Python 审计器没有导入 canonical 统计代码，从原始行重建固定矩阵，将缺失、未运行和超截止时间样本保留为无穷大，用 `ceil(0.95*N)` 计算 P95，并对比全部 100 个观察者的分位值。结果与 canonical 汇总一致。算术审计本身不替代业务及资源检查。

对保留数据库的独立只读查询确认：暖身加正式共 **101 个真实 run、101 条 Agent 评论和对应 run/comment 链接**；每个 run 都是 `settled` / `success` / `host_exit_observed`，恰好一个授权且观察完成的 launch；command、runtime、Agent、issue 身份与原始记录一致；实际评论字节的 SHA256 与样本一致。读者与触发者、资源所有者分别独立，未用同一账号充当全部观察者。

写回 101 次运行都记录了开始和结束时 100 个打开的 socket，零 reconnect、零 disconnected observer，两个指标配对及运行身份完整。14 次 owner-scoped 心跳结果保留，最终 heartbeat 正常停止，daemon 退出 0，101 条 ledger 均为 `proof_acknowledged`。300 秒 fixture 生命周期、150 秒 freshness fence、10/30 秒采样 cutoff 保持原值。

## HTTP 错误与可复核边界

实际 API 日志中有 **8 次 `GET /api/collaboration/runtime/capabilities` 返回 503**，以及 **1 次 run 的 `stop-intent` GET 返回 503**。对应行号、请求身份、耗时和原始日志 SHA 保存在 `api-status-observations.json`。该 run 和其余 100 个 run 最终均经独立数据库检查确认成功结束；没有删除这些中间响应，也没有声称所有 HTTP 请求成功。仅凭状态码不能确定拒绝的具体原因。

日志另外记录 517 次 404，其中 516 次为评论读取，1 次为原创建键读取。采样器有“结果出现前必须不存在”的预检以及等待结果期间的读取；本报告保留实际路径分类，不把全部 404 逐条推断成某一种原因。

传播原始样本保存了每个观察者的 `observer_user_id` 和 `socket_id`，独立核对为 100 个身份且每个观察者在 105 轮内保持同一 socket。写回源码会在每次运行开始比较 `socket_id`、连接次数和 close 时间，断连或替换会使采样失败；但它的 `observerConnections` 数组只在内存中，原始报告保存的是 socket 数量和断连索引，没有序列化全部身份基线。这是现有采样器相对 spec 的证据持久化缺口。没有借用传播阶段的 socket ID 填补该缺口，也没有修改本次冻结的测试源码。

## 环境与清理

本次只启动实际 API。canonical HTTP/WS host helper 要求的 Web/renderer URL 变量明确指向同一已启动的 API origin；没有声称启动生产 Web、浏览器或 Electron。执行器通过当前 runtime 的独立准入、工具/构建/源码校验和数据库所有权检查，只分配一个新数据库，使用外部 0700 HOME/TMP/采样目录。密码等准备状态留在外部 `samples/private/`，没有复制进 QA。

运行前实际 cluster 客户端为 0，`max_connections=90`；API 使用产品默认 pool max 25 / min 5、单副本、无 Redis，进程 `GOMAXPROCS=2`。API descriptor limit 保持 unknown；sampler 自己观察到的限制没有替代 API 的限制。使用本机 macOS arm64 / Go 1.27.0 的确定性原生 host Provider、当前 CLI 和生产 poller 路径，测试 poller 的 50ms 间隔、容量 1 也明确记录，不等同生产 daemon 调度或商业模型。

本任务其他运行服务在测量期间暂停。stage-router 与 memory fake 的编译仅发生在注册等待窗口，均在 04:13:31 UTC 前结束；传播阶段于 04:15:25 UTC 开始。未宣称控制了整台机器的其他用户进程；原采样器保留各阶段系统负载和资源快照。

最终 API 端口关闭，实际全 cluster 客户端为 0，拥有的后代进程全部退出，无 fallback termination。gate 原始值 `enabled=false`、`creation_enabled=true`、`host_execution_enabled=false` 精确恢复，epoch 从 2 增至 4。全部 32,724 个 tracked 源文件、已登记六个支持文件、工具、构建和 fixture 绑定保持一致。原始数据库与证据保留；两个 runtime 锁清除后，服务执行权明确交回 runtime owner。

## 证据与改动范围

以下相对于 `artifacts/qa/main-post-merge-e2e-20261124-080504/performance/`：

- `performance-01/{inputs,allocation,completion,cleanup}.json`：独立受审输入、实际 PID/源码/数据库、三个阶段退出及终审。
- `performance-01/{prepare,propagation,writeback}/`：每阶段 collection、实际 Playwright JSON、完整命名 inventory、命令和原日志。
- `performance-01/samples/`：manifest、10,500 条传播样本、202 条写回样本、run/心跳记录、原始汇总及资源观察；不含私有账号密码文件。
- `performance-01/independent-arithmetic.json`、`independent-protocol.json`、`independent-database.json`：独立算术、协议身份与数据库对账。
- `performance-01/api-status-observations.json`：保留九次 503 及其他 4xx 的路径分类。
- `adapter-selftest-01.json`、`auditor-selfcheck-01.json`：执行器七项离线边界检查和独立审计器八项统计反例检查。

变化限于新的 task-private `run_performance.py`、`performance_selftest.py`、`audit_performance.py`、本研究报告及证据。复用了原 runtime 的准入、数据库、来源和命名对账函数，以及已复核的后代进程清理器；没有修改产品、canonical 测试、原 runtime 脚本或依赖。产品 lint/typecheck 沿用本 campaign 相同源码的既有结果，本次仅对新私有执行器运行离线检查。
