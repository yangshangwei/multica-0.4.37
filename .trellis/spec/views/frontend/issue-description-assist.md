# Issue description assist

The shared create panels in `packages/views/modals/` expose text-only AI assistance for manual descriptions and agent prompts. The suggestion is ephemeral until explicitly adopted; it must never enter a create/dispatch payload by merely generating or previewing it.

## Editor ownership

- Read current Markdown from ContentEditor, not only the debounced draft. Flush pending updates when taking the request snapshot and immediately before adoption/undo.
- Compare canonical live Markdown before adoption and undo. ContentEditor persists trimmed Markdown; focusing a final fenced-code block may add a trailing paragraph, which must not invalidate undo.
- `adoptContent` does not emit onUpdate. Explicitly write the serialized editor content to the current draft and restore keyboard focus.
- Use the existing live upload gate: an in-flight placeholder is deliberately absent from serialized Markdown and would be lost by replacement.
- Cancel on unmount; reset between continuous creations and mode changes. Match response ownership by AbortController identity, not only an isPending flag.
- The real-editor regression in `issue-description-assist-editor.test.tsx` covers file cards, mentions, code and focus; do not replace it with an always-successful editor stub.

## API contract

`POST /api/issues/optimize-description` uses the captured workspace header and existing membership gate. Body: `{text, title?, mode}`. Clients request `Accept: text/event-stream`: append-only `text_delta {text}`, authoritative `done {text, questions}`, or sanitized `error {code, error}`. Clients without that Accept header retain the JSON `{text, questions}` response. Questions remain outside replacement text.

The backend uses `server/pkg/llm`, never the daemon or agent execution path. Input is limited to 20000 Unicode characters (title 500), a 128 KiB request, 45 seconds, and eight process-wide requests. Unconfigured deployments return `ai_unavailable` without an upstream call. Do not reflect or log prompts, model payloads or upstream credentials. Core uses `parseWithFallback` with redacted diagnostics and rejects invalid suggestions.

Goldmark alone cannot protect the editor's standalone `!file[name](url)` syntax: inventory full file-card lines separately. Preserve paired mention label/destination, fenced code language/body, links, images and raw HTML. Revalidate these invariants before sending the adoptable `done` result. Intent and language preservation additionally rely on the system prompt and require real-model product evaluation.

## Streaming invariants

- The LLM wrapper shares parameter construction and unsupported-parameter negotiation between JSON and streaming calls. Never retry or replay a stream once content was delivered.
- The backend extracts only the top-level JSON text field incrementally, preserving split escapes, UTF-8 and surrogate pairs. Bound raw output (180000 bytes) and text (30000 runes); complete schema and protected-Markdown validation remain mandatory before done.
- Render partial text as inert text. Do not render Markdown images/links before final validation. Clearing, retrying, canceling or unmounting invalidates callbacks by request identity.
- The core SSE reader supports split UTF-8 and CRLF, bounded frame buffering, event validation, and abortable reads. Require explicit done; EOF is failure. Cancel/release the reader at every terminal path. Progress callbacks are client options, never request JSON.
- `TestOptimizeIssueDescriptionStreamsBeforeUpstreamCompletes` proves real first-delta flushing before the upstream completes. Keep this test alongside parser and UI tests; concatenating a final response into chunks is insufficient streaming evidence.
