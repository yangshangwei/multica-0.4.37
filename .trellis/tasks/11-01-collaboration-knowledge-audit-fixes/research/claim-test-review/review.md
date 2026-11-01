# Independent claim test-validity review

Verdict: **pass-with-notes** for tip `a4e444bdf90c84939189fc2981cdacab50c678de`, product `30edf3e36`, including H3/M3 `94e790d2d`, compared with `4e3b61500`. No blocking product defect or invalid positive result found. Reviewer `/root/claim_test_review` did not author these fixes and changed only this review directory. No DB, migration, installed Agent CLI, Provider or dependency operation ran.

## Test validity

| Area | Assessment |
| --- | --- |
| H3 growth | The fixture creates260 negotiated claimed-only rows, exceeding the old129-row cutoff. Long-session control verifies both original generation/native binding and subsequent preparation. The retained red run actually fails at the old knowledge-context refusal; the green run names the intended tests. |
| Deep exposure and corruption | The original exposure row is deliberately older than two pages. Withdrawal must retire its session. Corruption is tested after a valid resume control, so generic always-refuse behavior cannot satisfy the test. Runtime tests contrast benign claimed growth, retained private exposure and a damaged old row through an unrelated agent/scope. |
| Runtime key-to-row loss | A DBTX probe deletes the first selected key or clears both version/delivery immediately before the actual batch query. It asserts the probe ran and requires the precise refusal. The test transaction rolls back its injected mutation. This tests the intended selection/read gap rather than only decoder shape. |
| Namespace identity | Existing canonical matrices explicitly exercise same runtime, another runtime under the same daemon, another daemon under the same owner, and deleted old runtime/agent links with immutable identity retained. These complement the260-row growth tests rather than requiring all namespace variants to repeat that matrix. |
| M1 | Actual single/batch claim paths are driven under an injected history-query shutdown for both Claude and Codex; task queue state, absence of failure reason, no delivered descriptor and a later reclaim are checked. Finalization reaches the32-claim bound through32 real finalized/requeued claims and verifies the next refusal settles once, preserves claim count and does not poll-deliver again. The pure classification matrix owns wrapped/domain/missing/context/connection cases. |
| Managed settings | Independently reran the actual buildClaudeArgs construction tests:9 passes,0 fail/skip. An overlay restores settings-last ordering without editing product files:7 required-value subcases plus their parent fail, while the ordinary competing-settings control passes. This is a killed construction mutation, not a paid/installed Claude parser result. |
| M3 plan scope | The corrected plan probe requires at least one negotiated-history partial index and rejects task-table sequential scans with enable_seqscan=off. Its prior red probe fails against the old query/generated code. This demonstrates index eligibility; it does not prove selective GIN access in every UNION arm, default-planner behavior or production performance. |

## Evidence binding

Every inspected declared log SHA-256 matched the actual retained bytes. `H3-M3-source-and-sqlc.json` maps the reader/query/generated/migration/test bytes exactly to `94e790d2d`; the intentional later managed-argument commit changes the execution spec/operator guide and one service test comment. The old SQL and generated file are byte-identical at `6aed33aad` and the actual red source `3110295d0`, so the preserved source labels are explicit rather than being relabeled as a clean baseline run.

The full service rerun has3299 named run events:3281 pass,18 explicit skips,0 fail, and a terminal package pass at1265.972s. Its first600s package timeout remains present with no named failing assertion; the rerun uses a fresh recorded database and a40m package bound. The first high-parallel agent timing failures and the bounded1950-pass/4-skip rerun are both retained, with zero ambient Agent CLI guard violations. These are audited author/coordinator records; this reviewer did not rerun DB suites or the full package.

Exact commands, source hashes, counts and own raw-log hashes are in `review.json`, `evidence-audit.json`, `baseline-query-binding.json`, and `managed-*.json/jsonl` here.

## Nonblocking finding

**CT-1 — tie-boundary regression strength (low).** The growth helper uses `date_trunc('second',now())` for harmless claimed-only rows, while each decisive retained session/exposure/corrupt row is shifted one day older. Consequently, no named case fixes a decisive row below the first128 IDs in one fixed timestamp group. A timestamp-only cursor regression could skip harmless tied rows yet still reach the older decisive row and satisfy these assertions. Current production SQL correctly compares `(created_at,id)` and uses matching DESC ordering; this is a test gap, not a demonstrated product bug. No DB or dynamic cursor mutant ran for this observation.

Recommended follow-up: set one fixed timestamp for all negotiated rows, prove valid resume, corrupt a deterministic row outside the first128 ordered IDs, require `knowledge_context_changed`, then kill a cursor mutant that compares only timestamps. Keep the fixture/expectations in the existing canonical growth suite.

## Remaining boundary

The local claim manifests are empty and its design/implement files are absent; the approved artifacts were loaded from the existing knowledge integration task as directed. No duplicate task or plan was created. The retained EXPLAIN is not a benchmark. Complete handler/migration gates, any coverage follow-up and final combined-source checks remain coordinator work. Branch-local pass-with-notes does not close knowledge enablement, real-model or release acceptance.
