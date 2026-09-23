# 重构技能库生命周期分类与多维标签

## Goal

让技能库围绕软件研发与软件交付的完整工作体系组织：每个 skill 选择一个稳定的主分类，工作区标签补充技术领域、平台、工作方式、交付物和风险等横向属性。用户应能浏览、筛选和批量整理大量 skill，而不把来源、分类和标签混成同一维度。

## Background

- 当前固定分类为 `research`、`writing`、`engineering`、`operations`、`data`、`other`，存储在 `skill.config.presentation.category`（`packages/core/skills/presentation.ts:15-26`）。
- 当前工作区标签已经支持 `resource_type = "skill"`、多对多挂接、搜索和筛选（`packages/core/types/label.ts:8-34`、`packages/views/skills/components/skills-page.tsx:827-874`）。
- 来源已经是独立筛选维度，不能再承担能力或生命周期语义（`packages/core/skills/stores/view-store.ts:42-67`）。
- 已安装 Desktop 可能连接较新的 backend，旧分类 key 不能删除或改名（`CLAUDE.md:77-79`）。

## Product Model

### 主分类：单选

主分类回答：“这个 skill 在软件研发与交付体系中的主要归属是什么？”第一组覆盖交付主链路，第二组收纳贯穿全流程的支撑能力。

| 稳定 key | 中文显示名 | 英文显示名 | 变更 |
| --- | --- | --- | --- |
| `research` | 需求与规划 | Planning & requirements | 保留 key，更新显示名 |
| `design` | 设计与架构 | Design & architecture | 新增 |
| `engineering` | 开发与集成 | Development & integration | 保留 key，更新显示名 |
| `quality` | 测试与质量 | Testing & quality | 新增 |
| `operations` | 发布与运维 | Release & operations | 保留 key，更新显示名 |
| `writing` | 协作与知识 | Collaboration & knowledge | 保留 key，更新显示名 |
| `data` | 数据与自动化 | Data & automation | 保留 key，更新显示名 |
| `other` | 通用工具 | General tools | 保留 key，更新显示名 |

固定顺序为上表顺序。每个 skill 仍然只能有一个主分类；跨阶段或次要属性使用标签表达。

### 标签：多选

标签回答：“它适用于什么领域、平台、工作方式、交付物或风险场景？”继续复用工作区 skill 标签，不新增自由文本数组，不把标签写进 `config.presentation`。

推荐词表采用可读前缀，但第一阶段不解析前缀、不自动创建标签，也不增加数据库字段：

- 领域：`领域/产品`、`领域/前端`、`领域/后端`、`领域/数据`、`领域/AI`、`领域/安全`、`领域/基础设施`、`领域/文档`
- 平台：`平台/Web`、`平台/Desktop`、`平台/Mobile`、`平台/Server`、`平台/CLI`、`平台/Cloud`
- 方式：`方式/分析`、`方式/生成`、`方式/审查`、`方式/自动化`、`方式/报告`
- 产物：`产物/PRD`、`产物/ADR`、`产物/API`、`产物/代码`、`产物/测试`、`产物/发布包`、`产物/运行手册`
- 风险：`风险/生产操作`、`风险/安全敏感`、`风险/破坏性`

标签只用于组织和发现，不能授予权限，也不能代替执行审批或安全边界。

## Requirements

### R1 分类契约

- `SKILL_CATEGORIES` 按上述八项排序；保留全部六个旧 key，仅增加 `design` 和 `quality`。
- 缺失、非法或新客户端暂不认识的分类继续容错为 `other`，列表不能因配置漂移而崩溃。
- 默认图标调整为：`research=list-checks`、`design=landmark`、`engineering=code`、`quality=clipboard-check`、`operations=rocket`、`writing=book-open-text`、`data=database`、`other=wrench`。
- `design` 与 `quality` 增加独立的亮色/暗色语义色 token；所有分类继续由统一映射控制图标和颜色。

### R2 内置 role skill 归类

- 需求澄清 → `research`
- 架构决策记录 → `design`
- 代码审查、安全审查、测试报告 → `quality`
- 发布检查 → `operations`
- 文档更新、进展报告 → `writing`
- 只更新模板的默认 metadata；已物化且可能被用户编辑的工作区 skill 不自动改写。

### R3 标签工作流

- 保留现有创建页、详情页、列表/卡片展示、搜索和标签筛选能力。
- 在批量工具栏增加“管理标签”：针对可编辑的已选 skill，对一个现有 skill 标签执行批量添加或批量移除。
- 混合选中状态必须明确：全部拥有、部分拥有、全部没有三种状态可区分；点击后执行确定性的“全部添加”或“全部移除”。
- 第一阶段复用现有单资源标签 API，并采用和批量设置分类一致的逐项失败统计；不新增 batch endpoint。
- 文档说明推荐标签维度和命名方式；系统不自动创建标签，避免污染已有工作区词表。

### R4 浏览和编辑体验

- 左侧分类栏、窄屏 chips、筛选菜单、分类列、排序和分类空状态全部展示新名称与八项顺序。
- 新建和详情编辑器可选择八个分类，批量设置分类也可选择八项。
- 分类栏仍只展示一个主分类；来源继续在独立分组，标签继续在筛选菜单中。
- 八个分类在常见 Web/Desktop 宽度下不得出现文本截断、计数覆盖或无意义换行；窄容器保持水平滚动。

### R5 兼容性与数据策略

- 不新增数据库列，不迁移存量 `config.presentation.category`。
- 旧六类 skill 自动获得新的显示名称；存量 skill 不做内容猜测或自动重分类。
- 旧客户端读取 `design`/`quality` 时可能按既有容错显示为 `other`；普通内容保存不会携带 `config`，只有显式修改展示属性才可能覆盖分类（`packages/views/skills/components/skill-detail-page.tsx:1044-1060`）。
- 新服务端继续接受六个旧 key；不得通过删除旧 key 强迫客户端升级。
- 新客户端连接不支持新 key 的旧服务端时，展示服务端返回的校验错误，不静默降级或改写为其他分类。

## Out Of Scope

- 工作区自定义主分类或调整分类顺序。
- 给标签新增 `dimension`/`group` 数据库字段，或根据标签名前缀实现程序逻辑。
- 自动分析 skill 内容并归类、自动迁移用户已有 skill、自动创建推荐标签。
- 修改 `apps/mobile/`。
- 通过标签改变权限、审批或执行行为。

## Acceptance Criteria

- [ ] Core 与 Go 端分类枚举都包含八项且顺序一致，parity 测试通过。
- [ ] 六个旧 key 的配置无需迁移即可解析；缺失、非法分类仍回退到 `other`。
- [ ] `design` 与 `quality` 可创建、更新、导入并原样落库；非法值仍返回 400。
- [ ] 八个默认图标全部在白名单中，八个 tone 映射和亮/暗 token 完整。
- [ ] zh-Hans / en / ja / ko 四语显示名完整，locale parity 测试通过。
- [ ] 左侧栏、chips、筛选菜单、分类列、排序、新建、详情和批量分类操作都使用同一份八项枚举与顺序。
- [ ] 八个内置 role skill 的默认分类符合 R2，现有用户副本不被后台自动改写。
- [ ] 用户可对多个可编辑 skill 批量添加或移除现有标签；部分成功时报告成功、失败和跳过数量。
- [ ] 分类、来源和标签筛选继续按维度求交；标签搜索行为不回退。
- [ ] `apps/docs/content/docs/skills.mdx` 与 `skills.zh.mdx` 说明主分类和推荐标签词表，并明确标签不授予权限。
- [ ] Core、views、server 的定向测试通过；`pnpm typecheck`、`pnpm lint`、`pnpm test`、`make test` 通过。
- [ ] Web/Desktop 的宽屏、窄屏和暗色模式截图验证无溢出、重叠或不可辨识选中态。
