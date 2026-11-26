# 实施计划

## 范围

- 空目录欢迎组件与示例卡片
- 迭代列表分支及创建按钮接线
- 英文和简体中文文案
- 首次空目录、空结果分流及现有状态回归测试

不修改 Core/API、迭代详情页、迭代启用配置或其他欢迎页。

## 步骤

1. 在 `iteration-navigation.test.tsx` 增加首次空目录与边界测试，确认测试因当前缺少欢迎页而失败。
2. 实现 `IterationWelcome` 展示组件和静态本地化示例。
3. 在 `IterationOverview` 中只对成功、启用、未筛选且全目录为空的首次状态显示欢迎页；让欢迎 CTA 打开现有创建对话框，并避免重复创建按钮。
4. 为英文和简体中文增加等价文案，运行 locale parity 检查。
5. 运行迭代导航测试和相关 UI 组件测试，修正失败。
6. 执行 views lint/typecheck、Trellis 全范围质量检查和一次 Impeccable detector；检查 Web 与 Desktop 共用约束及窄屏布局。
7. 用隔离工作区运行 `e2e/iteration-welcome.spec.ts`，捕获桌面英文与手机简体中文页面，确认实际 CSS、粗指针操作和横向溢出。

## 回归场景

- 成功空目录且功能启用 → 显示欢迎页。
- 欢迎 CTA → 打开既有创建对话框。
- 目录有任意历史项但当前列表为空 → 不显示首次欢迎。
- 筛选无结果 → 显示无匹配状态和清除筛选。
- 初次加载、错误/刷新错误、功能禁用 → 不显示欢迎页。
- 单个空计划迭代 → 保留现有添加任务和新建任务入口。

## 验证命令

```bash
pnpm --filter @multica/views exec vitest run iterations/iteration-navigation.test.tsx
pnpm --filter @multica/views lint
pnpm --filter @multica/views typecheck
pnpm --filter @multica/views exec vitest run locales/parity.test.ts
pnpm exec playwright test e2e/iteration-welcome.spec.ts
```

如全包质量检查发现与本次无关的既存问题，记录具体失败和受影响范围，不修改无关文件。
