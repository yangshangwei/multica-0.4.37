# Research: R19 source-denial display and authoritative read fences

- Query: Explain the R19 first-case source-denial banner failure without assuming that every GET error is an incompatible reader lock.
- Scope: Internal; static source inspection and immutable R19 evidence only. No database, API, browser, build, test, migration, or Git command was executed.
- Date: 2026-11-16
- Active task: `.trellis/tasks/11-13-pay-241-s2-stage-approvals`

## Findings

### The missing banner is downstream of successful authoritative denial

The original R19 four-case result is `0 PASS / 1 FAIL / 3 NOT_RUN`, with retry zero. The first case fails at `collaboration-stage-approvals.spec.ts:351`: after removing the reviewer's source membership and reloading, the original five-second assertion cannot find the source-denial text. This is not evidence that the API never denied access.

The immutable `r19-metric-freshness-fixture-r01/evidence/four-scenarios-read-diagnostics-r01.jsonl` contains the following distinct observations, all for the real reviewer and current candidate:

| Raw line / trace | UTC time | Observation |
| --- | --- | --- |
| 797 / 267 | 12:01:25.777840 | R4 succeeds with a readable candidate before the final loss of source access is reflected. |
| 805 / 270 | 12:01:26.005911 | A concurrent R4 fails at `LockArtifactMembers`, SQLSTATE `55P03`. |
| 807 / 270 | 12:01:26.007431 | That same request returns `503 collaboration_unavailable`. |
| 820 / 275 | 12:01:26.109018 | The **first captured R1 source denial returns 200**, both children `can_open = false`, null content digests, and the reviewer's role projection marked external. |
| 825 / 276 | 12:01:26.146042 | A later **R4 returns 200**, `candidate.baseline = null`, `blocked_evidence = 1`, empty reports/decisions/execution arrays, and four signable-step entries with `allowed = false`. |
| 859 / 290 | 12:01:26.739337 | A **fresh R1 returns 200**, both children `can_open = false`, null content digests, and the reviewer's role projection marked external. |
| 868 / 293 | 12:01:26.783603 | Another fresh R1 returns the same authoritative source denial. |
| 872 / 291 | 12:01:26.785874 | A separate overlapping R4 returns 503 following a non-PgError query failure. |

These are response-completion timestamps, not exact database commit/lock-release or browser-consumption timestamps. The result ends at 12:01:32.308Z. Denied projections existed before that terminal failure; the currently retained traces do not establish which response the browser ultimately consumed.

The statically identifiable display gap is in `packages/views/collaboration/stage-code-panel.tsx`:

1. Lines 32–34 derive `record` only from error-free, available R4 data.
2. Lines 62–64 render the denial message **only inside `record && ...`**.
3. `packages/core/collaboration/stage-queries.ts:39–43` treats a fresh R1 denial as authoritative and calls `eraseCollaborationStageSource` before publishing it.
4. `packages/core/collaboration/stage-source-cache.ts:15–33` increments workspace authority generation, cancels pending sibling private reads, and clears their private data. A previously accepted anonymous R4 identity can be preserved, but a readable or missing R4 becomes absent/pending.
5. `packages/core/collaboration/stage-presentation.ts:10–12` already detects authoritative source denial from R1 child `can_open === false`; `collaboration/index.ts:108` exports it through `@multica/core/collaboration`.
6. `deriveCollaborationStageCodeState` already uses that R1 denial to suppress source access and authorizing actions (`stage-code-presentation.ts:25–30`). With no R4 `record`, however, the component omits the matching explanation.

Therefore the smallest proposed behavior repair is to show the existing source-denial message when **R1 itself denies source access**, even while R4 is missing, pending, unsupported, or failed. Keep current source-cache cancellation, private-data scrubbing, query error/retry presentation, original request identities, and form locks. Do not synthesize an R4 record or retain cached readable evidence to make the banner appear. A deterministic failing component regression remains required before implementation; this research is not itself a test result.

The first R1 denial at line 820 precedes the anonymous R4 at line 825 by about 37 milliseconds. That directly matches the source-generation cancellation path: an R4 request begun before R1 denial becomes superseded before its result is returned by `protectCollaborationStageSource`. The late anonymous HTTP body does not guarantee an accepted R4 Query record. Later R1 denials repeat the same fence.

### The observed member lock is already compatible between readers

`server/pkg/db/queries/agent_artifact_execution.sql:66–68` defines the actual failing query as:

```sql
SELECT * FROM member
WHERE workspace_id = $1 AND user_id = ANY($2::uuid[])
ORDER BY user_id, id FOR SHARE NOWAIT;
```

`lockStageFinalSigners` calls this existing shared query at `collaboration_stage_signer.go:240`, after comparing retained signer, participant/account generations, source-member identity, role/policy revisions, and expiry. It then repeats the exact retained member-ID comparison at lines 244–246. Calling this helper from a read does **not** introduce an exclusive member fence.

Shared locks coexist with each other and intentionally conflict with role changes/member removal. `workspace_revoke.go:230–238` deletes the member and commits in one application transaction. The original `55P03` is compatible with overlap between the read and a genuine member writer. Its exact conflicting holder has not been proved by the captured R19 evidence; a QueryEnd timestamp alone cannot identify it. There is no basis here to replace SHARE with KEY SHARE, remove NOWAIT, catch-and-ignore `55P03`, add automatic retries, or weaken revoked-signer checks.

### Reachable R4/T4 lock families and writer invariants

The concrete marker is constructed only by `readCollaborationStageEvidence` (`collaboration_stage_evidence_read.go:15–39`). The function starts one repeatable-read transaction, resolves current issue/source access in that snapshot, keeps the same marked transaction through nested evidence checks, and commits only after the entire projection succeeds. Errors discard the assembled private projection. Inherited `Begin` does not propagate the marker to arbitrary nested writers.

| Family / path | Read fence in the current source | Preserved invariant |
| --- | --- | --- |
| Workspace | `LockArtifactWorkspace` → `TryLockCollaborationAuthorityWorkspaces`, `FOR KEY SHARE NOWAIT` (`artifact_authority.go:88–96`; `collaboration_authority.sql:54–55`) | Existing workspace deletion fence and exact workspace identity. |
| Initial artifact issue | `evaluateDeliveryEvidenceReadInTx` → `LockDeliveryEvidenceIssueRead`, exact issue/workspace, `FOR SHARE NOWAIT` (`delivery_evidence.go:79–100`; `delivery_evidence_read.sql:4–6`) | Public `EvaluateDeliveryEvidenceInTx` passes constant false and keeps `LockLifecycleIssue` as its original writer fence. |
| Project | `LockProjectForAssociationNowait`, exact project/workspace, `FOR SHARE NOWAIT` (`project.sql:92–93`) | Project association/version remains authoritative; no added reader mode needed. |
| PR/link sets | Existing table `LOCK ... IN SHARE MODE NOWAIT` (`delivery_evidence.go:129–133`) | Covers absent/new links and head ties; read/read compatible, conflicts with real PR writers. |
| Artifact principal join | `LockDeliveryArtifactPrincipalsRead`, original task/agent/runtime/issue/origin-member/owner-member/user joins, `FOR SHARE OF ... NOWAIT` (`delivery_evidence_read.sql:8–18`) | Original join predicates preserved; writer entry retains original task-capability fence. |
| Users and members | Sorted `LockArtifactUsers` and `LockArtifactMembers`, both `FOR SHARE NOWAIT` (`agent_artifact_execution.sql:62–68`; `artifact_authority.go:190–235`) | Disabled accounts, exact existing membership, manager and invocation permissions remain checked. |
| Signer membership | Same shared `LockArtifactMembers` at `lockStageFinalSigners:240`; comparison to frozen `SourceMemberID` | Removed/rejoined membership cannot revive an old approval. |
| Password/PAT | `LockArtifactPasswordCredential`, `LockArtifactPAT`, both `FOR SHARE NOWAIT` (`agent_artifact_execution.sql:70–75`; `artifact_authority.go:101–183`) | Actual original JWT/PAT/password version and database-clock expiry. Managed bound credentials have additional existing binding/installation/namespace checks; no evidence they were the R19 browser branch. |
| Invocation targets | `LockTaskCapabilityInvocationTargets`, `FOR SHARE NOWAIT` (`task_token.sql:31–32`) | Original originator, initiator, claimant and target grants still checked. |
| Artifact approval/execution | `LockDeliveryArtifactApprovalRead` / `LockDeliveryArtifactExecutionRead`, `FOR SHARE NOWAIT` (`delivery_evidence_read.sql:20–24`; `delivery_evidence.go:311–326`) | Public `lockDeliveryArtifactInTx` selects its original writer mode; request/spec/receipt/generation/output identities retained. |
| Retained QA issuer access | Marked branch in `stageExecutionApprovalChain:93–106` selects `readCompletionCollaborationAccess`; collaboration/session, participant/account fences remain shared (`delivery_completion_collaboration.go:48–75`) | Public `completionCollaborationAccess` passes constant false, even if given a read marker. |
| Exact report run | Marked QA branch calls `stageQAReportReadAs`; id-only `SELECT ... FOR SHARE NOWAIT`, one-row check, then original scoped getter (`collaboration_stage_qa.go:19–27,64–74`) | Public `stageQAReportAs` keeps constant-false writer entry and original exclusive run fence. |
| Candidate/review/report/intent/metric projections | Exact scoped plain getters and bounded list queries, within the same RR snapshot (`collaboration_stage_review_view.go:134–224`; `collaboration_stage_execution_view.go:39–117`) | Private arrays withheld on source denial; source getters cannot be relabelled as mutations. Actual execution-source adapters retain their own current-authority contract. |

No missing exclusive-to-shared propagation was identified in the currently inspected R4/T4 chain. Existing deterministic database regressions already exercise real two-PID R4/R4, R4/T4, valid standing QA/release chains, reader/writer conflicts, and revocation. Preserve them, their original five-second barriers, and their accepted immutable observations:

- `TestStageArtifactReadProjectionConcurrentR4`, `...ConcurrentR4T4`, `...WriterFences`, `...Revocation` in `collaboration_stage_evidence_read_concurrency_test.go`.
- `TestStageExecutionReadStandingApproval`, `...ReadWriterAccessFences`, `...ReadRetainedIssuerFences` in `collaboration_stage_execution_read_concurrency_test.go`.
- `TestStageExecutionRunReadFences` in `collaboration_stage_execution_run_read_fences_test.go`.

### Diagnostics and source identity

The actual tracer source is the retained R18 build-source `server/cmd/server/pay241_read_diagnostics_test.go` (231 lines). It uses `pgx.QueryTracer`, not a Rows/Scan/DBTX replacement. Its callbacks hash/query-label the SQL, carry context metadata, read the PID, classify error types/SQLSTATE, collect stack frames, and append local JSON. It issues no diagnostic SQL and changes no query result or Rows lifecycle. No direct tracer contract defect was identified. A separate independent checker owns the deeper pgx error/cancellation classification; no production repair should assume a tracer defect.

The raw stream begins recording generic non-PgError query failures and `connLockError` rollbacks well before source removal. The captures record types, not error strings or connection status. Installed pgx 5.9.2 uses `connLockError` for busy, closed, and uninitialized connection states; its type alone does not prove busy or cancellation. These are distinct from the one recorded R19 member-row `55P03`; an unrecorded error string cannot be relabelled as that SQLSTATE. The independent checker is responsible for their precise cascade findings. Synchronous local tracing has scheduling overhead, but that overhead was not measured and does not justify a repair here.

Static SHA256 comparisons show current ROOT equals the actual R18 runtime build-source for all ten requested read-chain files: evidence-read, signer, review-view, execution-view, QA, execution-approvals, completion-collaboration, delivery-evidence, read SQL, and generated read SQL. No backend reintegration is needed to obtain that inspected parity. This is a ten-file scoped identity check, not whole-ROOT or full-S2 acceptance.

## Recommended bounded implementation and validation

1. **Files:** `packages/views/collaboration/stage-code-panel.tsx` and its existing `stage-code-panel.test.tsx`. Reuse exported `collaborationStageSourceDenied`; avoid a new store/helper/protocol or dependency. Preserve all peer UI changes.
2. **RED:** Add a B01 regression using the actual shared stage panel, real QueryClient and existing API fixture. Start with readable R4, hold an overlapping private R4, deliver authoritative R1 with unreadable children, and assert the denial banner appears while private source/review text and approval actions are absent. Do not resolve the private R4 to a denial just to make the assertion pass. Also cover an unavailable/failed R4 under already-known R1 source denial if needed to exercise the error-render branch.
3. **GREEN:** Render the same existing banner from authoritative R1 denial independently of `record`, while keeping the original R4-only anonymous denial behavior and avoiding duplicate banners. Do not remove query failures or enable controls.
4. **Commands:** Use the owning Views Vitest runner for `collaboration/stage-code-panel.test.tsx`; retain the existing Core `collaboration/stage-queries.test.ts` / `stage-source-cache.test.ts` as canonical cache/fencing controls. Run scoped lint and the consuming typechecks. Root must register exact new frozen source/output paths and commands; this researcher ran none of them.
5. **Runtime:** Rebuild the changed shared frontend's production Web and Desktop renderer on the same accepted product graph. Reuse backend artifacts only after their scoped hashes and actual runtime configuration/ownership match. Bind one new four-case run, workers 1/retries 0/maxFailures 1, preserving original bodies, the five-second banner assertion, twenty-second operation budgets and 480-second scenario budgets. Old R19 failure remains immutable.
6. **Gate:** Only fresh actual four-case passes plus independent A01–A12 evidence can close S2. A passing banner regression or hash parity does not close S2, S3, real S4, S5, or the parent 24-case ledger.

## Files found

- `packages/views/collaboration/stage-code-panel.tsx` — banner display presently depends on available R4 data.
- `packages/views/collaboration/stage-code-panel.test.tsx` — real QueryClient/API shared panel fixtures; current denial tests rely on R4 denial.
- `packages/core/collaboration/stage-presentation.ts` — existing authoritative R1 source-denial predicate.
- `packages/core/collaboration/stage-queries.ts` and `stage-source-cache.ts` — workspace-wide private evidence generation, cancellation and scrubbing.
- `server/internal/service/collaboration_stage_evidence_read.go` and related review/QA/artifact helpers — concrete same-transaction read fence selection.
- `server/pkg/db/queries/delivery_evidence_read.sql` and `agent_artifact_execution.sql` — exact shared read and signer/member SQL.
- Retained private `r19-metric-freshness-fixture-r01/runtime/four-scenarios-r01/result.json` — immutable original four-case failure.
- Retained private `r19-metric-freshness-fixture-r01/evidence/four-scenarios-read-diagnostics-r01.jsonl` — actual reviewer response/query chronology.

## External references

No external reference or API lookup was needed. SQL definitions, installed pgx tracer interfaces and the current repository specification are the primary evidence. No network or database was accessed.

## Related specs

- `.trellis/spec/server/collaboration-stage-approvals.md:58–106` — RR snapshot, read/read fences, writer invariants and authoritative R1 denial before secondary hydration.
- `.trellis/spec/server/database-test-isolation.md` — fresh explicitly owned test databases and exact package/test admission.
- `.trellis/spec/server/verification-environment.md` — source/configuration binding and independent final inventory.

## Caveats / Not Found

- R19's exact conflicting member-row holder and precise browser response-consumption order were not captured by this read-only research.
- The UI display gap is proved by source control flow and matches retained authoritative-denial responses; a fresh deterministic RED/GREEN regression is still required.
- No code was changed, no tests were run, and no parent matrix or acceptance state was modified.
- The original R19 SQLSTATE, generic errors, and all prior failed runs remain retained; one cannot be used to overwrite or explain another without additional evidence.
