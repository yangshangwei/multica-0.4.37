# Full end-to-end verification

Run all 114 Playwright cases at local main 4924ac74c using isolated production Web/API, Electron renderer/preload, task-owned CLI, changelog feed and a separate device-auth phase. Preserve unrelated main-worktree changes. Diagnose failures from evidence; update stale acceptance navigation without weakening business assertions. No remote deployment or real provider-agent execution.

Acceptance: every case executed, final pass/fail/skip accounted for, traces and report saved, environment failures distinguished from product/test defects, scoped fixes verified and committed.
