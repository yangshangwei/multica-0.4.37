# Backend streaming implementation

- Endpoint is still `POST /api/issues/optimize-description`; `Accept: text/event-stream` selects SSE, while ordinary clients keep the existing JSON response.
- Authorization, input/configuration and concurrency failures remain normal HTTP JSON errors. Once generation begins, SSE emits `text_delta` with append-only decoded text, then exactly one validated `done` or sanitized `error` event. SSE disables cache/proxy buffering and flushes each event.
- `pkg/llm.GenerateJSONStream` owns all SDK types. Shared private helpers keep GPT-5.6 reasoning, token limits and two-step explicit parameter compatibility negotiation aligned with `GenerateJSON`. Partial streams are never replayed; callback errors close the upstream.
- The handler incrementally extracts only the root `text` string with one-pass escape decoding, split UTF-8 and Unicode surrogate support. It bounds raw JSON at 180000 bytes and decoded text at 30000 runes. The full result still passes the existing JSON, links, mentions, code and custom `!file` validation before `done`.
- UI must treat deltas as inert provisional text. Partial output does not pass protected-Markdown validation and must not render remote images/links or enable adoption.
- Existing request deadline (45 seconds), input bounds and eight-slot concurrency gate apply to both modes. Client disconnect/timeout closes upstream HTTP work and releases the slot.
- Tests use local upstream HTTP servers only. A held-open upstream test proves that the first SSE event arrives before upstream completion. Additional tests cover split escapes/UTF-8/surrogates, limits, malformed result, provider failure, premature EOF, length termination, compatibility negotiation, cancellation, timeout and unchanged JSON mode.
