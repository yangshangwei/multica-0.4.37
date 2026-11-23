# Independent frontend verification audit

2026-11-23. Read-only audit by native `standards_review`. Root and private source both resolve to `73a28ed90fdcfd9eda1ac5d2970df5af2c486f94`; the private checkout additionally contains the task-owned readiness corrections. This audit inspected retained results and current source hashes only: no tests, browser sessions, builds, staging, task-status changes or product edits. This report is its only retained edit.

## Result

`frontend-verification.json` has correct aggregate arithmetic and appropriately excludes the initial interrupted Views run. The reconciled non-mobile total is **15,054 tests**; Mobile is **196 additional tests**, separately scoped and without native UI acceptance. The evidence supports package/file-level reconciliation with complete affected-file replays. It does **not** establish a single complete post-fix run of every Views test or an independently retained per-case result ledger.

Raw logs root `Q`: `.omx/private/qa-work/main-readiness-20261123/`.

| Package / evidence | Raw result and applicability |
| --- | --- |
| Core, `Q/ts-tests.log:714` | 381 files / 5,502 tests passed before the overall command was interrupted in Views. Parsed all 381 completed Core file lines and independently summed 5,502. Core source and its shared package/config/dependency inputs did not change from `cb192014b` through the final source. |
| Initial Views, `Q/ts-tests.log` | Incomplete; no complete package pass credited. `Q/ts-interruption.json` records the deliberate SIGTERM to remove resource competition. Individual partial successes are not added to later totals. |
| Docs, `Q/ts-tests-r2.log:27` | 10 files / 64 tests passed. All ten file rows independently sum to 64. Docs source, generator and dependencies are unchanged through the final source; canonical output synchronization was reviewed separately. |
| Views baseline, `Q/ts-tests-r2.log:6575` | 630 files / 7,845 tests: 7,844 passed and one stale Settings expectation failed. Parsed all 630 distinct file rows; counts sum to 7,845. The original overall command remains failed. |
| Settings replay, `research/settings-test-repair.md` | Complete settings file (24) plus canonical page-header file (8): 32 passed on final source after the assertion-only fix. These already belonged to the baseline; do not add 32 to the package total. The retained failure is closed by the successful owning-file replay. |
| Incoming UI replay, `Q/ts-incoming.log:6` | 4 complete files / 53 tests passed on final source. The same prior file group held 46 tests; seven tests are new. Count calculation below. |
| Web, `Q/ts-consumers.log:132` | 36 files / 322 tests passed; all file-row counts independently reconciled. |
| Desktop, `Q/ts-consumers.log:1061` | 104 files / 1,314 tests passed; all file-row counts independently reconciled. Consumer command ended with 2/2 successful Turbo tasks. |
| Mobile, `Q/mobile-tests.log:6` | 28 files / 196 tests passed, separately executed. This is unit verification, not a native device/simulator journey. |

### Deduplication arithmetic

The baseline file rows for the incoming group are:

| File | Baseline tests | Final tests |
| --- | ---: | ---: |
| `collaboration/notifications-page.test.tsx` | 27 | 27 |
| `marketplace/marketplace-page.test.tsx` | 14 | 14 |
| `marketplace/agent-templates-market-panel.test.tsx` | 5 | 7 |
| New `marketplace/marketplace-welcome.test.tsx` | 0 | 5 |
| Total | 46 | 53 |

The new welcome file has four declarations, one parameterized over true/false, therefore five cases. The panel adds two cases. Views is `7,845 - 46 + 53 = 7,852`, with `630 + 1 = 631` files. The corrected Settings replay replaces an existing failed result; it does not increase the case population.

Non-mobile: `5,502 + 7,852 + 64 + 322 + 1,314 = 15,054`. Mobile remains separately reported; the optional combined arithmetic is 15,250, not a single unified run.

The actually present private cache `src/packages/views/node_modules/.vite/vitest/da39a3ee5e6b4b0d3255bfef95601890afd80709/results.json` contains 631 file entries, all `failed: false`, including the four incoming files and both settings/page-header replay files. This is **corroboration only**: its per-file fields are just `duration` and `failed`, with no source hashes, case names or invocation timestamps. The compact incoming log reports 4/53 but does not itself enumerate the four filenames or all case names. Do not describe this as an independent per-case inventory.

## Source applicability and private/root equality

Compared `git diff cb192014b..73a28ed90` for Core, Docs, Mobile, Desktop, UI, shared TypeScript/ESLint packages, canonical docs generator, `package.json`, `pnpm-lock.yaml`, `pnpm-workspace.yaml` and `turbo.json`: no changes. Between `95c8b12dc` and final source the only application/package changes are the five market component/test paths covered by the market replay; notification changes were already in the complete Views baseline and were replayed again.

SHA-256 compared **9,332 tracked files** across `apps`, `packages`, `server`, `scripts`, `.github`, `e2e` and root package/dependency/Turbo configuration between live root and private checkout. No tracked file was missing. Differences were exactly:

- `.github/workflows/ci.yml` and `scripts/check-image-budget.mjs` (owned CI checker correction);
- `e2e/localized-template-defaults.spec.ts` (owned welcome-link locator correction);
- `packages/views/settings/components/settings-page.test.tsx` (owned stale assertion correction);
- four canonical embedded docs JSON outputs: collaboration, desktop-app, feature-overview, tutorial.

All other tracked implementation/test/config bytes in that comparison are identical, including the incoming market and notification product files. Private untracked additions are the new image-check regression test and new image-budget spec; they are explicitly task-owned and outside a tracked-file equality claim. The two modified Web spec documents are also task-owned and were independently reviewed earlier.

This supports reuse of unchanged owning-package evidence and affected-file replay; it does not prove that every consumer interaction affected transitively by a new market component has been re-executed. Final production Web/Desktop browser coverage is parent-owned and remains necessary for the incoming UI changes.

## Static checks and warnings

`Q/static-final.log:169` records **15 successful / 15 total Turbo tasks**, zero cache hits. More precisely, these are **13 lint/typecheck tasks plus two MDX prerequisite generators** (`web:mdx`, `docs:mdx`), not fifteen distinct lint/typecheck suites. No errors appear in the successful pipeline.

Warnings are 27 Views + 3 UI + 1 Desktop = **31**. Compared warning diagnostic messages against `Q/static-baseline.log`: all 31 are identical, substantiating the existing-warning characterization. No warning was silently removed or converted into a pass claim.

`Q/mobile-static-results.json` records exit 0 for both Mobile typecheck and Mobile lint; associated logs are present. Mobile lint reports **0 errors / 7 warnings**. Those files have no source changes in this task or candidate refresh, so these are existing diagnostics in unchanged source. The pnpm configuration notice is separate from ESLint's warning counts. Successful check exits are evidenced by the retained parent execution records and logs; this reviewer did not rerun them.

## Recommended report wording

“Package suites and complete affected-file replays reconcile to 15,054 non-mobile tests, with the one retained stale test failure corrected and replayed successfully. Mobile adds 196 passing unit tests. No unresolved failure remains in that reconciled set. The full Views suite was not rerun once more after the fix; the original failed and interrupted runs remain retained. Final static pipeline completed 15 tasks, including two MDX prerequisites, with zero errors and 31 existing warnings; Mobile static checks passed with seven warnings. Browser/build/backend acceptance is reported separately.”

## Audited raw-log SHA-256

```text
b86ebc32afe7891b5e4c904b7b015e5d37638cfaee6ef4d51831118d9305e599  ts-tests.log
ded5e0c62090d64ad08c0ac826284a6f18ea2cd13c64e3bc97e56944adc19d2a  ts-tests-r2.log
72d421fa3b6ee55de10d05ede61f2a147ad7d49ab65bcc5e5fbaa7405ddcf0bb  ts-consumers.log
026b864e6a8e6da220e66a9abef5a6ab8b06843b666945a6f9fd9f0370b1555b  ts-incoming.log
dabdcdd64d5af2d64b4ab0cc1820730824dc239319e17be71d93dbbb28ce1ee3  static-final.log
18d5809c5f36acb1eda53df66b488952b37bd1e69bbbaac6bd75e84f0fbdccbd  mobile-tests.log
b088dd89f46094c36f806113430a5514d54725ca237cdfb960e6b221ed10932f  mobile-lint.log
f7e8c8b593d633106e19f6c52c4551281547a4cf76ad7da233ba17a984e48fce  mobile-typecheck.log
fdd00661cf1e230c9e77310e7ba494a3bef7bade1b4971b5fe7ee18fae3d0e71  mobile-static-results.json
```
