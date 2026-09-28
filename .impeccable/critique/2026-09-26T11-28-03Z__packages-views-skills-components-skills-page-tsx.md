---
target: 技能库页面，尤其内置 skill
total_score: 15
max_score: 28
na_heuristics: 3,5,9
p0_count: 0
p1_count: 1
timestamp: 2026-09-26T11-28-03Z
slug: packages-views-skills-components-skills-page-tsx
---
Method: dual-agent (A: /root/skills_design_review · B: /root/skills_evidence)

技能库界面评审：重点是内置模板

依据为用户截图、当前源码和一次静态检测。截图中的内置目录已展开；当前源码默认收起。未操作真实页面。

设计判断

现有深色界面、分类色、智能体使用情况和管理工具适合工作型产品，业务特征明确。主要问题是模板目录和工作区实例同时占据主页面，名称与说明重复，用户需要自行理解两者关系。应先改善入口与语义，再处理卡片密度。

值得保留

- 分类图标和使用智能体数量有业务意义，帮助识别用途和使用情况。
- 搜索、筛选、排序、列表/卡片与批量操作已经具备，适合日常管理。
- 现有模板创建界面已有搜索、分组、正文预览和独立副本流程，可以复用。

优先问题

1. P1：内置模板与工作区 skill 身份不清。
   同屏 15 个模板与 14 个工作区 skill 的计数口径不同；相同名称和描述再次出现，“打开 skill”未显示目标名称或状态。builtin-skill-catalog.tsx:39 使用 find 匹配第一个工作区实例，多个副本会被压成一个不明确的目的地。
   建议：明确使用“内置模板”和“工作区 skill”；显示“工作区已有”及具体名称，有多个时显示数量并提供选择；按真实权限显示编辑能力。模板预览说明只读，创建副本后独立维护。
   位置：packages/views/skills/components/builtin-skill-catalog.tsx:39；packages/views/locales/zh-Hans/skills.json:506。
   后续方向：impeccable clarify。

2. P2：展开模板目录挤占工作空间，查找范围不易理解。
   目录位于工具栏之前，独立限高滚动；页面搜索和分类只筛工作区实例。这是当前刻意设计的范围，不是已证实的过滤错误。展开后，用户却容易把工具栏理解为整页搜索。
   建议：用一行“内置模板 · 15 / 浏览模板”替代展开的大目录。点击复用已有搜索与预览界面，让首页集中管理工作区内容。空工作区可更突出从模板开始的入口。
   位置：packages/views/common/builtin-template-catalog.tsx:78；packages/views/skills/components/skills-page.tsx:1009；packages/views/skills/components/template-skill-create-panel.tsx:123。
   后续方向：impeccable distill、impeccable layout。

3. P2：模板说明更像执行指令，缺少便于挑选的简短用途。
   长说明混合适用场景、执行步骤、限制和专业术语。多种质量类 skill 很难快速比较。
   建议：目录用“什么时候用 + 得到什么”的人类阅读摘要，例如“根因分析：定位异常原因，给出证据和修复方向”。完整规则保留在预览；不要为了展示缩短与服务端约定一致的执行描述。
   位置：packages/views/common/builtin-template-catalog.tsx:101；packages/views/locales/zh-Hans/skills.json:25。
   后续方向：impeccable clarify。

4. P2：工作区卡片密度与标题可读性失衡。
   截图一行八张卡，长标题被截断，同时卡片留有空标签空间。当前固定高度 172px，最小宽度 240px，标题单行截断；底部来源铅笔可能被误解为编辑操作。
   建议：优先保障标题完整或允许两行；无标签时减少无效占位；来源说明使用明确文字或辅助提示。保持分类色和智能体使用情况。
   位置：packages/views/skills/components/skill-card.tsx:22、70、108、152；packages/views/skills/components/skill-card-grid.tsx:12。
   后续方向：impeccable layout、impeccable polish。

建议布局

Skills · 14                                      + 新建 skill
内置模板 · 15     从模板创建独立副本                浏览模板 →
搜索工作区 skill...                     筛选 / 排序 / 视图
分类侧栏                     工作区 skill 卡片或列表

模板浏览中：左侧搜索与模板列表，右侧完整预览；显示工作区已有情况，提供“创建副本”和目标明确的现有 skill 链接。无需新增一套并行的模板浏览流程。

认知与用户体验

展开状态下，单一焦点、分组、主次层级、重复决策四项有改进空间。不是条目超过某个数字就有问题，而是模板与实例的重复展示要求用户自行建立关系。首次用户会疑惑是否已添加、能否编辑；频繁用户需要更直接地搜索和管理工作区内容；辅助技术用户需要可键盘触达的主打开入口。

启发式静态评价（不是完整交互评分）

| 项目 | 分数 | 说明 |
|---|---:|---|
| 系统状态可见 | 2/4 | 计数存在，但模板/工作区状态不清 |
| 用户语言 | 2/4 | 名称易懂，说明和动作有歧义 |
| 用户控制与退出 | 未观察 | 未实际操作 |
| 一致性 | 2/4 | 视觉一致，对象语义不够一致 |
| 错误预防 | 未观察 | 未提交或触发冲突 |
| 识别优于记忆 | 2/4 | 图标有帮助，关联仍靠推断 |
| 灵活性与效率 | 3/4 | 多种管理工具齐全，模板入口偏重 |
| 简洁与美观 | 2/4 | 展开后重复与密度失衡 |
| 错误恢复 | 未观察 | 未触发失败 |
| 帮助与文档 | 2/4 | 有说明和预览，入口缺关键关系 |
| 已评分合计 | 15/28 | 仅限截图和源码支持的部分 |

静态检测与额外观察

Impeccable detector 对 builtin-skill-catalog.tsx、skills-page.tsx、skill-card.tsx 检查一次，退出码 0，输出 []。主项和建议项均 0；无误报。这不能验证交互或信息架构。

人工源码检查补充：skill-card.tsx:53 的主打开区域为 clickable div，use-row-link.ts:72 返回鼠标事件，不提供键盘激活；键盘用户仍可间接使用菜单“在新标签页打开”。需补足主入口的链接/键盘语义。重复的模板按钮可访问名称未包含模板名，独立跳转按钮时上下文偏弱。skill-list-toolbar.tsx:193 在窄视口隐藏搜索；未做移动端实测。目录本身已有语义结构、焦点环、加载/错误状态和减少动画支持，不应误报为缺少这些基础能力。

来源回退为“手动创建”会掩盖模板来源；建议基于可靠 template_source 信息显示出处。平台内置与部署自定义模板应保持区分，当前目录会把全部模板叫作“内置 skill”。这些是次要清晰度问题，不宜先扩大改造范围。

Questions skipped: 用户明确关注内置区，入口、对象关系、摘要三个优先点已足够确定；本次给出评审建议，不改变产品行为。
