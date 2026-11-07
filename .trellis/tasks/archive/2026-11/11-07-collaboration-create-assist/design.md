# Reuse the ordinary composer with collaboration-specific boundaries

## Boundary

Implement in shared Views. Expected files: collaboration/creation-form.tsx (expand shell, callbacks and description integration); a focused collaboration/creation-description-assist.tsx if needed for textarea adapter and scoped guards; modals/issue-description-assist.tsx (small opt-in plain-text/label/disabled/current-owner extensions); relevant tests and en/zh collaboration locales. Existing core optimize mutation/API/streaming and backend membership checks are reused unchanged. No new dependencies.

## Expansion

Keep one Dialog, with explicit header expand and Close buttons using existing Lucide Maximize2/Minimize2 and DialogClose. Prevent duplicate default close. Use current544px/42rem default and expanded ordinary-composer-like56rem width with viewport-bound height and12px gutters. Local state only; no remount on toggle. Reset on reopen. AI onNeedsSpace may grow default height once for the open session; explicit expand/collapse takes precedence and can collapse back without losing AI state. Communicate sizing via callbacks/props only, never mirror request business state in the shell. Keep standalone unconstrained panel valid without Dialog controls.

## Reusable assistant

IssueDescriptionAssist currently needs four editor methods: getMarkdown, flushPendingUpdate, adoptContent, focus. A stable textarea adapter reads live DOM/raw text, writes the adapter read value synchronously before the assist rereads after adoption, and persists through its onChange callback into the existing draft only. No parallel persistent state. Existing Markdown callers retain trimming/normalization; an explicit plain-text mode preserves raw whitespace for collaboration and renders stale/completed preview as inert text. Never force-cast collaboration attachments into normal Attachment objects or inject fake workspace providers.

Expose the exact user-requested label “帮我完善” via a localized label override while keeping request mode manual. Empty body stays disabled like ordinary task creation; no title-only generation. Length guard counts Unicode code points, displays a specific reason and only disables AI; do not silently trim title/body to satisfy API limits.

## Async and permission ownership

SelectedCreationSource is keyed by apiScope:userId:sourceId; assistant additionally keys by draftId. Capture sourceId explicitly for useOptimizeIssueDescription. A live scope/draft predicate guards request start, delta, completion and follow-on merge/undo. Closing/source switch aborts via assistant unmount; late callbacks are discarded by request identity and current-owner predicate. Unauthorized/stale output is not retained as preview. Editing/uploading/submitting/pending delivery instead follows existing stale/blocked preview without overwriting current input.

Do not wrap the AI endpoint in protectCollaborationDelivery: route absence404 is not authoritative source loss. Retain existing source options authority handling and preserve all AI-error drafts/files. Generate/undo/merge disabled presentation and handlers must agree with locks; don't expose an enabled inert control.

## Layout and supplementary polish

Keep compact description action beside the editor, generation/status/clarifications in scrolling body, and footer fixed. Grow assist space only when used; reveal adopted editor after layout settles only while focus belongs to AI controls. Preserve the recent lightweight title focus, full recipient IDs and compact empty attachment treatment. Scope additional optimizations to real feature-related defects found during visual verification.

## Rollback

A scoped Views revert restores previous behavior with no migrations. Shared assist extensions default to existing ordinary composer semantics; its real-editor tests protect markdown/file/mention/code behavior.

## Browser adjustment

The first new-helper capture clipped the advanced summary by9px at722×660. A standard optional shared-root className lets only the collaboration wrapper use py-0, reclaiming16px without enlarging the refined default dialog or changing ordinary-task spacing.
