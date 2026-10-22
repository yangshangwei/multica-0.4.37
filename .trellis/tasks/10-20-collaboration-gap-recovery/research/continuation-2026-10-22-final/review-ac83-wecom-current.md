# Independent current AC83 and WeCom commit-scope review

Active task: `.trellis/tasks/10-20-collaboration-gap-recovery`. Reviewer:
`/root/evidence_check` (`trellis-check`), read-only except these two `review-*`
artifacts. No task creation, replanning, ledger/spec/product/test edits, DB race,
client launch, environment action, provider account access or commit.

## Decision

**Promote full AC83**, retaining the explicit limitations below. The current
notification report is supported by the actual source and raw receipts.
**Accept the minimal WeCom relay production file plus three ordinary fixture
files as an independent slice**, after root's intended current adjacency rerun.
**Hold full AC84** until current expanded transport and non-IM/derived evidence
is independently correlated. No production gate/account scope changes follow.

## Integrity and actual evidence

- Paired Web/Electron raw: `research/continuation-2026-10-22-resume/current-lane-notification-clients.stdout`,
  SHA256 `506299b1a544078c73af21a0425d4349d1ffa11c71ea8c708ba865522c0066c8`.
  Independently parsed 8/8, retry 0, zero skip/unexpected/flaky/error. The two AC83
  cases are `collaboration-offline-result-notifications.spec.ts:15`, Web/Electron.
- Embedded notification attachments exactly equal current saved files: Web SHA
  `90f2007d68af17fa867d1086a579b262f887b9d0482c16090c9e84630ae705f9`,
  Electron SHA `d165c59ce8dc9337b547d274ab302b52f3e6e6664047ce09e3ab6ae2c9e2c85a`.
  All seven source hashes equal current, including spec `86a2efd01f7d4e67bc83fb9452fb7fb1c22d7d8542045920aedc3f9166c8f117`
  and fixture `ab8c09d4600e354231e870721665798cbedd0cb584350f03cb96ebaeb3ac615c`.
- Canonical raw: `research/continuation-2026-10-22-resume/notification-current-canonical.stdout`,
  SHA `87c23981b2e947511da8b2e10e5d88bf4b00178f942e719d07d3e7847aac0d80`.
  Reparsed 15 top-level/31 total pass, zero fail/skip, serial race under service_s5.
  All five recorded test sources equal current. Actual commands/hashes in JSON.
- Invite/assignment/transfer/comment clauses use unchanged current
  `offline-handoff-coordinator-rerun` 2/2 and source preflight: spec
  `c70c973f61c8b0f09b64da4c5c23a0eb34c120dd7525bc89d750a2de934cdb0e`,
  fixture `733fb4d46b603752c518ac316de0fa1258623b9b24cb7902b8be9d415be2fd35`.
- Authoritative WeCom isolated proof: `research/continuation-2026-10-22-resume/outbound-current-isolated-final.stdout`,
  SHA `c0f4fc5b5dac7838d6b11d351b4044220be15250ccfb603286af1355f96571c5`.
  Reparsed 28 top-level/68 total pass, zero fail/skip, Redis supplement included,
  exact HEAD+8-file snapshot. The production+ordinary4 hashes remain identical.
  Expanded Lark/DingTalk/Telegram test drift is expected sibling work and is
  **not** evidence against the minimal fix. That new test code needs its own run.

## Full AC83 clauses

1. The external recipient has zero workspaces/source membership, closes the
   client before event creation, and performs one fresh real login after six
   events. Both clients see exactly one information request, return, acceptance,
   AI completed, failed and confirmed stopped notice. Current offline handoff
   separately covers invitation, assignment/transfer and new-comment receipt.
2. Natural proof is held at 503: no run notice and proof_state unknown. Only
   proof release produces succeeded/failed/stopped. Actual group/run identities
   match immutable replay; canonical success/failure/missing/stopped/never-launched
   confirms no Stop-intent announcement and no proxy human actor.
3. All six actual notification title links open the account issue. Run fragments
   focus the precise records. Missing business result remains null/partial and
   is visibly explained.
4. Actual human workflow event actor receives zero notices, other participant
   and recipient one each. Invite/transfer self-exclusion is additionally in
   current offline handoff; actual comment helper excludes its human actor.
5. Twelve actual read/archive POSTs per client return 200/timestamps. Issue,
   execution and both workbench snapshots stay equal; only the archived row goes.
6. One removed notice is added. All 11 retained notices lose title/source/run
   anchors. Live account access_changed clears detail; old Web/Electron result
   href denies title; issue and run-record URLs return 404, participating empty.
7. Four checkpoints per client show creator membership=1, recipient=0 and zero
   issue inbox/subscriber rows. Canonical workflow additionally checks source
   workspace inbox=0; result notification checks recipient ordinary inbox=0.
8. Generic post-commit outbox publisher/source failure retains committed business
   and pending durable delivery, then retries stable IDs without private payload.
   Required notification persistence faults are different: their transaction
   rolls back atomically. They are not claimed as tolerated delivery errors.
9. Original-key workflow and exact natural proof replay preserve receipt, issue,
   notice identities and counts. Canonical concurrency freezes recipients and
   outbox retry retains one notification.

## Minimal WeCom production+ordinary4 scope

| File | Current SHA256 |
| --- | --- |
| `server/internal/integrations/wecom/relay_outbound.go` | `c0b2be0d56ddde20435dadc2e6de6e04b8ea998a240a73cb75066b22f6f6f1d6` |
| `server/internal/integrations/wecom/outbound_test.go` | `e575bf3a5ba0a7a41dc9ecd2e3df02460c1caf8c6c6763aecfee231a6d3596ad` |
| `server/internal/integrations/wecom/outbound_two_replica_db_test.go` | `2ff230b4f9d3db8b9c767c6b50c6112870a14ee72606e83964c918155b71d672` |
| `server/internal/integrations/wecom/relay_ordering_db_test.go` | `1d7f3ccb588aa7dd6b4042dcf17a20f1914e4143262f36476bf4f431228dedf0` |

All reply checks precede sender/media/object access: exact immutable
channel/task/installation/chat/type, task/session, channel-ingested provenance,
active installation and frame workspace. Missing rows/mismatch terminate;
pre-wire dependency errors release/retry the same event with stage-only logs.
The ordinary fixture diffs supply TaskID/workspace/provenance without changing
existing lease/dedupe/drain/context assertions. Their helpers depend on existing
committed fixtures, not new boundary-test-only helpers; the four files can be
staged independently. Root owns current adjacency and staging.

## Findings (fixed)

None required within reviewed scope. No product/test edit by this reviewer.

## Findings (not fixed)

- Existing ordinary attachment MessageID can name another chat session within
  the same workspace. `outbound_media.go:182` and attachment.sql:166 do not bind
  it to the validated task/session. Collaboration files use the separate
  `collaboration_attachment` table; this is not a demonstrated collaboration
  private leak. Outside assigned write scope; retain the low finding separately.
- Retry exhaustion may be counted as drop_no_connection despite a DB fault;
  observability only, unchanged watcher logic.
- Exact Redis release after this new read fault is not directly asserted
  (dedupe=nil); generic Redis release/lease paths are in the zero-skip 28/68 run.
- Inbox-kind relay has no receiver recheck. Collaboration is excluded at the
  producer; do not silently claim receiver evidence or change that path here.
- Full AC84 current tail/non-IM/AC96 correlation remains open, unrelated to
  whether the minimal fixed WeCom reply boundary can commit.

## Limitations

Electron authenticates a real password API into a fresh profile; login-page UX
and installed full Main are separate. Assignment/transfer share transferred
kind; creator receipt of transfer-back is not reasserted by this UI case.
Comment self-exclusion relies partly on existing AC90/91/97 evidence and actual
helper. Browser isolation zero counts have no ordinary positive in this exact
case; canonical/source rules own it. Generic creation outbox proves delivery
failure handling rather than each kind independently. Missing-outcome client
failure is synthetic executable runtime evidence; explicit business failure is
canonical and paid provider account evidence remains separate. Root preflight
supplies listener/build identity; this reviewer did not re-probe or rebuild.
No installed/old-client, other OS, provider quota, Mobile or full-97 claim.

## Verification

- Lint: four reviewed Go files `gofmt -l` clean; `git diff --check` pass.
- TypeCheck: current WeCom `GOTOOLCHAIN=go1.26.6 go vet ./internal/integrations/wecom`
  exit 0. Existing AC83 strict tsc and ESLint receipts both exit 0, recorded source
  hashes unchanged.
- Tests: client 8/8, canonical 15/31, isolated WeCom/IM 28/68 receipts independently
  hash-checked and parsed. No duplicate DB/client test launched by this reviewer.
  Root's expanded-current adjacency rerun remains the downstream prerequisite.
