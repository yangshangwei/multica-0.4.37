# Collaboration task boundaries

Creation and new host execution remain gated. Human readers negotiate
`execution_view_version=6` to receive fixed execution modes and separate managed
termination/descendant cleanup facts. Channel order, original group-input history
and every input reader remain available. v5 readers get an unavailable graph for
host execution while retaining human issue access; direct host AI operations
require v6. A mixed legacy comment retains its human content/notifications and
blocks only incompatible AI targets. Replaying its original request after an
upgrade does not enqueue those targets.

Host mode reuses ordinary Provider execution on Linux, macOS and Windows.
Resource owners authorize the selected agent/runtime, account source and tool
permissions once for this issue, including its future authorized participants.
Separate sessions and directories organize work; they are not an OS sandbox.
The process can use host files, accounts and tools allowed by ordinary runtime
configuration. Explicit attachments and fixed Git commits remain the prepared
input; private dirty worktrees are not copied automatically.

Every direct Provider/helper launch, including internal retries, has its own
durable identity, current permit and terminal observation. A retry requires the
previous launch's accepted termination and an unsealed, authorized run. Stop
seals dispatch before cancellation; a transient disconnect can leave existing
host work running, but cannot authorize another launch. A recovered receipt or
renewed grant never resumes a queue.

Business outcome, managed execution end and descendant cleanup are independent.
`host_exit_observed` means the registered execution lifecycle ended and launches
are sealed. Cleanup may be best-effort or unverified outside that managed scope;
it never proves every detached background program disappeared. Unknown starts
or waits retain their recovery barrier. Outcome upload failure does not block a
valid end receipt; missing server records remain incomplete. An exact stored
outcome may recover its original ACK after settlement, but that does not allow a
new result write or restart.

Legacy Linux namespace runs retain `exited_proven` and their original proof
rules. Once host cutover is enabled, new roots require host capability; already
claimed legacy groups drain under their fixed original mode. Mode or managed
permission changes invalidate old consent, and renewed consent still needs an
explicit Resume.

The human `POST /api/collaboration/issues` is registered behind the persisted
creation gate and checks explicit source authority. Unknown creation outcomes
recover by the original key under `/creations/{requestId}`. Private attachments
use dedicated drafts and current-authority byte endpoints; storage keys, public
uploads and ordinary signed downloads are not substitutes. A draft bundle is
consumed once, while an original request can be replayed. Removing a comment
reference preserves delivered files and immutable input history. These human
file APIs do not expand the issue-bound CLI command surface below.

The own-result GET `/creations/{requestId}/result?source_workspace_id=...` returns
only actor/source/request identity, outcome and code. A validated HTTP 200
`rejected` result seals the old key and allows a new explicit submission, including
the same input after conditions improve. Unknown/missing results keep the old
key; reading or reloading never submits automatically. A result grants no issue
or source access; successful detail recovery retains its normal authorization.

Human creation may author initial status, priority, dates, labels/property values
and source structure/iteration in the same issue transaction. Later metadata
edits use the shared PATCH; iteration membership uses the dedicated PUT with its
source/issue/iteration versions and switch/removal reason. Project, parent,
stage and iteration changes require current source membership as well as issue
editing authority. Outside collaborators see only minimal references. A source
card move cannot substitute for transfer or whole-issue lifecycle actions.
Ordinary workspace iteration `move` previews/operations also refuse a batch
containing a collaboration issue; use the dedicated authorized workflow.
Automatic period closure/rollover retains its server-owned contract.

Source members attach/detach ordinary children through the child's existing
ordinary parent operation. The shared detail projects the complete ordinary
child reference set; only source members get open links. Outside participants
cannot browse a source directory or open those children. These writes preserve
the parent's collaboration version, handler, participation and AI queue.

Human readers use `GET /api/collaboration/issues/{id}/runs/{runId}/records?execution_view_version=6`
with opaque cursors and limit 1–100 (default 50). Every page rechecks current
issue access. It returns source-owned run records and that run's frozen prompt;
current batch placement or resource names cannot reconstruct old input/provenance.
The ordinary Provider adapter records bounded messages and exact usage under the
original run/input authors. The legacy static executor may add a version-1 `record` to its existing
`{outcome,body}` result: bounded typed messages/tool records and exact optional
usage. Missing counters are not zero; configured resource provenance does not
identify a verified provider billing account. Proofs never carry missing logs.
One completed/failed/confirmed-stopped group creates one durable account result
event. A member or interim leader result is not whole-squad completion. Its
notification opens the representative final run through an additive `run_id`
anchor; current access still gates every read, and old clients open the issue.

Human authors edit/delete ordinary comments with their original content revision;
thread resolution and own reactions use dedicated discussion endpoints. They do
not change content revisions, notify participants or dispatch AI. Discussion
receipts correlate issue/comment/request/action and the exact emoji. Comment
content remains limited to 65,536 UTF-8 bytes; its PATCH JSON envelope permits
512 KiB for escaping and old-request recovery. An oversized old request is
retired only after a correlated persisted refusal is read, retaining text for a
new explicit edit. Replies, tombstones, history and delivered files remain.

Source workspace refresh events have an empty payload; consumers re-read under
current authority. They neither carry private collaboration content nor add
source membership or execution permission. The issue-bound CLI surface below is
unchanged by these human UI/API additions.

Account/issue WebSockets carry scoped invalidation facts and recheck authority
before sending. Reconnection refreshes authorized views; it never resumes AI.
Resource forms use only the current issue roster, explicitly accessible candidate
spaces and each owner's own installation identity. A disabled participant may
remain visible for creator-managed removal without becoming an effective mention
or execution participant.

On `unsupported_collaboration_action`, do not retry ordinary issue/comment writes,
delete, batch or rerun elsewhere. Mixed edits refuse atomically. Machine credentials
cannot borrow human/source membership for shared content or control. Ordinary
execution also rejects captured/merged collaboration sources; an ordinary child's
parent reference grants no access to private collaboration content.

Use dedicated controls: target Resume is atomic, Resume All reports channel/hold
results separately, and consent never releases a hold. Recover the original key
at `control/operations/{requestId}` without automatic retries using newer versions.
Resource withdrawal or rebinding never removes the old run's proof requirement.
Held input cannot block valid input. Root batch return requires complete group
nonlaunch proof. M11 continuation input can return while unrelated members work
only after every run that read it proves nonlaunch; one reader's proof cannot
erase another reader's work. Final squad outcome follows the last group-allocated
leader turn, not transaction timestamps. Environment changes use an immutable Switch proposal, each
actual owner's consent, then an atomic commit after old work is proven ended.
Commit leaves `switch_hold`; Resume is a separate explicit action. The former
`runtime-binding` route is removed. Recover a Switch operation by its original
key at `switch/operations/{requestId}`; late consent never changes a refusal.
Configuration writers persist invalidation. Shared skill body, supporting-file
and release changes invalidate enabled bindings and require fresh owner consent;
identical bundles and presentation-only changes preserve consent.

## Commands inside an admitted run

Both supported execution modes supply five identity variables:
`MULTICA_COLLABORATION_TOKEN` (`mcb_` business capability),
`MULTICA_COLLABORATION_SERVER`, `MULTICA_COLLABORATION_CONTEXT`,
`MULTICA_COLLABORATION_ISSUE_ID` and `MULTICA_COLLABORATION_RUN_ID`.
Legacy v2 context is the complete run identity JSON. Host v3 wraps that identity
as `{protocol_version:3,containment:"host_process_supervision_v1",identity:...}`.
The issue/run variables must match the bound identity. Partial, unknown-version or
conflicting identity refuses before ordinary profile or credential lookup.
`MULTICA_COLLABORATION_RUNNER_CONFIG` configures the host daemon and is not a
provider identity.

Use only these existing commands for the bound issue UUID:

```text
multica issue get <issue-uuid> --output json|table
multica issue comment list <issue-uuid> --output json|table
multica issue comment add <issue-uuid> --content <text> --request-id <uuid>
multica issue comment add <issue-uuid> --content-stdin --request-id <uuid>
multica squad activity <issue-uuid> action|no_action|failed --request-id <uuid>
```

Comment add permits `--parent <comment-uuid>`; activity permits `--reason`.
The two writers require `--request-id` in this run context. Keep a distinct UUID
for each intended comment/activity, and repeat the exact UUID and payload after
an uncertain response. Do not create a replacement key. This contract does not
add idempotency to ordinary task commands. Authorization rejection means stop;
do not sign in as the runtime owner or switch credentials.

Reads return authorized run context and the newest 200 comments. Other commands,
comment filters, attachments/content-file and profile/server/workspace overrides
are unsupported here. M08 delegation uses a comment mentioning a frozen roster
member, then the leader ends its turn. Member replies/completion coalesce a new
leader continuation; an eligible human M11 input joins that continuation without
rewriting a running prompt.

Host runner configuration selects the already registered ordinary runtime and
its verified provider version. It uses the current Multica CLI and an independent
collaboration working directory; legacy static configuration instead mounts its
explicit CLI at `/multica`. The adapter supplies only issue-scoped business
credentials, refreshing them for each real attempt. Managed and termination
credentials are not injected into Provider arguments/environment. This is an
application protocol boundary, not secrecy from a hostile process under the same
OS user. Native Windows shell/batch wrappers require a Job before resume and
confirmation that their managed Provider child ended; native executables can use
the declared direct-process downgrade. Platform/Provider combinations stay
unavailable until their actual acceptance evidence is recorded. Compilation and
one successful fixture are not general Provider or platform acceptance.

Human source members may reorder without advancing collaboration revision. Other
edits/handoffs retain original versions and keys; stale status confirmation needs
an explicit human decision. Full contracts and source anchors are in
[working-on-issues-source-map.md](working-on-issues-source-map.md); ordinary rules below are not shared APIs.

