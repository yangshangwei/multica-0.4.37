# PAY-241 S3 — independent Extras-A R05 check, 2026-11-18

**Verdict: ACCEPTED as a bounded Extras-A increment.** One R05 execution passed **6/6 top-level tests and 18/18 named subcases**, with no failures, skips, unstarted cases, flaky results or Playwright retries. Both Web and Desktop passed one top and three subcases in each of notifications, human uploads and human read withdrawal. These are not 18 parent business cases. **TC-14, TC-21 and TC-22 remain PARTIAL; full S3 remains open.**

This was an offline review of originals under `/Volumes/artisan/.multica-acceptance/pay241-s3-clients-20261118-x4c5mjep`. Below, `R` means `runtime/extrasA-r05`; `H` means `verification/extras-a-drawer-author-r05/harness`. No database connection, API request, browser, build, test replay or process control was used for this check. No earlier round supplied a pass; the separate R04 upload review is not part of this verdict.

## Verification: execution and original hashes

The runner command selects only `collaboration-recovery-extras.spec.ts`, one worker, zero retries and `--max-failures=0` (no early cutoff). Its exit was 0. The Playwright JSON has exactly six distinct titles, one passed result at retry 0 per title, `repeatEach=1`, and no report errors. Each attached JSON is byte-identical to its corresponding case file under `R/test-results`. Each case contains three distinct, started, terminal `PASSED` entries with results. The six issue IDs are distinct and all six reports name the R05 database and deployment.

| Original case file | Issue ID | SHA-256 (case and reporter attachment) |
| --- | --- | --- |
| `web-extras-notifications.json` | `01a1eea9-4434-7e57-bb5b-4adad4c6cdb3` | `3a0ea3a11a5b0ae500c2847153a52c1216093c9348a1b3514edfc4cdfa6a88dc` |
| `desktop-extras-notifications.json` | `01a1eea9-b950-7318-9546-f90245a2643e` | `097d91db1bf8c7a1c34b7ddff3038641bd0d54eeebed1ea7c93c9e4c29f437b3` |
| `web-extras-uploads.json` | `01a1eea9-6867-797d-b855-3acc6b177d3e` | `4d8328b1c44f89d1743605f10aba9bc302ad614b159a061c7ae98cbb3aca792d` |
| `desktop-extras-uploads.json` | `01a1eea9-ea12-7b81-a9fc-40ed5cd3ac95` | `d0d8a1947a9d7799de5db25fb059175011f5d06ceca164071fcf02e885470afc` |
| `web-extras-revocations.json` | `01a1eea9-7d6c-7301-90dd-d7d8534af80b` | `eee9057f8a452826e574bb16ae703a3faa6c40ea972f1b077768fc268233a044` |
| `desktop-extras-revocations.json` | `01a1eeaa-07df-70ab-bad1-66226f46a81b` | `e87f4b0b7c69008227c2fe5e4157979604f31cffbe10727a1abc9898e73be846` |

I rehashed all **11/11** files named by the seal and all **54/54** distinct final-manifest entries; sizes, digests and recorded modes matched. The manifest does not hash itself; the seal pins it. The runner's before/after source inventories are byte-identical; all 8,915 selected entries (8,672 product source, 243 harness), 638 resolved inputs, two tools and five product fingerprints matched their files. The deployed API reported `640cfa91e`, matching the selected product source `640cfa91e5c9c883aeb8ee6ab2c03551b3d45c54`; the R05 fixture was an explicit harness overlay. **Lint / TypeCheck:** the sealed author results record zero errors/warnings on the actual spec and fixture and TypeScript exit 0. The six-item static listing is not counted as a run.

| Original | SHA-256 |
| --- | --- |
| `R/seal-final-r05.json` | `98398e457aab7bbebd3632dbc41652fe7e38f28860dfcb7cc1175a112311d470` |
| `R/raw-artifact-manifest.json` | `0e51831c28ba1033b1627dc1428ee5e640fde2e9cee0076675af75aeb1e785ee` |
| `R/playwright-report.json` | `0de570cd9ddf958ee216de00918f632ea8299d737099dcb414769de28f232527` |
| `H/e2e/collaboration-recovery-extras.spec.ts` | `9c1030bb0291118e691ebe6412f77dc794c2ac21003420b61ab15fe07423144b` |
| `H/e2e/fixtures/collaboration-recovery-extras.ts` | `5a152b29edafaf7fa50e4b359248f5c0a7177b50a6683750e82455db2ae0265a` |
| `H/e2e/fixtures/collaboration-recovery.ts` | `7f125d331adcfa85dab491b3def6ec07bca084238b71eae90f8e5eafcbffb3d1` |
| `runtime/api-extrasA-r05/stderr.log` (HTTP access records) | `04038c0937a642b79df6267d301074b208ee84135b15a6895a732d951c5db798` |
| `R/upload-row-object-reference-audit.json` | `45cbe382c22582a4242bd6cba7151f0c3a19e6c05fcead75dea7c28b16b3cfc2` |
| `intake/extrasA-r05-before-launch.json` | `70463664d5647f951d1b4a7880431b4fa1851748068f9f6e64ce348aa912192f` |
| `R/database-before-owned-stop-r05.json` | `c59a1f720d6640ce5a2fc88ec4c110a20c20c4325cd5eec1d3843b68cb93382a` |
| `R/database-after-owned-stop-r05.json` | `6bbc152cfa0ee5b6197b1e9b017249919b6808fa21caa7a963a11e37deef5b74` |
| `R/stop-owned-api-r05.json` | `322e0b069e07786ba635c04b0047cab4cd4f7120ef9765f320abef71f9e9ab42` |
| `R/native-and-web-session-cleanup.json` | `f600a1ec16853273b69e93f946c4d53afe4b4cd5c8c854e99a4d28596c03df60` |
| `R/held-web-renderer-provenance-after.json` | `d8f96cbe1c7e91ae7d77cf5f4a778cf493491eb0632c749a967caf7c4c44074b` |

## What the nine subcases proved on each platform

### Notifications — original recipients and receipts (TC-22)

- `mixed_repeat_replay_reconnect`: the real composer contained the same person twice and Agent A twice. One comment/event produced one waiting, ungrouped A command and exactly two distinct notifications: the mentioned handler and the unmentioned participating owner. Neither the author nor the outsider received that event. The handler explicitly clicked **Mark read**: one POST 200 for its original notification ID per platform, then navigation to the original comment. This is not read-on-navigation. The author's original socket closed; both old and replacement sockets had a 101 handshake and issue subscription acknowledgement (Web cookie; Desktop token authentication). Same-key POST replay and operation GET returned the original receipt, without a new UI write, command, notification or dispatch.
- `late_join_and_edit_delta`: the original request was replayed before disconnect, after reconnect and after the late participant joined. The two original recipient/notification IDs remained unchanged. Editing in the late person's mention produced exactly one new mention notification; a prose-only edit produced none. The original Agent input/content remained unchanged.
- `target_withdrawal_after_preview`: the owner revoked A's actual grant after the two-target preview and before the intercepted comment was forwarded. The human comment committed (201); A returned `target_unavailable` with no new command, B got one waiting command. The original A command, batch, event and content survived. The three current human recipients were the handler (mentioned), owner and late participant (not mentioned). Replaying the same request and reading its receipt produced no second effect.

### Human uploads — original multipart and stored objects (TC-14)

- `committed_upload_GET_reload`, `same_bytes_unknown_retry`, `changed_reselection_matching_retry`: all six originals already had a persisted draft/key/file fingerprint when the real server committed 201 and the harness withheld the acknowledgement. The **upload POST counts per platform were 1, 2, 2**, respectively. The first recovered by a GET 200 on the original draft/request after reload, with no second POST. The second retried with the retained file. The third rejected same-name/same-size different bytes without a POST or replacement key; after another reload the exact original file succeeded. The successful retries returned the first attachment ID/version, not just a file with a matching name. Each settled selection was delivered by a distinct explicit comment request; its replay and receipt GET matched the original.
- I joined the six originals to the audit rows by **uploader, draft and upload request**, then checked attachment ID, source workspace, issue, comment receipt, size and content SHA. Each tuple has one winning row, one ciphertext object and two active revision-1 references: its exact issue and its exact comment. The access log has one authorized `GET …/content` 200 per winning ID; the fixture compares returned bytes with the original buffer. This is application-mediated plaintext verification, not an independent offline decryption or a `/download` route claim for these six uploads.
- Across R05 there are **10 distinct rows and 10 distinct ciphertext files**, not six: the four controls are two delivered withdrawal fixtures (two references each) and two deliberately unbound drafts (zero each). All ten on-disk sizes and SHA-256s match the audit; no row lacks a file, no unlisted file appears in that object directory, and no ordinary attachment row was created. References reconcile as **12 + 4 = 16**. No key or plaintext export was needed.

### Human withdrawal — loss of the last read basis versus a surviving basis (TC-21)

- `participant_last_basis_offline_and_delayed_read`: 14 distinct positive endpoints per platform became 14 matching 404s after participant removal while the observer was offline. They include detail, comments/history, activity/commands, attachment metadata/content/download, stage/memory and the reader's own draft list/original operation. Reconnection cleared the private UI, preview, composer and saved draft; historical notifications no longer exposed a title or openable comment anchor. Rejoin advanced the participant epoch **1 → 3**, restored 12 current reads and did not restore the old content/download capabilities. In a separate window the harness held a real authorized comment GET 200 containing the private comment, then removed the participant again. **Both clients cancelled that response at the access fence** (`released_at=null`, `cancelled_at_fence=true`); private content did not reappear. Neither run delivered a stale body to the renderer or tested a server slow-storage barrier.
- `source_only_membership_loss`: the source member had no participant epoch. Twelve previously authorized reads returned 404 on the same paths after removal; the workspace changed 200 → 404. The old native capability pair also returned 404 before expiry.
- `source_loss_participation_survives`: removing source membership kept the independent participant epoch at 1. Twelve current protected reads still returned 200, the saved draft survived, fresh content/download capabilities worked, and an explicit new comment committed 201. The preview and the enclosing Attachments drawer were each closed through their own UI controls before previewing/sending the comment. The workspace changed 200 → 404 and reorder became false; manager-only field options were 403 **both before and after**, not a newly lost right. Across the three subcases there were eight pre-expiry 404 checks on six distinct old capability URLs per platform (the first pair was checked again after rejoin).

## Retained state and process ownership

The three recorded snapshots identify only `multica_pay241_s3c_20261118_x4c5mjep_extras_r05`: same 853-entry migration ledger, 663 ready/valid indexes and 211 public tables. The original gate is identical at allocation, before stop and after stop: **creation on, execution off, host execution off, epoch 2**, no freeze. All 211 post-suite table counts are unchanged across the stop. The relevant business counts started at zero and ended at six collaboration roots, 14 comments, 120 events, 102 operations, 72 collaboration notifications, four commands, ten collaboration attachments, 16 references and two issue handoffs. Execution groups, runs, run events, runtime reservations and `agent_task_queue` remained zero. Two issue handoffs do not constitute an exercised run/handoff reader.

Launch, readiness, stop request and stop result identify the same owned API PID/PGID **40452**, port **64930**; the recorded normal SIGTERM removed the process and listener. Other connections to the dedicated DB went **0 at allocation → 6 before stop → 0 after**; this is not a cluster-wide zero. All 14 case sessions closed their page and context; seven distinct harness Electron processes exited 0 without a signal, with no cleanup errors or remaining owned processes. The pre-existing production Web listener **97008:64931** and renderer **97555:64932** retained the same provenance; they were not owned or stopped by R05. Desktop here is the matching renderer/preload in native Electron, **not installed production Main**.

## Findings and acceptance boundaries

**Fixed:** none; no product or fixture change was needed or made. **Not fixed:** one audit-provenance qualification. The audit records `query_sha256` = `1da4d3a61b76151c1c225024f1e6d9fd96387982bf09e342399876258de291fd`; the selected product file named by `original_source_query` (`server/pkg/db/queries/collaboration_attachments.sql`) hashes to `76b0b8e90767596fccae049db23d24b02237d8c6550d32020958c0919ca18364`. The final manifest does not separately identify the audit SELECT bytes. Do not cite the former as the product file hash or as an independently reproduced query. This limits SQL provenance, not the six reconciled case/row/object/reference results. The sealed originals were left untouched.

| Parent case | Evidence admitted from R05 | Still outside this increment |
| --- | --- | --- |
| **TC-14 — PARTIAL** | Human unknown-upload recovery and comment delivery/replay converge on the original bytes, identity and references. | Agent run-local output uploads, timeout/failure/late-result retry and result deduplication, or real-model timeout/quality. No Provider ran here. |
| **TC-21 — PARTIAL** | Current **human** participant/source read authorization, UI cleanup and pre-expiry invalidation, with a surviving-participation positive control. | Actual queued-author withdrawal and reads via an original live runtime business token (run context, indirect files/sources and handoffs). Actual run/permit reader denominator **0**; Extras-B is separate. |
| **TC-22 — PARTIAL** | Typed mentions, participant delivery and original-event deduplication across replay/reconnect, explicit read, edit delta and owner withdrawal. | Ordinary subscriber/inbox delivery (`issue_subscriber=0`, `inbox_item=0`), and proven success/failure/stopped result notifications from real controlled runs (Extras-B). A nonmentioned participant is not an ordinary subscriber test. |

No conclusion on the separate structured block/repair cases, real models, funds or production deployment. This review does not promote a parent row or close the S3 task.
