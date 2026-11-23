# Settings audit remediation verification

F01–F17 are fixed and verified. Final checks pass **907 tests** across 74 test
files, scoped ESLint, five TypeScript targets, package-boundary checks and the
27-file UI detector. Native acceptance covers the audited error, save, layout
and accessibility behavior with a disposable Electron harness.

Work commits: `f3e1161a4` (core) and `6ccd5e643` (settings/specs).

Machine-readable results: [summary.json](verification/summary.json),
[native acceptance](native-verification/acceptance-final.json), and
[source inventory](verification/source.json). The 76 source/test/locale paths
are recorded with hashes; four spec paths document the resulting contracts.

## Changes by finding

All settings component paths below are relative to
`packages/views/settings/components/`; core and desktop paths are named
separately where relevant.

| ID | Result and principal files |
| --- | --- |
| F01 | `issue-statuses-tab.tsx` supplies category labels from the parent query; child formatting no longer creates retrying observers. |
| F02 | `settings-query-feedback.tsx`, catalog/member/workspace/provider views and their core options distinguish pending, failure, authorized cache and successful emptiness. `core/query-access.ts` clears denied data and its cancellation rollback snapshot. |
| F03 | `labels-tab.tsx`, `properties-tab.tsx` and `quick-actions-tab.tsx` size their grids by available content width. |
| F04 | `properties-tab.tsx` bounds the editor with reachable header/body/footer and all twelve options. |
| F05 | Properties, status, member, VCS and token fields plus desktop daemon/behavior controls have purpose-specific accessible names. |
| F06 | `keyboard-shortcuts-tab.tsx` exposes the current binding or unassigned value through accessible descriptions. |
| F07 | Quick Action warnings and provider connected text use existing semantic foreground tokens, without reducing entire-row opacity. |
| F08 | MCP, triage, provider and nested DingTalk views retain authorized nodes/drafts after transient errors. Denial removes protected content permanently until a successful read. The shared agent integrations caller passes the original group Query. |
| F09 | Account/workspace avatar callbacks propagate failed persistence to the existing crop/upload workflow. |
| F10 | Account/workspace name validation exposes associated errors and unsaved status while preserving other active form values. |
| F11 | Invitation, member/prefix and token handlers reject duplicate pending submissions; failed confirmations retain their target and retry path. |
| F12 | `tokens-tab.tsx` applies saved-token acknowledgement to every result dismissal path. |
| F13 | `github-tab.tsx` locks competing settings/connection operations until the accepted write settles. |
| F14 | `use-auto-save.ts` returns an awaited candidate result; `repositories-tab.tsx` removes a row only after persistence succeeds. |
| F15 | `preferences-tab.tsx` serializes accepted timezone choices by locking the control while saving. |
| F16 | Core PAT schemas/client methods reject malformed responses with redacted diagnostics. Auth mutation hooks return no secret data and prevent retry/offline replay; the one-time credential remains local to the guarded view. |
| F17 | Desktop `updates-settings-tab.tsx` has a stable live region and a focus-preserving busy button with a synchronous duplicate guard. |

## Review follow-ups

Independent review reproduced and fixed cancellation restoring a denied Query
snapshot, missing invitation/share-link/repository boundaries, and opaque
triage membership state. Nested DingTalk reads now reuse the existing core
pagination options; both the settings and agent callers preserve their Query
state. Permission joins also clear denied data. A denied inactive list cannot
silently block a still-authorized active group's Forget action.

Native verification found an additional platform distinction: Chromium blurs
a button that becomes natively disabled, whereas jsdom does not. The manual
update button now keeps native focusability, exposes `aria-disabled`, and uses
a synchronous pending guard. Completion never calls `focus()` to reclaim a
user's deliberate focus change.

The consolidated independent review is
[continuation-review.md](research/continuation-review.md); lane reports retain
the original failing-before and passing-after evidence. No confirmed source
defect remains in the agreed scope.

## Final checks

| Check | Result | Evidence |
| --- | --- | --- |
| Shared settings, triage, avatar, locale parity and shared agent integration tests | 783 passed / 58 files | `verification/check-runs.json` (views-tests) |
| Core access, PAT, save/query and provider regressions | 105 passed / 13 files | `verification/check-runs.json` (core-tests) |
| Desktop daemon, behavior and updater tests | 19 passed / 3 files | `verification/check-runs.json` (desktop-tests) |
| TypeScript | Core, views, Web, desktop node and desktop renderer passed | `verification/check-runs.json` (typechecks) |
| ESLint | 23 core, 45 views and 6 desktop files passed | `verification/lint-runs.json` |
| UI static detector | 27 changed UI files, zero findings | `verification/detector-final.json` |
| Package boundaries | Zero forbidden imports/accesses | `verification/package-boundaries.json` |
| Native acceptance | F01–F17 evidence passes; 18 catalog geometry/pointer/keyboard combinations | [native-acceptance.md](native-acceptance.md) |
| Text contrast | Warning 6.04/8.76; connected 5.40/7.31 (light/dark) | Native configuration/integration results |
| Cleanup | Owned processes exited, temporary profile removed; original services healthy | `native-verification/cleanup-final.json` |

Exact final commands and their output are retained in [check-runs.json](verification/check-runs.json). The local `.log` copies follow the repository log ignore rule. Overlapping intermediate
runs are not added to the final 907-test total. The existing pnpm configuration
location warning was informational; no dependency or package-manager setting
was changed for this task.

## Simplifications and boundaries

One feedback helper and one core access boundary replace divergent recovery
branches. Status children reuse parent labels. The token list moves from a
manually mirrored array into its account-scoped Query. DingTalk reuses its
existing paginated options. Awaited save results and synchronous pending guards
keep confirmation ownership explicit without a new store, dependency, form
framework, endpoint or global retry policy.

The working checkout contains parallel tasks. Source hashes record the exact
tested checkout. `packages/core/api/client.ts` also contains concurrent
collaboration changes; only this task's PAT import and methods are included in
its work commit. Other task files and API hunks remain untouched.

No live backend mutation, release or deployment was performed. Native evidence
is macOS Electron with memory responses and platform IPC fixtures; it does not
claim Windows/Linux execution, spoken screen-reader output, production
performance or gated plugin/billing coverage. The existing workspace-leave
realtime ordering remains the separately documented lifecycle debt.
