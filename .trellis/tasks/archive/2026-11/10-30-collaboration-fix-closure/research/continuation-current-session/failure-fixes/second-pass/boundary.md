# Second browser-fixture maintenance boundary

The retained v12 directed run passes the repaired parent selector and reaches a
second stale interaction: secondary editing actions now live in the shared
Issue actions region's More actions disclosure. The current ordinary-comment
flow publishes after one explicit Send comment action; preview remains optional.
The source route canonicalizes UUIDs to displayed identifiers.

Owned files:

- `e2e/fixtures/collaboration-editing.ts`: reveal the current secondary-action
  disclosure before using edit/source/iteration actions, using the same real
  button/disclosure pattern as `openJourneyAction`; ordinary comments use one
  explicit send. Retain all source UUID, stage, field, iteration, reply, history,
  reaction, resolve/reopen, original-PATCH recovery and tombstone assertions.
- `e2e/collaboration-editing-web.spec.ts`: reveal source/recipient controls,
  assert canonical source navigation, and check forbidden external actions only
  after opening the real secondary-actions disclosure. Preserve catalog privacy,
  read/write, narrow layout and access-loss assertions.
- `e2e/collaboration-account-desktop.spec.ts`: root widened ownership to the
  existing editing scenario's final Edit issue click after discussion/navigation.
  Reveal that action through the same helper before the original 200% zoom
  description, viewport, tab, error and account-header assertions. Other scenarios
  in this file are outside this pass.

This pass changes no product/API/permissions/copy, timeout or skip condition.
The frozen v12 candidate and its failed logs remain untouched. Root owns live
browser/native runs and all services; this lane runs only package/type/discovery
checks and writes new evidence in this directory.
