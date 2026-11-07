# No-statistics reprepare discriminant

Active task: .trellis/tasks/11-02-branch-functional-regression.

The completed run1 proves cache warming alone leaves the repeated scan/sort, and ANALYZE updates both statistics and cached-plan validity. It does not separate improved planner inputs from a fresh plan using unchanged no-statistics inputs. This is a diagnostic identifiability gap, not an authorization for SQL/index/product changes.

Hypothesis before execution: if no-statistics deallocation/reprepare on the dedicated backend retains the repeated event scan/sort, stale cached-plan identity alone is insufficient in this fixture. If fresh preparation chooses an ordered index path without statistics, document that result instead. Either outcome preserves original2171msFAIL and cannot reconstruct original tail samples.

Add one new numbered run2 on new database multica_func7af9_next_cached_diag_2. Preserve all run1 evidence. Use same reviewed new-file-only diagnostic and exact original command/SQL/argument sequence with production fences and full rollback. Execute original typed flow, no-stat cached replay, a no-stat reprepare control after pgx.Conn.DeallocateAll outside the transaction, then the documented task-only ANALYZE phase. Capture actual SendBatch plans/notices and prepared names/times/counter reset, table statistics and planner switches before/after reset. Do not force a generic/custom plan, change SQL/indexes, or modify frozen thresholds. Any fresh preparation/reset must stay on this new dedicated backend; no shared cluster setting or existing service change.

Rollback all measured business transactions, check logical fingerprints, retain physical/cache/WAL limitations, preserve source/frozen hashes, close own connections. Relevant checks: gofmt/compile/actual diagnostic once, JSON/SQL/args/counter/hash checks. Record results separately as research/perf-cached-reprepare-diagnosis-1.md and evidence/next-perf-cached-plan-2/. Independent review is separate from child self-verification and coordinator verification.
