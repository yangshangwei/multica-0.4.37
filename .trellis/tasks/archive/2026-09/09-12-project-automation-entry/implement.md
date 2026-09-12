# 项目自动化模板入口与预填 — implementation checklist

- [x] Read CLAUDE.md, parent contract and referenced spec manifests.
- [x] Inspect existing code and useful baseline tests.
- [x] Add meaningful failing regression tests for the new behavior.
- [x] Implement within owned files, reusing existing helpers and UI.
- [x] Run relevant tests, type/lint checks where applicable, fix observed failures.
- [x] Review against parent acceptance; report changed files, simplifications and risks.
- [x] Record exact commands/outcomes in evidence.md; leader owns final task state and commits.

## Verification commands
- `corepack pnpm --filter @multica/views exec vitest run autopilots/components/template-create-autopilot-page.test.tsx projects/components/project-automations-section.test.tsx`
