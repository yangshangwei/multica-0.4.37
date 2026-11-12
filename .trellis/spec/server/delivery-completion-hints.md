# Completion evidence hints

## 1. Scope / trigger
Explicit human-only pre-completion hints for ordinary issues reuse existing
policy and evaluation recovery. The API remains non-authorizing. Shared
Web/Desktop and CLI status-only consumers now request it before an ordinary
done-category update; actual completion enforcement follows the separate
transactional contract. Independent collaboration retains its own
exact-submission/initiator/R38 contracts.

## 2. Signatures
`POST /api/issues/{id}/delivery-evidence/completion-hint` accepts exactly:

```json
{"expected_issue_revision": 4, "target_status": "done", "evaluation_key": null}
```

The first value is a nonnegative int64. Target status uses existing catalog
normalization and strict active-status resolution. Key is explicit null or a
canonical nonzero UUID belonging to the current human evaluator. Limit4096bytes;
reject omitted, duplicate, unknown, wrong-type and trailing JSON fields.
Normalized custom target keys must pass the existing status-key syntax validator
before PostgreSQL text encoding (including embedded NUL); Resolve still owns
workspace existence and archived-state validation. Invalid syntax is400, not503.

## 3. Contracts
Return locked issue identity/revision, normalized target key/category, nullable
project identity, current policy revision/mode, `hint`, `reason`, `checked_at`,
`enforced:false` and `usable_for_completion:false`. Include `evaluation` only
when this request actually recovers it with complete current authority.

- Non-done target, no project or off policy: `not_applicable`; do not look up or
  disclose supplied evaluation keys. Off is not skip or dangerous-action approval.
- Observe policy + done category + null key: `attention/evaluation_required`.
- Key supplied: reuse existing immutable evaluation recovery; original pass and
  current applicability yield `matching/observation_still_matches`; all other
  successful recovery outcomes yield attention with applicability reason.
- Missing/another evaluator's key, lost original authority/binding and DB failures
  remain refusal, never downgraded to a harmless attention response.

Only current workspace owner/admin can enable/disable observe through the
existing CAS policy API. Hints require explicit client requests. All previous
completion behavior remains available; no automatic hint exists in old clients.
Current/pass or matching never authorizes a later write, proves general quality,
or proves live remote Git HEAD. Original hold/unknown cannot upgrade or renew.

Use deliveryPolicyTx including exact schema/index readiness, workspace/issue/
project and current principal/credential fences. Reuse lifecycle issue lock and
project SHARE NOWAIT to serialize the observation against policy PUT. For custom
targets hold the existing shared status catalog lock before the issue row and
Resolve, matching ordinary writers and bounded by
the artifact transaction lock timeout. Do not seed status rows on this read path.
Evaluated checked_at equals evaluation.applicability.checked_at, not a later
unverified clock. Other branches use DB time. Confirm commit and current
credential before success. Cache-Control:no-store. No new durable rows, packaging,
queue, comments, lifecycle writes or automatic retries.

The read transaction ends before any later completion. Future enforcement must
recheck identity, policy, expiry, exact submission/artifact bindings and evidence
inside EACH actual writer's transaction, including batch per-item writes,
create-done/comment-child, move/project change, GitHub/VCS, collaboration
acceptance/self-completion/R38 admin, and custom done categories. Batch currently
commits per issue. Private evaluator identity is not transferable to automation
or other humans. Skip and these additional semantics require separate contracts.

## 4. Validation and error matrix
| Condition | Outcome |
|---|---|
| Malformed body or invalid/archived target |400, no write|
| Machine/forged/revoked principal |authentication refusal or403/404|
| Stale issue revision |409|
| Independent collaboration through ordinary route |409 unsupported_collaboration_action|
| Missing/other evaluator key |404 without evaluation|
| Cross-issue key or missing retained binding |409 without evaluation|
| Lock contention |409, no automatic retry|
| Missing/invalid schema or failed/uncertain commit |non-success|
| Original/current nonpassing evidence |attention; immutable original observation|
| Current matching original pass |matching; both permission flagsfalse|

## 5. Good / base / bad
Good: the original human evaluates an actual controlled archive, asks for a
custom done target hint and sees later policy/PR/expiry changes as attention.
Base: old clients keep their completion APIs. Bad: infer an issue is allowed to
complete from matching, off or an absent hint; infer full gate coverage from one
endpoint.

## 6. Required tests
Use owned fresh handler/router DBs per normal/race. Verify actual tar bytes/SHA,
persisted evaluation immutability and zero additional artifact/task/comment/
status/evaluation writes. Cover current credentials and original principals,
policy/revision/PR/expiry/unknown, custom active/archived status, missing keys,
current schema readiness and bounded lock conflicts, commit failures, and
collaboration refusal. Verify checked_at equality. Retain failed runs and final
source/log hashes. Old artifact/observer/policy compatibility, build/vet/sqlc,
format and skills contracts accompany independent review. No performance,
browser/deployment/full-repo or real-model conclusion follows.

## 7. Wrong versus correct
Wrong: consume historical pass as a completion ticket, read someone else's
private evaluation, duplicate the observer, or call this a global gate.
Correct: recover the exact original human key under current authority, report a
non-authorizing hint, and require the separate writer transaction to authorize actual completion.

## Reused service implementation

Policy reading and evidence validation now share the transaction-local service
core (`service/delivery_policy.go`, `service/delivery_evidence.go` and
`service/artifact_authority.go`). The explicit hint retains its original strict
HTTP body, immutable private recovery, locks and non-authorizing response.
Core extraction does not itself implement any completion writer gate.

## Subsequent completion enforcement

The later [transactional completion contract](./delivery-completion-enforcement.md)
adds explicit authorizations and current checks in actual writers. The historical
observe/hint increment above remains non-authorizing; its private evidence and
original actor/retention boundaries remain. Current source-policy enforcement,
project-change rules and distinct R38 handling follow that shared contract.

## Ordinary UI/CLI consumers (2026-11-12)

### Scope and signatures

Shared issue detail/actions/surface/subissue status-only updates and CLI
`issue status` / status-only `issue update` consume the existing POST endpoint.
They send exactly `expected_issue_revision`, `target_status`, and
`evaluation_key: null`; private evaluation data is not exposed through the
consumer DTO. Batch, move, create-done, mixed-field updates and independent
collaboration/R38 prompts are outside this increment.

### Consumer contract and validation

- Resolve custom done categories from the current catalog/server response.
- `attention` can show a UI confirmation or CLI stderr advisory; it grants no
  completion permission. The actual PUT remains authoritative.
- Missing revision/category, malformed success, unknown response flags or
  transport failure cannot silently proceed. Only unstructured legacy 404/405
  may bypass the hint; structured resource/permission/delivery codes remain
  refusals, including `error_code`, `resource`, `permission_code`, and
  `delivery_code` fields.
- A retry reads the current issue/revision and releases the stale dialog.
  Unmounted owners cannot confirm an old hint; duplicate confirmation is
  single-flight through actual mutation settlement.
- Detail `onDone` and run-confirm continuations execute only after the real
  mutation succeeds. Cancellation, hint refusal, unknown result or failed PUT
  does not navigate or report completion.

### Examples and regression assertions

Good: a stale-revision retry reloads the issue, confirms the new hint, then
waits for one successful PUT. Base: an unstructured old-server 404 preserves
the old update flow. Bad: a structured 404 is treated as route absence, an
unmounted dialog mutates an issue, or `matching` is consumed as authorization.

Core tests assert strict null-key requests and structured-error classification.
Shared gate tests assert latest revision on retry, zero mutation after unmount,
one mutation for duplicate confirmation and release only after settlement.
CLI tests assert stderr-only advisory, rejection before PUT and success output
only after PUT succeeds. Existing backend hint and enforcement matrices remain
independent; consumer checks do not replace them.
