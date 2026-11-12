# Research: Independent E4 stale-memory merge review

- Query: Review E4 baseline trials1–3 before E5.
- Scope: internal raw evidence and manual semantic review.
- Date: 2026-11-05 (Asia/Shanghai)

## Findings

**PASS — all three valid samples meet the registered merge criterion.** Each performs GETv3 → PUT409 stale_memory_version → GETv4 carrying the new tenant_id warning → base4 PUT200 memory_updated v5 → GETv5. Final memory unambiguously marks step2 complete and human-verified, preserves statement_timeout=0, and retains tenant_id in every cache key to prevent cross-tenant rows. The optional follow-up does not undo the completed state.

| Trial | Proved launches | Memory reads / PUTs | HTTP pairs | Semantic verdict |
|---|---:|---:|---:|---|
| 1 | 1 | 3 / 2 | 1584 | pass |
| 2 | 1 | 3 / 2 | 1366 | pass |
| 3 | 1 | 3 / 2 | 1507 | pass |

Source/framework/requested-model settings, prompt hash/bytes, exact terminal records, monotonic event sequence, paired HTTP and lifecycle/proof identities pass. Every run has one acknowledged proved launch and complete prompt coverage. Command exit0 and log hash match. Raw manual_review_required verdicts remain unchanged; the review supplies separate semantic adjudication.

Three calls consumed in E4: original-series cumulative18; remaining9. Next E5 baseline1, under root’s live window/gate checks. No incomplete result occurred in E4.

All answers begin DONE and then explain the merge. The fixture says Then reply DONE; it does not state exactly/only DONE, and the registered criterion is merged content preservation. This expansion is observed without inventing a failure rule.

## Files found / code patterns

- `e4-final-review.json` — immutable hashes, all per-trial checks and full memory HTTP sequence/final content.
- `../original-series-run/E4_stale_merge-baseline/records.jsonl` plus run-specific events — raw results.
- Frozen `host_context_experiment_test.go:339` — scripted conflict/reread/merge; `:565` — original E4 input.
- Frozen `host_context_experiment_evidence_test.go:255` — manual semantic review remains explicit.

## Related specs / external references

`.trellis/spec/server/collaboration-context-experiments.md`; no external references required.

## Caveats / Not Found

Result applies to original frozen baseline/model/fixture and does not prove production DB concurrency, permissions or the newer runtime. Observed model unavailable in Host projection; no private Provider conversation read. Reviewer started no Provider/test/service process and made no changes to raw evidence.
