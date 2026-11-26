# Historical P1/O1 evidence audit — 2026-11-20

This independent, read-only audit supports the recorded, sample-specific P1-02 and O1 platform conclusions and reproduces the official-CLI usage totals. It preserves both P1 failures. It does **not** complete AC9/AC10, admit another run, establish general model reliability, or validate the production app-server backend.

Audit checkout: `4c78c8b98958cb8a76f1c9a448b56e94285c36b3`. Historical run registrations name source commit `624eb71b96de1f1945d269c47378d6a31005ccd9`; these are different identities. Historical evidence root: `/Volumes/artisan/code/2026/secrem-v2-cli-evidence-20261120`. New private audit output: `/Volumes/artisan/code/2026/secrem-v2-resume-audit-20261120T065505Z/p1-o1-audit`.

The audit read retained files only. No model, agent CLI, gateway, account, database, container, or historical harness was executed. No historical evidence, template, admission, or implementation was changed. The blocked C1 continuation was not retried or delegated. Repository write ownership is this report alone.

## Evidence and method

Read the active PRD/design/implementation records, all `check.jsonl` references, `cli-direct-review-20261120.md`, the two semantic assessments, the usage summary, frozen expectations, the actual protocol, and the bound historical O1 harness. Checked retained platform snapshots, HTTP observations, keyed workspace diffs, native events, broker outcomes, permits, ledgers, source bindings, and raw gateway response usage. Request/response bodies and provider sessions remain private; the report contains object identities, aggregates, and hashes.

The private, standard-library audit completed 614 consistency assertions. These are audit assertions, **not 614 product tests**. They include:

- All 12 evidence hashes cited by the P1-02/O1 assessments match their retained files. Repository and private semantic-assessment copies are byte-identical.
- The current 16-file fixture inventory matches the consumed freeze, SHA256 `c6bf4f474c1b3bb405b78c549ad5bee6c32bae4eeff66ec0efcadff7eddf18d8`; expected-actions SHA256 is `16447913b82abb6ec842efa0468d66307d5326dfd0b72c2b4f0fdaa9e844138b`.
- All 29 P1-02 and 30 O1 registered source digests resolve to matching bytes in retained history, retained binaries/configuration, or the checkout. This establishes present byte availability and equality; it does not independently attest when every copy was created.
- Recomputed all nine complete keyed workspace maps from before/after snapshots. Both recorded diffs match exactly. Neither has removals, an added workspace issue, squad changes, or member changes. Agent changes are only `updated_at`. Issue changes are lifecycle/revision fields; task changes are lifecycle/result/session/work-directory fields. O1 additionally changes one subscription reason.
- Both saved leader and squad bodies match the frozen source after the source's single trailing newline is removed. They remain template version 2. Hashing the untrimmed source and saved body as if they were identical bytes would give different digests; the difference is exactly that newline.
- Rehashed each inspected input at the end of the automated audit; those bytes were unchanged.

## P1-02: platform facts and the original automatic failure

Issue `01a1f759-1e81-7217-a21c-40b6b3db4c4b` belongs to workspace `9abd676d-d836-4197-9e21-2eb333ee9711`. Its sole task is leader task `01a1f759-1f46-7d00-a333-d9ce2b3536ed`, author/agent `d71d7e61-ef1a-4fac-a351-dd526f7b1a96`. The task completed; the issue is `in_review`, not `done`. There are no member tasks, pending tasks, or additional workspace issues.

Handoff comment `01a1f75e-d4ba-7d00-8f8a-fc0fc819ef35` is an ordinary agent comment, revision 1, with that exact author and source task. Its successful HTTP create used the task credential class, and request content, saved response content, and final snapshot content agree. Its recorded keyed receipt points to the same comment, actor, and task with no delegation targets. The owner mention matches the actual human security-owner seat, `a9cc6c08-06a7-4dec-84e9-52713ee28caf`.

The case has no repository/resources. The native record includes a successful read of the frozen drill and only two completed file-change events, creating/deleting a scratch handoff Markdown file. Backend/transport metadata records unchanged AGENTS and drill hashes. The report retains synthetic-versus-real uncertainty and unresolved risk. The incident path is the documented **human fallback when no incident receiver exists**; it does not prove delivery to an incident integration or acceptance by a real incident responder.

The single activity evaluation, `01a1f75f-e983-73dc-95e2-ad9f65e48538`, is genuinely `failed` under the same agent/task. Broker evidence contains one refused supplemental workspace-member-list command; the native record has the corresponding exit 126. The reason records that error alongside the completed handoff/status work.

The original automatic verdict remains `disproved-in-this-sample`: eight checks true, `one_platform_action_evaluation` false. Its SHA256 is `8a0e6dda67357dbb4a60816c8cba0426b6c77237996970eb36501156555269fc`. This was not changed to pass. The frozen P1 requirement is “protocol-consistent activity”; the bound protocol defines `failed` for an encountered error. Thus the separate semantic interpretation is consistent with the original requirement and observed error, while the literal action-only automatic oracle remains failed. This is a limited finding for this sample, not a repaired generic checker.

## O1: author/source graph, inert review, and owner permit

Issue `01a1f76a-a4e9-7c21-9a4b-ea8ec66d8734` finishes `in_review` with exactly four completed tasks: two squad-leader turns, one plain-leader turn, and one security-reviewer turn. There is no implementer/verifier task or additional workspace issue. All eight original automatic platform conditions remain true.

| Role/turn | Actual task | Actual author |
| --- | --- | --- |
| Intake squad leader | `01a1f76a-a593-77f3-915d-054c5c35314c` | `c0c35c62-adb4-4f3c-804c-b3296956c196` |
| Security reviewer | `01a1f771-741b-71ad-a8d0-fc9f677e0918` | `d3881ac5-516b-40be-9919-54a6f7f15083` |
| Plain leader after invalid reply | `01a1f779-2ff5-79ac-ad5b-8630ff29cf14` | `c0c35c62-adb4-4f3c-804c-b3296956c196` |
| Hold squad leader | `01a1f77e-0f63-7399-a99c-f775467020d2` | `c0c35c62-adb4-4f3c-804c-b3296956c196` |

All six comments are revision 1. Each successful create's HTTP content and original author/source match its persisted snapshot. Four agent comments use task credentials and match their actual tasks; both human comments use the human credential class and owner `145c9a88-ba1b-4583-9ac9-ec750c851342`.

| Comment | ID | Recorded relationship |
| --- | --- | --- |
| G1 | `01a1f771-72ec-7ae1-b491-501efc6cab9d` | Intake leader; top level |
| Review | `01a1f778-8826-7b98-8c29-5b5a325b5e65` | Reviewer; reply to G1; `dispatch_eligible=false` |
| Invalid owner reply | `01a1f779-2faf-77e3-bca9-d6167ce86d80` | Owner; reply to G1, no mention |
| Explanation | `01a1f77a-f540-70e2-8c27-116437ff8d09` | Plain leader; reply to invalid owner comment |
| Valid hold decision | `01a1f77e-0f18-7887-a56e-657484bb7001` | Owner; new top-level comment, no mention |
| Closing handoff | `01a1f782-3077-7a8b-8a4f-60b5189b086c` | Hold leader; reply to the valid decision |

The reviewer's task names the G1 comment as its trigger and the intake leader task as `delegated_from_task_id`. The G1 receipt targets that exact reviewer and task. Intake-after and review-after snapshots contain the same two task IDs: the result did not enqueue a leader. Security Reviewer is template version 2/Observer. Its captured instructions include the restricted capability briefing.

**Capability observation limit:** the raw `task_token.api_capability` database row is absent from the retained snapshots. The bound historical harness (SHA256 `5d99e2138caa4d98e305d4dfb6d9d4b69b0d0cca212fe9d305587f34c9c513ac`) queries the actual claimed task/token, parses that row, and rejects a non-reviewer profile or different issue before execution. The retained actual O1 log has one PASS, zero FAIL/SKIP; the corresponding binary digest matches its registration. Together with role provenance, capability briefing, and the inert persisted result, this supports the historical restriction check. It is not a new raw-row/database observation by this audit.

For the invalid reply, the actual claim has no Squad Operating Protocol or squad final-evidence body. Before/after snapshots retain exactly the same three tasks and `in_progress` status. The plain turn adds the explanation comment but no delegation or release. The later genuine top-level owner decision creates the fourth task with squad context.

The private pending and approved permits agree on **all** pending fields, including the review's source task. The permit binds the actual saved review and decision contents:

- Review content SHA256: `b9e08972d8374304256773fa3bf5c49f6b30bbdc1957baa3689d7f729a45df6a`.
- Decision content SHA256: `cf82789718dd7a4994b02452ebd86eaf8b66b773e2709965db3d2a69f9357bda`.
- Recorded order in UTC: review `05:40:33.947306` → permit `05:46:36.042426` → owner comment `05:46:36.178335` → closing handoff `05:51:06.862692`.

The owner identity matches the human security-owner seat. The hold decision and closure retain the explicit `2026-12-04` review date. The closure references the original review, review task, owner, and owner decision, while preserving unresolved risk. There are exactly two `action` evaluations, each belonging to a squad-leader turn; no squad activity was introduced by the plain reply.

All 33 retained O1 broker requests completed with exit 0 and no refusal. Four native turns have valid completion records and unchanged AGENTS metadata. Eight completed native file-change events are confined to scratch Markdown files. This is a single synthetic, embedded-material hold case: no independent acquisition of an actual build artifact, repaired artifact, G2, or production behavior is established. The permit demonstrates exact content binding and recorded chronology; it cannot independently prove a person's cognitive review or authenticate the permit writer beyond the retained operator record.

## Preserved P1-01 failure

The original P1 issue `01a1f73f-1dda-769f-8409-71950b1bdf50` remains `in_progress`. Its sole leader task completed, but the original verdict is `disproved-in-this-sample`. The three failed automatic conditions are handoff text, `in_review`, and a literal `action` evaluation. There was no successful task-credential comment POST; the platform Report retained the final failure explanation as original-author/source comment `01a1f744-7726-7687-9558-90b3d330537a`.

Raw retained provider-visible tool outputs include `Errno 30`/read-only-filesystem failure; the native file-change event failed, and the broker has three refusals with native exit 126. The original comment reports the missing drill and failed handoff. These support the documented environment-failure interpretation without converting the sample into successful handoff evidence. Repeated tool output in request history is not counted as multiple distinct filesystem failures.

Original verdict SHA256: `f3e85d52a97635bc4efbd2ff8c0aef1d72f275f6ea5a7fd7c366264940cf7417`. Original after-snapshot SHA256: `82c86b92f2a193a8196c82f6e74fe7e3d99c7ccd61ce7d145589c57fff97c407`.

Only 27 of its 28 bound source files can currently be resolved. The missing original `direct_cli_backend.go` digest is `5fb8b93ff0c69791a5bd961bfa48c5d24b3a209510346aa4b6938ee0a259f34f`. The compiled binary, recorded source digest, events, and platform evidence survive; they cannot reconstruct those missing source bytes. A later backend copy is not substituted for the original.

## Official-CLI starts, requests, and usage

All four ledgers bind to their retained registration digests. Seven started/finished ledger records match seven native transport records. Every retained gateway request maps to exactly one start interval and one ordered reservation; there are no unmatched or multiply attributed requests. All 70 request and response file digests match their gateway metadata. Each stream contains one completed response with a distinct response ID, HTTP 200, and complete metadata.

Observed request bodies request model `api666-zzzcoding/gpt-6-astra` with effort `max`, and retain the normal `codex_exec/0.164.0` user agent. All 70 completion responses report that model string. Responses do not independently report effective reasoning effort, so `max` is an observed request setting, not an independently observed provider execution setting.

| Sample/turn | Starts | Model POSTs | Input tokens | Cached input subset | Output tokens |
| --- | ---: | ---: | ---: | ---: | ---: |
| P1-01 | 1 | 9 | 466,674 | 348,544 | 9,631 |
| P1-02 | 1 | 12 | 636,103 | 335,616 | 7,598 |
| O1 intake | 1 | 11 | 562,654 | 448,384 | 7,258 |
| O1 reviewer | 1 | 11 | 410,304 | 344,320 | 11,800 |
| O1 plain reply | 1 | 7 | 534,381 | 150,144 | 2,436 |
| O1 valid hold | 1 | 9 | 1,018,054 | 764,416 | 5,079 |
| C1 first turn | 1 | 11 | 603,069 | 483,328 | 7,242 |
| **Total** | **7** | **70** | **4,231,239** | **2,874,752** | **51,044** |

O1 alone totals 38 POSTs, 2,525,393 input, 1,707,264 cached input, and 26,573 output tokens. The plain-reply and hold native totals are cumulative across the resumed leader session. Their deltas match the per-response gateway totals exactly. Summing all four native O1 terminal totals directly would double-count prior leader usage. Equivalently, the final leader cumulative total plus the separate reviewer total gives the correct O1 sum.

Six wrapper ledger outcomes are `completed`; C1 is `evidence_failed`, exit 125, with its stopped ledger preserved. All seven native streams contain a valid completed turn. Native completion and recorded usage do not make the C1 platform trial complete. This audit uses C1 only for aggregate start/request/usage reconciliation, not continuation or behavioral acceptance.

Cached input is already included in input, not added again. No monetary cost is supplied. Historical SDK figures—two starts, seven POSTs, unknown usage, one catalog read—remain a separate cohort; their raw gateway records are not in this assigned CLI evidence root and were not independently re-audited here. The seven starts are the real official-CLI cohort, not an inventory of no-model probes or all historical Codex processes.

## Cited evidence hash verification

Paths below are relative to the relevant private run directory. Every value matches the corresponding previously published semantic assessment.

| Sample | File | SHA256 |
| --- | --- | --- |
| P1-02 | `after.json` | `a24598237f79431f4ca4ac4f411473b322e008e1b7d4dded7f8f16f022d34075` |
| P1-02 | `workspace-diff.json` | `d36df770d74552c7c27e0870de18f7ab05f8b313c0f2ffcd707c57bf73d3e12b` |
| P1-02 | `http.jsonl` | `38c496c437eed89cc5e6ef6c8a302d4b7e25e85aa4365cb3f4d16d77caad9169` |
| P1-02 | `result.json` | `c2ba9c65c95b41653bc5d31fdc3735a56d403966e073e41c59f62090cc4c350f` |
| P1-02 | `verdict.json` | `8a0e6dda67357dbb4a60816c8cba0426b6c77237996970eb36501156555269fc` |
| O1 | `final.json` | `4077ca87b57d263a4fd664e4d4a5cbe775f0befee20001ed0e08f620c6d38457` |
| O1 | `O1-workspace-diff.json` | `c65dee8118ed22431eccd3e07b11d89346133da773e9b3d5a78a9fb9b7862b2e` |
| O1 | `O1-verdict.json` | `ba434fc6ced125c743cb02b3b32202c140faed6f008a330b02ac3ff11122b34c` |
| O1 | `http.jsonl` | `64a352faffa7a1188b081700079552eda216068f9afc96429c4ba8388430a5bd` |
| O1 | `human-hold-pending.json` | `f00674465027f48e8b2c961e43351b7cfd6ad3af01c7d96b68327533c3291500` |
| O1 | `human-hold-approved.json` | `e1dbfc1aeab7af726bc839dcc47171a40eede8908a0a685d30498490f199b279` |
| O1 | `O1-human-decisions.json` | `101d6ce01845fad5e17c20e33598b0134d7cbbe2ab2fe77f3c8dd8c45bf7bc11` |

Private audit script SHA256: `32b2f3666630541fe70519fa5e1a8d725c83528a05e6081e16d6d0479565b52f`. Private `audit-results.json` SHA256: `4fa221c6047cf2cbbb6644ee1ca4d1a798467fe580d8f9afa6dd254ac3783707`. Both files are mode 0600 in the mode-0700 audit directory. The result retains the complete checked-input hash inventory and sanitized assertions; it contains no raw request/response/session bodies or credentials.

## Verification and remaining limits

- Historical evidence consistency: pass for the 614 automated assertions, cited hashes, full keyed diffs, identities, permit bindings, and usage reconciliation described above.
- Product lint/typecheck/tests: not run. This is a bounded read-only historical audit and one documentation addition; retained historical test results are labeled as such, not claimed as newly executed.
- No code simplification or implementation change was made. No broader spec edit is needed from these evidence-only observations.
- Local hashes prove equality to retained bytes and consistency with published digests; they are not an external signature, immutable timestamp service, complete syscall trace, or independent account-billing statement.
- The historical P1-01 source gap, absent raw capability row, operator-permit observation limit, lack of monetary cost/effective-effort reporting, and separate unaudited SDK cohort remain explicit. P1-02/O1 results apply only to their synthetic official-CLI samples. C1/D1 and the full verifier pilot remain outside this acceptance conclusion.
