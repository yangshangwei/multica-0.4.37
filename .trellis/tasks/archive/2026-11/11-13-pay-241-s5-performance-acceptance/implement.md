# S5 执行计划

状态：in_progress（2026-11-14激活，工具与演练已完成，正式验收待依赖）。独立规划审查已完成（`research/plan-review.md`，可开始实现采样工具，阻挡项0）。主会话采纳审查第3节的三项较严选择：pooled与每事件最慢观察者双重p95判定、100个独立run各一条结果、沿用共享PostgreSQL集群并记录余量与负载。最终验收运行与总核销仍依赖S2–S4。

## 0. 已完成

- [x] 建立S5任务并完成100用户/WS/回写测量研究。
- [x] 固定完整分母、预热、时钟、起止点、丢失/中止口径和真实模型边界。
- [x] 独立审查采样工具设计，对照当前源码核实账号、会话、事件、回写夹具与环境规则，修订PRD/design。

## 1. 采样工具（现在可做，不依赖S2–S4）

由原生 `trellis-implement` 在design.md第7节的四个新文件内完成，不修改任何已有文件。主会话负责环境分配与实测，子代理不得选库、迁移或改门禁。

1.1–1.5 已由 `s5_harness_impl` 完成，见 `research/harness-implementation.md`：反例测试20/20先失败后通过，定向tsc/ESLint干净，未启用时3项skip。四项实施偏离（启用后环境不合格即失败、按阶段分开的summary/failures、注册直调 `/auth/register` 读取 `Retry-After`、连接池/Redis/副本由manifest声明并拒绝超出默认的形态）经主会话2026-11-14采纳，已写入design.md第2、5、6节。

- [x] 1.1 先写 `collaboration-performance-stats.ts` 的反例测试并保留首次失败：nearest-rank（N=100/10000/20，整数rank）、缺失按+∞进入分位数、缺失超过5%时p95为无穷、任何缺失使判定失败、重复 `event_id` 不产生第二个样本、非 `S+1` 序号或 `workbench_changed` 帧不匹配、早于写响应到达的帧被保留、预热不进入正式分母、中止余项记NOT_RUN且分母仍为10000、每事件最慢观察者p95与pooled p95分别判定。
- [x] 1.2 实现纯函数使其通过；`node --experimental-strip-types --test e2e/fixtures/collaboration-performance-stats.node-test.mjs`。
- [x] 1.3 实现 `collaboration-performance.ts`：环境守卫、prepare（分窗注册、原键恢复、私有状态）、100会话建立、静止屏障、带种子相位抖动、观察与读取、增量样本日志、资源快照。
- [x] 1.4 实现 `collaboration-performance.spec.ts` 的 prepare / propagation / writeback 三个测试；未显式启用时全部skip。
- [x] 1.5 定向tsc与ESLint（沿用S3 timeout-proof的 `research/targeted-static-check.json` 方式，记录完整argv与配置）；确认未启用时 `pnpm exec playwright test e2e/collaboration-performance.spec.ts` 只报告skip。
- [x] 1.6 r01后修正：工具为回写runtime按生产daemon方式发送心跳并落盘，触发前记录runtime在线状态，离线触发记 `runtime_offline`；核对其他随时间到期的凭证与租约（design.md第4节）。报告 `research/harness-fix-r01.md` / `.json`，28项工具测试、定向tsc/lint通过，未启用套件3项skip。后续独立检查修复ACK/100socket约束，31项工具测试通过；r02已证明255秒持续心跳与完整回写，正式最终候选仍未验收。

## 2. 工具演练（现在可做）

- [x] 2.1 主会话分配新的仓库外运行目录与新数据库，冻结当前提交的源码副本加四个工具文件；构建API、migrate、CLI、daemon测试二进制与 `collaboration-host-provider.go`，记录哈希。核对数据库身份后迁移。
- [x] 2.2 记录门禁原值、PostgreSQL连接余量、主机负载与描述符上限；以password配置启动API，核对 `/health`、`/api/config`。
- [x] 2.3 运行prepare（约15分钟以上，受注册限流约束），核对100个distinct身份与参与者清单。
- [x] 2.4 r01失败完整保留；修复后以新库/新目录重新prepare并完成r02：10000传播样本与100个独立run的两种回写样本全部收敛，见 `research/rehearsal-r02.md` / `.json`。独立79项核对通过；仅为工具演练，不写入TC-24通过结论。
- [x] 2.5 r01与r02均已关闭自有socket/daemon/API并恢复分配原值（各自epoch5→6）。r02 API pid67305和18996监听关闭，最终只读快照确认数据库连接0、自有二进制进程0；原数据库/失败/全部样本保留。

r02准备（2026-11-14续接）：新运行根 `/Users/artisan/.multica/acceptance/pay241-s5-20261114-r02`、新库 `multica_pay241_s5_r02_api`，保持r01产品提交 `fb9f4c070` 以验证工具修复；五个产品/受控测试二进制经r01哈希账本校验后复用，四个工具文件另行冻结。锁文件离线安装和迁移到804通过。隔离API pid67305、18996端口，health匹配该提交。独立检查修复了真实ACK形状、100socket完整负载约束及诊断起点，31项工具测试/tsc/lint通过，见 `research/harness-fix-review.{md,json}`；按最终4个源码哈希重冻后prepare、propagation、writeback全部通过，独立79项检查及最终资源核对通过。全部身份与配置记录在仓库外私有目录；不代表最终候选验收。

## 3. 最终验收运行（依赖S2–S4稳定）

- [ ] 3.1 在最终候选提交上重新冻结源码，构建并绑定API、production Web、Desktop renderer与受控二进制；新数据库、新运行编号。
- [ ] 3.2 重新prepare并执行10000个传播样本与100个run的回写样本；另开少量真实Web/Electron页面的DOM可见测量并单列。
- [ ] 3.3 按 `verification-environment.md` 的最终身份核对做源码/构建/配置终态审计。
- [ ] 3.4 独立 `trellis-check` 核对原始样本、分位数、缺失与资源恢复；不得删除坏样本后重算，不得在多次运行中挑选。

## 4. 总核销

- [ ] 4.1 逐条核销24例、缺陷和实际角色验收结论，更新父任务 `test-matrix.csv` 与 `gaps.md`；任何必需外部未测保持开放。
- [x] 4.2 新增 `.trellis/spec/web/frontend/collaboration-performance-qa.md`，记录完整分母、真实ACK、100socket断连约束与环境/证据边界；这是工具已验证知识，未提前核销总验收。
- [ ] 4.3 仅以明确文件清单Lore本地提交。完整S5与总验收条件达成后才能归档或关闭父任务。

## 验证命令（实施时核对后使用）

```bash
node --experimental-strip-types --test e2e/fixtures/collaboration-performance-stats.node-test.mjs
node "$TASK_RUN_ROOT/run.mjs" "$TASK_RUN_ROOT/perf.json" pnpm --dir "$TASK_SOURCE_ROOT" exec playwright test e2e/collaboration-performance.spec.ts -g prepare --workers=1 --retries=0
node "$TASK_RUN_ROOT/run.mjs" "$TASK_RUN_ROOT/perf.json" pnpm --dir "$TASK_SOURCE_ROOT" exec playwright test e2e/collaboration-performance.spec.ts -g "propagation|writeback" --workers=1 --retries=0
```

`run.mjs` 与 `perf.json` 是本次运行新建并重新绑定的私有runner与配置；以上命令在本次审查中均未执行。

## 回滚与边界

工具全部是新增测试文件，可整体撤回而不影响产品。不为通过而调整限流、连接池、门禁或中继间隔；不调用真实模型、真实资金或生产发布。
