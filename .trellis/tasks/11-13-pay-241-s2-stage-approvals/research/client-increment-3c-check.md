# Client increment 3c — independent check

Reviewer: `/root/s2_client_check_resume`, 2026-11-14.

**APPROVE after one local schema correction.** No open findings remain in this bounded client review. This accepts the client increment for integration; server 2c/2d, actual API/CLI/Git execution, production Web/Desktop and complete A01–A12 remain separate gates.

## Findings (fixed)

### C3C-C01 / P2 — a conflicting terminal fact could be hidden behind the first match

Files: `packages/core/collaboration/stage-schema.ts`, `packages/core/collaboration/stage-execution.test.ts`.

The R4 schema validated each operation against the first fact matching its fact ID or intent ID. Fact IDs were unique, but two different fact IDs could reuse an intent or approval. A correct first row therefore masked a contradictory second row. A single fact reusing the visible operation's approval with a different intent also escaped the correlation check. These responses violate the frozen one-intent-per-approval and one-terminal-fact-per-intent contract.

Two independent counterexamples failed against the submitted source: an extra fact sharing an intent but naming another approval, and an extra fact sharing an approval but naming another intent. All 33 existing tests in that file passed. A subsequent third counterexample reproduced the same-approval/different-intent single-fact case after the first uniqueness repair.

The correction requires unique intent and approval IDs across 2c facts, and includes approval identity when checking overlapping operation/fact projections. It rejects the whole contradictory response rather than dropping a row or choosing a winner. Older responses without the 2c array group retain their unknown/unavailable compatibility behavior. The change adds no public field, route or execution authority.

Only these two live files changed during review. The parent authorized the bounded correction; original implementation snapshots and every failed run remain unchanged. The first red source was reconstructed and matched the hashes captured before the repair.

## Findings (not fixed)

None. The unfinished server DTO draft's null execution arrays are a server 2c implementation concern already reported by the server checker, not an alternative to the frozen contract and not an unresolved client defect in this review.

## Review evidence

- **Source reconstruction:** 496 files from the 3b2 implementation freeze, its 34-file final B01 overlay, and the 16-file 3c overlay. The root separately authorized 2,026 missing current dependency/configuration files whose bytes still matched `resume2-workspace-manifest.json`. The resulting 2,526-file copy identifies those dependencies as a current supplement, never as historical 3b2 verification. Workspace package links resolve inside that copy; installed third-party package locations and metadata are separately bound.
- **Implementation evidence:** all 14 submitted raw logs and direct exit files match their declared hashes and statuses, including first red and intermediate fixture/implementation failures. The two frozen contracts match the 3c report.
- **Parsing:** arrays are bounded, the additive group is complete or omitted, native operation IDs use the 256-byte UTF-8 boundary rather than UUID validation, source-denied projections require empty private groups, and overlapping execution identities now remain one-to-one. Unknown and malformed evidence cannot become an approval signal.
- **Release:** the form selects an R4 target; the selected valid QA approval determines the T4 report. The report must match the candidate, current summary, source revision and the same non-null artifact. Its extra `evidenceReport` value stays inside client preparation and never enters HTTP JSON.
- **Rollback:** approval is verification-stage only, with the original successful deployment, its immediately previous artifact, its release approval and an exact current anomalous metric. Ambiguity, expiry, target/deployment drift and missing source support block it. Reason-only return remains available.
- **Execution versus approval:** intent history prevents revocation across pending/unknown/not-started/failed states. A full 50-row page does not prove an old approval remains unconsumed. A9 follows the selected current valid release approval to one matching successful intent/fact, never merely a success label or the newest unrelated fact.
- **Privacy and recovery:** release's T4 read uses the existing source-generation fence. Its artifact-specific denial clears cached private evidence even while R1 child membership remains readable. Old code-review/QA request bytes and original GET404 recovery are exercised; no automatic repost, new key or public execution/metric/simulation control is introduced.

## Verification

All commands below ran against the reconstructed private source and private HOME/TMP. No API, database, provider, model account, browser or Desktop process was launched.

| Check | Result |
| --- | --- |
| Initial independent Core regression | 19 files, 400 passed, 0 failed/skipped |
| Initial independent Views regression | 4 files, 54 passed, 0 failed/skipped |
| First independent counterexamples | 2 failed / 33 passed; preserved |
| Additional approval-correlation counterexample | 1 failed / 35 passed; preserved |
| Final Core regression after correction | 19 files, 403 passed, 0 failed/skipped |
| Final Views regression after correction | 4 files, 54 passed, 0 failed/skipped |
| Final Core and Views typecheck | Both passed |
| Final changed-file Core and Views lint | Both passed, zero findings |
| Final source/link/live-file audit and scoped whitespace | Passed; only the two declared source files changed |

The first Core lint invocation failed before linting because the private runner looked for ESLint in Core's local dependencies; Core resolves the existing root tool. The corrected invocation uses the already-bound root ESLint entry point and passes. This is retained as a checker setup failure, not a product lint defect. Repeated runs are not added together as acceptance cases.

Exact source, dependency, command, exit and log hashes are in the JSON companion and `/Users/artisan/.multica/acceptance/pay241-20261114-resume/s2-client3c-check/`. The final 16-file increment is frozen under `checked-source/`; only two files differ from the implementer's frozen overlay.

## Acceptance limits

These are synthetic unit and static checks of the shared client. They do not verify real deployment, rollback, metrics, payment, provider truth, installed Desktop Main or browser interaction against the completed server. M1/activity client 2d remains future work. No spec, server, E2E, platform routing, dependency, database or Git-index change was made by this reviewer. Parent acceptance and all real external NOT_RUN items remain unchanged.
