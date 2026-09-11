# Built-in Role Skill Localization Implementation Plan

**Goal:** Present the seven built-in role skills in Chinese consistently, with bilingual search and unchanged stored/runtime identities.

**Architecture:** A shared pure presentation resolver reads existing locale resources through i18next. A thin hook provides a locale-aware resolver to shared web/desktop components. Existing workspace queries supply provenance where compact agent summaries omit it.

**Tech stack:** TypeScript, React, i18next, TanStack Query, Vitest, existing Playwright/browser tooling.

## Work sequence

1. Add failing behavior tests for Chinese list/detail/picker presentation, English search, identity preservation, and language switching.
2. Add the seven locale entries to skills.json in en/zh-Hans/ja/ko, preserving the current English fallback text in non-Chinese locales.
3. Implement `skills/lib/skill-presentation.ts` and `skills/hooks/use-skill-presentation.ts`; add canonical pure tests for provenance and custom-content boundaries.
4. Integrate the resolver into `skills-page.tsx` and `skill-detail-page.tsx` without changing editable state or save payloads.
5. Independently integrate agent skill assignment/picker surfaces and any directly related invocation selector. Keep raw command values and UUIDs unchanged.
6. Run targeted skill/agent tests and locale parity; fix failures. Run the package typecheck and lint, then static analysis and relevant broader checks.
7. Review the complete diff and perform a bounded browser inspection of list, detail, and selector in Chinese and English. Save visual evidence/verdict under .omx.
8. Record any non-obvious project convention, inspect final git status, and report changes and verification accurately.

## Commands

- `pnpm --filter @multica/views exec vitest run skills agents/components/skill-picker-list.test.tsx agents/components/tabs/skills-tab.test.tsx locales/parity.test.ts`
- `pnpm --filter @multica/views typecheck`
- `pnpm --filter @multica/views lint`
- `pnpm knip`
- `git diff --check`

## Rollback

Revert only this task's source changes. No data mutation, migration, or content conversion needs to be reversed.
