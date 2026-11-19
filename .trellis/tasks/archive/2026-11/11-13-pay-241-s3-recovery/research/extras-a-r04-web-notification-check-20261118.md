# R04 production-Web notifications — TC-22, 2026-11-18

**Recommendation: `PARTIAL` for current TC-22 after the coordinator’s independent reconciliation; not `PASS`.** The [parent matrix](../../11-13-pay-241-collaboration-acceptance/test-matrix.csv) records historical `PARTIAL`, current `NOT_RUN` at review. R04 completed the Web notification test and all three named subcases on the first attempt. The selected Extras-A run still exited **1**: of six tests, Web notifications and uploads passed, Web revocations failed, and all three Desktop tests had no attempts. Of 18 named subcases, 8 passed, 1 failed, 9 did not run (`maxFailures=1`, retries 0).

## What the Web case proved

- **`mixed_repeat_replay_reconnect`:** The composer selected the same human handler twice and Agent A twice. Preview resolved one A target; the unchanged POST returned 201, one comment/event (seq 16) and one ungrouped, waiting A command. The command matched the original event ID/sequence, comment, target, channel, batch, author and input content. Across complete per-account collaboration notification pages, exactly two row IDs belonged to that event: handler **mentioned**, resource owner **not mentioned**; none for creator or outsider. The handler’s explicit **Mark read** POST returned 200 for that same row; refetch preserved its read timestamp and the link opened the original comment. Navigation alone was not credited.
- **Replay and reconnect:** The creator’s original-key API POST and operation GET returned the identical full receipt before disconnect, after reconnect, and again after a late participant joined. The author’s native socket physically closed with HTTP offline; its distinct replacement had a Cookie-authenticated 101, current-principal check and an issue `subscribe` → `subscribe_ack`. No extra author UI write occurred during reconnect. The case compared comment, command and notification state across each replay, not WebSocket frame counts.
- **`late_join_and_edit_delta`:** The late participant received no original-event backfill; the original two recipient/row-ID pairs stayed fixed. Editing the original comment to mention that participant produced exactly one `mention_added` row (seq 18). A prose-only edit produced none (seq 19); neither edit changed the original Agent command.
- **`target_withdrawal_after_preview`:** After the UI captured a new human+A+B request, the resource owner revoked A’s actual grant (200) before that unchanged request was forwarded. The comment committed (201, seq 21): A returned `blocked/target_unavailable` with no command; B gained exactly one waiting command. Handler was mentioned, owner and late participant were not; creator and outsider got no row. The earlier A command remained identical and waiting. Replaying **this second key** and GETting its receipt changed nothing. The final ledger contains two comments, two commands and six distinct notification IDs **for the four comment events** (2 + 1 + 0 + 3).

## Limits on credit

The saved gate snapshot shows creation on, execution and host execution off (epoch 2). The issue was paused, the managed runtime offline, and issue projections had no groups or runs. The fixture recorded zero ordinary dispatches, Provider launches and simulated permits. A waiting input is neither an Agent inbox notification nor a proven execution result. TC-22 requires correct mention **and subscription** delivery with same-event deduplication: this covers collaboration participant fan-out, not ordinary `issue_subscriber`/inbox behavior or Desktop parity. [Extras-B](recovery-extras-intake-20261118.md) separately owns proven success/failure-or-incomplete/Stop result notices, queued-author withdrawal and live runtime-token revocation. No independent DB/object/process audit, rerun or matrix edit was performed here.

## Evidence (SHA-256)

Relative to `pay241-s3-clients-20261118-x4c5mjep`: `H=harness-extrasA-r04/e2e`, `R=runtime/extrasA-r04`, `V=verification/client-extras-a-r04`. Cited harness bytes match the recorder manifests; before and after are byte-identical.

| File | SHA-256 |
| --- | --- |
| `H/collaboration-recovery-extras.spec.ts` | `9c1030bb0291118e691ebe6412f77dc794c2ac21003420b61ab15fe07423144b` |
| `H/fixtures/collaboration-recovery-extras.ts` (notification case, lines 248–368) | `c86229ef94f3444118affe0a9d0138a4586e1fc73d166ff8e41800bbd984ae1f` |
| `H/fixtures/collaboration-recovery.ts` (transport/socket checks) | `7f125d331adcfa85dab491b3def6ec07bca084238b71eae90f8e5eafcbffb3d1` |
| `R/playwright-report.json` (selected inventory) | `26edd9f44fa925f9446094cba020718fe9846cbe62d729654603c9a5907cb97a` |
| `R/test-results/*55e12*/web-extras-notifications.json` and its attached copy | `58e59957be767ed0823de6360833cc354f53e8f1fdf565b5993bfbf44f68ad81` |
| `R/database-before-owned-stop-r04.json` (recorded gate flags only) | `ef5a863c8901d8d9a4ac8b5ea592a5e4aad98ef003f2db5220f91b65b5e3cf0f` |
| `V/command.json` | `0553b56d3a80147a6e397a5bd78146a576e3117c5ce98ab783c8b263efd26753` |
| `V/result.json` | `7b21e2bb88439ac0046a458c3f497219f58b561d288fd2f1930f719c55692fec` |
| `V/source-before.json` and `V/source-after.json` | `7ef70275a2d619eeee08d5e879a7b098bc9e5500d8831b4c756992066e3afeb1` |
