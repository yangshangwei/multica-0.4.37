# Independent lifecycle control review

Decision: **approve complete AC09/20/21/22 at their P5 primary layer** from the
named current evidence. No blocking implementation, privilege, assertion or
source/receipt finding. The reviewer did not implement this lifecycle slice and
made no product, spec, ledger or commit changes. AC34/57, installed clients and
real provider account evidence remain separate; full task acceptance is false.

Reviewed final source is `e2e/collaboration-lifecycle-controls.spec.ts`
SHAda1bc19cb1fbd6074b6b9b85df47a89e05730e2608a0997b19724d66915a1fd4 and
`e2e/fixtures/collaboration-lifecycle-controls.ts`
SHA3cbac90ca7adfd9aef3b17b9d5c93b5bd8138afe4e7069f3fdd8b5456949651d.
Both actual creator surfaces run the same flow: Web creator/Electron handler,
then Electron creator/Web handler. Public registration, owner consent, creation,
typed comment, removal and control actions use production APIs and current
observations; the UI drives the named comment/removal/Stop/Resume interactions.

## Clause decisions

**AC09.** After the original handler input actually starts in a contained Linux
provider, removing its author leaves A paused. A new creator comment submitted
during that pause is accepted as waiting with exact `target_paused` preview.
Both real clients refresh and lose/restore Chromium fetch; native account IPC
also refuses while the server-switch fence is active and succeeds after restore.
The real daemon transport closes sockets, observes failed runtime requests and
then receives successful requests. Session/execution remains identical, the new
input stays waiting, and no additional A provider visit occurs. The same checks
repeat after proof/rejoin/new resubmission. Reconnection is actual interrupted
transport, not merely a mocked service-shaped read or token reuse.

**AC20.** B is explicitly paused before the departed handler submits a real typed
waiting input. Removal cancels that exact B input and A's waiting handler input.
Original body, author, target/channel/batch/sequence and automatic cancellation
event identity remain unchanged through client/daemon reconnect, natural proof,
reinvite, fresh owner consent and explicit B Resume. B remains without an active
group and has zero provider visits. A's old cancelled input is never consumed;
the rejoined author must explicitly submit the same body with a new comment,
event, command, request key and later sequence. Only that fresh input becomes
part of the next valid batch. Returning participation or authorization does not
restore an old key's intent.

**AC21.** The initial A run actually consumes the handler's original input and
remains in flight while that handler and the creator append separate waiting
commands. Creator removal stops the former author's run, cancels its waiting
inputs, and retains the creator input without starting it. The provider socket
closes without fixture `release` of that original run; the production supervisor
creates its natural sealed proof. The transport temporarily withholds proof
delivery, preserving server `unknown`/unsettled state. The UI explicitly shows
`Stop accepted; termination remains unconfirmed`, `Waiting for termination` and
no confirmed label. After the original proof is accepted it displays termination
confirmed while the channel remains paused. Actual physical proof and accepted
business Stop are independently observed.

**AC22.** The current creator's Resume before proof receives409 `stop_unconfirmed`
and changes no execution state. The departed actor's attempted Resume receives404,
and its client has no private title or Resume control. Original proof is accepted
with the same hashed request bytes as every held/reconnected attempt, but does
not unpause A or change the refused original-key receipt. Reinvite and reconnect
still do not unpause it. Only a fresh current creator Resume starts one new A run
containing the exact creator survivor, input added during pause and fresh same-body
resubmission, in sequence. Old stopped/cancelled identities are not re-executed;
both clients show their durable distinct states after refresh. An independent C
target/channel and a nonempty peer issue retain complete original snapshots at
the removal/reconnect/final checks. Canonical tests separately cover unrelated
active execution and author/owner scope beyond this inert C/paused peer control.

## Verified execution and provenance

`lifecycle-clients-first.json` is complete current production Web/actual Electron
**2/2**, **59.501 seconds**, zero unexpected/skipped/flaky/retry. Its command/result
record six exact source hashes and explicit test-built Linux daemon/provider
binary hashes. Reviewer source comparisons are exact; binary SHA57f9aa8c2306992964aa714eef324fb5435e8218f143bb28702378adbe67d358 and
SHA80c3225030ff105764e3b65309d9b3e599c223a8914def1cb37ab6f3217a9f94 still match.
Raw JSON SHA64b1212ca0017cec9d0e0ff39bad7b9a992490aad27e3380863a07b50ac11cda
matches its result and final evidence. Unique `lifecycle-web-identities.json` and
`lifecycle-electron-identities.json` retain all actual commands, provider inputs,
namespace/run identities, transport attempts and two reconnects per case.

Both cases have exactly two A visits and zero B visits. The first original run
has `released_ms:null` and a real socket close; the second consumes exactly the
three named valid inputs. Original and already-paused cancelled inputs are absent
from every provider prompt. Proof attempts include503 and200 with one stable
original hash; no proof error is recorded. Namespace inode reuse between
nonoverlapping runs is permitted; distinct run/group identities prove new work.
The test does not infer concurrent distinct namespaces from that reused inode.

The production poller fixture starts actual Linux processes and the real
`runBatchPoller`/Runner/controller; the callback observes inputs and can release a
later provider, but cannot manufacture termination proof. Main/native routing and
preload/renderer use the approved `content-admin-preflight.json` source/ownership
proof; no unknown listener or production rollout gate change is used.

Canonical service race `lifecycle-service-canonical*` passes15 top-level/35 total,
no fail/skip,27.124 seconds on schema737 `service_s6_other`. The reviewer parsed
actual PASS events and verified JSONL SHA24735e696d5eb4e1bf212659d908b62c39b51af2138156a70751a640457f95e8.
It covers exact removal/author cancellation, non-root author, stopped-input
nonrevival, voluntary departure/minimal recovery, original-key Resume, unrelated
execution, alias holds and whole lifecycle barriers. Current strict
`lifecycle-static-paused-typecheck*`, real scoped lint and discovery pass with
final source hashes; repository `git diff --check` passes.

The implement agent's later provider routing interruption happened after the
complete PASS receipts were delivered. It is a reporting/runtime interruption,
not a product test failure and not a fabricated or retried test result. The
coordinator's `lifecycle-current.md`/`lifecycle-final-evidence.json` describe the
already-completed raw evidence accurately.

Actual clients plus contained test-created provider establish this bounded P5
workflow. They do not establish an authenticated commercial provider, full
installed Main/installer, other OS containment, same-lane squad alias UI or live
whole-issue completion/cancellation UI. Those untested clauses remain elsewhere
in the existing ledger; neither task nor parent should be archived.
