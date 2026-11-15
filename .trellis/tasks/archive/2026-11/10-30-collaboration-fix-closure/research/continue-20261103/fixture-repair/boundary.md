# Parent issue fixture repair boundary

The preserved v11 native editing run fails at
`e2e/fixtures/collaboration-editing.ts:113`: `selectOption` targets the labelled
Parent issue button, and Playwright refuses because it is not a native select.
Current `CollaborationSourceInputs` uses a searchable `PropertyPicker` with
button rows; Project and Iteration still use native selects. Commit
`4ba3a868a` introduced the parent picker before candidate304f74dd3.

Only `e2e/fixtures/collaboration-editing.ts` in a new isolated worktree may change.
Replace the stale parent-control action with its real labelled trigger, visible
search field, matching parent button, popup closure and selected-label checks.
Preserve every existing exact parent UUID, stage, project, iteration, authority,
revision and discussion assertion. The current source and public API behavior
remain unchanged.

The existing Web editing and Desktop initial-fields tests provide the regression:
v11 retains the original failing native evidence; root will execute their
unchanged test bodies with the repaired shared fixture after the complete run.
This implementation lane performs source/diff/syntax checks only, with no live
services, browser, database, Provider, account, dependency or commit action.

No live v11 files, old evidence, other agent files or global task metadata are
writable here. No forced click, sleep, timeout change or assertion weakening is
part of the repair.
