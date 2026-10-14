# Core field editing and recipient resolution

## Change boundary

The existing core collaboration client can edit title/body/dates and perform
human handoffs, but cannot carry priority, label-set or property patches, read
authorized field options, or resolve an exact recipient in an explicit context.
This slice owns those response schemas, request types, API methods and focused
tests. Server authorization/writes, routes, UI and task completion stay with the
other assigned lanes.

Use the existing private collaboration transport, decimal version validation and
operation receipts. Preserve original keys and omitted/null request fields;
missing new detail fields remain unknown. Validate critical field definitions
and value types without ordinary property-schema defaults that discard values.
New option/recipient responses must match the requested issue/source, and old
account/server responses must be rejected. No new dependencies or commits.

## Baseline

`packages/core/api/client.ts` already contained unrelated settings work. Before
editing, its exact contents and existing diff were saved outside the repository:

- `/var/folders/3n/gbt3p39s5pdc4l55js62gxnm0000gn/T/collaboration-core-baseline-rl1we2wn/client.ts`
- SHA-256: `1c60d3d02f8936ab3165d9cd95ec97c69c4866092b62ec372c700fedf12a0c50`

Only its collaboration import/methods will receive incremental edits. Other
dirty files are not owned by this slice.

## Verification plan

Add failing schema/API cases before implementation. Verify exact context/wire
fields, empty/null/omitted patches, original versions/keys, malformed definitions
and values, privacy in errors/logs, and delayed results after scope replacement.
Run focused collaboration tests, scoped ESLint and core typechecking; report
actual outcomes below. No DOM, database, real AI or native application tests are
part of this core slice.

## Results

The coordinated R42/R29 wire contracts are implemented in core:

- `editCollaborationIssue` carries priority, full label-set replacement and
  partial property patches. Omitted fields and outer null preserve their wire
  meaning; `[]` clears labels, a null property value clears only that key, and
  `{}` remains an explicit no-op. Original request key/versions are unchanged.
- `getCollaborationFieldOptions` reads only the issue-scoped options route. Its
  strict response includes source/issue identity and an exact decimal revision;
  the issue identity must match the request.
- Detail parsing retains optional new fields without synthesizing empty data.
  Present property values must have exactly matching minimal definitions and
  compatible value types. Referenced archived definitions and member UUIDs do
  not require present source membership. Editing options only contain active
  definitions, with valid nonambiguous select options.
- `resolveCollaborationRecipient` posts the original kind/value and exactly one
  issue or source context. A creation result must have a null issue ID and the
  requested source; an issue result must have the requested issue. It strips
  unrelated response fields and never calls broad user/workspace APIs. User-ID
  queries additionally match the returned recipient to the original UUID, while
  retaining the original submitted value and accepting UUID case normalization.
- Both reads use the existing private collaboration transport: no ambient
  workspace headers, redacted schema/HTTP logs, normal API error codes and
  `Retry-After`, and rejection of late responses after credential/server changes.
  Confirmation does not invite or transfer anyone.

Commands executed on 2026-10-14:

| Command | Result |
| --- | --- |
| `pnpm --filter @multica/core test collaboration/recipient-schema.test.ts api/collaboration-recipients.test.ts` (before implementation) | Expected red: 28 failures from missing recipient parser/API |
| Same command after recipient implementation | 2 files, 29 tests passed; additional invalid-context regression included |
| `pnpm --filter @multica/core test collaboration/fields-schema.test.ts api/collaboration-fields.test.ts` (before implementation) | Expected red: 73 failures, 1 pass; missing field route, omitted new PATCH fields and unsafe dropped malformed values |
| `pnpm --filter @multica/core test api/collaboration-recipients.test.ts -t 'binds a user-ID confirmation'` (before target-identity hardening) | Expected red: a different recipient UUID was accepted despite matching issue scope |
| `pnpm --filter @multica/core test collaboration` (final run after target-identity hardening) | 11 files, 354 tests passed |
| `pnpm --filter @multica/core exec eslint collaboration api/collaboration-fields.test.ts api/collaboration-recipients.test.ts api/client.ts` | Exit 0 |
| `pnpm --filter @multica/core typecheck` | Exit 0 |
| `git diff --check -- packages/core/collaboration packages/core/api/client.ts packages/core/api/collaboration-fields.test.ts packages/core/api/collaboration-recipients.test.ts .trellis/tasks/10-10-cross-workspace-collaboration/research/core-fields-implementation-2026-10-14.md` | Exit 0 |

pnpm printed its existing configuration warning about `pnpm.onlyBuiltDependencies`
and `pnpm.overrides` being ignored in package.json. This slice does not change
package-manager configuration or dependencies. A baseline-to-current comparison
of `client.ts` showed only collaboration imports, two new methods, and the added
edit field mapping/type entries; unrelated settings changes remain intact.

Files changed: `packages/core/collaboration/schema.ts`, `index.ts`,
`test-fixtures.ts`, `fields-schema.test.ts`, `recipient-schema.test.ts`;
`packages/core/api/client.ts`, `collaboration-fields.test.ts`,
`collaboration-recipients.test.ts`; and this report. The existing private
transport and shared operation writer handle both additions; no parallel
request stack or new state store was introduced.

## Limits and handoff

- These tests prove core parsing/transport contracts using fixture HTTP responses,
  not the backend authorization, transaction, rate-limiter or normalization rules.
  Backend lanes and root own those tests and the final DTO comparison.
- No views, routes, React Query wiring, native client, database or real AI tests
  were run by this slice. Full P1/P3/P6/P8 acceptance remains pending.
- HTTP errors intentionally keep the existing `ApiError` response semantics for
  caller handling. Private backend messages/bodies are redacted from logging;
  this change does not replace all collaboration errors with a new sanitizer.
- No commits were made. All changes remain reviewable in the shared worktree.
