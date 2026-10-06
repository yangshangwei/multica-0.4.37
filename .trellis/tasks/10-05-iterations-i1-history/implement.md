# 承诺、范围事件与历史统计实施清单

- 启动门槛：FG。前置整个任务：无；按里程碑核对。
- 读取父 design/api-contract/test-spec 与本任务 manifests。
- 先写对应失败测试，再按父 implement.md 所有权执行；不得跨写其他任务。
- 运行父 test-spec 对应 canonical 检查及受影响回归。
- 在本目录 verification.md 记录 commit/命令/结果/限制，满足 HG 后交父集成。
- FG 已通过；本轮已实现持久事实投影、图表、原始承诺及冻结 payload，本地验证见 verification.md，待父任务整合验收。recorder/writer 接线归 foundation。
- HG 用真实 DB fixture 验证冻结 payload，实际 end/handoff 后不漂移属于 CG/VG，不能反向等待 closure 完成才验收 HG。

## 2026-10-06 HG execution plan

1. Add real database history fixtures before implementation: immutable original
   commitment, sequence-delimited start, A–J statistics, current participation,
   display capture and stored snapshots. Reuse canonical statistics/calendar.
2. Add tenant-scoped historical reference capture and the `OriginalFacts`
   contract used by lifecycle start. Preserve identifiers and display names at
   capture time; lock mutable references with NOWAIT in writer transactions.
3. Project the complete persisted event stream after its start marker. Read
   closed iterations solely from validated stored snapshots; no live fallback.
4. Build immutable snapshot payloads from frozen originals, resolved current
   scope, events and canonical statistics. Closure owns actual persistence.
5. Add protected coherent detail/issues/events reads, stable scoped keyset
   pagination, stale-cursor and malformed-payload tests. Coordinate SQL/router
   changes with the integration owner, leaving the release flag disabled.
6. Run isolated database tests, domain regressions, Go build/vet and diff checks;
   record red/green evidence and remaining CG/VG limits in verification.md.

## Integrated acceptance

2026-10-06: HG passed. Commits `1247d2728` and `4c9be7652`; see
[parent integrated evidence](../10-05-iterations-i1/lg-hg-verification.md).
Release remains off; CG is next and foundation remains open through FCG.
