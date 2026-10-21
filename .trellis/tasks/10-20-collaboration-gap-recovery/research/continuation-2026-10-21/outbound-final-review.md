# AC84 outbound final independent review — 2026-10-22

**The bounded four-source outbound slice passes and is ready for a scoped Lore commit. AC84 as a whole remains `not_verified_in_this_task`.** Plugin valid-payload withholding, producer isolation and the Slack private-origin boundary now have actual transport/SQL proof. Other IM private direct, streaming/failure, relay and media paths remain unverified at their own adapters.

## Findings (fixed)

None. Four source hashes match the coordinator's complete isolated snapshot; reviewer gofmt over all four files is clean. The production change is precisely the unchanged existing14-line dispatcher gate/helper; the38-line pure predicate regression is also carried unchanged. No unrelated product/interface edit is needed by this verified slice.

## Findings (not fixed)

1. **Whole AC84 remains pending for other IM paths.** Only Slack has new collaboration-specific adapter execution. Lark/DingTalk/Telegram/WeCom general origin and missing-delivery regressions have actual current PASS events, and sources show ordinary event/delivery ownership, but those do not directly test private collaboration body/comment/attachment/record through every failure, stream, direct relay/media consumer. Source tracing and a shared producer-isolation test cannot be reported as all adapters' private transport proof.
2. **Synthetic malformed plugin payload compatibility limit.** `issueIDFromPayload` (`plugin_event_bridge.go:80`) unmarshals one composite issue/comment/issue_id shape. A conflicting `comment:string` causes zero extracted identity; zero-ID events intentionally bypass classification. The first synthetic test therefore reached the local TLS hook. Current `taskEvent/taskFailedFields` emits `issue_id` and `error:string`, with no conflicting comment:string; the final failure-event case invokes that actual emitter. Retain the malformed result as a defensive compatibility limitation, not evidence that the valid current production emitter leaks. Extractor hardening/public semantics would be a root judgment call and is left unchanged.
3. **Proof/transport scope.** Managed Claim/Permit and private storage are actual service writes; final outcomes/proofs are synthetic protocol input. Plugin network is local TLS; Slack sends to a local recording double and persists real output provenance. No external IM, provider/kernel, full Main/installer or other-OS evidence follows.

## Verified product and source scope

`PluginEventDispatcher.run` derives the issue ID after its dynamic feature flag,
then calls `withholdCollaborationEvent` before installation/hook lookup. Valid
shared identity and real lookup error both return before signed hook transport.
Ordinary/no-issue semantics are retained. The new helper performs one lookup for
valid IDs, preserves issue-less events and fails closed on lookup error. The
bridge vocabulary and HMAC transport remain unchanged.

The initially isolated HEAD plus only two new tests failed every private plugin
shape while three other top-level tests passed. This is concrete dependency
evidence: the pre-existing uncommitted plugin gate was necessary. Root then
included the unchanged gate and pure regression as an explicit owned prerequisite;
HEAD plus all four source files passed5 top-level/12 total test pass events. No
other dirty product source was included. The test pair alone must not be committed
as a self-contained behavior slice without that gate prerequisite.

| Complete four-source slice | SHA-256 |
| --- | --- |
| `server/internal/service/collaboration_outbound_boundary_test.go` | `f565b46b2631214ccfc7a335683b2d16411f8872b59205ffe49fb63adde75786` |
| `server/internal/integrations/slack/collaboration_outbound_boundary_test.go` | `41ee89285877f88ec2e43d755ea0def87b7084d02da6f1feb6060c8e67e9c9ab` |
| root-owned `server/internal/service/plugin_event_dispatch.go` | `32d09f67f4834fce4388ef0ac65a898c44848f2f7f14752ac369c5a4f826975e` |
| root-owned `server/internal/service/plugin_event_dispatch_test.go` | `71cfedb70ab225a001749062f3ceebca7314b88fce904e4d23670f1ee0c8fc62` |

The coordinator's complete snapshot is HEAD
`7794f57d969fb583ddf02c830f6d1ab72186595d`, with only those four changes.

## Assertion-level bounded result

`TestCollaborationOutboundPluginDispatcherWithholdsPrivateEvents` creates a real
collaboration issue/comment and encrypted delivered attachment whose actual
bytes are read and compared, plus a nonempty retained result sentinel. The
synchronous real worker receives five valid issue/comment/task payload shapes
and must make no local HTTP call or invocation row. The injected real
`IsCollaborationIssue` query failure also withholds before transport. An ordinary
child with the shared parent reaches the TLS hook with its own ID/description,
no private parent sentinel, valid HMAC and one successful invocation. This is
actual worker-to-transport evidence; the separate canonical bridge regression
owns subscription vocabulary/shape extraction.

`TestCollaborationOutboundPublicWritesStayOffOrdinaryEventBus` drives real
upload/Create/CreateComment/Invite/Transfer. Private retained events exist,
ordinary task rows and ordinary published events do not. An ordinary child
creation then publishes its own ordinary event without private parent data.
`TestCollaborationOutboundManagedResultStaysOffOrdinaryChatEvents` adds actual
managed Claim/Permit, runtime comment/outcome/record and proof acceptance. Private
records remain readable; modern run task_id is null, with no ordinary task or
chat/IM bus event. These cover the actual named writer subset and source ownership;
they are not blanket execution of every collaboration producer.

`TestCollaborationOutboundSlackRejectsSharedAndChildWithoutChannelDelivery`
creates real shared/ordinary-child/chat/live binding/ingested input rows. A source
run ID and ordinary child task lacking immutable channel delivery are ignored,
with one exact delivery read, zero credential-installation reads and zero sends.
A delivery lookup error also fails closed. Once the ordinary child's real delivery
snapshot is added, actual registered EventChatDone subscriber sends exactly the
child answer to the recorded channel/thread and persists message2.0 plus assistant
outbound provenance. No parent sentinel enters that answer. The sender is local;
the adapter and query decisions are production code.

## AC84 canonical correlation

All27 recorded PASS entries/24 distinct identities in
`outbound-boundary-canonical-correlation.json` were independently matched to
actual raw line/event/package/test, raw SHA and current source hashes. Three
machine-boundary function hashes remain equal to the recorded source snapshot
although unrelated portions of that file changed. The record count is provenance,
not full acceptance.

| Parent clause | Current support | Recommendation |
| --- | --- | --- |
| Ordinary child/unauthorized machine minimal references, no private reads and writes | Actual handler machine detail/search/list/table/child, plugin-action refusal, source capture and ordinary execution boundaries | Current exact canonical support; outbound cannot replace permission proof |
| Dedicated current issue/target/resource capability for authorized context | Runtime broad-source/purpose denial, foreign-file/run refusal, frozen dependencies and squad stopped/unfrozen-member context | Current exact canonical support; complete derived owner matrix remains AC96 |
| Plugin valid current payloads withheld | Real DB/TLS worker, lookup-error refusal, ordinary positive and unchanged pure gate regression | Bounded passed |
| Collaboration public/managed writers isolated from ordinary IM producers | Actual named writes and retained private records/no engine event | Bounded passed for executed writer subset |
| Slack private-origin/missing delivery/error and ordinary transport/provenance | Actual SQL+subscriber sender double with durable ordinary output | Bounded passed |
| All remaining IM private direct/failure/stream/relay/media surfaces | General current gate tests and source traces only | **Pending; keep whole AC84 pending** |

Other adapters are distinct consumers, not one common sender. Lark, DingTalk and
WeCom load immutable delivery then classify ordinary input before send. Telegram
resolves exact task delivery before installation/sender. WeCom also has separate
InboxNew and relay/media paths. Current generic cases are supplemental support;
none is relabeled as a collaboration private-payload transport test here.

## Actual execution and retained attempts

Go1.26.6 darwin/arm64, schema737 task-owned `service_s6_collab`, repository
agent CLI guard and serial race flags. No production gate or real provider access.

| Run | Actual result |
| --- | --- |
| Current focused second | 4 top-level/11 test pass events, no fail/skip; package service2.989s, Slack1.878s |
| Current canonical adjacency | 16 top-level/65 test pass events, no fail/skip; package service10.237s, Slack1.850s |
| Initial HEAD+only two tests | Failed missing plugin-gate prerequisite; three other top-level tests passed |
| Final HEAD+complete four sources | 5 top-level/12 test pass events, no fail/skip; wall24.345s |

The first current-tree failed fixture attempt remains retained: malformed
comment:string payload, missing ordinary defaults, wrong modern task_id
expectation and unused Slack import. Only owned tests were corrected. The
private original log is retained mode0600; the checked safe artifact masks exactly
one revoked local callback token, preserving failure events/results. Reviewer
verified its manifest/safe hash and mode without reading private raw content.

Raw current focused SHA is
`72f9765aa28d2e7a64cc442dc9d0bcd1573a3cad852e775c3ecf7b52b63bd46e`;
adjacency SHA is
`47273d184bb3ab79f41ce77be045d2dd3880b2ffc6d57b95e3b04a2a76ea529e`;
complete isolated SHA is
`6d174620f5025f0be116de0dc64c9eae6f2837a14da2b9f33ec83ae3921aaf7e`.
Exact commands, results, PASS/fail lines and source hashes are in the JSON review.

## Verification

- Lint: scoped gofmt over all four sources passes; recorded service/Slack vet passes with empty diagnostics.
- TypeCheck: actual race compilation of current and complete isolated slice passes.
- Tests: bounded focused, canonical adjacency and four-source isolated checks pass; old failures retained separately.
- Reviewer ran reads/gofmt only, no DB/native/build/runtime suites.

## Commit and remaining work

Commit the complete four-source foundation with its scoped evidence via Lore, or
commit the gate/pure regression first and the two tests afterward. Preserve the
existing gate unchanged; exclude unrelated shared source. The complete isolated
pass establishes no other dirty dependency. Keep the full AC84 ledger row open
for exact other-adapter private direct/relay/media evidence and retain malformed
payload compatibility, synthetic proof and external transport limits.

This lane changed only `outbound-final-review.md` and
`outbound-final-review.json`. No mechanical fix, simplification, dependency,
product/spec/ledger/index/environment/gate edit or task archive occurred.
