---
target: 11-07-collaboration-create-composer plan audit (plan coverage, not rendered UI)
total_score: 27
max_score: 40
na_heuristics:
p0_count: 0
p1_count: 3
target_identity: "file:/Volumes/artisan/code/2026/multica-0.4.37/.trellis/tasks/11-07-collaboration-create-composer/design.md"
target_fingerprint: "sha256:c48fe4917fc954a7f02eb3ad1fc72a8d084f4930b5f99f9df5c487441f70ecdd"
target_path: /Volumes/artisan/code/2026/multica-0.4.37/.trellis/tasks/11-07-collaboration-create-composer/design.md
timestamp: 2026-11-07T12-47-02Z
slug: 7-collaboration-create-composer-design-md-fed16bd8
---
# 新建协作任务 composer 方案审计

日期：2026-11-07。对象：本任务的 `prd.md`、`design.md`、`implement.md`。
参照：`packages/views/modals/create-issue.tsx` 和当前协作创建实现。
模式：Operate；方案审查。设计判断与检测器/浏览器证据由两个隔离子代理分别收集，设计审查完成后才合并检测器结果。

## 结论

**保留 composer-first 方向；补齐布局、身份展示和焦点恢复契约后再实现。**
当前方案正确地选择了已有产品语言，但“紧凑控件”尚不足以规定协作场景的完整交互。
本次发现 0 个 P0、3 个 P1 方案缺口、2 个 P2 方案缺口。它们是实施前需要处理的风险，不能表述成尚未实现的新界面已经发生的故障。

值得保留：紧凑上下文头、大标题与低边框编辑区、语义 token、渐进展开高级字段、仅正文滚动、草稿及原始请求恢复、现有纯文本载荷。无需换编辑器、增加依赖或改动 API。

## 优先问题

### P1-1：固定工具栏必须分离附件入口和附件详情

位置：`design.md:48`、`implement.md:12`；现有 `packages/views/collaboration/attachment-picker.tsx:43`、`:74`、`:80`、`:90`。

设计树把 attachment picker 放入固定工具栏，但目前该组件同时承载最多 32 个文件、勾选状态、上传取消、结果查询、重试和重新选文件。若整个组件随 compact 模式移入固定区，多附件和恢复状态会持续增加固定区高度，挤压小窗口的编辑空间。

补充契约：固定区只放“添加附件”入口及简短状态/数量；文件清单、选择复选框、共享说明、错误和恢复操作放入滚动正文。保持一个上传状态拥有者，不通过重复挂载 picker 获得两个展示位置。正文错误出现时沿用已有一次性 reveal，不夺走当前焦点。

验收：0/1/32 个文件、长文件名、上传中、取消、结果未确认、重新选文件；320×568 和 390×740 下正文仍可编辑、所有文件和恢复动作可到达、footer 不被挤出。

建议：`$impeccable adapt`。

### P1-2：隐藏文件输入必须同步迁移焦点恢复目标

位置：`design.md:18`；`packages/views/collaboration/attachment-picker.tsx:60`；`packages/views/collaboration/focus.ts:15`。

当前 onSettled 会把丢失的焦点恢复到原生 file input。计划将其隐藏后，这个目标不再可靠：display:none 不能获得焦点，仅视觉隐藏又可能留下不可见 Tab 停靠点。

补充契约：compact 模式将焦点恢复到可见“添加附件”按钮；默认模式保留现有输入目标。按钮使用 `type="button"`，支持键盘触发，不触发表单提交。上传取消、恢复成功、重试成功后保证可见焦点；用户已经移到其他有效控件时不抢焦点。

建议：`$impeccable harden`。

### P1-3：接收人 compact 不能省略完整账号身份

位置：`prd.md:39`、`design.md:62`；`packages/views/collaboration/account-picker.tsx:188`；`.trellis/spec/views/frontend/collaboration-creation.md:31`。

协作接收人不能只按普通负责人的“头像 + 姓名”短胶囊处理。当前契约要求选中触发器和候选项保留完整账号 ID、允许换行，并保留单独的“已选择某人”句子。同名或 UUID 前缀相同的账号必须可辨认。

补充契约：compact 减少边框和留白，完整身份仍在选中控件中直接可见；不以截断、短 UUID、仅 tooltip 或仅读屏文本代替。保留允许展示的外部标记，接收人与 AI 控件有稳定的可访问名称。

验收：默认自己、两个同名账号、相同 UUID 前缀、长姓名、外部账号；窄屏换行且不横向溢出。继续保持原有查询、IME、延迟搜索期间禁止旧结果提交和精确查找语义。

建议：`$impeccable clarify`。

### P2-1：协作解释与折叠字段摘要需要明确位置

位置：`design.md:40`、`prd.md:40`；`packages/views/locales/zh-Hans/collaboration.json:551`、`:605`、`:992`。

现有来源/共享说明、AI 参与方式解释、仅共享已选文件的提示都有决策价值。布局树只有控件名称，未说明压缩后这些内容的去处。高级字段遇错展开已经写清，但有效的非默认草稿值仍可能藏在折叠区内。

补充契约：来源与共享关系保留一句简短说明；AI 参与方式说明邻近入口；共享提示邻近文件清单。高级字段折叠时增加“已设置 N 项”或少量非默认值摘要，帮助用户重开草稿后识别保留的选择。这是建议增加的呈现要求，不是当前实现的回归。

创建不可用时保留对应原因：标题为空、上传中、待查询结果、无效初始字段和来源权限问题应可区分。`design.md:81` 所说的 disabled 按钮不能本身触发原生必填气泡；应保留 required/submit guards，同时提供可见且关联的原因，不把 disabled 当成完整的错误提示设计。

建议：`$impeccable clarify`。

### P2-2：视觉统一中的键盘差异必须有明确记录

位置：`prd.md:80`；`packages/views/modals/create-issue.tsx:994`；`packages/views/collaboration/creation-form.test.tsx:484`。

普通任务标题中的 Enter 结束标题编辑，组合键提交；本任务明确保留协作表单的原生 Enter 提交。视觉接近后，用户更容易认为两者键盘行为相同。

本轮保留已规定的行为，但在设计和验收中记录这个例外，不复制尚未实现的快捷键提示。验证标题 Enter、描述换行、接收人候选 Enter、精确查询 Enter、中文 IME Enter；picker 内操作不得意外创建任务。

建议：`$impeccable audit`。

## 三种方案

三种方案都属于单页 composer；不加入向导、桌面左右分栏或新存储。B/C 调整了当前 design 中的控件位置，应在选择后同步设计稿，不能把本审计视为已经改选。

| 方案 | 布局 | 适用与代价 |
| --- | --- | --- |
| A：现方案修正版，底部属性条 | 来源头部；标题/描述/高级字段/附件详情在正文；底部放接收人、AI、附件入口；独立操作 footer | 与普通新建任务最接近，改动较小。完整账号身份必须允许多行，窄屏固定区压力最大。 |
| B：上方协作上下文带 | 来源下方排列接收人完整身份与 AI；正文编辑；底部附件入口及操作 | 最容易先确认“从哪里、交给谁”；顶部较重，必须保持标题为主要编辑目标。 |
| C：正文紧凑协作区，推荐 | 来源头部；正文中依次为标题、描述、轻量接收人/AI 区、附件详情、高级字段；底部附件入口及操作 | 固定区最小，长身份、多附件和恢复状态最容易容纳。长描述时接收人需要滚动查看；短草稿首屏应能发现该区。 |

**建议 C；如果最重视与普通任务完全相同的底部属性位置，采用 A 并补齐三个 P1。**
C 延续“先写任务”的层级，不把接收人与 AI 藏进高级字段。为避免重新变成长表单，接收人/AI 使用紧凑横向组合和轻量标签，在窄屏自然换行，减少大块边框和重复段落标题。

推荐方案示意（结构草图，不是已实现界面）：

```text
来源工作空间 ▾  /  新建协作任务                    ×
─────────────────────────────────────────────────
标题                                               ↑
写下任务目标……                                     │
                                                   │
描述                                               │ 正文滚动
补充背景与交付要求……                               │
                                                   │
接收人 [姓名 · 外部标记 ▾]    [AI 参与方式 ▾]        │
       完整账号 ID（可换行）                        │
已选择某人 / 简短 AI 说明                           │
                                                   │
附件清单 · 勾选 · 共享说明 · 上传恢复                │
更多字段 · 已设置 N 项                              ↓
─────────────────────────────────────────────────
添加附件  草稿已保存在此设备          取消  创建协作任务
```

## 视觉与验收应补入设计/实施计划

- 对齐普通 composer 的上下文层级、`text-title` 标题、`text-body` 正文、`text-caption` 辅助文字和约 20px 的内容边距；复用语义颜色及现有焦点样式。普通 composer 的 42rem 默认最大宽度可作起点，但不要照抄 440px 高度，协作完整身份和恢复内容需要独立验证。
- 明确标题是否随正文滚动。普通任务标题固定、描述滚动；当前协作方案标题也在滚动区，这属于可接受但需要记录的差异。
- `design.md:40` 的 expand affordance 尚无 PRD/状态/验收定义：从本轮移除，或补齐展开/收起尺寸、焦点、触屏目标和减少动画偏好的行为，避免只画一个图标。
- 浏览器矩阵至少包括桌面、390×740、320×568，浅色/深色，中英文，长姓名/完整 UUID/长文件名/长描述，200% 缩放；确认主文档无横向溢出、粗指针目标至少 44px、实际焦点环可见、picker portal 不被正文裁剪。
- 状态矩阵包括来源加载/空/错误、已选来源锁定、无效高级字段自动展开、上传及恢复、已确认拒绝、未确认创建、取消重开和独立 unconstrained panel。保留业务测试；DOM 单测不能代替真实布局矩形和绘制检查。
- 将 `locales/parity.test.ts` 明确加入验证命令；为 compact 文件按钮增加真实键盘焦点路径，保护 default picker 调用方。检测器结果应按目标文件与既有基线核查，不能以整个 collaboration 目录零告警替代本任务的验收。

## 计划覆盖度（非渲染质量评分）

| Nielsen 启发式 | /4 | 主要依据 |
| --- | ---: | --- |
| 系统状态可见 | 3 | 保留 pending/error/recovery；呈现细节待补 |
| 与用户语言匹配 | 3 | 协作术语明确；解释位置待补 |
| 用户控制与自由 | 3 | 取消、关闭和草稿保留 |
| 一致性与标准 | 3 | 视觉参考明确；Enter 例外未写清 |
| 错误预防 | 3 | 来源锁定、字段验证、草稿恢复 |
| 识别优于记忆 | 2 | 完整身份和折叠摘要未落图 |
| 灵活性与效率 | 2 | 缺少完整键盘矩阵 |
| 美观与简洁 | 3 | 编辑优先；固定区容量未定义 |
| 错误识别与恢复 | 3 | 业务恢复充分；隐藏输入焦点遗漏 |
| 帮助与文档 | 2 | 原有情境帮助去向不明 |
| 合计 | **27/40** | 可接受；实施前补齐关键契约 |

首次使用者需要看懂来源、接收人和 AI 的关系；键盘用户需要上传/恢复后的可见焦点；小屏及易中断的用户需要足够正文空间和重开草稿后的选择摘要。认知路径应保持“确定来源 → 写任务 → 确认协作设置 → 创建”。高级字段包含多种选择，应继续渐进展开。

## 检测器与浏览器证据

- 使用已安装 skill 入口扫描 `creation-form.tsx`、`account-picker.tsx`、`attachment-picker.tsx`，退出码 0，JSON 为 `[]`，无告警。它是当前源码的机械检查，不是对拟议布局、对比度、焦点和响应式的通过证明。
- 计划中的 `pnpm exec impeccable` 当前不可用：尝试 `pnpm exec impeccable --help` 返回退出码 254、`Command "impeccable" not found`。本机可用入口为 `/Users/artisan/.agents/skills/impeccable/scripts/impeccable`。应修正验证命令，不为此添加项目依赖。
- 对正在运行的 Electron Vite renderer（localhost:5666）创建了独立浏览器标签。HTTP 200，但脱离 Electron preload 后页面进入错误边界，提示读取 `appInfo` 失败。没有打开实际新建界面；这是检查环境限制，不应记作协作页面缺陷。没有注入模拟 bridge，没有提交任务，也没有启动整套服务。
- 回退查看受版本控制的 `.trellis/tasks/archive/2026-11/11-02-collaboration-creation-audit-fixes/research/browser/` 中 390×740、320×568 的浅色/深色截图。旧界面确有较高的说明/来源区和带边框表单，320px 初始视口正文空间紧张。这支持降低表单纵向层级的方向。
- 历史记录来自 2026-11-02 共享组件生产构建、Chromium、模拟 API 与内存 StorageAdapter。当前 `creation-form.tsx` 和 `account-picker.tsx` 的哈希已与该记录不同，`tokens.css` 仍相同，因此这些图片不是当前源码的响应式验收，更不是新方案的验收。普通新建任务本轮仅做源码对照。
- 未生成可见 detector overlay；未启动 impeccable live-server。独立浏览器标签已关闭，结束时未发现残留 gstack browse 进程。未运行产品测试、lint、typecheck、真实创建请求、实机触控或读屏朗读。

建议修正后的命令（本轮未运行其中的测试/类型/lint）：

```sh
pnpm --filter @multica/views exec vitest run collaboration/creation-form.test.tsx collaboration/account-picker.test.tsx locales/parity.test.ts
pnpm --filter @multica/views typecheck
pnpm --filter @multica/views lint
/Users/artisan/.agents/skills/impeccable/scripts/impeccable detect --json packages/views/collaboration/creation-form.tsx packages/views/collaboration/account-picker.tsx packages/views/collaboration/attachment-picker.tsx
git diff --check
```

检测器路径应在其他开发机按实际安装位置解析。该命令列表仍需配合上面的真实浏览器矩阵；新增/变更的 compact 焦点测试也必须纳入执行范围。

## 建议顺序与范围

先通过 `$impeccable adapt` 明确固定区/滚动区边界，再用 `$impeccable harden` 补焦点和恢复状态，随后用 `$impeccable clarify` 完成身份、说明与摘要。实施后做 `$impeccable audit` 和最终 `$impeccable polish`。

本轮仅新增审计记录，未修改原 PRD/design/implement、产品文件或任务状态，未运行产品测试。建议的简化是减少固定区内容、复用现有控件与状态拥有者。剩余风险是新布局尚未实现，不能据此宣称新 UI 的可访问性、性能或响应式已经通过。

Questions skipped: 本轮请求为已有方案审计和方案比较；评审不依赖新的用户信息，未进入实现或替用户改选布局。
