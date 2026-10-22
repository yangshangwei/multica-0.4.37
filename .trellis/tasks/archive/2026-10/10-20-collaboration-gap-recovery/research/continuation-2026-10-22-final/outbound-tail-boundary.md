# AC84 outbound tail — bounded execution boundary

Active task: `10-20-collaboration-gap-recovery`. This is the existing S6
acceptance slice, not a new task or plan. The root owns ledger/spec changes and
commits. Production creation/execution gates remain closed.

The remaining evidence gap is coverage of the existing Lark/Telegram cancellation
subscribers, Slack typing lifecycle subscriber, and attachment-bearing/file-only
completion payloads in the three text-only IM adapters. The independently reviewed
WeCom relay/media fix and its ordinary fixture repairs remain root-owned.

Owned edits are the existing Lark, DingTalk and Telegram
`collaboration_outbound_boundary_test.go` files, a focused new Slack typing
boundary test, and task evidence files with the `outbound-tail-` prefix. The
assigned WeCom boundary test is already complete and needs no change. No
production interfaces, storage, transports, fixtures outside these files, ledger,
spec, dependencies, gate configuration or provider accounts are modified.

Reuse the current real PostgreSQL private-source fixtures and recorded/local
transport fixtures. Shared run IDs lack ordinary channel delivery; an ordinary
child sharing a live chat binding lacks authority until its exact immutable
delivery exists. Assert the actual refusing stage before installation credentials
or wire operations. Ordinary positives must cross that stage and retain their own
content. Cancellation/typing cleanup may only affect tracked local state; it must
not convert protected event contents into a reply.

Source inspection found no outbound binary/media route in Lark/DingTalk/Telegram:
their completion payload is text-only; Lark and DingTalk media resolvers are
inbound download/upload services. Telegram supports sendMessage/editMessageText.
The normalized `channel.OutboundMessage` deliberately has no media fields. Test
attachment-bearing map payloads at the existing boundary, file-only drops and
ordinary text positives. Do not invent nonexistent storage callbacks to obtain a
zero counter, or claim successful outbound file support from these tests. The
root confirmed AC84 concerns withholding on existing exits and does not require
adding media support.

Verification will save exact commands, raw Go JSONL, exit/count/skip results and
source hashes before/after each run. DB-backed runs use only task-owned
`service_s5` after coordination with the root, with the agent-CLI guard,
`-race -p 1 -parallel 1 -count=1 -timeout=30m`. Narrow passing checks precede adjacent
package regression checks and `go vet`/`gofmt`. First failures remain recorded and
are diagnosed without broadening product scope. No external IM/provider access.

The lane cannot promote full AC84 by itself. Root must correlate current canonical
agent/machine minimal-reference, write-denial, dedicated-identity read and unknown
identity evidence; derived authorization remains AC96-owned.
