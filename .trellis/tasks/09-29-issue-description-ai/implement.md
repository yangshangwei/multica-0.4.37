# Issue Description AI Implementation Plan

**Goal:** Improve task descriptions without changing intent or triggering task execution.
**Architecture:** Shared views component, core API/mutation, authorized Go handler using existing llm client.
**Tech Stack:** React/Tiptap, TanStack Query, Zod, Go/Chi and existing llm package.

1. Backend: inspect existing LLM and router/auth patterns. Write failing handler tests for auth, validation, unavailable AI, success and malformed output; implement optimize handler, bounded inference and reference preservation. Update outbound consumer allowlist/docs. Run targeted Go tests and go vet.
2. Core: add request/response contract and abortable API plus mutation in existing issues exports. Test malformed responses and request abort propagation; run core targeted Vitest/typecheck.
3. Views: write behavioral tests for shared preview/apply/undo, stale input, failures and cancellation. Implement inline UI using existing ReadonlyContent and ContentEditor.adoptContent. Integrate both composers preserving existing edits; add bilingual strings and integration assertions. Run views tests, lint/typecheck.
4. Review full task diff against specs; run workspace typecheck/lint and focused regression tests. Inspect UI in browser if available; record visual verdict and limitations. Update task evidence/spec learning, preserve unrelated work.

## Streaming continuation
See `streaming-plan.md` for the user-requested SSE transport and provisional-preview contract.
