# Desktop account workbench integration — 2026-10-18

## Boundary and plan

The existing Desktop tab store rejects `/account`; boot and shell chrome depend on a successful workspace list. D10 requires a persistent authenticated account destination independent of that list. Extend the existing platform tab store in place (preexisting platform-store debt, explicitly approved by the parent); do not move mature workspace state or introduce another application store.

1. Add failing tests for v4 → v5 persistence, a single account tab that updates issue/view selection with history, workspace eviction preserving account, and coordinator restoration keys.
2. Add account/workspace/none scope and a separate account group. Preserve workspace tabs, pins, MRU and histories. Bind persisted tabs to server/viewer; clear previous-account state on identity change.
3. Wire the existing shared AccountWorkbenchPage through the sole router coordinator. Clear workspace mirror before account children render and guard stale workspace layouts from writing it back.
4. Keep account access available during workspace-list pending/error/empty boot, while identity/setup/password guards remain authoritative. Provide global account/workspace navigation and keep workspace recovery actionable.
5. Accept account deep links and account detail popouts through validated paths and native message readiness. Preserve existing workspace issue windows.
6. Run targeted Desktop store/coordinator/navigation/boot/native-path regressions, renderer typecheck, changed-file lint and diff checks. Parent owns native Electron acceptance, shared UI/core protocol work, specs and commits.

## Expected files

- Existing tab-store, coordinator, navigation, App boot, shell, tab bar and workspace route layout: own the missing routing/persistence behavior.
- Small account route component: platform wiring only; shared queries/UI stay in views/core.
- Native shared issue-window parser, preload/Main message wiring and dedicated-window integration: required account deep-link/popout entry paths.
- Adjacent tests: canonical persistence, navigation, boot and native validation evidence.

## Native acceptance handoff

Use an isolated profile and test-owned runtime config/API/CORS origin. Never launch installed daemon/AI CLIs; existing retained-languages-desktop fixture demonstrates isolation. Verify A → account → B; repeated issue links update one account tab; current/participating history and scroll restore; restart retains selection; workspace-list 500/zero workspaces retain account; setup/password remain blocking; account deep link cold boot and popout; server/viewer changes clear previous-account tabs; Cmd+W remains functional. No execution capability gate is enabled by this slice.
