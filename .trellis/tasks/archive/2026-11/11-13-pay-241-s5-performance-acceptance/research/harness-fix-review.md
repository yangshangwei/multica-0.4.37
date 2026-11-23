# S5 harness repair: independent check

**Ready for r02 rehearsal. Open blocking findings: 0.** The four final files and r02 overlay have been independently rechecked and match the hashes below. This is tool clearance, not S5 or TC-24 acceptance. Review date: 2026-11-14; reviewer: native `trellis-check`, `/root/s5_check`.

The dispatch explicitly selected S5; the stale S2 hook context did not change task ownership. The review read S5's PRD/design/implementation plan, its check context, the parent acceptance boundaries, and the original repair report. No API, database, real WebSocket, daemon, Provider, model or external request was made. The only source edits are in the two S5 files named below; no product, shared fixture, threshold, pool, dependency or Git index was changed.

## Findings (fixed)

- **F01 — Real successful HTTP heartbeat was rejected.** In `e2e/fixtures/collaboration-performance.ts:973`, the harness required `body.runtime_id`. The frozen API's `server/internal/handler/daemon.go:1291` deliberately omits that field from HTTP responses, returning `status: "ok"` and optional pending actions. The field belongs to the WebSocket ACK. Existing fake positives invented it, so the original 28 passing tests missed a guaranteed initial-heartbeat failure. The harness now checks the real HTTP response shape. Identity remains bound by actor 0's enrolled `mdt_` credential, the request's runtime ID, and the separate owner runtime-list check. Fake HTTP success responses now use `{status: "ok"}`.
- **F02 — Reduced observer load could still pass.** In `runWriteback`, `open_sockets` was only recorded. A counterexample closed observer 99 after warmup and refused its later reconnects; the actual orchestration still completed all 101 fake runs and returned `pass=true`. The parent approved the minimal correction before its implementation. Each boundary now requires exactly 100 connected observers. Each run also retains the existing `socket_id`, connection count and close timestamp, so a timed disconnect or replacement makes both samples `missing`, even after recovery. The rest remain `NOT_RUN`. Existing read timestamps and `observed_latency_ms` retain diagnostic timing without counting the invalid sample as converged. No denominator changed.
- **F03 — Daemon-start diagnostic preceded the actual request.** `daemon_start_requested_ms` was recorded before awaiting the initial heartbeat. It now sits immediately before `startHostDaemon`. The metric clocks, deadline and five-minute fixture context are unchanged.

The node test file adds three named orchestration regressions: a complete successful 101-run path, a 99-observer boundary rejection, and timed disconnect with/without recovery. The successful path verifies 100 distinct formal trigger identities, distinct run IDs, independent reader credentials, 202 retained rows, sample timestamp ordering, cleanup order and no remaining fake timers. The disconnect tests preserve the successful warmup where applicable, the failing pair, all remaining planned pairs and measured diagnostic timestamps.

## Findings (not fixed)

None requiring a code change remain in this bounded review. The existing five-minute host fixture is a rehearsal limitation, not a new failure to conceal or a threshold to extend. Actual sustained heartbeat, API authorization and process/socket teardown still require the coordinator's live r02 evidence.

The planned `.trellis/spec/web/frontend/collaboration-performance-qa.md` belongs to implementation step 4.2 and is outside this review's write scope. When added, it should preserve the HTTP ACK distinction, the exact 100-observer requirement, and invalid samples' separate diagnostic timing. The current S5 PRD/design already require these behaviors.

## Sampling and lifetime audit

- Preparation uses public account registration and participant invitations; measurement logs in every account again, verifies `/api/me`, unique tokens/users/sockets, authentication/subscription ACKs, own-token authoritative reads and the exact active participant list.
- Propagation retains five warmups plus 100 formal writes, with formal writer k equal to observer k. Matching uses the issue's exact `S+1` frame before its own-token GET, keeps original frame time and BigInt versions, and requires the full 10,000-sample denominator. Unknown or failed writes and unexecuted pairs enter the result as missing/+∞. Pooled p95, per-event slowest-observer p95 and zero missing remain separate requirements.
- Writeback retains one warmup plus 100 formal independent runs on the same issue. The reader differs from both trigger and resource owner. A 404 precheck, exact comment/Agent identity, run-record attribution, and authoritative settlement/proof/success/result reads remain required. Business and finalization retain separate complete denominators, origins and budgets.
- Heartbeat sends `runtime_id` and `supports_batch_import:true` using the enrolled owner-bound managed credential. It starts immediately, waits for readiness, then repeats 15 seconds after completion with a 10-second request bound. Failures remain sticky; logs omit credentials and response text. Owner runtime-list reads validate exact owner/workspace/daemon/runtime identity, online state and freshness before triggering, outside both metric intervals. Cleanup joins heartbeat cancellation after daemon settlement and before restoring the gate or closing its log.
- Runtime freshness remains 150 seconds; managed credential expiry is checked; the fixture still has its five-minute context. Sample deadlines remain 10/30 seconds and budgets 2/5 seconds. No production defaults or prior failed evidence were changed.

## Verification

| Stage | Observed result |
| --- | --- |
| Prior source and evidence binding | All four source hashes matched the implementation report, r02 source and overlay at review start; previous static output and 3-skip Playwright log hashes matched |
| Real HTTP ACK shape before repair | 26 pass, 2 fail; `runtime_heartbeat_invalid_ack` incorrectly replaced success/offline classification |
| ACK repair plus full positive path | 29/29 pass |
| Observer-loss counterexample before guard | 1 fail: actual `pass=true`, expected false |
| Observer guard and three added regressions | 31/31 pass; targeted TypeScript and ESLint exit 0 |
| Final timestamp correction, exact final bytes | **31/31 pass, no skips; TypeScript pass; ESLint pass**; source hashes unchanged before/after checks |

Final argv, stdout/stderr, output hashes and before/after source hashes are embedded in [harness-fix-review.json](./harness-fix-review.json). The earlier review red runs are separately recorded with tool-output identifiers; the implementation's original red/green files were not overwritten. Tests use the actual TypeScript harness in a VM with explicit transport/DB/daemon doubles; temporary evidence directories are removed after each scenario. Unexpected real transport is refused.

The final checks reuse the existing targeted S5 TypeScript/ESLint configurations. No broad repository checks were run. The prior opt-out run remains 3 skipped/0 executed and hash-verified; the spec and opt-in guard did not change in this review.

## Final source binding

| File | SHA-256 |
| --- | --- |
| `e2e/collaboration-performance.spec.ts` | `904857638743cccc59347880c63ba28ec151cbbc95e69dc32504301dc9651723` |
| `e2e/fixtures/collaboration-performance.ts` | `75d25dc3f531fa80ca807d7f3da95a0bcc746082702bcf966f225f09b9d6471a` |
| `e2e/fixtures/collaboration-performance-stats.ts` | `a728075a5d1c5d52ef13929dc3cda5c5427cc6cc1adb8f1874f823d0f2b3a73d` |
| `e2e/fixtures/collaboration-performance-stats.node-test.mjs` | `9c67395d28af9ff92354e1f7f83d1095181b5a03477e742bf607ad1cde5cc585` |

The coordinator retained the original r02 overlay under `pre-review-harness`; the final performance harness hash supersedes the intermediate `ad2d672d…` hash from the first refreeze. The final r02 source and `source-overlay.json` were rechecked after refreeze: all four match the final set above. The reviewer did not touch the r01 snapshot, r01 measurements or database, r02 configuration or processes. S2–S4 stability, final production Web/Desktop verification, all 24 parent cases and real human sign-off remain open.
