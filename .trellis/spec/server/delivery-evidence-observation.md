# Delivery evidence observation

## 1. Scope and trigger

`delivery-artifact-observe-v1` evaluates one existing controlled local-synthetic
tar receipt against a locked issue/project and database-observed PR candidate.
It is the observe stage of the M1 delivery gate. It does not enforce `done`,
write an evaluation ledger, grant a reusable acceptance token, verify remote Git
HEAD, or validate broader quality checks. Existing clients retain their update,
completion and approval behavior.

## 2. Signatures

`POST /api/issues/{id}/delivery-evidence/evaluate` accepts verified human JWT/PAT
readers with current issue access and controlled-artifact reader authority.
Machine credentials and supplied actor headers cannot acquire this authority.
Issue path aliases resolve through the existing loader before transactional use.
No new schema, dependency, UI or CLI flow is introduced.

## 3. Contracts

The JSON body is bounded to 4096 bytes and contains exactly these fields:

- `policy_version`: `delivery-artifact-observe-v1`.
- `expected_issue_revision`: nonnegative integer.
- `expected_project_id` and `expected_project_revision`: both explicit null for
  no project, otherwise canonical UUID and nonnegative integer.
- `approval_id`: canonical UUID.
- `expected_source_revision`: 40 or 64 lowercase hexadecimal characters.
- `expected_spec_digest`, `expected_artifact_digest`: 64 lowercase hex characters.

Reject extra, duplicate, case-alias, omitted, wrong-type and trailing JSON fields.
The response carries `mode: observe`, policy, `verdict` (`pass`, `hold`, `unknown`),
reason, issue/project versions, approval/execution identity, source revision,
observed head, spec/artifact/receipt digests, producer agent/task/generation,
`observed_at` and `valid_until`. No paths, filenames or credentials are returned.
The handler emits `Cache-Control: no-store`.

Current caller, original human initiator, originator, runtime owner, approver
and human claimant remain subject to current authority checks. Historical tasks
can be completed, but task generation, agent, runtime and resource binding must
still match. Cancelled/failed producers do not pass. Archived/restricted or
non-operator producers and lost invocation/management rights cannot pass.

Use existing workspace/lifecycle/project and artifact authority fences. The
four GitHub/VCS PR and link tables take a short `SHARE NOWAIT` lock because
existing writers have no shared issue-row lock protocol: row locks alone miss
new links. This is intentionally a global contention point for the observation
transaction; return conflict instead of waiting. The existing PAT middleware
updates last-used asynchronously; that ancillary write can also yield a transient
409 against the current-credential lock. This refusal is not authorization failure
or a pass. Test positive controls synchronize the actual committed last-used
write at the authenticated request-body boundary without weakening live locks.
Do not claim high-throughput
performance from these targeted tests. Approval and receipt are read under locks;
no observation is returned after a failed commit.

PR selection uses current stored facts, excludes reference-only links, prefers
open/draft then latest update time, and returns unknown if equally preferred
heads disagree. The provenance string explicitly states
`database_observed_pr_head_not_remote_verified`. Missing head is unknown even
when a caller declares a SHA. A current DB head proves only what this server has
observed, not an unreceived remote push. Publicly editable issue metadata is
never trusted as policy or evidence.

Validate frozen canonical spec and request binding. Restore the original
receipt map's canonical JSON with `UseNumber` and compare the stored SHA-256;
never hash PostgreSQL JSONB text. Compare receipt identity, actor, resource,
policy, declared revision and output fields to the retained row. Receipt time
can precede the final row timestamp because production takes two database clock
readings. Its valid range starts at claim and ends at finalization.

This version conservatively uses the original artifact admission `expires_at`
as its observation horizon. An expired observation holds; the historical
successful artifact remains retained. This is not a general artifact lifetime
policy. Even a pass is a snapshot at `observed_at`, never an authorization to
complete or publish later.

## 4. Validation and error matrix

| Condition | Outcome |
| --- | --- |
| Malformed body or unsupported policy | 400 |
| Machine actor, revoked credentials or authority | Authentication refusal or 403/404 |
| Issue/project expected version mismatch | 409 |
| Lock contention | 409; no retry or partial pass |
| Database/commit failure | Non-success, no usable observation |
| Missing execution/unclaimed evidence or unknown execution | unknown |
| Succeeded row with missing/corrupt receipt | hold (`receipt_invalid`) |
| Failed execution/producer or expired observation | hold |
| Spec/artifact/source mismatch or invalid receipt | hold or binding conflict; never pass |
| No observed PR head / conflicting equally ranked heads | unknown |
| Matching successful receipt, current authority, versions and observed head | pass for this observation only |

## 5. Good, base and bad cases

Good: actual controlled tar execution yields a valid receipt, the same candidate
is the observed PR head, and a current human receives an exact versioned
observation. Base: old clients do not call this endpoint and keep existing
completion behavior. Bad: self-report a green verdict in metadata, use a
matching declared SHA without observed source facts, or reuse an observation as
permission for a later lifecycle transition.

## 6. Required tests

Handler tests must verify genuine packaging/receipt integration, binding and
expiry negatives, original distinct principal revocation, completed-task reads,
PR ambiguity and concurrent issue/project/PR/authority/receipt changes. Router
tests must use actual authentication, generated task credentials for packaging,
actual tar bytes, legacy update compatibility and post-auth credential changes.
Use separate task databases, canonical private TMPDIR (the packager refuses
symlink ancestors), preserve failed attempts and freeze final source hashes.
Count named results including subtests separately per ordinary/race run.

## 7. Wrong versus correct

Wrong: infer a trusted repository HEAD from mutable metadata, hash JSONB text,
or block only one done writer and call that an enforced project gate.
Correct: recompute observation from retained receipts and current locked
facts, report unknown when source facts are missing, and cover every terminal
writer in a separate reviewed enforcement package.

## Later persistent observation increment

The stateless endpoint above still does not write an evaluation ledger.
[Project delivery policy and durable evaluations](./delivery-policy-evaluations.md)
adds separate human-only off/observe policy and immutable evaluation endpoints
using this observer. That increment also leaves all completion writers ungated.

## Shared transaction-local implementation

`service/delivery_evidence.go` owns the single observation and canonical receipt
algorithm. The handler retains strict request parsing, authenticated header
checks and response/transaction ownership. `EvaluateDeliveryEvidenceInTx` takes
the caller's `pgx.Tx` and explicit `ArtifactAccess`; it creates queries from that
transaction and never begins, commits, rolls back or retries it.

`service/artifact_authority.go` accepts explicit verified credential snapshots
for current authority checks. Credential metadata is excluded from public JSON.
Do not switch authentication contexts to impersonate another human or pair a
borrowed transaction with queries from a pool. This extraction alone installs
no completion interceptor and does not make historical observations permits.

## Subsequent transactional consumer

The later [transactional completion contract](./delivery-completion-enforcement.md)
adds explicit authorizations and current checks in actual writers. The historical
observe/hint increment above remains non-authorizing; its private evidence and
original actor/retention boundaries remain. Current source-policy enforcement,
project-change rules and distinct R38 handling follow that shared contract.
