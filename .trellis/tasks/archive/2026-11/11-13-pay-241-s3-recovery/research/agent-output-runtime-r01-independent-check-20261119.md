# Agent output live R01 independent check — 2026-11-19

**NOT_ACCEPTED.** The retained whole round has **2 failed tops**, retry0, with **5 PASSED /1 FAILED /10 NOT_RUN** subcases. Web fails during setup before the first output case. Desktop reaches five passed cases, then fails its Stop receipt comparison. Partial passed cases remain evidence of this failed round; they cannot be spliced into a future round.

This check writes only this report and its JSON companion. It performs read-only source, hash, evidence, object and closure reconciliation. No product, fixture, database, runtime, test body, spec, task state, Git or other report was changed. The accepted code report and its historical live `NOT_RUN` checkpoint remain unchanged: Markdown `52434cf5ff3f9245c888b775fe2edbd740354b1b156e94f50b35c98ca671f009`, JSON `19194f764c28ac9405d087b5993125a752434e934a330e032fb5bfaec3be412c`. Its `PASS_CODE_FUNCTIONAL_AND_FINITE_HARNESS_PREREQUISITES` disposition does not accept live R01.

## Execution and exact inventory

`verification/runtime-output-r01/command.json` pins the original R04 harness and production R01 services. Command exit1 runs from `2026-11-19T08:21:56.466723+00:00` to `2026-11-19T08:23:18.286748+00:00`. It uses one worker, retries0, max-failures0, JSON reporter and a closed environment. Both tops execute with retry0. Playwright records expected0/unexpected2/skipped0/flaky0/global-errors0. Web duration is41,997ms; Desktop33,084ms. stderr is empty.

| Exact subcase | Web | Desktop |
| --- | --- | --- |
| `agent_file_unknown_response_original_cli_retry` | NOT_RUN | PASSED |
| `same_key_same_bytes_concurrent_replay` | NOT_RUN | PASSED |
| `changed_bytes_metadata_retry_and_mixed_race` | NOT_RUN | PASSED |
| `failed_result_frozen_files_and_both_shared_renderers` | NOT_RUN | PASSED |
| `explicit_new_success_preserves_old_failure` | NOT_RUN | PASSED |
| `explicit_stop_preserves_output_and_denies_late_token` | NOT_RUN | FAILED |
| `author_withdrawal_denies_original_output_token` | NOT_RUN | NOT_RUN |
| `grant_withdrawal_denies_original_output_token` | NOT_RUN | NOT_RUN |

Desktop starts6/8 case bodies; Web starts0/8. `business_case_count:null` in both failed sidecars is not a completed business denominator. The explicit inventories provide the partial counts.

| Planned whole-round denominator | Observed R01 |
| --- | --- |
| 2 passing tops /16 passing cases | 2 failed tops;5 PASSED /1 FAILED /10 NOT_RUN |
| 10 physical Providers /10 intentional runs /10 original inputs | 3 /3 /3, all Desktop |
| 12 actual **output** CLI invocations | 4 output CLI invocations;8 callback facts =4started+4completed |
| 10 Agent files /20 issue+comment refs | 3 Agent files /6 Agent refs |
| 6 human creation/input files | 2, one per platform setup |
| 4 server-recorded outcomes | 2, original failure and explicit success |
| 0 recovery Provider launches | 0 for the3 reached original-ledger recoveries |

The wide monitor separately records3 startup `issue get` CLI processes, so7 run-local CLI processes exist in total. These startup reads and15 HTTP multipart replays are not output CLI invocations. The source-pinned Provider startup command is `collaboration-host-provider.go:540`.

## Findings (fixed)

None in this read-only assignment. No code fix was made by this checker.

## Findings (not fixed)

### R01-WEB — explicit Web socket configuration lacks `/ws`

The primary assertion is `source/client-output-run-r04/e2e/fixtures/collaboration-recovery.ts:860`, called by output fixture`:906`. It waits35seconds for a distinct live application socket with an issue `subscribe_ack`. The production Web config contains `NEXT_PUBLIC_WS_URL=ws://127.0.0.1:64960`; six actual sockets target the root path, close quickly, and record0frames/0handshakes. No Provider, output CLI or multipart output is reached.

The production URL resolver at `apps/web/config/runtime-urls.ts:103-104` returns the cleaned explicit URL. The bound API registers `/ws` at `source/server-green-r03/cmd/server/router.go:1387`. The observed URL therefore misses the required endpoint. Desktop uses `/ws` and records two101 upgrades with authentication and issue subscription. This evidence establishes a configuration prerequisite failure, not a product defect. Main owns a new correctly configured production Web build and fresh whole run. R01 source/config/raw remain frozen.

### R01-DESKTOP — raw replay is compared with a canonical receipt

At immutable output fixture`:1013-1014`, the replay comes directly from `requestJSON` and is compared with `stop.receipt`. The existing control helper parses the original through `CollaborationControlReceiptSchema` at `collaboration-recovery-runtime.ts:434`; its `receiptBase` uses `z.object` without `proposal_id` at `packages/core/collaboration/control-schema.ts:65-87`, followed by the consistency refinement`:145`. The only saved diff is extra `proposal_id:null` on the raw replay.

The original captured Stop request and parsed receipt both have request ID `65157e43-4c21-41e6-b7fa-63e479274190`. Actual response metadata binds the same POST/key to HTTP200. The original E2E owner independently corroborates this boundary. The existing observer retains no `/control` raw-body bytes/hash, so this conclusion relies on captured original payload/headers, actual response/request metadata, parsed receipt identity and the sole retained diff. It does not claim byte equality between two raw control bodies.

The local fixture remedy is to parse the same replay through the existing Core schema, keeping its original key/body and full equality. This belongs to the E2E owner, outside this check's write scope, and a future fresh whole round must exercise it. The R01 Stop case remains FAILED.

### R01-MONITOR — self sampling children and latest roles pollute the inventory

`runtime/runtime-processes-r01/final.json` and `observations.json` are byte-identical. They contain4,286 unique process records/first-observation events;4,247 have PPID61519 with unavailable kernel paths. The original helper selects its own root-path process, walks descendants, then discards itself (`:25-30`). Its only sampling subprocess call is `/bin/ps` (`:15`). The sole final `alive` PID68894 has PPID/PGID61519, unavailable executable path, and argument hash `f7a4c6b84351253786c2986be776ec678befd1302f3a9190182a50d527256b07`. This is a monitor-child observation limitation; it is not evidence of a lingering product service. The raw monitor cannot certify universal process closure.

Latest roles also drift: readiness PID61610 ends labelled `controlled_provider`, and known recovery daemon62591 ends `owned_other`. Actual Provider PIDs are62404/62528/62647; output CLI PIDs62429/62438/62558/62661; startup read CLI PIDs62405/62530/62648. The five source/evidence-bound daemon PIDs are61638/62324/62477/62591/62707, including three original-ledger restarts. Denominators come from source, callbacks, original run ledgers and separate closure receipts, not latest monitor labels. Main owns any new monitor helper; original R01 helper/raw are retained unchanged.

## Reached Stop facts and retained original identities

Before the receipt assertion fails, Stop run `5ae53482-21fd-4484-9ecf-d7ac9073ac06` produces file `3fecb393-5bdc-463e-ba65-3a9ea9941f99`, returns `stop_requested`, reaches `forced_disposition:stopped`, `progress_state:settled`, `proof_state:host_exit_observed`, and replays its original proof with HTTP200. It has **no server outcome event/result/frozen result refs**. A generated outcome UUID in its local rejected ledger is not a server-recorded outcome.

Its original multipart replay, changed-name replay, new late request key and runtime output GET all return403; the read refusal is `business_capability_required`. Before/after counts are identical. The subsequent original input receipt assertion, authorized human read after Stop, Stop comment preview/download, final retained-history comparisons, both withdrawal cases and complete five-run tail are not reached. These reached facts do not promote the failed case.

| Original run | Original output comment/request UUID | File UUID | Server outcome |
| --- | --- | --- | --- |
| `1fb33dc0-d76d-4cb2-a74a-ff585aa4ced1` | `03eeaf21-d245-4086-988e-645d1519294c` | `570c8b0d-186a-410a-9906-2d70773b05fa` | failure |
| `ea2f6dd0-c972-4907-a764-343f7db85fe3` | `92e13315-6bd9-43b4-a999-a6202501c88a` | `c081ce95-013b-435d-aac6-7f785bbcb4a4` | success |
| `5ae53482-21fd-4484-9ecf-d7ac9073ac06` | `c2a80c5c-c898-431e-8a86-9aa8301aa49f` | `3fecb393-5bdc-463e-ba65-3a9ea9941f99` | absent; Stop |

Each actual file belongs to Agent `49c4349e-d933-4198-8750-73e31dd18394`, its original run and attempt1, with null human uploader/draft. The original failed output is128bytes, explicit success108bytes, Stop105bytes. Ciphertext lengths are156/136/133 and differ in hash from plaintext; recorded encryption-key length is32bytes without exposing key material. Six refs retain their exact issue/comment identities.

Twenty output snapshots preserve identical file metadata/ciphertext facts and runtime comment/outcome events: first file appears20times, success9, Stop3. All three retained ciphertext files still match their recorded hashes/lengths. The global hashed object set equals five retained encrypted rows/objects: two human files plus three Agent files. Combined audit attachment refs are8, with6Agent refs plus2human issue refs. Desktop-local snapshot counts4uploads/7refs exclude Web's setup file/ref; those local counts must not replace combined5/8.

The original failure and explicit success each freeze exactly its own one file ID in the server outcome and records. Original outcome ACKs return200 content-free `outcome_recorded`, using unchanged request hashes. The five Desktop passed cases verify original-key unknown-output recovery (first CLI exit2, same Provider retry CLI exit0), existing-winner concurrent replays201/201, changed bytes/name409 and mixed pair201/409, failed file/result preview+download, and explicit success retaining the old failure. Both shared rendering contexts here are the comment and result surfaces on **Desktop**; Web's counterpart is NOT_RUN.

All three original ledgers retain their identity/proof receipt/hash and literal one-launch lists through restart, ending `proof_acknowledged`, with host-exit proof and tracked process-group cleanup. Final Desktop capabilities3/3 are revoked and reservations3/3 released. Desktop stores18 notifications, including3each `ai_failed`/`ai_completed`/`ai_stopped`; combined retained notification count is21. These observations are partial R01 facts, not full output acceptance.

## Transport and primary-versus-teardown reconciliation

Desktop records19 multipart facts:4`GENUINE_CLI` +15`SAME_AGENT_TRANSPORT_REPLAY`, with7HTTP201/3HTTP409/9HTTP403 and one intentional original output reply drop. Its five output passthrough failures match exactly five original-runtime ACK-loss facts: failure outcome+proof, success outcome+proof, Stop proof. The immutable fixture's teardown contract explicitly recognizes those original-owner ACK losses. Both output/runtime transport error arrays and process-observer error arrays remain empty on both platforms.

All105 Desktop `requestfailed` entries are GET `net::ERR_ABORTED`, with0idempotency keys and0write requests. The Web's one aborted POST targets `/api/client-usage`, separately from output writes. No per-request abort initiator is retained; these are cancellation observations and do not prove105 output write failures or replace either primary error. Web's1,140 runtime transport polling facts carry0original run requests; zero output traffic does not mean the daemon never polled.

Both tops preserve the primary error before teardown. Their five cleanup owners all fulfill: controlled daemon, output transport, original runtime transport, process observer and execution gate. All session cleanups fulfill, page/context closures are recorded, and cleanup error arrays are empty. Web daemon exits0 with ledger0; Desktop daemon exits0 with ledger3; both have empty errors/stderr. Desktop native PID62345 exits0 after page/context closure. One Desktop socket retains null `closed_at` in a snapshot taken before cleanup; later lifecycle/native closure and service receipts must be used instead of calling that snapshot a leak.

API59736/PGID59736/port64960, Web59760/PGID59760/port64961 and renderer59767/PGID59767/port64962 retain matching launch/ready/stop identities and config hashes. All stop via SIGTERM and report absent listeners. The API's binary/health PID/deployment identity matches its bound source/readiness receipt.

The database's initial/admission gate is epoch2 with executionfalse/creationtrue/hostfalse; Web restores epoch4, Desktop epoch6, with the same flags. Actual end has10API connections; after owned shutdown0. The856 migration entries and663ready-valid indexes remain exact. All211 table counts are identical between actual end and shutdown, including3runs/5attachments/8refs/21notifications. Shutdown preserves every failed-round fact and all five encrypted objects; no history/object deletion repairs the round.

## Evidence binding

The4136-file frozen R04 harness has identical before/after manifests and current byte/mode hashes. Both sidecars'13source/program entries resolve against the pinned harness CWD or explicit executable path and rehash exactly. Node/Playwright, recorder, browser config, service configs and bound binaries also match. Browser config SHA is `ca8fd79bda3c641de65b665a02ac04391da5fc0555abf73146f9aa00b2e87b07`; fixture `f75c2f9ec631f311ef205d64efbc714751198264d8ee80b0b63ce067c0ff8e15`; Provider source `dd97c9266823a1fe6617e760306d18e17165f67fd2f981ae8a93f7fa4aca2f89`. Playwright-attached JSON copies are byte-identical to both sidecars. No raw payload, secret value, key material or full process arguments are reproduced here.

Evidence root: `/Volumes/artisan/.multica-acceptance/pay241-s3-output-20261119-tm89sjgy`. Full bounded artifact registry and per-run identities are in the JSON companion.

| Retained artifact | SHA-256 |
| --- | --- |
| `verification/runtime-output-r01/command.json` | `62c269dd581993d675d2e0c5ed5abd0fe9c746d4f6f2e6323898bee94d706473` |
| `verification/runtime-output-r01/result.json` | `db07f5838e34eca81607c25ab3d4ab8ed822e97dfd27683012401264065bed57` |
| `verification/runtime-output-r01/stdout.log` | `37d0fc82d7165674b8a6e90e05a00edc16e561bae2023effb134938acf3242e7` |
| `verification/runtime-output-r01/source-before.json` | `4de74f70a2a67c5f0ce34d1b86c7fcb213cddb91514935cf5cb361eda3af734e` |
| `audit/output-runtime-r01-owned-closure.json` | `f09cf8599a40d30d5232ec131b3dfb6c6b44897056ed7a366707d20792ade6fa` |
| `audit/output-runtime-r01-primary-diagnosis.json` | `f701f51015430332fedd5217a51ad2ba86582b1a1c62cd83bf29741a63bf34b8` |
| `runtime/runtime-processes-r01/final.json` | `733f91ed755f653829ffbe8192d9217aed126480088294b324315751ef88650b` |
| `runtime/test-results-output-r01/collaboration-runtime-outp-6986f-sults-and-current-authority-chromium/desktop-runtime-output.json` | `8b419bc686911750d664e2cb6465af9ddebe13a3822e41a7d697f46c027d8387` |
| `runtime/test-results-output-r01/collaboration-runtime-outp-e40a9-sults-and-current-authority-chromium/web-runtime-output.json` | `a3b3562720046b6305491c1a6090a1da8b634a28d0efbcc16dd3de12fe59251b` |

## Verification

- Lint: **PASS in accepted prerequisite evidence**; no rerun or code change in this read-only R01 check.
- TypeCheck: **PASS in accepted prerequisite evidence**; no rerun in this check.
- Tests: **FAIL for recorded live R01**,2failed tops, retry0; no new test body was executed by this checker.
- Read-only source/config/tool/raw/attachment/object/inventory/closure reconciliation: **PASS**.
- Report JSON/denominators/write-scope validation: **PASS**.

The missing Web run, failed Stop tail and both withdrawal cases remain open. The expected10Providers/12outputCLIs/10Agentfiles/20Agentrefs/6humanfiles/4recorded outcomes were not reached. Current output R02/R09, ordinary UI CAS and full S3 are not closed by this report. Real model/account quality and installed Main remain separately labelled. Original failed rounds, raw evidence, code prerequisite reports and unrelated WIP are preserved.
