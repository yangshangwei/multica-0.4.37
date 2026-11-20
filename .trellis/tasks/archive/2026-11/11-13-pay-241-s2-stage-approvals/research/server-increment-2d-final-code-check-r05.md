# S2 2d final code check r05

**Verdict: NOT_ACCEPTABLE for complete r03 code verification. Do not enable P2.** Both service normal partitions passed, but two different service race leaves failed. They have complete inventories and unchanged source; neither failure may be erased by a later narrow pass or attributed to a product bypass without its missing evidence. The current review is complete as a changes-requested handoff. The owner’s new diagnostic work is a separate continuation.

This is the final 2d code/writer check dispatched as `/root/s2_final_code_check_r05`, not the four Web/Desktop scenarios or A11/A12 acceptance. P2 remains false, actual scenarios remain 0/4, and full S2 remains incomplete. The parent 24-case matrix remains 4 PARTIAL / 20 NOT_RUN.

## Findings (fixed)

No product, test, shared helper, specification, migration, peer file or Git write was made by this reviewer. The following owner repairs were inspected in the immutable source and checked through their actual callers; this section does not claim the failing race campaign passed.

- File: `server/internal/service/collaboration_lifecycle.go:507`
  - Issue: read-only completion preparation set a successful delivery confirmation before independent M1 admission. An issuer-revoked refusal could consequently carry that confirmation.
  - Fix: `write:false` returns before setting either confirmation or acceptance decision. The unchanged Q05 negative assertions now cover original-key GET and issuer restoration: the old key stays refused, with no confirmation, event or M1 consumption. The first complete normal and race partitions passed those assertions. The withdrawn r01 failure remains recorded.
- File: `server/internal/service/admin_collaboration.go:135`
  - Issue: the generic administrative error conversion discarded the bounded stage blockers.
  - Fix: preserve `StageCompletionError` for the administrative handler. All three M1 modes exercise incomplete and verified R38 writes, with real password-administration/organization checks and no fabricated initiator acceptance.
- File: `server/internal/handler/issue.go:4929`; `server/internal/service/task.go:2691`
  - Issue: deletion cancelled tasks and failed autopilot work before the final parent/collaboration refusal; a late required write could leave tokens, quota or cancellation events outside rollback.
  - Fix: the original deletion transaction locks the task/token/autopilot/quota intersections with NOWAIT and owns cancellation, token revocation, quota settlement and issue/attachment changes. Publication/reconciliation follows known commit. The actual single/batch/late-attach/fault cases are retained in the independent handler writer evidence, not replaced with a new completion implementation.
- File: `server/internal/service/collaboration_stage_project.go:32`
  - Issue: an absent policy was mistaken for an unknown stored contract, so an ordinary empty project’s virtual-off read could lead to a schema-error delete.
  - Fix: distinguish `found:false` from an unsupported stored contract. The existing knowledge/deletion boundary is preserved; the three-mode empty-project and managed-project controls remain separate.
- File: `server/internal/service/collaboration_stage_completion_tx_test.go:1151` and `:1237`
  - Issue: this reviewer found that the original completion-first trust/metric tests only rejected a competing transaction while the owner held its lock; they did not prove that the second write committed afterward.
  - Fix: the owner added an actual post-owner trust UPDATE/Commit and a successful metric Record, followed by historical same-key recovery with no new event. Both first-partition modes ran those cases. These real two-backend orders are distinct from the private same-transaction corruption/clock seams.
- File: `server/internal/service/collaboration_stage_test.go:537`
  - Issue: the pre-existing proposal test expected every stage event to be `other`, then its replacement initially omitted the legitimately emitted rejection kind.
  - Fix: check the frozen six-kind closed metadata contract and retain the no-prose/no-private-payload assertions. The r03 foundation normal passed it. ROOT’s stronger value/ID/sequence canonical supplement is separately bound by its integration audit; it is not a new whole-source pass.

The owning-transaction candidate/submission/handoff/operation locks, current caller/participant/account checks, original operation header and full result/basis/reason binding, legitimate before-to-after transitions, unfinished M1 rejection and post-receipt final-clock checks were read in `collaboration_stage_completion.go`, `collaboration_stage_completion_binding.go`, `collaboration_stage_signer.go` and the actual workflow/admin writers. Their normal evidence is complete. The result-corruption race assertion below remains unverified in its failed run.

## Findings (not fixed)

### R05-V01 — the result-corruption race failed before its injection

File: `server/internal/service/collaboration_stage_completion_tx_test.go:311`.

`TestStageCompletionFinalOperationHeaderAndResultCannotChange/result` failed in the first r03 race partition with `hits=0`, SQLSTATE `25P02` (transaction already aborted), an all-zero receipt and no delivery confirmation. Its parent is the second FAIL record. The operation-result mutation never ran, so this is **not evidence that corrupted result data passed the guard**. It is a blocking verification gap: the first underlying SQL error and the intended race assertion are absent from this run. Normal passed the original assertion.

Original evidence: `logs/service-completion-race-r03.jsonl`, SHA256 `f569bac9772d5ff2ce4fa74bfe6dbcee4e3502e489e1c930a298370bb82d023e`. This command completed all 37 top-level names and 128 terminal records, with 126 PASS / 2 FAIL / 0 SKIP and no source drift. ROOT verified the same raw failure and assigned the sole owner a new private first-SQL diagnostic allocation. This reviewer does not add database diagnostics or change frozen tests/products. The cause remains unclassified.

### R05-V02 — late authenticated execution history returned conflict

File: `server/internal/service/collaboration_stage_execution_test.go:290`.

`TestStageExecutionLateAuthenticatedFactDoesNotRenewApproval` failed in the second r03 race partition: the late `RecordStageExecutionObservation` returned `stage_execution_conflict` and an empty receipt after 16.94 seconds. Normal passed. The original run does not capture the initial entry state, callback count, retained terminal or dispatch time anchor, so it cannot distinguish a pre-entry fixture expiry from a history-preservation regression.

The actual test selects a two-second release approval and checks only the initial Prepare error. It does not require an initial `unknown` operation or one callback before declaring a later success. A genuinely `not_started` original terminal would make that subsequent contradictory success invalid under the existing contract. This is a **possible explanation to test, not the assigned cause**. The owner’s new diagnostic must retain the original two seconds and assertions and record the missing state/time/terminal facts first.

Original evidence: `logs/service-foundation-race-r03.jsonl`, SHA256 `4c15bf683b325977f256c6201d09683446fdf87752304fe9d133359fbccc7dd7`. All 79 top-level names and 193 terminal records ran, with 192 PASS / 1 FAIL / 0 SKIP, no timeout and no source drift. It is independent of R05-V01 and the owner’s earlier setup-unknown failure.

### Retained scope limits

- The global concurrent-index cleanup invariant retains the eight F2 omissions at migrations 795/796/797/798/799/801/802/803. This reviewer performed no migration/up/down or assertion repair. S2 readiness and the root-owned 39-assertion DDL evidence do not make that global result green.
- A broader read-only `gofmt -l` scan listed 12 non-S2 files, all byte-identical to the checked green-r03 baseline and outside the 116-file S2 formatter list. The full-tree result remains non-green; the scoped S2 formatter is green. Paths and hashes are preserved in `preexisting-format-inventory-r02.json`. No unrelated formatting was changed.
- The withdrawn r01 owner run, the r02 reviewer timeout and the owner’s r03 setup-unknown/default-timeout race remain failed historical runs. A later narrowly passing late-issuer/canonical check does not relabel them or prove their cause. No autovacuum, load or product-deadlock attribution is established.
- Actual source-matched CLI/API/Host/Web/Desktop execution, legitimate P2 activation, A01–A12, CI Go1.26/Linux, real provider/remote CI/payment/deployment and final S5 evidence are not established here. They remain separate gates, not failed assertions silently removed from this code inventory.

## Verification

Input: `/Users/artisan/.multica/acceptance/pay241-20261115-continuation/resume-r05/combined-candidate-r05-r03/source`. Object manifest SHA256 `81cacce71bc781e8c6ca2a6f27fa379ba9befe3671b7187fb81baf71b818720a`; 4,965 regular files. The 4,960-file bare-list parent manifest has SHA256 `af095ab3de74686e00b837bcc47ed12f29541e23ac39e764f8a86b5ccd57df76`. Every execution audit checks the complete file list, size, mode and SHA, including unexpected/special files. All initial/before/after/final bindings match. The two `server/.omc` files are retained non-product snapshots, never executed or merged into ROOT.

| Check | Actual result |
|---|---|
| Service completion/exit/writer normal | PASS: 37/37 top-level, 128 named terminals, 0 FAIL/SKIP/missing/unfinished/duplicate |
| Same service partition race | FAIL: 37/37 top-level, 126 PASS + 2 FAIL, 0 SKIP; R05-V01 leaf and parent |
| Service foundation/review/report/M1 normal | PASS: 79/79 top-level, 193 named terminals, 0 FAIL/SKIP/missing/unfinished/duplicate |
| Same service partition race | FAIL: 79/79 top-level, 192 PASS + 1 FAIL, 0 SKIP; R05-V02 |
| Independent handler normal/race | PASS: 97/97 top-level, 320 named / 270 leaf each; no FAIL/SKIP/missing/unfinished/duplicate |
| Independent router normal/race | PASS: 53/53 top-level, 207 named / 173 leaf each; no FAIL/SKIP/missing/unfinished/duplicate |
| Pure protocol normal/race | PASS: 36 AST top-level / 336 named each |
| Pure internal CLI normal/race | PASS: 12 AST top-level / 76 named each |
| Pure root CLI + ordinary completion normal/race | PASS: 39 AST top-level / 105 named each |
| Go lint/static/type analysis | PASS scoped: vet eight actual packages and full backend build; no diagnostics |
| S2 formatting | PASS: 116 declared Go files; broader old exceptions retained separately |
| Frontend reuse | Bound existing nine checks: five typechecks, two scoped lint, Core1,850 and Views404; reviewer did not rerun or use peer comment-UI source |

The actual Go-list-selected `TestGoFiles`/`XTestGoFiles` and Go AST define the source inventory. The two anchored service alternations partition **116 unique top-level names without overlap**. Normal has 321 different named terminal records; race has 318 PASS + 3 FAIL, representing two leaves and one parent. Parent/subtest records, repeated modes, pure checks and another package are not new business cases. The original ten-minute Go budget was not increased. The r02 command’s 14 missing names and one unterminated name are expressly excluded from complete evidence.

Each service partition/mode uses its own expressly allocated config/database through the original identity launcher. Before every fixture run this reviewer checked actual database identity, READ ONLY, zero other connections, 835 ledger rows ending at exact864 and bare-hex64 constraints. All four final r03 databases have zero transient stage facts and zero other connections; immutable history is retained. The failed r02 database retains its interrupted fixture rows and was not reused. HOME/TMP roots were proven outside Git. No whole `./...` database test, pool/cluster setting change, real account/model, standalone API/browser, migration or Git operation was performed.

The exact argv, start/end/exit, all source/AST/named inventories, stdout/stderr/config/launcher/helper hashes and database snapshots are in the JSON companion and private `resume-r05/final-code-review/`. Toolchain is **Go1.27.0 darwin/arm64**, PostgreSQL17.11. No Go1.26/Linux CI success follows from these executions.

The separate HTTP reviewer `/root/s2_f06_check_r05` completed the four handler/router commands on the same r03 manifest. Its [MD](server-increment-2d-handler-router-check-r05.md) SHA256 is `07a4b1ab0ae633324e1da4a037c1f1aa23110d73fb061e7f8f7b8398170358b7`; [JSON](server-increment-2d-handler-router-check-r05.json) SHA256 is `1ecf41a122c05f883c903789e2f958233ecdd7db182ba39b133aad01ec9776d8`. This reviewer independently reassembled/rehashed those four raw inventories and source snapshots: all recorded counts, selected names and hashes match. F06 is a subset of router207: 10 top-level / 112 named / 95 leaf each mode, with eleven equal before/after witnesses. Handler exit30 and writer32 are subsets of handler320. Seeded attachment/event/operation rows may be nonzero; unchanged complete hashes establish the read-only behavior. The peer's four final databases retain protected M1 histories, have 25 transient tables empty and zero other connections. The ROOT-only execution harness and M1 browser fixture were deliberately not selected, not silently skipped or counted as code passes.

## W01–W18 disposition

These rows describe code and named-fixture coverage. A row marked covered does not complete the enclosing S2 requirement or its later product journey.

| ID | Current code/fixture evidence | Limit |
|---|---|---|
| W01 | Actual SubmitAcceptance, unique immutable candidate binding, three M1 modes, refusal before delivery effects, post-receipt authority/time recheck | Product journey not run |
| W02 | Actual exact-submission lifecycle, owning locks, M1 terminal/full result and historical reopen/key recovery | R05-V01 leaves the operation-result corruption race assertion unhit |
| W03 | Actual self-result read-only admission, atomic generated submission/attachment/result and M1 combinations | Same R05-V01 final-result verification gap; no separate E2E self positive |
| W04 | Actual password R38/organization/disabled creator, blockers and final guard after admin audit/receipt; no initiator accepted event | Administrative product journey not run |
| W05 | Actual ordinary PUT/legacy/Agent root refusals and ordinary-child M1 controls | Source-matched installed/client runtime remains separate |
| W06 | Actual Move delegates original atomic writer; child semantic edits invalidate old candidate | Product journey remains separate |
| W07 | Actual mixed-root pre-effects refusal, sequential committed prefix, duplicate grant/key controls | No full-batch replay ledger is claimed |
| W08 | Actual Create/CreateInTx/comment-child/custom-done; human rights and approved machine materialization with zero auto-dispatch | Real provider decomposition not run |
| W09 | Actual WriteIssueStatus and signed GitHub/VCS root refusal; ordinary M1 trust/signature consumers preserved | Real remote integration not verified |
| W10 | Frozen current CLI allowlists/strict report protocol and ordinary completion unit transport checked normal/race | Matching CLI against real acceptance API/Host not run |
| W11 | Actual EditIssue move/clear under stage × M1, explicit-off/current-source/reason controls and real ordering tests | Product UI journey not run |
| W12 | Actual project impact/delete, enabled-policy blocking and virtual-off empty-project control | No destructive migration or production delete |
| W13 | Actual root/mixed/ordinary-parent delete before effects, late attach, required-fault rollback, known-commit publication | Scoped local storage/event fixture evidence |
| W14 | Persistent child mapping and separate semantic identity; edit/parent/project/delete staleness, status/position controls | Does not make ordinary children follow all root stages |
| W15 | Real enqueue/start/persistent source identity, unapproved/stale lifecycle rejection and both actual orders | No commercial provider or native containment proof |
| W16 | Actual cancel independence and reopen/development/new scope; consumed original key stays historical | No implicit Resume or approval revival |
| W17 | Four root/R38 read adapters, explicit ordinary source, numeric revisions, F06 strict/ACL/cursor/empty/done-child/no-write inventory | Independent HTTP report is bound separately; no browser acceptance |
| W18 | Safe activity allowlist, exact minimal original receipt and frozen blockers; Core/shared timeline/schema checked | Stronger LIVE canonical supplement is separate; actual timeline journey not run |

## Q01–Q12 disposition

| ID | Current evidence | Disposition |
|---|---|---|
| Q01 | Three modes × submit/submission/self/R38 verified and incomplete; enforce missing grant remains refused | Covered fixture matrix, enclosing full check still red |
| Q02 | Owning Tx/before/allowed after, wrong Tx, missing/unfinalized/reuse, NA finalization, original actor/header/result | **VERIFICATION BLOCKED by R05-V01** |
| Q03 | Last DB clock after receipts/audit, issuer/caller/PAT/source/CI/artifact/target/metrics; real orders distinct from same-Tx seams | Covered scoped assertions; not an external execution guarantee |
| Q04 | Exact immutable submission/candidate/lifecycle/scope/operation/handoff locks and legacy/corrupt refusal | Covered |
| Q05 | P ≠ E without source membership, real issuer authorization, minimal grant, revoked/expired roles and frozen refusal recovery | Covered in service + separate actual-auth HTTP evidence |
| Q06 | Read-only source/root preview and self attachment inspection, zero business-write witnesses | Covered by independently bound F06 |
| Q07 | Strict objects/aliases/limits, root/source/viewer/revision cursor, empty/cached/mid-revoke orders | Covered by independently bound F06 |
| Q08 | Lost/unknown commit, original refused/consumed key and current read authority; late external fact history | **VERIFICATION BLOCKED for late fact by R05-V02** |
| Q09 | Exact864 ledger and readiness dependencies; missing state is unavailable, P2 remains false | Readiness negative/structural coverage; actual opening NOT_RUN |
| Q10 | Policy/project/submit/complete ordering, physical-target uncertainty, human-child independence | Covered scoped transactions |
| Q11 | Four exact835/864 schemas plus source shapes/cleanup map; root-owned six raw controls + one immutable supplement,39 assertions | Retained external DDL evidence; checker ran no migrations |
| Q12 | Immutable inputs/configs/owned cleanup collected | **Product/runtime acceptance NOT_RUN:** P2 false, four scenarios0 |

## Evidence, integration and continuation

Seventy task/spec/hook inputs were recorded after reading the complete saved hook and check profile. The 1b/2a/2b/2c original independent results and later closures remain distinct; this final pass did not inspect only the new completion tests. This reviewer rehashed **277 absolute private/report references with zero mismatches**; 78 relative or mutable historical product pointers were not substituted for immutable sources.

The frontend input/build binding independently matches 3,127 inputs, 1,961 dependency links, 1,586 outputs and 23 external traces. Ten historical records contain one clone/setup plus **nine validation commands**. The two inherited optional trace directory entries are preserved as the original artifact record, not new missing files.

ROOT’s integration audit SHA256 `604bd2eb4971e5f794b2be270692439ac4b610049798679c8705dc08052b7390` matches: 17 integrated paths, 19 comparisons, 4,755 protected inputs without drift, and the five expected F06/writer test hashes. This proves the specified merge, not current whole-ROOT or S2 acceptance. All peer changes and migration865 remain outside this reviewer’s writes.

Do not enable P2, recommend opening, archive S2 or turn A11/A12/the four scenarios green on this report. Close R05-V01/V02 with new independently bound evidence or a new reviewed candidate while preserving the original runs. Then the sole owner can perform actual readiness/activation checks, and ROOT can build and run the matched runtime/product acceptance. Root-owned diagnostics and later supplements are not backdated into r03.

Spec follow-up remains ROOT-owned: after executable closure, synchronize the actual private final-guard/receipt/clock/deletion contracts in `collaboration-stage-approvals.md` and `delivery-completion-enforcement.md`, retaining activation/runtime limits. Changed repository files are only this MD/JSON pair. Simplifications: none; no product refactor or dependency. All reviewer-owned test/static processes are ended and databases remain retained.
