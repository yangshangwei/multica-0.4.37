# Journal - artisan (Part 4)

> Continuation from `journal-3.md` (archived at ~2000 lines)
> Started: 2026-11-21

---



## Session 175: AI小队空状态首页实现与生产验收
<!-- trellis-session: v=2 fp=87ef6161093dbe2f -->

**Date**: 2026-11-21
**Task**: AI小队空状态首页实现与生产验收
**Branch**: `codex/cross-workspace-collaboration`

### Summary

沿用已批准的小队首页任务，完成手动协作演示、创建和应用内文档入口；126项测试与3条生产浏览器流程通过，独立复核通过并清理验收环境。

### Main Changes

Approved continuation of the existing AI squad first-use task; no new task or scope expansion.

### User outcome

- Show an introduction and a clear creation entry when a workspace has no saved squads.
- Present delegation, roles and result handoff as three manual collaboration examples.
- Keep the existing template directory and both creation methods available.
- Route the documentation action to the embedded squad guide.
- Support both interface languages, dark appearance, narrow windows and keyboard use.

### Product boundary

- `SquadWelcome` is a shared view-only component.
- The existing squad Query owns list data and success/error state.
- Unknown/pending reads do not become onboarding.
- Errors take priority, including refresh failure after a successful empty read.
- A filtered-empty populated workspace keeps collection recovery.
- Template URLs are independent of the saved squad directory.
- The hero owns the single creation AppLink.
- The original chooser owns template and custom creation.
- No new store, dependency, data migration or router layer was added.
- Example values are marked illustrative and do not execute agents.

### Accessible manual examples

- The selected dot uses pressed semantics.
- Previous/next respect the first and last example.
- A live status names the current example.
- Inactive examples are aria-hidden and inert.
- Ordinary re-renders retain selection.
- A workspace identity change remounts the first example.
- Reduced motion removes authored CSS transitions.
- All actionable controls retain at least a 44px target.

### Visual correction

- Compared the preserved screenshot reference and actual built page.
- Initial verdict: 88 because local radius aliases silently resolved to square corners.
- `@theme inline` aliases were not runtime CSS variables.
- Used exact formulas based on runtime `--radius` in the local module.
- The final built preview radius is 18px.
- No global token changes were required.
- Final reference verdict: 94; the mechanical detector returned an empty list.

### Verification

- Shared page/chooser/locales: 93 passing tests.
- Documentation slug and bundle parity: 5 passing tests.
- Core docs schema and route consistency: 28 passing tests.
- Scoped ESLint and final Views typecheck passed.
- Embedded docs Go tests passed.
- Production API, Web and Electron renderer builds passed.
- Three complete final browser journeys passed with zero skipped/flaky/unexpected cases.
- Checked English and Chinese at 1440, 1024 and 390 pixels.
- Checked the 1024x768 initial creation action and dark appearance.
- Checked actual Electron compositor/scroll behavior at 200-percent zoom.
- Checked complete creation, templates, docs and manual keyboard navigation.
- The first browser run is preserved separately because native scroll had not painted.
- The final full replay adds stable painted-layout and viewport-bound assertions.
- All twelve product/test source hashes match the final evidence manifest.
- Independent code and visual finish reviews passed.

### Cleanup and scope limits

- Stopped only the recorded task API, Web and renderer processes.
- Confirmed all three ports closed.
- Removed only the labelled synthetic database container.
- Removed task-private state after verification.
- Left other tasks and their staged/unstaged edits intact.
- The inherited malformed-success squad list fallback is still an empty array.
- API/schema changes remained excluded from the approved scope.
- Native acceptance covers the built renderer, preload and routing fixture.
- It does not claim packaged Main, installers/updaters or real-model execution.
- Embedded docs were read through the task-local API; disconnected-client mode was not tested.

### Durable record

- Feature work is committed separately from lifecycle bookkeeping.
- Task is archived under `.trellis/tasks/archive/2026-11/11-20-squad-welcome`.
- Exact source/build/capture evidence is under `artifacts/qa/squad-welcome-20261120`.
- Relevant view specs record empty-state, manual-example and CSS runtime-token contracts.


### Git Commits

| Hash | Message |
|------|---------|
| `3059b899a64c20270d4bad41f00d8e3593e4f9df` | feat(squads): help teams start their first AI squad |

### Status

[OK] **Completed**
