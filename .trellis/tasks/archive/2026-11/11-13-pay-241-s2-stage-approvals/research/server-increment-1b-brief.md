# S2 server increment 1b brief (preserved)

Source: the brief the PAY-241 coordinator session (-25) sent to the server implementer `as2_server_impl` at 2026-11-14T00:48Z. That implementer stopped at 00:48Z on a model error after implementing most of 1b and editing wire-contract section 13 (items 3–5), without writing the 1b report. Session -45 took over S2 at 2026-11-14 09:50 local. Kept verbatim below so the successor works from the original requirements.

---


Increment 1 report read and its scope accepted as delivered; before increment 2, do a bounded "increment 1b" that closes the three items from my earlier wire-contract review, which the report does not address. Same ownership, databases, wrappers and safety rules as before; still no git writes; still stop at the boundary and report.

Decisions you asked for:
- Proposal children stay title/description/priority only (unassigned, `todo`, root's project). Do NOT add assignees. Accept must never dispatch, and assignment through the ordinary entry points keeps the existing invocation checks in one place. Keep it listed as a scoped simplification.
- Race databases: I am allocating and migrating two now; I will send their config paths in a follow-up message. Until then keep `-race` to pure packages.

Increment 1b — required:

1. Descendant bypass (your deviation 2). "A machine creating a sub-issue under an ordinary child is unchanged" means an agent can decompose work under a managed root without any approved proposal and have it executed. That defeats PRD S2-R02/A02 ("未批准不产生可执行子任务 … 拒绝或未批准零派发"). Close it: machine-driven creation or attach anywhere below a managed root (bounded ancestor walk to the nearest collaboration root; state the depth bound and what happens beyond it — it must refuse, not allow) answers `stage_proposal_required`, and the enqueue/start gate treats a machine-origin descendant without an approved/confirmed mapping as `stage_child_unapproved`. Human-created descendants keep their ordinary contract (control tests). Keep the lock-order rule (no child→root blocking cycle; NOWAIT / pre-collection, re-read after locking). Red-first tests per real entry point: CreateInTx via service and POST /api/issues, update/batch attach, enqueue, start. Decide and document how a legitimately approved child may be further decomposed by a machine: my default is that it may not in v1 (a human files a new proposal or creates the sub-issue manually); if you find that this breaks an existing supported flow, stop and report instead of choosing silently. Update contract section 10 and section 13.

2. A5 `confirm_scope` must bind the code version. The user-confirmed rule is that on first enablement or root reopen the product approver explicitly confirms the adopted existing children AND their code version (PRD "已确认产品规则"; design "首次启用、重开与报告生产入口": "记录child IDs/当前版本、代码候选及root lifecycle"). Today A5 binds only `issue_id` + title/description digest. Add a required per-child `code` member: `null` = explicit "no code carried forward", otherwise a reference the server verifies against the PR actually linked to that child (provider, pull request id, head sha) from the existing connector facts; a reference the server cannot verify is refused, never stored as confirmed; R1 exposes the confirmed code reference and whether it is still current (a changed head makes it non-current and feeds `scope_state` or a per-child flag — choose one and document it). If the verified PR facts are genuinely unreachable from increment-1 code, freeze the field now, accept only `null` in 1b with a named refusal for non-null (`stage_code_reference_unavailable`), and state that increment 2 enables verification — but do not leave the field out. Migration for the new column(s) as a new numbered file (820+), indexes in their own single-statement concurrent files. Update contract 5.1, 7.5, section 11 if the confirmation changes, and section 13; the client lane builds A5 last and is waiting for this.

3. Freeze, as increment-2 contract text (no code yet): the refusal codes and body shape that the existing submit / complete / lifecycle / R38 / project-change / project-delete endpoints will return when stage evidence is incomplete, including a machine-readable list of the missing items (PRD R04/A03/A09: the refusal names what is missing and the state does not change), and the exact R4 shape including `max_expires_at`. Put them in sections 8 and 10.

Also in 1b, small and cheap:
- Add the missing positive test through `EnqueuePreparedIssueTaskInTx` if a prepared set can be built from existing fixtures; if not, say precisely what is missing.
- Deviation 6 (a task queued before the gate applied is refused at start and stays unstarted, invisible to users): make R1's `execution_gated` true for such a child and name the refusal in the contract, so the client can show why nothing runs. No new UI surface.
- Deviation 8 (content digest visible to external readers who cannot open the child): confirm it leaks nothing beyond equality of content, and note it in the contract; no change needed if so.

Do not classify `report_snapshot*` in the deletion manifest and do not touch `TestBatchUpdateIssues_AutopilotRunAssign`; both belong to other work and I will report them separately.

Report as `research/server-increment-1b-results.md` / `.json` with the same structure (commands, exit codes, named tests, first-failure logs with names starting `s2-server-inc1b-`, contract changes, what was not verified), then message me in under 300 words.



Two race databases are allocated for the S2 stage suites; they are yours and nobody else's.

- service package: `/Users/artisan/.multica/acceptance/pay241-20261113-01/configs/s2_race_service.json`
- handler package: `/Users/artisan/.multica/acceptance/pay241-20261113-01/configs/s2_race_handler.json`

Use them only through `run.mjs`, only for `-race` runs, one package per database (`./internal/service` on the first, `./internal/handler` on the second), and scope `-race` to the stage tests and the existing tests whose code paths you changed (CreateInTx, update, enqueue/start, workflow mutation) rather than the full packages.

Both were migrated at 2026-11-13T23:36Z from the working tree and are at `820_collaboration_stage_child_code`, with these file hashes at the time: up `bd7c405d…9cc81f`, down `5bae0fb1…ab3e5`. If you changed either 820 file after that, or add 821+, you are authorized to bring exactly these two databases (and your two original ones) in line with `go -C server run ./cmd/migrate up`, and to run a targeted down/up of your own stage migrations if a file changed after being applied — record what you ran. Do not use any other database.

