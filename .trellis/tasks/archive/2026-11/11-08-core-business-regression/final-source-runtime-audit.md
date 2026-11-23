# Final source, package and runtime audit

Final read-only audit completed after root stopped the owned services. **All current production bytes/modes match the final v10 package build; both full payload inventories match; owned services and temporary execution resources are stopped.** Current test results remain separately classified, including original failed full runs.

Evidence: `/Users/artisan/.multica/qa/core-20261108/final-runtime-audit-v4/source-and-payload.json`, SHA-256 `db90925e1ce5a16d94dc105f707e29b171ce161e70b88936d88e8f88a7acb68f`. Cleanup is `final-runtime-audit-v4/cleanup.json`, SHA-256 `9a016d50031df244d3eaf33f260135ff401a83d9dd210bc54e787d435ab0a335`. Earlier v1/v2/v3 JSON and report bytes are preserved; `report-before-v4.md` preserves the immediately previous report.

## Exact current source

Root HEAD is `7843dd91aeb79c964e9d0bbb129c67b9411e6427`; QA and compiled runtime HEAD are `6201a58102668ce19e5f2959ea7dfb807d932a43` plus the recorded final dirty runtime overlay. The root's later two commits modify tests only. QA now contains those exact test changes as well.

Both current root/QA differ from `packaged/source-files-v10.json` only in these four E2E paths:

- `e2e/collaboration-account-desktop.spec.ts`: native focus readiness and failure-only author diagnostics.
- `e2e/fixtures/collaboration-command-history.ts`: explicit review of a genuinely stale deletion confirmation, original response/immutable-history assertions retained.
- `e2e/core-business-journeys.spec.ts`: member-page readiness before opening the status picker.
- `e2e/fixtures/collaboration-preparation-records.ts`: scope the result assertion to recorded entries.

Every before/after hash and mode is captured. All production source bytes/modes remain identical. One explicitly named pre-existing root `server/.omc/.../hud-state.json` has unchanged bytes but a private file-mode difference and is tool state, not product source. No generic ignored-file rule was used.

The two later root-only test commits were synced and replayed as `concurrent-test-regression`: 7 PASS in29.8s. The repaired native/Web command-history pair passed the complete 12-case replay `delete-final12` in79.491s with zero retries/skips; a separate real-write stale-confirmation control proved the repaired branch. Original full-run failures remain failed and are not relabeled.

## Backend and package binding

All current root/QA hashes match the nine entries in `backend/product-final-hashes.json` (eight production implementation/query/generated files plus one service regression) and all13 paths in `backend/final-source-audit-v3.json` (nine modified tracked, four new). Thus the batched count/participation fixes and final fixture classifier are the verified current bytes.

Final v10 build manifest: 27,612 files, SHA `29e2d9c6af8954a6104b0ae0abf58d061b266c00834a069dc1c5fde97d00a11a`. The actual mac-6/Linux-5 archives were rehashed in full: **745 macOS files and80 Linux files** match exact bytes, sizes and modes. Both current final packaged journeys are terminal PASS: native-alias-final37.336s and packaged-linux-final115.488s, zero skips/retries and no cleanup errors. Their original result/final hashes are copied into this audit. This does not imply signing/notarization, installer/updater or commercial-model acceptance.

## Recorded live runtime, then verified shutdown

The prior live observation bound API PID2361 at18953, production Web PID91707 at13873 with Build ID `dAyxboJLhjS_IENeU-Bkj`, and renderer PID90538 at6047. Actual served renderer bytes, preload, rendered API `http://localhost:18953` and WS `ws://localhost:18953/ws` matched. Database identity was `multica_core_final_20261108232745_2192`.

The Web configuration mismatch is fully explained: current env is the build env's byte prefix plus only DATABASE_URL/POSTGRES_DB assignments; parsed browser-origin/CORS values are identical. Recomputed build fingerprint63a99910… equals the registry record; final database env fingerprintc137beb4… differs because path/full env bytes are hashed. No registry record was rewritten to hide it.

After root cleanup, read-only lsof checks show **18953,13873,6047,19080,19594 closed**. No restart was performed to recreate live evidence. The unrelated daemon on19602, PID48070, remains untouched. See `cleanup-report.md` for database/container/profile inventory and preservation boundaries.
