# 安全修复小队试点包

状态：`blocked_pending_model_config`（2026-10-27）

原 daemon/runtime 的恢复已在隔离容器中验证，8 个角色继续保留原身份、自治等级、模板来源和私有 Access。真实升级演练 `POLA-3` 已到达模型服务，但匿名 OpenCode 返回额度限制；当前没有成功的角色输出。三类案例 `POLA-4`、`POLA-5`、`POLA-6` 已预建为未分配的 backlog。容器已停止，临时开发凭据副本已移除，等待可用模型配置和网络边界检查后续跑。

当前证据见 `trial-results.md`、`evidence/run-state.json`；合成失败基线见 `fixtures/`，隔离副本和独立 QA 顺序见 `fixture-guide.md`，恢复运行时的方法见 `runtime/README.md`。这些准备材料不代表阶段一验收通过。

这份试点包把阶段一变成可以重复执行的工作区操作。它只使用非生产仓库、脱敏输入和现有角色，不创建新的产品模板，也不允许 agent 触及生产、真实凭据或客户原始数据。

## 运行前置条件

1. CLI 已登录目标 Multica 服务器，并选定试点工作区。
2. 恢复专用非生产容器中的原 daemon ID，并核对其所有成员工作空间的旧任务队列。profile 的 `workspace_id` 不限制 daemon 的工作空间发现范围。
3. 试点操作者拥有创建/运行角色 agent 和 squad 的权限；所有新 agent 的 Access 先保持私有或指定试点成员。
4. 不向 runtime 注入生产数据库密码、个人 SSH key、长期云凭据或可写生产 MCP。
5. 提供可用的模型配置；补齐 API/模型出站目的地约束的验证。当前容器默认 bridge 的出站访问未受限制，不能据此勾选完整的网络最小权限验收。

## 试点花名册

| 座位 | 模板 | 自治等级 | 默认交付 |
|---|---|---|---|
| Leader | `maintenance-lead` | Coordinator | 每轮只路由一个下一步，记录 route 和 evidence_state |
| 分诊 | `product-analyst` | Observer | 影响资产、严重程度、验收标准 |
| 安全审查 | `security-reviewer` | Observer | 适用性、威胁模型、供应链和安全发现 |
| 诊断 | `diagnostician` | Contributor | 仅在原因未知时复现、最小化和定位 |
| 实现 | `implementer` | Contributor | 一项最小修复和无修复时失败的回归测试 |
| 验证 | `qa-engineer` | Contributor | 独立回归、负面/权限测试和真实命令结果 |
| 兼容性 | `migration-reviewer` | Observer | 仅在 schema/API/客户端契约受影响时派活 |
| 正确性复审 | `code-reviewer` | Observer | 读取最终 diff，报告实际失败表现 |

`release-engineer` 不加入这支试点小队。生产发布、线上迁移、凭据访问、外部通知和不可逆操作必须转给现有 incident/release 路径并等待人工审批。

## 小队指令

把下面的内容作为试点 squad instructions；成员自身的模板指令继续有效：

```text
你负责协调安全事项的非生产分诊和修复证据。

先记录 type（dependency/code/other）、source、affected_surface、severity、exploitability、evidence_state、production_impact、route、blocked_actions、next_owner。

dependency：安全审查是否受影响 → 需要时做迁移审查 → 实现一次只改一项 → QA 独立验证 → 安全/代码复审。
code：安全审查威胁模型 → 原因未知时交诊断 → 实现补回归测试 → QA 做权限/边界/负面验证 → 安全/代码复审。
other：按配置、供应链、Agent/Skill/MCP、运行信号选择安全审查、agent-evaluator 或 reliability-engineer；没有证据就保持 hold/unknown。

如果确认当前生产可利用、凭据暴露、客户数据暴露或需要高风险操作：先请求人工介入，再交 incident/release；本小队不得读取真实 secret、运行生产命令或编写可用攻击载荷。

修改代码的成员不能验证自己的修改。派活后记录成员、理由和停止条件；整体证据齐全后交 review-gate 或人工，不把父任务标记为 done。
```

## 执行顺序

1. 用 `cases/dependency-advisory.md`、`cases/code-authorization.md` 和 `cases/other-supply-chain.md` 各创建一条非生产 issue，依次分配给本小队。
2. 对每条 issue 使用 `evidence-template.md` 回读评论和 activity，记录实际路由、成员、命令、测试结果、未知项和下一 owner。
3. 用 `cases/production-escalation.md` 做一次只读升级演练：不执行生产动作，只确认人工审批请求和 incident/release 交接内容。
4. 三类案例都闭环后，填写 `trial-results.md`，列出误分流、重复交接、指令冲突和模板缺口。

## 安全停止条件

- 需要真实凭据、客户原始数据、生产 shell、线上迁移或外部通知时立即停止并转人工。
- 安全结论缺少跨租户边界、依赖来源、SBOM 或运行信号时写 `hold`/`unknown`，不能用“测试通过”替代。
- 同一修复失败两次、两位成员对根因有分歧，或两次缓解仍未恢复时停止继续派活并升级。
