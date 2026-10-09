# 桌面核心页面审计修复 · 2026-10-09

本次落实原审计的 10 组问题，并保留现有视觉体系。任务先创建 PRD、设计和实施计划，再进入开发；起始工作副本已有候选修复，本轮逐项核实并补齐焦点、编辑器弹出层、窄窗选择、动态减少动效和批量工具条的剩余缺口。

原报告 [桌面核心页面 impeccable 审计](./2026-10-09-desktop-core-impeccable.md) 的 **13/20** 是审计时的记录，保持不变。本报告记录修复和定向验收，不以视觉截图分数替代新的完整 20 分审计。

## 修复对应关系

| 审计问题 | 当前行为 | 主要实现与验证 |
|---|---|---|
| P1-01 关闭聊天仍可聚焦 | 隐藏窗口 inert、退出无障碍树；关闭菜单与编辑器链接卡；保留编辑器、草稿、上传；仅在聊天拥有焦点时归还入口 | chat-window / use-chat-input-focus / chat-input / chat-queue；ContentEditor / link-hover-card；原生焦点红绿记录和 13 项 ownership 回归 |
| P1-02 500 显示为空 | 收件箱、归档、项目列表/详情区分冷失败、缓存刷新失败、真实空数据、404 与权限丢失；失败可重试，刷新保留选择和编辑内容 | inbox-page / projects-page / project-detail；真实 QueryClient/API 测试、Electron 冷/缓存 500 重试 |
| P1-03 批量选择无名称/焦点 | 使用有名称、可见焦点的共享 Checkbox；范围名称描述实际列表；450px 有选择列和可用批量动作 | list-row / list-view / agents-page / AgentBatchToolbar / projects-page；Tab、Space、部分选中、筛选范围和窄窗截图 |
| P1-04 项目标题无键盘入口 | 标题 AppLink 可用 Tab/Enter；保留整行指针、修改键和菜单，内嵌控件不重复导航 | projects-page / rowLinkInteractiveProps；标题导航与点击隔离回归、真实键盘导航 |
| P1-05 甘特文字对比不足 | 状态类别使用独立语义前景与既有状态填色；两主题七类普通文字最低 **4.556:1** | gantt-view / tokens.css；真实 Electron Canvas RGB/透明度合成测量 |
| P1-06 标签仅能拖动重排 | Alt+Shift+方向键和左右移动菜单；遵守固定/工作空间边界，保留焦点和当前目的地 | desktop tab-bar；菜单/键盘顺序、焦点、边界测试和原生交互 |
| P2-07 分页与聊天入口重叠 | 分页使用共享 clearance、可换行；中英文 900px 与原生 200% 启用分页无重叠，可进入 offset 50 | triage-page；真实按钮点击、矩形和原生截图 |
| P2-08 标签语义不一致 | 桌面表达 aria-current；智能体/技能使用标准 Tabs 漫游焦点、手动激活和 panel 关联，保留草稿/路由 | agent-overview-pane / skill-detail-page / file-tree；方向键、Home/End、Enter/Space 和编辑器回归 |
| P2-09 忽略减少动态效果 | 启动和运行时偏好都生效；动画中切为 reduce 会终止旧弹簧并立即使用最终几何 | desktop-layout；MotionValue.jump，实际动画帧和启动/运行时偏好测试 |
| P2-10 甘特 DOM 随跨度增长 | 日期按视窗绘制，任务行虚拟化；完整跨度保留，三种缩放能到末端，键盘能跨窗口 | gantt-view；1000 行、2020–2030 年跨度：最多 **22 行 / 772 DOM 元素**，Home/End 通过 |

## 本轮补齐的边界

原生 Chromium 在移除已聚焦的聊天头部按钮时，可能先发 focusout，随后 React 才给父节点加 inert。仅检查事件时的 inert 会误清关闭焦点归还记录。本轮用同次提交的断开目标记录解决该顺序，并保留主动 blur、焦点转到页面及较早 header blur 的“不抢焦点”行为。原生失败和成功证据均保留。

仅让聊天根节点 inert 不能关闭挂在 document.body 的编辑器链接悬浮卡。ContentEditor 的可见性输入默认 true，浮窗传实际可见条件；隐藏时清理浮层与 hover 状态，编辑器实例、文档和上传继续保留。

450px 下选择列恢复后，旧批量工具条的 absolute 居中 auto 宽度会把计数挤成多行并遮住筛选/表头。窄窗现在使用正常流 footer，预留聊天入口位置；数量不换字，动作可换行，列表保留至少表头、分组和一行的内部滚动空间。宽桌面仍使用原有居中浮层。

## 验证与证据边界

- 集成视图测试 **20 文件 / 418 项**；桌面外壳 **4 文件 / 111 项**，合计 **529 项通过**。
- 定向 lint 无错误；项目详情保留 2 个既有 react-hooks/exhaustive-deps 警告。检测器首次完整范围无发现；新增工具条另做定向确认。
- 独立代码审查覆盖数据恢复、权限缓存、导航、Tabs、虚拟化、焦点归还；焦点 13 项用例另外在 StrictMode 下通过。
- 真实 Electron 39.8.7 使用独立 profile、真实 preload/renderer 与本地只读 API；本轮没有发送消息、运行智能体或提交批量业务操作。
- 冷/缓存 500 使用真实请求故障注入。甘特极端数据和启用分页 total 使用独立渲染器 Query 缓存夹具，证明布局和交互，不宣称覆盖 API 解析、生产性能或实际批量吞吐。
- 智能体页面的 active/archived scope 互斥，混合归档/活跃工具条合法输入由真实 DOM 回归覆盖；没有假造原生混合业务操作。
- 截图和矩形覆盖中英文、浅深色、1440px、900px及原生 200% 有效 450px；原始 32 张审计截图保持不变。修复证据在对应 Trellis 任务的 evidence/。

最终类型检查、视觉确认分数和提交/归档信息由任务 verification.json 记录。首次视觉检查为 86/revise，发现的唯一工具条问题已经修复并通过原生几何与键盘确认，最终独立截图结论在 visual-verdict.json。

未运行生产构建、生产性能基准、VoiceOver/NVDA 全流程或 Windows/Linux 实机验收。右侧详情栏内部动效属于原审计左侧栏证据之外的相邻问题，未扩大本任务。未新增依赖、API 或数据库变更。
