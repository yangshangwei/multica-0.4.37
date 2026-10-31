# Initial full-repository static verification

The requested commands ran sequentially in `/Volumes/artisan/code/2026/mc-runtime-integration-20261031`, with no source edits, Provider calls, DB operations, full TypeScript test suite, or commits.

| Command | Exit | Duration | Turbo success | Executed | Cached |
| --- | --- | --- | --- | --- | --- |
| `pnpm lint` | 0 | 13.858s | 6/6 | 3 | 3 |
| `pnpm typecheck` | 0 | 72.034s | 9/9 | 4 | 5 |

Lint reported 31 warnings and zero errors: Desktop 1, UI 3, Views 27. No automatic fixes were run. Typecheck reported no type errors. Cached tasks replay their original console paths, including the main worktree; those paths are not claims of new execution there. Exact task cache events are retained in `summary.json` and the raw logs.

The source scope includes tracked and untracked files under apps/packages/e2e plus root dependency/Turbo/TS/ESLint configuration. All 3,586 normalized source hashes were unchanged before and after the checks. The only permitted normalization is the documented `apps/web/next-env.d.ts` production/dev route-types import; that file was not restored or otherwise edited. Source aggregate SHA-256: `011d8b2a5051ab77cd74df80a47551e7d4c9d8f2681117b9bc7f05b46f2079a3`. Base HEAD: `e3620721efacb820b89709c8db9d326d9cf2b00b`; the source hash manifest identifies the uncommitted integration snapshot.

This is initial static validation, not final acceptance. Previously reported behavioral/coverage findings in `../frontend/review.md` remain for their owners. Any imported retention/navigation/query fixes change this source snapshot and require affected checks to be rerun.

Evidence: `lint.log`, `lint.json`, `typecheck.log`, `typecheck.json`, `source-before.json`, `source-after.json`, and `summary.json`.
