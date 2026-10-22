# 实施与验证计划

状态：in_progress。2026-10-21 终审通过，用户针对最终摘要确认“按计划开始实施”，已执行 `task.py start`。本计划的具体切片边界允许按实际代码收窄，产品语义与父 R/AC 不减少。

## 1. 开工与任务边界

- [x] 2026-10-21 主会话读取当前 task/branch/status，将本次续接时的 40 个 status 条目（展开为 57 个文件；原规划时为 38 项）保存到仓库外 `/tmp/multica-gap-recovery-20261021-h6phxwo9/`；包含文件副本、SHA-256、HEAD、staged/unstaged 及合并 binary diff。实施开始前重验基线，不覆盖或归入新成果。
- [x] 最终规划与 manifest 验证通过，2026-10-21 收到针对最新摘要的实施确认并启动当前子任务。沿用父任务的原产品批准，不重新询问已定 M01–M11。
- [ ] 本子任务直接先实施 S1；S2–S5 按下表各自形成独立可验收提交。若需要另建独立交付任务，以当前子任务计划明确其前置和验收，不能由 parent/child 位置推导依赖。
- [ ] 每次 dispatch 首行 `Active task: .trellis/tasks/10-20-collaboration-gap-recovery`，优先 native context injection，缺失时 child 读取对应 JSONL；研究只读研究上下文。代理不得递归编排或回滚他人改动。
- [ ] 协调者独占生成 SQL、路由/API barrel、共用 locale/schema 整合及环境/构建身份；每个实施切片由 `trellis-implement`、检查由独立 `trellis-check` 负责。

## 2. 独立切片所有权与顺序

| 切片 | 所有权 | 依赖与复核 |
| --- | --- | --- |
| S1a 服务端扫描 | `collaboration_runtime.go`、wake/promotion 的必要 helper、runtime SQL、相邻 DB tests；生成 SQL 交协调者 | 先写固定 16/局部错误回归；保留当前 reservation/derived 未提交修复；先定扫描和可选 API 形状 |
| S1b daemon/proof | `server/internal/daemon/collaboration.go`、poll loop 必要接线、`collaborationrun` 账本恢复和测试 | 独立 proof 公平性可并行；candidate 优先合同依 S1a；不动 sealed proof/nonce 或未知容量 |
| S2 资源提供 | resource service/handler/必要 SQL；之后 core resource schema/query 与 shared resource form/panel | 服务端最小提供/采纳合同先于客户端；权限/资源矩阵主测试在 DB；Web/Desktop 验接线 |
| S3a `@all` | core comment helper、shared composer/必要正文提及入口与测试 | 与 S2 可并行；两份 locale 由协调者合并；服务端现有通知矩阵复用 |
| S3b 取消历史 | queue/lifecycle/invalidation 取消 provenance、受保护命令读取；随后 core schema/query 与 views history/execution | 先定准确关联与历史缺失规则；不改 cancel-vs-stop 及 M11 消费合同 |
| S4 运行交付 | collaboration protocol/delivery capability、daemon controller/provider 适配、文件准备与原生测试 | S1 控制与 S2 授权稳定；每个 provider/OS 组合独立能力证据，不能用普通 adapter 存在代替验收 |
| S5 代管 | collaboration administration service/handler、core admin API、shared forms，Web/Desktop platform wiring | 依现有 lifecycle/队列/历史稳定；与 S4 可并行；password/admin 环境独立 |
| S6 全量验收 | 协调者与独立检查者，97 项台账、源码 manifest/真实环境、必要用户/内置技能说明 | 所有切片各自已检查；父任务最终集成仍按原 97 项 |

## 3. S1 失败回归优先

- [ ] DB-backed 17+ waiting：前 16 项持续目标特有拒绝且不退出查询，第 17 项确实具独立合法准入；有限回绕后前进。禁止用真实 namespace unknown 占用作为可运行后继。
- [ ] 候选页边界/第一领取失败、多个 poller/重启：覆盖 cursor 已越过 reservation key、优先者在页外及新出现/优先级变化的 reservation；低等级放行前完成完整优先等级检查与有界回绕，reservation 持有者先得到机会、新根不能抢占，真实容量独立的同等级后继仍有限轮次被尝试。真实 pending 派生仍延后新根，撤权派生不永久阻塞；同 lane FIFO/active group 不重复。
- [ ] 单对象 wake/promotion error/5xx 不吞掉既有合法候选；全局身份/ctx/DB 错误仍返回；unknown commit 查原身份不重复 mutation。用明确 fault/事务屏障，不能任意吞错或 permanent hold。
- [ ] 9+ sealed/pre-burn proof 前 8 项持续 HTTP 失败；第 9 项有限轮次 ACK，失败原 receipt/nonce 保留。另测前 8 项跨 supervisor ErrBusy、ACK 丢失、恢复 cursor 崩溃/重启、总 deadline。
- [ ] 第一候选持续非 unknown 领取失败给后继机会；任何 unknown/burned/损坏 ledger 仍保留容量和 update barrier，不能通过轮转额外 fork。
- [ ] 各回归先证明失败，再最小修复；相邻已存在测试通过后独立检查，无原因不重复全套。

## 4. S2/S3 行为与验证

- [ ] P/D/H 三账号、两个独立空间，owner 不是组织者且组织者无 owner 空间 membership：提供具体组合→creator/handler 采纳→各 owner consent→有效 @。普通参与者不能 register、组织者不能代签、不开放额外目录、未参与 owner/资源失效竞争拒绝。
- [ ] core schema 包含畸形/旧可选字段/跨单/关联/epoch 测试；现有资源 inventory 目录拒绝保持。原 key 不确定恢复、提供撤回与已采纳 grant 撤回各测正确入口。
- [ ] `@all` helper 特例和共享入口；显式区分人员/AI，all+具体 agent/squad、actor 自己、失效参与者、编辑差集、原 key/通知去重。不要通过 DOM 重跑整套服务端权限矩阵。
- [ ] 未消费取消输入原文/附件/作者/target/seq、精确取消事件及原因；刷新/换端/评论编辑删除后仍可查。自动 remove/exit/disable/close 及 M11 unread/claimed 各有一处主验证，缺历史 provenance 显示缺失不编造。
- [ ] waiting cancel 与 promote/claim 确定性竞争；后继 FIFO、当前 run 和两个暂停层不变，失败不自动 Stop。另测 R26/M06 Switch 后取消：waiting 输入只经原 Switch primitive 迁到新队长通道末尾，取消关联其精确当前身份，旧消费快照、已取消/已停止输入及历史 membership 保留。撤权暖缓存和历史/附件直链不能越权。
- [ ] 生产 Web 与实际 Electron 验新资源角色分离、混合提及和取消历史接线；宽/窄/200% 缩放和明确无溢出，每次视觉编辑前使用 visual-verdict 并留证。

## 5. S4/S5 与完整验收

- [ ] 固定附件版本/commit 准备和 run-bound 字节能力；目录/凭据/未提交修改不搬运，准备错误与许可不确定按 E03/E04 收敛。provider record/usage/result/proof 关联精确。
- [ ] 从 canonical provider registry 建 provider×OS 支持矩阵；fake executable 原生测所有 launch/fallback/后代，Linux setsid/double-fork/stdio/进程复用、Windows Job 失败和完整 macOS 观测。无法满足者明确 unsupported，不降低 proof。
- [ ] 真实 provider smoke 保留为单独授权检查；默认测试严格双门禁且不查找用户 CLI。一般 provider 支持、合成执行证据和付费真实模型证据分别列出。
- [ ] 专用代管 read/candidate/action/recovery、锁内 password/session/role/org/source/disabled creator 重验；条件 successor 含幂等 payload，复用 lifecycle 同事务 effects。
- [ ] AC65/66/69/70 的双禁用、原账号恢复、候选禁用/退出、角色/范围变更、重放/换候选/并发责任变更；失败无半次重开/待办/事件，creator 不变、不增加 admin 协作身份、不恢复 AI。
- [ ] Web 管理与 Desktop 账号壳共享表单；独立 password/admin-enabled 环境证明，默认 legacy E2E 不归为代管证据。
- [ ] 为 `acceptance-matrix.json` 全部 97 项补实际 test identity/commands/results/source/env，按父主归属验证；旧入口/普通任务/关闭 gate 读取和收尾、重启/丢响应/迁移/原生安装包分别留证。
- [ ] 全部完整合同满足之前 parent 不归档、创建/执行 gate 不启用。无法执行的 provider/OS/安装包/S3/Redis 场景保留明确限制，不能以 skip 声称通过。

## 6. 验证入口

以下为实施命令，不表示已运行。Go 从 `server/` 执行，并只指向本任务隔离数据库；Linux 名称必须在实际 Linux 运行，darwin skip 不算证据。

```bash
# 生成由协调者串行执行
make sqlc

# 相邻服务端与恢复检查（实际新增用例按最终命名收窄）
GOTOOLCHAIN=go1.26.6 bash ../scripts/go-test-with-agent-cli-guard.sh go test -race ./internal/daemon ./internal/daemon/collaborationrun -run 'Collaboration|Runner|Ledger|Retained|Nonce|Seal' -count=1
GOTOOLCHAIN=go1.26.6 bash ../scripts/go-test-with-agent-cli-guard.sh go test -race ./internal/service ./internal/handler ./cmd/server -run 'Collaboration' -count=1
GOTOOLCHAIN=go1.26.6 go vet ./internal/daemon/... ./internal/service ./internal/handler ./cmd/server

# 前端相邻范围
pnpm --filter @multica/core test collaboration
pnpm --filter @multica/views test collaboration
pnpm --filter @multica/core typecheck
pnpm --filter @multica/views typecheck
pnpm --filter @multica/web typecheck
pnpm --filter @multica/desktop typecheck
pnpm --filter @multica/core lint
pnpm --filter @multica/views lint

# 最后统一检查 + 统一入口之外的静态范围
make check
pnpm knip
pnpm check:ui-exports
node apps/desktop/scripts/verify-renderer-file-access.mjs
```

生产 Web E2E 以 task-owned production API/Web、准确源码/配置/build ID 为依据，一 checkout 同时只一次 build；不能 next dev 或复用未知监听。独立 P7 配置沿父计划 password/admin-enabled，真实 Electron 与打包 Main 启动分开。所有 test pass/fail/skip/未运行和重试记录按真实执行保存。

## 7. 本轮规划完成检查

- [x] PRD 收敛无重复临时问题，三个复杂规划文件与真实 JSONL 存在。
- [x] 运行 `task.py validate`、JSON/JSONL/97-ID owner/相对链接/上下文预算和 `git diff --check`；保存 `planning-verification.json`。
- [x] 独立规划复核无未解决具体冲突；F1/F2 已在任务文档修正并由 `research/planning-final-review-2026-10-21.md` 确认关闭。
- [x] [最终摘要](planning-summary.md) 已落盘，展示目标、范围、接受的边界、风险、顺序和产物，供用户审阅最新规划。实施确认及 `task.py start` 仍按第 1 节单独跟踪。

## 8. 提交与回退

每个行为切片验证后按 Lore intent/body/trailers 独立提交，保留 Tested/Not-tested 和原基线差异；不包含他人改动。schema 加法保留历史且无 FK，索引各自 CONCURRENTLY。回退关闭新入口/准入，保持读取/Stop/proof；存在活协作数据不回退旧 writer、删表、删未知 run 或清屏障。

## Current verified execution record — 2026-10-21 continuation

This section records execution of the existing plan; it does not replace or
replan the ordered slices above.

- [x] Restore same task/branch; preserve dirty work with an external snapshot.
- [x] Partition all 1153 service top-level race identities, run remaining Go and
  Redis supplements, and collect production API/Web/Playwright prerequisites.
- [x] AC90/91/97 mixed H+A+B+S, partial refusal/original-key dedup, fresh offline
  login/anchors/read/archive/edit deltas: production Web and actual Electron.
- [x] AC95 agent/squad/whole-issue controls and truthful partial-proof reasons:
  production Web and actual Electron, canonical service and real Linux proof.
- [x] AC33 complete combined Web/Electron A/B overlap, coalescing, exact one-input
  cancellation, actual daemon proof and durable history/results after repair.
- [x] S5 password/admin-enabled Web/Desktop journey and backend atomic boundaries.
- [x] Independently review the proposed ledger, preserve all 97 definitions and
  update authoritative status to 64 passed /33 not verified, including AC63's
  expanded encrypted attachment and permission-preservation refusal snapshot.
- [x] Capture nullable-reason wire and execution/evidence scope in specs.
- [ ] Finish isolated Lore slice commits and extend remaining exact clauses;
  AC10/38/40/50/74/88 rejected automatic promotion and remain open.
- [ ] All 97 criteria with current evidence; provider/OS/packaging/rollout gates
  remain separate and neither child nor parent may be archived yet.


## Current acceptance extensions — 2026-10-22 execution record

No planning phase or task creation was repeated. The earlier S1–S5 and prioritized
AC90/91/97, AC33/95, S5 password journeys are completed as recorded above.
Current authoritative acceptance is79 passed /18 not verified; the original
97 definitions and primary-owner mapping remain unchanged.

- [x] Current pipeline receipt validation1153 service identities, safe environment
  identity, production/native creation/download2/2 and matching backend6.
- [x] AC06/16/28/30 actual handler grant/control/active acceptance correlation;
  independent whole-clause review, isolated HEAD and Lore `ccedea89e`.
- [x] AC59/77 offline public assignment/transfer/archive/fresh login/revoked entry,
  actual Web/Electron, independent review and Lore `28681f527`.
- [x] Source transfer/departure/iteration-revision races and external planning,
  isolated two-file proof, Lore `7794f57d9`.
- [x] AC93 all seven actual typed comment state entries, original barriers and
  ordinary fallback negatives, independent review and Lore `cef01d954`.
- [x] AC80/81 exact source role/reason/strict iteration PUT plus canonical/race
  clauses, independent full review and Lore `829815775`.
- [x] Bounded plugin/Slack real transport+ordinary positives and original gate,
  current/isolated proof, Lore `1cb3f5073`; full AC84 remains partial.
- [ ] AC67 typed creation body, AC75 retained comment-file cross-client path.
- [ ] AC09/20/21/22 actual pause/removal/proof/reconnect dual-client journey;
  AC34/57 same-lane and explicit whole close truthfulness.
- [ ] AC49 multi-target/member owner exit and AC40 public Switch/Resume with
  busy destination; AC96 complete actual owner/runtime/derived catalogue matrix.
- [ ] AC83 complete offline lifecycle/result fanout, AC85 source identity/drag,
  AC89 source board/CLI reorder, AC31/32 installed clients, AC43 real providers.

Each outstanding box is existing S6 acceptance, not a new feature or changed
plan. Production gates remain closed, real provider accounts/quota and rollout
retain their original separate authorization. Child/parent stay in_progress.

## Current reviewed execution ledger — 2026-10-22

The current independent review promotesAC09/20/21/22,40/49,67/75 and records
87 passed /10 not verified. Original97 definitions/primary owners retain
SHAe29ead58ba9e2c45a4099a52611c55e12dcae3b735136e058aafc87674cf217e.
This is execution of the existing S6 plan, not a replacement plan.

- [x] Original handoff exact serial race112/406 and omitted daemonws22/30.
- [x] Current Core1024/Views94 and Core/Views/Web/Desktop typechecks.
- [x] AC67/75 current content3/3 and canonical13/19, independent whole-clause review.
- [x] AC09/20/21/22 actual Web/Electron+Linux2/2, canonical15/35 and independent review.
- [x] AC40/49 current26/132 and final isolated pair2/2, independent review.
- [x] Six verified Lore behavior/prerequisite commits424ea35ed,d11ddd78f,449bbec5c,
  84cdbd233,43854e62e,238d4bb39.
- [ ] Existing AC34/57 same-lane and live complete/cancel journey.
- [ ] Existing AC83 offline information/acceptance/proven result fanout.
- [ ] Existing AC84 other IM direct/stream/relay/media path.
- [ ] Existing AC85/89 source external identity and drag/CLI reorder; AC96 complete
  new participant/all owner/derived current catalogue matrix.
- [ ] Existing AC31/32 installed/old-client acceptance; AC43 real providers retains
  separate authorization. Production gates stay off; child/parent in_progress.
