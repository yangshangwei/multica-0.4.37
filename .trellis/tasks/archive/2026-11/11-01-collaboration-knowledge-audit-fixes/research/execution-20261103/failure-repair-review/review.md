# v13 failure-repair patch review

**Pass-with-notes; zero blocking findings.** Six selected patches apply cleanly to seven isolated copies of frozen `85ef5d20578f63c7c4f879f035acbd414e80f306`. All original/proposed/patch hashes match author records. Strict TypeScript noEmit/import checking of the combined overlay reports zero diagnostics. Candidate source and all runtime resources were not modified.

## Contract findings

- Boundary: the real external handler submits result/basis and creator accepts its exact ID before the original workspace-deletion checks. Final access is still read as the original handler, while immutable author and creator decision are explicitly retained. The21 wrong-role403 attempts, inventory/file/comment/deletion/queue/quiet checks remain. Rendered action inventory now includes hidden descendants and current role plus delivery-state semantics.
- Workbench: the former handler's real UI leave is a valid cross-account revision change after delivery hands responsibility to the creator. Service departure advances revision without replacing the pending result or creator authority. The original test's stated stale-lifecycle purpose remains: exact409, preserved form/input/submission, no automatic POST/GET, explicit use-current, fresh key, exactly2 lifecycle writes and no AI work. Description/evidence correctly name the new participant mutation; this does not claim pending-delivery transfer works or cover separate authority-loss remount behavior.
- Memory: real handler submission and exact creator UI acceptance establish closure. Concurrent-edit/merge/restore/history and closed purge/read-only invariants remain; the second original-key recovery case is unchanged. The memory anchor remains unassisted.
- Mention/preview: the current Send/preflight contract is tested with an actual new preview, no comment POST and unchanged comments/execution before explicit confirmation. Suppression, exact final receipt, grant revocation, replay deduplication, queue/notification/outsider and author assertions remain. Shared Web/native positive controls must run too.
- Viewport: native scrollIntoView handles all ancestors for screenshot setup. Actual scroll-root guard, two frames,ratio1,full geometry,wide/narrow dimensions and product assertions remain. No weaker visibility threshold or production navigation change.
- Preferences: exactly three previous waits are restored; the proposed file equals304f74dd3 and all five owning product files still match that source. This preserves the historical successful synchronization; it does not establish autofocus as the sole cause of the later failure.

## Required actual verification

`directed-inventory.json` lists all13 original failures plus two shared mixed-mention positive controls and the unchanged memory recovery case: **minimum16 unique cases**. The simpler unfiltered eight-file command includes one extra existing settings navigation control and should report17 cases. No browser test has run in this review.

Root may apply the exact hash-bound patches after rechecking the seven original hashes, then run directed verification with owned production Web/native/API/database evidence. All original v13 failures and its final identity audit remain historical. A new complete gate is still required after repair integration; syntax/typechecking and a future directed pass cannot replace it.

The exact patch manifest, reviewed file hashes, contract sources and verification scope are recorded in `patch-manifest.json` and `review.json`.
