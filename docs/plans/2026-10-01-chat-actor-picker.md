# Shared chat actor discovery

**Goal:** Reuse the issue-creation actor picker's search, responsibility previews,
categories, favorites, recent usage and full directory in chat.

**Architecture:** Keep `QuickCreateActorPicker` as the shared discovery surface.
Add a chat variant with caller-owned eligibility, positioning and pin limits.
The chat adapter supplies existing server pins and session-derived recents;
squads supply leadership metadata only. Chat continues selecting agent IDs.

**Tech stack:** React, TanStack Query, Base UI, Vitest, Playwright.

## Scope and cleanup plan

1. Extend the existing chat picker tests with responsibility search, category
   navigation, runtime guards, pins and keyboard selection; establish a failing
   regression before changing implementation.
2. Add the minimal chat variant to the existing actor picker. Hide squad
   selection and task-creation instructions/default actions in chat. Preserve
   creation and manual-assignment behavior.
3. Replace both duplicate chat lists with the shared picker. Preserve new-chat
   zero/single-agent shortcuts and quick-bar selection semantics.
4. Use chat's existing server pin mutations (five-agent limit); never write task
   creation preferences. Derive recents from non-archived session activity.
5. Run focused tests, views lint/typecheck, static diff checks, and browser
   interaction/layout checks. Record shared contracts in the views spec.

## Acceptance

- Both new-chat and switch-agent entry points use the same component as creation.
- Search matches saved responsibilities as well as names/pinyin.
- Mika and planning categories retain their existing identity rules.
- No squad ID reaches the agent-only chat selection callback.
- Runtime-unbound agents stay disabled; offline but bound agents stay selectable.
- Favorites are chat pins, recent usage comes from accepted chat sessions, and
  task creation defaults/history remain untouched.
- Existing actor-picker keyboard, focus, paging and creation tests remain green.
- Narrow screens constrain the popup to the viewport.

## Verification

- 378 related Vitest tests pass across 27 files, including shared discovery and
  chat regressions. Responsibility search and pending-pin regressions were first
  observed failing, then passed after implementation.
- Views typecheck passes. Views lint has zero errors and 26 existing warnings;
  changed-file lint and the Impeccable detector are clean.
- An isolated production Web build passes. Three Chromium tests pass against
  that build, covering both entry points, English/Chinese and 1280/390px layouts.
- Screenshot review passes (95/100). Evidence and build/source identity are in
  `/tmp/chat-actor-picker-evidence/`; the temporary production server and worktree
  were removed after verification.
- Browser fixtures never submit chat messages or execute agent CLIs. Native
  Electron execution and actual agent replies were not exercised.
