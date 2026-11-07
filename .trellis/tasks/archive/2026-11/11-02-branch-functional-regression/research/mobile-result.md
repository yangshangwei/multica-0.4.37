# Mobile verification result

Candidate: `7af9ab9905c98c77bb091223ebe687c639475bb4`. This lane passed its available automation with existing lint warnings. Actual iOS runtime testing is blocked by the unavailable simulator toolchain. These results do not establish native UI or end-to-end acceptance.

| Check | Actual result | Evidence |
| --- | --- | --- |
| `pnpm --filter @multica/mobile typecheck` | PASS, exit 0 | `evidence/mobile/typecheck.*` |
| `pnpm --filter @multica/mobile lint` | PASS, exit 0; 0 errors, 7 warnings | `evidence/mobile/lint.*` |
| Guarded `vitest run --maxWorkers=1 --reporter=json` | 196 PASS, 0 FAIL, 0 skipped/todo; 28 test files | `evidence/mobile/vitest-results.json`, `unit.*` |
| `bash apps/mobile/scripts/ios-run.test.sh` | PASS, exit 0; three wrapper scenarios with fake pnpm | `evidence/mobile/ios-wrapper.*` |
| `xcrun simctl list devices available --json` | BLOCKED prerequisite, exit 72; simctl unavailable | `evidence/mobile/simulator-inventory.*`, `ios-prerequisites.json` |

The Vitest JSON reports 70 suites because nested suites are included; the distinct test file count is 28. The 196 assertions cover headless helpers, serializers, schema compatibility, cache/query/realtime logic and project API behavior. The configured Node test environment does not render React Native UI. The iOS wrapper regression verifies prebuild ordering, argument forwarding and abort-after-prebuild-failure; it runs no Xcode build or device action.

Expo lint used its repository default inputs (`app` and `components`, with no `src` directory); it does not establish lint coverage of `data/` or `lib/`. The seven warnings are retained without source edits:

- `app/(app)/[workspace]/more/issues.tsx:97`: unstable logical-expression dependency for useMemo.
- `app/(app)/[workspace]/more/settings.tsx:35` and `more/settings/notifications.tsx:23`: Array<T> style warnings.
- `components/chat/chat-composer.tsx:32`: unused Ionicons import.
- `components/issue/comment-card.tsx:421` and `issue-reaction-row.tsx:20`: unstable logical-expression hook dependencies.
- `components/issue/comment-context-menu.tsx:192`: missing getName callback dependency.

These are static observations, not reproduced UI defects. No automated fix was applied. The package manager also emitted its existing ignored-pnpm-field warning; the exact stderr is preserved.

The selected developer directory is `/Library/Developer/CommandLineTools`. Read-only inspection found no Xcode application name under `/Applications`, `/Users/artisan/Applications` or `/Volumes/artisan/Applications`. No system developer setting was changed, no tool downloaded, no simulator started and no physical device accessed. A complete local Xcode/simulator runtime is needed for iOS functional and Web/Desktop parity journeys; those remain unexecuted.

Every command has timestamp, SHA, argv, cwd, exit code and stdout/stderr hashes in `evidence/mobile/*.json`. CI/offline/no-telemetry/no-dotenv environment overrides kept Expo noninteractive and local. Agent CLI guard stayed active for Vitest and reported no ambient CLI invocation. The source SHA remained fixed, and final git status showed only the two pre-existing/new task directories as untracked. No product source, dependency or configuration was changed; no commercial Provider call was performed.

Changed files are task evidence under `evidence/mobile/` plus this report and `mobile-result.json`. No code simplification was attempted. Remaining risks are the seven lint warnings, the default lint input limit, and missing actual iOS runtime/semantic-parity evidence.
