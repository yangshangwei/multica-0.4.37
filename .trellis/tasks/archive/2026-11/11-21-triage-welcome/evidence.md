# 分拣台欢迎页验收

已完成确认的首次空白范围。共享页面显示「先收集，再决定做什么」、创建/CSV/文档入口和静态任务流示例。成功空队列、零未决数及未筛选空历史共同确认首次使用；日常清空、稍后处理和筛选保留各自反馈。

## 验收对应

| 标准 | 证据 |
| --- | --- |
| AC1 首次空白及上下文排除 | `triage-page-recovery.test.tsx` 的真实 QueryClient 历史确认、缓存失效、刷新错误与筛选/排序测试；`triage-page.test.tsx` 的原有路由、选择和分页回归 |
| AC2 操作可用及返回队列 | `e2e/triage.spec.ts` 真实创建、CSV、应用内文档导航；创建按钮以键盘 Enter 打开，创建及导入后回到实际队列 |
| AC3 被动示例 | 双语示例保持同一编号、状态和保留评论附件的说明；宽屏和滚动后的窄屏截图已人工检查 |
| AC4 日常空态和恢复 | 审核后空态标题保持键盘焦点；真实点击历史、稍后处理；组件测试覆盖旧的空历史失效及查询失败 |
| AC5 布局及可访问操作 | 两种语言 × 两种主题 × 1440/1024/390/320 宽度，共16组截图；逐组验证创建按钮在首屏、内容无横向溢出及示例圆角实际生效 |

## 最终检查

- 隔离源码候选中的分拣台及语言键测试：18 个文件、262 条通过，使用 `--maxWorkers=2`。
- 相关 Core 分拣台和路径测试：103 条通过。
- 内置文档链接校验：4 条通过。
- `triage.spec.ts`、`triage-layout.spec.ts`、`triage-branches.spec.ts` 和 `triage-duplicate-picker.spec.ts`：21 条真实 API 浏览器用例通过，1 个 worker、0 次重试。
- Core、Views 和 Desktop 的 typecheck 通过；生产 Web 构建及其 TypeScript 检查通过。
- 变更的 Views 文件 ESLint、范围内 `git diff --check`、Trellis 上下文校验通过；Impeccable detector 返回 `[]`。
- 独立 Trellis 检查未发现需要修改的缺陷。Impeccable / visual-verdict 首轮及最终确认评分 94，达到90分阈值。

## 环境及原始证据

本地完整日志、隔离源码和截图保留于：

`.omx/private/qa-work/triage-welcome-20261121/`

- `final-environment.json`：基线提交、7个任务文件的 SHA-256、最终生产 Build ID、实际 API PID/提交及检查摘要。验收后再次比较了候选与当前任务文件，哈希一致。
- `web-final-build.log`、`browser-final.log` 和 `browser-final/`：最终生产构建、21条浏览器用例结果、截图及16组布局指标。
- `triage-tests-final.log`、`core-related-tests.log`、`docs-link-tests.log`、`views-typecheck-final.log`、`desktop-typecheck-final.log`：最终针对性检查结果。
- `.omx/state/triage-welcome/ralph-progress.json`：参考图、人工视觉评估及最终确认。

首轮隔离测试与多个 TypeScript/构建同时运行时，4个已有表单测试触及5秒超时；减少并发后完整262条测试在原超时限制下通过。当前工作区另有 Inbox/Iteration 的进行中修改；隔离候选以任务开始时的提交为基线，只覆盖本任务文件，未改动这些工作。

## 范围与限制

复用了现有创建、导入、查询和导航，只新增一个展示组件及其状态分支。首次使用状态由服务器数据派生。验证覆盖生产 Web；Desktop 的共享组件消费通过类型检查。未操作原生 Electron 窗口、安装包或实际模型，也未运行全仓 `make check` 或无关 Go 测试。
