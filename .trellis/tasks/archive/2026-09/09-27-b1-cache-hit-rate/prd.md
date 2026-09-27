# Accurate cache hit rate

## Goal
Reproduce the wrong rendered cache-hit percentage with cache writes, then port the six-file patch. Cache read / (plain input + cache read + cache write); zero input is unavailable; near-100 percent must not round to an unjustified perfect hit. Preserve two locales, existing cost semantics and no dependency additions.

## Source
Upstream commit: `470fd1fc0dcbbb8b86a8d13c6b4c43b6f2a10e1f`. The user approved this bounded backport.

## Allowed implementation/test files
- `packages/views/issues/components/issue-usage-dialog.test.tsx`
- `packages/views/issues/components/issue-usage-dialog.tsx`
- `packages/views/runtimes/components/usage-section.test.tsx`
- `packages/views/runtimes/components/usage-section.tsx`
- `packages/views/runtimes/utils.test.ts`
- `packages/views/runtimes/utils.ts`

## Acceptance
- Observable red/green regression evidence.
- Relevant focused tests pass, with logs and command recorded.
- No schema, dependency, locale expansion or unrelated production edits.
- Leader reviews and commits separately; this child must not commit or merge.
