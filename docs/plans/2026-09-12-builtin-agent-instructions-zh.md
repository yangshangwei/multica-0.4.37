# 内置智能体指令中文维护方案

日期：2026-09-12。状态：源码、转换工具及用户指定的“海卫三”实例转换全部完成。此文件保留最初调研快照，末尾记录实施和实际转换证据。

## 结论

建议在当前中文团队部署中，将内置指令的正式维护正文改为简体中文，继续让编辑框展示真实保存、实际执行的内容。复用现有 Markdown 文件、`instructions` 字段和保存链路，不引入实时翻译服务或两份可分别编辑的中英文正文。

先解决 17 份智能体指令和 8 份小队指令，同时处理已有实例。Mika 保留产品指令只读、团队补充可编辑的结构。7 份配套角色 skill 正文已经中文化，可直接沿用术语。

本建议以“本团队部署中文优先”为边界。直接翻译服务端内置文件会影响该部署下所有工作区的新建模板，以及所有 Mika 后续执行的系统段；不会根据查看者 UI 语言分别选择正文。如果这个部署也服务其他语言团队，应采用后文的按实例固定指令语言方案。

## 需求与范围

- 团队可以用中文阅读、修改、保存和维护角色指令。
- 覆盖工作角色、小队负责人、Mika，以及影响负责人行为的小队公共指令。
- 中文化保持原职责、权限、审批、状态流转、输出内容要求和协作规则。
- 已有实例中的团队定制必须保留；切换个人界面语言不能改写团队共用指令。
- 沿用 `apps/docs/content/docs/developers/conventions.zh.mdx`：任务、工作区、智能体等使用中文，CLI、路径、字段、状态值和 skill 规范名称保留原样。
- 本轮提供源码调研和实施方案，不批量更新已有数据，不发布服务，不调用真实智能体。

## 全部内置指令清单

以下为实施前的仓库模板清单，不是线上工作区已创建实例数量。角色注册表：`server/internal/service/builtin_agent_templates_roster.go:14`。前 8 个工作角色可从智能体模板选择器创建；后 8 个负责人由小队模板配置流程创建。当前这些正文已经中文化。

| 分类 | 角色 | key | 行为版本 | 自主权限 | 指令现状 |
|---|---|---|---:|---|---|
| 系统助手 | Mika | `mika` | 系统版本随服务发布 | 独立系统契约 | 英文，产品层只读 |
| 工作角色 | 产品分析师 | `product-analyst` | 1 | observer | 英文 |
| 工作角色 | 架构师 | `architect` | 2 | observer | 英文 |
| 工作角色 | 实现工程师 | `implementer` | 1 | contributor | 英文 |
| 工作角色 | 测试工程师 | `qa-engineer` | 1 | contributor | 英文 |
| 工作角色 | 代码审查员 | `code-reviewer` | 1 | observer | 英文 |
| 工作角色 | 安全审查员 | `security-reviewer` | 1 | observer | 英文 |
| 工作角色 | 发布工程师 | `release-engineer` | 2 | operator | 英文 |
| 工作角色 | 技术文档工程师 | `technical-writer` | 2 | contributor | 英文 |
| 小队负责人 | 特性交付负责人 | `feature-delivery-lead` | 1 | coordinator | 英文 |
| 小队负责人 | 缺陷修复负责人 | `bug-fix-lead` | 2 | coordinator | 英文 |
| 小队负责人 | 合并门禁负责人 | `review-gate-lead` | 2 | coordinator | 英文 |
| 小队负责人 | 需求预研负责人 | `discovery-lead` | 1 | coordinator | 英文 |
| 小队负责人 | 文档同步负责人 | `docs-lead` | 2 | coordinator | 英文 |
| 小队负责人 | 例行维护负责人 | `maintenance-lead` | 2 | coordinator | 英文 |
| 小队负责人 | 发布负责人 | `release-lead` | 2 | coordinator | 英文 |
| 小队负责人 | 事故响应负责人 | `incident-lead` | 2 | coordinator | 英文 |

Mika 原文：`server/internal/service/builtin_agents/mika/INSTRUCTIONS.md:1`。

16 个角色原文：`server/internal/service/builtin_agent_templates/<key>/INSTRUCTIONS.md:1`。

还应关联处理的 8 个小队公共指令位于 `server/internal/service/builtin_squad_templates/<key>/INSTRUCTIONS.md:1`，key 为 `feature-delivery`、`bug-fix`、`review-gate`、`discovery`、`docs`、`maintenance`、`release`、`incident`，均为英文。这些正文存入 `squad.instructions`，与负责人自己的 `agent.instructions` 是两个字段。

7 个角色 skill 位于 `server/internal/service/builtin_role_skills/`：需求澄清、架构决策记录、测试报告、代码审查、安全审查、发布检查、文档更新。正文已经中文化，英文 frontmatter 和规范名称是有意保留的。来源：`.trellis/spec/server/builtin-templates.md:18`。

## 现状与原因

| 环节 | 当前行为 | 对方案的影响 |
|---|---|---|
| 中文界面 | `zh-Hans/agents.json:543` 的介绍和标签仍有 `System Prompt` | 改为“系统指令”，说明这是角色规则 |
| 模板语言 | `agent_template.go:69` 只翻译标题、简介；正文调用无语言参数的 `Instructions()` | 仅补前端 i18n 不会改变正文 |
| 普通智能体编辑 | `instructions-tab.tsx:39` 从 `agent.instructions` 初始化，`:174` 原样保存 | 已支持中文文本，可复用现有链路 |
| 创建角色 | `agent_template.go:187` 将模板正文复制入实例；`:206` 保留来源 | 翻译模板只影响后续创建 |
| 模板来源 | `agent.ts:507` 的 key/version 只记录来源 | 版本相同不代表正文未经修改 |
| Mika | `builtin_agents.go:58` 每次读取内嵌系统段，`:73` 拼接团队补充 | 发布新正文会影响既有 Mika 后续执行，但不覆盖补充 |
| 小队复用 | `squad_template.go:458` 复用旧角色，不重置正文、权限或运行时 | 新建小队也可能复用仍为英文的旧角色 |
| 运行时 | `handler/daemon.go:2167` 取真实正文；`execenv/runtime_config_sections.go:109` 原样写入 Agent Identity | 无英文输入限定，无需把中文再翻译成英文 |

上述简写路径分别属于 `packages/views/locales/`、`server/internal/handler/`、`packages/views/agents/components/tabs/`、`packages/core/types/`、`server/internal/service/`、`server/internal/daemon/`。

另有一处容易误导团队的中文 placeholder：`packages/views/locales/zh-Hans/agents.json:552` 写着“即使聊天用中文，任务也一律用英文写”。这是界面示例，不是已执行的指令，应替换成中文团队的上下文示例。

实际运行还会叠加自主权限政策、小队协议与成员名单、运行时工作规则，以及可用的 skill。证据：`server/internal/handler/daemon.go:2198`、`:2346`，`server/internal/handler/squad_briefing.go:162`。因此页面上的指令字段不是最终完整 prompt。第一阶段翻译团队需要维护的正文；这些平台注入规则保留现有行为，并在编辑框说明中交代叠加关系。若以后需要完整执行指令预览，应按具体任务上下文只读展示，另行设计。

## 方案比较

| 方案 | 能否中文维护 | 代价与限制 | 判断 |
|---|---|---|---|
| 翻译标签或显示临时译文 | 只能改善阅读；保存内容可能仍是英文 | 容易出现看到、修改、执行三者不一致 | 不满足本次核心需求 |
| 单份中文正式正文 | 可以，编辑、保存和执行一致 | 需翻译模板并显式迁移已有实例；部署默认正文变成中文 | 推荐用于当前中文团队 |
| 按实例固定指令语言 | 可以，同时支持多个语言团队 | 需正文多语言资源、固定语言元数据、创建与预览一致性、历史版本与迁移规则 | 确有多语言工作区需求时采用 |

第三种方案中，个人 UI 语言只作为“新建指令语言”的建议值；创建后固定到实例，查看者切换 UI 语言不改变其共享正文。Mika 还需明确工作区级指令语言，因为它的系统段不存实例正文。不能只给 GET 接口加一个临时翻译参数就宣称完成多语言维护。

## 推荐设计

### 中文正文与编辑体验

1. 在现有 `INSTRUCTIONS.md` 中维护中文正式正文，沿用 Markdown 结构：职责、不负责的事项、输入、工作方式、交付格式、完成标准、需要人工介入的情况。逐段保留原规则，不统一抹平不同角色的职责。
2. 界面标签用“系统指令”；说明可写为“设定该智能体的角色和工作规则，支持 Markdown。运行时还会叠加平台规则及已绑定的 skill。” 模板预览、创建后详情、保存后重读都显示同一正文。
3. Mika 显示中文系统段，保留只读属性；“工作区补充”继续编辑现有 `instructions`。`builtin_agents.go:44` 的补充分隔与说明也要翻译，`{{AGENT_NAME}}` 必须保留并正常替换。
4. 切换界面语言不翻译、不覆盖编辑框正文，也不生成脏状态。英文原文保留在 Git 历史及迁移备份中，不在产品里增加第二个可编辑正文。
5. 更新源码中“指令固定英文”的旧注释与文档，主要为 `builtin_agent_templates.go:123` 和 `packages/core/api/client.ts:1561`。代码注释本身继续使用英文。

### 已有实例转换

现有 `template_key`、`template_version` 没有原始正文快照、语言或内容哈希。普通更新 SQL 仅按 ID 更新，未提供正文比较交换：`server/pkg/db/queries/agent.sql:139`。不要把调用 API 前后各读取一次当成并发安全。

实施时先对明确的目标工作区做只读盘点，导出实例 ID、来源、版本、原正文、更新时间与内容哈希，并生成可审阅的转换清单。备份放在受控本地目录，不提交团队私有指令到 Git。

| 实例情况 | 转换方式 |
|---|---|
| 正文与可识别的历史默认全文一致 | 应用那个历史版本对应的中文译文；不顺便升级成当前角色行为 |
| 正文有团队修改 | 根据实际正文单独生成中文版本，对照保留团队新增、删除和调整的规则后再更新 |
| 已经是中文 | 保持原样 |
| 无来源、来源不明或历史正文无法确定 | 保留原文并标出，逐个审阅；不能按角色名称或文本前缀匹配覆盖 |
| Mika | 系统段随代码发布；团队补充按实际内容单独处理，不用模板替换 |

推荐用一次性、限定工作区的维护工具执行选定清单；逐条事务更新必须同时匹配实例 ID、工作区 ID、原正文和导出时的 `updated_at`。有冲突就跳过并报告，不能重试覆盖新内容。仅修改正文和更新时间，沿用正常变更的事件通知/缓存失效约定。小队正文遵守相同规则。

回滚也必须检查当前正文和更新时间仍与本次写入匹配，再恢复备份；后来又被人修改的实例不能自动回滚。维护工具必须提供 dry-run、变更结果清单和重复执行无变化的行为。开发这项工具不等于现在执行数据变更。

### 保持行为与版本诚实

- 架构师仍是 observer，只交付评论中的方案或 ADR 草稿；技术文档工程师仍只在隔离分支改文档。
- 发布工程师仍是 operator，每项高风险动作走记录化审批；发布负责人仍是 coordinator。不能因为翻译而把二者混为一谈。
- 小队负责人保留路由、交接和父任务状态边界，不能把“分配了工作”翻成“已完成交付”。
- `observer`、`contributor`、`coordinator`、`operator`、状态值、风险类别、CLI、路径、模板 key、mention 格式与占位符都原样保留。
- 此次只改语言，保留行为版本，用源码提交及迁移清单的前后内容哈希记录变更；不得依赖行为版本区分同版本的中英文正文。若同时改职责或输出语言规则，另作行为变更并更新版本。
- “中文写指令”和“强制中文输出”分别处理。Mika 已要求跟随成员/任务语言回复（`mika/INSTRUCTIONS.md:5`）。本次保留该规则；团队需要统一中文交付时，可明确维护团队偏好，而不在翻译时悄悄增加全局硬限制。

## 实施顺序与文件范围

1. **固化基线与翻译正文。** 建立当前及需迁移历史正文的对应表；翻译 16 个角色、Mika、8 个小队；保留所有行为约束和技术标识。先处理代码审查员、一个小队负责人及 Mika 作为代表，再完成其余正文。
2. **同步页面与契约。** 修改中文标签、介绍、补充示例和旧英文限定注释；复用当前保存链路。更新英文标题断言为中文章节与关键规则断言，保留现有权限映射和角色差异验证。
3. **准备并执行目标工作区转换。** 先交付 dry-run 清单与备份，再由受控工具处理已选实例。初次只转换代表角色，验证保存、重读和任务领取正文后再扩大；覆盖复用旧角色的小队情形。
4. **完成回归与交付。** 验证中文往返、跨语言草稿、Mika 边界、历史内容与并发冲突、回滚；产出已转换/已中文/需单独处理/冲突跳过清单。部署与实例更新是两个可分开回退的步骤。

主要代码位置：`server/internal/service/builtin_agent_templates/`、`builtin_agents/mika/`、`builtin_squad_templates/`、`builtin_agents.go`；`packages/views/locales/zh-Hans/agents.json`；相关服务/handler/前端测试。迁移维护工具按现有后端授权、事务和更新通知模式实现，必要的条件更新查询通过 sqlc 生成。无需新增业务表或翻译依赖。

## 验收与验证

- 16 个角色模板、Mika 产品段和 8 个小队正文都可中文阅读；技术标识保留原样，7 个已中文的角色 skill 没有重复改写。
- 模板预览、新建结果、数据库重读和后续任务领取的角色正文一致。
- 编辑中文保存后重开不丢字、不变回英文；切换个人 UI 语言不改正文或未保存草稿。
- Mika 产品段继续只读，补充可以保存，更新系统段不覆盖补充；改名后占位符正常替换。
- 既有自定义正文、无来源正文、已中文正文不会被默认模板静默覆盖；历史版本对应的规则完整保留。
- 批量转换限定工作区，冲突可检测，重复执行无额外变化；回滚不覆盖迁移后的人工修改。
- 既有自主权限、审批、父任务状态、小队复用和 skill 绑定规则通过回归。

具体测试落点：

- `server/internal/service/builtin_agent_templates_test.go:101`、`builtin_agent_autonomy_test.go:176`：中文章节、角色能力映射、小队完成边界和技术标识。
- `server/internal/handler/agent_template_test.go:82`、`squad_template_test.go:246`、`mika_agent_test.go:126`：创建复制、复用保留修改、Mika 拼接。
- `packages/views/agents/components/tabs/instructions-tab.test.tsx`、`create/agent-configuration-panel.test.tsx:28`、`create/use-create-template-agent-submit.test.tsx:124`：真实正文展示与保存、中文往返、系统只读、语言切换、服务端决定模板内容。
- 新增迁移工具测试：默认历史正文、自定义正文、同名非模板、同版本不同正文、跨工作区、并发写入、幂等和条件回滚。

实施后的验证命令以 `package.json`、`Makefile` 为准：定向 Vitest 和 Go 测试先行，然后运行 `pnpm lint`、`pnpm typecheck`、`pnpm test`、`make test`、`pnpm knip`、`git diff --check`，并做 Web/Desktop 的代表流程验证。真实模型的表现需要代表任务验证，不能仅凭静态字符串测试声称中英文效果完全一致；默认测试不调用用户已登录的真实智能体。

## 本轮证据与限制

已逐文件核查 1 + 16 + 8 + 7 份源文件，阅读注册表、创建/编辑/领取链路及现有测试契约；脚本复核前三组正文没有汉字、最后一组每份均有中文正文。已核查工作区 Git 初始无修改。

本机 `make status` 显示该 checkout 的 API 健康响应归属不匹配、Web 未运行，因此本轮没有将运行界面或数据库内容当作已验证证据，也没有修复或重启用户环境。实际工作区实例数量、历史版本分布及定制内容，在数据转换前再做只读盘点。

本轮仅写文档与任务元数据，未运行产品 lint、typecheck 或测试，也未作模型效果验证。不存在“已上线中文指令”的结论。

## 实施记录

用户已明确要求推进本计划。源码在隔离分支 `feat/agent-instructions-zh` 实现，并按文件补丁合入原工作区，保留另一项技能模板开发的改动。

- 25 份正文（Mika + 16 个角色 + 8 个小队）完成中文化；原 inline code 技术值全部保留，行为版本和权限映射未改。
- 中文指令标签、说明和团队补充示例已更新；继续复用原编辑器、保存和状态逻辑。
- agent/squad PUT 新增成对的 `expected_instructions` / `expected_updated_at`，SQL 原子检查原文、完整时间戳及工作区。冲突 409 不写入、不通知；GET/PUT 提供能力 header，时间戳保留微秒精度。
- `scripts/localize_agent_instructions.py` 提供只读计划、显式写入、私有备份/日志、冲突跳过、幂等和有条件回滚。文档为 `scripts/localize-agent-instructions.md`。
- 独立审查发现并修复回滚原备份摘要校验缺口和 404 冲突记录遗漏，均先复现后补回归。

验证证据：

- 全仓 TypeScript 测试：662 个文件、7,771 个用例通过。
- 全仓 lint、TypeScript 检查通过；lint 存在原有警告，无错误。
- Go 完整测试通过，包含受保护的模拟智能体子进程测试；首次发现既有 `TestGitEnv` 假定没有宿主 Git 环境覆盖，清理测试子进程的该类覆盖后重跑通过，未为此修改产品代码。
- 新增条件更新与关联更新回归通过 race 检查；handler/service `go vet` 通过。
- 转换工具 17 项单元测试通过；Python 编译检查及 diff 检查通过。
- Chromium 验证中文角色正文的模板预览、创建流程和请求边界：1 项通过。测试 API 使用隔离数据库，关闭邮件发送服务，未调用真实智能体。
- `pnpm knip` 仍报告 9 个未引用文件、2 个未引用依赖，涉及文件均不在本次改动范围；未扩展本任务清理它们。

已有实例的只读盘点：当前登录凭据可读取“海卫三”和“火星”。前者有 21 个可见活动智能体及 8 个小队，15 条匹配当前英文基线；后者有 8 个智能体及 2 个小队，7 条匹配基线。其余包括 Mika 的补充、手写智能体以及旧版本角色，不能直接覆盖。9 份历史角色译文已经单独准备并按原文摘要核对，保留历史行为。其他本地工作区通过此凭据返回 404，未绕过访问控制写入。

用户随后明确指定“海卫三”。已完成本地后端更新，在确认新接口支持原子更新前提后重新读取完整时间戳生成执行计划，并应用 9 份逐段审阅的历史版本译文。实际转换结果为 16 个内置角色、8 个小队，共 24 条，零冲突；4 个自建智能体原本为空的指令和 Mika 的空白工作区补充保持原样。Mika 的产品系统段已随后端更新为中文。

已逐条回读确认正文与选定译文完全一致，模板版本、自主权限、模型、运行时、访问设置和其他配置不变。再次预演返回 `already_matches: 24`，不会重复写入；24 条成功回执通过备份摘要和条件回滚计划校验。对照确认“火星”的已保存指令及更新时间没有变化。内嵌的 Mika 系统段属于平台级更新，不属于上述工作区行数据转换。

实例原文、译文、写前快照、更新日志和回读报告保存在原工作区的 `.omx/rollouts/agent-instructions-zh-20260912/`，文件权限为 `0600`，不纳入 Git。核心记录为 `naiad-plan-approved.json`、`naiad-applied.jsonl`、`naiad-verification.json`。保留了更新前后端可执行文件，以便需要时回退本地服务。

运行时平台规范仍保留既有语言和行为；真实模型执行效果未评测。此次只更新配置，没有启动智能体任务或发送团队消息。

Trellis 资料：`.trellis/tasks/archive/2026-09/09-12-builtin-agent-instructions-zh/`，实施与实例验证已完成。
