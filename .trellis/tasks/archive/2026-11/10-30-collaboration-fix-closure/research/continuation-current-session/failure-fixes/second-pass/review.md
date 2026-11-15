# Second pass source review and static checks

The retained v12 editing failures reach a hidden secondary action, after the
first parent-picker repair succeeds. The fixture now opens the existing More
actions disclosure inside the labelled Issue actions region before clicking
edit/source/iteration actions. This uses the same real-user pattern as the
verified detail journey. The native editing scenario uses the helper again for
its post-discussion 200% zoom check. Other native scenarios were not edited.

Ordinary seeded discussion content has no explicit AI target. Its helper now
uses one Send comment click and relies on the product's validated internal
preflight; it still checks the actual 201 response and retained comment. The
original code had no intermediate preview-content/AI-confirmation assertion to
remove. Reply, history, original-key edit recovery, PATCH count, reactions,
resolution, revision and tombstone assertions are intact.

Web source navigation additionally waits for the canonical issue identifier.
Recipient edit controls are revealed through the real disclosure; the absence
of source editing/iteration controls also checks hidden buttons, so collapsing
cannot mask forbidden controls. Catalog privacy, same-issue editing and
access-loss checks remain.

Workflow/comment component tests passed 29/29 in three package-scoped files.
Current three-file E2E TypeScript validation and diff check exited 0. Playwright
discovery found the original 15 tests; it did not execute browsers. Exact
commands, source/log hashes and the delta from untouched frozen v12 are retained
in verification.json and second-pass.patch. Root owns all live browser/native
runs and complete acceptance. No product, API, timeouts, skips or dependencies
were changed, and this lane called no service, database or Provider.
