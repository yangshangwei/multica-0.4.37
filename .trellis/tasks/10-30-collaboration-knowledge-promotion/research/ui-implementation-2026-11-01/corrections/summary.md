# B2 recovery and audit correction evidence

Ready for main's independent recheck; this lane does not close the review findings or claim screenshot/platform acceptance.

## Recovery contract

Raw ApiClient methods and receipt schemas are unchanged. The Core action helper now exposes two narrowly scoped outcome errors:

- `KnowledgeUnusedIntentError`: the current intent is proven unused. Its reason distinguishes preservation before POST, local validation before dispatch, and an exact recognized unsealed rejection. Only an operation with no previous stored identity, no earlier dispatch, and no overlapping dispatch can receive this classification. The unused identity is retired; `cleanupFailed` reports a local removal failure without converting the known no-write result into an uncertain write. The UI retains human draft fields and permits explicit editing/cancellation/new intent generation.
- `KnowledgeReceiptCleanupError`: contains the already validated, content-free receipt when local removal fails after a known server result. The UI applies the known success/refusal feedback, releases that completed intent, and separately reports cleanup trouble. It does not offer another POST for the known outcome. Session-local settled request IDs prevent the retained local entry from being mislabeled unresolved; after reload, any remaining record is recovered by GET.

Stored recovery identity remains content-free and unchanged in shape. Its prior existence conservatively covers a reload; a QueryClient-scoped dispatch counter also protects earlier attempts if storage is replaced and guards overlapping calls. Prior transport/commit ambiguity stays on the same UUID/payload through later 4xx responses, absent results, and pre-POST storage failures. `idempotency_conflict`, unknown/malformed errors, 5xx, stale-auth responses and GET not-found do not establish an unused intent. There is no automatic replay or replacement candidate.

The existing ten-second source-reader deadline and immutable reviewed revision/hash behavior are unchanged.

## Audit facts

The live review displays proposer ID when no lawful local name is available; known source creator/handler names still take precedence. Approval actor/time remain visible after later decisions. Latest decision actor/time render separately unless exactly identical to the approval facts, and available-source decision reasons render once as plain text. Source-unavailable projections show permitted audit identity/time but no reason; restricted projections still show no proposer/approval/source facts or body. No directory lookup was added. The version dl label now identifies the actual version rather than calling it state.

## Verification

Initial regressions reproduced **2 Core failures and 3 UI failures** before the fixes (with 23 other tests passing, zero skipped). The focused green runs contain **14 Core action tests and 17 UI interaction tests**, including additional overlap, storage and deduplication regressions.

Final affected checks:

- **25 Core tests passed** across action and API suites.
- **83 View tests passed** across knowledge interactions and locale parity.
- Zero failures or skipped tests in both runs.
- Full Core and Views typechecks passed.
- Scoped Core/Views lint passed without errors or warnings.
- Scoped `git diff --check` passed.

`verification.json`, exact `*-command.json`, red/green/final logs and JSON reports retain the execution evidence. `changed-source-hashes.json` records the eight corrected files; `source-manifest.json` records the complete 22-file B2 plus correction source set. The original B2 implementation evidence in the parent directory is preserved.

No Go, DB, provider/model, router, dependency, commit, detector or screenshot actions were performed. Main owns the existing single detector result, independent visual review and production Web/native Desktop acceptance.
