# Full verification environment repair boundary

The full gate assumes Desktop renderer/preload and Collaboration API/native endpoints exist, but only builds Web and supplies NEXT_PUBLIC_API_URL. This is environment orchestration, not product permission or UI behavior.

Implement in scripts/check.sh and check.test.sh, with a small check-desktop-renderer.mjs helper and its focused Node test if required. Reuse existing registry allocation, launch_detached, listener ownership and stop_component. The helper uses the already-installed Vite preview API with configFile:false and explicit built renderer output, avoiding dev compilation or production Electron Main. Build via the existing desktop package command; bind Collaboration API/WS/source root and renderer to this check allocation; keep source env unchanged. Keep explicit input-origin validation, task-owned CORS and lifecycle cleanup. No dependencies, product edits, test skips, retry increases or real Agent opt-ins.

Tests must fail before implementation for missing build/order/configuration and renderer cleanup. Use test-only temporary files and loopback service for helper validation; no API/database writes. Parent owns Desktop build and subsequent focused E2E, independent review, commit and full03.


## Implemented and frozen

- `scripts/check.sh`: uses the registry slot's renderer port, appends allocated Collaboration API/WS/source-root/renderer and Desktop output paths to the copied env, preserves source env, includes owned renderer in allowed origins, builds Desktop before Playwright, and starts/stops the static renderer using existing process helpers. Readiness requires actual served index bytes and owned listener. Phase count remains6.
- `scripts/check-desktop-renderer.mjs`: serves only built output through existing electron-vite's Vite dependency with `configFile:false`; records PID, canonical path, origin and index SHA256; no Electron, daemon, new package dependency or build during serving.
- `scripts/check.test.sh`: adds Desktop build/start failure, ordering and cleanup assertions; stale API/source/build/port overrides; original env remains unchanged. Existing origin validation tests retained. Runs the new helper tests.
- `scripts/check-desktop-renderer.test.mjs`: real loopback checks for static HTML/JS and listener evidence/shutdown, absent build refusal, occupied port refusal without disturbing its owner.
- `.trellis/spec/web/frontend/e2e-run-environment.md`: records complete-gate native artifact and ownership contract, API mapping and password mock distinction.

The sibling check agent's `e2e/projects-p1-lifecycle.spec.ts` change was observed and preserved; it is outside this slice's ownership.

## Verification

All logs are under `/Volumes/artisan/code/2026/mc-merge-takeover-archives-20261108/verification/`:

- `check-env-before.log`: new orchestration regression fails on original script: Desktop build failure still yielded exit0 and Playwright execution.
- `check-env-after.log`: orchestration and existing origin tests pass after implementation.
- `check-renderer-helper.log`: first helper run exposed only macOS `/var` vs `/private/var` test-oracle mismatch; retained, then test corrected to compare canonical path.
- `check-env-after-02.log`, `check-env-after-03.log`: final whole check.test.sh pass, including3/3 real helper tests with0 skipped/failed. Latest run additionally checks stale source environment and renderer port overrides.
- `bash -n scripts/check.sh scripts/check.test.sh`, both new JS `node --check`, and `git diff --check`: pass.

Main built the standard Desktop package successfully and owns further API/Web/native focused verification. No API/DB/browser test was started by this implementer. Candidate files frozen after the successful script checks; no commit/full gate/merge performed here.

## Focused startup interface

Source `scripts/check.sh`, load the recorded source env, and call `prepare_check_environment`. Register `check_cleanup` before starting services. `ensure_database; migrate_database` sets up only the task API database. With corresponding CHECK_STARTED_* flags set before each start, use `start_api`, `start_check_desktop_renderer`, and `start_web`. The main already built Desktop; this focused path does not rerun static/Go stages or create a Go database. Run original failed browser cases, set CHECK_COMPLETE=true only after pass, and let cleanup stop recorded owned processes. Full `check_main` still runs the complete6-stage gate.

Supplying required Collaboration endpoints will activate additional human suites that previously had environmental skips. Their password/admin/installed/provider switches remain unchanged. Test counts can therefore expand; full-gate success must use the new complete run, not a count-preserving filter or skipped native cases.


## Independent-review follow-up: source-file renderer port

Reviewer found that resetting only the shell's DESKTOP_RENDERER_PORT before bind_paths was insufficient: the later copied-env reload could restore a stale assignment from the source file. Source freeze was honored through focused e2e-repair-01 (parent reports27/27 original failed cases passed, then owned services cleaned). After the parent lifted freeze, a regression added an actual `DESKTOP_RENDERER_PORT=19996` source-file assignment plus a distinct inherited shell value. Before fix, `check-env-port-before-02.log` fails with “Reloading copied env restored a stale source-file renderer port”.

The fix appends the allocated renderer port to the task copy alongside API/Web ports. Regression asserts the reloaded value equals the allocator's port, matches the manifest/renderer URL/allowed origins, and leaves the original source assignment untouched. `check-env-port-after-final.log` passes all orchestration/origin checks and3 helper HTTP tests; Shell/JS syntax and diff whitespace checks pass. The old Web-only spec section was rewritten to describe the default built native renderer and copied-env binding accurately. Candidate slice frozen again; no browser/full-gate rerun or commit performed by this child.
