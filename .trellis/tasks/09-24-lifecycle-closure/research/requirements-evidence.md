# 要求到证据审计

| 要求 | 当前权威证据 | 当前结论 | 缺口/下一步 |
|---|---|---|---|
| RCA 接入 bug-fix/maintenance/incident | `server/internal/service/builtin_squad_templates.go`、三份 squad instructions、`builtin_agent_autonomy_test.go` | 已证明模板路由和先止血后 RCA 规则 | 真实模型 RCA 行为仍未授权运行；需保留跳过证据 |
| RCA → 修复交接 | diagnostician instructions/skill、bug-fix instructions、`lifecycle_handoff_fixtures.json` | 静态职责边界和 `diagnosis_ref` 下游消费已证明 | 没有独立的结构化 RCA 任务 API；fixture 只验证交接格式，不宣称后台自动创建 |
| 事故学习 | `multica-incident-learning/SKILL.md`、incident instructions、`server/internal/service/testdata/lifecycle/lifecycle_handoff_fixtures.json` | 输入/输出/unknown/防重复规则和脱敏 issue/comment 交接已由纯 Go fixture 验证 | 没有后台自动创建预防任务；fixture 证明下游消费格式，不宣称自动化运行 |
| 发布后验证 | `multica-rollout-and-canary-verification/SKILL.md`、release instructions、同一 fixture | 同 digest、baseline/window/signals/decision、未获批不回滚及 digest/window/baseline 失败路径已验证 | 没有 Observability MCP/持久化 canary 记录；仍只做脱敏 fixture |
| Reliability Engineer | roster、instructions、skill、template/handler tests | listed、Contributor、skill 附着和 squad 可达已证明 | 真实 SLO/队列/备份信号未接入生产；只能验证 unknown 处理 |
| Agent Evaluator | roster、instructions、skill、review/release roster、tests | listed、Observer、版本化 case 契约和 squad 可达已证明 | 真实模型/工具轨迹评测未授权；需用静态 case manifest 演练 |
| Experience Validation | `research/workload-gate.md`；v0.4.46/v0.4.47 发布 E2E 与桌面证据 | 两周期有浏览器/桌面工作量，但没有逐周期可访问性结果或 QA 排队/覆盖缺口 | 未达 listed 阈值；继续由 QA + Playwright/Chrome MCP + product analyst 路由，满足连续两个后续周期门槛后复评 |
| Migration Review | `research/workload-gate.md`；两次发布离线升级记录 | 发布升级不是 schema/API/客户端迁移审查；没有两周期独立兼容窗口/校验/回滚报告 | 未达 listed 阈值；继续由 architect + security + release 路由，达到每周期一项迁移并有独立交付物后复评 |
| 五类治理能力 | architecture/security/release/progress/reliability skills 与治理测试 | 触发/产物/验证/边界已写入 skill | 需要至少一个失败证据样例，不新增治理 coordinator |
| MCP | `builtin_mcp_templates.go`、`builtin_mcp_templates_test.go` | 仅 3 个无凭据模板，keyless 约束已证明 | Git/CI、Observability、Database 仍需版本/权限/脱敏设计后准入 |

## 证据等级

- `proven`：当前仓库有自动化测试或权威模板/规范支持。
- `static-only`：只有 prompt/skill/文档规则，不能证明运行时会自动产生后续任务。
- `blocked-by-authorisation`：真实模型、生产指标、凭据或外部系统需要明确授权。
- `missing`：没有当前产物或验证。
