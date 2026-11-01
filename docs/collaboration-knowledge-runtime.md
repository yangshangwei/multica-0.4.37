# Collaboration knowledge runtime operations

Private knowledge execution is **off by default**. Human proposals and approvals
create retained, source-bound contributions; approval does not itself enable
execution, grant source access or write project/workspace base text or skills.
Only the built-in local Claude adapter currently supports private consumption.

## Startup setting

```dotenv
MULTICA_KNOWLEDGE_CONTEXT_ENABLED=false
```

Only the exact string `true` opts in. Empty, `1`, `TRUE` and whitespace variants
remain off. The router reads it once at construction, passes it through
`handler.Config` to its `TaskService`, and collaboration runtime helpers use that
same instance gate. Change the configured value and restart/recreate every API
process to apply it; changing a shell after startup or a database flag does not
update an existing instance. Mixed settings across replicas cause inconsistent
admission and must not be used as a rollout method.

[`docker-compose.selfhost.yml`](../docker-compose.selfhost.yml) forwards the value
with a false default; [`.env.example`](../.env.example) documents it.
[`scripts/check.sh`](../scripts/check.sh) forces its generated check environment
to false even when inherited configuration enables it. Existing collaboration
creation, host execution and owner-consent gates still apply independently.

## Before enabling a deployment

1. Install migrations through 760 and the matching server/daemon/client code.
   Use the repository migration runner; index migrations run individually and
   concurrently. Verify retained candidate, target revision, invalidation and
   internal task delivery support are present, not merely a migration ledger row.
2. Retain current claim readers, preparation/authorization/observation handlers,
   exact-session history and fenced pointer writers on every serving instance.
   Deploying an older reader can bypass protection even if its injection is off.
3. Check the final source-bound G2 disposition and integrated evidence in the
   [knowledge task](../.trellis/tasks/10-30-collaboration-knowledge-promotion/implement.md).
   Required coverage includes ordinary HTTP/batch/live WS and managed claims,
   all ordinary prompt branches, real server/DB/daemon composition, collaboration
   HostExecutor composition, role/purge/expiry races, native history, private logs
   and source/target UI invalidation. A test name or compile-only run is not a pass.
4. Verify the intended built-in local Claude version accepts the admitted safe
   options. Collaboration private runs require official print-mode
   `--no-session-persistence`; an unsupported version is refused without retrying
   after removing the flag. An arbitrary Claude-compatible wrapper/profile is
   not supported. Ordinary resumable knowledge uses a different history policy
   and refuses that flag.
5. Keep the invalidation worker, relay and current credential checks running.
   Ensure operators can observe the content-free backlog diagnostics below.

The retained [ordinary composite report](../.trellis/tasks/10-30-collaboration-knowledge-promotion/research/composite-acceptance-2026-11-01/handoff.md),
[C4 handoff](../.trellis/tasks/10-30-collaboration-knowledge-promotion/research/collaboration-runtime-integration-2026-11-01/handoff.md)
and [UI evidence](../.trellis/tasks/10-30-collaboration-knowledge-promotion/research/ui-platform-acceptance-2026-11-01/summary.md)
describe bounded snapshots and distinct scopes. They do not collectively declare
G2 passed. Final integrated/privacy checks and the task's final disposition own
that decision. Fake Provider processes prove application input/lifecycle delivery;
they do not prove model comprehension or a real Provider/version/OS acceptance
matrix. Windows, installers and multi-account/quota acceptance remain separate.

## Expected behavior and diagnosis

| Observation | Meaning and next check |
| --- | --- |
| Approved item absent from an input | Check the switch, negotiated built-in runtime, applicability/target, current source availability, actual originator/runtime owner and output audience. Approval is not a source visibility grant. |
| `knowledge_context_changed` | Claim/session/reviewed manifest/invocation changed. Preserve the original result and cleanup; do not auto-replay the business input. |
| `knowledge_authorization_denied` | Current credential/consumer/source authority no longer permits disclosure/start. Accountable or installer identity is not a substitute. |
| `knowledge_busy` / `knowledge_unavailable` | Bounded lock/dependency/proof failure. Inspect content-free health/DB diagnostics; never force a base fallback that can restore exposed history. |
| `knowledge_invocation_unsupported` | Runtime, selector or effective invocation is outside the admitted contract. Check retained current configuration, not only its registered protocol name. A fresh task whose options sit outside the negotiation allowlist, such as `--allowedTools` or `--add-dir`, runs without knowledge only when its provider namespace has no recorded private exposure; with recorded exposure or a retained negotiation it still refuses. Whatever the switch, Claude session selectors and hand-offs (`--continue`, `--resume`, `--teleport`, `--cloud`, `--remote-control`, `--bg` and similar), a subcommand operand such as `attach`, a bare `--`, and agent `custom_args` that are not a JSON string array refuse. |
| `knowledge invalidation scan deferred` / `delivery deferred` | Pending work is retained for retry. Restore DB/relay availability; do not mark jobs delivered manually. |
| `knowledge invalidation backlog` | `pending_jobs` and `oldest_seconds` are reported at most once per minute when the oldest job is at least 30 seconds old. No source content is needed for diagnosis. |
| `task claim: knowledge history load failed; requeueing claim` | A database read failed while the server built a claim's knowledge decision. The task returned to the queue with no failure reason and the next poll retries it; it was not settled as `knowledge_unavailable`. Restore DB availability. The same task logging this on every poll is a persistent fault to investigate. |

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
do not delete those records to force a downgrade. Target deletion must not
resurrect a target from delayed scans. Preserve backups and use forward repairs
for a nonempty deployment.

Implementation details and regression ownership are in the
[server candidate](../.trellis/spec/server/collaboration-knowledge.md),
[execution](../.trellis/spec/server/collaboration-knowledge-execution.md),
[Core](../.trellis/spec/core/frontend/collaboration-knowledge.md) and
[shared UI](../.trellis/spec/views/frontend/collaboration-knowledge.md) specs.
