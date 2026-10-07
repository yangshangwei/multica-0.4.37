# Release design

Reuse existing build, installer, changelog, update publication and acceptance tooling. Keep the branch named by the user. Publish exclusively to the fork with explicit repository arguments.

Change boundary: release plumbing, deployment configuration and operator docs, plus any demonstrated acceptance-blocking defect. Preserve app behavior and unrelated worktree changes. Configure existing feature flags at their deployment boundary rather than changing their backend defaults.

Run `make check` before publication; run dedicated environments for incompatible authentication modes and feature suites. Never overlap production Web builds in this checkout. Windows acceptance runs on the hosted Windows runner, using a supplied 0.6.0 candidate version with `--publish never`.

Prepare the cumulative changelog from the frozen release commit and verified previous fork history. Build the Linux amd64 archive with those exact bytes. Collect checksums and native acceptance evidence before releasing assets. Preserve prior installation credentials and require explicit, documented enabling of optional deployment features.
