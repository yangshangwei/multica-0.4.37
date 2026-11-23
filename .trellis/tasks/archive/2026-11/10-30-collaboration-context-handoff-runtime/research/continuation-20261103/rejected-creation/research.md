# Research: rejected collaboration creation feedback

- Query: Diagnose the v11 failure in `e2e/collaboration-delivery-web.spec.ts:191`, where the rejected-creation preservation message was reported as missing.
- Scope: internal
- Date: 2026-11-03

## Findings

The failure is test drift, not a product rendering or recovery-state defect. The v11 candidate's Playwright error expected the old sentence:

> `The issue was not created. Your draft and files are preserved; you can submit again.`

The candidate product locale had already changed the sentence to:

> `The issue was not created. Your draft and files are preserved.`

The v11 error-context snapshot confirms that the latter sentence was visibly rendered inside the `New collaboration issue` dialog, together with the rejection-specific reason `The selected recipient is unavailable. Choose another account.` and the preserved title/file checkbox. Therefore the rejected mutation reached `CollaborationCreationRejectedError`, the form stayed mounted, and draft/file preservation worked.

Evidence and code path:

- `/Volumes/artisan/code/2026/mc-collaboration-final-owned-20261103/test-results/collaboration-delivery-web-276fc-r-reviving-the-rejected-key-chromium/error-context.md` shows the visible status text and checked `Include correction-note.txt` checkbox.
- `packages/views/collaboration/creation-form.tsx:261-264` renders `creation.rejected` when `create.error instanceof CollaborationCreationRejectedError`; the same block renders the mapped rejection reason.
- `packages/core/collaboration/creation.ts:80-101` resolves a structured 403 result through `getCollaborationCreationResult`, then throws `CollaborationCreationRejectedError` after retaining the guarded draft state.
- `packages/views/locales/en/collaboration.json:561` defines the current sentence without the suffix.
- Commit `55bb7802f7` changed the locale from the old sentence with `; you can submit again.` to the current shorter sentence, but the test remained unchanged in the candidate.
- The candidate's failed source expected the old sentence at `e2e/collaboration-delivery-web.spec.ts:240`; the current working tree has already changed that expectation to the current locale text.

Smallest fix: keep the test assertion aligned with the locale string (`The issue was not created. Your draft and files are preserved.`). No product change is indicated. After the assertion update, rerun this focused E2E and the two parent-selector failures separately; then refresh the final full-check candidate because v11's candidate/source binding predates the correction.

## Caveats / Not Found

- The v11 result did not include a trace showing the request timeline, but the error-context DOM snapshot is sufficient to establish that the feedback was painted before the assertion timeout.
- The current working tree is dirty and already contains the one-line test correction; this research did not modify code or tests.
- This conclusion applies to the v11 candidate and its exact locale/test mismatch. It does not claim the full candidate passes until a source-bound rerun completes.
