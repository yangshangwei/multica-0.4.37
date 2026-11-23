# R29 RED source r02: grant-lock witness correction

The current new test is frozen at 558 lines, SHA256
`7e927d2721c00d98fead050c2129a223aac887b7cac8ef058a4789fa640a17d5`,
mode `0644`. The original 550-line draft (`d0633ca0…`) and original r01
implementation Markdown/JSON remain preserved. Main explicitly authorized this
single assertion correction before the first Go build or test execution.

## Exact correction

Only `writer_first`'s consumed-GET lock witness changes. Pinning its query name
to the pre-repair `LockDeliveryCompletionAuthorization` would require a later
shared-read implementation to fake that old SQL annotation or change the test.
The corrected assertion identifies the actual grant row-lock failure by all of:

- Original real PostgreSQL SQLSTATE `55P03`.
- Captured backend PID equals the actual reader transaction's backend.
- UUID arguments are exactly the single real grant ID, with no other UUID.
- The captured SQL SHA256 is valid canonical 64-character lowercase hex.
- Query is a lock/read/get intent for `DeliveryCompletionAuthorization`, or an
  actual `delivery_completion_authorization` ID row-lock statement with
  SHARE/KEY SHARE/UPDATE and NOWAIT. The original query remains accepted.

This retains the real SQL-forwarding error witness, exact semantic/domain409,
zero private response, actual consumed-terminal writer hold and unchanged
post-release recovery. It does not accept an arbitrary 409 or arbitrary
PostgreSQL failure, fabricate results, rewrite SQL or alter a product.

`reader_first`'s real Revoke still must fail at the exact original
`LockDeliveryCompletionAuthorization`, with `55P03` and zero terminal scans.
The original consumed terminal, same-P completion/Get/operation baselines,
private source/password/D denials, deadline, PID, before-release terminal and
no-effect assertions are unchanged.

## Preserved scope and inventory

The source change remains one NEW file:
`source-red-r01/server/internal/service/delivery_completion_consumed_read_concurrency_test.go`.
Complete exact reconstruction from the preserved draft proves that only the
authorized writer-first assertion block changed. All 5,051 existing source
files retain bytes/modes; current complete source has 5,052 paths with the same
sole addition.

| Top-level | Leaves |
| --- | --- |
| `TestDeliveryCompletionConsumedGetReadCoexistence` | `r4_first`, `authorization_first`, `same_grant_siblings`, `operation_first`, `operation_after_authorization` |
| `TestDeliveryCompletionConsumedGetWriterFences` | `writer_first`, `reader_first` |
| `TestDeliveryCompletionConsumedGetCurrentAuthority` | `wrong_actor`, `changed_password`, `source_revoked` |

The static inventory remains three top-level/ten leaf/thirteen named identities,
with no skip/parallel calls. Existing helpers/products/fixtures/SQL/generated
files, ROOT server, configurations, peers and historical evidence are unchanged.
No dependencies or agents were added.

The setup and cleanup contracts remain those documented in r01: each leaf has
one genuine same-P M1 authorization and actual completion, immutable consumed
terminal/artifact/request/receipt/status checks and actual unheld consumed Get
and own completed operation before the baseline. Negative setup precedes its
new baseline. R4 compares the actual projection after completion; no valid-QA
assertion or epoch reset is introduced. Get borrows the actual transaction,
uses real credentials and final checks, and completes the second method before
first release. Revoke terminal scans are real forwarded scans. Existing
72-relation/stage-exit/admin/adapter equality and LIFO release/cancel/drain/
rollback remain unchanged. Reader five-second and artifact250ms budgets remain.

## Evidence and main-owned execution

Main preserved the original draft in
`evidence/test-draft-r01.go.txt`, full SHA256
`d0633ca0e78712c49c1b40856cfc5a4a80ef065212e9198fac3742a76493071d`,
with `test-draft-r01-preserved.json`, SHA256
`4b9d6102962bcb1bba15772ee6a8d963ff25b40d95a70940ee8d297f338b4f09`.
Original report r01 was sealed separately and is not overwritten. The diagnosis
`780cb85a…` remains verified and its original runtime holder remains unproved.

After the correction, gofmt succeeds and read-only complete byte/mode/static
inventory checks pass. Current canonical complete-source fingerprint is
`8b75897917a1d02f04aad1cc881bb2dc06d5e41ca14c0982be11499f31285998`
(sorted-key compact JSON rows plus trailing newline). No Go AST/build/test/list/
TestMain, SQL/DB/API/browser, signals or Git execution occurred here.

Main must bind this r02 SHA to its real AST/compile-only and original-product
RED permits, using its recorded decoded profile without inherited variables.
Compilation can use a new main-owned binary path:

```text
/opt/homebrew/bin/go test -c -o <R29root>/evidence/service-consumed-read-red.test ./internal/service
```

The unchanged RED command, from `source-red-r01/server`, is:

```text
/opt/homebrew/bin/go test -json -count=1 -timeout=10m ./internal/service -run '^(TestDeliveryCompletionConsumedGetReadCoexistence|TestDeliveryCompletionConsumedGetWriterFences|TestDeliveryCompletionConsumedGetCurrentAuthority)$'
```

No runtime pass or compiler result is claimed. Source remains frozen for main's
first execution and later explicit bounded GREEN scope. This correction is a
pre-execution test-boundary repair; it is not a relaxation after a GREEN failure.
