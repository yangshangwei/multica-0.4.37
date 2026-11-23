# 运行时欢迎页验收

| 标准 | 验收结果 |
| --- | --- |
| AC1 空态与恢复 | `runtimes-page.test.tsx` 覆盖两份清单成功为空、读请求等待、失败后重试和后台刷新失败。已有机器（含离线）、待配置档案、本机占位及启动中状态继续使用相应的管理视图。 |
| AC2 首次使用介绍 | 欢迎页按参考图保留左侧说明、右侧手动轮播和两种接入示例。示例标签标明设备、CLI 及连接状态的用途。 |
| AC3 连接入口 | 浏览器实际打开“添加电脑”弹窗并进入当前工作区的连接指南。组件测试覆盖配置桌面端下载地址后的入口切换及安全链接属性。 |
| AC4 切换到机器列表 | 隔离 API 注册一台测试机器，WebSocket 失效通知触发页面切换。注销后 API 返回离线状态，页面按既有规则显示 `Recently lost`，机器仍可打开。 |
| AC5 布局与键盘 | 中英文与明暗主题在 1440、1024、390 和 320 px 共 16 组布局下通过滚动、溢出和圆角检查。Chromium 验证轮播位置播报、隐藏卡片、减少动态效果，以及两个箭头在到达端点后仍保留焦点。 |
| AC6 验证与终审 | 24 个运行时及语言测试文件共 370 条通过。变更的 Views 文件 ESLint、Views 和 Desktop 类型检查、生产 Web 构建及 Chromium 完整场景通过。终审判定焦点问题已解决，视觉复核 95/100。 |

## 端点焦点回归

旧生产构建中，按 Enter 前进时箭头在端点变为原生 `disabled`，新增的 `toBeFocused()` 断言因此失败。两个箭头启用共享 Base UI Button 的 `focusableWhenDisabled`，通过 `aria-disabled` 表达端点状态。修复后的生产用例验证双向操作均保留焦点。远程示例截图显示处于禁用状态的下一步箭头和可见焦点环。

## 证据和范围

本地证据位于 `.omx/private/qa-work/runtime-welcome-20261121/`。

- `resume-environment.json` 记录基线提交、生产 Build ID、API 和 Web 进程标识以及 9 个任务源码与测试文件的哈希。任务工作区与隔离候选的这些文件一致。
- `resume-focus-red.log` 保留修复前的失败。`resume-web-build.log` 与 `resume-browser.log` 记录重建和 1 条 Chromium 场景通过，使用一个 worker、零次重试。
- `resume-browser/` 保留 26 张截图与 16 组布局测量。`resume-contact-sheet.jpg` 汇总截图，`resume-capture-validation.json` 记录尺寸及哈希。
- `resume-check-review.md` 与 `resume-check-*.log` 记录独立 Trellis 检查。修复前的 Impeccable 检查器结果为 `[]`，修复后的独立终审和评分记录在 `.omx/state/runtime-welcome/ralph-progress.json`。
- `resume-cleanup.json` 记录测试工作区已删除，任务专用 API、Web 和临时数据库已清理。

验收覆盖生产 Web 与 Desktop 类型检查。原生 Electron、安装包及全仓 `make check` 未纳入本次验证。连接场景使用隔离 API 的测试注册数据。
