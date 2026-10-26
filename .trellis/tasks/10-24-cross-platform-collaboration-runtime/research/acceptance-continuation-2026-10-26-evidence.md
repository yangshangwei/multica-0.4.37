# Acceptance continuation evidence audit

Audit snapshot: 2026-10-26T07:24:01.284749+00:00. Root HEAD: `1bfd169dca196dfd22bc1429a2933204d7f9e05b`.

This is a read-only recovery audit. No tests, source changes, service operations, account access, publication or gate changes were performed by this audit.

## Current authoritative ledgers

- Collaboration: `/tmp/multica-specialized-acceptance-20261026/collaboration/case-ledger.json` contains exactly 58 original Chromium identities, 30 passed and 28 pending. Each of the 30 pass rows was checked against its retained Playwright JSON: one matching original title/project, `expected` aggregate status, and an actual final `passed` result. No missing result files, skipped-result passes, or mismatches were found.
- Identity: `/tmp/multica-specialized-acceptance-20261026/identity/case-ledger.json` is already 31/31 passed. All 31 rows were independently matched to raw `runs/*/results.json` actual pass results.
- Device login is complete in `/tmp/multica-specialized-acceptance-20261026/identity/runs/20261026-140404-device-auth`: exit 0, one original case, zero extras, one actual execution, 1.369 seconds, no reported errors. Its `trace.zip` is retained. This supersedes the user's older device-login remainder.
- The `.omx/reports/e2e-codex-host-20261026` and `e2e-switch-fix-20261026` summaries are older campaign evidence. Do not count their earlier passes or failures as current-snapshot execution.

## Source and artifact applicability

A content-hash comparison examined 7,325 tracked files under `server`, `apps/web`, `apps/desktop`, `packages`, `scripts` and `e2e` between root and both detached worktrees.

`collaboration-source` has **zero Go/runtime source differences** from root. Its remaining differences are:

- Production frontend: `packages/views/editor/title-editor.tsx` lacks the root autofocus fix from `7fae5be67`.
- Test: `packages/views/editor/title-editor.test.tsx`.
- E2E: `e2e/capability-market.spec.ts`, `e2e/collaboration-installed-main.spec.ts`, `e2e/fixtures/collaboration-installed-main.mjs`, `e2e/fixtures/collaboration-preparation-records.ts`.

`collaboration-package-source` is a clean detached worktree at `d4d35ec99e0f370fdc01b3840df1428379e57682`. It has the same six differences plus `server/internal/service/collaboration_switch_view.go`, `server/internal/service/collaboration_host_integration_test.go`, `e2e/collaboration-account-desktop.spec.ts` and generated `apps/web/next-env.d.ts`. The service fix is present in the executing collaboration worktree, but absent in the package-source snapshot. The generated file is not a product-code defect.

The existing Linux arm64 directory package is internally pinned and intact, but cannot be described as byte-matching all root HEAD source while the frontend autofocus fix is absent. Preserve its original provenance. Sync the approved current source/fixture files and rebuild only the affected Web/renderer/package artifacts if a root-current candidate is required; the executing Go/runtime sources already match and do not need rebuilding merely for frontend drift.

Package provenance: `/tmp/multica-specialized-acceptance-20261026/collaboration/native/package-manifest.json`, with eight successful build records in `native/build-records.json` (Go 1.26.9, Linux arm64 and Darwin arm64 fake executables; package `--linux dir --arm64 --publish never`). The package's three hashes and sizes all match the manifest:

| Artifact | SHA-256 |
| --- | --- |
| `multica-desktop` | `379334fb6707615062d32fcc481a8a8d3140938531378ccba14f3fa169665043` |
| `resources/app.asar` | `dfd29e20d80a32558e389d51c0d5c122feaa7f522ce94aedb861f07ffcb6edbe` |
| bundled `multica` | `980f5a09628f0cdf94e3893f8572fb3425a60193dc338ccd9eef1d20b6b246d1` |

Every configured fake daemon/provider/CLI/relay path in `collaboration/environment/private.json` exists. `COLLABORATION_INSTALLED_APP_DIR` resolves to the manifest's package directory and `COLLABORATION_HOST_INSTALLED_ASAR_SHA256` matches its ASAR. The build records prove successful compilation, not native lifecycle execution. This Linux directory package does not prove macOS/Windows installers or authenticated Provider acceptance.

## Remaining original collaboration identities

Stable identity is original ledger number + project + file + full title; line numbers can move. Current pending rows:

| Ledger n | File | Original title |
| --- | --- | --- |
| 81 | `e2e/collaboration-installed-clients.spec.ts` | cached v0.6.7 CLI cannot bypass paused, unconfirmed-stop or removed-author commands |
| 299 | `e2e/collaboration-control-journey-web.spec.ts` | Web controls recover partial Resume, isolate agent/squad lanes and stop the whole issue |
| 300 | `e2e/collaboration-control-journey-web.spec.ts` | Web pending termination proof keeps A paused while whole Resume releases B and squad S |
| 301 | `e2e/collaboration-cross-client-execution.spec.ts` | AC33 Web and actual Electron collaborators execute independent A/B namespaces and one coalesced A follow-up |
| 347 | `e2e/collaboration-control-journey-desktop.spec.ts` | Desktop controls recover partial Resume, isolate agent/squad lanes and stop the whole issue |
| 348 | `e2e/collaboration-control-journey-desktop.spec.ts` | Desktop pending termination proof keeps A paused while whole Resume releases B and squad S |
| 368 | `e2e/collaboration-administration-desktop.spec.ts` | Desktop administers disabled-creator collaborations in the account shell and recovers the retained receipt |
| 369 | `e2e/collaboration-administration-web.spec.ts` | Web administers disabled-creator collaborations with explicit succession and original receipt recovery |
| 370 | `e2e/collaboration-offline-handoff.spec.ts` | Web offline invite and assignment need no acceptance; transferred-away work survives archive and fresh login until removal |
| 371 | `e2e/collaboration-offline-handoff.spec.ts` | Electron offline invite and assignment need no acceptance; transferred-away work survives archive and fresh login until removal |
| 372 | `e2e/collaboration-offline-result-notifications.spec.ts` | Web offline external participant logs in to information, acceptance and proven AI result notifications without source inbox duplication |
| 373 | `e2e/collaboration-offline-result-notifications.spec.ts` | Electron offline external participant logs in to information, acceptance and proven AI result notifications without source inbox duplication |
| 374 | `e2e/collaboration-retained-attachment.spec.ts` | Web typed human and owned AI creation stays atomic and idle from its first private upload through restart |
| 375 | `e2e/collaboration-retained-attachment.spec.ts` | Electron typed human and owned AI creation stays atomic and idle from its first private upload through restart |
| 376 | `e2e/collaboration-retained-attachment.spec.ts` | Web and Electron retain exact comment file bytes and immutable AI input after display removal, deletion, refresh and cross-client restart |
| 377 | `e2e/collaboration-selection-boundary.spec.ts` | AC87 Web and actual Electron hide subtask creation for shared body/comment selections while ordinary children retain it |
| 378 | `e2e/collaboration-source-surface.spec.ts` | Web and Electron source list, board, timeline and comments preserve external identity and scoped layout order |
| 379 | `e2e/collaboration-host-installed.spec.ts` | Linux packaged full Main controls a real native host run and explicitly resumes the shared Agent |
| 380 | `e2e/collaboration-installed-main.spec.ts` | installed Linux full Main and production Web hand off the same issue, preserve authority/history and reject unconfirmed Resume |
| 381 | `e2e/collaboration-host-runtime.spec.ts` | host Web and actual Electron share one ordinary Agent, preserve authors, and explicitly resume after scoped Stop |
| 382 | `e2e/collaboration-lane-close-controls.spec.ts` | Web same-lane agent and squad controls retain independent holds and whole-issue pause for new targets |
| 383 | `e2e/collaboration-lane-close-controls.spec.ts` | Web creator complete cancels every author's waiting inputs while live natural proofs remain unconfirmed |
| 384 | `e2e/collaboration-lane-close-controls.spec.ts` | Web creator cancel cancels every author's waiting inputs while live natural proofs remain unconfirmed |
| 385 | `e2e/collaboration-lane-close-controls.spec.ts` | Electron same-lane agent and squad controls retain independent holds and whole-issue pause for new targets |
| 386 | `e2e/collaboration-lane-close-controls.spec.ts` | Electron creator complete cancels every author's waiting inputs while live natural proofs remain unconfirmed |
| 387 | `e2e/collaboration-lane-close-controls.spec.ts` | Electron creator cancel cancels every author's waiting inputs while live natural proofs remain unconfirmed |
| 388 | `e2e/collaboration-lifecycle-controls.spec.ts` | Web creator removal keeps paused inputs and original natural proof through client and daemon reconnect |
| 389 | `e2e/collaboration-lifecycle-controls.spec.ts` | Electron creator removal keeps paused inputs and original natural proof through client and daemon reconnect |

## Execution entry points and constraints

- Runner: `/tmp/multica-specialized-acceptance-20261026/collaboration/environment/run-specs.py <pass-name> <spec basename>...`. It executes one spec file per invocation, `--project=chromium --workers=1 --retries=0 --trace=retain-on-failure`, preserving per-file JSON and logs under `results/<pass-name>/`.
- The runner supports `LANE_OVERRIDES` for a selected JSON override file and `LANE_RESULTS_ROOT` for an isolated evidence destination. It reads the private environment in-process; do not print its secrets.
- Serialize password/managed Linux-namespace groups with `host_execution_enabled=false`, and host-runtime/host-installed groups with the explicitly enabled host gate. Host cutover rejects new namespace roots, so those groups cannot share the gate concurrently.
- Retain authentication/registration budgets and original product assertions. Configured missing prerequisites must fail rather than be counted as successful skips. The administration pair additionally requires the two synthetic platform-admin identities.
- Record latest actual executions without rewriting earlier attempts or their provenance. Do not reclassify old failures as fixed merely because this audit found a newer source fix.

## Verification scope

Read-only ledger, raw result, source-byte and package-byte checks passed as described. No lint, typecheck, test execution or full product review was run in this audit because its ownership excludes source edits and test/service operation. Remaining platform/account/release gates stay with the active task.
