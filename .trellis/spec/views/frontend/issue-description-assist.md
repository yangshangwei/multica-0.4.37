# Issue description assist

The shared create panels in `packages/views/modals/` expose text-only AI assistance for manual descriptions and agent prompts. The AI action opts into replacing the active draft with the validated completed result. Streaming fragments remain ephemeral and must never enter a create/dispatch payload. Completion must preserve newer edits and any active upload/submission; blocked results remain inspection-only.

## Editor ownership

- Read current Markdown from ContentEditor, not only the debounced draft. Flush pending updates when taking the request snapshot and immediately before adoption/undo.
- Read current upload/submission guards at async completion, not the request closure. Compare canonical live Markdown before adoption and undo. ContentEditor persists trimmed Markdown; focusing a final fenced-code block may add a trailing paragraph, which must not invalidate undo.
- `adoptContent` does not emit onUpdate. Explicitly write the serialized editor content to the current draft. On asynchronous completion, restore keyboard focus and reveal the editor only while focus is still inside AI controls; never interrupt title/property editing.
- Use the existing live upload gate: an in-flight placeholder is deliberately absent from serialized Markdown and would be lost by replacement.
- Cancel on unmount; reset between continuous creations and mode changes. Match response ownership by AbortController identity, not only an isPending flag.
- The real-editor regression in `issue-description-assist-editor.test.tsx` covers file cards, mentions, code and focus; do not replace it with an always-successful editor stub.

## API contract

`POST /api/issues/optimize-description` uses the captured workspace header and existing membership gate. Body: `{text, title?, mode}`. Clients request `Accept: text/event-stream`: append-only `text_delta {text}`, authoritative `done {text, questions}`, or sanitized `error {code, error}`. Clients without that Accept header retain the JSON `{text, questions}` response. Questions remain outside AI replacement text. Show at most two execution-critical questions with inline answers and an explicit executor-decision option. Merge answered question/answer pairs into the latest live draft only when the user chooses to add answers; do not require another AI call or duplicate merged answers. Undo of a merge restores both the previous draft and its clarification inputs. Creating a task never requires completing the questions.

The backend uses `server/pkg/llm`, never the daemon or agent execution path. Input is limited to 20000 Unicode characters (title 500), a 128 KiB request, 45 seconds, and eight process-wide requests. Unconfigured deployments return `ai_unavailable` without an upstream call. Do not reflect or log prompts, model payloads or upstream credentials. Core uses `parseWithFallback` with redacted diagnostics and rejects invalid suggestions.

Goldmark alone cannot protect the editor's standalone `!file[name](url)` syntax: inventory full file-card lines separately. Preserve paired mention label/destination, fenced code language/body, links, images and raw HTML. Revalidate these invariants before sending the adoptable `done` result. Intent and language preservation additionally rely on the system prompt and require real-model product evaluation.

## Streaming invariants

- The LLM wrapper shares parameter construction and unsupported-parameter negotiation between JSON and streaming calls. Never retry or replay a stream once content was delivered.
- The backend extracts only the top-level JSON text field incrementally, preserving split escapes, UTF-8 and surrogate pairs. Bound raw output (180000 bytes) and text (30000 runes); complete schema and protected-Markdown validation remain mandatory before done.
- Render partial text as inert text. Do not render Markdown images/links before final validation. Clearing, retrying, canceling or unmounting invalidates callbacks by request identity.
- The core SSE reader supports split UTF-8 and CRLF, bounded frame buffering, event validation, and abortable reads. Require explicit done; EOF is failure. Cancel/release the reader at every terminal path. Progress callbacks are client options, never request JSON.
- `TestOptimizeIssueDescriptionStreamsBeforeUpstreamCompletes` proves real first-delta flushing before the upstream completes. Keep this test alongside parser and UI tests; concatenating a final response into chunks is insufficient streaming evidence.

## Create dialog layout

- Both modes share bounded 440px default / 600px assist sizing with an 85dvh cap and 12px phone gutters. Assistance requests space once when generation starts; retain expanded space through completion and mode switches. Explicit user expansion takes precedence.
- The middle editor/assist region scrolls; footer remains outside it. Keep the scroll container min-height zero and constrain the editor minimum inside that region, scoped to create panels rather than changing ContentEditor globally. The editor wrapper needs an intrinsic flex basis (`flex-auto shrink-0`): a zero flex basis with a fixed minimum lets long Markdown overflow over the assist controls and intercept clicks.
- `onNeedsSpace` and `onRevealEditor` are layout callbacks only; do not mirror assistance business state in the shell. Reveal after the next animation frame so post-adoption DOM height is current.
- Clarification disclosure preserves typed answers, exposes aria-expanded, and displays outstanding counts. Single-line textareas grow through field-sizing-content to roughly four lines. Keep mobile assist controls at least 44px tall.
- Visual QA must mount the full CreateIssueDialog with real panels and editors; API/provider fixtures may be mocked. Verify both modes, viewport gutters, fixed footer position, and low-height scrolling in addition to isolated component tests.
