# 技术设计（待独立评审）

## 最小行为差异
现有 task_token 绑定 user/agent/task/workspace，角色 autonomy 仅阻断部分写动作。增加服务端冻结的任务能力，并在 Auth 入口统一失败关闭，仍执行原处理器授权。避免在每条路由复制 allow/deny 规则。

## 数据与身份
在 task_token 增加可空 JSON capability（旧令牌 NULL 保持旧行为）。v1 文档含 profile、issue/project UUID、originator UUID、claim generation、runtime UUID；严格校验未知版本／不完整值，拒绝而不降级。由 FinalizeTaskClaim 内锁定当前任务和可信角色来源决定，不能由请求体、显示名、prompt 或客户端设置。角色 key + 最低新模板版本决定新策略；不可复用较高自治等级绕过本策略。sqlc 生成；无外键、无新增索引。down 在存在受限凭据时拒绝丢弃安全约束。

新模板：progress-reporter 原1→2；code-reviewer/security-reviewer 原1→2；migration-reviewer 原2→3（实施前核对）。已有实例版本不写入、不升级。新实例非 issue 任务也不能获得无限制凭据；可拒绝 claim 并给出明确诊断，后续另建受限 chat 能力。

## 请求检查
Auth 两种认证模式均将已验证 task token 的 capability 存入私有 context。使用同一 guard 验证当前 task 的 agent/runtime/issue/originator/generation/status，以及 token 的 workspace/user 绑定。当前 runtime owner 和 originator 都必须仍具工作空间权限，originator 仍能 invoke 当前 agent（复用既有规则）；缺失或撤销拒绝，数据库错误503。

严格允许列表：GET 当前 issue（支持已解析的 UUID/human identifier），GET 允许 issue 的 comments/timeline；报告员另可 GET /api/issues 且只有精确单一 project_id 等于冻结项目，防重复／别名／忽略过滤参数。实际列表处理器的可见性与过滤能力须检查。其他路径／方法包括 WebSocket、搜索、技能全库、附件、各类代理端点默认拒绝。POST 当前 issue/comments 允许；把受限 context 传至真实评论调度入口，使显式／隐式／延后唤醒全部不发生。结果保留评论但不能声称委派成功，显式目标返回 blocked 的现有 outcome 结构和稳定原因。

读权限始终受当前 project/issue workspace、当前 originator 及原处理器授权交集约束；项目移动不能扩权。只限制 mat_，不根据不可信头给人类授予机器身份。

## 工具与领取
同一 shared claim 构造逻辑对受限角色移除平台注入的 MCP config、remote MCP connections、plugin tools，不产生 mdt_。FinalizeTaskClaim 对受限 token + daemon token 组合拒绝，防未来入口绕过。技能仍可随任务内联提供；必要懒加载需严格绑定本 task，不能开放全库。

## 影响与取舍
角色不再自己改变 in_review。模板／role skill 明确提交意见并交还业务状态管理；已有人工 done 规则不变。评论不自动唤醒 leader，首批由已有有权主体继续推进；有效委派子任务须区分交付与执行。新政策窄于旧权限，不扩大能力。不引入通用权限服务、新依赖或 shell 沙箱；后续高风险执行独立设计。

## 回退
原子提交可回退，但生产回退需先撤销受限 token，再回退实现／schema；保留受限令牌时 down 拒绝。本任务只本地验证，不执行真实回退部署。

## 独立评审后修订 R1
评论禁派发复用已有持久 comment.dispatch_eligible，不只依赖请求context。CreateComment增加服务端only suppress_execution输入，与admission条件取AND（默认false保持旧调用），立即路由与reconcile都读取持久值。API评论的普通事件带已有suppress_execution，禁止自动解开resolved线程和取消deferred escalation。任务完成自动生成的评论也必须设置相同限制：允许在agent_task_queue保存claim冻结capability（与token同事务），防token删除后失去限制；对reclaim不得清除已冻结限制。完整检查dispatch_eligible现有消费者以避免引入第二派发机制。DB故障拒绝派发，不降级。

受限claim若配置外部MCP/plugin工具，首批拒绝并返回明确unsupported-policy原因，避免悄悄移除用户配置。不启动实际provider；没有工具配置的claim正常可执行。服务层防止为受限task生成mdt_；daemon工具端点以持久task scope拒绝。如果选择直接收窄注入，需要在claim指令明确该限制且保留服务端backstop。

## 授权实现层的边界补充
资源解析与invoke判断优先放在现有handler层复用loadIssueForUser/invocation decision；Auth仅负责把验证过的token信息传入不可由header伪造的context。可采用handler.TaskCapabilityGuard在每个Auth路由组之后统一挂载，而非让middleware依赖handler或复制invoke算法。必须检索全部Auth调用，覆蓋两个普通认证入口；独立的WS/DaemonAuth仍保持mat_拒绝。无scope的旧token保持旧行为，但新受限角色缺少scope的token拒绝，防漏签发路径。

## 独立评审采纳 R2（可开始实施）
- 采用plan-review.md全部实现约束。canonical issue loader对实际消费的row验证scope，不接受前置guard检查后再次无检查reload。GET detail/comments/timeline使用一致授权snapshot或有界资源锁维持project范围；列表SQL自身AND冻结project。POST comment在同一写入事务检查当前task/principal/issue scope并写入（避免自持读锁死锁）；不宣称仅靠中间件可保证原子撤权。加入move/revoke确定性竞态测试。
- handler层guard查询当前invoke目标和members后调用已有纯判定，错误明确503；不得复制权限模型或引入middleware→handler循环。
- claim响应的源issue/project/agent身份必须与Finalize锁定数据一致，失败不给令牌。新增高优先级能力brief：只读范围、当前任务评论、状态由人类／原有有权主体推进、没有自动队长唤醒、平台工具限制。不得仅改模板并留下通用runtime矛盾。
- 本子任务的读取保证是HTTP API资源访问，不宣称模型上下文天然仅含这些资源：既有任务指令、用户输入、历史session、宿主文件均不是新scope生成的授权。受限claim应去除跨范围siblings摘要并清除未证明兼容的prior-session恢复；冻结当前受限上下文后才可再研究恢复。不借此重构collaboration runtime。
- 允许资源附带的附件内容属于其read范围；不得凭写API绑定已有附件。已有签名下载有效期沿用现有机制，任务令牌撤销不等价于撤销已发出的短期URL；此限制必须在验证范围记录。不新增attachment grant或增加签名TTL。
- rollback在一个DO block内以确定顺序ACCESS EXCLUSIVE NOWAIT锁token／task表，再EXISTS检查和DDL；非空能力或并发mint不得失去策略。私有fixture schema测试，不改任务外数据。
- completion可由不可变template key/version或冻结task capability判定受限，读失败禁止派发。实现以最小持久字段满足需求，不重复派发系统。
