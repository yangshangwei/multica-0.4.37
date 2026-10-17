# Independent Trellis review

Reviewed on 2026-10-17 by the dispatched check agent. No product, test or spec edits were required by this review. The reviewer inspected source and saved execution evidence; the main session ran the services and verification commands.

## Findings

No blocking correctness, accessibility or coverage defects found in the scoped changes.

- `packages/views/auth/password-form.tsx:164` reserves the complete localized error footprint, preventing blur validation from moving Submit during a click. Hidden feedback has no alert role or input error association; active feedback retains the existing focus, keyboard and validation behavior. The unchanged browser regression proves one-click mismatch handling and exactly one corrected registration.
- `packages/views/modals/creator-recommendations.tsx:102` bounds only the compact list and retains native scrolling, every suggestion, explicit adoption and the original desktop cap. The strengthened English/Chinese tests tab through all three controls, require their full visibility, adopt the last, and retain editor/default/recent values.
- The two inbox spec files now locate the real list item and named native title button. Notification content, read state, archived preview, reload persistence and compact return assertions remain intact.
- The three new notes in `component-guidelines.md`, `quality-guidelines.md` and `quick-create-actor-picker.md` accurately record these contracts and their canonical test layers. Screenshot `animations: 'disabled'` additions stabilize captures without weakening assertions.

## Result identity audit

Identities were independently matched by spec file, project and test title, rather than changing source line numbers.

| Evidence | Passed | Failed | Skipped |
| --- | ---: | ---: | ---: |
| `baseline-password.json` | 0 | 1 | 0 |
| `baseline-legacy.json` | 0 | 5 | 0 |
| `repaired-password.json` | 14 | 0 | 0 |
| `repaired-legacy.json` | 13 | 1 | 0 |
| `final-recommendations.json` | 4 | 0 | 0 |

All **six original failed identities** map to passing final results. The repaired/final files contain **28 unique existing cases, all with a passing latest result**. They represent 32 executions: 31 passes and one earlier native environment failure. All four final recommendation identities overlap the legacy run; three had already passed. Do not report 32 unique cases or erase the earlier failure. Every selected final result contains one execution at retry 0; all runs use one worker, zero configured retries and repeatEach 1. The separate native diagnostic is not additional business coverage. `result-summary.json` agrees with this accounting.

The native diagnostic records CORS rejection from `http://127.0.0.1:58657` to the task API at `http://localhost:18157`. `native-cors-fixed.txt` proves that exact origin was subsequently accepted; the real Electron case then passed in the unchanged business flow. This supports an environment correction, not a product fix or relaxed assertion.

## Verification evidence

- **Lint / TypeCheck: pass.** `static-checks.log` reports 15/15 successful tasks, no cached tasks and no errors. Its 31 lint warnings are outside the scoped changes. `e2e-types.log` contains no TypeScript diagnostics, but does contain a pnpm configuration warning; it is not literally empty.
- **Views tests: pass.** `views-tests.log` reports 563 files and 6,907 tests passed. Existing jsdom capability messages remain visible in the log.
- **UI exports: pass.** `ui-exports.log` reports 62 checked files and clean wildcard exports.
- **Static diff: pass.** The reviewer ran `git diff --check` over the scoped source/tests and all three spec notes after their final edits.
- **Environment provenance:** saved Web records use production `next start`; baseline build `Xe1tHGQAZDM-65LX-uocO` changes to repaired build `ce_N_SyJjHUSFX0kMoAg5`. Auth snapshots match password versus legacy suites. The API records explicitly show development-mode `go run`; this was production Web verification, not a production API deployment.
- **Main-session visual/cleanup evidence:** `qa02-visual-final.json` records 97/pass with no suggestions. `cleanup-verification.json` records no remaining task database, listeners, registry or temporary checkout, while original user services remained running.

No remaining task defect was identified. This focused review does not claim a rerun of the historical 328-case suite, physical-touch testing or real screen-reader testing.
