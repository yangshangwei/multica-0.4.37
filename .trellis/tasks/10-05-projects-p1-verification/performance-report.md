# P1 API 与活跃分页性能验收

状态：完整采样已结束；性能未通过：静态延迟通过，但1/s与10/s活跃分页未达到可用性门槛。生成时间 2026-10-05T10:47:38.451Z。

## 预算与环境

测量前固定 HTTP P95≤2,000ms、错误0、同版本计数/页一致100%；双客户端收敛≤5,000ms由 browser lane 独立执行。空闲第二页成功率至少95%；持续变更低于50%明确标为不可用，需复审 ADR。未按结果移动阈值。

- 主机 Apple M4，10 CPU、16 GiB，macOS 25.2 arm64；Go 1.27.0；容器 PostgreSQL 17.11 aarch64，pg_stat_statements未启用。
- 实现前基线是 `git archive 0af59c5d5 server` 后独立编译、独立数据库与19072端口，绝非新接口冒充旧基线。
- 新接口是19071运行中的 `bb1c20dcb` 二进制原样复制到独占19073，源运行记录source_id为 `5dd65e9bbe9887d792e93ef488cde8c4299657594b6b6177cbce22650152f197`；二进制SHA256与各次 `/health` PID/commit/started_at均保存在结果中。
- 两库各500项目：目标项目10,000正式任务，另100 pending+100 rejected+100 duplicate；其余499项目各10正式任务，总计15,290任务。真实HTTP登录/建工作空间；批量fixture用已有TestApiClient与参数化SQL。没有执行任务。
- 同一认证成员发压测；并发1/10分别200请求，每接口20次预热。冷样本指5次新API进程的首个业务请求；数据库和OS缓存仍热，不称整机冷启动。
- 静态窗口有其他团队构建/测试共享本机资源；活跃0/s窗口还包含独立查询计数与全量页核对的短时读负载。全部请求原始耗时保留，未剔除离群点。

## 静态 HTTP 延迟

单位ms。旧 API只有项目详情及列表，新增overview/risk没有伪造旧值。

| 接口 | 并发 | 旧P50 | 旧P95 | 新P50 | 新P95 | 新错误 | ≤2s |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| legacy_detail | 1 | 6.1 | 7.7 | 9.8 | 12.1 | 0 | 通过 |
| legacy_detail | 10 | 8.1 | 12.2 | 38.5 | 46.3 | 0 | 通过 |
| legacy_list_500 | 1 | 11.1 | 14.7 | 19.5 | 30.6 | 0 | 通过 |
| legacy_list_500 | 10 | 20.8 | 29.4 | 91.8 | 121.9 | 0 | 通过 |
| overview | 1 | — | — | 34.3 | 39.2 | 0 | 通过 |
| overview | 10 | — | — | 315.0 | 325.0 | 0 | 通过 |
| risk_blocked | 1 | — | — | 35.0 | 39.2 | 0 | 通过 |
| risk_blocked | 10 | — | — | 323.0 | 414.2 | 0 | 通过 |
| risk_overdue | 1 | — | — | 35.3 | 40.2 | 0 | 通过 |
| risk_overdue | 10 | — | — | 325.2 | 363.1 | 0 | 通过 |
| risk_unassigned | 1 | — | — | 35.3 | 42.4 | 0 | 通过 |
| risk_unassigned | 10 | — | — | 324.1 | 364.5 | 0 | 通过 |
| risk_in_review | 1 | — | — | 35.5 | 42.0 | 0 | 通过 |
| risk_in_review | 10 | — | — | 322.9 | 347.9 | 0 | 通过 |

新旧项目API的绝对延迟通过2秒门槛，但并发10的旧详情P95约12.2→46.3ms、500项目列表约29.4→121.9ms，属于明显相对回归。新增分项及RR授权事务增大SQL数量，不能只报告新增接口很快。

## 实际 SQL 与资源

通过仅对独占查询实例的连接设置 `log_min_duration_statement=0`，按私库backend PID和请求时间窗口读取PostgreSQL日志中的execute/statement，排除Parse/Bind计时项。该日志开关未用于正式延迟窗口，也没有修改共享PostgreSQL全局配置。每endpoint额外发5次真实HTTP请求，保留SQL名称直方图和docker命令。

- 旧详情／列表：各5条SQL/request。
- 新详情／列表：各13条statement/request（包括BEGIN、SET TRANSACTION、COMMIT）。
- 新overview：20条statement/request；四风险页：22条。blocked观测窗口额外出现1次ping和4次后台webhook claim，原始总数115/5=23，明确扣除这5条后台语句后为22，未将后台负载伪装为请求查询。
- 所有页的查询数随本次接口形状固定，不随项目数产生N+1。EXPLAIN ANALYZE/BUFFERS/JSON包含正式输入、旧单项目／500项目统计、新500项目分类统计和50项页面读取，保存完整plan。

CPU/RSS样本：旧峰值 117.3% / 67.1 MiB；新峰值 115.5% / 80.2 MiB。CPU按ps进程口径可大于100%；新采样覆盖后半静态窗口及活跃窗口，旧在独立资源重复运行中采样。

## 同版本完整集合核对

独立SQL与真实API逐页遍历核对，非仅检查第一页或两个同源计数字段：blocked 1,000项/10页、overdue 3,333项/34页、unassigned 2,000项/20页、in_review 1,000项/10页，按100页大小；全部ID、排序、总数和snapshot_version一致，agreement=100%。执行在0/s稳定窗口，保存实际所有ID。

## 活跃分页（每种负载至少10分钟）

每秒翻一页；后台每秒0/1/10次真实DB变化，轮换截止日期、指派和准入状态，限定自己的工作空间/任务。每阶段恢复相同初始fixture。

| 变更/s | 时长s | 页请求 | P95ms | 错误 | 刷新比例 | 第二页成功/尝试 | 成功率 | 最长连续重置 |
| ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| 0 | 601.0 | 601 | 96.8 | 0 | 0.0% | 30/30 | 100.0% | 0 |
| 1 | 600.0 | 600 | 90.7 | 0 | 60.3% | 155/362 | 42.8% | 32 |
| 10 | 600.0 | 600 | 88.9 | 0 | 99.8% | 0/599 | 0.0% | 599 |

到第二页等待时长从首次下一页请求开始，包含重置后的定时重试；与单个成功HTTP耗时分开。未在观察窗内到达的事件为右删失，不伪造P95：

| 变更/s | 到达事件 | 等待P50ms | 等待P95ms | 最长等待ms | 结束时仍未到达的已观察等待ms |
| ---: | ---: | ---: | ---: | ---: | ---: |
| 0 | 30 | 88.3 | 97.2 | 136.7 | — |
| 1 | 155 | 86.5 | 7078.8 | 31091.1 | 5080.1 |
| 10 | 0 | — | — | — | 598085.7 |

完整采样证明持续变更会反复回到第一页：全输入digest改变就重置，后续页面难以到达。HTTP成功和计数正确不等于分页可用；两档活跃负载均低于预算，必须将其作为未完成风险交由父任务复审ADR-01，不能以Playwright脚本执行成功替代产品通过。

## 可复现命令与证据

```sh
# repository root; the env file contains only this lane's isolated database/port
set -a
source .omx/projects-p1-performance/baseline.env
set +a
PERF_LABEL=baseline PERF_MODE=static pnpm exec playwright test --config=.trellis/tasks/10-05-projects-p1-verification/performance/playwright.config.ts
set -a
source .omx/projects-p1-performance/current.env
set +a
PERF_LABEL=current PERF_MODE=full pnpm exec playwright test --config=.trellis/tasks/10-05-projects-p1-verification/performance/playwright.config.ts
# Dedicated SQL-logging observer (same private database, a separate owned API port)
PORT=19074 NEXT_PUBLIC_API_URL=http://localhost:19074 PERF_LABEL=current PERF_MODE=queries pnpm exec playwright test --config=.trellis/tasks/10-05-projects-p1-verification/performance/playwright.config.ts
node .trellis/tasks/10-05-projects-p1-verification/performance/full-scope-audit.mjs
node .trellis/tasks/10-05-projects-p1-verification/performance/report.mjs
```

可复用脚本与预算位于 `performance/`，原始请求/活跃页/CPU样本、冷启动PID、SQL直方图、EXPLAIN和汇总位于 [performance/results/summary.json](performance/results/summary.json)。认证token只保留在ignored的私有session文件，不复制到报告。大型源码archive和测试二进制留在ignored的 `.omx/projects-p1-performance/`；并未改动原main或19071服务。

跨端30次收敛、Web最终版本和新的健康实现如果发生变化，须由父任务追加关联证据；本报告当前只绑定已记录的二进制与源码。
