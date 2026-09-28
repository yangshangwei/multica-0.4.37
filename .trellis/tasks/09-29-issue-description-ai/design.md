# Design

Use a shared IssueDescriptionAssist component beneath the existing ContentEditor inside the editor scrolling region. Ghost button: AI optimize description (manual) / AI optimize instructions (agent). Inline preview uses existing ReadonlyContent and semantic tokens; questions are separate from replacement Markdown. No nested dialogs or gradient decoration.

API: POST /api/issues/optimize-description (workspace pinned with X-Workspace-ID header); request {text: string, title?: string, mode: "manual" | "agent"}; response {text: string, questions: string[]}. This endpoint only produces text. Existing server LLM handles generation with system instructions that treat input as data, preserve intent/language/constraints and references, and return JSON. Return explicit unavailable/provider/invalid-output errors. No database migration.

Core API schema and useOptimizeIssueDescription mutation wrap abortable requests. Component stores ephemeral request/preview/undo snapshots locally; adopted body writes to current draft using host callback and ContentEditor.adoptContent. Compare actual editor Markdown immediately before apply/undo; account for serialization normalization when saving the undo guard. New request, cancel, unmount or mode remount invalidates prior generation. Upload gates are checked live and reactively.

Keep normal issue submission independent. Shared modal code automatically serves web and desktop; mobile UI is outside this task.

## Streaming continuation
See `streaming-plan.md` for the user-requested SSE transport and provisional-preview contract.
