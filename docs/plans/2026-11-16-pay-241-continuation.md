# PAY-241 Continuation Implementation Plan

> 历史推进记录：以下状态与验收数字保留于文档所述日期。相关任务已迁入归档目录，引用已更新；当前进度以归档任务的验收矩阵和后续记录为准。

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Finish the original S2 product acceptance with the verified read fences and control-reply codec, close its evidence gate, and advance the existing S3–S5 tasks through their recorded dependencies.

**Architecture:** Continue the existing role, source-authority, exact-version and transactional completion contracts. Reuse the accepted shared denial presentation and immutable-receipt shared read fence, then bind each actual runtime result to the source and products it consumed.

**Tech Stack:** Go, PostgreSQL/pgx/sqlc, React/TypeScript, TanStack Query, production Next.js, Electron and Playwright.

---

## Current continuation: R23

更新于 2026-11-17，Asia/Shanghai。现有 S2 继续 `in_progress`，不创建新任务。R22 原四场景已执行一次：**1 PASS / 1 FAIL / 2 NOT_RUN**。第二场景的回滚已在后端成功；客户端把合法的不透明条件版本令牌按数字解析，造成结果未确认。原失败、成功回执和请求身份完整保留，不能重放原回滚请求。

R23 仅修两份测试协议/回归文件。真实 RED 14 PASS / 2 FAIL，GREEN 16 PASS，strict TypeScript 与实际两文件 ESLint 通过；独立最终检查已封存 ACCEPTED、阻挡项0。本次已接入两条 ROOT 路径并同步新的隔离测试副本；接入后的16例、strict类型和实际两文件lint独立通过。已核对3359个新测试源输入、1961个依赖链接、1676个原前端产物、5047个Go输入和4个程序，字节无漂移；26个workspace链接仅随副本根路径迁移。W10 普通 CLI 实际 GET/PUT 拒绝证据已独立接受。

R22 自有 API/bridge 已停止；实际停止后连接0、gate 恢复 epoch28、27run结算、12artifact完整、无活动租约或未释放预留。145 个路径、64 个既有 generation 身份和12份tar的独立恢复检查已封存ACCEPTED；15条历史队列与旧unknown意图1条保留。本次已创建8个全新0700槽位、保留全部旧145路径并恢复暂存根0500；新的运行配置及停机时数据库状态已核对，等待单一API准入。父账本仍为 **24 条：0 PASS / 9 PARTIAL / 15 NOT_RUN**。

| 顺序 | 当前动作 | 责任与完成门槛 |
| --- | --- | --- |
| 1 | 完成 R23 接入后检查与 R22 恢复封存 | 两条 ROOT 路径已整合；Trellis 独立检查16例、strict类型、实际两文件lint，并核对145路径/12tar/停止后历史；确认当前源码与消费产品绑定 |
| 2 | 用新绑定执行原四场景 | 停机时分配八个新0700槽位并恢复根0500，核对复用源码/产品、配置、日志/socket、已有管理员与原端口；workers1/retries0/maxFailures1，一次执行 |
| 3 | 完成 S2 A01–A12 | 对实际结果、W10及停止后资源做独立复核，全部证据通过后结束 S2；每次失败保持原判 |
| 4 | 推进 S3 恢复 | 绑定最终 S2 后先跑 priority/handler 冲突和普通子任务数字 CAS，再按现有 v2/r05 计划推进评论、WS、mentions、上传/迟到结果和阻塞信息 |
| 5 | 接入真实 S4，执行最终 S5 | I02–I09 登记实际非生产输入后执行 E01–E07；稳定最终候选后采集100人/10,000传播样本/100独立写回，验收p95和父24条及业务签字 |

当前详细进度与证据见 `S2/research/progress-20261117.md`。S4 缺失真实输入时继续独立准备，相关真实案例及父任务保持开放。以下为保留的历史计划和检查点。

## Retained R22 code checkpoint

Updated 2026-11-17, Asia/Shanghai. The additional capability-GET account-read repair is now **CODE_ACCEPTED_R22_SCOPED** and integrated into ROOT's two existing service files plus one owning-layer regression. The genuine original-code RED has exactly two read coexistence failures with all writer/authority controls passing. GREEN normal 4/8/7 and combined race 7/25/22 pass, as do format, service vet and the four matching Go builds. The new API inputs are prepared; original four-case execution remains the next gate. The original invalid-control draft and setup-blocked SQL run remain preserved.

The current session resumes the existing S2 `in_progress` task. No new task is required. The R21 one-line `GetWorkflowOperation` repair and its 214-line owning-layer regression are already integrated into ROOT; their bytes match the independently accepted private GREEN. The targeted normal run has 6 named PASS, the combined race run has 9 named PASS, and service vet passes. The four consuming Go products also pass their recorded builds. These overlapping code checks remain separate from business acceptance.

The previous session reached actual API/administrator/platform preflight but started **zero** original four-case commands. Its API has stopped; the accepted production Web and Desktop renderer remain on ports 64925/64926. The latest completed original four-case run is still R20: **1 PASS, 1 FAIL, 2 NOT_RUN**. The parent ledger remains **24 cases: 0 PASS, 9 PARTIAL, 15 NOT_RUN**. The R20 exclusive lock holder remains unproved.

R22 source research found another independently reachable pure-GET boundary: every dispatch calls `EnsureCapabilities`, whose matching-summary path calls `RuntimeCapabilities` without a POST. The original GET reached `LockPasswordUser FOR UPDATE` despite making no durable write. The real two-connection regression now proves both read orders, and the accepted GET-only shared account repair preserves registration writers, session predicates and original runtime/binding/namespace fences. This does not identify the historical R20 holder.

| Order | Deliverable | Next action and completion gate |
| --- | --- | --- |
| 1 | S2 actual product flow | Code RED/GREEN, independent review, ROOT integration and matching Go products are done. Finish the current API input review, restore one API with fresh logs/socket, and run all four original cases with workers=1, retries=0 and max-failures=1. Preserve the original 5/20/480-second deadlines and every failed result. |
| 2 | S2 final evidence | Finish the matching ordinary-CLI rejection receipt for W10, reconcile A01–A12 and restore owned gates/processes. Accept S2 only after all required evidence and independent review pass. |
| 3 | S3 recovery | Rebind the already reviewed v2/r05 plan to final accepted S2; start priority/handler/numeric-CAS controls, then comment/WebSocket recovery, mixed mentions, uploads/late results and blocking-information repair. |
| 4 | S4 real integrations | Update I01/I10 internal bindings when S2 is accepted; register actual I02–I09 repository/CI, payment/fulfillment sandbox, nonproduction release/rollback, metric and signer inputs before E01–E07. |
| 5 | S5 and parent acceptance | Freeze the final S2–S4 candidate; collect 10,000 propagation samples from 100 participants and 100 independent writebacks, recompute p95, check restoration and reconcile all 24 requirements. |

S4's absent external inputs do not prevent independent preparation. Its real cases and the parent remain open until those inputs and formal sign-offs exist. The old S5 r02 rehearsal remains historical evidence.

Current evidence: `research/r21-red-green-boundary-independent-check-20261116.json`, `research/r21-runtime-product-entry-independent-check-20261116.json`, and their retained integration/build/runtime references under the existing S2 task. The implementation/check agents have bounded write scopes; the coordinator owns runtime actions and progress records.

## Retained starting point: R19/R20

- Continue `.trellis/tasks/archive/2026-11/11-13-pay-241-s2-stage-approvals`, already `in_progress`; its PRD records approved S2–S5 execution.
- S1 is archived. S3's controlled timeout proof is archived; remaining S3 acceptance, S4 real external inputs and S5 final-candidate acceptance are open.
- The parent ledger contains 24 cases: **0 PASS, 9 PARTIAL, 15 NOT_RUN**.
- R18 recorded **1 PASS, 1 FAIL, 2 NOT_RUN**. Its release/rollback journey reached metric observation and failed on the original observation-window contract.
- R19's fixture registration was independently checked against the existing 120-second metric window. Its actual four-case run on 2026-11-16 recorded **0 PASS, 1 FAIL, 3 NOT_RUN**. The first failure is the original five-second source-denial banner assertion at `e2e/collaboration-stage-approvals.spec.ts:351`; the metric journey was not reached.
- The earliest R19 R1 denial at trace275 returned `can_open=false` and null child digests. The later R4 response at trace276 returned HTTP200 with `baseline=null`, `blocked_evidence=1`, empty private evidence arrays and four blocked signable entries. The source generation guard can supersede that in-flight R4 after R1 clears its cache; later R1 responses at trace290/293 repeat the denial.
- `StageCodePanel` displays source denial only inside its `record` branch. Current source denial can erase/cancel that private R4 record; the explanation must remain visible from authoritative R1 denial.
- One real `55P03` occurred at the existing shared signer-member fence. The passive tracer uses pgx's query-tracing interface. Preserve these observations and complete their independent review; a UI repair does not establish that every transient read error is fixed.
- Existing task/progress summaries describe R15/R16 and need a current dated entry while keeping earlier records intact.

R19 result: `/Volumes/artisan/.multica-acceptance/pay241-20261115-continuation/resume-r05/r19-metric-freshness-fixture-r01/runtime/four-scenarios-r01/result.json`, SHA256 `c522677bce9e06475afe4043a6f9149608c26888f1496c0934084868d6b4ba6f`. Its stdout, stderr and original reporter hashes were checked.

## Task 1: Finish diagnosis and the retained after-check

**Owners:** coordinator, `r20_read_diagnosis` for the application graph, and `r20_evidence_check` for independent evidence and passive pgx instrumentation.

**Files:** current S2 `research/r20-read-lock-diagnosis-20261116.{md,json}` and `research/r20-r19-evidence-check-20261116.{md,json}`. Original R18/R19 private evidence stays immutable.

1. Reconcile the four original case records and exact reviewer requests, driver errors and timestamps.
2. Independently confirm the authoritative source-denial/cache/render sequence and identify the existing helper that represents current R1 source readability.
3. Check the passive tracer and all reachable read/writer lock invariants. Keep transient writer contention and cancelled requests separate from the confirmed missing banner.
4. Verify retained source/product hashes, stopped owned API/bridge, original gate restoration, run settlement, reservations and immutable artifact history.
5. Record the smallest confirmed behavior gap and exact regression scope before product edits.

**Gate:** independently supported root cause, accepted retained after-check and bounded implementation scope.

## Task 2: Repair source-denial presentation

**Owner:** `trellis-implement`; a separate `trellis-check` agent reviews the result.

**Expected files:** `packages/views/collaboration/stage-code-panel.tsx` and its existing owning-layer component suite. Use the existing source-readability helper; widen this list only if the diagnosis demonstrates a necessary shared helper change.

1. Add a component regression for authoritative R1 source denial with an absent/cancelled R4 record. Assert that the existing localized denial text remains visible and private source/decision/report content and approval actions remain absent.
2. Run the regression on the unchanged implementation and retain its failure. Keep loading, unknown and network-error states distinct from explicit source denial.
3. Render the explanation from current authority outside the private-record branch. Preserve source-cache scrubbing and existing anonymous R4 behavior.
4. Run the owning component suite plus existing stage-source cache and stage-state controls, package type-check and scoped lint.
5. Independently review the diff, raw results and exact source. Integrate only accepted paths, retaining peer edits.

**Gate:** regression RED/GREEN, affected checks pass and independent review has no unresolved blocker.

## Task 3: Complete S2 product acceptance

**Files:** existing `e2e/collaboration-stage-approvals.spec.ts`, `e2e/collaboration-stage-execution.spec.ts`, their fixtures and the S2 A01–A12 matrix.

1. Register a new private runtime suffix, unused staging generations and source/product/configuration hashes. Verify database and listener ownership before startup.
2. Build production Web and Desktop renderer from the accepted source. Reuse unchanged Go/CLI/Host products only when consumed inputs and output hashes match.
3. Verify matching API, production Web, Desktop renderer, authenticated synthetic administrator and original origins.
4. Run the existing four-case Playwright command with `--workers=1 --retries=0 --max-failures=1`. Retain the original case bodies and five/20/480-second deadlines.
5. Preserve any new failure and return to its responsible layer. On a pass, independently reconcile A01–A12 and the final after-check.
6. Update the parent ledger only for conclusions supported by accepted evidence; archive S2 after its full gate passes.

**Gate:** all four actual journeys pass, required A01–A12 evidence is complete and owned resources are restored.

## Task 4: Complete S3 recovery

**Files:** existing `.trellis/tasks/archive/2026-11/11-13-pay-241-s3-recovery/{prd,design,implement}.md`, approved implementation scope and owning-layer regressions.

1. Refresh the independently accepted v2/r05 references against final S2 source/locks; retain the archived timeout proof.
2. Activate the existing S3 task after its dependency gate.
3. Start with priority/handler commit-order conflicts and ordinary-child numeric CAS, using existing real commit barriers. Then execute cross-page comment recovery, WebSocket reauthentication, mixed mentions, revoked authority, blocking information, upload idempotency and late results in the recorded order.
4. Run owning-layer checks and actual Web/Desktop recovery journeys, followed by independent review.

**Gate:** all remaining S3 requirements have accepted evidence.

## Task 5: Resolve S4 inputs and perform S5 final acceptance

**Files:** existing S4 intake/research, S5 harness/research, parent `test-matrix.csv`, `gaps.md` and `session-progress.md`.

1. Reconcile S4 I01–I10 against actual supplied nonproduction payment/fulfillment, repository/CI, deployment/rollback and metric/reviewer inputs. Record exact missing inputs without credentials.
2. Run each real E01–E07 case once its target and access limits exist. Continue independent preparation while inputs are absent.
3. Freeze the accepted final candidate and matching products for S5. Execute 100 distinct participants, 10,000 formal propagation samples and 100 independent result/terminal writebacks.
4. Independently recompute p95 and resource restoration. Required limits remain propagation p95 ≤2 seconds and result writeback p95 ≤5 seconds.
5. Reconcile all 24 parent cases and required human acceptance. The r02 performance rehearsal remains historical evidence.

**Gate:** real external requirements, final performance and parent acceptance are supported. Missing external inputs keep the relevant tasks open.

## Execution boundaries

Use existing task-owned environments and narrow source integration. Preserve failures, retained databases, artifact generations and other sessions' edits. Add no dependencies. Real model accounts, real funds and production deployment are outside this continuation. Main session coordinates plans, task/spec updates and authorized scoped Lore commits; implementation/check agents stay inside their assigned files.

## Execution checkpoint: R20 result and R21 boundary

Task 1 and Task 2 passed independent review. Original-code RED has three missing-banner failures; the repaired component suite passes 22 tests, with 44 Core controls and passing type-check/lint. The matched production Web/Desktop builds and all 1,676 compiled outputs were independently accepted.

Task 3's R20 original four-case run recorded **1 PASS, 1 FAIL, 2 NOT_RUN**. The first complete journey passes, including the original source-denial assertion. The second stops at the original five-second QA approval-button assertion: a final R4 read fails with shared-user-lock `55P03` while T4 report detail remains passed/usable. The approval has not expired. Current evidence does not identify the actual lock holder. Keep all original results and investigate before repairing another layer.

The next bounded test establishes whether actual `GetWorkflowOperation` receipt recovery and actual R4 evidence reads coexist. Its current immutable GET takes the original exclusive user lock. Use the existing real stage fixture with an artifact-bound candidate and original approval request, two distinct backend connections, both read orders and a real writer-held negative control. Retain the five-second reader bounds and current-account authorization. If the new regression fails, the minimum candidate repair is only `GetWorkflowOperation`'s read fence plus the new owning-layer test; other receipt readers and production writers stay outside this slice. Passing this regression does not establish that this GET held the R20 failure's lock.

R20 after/source/history/stop audit passed. Its 23 Host runs are settled, 10 artifacts complete, gate booleans restored at epoch24 and owned API/bridge stopped; historical queues and the prior unknown intent remain intact. A01–A12 mapping also identifies one small remaining W10 runtime receipt: matching ordinary CLI rejection against a synthetic collaboration root, with unchanged state and zero write effects. Complete that evidence in the next bound runtime without changing the four-case denominator.
