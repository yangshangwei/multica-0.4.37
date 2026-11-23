# Durable delegation receipts

## 1. Scope / trigger

Opt-in idempotent ordinary issue comment creation and recovery. Reuses the
existing comment routing and task admission engine. Per-target receipts cover
explicit agent/squad mentions, matching `trigger_outcomes`. A keyed comment
without explicit targets may use existing implicit routing, but its empty
target list does not prove or identify that implicit dispatch. The new opt-in
does not promise automatic implicit follow-up recovery without a retained
explicit target admission; use ordinary no-key comments for legacy routing. Immediate CLI consumption
is described separately in `squad-delegation-receipts.md`. No-key creates,
edits, quick actions and historical comments do not acquire this guarantee.
No new execution engine, completion proof or automatic unknown recovery exists.

## 2. Signatures

- `POST /api/issues/{id}/comments` with a canonical nonzero UUID `Idempotency-Key`.
- `GET /api/issues/{id}/delegation-receipts/{key}` returns the original receipt.
- Comment JSON adds `delegation_receipt`: `key`, `state`, `comment_id`, `targets`.
- Targets preserve `target_type`, `target_id`, `status`, `reason_code`; actual
  admission paths additionally carry `task_id`, `executing_agent_id` and optional
  `executing_squad_id`. Authorized accepted targets may carry
  `runtime_observation: {status, observed_at}`.
- `comment_delegation_receipt` retains request digest, actor, originator, issue,
  source task, comment, initial/final response and observation timestamps.

## 3. Contracts

Key namespace includes workspace, verified actor and originator. Request digest
binds resolved issue and normalized saving/routing inputs. Source task is frozen
as evidence separately from retry identity: a new authorized task of the same
agent and originator can recover an old receipt. A changed originator cannot
read another person's private receipt. Resource aliases resolve before DB use.
Supplied actor headers cannot manufacture a durable machine identity.

The first release refuses keyed requests containing attachments before writing;
legacy attachment comments retain their behavior. Their separately committed
linking and expiring URLs are not a frozen receipt contract.

Claim and comment commit together in a short transaction under current authority
fences. Only the definite first committer enters existing routing/admission.
The service includes mutexes, external overlay setup and event publication: do
not shallow-copy it or wrap its entire execution in a long DB transaction.

A request starts as `unknown`; successful final recording becomes `recorded`.
`recorded` means the admission report was stored, not that all targets ran.
Blocked or unknown targets remain explicit. Missing/malformed task binding never
becomes a successful durable admission fact. Initial unknown target slices must
not alias mutable final target slices; finalization failures must expose only
retained uncertainty. No repeated POST or GET takes over unknown work. Automatic completion and
cancelled-comment survivor reconciliation must also consult retained admission
facts before enqueueing. Explicit authorized edits are separate legacy mutations;
they never rewrite the original receipt.

Use actual task IDs returned by enqueue and atomic merge/planned registration.
Never select the latest task to reconstruct identity. Freeze the requested target
UUID separately from its executing agent and historical task squad context.
Automatic replay requires an originally admitted exact target type/UUID and
executing agent; a shared agent alone cannot authorize a previously rejected
squad. Historical `executing_squad_id` is not the requested routing context:
a direct agent mention may have been covered by an existing squad task and
still deserves its originally requested direct-agent follow-up. Roster replacement must not rewrite
history. Coalesced means the existing queued task covers the comment. Deferred
means a concrete task retained its planned obligation; it is not evidence of a
future completed run. The ordinary engine's completion reconciliation remains
responsible for follow-up, with its existing limits.

Lock the target issue before reading its current project for restricted reporter
scope checks; locking the reporter source issue alone cannot fence a different
target moving out of the project. Use NOWAIT across this lock graph.

Every read/replay checks current issue/workspace, credential and principal
permissions. Original task completion does not transfer ownership of the key;
new valid task credentials must meet current authority. Rejected private targets
retain only request-known identity and a generic reason, not privileged resolved
runtime details. Successful keyed responses use `Cache-Control: no-store`.

Transiently offline runtimes retain existing queued-wait semantics. Persist the
actual availability observation made after admission by reading the returned
task ID and its bound runtime ID, never the agent's newly rebound runtime. The
UTC `observed_at` marks that sample, not continuous liveness or atomic status
throughout enqueue. Later runtime changes
must not rewrite it on recovery. Missing/unusable runtime refusal is distinct. Admission alone is never execution or delivery completion.

## 4. Validation / error matrix

| Condition | Outcome |
| --- | --- |
| Invalid/duplicate/noncanonical/zero key, unsupported keyed attachments | 400 before claim/comment |
| Same authorized key and request | Original comment and retained receipt; no dispatch |
| Same key with changed request or issue | 409; no new comment/task |
| Busy authority/claim contention | Conflict; no fabricated success |
| Current authority lost or other subject's key | 403/404; no private receipt leak |
| Commit/dispatch/finalization uncertainty | Non-success or retained unknown; no automatic rerun |
| Task binding cannot be proved | unknown target; never queued proof inferred from HTTP |
| Replaced squad member/leader | Original frozen UUID and task facts stay unchanged |

## 5. Good / base / bad cases

Good: caller keeps its UUID key, loses a response, and recovers the same stored
comment and actual task ID after restart without posting again. Base: older
clients omit the key and keep immediate-only behavior. Bad: generate a new key
automatically after a timeout or infer completion from `recorded`/`queued`.

## 6. Required tests

Use separate owned handler/router databases and real credentials. Assert actual
comment/queue/receipt identities for repeated and concurrent requests, changed
payload, queued/coalesced/planned deferral/refusal, roster replacement, current
revocation, actor/originator isolation, restart and unknown no-rerun. Fault tests
must distinguish pre-commit rollback, uncertain commit and finalization failure.
CLI tests assert exact per-request key, no ambient/sticky header, no automatic
transport replay, preserved JSON/exit semantics and read-only recovery.
Migrations use no foreign keys or inline indexes; each concurrent index has its
own file. Rollback fences must refuse retained evidence. Because concurrent
index drops span separate migration steps, disable admission and hold a
maintenance window across the entire rollback; never delete retained rows to
make a downgrade succeed. Operational rollback keeps evidence and guards. Validate sqlc idempotence,
build/vet, normal/race, skill contracts and final source hashes. Preserve every
failed run; simulated queue tests do not close real-provider acceptance.

## 7. Wrong versus correct

Wrong: store only after dispatch and retry when no row exists, alias initial
unknown targets to mutable results, or use a current roster to rewrite a receipt.
Correct: claim before effects, preserve uncertainty across process loss, and
record IDs returned from actual persisted admission operations.
