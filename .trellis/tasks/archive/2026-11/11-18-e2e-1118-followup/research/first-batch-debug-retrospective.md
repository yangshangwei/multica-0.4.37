# Bug Analysis: ordinary issue reads and history disclosure lifetime

## 1. Root Cause Category

- **#37 — E: implicit assumption; D: coverage gap.** The mutation assumed that a currently visible UUID detail row would survive normal query cancellation. An identifier can seed initialData after the UUID fetch starts; cancellation restores the earlier empty state. Older in-flight detail responses also publish stale Todo while an ordinary PUT is pending.
- **#38 — B: cross-layer contract; E: implicit assumption.** Workspace/UUID component keys separate entities but cannot retain client intent when the platform replaces and remounts their parent page segment. A real user can expand history after replacement starts and before it commits.

## 2. Why earlier attempts were insufficient

1. #37's first endpoint-match interception could select the reaction read rather than the detail observer. It did not establish which query's old result was held. The final browser regression holds every naturally started pre-write UUID GET and leaves request counts diagnostic.
2. #37's one-line exact cancellation protected old responses but erased late-seeded initialData. Real production QueryCache events, then a real canonical-hook regression, proved the empty-fetch → late Todo → cancellation-empty → reseeded Todo sequence. Conditional restoration of the already loaded row fixed this without changing default cancellation semantics. `revert:false` was rejected because it introduces a CancelledError/read error state.
3. #38's original test passed in the new environment and could not be called a RED. Independent page/history response gates reproduced the actual native-open failure twice. Recording only the replace-call instant or adding a controlled open prop inside the remounted local component would miss the user's later click or lose its owner.
4. #38's early parent-link locator matched two legal links. Its negative fixture failure was kept separate from the later frozen, unique-AppLink product regression.

## 3. Prevention Mechanisms

| Priority | Mechanism | Concrete action | Status |
| --- | --- | --- | --- |
| P0 | Cache contract | Preserve only the loaded workspace/UUID row when exact cancellation removes it; retain existing rollback and bound-completion authority | Done |
| P0 | Route ownership | One transient current-issue Core record per mounted navigation surface, StoreApi-only injection, pathname and generation fences | Done |
| P0 | Real boundary tests | Real QueryClient/canonical hooks, delayed natural reads, real RSC commit/remount and A→B AppLink | Done |
| P0 | Session lifecycle | Mounted-store registration and real clearClientSessionData integration; old-session callbacks cannot repopulate state | Done |
| P1 | Native verification | Production Electron preload/renderer/router plus real in-app A→B and existing bilingual iteration files | Done |
| P1 | Evidence discipline | Unchanged original tests, RED/GREEN hashes, fresh production builds/database/reports, one worker and no retries | Done |

## 4. Systematic Expansion

- Similar risks occur at aliased detail reads and remounted routed disclosures. Check identity, observer lifetime and the actual query's revert state before adding a local UI projection or global registry.
- Keep embedded inbox/dialog state local. Route intent retention must not broaden into visited-entity persistence, server-data duplication or permission bypass.
- Query owns historical rows and cancellation. Transport lifetime is not the disclosure contract: a remount may cancel and re-read the same issue.
- An original flaky title passing once is a control result. Establish a stable failure with independently controlled boundaries before implementation; keep fixture/collection errors distinct from product failures.
- Native MemoryRouter assertions use active virtual tab paths. Renderer evidence does not prove packaged main or real-agent behavior.

## 5. Knowledge Capture

- [x] Core state-management contract: exact read cancellation and late initialData preservation.
- [x] Core iteration-operations contract: routed disclosure scope, StoreApi injection, session cleanup and required tests.
- [x] Views state-management contract/index: Core ownership and Context as dependency plumbing.
- [x] Child execution records and joint result: original failures, intervening attempts, final code/test fingerprints and verification limits.
- [x] Template applicability checked: this application repository has no `src/templates/markdown/spec` directory; no template mirror is owned here.

Only #37 and #38 are accepted in this batch. Existing PAY-241 S3 collection work and the #51/#52 creation-dialog task retain their own ownership and acceptance.
