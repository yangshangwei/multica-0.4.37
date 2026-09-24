# 研发交付与 Agent 质量双闭环闭合实施

## Goal

继续完成 RCA、事故学习、发布后观测、可靠性与 Agent 评测、体验/迁移 workload gate、治理能力及两条闭环的可验证接线。

## Requirements

- 用本地脱敏 issue/comment JSON 夹具验证 RCA comment 能被修复任务消费，事故学习能产出带 owner/验收信号的预防任务，发布观察能绑定同一 artifact digest。
- 明确 digest 不一致、观察窗口未结束、缺 baseline、重复事故和缺真实评测轨迹时的 `unknown`、阻塞或既有任务关联结果。
- 从最近两个发布周期建立 `experience-validation-engineer` 与 `migration-reviewer` 的 workload gate；未达阈值不新增 listed role、权限、skill 或 MCP。
- 记录两条闭环的实际产物落点、真实模型和生产指标未授权限制，以及 Git/CI、Observability、Database MCP 的准入前置条件。

## Acceptance Criteria

- [x] 脱敏 fixture 通过 Go 测试验证 RCA → 修复、事故学习、防重复和同产物发布观察交接。
- [x] 五类失败条件有显式 `unknown`、阻塞或链接既有任务的断言；Agent 评测 fixture 覆盖正确性、工具失败、越权、成本、延迟和漂移。
- [x] 最近两个发布周期的 workload gate 有来源、计数、阈值和复评条件；当前不新增体验/迁移 listed role。
- [x] 研究文档、闭环演练、MCP 取舍与实现状态一致；真实模型、生产指标和生产操作仍明确未测试/未执行。
- [ ] 真实模型 RCA smoke 和生产 observability 连接未获授权，不能标记为已通过。

## Notes

- Keep `prd.md` focused on requirements, constraints, and acceptance criteria.
- Lightweight tasks can remain PRD-only.
- For complex tasks, add `design.md` for technical design and `implement.md` for execution planning before `task.py start`.
# 研发交付与 Agent 质量双闭环闭合实施

## Goal

把现有模板能力闭合成可追踪、可复核的研发交付和 Agent 质量流程：失败能进入 RCA，RCA 能交给修复，修复能进入同产物发布和观察，事故能产生防重发任务；Agent/Skill/MCP 变更能进入版本化评测、发布门禁和漂移后续任务。

## Requirements

1. RCA 闭环：验证 `bug-fix`、`maintenance`、`incident` 的诊断路由、已知原因直达实现的理由、事故先止血后 RCA，以及 RCA 输出到修复任务的独立交接。
2. 事故学习和发布验证：验证 `multica-incident-learning` 与 `multica-rollout-and-canary-verification` 的输入、输出、`unknown`、同 artifact digest、观察窗口、回滚建议和防重复任务契约。
3. 核心角色：验证 `reliability-engineer` 和 `agent-evaluator` 的模板创建、skill 附着、权限、独立交付物，以及在 incident/release/review-gate 中的可达路径。
4. 专项角色 gate：从仓库任务、E2E、迁移记录或发布记录中给出体验验证和迁移审查的实际工作量证据；达到阈值才新增 listed role，否则保持明确的 skill/路由建议和复评条件。
5. 治理能力：验证契约兼容、威胁建模、供应链、产品结果、灾备五类能力的触发、输入、输出、责任角色、验证信号和审批边界。
6. 两条闭环演练：提供脱敏的失败→RCA→修复→发布→观测→复盘→防重发，以及 Agent 变更→评测→门禁→发布→漂移监控演练，并指出每一步的实际任务/评论/产物落点。
7. MCP 取舍：保持无凭据浏览器/推理 MCP；Git/CI、Observability、Database MCP 只有在来源版本、只读权限、凭据注入、脱敏和审计条件具备时才准入。

## Constraints

- 不读取生产凭据、客户原始数据，不执行未经批准的生产发布、回滚、迁移、恢复、通知或付费模型调用。
- 不覆盖既有 workspace agent/skill/squad 副本；模板更新必须通过创建时复制和版本 provenance 验证。
- 不按 frontend/backend/mobile 拆角色；没有独立交付物不得新增 coordinator。
- 真实模型 smoke 只在显式 `agentintegration` 和 `MULTICA_RUN_REAL_AGENT_SMOKE=1` 授权下运行；默认测试禁止调用用户安装的 Agent CLI。

## Acceptance Criteria

- [ ] RCA 三条路由有通过/跳过/停止证据，且诊断结论能被修复任务消费，不把事故主任务阻塞在 RCA 上。
- [ ] 事故学习和发布验证各有可复核的脱敏 fixture；缺证据、digest 不一致、观察窗口未结束、重复事故都显式产生 `unknown`/阻塞/关联结果。
- [ ] 两个核心角色从模板创建并在三个相关 squad 中可达；workspace 自定义 copy 保持不变。
- [ ] 体验/迁移专项角色有真实 workload 统计和阈值结论；未达阈值时默认 roster 不出现，达到阈值时才执行完整角色上线检查。
- [ ] 五类治理能力在既有 role skill/squad 路由中有触发-产物-验证矩阵，并有至少一个失败证据样例。
- [ ] 两条闭环演练的每个阶段都有任务/评论/skill/squad 或验证文件落点；没有只靠静态描述宣称闭环。
- [ ] 通过相关 Go、TypeScript、模板和浏览器检查；所有未运行的真实模型/生产验证都在证据文件中注明原因和复现命令。

## Non-goals

- 本任务不建设 observability 数据库、事件总线或自动生产控制面；如现有 issue/comment API 无法表达证据，先记录最小后续设计，不偷偷增加持久化模型。
- 本任务不自动升级或回滚既有 workspace 副本。

## Closeout（2026-09-24 归档说明）

本任务是双闭环的**宽泛收口 child**，其交付物已在各兄弟任务中完成并归档，故带说明归档，不重写上面的双 PRD。Block B 七条验收对应落点：

- RCA 三路由 / 不阻塞止血 → `09-24-lifecycle-runtime-handoffs`、`09-24-rca-loop-verification`、`09-24-lifecycle-closure-v2`
- 事故学习 / 发布验证脱敏 fixture → `09-24-incident-learning-canary`、`09-24-lifecycle-runtime-handoffs`
- reliability / agent-evaluator 三 squad 可达 → `09-24-reliability-agent-evaluator`
- 体验 / 迁移 workload gate → `09-24-experience-migration-roles`（角色已上线，提交 `c21d03a43`）
- 五类治理 + 失败样例 → `09-24-delivery-governance-recovery`（`ed0fa7789`、`491e1127c`）
- 两条闭环演练落点 → `09-24-lifecycle-implementation`、`09-24-lifecycle-dual-loop-rehearsal`、本任务 `research/closure-rehearsal.md`
- Go / TS / 模板 / 浏览器检查 + 文档化跳过 → `09-24-lifecycle-closure-v2/research/verification.md` 及上述任务 evidence

更正与限制：

- Block A“当前不新增体验/迁移 listed role”为早期定格；两角色其后已上线，本任务 `research/workload-gate.md` 判定两周期均“达到 listed gate”。
- 真实模型 RCA smoke 与生产 observability 连接为 authorization-gated，记为文档化跳过（可复跑命令见相关 evidence）。
- requirements-evidence.md 引用的 `09-24-lifecycle-finalization/research/workload-gate.md` 随该任务归档，内容现位于 `archive/2026-09/` 下；本任务 `research/workload-gate.md` 为自洽副本。
