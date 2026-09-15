# Design — 设置页左侧导航四分组重构

## 1. 现状

`packages/views/settings/components/settings-page.tsx` 用四个平行常量描述导航：

- `ACCOUNT_TAB_KEYS` + `ACCOUNT_TAB_ICONS`（个人组，key 直接当 tab value）
- `WORKSPACE_TAB_KEYS` + `WORKSPACE_TAB_VALUES` + `WORKSPACE_TAB_ICONS`（工作区组，key 与
  tab value 不同，如 `general → "workspace"`、`issue_statuses → "issue-statuses"`）

渲染时两段 `map`，中间插一个分组标题 `<span>`；flag 过滤只作用于工作区组
（`visibleWorkspaceTabKeys`）。`validTabs` 由这些常量派生，用于 `?tab=` 白名单校验。

要拆成四组，继续加平行常量会变成 8 个数组，且「组为空则不渲染标题」这种规则没有承载处。

## 2. 目标结构

单一声明式结构，key / value / icon / flag 收在一处：

```ts
type SettingsNavItem = {
  /** i18n key under page.tabs */
  key: string;
  /** ?tab= value; defaults to key when omitted */
  value?: string;
  icon: React.ComponentType<{ className?: string }>;
  /** Feature flag gating this item; absent = always visible */
  flag?: string;
};

type SettingsNavGroup = {
  /** i18n key under page.groups */
  id: "account" | "workspace" | "issue" | "connections";
  items: readonly SettingsNavItem[];
};

const SETTINGS_NAV_GROUPS: readonly SettingsNavGroup[] = [
  { id: "account", items: [
    { key: "profile",       icon: User },
    { key: "preferences",   icon: SlidersHorizontal },
    { key: "notifications", icon: Bell },
    { key: "shortcuts",     icon: Keyboard },
    { key: "tokens",        icon: Key },
  ]},
  { id: "workspace", items: [
    { key: "general", value: "workspace", icon: Settings },
    { key: "members",                     icon: Users },
    { key: "billing", icon: CreditCard, flag: BILLING_WORKSPACE_SUBSCRIPTIONS_FLAG },
  ]},
  { id: "issue", items: [
    { key: "issue_statuses", value: "issue-statuses", icon: CircleDot },
    { key: "labels",                                  icon: Tag },
    { key: "properties",                              icon: SlidersHorizontal },
    { key: "quick_actions",  value: "quick-actions",  icon: Zap },
  ]},
  { id: "connections", items: [
    { key: "repositories", icon: FolderGit2 },
    { key: "integrations", icon: Plug },
    { key: "mcp",          icon: Server },
    { key: "plugins",      icon: Blocks, flag: PLUGINS_V1_FLAG },
  ]},
] as const;
```

`value ?? key` 保证所有现有 `?tab=` 取值不变。

### 2.1 flag 求值

`useFeatureEnabled` 是 hook，不能在 `map` 里按需调用。保持现在的做法：在组件顶层各调一次，
放进一个 `Record<string, boolean>`，再喂给纯函数过滤：

```ts
const pluginsEnabled = useFeatureEnabled(PLUGINS_V1_FLAG, false);
const billingEnabled = useFeatureEnabled(BILLING_WORKSPACE_SUBSCRIPTIONS_FLAG, false);

const flagState = React.useMemo(
  () => ({
    [PLUGINS_V1_FLAG]: pluginsEnabled,
    [BILLING_WORKSPACE_SUBSCRIPTIONS_FLAG]: billingEnabled,
  }),
  [pluginsEnabled, billingEnabled],
);

const visibleGroups = React.useMemo(
  () => visibleSettingsNavGroups(SETTINGS_NAV_GROUPS, flagState, extraAccountTabs),
  [flagState, extraAccountTabs],
);
```

`visibleSettingsNavGroups` 是纯函数：按 flag 过滤 item、把 `extraAccountTabs` 追加到
`account` 组末尾、丢掉过滤后为空的组。放在同文件导出，便于单独用 node 环境测试
（CLAUDE.md：纯逻辑矩阵归 `.test.ts`，不通过 DOM 挂载重跑）。

新增文件 `settings-nav.ts` + `settings-nav.test.ts`（`// @vitest-environment node`）承载类型、
常量与该纯函数；`settings-page.tsx` 只负责渲染。

### 2.2 渲染

```tsx
{visibleGroups.map((group) => (
  <React.Fragment key={group.id}>
    <span className={GROUP_LABEL_CLASS}>{t(($) => $.page.groups[group.id])}</span>
    {group.items.map((item) => (
      <TabsTrigger key={item.value} value={item.value} className={SETTINGS_TAB_TRIGGER_CLASS}>
        <item.icon className="h-4 w-4" />
        {item.label}
      </TabsTrigger>
    ))}
  </React.Fragment>
))}
```

`GROUP_LABEL_CLASS` 统一为 `hidden px-2 pb-1 pt-4 text-caption font-medium text-muted-foreground md:block`，
首组用 `pt-2`（沿用现状：第一组紧贴标题，后续组间距更大）。纯函数输出的 item 带上已解析好的
`label`，让渲染层不必区分「i18n key 项」和「desktop 注入项」。

`workspace_fallback` 与 `useCurrentWorkspace()` 在导航里不再需要 —— 分组标题固定文案。
（`useCurrentWorkspace` 在本文件无其他用途，一并移除 import。）

## 3. 菜单项收敛

### 3.1 任务 / 聊天 → 偏好设置

`preferences-tab.tsx` 现在是单个 `SettingsSection`（外观、语言、时区、评论栏）。合并后结构：

```
SettingsTab title=偏好设置
  SettingsSection (现有：外观/语言/时区/粘性评论栏)
  SettingsSection title=issue.quick_create_title  desc=issue.quick_create_description
  SettingsSection title=issue.manual_create_title desc=issue.manual_create_description
  SettingsSection title=chat.floating_title
```

- `issue-tab.tsx` / `chat-tab.tsx` 的 JSX 原样搬迁：`useIssueCreateSettingsStore`、
  `useChatStore` 的读写与 `savedToast()` 不变。为控制 `preferences-tab.tsx` 体积，搬迁为
  同文件内的两个局部组件 `IssueCreateFieldSections` / `FloatingChatRow`，与既有
  `StickyCommentBarRow` / `TimezoneRow` 的写法一致。
- `issue-tab.test.tsx` **存在**（3 条用例：字段开关渲染与持久化矩阵）——迁移到
  `preferences-tab.test.tsx` 新增 describe 块。原用例用 `getAllByRole("switch")` 的位置索引，
  合并后页面上多了粘性评论栏与悬浮聊天两个开关，位置索引会错位：改用带名字的查询
  （`getAllByRole("switch", { name: "Priority" })` 等）重写断言，语义不变。
- `issue.description`（原 tab 级说明）降级为 `quick_create_title` 那组的补充说明会重复，
  直接删除该 key。
- 删除 `issue-tab.tsx`、`issue-tab.test.tsx`、`chat-tab.tsx`。（二者不在 `components/index.ts` 的公开导出里，只被
  `settings-page.tsx` 引用，删除后同步移除该 import。）

### 3.2 GitHub → 集成

`github-tab.tsx` 目前是 `SettingsTab title={page.tabs.github}` 包着 4 个 `<h2>` + `Card` 区块。
`integrations-tab.tsx` 里的同级子组件（`LarkTab` / `SlackTab` / `VCSTab`）都是**裸 Card**，
由父级提供 `SettingsSection title`。因此：

- `GitHubTab` 去掉 `SettingsTab` 外壳，改为 `<div className="space-y-6">` 包裹原有 4 个区块；
  内部 `<h2 className="text-body font-semibold">` 降为 `<h4>`（`SettingsSection` 的标题是
  `<h3>`，避免标题层级跳跃），className 不变。
- `integrations-tab.tsx` 在最前面插入：

```tsx
<SettingsSection
  title={<span className="flex items-center gap-2"><GitHubMark className="h-4 w-4" />GitHub</span>}
  description={t(($) => $.github.page_description)}
>
  <GitHubTab />
</SettingsSection>
```

  GitHub 段落无 flag/config 门控（与现状一致，始终显示），放在首位。文件顶部说明 GitHub
  有独立顶级 tab 的注释要改写。
- 新增 `github.section_title = "GitHub"`（四语言同值），删除 `page.tabs.github`。

### 3.3 实验室 → 删除

删除 `labs-tab.tsx`、`settings-page.tsx` 的 import、`page.tabs.labs`、`labs.*` 三个 key。
`labs.toast_failed` 经确认已无任何引用（Co-authored-by 开关早已迁到 GitHub tab）。

## 4. URL 兼容

```ts
const LEGACY_WORKSPACE_TAB_REDIRECTS: Record<string, string> = {
  lark: "integrations",
  // The GitHub App install callback lands on /settings?tab=github — the Go
  // handler's default return target (handler/github.go, githubSettingsURL).
  // GitHub settings now live inside Integrations.
  github: "integrations",
};
```

- `github` 映射是**必须**的：它是后端驱动的 URL，属于 CLAUDE.md 里「API 边界」那一类，不是
  内部兼容层。后端 `return_to` 白名单（`github` / `repositories`）不改动。
- `issue` / `chat` / `labs` **不加**映射：站内链接直接改，未知值按现有逻辑回退默认 tab。
- 站内 `?tab=issue` 两处改为 `?tab=preferences`：
  `packages/views/modals/create-issue.tsx:1302`、
  `packages/views/modals/quick-create-issue.tsx:823`（对应两个测试断言同步改）。

## 5. 文案变更（四语言各一份）

| 操作 | key |
| --- | --- |
| 新增 | `page.groups.account` / `page.groups.workspace` / `page.groups.issue` / `page.groups.connections` |
| 新增 | `github.section_title` |
| 删除 | `page.my_account`、`page.workspace_fallback` |
| 删除 | `page.tabs.issue`、`page.tabs.chat`、`page.tabs.labs`、`page.tabs.github` |
| 删除 | `issue.description`、`labs.*`（3 个） |
| 保留 | `issue.quick_create_*` / `issue.manual_create_*` / `issue.fields.*` / `chat.floating_*` / `github.*`（其余） |

zh-Hans 取值：个人设置 / 工作区管理 / 任务配置 / 连接与扩展。
en：Personal / Workspace / Issues / Connections。ja / ko 同义翻译。

## 6. 影响面与非目标

改动文件：

- `packages/views/settings/components/settings-nav.ts`（新增）+ `settings-nav.test.ts`（新增）
- `packages/views/settings/components/settings-page.tsx`
- `packages/views/settings/components/settings-page.test.tsx`
- `packages/views/settings/components/preferences-tab.tsx` + `preferences-tab.test.tsx`
- `packages/views/settings/components/integrations-tab.tsx` + `integrations-tab.test.tsx`
- `packages/views/settings/components/github-tab.tsx` + `github-tab.test.tsx`
- 删除：`issue-tab.tsx`、`chat-tab.tsx`、`labs-tab.tsx`（三者均无独立测试文件，且未在
  `components/index.ts` 中导出）
- `packages/views/modals/create-issue.tsx`、`quick-create-issue.tsx` + 两个测试
- `packages/views/locales/{en,zh-Hans,ja,ko}/settings.json`

不动：

- 后端（`server/internal/handler/github.go` 的 `return_to` 白名单保持 `github` / `repositories`）
- 各 tab 的业务逻辑、store、query
- `apps/web` / `apps/desktop` 的路由与 `extraAccountTabs` 调用点
- `apps/mobile`（独立设置页，不共享本导航）

## 7. 风险

| 风险 | 缓解 |
| --- | --- |
| GitHub 安装回调落到集成页后需滚动才能看到 GitHub 段落 | GitHub 放在集成页首个 section，回调后即在视口内 |
| 删除 locale key 漏掉某语言 → parity 测试红 | 四个 json 一次改完，`parity.test.ts` 兜底 |
| 分组标题 `pt-4` 让首组与「设置」标题间距过大 | 首组沿用 `pt-2`，与现状一致 |
| GitHubTab 内部 `<h2>` 降级改错 className 影响视觉 | 只改标签名，className 原样保留 |
