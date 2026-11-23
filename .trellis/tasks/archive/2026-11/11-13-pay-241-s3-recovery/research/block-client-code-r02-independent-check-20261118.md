# Blocking client R02 scoped independent code correction

- Reviewer and correction owner: `/root/s3_check_batch_block`.
- Active task: `.trellis/tasks/11-13-pay-241-s3-recovery`.
- Decision: **VERIFIED_CLIENT_CODE_CORRECTIONS_R02**. Both source-confirmed defects are fixed and ready for guarded integration. This report does not accept full blocking/S3 behavior.
- Main's initial admission is `block-client-code-check-fix-admission-r02.json` at the private evidence root. A subsequent explicit main-session message adds `delivery-schema.ts`, `operation.ts` and `information-action.test.ts` to the first two files in the fresh `source-block-client-work-r02` copy. The scope expansion is retained privately. Frozen R01, ROOT and the rejected combined R01 input were not edited.

## Findings (fixed)

### BC-R02-01 — actual inactive return reason rejected the whole issue

File: `source-block-client-work-r02/packages/core/collaboration/information-schema.ts`.

The authored server's `collaboration_information_state.go:473` emits `information_request_not_active` when return is unavailable. This includes an ordinary initial issue with no active request, for both managed and unmanaged projects. The strict Core reason enum omitted that existing literal, so the new present `information_workflow` made `parseCollaborationIssue` return null for a routine valid detail.

The correction adds precisely that server-emitted literal to the existing closed enum. It does not accept arbitrary strings, change server wire names, alter authority, or add a dependency or abstraction. All direct workflow, chain and repair-options action reason literals were compared with the client; this was the sole missing actual code.

File: `source-block-client-work-r02/packages/core/collaboration/information-schema.test.ts`.

Three canonical parser assertions were added: initial managed=false and managed=true detail projections preserve the actual inactive return reason and null current request; an unknown inactive-action reason still makes the whole issue unavailable. These exercise the user-facing root parser, rather than mirroring the enum declaration.

The tests were written before the literal correction and executed against the unchanged R01 parser bytes inside R02. RED records **24 assertions: 22 PASS / 2 FAIL / 0 pending**, with only the two actual initial projections failing. After the one-literal correction, the same file records **24 PASS / 0 FAIL / 0 pending**. Frozen R01 remained unchanged in both runs. No failure or prior raw report was overwritten.

### BC-R02-02 — known sealed semantic 503 cannot settle original repair recovery

Files: server `collaboration_information_state.go:377–384`, `collaboration_information_repair.go:105–106` and `:178–182`, `collaboration_workflow.go:652` and `:795`; client `delivery-schema.ts`, `operation.ts` and `information-action.test.ts`. Server provenance is source evidence; client behavior is independently executed mocked transport evidence. No API or database execution of this edge was performed.

The exact server path is:

1. `loadInformationState` propagates ordinary SQL/lookup errors immediately. It marks `PendingUnknown` only for `errCollaborationDeliveryEvidence` or a persisted current submission whose evidence state is not available.
2. `projectDeliverySubmissions` returns that semantic evidence error for malformed or mismatched persisted event/handoff/decision facts. SQL lookup failures return their actual errors instead.
3. `informationRepairOptions` refuses this semantic unknown pending evidence with `information_context_unavailable`, status 503.
4. The repair write callback wraps that domain refusal in `t.result(503)`. The existing workflow owner completes the original operation and returns the error with its exact 503 status; own operation GET projects the content-free sealed status 503.
5. Core's `collaborationDeliveryRefusalStatus` maps `information_context_unavailable` only to 409. `executeCollaborationAction` treats the sealed GET's status 503 as a mismatch, leaving a known failed original intent pending.

The correction gives the existing pure refusal mapper an optional original receipt status. Only `information_context_unavailable` with exact status 503 gets this additional classification. The operation owner supplies the hint only while recovering a repair's original GET; its HTTP-error catch retains its existing classification. Delivery/repair confirmations and non-null status-options extensions cannot turn this refusal into a settled outcome. Every existing single-argument mapper caller retains its prior behavior, including this code's 409 refusal. No server status or wire was changed.

The canonical transport regression first preserves an uncertain original POST, then observes the exact original GET and requires actual status 503 plus retirement of that same draft without another POST. Before the production correction, **22 assertions: 20 PASS / 2 FAIL / 0 pending**; only the two exact semantic-503 GET outcomes fail. The final full Core profile includes all 22 as PASS. Controls preserve existing 409 and stored 403 outcomes, successful repair, strict mismatched/unknown/malformed outcomes, confirmation/status-options rejection, an unsealed dependency HTTP 503, a different operation kind, and protected-read HTTP 403. The declared 503 from an initial POST remains pending until original GET recovery. The canonical fixture now creates a fresh QueryClient after each test so a real denied-access epoch cannot leak into the following case.

This fixes recovery of a known sealed semantic refusal without manufacturing a dependency/commit failure receipt or a successful repair. All failed candidates and RED receipts remain retained.

## Findings (not fixed)

No additional issue was found within the five-file correction scope. Full server/history/policy/authority admission and matching runtime acceptance remain separate required work; they are not supplied by these mocked client checks.

## Verification

- Tests: independently executed initial schema RED **22/24 PASS, 2 expected FAIL** and GREEN **24/24 PASS**, then semantic-503 RED **20/22 PASS, 2 expected FAIL**. Final same nine Core files as the author's complete profile are **171/171 PASS, zero failed/pending**: original 154 assertions plus 17 new regressions. Every native assertion and both RED failures are retained under distinct labels.
- TypeCheck: independently executed final full Core and Views `tsc --noEmit --incremental false --composite false` projects, **exit 0 each**. No emitted or incremental source output. Views uses its explicit project path from the recorded private Core cwd.
- Lint: independently executed existing Core ESLint on the exact five allowed files with `--no-ignore`, **exit 0; five files, zero errors/warnings/messages**. The earlier two-file lint/typecheck records remain separate.
- Environment: actual Node 22 and installed package CLIs, private Core cwd, the exact `run-nodb.mjs` wrapper and `configs/nodb-client-tests-r01.json` with `NODE_ENV=test`, absent database/token settings and blank model/account configuration. Cwd/argv/PIDs, tool/config/driver hashes, raw stdout/stderr and source snapshots are retained for each invocation. No database, API, browser, build or provider execution occurred.
- Preservation: all **3,624** declared R01 source/config/asset entries and modes match the original manifest. Final R02 differs in exactly the five admitted files, without additions/deletions. The first two-file intermediate manifest/patch/report text is retained separately. Each invocation's R02 before/after snapshot has zero drift. The original author's Core 154 and Views 93 receipts remain retained; Core was rerun as the final expanded 171-assertion profile, while Views DOM tests were not duplicated. Final Views typecheck was independently executed.
- Artifacts: private support, the exact five-file patch/full R02 manifest and complete dependency-link reconciliation are under `independent-check/block-client-code-r02`. The adjacent JSON binds every output and both fixed findings.

No parent matrix, public DTO, Trellis task status, archive, ROOT code or Git state was changed. Full blocking verification still needs server/cross-layer admission, current combined source binding, matching API/production Web/Desktop journeys and final owned-resource audit. Full S3 remains in progress.
