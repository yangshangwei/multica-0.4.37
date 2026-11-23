# Current candidate performance result

**2 PASS /1 FAIL /0 SKIP, command exit1.** Run once at candidate `1a32ca36c2c4f4455a01bdd39d0e0ab4db20cccc`, after root explicitly released the quiet measurement window. No race instrumentation, automatic retries, assertion edits, threshold changes or product changes. The precompiled handler binary hash remained `4d7cf20d5bec628e22adc313badf7064a91512419495f0938c1b0721a92ed22c`.

| Metric | Measured P95 ms | Frozen limit ms | Result |
| --- | ---: | ---: | --- |
| Iteration detail |78.280|2000|PASS|
| Iteration preview |396.391|2000|PASS|
| Iteration closure |**2155.025**|**2000**|**FAIL**|
| Disable preview |862.301|3000|PASS|
| Disable apply |2110.446|3000|PASS|
| Triage list |451.003|2000|PASS|
| Triage filter |459.553|2000|PASS|
| Triage action |110.369|2000|PASS|

`TestIterationI1P95` failed only its closure threshold. `TestIterationI1DisableP95` and `TestTriageQueueScaleBaseline` passed. Iteration and disable each retained5 warmups plus30 measured cycles, with no sample removal. The iteration fixture has1000 tasks; disable has51 plans/1530 memberships. Triage has10000 formal and2000 pending tasks,10 samples per operation. Exact samples and environment/SQL/lock metadata are in `evidence/performance-1/samples.json`; named results and command in `evidence/performance-1/run.json`.

The closure threshold was exceeded by155.025ms (7.75%). This preserves the pre-existing class of performance gate failure but does not establish a code root cause or prove comparability with the older candidate's2171ms result. These are local Go-handler/PostgreSQL measurements, not browser/network latency or a production SLO. Root's quiet-ready limits this task's overlapping load; the shared machine/cluster remains a measurement limitation.

Execution used the existing agent-CLI guard, exact3 named tests, count1, parallel1 and40-minute cap. GOMAXPROCS2 and cache locations were explicit. HOME/TMP resided under verified non-Git `/Volumes/artisan/e2e-private/core1a32-20261105/conditional/performance-1`; no ambient deployment/account environment was passed. The database was the exact root-allocated `multica_core1a32_20261104_performance`,752 migrations and no issues before the run.

The first completion audit observed1 other database connection; that observation is preserved. A subsequent read-only audit at2026-11-05T03:26:16.558Z observed0, and the owned runner had exited. No connection was terminated and the first transient connection's source was not inferred. Source/tool fingerprint and binary hash match; no new test rerun is planned. Overall acceptance remains failed on the original2000ms gate.
