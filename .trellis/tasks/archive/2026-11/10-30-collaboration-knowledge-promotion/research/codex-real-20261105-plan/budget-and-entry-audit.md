# Budget and entry audit for knowledge acceptance continuation

Date:2026-11-05. Scope: bounded read-only planning audit of current schedule, two stopped real ledgers and executable entry guards. No model/network/DB/build/test calls, credential reads, git mutations or product edits. Only this report was written.

## Decision

**The full original acceptance matrix still needs15 successful measured Agent executions. With2 failed attempts already consumed, the minimum cumulative budget is17, assuming every subsequent execution succeeds. Current authorized cap15 leaves13, so the complete original matrix cannot finish under that unchanged cap.** A successful baseline alone would not close the knowledge task.

This is an accounting consequence, not a reason to reimplement the product or discard failed trials. The remaining work has two independent prerequisites: demonstrate that the gateway permits the actual app-server client flow, and prepare a continuation entry that debits both historical attempts. Gateway remediation is pending the user's response in the main session.

## Actual budget and successful coverage

| Group | Registered measured starts | Successful real samples so far | Still needed |
| --- | ---: | ---: | ---: |
| E0b ordinary baseline | 1 | 0 | 1 |
| K1 pending/approved/corrected/withdrawn | 4 | 0 | 4 |
| K2 fresh + exact native resume | 2 | 0 | 2 |
| K3 issue/comment/Chat/quick-create/autopilot/root/member/continuation | 8 | 0 | 8 |
| Total | 15 | 0 | 15 |

K2's frozen third-launch refusal is a required zero-process assertion, not a third paid start. K3 requires3 labelled fake seed starts, not3 extra real starts. Thus minimum next real launches remain15, not18. Schedule: [rk1k_entry_test.go:29](/Volumes/artisan/code/2026/mc-knowledge-codex-20261104/server/cmd/server/rk1k_entry_test.go:29). K2 zero-third assertion: [rk1k_scenarios_test.go:644](/Volumes/artisan/code/2026/mc-knowledge-codex-20261104/server/cmd/server/rk1k_scenarios_test.go:644). K3 fixed five ordinary +three Host measured entries and fake seeds: [rk1k_scenarios_test.go:653](/Volumes/artisan/code/2026/mc-knowledge-codex-20261104/server/cmd/server/rk1k_scenarios_test.go:653).

Both observed real attempts failed in E0b before K1. Their actual result files contain only parent/E0b failed terminals and no summary:

- Initial: `.omx/private/knowledge-codex-20261105/real-01-run/result.json`; one actual start, process_complete=false, complete=false; ledger stopped.
- Retry02: `.omx/private/knowledge-e0b-retry02-20261105/real-retry02-run/result.json`; one additional actual start, prior_actual_starts1/prior_debits1, attempt_limit1; ledger stopped.

Cumulative consumption is **1 +1 =2**, not3: the retry's `prior_actual_starts=1` references the initial attempt and must not be added a second time. Remaining authorized capacity is **15−2=13**. Fake suite passes are delivery/framework evidence and do not reduce the15 required real samples.

Verified immutable history:

| Ledger | SHA-256 |
| --- | --- |
| Initial stopped ledger | `a5c786be5a8128b52779eee3db3a850980610924540efd0612ef8b9a66ae9c9e` |
| Retry02 stopped ledger | `dc9e8779a6fb6ac3ac010a77a115239a024fab5aa53e16a578c753caccb69a4c` |

## Why neither existing entry can simply continue

1. **The original full entry restarts E0b and expects15 new measured successes.** Its final counter is hardcoded to15 paid starts, and its full registration mode rejects prior debit/history fields. Reusing the old ledger refuses because it is stopped; using a blank new ledger without historical debit would falsely grant another15. Entry: [rk1k_entry_test.go:46](/Volumes/artisan/code/2026/mc-knowledge-codex-20261104/server/cmd/server/rk1k_entry_test.go:46); full execution and counters: [rk1k_scenarios_test.go:437](/Volumes/artisan/code/2026/mc-knowledge-codex-20261104/server/cmd/server/rk1k_scenarios_test.go:437), [rk1k_scenarios_test.go:704](/Volumes/artisan/code/2026/mc-knowledge-codex-20261104/server/cmd/server/rk1k_scenarios_test.go:704).
2. **The retry02 entry is deliberately limited to the single historical predecessor.** It requires prior_actual_starts1/prior_debits1, a prior full-mode registration, and exactly one original stopped launch. It cannot represent prior cumulative2 or point to the retry ledger. History guard: [rk1k_codex_retry_test.go:41](/Volumes/artisan/code/2026/mc-knowledge-codex-20261104/server/cmd/server/rk1k_codex_retry_test.go:41).
3. **The retry ledger admits only one new process and is now stopped.** Its sample summary hardcodes total2/remaining13; merely changing JSON counters fails guards and/or misreports results. Limits: [rk1k_codex_retry_test.go:77](/Volumes/artisan/code/2026/mc-knowledge-codex-20261104/server/cmd/server/rk1k_codex_retry_test.go:77); summary: [rk1k_codex_retry_test.go:107](/Volumes/artisan/code/2026/mc-knowledge-codex-20261104/server/cmd/server/rk1k_codex_retry_test.go:107).
4. **Cap15 is also enforced in executable admission.** A new approved cap17 cannot work by editing a standalone plan alone. The code must validate the new explicitly authorized allocation and cumulative history consistently. Existing check: [rk1k_codex_registration_test.go:75](/Volumes/artisan/code/2026/mc-knowledge-codex-20261104/server/cmd/server/rk1k_codex_registration_test.go:75); physical reservations: [rk1k_codex_registration_test.go:203](/Volumes/artisan/code/2026/mc-knowledge-codex-20261104/server/cmd/server/rk1k_codex_registration_test.go:203).

## Feasible scope under unchanged cap15

The13 remaining slots permit only partial acceptance even with no more failures.

A useful group-preserving sequence is E0b1 → K1four → K2two, consuming7 further starts, cumulative9 and6 slots left. Full K3 requires8, so this closes baseline/lifecycle/session groups but leaves K3 incomplete. Selecting6 of8 K3 entries would consume all13 remaining starts while leaving2 mandatory cases unverified; that is a reduced scope requiring explicit recorded selection and cannot be called full acceptance. Do not spend a partial K3 group by default or silently drop cases to fit the budget.

If the budget is kept unchanged, recommend stopping after complete E0b/K1/K2 groups and reporting K3 still open, rather than changing the original criteria. This is a proposed partial scope, not authorization to run it now.

For the original complete matrix, request only the material budget decision: cumulative cap **at least17** (2 historical failures +15 new measured samples).17 is a mathematical minimum, not a guarantee: any further failed physical launch raises the minimum needed for eventual full success. No extra buffer is assumed and no quota transfers from other series are allowed.

## Smallest justified implementation next

Proceed with a focused test-harness continuation change while waiting for gateway evidence/budget decision; no further product adapter changes are justified by the observed403.

1. **One immutable continuation history representation.** Retain both original registration/result/ledger references with exact hashes. Validate each unique physical attempt once, reject missing/corrupt/cyclic/duplicate history and conflicting account/provider/model namespaces, and derive prior debits2 from facts. Do not use a free caller-supplied count or mutate stopped ledgers.
2. **Separate global allocation from current attempt/window limit.** Current authorized cap stays15 until explicitly increased. Before every physical start require `validated_prior_debits + current_reserved_debits < authorized_total_cap` and `current_reserved_debits < explicit_attempt_limit`. Bind both to the reviewed current registration/authorization. Keep a new ledger per numbered attempt, mode isolation, failed-start reservation conservatism, actual-start recording and outer durable sample ACK.
3. **Use narrow staged entries that reuse unchanged scenario helpers.** The existing E0b helper can support a newly numbered baseline attempt after prior2 without hardcoded total2/remaining13. If the gateway is still uncertain, give that entry at most1 current launch and stop regardless of outcome. Do not recreate a one-off `prior==2` implementation for each future failure. A knowledge-only K1/K2/K3 entry may then require a successful, exact-source/provider baseline artifact and execute14 original measured cases without paying for E0b again. Alternatively, once gateway repair is established and cap17 approved, one full continuation after prior2 can execute all15 directly; choose this simpler option if no staged gateway retry is needed.
4. **Keep compound groups intact.** K2's2-turn native session and zero-third refusal run together in one fixture/window. Never create a generic cursor that splits that session and claims resumed proof from unrelated fixtures. Preserve fixed K3 row identities and3fake prefixes.
5. **Meaningful default-safe controls before real calls.** Verify second-history debit inclusion (no double count), stopped-history immutability, wrong/missing history hashes, cap exhaustion, mode/window limits, attempted second launch in one-shot mode, complete-vs-incomplete sample publication, and successful baseline gating. Compile and run the relevant fake-only scenario controls on owned databases, followed by independent review and current executable/config/source binding.

The minimal immediate code slice is budget/history admission plus bounded baseline continuation in tagged `rk1k` files and coordinator launcher. Knowledge-only entry/group extraction is justified only when the chosen staged plan needs it. Avoid a universal workflow engine, new dependencies, migrations, UI work or broad product refactoring.

## Recommended operational plan

1. Record this budget correction in current task progress and retain old failures as consumed attempts.
2. Finish the no-model continuation implementation/control tests and concrete draft package, leaving real admission false.
3. Obtain concrete gateway repair evidence for the actual Codex app-server path (the observed rejection alone does not prove the binary unofficial). Do not spoof client identifiers or spend unnumbered health probes.
4. Resolve whether to complete original scope with minimum cap17 or keep cap15 and accept partial coverage. Main may ask this budget question while doing steps1–2; no repeated permission for already authorized coding/account use.
5. Use one bounded E0b run if needed. On pass, bind its evidence and run the selected remaining complete groups in an exclusive window. On incomplete result, stop, preserve consumption and recompute remaining capacity; do not loop retries.
6. Independently review real evidence before changing task status. Production enablement, platform and parent release gates remain separately tracked.

This is a concrete path forward without pretending the gateway failure or the13-slot remainder already satisfy full acceptance.
