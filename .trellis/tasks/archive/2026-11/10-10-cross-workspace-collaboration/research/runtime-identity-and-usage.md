# 执行身份、凭据与用量：源码核查

- 日期：2026-10-12。
- 源码基线：`327a539481f4f9c004388e2d230d038877f7ad02`。
- 方法：静态检查现有源码和安全规范。本记录不代表跨空间协作已经实现或通过运行验收。

## 已确认的现有机制

| 事实 | 依据 | 对本任务的约束 |
| --- | --- | --- |
| 人类调用使用本人身份；智能体调用沿执行记录追溯 `originator_user_id`，已终止执行不能继续借用该人的调用权限 | [invokeOriginatorFromRequest](../../../../server/internal/handler/agent_access.go#L234)、[既有安全契约](../../../spec/server/security-boundaries.md#scenario-2-agent-created-triggers-act-as-the-authorizing-human) | 不能把协作者的指令伪装成运行环境拥有者发起；整单发起人、指令提交者与资源拥有者须分别识别 |
| daemon 要求执行使用任务级 Multica 令牌，不能回退到 daemon 自身的个人凭据 | [taskScopedAuthToken 的调用](../../../../server/internal/daemon/daemon.go#L7597) | 本单资源授权不能变成共享环境拥有者的个人 Multica 登录凭据 |
| 任务令牌绑定一个工作空间，请求头或查询参数不能扩大绑定范围 | [ResolveWorkspaceIDFromRequest](../../../../server/internal/middleware/workspace.go#L69) | 跨空间业务任务和执行关联需要明确授权路径，不能靠切换工作空间请求头绕过现有门禁 |
| 执行可使用 daemon 用户现有的本机工具状态，以及智能体设置中的供应商 API 环境变量；Multica CLI 的任务认证另行隔离 | [本机状态处理](../../../../server/internal/daemon/daemon.go#L7639)、[自定义供应商环境](../../../../server/internal/daemon/daemon.go#L7670) | 供应商账号由实际执行配置决定，不会仅因本单换处理人就自动换成该人的账号 |
| `CostUSDTicks` 是供应商报告的成本；零表示未报告，不能解释为免费。daemon 将模型用量汇入本次执行的上报数据 | [TokenUsage](../../../../server/pkg/agent/agent.go#L177)、[用量上报](../../../../server/internal/daemon/daemon.go#L7991) | 用量和成本记录不能单独证明费用已经在协作者之间分摊，也不能由资源拥有者字段推断实际付款人 |

## 已解决的技术辨析

- Multica 的任务认证与供应商登录/API 凭据是两条不同的链路。
- 本单发起人用于业务权限，指令提交者用于执行归因，环境拥有者用于资源授权；三者可能不是同一账号。
- 新的按单授权应明确允许的执行范围，同时保留现有真实调用人、任务令牌和工作空间边界。

## 已确认的产品选择

2026-10-12，用户选择"沿用所选运行环境的账号"，已记录为 [PRD](../prd.md) 的 R28、AC43 和 AC44。首版沿用所选环境已配置的供应商账号、额度和计费方式，不自动切换为各指令提交者的供应商账号；实际提交者仍独立记录，平台任务认证边界保持。

具体外部付款方取决于供应商账户及其组织设置，本次未据代码字段推断个人账单或代扣结果。
