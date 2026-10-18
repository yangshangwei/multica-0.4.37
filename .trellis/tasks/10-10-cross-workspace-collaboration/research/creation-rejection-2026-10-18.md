# Durable creation rejection

## Change boundary

A recipient or creation gate can reject a valid creator's request before the
current service reserves an operation. Replaying that old request after the
condition improves can create an issue, so a client cannot safely edit its
pending intent. The authority belongs in `CollaborationService.CreateInTx` and
its transaction owner, using the original actor/source/request and payload hash.

The implementation changes `service/collaboration.go`, adds a creation-result
helper and HTTP reader, registers that reader in `router.go`, and adds adjacent
service/HTTP regressions. Existing operation queries suffice; there is no schema
migration or generated-code edit. The parent also authorized the five-line
`IssueService.CreateInTx` correction that distinguishes a genuinely absent
parent from a failed database lookup. Creation success and attachment
consumption remain under the existing transaction. Frontend changes
belong to `client_check`; execution, provider, upload and other operation families
are outside this slice. There is no broad refactor.

## Client contract

`GET /api/collaboration/creations/{requestId}/result?source_workspace_id=UUID`
returns HTTP 200 only for a validated completed own operation:

```json
{
  "request_id": "original key",
  "source_workspace_id": "source UUID",
  "actor_user_id": "authenticated creator UUID",
  "outcome": "rejected",
  "code": "account_unavailable"
}
```

The allowed pairs are `created/created` and `rejected/<code>` with this closed
rejection-code set. POST/replay returns the original status below; the result
reader returns HTTP 200 for a validated settled result, including a rejection.

| Code | Original status | Settled condition |
| --- | --- | --- |
| `account_unavailable` | 403 | Chosen recipient is absent, disabled, setup-only or restricted |
| `collaboration_unavailable` | 503 | The locked creation gate is closed |
| `invalid_attachment_selection` | 400 | Invalid selection reaching the authorized service |
| `attachment_not_found` | 404 | Missing or foreign attachment selection |
| `attachment_scope_mismatch` | 409 | Selection does not belong to this usable draft/source |
| `attachment_draft_consumed` | 409 | Another operation already consumed the upload bundle |
| `parent_issue_not_found` | 400 | Parent is absent from the source workspace |
| `project_not_found` | 400 | Project is absent from the source workspace |
| `triage_review_required` | 409 | Selected parent is not admitted formal work |
| `issue_limit_reached` | 402 | The locked source capacity check refuses creation |

The reader returns no issue content,
recipient, attachment metadata, payload hash or credentials. Missing operations
return 404 `operation_not_found`; unfinished ones return 409
`operation_incomplete`. Invalid stored results and dependency or commit errors
remain unavailable. The actor must still be an authenticated, active human;
minimal own receipts do not grant source or issue access.

The original POST and creation GET keep the existing successful issue shape.
The original GET returns 404 `not_found` for a rejected creation; the new reader
is the only creation-rejection proof. A missing new-reader result is 404
`operation_not_found` and must not be confused with rejection.
The client must not retire an intent from a POST error, a missing operation, or
an unknown commit. It may retire only a correlated, settled rejection, preserve
input and still-unconsumed uploads, recheck source authority, and require an
explicit resubmission before submitting a new key. The input may remain unchanged
when the refusal condition has improved; reading the result never resubmits.
Old keys stay sealed, even
after the corrected creation consumes the same draft and the old recipient is
enabled again.

## Transaction and authority contract

- Parse a trusted human actor; take sorted actor/recipient user locks and
  recheck the actor account/password version. Inspect the original operation
  under the actor lock before revalidating recipient, gate or source.
- For a new operation, keep the existing gate `FOR SHARE`, source workspace
  `FOR KEY SHARE`, and source membership fences. Only after valid actor/source
  authority reserve the original source/actor/request key and unchanged payload
  digest. Authentication and source authorization refusals create no operation,
  account fence or business rows.
- Recipient and closed-gate refusals complete the operation directly. For
  admitted creation, a savepoint follows reservation; issue/counter, account
  fences, participants, session, attachment delivery and event/notification
  writes are inside it. A specifically recognized business selection refusal
  rolls back that work before completing the original operation. The outer
  actor/source/gate locks survive that rollback until commit.
- `CreateInTx` still borrows the caller's transaction. Its private staged-error
  marker lets `Create` commit a known refusal before returning its fixed domain
  error. A caller can instead roll back, which removes the staged refusal too.
  No borrowed helper commits or publishes by itself.
- Any dependency, savepoint, required receipt write or unknown commit error
  rolls back or remains unknown. In particular, only `pgx.ErrNoRows` becomes
  parent-not-found; network/query faults do not. A real commit followed by a
  lost acknowledgement returns unavailable until the original result is read.
- Result reads lock and revalidate the active caller before looking up their
  operation. A self-only content-free result can survive source departure;
  successful issue reads still require current object access. This result
  never grants source, issue or attachment authority. Corrupted or unsupported
  stored result pairs/correlation return unavailable, without private values.

## Verification plan

Start with a failing disabled-recipient and closed-gate replay regression. Then
cover concurrent old/corrected requests, transactional refusal persistence,
borrowed rollback, uncertain commits (including lost acknowledgements after a
real commit), exact own-operation correlation, actor/source failures without
writes, successful recovery, and whole-draft one-time consumption after a
corrected intent. HTTP tests verify route registration, fixed statuses,
no-store and private-field redaction. Use only the assigned integration database
and guarded Go test runner, with `-race -p 2 -parallel 2`.

## Verification evidence

Evidence directory:
`/var/folders/3n/gbt3p39s5pdc4l55js62gxnm0000gn/T/multica-cwc-product-20261018-nebz79rl`.
Only `cwc_product_integration_nebz79rl` on the existing task PostgreSQL listener
was used. All Go tests ran through
`bash ../scripts/go-test-with-agent-cli-guard.sh go test -race -p 2 -parallel 2`.

- `creation-rejection-red.log`: recipient and gate restoration both reproduced
  the bug; original-key replay unexpectedly succeeded before the change.
- `creation-rejection-service.log`: creation/result, recipient eligibility,
  attachment creation/draft regressions and ordinary parent `CreateInTx` tests
  passed (19.777 s). Includes concurrent old/corrected requests, unchanged
  payload identity, account/password/source boundaries, whole-draft reuse,
  savepoint/required-write failures, both unknown commit outcomes, corrupt
  result correlation, and real password-recovery lock races. No skipped tests.
- `creation-rejection-handler.log`: HTTP result privacy, original success and
  refusal statuses, query/actor validation, missing/corrupt receipt behavior,
  and lost-acknowledgement versus rolled-back commit cases passed (2.522 s).
- `creation-rejection-routes.log`: production router reached the new result
  endpoint and retained original successful POST/GET recovery under a closed
  gate (2.490 s).

- `creation-rejection-fences.log`: deterministic database waiters confirmed
  recipient mutation, source removal, gate mutation and result lookup wait for
  the refusal commit after savepoint rollback. Actor/source zero-write checks
  and malformed-actor pre-dependency rejection also passed (3.646 s).
- `creation-rejection-vet.log`: guarded `go vet -p 2 ./internal/service
  ./internal/handler ./cmd/server` passed with no diagnostics.
- `creation-rejection-static.log`: scoped `git diff --check` and `gofmt -l`
  passed with no diagnostics. The evidence JSON records commands and the code
  file digests at handoff.

The integration database was released to the parent after these checks. No
backend verification remains pending for this slice.

## Limits and follow-up

Frontend schema/state/form integration and the real Web correction journey
belong to `client_check`; they are not substituted by these service/HTTP tests.
Malformed transport input rejected before the authorized service has no durable
operation. Unknown results stay recoverable with their original key. No data
migration, dependency, feature flag, provider execution, service restart, local
commit or complete product/AC acceptance was performed by this slice.
