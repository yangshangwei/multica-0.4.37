# C 组独立审查：角色、冒烟入口与补充请求展示

审查日期：2026-10-30。审查者：本次独立 C 组代理。目标代码固定为 `/Volumes/artisan/code/2026/mc-collab-integ` 的 `4b0cbfde7a2a03142e70ceb91294ec8d8f1ff074`；本报告中的代码行号均基于该 HEAD。实验框架另行注明其 worktree。

本次只读审查了 `e6ed3d1c3`、`fd3836da2`、`4b0cbfde7` 的最终集成行为、调用链和现有断言；没有修改产品代码，没有运行真实 Provider、账号诊断、数据库或浏览器。主会话负责本轮定向单测的实际执行与持久证据，不在这里重复运行。

## 独立结论

三项提交的窄范围实现均与其声明相符，未发现必须回退这三项实现的新权限绕过或确定性回归。结论是“窄范围静态审查通过，最终 HEAD 运行验收待完成”，不是“协作上下文交接已经可靠”或“可直接合入”。

| 提交 | 当前实际行为 | 断言质量 | 结论与边界 |
| --- | --- | --- | --- |
| `e6ed3d1c3` | 初始 prompt 包含服务端 `role`、`step_key`；成员得到“只做队长派活，经 `issue get` 找 `dispatch_comment`”的指引 | 新单测验证 role/step 文本和成员说明，验证 leader 不带成员说明；旧 service/e2e 测试验证运行时能读派活正文 | 修复“角色/步骤丢失”通过；派活正文仍需模型主动召回，续作结果和已读标记仍缺 |
| `fd3836da2` | 正式真实账号 smoke 的测试白名单允许当前 permit 的业务 token 调用本 run 的 `POST /memory/base` | 新单测既验证放行正确 permit token，也验证拒绝未知 token；没有启动模型 | 冒烟入口的确定性阻塞已修；是否真正启动和完成模型须跑正式 smoke，E0b 不能替代 |
| `4b0cbfde7` | 补充交回表单显示所回答请求的请求人、正文；状态失效时显示回退状态；说明框与原请求通过 a11y 描述关联 | 两个新增测试实际断言请求正文、请求人、accessible description、回退状态和空正文 | 展示修复通过；接收方浏览器上的可见性和实际返回行为还应在最终 HEAD 验收 |

## 1. 小队角色及派活内容

### 已确认修复

服务端本来已经将 `StepKind`、`StepKey` 和 `ParentRunID` 放入 host claim：`server/internal/service/collaboration_runtime_capabilities.go:274`。协议字段定义位于 `server/pkg/protocol/collaboration_host.go:31`。

现在 `hostPrompt` 将 `Role`、`StepKey` 编入 Provider 的 JSON 输入，值直接来自 claim：`server/internal/daemon/collaborationrun/host_runner.go:496`。成员角色还增加了明确的派活查找指引：同文件 `:517`。这一改动没有把用户文本解释成身份，也没有改变服务端当前授权判断。

`TestHostPromptTellsAMemberRunItsRoleAndStep` 的关键断言位于 `server/internal/daemon/collaborationrun/host_prompt_role_test.go:23`：它验证 `"role":"member"`、`"step_key":"dispatch-1"` 和 `dispatch_comment` 的存在。`:28` 还验证 leader prompt 保留角色且不带成员说明。这是有价值的回归：删除本次实现后会失败。

派活正文的读取链也确实存在，不是文案指向一个空字段：

1. `RuntimeContext` 查本 run 对应的 `collaboration_delegation`，以 `SourceCommentID` 读取评论，并赋给 `DispatchComment`：`server/internal/service/collaboration_squad.go:187`、`:195`、`:199`。
2. CLI 的类型包含 `DispatchComment`：`server/internal/cli/collaboration.go:179`；`Read` 使用本 run 的 `/context`，并验证响应身份和单据一致：同文件 `:196`。
3. 旧 service 测试明确比较派活正文与原评论，而非只检查字段存在：`TestCollaborationSquadCommentCapacityOneLeaderMemberContinuation`，`server/internal/service/collaboration_squad_test.go:119`。
4. 旧 e2e 也比较了完整派活正文：`e2e/collaboration-host-squad.spec.ts:89`；这是假 Provider 的生产链路断言，不能证明真实模型会主动读取或遵从。

### 仍缺什么，以及如何分类

**依赖模型自觉：派活正文没有进入初始 prompt。** `hostPrompt` 的结构体中没有 `dispatch_comment` 字段，只有角色、步骤、父 run、原人类输入、附件/Git 和协作上下文；正文须由模型执行 `multica issue get` 后才可见（`host_runner.go:496`、`:519`）。因此不能把此修复写成“成员已必然获得派活内容”。测试只验证查找指引，未验证真实工具调用、派活内容使用或“只发 BETA”的行为。E3 正是补这一层证据。

**实现能力缺口：队长续作缺成员完成结果与输入处理标记。** claim 冻结时仍收集本组所有 `in_flight` 输入（`collaboration_runtime_capabilities.go:289`、`:295`），`CollaborationRunInput` 不含 `reader_run_ids` 或“已由哪个 run 处理”的字段（`server/pkg/protocol/collaboration.go:35`）。`consumeJoinedInputs` 将新并入输入纳入该批次（`server/internal/service/collaboration_squad_join.go:170`）。`RuntimeContext` 的返回结构没有成员 outcomes、唤醒原因或最终答复列表（`collaboration_squad.go:47`）。这不是本次 role/step 修复引入的回归，但完整交接目标尚未满足。

**设计取舍加缺少告知：多个唤醒合并成一个续作。** `queueContinuation` 发现已有 prepared continuation 就返回原 ID，不补充所有触发原因（`collaboration_squad.go:345`）；`parent_run_id` 因而不能当作“最后完成成员”的确定标识。规范明确要求最多一个排队续作（`.trellis/spec/server/collaboration-squads.md:32`），但没有相应的结构化完成结果投影。人看的运行记录会投影 `squad_activity.reason`（`server/internal/service/collaboration_records.go:417`），不等于后续 Agent 可读。

**依赖模型自觉：派活语法仍没有随 host brief 提供。** `hostBrief` 只提示 `issue get` 和显式 UUID request-id（`host_runner.go:476`），未提供 `[@Name](mention://agent/<uuid>)` 派活格式和 frozen roster 的约束。旧 e2e 自己构造正确格式（`e2e/collaboration-host-squad.spec.ts:74`）。不能用该测试证明未预先知道格式的真实队长会正确派活。建议在后续上下文改进中补固定 brief，首轮不要把它暗含为既有修复已完成。

### 小范围验收建议

- 首轮：在最终 HEAD 复跑 role/step 单测及已有 service/e2e 派活断言；按预登记做 E3 前后各 3 个有效 trial，并分别保存 actual prompt、`issue get` 工具结果、发出的评论和结束结果。
- 后续最小改进：从已有 delegation、run event、input reader 记录投影接手所需字段；优先送达明确派活正文、成员完成结果与已处理/新增输入标记。不要把 provider 私人会话或全量记录作为唯一交接媒介。
- E3 能验证这次模型/CLI/配置下的行为，不能升级为所有 Provider 或任意提示词的可靠保证。

## 2. 正式冒烟白名单

### 已确认修复及权限范围

生产 runner 在启动 Provider 前调用 `hostMemory`（`server/internal/daemon/collaborationrun/host_runner.go:131`）；除 `memory_unsupported` 外的最终错误会让 Provider 不启动（`host_memory.go:37`、`:41`、`:45`）。正式 smoke 旧白名单缺 `POST /memory/base`，因此测试会在启动前报 403。

本次新增只增加了一个精确 method/suffix 组合：`server/internal/daemon/collaborationrun/host_agentintegration_test.go:211`。原有的 run path 前缀、无 query/cookie/workspace header 和 permit token 校验保持（同文件 `:190`、`:194`、`:218`）。`/memory/base` 仍须持有 fixture 已发 permit 对应的 `mcb_` token；没有允许普通用户 token、daemon token 或 termination token 替代业务 token，也没有放开 `/memory` 写入。

`TestHostProviderAccountSmokeAuthorizesSharedContextBase` 在 `host_agentintegration_authorizer_test.go:17` 对正确 token 做正例，`:25` 对未知 token 做反例；它不会发现/执行用户安装的 Provider CLI。这一回归断言足以证明缺失 allowlist 条目已被覆盖，但不是对完整白名单安全矩阵的证明。错误 method、其他 run、query/cookie/workspace header 的拒绝仍由源码现有条件保证，本次新单测没有逐个重跑这些条件。

### E0 与 E0b 的重要区别

正式 `TestHostProviderAccountSmoke` 通过 `authorizeHostAccountSmokeRequest`（`host_agentintegration_test.go:107`）后才进入 fixture。它在任何 CLI 发现、版本查询和账号访问之前检查 `MULTICA_RUN_REAL_AGENT_SMOKE=1`（`:31`），而后验证真实 termination/proof/permit、CLI context 读取、成功结果及 ledger ACK（`:137`、`:152`、`:155`、`:159`）。它才是修复后 E0 的正确入口。

未提交的实验框架 `/Volumes/artisan/code/2026/mc-context-exp/server/internal/daemon/collaborationrun/host_context_experiment_test.go:149` 自建处理器；`:198` 的 default 直接进入 `fixture.serve`，并未调用上述 authorizer。其 `E0b_sanity`（`:299`）虽然用了类似 prompt，但即使在旧 authorizer 仍缺路由的版本上，也不会复现那一处 403。

因此：

- “authorizer 单测通过”只证明确定性测试入口已放行必要路由。
- “E0b_sanity 通过”不能证明 `fd3836da2` 已修复正式 smoke 的阻塞。
- 修复后 E0 必须实际运行精确的 `TestHostProviderAccountSmoke`，记录所用代码 HEAD、CLI/model 版本和实际启动证据。环境 401/429/503/超时单独归档，不用它们证明产品成功或失败。

可复跑的无账号回归命令（由主会话执行并保存输出）：

```bash
cd /Volumes/artisan/code/2026/mc-collab-integ/server
env -u GIT_CONFIG_COUNT go test -tags=agentintegration ./internal/daemon/collaborationrun/ -run '^TestHostProviderAccountSmokeAuthorizesSharedContextBase$' -count=1 -v
```

正式 E0 命令须选择实际 Provider 与其绝对可执行路径；正则必须为 `^TestHostProviderAccountSmoke$`，不能把上面的无账号单测当正式 E0。默认模型、测试产生的临时 CLI 路径以及 provider model 解析必须记录在实验材料中；现有 smoke 日志只固定输出 provider CLI 版本/OS/arch（`host_agentintegration_test.go:79`），不足以单独复现模型条件。

## 3. 补充请求交回表单

### 请求、身份和返回目的地没有被混淆

服务端 `InformationRequestView` 读取当前 open 请求及 requester、recipient、note 和恢复状态（`server/internal/service/collaboration_handoff.go:306`、`:326`）；handler 返回账号摘要与请求正文（`server/internal/handler/collaboration.go:196`）。前端 schema 对存在但损坏的请求拒绝解析，相关矩阵已在 `packages/core/collaboration/handoff-schema.test.ts:18`、`:34`。

新 UI 使用表单观察到的 `baseline.current_information_request`（`packages/views/collaboration/workflow.tsx:132`），展示该请求的 requester 与正文，使用 `RichContent resourcePolicy="restricted"`（`:155`）而不把跨空间内容按来源 workspace 资源渲染。restricted 的实际资源渲染语义有独立测试 `packages/views/rich-content/rich-content-parity.test.tsx:198`；本次 workflow 测试 mock 了 RichContent，因此它本身不证明这一渲染边界。

实际提交沿用同一个 baseline request ID（`workflow.tsx:142`），服务端在锁内确认它仍是当前 open 请求，且当前处理人正是 recipient（`collaboration_handoff.go:202`）；随后责任只交回原 requester（`:207`、`:234`）。新增展示没有授予原先没有的操作权限。

正常恢复状态已经通过服务端 `status-options` 返回单一默认选项（`server/internal/service/collaboration_lifecycle.go:161`、`:192`）；前端显示该选项（`workflow.tsx:129`、`:167`），不提供任意状态选择。原保存状态已失效时，新提示从请求视图的 `return_status` 显示回退目的地（`:158`）。这是恢复状态，不是新的处理人；处理人仍是 requester。

### 现有断言实际覆盖了什么

“请求人和 return_status 缺断言”并不符合当前代码：

- `shows the open information request to the person returning it`（`workflow.test.tsx:147`）在 `:151` 验证正文，`:152` 验证 `Requested by Requester`，`:153` 验证原请求成为说明框的 accessible description。
- `says where the issue returns when the request's saved status is gone`（`:157`）在 `:159` 验证完整回退状态文案，`:160` 验证空说明提示。
- 旧生产链路 e2e 已经通过接收方页面执行实际交回，并断言恢复到 requester 和原状态（`e2e/collaboration-handoff-chain-web.spec.ts:82`）。其 helper 验证单一选项和精确的 request ID、revision、epoch、status-options version 与 note（`e2e/fixtures/collaboration-handoff-chain.ts:59`、`:72`、`:75`）。这些是已存在的断言，需最终 HEAD 实际重跑，不能只凭源码算通过。

当前缺的是：该 e2e 在填写交回说明前，尚未断言接收方浏览器真正可见原请求正文与 requester。新 component 测试没有断言正常（非 fallback）恢复状态下拉的选中值，也没有做 submit；后者的提交和服务端状态行为已经由上述 e2e 的既有断言负责，不建议在 component 套件复制一整套业务矩阵。

### 首轮最小建议与必要性

首轮不需要为请求人、正文或 fallback 文案再造重复单测，也没有依据要求重做表单或新增流程。建议只做以下验收补口：

1. 在现有 `directedFormThroughUI` 的 return_information 分支中，提交前增加“原请求正文、请求人可见”断言，保持与 baseline request ID 对应。这样能关闭最初“收到请求的人看不到问题”的浏览器证据缺口。
2. 在正常返回状态的既有 component case 增加一个下拉值断言即可；不要把 service 的嵌套请求、撤权和状态矩阵搬到 DOM。
3. 最终 HEAD 重跑 workflow 两个新增用例、core handoff schema、restricted RichContent 的已有测试，以及 Web 接收方交回 e2e；Desktop 共用组件，但原生可见性/表单交回证据须单独记录，不能自动由 Web 通过推导。

这些属于“小范围验收证据补齐”，不是已证实的新增产品缺陷。补口前可以称展示实现已审查，不能称接收方实际界面验收完成。

## 首轮与后续范围边界

| 分类 | 本轮确认的事实 | 安排 |
| --- | --- | --- |
| 设计取舍 | 固定原始作者、冻结输入、续作合并、按单身份及本机执行权限继续保留 | 首轮不得为了让实验过而放松授权/身份门槛 |
| 实现偏差已修 | host prompt 丢角色/步骤；正式 smoke 少必要路由；交回表单漏请求正文 | 最终 HEAD 回归与最小浏览器补口 |
| 依赖模型自觉 | 成员主动读 delegation；队长从评论猜完成结果；结束前保存协作上下文 | E3 等正式实验注明限制，后续改结构化交接 |
| 缺少验证 | 修复后真实 E0、前后 E3、最终 HEAD 接收方 UI 和 Desktop 原生记录 | 未完成前不标“全部验收通过” |

对整体问题的回答：接手者现在可以通过新增表单读取人的补充问题，也可以在模型主动召回时取得派活正文；仍不能保证后续 Agent 获得完整成员结果、已处理输入和所有关键决定。三项补丁没有填平这一差距，规划应将其留在明确的后续交接改进里。
