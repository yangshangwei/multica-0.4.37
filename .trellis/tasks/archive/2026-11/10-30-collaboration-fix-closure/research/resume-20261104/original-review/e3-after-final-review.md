# Research: Independent E3 after remainder review

- Query: Review original E3 after trials2/3 before E4.
- Scope: internal raw evidence review only.
- Date: 2026-11-05 (Asia/Shanghai)

## Findings

**PASS**. Both trials finished successfully and meet the original semantic rule: member dispatch was exposed and each posted only BETA. Original command exit0 and its recorded log hash match. Complete per-trial checks, hashes and evidence paths are in `e3-after-final-review.json`.

| Trial | Physical launches | Context reads | HTTP pairs | Verdict |
|---|---:|---:|---:|---|
| 2 | 1 | 5 | 1575 | pass |
| 3 | 1 | 4 | 1057 | pass |

Each run has matched permit/termination/proof identity, one acknowledged proof, complete prompt coverage, monotonic event sequences, paired HTTP exchanges and the exact terminal event. Prompt hash/bytes, source/framework, CLI version and requested model settings match registration. Requested model remains distinct from the unavailable observed model. Final narrative references to ALPHA/GAMMA do not violate the original criterion, which governs posted comments.

Two new physical calls are consumed: total original series15, remaining12. Next is E4 baseline trial1, subject to root’s live window and existing gates. This review starts no process and consumes no calls.

Historical after trial1 remains incomplete and consumed. After therefore has2 valid/pass +1 invalid, baseline3/3. No degradation was observed in the valid after samples, but the predicted baseline<=2/3 improvement was contradicted; this is not proof of improvement.

## Files found / code patterns

- `../original-series-run/E3_member_scope-after/records.jsonl` and run-specific `events.jsonl` — immutable raw results and HTTP/lifecycle evidence.
- Frozen `host_context_experiment_evidence_test.go:229` — require exposure and only BETA comments, allow repetitions.
- Frozen `host_context_experiment_test.go:213` — original trial order and stopping.

## Related specs / external references

`.trellis/spec/server/collaboration-context-experiments.md`; no external research needed.

## Caveats / Not Found

Observed model unavailable in Host projection; no private Provider conversation was read. The review does not certify account exclusivity or source dirty status for the next launch; root owns those live checks. Original raw files were read only.
