# Windows candidate acceptance implementation plan

> Execute the approved candidate-build and Windows CI acceptance scope in the existing platform-admin worktree.

**Goal:** Produce a Windows x64 candidate and native, independently inspectable installer/runtime evidence without publishing a release.

**Architecture:** Reuse the existing manual desktop-smoke workflow. Add an opt-in Windows x64 acceptance job, preserving the default multi-platform smoke jobs. The new job builds the exact checked-out commit, verifies a previous release asset by its published SHA-256, runs disposable per-user installer lifecycle checks and selected native managed-daemon tests, then uploads candidate files and reports.

**Scope:** Current installer, old-to-new installer upgrade, uninstall/reinstall, architecture/version/signature observations and selected managed identity/transport tests. GUI, full backend integration, real-user configuration retention and signed/offline certificate trust remain separate gates unless directly tested. No version/tag/release/production change.

**Choices:** Prefer Windows hosted CI for authoritative native execution. A Mac cross-build is useful supplementary evidence but cannot validate Windows runtime behavior; Wine is not treated as a Windows acceptance substitute. Reuse existing packaging and installed-CLI smoke contracts rather than introduce dependencies.

1. Add bounded PowerShell lifecycle and parser/fixture tests under apps/desktop/scripts; refuse non-disposable or pre-existing Multica environments before mutation.
2. Add opt-in Windows x64 workflow input/job, native script checks, selected daemon tests with JSON result validation, exact-version packaging and optional previous release download with checksum validation.
3. Validate scripts/workflow contracts and cross-compile Windows CLI/test binaries locally. Attempt candidate assembly in an isolated checkout if useful; retain failures and provenance.
4. Review and commit the concrete workflow/scripts. Publish only a CI test branch and dispatch the existing manual workflow against that exact ref; do not merge/tag/publish a release.
5. Observe CI, fix bounded failures and rerun when necessary. Download final artifacts, verify hashes and source/run identity, update S07 report with real passes and untested gates.
