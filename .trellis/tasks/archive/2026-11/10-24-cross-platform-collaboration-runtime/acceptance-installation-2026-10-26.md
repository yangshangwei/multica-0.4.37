# Linux arm64 installer acceptance continuation

Completed locally. Final actual executions passed without retries/skips:
Debian-installed full Main in 30.418 seconds and the default directory full
Main in 26.853 seconds. The unchanged accepted executable/ASAR/CLI hashes were
rechecked after both runs. See
[the source-specific result](research/deb-install-acceptance-2026-10-26.json)
and [independent review](research/installer-review-2026-10-26.md).

The first offline installation failed on three absent runtime dependencies;
a separate task image supplied them, and the fresh-container offline preflight
then passed. The first full installed-client run found a new assertion that
missed the CLI's existing leading `v`; the comparison now follows the existing
normalization, preserving raw versions. Both initial failures remain retained.
The cleanup review also found and repaired a daemon-close rejection path that
could skip closing Main. Boot observations and failure evidence are preserved
through the corrected nested cleanup.

Owned services stopped successfully. All isolated gates are false (epoch 7),
ports 19180/14100/6274 are closed and no owned Main/preflight containers remain.
Data, package, images and raw attempts are retained. The broader task remains
in progress for the other platform/account/delivery gates below.

The resumed control inventory is already complete: identity 31/31 and
collaboration 58/58. Independently audit those retained results; do not rerun or
relabel the inventory to imply a new full-source verification.

## Existing S6 gap and boundary

The accepted Linux artifact is an electron-builder directory package. It does
not yet prove installation through an actual OS package manager. Continue S6
with a Linux arm64 `.deb` built from the exact accepted prepackaged bytes and
installed into a fresh UUID-owned Docker container. The current cached Debian
arm64 image contains `dpkg` and the existing full Main browser prerequisites.

The installer fixture owns this gap. Expected source edits are limited to
`e2e/fixtures/collaboration-installed-main.ts`, its `.mjs` controller, and
`e2e/collaboration-host-installed.spec.ts`: an explicit `.deb` opt-in, actual
`dpkg` installation, installed-file/package provenance, and the same full Main
control assertions. Preserve the default directory/volume behavior and the
original case title. Missing configured prerequisites must fail. Keep endpoint,
real-password-form, current ASAR/CLI, Stop/restart/Resume and audit assertions.

No application behavior, dependency, production authentication/admission,
timeout budget, public release or real Provider-account changes are needed.
macOS full Main remains a separate gate: production configuration/preferences
currently use the real OS home and are not isolated by `--user-data-dir` alone.

## Sequence and evidence

1. Reconcile all original results and source/artifact hashes independently.
2. Package the pinned Linux arm64 directory with electron-builder
   `--prepackaged`, `--linux deb --arm64 --publish never`; retain command,
   version, architecture, source, three original artifact hashes and `.deb` hash.
3. Inspect and install the `.deb` in an isolated container; assert package status,
   actual executable/ASAR/CLI paths and hashes, desktop entry, protocol/icon
   metadata and standalone CLI separation. Retain package-manager output.
4. Enable only the existing isolated collaboration fixture gates, verify owned
   API/production-Web readiness, and run the existing full Main host-control
   journey against the installed executable, with zero automatic retries.
5. Run the original directory-package path after fixture changes to protect its
   behavior. Run scoped lint/typecheck and a separate Trellis review.
6. Close all isolated gates, stop only owned services/containers, and preserve
   databases, input packages, original attempts and raw evidence.
7. Record the exact supported scope: Debian arm64 installer/client controls a
   separate native macOS test-owned Provider runtime. This does not prove other
   distributions/architectures, real Provider accounts, updates, signed macOS
   installation, native Windows or production release.
