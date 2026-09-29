# AI optimization for issue creation

User approved the inline preview design and explicitly requested task creation and implementation on 2026-09-29.

## Scope
Web/desktop shared manual description and agent prompt creation. Add a low-emphasis AI optimize action above properties, preserve original editor content while generating, preview suggested Markdown and separate clarification questions, explicitly apply, regenerate, discard, cancel and single-level undo. No agent execution or issue creation during optimization. No new dependencies.

## Acceptance
- Both creation modes share implementation, use current editor text (flush debounce), and maintain independent drafts.
- Empty text and active uploads cannot optimize. Pending submit disables application. Cancel/unmount/mode change ignore late responses.
- Applying or undoing cannot overwrite newer edits; compare live editor snapshots, not debounced state alone.
- Suggestions do not participate in submit before adoption; failed or unconfigured AI does not block ordinary creation.
- Preserve links, attachment references, mentions, code and explicit constraints; unknown requirements are separate questions.
- Reuse server/pkg/llm with existing MULTICA_LLM configuration, authenticated workspace authorization, bounded request size/time/concurrency; do not log private text.
- English and Simplified Chinese UI strings. Accessible keyboard actions and status announcements, bounded preview scrolling.
- Tests cover API malformed response, server validation, provider failure, preview/apply/undo/staleness/cancel and integration with both composers.

## Existing changes
Preserve pre-existing quick-create-issue.tsx/test, en/zh-Hans modals.json and unrelated dirty files. Do not commit others' changes.

## Streaming continuation
See `streaming-plan.md` for the user-requested SSE transport and provisional-preview contract.
