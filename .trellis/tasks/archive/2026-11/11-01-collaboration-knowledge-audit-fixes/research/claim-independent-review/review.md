# Independent knowledge claim review

Verdict: **pass-with-notes**, no blocking product defect found. Reviewed branch tip `a4e444bdf90c84939189fc2981cdacab50c678de`, product `30edf3e36` (including H3/M3 `94e790d2d`), compared with `4e3b61500ed0afbc12cf4c79b1c6aea866778d0e`. Reviewer `/root/knowledge_web_review` is independent of claim implementers. Product, other evidence and all databases stayed untouched; only this review directory was written.

## Assessment

| Item | Independent finding |
| --- | --- |
| H1 / H1 follow-up | v1 built-in Claude invocation distinguishes explicit session selectors/argv breakers from otherwise unsupported ordinary options. Both daemon defaults and agent custom arguments are scanned after the same unshell-quoting used at launch; malformed custom JSON refuses. Unknown ordinary options do not become positive knowledge admission. Legacy invocation behavior and the retained namespace exposure guard remain intact. |
| M1 | Generic dependency errors during claim construction requeue the exact dispatched generation without adding a failure reason. Typed knowledge refusals and vanished rows stay terminal. Both single and batch finalization settle typed knowledge refusals; other finalization errors preserve the prior requeue path. Existing CAS and settlement-failure recovery protect a newer claim. No private context is added to the new content-free status logging. |
| H3 | Exact-session history uses descending `(created_at,id)` keyset pages of128. The runtime path first gets the complete relevant namespace keys, then reads delivery rows in bounded128-row batches and restores selected-key order. Missing selected rows or lost version metadata refuse. Existing exposure, descriptor and union-manifest checks are retained; history length alone no longer causes rejection. |
| M3 | All key-query UNION arms retain the negotiated-row predicate, with agent/runtime partial btree and JSON containment GIN migrations761–763. The up files each contain one concurrent index statement; no FK/cascade was added, cleanup map entries match, and down files retain the existing protected-history guard. |
| Managed settings | The existing owned settings pair is placed before both filtered configured argument lists, so a dangling value-taking option cannot consume the only managed flag. Competing settings are still removed; ordinary options are preserved. No new general parser, option admission or dependency was introduced. The leaf knowledge invocation allowlist remains unchanged. |

## Independent verification

- Guarded fake/argv/fingerprint targeted tests:64 named test/subtest passes,0 fail/skip.
- Same targeted coverage under race, count2:128 named passes,0 fail/skip.
- Current-source build and vet for service, handler, migrate and agent packages:exit0. These commands compile/check only; no TestMain or migration ran.
- Static migration shape and duplicate-prefix checks:pass; `git diff --check`:exit0.
- Recorded history/boundary/service and managed-argument logs match their declared SHA-256 values. Current managed-argument files match the recorded source hashes.

Exact source trees, file hashes, commands, exit codes and raw-log hashes are in `source-binding.json`, the per-command JSON files, `author-evidence-check.json` and `review.json`.

## Notes and remaining gates

**CV-1 — database evidence scope.** This reviewer did not rerun DB-backed suites, per assignment. The retained service run reports3281 named passes,18 explicit skips,0 failures and exit0 on its own new task database. That run's dirty-source manifest maps H3/M3 bytes to94e790d2d; it must not be relabeled as a clean final-candidate full run. Full handler/migration and merged-candidate checks remain coordinator work. The failed first10-minute suite and high-parallel fake timeouts remain preserved, not erased from the record.

**CV-2 — performance scope.** The retained EXPLAIN uses `enable_seqscan=off`. It demonstrates eligible negotiated-row partial indexes, but some UNION arms use a broad negotiated partial-index scan rather than a selective GIN path. This is not a default-planner or production-size benchmark. The approved complete key pass and O(negotiated namespace) claim cost stay explicit; no new exposure summary or cross-workspace owner policy was inferred.

**CV-3 — prior incident.** The already disclosed default-database migration incident0aedf934e remains a separate preserved incident. This reviewer ran no database command and attempted no recovery or rollback. It is not acceptance evidence.

There are no product edits or self-approved fixes from this reviewer. Branch-local pass-with-notes does not close final combined-source, real-model, installation or platform acceptance.
