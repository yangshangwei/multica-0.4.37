# Research: DeepSeek Harness 的工程物料管理参考

- Query: DeepSeek Harness 中哪些实际工程实践可用于组织产品需求、测试问题和 AI 协作物料，哪些不能直接照搬？
- Scope: external primary sources，结合本任务 D3/D7/D8 作适用性判断；不实施产品功能。
- Date: 2026-11-12
- Source: `deepseek-ai/deepseek-harness`，固定提交 `e8f7b93a344964de3dca1ace9e72b6e4d6de87cb`，当时默认分支为 `master`。本文外部源码链接全部固定到该提交。

## Findings

### 1. 这是工程实践参考，并非现成的业务物料仓标准

README 将项目定义为基于 Cordis、everything-is-a-plugin 的 agent harness，并标记为 developer preview。CONTRIBUTING 明确称仓库是 “an official showcase, and a source of inspiration, but not a mandate from us”。因此不能把本项目为 Multica 设计的 `requirements/`、`issues/`、PRD、缺陷编号或业务审批规则说成 DeepSeek 官方要求。

来源：[README.md:5–13](https://github.com/deepseek-ai/deepseek-harness/blob/e8f7b93a344964de3dca1ace9e72b6e4d6de87cb/README.md#L5-L13)、[CONTRIBUTING.md:9–19](https://github.com/deepseek-ai/deepseek-harness/blob/e8f7b93a344964de3dca1ace9e72b6e4d6de87cb/CONTRIBUTING.md#L9-L19)。后者让外部用户在 GitHub Discussions 报告问题，也不是规定把测试缺陷写进某个 Git 目录。

### 2. 一个事实只保留一个维护位置，入口文件按职责分层

`docs/AGENTS.md` 的直接原则是 “Each fact has one home … elsewhere, link there”。其分工包括：根 AGENTS 放每次会话都需要的简短规则；子目录 AGENTS 放局部规则；Agent Notes 放决策理由及放弃的方案；postmortem 放事故复盘；cookbook 放操作步骤；skills 放可复用工作流；生成的参考页由源码生成并检查新鲜度。

来源：[docs/AGENTS.md:15–36](https://github.com/deepseek-ai/deepseek-harness/blob/e8f7b93a344964de3dca1ace9e72b6e4d6de87cb/docs/AGENTS.md#L15-L36)。

可迁移建议：物料仓的根入口解释去哪里找需求、问题、规范和操作流程；PRD 正文、问题正文各有一个权威位置，其他文档通过稳定标识和链接关联。不要在需求目录、测试目录与知识库各复制一份相同缺陷正文；不要把每次运行日志都变成 skill。

### 3. Agent Notes 有明确目录、模板、分类及生命周期

实际布局是 `.agents/notes/{lifecycle}/{class}/yyyy-mm-dd-topic-title.md`。活动生命周期为 `proposed`、`implemented`、`rejected`；分类为 `feature`、`bug-fix`、`simplification`、`architecture`、`process`、`testing`。文件名日期取首次提出时间；相互引用使用可校验的相对 Markdown 链接。提案模板包含 Problem、Proposal、Alternatives considered、Acceptance criteria、Risks；已实施记录改为描述实际决定及后果。

来源：[notes/README.md:7–34](https://github.com/deepseek-ai/deepseek-harness/blob/e8f7b93a344964de3dca1ace9e72b6e4d6de87cb/.agents/notes/README.md#L7-L34)、[模板与状态转换:54–121](https://github.com/deepseek-ai/deepseek-harness/blob/e8f7b93a344964de3dca1ace9e72b6e4d6de87cb/.agents/notes/README.md#L54-L121)。

这不只是文字约定。`scripts/agent-note-tree.ts:12–22` 定义封闭枚举，`:41–79` 校验目录深度、分类及文件名。源码来源：[agent-note-tree.ts](https://github.com/deepseek-ai/deepseek-harness/blob/e8f7b93a344964de3dca1ace9e72b6e4d6de87cb/scripts/agent-note-tree.ts#L11-L79)。本次只读代码，未运行该仓库检查。

适用边界：这些是决策记录的文档生命周期，不是需求/缺陷的实时执行状态。Multica 已按 D3 承担指派和进度，不能据此把业务物料按 `todo/`、`running/`、`done/` 反复搬目录。保留稳定业务路径与内容版本，更符合当前分工。

### 4. 上游明确反对集中维护 Agent Note 总索引

`notes/README.md:19` 要求把活动目录树作为 working inventory，并明确 “Do not add a centralized INDEX.md”；理由是重复路径元数据，而且不相关的修改会争抢同一生成文件。`agent-note-tree.ts:47–49` 直接将此类根索引判错。

来源：[notes 索引规则](https://github.com/deepseek-ai/deepseek-harness/blob/e8f7b93a344964de3dca1ace9e72b6e4d6de87cb/.agents/notes/README.md#L17-L19)、[实际校验](https://github.com/deepseek-ai/deepseek-harness/blob/e8f7b93a344964de3dca1ace9e72b6e4d6de87cb/scripts/agent-note-tree.ts#L44-L49)。

可迁移建议：避免一份所有 Agent 都手改的全仓 `index.json`/任务总表。物料元数据放在各自文件或目录，Multica 可扫描构建可重建的索引；需要阅读入口时使用小型目录说明与链接。此规则只针对上游 Agent Notes 的集中索引，不能误称上游禁止所有导航、目录页或生成索引。

### 5. 长期知识要处理替代关系，归档不是删除历史

新 Agent Note 必须搜索旧记录并做完整/部分替代判断。更新已有决策的事实时修改其原记录；不同决策用新记录替代并交叉链接。仍有价值的部分替代不能直接删除。只有满足条件的已实施记录才能归档，归档内容冻结，不作为当前权威；上游通过归档清单及摘要校验约束修改。

来源：[notes/AGENTS.md:3–7](https://github.com/deepseek-ai/deepseek-harness/blob/e8f7b93a344964de3dca1ace9e72b6e4d6de87cb/.agents/notes/AGENTS.md#L3-L7)、[notes/README.md:36–50](https://github.com/deepseek-ai/deepseek-harness/blob/e8f7b93a344964de3dca1ace9e72b6e4d6de87cb/.agents/notes/README.md#L36-L50)、[归档规则](https://github.com/deepseek-ai/deepseek-harness/blob/e8f7b93a344964de3dca1ace9e72b6e4d6de87cb/.agents/notes/archived/AGENTS.md#L3-L7)。

可迁移建议：将 bug 修复产生的稳定知识、规范变更与 skill 候选进行去重、验证和审核；记录来源及替代关系。原始缺陷、复盘、规范与 skill 是不同对象。普通缺陷无需一律升格为复盘或 skill；上游复盘标准要求问题机制隐蔽、逃逸原因具有系统性且重新发现成本高，见 [postmortem/README.md:5–11](https://github.com/deepseek-ai/deepseek-harness/blob/e8f7b93a344964de3dca1ace9e72b6e4d6de87cb/docs/postmortem/README.md#L5-L11)。

### 6. 让 Agent 按范围加载规则和流程，不是每次读取整个物料仓

指令插件按项目根到工作目录的顺序加载 AGENTS/CLAUDE，并在文件操作进入更深目录后补充相关规则；内容进入持久会话记录，有独立的上下文预算。skill provider 先发现 YAML 元数据，正文按需加载；兼容 `<name>/SKILL.md` 和扫描根下的 `<name>.md`，不会任意递归发现所有 `**/SKILL.md`。

来源：[agent-instructions/README.md:12–38](https://github.com/deepseek-ai/deepseek-harness/blob/e8f7b93a344964de3dca1ace9e72b6e4d6de87cb/packages/context/agent-instructions/README.md#L12-L38)、[skill-filesystem/README.md:28–56](https://github.com/deepseek-ai/deepseek-harness/blob/e8f7b93a344964de3dca1ace9e72b6e4d6de87cb/packages/skill/skill-filesystem/README.md#L28-L56)。

可迁移建议：顶层只放简明定位与写入规则，需求、测试和知识目录拥有各自范围说明；一次运行通过固定版本清单获取当前需求、关联问题、验收条件及必要规范。不直接把所有 PRD 当作自动执行指令，也不能因为采用同名 `AGENTS.md` 就声称 Multica 已具有 Harness 的指令加载实现。

### 7. 计划、临时待办与运行产物有运行时所有者

上游 `todo_write` 是单一 agent session 所有的列表，每次整体替换，保存在 `todo/write` 会话事件中；它不是多人共享的业务任务仓。plan mode 先探索并提交方案审核，但文档明确这种模式依靠指导文本，不通过隐藏工具提供强制约束；计划模式状态在会话日志中，完整 Markdown 也保留在调用/dispatch 日志中。deliverables 同样把交付文件和每轮改动记录为持久 Session 事件。

来源：[tool-todo/README.md:50–75](https://github.com/deepseek-ai/deepseek-harness/blob/e8f7b93a344964de3dca1ace9e72b6e4d6de87cb/packages/todo/tool-todo/README.md#L50-L75)、[plan/README.md:12](https://github.com/deepseek-ai/deepseek-harness/blob/e8f7b93a344964de3dca1ace9e72b6e4d6de87cb/packages/plan/README.md#L12)、[plan-mode/README.md:81–109](https://github.com/deepseek-ai/deepseek-harness/blob/e8f7b93a344964de3dca1ace9e72b6e4d6de87cb/packages/plan/plan-mode/README.md#L81-L109)、[deliverables/README.md:12](https://github.com/deepseek-ai/deepseek-harness/blob/e8f7b93a344964de3dca1ace9e72b6e4d6de87cb/packages/deliverables/README.md#L12)。

适用边界：本项目可以借鉴“每类对象有唯一所有者、运行状态可恢复”，但物料仓的业务任务定义不能用一份 agent todo 列表替代。D8 的项目级开关、待办不派发和方案审核后的编码限制仍需 Multica 服务端实现；引用上游 plan mode 不会自动提供这些产品保证。

### 8. 目录与内容管理应当有机器检查，验收读实际结果

上游文档流程要求检查分类、链接、模板与生成物新鲜度，并通过 `test:docs`/`doc-sync` 执行。测试政策专门写明 “Verify the world, not the self-report”：重新读文件或执行外部验证，不能只检查 Agent 宣称成功的文本。

来源：[dsh-doc-writing/SKILL.md:117–128](https://github.com/deepseek-ai/deepseek-harness/blob/e8f7b93a344964de3dca1ace9e72b6e4d6de87cb/.agents/skills/dsh-doc-writing/SKILL.md#L117-L128)、[docs/testing.md:33–41](https://github.com/deepseek-ai/deepseek-harness/blob/e8f7b93a344964de3dca1ace9e72b6e4d6de87cb/docs/testing.md#L33-L41)。

可迁移建议：物料提交检查稳定 ID 唯一、必填元数据、关联对象可解析、受控路径、结构版本和附件引用；合并后读取实际 Git 内容与提交，而不是相信 Agent 的“已经提交”。测试缺陷应绑定被测需求版本、代码/构建版本、复现步骤、预期/实际结果和可验证证据。具体字段与状态仍是本项目的设计，不是上游现成格式。

## Files found

| 上游文件/目录 | 本次用途 |
| --- | --- |
| `README.md`、`CONTRIBUTING.md` | 确认项目定位、成熟度和公开问题反馈方式 |
| `AGENTS.md`、`docs/AGENTS.md` | 全局/局部规则分层、一个事实一个归属及文档检查 |
| `.agents/notes/README.md`、其 `AGENTS.md` | 决策记录分类、模板、替代、归档、反集中索引规则 |
| `scripts/agent-note-tree.ts` | 核验规则存在于实际目录检查器 |
| `.agents/skills/dsh-doc-writing/SKILL.md` | 元数据、按需阅读、文档投影和验证流程 |
| `docs/postmortem/README.md`、`docs/testing.md` | 复盘与实际结果验证的边界 |
| `packages/context/agent-instructions/README.md` | 按范围加载并记录指令上下文 |
| `packages/skill/skill-filesystem/README.md` | skill 文件格式、目录发现、正文按需加载 |
| `packages/todo/tool-todo/README.md` | 会话待办的所有权与持久化 |
| `packages/plan/plan-mode/README.md` | 方案审核入口及会话内状态 |
| `packages/deliverables/README.md` | 文件交付与变更证据的运行时记录 |

## Related specs

- [本任务 PRD](../prd.md)：D3 的 Git/Multica 数据归属、D7 的物料合并、D8 的项目级启动开关仍是本项目约束；上游参考不覆盖或撤销这些决定。
- [本任务设计](../design.md)：稳定物料身份、固定输入版本、受控 Git 写入及恢复规则。
- [Trellis 工作流](../../../workflow.md)：规划研究落盘；当前任务继续保持 planning。
- [跨层思考指南](../../../spec/guides/cross-layer-thinking-guide.md)：目录/元数据、Git 写入、Multica 投影与 Agent 输入需要贯通核对。

## Caveats / Not Found

- 在上述实际读取的项目入口、Agent Notes、文档规则与运行包中，未找到用于产品 PRD 与 QA 问题的官方业务物料目录规范；不把这个有限范围的观察扩大成“全仓任何角落都没有相关文件”。
- `.agents/notes/{lifecycle}/bug-fix/` 存放有长期价值的修复决策理由，不等于用户报送的所有缺陷；postmortem 也不是普通缺陷清单。
- 未找到可直接沿用的 Multica 跨工作空间物料 ID、需求/缺陷关系、产品审批映射、项目启动开关或物料 PR 自动合并方案，这些须由本项目设计。
- 上游文档说明、目录检查器源码和本次定向读取能够支持上述设计借鉴，不能证明上游所有流程已在本机验证；未安装或运行 Harness、未执行其测试，也未调用真实 Agent。
- 对上游的说明与可迁移建议已分开。提议的 Multica 目录、元数据及维护自动化应在主设计中列为待评审方案，不能作为已经确认的 D9 或已交付功能。
