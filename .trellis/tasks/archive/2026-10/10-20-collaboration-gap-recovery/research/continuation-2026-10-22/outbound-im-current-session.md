# Current other-IM outbound boundary recovery

## Existing behavior gap and ownership

AC84 already has four package transport regression files. The retained resume2
red and fresh current red both reproduce the Wecom relay receiver sending text
and reading media before immutable channel delivery validation. Direct Wecom
validates delivery, task provenance and current installation before touching a
sender. The separate relay consumer checks only a live socket and trusts the
frame task/session/workspace/chat. A missing query implementation can also reach
media and panic. This is an actual receiver boundary defect.

The smallest fix belongs in `wecom/relay_outbound.go:deliverRelayed`, before
text/media sender access. Reuse `GetChannelTaskDelivery`, `GetAgentTask`,
`TaskInputIsChannelIngested`, current installation and the existing typed chat
conversion. Match frame installation/task/session/workspace/chat/type against
those persisted facts. Missing/unknown/malformed or changed authority ends the
reply before any wire/storage access. Preserve the relay queue, dedupe, attempt
outcome and unrelated inbox path. No production API, dependency, migration,
provider account or externally sent IM message is required.

Owned source is the four existing integration regression files plus the minimal
Wecom receiver fix. Existing ordinary relay wire tests must acquire explicit
test-owned delivery provenance for their intended positive path; do not weaken
new private refusal assertions or allow nil queries in product code. Canonical
shared run/private attachment fixtures remain DB-backed schema737. Root owns
spec/ledger/environment/commits; broad shared work remains unstaged.

## Red reproduction

`outbound-im-resume2-red*` records3 failed top-level/19 failure events with
source-stable receiver assertions. Fresh
`research/continuation-2026-10-22-resume/outbound-current-red*` repeats the
three exact Wecom relay regressions under guarded serial race, including actual
ordinary media positive and missing-query refusal. Those are product assertion
failures, unlike the earlier Go full-package ten-minute budget.

Green/current adjacent and isolated evidence follows after the minimal fix.

## First green refinement

Fresh first fix removes all prior relay private/mismatch failures (7top/43pass),
then an existing read-fault regression exposes terminal handling of recoverable
DB lookup failures. Preserve the original relay event identity for dependency
errors that occur before wire/storage; only missing rows or immutable/authority
mismatch are terminal. Use the existing outcomeProvablyNotSent to release the
claim for bounded retry, with a fixed stage-only diagnostic and no raw DB error.

## Final current validation

Four-package collaboration selectors pass8 top-level/48 total under serial
race, no fail/skip. Full adjacent Lark/DingTalk/Telegram/Wecom race passes875
top-level/1346 total,115.391s, no failure/source drift. Three Redis-only tests
are separately supplemented; their first run passes2/3 and reveals sibling
turn fixture omitted its workspace, fixed by retaining the existing parent's
workspaceID on that test-only sibling. No queue/dedupe product semantics change.

The first adjacent attempt is retained as failed and source-drifted because the
fixture repair occurred while its compiled suite finished; it is not accepted
current evidence. The final source-stable four-package run is authoritative.
ExactHEAD+only sevenowned files passes25top/65total no fail, separate Redis
limitations; scoped go vet passes. Final isolated snapshot is refreshed only
with the eighth test-only sibling file before its final Redis/selector check.
