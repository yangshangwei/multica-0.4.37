# R31 QA entry native failure diagnosis

**ORIGINAL_QA_R4_USER_FENCE_FAILURE_PROVED; HOLDER_UNPROVED.** E03's final R4 reads fail at the current QA user's real SHARE NOWAIT fence with SQLSTATE55P03, then the same request rolls back and returns503. The panel withholds its approval entry after that read error; its explicit Refresh already exists. No product repair is authorized by this diagnosis.

Current original4 is **2PASS/1FAIL/1NOT_RUN, retry0**. E02 completed synthetic release/rollback; E03 fails in common QA setup before its late-deployment purpose; E04/M1/admin is NOT_RUN. Only this new Markdown/JSON pair was written and sealed0444. No source/test, Go/AST/test/list/TestMain, DB/SQL/API/browser, process/signal/Git or recursive-agent operation occurred. The separate peer owns claim/cache/harness/writer inventory.

## Exact native request and response

Summary SHA256 `b2c396b674d5d94f61f6c929b7c6734f0e1eedd92582ba2ceafe083a6d8e41a5` binds full diagnostic SHA256 `d6200517ec3ded67e71f16687fdc6309e7f189b56dae7626db426a99b36e8fc8`:3193records, no4096limit event.

| Request / trace | Reader PID | First failing query UTC | Same-request HTTP UTC/status |
| --- | --- | --- | --- |
| `7057cde6` / 1055 | 96015 | 2026-11-17T20:44:20.339608Z | 2026-11-17T20:44:20.340479Z /503 |
| `b0ee8d1e` / 1061 | 96066 | 2026-11-17T20:44:35.351888Z | 2026-11-17T20:44:35.352262Z /503 |

Both are canonical GET `/api/collaboration/issues/01a1eb40-a908-7ce0-a472-b9d3d743a85b/stage/code-candidates/01a1eb40-aba3-7995-b2e8-e1971ff79660`, verified passwordJWT QA `8aa26ea6-e1a9-4607-8b20-dcc4834731b8`. Capture lines3171–3174 and3190–3193 show begin → failed LockArtifactUsers/*pgconn.PgError/55P03 → successful rollback → complete82-byte503 `{"code":"collaboration_unavailable","error":"Collaboration data is unavailable."}`. Original body SHA256 `da21f7fe9d093cb74a1634ec4b62c968b35b77b279bd8a3858095b82801df96b`. Request ID/trace/path/user/PID agree. **These actual HTTP refusals are caused by the captured query error**, rather than an uncorrelated nearby log.

Generated query digest `43bdd2c80bda2504391b87839ea62ff27895c1bbf8f6e87e1f8da02ac3f10d04` matches the captured statement. `agent_artifact_execution.sql.go:584–590` selects users with `WHERE id=ANY($1::uuid[]) ORDER BY id FOR SHARE NOWAIT`. Native frames show generated row consumption:596 → service LockArtifactUsers:212 → **evaluateDeliveryEvidenceInTx:104 → private read entry:84 → stageArtifactObservationAs:249 → checkStageArtifactCurrent:266 → StageCodeEvidence callback:160 → readCollaborationStageEvidence:36 → actual handler:290**. StageCodeEvidence returns zero projection on error; handler maps the non-domain error to collaboration_unavailable503.

At the exact failing call104, the only ID supplied is the current explicit ArtifactAccess credential's `p.UserID`. The same request verifies QA above. Source therefore reconstructs a **singleton current-QA target** for these final failures; raw UUID arguments were not captured. The retained reviewer and producer chains were not reached by that initial-user failure. SHARE locks coexist with shared readers but conflict with an incompatible user-row writer fence. **Holder PID/process/request/operation remains unproved.** Preserve SHARE: it intentionally fences non-key account updates; KEY SHARE/unlocked reads would weaken that boundary.

OriginalQA R4 inventory is21responses:11success/10failure. Six first errors are LockArtifactUsers55P03. Five occur in initial current-QA observation; the earlier `60b15a6f`/trace985/PID95872 instead reaches retained reviewer → lockStageFinalSigners → stageArtifactSourceReadAs → readDeliveryArtifactInTx → CheckArtifactAuthorityInTx:360. That explicit retained credential and original initiator/approver user set differs from the current HTTP viewer; do not label all user-lock errors as QA's singleton. Four other first errors have errors.errorString and no SQLSTATE/message, in candidate/account/project getters; they are not55P03 solely by type. JSON retains all ten first errors and frames.

RootGET `8be08d6c`/trace1060/PID96054 at20:44:35.287978 returns200,5080completebytes, Todo/revision12/lifecycle1/current write-context1. The separate R4 subsequently fails. A20:44:42 read-only snapshot has0locks/idle sessions and cannot name or exclude an earlier holder. Transaction event timestamps are boundaries, not exact physical lock-release evidence.

## Why the entry is absent

Original result SHA256 `d1d6dbe55c8dcd9ee8f1692f0ec21b117880e5d3f2b3795866d8656500e64d71`; E03 evidence `66bd220f91f3b5b65dc3909221b297ffde0aec2e7b90c86bb645c3248d4f1b5f`. Failure is **fixture329 waiting20000ms to click ApproveQA**, spec353 → releaseReady217. The QA form was not opened; selected-report/checkbox/RecordApproval steps and QA decision POST were not reached. This differs from the earlierR29 failure inside an open modal.

Successful original R4s `7e5b0d58` at20:44:19.473362 and **40558d83 at20:44:20.013613** already returned the exact current candidate/version/digest, valid original code review, report `27f36995-a60d-4700-af4a-c143e2bb0b04`/run `79990664-d4e5-411a-b462-395e8f5f5aa4` passed/usable, and **QAallowedtrue**, expiry20:59:06Z. The18.935 success was still execution_unsettled; actual settlement superseded it before19.473. The prepared report/log verification and planned ordinary-member report404 at20.061 are independent facts, not source-denial evidence forQA.

Frozen StageCodePanel32–34 computes `record = !query.error && query.data?.state === "available" ? … : null`. A fresh R4 error withholds that record even if earlier Query data remains. Derivation29–30 returns approve[] for null; panel78 maps that list. Thus **ApproveQA disappears**, instead of a modal submit merely becoming disabled. The original locator only waits for that withheld button.

Panel60 already renders CollaborationQueryState with an explicit `query.refetch()` callback, outside the record fragment. The shared component renders Refresh. Its source-denial line62 is separate; a generic contention error is not proof of source revocation. Keep this current-error gate and original identities; cached good report/candidate cannot authorize a signature through a failed current read. Root/R1 errors can lock the parent independently, but final root200 plus R4native55P03 distinguishes this request.

The existing hook has retryfalse and15sforeground refresh; the final two failures are approximately15s apart. `stageApprovalUI` performs no explicitRefresh and no automatic retry. Do not lengthen20s or silently retry/submit. Screenshot3 shows the QA page top ending at Codeandapprovals; screenshots1/2 are other actors' open pages. They do not prove a missing Refresh or a QA modal. The actual Refresh visibility at the failed scroll position remains uncaptured; the source affords it.

## Next bounded evidence

A legitimate account/claim writer may conflict with this private read. The capture establishes **reader refusal**, not an unnecessary read fence or a particular idleclaim holder. Preserve accepted shared-reader/default-writer/currentQA/originalreviewer/producer/credential/source boundaries. M1/GetByID was not reached here and is not implicated by these frames.

After the peer's exact idleclaim classification, recommend one new owning-layer file `server/internal/handler/collaboration_qa_idle_claim_read_concurrency_test.go`; if the actual seam is service-owned, ROOT selects the single service equivalent before dispatch. Exercise actual `/api/daemon/tasks/claim`, real sameQA managed/password principal, real artifact-bound QA stage and settled usable report with valid original reviewer. **Emptyqueue is not automatically no-write**: first distinguish due-reclaim/finalization/admission/account/queue/lease effects.

Proposed3top/8leaf/11named: TestQAIdleClaimAndStageRead(idle_claim_first,r4_first,same_qa_read_control), TestQAStageReadActualWriterFences(writer_first,reader_first), TestQAStageReadCurrentAuthority(changed_password,disabled_account,revoked_source). Names/counts are proposals, not written tests. Forward genuine SQL/returns, hold successful first authority/Tx at a real barrier until second terminal, distinctPIDs, exactquery/UUID/SQLSTATE/caller, currentsignability/report and existing72relation/admin/adapter no-effect after intentional setup. Retain genuine nonemptyclaim/reclaim/account writer controls and credential/source/generation denials. Stop fixture atQA before signingQA/release.

If actual emptyclaim has an unnecessary exclusiveaccount fence and no writer effects, new realRED can justify a separately authorized idlebranch repair while real writers keep exclusive defaults. If it is a genuinewriter,55P03 is an expected control; **do not force read/readRED or repairR4**. Current sameQA read/read control should coexist. Peer evidence determines the concrete claim call/fixture; no repeated original4 guessing is needed.

OptionalUI guard is one owning case in packages/views/collaboration/stage-code-panel.test.tsx: currentQA goodcandidate/report → exactfreshR4503 → noapproval/privateprojection plus enabledscopedRefresh → explicitlyfreshauthorizedresponse restores originalIDs/no unsolicitedPOST. It should pass the current intended contract; no UI product repair or E2Eautorecovery is authorized.

Main integrates both investigations and authorizes only the smallest demonstrated owner/test scope on fresh owning normal/race profiles before productrepair. Current2/1/1, historicalR29one/one/two andR28three/one/zero plus completed writes remain immutable. E03late-purpose, E04/M1/admin, fullS2/A01–A12/parent24/S3–S5 retain their gates.

## Source and restoration

Consuming binding `f3beccd23f824eaf4f59c38b7dade4adf4be187b735f78e7d27160263026c1e1` names5053R30backend files and unchanged frontend. This report checked27relevant filebytes/modes against those manifests; no complete newsource/build audit or test ran. Exact query/frame source matches those consumed bytes.

Main stopped API3961/bridge3958 normally once; platforms retained. Afterstop connections0, gatefalse/true/false epoch66,58settled/41complete, leasesunreleased0. Retained audit binds10129refs/1961links,310priorpaths and338currentpaths with usedslotsfreeze0500. These are existing receipts, not live probes by this agent. No source, prior result, artifact or completed write was changed. CapturedUTC `2026-11-17T20:55:58.473855+00:00`.
