# Independent design review: controlled artifact execution

Reviewer: `trellis-check` / `controlled_plan_review`
Baseline: `01becb2b0` on the isolated M1 worktree
Scope: design and existing source only; no product edits, database, adapter, model, or remote execution.

## Verdict

**Revise the design contract before implementation.** The narrowed local-only pack adapter is a good first seam, but the current artifacts still describe a mutable permit created from a generic approval and leave several state, provenance, and filesystem details implicit. The following constraints are required to make the result independently verifiable. They do not require a deployment platform or a new human approval flow.

## Blocking findings

### 1. Freeze the typed request before approval

`agent_approval_request.plan` is currently free text and the existing create endpoint accepts any plan an actor supplies. A later permit cannot safely add `commit`, artifact manifest, environment, or adapter arguments to an already approved row: the human may have approved a different action.

Add the new package endpoint as one transaction that:

1. validates a canonical typed package specification;
2. stores the exact canonical specification and its digest;
3. creates the pending approval whose generated summary/plan displays that same specification;
4. records the initiating human, task, agent, workspace, and issue/resource provenance.

The human decision endpoint may change only decision status/note. It must not accept replacement specification fields. Existing generic approvals remain legacy and cannot be converted into artifact executions. If a client retries package creation, require an explicit idempotency key and return the original approval/specification when the key and canonical request digest match; a different digest is a conflict.

The typed specification should contain a fixed action (`package_release_candidate`), resource kind/id, source revision, input manifest digest plus sorted entries (`relative path`, byte count, SHA-256), target environment (`local-synthetic`), adapter policy version, and a server-derived normalized adapter invocation digest. Reject unknown fields, duplicate paths, traversal, absolute paths, invalid digest lengths, empty IDs, unsupported environment/action/policy, and size/file-count limits. The approval display must be bounded and generated from the canonical bytes, rather than trusting caller prose. Existing approvals have no expiry column; make the new permit expiry explicit, bound it by a documented local policy, compare using database time at claim, and distinguish expiry of admission from expiry of an already claimed result.

### 2. Separate approval decision, execution state, and legacy compatibility

Do not call the existing `MarkAgentApprovalRequestExecuted` before the adapter runs. That row's `executed` status currently means a caller has reported execution; using it for a pre-side-effect reservation would falsely report success when the process crashes. It also changes compatibility semantics for old clients.

The one-to-one execution/receipt table can hold the frozen specification in an unclaimed state from package creation; no second specification table is needed. Give `approval_id` its own unique index. In one claim transaction, lock the approval row and this row, revalidate current human/agent/task/resource authority and the explicit permit expiry, and compare-and-set the unclaimed state to `unknown` plus claim identity before committing. Only after that commit may the adapter run. A crash or timeout therefore leaves `unknown` forever until an explicitly authorized reconciliation path exists; there is no automatic retry.

The existing cancel route must lock the same approval/execution records and refuse cancellation once an execution row has been claimed. Otherwise cancel can race a claimed execution and contradict the receipt. The legacy self-report `/execution` route must reject artifact-bound approvals so it cannot bypass the adapter or fabricate a result. The new execution route may mark the legacy row `executed` only in the transaction that durably records a verified `succeeded` receipt; failed/unknown results must not acquire that completion marker. Legacy unbound approvals retain their current behavior. A second claim with the same binding returns the retained terminal/unknown receipt and does not run the adapter; a changed expected digest conflicts. Because PostgreSQL commit and local filesystem publication cannot be atomic, an ambiguous claim commit must not run the adapter, and an ambiguous receipt commit must not report a definite success/failure. A read-only lookup can recover the retained original state, never grant a second execution.

Receipt updates use compare-and-set from `unknown` and include `execution_id`, approval/spec digest, actor, resource/action, commit, input/output digest, target environment, adapter policy, result code, timestamps and bounded machine-generated metadata. Caller-provided notes cannot replace these facts. `succeeded`, `failed`, and `unknown` are a closed set; unknown or an unrecognized stored status never grants another execution.

### 3. Revalidate authority and provenance at every boundary

The existing approval handler permits an agent to file a request and lets a human owner/admin decide it. The new package path must preserve those semantics while freezing the effective actor chain:

- the initiating principal comes from the verified human session or task token, never `X-*` headers or body IDs;
- the agent, task, workspace, issue/resource and originator are resolved from trusted rows in the creation transaction;
- approval is human-only and uses the existing `canManageAgent`/workspace authority;
- claim/execute rechecks current membership, disabled/revoked account, agent identity, task generation/status, originator/invocation authority and workspace/resource scope;
- an agent can claim only its own frozen task execution; a human can claim only when the existing management authority still holds;
- changing the display name, prompt, autonomy, task assignment, runtime owner, or approval note cannot widen the permit.

Do not treat `source_commit` as proven merely because it is a 40-character string. For this local adapter either bind a coordinator/operator-supplied staging manifest whose source identity was verified before approval, or explicitly label the field as a declared revision and keep that limitation in the receipt. The packager must verify the exact manifest bytes/digest it was approved to package.

### 4. Define the filesystem trust boundary precisely

`os.Root` is the right primitive for confinement, but it is not by itself the full contract. The adapter must:

- accept only operator-configured staging/output roots; reject user-selected absolute paths and disable the adapter when no approved root is configured;
- verify root ownership/identity before and after the operation and ensure input/output roots are distinct;
- normalize slash-separated relative paths and reject `..`, empty components, absolute/volume paths, NUL, duplicate names and unsupported special files;
- inspect every parent component and final entry with `Lstat`; reject symlinks, mount-like replacement, non-regular files and (unless explicitly allowed) hard-link surprises;
- open/read through the confined root with no-follow semantics and hash the bytes while producing the archive. A preflight hash followed by an unchecked reread is a TOCTOU gap;
- use deterministic `archive/tar` headers: sorted paths, fixed mode policy, zeroed uid/gid/user/group, fixed timestamp, no ambient PAX metadata, bounded file/total sizes;
- create the output with exclusive creation under a fixed execution-id path, reject collisions/symlink targets, sync file and parent directory, reopen and hash the final bytes, then persist the output digest;
- never invoke a shell, subprocess, network client, environment credential, or arbitrary caller-provided argv.

The adapter should receive a typed specification and a coordinator-owned root handle/configuration, not a raw path or command string. A `local-synthetic` policy must be checked in both the handler and service/adapter boundary.

### 5. Make migration and index rules executable

The repository requires every new index to use `CREATE [UNIQUE] INDEX CONCURRENTLY` in its own single-statement migration. Do not add a table primary key or inline unique constraint that silently creates a non-concurrent index if this rule applies to the new table. Use a UUID execution ID plus separate concurrent unique indexes for execution ID, approval ID, and any creation idempotency key; keep each index migration separate and update sqlc afterward.

The down path must acquire the execution table's `ACCESS EXCLUSIVE NOWAIT` lock, verify that no permit/receipt rows exist, and drop the table only inside the same protected block. Index down migrations must be separate concurrent drops as required by the migration runner. Test populated, empty-maintenance, concurrent claim, and rollback-with-open-receipt cases in a private schema. A busy writer is a refusal, not permission to wait or delete security evidence.

### 6. Define read authority and unknown-result recovery

Receipts and specifications must be readable only by the initiating human/agent where explicitly needed, the current authorized agent manager, or an auditor role already present in the workspace contract. Do not expose artifact paths, manifests or output details through broad workspace queues. Agent approval/claim endpoints must reject another agent's IDs and all workspace/header substitution.

There is intentionally no automatic rerun after an unknown result. If a later reconciliation route is added, it must be a separate human/auditor decision that compares the retained output digest and execution identity; it must never invoke the side effect again by default. Until that route is designed, unknown is terminal for execution and visible as unknown to callers.

## Accepted design choices

- A single fixed in-process stdlib tar adapter is a suitable first implementation. It keeps the child deterministic and avoids pretending that a host shell is sandboxed.
- A new additive execution/receipt table is preferable to overloading the existing approval status or execution-note field.
- Existing approval and machine-credential gates can be reused, but they must be rechecked at package creation and claim; a generic approval row alone is insufficient.
- Local-only verification with synthetic files, isolated roots, private databases, and no remote push is enough for this child. It cannot claim production deployment safety.

## Required negative/positive matrix before acceptance

1. Human creates canonical package spec; approval display matches the exact canonical digest.
2. Agent request cannot substitute another agent/resource/commit/manifest; human control still works.
3. Approval decision race yields one winner; cancel/claim race cannot create contradictory states.
4. Claim race yields one execution ID and one adapter invocation; replay returns the retained receipt.
5. Every binding mutation (actor/resource/action/commit/input digest/environment/adapter policy/derived argv digest) conflicts before adapter invocation.
6. Expired/revoked/consumed/unknown permits never run again.
7. Input mutation, symlink/parent replacement, traversal, duplicate path, size limit, output collision and output symlink are refused without an output artifact.
8. Successful archive is byte-deterministic across repeated input ordering and has a verified final digest.
9. A known pre-side-effect refusal is a recorded failure; a crash/ambiguous claim or receipt commit, timeout, or uncertain filesystem publication remains unknown and produces no retry. Verify an existing output file never substitutes for a committed success receipt.
10. Legacy approval rows and existing decision/execution endpoints retain their current behavior.

## Verification

No tests, database, filesystem adapter, model, remote service or production operation were run. The findings above are design requirements for the implementer; the child is not ready to start implementation until the plan records the typed pre-approval spec, legacy status separation, migration/index strategy, and exact filesystem/unknown-result contract.
