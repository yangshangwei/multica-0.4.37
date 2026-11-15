# Named attachment region compatibility

Confirmed new UX component change: CollaborationCommentAttachments now has an accessible Attachments region. It has no heading; the whole-issue CollaborationAttachmentPanel has its own Attachments heading. Therefore a page-wide getByRole(region, Attachments) can match both legitimate surfaces, especially after posting a file-bearing comment or opening command history.

Affected explicit locators:
- e2e/collaboration-delivery-web.spec.ts:149 (after posting comment-preview.png) — deterministic ambiguity in new UI.
- e2e/collaboration-account-desktop.spec.ts:360,388 — potential when recorded/comment attachments exist; current scenario initially only creation file, so no forced rewrite without evidence.
- e2e/fixtures/collaboration-retained-attachments.ts:97,169,355 — current concurrent core-owner file. Do not edit without handoff. Proposed issue-panel locator: page.getByRole("region", { name: "Attachments", exact: true }).filter({ has: page.getByRole("heading", { name: "Attachments", exact: true }) }).

Preserve reference-card preview locators scoped by their owning comment/command/record. Do not hide an ambiguous target with .first(). The old frozen1a candidate has only the original whole-issue region, so its existing result is not invalidated and needs no immediate rebuild for this finding.
