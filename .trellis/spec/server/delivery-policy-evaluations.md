# Project delivery policy and durable evaluations

## 1. Scope / trigger

This observe-only increment persists fixed-policy project configuration and
immutable evaluations alongside the existing stateless observer. It does not
intercept lifecycle writes, enable skip, or grant completion/release authority.
The existing `/delivery-evidence/evaluate` retains its strict body and no-ledger
behavior. Ordinary, batch, create-as-done, GitHub/VCS and independent
collaboration completion still require a separate complete enforcement package.

## 2. Signatures

- Human-only GET/PUT `/api/projects/{id}/delivery-policy`.
- Human-only POST `/api/issues/{id}/delivery-evidence/evaluations`, mandatory
  canonical nonzero UUID `Idempotency-Key`.
- Human-only GET `/api/issues/{id}/delivery-evidence/evaluations/{key}`.
- `project_delivery_policy` retains append-only project policy revisions.
- `delivery_evidence_evaluation` retains the original request and observation.

## 3. Contracts

Policies default to virtual revision0/off. PUT accepts exactly
`expected_policy_revision`, `policy_version`, `mode`, `stage`, `reason`.
Only current workspace owner/admin may append; project leadership alone does
not grant policy authority. Mode is off/observe, policy version is
`delivery-artifact-observe-v1`, stage is done. Stage identifies a future
completion observation, not an installed completion interceptor. An accepted
same-value PUT is a new revision; stale CAS conflicts. Lost PUT responses are
resolved by reading the latest policy. Policy changes do not mutate the general
project revision. Off blocks new persistent evaluation admission and retains
history; it does not disable other security boundaries or the stateless observer.

Persistent POST adds `expected_policy_revision` to the stateless observer's
exact body. It requires an existing project with current observe policy for
new admission. Namespace is workspace + verified human evaluator + request key;
resolved issue and all body bindings are in the canonical request digest.
Same key/same request recovers; changed request or issue conflicts. A renewed
valid credential for the same human can recover. Other humans and machine
credentials cannot use the original key to read private evaluations.

Policy reason is trimmed and limited to1–1000Unicode code points; the strict
policy body is bounded to8192bytes, evaluation body to4096bytes.

The response nests the immutable original `observation`, alongside evaluation
and key identity, evaluator, policy revision and stage. Separate `applicability`
reports status/reason/checked_at from current revalidation;
`usable_for_completion` is always false. Never replace the original observation
or extend its expiry. Original unknown is a completed stored observation, not a
claim available for takeover. Original hold/unknown cannot become passing by
replay; a new explicit evaluation requires a new key and fresh admission.

Replays branch before *new-admission* policy/version gates so policy off/change
and issue moves can invalidate rather than destroy authorized recovery. Check
current issue/workspace/caller and key ownership first. Recheck the retained
artifact with a temporary request carrying CURRENT locked issue/project
versions, preserving original approval/SHA/digests. Only then compare original
versions and policy and mark applicability. Catching an early version409 and
returning old evidence skips later producer/approver authority and is forbidden.
The same current authorization applies to saved hold/unknown. If a previously
bound execution is missing, refuse historical content; the observer's early
`evidence_missing` return is not proof of authorization to the old material.

An originally passed observation is current only when the live observer still
passes and its execution/producer/task/generation/receipt/spec/artifact identities
match, original resource/policy revisions match, and the original window is
unexpired. Current or historical pass is not authorization for later actions.
The observed PR head is only a database observation, never remote live Git HEAD.

Reuse artifact current-principal fences, bounded no-retry transactions, issue
lifecycle and project SHARE/UPDATE NOWAIT locks. Policy writes serialize through
the project row, including absent first-policy rows. Evaluation and observation
commit atomically with no packager, queue, lifecycle or comment side effect.
Do not wrap these paths in the retrying project helper. Verify new schema and
exact usable unique-index shapes separately from artifact schema readiness.
Commit ambiguity returns non-success; the same key recovers a row only if it
actually committed. No automatic evaluation or execution retry is introduced.

Policies/evaluations survive resource deletion. Missing workspace/issue denies
public recovery; deleted project denies policy reads. A surviving detached issue
can recover stale history after complete current artifact authorization. Do not
register retained history for ordinary teardown or create database foreign keys.

## 4. Validation and error matrix

| Condition | Result |
| --- | --- |
| Invalid/extra/duplicate fields, key, unsupported mode/stage/version | 400, no write |
| Machine/forged/revoked current authority | 403/404 or authentication refusal |
| Stale policy CAS/new-admission version or changed same-key request | 409 |
| Lock contention | 409, no partial success |
| Missing/invalid new schema or failed/uncertain commit | Non-success, no claimed persistence |
| Same authorized key after policy/resource change | Original observation, stale applicability |
| Original window elapsed | Expired applicability, original facts retained |
| Original or current unknown | Nonpassing applicability (stale/expired takes precedence), no promotion/write |
| Missing previously retained execution | Refuse historical content |
| Same original bindings and currently valid pass | Current observation only; completion permission false |

## 5. Good / base / bad cases

Good: retain a human evaluator's original result, recover it after restart with
a valid credential, and see a later policy revision make it stale.
Base: old clients keep existing writes and stateless observation.
Bad: read stored pass as a reusable ticket, re-evaluate unknown under the same
key, use off as a skip approval, or bypass current authority after version drift.

## 6. Required tests

Use task-owned fresh databases per package and normal/race run. Real router
authentication must produce a controlled tar, verify actual bytes/digest and
persist matching observation identity. Assert one row for concurrent duplicates,
immutability across restart/replay, policy CAS/ABA/off, actor isolation, role and
original-principal revocation after version drift, PR changes/ambiguity, expiry,
missing bindings, transaction failures and resource deletion retention.

Migrations776–780 use separate concurrent index files and exact readiness validation;
actual private-schema tests cover empty maintenance rollback, independent policy
and evaluation retention, concurrent writers and invalid index shapes. Stop
admission throughout a multi-file downgrade. Never delete real evidence to
permit downgrade. Run old observer/artifact compatibility, backend build/vet,
formatting, sqlc idempotence, static contracts and independent final source/log
hash verification. Tests do not close historical real-model acceptance gates.

## 7. Wrong versus correct

Wrong: gate one done writer and call this a global gate, or return an old pass
after the evaluator exits early on resource-version mismatch.
Correct: keep this package observe-only, explicitly defer all done writers, and
finish current authority validation before projecting historical applicability.

## Later explicit hint consumer

The ordinary-issue human completion hint API consumes this existing recovery
contract without changing it. See [Completion evidence hints](./delivery-completion-hints.md).
It does not transfer evaluator identity or intercept any completion writer.
Policy off suppresses this explicit hint's evidence consumption; original-key
historical recovery through the existing evaluation API remains authorized and
returns its existing stale applicability. No shared audit or skip was added.

## Shared policy and observation core

`service/delivery_policy.go` owns the transaction-local current-policy read and
exact existing schema check. Complete validators accept only the borrowed
`pgx.Tx` and create their own transaction queries. HTTP policy/immutable private
evaluation handling remains in handler and delegates observation to the single
service implementation. This extraction adds no mode, schema or permission.
