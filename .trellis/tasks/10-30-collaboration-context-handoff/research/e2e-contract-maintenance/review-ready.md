# E2E contract patch ready for independent source review

Worktree `/Volumes/artisan/code/2026/mc-e2e-contract-maintenance-20261102`, branch `codex/e2e-contract-maintenance-20261102`, base `c3db64b24fea7116622d106ce2231853e1be72f9`. The code patch is committed as `1ecabb96457403d9601752f37eab89934265866c` and confined to 11 E2E spec/helper files. Root's read-only reviewer can inspect `git diff c3db64b24 1ecabb964`; independent review is still required before local integration.

The three current canonical failures were reproduced on the unmodified target contracts after preserving an earlier setup/CORS failure. Repairs preserve business, permission, bytes, idempotency, revocation and notification assertions: current exact recipient picker interaction, exact stability excluding only validated authority evaluation time, and paged-record selectors with direct first/next response/cursor checks. Existing handoff workflow selection retains its confirmation wrapper. The runtime specs now have strict password-only lane guards and ENV-1 has positive persisted/API/UI assertions as well as the negative misattribution check.

Current actual results, all worker 1 / retries 0 and API/renderer `c3db64b24`:

- `repaired-legacy-10`: 9 pass, 1 intentional password skip (Leave), 0 fail.
- `repaired-password-5`: 3 retained-attachment pass, 0 skip, 2 failures from missing environment prerequisites. Leave's first Resume receives `503 collaboration_execution_unavailable`; source-surface reaches its ordinary source CLI check after the changed selector, but lacks `COLLABORATION_SOURCE_CLI`.
- `runtime-password-3`: 3 pass, 0 skip, 0 fail, including positive canonical ENV-1.
- Root then completed its task-only protocol gate and source CLI prerequisites. `repaired-password-final-2`: **2 pass, 0 skip, 0 fail**. No active browser run remains. **The historical target inventory is now 14 pass / 0 pending**, plus 3 runtime/password passes. All target passes bind to the same committed patch bytes; the historical failures/skips remain separately preserved in `verification-runs.json`.

Strict typecheck of changed E2E sources passes; repo base ESLint with test fixtures correctly classified for declared devDependencies passes all 11 files with zero errors/warnings. The expanded strict diagnostic set has 51 existing nullability errors only in the unchanged desktop spec and is exactly identical to the base commit. `static-validation.json` and `typecheck-baseline-comparison.json` preserve these boundaries. No dependency, timeout, retry, product/API/CORS, installed Agent CLI or Provider operation was changed. Provider calls remain 0. Root owns independent review, local integration, source rebinding and final same-candidate full acceptance.

`acceptance-ledger.json` maps all 14 historical case keys to actual passing commands, lanes and hashes. `ENV1-facts.json` preserves the positive persisted/API facts, `root-gate-amendment.json` preserves the coordinator-owned gate change, and the ledger records the actual CLI hash observed by both clients. This bounded result does not close parent release or real-model acceptance.
