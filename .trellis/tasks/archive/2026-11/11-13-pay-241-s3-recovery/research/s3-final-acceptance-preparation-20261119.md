# Research: S3 final R01–R09 acceptance preparation

- Query: Reconcile the accepted PAY-241 S3 increments against the final R01–R09 requirements after Agent-output R03 and the ordinary UI CAS code check; identify reusable source-qualified evidence and the finite remaining UI/R09 gates.
- Scope: internal, read-only
- Date: 2026-11-19

## Findings

This is a preparation map, not an acceptance verdict. It does not mutate the parent matrix, promote TC-16, archive S3, or claim full PAY-241 completion. The current parent matrix remains **6 PASS / 11 PARTIAL / 7 NOT_RUN**. The exact previously accepted increment reports are retained as their own denominators; their results must not be spliced into a new run.

### R01–R09 disposition

| S3 requirement | Reusable accepted evidence | Remaining finite disposition |
| --- | --- | --- |
| **R01 timeout/failure/cancellation identity** | Archived timeout proof T01–T06 proves a test-owned 15-second execution deadline, both Stop/timeout orders, cross-process original-ledger/outcome/proof recovery, and a distinct explicit retry that retains the old failure. Controlled RUN/OUT rounds add current failed/successful/stopped result identity and owned termination. | Reuse only under the recorded T source and unchanged supervision/Stop algorithms. Do not relabel this as real-model quality or current live timeout acceptance. Run the conditional timeout regression only if the final freeze changes a listed host/Stop owner. |
| **R02 unknown/repeat/late result/output recovery** | Human upload/withdrawal A, ordinary notification N, controlled RUN L1/L2, and Agent-output OUT R03 jointly cover original identities, same-key replay, changed bytes, frozen output references, current-token denial and late-result refusal. OUT R03 is a fresh accepted 2-top/16-case round with 10 Provider turns, 12 output CLI invocations, 10 Agent files, 20 Agent references, 4 stored outcomes and zero recovery launches. | Keep OUT R01/R02 as immutable failed history. No additional controlled stimulus was identified for the accepted scope. Preserve the boundary that real-model quality and installed Main are separate. |
| **R03 offline comment recovery** | Batch B R01 is accepted: Web/Desktop 2 tops, 55 comments plus two controls per platform, two complete 50+7 page passes, two physical reauthenticated reconnects, persisted IDs/order/UI and no observer business writes. | Retain B's source and denominator. It proves persisted-data/UI convergence, not delivery of every WebSocket frame or a frozen snapshot during concurrent writes. |
| **R04 same-version field conflict** | F R06 proves collaboration authority-first 403/stale 409, ordinary child numeric API CAS, original receipts and explicit fresh continuation while preserving the optional versionless API contract. | **Open:** actual current ordinary UI picker requests from the supplied revision on both Web/Desktop, both commit orders, natural 200/409 `revision_conflict`, unchanged losing field, visible conflict/reload, explicit R+1 continuation and a separately labelled versionless API control. Static Core/Views proof and API-only controls cannot close this gate. |
| **R05 withdrawal/current authority** | A proves human/source/participant withdrawal and surviving participation. RUN proves 40 original-token authorized reads, 80 exact post-withdrawal denials, retained proof and zero recovery launches. OUT proves Agent-output write/read denial and frozen output identity. | Final review must bind each accepted action to its original capability/grant/attempt and content-free replay boundary. No broader S4 authorization or real-model claim may be inferred. |
| **R06 notification identity/recipient correctness** | A covers typed human/Agent mentions and collaboration notices; N covers literal ordinary `new_comment` stable-ID replay in normal/race (35 tops/57 names each); SUB R05 covers Subscribe/read/anchor/reconnect/reload/Unsubscribe; RUN/OUT cover controlled result recipients and late-event refusal. | Preserve the distinction between terminal collaboration recipients and current ordinary subscribers. N does not cover every generic event, every ordinary `mentioned` writer, or mixed-version old/new writers. |
| **R07 block/repair history** | K R08 is accepted on Web/Desktop: 2 tops/12 subcases, required reason/owner/condition/ETA, actual return/repair actor/time/restored status, 48 linked history items and 70 operations. | No new block implementation/body is identified. Retain the first legal-read 503 as historical diagnostic and keep the selected K source/reader fingerprints in the final inventory. |
| **R08 provenance/source/authority for all recovery actions** | The accepted T/F/B/A/K/N/SUB/RUN/OUT reports each bind original actor, issue/source, request/event/operation, revision, run/attempt/capability/grant, file/object, result and proof facts within their own denominators. | Perform one finite final provenance inventory over those action families, then add the new UI CAS requests (observed revision, actual PUT/response, loser conflict, fresh continuation) to the same source-qualified ledger. R08 is not the wider S4 provider/branch/CI/deployment audit. |
| **R09 source/environment/independent acceptance** | Each accepted increment has a dedicated source/config/database/program/client/resource closure. A read-only source comparison confirms **29/29 domain-owner files** still match the prior final-readiness snapshot and **95/95 host/supervision files** match accepted OUT server r03. OUT code/static checks and the ordinary UI code check are independently accepted within their scopes. | **Open:** integrate the accepted ordinary UI candidate into one coherent ROOT source, rebuild matching production Web and Electron renderer/preload, execute the complete two-platform UI round, and run the final full-scope quality/source/resource/gate/PID/listener/DB/retained-evidence reconciliation. |

### Ordinary UI candidate status

The ordinary UI implementation and owning-layer regression evidence are accepted as a bounded code/static increment: Core 64 named cases plus Views 216 named cases (280 total), whole Core/Views type checks, targeted lint, and exact 18-path candidate/source manifest reconciliation. The candidate implements supplied-rendered revision capture, existing confirmation token retention, common positive batch handling, and compatibility for missing/heterogeneous revisions.

The candidate is still isolated under the private UI campaign. The current ROOT has **zero** of the 18 product/test paths matching the private candidate and all 18 still match the pre-change BASE. The previous UI runtime round is a retained setup/locator failure with zero genuine PUT captures; it is not a missing-revision RED. The next runtime round must be admitted only after a coherent ROOT-based source/build/config allocation and must use natural `route.continue()` plus exact request/response association. No interceptor, cache reread, synthetic PUT or response fulfillment can supply the revision for the proof.

The UI acceptance denominator is two complete platform tops with priority-first and assignee-first orders, both original same-R genuine PUTs, one natural winner (200), one exact loser (409), visible conflict and authoritative refresh, one explicit R+1 continuation, and a separate versionless API control. Any failed or unstarted top remains retained and cannot be combined with a later passing top.

### Source compatibility check performed in this lane

The following checks were read-only comparisons against recorded manifests and current files:

- 29/29 final-readiness domain-owner fingerprints remain byte/mode equal.
- 95/95 host/supervision files remain byte/mode equal to the accepted OUT server r03 source.
- All 18 ordinary UI product/test paths in the private candidate match the accepted code-check report; all 18 current ROOT paths still match BASE, confirming integration is still pending.
- Existing output stored-outcome consumers (`decodeCollaborationStoredOutcome` and its ACK/records/checkpoint/retention/seal/handoff/test-report callers) are already covered by OUT's accepted normal/race 82-top/203-name owning-layer evidence. Do not add a broad duplicate suite merely because `89740ed14` is newer.

### Recommended final sequence

1. Admit a fresh coherent ROOT-based UI source/build/config/database allocation; keep the old hybrid/locator failure and all earlier UI reports immutable.
2. Bind the accepted eight product paths and ten owning-layer tests, the two new UI harness paths and the explicitly released shared-helper contract sync to the selected candidate. Reuse the accepted 280-case Core/Views evidence when that exact consuming graph remains unchanged; integration alone is not a reason to replay it.
3. Complete the matching production Web and Electron renderer/preload builds and verify source, package links, executable/config hashes, API/WS origin and native ownership before browser execution. A build already completed on that exact source may be bound rather than repeated.
4. Run the complete two-platform UI CAS round with workers=1, retries=0 and no early cutoff. Reconcile actual request identities, revision values, 200/409 bodies, conflict UI, fresh continuation, versionless control, protected parent snapshots, zero execution side effects, gate restoration, owned PID/listener closure and database connections.
5. After UI acceptance, integrate the exact accepted UI deltas without replacing other ROOT WIP; sync the executable Core/Views contract and perform the final affected-package quality check and finite R01–R09 provenance/source/resource reconciliation. The previous readiness map calls for nonmobile `pnpm typecheck`/`pnpm lint`; bind any usable existing exact-source checks and run only the still-uncovered consuming graph. Reuse unaffected T/F/B/A/K/N/SUB/RUN/OUT evidence by explicit source/config/action binding; rerun a conditional host timeout check only if its owner changed.
6. Keep S4 real inputs, S5 final performance and business signatures as separate parent gates. They are not extra conditions for archiving a correctly bounded S3, but they do prevent full PAY-241 completion.

## Files found

- `.trellis/tasks/11-13-pay-241-s3-recovery/prd.md` — S3 R01–R09 requirements and boundaries.
- `.trellis/tasks/11-13-pay-241-s3-recovery/research/s3-final-source-readiness-20261119.md/json` — prior source-qualified R01–R09 readiness map and exact resource inventory.
- `.trellis/tasks/archive/2026-11/11-13-pay-241-s3-timeout-proof/report.md` — accepted T01–T06 timeout/Stop/proof recovery increment.
- `.trellis/tasks/11-13-pay-241-s3-recovery/research/client-field-r06-independent-check-20261118.md` — accepted F R06 collaboration/API CAS and optional ordinary UI boundary.
- `.trellis/tasks/11-13-pay-241-s3-recovery/research/comment-batch-r01-independent-check-20261118.md` — accepted B R03 offline comment/reconnect batch.
- `.trellis/tasks/11-13-pay-241-s3-recovery/research/extras-a-r05-independent-check-20261118.md` — accepted A mentions/uploads/human withdrawal increment.
- `.trellis/tasks/11-13-pay-241-s3-recovery/research/block-runtime-r08-independent-check-20261118.md` — accepted K block/repair/history increment.
- `.trellis/tasks/11-13-pay-241-s3-recovery/research/ordinary-notification-green-independent-check-20261119.md` — accepted N ordinary comment notification deduplication.
- `.trellis/tasks/11-13-pay-241-s3-recovery/research/ordinary-subscription-r05-independent-check-20261119.md` — accepted SUB ordinary watcher journey.
- `.trellis/tasks/11-13-pay-241-s3-recovery/research/controlled-runtime-r03-independent-check-20261119.md` — accepted RUN controlled runtime result/revocation round.
- `.trellis/tasks/11-13-pay-241-s3-recovery/research/agent-output-runtime-r03-independent-check-20261119.md` — accepted OUT Agent-output round.
- `.trellis/tasks/11-13-pay-241-s3-recovery/research/agent-output-code-check-20261119.md` — accepted OUT owning-layer code/static checks.
- `.trellis/tasks/11-13-pay-241-s3-recovery/research/ordinary-ui-cas-code-independent-check-20261119.md` — accepted ordinary UI code/static check (280 names).
- `.trellis/tasks/11-13-pay-241-s3-recovery/research/ordinary-ui-cas-e2e-implementation-20261119.md` — UI harness static readiness and retained runtime NOT_RUN scope.
- `.trellis/spec/server/collaboration-runtime-recovery.md` — runtime recovery identity, cursor, proof and no-new-launch contracts.
- `.trellis/spec/server/collaboration-runtime-output.md` — Agent-output provenance, frozen outcome references and current-authority read/write contracts.
- `.trellis/spec/web/frontend/collaboration-recovery-qa.md` — production Web/native recovery evidence and source/resource closure requirements.
- `.trellis/tasks/11-13-pay-241-s3-recovery/research/s3-final-acceptance-preparation-20261119.json` — machine-readable fingerprints produced by this read-only lane.

## Code patterns

Current ROOT source owns the output contract: `server/internal/service/collaboration_runtime_output.go:47` decodes the unchanged original outcome separately from server-owned output IDs; `:70` freezes current-run/current-attempt output rows. `server/internal/service/collaboration_runtime_capabilities.go:480` compares that complete original outcome on replay rather than comparing the extended storage shape. The same decoder serves records at `collaboration_records.go:300` and `:412`, checkpoint retention at `collaboration_checkpoints.go:81`, handoff at `collaboration_runtime_handoff.go:448`, and test reports at `collaboration_test_report.go:284`. These selected owners match the previously accepted source fingerprints.

For the following UI citations, the source prefix is `/Volumes/artisan/.multica-acceptance/pay241-s3-ui-cas-20261119-u691amwe/source/client-ui-product-work-r02/`; ROOT remains BASE at this inspection:

- `packages/core/issues/mutations.ts:74` takes the supplied Issue, preserves an explicit token, restricts capture to ordinary priority/assignee changes, and never looks up a newer cache revision.
- Shared actions capture at `packages/views/issues/actions/use-issue-actions.ts:91`; board, table and child-detail leaf owners capture at `components/board-card.tsx:81`, `components/table-view.tsx:2157`, and `components/issue-detail.tsx:713`.
- `packages/views/issues/actions/run-confirm-gate.ts:93` and `:115` echo only an already supplied token into the existing intent. `packages/views/modals/run-confirm.tsx:174` preserves it during confirmation.
- `packages/views/issues/components/batch-action-toolbar.tsx:94` requires one known positive common rendered revision before attaching the existing common token; `:118`, `:151`, and `:167` retain it through direct and confirmation paths. Heterogeneous/missing observations remain the selected compatibility boundary.

Related specs: `.trellis/spec/core/frontend/state-management.md:73` (ordinary optimistic ownership); `.trellis/spec/core/frontend/issue-batch-outcomes.md:3` (single common request/partial outcomes); `.trellis/spec/server/collaboration-runtime-output.md:26` (frozen references/current authority); `.trellis/spec/web/frontend/collaboration-recovery-qa.md:82` (exact original identities and source-qualified current-client verification). The fresh final UI check must preserve those existing boundaries.

## External references

No external research was needed. The version/source references here are local accepted artifacts: timeout source `feb87aee7` plus its recorded overlay, controlled SUB/RUN API health `b1901a48f`, output API health `699d510b9`, output integration commit `89740ed14`, and the exact UI candidate manifest recorded by the 280-case code check. The integration commit must not replace an older binary's actual health stamp.

The older remaining-acceptance map's subscription/runtime aliases are superseded for this preparation by the exact `ordinary-subscription-r05-independent-check-20261119.*` and `controlled-runtime-r03-independent-check-20261119.*` paths above. Its Agent-output-open and UI-code-pending statements are historical. The later output R03/code check and ordinary UI code check close those specific incremental steps; they do not themselves close R04/R09. Existing report bytes were not edited.

## Caveats / Not Found

- No API, database, browser, Provider, build, lint, typecheck, process-control or Git command was run in this research lane; recorded passes are cited as prior evidence only.
- The final UI runtime body remains unexecuted after the retained setup/locator failure; no TC-16 or R09 promotion is justified by static evidence.
- Agent-output R03 proves the implemented run-bound output contract and its bounded runtime denominator; it does not prove real-model quality or installed production Main compatibility.
- Ordinary notification N is intentionally narrow: generic event writers, ordinary `mentioned`, mixed old/new servers, and post-insert broadcast durability are outside its guarantee.
- S4 external systems, S5 final performance, real payment/fulfillment, production release/rollback and business signatures were not found in the supplied evidence and remain separate parent work.
