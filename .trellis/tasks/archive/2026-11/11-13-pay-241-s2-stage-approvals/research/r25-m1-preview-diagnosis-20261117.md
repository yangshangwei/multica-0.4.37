# Research: R24 M1 preview timeout follows a refused candidate GET

- Query: Does the fourth R24 scenario wait for the wrong preview route, select the wrong artifact, or encounter a product read refusal before selection?
- Scope: internal; immutable R24 runtime evidence, consumed frontend/backend source, matching ROOT source, and existing owning-layer contracts/tests. No source, configuration, API, database, browser, process, test, build or Git operations.
- Date: 2026-11-17, Asia/Shanghai.
- Active task: `.trellis/tasks/11-13-pay-241-s2-stage-approvals`.

## Findings

### Confirmed failure boundary

**The current completion-preview waiter uses the correct route. The preceding actual candidate GET returned HTTP409, and the original screenshot shows the artifact picker absent with the normal unavailable explanation.** A test-only preview-route repair is not supported by this evidence. The underlying candidate refusal has no retained response body or matching PostgreSQL error; its cause remains open.

R24's original complete run remains **3 PASS / 1 FAIL / 0 NOT_RUN, each retry0**. Result SHA256 is `9ed5f3d54e38cd718fd5a97d84f1e13df4d749eb8bf309d5e3534742f42a84a3`, reporter SHA256 `2d43f2b9ad576bd62c27bafef0c749411f20b1229714303acc12626656fff5d5`. The first three cases passed in 72.965, 50.477 and 39.007 seconds. The fourth ended after 63.148 seconds; its reported error is the original **20-second** `page.waitForResponse` in `e2e/fixtures/collaboration-stage-execution.ts:78`, called by the execution spec at `:487`. This is not an exhausted 480-second whole-case deadline.

The R24 Submit result discovery fix worked: the fourth scenario's real POST to the root's `acceptance-submissions` returned200, request `ad4df2a2-e9a8-4aa4-9242-6ab67c270367`, submission `83f6f4a3-fee7-4d9a-8c84-6ea9c50b7216`, collaboration revision17. That original write is retained and must not be replayed. The earlier201 authorization in this case targets its separate synthetic self-root negative control; it is not the pending shared Web completion authorization.

The fourth case has 21 recorded steps and50 retained HTTP facts, with empty cleanup-failure and page-error arrays. Its last business step is `shared M1 transport trace` at `07:04:17.927Z`; the final gate restoration step is at `07:04:39.230Z`. No candidate/preview response is attached to its explicit HTTP facts because `stageCompletionReview` records the preview only after its awaited response succeeds.

### Actual UI and HTTP observations

The original `test-failed-2.png` shows the creator's **Accept and complete** form:

- the submitted Result and Basis are visible;
- Status is Done;
- Use completion authorization is checked and Authorization method is Review and authorize;
- Source sub-issue is the expected ordinary child `01a1e851-7f3d-7a87-b703-6f46f91cea25`;
- **Artifact evidence is not rendered**;
- the normal message says: `Completion evidence or authorization is unavailable. Refresh current evidence or query the original request.`;
- both Authorize exact completion and final Accept and complete remain disabled.

The original final `harness.stderr.log` contains the following narrow request facts for the exact root `01a1e851-7e6e-795a-b3d1-8088000a1d91`:

| Original log line | Method/path | Result |
| --- | --- | --- |
| 16911 | GET `.../status-options` | 200,13.405ms, request `6424fc14` |
| 16912 | OPTIONS `.../delivery-evidence/candidates` | 200,9µs; CORS preflight only |
| 16916 | GET `.../delivery-evidence/candidates` | **409**,924.295ms, request `8c3677fc`, creator `3e3fa0f7-cfa8-4fa4-832e-812a5b7c811f`, Web0.4.40/macOS |

Do not call OPTIONS200 a successful candidate read. The path log omits the query string, body and low-level cause, so the exact request's `source_issue_id`, response code and SQLSTATE cannot be reconstructed from that line alone. The helper source and visible selected child establish the intended selection, not a full captured network request.

There is no completion-preview path in the retained harness text. The logger is not a complete network/request-arrival recorder; this absence alone does not prove that no POST occurred. More specifically, source and the screenshot support a failed candidate/read prerequisite: the helper's `Promise.all` starts a preview waiter while `selectOption` waits for the Artifact evidence locator, which can never appear from the unavailable candidate page. The reporter exposes the preview wait timeout; it contains no independent success terminal for the sibling `selectOption`. Do not claim the artifact was selected.

### Source trace: the preview path is correct

| Boundary | Actual source behavior |
| --- | --- |
| E2E helper `collaboration-stage-execution.ts:69–80` | Opens the real complete form, enables authorization, selects the source child, then waits for POST `/api/collaboration/issues/{root}/delivery-evidence/completion-preview` while selecting an artifact. |
| Shared view `completion-authorization.tsx:42–45` | `page` is nonnull only with no query error and `candidates.data.state === "available"`. It derives the exact target from that page. Preview is enabled only with a target and matching selectable approval. |
| Same view `:105–110` | Artifact evidence label/select live exclusively inside `{page && ...}`; unavailable candidates remove this control. |
| Core `completion-queries.ts:28–37` | Candidate and preview Query keys are bound to source/target, Server/viewer/workspace/access; automatic retries are disabled. Only nonnull preview input enables its request. |
| Core client `client.ts:2281–2300` | Candidate URL is `/api/collaboration/issues/{root}/delivery-evidence/candidates?source_issue_id=...`; preview URL is exactly the helper's expected POST route. HTTP409 becomes `{state:"unavailable",code}`. |
| Actual router `server/cmd/server/router.go:1706–1707` | Registers the same candidate GET and completion-preview POST. |

The six decisive frontend files match ROOT and the consumed R24 source byte-for-byte. The completion service, handler and actual router also match the Go source consumed by the accepted R22 binaries. This is a selected source-pair comparison; it does not certify the entire current checkout or rebuild products. Exact paths/SHA256 are bound in the companion JSON.

### Missing diagnostic coverage

The consumed private `server/cmd/server/pay241_read_diagnostics_test.go` has **two independent coverage limits**:

1. `pay241DiagnosticPath` at `:102–116` accepts only GET root/R1, stage/R2, code-candidate/R4 and run/test-report/T4 reads. It does **not** admit GET `delivery-evidence/candidates` or POST completion-preview at all.
2. `pay241DiagnosticRecordLimit` is4096. The actual R24 JSONL ends with `capture_limit` at **07:02:51.316987Z**, during the second case. Its4096 ordinary records precede the fourth case; there is no matching fourth-root/request record.

Therefore a missing409 body/PGerror is an explicit test-only capture gap. Raising the global limit alone would still omit this endpoint. Existing earlier 55P03/40001 records belong to different requests/cases; they do not identify this candidate409 or its lock holder.

### Candidate read source: plausible lock contender, not proven original cause

`collaboration_completion_preview.go:61–120` implements `lockCompletionPreviewRoot`. It checks the human actor, artifact transaction, current source/account credentials and status catalog, then takes:

- root `LockLifecycleIssue` (`FOR NO KEY UPDATE NOWAIT`);
- root `LockIssueCollaboration` (`FOR UPDATE`);
- child `LockLifecycleIssue` (exclusive NOWAIT).

`ListCollaborationDeliveryCandidatesInTx:123–157` then calls the ordinary `ListDeliveryEvidenceCandidatesInTx`, whose `lockDeliveryPreviewIssue` in `delivery_completion_preview.go:122–159` also takes the source child's original exclusive lifecycle fence. Selected evidence observation/authorization uses additional actual artifact/project/current-credential checks. Source access does not grant artifact access.

By contrast R4/T4's explicit `collaborationStageEvidenceReadTx` in `collaboration_stage_evidence_read.go` switches actual artifact projection reads to the existing shared `readDeliveryArtifactInTx`/`LockDeliveryEvidenceIssueRead` (`FOR SHARE NOWAIT`) and other dedicated shared artifact fences. A concurrent candidate read's exclusive child fence and an existing artifact-bound R4 read's shared child fence can conflict. Other exclusive artifact/project/principal paths may also matter. This is an independently reachable **static hypothesis**; current evidence does not prove that R4 held the original409's contended lock, that a particular SQLSTATE occurred, or that all refusal reasons are locking.

The delivery error wrapper deliberately maps55P03/40P01/40001/23505 and applicable artifact409 to content-free409 `delivery_completion_conflict`; domain/cursor/evidence conflicts also exist. The original response body is absent, so the current candidate409 must not be mislabeled as a specific error code merely from these mappings.

### Smallest next investigation and permitted repair shape

**Next action is targeted passive capture and/or a real owning-layer read-coexistence regression, not a speculative E2E or product fix.** Main owns runtime actions and exact new source/config/command permissions.

1. In a new private test-only diagnostic source, narrowly admit the actual candidate GET path and collect its original status/body plus correlated first PostgreSQL query failure/source frames/backend ID. Preserve the existing cap, body limit,5-second read deadline,20-second UI deadline, roles, original sequence and no automatic retry. Keep credentials out of logs. Capturing only this relevant boundary avoids exhausting the cap with unrelated R1/R4 polling. Preview POST can be separately admitted only when its actual follow-up is needed.
2. Reuse the real artifact-bound `stageReviewFixture`, `stageProjectionBarrierStarter`, `holdStageArtifactR4`, distinct backend PID and5-second reader controls to test candidates versus R4 in both orders. The candidate method receives the **actual pgx.Tx** directly: capture/hold that borrowed transaction using the existing SQL-forwarding barrier, not a stubbed success row. Retain real current authority, full candidate/page identity, no grants/events/status/queue/tar changes, and real writer-held negative controls.
3. If that bounded regression produces a genuine RED, assign its repair to the relevant completion read boundary with explicit private read markers/shared fences. Ordinary completion/authorization writers, current credential/source memberships, exact child/root checks, schema readiness, R38 and retained original receipts remain unchanged. Inspect all nested ordinary/artifact read paths before choosing files; a one-line root lock replacement alone may leave an inner exclusive fence.
4. Keep UI unavailable/refusal and source-cache protection. Do not use a previous cached200, force the missing picker, call the API as an administrator, auto-refetch/retry, send a crafted preview to bypass the current page, or change `waitForResponse` to a different route.
5. Once the correct source boundary has independently accepted RED/GREEN, execute a **fresh** complete four-case journey. Bind its source/products, new staging IDs and all original submit/M1/shared Web/Desktop/GET404/original-operation/recovery/R38/project-exit assertions. Neither a focused coexistence test nor one form-opening success closes A09/A11/A12.

For later helper observability, waiting for/retaining the actual candidate response before waiting for preview would expose the true prerequisite refusal. It is a separate test quality change, not permission to ignore409 or force the original journey through an unavailable projection.

### Exact passive diagnostic slice: one private tracer file

**Final scope is one existing private test file only:** `server/cmd/server/pay241_read_diagnostics_test.go` in a **new private source copy**. ROOT, the R24 E2E helper/spec and every retained consumed source remain unchanged. No frontend listener is necessary: the existing backend observer already captures the admitted response and correlated query errors.

Replace only `pay241DiagnosticPath`'s private allowlist with the actual **GET** candidate path classifier. Its relevant split is seven segments: `"", "api", "collaboration", "issues", <canonical-UUID>, "delivery-evidence", "candidates"`. Preserve the existing method check and `stageControlUUID` validation. Exclude the unrelated root/R1, stage/R2, code-candidate/R4 and report/T4 polling paths for this new diagnostic slice, so earlier cases cannot exhaust the unchanged cap. Keep `pay241DiagnosticRecordLimit=4096`, `pay241DiagnosticBodyLimit=1<<20`, exclusive0600 log creation/0700 parent, pool/tracer/authentication/SQL logic, query collection and existing response Tee byte-for-byte.

`pay241DiagnosticHTTP:204–232` already records the actual original HTTP status, raw `body_base64`, full `body_bytes`, `body_sha256` and `body_complete`; it does not issue another request or alter a response. Its context scope correlates `request_trace`, request ID, verified user and credential **kind** to pgx `TraceQueryEnd`'s actual backend PID, query name/hash, SQLSTATE/severity/error type and source frames. No SQL argument, Authorization, Cookie, bearer or credential value is recorded. Known lifecycle lock arguments disclose only the exact issue UUID. Response bytes are preserved **without redaction or normalization** in the task-private sink; this is synthetic diagnostic data and can include protected candidate metadata. Do not describe the base64 bytes as redacted. The exclusive/private sink and nonsecret report projection retain the intended boundary.

OneMiB body overflow is explicit: the recorded body buffer is bounded, hash/byte count cover the original stream, and `body_complete=false` distinguishes an incomplete stored body. The4096cap records one `capture_limit` event and then stops. A diagnostic capture error writes only the fixed `PAY241_DIAGNOSTIC_CAPTURE_ERROR` log marker. There is no new async browser callback, response waiter or callback cleanup to manage. The original helper's click, source selection, artifact selection, preview wait,20-second and480-second bounds and every business assertion remain unchanged.

PreviewPOST is **excluded until an actual later failure requires it**. GET candidates alone is sufficient to capture this original prerequisite409 and first query error. Raising the global cap or admitting every API path is unnecessary. If the full four fresh original cases pass with this passive classifier, that run can supply diagnosis and fresh controlled acceptance under the normal final independent gate; if it fails, preserve the complete four denominator and captured error. A separate E04 diagnostic followed by a duplicate four-case run is not inherently required.

**Dispatch/check plan:**

1. Freeze a new private Go source from the accepted R22 consumed source; guard the old tracer SHA/mode and old source manifest. Preserve all product and nontracer bytes. This allowlist is the sole edited existing file; diagnostic check artifacts stay outside ROOT.
2. Confirm exact valid canonical GET candidates is admitted. Reject POST candidates, malformed/noncanonical/all-zero root UUID, extra candidate path segments, R1/R2/R4/T4 and completion-preview. Use the existing `stageControlUUID` predicate, without duplicating or weakening it.
3. Compare the whole file outside `pay241DiagnosticPath` byte-for-byte; verify unchanged4096/oneMiB limits, exclusive sink, pool/tracer/Tee/authentication and SQL. Run gofmt verification and matching harness compilation. A small package-local classifier test may be written/executed only if main grants that extra test file explicitly; the initial production/diagnostic source write boundary remains one existing file.
4. Rebuild **only** the matching `stage-router.test` from this private source, with new source/binary/config/log/socket bindings. Reuse the unchanged production Web/Desktop, R24 frontend, CLI/Host/Provider and original synthetic administrator. A new fixture and eight new staging slots avoid replaying the R24 successful submission.
5. Execute the fresh unchanged four-case command workers1/retries0/maxFailures1 with original5/20/480s contracts. Inspect actual candidate response body/HTTP identity and correlated first PG query error. A mapped409 alone still does not identify the physical lock holder; if needed capture a real holder with a separate bounded two-connection regression, not a guessed SQLSTATE.

If a genuine409body/query/SQLSTATE witness points at read coexistence, the smallest real test seams are:

- `stageArtifactReadFixture` + `newStageReviewFixture` + exact candidate/ordinary child from `collaboration_stage_evidence_read_concurrency_test.go`;
- `holdStageArtifactR4` at`:76`, its real post-authority `ListCollaborationCandidateTestReportEvents` barrier, SQL-forwarding `stageProjectionBarrierStarter`/Tx at`:15–65`, and distinct `PgConn().PID()` checks;
- actual `ListCollaborationDeliveryCandidatesInTx(ctx, tx, root, source, org, credential, limit, cursor)` at `collaboration_completion_preview.go:123`, using the real borrowed pgx.Tx and credential from the existing fixture;
- reverse-order hold immediately after the actual candidate reader has acquired its relevant root/child/artifact locks and before real commit, then invoke actual R4 on a second connection while those locks remain held;
- retain both five-second contexts, complete page/approval/source/root identity, current-authority and no-write row/tar comparisons; keep actual writer-first and reader-first negative controls from `TestStageArtifactReadProjectionWriterFences` at`:171` and membership/password/current-source controls from the existing router suite.

Never stub a row or returned readiness/permission value, invoke a pool query outside the borrowed transaction, release the first reader before claiming coexistence, or count a same connection as two readers. If the captured409is a different authority/schema/evidence conflict, follow that actual witness instead of preselecting a shared-lock repair.

### Existing patterns and tests

- `packages/views/collaboration/completion-authorization.test.tsx:86–94` uses the real shared form wiring and waits for Artifact evidence before selecting it; it owns happy-path UI and current refusal behavior, not the real browser's database concurrency.
- `server/cmd/server/collaboration_stage_completion_preview_integration_test.go:95` covers actual candidate/preview routes, exact source/root numeric revisions, current authorization binding, no read side effects and consumed original binding for self/submission/admin.
- Same file `:671` covers real membership read/revocation orders; `:773` covers current password/admin/organization changes. They are retained tests, **not rerun by this researcher**.
- `collaboration_stage_evidence_read_concurrency_test.go:15–101` provides SQL-forwarding barriers and actual held R4; `:127–165` covers R4/R4 and R4/T4 coexistence; `:171` retains writer fences.
- `collaboration_workflow_read_concurrency_test.go` demonstrates the accepted immutable receipt/R4 shared-read boundary and writer/authority controls. It does not prove candidates/R4 coexistence.

## Files found

- `e2e/fixtures/collaboration-stage-execution.ts` — the exact preview waiter, real form and unchanged20s action boundary.
- `packages/views/collaboration/completion-authorization.tsx` — candidate availability, conditional artifact picker and preview gating.
- `packages/core/collaboration/completion-queries.ts` / `packages/core/api/client.ts` — protected Query scope and actual GET/POST routes/refusal projection.
- `server/internal/handler/collaboration_completion_preview.go` — five-second actual candidate/preview handler and error mapping.
- `server/internal/service/collaboration_completion_preview.go` / `delivery_completion_preview.go` — borrowed root/child current-authority locks and nested ordinary candidate read.
- `server/pkg/db/queries/delivery_evidence_read.sql` — dedicated shared projection fences; existing writers retain their original locks.
- Actual `server/cmd/server/router.go` — current completion routes.
- Private `pay241_read_diagnostics_test.go` — scoped passive capture classifier and4096-record limit.
- R24 result, reporter, case evidence, PNG2, final stderr log and passive JSONL — original failure, successful result submission, actual candidate409 and capture gap.
- Companion JSON — decisive refs/hashes, selected source pairs, original outcomes and operation counts.

## Related specs and external references

Read `.trellis/workflow.md`, `CLAUDE.md`, `.trellis/spec/server/delivery-completion-enforcement.md`, `.trellis/spec/core/frontend/collaboration-workbench.md`, and the relevant production/modal/recovery sections of `.trellis/spec/web/frontend/e2e-run-environment.md`. They retain current authority, bounded no-write reads, explicit exact authorization, original-key recovery, no speculative retries and honest scoped acceptance.

No external documentation was fetched; all route/state/locking facts are from the exact installed/consumed repository source. The expected preview path does not require an unfamiliar API or library decision.

## Caveats / Not Found

- No full Playwright trace or DOM snapshot: original trace configuration is off. PNG2 proves its rendered state, not a captured `select.value` or completed `selectOption` event.
- No original candidate query string/body/error code/SQLSTATE/lock holder. Narrow request log status409 is established; its cause is unproved.
- No future candidate/preview request, test, migration, build or process action was performed by this researcher. No original result, submission, grant, completion or gate was changed/replayed.
- Main separately reported graceful R24 API/bridge stop and a fresh after-stop snapshot. Those lifecycle results are main-owned and are not substituted for the missing original candidate response.
- Full S2/A01–A12, S3–S5, parent24, installed Main, real payment/CI/deployment/model and business-person sign-off remain separate gates.
