# P0 implementation: one service observer and authority core

Date: 2026-11-11. Implementer: native `trellis-implement` / `implement_observer_core`.
Scope: behavior-preserving extraction only; no completion grant, mode, schema, writer,
route, dependency, packaging, queue or public response change. This is not full
completion enforcement or M1 acceptance.

## Change boundary and baseline

The original observer and artifact authorization were handler methods, so service
writers could not reuse them without depending on HTTP or creating a second
observer. They now live in service and borrow the caller's transaction.

The main session completed the new task-owned pre-edit baseline before authorizing
product edits: handler 325 and real-router 103 named passes, zero fail/skip in each.
Evidence remains in `runs/p0-baseline-handler-normal01.json` and
`runs/p0-baseline-router-normal01.json`; these runs were executed by the main
session, not this implementer. Existing behavior was protected before movement.

## Files and simplification

- `service/artifact_authority.go` owns one copy of current credential, workspace,
  users/membership, original artifact authority and retained specification checks.
- `service/delivery_evidence.go` owns the sole observation algorithm, canonical
  receipt validation and the unchanged request/response value types.
- `service/delivery_policy.go` owns the unchanged current policy read and exact
  policy/evaluation schema readiness check. Modes remain off/observe.
- `handler/agent_artifact.go` retains transport decoding, deny-only actor headers,
  transaction ownership, packager/claim/finalization and thin core adapters.
  Existing private `artifactFailure` translation preserves consumers such as
  durable delegation without editing their modules.
- `handler/delivery_evidence.go` retains strict JSON parsing, aliases the shared
  value types and delegates the observer. Header hints become deny-only data in
  the local credential value; no authentication context is rewritten.
- `handler/delivery_policy.go` retains strict policy decoding, authority for policy
  management, CAS/writes and public transport behavior, delegating read/readiness.
- `handler/delivery_evaluation.go` and `delivery_completion_hint.go` only change
  their current-policy read calls to pass the existing transaction. Their private
  identity namespace, immutable history, applicability and permission-false
  behavior remain in place.

No second observer or parallel authority algorithm remains in handler. Small
handler wrappers exist only to preserve established consumers and transport
error shape. No product writer, migration, generated SQL, constructor or router
was changed.

## Frozen callable boundary

All complete borrowed validators and the policy read accept only `pgx.Tx` and
construct `db.New(tx)` internally. They cannot accidentally pair a pool query
object with a different transaction:

- `PrepareArtifactTransaction(ctx, tx)`
- `CheckArtifactCredentialInTx(ctx, tx, credential)`
- `CheckArtifactAuthorityInTx(ctx, tx, access, req, prior, approver, full, manage)`
- `EvaluateDeliveryEvidenceInTx(ctx, tx, access, initial, req)`
- `CheckDeliveryPolicySchema(ctx, tx)`
- `CurrentDeliveryPolicyInTx(ctx, tx, workspace, project)`

`ArtifactAccess` carries explicit verified credential values, selected workspace
and optional metrics. `ArtifactCredentialSnapshot` carries the actual principal,
password-session metadata and verified task token as non-public JSON fields;
`Actor()` derives typed actor identity. It is not a request decoder, permission
or shared token. Service code does not read another actor from context and does
not use WithAuthenticatedPrincipal/WithPasswordSession or synthetic HTTP headers.

`LockArtifactWorkspace` and `LockArtifactUsers` are small query-level lock/read
helpers, not complete admission gates. `ReadArtifactSpec` is pure retained-row
validation. Service callers still own the required workspace/catalog/issue/project
lock ordering and must run schema preparation as appropriate before admission.
No core function begins, commits, rolls back or retries a transaction.

The original NOWAIT fences, 250ms timeout, database clock, credential revocation,
managed binding, original producer/approver/claimant authority, RuntimeLookup
attribution, PR table locks, fixed observer ordering and canonical receipt checks
were moved with the implementation. UUID strings entering service queries use
checked `util.ParseUUID`; malformed internal values refuse instead of panicking.
HTTP malformed inputs continue to be rejected by their original strict boundaries.

## Validation performed

- `p0-core-build01`: full backend build passed after the initial extraction.
- `p0-core-vet01`: full backend vet passed after that extraction.
- The main session then completed the first post-extraction handler 325/router
  103 normal runs, zero fail/skip, before the public API was tightened to tx-only.
  Those source identities and original results remain separate evidence.
- `p0-core-build02`: full backend build passed after the tx-only signature change.
- `p0-core-vet02`: full backend vet passed after the tx-only change, with no
  DB or real-agent execution.
- `gofmt` and `git diff --check` passed for the edited files.
- Source inspection confirms exactly one observer and receipt validator, no
  service HTTP/request dependency and no transaction ownership or identity
  context injection in the new core.

The main session owns final fresh normal/race handler/router runs, the focused
durable-delegation compatibility run, source freezing and independent review.
Recommended focused legacy-wrapper filter:
`^TestDurableDelegation(ActualQueueMergeDeferredAndRestart|ReporterLocksTargetBeforeScopeRead|UnknownDoesNotRedispatch)$`.
It covers actual human queue/recovery, restricted task-token scope and translated
403 denial after target movement, plus retained uncertain outcomes. This report
does not relabel intermediate compatibility checks as gate acceptance.
No new product tests were added to mirror a moved implementation; the actual
existing endpoint/credential/receipt regressions provide the behavior contract.

## Retained failures and limitations

This lane had no build/vet failure at the time of writing. One early read-only
query guessed `server/internal/util/uuid.go`; the file does not exist, and the
actual helpers were found in `util/pgx.go`. A subsequent filename-filter lookup
returned no match. These are source lookup mistakes, not product red evidence;
no database command or state mutation occurred in those attempts.

P1 must still define server-retained issuer metadata explicitly (do not serialize
this internal snapshot into a public response), grant/consumer binding, current
issuer and consumer checks, and atomic consumption. P2–P4 must still cover all
actual writers. No enforce or usable skip exists from this extraction. Existing
source-map/spec references pointing into moved handler implementation need the
main session's final documentation update. No commit, push, deployment, external
message, historical database/container action or provider call was performed by
this lane.

## Source bindings after tx-only extraction

| File | SHA-256 |
| --- | --- |
| `server/internal/service/artifact_authority.go` | `c6cf948fe2a6eb87ab349cdebe237cfe12a2288ec55968476c099225f2bf20d1` |
| `server/internal/service/delivery_evidence.go` | `34add212a607e2b10ed3bb4c2999fdd86ab5d0fb1bbd7e0f97a81515a9e7b4ed` |
| `server/internal/service/delivery_policy.go` | `9107e5180612888ff91253c4c23ceee9d3c79e36650499588dc88201ea82f801` |
| `server/internal/handler/agent_artifact.go` | `8a153b06bb7e462da2639cee19cb4af5862973f7fb7eb76f422d81089b874d5e` |
| `server/internal/handler/delivery_evidence.go` | `ba97ee4b057f5bdfe86902e90b1b2daf6f57469ee7b6d236c555578db093e4b3` |
| `server/internal/handler/delivery_policy.go` | `7d2b62f7022b605bd80b2d2c783ccb05871409ef0d89f780ab756c2120357fbd` |
| `server/internal/handler/delivery_evaluation.go` | `4740444b675b5f17a39fe3cdedbd831f6caf07db6cf3ddcfdb2124bd885b8e62` |
| `server/internal/handler/delivery_completion_hint.go` | `6f7ce829c64c8d21b1271759541974c0324eb3625550f4b4c562e0f8038ab44a` |
