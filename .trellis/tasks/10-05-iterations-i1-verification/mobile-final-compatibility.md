# I1 mobile final compatibility

Verified 2026-10-07 11:24 Asia/Shanghai against the uncommitted workspace on
`codex/projects-p1`, HEAD `115b4cd28277941a8d9ad8fda5acecf99b999152`.
This closes the bounded ITR-033 mobile notice gap; the parent task owns UG/VG
acceptance. No mobile iteration editor, closure/rollover action, release-flag
change, commit, push, merge or deployment was added.

## Behavior and parity

- A current-workspace iteration notice with a supported kind and valid UUID
  shows the native `Alert`: **Open iterations on Web or Desktop**. Its message
  directs the user to a supported Multica Web/Desktop version to view and
  manage iterations and explains that mobile iteration actions are unavailable.
- `start`, `end`, `cancel`, `dates_changed`, and `overdue` recognize the detail
  target. A `disable` notice recognizes the workspace iteration-list target
  without requiring an iteration ID, matching the server notification payload.
- Existing mark-read behavior stays before the branch. Every iteration notice
  returns before project/issue routing; a stale, malformed, unknown-kind, or
  foreign-workspace payload cannot fall through to a task or project route.
  The helper itself does not mutate the read state or retained payload fields.
- Web/Desktop parity was checked against
  `packages/views/inbox/components/inbox-display.ts:getInboxDestination`,
  `packages/core/inbox/queries.ts:deduplicateInboxItems`, and
  `server/internal/service/iteration_notifications.go`. Mobile keeps its
  independent UI/data layer and existing English/native-Alert convention.
- Existing compatibility tests still demonstrate that ordinary mobile text
  updates omit iteration fields, current-server planning fields remain readable,
  old-server omitted planning fields remain unknown, and an iteration notice
  with `issue_id: null` survives inbox parsing/deduplication alongside old types.

## Changes

| File | Purpose |
| --- | --- |
| `apps/mobile/app/(app)/[workspace]/(tabs)/inbox.tsx` | Native supported-client explanation and terminal iteration branch |
| `apps/mobile/lib/inbox-display.ts` | Pure current-workspace/known-kind/UUID target guard |
| `apps/mobile/lib/inbox-display.test.ts` | Canonical guard matrix and retained-payload regression |

The existing uncommitted `data/iteration-compatibility.test.ts` and
`components/inbox/detail-label.tsx` were preserved. No dependency or test-renderer
configuration changed. No source-scanning tests were introduced.

## Checks

All commands below ran from the repository root. Tests used
`scripts/go-test-with-agent-cli-guard.sh`; no ambient agent CLI invocation was
reported. These Node/shell checks use no database, backend, browser, simulator
or native build process; an exclusive test database is therefore not applicable.

| Command | Result |
| --- | --- |
| `bash scripts/go-test-with-agent-cli-guard.sh -- pnpm --filter @multica/mobile exec vitest run lib/inbox-display.test.ts` | RED: 8 new tests failed because the helper was absent; 3 existing tests passed |
| `bash scripts/go-test-with-agent-cli-guard.sh -- pnpm --filter @multica/mobile exec vitest run lib/inbox-display.test.ts data/iteration-compatibility.test.ts` | GREEN: 2 files / 14 tests passed, including a final rerun after the test-fixture type correction |
| `bash scripts/go-test-with-agent-cli-guard.sh -- pnpm --filter @multica/mobile test` | 28 files / 196 tests passed; `ios-run.test.sh: all assertions passed` |
| `bash scripts/go-test-with-agent-cli-guard.sh -- pnpm --filter @multica/mobile typecheck` | Final exit 0; initial test-array inference error fixed with explicit `InboxItem["details"][]`, without casting |
| `bash scripts/go-test-with-agent-cli-guard.sh -- pnpm --filter @multica/mobile lint` | Exit 0; 0 errors, 7 warnings in untouched files |
| `git diff --check -- 'apps/mobile/app/(app)/[workspace]/(tabs)/inbox.tsx' apps/mobile/lib/inbox-display.ts apps/mobile/lib/inbox-display.test.ts` | Exit 0 |

The full mobile suite and lint preceded only a type annotation in the test
fixture. The final focused suite and typecheck include that annotation; runtime
source is unchanged from the full-suite run.

Existing lint warnings are in `more/issues.tsx`, `more/settings.tsx`,
`more/settings/notifications.tsx`, `chat/chat-composer.tsx`,
`issue/comment-card.tsx`, `issue/comment-context-menu.tsx`, and
`issue/issue-reaction-row.tsx`. pnpm also emits the existing root configuration
warning about `pnpm.onlyBuiltDependencies` and `pnpm.overrides`; it does not
fail the commands. These are outside this compatibility slice.

## Source identity and limits

SHA-256 at 2026-10-07T03:24:21Z:

| Source | SHA-256 |
| --- | --- |
| `apps/mobile/app/(app)/[workspace]/(tabs)/inbox.tsx` | `a971b47c0a12e206e0eb41de4cf93c2b21c56a549473022896dfe53bd73d88ae` |
| `apps/mobile/lib/inbox-display.ts` | `14624942906683e48b882b4901e926273d95feccfa82502ce89d176113f20f9d` |
| `apps/mobile/lib/inbox-display.test.ts` | `78b0acd37e5952d40a94082e5736c5eb3e1d328aa93b38968b89c8d2ab6c43e3` |
| `apps/mobile/data/iteration-compatibility.test.ts` | `39030cbf7a756175151bb4e26722b7b2c61f7d3e6aa3d40ebb30a418a167f796` |
| `apps/mobile/components/inbox/detail-label.tsx` | `765a6004a10553a69cbc803ec4798e5c662b7431fbd772aca2a7f984b28ee0c7` |

Native Alert presentation, tapping on a device/simulator, accessibility focus,
and cross-client live updates were not exercised. The app's current Vitest lane
is deliberately Node-only; it proves the target policy and existing headless
compatibility, not native rendering. Alert wiring was inspected in source and
checked by TypeScript/lint. Server permissions and missing-confirmation 428
behavior remain covered by the parent's existing backend evidence, not by this
mobile-only run. Full mobile I1 UI remains deferred by the approved scope.
