# Knowledge closure-readiness — #6 and #7

**#7 has 21 independently reviewed dispositions and no open confirmed product finding. It can close without waiting for #6's real-model/release gates, after the coordinator's AC1/L7 clarification and the sole-owner actual complete-check prerequisite. It is not closed by this report.**

**#6's deterministic feature/G2 evidence is verified within its recorded layered scope. Joint-candidate reconciliation is conditional; real RK1-K1–K3, ordinary E0b, enablement and separately owned platform/account/release tracking remain open.** `verified` means evidence exists for the stated scope; `conditional` means an explicit local condition remains; `open` means the required evidence/input is absent. No status, source, adapter, launcher, parent AC0/AC3, budget or runnable flag was changed. No suite, DB, service, browser, container or Provider operation was run.

## Ownership and source

The shared full-check owner is first-closure thread **01a19c0d-cce1-73e3-99ee-2c3c9e9239ba**, task `10-30-collaboration-fix-closure`. Its actual success, with required named knowledge results and explicit skips, can satisfy the remaining local joint-check condition. This thread starts no duplicate check and uses none of that owner's resources. A future result is not counted before it exists.

Final candidate `64a9994ec549e8dff73ab14456dc9a9f4b63f7c7` retains product source ID `9f72fc214b8e81f6fc3c1969be5e3304b7f158f60a6f7305f1531d55574d80ee`, 7,960 tracked regular files and server tree `a1b81d7a86630ac1d1b87c7712c17e30765fd449`. Read-only Git comparison shows no product change from final639 or the CCG2 E6 product; the audit merge `6cb9fcb36` is an ancestor. Product source under `6aed33aad` and initial audit-task commit `4e3b61500` is identical. The accepted RKI6 review remains byte-identical (JSON SHA-256 `109bdcbadd1f4edd11da9ed82137e75a50a67e0b61c9679589f8094a313dd442`).

This review independently checked all 82 retained default Go raw-file hashes and named counts:20,852 pass events, 0 fail, 22 explicit skip and 9 compile-only packages. The canonical audit/L7 tests below have actual final639 pass terminals. Ordinary and Host raw composite hashes/counts also match. These are existing results on unchanged bytes; no checks were repeated. The whole third attempt remains **exit2 / failed_not_accepted**. Older service 3319 pass / 3 fail / 18 skip and migration 523 pass / 13 fail remain failures. Some `green.txt` files contain earlier failed/setup attempts; filenames alone are not pass evidence.

## #7 — all 21 audit dispositions

Claim correctness/security/test-validity, launch, write, web and combined integration reviews are present with pass/pass-with-notes and zero blocking findings. Their scope/commit/hash manifests remain distinct. CT1's root recheck, IW1's independent recheck, migration fixture, skill entry and plan assertion follow-ups were also read. All original 21 IDs occur in the union of the independent lane audit lists. L7 is existing correct behavior, not a fabricated repair; L13 is the documented terminology/parity exception.

| ID | State/disposition | Result | Final canonical regression | Independent lane report |
| --- | --- | --- | --- | --- |
| H1 | verified; repaired | safe invocation remains ordinary; unsafe session selectors refuse; managed settings precede trailing value arguments | `server/internal/service/task_knowledge_invocation_test.go:33` — TestKnowledgeGateOffSafeArgsKeepOrdinaryClaim | claim-independent-review/review.json |
| H2 | verified; repaired | unreachable prior workdir rebinds an untouched preparation to a fresh generation; stale bindings refuse | `server/internal/daemon/knowledge_context_claim_test.go:346` — TestKnowledgeUnreachablePriorWorkdirRebindsFreshGeneration | launch-independent-review/review.json |
| H3 | verified; repaired | complete keyset/session and namespace history; deep exposure/corruption still refuses | `server/internal/service/task_knowledge_history_growth_test.go:47` — TestKnowledgeResumeReadsCompleteLongSessionHistory | claim-independent-review/review.json |
| M1 | verified; repaired | transient claim errors requeue; domain/finalize refusals settle without retry loops | `server/internal/handler/daemon_knowledge_test.go:342` — TestKnowledgeClaimTransientHistoryErrorRequeuesTask | claim-independent-review/review.json |
| M2 | verified; repaired | retry whole namespace transaction up to 1.5s without holding locks; exhaustion retains terminal busy | `server/internal/service/task_knowledge_concurrency_test.go:297` — TestKnowledgePreparationWaitsOutSiblingNamespaceHolder | launch-independent-review/review.json |
| M3 | verified; repaired | three concurrent metadata history indexes and complete namespace key lookup; catalog/negative control corrected | `server/internal/service/task_knowledge_history_growth_test.go:175` — TestKnowledgeRuntimeHistoryIsIndexDriven | claim-independent-review/review.json |
| M4 | verified; repaired | decision input survives unchanged refresh; identity changes reset it while private body is erased | `packages/views/collaboration/knowledge-panel.test.tsx:112` — keeps typed decision input and review confirmation across an unchanged refresh but resets them for a new revision | web-independent-review/review.json |
| M5 | verified; repaired | only own proposal completion closes its draft; recovery and unrelated decisions preserve new drafts | `packages/views/collaboration/knowledge-panel.test.tsx:354` — a review decision leaves an open proposal draft intact and only the panel's own proposal closes it | web-independent-review/review.json |
| L1 | verified; repaired | DB clock expiry returns sealed409; non-expiry NoRows is not synthesized as expiry retry | `server/internal/service/collaboration_knowledge_concurrency_test.go:408` — TestKnowledgeApprovalExpiryUsesDatabaseClockNotApplicationClock | write-independent-review/review.json |
| L2 | verified; repaired | expiry CHECK violation is invalid request; unrelated database defects retain server-error semantics | `server/internal/service/collaboration_knowledge_authority_test.go:186` — TestKnowledgeProposalExpiryCheckViolationIsInvalidRequest | write-independent-review/review.json |
| L3 | verified; repaired | final-send authorization runs before each write deadline | `server/internal/realtime/collaboration_knowledge_test.go:130` — TestKnowledgeSlowFinalSendAuthorizationDoesNotSpendWriteBudget | write-independent-review/review.json |
| L4 | verified; repaired | timeout attempts are recorded and later hints progress under bounded consecutive failures | `server/internal/service/collaboration_knowledge_invalidation_test.go:704` — TestKnowledgeTimedOutHintDoesNotBlockLaterHints | write-independent-review/review.json |
| L5 | verified; repaired | PAT last-used contention waits briefly while revoke/write serialization remains | `server/internal/service/collaboration_knowledge_credentials_test.go:142` — TestKnowledgePATWriteWaitsOutLastUsedUpdate | write-independent-review/review.json |
| L6 | verified; repaired | descriptor decode failure refuses only that task; siblings and strict ledger versioning remain | `server/internal/daemon/knowledge_context_claim_test.go:719` — TestKnowledgeClaimDescriptorDecodeFailureRefusesOnlyThatTask | launch-independent-review/review.json |
| L7 | verified; not_reproduced_test_covered | not reproduced; existing correct Started-failure cleanup is locked by actual process/daemon tests | `server/pkg/agent/knowledge_launch_test.go:772` — TestKnowledgeClaudeStartedCallbackFailureStillReportsTerminated | launch-independent-review/review.json |
| L8 | verified; repaired | refresh waits for the source grant; source/authority/validation errors preserve draft and stay distinct | `packages/views/collaboration/knowledge-panel.test.tsx:491` — submit waits for a refreshing source list instead of reporting a changed source | web-independent-review/review.json |
| L9 | verified; repaired | UTF-8 byte limits give field errors for proposal and decision reason | `packages/views/collaboration/knowledge-panel.test.tsx:532` — attributes over-byte summary, conclusion and evidence to the field instead of a source failure | web-independent-review/review.json |
| L10 | verified; repaired | expired later-page authorization resets to page1 rather than recurring generic errors | `packages/views/collaboration/knowledge-panel.test.tsx:221` — restarts at the first page when a later page loses its authorization boundary | web-independent-review/review.json |
| L11 | verified; repaired | unavailable interface stops interval polling until explicit refresh | `packages/views/collaboration/knowledge-panel.test.tsx:211` — stops interval polling an unavailable knowledge interface until an explicit refresh | web-independent-review/review.json |
| L12 | verified; repaired | same-source recovery works beyond loaded page; foreign recovery stays outside; dismissal is local only | `packages/views/collaboration/knowledge-panel.test.tsx:390` — recovering a request from the same source opens it even when it is outside the loaded list page | web-independent-review/review.json |
| L13 | verified; terminology_repair | workspace/issue terminology aligned; locale key parity preserved | `packages/views/locales/parity.test.ts:73` — locale namespace key parity and knowledge terminology | web-independent-review/review.json |

Current source also has the named own-lost-proposal test (`knowledge-panel.test.tsx:342`), separate authority/source error tests (`:463`,`:473`), and same-source recovery beyond the loaded page (`:390`). They resolve recorded web follow-ups; old design/progress paragraphs remain historical evidence, not newly inferred product gaps. The authoritative `.trellis/spec/views/frontend/collaboration-knowledge.md:106` already requires the authorized-detail fallback when list membership is unknown and matches the current regression. Claim history tests retain complete reads and fail-closed exposure; controlled plan eligibility/catalog checks do not establish production-size default-planner performance.

## #7 — AC1–AC9 and independent closure

The original #7 PRD expressly excludes real Provider acceptance and whole `make check`; its ACs are local code, tests, migrations and spec contracts. The later authorized continuation adds the coordinated actual complete-check condition. Neither the PRD nor that continuation makes #7 wait for #6's model comprehension or parent installer/account/release gates. After the conditions below, the coordinator can close #7 with all 21 dispositions and the retained L7/skip/history qualifications. #6 and parent acceptance stay separate.

| AC | State | Verified evidence | Condition or limit |
| --- | --- | --- | --- |
| AC1 | conditional | 20 actual defect/terminology dispositions have retained red/control and regression evidence; L7 correct behavior has2 retained pass names and3 final default pass names. | Apply the proposed L7 clarification; never manufacture baseline red or classify every historical green-file segment as passed. |
| AC2 | verified | H1 current safe OFF ordinary/nil descriptor and unsafe selector tests pass; managed argv ordering reviewed. | No extra run; retain unsupported/legacy/exposure boundaries. |
| AC3 | verified | H3 long histories, deep exposure/corruption and CT1 tied cursor controls are present and passed; all reviewed SQL bytes merged. | Keep O(N) namespace cost and lack of production-scale default-planner benchmark explicit. |
| AC4 | verified | M1 requeue and M2 bounded retry regressions pass. | Beyond the1.5s budget busy remains terminal by accepted design; no indefinite or business-input replay promise. |
| AC5 | verified | H2 real HTTP/daemon rebinding and seven acceptance mutants reviewed; stale binding cannot launch. | No same-source rerun needed. |
| AC6 | verified | M4/M5/L8–L12 shared UI tests cover refresh, exact-identity reset, own/foreign/lost recovery and field attribution; security cache erase remains. | Retained TS/current code and lane review; no assertion that every interaction was rerun in native UI. |
| AC7 | verified | 761–763 are reviewed single-statement concurrent indexes, no FK; down guards/cleanup registration and generated SQL retained; migration fixture and plan rechecks pass. | Historical523pass/13fail migration attempt stays failed. |
| AC8 | conditional | Retained final639 Go 82 package hashes/counts reconstruct without mismatch:20852 named passes/0fail/22skip,9 compile-only; current product bytes equal final64. TS 13082/static 15 passed in the failed whole-check attempt. | Only shared owner 01a19c0d-cce1-73e3-99ee-2c3c9e9239ba may supply actual new full-check exit0 and required named knowledge results; skips remain explicit. |
| AC9 | verified | Claim/write/web/launch docs and built-in guidance/source maps were reviewed and merged; skill-entry template recheck passes 10 named events. | Retain original service failures and per-commit scope; no source/spec edit performed here. |

### Proposed exact AC1 clarification

The current AC1's universal failing-baseline wording conflicts with the already accepted L7 not-reproduced disposition. This is a documentation reconciliation, not a request to invent a failure or alter the correct process cleanup. The coordinator owns applying it:

> 每个已复现的产品缺陷 ID 都有具名回归测试：在 6aed33aad 的对应未修复产品代码上失败，修复后通过，并保留实际红绿运行输出。L13 用 locale parity 测试和术语核对代替。L7 经设计和独立分道复核未复现，不要求伪造失败基线或新增产品修复；以 TestKnowledgeClaudeStartedCallbackFailureStillReportsTerminated 与 TestKnowledgeStartedObservationRefusalStillConfirmsProcessExit 的实际进程退出、Terminated 恰好一次、无再次授权断言作为测试处置，相关 Host lifecycle 回归保留。原审计主张和所有历史失败样本原样保留；L7 只能记为“未复现，已有正确行为由测试覆盖”，不能计作已复现并修复的产品缺陷。

Basis: #7 `design.md:312`, `research/launch-independent-review/review.json` (L7 explicitly audited; no blockers), and `research/evidence/L7-takeover-green.jsonl` (two named passes). Final canonical tests are:

- `server/pkg/agent/knowledge_launch_test.go:772` — `TestKnowledgeClaudeStartedCallbackFailureStillReportsTerminated`: actual process starts, failure becomes refusal, exactly one waited `LaunchManagedEnded` for matching PID/instance, and no second permit/start.
- `server/internal/daemon/knowledge_context_claim_test.go:850` — `TestKnowledgeStartedObservationRefusalStillConfirmsProcessExit`: production runner confirms one exit, terminal observation matches process/attempt, and authorization remains one.
- `server/pkg/agent/launch_lifecycle_test.go:358` — existing `TestHostLifecycleStartedFailureStillWaitsAndBlocksFallback` retains the Host form of the same contract.

All three have retained final639 default pass terminals. No L7 baseline failure is asserted, and the original audit claim remains retained.

## #6 — AC1–AC11 and G2

The #6 PRD distinguishes implementation/application delivery from real-model/platform/final enablement. `implement.md` §7 keeps the task in_progress to track the independent real-model/release boundaries. Local design findings K1–K5 are not the same identifiers as real experiments **RK1-K1–K3**.

| AC | State | Verified evidence | Condition or limit |
| --- | --- | --- | --- |
| AC1 | verified | A1/A2 external-handler/hidden-target policy tests plus original deterministic review and pending-v2 CCG1 assertion prove unapproved state cannot affect contributions. | Real RK1-K1 understanding still open. |
| AC2 | verified | Human source Read plus target-role/lead matrix, actor/machine refusal and both commit orders independently reviewed. | No Agent/machine or runtime owner substitutes human approval. |
| AC3 | verified | Immutable lineage and atomic correction matrix; CCG1 actual second fake stdin contains v2 and excludes v1. | Retain version/hash/source evidence; no real-model proof. |
| AC4 | verified | Shared UUID namespace and original-actor content-free recovery matrices; ordinary/Host lost-response composites pass. | Do not mint replacement IDs or sum overlapping runs. |
| AC5 | verified | Source availability/purge/delete and expiry/withdraw/supersede matrices; CCG1 withdrawal gives409 and no third process; Host invalidation composite passes. | Historical lawful disclosures are not retroactively erased. |
| AC6 | verified | Ordinary surfaces, originator/accountability separation, source participation and squad root/member/continuation layered evidence retained. | K3 real entry coverage is unimplemented and must be preregistered per entry. |
| AC7 | verified | Final639 ordinary27 and Host17 pass per normal/race; CCG1 exact fake stdin and CCG2 deployed ON/OFF provenance bind relevant unchanged final64 bytes. | Fake executable input proves application delivery, not model comprehension. |
| AC8 | verified | ExactResumeThenFreshReplacement, LegacyCannotReuseExposure and refusal composites; history/audit fixes reviewed. | RK1-K2 actual native Provider reuse/revoke/original-key evidence remains open. |
| AC9 | conditional | Original 7 Web/1 native macOS layered UI acceptance, current shared UI/cache regressions and F3 rechecks retained. | Reconcile required final-candidate Web/Desktop identities with sole-owner complete check; original platform counts are not all post-audit interactions or installer/account evidence. |
| AC10 | conditional | Original 747–760 and new761–763 migrations, sqlc/static/full-package evidence and built-in no-skills rules reviewed; current source equivalence verified. | New shared complete-check success remains the local joint prerequisite; keep all historical failure/skip boundaries. |
| AC11 | verified | F1–F4 parent KEY SHARE/member SHARE/independent revision and no-source-relock fanout matrices reviewed; IW1 DB-time budget followup passes. | Do not claim untested production-scale planner or arbitrary retained-history workload guarantees. |

G0/G1 retain their frozen deterministic dependency scope. G2's ordinary issue/comment/Chat/quick-create/autopilot, exact session/exposure, pointer writers, independent disclosure and per-launch authorization have reviewed application-input evidence. G3 retains allocated original ownership/environment evidence. Current joint-candidate reconciliation awaits the single complete-check owner; these gates do not prove Provider understanding or grant production enablement. Production knowledge remains default false.

### Existing actual-input, correction/withdrawal and deployed-switch evidence

- **Ordinary composites:** each normal/race 27 named passes, 0 fail/skip,14 top-level identities. They exercise actual HTTP/WS/daemon input, trusted consumers, original-key recovery, exact resume/fresh replacement and refusal. These are the same identities rerun under race, not54 new independent cases.
- **Host composites:** each normal/race 17 named passes, 0 fail/skip,9 top-level identities. The 17 includes CCG1; its extra1/1 results must not be added as a new independent case to that set.
- **CCG1:** `knowledge-crosschain-correction-20261102/independent-review/review.json` is pass-with-notes. The exact source file `server/cmd/server/collaboration_knowledge_correction_composite_test.go` hashes to `4895bf49e1cdd94aa9d29d39f623a6ddd059d3865beadd3ede3411a5f6ac1f41` in final64. First fake input gets approved v1; a real human HTTP v2 proposal stays pending and does not displace v1; exact v2 approval makes a separate physical input contain v2 only; withdrawal at the next authorization returns409 with **no third process**. This satisfies local correction/actual-input/withdrawal coverage and claims no real-model journey.
- **CCG2:** `knowledge-deployed-probe-20261102/independent-review/review.json` is pass-with-notes. The real deployed OFF and ON environments each have 1 actual pass, 0 fail/skip, distinct source-env fingerprints, API identities, binary hashes and supervised Host proof. OFF has no knowledge section/contribution/events or persistence flag; ON has the exact approved contribution, one claim/preparation/authorization and `--no-session-persistence`. Both use test-owned Providers. Final64 has identical relevant product bytes and all four exact harness-file hashes. Reuse this existing evidence within its scope; do not rerun it merely to obtain another count.

The original 7 Web/1 native macOS layer remains its original source/interaction coverage. Current audit UI fixes have independent shared-code regression evidence; final required Web/Desktop identities must be reconciled with the sole-owner result. No UI→real Provider journey, installer upgrade or universal native coverage is inferred.

## Real knowledge framework — precise current gaps

There **is** a reviewed runtime RK1 framework:17 files, SHA-256 `5eaea44f480f8e94a9a90d35025ca2d68d76c895c5bf70fa902da37c5bed94c9`; its launcher hash `3bc7962c996143200b1834e9246ec215e8a0d3e03fc38e832abd2eddb1769c99` matches registration. RKI1–5 source review and the accepted RKI6 normal/race result review are present. This supersedes stale blanket “no framework” wording.

The executable table at `server/cmd/server/rk1_scenarios_recall_test.go:223` contains only E0a/H1/H2/H2b/H3a/H3b/H5/H6. K1–K3 and ordinary E0b are absent. `newRK1Fixture` forces knowledge false at `rk1_fixture_test.go:75,86`; `run_rk1.py:269–271` derives dispatch/budget only from the runtime addendum. The old registry's K1–K3 entries therefore cannot be launched by flipping a flag or borrowing runtime calls. Paid source ID remains empty and runnable remains false.

| Missing input | State | Current fact | Minimum precise prerequisite |
| --- | --- | --- | --- |
| K-FRAME | open | Existing17-file runtime framework and accepted RKI6 fake results are present. K1/K2/K3/E0b are absent from executable scenario map, and newRK1Fixture explicitly disables knowledge. | A separately reviewed knowledge-capable fixture/scenario dispatch and ordinary-native entry. Preserve real router authorization, actual handoff/recording/knowledge grants, input/bind/proof/ledger capture and fake/stand-in no-provider guards; do not merely flip existing runtime registration flags. |
| K-REGISTER | open | Registry has RK1-K1=4 launches, RK1-K2=2, K3 launch count per actually wired entry; runtime launcher admits only runtime_addendum scenarios. | Freeze exact K1 state schedule (including which withdrawal/expiry paths are actually measured), K2 same-native-session/revoke/recovery schedule and zero-launch refusal expectations, and K3 entry table: issue, comment, Chat, quick-create, autopilot, root, member, continuation. Define exact call counts/order/cursor/stop rules before requesting or using any knowledge budget. |
| K-BUDGET | open | Knowledge K1–K3 and ordinary E0b have no allocated calls; runtime 25 and original 14 are separate. | Explicit knowledge allocation covering registered K1 four + K2 two + final K3 N launches, with E0b one separately accounted if selected as prerequisite. Refusals must use zero paid launches. Old proposed14/29 aggregate estimates are not authorization; no transfer from runtime 25/original 14. |
| K-BIND | open | Existing runtime framework/launcher hashes match; paid source_id is empty and runnable=false. | After the sole-owner local complete check actually succeeds, freeze final product/overlay, new knowledge framework/launcher, actual API/daemon/CLI/artifacts, applied configuration hashes, requested/observed model, thinking/tier, isolated DB and run/session/attempt/output identities. No automatic paid binding from fake source equivalence. |
| K-ACCOUNT-WINDOW | open | Registration names a Claude CLI/model/gateway and production profile exposure; current scope authorizes local work only. | Verify the selected permitted Provider/account source and nonsecret identity/configuration fingerprints within applicable authority; explicit exclusive gateway window and existing upstream recovery evidence, serial launch ownership and first-incomplete stop. No new Provider authorization is inferred. Runtime H5 already uses self-controlled Multica owner/contributor with one Provider account; separate AC43 two-Provider-account acceptance remains parent-owned. |
| K-OBSERVE | open | Fake/raw application-input evidence is complete for its scope; real K1/K2/K3 observations are absent. | Run only after all above inputs/authority exist; preserve actual per-launch prompts, readable tool results, semantic criterion verdicts, exact native-session/exposure/recovery/refusal evidence, numbering and consumed counts. A test exit0 or configured model must not replace actual model/visibility evidence. |

## Budget and gate separation

| Ledger | Current allowed accounting | Next boundary |
| --- | --- | --- |
| Original frozen E0–E6 | remaining 14; original invalid samples/numbering retained | E3 after trial2, then E4→E5→E6 under its existing owner/window; no source/model/slot substitution |
| RK1 runtime addendum | approved cap 25, consumed 0; H2b included, H6 counts two launches per trial | Runtime-only scenarios; current paid entry not runnable; final binding and authorized exclusive window still required |
| Knowledge RK1-K1–K3 / ordinary E0b | allocated 0, consumed 0; K1 registered 4, K2 registered 2, K3=N not yet fixed; E0b 1 separately excluded from runtime 25 | Knowledge budget must be explicitly allocated after exact scenarios/counts exist; no transfer from either other ledger |

Older proposed minimum 14/full-without-K3 estimate 29 is a proposal, not a grant. Runtime H5's two self-controlled Multica actors with one Provider account are already clarified by the addendum; parent AC43's separate Provider-account/quota evidence remains open and is not imported as a new K1/K2 implementation requirement.

Passing the future local full check can close the local complete-check prerequisite. It cannot supply real K1–K3 observations, allocate calls, make the paid entry runnable, enable production knowledge, restore the incident database, or close installer/account/release acceptance. This report creates no Provider authorization and changes no original/global ledger.

## Evidence and verification limits

The companion JSON records all 21 dispositions, both AC matrices, source/evidence SHA-256 values, independent/follow-up reviews, reconstructed82 Go-package counts and four composite inventories. The accepted RKI6 review is unchanged. Only this Markdown and its JSON companion were written. Lint/typecheck/tests were not run in this read-only review; existing results were parsed and compared. Source/status/launcher/parent AC0/AC3 and every failed/incomplete historical sample remain untouched.
