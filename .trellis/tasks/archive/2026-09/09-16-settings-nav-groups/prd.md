# 重构设置页左侧导航为四个分组

## Goal

让 Web / Desktop 设置页左侧导航与设计稿一致：从当前「我的账号 + 动态工作区名」两个分组，
重构为「个人设置 / 工作区管理 / 任务配置 / 连接与扩展」四个分组，并把设计稿中不存在的四个
菜单项收敛掉（不丢功能）。

设计稿（用户提供，权威来源）：

```
设置
个人设置
  个人资料 / 偏好设置 / 通知 / 快捷键 / API Token
工作区管理
  常规 / 成员
任务配置
  任务状态 / 标签 / 属性 / 快捷操作
连接与扩展
  代码仓库 / 集成 / MCP
```

## Requirements

### R1 导航分组结构

- 左侧导航渲染四个分组，顺序与组内顺序严格按设计稿。
- 分组标题固定文案，四语言（en / zh-Hans / ja / ko）齐备。
- 「工作区管理」标题不再显示动态工作区名称，改为固定文案。
- 某分组内所有项都被 feature flag 过滤掉时，整组（含标题）不渲染。
- 移动端横向滚动布局不变：分组标题仍只在 `md` 以上显示，横向顺序等于四组拼接顺序。

### R2 菜单项收敛（设计稿中不存在的四项）

| 现有项 | 处理方式 | 功能归属 |
| --- | --- | --- |
| 个人设置 › 任务 | 删除独立 tab | 内容并入「偏好设置」，作为两个 section |
| 个人设置 › 聊天 | 删除独立 tab | 内容并入「偏好设置」，作为一个 section |
| 工作区 › GitHub | 删除独立 tab | 内容并入「集成」，作为首个 section |
| 工作区 › 实验室 | 直接删除 | 当前是空占位页，无功能损失 |

- 三项合并后原有开关行为、持久化、toast 全部保持不变。
- flag 控制的「账单与套餐」「Plugins」默认关闭，与设计稿默认态一致；开启时分别归入
  「工作区管理」和「连接与扩展」。
- Desktop 注入的 Daemon / 服务端 / 更新三项继续追加在「个人设置」组末尾，`extraAccountTabs`
  接口签名不变。

### R3 URL 兼容性

- 现有全部 `?tab=` 取值不变（`workspace`、`issue-statuses`、`quick-actions` 等）。
- `?tab=github` 必须继续可用并落到「集成」：Go 后端在 GitHub App 安装回调后重定向到
  `/settings?tab=github`（`server/internal/handler/github.go:455`，默认 return target 就是
  `github`）。这是后端驱动的边界，需要保留重定向映射。
- 仓库内指向 `?tab=issue` 的两处链接直接改成 `?tab=preferences`（内部代码，不加兼容层）。
- `?tab=labs` / `?tab=chat` 不加映射，落到白名单外 → 回退默认 tab。

### R4 图标

- 「标签」图标从 `Tags`（双标签）改为 `Tag`（单标签），与设计稿一致。
- 其余图标保持现状（已与设计稿一致）。

## Constraints

- 遵循 `packages/views` 边界：不得引入 `next/*`、`react-router-dom`、store。
- 文案改动必须四语言同步，`packages/views/locales/parity.test.ts` 必须通过。
- 不新增 UI 组件；复用 `settings-layout.tsx` 的 `SettingsSection` / `SettingsCard` / `SettingsRow`。
- 不做超出本任务的重构（不动各 tab 内部业务逻辑）。

## Acceptance Criteria

- [ ] 默认态（flag 全关）下，左侧导航逐项等于设计稿：4 个分组、14 个菜单项、顺序一致。
- [ ] 四个分组标题在 en / zh-Hans / ja / ko 下都有对应文案，`parity.test.ts` 通过。
- [ ] 「偏好设置」页含原「任务」的两个 section（智能体创建 / 手动创建字段开关）与原「聊天」的
      悬浮窗开关，开关读写与 toast 行为与合并前一致。
- [ ] 「集成」页首个 section 是 GitHub，内容与原 GitHub tab 一致（总开关、连接、功能、仓库入口）。
- [ ] `?tab=github` 打开设置页时落到「集成」并高亮「集成」菜单项。
- [ ] `?tab=labs`、`?tab=chat`、`?tab=issue` 不再渲染独立 tab；`issue` 的两处站内链接已指向
      `?tab=preferences`。
- [ ] Plugins / Billing flag 关闭时，「连接与扩展」「工作区管理」不出现空分组标题；开启时各自
      出现在对应分组内。
- [ ] Desktop 的 Daemon / 服务端 / 更新仍在「个人设置」组末尾，`?tab=updates` 深链仍可用。
- [ ] `pnpm typecheck` 与 `pnpm test --filter @multica/views`（含新增回归测试）通过。
