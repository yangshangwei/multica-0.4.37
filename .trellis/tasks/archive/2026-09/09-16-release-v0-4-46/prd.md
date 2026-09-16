# Verify end-to-end business flows and release V0.4.46

## Goal

Prove that recent changes and core business workflows work end to end, then commit/push and publish V0.4.46 with an intranet upgrade installer and the requested Windows desktop installer.

## Requirements

- Cover both the recent v0.4.45-to-HEAD changes and the complete existing business test suite, including auth, workspace, issues, comments, settings, navigation, onboarding, agents, skills, automation/reporting, changelog and applicable platform integrations.
- Diagnose and repair failures before shipping. Do not weaken test contracts or represent skipped checks as passed.
- Verify the current implementation with real API/database/browser behavior and relevant static/unit/packaging gates.
- Release version is v0.4.46. Follow the established project release and intranet packaging conventions.
- Deliver a self-contained intranet upgrade installer, exercise a real previous-version upgrade, and verify persistence of data and deployment configuration.
- Deliver and install-test the Windows architecture requested by the user. Clarify whether x86 means ia32 or prior x64; report the actual binary architecture.
- Preserve unrelated pre-existing work, user data, credentials, and existing releases.

## Acceptance Criteria

- [ ] Full business E2E and recent-change regression evidence is current, covers actual release source, and has no unexplained failures.
- [ ] Relevant lint, typecheck, unit/component, Go race/static, and packaging checks pass.
- [ ] Native/platform-specific checks and any exclusions are explicitly accounted for.
- [ ] Verified source and release metadata are committed and pushed; v0.4.46 resolves to the intended main commit.
- [ ] Published release contains the intranet upgrade installer, Windows desktop artifact, and matching release notes/feed.
- [ ] Offline upgrade from a prior release succeeds with data/configuration persistence; installer archive and checksums are verified.
- [ ] Windows installer succeeds in the matching native Windows environment, and its runtime/CLI versions match v0.4.46.
- [ ] Published asset hashes match local deliverables and user receives usable download links, local paths, and a concise verification report.
