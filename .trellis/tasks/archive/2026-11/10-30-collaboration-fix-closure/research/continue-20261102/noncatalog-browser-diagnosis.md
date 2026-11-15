# Non-catalog v9 browser failures

Read-only review of frozen candidate `64a9994ec549e8dff73ab14456dc9a9f4b63f7c7`. Sources, original failure JSON and hashes are recorded in `noncatalog-browser-diagnosis.json`. No source/environment change, test, database, HTTP or Provider operation occurred.

## Findings (fixed)

None: this assignment permits diagnosis and report writes only.

## Findings (not fixed)

### NC-1: closed memory history is collapsed before historical purge

`e2e/collaboration-memory-web.spec.ts:84` waits for `Purge v2`, but the failed snapshot shows closed/read-only v4 and a collapsed Version history. The frozen memory panel mounts historical rows only inside its CollapsibleContent. Historical purge remains supported; it is simply not visible.

Minimal change: explicitly open Version history after closure, then assert that Restore controls remain absent before choosing Purge v2. Keep every original purge/API/current-v4/no-edit assertion. The pre-existing main worktree diff adds these exact two lines and belongs to `11-02-branch-functional-regression`; coordinate its use with that owner.

This is a confirmed test interaction omission. The updated original scenario still needs an actual zero-skip/zero-retry replay.

### NC-2: the manual-create overflow closes before priority selection

`e2e/settings-preferences.spec.ts:136` resolves Set priority, observes its opening animation as unstable, then loses the node at `1712752.296`. The menu had been opened approximately210ms earlier. The pre-cleanup screenshot retains the manual dialog, with the menu closed and a focus outline on Switch to Agent. API requests in this interval succeed. The later onboarding error-context snapshot is produced after timeout cleanup deletes the workspace, so it is not the cause of the menu failure.

Relevant source: the manual TitleEditor is initialized after mount and requests natural focus after50ms; its guard preserves another text entry's focus but not menu/button focus. The manual dialog also has a300ms transition. These establish a plausible focus/transition race, not a proved exact handler.

First minimal replay: after switching to Manual, wait for the naturally focused `Title (required)` editor before opening More options, then assert the real menu/menuitem is visible before selecting. Preserve all hidden-field, High-priority, POST201, server value and persisted-preference assertions. If it still closes after initial focus has settled, record focus changes and fix the responsible component. Do not use forced clicks, sleeps or broad timeout/retry changes.

Classification remains an observed popup lifecycle failure with a test synchronization hypothesis; a product focus defect is not yet excluded.

### NC-3: invitation acceptance succeeds but the frontend route does not commit

`e2e/workspace-access-branches.spec.ts:93` retains the invite URL at the5s URL assertion. The failed DOM already says “You joined Access owner!” and “Redirecting to workspace…”. The original task API log records acceptance200 in22.245ms, onboarding200, `/api/me`200 and two workspace-list200 responses in6.631ms and1.445ms. This excludes backend rejection and a slow workspace API as the observed failure.

The shared InvitePage sets accepted state before refetching the workspace list, then schedules `push(destination)` after1s. The Web adapter uses Next router.push for a different route. There is no retained invitation trace, so the destination RSC request, actual push timing and any front-end exception cannot be reconstructed from the current artifacts.

Replay the original test with trace, page-error recording, exact accept/workspace-list responses and the joined-workspace navigation request. Keep the original URL, exactly-one membership and accepted-invitation revisit assertions. Do not change the backend, manually navigate or extend5s without this evidence. If refetch fails after accepted state, the current done branch can hide the catch error; fix that exact path only after reproduction.

## Verification

- Source/hash and original JSON/error-context/trace reads: completed.
- Lint/typecheck/tests: not run, because this is a read-only diagnostic slice during coordinated verification.
- Memory: minimal test change identified; replay required.
- Preferences/invitation: observed failure layer identified; exact cause remains gated on focused replay.
- Catalog failures and mutable parallel UI changes were outside this slice.
