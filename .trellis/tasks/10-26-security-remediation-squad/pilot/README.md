# 安全修复小队试点包

状态：`phase1_real_results_routing_gate_open`（2026-10-27 重试后）

原 daemon/runtime 已恢复并完成真实模型任务，8 个角色保留原身份、自治等级、模板来源和私有 Access。`opencode/nemotron-3.5-lightning-free` 短探测返回 `READY` 后，四个父案例 `POLA-3`～`POLA-6` 和八个独立成员子任务 `POLA-7`～`POLA-14` 均已交到 `in_review`。隔离实验室的依赖与授权补丁通过六项测试，供应链风险继续保持 `hold/unknown`。

阶段一尚未验收：两次修复由 leader 直接实现，它还曾以自己的身份代写成员报告；操作者随后手动派发独立审查子任务。进一步只修改试点小队指令后，已证明一条真正的 leader → 安全审查员委派，包含精确 Markdown mention、真实成员 task 和 delegation 来源；成员随后因匿名模型限额未返回结果，leader 又误把派发记录为 `no_action`，未证明成员返回与 leader 收口。诊断工程师和实现工程师本人的交接、报告准确性、网络出站约束仍需补验。最终容器已停止，临时开发认证副本已删除。

当前证据见 `trial-results.md`、`evidence/real-pilot-summary-20261027.json` 和 `evidence/run-state.json`；最后的派活验证见 `evidence/retry-routing-validation-20261027.json`，最终清理见 `evidence/routing-final-cleanup-20261027.json`；可移植的补丁回放见 `evidence/portable-replay-20261027.json`。合成失败基线保留在 `fixtures/`，实际补丁保存在隔离副本及 `evidence/lab-diff-20261027.patch`。恢复方法见 `runtime/README.md`。前次 `blocked_pending_model_config` 及其失败任务保留为历史，不能覆盖本次成功结果，也不能用本次测试通过替代路由验收。

这份试点包把阶段一变成可以重复执行的工作区操作。它只使用非生产仓库、脱敏输入和现有角色，不创建新的产品模板，也不允许 agent 触及生产、真实凭据或客户原始数据。

## 运行前置条件

1. CLI 已登录目标 Multica 服务器，并选定试点工作区。
2. 恢复专用非生产容器中的原 daemon ID，并核对其所有成员工作空间的旧任务队列。用户 PAT/JWT profile 的 `workspace_id` 不限制账号成员工作空间的发现范围；工作空间 daemon token 仍受自身绑定范围限制。
3. 试点操作者拥有创建/运行角色 agent 和 squad 的权限；所有新 agent 的 Access 先保持私有或指定试点成员。
4. 不向 runtime 注入生产数据库密码、个人 SSH key、长期云凭据或可写生产 MCP。
5. 使用经本轮真实执行验证的模型配置，并在新运行前检查其可用性；匿名服务可发生限额、传输或证书错误。补齐 API/模型出站目的地约束的验证，当前默认 bridge 不能证明完整的网络最小权限。

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

下面保留首次案例使用的原始指令作为历史。当前试运行小队已通过成对的 `expected_instructions` / `expected_updated_at` 前置条件更新为 `runtime/pilot-instructions-v2.md`，只加强此小队的协调、精确 mention 和结束回合要求；8 人 roster、共享角色内容、自治与 Access 不变。新指令只证明了真实派发，成员返回与收口仍待验证。

```text
你负责协调安全事项的非生产分诊和修复证据。

先记录 type（dependency/code/other）、source、affected_surface、severity、exploitability、evidence_state、production_impact、route、blocked_actions、next_owner。

dependency：安全审查是否受影响 → 需要时做迁移审查 → 实现一次只改一项 → QA 独立验证 → 安全/代码复审。
code：安全审查威胁模型 → 原因未知时交诊断 → 实现补回归测试 → QA 做权限/边界/负面验证 → 安全/代码复审。
other：按配置、供应链、Agent/Skill/MCP、运行信号选择安全审查、agent-evaluator 或 reliability-engineer；没有证据就保持 hold/unknown。

如果确认当前生产可利用、凭据暴露、客户数据暴露或需要高风险操作：先请求人工介入，再交 incident/release；本小队不得读取真实 secret、运行生产命令或编写可用攻击载荷。

修改代码的成员不能验证自己的修改。派活后记录成员、理由和停止条件；整体证据齐全后交 review-gate 或人工，不把父任务标记为 done。
```

## 下一轮执行顺序

1. 复用试点 v2 指令，恢复可用模型后完成已证明委派的成员返回与 leader 收口，并纠正派发应记 `action` 的错误。用 `[@小队名](mention://squad/<UUID>)` 激活真正的 leader；其成员交接使用 `[@成员名](mention://agent/<UUID>)`。仅回复 leader 旧评论会走普通 agent 回合，不能替代小队激活。精确链接必须保留方括号，最终仍以真实 task、评论作者和 `source_task_id` 为证。
2. 用已记录的失败基线验证 `diagnostician → implementer → QA` 的实际分工；不得由 leader 实现或替成员写结论。补丁正确性与路由正确性分别留证。
3. 回读所有角色评论、activity 和任务结果，更正测试数量、基线/当前 diff、来源哈希及版本锁定的权限含义；统一分诊字段继续使用规定枚举。
4. 补齐 API/模型目的地约束，保持供应链缺证事项的 `hold/unknown` 和 owner。四例满足完整验收后才进入阶段二。

## 安全停止条件

- 需要真实凭据、客户原始数据、生产 shell、线上迁移或外部通知时立即停止并转人工。
- 安全结论缺少跨租户边界、依赖来源、SBOM 或运行信号时写 `hold`/`unknown`，不能用“测试通过”替代。
- 同一修复失败两次、两位成员对根因有分歧，或两次缓解仍未恢复时停止继续派活并升级。
