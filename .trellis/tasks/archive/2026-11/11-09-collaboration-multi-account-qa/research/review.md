# Independent QA review

Reviewed the three new test files, current product comment DTO, and original Playwright/evidence artifacts. No database operations were executed by this reviewer.

## Result

No remaining blocking source or evidence finding. The latest runs passed four scenarios with zero skipped tests: three-account closure, native squad, native runtime switch/Stop, and two-owner shared-context handoff.

The primary closure contains three distinct resource owners, spaces, Agents, runtimes and installations, plus an unrelated account. Five actual Provider processes ran in owner counts 1/2/2, each writing through the run-local CLI. Shared context progressed v1 through v6. Two result submissions retain exact returned/accepted decisions by the Product initiator; final status is done. All five local ledgers are sealed and proof_acknowledged with host_exit_observed; no page or cleanup errors, and the gate was restored.

The supplementary scenarios add eight actual Provider processes: squad four, switch two, memory handoff two. The switch also records a continuation proven never launched, which is not counted as a process.

The first primary attempt remains failed in run-01: its assertion expected author_id in the public Agent-comment DTO, which intentionally projects author=null. The corrected assertion verifies that DTO plus exact memory author and per-run comment/resource records. This is a test-harness defect, not a demonstrated product defect; the failed run and its first actual Provider execution remain retained separately.

## Findings resolved

The implementer corrected the initial cleanup flow that could label evidence passed before cleanup verification or lose failure evidence on gate/context cleanup exceptions. The final flow captures cleanup outcomes, verifies exact local proof ledgers, writes evidence and only then raises cleanup failures.

Final deltas add actual private ordinary-issue read denials for D/Q and a complete unchanged snapshot, plus exact original-key AI POST replay. No new API, product implementation or business-state SQL was introduced.

## Static checks

- Strict TypeScript noEmit for new spec and helper: pass.
- ESLint repository base with development dependencies explicitly allowed for E2E fixtures: pass. The unadjusted shared rule accepts *.spec.ts but omits e2e/fixtures/*.ts and falsely rejects their declared @playwright/test dev dependency.
- Go vet task Provider: pass.

Exact commands and source hashes are in independent-test-review.json.

## Evidence boundaries

Deterministic Provider execution proves scheduling, authority, input delivery, CLI writes and lifecycle observations. It does not prove commercial-model reasoning, actual payment correctness, a second physical machine or OS sandboxing. The final Product screenshot proves visible completion/acceptance; lower comments/history were still loading at capture, so history conclusions rely on exact API records and per-run evidence.
