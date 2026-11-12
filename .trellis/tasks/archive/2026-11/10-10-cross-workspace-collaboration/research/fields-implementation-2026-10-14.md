# R42 field editing implementation — 2026-10-14

This is a scoped continuation of the approved collaboration task. The public
creation gate and shared execution remain closed; this slice does not establish
full P1–P8 acceptance.

## Change boundary and extraction plan

- Extend the existing collaboration metadata PATCH to priority, the complete
  source issue-label set, and a partial custom-property value patch. Preserve
  text/date/status semantics, the four exact versions, and the original request
  key. Null/omission preserves each new field; an empty label array clears labels;
  a null property value clears that definition, including archived definitions.
- Reuse the workflow transaction owner for current creator/handler authority,
  source/I1/issue/session locks, durable refusal/replay, facts, history and receipt.
  Add field validation in its callback, with sorted property advisory try-locks
  and source definition/member SHARE NOWAIT locks. The ordinary property writer
  uses `prop:<UUID>` (and definition updates also use `props:<workspace>`); the
  ordinary label writer already takes the shared source I1 fence.
- Move only the pure typed property validation and its value/config helpers from
  `handler/property.go` to `util/issue_property_value.go`. Keep ordinary property
  administration, actor lookup, errors and single-key writes unchanged. Retain
  the existing handler tests as regression proof, updating only helper references
  if necessary. No new dependency, generalized validation layer or workflow owner.
- Add a source-only field-options read for active creator/current handler. Detail
  adds selected labels, values and only referenced definitions (including archived
  ones); neither read exposes usage counts, management metadata or actor directories.
- Owned files: collaboration edit service/handler/tests and SQL; new collaboration
  fields service/handler/tests; detail DTO; the narrow property helper extraction.
  Root owns router and generated sqlc edits. Other lanes' changes are preserved.

## Implementation

- `CollaborationEditParams` extends the existing `edit_issue` operation with optional
  priority, a complete label set and a property-key patch. Absent new fields are
  omitted from the durable payload digest, preserving pre-R42 metadata receipts.
  A present empty label slice has a distinct digest from omission. The service
  returns the existing content-free receipt and stores deterministic refusals.
- Selected labels must be source issue labels. Property keys are canonical UUIDs;
  alias collisions are refused. The ordinary validator still owns all nine value
  types and canonicalization, including exact select-option IDs and canonical
  member references. Actor values require a current source membership held with
  SHARE NOWAIT through commit, without exposing a member directory.
- Selected property advisory locks are acquired in UUID order using the ordinary
  `prop:<UUID>` key; ordinary definition updates hold `props:<workspace>` then the
  same property key. Definition rows also use SHARE NOWAIT. Conflicts roll back
  the entire existing workflow transaction and retry at most three times. The
  existing I1 workspace fence intersects ordinary label update/delete writers.
- Property patches preserve unrelated raw JSON values, including integers beyond
  JavaScript's safe range. PostgreSQL checks the same 16 KiB JSONB budget and JSONB
  equality before required writes. Equivalent numeric representations, duplicate
  label selection and unchanged values produce only a receipt.
- Value update, label replacement, applicable I1 facts, collaboration revision,
  history and receipt are atomic. History includes safe before/after label summaries
  and the property bags. Existing command snapshots, queue/run/grant state, account
  and lifecycle epochs, information requests and notifications remain unchanged.
- `FieldOptions` uses a read-only repeatable snapshot and current Edit authority;
  its DTO contains only source issue labels and active definitions. Detail builds
  role/version, values, selected labels and referenced definitions in one repeatable
  snapshot; archived definitions and retained departed-member references remain
  readable. Unreferenced definitions and ordinary catalog usage/management metadata
  are omitted. Broken definition config produces `collaboration_unavailable`.

## Changed files

- `server/internal/service/collaboration_edit.go`, `collaboration_edit_test.go`
- `server/internal/service/collaboration_fields.go`, `collaboration_fields_test.go`
- `server/internal/handler/collaboration.go`
- `server/internal/handler/collaboration_edit.go`, `collaboration_edit_test.go`
- `server/internal/handler/collaboration_fields.go`, `collaboration_fields_test.go`
- `server/internal/handler/property.go`, `property_test.go`
- `server/internal/util/issue_property_value.go`
- `server/pkg/db/queries/collaboration_edit.sql`, `collaboration_fields.sql`
- This evidence file.

Root separately owns router registration, generated sqlc files, core integration,
spec updates and the independent review. No migration, dependency or commit was
made by this lane.

## Verification evidence

All DB runs sourced the dedicated task-owned `fields.env` (schema 619) and used
`bash scripts/go-test-with-agent-cli-guard.sh -- go -C server test ...`. They did
not fall back to the application database or execute installed AI CLIs.

- Before extraction: existing ordinary property value/actor validation, actor UUID
  canonicalization, typed HTTP writes and in-use option-removal regressions passed
  (7 top-level test groups; 1.29 s).
- Red proof: the new `TestCollaborationEditHTTPSourceFields` received HTTP 400
  `invalid_request` when it expected a valid external handler's HTTP 200 edit.
- After extraction, the same ordinary validator/HTTP regressions passed along with
  collaboration handler tests. The full util package passed (0.56 s).
- Focused service field/edit matrix passed (11.97 s), including all value types,
  retained raw JSON, no-op/clear behavior, source and role denials, required-write
  rollback, prior receipt compatibility, unknown commit and durable refusals.
- Deterministic contention tests passed for the ordinary property advisory key,
  direct definition archival, member removal, label deletion and I1 label fence.
  Busy retries leave no receipt or partial write. Current creator/handler, transfer,
  account disable/revocation and I1 recorder regressions include R42 field edits.
- Actual-workspace race run passed the service edit/fields suite (14.556 s) and
  root's authenticated router edit/field-options/recipient/status scenario (2.263 s).
  The initial handler snapshot fixture incorrectly replaced the chi request
  context; it was corrected to derive the timeout from `r.Context()`.
- The actual handler race rerun with the broader filter
  `Collaboration|Property|Properties|ParseActorRef` passed (11.178 s), including
  the deterministic detail snapshot test and ordinary property behavior.
- Actual `go -C server vet ./internal/service ./internal/handler ./internal/util`
  passed. Formatting and focused whitespace checks passed.

During a temporary API failure of the coordinating session, repo-pinned sqlc
v1.31.1 generated to a private temporary directory; no workspace-generated file
was edited by this lane. The complete collaboration service race suite also
passed with that generated-only overlay (159.86 s). Root subsequently ran normal
`make sqlc`; all three affected generated files, including the other lane's
recipient queries, matched the temporary output byte for byte. Final focused
service, handler, router and vet checks used the actual workspace without overlay.

## Remaining scope

Full UI, P3 comments/attachments/history delivery, positive execution admission,
native containment and complete end-to-end acceptance remain outside this slice.
This evidence does not mark full AC73/P1–P8 acceptance or authorize opening the
creation/execution gates. Root's independent review and task-level integration
remain separate gates.
