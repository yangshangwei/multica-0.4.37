# Detail journey current-task patch review

**PASS for the narrow harness change; runtime acceptance remains NOT_RUN.** The patch is not applied by this reviewer.

- Patch SHA-256: `c67d35c9f70ed55fb70f660d51330b04c427b62ce57141c6e50ec465a694f7b9`
- Original fixture SHA-256: `56181bfef4c16cddfee5daf5a9f1ecafe0f6f6e49a7016670dc608226a685219`
- Proposed fixture SHA-256: `6f5e204e07217cd3875324097d300cf2dfe33bb1ac282b1dc280c56c365d3766`

The change removes the historical task/path coupling and requires explicit current task plus an absolute allocation record. Both permit and allocation must declare the same task/checkout; the permit binds the allocation realpath and SHA-256. Existing origin, database, Build ID, source/artifact, listener, API health, served renderer, rendered API/WS and manual-only gate assertions remain in place. No business assertions change.

## Required guard cases before live acceptance

Exercise the actual exported `detailJourneyProvenance`, not copied conditions. Use temporary fixture records and deterministic module-boundary mocks; count listener/fetch/DB/auth calls. Early new-gate failures must occur before live boundaries.

- **D01 Valid new task and exact allocation** — Set current task environment; permit.task and allocation.task match; both source_root resolve to candidate; permit allocation path/hash match actual temp allocation bytes; all existing identities valid. Expected: Actual detailJourneyProvenance returns current task and reaches existing listener/health/read-only DB validation. No login or mutation is performed by this function.

- **D02 Required input absent** — Remove task, allocation env, or provenance env separately. Expected: Throws before listener, fetch, pg.connect and TestApiClient/login.

- **D03 Invalid task name** — Use absolute task, ../ traversal, nested path, or task string with separator after task slug. Expected: Rejects at new task format gate before any live side effects.

- **D04 Relative allocation filename** — Set allocation env to relative file path. Expected: Rejects before reading allocation or fetching services.

- **D05 Permit disagrees with current task** — Valid new task env but permit.task is old task; separately set runtime_permitted=false or wrong source_root. Expected: Rejects original permit identity gate before allocation/network/DB.

- **D06 Exact allocation path mismatch** — Permit points at one file; env points at a different resolved file containing identical bytes. Expected: Rejects resolved-file binding even though byte hash matches.

- **D07 Allocation bytes changed** — Hash original bytes in permit, then change allocated database or any byte without changing permit hash. Expected: Rejects new allocation digest gate before origin, listeners, fetch or DB.

- **D08 Allocation ownership mismatch** — Update permit hash to the fixture bytes but allocation.task or resolved allocation.source_root belongs to another task/checkout. Expected: Rejects allocation ownership; hash correctness alone is insufficient.

- **D09 Missing allocation binding metadata** — Remove allocation_file or allocation_sha256 from otherwise valid permit; use malformed JSON separately. Expected: Fails closed before service calls, without falling back to historic allocation.

- **D10 Existing provenance guards remain active** — With valid current-task inputs, parameterize wrong allocated endpoint/DB, credentialed or remote origin, changed source/artifact hash, wrong Build ID, different PID, wrong health commit/auth mode, served renderer hash drift, rendered API/WS drift and wrong DB gates. Expected: Each actual old guard rejects at its existing boundary; count preceding read-only operations and prove no authentication/mutation occurs.

- **D11 Canonical alias behavior** — Use two symlink aliases to the same actual allocation file, same bytes/hash; then retarget one alias to another file. Expected: Same realpath aliases are accepted as designed; different realpath rejects. This checks resolved identity rather than textual path equality.

## Handoff

After applying the patch in the candidate test-harness scope, update the harness source identity and issue new root provenance for the current allocation. Preserve original failure/blocker evidence. Run both unchanged Web/Desktop journeys only after contract checks pass and production services match the new permit.

Read-only static patch/hash review only; no tests, builds, code edits, services or database operations were performed during quiet handoff. Author syntax/apply-check evidence is preparation, not a runtime pass.
