# 内置研发智能体第一阶段：角色模板、Squad 模板、自治等级与审批边界

来源：`.omx/plans/builtin-agent-rollout.md`（Phase 0/1，以及 Phase 2/3 中可执行的部分）。

## Goal

让一个新工作区能在几分钟内staff出一支标准研发团队：从内置角色模板创建普通 workspace Agent、从 Squad 模板一次拉起整条路由链，并让「这个智能体被允许做到哪一步」成为服务端真正执行的规则，而不是界面上的标签。

前提约束：**不改变任何既有 Agent、Skill、Squad、Task 与自动化的行为**。

## 已确认决策

1. **模板是复制语义，不是 Mika 那种服务端分层。** 创建时把指令复制进 `agent.instructions`，工作区从此拥有它；记录 `template_key` + `template_version` 作为来源，供后续升级给出差异而不是替换。
   理由：计划书明确要求「模板只能复制为工作区 Agent 后修改」且「升级不覆盖工作区管理员的本地修改」。Mika 的热更新模型与这两条直接冲突。
2. **首批 8 个可选角色，不按技术栈拆分。** 前端/后端/移动端不是独立模板，而是同一个 Implementer 配不同 Skills 与项目资源。
3. **角色 Skills 不进 `BuiltinSkills()`。** 那个集合会发给每个智能体的每次执行；新增 7 个常驻 Skill 等于给全平台所有提示词加长。改为按需 materialize 成工作区 Skill 并绑定到该智能体。
4. **自治等级必须可执行。** 仅展示标签不算完成；服务端要在智能体自己调用 API 时真的拒绝越权请求，并把同一套规则注入它读到的提示词。
5. **审批是可审计边界，不是沙箱。** 任务继承 daemon 用户权限，服务端拦不住 shell 命令。因此第一阶段交付的是「谁在何时批准了哪一个动作」的记录与门禁，文档必须如实这么写，不得暗示更多。
6. **Squad 仍是 leader 路由模型**，不引入隐式并行。

## Requirements

### R1 内置角色模板（8 个）

- Product Analyst、Architect、Implementer、QA Engineer、Code Reviewer、Security Reviewer、Release Engineer、Technical Writer。
- 每个模板必须写明：职责、不负责的事、开始前读什么、输出格式、完成标准、何时上抛给人类。
- 模板指令为英文（与仓库既有 agent-harness 文本一致）；选择器展示用的标题与描述需 en/zh/ko/ja 四语。
- 模板需带版本号；选择器要能在创建前展示完整指令原文。
- 每个模板默认附带的 Skill 数量上限为 1（防止提示词膨胀）。

### R2 从模板创建普通 workspace Agent

- 创建结果是普通 Agent：同一张表、同一套 Access、同一套 Runtime 与 Task 生命周期，既有页面无需特殊分支。
- 复用既有创建路径的全部校验：runtime 归属与可用性、thinking level / service tier、名称唯一冲突（409）、invocation permission。
- 请求只接受人真正选择的字段（名称、runtime、model、thinking、tier、Access、语言）。指令、Skills、并发上限、自治等级一律由服务端决定。
- 公开的 `POST /api/agents` **不得**接受 `template_key` 或 `autonomy_level`：否则客户端可以伪造来源，或凭空造出一个 Operator。
- Access 缺省「仅自己」，与从空白创建一致；放开必须是显式选择。

### R3 角色 Skills（7 个）

- 需求澄清、ADR、测试报告、代码审查、安全审查、发布检查、文档变更。
- 首次需要时写入工作区 Skills；同名 Skill 已存在时**按原样复用，永不覆盖**。
- 记录来源与版本，且该来源类型不得被「从来源更新」识别（角色 Skill 通过模板升级，不从 URL 重新拉取）。
- 同一角色的第二个 Agent 必须绑定同一份 Skill 记录，使一次修改覆盖所有使用者。

### R4 Squad 模板（2 个）

- Feature Delivery Squad：Lead + 产品分析 → 架构 → 实现 → 测试 → 代码审查。
- Bug Fix Squad：Lead + 定性 → 实现 → 验证。
- 一次调用建齐缺失的角色 Agent、squad 行与全部成员关系；**要么全部成功，要么什么都不留**。
- 工作区已有的角色 Agent 必须复用且不作任何修改（包括被改过的指令与被调低的自治等级）。
- 若某个非模板 Agent 已占用某角色的默认名称，请求失败并说明，不得静默接管一个指令不明的 Agent。
- 路由规则复制进 `squad.instructions`，工作区可改，发布不覆盖。
- Squad leader 的自治等级为 Coordinator。

### R5 自治等级（四级）

- Observer / Contributor / Coordinator / Operator，累加语义。
- **未声明等级的智能体不受任何影响**——这是所有历史智能体的状态，行为必须逐字节不变（含提示词）。
- 服务端在智能体自己发起的 API 请求上执行：
  - Observer 不能改 issue 状态/负责人，不能建 issue；
  - 常驻自动化与 Squad 写操作需要 Coordinator；
  - 记录高风险动作的执行需要 Operator。
- 同一套规则由服务端在每次 claim 时注入提示词，工作区无法删除该段。
- 只有人类可以修改某个 Agent 的等级：智能体的 task token 携带 owner 的 user id，必须单独拦住这条自提权路径。

### R6 高风险动作的人工审批

- 五类：生产发布、对线上数据迁移、读取密钥、对外通知、不可逆删除。
- 任何智能体都可以**提出**申请（申请许可永远是安全方向）；只有人类可以**决定**。
- 未获批准时，Operator 的交付物就是计划本身。
- 一次审批只覆盖它描述的那一个动作。
- 申请、决定、执行结果全部可追溯。
- 智能体必须有可用的入口（CLI），否则它会自行发明一种做法。

### R7 版本化与不覆盖

- 模板与角色 Skill 都带版本；升级不得覆盖工作区管理员的本地修改。
- Agent 与 Squad 都记录来源模板与版本。

### R8 测试、文档与规范同步

- 单元、集成、E2E 覆盖关键逻辑。
- `agents` / `agents-create` / `squads` 文档四语更新。
- 按仓库规则同步受影响的内置 Skill（`multica-creating-agents`、`multica-squads`）及其 source map。

## Constraints

- 不新增依赖。
- 遵守既有包边界、API schema（`parseWithFallback`）、权限模型与迁移规则：不加外键；索引必须 `CREATE INDEX CONCURRENTLY` 且各自单语句文件。
- 前端不得把服务端枚举当成封闭集合：未知自治等级/审批状态必须能渲染，且未知状态一律视为「未批准」。
- 生产发布、数据库迁移、密钥读取默认不放行。

## Acceptance Criteria

- [x] 新工作区可从 8 个模板中创建 Agent，创建后可正常分配 issue、@提及、执行。
- [x] 创建出的 Agent 的 `instructions` 与模板原文逐字相同，并带 `template_key`/`template_version`/`autonomy_level`。
- [x] `POST /api/agents` 传入 `template_key` 与 `autonomy_level` 时，创建结果两者均为空。
- [x] 工作区已有同名角色 Skill 被改写过时，再次创建该角色复用它且内容不变，工作区中该名称只有一份。
- [x] 两个 Squad 模板可各自一次拉起完整花名册；先后创建时共用同一个 Implementer。
- [x] 非模板 Agent 占名时 staffing 返回 409，且不留下任何 squad 或 Agent。
- [x] Observer 智能体改 issue 状态/负责人、建 issue 均被拒绝且数据未变；Contributor 可以。
      **覆盖每一条写这些字段的路由**：`PUT /api/issues/{id}`、`POST /api/issues/batch-update`，
      以及显式 null 的取消分配。第一版只堵了前者且只看指针非空，两个绕过都在 2026-09-06 的
      审核里实测确认后补齐（见 `implement.md` S10）。
- [x] 未声明等级的智能体在同样操作上不受影响，claim 到的指令与其自身文本完全一致。
- [x] 智能体无法通过 API 提升自己的自治等级。
- [x] 审批：智能体不能决定（含自己的申请）；重复决定返回冲突；未批准/已拒绝无法记录执行；Contributor 持有已批准申请仍不能记录执行。
- [x] 一次申请的申请、决定、执行三条审计记录齐全。
- [x] 人类有可用的审批入口：`/{ws}/agents/approvals` 队列页，从智能体页面进入并显示待处理数量；
      批准需二次确认并复述被授权的那一个动作。
- [x] 既有 Agent/Squad/自动化的现有测试与行为保持不变（全量 Go 与 TS 套件）。
- [x] E2E：从模板创建 Agent 的完整流程在真实应用中通过（2026-09-06 执行，连续两次通过；顺带修掉 spec 自己的无主 runtime fixture bug，见 `implement.md` S10.4）。

## Out of scope（本阶段不做）

- Phase 4 的模板质量看板、固定评测集、按工作区灰度可见性。
- 按角色分别选择 runtime 的 Squad staffing 界面。
- 模板版本升级的差异对比与手动升级动作（本阶段只保证记录了版本，不覆盖）。
- `agent create --from-template` 之类的 CLI 入口。
