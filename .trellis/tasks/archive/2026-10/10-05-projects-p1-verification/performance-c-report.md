# ADR-05 C：持续分页性能复测

状态：本次API与活跃分页预算通过。2026-10-05T12:21:53.661Z。

## 来源与固定方法

后端提交 `1bc58911981ec554673a6254c2af861d918215c9`，binary SHA256 `7c03575c8d335c86124f29d34a4c0978aeb6eeb76ff085ec66f922660f41d875`；源文件构建前后hash一致，完整provenance及各次health PID/commit保存在results-c。独立19075端口、全新私库，未覆盖原A实验或19071。

沿用测量前固定预算：HTTP P95≤2秒、错误0、当前页/总数同版本一致100%；空闲第二页成功≥95%、活跃≥50%。非空suffix每一次必须严格前进，100%正确性单独判断，不能被可用性成功率平均掉。terminal空后缀单列。

每库500项目：目标10,000正式+300非正式，其余499项目各10正式。每接口20次预热，并发1/10各200请求；另5次新API进程首个概览请求。CPU/内存/PG/Go环境与旧报告相同；共享机器仍有其它团队构建/测试，保留全部离群值。

C每页保持当前RR正式集合、授权、全量统计与准确后缀；只保留last_id向前边界。refreshed=true允许继续，不代表回首页；不宣称跨版本旧页的拼接等于一个当前完整集合。

## 静态 HTTP

| 接口 | 并发 | 原A P95ms | C P50ms | C P95ms | 错误 | 2秒预算 |
| --- | ---: | ---: | ---: | ---: | ---: | --- |
| legacy_detail | 1 | 12.1 | 11.1 | 15.1 | 0 | 通过 |
| legacy_detail | 10 | 46.3 | 50.8 | 67.5 | 0 | 通过 |
| legacy_list_500 | 1 | 30.6 | 18.9 | 30.0 | 0 | 通过 |
| legacy_list_500 | 10 | 121.9 | 89.4 | 101.0 | 0 | 通过 |
| overview | 1 | 39.2 | 36.2 | 54.7 | 0 | 通过 |
| overview | 10 | 325.0 | 313.8 | 338.4 | 0 | 通过 |
| risk_blocked | 1 | 39.2 | 34.9 | 39.9 | 0 | 通过 |
| risk_blocked | 10 | 414.2 | 322.3 | 387.9 | 0 | 通过 |
| risk_overdue | 1 | 40.2 | 35.3 | 47.1 | 0 | 通过 |
| risk_overdue | 10 | 363.1 | 323.4 | 335.0 | 0 | 通过 |
| risk_unassigned | 1 | 42.4 | 34.7 | 39.5 | 0 | 通过 |
| risk_unassigned | 10 | 364.5 | 334.4 | 557.9 | 0 | 通过 |
| risk_in_review | 1 | 42.0 | 35.5 | 40.1 | 0 | 通过 |
| risk_in_review | 10 | 347.9 | 333.3 | 551.8 | 0 | 通过 |

## 各10分钟的活跃分页

| 变更/s | 时长s | 页请求 | HTTP P95ms | A第二页成功率 | C第二页成功/尝试 | C成功率 | 非空严格前进 | terminal | 实际回退 |
| ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| 0 | 600.0 | 600 | 91.9 | 100.0% | 30/30 | 100.0% | 570/570 | 0 | 0 |
| 1 | 601.0 | 601 | 86.9 | 42.8% | 31/31 | 100.0% | 570/570 | 0 | 0 |
| 10 | 600.0 | 600 | 92.2 | 0.0% | 31/31 | 100.0% | 569/569 | 0 | 0 |

第二页到达等待包含重置/重试，不将成功HTTP耗时冒充全部等待：

| 变更/s | A等待P95ms | C等待P95ms | C最大等待ms | C未完成等待ms |
| ---: | ---: | ---: | ---: | ---: |
| 0 | 97.2 | 91.0 | 91.9 | — |
| 1 | 7078.8 | 88.5 | 88.9 | — |
| 10 | — | 90.8 | 128.9 | — |

## 正确性、SQL与资源

独立SQL与真实API完整遍历四种风险，比较全部ID/排序/总数/版本，执行在无写入窗口；结果见current-full-scope-audit.json。动态窗口每个非空页校验所有ID严格大于请求anchor、页内排序且准入正式；总数与同响应overview一致。低ID再入险、anchor删除/移动/非正式、空suffix/positive total和版本绑定已由真实DB回归覆盖。

进程监测1801条，CPU峰值114.8%、RSS峰值82.1MiB。冷进程5条原始样本、精确EXPLAIN与独立SQL日志计数保留在results-c；SQL观测单独在19076实例启用连接级日志，不改变正式延迟测量。

## 复现与证据

```sh
set -a
source .omx/projects-p1-performance-c/current.env
set +a
PERF_LABEL=current PERF_MODE=full pnpm exec playwright test --config=.trellis/tasks/10-05-projects-p1-verification/performance/playwright.config.ts
node .trellis/tasks/10-05-projects-p1-verification/performance/report-c.mjs
PERF_RESULTS_DIR=.trellis/tasks/10-05-projects-p1-verification/performance/results-c node .trellis/tasks/10-05-projects-p1-verification/performance/assess.mjs
```

[C汇总与安全原始样本](performance/results-c/summary.json)。[原A失败报告](performance-report.md)和原始样本仍保留。token session、私库URL和二进制位于ignored目录，不进入报告。此报告只验证后端/API分页性能；最终Web/Desktop粘性提示及30事件双端≤5秒收敛由browser lane独立证明。
