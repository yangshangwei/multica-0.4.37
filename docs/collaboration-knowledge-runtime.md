# Collaboration knowledge runtime operations

Guarded knowledge use is **on by default** when its startup setting is unset or
empty. Explicit disables remain effective. Formal project collaboration
acceptance can generate and adopt source-bound knowledge through the configured
server model and project policy; manual human proposals and decisions remain
available. Adoption never grants wider source access or writes project/workspace
base text or skills.
Built-in local Claude and Codex have provider-specific private-consumption
adapters. Custom profiles and remote runtimes are not admitted. Production
enablement still requires acceptance for the actual provider/version/platform.

## Startup setting

```dotenv
MULTICA_KNOWLEDGE_CONTEXT_ENABLED=true
```

Unset and empty values enable use. For a nonempty value, only the exact string
`true` enables it; `false`, `1`, `TRUE` and whitespace variants remain off. The router reads it once at construction, passes it through
`handler.Config` to its `TaskService`, and collaboration runtime helpers use that
same instance gate. Change the configured value and restart/recreate every API
process to apply it; changing a shell after startup or a database flag does not
update an existing instance. Mixed settings across replicas cause inconsistent
admission and must not be used as a rollout method.

[`docker-compose.selfhost.yml`](../docker-compose.selfhost.yml) forwards the value
with a true default; [`.env.example`](../.env.example) documents it.
[`scripts/check.sh`](../scripts/check.sh) forces its generated check environment
to false even when inherited configuration enables it. Existing collaboration
creation, host execution and owner-consent gates still apply independently.

## Upgrade order

1. Apply the current migrations, including the automation records introduced in
   844–860 and 865, and matching server/daemon/client code.
   Use the repository migration runner; index migrations run individually and
   concurrently. Verify retained candidate, target revision, invalidation and
   internal task delivery support are present, not merely a migration ledger row.
2. Use a maintenance window to quiesce writes while all serving API instances,
   clients and daemons acquire strict v1/v2 readers and current source/session
   fences. Restore traffic only after those readers are present. Do not mix old
   API instances with writers of new records. Keep existing explicit startup and
   project-policy disables; a default upgrade must not overwrite them.
3. Check current automatic-knowledge acceptance and exact source/configuration
   evidence in the
   [automation task](../.trellis/tasks/11-14-project-knowledge-automation/implement.md).
   Its Web/Desktop journey must prove formal acceptance, adoption, actual daemon
   input, immediate reopen refusal and reacceptance with a fresh proof, plus
   no-project/manual zero-work cases. Also retain the source-bound G2 disposition
   and integrated evidence in the
   [knowledge task](../.trellis/tasks/archive/2026-11/10-30-collaboration-knowledge-promotion/implement.md).
   Required coverage includes ordinary HTTP/batch/live WS and managed claims,
   all ordinary prompt branches, real server/DB/daemon composition, collaboration
   HostExecutor composition, role/purge/expiry races, native history, private logs
   and source/target UI invalidation. A test name or compile-only run is not a pass.
4. Verify the intended built-in local provider version accepts its admitted
   invocation and session policy. For Claude, verify the safe print options. Collaboration private runs require official print-mode
   `--no-session-persistence`; an unsupported version is refused without retrying
   after removing the flag. An arbitrary Claude-compatible wrapper/profile is
   not supported. Ordinary resumable knowledge uses a different history policy
   and refuses that flag. Codex accepts no default/custom CLI arguments for
   managed knowledge; choose model/thinking/service tier through the structured
   settings. Ordinary Codex knowledge uses an acknowledged persisted thread.
   Collaboration requires an explicitly ephemeral thread and rejects missing or
   ignored persistence policy before sending knowledge input. A failed managed
   resume is terminal and is never automatically replayed in a fresh thread.
5. Keep the invalidation worker, relay and current credential checks running.
   Ensure operators can observe the content-free backlog diagnostics below.

The retained [ordinary composite report](../.trellis/tasks/archive/2026-11/10-30-collaboration-knowledge-promotion/research/composite-acceptance-2026-11-01/handoff.md),
[C4 handoff](../.trellis/tasks/archive/2026-11/10-30-collaboration-knowledge-promotion/research/collaboration-runtime-integration-2026-11-01/handoff.md)
and [UI evidence](../.trellis/tasks/archive/2026-11/10-30-collaboration-knowledge-promotion/research/ui-platform-acceptance-2026-11-01/summary.md)
describe bounded snapshots and distinct scopes. They do not collectively declare
G2 passed. Final integrated/privacy checks and the task's final disposition own
that decision. Fake Provider processes prove application input/lifecycle delivery;
they do not prove model comprehension or a real Provider/version/OS acceptance
matrix. Windows, installers and multi-account/quota acceptance remain separate.

## Project extraction and adoption

Three controls are independent:

| Control | Effect |
| --- | --- |
| `MULTICA_KNOWLEDGE_CONTEXT_ENABLED` | Controls new use/disclosure/launch authorization; it does not disable extraction. |
| Project knowledge policy | Defaults to system-enabled, max3 new candidates. Current human project lead or workspace owner/admin may disable or lower max to1–3. Off stops new work and in-flight adoption, preserving already valid knowledge. |
| `MULTICA_LLM_*` | The router-owned server assist client generates/checks knowledge and bears the model charges. With both key and base URL absent there are zero upstream requests; jobs wait for configuration. No personal CLI account or original daemon queue is used. |

Only exact formal project collaboration acceptance (including self-acceptance)
creates a durable job. No-project/manual/disabled-policy cases create none.
Ordinary done, execution success, return and cancellation are not acceptance.
The acceptance transaction writes a lightweight intent; model requests happen
after commit and cannot undo the business result.

Each new batch has0–3 candidates (body2KiB, summary256B, up to4 exact quotes),
complete source material up to8 items/128KiB and complete target comparison up
to16 items/128KiB. Each request includes at most288KiB decoded input and2MiB
wire JSON. Generation and checking have separate60s deadlines,6000/3000 output
tokens and24/16KiB output limits. Each API instance processes at most2 batches;
a120s batch uses a180s lease. Reacceptance fixes all affected predecessors into
up to6 review batches plus1 new batch. These are per-instance/per-batch limits,
not a global billing cap; SDK transport/parameter retries share the deadline.

Job status preserves up to 42 complete latest-execution stage-attempt records per
job (7 batches × 2 stages × 3 allowed attempt numbers); 43 records are rejected.
The current worker writes attempt 1 only and normally produces at most 14 records.
Keep additional valid retained attempts; the normal worker count is not a
projection limit. SDK transport/parameter retries are not separate stage records.

Authorize and budget-check the complete target comparison before either model
call. Generation sees permitted source material and same-source predecessors
only; a valid empty result completes without checking. Checking also sees the
complete currently readable target comparison. Raw model
reasons and their full actual input-source closure remain private; public status
uses fixed reason codes. Usage preserves real zero, partial and missing counts;
unknown or failed calls are not reported as zero cost. Rule adoption is not a
claim that a model's statements or quoted test claims were independently proven.

Duplicates are skipped. Conflicts, incomplete evidence/comparison, changed
bindings or capacity become review work. No old contribution is evicted or body
truncated. Completed outcomes and successful phases are retained; uncertain calls
require an authorized explicit retry. Retry keeps the accepted-job identity and
old attempts, binds the current policy to a new execution, and preserves already
saved new lineages, settled predecessor slots and the accepted event's
new-knowledge quota.
That quota counts each automation-root lineage once any version was published,
including later human approval and withdrawn/superseded history. Pending
unpublished proposals do not consume it, and existing-lineage restoration or
revision does not consume new-lineage quota. The current execution's policy
limit bounds further automatic publication; human decisions do not reset the
retained count.

Reopening changes the source epoch, immediately invalidating automatic proofs
for new disclosure/authorization without waiting for an outbox scan. History
stays approved with paused usage. Reacceptance restores unchanged supported
body and summary or approves an exact-lineage revision; conflicting, expired,
unavailable or manually overridden items stay paused/pending. Untouched automatic pending
successors do not become a human veto. Manual withdrawals cannot be resurrected
under a fresh candidate ID. Paused unexpired approvals still reserve capacity.
Automatic validity follows the original approval origin. A human-approved
automated proposal retains human approval's consumption semantics.

A v2 manifest binds the candidate/version/content hash plus validity revision,
accepted event, submission and closed epoch. Original sources and new positive
support dependencies are locked and rechecked. Old automatic exposures cannot
be silently replaced with a new proof or replayed into a fresh fallback session;
a separately authorized fresh execution prepares the current proof.

Candidate APIs negotiate `X-Multica-Knowledge-Version: 2`; an absent header means
v1, and the candidate body's `protocol_version` must agree. Policy/jobs require
header version 2, with no `protocol_version` in policy/retry bodies. Duplicate
header values return `400 invalid_knowledge_request`; an unsupported single
value (including empty), or v1 negotiation for policy/jobs, returns
`409 knowledge_protocol_unsupported`. A supported candidate header/body mismatch
returns `400 invalid_knowledge_request`. Malformed/duplicate/unknown policy/retry
JSON fields use the workflow decoder's `400 invalid_request`; decoded required
field and canonical/range validation uses `400 invalid_knowledge_request`.
V1 serves only complete representable legacy scopes, never filtered fake-empty
collections or fabricated human actors. Original v1 receipts keep their sealed
version and values.
Ordinary retained metadata reads1/2 explicitly, with old nested claims retained
in a monotonic v2 envelope. Collaboration keeps the discoverable database slots
`knowledge:claim:v1` and `knowledge:prepare:v1` while versioning their payloads.
After new records exist, disabling automation is not permission to deploy an
older reader or delete history to force a downgrade.

Project policy lives at `/api/projects/{id}/knowledge-policy`; issue job status
and authorized retry live at `/api/collaboration/issues/{id}/knowledge-jobs`.
Policy/retry writes have original-key recovery endpoints and source-safe display
leases. These endpoints do not expose a public system-approval command.
An exact policy/retry write refusal, including same-key write replay, returns
HTTP 409 with its sealed version, code, request/resource identity and revision.
Operation GET returns HTTP 200 with those same sealed values and no body `status`
field; a refusal code remains a refusal. After transport
uncertainty, query that original request; role loss does not erase a committed
actor-owned receipt. Do not replace an uncertain write with a new request ID.

## Expected behavior and diagnosis

The daemon places its managed Claude settings before configured default/custom
options and removes competing settings options. An incomplete configured option
may still fail CLI parsing, but cannot consume the managed settings flag and
silently restore inherited skills. Fix the configuration rather than bypassing
the managed policy.

| Observation | Meaning and next check |
| --- | --- |
| Approved item absent from an input | Check the switch, negotiated built-in runtime, applicability/target, current source availability, actual originator/runtime owner and output audience. Approval is not a source visibility grant. |
| `knowledge_context_changed` | Claim/session/reviewed manifest/invocation changed. Preserve the original result and cleanup; do not auto-replay the business input. A current daemon that cannot reach the selected prior session starts a fresh bound session with the ordinary continuity notice. An older daemon talking to the updated API cannot report that drop and remains refused with `knowledge_context_changed` on each subsequent turn for the affected session until the daemon is upgraded. An upgraded daemon talking to an older API instead reports `knowledge_unavailable` because that API rejects the new field; upgrade the API first. A claim descriptor the daemon cannot decode refuses only that task; the other tasks in the claim run. |
| `knowledge_authorization_denied` | Current credential/consumer/source authority no longer permits disclosure/start. Accountable or installer identity is not a substitute. |
| `knowledge_busy` / `knowledge_unavailable` | Bounded lock/dependency/proof failure. The server already re-runs a contended preparation or authorization for up to 1.5s and the daemon retries three times, so `knowledge_busy` means sustained contention, not one sibling claim. Inspect content-free health/DB diagnostics; never force a base fallback that can restore exposed history. |
| `knowledge_invocation_unsupported` | Runtime, selector or effective invocation is outside the admitted contract. Check retained current configuration, not only its registered protocol name. A fresh task whose options sit outside the negotiation allowlist, such as `--allowedTools` or `--add-dir`, runs without knowledge only when its provider namespace has no recorded private exposure; with recorded exposure or a retained negotiation it still refuses. Whatever the switch, Claude session selectors and hand-offs (`--continue`, `--resume`, `--teleport`, `--cloud`, `--remote-control`, `--bg` and similar), a subcommand operand such as `attach`, a bare `--`, and agent `custom_args` that are not a JSON string array refuse. |
| `knowledge invalidation scan deferred` / `delivery deferred` | Pending work is retained for retry. Restore DB/relay availability; do not mark jobs delivered manually. |
| `knowledge invalidation backlog` | `pending_jobs` and `oldest_seconds` are reported at most once per minute when the oldest job is at least 30 seconds old. No source content is needed for diagnosis. |
| `task claim: knowledge history load failed; requeueing claim` | A database read failed while the server built a claim's knowledge decision. The task returned to the queue with no failure reason and the next poll retries it; it was not settled as `knowledge_unavailable`. Restore DB availability. The same task logging this on every poll is a persistent fault to investigate. |
| `task claim: knowledge finalization refused; settling task` | Claim finalization refused the task with the logged code, and the task failed with that reason instead of returning to the queue. With `knowledge_context_changed`, the usual cause is a task re-claimed 32 times after lost claim responses or daemon exits before start. Check daemon connectivity; do not requeue the task by hand. |

The worker wakes every 500ms and handles at most 100 scan and 100 delivery steps
per pass, preserving original hint IDs and cursors across restart. Target hints
contain only kind, target ID and revision, with current scope and original-token
freshness checked immediately before send. UI clears private data before current
GET; a 30-second maximum monotonic lease and 15-second visible refresh bound stale
display when hints fail. This does not retract bytes already lawfully disclosed.

Source-bound contribution text belongs only in authorized projections and the
actual per-turn prompt. Generic logs, stable files and supervisor ledgers must
not contain it. Authorized failure evidence can contain Provider quotations;
retain it in its existing private outcome/transcript path without copying it to
generic logs or diagnostic reports.

An uncertain unreachable-resume preparation is replayed with its original
request ID, original claimed session, invocation fingerprint and
`resume_unavailable:true`. The server returns the same fresh binding after
reauthorization; the daemon does not manufacture a new generation or repeat the
business input. A newly keyed request cannot replace an already prepared claim.
Keep this transport replay distinct from starting another model process.

## Disable and recover safely

Set the switch to false and restart the API instances using the guarded version.
New positive negotiations, disclosures and launch authorizations stop. This is
not a kill switch for an already started process. Existing Stop/termination
observations, fenced completion/failure, original receipts and recovery remain
necessary; do not clear an uncertain launch just because admission is closed.
An ordinary provider namespace with retained private exposure can also refuse
legacy/base delivery, even with no selected session pointer. Closing the switch
does not make that history safe to resume through an older or unsupported client.

Keep guarded history readers, candidate/version/receipt records, task exposure,
target routing, invalidation scans/hints and the worker. Never roll back to an old
claim reader that treats private history as ordinary reusable state. Migration
downs reject retained candidates, negotiated task data and pending outbox work;
do not delete those records to force a downgrade. The stored per-task knowledge
record is read strictly, and any change to its shape bumps its version. Install a
release that reads a new version before any release writes it, and roll back only
to a release that reads every version written since. An older reader refuses the
affected tasks with `knowledge_context_changed`; it never rewrites them. Target deletion must not
resurrect a target from delayed scans. Preserve backups and use forward repairs
for a nonempty deployment.

Authorized project/workspace deletion removes its policy overrides and target
counters, retaining final hints for published targets. Candidate, job, batch, attempt,
evaluation, validity and invalidation history remain retained without cascading
deletion. Deleting a target does not recreate it through delayed work or authorize
removal of that history.

Implementation details and regression ownership are in the
[server candidate](../.trellis/spec/server/collaboration-knowledge.md),
[execution](../.trellis/spec/server/collaboration-knowledge-execution.md),
[Core](../.trellis/spec/core/frontend/collaboration-knowledge.md) and
[shared UI](../.trellis/spec/views/frontend/collaboration-knowledge.md) specs.
