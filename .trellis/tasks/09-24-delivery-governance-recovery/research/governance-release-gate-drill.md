# 发布门禁治理演练证据

## 场景

一次发布同时改变已存储数据（新增列并回填）、引入一个新的外部依赖、触及旧桌面客户端与插件的响应契约，并声称带来产品结果改进（某转化指标上升），且要求具备可恢复能力（备份点与恢复演练）。这类发布最容易在单点角色处漏检：任一治理检查缺证据，就不能凭“构建通过”或“可回退提交”放行。本演练用现有角色、role skill 和小队路由把五项治理检查串成一条跨小队链路，并记录跳过项与人工审批点。

## 演练路径

| 检查 | 触发 | 小队/角色 | role skill | 需要的证据 | 缺证据时的结论 | 归属阶段 |
|---|---|---|---|---|---|---|
| 契约兼容 | 改动触及旧客户端/插件契约或 API/OpenAPI | 上游 architect（discovery/feature-delivery）→ 合并门禁·迁移审查员复核 | multica-architecture-decision-record (v5) | 旧客户端/插件矩阵、parseWithFallback、兼容窗口 | `hold` | 上游澄清/设计 → 合并门禁复核 |
| 安全（威胁建模） | 触及授权/令牌/查询/数据暴露 | 合并门禁·安全审查员 | multica-security-review (v3) | 信任边界、滥用路径、缓解措施 | `hold` | 合并门禁 |
| 供应链 | 新增或升级外部依赖/构建产物 | 合并门禁·安全审查员 | multica-security-review (v3) | 依赖来源、锁文件、SBOM/来源、digest | `hold` | 合并门禁 |
| 产品结果 | 声称产品结果/指标改进 | 进度报告负责人（无 roster 席位，交接） | multica-progress-report (v3) | baseline、观察窗口、实际结果 | `unknown` | 发布外交接 |
| 灾备恢复 | 改动已存储数据、需要备份/恢复能力 | 发布·发布工程师（备份点）+ 发布·可靠性工程师（恢复后采样） | multica-release-check (v4) + multica-reliability-engineering (v2) | 备份点、恢复演练、RPO/RTO、同产物恢复后采样 | 无演练→`hold`；观察窗口未完成→`unknown` | 发布 |

链路方向：上游 architect/ADR 给出旧客户端·插件矩阵和兼容窗口 → 合并门禁的安全审查员做信任边界与供应链/来源/SBOM、迁移审查员复核兼容窗口 → 发布阶段发布工程师准备备份点/恢复演练/RPO·RTO、可靠性工程师在同一产物上做恢复后采样。发布小队核对上游证据是否齐全，不重做上游审查。

## 跳过项

- 供应链检查在本次确有依赖变更时执行；若某轮无依赖或构建产物变更，则跳过并写明原因。
- 体验验证在触及关键用户路径或跨端时执行；无关键路径改动时跳过并写明原因。
- 产品结果检查因门禁 roster 无席位承接，以 handoff 形式交给进度报告负责人；在收到 baseline 和观察窗口前，结论保持 `unknown`，不计作达标或失败。
- 跳过必须写明原因；“未收到检查结果”不等于“检查通过”或“计数为零”。

## 人工审批点

- 新依赖引入、不可逆迁移：需 ADR 记录，并在合并门禁请求人工介入（生产数据、凭据、不可逆迁移或产品负责人接受兼容/体验风险）。
- 生产发布、回滚、恢复：由发布工程师（operator）逐项申请人工审批，一次审批只覆盖其明确描述的那一项操作；一个环境的审批不授权下一个环境，发布审批不授权回滚。
- 兼容/体验风险接受：由产品负责人显式接受，门禁与发布不得代签。

## 已验证

| 测试 | 证据 |
|---|---|
| `TestGovernanceSkills_FailureExampleContracts` | 五个治理 role skill 各自的失败样例规则与版本齐全：ADR(v5)/release-check(v4)/security(v3) 缺证据给 `hold`，progress(v3)/reliability(v2) 缺证据给 `unknown`。 |
| `TestSquadTemplates_LifecycleEvidenceRouting` | 合并门禁/发布/维护/事故把生命周期证据路由到预期角色；发布要求同一产物的观察窗口，并在涉及迁移时要求旧客户端兼容证据。 |
| `TestReleaseGateGovernanceDrill_Contract` | 发布路由把五项治理检查串成一条链并要求说明跳过项，保留逐项人工审批；每项检查解析到对应 role skill 且该 skill 挂在预期角色上。 |

## 未验证 / 明确不执行

- 未读取生产凭据。
- 未执行生产发布、回滚、迁移、恢复或客户通知。
- 未启动真实智能体，未连接生产 Observability、Git/CI、Database、artifact registry 或 feature-flag MCP。
- 本地测试证明模板与治理合同，不证明真实生产 SLO、客户结果或外部供应商 API 可用性。

## 运行命令

```bash
(cd server && go test ./internal/service -run 'TestGovernanceSkills_FailureExampleContracts|TestSquadTemplates_|TestReleaseGateGovernanceDrill_Contract|TestSquadLeads_' -count=1 -v)
(cd server && go vet ./internal/service)
```
