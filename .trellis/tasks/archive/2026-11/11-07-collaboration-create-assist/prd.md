# Expand collaboration creation and improve descriptions with AI

## Goal and confirmed direction

The user requests the ordinary task composer's expand/collapse affordance next to Close, a “帮我完善” AI action for collaboration descriptions, and a focused review of other interaction gaps. The user explicitly approved “新建并推进”. Preserve the approved option C visual hierarchy and the refined default density; add only functionality that supports this request.

## Requirements

### R1 Expand and collapse

- Put an expand/collapse icon immediately before Close with familiar ordinary-composer visuals, localized accessible labels/tooltips and pressed state.
- Expanded mode increases usable width/height within viewport gutters; collapse restores the refined default. Preserve the mounted draft, selection, uploaded files, active request and keyboard focus while toggling. Reopening starts at default size.
- Keep header and footer in normal flow and scroll only body. All modes retain narrow/coarse-pointer44px controls, long-source readability and reduced-motion behavior.

### R2 Help me improve the description

- Place a compact “帮我完善” action next to/below description using existing IssueDescriptionAssist logic and existing backend endpoint, source workspace UUID and manual description mode.
- Preserve plain textarea storage and exact original text for undo; do not migrate to rich text or trim the raw draft accidentally.
- Match existing behavior: nonempty description required; provisional stream is display-only; validated completion auto-updates an unchanged/unblocked draft with Undo. New edits or active upload/create/recovery prevent adoption. Preserve clarification/answer merge and undo behavior.
- AI state is scoped to server/viewer/source/draft identity; switching sources, closing, losing authority or replacing draft cannot publish old content. Abort/cancel and discard stale callbacks. Never send attachments/credentials or mutate ambient workspace to call AI.
- AI failure, unconfigured server, route absence, timeout, malformed result and cancellation leave draft/files usable. Do not translate an AI-route404 into source-loss cleanup.
- Provide visible input-limit feedback for body20000/title500 Unicode characters; disable only AI action. Normal collaboration limits remain unchanged and long input can still be created. Do not truncate text silently.
- Reuse default ordinary composer behavior without regressions. Collaboration completed-preview text must stay inert or use restricted resource policy, never ambient workspace file resolution.

### R3 Focused UI review

Resolve expansion/AI placement, loading/cancel/retry, input-limit explanation, keyboard focus, narrow/footer overflow and generated-result visibility within this feature. Keep full account identity, source locks, draft persistence, attachment ownership, original-create recovery and prior IME/initial-focus fixes. The AI editing helper is independent of task AI execution policy; it does not grant or start runtime execution.

## Acceptance criteria

- [x] Expand/collapse controls work by pointer and keyboard, preserve draft/files/ongoing stream, restore focus and reset size on reopen.
- [x] Selected source workspace explicitly owns the AI request even on /account with a different ambient workspace.
- [x] Real textarea adoption/undo preserves original whitespace; clarification merge uses latest draft; streamed fragments never enter create payload.
- [x] Source/viewer/API/draft switch, unmount and authority loss discard late output; normal edit/upload/create locks prevent accidental overwrite.
- [x] Missing AI/404/error/cancel preserve draft and attachments; AI-only input limits are visible and do not block ordinary creation.
- [x] Ordinary assist and real-editor regression suites remain green, along with collaboration, locale parity, Views typecheck/lint and scoped detector.
- [x] Actual shared component browser evidence covers default/expanded, source/header/actions, empty/pending/success/stale/error/retry/undo, long output, both themes, desktop and390x740/320x568, plus keyboard and coarse-target geometry.

## Out of scope

No new AI provider, daemon/agent execution, backend/API route/schema changes, attachment-content ingestion, source permissions changes, rich-text migration, mobile UI, new dependency/store, or unrelated global redesign. AI content quality against a live paid model and physical-device acceptance are separate from deterministic integration verification.
