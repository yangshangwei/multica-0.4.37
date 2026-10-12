# Merge readiness design

## Boundaries

The behavior gaps live in identifier recognition, credential-file publication,
and committed evidence assets. Work directly at those boundaries and retain the
existing API, authentication, transaction and UI contracts.

### Task references

Use the workspace prefix contract (1–10 ASCII alphanumeric characters) with a
positive issue number. Align the iteration parser and backend number extraction
with existing project/triage pickers. Reuse an existing suitable helper if present;
avoid introducing a new abstraction for unrelated reference formats. Keep UUID
parsing, case-insensitive deduplication, text search and limits unchanged.

Expected files: iteration candidate implementation/tests; backend issue search
parser and its pure table tests. A shared helper is justified only if existing
call sites already represent the same complete contract.

### Credential publication

Replace the two-operation hardlink publication/removal with a no-replace atomic
rename on supported Unix hosts: Linux `renameat2(RENAME_NOREPLACE)` and macOS
`renameatx_np(RENAME_EXCL)`, available in the existing x/sys dependency. Continue
using pinned directory descriptors and private staging files. Retain no-follow,
owner, mode and single-link validation plus final file/directory fsync before DB
commit. Fail closed if the filesystem cannot provide the required operation.

The Windows maintenance path remains explicitly unsupported. Do not weaken
single-link validation or remove arbitrary files to recover an old unsafe inode.
Use a subprocess exiting after the real publish syscall to prove the old failure
and the new retry behavior without defer cleanup masking the interrupted state.

Expected files: `server/cmd/server/platform_admin_credentials*` and narrow
platform-specific publish implementations/tests. No DB schema or service
transaction change is required. Existing old-version interrupted files remain
operator-inspected recovery cases, not silently trusted.

### Evidence images

Retain PNG names, dimensions and all documentation links. Prefer lossless PNG
optimization; if insufficient, use a high-quality palette encoding and inspect
text/contrast. Use installed image tooling only, with originals backed up outside
the repository while validating. Do not relax the CI budget or add exemptions.

### Draft delivery found during verification

Tiptap's editable DOM can receive an update before its deferred `create` event.
Use the existing pending debounce handle to keep `onCreate` from acknowledging
unsent user text as the emitted-content baseline. Preserve normal autosave,
the original Markdown base, and opt-in `flushPendingOnUnmount`; ordinary
cancel-by-unmount callers retain their discard behavior. Deterministic tests
hold the mock's create event until after input, then verify autosave and early
unmount independently. The original browser steps and assertions stay intact.

## Integration and release implications

The source already contains remote main. Recheck remote heads before pushing and
merging; incorporate new target changes if they appear. Use a normal PR merge
commit to preserve existing decision-history commits. Keep the feature branch.
This request does not require a VERSION file, release tag or deployment.

Concurrent release-documentation edits appeared in the original checkout during
implementation. Integration therefore uses the isolated sibling worktree
`/Volumes/artisan/code/2026/multica-projects-p1-merge-readiness` and branch
`codex/projects-p1-merge-readiness`, based on the assessed source commit. Only
owned corrections were transferred; parallel `.env.example` and v0.6.5 release
document edits remain in the original checkout. The PR still incorporates the
complete assessed projects-p1 history, with its corrective commits on top.

Existing branch behavior to document: migrations 567–570, default-enabled new or
missing iteration/triage settings with explicit opt-outs preserved, removed
FF_ITERATIONS_I1, Asia/Shanghai fallback for unconfigured planning timezones,
and guarded rollback after administrator initialization markers exist.

## Rollback

Each corrective change is a separate reviewable commit. Before merge, revert a
failed correction on the feature branch if necessary. Do not run application
database down migrations or automatically revert deployed services. All database
verification must use isolated fixtures/environments owned by this task.
