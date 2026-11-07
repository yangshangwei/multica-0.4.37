# Assist and expansion browser matrix

No build until both product implementers report stable. Production bundle uses current actual shared components, CSS and locale files. Existing ApiClient/readIssueDescriptionStream parse a mock ReadableStream with real SSE text_delta/done/error framing at `/api/issues/optimize-description`. Network transport is blocked externally; no real AI calls, quota or submissions. Draft persistence remains page-local memory StorageAdapter. Ambient workspace is deliberately a different UUID; every mock request records X-Workspace-ID.

## Feature batch

- Default and expanded1280×900,390×740,320×568 light/dark: adjacent expand/Close, pressed state, keyboard toggle,12px gutters,44px coarse controls, fixed footer/body scrolling, preserved title/description/files/active SSE, reopen defaults.
- Light/dark390: partial display-only, final auto-adoption, exact raw whitespace Undo, long output and clarification reveal, merge into latest edited draft, merge Undo.
-320English: stale user edits keep draft and render inert output; source switch cancels/discards old response and next request names selected source explicitly; no ambient file/image resolution.
-320English errors: AI404/unconfigured503/500 retain text/files; explicit retry; cancel; codepoint body20000/title500 guards affect only AI; create during partial sends only accepted description.

## Existing affected contracts

Reuse nativeEnter/CDP composition, uploadcancel/recovery/retry focus boththemes, fullUUID coarse320light/dark,32files320light/dark, knownsourceinitialfocus boththemes. Do not rerun unrelated historical cases. Fixture assertions for old height/no-scroll are not new acceptance for a helper that intentionally adds controls.

## Evidence discipline

Primary current and expandreference screenshots copied locally to user-current.png and user-expand-reference.png. Baseline78/revise written before product inspection. One combined inspection, one correction+confirmation. Assert current source hashes, inspect screenshots rather than deriving taste from test counts, persist visual verdict before UI correction requests. Avoid preserving repetitive raw error bundles; concise failure evidence is enough. Each browser driver owns an ephemeral127.0.0.1 listener and fresh contexts and closes them in finally. No full stack, installations, providers, real accounts or product edits by this browser worker.
