# V0.4.46 execution and verification

- [x] Audit current remote / local source and prior successful packaging procedures.
- [x] Enumerate full business flow coverage and recent-change coverage; identify conditional skips.
- [x] Create isolated local QA environment with migrated database and production Web server.
- [x] Run lint, typecheck, frontend unit/component suites, Go race tests and static analysis, and packaging script tests.
- [x] Run full Playwright business suite and targeted recent navigation/settings/onboarding/localization regressions.
- [x] Run real Electron fixture smoke and visually inspect the changed flows with screenshot evidence.
- [x] Reproduce and fix defects, add meaningful regression coverage, and rerun affected gates.
- [x] Review the release diff and identify the existing tag-driven version/changelog/feed contract.
- [x] Commit verified changes using Lore trailers and push the main candidate.
- [ ] Validate Windows x64/ia32 packaging and installation on the pushed candidate before creating the immutable tag.
- [ ] Create v0.4.46 and publish through the established release pipeline, rebuilding tagged desktop artifacts.
- [x] Build the requested offline intranet upgrade installer and Windows desktop installer; validate contents and native Windows installation.
- [x] Perform real previous-version to v0.4.46 offline upgrade with data/configuration persistence checks.
- [x] Verify published release assets against local SHA-256 values, document limitations, and deliver download paths plus test report.
- [x] Final requirement-by-requirement audit; archive task / persist evidence and mark goal complete only when all requirements are proven.
