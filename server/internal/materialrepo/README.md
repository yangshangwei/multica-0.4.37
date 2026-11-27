# Material repository initialization kernel

This package performs deterministic, pure checks for an independent repository
initialization workflow. It does not read Git, execute a runtime, store state,
authorize a human decision, or establish that a claimed commit was merged.

## API

- `BuildCatalog(Binding)` returns twenty base files, sorted paths, per-file
  SHA-256 and a deterministic catalog digest. Relative root paths scope every
  generated file; only `material.yaml` substitutes the stable system ID.
- `ParseManifest(string)` accepts the supported schema/bundle/system/layout,
  with equivalent YAML formatting. Unknown fields/versions, duplicate keys,
  aliases, anchors, extra documents, custom tags and malformed input refuse.
- `Inspect(Binding, Snapshot)` classifies required paths and, only when there
  are no conflicts, creates a missing-files-only `Candidate`. Existing README
  content is preserved. Managed rules/templates require the exact known bundle;
  a manifest may differ in formatting, but must describe the same known format
  and identity. Already-compatible trees produce no candidate.
- `ValidatePublication(Candidate, expectedHead, Publication)` checks the frozen
  base commit, exact head, complete added-file diff and full resulting tree.
- `VerifyMerged(Candidate, actualMergeCommit, Snapshot)` returns byte evidence
  after verifying the actual commit's tree, required files and preserved data.
- `VerifyExisting(Binding, Snapshot)` returns evidence for a compatible existing
  tree without manufacturing a candidate, PR or human approval.

`Snapshot` must come from a trustworthy, independently authenticated read of one
immutable Git commit. It must include the complete repository tree, including
explicit directory entries. Each file needs SHA-256, and every required file
needs a complete UTF-8 body. Unmanaged regular files may omit their bodies while
retaining an independently computed SHA-256. An empty confirmed tree uses a
non-nil empty slice; nil, partial trees and symbolic commits such as HEAD refuse.
The caller must validate that the observation belongs to the authorized repo,
root, runtime, task generation and operation before invoking these pure checks.

Persist the exact candidate and its digest with the operation. Inputs are not
signatures: code able to invent a whole snapshot can invent its digest. Never
accept a model-created snapshot, proof or `complete: true` as trusted readback.
Bind review number/source/target branches and current human approval externally;
only use the actual independently observed merge SHA with `VerifyMerged`.

## Preservation and limits

Initialization is add-only. Existing custom AGENTS, templates or schemas produce
named conflicts and no executable candidate. Adoption/upgrade requires a future
independent maintenance contract; this package cannot silently accept or replace
those files. It does not widen the ordinary material publication allowlist.

The initial checker deliberately accepts only regular mode100644 files and
mode040000 directories. Executables, symlinks and submodules cause refusal,
including unmanaged ones; nothing is followed or rewritten. Case-folded path
collisions and missing/file ancestors also refuse. Trees are limited to10000
entries and32MiB of supplied data, with1MiB maximum per supplied file body.
Candidate admission checks the projected tree against those same limits before
returning any additions.

Publication and merged verification require the exact original tree plus the
catalog additions. Even unrelated concurrent target changes require a fresh
inspection; this conservative policy cannot silently expand the approved diff.
A no-op initialization uses `VerifyExisting`, not a manufactured merge receipt.

Templates are incomplete authoring aids, not business records. Six JSON Schema
assets document the structural format; this package validates the root manifest
and exact assets, not every future release/issue business instance. No schema
engine, credentials or live Git/network tests are introduced. Empty repositories
without a base commit must be handled by the owning workflow; the kernel never
creates a first default-branch commit.

Tests cover repeat inspection, retained user/test data, malicious/incomplete
observations, semantic YAML compatibility, immutable candidate mutation,
foreign-file diffs and exact post-merge preservation.

Run `node --test server/internal/materialrepo/schema-path.test.mjs` from the
repository root to verify the stored schema path patterns in ECMAScript. It
shares a fixed path corpus with the Go checker and requires no Node dependency.
JSON Schema character lengths do not replace the host's UTF-8 byte limits.
