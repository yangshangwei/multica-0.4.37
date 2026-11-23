# Independent multi-owner departure and public Switch review

Decision: **approve complete AC49 and AC40 at their P4 primary layer**, using the
two new exact service journeys plus current canonical adjacency. No blocking
scope, assertion, isolation or authorization finding. AC96 remains partial; this
slice does not establish its entire all-owner/catalogue/runtime/ordinary matrix.
The reviewer did not implement the two tests and made only these review records.

Final source pair: `collaboration_owner_departure_targets_test.go`
SHAb04abb1716ed2fff890cd76b5ca709aeaaa61c7444baba60a18beb7d982ab900 and
`collaboration_public_switch_resume_test.go`
SHAce1ac1cd8e0bce606f00ba0c4718b817a010a89a86ba5c37850464cdb1a00ede.
Only test files are introduced; the separately reviewed owner-context bugfix is
already committed as424ea35ed. Tracked test fixtures build real service grants,
scope and permissions; no `ReconfigureTarget` substitutes for public Switch.

## AC49 whole clause

The departure fixture gives D two direct agents and one squad member, each with
its own runtime/daemon binding. An independent creator owns the squad leader and
another unrelated C target. Public Register/Consent makes all four targets
selectable; actual service List/Claim/Permit consumes original direct/root inputs
and a real squad comment delegation creates/claims/permits D's member run.
The member assertion verifies its own grant/epoch/runtime/binding/installation/
owner plus original parent and member step. Leader daemon and leader business
token cannot claim/read that member run. Every actual resource owner's
authorization remains independent of the request daemon.

D has one waiting input on each affected target and creator C has its own waiting
input on each. D's actual public Leave revokes **exactly three D grants**, no
live D grants remain, all three D inputs cancel with `participant_left`, and all
three creator inputs remain waiting. Affected direct/squad targets are held;
all four possibly-live direct/root/member runs retain unknown proof, exact
incremented stop generation, stop-request records and unreleased reservations.
Affected runtime candidate lists and stale member claim cannot continue work.
The new test does not accept a synthetic business Stop as physical termination.

Unrelated C already has a completed proof/history, an active permitted run and a
future waiting input. Its full target/channel/grant/command/group/run/attempt/
proof/reservation/record snapshot stays byte-equal, and whole issue remains active
with original control version. Immutable group inputs, consumed commands,
dependencies, run/prompt/nonce/parent/grant identity, attempt/proof identity and
record facts are compared before/after with only legitimate mutable stop/
detach fields excluded. Source membership does not restore D's departed
collaboration control. Own departure GET returns the retained receipt without
changing protected state. These are concrete multi-target/member and other-owner
checks beyond earlier single-group removal coverage.

## AC40 whole clause

The public Switch fixture creates one typed squad S, retains one cancelled and
one stopped/proven original input, then appends three waiting S inputs by two
authors. A new leader's separate direct target has one currently permitted input
and another waiting head on its active destination channel. Actual squad writer
changes the configured leader. Public Propose/Consent captures that configuration;
another non-owner consent is refused, and staging/consent leaves all live S
target/input/history state unchanged.

Public Commit switches the existing S identity/configuration to the new channel,
keeps `switch_hold`, increments exact target/configuration versions and appends
its three waiting input sequences **after** the destination head in original
relative order. Existing typed target/resource/kind stay S; old stopped/cancelled
inputs and original old channel do not move. The unrelated direct target and
current run retain state; Switch does not stop or hold them.

One switched S waiting input is explicitly cancelled. After the direct current
run's proof, its earlier direct head starts and finishes normally while the two
remaining S inputs are still waiting. Subsequent List stays empty for S until
fresh explicit Resume releases its hold. That Resume/Claim creates a new run in
the actual replacement agent/runtime/channel, consuming exactly the two retained
S inputs in order with their original typed target. Terminal input identities,
stopped group/run/reader/dependency/attempt/proof snapshots and old lane remain
identical through completion. Original Switch GET receipt remains original and
does not mutate later execution.

Current canonical Switch tests cover every owner's new consent, no live writes
during staging, refusal/replay identity, old proof barriers, required-write
rollback and unknown commit GET. Transfer's canonical test separately proves
inviting/transferring responsibility does not switch existing execution or invent
membership/consent. New target Register and existing-target Switch are exercised
as separate operations. This closes the ledger's exact missing public staged
Switch-to-busy-destination then fresh Resume/Claim clause; it is a service journey,
not a browser or physical provider execution claim.

## Evidence verification

Reviewer compares exact current source pair to
`owner-switch-isolated-head-final-source.json`. Coordinator HEAD plus prerequisite
424ea35ed and only these two files passes serial race **2/2**, zero fail/skip,
**14.247 seconds** on schema737 `service_s6_commit`; no other dirty source is
copied. Raw JSONL
SHA081c57e9d674d57b1c68fcd65991fd06a4ceb026145017d31e88077838cf9f10 matches
the result. Both exact named top-level PASS events are present.

Fresh `owner-switch-adjacent-final-20261022*` passes **26 top-level /132 including
subtests**, zero fail/skip, **36.991 seconds** on schema737 `service_s5`. Reviewer
parses actual test events, confirms every expected identity and JSONL
SHA2efacb3739db3833d3dfdb16407e2687f29b01e57c9b059cce7cf03ad874d2ee.
Final pair source matches before/after and isolated manifest; gofmt diagnostics
are empty and `git diff --check` passes. Coordinator scoped service/auth vet
also passes. Earlier source attempts, intermediate identity-fixture FK deletion
and retained source-isolation corrections are not substituted for the final
current hashes/results.

Claims/permits/outcomes/proofs here use real service transactions and synthetic
proof bodies, not a running kernel process or commercial provider account. This
P4 canonical scope suffices for AC40/49; native UI/proof has its separate evidence
and full AC96 still needs independent exact all-owner/catalogue/derived/old
credential matrix review. Installed Main/old clients, other OS and account/quota
smoke remain separate authorization-scoped work. Neither task nor parent is
archived and production gates remain closed. Recommendations are not ledger
mutations; coordinator owns final promotion and Lore commits.
