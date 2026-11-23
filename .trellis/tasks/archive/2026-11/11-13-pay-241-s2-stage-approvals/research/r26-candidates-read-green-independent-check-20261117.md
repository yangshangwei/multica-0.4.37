# R26 independent candidate-read GREEN review

**Verdict: `ACCEPTED_R26_CANDIDATE_ONLY_NORMAL_RACE`. Open candidate-scope findings: 0.**

Reviewed on 2026-11-17 by `/root/r26_red_boundary_check`, continuing the independently accepted R26 RED boundary. The two-file candidate repair and its exact normal/race evidence pass this scoped review. Main owns ROOT integration and subsequent ROOT compilation/vet, progress and spec updates. Full completion-preview, managed `boundSubmission`, R38, the original four scenarios and S2 acceptance remain open.

The checker wrote only this new Markdown/JSON pair and private `green-independent-check` support. No product/test source, configuration, old evidence or history was changed; no tests, builds, SQL, API calls, process signals, Git operations or subagents were executed.

## Exact change and preservation

The private GREEN source is `r26-m1-candidates-read-r01/source-green-r01`. Its5,048 paths equal the RED path set. Exactly two product files differ; all5,046 protected inputs match RED bytes/modes, including the unchanged674-line test SHA256 `086a559dcb420a41a88dcd5e499edae2f76325b49b4b0266bcf4704865a46ce9`. Existing SQL/generated code, fixtures, current-credential helpers and actual writer files are unchanged.

| Product file | Accepted GREEN SHA256 |
| --- | --- |
| `server/internal/service/collaboration_completion_preview.go` | `07637fee75dc894a50dd32eaa2beee3254784f015f0096fdd8cd3b44410fd0fe` |
| `server/internal/service/delivery_completion_preview.go` | `2f9103c1bc3e4018cd6ec351b040085b7d76593faca25427e8a8774918353a75` |

The exact diff contains five selected dispatch replacements plus explanatory comments. Independent reconstruction of those replacements equals the complete GREEN bytes, so no other logic change is hidden within the two files. The retained Go AST record agrees: three changed bodies, seven unchanged functions, unchanged signatures and non-function declarations, and no added/removed function.

| Preparation step | Accepted dispatch |
| --- | --- |
| Root issue row | Existing `LockDeliveryEvidenceIssueRead`, same root/workspace |
| Root collaboration row | Existing `TryLockCollaborationStageRootShared`, same root |
| Ordinary source child | Existing `LockDeliveryEvidenceIssueRead`, same source/workspace |
| Repeated ordinary child preparation | Existing `LockDeliveryEvidenceIssueRead`, same issue/workspace |
| Each candidate's artifact | Existing `readDeliveryArtifactInTx`, same transaction/access/issue/execution |

The issue query remains `FOR SHARE NOWAIT`. `TryLockCollaborationStageRootShared` locks `issue_collaboration` with `FOR SHARE NOWAIT`; it introduces no dependency on a configured stage task. Its returned lifecycle scalar is not substituted for access: the existing current access/admin recheck follows both root fences. Source workspace, exact parent/project and ordinary-child checks are preserved.

The artifact dispatch selects the existing shared issue/principal, approval and execution queries, closing the three nested exclusive acquisitions together with the four issue/collaboration acquisitions. Canonical spec/request/receipt, producer identity/generation, current manager/owner/invocation eligibility and finite expiry checks use the same implementations. A new marker is unnecessary: candidate enumeration directly calls the existing read helper. The existing R4 marker and all constant-false actual writer wrappers remain unchanged.

## Interface, errors and lock order

The package caller map confirms each private root/issue helper is called only by its existing candidate GET and pure preview POST service entries. Actual authorization creation/consumption, lifecycle, Stop, R38 and stage writers do not call either helper. `EvaluateDeliveryEvidenceInTx`, `lockDeliveryArtifactInTx`, `completionCollaborationAccess`, `completionAdminNomination`, completion prepare/finish, stage completion, lifecycle/handoff/admin and status-writer source files are byte-identical to RED.

Workspace → current users/credentials → status catalog → root issue/collaboration → source child → nested artifact order is retained. Shared rows still exclude actual updates/deletes, as the real two-order writer controls demonstrate. No lock is omitted, no transaction is replaced, and no retry or credential fallback is added. The five-second read,250ms artifact timeout, default candidate isolation, existing R4 repeatable-read isolation and ten-minute package budget remain unchanged.

Human-only access, source membership, parent/project binding, malformed/actor-bound cursor checks, visible-set pagination,200-row scan limit, schema/policy readiness and final credential/database-clock checks are unchanged. No new DTO, field, HTTP flag, public read token, dependency, SQL, migration or template/configuration update is required. Error classification and empty/partial-page refusal behavior are preserved.

The changed ordinary candidate primitive is actually exercised by every nested collaboration candidate leaf. Its direct handler and boundary parsers are unchanged. No additional ordinary-candidate test is required for this exact delta. Preview preparation shares these helpers, but its additional observer/admin/session/attachment/stage and managed-submission paths remain separate; successful candidate reads do not prove successful preview.

## Actual RED → normal → race evidence

All three runs retain the same4 top-level /11 leaf /15 named identities and exact test bytes. Normal and race are separate fresh admitted service allocations, each with its own complete environment and permit. All5,051 references in each GREEN/race permit match. Commands, cwd, profile hashes, database selection and actual before/after database identities agree; ambient inheritance and API pool/publisher/auth-mode overrides are absent. No suite was rerun by the checker.

| Run | Named PASS /FAIL /SKIP | Result SHA256 |
| --- | --- | --- |
| Original RED |9 /6 /0 | `f6ab208639390515e012d80349ed63649963c02d14a9b856914e03a0ad72aa93` |
| Normal GREEN |15 /0 /0 | `c36bf7b021d6df51d3d31ec8a8ce5432ebaefa30cfed1267968ec72117c9c284` |
| Same-source race |15 /0 /0 | `57e05bb390a6b662f5e8af687d1838b057e463a5be69a7db03e823917e784043` |

Independent parsing of all167 JSON lines in each passing stream found exactly15 runs and15 named PASS terminals, one package PASS, and no parse/shape errors, pending, missing, extra or duplicate identities. The normal raw SHA256 is `692e713b3b4246a7d3c45242a6d665c64cdf18830adb4946f19b497135b18426`; race raw SHA256 is `ca5957cdd185fbb7a520f3fe8904eaeca310be1697a67a2febac067d09e657c1`. Counts are not added across modes as different business cases.

| Concurrent leaf | Normal backend pair | Race backend pair |
| --- | --- | --- |
| R4 held → candidate |54145 /54152 |56020 /56027 |
| Candidate held → R4 |54155 /54161 |56030 /56039 |
| Same-creator candidate siblings |54164 /54171 |56042 /56049 |
| Different current-manager siblings |54173 /54180 |56052 /56059 |
| Actual observer held → reader refusal |54182 /54188 |56062 /56069 |
| Reader held → actual observer refusal |54190 /54196 |56072 /56080 |

Each pair is nonzero and distinct. The unchanged source and successful witness labels establish that the second actual method returned and committed/rolled back while the first transaction still held its locks. Cleanup released, cancelled, drained and rolled back handles before the final snapshot. All four read-coexistence leaves succeed in both modes, with exact baseline-equivalent metadata and no PgError.

Only the two legitimate writer-control PgErrors remain in each passing stream: writer-first rejects the shared candidate `LockDeliveryEvidenceIssueRead` with actual55P03/domain409; reader-first rejects unchanged `LockLifecycleIssue` with actual55P03 before `LockArtifactUsers`. Both raw SQL hashes match the generated queries. The real observer must pass before its writer-first hold, and fresh reads recover after release. Wrong-workspace child, external participant without source membership, disabled account, expired credential and demoted artifact manager controls all retain their exact refusal/empty-page behavior.

Each mode records22 matching before/after snapshot pairs, complete rows/counts for72 relations plus the original stage-exit snapshot. Each has zero final other database connections, the unchanged853-entry ledger and663 identical valid/ready/live public indexes. The fixture-lifetime configuration invalidation count grows21→43 and knowledge invalidation count18→51 in each fresh allocation; these retained setup/cleanup histories are outside the per-read72-relation snapshot. Equality of all211 tables or old historical rows is not claimed. The existing fixture's real packaging and metadata checks are not a new tar-byte audit, because it does not expose an archive path.

## Findings (fixed)

- **R26-C01, fixed by the implementation owner and independently verified:** The candidate root helper, repeated ordinary child preparation and artifact enumeration formerly excluded authorized R4/sibling readers. The five existing shared-read dispatches now pass both real read orders and sibling identities while preserving the original actual writer and authority refusals. No checker self-fix was needed.

## Findings (not fixed)

No open defect was found in the candidate increment. The previously recorded managed `boundSubmission` gap and additional full-preview/R38 paths remain outside this accepted scope. Their real regression and bounded follow-up are still required before claiming full preview/S2 acceptance; they were neither repaired nor newly accepted here.

## Verification

- **Lint/static analysis:** implementation owner's retained gofmt and `go vet ./internal/service` both pass; source/profile and empty logs match. The checker did not rerun these commands.
- **TypeCheck/compile:** retained `go test -c ./internal/service` passes without TestMain/database execution. Its86,047,314-byte binary SHA256 `16fa5594a6b2b78b06f450806b28d9fb8f41a8d5320b4d6a161ac96b934bde22` was independently rehashed. Main's normal/race commands were `go test`; execution of this particular compile-only binary is not claimed.
- **Tests:** actual normal and race each15 PASS /0 FAIL /0 SKIP, with exact complete identity, real writer/current-authority controls and preservation evidence independently reconciled.
- **Independent source/evidence audit:** PASS. Private `green-independent-check/audit-result-r01.json` SHA256 is `f59d106802d208aec3fe3c87b1690a18f876f06ba496f0342128f572932851d6`.

The sealed plan's existing R4/read/writer-control sources and their actual default symbols are unchanged, so retained accepted observations may remain retained; no old totals are added and no broad A/W/Q replay was performed. Main can hash-guard integration of the two accepted product files and unchanged new test, then perform its ROOT compile/vet and document the accepted read-preparation exception in the relevant spec. Existing source/policy/credential/gate/clock and real writer contracts remain authoritative.

R24 M1 and R25 QA historical holders remain **UNPROVED**. ROOT integration, full preview, original four scenarios, A01–A12, full S2, parent24, S3, real S4 and final S5 acceptance are not established by this candidate review.
