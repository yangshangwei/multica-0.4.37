# TC-13 post-QA judgment protection — independent check

**ACCEPTED_TC13_DEFINED_QA_PROTECTION_CASE; blocking findings 0.** Main may
integrate the two exact test files and mark the defined TC-13 case PASS using
this new service/actual-router proof together with the original accepted R33
human UI/history evidence. No product change was required. This is not a new
browser journey, real-model judgment test or full PAY-241 acceptance.

The [JSON record](tc13-independent-check-20261119.json), SHA-256
`0e96f229edd542633b7aa8e6526f25da882816fdc4bd2e6ed82b32920dc1b7c2`,
contains 56 source/configuration/command/raw/catalog references and the exact
named inventories. The reviewer ran no tests, DB/API requests or processes.

## Actual verification

| Package/mode | Tops | Named records | Result |
| --- | ---: | ---: | --- |
| Service normal | 2 | 10 | PASS |
| Service race | 2 | 10 | PASS |
| Production router normal | 1 | 7 | PASS |
| Production router race | 1 | 7 | PASS |

Normal/race names match exactly, each starts and terminates once, with zero
failures, skips, duplicates, missing or unfinished records. These are **17
unique named tests / 34 executions**, including three tops and fourteen
refusal children; they close one business case, not 34 business cases.
Compile-only checks, two-package vet and gofmt all exit 0 with empty output.
All executed source/configuration/tool/raw bindings match their records.

## What is proved

The service tests create a real passing report and settled protocol run through
the existing runtime owners, then record an actual human QA **return** or
**approval** while the task remains in QA. In each state, a task principal and
a JWT principal marked AgentActor attempt the opposite decision and revocation
of the exact human decision. They receive the existing human-only refusal.

After every attempt, seventeen complete SQL collections, the stage/candidate/
review projection and the recovered original human receipt remain unchanged;
no denied operation is persisted. The derived review head is covered by the
unchanged full decision rows; report bodies/logs live in the compared run-event
rows, and collaboration events are the retained outbox. Only read-time
`CheckedAt` and signability `MaxExpiresAt` are normalized. Actual decision
expiry, signer, timestamps, dependencies and content remain exact.

A distinct legitimate opposite human request succeeds after all refusals,
proving that the service envelope was otherwise current and usable. Its new
decision does not rewrite the original immutable judgment. The approval path
uses the exact usable passing report, code-review dependency and bounded
future expiry.

The router test calls the actual `NewRouter` handler with real authentication
and `TaskCapabilityGuard`. Two actual daemon claims issue generic and
restricted `mat_` credentials; each token's persisted user/Agent/task/workspace
binding and a successful supported ordinary GET are checked before the refusal
baseline. Generic task identity and JWT-plus-Agent hint get `403 forbidden`;
the restricted token gets the distinct closed-capability refusal. Six precise
opposite-decision/revocation requests preserve seventeen workspace collections,
stage/evidence projection and the original QA receipt.

The router's valid human QA-return fixture has no runtime report. Its
opposite-approval request therefore proves **authentication ordering**, not
full approval admissibility. A human control reaches the expected
`409 stage_test_report_unusable`; the service tests provide the fully signable
opposite-approval proof. This explicit layered boundary is sufficient for the
agreed TC-13 gap and does not claim a newly executed router approval journey.

## Source and environment closure

The original server source is committed HEAD `50b50a7c9`; all 5,089 private
baseline rows remain unchanged. The frozen service execution source has 5,090
rows; the later router source has 5,091 and selects only its router package.
The two owned file hashes are:

- Service: `090e46545b770b5bae4d958a8060a306f59821321f29c16cc47db16296bb8d3a`.
- Router: `985acd0d76b54d3a487db5e6ff5ad30e2181354893edaec52ce12e46659e0e90`.

Current ROOT production source still matches. One unrelated ROOT
`server/.omc/state/.../hud-state.json` changed; it is runtime bookkeeping, not
an executed frozen-source change. No blanket 5,089-row ROOT identity claim is
made.

Four separate fresh package/mode databases have matching 856-entry migration
ledgers and 663 indexes; 1,712 migration files match the template source.
Configs retain exact hashes/mode0600 and private trusted HOME/TMP/GOTMPDIR.
Every final database has zero connections and zero tested business rows.

The existing service fixture unconditionally clears `creation_enabled` during
cleanup. Both raw after-states retain that observed false value; Main then
restored only the original true flag, preserving epoch 2 and all other gate,
catalog and table values. Router gates remained unchanged without restoration.
Each service database retains one internal organization plus six config and
six knowledge invalidation rows; each router database retains six config and
two knowledge invalidation rows. They were not deleted to manufacture emptiness.

Source, timing and row shape explain the retained fixture history: resource
configuration/deletion triggers and the fixture's participant invitations.
Successful raw test output does not retain the original fixture UUID/request
map after cleanup, so exact invalidation-cause-ID joins are not claimed. The
no-side-effect conclusion rests on the assertions surrounding each denied
request, not post-cleanup counts.

## Spec and remaining limits

The new “Post-QA machine refusal regression” section in
`collaboration-stage-approvals.md` accurately states the two-layer proof,
`mat_` versus `mcb_` distinction, immutable judgment, read-clock normalization
and explicit gate restoration. No additional spec or product correction is
required by this review.

HTTP authentication here is the actual legacy-mode router TestMain; service
fixtures also exercise their established password credential context. This is
not a new password-deployment or physical Provider test. R33's human UI evidence
keeps its original source and denominator. Real model quality, external S4,
final S5 and the other parent-case gaps remain open. Main owns exact integration,
matrix/spec synchronization and the directed commit/push.
