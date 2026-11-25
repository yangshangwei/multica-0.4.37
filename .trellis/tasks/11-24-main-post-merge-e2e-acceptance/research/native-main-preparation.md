# Native Main preparation

This is L7 preparation for candidate `103e2bc329a9fb845d9f45c20b2a9bb3ebb54625`.
It does not establish a Main, daemon, installed-client, updater, or Provider
acceptance result. The parent session owns launch admission and live resources.

## Change boundary

The retained `current01` and `current02` stages both exited 1 before dependency
deployment. `/opt/homebrew/bin/pnpm` resolves to installed PNPM 11.13.1; the
project requests PNPM 10.28.2. The fresh private HOME has no PNPM package-manager
mirror metadata. Setting `npm_config_manage_package_manager_versions=false`
did not bypass that resolution in PNPM 11. No package was assembled by those
attempts. Their inputs, logs, and receipts remain unchanged.

The remaining preparation uses the existing installed production dependency
closure, including installed optional dependencies and peer links within the
selected production closure, rather than
installing or downloading packages. It copies dependency bytes to a new secure
directory, rebinds every package link into that directory, and excludes copied
development caches and `node_modules` before reconstructing the selected graph.
The existing electron-builder manual traversal collector then selects production
dependencies without a package-manager mirror or package installation.

Only task-private native preparation scripts, task-native QA/secure outputs, and
this report are in scope. Current Desktop `out/`, resources, source, Web assets,
live runtimes, and historical evidence remain owned by the parent/other lanes.
The preparation records complete input/output manifests and archive member
integrity/bounds before any functional admission. New labels preserve failed
attempts instead of replacing their evidence.

## Verification to finish

1. Test dependency isolation/refusal and ASAR corruption detection with synthetic
   files, then record current script hashes and syntax/typecheck results.
2. Freeze current source, all 495 Desktop output files, resources, installed
   dependency bytes, tool versions/hashes, and builder configuration.
3. Save a read-only exact macOS protocol snapshot before package assembly.
4. Assemble one stable task-owned arm64 `.app` using existing Electron bytes,
   no dependency rebuild, no publishing, no notarization, and no Main launch.
5. Validate every packed/unpacked member, archive bounds, exact product output
   bytes, CLI bytes, executable/ASAR/package hashes, and protocol snapshot after
   assembly. Preserve any failed package for diagnosis.
6. Save all 26 provider overrides as absolute missing task-owned paths, plus a
   separate native environment that selects only the task-owned discovery fake
   for Claude. OMP and DSH must be explicitly present.
7. Give the parent exact preflight/restore commands, bound review inputs, and
   unresolved launch barriers. No protocol setter or unregister is executed by
   this preparation lane.

## Retained preparation evidence

The initial `preparation-checks-01.json` records Python syntax, Swift typecheck,
and 17 fake-only self-tests passing. Its hashes precede the small environment
change in `prepare_native.py`; a fresh verification record is required for the
final implementation. The two deploy receipts each retain exit 1 and the same
449-byte mirror error log. No original failure is counted as a pass.

Further results and exact evidence paths will be appended after preparation.

## 2026-11-25 continuation

The task-owned `current06` package now assembles successfully and passes complete
ASAR/product-byte verification, deep strict macOS signature verification and the
bundled CLI's actual version command. The current05 signing failure was caused
by duplicate-expanded framework links in the installed Electron cache; a fresh
private mirror restored only14 proven byte-identical links while preserving743
resolved files and the original cache. No application was launched.

See `native-continuation-results.md` for the exact package, evidence, native
environment, unchanged canonical installed-host test, and protocol restoration
handoff. The L7 production Main journey remains NOT_RUN until the parent
coordinates its reviewed runtime and exact system-protocol cleanup.
