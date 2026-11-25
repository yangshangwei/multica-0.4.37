# Current behavior and reuse evidence

Inspected 2026-11-25 in `/Volumes/artisan/code/2026/multica-0.4.37` on main. These observations establish the baseline; they are not evidence that the proposed feature exists.

## Baseline verification

```sh
pnpm --filter @multica/views exec vitest run editor/extensions/mention-suggestion.test.tsx modals/quick-create-actor-picker-model.test.ts --maxWorkers=2
```

Observed result: 2 files passed, 51 tests passed, duration 9.91s, 2026-11-25 19:52 Asia/Shanghai. Vitest v4.1.0. pnpm emitted an existing configuration warning about ignored package.json pnpm fields; the test command exited 0. No product source was modified by this task before planning.

## Baseline facts

| Evidence | Implication |
| --- | --- |
| `packages/views/editor/extensions/mention-suggestion.tsx:115` | Users groups agents, squads and members together |
| `packages/views/editor/extensions/mention-suggestion.tsx:154`, `:353` | A global 20-row display cap hides overflow without a browsing action |
| `packages/views/editor/extensions/mention-suggestion.tsx:744`, `:765` | Agent/squad matching uses name/pinyin and omits saved responsibility search |
| `packages/views/editor/extensions/mention-suggestion.tsx:272` | Selection already tracks typed identity; preserve it through reordering |
| `packages/views/editor/extensions/mention-suggestion.tsx:518` | Rows show names/type badges; actors do not display responsibilities |
| `packages/views/modals/quick-create-actor-picker-model.ts:20`, `:56` | Reusable metadata and rank logic with description search |
| `packages/views/modals/quick-create-actor-picker-model.ts:34` | Quick-create Agents excludes squad leaders; the proposed inclusive Agents tab needs its own predicate |
| `packages/views/modals/quick-create-actor-picker-model.ts:74` | Reference pagination limits rendered rows, not the search domain |
| `packages/views/editor/extensions/mention-recency.ts:58`, `:82` | Mention-specific device/workspace history exists; do not substitute creation history |
| `packages/views/modals/manual-create-assignee-picker.tsx:46`, `:55` | Reference pattern for scoped favorites and guarded hydration |
| `packages/core/issues/stores/quick-create-store.ts:110`, `:138` | Preference readiness depends on live user/workspace/reset scope, not only persist.hasHydrated |
| `packages/core/workspace/queries.ts:49`, `:56`, `:110` | Existing member/agent/squad Query options can supply directory lifecycle without a new API |

## Host scope

The current issue-detail new-comment editor is `comment-input.tsx:220`; threaded reply is `reply-input.tsx:242`; existing-comment and reply-edit editors are in `comment-card.tsx:742` and `:1080`. These belong to shared Views and are reused by Web and Desktop. `ContentEditor` already separates `mentionMode`/context options at `content-editor.tsx:195`, forwarded by `editor/extensions/index.ts:268`.

Do not use `currentIssueId` as a presentation flag; it also owns unrelated issue-reference behavior. A separate opt-in bounds the richer popup to the requested composition surfaces.

## Reuse decision

Reuse metadata/ranking and design tokens, not the task-creation popover controller. Its own search field, selection callbacks, default actions and preference writes are not the mention contract. Read creation favorites as shortcuts, retain mention recency, and keep all creation preference writes out of mention acceptance.

No added backend API, database migration or dependency is needed. The supplied screenshot is the problem reference; it is not evidence of an approved target layout.
