# M1-S1 shared Web/Desktop implementation

Initial lane status: implementing after explicit user approval. Owner: m1_views_implement. This file is an initial scope handoff; final result is in the resume reports below.

## Change boundary

The existing ordinary status-only hint gate cannot collect a signed acceptance or pass a completion binding. The new shared modal consumes the core evidence/authorization actions, while the gate routes enforce policy to it and retains non-authorizing hints elsewhere. Existing single-issue success callbacks are forwarded only after a verified first submission; the old plain PUT continuation must never follow the new flow.

Expected files: the shared completion hint gate and its three ordinary callers, a new authorization modal and registry entry, English/Chinese issue locales, and focused views tests. The core owner owns ModalType and all API/state/cache logic. Drag, batch, create-done, mixed edits, collaboration/R38, app routing, dependencies, server code and runtime/Provider execution remain outside this write scope.

The initial views working tree was clean. Existing hint tests protect unchanged behavior. New tests will exercise explicit acceptance/submit, exact core wiring, recovery-only unknowns, cancellation/unmount, and callbacks. Product state matrices remain canonical in core. Visual-verdict was read; no screenshot/reference or visual result exists yet. Parent owns live Web/Desktop validation.

## Checks

This was the initial lane handoff snapshot. Final implementation and verification are recorded in [m1-clients-resume.md](m1-clients-resume.md) and [m1-final-check.md](m1-final-check.md). The live enforcing Web/Desktop modal remains unverified because the available Demo project is not in enforce mode.
