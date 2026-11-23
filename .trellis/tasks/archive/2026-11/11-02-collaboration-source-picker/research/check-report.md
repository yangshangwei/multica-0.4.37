# Source-picker final code check

## Findings (fixed)

None. This check made no product edits and did not invalidate the candidate browser build.

## Findings (not fixed)

No confirmed code defects or unresolved design/boundary findings in this task's owned changes.

The review covered the shared parent selector, creation/source-edit query-state wiring, the optional `PropertyPicker.searchMaxLength` passthrough, both locale additions, source-parent SQL/service/generated code and their focused regressions. Unrelated dirty acceptance tasks, app environment files and existing backend test changes were outside this review.

## Behavior and boundary review

- Empty search reuses the current source's existing protected options and opens visible candidates. Reopening clears the picker query. The recent-200 list retains truthful truncation feedback; it is not described as exhaustive.
- Pending or failed searches render no selectable stale candidate rows. Parent selection retains its UUID and display text across result changes; the optional clear action remains separate from real search matches.
- Creation transient failures reuse only Query data under the current Server/viewer/source key, preserving the mounted form. Actual denial is excluded from that fallback and the existing protected source-access cleanup erases source data/drafts.
- Source-field editing uses the same picker and blocks confirmation after a failed field read. Existing generic field-error retry also covers an initial error with no retained catalog.
- The existing picker owns Arrow/Enter/IME handling, close/reset and nested-popover focus. Source candidate/clear rows and the trigger retain 44px targets, use semantic tokens and wrap long titles. New retry/selection actions are explicit `type="button"`.
- Full displayed identifiers are compared against `util.ResolveIssuePrefix` used by the same source's display projection. SQL keeps literal title, bare number and UUID predicates, workspace/self/admission filters and the recent-200 window. It does not strip unknown prefixes, change eligibility or alter the API response/schema.
- The new component regressions exercise actual primitives and caller Query wiring; the identity/filter matrix stays in canonical backend tests.

## Verification executed by this reviewer

All commands below exited 0:

```sh
pnpm --filter @multica/views exec eslint \
  collaboration/field-inputs.tsx collaboration/field-inputs.test.tsx \
  collaboration/creation-form.tsx collaboration/creation-form.test.tsx \
  collaboration/workflow.tsx collaboration/field-editing.test.tsx \
  issues/components/pickers/property-picker.tsx
pnpm --filter @multica/views typecheck
pnpm --filter @multica/core test \
  collaboration/field-queries.test.ts collaboration/delivery-api.test.ts \
  collaboration/fields-schema.test.ts
gofmt -l server/internal/service/collaboration_source_fields.go \
  server/internal/service/collaboration_source_fields_test.go \
  server/pkg/db/generated/collaboration_source_fields.sql.go
git diff --check -- \
  packages/views/collaboration/field-inputs.tsx \
  packages/views/collaboration/creation-form.tsx \
  packages/views/collaboration/workflow.tsx \
  packages/views/collaboration/creation-form.test.tsx \
  packages/views/collaboration/field-editing.test.tsx \
  packages/views/issues/components/pickers/property-picker.tsx \
  packages/views/locales/en/collaboration.json \
  packages/views/locales/zh-Hans/collaboration.json \
  server/pkg/db/queries/collaboration_source_fields.sql \
  server/pkg/db/generated/collaboration_source_fields.sql.go \
  server/internal/service/collaboration_source_fields.go \
  server/internal/service/collaboration_source_fields_test.go
```

- Lint: pass, no task-file findings.
- TypeCheck: pass, `tsc --noEmit`.
- Core boundary tests: **68 passed in 3 files**, including protected field-option reads, creation API/schema compatibility and source-option projections.
- Go formatting and task-owned whitespace: pass, no findings.
- The existing pnpm package-field warning is unrelated and did not fail these commands.

## Additional evidence reviewed

- `ui-implementation.md`: **95 passing tests in 5 files**, covering source picker, creation/source-field wiring, existing PropertyPicker behavior and locale parity; final **7/7** selector rerun and file lint passed after the clear-row target-height adjustment. UI typecheck and changed-file lint passed before reviewer checks.
- `backend-implementation.md` and `backend-regression-green.log`: **13 selected top-level service tests passed** on the recorded independent database, including complete/case-insensitive/numeric/legacy identifiers, wrong-prefix rejection, older-than-window lookup and existing source authority/atomicity/iteration behavior. The original SQL failed the new identifier regressions; existing title/number/UUID/window cases remained green. Backend `go vet` was reported passing.

The reviewer did not repeat the already passed full UI or DB-backed service suites, create/migrate/drop a database, execute installed agent CLIs, rebuild apps or touch task/spec/journal metadata. Production browser captures and actual HTTP creation/save proof are the parent/browser lane's separate acceptance evidence. No full monorepo test run or native desktop acceptance is claimed by this check.
