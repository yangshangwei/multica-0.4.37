# Test fixture repair boundary

The current acceptance fixtures assume a historical task path and pre-delivery UI. The product already exposes Submit result / Result / Basis, explicit submission-bound acceptance, and secondary workflow actions under More actions. A managed Desktop login can start its own daemon even when the separate login auto-start preference is false.

Owned edits: root e2e/fixtures/collaboration-detail-journey.ts (explicit current task/allocation binding); e2e/collaboration-installed-main.spec.ts and e2e/fixtures/collaboration-installed-main.mjs (current UI, exact delivery assertions, stage-correct daemon identity). The two handoff-chain files already have equivalent current-contract changes committed in root; reuse them rather than overwrite. No product, dependency, running checkout or active service changes.

Preserve source/hash/PID/DB guards, original status/history/permissions/no-AI assertions, current startup autoStart=false and initially stopped daemon, all explicit Stop/Resume assertions. Post-login running is accepted only after a successful real form login and exact private profile/config/user/PID/health identity. Snapshot/gate changes remain root-owned.

Original failures remain immutable. Pure provenance red/green and static collection/node contracts precede root-owned unchanged-business E2E replays after the frozen harness is explicitly updated.
