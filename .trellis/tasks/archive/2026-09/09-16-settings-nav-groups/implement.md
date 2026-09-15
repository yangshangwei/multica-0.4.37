# Implement — 设置页左侧导航四分组重构

执行顺序按「先文案、再结构、再合并、最后测试」，每步之后代码都应保持可编译。

## Step 1 — 文案（四语言）

- [x] `packages/views/locales/{en,zh-Hans,ja,ko}/settings.json`
  - 新增 `page.groups.{account,workspace,issue,connections}`
  - 新增 `github.section_title`（四语言同值 `"GitHub"`）
  - 删除 `page.my_account`、`page.workspace_fallback`
  - 删除 `page.tabs.{issue,chat,labs,github}`
  - 删除 `issue.description`、`labs`（整个对象）
- [x] 取值：
  - zh-Hans：个人设置 / 工作区管理 / 任务配置 / 连接与扩展
  - en：Personal / Workspace / Issues / Connections
  - ja：個人設定 / ワークスペース / タスク設定 / 接続と拡張（弃用初稿「連携と拡張」——与
    Integrations 菜单项「連携」撞词）
  - ko：개인 설정 / 워크스페이스 / 태스크 설정 / 연결 및 확장

**验证**：`pnpm test --filter @multica/views -- locales/parity` 通过。
（此时 `settings-page.tsx` 仍引用已删 key → 类型/运行会红，Step 2 修复。）

## Step 2 — 导航结构

- [x] 新建 `packages/views/settings/components/settings-nav.ts`
  - `SettingsNavItem` / `SettingsNavGroup` 类型
  - `SETTINGS_NAV_GROUPS` 常量（四组，顺序见 design.md §2）
  - `visibleSettingsNavGroups(groups, flagState, extraAccountTabs)` 纯函数：
    flag 过滤 → `extraAccountTabs` 追加到 `account` 组 → 丢弃空组 → 返回带
    `{ value, label?, icon }` 的组列表（label 解析交给调用方传入的 `t`，或返回
    i18n key 由渲染层解析 —— 实现时二选一，保持纯函数不依赖 i18n 实例）
- [x] 改写 `settings-page.tsx`
  - 删除 `ACCOUNT_TAB_*` / `WORKSPACE_TAB_*` 六个常量
  - `visibleGroups` + 单段 `map` 渲染（design.md §2.2）
  - `validTabs` 从 `visibleGroups` 派生
  - `LEGACY_WORKSPACE_TAB_REDIRECTS` 增加 `github: "integrations"`，补注释说明是后端
    `githubSettingsURL` 的默认落点
  - 移除 `useCurrentWorkspace` import 与 `workspaceName`
  - `Tags` → `Tag` 图标
  - 删除 `IssueTab` / `ChatTab` / `LabsTab` / `GitHubTab` 的 import 与对应 `TabsContent`
  - 首组标题 `pt-2`、后续组 `pt-4`

**验证**：`pnpm typecheck`（此时 issue/chat/labs/github 四个文件还在，但已无人引用）。

## Step 3 — 任务 / 聊天 并入偏好设置

- [x] `preferences-tab.tsx`：在现有 `SettingsSection` 之后追加三个 section
  - 局部组件 `IssueCreateFieldSections`（搬 `issue-tab.tsx` 的两个 section，含
    `useIssueCreateSettingsStore` 读写与 `savedToast`）
  - 局部组件 `FloatingChatRow`（搬 `chat-tab.tsx` 的开关，含 `useChatStore`）
- [x] `preferences-tab.test.tsx`：迁入 `issue-tab.test.tsx` 的 3 条用例（合并页多了粘性评论栏
  与悬浮聊天开关，位置索引断言改为带名字的 switch 查询），并补悬浮聊天开关用例
- [x] 删除 `issue-tab.tsx`、`issue-tab.test.tsx`、`chat-tab.tsx`
- [x] `packages/views/modals/create-issue.tsx:1302`、`quick-create-issue.tsx:823`：
  `?tab=issue` → `?tab=preferences`（注释同步改），对应测试断言同步改

**验证**：`pnpm test --filter @multica/views -- preferences-tab create-issue quick-create-issue`

## Step 4 — GitHub 并入集成

- [x] `github-tab.tsx`
  - `SettingsTab` 外壳 → `<div className="space-y-6">`，移除 `SettingsTab` import 与
    `page.tabs.github` 引用
  - 四处 `<h2 className="text-body font-semibold">` → `<h4>`（className 不变）
  - 顶部注释改写：不再是顶级 tab
- [x] `integrations-tab.tsx`
  - 顶部注释改写：GitHub 现在是本页首个 section
  - 在 `messagingIntegrationsEnabled &&` 的 Lark 段之前插入 GitHub `SettingsSection`
    （`GitHubMark` 图标 + `github.section_title` + `github.page_description`）
- [x] 删除 `labs-tab.tsx`
- [x] `github-tab.test.tsx`：`searchParams` 由 `tab=github` 改为 `tab=integrations`

**验证**：`pnpm test --filter @multica/views -- github-tab integrations-tab`

## Step 5 — 回归测试

- [x] 新建 `settings-nav.test.ts`（首行 `// @vitest-environment node`）
  - 默认态返回 4 组 / 14 项，顺序逐项断言（这是设计稿的验收点，用快照式数组断言）
  - flag 全关时 `workspace` 组不含 billing、`connections` 组不含 plugins，且两组都非空
  - 构造一个「组内全部被 flag 过滤」的用例，断言该组被整体丢弃（空标题回归）
  - `extraAccountTabs` 追加到 `account` 组末尾
  - `value ?? key` 映射正确（`general → workspace` 等）
- [x] `settings-page.test.tsx` 补充
  - 四个分组标题都渲染（en 文案）
  - `?tab=github` 落到集成：断言渲染 `IntegrationsTab` 且 `集成` trigger 为 active
  - `?tab=labs` / `?tab=chat` 回退到默认 tab
  - 既有 Plugins / Billing flag 两组用例保持通过
  - 说明性注释指向 `settings-nav.test.ts` 为顺序矩阵的 canonical 位置（避免 DOM 里重跑矩阵）

**验证**：`pnpm test --filter @multica/views`

## Step 6 — 全量检查

- [x] `pnpm typecheck`
- [x] `pnpm lint`
- [x] `pnpm test`（turbo 全量：2 个 autopilot-dialog 5s 超时，单独复跑通过——既有并行负载
  flake，与本次改动无关；直接跑 views 全量 435 文件 / 5298 项全绿）
- [ ] 人工核对：`make up` 后打开 `/{slug}/settings`，对照设计稿逐项检查分组名、项名、顺序、图标
  —— 待办：web dev 服务器被并行会话重启中，暂不可达。结构层面（顺序矩阵、分组标题、
  tab 名、图标）已由 settings-nav.test.ts / settings-page.test.tsx 单测覆盖，唯像素级
  目检未做。
- [ ] 桌面端核对：Daemon / 服务端 / 更新 仍在「个人设置」末尾，`?tab=updates` 可达
  —— 逻辑已由 settings-nav.test.ts（注入 tab 追加语义）与 sidebar-version.test.tsx
  （`?tab=updates` 深链断言）覆盖；活体桌面目检未做。

## 回滚点

- Step 2 完成后如发现结构不合适：`settings-nav.ts` 可整体回退，`settings-page.tsx` 恢复
  两数组写法，Step 1 的文案改动独立可留。
- Step 3 / Step 4 互不依赖，任一步出问题可单独 revert 而不影响导航结构。

## 检查清单（对应 prd 验收）

- [x] 默认态导航逐项 == 设计稿
- [x] 四语言分组文案齐备，parity 通过
- [x] 偏好设置含建单字段 + 悬浮聊天开关，行为不变
- [x] 集成页首个 section 是 GitHub，内容完整
- [x] `?tab=github` → 集成
- [x] 站内 `?tab=issue` 已改为 `?tab=preferences`
- [x] flag 关闭不出现空分组标题
- [x] Desktop 三项与 `?tab=updates` 正常
- [x] typecheck / lint / test 全绿
