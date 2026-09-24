# 要求到证据审计

| 要求 | 当前权威证据 | 当前结论 | 缺口/下一步 |
|---|---|---|---|
| RCA 接入 bug-fix/maintenance/incident | `server/internal/service/builtin_squad_templates.go`、三份 squad instructions、`lifecycle_handoff_runtime_test.go` | 模板路由、先止血后 RCA 规则和三类 runtime handoff 已证明 | 真实模型 RCA 行为仍未授权运行；需保留跳过证据 |
| RCA → 修复交接 | diagnostician instructions/skill、`lifecycle_handoff_fixtures.json`、`TestCreateLifecycleHandoffPersistsRCARepairEvidence` | 结构化 `diagnosis_ref`、`regression_test`、结论/证据/未知项会写入下游修复 issue，workspace 边界和 dedup 仍复用现有 API | 真实模型 RCA 轨迹未授权运行；当前证明限于脱敏本地 issue |
| 事故学习 | `multica-incident-learning/SKILL.md`、incident instructions、`server/internal/service/testdata/lifecycle/lifecycle_handoff_fixtures.json` | 输入/输出/unknown/防重复规则和脱敏 issue/comment 交接已由纯 Go fixture 验证 | 没有后台自动创建预防任务；fixture 证明下游消费格式，不宣称自动化运行 |
| 发布后验证 | `multica-rollout-and-canary-verification/SKILL.md`、release instructions、同一 fixture | 同 digest、baseline/window/signals/decision、未获批不回滚及 digest/window/baseline 失败路径已验证 | 没有 Observability MCP/持久化 canary 记录；仍只做脱敏 fixture |
| Reliability Engineer | roster、instructions、skill、template/handler tests | listed、Contributor、skill 附着和 squad 可达已证明 | 真实 SLO/队列/备份信号未接入生产；只能验证 unknown 处理 |
| Agent Evaluator | roster、instructions、skill、review/release roster、tests | listed、Observer、版本化 case 契约和 squad 可达已证明 | 真实模型/工具轨迹评测未授权；需用静态 case manifest 演练 |
| Experience Validation | `09-24-lifecycle-finalization/research/workload-gate.md`；v0.4.46/v0.4.47 发布 E2E、桌面、截图和失败分类证据 | 两周期均达到关键路径数量并有可复核跨端/截图/覆盖缺口材料；新增独立交付物和 gate 测试 | **已达 listed 阈值**；由体验验证工程师 + QA + product analyst 路由，缺可访问性/跨端证据时 hold/unknown |
| Migration Review | `09-24-lifecycle-finalization/research/workload-gate.md`；两次发布的客户端/API/schema 兼容、升级校验和回滚限制记录 | 两周期各有兼容迁移及旧/新契约、窗口、校验、重试/回滚材料；新增独立审查交付物和 gate 测试 | **已达 listed 阈值**；由迁移审查员 + architect + security + release 路由，证据缺失时阻塞 |
| 五类治理能力 | architecture/security/release/progress/reliability skills 与治理测试 | 触发/产物/验证/边界已写入 skill | 需要至少一个失败证据样例，不新增治理 coordinator |
| MCP | `builtin_mcp_templates.go`、`builtin_mcp_templates_test.go` | 仅 3 个无凭据模板，keyless 约束已证明 | Git/CI、Observability、Database 仍需版本/权限/脱敏设计后准入 |

## 证据等级

- `proven`：当前仓库有自动化测试或权威模板/规范支持。
- `static-only`：只有 prompt/skill/文档规则，不能证明运行时会自动产生后续任务。
- `blocked-by-authorisation`：真实模型、生产指标、凭据或外部系统需要明确授权。
- `missing`：没有当前产物或验证。
