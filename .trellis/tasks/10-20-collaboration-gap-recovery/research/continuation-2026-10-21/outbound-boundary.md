# AC84 outbound boundary — 2026-10-21 continuation

Existing S6 execution slice, completed across midnight into2026-10-22 Asia/Shanghai. This lane adds tests and evidence only; the coordinator owns the pre-existing14-line plugin gate and38-line pure regression prerequisite. **Current-tree focused race passes4 top-level/11 total; adjacency passes16/65; exact HEAD plus the complete four-source slice passes5/12**, all with zero skips/failures. AC84's full ledger status remains a separate independent decision; this report does not promote it.

## Behavior and actual path

Plugin event delivery is `SubscribePluginEvents` → `PluginEventDispatcher.run` → `IsCollaborationIssue` → installation/hook lookup → signed `InvokeHook` HTTP. The guard runs before installations and transport, withholds shared identity and lookup errors, and leaves ordinary child hooks available. The test executes the real worker delivery synchronously to prove absence after it returns. It uses a local TLS hook, real database installation and original payload representations; it does not merely invoke the withholding predicate.

IM outbound is owned by ordinary task/chat event subscriptions and durable channel routing; there is no collaboration content rehydration consumer. Collaboration human/runtime writes persist `collaboration_event` and `collaboration_run_event`. `CollaborationRealtime` sends metadata-only account/issue/workspace invalidations through a RelayPublisher, not ordinary `EventChatDone`, `EventTaskMessage` or `EventInboxNew`.

| Actual IM consumer | Current ordinary event/delivery ownership | Evidence scope here |
| --- | --- | --- |
| Slack `outbound.go:70,111` | ChatDone; exact `channel_task_delivery`, actual task and owned input's immutable channel-ingested flag before installation/send | New collaboration/child missing-delivery and lookup-failure test; actual subscribed ordinary child sender and durable message provenance positive |
| Lark `outbound.go:285,343,370` | ChatDone/failure; delivery snapshot and input origin; cancellation only clears typing, never sends content | Source traced; existing `TestPatcherSkipsDirectChatTaskOnBoundSession` actual PASS |
| DingTalk `outbound.go:57,83,98` | ChatDone/failure; delivery snapshot and input origin before token/client send | Source traced; existing `TestOutboundFailsClosedWithoutTaskDeliverySnapshot` actual PASS |
| WeCom `outbound.go:138,197,217` | ChatDone; delivery snapshot/input origin before socket or relay. InboxNew is a separate member-notification route | Source traced; existing direct-chat/missing-task tests actual PASS; collaboration writes create no ordinary inbox/task/chat event |
| Telegram `outbound.go:205,972` | TaskMessage/ChatDone/failure/cancel; exact delivery snapshot before streaming/terminal sender | Source traced; existing delivery-lookup failure actual PASS |

These adapters are separate consumers of the same producer/SQL ownership contract, not one common sender. Only Slack has new collaboration-specific executed adapter proof in this slice. General existing adapter PASS records are retained as supplemental evidence, not relabeled as private collaboration execution in each adapter. WeCom relay/media and direct engine `OutboundReplier` flows belong to ordinary channel inbound/outcome ownership; there is no proven collaboration-target → IM bridge.

## Tests written

- `TestCollaborationOutboundPluginDispatcherWithholdsPrivateEvents`: real collaboration creation with encrypted delivered attachment bytes, real comment and nonempty retained record. Five issue/comment/task event shapes carry protected sentinels. Actual dispatcher yields zero HTTP/invocations, including real DB lookup-failure injection. Ordinary child hook reaches the TLS endpoint, contains its own description/ID, contains no protected parent sentinel, verifies HMAC, and records one successful invocation.
- `TestCollaborationOutboundPublicWritesStayOffOrdinaryEventBus`: actual upload/Create/CreateComment/Invite/Transfer persist at least four collaboration events and no ordinary task or bus event. Ordinary child creation emits one ordinary issue-created event with own ID/context and no parent private body/comment/attachment/record.
- `TestCollaborationOutboundManagedResultStaysOffOrdinaryChatEvents`: actual managed Claim/Permit, runtime comment/outcome/typed record and accepted sealed proof retain readable private records. The modern run's `task_id` remainsNULL, there is no ordinary agent task and no ordinary bus event. Server state/capability proof is tested; native process containment is not claimed.
- `TestCollaborationOutboundSlackRejectsSharedAndChildWithoutChannelDelivery`: real source/workspace/agent/shared issue/ordinary child/chat/live Slack binding and ingested ordinary input. A retained source run ID and child task without delivery snapshot receive no send or credential lookup despite a live binding and payload containing all sentinels; injected query error fails closed. Adding the child's immutable delivery allows exactly one actual registered subscriber send to the correct channel/thread, then persists message2.0 and assistant outbound provenance. External Slack sender is an existing local recording double; no Slack network request occurs.

Sentinels cover private description, comment, attachment bytes/name and retained result content. Source fixture reads confirm encrypted attachment contents exist. The standalone Slack file has source-owned retained record/attachment sentinels and tests actual SQL/adapter decisions; the real upload/runtime content boundaries are tested in the service file and canonical runtime/file suites.

## Exact commands/results

From `server/`, focused current-tree command:

```sh
python3 /tmp/multica-gap-recovery-20261021-h6phxwo9/with-db.py service_s6_collab bash ../scripts/go-test-with-agent-cli-guard.sh go test -json -race ./internal/service ./internal/integrations/slack -run '^TestCollaborationOutbound' -p 1 -parallel 1 -count=1 -timeout=30m
```

`outbound-boundary-focused-second-command.json`: exit0, started `2026-10-21T16:06:18.223927Z`, finished `16:06:36.658533Z`. Four top-level elapsed0.26/0.21/0.56/0.19s,11 PASS events, no fail/skip. Raw SHA256 `72f9765aa28d2e7a64cc442dc9d0bcd1573a3cad852e775c3ecf7b52b63bd46e`.

Scoped adjacent command uses the same guard/database/serial flags and selector:

```text
^(TestCollaborationOutbound.*|TestEventBridgeMapsInternalEventsToThePublishedVocabulary|TestEventBridgeFindsTheIssueInEveryPayloadShape|TestEventDispatchWithholdsCollaborationIssues|TestHookRequestIsSignedAndVerifiableByTheReceiver|TestLegacyExecutionCannotInheritCollaborationComments|TestLegacyExecutionSQLBoundaryAndOrdinaryChild|TestCollaborationManagedRuntimeBatchProofAndExactReplay|TestCollaborationSquadRuntimeCommentAuthorityAndMentionBoundary|TestOutbound_.*)$
```

`outbound-boundary-adjacent-first-command.json`: exit0,16 top-level/65 PASS events, no fail/skip. Raw SHA256 `47273d184bb3ab79f41ce77be045d2dd3880b2ffc6d57b95e3b04a2a76ea529e`. It covers canonical bridge/transport, ordinary child/shared comment inheritance, ordinary SQL lifecycle, managed capability and squad boundary plus Slack adjacency.

Static checks, exact output in `outbound-boundary-static-result.json`:

```sh
gofmt -l server/internal/service/collaboration_outbound_boundary_test.go server/internal/integrations/slack/collaboration_outbound_boundary_test.go
# From server/
go vet ./internal/service ./internal/integrations/slack
```

Both exit0, empty stdout/stderr. Guarded race compiles the changed Go files. No frontend/TypeScript source changed; frontend suites are not rerun.

The exclusively allocated schema737 database was `service_s6_collab`, not shared `service_s5` or the task API database. Root received immediate release after tests and now owns it. No later DB test is run by this lane.

## Retained failures and exact commit dependency

The first current-tree attempt is honestly failed, not green evidence:

- Four initial event subtests passed, then malformed synthetic task-failed payload `comment:string` caused the whole legacy issue-ID shape unmarshal to fail and sent the private fixture to the **local** TLS hook. Production `taskEvent/taskFailedFields` uses `error:string` and no top-level `comment:string`; the final case calls that canonical emitter instead of inventing the conflicting event shape.
- The ordinary child positive lacked explicit default status/priority; it now supplies actual required `todo/none`.
- The managed fixture assumed an ordinary engine task; current supervisor uses aNULL task_id and no ordinary queue row. Final test asserts absence, not a fabricated row.
- Slack initially contained an unused pgx import, removed locally.

These are owned test repairs, no product edit or removed authority/private-content assertion. `outbound-boundary-focused-first.jsonl` retains all failures. Its first assertion printed one already-revoked synthetic callback token; an exact private raw copy is retained mode0600 under `/tmp/.../outbound-boundary-focused-first-private.jsonl`, and the repository safe copy masks only that token. `outbound-boundary-focused-first-redaction.json` records original/safe hashes and count1; event/results are unchanged. Later tests never print hook bodies/credentials.

Root's first **HEAD + only two new test files** run failed all five private plugin cases while the other three top-level tests passed. This proves the private outbound guard was still shared dirty foundation, not a failure in current-tree behavior. `outbound-isolated-head-race-result.json` records3 top-level passes and the missing-gate failures, raw `1af7507ba20b4481b18c513e0caed412bfe0252cc864d8cce359f1015f179fad`.

Exact source diff inspection found only `plugin_event_dispatch.go`: three-line guard after issue-ID extraction plus11-line helper/comment; no bridge/hook change. Root explicitly owns inclusion of that unchanged14-line diff and the unchanged38-line predicate regression in `plugin_event_dispatch_test.go`. No product/helper file was modified by this lane.

Root's isolated **HEAD + complete four-source slice** then passed5 top-level/12 total, no fail/skip,24.345s. `outbound-foundation-isolated-head-source.json` records exact HEAD `7794f57d969fb583ddf02c830f6d1ab72186595d`, only changed sources and no other shared dirty code. Raw SHA `6d174620f5025f0be116de0dc64c9eae6f2837a14da2b9f33ec83ae3921aaf7e`. This complete slice, not the two-file pair alone, is independently executable.

| Complete source slice | Current SHA256 |
| --- | --- |
| `server/internal/service/collaboration_outbound_boundary_test.go` | `f565b46b2631214ccfc7a335683b2d16411f8872b59205ffe49fb63adde75786` |
| `server/internal/integrations/slack/collaboration_outbound_boundary_test.go` | `41ee89285877f88ec2e43d755ea0def87b7084d02da6f1feb6060c8e67e9c9ab` |
| root-owned existing `server/internal/service/plugin_event_dispatch.go` | `32d09f67f4834fce4388ef0ac65a898c44848f2f7f14752ac369c5a4f826975e` |
| root-owned existing `server/internal/service/plugin_event_dispatch_test.go` | `71cfedb70ab225a001749062f3ceebca7314b88fce904e4d23670f1ee0c8fc62` |

## AC84 exact canonical correlation

[outbound-boundary-canonical-correlation.json](outbound-boundary-canonical-correlation.json) includes27 completed PASS records covering24 distinct identities; no missing collected identities. Each points to its actual artifact/line/source and SHA, not test-name inference. Canonical function bodies remain equal to the retained source manifest; the machine boundary file changed outside these functions, so the three exact function hashes/equality checks are retained explicitly.

| AC84 clause | Current canonical and new supporting evidence | Remaining assessment |
| --- | --- | --- |
| Ordinary children and unauthorized/unknown machines see unopenable minimal references with no private fields | Handler `TestLegacyIssueDetailDoesNotExposeCollaborationToMachine`, `TestCollaborationMachineSearchAndListKeepOnlyReferences`, `TestCollaborationMachineReferencesDoNotRevealStatusOrPrivateOrdering`, `TestSourceCollaborationTableAndChildMachineBoundary`; service own-child capture and source capture denial | Exact references/list private-filter/unknown-principal boundaries supported; new IM sender is not a replacement for these tests |
| Writes denied; ordinary child does not inherit private parent comments/capture/grants | `TestLegacyExecutionCannotInheritCollaborationComments`, `TestLegacyExecutionSQLBoundaryAndOrdinaryChild`, `TestSourceContextOrdinaryChildRetainsOwnCaptureAndWorkspaceScope`, `TestSourceContextCollaborationBuildersDenyBeforeProjection`, `TestSourceContextCollaborationDirectWritersCannotUsePrebuiltCapture`; existing plugin Action refusal | Ordinary/shared separation supported at assigned backend layers |
| Plugin/IM outbound contains no protected body/comment/file/record | New real DB/TLS plugin, no ordinary collaboration event producer, real managed result/no engine event, and actual Slack consumer/durable positive; traced remaining adapter common gates/general current tests | New collaboration-specific Slack execution only. Full adapters/direct relay/media remain separately scoped; do not label every adapter newly verified |
| Dedicated current issue/target/resource capability is required for authorized/derived context; ordinary JWT/PAT/task or known mention cannot grant it | `TestCollaborationRuntimeManagedBoundaryRejectsBroadCredentials`, `TestCollaborationRuntimeNarrowHandlersNeverFallBackToOtherPurposes`, `TestCollaborationRuntimePreparationScopeAndRevocation`, `TestCollaborationRuntimePreparationRejectsAnotherLiveRunAndIssueFile`, frozen-file tests and squad runtime/continuation context | Current explicit capability data and denial supported; AC96 still owns the complete derived/all-owner matrix |
| Humans retain original rules; unknown identities are not treated as humans | Machine search suite uses JWT/PAT positive controls and missing/future principal negatives; `TestCollaborationCreateRejectsUntrustedPrincipalBeforeDependencies` | Explicit source controls supported |

The evidence materially closes the missing executed plugin/IM boundary, but this implementation lane makes **no automatic full AC84 promotion**. Independent P3 review should decide whether the shared producer isolation plus actual Slack and current adapter-specific gate tests suffice for the whole outward contract, or request exact remaining WeCom/media/other adapter private-payload paths. AC96 remains open independently.

## Risks and limits

- Malformed synthetic event payload can erase the legacy issue-ID extractor's identity, as retained first failure demonstrates. No real event producer emits that conflicting shape; future product hardening is a separate root judgment call and is not silently added here.
- No external Slack/IM recipient receives a message. Plugin HTTP is local TLS and Slack sender is the canonical test double; real durable output rows prove local dispatch/provenance.
- Managed server proof/capability tests use synthetic kernel descriptors; actual native process containment/provider-account execution is not inferred.
- Other IM consumers were traced and their current existing origin/no-delivery tests correlated, not newly run private collaboration integrations.
- Full monolithic checks, frontend/native UI/packaging and real provider/account/quota smoke are not rerun for this Go test-only slice.
- New sources plus root-owned pre-existing gate/regression must be staged together or in explicit prerequisite order. The first two-file-only isolated failure remains evidence.

Changed files by this lane: two new test files, this report and `outbound-boundary-*` command/result/raw/correlation artifacts. Simplification: none; no dependencies/product edits. Previous offline review remains preserved. Matrix/catalog/progress/spec/index/runtime/production gates are not changed by this lane. Root owns final review/Lore commit and parent remains in_progress.

Final validation passed: task context15/16 manifests, repository `git diff --check`,
current source equality with root's isolated four-file snapshot, and27 canonical
PASS records/24 distinct identities. Exact commands/results and artifact hashes
are in `outbound-boundary-final-result.json`. No further DB run occurred after
exclusive slot release.
