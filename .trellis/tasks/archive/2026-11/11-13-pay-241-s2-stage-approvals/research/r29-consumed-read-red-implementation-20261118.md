# R29 consumed authorization read: RED test source

The new 550-line regression is formatted and statically inventoried as three
top-level/ten leaf/thirteen named identities. All 5,051 existing source files
retain their original bytes and modes. The new test is the sole source addition,
for 5,052 files. This implementer has not compiled or executed it; main owns
real Go AST/compilation, the exact original-product RED and all database choices.

Owned new test:
`/Volumes/artisan/.multica-acceptance/pay241-r29-receipt-read-20261118-6dhi01s2/source-red-r01/server/internal/service/delivery_completion_consumed_read_concurrency_test.go`.

Its SHA256 is
`d0633ca0e78712c49c1b40856cfc5a4a80ef065212e9198fac3742a76493071d`,
mode `0644`. Both new report names were checked absent before writing. Existing
helpers/products/tests/SQL/generated files, ROOT server, configurations, prior
evidence/history and peer edits were not modified. No dependencies or agents
were added.

## Diagnosis and retained limits

The complete diagnosis was read and its Markdown SHA256 verified as
`780cb85add6cbf815f58f7906fedd0066b78a50c0419cf0a902ebf16c1325479`.
It records Desktop's successful original completion and consumed-grant GET,
followed by Web's successful own-operation GET and failed consumed-grant GET.
Those actual Web requests are sequential. Early project probes were reached.
The original write/authorization must not be replayed; its physical 409 holder
and exact query/SQLSTATE remain unproved.

The source shows GetByID's original UPDATE grant/root/source and issuer-artifact
read fences, while the operation getter uses current-user SHARE and own-operation
SELECT. The new test establishes deterministic current-fixture read conflicts;
it does not assign those conflicts to the old runtime failure.

## Genuine consumed setup and snapshots

Each leaf uses existing low-level helpers in this order:
`newStageCompletionFixture` → M1 enforce → real `stageCompletionSubmit` →
`stageCompletionAcceptParams` → `stageCompletionM1ForLifecycle(f.ctx, params)`
→ exactly one real `ChangeLifecycle` with that binding. The same actual P is
issuer and consumer. A real grant, consumed terminal, immutable submission and
completed status revision result from the actual services.

The builder checks exact original target/request digest/consumer/body, selected
ordinary-child artifact/approval/spec/source/receipt/producer identity, terminal
actor/operation/submission and before/after status revisions. Expected minimal
completion receipt is assembled from the actual completed response and retained
terminal, rather than seeded rows or a fabricated ready response. It then calls
real unheld GetByID, requires `state/applicability=consumed` and that exact
receipt, and reads the real P's own operation with `200/completed` and the exact
delivery confirmation.

Only afterward is the no-write baseline registered. The three authority controls
register it after their intentional D/password/source changes. The prohibited
preview fixture with pre-registered zero-grant/equality cleanup is never called
or weakened. `r28NoWrites` is reused only as a projection adapter over the genuine
base fixture, retaining complete 72-relation, stage-exit, root admin/audit/control
and mutex-protected adapter call/target/invocation equality. It creates no actor,
grant, role, preview or stage fact. This is not an all-table or fresh archive-byte
audit.

## Exact frozen inventory

| Top-level test | Literal leaves |
| --- | --- |
| `TestDeliveryCompletionConsumedGetReadCoexistence` | `r4_first`, `authorization_first`, `same_grant_siblings`, `operation_first`, `operation_after_authorization` |
| `TestDeliveryCompletionConsumedGetWriterFences` | `writer_first`, `reader_first` |
| `TestDeliveryCompletionConsumedGetCurrentAuthority` | `wrong_actor`, `changed_password`, `source_revoked` |

There are no skip/parallel calls. Each top/leaf identity is static and preserved
even when another leaf fails; no old test count is added as new coverage.

## Transaction, trace and cleanup boundaries

GetByID receives an actual borrowed `pool.Begin` transaction through the existing
SQL-forwarding `completionCandidatesTraceTx`. It uses the real reader credential,
checks the final credential and commits, or leaves the successful first read
uncommitted while the second method runs. Every two-order/sibling case requires
distinct nonzero backend PIDs and the second actual method's commit/rollback or
terminal before first release. Held GET context/transaction status is checked
before commit. Readers retain the original five-second contexts, default preview
isolation, ordinary pool and existing 250ms artifact transaction preparation.

After actual completion, R4's candidate/QA chain may legitimately be stale. The
test compares the complete stable real unheld R4 projection rather than forcing
valid approvals or resetting epochs. The held R4 uses the existing post-artifact
barrier (`ListCollaborationCandidateTestReportEvents`) with real P context.
`holdStandingExecutionR4` and its active-QA-chain assumptions are not used.

The operation's existing generic holder hardcodes reviewer context, so the new
test uses the existing barrier starter with the actual completed P/request key.
Both operation/Get orders require passing controls, matching actual user-SHARE
semantics; they are not forced to fail as RED.

Writer-first calls actual `RevokeDeliveryCompletionAuthorizationInTx` on the
consumed grant. A forwarding row wrapper counts successful real terminal scans,
without changing SQL/arguments/rows/destinations. Semantic 409 must occur after
one consumed-terminal scan, with no PostgreSQL failure and a live transaction
still holding original UPDATE locks. The second real GET must refuse through
actual `55P03/LockDeliveryCompletionAuthorization`; fresh consumed GET recovers
after writer rollback. Reader-first requires original revoke to fail at that
upfront UPDATE with zero terminal scans, then its original semantic consumed
refusal must recover after GET release. No terminal is revoked or rewritten.

Authority controls retain original credentials: actual different participant D
is expected to receive 404 `delivery_completion_not_found`; canonical password
CAS increments the real version and old P session is expected 403
`delivery_completion_denied`; actual source member deletion is expected 403
with no private receipt. P's own completed operation remains readable after
source membership loss. Exact returned domain/code and first real PostgreSQL
error are logged; these are predictions until main's RED, not executed results.

Existing cancellation/rollback plus one-time barrier release and channel drains
run before equality snapshots and fixture cleanup, including failures. The
first holder must remain pending/within its original deadline until the second
terminal. No SQL result, cached 200, permission, stage fact, epoch, isolation,
timeout or assertion is rewritten to obtain success.

## Static verification and main-owned commands

Only gofmt of the new file and read-only/static checks ran. Static literal
function-body inventory matches the exact three-top/ten-leaf map. Complete
comparison to `evidence/source-before.json` found zero byte/mode mismatches and
only the new test path. The canonical current manifest fingerprint is
`b3d0cf3a5445492bd605b8d4b1ecf5233fcc40a0659f2a46a6ccfa85e6518c7d`
(sorted-key compact JSON rows with a trailing newline).

Main must run real AST/type validation and compile from `source-red-r01/server`
with its decoded, recorded RED profile and no inherited environment. A concrete
compile-only command, selecting a new main-owned output path, is:

```text
/opt/homebrew/bin/go test -c -o <R29root>/evidence/service-consumed-read-red.test ./internal/service
```

The exact main-owned RED selector/command is:

```text
/opt/homebrew/bin/go test -json -count=1 -timeout=10m ./internal/service -run '^(TestDeliveryCompletionConsumedGetReadCoexistence|TestDeliveryCompletionConsumedGetWriterFences|TestDeliveryCompletionConsumedGetCurrentAuthority)$'
```

This implementer ran zero Go builds/tests/list/TestMain, SQL/DB/API/browser,
signals or Git operations and spawned no agents. Source remains unchanged for
main's compilation/real RED and a later explicit bounded product dispatch.
Historical holder, complete joint journey/A01–A12/S2 and parent/S3–S5 acceptance
remain open.
