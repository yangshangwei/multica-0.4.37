# 协作创建表单焦点修复

按要求复测并修复上一轮 `creation-form.test.tsx` 的焦点失败，不创建 Trellis 任务。保留此前已有的布局改动；本轮产品改动仅在创建入口的初始聚焦逻辑。

## 根因

Base UI 的初始聚焦先在微任务中决定目标，再排入动画帧。其“焦点是否已在弹窗内”的判断使用此前捕获的元素，实际执行时不会重新检查用户是否已聚焦来源选择框。异步来源数据及调度顺序使标题输入框在用户选择来源控件后再次获得焦点，整文件失败、单独运行偶尔通过。

## 修改

- `packages/views/collaboration/creation-form.tsx`：创建入口接管初始聚焦；仅在触发按钮仍拥有焦点时同步移交到弹窗，向 Base UI 返回 `false` 关闭其无条件排队聚焦。复用现有 `recoverFocus`，在执行帧中重新解析目标和当前焦点，并拒绝已关闭弹窗。
- `packages/views/collaboration/creation-form.test.tsx`：保持原来源/标题焦点断言，通过受控动画帧稳定覆盖异步加载与用户提前聚焦的顺序；新增关闭后再释放排队帧的回归，确认弹窗不重新获得焦点且焦点返回打开按钮。
- `.trellis/spec/views/frontend/collaboration-creation.md`：记录实际执行时检查焦点、拒绝关闭弹窗以及受控帧回归的约定。

没有修改共享 focus helper、Dialog primitive、Core 状态、API、依赖或其他功能，没有放宽原断言或增加自动重试。

## 证据

| 运行 | 结果 | 文件 |
| --- | --- | --- |
| 原整文件修复前 | 23 通过 / 1 失败 | `implementation/red-full.{log,json}` |
| 受控帧复现修复前 | 1 通过 / 1 失败 | `implementation/red-deferred.{log,json}` |
| 受控帧复现修复后 | 2 / 2 通过 | `implementation/green-deferred.{log,json}` |
| 原整文件修复后 | 24 / 24 通过 | `implementation/green-full.{log,json}` |
| 含关闭回归的整文件 | 25 / 25 通过 | `implementation/green-with-close.{log,json}`、`verification/creation-final.{log,json}` |
| 创建、收件人、附件、焦点相关组件 | 50 / 50 通过 | `implementation/green-related.{log,json}` |
| 协作相关完整回归（含关闭回归） | 32 文件、284 / 284 通过 | `verification/collaboration-final.{log,json}` |
| 修改文件 ESLint | exit 0 | `verification/eslint-corrected.log` |
| Views TypeScript | exit 0 | `verification/typecheck-corrected.log`、`verification/final.json` |

独立只读审查未发现产品实现阻塞问题，建议的关闭时序覆盖已补齐。补充用例最初误用了 Playwright 的 `exact` 参数，最终类型检查发现 TS2769；已移除该不受 Testing Library 支持的参数，其字符串名称匹配仍为精确匹配。保留首次类型失败日志，修正后再次运行整文件及静态检查。

本轮验证是共享组件及静态检查，未重复前一轮完整后端、全量前端或生产 E2E。受控动画帧验证真实组件和真实 Dialog primitive 的异步焦点行为；未据此宣称物理触屏设备或屏幕阅读器验收。
