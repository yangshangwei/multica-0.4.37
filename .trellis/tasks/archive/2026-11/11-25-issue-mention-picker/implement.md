# Implementation and verification plan

Status: implementation and verification complete on 2026-11-25. The checklists below retain the approved execution plan; `verification.md` and the final handoff in `progress.md` record actual results and limitations. Implementation approval was "按照计划推进"; subsequent commit/push authorization was "提交并推送".

## Baseline and ownership

- Source baseline: current checkout `main`, starting commit `a7bf6ba70`; preserve unrelated work and do not create a branch or commit without user instruction.
- Known baseline: 2 test files / 51 tests passed on 2026-11-25. See `research/current-behavior.md`; this is not feature acceptance.
- Main session owns requirements, integration, scope decisions, documentation and final evidence.
- After approval, use a `trellis-implement` native subagent for the cohesive editor/helper/host patch. A separate `trellis-check` agent reviews the completed diff and evidence. Avoid parallel writes to the editor files; a bounded browser-evidence lane can run once the candidate is runnable.
- Every dispatch starts with `Active task: .trellis/tasks/11-25-issue-mention-picker` and loads the curated context. Child agents may not widen the approved scope or commit.

## 1. Protect current behavior before extraction

- [ ] Verify starting Git status and record the current commit.
- [ ] Reuse the existing mention, popup, quick-create discovery and composer regression suites. Add only missing behavioral coverage required for opt-in isolation, focus-safe controls and full-directory filtering.
- [ ] Before moving catalog/search helpers, confirm existing quick-create ranking, inclusive/exclusive type rules, paging and full-description tests protect them. Add missing protection before the move.

## 2. Implement the bounded discovery model

- [ ] Extract shared pure catalog/search code into a neutral Views module only where necessary; update direct callers and their tests, without compatibility forwarding layers.
- [ ] Add categorized mention projection: permission-first metadata, inclusive agent/leader filters, favorites/recent deduplication, complete local search, 50-row expansion and truthful counts.
- [ ] Preserve separate bounded issue search, cancelled/direct-hit ranking and standard/context modes.
- [ ] Cover 550 synthetic candidates, beyond-preview description matches, pinyin, identical names, typed IDs, missing fields, archived/forbidden/disabled candidates and page-boundary targets.

## 3. Build the categorized popup and connect the hosts

- [ ] Wire an explicit opt-in presentation option through ContentEditor/extension factory; use the same suggestion-popup controller with the categorized list.
- [ ] Subscribe to existing directory Query options using the correct QueryClient and scope; handle cold/pending/error/cached/late reads without a directory mirror store.
- [ ] Read existing favorites with scoped hydration, and keep mention recency separate from task-creation preference writes.
- [ ] Add fixed category controls, agent-leader subset, saved description/leadership rows and Show more. Keep the existing editor as the query input.
- [ ] Implement the explicit Alt/Option+ArrowDown filter-entry route, roving category buttons and auxiliary Tab sequence/Return to typing. Preserve selection without selection-only transactions; focused-control Escape exits the correct suggestion plugin. Cover composition/Safari guards, disabled rows and async updates without changing shared global keymaps.
- [ ] Opt in new comments, replies and both comment/reply edit editors. Cover these hosts with integration tests and preserve drafts/uploads/submit behavior.
- [ ] Add Chinese/English editor copy using the conventions document; keep existing creation wording and filters unchanged.

## 4. Run focused verification, then package checks

Start with existing and added focused suites using bounded workers:

```sh
pnpm --filter @multica/views exec vitest run editor/extensions/mention-suggestion.test.tsx editor/extensions/suggestion-popup.test.tsx editor/extensions/mention-extension.test.ts editor/extensions/suggestion-trigger-arming.test.ts editor/extensions/submit-shortcut.test.ts modals/quick-create-actor-picker-model.test.ts modals/quick-create-actor-picker.test.tsx issues/components/comment-composers.test.tsx --maxWorkers=2
```

Include new categorized model/component tests and affected comment-edit tests. Then:

```sh
pnpm --filter @multica/views typecheck
pnpm --filter @multica/views lint
pnpm --filter @multica/web typecheck
pnpm --filter @multica/desktop typecheck
git diff --check
```

Run Core tests only if Core code changes become necessary; such changes require a concrete reason and updated scope. No backend changes are planned, so no unrelated Go suite is required for this UI-only task. Record any pre-existing failure separately and do not fix unrelated code.

## 5. Real browser and Electron verification

- [ ] Follow the existing E2E environment contract: use an isolated environment and production Web build/start for timed journeys. Do not reinterpret a cold development compile as a product regression.
- [ ] Reuse the existing quick-create E2E directory mocking and Electron isolation patterns. Add bounded coverage for categorized mentions with 550 synthetic actor rows; do not create hundreds of real agents or send comments to live agents.
- [ ] Exercise comment/reply/edit, five categories, leader subset, typed names/pinyin/descriptions, no-results recovery and paging beyond 20/50.
- [ ] Verify actual inserted mention identity/Markdown and that the draft remains unsent. Verify no creation favorite/history/default mutation.
- [ ] Exercise actual typing after clicking filters, the advertised keyboard route into every auxiliary action and back to typing, IME ownership including Escape where supported, Enter/Tab acceptance, focused-control plugin exit/reopen, page expansion and late results. Include identical-query account/workspace changes and late responses; query-change cancellation alone is insufficient. Observe viewport/caret behavior at top and bottom edges.
- [ ] Capture desktop and 375px screenshots in Chinese/English and both themes; exercise a native Electron window with a dedicated profile. Use `visual-verdict` before each next visual edit; persist its JSON to `.omx/state/issue-mention-picker/ralph-progress.json` without activating Ralph mode.
- [ ] Record renderer console/network failures, startup commit and synthetic-data scope. Static screenshots alone do not prove insertion/focus behavior.

## 6. Independent check and final handoff

- [ ] Dispatch `trellis-check` against the completed diff, all acceptance criteria and evidence. Fix relevant findings, then rerun only checks affected by those fixes.
- [ ] Update the shared Views spec with the new explicit opt-in, discovery/category semantics, focus contract and read-only favorite reuse. Do not document proposed behavior as implemented before verification.
- [ ] Report changed files, reused/extracted logic, test results, actual Web/Electron evidence and any remaining limitations. Do not claim completion based on the baseline run.

## Rollback and scope fences

Remove/revert only task-owned host opt-ins and categorized code. Preserve old default/context behavior and unrelated local files. Avoid changing server endpoints, global keyboard handling, auth preference formats, Chat semantics or the issue-creation selection lifecycle. A material product/scope change returns the plan for review before implementation continues.
