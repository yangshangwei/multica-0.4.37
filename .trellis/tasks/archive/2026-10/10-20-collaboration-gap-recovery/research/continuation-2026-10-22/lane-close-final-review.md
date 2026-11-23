# Independent same-lane and whole close acceptance review

Decision: **approve complete AC34 and AC57 at their P5 primary layer** using the
final exact-source six-case client run plus current canonical service/previous
Linux proof scope. No blocking product, assertion, authority or current evidence
finding. Reviewer did not implement these new tests and performed no native
launch, product/fixture/spec/ledger/commit mutation during review.

Final exact source is `collaboration-lane-close-controls.spec.ts`
SHA806b31844505bc685c9864724aa375b2de3f1264f08386a63a0c825e85594b98 and
`fixtures/collaboration-lane-close-controls.ts`
SHAceaa9fea406c8e248a00ce2b6fe910fb6f3573e9d689ff1f6f4bed4cd383afb7.
The pair reuses existing public lifecycle/native/runtime fixtures; two Web and
Electron same-lane cases and four Web/Electron complete/cancel cases all execute
at these same bytes, not just a union of old mismatching source results.

## AC34 whole clause

Public squad registration and actual owner consent place S, with A as leader,
in A's real channel. Both A and S have typed waiting inputs. Stop A's exact
current receipt lists both target identities and one paused channel; actual UI
shows the common lane and both typed resource IDs. Resume S restores the same
channel/aliases. Independent B and C channel/target snapshots remain equal.
B's own Stop/Resume preserves A/S and works when the global issue is active.

Revoking S's member resource holds S only. Resume A restores channel state while
S stays `grant_required`; reconsent alone leaves that hold intact, and only a
fresh S Resume releases it. Whole Stop pauses issue/all channels. A new D is
created/registered/consented after that Stop; individual D Resume is refused
`issue_paused`, and its new input stays waiting with the same explicit reason.
The other actual client independently reloads and tries S Resume; refusal and
visible explanation remain exact with no execution change. Only fresh global
Resume releases the whole issue. All A/S/D inputs remain waiting on these offline
resources, and the unrelated peer snapshot is unchanged. No automatic provider
execution is inferred from the offline alias case.

`observedLaneControl` refreshes current authoritative versions before each new
human action and verifies actual POST fields. It does not automatically retry or
recompute the original refused key. Existing canonical15/35 tests independently
cover active/derived Stop scope, aliases/holds, later target-set fences and
unrelated sessions. Visible same-lane/new-target/other-client clauses now have
actual matching Web/Electron evidence instead of only generic service controls.

## AC57 whole clause

All four actual creator complete/cancel cases launch permitted test-created
Linux A/B providers, one original input from each collaborator, with both authors
appending waiting input to both channels. The real shared lifecycle form sends
Complete issue/Cancel issue with observed prescribed status; it creates no fake
terminal rows. Business receipt closes the issue while both actual run states
remain `unknown`/unsettled/ending and forced Stop. All four author waiting inputs
cancel with immutable body, author, typed target, batch/sequence, actual creator
cancellation actor/event/time and exact issue-completed/cancelled reason.

Both clients independently show Closed/disabled Resume plus `Waiting for
termination`, `Termination unconfirmed` and records text `Stop accepted;
termination remains unconfirmed`. The fixture never releases either original
provider socket. The real daemon/supervisor terminates it and creates original
natural sealed proof. A task transport withholds proof delivery; after A proof
is accepted, B remains genuinely unconfirmed while closed issue and cancelled
input histories stay identical. After B proof, both actual runs are settled and
`exited_proven`, original consumed inputs remain Stopped, all waiting inputs stay
Cancelled, issue is still terminal and channels paused. Both clients show exact
history/reasons after refresh; unrelated peer is unchanged. Each original proof
has503 then200 attempts with one stable SHA256 and no injected proof body.

## Current proof and corrected earlier attempts

Final `lane-close-clients-final.json` is **6/6**, **107.534s Playwright duration /
108.507s command elapsed**, zero failed/skipped/flaky/retry. Raw
SHA3bbcab7c52cb54937c6350bc4051234513ca966476258cf8d921b7a814356558 matches
its final result/summary. Reviewer parsed all six actual statuses and original
embedded attachments, compares six current source hashes and unique
`lane-close-final-{web,electron}-{same-lane,complete,cancel}-identities.json`.
Public registration projection contains only typed target/resource/channel/name;
there is no runtime credential object in the final evidence. Embedded token
audit has zero violations. Current strict `lane-close-static-public*`
TypeScript/lint/discovery pass at the exact pair; `git diff --check` passes.

Current canonical `lifecycle-service-canonical*` has15 top-level/35 total actual
passes, no fail/skip and matching SHA24735e696d5eb4e1bf212659d908b62c39b51af2138156a70751a640457f95e8.
Relevant identities include alias independent holds, later target-set fencing,
unrelated target/issue scope, whole lifecycle barriers and original-key proof/
Resume nonrevival. Named actual Linux/daemon evidence remains separate from
synthetic service proof payloads. `lane-close-preflight.json` and its parent
verify owned API22414, production Web/build and exact renderer/index plus named
daemon/provider binaries; no unknown listeners or provider accounts are used.

Initial mixed5-pass/1-refusal and later alias2-pass results remain historical.
The refusal was legitimate `stale_control_version` after a prior Stop advanced
an unchanged visible Paused state, corrected by current observation before a
new explicit action. An earlier raw/extracted registration summary accidentally
included a synthetic managed runtime credential. The implementation audit
quarantined original files under private permissions and produced sanitized
repository copies, with `lane-close-artifact-redaction-repair.json` explicitly
recording original hashes. Those copies have different hashes because of
redaction; no original verdict is relabelled as a new execution. The current
final six-case public projection has zero credential/nonce violations and
supersedes prior source/identity attempts for acceptance. No credential value
was printed during this independent review.

The fixture executes actual clients, production supervisor and explicit stdlib
Linux provider. It does not establish commercial model/account/quota, installed
full Main/installer/old clients or other OS containment. These limits remain
separate and do not negate the named P5 clauses. Full97 acceptance is false;
coordinator owns ledger promotion, specs and isolated Lore commit.
