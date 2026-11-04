## 2026-11-04 #7 已独立关闭，转入 #6 框架验证

#7 的 AC1–AC9 / 21 项、共享 v16、四 CCG 及本地 `eda0d68fb` 源码一致性已独立通过并完成归档。24 项旧 source overlay 有逐文件备份和 scoped stash，其他修改保留；历史失败/数据库/原始报告哈希与权限保持原值。稳定交接：`research/knowledge-framework-final-20261104/task7-closure-handoff.json`；归档报告：`../archive/2026-11/11-01-collaboration-knowledge-audit-fixes/closing-report-20261104.md`。当前 #6 仍在进行，下一步保留六文件与旧17、同步验收源码、最小修正真实 HTTP List/promote 轮询，再新专库普通/race及独立源码绑定。知识预算/真实调用 0/0，生产默认 false，真实及发布门单列开放。

- [x] #7 全范围独立终审、本地验收源码集成、稳定交接和归档引用迁移。
- [ ] 同步 `eda0d68fb` 到既有框架 worktree，保留六文件/旧17，修正 `nextRunForIssue` 的真实 HTTP List seam。
- [ ] 两份新专库分别跑七类完整 fake 普通/race；E0b/K1/K2/K3 与15 measured +3 seed、0 paid、durable summary均须齐备。
- [ ] 全六文件独立终审及最终产品/框架/CLI/工具/配置/专库绑定，本地提交集成。
- [x] 真实预算/调用0/0、不开真实入口、不转用额度，生产默认false。

# Current K3 root-cause and source synchronization (2026-11-03)

Independent current-source review: `../archive/2026-11/11-01-collaboration-knowledge-audit-fixes/research/progress-recheck-20261103-01a1a1c6-97cddb08/knowledge6-k3-review/review.json`. Normal8 fails before the first squad/root sample because `nextRunForIssue` queries stored rows without invoking the actual runtime HTTP List, whose product path promotes waiting inputs. The later seed-member edit does not address this earlier failure. After task #7 closes, preserve all six files and the original seventeen-file bindings, merge the current tested candidate into the divergent framework worktree, and verify production roots match it. Correct only the fixture polling seam to call real List and filter eligible IDs for the exact issue/session; retain the one-run assertion. Then perform fresh dedicated normal/race runs and independent final six-file review. Allocation/calls remain 0/0. Previous plans and failures below are retained.

# Knowledge framework implementation plan

Current continuation: [2026-11-03 progress recheck and plan](../archive/2026-11/11-01-collaboration-knowledge-audit-fixes/advance-plan-2026-11-03.md). The six files now exist but remain uncommitted. The latest dynamic attempt is normal8: E0b/K1/K2 pass, K3 fail; a subsequent source edit has compile-only evidence. Full normal/race and current six-file independent review remain open. The old shared-check dependency at the end of this document is historical; current source/ownership comes from the linked recheck. This does not change the authorized scope or budget.

## Goal and authority

Continue the already-authorized implementation of knowledge promotion with a test-only sibling of RK1. Implement the source-backed proposal in ../archive/2026-11/11-01-collaboration-knowledge-audit-fixes/research/knowledge-real-model-preregistration-20261102/design.md and its entry-matrix.json. This plan authorizes no real Provider launches or production enablement. Knowledge allocation/consumption stay 0/0, paid source identity stays empty, and real registration stays not runnable.

Original E0–E6 remaining14 and runtime RK1 approved25 are separate ledgers owned by their existing coordinators. Preserve all original runtime tests, registrations, source bindings and experiments.

## Change boundary

- Gap: the existing 17-file RK1 framework constructs knowledge-off fixtures and captures only collaboration runs. It cannot prove ordinary knowledge inputs or the fixed K1/K2/K3 schedules.
- Owning seams: router construction, existing Provider capture primitives and the ordinary daemon's actual claim/run/report lifecycle. Reuse these seams rather than inventing task/run identities or duplicating product execution.
- Base: clean shared v10 repair candidate 118fc53ea8eb8ebc53b4f9c1131e3e7910eacce9. Its committed server/apps/packages trees match final4. New knowledge code and test source require a new later binding; they do not alter v10's frozen source.
- Worktree: /Volumes/artisan/code/2026/mc-knowledge-framework-20261103, branch codex/knowledge-framework-20261103. Root and shared final4/v10 resources remain outside write/execution scope.
- Expected new code: server/cmd/server/rk1k_fixture_test.go, rk1k_provider_test.go, rk1k_scenarios_test.go, rk1k_evidence_test.go, rk1k_entry_test.go and server/internal/daemon/rk1k_driver_agentintegration.go. All are guarded by agentintegration && !windows. Existing 17 rk1_*_test.go files remain byte-for-byte unchanged.
- No new dependencies, product policy, production API, old launcher modification, model/account discovery, default-database mutation, full-check duplication, remote publication or release.

## 1. Establish isolated source and default-safe guards

- [x] Create the isolated worktree, record the exact base and 17 borrowed-file hashes, and compile tagged baseline without executing TestMain. Baseline compile exit0 at2026-11-02T19:34:01Z; this is no scenario pass.
- [ ] Build a basename-dispatched knowledge-only wrapper and explicit fake/stand-in plans. Leave rk1-provider and rk1-sim-claude dispatch unchanged.
- [ ] Preserve real ordinary task versus collaboration run identity as a discriminated observation. Invalid/missing identity, unauthorized real admission and ambient executable discovery must refuse before a child starts.
- [ ] Use existing frame/capture/wait/finalizer helpers; preserve ordinary business environment and host containment policy separately.

## 2. Build true producers and the thin ordinary driver

- [ ] Construct a knowledge-enabled router from newCollaborationRouteFixture; compose reusable RK1 target/host helpers without calling its knowledge-off constructor.
- [ ] Reuse Daemon.New, authenticated Client.ClaimTask, runTask and reportTaskResult through one build-tagged bridge, retaining one daemon/configuration for K2's native history.
- [ ] Drive actual human HTTP producers for issue, explicit mention comment, private Chat, AI quick-create, manual run-only autopilot, collaboration comments/root, actual delegated member and actual wake/proof continuation.
- [ ] Verify task-token project CLI reads and inert quick-create issue linking without human/PAT fallback. Preserve actual attribution, original fences, roles and readers.

## 3. Execute finite fake schedules and evidence faults

- [ ] K1: four distinct consumer issues/runs for pending, approved, corrected and withdrawn states; no lawful previous output contaminates absence observations.
- [ ] K2: two native turns with distinct eligible alpha/beta, then freeze third claim/preparation before source revocation and prove unchanged authorization/replay refuses with zero third process. Supported fresh claim after prior revocation is a separate lawful control.
- [ ] K3: eight fixed producer/role samples in registered order, one measured fake process per row; member/continuation have explicitly separate fake seed starts.
- [ ] E0b remains a distinct ordinary prerequisite. Preserve exact counters, original request identities, bounded dropped responses and stop-on-incomplete behavior.
- [ ] Negative controls cover stale input, wrong complete semantic answer, revoked/unavailable/busy preparation, native ID/ACK defects, durable evidence faults and no extra later starts.

## 4. Verify and review before binding

- [ ] Begin with compile/static and controlled fake-wrapper checks that do not initialize a database; record red/green evidence for new behavior.
- [ ] Run normal/race fake suites only after an explicit task database allocation and coordinated resource window. Each DB-backed invocation gets a new migrated multica_rk1k_* database; cmd/server TestMain runs before body skips. No shared final4/v10 database, API, Web, Redis or renderer is used.
- [ ] Save named results, raw hashes, eight row identities, 4/2/8 measured counters and separate fake-seed counters under research/knowledge-framework-20261103. A compile or skipped test is not scenario evidence.
- [ ] A separate trellis-check agent reviews all six files and required contracts, reproduces useful bounded controls and reports unresolved findings. Root owns final verification and integration.
- [ ] Keep real entry non-runnable until separately funded, reviewed, source/account/artifact-bound and assigned an exclusive Provider window. Update progress truthfully; neither #6 nor #7 closes on this framework's compile or fake results alone.

## Shared acceptance dependency

V9 finished exit2 with browser334pass/16fail/89skip. All16 targeted replays were separately reported green; v10 is prepared at118fc53e but has no complete result. The sole full-check owner remains thread01a19c0d-cce1-73e3-99ee-2c3c9e9239ba/task10-30-collaboration-fix-closure. This work never rewrites that evidence or starts a duplicate full check.
