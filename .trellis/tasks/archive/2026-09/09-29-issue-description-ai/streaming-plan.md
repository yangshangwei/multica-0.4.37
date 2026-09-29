# Streaming continuation

User requested streaming on 2026-09-29. The same approved inline preview remains; partial text appears progressively, while Apply is available only after the final validated result.

## Contract and sequencing
1. Existing endpoint negotiates SSE with `Accept: text/event-stream`. JSON response remains for clients using the existing API contract. No new provider configuration or dependencies.
2. SSE events: `text_delta` with `{text: string}` (append-only decoded provisional text); `done` with `{text: string, questions: string[]}` (authoritative validated result); `error` with `{code: string, error: string}` (sanitized failure). Headers disable buffering/cache. Cancel stops upstream and late callbacks are ignored.
3. Server/pkg/llm owns SDK streaming calls and JSON parameters. Reuse current model/token/timeout/compatibility policies; never restart after any content. Backend extracts the top-level JSON text string incrementally including escaped Unicode, bounds aggregate raw output, and validates full JSON plus protected Markdown before done. Partial text never authorizes adoption.
4. Core fetchRaw retains auth/CSRF/workspace/401 behavior. Parse arbitrary UTF-8/SSE chunk boundaries with bounded buffering, validate event schemas, require explicit done, cancel reader and reject incomplete/error streams. Older JSON responses still work. Mutation carries optional onText callback separately from request data.
5. Views renders partial text as inert plain text, not Markdown with remote images/links; completed content uses existing renderer. Cancel/error clears partial text. Keep original editor untouched until explicit Apply, preserve stale/undo/upload/reset/focus invariants.

## Verification
- Lock existing JSON behavior with existing tests before changes.
- Backend: actual first delta flushes before fake upstream closes; escaping/surrogate/code/reference cases; invalid final result has no done; disconnect, timeout, provider failures and limits; legacy JSON path unchanged.
- Core: split UTF-8/CRLF/multiple events/comments, malformed delta/done/error, EOF without done, cancellation, callback ownership, terminal reader cleanup, JSON compatibility.
- Views: show partial before completion, cannot Apply partial, error/cancel clears it, stale callbacks ignored, final result Apply/undo works; real editor regression stays green.
- Full affected typecheck/lint, targeted Go tests/vet and browser incremental preview with controlled stream.
