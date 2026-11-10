# Completion evidence hints

## 1. Scope / trigger
Explicit human-only pre-completion hints for ordinary issues. This adds an API
consumer of existing policy and evaluation recovery, not an observer, engine,
transition interceptor, UI/CLI prompt or global completion gate. Independent
collaboration remains under its own exact-submission/initiator/R38 contracts.

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
non-authorizing hint, and keep every completion interceptor explicitly pending.

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
