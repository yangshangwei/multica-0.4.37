# R28 submission-preview read implementation

The frozen GREEN source changes exactly three product files. A validated
`collaboration_submission` preview now retains compatible shared read fences
through the existing issuer/report/run/artifact chain, using its original
borrowed transaction. Writer, ordinary, self and admin defaults remain on their
original dispatches. Compilation and service vet both exit 0.

Main reports the actual fresh normal union passed all 26 named identities:
seven top-level/nineteen leaf, including the unchanged R28 read/control inventory
and R27 binding/admin baseline. Its result SHA256 is
`d016f60680e3175d86ef8a7f1bdc77db5a23d881c0254e1c0aab22c23d794e81`.
Race and affected original controls remain main-owned. This implementer ran no
tests, TestMain, DB/SQL/API/browser commands, signals or Git operations.

## Genuine RED basis and narrow ownership

Original R28 RED result SHA256:
`e0cd8f3efef746a02a35d6f1baf5f54f4156302b3a1952f19b3ca1c30559d04f`.
It has thirteen PASS/five FAIL named identities: all ten writer/authority/binding
leaves pass; three read-coexistence leaves and two parents fail. No skipped,
missing, extra, pending, parse/drift identities or final DB connections remain.
The first real failures were root collaboration UPDATE in R4-first, shared
canonical child in preview-first R4 and root collaboration SHARE in sibling
preview. These prove their reached early fences, not every later query.

The reviewed repair design is
`research/r28-preview-read-repair-design-20261117.md/json`, JSON SHA256
`ef5b3a5a3fb19042ec19e247b534821268b772f4245e74201279731280474c74`.
Implementation was explicitly authorized after that real RED. Original RED,
its 602-line test, both R27 tests, all existing fixtures/SQL/generated files,
configurations, dependencies, reports and peer edits are preserved. ROOT product
is read-only for this implementation. No subagents were spawned.

## Local factoring boundary

Only these existing files in `source-green-r01/server/internal/service/` change:

| File | Change |
| --- | --- |
| `collaboration_completion_preview.go` | After original request/current-root/version checks, only submission targets construct the existing value marker; four selected dispatches use private/existing read helpers |
| `delivery_completion_collaboration.go` | Factor the existing intent body into one private mode helper; original intent passes constant false, submission-only private read entry passes true |
| `collaboration_stage_completion.go` | Factor the existing state/facts bodies into private modes; original state/facts/final-check wrappers pass constant false; private read facts select task/child SHARE |

The preview constructs exactly `collaborationStageEvidenceReadTx{Tx: tx}`, not a
pointer or outer wrapper, after the unchanged issue revision/status check. The
same value remains in `db.New(tx)`, selected intent/artifact/facts/evidence calls,
submission checks, retained QA issuer/report/source/run checks, release
reobservation and final signer/clock checks. It never Begins another transaction
or sets repeatable-read. R4's existing transaction owner and marker are unchanged.

Four submission-only dispatches change: private
`readCompletionCollaborationIntent`, existing `readDeliveryArtifactInTx`, private
`readStageCompletionFacts` and existing `evaluateDeliveryEvidenceReadInTx`.
Other target kinds retain the original transaction/helpers. The root/source
preparation, creator/consumer identity, exact submission/operation/handoff,
selected-vs-canonical child match, credentials, sources, bound-ID initializer,
UTC expiry, errors and permission flags are unchanged.

Intent mode affects only existing normal consumer access: read=true uses
`readCompletionCollaborationAccess`; original=false uses
`completionCollaborationAccess`. The new read entry rejects other target kinds.
All creator/handler/capability/epoch/status/catalog/current-submission and digest
predicates stay in the one existing body. Self preparation, admin nomination,
advisory serialization and nonempty attachment UPDATE locks are unchanged.

Read state performs the actual same-transaction id-only fence:

```sql
SELECT issue_id FROM collaboration_stage_task WHERE issue_id=$1 FOR SHARE NOWAIT
```

It then uses existing `loadStageDetails(..., false)`/`GetCollaborationStageTask`.
`pgx.ErrNoRows` is accepted solely as the original virtual absent head; other
errors remain fatal. The retained root SHARE excludes concurrent original head
creation. There is no bare unlocked getter, synthesized ready state, new404,
generated helper or `Ensure` call. Original state mode retains
`loadStageDetails(..., true)` and its UPDATE fence, even with an existing marker.

Read facts select that state mode and existing
`LockDeliveryEvidenceIssueRead` for the original canonical child/workspace IDs.
Original facts select the original state and `LockLifecycleIssue`. Every parent,
project, content, CI, approval, issuer, source/fact, rollback, final callback and
clock check remains in the same fact body and order. The existing marked nested
helpers supply their previously implemented shared fences; no validator is
duplicated and their source is not modified.

## Source and static evidence

The GREEN source has the same 5,051 paths/modes as RED; all 5,048 other files
are byte-identical. Each complete product file exactly equals a reconstruction
of only the described additions/dispatch replacements. Three patches and the
complete source manifest are under R28 `evidence/`.

| Product | GREEN SHA256 |
| --- | --- |
| Preview | `b7ea8fe6ef2490e95e405b78b265154bab69511fc5aaf2519c1babde1b2ae6da` |
| Intent | `20fc764b20edbfae356db7785f82ac17c0d12a71d23f9a0dcf318624a71e0084` |
| Stage | `cce754252303652d27a10c335e1e1279906a6b20a543d61b4822baf3a31ce7cb` |

The 602/224/174-line tests retain `31885022…`, `f1aa77c0…` and `658bb12c…`.
Real Go AST confirms the R28 five-top/thirteen-leaf/eighteen-named inventory.
Product AST confirms every original signature, exactly five changed bodies and
five added private factoring functions. Both new private read entry references
are confined to `GetCollaborationCompletionPreviewInTx`. Original intent,
state, facts and final-check writers pass constant false; read entries pass true.
Actual writer caller references and other unchanged functions are recorded.

The first product-AST utility failed because `go/printer` cannot print a
standalone `*ast.FieldList`; its source/stderr and empty stdout remain preserved
as r01. A new r02 evidence-only utility handles that node and passes the caller,
signature, marker-value, submission gate, shared-task/virtual-head and default
mode audit. This tooling failure was not a product/test failure; no product or
test was changed to repair the utility.

Exactly one compile-only command and service vet used the decoded GREEN JSON
profile, SHA256
`caf5fab0b231415876c45449397796bb8c822766b4b1b2383773d26c8893f06f`,
with no inherited variables or database function called by its loader.
Both exited 0 with empty stdout/stderr. The unexecuted
`service-preview-read-green.test` binary SHA256 is
`f9ec53f6fa624f06e05aec6788cae81cd7ba8cfb6c62bccab36d5665434eee3d`.
`preview-read-green-postchecks-r01.json` rechecks complete RED/GREEN manifests,
the three exact product deltas, unchanged tests, static log/binary and caller-AST
hashes. All identities match after checks; original RED remains unchanged.

## Preserved controls and remaining gates

The unchanged R28 controls exercise original observer both orders, original
stage-state plain/marked UPDATE, current account/password/artifact manager/QA
issuer loss and missing binding/actual rework. The unchanged R27 tests preserve
exact binding/UTC/off/stale-target/external/expiry and current-submission R38.
Actual normal union PASS is main evidence, not an implementer test execution.
Source false-default/caller equivalence does not substitute for the remaining
main-owned affected original completion and read/write controls.

Race union, affected controls and independent final GREEN acceptance remain
open until their actual evidence is complete. Nonempty attachment, self/admin
concurrency, ordinary preview, original four joint scenarios, A01–A12/S2 and
parent acceptance remain separate. Historical R24/R25 holders are unproved;
the metadata snapshots do not constitute a new archive-byte audit.
