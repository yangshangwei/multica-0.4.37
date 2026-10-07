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
