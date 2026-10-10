# 最终浏览器失败定位与修复

2026-10-10 首轮完整浏览器执行：265 expected、4 unexpected、50 skipped、0 flaky，配置为单 worker／零重试。静态检查、非移动端 10,649 项测试、70 个 Go 包 race/vet、Electron 和生产 Web 构建此前均已通过。首轮 JSON 保留在 `.omx/state/iteration-settings-final-20261010/playwright-initial.json`，截图与 trace 保留在验证 worktree 的 `test-results-initial/`。

以下修正仅涉及测试同步与证据采集，应用源码仍为 `cfa0254eb` 加设置权限恢复补丁。完整复跑结果由父 verification.md 统一记录。

| 用例／文件 | 证据与根因 | 最小修正 | 独立复验 |
| --- | --- | --- | --- |
| agent-category-grouping | 1440→390 的首帧中 document 宽 1416、body 宽 390；唯一越界元素是退出中的分类菜单，约 8.9ms 后宽度恢复，后续 40 帧均不溢出。原用例 3 次中 1 次失败。 | Escape 后等待菜单 portal 移除，再调整视口；原严格无溢出表达式保留。 | 修后 3/3，通过且零重试。 |
| projects-p1-lifecycle，P1-L12 | 事件记录显示 Escape 在弹层尚未打开时发送；之后异步弹层打开，下一次点击触发器把它关闭，选项定位器命中退出中的旧列表。原用例 3 次中 1 次失败。 | 先等待 listbox 显示再断言非法选项不存在；Escape 后等待隐藏。422、403 与真实写入次数检查保留。 | L11／L12／L13 各 3 次，共 9/9，零重试。 |
| iterations-audit-desktop | 完整执行中 popup 拦截 Progress 点击；原用例独立 3/3 通过，属于时序敏感失败。诊断还记录 Playwright viewport=null，截图使 DPR 1→2，证实 raw CDP 与截图状态不同步；不能断言 DPR 是 popup 重开的唯一原因。 | 等待 Select 焦点、关闭与焦点返回；使用 page.setViewportSize；保留触控模拟，逐图检查视口／DPR／pointer／PNG 尺寸不变。 | 修后 3/3，零重试；每轮 19 张截图一致。 |
| upstream-selected-fixes，中文和 emoji 收件箱预览 | 原 trace 确认 WS 触发的旧文档归档 GET 被 waiter 捕获，goto 完成后旧 Response body 已被浏览器丢弃；新文档实际 GET 随后正常返回 JSON。 | 使用现有同身份 TestApiClient 读取真实归档列表；保留 preview、details、完整评论与浏览器导航断言，去除跨文档 Response 生命周期依赖。 | 目标 3/3，零重试。 |

各修改文件的定向 TypeScript、ESLint／diff 检查均通过。未使用强制点击、任意延时、增加通用超时或删除关键断言。

## 所有权与源码

根工作区已有另一任务的迭代工具栏调整，包含 `iterations-audit-desktop.spec.ts` 的额外用例和选择器更新。本次只暂存验证 worktree 生成的独立审计补丁，保留该并行工作。验证目录未导入其他任务的产品代码、fixture 或文案。

完整验证候选与 SHA-256 位于 `.omx/state/iteration-settings-final-20261010/verified-code.json`；应用输入与完整 TypeScript/Go 运行一致，后续仅四份浏览器测试变化，外加 Next 生成的 route-types 引用。测试文件也计入运行身份，因此在全部补丁稳定后重新启动生产服务并核对状态，再运行最终完整浏览器套件。

原始诊断与补丁在主工作区 `.omx/state/iteration-settings-final-20261010/{category,inbox}/` 和验证 worktree `.omx/iteration-audit-debug/`。先前原型、10-09 失败及本轮中止记录均保留，不被最新结果覆盖。

## 补充视觉检查

当前场景最终通过，12 张截图覆盖设置／总览／已加载任务详情／键盘数据表、中英文、1440px／900px、浅色／深色。初始采集包含任务加载骨架；补充等待一度误用不含任务编号的精确文本定位。修正为实际带编号的任务文本并等待可见后重新采集通过，产品代码未变化。相关初始记录保留在验证目录 `.omx/iteration-final-visual/{initial-captures,loading-wait-error}/`。最终图片与结构化视觉 verdict 在父任务 `final-evidence-2026-10-10/`。

## 最终结果

完整复跑为 269 passed、0 unexpected、50 条件跳过、0 flaky。随后当前分支合并回归 19/19 通过；审计测试的三处纯 lint 修正后，Electron audit 又通过 1/1。最终源码与构建信息见父 verification.md，原失败不再构成未关闭事项。
