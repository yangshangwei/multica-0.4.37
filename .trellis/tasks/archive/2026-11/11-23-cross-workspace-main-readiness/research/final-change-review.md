# Independent review of the remaining changes and replay evidence

Date: 2026-11-23. Reviewer: native `standards_review` child. Scope: the localized Squad locator repair and three Web spec documents in the private checkout at `95c8b12dc619488e050e96dba8f97a527e2f4a4c`; retained five-case replay evidence; incoming root commits `9bca2c02ee710ee54311ec74fdf0572682f470c4` (market restoration) and `2d8fa836ef1df3c47557aecd05e31365f8277f32` (notification row navigation). Root was observed at `73a28ed90` during this review.

## Verdict and evidence boundary

No blocking defect found in this bounded code/spec review. The localized fixture change retains its original business assertions, and the five latest successful case results are supported by the raw reports. The incoming market and notification changes fit their recorded purpose and use the existing adapters; no high-confidence new regression was identified by inspection.

The five browser passes belong to **`cb192014b2f27e9bb64112db52073cbb31ce8f04` plus the locator repair**, production Web build **`_7zjZ-yIB7lbnEWws5Osv`**. They must not be relabelled as a run of `95c8b12dc`, `9bca2c02e`, or the final refreshed candidate. The report already preserves this boundary, including the API predating the regenerated embedded docs. Final candidate integration and affected verification remain parent-owned.

No browser, build, TypeScript, Go or other heavy suite was run by this reviewer. Read-only source comparison, receipt/hash comparison and whitespace checks were performed. No product/source files, task status, index, commits or running resources were changed; this report is the only retained edit.

## Localized fixture and new spec contracts

`e2e/localized-template-defaults.spec.ts:82` changes only the creation-link locator: exact localized accessible name intersected with the exact `/<slug>/squads/new` href, without assuming a header ancestor. It does not force a click, pick an arbitrary first match, alter a deadline, call a replacement API instead of the UI, or weaken the following assertions.

Both original case bodies remain unchanged: Chinese role names and omitted-name request language, reuse of the same agent IDs/leader, preservation of edited names/instructions, 409 collision feedback, no partial squad/agent/skill changes, and successful English recovery. Original source hash and the red receipt match; the green receipt matches the actual changed file.

The three Web spec additions accurately describe existing implementation and observed evidence:

- `image-budget-check.md` retains 300 KiB growth semantics, streamed NUL records, subprocess failure visibility, untracked-file limits, and the **PR-wide** exception boundary. Its index entry is valid. It does not claim a per-file allowlist or remote success.
- `e2e-run-environment.md` records the welcome-page link contract without requiring a redundant product action.
- Its namespace-control gate contract is supported by `e2e/fixtures/collaboration-control-journey.ts`: signed registration has no host capability and claim/permit specify `linux_pid_namespace_v1`. `server/internal/service/collaboration_runtime_preparation.go:184` refuses a newly promoted group without host supervision when the host gate is enabled. The documented separate allocation and original-boolean restoration preserve the server boundary.

Checks: private `git diff --check` for the localized spec and three Web spec paths passed; `git show --format= --check` for both incoming code commits passed.

## Raw replay audit

Raw root `Q` below is:

`.omx/private/qa-work/main-readiness-20261123/core/`

| Exact raw report path under Q | Observed cases | Result |
| --- | --- | --- |
| `localized-red/playwright.json` | Original Chinese staffing/reuse case and localized collision case | Both `timedOut`; errors explicitly wait on `locator('header').getByRole('link', { name: '新建AI小队', exact: true })` at line 82. The pending template-response promise also times out. |
| `localized-green/playwright.json` | Same two original titles | Both passed, retry 0, zero skipped/flaky. |
| `controls-original/playwright.json` | Original Web live-revoke and terminal-reopen cases | Both passed, retry 0, zero skipped/flaky. |
| `s1-red/playwright.json` | No test selected by the anchored grep | Zero cases; explicit `No tests found`, exit 1. Correctly excluded from distinct-case pass/fail accounting. |
| `s1-red-original/playwright.json` | Original S1 outsourcing case | Failed at unchanged `page.waitForResponse` 20-second deadline, before the knowledge disclosure step. |
| `s1-red-isolated/playwright.json` | Same original S1 outsourcing title | Passed, retry 0, zero skipped/flaky. |

For every run, compared `core-replay-current.json` to its exact `Q/<run>/run.json`, `cleanup.json` and `playwright.json`: source commit, Web build, fixture hashes, stats, process exit, gate-at-start, post-cleanup gate and closed-port results match. API health commit matches the run commit. Initial/final gate booleans match, epochs do not decrease, and all recorded run ports are closed. These retained receipts establish the run-time cleanup evidence; the cluster is explicitly handed to the parent and was not stopped by this review.

`controls-original/api.log` has six actual runtime requests: two GET run lists, two claim and two permit requests, all HTTP 200. Its live browser-start gate is `enabled=true`, `creation_enabled=true`, `host_execution_enabled=false`. The original identity, target, claim and permit assertions remain unchanged.

Successful semantic evidence attachments are retained as follows:

- `localized-green/playwright.json`: inline `localized-squad-reuse` attachment.
- `controls-original/playwright.json`: inline `live-revoke-evidence` and `terminal-reopen-evidence` attachments.
- `s1-red-isolated/results/collaboration-business-sce-e8bed-nd-reuse-approved-knowledge-chromium/attachments/s1-outsourcing-evidence-b0e7aef7816741a4a7c93db3102869bec959dd4e.json`.

The S1 spec, disclosure helper, control helper, and both control specs have exact SHA-256 parity with immutable `cb192014b` source and their run receipts. S1's original assertion/disclosure path passed without any speculative helper repair. `s1-red-original/resource-bounded.json` retains two separate ten-second Docker command timeouts plus VM measurements; this supports the documented resource limitation. It does not establish a database wait matrix or a uniquely proved causal mechanism for the browser timeout. The original failed attempt remains visible; the successful isolated rerun does not erase it or prove all attention-state variants.

Latest distinct cases are exactly **2 + 2 + 1 = 5 passed**, with no skips or automatic retries. This is a targeted replay, not complete branch acceptance or real-model acceptance.

## Incoming market and notification commits

Recorded meaning checked against root journal Session 193 (`.trellis/workspace/artisan/journal-4.md:684`) and each commit's intent/body, plus the updated capability-market and notification specs. The journal's 112-test and Web/Electron claims were read as the other session's evidence, not rerun or independently recounted here.

`9bca2c02e`:

- `MarketplaceCompactExample` initializes native `open` from the existing per-route/tab memento before layout and records only changed active toggle state, preserving explicit false/default closed. The category key and workspace remount boundary preserve ownership; no store, preference namespace or query was added.
- The agent catalog's existing ref callback now requires active + ready before its first restoration; its existing writer also guards active/ready/restored. Saved zero remains accepted. The real leading example supplies its height before parent offset assignment.
- The tests exercise initial true/false, inactive events, real disclosure-plus-parent restoration with scroll clamping, and existing source-aware Back. The native E2E retains earlier assertions, opens actual sidebar groups, and adds compact example/offset recovery rather than reducing the original journey.

`2d8fa836e`:

- Notification row pointer handlers use the already calculated exact href; unavailable items keep their content-free notification destination. Selection does not implicitly issue read/archive writes.
- Title anchors preserve native keyboard/modifier behavior via `AppLink`. `rowLinkInteractiveProps` isolates both click and auxclick on the title and action wrapper; retry keeps its auxclick shield and explicitly stops click propagation before its action.
- Read/archive remain awaited existing operations. The added tests retain exact notification/source/filter targets and verify action isolation; the shared navigation tests continue owning the full modifier matrix.

No new product boundary, persistence system or redesign is needed for these two fixes based on the inspected changes. Their inclusion in the final validated candidate still needs the parent's source/build refresh.

## Private reviewed-file fingerprints

```text
3780a5a736dd80e2a62f0b2fee7277de0b8926db1cfd025e565bacef4a173f7c  e2e/localized-template-defaults.spec.ts
6debb960e2b213337f88463b8a955fb4a748d6b5a155435a808106d8d3bc79a8  .trellis/spec/web/frontend/image-budget-check.md
35898a4c450fc59ef00ef60cf7e772145b79b1b4d7a508d54c160c69f869d206  .trellis/spec/web/frontend/index.md
d2f61bc84d0fc08544033e0bdcd4282094e2dc73ed98a8ba1c151ac0c982c400  .trellis/spec/web/frontend/e2e-run-environment.md
```
