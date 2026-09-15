# 同步引导侧栏与「进入项目」终点项

## Goal

Onboarding 的最后一步（连接运行时）的 CTA 是「进入项目」，点击后引导结束并落到工作区 Projects 页。但左侧引导栏（StepSidebar）只列出三个持久化步骤，对「进入项目」这一真实下一站完全不知情，用户看栏无法预期 CTA 之后还会进项目。

在左侧栏（及移动端进度条）增加一个「进入项目」终点项：只展示、不可导航、永远不会成为"当前步"，作为引导的出口预告，与右侧 CTA 语义对齐。

## Requirements

- 终点项紧跟在 `ONBOARDING_STEP_ORDER` 的最后一项（runtime）之后渲染，样式与"未来步骤"一致（弱化、不可点击、indicator 为空环）。
- 终点项不是 `OnboardingStep`，不加入 `ONBOARDING_STEP_ORDER`，不影响 `nextStep` / `handleBack` / `canReturn` 等导航逻辑。
- 移动端 `StepProgressBar` 的分段数同步包含终点项（最后一段永远不点亮，表示引导之后的下一站）。
- 文案：`step_nav` 新增 `project` 条目（label + description），en / zh-Hans / ja / ko 四个语言全部补齐，通过 locale parity 测试。
- 不改动 `onboarding-flow.tsx` 的完成/导航逻辑，不改动 `step-order.ts`。

## Acceptance Criteria

- [x] 桌面端引导栏在「连接运行时」下方显示「进入项目」终点项；runtime 为当前步时，终点项呈弱化样式，不可点击（无 button、无 hover 反馈）。
- [x] 终点项在 about_you / workspace 步骤时同样可见（始终渲染，作为流程预告）。
- [x] 移动端（<md）进度条分段数与侧栏项数一致。
- [x] 四个语言的 `step_nav.project` 均存在，`pnpm test` 中 locale parity 测试通过。
- [x] 侧栏现有行为不回归：已完成步骤仍可点击返回，Back 按钮行为不变。
- [x] `pnpm typecheck` 通过；相关组件测试更新并通过。

## Notes

- 背景与方案讨论（方案 A/B/C 对比）见本会话；选定 A：终点项承认「进入项目」是流程的真实下一站，但不伪装成可交互的表单步骤。
- `step-order.ts` 注释中「Runtime is the final form step」的表述可顺带补充一句：项目落地页以终点项形式在栏中预告。
