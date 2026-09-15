# Quality Guidelines

> Code quality standards for frontend development.

---

## Overview

<!--
Document your project's quality standards here.

Questions to answer:
- What patterns are forbidden?
- What linting rules do you enforce?
- What are your testing requirements?
- What code review standards apply?
-->

(To be filled by the team)

---

## Forbidden Patterns

<!-- Patterns that should never be used and why -->

(To be filled by the team)

---

## Required Patterns

<!-- Patterns that must always be used -->

(To be filled by the team)

---

## Testing Requirements

### Common Mistake: importing the real `useChatStore` in a views test

**Symptom**: A component test that renders anything reading `useChatStore` (e.g. the floating-chat toggle in `preferences-tab.tsx`) fails with `Chat store not initialised — call registerChatStore() first`.

**Cause**: `useChatStore` exported from `@multica/core/chat` is a **Proxy singleton**, not a plain Zustand hook. The real store instance is registered once at app boot via `registerChatStore()`; in a test environment that never happens, so every call — including the `apply` trap from a selector call — throws.

**Fix / Prevention**: Mock the module with the callable-store shape (CLAUDE.md rule for `@multica/core` stores). Reference implementation: the `chatState` mock in `packages/views/settings/components/preferences-tab.test.tsx`.

```ts
// Bad — the Proxy throws on first selector call:
import { useChatStore } from "@multica/core/chat"; // real singleton in test

// Good — callable store + getState:
const chatState = vi.hoisted(() => ({
  floatingChatEnabled: false,
  setFloatingChatEnabled: (v: boolean) => { chatState.floatingChatEnabled = v; },
}));
vi.mock("@multica/core/chat", () => ({
  useChatStore: Object.assign(
    (selector?: (s: typeof chatState) => unknown) =>
      selector ? selector(chatState) : chatState,
    { getState: () => chatState },
  ),
}));
```

> Setters in the mock state should mutate the mock state (not just be `vi.fn()`) when the test asserts a post-toggle value.

<!-- What level of testing is expected -->

---

## Code Review Checklist

<!-- What reviewers should check -->

(To be filled by the team)
