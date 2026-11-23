# Research: Parent issue picker failures

- Query: Explain the v11 `locator.selectOption` failures in the shared collaboration creation fixture, and identify the smallest correct repair.
- Scope: internal
- Date: 2026-11-03

## Findings

The two failures in the v11 browser run have one cause. The recorded candidate still used the old fixture interaction at `e2e/fixtures/collaboration-editing.ts:113`:

```ts
await dialog.getByLabel("Parent issue", { exact: true }).selectOption(fixture.parent.id);
```

The current product implementation renders the parent control through `PropertyPicker`, not a native `<select>`. `packages/views/collaboration/field-inputs.tsx:123-142` gives the trigger an accessible label (`triggerAriaLabel`) and renders a `Button`; the parent choices are `PickerItem` buttons in the picker content. Therefore Playwright correctly resolves `getByLabel("Parent issue")` to a `<button>` and rejects `selectOption` with “Element is not a `<select>` element”. The same helper is called by both `e2e/collaboration-account-desktop.spec.ts:544` and `e2e/collaboration-editing-web.spec.ts:22`, which explains the Desktop and Web failures together.

The smallest repair is test-fixture-only: open the trigger by role, choose the rendered parent option by role/name, then fill the current stage label. The working tree already contains this direction in `e2e/fixtures/collaboration-editing.ts:113-117`, but it must be preserved in the candidate that is actually rerun:

```ts
await dialog.getByRole("button", { name: "Parent issue", exact: true }).click();
const parentOption = page.getByRole("button", { name: `${fixture.parent.identifier} ${fixture.parent.title}`, exact: true });
await expect(parentOption).toBeVisible();
await parentOption.click();
await dialog.getByLabel("Subtask stage number (optional)", { exact: true }).fill("2");
```

The fixture type must expose `identifier` (`e2e/fixtures/collaboration-editing.ts:17`) because the rendered option includes it (`field-inputs.tsx:138-140`). The runtime object returned by `source.createIssue` already carries the identifier; the type omission was local to the test fixture. The current working-tree change adds that field and uses `fixture.parent.title`, which matches `seedCollaborationEditing` (`:51`) and the rendered `parent.name` DTO field. A stale candidate variant used `fixture.parent.name`, which is not present on the fixture type and should not be adopted.

The stage label also changed with the UI contract: `field-inputs.tsx:145-147` exposes `Subtask stage number (optional)`, so the old `Stage under parent` locator would be the next failure after the picker is fixed. The current working-tree update changes both creation and edit paths to the new label.

No product code change is indicated. Both failing tests call the shared helper, so one fixture update repairs both failures; the individual Desktop/Web specs should remain unchanged.

## Files found

- `packages/views/collaboration/field-inputs.tsx:94-147` — authoritative parent picker implementation and accessible labels.
- `e2e/fixtures/collaboration-editing.ts:17,51,113-118,147` — shared fixture type, seed data, and stale/current interaction paths.
- `e2e/collaboration-account-desktop.spec.ts:535-545` — Desktop test invoking the shared helper.
- `e2e/collaboration-editing-web.spec.ts:9-22` — Web test invoking the shared helper.
- `/Volumes/artisan/code/2026/mc-collaboration-final-owned-20261103/.omx/private/final-check-v11/make-check-v11.log:114445-114553` — v11 error evidence for both selector failures.

## Related specs

- `.trellis/spec/web/frontend/e2e-run-environment.md` — browser checks must use a production Web server; the v11 run did so through `make check`.
- `CLAUDE.md` testing rules — E2E flows belong in `e2e/` and should use the shared fixtures/helpers.

## Caveats / Not Found

- The v11 candidate completed with 3 browser failures; this note only explains the two shared parent-picker failures. The separate creation-rejection notice failure needs independent diagnosis.
- The candidate worktree has an uncommitted helper edit; its exact contents must be folded into the next bound candidate before rerunning. Do not assume the main checkout’s uncommitted test edit was part of the failed v11 source hash.
