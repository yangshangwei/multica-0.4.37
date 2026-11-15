# V12 current-source candidate preparation

Candidate `b6c74033f1913e46418ec1b119da33236dcba805` is frozen at
`/Volumes/artisan/code/2026/mc-collaboration-closure-v12-20261103` on
`codex/collaboration-closure-v12-20261103`. It contains committed shared
HEAD `084ea3c987fa4ab8418a2ae5b3b7a20c096c36d8` plus 12 exact product/E2E
overlay files. Generated Next declarations, task/spec/journal changes and other
untracked files were excluded. The required recipient-picker fixture is included.
`overlay.json` and `overlay.patch` are the exact review inputs.

The scoped commit follows Lore and does not claim complete acceptance. No shared
product source, old v11 file, service, database or Provider state was changed.

## Dependencies and source identity

All 14 dependency-defining inputs equal the retained owned v11 candidate. Eleven
dependency/SDK directories were APFS cloned with `cp -cR` into distinct candidate
paths and inodes. No install, lifecycle hook or network access was needed.
Eight sampled package/native/tool files retain their exact bytes with distinct
file identities. All 22 declared workspace edges and 66 actual exports resolve
inside the new candidate, with one physical core/auth source.

APFS copying preserves generated `.bin` wrappers' absolute `NODE_PATH` prefixes.
Only those 607 ignored generated wrappers were rebound from donor to candidate
paths; their original/new hashes are in `dependency-bin-rebind.json`. Actual
package source, lockfile and copied pnpm SDK bytes were not changed. Copied
`.modules.yaml` still describes the historical donor store location; no install
or store lookup was performed. Do not install into this frozen verification
layout without a new preparation step.

Before/after snapshots both contain 17,535 identical tracked file/hash/mode rows,
fingerprint `f7f70cfab0a9910ae06b3007a83a0f8f2cff0da9041d34c6ca0938ede6acff02`.
The development checkout source identity is
`00d0c15ba23200fa0f0025966b4214e6e018f6e32f356f06ec2746705ba46f50`.
Main separately computes the canonical product manifest.

## Builds and checks

Pinned Node22.23.2, copied pnpm10.28.2 and Go1.26.6 were used with fresh private
homes and `/private/tmp/multica-final-check-v12-fpzft_b8`, outside Git. The
existing builder policy was reused with only candidate/hash/output rebinding;
its sandbox denied network and source writes. CLI and desktop builds both
exited0, and source remained clean.

- Current CLI: `.omx/private/final-check-v12/multica`, SHA256
  `b9138d86da10b1e5d9e4c859d131ee38132d8b2b5d3a2073031d77d0f54e24c9`;
  `--version` reports commit `b6c74033f`.
- Current desktop output: `apps/desktop/out/` with Main1, preload1 and renderer493
  files. Distinct staged copies are under
  `.omx/private/final-check-v12/artifacts/{main,preload,renderer}` and match the
  installed layouts byte-for-byte; manifests are in `artifact-bindings-v12.json`.
- Private Python AST2, Node syntax2, actual pinned pnpm version and
  `git diff --check` passed.

Repository tests/lint/typecheck, production SSR, full Make, live runtime ownership,
post-run audit, password7 and model experiments belong to main. These builds
do not establish installer, packaged Main lifecycle or Provider acceptance.
