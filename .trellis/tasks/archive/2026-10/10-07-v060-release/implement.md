# Execution and evidence

1. Inspect current branch, prior fork releases, deployment config and existing runners.
2. Execute full `make check`; log to `.artifacts/v0.6.0/make-check.log` (session 21852).
3. Fix narrowly demonstrated release plumbing gaps; regression-test them. Write version-specific Chinese instructions.
4. Execute gated production Web/Desktop suites sequentially in isolated environments; inspect assertions, failures and skipped coverage.
5. Commit only task-owned changes using Lore messages and push this branch. Dispatch Windows native acceptance for version 0.6.0; download verified candidate and evidence.
6. Freeze release commit, prepare immutable cumulative changelog, build and rehearse Linux amd64 offline upgrade. Check guide bytes, architecture, image versions, data/config preservation and checksums.
7. Publish v0.6.0 and assets to the fork, verify downloadable bytes and version/commit identity, record final coverage and limitations. Archive task after all deliverables pass.

No initial blocker. Existing initial dirty paths are excluded from release commits; release builds may use an isolated clean checkout to preserve them.

## 2026-10-08 local package continuation

The user resumed work and explicitly chose to reuse this task. This continuation
delivers the Linux upgrade archive and Windows installer locally. Network and
Docker access have recovered. Reuse the accepted Windows run and its exact source
commit `082723894306372ec1dd46ea6c08934468681e7d` for both platforms; subsequent
commits only change tests.

No v0.6.0 GitHub release exists. Generate the local cumulative feed from the frozen
commit with `status=unreleased`, retaining verified published history. Do not mark
local preparation as remote publication. A later GitHub release must still follow
the canonical tag/CI changelog preparation and exact-byte rebuild procedure above.

Verify the Linux archive and rehearse v0.5.5 → v0.6.0 in the retained disposable
amd64 Docker environment. Recheck downloaded Windows artifact hashes against
native acceptance evidence and update metadata. Deliver a checksum manifest and
a concise local handoff record with the actual verification limits.

## 2026-10-08 formal publication continuation

The user now explicitly requests GitHub Release publication. Retain the accepted
source and Windows bytes. Fast-forward the fork's `main` to the frozen accepted
commit, create v0.6.0 there, and push the tag to run the canonical release workflow.
Download its exact `release-changelog` artifact for a separate formal Linux build
and upgrade rehearsal. Preserve the prior local delivery unchanged. After CI and
artifact checks pass, upload verified installers, documentation and checksum
evidence to the fork's Release, with Windows update metadata uploaded last.
Download the published files to recheck their bytes before closing the task.

### New security gate after publication was requested

Release CI `37702847716` failed before changelog generation or publication.
The live advisory `GO-2026-6629` reports reachable `golang.org/x/text v0.40.0`
(`secure/precis`) and fixes it in v0.41.0. The old binary acceptance does not
authorize publishing that vulnerable dependency. Patch only the existing Go
dependency and reuse the already validated Windows candidate contract-test fix
from `7de86ce8e` in an isolated worktree. Re-run security/tests and Windows native
acceptance, then the canonical release and Linux upgrade checks on the new SHA.
The user is choosing a new patch release versus explicitly replacing the newly
created, never-published v0.6.0 tag. Do not move a tag or infer this decision from
elapsed time. Preserve prior artifacts as historical evidence, not publishable
replacement binaries.

The user selected v0.6.1. The old v0.6.0 tag remains at `082723894`; v0.6.1
points to fixed commit `501f4b55f1675e59e46b86d094131d44bff1efd8`. Main CI
`37704376562` completed successfully. New release run: `37709633747`; new native
Windows acceptance: `37709646248`. All final outputs belong under
`.artifacts/v0.6.1/`. Preserve source-guide bytes and include version-specific
instructions explaining the older v0.6.0 filename examples where necessary.
