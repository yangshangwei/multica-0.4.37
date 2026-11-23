# R27 managed-submission preview: RED source preparation

This is the implementer's compile-only RED slice. The new regression source and
its real Go AST inventory are frozen. The main session owns the first actual
test execution and any subsequent GREEN authorization. No product change,
database query, test body, TestMain, API or browser execution occurred here.

## Owned changes and protected inputs

- New test only:
  `/Volumes/artisan/.multica-acceptance/pay241-r27-preview-20261117-x12xjirm/source-red-r01/server/internal/service/collaboration_completion_preview_managed_submission_test.go`.
- New task-local report: this document and its matching JSON file.
- New private evidence support under the R27 root's `evidence/` directory.
- The accepted R26 source has 5,048 protected files. Every protected SHA256 and
  permission mode matches `evidence/source-before.json`. The sole additional
  source path is the new test; the new complete source has 5,049 files.
- ROOT product, existing fixture/test/SQL/generated files, historical evidence,
  dependencies and task artifacts other than these two new reports were not
  modified. No subagents were spawned.

## Genuine setup and intended assertions

Each leaf starts with the existing `newStageCompletionFixture(t)`. That fixture
uses the actual managed/password registration, claim, report and human approval
chain, a controlled successful deployment and the real
`AdvanceStage(..., "verification", ...)`. It then calls the existing
`stageCompletionM1Policy`, `stageCompletionSubmit` and
`stageCompletionAcceptParams`. The submission ID is the real
`SubmitAcceptance` confirmation. The target is built by the existing
`completionCollaborationTargetFromWorkflow` from fresh `ResolveIssueAccess`;
the cached pre-submission issue is not used for its revision.

No M1 grant, synthetic ready response, SQL-seeded submission, arbitrary target
revision, R4 read marker or relaxed stage check is introduced. The default
artifact fixture already supplies the creator's current source/initiator read
authority; independent artifact/product-role setup is unnecessary for this
slice. Existing helpers supply the real credential snapshot and transaction
trace rather than replacing returned database facts.

`TestCollaborationCompletionPreviewManagedSubmission` contains exactly these
five literal leaves, for one top-level, five leaf and six named identities:

| Leaf | Required actual result |
| --- | --- |
| `enforce_authorization_available` | `authorization_available`, exact current root/submission/ordinary-child/artifact/consumer/project binding, canonical passed observation and saved evidence/credential/release expiry; empty stage blockers |
| `policy_off_non_authorizing` | `policy_off`, empty stage blockers and nil observation/binding/expiry |
| `stale_target` | Original service 409 `delivery_completion_conflict`; only the observed exact issue revision is incremented |
| `wrong_actor` | Original service 403 `delivery_completion_denied`; the genuine current external handler is first shown to retain participation and lack source membership |
| `expired_credential` | Original service 403 `delivery_completion_denied`; only the actual creator JWT principal expiry is moved into the past, preserving the password-session snapshot |

The reviewer is not used as the wrong actor: a current source manager may
legitimately preview for the creator consumer. Every successful preview must
retain `enforced=false`, `usable_for_completion=false` and
`skip_supported=false`. Refusals may retain public internal envelope fields,
which the existing HTTP adapter discards, but may retain no private observation,
binding, expiry, completed check or stage result. This does not impose a new
universal zero-DTO contract on the existing service.

Each preview uses `beginCompletionCandidatesRead` and an actual
`pool.Begin` transaction with the existing five-second action context. It calls
`GetCollaborationCompletionPreviewInTx`, the final credential check and actual
commit, or rolls back on refusal. Setup occurs before that borrowed transaction;
the default pool and isolation are preserved. The main's ten-minute suite budget
remains unchanged. A failing positive leaf does not suppress the other four
named leaves.

## Side-effect and diagnostic evidence

Before preview, durable completion authorizations, terminals and evaluations
must be empty. `completionCandidatesNoWrites` reuses the existing complete
72-table rows/counts and stage-exit snapshot. It covers the current root/child
status and identity, formal/stage submissions, completion facts, stage approvals,
ordinary/collaboration queues, execution, operations, events, notifications,
attachments, inbox and current account/artifact/PR authority. A separate frozen
JSON comparison under the controlled source's mutex covers its actual call,
target and last invocation state. These checks run before fixture cleanup,
including after a failed assertion.

The forwarding transaction tracer retains original PostgreSQL failures without
rewriting SQL, arguments or returned rows. The test logs the actual normalized
service status/code, backend identity, returned reason and stage blockers.
Therefore the main's RED stream will record the real refusal, rather than
equating every negative with 409. No blocker has yet been observed at runtime
by this implementer. Static inspection suggests the original preview's temporary
submission guard does not initialize the bound submission; that remains a
candidate diagnosis until the main's genuine RED result.

The reused snapshot does not cover every database table, immutable knowledge
history, installation/binding tables or archive filesystem bytes. It is not an
all-table audit or a new tar-byte verification. R24/R25 historical lock holders
remain unproved. This slice is not complete preview read-lock, full S2, payment,
remote CI, deployment or parent 24-case acceptance.

## Frozen verification

- Formatted test SHA256:
  `f1aa77c0749504ee1826942c1511e5ad751a2254c8385ac69c2b0514ebc865e7`.
- `submission-preview-ast-r01.stdout.json`: real standard-library Go AST,
  exactly one top-level/five leaves, matching the frozen plan; no parallel or
  skip calls. The accepted R26 AST helper was copied into new R27 evidence;
  its original bytes and results were preserved.
- `submission-preview-freeze-r01.json` and
  `submission-preview-source-after-r01.json`: full protected-byte/mode and
  complete 5,049-file identity records.
- One compile-only command, with no inherited environment:
  `/opt/homebrew/bin/go test -c -o <R27root>/evidence/service-submission-red.test ./internal/service`,
  cwd `<R27root>/source-red-r01/server`.
- The JSON environment hash is
  `ed97bfafe79667d36317fa5035fe25d7248377bf653b452169b93b59a1ac8420`.
  The existing `db_audit_r01.load_environment` parses that environment without
  calling a database function.
- Compile evidence is recorded separately in
  `submission-preview-compile-r01.json` and its exclusive stdout/stderr logs.
  The single prescribed compile finished with exit 0, no timeout and empty
  stdout/stderr. The unexecuted binary SHA256 is
  `67f63f18ddf2fbdeb73f9a28c373dbd181a20117b658524d1ee6e0f719d4b697`.
- `submission-preview-postcompile-r01.json` rechecks every frozen source file
  after compilation: 5,048 protected files, zero byte/mode mismatches and
  exactly the same 5,049-file source manifest. It also confirms all compile
  log/binary hashes and the existing snapshot's exact 72-table inventory.

Next gate: the main verifies the frozen source and compile result, then runs the
single new selector against its explicit RED database and retains every leaf's
original result. Product edits require a separate bounded GREEN dispatch after
that result.
