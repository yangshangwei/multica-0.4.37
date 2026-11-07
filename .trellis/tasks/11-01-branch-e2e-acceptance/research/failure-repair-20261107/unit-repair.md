# Unit failure repairs

**The original four unit failures are fixed on the isolated repair candidate.** Baseline `e54d09be4`; no commit or live-root integration performed by this agent.

## Changes

- `packages/core/diagnostics/diagnostic-context.ts` and its test: explicitly classify squad chooser/manual/template screens, preserving resource-ID and unknown-path masking. The new exact-screen test failed for all3 routes before the fix; the existing builder-parity failure is also retained.
- `packages/views/locales/{en,zh-Hans}/skills.json`: current progress-report default now describes the current project, exactly matching current Chinese embedded frontmatter. Previous descriptions retained under the existing `description_v1` convention.
- `packages/views/skills/lib/skill-presentation.ts` and its test: recognize only the two exact shipped progress-report defaults (English display/adoption copy and Chinese embedded copy), preserve their old wording when localizing retained workspace records, and preserve customization/unverified-copy behavior. Current source content and permission scope are unchanged.

## Verification

| Run | Result |
| --- | --- |
| Original diagnostics |14PASS/1FAIL|
| Original Skill presentation |106PASS/3FAIL|
| Added exact squad-screen RED |14PASS/4FAIL|
| Final diagnostics + route suites |33PASS/0FAIL,3files|
| Final Skill presentation + locale parity |180PASS/0FAIL,2files|
| Targeted ESLint; core/views typecheck; diff check |PASS|

The final suites total213 unique tests. Earlier parity and reproduction executions are not added to that count. A first intermediate Skill run113PASS/1FAIL is retained: the new progress tuple was accidentally sent through an old English-only summary expectation. Original two skills retain that expectation; progress now has an explicit two-language summary test. No existing assertion was weakened.

The old Chinese default is verified from `cd95c8e40^` embedded Skill frontmatter; the backend loader materializes this field, and the template adoption panel can persist its localized English description. Exact matching therefore preserves supported old copies, rather than inventing a compatibility alias. Custom text does not acquire default search keywords or summaries. This change is display-only and does not migrate records or grant broader access.

All commands, initial failures, final counts and file hashes are in `unit-repair.json`; raw logs and patch are in `.omx/private/acceptance-repair-20261107/units/`. The first Skill command accidentally named a nonexistent locale filter; it executed only the real presentation file and was not counted as locale coverage. The correct parity file subsequently passed. A lint-runner path typo performed no work and was followed by the correct recorded command.

Remaining work: parent independent review, complete affected-package checks, scroll repair and production Web/Electron revalidation. No claims made about the unresolved E2E failures.
