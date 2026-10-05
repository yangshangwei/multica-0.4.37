# RR-02: raw imperative markdown versus normalized editor flush

2026-10-06. A final real-browser publication flow revealed an additional RR-02 boundary after the earlier plain-text regression passed.

## Observed cause

The parent captured `.omx/p1-preview-wire.json`: the successful preview request body ended with whitespace after an agent mention; the server response trimmed that whitespace, returned200 and included the expected member recipient. The UI returned to editing rather than preserving Publish. This was not a React crash or missing recipient.

`ContentEditor.getMarkdown()` intentionally returns raw serialized markdown (`content-editor.tsx:902`), while debounced onUpdate and unmount flush call `normalizeEditorMarkdown`, whose normalization is `md.trim()` (`:96`–`:102`). The composer had stored the raw imperative body before preview, so the later normalized flush looked like a genuine edit and reset a successful preview.

## Bounded change

Only the project's progress composer changes. Its imperative reads for preview, cancellation and deletion-only local-text capture now trim the body, matching the existing editor callback contract. The global ContentEditor API remains unchanged. A genuinely different body still invalidates earlier preview consent.

## RED → GREEN

The shared editor substitute now preserves raw getMarkdown while trimming callback/unmount values, including member/agent mention markdown and trailing space.

Before the source fix, the targeted two-case run produced **1 failure / 1 pass**: the trailing-space mention case returned to the editor; the genuine-edit invalidation case already passed.

After the fix:

- `pnpm --filter @multica/views test projects/components/project-review-regressions.test.tsx projects/components/project-management.test.tsx`: **2 files / 26 tests passed**.
- `pnpm --filter @multica/views typecheck`: passed.
- Scoped ESLint on the changed composer and regression file: passed.
- `git diff --check`: passed.

The regression verifies normalized outgoing body, retained member recipient, visible normalized preview body, enabled Publish and no remounted editor. Its companion verifies a genuine edit while the preview request is pending prevents the old response from becoming publishable. Existing cancellation, navigation, session and revocation recovery cases remain in the passing targeted suites.

Remaining parent verification: rebuild Web/Desktop and rerun the real publication/E2E flows. This document does not claim those rebuilt results before they run.
