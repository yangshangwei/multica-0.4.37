# AC84 IM outbound tail — current execution report

Active task remains `10-20-collaboration-gap-recovery`, S6. No new task, replanning,
ledger/spec promotion, commit, provider account or production gate change. The
root owns the independently reviewed WeCom relay fix and its ordinary fixtures.

## Decision and changed files

The review's remaining IM clauses now have current tests on the actual existing
exits. Full AC84 still needs root correlation of its canonical non-IM clauses.

- `server/internal/integrations/lark/collaboration_outbound_boundary_test.go`:
  retain the real encrypted attachment ID; add attachment-bearing/file-only
  completion boundary plus actual local HTTP cancellation/typing cleanup.
- `server/internal/integrations/dingtalk/collaboration_outbound_boundary_test.go`:
  retain the real encrypted attachment ID; add attachment-bearing/file-only
  boundary with a counter over every actual HTTP transport call.
- `server/internal/integrations/telegram/collaboration_outbound_boundary_test.go`:
  extend the existing final/failure/stream test with attachment/file-only/cancel
  events and exact ordinary stream cancellation without a replacement send.
- `server/internal/integrations/slack/collaboration_typing_boundary_test.go`:
  actual Slack API client over loopback, all three lifecycle subscriptions,
  untracked private identities, unrelated tracked ordinary state, exact one-time
  removal and ordinary reaction positives.
- `research/continuation-2026-10-22-final/outbound-tail-*`: bounded change record,
  before-hash/snapshot pointer, replayable receipt runner, raw JSONL, commands,
  results, source hashes and this report.

The owned WeCom boundary file is unchanged (`cbfeb9d0…d28929a`). Its existing
direct/relay/media/fault tests are rerun here; no other agent's files are edited.
No production code was added. Reuse existing private-source/database fixtures,
`newDingtalkSendServer`, `roundTripFunc`, fake Lark client and Slack configuration;
there is no new shared abstraction or dependency.

## Actual assertions and source scope

The Lark/DingTalk/Telegram fixtures retain a real collaboration source body and
comment, encrypted delivered attachment, source-only prepared run and stored run
event, alongside an ordinary child with its own real channel-ingested input.
The shared run and child lack `channel_task_delivery` while a live chat binding
exists. A text-bearing protected event is refused at the exact immutable delivery
lookup before installation credentials or wire work. Delivery faults stay errors.
The ordinary child becomes deliverable only after its exact delivery is written.

| Existing path | Executed additional assertions |
| --- | --- |
| Lark attachment fields | Shared run/child × text-plus-attachment/file-only each lookup delivery exactly once and use no installation, credentials, text/card/patch or resource-download calls. Faults propagate. Exact ordinary text sends once; file-only reaches authorized parsing then emits nothing, with no resource download. |
| Lark cancelled | Untracked shared run/child events touch no queries/credentials/HTTP. Actual ordinary Typing ADD crosses local token/reaction HTTP. Cancellation with private extras removes only the tracked ordinary message/reaction once, even when delivery lookup would fail, and no private field is sent. |
| DingTalk attachment fields | Text-plus-attachment rejects at delivery; file-only is dropped by the existing empty-content parser **before** delivery. Zero installation/decrypt and zero requests across the entire HTTP transport. Ordinary positive sends exactly token mint + sampleMarkdown own text; unknown attachment fields never enter that body. |
| Telegram attachment/cancel | Text-bearing protected events reject at delivery. File-only drops before enqueue/target resolution; cancellation clears local streams and performs no lookup/send. Actual ordinary streamed send retains message99; cancellation releases its exact scheduler reference and removes stream state without editing/replacing the visible partial. Its own attachment-bearing completion/failure then send normally; file-only sends nothing. |
| Slack typing subscriber | Both synthetic private identities × chat:done/task:failed/task:cancelled do no installation/decrypt/HTTP work without tracked state and preserve an unrelated tracked reaction. All three ordinary endings each remove only their tracked reaction once via real loopback reactions.add/remove requests. Private body/comment/file/record extras never become wire values. |

Slack typing identities are intentionally synthetic: this manager has no
task/business-content query surface. Its current stored source/delivery
classification is verified by the existing Slack outbound boundary test in both
passing runs, not inferred from the typing fixture.

### Corrected media-scope inference

There is no existing outbound binary/media route to implement in these three
adapters. `channel/message.go:196` explicitly models text/thread/reply only;
`protocol/messages.go:247`'s completion payload has no media field. Lark's
`PatcherQueries` (`outbound.go:144`) has no attachment/storage methods and
`sendChatReply` (`:435`) chooses text or markdown card; `APIClient`'s
DownloadMessageResource is an **inbound** fetch, exercised here by a fail-on-use
probe. DingTalk `outboundQueries` (`:21`) is task/delivery/installation only and
`outbound_send.go:101` constructs sampleMarkdown text. Telegram `outboundQueries`
(`:68`) is delivery/installation only; `sender.go:64` and `api.go:261` call
sendMessage, while streaming uses editMessageText. No sendPhoto/sendDocument or
outbound attachment/storage reader exists in these packages.

Lark `media_ingest.go` and DingTalk `media.go` are inbound download plus storage
upload. `CapAttachment` describes inbound capability, not an implemented outbound
file API. Therefore no fake outbound storage callback or impossible ordinary
binary-send positive was invented. The root explicitly confirmed the scope:
AC84 requires withholding private content on existing exits, not new media
support. Attachment-bearing map extras here prove no bypass of those exits;
they do not prove successful file output or authorized private downloads.

## Verification receipts

All Go commands execute from `server/` through the verified launcher
`python3 /tmp/multica-gap-recovery-20261021-h6phxwo9/with-db.py service_s6_other`.
The actual task database reports `737_collaboration_runtime_preparation`.
`psql` was absent from this sub-agent's PATH; the existing executable
`/opt/homebrew/opt/libpq/bin/psql` supplied the read-only prerequisite. No package
installation or environment mutation was needed.

The replayable capture command is:

```sh
python3 .trellis/tasks/10-20-collaboration-gap-recovery/research/continuation-2026-10-22-final/outbound-tail-run.py narrow-final -- \
  bash ../scripts/go-test-with-agent-cli-guard.sh go test -json -race \
  ./internal/integrations/lark ./internal/integrations/dingtalk \
  ./internal/integrations/telegram ./internal/integrations/wecom \
  ./internal/integrations/slack -run '^TestCollaborationOutbound' \
  -p 1 -parallel 1 -count=1 -timeout=30m
```

`outbound-tail-*-command.json` contains the exact expanded command/cwd/HEAD and
source hashes; `.jsonl` is unmodified Go output, `.stderr` preserves errors and
`*-result.json` contains exact test identities, counts, exit, timings, before/after
hashes and raw-output hashes. `GOTOOLCHAIN=go1.26.6` and the agent CLI guard apply.

| Run | Exit | Top-level / all tests | Source and interpretation |
| --- | --- | --- | --- |
| narrow-first | 1 | 11/67 passed, no failed tests | Four IM packages pass. New Slack test has one unused `pgtype` import and its package does not build; retained honestly. No source drift. |
| narrow-final | 0 | 13/77 passed, 0 skips | All five packages pass. Only concurrent root addition `service/collaboration_catalogue_matrix_test.go` changes the broad manifest; no integration source changes. |
| adjacent | 0 | 238/414 passed; 3 Redis skips | `-run '(Patcher\|Typing\|Outbound\|Sender\|Relay\|Media)'`, five packages, same serial race flags. No source drift. Includes **all13/77 collaboration boundary identities on exact current source** and is the authoritative lane receipt. |
| redis-final | 0 | 3/3 passed, 0 skips | The exact three omitted Redis cases, same task DB/race/serial flags and confirmed loopback Redis. Separate supplemental evidence; no source drift,8.3s. |
| isolated-final | 0 | 33/97 passed, 0 skips | Base `cae15dd72` archive plus exactly9 outbound files, five packages with Redis and prescribed relay/two-replica filter,52.2s. No isolated-source or root-copied-file drift. |
| vet | 0 | five packages | `go vet -p 1` for the same package list. No source drift, empty output. |

`gofmt -l` is empty for the five boundary test files; focused `git diff --check`
is clean. The adjacent stdout/stderr hashes and current integration/owned source
hashes match. The root's later `collaboration_catalogue_matrix_test.go` edit is
reported separately in `outbound-tail-summary.json`; it is outside this lane and
was not changed during the authoritative adjacent run.
All stdout/stderr data is local/synthetic; no environment secret is recorded.

The 3 Redis skips are the existing WeCom
`TestRelay_ARetriedReplyKeepsItsPlaceInLine`,
`TestRelay_AReplyNoReplicaCouldSendIsCountedOnce` and
`TestRelay_ADeliveredReplyIsCountedOnceAndNotAlsoLost`. These are not new acceptance
passes. They were subsequently rerun explicitly and passed3/3, with
`REDIS_TEST_URL=redis://127.0.0.1:57244/0`, following the root's follow-up dispatch.
Before that run, direct Redis `PING` returned `+PONG` and `lsof` confirmed the
existing Docker-owned loopback listener; `outbound-tail-redis-preflight.json`
records both. The exact run is:

```sh
python3 .trellis/tasks/10-20-collaboration-gap-recovery/research/continuation-2026-10-22-final/outbound-tail-run.py redis-final -- \
  env REDIS_TEST_URL=redis://127.0.0.1:57244/0 \
  bash ../scripts/go-test-with-agent-cli-guard.sh go test -json -race \
  ./internal/integrations/wecom \
  -run '^TestRelay_(ARetriedReplyKeepsItsPlaceInLine|AReplyNoReplicaCouldSendIsCountedOnce|ADeliveredReplyIsCountedOnceAndNotAlsoLost)$' \
  -p 1 -parallel 1 -count=1 -timeout=30m
```

`outbound-tail-redis-final-{command,result}.json`, `.jsonl` and `.stderr` retain
the original result and hashes. This supplement closes the omissions without
rewriting the adjacent suite's skipped counts. It edits no WeCom source or fixture.
The root's reviewed isolated WeCom Redis receipt remains separate.

### Independent nine-file commit isolation

Following the root's final dispatch, `outbound-tail-isolated-run.py` creates a
fresh external `git archive cae15dd72ee568f28ce29b19f0c76765b72f26cd` and overlays
only the three extended boundary files, unchanged WeCom boundary, new Slack
typing test and the four root-owned WeCom relay/ordinary-fixture files. No
other working-tree file or task helper is copied. The run compiles on that exact
base without any missing-WIP helper. The independent archive is retained outside
the checkout; its directory, archive hash and **all nine copied hashes** are in
`outbound-tail-isolated-final-command.json` and `*-result.json`.

The exact isolated filter is
`^Test(CollaborationOutbound|TwoReplicas|Relay_|DeliverRelayed|DrainDelivers|RelayedInbox)`
over all five IM packages, with the same guarded serial race flags and
`REDIS_TEST_URL=redis://127.0.0.1:57244/0` on `service_s6_other`. Redis PING and
listener are rechecked before the run. Result:33 top-level/97 tests passed,
0 failed/0 skipped,52.249s. Entire archived Go source is hash-stable, and all
nine root files are unchanged before/after and match the copied artifacts.
Raw JSONL SHA-256 is
`e80db4c95c7a0682db22ad9e715b783c2bd6541c1252d74c1f28b1f8f4be7281`;
stderr is empty. This is the independent commit-slice evidence; broader adjacent
and Redis receipts retain their original separate counts. No root product source,
ledger/spec or git state was mutated by the isolation step.

## Risks and handoff

The lane closes the review's additional IM cancellation/typing/unknown-attachment
clauses. Root still must cite/rerun current canonical agent/machine minimal-parent
reference, all write denials, dedicated-run current-authority reads and unknown
identity not treated as human. Derived authorization/catalogue stays with AC96.
This report alone must not mark full AC84 passed.

Retain the earlier WeCom review's low-scope observations (relay ordinary message
attachment identity, retry-exhaustion observability, inbox producer boundary).
They were not changed here. No new product defect was observed in this lane.
External IM delivery, provider accounts/quota, full native installed clients,
other OS/runtime containment and production rollout are untested/separately
authorized. Neither child nor parent task may be archived on these results.
