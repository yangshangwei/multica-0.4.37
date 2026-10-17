# 桌面核心页面审计修复结果

已完成原报告 F01–F09 的修复。保留现有界面和包边界，没有新增依赖、修改后端、创建 Trellis 任务或提交代码。

| 问题 | 修复结果 | 验证 |
| --- | --- | --- |
| F01 筛选输入吞键 | 文本编辑与菜单快捷导航隔离；保留 Escape/Tab，保护输入法组合键 | 真实 Base UI 回归；Electron 输入 `abc xyz` |
| F02 智能体菜单焦点透明 | 键盘焦点显示按钮和既有焦点环 | 原生 Tab：focus-visible=true、opacity=1 |
| F03 缺少名称 | 看板操作包含列上下文、成员操作包含姓名；项目抽屉增加标题 | 可访问名称检查；200% 原生缩放抽屉 |
| F04 标签对比度不足 | 按 WCAG 相对亮度选择文字色，保留有效标签底色 | 4,096 色采样回归；双主题实际标签最低 4.71:1 |
| F05 收件箱选择语义 | 原生标题按钮承接焦点，aria-current 标记当前通知；独立保留行内操作 | 80 条虚拟列表移动到第 41 条：真实焦点、当前项和位置一致，仅挂载 30 条 |
| F06 状态只能拖拽排序 | 复用上移/下移控件，保护提交中状态并播报结果；显示归档条目时禁用排序并解释恢复方式 | 真实 QueryClient/mutation；原生键盘排序、归档保护、重复提交和焦点保持 |
| F07 甘特图误报空态 | 首次失败提供重试；刷新失败保留画布；空项目也显示恢复入口 | 冷/热缓存、空项目、403→500、重新挂载、取消旧请求等回归；Electron 故障场景 |
| F08 通知/用量伪默认值 | 通知偏好和运行时用量（含按智能体明细）区分加载、失败、空数据及缓存刷新失败 | Query 回归；通知失败无开关、重试期间按钮禁用、成功后恢复 7 个开关 |
| F09 折叠状态缺失 | 补充展开状态和稳定区域关联；保留描述编辑器，并关闭隐藏时的编辑器浮层 | 折叠/展开回归；Electron 编辑器节点保留 |

## 检查结果

按测试文件去重，**20 个文件、307 项测试通过**：

- 输入、标签与相关选择器：5 文件、36 项。
- 设置与运行时：5 文件、31 项（含真实状态排序缓存回归）。
- 收件箱与项目：4 文件、64 项。
- 共享任务界面、控制器、目录、甘特图、翻译一致性：5 文件、157 项。
- Core 查询：1 文件、19 项。

`views`、`core`、桌面 main/preload 和 renderer 类型检查通过。相关 ESLint 无新增错误或警告；保留项目详情的 2 条既有 Hook 警告、用量图表的 4 条既有依赖警告。`git diff --check` 通过。

Impeccable 对修改的 13 个 UI 实现文件扫描返回 `[]`。桌面取证共 10 组检查通过，保存 14 张修复截图。视觉确认保留原有布局、字号和主题，未增加额外视觉重构。

## 修改范围与复用

- `packages/views/issues/`：筛选输入、看板操作名称、共享甘特图恢复及回归。
- `packages/views/agents/components/agent-row-actions.tsx`：可见焦点。
- `packages/views/labels/label-chip.tsx`：可测的颜色对比度计算。
- `packages/views/inbox/`：语义列表、原生选择按钮和跨虚拟边界的焦点交接。
- `packages/views/projects/`：抽屉名称、折叠状态和描述编辑器可见性。
- `packages/views/settings/`、`packages/views/runtimes/`：成员命名、排序与读取恢复。
- `packages/core/issues/queries.ts`：在所属 Gantt 查询中清除已被拒绝的缓存，并检查 AbortSignal，防止取消的旧请求清除新结果。
- 对应中英文文案与测试；交互规范补充到 `.trellis/spec/views/frontend/desktop-core-accessibility.md`。

复用 Base UI、现有 `ReorderControls`/`adjacentOrder`、React Query、集合错误态和编辑器 `isVisible`。移除了不可靠的归档分段排序判断和无语义的收件箱行按钮模拟，没有建立额外数据层或状态镜像。

## 证据与限制

- [桌面验证结果](remediation-native.json)
- [修复截图总览](screenshots/remediation-contact.jpg)
- [最终视觉确认](remediation-visual-final.json)
- [Impeccable 扫描](remediation-detector.json)
- [修改文件清单](remediation-changed-files.json)
- [修复前报告](report.md)

原生验证使用 macOS Electron 39.8.7、当前开发渲染器和隔离 profile。API 写入保持拦截；收件箱大列表、状态排序使用本地 UI/Query 夹具，没有修改保留业务数据或启动智能体。

本地后端在甘特图最终复核时返回 `503 Service Unavailable`。已验证界面保留内容并显示错误；成功重试使用隔离 ApiClient 夹具验证，因此不将本次结果宣称为后端恢复或服务写入端到端验收。HTTP 冷失败、真实 Query 缓存与组件恢复均有独立证据。工具连接/缩放导航的失败尝试保留在审计目录，最终结果以 `remediation-native.json` 为准。

没有运行 Windows/Linux、VoiceOver/NVDA、真实输入法候选窗、生产构建性能或完整后端测试。现有删除测试的 act 提示和上述既有 lint 警告没有在本次范围内改动。
