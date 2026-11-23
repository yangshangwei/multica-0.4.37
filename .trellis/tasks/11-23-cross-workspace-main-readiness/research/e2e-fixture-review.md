# Independent review of browser-fixture repairs

2026-11-23. Read-only review of the private `src` diff against `73a28ed90`, plus retained raw reports/trace. Fourteen E2E source/helper files are modified; their consumers span more files. No source edits, browser or unit execution, staging or extra agents. `fixture-repair-replay` was running during review, so its collection is not credited as a completed pass.

## Verdict

No substantive assertion weakening found in the scoped changes. Obsolete navigation assumptions are replaced with current real controls; the changed workbench step establishes the issue identity before testing its permissions/narrow detail; the visual assertion now measures the intentionally two-row directory's four targets. Original backend/business assertions, deadlines, cleanup, error evidence and final actions remain.

Accounting correction: the frozen **22 failures occupy 18 distinct spec files**, not 17. The new replay collection is 59 cases across **17 additional files**; the omitted eighteenth is the unchanged recipient-picker original, already passed in `postmodal-focus`. The union covers all 22 original failed identities. A passing collection or planned union is not itself successful replay evidence.

## Semantics checked

- **Account/logout helper:** `openAccountMenu` identifies exactly one avatar-bearing button inside the existing sidebar footer, requires a nonempty accessible name, opens it and retains the visible Log out assertion. It no longer assumes the product-brand text is the account trigger. All three original consumers remain unchanged, including real login/reload/logout and subsequent access refusal.
- **Workspace switching:** `capability-market.spec.ts` uses the exact Switch workspace button and first checks its `aria-description` equals the current workspace's name. Existing target menu-item identity, destination URL, stored filters, draft isolation and per-workspace resource counts remain. The notification journey's current-workspace-name click was not changed; it intentionally restores that workspace's prior location.
- **Workspace-list error/retry:** `collaboration-account-desktop.spec.ts` replaces arbitrary `Refresh.first()` with the exact named retry in Application navigation. It captures `desktop-workspace-list-error.png` **while the controlled 500 is still active**, then releases the error, clicks retry, awaits a real 200 workspace response, verifies disappearance of retry and continued account detail. It strengthens recovery without skipping the failed state or altering account data.
- **Settings navigation:** iteration helper/audit selectors use exact Settings plus exact workspace href; the knowledge helper uses exact Settings plus the sidebar menu-button constraint after selecting the intended workspace. Navigation smoke scopes Settings to Application navigation. These changes preserve actual UI navigation and every subsequent iteration/knowledge/settings assertion; none changes the unrelated “Workspace settings saved” result text.
- **Welcome entry:** native MCP/Skills cases expand the actual Resources & connections disclosure before clicking its real link. Their creation/cancel/focus, manual examples, native screenshots and narrow overflow assertions remain. Skill market verifies API inventory is empty before visiting its named welcome region, retains the absence of inline template tabs, and follows From template via keyboard. Workspace-defaults adds an explicit empty-squad API assertion, follows Browse templates to the exact squad marketplace URL/selected category, and retains the final saved-project `needs_runtime` and zero auto-created squads checks.
- **Localized Squad entry:** exact localized accessible creation name intersects the intended workspace creation href; role localization, omitted-name request, reuse, edited copies and collision atomicity are unchanged.
- **Responsive dialog regressions:** the existing Web and Desktop rail cases now retain title/source across compact/large transitions, then test compact creation closes navigation and Escape returns focus. Their original navigation/width/current-workspace assertions remain. The actual recipient test was not rewritten to reopen its lost dialog or resize earlier; it passed after the product fix.

## Workbench trace supports reselecting the issue

Inspected the retained ZIP directly:

`core/postmodal-focus/results/collaboration-workbench-we-140e7-tion-handoff-and-revocation-chromium/trace.zip`, stream `1-trace.trace`.

- `call@113`, 35,576.354ms: clicks the Participating sidebar link.
- Main-frame snapshot 35,719.973ms: URL is `/account?view=participating`, without `issue`, following the previous issue detail URL.
- `call@157`, 36,562.710ms: viewport changes to 390×844; immediately before and after, the URL is still the list-only Participating route.
- `call@159`, 36,571.697ms: begins the failing Back to list visibility assertion.

The test had deliberately navigated away from selection before resizing. Clicking the visible participating issue, asserting `searchParams.issue === created.id`, and checking its exact heading restores the actual precondition. This is not masking selection loss caused by resize. It also makes the retained no-Transfer assertion meaningful against the transferred-away issue rather than an empty detail area. Recipient authority, cancellation receipt, comment delivery, revoke and header-isolation assertions remain intact.

## Directory geometry is an intentional contract change

Both full-run and post-modal specialist failures measured 110px against an obsolete 64px whole-tablist maximum. `marketplace-welcome.module.css` declares two columns, 52px minimum target height and a 6px gap: two rows naturally measure 110px. The approved market component has exactly four categories.

The repair keeps the old upper bound for the independent workspace-entry link. For the market directory it requires exactly four target rectangles, each width/height ≥44px and contained within the directory (one-pixel numerical tolerance). Existing page horizontal overflow, dialog viewport bounds, dialog overflow, primary-action visibility, locale/theme and unmodified saved-content assertions remain. No new max-height assertion is claimed, and the metric is container containment rather than proof that a directory behind an open modal is currently clickable. The owning marketplace journey remains responsible for actual category keyboard/click behavior.

## Raw execution/provenance checks

All paths below are under `.omx/private/qa-work/main-readiness-20261123/core/`.

| Run | Actual terminal result | Source digest / audit |
| --- | --- | --- |
| `final-default/playwright.json` | **538 = 386 passed + 130 skipped + 22 failed**, zero flaky/automatic retry | `5ed08fcf45aa4a94af56fd35f5b84706174c31235ea8b242419bb9bfaa889a18`; source-start equals source-end; completion exit 1; owned Provider/API/Web/renderer ports recorded closed. |
| `responsive-rail-red/playwright.json` | **2 failed**, no skip/retry | `28b448407235111eea09bf58671fd0bc7746752280cb69da98b22237c27a60b2`; source preserved. Web fails finding the source combobox and Desktop fails finding the dialog, both consistent with responsive unmount. Neither is a pass. |
| `postmodal-focus/playwright.json` | **5 passed + 2 failed**, no skip/retry | `143074616522c50e4793f3c9856521ddc70736e412d2f491937a68196c201769`; source preserved; completion exit 1, owned ports closed. Passed: both extended rail originals, unchanged recipient creation, unchanged second-Enter iteration original and existing lifecycle conflict case. Remaining reds: workbench missing selected detail and obsolete market geometry. |
| `fixture-repair-replay/collection.json` | **59 collected / 17 files**, not yet terminal at audit time | Contains every original failed identity except recipient; adding that exact passed identity from postmodal-focus leaves no missing original failure. Do not describe this planned coverage as 59 passed until terminal JSON and cleanup arrive. |

The three completed runs' own hashes/receipts were compared without rebinding them to newer bytes. This review does not assert that every changed fixture has passed yet or that original skips became passes. Final identity reconciliation, conditional admissions and new-source applicability belong in the final ledger audit.

## Review boundary fingerprints

```text
ed8c180a9c731404dc3f81d001b28e2b24caa6aa558d0c334b26c105183fdc30  e2e/capability-market.spec.ts
ca2201763b582cb285d53de3c2451968b5ad42a19c1996f29cccccec5994bb87  e2e/collaboration-account-desktop.spec.ts
8e7e5513fbfef4c552ec39deb2b37247d4a686dcd12339c0812d80fac6d15052  e2e/collaboration-workbench-web.spec.ts
0c22f72ef4379203c43437991db91df94b8110f38de2a5f2ebed9f6bff0ca3fa  e2e/fixtures/collaboration-knowledge.ts
2222eac98dcb3b2609dd40a9d8e28515ddbed3605c6a695b8a927d8641221e53  e2e/fixtures/iterations-i1.ts
fac7aa03e29bbd6c4e69612d0f253b8261dff80ee4700b18dabf1ee8dcd81b88  e2e/fixtures/sidebar-navigation.ts
4fdd359b302b0612e52c7dd3d09d40bb9dd2697de5bf22e235c0e95454784f5b  e2e/iterations-audit-desktop.spec.ts
ac8a27922da9a82539af349da12b20b851abd3ff5e435cdd14bdc73ea9e89033  e2e/localized-template-defaults.spec.ts
27d92a1eb1af5bed53fa3ea5bb5a3f348ec5e1a569696a92c894b43fc98f6995  e2e/mcp-welcome-desktop.spec.ts
7cb18382c3ec6232f876ee0eae6dfbb42c8776629c948c601fecc062f7e948a3  e2e/navigation.spec.ts
ce1a56265767bc5f40498e9384748de8c3b242d70af4830b8d31d7516e278daf  e2e/sidebar-icon-rail.spec.ts
85ddd5d70abf9a6faa75896016d65bde2cc6674cecadd1b8e9aeb412e061fe80  e2e/skill-market.spec.ts
276ba16154fbdeee100a01ec5e2025ea13d7231b6c0f8cc1cdc3b3d8baa71b15  e2e/skill-welcome-desktop.spec.ts
97d17916bb093993be9d7b9b76a5fc4b409c5f794de457814c95a62f74647c3e  e2e/workspace-defaults.spec.ts
```
