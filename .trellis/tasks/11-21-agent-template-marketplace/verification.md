# Verification — 2026-11-21

## Outcome

Implemented the approved shared-content, two-entrance design. Capability market now appends Agent templates; the market has no saved-agent dependency or instance inventory. The original picker, existing-instance links, squad context, template configuration and submit semantics remain available.

Source-aware Back uses the existing safe-history helper with a fixed market fallback, with explicit squad context taking priority. Shared role cards retain the original visual system. Narrow/coarse-pointer fixes keep the selected category visible, prevent type-label overlap, and provide 44px touch actions.

## Checks and evidence

| Check | Result | Evidence |
| --- | --- | --- |
| Core targeted unit tests | 3 suites, 34 tests passed | `evidence/core-tests.log` |
| Views targeted unit tests | 8 suites, 85 tests passed | `evidence/views-tests.log` |
| Final production Web and native Desktop acceptance | 9 tests passed, 0 retries | `evidence/e2e-final.log` |
| Core and Views typecheck | Passed; Views rerun after observer implementation | Implement/check agent verified exit 0; no type/interface change after the final CSS-only sizing fix |
| Full Core lint | Passed | Independent check agent, exit 0 |
| Full Views lint | Passed with 27 pre-existing warnings, 0 errors | Independent check agent; changed source also passed focused ESLint after each final fix |
| Initial independently committable market variant | 14 market tests passed | `evidence/isolated-index-market-tests.log`; excludes the then-uncommitted header/provider WIP |
| Visual review | Pass, 95/100 | `evidence/visual-verdict.json` and three retained PNGs |
| Whitespace / locale parsing | Passed | Isolated staged diff check and parsed locale JSON |

Final unit execution used at most two workers; final browser/native execution used one worker and no retries. The E2E result is from the final compiled candidate, including the label-width fix and the native real create-and-open assertion.

## Acceptance mapping

| PRD criteria | Verified outcome |
| --- | --- |
| AC1 | Direct market URL selects Agent templates; card links configure the chosen key. Merely opening cards/instances does not create or staff an agent. |
| AC2 | Panel unit tests assert no agent-list API dependency or instance/empty-instance content. |
| AC3 | Original chooser/picker/configuration, renamed active instances, archived exclusion, new-tab links and scroll restoration pass. |
| AC4 | Unit cases cover safe history, cold fixed fallback, unknown source and squad priority; native modifier/new-tab flow exercises the cold fallback. |
| AC5 | Canonical submit tests cover ordinary/squad success, join failure and request ownership. Web and native creation open the new agent; market source is absent from the API body. |
| AC6 | Market journey covers direct URL, keyboard selection, refresh, Back/Forward and unrelated query/hash retention. |
| AC7 | Market unit/E2E suites protect lazy mount, retained inactive panels, observer gating and existing Skills draft/portal behavior. |
| AC8 | Template pending/error/retry/empty and unknown policy/key states are covered; market workspace isolation passes. |
| AC9 | 1440/768/390/320 captures show no page overflow; coarse-pointer geometry asserts actions >=44px and label text within its tab. Selected tab viewport ratio is 1. Native English and Chinese catalogs are readable. |
| AC10 | Production Web and native Electron use the shared views; native additionally performs an actual backend template create using a fixture runtime with no process/model attached, then opens the saved agent. |

## Responsive regressions found and fixed

1. A 768px coarse-pointer measurement returned 36px instead of 44px for role actions. Added `pointer-coarse:min-h-11` and retained ordinary fine-pointer desktop sizing.
2. At 320px, the selected type had only ~0.548 viewport visibility. The type strip now reveals its selected tab on URL selection and width changes, with observer cleanup.
3. The coarse-pointer capture showed long type labels overlapping under flex shrink. The strip now gives type buttons intrinsic width via `flex-none`; the new text-width assertion first failed with 98px content inside an 84px tab and passes after the fix.

Stale test setup was corrected without changing product behavior: market acceptance now uses the current Skill welcome copy/action, and native Back is scoped to the creation banner rather than the desktop navigation control. Native resource initialization and cleanup are protected by nested `finally` blocks.

## Production provenance

- Isolated verification checkout: `/Volumes/artisan/tmp/agent-template-marketplace-skedr3sf`.
- Environment: `check-20261121063709-36694`; API 18370, production Web 13290, built Desktop renderer 5464.
- Final Web build: `3yVM25ucBSLm31AFgK3-h`; Web and Desktop builds succeeded.
- `runtime-environment.json` records the serving process/build identity before task-owned services are stopped. `source-snapshot.json` records the source snapshot descriptor; `final-candidate-files.json` records the final root file hashes.
- Fourteen code/test/locale files are byte-identical between root and the tested snapshot. Both agent locale files have identical `role_templates` objects; the snapshot additionally contains unrelated `approvals.learn_more` copy. That unrelated key is excluded from this task commit. Root-only spec updates are documentation.
- The original checkout's occupied/unowned services were not reused or stopped. Acceptance used isolated databases and auth configuration, preserving the source deployment settings.

## Static-analysis baseline and limits

Full `pnpm knip` ran and returned exit 1 for existing repository findings: 10 unused files, one plugin-sdk development dependency and 2268 unlisted-dependency entries, predominantly historical copied checkouts under `artifacts/`. The Core/Views scoped scan reported five unchanged baseline files. The independent reviewer confirmed no new task-owned card/panel/source path was reported; no scanning configuration or unrelated code was changed.

Mobile, packaged installers/updaters and real model/agent execution are outside this change. The fixture runtime used for native creation has no daemon process or attached model; native assertions confirm no daemon start or installer invocation.

## Retained visual evidence

- `evidence/market-desktop-zh.png`: native Chinese market with the real built-in roles.
- `evidence/market-phone-320.png`: selected type fully visible and single-column role cards.
- `evidence/market-touch-768.png`: nonoverlapping type labels and 44px role actions.

## Ownership and cleanup

Only the current task's 18 code/spec paths and task artifacts enter the work commit. During final ownership validation, the pre-existing market collection header, provider test wiring and help-link copy landed independently in parent `a1d791dcb`. The guarded index preparation detected that HEAD changed and stopped before committing; its index was then refreshed from that parent. The original working files are retained, and unrelated staged entries remain unchanged.

Shared collection-header and creation-shell implementations are byte-identical to the tested snapshot. The newer docs slug map adds only an unrelated `usage` entry. No feature-source rebase or product edit was required by the parent commit.

Stop only this task's API/Web/renderer processes after acceptance; retain the isolated source and database record for review/TTL cleanup. Archive only this task after committing, then record the session without sweeping other journal edits.
