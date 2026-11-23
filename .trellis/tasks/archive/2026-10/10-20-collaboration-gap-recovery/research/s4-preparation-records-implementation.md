# S4 protected preparation summary

Status: implemented and scoped checks passed; ready for independent review.
Owner: `s4_preparation_records`. Date: 2026-10-21.

## Change boundary

The negotiated runtime manifest and preparation observations exist, but human
run records currently omit their factual file/commit/stage summary. Add one
optional projection to the existing issue-authorized records page, preserving
protocol 1 and the existing record-item union. The service validates the frozen
run/resource/input correlation and reads only the latest observation per stage
at the page cursor's existing sequence watermark. It never projects invocation,
manifest/bundle digests, credentials, local paths or raw events.

Changes: `collaboration_records.go`, a dedicated preparation-record
helper and tests, one bounded SQL query, the core run-record schema and adjacent
schema/query tests, and one compact shared summary component mounted in the
existing record disclosure. The coordinator owns SQL generation, locales,
barrels, actual Web/Electron evidence and contract documentation. A small
adjacent query integrity guard was approved by the coordinator so conflicting
summaries cannot be cached in one page chain.

No runtime/claim/proof/lifecycle mutation, new control, inferred execution result,
provider support claim or general resource inventory is part of this change.
The component reuses the existing semantic typography and details layout; no
local refactor is planned.

## Delivered contract

The additive `preparation` field is omitted for unclaimed/legacy runs. The
existing protocol version remains 1. A supported summary contains only:

```text
version: 1
attachments: [{id, version, name, size_bytes}]
git: {object_format, commit_oid} | null
stages: [{seq, stage, state, code?}]
```

Stages are the latest real observation for each of attachments, bundle, Git and
workspace, sorted by their recorded sequence. They are not a complete event
history. The SQL read uses the existing cursor's `through_seq` and returns at
most five rows; an unknown stage or fifth row rejects the projection. Each
selected event must match the recorded run identity, producer event ID,
authorized attempt and original manifest. Manifest validation includes exact
frozen commands/resources and its private digest. No live runtime/configuration
lookup is used by the human projection.

The private invocation, manifest/bundle digests, attachment destinations,
capabilities and attempt nonces never enter the new DTO. Its presence does not
assert successful preparation. The service never changes outcome, proof,
reservation, hold or run state. Later file delivery/resource edits cannot rewrite
the summary, and actual proof plus engine/runtime cleanup preserve it.

Core validates bounded names/versions/bytes, full SHA-1/SHA-256 commits, stage
order/uniqueness and finite failure reasons; unknown optional summaries make
the records route unavailable under its existing issue-authority recheck.
Malformed data cannot add proof or control authority. Absent and explicit-null
legacy summaries remain supported. The page-chain guard compares the entire
public summary before cache writes, while a coherent full refresh may replace
the captured watermark. Existing denial cleanup still removes all pages.

The shared component mounts inside the existing lazy disclosure. It shows a
count and expandable exact file versions, fixed commit, and recorded/missing
stage facts with localized finite failure reasons. IDs, long names and full
commit OIDs wrap. Missing data is "Not recorded" / "未记录", not "not run".
It reuses the existing endpoint, transaction, cursor, protocol validation and
native details layout, with no new state store or controls.

## Files

- `server/internal/service/collaboration_records.go`
- `server/internal/service/collaboration_preparation_records.go`
- `server/internal/service/collaboration_preparation_records_test.go`
- `server/pkg/db/queries/collaboration_records.sql`
- `packages/core/collaboration/run-records-schema.ts`
- `packages/core/collaboration/run-records-queries.ts`
- `packages/core/collaboration/run-records-queries.test.ts`
- `packages/core/collaboration/run-preparation-schema.test.ts`
- `packages/views/collaboration/run-records.tsx`
- `packages/views/collaboration/preparation-summary.tsx`
- `packages/views/collaboration/preparation-summary.test.tsx`
- This report and `s4-preparation-records-locales.json`.

The coordinator generated SQL and merged the `runPreparation` locale fragments
into both collaboration dictionaries. The existing preparation-event filter in
the record-items query predates this slice and was preserved. No runtime,
S3/S5, API client/barrel, template or migration source was edited by this owner.

## Verification

Environment: task-owned `service_s4` database through migration 737, private
wrapper below, Go 1.26.6 on darwin/arm64. HEAD was
`8ecb98883504443a6ec1a5fb4a4c6295a985ae04` plus the shared uncommitted task work.
Database credentials were neither copied into the repository nor printed.

Failure-first evidence:

- `TestCollaborationPreparationRecordsFrozenSummary` failed with "claimed run
  omitted its recorded preparation summary" before the projection existed.
- The first two `run-preparation-schema.test.ts` cases failed because the
  parser discarded preparation and explicit null.
- All four initial `preparation-summary.test.tsx` cases failed because the
  protected record disclosure had no preparation region. Log:
  `/tmp/multica-gap-recovery-20261021-h6phxwo9/s4-preparation-summary-red.log`.

Final commands (Go from `server/`, pnpm from repository root), all exit 0:

```bash
python3 /tmp/multica-gap-recovery-20261021-h6phxwo9/with-db.py service_s4 \
  bash ../scripts/go-test-with-agent-cli-guard.sh go test -race -json \
  ./internal/service \
  -run '^TestCollaboration(PreparationRecords|RunRecords|RuntimePreparationObservationKeepsInstalledRecordsReadable)' \
  -count=1
GOTOOLCHAIN=go1.26.6 go vet ./internal/service
pnpm --filter @multica/core test run-preparation-schema.test.ts run-records-schema.test.ts run-records-queries.test.ts
pnpm --filter @multica/views test preparation-summary.test.tsx execution-panel.test.tsx run-navigation.test.tsx
pnpm --filter @multica/core typecheck
pnpm --filter @multica/views typecheck
pnpm --filter @multica/core exec eslint collaboration/run-records-schema.ts collaboration/run-records-queries.ts collaboration/run-preparation-schema.test.ts collaboration/run-records-queries.test.ts
pnpm --filter @multica/views exec eslint collaboration/run-records.tsx collaboration/preparation-summary.tsx collaboration/preparation-summary.test.tsx
```

- Go race: **10 top-level tests, 18 subtests passed; zero failed or skipped**.
  Includes six new preparation-record tests, three existing records tests and
  the installed-client record-kind regression. Exact JSONL:
  `/tmp/multica-gap-recovery-20261021-h6phxwo9/s4-preparation-records-race.jsonl`.
- Core: **103 tests across 3 files passed**. The schema suite covers private
  field stripping, invalid optional summaries, exact large sequences, command
  attachment inclusion and redacted diagnostics. Query cases cover conflicting
  page summaries, coherent refresh and complete denial cleanup.
- Views: **10 tests across 3 files passed**, including four new summary tests.
  Exact log:
  `/tmp/multica-gap-recovery-20261021-h6phxwo9/s4-preparation-summary-tests.log`.
- Both package typechecks, both scoped ESLint commands, service vet, gofmt and
  scoped `git diff --check` passed.

The earlier 9-top-level race run overlaps the final 10 and is not additive.
No real provider binary, account or quota was accessed, and no commit was made.

## Remaining evidence and handoff

Independent review and actual rebuilt Web/Electron preparation rendering remain
with the coordinator. The existing history-web-v4 narrow screenshot was viewed
only as a typography/wrapping reference; it is not evidence of this new summary.
No new visual screenshot, production/browser run or Linux provider test was
claimed by this slice. The coordinator also owns spec/built-in skill updates,
global verification and the full acceptance matrix; S4 and parent completion
must not be inferred from these scoped tests.
