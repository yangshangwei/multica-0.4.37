# Independent check: WeCom relay receiver fix and AC84 IM outbound

Checker: `outbound-check` (read-only; no product/test/spec/ledger/env mutation, no DB tests run by this checker).
HEAD `cae15dd72ee568f28ce29b19f0c76765b72f26cd`. Machine-readable twin: `outbound-im-current-review.json`.

## Decision

- Minimal WeCom relay fix: **accept**. No blocking defect found in `wecom/relay_outbound.go`.
- Fixture corrections in `outbound_test.go` / `outbound_two_replica_db_test.go`: **accept**. They supply provenance; receiver was not weakened.
- AC84: **remain partial**. The WeCom relay/media clause is now executed; other clauses still lack current collaboration-specific proof (listed below).

## Source reviewed (current hashes match all evidence manifests)

| File | SHA-256 |
| --- | --- |
| `wecom/relay_outbound.go` | `c0b2be0d…f6d1d6` (HEAD `7633bbd8…`) |
| `wecom/outbound_test.go` | `e575bf3a…596ad` |
| `wecom/outbound_two_replica_db_test.go` | `2ff230b4…71d672` |
| `{lark,dingtalk,telegram,wecom}/collaboration_outbound_boundary_test.go` | `1ad4f5a7…`, `9079b5c2…`, `a2b2c233…`, `cbfeb9d0…` (identical to resume2-red manifest) |
| `{lark,dingtalk,telegram,wecom}/outbound.go`, `wecom/outbound_media.go` | unchanged from `outbound-im-boundary-20261022.json` before-hashes |
| `wecom/relay_ordering_db_test.go` (8th file, test-only) | `1d7f3ccb…dedf0`; adds `workspaceID: turn.workspaceID` to `seedSiblingTurn` only |

The eighth file is the same fixture correction applied to the Redis ordering sibling turn. Without it, the receiver correctly ends the sibling frame (its workspace is empty). The assertions in that file are unchanged.

## Fix correctness (`deliverRelayed`, reply kind only)

The check order is correct, and every check runs before `senders.get`, `sendTextCtx` or `deliverAttachmentsByID`:

1. `o.q == nil` → terminal. This is an interface field. Production `NewOutbound` always passes `*db.Queries`. A typed-nil pointer is not guarded, but no caller constructs one.
2. Malformed/missing `TaskID` → terminal.
3. `GetChannelTaskDelivery`: exact `TaskID`, `ChannelType==wecom`, `InstallationID`, `ChannelChatID`, `aibotChatTypeFromChannel(ChatType)`. Any mismatch → terminal. A shared collaboration run ID has no delivery row, so it returns `ErrNoRows` → terminal.
4. Malformed session/workspace → terminal. This now requires `WorkspaceID`. The production publisher `service/task.go:7112` sets `WorkspaceID: workspaceID` on `EventChatDone`, and `outbound.go` copies `e.WorkspaceID` into the frame. Real frames are therefore not newly dropped. Only the old test `chatDoneFor` sent `""`, and the root corrected that fixture.
5. `GetAgentTask`: exact ID and `ChatSessionID`.
6. `engine.TaskInputIsChannelIngested` (same gate as the direct path): a Web-typed input → terminal.
7. `GetChannelInstallation`: exact ID, `WorkspaceID` equals frame workspace, `status=active`.

`relayReadFailure`: `pgx.ErrNoRows` → `outcomeDone`. Any other error, including ctx cancellation during drain, → `outcomeProvablyNotSent` with a fixed stage-only WARN (no `err`, no IDs).

Semantics check: `outcomeProvablyNotSent` → `perform` calls `seen.forget` plus `dedupe.Release` and returns false. `step` then re-offers on the bounded `retryPlan`, with the same `eventID`/claim key. Nothing has reached the wire, so release cannot create a duplicate. Not all DB errors are terminal; only absent rows and immutable/authority mismatches are. Inbox frames bypass the new block, so their behavior is unchanged.

The new logging does not leak: the fault-stage log carries only `stage`. The downstream `droppedFor`/`unconfirmedFor` are pre-existing and reached only after authorization.

## Test-assertion check (actual clauses, not names)

- `…ReceiverRejectsPrivateAndUnknownDeliveryBeforeWire` (shared run / child without delivery / delivery fault): `deliveryReads==1`, zero installation/attachment/object reads, and zero `cmdSendMsg`+`cmdUploadMediaInit` frames.
- `…RejectsTamperedOrInactiveDelivery` (13 scenarios: installation with a live foreign socket, chat, chat_type, session, workspace, missing/malformed task/session/workspace, inactive installation, other_channel, web_input, missing_queries): `perform` returns true (terminal, no retry), with no media/storage/wire access. `deliveryReads==1` is required except for pre-lookup refusals.
- `…ReadFaultRetainsRetryWithoutPrivateWire` (delivery/task/origin/installation): the fault error string embeds the private sentinels. The test asserts `perform` returns false, no wire/media, a nonempty log containing no `IM_BOUNDARY_PRIVATE_`, recovery on the same item, and a replay of the same item with no second send.
- `…RelayOrdinaryChildUsesItsExactDeliveryAndOwnMedia`: exactly one markdown plus one media upload over the real recorded sender, and one read each of delivery, installation and attachment.
- Direct WeCom: shared run/child × final/file-only reach no installation/attachment/object/wire. A delivery-lookup error propagates. The ordinary positive sends one text plus one media, with no private sentinel in any frame.
- Ordinary relay fixtures (`relayWireFixture`): explicit test-owned delivery, task with `ChatSessionID`, an active installation in the frame workspace, and `askedOverWecom()`. Dedupe, lease, drain and ctx assertions are unchanged.

## Evidence (raw hashes verified; failures retained, no union)

| Run | Exit | Counts | Note |
| --- | --- | --- | --- |
| `continuation-2026-10-22/outbound-im-resume2-red` | 1 | 7 top-level/43 tests; 19 failed + package (20 fail events) | Pre-fix; source stable at `238d4bb39` |
| `resume/outbound-current-red` | 1 | WeCom relay only, all failed | Pre-fix at `cae15dd72`; stdout `6d03ad80…` |
| `resume/outbound-current-green` | 1 | 7/43 passed; ReadFault ×4 failed | First fix treated DB faults as terminal; refined |
| `resume/outbound-current-green2` | 0 | 8/48 | Manifest predates the fixture edits; fix source equals current |
| `resume/outbound-current-adjacent-first` | 1 | 868/1339 passed; 7 old relay tests failed | `source_changed_during_run` = the 2 fixture files; failures were old fixtures lacking provenance |
| `resume/outbound-current-adjacent-final` | 0 | 875/1346, 3 Redis skips | stdout `816da4df…`. 194-file manifest; the only drift since is test-only `relay_ordering_db_test.go`, a file whose tests this run skipped |
| `continuation-2026-10-22/outbound-current-isolated-head` | 0 | 25/65, 3 Redis skips | 06:03Z; exact HEAD + 7 files; jsonl `d4f1cf37…`; 46.3s. This is the run cited in the root message |
| `resume/outbound-current-vet` | 0 | four packages | Manifest predates the test-only 8th-file edit. The checker re-ran `go vet` on all four packages on current source: exit 0; `gofmt -l` on the 8 files is clean |
| `resume/outbound-current-redis` | **1** | 2/3; `TestRelay_ARetriedReplyKeepsItsPlaceInLine` failed (`relay_ordering_db_test.go:228`: chat received only `答案一`) | Retained failure, not unioned; caused by the sibling fixture missing its workspace; stdout `397bba5d…` |
| `resume/outbound-current-redis-final` | 0 | 3/3 | 06:20Z with `REDIS_TEST_URL`, after the sibling fix; stdout `e0820955…`; no current drift |
| `resume/outbound-current-isolated-final` | 0 | 28/68, **0 skips** | 06:22Z; exact HEAD + 8 files with Redis; stdout `c0f4fc5b…`; all 8 copied hashes equal current. This is the authoritative isolated run |

All runs record `real_provider_account_access:false`, `production_gate_change:false` and no external IM. Transport is fake/local and the DB is task-owned.

## Open findings (not self-fixed; root decision)

1. **Low, untested scope, not a collaboration-private leak.** `deliverAttachmentsByID` still trusts `frame.MessageID`. `ListAttachmentsByChatMessage` filters only `chat_message_id AND workspace_id` (`attachment.sql:166`) and is not bound to the validated task/session. A stale or tampered frame with a valid task but another session's message ID in the same workspace could post that session's ordinary chat attachments into this delivery's room. Collaboration private files live in `collaboration_attachment` and are unreachable here. The direct path trusts the publisher's own `msg.ID`, so this exposure exists only on the relay path. Suggested follow-up: require the attachment rows' `task_id`/`chat_session_id` to match the validated task/session, or check the chat message's session, plus a negative test. Otherwise, document that frames are trusted for content and message identity.
2. **Low, observability.** Read faults that exhaust `retryPlan` log "giving up … after retries". The publisher's watcher later counts `drop_no_connection`, which mislabels a DB outage. There is no correctness impact.
3. **Coverage note.** Redis `Release` and re-claim after a read fault is not asserted in the collaboration test (it uses dedupe=nil). The same release branch is exercised by `TestRelay_LeaseMoveMidFlightIsRescuedByRetry` and `TestRelay_DedupeOutageWithholdsRatherThanDuplicates` in the isolated run. Acceptable.
4. **Inbox relay has no receiver recheck.** Collaboration content is withheld at the producer (`TestCollaborationOutboundPublicWritesStayOffOrdinaryEventBus` in the matrix). There is no relay-level collaboration inbox negative. This is not a regression from this change.

## AC84 recommendation: partial

Now executed: plugin TLS (earlier), Slack, and this lane's Lark final_text/markdown/failure_card, DingTalk final/failure, Telegram final/failure/stream, and WeCom direct/file-only/relay/tamper/fault/media, each with ordinary positives.

Still lacking current collaboration-specific proof:

- `EventTaskCancelled` subscribers (Lark `outbound.go:288`, Telegram `:209`) are not among the boundary kinds.
- Lark/DingTalk/Telegram outbound media/file paths are not covered, unlike WeCom.
- The Slack typing-indicator subscriber (`typing_indicator.go:253-255`) is not covered.
- The non-IM AC84 clauses (agent/machine minimal reference, write denial, dedicated-identity reads, unknown identity not treated as human) are listed by test identity in the matrix. This review did not see current-run evidence for them, and the root must cite or rerun.
- The derived authorization matrix stays with AC96.

Do not promote AC84 to passed on this lane alone.
