# 迭代页面修复后审计

本轮复审为 **19/20**，原审计的 19 项问题均已处理，随后发现的“禁用迭代”危险样式遗漏也已补齐。实现一致性通过；没有发现剩余的 P0/P1/P2/P3 实现问题。响应式维度保留一分，因为本轮验证使用 Chromium 触屏仿真，尚未在物理触屏设备上验证。提交前在独立工作树中重新构建并验证了实际提交候选。

| 维度 | 分数 | 证据 |
| --- | --- | --- |
| 可访问性 | 4/4 | 实际文字对比度、可见搜索与模式标签、面板键盘焦点、唯一主地标、可区分的恢复操作 |
| 性能 | 4/4 | 打开筛选新增任务分页请求为 0；筛选元数据覆盖完整历史范围；分组保留完整读取 |
| 主题 | 4/4 | warning/info/destructive 文字与填充分别使用 token；浅深主题实测通过 |
| 响应式 | 3/4 | 680px 窄屏与粗指针仿真通过，按钮/选择器 44px，长标题无面板溢出；物理设备未验证 |
| 实现一致性 | 4/4 | Select/Textarea/Checkbox/Skeleton 复用组件库；折叠样式一致；确认摘要与任务事实分区 |
| 合计 | **19/20** | 原审计 13/20 |

## 实测结果

| 检查 | 浅色 | 深色 |
| --- | --- | --- |
| 逾期提示文字 | 5.84:1 | 9.32:1 |
| 当前徽章文字（包含实际底色） | 5.58:1 | 7.33:1 |
| 取消/删除确认文字 | 5.79:1 | 5.55:1 |
| 禁用所有迭代确认文字 | 5.79:1 | 5.55:1 |

所有这些文字均超过 WCAG AA 的 4.5:1。危险按钮原先把填充 token 当文字使用，在实际粉色底上只有 3.98:1；新增独立文字色后通过。颜色在有限主题过渡完成后测量，避免把动画中间帧当作最终主题颜色。

最终视觉复核为 **95/100（pass）**。深色日期输入的原生控件实测 color-scheme 为 dark；曾出现的黑色图标来自测试手动切换类名而未同步主题提供器，已先用失败断言复现，再修正测试并重新验证。

提交前新增浅色/深色与宽屏/680px 粗指针窄屏的四张禁用预览截图。确认按钮的实际文字颜色与 `--destructive-foreground` 一致，两种主题均超过 4.5:1，窄屏点击高度为 44px。弹窗和文档均无横向溢出；关闭预览后设置仍保持启用，未执行禁用。独立视觉复核检查了这四张截图以及取消预览、总览和中文深色创建表单。

Tab 面板实测显示 2px 实线焦点框，向内偏移 2px。详情实际 DOM 只有一个 main：Web 由壳层提供，Desktop 迭代路由显式提供外层地标。打开筛选与打开状态选项没有新增 iteration-issues 请求。触屏仿真报告 pointer:coarse、5 个触点、680px 宽；详情按钮和筛选选择器均达到 44px。历史长标题按钮允许换行，实测高度 102px；实时任务标题也使用明确的 overflow-wrap，并通过面板自身的溢出断言。

## 19 项问题的处理

| 原问题 | 修复与检查 |
| --- | --- |
| 1 逾期文字 | warning-foreground；实际浅深色对比度通过 |
| 2 当前徽章 | info-foreground；实际着色背景对比度通过 |
| 3 Tab 焦点 | 共享 TabsContent 焦点描边；真实键盘 Tab 到面板通过 |
| 4 搜索标签 | 搜索任务可见标签、同名可访问名称与提示文字 |
| 5 原生控件 | 迭代选择、筛选、模式、协调人、描述与勾选项复用现有基础组件 |
| 6 确认结构与危险操作 | 变更摘要、完整任务事实、去向和计数；操作专属确认文案；取消、删除、禁用统一使用危险样式及可读文字色 |
| 7 历史长标题 | 可变高度、明确换行、字段化对比；窄屏无面板溢出 |
| 8 筛选全量请求 | 原有 issues 响应返回完整历史 filter_options；移除为选项而触发的全量分组读取 |
| 9 触屏目标 | 一致粗指针 44px 规则，覆盖按钮、选择器、弹出选项、图标关闭、链接与折叠摘要 |
| 10 透明灰 | 装饰标记使用 faint-foreground，正文保留可读文字色 |
| 11 主次操作 | 创建/开始/确认优先，重试和分页次要 |
| 12 必填与禁用原因 | 原因必填说明、模式标签、未填原因/未选移交目标/已有当前迭代的说明 |
| 13 恢复名称 | 操作与对象明确；缓存缺失且原因相同时保留原对象 ID；恢复仍使用原请求 |
| 14 main 嵌套 | 共享页不重复提供地标；仅 Desktop 迭代路由显式提供外层 main |
| 15 标题层级 | 范围变化日期分组使用 h2 |
| 16 静态状态播报 | 总览不变化的提示移除不必要的 status |
| 17 分页与文案 | 总览加载更多；任务/项目分页可返回上一页，筛选或身份变化重置游标 |
| 18 加载 | 页、列表、活动、选择、设置与历史比较使用有形状的 Skeleton |
| 19 边角完成度 | 复用折叠样式；历史比较按字段显示；原始事件 JSON 放入次级原始记录披露 |

历史 ID 存在而名称缺失时显示“未记录”，不误报“无”。保存的时区、完整预览、基线版本、冻结事实、权限边界、访问撤销与原请求恢复均保留。

## 验证

- 迭代组件与语言一致性：15 个文件，224 个测试通过。
- 核心迭代与 API：13 个文件，167 个测试通过。
- 创建任务与弹窗的迭代上下文：2 个文件，66 个测试通过；以上共 457 个单元与组件测试。
- 工作区 typecheck：9/9 项通过；core/ui/views lint 通过，原有范围外警告保留。
- Go 历史读取/筛选测试与 go vet 通过，使用独立测试数据库。
- 4 项实际生产 Desktop E2E 通过，耗时 43.4 秒：审计交互与禁用危险样式、丢失响应恢复与冻结历史、设置正常切换、原子移交且保留下一迭代已有任务。
- UI 导出检查 62 个文件通过，diff whitespace 通过。
- 首次验收的 Impeccable detect 返回 0；提交前复核采用实际 DOM、颜色、请求与截图证据，不将静态扫描作为视觉通过的依据。

提交前保存 19 张最新截图，覆盖中英文、浅深主题、总览、任务、进度、范围变化、创建、结束、取消、删除、禁用、历史比较和设置。独立临时构建与 API 没有替换已有运行环境。永久 E2E 用 TestInfo 输出目录保存证据，不再把固定日期任务目录写入测试逻辑。

## 证据和变更范围

- [实际渲染数据](../../.trellis/tasks/archive/2026-10/10-09-iterations-audit-remediation/browser-evidence/closeout/rendered-metrics.json)
- [全部检查与提交候选指纹](../../.trellis/tasks/archive/2026-10/10-09-iterations-audit-remediation/verification.json)
- [实际提交补丁](../../.trellis/tasks/archive/2026-10/10-09-iterations-audit-remediation/committed-changes.patch)
- [提交文件清单](../../.trellis/tasks/archive/2026-10/10-09-iterations-audit-remediation/changed-files.json)
- [提交范围与排除项](../../.trellis/tasks/archive/2026-10/10-09-iterations-audit-remediation/research/commit-scope.json)
- [浅色禁用预览](../../.trellis/tasks/archive/2026-10/10-09-iterations-audit-remediation/browser-evidence/closeout/disable-preview-light-wide.png)
- [深色窄屏禁用预览](../../.trellis/tasks/archive/2026-10/10-09-iterations-audit-remediation/browser-evidence/closeout/disable-preview-dark-coarse-narrow.png)
- [键盘焦点截图](../../.trellis/tasks/archive/2026-10/10-09-iterations-audit-remediation/browser-evidence/closeout/keyboard-panel-focus.png)
- [中文深色创建表单](../../.trellis/tasks/archive/2026-10/10-09-iterations-audit-remediation/browser-evidence/closeout/create-form-zh-dark-narrow.png)

修改集中在共享 iterations、UI token/控件、历史筛选 API、双语文案和 Desktop 迭代路由。提交同时包含这些页面所必需的既有进度、范围活动和创建任务上下文实现。没有新增依赖或数据库迁移。

## 验证边界与提交状态

验证使用实际 production Electron renderer；没有执行独立 production Next.js E2E，也没有物理触屏或完整读屏遍历。Web 使用相同共享组件，平台 wiring 已经通过类型及回归检查；不把这些检查称为完整 Web 浏览器验收。

用户已授权复核通过后提交并归档。最终候选共 66 个文件，在基于 `7e1eb8058` 的独立工作树中完成上述验证。混合文件按改动片段选择：颜色文件仅提交状态文字 token，项目语言文件仅提交迭代内容；保留甘特图、桌面核心交互、发布、离线升级和迭代 rollout 移除等其他工作。最终提交信息与归档状态见任务记录。
