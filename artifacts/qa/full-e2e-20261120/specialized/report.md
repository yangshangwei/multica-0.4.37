# Specialized current-run acceptance and repairs

All **22 unique cases passed** on their final current-run executions, with **0 unresolved failures and 0 skips**. Original failed runs remain in their own files.

| Area | Cases | Result |
| --- | ---: | --- |
| Explicit completion authorization, Web/Desktop | 2 | Pass, fresh local artifacts and final fixed build |
| Collaboration detail, Web/Desktop | 2 | Pass after real cache-authority defect fix |
| Iteration history navigation | 2 | Pass after obsolete RSC synchronization repair |
| Creator recommendation, Web/Desktop/Chinese narrow | 4 | Pass, deterministic local provider |
| Compiled CLI business/authority | 6 | Pass, task-owned compiled CLI |
| Device login isolation | 1 | Pass |
| Live changelog publication | 1 | Pass, no page/server restart |
| Automatic project knowledge, Web/Desktop | 2 | Pass, real managed daemon and controlled model/provider |
| Linux full packaged Main | 2 | Pass, real Main/preload/renderer/bundled CLI |

## Product defect and regression

Completing a collaboration task could make the UI claim access was lost. The still-mounted action-options query refetched after completion and correctly received403 because completion was no longer eligible. The client wrongly cleared all issue data. Core now revalidates actual issue read authority inside the same auth/access fence and preserves valid detail while keeping the action refused. Actual read denial still clears protected data. See `detail-defect.md`, original `detail-browser.json`, green `detail-fixed.json`, and five new hook regressions.

## Fixture repairs

- Iteration history now holds the real history.replaceState canonicalization call, clicks the native summary while the UUID address remains, releases the captured call, and verifies the same open/connected element before the real history response. No product navigation changed.
- Packaged host assertion now checks exact first/third outcome200 and stopped second outcome403, all non-outcome200, exact three identities/one physical launch identity per run, immutable event/hash/receipt and termination→outcome→proof order. Existing business results, native execution, restart, pause/Resume and sealed-ledger checks remain. Parent independently reviewed the change.
- Preserved CLI path, changelog test-selection, package-manager setup and inappropriate shared host-gate failures; corrected only runner configuration and retained separate successful replays.

## Verification

- Main initial static/unit lane: see `../static/report.md` (14,709 fresh Vitest passes across all packages including mobile).
- Final changed Core/Views/Web lint/typecheck: all six direct fresh commands passed; no Turbo result cache or mdx rebuild.
- Final Core focused21/21 and Views workflow30/30 passed with unchanged default timeouts. Earlier load-sensitive5s failures retained.
- Review: `review.md`; source assertions were not weakened.

## Package identity and cleanup

The current unsigned Linux ARM64 directory package embeds the fixed current495-file Desktop output; every packaged output hash matches. Electron39.8.7 and CLI are AArch64. ASAR/executable/CLI identities are recorded in `packaged/package.json` and `packaged/asar-verification.json`. Actual package login used the real password form; native IPC was not stubbed. Four task-created Main containers from original/replay runs are confirmed removed.

Both owned native PostgreSQL containers are stopped with private forensic dumps retained. Owned API/Web/renderer/model/provider processes stopped, service ports free, zero remaining database clients; original manual-only execution gates restored. No other agent/environment/user database was stopped or changed. Receipts: `cleanup.json`, `knowledge/cleanup.json`, `packaged/cleanup.json`, `packaged/final-verification.json`.

## Coverage limits

Controlled model/provider output does not establish real-provider reasoning quality. Linux directory package acceptance is not dpkg installation, updater/restart upgrade, signing, or packaged macOS/Windows acceptance. Mobile static/headless passes do not establish real-device UI coverage.

## Evidence

- `summary.json` and `browser-case-inventory.json`: exact22 final case identities.
- `detail-defect.md`, `history-fixture-fix.md`, `packaged/host-fixture-fix.md`: reproduction, ownership and repair.
- Per-run JSON/logs, screenshots and decoded Main transport evidence retain original errors and successful replays.
