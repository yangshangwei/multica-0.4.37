# P4 Core and shared Web/Desktop integration

2026-11-14; native `trellis-implement` child `p0_core_contracts`.

## Behavior delivered

- Knowledge reads negotiate v2. Original v1 write bodies and recovery requests
  retain header1 and their original receipt identity; no failed POST is retried
  with a different version. New human proposals select the currently read
  version, and exact decisions use the version of their displayed detail.
- Policy/job DTOs are strictly parsed, including real zero versus unknown/partial
  usage, fixed outcome codes, exact IDs/revisions and bounded job/attempt lists.
  Their server `KnowledgeProjection` leases use the existing Core cache,
  authorization fences, disconnect/hint erasure and guarded refresh. Unknown
  protocols and access denials stop interval polling.
- Project policy uses an awaited shared control for enabled/max1–3 settings,
  truthful system defaults, explicit concurrent-change recovery and original-key
  GET recovery after uncertain writes. Only current server grants enable edits.
- Delivery status matches the exact submission, accepted event and current
  project. It distinguishes processing, saved/restored/revised results, pending,
  configuration absence and reopen pause; no-project/manual flows issue no status
  request and show no fabricated processing. Authorized retry retains the exact
  job revision/request. Model usage shows provider-reported values without
  claiming verification.
- Candidate audit separates server AI proposals, system rule adoption and later
  human decisions, while keeping approved history distinct from paused usage.
  Source-unavailable/restricted projections keep their existing privacy boundary.
- Exact delivery links use the existing account route with
  `#delivery-{submission UUID}`. The delivery section opens and locates retained
  history, auto-loading up to10 existing pages before leaving explicit Load more
  continuation. It never substitutes the latest submission when a linked version
  has not been loaded.
- Human correction/derivation can select a delivery source already present in its
  candidate. Submit re-reads the authorized candidate and uses the exact server
  source reference/hash. No JavaScript reconstruction of Go's delivery hash and
  no source bodies in component state. Arbitrary fresh manual delivery selection
  beyond existing candidate references was not added.

## Changed files and reuse

The exact38 source/test/locale paths and end-of-slice hashes are in
`p4-owned-files.json`. They are limited to Core API/collaboration/paths and shared
Views collaboration/project/locale code. Existing work in those files was kept.

The implementation reuses the existing lease/cache lifecycle, schema parser,
scope guards, recovery conventions, Base UI controls and account navigation.
Policy/job recovery stores content-free routing identities only; original payloads
remain in memory, and reload recovery never reconstructs or automatically sends a
request. Previous ambiguous attempts survive later storage failures.

Both `apps/web/platform/account-workbench-route.tsx` and the Desktop equivalent
already use `parseAccountPath` and pass the complete destination to shared views.
Desktop's Main/preload sanitizer also round-trips through the same Core parser.
No app-specific view copy or new global route was needed.

## Validation

Final captured runs:

- Core knowledge plus account-route tests: **14 files /113 tests passed**;
  `p4-core-tests.json` and `p4-core-tests.log`.
- Shared Views knowledge, policy/status, delivery, collaboration/project wiring
  and locale parity: **6 files /156 tests passed**; `p4-views-tests.json` and log.
- `pnpm --filter @multica/core typecheck` and Views typecheck: passed, separate
  `p4-*-typecheck.log` files.
- Scoped Core/Views ESLint: zero errors. The new delivery-effect dependency
  warning was fixed; final touched-UI lint is clean. Two pre-existing warnings
  remain in `project-detail.tsx` (`project` and `updateProject` dependencies).
  Comparison against the P0 file backup confirms this slice only adds its policy
  import/mount there, preserving those unrelated effects.
- Impeccable Operate/craft guidance was applied to the existing design. Its single
  final mechanical detector run returned `[]`; `p4-impeccable-detect.json`.

New tests first exposed missing version headers and uncertain-retry storage
cleanup. Those were corrected. Initial broader Views failures came from old
two-argument recovery expectations and a composition fixture without a real
QueryClient; the actual status behavior remains tested with real Query/Mutation
wiring in `knowledge-automation.test.tsx`, while the composition suite mocks that
sibling deliberately. A temporary test filename collision was corrected before
implementation by moving the new status-schema tests to their own file and
restoring all20 earlier candidate-v2 tests; those continue passing.

## Remaining integration evidence

Root still owns independent Trellis review, production Web/native Desktop visual
journeys, actual controlled daemon input/reopen/reacceptance integration, full
repository checks and P5 documentation/generated-help synchronization. These
component tests and the mechanical detector do not claim those journeys passed.
No real account/model, commit, push or deployment was performed by this slice.
