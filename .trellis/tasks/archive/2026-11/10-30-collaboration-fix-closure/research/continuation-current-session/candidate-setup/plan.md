# Frozen current-source candidate preparation

The v11 full check completed with exit2. Its source, logs, bindings and live
resources remain retained evidence. This slice creates a new isolated candidate
for the main session's next complete check.

## Boundary

- Create only `/Volumes/artisan/code/2026/mc-collaboration-closure-v12-20261103`
  from the current shared HEAD plus the current tracked product/E2E overlays.
- Exclude generated `apps/web/next-env.d.ts`, all task/spec/journal edits, and
  unrelated untracked paths. Include the required recipient-picker E2E fixture.
- Freeze the two owned E2E fixes only after their implementer declares them
  stable; preserve exact file hashes and the candidate overlay diff.
- Keep same-lock dependencies in candidate-owned APFS copy-on-write directories,
  then verify actual workspace resolution. Do not install dependencies or run
  lifecycle hooks.
- Build the current CLI and desktop Main/preload/renderer using pinned Node,
  pnpm and Go, private output paths and a Git-external temporary directory.
- Do not edit shared product source, old v11 files, database state, services,
  Provider state, launcher bindings or acceptance records.

## Verification

Record the new candidate commit, source/configuration hashes, source preservation,
dependency manifest equality, actual resolved workspace graph, build commands,
exit codes and artifact hashes. Main owns all product tests, service startup,
full-check launch and final acceptance.
