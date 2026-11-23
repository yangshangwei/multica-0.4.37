# Final browser evidence audit

2026-11-23. Read-only audit by native `standards_review`: raw Playwright results, identity reconciliation, source manifests, run/configuration receipts and cleanup. No additional tests, browser sessions, product changes or resource mutations. The reviewer authored the two bounded product fixes earlier; the parent reviewed their implementations. This document independently checks the **evidence and scope of the completion claims**, not an independent code-review claim about those authored fixes.

## Verdict and exact counts

Recomputed the ledger from all **26 recorded accepted-attempt runs** using exact `(project, file, title)` identities. The raw outcomes exactly match `core/final-ledger.json`:

- **538 distinct identities: 464 latest admitted passes, 74 remaining skips, 0 unresolved failures.**
- All **22 original full-run failure identities**, across 18 files, now have passing terminal evidence.
- All **29 identities that failed in any retained accepted attempt** eventually passed, including the two added responsive rail red observations, the host-mode mismatch, conditional fixture failures and denied-notification loop.
- Zero duplicate identity within a run, automatic retry, unfinished result, or missing completion/source receipt in the 26-run ledger.
- Every remaining skipped identity has an explicit skip annotation. There are 74 cases in 33 files; they were not counted as passes.

Ledger SHA-256: `43fefb60a32de20f415d649422d7cb6009180c8b8bf7aeaaa449b4934e24bf96`.

This is **本轮分层验证和受影响用例重放后的去重结果**. It is not “538 passed” or one complete pristine run on the final tree. Exactly **312 latest passing identities in 97 files retain the initial full-run evidence**; affected slices were subsequently rebuilt/replayed. Their original hashes remain unchanged in the ledger.

## Raw runs and failure closure

All runtime paths below are relative to `.omx/private/qa-work/main-readiness-20261123/core/`.

| Run / group | Raw result | Interpretation |
| --- | --- | --- |
| `final-default` | 386 pass /130 skip /22 fail, 538 total | Frozen initial full inventory; retained as failed. |
| `responsive-rail-red` | 2 fail | Existing Web/Desktop rail cases extended before the product repair; both exposed lost creation state. |
| `postmodal-focus` | 5 pass /2 fail | Both repaired rails, recipient creation and two existing cases passed; workbench/geometry fixture failures remained. |
| `fixture-repair-replay` | 53 pass /6 fail, 59 total | Seventeen additional originally failing files; recipient was already separately covered. |
| `final-navigation-replay` | 40 pass | Closed remaining navigation/cross-platform href/fixture assumptions and replayed the relevant sidebar/notification/market flows. |
| `final-password` | 14 pass | Configured password scope. |
| `final-controls` | 9 pass | Configured namespace-control scope. |
| `final-recommendation` | 22 pass | Deterministic task-owned recommendation/quick-create scope. |
| `final-host` | 7 pass /1 fail | Knowledge-enabled host scope; ordinary reusable-session original conflicted with this configuration. |
| `final-host-stateful` | 1 pass | Entire unchanged ordinary host original under explicit knowledge-off configuration. |
| `final-admin-01` through `09` | 9 total pass | Nine independent task-owned admin allocations. |
| `final-password-human` | 8 pass /3 fail | Retained first conditional result. |
| `final-password-human-replay` | 1 pass /2 fail | Explicit CLI fixture admitted the source-surface case; the two archived-target failures persisted. |
| `offline-target-diagnostic` | 1 fail | Preserved request-storm diagnosis, not an acceptance pass. |
| `final-denial-replay` | 2 pass | Original Web/Electron offline-handoff journeys passed after the actual realtime guard repair. |
| `final-denial-regressions` | 23 pass | Affected realtime/account/notification recovery originals replayed on the rebuilt fix. |
| `final-mcp-native` | 1 fail | Old collapsed resource directory/Settings navigation assumption retained. |
| `final-mcp-native-replay` | 1 pass | Entire native MCP original passed after its bounded navigation fixture correction. |

Independent latest-outcome selection preserves all attempts and prefers the latest **executed** result; a later skipped configuration never erases an actual prior execution. Every original 22 failed tuple resolves through `fixture-repair-replay` or `final-navigation-replay`. Later conditional failures resolve through their explicit owning replays; counts are not added across attempts.

## Stateful host configuration is a legitimate scoped replay

The host original's source SHA is identical in `final-host` and `final-host-stateful`. The original expected `resumed=true`, equal workdir/session and distinct launch temporary directory remains unchanged.

- Knowledge-enabled failed evidence: two launches, both `resumed=false`, both no-session-persistence=true, distinct workdir/session hashes.
- Knowledge-off passing evidence: four launches completing the full scenario; initial launch fresh, second `resumed=true`, no-session-persistence=false, equal first/second workdir/session hashes.

The per-run claim namespace policy, including the empty contribution set, is documented in `host-resume-diagnosis.md`. The passing run records `conditional.knowledge=false`; S1 remains knowledge-enabled in its separate successful host scope. No original assertion or product security policy was relaxed.

## Archived notification fix is proved without changing expected denial

The final fixture retains the original generic canonical denial for the archived missing-row link, keeps `Issue unavailable` for the known redacted notification, and retains API404, private title/discussion absence and zero source-membership checks. It additionally verifies the settled denial node stays connected/visible for a bounded one-second observation. The earlier alternate wording attempt remains a failure and is not presented as the fix.

Decoded `offline-handoff-identities` attachments in `final-denial-replay/playwright.json` record only the final archived-target observation:

- Web: four notification HTTP200 responses, two issue HTTP404 responses.
- Electron: one notification HTTP200 response, one issue HTTP404 response.
- **Zero HTTP503 within that bounded archived-target window.**

Earlier portions of the two complete journeys still contain some HTTP503s; do not broaden this to “no 503 anywhere.” Canonical unit regression owns exact denied-binder/recovery counts; these actual browser observations establish stable visible refusal without the reproduced storm. `final-denial-regressions` separately passes 23 affected originals on the rebuilt core fix.

## Source applicability and final mechanical delta

Every one of the 26 admitted runs has matching `source-start.json` / `source-end.json` digests, its run admission digest matches, and `completion.json.source_preserved=true`. Per-run commits remain `73a28ed90` with explicit dirty-source manifests. Across the initial baseline and last runtime, the only production-file changes are the three shared modal ownership/focus files and `packages/core/realtime/collaboration.ts`; other differences are owning tests/fixtures and the dbid test. Earlier passes are retained with an impact-scoped replay argument, not relabelled as final-byte executions.

Final runtime snapshot: `a0c856da7308e0ed3e6bc441d5d3be2fbb2ef8734edb39200934c570ecde387e`; Web build `VLKTO9SF0CdGqBRibRIvB`.

After that successful MCP runtime, one lint-only callback declaration changed from empty destructuring to `{ browserName: _browserName }`. The test body, assertions and product files are unchanged. The runtime MCP test hash is `03bdb06005ceb8de2e556a552030211dbc403741fade9c5b04660fecfc301e47`; current source is `1849232e3232a55d7221a95af18b4bd6ec861723c18467591b2640ac65014c1e`. `final-e2e-checks.json` records scoped ESLint, diff-check and MCP collection exit0 after that edit. This is not a new execution of the final test bytes.

Compared `final-handoff-source.json` to the last runtime manifest: only that one MCP test differs. The handoff digest is `8222d487253da58bf90b77d8b3802dab3a0781f49c73da14faaf53d67c2657f8`; every listed live private file matches it at this audit. No unreported product drift was found.

One earlier intermediate review described an added Desktop workspace-retry click. That experimental addition was removed; the final Desktop fixture preserves the original requirement—visible exact Retry loading workspaces during a controlled list failure, screenshot before cleanup, and usable account detail. The final evidence must not claim that case performed the removed successful retry sequence. Relative-vs-absolute Desktop Settings href matching and opening actual directory disclosures are intentional bounded fixture corrections; no original business assertion was dropped.

## Cleanup confirmed

For all 26 ledger runs, compared `completion.json` with `cleanup.json`: exits agree, all recorded owned application ports are closed, original gate booleans are restored and epochs do not decrease. `final-cleanup.json` additionally lists **32 completed allocations**, including earlier targeted attempts excluded from the final source ledger. Independently compared all 32 entries to their raw cleanup receipts: no port/gate mismatch.

The browser PostgreSQL cleanup records exact container `38907dd49b12794489a54982f4832e3622aff82c9f43bcffcef60ebd94725788`, owner `main-readiness-70808c3cbb5a`, and zero other client connections before stopping. Current read-only Docker inspection confirmed that ID/owner and `exited` state; loopback port **53588 is closed**. No force-disconnect or unrelated resource action occurred during this audit. Backend PG/Redis cleanup is separately confirmed in `verification-evidence-audit.md`.

## Remaining limits and durable links

The 74 skips retain exact prerequisite reasons: special allocation/provenance families, packaged or Linux-specific execution, device/password migration variants, dedicated memory/closure providers, performance and other explicitly gated acceptance. They are neither hidden failures nor verified product passes. No commercial Provider-account behavior, mobile native UI, packaged Main/installer, or Windows runtime acceptance follows from the local results.

Authoritative retained artifacts:

- `core/final-ledger.json`: exact identities, all admitted attempts and selected outcome.
- `core/<run>/{playwright.json,run.json,source-start.json,source-end.json,completion.json,cleanup.json}`: each actual execution.
- `core/final-cleanup.json`, `core/database-cleanup.json`: all allocation/resource cleanup.
- `core/final-handoff-source.json`, `core/final-e2e-checks.json`: final mechanical source delta and static checks.
- `.trellis/tasks/11-23-cross-workspace-main-readiness/research/broad-browser-final-summary.json`: compact handoff; counts/digests matched this audit.

Final refresh: `broad-browser-final-results.md` and `browser-case-results.json` are now present. The narrative accurately distinguishes recorded-source reuse, scope-specific recovery, the final mechanical test delta, remaining skips and unapproved remote/exemption actions. The compact summary and all raw links above exist. Remote CI and the image-budget exemption remain separate: no PR/Actions success or approved image exception is established by browser acceptance.
